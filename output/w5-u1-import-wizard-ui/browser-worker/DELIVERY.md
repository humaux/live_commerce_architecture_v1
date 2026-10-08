<!-- Purpose: independent W5-U1 browser-test source handoff, actual commands and narrow evidence boundaries.
Depends on: commit97264f53, migration-import-v1/0152/0156, actual Go handlers and signed HTTPS OIDC.
Used by: root scoped application, integrated strict TS checks and GitHub --browser-migration-import acceptance. -->
# W5-U1 independent browser worker delivery

- task_id: `a83bad3d-2b4d-4b4a-b598-85f9e4a99cea`; child `codex-w5-u1-ui-sub-browser` (no independent claim or delegation).
- Role/config: independent test_worker, configured `gpt-6.1-sol/high`; runtime identity UNKNOWN (parent card).
- Base: `dc11b5ce8ceaf3c755d7192545f34f3c579108cf`; branch `unit/w5-u1-browser-worker`.
- Commit: `97264f53c7f88fa5862334374b80ad844c5a6096`; worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/w5-u1-browser-worker`, clean after commit.
- Changed paths ONLY: `tests/foundation/browser_migration_import_ui_test.go` (538 lines), `tests/admin/import-wizard.spec.ts` (327 lines). No apps/shared scripts/dependencies/contracts/lockfiles/root UI changes, push or merge.
- Summary: fresh unique store and scoped synthetic import fixture; actual Go API/PG commands; signed MOCK OIDC over always-HTTPS Next; no auth-cookie injection or successful JSON response mocks. Raw byte observer hashes original CSV and verifies no header key/multipart; bounded post-commit drop, truncated preview and history503 faults pass through actual handlers. Fifteen authored browser cases, including four en/zh-TW desktop1440/mobile390 variants. Store-scoped SQL/readback proves consent/capability zero, profiles/batches/audits/replay counts, citynull, optional-field preservation/clearing, historical paging and unchanged money/report facts.

## Root integration contract

- Go entry `TestBrowserMigrationImport`; exact selector `^TestBrowserMigrationImport$`; gate `LC_MIUI_ACCEPTANCE=1` plus `LC_TEST_DATABASE_ALLOWED=1`.
- Mode `--browser-migration-import`; root owns scripts/GATES/global config integration. Generated explicit `import-wizard.spec.ts` config does not require global suite; optional root suite `migration-import`.
- Domain env supplied by Go: `LC_MIUI_STORE`, `LC_MIUI_CONTROL_URL`, `LC_MIUI_CONTROL_KEY`; runner env `LC_BROWSER_PUBLIC_ORIGIN`, `LC_BROWSER_EVIDENCE`. No browser bearer/session-cookie injection.
- Root import selectors exactly: `import-wizard`, `import-type-*`, `import-file`, `import-guess`, `import-map-*`, `import-preview`, `import-confirm-next` THEN `import-confirm`, `import-rows`, `import-row-*`, `import-row-previous/next`, `import-restart/back/retry-same/download-failed`, receipt/replayed/consent/city/erased-count. History: `customer-historical-orders`, `historical-orders-count`; accessible importHistoryCopy Previous/Next/Refresh/Retry labels. Two-step confirm and receipt counts are scoped independently of nested Preview counts.
- Runtime red calibration: `LC_MIUI_CALIBRATION=drop-customer-commit` must fail named `MIUI-RED-COMMIT-RECEIPT`; `truncate-preview` must fail `MIUI-RED-PREVIEW-DATA`. These are actual response faults AFTER signed stack readiness. Setup/import/login failure is not intended red. Remove env and run same immutable integrated SHA for green. NOT_RUN here.

Coverage: both customer→order file flows, unknown-header manual mapping/guess/back/two-step confirmation, failure-first code-only preview and fifty-row verdict paging, partial failure server CSV and all-invalid/stale-zero local CSV, original BOM/CRLF body SHA, exact2MiB/+1 and5000/+1 data lines, Big5 remediation, normal replay and changed-map conflict, actual erasure-triggered fresh409 for BOTH kinds with no partial effects, UNKNOWN explicit same-body/map/count replay BOTH kinds/no auto retry, pagehide/auth revocation, optional unmapped-preserved/mapped-empty-cleared fields, consentignored4/consentzero, citydiscarded/null and valid canonical city, archive50+1 readonly/reload/retry, actual customer erasure/tombstone, read-only identity, wizard/console/storage/IndexedDB raw/whole-file absence. Backend erasure caveat is respected: no universal404 claim for retained transaction owners. No imaginary readiness endpoint; current scoped customer receipt enables orders and page navigation resets that receipt.

## Actual commands and codes

| Command / run | Exit | Evidence |
| --- | --- | --- |
| `node --check --experimental-strip-types tests/admin/import-wizard.spec.ts` | 0 | Final syntax only; imports/runtime not evaluated. |
| `gofmt -w tests/foundation/browser_migration_import_ui_test.go` | 0 | Owned source only. |
| `GOTOOLCHAIN=go1.27.1 go test -tags browser -count=1 -timeout=30s -run '^TestMiuiDropWrapper$' ./tests/foundation` with mutation below | 1 | `red-wrapper.log`, `red-wrapper-status.json`; runmiui-wrapper-red PID10323 finished in15s. Named `MIUI-DROP-AFTER-COMMIT: handler invocations=1 want2`. |
| same command after restoring original handler | 0 | `green-wrapper.log`, `green-wrapper-status.json`; PID12872 finished. |
| same focused command with `-v`, after added readback/process metadata | 0 | `final-wrapper.log`, `final-wrapper-status.json`; PID18731 finished, exact self-test PASS1. |
| same focused command with `-v`, final owned process-group cleanup source | 0 | `final-cleanup-wrapper.log`, `final-cleanup-wrapper-status.json`; PID21895 finished, `=== RUN TestMiuiDropWrapper`, PASS1/FAIL0. Final SHA hashes below. |
| `git diff --cached --check` | 0 | Actual staged two-file source patch. |
| `git commit -m 'test: add signed HTTPS CSV migration import browser gates'` | 0 | Commit above. |
| `git rev-parse HEAD`, `git status --short`, `shasum -a 256 <two paths>` | 0 | Commit/hash/clean-tree readback. |

All Go runs use an outer stdlib process-group supervisor, total180s deadline (timeout exit124), Go30s timeout, state JSON with runid/PID/start/end/log/exit code, and log-only child output. Own subcanvas registered matching/failure rules and handoff: current child monitors until terminal completion; automatic child final wakes root. All four runs are FINISHED, no unattended process. Go focused self-test compiles browser-tag sources but invokes no PG fixture, Next or browser.

Reproducible helper-only red mutation:

```diff
- next.ServeHTTP(recorded, r)
+ if !match || fault != "drop-customers" { next.ServeHTTP(recorded, r) }
```

This skips completion of the original handler before dropping its response. The independent handler-invocation marker detects1 instead of2 calls (initial + retry). Restored wrapper calls handler then drops exactly once; second response passes through. This is MOCK HTTP injection-mechanism evidence, not SQL commit/idempotency or UI acceptance. Actual browser cases independently require exactly one batch/audit and same request triple on replay. Red sample/log is retained; no failing tests deleted/weakened.

Future runtime processes also register `next-process.json` / `playwright-process.json` with owned PID, log,24min timeout, success/failure match, handoff/wake. Next cleanup is installed immediately after Start; Playwright uses its own process group and context cancellation/group cleanup, including error paths. These paths are authored but runtime NOT_RUN.

## Evidence bounds / risks / NOT_RUN

Overall source E1 (structure ready); DB-free helper has narrow E3/MOCK red→green evidence. BROWSER, REAL_PG, production Next build, full mode, strict spec TypeScript, browser calibration, 10-shard sweep/aggregate, independent source/runtime review all NOT_RUN here. Old test base lacks new import UI/copy libs; relative actual-repo imports are preserved and strict tsc is root's integrated check, no virtual source overlay. Provider SANDBOX/LIVE are not involved in import workflow and NOT_RUN.

Source semantics can be accepted only after root applies/checks diff and runs immutable integrated CI. In particular actual signed-store permission projection, frontend timeout/visibility timing, download authorization and full-browser layout require runtime evidence. Same-file conflict and stale branches must not be silently retried; missing rendered controls or failed assertions remain defects. Source plan is `../browser-plan.md`; exact frozen wire pointer is `../backend-interface.md`.

Dependency install session34361 returned Unknown process id on the one authorized poll. This does not prove install success/failure; no install restart or dependency mutation was made. Go/Node tools worked for the checks above. No PG/Next/browser/container was started locally; helper httptest servers closed by defer; all owned subprocesses finished. Existing parent/source work was never reset or reverted.

Source SHA-256:

- Go: `1fca0a6876b9d42a7829a9be66c4fa40277119d973482b8ec38d48f0502da4aa`
- Spec: `b13c2bd48a21562f650e682856a86d86509feac15eb08849e8e6c33fc856e686`

Evidence root: `/Volumes/data/live_commerce_architecture_v1/output/w5-u1-import-wizard-ui/browser-worker/`.
