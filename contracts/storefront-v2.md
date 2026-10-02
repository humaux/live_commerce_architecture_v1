# Storefront v2 — independent-store baseline (R4)

Status: FROZEN 2026-10-01 by the integrator for R4 wave 1. Producers implement exactly these shapes; the
consumer (unit storefront-shell, wave 2) codes against them. Changes need an integrator amendment here.
Why: owner correction 2026-10-01 — the product must be a qualified independent-store SaaS (SHOPLINE parity
for a Taiwan merchant, see output/r4-parity screenshots + memory "[research] 直播SaaS 独立站基线差距矩阵").
Rules from AGENTS.md apply unchanged: store/tenant from server auth or the published-origin resolver only,
never from the request; PG is the only source of truth; no client-supplied amounts.

Merchant content (titles, descriptions, page bodies, nav labels) is single-language, stored as typed by
the merchant. UI chrome stays zh-CN/zh-TW/en via the existing i18n package.

## A. Catalog v2 (producer: unit catalog-core, migration 0086)

Data model additions:
- Product `status`: `draft | active | archived` (existing rows `active`/`archived` keep meaning). Only
  `active` products of a published store are buyer-visible. New products default to `draft`.
- Product `slug` (`^[a-z0-9]+(?:-[a-z0-9]+)*$`, ≤ 80, unique per store, generated from the title on
  create when absent — pinyin is NOT required: fall back to the product id prefix), `seo_title` ≤ 70,
  `seo_description` ≤ 160 (both optional).
- Options: product has 0..3 option axes `{name ≤ 30, values[] 1..50, each ≤ 40}`; each SKU carries
  `option_values[]` aligned to the axes (unique combination per product). SKU `title` is derived
  (`values joined " / "`, or "預設" when no axes) — not stored separately.
- SKU `compare_at_minor` optional (> price), for strike-through.
- Collections: `{id, slug, title ≤ 80, description ≤ 2000 plain text, image_id?, sort_mode: manual |
  newest | price_asc | price_desc, status: active|hidden}`, manual membership with positions. A product
  may be in many collections.
- Stock hint for buyers is derived, never raw counts: `in` (available ≥ 6), `low` (1..5), `out` (0),
  computed over all warehouses' available quantity (on_hand − reserved) for the SKU.

Buyer reads (Go private buyer routes behind the storefront BFF, published-origin scoped):
- `GET catalog/v2/products?collection=<slug>&q=<text ≤ 60>&sort=newest|price_asc|price_desc|title&min=<minor>&max=<minor>&after=<cursor>&limit=≤48`
  → `{store:{name, currency}, products:[{id, slug, title, price_min_minor, price_max_minor, compare_at_min_minor|null, cover_image_id|null, in_stock: bool}], next: cursor|null}`.
  `q` matches title/description/SKU code, case-insensitive literal (escape LIKE), active only.
- `GET catalog/v2/products/{slug_or_id}` → `{id, slug, title, description, seo:{title, description}, images:[{id,width,height}], options:[{name, values[]}], variants:[{sku_id, title, option_values[], price_minor, compare_at_minor|null, stock: in|low|out}], collections:[{slug,title}]}`; 404 identical for unknown/draft/archived/foreign store.
- `GET catalog/v2/collections` → `{collections:[{id, slug, title, image_id|null, product_count}]}` (active only, count of active products).
- `GET catalog/v2/collections/{slug}` → `{id, slug, title, description, image_id|null}` (products via the list route with `collection=`).
  AMENDMENT 2026-10-01 (unit storefront-integration, migration 0093): both reads also return `id` (the collection uuid). The photo URL is
  `/media/c/{id}/{image_id}` (the Go route needs the collection id, so the storefront could not build it from the earlier slug-only shape).
- Images keep the catalog-media path `/media/p/{product_id}/{image_id}`; collection images use `/media/c/{collection_id}/{image_id}` (same rules: published store, active collection, immutable cache).

Merchant admin routes (scope from server auth; catalog:read / catalog:write): CRUD for collections
+ membership + order; product PATCH accepts `status, slug, seo_title, seo_description, options`; SKU
create/PATCH accepts `option_values, compare_at_minor`. Optimistic `expected_version` everywhere.

### A. Implemented shapes and acceptance list (unit catalog-core, migration 0086, appended 2026-10-01)

Implementation facts the tester codes against (the frozen text above stays authoritative; where it was silent, this is the choice made):
- DB default of `catalog.products.status` stays `active` (raw-SQL fixtures, legacy rows); `catalog.CreateProduct` / `POST products` default to `draft`. Create accepts `status` draft|active only.
- Merchant routes (base `/v1/admin/stores/{store_id}`): `GET catalog-products?q&status&cursor&limit` (catalog:read+inventory:read; cards with `cover_image_id`, `price_min/max_minor`, `sku_count`, `available`), `GET products/{id}` (product + `skus[]` with `available`), `PATCH products/{id}` (partial: `name description status slug seo_title seo_description options` + `expected_version`), `POST skus` / `PATCH skus/{id}` (`option_values`, `compare_at_minor`; PATCH is a full replace, absent compare-at clears, absent option_values keeps), `POST skus/{id}/price` (`compare_at_minor` absent=keep, null=clear), collections: `GET|POST collections`, `GET|PATCH collections/{id}`, `POST collections/{id}/delete`, `PUT collections/{id}/products` (complete ordered list), `POST|GET collections/{id}/image`, `POST collections/{id}/image/delete`. Schemas: `contracts/catalog-inventory-openapi.json`.
- Buyer routes (no buyer bearer, BFF key + verified origin, like `/v1/buyer/media/p`): `GET /v1/buyer/catalog/v2/products`, `.../products/{slug_or_id}`, `.../collections`, `.../collections/{slug}`, `GET /v1/buyer/media/c/{collection_id}/{image_id}`. Authorization or Cookie header = 403; unknown query key / bad value = 422; unknown, draft, archived, hidden, foreign or unpublished = 404 `not_found`.
- List: only active products with >= 1 active SKU in the store currency. `sort` absent = the collection's `sort_mode` (manual = membership order), else newest. `min`/`max` select products whose [price_min, price_max] overlaps. `next` is an opaque offset cursor bound to the filter (changing any filter or `limit` makes it 422). Stock: `available = on_hand - reserved - allocated - unavailable` summed over warehouses (the ledger's Available; equals the contract's on_hand - reserved while allocated = unavailable = 0), `in` >= 6, `low` 1..5, `out` <= 0; raw counts never leave the server.
- `seo_title` / `seo_description` are strings, `""` = unset. A product with no axes keeps `option_values: []` and title `預設`; legacy axis-less products may hold several SKUs. Uniqueness of a combination is per product among ACTIVE SKUs with at least one value. Changing `options` is 409 while an active SKU would not align with the new axes. At most 100 active SKUs per product, 200 collections per store, 500 products per collection.
- Slug: invalid shape or UUID-shaped = 422, taken (explicit) = 409, generated = suffixed `-2`, `-3`, a title without ASCII letters/digits falls back to the 12-hex id prefix (collections: random 12 hex). Stock may be adjusted on a draft product (inventory.AdjustOnHand allows draft); reservations still require active.
- Collection image: one per collection, upload replaces it (new id); the public bytes route serves only an active collection of a published store, `Cache-Control: public, max-age=86400, immutable`.

Acceptance (what must be proven; each with one red run before its green):
- CC01 migration 0086 on a populated pre-0086 DB: legacy products get a valid unique slug (id prefix), keep their status; legacy SKUs get `option_values {}`; FORCE RLS is back on `catalog.products`; replay of an older command result still decodes.
- CC02 draft lifecycle: new product is `draft`; not in buyer list, detail (id and slug), feed, cart or purchase-entry; `PATCH status=active` makes it visible; back to `draft`/`archived` hides it again.
- CC03 slug rules: generated, suffixed on collision, id-prefix fallback; explicit taken = 409; invalid/UUID-shaped = 422; buyer detail by slug and by id return the same product; foreign-store slug is 404.
- CC04 options and variants: axes validation (<= 3, <= 50 values, lengths, duplicates); SKU alignment (arity, value in axis) = 422; duplicate active combination = 409, re-creatable after archive; axes change that strands an active SKU = 409; derived titles; matrix of 3 axes through the admin editor.
- CC05 compare-at: must exceed price on create/update/price route (422); price rise past it is 422 unless `compare_at_minor` is sent in the same request; `null` clears; buyer shows `compare_at_min_minor` / `compare_at_minor`; checkout/quote never use it (server price only, I05).
- CC06 buyer list: literal q (escaped `%` `_`, case-insensitive, name/description/SKU code), sort modes, price overlap filter, collection filter, limit <= 48, cursor stability and filter binding, `next: null` on the last page, no draft/archived/foreign/zero-SKU product.
- CC07 stock hints: in/low/out boundaries (6, 5, 1, 0) summed over two warehouses; reservation lowers it; no raw count in any buyer payload.
- CC08 collections: CRUD with optimistic versions, membership replace (add, remove, reorder, foreign/unknown id 404, duplicate 422, > 500), manual order vs sort modes, hidden collection absent everywhere (list, detail, filter, image), draft/archived members not shown nor counted, delete keeps products.
- CC09 collection image: sniffed type (SVG/GIF refused 422, > 2 MiB 413), replace gives a new id and the old id is 404, bytes + immutable cache + nosniff on `/media/c`, 404 for unpublished store / foreign collection.
- CC10 isolation and authority: other tenant/store cannot read or write any of these rows (404); catalog:read / catalog:write / inventory:read permissions per route; `X-Tenant-ID`/Host never select scope; buyer routes refuse Authorization and Cookie.
- CC11 idempotency: every command replays its first answer for the same key and bytes, 409 for the same key with other bytes; a null/absent `compare_at_minor` differ in the replay hash.
- CC12 admin UI (browser, three locales, 375 px and desktop): products list (search, status filter, cover, price range, stock, paging), editor (create as draft, publish, slug, SEO counters, photos, axes -> matrix -> create variants, price/compare-at, SKU rename, stock adjust with retry of an uncertain write, archive variant), collections page (create, edit, image, add/remove/reorder products, hide, delete), nav entries Products / Collections / Inventory (ledger stays reachable); no raw HTML, no horizontal page scroll.

## B. Store design (producer: unit store-design, migration 0087)

One JSON document per store, versioned. `draft` (one per store, CAS on `version`) and an append-only
list of `published` versions (`version`, `published_at`, `published_by`). Publish copies draft → new
published version; rollback = publish a copy of an older published version (never mutate history).
Validated server-side against this schema (unknown keys rejected; strings length-capped; URLs must be
`https://` or `line://` / `tel:` / `mailto:` where stated; no HTML anywhere; images are store-media ids):

```json
{
  "profile": {
    "name": "≤60", "tagline": "≤120|null", "logo_image_id": "uuid|null", "favicon_image_id": "uuid|null",
    "accent_color": "#RRGGBB", "announcement": "≤140|null",
    "contact": {"email": "≤120|null", "phone": "≤30|null", "address": "≤200|null",
                "line_url": "https://line.me/... |null", "facebook_url": "https://|null", "instagram_url": "https://|null"}
  },
  "nav": {
    "header": [{"label": "≤30", "kind": "home|all_products|collection|page|url", "target": "slug|https-url|null"}],
    "footer": [{"label": "≤30", "kind": "page|url|collection", "target": "..."}]
  },
  "home": {"sections": [
    {"type": "hero", "image_id": "uuid", "heading": "≤80|null", "subheading": "≤160|null", "cta_label": "≤24|null", "cta_kind": "all_products|collection|page|null", "cta_target": "slug|null"},
    {"type": "featured_collection", "collection_slug": "slug", "heading": "≤80|null", "limit": "4..24"},
    {"type": "product_grid", "heading": "≤80|null", "sort": "newest|price_asc|price_desc", "limit": "4..48"},
    {"type": "rich_text", "heading": "≤80|null", "body": "≤4000 restricted markdown"},
    {"type": "image_text", "image_id": "uuid", "heading": "≤80|null", "body": "≤2000 restricted markdown", "image_side": "left|right"}
  ]},
  "pages": [{"slug": "slug", "title": "≤80", "body": "≤20000 restricted markdown"}]
}
```
Limits: header nav ≤ 8, footer ≤ 12, sections ≤ 20, pages ≤ 20. Restricted markdown = paragraphs,
`**bold**`, `*italic*`, `- lists`, `[text](https://...)` links only; rendered by the storefront with
escaping (no raw HTML pass-through). A store with no published version renders a default derived from
`control.stores.name` + a `product_grid` section.

Store media: `design.store_media` (same validation/limits as catalog-media CM1/CM2, ≤ 60 per store),
served at `/media/s/{image_id}` (published store only).

Buyer read: `GET design/published` → `{version, document}` (published-origin scoped).
Preview: admin issues a short-lived (15 min) signed preview token bound to store + draft version; the
storefront renders the draft when `?preview=<token>` is present, with `Cache-Control: no-store` and a
visible "預覽" banner; tokens never grant anything else.
Admin routes (store:write — use the narrowest existing permission the settings wizard uses):
`GET/PUT design/draft`, `POST design/publish {expected_draft_version}`, `GET design/versions`,
`POST design/rollback {version}`, `POST design/preview-token`, store-media upload/list/delete.

## C. Checkout additions (producer: unit checkout-offline, migration 0088)

- Payment mode `bank_transfer` for home delivery (and CVS pickup when the merchant enables it):
  merchant configures bank name, branch, account name, account number (shown to buyers only on their
  own order page), transfer window hours (6..168, default 72). Order is placed `AWAITING_TRANSFER`,
  stock reserved until the window ends (reuse the existing expiry worker path), buyer may submit
  `{last5 digits, amount_minor, paid_at}` once (editable until confirmed), merchant confirms or rejects
  (audited, permission `payments:refund`-level or a new `payments:confirm` granted to owners), confirm →
  order CONFIRMED with a `payments.facts`-equivalent offline fact so finance shows it in its own
  column; expiry → order cancelled and stock released. No PSP involved; never auto-confirm.
- Free-shipping threshold per delivery policy: `free_shipping_threshold_minor|null`; quote applies
  shipping 0 when merchandise subtotal ≥ threshold (server-side, in the existing quote path).
  AMENDMENT 2026-10-01 (unit storefront-integration): every `GET /v1/buyer/checkout-options` row carries the same key
  `free_shipping_threshold_minor` (integer minor units, or `null` when the policy has none or the threshold is 0 = always free, which needs no
  hint). It lets the cart and the delivery step word "add X more for free delivery"; the quote stays the only authority on the amount charged.
- Buyer email (optional, validated, ≤ 254) captured at checkout and stored on the order for
  notifications (PII: erasure/export paths of customers-privacy must include it).
  AMENDMENT 2026-10-02 (checkout-offline fixes K3-03, migration 0099): the buyer's transfer proof (`proof_last5`, amount, paid_at) belongs to the buyer privacy export (key `bank_transfer_proof` on the order, `null` without a submission) and erasure clears `proof_last5` while the amount and paid_at stay as the financial record with no identity.
- AMENDMENT 2026-10-02 (checkout-offline fixes K3-01/02/04, migration 0099): the transfer decisions of one Idempotency-Key serialize on a key-scoped advisory lock, so a key reused for another order is 409 `idempotency_conflict`, never a 5xx; `POST .../bank-transfer/refund-offline` takes `{}` (no stock change, as before) or `{"restock":bool}`, and `restock=true` releases the confirm allocation (one MERCHANT `DEALLOCATE` ledger row per line, audit `checkout.bank_transfer_refunded_offline_restock`) in the same transaction as the refund fact only while the order has not been handed to fulfilment, else 422 `already_shipped`; a confirm with no buyer submission stays allowed and answers `confirmed_without_proof:true`, adding the audit row `checkout.bank_transfer_confirmed_without_proof`, and the merchant dialog warns first.

## D. Staff (producer: unit staff-team, migration 0089)

Roles = fixed permission bundles over the existing permission list (0065): `owner` (all), `admin`
(all but staff management and billing), `live_operator` (live:*, catalog:read, orders:read,
inventory:read), `fulfilment` (orders:read, fulfillment:write, orders:export, inventory:*), `viewer`
(all :read). Owner invites by email (existing mail sender; one-time token, 72 h, single use, bound to
store + role), invitee signs up / logs in with password auth and accepts; owner can change role or
revoke (immediate: sessions of that membership stop authorizing on next request). At least one owner
always remains. All actions audited.

## E. Buyer communications (producer: unit buyer-comms, migration 0090)

Amendment written before code; the integrator reviews it on merge. Owners: SQL schema `notify` + `checkout.guest_order_lookup`
(migration 0090, owner role commerce_checkout_writer), Go `internal/notify` (renderer, worker, merchant toggle), `internal/buyerhttp`
(lookup route), storefront `/[locale]/orders/lookup`. Evidence labels per AGENTS.md; real SMTP is never used by tests (loopback fake).

**E1. Events (exactly once per (order, kind)).** `notify.outbox` has PRIMARY KEY (order_id, kind); rows are inserted by AFTER UPDATE
triggers inside the same transaction that moves the order, `ON CONFLICT DO NOTHING`, never from Go and never inside an SMTP call.
A trigger failure is swallowed with a WARNING: a mail problem must never roll back a payment or a stock move.

| kind | transition that enqueues it |
|---|---|
| `placed` | `commercial_state` becomes AWAITING_TRANSFER (bank_transfer), or CONFIRMED for pay_at_pickup (pickup details) |
| `paid` | CONFIRMED for card (capture) or for bank_transfer (merchant confirmed the transfer) |
| `shipped` | `fulfillment_state` becomes MERCHANT_SHIPPED (carrier + tracking), or a CVS shipment reaches AT_DC / AT_STORE (pickup store) |
| `cancelled` | CANCELLED coming from AWAITING_TRANSFER or CONFIRMED (expiry, rejection window end, cancel). Abandoned DRAFT / AWAITING_PAYMENT card checkouts send nothing |
| `refunded` | Stripe refund fact SUCCEEDED, bank_transfers REFUNDED_OFFLINE, or collection_state REFUNDED_OFFLINE |
| `merchant_new` | the first transition of an order into AWAITING_TRANSFER, or into CONFIRMED from DRAFT / AWAITING_PAYMENT (not bank-transfer confirm) |

**E2. Delivery.** One worker loop (`notify.Worker`, hosted by `cmd/expiry-worker`: it already holds the checkout-lifecycle worker pool and
the only River client that handles order expiry; no new process, no River job kind, because queue admission is guarded by post_river
triggers owned by other units) claims rows through `notify.claim_batch` (FOR UPDATE SKIP LOCKED, state PENDING -> SENDING), renders, sends
one SMTP attempt, records through `notify.record_result`. SENT is final. A definite refusal (mail.ErrFailed) goes back to PENDING with
backoff 2 min, 10 min, then FAILED after the third attempt. `mail.ErrUnknown` records UNKNOWN and is NEVER re-sent (SMTP has no
idempotency key, I06): it is logged once (`buyer_mail_unknown`, kind + order id only) and stays queryable. A SENDING row whose worker died
(claimed over 15 min ago) becomes UNKNOWN for the same reason. A PENDING row older than 24 h becomes SKIPPED (a stale "shipped" mail is
worse than none). The recipient is read from `checkout.orders.buyer_email` at send time only; no body, no address is stored.

**E3. Caps (must not starve merchant login codes).** Counted per UTC+8 day over rows that reached SENDING: all buyer and merchant notify
mail together may not exceed floor(COMMERCE_MAIL_DAILY_CAP x 60 / 100); per store at most 30 buyer mails in a rolling hour. Over a cap the row
stays PENDING (retried next tick, SKIPPED after 24 h). The login-code buckets of identity are a separate ledger and are untouched.

**E4. Content.** Locale zh-TW unless the order snapshot carries a known locale (today it does not; en and zh-CN copy exist for when it does).
Plain text + simple HTML, no remote image, no tracking pixel, all dynamic text HTML-escaped, store name + order number + link
`{published origin}/{locale}/orders/{order_id}` (origin = the store's ACTIVE storefront domain; no link when none). Order number = first 12 hex
digits of the order id, uppercase, shown as `XXXX-XXXX-XXXX`. `placed` (bank_transfer) carries the order's own bank snapshot and the
transfer deadline (`orders.expires_at`); `placed` (pay_at_pickup) the pickup store; `shipped` carrier + tracking number (+ https tracking
URL when present) or CVS store + shipping number.

**E5. Guest order lookup.** `POST /v1/buyer/orders/lookup` (BFF `POST /api/buyer/orders/lookup`, page `/[locale]/orders/lookup`). Body
`{order_ref, contact}`: order_ref = the order number (12 hex, dashes/spaces/case ignored) or a full order id; contact = the order's buyer
email, or the delivery phone (digits, +886/leading 0 ignored). The BFF mints a fresh cookie envelope exactly like `session/prepare`
and sends its token as the bearer; Go resolves the store from the published origin only (never the body), forwards the client IP
(`X-Commerce-Client-IP`, one valid literal, as for password auth) and calls `checkout.guest_order_lookup`. On a match the definer registers
a NEW capability session for the order's existing buyer owner (hash of the BFF token; same TTL and cookie as checkout) and returns
`{order_id}`; the BFF then sets the cookie (replacing any current buyer cookie) and the page navigates to `/{locale}/orders/{order_id}`.
Every mismatch (unknown order, wrong email/phone, erased owner, other store) is the same 404 `not_found`, produced by the same work (one
index range scan, one sha256 compare against the stored value or a dummy), so neither body nor timing reveals existence. Limits (fixed
10-minute windows, counted before any lookup, per hashed key): 10 per client IP, 5 per order ref, 200 per store -> 429 `rate_limited` with
Retry-After. The issued session is VIEW-ONLY (integrator ruling): `buyer.capability_sessions.view_order_id` names the one order it may read. The Go buyer
handler classifies every authenticated request through `buyer.session_view_order` and, for a view-only session, allows only GET session, session
bootstrap/retire/logout and GET `/v1/buyer/orders/{that order}` plus, read-only, GET of that order's `/payment` status and `/bank-transfer`
instructions; every other route (order list, other orders, any POST/PUT incl. payment prepare/handoff and the transfer proof, CVS, claims,
consents, privacy export / erasure, cart, checkout) is 403 `forbidden` (default deny, so a new route is closed until listed). The checkout-issued
capability (view_order_id NULL) keeps its rights. AMENDMENT 2026-10-01 (defect D2, migration 0098): a lookup whose bearer already names a
capability session (a registered buyer's token reused as the lookup bearer; the BFF always mints fresh, so only a buggy/hostile client does
this) is NOT a mismatch — the order did match — so the indistinguishable 404 does not apply; it is refused with a clear non-retryable
409 `conflict` and the existing session is left untouched (issuing over it would silently downgrade a full buyer capability to view-only).

**E6. Merchant new-order mail.** Sent to the store's owner address(es) (`identity.store_staff` role owner, verified password email), one
mail per store per 5 minutes covering every pending `merchant_new` row ("N new orders", no buyer data, link to the admin orders page is not
included because the admin origin is not store data). Opt-out: `notify.store_settings.merchant_new_order_email` (default true), merchant
routes `GET|PUT /v1/admin/stores/{store_id}/notification-settings` (integration:read / integration:manage), a Settings toggle. An opted-out
store's rows are SKIPPED.

**E7. Retention and erasure.** The mail log keeps kind, order id, state, attempt count, timestamps and `recipient_hash`
(sha256 of "order id : lowercase address"), never a body or an address. When `checkout.orders.buyer_email` is set to NULL (customers erasure
via `checkout.clear_buyer_email`) a trigger clears `recipient_hash` of that order's rows and SKIPs its PENDING ones. Rows older than 180 days
are deleted by `notify.claim_batch` (bounded, 200 per call).

**E8. Configuration.** `COMMERCE_BUYER_MAIL_ENABLED=1` switches the loop on in `cmd/expiry-worker`; then it needs the same SMTP variables
as the API (COMMERCE_SMTP_HOST / _USERNAME / _PASSWORD[_FILE], COMMERCE_MAIL_FROM) and COMMERCE_MAIL_DAILY_CAP (default 200, 20..100000). Unset
= no mail is claimed (rows wait up to 24 h, then SKIPPED).

## B-acceptance (unit store-design, implemented; evidence labels per AGENTS.md)

Wire facts the storefront-shell consumer needs (all additive to section B; migration 0087, Go `internal/design`):
- Buyer reads (private Go routes behind the storefront BFF, BFF key + `X-Commerce-Storefront-Origin`, no buyer bearer,
  no query string, GET only):
  `GET /v1/buyer/design/published` -> `{"version": n, "document": {...}}`; a store with no published version answers
  `version: 0` and the contract default (store name, accent `#247965`, one `product_grid`), never 404.
  `GET /v1/buyer/design/preview` with header `X-Commerce-Design-Preview: <token>` -> same shape for the DRAFT, `Cache-Control:
  no-store`. A missing, malformed, unknown, expired (15 min), other-store or stale token (the draft was saved after the token
  was issued) is the same `404 not_found` as an unpublished store. The token travels in the header, never in a query string
  between BFF and Go; the browser still uses `?preview=<token>` on the storefront URL.
  `GET /v1/buyer/media/s/{image_id}` -> image bytes (`public, max-age=86400, immutable`, `nosniff`, sandbox CSP); 404 unless the
  image belongs to the published store.
- Documents returned to buyers are the server-normalised form: every nullable key present as `null`, `accent_color`
  lower-case, strings trimmed; nav/home/pages always present.
- Admin: `GET|PUT design/draft` (PUT body `{expected_version, document}`; `expected_version` 0 creates), `POST design/publish
  {expected_draft_version}`, `GET design/versions`, `POST design/rollback {version}`, `POST design/preview-token {}`, media
  `GET|POST design/media`, `GET design/media/{id}`, `POST design/media/{id}/delete`. Permissions `integration:read` /
  `integration:manage` (the Settings storefront card's). 409 `conflict`: stale version, draft already live, rollback to the live
  version, media still referenced by the draft or the live version, 60-image cap. 422 `invalid_request` carries
  `details: {path, reason}` of the first offending field (`home.sections[2].heading`).
- Rules chosen where section B was silent: containers (`nav`, `home`, `pages`, `profile.contact`) may be omitted; every leaf
  without `|null` is required; `""` for a nullable string is stored as null; `cta_target` must be null unless `cta_kind` is
  `collection`/`page`; `page` targets must be a page slug of the same document; page slugs unique; `line_url` accepts https
  or `line://`; markdown bodies may be empty. Rollback publishes a copy as a new version and leaves the draft untouched.

### D acceptance (appended by unit staff-team, migration 0089; append only)

Implemented as: `identity.store_staff` (role label) + `identity.staff_invitations` (sha256 token hash, 72 h CHECK) + definers
`identity.staff_{list,invite,record_invite_mail,revoke_invite,set_role,remove,accept}` (EXECUTE commerce_identity only; owner commerce_staff_writer).
Transport: `POST /v1/identity/staff/{list,invite,revoke-invite,set-role,remove,accept}` (BFF key + merchant bearer) behind
`POST /api/team/{action}`; pages `/[locale]/team` and `/[locale]/invite/[token]`. Role bundles are `identity.staff_role_permissions(role)`
(owner = the whole live permission catalogue at grant time, incl. ads:* and billing:manage, the store creator included, existing owners backfilled; admin = catalogue minus billing:manage; viewer = every :read; staff management = the owner role row itself).
Navigation: `GET /v1/admin/stores` items also carry the caller's `role` and effective `permissions`; the admin hides entries the member cannot use (display hint, Go authorizes every request). `/[locale]/invite/**` is served with real `Referrer-Policy: no-referrer` and `Cache-Control: no-store` headers.
- Owner floor: every mutating definer checks it under a per-store advisory lock and a deferred constraint trigger re-checks at commit (direct DML cannot orphan a store).
- Revoke = role row + store grants deleted, membership deactivated when no grant remains; `identity.resolve_access` therefore refuses on the next request.
- Accept refusal is one generic PT404 `invite_invalid` (unknown/expired/revoked/used token, other email, OIDC-only account); PT409 `already_member` only for the caller's own membership.
- Limits: 20 live invitations and 50 creations per 24 h per store (PT429 `too_many_invitations`). Mail: one send after commit, outcome in `mail_state`, a resend is a new invitation that revokes the old one.
- Evidence tier: REAL_PG author smoke `tests/foundation/staff_team_smoke_test.go` (4 tests, one red run each for email binding and owner floor); browser/E2E and mail over real SMTP: NOT_RUN.

## F. Promotions (producer: unit promotions, migration 0091; amendment written before code, R4 wave 2)

Discount codes. The server quote stays the only price authority (cart-quote-v1, checkout-quote-validation-v1): a code is validated
and applied INSIDE `storefront.CreateQuote`, its effect is frozen in the quote snapshot, and BeginCheckout re-validates it under a
lock. No client amount, no second total: the existing `discount_minor` fields (line + aggregate) are filled.

Code (merchant-managed, one row per `(store, code)`):
- `code`: stored upper-case, `^[A-Z0-9-]{3,24}$`; buyers type any case, the server trims and upper-cases. Unique per store.
- `kind`: `percent` (integer 1..90, floor rounding on the merchandise subtotal) or `fixed` (positive amount in store-currency
  minor units, capped at the merchandise subtotal). The discount never exceeds the merchandise subtotal, so goods never go below 0.
  Whole-currency-unit rule: TWD is charged in whole dollars (stripe-psp-v1 D15, amount%100), so for a TWD store the discount is
  rounded DOWN to a whole dollar (never up) and a `fixed` amount must itself be a whole-dollar multiple of 100 minor units
  (`invalid_promotion` otherwise); other currencies use their minor unit. A code whose discount on the cart is 0 after this rounding
  is refused as `promo_invalid` instead of burning a use.
- `min_subtotal_minor` (>= 0, compared with the PRE-discount merchandise subtotal), `starts_at` / `ends_at` (optional instants;
  the admin UI enters Asia/Taipei wall time and sends RFC 3339 with `+08:00`; `ends_at > starts_at`), `total_limit` and
  `per_buyer_limit` (optional positive integers), `status` `active|paused`, `version` (CAS on every change).
- A code never discounts shipping. Free shipping stays the delivery policy threshold of section C, compared with the PRE-discount
  merchandise subtotal (a code cannot un-waive shipping the buyer qualified for; the threshold is a statement about the cart).
- One code per order. Tax is computed on the discounted goods (line discount allocated proportionally, largest remainder, ties to the
  lower line index); exclusive total = subtotal - discount + shipping + tax, inclusive total = subtotal - discount + shipping.

Quote: `POST /v1/buyer/quotes` accepts optional `promo_code` (omitted/`""` = none). The quote snapshot gains
`promotion: {id, code, version, kind, percent, fixed_minor}` ONLY when a code applied (key absent otherwise, so pre-0091 snapshots
round-trip byte-equal through `checkout.begin_hold`'s comparison); the buyer quote response carries `promotion: {code, kind, percent,
fixed_minor}` under the same rule. A refused code makes the quote request fail with HTTP 422 and a coded envelope (no quote is
stored): `promo_invalid` (unknown, other store, or paused: one answer, nothing to enumerate), `promo_not_started`, `promo_expired`,
`promo_min_subtotal`, `promo_used_up`, `promo_buyer_limit`. BeginCheckout adds `promo_changed` (the code was edited or paused after
the quote was priced: re-quote). Quote-time checks are advisory (no lock); BeginCheckout's are authoritative.

Usage counting (atomic, no oversubscription): a redemption row is written by `promotions.redeem` in the SAME transaction as
`checkout.begin_hold`, after the order row exists, under `FOR UPDATE` on the code row, which serialises every placement with that
code. Usage = redemptions whose order is still `DRAFT | AWAITING_PAYMENT | AWAITING_TRANSFER | CONFIRMED`; an expired or cancelled
order (`CANCELLED`) frees its use automatically (nothing to decrement, so expiry/cancel paths are untouched). A refunded or shipped
order keeps counting (a refund is not a reason to reuse a limited code).
Per-buyer limit identity: the buyer capability owner, the sha256 of the lower-cased order e-mail (when given) and of the digits of the
destination phone (always present at Begin). Any match counts. WEAKNESS (documented, not hidden): a buyer who uses a new device AND
a new phone AND a new e-mail is a new buyer; the code is a marketing control, not an entitlement. Hashes are salted with the store id
and stored only for this check. Erasure (customers.apply_erasure → promotions.clear_buyer) clears the e-mail/phone hashes, so a
returning erased buyer restarts their per-buyer limit — privacy is preferred over limit enforcement (total_limit, the aggregate cap,
is unaffected).

Merchant admin (scope from server auth; permission `pricing:read` to list, `pricing:write` to change; same Idempotency-Key receipt
rules as the other settings): `GET /v1/admin/stores/{store_id}/promotions` -> `{promotions:[{id, code, kind, percent|null,
fixed_minor|null, min_subtotal_minor, starts_at|null, ends_at|null, total_limit|null, per_buyer_limit|null, status, version, used,
created_at}]}` (`used` = active usage above); `POST .../promotions` (create, `code` immutable afterwards) ; `POST
.../promotions/{id}` with `expected_version` (edit any of the other fields or `status`); 409 `version_changed` on a stale version,
409 `promo_exists` on a duplicate code, 422 `invalid_promotion` on a rule violation. Pausing never touches placed orders.

Defence in depth: `promotions.redeem` also checks that the order snapshot's frozen effect equals the code row's own terms and that the
discount is possible (1 <= discount <= subtotal, not above the code's own percent or fixed amount); otherwise `promo_changed`. It is a
bound, not a second calculator: the amount is computed only by `internal/pricing`.

Other consumers of the order amounts (verified, unchanged): ECPay shipment for a CARD order declares the PRE-discount merchandise
subtotal as goods value; a pay-at-pickup order collects the order total (discounted).

Money rules unchanged: refunds cap at the CAPTURED (paid) amount, which already equals the order total = the discounted quote total;
Stripe sees one line item equal to that total (stripe-psp-v1 D7), so no Stripe-side discount exists.

## G. Merchant tools (producer: unit merchant-tools, migration 0094; amendment written BEFORE implementation, 2026-10-01)

Three admin-side tools that close the SHOPLINE gaps "overview", "product import/export" and "Create Order in admin".
Base `/v1/admin/stores/{store_id}`; scope from the merchant bearer only (never a request tenant/store); every read is private/no-store.
Permissions use the existing list (0065/0089 bundles); no new permission is introduced.

### G1. Dashboard (`GET dashboard`, `orders:read`; the admin landing `/{locale}`, the Ledger moves to `/{locale}/inventory`)
Days are Asia/Taipei calendar days (UTC+8, same as finance Q11): `today` = the current day, `last_7_days` = today and the six days before.
```json
{"generated_at":"RFC3339","timezone":"Asia/Taipei",
 "orders":{"today":0,"last_7_days":0},
 "gmv":[{"currency":"TWD","environment":"LIVE",
   "today":{"card_minor":0,"bank_transfer_minor":0,"pay_at_pickup_minor":0},
   "last_7_days":{"card_minor":0,"bank_transfer_minor":0,"pay_at_pickup_minor":0}}],
 "todos":{"awaiting_transfer_confirmation":0,"to_ship":0,"cvs_awaiting_label":0,"low_stock_skus":0,"open_refunds":0},
 "latest_orders":[ /* <= 10 merchantorders Summary rows, newest first */ ]}
```
- `orders` counts placed orders created in the window with state AWAITING_PAYMENT, AWAITING_TRANSFER or CONFIRMED (not DRAFT holds, not CANCELLED).
- `gmv` is NOT a second ledger: it is `reporting.Finance` (identity.read_finance_summary, 0078/0085/0088) re-cut to the two windows: card = `captured_minor`, bank transfer = `bank_transfer_confirmed_minor`, pay at pickup = `pickup_collected_minor`; one entry per (currency, environment) present, never summed across currencies or environments (I05).
- `todos`: `awaiting_transfer_confirmation` = bank transfers in SUBMITTED; `to_ship` = the order list's `unshipped` predicate (CONFIRMED, MANUAL_UNASSIGNED, manual_shipment_eligible); `cvs_awaiting_label` = the CVS-destination subset of `to_ship` with no live ECPay shipment attempt; `low_stock_skus` = active SKUs of active products whose available (on_hand - reserved - allocated - unavailable, all warehouses) is <= 5 (a SKU with no balance counts); `open_refunds` = Stripe refunds with no terminal refund fact.
- One SQL definer per block (`identity.dashboard_orders`, `identity.dashboard_todos`, the finance definer); the low-stock block is a plain scoped read under RLS (commerce_runtime already reads catalog and inventory; no new grant). Read-only; <= 300 ms on 10k orders with the existing indexes (merchant_orders_history, orders_unshipped, bank_transfers_open). Measured (REAL_PG, 10,002 orders): the whole GET answers in 39 ms; the orders, finance and latest-10 blocks take 2-5 ms each; `to_ship`/`cvs_awaiting_label` cost ~0.35 ms per unshipped candidate (they reuse the order list's own eligibility predicate, evaluated once), so a typical backlog of ~100 is ~40 ms and a 2,000-order backlog ~0.7 s.

### G2. Product CSV
`GET products/export.csv` (`catalog:read` + `inventory:read`) answers `text/csv; charset=utf-8` with a UTF-8 BOM, CRLF rows, attachment `products-<date>.csv`. One row per ACTIVE variant (SKU); a product without a SKU yields one row with empty SKU columns. Archived products and archived SKUs are not exported. Refused 413-class `export_too_large` above 50,000 rows.
Columns, in this order (import accepts any column order and ignores extra unknown columns that start with `x_`; any other unknown header is a file error):
`handle, title, description, status, option1_name, option1_value, option2_name, option2_value, option3_name, option3_value, sku, price, compare_at_price, stock:<warehouse name> (one per warehouse, name verbatim), collections, image_count`.
- `handle` = product slug; `status` = draft|active|archived; `price`/`compare_at_price` = decimal major units of the store currency (JPY/KRW/VND exponent 0, every other currency exponent 2; the SKU's own minor units are exact); `collections` = collection slugs joined by `|`; `image_count` is informational and ignored by import.
- Product-level fields (`title, description, status, option*_name, collections`) are repeated on every row of the product; on import the first row of a handle is authoritative and later rows of the same handle with a different product-level value are a row error.
`POST products/import/preview` and `POST products/import/commit` (`catalog:write` + `inventory:write`; body = the raw CSV, `Content-Type: text/csv`, <= 2 MiB, <= 1,500 data rows (measured, see below), UTF-8 with or without BOM, comma separator, RFC 4180 quoting).
- Both run the SAME apply code in one transaction; preview always rolls back, commit commits only when there are zero row errors (all-or-nothing). Per row a savepoint isolates its failure so the preview lists EVERY error: `{"row":<1-based data row, header = row 0>,"column":"<header>|\"\"","code":"<code>"}`; codes are `required`, `too_long`, `invalid_value`, `duplicate_sku`, `conflict_product_field`, `slug_taken`, `sku_in_other_product`, `option_mismatch`, `sku_archived`, `unknown_warehouse`, `unknown_collection`, `limit`, `conflict`.
- Match: product by `handle` (slug), SKU by (`sku` code) within the store. Existing product -> patch changed fields (name, description, status, options, with the stored version); existing SKU -> price/compare-at/option values updated through the catalog functions; new product defaults `draft` when `status` is empty; NEVER deletes or archives anything (a row missing from the file leaves its product and SKU untouched). A SKU code found under another product is `sku_in_other_product`.
- Stock: per `stock:<warehouse>` cell (an integer >= 0 = the target on-hand; empty = leave) the importer computes `delta = target - on_hand` and applies it through `inventory.AdjustOnHand` with reason `csv import <first 12 hex of the file sha256>`; it never writes the ledger itself and never goes below reserved (the adjustment path refuses: row error `conflict`).
- Idempotency: the commit is a `merchanttools.csv.import` command keyed by `csv-<file sha256 prefix>`; uploading the same bytes again replays the first summary (`replayed:true`) and changes nothing. Images are NOT imported (`image_count` is read-only); the export is the only place they appear. Collections named in `collections` must already exist (membership is added at the end of the list, never removed).
- Measured ceiling (REAL_PG, MTC04): one transaction at 11-18 ms per product+variant+stock row on an idle machine (1,500 rows = 13.7 s; 2,000 rows = 35 s in an earlier run, 55 s of the 60 s budget under load 38: up to 4x variance seen); 5,000 rows exhausted PostgreSQL's lock table (`out of shared memory`) because every catalog/inventory command holds a transaction advisory lock until commit. Hence `<= 1,500` rows AND a work cap of 6,000 lock-taking commands per file (product create/patch 1, variant create/option update/price 1 each, collection membership 1, stock adjustment 2; rows that change nothing cost 0): a file that crosses it is refused with the `limit` error at the row where it became too large, nothing written (1,500 new products with one stock cell each reach it exactly; about 1,000 with two stock columns cross it). The brief's 5,000 rows is NOT met; split larger files.
- Answer: `{"file_sha256":"<64 hex>","rows":n,"created_products":n,"updated_products":n,"created_skus":n,"updated_skus":n,"stock_adjustments":n,"unchanged_rows":n,"errors":[...],"committed":bool,"replayed":bool}`; `errors` is capped at 200 entries (`errors_truncated` marks a cap). Errors in a preview are a 200; a commit with errors is a 422 whose body is this same result (`committed:false`, nothing written) — not the generic error envelope.

### G3. Manual (merchant-created) order
`GET orders/manual/options` (`inventory:reserve` and `catalog:read`) -> `{"options":[{"option_key","market_id","country","delivery_code","delivery_kind":"home|cvs_*","mode","name_hans","name_hant","name_en","currency","service_version","allocation_version","payment_modes":["bank_transfer","pay_at_pickup","cash_on_delivery"],"pickup_selection":"ecpay_map|buyer_entered|null"}]}` = the buyer checkout options (same `checkout.Service.ListOptions`) with card removed. Amendment home-cod R5 (G-UI8 D01): a home row of a store that enabled cash on delivery lists `cash_on_delivery` and carries the offer as `cod_carrier` (`black_cat|hsinchu`), `cod_max_minor` and, only when non-zero, `cod_surcharge_minor` (whole TWD in minor units, exactly as the buyer option); no other row has any `cod_*` key.
`POST orders/manual` (`inventory:reserve` and `catalog:read`: picking SKUs and the link origin need the catalog read; Idempotency-Key required, JSON, <= 64 KiB, unknown keys at any level rejected, so a price field is a 400):
```json
{"items":[{"sku_id":"uuid","quantity":1}],            // 1..50 lines, quantity 1..1000, distinct sku_id
 "customer":{"name":"<=120","phone":"6..32 digits","email":"<=254|\"\""},
 "delivery":{"option_key":"<from options>","home_address":{"region","city","postal_code","line1","line2"}|null,
             "cvs":{"store_code","store_name","store_address"}|null},   // exactly one of home_address / cvs, matching delivery_kind
 "payment_mode":"bank_transfer|pay_at_pickup|cash_on_delivery","locale":"zh-CN|zh-TW|en"}
```
- Pipeline (nothing is re-implemented): a server-held buyer capability is registered for the store (`buyer.Service.RegisterForTrustedStore`, token = base64url(HMAC-SHA256(deployment BFF key, "merchanttools.manual|"+store+"|"+Idempotency-Key)), TTL 30 days, so a retry derives the same capability and every step replays by its derived key) -> `storefront.SetCart` -> `CreateQuote` (price from the server quote; the request has no price field and a merchant cannot override one) -> `SetDestination` / `checkout.BuyerCVS.EnterStore` -> `checkout.Service.Begin` (the buyer begin path: same stock reservation, expiry job, bank-transfer window, pay-at-pickup rules, buyer email). Card is refused (a merchant cannot take a card for a buyer). Pay at pickup is CVS-only (Begin refuses otherwise, `pay_at_pickup_unavailable`). Cash on delivery is home-only and only on a row that lists it (`cash_on_delivery_unavailable`); Begin applies the whole-TWD total and the per-order cap (`cash_on_delivery_amount_exceeds`, total + fee <= `cod_max_minor`) and the open-order limit (`cash_on_delivery_limit`); the order waits `AWAITING_COLLECTION` with the stock committed, no payment attempt, and the carrier collects total + `cod_surcharge_minor` at the door. A CVS delivery on a store with an ECPay map connection is refused `cvs_entry_unavailable` (the pickup must come from the map there).
- Source: migration 0094 adds `checkout.orders.source` (`storefront` default, `merchant_manual`); `fulfillment.mark_order_merchant_manual(merchant_token_hash, store, order)` (merchant bearer re-checked with `inventory:reserve`, order of the same store created in the last 10 minutes and still `storefront`) sets it, writes the audit row `order.manual_created` in the same transaction as the command receipt `merchanttools.order.manual`. `fulfillment.mark_order_merchant_manual(hash, store, order, link_sha256)` also records the buyer link (below). The merchant orders list and detail carry `source` (`storefront|merchant_manual`, per page through `identity.read_order_sources`, `orders:read`; the older projection definers are untouched; the dashboard's latest orders carry it too) and the Orders page badges `merchant_manual` rows.
- There is NO note field: a free-text note would put buyer PII in a column the customers-privacy export/erasure paths do not cover, and the command receipt keeps only a hash of the request. A note needs its own column plus erasure coverage (follow-up, recorded in output/merchant-tools/DEVIATIONS.md).
- Answer 201: `{"order_id","commercial_state":"AWAITING_TRANSFER|CONFIRMED|AWAITING_COLLECTION","payment_mode","total_minor","currency","expires_at","buyer_link":"https://<origin>/<locale>/order-link#o=<order_id>&t=<link token>"|null,"link_state":"configured|storefront_unavailable|domain_selection_required","source":"merchant_manual"}`. The link carries a single-use LINK TOKEN (32 random bytes, base64url) only in the URL fragment (never sent by a browser, never logged); it is NOT a capability. It is derived by HMAC from the Idempotency-Key (so a replay of the placement returns the same link, dead once used) and stored only as sha256 in `checkout.order_links` (store, owner, order, `expires_at <= created_at + 7 days`, `redeemed_at`).
- Link exchange (buyer): the storefront page `/{locale}/order-link` reads `#o=<order>&t=<token>` client-side, replaces history at once and POSTs `{order_id, token}` ONCE to the storefront BFF `POST /api/buyer/orders/link`, which mints a fresh capability token like `session/prepare`, calls Go `POST /v1/buyer/orders/link` (bearer = the new token) and sets the buyer cookie ONLY on a 200 naming the same order. Go `checkout.redeem_order_link` (commerce_buyer_issuer) throttles (ip 10, order 5, store 200 per 10 minutes, the guest-lookup bucket table), finds the link by (store, order, sha256), marks it used and issues a FULL capability (`view_order_id` NULL, via `buyer.issue_owner_capability`) of the order's owner, so the buyer can pay (bank-transfer proof, pay-at-pickup views). The owner of a manual order belongs to that order alone, so the capability is scoped to exactly that order. Unknown order, wrong token, other store, used, expired and inactive owner are ONE identical `404 not_found` (the page shows one refusal text); a throttle is 429. The browser then replaces itself with `/{locale}/orders/{order_id}`.
- Replay: same key + same body = the first answer (HTTP 200, same fields); same key + other body = 409 `idempotency_conflict`. Errors: 422 `invalid_request`, 409 `conflict`, 409 `insufficient_inventory`, 422 `bank_transfer_unavailable|pay_at_pickup_unavailable|pay_at_pickup_amount_exceeds|cash_on_delivery_unavailable|cash_on_delivery_amount_exceeds|cvs_entry_unavailable`, 429 `cash_on_delivery_limit` (pay at pickup needs a whole-TWD total within the CVS limit, begin_hold decides), 403/401/404 by authority, 503 `retry_later`.
- Audited: `order.manual_created` (principal from the bearer). Money: the total is the server quote (I05); the quote TTL (minutes) bounds the pipeline.

### G acceptance (what the tester proves; each with one red run before its green)
MT01 dashboard numbers equal a hand-built fixture across today / 7 days, three payment modes, the five counters and the 10-row list, and finance reuse (card never includes bank/pickup); MT02 dashboard authority (orders:read, other store 404, no tenant from input) and read-only (zero writes); MT03 export columns/BOM/one row per variant/stock per warehouse/collections/image count, escaped commas, quotes and newlines survive a round trip; MT04 import preview lists every error with its row, writes nothing; MT05 commit is all-or-nothing (one bad row of 100 leaves zero changes); MT06 create vs update by slug+sku, new products draft, nothing deleted; MT07 stock only through the adjustment path with the reason, never below reserved; MT08 same file twice = replay, no second adjustment; MT09 limits (2 MiB, 1,500 rows, the 6,000-command work cap, formula-injection cells are exported with a leading apostrophe and imported back without it); MT10 manual order home + bank transfer / CVS + pay at pickup end to end, stock reserved, AWAITING_TRANSFER window, `source=merchant_manual`, audit row, replay with the same key, other body 409; MT11 manual order authority (inventory:reserve + catalog:read; viewer, live_operator and a reserve-only role are 403) and no price override surface; MT12 admin UI (browser, three locales, 375 px and desktop): dashboard, import wizard (upload, preview with row errors, commit), manual order form; nav keeps Inventory (the ledger).
