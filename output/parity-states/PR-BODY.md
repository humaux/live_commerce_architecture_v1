PARITY-STATES connects the owner-signed recovery panels to shared admin state boundaries and buyer cart/order/link/lookup branches. Current commit is **tests-only RED**, with no application implementation yet. Exact authority: owner card 2026-10-30; Figma iJ9lDdFIXgTlKkt5Tvrc0X page 4:8829; ux-states DELIVERY.md single current 2026-10-28 section. No Figma writes.

Base: `df35b338b5cfde5de34bb590e2ba118efc17e4a4`. Branch: `unit/parity-states`. Target: `r3/integration`.

New coverage runs through existing browser-merchant-orders-ui and browser-storefront gates on REAL routes. Read fixtures are explicitly MOCK. All three locales and 1440×900 / 390×844; signed copy/controls, no raw `403 ·`, loading/error/empty, expired session/link, repeated read failure, original-order uncertainty, real clicks and refresh behavior. Recovery must not replay commands or payment signals. Existing parent gate assertions stay verbatim and in order. No CheckoutFlow changes.

Evidence: E1 `node --check` for both specs and parent entries, exit 0; generated browser ownership + `--check`, exit 0. CI assertion RED PENDING; GREEN NOT_RUN; own PASS E0. See output/parity-states/DELIVERY.md and red.log. Local tests/build/server/browser/PG/provider/production and independent review NOT_RUN. No completion claim. Wait ≤10 minutes for run registration, then integrator relays a fresh session after Foundation/Gates concludes; implementation only after causal assertion RED.

Outside explicit write paths (integrator review required):
- tests/admin/orders-ui.spec.ts — ADD spec import/registration only (+ dependency header).
- tests/storefront/shop-real-gate.mjs — ADD spec import/call only (+ dependency header).
- output/ci-select-backend-browser/browser-ownership.json — generated, never hand-edited.
- output/parity-states/** — card delivery/red receipt and read-only Figma/Eagle references.

The existing session-expired UI is in WorkspaceFrame, not a standalone route. Implementation will list actual same-app paths needed for shell denial/expiry and CartProvider's swallowed failed read under owner scope preapproval. No contract/API/migration/backend/lockfile change. No module page edits on admin.

Parity sheet: screenshot paths are under each browser artifact root; delivery PNGs are under ux-states/output/ux-states/shots. Captures are pending CI. Generic admin list/error and session branches keep truthful task copy; merchant collection/Meta-operation examples are not fabricated. No prototype/sample-order scaffolding ships.

|Frame|Artifact screenshot|DELIVERY PNG|
|---|---|---|
|1429:34986|parity/1429-34986.png|merchant-forbidden-desktop.png|
|1429:35006|parity/1429-35006.png|merchant-forbidden-mobile.png|
|1429:35150|parity/1429-35150.png|buyer-empty-desktop.png|
|1429:35170|parity/1429-35170.png|buyer-empty-mobile.png|
|1429:35190|parity/1429-35190.png|buyer-forbidden-desktop.png|
|1429:35210|parity/1429-35210.png|buyer-forbidden-mobile.png|
|1429:35230|parity/1429-35230.png|buyer-offline-desktop.png|
|1429:35249|parity/1429-35249.png|buyer-offline-mobile.png|
|1429:35268|parity/1429-35268.png|buyer-expired-desktop.png|
|1429:35288|parity/1429-35288.png|buyer-expired-mobile.png|
|1429:35308|parity/1429-35308.png|buyer-unknown-desktop.png|
|1429:35331|parity/1429-35331.png|buyer-unknown-mobile.png|
|1429:35448|parity/1429-35448.png|buyer-offline-desktop-read-failure.png|
|1429:35466|parity/1429-35466.png|buyer-offline-mobile-read-failure.png|
|1429:35484|parity/1429-35484.png|buyer-unknown-desktop-read-failure.png|
|1429:35506|parity/1429-35506.png|buyer-unknown-mobile-read-failure.png|


Risk: new specs are not executed locally; fixture reachability and actual assertion RED must be read from CI before implementing. Independent visual comparison/density and complete implementation copy inventory remain PENDING.
Rollback: revert this PR's commits; no data or schema change.

🤖 Generated with Codex (dispatched by Claude Code integrator).
