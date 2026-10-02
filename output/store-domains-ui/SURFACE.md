# Store domains UI — scoped surface record

Reference: owner-supplied `output/r5-research/comps-webgpt/01-admin-overview.png` (main checkout).
This task inherits the incumbent Settings/onboarding shell; it does not replace the shell or invent overview controls.

- Thesis: a merchant can distinguish publication from address setup, read server state and copy exact DNS records.
- Own world: quiet white settings surface, dark ink, restrained orange verification action, flat 1px borders and 8px corners. Existing shell typography and navigation remain outside this unit.
- Story: publication card → independent address section → domain rows → custom-domain request → DNS instructions and honest pending/error feedback.
- First viewport: address, human-readable state and permitted next action; no invented live/verified claims.
- Form: narrow extension of an approved surface; no new concept roll. At 390px rows/forms wrap; controls are at least 44px.
- Functional semantics take precedence over generated comp content. Three languages; no task center/search/notifications added.

## Audit and scoped corrections

P1: domain input nested in the frozen publication card; separated into `storefront-domains-card`.
P1: failed handle preview could display stale availability; clear prior result, abort obsolete fetch, show unconfirmed state.
P0 backend dependency: the domain routes ignore Idempotency-Key. UI now persists a pending marker before any domain command, clears it only for a confirmed outcome, and never replays UNKNOWN. The same session remains read-only for domain writes across refresh until reconciliation. This is a containment boundary, not backend idempotency.
P2: DNS and lifecycle instructions were unstyled and unavailable for one-click copying; add scoped layout, copy feedback and merchant language.

## Known backend-dependent item

Item 2 is BLOCKED: domain rows contain no authoritative platform/custom discriminator. The UI cannot safely infer ownership class from hostname, token-null, ordering or serving status. Backend must expose per-row `kind` or `is_platform` from stored authority. Existing backend mutation rejection is retained; the platform row still has visible controls until the contract changes. No guessed workaround is shipped.

Apex DNS instructions also lack an edge IP. The UI copies TXT and shows explicit missing-IP guidance, never offers the CNAME hostname as an A value. A fully actionable apex record needs a backend value. Onboarding with no configured base reports internal workspace created but URL not ready, without inventing a buyer address.

## Verification boundary

Browser gates use local fixtures, production Next + Go + isolated PG, signed MOCK IdP and MOCK DNS/TLS. No production, provider or real certificate acceptance is claimed. Screenshots include only synthetic test data. Final results and remaining visual issues belong in SUMMARY.md.
