# CI-DRIFT-FN — open structural scanner limits

Owner decision after the adversarial review of PR #55 at `b8fc5d267431d243e6d1bdde3f3bd432bb736836`: freeze this PR's scanner implementation and capture the following counter-examples as exact-signature expected-fail tests. No further scanner patch is authorized here. A separate research task, `4a0c4279-fe4f-4cce-b426-9557bbd086a4`, covers the name-based, single-package traversal and mount-provenance design.

| ID / regression | Open behavior to capture | Evidence state |
| --- | --- | --- |
| CDF-4 / middleware in another package | `buildBuyer` calls `mw.WithHeader(h)` from `internal/mw`; changing the header value is not traversed as a handler dependency. A sound detector must mark the dependent route findings TOUCHED. | Review-derived E1 counter-example; new fixture CI NOT_RUN. |
| CDF-5 / non-path global value | Changing `globalLabel`, which a reachable handler reads into its response, is outside the path-only global-reference spans. A sound detector must mark the dependent route findings TOUCHED. | Review-derived E1 counter-example; new fixture CI NOT_RUN. |
| CDF-6 / MOUNT split across files | Builders and mount helpers live in `cmd/api/factory.go`, while `main` mounts them from `cmd/api/main.go`. A child route edit must also mark the parent MOUNT TOUCHED. The current gate can reject the child through other findings while missing this attribution. | Review-derived E1 counter-example; new fixture CI NOT_RUN. |

The new fixtures first require a valid legacy inventory. Only GitHub CI-confirmed, complete output and exit signatures may become expected-fail entries. UNEXPECTED_PASS, setup errors, changed signatures, missing baseline/provenance and unknown failures are hard failures. Existing cases and the whole-factory-span rule remain intact. This document will record the actual red run and exact signatures when CI executes them.

No TOUCHED baseline is allowed. These open limits are not repaired by the local-fixture widening in PR #55 and are not accepted as general scanner soundness.
