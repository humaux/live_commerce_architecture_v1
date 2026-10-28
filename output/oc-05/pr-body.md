Implement OC-05 server quick start with assigned migration 0205: existing stores are backfilled as completed; new stores get truthful derived steps and persistent, idempotent completion/dismissal. Scope and immutable completion timestamps remain server controlled. UI belongs to OC-06.

GET-422 fix (attempt 1 for this blocker): merge current integration, construct genuinely bodyless GET test requests, and send {} only for POST in cross-scope cases. The strict production GET-body refusal stays intact; add an empty/nonempty present-body rejection test. Every existing 200/404/domain assertion is retained.

RED: original tests-only head 5da45aea079019ed17d12368a80e4b7183dfb368, Gates 40619573790 / shard:unit job 122603618623 (HTTP 404 and missing migration assertions). Subsequent blocker: Gates 40650048011 / 122733463849, five OC05 GET assertions returned 422; read failed receipts 122843044695 and required 122909447868 too (both downstream). Filtered raw evidence: output/oc-05/red.log and fix-get-body-red.log.

GREEN: pending this push's own Gates/Foundation. Current run/job ids will be recorded in a PR comment without an evidence-only push. Static E1: go build and go vet exit 0; gofmt and generated depmap exit 0, map unchanged. Source hash and commands in output/oc-05/fix-get-body-static.log. No local tests or servers. Author PASS = E0; independent review, runtime acceptance and sandbox/live/production NOT_RUN. Integrator will resume this lane on CI completion.

This targeted fix adds no out-of-card files. Existing PR scope outside contracts/tasks.json write_paths, for integrator approval/review: contracts/merchant-settings-wizard-v1.md, internal/httpapi/handler.go, tests/foundation/r2_integration_upgrade_test.go (assigned pin 135), tests/quickstart/doc.go; scripts/dev/pr-modes.mjs (previously authorized one-line backend classification). docs/engineering/dependency-map.md uses the generated-map allowance. No further scope expansion.

Humaux tools unavailable; store/canvas/coord updates NOT_RUN. No merge into integration/main, auto-merge, force-push, statuses or production actions.

🤖 Generated with Codex (dispatched by Claude Code integrator).
