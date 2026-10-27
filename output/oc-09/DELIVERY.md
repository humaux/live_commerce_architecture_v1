# OC-09a delivery (order-page / pre-order COD copy)
Scope: apps/storefront/lib/cod-copy.ts (3 locales: `recorded`, `orderStates.PENDING`, `codNote`) + tests/storefront/cod-footer.spec.ts. CodOrderStatus.tsx unchanged (amount stays server `cod_collect_minor`).
Footer 「到貨需付」 is DEFERRED to OC-09b: the quote/options carry no COD due field (only `total_minor` and `cod_surcharge_minor`; OrderFlow.tsx:553 adds them on the client). Integrator ruling: server adds a quoted COD due + contract amendment first. CheckoutFlow.tsx and OrderFlow.tsx untouched.
zh-TW is the owner-approved text (OWNER-APPROVED-COPY.md); zh-CN and en are same-meaning translations.
RED: NOT_RUN. tests/storefront/cod-footer.spec.ts is a node:test file, but no CI entry runs it. Registration needs one line in scripts/dev/test-node.sh (shared file, not edited here): `node --test --experimental-strip-types tests/storefront/cod-footer.spec.ts`. Until then CI cannot prove RED or GREEN.
GREEN: NOT_RUN (same reason). Existing home-cod-buyer.mjs matches only /現金/, /cash/i, and "已貨到收款", all still satisfied by the new text (read by inspection, not run).
Local checks: scripts/check_packet.py ran; no tests, no dev server, no browser.
Risks: `recorded` is also shown for COLLECTED/RETURNED states, where 「到貨時要付的金額」 reads slightly oddly (owner-approved row, kept). 390px/3-locale visual check not run (no layout change).
Head: see PR.
