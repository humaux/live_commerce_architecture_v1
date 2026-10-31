# FEAT-AUTO-COLLECTIONS-FIX1 — PR #371

Date: 2026-10-31. Implementation lane: Codex, model GPT-6 (runtime-provided family;
exact model/effort identifier UNKNOWN). No delegation. Worktree:
`/Volumes/data/live_commerce_architecture_v1/.worktrees/feat-auto-collections`.
Branch `unit/feat-auto-collections`; target `r3/integration`.

## Authority and current source

The FIX1 human card and its 2026-10-31 integrator ruling authorize migration 0212,
the storefront-v2 section A amendment, collection handlers/SQL/BFF, and the R2 pin
as base migration count plus this card's files. Read repository AGENTS.md,
架构.md sections 0, 1, 2, 26, 27, contracts/tasks.json, storefront-v2 section A
and its automatic-membership amendment, and directly affected code/callers.
Integration base `a3dc78e4d5ec72831aaf6a376793f0f6a7a56425` was fetched and merged
before FIX1 edits. Its R2 glob includes 139 SQL files; this branch adds exactly
`0212_collection_rules.sql`, so the exact-count assertion becomes 140.
The base Gates run `41105181453` was still in_progress when inspected; base GREEN
is UNKNOWN. No class (c) claim is made.

Existing remote head `7cdb89bda0a2684cb6f0d70cd8fa4103ffe51d90` contains tests
only. Existing local implementation `70679c051fbcdc7ec5304a62876f596f7a8c3ea5`
had never been pushed because the R2 pin allocation was blocked. This ruling
resolves that blocker. This is the first implementation push for these assertions,
not a third repair attempt. No tests or fixtures are rewritten.

## Failing-job classification (raw GitHub logs, E3 RED only)

Run `41055677410`, attempt 1, RED head `7cdb89bda0a2684cb6f0d70cd8fa4103ffe51d90`,
tested synthetic merge `37938521d57438bc7cda00004a6f6e6b523afed0`.
The exact test names below are children of `TestCatalogCoreAutomaticCollections`.

| Failing job | Failing test / check | Evidence | Class and directed resolution |
| --- | --- | --- | --- |
| 124069013163 — gate (shard:g10) | all_any_read_time_sort_and_no_materialization | collection_rules_test.go:53, POST /collections status=400 want 200; 2026-10-30T12:49:50.0233575Z | (b) missing product mode/rule input and SQL read-time membership; existing implementation adds validation and shared SQL predicate without copied members |
| 124069013163 — gate (shard:g10) | buyer_visibility_count_detail_and_unpublished_origin | collection_rules_test.go:83, POST /collections status=400 want 200; 2026-10-30T12:49:50.4706270Z | (b) missing automatic collections; existing SQL implementation preserves origin/product/SKU fences and derives public filter/count/detail consistently |
| 124069013163 — gate (shard:g10) | mode_switch_preserves_manual_list_and_versions | collection_rules_test.go:158, mode PATCH replay=400/400, identical=false; 2026-10-30T12:49:50.9220246Z | (b) missing versioned mode/rule PATCH; existing implementation retains manual rows and uses the existing transaction/idempotency ledger |
| 124069013163 — gate (shard:g10) | create_retry_scope_and_staff_permissions | collection_rules_test.go:183, scoped automatic create/replay=400/400, identical=false; 2026-10-30T12:49:51.3157925Z | (b) missing automatic create input; existing HTTP wire accepts rules, discards body scope fields and retains server authentication/permissions |
| 124069013163 — gate (shard:g10) | validation_is_atomic_and_unknown_rules_are_refused | collection_rules_test.go:221, POST /collections status=400 want 200; 2026-10-30T12:49:51.6968578Z | (b) valid setup request rejected before semantic assertions; existing bounded grammar returns 422 for unsupported rules and rolls back refused writes |
| 124084325791 — receipts | Every planned mode has one fresh PASS receipt with tests | 2026-10-30T13:19:22.4328310Z: FAIL: shard:g10: status FAIL; matrix-receipts: RED (1 problem(s)) | downstream of g10; receipt code unchanged |
| 124084702834 — required | required status aggregation | 2026-10-30T13:21:25.5434354Z: gate=failure, receipts=failure; aggregate=skipped (expected false) | downstream of g10/receipts; required code unchanged |

The parent test FAIL summarizes its five children. All other run jobs passed or
were skipped; aggregate was skipped, not failed. No browser assertion failed in
this RED run, so no browser artifact is relevant to these backend assertions.
No class (a) rewrite, deletion, skip, threshold/fixture/exemption change, or
class (c) base-failure exemption is used. Raw filtered receipts: `fix1-red.log`;
the committed tests-only RED record remains in `red.log`.

## Changes and acceptance gate

The existing implementation handles the root input/SQL gap. FIX1 completes its
authorized exact R2 count pin, retains explicit source headers, merges integration,
and regenerates browser ownership and depmap from the current source. Acceptance
is the PR's own Gates and Foundation CI on the new head, including all five
unchanged new subtests, legacy catalog/isolation gates, and R2 upgrade gate.
Local behavioral tests, servers and database execution are forbidden and NOT_RUN.
If a listed assertion remains red after this directed batch, report the exact
assertion and evidence as BLOCKED; do not attempt a third approach.

## Scope inventory

Authorized product paths: migrations/0212_collection_rules.sql,
contracts/storefront-v2.md section A amendment, internal/catalog/collections.go,
internal/httpapi/collections.go, existing admin collections BFF route.
Additional paths explicitly required by the card/preamble: unchanged RED test
tests/foundation/collection_rules_test.go, tests/foundation/r2_integration_upgrade_test.go
(pin ruling), generated output/ci-select-backend-browser/browser-ownership.json,
and docs/engineering/dependency-map.md if regeneration changes it.

Inherited implementation commit also changes contracts/catalog-inventory-openapi.json;
this is outside the FIX1 explicit write paths and needs integrator approval before
merge. FIX1 does not edit it. Existing output/FEAT-AUTO-COLLECTIONS delivery,
red.log and source-SHA256.txt are inherited artifacts; pre-existing dirty/untracked
files there are excluded from this batch's new commit, with one exception noted
below. This filesystem resolves output/feat-auto-collections to the existing
output/FEAT-AUTO-COLLECTIONS directory; the latter remains the canonical Git path.
FIX1.md and fix1-red.log use that existing task output root.

Evidence-file handling correction: the initial FIX1 log download mistakenly
overwrote the already-dirty red.log because those directory spellings alias.
The committed red.log was recovered from Git and the new raw receipts were
moved to fix1-red.log. The exact prior uncommitted red.log bytes could not be
recovered. Original CI remains available; no product/test source was affected.

## Evidence boundary and handoff

E3: causal RED only, read from GitHub original logs. Implementation self-review
is E0; static checks provide E1 only. GREEN and independent acceptance PENDING.
No local tests/servers/PG/browser, production/LIVE, merge, auto-merge, force-push,
commit statuses or deployment. Humaux search/store/canvas/coord NOT_RUN because
no Humaux tool is callable in this session; this file is repository evidence,
not a substitute claim that memory or canvas was updated.

Static-check commands/results and the exact pushed-head CI run IDs are appended
to this file/PR comment at handoff. No evidence-only push after GREEN.

Static checks: all exited 0 on the source hashes in fix1-static.log: go vet of catalog/httpapi/foundation; gofmt -l (empty); diff whitespace checks; browser ownership generator (54 modes) and --check; depmap generator and --check (no diff). E1 only.
