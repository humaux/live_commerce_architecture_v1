# Admin visual audit — before implementation

Status: MOCK/source audit, not LIVE. Base product-editor-visual `897e0fd6`; frozen harness imported at `70d9d1d2`.

## Confirmed structure

- ADM-15: the offer form places five fields plus two actions in an auto-sized grid; the live-price hint consumes the flexible width. The English desktop evidence visibly clips “Add offer” and reduces product/SKU selects to unusable widths. Split field, hint and action rows; preserve commands.
- ADM-23: catalog selection-column sizing collides with a generic first-column rule. Desktop status overlaps product metadata; mobile selection consumes a separate blank row. Give each column an explicit role and keep numeric values aligned without losing 44px hit areas.
- Foundation: W0 owns navigation but domain CSS independently owns buttons, badges, fields, headers and tables. The six accent definitions, 38–45px controls and independent heading insets are multiple implementations of the same intent. Fix canonical primitives then migrate consumers; do not append a universal override sheet.
- Breadcrumb/title copy has two sources; create-product matches the edit route. Domain breadcrumbs duplicate shell breadcrumbs on claims/customers. Registry titles must be authoritative with explicit create/detail entries.
- Repeated empty states and equal-height grids inflate billing, attribution, import and claims. Preserve factual counters and warning meaning while consolidating presentation.
- Native file/date UI needs actual locale verification; static browser-default screenshots alone do not establish a localized-date defect.
- ADM-34 editor repairs are inherited, not reimplemented. ADM-35 placeholder is missing print coverage, not a clean result.

## Evidence and scope

Authoritative manual report: `/Volumes/data/live_commerce_architecture_v1/output/visual-review/REVIEW-admin.md` and `defects-admin.json` (35 findings).
Before corpus: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ui-visual-audit/output/ui-visual-audit/20261005T045559Z/` (1586×992 and 390×844, zh-TW/zh-CN/en).
Root inspected full-page claims English desktop and catalog zh-TW desktop/mobile before changes. Further per-finding inspection accompanies each scoped migration.
`packages/ui/src` is the actual canonical shell/token location, outside the initial allowed paths. Clarification requested; no package modifications until authorized.

## Acceptance

Frozen visual runner: all admin blocking R2/R3/R6 zero; stronger R4 criterion is all admin R4 counts zero (sampled crops are not sufficient). Real-click regression, all admin browser modes, admin tsc, node tests and architecture gates. Capture after corpus and map every finding to before/after evidence. No threshold changes or behavior/data-flow changes.

## Responsibility

- Root: integration, shared foundation, final independent reruns; worktree `admin-visual`.
- `av_claims`: ui_worker, gpt-6.1-sol/high, base `70d9d1d2`, separate `admin-visual-claims`; StudioClaims/claims CSS and focused admin tests only.
- `av_catalog`: ui_worker, gpt-6.1-sol/high, base `70d9d1d2`, separate `admin-visual-catalog`; ProductList/catalog CSS and focused admin tests only.
- `av_foundation`, `av_harness`: read-only explorers, gpt-6.1-sol/medium; no source writes.

Pending: all unit gates and after evidence. Owner/non-author visual approval remains external to author delivery.
