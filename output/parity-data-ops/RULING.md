# PARITY-DATA-OPS integrator ruling — 2026-11-04

Authority: integrator (opus-orchestrator-orca). Owner instruction 2026-11-04: 「你看能力来编排车队吧，继续推进」. Owner precedent for parity holds (#476): build to the signed design where the product supports it, and let the integrator rule the rest 「符合业务」. Frame ids come from `output/parity-data-ops/DELIVERY.md` at a857d03b3.

## Rulings

| Frames | Ruling | Reason | Follow-up |
|---|---|---|---|
| 1028:7221 / 1028:7299 source-specific customer required error | **RESOLVED** (no longer BLOCKED) | The preview now carries a preview-only canonical `ErrorField` (`internal/migrationimport/customers_csv.go:94,231,241`, `customers.go:298-299`): field name only, no PII, never persisted, and pinned by `customers_test.go:290-298`. CI was green at a857d03b3. | none |
| 1028:7087 / 1028:7165 demo CSV chooser | **SUPERSEDED** | Prototype-only demo picker. The product uses the real native file chooser, which is covered. A fake chooser in production would mislead merchants. | Figma: mark these frames prototype-only |
| 6:22818 / 6:22986 inventory history (庫存流水) | **DEFERRED** with an issue | Needs a new scoped read contract (timestamp, actor, source, reason, before/after per movement). catalog-ledger returns balances only. Showing fabricated history or dead "查看流水" controls is worse than not showing them. | #542 item 1 (inventory history read API + UI) |
| 5:15211 / 5:15352 product import recent-task rows | **PARTIAL** accepted | The frame renders on the real route with real data. Only the "最近任務" rows are omitted: there is no product import jobs/list API, and no local history is fabricated. | #542 item 2 (import jobs list API) |
| 30:1441 / 30:1528 named per-product preview rows | **PARTIAL** accepted | The preview shows the real aggregate counts and errors. The per-row product name/outcome/description needs an additive ImportResult rows DTO. | #542 item 3 (per-row preview DTO) |
| 6:19278 / 6:19486 tag/source collection rules and drag order | **PARTIAL** accepted | The automatic API supports created_at/within_days only, and there is no collection-position write contract. The existing creation-time rules are implemented. | #542 item 4 (collection rule types + position write) |

None of these rulings removes or weakens an assertion. Each DEFERRED/PARTIAL frame keeps its DELIVERY.md row with the reason. RC readiness lists them as explicit known gaps, not as COVERED.

## Shared browser helpers (integrator approval requested in the PR body)

Approved: `tests/storefront/merchant-product.mjs` and `tests/storefront/catalog-media-gate.mjs`.

- The catalog-media Ledger facts are unchanged: renamed product, new SKU price, archived and active status, disabled stock control for the archived SKU, and the real cover photo. They are now read from the signed SKU inspector after selection instead of the removed table columns. The archived state is asserted visible on both viewports, which is stronger than before.
- `selectLedgerRow` now returns the server-owned available balance. Every caller was updated (`merchant-buyer-gate.mjs:136-137` asserts the balance is 0; the others ignore the return value).
- `createProductInEditor` reaches the editor by real clicks (inventory → 返回商品 → product-new) instead of a direct goto.
