# Merchant operations — visual decision brief

Confirmed: visual comps before UI code; three locale routes with user choice. Scope first merchant workspace, not buyer storefront or platform administration. Mode: Operate. Users work in ordinary daylight on desktop, with mobile quick checks; light surfaces and legible text are the default for this surface, not a whole-product dark/light mandate.

Audit-first: repository currently has no UI/CSS/component system, only APIs and design docs. Existing SHOPLINE evidence is workflow reference, not visual authority or permission to copy its brand. Avoid repeated KPI cards, fake revenue, repeated CTAs and unsupported controls. Show realistic but clearly marked synthetic product data; never imply sandbox equals live. Meta messages, own-site chat and platform support have separate navigation entry points.

First screen job: inspect products/SKUs and available/reserved inventory, locate exceptions, enter create/edit flows without leaving current store. Live session and integration availability remain visible but cannot imply verified provider connection. Language switch is persistent, locally named (简体中文/繁體中文/English); store switch distinct from language/currency.

Seven grounded candidate systems, before seed: (1) retail packing workbench / SKU ledger, (2) broadcast production rundown, (3) transit dispatch exception board, (4) invoice reconciliation register, (5) product contact-sheet catalog, (6) library finding-aid split panes, (7) civic wayfinding typographic console. These cover operating workbenches, temporal schedules, financial documents, catalog imagery and spatial navigation. A generic KPI dashboard and neon dark control room are not default candidates.

No approved brand name, logo or actual product photos. Working title 直播商家工作台 / Commerce workspace, not a claimed customer brand. Product thumbnails in comps are clearly synthetic illustrations. Comp approval is pending; no UI screenshot currently proves functionality.
# Approved composition and implementation inventory

User approval: decision `a30e0e29`, option `wide-ledger`, 2026-09-20. Approved comp: `.impeccable/mocks/composition/wide-ledger.png`. The earlier A choice `6ecd43a2` selected the world; this second choice approves the actual layout. Native image generation was used; each raster has its exact prompt embedded and in its sidecar.

The product ledger dominates the viewport. Navy navigation stays at the left, a single filter row precedes the full-width aligned SKU table, and the selected SKU's stock and adjustment form appear in one horizontal tray below it. Approximate 1586×1024 reference: 214px rail, 64px toolbar, content starts x232. Table spans to x1566, rows roughly 52px; tray is about 180px tall. Heading 28px/700, row text14px, labels12–13px; compact controls with 5–7px radii, 1px cool rules, no heavy shadows. The system sans family is intentional for this operate surface.

|Visible ingredient|Implementation medium and commitment|
|---|---|
|Left navy rail, seven domain entries and status rows|Semantic nav + consistent line SVG icons, active item, independent website/Meta/support domains; no fake badges or enabled dead links|
|Global bar, store and language choice|Semantic forms and links; zh-CN/zh-TW/en route switch preserves current path/query; store identity never derived from language|
|Page title/tabs/filter row|Semantic heading and actual scoped filter affordances; no decorative duplicate search or unimplemented category claims|
|Product/SKU ledger|Native table, row selection, aligned money and numeric columns, real load/empty/error/pagination states; 9 rows visible in demo fidelity fixture|
|Product photos|Produce standalone synthetic thumbnail atlas; CSS positions each cell; label demo data, never present synthetic assets as customer catalog or crop UI mock pixels|
|Main 新增商品 action|Teal semantic button matching comp size and position, launches accessible inline form; permission/loading/disabled state remains truthful|
|Bottom contextual tray|Selected product photo/name, on-hand/reserved/available figures, quantity and required reason, one primary adjustment action; optimistic version and original idempotency key retained on retry|
|Mobile adaptation|Collapse navigation; show priority columns and accessible details instead of shrinking all numbers; bottom tray becomes full-width detail region without trapping focus|

Generated-comp defects are not requirements: omit fake red notification dot, replace unsupported automatic channel-sync subtitle, use actual store currency, use real transaction timestamps, make inventory adjustment reason required, do not fabricate safety-stock controls or product categories. Product images and reference prices are demo-only. Live API pages must show empty/not connected when appropriate. Current backend does not expose joint SKU+stock ledger or server search; add an explicit scoped read projection before claiming a complete live ledger, never stitch truncated first pages or hide missing data.

Direction contract: An operate-first product register for a merchant checking SKU availability while live selling. The first viewport must read as one coherent working ledger, not a dashboard of summary cards. The signature interaction is selecting a row to reveal the matching stock-and-adjustment tray without losing the list. Tabs, controls and illustrations use the approved navy/teal ledger grammar. Preserve the table/tray topology at desktop; responsive variants preserve task order rather than squeeze columns. Every numeric state must come from the scoped API or the plainly labeled isolated demo fixture. The visual gate compares an actual screenshot to the approved comp at the same dimensions before broader UI implementation and after final interaction wiring.
