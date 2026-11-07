# RVFIX-440 lane report

| failing job/test or finding | class | what changed (file:line) |
|---|---|---|
| A1 buyer_order_evidence case flipped | (b) PR weakened a gate | whole file restored, tests/foundation/buyer_order_evidence_test.go:259 |
| A2 BC02 order-gate + bare aria-label skeleton | (b) | order-gate.mjs restored; CartView restored to visible `Loading…` `role="status"`, CartView.tsx:40 |
| A3 GATE_BASELINE moved into branch | (b) | tests/ci/next-startup.test.mjs:242 back to 9b738e0 |
| P0 collection heading + gate bent to fit | (b) | collections/[slug]/page.tsx:47, shop-real-gate.mjs:168 |
| P0 demo hero shown for every tenant | (b) | SignedHero removed (HomeSections.tsx back to base), shop.css |
| P0 fabricated search count | (b) | search/page.tsx:30-31, shop-copy searchResults |
| B contact page (PII in URL, fake sent) | (b) | route deleted; click-sweep / sweep-shard-lib(+test) back to base; 4 frames DEFERRED → #375 |
| C mobile sort sheet | (b) | ListControls.tsx sheet removed, shop.css sheet CSS removed; 782:4249 NOT_COVERED |
| D spec honesty | (b) | buyer-browse-parity.spec.ts rewritten: step()/shot() ledger, real fill/click/selectOption/reload, manifest computed at the end, true-count log |
| output/parity-buyer-browse/RVFIX.md | report | finding → change → CI run, counts (24 COVERED plan / 41 NOT_COVERED / 4 DEFERRED / 4 BLOCKED) |

Nothing was run locally (owner rule); the new spec assertions are unrun until CI. CI run ids are filled in by the integrator after push.

STATUS: DONE
