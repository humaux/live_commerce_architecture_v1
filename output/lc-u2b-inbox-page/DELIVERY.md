# LC-U2b inbox page + BuyerPanel delivery

- Branch: `unit/lc-u2b-inbox-page`; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u2b-inbox-page`.
- Base: `f73405150a1cca3c588d4546f3b5a8e0d1a456a3` (owner-assigned base overrides the older brief reference). Tested source commit: `c3bca7871953c5f88c055c7619030aecc4c182a4`; authorized trunk merge: `26e9aec2` of `1aad42d0`; final evidence commit is reported in the author handoff. Source is also bound by `source-hashes.txt`.
- Author: Codex-4, GPT-6-based Codex; exact parent model/effort not exposed. Only the requested local trunk merge; no push/deployment.
- Evidence: **E3 / MOCK for actual-source Node component/hook and delayed-route counterexamples; E1 for TypeScript and harness/parser registration. Author scope follows the integrator ruling; browser acceptance is pending CI, not locally claimed. Backend feature availability is deferred to LC-B3b and does not block LC-U2b merging first. Current source BROWSER / REAL_PG / SANDBOX / LIVE NOT_RUN locally. Prior browser CI failures at 08614fb6 and d313d97c remain failures; the recovered d313 evidence and current fixes are recorded below, without a current browser acceptance claim.**

## Summary

- `apps/admin/app/[locale]/messages/page.tsx:19`, `components/Inbox.tsx:21`: authenticated store entry, messages registry, list filters/cursor paging and memory-only selection. SSR serializes no buyer/thread data.
- `components/InboxThread.tsx:21`: A9 paging, A10 only with reply permission, keyed A11/A12, authoritative generation/window and fixed delivery/errors; published templates and explicit dm_session capability checks.
- `components/BuyerPanel.tsx:17`: standalone store + conversation/bundle + callbacks, independent hide/session fence; orders gated by orders:read; no inferred link CAS version.
- `src/features/messages/privacy.ts:8`, `use-privacy.ts:11`: abort epochs, controller-bound tickets, synchronous hide clearing, terminal navigation revocation; exact immutable receipt on uncertain retry. Normal Refresh retains unknown delivery.
- `lib/inbox-bff.ts:15`, catchall route: precise A8–A14/GET templates, strict method/query/body, real Request stream/auth/CSRF tests, no-store and fixed safe errors. Bounded 1 MiB admits 50 valid Unicode messages and refuses excess.
- `tests/foundation/browser_inbox_ui_test.go:31`, `tests/admin/inbox-ui.spec.ts:157`: registered `--browser-inbox`, signed MOCK OIDC + real Next/Go/PG, real click/PG readback, privacy/cross-store/read-only/window/capability cases and committed-send/lost-ACK same-receipt retry. INU06/INU07 cover zh-TW, zh-CN and en at 1440/390 with independent literal labels, real locale changes and mobile Back clearing.

Contract/interface changes: **none**. Additional write scope: the owner-approved shell-registry expectation and owner-required C root fix in `tests/foundation/browser_click_sweep_test.go` (acceptance fixture only). No production backend Go, migrations, OpenAPI, module or dependency lock changes; existing Go services are mounted only in the acceptance harness. `playwright.config.ts` gets the necessary suite registration. zh-TW/zh-CN/en unit copy is complete per the integrator correction; packages/i18n is unchanged.

## Current PR #8 CI recovery / persisted UNKNOWN / real bundle-copy proof (source c3bca787)

`git fetch origin && git merge origin/unit/lc-u2b-inbox-page` exited0, already up to date at4d19a45a. Each green implementation step was committed:

| Commit | Change and red→green evidence |
| --- | --- |
| `7b50074c` + `f6e1745a` | A9 persisted outbound UNKNOWN shows the full three-locale Messenger warning and blocks new/captured submissions. Preserve known UNKNOWN across later pages; absence is not reconciliation. `persisted-unknown-ci-node-red.log`4 PASS/5 RED →9 GREEN; reviewer pagination counterexample `persisted-unknown-page-red.log`9 PASS/1 RED →10 GREEN. Strict TypeScript exit0. |
| `f982e5a1` | INU09 creates a real flagged claims bundle, mounts existing M6/M7, publishes synthetic HTTPS storefront, clicks the existing copy button through real BFF and native clipboard, and compares PG link/hash/generation/expiry/principal plus token-free command receipt. Initial structural seam RED and the historical ordering-guard RED are retained. The fixture closes the unused inherited claim window for isolation. Correction: 0122 removes the historical 0060 unique index, so a second OPEN window was never a current-schema startup failure. Parent Node11/11, strict TS and check-gates exit0. Actual Go/PG/browser execution remains NOT_RUN. |
| `c3bca787` | Replace fixed ticks in recovery tests with actual list/panel/thread state and complete copy-action promises. Original67 assertion ASTs/order and shared Host unchanged; one completion assertion added. Bind existing async `copy` directly to onClick so tests can await its entire result; no client proxy remains. Actual stopped-guard removal yields POST3 versus expected2 RED, restoration GREEN. |

**CI failure diagnosis:** [run37652592946](https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/actions/runs/37652592946), foundation job112899493153, ran Node24.21.0. The old recovery suite used12 `setImmediate` turns to assume WebCrypto/A8/A13/M7 completion. Those ticks do not establish readiness. Under identical CI clock2026-10-07T16:32Z/UTC/C.UTF-8 on macOS, old source passes23 with ordinary crypto but fails23 when completion of the **real** SHA digest is delayed50ms. The old undefined-VNode / busy-input signatures recur; this proves the scheduling assumption is unsafe, not that timezone or the Node version alone caused the remote timing. `ci-37652592946-failed.log`, `ci-recovery-ci-node-red.log` and its source snapshot preserve the observations.

- Local original Node24.15.0; exact24.21.0 runtime obtained from official nodejs.org with published SHA256 verified (`ci-node-version.log`). `CI=true TZ=UTC LANG=C.UTF-8 pnpm install --frozen-lockfile --ignore-scripts` exited0 and reported dependencies up to date; no lockfile edit, no claim of recreating dependencies. macOS execution does not constitute Linux CI acceptance.
- Positive readiness now requires actual A8 bundle+idle, selected A13 facts+idle, A9/A10 authority+idle, or the recovery action's committed result. Held requests await arrival; privacy-boundary and captured-terminal negatives await the **whole actual action**, including any incorrectly started asynchronous path. No fixed-tick readiness remains in the recovery suite.
- Independent review caught an intermediate shortened negative wait. `ci-recovery-action-red.log` fails before the handler exposes its Promise. `ci-recovery-stopped-mutation-red.log` removes only the stopped action guard, keeps the disabled UI, and fails the captured retry assertion3!=2; `ci-recovery-stopped-restored-green.log` restores that exact guard and passes. No mutation is retained in production source. The full pressured recovery suite is23/23; all67 original assertions remain with exactly one added completion assertion (`ci-recovery-action-assertions.log`).
- A9 UNKNOWN is latched only within the live component lifetime, cleared on terminal clearing/new component, and never stored in browser storage. Later A9 pages cannot silently remove its warning/guard. Existing lost-ACK same-key behavior remains when no final UNKNOWN was learned.
- INU09 also checks cached recopy causes exactly one M7 receipt/generation and no DM, pending flag clears on Refresh, and the click ledger records the action. Token stays in browser memory/clipboard; Node/PG comparison uses digest/booleans, not the raw credential. Only this credential-bearing case disables trace/video/automatic screenshots, avoids reading the M7 response body and clears its task-owned clipboard. Static/evaluator cases are E1/MODEL_ONLY, not native clipboard acceptance.
- P2 `4209444305` capped storefront-origin scan is **unchanged**; the integrator owns `output/integrator/FOLLOWUPS.md`. No production Go, migration, OpenAPI or new BFF route was changed. Go edits are acceptance harness only.

Final permitted gates ran against immutable **c3bca7871953c5f88c055c7619030aecc4c182a4**, exact CI Node24.21.0 first on PATH with `CI=true TZ=UTC LANG=C.UTF-8`. No source changed during these runs. `ci-final-gates.json` binds argv, environment, source, exit, elapsed time and timeout; process timeout300s plus5s owned-group termination grace.

| Exact command | Exit / evidence |
| --- | --- |
| `node --test --experimental-strip-types tests/admin/inbox-review.test.ts tests/admin/inbox-review-recovery.test.ts tests/admin/inbox-window.test.ts tests/admin/inbox-evidence.test.ts tests/admin/inbox-entry.test.ts tests/admin/inbox-browser-source.test.ts tests/admin/inbox-unknown.test.ts tests/admin/inbox-bundle-browser.test.ts` | **0,77/77**; `ci-final-components.log` |
| `bash scripts/dev/test-node.sh` | **0,714/714**; `ci-final-node.log`; optional R04 binary suite explicitly NOT_RUN |
| `bash scripts/dev/check-gates.sh` | **0**,79 modes/1219 foundation names/header ratchet; includes the script's normal and browser-tag **static go vet**, not Go/PG/browser runtime; `ci-final-check-gates.log` |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | **0**; `ci-final-admin-typecheck.log` |
| `pnpm exec tsc --noEmit -p output/lc-u2b-inbox-page/pr8-test-tsconfig.json` | **0**; `ci-final-test-typecheck.log` |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` | **0**,49 automatic modes/deploy=false; `ci-final-pr-plan.log` and parsed JSON |
| `node --test --experimental-strip-types tests/admin/inbox-review-recovery.test.ts` with matrix environment, plus `nice -n 19` variants | **0 for32 valid profiles,736 cases**,12 nice19 runs; `ci-final-recovery-matrix.json` stores exact binary/argv/env and per-run log pointers |
| `git diff --check`; ancestry4d19a45a and26e9aec2; `gofmt -l tests/foundation/browser_inbox_ui_test.go` | **0 each**, no formatting output; `ci-final-static.log`,37 source SHA256 values verified |

The current32-profile matrix covers CI24.21 at `2026-10-07T16:32:00Z`, `2031-06-15T00:01:00Z`, `2098-06-15T23:59:00Z`, UTC/Asia-Shanghai, real crypto completion delay0/50ms, repeated normal/nice19 execution, plus local24.15 controls. Only `Date.now` is varied; OS time is untouched. Each child has60s timeout and5s termination grace. First20 unchanged-source successes are retained in `ci-final-recovery-matrix-initial.log`; final12 are in `ci-final-recovery-matrix.log`, all referenced by the final manifest.

**Preserved additional experiment failure:** initially selecting `2098-12-31T23:59:00Z` leaves only60s before the fixture's2099 hard deadline. Its7 positive-send preconditions failed because the implemented five-minute cutoff correctly disables sending;16 other cases passed. That exit1 is retained in the initial log/JSON. The matrix input was changed to June's late evening within the fixture validity window; no product rule, fixture or assertion changed. This additional failure is not presented as a pass. The window suite separately verifies equality/inside/outside the cutoff.

Independent source reviews closed the UNKNOWN paging issue (`b77e3c18`), the browser fixture setup review (its earlier single-OPEN premise was incorrect and superseded by `d47bc17f`) and full-action negative observation issue (`dd59f2b1`); no remaining confirmed scoped P0/P1/P2. These reviewers performed read-only source/AST checks, not browser acceptance. A separate read-only delivery audit verified all six current-source exit records,77/714 test counts,32 valid matrix rows/736 cases/12 nice19 runs,37 hashes and the preserved failed boundary experiment; it found no factual mismatch. Test workers used isolated branches/worktrees; parent applied their changes and reran current-source gates. Configured workers/reviewers were gpt-6.1-sol/high or platform_explorer/medium; actual backend model revision is not exposed.

**E3/MOCK for local actual-source tests; E1/MODEL_ONLY for browser harness/evaluator readiness. NOT_RUN:** current Linux required CI, actual INU09 real Go/PG/native clipboard, normal browser and same-SHA INU05 calibration, production build and LIVE. Integrator pushes; required49 modes include foundation-shards, browser-inbox and click-sweep. Preserve INU09 PG/clipboard/ledger proof and normal-green versus `LC_INBOX_CALIBRATION=retain-thread` red specifically on INU05. No push or production action was performed by the author. All owned test processes ended; the checksum-verified task-only Node runtime/archive was removed after the last run (`ci-final-cleanup.log`). Its official URL/SHA remains recorded; shared dependency caches and worker worktrees were preserved.

## Historical PR #8 A/B/C/D handoff (source 0875e548)

- **A / INU01:** recovered run 37632486619 trace has A9 HTTP200 with `attachments:null` and a page error `Cannot read properties of null (reading map)`. This is a render exception, not evidence that a focus event cleared selection. `Message.attachments` now admits the actual nullable Go projection; rendering treats valid null as no attachments. Malformed non-array/attachment shapes fail closed before painting. Actual Inbox → Thread/BuyerPanel tests preserve selection and DM rendering for null and []; no backend field is fabricated.
- **B / INU02–07:** signed auth/default-store navigation must settle before using `shell-store-selector`. The actual `selectFixtureStore` drives that control, waits the exact canonical overview URL and old Inbox removal, checks the exact selector value/nav, then opens Messages in the fixture store. INU04 uses the same real switch. Existing store/tenant assertions are retained. The Node counterexample transpiles/invokes the real login helper with pending auth/store navigation; it is MOCK ordering proof, not a Playwright pass.
- **C / sweep page load:** the sweep omitted `Options.Inbox`, so the real router did not mount A8; the owner already had `inbox:read`. The acceptance fixture now constructs the existing synthetic payload keyring, Inbox and templates services and reuses the existing seal/job client. Before browser startup, the actual signed-owner router must return200 with canonical `items:[]`; the anonymous router must still return401. These assertions are authored but **Go NOT_RUN locally**. Source checks are **STRUCTURE_ONLY**. No production Go or DB contract changed.
- C frontend distinguishes list404 from a conversation404 and retains `privacy.expire`, private clearing and the request fence. It never converts404 to empty success. Three locales show one inbox-scoped unavailable alert without false sign-in guidance. Actual HTTP200 empty-list tests show no alert, thread/panel, extra read across rerenders or visible focus. Ledger `keeps updating` denotes failed 8-second settling; the preserved entries are `page-error-state`, not evidence of an actual polling/request storm. The sweep fixture now supplies the missing real route; runtime confirmation remains CI.
- **D / CI-only1!=2:** all initial/terminal A9/A10/A13/A14 checkpoints now await the actual committed state, and held requests await their transport-arrival promise. Original69 assertion ASTs are identical versus39abae9f; no receipt/key/body/refusal assertion was relaxed. The only fixed-tick drain left follows disposal of an already aborted host and is not a readiness gate. Immediate and50ms A9 authority variants both remain. The earlier39aba repeat did expose adjacent A11/A12/A14 fixed-tick flakes; those failures are retained and corrected together.
- **Queued review items retained:** bundle-only rows offer memory-only Copy claim link through existing claims M6/M7, never Send; UNKNOWN retries keep the exact key/body. Provider health is a hard gate for one binding and advisory for several, with A12 authoritative. Five-minute cutoff is strict, uses a boundary timer and rechecks captured callbacks. Exact A1.1 zh-TW instruction and zh-CN/en equivalents sit beside Copy. Missing A14 version remains honestly disabled pending LC-B3b. Merge26e9aec2 and evidence-folder/safe-Go-summary fixes are included.

Red-first records (actual commands in each log):

| Command before the corresponding source fix | Exit / evidence |
| --- | --- |
| `node --test --experimental-strip-types tests/admin/inbox-window.test.ts` | **1, 3 PASS /7 RED**; `window-red.log`, baseline6202abb1; strict deadline/timer and three-locale manual instruction |
| `node --test --experimental-strip-types tests/admin/inbox-entry.test.ts` | **1, 2 PASS /7 RED**; `entry-red.log`, baseline39abae9f; A null/malformed rendering, C three-locale context and harness registration |
| `node --test --experimental-strip-types tests/admin/inbox-browser-source.test.ts` | **1**; `browser-store-red.log`, actual old login pending navigation |
| Full14-case review repeats at39abae9f, normal/nice19 | **1 at run4 of both profiles**; `window-repeat-results.json` and logs preserve6 successful/2 failed runs; `node-readiness-red.log` preserves the failed readiness checkpoints |
| Strengthened `node --test --experimental-strip-types tests/admin/inbox-entry.test.ts` before reviewer-P2 correction | **1, 6 PASS /3 RED**; `entry-auth-guidance-red.log`, source9f6fe578; false EN/TW/CN sign-in guidance |

Green records at intermediate sources remain historical: `browser-blocks-green.log`10/10, `entry-auth-guidance-green.log`9/9, worker `node-readiness-green.log`40 complete14-case runs, and `final-*` six gates plus100 runs at9f6fe578. The initial static-only method spelling failure is retained in `browser-blocks-static-initial.log`; production fixture uses `http.MethodGet`, with assertions unchanged. Captured logs normalize trailing whitespace only; failures, counts and exits are preserved.

Final allowed checks ran on immutable **0875e548bd70dc38ab8720b92b85940ad9046f53**, with no source edits during execution. Exact argv/exits/source/timeouts are in `unified-gates.json`; each process has300-second timeout and5-second owned-group termination grace. Sessions29594/8928 ended; no author background processes remain. `source-hashes.txt` verifies34 current files.

| Exact command | Exit / evidence |
| --- | --- |
| `node --test --experimental-strip-types tests/admin/inbox-review.test.ts tests/admin/inbox-review-recovery.test.ts tests/admin/inbox-window.test.ts tests/admin/inbox-evidence.test.ts tests/admin/inbox-entry.test.ts tests/admin/inbox-browser-source.test.ts` | **0, 62/62**; `unified-components.log` |
| `bash scripts/dev/test-node.sh` | **0, 699/699**; `unified-node.log`; optional R04 binary suite explicitly NOT_RUN |
| `bash scripts/dev/check-gates.sh` | **0**, 79 modes /1219 foundation names /header ratchet; `unified-check-gates.log` |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | **0**; `unified-admin-typecheck.log` |
| `pnpm exec tsc --noEmit -p output/lc-u2b-inbox-page/pr8-test-tsconfig.json` | **0**; `unified-test-typecheck.log`, real strict admin config |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` | **0**, 49 automatic modes, deploy=false; raw `unified-pr-plan.log`, extracted `unified-pr-plan.json` |
| `node --test --experimental-strip-types tests/admin/inbox-review.test.ts` ×50 | **0 every run, 50/50**,14 cases each; `unified-repeat-normal.log` |
| `nice -n 19 node --test --experimental-strip-types tests/admin/inbox-review.test.ts` ×50 | **0 every run, 50/50**,14 cases each; `unified-repeat-nice19.log` |
| `git diff --check`; `git merge-base --is-ancestor 26e9aec2 HEAD`; `git merge-base --is-ancestor 1aad42d0 HEAD`; `gofmt -l tests/foundation/browser_click_sweep_test.go` | **0 each**, gofmt output empty; `unified-static.log`; static formatting is not Go compilation |

Repeat proof:100 bounded processes /1,400 passing cases, including200 immediate/delayed uncertain-send scenarios,42.796 seconds;30-second per-process timeout plus5-second grace. Per-run immutable SHA, exact command, exit, timeout and counts: `unified-repeat-results.json`. New behavior tests did not replace old assertions.

Independent read-only `security_reviewer`, configured gpt-6.1-sol/high, reviewed source0875e548 and corrected the additional sign-in P2; final review found no remaining confirmed P0/P1/P2 in this scope (Humaux `1b92d328-4079-4b6b-a260-f866692c5692`). It independently checked69 unchanged assertion ASTs and existing Go signatures/wiring but did not execute browser/Go/PG. A/C research, fixes, plan and code links are stored in Humaux. Independent read-only explorer (gpt-6-luna/medium) audited the current delivery against all six gate records,100 repeat rows,699 Node cases,62 component cases,34 hashes,49-mode plan and ancestry/static logs; no factual discrepancy. Its Humaux record is titled `[research] LC-U2b unified DELIVERY evidence audit`. Parent independently verified the same manifests and current source hashes; the final handoff commit changes evidence only.

**Current CI / browser acceptance NOT_RUN:** integrator pushes the handoff to PR8; required foundation-shards and all49 automatic modes include browser-inbox and click-sweep. Re-dispatch same-SHA `LC_INBOX_CALIBRATION=retain-thread`; it must fail INU05, not startup/locale/store/thread mounting. Preserve `output/playwright/inbox-ui/<timestamp>` logs/traces/screenshots and the sweep ledgers. Go compilation/PG, normal/calibration browser, visual QA/build, real clipboard/platform and LIVE remain NOT_RUN locally. These fixes are E3/MOCK code-side evidence, not normal-green/calibration-red acceptance. No push or deployment performed.

## Initial author commands and exits (source b272d802; historical evidence)

All commands run in the assigned worktree unless stated. Logs are retained under this directory; trailing whitespace in captured logs is normalized only.

| Command | Exit / evidence |
| --- | --- |
| `pnpm install --frozen-lockfile` | 0; installed frozen dependencies, no lock edits |
| `node --test --experimental-strip-types tests/admin/inbox-privacy.test.ts` before implementation | 1; `red.log` |
| `node --test --experimental-strip-types tests/admin/inbox-bff.test.ts` before BFF registration | 1; worker `bff-red.log`, real handler returned 404 |
| Same privacy command before each new fix | 1 each; `retry-red.log`, `calibration-red.log`, `hidden-focus-red.log`, `navigation-red.log`, `ticket-scope-red.log` |
| Same real BFF command before response-budget fix | 1; `page-budget-red.log` (valid Unicode page returned 503) |
| `node --test --experimental-strip-types tests/admin/inbox-privacy.test.ts tests/admin/inbox-copy.test.ts tests/admin/inbox-bff.test.ts` | **0, 17/17**; `green.log` |
| `pnpm --filter @live-commerce/admin typecheck` | **0**; `typecheck.log` |
| `pnpm exec tsc --noEmit --strict --target ES2023 --lib dom,dom.iterable,esnext --module esnext --moduleResolution bundler --skipLibCheck --typeRoots apps/admin/node_modules/@types --types node tests/admin/inbox-ui.spec.ts` | **0**; `spec-typecheck.log` |
| `node --test --experimental-strip-types tests/admin/shell-architecture.test.mjs` | **0, 2/2**; `architecture-green.log`; initial formatter red retained in `gates-red.log`, reused shared amount/Taipei-time helpers |
| `bash scripts/dev/check-headers.sh` | **0**; `headers.log` |
| `git diff --check` | **0**; no whitespace findings |
| `bash scripts/dev/test-node.sh` | **0** at source commit b272d802; `node.log`; initial exit 1 preserved in `node-red.log`. Existing optional R04 binary suite reports NOT_RUN, not PASS |
| `bash scripts/dev/check-gates.sh` | **0**, 78 modes / 1218 foundation names; `check-gates.log`; old registry red preserved in `registry-red.log`. Owner-approved expectation follows required navigation and adds staff denial; no threshold/allowance widened |

Worker-only registration/static checks and historical failure commands are preserved in `browser-CI.md` and `bff-green.log`; their source hashes refer to worker commits, not this final tree. Parent independently reran the integrated focused tests/typechecks/architecture/header checks above. Required browser bug injection -> red -> restored green is **NOT_RUN**, so the new browser gate is structurally registered but not calibrated/accepted.

## Deferred availability / NOT_RUN (integrator-approved merge sequence)

1. **Resolved:** owner explicitly approved `tests/admin/shell-registry.test.ts` scope extension. Old owner/messages absent expectation now asserts owner visible plus orders-only staff invisible/direct route denied. Registry target 10/10, full Node and check-gates green; red logs retained.
2. Correct admin locales are zh-TW/zh-CN/en; INU06 and INU07 exercise all three at both widths. No shared locale package changes or outstanding locale prerequisite.
3. Frozen A13 `internal/inbox/read.go:262` still returns empty claims/orders, ordinal 0, no display name/auto-reply; A8/A9/A13 expose no link version for A14. Honest empty/unavailable states are rendered; manual customer-link controls remain disabled with a reason. No guessed version or read-via-write. The integrator confirmed these real gaps and opened LC-B3b to supply them; LC-U2b merges first, then a small follow-up enables controls. No field is fabricated.
4. A8 validates session_id but does not use it in the query; live_comment projection is absent. Flagged bundle-only rows now use their actual A8 session_id plus the existing claims M6 generation and M7 link issuance to offer only Copy claim link beside A13. No conversation reply or inferred buyer/deep link is added.
5. A9 exposes no binding id for capability matching. Per the latest ruling, exactly one provider binding uses its named dm_session hard gate (ok/review_required with badge); several distinct bindings are advisory with a warning and A12/Check decides the scoped refusal. No/unknown health or a single binding without named DM stays disabled. Go is authoritative.
6. Browser, Go runtime tests, PG runtime, visual QA/screenshots and current normal/calibration runs are NOT_RUN locally under the owner RAM rule; prior 08614fb6 normal/calibration CI both FAILED without retained Playwright evidence. CI harness currently targets headed Chromium; WebKit for this new mode is NOT_RUN.

## Integrator-ruling follow-up (source 13ea07a8)

- Corrects the brief to the actual zh-TW/zh-CN/en admin locales; no shared locale package edit. Empty/unavailable backend states and disabled missing-version link controls remain unchanged pending LC-B3b.
- Registry now additionally asserts exact `/messages` and `inbox:read`, retaining owner admission and orders-only staff denial.
- CN golden-label RED: `node --test --experimental-strip-types tests/admin/inbox-copy.test.ts` -> **1** (`ruling-copy-red.log`: zh-CN incorrectly returned Traditional Chinese). After corrected CN: `node --test --experimental-strip-types tests/admin/inbox-copy.test.ts tests/admin/shell-registry.test.ts` -> **0**, 11/11 (`ruling-focused-green.log`).
- `pnpm --filter @live-commerce/admin typecheck` -> **0**, `ruling-typecheck.log`.
- The strict Playwright-spec TypeScript command above -> **0**, `ruling-spec-typecheck.log`.
- `bash scripts/dev/test-node.sh` -> **0**, `ruling-node.log`; `bash scripts/dev/check-gates.sh` -> **0**, `ruling-check-gates.log` (78 modes, 1218 test names). Both pinned to 13ea07a8, supervised by Python stdlib with a 300-second timeout, 5-second owned-group termination grace, no source edits during execution. Optional R04 binary suite remains NOT_RUN.
- Read-only reviewer (same security_reviewer configuration) found no new confirmed P0/P1 in this correction; source evidence Humaux `ee1a0d10-2ab9-49cf-abf1-1aefc9609787`. Browser/runtime behavior remains NOT_RUN. Normal --browser-inbox and retain-thread calibration run on the PR, by the integrator.

## PR #8 Opus privacy fixes (source 08614fb6)

- P1-1: window focus/pageshow while already visible now only checks the session. Only a real hide permits reveal, epoch refresh and private clearing; unchanged-session focus preserves the selected conversation, draft, pending A12 signal and immutable receipt. Changed CSRF/session still revokes immediately; late session checks after unmount cannot clear a new scope.
- P1-2: a common terminal `clearGone` handles A9/A10/A11/A12 404, clearing DM/name display, draft, templates, delivery and reply receipt before invalidating follow-up work. `finally` cannot re-read after that epoch ends; a later 503/network failure cannot resurrect old text. Captured stale action/send callbacks are also refused.
- Cheap P2s: non-JSON upstream 401 clears both auth cookies before body parsing; A14 404 clears BuyerPanel facts and the customer draft. Existing missing-version disabled controls remain unchanged.
- RED before production: `node --test --experimental-strip-types tests/admin/inbox-privacy.test.ts` -> **1**, `pr8-focus-red.log`; actual visible reveal invalidated the ticket. Real Request BFF command -> **1**, `pr8-cookie-red.log`, opaque 401 returned 503 without revocation.
- Independent test-only worker commits `2327ae88`/`38713611`, gpt-6.1-sol high test_worker, separate `.worktrees/lc-u2b-pr8-tests`: original actual hook/component/client tests **6 PASS / 5 RED** and the two write-404 countercases **2 RED**, retained in `pr8-components-red.log` / `pr8-write404-red.log` with baseline hashes. Production privacy logic was never copied into the driver; React hook scheduling and fetch only are MOCK. Future-version BuyerPanel fixture is explicitly MOCK, never a claim that frozen Go publishes a version.
- `node --test --experimental-strip-types tests/admin/inbox-review.test.ts` -> **0, 13/13**, `pr8-components-green.log` (actual TSX/hook/client/Fence/Receipt). Includes focus/pending send, same-session pageshow, true hide/revalidation, changed-session revocation, read/send-finally/write 404, immutable retry, stale completion, A14 404 and missing-version guard. Original assertions retained; the suite is registered in `test-node.sh`.
- `node --test --experimental-strip-types tests/admin/inbox-privacy.test.ts tests/admin/inbox-copy.test.ts tests/admin/inbox-bff.test.ts` -> **0, 19/19**, `pr8-focused-green.log`.
- `pnpm --filter @live-commerce/admin typecheck` -> **0**, `pr8-typecheck.log`. `pnpm exec tsc --noEmit -p output/lc-u2b-inbox-page/pr8-test-tsconfig.json` -> **0**, `pr8-review-project-typecheck.log`; it extends the real admin strict config. Initial standalone test compile -> **1** for missing @/ path configuration, retained in `pr8-review-typecheck.log`; no type threshold was relaxed.
- `bash scripts/dev/test-node.sh` -> **0**, `pr8-node.log`; `bash scripts/dev/check-gates.sh` -> **0**, `pr8-check-gates.log`. Both run on immutable source 08614fb6 under the same 300-second owned-group supervisor documented above. Optional R04 binary suite remains NOT_RUN.
- Targeted read-only security recheck found the missed A11/A12 write-404 retention; added red tests and shared cleanup solved it. Final recheck found no further confirmed P0/P1 in that fix, Humaux `2ec82daf-c65f-438c-a542-b0e62ca26939`. Opus final review/PR runtime acceptance still pending.
- BROWSER/Go/PG/build/visual/provider modes remain **NOT_RUN** locally. `--browser-inbox` and `LC_INBOX_CALIBRATION=retain-thread` run on PR #8 after the integrator pushes. The intentional true-hide defect remains intact and must fail INU05 specifically; none of its assertions changed.

## PR #8 browser failure evidence repair (source aa346aa7)

- CI observed at 08614fb6: normal [37627918482](https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/actions/runs/37627918482) and calibration [37627924858](https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/actions/runs/37627924858) both failed `TestBrowserInboxUIRealChain` with only a generic Playwright exit 1. `gh run view 37627918482 --log-failed` and `gh run view 37627924858 --log-failed` each exited **0**; saved as `ci-37627918482.log` / `ci-37627924858.log`. Captured logs normalize trailing whitespace only; failure text and timestamps are retained. No failing INU case or production-flow root cause can be recovered from these logs.
- `tests/foundation/browser_inbox_ui_test.go:124` now writes all owned evidence to `output/playwright/inbox-ui/<UTC timestamp>`: Next/Playwright logs, click ledger, native visibility records, screenshots and Playwright result/trace files. Existing `gates.yml` always-upload scope already covers this directory. The log is closed before failure parsing (`:239`); Go logs expose at most 20 records of test counts, failed INU IDs and source coordinates, never arbitrary titles/assertions/DOM text. Missing/no-summary output gets a fixed diagnostic pointing to the artifact. The actual Go summary fixture runs as the existing browser test's `failure_diagnostics` subtest on CI; **Go compilation/runtime NOT_RUN locally**.
- Independent read-only review found a greedy failure-header parser P2. A title containing a second source coordinate/INU token selected the wrong case; `ci-evidence-parser-red.log` shows **1** at 84aa157c. The parser now anchors the reporter path/separator/first INU token; the strengthened Node counterexample and the actual Go fixture preserve the original coordinate/INU case. `ci-evidence-parser-green.log` shows **0, 5/5**. No test assertion was weakened.
- Source-reading counterexample: INU04's two actual delayed A9 callbacks did not release their completion promises if canceled-request fulfillment threw. The Node gate transpiles and invokes the actual callbacks with a MOCK rejected fulfillment; `ci-evidence-red.log` is **1, 5 RED** against merge baseline 26e9aec2. Both callbacks now release in `finally` while propagating the original error; all stale-response and hidden-thread browser assertions remain. This is a reproduced harness wait defect, **not proof that it caused either lost CI failure**.
- Authorized `git fetch origin && git merge origin/r3/integration` initially exited **1** for three content conflicts. Preserved both exact inbox/operations BFF permissions and no-store branches, and both inbox/operations-ads gate modes. `bash -n scripts/dev/test-node.sh scripts/dev/test-local.sh` -> **0**, `git add <three resolved paths>` -> **0**, `git commit --no-edit` -> **0**, merge **26e9aec2** includes fetched trunk **1aad42d0**. The retrospective whole-merge `git diff d868bd68 26e9aec2 --check` -> **2**, `ci-evidence-merge-check.log`, for pre-existing whitespace in incoming historical evidence logs; those logs were retained. The final unit source diff check is **0**.
- Initial new-test driver setup failures (wrong TypeScript facade and generated function return) are retained, with source dumps omitted, in `ci-evidence-test-setup.log`; they are not acceptance evidence. Corrected test driver uses the installed `typescript-api`. The strict test project now also covers the new evidence test and the actual browser spec without starting a browser.

Final local checks are pinned to **aa346aa741df9b50baa66e9002b3e3efde65788b**, source unchanged during execution. Each owned process group has a 300-second timeout and a 5-second termination grace. `source-hashes.txt` binds 27 unit files, including the merged shared route/runners/workflow.

| Exact command | Exit / evidence |
| --- | --- |
| `node --test --experimental-strip-types tests/admin/inbox-evidence.test.ts` | **0, 5/5**, `ci-evidence-green.log`; initial **1** in `ci-evidence-red.log`, parser **1** in `ci-evidence-parser-red.log` |
| `bash scripts/dev/test-node.sh` | **0**, `ci-evidence-node.log`; optional R04 binary remains explicitly NOT_RUN |
| `bash scripts/dev/check-gates.sh` | **0**, `ci-evidence-check-gates.log`; 79 documented modes / 1219 top-level foundation tests, header ratchet green |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | **0**, `ci-evidence-admin-typecheck.log` |
| `pnpm exec tsc --noEmit -p output/lc-u2b-inbox-page/pr8-test-tsconfig.json` | **0**, `ci-evidence-test-typecheck.log` |
| `gofmt -w tests/foundation/browser_inbox_ui_test.go` | **0**, `ci-evidence-static.log`; static formatting only, source hash unchanged |
| `git diff --check` | **0**, `ci-evidence-static.log`; unit delta only |

Command outcomes, source SHA and timeout status are also in `ci-evidence-gates.json`; final read-only delivery audit checked the command records, and its missing static evidence/merge-command mismatch are corrected in `ci-evidence-static.log` and this section. `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` -> **0**, `ci-evidence-pr-plan.json` (49 modes, deploy=false). Independent security_reviewer (gpt-6.1-sol high) read-only final review at aa346aa7 found no remaining confirmed P0/P1/P2 in the evidence/parser/barrier/merge scope; Humaux `e2954fa3-6c8a-4e2c-91de-cf2966e71792`. That review did not independently execute Go/browser/PG or recover the lost runtime failure. Backend gaps remain deferred to LC-B3b; no Go backend/migration/OpenAPI or API-contract changes. Current normal browser green and INU05 calibration red remain **NOT_RUN locally / required on GitHub**.

## PR #8 Codex-review bundle recovery and capability ruling (source e5a70499)

- **P1 / 4207191773**: `Inbox.tsx:310` now opens the existing A13 BuyerPanel with a keyed `BundleRecovery` beside it. The only recovery control copies a claim link; it never renders a Send or release-binding action. It reuses existing `readClaimBundles` cursor pages to find the exact bundle's generation, existing catalog/storefront-origin helpers, and `issueClaimLink` (`bundle-recovery.tsx:92`) through the current live-session M7 BFF route. No new BFF path or guessed CAS value. Existing `live:read`/`live:manage` and catalogue authority remain enforced by the actual clients/API; missing identity/origin/grants shows a fixed reason.
- The issued URL exists only in a component ref and the authorized clipboard (`:102`), never DOM attributes/text, request/navigation URLs, browser storage or logs. Clipboard denial allows recopy of the same cached link without another POST. UNKNOWN retains the original key, frozen generation/release=false body, origin and session boundary; only explicit same-request retry is offered. A replay's token=null cannot recover the credential, so this scope stops and displays a fixed warning without a fresh key. Hide, logout, keyed selection/store/locale change and unmount drop receipts/URLs; every asynchronous completion checks the fence before copying. Missing CSRF also revokes cached copy and retry.
- **P2 / 4207466870**: `InboxThread.tsx:79` counts distinct binding_id among all capability rows for the conversation's provider. One binding gates on named DM rows being ok/review_required, with the review badge. Several bindings show `inbox-capability-advisory` and allow a valid open-window reply; other provider rows do not affect the count. Missing health/single unnamed DM stays disabled; A12's scoped 409 capability uses the fixed copy. A9 exact binding_id remains deferred to LC-B3b.
- **P2 / 4207191782**: BuyerPanel has **no source delta** from 4ba673ba. A14 controls stay disabled with the missing-version reason; LC-B3b will expose link_version. No fake field or guessed version.
- **Prior requested changes included**: merge **26e9aec2** includes trunk **1aad42d0**; evidence stays under `output/playwright/inbox-ui/<timestamp>` and Go prints sanitized Playwright failure summaries. Previous CI failures and the artifact repair red→green are retained in the preceding section. No additional Go/backend, migration, OpenAPI, dependency-lock or BFF changes in this follow-up.
- Independent test_worker gpt-6.1-sol high used clean `.worktrees/lc-u2b-pr8-tests`, branch `codex/lc-u2b-review2-tests`, base 4ba673ba; test-only commits b62ffcbb/e42fdbb4 were applied in the parent. The generic hook/child host was split from the old >600-line test: original 13 test AST/assertions unchanged. New 20 behavior cases were **8 PASS / 12 RED, exit 1** before implementation (`review2-test-red-final.log`; initial 18-case log retained). Two missing-CSRF cases were then added; their old baseline cannot reach the absent recovery feature, explicitly recorded as guard NOT_RUN rather than a demonstrated CSRF bug (`review2-test-csrf-red.log`). Captured red logs normalize only trailing whitespace and retain failures.

Independent source review found a cached-expiry P2: M7 expires_at was discarded, so a denied first clipboard attempt could later copy an expired URL. Test-only worker 2c5a6b37 demonstrated **exit 1, 1 RED** against 75d589d8 before the guard (`review2-expiry-red.log`). The ref now keeps expires_at and checks it before any copy; expiration clears the credential, displays a fixed reason and stops this scope without issuing again. A strengthened UNKNOWN case also proved a captured old retry callback could mint a fresh key after token-null replay at 75d589d8; `review2-stopped-red.log` is **1, 2 RED** for these countercases. A terminal ref now blocks both cached and captured callbacks before React commits disabled state. All prior assertions remain; `review2-expiry-green.log` shows **0, 36/36**. No original CI root cause is inferred.

Final permitted checks below ran on immutable **e5a70499a204c2e227bfc49ed7527a7b90596860** with 300-second owned-process timeout and 5-second termination grace; source unchanged during execution. `source-hashes.txt` now binds 30 unit files. Exact argv/source/exit/timeout records: `review2-gates.json`.

| Exact command | Exit / evidence |
| --- | --- |
| `node --test --experimental-strip-types tests/admin/inbox-review.test.ts tests/admin/inbox-review-recovery.test.ts` | **0, 36/36**, `review2-components-green.log`; new 23 plus original 13 actual-source cases |
| `bash scripts/dev/test-node.sh` | **0, 678/678**, `review2-node.log`; optional R04 binary explicitly NOT_RUN |
| `bash scripts/dev/check-gates.sh` | **0**, `review2-check-gates.log`; 79 modes / 1219 foundation names / header ratchet |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | **0**, `review2-admin-typecheck.log` |
| `pnpm exec tsc --noEmit -p output/lc-u2b-inbox-page/pr8-test-tsconfig.json` | **0**, `review2-test-typecheck.log` |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` | **0**, `review2-pr-plan.json`; 49 automatic required modes, deploy=false |

Final read-only delivery audit matched all five exit/source/timeout records, 36 component cases, 678 Node cases, 30 hashes and 49-mode PR plan; no factual mismatch. `git diff --check` and ancestry checks for 26e9aec2/1aad42d0 each exited **0**, captured in `review2-static.log`.

Final read-only independent security_reviewer (gpt-6.1-sol high) at e5a70499 found no remaining confirmed P0/P1/P2 in this scoped recovery/capability change; Humaux `3a98fcf4-5d0a-4d0d-8c04-cd898800211c`. The review verified unchanged/strengthened assertion sequences and source hashes, and did not independently run browser/Go/PG or real clipboard acceptance.

The imported `inbox-review-host.test.ts` has no standalone assertions; its actual use by both review suites is documented in the Node runner, without adding a vacuous test run. Evidence is **E3 / MOCK for Node actual-source flows**, **E1 for typecheck/registration**; browser/Go/PG/build and visual/exported screenshot checks remain **NOT_RUN locally**. Required GitHub normal gates and same-SHA retain-thread calibration remain pending; calibration must fail INU05, with preserved evidence. Backend A13/A14 availability work remains deferred to LC-B3b.

## PR #8 Foundation Node timing failure (source 66dd7b03)

- Observed [Foundation checks 37632486132](https://github.com/luogangan7-lgtm/live_commerce_architecture_v1/actions/runs/37632486132) at the integrator-pushed d313d97c: one uncertain-send test failed with actual 1 / expected 2 POSTs. `gh run view 37632486132 --log-failed` -> **0**, `nodeflake-ci-37632486132.log`. The test now lives at the corresponding case in the split `inbox-review.test.ts`, with the bundle P1/binding P2 handoff already retained.
- Source chain: `Host.settle()`'s 12 setImmediate turns cannot guarantee WebCrypto/A9/A10 completion. Send finally briefly sets busy=false then immediately starts authority reload; a premature second submit meets the real inFlight guard. Independent test_worker ee4e48bb, clean `codex/lc-u2b-nodeflake-tests` worktree from 30ddeedc, parameterized only the existing uncertain-send case with immediate and 50 ms post-send MOCK A9. Before the waiter change: **exit 1, 1 PASS / 1 RED**, `nodeflake-red.log`; latest submit helper correctly catches busy earlier than the old remote 1!=2 assertion. The delay is adversarial fixture I/O, never a waiter sleep. This reproduces the readiness defect; the remote count alone does not independently prove the full CI schedule.
- `inbox-review-host.test.ts:93` adds a bounded root-host `waitFor` driven by real state-setter commit notifications. A queued microtask coalesces send-finally's transient busy=false followed by reload=true; success/timeout/disposal clean observers/timers. The target waits for initial authorized A9+A10 completion, then full first-send revalidation and enabled reply control, then retry revalidation. It uses **no fixed ticks**. Exact 2 sends, identical bodies/keys and the same receipt object remain asserted for both immediate/delayed cases. The generic helper's unrelated settle checkpoints remain unchanged; root-only event waiting is not claimed to observe nested child commits.
- The first candidate readiness predicate incorrectly required an enabled reply before the initial empty draft; its explicit timeout failure is retained in `nodeflake-waiter-initial-condition.log` as setup evidence. The corrected initial predicate waits for loaded DM plus idle A9/A10, then the existing input/action assertions remain. Focused immediate/delayed tests now **0, 2/2**, `nodeflake-green.log`.
- **Repeated proof on immutable 66dd7b03:** 50 complete 14-case review-suite processes normally and 50 under `nice -n 19`, all **exit 0, 14/14 each**, totaling **1,400 passing cases** (200 uncertain-send scenarios). Logs: `nodeflake-repeat-normal.log`, `nodeflake-repeat-nice19.log`; per-run source/exit/timeout/count records and exact argv: `nodeflake-repeat-results.json`. Elapsed 62.255 seconds. Each process had a 30-second timeout and 5-second termination grace; supervised exec55893 ended.

| Exact command | Exit / evidence |
| --- | --- |
| `node --test --experimental-strip-types tests/admin/inbox-review.test.ts` (50 runs) | **0 each, 50/50**, `nodeflake-repeat-normal.log` |
| `nice -n 19 node --test --experimental-strip-types tests/admin/inbox-review.test.ts` (50 runs) | **0 each, 50/50**, `nodeflake-repeat-nice19.log` |
| `node --test --experimental-strip-types tests/admin/inbox-review.test.ts tests/admin/inbox-review-recovery.test.ts` | **0, 37/37**, `nodeflake-components.log` |
| `bash scripts/dev/test-node.sh` | **0, 679/679**, `nodeflake-node.log`; optional R04 binary NOT_RUN |
| `bash scripts/dev/check-gates.sh` | **0**, `nodeflake-check-gates.log`; 79 modes / 1219 foundation names / headers |
| `pnpm --filter @live-commerce/admin exec tsc --noEmit` | **0**, `nodeflake-admin-typecheck.log` |
| `pnpm exec tsc --noEmit -p output/lc-u2b-inbox-page/pr8-test-tsconfig.json` | **0**, `nodeflake-test-typecheck.log` |
| `node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` | **0**, `nodeflake-pr-plan.json`; 49 modes, deploy=false |

All final local gates ran on unchanged **66dd7b038728ad72d5def0e78deef2b48a3d03a0**, supervised exec45647 with 300-second timeout and 5-second grace; `nodeflake-gates.json` records exact commands/outcomes. Thirty unit source hashes are current. Independent read-only source review `9a7a7c17-33fc-4f41-a551-6f33eccfeb21` found no confirmed scoped P0/P1/P2 and no weakened count/body/key/receipt assertions; it did not independently run tests or establish a remote runtime cause.

Final read-only evidence audit matched the 100-run manifests/logs, five final command records, all counts and source hashes with no factual mismatch. `git diff --check` and ancestry checks for 26e9aec2/30ddeedc each exited **0**, captured in `nodeflake-static.log`.

Production code has **no delta from 30ddeedc**: bundle-link P1 + UNKNOWN/expiry guards, provider-binding P2, A14 disabled reason, trunk merge **26e9aec2**, and `output/playwright/inbox-ui` + sanitized Go failure logs are all included. Evidence is **E3 / MOCK for local Node**; current browser/Go/PG/build/real clipboard and same-SHA INU05 calibration remain **NOT_RUN locally**. Integrator pushes this combined handoff and runs the required set. No push, production or live-key action by the author.

## Independent work and custody

Base for all worktrees: f7340515. No recursive delegation; interfaces frozen before UI/BFF wiring. Workers committed locally; parent applied their patches in its own branch.

| Role / configured model+effort | Worktree / ownership |
| --- | --- |
| Codex-4 / parent exact variant unknown | assigned unit worktree; page, Inbox/Thread/BuyerPanel, feature privacy/copy/styles/routes and final evidence |
| ui_worker / gpt-6.1-sol high | `.worktrees/lc-u2b-bff`, `codex/lc-u2b-bff`; lib/inbox-types/client/bff, catchall, real-handler test; commit b9139ad1 |
| test_worker / gpt-6.1-sol high | `.worktrees/lc-u2b-browser`, `codex/lc-u2b-browser`; Go/spec + mode/runner/workflow/Playwright registrations; commits b396f8da, fc9bd6c3 |
| explorer / gpt-6-luna medium | read-only frozen DTO/ingress audit; no writes |
| platform_explorer / gpt-6.1-sol medium | read-only Go helper/schema/permission audit; no Go execution |
| security_reviewer / gpt-6.1-sol high | read-only privacy review; receipt, hidden-event and terminal navigation findings fixed; final targeted source review found no further confirmed P0/P1. Not runtime acceptance |

Humaux parent task `d378c85f-2030-4047-8dc9-e297bc45bfd4`; charter/fixes/problems stored, canvas updated and code indexed/linked. Independent terminal recheck memory `cd8d0b66-e163-4a57-8612-2a405d62faa1`. Opus/K3 acceptance still required. No active author processes; own transient patch/index fixtures removed; dependencies and worker commits preserved. No SSH, production host, live keys or real customer messages used.

## CI gates (integrator; current source NOT_RUN)

PR #8 already exists. The integrator pushes this commit; the fetched PR-triggered workflow automatically runs `foundation-shards` and the full CI-runnable browser set selected by `scripts/dev/pr-modes.mjs`, including `--browser-inbox`, admin-shell, click-sweep and visual-lint. Require the stable `Gates (GitHub runners) / required` result. LC-U2b remains allowed to merge before LC-B3b. Pin the PR SHA, register run IDs/log paths with a timeout and completion notification; current-source CI is NOT_RUN by the author. Relevant commands:

```sh
bash scripts/dev/test-node.sh
pnpm --filter @live-commerce/admin typecheck
pnpm run build:admin
bash scripts/dev/check-gates.sh
bash scripts/dev/test-local.sh --browser-inbox
LC_INBOX_CALIBRATION=retain-thread bash scripts/dev/test-local.sh --browser-inbox
bash scripts/dev/test-local.sh --browser-admin-shell
bash scripts/dev/test-local.sh --browser-click-sweep
bash scripts/dev/test-local.sh --browser-visual-lint
```

Normal gates run automatically on the integrator's PR push; dispatch calibration separately with `extra_env='LC_INBOX_CALIBRATION=retain-thread'`. Calibration must fail **INU05 hidden thread retains private DM** after a trusted native hide, not startup/compile/locale failure. Dedicated loopback acceptance + loopback-test flag gate the intentional defect; generic fixture bypass stays off. Preserve normal-green/calibration-red at identical SHA and require restored normal green. Expected artifacts: next/browser logs, click ledger, native visibility record, 1440/390 screenshots and Playwright traces. Current-source artifacts must be retained under `output/playwright/inbox-ui/<timestamp>`; prior 08614fb6 Playwright artifacts were lost. Neither prior failure qualifies as normal-green or INU05 calibration-red acceptance.
