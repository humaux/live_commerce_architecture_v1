# PAY-RM1 — remove the never-deployed W4-01B PAYUNi notify receiver (backend clean-up)

- **Branch:** `unit/pay-rm1-remove-payuni` (worktree `.worktrees/pay-rm1-remove-payuni`)
- **Base:** `86e404a4` (= tip of `r3/integration`; fast-forward, no merge needed)
- **Commits:** RED `1c4eb804`; GREEN `c6015862` (0161 + migrate.go + shims), `55f1d1fd` (Go tree), `e8b2b680` (foundation tests), `fb1bfdc6` (docs/contract), `4963e2fc` (depmap), `05f364db` (gofmt); delivery commit = this file.
- **Role/model:** backend implementer, "Aliyun Qwen" (runtime reports model `qwen3.8-max`); reasoning effort not disclosed by runtime. Orchestrated via Claude Code.
- **Allowed write paths used:** `migrations/`, `cmd/api/`, `internal/`, `tests/foundation/`, `contracts/`, `docs/`, `output/pay-rm1-remove-payuni/`. Not touched: `go.mod/go.sum`, pnpm lockfiles, OpenAPI/shared schema, other migrations, other tasks' `output/` dirs.

## Summary (scope per Integrator 裁决: remove ONLY W4-01B = merge 651c5744 / migration 0136; migration number overridden to 0161)

**New forward-only migration `migrations/0161_remove_payuni_notify.sql`:**
- Guard (`:14-23`): counts `payments.payuni_notify_receipts` rows and `review_cases reason='NOTIFY_MISMATCH'` rows; RAISEs `'PAYUNi notify removal refused: …'` with `ERRCODE='22023'` if either > 0. Runs before any DDL inside the single numbered-phase transaction → a refusal leaves the DB byte-identical (RM02 pins table/constraint/ledger survival).
- `review_cases` restore (`:29-45`): drop `payuni_notify_review` policy; rebuild `review_cases_reason_check` by inverse regexp from the **live** `pg_get_constraintdef` (shape-verified before and after; refuses on drift instead of guessing); drop `review_cases_report_hash_scope`; `source_report_hash SET NOT NULL` (safe: guard proved no NULL-hash rows).
- Drops (`:49-54`): tables `payuni_notify_receipts`, `payuni_notify_endpoints` (policies/trigger/grants cascade); functions `payuni_record_notify`, `payuni_resolve_endpoint`, `set_payuni_notify_endpoint`, `guard_payuni_notify_receipt` (full signatures).
- Role (`:59-60`): `REVOKE USAGE ON SCHEMA payments FROM commerce_payuni_ingress; DROP ROLE commerce_payuni_ingress;` (owns nothing → no REASSIGN).
- `0136` never edited. River wake-grant REVOKE deliberately NOT in 0161: `river_payment` doesn't exist yet during the numbered phase on a fresh DB.

**`migrations/migrate.go` (~204-212):** post-River `GRANT UPDATE(scheduled_at) ON river_payment.river_job TO commerce_integration_writer` → `REVOKE … FROM …`, re-executed every Apply so upgraded DBs converge to the fresh ACL. No-op where never granted.

**Go removals (exact revert of 651c5744's Go surface):**
- Deleted whole files: `cmd/api/payuni_notify.go`, `cmd/api/payuni_notify_test.go`, `internal/payments/payuninotify/` (doc/handler/handler_test/inbox), `internal/platform/payuni_runtime.go`, `internal/integrations/accounts/payuni_crypto.go`.
- `cmd/api/main.go`: dropped `loadPayuniNotifyConfig` block (was ~:60), `buildPayuniNotifyHandler` + `defer closePayuniNotify` (was ~:108), `mountPayuniNotify` mount (was :211).
- `internal/platform/platform.go`: dropped `payuniIngress*` vars, the 3 `pg_has_role(commerce_payuni_ingress)` SELECT columns, 3 Scan args, `"payuni_ingress"` membership entry, the `roleValid` branch and the `validatePayuniAuthority` branch.
- `internal/platform/stripe_runtime.go`: dropped `payments.set_payuni_notify_endpoint(…)` from the registrar pin list (was :115-117).
- `internal/integrations/psp/payuni/client.go`: reverted `notifyOnly` field/param, `NewNotify`, `NotificationAuth`, `AuthenticateNotification`, `authenticateNotification`; `VerifyNotification` inlined again — verified hunk-by-hunk against `git diff 651c5744^1 651c5744`.
- `internal/integrations/psp/payuni/client_test.go`: removed `TestNotifyOnlyClientAuthenticatesButCannotBuildOrQuery`, `TestAuthenticateNotificationBoundedAndRejectsUntrusted`; kept `notification()` (used by kept `TestNotificationAuthenticationAndBinding`).
- `internal/integrations/accounts/crypto_test.go`: removed `TestSealPayuniAndOpenNotifyRoundTrip`.

**Foundation tests:**
- Deleted: `payuni_notify_test.go`, `payuni_notify_authority_test.go`, `k3_w4_01b_adversarial_test.go` (all pinned the removed receiver; superseded by RM01/RM02).
- `worker_authority_split_test.go`: WAS02 notify-definer EXECUTE loop removed (definers gone; RM01 pins absence).
- `hosted_payment_authority_test.go`: W4-01B notify-route NotifyURL pin + unused `strings` import removed.
- `hosted_payment_test.go:99`: hpSetup config back to pre-W4-01B `…/payment/return` + `…/payment/notify`.
- `meta_claims_intake_schema_test.go` (~:221) and `stripe_live_schema_test.go` (~:670): removed the mirrored W4-01B grant shim line (REVOKE is ACL-neutral, so shim lists stay exact).

**New gates (RED first, committed in `1c4eb804`):**
- `tests/foundation/payuni_removal_test.go` — RM01: pre-0136 DB (skip-ledger) vs fresh DB with 0161 → identical `review_cases_reason_check`, all W4-01B objects absent, wake grant absent, stripe `UPDATE(kind)` control grant present, old path (`apply_capture_payuni_v1`, `integration.merchant_accounts`, `commerce_payment_registrar`) present. RM02: 0160-state DB + one seeded receipt row, then one seeded `NOTIFY_MISMATCH` review case → 0161 must abort with `'PAYUNi notify removal refused'` + `22023`, nothing dropped, ledger unmarked.
- `cmd/api/payuni_removed_test.go` — RM03 (DB-free): full mount chain; POST `/v1/hooks/payuni/notify/<43×A>`, `/v1/hooks/payuni`, `/v1/hooks/payunix` → 404; stripe/meta webhook controls still route.

**Pin updates (all honestly re-run, see below):** `r2_integration_upgrade_test.go:73-76` count **86 → 87** (0161 adds one file); stripe registrar function-list pin −1; MCI02/SL02 shim privilege lists −1 grant; WAS02 −1 block; hosted authority −1 pin block.

**Docs/contract:** `contracts/payuni-wire-v1.md` — appended "Amendment: W4-01B notify receiver removed (PAY-RM1, migration 0161)" (pre-wire-package surface restored; unresolved background-transport/ACK warning stands; old path explicitly out of scope). `docs/delivery/units/w4-01b-payuni-notify.md:3` — status → REMOVED with 0161 semantics. `docs/engineering/dependency-map.md` regenerated (`depmap.sh --check` → "up to date").

## Deliberately NOT changed (documented deviations from the brief's removal list)

- `cmd/api/stripe_sp15_test.go:233` ("payuni_notify_absent"): re-derivation shows it pins `COMMERCE_PAYMENT_NOTIFY_URL` (old buyer-path env), not W4-01B — kept.
- `GATES.md` / `scripts/dev/test-local.sh`: no W4-01B rows/modes exist (verified during RED; only `payuni-ui-baseline.mjs` old-path reference at GATES.md:173) — nothing to remove.
- `docs/delivery/units/INDEX-w3-w6.md`, `w4-02b-payuni-activation.md`, `pay-remove-payuni.md`: historical/brief documents — kept.
- `output/w4-01b-payuni-notify/`, `output/k3-w4-01b/`: other tasks' evidence — forbidden to delete, untouched.
- `migrations/0136_payuni_notify.sql`: forward-only rule — never edited.
- Old PAYUNi hosted/query/capture path (0014–0018, `internal/integrations/psp/payuni` New/NewQuery/VerifyNotification, `payments.apply_capture_payuni_v1`): OPEN-1, out of scope, RM01 actively pins its survival.

## Tests + exit codes (unfiltered, final committed tree; logs in this directory)

| Command | Exit | Evidence |
|---|---|---|
| `go build ./...` | 0 | `gate-go-build.log` (empty = clean) |
| `go vet ./...` | 0 | `gate-go-vet.log` (empty = clean) |
| `go vet -tags browser ./tests/foundation` | 0 | `gate-go-vet-browser.log` (empty = clean) |
| `go test ./internal/... ./cmd/...` (no -run filter) | 0 | `gate-go-test-unit.log` — 80 ok, 0 FAIL |
| `bash scripts/dev/check-gates.sh` | **1 first run** (gofmt wanted the 2 new test files) → fixed via `gofmt -w` + commit `05f364db` → **0** on re-run | `gate-check-gates.log` ("check-gates: ok (73 modes…)", "check-headers: OK") |
| `bash scripts/dev/depmap.sh --check` | 0 ("depmap: up to date") | inline |

**RED evidence (`red.log`, trunk 86e404a4):** RM03 FAIL (notify POST = 200, want 404, routed=map[payuni:1]); RM01 FAIL (all W4-01B objects survive; captured exact pre-0136 vs current constraint defs); RM02 FAIL (`expected exactly one 0161_* removal migration, got []`); R2 FAIL (`R2 migration set = 86 files, want 87`).

**GREEN PG pins** — all via `bash scripts/dev/test-focused.sh '<regex>'` (machine-wide lock, disposable pinned-digest PG 18.6 containers, `-race`), each exit 0; combined summaries in `green.log`:

| Regex | Result | Log |
|---|---|---|
| `'^TestRemovePayuniNotifyRM0'` | PASS=2 FAIL=0 exit=0 | `pin-rm.log` |
| `'^TestR2IntegrationUpgradeFromReleaseHead$'` | PASS=1 exit=0 (count 87) | `pin-r2.log` |
| `'^TestT06WorkerAuthorityAndFunctionACL$'` | PASS=1 exit=0 | `pin-t06-acl.log` |
| `'^TestWAS0[1-6]'` | PASS=6 exit=0 | `pin-was.log` |
| `'^TestBuyerPaymentHosted(SQLAuthorityAndScope\|AtomicPreparationAndIndependentWire)$'` | PASS=2 exit=0 | `pin-hosted.log` |
| `'^TestStripe(SL02Schema\|AuthorityRegistrarMatrix\|AuthorityIngressMatrix)$'` | PASS=3 exit=0 | `pin-stripe.log` |
| `'^TestMetaClaimsMCI02UpgradeAndExactPrivilegeDelta$'` | PASS=1 exit=0 | `pin-mci02.log` |
| `'^TestPaymentMethodsAuthorityAndEnvironment$'` | PASS=1 exit=0 | `pin-payment-methods.log` |
| `'^TestBuyerPayment(QueryHistoricalCredentialAndACL\|CaptureQueryJobBindingAndRollback)$'` (old payuni path fence) | PASS=2 exit=0 | `pin-payuni-oldpath.log` |
| `go test ./cmd/api -run '^TestRemovePayuniNotifyRM03' -count=1 -v` (post-gofmt re-run) | PASS | `green-rm03.log` |

**Evidence class:** REAL_PG (schema/ACL/RLS/migration-replay on disposable containers) + DB-free HTTP router assertions. PROVIDER_MOCK at most — **no PAYUNi/Stripe/Meta endpoint was called; no real money, no live keys, no real buyer PII** (RM02 seeds synthetic rows: `mock-account`, `ORDER_RM02`). Status taxonomy: schema/ACL gates = REAL_PG; router gate = REAL HTTP (no provider); everything provider-side = NOT_RUN by design.

## Risks

1. `DROP ROLE commerce_payuni_ingress` fails loudly if any environment added dependencies beyond 0136's (receiver never deployed/enabled → not expected; failure is an error, never a silent REASSIGN).
2. Reason-check restore is regex-inversion of 0136's regex, but shape-verified before/after against the live catalog; unexpected shape → RAISE, not guess. RM01 pins byte-equality with the pre-0136 definition.
3. The migrate.go REVOKE runs on every Apply; harmless no-op where the grant never existed (fresh DBs).
4. `check-headers` base pin remains `86e404a4` — passing.

## NOT_RUN / BLOCKED

- **NOT_RUN (locally, by gate discipline "RAM-heavy gates run on GitHub"):** full `./tests/foundation` suite; `scripts/dev/test-local.sh --browser-payment`; `scripts/dev/test-local.sh --payment-worker`; `scripts/dev/release-gate.sh --strict`; `TestBrowserPayuniBaseline` (browser-tagged, old path).
- **NOT_RUN (by hard constraint):** any SANDBOX/LIVE PAYUNi or Stripe provider call; any production action; real refunds/ads/shipping.
- **BLOCKED:** none. No blocker required the two-fix escalation.
- Not delivered: production source/deployment (this packet is architecture + gates; 0161 executes only via `migrations.Apply`).

## Integrator to-do

1. Merge `unit/pay-rm1-remove-payuni` (base = trunk tip `86e404a4`; linear, no migration-number collision: 0161 was integrator-assigned).
2. Run CI gates: full foundation suite, `test-local.sh --browser-payment`, `test-local.sh --payment-worker`, `release-gate.sh --strict`, `depmap.sh --check` (already green locally).
3. Confirm no environment/deploy config still references `COMMERCE_PAYUNI_NOTIFY_ENABLED` (loader deleted; a stale env var is now simply ignored, but hygiene: remove it).
4. OPEN-1 (old PAYUNi hosted/query/capture path) remains open — untouched here by design.
