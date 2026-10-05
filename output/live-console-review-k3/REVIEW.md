# Adversarial review of contracts/live-console-v1.md (Sonnet 5.5 standing in for K3)

Reviewer: Claude Sonnet 5.5 (K3 quota exhausted). Date 2026-10-05. Branch unit/contract-live-console. Read-only except this file.
Evidence label: DESIGN review; code facts re-checked by grep at this worktree (cited inline). Nothing was run.

## Verdict: FREEZE_AFTER_FIXES

The architecture is sound: ledger for every send, no tag, no text in PG, PSID sealed per send, one `mpr:` key per comment, Check re-reads
policy. No Meta-policy violation found in the written rules. But 14 P1 gaps would make a backend agent guess or leave a privacy/money hole,
and none is fixable by the implementer. Counts: **P0 = 0, P1 = 14, P2 = 17**.

Verified OK (attacked, held): self-authored comments are already dropped by intake (`meta-claims-intake-v1` fail-closed list: `from.id = asset_id`,
`parent_id` present), so the recommend comment (contains the keyword) cannot create a claim; `live_claim_window_one_open` is the only
store-wide unique on windows, `ingest.go:150` filters by session; `claim_sources_one_active` (0064:173) makes one comment map to one session;
HPKE private ring is claims-worker-only, API holds only the public ring and the payload keyring (`secrets.manifest.tsv:88`) so the §3.4 split is deliverable.

---

## P1 findings

### P1-1  Manual private-reply rules cannot be evaluated where they are placed (§3.3, §4.1, §4.3)
The producer definer runs in PG; the API has no Graph access. Comment `created_time`, "not the Page's own comment" (`from.id` is never returned to the
API, §2.4) and parent/child status are only known to the worker ring buffer. Backend must guess.
Replace the §3.3 "Manual private reply" row condition list with:
> The API calls bridge route `POST /internal/v1/comment-facts {tenant_id, store_id, session_id, source_id, comment_ref}` →
> `{created_at, is_page: bool, is_reply: bool, found: bool}` (ring buffer, else one Graph `GET /{comment_id}?fields=created_time,from{id},parent{id}`).
> `found=false` → `409 comment_unknown`. The API passes `p_comment_created_at`, `p_is_page=false` to `inbox.plan_manual_private_reply`, which
> freezes `comment_created_at` in the request (§4.1 frozen-request list gains it). `deadline_at` and the IG-live `received + 15 min` are computed from
> that frozen value. Check does NOT call Graph (MCI Check is lock-free, no network); a Graph 4xx "not the Page's comment / cannot reply" maps per §4.3.
Also add to §0.1 UNKNOWN: **LC-U10** whether a private reply may target a *child* comment (`parent_ref` set); until probed `is_reply=true` → `409 reply_comment_unsupported`.

### P1-2  Manual-first vs auto-apply race leaves an ACCEPTED claim with no link (§4.2, §14.1 clause 2)
The check "intake PENDING / ACCEPTED-plannable" and the later `plan_claim_reply` are separate transactions; only the unique `mpr:` index serialises.
If manual wins, the auto path commits the claim with `claim_reply_skipped:reply_used` and the buyer never receives a claim link; nothing surfaces it.
Replace §4.2 last sentence and §14.1 clause 2 tail with:
> `plan_manual_private_reply` takes `FOR SHARE` on the comment's `claims.meta_intake` row when one exists and refuses when `state IN ('PENDING')` or
> when an auto reply is still plannable; the intake apply takes `FOR UPDATE` on the same row, so exactly one wins. If the auto path still meets an
> existing `mpr:` operation of message_type `manual_private_reply`, it commits the claim, writes `claim_reply_skipped:reply_used`, **and sets
> `claims.bundle.link_pending_manual=true`**; the console shows 「此買家的認領連結尚未送出，請在私訊中補發」 and the bundle appears in A8 `unreplied`.
Gate LCN07 adds: manual and auto planned in opposite orders under `-race`, 100 iterations: never an ACCEPTED claim with neither a link operation nor `link_pending_manual`.

### P1-3  Retention gaps: outbound copy can live forever; C5b deletion hits an FK; erasure misses new tables (§10, §3.6, §3.4)
(a) C5 `inbox.outbound_messages` requires "operation terminal". UNKNOWN is not terminal (external-operation §Complete: "No automatic UNKNOWN→READY", UNKNOWN persists) so a
display copy of buyer-facing text survives indefinitely. (b) `inbox.conversation_state.conversation_id → social.conversations` as a real FK blocks C5b
(`DELETE social.conversations`, claims-retention-purge-v1 §1 row C5b) with 23503, so conversations (and their peer_key) are never purged; "FK cascade from the definer" is not a
mechanism. (c) Actor erasure RD4 removes `bundle_peers` only; outbound text to that person, `conversation_state`, `send_secrets` remain.
Replace §10 rows C5, C5c and the RD4 clause with:
> C5 (extended): `inbox.outbound_messages` eligible when `created_at < now() − social_days` **regardless of operation state**; the operation's frozen request is redacted by C4.
> C5c: `inbox.conversation_state` has **no FK** to `social.conversations` (soft reference, `conversation_id` + `tenant_id,store_id` indexed); `claims.run_retention` step C5b deletes
> `inbox.conversation_state` and `inbox.outbound_messages` of the conversations it deletes **in the same statement batch, before** the conversation delete (ordering C5 → C5c → C5b).
> RD4 / `apply_actor_erasure`: for every `peer_key` it already resolves, also DELETE `inbox.outbound_messages`, `inbox.conversation_state`, `inbox.send_secrets` and `inbox.bundle_peers` of that peer.
Gate LCN13 adds: UNKNOWN operation older than `social_days` → display copy gone; C5b run with a populated conversation_state succeeds; erasure of a peer leaves zero rows in all four tables.

### P1-4  Bearer link / free text residue in the display copy and the send secret (§3.4)
"a payment/claim link inside it is replaced by `{{付款連結}}`" is undefined for free-typed text: a merchant who pastes a buyer link into A12 gets it
AES-sealed into `outbound_messages` for 30 d (a live bearer credential persists, I11 spirit). Also `send_secrets` are wiped "on every terminal state", but
UNKNOWN is non-terminal and never redispatched, so `{psid, text}` stays up to 8 d for no purpose.
Replace the `{{付款連結}}` sentence and the wipe sentence with:
> Before sealing the display copy the API replaces **every** `https?://…` run and every bare domain (the §3.5 pattern) with `{{連結}}`; templates keep their own placeholders.
> `inbox.send_secrets` is wiped by the completion transaction on **SUCCEEDED, FAILED_FINAL, BLOCKED_POLICY, STALE_BINDING and UNKNOWN** (UNKNOWN never redispatches and Reconcile is query-only), and by C7 at 8 d.
Also: `body_sha256` in the frozen request (§4.1) is an unsalted hash of low-entropy text ("謝謝", "ok") = a dictionary oracle that outlives C5 until C4. Keep it only on the
outbound row (deleted by C5) or use `HMAC-SHA256(per-tenant key from the payload keyring, text)`. (P2 part.)

### P1-5  Customer suggestion and order prefill from `owner_id` contradict I09 (§3.7(a), §5.1 step 1)
I09: "链接来源不是人物身份". `claims.bundles.owner_id` is bound to whoever opened the claim link first; a forwarded link binds a different human than the
commenter/DM peer. §3.7(a) auto-suggests that buyer as the conversation's customer and §5.1 step 1 pre-fills name/phone/email/last delivery (including a CVS
store and home address) from that buyer's last order into a drawer that will ship goods and send a payment link to the *peer*. Mis-delivery and cross-person PII disclosure.
Replace §3.7(a) and §5.1 step 1 "customer/last delivery" clause with:
> Customer and delivery are pre-filled **only** from an explicit merchant link (`inbox.conversation_state.customer_id`, A14) and are shown as 「已手動連結」. `owner_id` never produces a
> suggestion, a prefill, or `linked_customer_id`. A13/A15 return `customer: null, last_delivery: null` for unlinked conversations; the UI shows empty fields.

### P1-6  Duplicate for-buyer orders and double stock reserve with different keys (§5.1, LCN12)
LCN12 only tests double-submit under the same Idempotency-Key. Two staff, two tabs, or a retry with a new key create two orders from the same bundle: two holds
(I03 not violated but stock is wrongly consumed), two payable links for one buyer (double-charge exposure), and the 0105 ledger charges the live price only on the first.
The buyer's own claim link then reprices at catalog (0105 rule) with no warning.
Add to §5.1 after step 3:
> `inbox.order_for_buyer` has `UNIQUE (tenant_id, store_id, bundle_id) WHERE order_state <> 'CANCELLED'` (one row per bundle in `bundle_ids`; table gains `bundle_id` rows, not an array).
> The request takes `pg_advisory_xact_lock(hash(store, bundle))` for each bundle (sorted order) before SetCart. A bundle that already has a live (non-CANCELLED) for-buyer order, or
> whose claim lines are fully consumed in `claims.live_price_uses`, → `409 bundle_already_ordered {order_id}`; the UI offers 「查看訂單」, not a second order. `for.bundle_ids` empty = plain manual order, no live price.
Gate LCN12 adds: two different keys on the same bundle → one order, one hold; staff A and B concurrently → one 409.

### P1-7  OPEN-13 ruling condition (1) fails open when peers are unknown or there is no conversation (§5.3, ruling)
The ruling says the server derives the bundle's peer key and the thread's peer key and refuses on mismatch. But `inbox.bundle_peers` exists only after a SUCCEEDED private reply
carrying `recipient_id` (LC-U6, §3.7), so for many bundles (no reply sent, LC-U6 false, IG) there is no bundle peer; and `conversation_id: null` (the v5 "幫他建立訂單" button on a comment) has no
thread at all. As written the "mismatch" test is vacuous and the merchant effectively chooses whose live price applies, with only `inventory:reserve`.
Append to the ruling:
> Condition (1) is evaluated **fail-closed**: when `for.bundle_ids` is non-empty, the request must carry `conversation_id`; both peer keys must exist and be equal, else
> `409 bundle_buyer_unverified` (missing) or `bundle_buyer_mismatch` (differs). With no verifiable peer the order is created at catalog price (`live_price: "not_applied", reason`),
> never silently at live price. Granting a live price additionally requires `live:manage` (in addition to `inventory:reserve`), because it changes price.
> The attestation row is single-use: `claims.merchant_origin_grants` carries `request_id` and `quote_id`; `claims.live_prices` honours it only for that quote, and it is consumed by the Begin transaction.
LCN12 adds: unknown peer → catalog price; mismatched peer → 409; grant reused by a second quote → catalog price.

### P1-8  For-buyer crash window between Place, record and DM (§5.1 steps 3-5)
`ManualOrders.Place` commits SetCart/Quote/Begin in separate transactions; the `order_for_buyer` row, audit and DM plan come later. A crash in between leaves a stock-holding order with a
consumed live-price ledger, no `order_for_buyer` row (so no dedupe, P1-6), no link and no DM; replay returns `buyer_link: null`.
Add to §5.1:
> Steps 3-4 are **resumable** under the Idempotency-Key: on replay the server (a) reads the order from `command.Run`, (b) upserts `inbox.order_for_buyer` keyed by `order_id`, (c) if `send_payment_link` and no `meta.dm_send` exists for `(order_id)`, calls
> `regenerate-link` (the previous link is superseded) and plans the DM with the new link. The DM semantic key is `mdm:` + hash(conversation, `order-pay-link`, order_id), so a replay can never plan two.
LCN12 adds: kill the process after Begin, before the record → replay completes, one order, one DM operation.

### P1-9  Bridge is undefined for more than one claims-worker replica (§2.2, §2.3)
The lease ensures one poller per source, but the bridge address is one `COMMERCE_CLAIMS_CONSOLE_ADDR`. With two replicas a request lands on a replica with no buffer; the epoch
differs per replica so browsers see `reset:true` forever. No statement says what the non-holder answers.
Add to §2.3:
> v1 supports exactly one claims-worker replica per stack for the console bridge (smoke asserts `replicas=1` for the service). If the lease row's `holder_id` is not this process, the route
> answers `421 {code:"not_poller_owner"}` and the API returns `stream.state=unavailable` without retrying across replicas. Multi-replica routing (holder address in the lease row) is an upgrade, not v1.
Also state in §2.2 that `epoch` is persisted in the lease row (`poll_epoch`) so a lease handover keeps it monotonic and `reset` is only sent on a real buffer loss.

### P1-10  Graph token transport and paging cursors can leak the token (§2.4, I11)
§0.1 L7 rejects a token in a URL for streaming, but the poller's own GET calls specify nothing. Graph's `paging.next`/`previous` URLs embed `access_token`; a worker that follows `next`, logs
a request URL or returns it in `older_cursor` leaks the Page token. MCI U6 (Bearer header acceptance) is still UNKNOWN.
Add to §2.4:
> Every Graph call of the poller sends the Page token only in `Authorization: Bearer` (until MCI U6/LC-U5 proves otherwise the poller is MOCK-only); no `access_token` query parameter, ever.
> The poller never follows `paging.next/previous`; it builds the next request from `paging.cursors.after/before` only, and never logs or returns a Graph URL. `older_cursor` = HMAC(key, `{tenant, store, session, source, graph_cursor, exp}`) so a cursor from another tenant/source/session is rejected `400 invalid_cursor`; expiry 10 min.
LCN01 adds: fake Graph that embeds `access_token` in `paging.next`; the worker must not call it, and a sentinel token must not appear in logs.

### P1-11  Poller bounds and token freshness (§2.2, I23, I07)
(a) Demand-driven pollers have no ceiling: any `live:read` principal can wake a poller for every session with an active source (draft, ended, archived), each 0.5 rps against Graph; N sources on one Page
share one Page rate budget but are polled independently ("one poller per source"). (b) The token is loaded once; a disconnect, re-point or revoked principal does not stop the poller until the lease expires
(30 s), a re-point is only noticed when the loader is called again.
Replace §2.2 "A poller runs while …" and add:
> A poller runs while `(window OPEN) OR (lifecycle='live' OR ended < 2 h) AND demand_until > now()`; never for draft/archived. Fleet cap `COMMERCE_CONSOLE_MAX_POLLERS` (default 20), per-tenant cap 5; over cap → `stream.state=unavailable, reason=poller_cap`.
> Pollers sharing a Page asset share one rate budget and one backoff state (key = `asset_id`), pacing per §2.4 over the sum.
> The loader `load_meta_page_token_for_poll` is called on every lease renewal (10 s); zero rows → stop within 10 s, drop and zero the buffer, `stream.state=unavailable`.

### P1-12  Takeover generation semantics are ambiguous and LCN10 contradicts §3.6 (§3.6, §4.1, §4.3)
(a) "generation 0" means both "no conversation known" and "known conversation at generation 0"; Check cannot tell whether to compare or to resolve. (b) For the first private reply
the conversation is never known (bundle_peers is written *after* success), so a human-mode thread is bypassed; LCN10 ("planned at gen 0 then human takeover → Check human_takeover") only works when known. (c) A
human-originated DM queued at generation N is denied `takeover_changed` if the assignee releases (N+1) before dispatch; §4.3 does not say Check's takeover test applies to automated ops only.
Replace the §3.6 "Automated sends" and "Human sends" bullets with:
> The frozen request carries `conversation_known: bool` and `takeover_generation`. Automated send: `conversation_known=false` → Check resolves the conversation through `bundle_peers`/`peer_key`; if found and `mode='human'` → `human_takeover`. `conversation_known=true` → compare generation, mismatch → `takeover_changed`.
> Human send: Check does **not** compare generation; it requires only that the principal still holds `inbox:reply`. Takeover comparison applies to operations with `origin='auto'` only (field in the frozen request).
> Implicit takeover happens only for DM sends (A12) and manual private replies **to a comment whose peer is already linked to a conversation**; public replies and recommend never take over.
LCN10 adds the unknown-conversation case and the release-after-human-send case.

### P1-13  What counts as an "inbound message" for the 24 h window is undefined (§3.3, §3.6 trigger, §0.2)
The `AFTER INSERT` trigger sets `last_inbound_at` for every `social.messages` row. The consumer projects `page_message` / `instagram_message` families; if echoes (IG `messages` can carry `is_echo`), read
receipts, reactions or delivery events land there, `last_inbound_*` is inflated and the planner opens a 24 h window Meta does not grant: a send outside policy. Also the worker's Check needs a definer to read the
window; none is named (`commerce_claims_worker` has no read path to `inbox.conversation_state`).
Add to §3.6 and §3.3:
> `last_inbound_at/seq` advance only for rows with `direction='in'`, `is_echo=false`, and a user text/attachment/postback payload (classifier output `user_message`); reactions, read, delivery and echo rows never advance it.
> New definer `inbox.dm_window(p_tenant, p_store, p_conversation) → {last_inbound_at, mode, takeover_generation}` (STABLE, owner `commerce_meta_writer`, EXECUTE `commerce_claims_worker`, tenant/store passed from the frozen request) is the only Check read path.
> The window is `last_inbound_at + 24 h` using Meta's event time; if `occurred_at` is more than 5 min in the future of server time it is clamped to server time.
Gate LCN06 adds: echo, reaction, read-receipt rows never open a window.

### P1-14  `command.Run` / BFF must not persist or log A4/A5/A12 bodies (§12, §3.4)
`internal/command/command.go:64,88` stores `request_hash` and `response` in `ops.command_results`. The contract never says that DM/private-reply/public-reply
**request canonicalisation** must be a hash over the sealed ciphertext reference and never the plaintext, nor that the response excludes `text`. The admin BFF proxies A4/A5/A12 bodies (plain text with buyer PII).
Add to §12:
> A4/A5/A12/A16 pass `command.Run` a canonical request of `{kind, target ids, body_sha256 (HMAC form of P1-4), expected_generation}`; the plaintext never reaches `ops.command_results`, access logs, BFF logs or `pg_stat_statements`-visible parameters (the API seals before any SQL).
> LCN04 sentinel scan includes `ops.command_results`, BFF access logs and River job rows.

---

## P2 findings (each with the text change)

1. **§3.3/§4.3 hard 4xx maps to UNKNOWN before LC-U9.** Deterministic local errors (oversize text, bad recipient, Meta 100/200) become "不確定是否送達". Add: a 400 whose Graph `error.code ∈ {100, 2018278}` or `error_subcode` of size/format is FAILED_FINAL `invalid_request` **if the request was never accepted** (no `message_id`); only 5xx/timeout/ambiguous stays UNKNOWN. Keep the table of codes in LC-U9 evidence.
2. **§3.3 text limits per platform.** §11 says 2000 runes for all; Meta documents Messenger text 2000 characters and Instagram text about 1000 bytes (verify at probe). Change: "Messenger 1..2000 runes; Instagram 1..1000 **bytes** (UTF-8, CJK ≈ 333 chars); public 300 runes; planner returns `422 invalid_text {max}`".
3. **§3.5 normalisation is NFC; full-width digits and letters survive.** Replace "full-width dots normalized first" with "NFKC + case-fold + strip zero-width/format characters (Cf) before matching"; add `LINE ID`, `@handle`, `t.me`, `wa.me` words and a digit run ≥ 8 after removing spaces/dashes. LCN08 adds `ｗｗｗ．ａｂｃ．ｃｏｍ`, `０９１２３４５６７８`, `l i n e`.
4. **§3.5 recommend template includes the keyword (`A1`) in a comment visible to everyone.** Safe now (intake drops Page self-comments and replies), but the rule is invisible: add a sentence "recommend relies on `meta-claims-intake` §Fail-closed (`from.id = asset_id`); a regression test (LCN08) posts the template through fake webhook and asserts no claim is created".
5. **§3.4 `body_sha256` unsalted** (see P1-4); adopt HMAC.
6. **§2.2 ring buffer ignores deletions.** A buyer who deletes a comment stays on screen up to 2 h. Add: the consumer's `verb=remove` (existing feed webhook) publishes `{source, comment_ref}` to the worker (id only, no text) which evicts it; entries also expire at 30 min for refs not seen in the last poll window. Meta platform terms expect honouring user deletion.
7. **§6 capability chicken-and-egg.** `reply_public`/`private_reply` "Evidence that sets ok: …first SUCCEEDED send" would keep the first send disabled forever. Change column to "Evidence label" and add: `ok` is set by permissions + tasks + subscription; a successful send only upgrades `evidence` to LIVE_SEND.
8. **LC-U11 Handover/primary receiver.** Subscribing an app to `messages` can change which app owns the thread (Page Inbox becomes secondary); a Page that already runs another chatbot never delivers DMs to us. Add LC-U11 (who is primary receiver after `subscribed_apps`, can Business Suite still reply) and fail `dm_session` closed (`unknown`) until probed; add to R4.
9. **OPEN-10 echoes.** `unreplied` ignores replies typed in Meta Business Suite, so threads look unanswered. Add `unreplied` hint 「可能已在 Meta 後台回覆」 when `last_inbound_at` is older than 24 h; no logic change.
10. **§3.6 A10 `read_seq` unclamped / viewer-wipes-unread.** Specify `read_seq = LEAST(GREATEST(read_seq, requested), last_inbound_seq)`; and unread is cleared only by principals with `inbox:reply` (OPEN-9 counter, below).
11. **§4.1 no outbound rate cap.** Different Idempotency-Keys mean unlimited identical DMs. Add per-store limit 60 manual sends/min and `409 duplicate_recent` for same `(conversation, body hmac)` within 30 s; I23.
12. **§2.5 `console_marks` scoping and index.** State that the definer joins through the session's `claim_sources (object, asset_id)` (not by `comment_ref` alone) and that `integration.operations` gets an expression index on `(request->>'comment_ref')` for `meta.public_reply` (else seq scan on the largest table).
13. **§2.4 IG fallback definer is missing.** §3.2 defines only `social.read_thread`. Add `social.read_comment_events(p_session, p_after_seq, p_limit)` with the same envelope+AAD return and `live:read`, or drop OPEN-4's fallback.
14. **§8 grep-gate callers are real.** Name them in the gate: `internal/catalog/document.go:488,498` (store-wide `EXISTS` on OPEN windows: safe, but locks product edits while any of up to 5 windows is OPEN), `migrations/0071_claims_retention.sql:529` (C2 reopen guard per session: safe), `0079` billing guard (new opens only). Add to LCN09 "no query selects a single OPEN window without `session_id`".
15. **§4.2 BLOCKED manual quota.** An auto reply that ended BLOCKED_POLICY/STALE_BINDING (zero HTTP, provably unsent) still occupies the `mpr:` key, so the merchant can never reach the buyer. Add: `reply_used` applies only to states READY, DISPATCHING, SUCCEEDED, UNKNOWN, FAILED_FINAL; for BLOCKED_*/STALE_BINDING manual uses `mpr:` + `:m1` (second key class in the same partial index `semantic_key LIKE 'mpr:%'`), at most once.
16. **§2.6 reset semantics under tab refresh and HMAC cursor** fine, but add "`limit` ≤ 100, `after_seq` negative → 400" and the response must exclude comments with `from.id` of the Page's *other* apps. Minor.
17. **§3.7 `peer_key` is an unkeyed hash of a 16-digit id** (existing design, `meta-consumer`): `inbox.bundle_peers` keeps it for `intake_days` (≥ social_days) and ties a comment actor to a DM peer. Note in §12 that this link is personal data, covered by RD4 (done in P1-3); no change to the key.

---

## OPEN-n table (my answer)

| OPEN | Answer | Condition / counter-text |
| --- | --- | --- |
| 1 polling | Agree | Add a measured gate: p95 lag with 5 tabs on one session ≤ 5 s in LCN01 before LIVE. |
| 2 internal HTTP bridge | Agree, with P1-9 and P1-10 | single replica in v1; Bearer header only. |
| 3 FB poll object | Agree | LC-U1 decides; keep only one path in code after R2. |
| 4 IG live comments | Agree with change | needs `social.read_comment_events` (P2-13); else drop the fallback. |
| 5 streaming endpoint | Agree, rejected | Token in URL. Reopen only if header auth is proven (LC-U5). |
| 6 ring buffer | Agree 2000/2 h/10 min | add deletion eviction (P2-6). |
| 7 permissions | Agree | owner/admin also get them through the role-bundle definition, not only the P3 backfill, so later admins have them; `viewer` exclusion must be a test (LCN03). |
| 8 takeover trigger | **Counter** | Implicit on first DM send only (not public replies); takeover expires: `human_until = last_human_outbound_at + 6 h`, after which mode reverts to `auto` with generation+1, otherwise one DM freezes every later automated reminder to that buyer forever. |
| 9 read state | **Counter** | Store-level kept, but unread is cleared only by principals holding `inbox:reply`; a viewer opening a thread must not hide it from the assignee (P2-10). |
| 10 echoes | Agree | with hint (P2-9). |
| 11 quota priority | Agree with P1-2 | The "priority" is really first-writer-wins; state it that way and add the `link_pending_manual` safety. |
| 12 public content | Agree | NFKC normalisation (P2-3). |
| 13 live price | Agree with the ruling plus P1-5/6/7/8 | merchant attestation is acceptable only fail-closed, single-use, permission `live:manage`, one live order per bundle. Fallback remains valid and is safer if P1-7 cannot be built in LC-B6. |
| 14 PSID custody | Agree | wipe at UNKNOWN too; HMAC `body_sha256` (P1-4). |
| 15 cap 5 | Agree | add fleet/tenant poller cap (P1-11). |
| 16 comments after end | Agree | |
| 17 `inventory:write` to live_operator | Agree with bounds | fixed reason `live_console_edit`, `|delta|` ≤ 1000 per call, never below reserved, every call audited; otherwise a live operator can silently rewrite stock truth (I03). |
| 18 recommend = Page comment | Agree | one recommend comment per offer per 10 min (rate cap). |
| 19 outbound retention | Agree 30 d, **delete by age not by terminal state** (P1-3). |
| 20 no per-read audit | **Counter** | Add coalesced audit `inbox.thread_opened` (principal, conversation, ≤ 1 per principal per conversation per hour, no content). DM text is buyer PII; "who read it" is the one question a merchant or regulator will ask. |

## Attack checklist result (for the record)

| Area | Result |
| --- | --- |
| Meta 24 h, no tag, one reply per comment | Rules correct; windows depend on P1-13 (what counts inbound) and P1-1 (comment facts). |
| Public reply content | Correct in intent; bypassable by full-width forms (P2-3). |
| Privacy | No clear text/name/PSID persists by design; gaps are P1-3, P1-4, P1-14, P2-5/6. |
| Money | OPEN-13 sound with P1-6/7/8; no price field accepted (verified in §5.1 body and LCN12). |
| Concurrency | `mpr:` unique index is the right serialiser; P1-2, P1-12 close the race and ambiguity. |
| Tenant isolation | Definer-side checks are stated; bridge re-check good; marks join scope P2-12. |
| Underspecified for a backend agent | P1-1, P1-9, P1-12, P1-13 and P2-1/2/13. |
