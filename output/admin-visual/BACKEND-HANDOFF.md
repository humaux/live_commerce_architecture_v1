<!-- Purpose: Transfer a shared MetaConnect mock-contract repair to its authorized backend owner. -->
<!-- Depends on: owner scope ruling,0119 subscription change and4b9 pre-browser gate failure. -->
<!-- Used by: integrator/backend unit dispatch; does not authorize UI-unit Go edits. -->
# Shared fakeGraph subscription contract

## Fresh f1 blocker — distinct from the repaired subscription mock

**Runtime closure:** upstream5251 was consumed by7b1fa50d after f1 completed. The unchanged MetaConnect gate returned0 at23e08490 (`gates-20261005T192028Z/browser-meta-connect.log`); visual and full click fixtures subsequently started successfully. Final612 full click-sweep returned0. This repair belongs to the backend unit; no UI parser workaround or Go edit was authored here.

**Upstream disposition (2026-10-06 02:50 CST):** integration `1fe3ab75` merged backend `75f06034`, including the missing DELETE grant and notify schema USAGE. The duplicate, still-unclaimed card `b5bf86a4-4995-4674-a463-e1166de017fa` was canceled after verifying the source diff. Root f1 batch is still immutable/running; consume the upstream fix only at its safe end boundary and rerun failed modes. This source inspection is not yet a green runtime result.

2026-10-06, source `f1c5199adeb0c59cd74bb7b6e34a0cd0977bf2bf`: `--browser-meta-connect` and `--browser-visual-lint` both exit 1. The latter stops **before** browser startup and produces no lint.json.

- Direct Go fixture at `tests/foundation/browser_click_sweep_test.go:355`: POST `/meta-connect/pick` returns **503** (`unavailable`, `retryable=true`), expected 201. Evidence: `gates-20261005T164802Z/browser-visual-lint.log:162`.
- Both browser traces also show pick 503 followed by status 200 with a valid `{connected,count,cap,pages}` envelope and an empty pages array. Evidence directories: `output/playwright/meta-connect-gate/20261005T182133.052579000/` and `output/playwright/meta-connect/20261005T182207.764203000/`.
- The direct Go reproduction excludes the BFF/browser as a necessary cause. The strict TypeScript parser accepts the returned status shape; changing UI parsers or assertions is not justified.
- Exact database error/SQLSTATE is **UNKNOWN** because the retained responses contain only the safe generic error. The new 0125 `meta_health_on_connection` trigger in the bind transaction is an investigation candidate, **not a proven cause**.
- Static follow-up: 0125:87 executes DELETE on `integration.binding_capabilities`; the SECURITY DEFINER is owned by `commerce_integration_writer` (:522), but :489 grants only SELECT/INSERT/UPDATE. RLS FOR ALL does not grant SQL DELETE. This is a concrete static privilege gap and strong 42501 candidate; runtime SQLSTATE remains unconfirmed. Current integration `0c1dce66` has the same migration. Backend coordination card: `b5bf86a4-4995-4674-a463-e1166de017fa` (submitted, not claimed/completed by this unit).

Backend owner action: capture sanitized SQLSTATE/function information for the real runtime Pick path, repair in an authorized backend worktree, and retain transaction rollback/status-count negative tests. Do not expose credentials, relax the expected 201, or modify the frozen visual detector. This UI unit continues its existing serial batch without changing the pinned source. The repaired old 502 below must not be mistaken for this new 503.

**2026-10-06 disposition:** implemented upstream in f544fac5 / trunk-green35d9ed73, now imported by mergef1c5199a. The shared mock requires feed,messages while preserving Page/token/failSubscribe checks. The unclaimed duplicate coordination card364570fa was canceled to avoid duplicate work; this is not a claim that the new UI runtime gates passed. Fresh f1 verification is queued. No Go fix was authored by admin-visual.

Owner chose: **backend unit repairs; preserve admin-visual scope**.
Coordination card: **364570fa-dddb-4739-8225-75aac6f7035e**, submitted; no backend claim/completion asserted.
Failing baseline:1a6a917774672f297e13ce56ef070a3084a2a924; rebased UI source4b9d5dc85cc58047c4aa06875a2461cb2c349d49.

## Cause and evidence

- internal/metaconnect/graph.go205 sends exact subscription value feed,messages after0119.
- tests/metaconnect/fakegraph/server.go214 still rejects everything except feed, returning Graph400.
- This maps through ErrConnectFailed to HTTP502 meta_connect_failed; tests/foundation/browser_click_sweep_test.go355 expected201 from /meta-connect/pick.
- Unchanged --browser-visual-lint exits1 before browser startup, with no new lint.json. Evidence: gates-20261005T120525Z/browser-visual-lint.log162. --browser-meta-connect is also red. No detector/CSS cause is inferred.

## Backend boundaries and acceptance

Use the backend unit's own worktree. Expected scope: shared fakegraph/server.go and corresponding pure Go tests. Validate the new required feed+messages contract; preserve token/Page/failSubscribe checks and reject missing/wrong fields. Record a real red on the old mock and green after repair. Run gofmt, package tests and appropriate focused MetaConnect PG tests (TestMetaConnectGateState is an existing candidate).

Do not roll back production subscribed_fields, change the frozen tests/ui detector, weaken expected201, touch this running UI worktree, or call live Meta. Send the repair commit/evidence to the integrator. admin-visual consumes the merged fix at a safe test boundary and reruns visual/MetaConnect/sweep unchanged. Existing captures cannot stand in for a new green result.
