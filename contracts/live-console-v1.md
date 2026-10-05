# Live console v1 — comment read-through, Messenger/IG inbox, manual replies, order for a buyer (W2)

Status: **DRAFT 2026-10-05 (DESIGN; every gate NOT_RUN)** — contract author: integrator (Opus), unit W2-00C of
`output/arch-conformance/IMPLEMENTATION-PLAN.md`. Next: one K3 adversarial review, integrator answers to the `OPEN-n`
items (§15), then FROZEN under PROCESS.md §2.2. No implementation unit may start from this file before FROZEN.
Evidence label of this file: DESIGN. Nothing here authorizes a LIVE send outside the §13.3 probe plan.

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
A5-3 live-video picker), A7, claim-direct-checkout (cart merge). Migration numbers below are **placeholders P0..P6
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
Graph error codes for "window closed", "already replied", "no MODERATE task" (= MCI U3).

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
  generation bigint, lease_token_hash bytea, lease_until timestamptz, demand_until timestamptz)` (P2, FORCE RLS,
  writer `commerce_integration_writer` through definers only). Lease 30 s, renewed every 10 s; a second worker replica
  finds the lease held and does not poll (I23: at most one Graph poller per source fleet-wide).
- Token: new loader `integration.load_meta_page_token_for_poll(p_source uuid, p_generation bigint, p_lease_token
  bytea)` (owner `commerce_integration_writer`, EXECUTE `commerce_claims_worker`), fenced like
  `load_meta_page_token` but on the poll lease; returns the current head for the source's binding, zero rows when the
  binding is disabled/re-pointed (→ poller stops with `stream_state=unavailable`). The token never leaves the poller
  goroutine; it is zeroed when the poller stops.
- A poller runs while `(window OPEN) OR demand_until > now()`. `demand_until` is extended to `now()+120 s` by the
  bridge (§2.3) whenever the console asks for the session's comments, at most once per 15 s per source.
- Ring buffer, process memory only (OPEN-6): per source ≤ 2000 comments, entry age ≤ 2 h, buffer dropped 10 min after
  the poller stops. Justification: N admin tabs on one session must cost one Graph poll, not N; a restart loses only
  what Graph can re-serve. Each entry gets a buffer-local `seq` (monotonic) under a random `epoch` chosen at buffer
  creation.

### 2.3 Bridge API ↔ claims-worker (OPEN-2)

claims-worker gains one internal listener `COMMERCE_CLAIMS_CONSOLE_ADDR` (backend Docker network only; never routed by
Caddy; smoke asserts it is not reachable from the edge network). Auth: `Authorization: Bearer <commerce_console_bridge_token>`
(new secret, 32 random bytes, mounted in `api` and `claims-worker`), constant-time compare. One route:

`POST /internal/v1/comment-page` body `{tenant_id, store_id, session_id, source_id, after: {epoch, seq} | null,
before_cursor: string | null, limit 1..100}` → `{epoch, items: [Comment], next_seq, older_cursor, stream: StreamState}`.

The worker re-checks `(tenant, store, session, source)` through `live.console_source(…)` (STABLE definer, EXECUTE
`commerce_claims_worker`) before touching a buffer; mismatch → 404. The API is the only caller and has already
authorized the merchant (`live:read`, store scope from the bearer, I01). Bodies are never logged; the bridge client
and server use the redacted error style of meta-inbox ("fixed safe errors").

Rejected: Page token in the API (custody clause 5); PG table or `NOTIFY` carrying comment text; a new broker; one
operation-ledger row per poll (reads have no side effect, I06 targets external actions; 2-s polling would add
~10 000 operations per 5.5-h live); Meta's streaming endpoint (L7, token in URL).

### 2.4 Graph polling, backfill, rate limits

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
- Older than the buffer: the API passes `before_cursor` (Graph's opaque paging cursor, wrapped by the worker in an HMAC
  under a per-process key so the browser cannot inject a foreign cursor); served directly from Graph, not buffered.
- IG (OPEN-4): if LC-U3 shows Graph cannot read live media comments, the console serves IG live comments from the
  webhook copy `social.comment_events` (decrypted in the API with the payload keyring through the §3.2 read definer,
  `family=comment`, same session's media id); webhook delivery happens only during the broadcast (F4).

`StreamState = {state: live|throttled|reauth_required|unavailable|not_started, poll_interval_ms, last_ok_at,
lag_ms (now − newest created_time), source_platform: facebook|instagram, video_embeddable: bool}`. IG →
`video_embeddable=false` (UI shows 「IG 直播無法嵌入，請在手機上觀看」).

### 2.5 Linking a comment to claims without storing text

The API joins the comment page with `live.console_marks(p_session uuid, p_refs text[])` (STABLE SECURITY DEFINER,
owner `commerce_claims_writer`, EXECUTE `commerce_runtime`, requires `live:read` via `identity.principal_holds`,
≤ 100 refs, every ref `^[0-9_]{1,80}$`). Join keys: `claims.meta_intake (object, asset_id, comment_ref)` →
`applied_event_id` → `claims.events` (status, reason, offer, quantity, bundle); `integration.operations` by
`semantic_key = 'mpr:'||…` (auto/manual private reply state) and by `request->>'comment_ref'` for public replies;
`live.comment_prints`. Result per ref (all nullable):

```
{ref, intake: {state, drop_reason} , claim: {status, reason, offer_id, keyword, quantity, bundle_id},
 private_reply: {kind: auto|manual|out_of_stock, state, blocked_reason}, public_replies: int,
 printed: {count, last_at}, private_reply_available: bool, private_reply_unavailable_reason}
```

`private_reply_unavailable_reason` ∈ `used`, `auto_pending`, `expired_7d`, `ig_live_ended`, `capability`,
`page_comment`.

### 2.6 Console HTTP and cadence

`GET /v1/admin/stores/{store_id}/live-sessions/{session_id}/comments?after_epoch=&after_seq=&limit=` and
`?before_cursor=`. Response `{epoch, reset: bool, items: [ConsoleComment], next: {epoch, seq}, older_cursor,
stream: StreamState}`; `reset=true` when `after_epoch` ≠ current epoch (worker restarted: the UI clears and re-reads).
`ConsoleComment = {ref, parent_ref, created_at, author_name (may be null, LC-U2), text, is_page: bool,
has_attachment: bool, marks}`. Headers `Cache-Control: no-store`, `Referrer-Policy: no-referrer` (I15).

Cadence (OPEN-1, deviation A17): **polling**, not SSE, in v1 — comments every 3 s while the tab is visible, console
read model (§7) every 5 s, inbox list every 10 s; all pause when `document.hidden`; exponential back-off on 5xx
(3 s → 30 s). SSE is an upgrade signal when measured API load or lag (> 5 s p95) says so.

## 3. Messenger / IG DM inbox

### 3.1 Subscription (fixes deviation A18)

- Connect (`metaconnect.subscribe`) subscribes `subscribed_fields=feed,messages`. App-level webhook config adds object
  `instagram` fields `messages` (owner/Kimi WebBridge, §13.3; LC-U4).
- Existing connections: a one-shot claims-worker job `meta_resubscribe_v1` (P3 table
  `integration.meta_resubscribe_jobs`, same lifecycle and audit pattern as `meta_unsubscribe_jobs` of migration 0100:
  PENDING → LEASED → SUCCEEDED | FAILED | UNKNOWN, sealed token copy wiped at terminal) enqueued once per active
  connection by the P3 migration's backfill function; result read back with `GET /{page}/subscribed_apps` and written
  to the capability table (§6).
- The consumer already projects `page_message` / `instagram_message` (meta-consumer MC02). No consumer change.

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
| DM RESPONSE | `POST /{page_id}/messages` (IG: `/{ig_id}/messages`) `messaging_type=RESPONSE`, `recipient.id`=PSID | `dm_session` capability ok; last inbound message `occurred_at + 24 h − 5 min > clock_timestamp()`; conversation not purged | `window_closed`, `capability`, `conversation_gone` |
| Manual private reply | `POST /{asset_id}/messages` `recipient.comment_id` | no `meta.private_reply` operation for this comment (any kind, any state); comment age `< 7 d − 1 h`; IG live: source session window OPEN and comment received `< 15 min`; not the Page's own comment; no intake of this comment in PENDING | `used`, `expired_7d`, `ig_live_ended`, `page_comment`, `auto_pending` |
| Public reply | `POST /{comment_id}/comments` (IG: `POST /{ig_comment_id}/replies`) | `reply_public` ok; text passes §3.5; IG live media: refused until LC-U8 | `capability`, `public_reply_forbidden_content`, `ig_live_unsupported` |
| Offer recommend comment | `POST /{live_object_id}/comments` (FB only) | `reply_public` ok; offer active, not sold out; fixed template | `capability`, `offer_unavailable` |

- **No message tag, no UPDATE type, ever.** Outside the 24 h window the send is refused before planning (`409
  window_closed`) and again at Check (BLOCKED_POLICY). The UI shows 「買家 24 小時內沒有傳訊息，現在不能私訊；可等他回覆或在留言公開回覆（不能放付款連結）」.
- A human send never falls back to another channel automatically (meta-claims-intake §6.1 rule kept).
- All four go through the ledger (§4): UNKNOWN is never blind-retried; Reconcile is query-only and returns UNKNOWN
  until a read proves delivery (MCI U4 precedent).

### 3.4 Outbound storage and PSID custody (OPEN-14)

- `inbox.outbound_messages(tenant_id, store_id, id, conversation_id NULL, comment_ref NULL, kind dm|private_reply|
  public_reply|recommend, operation_id, principal_id, template_id NULL, template_version NULL, key_id, nonce,
  ciphertext, body_sha256, created_at)` (P4, FORCE RLS). Ciphertext = the **display copy** sealed in the API with the
  payload keyring under AAD class `outbound` (`["livecommerce/meta-outbound/v1", tenant, store, id, kind, key_id]`); a
  payment/claim link inside it is replaced by the placeholder `{{付款連結}}` before sealing (no bearer link persists).
- `inbox.send_secrets(operation_id PK, tenant_id, store_id, sealed bytea, enc bytea)` (P4): the **dispatch copy** —
  `{recipient_psid?, text}` sealed by the API to the Page HPKE **public** ring (info
  `["livecommerce/meta-send/v1", tenant, store, operation_id]`). Opened only by claims-worker in `LoadSecret` (a new
  lease-fenced loader `inbox.load_send_secret(operation, generation, lease_token)`); wiped by the completion
  transaction on every terminal state and by retention C7 (§10) if an operation stays non-terminal past 8 d.
- The PSID is obtained in the API by opening the newest inbound message of the conversation (`sender.id`) and exists
  in clear only in API memory and inside the sealed secret. The operation request holds `conversation_id`, `peer_key`
  (already stored on the conversation) and `body_sha256`, never the PSID or text (external-operation "No …
  unredacted conversation").

### 3.5 Public reply content rule (server side, OPEN-12)

Rendered text (after template substitution) is rejected with `422 public_reply_forbidden_content` and a reason when it
contains: any URL or bare domain (`https?://`, `www.`, `[a-z0-9-]+\.(com|tw|hk|cn|net|org|shop|store|me|io|app|link|ly)`
case-insensitive, full-width dots normalized first), the store's own origin, the words 結帳/付款連結 followed by a link
placeholder, a phone-like run of ≥ 8 digits, an email address, or a template variable from the buyer class
(`{{order.*}}`, `{{buyer.*}}`, `{{link.*}}`). Length 1..300 runes after NFC. Templates usable for public replies are
flagged `public_safe=true` at publish (W2-05B) and still re-validated at send. The same rule applies to the recommend
comment (fixed template `offer-recommend/v1`: product name, variant, keyword, live price; no link).

### 3.6 Conversation state, read/unread, takeover (closes §10.3, replaces the hard-coded 0)

`inbox.conversation_state(tenant_id, store_id, conversation_id PK → social.conversations, mode text CHECK IN
('auto','human') DEFAULT 'auto', assignee_principal uuid NULL, takeover_generation bigint NOT NULL DEFAULT 0,
read_seq bigint NOT NULL DEFAULT 0, last_inbound_seq bigint, last_inbound_at timestamptz, last_outbound_at
timestamptz, status text CHECK IN ('open','done') DEFAULT 'open', customer_id uuid NULL, customer_link_principal
uuid NULL, version bigint, updated_at)` (P3). `last_inbound_*` is maintained by an AFTER INSERT trigger on
`social.messages` (owner `commerce_meta_writer`, creates the state row on first message).

- Unread = `last_inbound_seq > read_seq`; store-level, not per staff (OPEN-9). 未回覆 = `last_inbound_at >
  coalesce(last_outbound_at, '-infinity')`.
- Takeover (OPEN-8): the first manual send in a conversation in `auto` sets `mode='human'`, `assignee=sender`,
  `generation+1` in the planning transaction; explicit `POST …/takeover` / `…/release` (CAS on generation). Release
  sets `mode='auto'`, `assignee=NULL`, `generation+1`. A different staff member may send while another is assignee
  (shown in UI); reassignment = takeover with CAS.
- **Automated sends** (first private reply, later W3 reminders/out-of-stock) carry the conversation's generation at
  plan time when a conversation is known for the actor (§3.7), else 0. Check denies with `human_takeover` when the
  known conversation is now `mode='human'`, and `takeover_changed` when its generation differs from the frozen one.
- Human sends carry `expected_generation`; a stale value → `409 takeover_changed` before planning.

### 3.7 PSID ↔ claims ↔ customer link

- Comment actor ↔ conversation: when a private reply (auto or manual) SUCCEEDS, the Send API body carries
  `recipient_id` (L3; LC-U6). The claims-worker completion hook computes `peer_key` for `(app, object, asset,
  recipient_id)` and inserts `inbox.bundle_peers(tenant_id, store_id, bundle_id, peer_key, app_id, object, asset_id,
  operation_id, created_at)` (P4; FORCE RLS; written only through the Finish hook definer, the taiwan-cvs R-7a / ads
  F24 pattern). Join to `social.conversations` on `(app, object, asset, peer_key)`. If LC-U6 fails the link is never
  made and the buyer panel falls back to "same person?" unknown (no guessing from names).
- Conversation ↔ customer: (a) suggested when a linked bundle has `owner_id` (buyer) mapped to a customer
  (customers-billing projection); (b) explicit merchant link `POST …/inbox/conversations/{id}/customer-link`
  (`inbox:reply` + `customers:read`, CAS, audit `inbox.customer_linked|unlinked`). Never cross-tenant (I09); never
  automatic merge.

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
- Frozen request (≤ 2 KiB): `{v:1, kind, platform, asset_id, conversation_id?, peer_key?, comment_ref?, session_id?,
  offer_id?, outbound_id, body_sha256, template_id?, template_version?, policy:"lcn-policy/v1", takeover_generation,
  deadline_at, principal_id}`. `deadline_at` = DM: window end − 5 min; private reply: least(comment + 7 d − 1 h,
  IG live: received + 15 min); public/recommend: plan + 15 min (a stale public reply is worse than none).

### 4.2 Private-reply quota (one per comment, shared; plan risk 「私密回复额度冲突」)

Priority: **auto claim-link reply > out-of-stock auto reply (W3-04B) > manual**. Mechanism: all three use the same
`mpr:` key, so at most one exists. Manual planning is refused (`409 auto_pending`) while the comment's intake is
PENDING, or while it is APPLIED with an ACCEPTED event that may still plan an auto reply (source `private_reply=true`
and `claim_reply_plannable` would return OK). Amendment §14.1 clause 2: when `plan_claim_reply` meets an existing
`mpr:` operation it writes audit `claim_reply_skipped:reply_used` and commits the claim (no FAILED intake).

### 4.3 Adapter routes (claims-worker)

`(facebook|instagram, meta.dm_send|meta.private_reply|meta.public_reply|meta.offer_recommend, service)` use the §6.4
`LoadSecret`/`DispatchWithSecret` pair: `LoadSecret` loads the Page token (existing loader) **and** the send secret
(§3.4). Check (lock-free, `commerce_claims_worker` pool) re-reads: binding/capability, deadline, window (DM), takeover,
principal still holds `inbox:reply`, comment budget (private reply), outbound row exists. Outcome mapping as MCI §6.3:
2xx with id → SUCCEEDED (`provider_reference` = message/comment id); documented permanent 4xx (LC-U9 once probed) →
FAILED_FINAL with code; any other 4xx before LC-U9, 5xx, 429, timeout, transport, unparsable → UNKNOWN; Reconcile
query-only. A Finish hook writes `inbox.bundle_peers` (private replies) and `conversation_state.last_outbound_at`.

### 4.4 Visible send states

Thread items and comment marks expose `send_state` ∈ `queued` (READY/DISPATCHING), `sent` (SUCCEEDED), `failed`
(FAILED_FINAL + code), `blocked` (BLOCKED_POLICY/STALE_BINDING + code), `unknown` (UNKNOWN: 「不確定是否送達，請到
Messenger 確認，系統不會重送」). The auto first-reply state (M08 #4/#6) is exposed the same way.

## 5. Order for a buyer (幫他建立訂單)

### 5.1 Flow

1. `GET …/inbox/order-prefill?bundle_id=` or `?conversation_id=` → `{items: [{sku_id, offer_id, keyword, name,
   variant, quantity, live_price_minor, catalog_price_minor, sellable}], bundles: [bundle_id], customer: {name, phone,
   email} | null, last_delivery: {option_key, cvs: {store_code, store_name, store_address}} | {option_key,
   home_address} | null, suggested_option_key}` — items = the open claim lines of the actor's bundles (via §3.7 for a
   conversation; capped at 50); customer/last delivery from the actor's most recent order of this store (requires
   `orders:read`; omitted otherwise); `suggested_option_key` = 7-ELEVEN pickup when the last order used it.
2. UI edits quantities (± per line, 0 removes), picks delivery and payment from the **reused** `GET
   …/orders/manual/options`.
3. `POST …/orders/for-buyer` (new) — body exactly `{items, customer, delivery, payment_mode, locale, for:
   {bundle_ids: [uuid] (0..5), conversation_id: uuid | null}, send_payment_link: bool}`; `Idempotency-Key` required.
   Server: runs the **existing** `merchanttools.ManualOrders.Place` pipeline unchanged (SetCart → CreateQuote → Begin;
   the Quote is the only price, I05/I08; stock reserved by `begin_hold`, I03); then one merchant transaction records
   `inbox.order_for_buyer(order_id, conversation_id, bundle_ids, principal_id)` and audit `order.for_buyer_created`.
4. If `send_payment_link` and the conversation's 24 h window is open, the same request plans one `meta.dm_send` with
   template `order-pay-link/v1` (link sealed only in the send secret, §3.4). Window closed or no conversation → order
   is created, `send: {state: "not_sent", reason: "window_closed" | "no_conversation"}` and the UI offers copy-link.
5. Response = the `ManualResult` of `POST orders/manual` + `{send: {operation_id, state} | {state:"not_sent",
   reason}}`. Replay (same key) returns the same order and the same operation; `buyer_link` is null on replay (as
   today; `POST …/orders/manual/regenerate-link` is **reused** for a new link).

### 5.2 Reused vs new

| Endpoint | Status |
| --- | --- |
| `GET …/orders/manual/options` | reused unchanged |
| `POST …/orders/manual` | reused unchanged (still the plain manual order) |
| `POST …/orders/manual/regenerate-link` | reused unchanged |
| `GET …/live-sessions/{sid}/claims/bundles`, `POST …/claims/bundles/{id}/link` | reused unchanged (send the claim link instead of building an order) |
| `GET …/inbox/order-prefill` | **new** |
| `POST …/orders/for-buyer` | **new**, a thin wrapper over `ManualOrders.Place` + link record + optional DM |

### 5.3 Card and live price

- Card: per G3 ruling 2026-10-05, `payment_mode` may include PAYUNi card **when the manual-order options list it**
  (the change belongs to the G3/PAYUNi unit; this contract only passes the mode through).
- Live price (OPEN-13): the manual buyer is a fresh capability, so `claims.live_prices` returns nothing today and the
  order would be at catalog price. Recommended: a **merchant-attested claim origin** — `for.bundle_ids` lets the
  server set `CartInput.Origins` (server-only field) for lines whose offer has a live price, and `claims.live_prices`
  gains one branch: the cart's buyer capability was issued by this for-buyer request with an attestation row
  `claims.merchant_origin_grants(buyer_id, bundle_id, principal_id, expires_at = now()+15 min)` written before SetCart;
  the bundle is in the store, not purged, its session not archived; the 0105 consumption ledger still caps the live
  price at the claimed quantity across all orders. No link expiry requirement (the merchant attests). Money-path
  review by Claude is mandatory. Fallback if rejected: catalog price, and the drawer shows 「直播價不適用代建訂單，請改傳認領連結」.
- Draft orders are not introduced (deviation A9/D5): the order exists and holds stock as soon as it is created.

## 6. Capability states (§10.2), exposed to the UI

The table `integration.binding_capabilities` and its probe belong to W1-01B (`meta-connection-health-v1.md`). This
contract freezes the capability vocabulary the console consumes:

| Capability | Requires | Evidence that sets `ok` |
| --- | --- | --- |
| `read_comment` | `pages_read_engagement`, task `MODERATE` (L1); IG `instagram_manage_comments`; subscription `feed` | permissions + tasks read back; first successful console poll (`LIVE_READ`) |
| `private_reply` | `pages_messaging`, task `MESSAGING`; IG `instagram_manage_messages` (per connect clause 2) | permissions; first SUCCEEDED private reply (`LIVE_SEND`) |
| `dm_session` | `pages_messaging`, task `MESSAGING`; subscription `messages` (+ IG app-level `messages`) | `subscribed_apps` read back contains `messages`; first inbound DM projected (`LIVE_READ`) |
| `reply_public` | `pages_manage_engagement`, task `MODERATE` (L2); IG `instagram_manage_comments` | permissions; first SUCCEEDED public reply (`LIVE_SEND`) |

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
`expected_version` (CAS; conflict → UI re-reads). Requires `inventory:write` (OPEN-17). No auto-pause on sold out in
v1 (claims never touch stock, arch §11.2; the buyer sees sold-out at checkout, claim-direct-checkout B1).

### 7.3 Recommend (推薦) an offer

`POST …/live-sessions/{sid}/claims/offers/{offer_id}/recommend` `{expected_version, post_comment: bool}`
(`live:manage`; `post_comment` additionally `inbox:reply`). Writes the `offer featured` timeline event of W2-01B
(new table `live.offer_timeline` in P0, append-only `(session, offer, kind='featured', at, principal)`, used by
逐品成效), sets `recommended` in §7.1, and when `post_comment` plans
`meta.offer_recommend` (FB only; IG → `422 ig_live_unsupported`). Graph has no documented "pin a live comment" call;
"pin" in the UI means the console highlight plus this optional Page comment (OPEN-18).

### 7.4 Comment-label print record (列印)

`POST …/live-sessions/{sid}/comments/{comment_ref}/print` (`live:manage`, idempotent per key) → `live.comment_prints
(tenant_id, store_id, session_id, comment_ref, print_count, first_printed_at, last_printed_at, last_principal_id)`
(P2, FORCE RLS, upsert by definer). The label content (display name, keyword, quantity, time) is rendered by the
browser from the in-memory comment it already holds (W3-U3); the server stores only the fact of printing. Retention
class C3 (`intake_days`).

## 8. Several OPEN windows per store

Replace `live_claim_window_one_open (tenant_id, store_id)` with a per-store cap. Safe because, and only while:
1. every Meta comment maps to at most one session (`claim_sources UNIQUE(object, asset_id, source_object_id) WHERE
   active`, MCI03) and ingest already filters `session_id` (`ingest.go:150`);
2. the manual record path names the session in its URL (`…/live-sessions/{sid}/claims/manual`) and the handler uses
   that id, never "the current OPEN window" (W2-01B audits every caller of the removed index, grep gate);
3. bundles, lines, links and live-price consumption are per session/bundle (unchanged);
4. a cart merging claims of two sessions yields one order counted in both sessions (L1 ruling 3, `multi_session_orders`
   footnote) — totals across sessions are never summed in one figure;
5. a cap of **5 OPEN windows per store** (OPEN-15), enforced in `claims.SetWindow` (`internal/claims/merchant.go:151`) under a store-level advisory lock taken before the count
   (`409 too_many_open_windows`), bounding pollers (I23); billing BD5 still blocks opening any new window when
   RESTRICTED.

Gate LCN09: two sessions OPEN at once on two Pages; the same buyer comments the same keyword on both → two bundles,
two auto replies, two links; closing one does not affect the other; KC/MCI suites stay green.

## 9. Session lifecycle (W2-01B; amends studio-v1)

New column `live.sessions.lifecycle ∈ draft → live → ended → archived` (P0; separate from the media programme state,
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
| C5 (extended) | `inbox.outbound_messages` | `created_at < now() − social_days` and its operation terminal | DELETE |
| C5c | `inbox.conversation_state` | its `social.conversations` row deleted (C5b) | DELETE (FK cascade from the definer, not ON DELETE) |
| C7 | `inbox.send_secrets` | operation terminal (wiped at completion) or `created_at < now() − 8 d` | DELETE |
| C3 (extended) | `live.comment_prints`, `inbox.bundle_peers` | `created_at < now() − intake_days`; bundle_peers also on bundle de-identify (C2) and actor erasure (RD4) | DELETE |
| C4 (extended) | `integration.operations` actions `meta.dm_send/public_reply/offer_recommend` and manual `meta.private_reply` | terminal and `created_at < now() − intake_days` | redact `comment_ref`, `peer_key`, `conversation_id` as RD-C4 |

Comment text and names have no class: they are never stored (§2.1). `inbox.order_for_buyer` follows order retention.

## 11. API list

Base `/v1/admin/stores/{store_id}`. Every route: merchant bearer → server-resolved tenant/store (I01), strict JSON
decoder (unknown keys 400), `Cache-Control: no-store`, errors `{code}` from the fixed list. Writes need
`Idempotency-Key`. New permissions (P3, OPEN-7): `inbox:read`, `inbox:reply`.

| # | Method path | Permission | Request → Response | Errors | Audit |
| --- | --- | --- | --- | --- | --- |
| A1 | GET `/live-sessions/{sid}/console` | live:read | — → §7.1 | 404 not_found | — |
| A2 | GET `/live-sessions/{sid}/comments` | live:read | query §2.6 → §2.6 | 404, 409 no_source, 503 stream_unavailable | — |
| A3 | POST `/live-sessions/{sid}/comments/{ref}/print` | live:manage | `{}` → `{print_count, last_printed_at}` | 404, 422 invalid_ref | — (the row is the record) |
| A4 | POST `/live-sessions/{sid}/comments/{ref}/private-reply` | inbox:reply | `{text}` \| `{template_id, template_version}` → `{operation_id, outbound_id, send_state}` | 409 used\|auto_pending\|expired_7d\|ig_live_ended\|page_comment\|capability, 422 invalid_text | inbox.private_reply.planned |
| A5 | POST `/live-sessions/{sid}/comments/{ref}/public-reply` | inbox:reply | same as A4 | 409 capability\|ig_live_unsupported, 422 public_reply_forbidden_content | inbox.public_reply.planned |
| A6 | POST `/live-sessions/{sid}/claims/offers/{oid}/recommend` | live:manage (+inbox:reply if post_comment) | `{expected_version, post_comment}` → `{recommended_at, operation_id?}` | 409 version_conflict\|offer_unavailable\|capability, 422 ig_live_unsupported | live.offer.recommended |
| A7 | POST `/live-sessions/{sid}/lifecycle` | live:manage | `{action, expected_version, open_window?}` → `{lifecycle, version, window}` | 409 version_conflict\|invalid_transition\|too_many_open_windows, 402 billing_restricted | live.session.started\|ended\|archived |
| A8 | GET `/inbox/conversations` | inbox:read | `?filter=all\|unreplied\|messenger\|instagram\|live_comment&session_id=&cursor=&limit≤50` → `{items: [{conversation_id?, bundle_id?, platform, display_name?, last_at, unread, unreplied, mode, assignee, window_open_until, linked_customer_id}], next_cursor, unread_total}` | 400 invalid_filter | — |
| A9 | GET `/inbox/conversations/{cid}/messages` | inbox:read | `?before_seq=&limit≤50` → `{items: [{direction, seq?, at, text, attachments, kind?, send_state?, principal_id?, unreadable?}], window_open_until, mode, takeover_generation}` | 404 | — |
| A10 | POST `/inbox/conversations/{cid}/read` | inbox:read | `{read_seq}` → `{read_seq}` | 404, 422 | — |
| A11 | POST `/inbox/conversations/{cid}/takeover` · `/release` | inbox:reply | `{expected_generation}` → `{mode, assignee, takeover_generation}` | 409 takeover_changed | inbox.takeover\|inbox.release |
| A12 | POST `/inbox/conversations/{cid}/messages` | inbox:reply | `{text \| template ref, expected_generation}` → `{operation_id, outbound_id, send_state, takeover_generation}` | 409 window_closed\|takeover_changed\|capability\|conversation_gone, 422 invalid_text | inbox.dm.planned |
| A13 | GET `/inbox/buyer-panel` | inbox:read (+orders:read for orders) | `?conversation_id=` \| `?bundle_id=` → `{display_name?, platform, purchase_ordinal, claims: [{session_id, offer_id, keyword, quantity}], claim_total_minor, orders: [{order_id, number, state, total_minor, created_at}], auto_reply: {send_state}?, linked_customer_id?, window_open_until?}` | 404, 400 | — |
| A14 | POST `/inbox/conversations/{cid}/customer-link` | inbox:reply + customers:read | `{customer_id \| null, expected_version}` → `{customer_id, version}` | 409 version_conflict, 404 | inbox.customer_linked\|unlinked |
| A15 | GET `/inbox/order-prefill` | orders:read + inventory:reserve | `?bundle_id=` \| `?conversation_id=` → §5.1 step 1 | 404 | — |
| A16 | POST `/orders/for-buyer` | inventory:reserve (+inbox:reply if send) | §5.1 step 3 → §5.1 step 5 | the `orders/manual` codes + 409 capability | order.manual_created + order.for_buyer_created (+inbox.dm.planned) |
| — | `/message-templates` (W2-05B) | live:manage / inbox:reply | owned by W2-05B; this contract fixes only `{template_id, version, public_safe, kinds}` | | template.published |

Text inputs (A4/A5/A12): 1..2000 runes (public 300), NFC, no control chars except `\n`; template refs resolve only
published versions of the store. The internal bridge (§2.3) is not part of this list and never in the admin BFF
allowlist. BFF allowlist additions (UI units): A1–A16.

Role defaults (OPEN-7, OPEN-17): `live_operator` += `inbox:read, inbox:reply, inventory:write`; `viewer` excludes
`inbox:read` explicitly (DM text is buyer PII, not an analytics read); owner/admin receive both via a P3 backfill of
existing grants; fulfilment unchanged.

## 12. Privacy and security checklist

- Comment/DM text, names and PSIDs never in logs, River args, audit, metrics, PG (except the existing encrypted
  copies and §3.4 sealed copies), URLs, browser storage (I11); console responses `no-store` (I15).
- Bridge token and HPKE/payload keys from `*_FILE` secrets only; the bridge listener is not on the edge network.
- Cross-tenant/store reads return 404 for A1–A16 (gate LCN03); conversation ids are checked against the bearer's
  store inside the definer, never trusted from the path alone.
- Public replies cannot carry links/PII (§3.5); DMs may carry the payment link (inside the window only).
- No LLM, translation or automated free text; automated messages remain fixed templates.

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
| LCN01 | MOCK | Poller: one lease per source across two worker processes; lease loss stops polling; token loader refuses a stale generation; backoff ladder on usage headers, 429, codes 4/17/32/613; 190 → reauth; ring cap/age/idle drop; restart → new epoch → `reset:true` |
| LCN02 | MOCK | Bridge: wrong/missing token 401; edge network cannot reach it (compose smoke); tenant/store/session/source mismatch 404; bodies absent from logs |
| LCN03 | MOCK | Cross-tenant and cross-store A1–A16 → 404; `viewer` cannot A8/A9; `live_operator` can; permission backfill for owner/admin |
| LCN04 | MOCK | **Leak scan**: sentinel comment text, names, PSID, link token absent from every table, audit, River args, operation requests and logs after a full console session (I11) |
| LCN05 | MOCK | Marks: auto-claimed, dropped, rate-limited, unknown-keyword, manual-replied, public-replied, printed comments map correctly with no text join |
| LCN06 | MOCK | DM: inside window → one operation; window − 4 min → refused at plan; window closing between plan and Check → BLOCKED_POLICY zero HTTP; never a tag in any request body (fake Graph asserts) |
| LCN07 | MOCK | Private reply quota: manual after auto → `used`; manual while PENDING → `auto_pending`; manual first then auto apply → `claim_reply_skipped:reply_used`, claim ACCEPTED; 20 concurrent manual attempts → one operation |
| LCN08 | MOCK | Public reply: each §3.5 pattern (incl. full-width dots, store origin) rejected server-side; template `public_safe` re-validated; IG live refused |
| LCN09 | MOCK | Multi-window §8 cases; cap 5; manual record lands in the URL session only |
| LCN10 | MOCK | Takeover: implicit takeover on first send; automated send planned at gen 0 then human takeover → Check `human_takeover`; release → gen+1 → old automated op `takeover_changed`; stale `expected_generation` 409 |
| LCN11 | MOCK | UNKNOWN: 5xx/timeout/garbled → UNKNOWN, query-only, never a second POST (child-process kill after send); send secret wiped at every terminal state |
| LCN12 | MOCK | For-buyer: prefill from bundle and from conversation; quantities edited; Quote is the only price (request price ignored → 400 unknown key); stock reserved once under double-submit; DM planned only inside window; replay returns same order/op; live price per OPEN-13 outcome (LPC01–06 stay green) |
| LCN13 | MOCK | Retention §10 classes; send secrets older than 8 d removed; outbound display copy has no link |
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
4. **R4 DM in**: test user sends a Messenger DM and an IG DM → `social.messages` row, inbox shows text (LC-U4).
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
   `claim_reply_skipped:reply_used`, claim and intake commit (no operation, no link); any other 23505 stays an
   invariant breach. `claim_reply_plannable` gains skip code `reply_used`.
3. §6.2: request `takeover_generation` = the generation of the actor's known conversation (via `inbox.bundle_peers`)
   or 0; §6.3 Check gains deny codes `human_takeover`, `takeover_changed`. "Takeover is always 0 in v1" is removed.
4. Amendment "Merchant connect" clause 3: `subscribed_fields=feed` → `feed,messages`; existing connections are
   resubscribed by `meta_resubscribe_v1` (§3.1).
### 14.2 meta-consumer-v1
"No DM policy/window engine, public sending, … social read UI" known limit → superseded by live-console-v1 §3–4; the
consumer and its gates are unchanged.
### 14.3 meta-inbox-v1
Read authority note: the API process may open `social.messages` / `social.comment_events` envelopes only through
`social.read_thread` (and the IG fallback of §2.4) with `inbox:read`; no inbox (meta_private) body is ever read.
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
`manual_private_reply` on the default lane; CRP02/MCI02/KC03 equality lists gain the P0–P6 privilege rows of the
implementing units (integrator merges them).

## 15. Open questions (each with the recommended answer)

- **OPEN-1** Transport: polling (3 s comments, 5 s console, 10 s inbox) vs SSE now. → Polling in v1 (A17 ruling);
  SSE when p95 lag > 5 s or API load measured too high.
- **OPEN-2** API ↔ claims-worker bridge: internal HTTP with a shared token vs PG-mediated cache vs Page token in API.
  → Internal HTTP (§2.3), single listener, backend network only.
- **OPEN-3** FB poll object: live video id (A5-3 picker) vs feed post id. → Live video id when known, else post id;
  LC-U1 probe decides and the losing path is deleted.
- **OPEN-4** IG live comments: Graph read vs webhook copy. → Graph if LC-U3 passes; else decrypt
  `social.comment_events` (no new storage).
- **OPEN-5** Meta streaming endpoint. → Rejected (token in URL) unless LC-U5 proves header auth.
- **OPEN-6** Ring buffer bounds. → 2000 comments / 2 h / dropped 10 min after the poller stops.
- **OPEN-7** Permissions. → New `inbox:read`, `inbox:reply`; live_operator gets both; viewer explicitly excluded from
  `inbox:read`; owner/admin backfilled.
- **OPEN-8** Takeover trigger. → Implicit on the first manual send + explicit release (and explicit takeover button).
- **OPEN-9** Read state. → Store-level `read_seq`; per-staff later if multi-agent teams ask.
- **OPEN-10** `message_echoes` (replies typed in Meta Business Suite). → Not in v1; banner 「在 Meta 後台回覆的訊息不會顯示在這裡」.
- **OPEN-11** Private-reply quota priority. → Auto claim reply > out-of-stock > manual (§4.2).
- **OPEN-12** Public reply content. → No URLs/domains/phones/emails/buyer variables, ≤ 300 runes (§3.5).
- **OPEN-13** Live price on an order built for a buyer. → Merchant-attested claim origin with the 0105 ledger cap
  (§5.3), Claude money-path review; fallback catalog price + 「改傳認領連結」.
- **OPEN-14** PSID custody. → Never stored in clear; sealed per send to the HPKE public ring, wiped at terminal;
  comment↔conversation link via private-reply `recipient_id` (LC-U6).
- **OPEN-15** Multi-window cap. → 5 OPEN windows per store.
- **OPEN-16** Comments after `end`. → Shown and counted, never claimed (window closes on end).
- **OPEN-17** Live stock edit by live_operator. → Grant `inventory:write` to live_operator (SHOPLINE 小幫手 parity).
- **OPEN-18** Recommend = Page comment? → Optional per click, FB only, fixed template; no "pin" API.
- **OPEN-19** Outbound retention. → Same `social_days` class as inbound (30 d default).
- **OPEN-20** Per-read audit of decrypted DMs. → No per-read audit (noise); access is permission-gated and
  `no-store`; revisit if a DSAR/access-log requirement appears.

## 16. Implementation units (backend DeepSeek, UI Codex, tests K3)

Order respects 2 writing units / 4 agents; upstream interfaces frozen before downstream starts.

| Unit | Owner | Covers | Write paths | Migration (placeholder) | Gate | Depends |
| --- | --- | --- | --- | --- | --- | --- |
| LC-B1 = W2-01B lifecycle + multi-window | DeepSeek | §8, §9, §7.3 timeline event, A7 | `internal/live/lifecycle.go`, `internal/httpapi/live_lifecycle.go`, `internal/claims/merchant.go` (window cap only) | P0 (0120) | `--studio-backend`, `test-focused.sh 'LiveLifecycle'`, LCN09, G07 | A5, A7 merged |
| LC-B2 = W2-02B comment read-through | DeepSeek | §2, §7.4, A2, A3 | `internal/integrations/metareply/comment_poll.go`, `internal/integrations/metareply/bridge.go`, `internal/live/stream.go`, `internal/httpapi/live_stream.go`, `cmd/claims-worker/main.go` (wiring), `deploy/compose.yml` + `secrets.manifest.tsv` (bridge token, integrator-merged) | P1 (0121) | `--live-console` LCN01/02/04/05 | A5-3, W1-01B |
| LC-B3 = W2-03B inbox read + subscription | DeepSeek | §3.1, §3.2, §3.6 state, §3.7 customer link, A8–A11, A13, A14, permissions | `internal/inbox/**` (read side), `internal/httpapi/inbox.go`, `internal/metaconnect/graph.go` (fields), `internal/integrations/metareply/resubscribe.go` | P2 (0122) | `--inbox` LCN03, G07 | W1-01B |
| LC-B4 = W2-04B sends + takeover | DeepSeek | §3.3–3.6, §4, A4, A5, A6 comment, A12; §14.1 clauses 2–3 | `internal/inbox/send*.go`, `internal/integrations/metareply/{send_dm.go,public_reply.go,manual_reply.go}`, `internal/integrations/metareply/routes.go` (generation re-check only) | P3 (0123) | `--inbox-send` LCN06–08, 10, 11; Claude final review | LC-B3 |
| LC-B5 = W2-05B templates | DeepSeek | template ids/versions, `public_safe`, `order-pay-link/v1`, `offer-recommend/v1` | `internal/msgtemplates/**`, `internal/httpapi/templates.go` | P4 (0124) | `--msg-templates` | contract frozen (parallel with LC-B4) |
| LC-B6 = W2-06B order for buyer | DeepSeek | §5, A15, A16, OPEN-13 outcome | `internal/merchanttools/order_for_buyer.go`, `internal/httpapi/merchanttools.go` (routes only), live-price branch in `internal/claims` + SQL if OPEN-13 accepted | P5 (0125) | `--browser-manual-order` + LCN12, LPC01–06, G07; Claude money review | LC-B3, LC-B4 |
| LC-B7 console read model | DeepSeek | §7.1, §7.2 read side, A1 | `internal/live/console.go`, `internal/httpapi/live_console.go` | P6 (0126) only if a definer is needed | `--live-console` | A5-1, LC-B2 (stream stats) |
| LC-U1 = W2-U1 workspace shell | Codex | v5 nav, three phases, left column (FB embed, IG notice), status bar, offers on/off, live stock, recommend, 5 s polling | `apps/admin/src/features/live/**`, `apps/admin/components/{LiveWorkspace.tsx,LiveConsole.tsx,Studio.tsx,StudioClaims.tsx}`, `apps/admin/src/routes.ts` | — | `--browser-live-console`, `--browser-studio-ui`, `--browser-live-claims` | LC-B1, LC-B7 frozen; v5 |
| LC-U2 = W2-U2 stream + buyer panel + 訊息 | Codex | middle/right columns, filters 全部/關鍵字/私訊/待回覆, reply mode toggle, rule hints, takeover, inbox page | `apps/admin/components/{CommentStream.tsx,BuyerPanel.tsx,Inbox.tsx}`, `apps/admin/lib/inbox-*.ts`, BFF allowlist | — | `--browser-inbox`, `--browser-live-console` | LC-B2..B5 frozen, LC-U1 merged |
| LC-U3 = W2-U3 create-order drawer | Codex | §5 UI | `apps/admin/components/CreateOrderDrawer.tsx`, `ManualOrder.tsx` (extract shared form only) | — | `--browser-manual-order` | LC-B6, LC-U2 |
| LC-U4 = W3-U3 label print | Codex | §7.4 UI, print CSS | `apps/admin/components/CommentLabelPrint.tsx`, `apps/admin/app/print.css` | — | `--browser-live-console` (print DOM) | LC-U2 |
| LC-T1 = W2-T1 independent acceptance | K3 | LCN03/04/06/07/10/14 adversarial + browser | `tests/admin/live-console*.spec.ts`, `tests/foundation/live_console_gate_test.go` | — | red run then green | before LC-U3 merge |
| LC-X1 LIVE probe | Claude (Sonnet) + Kimi WebBridge | §13.3 | `output/live-console-probe/` | — | LCN15/16 | LC-B2..B4 merged, owner preconditions |

Every writing unit uses its own worktree/branch, the AGENT-PREAMBLE lifecycle (red → green, header ratchet), and
writes `output/<unit>/DELIVERY.md`. Migration numbers, OpenAPI, shared JSON schema, `go.mod/go.sum`, pnpm lockfiles,
compose and the secrets manifest are integrator-merged only.
