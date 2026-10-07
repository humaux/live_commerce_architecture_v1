<!-- Purpose: unit brief for S2-OPEN-1, the operator step that resolves an unmapped_source settlement row so period close can proceed.
Depends on: contracts/stripe-platform-account-v1.md §6 (settlement), migrations/0150_platform_settlement.sql, cmd/stripe-admin/settlement.go,
  internal/payments/stripeadmin, tests/foundation/platform_settlement_test.go, output/w4-s2-platform-settlement/DELIVERY.md (S2-OPEN-1).
Used by: the backend implementer (Aliyun Qwen), the integrator and the Opus money review. -->
# Unit brief — settlement-resolve (S2-OPEN-1, before LIVE)

## Problem
Any charge on the platform Stripe account that this system did not create (Dashboard charge, payment link, an owner test, or a
payment intent shared by two sessions) is recorded as a `payments.settlement_unattributed` row with reason `unmapped_source`.
`settlement-close` (0150 ~line 530) refuses the WHOLE environment's close while any such row sits in the window
(`PT409 settlement_unattributed`), and there is no tool to clear it. The W4-S2 test deletes the row with an owner-pool DELETE
only to continue. One stray Dashboard charge would therefore freeze every store's statement.

## Scope (backend only)
1. Migration **0163** (forward-only, one transaction):
   - an append-only `payments.settlement_unattributed_resolutions` table: the `balance_txn_id` (FK to the unattributed row, UNIQUE —
     one resolution per row), `environment`, a `resolution` CHECK IN (`not_store_revenue`, `assigned_to_store`), `target_tenant_id` / `target_store_id`
     (required iff `assigned_to_store`, NULL otherwise; the store must exist and must have used the platform account), `operator`, `ticket`
     (non-empty), `note` (bounded text), `resolved_at`. FORCE RLS like the 0150 tables; no UPDATE/DELETE grants to anyone.
   - one SECURITY DEFINER function (search_path=pg_catalog, owner and EXECUTE grants in the 0150 operator style) that records a resolution:
     refuses rows whose reason is not `unmapped_source`, already-resolved rows (idempotent replay with the same payload returns the
     existing row; a different payload → `PT409`), rows in a period that is already closed for any store, and unknown target stores.
   - the close check (0150 ~line 530) ignores `unmapped_source` rows that have a resolution. Patch it in place with the house pattern
     (`pg_get_functiondef` + replace + EXECUTE, with an exactly-once anchor RAISE). Do NOT CREATE OR REPLACE the whole close function.
   - `assigned_to_store`: v1 does NOT move money into a store statement. It is recorded and printed by close/export as an operator note
     for manual handling. Write this limit into the contract amendment.
2. `cmd/stripe-admin`: a new `settlement-resolve --environment --balance-txn --resolution [--target-tenant --target-store] --note`
   subcommand plus the existing `--operator --ticket` rules, an audit row in the same transaction as the other subcommands, and one JSON
   result line. Add it to the `deploy/scripts/ops-admin.sh` allowlist. It never calls Stripe and never reads the LIVE pair.
3. Contract amendment (interface first): add a §6.x "Resolving unmapped rows" to `contracts/stripe-platform-account-v1.md`, and add the
   step to the §8 LIVE checklist. Update the 0150 close-refusal text so it names the resolve step.
4. Pins: R2 migration count (`tests/foundation/r2_integration_upgrade_test.go`) +1, and any function-count or privilege pins that the
   new function or table touches (grep `functions !=`, `has_table_privilege`, the SL/settlement pins in platform_settlement_test.go).

## Acceptance (red → green, REAL_PG, focused regexes only on this machine)
- RED first: a test that seeds an `unmapped_source` row and shows that close refuses and no resolution path exists.
- GREEN:
  - resolve `not_store_revenue`, after which close succeeds and the statement totals are unchanged;
  - an idempotent replay returns the same row;
  - a different payload on a resolved row → 409;
  - resolving a `foreign_connection` or `unsupported_type` row is refused;
  - `assigned_to_store` without a target, or with an unknown or foreign store, is refused;
  - a row inside an already-closed period is refused;
  - the runtime and merchant roles cannot read or write the resolutions table (privilege pin);
  - the CLI subcommand is end to end against PG, with its JSON line and audit row;
  - the W4-S2 test no longer needs the owner-pool DELETE and uses the resolve path instead.
- Gates: `go vet ./...` and `go vet -tags browser ./tests/foundation`, `bash scripts/dev/check-gates.sh`, and
  `bash scripts/dev/test-focused.sh '^(TestPlatformSettlement|TestR2IntegrationUpgradeFromReleaseHead|TestStripeAdmin|TestOpsAdmin)'`.
  List the heavy CI gates in DELIVERY.md (foundation-shards, deploy-smoke).

## Integrator 裁决
- A resolution never deletes or edits the unattributed row: append-only, auditable.
- v1 has no money movement into store statements. `assigned_to_store` is a recorded note only.
- Evidence class: REAL_PG + MOCK. No Stripe calls.
