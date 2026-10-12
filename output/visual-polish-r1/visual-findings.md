# Visual acceptance review — live-commerce SaaS (admin + storefront + platform)

Review basis: static Playwright screenshots in `visual-shots/` (zh-TW desktop 1586x992 + zh-TW mobile 390x844 opened for every page; zh-CN/en opened where text length could break layout). `lint.md` latest run: PASS, 0 blocking (R7 clipped-text 71 warns, R5 tap-target 13 warns — lint findings are not repeated here). Judgements below are what the lint cannot see. All content is synthetic/MOCK fixture data; no live-commerce verdict is implied.

VERDICT: PASS

No P1 found. All 57 pages are usable, correctly localized (zh-TW primary), money is TWD, and mobile degrades sensibly. P2 items below are visible polish, not acceptance blockers.

## admin ads: FIX
- [P2] zh-TW-desktop (and zh-TW-mobile): 成效 (Results) money metrics render as bare "$0" (已付款 $0 / 已退款 $0 / 淨額 $0 / 花費 $0 / 購買金額 $0) while every other admin/storefront surface uses "NT$…" — and the en locale of this same page correctly shows "NT$0". The zh-TW metric values appear to bypass the shared money formatter. Fix: route the 成效 metric values through the same NT$ money component used everywhere else.

## admin ads-attribution: PASS

## admin billing: PASS

## admin collections: PASS

## admin customers: PASS

## admin customers-customer: PASS

## admin customers-import: PASS

## admin design: PASS

## admin finance: PASS

## admin finance-reports: PASS

## admin home: PASS

## admin inventory: PASS

## admin invite-token: PASS

## admin messages: PASS

## admin orders: PASS

## admin orders-cvs-print: PASS

## admin orders-new: PASS

## admin products: PASS

## admin products-import: PASS

## admin products-new: FIX
- [P2] (uncertain) zh-TW-mobile: the full-page capture ends abruptly mid-form (~929px tall, at the sticky 儲存草稿/上架 bar) — the 詳情圖 section, 基本資訊 card and variant area below the 圖片規格 select are absent from the capture, so the lower half of the form could not be visually verified in zh-TW. The en capture of the sibling page (products-product) shows the full form rendering correctly, so this is likely a capture/inner-scroll artifact rather than a real defect — but please verify on a real device that the zh-TW mobile form scrolls to and submits the lower sections.
- [P2] zh-TW-mobile: the anchor tab bar (商品圖片 / 基本資訊 / 規格變體 / 搜尋) clips the last tab ("搜尋" is cut at the right edge) with no scroll affordance. On admin orders mobile, overflowing tabs carry a 「向左捲動・向右捲動」 hint; the same hinted-scroll pattern (or fitting the labels) should apply here.

## admin products-new-variants: FIX
- [P2] (uncertain) zh-TW-mobile: same truncated capture as products-new — content below the anchor tabs is not in the shot; verify scroll reachability on device.
- [P2] zh-TW-mobile: the sticky save bar wraps 「尚未儲存的修改」 awkwardly (label breaks mid-phrase next to the buttons). Consider shortening to 「未儲存變更」 or giving the label its own line on 390px.

## admin products-product: FIX
- [P2] (uncertain) zh-TW-mobile: same truncated capture as products-new — the en mobile capture of this page shows the complete form (Main images → Save changes / Unpublish / Publish → footer), so the layout itself is sound; the zh-TW capture just stops mid-form. Verify on device.
- [P2] zh-TW-mobile: same clipped last anchor tab as products-new (no scroll hint).

## admin promotions: PASS

## admin returns: PASS
- See cross-page consistency note on filter control style (pill buttons vs underline tabs).

## admin reset: PASS

## admin settings: PASS

## admin settings-operations: PASS

## admin settings-payments-card: PASS

## admin settings-settlements: PASS

## admin signed-out-home: PASS

## admin signup: PASS

## admin studio: PASS

## admin studio-claims: PASS

## admin studio-console: PASS

## admin team: PASS

## storefront cart: PASS

## storefront checkout: PASS

## storefront claim: PASS

## storefront collections: PASS

## storefront collections-slug: PASS

## storefront data-deletion: PASS

## storefront home: PASS

## storefront legal-anti-fraud: PASS

## storefront legal-slug: PASS

## storefront order-link: PASS

## storefront orders-lookup: PASS

## storefront orders-orderid: PASS

## storefront pages-slug: PASS

## storefront privacy: PASS

## storefront products: PASS

## storefront products-slug: PASS

## storefront search: PASS

## platform contact: PASS

## platform data-deletion: PASS

## platform home: PASS

## platform privacy: PASS

## platform terms: PASS

## Cross-page consistency
- [P2] (uncertain) Filter control idiom differs between list pages: admin orders / finance-reports / design use underline tabs, while admin returns uses pill filter buttons. Both are internally consistent and clearly labeled, so this is not a wrong-product page — but if one idiom is intended as the list-filter standard, returns should converge on it (or the divergence should be documented as intentional).
- [P2] admin ads 成效 metrics drop the NT$ prefix in zh-TW only (see admin ads) — every other page in all three apps formats TWD as NT$n; one shared money component should own this.
- Mobile scrollable tab bars are inconsistent in affordance: admin orders mobile shows the 「向左捲動・向右捲動」 hint, the product-form anchor tabs (products-new / products-new-variants / products-product) clip without any hint. Same component family should share the affordance.
- Everything else is consistent: status badge palette (gray 未付款/未設定, amber 待轉帳/待貨到收款, green 已確認/上架中/啟用中), green primary buttons, breadcrumb + title + description page pattern in admin, NT$ money everywhere except the ads finding, zh-TW with no leaked untranslated keys or mixed simplified/traditional characters on any opened page, and the storefront/platform each read as one coherent product (platform marketing site is deliberately a different skin — orange CTA, dark footer — and is internally consistent).
