# Live console v1 — comment read-through, Messenger/IG inbox, manual replies, order for a buyer (W2)

Status: **FROZEN (2026-10-05, integrator delegated; adversarial review by Sonnet in place of K3; K3 re-review when
quota resets)** — contract author: integrator (Opus), unit W2-00C of `output/arch-conformance/IMPLEMENTATION-PLAN.md`.
Review `output/live-console-review-k3/REVIEW.md` (FREEZE_AFTER_FIXES, 0 P0 / 14 P1 / 17 P2): all P1 applied, P2 per
§17, every `OPEN-n` RESOLVED (§15). A K3 finding at re-review reopens only the clause it names (PROCESS.md §2.2).
Evidence label of this file: DESIGN; every gate NOT_RUN. Nothing here authorizes a LIVE send outside the §13.3 probe plan.

UI target: 示意页 v5 (owner-adopted as the W2 acceptance target by integrator ruling 2026-10-05), the 直播中 one-screen
console (status bar, left = video + offers, middle = comments and DMs in one stream, right = buyer panel) and the
top-level 訊息 inbox that reuses the same buyer panel. Owner decisions bound here: 「公开回复也要做」 (public replies via
`pages_manage_engagement`), the 直播接单主流程 (keyword comment → claim → private reply with link → buyer pays; the
merchant can chat and build an order for the buyer from the console), and G3 (manual order may offer card, through
PAYUNi; integrator ruling 2026-10-05).

Amends, without editing them (amendment text in §14, recorded by the **integrator at freeze**, never by an implementer):
`meta-claims-intake-v1.md` (§1 Out list, §6.1–6.3, Amendment "Merchant connect" clause 3), `meta-consumer-v1.md`
(known limits), `meta-inbox-v1.md` (read authority note), `live-keyword-claims-v1.md` (§1 one OPEN window per store),
`studio-v1.md` (session lifecycle), `storefront-v2.md` G3 (order for a buyer), `claims-retention-purge-v1.md` (§1
classes), `external-operation-v1.md` (new actions). `invariants.json` is unchanged. Depends on W1-01B
`meta-connection-health-v1.md` (the capability table, §6) and A5 (`live-a5-session-flow.md`: A5-1 outcome read model,
A5-3 live-video picker), A7, claim-direct-checkout (cart merge). Migration numbers below are **placeholders 0120–0126
(0120 upward)**; the integrator assigns real numbers at dispatch.

## 0. Facts

### 0.1 Meta (retrieved 2026-10-05 unless noted)

| # | Fact | Source |
| --- | --- | --- |
| L1 | `GET /{object-id}/comments`: "The same permissions required to view the parent object are required to view comments" (`pages_read_engagement`); params `filter=toplevel\|stream`, `order=chronological\|reverse_chronological`, `summary=true` (count); cursor paging; "Objects with tens of thousands of comments may encounter paging limits". For page posts, reading comment ids requires the `MODERATE` task. | developers.facebook.com/docs/graph-api/reference/object/comments/ |
| L2 | `POST /{object-id}/comments` (reply to a comment = object is the comment): Page access token with the `MODERATE` task and `pages_manage_engagement`; body `message`; returns the new comment id; the edge cannot update or delete. | same page |
| L3 | Send API success body: `recipient_id` (page-scoped id) and `message_id`. Messaging types: RESPONSE (inside the standard window), UPDATE, MESSAGE_TAG. "your app has up to 24 hours to send a message" after the person's last message. | developers.facebook.com/docs/messenger-platform/send-messages/ |
| L4 | Message tags: the Meta page states that from 2026-04-27 requests carrying `CONFIRMED_EVENT_UPDATE`, `ACCOUNT_UPDATE`, `POST_PURCHASE_UPDATE` "will receive error code 100". SHOPLINE help (competitors.md S34) states Meta removed Messenger message tags on 2026-02-09. The two dates differ; this contract does not depend on which is right: **no tag is ever sent** (§3.3). | send-messages page; `output/live-console-research/competitors.md` §0.4 |
| L5 | Private replies: one message per comment, within 7 days of the comment; IG Live: only during the broadcast (meta-claims-intake F1/F2). Further messages only after the person responds (24 h). | meta-claims-intake-v1 §0 F1, F2 |
| L6 | The live-video comments reference page (`/docs/graph-api/reference/live-video/comments/`) timed out again (as in 2026-09-28, U1). Whether `/{live_video_id}/comments` or `/{post_id}/comments` returns live comments in near real time, and with which latency, is UNKNOWN (LC-U1). | — |
| L7 | Meta's streaming endpoint for live comments (`streaming-graph.facebook.com/{live_video_id}/live_comments`) is documented with the token as a query parameter (secondary knowledge, not re-retrieved). A token in a URL violates I11; **rejected** unless the LIVE probe proves header auth (LC-U5). | — |

UNKNOWN, each fails closed until its §13.3 probe closes it: **LC-U1** live-comment read object and latency (FB);
**LC-U2** whether Graph returns `from{id,name}` on comments of the merchant's own Page for non-role users;
**LC-U3** whether IG live media comments are readable through Graph during the broadcast (else webhook only, F4);
**LC-U4** whether IG DMs need the Page `messages` subscription in addition to the app-level `instagram` `messages`
field; **LC-U5** header auth on the streaming endpoint; **LC-U6** whether a private reply's Send API response carries
`recipient_id` (L3 is documented for ordinary sends); **LC-U7** whether a Page comment's `from.id` equals the
commenter's PSID (= meta-claims-intake U8 / retention U-D1); **LC-U8** public replies on IG live media; **LC-U9** the
Graph error codes for "window closed", "already replied", "no MODERATE task" (= MCI U3); **LC-U10** whether a
private reply may target a child comment (`parent_ref` set) — until probed `409 reply_comment_unsupported`;
**LC-U11** which app is the Messenger primary receiver after our `subscribed_apps` call (a Page running another
chatbot may never deliver DMs to us; can Business Suite still reply) — `dm_session` stays `unknown` until probed.

### 0.2 Code facts (read 2026-10-05 at `80b79487`)

- Page subscription is `subscribed_fields=feed` only (`internal/metaconnect/graph.go:198`): inbound DMs never arrive
  (deviation A18, P0 for the DM flow).
- Page tokens open only in `cmd/claims-worker` (HPKE private ring, meta-claims-intake "Merchant connect" clause 5); the
  API can seal, never open. `cmd/claims-worker` has **no HTTP listener** today.
- The Meta payload keyring (AES-GCM, `commerce_meta_payload_keys_json`) is mounted in `api` and `meta-worker`
  (`deploy/secrets.manifest.tsv:88`). `social.messages` / `social.comment_events` hold the webhook copy encrypted
  under it; `social.conversations` is keyed by the unkeyed `peer_key = tupleHash("meta-social-peer/v1", app, object,
  asset, sender.id)`; no PSID in clear anywhere. No authorized reader exists (meta-consumer "Known limits").
- Retention C5 deletes `social.*` rows after `social_days` (default 30, min 8); inbox bodies expire 24 h / 7 d
  (claims-retention-purge-v1 §1).
- `takeover_generation` is the literal `0` in `integration.plan_claim_reply` (`migrations/0064_meta_claims_intake.sql:871`).
- One OPEN window per store: `live_claim_window_one_open` (`migrations/0060_live_claims.sql:91`); windows are per
  session; ingest already filters `session_id` (`internal/claims/ingest.go:150`).
- The claims board has no timer (`StudioClaims.tsx`); orders page polls every 20 s (deviation A17).
- Manual order (`POST /v1/admin/stores/{store_id}/orders/manual`, `internal/merchanttools/manual.go`): buyer path
  under a server-held buyer capability, SetCart → CreateQuote → destination → `checkout.Begin`; permission
  `inventory:reserve`; body exactly `items, customer, delivery, payment_mode, locale`; no price field, no card, no
  message to the buyer. A live price applies only through a claim origin bound to the calling buyer with an unexpired
  link (live-keyword-claims "Live tools (R4)" rules 4–5, consumption ledger 0105).
- Staff roles (0089): `live_operator` = `store:read, live:read, live:manage, catalog:read, orders:read,
  inventory:read`; `viewer` = **every** `%:read` of the catalogue; owner/admin bundles are resolved at grant time.
- Stock edit = `POST /v1/admin/stores/{store_id}/inventory/adjustments` (`inventory:write`, `{warehouse_id, sku_id,
  delta, expected_version, reason}`); offer on/off and live price = `PATCH …/claims/offers/{offer_id}` (`live:manage`).

## 1. Scope

In: per-session comment read-through for the console (§2); Messenger and IG DM inbox on top of the existing encrypted
social copy (§3.1–3.2); three manual outbound kinds — DM RESPONSE, manual private reply, public comment reply — plus
the offer-recommend public comment, all through the external-operation ledger (§3.3–3.5, §4); conversation takeover
(§3.6); order for a buyer from the console (§5); capability states exposed to the UI (§6); console read models (§7);
several OPEN windows per store (§8); session lifecycle start/end/archive (§9).

Out: comment hide/delete, blocklist (W3-05B), checkout reminders (W3-03B), out-of-stock replies (W3-04B, but its quota
rule is fixed here, §4.2), giveaways, broadcasting outside the 24 h window, message tags, attachments sent by the
merchant, translation/LLM drafts, merchant replies made inside Meta Business Suite (echoes, OPEN-10), LiveKit
self-streaming (studio, stays MOCK/off), IG video embed (does not exist), per-staff read state.

## 2. Comment stream read-through

### 2.1 Posture

Comment text and commenter names are **read from Meta on demand and never written to PG, logs, River args, audit,
metrics labels or browser storage**. The only new persistence keyed on a comment is the plain Meta comment id
(`comment_ref`, IR-4 precedent: platform object id, retention class C3) in print records (§7.4) and in reply
operations (§4). The webhook copy that already exists (`social.comment_events`, encrypted, C5) is not changed; it is
read only as the IG live fallback of OPEN-4.

### 2.2 Who polls (closes the I11 custody question)

- The poller lives in `cmd/claims-worker` (the only process that can open a Page token). One poller per active
  `live.claim_sources` row, held under a lease row `live.comment_poll_leases(tenant_id, store_id, source_id PK,
  generation bigint, holder_id text, poll_epoch bigint, lease_token_hash bytea, lease_until timestamptz,
  demand_until timestamptz)` (0123, FORCE RLS,
  writer `commerce_integration_writer` through definers only). Lease 30 s, renewed every 10 s; a second worker replica
  finds the lease held and does not poll (I23: at most one Graph poller per source fleet-wide).
- Token: new loader `integration.load_meta_page_token_for_poll(p_source uuid, p_generation bigint, p_lease_token
  bytea)` (owner `commerce_integration_writer`, EXECUTE `commerce_claims_worker`), fenced like
  `load_meta_page_token` but on the poll lease; returns the current head for the source's binding, zero rows when the
  binding is disabled/re-pointed (→ poller stops with `stream_state=unavailable`). The token never leaves the poller
  goroutine; it is zeroed when the poller stops. The loader is called again on **every lease renewal (10 s)**: zero
  rows (disconnect, re-point, disabled binding) → the poller stops within 10 s, drops and zeroes its buffer and
  reports `stream.state=unavailable`.
- A poller runs while `(window OPEN) OR ((lifecycle='live' OR ended_at > now() − 2 h) AND demand_until > now())`;
  never for `draft` or `archived` sessions. `demand_until` is extended to `now()+120 s` by the bridge (§2.3) whenever
  the console asks for the session's comments, at most once per 15 s per source. Caps (I23): fleet
  `COMMERCE_CONSOLE_MAX_POLLERS` (default 20), per tenant 5; over a cap → `stream.state=unavailable,
  reason=poller_cap`. Pollers on sources of the **same Page/IG asset share one rate budget and one backoff state**
  (key = `asset_id`); §2.4 pacing applies to the sum of their calls.
- Ring buffer, process memory only (OPEN-6): per source ≤ 2000 comments, entry age ≤ 2 h, buffer dropped 10 min after
  the poller stops. Justification: N admin tabs on one session must cost one Graph poll, not N; a restart loses only
  what Graph can re-serve. Each entry gets a buffer-local `seq` (monotonic) under an `epoch` persisted in the lease
  row (`poll_epoch`, incremented only when a buffer is created empty), so a lease handover keeps it monotonic and
  `reset` is sent only on a real buffer loss.
- Deletions (users' deletions are honoured): every 60 s the poller re-reads the ids of buffer entries younger than
  30 min with one batched `GET /?ids=<≤50 ids>&fields=id` (same token transport, §2.4, inside the shared rate budget)
  and evicts ids Graph no longer returns; entries older than 30 min are evicted when the buffer is next trimmed or
  expire at 2 h. No new cross-domain edge from the meta-worker consumer (meta-consumer step 5 stays unchanged).

### 2.3 Bridge API ↔ claims-worker (OPEN-2)

claims-worker gains one internal listener `COMMERCE_CLAIMS_CONSOLE_ADDR` (backend Docker network only; never routed by
Caddy; smoke asserts it is not reachable from the edge network). Auth: `Authorization: Bearer <commerce_console_bridge_token>`
(new secret, 32 random bytes, mounted in `api` and `claims-worker`), constant-time compare. Two routes:

`POST /internal/v1/comment-page` body `{tenant_id, store_id, session_id, source_id, after: {epoch, seq} | null,
before_cursor: string | null, limit 1..100}` → `{epoch, items: [Comment], next_seq, older_cursor, stream: StreamState}`.

`POST /internal/v1/comment-facts` body `{tenant_id, store_id, session_id, source_id, comment_ref}` → `{found: bool,
created_at, is_page: bool, is_reply: bool}` — from the ring buffer, else one Graph `GET /{comment_id}?fields=
created_time,from{id},parent{id}` within the shared rate budget. It is the only way the API learns the facts the
manual private-reply rules need (§3.3); `from.id` itself never leaves the worker.

The worker re-checks `(tenant, store, session, source)` through `live.console_source(…)` (STABLE definer, EXECUTE
`commerce_claims_worker`) before touching a buffer; mismatch → 404. The API is the only caller and has already
authorized the merchant (`live:read`, store scope from the bearer, I01). Bodies are never logged; the bridge client
and server use the redacted error style of meta-inbox ("fixed safe errors").

**Replicas.** v1 supports exactly one claims-worker replica per stack for the console bridge (smoke asserts
`replicas=1` for the service). If the lease row's `holder_id` is not this process the route answers `421
{code:"not_poller_owner"}` and the API returns `stream.state=unavailable` without retrying across replicas.
Multi-replica routing (holder address in the lease row) is an upgrade, not v1.

Rejected: Page token in the API (custody clause 5); PG table or `NOTIFY` carrying comment text; a new broker; one
operation-ledger row per poll (reads have no side effect, I06 targets external actions; 2-s polling would add
~10 000 operations per 5.5-h live); Meta's streaming endpoint (L7, token in URL).

### 2.4 Graph polling, backfill, rate limits

- Token transport (I11): every Graph call of the poller and the bridge sends the Page token only in
  `Authorization: Bearer`; no `access_token` query parameter, ever. Until MCI U6 / LC-U5 prove Graph accepts the
  header, the poller is MOCK-only. The poller never follows `paging.next/previous` (those URLs embed
  `access_token`); it builds the next request from `paging.cursors.after/before` only, and never logs or returns a
  Graph URL.
- Object (OPEN-3): FB = the live video id chosen through the A5-3 picker when the source has one, else the source's
  `source_object_id` (feed post id); IG = media id. LC-U1 decides; until then FB console comments are MOCK.
- Start: backfill `order=reverse_chronological&filter=stream&limit=100`, ≤ 5 pages or 30 min of age, whichever first.
  Steady state: every **2 s**, `order=reverse_chronological&limit=50`, stop paging at the first id already in the
  buffer (dedupe by id; out-of-order arrivals are inserted by `created_time`, `seq` stays arrival order).
  Fields `id,created_time,message,from{id,name},parent{id},attachment{type}` (`from.id` is used only in memory to mark
  the Page's own comments and is never returned to the API).
- `summary=true` on one poll per 30 s gives `total_count` for the comments stat (§7.1).
- Rate limits: read `X-Business-Use-Case-Usage` / `X-App-Usage`; usage ≥ 50 % → 5 s, ≥ 80 % → 15 s; HTTP 429 or Graph
  codes 4/17/32/613 → exponential backoff 2 s → 60 s, `stream.state=throttled`. Graph 190 → `stream.state=reauth_required`
  and the existing `meta_connect_mark_reauth` path. Every Graph error is a reason code, never text.
- Older than the buffer: `older_cursor` = base64url(`{tenant, store, session, source, graph_cursor, exp}` ‖
  HMAC-SHA256 under a per-process key), expiry 10 min; a cursor of another tenant/store/session/source, expired or
  tampered → `400 invalid_cursor`. Served directly from Graph, not buffered.
- IG (OPEN-4): if LC-U3 shows Graph cannot read live media comments, the console serves IG live comments from the
  webhook copy `social.comment_events` through `social.read_comment_events(p_session uuid, p_after_seq bigint,
  p_limit int)` (STABLE SECURITY DEFINER, owner `commerce_meta_writer`, EXECUTE `commerce_runtime`, `M` transaction +
  `live:read`, limit ≤ 100; returns `instagram_live_comment`/`instagram_comment` events of the session's active IG
  source asset received since the session's first window `opened_at`, with the same envelope + AAD columns as
  `social.read_thread`); the API decrypts with the payload keyring and discards events whose `media.id` differs from
  the source's `source_object_id` (the media id is only inside the ciphertext). Webhook delivery happens only during
  the broadcast (F4).

`StreamState = {state: live|throttled|reauth_required|unavailable|not_started, poll_interval_ms, last_ok_at,
lag_ms (now − newest created_time), source_platform: facebook|instagram, video_embeddable: bool}`. IG →
`video_embeddable=false` (UI shows 「IG 直播無法嵌入，請在手機上觀看」).

### 2.5 Linking a comment to claims without storing text

The API joins the comment page with `live.console_marks(p_session uuid, p_refs text[])` (STABLE SECURITY DEFINER,
owner `commerce_claims_writer`, EXECUTE `commerce_runtime`, requires `live:read` via `identity.principal_holds`,
≤ 100 refs, every ref `^[0-9_]{1,80}$`). Every join goes through the session's `live.claim_sources (object,
asset_id)` — never by `comment_ref` alone; 0123 adds an expression index on `integration.operations
((request->>'comment_ref')) WHERE action IN ('meta.public_reply','meta.private_reply')`. Join keys:
`claims.meta_intake (object, asset_id, comment_ref)` →
`applied_event_id` → `claims.events` (status, reason, offer, quantity, bundle); `integration.operations` by
`semantic_key = 'mpr:'||…` (auto/manual private reply state) and by `request->>'comment_ref'` for public replies;
`live.comment_prints`. Result per ref (all nullable):

```
{ref, intake: {state, drop_reason} , claim: {status, reason, offer_id, keyword, quantity, bundle_id},
 private_reply: {kind: auto|manual|out_of_stock, state, blocked_reason}, public_replies: int,
 printed: {count, last_at}, private_reply_available: bool, private_reply_unavailable_reason}
```

`private_reply_unavailable_reason` ∈ `used`, `auto_pending`, `expired_7d`, `ig_live_ended`, `capability`,
`page_comment`, `reply_comment_unsupported`. (The page/child/age facts come from the ring buffer the API already
holds for the page; the authoritative check at send time is §3.3 via `comment-facts`.)

### 2.6 Console HTTP and cadence

`GET /v1/admin/stores/{store_id}/live-sessions/{session_id}/comments?after_epoch=&after_seq=&limit=` and
`?before_cursor=`. Response `{epoch, reset: bool, items: [ConsoleComment], next: {epoch, seq}, older_cursor,
stream: StreamState}`; `reset=true` when `after_epoch` ≠ current epoch (worker restarted: the UI clears and re-reads).
`ConsoleComment = {ref, parent_ref, created_at, author_name (may be null, LC-U2), text, is_page: bool,
has_attachment: bool, marks}`. `limit` 1..100; negative or non-integer `after_seq` → `400 invalid_cursor`.
Headers `Cache-Control: no-store`, `Referrer-Policy: no-referrer` (I15).

Cadence (OPEN-1, deviation A17): **polling**, not SSE, in v1 — comments every 3 s while the tab is visible, console
read model (§7) every 5 s, inbox list every 10 s; all pause when `document.hidden`; exponential back-off on 5xx
(3 s → 30 s). SSE is an upgrade signal when measured API load or lag (> 5 s p95) says so.

## 3. Messenger / IG DM inbox

### 3.1 Subscription (fixes deviation A18)

- Connect (`metaconnect.subscribe`) subscribes `subscribed_fields=feed,messages`. App-level webhook config adds object
  `instagram` fields `messages` (owner/Kimi WebBridge, §13.3; LC-U4).
- Existing connections: a one-shot claims-worker job `meta_resubscribe_v1` (0122 table
  `integration.meta_resubscribe_jobs`, same lifecycle and audit pattern as `meta_unsubscribe_jobs` of migration 0100:
  PENDING → LEASED → SUCCEEDED | FAILED | UNKNOWN, sealed token copy wiped at terminal) enqueued once per active
  connection by the 0122 migration's backfill function; result read back with `GET /{page}/subscribed_apps` and written
  to the capability table (§6).
- The consumer already projects `page_message` / `instagram_message` (meta-consumer MC02). No consumer change.
- Primary receiver (LC-U11): subscribing to `messages` can interact with the Page's handover/primary-receiver setup;
  `dm_session` stays `unknown` (sends refused, `409 capability`) until probe R4 records who receives DMs on the test
  Page and that Business Suite can still reply.

### 3.2 Inbound storage and read authority

Inbound storage is the existing `social.messages` (AES-GCM copy, C5 retention, default 30 d). Nothing new is stored
for inbound text. Reads:

- `social.read_thread(p_conversation uuid, p_before_seq bigint, p_limit int)` (STABLE SECURITY DEFINER, owner
  `commerce_meta_writer`, EXECUTE `commerce_runtime`; requires an `M` merchant transaction and
  `principal_holds(…, ['inbox:read'])`; limit 1..50) returns, per message, the envelope **and the immutable AAD
  context of its source event** (`key_id, nonce, ciphertext, app_id, object, asset_id, event_key, payload_hash,
  tenant_id, store_id, route_id, route_epoch, server_seq, occurred_at, direction='in'`). The API opens it with the
  payload keyring (it already holds it) and re-runs the frozen classifier projection to extract `{text, attachments:
  [{type}], sender_is_page:false}`; tag/hash/classifier failure → the message renders as `unreadable` (never dropped
  silently). Plaintext lives only in the response; `no-store`.
- `social.list_conversations(…)` (same owner/EXECUTE/permission) returns conversation metadata only (no ciphertext).
- Outbound bodies (what the merchant sent) are in `inbox.outbound_messages` (§3.4) and merged by `sent_at`.

### 3.3 Send rules (binding; re-checked at Check time, I07)

| Kind | Graph call | Allowed when (all at Check) | Deny code |
| --- | --- | --- | --- |
| DM RESPONSE | `POST /{page_id}/messages` (IG: `/{ig_id}/messages`) `messaging_type=RESPONSE`, `recipient.id`=PSID | `dm_session` capability ok; `last_inbound_at + 24 h − 5 min > clock_timestamp()` (window per §3.6, read at Check only through `inbox.dm_window`); conversation not purged | `window_closed`, `capability`, `conversation_gone` |
| Manual private reply | `POST /{asset_id}/messages` `recipient.comment_id` | Planning: the API first calls bridge `comment-facts` (§2.3); `found=false` → `409 comment_unknown`; `is_page` → `409 page_comment`; `is_reply` → `409 reply_comment_unsupported` (LC-U10); it passes `p_comment_created_at` and `p_is_page=false` to `inbox.plan_manual_private_reply`, which freezes `comment_created_at` in the request. Plan and Check: the `mpr:` rule of §4.2; `frozen comment_created_at + 7 d − 1 h > clock_timestamp()`; IG live: source window OPEN and frozen `comment_created_at + 15 min > clock_timestamp()`. Check never calls Graph; a Graph refusal ("not the Page's comment / cannot reply") maps per §4.3 | `used`, `auto_pending`, `expired_7d`, `ig_live_ended`, `page_comment`, `reply_comment_unsupported`, `comment_unknown` |
| Public reply | `POST /{comment_id}/comments` (IG: `POST /{ig_comment_id}/replies`) | `reply_public` ok; text passes §3.5; IG live media: refused until LC-U8 | `capability`, `public_reply_forbidden_content`, `ig_live_unsupported` |
| Offer recommend comment | `POST /{live_object_id}/comments` (FB only) | `reply_public` ok; offer active, not sold out; fixed template | `capability`, `offer_unavailable` |

- **No message tag, no UPDATE type, ever.** Outside the 24 h window the send is refused before planning (`409
  window_closed`) and again at Check (BLOCKED_POLICY). The UI shows 「買家 24 小時內沒有傳訊息，現在不能私訊；可等他回覆或在留言公開回覆（不能放付款連結）」.
- A human send never falls back to another channel automatically (meta-claims-intake §6.1 rule kept).
- Text limits (planner, `422 invalid_text {max}`): Messenger 1..2000 runes; Instagram 1..1000 **bytes** UTF-8
  (≈ 333 CJK characters; re-verified at probe S3); public reply and recommend 1..300 runes.
- Rate (I23): ≤ 60 manual sends per store per minute (`429 rate_limited`); the same `(conversation or comment,
  body_hmac)` within 30 s under a different Idempotency-Key → `409 duplicate_recent`.
- All four go through the ledger (§4): UNKNOWN is never blind-retried; Reconcile is query-only and returns UNKNOWN
  until a read proves delivery (MCI U4 precedent).

### 3.4 Outbound storage and PSID custody (OPEN-14)

- `inbox.outbound_messages(tenant_id, store_id, id, conversation_id NULL, comment_ref NULL, kind dm|private_reply|
  public_reply|recommend, operation_id, principal_id, template_id NULL, template_version NULL, key_id, nonce,
  ciphertext, body_hmac, created_at)` (0123, FORCE RLS). Ciphertext = the **display copy** sealed in the API with
  the payload keyring under AAD class `outbound` (`["livecommerce/meta-outbound/v1", tenant, store, id, kind, key_id]`).
  Before sealing, the API replaces **every** `https?://…` run and every bare domain (the §3.5 pattern, after the §3.5
  normalisation) with `{{連結}}`, whether typed or pasted; templates keep their own placeholders. No bearer link
  persists in a display copy.
- `body_hmac` = HMAC-SHA256(key derived from the active payload key with info `"livecommerce/meta-outbound-hmac/v1" ‖
  tenant`, rendered text), never an unsalted hash (low-entropy texts like 「謝謝」 would be a dictionary oracle). It
  lives on the outbound row and in the operation request; C4 redacts the latter.
- `inbox.send_secrets(operation_id PK, tenant_id, store_id, sealed bytea, enc bytea)` (0123): the **dispatch copy** —
  `{recipient_psid?, text}` sealed by the API to the Page HPKE **public** ring (info
  `["livecommerce/meta-send/v1", tenant, store, operation_id]`). Opened only by claims-worker in `LoadSecret` (a new
  lease-fenced loader `inbox.load_send_secret(operation, generation, lease_token)`). Wiped by the completion
  transaction on **SUCCEEDED, FAILED_FINAL, BLOCKED_POLICY, STALE_BINDING and UNKNOWN** (UNKNOWN never redispatches
  and Reconcile is query-only, so the secret has no further use), and by retention C7 at 8 d.
- The PSID is obtained in the API by opening the newest inbound message of the conversation (`sender.id`) and exists
  in clear only in API memory and inside the sealed secret. The operation request holds `conversation_id`, `peer_key`
  (already stored on the conversation) and `body_hmac`, never the PSID or text (external-operation "No …
  unredacted conversation").

### 3.5 Public reply content rule (server side, OPEN-12)

Matching runs on a normalised copy: **NFKC + case-fold + strip format characters (Unicode Cf, incl. zero-width) +
remove spaces and dashes between digits**. Rendered text (after template substitution) is rejected with `422
public_reply_forbidden_content` and a reason when the normalised copy contains: any URL or bare domain (`https?://`,
`www.`, `[a-z0-9-]+\.(com|tw|hk|cn|net|org|shop|store|me|io|app|link|ly)`), `t.me`, `wa.me`, `line id`/`lineid`, an
`@handle`, the store's own origin, the words 結帳/付款連結 followed by a link placeholder, a digit run ≥ 8 (phones),
an email address, or a template variable from the buyer class (`{{order.*}}`, `{{buyer.*}}`, `{{link.*}}`). The
stored/sent text is the merchant's original (only matching is normalised). Length 1..300 runes after NFC. Templates
usable for public replies are flagged `public_safe=true` at publish (W2-05B) and still re-validated at send. The same
rule applies to the recommend comment (fixed template `offer-recommend/v1`: product name, variant, keyword, live
price; no link). The recommend comment contains the keyword and is safe only because meta-claims-intake §3 drops the
Page's own comments (`from.id = asset_id`); LCN08 posts the rendered template back through the fake webhook and asserts
no claim, intake or reply is created.

### 3.6 Conversation state, read/unread, takeover (closes §10.3, replaces the hard-coded 0)

`inbox.conversation_state(tenant_id, store_id, conversation_id PK, mode text CHECK IN ('auto','human') DEFAULT
'auto', assignee_principal uuid NULL, takeover_generation bigint NOT NULL DEFAULT 0, human_until timestamptz NULL,
last_human_outbound_at timestamptz NULL, read_seq bigint NOT NULL DEFAULT 0, last_inbound_seq bigint,
last_inbound_at timestamptz, last_outbound_at timestamptz, status text CHECK IN ('open','done') DEFAULT 'open',
customer_id uuid NULL, customer_link_principal uuid NULL, version bigint, updated_at)` (0122). `conversation_id`
is a **soft reference** to `social.conversations` (indexed with `tenant_id, store_id`; **no FK**, so C5b can delete
conversations, §10). `last_inbound_*` is maintained by an AFTER INSERT trigger on `social.messages` (owner
`commerce_meta_writer`, creates the state row on first message).

- **What opens the 24 h window**: `last_inbound_at/seq` advance only for rows with `direction='in'`, `is_echo=false`
  and a user text/attachment/postback payload (classifier output `user_message`); reactions, read receipts, delivery
  events and echoes never advance it. The window is `last_inbound_at + 24 h` on Meta's event time; an `occurred_at`
  more than 5 min ahead of server time is clamped to server time.
- Check reads the window and takeover state only through `inbox.dm_window(p_tenant uuid, p_store uuid,
  p_conversation uuid) → {last_inbound_at, mode, takeover_generation, human_until}` (STABLE SECURITY DEFINER, owner
  `commerce_meta_writer`, EXECUTE `commerce_claims_worker`; tenant/store come from the frozen request) and its twin
  `inbox.dm_window_for_bundle(p_tenant, p_store, p_bundle)` (same owner/EXECUTE; resolves the thread through
  `inbox.bundle_peers`, zero rows when none) for automated sends with `conversation_known=false`.
- Unread = `last_inbound_seq > read_seq`; store-level, not per staff. `POST …/read` sets `read_seq =
  LEAST(GREATEST(read_seq, requested), last_inbound_seq)` and is honoured **only for principals holding
  `inbox:reply`** — a reader without it (e.g. an owner-granted `inbox:read`-only role) does not clear the assignee's
  unread mark (answer 200 with the unchanged value). 未回覆 = `last_inbound_at > coalesce(last_outbound_at,
  '-infinity')`; when `last_inbound_at` is older than 24 h the list adds the hint 「可能已在 Meta 後台回覆」 (echoes are
  not subscribed).
- **Takeover** (OPEN-8): implicit only on a DM send (A12) and on a manual private reply to a comment whose peer is
  already linked to a conversation (§3.7); public replies and recommend comments never take over. Implicit or
  explicit (`POST …/takeover`) takeover sets `mode='human'`, `assignee=sender`, `generation+1`; every human DM sets
  `last_human_outbound_at` and `human_until = last_human_outbound_at + 6 h`. Takeover **expires**: when
  `clock_timestamp() ≥ human_until` the conversation is `auto` again with `generation+1` — applied lazily by the next
  write or by the hourly retention job, and treated as expired by `dm_window` readers even before it is written.
  `POST …/release` = the same transition now. A different staff member may send while another is assignee (shown in
  UI); reassignment = takeover with CAS.
- The frozen request (§4.1) carries `origin: auto|human`, `conversation_known: bool` and `takeover_generation`.
  **Automated** (`origin=auto`: first private reply, later W3 reminders/out-of-stock): `conversation_known=false` →
  Check resolves the conversation through `inbox.bundle_peers` / `peer_key`; if found and effectively `human` →
  `human_takeover`. `conversation_known=true` → effectively `human` → `human_takeover`; generation differs from the
  frozen one → `takeover_changed`.
- **Human** (`origin=human`): A11/A12 carry `expected_generation` (stale → `409 takeover_changed` before planning);
  Check does **not** compare generation, it requires only that the principal still holds `inbox:reply`.

### 3.7 PSID ↔ claims ↔ customer link

- Comment actor ↔ conversation: when a private reply (auto or manual) SUCCEEDS, the Send API body carries
  `recipient_id` (L3; LC-U6). The claims-worker completion hook computes `peer_key` for `(app, object, asset,
  recipient_id)` and inserts `inbox.bundle_peers(tenant_id, store_id, bundle_id, peer_key, app_id, object, asset_id,
  operation_id, created_at)` (0123; FORCE RLS; written only through the Finish hook definer, the taiwan-cvs R-7a / ads
  F24 pattern). Join to `social.conversations` on `(app, object, asset, peer_key)`. If LC-U6 fails the link is never
  made and the buyer panel falls back to "same person?" unknown (no guessing from names). `peer_key` is an unkeyed
  hash of an enumerable id (meta-consumer design): this link is personal data, covered by retention C3 and erasure
  RD4 (§10).
- Conversation ↔ customer: **only** an explicit merchant link `POST …/inbox/conversations/{id}/customer-link`
  (`inbox:reply` + `customers:read`, CAS, audit `inbox.customer_linked|unlinked`), shown as 「已手動連結」.
  `claims.bundles.owner_id` (whoever opened the claim link first; a forwarded link binds a different person, I09)
  **never** produces a suggestion, a prefill or `linked_customer_id`. Never cross-tenant; never automatic merge.

## 4. Ledger rules for outbound operations

### 4.1 Actions and keys

| Action (provider facebook\|instagram, purpose `service`) | Semantic key | Producer definer (owner `commerce_integration_writer`, EXECUTE `commerce_runtime`) |
| --- | --- | --- |
| `meta.dm_send` | `mdm:` + hex(sha256(conversation_id \| Idempotency-Key))[:48] | `inbox.plan_dm(…)` |
| `meta.private_reply` (message_type `manual_private_reply`) | **the existing** `mpr:` + hex(sha256(object\|asset_id\|comment_ref))[:48] — the same global one-per-comment index of 0064 | `inbox.plan_manual_private_reply(…)` |
| `meta.public_reply` | `mpub:` + hex(sha256(object\|asset_id\|comment_ref\|Idempotency-Key))[:48] | `inbox.plan_public_reply(…)` |
| `meta.offer_recommend` | `mrec:` + hex(sha256(session\|offer\|Idempotency-Key))[:48] | `live.plan_offer_recommend(…)` |

- HTTP idempotency: `Idempotency-Key` (8–128 safe ASCII) through `command.Run`; same key + different body → `409
  idempotency_conflict` (I02); replay returns the original `{operation_id, outbound_id}`.
- Each producer, in one merchant transaction: permission check, rule check (§3.3), takeover update (§3.6), outbound
  display row, send secret, operation + READY event + River `external_operation_v1` job (`InsertTx`), audit. No
  network call. All four route through the claims-worker default lane (`commerce_claims_worker`).
- Frozen request (≤ 2 KiB): `{v:1, kind, origin: auto|human, platform, asset_id, conversation_id?,
  conversation_known, peer_key?, comment_ref?, comment_created_at?, session_id?, offer_id?, order_id?, outbound_id,
  body_hmac, template_id?, template_version?, policy:"lcn-policy/v1", takeover_generation, deadline_at,
  principal_id}`. `deadline_at` = DM: window end − 5 min; private reply: least(`comment_created_at` + 7 d − 1 h, IG
  live: `comment_created_at` + 15 min); public/recommend: plan + 15 min (a stale public reply is worse than none).
- Recommend rate: ≤ 1 `meta.offer_recommend` per offer per 10 min (`409 duplicate_recent`).

### 4.2 Private-reply quota (one per comment, shared; plan risk 「私密回复额度冲突」)

Rule: **first writer wins on the `mpr:` key**, with the manual path yielding to the automated ones (auto claim-link
reply, then out-of-stock auto reply W3-04B) wherever they can still be planned:

- `inbox.plan_manual_private_reply` takes `FOR SHARE` on the comment's `claims.meta_intake` row when one exists and
  refuses (`409 auto_pending`) when `state='PENDING'` or when an auto reply is still plannable (APPLIED with an
  ACCEPTED event whose source has `private_reply=true` and `claim_reply_plannable` would return OK). The intake apply
  takes `FOR UPDATE` on the same row, so exactly one of the two proceeds; the unique `mpr:` index remains the final
  serialiser.
- If the auto path still meets an existing `mpr:` operation of message_type `manual_private_reply` (amendment §14.1
  clause 2), it commits the claim, writes audit `claim_reply_skipped:reply_used` **and sets
  `claims.bundles.link_pending_manual = true`** (new column, 0123); the console shows 「此買家的認領連結尚未送出，請在私訊中補發」
  and the bundle's thread appears under A8 `unreplied`. The flag clears when a claim link is issued for the bundle.
- W3-04B (migration 0151): the automatic reply of a **sold-out** claim is a `meta.private_reply` of message_type `sold_out_reply`, the same single
  operation on the same `mpr:` key as the claim-link reply (rank "auto"): it consumes the comment's one private reply, so a manual reply after it is
  `409 used` and a restock cannot re-reply to that comment (only a DM inside the 24 h window or a new comment can). With the store switch off the
  claim is skipped (`claim_reply_skipped:sold_out_off`) and the budget stays for the merchant. See live-keyword-claims-v1 Amendment "W3-04B sold-out reply".
- `used` counts the key only while its operation is READY, DISPATCHING, SUCCEEDED, UNKNOWN or FAILED_FINAL. When the
  existing `mpr:` operation ended BLOCKED_POLICY or STALE_BINDING (zero HTTP calls, provably unsent), one manual
  private reply may use the second key class `mpr:` + `:m1` (same partial unique index, now `semantic_key LIKE
  'mpr:%'`), at most once per comment.

### 4.3 Adapter routes (claims-worker)

`(facebook|instagram, meta.dm_send|meta.private_reply|meta.public_reply|meta.offer_recommend, service)` use the §6.4
`LoadSecret`/`DispatchWithSecret` pair: `LoadSecret` loads the Page token (existing loader) **and** the send secret
(§3.4). Check (lock-free, no network, `commerce_claims_worker` pool) re-reads: binding/capability, deadline, window
(DM, via `inbox.dm_window`), takeover (only for `origin=auto`, §3.6), principal still holds `inbox:reply` (human),
comment budget (private reply), outbound row exists. Outcome mapping as MCI §6.3: 2xx with id → SUCCEEDED
(`provider_reference` = message/comment id); documented permanent 4xx (LC-U9 once probed) → FAILED_FINAL with code; a
400 whose Graph `error.code ∈ {100, 2018278}` or a size/format `error_subcode`, **with no `message_id`/`id` in the
body**, is a request Meta provably did not accept → FAILED_FINAL `invalid_request`; any other 4xx before LC-U9, 5xx,
429, timeout, transport, unparsable → UNKNOWN; Reconcile query-only. A Finish hook writes `inbox.bundle_peers`
(private replies) and `conversation_state.last_outbound_at`, and wipes the send secret (§3.4). (`human_until` is set
by the planning transaction of a human DM, §3.6.)

### 4.4 Visible send states

Thread items and comment marks expose `send_state` ∈ `queued` (READY/DISPATCHING), `sent` (SUCCEEDED), `failed`
(FAILED_FINAL + code), `blocked` (BLOCKED_POLICY/STALE_BINDING + code), `unknown` (UNKNOWN: 「不確定是否送達，請到
Messenger 確認，系統不會重送」). The auto first-reply state (M08 #4/#6) is exposed the same way.

## 5. Order for a buyer (幫他建立訂單)

### 5.1 Flow

1. `GET …/inbox/order-prefill?bundle_id=` or `?conversation_id=` → `{items: [{sku_id, offer_id, keyword, name,
   variant, quantity, live_price_minor, catalog_price_minor, sellable}], bundles: [bundle_id], customer: {name, phone,
   email} | null, last_delivery: {option_key, cvs: {store_code, store_name, store_address}} | {option_key,
   home_address} | null, suggested_option_key, live_price_eligible: bool, live_price_reason}` — items = the open claim
   lines of the actor's bundles (via §3.7 for a conversation; capped at 50). **Customer and delivery are pre-filled
   only from an explicit merchant link** (`conversation_state.customer_id`, A14; shown 「已手動連結」, from that
   customer's most recent order, requires `orders:read`); for an unlinked conversation or a bare bundle `customer` and
   `last_delivery` are `null` and the UI shows empty fields. `claims.bundles.owner_id` is never used (I09).
   `suggested_option_key` = 7-ELEVEN pickup when the linked customer's last order used it.
2. UI edits quantities (± per line, 0 removes), picks delivery and payment from the **reused** `GET
   …/orders/manual/options`.
3. `POST …/orders/for-buyer` (new) — body exactly `{items, customer, delivery, payment_mode, locale, for:
   {bundle_ids: [uuid] (0..5), conversation_id: uuid | null}, send_payment_link: bool}`; `Idempotency-Key` required.
   `for.bundle_ids` empty = a plain manual order (no live price). Server, in order:
   a. **One live order per bundle** (reservation row, because Place spans several transactions and no lock can be
      held across it). One merchant transaction inserts one row per bundle into `inbox.order_for_buyer(tenant_id,
      store_id, bundle_id, request_id, idempotency_key_hash, order_id NULL, state text CHECK IN
      ('pending','placed','released'), conversation_id, principal_id, created_at)` (0125) with `UNIQUE (tenant_id,
      store_id, bundle_id) WHERE state IN ('pending','placed')`, bundles in sorted order. Before inserting it locks any
      existing live row `FOR UPDATE` and releases it when its order is CANCELLED or it is `pending` older than 15 min
      with no order; a remaining live row of another request, or a bundle whose claim lines are fully consumed in
      `claims.live_price_uses`, → `409 bundle_already_ordered {order_id | null}` (UI offers 「查看訂單」, not a second
      order). A Place failure sets the rows `released`; the same Idempotency-Key finds its own `pending` rows and resumes.
   b. Live-price eligibility (§5.3) and, only when eligible, the single-use grant row.
   c. The **existing** `merchanttools.ManualOrders.Place` pipeline unchanged (SetCart → CreateQuote → Begin; the Quote
      is the only price, I05/I08; stock reserved by `begin_hold`, I03).
   d. One merchant transaction: set the reservation rows `placed` with `order_id`, audit
      `order.for_buyer_created` (+ `claims.merchant_origin_granted` when a grant was used).
4. If `send_payment_link` and the conversation's 24 h window is open, plan one `meta.dm_send` with template
   `order-pay-link/v1` (link sealed only in the send secret, §3.4), semantic key `mdm:` + hex(sha256(conversation |
   "order-pay-link" | order_id))[:48] — so no replay can ever plan two. Window closed or no conversation → order is
   created, `send: {state: "not_sent", reason: "window_closed" | "no_conversation"}` and the UI offers copy-link.
5. Response = the `ManualResult` of `POST orders/manual` + `{live_price: "applied" | "not_applied", live_price_reason,
   send: {operation_id, state} | {state:"not_sent", reason}}`.

**Resumable under the Idempotency-Key** (Place spans several transactions, so a crash may fall between 3c, 3d and 4):
a replay (a) reads the order from `command.Run` (or the Place step receipts), (b) completes its `inbox.order_for_buyer`
reservation rows (`placed`, `order_id`), (c) if `send_payment_link` and no `meta.dm_send` exists for `(conversation, order_id)`, calls the
**reused** `regenerate-link` logic (the previous buyer link is superseded) and plans the DM with the new link. A
replay otherwise returns the same order and operation, with `buyer_link` null (as today).

### 5.2 Reused vs new

| Endpoint | Status |
| --- | --- |
| `GET …/orders/manual/options` | reused unchanged |
| `POST …/orders/manual` | reused unchanged (still the plain manual order) |
| `POST …/orders/manual/regenerate-link` | reused unchanged (also called internally by a for-buyer replay, step c) |
| `GET …/live-sessions/{sid}/claims/bundles`, `POST …/claims/bundles/{id}/link` | reused unchanged (send the claim link instead of building an order) |
| `GET …/inbox/order-prefill` | **new** |
| `POST …/orders/for-buyer` | **new**, a thin wrapper over `ManualOrders.Place` + bundle lock/record + optional grant + optional DM |

### 5.3 Card and live price

- Card: per G3 ruling 2026-10-05, `payment_mode` may include PAYUNi card **when the manual-order options list it**
  (the change belongs to the G3/PAYUNi unit; this contract only passes the mode through).
- Live price (OPEN-13, RESOLVED with the integrator money-path ruling below): a **merchant-attested claim origin**.
  For lines whose offer has a live price, the server sets `CartInput.Origins` (server-only field) and
  `claims.live_prices` gains exactly one branch: the cart's buyer capability was issued by this for-buyer request and
  holds a matching **single-use** grant row `claims.merchant_origin_grants(tenant_id, store_id, request_id, buyer_id,
  bundle_id, quote_id NULL, principal_id, expires_at = now()+15 min, consumed_at NULL)` (0125); the branch honours a
  grant only for the one quote it is bound to (bound at CreateQuote), and the Begin transaction consumes it; a second
  quote on the same grant gets catalog price. The bundle is in the store, not purged, its session not archived; the
  0105 consumption ledger still caps the live price at the claimed quantity across all orders (LPC01–06 stay green).
  No link-expiry requirement (the merchant attests). Granting requires `live:manage` in addition to
  `inventory:reserve`, because it changes price.
- **Eligibility is fail-closed** (integrator condition (1)): with `for.bundle_ids` non-empty, a grant is written only
  when `for.conversation_id` is present, the thread's `peer_key` exists, every bundle has an `inbox.bundle_peers`
  peer key, and they are all equal. Outcomes:
  - peer unknown on either side, or no conversation (e.g. 幫他建立訂單 from a comment with no linked thread), or the
    caller lacks `live:manage` → **no grant**; the order is created at **catalog price**, `live_price: "not_applied"`,
    `live_price_reason: "bundle_buyer_unverified" | "no_conversation" | "permission"`, and the drawer shows
    「直播價不適用代建訂單，請改傳認領連結」 before the merchant confirms (prefill `live_price_eligible=false`). Never
    the live price.
  - both peer keys known and different → `409 bundle_buyer_mismatch`, nothing created.
- Draft orders are not introduced (deviation A9/D5): the order exists and holds stock as soon as it is created.

## 6. Capability states (§10.2), exposed to the UI

The table `integration.binding_capabilities` and its probe belong to W1-01B (`meta-connection-health-v1.md`). This
contract freezes the capability vocabulary the console consumes:

| Capability | Requires (all of them set `state=ok`) | Evidence label upgrades |
| --- | --- | --- |
| `read_comment` | `pages_read_engagement`, task `MODERATE` (L1); IG `instagram_manage_comments`; subscription `feed` | first successful console poll → `LIVE_READ` |
| `private_reply` | `pages_messaging`, task `MESSAGING`; IG `instagram_manage_messages` (per connect clause 2) | first SUCCEEDED private reply → `LIVE_SEND` |
| `dm_session` | `pages_messaging`, task `MESSAGING`; subscription `messages` (+ IG app-level `messages`); LC-U11 closed | first inbound DM projected → `LIVE_READ`; first SUCCEEDED DM → `LIVE_SEND` |
| `reply_public` | `pages_manage_engagement`, task `MODERATE` (L2); IG `instagram_manage_comments` | first SUCCEEDED public reply → `LIVE_SEND` |

`state=ok` comes from permissions + tasks + subscription read back (W1-01B probe); a successful send or read only
upgrades the `evidence` label, it is never a precondition (no chicken-and-egg on the first send).

Per capability and binding: `state ∈ ok | missing_permission | missing_task | not_subscribed | reauth_required |
review_required | unsupported | unknown`, `reason` (fixed code), `evidence ∈ DESIGN | MOCK | LIVE_READ | LIVE_SEND`,
`checked_at`. `review_required` = permission granted only through Standard Access (app-role users only; App Review
pending). Exposed through the W1-01B route and embedded in the console read model (§7.1) as `capabilities`.
**UI rule**: a send control is rendered enabled only when its capability is `ok` (or `review_required` with the
badge 「僅測試帳號」); otherwise it is absent or disabled with the reason (arch §10.2 "没有公开回复能力时UI不显示可用按钮").
Server rule: the planner refuses with `409 capability` regardless of what the UI shows; Check re-reads it.

## 7. Console read models

### 7.1 `GET …/live-sessions/{sid}/console` (`live:read`; polled every 5 s)

```
{session: {id, title, lifecycle: draft|live|ended|archived, version, started_at, ended_at},
 window: {state, generation, opened_at, match_mode},
 stats: {comments: {total, source: graph_summary|stream_seen|unavailable}, keyword_comments, buyers,
         orders: {count, amount_minor}, paid: {count, amount_minor}, currency, as_of},
 offers: [ConsoleOffer], capabilities: {facebook?: {...}, instagram?: {...}}, stream: StreamState,
 recommended: {offer_id, at} | null}
```

- `comments.total` comes from the poller (`summary.total_count`, else count seen) via the bridge, never stored;
  `unavailable` when no poller.
- `keyword_comments` = intakes of the session with `grammar_kind <> 'NO_MATCH'` + manual claim events; `buyers` =
  bundles of the session.
- Orders of the session = A5-1 attribution (`claims.live_price_uses ∪ claims.order_origins`, ruling L1-2);
  `orders` = non-CANCELLED orders, amount = order total; `paid` = orders with a trusted paid fact (I05). The UI label
  營業額 = `paid.amount_minor`; 訂單金額 = `orders.amount_minor`.

`ConsoleOffer = {offer_id, keyword, sku_id, product_name, variant_label, active, version, live_price_minor,
sku_price_minor, stock: {tracked, sellable, reserved, warehouse_id, balance_version}, claimed: {buyers, quantity},
ordered_qty, paid_qty, paid_amount_minor, sold_out, low_stock}`. `sellable` per the 0086 formula; `low_stock` =
tracked ∧ 0 < sellable ≤ 5; `sold_out` = tracked ∧ sellable ≤ 0. ordered/paid quantities = Σ order-line quantity of
the offer's SKU over the attributed orders whose (order, offer) pair is in the attribution union.

### 7.2 Offer on/off and live stock

Reused unchanged: `PATCH …/claims/offers/{offer_id}` `{active, expected_version}` (on/off, 開放/關閉) and
`{live_price_minor}`; `POST …/inventory/adjustments` with `delta = target − sellable` and the balance's
`expected_version` (CAS; conflict → UI re-reads). Permission: `inventory:write` as today, **or** the new narrow
`inventory:live_adjust` (OPEN-17: granted to `live_operator` instead of `inventory:write`). An adjustment authorized
only by `inventory:live_adjust` is bounded in `inventory.AdjustOnHand` (LC-B7): reason forced to `live_console_edit`,
`|delta| ≤ 1000` per call, the resulting on-hand never below `reserved + allocated` (`422 below_reserved`), and audit
`inventory.adjusted` carries the reason (I03: a live operator cannot silently rewrite stock truth). No auto-pause on sold out in
v1 (claims never touch stock, arch §11.2; the buyer sees sold-out at checkout, claim-direct-checkout B1).

### 7.3 Recommend (推薦) an offer

`POST …/live-sessions/{sid}/claims/offers/{offer_id}/recommend` `{expected_version, post_comment: bool}`
(`live:manage`; `post_comment` additionally `inbox:reply`). Writes the `offer featured` timeline event of W2-01B
(new table `live.offer_timeline` in 0120, append-only `(session, offer, kind='featured', at, principal)`, used by
逐品成效), sets `recommended` in §7.1, and when `post_comment` plans
`meta.offer_recommend` (FB only; IG → `422 ig_live_unsupported`). Graph has no documented "pin a live comment" call;
"pin" in the UI means the console highlight plus this optional Page comment (OPEN-18).

### 7.4 Comment-label print record (列印)

`POST …/live-sessions/{sid}/comments/{comment_ref}/print` (`live:manage`, idempotent per key) → `live.comment_prints
(tenant_id, store_id, session_id, comment_ref, print_count, first_printed_at, last_printed_at, last_principal_id)`
(0123, FORCE RLS, upsert by definer). The label content (display name, keyword, quantity, time) is rendered by the
browser from the in-memory comment it already holds (W3-U3); the server stores only the fact of printing. Retention
class C3 (`intake_days`).

## 8. Several OPEN windows per store

Replace `live_claim_window_one_open (tenant_id, store_id)` with a per-store cap. Safe because, and only while:
1. every Meta comment maps to at most one session (`claim_sources UNIQUE(object, asset_id, source_object_id) WHERE
   active`, MCI03) and ingest already filters `session_id` (`ingest.go:150`);
2. the manual record path names the session in its URL (`…/live-sessions/{sid}/claims/manual`) and the handler uses
   that id, never "the current OPEN window". Known callers that look at OPEN windows, each re-checked by W2-01B:
   `internal/catalog/document.go:488,498` (store-wide `EXISTS` on OPEN windows — stays correct, but now blocks keyword
   edits while any of up to 5 windows is OPEN; accepted), `migrations/0071_claims_retention.sql:529` (C2 reopen guard,
   per session: safe), the 0079 billing guard (new opens only: safe). Grep gate in LCN09: no query selects a single
   OPEN window without `session_id`;
3. bundles, lines, links and live-price consumption are per session/bundle (unchanged);
4. a cart merging claims of two sessions yields one order counted in both sessions (L1 ruling 3, `multi_session_orders`
   footnote) — totals across sessions are never summed in one figure;
5. a cap of **5 OPEN windows per store** (OPEN-15), enforced in `claims.SetWindow` (`internal/claims/merchant.go:151`) under a store-level advisory lock taken before the count
   (`409 too_many_open_windows`), bounding pollers (I23); billing BD5 still blocks opening any new window when
   RESTRICTED.

Gate LCN09: two sessions OPEN at once on two Pages; the same buyer comments the same keyword on both → two bundles,
two auto replies, two links; closing one does not affect the other; KC/MCI suites stay green.

## 9. Session lifecycle (W2-01B; amends studio-v1)

New column `live.sessions.lifecycle ∈ draft → live → ended → archived` (0120; separate from the media programme state,
arch §9.1; the existing `state` column, `CHECK (state='DRAFT')` in 0033, is the planning state and stays untouched).
`POST …/live-sessions/{sid}/lifecycle {action: start|end|archive, expected_version}` (`live:manage`, CAS, audit
`live.session.started|ended|archived`). `start` = lifecycle `live` and opens the claim window if closed (unless
`open_window=false`); `end` = closes the window (existing close path, interval closes) and stops demand-free polling;
`archive` only from `ended`, read-only afterwards. **Late comments after `end`** (OPEN-16): displayed if the console is
open, counted in `comments.total`, never claimed (window CLOSED → DROPPED as today, MCI06 grace 60 s unchanged).
Reopening an ended session = `start` again (new window generation), not allowed after `archive`.

## 10. Retention (amends claims-retention-purge-v1 §1)

| Class | Rows | Eligible | Action |
| --- | --- | --- | --- |
| C5 (extended) | `inbox.outbound_messages` | `created_at < now() − social_days` **regardless of operation state** (an UNKNOWN operation never becomes terminal); the frozen request is redacted by C4 | DELETE |
| C5c | `inbox.conversation_state`, plus the `inbox.outbound_messages` of the conversation | the conversation is selected by C5b | DELETE in the same batch **before** the conversation delete (order C5 → C5c → C5b); `conversation_state` has no FK to `social.conversations` (§3.6) |
| C7 | `inbox.send_secrets` | operation terminal (wiped at completion) or `created_at < now() − 8 d` | DELETE |
| C3 (extended) | `live.comment_prints`, `inbox.bundle_peers` | `created_at < now() − intake_days`; bundle_peers also on bundle de-identify (C2) and actor erasure (RD4) | DELETE |
| C4 (extended) | `integration.operations` actions `meta.dm_send/public_reply/offer_recommend` and manual `meta.private_reply` | terminal and `created_at < now() − intake_days` | redact `comment_ref`, `peer_key`, `conversation_id` as RD-C4 |

Actor erasure (RD4 / `apply_actor_erasure`): for every `peer_key` it resolves, it also deletes that peer's
`inbox.outbound_messages`, `inbox.conversation_state`, `inbox.send_secrets` and `inbox.bundle_peers` rows.
`claims.merchant_origin_grants` rows are deleted at consumption + 1 d or expiry + 1 d. Comment text and names have no
class: they are never stored (§2.1). `inbox.order_for_buyer` follows order retention.

## 11. API list

Base `/v1/admin/stores/{store_id}`. Every route: merchant bearer → server-resolved tenant/store (I01), strict JSON
decoder (unknown keys 400), `Cache-Control: no-store`, errors `{code}` from the fixed list. Writes need
`Idempotency-Key`. New permissions (0122, OPEN-7/17): `inbox:read`, `inbox:reply`, `inventory:live_adjust`.

| # | Method path | Permission | Request → Response | Errors | Audit |
| --- | --- | --- | --- | --- | --- |
| A1 | GET `/live-sessions/{sid}/console` | live:read | — → §7.1 | 404 not_found | — |
| A2 | GET `/live-sessions/{sid}/comments` | live:read | query §2.6 → §2.6 | 404, 409 no_source, 503 stream_unavailable | — |
| A3 | POST `/live-sessions/{sid}/comments/{ref}/print` | live:manage | `{}` → `{print_count, last_printed_at}` | 404, 422 invalid_ref | — (the row is the record) |
| A4 | POST `/live-sessions/{sid}/comments/{ref}/private-reply` | inbox:reply | `{text}` \| `{template_id, template_version}` → `{operation_id, outbound_id, send_state}` | 409 used\|auto_pending\|expired_7d\|ig_live_ended\|page_comment\|reply_comment_unsupported\|comment_unknown\|capability\|duplicate_recent, 422 invalid_text, 429 rate_limited | inbox.private_reply.planned |
| A5 | POST `/live-sessions/{sid}/comments/{ref}/public-reply` | inbox:reply | same as A4 | 409 capability\|ig_live_unsupported, 422 public_reply_forbidden_content | inbox.public_reply.planned |
| A6 | POST `/live-sessions/{sid}/claims/offers/{oid}/recommend` | live:manage (+inbox:reply if post_comment) | `{expected_version, post_comment}` → `{recommended_at, operation_id?}` | 409 version_conflict\|offer_unavailable\|capability, 422 ig_live_unsupported | live.offer.recommended |
| A7 | POST `/live-sessions/{sid}/lifecycle` | live:manage | `{action, expected_version, open_window?}` → `{lifecycle, version, window}` | 409 version_conflict\|invalid_transition\|too_many_open_windows, 402 billing_restricted | live.session.started\|ended\|archived |
| A8 | GET `/inbox/conversations` | inbox:read | `?filter=all\|unreplied\|messenger\|instagram\|live_comment&session_id=&cursor=&limit≤50` → `{items: [{conversation_id?, bundle_id?, platform, display_name?, last_at, unread, unreplied, mode, assignee, window_open_until, linked_customer_id, link_version}], next_cursor, unread_total}` (amendment "LC-B3b" adds link_version, the session_id filter and live_comment rows with an opaque keyset cursor) | 400 invalid_filter | — |
| A9 | GET `/inbox/conversations/{cid}/messages` | inbox:read | `?before_seq=&limit≤50` → `{items: [{direction, seq?, at, text, attachments, kind?, send_state?, principal_id?, unreadable?}], window_open_until, mode, takeover_generation, human_until, link_version, binding_id, has_unknown_outbound: boolean\|null}` | 404 | `inbox.thread_opened` (principal, conversation; coalesced ≤ 1 per principal per conversation per hour; no content) |
| A10 | POST `/inbox/conversations/{cid}/read` | inbox:read (effective only with inbox:reply, §3.6) | `{read_seq}` → `{read_seq}` | 404, 422 | — |
| A11 | POST `/inbox/conversations/{cid}/takeover` · `/release` | inbox:reply | `{expected_generation}` → `{mode, assignee, takeover_generation}` | 409 takeover_changed | inbox.takeover\|inbox.release |
| A12 | POST `/inbox/conversations/{cid}/messages` | inbox:reply | `{text \| template ref, expected_generation}` → `{operation_id, outbound_id, send_state, takeover_generation}` | 409 window_closed\|takeover_changed\|capability\|conversation_gone\|duplicate_recent, 422 invalid_text, 429 rate_limited | inbox.dm.planned |
| A13 | GET `/inbox/buyer-panel` | inbox:read (+orders:read for orders) | `?conversation_id=` \| `?bundle_id=` → `{display_name?, platform, purchase_ordinal, claims: [{session_id, offer_id, keyword, quantity}], claim_total_minor, orders: [{order_id, number, state, total_minor, created_at}], auto_reply: {send_state}?, linked_customer_id?, window_open_until?}` | 404, 400 | — |
| A14 | POST `/inbox/conversations/{cid}/customer-link` | inbox:reply + customers:read | `{customer_id \| null, expected_version}` → `{customer_id, version}` | 409 version_conflict, 404 | inbox.customer_linked\|unlinked |
| A15 | GET `/inbox/order-prefill` | orders:read + inventory:reserve | `?bundle_id=` \| `?conversation_id=` → §5.1 step 1 | 404 | — |
| A16 | POST `/orders/for-buyer` | inventory:reserve (+live:manage for a live-price grant, +inbox:reply if send) | §5.1 step 3 → §5.1 step 5 | the `orders/manual` codes + 409 bundle_already_ordered\|bundle_buyer_mismatch\|capability | order.manual_created + order.for_buyer_created (+claims.merchant_origin_granted, +inbox.dm.planned) |
| — | `/message-templates` (W2-05B) | live:manage / inbox:reply | owned by W2-05B; this contract fixes only `{template_id, version, public_safe, kinds}` | | template.published |

A9 additive delivery guard (PR #8 K3 ruling, 2026-10-08): every page returns `has_unknown_outbound` independently
of its display window. `true` means a final UNKNOWN was found; `false` is allowed only when the outbound history
was exhaustively checked and all states were known. `null` means the bounded read cannot exclude an older UNKNOWN
or an operation state is unavailable. The current existing definer reads at most 50 outbound rows: hitting that cap
without seeing UNKNOWN must return `null`, even if all displayed rows look successful. The UI permits sending only
on explicit `false`, retains any already-observed UNKNOWN, and shows an unavailable reason for null/missing authority.
A future unbounded metadata fact may restore eligibility for longer clear histories; this conservative fallback adds
no database privileges or migration and never equates an omitted message with delivery reconciliation.

Text inputs (A4/A5/A12): limits per §3.3 (Messenger 2000 runes, Instagram 1000 bytes, public 300 runes), NFC, no
control chars except `\n`; template refs resolve only
published versions of the store. The internal bridge (§2.3) is not part of this list and never in the admin BFF
allowlist. BFF allowlist additions (UI units): A1–A16.

Role defaults (OPEN-7, OPEN-17), changed in `identity.staff_role_permissions` itself (so later admins get them) and
backfilled for existing grants: `live_operator` += `inbox:read, inbox:reply, inventory:live_adjust` (not
`inventory:write`); owner/admin get `inbox:*` and `inventory:live_adjust` through the catalogue; `viewer` excludes
`inbox:read` **explicitly** (DM text is buyer PII, not an analytics read; LCN03 asserts it); fulfilment unchanged.

## 12. Privacy and security checklist

- Comment/DM text, names and PSIDs never in logs, River args, audit, metrics, PG (except the existing encrypted
  copies and §3.4 sealed copies), URLs, browser storage (I11); console responses `no-store` (I15).
- Bridge token and HPKE/payload keys from `*_FILE` secrets only; the bridge listener is not on the edge network.
- Cross-tenant/store reads return 404 for A1–A16 (gate LCN03); conversation ids are checked against the bearer's
  store inside the definer, never trusted from the path alone.
- Public replies cannot carry links/PII (§3.5); DMs may carry the payment link (inside the window only).
- No LLM, translation or automated free text; automated messages remain fixed templates.
- A4/A5/A12/A16 pass `command.Run` a canonical request of `{kind, target ids, body_hmac, expected_generation}` (A16:
  its non-text fields); the plaintext never reaches `ops.command_results` (request hash or stored response), access
  logs, BFF logs or SQL parameters visible to `pg_stat_statements` — the API seals before any SQL, and responses
  never echo `text`. LCN04's sentinel scan covers `ops.command_results`, BFF access logs and River job rows.

## 13. Evidence classes, gates and LIVE probe

### 13.1 What each class can prove

| Class | Proves | Cannot prove |
| --- | --- | --- |
| MOCK (REAL_PG + fake Graph `httptest`) | poller lease/backoff/ring/epoch; bridge auth; marks join; inbox read/decrypt; every send rule, quota, takeover race, UNKNOWN handling; for-buyer pipeline; retention | real payload shapes (LC-U1..U9), permissions, App Review, Meta-side windows |
| BROWSER (Chromium/WebKit, MOCK backend) | v5 console and inbox by real clicks, polling cadence, disabled controls by capability | anything about Meta |
| LIVE_READ (app-role test user) | comment read object/latency, `from` presence, `subscribed_apps` = feed,messages, inbound DM projected | sends |
| LIVE_SEND (app-role test user, owner-authorized test Page) | DM RESPONSE, manual private reply, public reply, recommend comment delivered and read back; `recipient_id` present; error codes | non-role users (needs App Review / Advanced Access) |

Everything Meta-facing is **MOCK until §13.3 passes**; the capability `evidence` field shows it.

### 13.2 Gates (all NOT_RUN; tests in `tests/foundation/live_console_*_test.go`, `tests/admin/live-console*.spec.ts`)

New `scripts/dev/test-local.sh` modes: `--live-console` (REAL_PG + fake Graph: poller, bridge, marks, lifecycle,
multi-window, read models), `--inbox` (REAL_PG: read definers, decrypt, permissions, cross-tenant), `--inbox-send`
(REAL_PG + fake Graph: sends, quota, takeover, UNKNOWN), `--msg-templates` (W2-05B), `--browser-live-console`,
`--browser-inbox`; `--browser-manual-order` gains the for-buyer case. Units touching migrations/GRANTs also run
`release-gate.sh --strict --only G07` at acceptance and after merge.

| Gate | Evidence | Required |
| --- | --- | --- |
| LCN01 | MOCK | Poller: one lease per source across two worker processes; lease loss stops polling; token loader refuses a stale generation and is re-called each renewal (disconnect → stop ≤ 10 s, buffer zeroed); backoff ladder on usage headers, 429, codes 4/17/32/613 shared per asset; 190 → reauth; fleet/tenant caps; no polling for draft/archived; ring cap/age/idle drop; deleted comment evicted; lease handover keeps `poll_epoch`, real buffer loss → `reset:true`; fake Graph embeds `access_token` in `paging.next` — never called, token never in a URL or log; tampered/foreign/expired `older_cursor` → 400; p95 lag ≤ 5 s with 5 tabs on one session |
| LCN02 | MOCK | Bridge: wrong/missing token 401; edge network cannot reach it (compose smoke); tenant/store/session/source mismatch 404; non-holder → 421 `not_poller_owner` → API `unavailable`; `comment-facts` found/page/reply cases; bodies absent from logs; compose `replicas=1` |
| LCN03 | MOCK | Cross-tenant and cross-store A1–A16 → 404; `viewer` cannot A8/A9 (explicit exclusion holds after a catalogue change); `live_operator` can; role-bundle and backfill for owner/admin; `inventory:live_adjust` bounds (reason forced, ±1000, never below reserved+allocated) |
| LCN04 | MOCK | **Leak scan**: sentinel comment text, names, PSID, link token absent from every table (incl. `ops.command_results`), audit, River args/job rows, operation requests, API/BFF access logs and SQL parameters after a full console session (I11) |
| LCN05 | MOCK | Marks: auto-claimed, dropped, rate-limited, unknown-keyword, manual-replied, public-replied, printed comments map correctly with no text join |
| LCN06 | MOCK | DM: inside window → one operation; window − 4 min → refused at plan; window closing between plan and Check → BLOCKED_POLICY zero HTTP; echo, reaction, read-receipt and delivery rows never open a window; future `occurred_at` clamped; never a tag in any request body (fake Graph asserts); per-platform text limits; rate cap and `duplicate_recent`; 400 code 100 without `message_id` → FAILED_FINAL `invalid_request` |
| LCN07 | MOCK | Private reply quota: manual after auto → `used`; manual while PENDING → `auto_pending`; manual first then auto apply → `claim_reply_skipped:reply_used`, claim ACCEPTED, `link_pending_manual=true`; manual and auto in opposite orders under `-race`, 100 iterations → never an ACCEPTED claim with neither a link operation nor `link_pending_manual`; 20 concurrent manual attempts → one operation; auto BLOCKED_POLICY → one `:m1` manual allowed, a second refused; child comment → `reply_comment_unsupported`; unknown comment → `comment_unknown` |
| LCN08 | MOCK | Public reply: each §3.5 pattern rejected server-side, incl. `ｗｗｗ．ａｂｃ．ｃｏｍ`, `０９１２３４５６７８`, `0912-345-678`, `l i n e`, zero-width splits, `t.me`, store origin; template `public_safe` re-validated; IG live refused; rendered recommend comment replayed through the fake webhook creates no claim/intake/reply |
| LCN09 | MOCK | Multi-window §8 cases; cap 5; manual record lands in the URL session only; grep gate: no single-OPEN-window query without `session_id` |
| LCN10 | MOCK | Takeover: implicit takeover on first DM send only (public reply/recommend never); automated send with `conversation_known=false` resolved at Check → `human_takeover`; known at gen N then human takeover → `human_takeover`; release → gen+1 → old automated op `takeover_changed`; human DM queued then released before dispatch → still sent (no generation check for `origin=human`); takeover expires 6 h after the last human DM; stale `expected_generation` 409; `read` by an `inbox:read`-only principal leaves unread unchanged |
| LCN11 | MOCK | UNKNOWN: 5xx/timeout/garbled → UNKNOWN, query-only, never a second POST (child-process kill after send); send secret wiped at SUCCEEDED, FAILED_FINAL, BLOCKED_POLICY, STALE_BINDING and UNKNOWN |
| LCN12 | MOCK | For-buyer: prefill from bundle and conversation — customer/delivery only from an explicit A14 link, never from `owner_id`; quantities edited; Quote is the only price (request price → 400 unknown key); two different keys on one bundle → one order, one hold; staff A and B concurrently → one `409 bundle_already_ordered`; kill after Begin before the record → replay completes, one order, one DM operation; DM planned only inside window; live price: unknown peer → catalog + fallback text, no conversation → catalog + fallback text, missing `live:manage` → catalog, mismatched peers → 409, grant reused by a second quote → catalog; LPC01–06 stay green |
| LCN13 | MOCK | Retention §10: UNKNOWN operation older than `social_days` → display copy gone; C5b with populated `conversation_state` succeeds; actor erasure of a peer leaves zero rows in outbound/state/secrets/bundle_peers; send secrets older than 8 d removed; display copy has no link (typed, pasted, full-width) |
| LCN14 | BROWSER | v5 flow by real clicks: 開始直播 → comment appears with claim tag → private reply/public reply toggle → DM → 幫他建立訂單 → payment link sent; disabled controls by capability; IG notice; click ledger |
| LCN15 | LIVE_READ | §13.3 R1–R4 |
| LCN16 | LIVE_SEND | §13.3 S1–S5 |

### 13.3 LIVE probe plan (pilot, app-role test user, real OAuth)

Preconditions (owner inputs; Meta UI only through Kimi WebBridge with owner consent for that change): app 大夢 in
development mode with the FLfB configuration containing `pages_manage_engagement` (added 2026-10-05) and
`pages_messaging`; app-level webhook fields Page `feed, messages` and Instagram `comments, live_comments, messages`;
an owner-controlled **test Page** (+ IG professional account) and an **app-role test user** (tester role) who is not a
Page admin. Credentials stay in `secrets.env`/the browser session, never in the repo or chat.

1. **Connect through the real flow**: on the pilot admin, a merchant account of the owner's pilot store clicks
   Connect → Facebook Login for Business dialog → picks the test Page with IG. Record: granted permissions, tasks,
   `subscribed_apps` readback contains `feed,messages` (R1). Existing connection: run `meta_resubscribe_v1` and read
   back (R1b).
2. **R2 comments**: owner starts a short FB live on the test Page (Live Producer); the test user comments `A1`,
   `A1+2`, a question, and a reply to a comment. Record LC-U1 (which object id returns them, p50/p95 lag over ≥ 20
   comments), LC-U2 (`from` present?), webhook `post_id` form (MCI U1).
3. **R3 IG**: same on an IG live (LC-U3 Graph read vs webhook-only).
4. **R4 DM in**: test user sends a Messenger DM and an IG DM → `social.messages` row, inbox shows text (LC-U4);
   record the Page's primary receiver / handover state and that Business Suite can still reply (LC-U11).
5. **S1 auto reply** (existing MCI12) on the `A1` comment; **S2 manual private reply** on the question comment;
   record `recipient_id` (LC-U6) and the `bundle_peers` link; a second manual reply on the same comment is refused
   locally (no HTTP).
6. **S3 DM RESPONSE** inside the window; then a conversation whose last inbound is > 24 h old → refused locally;
   one direct Graph attempt outside the window is **not** made (we never test Meta's enforcement by violating it).
7. **S4 public reply** on the question comment (FB) and on an IG non-live post; verify visible on the Page;
   **S5 recommend** comment on the live video.
8. Error codes (LC-U9) are recorded only when Meta returns them naturally (e.g. a reply to a deleted comment).
Evidence: `output/live-console-probe/<date>/` (redacted JSON shapes, screenshots, no names/text of real people —
the test user is an app-role account). Sends are to the app-role test user only, under the owner's 2026-10-05
whole-chain authorization for Meta testing; anything reaching a non-role person needs explicit owner approval.

## 14. Amendment clauses (recorded by the integrator at freeze)

### 14.1 meta-claims-intake-v1
1. §1 Out: remove "social read UI, human takeover UI, DM conversations after the reply, public comment replies";
   they are governed by live-console-v1.
2. §6.1: "a 23505 is therefore an invariant breach → intake FAILED(reply_key_conflict)" is replaced by: an existing
   `mpr:` operation of message_type `manual_private_reply` or `out_of_stock_reply` → audit
   `claim_reply_skipped:reply_used`, claim and intake commit (no operation, no link) and
   `claims.bundles.link_pending_manual := true`; any other 23505 stays an invariant breach. `claim_reply_plannable`
   gains skip code `reply_used`. The intake apply's `FOR UPDATE` on `claims.meta_intake` (already in §5.3) is the row
   the manual planner locks `FOR SHARE` (live-console §4.2). The global index becomes `semantic_key LIKE 'mpr:%'`
   (admits the `:m1` class).
3. §6.2: request gains `origin:"auto"`, `conversation_known` and `takeover_generation` (the generation of the actor's
   known conversation via `inbox.bundle_peers`, else 0 with `conversation_known=false`); §6.3 Check gains deny codes
   `human_takeover`, `takeover_changed` per live-console §3.6. "Takeover is always 0 in v1" is removed.
4. Amendment "Merchant connect" clause 3: `subscribed_fields=feed` → `feed,messages`; existing connections are
   resubscribed by `meta_resubscribe_v1` (§3.1).
### 14.2 meta-consumer-v1
"No DM policy/window engine, public sending, … social read UI" known limit → superseded by live-console-v1 §3–4; the
consumer and its gates are unchanged.
### 14.3 meta-inbox-v1
Read authority note: the API process may open `social.messages` / `social.comment_events` envelopes only through
`social.read_thread` (`inbox:read`) and `social.read_comment_events` (`live:read`, IG fallback of §2.4); no inbox
(meta_private) body is ever read.
### 14.4 live-keyword-claims-v1
§1 "At most one OPEN window per store" → "At most 5 OPEN windows per store, one per session (live-console-v1 §8)";
index `live_claim_window_one_open` dropped; KC suites unchanged otherwise.
### 14.5 studio-v1
Session lifecycle `draft|live|ended|archived` and A7 (§9); the media programme state machine is unchanged.
### 14.6 storefront-v2 G3
Manual order is also reachable as `POST orders/for-buyer` (§5); its rules (no price, Quote only, immediate order)
are unchanged; card follows the G3 ruling of 2026-10-05.
### 14.7 claims-retention-purge-v1 / external-operation-v1
§10 rows; new actions `meta.dm_send`, `meta.public_reply`, `meta.offer_recommend` and message_type
`manual_private_reply` on the default lane; C5 deletes outbound rows by age, C5c precedes C5b, RD4 erasure covers the
four `inbox.*` peer tables (§10); CRP02/MCI02/KC03 equality lists gain the 0120–0126 privilege rows of the
implementing units (integrator merges them).
### 14.8 staff roles (merchant-identity / 0089)
`identity.staff_role_permissions`: `live_operator` += `inbox:read, inbox:reply, inventory:live_adjust`; `viewer`
excludes `inbox:read` explicitly; owner/admin via the catalogue plus a backfill of existing grants (§11).

## 15. Open questions — all RESOLVED at freeze (2026-10-05)

- **OPEN-1** Transport. RESOLVED: polling in v1 — comments 3 s, console 5 s, inbox 10 s, paused when hidden (A17);
  LCN01 requires p95 lag ≤ 5 s with 5 tabs on one session; SSE only when that or API load is measured short.
- **OPEN-2** API ↔ claims-worker bridge. RESOLVED: internal HTTP with a shared bearer token on the backend network,
  exactly one claims-worker replica, non-holder answers 421; Graph token only in the Authorization header (§2.3–2.4).
- **OPEN-3** FB poll object. RESOLVED: live video id when the A5-3 picker stored it, else feed post id; LC-U1 decides
  and the losing code path is deleted after R2.
- **OPEN-4** IG live comments. RESOLVED: Graph if LC-U3 passes; else the webhook copy through
  `social.read_comment_events` (no new storage).
- **OPEN-5** Meta streaming endpoint. RESOLVED: rejected (token in URL); reopened only if LC-U5 proves header auth.
- **OPEN-6** Ring buffer. RESOLVED: 2000 comments / 2 h / dropped 10 min after the poller stops, with deletion
  eviction (§2.2).
- **OPEN-7** Permissions. RESOLVED: new `inbox:read`, `inbox:reply`; in the role-bundle definition and backfilled;
  `live_operator` gets both; `viewer` excluded from `inbox:read` explicitly, tested in LCN03.
- **OPEN-8** Takeover trigger. RESOLVED (reviewer counter adopted): implicit only on DM sends (and on a manual private
  reply whose peer is already linked to a thread), never on public replies or recommend; takeover expires 6 h after
  the last human DM (`human_until`), then the thread is `auto` with generation+1; explicit takeover/release remain.
- **OPEN-9** Read state. RESOLVED (reviewer counter adopted): store-level `read_seq`, clamped, and cleared only by
  principals holding `inbox:reply`; a reader without it does not hide the thread from the assignee.
- **OPEN-10** `message_echoes`. RESOLVED: not subscribed in v1; banner 「在 Meta 後台回覆的訊息不會顯示在這裡」 and the
  未回覆 hint 「可能已在 Meta 後台回覆」 after 24 h.
- **OPEN-11** Private-reply quota. RESOLVED: first writer wins on the `mpr:` key, the manual path yields while an auto
  reply is pending or plannable (row lock on the intake), `link_pending_manual` surfaces a claim left without link;
  one `:m1` manual retry only after a provably unsent BLOCKED/STALE auto reply (§4.2).
- **OPEN-12** Public reply content. RESOLVED: §3.5 rule with NFKC + case-fold + Cf strip; no URLs/domains/handles/
  phones/emails/buyer variables, ≤ 300 runes.
- **OPEN-13** Live price on an order built for a buyer. RESOLVED: merchant-attested, single-use, quote-bound grant
  under `live:manage`, capped by the 0105 ledger, one live order per bundle, and **fail-closed**: unknown peer on
  either side or no conversation → catalog price + 「直播價不適用代建訂單，請改傳認領連結」, never the live price;
  differing peers → `409 bundle_buyer_mismatch` (§5.3 and the integrator ruling below).
- **OPEN-14** PSID custody. RESOLVED: never stored in clear; sealed per send to the HPKE public ring and wiped at
  every terminal state including UNKNOWN; `body_hmac`, not an unsalted hash; comment↔thread link via `recipient_id`.
- **OPEN-15** Multi-window cap. RESOLVED: 5 OPEN windows per store, plus fleet (20) and per-tenant (5) poller caps.
- **OPEN-16** Comments after `end`. RESOLVED: shown and counted, never claimed (window closes on end).
- **OPEN-17** Live stock edit. RESOLVED (reviewer bounds adopted, stricter): `live_operator` gets the new bounded
  `inventory:live_adjust`, not `inventory:write`: reason `live_console_edit`, |delta| ≤ 1000, never below
  reserved + allocated, audited (§7.2).
- **OPEN-18** Recommend. RESOLVED: optional Page comment per click, FB only, fixed template, ≤ 1 per offer per 10 min;
  no "pin" API exists.
- **OPEN-19** Outbound retention. RESOLVED: `social_days` (30 d default), deleted **by age regardless of operation
  state** (§10).
- **OPEN-20** Audit of DM reads. RESOLVED (reviewer counter adopted): coalesced `inbox.thread_opened` (principal,
  conversation, ≤ 1 per principal per conversation per hour, no content) on A9.

## 16. Implementation units (backend DeepSeek, UI Codex, tests K3)

Order respects 2 writing units / 4 agents; upstream interfaces frozen before downstream starts.

| Unit | Owner | Covers | Write paths | Migration (placeholder) | Gate | Depends |
| --- | --- | --- | --- | --- | --- | --- |
| LC-B1 = W2-01B lifecycle + multi-window | DeepSeek | §8, §9, §7.3 timeline (`live.offer_timeline`), A7 | `internal/live/lifecycle.go`, `internal/httpapi/live_lifecycle.go`, `internal/claims/merchant.go` (window cap only) | 0120 | `--studio-backend`, `test-focused.sh 'LiveLifecycle'`, LCN09, G07 | A5, A7 merged |
| LC-B2 = W2-02B comment read-through | DeepSeek | §2 (poller, leases, caps, bridge incl. `comment-facts`, cursors, deletion eviction, `social.read_comment_events`), §2.5 marks, §7.4 prints, A2, A3 | `internal/integrations/metareply/comment_poll.go`, `internal/integrations/metareply/bridge.go`, `internal/live/stream.go`, `internal/httpapi/live_stream.go`, `cmd/claims-worker/main.go` (wiring), `deploy/compose.yml` + `secrets.manifest.tsv` (bridge token, `replicas=1`; integrator-merged) | 0123 | `--live-console` LCN01/02/04/05 | A5-3, W1-01B |
| LC-B3 = W2-03B inbox read + subscription + roles | DeepSeek | §3.1 (incl. resubscribe job), §3.2, §3.6 state/window/`dm_window`/read/takeover expiry, §3.7 customer link, A8–A11, A13, A14, permissions `inbox:read`/`inbox:reply`/`inventory:live_adjust` and role bundles (§14.8) | `internal/inbox/**` (read side), `internal/httpapi/inbox.go`, `internal/metaconnect/graph.go` (fields), `internal/integrations/metareply/resubscribe.go` | 0122 | `--inbox` LCN03, LCN10 (read/expiry part), G07 | W1-01B |
| LC-B4 = W2-04B sends + takeover | DeepSeek | §3.3–3.5, §3.6 takeover checks, §4 (planners, quota with intake row lock, `link_pending_manual`, `:m1`, rate caps, adapter routes, Finish hook, `bundle_peers`), A4, A5, A6 comment, A12; §14.1 clauses 2–3 | `internal/inbox/send*.go`, `internal/integrations/metareply/{send_dm.go,public_reply.go,manual_reply.go}`, `internal/integrations/metareply/routes.go` (takeover re-check only), `internal/claims/meta_intake.go` (reply_used skip only) | 0123 | `--inbox-send` LCN06–08, 10, 11, 13; Claude final review | LC-B2 (`comment-facts`), LC-B3 |
| LC-B5 = W2-05B templates | DeepSeek | template ids/versions, `public_safe`, `order-pay-link/v1`, `offer-recommend/v1` | `internal/msgtemplates/**`, `internal/httpapi/templates.go` | 0124 | `--msg-templates` | contract frozen (parallel with LC-B4) |
| LC-B6 = W2-06B order for buyer | DeepSeek | §5 (prefill from explicit link only, reservation rows, resumable replay, fail-closed single-use grant + `claims.live_prices` branch), A15, A16 | `internal/merchanttools/order_for_buyer.go`, `internal/httpapi/merchanttools.go` (routes only), `internal/claims/live_price*.go` (grant branch only) | 0125 | `--browser-manual-order` + LCN12, LPC01–06, G07; Claude money review | LC-B3, LC-B4 |
| LC-B7 console read model + bounded stock edit | DeepSeek | §7.1, §7.2 (incl. `inventory:live_adjust` bounds in `AdjustOnHand`), A1 | `internal/live/console.go`, `internal/httpapi/live_console.go`, `internal/inventory/inventory.go` (bound only) | 0126 (only if a definer or CHECK is needed) | `--live-console`, LCN03 (bounds) | A5-1, LC-B2 (stream stats), LC-B3 (permission) |
| LC-U1 = W2-U1 workspace shell | Codex | v5 nav, three phases, left column (FB embed, IG notice), status bar, offers on/off, live stock, recommend, 5 s polling | `apps/admin/src/features/live/**`, `apps/admin/components/{LiveWorkspace.tsx,LiveConsole.tsx,Studio.tsx,StudioClaims.tsx}`, `apps/admin/src/routes.ts` | — | `--browser-live-console`, `--browser-studio-ui`, `--browser-live-claims` | LC-B1, LC-B7 frozen; v5 |
| LC-U2 = W2-U2 stream + buyer panel + 訊息 | Codex | middle/right columns, filters 全部/關鍵字/私訊/待回覆, reply mode toggle, rule hints, takeover, inbox page | `apps/admin/components/{CommentStream.tsx,BuyerPanel.tsx,Inbox.tsx}`, `apps/admin/lib/inbox-*.ts`, BFF allowlist | — | `--browser-inbox`, `--browser-live-console` | LC-B2..B5 frozen, LC-U1 merged |
| LC-U3 = W2-U3 create-order drawer | Codex | §5 UI | `apps/admin/components/CreateOrderDrawer.tsx`, `ManualOrder.tsx` (extract shared form only) | — | `--browser-manual-order` | LC-B6, LC-U2 |
| LC-U4 = W3-U3 label print | Codex | §7.4 UI, print CSS | `apps/admin/components/CommentLabelPrint.tsx`, `apps/admin/app/print.css` | — | `--browser-live-console` (print DOM) | LC-U2 |
| LC-T1 = W2-T1 independent acceptance | K3 | LCN03/04/06/07/10/14 adversarial + browser | `tests/admin/live-console*.spec.ts`, `tests/foundation/live_console_gate_test.go` | — | red run then green | before LC-U3 merge |
| LC-X1 LIVE probe | Claude (Sonnet) + Kimi WebBridge | §13.3 | `output/live-console-probe/` | — | LCN15/16 | LC-B2..B4 merged, owner preconditions |

Every writing unit uses its own worktree/branch, the AGENT-PREAMBLE lifecycle (red → green, header ratchet), and
writes `output/<unit>/DELIVERY.md`. Migration numbers, OpenAPI, shared JSON schema, `go.mod/go.sum`, pnpm lockfiles,
compose and the secrets manifest are integrator-merged only.

## Integrator money-path ruling on OPEN-13 (Claude Opus, 2026-10-05; condition (1) made fail-closed at freeze)
Accepted with two added conditions: (1) the attested bundle's buyer must equal the conversation's buyer — the server
derives both from its own records (the bundle's peer key in `inbox.bundle_peers` and the thread's peer key) and the
merchant never chooses whose live price applies. **Evaluated fail-closed:** when either peer key is unknown, or the
request has no `conversation_id`, or the caller lacks `live:manage`, no grant is written and the order is created at
**catalog price** with `live_price: "not_applied"` and the fallback text 「直播價不適用代建訂單，請改傳認領連結」 —
never the live price; when both peer keys are known and differ → `409 bundle_buyer_mismatch`, nothing created.
(2) Every grant writes an audit event (`claims.merchant_origin_granted`: principal, bundle, buyer, order, lines,
live-price minor per line). The grant is single-use and quote-bound (§5.3); one live for-buyer order per bundle
(§5.1). The 0105 consumption ledger cap stays the only ceiling; LPC01–06 stay green.

## 17. Review disposition (REVIEW.md, 2026-10-05)

P1-1 … P1-14: all applied (§2.2–2.6, §3.3–3.7, §4.1–4.3, §5, §10, §12, ruling above). P2 applied: 1 (§4.3), 2 (§3.3),
3 (§3.5), 4 (§3.5), 5 (§3.4 `body_hmac`), 6 (§2.2, implemented by batched id re-read in the worker instead of a new
consumer→live edge), 7 (§6), 8 (LC-U11, §3.1, R4), 9 (§3.6), 10 (§3.6), 11 (§3.3/§4.1), 12 (§2.5), 13 (§2.4), 14
(§8), 15 (§4.2), 17 (§3.7). P2 skipped:
- **P2-16** "exclude comments from the Page's other apps": no defined meaning — comments come from one Graph object
  read with the store's own Page token and the Page's own comments are already flagged `is_page`; the `limit` /
  `after_seq` validation half of P2-16 is applied (§2.6).

## Amendment 1 (2026-10-05, K3 round-2)

Source: K3 adversarial re-review `output/live-console-review-k3b/REVIEW.md` (branch `unit/k3-lcn-review`, commit
`d231cc23`; FREEZE_AFTER_FIXES, 0 P0 / 4 P1 / 10 P2). Append-only: the frozen text above is not edited; where a clause
below conflicts with it, **this amendment wins** for the clause it names, and every other clause stays frozen.
Integrator-delegated (Opus); evidence label DESIGN; every added gate case NOT_RUN. Migration numbers are assigned by
the integrator at merge (real numbers already differ from the §16 placeholders: LC-B3 = 0119, LC-B5 = 0121, LC-B2 = 0123; 0122 reserved for LC-B1).

### A1.1 P1-1 — `link_pending_manual` remediation (amends §4.2 bullet 2, §11 A4/A8/A13, §14.1 clause 2)

1. **Copy.** The console text for `link_pending_manual = true` is replaced by 「此買家的認領連結無法送出：這則留言的私訊
   額度已用。買家回覆私訊後 24 小時內可用一般私訊補發認領連結」. A manual private reply does not open a 24 h window (L5),
   so A12 to the buyer's thread keeps returning `409 window_closed` until the buyer writes; the UI never offers a send
   control for the flagged bundle before that. No automatic send of any kind is planned for a flagged bundle. When an
   inbound message opens the window the merchant sends the link by A12 (DMs may carry links, §12); the flag clears when
   a claim link is issued for the bundle (unchanged).
2. **Exposure.** A13 gains `link_pending_manual: bool`; A8 lists flagged bundles as bundle-only items (A1.5 P2-2).
3. **Planner gate.** `inbox.plan_manual_private_reply` refuses with `409 auto_pending_confirm` when all hold: frozen
   `comment_created_at > clock_timestamp() − 120 s`, the session's claim window is OPEN, and the comment's source has
   `private_reply = true` (the intake row may not be staged yet, so the §4.2 `FOR SHARE` yield cannot fire) — unless the
   request carries `confirm_preempt_auto: true`. A4 body gains that optional key (strict decoder: only `true` or
   absent). The UI asks 「系統可能正要自動傳送認領連結。手動私訊會用掉這則留言唯一一次私訊機會，確定繼續？」 and resends
   with the flag. The confirmed send is audited `inbox.private_reply.preempt_confirmed` (principal, comment_ref).
4. LCN07 adds: inside 120 s without the flag → `auto_pending_confirm`; with it → planned; the full race (manual wins,
   claim ACCEPTED) leaves a bundle whose A13 shows `link_pending_manual`, A12 → `window_closed` before any inbound
   message, a DM with the link succeeds after one, and the flag then clears.

### A1.2 P1-2 — comment facts for IG and the IG webhook fallback (amends §2.3 `comment-facts`, §2.5, §3.3 row 2)

1. **Lookup order** (the API drives it; the bridge never decrypts):
   a. bridge `comment-facts` — ring buffer, else one Graph read inside the shared rate budget, **platform-branched**:
      FB `GET /{comment_id}?fields=created_time,from{id},parent{id}`; IG `GET /{ig_comment_id}?fields=timestamp,from{id},parent_id`
      (exact IG field set recorded at probe R3 as new **LC-U12**; until then the IG Graph branch is MOCK). `is_page` =
      author id equals the source `asset_id`; `is_reply` = `parent{id}`/`parent_id` present; `created_at` =
      `created_time` (FB) / `timestamp` (IG). The bridge answers `{found:false}` when neither buffer nor Graph has it.
   b. only for an IG source served by the OPEN-4 webhook fallback, and only after (a) answered `found:false`: the API
      reads the webhook copy through **`social.read_comment_facts(p_session uuid, p_comment_ref text)`** (STABLE
      SECURITY DEFINER, owner `commerce_meta_writer`, EXECUTE **`commerce_runtime`**, `M` transaction + `live:read`;
      returns the newest `social.comment_events` envelope + AAD columns for that comment of the session's IG source,
      zero rows when none), decrypts with the payload keyring it already holds, and derives the same three facts
      (`created_at` = the copy's `occurred_at`, which for IG is delivery time — known limit U7, unchanged). The
      author id stays in API memory. This is deliberately **not** run in claims-worker: that process does not hold the
      Meta payload keyring and must not gain it (custody, `secrets.manifest.tsv`).
   c. neither source → **manual private reply is disabled for that comment**: A4 answers `409 comment_facts_unavailable`;
      `live.console_marks` / the comment marks return `private_reply_available: false`,
      `private_reply_unavailable_reason: "facts_unavailable"`, and the UI shows 「無法確認這則留言的時間與作者，暫時不能私訊；
      可公開回覆或等買家私訊」. No guess, no default timestamp.
2. The §3.3 row-2 deny-code list and §2.5 reason list gain `comment_facts_unavailable` / `facts_unavailable`
   (the frozen `comment_unknown` stays for "not a comment of this session's source").
3. LCN02/LCN07 add IG cases: Graph branch found/page/reply; fallback copy found/page/reply; neither → disabled with the
   reason; the IG author id never appears in a response, log or table.

### A1.3 P1-3 — `duplicate_recent` and the recommend rate are DB-serialised (amends §3.3 rate bullet, §4.1 recommend bullet, §5.1 step 4)

1. Inside the planning transaction, **before** reading for duplicates and before any insert, the planner takes
   `pg_advisory_xact_lock(hashtextextended('lcn-dup|' ‖ store_id ‖ '|' ‖ target ‖ '|' ‖ encode(body_hmac,'hex'), 0))`
   where target = `conversation_id` for A12 and the §5.1 step-4 DM, `comment_ref` for A4/A5. It then checks for an
   operation of the same action, target and `body_hmac` created in the last 30 s (any state) and refuses `409
   duplicate_recent`. The second concurrent planner blocks on the lock until the first commits, then sees its row.
2. The recommend rule takes `pg_advisory_xact_lock(hashtextextended('lcn-rec|' ‖ store_id ‖ '|' ‖ offer_id, 0))`
   before checking for a `meta.offer_recommend` of that offer in the last 10 min.
3. Lock order: these advisory locks come **first** in the producer, before `conversation_state` `FOR UPDATE`, the
   intake `FOR SHARE` (§4.2) and the binding `FOR SHARE`; no other path takes an `lcn-dup|`/`lcn-rec|` key.
4. The 60 sends/min store cap stays approximate (no lock).
5. LCN06/07/08 add: two concurrent same-body requests with different Idempotency-Keys → exactly one operation and
   one `409 duplicate_recent` (A12, A4, A5 and the for-buyer DM); two concurrent recommends → one operation.

### A1.4 P1-4 — bounded redaction of UNKNOWN operations (amends §10 row C4, §14.7; amends claims-retention-purge-v1 §1 C4 and §0 "Rejected: redacting non-terminal / UNKNOWN")

1. **Actions covered:** `meta.private_reply` (auto, manual, `:m1`), `meta.dm_send`, `meta.public_reply`,
   `meta.offer_recommend`.
2. **Eligible** (`claims.run_retention` step C4, same `p_limit`, oldest first, `FOR UPDATE SKIP LOCKED`):
   `created_at < now() − intake_days` **and** `request` still holds an id to redact **and** either
   `state ∈ (SUCCEEDED, FAILED_FINAL, CANCELLED, BLOCKED_POLICY, STALE_BINDING)` or
   `state = 'UNKNOWN' AND (lease_until IS NULL OR lease_until < now())`. `intake_days` ≥ 8 (RD5) is past Meta's 7-day
   private-reply window and the 24 h DM window, so no remote action can still be attributed to the ids. A leased
   (reconciling) row is skipped and picked up by a later run; the worker's `complete_operation` locks the row
   `FOR UPDATE`, so the two never interleave.
3. **Why the 2026-09-30 rejection no longer holds for v1:** every covered action's Reconcile is query-only and returns
   UNKNOWN without reading `comment_ref`/`conversation_id` (MCI U4 unresolved; §4.3), so the ids have no use — the same
   argument as the §3.4 secret wipe. **Revisit trigger:** a reviewed reconciler that reads these ids for an action
   removes that action's UNKNOWN branch here until its reconcile budget is exhausted.
4. **Action:** `request := (request − 'comment_ref' − 'conversation_id' − 'peer_key') || '{"redacted":true}'`;
   `semantic_key := <prefix>-purged: ‖ id` with prefix `mpr` (incl. `:m1`), `mdm`, `mpub`, `mrec` (the `mpub:`/`mdm:`
   keys are deterministic in the merchant-chosen Idempotency-Key, which is not entropy); `request_hash` kept; state
   unchanged (an UNKNOWN stays UNKNOWN, flagged `redacted`). An adapter that reconciles a redacted request returns
   UNKNOWN with zero HTTP calls.
5. **Privileges** (the retention unit's migration; CRP02 equality updated by the integrator): `commerce_retention_writer`
   column SELECT on `integration.operations` gains `lease_until`; policies `operation_retention_read/update` widen
   `action='meta.private_reply'` to the four actions and the state list to "terminal five OR (UNKNOWN AND lease
   expired)"; WITH CHECK becomes `NOT (request ?| array['comment_ref','conversation_id','peer_key']) AND semantic_key
   ~ '^(mpr|mdm|mpub|mrec)-purged:'`. The replay flag path is unchanged.
6. **Consumers:** `live.console_marks` (§2.5) and A9 tolerate renamed/redacted rows (an old comment simply shows no
   reply mark; an old outbound row whose operation was redacted keeps its `send_state`).
7. LCN13 / CRP gates add: an UNKNOWN operation of each action older than `intake_days` with no live lease → ids
   redacted and key renamed; a leased one is skipped; a younger one untouched; a redacted UNKNOWN reconciled → UNKNOWN,
   zero HTTP; replay mode unchanged.

### A1.5 P2 dispositions

Applied:
- **P2-1** `display_name`: `social.list_conversations` stays metadata-only (merged in 0119). New
  `social.conversation_heads(p_ids uuid[])` (≤ 50 ids; same owner/EXECUTE/permission as `social.read_thread`) returns
  the envelope + AAD columns of the newest inbound message per conversation; the API derives `display_name` from it
  (null when unreadable). LC-B4.
- **P2-2** A8 bundle-only items: `{bundle_id, conversation_id: null, session_id, link_pending_manual: true,
  unread: false, unreplied: true, last_at: bundle created_at, mode: null, assignee: null, window_open_until: null}`;
  the UI opens A13 and offers only 「複製認領連結」 through the reused `POST …/claims/bundles/{id}/link` — never a send
  button. LC-B4 (API), LC-U2 (UI).
- **P2-3** takeover-expiry generation: ruled **as already implemented in 0119** — `inbox.dm_window` returns the
  *effective* generation (stored + 1 once `human_until` has passed), and the lazy write stores exactly that value, so
  Check is deterministic. Planners of automated sends freeze the **effective** generation read through `dm_window` /
  `dm_window_for_bundle`; an automated op planned after expiry therefore passes, one planned before the takeover is
  denied `takeover_changed` (conservative by design). LC-B4.
- **P2-4** producer signatures (all SECURITY DEFINER, owner `commerce_integration_writer`, EXECUTE `commerce_runtime`,
  `M` transaction, return the operation id; Go creates the operation UUID and River job first, as `plan_claim_reply`;
  refusals are SQLSTATE `PT409` with the deny code as message, `PT422` for `invalid_text`). Common tail ‹E› =
  `p_operation uuid, p_job bigint, p_outbound uuid, p_body_hmac bytea, p_display_key_id text, p_display_nonce bytea,
  p_display_ciphertext bytea, p_secret_enc bytea, p_secret_sealed bytea, p_template_id uuid, p_template_version int`.
  - `inbox.plan_dm(p_conversation uuid, p_expected_generation bigint, p_order uuid, ‹E›)` — locks: `lcn-dup` →
    `conversation_state` FOR UPDATE → binding FOR SHARE. Deny: `window_closed, takeover_changed, capability,
    conversation_gone, duplicate_recent`.
  - `inbox.plan_manual_private_reply(p_session uuid, p_comment_ref text, p_comment_created_at timestamptz,
    p_confirm_preempt boolean, ‹E›)` — locks: `lcn-dup` → intake FOR SHARE → `conversation_state` FOR UPDATE (only
    when the peer is linked) → binding FOR SHARE. Deny: `used, auto_pending, auto_pending_confirm, expired_7d,
    ig_live_ended, capability, duplicate_recent`.
  - `inbox.plan_public_reply(p_session uuid, p_comment_ref text, ‹E›)` — locks: `lcn-dup` → binding FOR SHARE. Deny:
    `capability, ig_live_unsupported, duplicate_recent` (content is validated in Go before sealing; the definer
    re-checks length only).
  - `live.plan_offer_recommend(p_session uuid, p_offer uuid, p_expected_version bigint, ‹E›)` — locks: `lcn-rec` →
    offer FOR SHARE → binding FOR SHARE. Deny: `offer_unavailable, capability, duplicate_recent, version_conflict`.
  LC-B4.
- **P2-5** `inbox.bundle_peers` has `UNIQUE (tenant_id, store_id, bundle_id, peer_key)`; the Finish hook inserts
  `ON CONFLICT DO NOTHING`. `dm_window_for_bundle` gains `p_app_id text, p_object text, p_asset_id text` and matches
  only peers of the operation's own platform/asset. LC-B4.
- **P2-7** money leftovers: (a) A15 items gain `live_quantity_remaining` (claimed − units held in
  `claims.live_price_uses` by non-CANCELLED orders) and the drawer shows it per line; (b) A16 with
  `send_payment_link: true` from a principal without `inbox:reply` → `409 capability`, nothing created (checked
  before step 3a); (c) A13/A15 by `conversation_id` with no `inbox.bundle_peers` row → `200` with `items: []`,
  `live_price_eligible: false`, `live_price_reason: "bundle_buyer_unverified"` (not 404), so the UI offers the bundle
  path. LC-B6 (A15/A16), LC-B4 (A13).
- **P2-8** LC-B6 additionally depends on LC-B5 (template `order-pay-link/v1`; merged as 0121).
- **P2-9** §9 note: re-`start` of an ended session can open a claim window with no IG broadcast running; the IG-live
  "window OPEN" proxy then passes locally and Meta refuses (UNKNOWN until LC-U9). Accepted as a known limit; probe S2
  records Meta's answer to an IG private reply after the broadcast ended as LC-U9 evidence. No code change.
- **P2-10** §0.2 annotation: "`subscribed_fields=feed` only" and deviation A18 describe base `80b79487`; **fixed by
  LC-B3** (`internal/metaconnect/graph.go` now subscribes `feed,messages`).

Skipped:
- **P2-6** (trigger must upsert): already satisfied — `inbox.advance_conversation_state()` in 0119 inserts
  `ON CONFLICT (tenant_id, store_id, conversation_id) DO NOTHING` and then updates; no change needed.

### A1.6 Unit deltas

- **LC-B2 (implemented in 0123, MOCK; integrator-completed 2026-10-05)** adds: the platform-branched IG Graph read in bridge `comment-facts` (A1.2 1a) with
  `is_page`/`is_reply`/`created_at` derivation and `{found:false}` when neither buffer nor Graph has the comment; the
  definer `social.read_comment_facts` (A1.2 1b, in 0123 beside `social.read_comment_events`) and the API
  fallback path that calls it; reason `facts_unavailable` in `live.console_marks`; LCN02/LCN07 IG facts cases.
- **LC-B4**: A1.1 (120 s confirm gate, `confirm_preempt_auto`, audit, `link_pending_manual` in A13 and A8), A1.2's
  `comment_facts_unavailable` refusal in A4, A1.3 advisory locks in all four producers, A1.4.4 adapters return UNKNOWN
  with zero HTTP on a redacted request, P2-1, P2-2, P2-3, P2-4, P2-5, P2-7(c) for A13; gates LCN06/07/08/10 additions.
- **LC-B6**: A1.3 lock for the step-4 DM (target = conversation), P2-7 (a)(b)(c) for A15/A16, P2-8 dependency on LC-B5;
  LCN12 additions.
- **Retention unit (LC-R1, follow-up to U08 retention-core; DeepSeek)**: A1.4 — `claims.run_retention` C4 widening,
  `apply_actor_erasure` C4 redaction covering the four actions, privilege/policy changes, CRP gate cases; amends
  `claims-retention-purge-v1` §1 C4 and its §0 rejection line by reference to this amendment. Write paths: one new
  migration (integrator-numbered), `internal/retention/**`, `tests/foundation/claims_retention_*_test.go`.
- **LC-U2 (UI)**: the A1.1 copy and confirm dialog, the A1.2 disabled-reason text, bundle-only A8 items (copy-link only).
- LC-B1, LC-B3, LC-B5, LC-B7: no change (P2-9 and P2-10 are text-only; P2-6 already satisfied in 0119).

## Amendment W3-03B checkout reminders (2026-10-06; migration 0144; revised after the independent review)

Owner ruling: a reminder is sent **only within 24 h of the buyer's last inbound message** (`messaging_type=RESPONSE`). Meta's fixed-format / utility
policy is unverified (EVIDENCE_GAP; message tags were removed 2026-02-09), so there is no tag, no UPDATE type, no utility template, no send outside the window.

- **Action.** `meta.dm_send`, purpose `service`, request `origin=auto`, `message_type=checkout_reminder`, `reminder_state`. Same ledger, same `inbox.check_send` (window +
  takeover + capability re-checked at dispatch: BLOCKED_POLICY `window_closed` / `human_takeover`, never retried; UNKNOWN never re-sent), same sealed dispatch copy and Finish
  hook (§3.4, §4.3). Check also re-reads the bundle state (`inbox.crm_bundle_facts`): paid / cancelled / expired / ordered since planning -> BLOCKED_POLICY `not_remindable`,
  zero HTTP. §4.4 `send_state` is reused unchanged. An auto send never takes a conversation over (§3.6); a conversation in human mode is skipped.
- **Candidates** (per session; facebook/instagram bundles with claim lines; a bundle is a candidate only when the session's claim window is CLOSED or its claim is ≥ 10 min old):
  `awaiting_payment` = an unexpired `AWAITING_PAYMENT` order of the bundle; `claimed` = no order at all. Paid, cancelled, expired-unpaid, fully-ordered, purged and line-less
  buyers are never reminded (the planner refuses `line_count = 0`).
- **The link completes the purchase; no new bearer type.** Per buyer, in the SAME merchant transaction as the plan (a refusal rolls the issue back):
  `claimed` and never opened (bundle has no owner) -> a re-issued CLAIM link (`claims.issue_link`, CAS on the scanned generation, old token dies, `origin/<locale>/claim#t=…`,
  template `checkout-reminder/v1`); `awaiting_payment` of a `merchant_manual` order -> a re-issued ORDER link (`fulfillment.regenerate_order_link`, the RegenerateLink rules,
  `origin/<locale>/order-link#o=…&t=…`, template `order-pay-link/v1`). Buyers whose link cannot be re-issued — an unpaid storefront-source order, or a claim already opened (its
  link only works in the owner's own browser) — are `link_unavailable` follow-ups. The bearer exists only in memory and the sealed dispatch copy; the display copy keeps `{{連結}}`.
- **Once per buyer per session.** Semantic key `crm:` + hex(sha256(session | owner_id-or-bundle_id))[:48]; unique among queued reminders and the operation's ledger key. A second
  trigger reports `already_reminded` (no second link is issued). A follow-up row is not a reminder: it becomes the queued one when the buyer becomes reachable.
- **Not reachable -> follow-up list**, `reason` in `window_closed | human_takeover | no_peer | capability | link_unavailable`. `no_peer`: the bundle has no `inbox.bundle_peers`
  link (no private reply reached the buyer); no private-reply quota is spent. The report's `link` is the store's non-bearer checkout URL for the merchant to copy.
- **Locks / cap (A1.3).** The planner takes the `lcn-dup|store|conversation|body_hmac` advisory lock first (hmac of template + bundle, never of the link), then the per-session `crm|` lock;
  reminders count toward the same 60 sends / store / minute as human sends (`rate_limited`, a per-buyer refusal).
- **Routes** (`/v1/admin/stores/{store_id}`): `POST /live-sessions/{sid}/reminders[/{bundle_id}]` (`inbox:reply` AND `live:manage`, Idempotency-Key; with a bundle id only that buyer,
  409 `not_remindable` when she is no candidate; 409 `no_storefront` without an active domain) -> `{queued, already_reminded, followup, restricted, refused, truncated, results:[{bundle_id, outcome:
  queued|followup|restricted|refused, code}]}` (`restricted`: W3-05B, a buyer on the store's blocklist is skipped with no link and no DM; follow-up reason `restricted`, see live-keyword-claims-v1 Amendment W3-05B). One scan transaction (follow-up rows + one audit row `inbox.checkout_reminder.triggered`, also when nothing is sent), then ONE TRANSACTION PER BUYER:
  a refusal (window closed since the scan, takeover, rate cap, state changed, link generation moved) refuses only that buyer. At most 100 SENDABLE buyers per call (follow-up and
  reminded buyers cost nothing, so a later call always reaches the rest). `GET /live-sessions/{sid}/reminders` (`inbox:read`) -> `{sent, queued, failed, followup:[{bundle_id,
  display_name, reminder_state, reason, link_copy_allowed}], link}`.
- **Templates are not localised** (msgtemplates has no locale): the text is zh-TW for every `reply_locale`; only the link's locale path follows the claim source. A per-locale
  template set is a W3-U2 / template follow-up.
- **DEFERRED: automatic trigger and settings** (integrator ruling 2026-10-06). The PSID and the display copy need the inbox payload ring, which stays API-only: claims-worker
  does not get it and there is no second sealing path. v1 = merchant-triggered only (single buyer or batch). No River kind `checkout_reminder_v1`, and the settings routes
  (`GET|PUT /live-settings/reminder`) are removed until an automatic path exists; the empty table `live.reminder_settings` is kept for it (no definer, no grant to a login).

## Amendment "LC-B3b" inbox read gaps (2026-10-07; migration 0165; integrator rulings in `docs/delivery/units/lc-b3b-buyer-panel.md`)

All changes are additive response fields or filters; no permission, route or error code changes; out-of-scope tenant/store stays 404 (LCN03); responses stay `private, no-store`.

- **Identity link (I09).** The only link from a conversation to bundles is `inbox.bundle_peers` (§3.7, written when a private reply succeeds) plus the explicit A14
  `linked_customer_id`. `claims.bundles.owner_id`, display names and any name matching never feed a panel field, a suggestion or a prefill.
- **A13** (`inbox.buyer_panel`). `?conversation_id=`: the bundles are the store's non-purged bundles whose `bundle_peers` row equals the conversation's
  (app, object, asset, peer); a conversation without links answers 200 with empty claims/orders (P2-7c). `?bundle_id=`: that bundle only (404 when purged or out of scope).
  - `claims` = accepted claim lines `{session_id, offer_id, keyword, quantity}`, newest session first (then keyword, offer_id), at most 50.
  - `claim_total_minor` = Σ quantity × the unit price the claim RECORDED (`claims.live_price_uses.unit_price_minor` of the newest use of that bundle/offer; 0 for a line
    never ordered at a live price), over the full set (not only the 50 shown). Never recomputed from the current catalogue.
  - `orders` is present only when the principal holds `orders:read` (decided inside the definer; the key is omitted entirely otherwise): orders created from those bundles
    (claim checkout `claims.order_origins` ∪ `claims.live_price_uses` ∪ A16 `inbox.order_for_buyer`, deduplicated by order_id), newest first, at most 20, `number` = `LC-` + the upper-case hex of the order id. Price-neutral checkouts are included through `order_origins`.
  - `purchase_ordinal` = count (full set, not gated by `orders:read`) of those orders in `CONFIRMED` or `AWAITING_COLLECTION`; CANCELLED and unpaid states never count. The UI shows 第 N 次購買.
  - `display_name` only from the conversation's own newest inbound envelope (as A8); never from a bundle or an order. `auto_reply: {send_state}` is the §4.4 state of the
    newest automated (`origin_kind=auto`) private-reply operation of those bundles (a manual send never counts); omitted when none.
- **A8.** `session_id` keeps conversations whose peer is `bundle_peers`-linked to a non-purged bundle of that session (also narrows the link-pending bundle rows in SQL before their bound).
  `filter=live_comment` returns the bundle-only rows (`bundle_id`, `session_id`, `conversation_id: null`, platform facebook rendered as `messenger`, real `link_pending_manual`) of
  the store's non-purged facebook/instagram bundles, at most `limit`, ordered by `(created_at DESC, bundle_id DESC)`. The LC-B3b review amendment (2026-10-08) adds keyset pagination: `next_cursor` encodes the last returned bundle's stored timestamp and ID using the existing opaque A8 cursor format. Follow it with the same filter and session; a full final page may lead to an empty terminal page. Tenant/store/session and keyset predicates apply before LIMIT. Other filters retain their conversation cursor and first-page pending-link append. Every item gains `link_version` (`inbox.conversation_state.version`; explicit `null` on bundle-only rows).
- **A9.** The header gains `link_version` (the value A14 takes as `expected_version`) and `binding_id` (the enabled binding of the conversation's provider and asset, same rule as `plan_dm`; `null` when none).
