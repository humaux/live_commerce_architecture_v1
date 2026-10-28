Adds signed GET/PUT store profiles for settings v2: required store name/contact email, optional phone, durable original-result replay and version CAS. The profile updates the same store name used by store discovery and OC-05. Scoped authorization is rechecked after row/advisory lock waits and replay; profile, audit and command receipt commit together. No provider call or new onboarding marker.

The owning contract maps all five simplified TW switches to the authorized market's existing current MANUAL service of each kind and its persisted code/policy/pricing facts. Missing or ambiguous services/policies stay unavailable; first launch remains MANUAL.

Validation and evidence:
- RED: Foundation **40762550914**, job **123078018073**, head `24cac517c734c0d09cd017f523c98d31738c8a24`, synthetic checkout `e431a572b1a5b24eee6a5ffaef99b8714d2ad11f`. Four new assertions failed on absent profile BFF/contract (404 instead of 200/401/409; missing amendment). `output/store-profile-api/red.log` contains the raw filtered receipt. The preceding browser-ownership failure was reachability only.
- E1 static: Go build/vet, Node syntax and packet checks return 0; command receipts in `output/store-profile-api/static.log`. Existing assertions stay intact. Profile-positive tests declare their own required permissions instead of inheriting live-only grants; added grant-denial, malformed-command, receipt-mismatch, middleware and restricted SQL-writer checks. No global fixture or expected denial was weakened.
- GREEN: **PENDING implementation-head CI**; exact run/head/job IDs will be in the handoff comment. No local tests, servers, PostgreSQL, browser, provider SANDBOX/LIVE, production or independent acceptance; tsc NOT_RUN because this worktree has no installed TypeScript dependencies. Author assessment E0.

Dependencies and ownership:
- Reuses merged **#254 / OC-05**. No marker reimplementation or implicit completion.
- Assigned migration **0209**; R2 pin **136 = 135 fetched base + this migration**, base `313c2c36f4ac064975c1c7c170d710cb63fa457f`. **Integrator must merge after #262/#265/settlement0208 and refresh the exact count for the then-current set.** #262/#265 were OPEN on readback. This lane never merges.
- Owning paths: contract amendment; internal/httpapi/profile.go and handler registration; migration0209; apps/admin exact profile BFF and existing live-settings-model/proxy helpers; profile/R2 foundation tests and existing BFF tests; output/store-profile-api/**.
- Additional generated path for integrator review: **output/ci-select-backend-browser/browser-ownership.json**, generated after merging origin/r3/integration (already up to date). This is explicitly authorized by CI-SPEED-3/RELAY1. **apps/admin/lib/live-settings-model.ts** and **apps/admin/lib/live-settings-proxy.ts** are same-app helper scope under the card's scope pre-approval. Dependency map was regenerated and has no diff. No other outside path, OpenAPI, dispatcher pin, drift baseline, lockfile or unassigned migration change.
- `output/store-profile-api/DELIVERY.md` records semantics, risks, rollback reference and handoff. Humaux is unavailable in this relay; integrator must mirror store/canvas evidence under its shared-state lock. No completion is claimed.

🤖 Generated with Codex (dispatched by Claude Code integrator).
