# Before-code audit — 2026-10-05

Base 0cc63a1d. Real production Next + Go + isolated PG, MOCK identity, no production data.
24 full-page PNGs and DOM measurements in `before/`: new/existing, empty/single SKU and 3 variants, zh-TW/zh-CN/en, 1586×992 and 390×844. Existing empty means legal saved product without images/options, not an invalid nameless product. No app source changed for capture.

Confirmed structural defects across three locales:
- P1 spec grid: desktop name input44px versus textarea50px; label/control start offset51–54px. Old bottom-alignment includes hint/chips in the value column.
- P1 matrix: inventory control88px desktop; row109px. At390 each SKU consumes507–525px. Quantity and tracking must share one line; wide matrix may scroll within its own region.
- P2 bulk hierarchy: five equal primary-looking bulk controls, all repeat the same interaction.
- P2 checklist: images appear twice; 10px circles far from labels, unclear pending/completed semantics.
- P2 navigation: price/inventory item still present after adding options removes its target. Source fallback scrolls to variants without correcting nav identity.
- P2 typography: name counter, value hint and New tag are10px. Variant names are present once real values commit; empty option setup has no name fallback, only New.
- P2 palette: checkbox RGB184/75/0 and editor primary actions use a local orange override instead of incumbent action-teal token.
- P2 density: name field1047px in this baseline (not1230px); image uploader occupies one120px tile inside a wide mostly-empty panel.

Positive: zero document horizontal overflow in all24states; save/recovery and merge-patch gate remain real; no new data/model needed. Mechanical layout detector returned[] (exit0), demonstrating why rendered evidence is necessary.

Spatial thesis: image/basic details → prices/variants → categorization/shipping/SEO. Bounded label/input groups, one shared bulk action, aligned44px controls. Preserve shell and save affordance; the matrix alone scrolls at narrow widths. Keep full SKU names and readable headers. Helpers≥12px; one required image item. Nav derived from rendered section availability.

Audit health (scoped judgment, not WCAG certification): accessibility2, performance3(no new hot path observed; not benchmarked), responsive2, theming2, implementation integrity2 =11/20. Fix via layout/adapt then one bounded confirmation pass. Manual visual acceptance remains integrator/owner responsibility.
