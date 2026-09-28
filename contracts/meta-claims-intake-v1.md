# Meta claims intake v1 — T10c comment → keyword claim → first private reply

Status: **DRAFT** (2026-09-28; contract designer, unit `design-meta-intake`). Not reviewed,
not frozen. No implementation, migration or gate may start from this file until the integrator
freezes it after one adversarial review (PROCESS.md §2.2). Evidence label of this file: DESIGN.
Every gate in §12 is NOT_RUN.

Amends, without editing them: `live-keyword-claims-v1.md` (FROZEN; §11.2 H1–H7 are the entry
conditions this file closes), `meta-consumer-v1.md` (FROZEN/PASS MC01–07; this file is the
"MC01 amendment" H4 asks for), `external-operation-v1.md` + `external-dispatcher-v1.md`
(accepted; this file adds one producer bridge and one adapter route). `meta-inbox-v1.md`,
`meta-webhook-protocol-v1.md`, `meta-runtime-v1.md` stay unchanged. `stripe-psp-v1.md` §0.2 is
used only as the pattern for per-store credential custody and reciprocal row/job atomicity (§7,
§5.3); nothing here touches Stripe objects or migrations 0061–0063 / post-River 0012–0013.

Migration numbers: **0064** (`0064_meta_claims_intake.sql`), post-River **0014**
(`post_river/0014_meta_claims_intake_queue.sql`). Integrator-owned; renumbered at merge if needed.

## 0. Facts (retrieved 2026-09-28 from developers.facebook.com)

| # | Fact | Source |
| --- | --- | --- |
| F1 | Page private reply: `POST /{PAGE-ID}/messages`, body `recipient.comment_id`. Requires `pages_messaging` and a Page access token of a person with the `MESSAGING` task. "Only one message can be sent to the person who commented." Must be sent "within 7 days from when the post or comment was created." Further messages only after the person responds (24 h window). "Cannot send private reply message to another facebook page." Standard Access apps only reach people with a role on the app. | <https://developers.facebook.com/docs/messenger-platform/discovery/private-replies/> |
| F2 | IG private reply: `POST /<IG_ID>/messages`, body `recipient.comment_id` + `message.text`. Facebook Login host `graph.facebook.com` (Instagram Login host `graph.instagram.com`). Permissions listed: FB Login `instagram_manage_comments`, `pages_read_engagement`; IG Login `instagram_business_manage_comments`. "within 7 days of the comment"; Live: "private replies can only be sent during the live broadcast"; "Only one message can be sent to the commenter". The page does **not** mention `instagram_manage_messages`. | <https://developers.facebook.com/docs/instagram-platform/private-replies/> |
| F3 | Page `feed` webhook, `item=comment`: value fields `from{id,name}`, `post_id`, `comment_id`, `parent_id`, `message`, `created_time`, `verb`, `post{status_type,…}`. Verb list includes `add`, `edit`, `edited`, `remove`, … . Feed webhooks require `pages_manage_metadata` and `pages_show_list`; the Page must install the app via `POST /{page-id}/subscribed_apps`. | <https://developers.facebook.com/docs/graph-api/webhooks/reference/page/>, <https://developers.facebook.com/docs/graph-api/webhooks/getting-started/webhooks-for-pages/> |
| F4 | IG `comments` / `live_comments` value: `id`, `text`, `from{id,username,self_ig_scoped_id}`, `media{id,media_product_type}`, `parent_id`, `ad_id`, `ad_title`, `original_media_id`. FB-Login permissions: `instagram_basic`, `instagram_manage_comments`, `pages_manage_metadata`, `pages_read_engagement`, `pages_show_list`. "Advanced Access is required to receive `comments` and `live_comments` webhook notifications." Account must be public. "Notifications for Comments on Live media are only sent during the live broadcast." | <https://developers.facebook.com/docs/graph-api/webhooks/reference/instagram/>, <https://developers.facebook.com/docs/instagram-platform/webhooks/> |

UNKNOWN (not retrievable or not stated; never assumed): U1 whether Facebook **live video**
comments arrive through the Page `feed` webhook and which `post_id` form they carry (live-video
comments guide fetch timed out); U2 whether IG private replies additionally need
`instagram_manage_messages` (F2 does not list it; the brief expected it); U3 the Graph error
code for "already replied / window closed"; U4 whether any read field (e.g. comment
`can_reply_privately`) proves a private reply was already sent; U5 current Graph API version to
pin; U6 whether Graph accepts the Page token in an `Authorization: Bearer` header. Each has a
LIVE probe in §12; until closed, the affected branch fails closed.

Observation for T07 (not changed here): F3 lists verb `edited`, while `normalize.go` accepts
only `add|edit|remove`; an `edited` delivery is quarantined. Harmless for claims (only `add`
qualifies) but a T07 fixture should record it.

## 1. Scope

In: FB Page feed comment `add` and IG `comments`/`live_comments` on an object bound to a live
session → text-free staged intake → `claims` ingest (source_kind `meta`) → optionally one
automated private reply carrying the claim link, through the external-operation ledger.

Out (unchanged NOT_RUN elsewhere): OAuth / Facebook Login issuer and token refresh (T07), social
read UI, human takeover UI, DM conversations after the reply, public comment replies, comment
snapshot reconciler, marketing, consent, identity edges (arch §14.1), inventory holds (arch
§11.2), T14 purge/DSAR. The production-mount blocker of claims §8 stays in force and now also
covers `claims.meta_intake` and Meta actor keys.

## 2. Session ↔ source binding (closes H1)

`live.claim_sources` (0064, commerce_runtime writes with `live:manage` AND
`integration:execute`, via `command.Run` + audit):

```
tenant_id, store_id, id uuid, session_id uuid → live.claim_windows,
platform text CHECK IN ('facebook','instagram'),
binding_id uuid, binding_version bigint → integration.bindings (provider 'meta', enabled at create),
object text CHECK IN ('page','instagram'), asset_id text ^[0-9]{1,40}$  -- = route asset
source_object_id text ^[0-9_]{1,80}$     -- FB: feed post_id as delivered; IG: media.id
private_reply boolean NOT NULL DEFAULT false, reply_locale text CHECK IN ('zh-Hant','zh-Hans','en'),
active boolean, version bigint, principal_id uuid (last changer), created_at, updated_at
UNIQUE (object, asset_id, source_object_id) WHERE active   -- one live object → one session, globally
CHECK (platform='facebook') = (object='page')
```

- `(object, asset_id)` must equal an enabled `meta_inbox.routes` row of the same tenant/store
  (checked in the definer; mismatch → `ErrConflict`). An object never maps to two sessions.
- Unbound objects never claim (no intake row). Deactivation is the only removal.
- FB live video mapping depends on U1: until the LIVE probe records the delivered `post_id`
  form for a live video, a `facebook` source is MOCK-only and the UI labels it "unverified".
- `private_reply=true` requires the binding to hold a current Page-token credential (§7) and is
  the merchant's explicit opt-in; its `principal_id` becomes the operation principal (§6.2).

## 3. Qualification and mapping (closes H2, H3)

Inside the existing consumer transaction (§5), after `projectSocial` returns family `comment`,
the consumer decides eligibility from the **already-authenticated decrypted unit** only:

| Kind | Qualifies | source_object_id | actor platform id | occurred_at |
| --- | --- | --- | --- | --- |
| `page_comment_add` | yes | `value.post_id` | `value.from.id` | event `occurred_at` (`created_time`) |
| `instagram_comment`, `instagram_live_comment` | yes | `value.media.id` | `value.from.id` | event `occurred_at` (entry `time` fallback) |
| `page_comment_edit/remove` | **never** (no retract either) | — | — | — |

Fail closed (no intake row, social fact still commits as today) when: `from.id` or object id
missing/malformed; `from.id` = `asset_id` (seller's own comment); `parent_id` present
(replies to comments do not claim in v1 — IR-7); no active `claim_sources` row; `occurred_at`
NULL; text over 256 bytes (claims parser short-circuit). Comment text and `from.name/username`
stay only inside the encrypted inbox/social ciphertext.

`grammar.Parse(text)` (pure, frozen kw-v1) runs in memory in the consumer. The staging function
resolves `p.Keyword` against `live.offers` of the bound session (keywords are immutable, so the
resolution is stable): the intake row stores `offer_id` when the head names an offer, otherwise
only `unknown_keyword=true` — **an unresolved keyword is never persisted** (claims R5/§8).

Actor key (H3): `actor_key = hex(HMAC-SHA256(K_actor, tupleHash-encoding("meta-claim-actor/v1",
app_id, object, asset_id, from.id)))`, 64 lowercase hex. `K_actor` =
`COMMERCE_CLAIMS_ACTOR_KEY` (32 bytes, base64; distinct from `COMMERCE_CLAIMS_LABEL_KEY` and
the reply link key; startup rejects equal keys), loaded only by the meta-worker consumer. Keyed,
because sender ids are enumerable; domain-separated from the unkeyed social `peer_key`
(`meta-social-peer/v1`). Same person on FB and IG, or on two apps/assets, yields different
bundles (actor ≠ person, claims §1). No key ring in v1: rotation splits bundles of an in-flight
session (documented limit, same as label key).

## 4. Data model — migration 0064

- `claims.bundles.platform` CHECK widens to `('manual','facebook','instagram')`;
  `claims.events.source_kind` to `('manual','meta')`, `platform` likewise; existing CHECKs
  `(platform='manual')=(label IS NOT NULL)` and `(source_kind='manual')=(principal_id IS NOT
  NULL)` stay and now carry meaning. `claims.events.reason` adds `RATE_LIMITED` (§4.2).
- `claims.events.source_event_id` for meta = `meta_inbox.events.id` (UUID; already unique per
  store across kinds). No FK (inbox retention must not block claims retention).
- `live.claim_window_intervals` (H5): append-only `(tenant,store,session,generation, opened_at,
  closed_at NULL)`; written by the existing window open/close commands in the same tx
  (integrator helper change in `internal/claims`). Unique `(tenant,store,session,generation)`.
- `claims.meta_intake` (text-free staging, FORCE RLS):

```
tenant_id, store_id, id uuid, inbox_event_id uuid UNIQUE, source_id uuid → live.claim_sources,
session_id uuid, platform text, actor_key text ^[0-9a-f]{64}$, comment_ref text,  -- see IR-4
occurred_at timestamptz NOT NULL, received_at timestamptz NOT NULL,             -- inbox created_at
grammar_version 'kw-v1', grammar_kind, offer_id uuid NULL, unknown_keyword boolean,
quantity int NULL, explicit_quantity boolean NULL, job_id bigint NOT NULL UNIQUE,
state text CHECK IN ('PENDING','APPLIED','DROPPED'), drop_reason text NULL, applied_event_id uuid NULL,
lease_xid xid8 NULL, created_at, updated_at
CHECK (offer_id IS NULL OR NOT unknown_keyword), CHECK (grammar_kind<>'NO_MATCH' OR offer_id IS NULL)
```

### 4.1 Roles

- `commerce_claims_intake` NOLOGIN, exactly one dedicated login (validated like
  `ValidateMetaConsumerPool`; mixed/owner/SET ROLE rejected in both directions). It applies
  intake rows and produces reply operations. Grants are through RLS policies whose scope is
  `claims.intake_scope()` — a STABLE definer returning `(tenant,store,session)` of the single
  `claims.meta_intake` row with `lease_xid = pg_current_xact_id()`. **No GUC-only authority**:
  setting `app.tenant_id` by hand grants nothing without a leased row (meta-consumer rule).
- `commerce_meta_consumer` gains EXECUTE on `claims.stage_meta_intake` only. No claims/live
  table privilege (claims §3.2 row "any `commerce_meta_*` role: none" stays true for tables).
- Definers owned by `commerce_claims_writer` (existing), fixed `search_path=pg_catalog`,
  `REVOKE ALL FROM PUBLIC`, `COMMENT ON` naming package and caller role.

### 4.2 Abuse bounds (H6)

Per `(session, actor_key)`: ≤10 ACCEPTED commands per rolling 60 s; per session ≤5000 bundles
with platform ≠ manual. Over the bound → `REJECTED/RATE_LIMITED` event (no bundle/line write,
no reply). Values are defaults pending IR-6.

## 5. Staging and apply (closes H4, H5)

### 5.1 Stage (consumer transaction, meta-consumer step 5–6 extension)

`claims.stage_meta_intake(p_event uuid, p_job bigint, p_attempt integer, p_object_id text,
p_actor_key text, p_occurred timestamptz, p_kind text, p_keyword text, p_quantity integer,
p_explicit boolean) RETURNS uuid` (NULL = not staged). It re-derives tenant/store/app/object/
asset/route from the **locked** inbox event + running River job exactly as
`finish_social_event` does (same validator; caller cannot supply scope), requires the social
comment fact of this event to exist in this tx, resolves the active source by
`(object, asset_id, p_object_id)` and the offer by keyword, inserts the intake row with
`lease_xid NULL, state PENDING`, and returns its id. `p_keyword` is used only for resolution and
never stored. The consumer then `InsertTx`s River kind `claims_meta_intake_v1`, args exactly
`{"intake_id":…, "version":1}`, and sets `job_id` via `claims.link_meta_intake_job(intake, job)`.

As with the social `subject_key`, SQL validates shapes, not that the Go consumer derived them
from the ciphertext; a fully compromised consumer holding the keyring is out of scope.

### 5.2 Atomicity (stripe §0.2 receipt/signal pattern)

Post-River 0014: a deferred constraint trigger on `claims.meta_intake` INSERT and one on
`river.river_job` INSERT of kind `claims_meta_intake_v1` re-read final rows at COMMIT and reject
an intake row without its exact linked job (kind/queue/args/no unique_key) or such a job without
its intake row, and any scope mismatch (follows `meta_job_commit`). The inbox event is marked
`processed` only in the same COMMIT. A rollback anywhere leaves no intake, no job, no social fact
and no processed mark (MCI04). No ACK path changes: the webhook was acknowledged at inbox
admission; staging runs in the consumer.

### 5.3 Apply (claims intake worker, `cmd/meta-worker` registers it; separate pool)

One bounded READ COMMITTED tx per job:

1. `claims.lease_meta_intake(intake, job, attempt)`: validates River job running/attempt exactly,
   locks the intake row `FOR UPDATE`, sets `lease_xid`, returns the row. `APPLIED`/`DROPPED` →
   commit, return nil (replay). Source inactive or session window has no interval with
   `opened_at <= occurred_at` and (`closed_at IS NULL` or `occurred_at < closed_at` and
   `received_at <= closed_at + 60 s`) → `DROPPED(window_closed)`, no claims event (claims §4.3
   step 3 semantics: WINDOW_CLOSED is not persisted).
2. `claims.IngestMetaIntake(ctx, tx, intakeID) (IngestResult, error)` — **new frozen entry**,
   sharing the unexported core of `IngestParsed`. Differences from the manual path only:
   window fence = the matched interval's generation (window row still locked `FOR SHARE`);
   `occurred_at` bound = interval, not "OPEN now"; unknown keyword = `UNKNOWN_KEYWORD` without a
   keyword; no `ActorLabel` (label NULL); `principal_id NULL`; rate bound §4.2 before the bundle
   step. Idempotency = claims §4.3 step 2 on `source_event_id` (immutable-fact compare).
3. On ACCEPTED that **created** the bundle and source `private_reply=true`: §6 plan in the same tx.
4. Mark intake `APPLIED` + `applied_event_id`, clear `lease_xid`, COMMIT.

Errors: 22023/invalid → `JobCancel`, intake stays PENDING (operator-visible); 40P01/55P03/
timeouts → retryable. KC15 guard widens: `Ingest/IngestParsed` only from `RecordManualClaim`;
`IngestMetaIntake` only from the intake worker.

## 6. First private reply (closes H7)

### 6.1 Budget rules (binding)

- At most **one** private reply per Meta comment, forever: operation `semantic_key =
  "mpr:" + hex(sha256(app_id|object|asset_id|comment_ref))[:48]` (UNIQUE per tenant/store in
  `integration.operations`). The key never includes template, policy, source version, link
  generation or token version, so no rule/token change can reset the budget.
- At most one automated reply per bundle in v1: only the ACCEPTED event that created the bundle
  plans a reply (IR-3). Later comments update the same bundle; the buyer reopens the same link.
- Never fall back to another channel (DM, public reply, other asset, other platform, email).
  Denial or failure is final for that comment.
- UNKNOWN is never blind-retried (external-operation v1). Reconcile is query-only; until U4 is
  proven it returns UNKNOWN and the budget exhaustion path marks manual-required.

### 6.2 Producer bridge

`integration.plan_claim_reply(p_intake uuid, p_event uuid, p_new_link_hash bytea, p_job_id
bigint) RETURNS uuid`, owner `commerce_integration_writer`, EXECUTE `commerce_claims_intake`
only. Derives everything from the leased intake row, the ACCEPTED claims event and the source:
principal = `claim_sources.principal_id`, binding/version = source binding (must be enabled and
equal), provider `meta`, action `meta.private_reply`, purpose `service` (IR-2). In one tx it:
issues link generation 1 for the new bundle (`claims.issue_system_link`, writer-owned, requires
generation 0 — a merchant-issued link never exists yet for a bundle created in this tx), inserts
the operation, its event and the `external_operation_v1` job (existing args
`{operation_id, version:1}`), and audit. Operation UUID is allocated by Go before the call.

Link token: `token = base64url_raw(HMAC-SHA256(K_link, tupleHash-encoding("meta-claim-link/v1",
tenant, store, bundle, operation_id)))` (43 chars, same format as `LinkToken`). Only
`sha256(token)` reaches SQL. The token is never stored, logged or put in the operation request;
the adapter re-derives it at Dispatch. `K_link` = `COMMERCE_CLAIMS_REPLY_LINK_KEY`, loaded by the
intake worker and the dispatcher process only. Merchant rotation via M7 kills the sent link
(claims §6 semantics). Key rotation between plan and dispatch → Check denies (BLOCKED_POLICY).

Frozen request (≤ 2 KiB, no token/text/name): `{v:1, platform, asset_id, comment_ref, bundle_id,
link_generation:1, locale, template:"claim-link/v1", origin_ref (published storefront id),
deadline_at, live_media: bool, session_id}`. `deadline_at = occurred_at + 7 d − 1 h` (F1/F2 with
margin), DB-computed.

### 6.3 Adapter route `(meta, meta.private_reply, service)`

- **Check** (every attempt; arch §10.2 "re-read at execution"): binding enabled + frozen
  version; source still active and `private_reply=true`; principal still holds
  `integration:execute` and `live:manage` for the store; `clock_timestamp() < deadline_at`;
  for `live_media` (IG live comment) the session's IG destination is currently `LIVE` per T08
  observation, else deny (F2 live rule; no observation = deny); current Page token credential
  present; link key available. Any failure → `ErrPolicyDenied` → BLOCKED_POLICY, zero calls.
- **Dispatch**: load Page token (§7), re-derive token, render fixed localized template
  (`claim-link/v1`: one sentence + URL `https://<origin>/<locale>/claim#t=<token>`; no merchant
  free text, price, discount or delivery promise in v1), `POST https://graph.facebook.com/
  <pinned U5>/{asset_id}/messages` with `{"recipient":{"comment_id":…},"message":{"text":…}}`,
  token in `Authorization` header if U6 holds, otherwise form body — never URL/query/log.
  `asset_id` = Page id (FB) or IG professional account id (IG, Facebook Login path).
  2xx with `message_id` → SUCCEEDED (`provider_reference` = message id ≤200). A documented
  permanent 4xx (policy, window, permission; codes per U3 once probed) → FAILED_FINAL with a
  fixed code. Timeout, 5xx, transport error, unparsable body, 429 → UNKNOWN.
- **Reconcile**: query-only. v1 returns UNKNOWN (U4). After U4 is proven LIVE, a reviewed
  amendment may map a positive read to SUCCEEDED. Never Dispatch again.
- Callback errors, request/response bodies, tokens and comment ids never reach logs or River.

## 7. Page-token custody (stripe §0.2 pattern, adapted)

- Page access tokens are **per store per binding**, never environment-global. 0064 adds
  `integration.meta_page_credentials(tenant, store, binding_id, version, key_id, nonce,
  ciphertext 17..8192, principal_id, created_at)` + head `current_version` on a
  `integration.meta_page_heads` row, deferred FK as in 0014. FORCE RLS; no runtime SELECT of
  ciphertext.
- Payload `meta-page-token-v1` = `{page_access_token}` only; AES-256-GCM with a distinct AAD
  `["livecommerce/meta-page-token/v1", tenant, store, binding, provider, asset_id, version,
  key_id]` and a keyring separate from the Meta payload keyring and from payment keys. The
  inbox/consumer processes never load it; only the dispatcher does.
- Provisioning in R1: operator registrar CLI (`integration.register_meta_page_token`,
  registry_writer-owned, EXECUTE registrar only; owner membership + store validated; audited;
  version CAS). Token plaintext is an owner-supplied registrar input, never in PG/logs. OAuth
  issuance and refresh remain T07.
- Loader `integration.load_meta_page_token(operation uuid, generation bigint, lease_token
  bytea)` (EXECUTE `commerce_worker`): lease-fenced like `load_stripe_credential`, returns the
  **current head for the frozen binding** (unlike Stripe's frozen version: a Page token refresh
  is not a semantic change, external-operation v1; an asset change is a new binding and
  therefore never used). Observations record the credential version used.
- Webhook signing stays per app+object (`COMMERCE_META_APPS_JSON`, meta-runtime v1): Meta signs
  with the platform app secret, and tenant routing is the server-owned asset→route map. The
  Stripe per-endpoint signing custody therefore has no Meta analogue; no per-store Meta webhook
  secret is introduced.

## 8. Privacy

Nothing in `claims.*`, `live.claim_sources`, `claims.meta_intake`, `integration.operations`,
audit, River args or logs holds comment text, unresolved keywords, `from.name`, username,
`from.id` in clear, or link tokens. `actor_key` (keyed), `comment_ref` (IR-4) and
`source_object_id` are pseudonymous personal/platform data with retention classes added to the
claims §8 list; purge/DSAR stays T14 and blocks production mount. Sentinel scans in MCI08.

## 9. Lock order

Consumer: meta-consumer order (tenant → store → binding → route → event → job) → social fact →
`live.claim_sources` (FOR SHARE) → `live.offers` (plain read) → intake insert → River job.
Intake worker: intake (FOR UPDATE) → then claims §5.6 order from `claim-source` advisory
onwards → `live.claim_window_intervals` → `integration.bindings` FOR SHARE → operation insert →
River job. Never takes a meta_inbox lock. Dispatcher: unchanged (binding → operation).

## 10. What each evidence class can prove

| Class | Proves | Cannot prove |
| --- | --- | --- |
| MOCK (UNIT/REAL_PG) | Signed synthetic webhook replay through the real API handler → inbox → consumer → staging → claims → operation → fake Graph `httptest` server; idempotency, atomicity, privacy, lock order, UNKNOWN handling | Real payload shape for FB live video (U1), permissions, App Review, window/limit enforcement by Meta |
| SANDBOX | Not applicable: Meta has no sandbox for Page messaging; a Meta test app with app-role users is LIVE with Standard Access | — |
| LIVE read-only probes | Token debug/permissions list, `subscribed_apps`, a real webhook delivery captured into the inbox (read path), comment field reads (U1, U4, U5, U6) | Sending |
| LIVE send | One private reply to an app-role test user on an owner-approved test Page | Anything for non-role users before Advanced Access/App Review |

## 11. Owner inputs (engineering cannot close)

O1 Meta app (id) in Live mode with Webhooks product; Page + IG professional (public) test assets.
O2 Permissions via Facebook Login for Business: `pages_manage_metadata`, `pages_show_list`,
`pages_read_engagement`, `pages_messaging` (+ person with `MESSAGING` task),
`instagram_basic`, `instagram_manage_comments`; `instagram_manage_messages` only if U2 says so.
O3 Advanced Access / App Review for the above (IG comment webhooks require Advanced Access, F4;
private replies to non-role users need it, F1). O4 A Page access token supplied to the registrar
for the test Page (until T07 OAuth). O5 Explicit approval in chat for each LIVE send probe (a
message to a real person, AGENTS.md). O6 Approval of the reply template wording and locales
(customer-visible copy, SHOPLINE parity). O7 Retention days for the new classes (U08).

## 12. Gates (all NOT_RUN)

Real-PG tests in `tests/foundation/meta_claims_intake_test.go`, one `TestMetaClaimsMCIxx…` per
gate; pure tests in `internal/claims` / `internal/integrations/meta`. Every sentinel synthetic.

| Gate | Evidence | Required |
| --- | --- | --- |
| MCI01 | MOCK (UNIT) | Qualification table §3 for all 7 kinds incl. edit/remove/reply/own-comment/missing `from.id`/missing object/NULL occurred_at; actor key vectors (domain separation vs peer_key; per app/asset/platform differ; key ≠ label key rejected); redacted formatting of every new input type |
| MCI02 | MOCK (REAL_PG) | 0064 fresh + populated-0063 upgrade, migrate twice; FORCE RLS; privilege matrix equality; `commerce_meta_consumer` has no claims/live table privilege; intake role without a leased row sees/writes nothing even with forged GUCs; pool validator rejects mixed roles; CHECK widening both directions |
| MCI03 | MOCK (REAL_PG) | `claim_sources`: route mismatch 409; one object → one session under concurrency; permissions `live:manage`+`integration:execute`; `private_reply` requires credential |
| MCI04 | MOCK (REAL_PG + River) | Signed webhook replay end-to-end to ACCEPTED claim; forced failure at social fact / stage / job insert / processed mark / COMMIT → zero intake, job, fact; intake without job and job without intake rejected at COMMIT; unbound object → social fact only |
| MCI05 | MOCK (REAL_PG) | Same inbox event ×20 concurrent → one intake, one claims event; redelivered webhook → duplicate; immutable-fact mismatch 409; unknown keyword stores no keyword (DB-wide sentinel scan); S03 unknown→create offer→redeliver = duplicate UNKNOWN_KEYWORD |
| MCI06 | MOCK (REAL_PG) | Window intervals: late webhook inside interval + grace accepted with that generation; beyond grace DROPPED; comment before open DROPPED; close/reopen does not merge generations; rate bound → RATE_LIMITED, no bundle |
| MCI07 | MOCK (REAL_PG + fake Graph) | One reply per new bundle; second comment same actor no reply; semantic key per comment survives template/source/key-version change (no second operation); link generation 1 hash = sha256(re-derived token); Check denials (deadline, source off, principal revoked, IG live not LIVE, binding changed, no credential) → BLOCKED_POLICY with zero HTTP calls; fake 2xx → SUCCEEDED; 5xx/timeout/429/garbled → UNKNOWN then query-only, never a second POST (child-process kill after send) |
| MCI08 | MOCK (REAL_PG) | Sentinel scan of every column in claims/live/integration/audit/River/logs: no comment text, name, username, raw `from.id`, token, Page token; Page token ciphertext AAD swap/tamper fails; consumer and API processes cannot load the page-token keyring |
| MCI09 | MOCK (REAL_PG) | Lock-order workload: consumer staging vs window close vs offer deactivate vs redeem vs issue_link vs dispatcher → zero 40P01; `pg_stat_activity` interleaves, no sleeps |
| MCI10 | REVIEW + regression | Source guards (callers of each ingest entry, no new dependency, no network in consumer/intake worker); MC01–07, MI01–07, KC01–15, dispatcher gates unchanged; full `go test -race ./...`, `go vet ./...`, `python3 scripts/check_packet.py`; independent test_worker + security_reviewer |
| MCI11 | LIVE read-only | U1, U4, U5, U6 probes and permission listing on O1 assets; captured webhook shape stored as a redacted fixture; no send |
| MCI12 | LIVE (owner-approved) | One private reply to an app-role test user on FB and one on IG (live comment during broadcast); readback in Page inbox; U2/U3 recorded |

Commands (record exit codes): `go test -count=1 ./internal/claims/... ./internal/integrations/meta/...`;
`bash scripts/dev/test-focused.sh '^TestMetaClaimsMCI'`; full suite per PROCESS §2.4.

## 13. Integrator rulings requested

IR-1 Gate prefix: brief asked `MI01..`; that collides with `meta-inbox-v1` MI01–07 (PASS), so
this draft uses `MCI`. Confirm or rename.
IR-2 Operation purpose `service` (reply to the buyer's own request) vs `transactional`; never
`marketing`.
IR-3 One automated reply per bundle (default) vs one per ACCEPTED comment (still ≤1 per comment).
IR-4 `comment_ref` storage: plain Meta comment id in `claims.meta_intake` and the frozen
operation request (needed as `recipient.comment_id`; retention class added) vs re-reading it
from the encrypted social copy at Dispatch (needs the Meta payload keyring in the dispatcher).
IR-5 River job (default, reciprocal deferred guards §5.2, grants the consumer one guarded job
kind) vs fenced poll by the intake worker (no River grant to the consumer).
IR-6 Late-arrival grace 60 s and rate bounds 10/min/actor, 5000 bundles/session.
IR-7 Replies to comments (`parent_id` set) excluded in v1.
IR-8 New frozen entry `IngestMetaIntake` + KC15 guard widening (touches FROZEN claims §4.3/§9).
IR-9 System link issuance `claims.issue_system_link` (generation 0 only) beside the merchant-
guarded `issue_link`; confirm no path lets it rotate or release.
IR-10 Page-token custody tables in 0064 vs reusing `integration.merchant_accounts` (would widen
its provider CHECK, which Stripe B1's 0061 also edits — ordering conflict).
IR-11 FB live-video claims stay MOCK-only until U1 is closed by MCI11.

## 14. Known limits

No claim retract on edit/remove; no DM follow-up or conversation takeover; no merchant template
text; IG Live replies only while T08 reports LIVE (no observation = no reply); actor key rotation
splits bundles; Reconcile cannot prove delivery until U4; Standard Access reaches only app-role
users; no performance or capacity claim; production mount blocked by T14 retention/DSAR.

## 14. Integrator rulings (2026-09-29, binding)

- IR-1 Gate prefix `MCI` confirmed. IR-2 Purpose `service`. IR-3 One automated reply per claim
  bundle (each comment still at most one, forever).
- IR-4 `comment_id` stored plainly (platform object id, not PII); the dispatcher never needs the
  Meta payload keyring.
- IR-5 River job with commit-time reciprocal checks. IR-6 60 s grace, 10/min per commenter,
  5000 bundles per session accepted. IR-7 Replies to comments excluded in v1.
- IR-8 Approved: `IngestMetaIntake` and the KC15 caller-guard widening amend the frozen claims
  contract; the amendment is recorded in `live-keyword-claims-v1.md` §0.1 by the implementing unit.
- IR-9 Approved with a gate proving `claims.issue_system_link` cannot rotate or release a link.
- IR-10 New 0064 tables for Page tokens (do not widen the 0061 provider CHECK).
- IR-11 Facebook live-video claims stay MOCK-only until probe U1.
- O6 (reply wording): ship default templates in zh-TW, zh-CN and en, chosen by the store's
  default locale; per-store editing is R2. Owner may replace the wording before go-live.
- O1–O5, O7 remain owner inputs; LIVE sends need explicit owner approval in chat.
