# CI-DRIFT delivery

## Current batch — PR #33 round 1b (2026-10-10)

- **READY for integrator pre-review**, branch `unit/ci-contract-drift`, base `45ef5641`; frozen tested source **`df9bf2b6b53c17ca2fdf767586deb756dc4476a5`**. Integrator trunk merge and setup-go v6.5.0 retained. No push. Implementer Codex-3; exact parent runtime model/reasoning is unavailable. Isolated Go/BFF workers and read-only reviewer; no recursive delegation. Per-file source hashes and worker configurations are retained in raw task evidence.
- Allowed writes: `scripts/dev/contractdrift/**`, `docs/delivery/GATES.md`, this DELIVERY and unit-local curated static evidence. No final product, contract, workflow, dependency or lockfile change; baseline **237**, byte-for-byte unchanged. LC-U3 remains explicitly paused. This delivery commit adds evidence only; tested source remains unchanged.

| Packet item | Root cause → fix / prior coverage | Actual CLI regression |
| --- | --- | --- |
| 1 raw Git provenance | Attributes/config suppressed hunks → force raw text/no textconv, fixed a/b prefixes and independent changed-source enumeration; missing raw provenance fails closed. | `TestReviewR1BForcedTextDiffCLI` (binary, textconv, mnemonic, no-prefix) |
| 2 pure deletions | Current-only spans ignored deleted lines → use actual merge-base spans and surviving line mapping; preserve legitimate complete removal and unrelated unknowns. Duplicate-signature regions cannot mask a new unknown. Ordinary and baseline-write paths share the check. | `TestReviewR1BDeletedUnresolvedCLI`, `TestReviewR1BDeletionFixLeavesUnrelatedUnknownCLI`, `TestReviewR1BDeletionCreatesSecondUnknownCLI` |
| 3 methodless Handle | **Already covered by r1**: unproven children are unknown, not mounts/notes. Added inline Handle/HandleFunc equivalence and transparent net/http.HandlerFunc pure405 proof; first200/unknown status remain errors. | existing `TestGoReviewMethodlessHandleCLI`; new `TestGoReviewR1BInlineMethodlessCLI`, `TestGoReviewR1BHandle405BodyCLI` |
| 4 unknown range tail | append discarded Unknown, range visited only known items → retain unknown alternative with expression/call provenance alongside known routes. | `TestGoReviewR1BUnknownRangeTailCLI`, `TestGoUnknownRangeTailRetainsKnownRoutesAndProvenance` |
| 5 buyer method | Admission replaced actual fetch method → prove passthrough, otherwise use fixed/default GET or report unknown; require actual transport and sink. | `TestReviewR1bBuyerTransportRealCLI` (DELETE, dynamic, omitted, removed function/fetch) |
| 6 extra leaf sinks | Specialized adapters returned before extras → consume individual proven calls, then scan remaining reached calls after export filtering. Extra POST is not clipped by exported GET. | `TestReviewR1bSpecializedExtraSinksRealCLI` (reports/tools/buyer/import/picklist, fixed POST/dynamic URL) |
| 7 P1 sink values | Name-only literal-call scans lost aliases → uniform reached-binding graph and unknown sink-value rule; aliases, wrappers, members, spread/higher-order, imports/reexports/workspace and proxy are unknown unless proven. | `TestReviewR1bSinkValuesRealCLI`, `TestReviewR1bImportedSinkValuesRealCLI`, `TestReviewR1bAdminProxySinkValuesRealCLI`, `TestReviewR1BGlobalNamespaceSinkRealCLI` |

- Added defenses from independent/parent review: runtime definitions take priority over erased types; shared/cyclic graphs preserve caller provenance without recursive diagnostic expansion; workspace export manifests join source spans and immutable snapshots. `TestReviewR1bTypeValueDualBindingRealCLI`, `TestReviewR1bSharedCyclicSinkGraphRealCLI`, `TestReviewR1bWorkspaceSinkBindingsRealCLI`, `TestReviewR1bWorkspaceManifestRetargetRealCLI`.
- The reached merchant transport has an individual URL/path/init passthrough proof, not a helper/module exemption. Config and URL-object escape/mutation, dynamic URL, method override and path reassignment fail closed. `TestReviewR1bMerchantTransportProofRealCLI`, `TestReviewR1bOriginObjectEscapeRealCLI`. Existing real-source fixtures copy actual workspace exports as well as apps; assertions retained.
- **Red → green:** curated `review-round1b/` and its `red-log-index.json` preserve compiled OS CLI failures and hashes of full raw logs. Item3's r1 coverage was already green; it was not presented as a newly reproduced bug. Initial buyer mutation's incorrect text anchor is superseded by authoritative `final-base-red.log`. The intermediate graph/config false positives and failed child race remain raw historical evidence, never counted green; parent final race passed. Temporary child snapshot overlay was restored exactly before its BFF-only commit.
- **Actual repository P1 proof at final source:** requested alias family, reached imported alias and proxy alias were checked through compiled CLI. Imported/proxy probes previously escaped with exit0 at `6012f01b`; same probes now exit **1**, exact restoration exits **0**, original/restored hashes equal. Each ERROR UNRESOLVED includes the changed producer file. See `review-round1b/real-alias-mutations.json`; no product mutation remains.

| Final local gate at df9bf2b6 | Result |
| --- | --- |
| `GOFLAGS=-p=1 go test -race -count=1 -timeout=300s -v ./scripts/dev/contractdrift` | 0; **64** top-level passes |
| `bash scripts/dev/test-node.sh` | 0; **1323** passes |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 |
| `bash scripts/dev/check-gates.sh` / `go vet ./scripts/dev/contractdrift` | 0 /0 |
| `bash scripts/dev/check-pkgdocs.sh` / `bash scripts/dev/depmap.sh --check` | 0 /0 |
| `bash scripts/dev/test-local.sh --list` | 0; **83**, unchanged versus base |

Evidence **E3 STATIC**, source hash **`48e1035694d1554075f1b7e655d637ad43190800621d2a7688a4115191075877`**. All four final runners recorded unchanged source, tracked status and historical-output hash. Normal CLI: Go278 / contracts146 / BFF292, errors0 / warnings362. Index: `review-round1b-evidence.json`; raw evidence: `output/playwright/ci-contract-drift-r1b/`. Independent exact-repair read-only review at df9bf2b6 found no remaining scoped finding (E1; parent owns E3 tests).

### CI gates / NOT_RUN

Required plan job still runs the CLI with the PR base. `node scripts/dev/pr-modes.mjs origin/r3/integration` selects **52** modes; exact list is in `review-round1b/pr-modes.json` and the evidence index. Browser modes and foundation/G07 remain GitHub-only per preamble; API/PG/provider runtime, SANDBOX/LIVE and deployment NOT_RUN. Static proof covers documented finite source forms and assumes standard platform intrinsics; it does not evaluate arbitrary JavaScript or certify runtime behavior. Integrator runs K3 pre-review/CI and pushes.

Own gate processes finished; Go/BFF child worktrees removed after preserving logs/commits. Integrator's untracked `output/ext-agents/` remains untouched.


## Historical batch — PR #33 round 1, superseded by round 1b

- **READY for integrator review**, branch `unit/ci-contract-drift`, base `6023b55a`; source commits `ce5860a3`, `3febbff5`, **`5f64bd2853b9ae0bb431f471436201f05d703b82`**. Synced integrator merge `de592635` and required-plan wiring `6023b55a`, both preserved. No push. Implementer Codex-3; parent model/effort unavailable in this runtime; isolated Go worker and read-only reviewer explicitly `gpt-6.1-sol/high`.
- Allowed writes: `scripts/dev/contractdrift/**`, `docs/delivery/GATES.md`, this unit's DELIVERY and curated evidence. Temporary actual product-source/BFF mutations were restored byte-for-byte; no final apps/internal/contracts/module/lock/workflow diff against `6023b55a`. No new dependency, mode, contract or baseline entry; baseline **237** unchanged. LC-U3 remains owner-paused with its seven uncommitted files and checkpoint preserved.

| Finding | Root cause and final behavior | Real CLI regression |
| --- | --- | --- |
| P1 methodless Handle | Handle skipped unknown classification; unproven children now UNRESOLVED MOUNT, new/touched sources fail and unchanged mounts warn. Unknown `/` cannot establish child coverage. | `TestGoReviewMethodlessHandleCLI`: new, legacy, touched registration and child dependency |
| P1 BFF deletion/retarget | Current-only aggregate could hide a formerly exposed route; compare actual producer file + upstream METHOD/path with merge-base, before dedupe. BFF_REMOVED always errors and cannot be seeded; full joint Go+contract retirement permitted. Backend-only routes stay INFO. | `TestReviewBFFRemovedRealCLI`, `TestReviewBFFRetargetRealCLI`, `TestReviewBFFDuplicateProducerAndJointRetirement` |
| P1 diff ownership spoof | Hunk content impersonated file headers; only `diff --git` starts a file header block, and hunk `---`/`+++` content is ignored. | `TestReviewDiffHunkHeadersRealCLI` |
| P2 suffix fallback | `*route("")` naming suppressed real routes; finite actual first-WriteHeader(405) body/status flow required, including imported responders and dependencies. Unknown factory remains a business route. | `TestGoReviewRouteSuffixCLI`, `TestGoReviewFactory405CLI`, `TestGoReview405ProofCLI` |
| P2 aliases | Bound ServeMux method-value calls silently vanished; finite alias/same-package argument propagation reports UNRESOLVED with assignment/call/argument provenance. | `TestGoReviewMethodValueAliasCLI` |
| P2 colon | Segment-internal colons collapsed distinct literals; only whole `/:name` segments normalize. | `TestGoReviewColonParameterCLI`, `TestGoReviewLiteralColonCLI` |

- Additional 405 proof guard: header arguments committing200 and local decorator-name shadowing each produced erroneous CLI0. `TestReview405DecoratorFirstWriteAndShadowCLI` is red→green; only typed pure header operations and the correctly bound delegate can preserve405. Assertions were retained; old opaque dispatch checks now additionally require the newly reported mounts.
- All six findings have **actual OS CLI red tests**. `review-round1/` contains their red logs and final Go green log. Three P1 **actual repository** mutations at final source: add the requested methodless NotFoundHandler registration; remove actual `apps/admin/app/api/stores/route.ts`; inject spoof hunk ownership and touch an unchanged-key legacy call in actual `internal/httpapi/handler.go`. Each gate exits **1**, then exact restoration exits **0** with equal original/restored hashes. `review-round1/real-mutations.json` and per-case JSON bind those results to `5f64bd28`.

| Local gate at 5f64bd28 | Result |
| --- | --- |
| `GOFLAGS=-p=1 go test -race -count=1 -timeout=240s -v ./scripts/dev/contractdrift` | 0, **43** top-level tests |
| `bash scripts/dev/test-node.sh` | 0, **1323** passes |
| `cd apps/admin && pnpm exec tsc --noEmit -p .` | 0 |
| `bash scripts/dev/check-gates.sh` | 0 |
| `go vet ./scripts/dev/contractdrift` | 0 |
| `bash scripts/dev/check-pkgdocs.sh` / `bash scripts/dev/depmap.sh --check` | 0 /0 |
| `bash scripts/dev/test-local.sh --list` | 0, registry unchanged |

- Evidence **E3 STATIC**, source hash **`29dbde62e8d012c855a340900dd876d0b8a5f70cfaede0a3fbd1e5774cd538ff`**; every final gate records unchanged source, tracked status and historical-output hash. Current inventories: Go278, contracts146, BFF292, errors0/warnings362. Root-local admission memo fixes7 formerly missed BFF tuples; baseline remains237. Curated index `review-round1-evidence.json`; raw task-local logs in `output/playwright/ci-contract-drift-r1/`.
- Read-only parent ratchet/diff review at `ce5860a3` found no scopedP0/P1, E1 only. Final independent K3/CI remains integrator-owned. The Go worker's check-gates stopped for missing `typescript-api` in its isolated worktree; parent's installed worktree gate passed. One extra fixture had a brace setup error, corrected before its green run; it is not counted as causal red evidence.
- Boundaries: unproven child muxes remain unknown; finite body proofs deliberately fail closed. Producer identity is file + upstream METHOD/path, not arbitrary browser-export semantics. No claim of API runtime behavior or provider acceptance.

### CI gates / NOT_RUN

Required `gates.yml` plan job continues running the drift CLI with the PR base SHA and propagating exit1. `node scripts/dev/pr-modes.mjs origin/r3/integration` selects **52** modes because shared gate/workflow paths changed; exact list is in `review-round1-evidence.json`. Browser modes, full foundation/G07, API/PG/provider/SANDBOX/LIVE and deployment are NOT_RUN locally for this static-tool batch. Integrator runs K3 pre-review/CI and pushes. No processes left running.


## Prior delivery — historical, superseded by the round 1 batch below

- **READY**; branch `unit/ci-contract-drift`, worktree `.worktrees/ci-contract-drift`, base `7975160e`. Tested source commit **`00791b32321db0972200fc05b2b75102c9d5e928`**. Parent runtime model/effort is not exposed; source-first Go/BFF workers and independent core reviewer explicitly used `gpt-6.1-sol/high` (Go inventory explorer medium). Integrator pushes/reviews; no push or deployment performed.
- Zero external dependencies. `go/parser`/AST resolves scoped constants/concats, aliases, same-package wrappers and finite loops; no go.mod/go.sum/lockfile change. TS lexer/resolver enumerates actual BFF method tables/grammars/forwarders independently of Go candidates, with one mapping table in `bff.go`. Contracts use JSON positions and absolute MD v1 routes; a preceding `Route base: /v1/...` is mandatory for relative forms. Every finding has file:line provenance.
- `baseline.json` records **237** legacy mismatches. Initial loading and seeding require merge-base findings; subsequent baseline keys only shrink. New, touched legacy, stale and added baseline entries exit1. Current and deleted positions use the merge-base's real source snapshot (including module scope), not stale baseline line numbers; committed/staged/working/untracked changes are covered. Changed unknown source fails. Methodless405 and mux mounts are notes. Opaque custom Go dispatch reports UNRESOLVED, never a claimed real404.
- Wired tool tests and gate into `check-gates.sh`; no registry mode added (82 unchanged). `gates.yml` PR matrix and `foundation.yml` already fetch full history, so no workflow edit needed. Missing base ref fails closed. Generated dependency map updated.

| Actual source inventory at base7975160e | Count |
| --- | ---: |
| Go METHOD/path | 278 |
| Contract METHOD/path | 146 |
| BFF METHOD/path | 285 |
| GO_ONLY | 200 |
| CONTRACT_ONLY | 33 |
| BFF_NO_GO | 0 |
| METHOD_MISMATCH | 4 |
| GO_NOT_IN_BFF (info) | 30 |
| UNRESOLVED (warning, custom dispatch/relative or unsupported docs) | 119 |
| Mux mount/fallback notes | 88 |
| Explicit405 fallback notes | 4 |
| Blocking errors / warnings | 0 /356 |

## Tests and evidence

- `GOFLAGS=-p=1 go test -race -count=1 -v ./scripts/dev/contractdrift` →0, **31 top-level tests**. `fixture-red-final.log` captured the unimplemented CLI failing before implementation; `all-final-green.log` is final green. Fixtures cover every drift kind, AST concat/scoping/wrappers/loops, normalized IDs, actual BFF source mutation, unknown forms, legacy/touched/stale/shrink/growth, missingbase, deleted-baseline/initial loaded-baseline attacks, shifted deletion, shared MD base, modular worker isolation, and multiline JSON changed-value provenance. Intermediate review counterexamples each saved RED→GREEN; assertions were retained.
- **Real repository mutation at final source00791b32:** temporarily register `GET /v1/__ci_drift_probe` in actual `internal/httpapi/handler.go` → **1**, `ERROR (NEW) GO_ONLY ... handler.go:113`; restore exact original bytes → **0**. `final-mutation-{red,green}.log` and `final-mutation-proof.json` show identical before/after Go hash `e0a69e92…`. No runtime handler or service was executed, no fixture surrogate stood in for real source.
- `bash scripts/dev/check-gates.sh`, `go vet ./scripts/dev/contractdrift`, `bash scripts/dev/check-pkgdocs.sh`, `bash scripts/dev/depmap.sh --check` →0 at final source00791b32. Full source hash **`02570eace0311555e0729563d336a25c6cbcdcd540eb8de331d35c9675a9e659`** unchanged before/after; tracked historical evidence unchanged, git clean. `final-static.{log,status.json}`.
- `bash scripts/dev/test-node.sh` →0, **1313 tests**; admin `pnpm exec tsc --noEmit -p .` →0 at60c6a603. Subsequent changes are this Go tool's provenance/baseline logic and comments only; all Node/admin application inputs are unchanged. `node/tsc.{log,status.json}`. Offline frozen install changed no dependency declaration/lock.
- Raw evidence is preserved under **`/Volumes/data/live_commerce_architecture_v1/output/ci-contract-drift/evidence/`**, also task-local ignored `output/playwright/ci-contract-drift`. Curated commit-bound metadata `output/ci-contract-drift/evidence.json`. First check-gates failed3 missing Used-by headers; fixed them, final gate0. Parser false positive parameter-only UUID import admission fixed via source regression; it is not grandfathered in baseline.

## Boundaries and review

Evidence **E3 STATIC**, real repository/source/Git and automatic tests; synthetic fixture trees. This does not certify API runtime behavior. Independent read-only core review found no further knownP0/P1 at00791b32 after reported counterexamples were fixed; external K3 and CI remain integrator-owned. Supported finite forms are documented in parser headers. Complex dynamic/custom handlers stay UNRESOLVED; editing those sources requires resolving that coverage rather than suppressing the gate. This batch fixes no legacy product routes/contracts.

Allowed writes: `scripts/dev/contractdrift/**`, `scripts/dev/check-gates.sh`, `docs/delivery/GATES.md`, generated `docs/engineering/dependency-map.md`, unit delivery/evidence. Temporary real Go mutation was restored; apps/contracts/production Go/module/locks/workflows have zero final diff.

## CI gates / NOT_RUN

`node scripts/dev/pr-modes.mjs origin/r3/integration` selects52 modes due to shared gate/tool wiring; complete exact list is in `evidence.json.pr_modes`. These CI/browser modes, full Go/PG/API runtime, providers, SANDBOX/LIVE and deployment are **NOT_RUN locally**; no runtime app change. CI already invokes check-gates, which runs the new31 tests and progressive drift gate. Independent K3/PR/push are the integrator's next step.
