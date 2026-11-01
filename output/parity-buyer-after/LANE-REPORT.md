# PARITY-BUYER-AFTER round 2: implementation lane report

Nothing was run (owner rule: CI only). RED results read from run 41240008680, job `gate (--browser-buyer-after-parity)` (124652891808): 18 order-status cases, 2 cancel-payment, 2 payment hand-off, 2 terminal-order, lookup, order-link, payment-return, 2 privacy, data-deletion and claim cases all failed on `missing text / missing control / legacy ... still shown` assertions (the expected reason, not compile).

Class: a = product did not match the signed frame (fixed in product), b = existing test asserted the pre-Figma UI (rewritten to the frame, same intent), c = RED spec fixture problem.

| Failing job / test or finding | Class | What changed (file) |
|---|---|---|
| `order status · card-*`, `pickup-*`, `cod-*` (18 cases): old 「你的訂單 / 訂單金額」, full UUID row, no 商品明細 / 收件與配送 | a | New `apps/storefront/components/order-status/{OrderStatusView.tsx,OrderTerminalView.tsx,copy.ts,format.ts,order-status.css}` (frame layout, mail order number `XXXX-XXXX-XXXX`, masked phone, pickup + COD blocks reuse `cvs-copy` / `cod-copy` wording); `app/[locale]/orders/[orderID]/OrderPage.tsx` mounts it instead of `OrderDetails` (checkout / history keep `OrderDetails`) |
| `cancel payment · home/cvs`, `payment hand-off page · home/cvs` | a | `components/OrderPayment.tsx` gains optional `renderFrame` + `onSubmitted` (unchanged state machine, `PaymentFrame` type); `OrderStatusView.tsx` draws 完成付款 / 付款 / cancelling note / 前往付款 screen (`payment-handoff`), native confirm kept |
| `terminal order view · completed/cancelled` | a | `OrderTerminalView.tsx`; cancelled-card probe reads the payment view once (NOT_STARTED = finished view, CLOSED_UNPAID keeps the 付款已關閉 frame) |
| `order lookup · default and not-found` | a | `app/[locale]/orders/lookup/LookupForm.tsx`, `lib/lookup-copy.ts` (+ heading, introDefault, submitMiss, footNote, kept) |
| `order link · expired link page` | a | `components/OrderLink.tsx`, `lib/order-link-copy.ts` (+ refusedTitle/Sub/Body/Go); `order-link-refused` testid kept |
| `payment return · neutral page` | a | `lib/payment-return.ts`: the zh-CN section now carries the frame's own first Chinese string (Traditional characters, Mainland wording, `lang="zh-CN"` kept for `payment-return.test.mjs`); the Simplified section is gone |
| `privacy · overview, consent settings, export` / `erase confirm, completed, blocked and unknown` | a | `components/PrivacyCenter.tsx` rewritten as overview / settings / export done / export failed / confirm / blocked / unconfirmed screens; copy in new `components/privacy/copy.ts`; `components/privacy/ContactStoreButton.tsx` |
| `data deletion instructions` | a | `app/[locale]/data-deletion/page.tsx` |
| `claim link · confirm, expired and unconfirmed result` | a | `components/ClaimLink.tsx`, `lib/claim-copy.ts` (+ confirmTitle, fromComments, noPay, mergedNote, subtotal, viewProduct, checkoutAll, goneTitle/Body/Note, viewCart, unknownTitle/Badge/Body/Note, reread) |
| RED fixture: claim preview line had `pending: false` while the cart lacks the SKU, which the product (and `claim.test.mjs`) treats as "nothing to check out" (button disabled, so the 認領結果待確認 state is unreachable) | c | `tests/storefront/buyer-after-parity.mjs`: `pending: true` in the claim-link mock only, no assertion edited |
| Shared setup paths that clicked old copy / testids | b | `tests/storefront/claim-checkout.mjs` (checkout label regex, merge notice, back-nav text), `tests/storefront/privacy-buyer.mjs` and `tests/storefront/ads-consent.mjs` (settings screen, done / blocked screens), `tests/storefront/buyer-comms-gate.mjs` and `tests/storefront/ads-attribution.mjs` (`order-id` full UUID -> mail order number heading), `tests/storefront/home-cod-buyer.mjs` (order-page H1), `tests/e2e/{deal-loop,live-tools}.spec.ts` (`claim.confirmTitle`) |
| Browser ownership | - | `output/ci-select-backend-browser/browser-ownership.json` regenerated (derive, then `--check`) |

## Legacy removed (assert gone in the RED spec)
Order page: 「你的訂單」, 「訂單金額」, 「前往新分頁付款」, the full-UUID row (`order-id`). Lookup: session note, the old intro in the default state. Privacy: heading 「你的隱私」, per-purpose toggle buttons, the native `<dialog>`. Claim: heading 「你登記的商品」, 「直接結帳」. Payment return: Simplified section. Data deletion: the numbered Meta steps.

## Mapping of rewritten pre-Figma test cases (same intent, nothing deleted)
- `claim-checkout.mjs`: 直接結帳 -> 前往結帳（共 N 件）; 其他商品也會一起結帳 (claim-merge-notice) -> 共 N 件一起結帳; 件可買 -> frame label; 重新開啟商家私訊中的連結 -> 請私訊商家重新取得.
- `privacy-buyer.mjs` / `ads-consent.mjs`: toggle button per purpose -> 管理這兩項選擇 -> checkbox -> 儲存選擇 (one PUT per changed purpose, same context `settings`, own key); download message -> 資料副本已下載 screen; erase `<dialog>` + Escape -> confirm screen + 取消; eraseBlocked text -> blocked screen.
- `buyer-comms-gate.mjs`, `ads-attribution.mjs`: `getByTestId("order-id")` full UUID -> level-1 heading contains the mail order number (URL still asserts the UUID).

## Decisions and deviations (for the integrator)
1. COD PENDING label is the frame's 「到貨需付」 (`codCopy.due`); `home-cod-buyer.mjs` lines ~393-394 and `cod-footer.spec.ts` still assert 「到貨時要付的金額」 on the CHECKOUT page (OrderDetails, unchanged by this lane), so they stay valid there. The frame's 「包裹送達時向貨運公司付款」 is NOT used: `cod-footer.spec.ts` asserts 「向貨運公司」 gone from `codCopy` (owner-approved OC-09a); the PENDING state line stays 「包裹送達時，付現金給物流人員」. Needs an integrator ruling if the frame wording should win.
2. 重新查詢付款狀態 is drawn only in the awaiting frame; the pending / cancelling states also get the same secondary button (`refresh-order`) because `shop-real-gate.mjs` drives its unknown-payment recovery through it and the frames' 「重新整理此訂單查詢」 needs a control.
3. 聯絡商店 / 聯絡店鋪 / 申請售後 scroll to the store footer contact block (no contact route exists); the 申請售後 destination (frames 26:773 26:1050 26:815 26:1087) is DEFERRED: no storefront route or buyer API.
4. 查看商品 on the claim page opens the product list: the claim preview has no product id / slug (additive preview field needed for a per-product link, DEFERRED).
5. Variant lines (奶油白 · 單一規格, 焦糖 · 數量 1) DEFERRED: order / claim API returns no variant title; the claim line shows `sku_code · 數量 N` instead. 商品示意 image placeholder not drawn (no image in the claim preview).
6. The privacy notice section (version-labelled, `?notice=` target) stays under the overview, and the O6 ads disclosure sits beside the ads checkbox on the settings screen; neither is drawn in the frames but both are compliance wording owned by `lib/privacy-copy.ts` (LC_PRIVACY_POLICY_VERSION).
7. Prototype-only frame strings (原型示範 notices, 預填示範, 示範回應 buttons, prototype switches, the 示範 tag) are not product UI and are not implemented; the erase confirm uses the real typed `ERASE` field and 「確認抹除我的資料」.
8. The cart list with remove buttons stays below the claim confirm card (it resolves a 409 and `claim.test.mjs` asserts removal on the ready page).
9. zh-CN and en strings of every new copy block are written from the zh-TW frame text (no DELIVERY copy table was available in this lane); a copy review is advised.
10. Store chrome (header / footer links per frame) is not touched: it belongs to the shop shell.

STATUS: DONE
