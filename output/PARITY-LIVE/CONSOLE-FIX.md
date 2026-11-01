# PARITY-LIVE CONSOLE-FIX — tests-only RED stage

Dispatched by Claude integrator, PR #355 into r3/integration; branch unit/parity-live, worktree parity-live. Starting head ef5e0316b8683486e366e15b6cadbea38e0f3c70. Role: UI implementer; actual runtime model/effort are not exposed. No delegation. No local tests or servers.

## Authority and rollback

Read AGENTS.md, 架构.md §§0/1/2/26/27, the CONSOLE-FIX card, MAIN's read-only output/integrator/diag/live-console-20261101.md and output/integrator/tools/cards/parity-common.md. Visual authority is the current v3 section in ux-live/output/ux-live/DELIVERY.md, Figma file iJ9lDdFIXgTlKkt5Tvrc0X/page4:8829 and the FIGMA-INVENTORY. Fleet Bridge returned a connected document; read desktop1429:39502 and mobile1429:39607 only. No Figma write or new design. Current source and raw GitHub receipts take precedence over historical output status text.

Fetched and merged origin/r3/integration first: Already up to date, integration fde11c1310fce20b576fc98023bd3003c66c205b. Rollback point is starting head above. Pre-existing modified RULING-FIX.md was preserved with a local console-fix-preexisting.patch backup; all prior untracked files remain untouched and unstaged. Do not revert or include these other-session files.

## Causal findings and bounded implementation plan

LiveConsole.tsx:126-139 wraps lifecycle action/state in default-closed live-advanced; :192-209 wraps the stock form and offer switch in default-closed data-live-tools. Refresh removes and recreates that subtree. These are primary seller actions, prohibited from collapsed disclosure by the card. Remove the action wrappers rather than persisting disclosure state or adding helper retries. Preserve all server capability/CAS/uncertain-command fences. showLiveTools currently asserts its first disclosure is attached; it must tolerate zero disclosures after their removal, preserving every caller and real-click behavior for any remaining secondary disclosure.

live-console.spec.ts:923-925 currently puts raw IDs into parity/<id>.png. The implementation must use id.replace(":", "-") for the basename, retain original IDs in ledgers, and update the PR frame-to-file list. This is naming only; no parity assertion changes. Existing E3 evidence: Gates41273678426/job124752200529 rejects parity/1429:39502.png with Contains the following character: Colon :. This does not establish why the preceding browser child failed.

The tests-only commit adds visibility checks BEFORE phase/showLiveTools in all six existing locale/viewport real-click cases, plus fresh-subtree visibility and persisted stock23/offer-off checks after the existing successful real-click sequence and a reload. Existing assertions and helper calls remain. No timeout, threshold, fixture, exemption or gate-adapter change. The implementation is deliberately not included until own CI proves assertion RED. Artifact-name implementation follows in the same implementation push after RED.

## bundle:07 — class (c), outside this PR's causal path

Raw CI Gates41273678426/job124752200718: TestBrowserReturnsUI fails at browser_returns_ui_test.go:111, returns receipt mode --browser-returns-ui, merge SHA4ab1edf13e04ec1d32fd7f3616157325c7f2adbb. Diagnosis recovered the restricted-session case, openOrder:69 called at :293, showing the signed-out page before returns assertions. Do not weaken authentication or call this a fulfillment permission failure.

Base evidence at integration fde11c131: tests/admin/returns-ui.spec.ts git blob7b7826dc726075f2237ad20385257ff7c398ee25; tests/foundation/browser_returns_ui_test.go blob e644991fde5feb6872834fbeeaf6c2e914f62276. Both are identical at this PR head; the restricted fixture at :99 grants orders:read and the auth swap is unchanged. git diff integration...HEAD contains no returns spec/harness, order page or auth implementation. Shared console changes run only on studio/console, and the console helper has no returns caller. No causal path from this PR's diff to the rejected swapped session was identified. Class (c) means inherited/out-of-card ownership here; no base-SHA CI reproduction or exact auth root cause is claimed. Change nothing in returns/auth; integrator routes it separately.

## Gate, evidence and handoff

Local static: ownership generator and --check, diff whitespace check; packet structural check recorded separately. No local runtime test, server, browser, Go test or fixture. E1 is structural evidence only; author correctness is E0. Original failure receipts are E3 at their recorded SHAs, not validation of this change. GREEN/E4/independent review are NOT_RUN.

One batched tests-only RED push, then register its own Foundation/Gates run IDs in red.log and PR comment (never a plan job). Wait at most about10 minutes; integrator relays a fresh session on conclusion. CI supplies job timeout/failure conclusion; local watcher/process not started. Implementation push is gated on a new test assertion failure; compile/infrastructure failure does not qualify. GREEN must come from #355's own implementation-head CI. No evidence-only push after GREEN.

Current staged source scope: tests/admin/live-console.spec.ts; generated browser-ownership.json; report/red.log/DELIVERY evidence. No out-of-card product source. Source planned next: LiveConsole.tsx, existing console helper and screenshot basename, all within admin/console scope. No migrations, contracts, OpenAPI, pins, lockfiles, baselines or dispatcher tables. No Go dependency change; depmap regeneration unnecessary for this tests-only stage.

NOT_RUN: new CI RED/GREEN at preparation time, product implementation, artifact recovery, all161 parity acceptance, current-head clicks/screenshots/density, independent review, local tests/servers, SANDBOX/LIVE/deployment. No task-owned process/port/container/fixture exists to clean up; keep raw failure evidence and existing owner files.

STATUS: WAIT_RED — tests-only regression ready; product fix and acceptance pending own CI assertion RED.
