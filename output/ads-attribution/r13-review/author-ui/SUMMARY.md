# R11 attribution UI implementation handoff

task_id: bc2e2b27-4a7d-4acb-b1b8-593142ae5a47
base_commit: 8f419664f5a2c82085daadce68ab41e31851696b
head_commit: 7687fb86
commits_to_cherry_pick: 42c3d5f9, c47b0c85, 7687fb86 (in order)
timeline_followup_task_id: 2728fc1a-6ade-4163-8914-41dedcaec96a
branch: unit/ads-attribution-r11-ui
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution-r11-ui
role: UI implementation and scoped model/render/browser assertion author
actual_model: UNKNOWN (runtime identifier is not exposed)
reasoning_effort: UNKNOWN (runtime setting is not exposed)
skills_used: frontend-architect; playwright

Changed paths (only these six source/test files):

- apps/admin/components/Attribution.tsx
- apps/admin/lib/attribution-copy.ts
- apps/admin/lib/attribution-model.ts
- tests/admin/attribution.fixture.ts
- tests/admin/attribution.spec.ts
- tests/admin/attribution.test.ts

Frozen R11 report fields strictly parsed: truncated:boolean, nullable draft/session
spend_minor, nullable numeric breakdown fields, breakdowns_unavailable day/dimensions.
Unknown draft/session spend and ROAS render —. Nullable breakdown metrics render
localized Not provided / 未提供. Unavailable dimension/day rows have explicit labels.
Session spend describes selected-period promotion of the post. Unauthorised
audience panel gives exact three-language reconnect instructions and links to
the real registered settings route with locale/store preserved.

The browser spec still uses real PG report fixtures, Go/BFF and user actions,
without API interception. It requires the root/r11_pg fixture's unknown_draft_id,
truncated:true, breakdowns_unavailable and unknown_breakdown. Existing six
locale/viewport cases now select the unknown draft and reload, verify omitted
hourly metrics and unavailable dimensions, and click the Facebook reconnect
entry to settings and reload. Browser execution is NOT_RUN by this task.
The no-insights insufficient/not_authorized sessions additionally assert
session spend and ROAS are — after both selection and reload (PG author
confirmed these sessions have distinct posts and no associated insight rows).

MOCK Node render tests transpile and execute actual production JSX with React
SSR. Only navigation, initial report state, shell and the independently tested
audience-read control are stubbed. They supplement the browser gate.

Actual commands and exit codes (cwd is the worktree above):

- pnpm install --offline --frozen-lockfile -> 0; 49 packages reused, no download.
- node --test --experimental-strip-types tests/admin/attribution.test.ts
  before source changes -> 1; 7 PASS / 4 FAIL / 0 SKIP. node-red.log.
- node --test --experimental-strip-types tests/admin/attribution.test.ts
  tests/admin/attribution-audience.test.ts tests/admin/attribution-format.test.ts
  -> 0; 20 PASS / 0 FAIL / 0 SKIP. node-green.log.
- Removing the JSX cap notice, then the same single-file Node command -> 1;
  8 PASS / 3 FAIL. node-cap-mutation-red.log. Mutation restored before commit.
- Same three-file Node command after restore -> 0; 20 PASS / 0 FAIL / 0 SKIP.
  node-cap-mutation-green.log.
- bash scripts/dev/test-node.sh -> 0; 373 PASS / 0 FAIL / 0 SKIP.
  node-full.log; optional R04 binary suite explicitly NOT_RUN.
- pnpm typecheck:admin -> 0. typecheck-admin.log.
- Manual Node tsc without explicit --types node -> 1 (Node type auto-discovery
  failed). typecheck-node.log preserves the failed diagnostic. No product defect.
- pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module ESNext
  --moduleResolution Bundler --allowImportingTsExtensions --esModuleInterop
  --typeRoots apps/admin/node_modules/@types --types node
  tests/admin/attribution.test.ts tests/admin/attribution.spec.ts -> 0.
  typecheck-tests.log.
- pnpm exec prettier --write on the six changed files -> 0.
- git diff --check -> 0.
- git add <the six listed paths> && git commit -m 'fix(admin): preserve R11
  attribution unknowns and reconnect guidance' -m 'Co-Authored-By: Codex
  <noreply@openai.com>' -> 0; 42c3d5f9.
- After PG author confirmed the two state-session facts, browser-only followup
  commit c47b0c85 adds spend/ROAS — assertions; identical explicit-node test tsc
  rerun -> 0, git diff --check -> 0, scoped commit -> 0.

All logs here are under the MAIN checkout, not disposable worktree output:
/Volumes/data/live_commerce_architecture_v1/output/ads-attribution/r11-ui/

NOT_RUN / pending: browser runtime; root REAL_PG; provider SANDBOX/LIVE;
independent non-author review; root merge and final gates. No Go/SQL/foundation,
dependency, lockfile or frozen contract file changed. No production action or
remaining task-started process.

Humaux memory title: ads-attribution R11 UI unknown reconnect cap authored 42c3d5f9
Followup memory title: ads-attribution R11 real session unknown assertions c47b0c85
Canvas: agent:r11_ui. Code index: livecommerce-r11-ui (six changed files).

Timeline contract followup (root freeze after initial handoff): local-only
timeline points have spend_minor:null; explicit Meta zero remains zero.
Commit 7687fb86 adjusts timeline DTO/parser/rendering and adds the null-spend
local-point Node fixture. Three exact JSX spend-cell assertions distinguish
unknown from real Meta zero in every locale.

- Pre-fix single-file Node command -> 1; 3 timeline failures (node-timeline-red.log).
- Final three-file Node command -> 0; 23 PASS / 0 FAIL / 0 SKIP
  (node-timeline-green.log).
- pnpm typecheck:admin -> 0 (typecheck-admin-timeline.log).
- Explicit-node test tsc command above -> 0 (typecheck-tests-timeline.log).
- git diff --check and scoped four-file commit -> 0; 7687fb86.
- bash scripts/dev/test-node.sh -> 0; 376 PASS / 0 FAIL / 0 SKIP,
  optional R04 binary NOT_RUN (node-full-timeline.log).

Timeline memory title: ads-attribution R11 timeline unknown spend authored 7687fb86
