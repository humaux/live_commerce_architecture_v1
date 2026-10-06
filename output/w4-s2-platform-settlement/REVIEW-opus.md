# W4-S2 platform settlement: independent money-path review (Opus)

- Reviewed: `unit/w4-s2-platform-settlement` @ `123dd0d9` (author `7ff7b03d` + trunk merge). Diff `git diff r3/integration...HEAD`, 25 files.
- Reviewer: Claude Opus, read-only. I did not edit or commit anything in the branch. This file is the only thing I wrote.
- Local checks (DB-free), at `123dd0d9`:
  - `go build ./...`: exit 0.
  - `go vet` on the touched packages: clean.
  - `go test -count=1` on `psp/stripe/...`, `payments/settlement`, `payments/stripeadmin`, `cmd/stripe-admin`, `httpapi` and `platform`: all `ok`.
  - `scripts/dev/check-headers.sh`: OK.
  - PG suites (PF09–PF13, SL02, R2): NOT_RUN here. They run in CI.
- Fee vector, checked independently with exact `Fraction` arithmetic: 1046 × 100000 / (25640 × 100) = 40.796. Half-up gives 41, so the result is **−4100**.
  - The sign is right. A positive Stripe fee becomes a negative `fee_store_minor`, `stripe_fee_minor` is Σ of those values, and `net = … + stripe_fee`. So the fee is deducted from the store (0150:225, 535).

## Verdict: **MERGE-AFTER-FIX** (0 × P0, 2 × P1, 14 × P2)

The core is sound:
- **Attribution.** Lines are attributed only from Stripe ids and cross-checked against the facts. The derived-connection eligibility check is at 0150:362-364.
- **Idempotency.** It is keyed by balance-txn id, with a PK plus a lookup in both tables.
- **Concurrency.** A per-environment advisory lock serialises sync and close (0150:286, 463).
- **Immutability.** Set-once triggers enforce it (0150:176-200).
- **Merchant read.** It is scoped to the token's store, with explicit predicates that hold even if the GUC is forged, and it exposes no Stripe ids.
- **No secrets.** Nothing secret is in any argument. No payout or transfer call exists.

Two defects can silently turn the weekly cycle into a permanent block or an incomplete statement. Both are small SQL fixes.

---

## P1

### P1-1: A transient fact state is frozen into a permanent `mismatch`, which blocks close forever with no way to recover

**Evidence**
- The cross-check against `payments.facts` / `refund_facts` runs **once**, at insert (0150:375-397).
- A replay with an identical hash is counted as a duplicate and skipped (0150:311-315). The mismatch is never re-evaluated.
- `guard_settlement_line` forbids any change except `statement_id` (0150:180-182).
- Close refuses while any in-scope line has a mismatch (0150:479-481). The all-store close aborts if **any** store has one.
- REFUND requires a `SUCCEEDED` fact (0150:383). The test itself leaves store B permanently unclosable (`platform_settlement_test.go:353-354`, `:405-407`), and nothing in the suite clears it.

**Why it matters.** These are ordinary flows, not attacks:
- **(a)** A sync runs before the worker records the CAPTURED or SUCCEEDED fact (webhook lag, a worker backlog, or a sync window that ends "now").
- **(b)** A card refund is `pending` when the sync runs.
- **(c)** A refund fails before it ever succeeds. Stripe debits the balance at create, so a REFUND balance txn exists. Only a `FAILED` fact will ever exist, so the REFUND line is `no_fact` forever.

In each case the store can never close again, and the weekly all-store close is dead for every store. The only remedy is a superuser UPDATE or DELETE on an append-only production ledger. That is a red-line data change, and S2-OPEN-1 has no tool for it.

**Fix (small)**
1. On replay, when an existing line has the same hash, `statement_id IS NULL` and `mismatch IS NOT NULL`, re-run the cross-check. Allow the trigger to change `mismatch`, and only `mismatch`, while `statement_id IS NULL`.
2. The REFUND cross-check should accept the refund when it has **any terminal fact**, `SUCCEEDED` or `FAILED`/`CANCELED`. Compare the amount and currency against `payments.stripe_refunds` (the debit happened either way). The REFUND_FAILURE line then nets it.
3. Add a PF10 case: sync before the fact gives `no_fact`, the fact is recorded, a re-sync gives `mismatch IS NULL`, and close succeeds. Add a second case: a refund that only ever FAILED, synced as REFUND plus REFUND_FAILURE, closes with a net of 0.

### P1-2: Sync coverage can be recorded for time that has not passed yet

**Evidence**
- `record_settlement_lines` checks only that the window is at most 8 days and that `to > from` (0150:277-280).
- Go checks the same: `stripeadmin/settlement.go:74-76`, `:107`, and the CLI only parses RFC3339 (`cmd/stripe-admin/settlement.go:74-80`). Nothing compares `to` with now.
- Close accepts coverage from `settlement_sync_runs.window_to` without looking at `synced_at` (0150:468-474).

**Why it matters.** The coverage rule exists so that a statement is complete before payout. An operator can run Thursday's sync with `--to` set to next Monday, which is natural for "this week's window". That records coverage up to Monday, and close at Monday + 72 h passes.

The balance transactions created between Thursday and Monday are then never read. Coverage says the window is done, and later windows do not cover those `created` times. Their charges are never credited, and their refunds and disputes are never deducted.

The same gap opens on a smaller scale when `to` equals now: transactions committed at Stripe moments after the list call are missed.

**Fix (one line, plus a test).** Pick one:
- In SQL, refuse `p_window_to > clock_timestamp() - interval '15 minutes'` with 22023.
- In close, count only runs with `synced_at >= window_to + interval '15 minutes'`.

Add a PF11 case where a future window is refused or not counted.

---

## P2

1. **`--connection` is not checked against `stripe_platform.connection_id`** (the author's stated risk). **Severity: P2, low.** It is structurally constrained today:
   - The SQL scope must be the platform store (0150:281-283).
   - `stripe_one_account_per_store_environment` is a non-partial unique index on `(tenant, store, environment) WHERE provider='stripe'` (0061:30).
   - The envelope AAD binds the environment (`accounts/stripe_crypto.go:192-199`), so a SANDBOX credential cannot open in LIVE.

   So the only openable connection is the designated one. If that ever changed, a foreign account's charges would land as `unmapped_source` and block every close in the environment permanently (see item 2). Add the assertion anyway: pass the connection to SQL and compare it with `sp.connection_id`, or have the CLI read it.
2. **`unmapped_source` has no resolution path (S2-OPEN-1).** Any charge on the platform account that this system did not create permanently blocks every close in that environment (0150:476-478). Examples: a Dashboard charge, a payment link, or an owner's manual test. Not merge-blocking under MOCK/REAL_PG, but it must be resolved before LIVE settlement starts. Add it to the §8 checklist and add a `settlement-resolve` operator step that writes a resolution row rather than deleting.
3. **`unsupported_type` is final and never blocks** (0150:321, 327-329; close checks only `unmapped_source`). A CHARGE, REFUND or DISPUTE with a null `fee`, `exchange_rate` or `source`, an unexpanded source, or a `reporting_category` that fails `balanceTypeWord` (balance.go:157-161) is silently left off the store's statement.
   - This conforms to contract §6.3, but it is not fail-closed for money that belongs to a store. A missed refund or dispute overpays the store.
   - When `source_object` is charge, refund or dispute, classify the row as `unmapped_source` (blocking), or at least list it in the sync report.
4. **Half-up is not symmetric.** `floor(x+0.5)` rounds −40.5 to −40 (0150:225, 534). A negative-fee row, such as a fee returned on a dispute reversal, at an exact half credits 100 minor less than the charge debited. Use half-away-from-zero or document the rule. Exact integer arithmetic, `div(2·fee·store + settle·100, 2·settle·100)` with the sign handled, also removes the `numeric`-division argument in the 0150:221 comment.
5. **A quiet week breaks the chain.**
   - The all-store close skips a store with no lines in the period (0150:532).
   - The next week, `previous_period_open` fires for that store (0150:515-523) and **aborts the whole all-store close** (delta 8).
   - Fix: the all-store close also emits empty statements for stores with any earlier statement or line, or for every enrolled store.
6. **The SL02 pin is widened.** `slsSettlementObject` accepts **any** privilege on `payments.stripe_sessions*` for four roles, including `commerce_runtime` and `commerce_checkout_writer` (`stripe_live_schema_test.go` new func, ~l.905-913).
   - PF09 does not pin the new column grants on `stripe_sessions`, `stripe_refunds` or `payment_attempts`.
   - Narrow the exemption to `commerce_payment_registry_writer` and the exact columns 0150:164/167/171 grant, and pin them in PF09.
   - The other pins are honest: registrar list +4, R2 73→74, SL02 hold-back and ledger +5.
7. **`app.settlement_op` is a GUC the caller can set.** The policies at 0150:137-169 let any registry-writer definer read across stores when the caller has set it. Every reader today uses explicit tenant and store predicates: I checked 0077:343, 824, 900, plus 0150:241-245 and 641. So nothing leaks now.
   - Hardening: say so in the policy COMMENT.
   - Make `stripe_sessions_payment_intent_idx` **UNIQUE**. Code at 0150:353 assumes one session per payment intent and uses `LIMIT 1` with no ORDER BY.
8. **`window_net` double-counts** a transaction that moved from `unmapped_source` to a line, because the unattributed row is kept (0150:429-435). That produces a false reconciliation difference. Exclude unattributed rows that now have a line.
9. **Sub-second window edge.** Go lists `created[gte]=from.Unix()` and `created[lt]=to.Unix()`, which floor (balance.go:80), while SQL records the fractional `to`. Require whole seconds in the CLI, or truncate before both.
10. **Ticket not in the DB audit (delta 7).** The `--ticket` value is echoed only on stdout. Add it to the audit `details` through an optional argument.
11. **Evidence hygiene.** The logs are not bound to a SHA.
    - `mutation.log` injected two faults together. The PF11 failure shown is the fee line (`-6700` vs `-6800`), and the carry-forward assertion is never reached.
    - Re-run the carry-forward mutation on its own and record its red run.
12. `record_settlement_payout` does not require `paid_at >= closed_at` (0150:583). Trivial.
13. `settlement-export --out` inside the one-shot container needs a mounted volume (the author's risk). Put it in the runbook.
14. **Migration order.** 0150 is not lexically adjacent to anything that shares objects:
    - 0149 (`product_media_v2`, catalog) and 0151 (`sold_out_reply`, msgtemplates/claims/integration) touch no `payments.*`/settlement object, and 0150 has no `CREATE OR REPLACE` (no stale body copied). The 0152 customer-import migration reads `payments.*` tables but defines its functions under its own owners (no registry-writer definers), so the `app.settlement_op` policies cannot affect it.
    - The integrator must still make sure migrate.go accepts 0149 landing **after** 0150 has been applied to a deployed DB.

## Item-by-item notes (brief questions)

1. **Money.** The fee vector, sign, half-up (positive domain), refunds, partial refunds (one line per refund id), REFUND_FAILURE (+), DISPUTE and DISPUTE_REVERSAL (the type=adjustment and dispute-source pairing; an unknown category becomes `DISPUTE_UNKNOWN`, which becomes `unmapped_source` and blocks) all hold.
   - A store currency other than TWD causes a currency mismatch and blocks.
   - Negative-net carry works: `least(prev,0)`, and a payout on net ≤ 0 is refused.
   - Overlapping windows cannot double-count: the PK on `balance_txn_id` and the cross-table lookup prevent it.
   - Defects: P1-1, P1-2, P2-3, P2-4.
2. **Attribution.** It comes only from Stripe ids, and the account and connection eligibility is checked (0150:357-365).
   - No tenant, store or amount is taken from input, metadata or webhooks.
   - A store that is not allowlisted and not enrolled cannot have a session on the platform account (the derived row exists only through allowlisted enable). A charge on another primary account gives `foreign_connection`, which is tested.
   - After a disallow, an enrolled store keeps settling, which AD-PF2 requires.
3. **Close.** It is set-once and immutable, and the +72 h rule is correct.
   - The coverage rule computes the union correctly but can be cheated (P1-2).
   - Close and sync are serialised by the advisory lock.
   - The PT409 replay is safe (see delta 6).
4. **SECURITY DEFINER.**
   - All five entry points and the helpers have `search_path=pg_catalog`, schema-qualified names, owner `commerce_payment_registry_writer`, PUBLIC revoked, and EXECUTE for the registrar only, or `commerce_runtime` only for the reader.
   - FORCE RLS, no DELETE.
   - `billing:manage` is enforced both in the route (`withToolsScope`) and in SQL (`resolve_access`, with a final re-check at 0150:644-646). The store comes from the path and is authorised by the token. The tenant comes from the token.
5. **Wire.**
   - Strict JSON, with exact keys on the projection side.
   - Caps of 50 pages and 8 days return ErrUncertain and nothing.
   - 5xx and transport errors map to `ErrProvider`. Nothing is written, and a rerun is idempotent.
   - The key comes from the stored envelope and goes in the Bearer header, never the URL, and nothing logs it.
6. **CLI and ops-admin allowlist.**
   - No subcommand takes a secret.
   - A LIVE sync requires the pair, and a SANDBOX sync requires `STRIPE_SANDBOX=1`.
   - Close, export and payout read no Stripe or LIVE variable.
   - `--connection`: P2-1.
7. **Migration order.** See P2-14.
8. **Tests.**
   - The red run without the migration is real: all 5 subtests fail (`red.log`, exit 1).
   - Mutation: see P2-11.
   - Contract §6 criteria are encoded, except the P1 cases, a missing `source` row in PF10 (only fee and rate are tested at PG level), the future window, and close/sync concurrency.
9. **Headers.** All new files carry Purpose, Depends on, Used by, Invariants and Status. Every definer call site has a callee comment, and `check-headers` passes.

## Rulings on the DELIVERY deltas

DELIVERY lists 9 deltas; all 9 are ruled here.

| # | Delta | Ruling |
| --- | --- | --- |
| 1 | Additive projection (`ReportingCategory`, Charge/Refund presentment pairs) | **ACCEPT.** Without them a dispute cannot be classified, and `store_minor` would not come from Stripe, so the amount cross-check would be vacuous. Amend §6.5 to 21 keys. |
| 2 | `ListBalanceTransactions(ctx,gte,lt)` pages internally | **ACCEPT.** All-or-nothing matches "nothing written" better than a caller-driven cursor. |
| 3 | Extra columns: `order_id`, `payload_sha256`, unattributed tenant/store, `settle_net` | **ACCEPT.** They are needed for the merchant view, changed-content detection and RLS. Amend §6.1. |
| 4 | `settlement_sync_runs` plus the optional window | **ACCEPT, conditional on the P1-2 fix.** §6.2 named a rule with no object behind it. Recording coverage on the last chunk only is correct. |
| 5 | `read_store_settlements(...,p_statement)`; list without lines; `read_settlement_statement` operator read | **ACCEPT.** The list stays bounded and the detail carries the lines. Amend §6.4. |
| 6 | Changed content gives PT409 with nothing written (no unattributed row) | **ACCEPT. It is safe.** An unattributed row cannot outlive the aborting transaction. The window then never records coverage, so close of every overlapping period is `sync_required`: equally blocking, and fail-closed. Chunks are separate transactions, so earlier chunks may stay committed, which is idempotent and harmless. The projection excludes `status` and `available_on`, so legitimate content does not drift. Amend the §6.2 sentence. |
| 7 | `--target-store` instead of `--store`; ticket echoed on stdout only | **ACCEPT the rename** (`--store` is the scope flag). Add the ticket to the audit `details` (P2-10). |
| 8 | Close choices: empty statement for an explicit target; fee clamp ≥ 0; `floor(x+.5)`; all-store abort on any mismatch | **ACCEPT with the fixes.** The clamp and the abort are fail-closed and fine. Half-up must be sign-symmetric or documented (P2-4). The empty-statement rule needs the quiet-week fix (P2-5). The all-store abort is acceptable only once P1-1 makes mismatches recoverable. |
| 9 | `app.settlement_op` cross-store reads, column-limited grants, PI index | **ACCEPT with hardening (P2-7).** Make the index UNIQUE and narrow the SL02 exemption (P2-6). |

## Must fix before merge

- P1-1: mismatch re-evaluation, plus a REFUND cross-check against any terminal fact, plus two PF10 cases.
- P1-2: a window may not extend past the sync time, plus a PF11 case.
- P2-6 (pin narrowing) is cheap; do it in the same pass.
- Then re-run `focused:^TestPlatformSettlement`, `^TestPlatformStripe`, SL02 and R2 in CI at the new SHA.
