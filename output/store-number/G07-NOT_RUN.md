# Full G07 skipped cases

Full command exit **1**, not PASS. The following 13 test/subtest events were explicitly marked NOT_RUN in `g07/G07.log`; they are not the cause of the nonzero exit and are not presented as successes.

| Test | Missing prerequisite / stated reason |
|---|---|
| TestStripeSP16Sandbox | STRIPE_SANDBOX=1 and owner Stripe test key |
| TestCustomersBillingCB10Sandbox | STRIPE_BILLING_SANDBOX=1 and platform Stripe test key |
| TestMailPA12SMTPProbe | Owner-run server SMTP probe; LC_MAIL_PROBE=1 and SMTP configuration |
| TestMetaAdsSandboxS1 | META_ADS_SANDBOX=1 and owner sandbox inputs |
| TestMetaAdsSandboxS2 | Same Meta sandbox prerequisites |
| TestMetaAdsSandboxS3 | Same Meta sandbox prerequisites |
| TestMetaAdsSandboxS4 | Same Meta sandbox prerequisites |
| TestMetaClaimsMCI11LiveReadOnlyProbes | META_LIVE_PAGE_ID and META_LIVE_PAGE_TOKEN |
| TestStripeSL08RestrictedKey | STRIPE_SANDBOX=1 and owner restricted test key |
| TestStripeRF10Sandbox | STRIPE_SANDBOX=1 and owner Stripe test key |
| TestManualFulfilmentMF02Schema/populated_upgrade_after_0062 | Existing test states migrations.Apply has no partial-apply hook for frozen 0062 fixture |
| TestStripeRF03Schema/populated_upgrade_from_0061 | Existing test states migrations.Apply has no partial-apply hook for frozen 0061 fixture |
| TestStripeSP21Registrar/sandbox_probe_against_real_stripe | STRIPE_SANDBOX=1, owner Stripe test key and account ID |

No credentials were retrieved or configured for this task. The task's own pre-0106 populated backfill test and R2 release-head upgrade test **did execute and pass**; these two legacy skipped fixture tests are distinct and were not used as substitutes.
