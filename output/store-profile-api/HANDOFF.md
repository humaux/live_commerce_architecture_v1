# STORE-PROFILE-API — waiting for RED

Assigned implementer, no recursive delegation. Exact runtime model variant/effort are not exposed; no substitute values asserted.
Branch `unit/store-profile-api`, worktree `/Volumes/data/live_commerce_architecture_v1/.worktrees/store-profile-api`.
PR #287 targets `r3/integration`. Base `bf161bac96252c35337e07713c08a275d48e3e17`; tests-only head `cb7d00c2d97504367599ff77d07db7c20cf3f4e6`.

Only committed changes: `tests/foundation/store_profile_http_test.go` and additive profile/contract coverage in `tests/admin/live-settings-bff.test.ts`. Static gofmt, go vet, node --check and diff whitespace checks exit 0 (E1); no local tests or servers. New Go tests are selected by the existing foundation catch-all shard. Existing BFF suite is already registered in scripts/dev/test-node.sh. No browser spec/page/route has been changed in this RED push.

## Resume boundary

Read the actual assertion failure from Foundation run 40760270986/job 123069991728 and Gates run 40760270998. Both queued at handoff. Do not infer RED from workflow failure before candidate assertions. Save real run/job/test failures in red.log before implementation. CI is bounded by workflow timeouts; integrator owns the next-session completion relay. No background processes remain.

## Frozen test-facing interface and minimum planned implementation

- GET/PUT `/v1/admin/stores/{store_id}/profile`; BFF GET/PUT `/api/stores/{store}/profile`.
- GET requires `store:read`; PUT uses existing `integration:manage`. Both resolve selected store from the bearer session, including final permission checks. No client tenant/principal/store body fields; unknown fields rejected by shared strict JSON decoding.
- PUT `{name, contact_email, contact_phone?, expected_version}`. Required expected_version includes zero for first profile write; missing/null must reject. Names 1–120 printable Unicode characters with nonblank content; email required plain bounded address, no display name/CRLF; optional phone max32 printable characters. Shared transport 64KiB, safe errors/no-store/request IDs, query refusal.
- Response `{name, contact_email, contact_phone, version}`. Legacy store GET returns existing name, blank contact fields, version0 without writes. Accepted PUT increments version; absent phone clears it. Store-list name and profile name must stay coherent.
- Reuse command.Run with principal-bound request digest, durable original-result replay even after newer versions, CAS and one audit in the same WithScope transaction. Reauthorize after row/advisory lock waits and on replay; rolled-back refusal must retain no receipt/audit/version. No provider/job side effects.
- SQL required: assigned migration **0209_store_profile.sql**. Prefer existing control.stores as the single name source plus contact/version columns; use the existing security-definer/resolve_access pattern for restricted name/contact writes, no broad runtime UPDATE grant. Read owning migrations/identity patterns before implementing. Current R2 migration-count pin134; if no other migrations merge before implementation push, adding0209 means135. Refresh origin/r3/integration and count at push time; do not guess based on expected merge order. Integrator must sequence after #262/#265/settlement0208.
- Contract amendment owns all five TW switches: home, cvs_711, cvs_familymart, cvs_hilife, cvs_okmart. Reuse the selected authorized market's existing MANUAL service of that kind and its existing code, policy_method, policy_version and pricing facts. Missing or ambiguous service/policy stays unavailable; no guessed shipping/tax, no implicit policy/market creation or API carrier enablement. Document the five-row mapping table. First launch MANUAL only.
- Quick-start marker depends on #254 (OC-05, OPEN when checked); no implementation of its marker in this card.
- Prefer `internal/httpapi/profile.go` plus register call using existing command/platform imports (avoid a speculative package). BFF can be an exact profile leaf using existing auth/backend helpers or tightly extend the catch-all; tests allow either. Full BFF static/Request tests use signed auth with synthetic upstream and reject fixture fallback, CSRF/origin loss, foreign store and bad route/query/key grammar.
- Contract test expects heading `## STORE-PROFILE-API`, profile field/version/key/permission terms, the five mapping rows and `#254`/`OC-05` references.

## Scope and authority

Read AGENTS.md; 架构.md sections0/1/2/26/27; merchant-settings-wizard-v1.md; merchant-settings-http-v1.md; related existing HTTP/command/platform/fulfillment code and tests. Read the named settings-v2 lane PLAN.md as a read-only blocker source; changed no other worktree or branch. Current origin/r3/integration matched the dispatch base when fetched. User card is authority for new contract/migration scope.

Planned paths are listed in PR.md. No explicit pre-existing STORE-PROFILE-API DAG entry was found. Humaux and code-map/index tools unavailable; no Humaux store, canvas update or coord action claimed. Integrator must mirror this handoff into Humaux using the shared lock protocol. Local output files are intentionally uncommitted to preserve the one tests-only RED push; final implementation push may include them. Do not push an evidence-only commit after GREEN.

NOT_RUN: CI assertions, GREEN, local tests/servers/PG/browser, provider sandbox/LIVE, production, independent review. No completion or PASS claimed.

## RELAY 1 — supersedes the waiting-state facts above

Foundation 40760270986 / job 123069991728 at cb7d00c2 failed only the pre-existing browser ownership currency assertion. This is a reachability failure, NOT a causal profile RED. The failed-job excerpt is appended in red.log. Gates 40760270998 remained queued at inspection.

Per RELAY 1, merged origin/r3/integration at 313c2c36f4ac064975c1c7c170d710cb63fa457f and regenerated output/ci-select-backend-browser/browser-ownership.json with its owning static generator (53 modes). The generated delta contains only the updated BFF test hash and new Go test hash. Original profile test assertions remain unchanged. Targeted blocker attempt: 1/2. No implementation until the new assertions actually fail in CI. No local tests or servers.

#254 is now MERGED (GitHub mergedAt 2026-10-28T20:07:09Z, merge commit 313c2c36f4ac064975c1c7c170d710cb63fa457f) and is included by that merge. Quick-start must reuse its code. The old base/pin estimate above is historical; refresh at implementation push. This relay creates no migration and changes no pin. Base-owned files arriving through the merge are not this card's authored changes.

Humaux tools became discoverable in this relay: memory search, rejected enumeration (3/9 representative entries), coord list and canvas read succeeded. One developer-rules memory_get had a transport failure; no full-text success is claimed. This is an assigned lane, so no separate task claim or recursive delegation. CI-only-evidence and ponytail skills were read; the card specifically authorizes this static generator. The integrator owns completion wakeup via a new relay after the new Gates/Foundation runs finish; no background process is left. Final run IDs are recorded in the PR handoff comment.
