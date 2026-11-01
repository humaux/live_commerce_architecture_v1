# PARITY-BUYER-AFTER round 1b: tests-only RED lane report

Tests-only commit. No test was run (owner rule: CI only). Copy source: `output/parity-buyer-after/FRAME-COPY.md` (186d1f5a2). Class column: n/a (new RED spec, no CI failure to classify yet).

| Finding / job | Class | What changed (file) |
|---|---|---|
| New real-route spec, 32 node:test cases, zh-TW signed copy, real clicks, click ledger, `parity/<frame-id>.png` per frame (colon written as dash) | n/a | `tests/storefront/buyer-after-parity.mjs` (new; MOCK buyer API for orders / privacy / claim layered over `shop-fake-api.mjs`, production Next behind the shop.example edge) |
| Mode `--browser-buyer-after-parity` (lc_build=storefront, no PG), prints `## matrix-receipts browser-spec` | n/a | `scripts/dev/test-local.sh` (new arm before `APPEND MODES HERE`) |
| Release-gate judge: node pass >= 32 and `ℹ fail 0` | n/a | `scripts/dev/release-gate.sh` (new branch before `--browser-ui-gallery`) |
| Gate doc row | n/a | `docs/delivery/GATES.md` |
| Ownership regenerated | n/a | `output/ci-select-backend-browser/browser-ownership.json` (derive, then `--check`) |

## Frame coverage
- Order status: 4:9963 4:10049 25:943 25:995; 1198:14549 14615 14682 14748 14815 14943 14994 15048 15099 15153; 1207:11941 12009 12062 12130; 1222:19371..19867 (cancel flow); 1222:21567..22533 (pay-at-pickup and COD); 63:1475..1478 (terminal views); 29:1064 29:1095 1198:10796 1198:10886 (payment hand-off page).
- Lookup 26:708 26:995 1253:17225 1253:17256; order link 26:746 26:1028; payment return 1207:16447 1207:16465.
- Privacy 5:15460 5:15540 5:15707 5:15793 26:888 26:1150 29:950 29:983 29:1011 29:1040 37:1083..37:1298; claim 4:12347 4:12417 66:1746 66:1747 66:1754 66:1755.
- The native confirm frames (1222:19371 19527 19657 19813) are captured as the page behind the dialog; only "a confirm was shown" is asserted.

## Not asserted (spec manifest.json `deferred`, to be listed in the PR body)
- 26:773 26:1050 26:815 26:1087 (申請售後 form, 售後申請 R-1001): no storefront route or buyer API in this module. The 申請售後 control on the order page is asserted, its destination is not.
- 1222:14056 1222:14106 (門市取貨資訊 checkout form): PARITY-CHECKOUT scope ("Not checkout/CVS files").
- Variant lines (奶油白 · 單一規格, 焦糖 · 數量 1): the order / claim API returns no variant title.
- Prototype-only copy (原型示範, 預填示範, 示範回應, "Prototype switch", 原型：… buttons), native confirm text and the unlabeled 「示範」 tag: never asserted.

## Frame conflicts the implementer needs an integrator ruling on (not worked around)
1. COD PENDING frame 1222:21795 shows 「包裹送達時向貨運公司付款」 and 「到貨需付 NT$1,280」, but `tests/storefront/cod-footer.spec.ts` (OC-09a, owner approved) asserts the old 「向貨運公司」 wording gone and the label 「到貨時要付的金額」. The spec asserts only 「到貨需付 NT$1,280」 and the 「請以現金付款…」 line, plus absence of 「到貨時要付的金額」. The cod-footer unit test needs the same ruling.
2. Lookup default frame 26:708 (「尋找你的訂單」, button 「查詢訂單」) vs not-found frame 1253:17225 (legacy intro, button 「查詢」). The spec asserts 26:708 copy in the default state and only the error and kept-values copy of 1253 in the not-found state.
3. Payment return frames carry a Traditional-character string with Mainland wording (刷新訂單) plus a zh-TW string. The spec asserts en and the zh-TW string, and asserts the current Simplified section absent (the frame has no zh-CN string).
4. Order number: frames show 「訂單 LC-10482」 / 「B-10470」; the API returns only the UUID. The spec asserts a short-code heading (`/^訂單 [A-Z0-9][A-Z0-9-]{3,}$/`) and that the full UUID is not shown.
5. Controls whose visible frame text is prototype copy but whose layer name is product copy: 「確認抹除我的資料」 (visible 「查看完成狀態示範」) and 「確認同一次操作」 (frame shows only 示範回應 buttons) are asserted by the layer-name wording; if the product label differs, change the spec in the implementation round with a note.
6. Store chrome (header and footer links per frame) is not asserted: the shop fake serves another design document.

## Legacy asserted gone
Order page: 「你的訂單」, 「訂單金額」, 「前往新分頁付款」, the full-UUID row (`data-testid=order-id`) and the UUID text. Lookup: the session note and the old intro. Privacy: heading 「你的隱私」. Claim: heading 「你登記的商品」. Payment return: the Simplified section.

STATUS: DONE
