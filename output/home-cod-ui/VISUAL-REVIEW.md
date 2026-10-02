# Scoped finish review

Operate/refinement using incumbent DESIGN.md and approved references 04/09; no new visual system. Independent read-only reviewer inspected PRODUCT.md, DESIGN.md, SURFACE.md, reference README and 12 required captures. No separate QUALITY BAR card was supplied; DESIGN.md was the quality baseline.

## Round 1: fix

- P1: Pending/confirmation order subtotal NT$25 had stronger emphasis than NT$75 actually collected by the carrier (includes NT$50 COD fee).
- P2: English 390px footer squeezed the numeric amount, leaving only NT$ visible.
- Keep: navy/teal incumbent shell, explicit distinction between order amount and carrier collection, manual-shipping wording.

## Repairs

- All quoted/order totals now explicitly labeled Order total / 訂單金額 / 订单金额.
- Shared COD amount markup: primary 28px collected total, labeled due/original amount according to state, subordinate included-fee breakdown. COD order subtotal 16px; quoted footer amount 20px.
- Footer uses bounded two-column grid; numeric amount does not wrap. Existing CTA remains accessible and no navigation or order authority changed.
- Added browser assertions: complete numeric text is inside its container and viewport; collected amount has stronger typography than subtotal/footer at 390/1366/1586 in the captured locales.

## Round 2

Independent disposition: **ship** for both scored findings. All 12 refreshed required captures reviewed; no new material visual regression found. Primary carrier amount and secondary fee are distinct, and English 390px footer shows the full NT$25 order amount. Bounded visual verdict only, not production/release acceptance.

Humaux record: `home-cod-ui refreshed visual verdict: COD hierarchy and mobile total resolved` — `42293e7b-e2d9-45aa-99e0-655c701f7a5b`.

Root also opened the final English 390px confirmation and zh-TW 390px pending order captures. All 30 PNG headers match their declared CSS dimensions. Browser-home-cod regenerated the captures and passed the new geometry/hierarchy assertions at fixture `20261002T124031.220115000`.
