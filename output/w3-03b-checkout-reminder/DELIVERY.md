# w3-03b-checkout-reminder delivery (revised after the independent review: 2 P1 + P2)
- Branch/commit: unit/w3-03b-checkout-reminder (SHA in the final commit message / `git log -1`)   Base: r3/integration 4509df6e (0129 LC-B6, 0136, 0139 merged)   Model: Claude Sonnet (fallback)
- Summary: migration 0144 (fixed template checkout-reminder/v1; inbox.checkout_reminders; empty live.reminder_settings; definers inbox.checkout_reminder_candidates,
  inbox.plan_checkout_reminder, inbox.reminder_report; private inbox.crm_bundle_facts; CREATE OR REPLACE of inbox.check_send (+ bundle-state re-check) and inbox.lcn_rate_check
  (+ reminders count toward the 60/min cap)); internal/inbox/reminders.go (scan + plan one buyer + report); internal/merchanttools/checkout_reminders.go (one scan tx, then one tx per
  buyer: issue link + plan); internal/httpapi/reminders.go (POST .../reminders[/{bundle_id}], GET .../reminders).
- P1-1 (livelock): only `send` verdicts count against the 100 limit (reminded / follow-up buyers cost nothing). Gate CR09: 105 buyers (3 follow-up), pass 1 = 100 sends + truncated,
  pass 2 = the remaining 2 sends with the 100 reminded not re-counted, all buyers handled.
- P1-2 (link must complete checkout): claimed + never opened -> re-issued claim link via claims.IssueLink (template checkout-reminder/v1); unpaid merchant_manual order -> re-issued
  order link via the fulfillment.regenerate_order_link rules (template order-pay-link/v1); issue + plan in ONE transaction per buyer, so a refusal rolls the link back. Gates CR01 (claim
  link hash = the bundle's current link, generation +1, 72 h, bundle holds the claim lines) and CR05 (order link = a live link of THAT order, previous link dead).
  LIMIT (needs an integrator ruling if unacceptable): an unpaid STOREFRONT-source order and an already-opened claim (bundle has an owner) cannot get a re-issuable link without a new
  bearer type -> `link_unavailable` follow-up (merchant copies the store checkout URL / asks the buyer).
- P2 done: (3) the scanned conversation is passed to the planner and verified to be a peer of the bundle (no re-pick); (4) scan lock dropped, planner re-checks under FOR SHARE;
  (5) per-buyer outcomes in the POST response, one buyer's race refuses only that buyer (gate PerBuyerRefusal: link rolled back); (6) lcn-dup lock first (A1.3) + reminders count toward
  the 60/min cap (gate CountsTowardTheSendCap); (7) check_send re-reads bundle state (CR03: paid after plan -> BLOCKED_POLICY not_remindable, zero HTTP); (8) candidates need window
  CLOSED or claim >= 10 min (gate YoungClaims); (9) planner refuses line_count=0 (gate SingleBuyer); (12) proconfig/search_path, column grants and policies pinned in the ACL gate +
  KC03 inventory (claims.bundles columns, claims.links generation, definer list, claims.links SELECT-probe exemption); (13) one audit row per pass incl. follow-up-only (gate
  FollowupOnlyAudit); (14) settings GET/PUT routes + definers removed, table kept (comment in 0144). (11) templates are not localised (msgtemplates has no locale): recorded in the
  amendment. (10) skipped per ruling; for the UI unit: the report's `display_name` is null for Meta buyers (no names are stored).
- Contract/interface changes: contracts/live-console-v1.md "Amendment W3-03B" (revised; auto leg + settings DEFERRED: keyring custody).
- Tests (local, focused): `bash scripts/dev/test-focused.sh '^(TestCheckoutReminder|TestLiveClaimsKC03Schema|TestT06WorkerAuthorityAndFunctionACL|TestR2IntegrationUpgradeFromReleaseHead|
  TestLiveConsoleSendMigration0128ExactACL|TestLiveConsoleTemplates|TestWAS)'` -> exit 0 (36 tests incl. TestPlatformOperator; TestCheckoutReminder = 15: CR01 window/takeover/once/claim link, CR02 concurrent, CR03
  Check re-checks, CR04 UNKNOWN, CR05 candidate set + order link, CR06 scope, CR08 no secrets persisted, CR09 livelock, no_storefront, single buyer, young claims, per-buyer refusal,
  send cap, follow-up-only audit/permissions, suspended store -> store_suspended, 0144 ACL); `bash scripts/dev/test-local.sh --inbox-send` -> exit 0; `bash scripts/dev/check-gates.sh` -> exit 0;
  `go test ./internal/inbox ./internal/httpapi ./internal/merchanttools ./internal/httperror ./internal/msgtemplates` -> ok. Red: output/w3-03b-checkout-reminder/red.log, red-httpapi.log,
  red2.log (the review-fix gates do not compile against the pre-review code: no per-buyer link-issuing pass existed). Green: gates.log, inbox-send.log.
- CI gates (NOT run locally per the owner's RAM rule; run on GitHub gates.yml): the broad regression
  `^(TestLiveConsole|TestLCN|TestK3LCB4|TestMsgTemplates|TestInbox|TestT06|TestKC03|TestR2Integration|TestLiveClaims|TestWAS|TestManualFulfilmentMF02Schema|TestMetaAdsMA02Schema|TestCustomersBillingCB02|TestTaiwanCvsSchema|TestMetaClaimsMCI02|TestCRP)`,
  and `release-gate.sh --strict --only G07` (full ACL/schema inventories; 0144 adds column grants on claims.order_origins, checkout.orders, claims.links, live.claim_sources and policies on them).
- Evidence class: MOCK (REAL_PG + loopback fake Graph); Meta LIVE NOT_RUN. Link redemption by a real buyer browser NOT_RUN (DB-level: stored link hash/owner/order verified).
- Deviations: (1) scan then one transaction per buyer (sealing in Go with the payload keyring + PSID; per-buyer isolation); (2) inbox.plan_checkout_reminder is the origin=auto variant of plan_dm
  (plan_dm = human + implicit takeover) sharing lcn_emit / check_send / the dm route; (3) own bundle_peers join, not dm_window_for_bundle; (4) no runtime table grants (0128 convention);
  (5) followup rows unique per (session, bundle), semantic_key unique only among queued; (6) no savepoints (lcn_emit pins the job xmin to the top-level xid); (7) the order-link re-issue
  repeats RegenerateLink's six lines inside the per-buyer transaction instead of calling it (it opens its own transaction); (8) extra files touched: internal/httpapi/inbox_send.go
  (deny codes), internal/httperror/error.go (messages), tests/foundation/live_claims_schema_test.go (pins), merchanttools package (new file only).
- Known limits: the POST is not one idempotent command (a replay re-scans; per-buyer link keys derive from the Idempotency-Key and the once-per-buyer guard prevents double sends);
  after a crash between a buyer's commit and the response the next call reports that buyer as already_reminded.
- NOT_RUN / BLOCKED: automatic River trigger and settings routes DEFERRED (inbox payload ring stays API-only); release-gate G07 and the broad regression -> CI.
- Integrator to-do: claims-retention-purge-v1 row for inbox.checkout_reminders (intake_days); serialise the fixed_templates template_id CHECK with W3-04B (0132); add test-local.sh
  --reminders; decide on the storefront-order / opened-claim limit above; per-locale reminder templates (UI/template unit); wire nothing else (cmd/api already passes ManualOrders).
