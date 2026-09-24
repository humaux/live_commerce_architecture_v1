# Approved B buyer route — implementation progress

Status: IN PROGRESS, not whole checkout or release acceptance.

The user chose **B 商品详情直接选购**. Source comp/approval:
[surface brief](2026-09-25-buyer-surface-proposal.md). Do not restore C or ask
for the composition choice again. Work is local/isolated; no customer systems,
live accounts, payment links or funds have been changed.

## Implemented boundaries

- `apps/storefront/app/[locale]/products/[productID]/page.tsx`: exact locale and
  UUID route; server GET renders a shell only. Existing browser coordinator
  activates the HttpOnly buyer session and loads the real private catalog.
- `ProductPurchase.tsx`: approved inline hierarchy in three locales; paged SKU
  discovery, integer quantity, preserved unrelated cart SKUs, eligible delivery
  choice and server quotation. Currency comes from the store/API, not locale.
  No stock guarantee or invented product images/reviews.
- `lib/purchase.ts`: reuse `buyerRequest`; native Web Lock plus bounded journal
  stores only exact cart/quote key/body, scoped by non-bearer session context.
  Unknown responses block new writes; retry reuses the same key. Historical cart
  receipts are followed by a current cart GET. Results recheck the session;
  context/epoch guards reject late UI updates.
- Draft SKU/quantity is per context/product. Language changes preserve it;
  a saved SKU beyond the first page is found along its cursors rather than
  silently purchasing a different variant. Removed choices need explicit review.
- [Current destination recovery](../../contracts/buyer-current-destination-v1.md)
  is an owned read, including an expired snapshot's head version. It does not
  prove a lost command committed or make an address eligible for checkout.
- Admin purchase-entry UI is integrated at `a14f38d` (author `c483a16`): original
  catalog receipts remain separate; lookup failure does not re-submit a write;
  copy/open refresh their configured URL. Admin and joint browser gate remain
  pending, so this is not release authorization.

No new external dependency. Storefront consumes existing workspace
`@live-commerce/i18n`; Next/React versions and BFF authority are unchanged.

## Evidence, accurately bounded

Evidence root: `/Volumes/data/output/live-commerce-buyer-inline-tests/`.

| Check | Evidence/result |
| --- | --- |
| Current destination PG/HTTP | `current-destination-pg-5.log`, exit 0; 23 buyer HTTP tests; explicit destination head/snapshot/event/receipt counters unchanged by current read. |
| Frontend helpers/transport | Node `--test --experimental-strip-types apps/storefront/tests/*.test.mjs`, 13 PASS; typecheck exit 0. |
| Actual production UI and transport | `browser-3.log`, exit 0; 13 scenarios; Chromium → two production Next instances → Go → isolated PostgreSQL. |
| Actual UI business extent | Select SKU/quantity → cart → delivery → quote; 3 locales preserve quantity/currency/session; reload restores quote. No UI order/payment claim. |
| Cross-tab recovery | Cart commits with responses dropped; second tab resets and activates a new session; old tab adopts it without revoking it or becoming stuck on its old journal. |
| Existing transport regression | Original catalog/cart/quote/destination/order flow still yields exactly one DRAFT order and hold across replay; **test-driven transport flow**, not an implemented order UI. |
| Visual captures | `.impeccable/review/buyer-inline/{hero-repro,mobile,desktop}.png`; hero 887×1774 at logical 390×780, actual synthetic database content. Desktop logical 1440×900. Independent verdict: both listed material fixes resolved. |
| Detector | `detector.json`; one Arial overused-font warning. Arial/PingFang is explicitly retained incumbent typography, not a new font choice. |
| Full Go race/vet | `full-regression.log`, exit 0; 376 top-level PASS, 0 FAIL/0 SKIP; foundation 152.198 s. No production writes. |

Early failed runs are retained: the added test first expected query 400/422,
where the private boundary correctly denies it with 403. BFF query rejection is
422. A test lock stub was corrected for the existing three-argument Web Locks
call. `current-destination-pg-4.log` exposed a synthetic expiry setup using two
volatile clocks: microsecond drift could exceed the database's 30-minute TTL.
The test now uses one statement timestamp; the application and database
constraint are unchanged. `pg-5` includes the added counter assertion. None of
the early failed runs counts as a pass.

Independent source reviews found five P2s and one follow-up stale closure; fixes
cover country in delivery identity, aggregate tax, safe active-session adoption,
paged/context-scoped drafts, late-response validation and pending recovery.
Final targeted source/causal browser closure memory:
`52105e98-173b-4072-8c6c-f8218fc063b2`. This closes the findings, not the entire
checkout, visual or provider gates.

The comp's TWD price and example shop are not production facts. Actual fixture
is USD; rendering must not change it to TWD to match a
picture. SKU order follows UUID-keyset discovery. The brief records omission of
a hamburger with no destination and a generic shop label until identity is
exposed by a reviewed projection. No shipping raster assets exist;
screenshots/comps are evidence, not page backgrounds.

The synthetic banner requires both explicit `COMMERCE_BUYER_DEMO_LABEL=1` and
a loopback private API origin; it is off by default and cannot label a normal
public merchant connection as a demo. This is disclosure only, not authorization.
After quote recovery, the footer displays the actual total, `Not paid` and
`View quotation`; a separate action changes delivery. The browser proves those
states before and after reload. The independent reviewer scored both listed
fixes resolved, `disposition: ship` at that bounded scope, memory
`1f916eca-9755-4ce3-beb5-781eb39cfb01`. This is not a whole-checkout approval.
Built-surface documentation is the next finish step.

## Remaining gates / next implementation

1. Record the built surface's design documentation (visual fix verdict closed).
2. Admin browser and saved product/SKU → actual public URL joint gate.
3. Buyer address/current-head confirmation, DRAFT order UI and exact recovery.
4. Per-order hosted checkout from authoritative total, merchant account and
   environment/currency, webhook/expiry/stock lifecycle and provider sandbox gate.
5. Trusted CVS/carrier acceptance, real publication DNS/TLS, deployment,
   performance/restore and all SaaS release gates.

Address and payment are explicitly not open on this page yet. This is an
increment toward the active SaaS goal, not a replacement or reduced goal.
