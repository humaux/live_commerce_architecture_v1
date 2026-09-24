# Buyer purchase surface — composition proposal

Status: PROPOSED, not approved, not a shipping UI. Source baseline `84c1169`.

## Audit before design

- `apps/storefront` currently has the accepted public API transport and client
  session coordinator, but no buyer page, product share route or approved comp.
- Existing merchant ledger/onboarding/settings are visual authority only. Do not
  transplant admin navigation, account-key forms or seven repeated settings cards
  into a mobile purchase journey. Reuse the navy/teal type, control and state family.
- Current buyer catalog explicitly provides product name/description, SKU code,
  currency and price; it has no media field, option-name map, stock guarantee or
  verified customer reviews. The first design uses those real fields. Product
  photography is not replaced with an invented API field or fabricated photo.
- Mobile social-link visitors need to confirm the product, SKU and quantity and
  then choose an eligible delivery method before reviewing the authoritative quote.
  The quantity/price shown here are synthetic demonstration values, not a quote.
- No live customer backend, production PSP configuration or funds are touched.

## Fixed scope and visual authority

Mode: Operate. New whole surface in the established world, not a brand redesign.
Mobile-first 390 × 780 logical viewport; each generated comp is a single portrait
first viewport, without a phone frame. Desktop must later adapt the approved
composition, not compress the admin table. Three free-choice locales: zh-CN,
zh-TW, en. Comps show zh-TW; locale switching must not change currency or identity.

Reference: `.impeccable/review/merchant-settings/mobile.png` (inspected). Palette,
type and control character carry over; merchant settings content does not.
Light surface is chosen for mobile shoppers reading product and payment details
under changing ambient light. No dark-mode identity exercise or marketing claims.

## Seven viable structures, ordered by task fit before dealing

1. **分步購買** — compact product summary remains visible; SKU/quantity, delivery,
   then review/payment are three explicit stages. Low field burden; more taps.
2. **購物清單優先** — editable order line and quantity lead, descriptions expand
   on demand; useful for repeat buyers, less explanation for first-time buyers.
3. **商品詳情直選** — description, price and SKU choices form one continuous page,
   with a sticky subtotal/continue bar. Familiar, but longer pages need care.
4. **一頁核對表** — product, delivery and quote are named anchored sections on one
   page, with completed sections collapsed. Easy backtracking; denser mobile form.
5. **商品摘要＋底部選購區** — read the product above a persistent lower purchase
   region; SKU and quantity stay next to the action. Clear thumb reach; less room
   for long descriptions. This is a nonmodal region, not forced popup navigation.
6. **目錄加購** — share-target product leads a bounded product list with quick SKU
   selection and a basket summary. Useful for multiple items; broader than one-item intent.
7. **規格訂購表** — one product with SKU rows and explicit per-row quantities;
   aggregate basket leads to delivery. Useful for multiple variants, less focused
   for the common one-SKU purchase. No invented option matrix/variant metadata.

Seed `baaadec1`, scope `surface`, mode `operate`, count 7: dealt **5, 3, 1**.
The first invocation with `--help` actually emitted an unscoped seed; it was not
used for a new world. The required scoped invocation reproduced that same key
and candidate count. No taste-based reroll was made. The three user choices are
A lower purchase region, B inline product detail, C step-by-step purchase.

## Shared demonstration content and data boundaries

Visible label: `示意資料 · 不會下單`.
Store: `示例商店`. Product: `帆布收納袋（兩入組）`.
Description: `一組兩入，方便分類收納日常小物。`.
SKU codes: `NAVY-01`, `SAND-01`. Price: `NT$390` (synthetic TWD390).
Quantity: 1. Action: `選擇配送` (C: `下一步：配送`).
Subtotal: `商品小計 NT$390`; `運費於下一步確認`.
No promise of stock, live checkout, supported logistics, authentication,
certification, sale price, payment success, customer count or actual brand.

The production flow must use catalog discovery, cart, Quote and checkout, not
hardcode this sample data. No payment can begin directly from this sketch.
Unavailable methods/PSP URLs must remain explicitly unavailable. Crawler GET
must not initialize a purchase, hold inventory or call a provider.

## Approval and implementation gate

`buildPath=comp` remains the user's confirmed preference. Decision payload:
`.impeccable/buyer-surface-options.json`. Each PNG and exact prompt sidecar is
unapproved until a recorded user selection. Do not implement a buyer visual page
from a merely generated comp. Backend/link-contract work may continue meanwhile.

All three images were generated with built-in `image_gen`, inspected, copied into
the project, and given embedded exact prompts. Actual dimensions are **887×1774**
for every image (same 1:2 aspect, equivalent to the 390×780 logical composition).
`embed-prompt --scan` reports **3 rasters, 0 missing**. They are composition
proposals, not browser acceptance screenshots. Decision page key `16c33590` was
served and opened; `--wait` returned exit 4 (page closed without an answer).
The three images were therefore presented inline and the structured question
was sent as the documented fallback. **Approval is still pending.**

After approval: record selection and sidecar approval; implement semantic controls,
all three locales, responsive desktop/mobile; prove matched comp dimensions and
real product-to-cart-to-quote-to-order flow. Provider payment, DNS/TLS and CVS
remain separately measured gates, never inferred from a visual design.
