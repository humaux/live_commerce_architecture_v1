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
- `GET catalog/v2/collections` → `{collections:[{slug, title, image_id|null, product_count}]}` (active only, count of active products).
- `GET catalog/v2/collections/{slug}` → `{slug, title, description, image_id|null}` (products via the list route with `collection=`).
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
- Buyer email (optional, validated, ≤ 254) captured at checkout and stored on the order for
  notifications (PII: erasure/export paths of customers-privacy must include it).

## D. Staff (producer: unit staff-team, migration 0089)

Roles = fixed permission bundles over the existing permission list (0065): `owner` (all), `admin`
(all but staff management and billing), `live_operator` (live:*, catalog:read, orders:read,
inventory:read), `fulfilment` (orders:read, fulfillment:write, orders:export, inventory:*), `viewer`
(all :read). Owner invites by email (existing mail sender; one-time token, 72 h, single use, bound to
store + role), invitee signs up / logs in with password auth and accepts; owner can change role or
revoke (immediate: sessions of that membership stop authorizing on next request). At least one owner
always remains. All actions audited.

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
(owner = all 25 permissions incl. billing:manage; admin = owner minus billing:manage; staff management = the owner role row itself).
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
and stored only for this check.

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
