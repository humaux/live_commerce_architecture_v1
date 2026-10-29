# FIX-299 — WebKit order gate

Status: causal MODEL_ONLY RED confirmed; first implementation submitted for CI. Engine acceptance PENDING.

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

The tests-only commit `17d65036df8f7a84ffbb5b288f63867a7bfbf7ac` adds two MODEL_ONLY deterministic focus-scroll injections around the actual BO03 and BH gate statements. Foundation run `40807614352`, attempt 1, job `123230198222`, failed on exactly those two new assertions: both delivered to `sticky-purchase-action` instead of the intended control. The step reports 487 tests, 485 passed, 2 failed, 0 skipped, exit 1. The raw failing assertions and receipt metadata are in `red.log`. These are **not** WebKit execution or independent acceptance.

## Implementation (first targeted attempt)

- BO03: wait for enabled, focus the recovered-address submit control and assert focus before the original single real pointer click. The original 5000ms journal-clear poll and replacement-key/CAS-version assertions stay verbatim and in order. No extra click, request retry, storage mutation or clock adjustment.
- BH: retain the history response observer registered before activation. Within the same awaited aggregate, wait for the post-navigation history toggle to be enabled, focus it, assert focus, then execute the original single click. Keeping readiness in that aggregate also preserves the gate-waits rule against orphan response promises. Original server-response/time/list/locale/locator assertions remain intact.
- Failure evidence: each page now gets a global screenshot/text index across contexts. Previously each context restarted at index 0 and overwrote earlier contexts, including the BH page. The retained historical screenshots therefore cannot establish BH's delivered click target.
- Generated browser ownership: fetched and merged `origin/r3/integration` (`7a2c4e7639634dd9bd70fcd0c2dfbeaae59b4910`; already up to date), then ran `node scripts/dev/derive-browser-ownership.mjs`. No browser mode was narrowed.
- Risk: focus-scroll displacement remains an E0 engine-cause hypothesis. The deterministic RED models that timing window; actual WebKit CI must establish effectiveness, and three green executions alone do not expose WebKit internals. If the same failure persists, inspect the now non-overwritten page evidence and use at most one further targeted fix before escalation.

Playwright's current official documentation (read 2026-10-29) confirms that clock installation affects every page in the context: <https://playwright.dev/docs/api/class-clock>. No clock change is justified yet. Navigation/hydration guidance: <https://playwright.dev/docs/navigations#hydration>.

Playwright 1.63.0 is pinned in the repository. Official API guidance re-read 2026-10-29: <https://playwright.dev/docs/api/class-locator#locator-focus>, <https://playwright.dev/docs/actionability>, <https://playwright.dev/docs/api/class-page#page-wait-for-response>. Focus preparation supplements, and never replaces, normal click actionability or the pre-armed response wait.

## Validation and continuation

- Local static (E1): `node --check tests/storefront/order-gate.mjs` and `node --check apps/storefront/tests/order-gate-timeout.test.mjs`, both exit 0. Ownership generator exit 0 (53 modes). No local test, server, browser, build, database or container started.
- RED: confirmed in this PR's Foundation CI on the tests-only head; see `red.log`. This is E3 evidence for the injected model failure only, not an engine fix or product acceptance.
- GREEN: PENDING. Must bind all results to the implementation head; retain all original gate assertions verbatim/in order and all existing timeouts.
- WebKit stability: NOT_RUN on this branch; require at least three successful CI executions of the WebKit order suite on the implementation head, recording workflow IDs, attempt numbers and WebKit job IDs. Do not confuse rerun attempts with distinct workflow IDs.
- Independent review: NOT_RUN; integrator owns acceptance. Never merge or enable auto-merge from this lane.
- Humaux search/claim/store/canvas: unavailable tool surface; no shared-state writes attempted.
- Queue handoff: stop monitoring within ten minutes of pushing; integrator resumes from the non-plan `WAIT_RUN` workflow ID.

## Next session

Read the implementation head's Foundation/Gates results from the PR receipt, ignoring old-head runs. After the first green WebKit job, rerun only that job twice on the unchanged head. Record all three workflow ID / attempt / WebKit job ID tuples and confirm that the order step actually ran and passed in each (not merely a successful plan). A GitHub job rerun retains its workflow run ID and increments run_attempt; do not invent three distinct workflow IDs. Put final receipts in a PR comment without an evidence-only push. Independent integrator review and engine-cause confirmation remain outstanding. Do not merge or enable auto-merge.
