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
