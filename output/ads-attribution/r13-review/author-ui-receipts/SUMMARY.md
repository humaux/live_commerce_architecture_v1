# R11 audience receipt UI fix

task_id: d09c3cb4-fd41-4bb0-91cd-efebc8b4c572
base_commit: 40128fe5 (root source incorporated only into the owned worktree)
local_merge_base: bea2a308c6906a3f2703297c8cd4843b12e8c2fc
head_commit: 72f36edbdde8d2726f15f12fbd065f825ad9b94f
cherry_pick: 72f36edb only (do not cherry-pick local merge bea2a308)
branch: unit/ads-attribution-r11-ui
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution-r11-ui
role: UI implementation and Node/browser assertion author
actual_model: UNKNOWN (runtime identifier not exposed)
reasoning_effort: UNKNOWN (runtime setting not exposed)
skills_used: frontend-architect; playwright

Only changed paths:

- apps/admin/lib/attribution-audience.ts
- apps/admin/lib/attribution-client.ts (explicitly approved transport discriminant only)
- apps/admin/lib/attribution-copy.ts
- apps/admin/components/AttributionAudienceRead.tsx
- tests/admin/attribution-audience.test.ts
- tests/admin/attribution.spec.ts

Root cause: 0113 plan_meta_audience returns prior.state in the in-flight/10-minute
cooldown branch. 0008 integration.operations CHECK freezes nine legal states.
The READY-only parser rejected the real SUCCEEDED replay as a malformed receipt,
leaving an unknown UI journal despite a completed operation. The browser also
required READY on the already-read session instead of testing a new request.

The parser accepts exactly READY, DISPATCHING, UNKNOWN, ACKNOWLEDGED, SUCCEEDED,
FAILED_FINAL, CANCELLED, BLOCKED_POLICY, STALE_BINDING. Arbitrary state strings
still fail closed. Typed success transport becomes kind:acknowledged, without
changing HTTP/auth/key behavior. The UI journal records the exact operation state
and checks phase/state consistency. Legacy unknown journals retain their key;
legacy queued receipts migrate only to READY. View categories are queued,
inflight, completed, unconfirmed, failed. Completed copy explicitly says the
completed read is reused and no new read was queued. Four failure states have
distinct localized descriptions. Network and provider UNKNOWN retain the same
key; fresh-key refresh is disabled until the same request is checked.

Frozen browser fixture consumed (root-owned Go fixture 0c5f93f3):

- audience_operation_id:string, primary session's existing SUCCEEDED operation.
- fresh_audience_sessions: Record<`${locale}/${width}`,uuid>, six distinct,
  authorized and initially unread sessions for en/zh-TW/zh-CN x 390/1586.
- The existing audience_read field remains unchanged for the forbidden branch.

Each actual-click case selects its fresh session, clicks once, strictly asserts
READY/new operation UUID and queued receipt, then reloads the persisted journal.
It returns to the primary session, clicks and strictly asserts SUCCEEDED plus
the exact existing operation_id and completed copy. Reload and another click
must return the same SUCCEEDED operation with a different HTTP idempotency key.
The root's Go post-browser operation counts prove no duplicate operation.
No state-union acceptance or response interception was added. The dispatcher
remains running; there is no assumption a fresh read stays READY indefinitely.

Actual commands and exit codes, cwd=the owned worktree above:

- git status --short / git rev-parse HEAD / git diff --stat HEAD 40128fe5 -> 0;
  verified clean before integrating root source.
- git merge --no-edit 40128fe5 -> 0; owned worktree only, merge bea2a308.
- node --test --experimental-strip-types tests/admin/attribution-audience.test.ts
  before source fix -> 1; 7 tests, 1 PASS/6 FAIL. node-red.log. Parse, journal,
  transport fail on the old implementation; the initial SSR stub was incomplete,
  so later controlled render mutation is the meaningful SSR red evidence.
- Same Node command final -> 0; 8 PASS/0 FAIL/0 SKIP. node-final-green.log.
- Temporary removal of completed rendering; node --test
  --experimental-strip-types --test-name-pattern='actual read control'
  tests/admin/attribution-audience.test.ts -> 1; 0 PASS/3 FAIL.
  node-completed-mutation-red.log. Restored before commit.
- Temporary re-keying of retry; node --test --experimental-strip-types
  --test-name-pattern='actual read hook' tests/admin/attribution-audience.test.ts
  -> 1; 0 PASS/1 FAIL. node-rekey-mutation-red.log. Restored before commit.
- bash scripts/dev/test-node.sh -> 0; 381 PASS/0 FAIL/0 SKIP. node-full.log.
  Optional R04 binary suite explicitly NOT_RUN (binary not configured).
- pnpm typecheck:admin -> 0. typecheck-admin.log.
- pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2023 --module ESNext
  --moduleResolution Bundler --allowImportingTsExtensions --esModuleInterop
  --typeRoots apps/admin/node_modules/@types --types node
  tests/admin/attribution-audience.test.ts tests/admin/attribution.spec.ts
  -> initial 1 (viewport variable widened to number), final 0 after retaining
  literal viewport types. typecheck-tests.log and typecheck-tests-final.log.
- pnpm exec prettier --write on the six listed changed files -> 0.
- git diff --check -> 0.
- git add <six paths> && git commit -m 'fix(admin): distinguish audience read
  receipts from new queued reads' -m 'Co-Authored-By: Codex <noreply@openai.com>'
  -> 0; 72f36edb. Worktree clean.

The Node hook counterexample executes the actual component's event handlers with
synthetic hook/transport/storage adapters. It proves persist-before-send,
network UNKNOWN -> reload -> same-key retry -> SUCCEEDED, provider UNKNOWN ->
same-key check -> SUCCEEDED, and completed receipt persistence. SSR executes
the real JSX in all three locales. Both are MOCK; neither replaces a browser.

Evidence directory (main checkout, retained):
/Volumes/data/live_commerce_architecture_v1/output/ads-attribution/r11-ui-receipts/

NOT_RUN: PG, browser runtime, independent review, root final gates, SANDBOX/LIVE.
No Go/foundation/SQL/contract/lockfile changed by this fix. No root checkout
mutation, production activity or remaining task-started process. Root must
independently run its final gates after cherry-picking 72f36edb.

Humaux memory title: ads-attribution R11 audience receipt UI fix 72f36edb
Canvas: agent:r11_ui. Code index: livecommerce-r11-ui.
