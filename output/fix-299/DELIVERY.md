# FIX-299 — WebKit order gate

Status: RED submission; implementation and engine acceptance PENDING.

- Role: dispatched Codex implementer; no further delegation. Runtime model/effort are not exposed by this session.
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/webkit-order-299`.
- Branch: `unit/webkit-order-299`; base: `7a2c4e7639634dd9bd70fcd0c2dfbeaae59b4910`; PR target: `r3/integration`.
- Authority: owner FIX-299 card; repository `AGENTS.md`; `架构.md` §§0–2,26–27; `contracts/buyer-order-ui-v1.md` BO03/BO06/BO07; `contracts/buyer-order-history-v1.md` acceptance 4–5; actual order-gate and checkout source at the base SHA.
- Scope: `tests/storefront/order-gate.mjs`, its existing `apps/storefront/tests/order-gate-timeout.test.mjs` regression harness, this evidence directory, and generated browser ownership if the gate source changes. No product UI/design change.

## Historical receipts (not current-head acceptance)

1. PR #265: Gates run `40787166900`, attempt 1, WebKit job `123167610452` failed. Original artifact `12861950656` contains `playwright/buyer-order-2320519067/browser.log`: `order-gate.mjs:660` exhausted the unchanged 5000ms `toBeNull` assertion. The pending destination marker retained `expected_version: 0`. `failure-1.png` shows the sticky **Choose delivery** action focused, while the intended **Bring back saved address** action did not clear recovery. The run's later green attempt is separate evidence.
2. PR #268 head `c88effdd4afb3ff6b144a1e0469cd64bcbfa2181`: Gates run `40802912905`, attempt 1, WebKit job `123216109068` failed. Artifact `12866524889`, `playwright/buyer-order-129935673/browser.log`: history response observation at `order-gate.mjs:101`, called at `:745`, exhausted 30000ms. Source already arms that observer before the toggle click. A listener registered after that click is therefore not this failure's explanation.

## Investigation / RED coverage

Candidate: focus/compositor scrolling can displace a pointer action after navigation or recovered-address rendering. The BO03 screenshot supports wrong-target delivery; it does not establish WebKit internals or establish that BH has the same cause. This remains E0 pending real-browser evidence.

The tests-only commit adds two MODEL_ONLY deterministic focus-scroll injections around the actual BO03 and BH gate statements. They assert that a single pointer action reaches the intended control, and that history observation is armed before activation. They are **not** WebKit execution or independent acceptance. No existing assertion, timeout, retry policy or fixture is changed.

Playwright's current official documentation (read 2026-10-29) confirms that clock installation affects every page in the context: <https://playwright.dev/docs/api/class-clock>. No clock change is justified yet. Navigation/hydration guidance: <https://playwright.dev/docs/navigations#hydration>.

## Validation and continuation

- Local static: `node --check apps/storefront/tests/order-gate-timeout.test.mjs`, exit 0. No local test, server, browser, build, database or container started.
- RED: see `red.log`; new assertions must fail in this PR's own CI before implementation is pushed.
- GREEN: PENDING. Must bind all results to the implementation head; retain all original gate assertions verbatim/in order and all existing timeouts.
- WebKit stability: NOT_RUN on this branch; require at least three successful CI executions of the WebKit order suite on the implementation head, recording workflow IDs, attempt numbers and WebKit job IDs. Do not confuse rerun attempts with distinct workflow IDs.
- Independent review: NOT_RUN; integrator owns acceptance. Never merge or enable auto-merge from this lane.
- Humaux search/claim/store/canvas: unavailable tool surface; no shared-state writes attempted.
- Queue handoff: stop monitoring within ten minutes of pushing; integrator resumes from the non-plan `WAIT_RUN` workflow ID.

## Next session

Read this PR's failing Foundation log and confirm both new assertions fail for wrong-target pointer delivery, not harness/setup errors. Then evaluate the candidate against real WebKit evidence before claiming root cause; keep unproven alternatives explicit. Any fix and generated ownership changes belong in one implementation push, with the RED run/job IDs recorded in `red.log`. After green, put final receipts in a PR comment without an evidence-only push. Stop after at most two targeted fixes to the same blocker.
