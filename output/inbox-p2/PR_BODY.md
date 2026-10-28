Closes humaux/live_commerce_architecture_v1#242

Item 1 removes successfully read rows from the unread filter while retaining the open thread. Item 2 adds a distinct read conversation exclusion; reload assertions now wait for the exact unread response, aria-busy=false and an untouched unread conversation. Item 3 replaces simulated UR02 sequence writes with signed synthetic ingress, committed messages and the production trigger. Item 4 records the historical final OC-12 GREEN and explains the pre-allowlist 400 counterfactual without claiming an unobserved RED.

TEMPORARY MUTATION RED HEAD — DO NOT MERGE. This continuation combines the remaining item 2/3 sensitivity checks in ONE push: the browser reload receives a real unfiltered BFF list; UR02 substitutes the former direct sequence increment for its second inbound message. Existing exclusions and real-message count/sequence assertions must fail on CI. Only after those failures are observed will one restoration push remove both mutations and retain the reload barriers. Maximum two additional pushes authorized by the relay; this is the first. Exact restoration instructions: output/inbox-p2/RED-HANDOFF.md.

Observed item 1 RED E3: tests-only 8c12cec74644d3c1134ced0a3a8f4cd7b8820396, Gates40628056371 / browser-inbox122622864648, INU01 row-removal count expected0/actual1 after A10=200. Evidence: output/inbox-p2/red.log and red-assertion.log.

Prior implementation head 7f6ea56586ad9c5c90319b05c86b2821de193dd1 GREEN E3: Foundation40650271764 /122688477621; Gates40650271800 /receipts122794721961 /required122818633209. Both headSha fields independently verified. These receipts do not accept the temporary mutation or final restoration head. New RED/GREEN run IDs will be posted in PR comments, without an evidence-only push after GREEN.

Outside-card files: none. Direct UR02 test tests/foundation/inbox_unread_trigger_test.go is within the issue's direct-test allowance. No migrations, pins, baselines, dispatchers, existing gate-adapter assertion changes or dependency/import changes. No merge or auto-merge. Local gofmt, git diff --check and go vet ./tests/foundation exit0 (E1); no local tests or servers. Humaux available on continuation; decision stored and own canvas updated under lock. Author self-report E0.

NOT_RUN/PENDING: new mutation RED own-assertion receipts; final restoration GREEN and current-head contractdrift acceptance; independent review; local tests/servers/browser/PG (policy), local tsc (no node_modules), LIVE Meta. Historical allowlist/browser mutation RED remains NOT_RUN; item4 uses the issue's permitted source explanation.

🤖 Generated with Codex (dispatched by Claude Code integrator).
