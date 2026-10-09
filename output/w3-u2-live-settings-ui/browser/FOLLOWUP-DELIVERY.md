# W3-U2 browser fixture contract correction
- Task: 662ad08e-sub-browser. Base: 8c14a6416c15252bbe1502c0ec4d13f66c4de51e; originalbase894990131cd7e13fe907693e295f64d99087df15. Follow-up commit: fd00246b1a4073d89c9980e651108aefc803ade0.
- Changed only tests/admin/live-settings.spec.ts and tests/foundation/browser_live_settings_fixture_test.go.
- MOCK PUT requires exact published merchant ID/version and rejects fixed IDs (422). New DB-free negative gate reproduced200→422 without effects.
- Conflict case fills valid merchant body then publishes beforePUT409; original version4/onePUT/no-autooverwrite/reload assertions retained; adds exactreturnedID/version andonepublishedreceipt assertions. All real UI test IDs remain unchanged.
- DB-free Go RED exit1 (1FAIL); GREEN exit0 (1PASS). StrictTS,headers,12spec collection,diffcheck exit0. Commands/currentfourfilehashes: FOLLOWUP-RECEIPT.json.
- E3 applies only to test-owned MOCK fixture regression. Updated-source BROWSER/PG/build/fullGREEN/LIVE NOT_RUN per parent instruction; previous missing-page RED is historical at its captured hashes. No heavy runner or production edits. No own background process left.
