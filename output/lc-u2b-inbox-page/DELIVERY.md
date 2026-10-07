# LC-U2b inbox page + BuyerPanel delivery

- Branch: `unit/lc-u2b-inbox-page`; worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u2b-inbox-page`.
- Base: `f73405150a1cca3c588d4546f3b5a8e0d1a456a3` (owner-assigned base overrides the older brief reference). Tested source commit: `aa346aa741df9b50baa66e9002b3e3efde65788b`; authorized trunk merge: `26e9aec2` of `1aad42d0`; final evidence commit is reported in the author handoff. Source is also bound by `source-hashes.txt`.
- Author: Codex-4, GPT-6-based Codex; exact parent model/effort not exposed. Only the requested local trunk merge; no push/deployment.
- Evidence: **E3 / MOCK for actual-source Node component/hook and delayed-route counterexamples; E1 for TypeScript and harness/parser registration. Author scope follows the integrator ruling; browser acceptance is pending CI, not locally claimed. Backend feature availability is deferred to LC-B3b and does not block LC-U2b merging first. Current source BROWSER / REAL_PG / SANDBOX / LIVE NOT_RUN locally. The two prior browser CI failures at 08614fb6 are recorded below, without a runtime acceptance claim.**

## Summary

- `apps/admin/app/[locale]/messages/page.tsx:19`, `components/Inbox.tsx:21`: authenticated store entry, messages registry, list filters/cursor paging and memory-only selection. SSR serializes no buyer/thread data.
- `components/InboxThread.tsx:21`: A9 paging, A10 only with reply permission, keyed A11/A12, authoritative generation/window and fixed delivery/errors; published templates and explicit dm_session capability checks.
- `components/BuyerPanel.tsx:17`: standalone store + conversation/bundle + callbacks, independent hide/session fence; orders gated by orders:read; no inferred link CAS version.
- `src/features/messages/privacy.ts:8`, `use-privacy.ts:11`: abort epochs, controller-bound tickets, synchronous hide clearing, terminal navigation revocation; exact immutable receipt on uncertain retry. Normal Refresh retains unknown delivery.
- `lib/inbox-bff.ts:15`, catchall route: precise A8–A14/GET templates, strict method/query/body, real Request stream/auth/CSRF tests, no-store and fixed safe errors. Bounded 1 MiB admits 50 valid Unicode messages and refuses excess.
- `tests/foundation/browser_inbox_ui_test.go:31`, `tests/admin/inbox-ui.spec.ts:157`: registered `--browser-inbox`, signed MOCK OIDC + real Next/Go/PG, real click/PG readback, privacy/cross-store/read-only/window/capability cases and committed-send/lost-ACK same-receipt retry. INU06/INU07 cover zh-TW, zh-CN and en at 1440/390 with independent literal labels, real locale changes and mobile Back clearing.

Contract/interface changes: **none**. Additional write scope: one shell-registry expectation update explicitly approved by the owner. No backend Go, migrations, OpenAPI, module or dependency lock changes; Go acceptance test harness is the brief's explicit exception. `playwright.config.ts` gets the necessary suite registration. zh-TW/zh-CN/en unit copy is complete per the integrator correction; packages/i18n is unchanged.

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
4. A8 validates session_id but does not use it in the query; live_comment projection is absent. Bundle-only rows have no usable conversation/source-link URL: read-only bundle panel and explicit unavailable copy-link control. No fabricated deep link or reply.
5. A9 exposes no binding id for capability matching. UI conservatively requires every matching-provider named dm_session row to be ok/review_required; missing/unknown blocks sends, review_required shows the app-role-only badge. Go is authoritative.
6. Browser, Go compilation, PG runtime, visual QA/screenshots and current normal/calibration runs are NOT_RUN locally under the owner RAM rule; prior 08614fb6 normal/calibration CI both FAILED without retained Playwright evidence. CI harness currently targets headed Chromium; WebKit for this new mode is NOT_RUN.

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
