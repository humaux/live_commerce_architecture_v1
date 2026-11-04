FEAT-LIVE-REPLAY adds durable official Facebook video references, an authenticated merchant publication API and published-origin buyer list/detail reads using the current catalog. UI remains pending PARITY-NEWFEAT. No backfill, media hosting, production release or customer message.

FIX5 repairs the four failures identified by the integrator's escalation ruling. Integration was merged first: base `4b8c04c7213025c173e260db6f9c81cb5b818c61`; migration stays **0219**, assigned R2 count **143**.

- Merchant replay GET now proves an otherwise unframed reader empty with a bounded one-byte read before delegating to the existing claims guard. The original GET reproducer and its 200 expectation are unchanged; body bytes/errors, invalid framing, query, key, UUID and authorization remain refused. The shared studio guard is unchanged.
- The resolver's closed ACL expectation adds only the required replay reader. Four additional exact owner/caller/search_path checks cover all 0219 definers, which already explicitly set owners, revoke PUBLIC and use `search_path=pg_catalog`.
- Every one of the six `live.offers` column rights is needed by the public projection: tenant/store fences, offer id, session restriction, SKU join and deterministic creation order. Add only those exact matrix entries, each with a reason. Narrow `live.replay_videos` from table SELECT to its seven read columns, removing unused requested_at. Full grant-to-body audit: `output/FEAT-LIVE-REPLAY/FIX5.md`.
- Add exactly one BACKEND_ONLY entry naming the replay Go test. No browser spec/runner currently calls replay routes. Requested derivation stopped at unrelated profile.go coverage; its original error is retained, no tool/gate edits. Ownership regenerated and checked (55 modes); depmap regenerated without diff.

| RED receipt | Failure |
| --- | --- |
| Tests-first Gates `41093988979`, unit `124202749403`, g10 `124202749598` | New permalink/persistence/replay route assertions failed before implementation. |
| Foundation `41275260986 / 124756060533` | Merchant replay file unclassified. |
| Gates `41275260978 / 124756873935` | Resolver closed ACL missing the new reader. |
| Same Gates / `124756873395` | Six extra reader SELECT columns on live.offers. |
| Same Gates / `124756873477` | Initial merchant GET 422, expected 200. |
| Same Gates / `124756873741`, bundle:07 | CVS WebKit: BASE per FIX5, unchanged, no exemption. |

Evidence: `output/FEAT-LIVE-REPLAY/FIX5.md`, `red.log`, `fix5-red-*.log`. Exact new head and workflow IDs go in a PR comment. GREEN pending; author PASS E0, local static readiness E1, prior CI failures E3 only. No extra RED push is needed for these already reproduced directed failures. Do not merge until the PR's checks and independent privacy review pass.

Static exit 0: go build and go vet owning Go packages; go vet ./tests/foundation; gofmt; node --check pr-modes; packet structure check; ownership generation then --check; depmap; git diff --check. No local tests/servers ran.

FIX5 outside-original-write-path support files, explicitly approved by its scope: `scripts/dev/pr-modes.mjs` (one entry only), `tests/foundation/published_storefront_test.go` (additive exact ACL/definer expectations), `tests/foundation/live_claims_schema_test.go` (six exact columns), generated `output/ci-select-backend-browser/browser-ownership.json`. No new unapproved source path.

Earlier explicitly granted support already in this PR: buyer dispatcher closure entry in `contracts/route-table-dispatchers.json` (review the entire closure; `output/buyer-route-table/FEAT-LIVE-REPLAY-FIX2-pin-candidate.md`), exact route-count expectation in `scripts/dev/contractdrift/go_extract_test.go`, replay tests in `tests/foundation/live_replay_test.go`, `internal/integrations/metareply/live_replay_test.go`, `internal/buyerhttp/routes_test.go`, and the assigned R2 pin in `tests/foundation/r2_integration_upgrade_test.go`. FIX5 does not edit these paths, the dispatcher pin or baseline. No migration number, OpenAPI, module or lock-file change this batch.

Risk: least-privilege SQL and the complete replay flow still need exact-head REAL_PG CI and the assigned independent privacy reviewer. Unrelated CVS WebKit failure may still block the aggregate. NOT_RUN: local tests/servers (prohibited), post-fix CI completion, independent E4/privacy review, SANDBOX/LIVE, browser/UI parity, merge and deployment. Integrator completion relay resumes this card; no merge/auto-merge/status post. Rollback this batch by reverting only the FIX5 commit; preserve earlier local evidence.

🤖 Generated with Codex (dispatched by Claude Code integrator).
