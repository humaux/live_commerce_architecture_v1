# W3-04B sold-out auto reply — independent review (Opus, read-only)

- Reviewed: branch `unit/w3-04b-sold-out-reply` HEAD `d29a8a34` (author `ab6c21fa` + trunk merge `8c3851e4`), `git diff r3/integration...HEAD`.
- Migration hash: `0151_sold_out_reply.sql` sha256 `b5e75fcb486acacb7c74ac5215d2aaba1e6efb02e6c534743abe87235eaabb44`.
- Reviewer re-ran (DB-free only): `go build ./...` exit 0; `go vet ./internal/claims/ ./internal/integrations/metareply/ ./internal/msgtemplates/ ./tests/foundation/` exit 0;
  `go test -count=1 ./internal/integrations/metareply/ ./internal/msgtemplates/` ok; `scripts/dev/check-headers.sh` exit 0.
  Plus a Go probe: `unicode.IsPrint` is **false** for U+3000 (ideographic space), U+200D (ZWJ), U+00A0. PG suites: NOT_RUN by reviewer (CI).
- Evidence class of the unit: MOCK + REAL_PG (author's logs). Meta LIVE: NOT_RUN.

## Verdict: **MERGE-AFTER-FIX** (2 × P1, all fixable inside 0151 before its checksum freezes; no P0)

---

## P1

### P1-1 SQL renders sold-out text that the Go adapter then refuses: the comment's only private reply is consumed and nothing is sent
- Evidence:
  - `migrations/0151_sold_out_reply.sql:47-50` (`msgtemplates.sold_out_body`) only rejects `[[:cntrl:]]` and foreign placeholders; it allows **any number** of `{{product.name}}`.
  - `0151:137,219` substitutes `left(p.name,60)` without any character check. Product names are only length-checked (`0002_catalog_inventory.sql:28`, `internal/catalog/catalog.go:672 validName`), so they can contain U+3000 / ZWJ / NBSP and even `\n`.
  - `internal/integrations/metareply/sold_out.go:60-71 soldOutTextOK` requires `unicode.IsPrint` for every rune and ≤ 400 runes. Both `checkSoldOut` (`sold_out.go:74-79`) and `replyContent` (`routes.go:410-415`) return `errBadRequest`.
  - A non-policy Check error becomes `completeAmbiguous(... "policy_check_failed")` (`internal/integrations/core/dispatcher.go:306-311`). The result is UNKNOWN with zero HTTP calls. The operation stays on the `mpr:` key, so `inbox.plan_manual_private_reply` answers `409 used` (SO04 path).
  - Over-length cases:
    - With 5+ placeholders a ≤ 280-char template renders to more than 400 runes. That is under 2048 bytes, so the plan succeeds and the dispatch is refused as above.
    - With ~12+ placeholders and CJK names the request goes over 2048 bytes. `0151:240` then raises 22023, which `claimsintake/poller.go:247-249 classify` treats as **final**. The intake is marked FAILED and the buyer's claim is rolled back (lost).
- Why this matters:
  - Full-width spaces are common in zh-TW product names, and emoji ZWJ sequences also occur. In those cases the buyer gets nothing and the merchant can no longer reply to that comment. This breaks the Integrator ruling "认领照常记录" and the contract text "a chosen template that later cannot render falls back to the fixed one (the claim never fails)" (`contracts/live-keyword-claims-v1.md` amendment).
  - The fence that consumes the budget (SQL) and the fence that refuses to send (Go) disagree.
- Fix (one place each, then add a regression test):
  - (a) Make the Go check match SQL. Reject only `unicode.IsControl` / C0–C1 and newline, not `!IsPrint`. Alternatively, have SQL normalise the name with `regexp_replace(name,'[[:cntrl:]]',' ','g')` and keep Go as a pure mirror.
  - (b) `sold_out_body`: allow `{{product.name}}` at most once, e.g. `(length(body)-length(replace(body,'{{product.name}}','')))/16 <= 1`.
  - (c) `plan_claim_reply`: never RAISE on render size. If the rendered text is over 400 characters or the request is over 2048 bytes, fall back to the fixed template.
  - Tests:
    - product name `"韓版　針織衫"` with U+3000 → SUCCEEDED send;
    - name containing `\n` → still sends (normalised);
    - a template with 2 placeholders is refused by `set_sold_out_reply`.

### P1-2 Sold-out facts are evaluated twice in READ COMMITTED; a change between the two raises 22023 (final) and loses the claim
- Evidence:
  - `claim_reply_plannable` (0151:149-160 patch) calls `integration.claim_sold_out_facts` in one statement.
  - Then, after `gen_random_uuid` / `InsertOperationJob` (`internal/claimsintake/poller.go:212-236`), `plan_claim_reply` calls it again in a new statement snapshot (0151:207-210): `IF NOT sf.reply_enabled THEN RAISE ... '22023'`.
  - Nothing locks `inventory.balances` or the `claims.sold_out_settings` row.
  - Race: store switch OFF, stock available at plannable (→ `OK`), and a concurrent checkout commits the last reservation before plan (or the merchant flips the switch OFF in that window). Plan then sees `sold_out AND NOT enabled`, raises 22023, and `classify` makes it final, so the intake is FAILED and the claim and bundle roll back.
  - This is exactly the "last unit in a live burst" case.
- Why this matters: the claim must always be recorded (Integrator ruling and the migration's own invariant at 0151:5). The window is milliseconds, but it opens under load, when it is most likely to be hit.
- Fix: `plan_claim_reply` must not re-decide what `plannable` already decided under its locks. Simplest: `IF sf.sold_out IS TRUE AND sf.reply_enabled THEN <sold-out branch> ELSE <link branch>`, with no RAISE. A link for a just-sold-out SKU is the pre-0151 behaviour, and Begin stays the stock authority. Alternatively, `SELECT ... FOR SHARE` the settings row in plannable; stock still cannot be locked, so the no-RAISE rule is needed anyway.

---

## P2

1. **No-link guarantee covers system links only.** `sold_out_body` accepts any store `private_reply` template, and `msgtemplates.publish` (0121:135-140) does not restrict URLs for non-public-safe kinds. So a merchant can put an external or payment URL into the "sold out" auto reply. Consider rejecting `https?://` in `sold_out_body`, or state in the contract that merchant text is the merchant's responsibility.
2. **The W3-05B migration number must be > 0151.** Its brief keeps placeholder `0133` (`docs/delivery/units/w3-05b-*.md:4,41`) and it also `CREATE OR REPLACE`s `plan_claim_reply` on top of the new body. Lexical apply order (`migrations/migrate.go:96,244`, which gap-fills) would let 0151 overwrite W3-05B's body on fresh DBs. The same hazard applies to any later unit numbered < 0151 that re-declares `fixed_templates_template_id_check` (CRP02 already had to hold 0151 back for this reason: `claims_retention_test.go` diff).
3. **0149 / 0150 vs 0151: no conflict.** 0149 touches `catalog.product_images*`, `products`, `skus`, and reads `inventory.balances`. 0150 touches `payments.settlement_*` and its own audit actions. Neither touches anything 0151 creates or replaces. Note the separate 0149 `SET CONSTRAINTS` finding from its own review.
4. **`reply_kind='sold_out'` on the claim event (brief scope 2) is not implemented.** It is surfaced only through `live.console_marks` → `out_of_stock`. With switch OFF there is no marker at all and `link_pending_manual` is not flagged (0151:153), so the merchant has no console cue that a sold-out claim went unanswered. Either list this as a deviation in DELIVERY or flag the bundle.
5. **SO03:** a paused offer is REJECTED at ingest (no claim recorded). The Integrator ruling said that pausing "也算没货" and that the claim should be recorded. This is a pre-existing ingest rule and is documented as risk (3). It needs an integrator decision, not a code change here.
6. **Scope source inconsistency.** `inventory.claim_sku_sold_out` takes tenant/store from GUCs (0151:73-74), while its caller passes explicit `p_tenant/p_store` (0151:134). They are equal in the intake transaction (`poller.go:165`), but `claim_sold_out_facts` could assert `current_setting('app.tenant_id')::uuid = p_tenant` to fail loudly rather than silently read another scope.
7. **`integration.claim_sold_out_facts` has no `COMMENT ON FUNCTION`** (every other new definer has one), and its header claims the comment is "EXECUTE owner only". The T06 pin covers its ACL.
8. **Tests:**
   - red.log is red only because `claims.sold_out_settings` is missing. That proves the suite needs the migration, not that each assertion can fail.
   - Missing cases: the P1-1 and P1-2 scenarios, a full-width-space product name, and a multi-placeholder template.
   - The "last unit claimed concurrently" scenario gets two links by design (claims never reserve; Begin is the stock authority). Say this in the amendment so no one expects claim-time exclusivity.
9. **The two LiveConsoleInbox failures are pre-existing fixture fragility, not caused by this unit.** Details:
   - `TestLiveConsoleInboxCrossStoreIsolation` (`live_console_inbox_test.go:513-534`) and LCN10 (`:420-438`) look up a fresh `lcConversation` (`:30-39`, NULL `last_inbound_at`/`last_outbound_at`) in `social.list_conversations('all',...,50)`.
   - That list is ordered `GREATEST(last_inbound_at, coalesce(last_outbound_at,'-infinity')) DESC, c.id DESC LIMIT 50` (`0119:325-326`). A fresh conversation sorts at -infinity with a random-uuid tiebreak, so once the shared `tenantA/storeA1` fixture holds more than 50 conversations, membership is random.
   - The failing subset ran no `TestSoldOut*` test, and 0151 creates no conversations.
   - Follow-up (separate unit): seed `last_inbound_at=now()` in `lcConversation` or page the list. Don't hide it here.
10. **DELIVERY.md is missing items AGENTS.md requires:** reasoning level, explicit write_paths, and per-command exit codes for the pin runs.

## Checks that passed (with evidence)

- **One reply per comment.**
  - Same `mpr:` key and unique index (0151:241-245).
  - The advisory lock `lcn-mpr|key` stays in `claim_reply_plannable` (0128:785).
  - SO04 (manual after → 409 used), SO06 (manual first → `reply_used` skip) and SO07 (20× concurrent → exactly 1 op) are in `sold_out_reply_test.go:217-306`.
- **No public reply, no system link, no token.**
  - The sold-out branch skips `claims.issue_system_link` (0151:209-228).
  - `check_meta_reply` skips the link proof only when `message_type='sold_out_reply'` (0151:283-288). `message_type` comes from the definer-written `operations.request`, so it is trusted.
  - The adapter posts frozen text once (`routes.go:353-404`). On 5xx or transport doubt the outcome is UNKNOWN; reconcile is query-only (`routes.go:447-449`). Covered by `TestSoldOutDispatchNeverRepostsOnDoubt` and SO08.
- **7-day window:** `deadline = least(occurred+7d-1h, live? received+15m)` (0151:221-222), enforced by Check (`'deadline'`, 0151:276).
- **No buyer PII in the text:** the frozen text is template + product name only. `comment_ref`, `app_id` and asset ids are app-scoped. Same fields as before plus `offer_id`/`text`.
- **Function bodies copied from the highest prior body:**
  - `plan_claim_reply` and `check_meta_reply` diff against 0128:817-880 / 882-925 shows only the added branches. There is no later replace in 0129-0148 or on trunk.
  - `claim_reply_plannable` and `live.console_marks` are patched in place via `pg_get_functiondef` with a unique-needle guard, so the 0143 suspension patch and the 0123 body survive.
  - The template-id CHECK keeps all ids `{order-pay-link, offer-recommend, checkout-reminder}` plus `sold-out-reply` (0151:29-31; pinned by `TestSoldOutReplyACL`).
  - Owners and EXECUTE are unchanged (CREATE OR REPLACE keeps the ACL; owner re-asserted).
- **A6 rule matches 0116:**
  - Untracked or missing SKU → never sold out (0151:76); the formula is identical to `0116:31-35`.
  - Partial shortfall → sold out (SO-OPEN-1 test).
  - Same transaction as planning, with no stock hold.
- **SECURITY DEFINER hygiene:**
  - All new functions use `search_path=pg_catalog`, schema-qualified names, `REVOKE ... FROM PUBLIC`, and minimal grants: `sold_out_body` and `claim_sku_sold_out` → `commerce_integration_writer` only; settings definers → `commerce_runtime`.
  - `claims.sold_out_settings` has ENABLE+FORCE RLS, no login-role table privilege and no DELETE grant.
  - Merchant scope comes from `inbox.lcn_scope()` (server-pinned GUCs) plus `identity.principal_holds` live:read / live:manage.
  - A cross-store template id is refused (test at `sold_out_reply_test.go:419`).
- **Switch OFF:** the claim is ACCEPTED with no operation, plus audit `claim_reply_skipped:sold_out_off` (policy 0151:113-115). One manual reply is still possible and a second is 409 (SO05).
- **Headers:** Purpose / Depends on / Used by / Invariants / Status are present on all new and changed files; definer call-site comments are present; `check-headers.sh` exit 0.
