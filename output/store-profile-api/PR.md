Store settings v2 needs authenticated, versioned store-profile read/write. This PR is still in **tests-only RED**: implementation is withheld until CI reaches the new assertions.

RELAY 1: Foundation run **40760270986**, job **123069991728**, head `cb7d00c2d97504367599ff77d07db7c20cf3f4e6`, failed only the existing browser-ownership currency gate. This is NOT causal profile RED. Attempt 1/2 merges current `origin/r3/integration` (`313c2c36f4ac064975c1c7c170d710cb63fa457f`) and regenerates its ownership artifact using `node scripts/dev/derive-browser-ownership.mjs`. No profile assertions, fixtures, thresholds or exemptions were changed.

Tests cover HTTP/PG persistence, store-name coherence, idempotent replay, changed-body/stale-version conflicts, concurrency, auth-derived scope, read-only access, malformed input, post-lock revocation rollback and the BFF Request boundary. Contract assertions require all five existing Taiwan MANUAL delivery-kind mappings and OC-05 reuse.

Scope and dependencies:
- #254 (OC-05) is MERGED and included by the base merge. Reuse its quick-start marker.
- Assigned migration if needed: **0209**, after #262/#265/settlement 0208. No migration or R2 pin change in this relay; refresh the pin from the actual base before implementation.
- Planned owning paths: contracts/merchant-settings-wizard-v1.md; internal/httpapi/profile.go and registration in handler.go; migrations/0209_store_profile.sql; apps/admin profile BFF route and required route helpers; tests/foundation/store_profile_http_test.go; tests/admin/live-settings-bff.test.ts; tests/foundation/r2_integration_upgrade_test.go; output/store-profile-api/**.
- Additional authored path: **output/ci-select-backend-browser/browser-ownership.json**, generated as explicitly authorized by RELAY 1. Base-owned files arriving through the merge are not authored by this card. No other out-of-scope changes.

Evidence: original assertion RED **PENDING**; GREEN **NOT_RUN**. Generator exit 0 (53 modes), whitespace static check exit 0; E1 static only, author assessment E0. No local tests, servers, PostgreSQL, browser, provider sandbox/LIVE, production or independent acceptance.

Resume after the integrator relays the new Gates/Foundation conclusion; final exact head and WAIT_RUN IDs are in the PR handoff comment. No silent background task remains. Rollback: return this lane to `cb7d00c2d97504367599ff77d07db7c20cf3f4e6` only if explicitly directed; no force-push.

🤖 Generated with Codex (dispatched by Claude Code integrator).
