RELAY 3 — contract-drift repair push handoff (2026-10-29 Asia/Shanghai)

PR #287 targets r3/integration from unit/store-profile-api. Causal RED is recorded
in red.log: Foundation40762550914/job123078018073, head24cac517, four new assertions.
Implementation and E1 static receipts are in DELIVERY.md/static.log; no local test
or server ran. GREEN remains PENDING; own assessment E0. Independent review,
provider SANDBOX/LIVE and production NOT_RUN. No merge or deployment.

This file intentionally precedes the implementation commit: the PR handoff comment
is the receipt for its exact head SHA, Gates/Foundation WAIT_RUN IDs and queue state.
Integrator owns the completion wakeup via a fresh relay; no background poller.
Humaux store/canvas unavailable in this relay; integrator must mirror the handoff.

Implementation head216d2979 failed Gates40765780013/plan123096837291 before Gates
test execution: two GO_ONLY profile routes and three unresolved leaf findings.
Raw receipt: relay3-drift.log. Targeted repair1/2 updates only the owning profile
contract table and BFF parameter binding to the existing scanner-supported forms.
No gate/test/baseline/exemption change. Foundation40765779877/job123089420820 was
still running when inspected; it is not evidence for the forthcoming repair head.

Fetched base remains313c2c36f; assigned migration0209 and R2 pin136 unchanged.
Node syntax and whitespace checks exit0; no local runtime verification. New exact
head and workflow WAIT_RUN IDs will be recorded in the PR comment after push.
