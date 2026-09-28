# Refund / fulfilment integrator rulings (2026-09-29) — binding for refund-core, fulfilment-core,
# owner-provisioning, refund-fulfilment-ui, refund-fulfilment-tests

All defaults proposed with the briefs are ACCEPTED:
1. D6 `identity.read_merchant_orders` replacement (with refund fields) moves to 0063; fulfilment-core
   alone owns the merchant order read (SQL + Go).
2. D4 new `identity.read_merchant_refunds` in 0062 with the round-2 reviewer's state rules.
3. D1 refund operation created like the checkout operation: UNKNOWN, `generation=1`, no lease.
4. D2 webhook prepare gains payment-intent and refund-metadata parameters; the 64-signal cap is
   enforced in prepare; ingress function allowlist updated accordingly.
5. D3 zero-match refund list within 15 min of the last send → snooze and list again.
6. D5/E5 policy names as written in the briefs; 0063 reuses them.
7. D7 `charge.refunded` reads keep the B1 loader's key version after rotation (documented limit).
8. D8 buyer `refund` field omitted for PAYUNi and when there is no refund activity (PAYUNi bytes
   unchanged).
9. E2 new read-only `GET /order-actions` returning the three permission booleans.
10. E3 buyer role gets SELECT on the shipment heads table.
11. E4 export sets tenant/store/principal GUCs itself before its audit row.
12. RF01/RF02 gate tests belong to the independent test unit.
13. RF11 split: BROWSER(MOCK) without keys + BROWSER(SANDBOX) with keys.
14. UI Q1–Q6: the UI brief's defaults (no new comp; native `<dialog>` for refund; three-language
    carrier names; existing badge palette; export button in the list toolbar; tracking link as a
    plain external link with `rel="noopener noreferrer"`). Owner may revise before go-live.
