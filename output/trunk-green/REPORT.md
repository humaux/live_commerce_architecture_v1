# trunk-green — 26 foundation reds on r3/integration 1a6a9177 (Sonnet; written by integrator from the agent's report)
Root cause for all 26: test expectations / fixtures / AST guards lagging today's frozen-contract merges. No product code or migration changed.
| test(s) | root cause | introduced by | fix |
|---|---|---|---|
| TestMetaConnect* (18) | subscribed_fields now feed,messages; fakegraph accepted only feed | lc-b3-inbox (live-console-v1 §3.1) | fakegraph + expectations accept feed,messages |
| TestT06WorkerAuthorityAndFunctionACL | 0119 adds claim_meta_resubscribe / finish_meta_resubscribe | lc-b3-inbox | function ACL list 74→76 |
| TestIdentityInitialStore…, TestOwnerProvisioningOP01 | owner bundle gains inbox:read, inbox:reply, inventory:live_adjust | lc-b3-inbox (§14.8) | expectations |
| TestStaffGateSG02… | fulfilment prefix match `inventory:` swept in live_adjust (live_operator only) | lc-b3-inbox | exact matching |
| TestMerchantSettingsHTTPDeliverySwitches | PUT returns default_warehouse_id, GET does not | delivery-allocation 0117 | assert that difference; all other fields still equal |
| TestMetaClaimsMCI10… (+ SharedPageLoaderNegatives) | 0118 adds a third pageSecretLoader callsite | live-a5-session | guard allowlist + two new negatives (stricter) |
| TestLocalRecoveryLogicalRestoreAndColdStart | 0119 trigger advances inbox.conversation_state | lc-b3-inbox | mutable-table set |
| TestR2IntegrationUpgradeFromReleaseHead | migration file count 49→56 | 0115,0117–0122 | exact count |
Verification: the 26 focused PASS=26 FAIL=0; two full runs FAIL=7 / FAIL=3 with different flaky tests each, all PASS on rerun — cause: other agents' test-local.sh/browser runs bypass the test-focused lock on a 1.9 GB Docker VM (PG recovery mode). Integrator follow-up: one shared Docker lock for every runner.
