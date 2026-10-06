# W4-S2 platform settlement: independent re-verification of the fix round (Opus)

- Re-verified: `unit/w4-s2-platform-settlement` @ `70c9c8df` (fix `ea50b880` on top of `123dd0d9`, then the trunk merge). Fix diff: `git diff 123dd0d9 ea50b880`. The merge `ea50b880..70c9c8df` does not touch `migrations/0150_platform_settlement.sql`. All file:line references below are at `70c9c8df`.
- Reviewer: Claude Opus. Read-only: I edited and committed nothing. This file is the only thing I wrote.
- Local checks (DB-free), all at `70c9c8df`:
  - `go build ./...`: OK.
  - `go vet` on the touched packages and `tests/foundation`: clean.
  - `go test -count=1` on `stripeadmin`, `settlement`, `cmd/stripe-admin`, `platform` and `psp/stripe/...`: all `ok`.
  - `scripts/dev/check-headers.sh`: OK.
  - An exact-rational cross-check of the SQL fee formula against an independent reference and a port of the Go `pslFee`: 300,000 random signed vectors plus exact halves for k = 0..49 in all four sign combinations, with 0 differences. The PF11 vector still gives −4100, and the returned-fee vector gives +5900.
- Not run here: the PG suites (PF09 to PF13, SL02, R2). The author's `red-fix.log`, `green-fix.log` and `mutation-fix.log` ran on "123dd0d9 + uncommitted fix", not on a commit. CI must run them at `70c9c8df` (see P2-a).

## Verdict: **MERGE**. P1-1 FIXED, P1-2 FIXED, 0 new P0/P1, 9 P2.

P2-a is the condition: CI must run green at `70c9c8df` on `focused:^TestPlatformSettlement`, `^TestPlatformStripe`, `^TestStripeSL02` and `^TestR2IntegrationUpgrade`.

---

## 1. P1-1 (transient mismatch): FIXED

### Trigger
`guard_settlement_line` (0150:191-193) raises PT409 in two cases:
- `OLD.statement_id IS NOT NULL`. So **any** UPDATE after close fails, including a no-op.
- Any column other than `statement_id`, `mismatch` or `fee_store_minor` differs.

The privilege matches: `GRANT UPDATE(statement_id,mismatch,fee_store_minor)` (0150:137). PF09 tries five forbidden changes on an unassigned line and expects PT409 for each: `store_minor`, `settle_fee`/`settle_net`, `kind`, `payload_sha256` and `txn_created_at`.

Only two statements write to `settlement_lines`: the re-check UPDATE (0150:445) and the close UPDATE. Both run under the per-environment advisory lock.

### Re-sync path
- An existing line whose hash is identical is skipped as a duplicate when it already has a statement or has no mismatch (0150:342). Otherwise `v_recheck` is set.
- On a re-check, the lookup in the unattributed table is skipped (0150:346). Attribution and the cross-check then run again exactly as at insert.
- The line must re-attribute to the same tenant, store and attempt, or the sync raises PT409 (0150:442). `attempt_id` is NOT NULL, so the `<>` comparison is never NULL.
- The UPDATE is narrowed by `statement_id IS NULL` and "something changed" (0150:445-446).
- If the re-check no longer attributes, the line is counted as a duplicate and left blocked. No unattributed row is written for a txn that already has a line (0150:459).

### REFUND cross-check
- A REFUND accepts SUCCEEDED, FAILED or CANCELED. A REFUND_FAILURE needs FAILED or CANCELED (0150:417).
- Amount and currency come from `payments.stripe_refunds` (0150:418), with column grants at 0150:173.
- REJECTED is correctly excluded: that refund never reached Stripe, so it has no balance txn. The kinds are defined at 0062:126.
- `stripe_refunds.amount_minor` is set once: the 0062 guard refuses changes, and the stripe_refund_schema test covers it.

### A refund that only ever FAILED nets 0
- REFUND has `store_minor = -amount` and REFUND_FAILURE has `+amount`.
- Close computes `refunded = -Σ(REFUND, REFUND_FAILURE)`, which is 0.
- PF10_recheck asserts `txn_RefundC2 = -800`. PF11 asserts C's net is captured plus fees, with no refund term.

### Store C closes after a late fact
- PF10_recheck first syncs under lag: CAPTURED is hidden and the refund fact is removed, giving 3 × `no_fact`.
- It then restores CAPTURED and adds a FAILED refund fact. The re-sync reports `rechecked=3`, and a third sync is a no-op.
- The PF11 week-1 all-store close then returns A=3, C=4 and P=1 lines.

### Can a re-check move a line that should stay blocked into mapped? No
Only `no_fact` can clear. Every input the other mismatch codes compare is immutable:
- **CHARGE amount/currency.** Compared with `payments.facts`. Its PK is `(tenant,store,attempt,kind)` (0018:85) and it has no UPDATE grant, so the lookup at 0150:409 is deterministic and immutable.
- **REFUND.** Compared with `stripe_refunds`, whose amount is set once.
- **DISPUTE.** Compared with the charge line, which is guarded by the same trigger.
- **Stripe side.** The Stripe values are hash-identical by construction.

A permanent amount mismatch therefore re-computes to `amount`, and the UPDATE matches nothing. This is shown in the test: `txn_ChargeB1` stays `amount` and the w3 all-store close refuses.

A `no_fact` line can move to `amount` or `currency`, which still blocks. That is correct.

## 2. P1-2 (future coverage): FIXED

### Enforcement
- **SQL (authoritative).** `p_window_to > clock_timestamp() - 15 min` is refused with 22023 (0150:298). Fractional bounds on either end are also refused with 22023 (0150:299).
- **Registrar.** `settledWindow(to)` returns ErrRejected (settlement.go:85, :114). The check runs **before** the Stripe list call, so `to` is at most list-start minus 15 minutes. That is the binding constraint, and the later SQL check is a backstop.
- **CLI.** `to.After(now-15m)` returns errUsage (cmd/stripe-admin/settlement.go:83). The unit test covers a window ending in 2099.
- **PF11.** It proves the registrar refuses both a future window and one 5 minutes back. SQL refuses both with 22023, and also refuses a fractional bound. No `settlement_sync_runs` row ends inside the margin.

Go **truncates** fractional seconds rather than refusing them (settlement.go:84, :126). Both bounds are truncated before the list call, which uses `from.Unix()`/`to.Unix()`, and before recording. So the recorded window equals the listed one, and `to` only shrinks. This is safe, and only SQL refuses fractions.

### Any other way to claim coverage?
The only writer of `settlement_sync_runs` is `record_settlement_lines` (0150:472). Nothing else inserts into it: no other function, grant or role path does.

The residual case is by design. Coverage is the registrar's attestation, and SQL cannot prove a Stripe list happened. A registrar caller, or the exported `RecordSettlementLines`, can record a settled past window with `[]` lines. This is within the operator-tool trust model (P2-f).

## 3. Quiet week

- **A store with an earlier statement gets one for the period.** The new UNION branch (0150:543) adds every store with an earlier statement. The `v_count=0 … CONTINUE` skip was removed. PF11 week 2 gives C=0 lines and P=0 lines, each with net 0, and the replay returns 3 statements.
- **A brand-new store with no statement and no lines does not block.** It is in no UNION branch, so it is never visited and `previous_period_open` cannot fire for it. When its first lines arrive, both `previous_period_open` probes (0150:573-577) are false, so it opens cleanly.
- **The carry-forward still applies to an empty statement.** `v_carry` is computed per visited store before and independently of the line sums (0150:572), and `v_net` includes it (0150:588). The statement CHECK keeps net = … + carried_in. Not tested with a **negative** carry into an empty statement: A's w2 net is positive (+3900), so its w3 empty statement carries 0 (P2-d).
- **New consequence (P2-b).** Once the all-store close is blocked by a genuine mismatch, the operator must close per store. Any store with an earlier statement that is skipped in week N then aborts the week N+1 all-store close with `previous_period_open`. The test leaves C and P without a w3 statement, which shows this. It is fail-closed and can be recovered with an explicit `--target-store` close. It needs a runbook entry.

## 4. Rounding

- **Formula.** `-100 · sign(fee·store·settle) · div(2|fee·store| + 100|settle|, 200|settle|)` (0150:238-239). This is floor(|q| + ½) on exact integers. `sign(q)` is the sign of fee·store·settle, because settle is the divisor. It returns 0 when the fee is 0, and `settle = 0` is caught before the call (0150:434).
- **Refunds.** A refund has store < 0 and settle < 0, so the ratio is positive and the share has the fee's sign. Its magnitude equals a charge's with the same |store| and |settle|. A returned (negative) fee at an exact half credits the same magnitude a positive fee debits.
- **Reference test.** The Go `pslFee` reference is exact `big.Rat`: half-up on |q|, then the sign. PF09 adds the negative-fee, negative store/settle and both-negative exact-half vectors for k = 0..5.
- **My check.** The exact-rational reference I wrote, a port of the test reference and the SQL formula agree on every vector (see the local checks above).

## 5. SL02 pin: narrowed

`slsSettlementDelta` (stripe_live_schema_test.go:914, used at :867) now allows only these privileges:

| Role | Allowed |
| --- | --- |
| `commerce_payment_registry_writer` | its own `payments.settlement_*`, record/close/read/guard objects, plus SELECT on exactly 11 existing-table columns: stripe_sessions ×6, stripe_refunds ×4, payment_attempts.order_id |
| `commerce_payment_registrar` | EXECUTE on the 4 operator functions only |
| `commerce_runtime` | EXECUTE on `read_store_settlements` only |

- `commerce_checkout_writer` gets nothing. The old `slsSettlementObject` wildcard was removed from `slsPlatformStripeDelta`.
- I found no other widening. The only `payments.stripe_refunds` prefix acceptance left is the pre-existing 0077 canary read for the registry writer.
- PF09 also pins the column sets from the 0150 side. stripe_refunds has 9 columns: 0077's 5 plus 0150's 4 (0077:186-187).

## 6. `record_settlement_lines` refuses a non-platform connection

`p_connection <> sp.connection_id` raises PT409 `not_platform_connection` (0150:306). PF10_platform_connection_assertion covers it with X's connection.

- `SettlementSync` always passes the connection, because `command.ValidID` makes it mandatory.
- SQL still accepts `p_connection IS NULL`. Only test callers of the exported `RecordSettlementLines` use that (P2-c).

## 7. Applied review rulings

- **Ticket in the audit details.**
  - It goes into the details on sync (0150:487), close (:605) and payout (:645).
  - The validation regex is the same in SQL, Go `refPattern` (registrar.go:49) and the CLI (`ticketPattern`).
  - PF12_ticket_in_audit checks all three actions.
- **Payment-intent index is UNIQUE** (0150:119).
  - The table and column existed before 0150: `payments.stripe_sessions.payment_intent_id`, 0061:301.
  - Duplicates in an existing DB are not credible:
    - `attempt_id` is the PK and `session_id` is UNIQUE (0061:299).
    - The PI is set once (the guard at 0061:577 and the pin in post_river/0012:470-486).
    - Stripe issues one PI per Checkout Session.
    - The fake Stripe server derives `pi_test_`+session_id (stripetest/server.go:248), so its PIs inherit session_id uniqueness.
    - The only direct test writer (`slrEnv.pin`) uses random tags.
  - SL02 "upgrade of a populated database" passed in the author's green run.
  - Recommend a one-line duplicate preflight before applying to any populated DB (P2-g).
- **`window_net` no longer double counts.** An unattributed row that has since become a line is excluded (0150:483). That matches how close treats unmapped rows.
- **Signatures and the registrar ABI.**
  - All three changed signatures agree everywhere: in ALTER OWNER, REVOKE and GRANT (0150:704-718), in `stripeRegistrarFunctions` (internal/platform/stripe_runtime.go:126-128) and in PF09.
  - `go build` passes, and no other Go or SQL caller uses the old signatures.
  - The contract text is stale (P2-e).

## 8. Mutation evidence: each of the 5 goes red for the right reason

| Mutation | First failing assertion |
| --- | --- |
| carry | :694 `carried=0(-6800)`. Single fault only, so the round-1 P2-11 is resolved. |
| recheck | :556 `Rechecked:0 Duplicate:4`, then close blocks with `settlement_mismatch` |
| future | :602 `SQL accepted a window ending at …`. The Go layers stayed intact, so this isolates the SQL guard. |
| quiet | :681 the w2 all-store close returns A only |
| terminal | :556 `Rechecked:2 Duplicate:2`. The REFUND stays `no_fact` with only a FAILED fact. |

- Later subtests also fail in each run, but those are knock-on failures from shared state. In every run the first failure is the targeted one.
- `red-fix.log` (unfixed code plus new tests) fails PF09 (missing amount/currency grants), PF10_recheck, PF11 (future window) and the dependents.

## 9. No regression

- **R2 pin.** It is 77 (r2_integration_upgrade_test.go:60-65). Trunk was at 76 with 0146, 0149 and 0151 (`r3/integration` pin "want 76"), and 0150 makes 77. The migrations directory holds 0144, 0146-0151.
- **0150 vs 0149.** No shared object: 0149 touches no payments, platform or registrar object.
- **0150 vs 0151.** No shared object either. 0151 touches only `ops.audit_events`, and only by adding INSERT policies for `commerce_integration_writer`. 0150 writes audit rows as `commerce_payment_registry_writer`, under existing policies.
- **0150 vs 0146.** 0146 only inserts audit rows.
- **No `CREATE OR REPLACE` in 0150.**
- **Not evidenced after the merge.** No log shows R2 = 77 passing at `70c9c8df`, so CI must run it (P2-a).

---

## P2 (none blocks the merge)

- **a. Evidence binding.** The fix-round logs ran on "123dd0d9 + uncommitted fix", not on a commit. No PG run exists at `70c9c8df`, which is post-merge with R2 = 77. Fix: CI at `70c9c8df` on PlatformSettlement, PlatformStripe, SL02 and R2.
- **b. A genuine mismatch has no resolution path** (`amount`/`currency`, or a `no_fact` whose fact never comes). It blocks every all-store close in the environment for good (0150:527-530). The fallback to per-store closes then breaks the quiet-week chain (§3). Fix: put it on the S2-OPEN-1 LIVE checklist with the `settlement-resolve` step and a runbook entry.
- **c. `p_connection` is optional in SQL** (0150:306). Fix: make it required, since the only production caller always passes it.
- **d. No test of a negative carry into an empty statement.** The code path is correct by reading.
- **e. The contract is out of date.** contracts/stripe-platform-account-v1.md:388-390 still lists the old signatures, and line 388 still has REFUND checked against `refund_facts` SUCCEEDED. Fix: the integrator amends §6.1, §6.2, §6.4 and §6.5, as already ruled.
- **f. Coverage is an attestation.** The exported `RecordSettlementLines` or a direct registrar SQL call can record a settled past window without listing Stripe. Fix: document the trust model, or make `RecordSettlementLines` unexported.
- **g. Preflight before the UNIQUE index.** Before applying 0150 to a populated DB, run `SELECT environment,payment_intent_id,count(*) FROM payments.stripe_sessions WHERE payment_intent_id IS NOT NULL GROUP BY 1,2 HAVING count(*)>1`. The expected result is 0 rows.
- **h. A misleading test comment.** It calls B's REFUND_FAILURE `no_fact` "NOT transient", but that line is recheckable once a FAILED fact arrives. Cosmetic.
- **i. The connection assertion runs after the Stripe read.** It is read-only, and `storedCredential` is already scoped to the platform store. Trivial.
