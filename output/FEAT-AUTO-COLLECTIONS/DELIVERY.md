# FEAT-AUTO-COLLECTIONS — relay 1 implementation checkpoint

Date: 2026-10-30. Role: dispatched implementation lane, no recursive delegation.
Runtime: Codex; actual model/effort identifier is not exposed in the runtime and is UNKNOWN.
Branch: unit/feat-auto-collections. Worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/feat-auto-collections.
Base: 3ea359f796d36153bb4b8aa7259e4e0c27676b86. PR: #371 into r3/integration.
No integration/main writes, merge, auto-merge, force-push, status posting or deployment.

## Authority and scope

Read AGENTS.md, 架构.md §§0,1,2,26,27, contracts/tasks.json T04 ownership,
contracts/storefront-v2.md §A, catalog collection commands/HTTP transport/BFF grammar,
catalog schema 0002/0086/0093/0149 and current R2 upgrade pin.
Fleet Bridge read-only status and get_design_context refreshed frames 6:19278 and 6:19486.
The desktop explicitly shows creation <7 days, tag and source. Schema has created_at but
no tag/source. Time rules implemented; tag/source DEFERRED. Existing sort modes retained.
No UI pages/design writes; UI delivery and its design process remain PARITY-NEWFEAT.

Paths: internal/catalog/collections.go, internal/httpapi/collections.go,
contracts/storefront-v2.md, contracts/catalog-inventory-openapi.json,
migrations/0212_collection_rules.sql, existing apps/admin/app/api/stores/[store]/[...resource]/route.ts
(comment documenting existing automatic JSON forwarding; no route/allowlist expansion).
Required tests and generated browser ownership were already pushed in RED.
Evidence files live under output/FEAT-AUTO-COLLECTIONS. depmap regenerated with no resulting diff.
No files in another module changed; no baseline/exemption/gate-adapter edits.

## Implementation

Validated all|any rules with 1..20 created_at within_days conditions (integer 1..3650).
Forward-only two-column migration inherits collection FORCE RLS and exact role grants.
Pure SQL timestamp predicate is shared by merchant preview/count and public filter/count/detail.
Public origin publication, store, active product and active store-currency SKU fences retained.
Manual members remain stored when switching modes; versioned atomic PATCH saves the switch.
Automatic list PUT returns conflict; retry uses existing scoped command ledger and saved result.
Automatic default manual sort resolves to newest; retained manual ranks never influence it.
Existing public JSON shape/image roles and media rendition enrichment remain intact.
No materialization, worker/job, dependency or index (scoped PK prefix already exists; no plan evidence).

## Evidence

E3 causal RED: head 7cdb89bda0a2684cb6f0d70cd8fa4103ffe51d90, base 3ea359f796d36153bb4b8aa7259e4e0c27676b86,
synthetic merge 37938521d57438bc7cda00004a6f6e6b523afed0.
Gates 41055677410, attempt 1, shard:g10 job 124069013163 FAILED all five new subtests
of TestCatalogCoreAutomaticCollections on HTTP 400 versus required 200 (mode/rules unsupported).
Raw receipt red-g10.log fetched using GitHub job logs API while workflow-wide --log-failed was unavailable.
Foundation 41055677331/job 124062480221 passed its structural selection; not product acceptance.

E1 static only, exit 0: gofmt changed Go files; GOTOOLCHAIN=go1.27.3 go vet
./internal/catalog ./internal/httpapi ./tests/foundation; python3 scripts/check_packet.py
(PASS_PACKET_STRUCTURE_ONLY, saved packet-check.json); bash scripts/dev/depmap.sh;
node scripts/dev/derive-browser-ownership.mjs and --check; git diff --check.
check_packet's tracked output restored to HEAD after retaining its receipt in this task directory.
No local test/server/PG/browser execution occurred. All behavioral checks belong to GitHub CI.

## Blocking allocation

The card assigns migration 0212 but provides NO R2 count pin. The owner explicitly prohibits
choosing that pin. tests/foundation/r2_integration_upgrade_test.go currently expects 139;
adding the assigned migration changes the set. The pin has not been edited or guessed.
Integrator assignment requested through the session's asynchronous question; no response yet.
Implementation push held to avoid a knowingly invalid gate and wasting the fleet CI batch.
Implementation GREEN/SQL runtime validation, independent review, local tests/servers/PG/browser,
production/LIVE and Humaux store/canvas/coord are NOT_RUN. Humaux tools are not available.
No new WAIT_RUN until the implementation can be pushed with the assigned pin.
