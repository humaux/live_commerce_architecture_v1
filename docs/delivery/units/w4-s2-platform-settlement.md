# Unit W4-S2: per-store settlement ledger, operator settlement CLI, merchant read-only view (backend, money path)

**Status.** DRAFT, written 2026-10-06 by a Claude Opus subagent. It waits for integrator review and freeze.
- Base: `r3/integration` `2fa471a1`.
- Migration placeholder: **0138**, which is free because W4-03B is cancelled.
- Covers [stripe-platform-account-v1](../../../contracts/stripe-platform-account-v1.md) §4.4 and §6.
- Worktree `.worktrees/w4-s2-platform-settlement`, branch `unit/w4-s2-platform-settlement`.

**Read first:** PREAMBLE → AGENTS.md → PROCESS.md → this file. Then read contract §0, §4.4 and §6 of
stripe-platform-account-v1, and these sections only: stripe-live-enable §5.2 (`ops-admin.sh` rules) and stripe-psp
§5.6 (response classification).

## Integrator 裁决 (overrides the body)

**Owner decision (2026-10-06):** 「不使用PAYUNi，直接允许商家使用平台的stripe进行收款」. The platform collects every
store's card money and settles with each store off-Stripe, from this ledger.

**Rulings.**
- Ledger money comes **only** from Stripe balance-transaction reads, cross-checked against existing worker facts. No
  input tenant or store, no metadata, no webhook payload.
- Attribution happens in SQL from Stripe ids (contract §4.4). An unmapped transaction is never guessed: it goes to
  `settlement_unattributed`, and close blocks while an unmapped charge exists.
- **PF-F5 is UNVERIFIED.** Before any code, WebFetch `https://docs.stripe.com/api/balance_transactions/object` and the
  list page. Record the exact field names, the `type`/`reporting_category` values for charge, refund,
  refund_failure, dispute and dispute reversal, and the `source` expansion shape in DELIVERY with the date. If a field
  differs from the contract, write it in DELIVERY and **stop for an integrator ruling**. Do not adapt silently.
- A payout is only **recorded** (operator-entered bank reference). This unit never calls a bank or a Stripe payout or
  transfer API. A real bank transfer is the owner's red-line action.
- Evidence ceiling: MOCK (fixture balance list) + REAL_PG. SANDBOX sync needs the owner's test key, otherwise NOT_RUN.
  Never LIVE.

**Open owner questions.** Defaults are in contract §11. Implement the defaults as data, not as code branches.

| # | Question | Default |
| --- | --- | --- |
| OQ-2 | Settlement cycle | weekly, Monday to Monday Asia/Taipei, closable after +72 h |
| OQ-3 | Platform fee % | `platform_fee_bps=0` while `LC_BILLING_ENABLED` is off |
| OQ-3 | Stripe-fee, FX and transfer-cost bearer | Stripe fees passed through at the charge's ratio; FX and bank fees borne by the platform; no reserve |
| OQ-6 | Payout setup and bank route | — |
| OQ-5 | Taiwan e-invoice responsibility | — (blocks LIVE opening, not this unit) |
| OQ-7 | Dispute handling | — |

## Key facts (base `2fa471a1`; `grep -n` again before implementing)

- **Attribution keys.**
  - `payments.stripe_sessions.payment_intent_id` (0061:304);
  - `payments.stripe_refunds` has the Stripe refund id (0062, latest body);
  - `payments.facts` holds CAPTURED amounts per attempt;
  - `payments.refund_facts` holds SUCCEEDED amounts.
- **Derived connections.** `integration.merchant_accounts.platform_connection_id` and `payments.stripe_platform` come
  from W4-S1 (0137).
- **Stored-credential CLI pattern.** `live-approve` unseals the stored key through `payments.stripe_registrar_credential`
  (`cmd/stripe-admin/live.go`, `internal/payments/stripeadmin/live.go`).
- **CSV helpers.** `internal/merchantorders/export.go`: `writeCSVLine`, `guardFormula`:90.
- **Wire package.** `internal/integrations/psp/stripe/` (`client.go` request and classification, `strictjson.go`).

## Scope

1. **SQL 0138**, contract §6.1 and §6.4:
   - tables `settlement_lines`, `settlement_unattributed`, `settlement_statements`;
   - functions `record_settlement_lines`, `close_settlement`, `record_settlement_payout`, `read_store_settlements`.
2. **Wire.** `stripe.Client.ListBalanceTransactions`:
   - exact projection (contract §6.5), strict JSON;
   - ≤ 50 pages and ≤ 8-day window, otherwise `ErrUncertain` and nothing returned;
   - expanded source parsed only for the listed ids.
3. **CLI.** `stripe-admin settlement-sync | settlement-close | settlement-export | settlement-payout`, with
   `--operator`/`--ticket`. The `ops-admin.sh` allowlist gains these four. None takes a secret. LIVE sync requires
   the flag+ref pair. Close, export and payout do not call Stripe.
   - Export: `--out` is an absolute path, the file must not exist, mode 0600, UTF-8 BOM, `guardFormula`.
   - Columns: `period_start, period_end, order_number, kind, amount, stripe_fee, txn_date`, then a totals block.
   - **No buyer PII column.**
4. **Merchant HTTP.**
   - `GET /v1/admin/stores/{store_id}/settlements?before=YYYY-MM-DD&limit≤52`;
   - `GET /v1/admin/stores/{store_id}/settlements/{statement_id}`;
   - both call `read_store_settlements`; `billing:manage` is required.

## Non-goals

- automated payouts, bank APIs, Stripe Connect transfers;
- tax and e-invoice;
- dispute evidence workflow;
- a scheduled sync job (the operator runs it weekly; add a cron line to the runbook);
- mismatch-resolution tooling (escalate);
- multi-currency stores;
- the UI (W4-U1).

## SQL authority (SECURITY DEFINER shape)

- **Operator functions:**
  - owner `commerce_payment_registry_writer`, `SECURITY DEFINER`, `SET search_path=pg_catalog`;
  - `require_stripe_registrar_scope` on the **platform** store;
  - per line or statement, `set_config('app.tenant_id'|'app.store_id', <attributed scope>, true)` before writing;
  - EXECUTE `commerce_payment_registrar` only.
- **Merchant reader:** owner `commerce_payment_registry_writer`, `identity.resolve_access(token, store,
  'billing:manage')`, EXECUTE `commerce_runtime` only.
- **All tables:** FORCE RLS, PUBLIC revoked, no DELETE, and no privilege for workers, ingress or the checkout/integration
  writers.
- **Set-once triggers:** `statement_id` on lines and the payout triple on statements.

**ACL pins:**
- `tests/foundation/stripe_authority_test.go` (new functions);
- `tests/foundation/worker_authority_split_test.go` WAS02 (no worker EXECUTE);
- `tests/foundation/merchant_orders_v2_acl_test.go` (runtime EXECUTE list gains `read_store_settlements`).

## Write paths (single owner)

**Backend (Claude Sonnet):**
- `migrations/0138_platform_settlement.sql`;
- `internal/integrations/psp/stripe/balance.go` (new) + `balance_test.go`;
- `internal/payments/settlement/` (new: sync mapping, close and export, CSV);
- `internal/httpapi/settlements.go` (new);
- `cmd/stripe-admin/settlement.go` (new);
- `tests/foundation/platform_settlement_test.go` (author smoke).

**Integrator hooks:** `internal/httpapi/handler.go` (one line), `cmd/stripe-admin/main.go` (dispatch),
`deploy/scripts/ops-admin.sh` (allowlist), `docs/runbooks/` weekly settlement steps, GATES.md, ACL pins.

**Independent tests (Kimi K3):** `tests/foundation/platform_settlement_gate_test.go` (PF09–PF13).

## Tests (red first, then green; save the red run to `output/w4-s2-platform-settlement/red.log`)

| # | Case | Expected |
| --- | --- | --- |
| PF09 | schema | net identity CHECK; payout CHECK; `settle_amount − settle_fee = settle_net`; set-once triggers; FORCE RLS; ACL |
| PF10 | sync from a fixture list: two stores' charges, a refund, a refund_failure, a dispute and its reversal, a payout, a `stripe_fee` txn, an unknown `source`, a charge of an unrelated primary account | lines in the right store scopes; payout and `stripe_fee` → unattributed `unsupported_type`; unknown source → `unmapped_source`; unrelated account → `foreign_connection`; a replay inserts 0; changed content for the same txn id → PT409; CAPTURED amount ≠ line → `mismatch='amount'`; missing `exchange_rate`/`fee`/`source` → fail-closed row; page 51 → `ErrUncertain`, 0 rows written |
| PF11 | close | totals and §6.3 vectors: TWD 100000 minor charge, HKD 25,640 amount, fee 1,046 → `fee_store_minor = −4100`; half-up at exactly .5; dispute uses the charge's ratio. Also: period not yet +72 h → refused; previous period open → refused; a late line rolls into the next statement; a mismatch or an unmapped charge blocks; a negative net carries forward; bps 0 and 500; replaying close returns the same statement |
| PF12 | payout | set once; amount ≠ net refused; net ≤ 0 refused; `paid_at` in the future refused; identical replay returns the stored time |
| PF13 | merchant read and CSV | store A cannot read B (not found); `billing:manage` required; no settlement-currency fields, txn ids of other stores or unattributed rows; CSV header has no PII columns; `=cmd` cells guarded; the file is 0600 and refuses to overwrite |

Write the PF11 fee vector in DELIVERY with its arithmetic before you code it.
- Arithmetic: 1046 × 100000 / 25640 = 4079.56 → /100 = 40.80 → round half-up = 41 → ×100 = 4100.
- Then confirm the SQL `numeric` result equals the Go result.

## Gates

**Local, focused:**
- `bash scripts/dev/test-focused.sh '^TestPlatformSettlement'`
- `go test ./internal/integrations/psp/stripe/... ./internal/payments/settlement/... ./cmd/stripe-admin/...`
- `bash scripts/dev/check-gates.sh`

**CI gates:** the full foundation suite, the `TestPlatformStripe` regression, and `release-gate.sh --strict`.

## Evidence / role / dependencies

- **Evidence:** MOCK + REAL_PG. SANDBOX sync (two PF-SBX stores) runs only with the owner's test key, otherwise
  NOT_RUN. Never LIVE.
- **Role:** backend = **Claude Sonnet** (DeepSeek unavailable). K3 independent gate. Claude Opus money review.
- **Dependencies:** **W4-S1 merged** (0137 derived connections and `stripe_platform`). Can run in parallel with any
  unit that does not touch `stripe-admin` or the refund tables.

## OPEN (with recommendations)

- **S2-OPEN-1: unmapped-charge resolution.** **Recommendation:** escalate to the integrator. A manual-attribution CLI
  waits until the first real case.
- **S2-OPEN-2: scheduled sync.** **Recommendation:** an operator cron in the runbook. Add a worker job only if the weekly
  manual run is missed.
