# product-ui review and stop line

## Ownership and evidence level

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/product-ui`; branch `unit/product-ui`.
- Base inspected: `e66560e7646ff405fef785fc183af6099b075bd4` (contains backend `39bb8ec`).
- Primary: Codex, actual model inherited from host (not exposed by runtime); no override. Sole code writer. No Go/SQL/deploy/credentials changes, push or merge.
- Read-only subagents: `product_contract`, `product_tests_audit`, `product_visual_review`; `gpt-6-luna`, reasoning `high`, same base/worktree, **no write paths** and no recursive delegation. Dedicated skill finish-review role unavailable; fresh read-only visual agent used as fallback.
- Browser evidence: production Next build + local signed mock identity + real Go + isolated PostgreSQL. No customer backend, live account or payment was accessed. Test product images and names are synthetic. The existing fixture uses **USD**, not TWD.

## Backend blockers (do not enable document editing by filling blanks)

1. `internal/catalog/document.go` `SaveProductDocument` is full replacement. Omitted status defaults to draft; omitted options/memberships/keywords are cleared and omitted active SKUs archived. The detail GET omits per-SKU keywords and product collection IDs. A list-level representative keyword is not sufficient. A live-session library read requires separate `live:read` and an existing session; it is not a catalog contract. Per-SKU weight/dimensions cannot be faithfully represented by the document's global dimensions. This also conflicts with PE14's “only changed fields” requirement.
2. `document.go:196` only calls the stock writer when `*stock.qty != 0`, including edit mode. `target_qty: 0` therefore never sets existing stock to zero. Product detail `available` is aggregate available-to-promise, not warehouse on-hand.
3. The list API supports name/slug/SKU search and status, not global keyword search or the requested smart-filter/sort semantics. Current UI labels filters/sort as applying to the loaded page, clears selection on page/search changes, and does not claim global results. Mixed tracked/untracked products are labelled as containing untracked variants, not globally infinite.

**Stop line:** existing-product save and inline price/inventory editing remain disabled/not implemented. The old ProductEditor/ProductVariants write flow is retired, not left as a second writer. This creates an intentional non-mergeable intermediate branch until the contract is corrected; it is **not** a completed replacement. No authority to patch Go was inferred.

Required backend handoff: either transactional patch semantics that preserve omitted fields, or a complete catalog-only editable document with versioned membership/keyword/logistics/warehouse state; fix zero targets; define server search/filter/sort or rule on page-local scope. Then implement edit/inline commands and re-run PE14/15 plus frozen CC12.

## Resolved in this unit

- Full document create with an idempotency key; staged draft -> image uploads (one stable key per file) -> image order -> bulk publication only after an authoritative image read. Double-click does not issue a second document. Successful stages are not repeated.
- UNKNOWN retains exact in-memory command bytes/key. A sessionStorage **receipt fence contains only the command UUID**, not drafts, media or credentials. If a page is lost before a terminal result, later create is blocked instead of issuing another key. The UI describes this as reconciliation required; it does not pretend to auto-recover files or query a nonexistent receipt endpoint. Reload reconciliation remains manual/backend-dependent.
- Store selector disabled while write outcome is uncertain; generic catalog retry closure checks store/session scope and cannot replay from a different context.
- ProductList opts into a per-store catalogue receipt UUID fence as well. A real copy was committed with `route.fetch`, then the response was dropped; the reload test saw reconciliation-required state and prevented a fresh-key duplicate. This transport fault is explicitly FAULT_INJECTED, not a claim about a real outage. Separate browser tabs do not share these sessionStorage fences.
- Strict bulk confirmation parser rejects empty, duplicate, malformed or ambiguous results. Publish confirms exact product/status before reporting success.
- Matrix generation limited to 100 before expansion; rows rehydrate by option combination, not random database UUID order. Multi-warehouse positive inventory requires explicit warehouse selection and an actionable UI prompt.
- Visual batch: image helper spacing corrected, row action link target >=44px; actual bottom-field geometry check and screenshot confirm the mobile fixed footer does not trap the last SEO field.

## Review record

- Read-only contract audit found the backend preservation and zero-target defects; local source corroborated them. Humaux: `225689e3-f0c4-4cb7-908c-1ea66a16b4f1`.
- Independent request review found UNKNOWN page-loss and store-context risks; fixes above were applied. Final bounded review is recorded in Humaux.
- Fresh visual review inspected all 18 list/editor locale/viewport screenshots against approved 02/03 comps. Confirmed helper-spacing issue and requested footer reachability evidence. One correction batch; targeted second review of three final images closed both findings. This is not whole-feature approval.
- Impeccable static detector: `design-detector.json` contains `[]`, exit 0. Product-specific orange accents use a contrast-safe darker tone; existing shell retained because W0 is not merged into this branch.
- No existing assertion was deleted or relaxed. The new editor browser mode is additive to the real catalog-core harness because Go test files are out of scope. Its existing postconditions still apply; it is not a standalone green acceptance suite.
- Frozen regression failures are **not all backend failures**: catalog-core still drives old name-only creation and expects a redirect; catalog-media requires working editing of images on an existing product, and this UI currently does not provide it; merchant-buyer has both a JS helper and Go fault injection tied to legacy separate creation commands. These are unresolved integration/behavior gaps. Do not label them as passing by replacing an assertion or adding a selector alone.

## Incomplete / NOT_RUN

- PE12 exact TWD buyer-page rendering and selecting a collection: NOT_RUN. Current browser flow proves USD create, image order, no duplicate document, persisted values and buyer HTTP visibility; not a TWD visual claim.
- PE14 safe edit; PE15 inline price/inventory and live-window refusal UI: BLOCKED / NOT_RUN. Bulk copy and unpublish subset executed.
- Collection assignment UI is present; persisted click coverage and >100 collection pagination: NOT_RUN / limited first page. Do not claim full catalogue membership support.
- Exact full PE13 “all prices 80” case, axis value drag, image drag/file-drop, 12th/13th image UI edge, all archive/restore combinations, shipping/SEO persistence and error-navigation dots: not fully covered/implemented. The executed matrix flow covers 12 local rows, only-empty price fill, quantity batch and persisted refresh, and Escape focus return.
- Full keyboard-only creation, axe serious/critical scan, every-control G-UI8 click ledger, all data-dependent error/permission states: NOT_RUN. No axe dependency was silently installed or shared lockfile modified.
- Reload of an UNKNOWN staged operation blocks safely but needs administrator reconciliation; no automatic receipt lookup endpoint exists in this contract. No local draft recovery claimed.
- No release, merge, production acceptance or full visual approval claimed. Integrator owns final contract rulings and follow-up assignment.
