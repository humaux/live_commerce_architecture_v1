# w3-03b-checkout-reminder delivery
- Branch/commit: unit/w3-03b-checkout-reminder   Base: r3/integration df6be6ce (+ merge of trunk with 0139)   Model: Claude Sonnet (fallback)
- Summary: migration 0131 (fixed template checkout-reminder/v1, inbox.checkout_reminders, live.reminder_settings, definers inbox.checkout_reminder_candidates /
  inbox.plan_checkout_reminder / inbox.reminder_report, live.get/put_reminder_settings, private inbox.crm_bundle_state); internal/inbox/reminders.go;
  internal/httpapi/reminders.go (POST .../reminders[/{bundle_id}], GET .../reminders, GET/PUT .../live-settings/reminder); wired in httpapi handler.
- Contract/interface changes: contracts/live-console-v1.md "Amendment W3-03B" (auto leg DEFERRED: keyring custody, per integrator ruling).
- Tests: `bash scripts/dev/test-focused.sh '^TestCheckoutReminder'` -> exit 0 (11/11: CR01-CR06, CR08, settings CAS, no_storefront, single buyer, 0131 ACL; red: output/w3-03b-checkout-reminder/red.log,
  red-httpapi.log; green: green.log). `go test ./internal/httpapi -run 'TestReminder|TestSend|TestInbox'` -> ok.
  Regression `^(TestLiveConsole|TestLCN|TestK3LCB4|TestMsgTemplates|TestInbox|TestT06|TestKC03|TestR2Integration|TestLiveClaims|TestWAS|TestCheckoutReminder)` -> 101 pass / 1 fail
  (TestLiveClaimsKC03Schema: claims inventory pins for 0131; fixed in tests/foundation/live_claims_schema_test.go), then schema re-run
  `^(TestLiveClaimsKC03Schema|TestManualFulfilmentMF02Schema|TestMetaAdsMA02Schema|TestCustomersBillingCB02|TestTaiwanCvsSchema|TestMetaClaimsMCI02|TestCRP|...0128ExactACL)` -> exit 0 (schema.log).
  `bash scripts/dev/test-local.sh --inbox-send` -> exit 0. `bash scripts/dev/check-gates.sh` -> exit 0 (gofmt + header ratchet).
- Gates run: as above. CR07 (auto settings trigger) not applicable: auto leg DEFERRED. CR03/CR04 use the shared harness dispatcher; a stray default-queue worker could race a test job,
  so crTrigger parks new jobs on a holding queue in one statement.
- Evidence class: MOCK (REAL_PG + loopback fake Graph); Meta LIVE NOT_RUN.
- Deviations: (1) candidates + per-buyer plan are two statements in one merchant txn (sealing in Go); (2) inbox.plan_checkout_reminder is the origin=auto variant of plan_dm
  (plan_dm is human + implicit takeover), sharing lcn_emit/check_send/dm route; (3) own bundle_peers join instead of dm_window_for_bundle (needs conversation id);
  (4) reminder link = store's non-bearer checkout URL (claim/order links are hash-only); (5) no runtime table grants (0128 convention); (6) followup rows are
  unique per (session,bundle), semantic_key unique only for queued; (7) no savepoints (lcn_emit pins job xmin to the top-level xid), a planner race aborts the pass;
  (8) extra files touched: internal/httpapi/inbox_send.go (deny codes), internal/httperror/error.go (messages).
- NOT_RUN / BLOCKED: release-gate.sh --strict --only G07 (full ACL inventories; machine lock queue) NOT_RUN: integrator to run after merge; automatic River trigger DEFERRED (payload keyring stays API-only); Meta LIVE NOT_RUN; live.reminder_settings has no consumer.
- Integrator to-do: claims-retention-purge-v1 row for inbox.checkout_reminders (intake_days); serialise the fixed_templates template_id CHECK with W3-04B (0132);
  add test-local.sh --reminders; decide whether to keep live.reminder_settings until an auto path exists.
