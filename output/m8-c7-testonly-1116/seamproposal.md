# Root-only integration proposal (not applied)

Current base has no parity module or parent hook. Do not restore the 2,000-line historical spec or PR440's other paths.
In tests/storefront/shop-real-gate.mjs add `import { checkC7DraftCancel } from "./buyer-browse-parity.spec.ts";`
and immediately after the existing SFR04 pass, before SFR05, call
`await checkC7DraftCancel({ browser, origin, evidence, productSlug: P("candle").slug });`.
This lets original SFR01–04 run before C7. The function separately checks baseline selection and quantity before asserting entry visibility.
No marker/count/time-limit changes are proposed; root must inspect existing Go driver's final-marker oracle and keep it unchanged.
Existing command, root/CI only: `bash scripts/dev/test-local.sh --browser-storefront` (real Go/PG synthetic stack); existing `.github/workflows/gates.yml` supplies that mode. No workflow/ownership edit by leaf.
Root must regenerate/check existing browser ownership via its generator when admitting the new file; this leaf did not run Node or touch generated JSON.

Proposed selectors reuse PR440: product-options-task-entry > button; product-options-task; dialog's Back to product button; product-buy[data-sku]; radio names 180g/300g; qty-input; cart-empty.
Role/name copy currently comes from historical PR440 en copy, not proven current promotion. Source gate must freeze this before interpreting native feature failure.
Candidate does not assert dimensions, confirm/add, prepopulated cart preservation, soldout/long labels, UNKNOWN recovery, three locales or desktop; those remain required later.

Exact proposed later UI write paths (HOLD; root must admit after native RED):

- apps/storefront/components/ProductBuy.tsx (separate committed choices/quantity from pending draft; preserve existing cart.add journal/CAS/UNKNOWN).
- apps/storefront/components/product/MobileProductOptions.tsx (new local task composition; reuse historical design, not wholesale PR import).
- apps/storefront/components/product/MobileProductOptions.module.css (new component-local formal tokens/layout; no globals).
- apps/storefront/lib/shop-copy.ts (only admitted three-locale option-task copy).

Missing root evidence: canonical promoted full signed frame manifest/version/raw context/PNG/hash; exact current selection/cancel policy authority; 13-product and 12-cart assertion plans mapped to current routes/data. Historical 51:1380 raw shows back navigation to 4:9532, not a complete committed-state rollback policy. Human task supplies rollback requirement; do not infer server/SKU truth from prototype variables.
