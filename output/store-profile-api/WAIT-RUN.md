RELAY 2 — implementation push handoff (2026-10-29 Asia/Shanghai)

PR #287 targets r3/integration from unit/store-profile-api. Causal RED is recorded
in red.log: Foundation40762550914/job123078018073, head24cac517, four new assertions.
Implementation and E1 static receipts are in DELIVERY.md/static.log; no local test
or server ran. GREEN remains PENDING; own assessment E0. Independent review,
provider SANDBOX/LIVE and production NOT_RUN. No merge or deployment.

This file intentionally precedes the implementation commit: the PR handoff comment
is the receipt for its exact head SHA, Gates/Foundation WAIT_RUN IDs and queue state.
Integrator owns the completion wakeup via a fresh relay; no background poller.
Humaux store/canvas unavailable in this relay; integrator must mirror the handoff.
