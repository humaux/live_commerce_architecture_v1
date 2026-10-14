# CI-DRIFT-FACTORY — confirmed unchanged-scanner gaps

Owner decision: keep the trunk scanner unchanged in PR #54. These are explicit expected failures in the new regression test, not accepted scanner behavior or repaired defects. A future scanner fix must retire the corresponding expected-fail; an unexpected pass fails the test.

All three observations come from head `b02134b886d6f251a63f8dc5a44b32a6f594dd62`, tested as merge `7315f8d62988ec8620dc3253b21e098ed828a750` into trunk `5497e55c3a0ad8ead2a4a8c30c5500c73af0c5c5`. Every case first passed its strict legacy coverage/provenance check. [Foundation 38487723575 / job 115605287048](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38487723575/job/115605287048) and [Gates 38487723574 / shard:unit 115606104511](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38487723574/job/115606104511) reproduced the same candidate findings. [Portable receipt](evidence/trunk-candidate-red.log), SHA256 `30c882d141f5186201ac355aa0b849b88f0612ae0da337f63b4564126a4d1c83`.

| ID / regression case | Observed result | Future acceptance |
| --- | --- | --- |
| CDF-1 / `route_edit` | Changing the child path to `/v1/trunk-factory/changed` correctly marks EXACT and the generic factory dispatch TOUCHED, with exit 1 / errors 2. The parent MOUNT `/v1/trunk-factory` remains a warning. This is a partial MOUNT attribution gap; the current gate already rejects the edit. | Preserve the existing EXACT rejection and attribute the child route change to its MOUNT. |
| CDF-2 / `middleware_dependency` | Changing the reachable middleware's response header value leaves EXACT, MOUNT and the generic dispatch at warning: exit 0 / errors 0 / warnings 3. | Reachable middleware dependency edits must mark the affected routes TOUCHED and exit 1. |
| CDF-3 / `body_dependency` | Changing the reachable handler constructor's response header value leaves EXACT, MOUNT and the generic dispatch at warning: exit 0 / errors 0 / warnings 3. | Reachable handler-body dependency edits must mark the affected routes TOUCHED and exit 1. |

The other 24 cases passed, including all 12 nested reassignment/shadowing, range/FuncLit, function-value and GenDecl caller-origin cases. Their strict assertions remain active. Syntax/setup errors, missing baseline coverage, different output signatures and regressions in these 24 cases are failures, never expected-fail skips. Product preflight acceptance belongs to OA-1 / PR #53; PR #54 does not implement or certify that change.
