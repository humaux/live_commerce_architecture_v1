# Historical worker evidence — superseded by parent DELIVERY

These notes bind the worker commits only. Integrator subsequently corrected the brief: current INU06/INU07 use zh-TW/zh-CN/en at 1440/390; all Japanese references below are superseded historical worker notes. Parent also corrects actual shell testids, adds terminal navigation revocation, and strengthens hidden DOM assertions. Final source hashes, commands and acceptance state are in DELIVERY.md/source-hashes.txt below. No browser runtime was executed.

# LC-U2b browser CI harness delivery

- task_id: LC-U2b-browser; parent task: d378c85f-2030-4047-8dc9-e297bc45bfd4
- base_commit: f73405150a1cca3c588d4546f3b5a8e0d1a456a3
- branch/worktree: codex/lc-u2b-browser; /Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u2b-browser
- Role: independent browser harness author. Codex/GPT-6 family; exact runtime model/reasoning: UNKNOWN (not exposed).
- Evidence: E1 structure/typechecking only. BROWSER MOCK/REAL_PG runtime: NOT_RUN. Author checks do not replace independent acceptance.

## Implementation and paths

Only seven allocated paths changed: tests/foundation/browser_inbox_ui_test.go, tests/admin/inbox-ui.spec.ts, playwright.config.ts, scripts/dev/test-local.sh, scripts/dev/test-node.sh, docs/delivery/GATES.md, .github/workflows/gates.yml.

The Go test uses existing lbSetup signed encrypted ingress and fake Graph dispatcher, the actual inbox service/router and Meta health reader, production Next and signed MOCK OIDC login. Catalogue-derived live_operator and viewer identities plus an explicit read-only principal prove the permission split. Real UI clicks read/mark/takeover/send/release and refresh/reopen assert real PG state. The fixture queues the ordinary send and an ambiguous-ack retry flow through the existing dispatcher; real PG requires exactly one outbound per conversation, Graph requires exactly two calls with exact synthetic recipients. The retry drops the first response only after route.fetch commits in real Go/PG, then clicks Retry this submission and verifies byte-identical body/key and identical operation/outbound IDs. URLs/storage/browser console/Next logs are scanned for synthetic DM/name/PSID/reply sentinels. Native Chromium tab switching proves trusted hide and a real click on reveal reauthorizes before painting; delayed genuine responses test conversation and shell-store invalidation.

Two supported route locales (zh-TW/en) are exercised at 1440/390. INU07 Japanese is explicitly SKIP/BLOCKED: global LocaleLayout does not admit ja. No mock Japanese route was fabricated. Runtime calibration is passed with LC_BROWSER_INBOX_ACCEPTANCE=1, COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS=1 and exact http://127.0.0.1:<port> origin. COMMERCE_FIXTURE_ENABLED=0 preserves the signed-identity prohibition on generic fixture bypass. Root owns the guard test; the browser spec never branches to force a failure.

## Actual local commands and exits

- Registration RED, before implementation: `node --input-type=module -e 'import assert from "node:assert/strict";import{readFileSync}from"node:fs";assert.match(readFileSync("scripts/dev/test-local.sh","utf8"),/test_mode.*--browser-inbox/);assert.match(readFileSync("playwright.config.ts","utf8"),/"inbox": \["inbox-ui.spec.ts"\]/);assert.match(readFileSync("docs/delivery/GATES.md","utf8"),/\| `--browser-inbox` \|/)'` -> exit 1. Observed AssertionError ERR_ASSERTION: test_mode.*--browser-inbox missing. This is registration RED, not a UI runtime RED.
- Same registration command after registration -> exit 0.
- `pnpm install --offline --frozen-lockfile --ignore-scripts` -> exit 0 (49 packages reused; no lock changes).
- `gofmt -w tests/foundation/browser_inbox_ui_test.go` -> exit 0; syntax/format only, no Go build/test.
- `node --check --experimental-strip-types tests/admin/inbox-ui.spec.ts` -> exit 0.
- `bash -n scripts/dev/test-local.sh`; `bash -n scripts/dev/test-node.sh`; `git diff --cached --check` -> exit 0 each.
- Initial strict spec typecheck with `--types node` but no `--typeRoots` -> exit 1, TS2688 cannot find node types at root. Correct dependency scope used below; failure retained here.
- `pnpm exec tsc --noEmit --strict --target ES2023 --lib dom,dom.iterable,esnext --module esnext --moduleResolution bundler --skipLibCheck --typeRoots apps/admin/node_modules/@types --types node tests/admin/inbox-ui.spec.ts playwright.config.ts` -> exit 0 (final changed spec checked).
- `pnpm --filter @live-commerce/admin typecheck` -> exit 0 (browser worktree baseline admin, UI implementation not merged yet).
- `bash scripts/dev/check-gates.sh` -> exit 0 after staging. Actual ending: check-gates ok (78 modes, all documented; every tracked test file is run); check-headers OK base f73405150a1cca3c588d4546f3b5a8e0d1a456a3. Existing >500-line warnings remain warnings. No production logic was changed.
- `bash scripts/dev/test-node.sh` -> exit 1. Existing suites ran; final failure: Could not find tests/admin/inbox-privacy.test.ts, tests/admin/inbox-copy.test.ts, tests/admin/inbox-bff.test.ts. These root-owned files were registered by parent instruction and are not yet in this worktree. R04 input suite also reports NOT_RUN because its binary is unset. Full Node acceptance must be rerun after integration; do not claim green from this partial run.

## CI commands (NOT_RUN)

Integrator must merge UI/BFF/tests and this branch, then pin the resulting SHA. On a GitHub runner:

```sh
bash scripts/dev/test-local.sh --browser-inbox
LC_INBOX_CALIBRATION=retain-thread bash scripts/dev/test-local.sh --browser-inbox
```

Workflow dispatch (integrator pushes; this agent does not push):

```sh
gh workflow run gates.yml --ref <integrated-branch> -f modes='["--browser-inbox"]'
gh workflow run gates.yml --ref <same-integrated-branch> -f modes='["--browser-inbox"]' -f extra_env='LC_INBOX_CALIBRATION=retain-thread'
```

Normal run must green for the supported route subset. Calibration must exit nonzero specifically at `INU05 hidden thread retains private DM`, with private DM still present after a trusted native hidden event. Generic startup/compile/timeouts are not calibration. Preserve both run IDs/artifacts and confirm exact same SHA; restore normal mode and rerun normal if code changes. Expected runtime artifacts: output/lc-u2b-inbox-page/browser/<run>/next.log, playwright.log, browser-console.json, native-visibility.json, click-ledger.json, screenshots/traces. Those artifacts do not exist yet: NOT_RUN.

Required affected regressions after integration, CI only: --browser-admin-shell, --browser-click-sweep, --browser-visual-lint; normal build:admin; Go browser compilation/PG chain. Local Node/typecheck/check-gates must independently rerun on the merged code.

## Unresolved / custody

- Japanese global route admission BLOCKED (outside allocated write paths). The brief's three-route-locale acceptance is incomplete.
- Browser/Go compile/PG/normal-green/calibration-red/affected browser regressions are NOT_RUN under owner RAM rule.
- Full Node gate currently fails missing root-owned files; preserve the failure, integrate them and independently rerun.
- BuyerPanel customer-link version gap is parent-owned BLOCKED; no unsupported live customer mutation was added to this test.
- Humaux memory/coord tools were not callable (ALL_TOOLS discovery yielded none). Store/subcanvas are NOT_RUN; parent must preserve shared leave-behind if it has access. No child task was claimed and no further agent delegated.
- No Go/PG/browser background processes were launched locally; Node/install/static checks exited. Own fixture/Next/native browser processes are bounded/cleaned by the future CI harness. No other task paths or lockfiles changed.

Follow-up: actual list/thread Refresh, Older messages and responsive Back controls are clicked and their real API/visible results asserted. Updated spec strict typecheck command above and gofmt returned exit 0 after the ambiguous-ack/calibration changes. No runtime gate executed.

## Committed source binding

- commits: b396f8da54a572e4ca895972c464cf828fe9b489 (registry/harness) then fc9bd6c37570733aa77886f457f402e8bd29719a (incremental retry/guard/mobile controls).
- working tree: clean after commits; no push.
- SHA256 `tests/foundation/browser_inbox_ui_test.go`: `c51fe528bc688bd652569091323ad2282b156d725c93fcc1744e7c5127b747ca`
- SHA256 `tests/admin/inbox-ui.spec.ts`: `b36e2bb1cf6385bcb8bf516101ec23f61d4423dbf924fbf3321c5fad22819f0b`
- SHA256 `playwright.config.ts`: `6605634f20629df4290f870db290b2b78335c8344ff40d1c5df6ef23e2e7a64b`
- SHA256 `scripts/dev/test-local.sh`: `4b30f8c57c56af660b7a8859d6112377032b5c208297aab5468334fbd1a2f8c9`
- SHA256 `scripts/dev/test-node.sh`: `c36c454e16399c691952ee6939d1de47cb7d650ce0213c37f46d020f1a2cf86a`
- SHA256 `docs/delivery/GATES.md`: `f6abc64db6206ddad0375836f28ed31f8da42f8e09bc80d2add7e3dfba304ffc`
- SHA256 `.github/workflows/gates.yml`: `88eddcb0d0a8dd0beb81e4ae3bf71a108b0978acbca0ddfb41c1a2cb63a00f79`
