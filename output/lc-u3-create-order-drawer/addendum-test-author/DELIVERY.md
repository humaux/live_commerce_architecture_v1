# LC-U3 retry regression delivery

- task_id: LC-U3 / u3_retry_browser (parent task 22260ee8-63b0-4d6a-8376-0f26e7a95b5f)
- base_commit: 6aabe6b6eb738245a451e35ac0b4009786c9ab6b
- branch/commit: unit/lc-u3-retry-tests / caaae332
- worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/lc-u3-retry-tests
- role: independent browser regression author; runtime model/reasoning identifier UNKNOWN
- changed paths: tests/admin/create-order-drawer-gate.mjs; tests/foundation/browser_create_order_drawer_test.go
- summary: Adds one real ManualOrder UNKNOWN/draft-edit/retry case ahead of the unchanged 24-case drawer matrix. First response commits through real Go then is aborted at the browser network edge. Merchant uses real inputs to change draft name and quantity; explicit retry must keep the original exact key/body, return HTTP 200, and leave exactly one real order and one request-specific manual receipt. Drawer UNKNOWN asserts native disabled name and quantity fields. The observer fixes operation to merchanttools.order.manual for the new exact endpoint and retains the original for_buyer observer. Exact final matrix becomes 25 and exact PG order count becomes 14 (one additional actual order).
- contract/production changes: none; no API/schema/runtime mutation.
- actual commands: node --check tests/admin/create-order-drawer-gate.mjs → exit 0; go test -tags browser ./tests/foundation -run '^$' → exit 0 (compile/vet only, no tests run); git diff --check → exit 0; gofmt -w tests/foundation/browser_create_order_drawer_test.go → exit 0.
- evidence: node-check.log; go-compile.log; source-sha256.txt in this directory.
- evidence level: E1, structure/compilation only. Planned runtime evidence is real Go/PG and browser with MOCK OIDC, synthetic contact data and a response-loss network-edge fault.
- NOT_RUN: browser/PG red run; browser/PG green run; new-key calibration; legacy 10-case rerun; full CI; independent review. Parent owns red-before-production capture and final independent acceptance.
- unresolved: Product P1 intentionally remains at this branch base; this test commit does not fix it. No runtime PASS is claimed. Parent handles Humaux store/canvas/coord under its existing task.
- owned resources: no browser/server/database/container started; compile process exited. Failure samples unchanged.
