# PR32 manual recovery backend delivery

- task_id: parent coord 2628dd5e; PR32 review 4254715306; child manual_recovery_backend.
- base_commit: b1229fbda54e3aa4a1262387d864a20382ec90f0.
- branch/commit: unit/lcu3-manual-recovery-backend / 501bafc1ba932678d86937983f372bbe2826ed75; Implemented-by: Codex-3. Not pushed.
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/manual-recovery-backend.
- role: backend implementer. Agent: Codex. Actual model and reasoning effort: UNKNOWN (not exposed in this child's runtime).
- write_paths changed: internal/merchanttools/manual_recovery.go; internal/httpapi/merchanttools.go; internal/httpapi/manual_order_recovery_test.go; tests/foundation/manual_order_recovery_test.go; contracts/storefront-v2.md G3. No order_attempt.go, migration, grant, dependency or apps changes.

The additive manual recovery route accepts the original strict ManualInput + Idempotency-Key. It normalizes before binding the existing shared outer attempt, with all three permissions, and probes the original private owner's Begin under its exact lock. It returns created only with the matching upper manual receipt proving atomic source/link/audit completion. Begin-only remains retry_later; original explicit replay finishes it exactly once. Absence commits checkout closure + recovery + audit atomically and makes the old key attempt_closed. No placement validation/options prerequisite and no claims cleanup. Existing A16R untouched.

Evidence class: E3 REAL_PG/race in the focused local environment; DB-free router E3. Current-source digest 426ccb381c5f36695c95dc01e00ffd33ceceb6e76aff836efa1e80da62bc9993 (algorithm in SHA256.json). All tested files unchanged through green and commit; worktree clean. Independent review is pending, so this is not an E4/CI/browser/release claim.

## Actual commands and exit codes

- `bash scripts/dev/test-focused.sh '^(TestManualRecovery|TestMerchantToolsManualOrder$)' ./tests/foundation ./internal/httpapi` -> exit 1: red PASS=1 FAIL=6 SKIP=0, new route 404, existing manual placement passes.
- `bash scripts/dev/test-focused.sh '^(TestManualRecovery|TestMerchantToolsManualOrder$|TestLiveConsoleRecovery)' ./tests/foundation ./internal/httpapi` -> exit 0: green PASS=32 FAIL=0 SKIP=0, race enabled by the script.
- `GOTOOLCHAIN=go1.27.2 go test ./internal/httpapi -run '^TestManualRecoveryRouter$' -count=1` -> exit 0.
- `GOTOOLCHAIN=go1.27.2 go vet ./internal/merchanttools ./internal/httpapi` -> exit 0.
- `bash scripts/dev/check-headers.sh b1229fbd` -> exit 0.
- `git diff --check` -> exit 0.
- `git commit -m 'fix(merchanttools): add manual order attempt recovery' -m 'Implemented-by: Codex-3'` -> exit 0.

New PG cases cover removed bank-transfer option and invalid frozen body sealing, completed/lost response readback after policy changes, normalization, exact backend termination after Begin but before manual receipt commit, explicit replay order/hold/source/link/audit exactly once, changed body and operation conflicts, missing scoped permissions/foreign store, cached created cannot bypass a fresh manual receipt probe, revoked orders:read after the original Begin lock wait, and concurrent seals.

## Evidence

All files in output/playwright/manual-recovery-backend: freeze.txt; red.log/red.exit/red-source.json; green.log/green.exit/green-source.json; SHA256.json; router.log/router.exit; headers.log/headers.exit; vet.log/vet.exit; diff-check.exit; commit.log/commit.exit; cleanup.json.

Red evidence note: the first exit recorder used zsh's read-only status variable after the test had already exited1; red.exit was repaired from the recorded test-focused exit1. Assertions were corrected to the existing idempotency_conflict code after inspecting its mapper; no acceptance threshold was weakened. Later recorder uses run_rc.

## NOT_RUN / risks / integration

- Full G07/release-gate, full check-gates/foundation suite, all browser modes, CI and independent review NOT_RUN in this child, per root's no-full-gates/no-browser scope. Root owns UI/BFF/Node/browser integration and required CI modes.
- Existing ten-minute source marking window remains: an aged incomplete manual pipeline may retain UNKNOWN when original replay cannot complete it. No new retry or cleanup behavior was introduced.
- No known remaining P0/P1 from focused evidence; independent reviewer must determine merge readiness. General production suitability UNKNOWN.
- Integrator: review/cherry-pick this commit, regenerate shared dependency map/registry as appropriate, run required CI and browser gates; no schema/migration/dependency merge required.
- Humaux decision: 0004ef0d-14e3-4de5-8936-ca44d7039021. Index completed 2 files/32 entities; unique registerMerchantToolsRoutes linked to that decision. Qualified ManualOrders.Recover lookup unsupported by index naming, so the route entity was used.
- Both task-owned PG containers removed by the harness; no process, fixture container or port retained (cleanup.json). No other task directory deleted.

- Humaux fix verified via exact grep: 673a86a7-d7b7-4ab9-a414-34d155975ccf, title [fix] PR32 4254715306 manual recovery backend 501bafc1 REAL_PG green. Own canvas updated under lock; parent element unchanged.
