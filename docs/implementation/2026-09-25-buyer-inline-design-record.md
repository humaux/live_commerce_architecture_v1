# Approved B buyer inline surface — built design record

Recorded from source baseline `ec926ae` on 2026-09-25. Mode: Operate.
This records the implemented product/SKU/quantity → cart → delivery → quotation
surface. Address confirmation, order creation UI and payment are outside this
implementation; this document is not full checkout or SaaS release acceptance.

## Authority and preserved system

The user approved **B 商品详情直接选购**. The [surface proposal](2026-09-25-buyer-surface-proposal.md)
and approved [composition sidecar](../../.impeccable/mocks/decision/buyer-inline-detail.png.json)
hold that decision. `PRODUCT.md` supplies product boundaries; root `DESIGN.md`
and `.impeccable/design.json` remain the incumbent merchant-ledger design system.

No new global token, shared component API or replacement visual world is needed.
Both global files are left byte-for-byte unchanged: the implementation reuses
their control family, while its purchase composition and sizes belong to this
surface. The merchant list/tray rules, merchant navigation, status chips and
settings guidance are not rewritten as buyer rules. No qualitative interview,
new comp or universal buyer-layout prohibition was introduced.

## Implemented visual facts

Source: [`globals.css`](../../apps/storefront/app/globals.css),
[`ProductPurchase.tsx`](../../apps/storefront/components/ProductPurchase.tsx) and
[`purchase-copy.ts`](../../apps/storefront/lib/purchase-copy.ts).

| Reused incumbent primitive | Storefront use |
| --- | --- |
| `navigation-navy`, `ledger-ink`, `muted-text`, `ledger-line` | Titles, readable text, supporting copy and fine dividers; CSS aliases are `--navy`, `--ink`, `--muted`, `--line`. |
| `action-teal` and `action-teal-hover` | Price, primary action and hover; `--teal` also colors carets and native selected radios. |
| `focus-blue`, `selection-mint`, `surface` | 3px visible focus with 3px offset, text selection and white working surface. |
| Existing Arial / PingFang SC / Microsoft YaHei stack | Operation-oriented text and tabular amounts; no new display family. |
| 5px control corners and flat depth | Buttons, selects and SKU rows; structure comes from 1px rules and spacing, with no shadow. |

The CSS values above match the incumbent frontmatter. Reuse describes actual
color/control values; it does not make the storefront's teal radio accent or
left-aligned unit price a revision of merchant selection/alignment guidance.

The following measurements are local implementation facts, not additions to the
global type, spacing or component scales:

- A 54px store/locale header precedes one continuous inline column. The main
  border-box is at most 760px with 21px horizontal padding; the footer contents
  are at most 718px. There is no admin rail, product-image block, modal purchase
  sheet or progress wizard in this surface.
- Body text is 16px; the product title is 28px/700 at 1.35 line height and 36px
  from 800px viewport width. Unit price is 32px/700. Choice/quantity labels are
  20px/700; quotation headings are 22px; footer amount is 24px.
- SKU choices are labeled native radio rows, at least 56px tall (60px from
  800px), separated by 10px. Selected rows use local pale blue fill/border;
  these are not the merchant `selected-row` token and are not promoted to it.
- Quantity is a labeled integer input between semantic minus/plus SVG buttons.
  The stepper is 180px × 46px. Below 360px it narrows to 154px, page/footer
  horizontal padding becomes 16px, and the primary label becomes 16px.
- The fixed white footer uses a fine top rule, 18px minimum bottom padding with
  safe-area support and a 58px minimum primary button. Main content reserves
  160px below it. Delivery and quotation continue the same column, separated
  by fine rules; their amounts use tabular figures and right alignment.
- Loading, invalid quantity, empty delivery, errors, pending recovery and unpaid
  quote states have text. Locale choices are zh-CN, zh-TW and en; displayed
  currency comes from the catalog or quote rather than the language selector.

## Actual footer and transaction boundary

| State | Summary and primary action |
| --- | --- |
| SKU/quantity selection | Selected line subtotal and delivery-next qualifier; **Choose delivery** writes the cart selection and opens eligible methods. Other existing cart SKUs are preserved. |
| Delivery selection | Selected line subtotal remains an estimate; **Get current total** requests the server quotation. No eligible method has explanatory text; non-home methods cannot advance to quotation in this UI. |
| Quote present or recovered | Server `quote.amount.total_minor` and `quote.currency`, **Not paid**, **View quotation**. The action scrolls to the quote; a separate **Change delivery** action reopens choices. |

The quotation lists its lines, shipping, tax, discount, total and expiry, followed
by an explicit no-payment/address-unavailable message. This documents the built
scope even where the earlier proposal described a future quote-to-order flow.

## Demonstration and asset truth

`buyerDemoLabel` in [`demo-label.ts`](../../apps/storefront/lib/demo-label.ts)
requires both `COMMERCE_BUYER_DEMO_LABEL=1` and a private API origin matching
`http://127.0.0.1:<port>` exactly. It is off by default and false for public API
origins. The label is **Synthetic data · test environment** / **示意資料 · 測試環境**
(with a zh-CN equivalent), not the comp's earlier “不會下單” statement. It is a
disclosure, not an authorization or billing bypass.

The fixture renders USD from its API, despite the comp's synthetic TWD price.
The header uses a generic localized storefront label; no reviewed shop-identity
projection or customer brand is invented. Product names, descriptions and SKU
codes remain API content. No product media field, stock promise, review rating
or shipping raster was added. PNGs are evidence, not page backgrounds.

## Verification and handoff

- Reused token values were compared against the actual storefront CSS and root
  frontmatter. The shipped Impeccable parser reads all eight canonical sections;
  sidecar JSON/schema-version, component reference/property shape and narrative
  identity checks pass. This is a bounded local shape check, not an invocation
  of the external Stitch linter.
- Existing [mobile](../../.impeccable/review/buyer-inline/mobile.png) and
  [desktop](../../.impeccable/review/buyer-inline/desktop.png) captures were
  inspected. Mobile/hero are the same 887×1774 PNG bytes; desktop is 3275×2597.
  Mobile shows the synthetic notice; desktop shows Total $819.53, Not paid and
  View quotation. Those are fixture observations, not merchant prices.
- The independent finish verdict `1f916eca-9755-4ce3-beb5-781eb39cfb01` resolved
  its two scored fixes (demo disclosure and quote footer), with `ship` limited
  to that scope. The [progress record](2026-09-25-buyer-inline-progress.md)
  identifies `browser-3.log` (13 scenarios); those gates were not rerun by this
  documenter. No browser, detector or new visual acceptance claim is added.
- Documentation gate: `git diff --check` passes; only this record changes.
  Global SHA-256 values remain `54a5b00dc6c32dc8e596c93de96c79e873f85024c317446b0f002387156b57b8`
  (`DESIGN.md`) and `6ec2d6e5a409354142e479777f4407ef202d36e719c486b9685c3688cfc2c5fb`
  (`.impeccable/design.json`). No runtime behavior, dependency, token or comp changes.

Not canonized: buyer-only sizing/colors, synthetic prices/content, the comp's
no-order promise and any implication of completed address/order/payment UI.
Their evidence and authority do not establish global design-system rules.

Author boundary: `impeccable_documenter`, isolated branch
`commerce/buyer-design-record-20260925`, base `ec926ae`, worktree
`/Volumes/data/live-commerce-worktrees/buyer-design-record-20260925`.
The agent inherits the caller configuration; the exact backend model identifier
and reasoning setting are not exposed by this tool. The integrator owns independent review
and merge; this documentation task creates no servers, fixtures or temp assets.
