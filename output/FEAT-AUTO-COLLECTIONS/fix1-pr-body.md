FEAT-AUTO-COLLECTIONS-FIX1 implements saved automatic collection rules after the tests-only RED checkpoint. Merchant create/PATCH accepts bounded all/any creation-time rules; SQL evaluates membership at read time for preview, buyer filters, counts and product-detail collections. Mode changes preserve the manual list, optimistic versions, authenticated scope and keyed retries. No UI changes; UI remains pending PARITY-NEWFEAT.

Merged `origin/r3/integration` at `a3dc78e4d5ec72831aaf6a376793f0f6a7a56425` first. The 2026-10-31 integrator ruling assigns the R2 pin as current base count plus this card's migration: **139 + 0212_collection_rules.sql = 140**. The exact assertion stays strict. Existing local implementation `70679c051` had never been pushed while this ruling was pending; FIX1 is its first implementation push.

RED (E3, causal failure only): [Gates 41055677410 / g10 job 124069013163](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41055677410/job/124069013163), head `7cdb89bda0a2684cb6f0d70cd8fa4103ffe51d90`, tested merge `37938521d57438bc7cda00004a6f6e6b523afed0`. All five new `TestCatalogCoreAutomaticCollections` subtests failed their own HTTP assertions: create status 400 versus 200, mode PATCH replay 400/400, scoped create replay 400/400. All are class (b) missing product behavior; no class (a) test rewrite or class (c) exemption. Receipts job `124084325791` reports only `shard:g10: status FAIL`; required job `124084702834` fails downstream. Aggregate was skipped. Full failing-job table: `output/FEAT-AUTO-COLLECTIONS/FIX1.md`; filtered original logs: `fix1-red.log` in the same directory.

GREEN: PENDING on this PR's new head. Exact Gates/Foundation run IDs will be posted in the handoff comment. Acceptance requires all unchanged automatic-collection tests, existing catalog/isolation tests, R2 upgrade and the PR's required checks. No GREEN or independent acceptance is claimed. Base run `41105181453` was still in_progress when checked, so no base-green assertion is used.

Local E1 only: `GOTOOLCHAIN=go1.27.3 go vet ./internal/catalog ./internal/httpapi ./tests/foundation`, `gofmt -l`, `git diff --check`; required browser-ownership generation and `--check`; depmap regenerated (no resulting diff). No local behavioral tests, servers or database execution. Evidence is in `FIX1.md` and `fix1-static.log`.

Outside the explicit FIX1 product write paths:

- `tests/foundation/collection_rules_test.go`: inherited tests-only RED coverage, unchanged by FIX1.
- `tests/foundation/r2_integration_upgrade_test.go`: pin update explicitly authorized by the integrator ruling.
- `output/ci-select-backend-browser/browser-ownership.json`: required generated ownership, refreshed after merging integration and changing the pin source hash.
- `contracts/catalog-inventory-openapi.json`: inherited from implementation commit `70679c051`, not edited by FIX1; **needs integrator approval before merge** because it is outside FIX1's explicit paths.
- `output/FEAT-AUTO-COLLECTIONS/*`: inherited delivery/hash/RED evidence plus this handoff table and logs. The filesystem aliases the lowercase path requested by the card to this existing canonical Git path. A mistaken refresh of the pre-existing dirty red.log was corrected by restoring its committed version and moving new receipts to fix1-red.log; exact prior uncommitted bytes were not recovered. Product/test source was unaffected.

NOT_RUN: local tests/servers/PG/browser, runtime SQL validation outside CI, production/LIVE, independent review, GREEN acceptance, Humaux search/store/canvas/coord (tools unavailable). No test, fixture, threshold, exemption, gate-adapter assertion, baseline or dispatcher list was weakened. No merge, auto-merge, force-push, commit-status posting or deployment.

🤖 Generated with Codex (dispatched by Claude Code integrator).
