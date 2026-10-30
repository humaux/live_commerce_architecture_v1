Wire the owner-signed StatePanel/InlineError into real admin and buyer recovery boundaries. Admin denial no longer renders raw `403 ·`; shared loading, empty, read failure and session expiry use signed panels. Cart/order/link/lookup branches share one storefront state component. Failed cart reads no longer appear empty; failed same-order payment status reads preserve the original result/amount without replaying payment. All three locales are in packages/i18n; existing caller task copy stays contextual.

Authority: owner PARITY-STATES card 2026-10-30; Figma iJ9lDdFIXgTlKkt5Tvrc0X page 4:8829; current ux-states DELIVERY 2026-10-28. Branch unit/parity-states, base r3/integration; no CheckoutFlow/API/contract/migration/backend/manifest/lockfile change. No Figma writes. The existing Stripe Continue/Cancel and polling/signal semantics remain; the signed Check status control itself only GETs the same payment. Recovery is placed ahead of long delivery details while original order facts/disclosures remain available.

RED (E3 observed failure only): head b3c4736a53279bb9557755e4540351f01c1ce126, Gates run 41052469401 / bundle:01 job 124056245071, tested synthetic merge c6e49cf79ab86b427a5ee676ef87650a8200e50a. Admin: 24 new PARITY-STATES signed-panel failures; all 21 original tests passed. Buyer: the final PARITY-STATES assertion reports missing signed panels and lookup expected DIV/actual legacy P. Foundation 41052469470 / 124051677598 passed. Earlier fragment-registration failure is not counted as RED. See output/parity-states/red.log.

GREEN: PENDING implementation push CI. Author PASS = E0. Static E1: node --check exit 0; ownership generator + --check exit 0 (53 modes); depmap regenerated without change; check_packet.py exit 0, packet structure only. tsc unavailable locally. No local tests/build/server/browser/PG/container runs. Independent review, screenshot parity, density, provider/SANDBOX/LIVE/production acceptance NOT_RUN. No completion claim.

Admin SSR harness allowlists now load the real signed components. Superseded denial-copy expectations require current signed copy; privacy and permission assertions remain. New shell recovery test now checks exact /studio?store=... and no denial after navigation: its fixture removes orders:read, so Overview is also denied. Existing storefront gate-adapter assertions/order, fixtures, thresholds and exemption lists remain unchanged.

Outside nominal write paths (same-app scope preapproval; integrator review list):
- apps/admin/components/WorkspaceFrame.tsx — actual session-expired/route-denied boundary (no standalone expired route exists).
- apps/admin/components/admin-state-block.css — remove double state-card border/padding.
- apps/admin/src/shell-copy.ts — remove raw-code denial strings shared by existing tests.
- apps/storefront/components/CartProvider.tsx — expose failed reads separately from durable write recovery; retain ordering fences.
- apps/storefront/components/CartView.tsx, OrderLink.tsx, OrderPayment.tsx — actual cart/link/payment state branches.
- tests/admin/orders-ui.spec.ts, tests/storefront/shop-real-gate.mjs — registered browser coverage from the RED commits; current admin copy/landing assertions track the signed panel.
- tests/admin/code-alignment-admin-state.test.mjs, shell-render.test.ts, staff-team.spec.ts — current signed expectations and real component import harnesses.
- output/ci-select-backend-browser/browser-ownership.json — generated after spec edits.
- output/parity-states/** — delivery/copy/parity/RED receipts and read-only references. Raw downloaded failed CI artifacts are local only, not committed.

Parity sheet: REAL routes, all three locales at 1440×900 and 390×844. Artifacts save these paths under their browser root; independent reviewer compares against ux-states/output/ux-states/shots. No sample brand/order facts, design labels or rehearsal controls are shipped. Admin generic task states retain truthful task copy rather than fabricating collection or OAuth examples. Captures/visual/density acceptance remain PENDING implementation CI.

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

Click ledger and required desktop/mobile/read-failure facts: output/parity-states/DELIVERY.md. Recovery click results are CI-owned; no local or author-only product PASS. BOURDIEU Q1–Q6, project contracts, Mobbin→Eagle principles, official Figma guidance and three-language copy tables are recorded there.

Rollback: revert the PR's commits; no data/schema change. Humaux tools are unavailable on this endpoint; store/canvas/coord are UNKNOWN, not claimed. Integrator relays a fresh session when Foundation/Gates completes; stop CI polling within ~10 minutes and report real run IDs.

🤖 Generated with Codex (dispatched by Claude Code integrator).
