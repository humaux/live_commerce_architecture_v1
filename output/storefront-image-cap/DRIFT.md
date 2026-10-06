# Frontend parser / limit vs backend drift audit (2026-10-06)

Scope: every strict response parser and limit in `apps/storefront/lib/*.ts` and `apps/admin/lib/*.ts` (lengths, counts, enums, key sets, number ranges)
compared with Go validation constants / JSON encoders (`internal/**`) and SQL CHECKs (`migrations/`, latest definition wins). Evidence class: DESIGN (static
read) + node unit tests (green.log); nothing here ran against PG or a browser. Direction that matters: the frontend rejecting data the backend may legally emit
(a null parse = "此頁面暫時無法載入" / "unavailable"). A frontend limit looser than the backend is OK.

Verdicts: **DRIFT-FIXED** (red->green test), **DRIFT-OPEN** (reported, not fixed here, owner named), **OK**, **NOTE** (theoretical or accepted).

## Findings (real drift)

| # | field | frontend limit (file:line) | backend limit (file:line) | verdict |
|---|---|---|---|---|
| D1 | product photos in the buyer catalog row used by the **cart page** (`loadCartDetails` -> `catalog?limit=100` -> `validProduct`) | `apps/storefront/lib/purchase.ts:122` was `v.length <= 8` (now `MAX_PRODUCT_IMAGES` = 12) | `internal/catalog/images.go:32` `MaxImagesPerProduct = 12` (migration 0109 widened 8 -> 12); the row carries every photo `internal/buyerhttp/projections.go:103` | **DRIFT-FIXED**. Same root cause as the pilot product-page bug (fbe0d012), second copy of the literal: a 9-photo product in the cart made the whole cart details read fail. Red `output/storefront-image-cap/red.log` (`9 photos`), green `green-full.log`. Test `purchase.test.mjs` "validProduct accepts up to the Go photo cap"; the existing Go-constant parity test now covers both parsers because they share one constant. |
| D2 | design text limits (profile name 60, tagline 120, announcement 140, contact 120/30/200, nav label 30, section heading 80 / subheading 160 / cta 24, markdown bodies 4000 / 2000 / 20000, page title 80) | `apps/storefront/lib/design.ts:56-58` clipped with `String.slice` = UTF-16 units | `internal/design/schema.go:363-514` counts runes (`w.text`, `w.markdown`) | **DRIFT-FIXED**. A valid 100-emoji announcement (100 runes, 200 units) was cut to 70 emoji, and a cut inside a pair left a lone surrogate. Now `clip()` by code point. Red `red.log` (`actual: 70, expected: 100`), test `shop.test.mjs` "design text limits count code points like Go". |
| D3 | claim window `match_mode` | `apps/admin/lib/claims-model.ts:11,77` only `EXACT` / `KEYWORD_QTY_ONLY` | `internal/claims/claims.go:72` also `KEYWORD_QTY_CONTAINS` (migration 0115, merged `44d4c8c7`) | **DRIFT-OPEN (latent)**. `parseWindow` throws for a window whose mode is CONTAINS, so the Studio claims board would be "unavailable" once any client sets it via the API. Not fixed here on purpose: widening the type also needs the mode copy/select/host prompt in `StudioClaims.tsx` + `claims-copy.ts` (3 locales), which is the separate A7 UI unit (`docs/delivery/units/live-a7-contains-match.md`, branch `unit/live-a7-contains`). Integrator: make sure that unit extends `claims-model.ts:11,77` (and `claims-request`/`claims-client` types) in the same commit as the backend mode becomes selectable. |
| D4 | Meta product feed size | `apps/storefront/app/feeds/meta.csv/route.ts:15` `MAX_BYTES = 8 MiB` (over -> failure) | `migrations/0080_meta_capi.sql:352` `ads.feed_rows ... LIMIT 20000` SKU rows, description up to 8000 chars each (`0002:29`) | **NOTE / DRIFT-OPEN (theoretical)**. A store with a few thousand SKUs and long CJK descriptions can exceed 8 MiB and the feed 5xx. Unreachable at pilot scale; needs a decision (cap rows in SQL or raise/stream the route limit), not a silent loosening. |

## Audited and in parity (no action)

| area | field | frontend (file:line) | backend (file:line) | verdict |
|---|---|---|---|---|
| shop-contract | product photos | `shop-contract.ts:99` 12 | `images.go:32` 12; parity test `shop.test.mjs` | OK (fbe0d012) |
| shop-contract | product title / description | `shop-contract.ts:81,104` 300 / 20000 (UTF-16) | `0002:28-29` 120 / 8000 chars | OK (>= 2x slack covers astral) |
| shop-contract | seo title/desc | 200 / 400 | `0086:32-33` 70 / 160 | OK |
| shop-contract | slug | 80, `str` | `0086:47` 80 + regex | OK |
| shop-contract | option axes / values / name / value | 3 / 50 / 60 / 80 | `options.go:28-31` 3 / 50 / 30 / 40 | OK (equal-or-looser, backend counts runes, UTF-16 slack exact 2x) |
| shop-contract | variants per product | 500 | `catalog.go:34` 100 active | OK |
| shop-contract | variant title | `shop-contract.ts:120` 200 | `0086:252` values joined by ` / ` (<= 3 x 40 runes + 6) | NOTE: 246 UTF-16 units only for three all-emoji 40-rune values; not worth a change |
| shop-contract | list page size / `next` | 100 / 2048 | `catalogv2.go:37` `v2MaxLimit = 48`; cursor ~100 chars | OK; new parity test pins `shop-query.ts` limit 48 and q 60 to `v2MaxLimit` / `v2MaxQuery` |
| shop-contract | collections list | 200 | `collections.go:23` `maxCollectionsPerStore = 200` | OK (parity test) |
| shop-contract | collection title / description | 200 / 4000 | `0086:82-83` 80 / 2000 | OK |
| shop-contract | image sizes enum 360/720/1080, `pixel_width <= width`, <= 3 | `shop-contract.ts:56-65` | `image_sizes.go:82,173`, `media.go:40` | OK |
| shop-contract | money | 0..1e12 | `command.MaxMoney`, `pricing/claim_price.go:18` 1e12 | OK |
| shop-contract | stock enum in/low/out | `:121` | `0086:254` | OK |
| shop-query | sort enum newest/price_asc/price_desc/title | `shop-query.ts:6` | `0086:174`, `catalogv2.go:224` | OK |
| purchase | cart items <= 100, qty 1..1e9, live price | `purchase.ts:102-110` | `0007:72` qty 1..1e9; pricing lines <= 50 (`pricing.go:351`) | OK (looser) |
| purchase | commercial states (6) / fulfilment states (5) | `purchase.ts:~280` | `0013:22`, `0018:200`, `0063:48`, `0072:40`, `0088:35`, `0107:38` | OK |
| purchase | payment modes <= 3, transfer window 6..168 | `purchase.ts:143` | `offline.go:129`, `checkout.go:64` | OK |
| purchase | COD max 100..2,000,000 minor, % 100; surcharge 0..100000 | `purchase.ts:166,176` | `0107:108-109` max_twd 1..20000, surcharge_twd 0..1000 (x100) | OK |
| purchase | free shipping threshold > 0 or null | `purchase.ts:173` | `options.go:224` Go maps 0 -> null | OK |
| purchase | tracking number / carrier name / URL / carrier codes | `purchase.ts:339-346`, `purchase-types.ts:181` | `0063:61-62`, `0107:710` | OK |
| purchase | recipient 120, phone, region/city/postal/line1/line2, pickup address | `purchase.ts:~465`, `orders-model.ts:315-326` | `0012:31-36`, `0012:8` (400) | OK |
| cvs-contract | store code per chain, name 1..40, address 5..120, return path | `cvs-contract.ts:187-203,89` | `cvs_rules.go:33`, `cvs.go:386-400`, `0093:18` | OK (new parity test pins name/address/return path) |
| cvs-contract | ECPay map hosts, subtypes, error codes, shipment/collection/selection states | `cvs-contract.ts:40-100,342,399,426` | `0072:298`, collection states `0107` | OK |
| payment-contract | PAYUNi 100..19,999,900 step 100 | `payment-contract.ts:310` | `checkout/payment.go:186` | OK |
| payment-contract | Stripe amount table | `payment-contract.ts:284` | `psp/stripe/amount.go:30-33` | OK |
| payment-contract | commercial/payment/handoff state sets, methods <= 2 | `payment-contract.ts:189-241` | `checkout/payment_view.go:136-196` | OK (exact mirror) |
| payment-contract | PAYUNi form EncryptInfo 16..24576 even | `payment-contract.ts:424` | `checkout/hosted.go:246` | OK |
| payment-contract | Stripe redirect `{1,4000}` | `payment-contract.ts:344` | `checkout/stripe.go:98` | OK |
| promo-contract / promotions-model | code `^[A-Z0-9-]{3,24}$`, percent 1..90, limits 1e9 / 1e6 | `promo-contract.ts:37-40`, `promotions-model.ts:31-48` | `promotions.go:30`, `0091:30-35` | OK |
| claim-contract / claims-model | keyword `^[A-Z0-9]{1,16}$`, qty <= 999, lines/items <= 50/100, offers <= 200, library <= 1000 | `claim-contract.ts:76-88`, `claims-model.ts:70-199` | `0060:53`, `claims.go:61`, `keyword_library.go:26` | OK |
| claims-model | rejected reasons (7 keys), `RATE_LIMITED` | `claims-model.ts:12` | `claims.go:91-98` (`boardReasons` = those 7; RATE_LIMITED never reported) | OK |
| bank-transfer | bank/branch/account <= 60, account regex, email <= 254 | `bank-transfer-contract.ts`, `transfer-model.ts:33-56` | `offline.go:42,129` | OK |
| design | nav 8/12, sections 20, pages 20, accent hex, url families | `design.ts:96,116,174` | `design/schema.go:30-35` | OK (new parity test) |
| admin catalog | name 120, description 8000, SEO 70/160, slug 80, axes 3/30/50/40, variants 100, collection 80/2000/500, SKU code 64 | `catalog-v2-model.ts:92-96` | `catalog.go:27,34,672-676`, `options.go:28-32`, `collections.go:24,124`, `0086:83` | OK (new parity test `tests/admin/backend-parity.test.ts`) |
| admin catalog | photos 12, 2 MiB, key set of an image row | `images-client.ts:11-12,36` | `images.go:30-46` | OK (parity test; mutation 12 -> 8 goes red, `mutation-red.log`) |
| admin catalog | product list `sku_count <= 1000`, product `skus <= 200`, `collection_ids <= 10000` | `catalog-v2-model.ts:143,173-174` | active SKUs only (`productlist.go:130`, `:186`) <= 100 | OK (looser) |
| admin product-document | keyword `[A-Z0-9]{1,16}`, max_per_order 1..999, dimensions <= 1e6 mm | `product-document.ts:145-170` | `0060:53`, `0109:27`, `0002:48-50` | OK |
| admin orders | all orders-model states/enums (commercial 6, fulfilment 5, payment 7, void reasons, refund states/reasons, pickup sources, verification kinds) | `orders-model.ts:10-126,406-408` | `0013`, `0063:65`, `0062:97,1330`, `0072:67` | OK |
| admin orders | tracking number regex, carrier name 80, note 200 | `orders-model.ts:363-390` | `0063:61-62` | OK |
| admin customers | tag name 1..20 + 8 colours, note 1..1000, notes <= 50 in detail, consent history <= 1000, claims <= 500 | `customers-model.ts:16,123-201` | `0139:70-72,624`, `0078:609`, `tags.go:88` | OK |
| admin logistics | pay-at-pickup max 1..20000, open 1..500, goods amount 1..20000, attempts <= 5, shipment states | `logistics-model.ts:125-239,159` | `0072:289-295,339-340`, `0073:1676` | OK |
| admin merchant-tools | CSV 2 MiB / 1500 rows / 200 errors; tracking import 500 rows | `merchant-tools-model.ts:9-10`, `tracking-import-model.ts:5` | `csvfile.go:29-32`, `tracking_csv.go:38` | OK |
| admin cod / transfer | max_twd 1..20000, surcharge 0..1000, window 6..168 | `cod-model.ts:29`, `transfer-model.ts:39` | `0107:108-109`, `offline.go:129` | OK |
| admin team | roles (5), mail states, lists <= 500 | `team-model.ts:8,25-28` | `0089:58,72,81` | OK; permissions are free strings, so new permissions (0119, 0139) cannot break it |
| admin storefront | handle `^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$`, domains <= 100 | `storefront-handle.ts:2` | `0106:32` | OK |
| admin meta-connect / ads | 10-page cap, pick list <= 120 (cap 500), recent refusals <= 20 | `meta-connect-model.ts:37`, `ads-model.ts:155-197` | `metaconnect/doc.go`, `meta_ads/oauth.go:25-28` | OK |
| admin images list key set | `content_type,height,id,position,product_id,size_bytes,version,width` | `images-client.ts:35` | `images.go:37-46` (0111 added no field) | OK |
| storefront cart item key set | `sku_id, quantity[, live_unit_price_minor]` | `purchase.ts:103-110` | `projections.go:115-121` | OK |
| storefront catalog image key set | `id,width,height` | `purchase.ts:~125` | `projections.go:90-94` | OK |
| ad-touch / buyer-server / order-payment / money / lookup / privacy / order-link | request-side cookie, header, host, key-shape limits | see files | Go regexes `ad_touch.go:50-51`, `handler.go:37-39`, `checkout.go:55` | OK (shape-equal; none of these parse a backend list) |

## Out of scope / not verifiable statically

- Unicode table skew: Go `unicode.IsPrint` vs the JS `\p{C}` / `\p{L}\p{M}\p{N}\p{P}\p{S}` classes differ for characters assigned after the older of the two Unicode versions (a brand-new emoji can be printable in Go and "unassigned" in V8). Affects destination text and admin order rows. Not testable deterministically; pin Go and Node Unicode versions together when either is upgraded.
- `ads-model.ts` `str()` default max 256 counts UTF-16 units for Meta-sourced names; no Go limit to compare against (Meta strings), left as is.

## Guard added so the next widening fails in node, not in a pilot

- `apps/storefront/tests/backend-parity.test.mjs` (list limit/query vs `catalogv2.go`, collections cap, design caps vs `schema.go`, CVS store bounds + return path vs `cvs.go` / migration 0093).
- `tests/admin/backend-parity.test.ts` (catalog limits, photo caps, SKU code pattern vs Go/SQL); registered in `scripts/dev/test-node.sh`. Injected fault (MAX_PHOTOS 12 -> 8) goes red: `mutation-red.log`.
