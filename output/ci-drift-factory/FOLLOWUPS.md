# CI-DRIFT-FACTORY — retired local-fixture follow-ups

The three original local-fixture gaps are retired by PR #55's widening change at `b8fc5d267431d243e6d1bdde3f3bd432bb736836`. Foundation [38498598779 / job 115639941947](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38498598779/job/115639941947) passed the contractdrift package with the former expected-fail helper removed. The cases use strict assertions and no longer skip. Exact full-output assertions are being added in the review follow-up; their new-revision CI result must be checked separately.

Historical unchanged-scanner failures remain preserved in [trunk-candidate-red.log](evidence/trunk-candidate-red.log), SHA256 `30c882d141f5186201ac355aa0b849b88f0612ae0da337f63b4564126a4d1c83`. That receipt belongs to head `b02134b886d6f251a63f8dc5a44b32a6f594dd62`, tested as merge `7315f8d62988ec8620dc3253b21e098ed828a750` into trunk `5497e55c3a0ad8ead2a4a8c30c5500c73af0c5c5`; it is not a current open-defect list.

The other 24 regression cases remain unchanged. The wider structural limits found by the adversarial review are tracked separately in [CI-DRIFT-FN FOLLOWUPS](../ci-drift-fn/FOLLOWUPS.md). Closing the local fixtures does not claim cross-package, global-value or split-file scanner soundness. Product preflight acceptance belongs to OA-1 / PR #53.
