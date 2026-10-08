<!-- Purpose: Hand off LC-U1 fixes for CI run 37470029840 with original privacy guard and honest focused-run status.
Depends on: run head 4b52eab1, merged trunk 71235fc4/base 1ad59288, actual LC-B7 DTOs and source hashes below.
Used by: Integrator's GitHub rerun; independent UI review. -->
# LC-U1 CI2 repairs

Branch: `unit/lc-u1-shell`; base `1ad59288541de095abe262590a5cdfa33756004a` merges trunk `71235fc4`.
Only LC-U1 was handled. W1-01U and W3-U1b remain with their owners. D3 `b86c5afe` is already an ancestor of the failed CI head `4b52eab10b66a37c5680627cb3b24846e74bb51e` (Git check exit 0); its parser/picker/parity fix was not repeated.

## Exact failures and fixes

| CI evidence | Actual cause | Change |
| --- | --- | --- |
| Console six main cases, `:88 receipts.length` | MOCK facts encoded an uninitialized Go receipt slice as `null`, before any command. This is not the production A1 `offers` array. | Initialize receipts as `[]`; require an array in facts. Preserve every command/effect count. Product parser still rejects null offers with `invalid_console_response`; an empty `[]` is valid. |
| Console reauthentication, `:243` timeout | Sign-out was inside the closed Account details. | Real click on its enclosing summary before sign-out in both paths; no force click or timeout change. |
| Console narrow cases, `:306` / `:319` | Generic alert locator also matched Next's global route-announcer. | Scope stock validation to its offer and backend refusal to Console. Keep message and no-mutation assertions. |
| TCV09 privacy guard | Facebook iframe CSP exception differed from frozen CVS baseline. | Remove iframe and route-specific `frame-src`; replace with a numeric Page/post link, active+verified source, fixed Facebook domain and `noopener noreferrer`. Original `TestCvsNoIframeAndPrivacy` is unchanged and now passes. |
| Sweep 126 loads / 3 failures | Fixture omitted `CommentStream`; real A1 returned `bridge_disabled/unavailable`, so Console was correctly detected as degraded. | Construct existing `NewBridgeClient` and `NewCommentStream` using an authenticated loopback MOCK and pass `Options.CommentStream`. No-source now takes actual Go's `not_started/no_source` branch. Status copy, detector, thresholds and `tests/ui/**` are unchanged. |

The external-link positive uses a real popup click with only the external Facebook dependency intercepted by MOCK HTML. No live Facebook request is made. The A5 fixture resets its fake source-binding flag when copying a scene. Manual preview refresh/focus reads occur **after** the unmodified `3500..8000ms` polling measurement, following an independent review finding; they cannot contaminate the captured cadence.

Independent read-only review confirmed the Console fixes and the cadence chronology, and found no blocking issue in the no-source sweep wiring. Boundary: the loopback bridge MOCK does not prove exact body scope pinning if a bound-source read occurs; bridge-authority acceptance is not claimed. The current fixture defect concerns the unbound-source branch, which does not contact the bridge.

## Evidence and commands

| Command | Exit / result | Evidence |
| --- | --- | --- |
| `bash scripts/dev/test-node.sh` on final test assertions | 0; 496/496 | `ci2-node-final.log` |
| Scoped Console/model/workspace Node tests | 0; 25/25 | `ci2-models-final.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `ci2-tsc.log` |
| `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module ESNext --moduleResolution bundler --allowImportingTsExtensions --typeRoots apps/admin/node_modules/@types --types node tests/admin/live-console.spec.ts` | 0 | `ci2-spec-tsc-final.log` |
| `bash scripts/dev/check-gates.sh` | 0; 72 modes | `ci2-check-gates-final.log` |
| From `tests/foundation`: `go test -run '^TestCvsNoIframeAndPrivacy$' -count=1 taiwan_cvs_guards_test.go` | 0 | `ci2-privacy-green.log` |
| `gofmt -l` on the two modified browser fixtures; `git diff --check` | 0; empty output | Command receipts |

The privacy command runs the unchanged standalone standard-library guard file only; no PG/foundation/browser suite is involved.

Per the latest instruction, two local focused attempts used `LC_BROWSER_CONSOLE_GREP='en-1586|unknown receipt|LC-U1 en:'` and `--browser-live-console`. **Neither reached PG or browser execution.** Both were canceled while queued behind another healthy owner (10435). Exact own PIDs/cwd, foreign lock holder and absence of each own container were checked; only owned waiters 13004 and 22626 were stopped. Both returned 137. The first was canceled to apply the cadence review fix; the second was canceled to hand off promptly for GitHub. Logs: `ci2-focused-browser.log`, `ci2-focused-browser-final.log`. These are **NOT_RUN/canceled waits**, not product FAIL or browser PASS. Own processes/containers are absent; foreign ones were not touched.

An optional grep is now passed as a Playwright argument by the fixture and logged `FOCUSED_SPEC_ONLY`; without it GitHub still runs the entire spec. A filtered success must never be reported as full-mode acceptance.

## CI gates

Integrator rerun on the final SHA:

- `focused:^TestCvsNoIframeAndPrivacy$` (or existing `shard:^Test[B-K]`)
- `--browser-live-console` without grep (all 11 cases)
- `--browser-click-sweep`
- `--browser-visual-lint`
- Standing D3 regression: `--browser-live-claims`, `--browser-e2e`

Current browser execution, screenshot acceptance, browser-fixture Go compilation and full foundation are **NOT_RUN**. No local sweep/visual/full foundation run, push, workflow dispatch, deployment or real provider action occurred. Optional R04 binary suite remains NOT_RUN (binary unset). E3 local model/privacy evidence does not accept the entire UI unit.

## CI2 artifact provenance

Run `37470029840`, head `4b52eab1`, is FAIL. Current Console artifact: `gate-2/ci-gates/live-console-2435658823/playwright.log` (**10 failed / 1 passed**), SHA256 `5ab0cd0a50ca83f943ff13d2650df87e905d0b73e3f555ea15be347365a1544f`.
Current sweep: `gate-0/ui-click-sweep/ledger.json`, timestamp `2026-10-06T14:04:48.663Z`, SHA256 `edc46cf236a7107460fe80c672cbee3e612cee1cc8eb88b7c46a6830ef47c90d`; 1019 controls PASS, 0 FAIL, 18 SKIP; 18/18 journeys PASS. Other uploaded October-4 ledgers are stale and not cited. Raw downloads/traces are ignored under `ci2-artifacts/`; run metadata is committed in `ci2-jobs.json`.

## Tested source SHA256

| File | SHA256 |
| --- | --- |
| `apps/admin/components/LiveConsole.tsx` | `944fc15fac9c492f337375aa8ddda095a2d041a1a91dfd6ea2a8b76ab8645b04` |
| `apps/admin/next.config.ts` | `fd03f73cd12b78f2e92d433b423341c1f65072061005da0700710c9d99d0ef6e` |
| `apps/admin/src/features/live/workspace-copy.ts` | `41f6b9d23bcb73f7c8757e1a9dde7f4c583e091bc35af34bfb65ca0523318f0d` |
| `apps/admin/src/features/live/workspace-model.ts` | `b533126ff8de4e34cc228fc0f67517ece66596e18db4c420f72a9488e46372d1` |
| `tests/admin/live-console-model.test.ts` | `9c070a0bf6db86d83fe6e2c7473b30b902aead66c31ad7ce24ce7edb1369d998` |
| `tests/admin/live-console.spec.ts` | `6fe003285f1fc01bcc1d88b1d807376bb4de26156fcf31ad670fb6542c319bf6` |
| `tests/admin/live-workspace.test.ts` | `3b4fb838e4280f8b28eb4e8712a6a386168a92cca75e4ffbe2a33371a7b2d8be` |
| `tests/foundation/browser_click_sweep_test.go` | `06bd816e3a2bf8e476ccd3ddba4431aa0950c5a67827136a3820d559266d44d1` |
| `tests/foundation/browser_live_console_test.go` | `56a331e5b4ed0430ddb5e4fa1a4c3a058a46054b69f838af33c024aad3516451` |
