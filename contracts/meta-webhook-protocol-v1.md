# Meta webhook protocol v1

Status: DRAFT for independent preflight. T07 / G06, G07, G14 contribution.
This is the byte-validation and event-admission boundary, not merchant OAuth,
durable inbox storage, subscriptions, message eligibility or LIVE qualification.
The next T07 increment must connect it to PostgreSQL + River atomically before
mounting a public route. No existing customer receiver is changed.

## Authority and reuse

- Architecture sections 6, 10 and 16 require independent Meta/site-chat domains,
  server-owned binding lookup, per-event deduplication and commit before ACK.
- Daerdo receiver raw-byte HMAC/challenge patterns are useful, but its fixed
  asset allowlist and short diagnostic retention are not a multi-tenant inbox.
- Use Go standard library only; do not generalize PSP `accounts.Keyring`, whose
  AAD and credential schema are intentionally payment-account-specific.
- Official Meta documentation URLs returned HTTP 429/unavailable on 2026-09-26:
  <https://developers.facebook.com/docs/graph-api/webhooks/getting-started/>
  and <https://developers.facebook.com/docs/instagram-platform/webhooks/>.
  [Meta's sample](https://github.com/fbsamples/messenger-platform-samples/blob/main/quick-start/app.js)
  confirms the challenge and Page envelope shape, not current permissions or
  production durability. Synthetic fixture support is not LIVE protocol proof.

## API and ownership

Package `internal/integrations/meta`, no provider network calls, no database
dependency in this protocol increment. API:

```go
type Config struct { AppID, Object, AppSecret, VerifyToken string }
func NewVerifier(Config) (*Verifier, error)
func (*Verifier) Verify(raw []byte, signature string) (Batch, error)
func NewHandler(*Verifier, func(context.Context, Batch) error) (http.Handler, error)
```

Config comes from server-owned environment/secret inventory, never POST fields.
`Object` is exactly `page` or `instagram`; AppID is 1..40 ASCII digits;
AppSecret and VerifyToken are 16..512 bytes, no control bytes. Verifier owns its
configuration. Formatting/JSON of Config, Verifier, Batch and Event is redacted.
Do not log request URI, signature, challenge, raw body or provider identifiers.
One handler instance represents one configured app + object: never try a list
of unrelated app secrets until one passes.

`Batch` carries configured AppID/Object, SHA-256 of the exact received bytes,
and all events. `Event` carries AssetID, Kind, ExternalID, OccurredAt (optional),
Key (lowercase SHA-256 hex), QuarantineReason and canonical JSON Payload. Payload
may contain PII; callers must encrypt it before persistence, never serialize
the whole Batch to a queue. Batch/Event contain no inferred tenant/store.
No getter may share input buffers; the successful batch owns its bytes.

## Signature, JSON and limits

1. POST reads at most **1 MiB + one byte**; over-limit is 413, no callback.
   Content-Encoding must be absent or identity. Content-Type must be
   application/json (optional charset UTF-8); reject other types with 415.
2. Exactly one `X-Hub-Signature-256` header, `sha256=` plus 64 hex characters.
   HMAC-SHA256 of the **unmodified bytes**, fixed-time comparison, before JSON
   parsing. Missing/wrong/malformed signature is 403, no callback. No SHA-1.
3. Reject invalid UTF-8, invalid JSON, duplicate keys at any depth, trailing
   JSON values and more than 64 nested containers. Preserve number precision
   (`json.Number`); never round provider IDs through float64.
4. Root must be an object. Max **1,000 event units** per batch. Reject oversized
   batches with 413 rather than slicing, partially committing or acknowledging.
   Limits are local admission bounds, not Meta's promised delivery maximum.
5. Unknown but valid JSON shapes become quarantine events, not silent success.
   A configured/payload object mismatch quarantines the whole envelope. A bad
   entry, invalid asset ID or unsupported event is represented with its payload
   and a fixed reason. Empty/missing entries produce one envelope quarantine.
   Every `entry[]`, `changes[]` and `messaging[]` unit is accounted for. A known
   entry that also contains an unsupported sibling event-bearing field (for
   example `standby`) also produces an entry quarantine; no sibling is lost.

## Normalization and identity

Recognized shapes (classification, not send authorization):

- Page `changes.field=feed`, `value.item=comment`, `value.verb` add/edit/remove,
  nonempty `comment_id` (1..200 ASCII alphanumeric, `_`, `-`, `:`, `.`) =>
  `page_comment_add/edit/remove`. Never coerce numeric IDs into strings.
- Instagram changes `comments` / `live_comments`, numeric string `value.id`
  => `instagram_comment` / `instagram_live_comment`.
- Page/Instagram `messaging` with message.mid (1..1024 printable ASCII),
  numeric string sender.id/recipient.id, recipient == entry.id, sender !=
  entry.id, absent/false is_echo => `page_message` / `instagram_message`.
  Echo, wrong recipient, malformed is_echo, missing IDs => quarantine.
- All other shapes => quarantine. No attachment fetching, send, comment reply,
  order mutation, identity merge, consent or messaging-window update occurs.

Canonical JSON means recursively sorted object keys using `encoding/json`
with `json.Number`, retaining arrays/order and all fields. The event key is
SHA-256 of JSON tuple `["meta-event-v1", configuredAppID, configuredObject,
assetID, kind, externalID, canonical-event-payload]`; tuple avoids delimiter
collisions. For recognized units payload is the full change/messaging object,
not body text and not entry delivery timestamp. A duplicate wrapped in a new
batch or key ordering shares a key. Different IDs with identical text, app,
object, asset, verb or changed payload remain distinct. Do not dedupe the
returned slice: durable storage owns the unique constraint and conflict policy.

Quarantine identity uses the relevant whole unit/envelope, preserving entry
context including time if needed. Unknown events MUST NOT become commerce
triggers. OccurredAt uses integral positive message.timestamp in milliseconds,
Page value.created_time in seconds, otherwise entry.time in seconds; absent or
invalid/out-of-range timestamps remain unset. It is evidence, not a monotonic
revision or a new message-window grant. Repeated identical transitions without
a provider revision cannot be distinguished: retain this limitation for later
reconciliation; never use arrival order to cancel paid orders.

## HTTP response contract

- Caller mounts the handler at an exact server-owned route. GET requires exactly
  one hub.mode=subscribe, hub.verify_token and hub.challenge; no extra keys;
  challenge is 1..200 ASCII `[A-Za-z0-9._-]`. Compare verify token in constant
  time (hash both to fixed width); return challenge as text/plain, no-store.
- POST rejects any query string (including a bare `?`). It verifies/parses all
  events before calling the supplied **commit function once**. Nil verifier or
  commit function makes NewHandler fail. No goroutine/early/background ACK.
- Commit returns nil ONLY after the complete batch has durable receipts and
  processing jobs/quarantine atomically committed. HTTP 200 `EVENT_RECEIVED`
  only then. Failure/cancellation => 503 without exposing callback error text.
  Caller can retry ambiguous commit; permanent event uniqueness is required.
- Other methods => 405 with Allow: GET, POST. Error bodies contain only fixed
  codes; all responses no-store. No public route is mounted by this increment:
  a test callback cannot substitute for PostgreSQL durability in deployment.

## Gates / completion boundaries

MWP01 raw-byte signature, Unicode escapes, byte change, wrong app/secret, unsafe
configuration and redaction. MWP02 precise JSON and duplicate/depth/size bounds.
MWP03 every batch unit, Page/IG separation, echo/recipient/unknown quarantine,
no truncation. MWP04 same event/reordered JSON/rebatched identity and distinct
ID/edit/delete/content/asset/app separation. MWP05 HTTP challenge/method/header/
encoding/query gates; blocked callback proves no early ACK, failure proves 503.
Independent test design + reviewer, root `go test -race` and `go vet` required.

NOT_RUN: actual PostgreSQL receipt/job transaction, durable uniqueness/encryption
and tenant asset routing; public handler wiring/rate limits; Meta signed sample,
OAuth/app review/subscriptions, all message reads/sends and full T07 acceptance.
Upgrade signal: next increment supplies durable admission; provider fixtures
extend recognized shapes only after source/live evidence and regression cases.
