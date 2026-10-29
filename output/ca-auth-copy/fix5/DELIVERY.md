# FIX5 #251 — WebKit fault-injection comparison (tests-only)

Status: comparison prepared, CI PENDING. Author assessment E0; syntax E1; historical assertion failure E3 negative evidence only. This is attempt 1 of the newly authorized FIX5 comparison, not a third speculative product patch. Do not switch the retained inspector case or change product recovery until CI establishes the real-failure result.

## Authority and scope

- Latest integrator card FIX5: compare real `context.setOffline(true)` transport failure against inspector `route.abort`; preserve recovery assertions, native click, URL and all three locales. Maximum two targeted attempts. No local tests/servers, no PR merge, no deployment.
- Read `AGENTS.md`; `架构.md` §§0,1,2,26,27: evidence environments remain distinct; same-app ownership and independent review; no claim of product/production acceptance from a browser model or author report.
- Read `contracts/merchant-password-auth-v1.md` authentication contract and `tests/admin/password-auth.spec.ts` current recovery assertions. `contracts/tasks.json` DAG inspected; FIX5 dispatch supplies the narrow authorization (no matching FIX5 task entry).
- Base on entry: f80a0b0c7f6fa95e560227d753306d407de4e4d9. Merge of current origin/r3/integration completed on own branch before ownership generation: 9dbb1fc0e654c25c777201bb0cf76fa56cfe0601 (integration input d8093bc43). Remote comparison baseline: 2d09eaea8df0e1627c0b5b0787172cfbea3f3a82.
- Runtime identity: dispatched Codex agent, inherited runtime model/effort not exposed as a verifiable ID; worktree /Volumes/data/live_commerce_architecture_v1/.worktrees/ca-auth-copy, branch unit/ca-auth-copy. No delegation or parent coord/canvas mutation.
- Humaux memory/coord/code-index tools absent from the active tool inventory; no store, canvas update, lock or code-memory link claimed. Repository/CI records are the available handoff trail.

## Controlled comparison

One shared recovery assertion body runs for offline then inspector faults. All three offline locales (zh-CN, zh-TW, en) register before the existing serial inspector RED, so a later inspector failure cannot hide successful offline results. No-JavaScript and healthy-hydration tests register once as before. Every existing assertion, timeout, inert-form check, real reload click and unchanged-URL check remains verbatim in its original scenario order. No gate-adapter assertions or exemption lists changed.

Offline setup buffers the actual server document through a document-only route, calls `context.setOffline(true)` before returning its unmodified response/body, and lets all JS requests fail through the browser network stack. No script routing or abort is installed in this variant. A requestfailed observation synchronizes the existing positive failure-count assertion. Offline failed-script URLs and their recovered requests feed the same counters/assertions as the inspector variant. The document route remains installed through reload, preserving routing's disabled-cache policy on both navigations. Context goes online immediately before the existing native link click. Original inspector routing is retained as the comparison control.

The HTML buffering is fixture setup only; no DOM mutation, synthetic click, API-driven acceptance, forced action or fabricated HTML. No credentials are entered in recovery cases; trace boundaries remain credential-free. Events add fault/locale labels and retain no query/header/body. If offline passes all locales in WebKit/Chromium and inspector still fails WebKit, the next implementation push may replace only the fault injection. If offline reproduces missing hydration, inspect its trace and fix the authorized admin auth recovery path instead. If setup itself fails, classify as setup failure rather than product evidence; use at most one further targeted FIX5 attempt.

Official API sources read 2026-10-29: [BrowserContext.setOffline](https://playwright.dev/docs/api/class-browsercontext#browser-context-set-offline), [Route.fulfill](https://playwright.dev/docs/api/class-route#route-fulfill), [Page.route cache policy](https://playwright.dev/docs/api/class-page#page-route).

## Gate and click ledger

CI commands already own execution: `bash scripts/dev/test-local.sh --browser-password-auth` via Chromium bundle, and `--browser-webkit` password-auth step. Do not run these locally. Expected comparison inventory is 24 cases (the original 21 plus three offline recovery cases). In serial mode a failing offline setup/assertion will skip later cases; that is negative/incomplete evidence, never a locale pass.

| Page | Control/action | Expected | Actual |
| --- | --- | --- | --- |
| /zh-CN/?recovery=1 | Offline -> online; native reload link click | Failure observed; inert form/loading; delayed link; URL unchanged; hydrated/enabled; recovery UI removed | CI PENDING |
| /zh-TW/?recovery=1 | Same native flow | Same shared assertions | CI PENDING |
| /en/?recovery=1 | Same native flow | Same shared assertions | CI PENDING |
| Same locales, inspector variant | Original route.abort -> native click | Same shared assertions; retained RED control | Historical WebKit zh-CN fails hydration; fresh CI PENDING |

No UI design/copy/product change in FIX5: Bourdieu, Mobbin/Eagle, design parity, Figma guidance, density and copy lint are not applicable to this test-injector comparison. Existing three-language visible copy unchanged.

## Validation, risk and handoff

- `node --check tests/admin/password-auth.spec.ts`: exit 0 (syntax only, E1).
- `git diff --check -- tests/admin/password-auth.spec.ts output/ci-select-backend-browser/browser-ownership.json`: exit 0 (whitespace only).
- Explicitly authorized generator `node scripts/dev/derive-browser-ownership.mjs`: exit 0; generated only the changed spec's source hash after the integration merge. No import/route/component/dependency changes, so no harness allowlist or depmap inputs changed.
- Changed authored source: tests/admin/password-auth.spec.ts only. Outside card source scope: generated output/ci-select-backend-browser/browser-ownership.json (pre-approved generated scope). Evidence under output/ca-auth-copy/fix5 is listed in PR body. Pre-existing dirty DELIVERY.md, FIX-RED-251.md, red.log and other untracked artifacts preserved and excluded from this commit.
- Existing RED raw pointers: red.log, inspector-playwright-red.log, inspector-recovery-events.jsonl. Gate run/job verified live with gh run view; logs read, not inferred from an old report.
- NOT_RUN: all local tests/builds/servers/browser/PG/containers; fresh CI results, final implementation decision, independent review, LIVE/deployment. Risk: offline setup may itself expose an engine-specific fixture failure; only CI can classify it. No tests, fixtures, waits or thresholds weakened.
- Rollback: revert this tests-only commit on the assigned branch; no production effect. Integrator owns CI completion notification and fresh-session relay. End after recording Gates/Foundation workflow run IDs; no silent watcher or long polling.
