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
Historical P0 dependency (closed by backend 727e355): domain writes originally ignored Idempotency-Key. The supplemental UI now journals the exact key/action/target before send, scoped to store + session hash. UNKNOWN permits only explicit same-command replay; refresh/reload remain read-only, and a failed replay cannot clear the original responsibility. Old marker-only records stay locked, because their request cannot be reconstructed safely.
P2: DNS and lifecycle instructions were unstyled and unavailable for one-click copying; add scoped layout, copy feedback and merchant language.

## Supplemental audit, 2026-10-02

Backend 727e355 now supplies authoritative `kind`. Platform rows display the address/type/status without suspend/detach buttons. No inference from hostname, token-null, ordering or serving status. Inspection of the first English mobile screenshot found type/status text touching; final source puts the type label on its own line.

Apex DNS uses every literal IPv4/IPv6 value in `edge_addresses`, shows its A/AAAA type and host/value pair, and offers per-record copy plus all-record copy. Missing edge addresses still show explicit platform-confirmation guidance: the backend resolver is best effort, so an unavailable value is never invented or substituted with a CNAME hostname. Browser display/clipboard cases use clearly labelled synthetic addresses; real DNS resolution is outside this UI unit. Onboarding with no configured base still reports internal workspace created but URL not ready.

## Verification boundary

Browser gates use local fixtures, production Next + Go + isolated PG, signed MOCK IdP and MOCK DNS/TLS. No production, provider or real certificate acceptance is claimed. Screenshots include only synthetic test data. Final results and remaining visual issues belong in SUMMARY.md.
