# fix-untracked-stripe-capture delivery

- Branch: `unit/fix-untracked-stripe-capture`; base `e705f70a694f7d0f4ed1c2967883e3f5d8d62803`.
- Product commit: `79a5ff6ed70a0ce6ec365c65941e37a182164175`; stronger cross-store test: `50a407a9a3131e551e93da5c4b09bf38b101b3cd`. This delivery-only commit changes no tested executable source.
- Author: Codex-4, GPT-6 family; exact runtime model/effort not exposed. Worktree `.worktrees/fix-untracked-stripe-capture`. Scope: new migration + foundation regression/pins + this evidence. No push.

## Change

`migrations/0167_stripe_untracked_reservation.sql` replaces only the two zero-reservation-loop guards in `payments.apply_stripe_observation` (latest previous body 0062). An empty plan qualifies only with a nonempty order quote array, positive quantities, every SKU resolving in the authenticated tenant/store with `inventory_tracked=false`, and no prior RESERVE ledger for the order/reservation. Real untracked Begin orders now record CAPTURED or CLOSED_UNPAID and finish the normal order/work-item transitions without inventory rows. Existing nonempty sorted locking/allocation/release paths stay byte-identical.

The RESERVE-history condition prevents a tracked-to-untracked edit from excusing deleted stock lines. No new SQL function/signature or table/column/permission. SECURITY DEFINER, `search_path=pg_catalog`, owner `commerce_checkout_writer`, owner-only EXECUTE ACL and the existing function COMMENT are preserved. SELECT and RLS for catalog/ledger already exist in 0013; no grant expansion.

RF12 restores exactly the two new proof blocks to the old guard and compares the resulting body byte-for-byte against 0062, then retains the original 0061→0062 refund-review assertion. R2 pin 91→92; CRP02 holds 0167 back for the populated second-stage upgrade and checks its checksum. KC03/MCI02 later-migration holdbacks are discovered dynamically, not hard-coded.

## Red → green and gates

Exact commands and results are machine-readable in `results.json`; each raw log is retained losslessly as `.log.gz`, with decompressed hashes in `logs.json`. Canonical uncompressed evidence: `/Volumes/data/live_commerce_architecture_v1/output/fix-untracked-stripe-capture/`. Current five source hashes: `source-hashes.json`.

| Command | Exit | Evidence/result |
|---|---:|---|
| `bash scripts/dev/test-focused.sh '^TestStripeUntracked'` (before migration) | 1 | `red-valid.log`: all-untracked capture AND close fail with PT409; mixed controls and all refusal cases pass. Earlier `red.log` also records a foreign-fixture permission error, corrected before this valid red. |
| `bash scripts/dev/test-focused.sh '^TestStripeUntracked\|^TestStripeRF12Guards\|^TestStripeSP06'` | 0 | `green-focused.log`: 6 top-level PASS, 0 FAIL/SKIP. |
| `bash scripts/dev/test-focused.sh '^TestR2IntegrationUpgradeFromReleaseHead$\|^TestLiveClaimsKC03\|^TestClaimsRetentionCRP02'` | 0 | `upgrade.log`: 3 top-level PASS, 0 FAIL/SKIP. |
| `LC_FOCUSED_TIMEOUT=1800s bash scripts/dev/test-focused.sh '^TestStripe(SP\|SL\|RF\|Untracked)'` | 0 | `stripe-regression.log`: 39 top-level PASS, 0 FAIL, 2 top-level SKIP; 551.411s at 79a5ff6e. Nested NOT_RUN cases below. |
| `bash scripts/dev/test-focused.sh '^TestStripeUntracked'` (final stronger foreign-untracked fixture) | 0 | `green-scope-control.log`: 2 top-level PASS, all 14 subcases PASS. Only test fixture changed after broad regression; production migration unchanged. |
| `GOTOOLCHAIN=go1.27.1 go test -race -p 1 -count=1 -timeout=600s -run '^TestStripe(SP\|SL\|RF)\|Refund' -v ./internal/integrations/psp/stripe/... ./internal/payments/... ./internal/merchantorders` | 0 | `stripe-package-gates.log`: 37 top-level PASS, 1 SKIP (SP16); settlement/stripeadmin/stripewebhook have no matching package tests and are NOT claimed covered by this command. Foundation has registrar/ingress coverage separately. |
| `pnpm install --offline --frozen-lockfile` | 0 | `install.log`, no lockfile change. |
| `bash scripts/dev/test-node.sh` | 0 | `node.log`, 1,137 tests PASS, 0 FAIL. |
| `pnpm --filter @live-commerce/admin typecheck` | 0 | `typecheck-admin.log`. No apps changed. |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates-final.log`: 82 modes, all documented; every tracked test assigned; header check PASS. |

Evidence level: **E3 within tested REAL_PG + MOCK scope**. Real catalog creation, checkout Begin and payment initiation; synthetic provider pin/observation owner fixtures are disclosed in test logs. Reconciliation runs through the actual `payments.apply_capture` dispatcher under the payment-worker role. Refusal cases prove zero payment facts, terminal inventory writes and work items; repeated successful observations yield one terminal fact/event. Cases include all-untracked, mixed tracked/untracked, deleted tracked lines, empty quote, foreign-store untracked SKU, missing reservation aggregate and tracked→untracked with deleted lines.

## Known limit / pending ruling

**Untracked at Begin → tracked before capture/close remains fail-closed PT409.** Inventory mode is mutable and absent from the frozen quote/order, so current catalog state cannot prove that historical empty plan. This is a real uncovered payment-recording scenario, not a passing test or a resolved finding. An async scope question asked whether to retain this minimal scope or add a separately frozen historical tracking proof. No permission to widen Begin/schema was assumed. Do not present this patch as solving all inventory-mode changes.

Independent read-only review of 79a5ff6e found no additional P0/P1 within the minimal scope, independently restored the 0062 body and verified the original red log/current hashes (E1 source review; no independent PG run). Humaux review `8a098950-96a1-4aa4-b09b-ed9e76952e42`. It also retains the mode-change concern above. K3 money review remains required.

## CI gates / NOT_RUN

- Required: full foundation-shards and **`bash scripts/dev/release-gate.sh --strict --only G07`**, plus independent K3 money review. G07 is mandatory for this migration; full local foundation was NOT_RUN under AGENT-PREAMBLE's Mac RAM rule. Local focused passes do not clear it.
- SANDBOX NOT_RUN: SP16, SL08, RF10, SP21 real sandbox probe (no owner test credentials/opt-in used).
- Existing nested RF03 `populated_upgrade_from_0061` skipped because its harness lacks a partial-apply hook; not claimed tested. R2, KC03 and CRP02 upgrade gates above actually ran.
- Browser, real Stripe capture/refund, physical devices, LIVE, production migration/deployment: NOT_RUN. No provider API/SDK change, no real charge, no live keys or customer messages.
- Integrator: confirm 0167 remains free at merge, adjudicate the historical tracking-switch limit, run K3 + full G07, then push/review. No edits to any merged migration.

## Cleanup

All author-started test processes finished; disposable PG cleanup used the existing script traps. No shared cache, other task directory or production data was removed. Git baseline is e705f70a; production rollback was not attempted or tested.
