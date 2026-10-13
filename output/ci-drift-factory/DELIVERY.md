# CI-DRIFT-FACTORY

Author record; runtime acceptance is pending GitHub CI and independent review.

## Scope and provenance

Base: `256e9740ced9c5de05a83f58eb9f4c79a231b7b2` (`origin/r3/integration`). Branch: `unit/ci-drift-factory`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory`.

| Participant | Role / model / reasoning | Worktree and allowed writes |
| --- | --- | --- |
| Author | Integration / `gpt-6.1-sol` / `ultra`; model and reasoning read from the local Codex configuration | Root worktree above; `scripts/dev/contractdrift/**`, `output/ci-drift-factory/**` |
| `drift_factory_boundary` | Read-only explorer / `gpt-6.1-sol` / `high`, explicit dispatch | Root worktree; no writes |
| `drift_factory_tests` | Independent test worker / `gpt-6.1-sol` / `high`, explicit dispatch | `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory-tests`, branch `unit/ci-drift-factory-tests`; initial base above, frozen-OA1 follow-up base `bfcfa5eb`; only `scripts/dev/contractdrift/go_factory_cli_test.go` and the subsequently assigned `go_factory_oa1_cli_test.go` |
| `factory_effect_audit` | Read-only explorer / `gpt-6.1-sol` / `high`, explicit dispatch | Root worktree at `7b652f59`; no writes; bounded effect audit `6939691e-0328-4c74-9145-3b7dbfbb13e0` |
| `factory_boundary_impl` | Integration worker / `gpt-6.1-sol` / `high`, explicit dispatch | `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory-effects`, branch `unit/ci-drift-factory-effects`, base `7b652f59`; only the two new factory/effect scanner files and small `go_opaque.go` / `go_eval.go` integrations |
| `factory_counter_tests` | Test worker / `gpt-6.1-sol` / `high`, explicit dispatch | Tests worktree and branch above, follow-up base `7b652f59`; only new `scripts/dev/contractdrift/go_factory_effect_cli_test.go` |

Delegate model/effort records describe the actual dispatch configuration, not independently measured provider telemetry. No product, migration, dependency, contract, baseline or gate-threshold change is authorized by this unit.

## Reproduction and acceptance

OA-1 PR #53 added a startup database identity check inside `buildBuyerWithCVS`. Its historical Foundation run [38474388216](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38474388216), job `115561367494`, reported 71 TOUCHED unknown-route findings at `a047e48812139cb172794b04c8df13d2f7fea532`. The factory's complete source span enters both opaque dispatch and handler-expression provenance, then spreads to the custom mount's subroutes. Codex-3 supplied the exact reproduction at `316848601032b434885dd513b11f0a8871ef57e5`: [Gates 38475612050 / job 115565420528](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38475612050/job/115565420528) reports the same 71 errors, exit 1. The retrieved original log binds the actual synthetic merge of that source into `256e9740`; see [portable excerpt](evidence/oa1-plan.log).

The second test fixture freezes the complete `ValidateSameDatabase` body and original `buyer.go` call block from that SHA, including the named error, deferred local cleanup, advisory-lock query, fresh Scan destination, and `closePools` failure branch. Source blobs are recorded in the test. The fixture's helper and imports predate the added call block. It retains aligned Go/contract/BFF route counts and EXACT/MOUNT warning coverage, plus counterexamples for captured configuration writes and a fake PGx receiver. The scanner never executes this fixture or contacts a database.

Acceptance uses the existing compiled scanner CLI and Git-backed fixtures. A startup-only validation addition must produce zero touched routes without changing the inventory or baseline. Handler-input changes, routing edits, middleware changes and state/alias mutations must remain TOUCHED. All existing CI-DRIFT cases remain required.

## CI receipts

| Check | Status |
| --- | --- |
| Tests-first scanner regression at `bfcfa5eb` | **RED** — [Foundation 38477346748 / job 115571192630](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38477346748/job/115571192630), actual execution 22:27:46–22:30:42 UTC, 2026-10-13. Both startup-only factory names incorrectly return exit 1/errors 3. The new middleware and child-handler dependency cases also expose false negatives (exit 0). [Portable receipt](evidence/red-factory.log). This is regression evidence, not acceptance. |
| Frozen OA1 probe regression at `7b652f59` | **RED** — [Foundation 38478824210 / job 115576095619](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38478824210/job/115576095619), actual execution 22:37:16–22:40:15 UTC, 2026-10-13. The aligned baseline passes, then adding the literal frozen probe produces exit 1/errors 3. The two toy false positives and both dependency false negatives also reproduce. [Portable receipt](evidence/red-frozen-oa1.log). |
| Full scanner package and `check-gates.sh` | RED at the scanner regression above; later Foundation checks NOT_RUN after the failing scanner batch |
| Required PR Gates / Foundation checks on final SHA | NOT_RUN |
| Independent final review | NOT_RUN |

Every local test, build and application run is NOT_RUN under the owner's CI-only rule. No local Go, Node, pnpm, gofmt, Playwright, server or PostgreSQL was executed. Production, provider LIVE/SANDBOX and deployment are outside this static scanner unit.

## Source batch and remaining acceptance

Independent fixtures were integrated through `a3197998` (including `701f9057`, `9f0ce4aa`, `8874719c`, `2b56d03b` and `75397ad3`): fifteen cases in the new effect fixture cover renamed imports, retained helper edits, pool callbacks, shared entropy, successful continuation Close, mutable error sentinels, custom Context/Scan hooks, nested unsafe constructor returns, deferred error clearing, cross-file builtin shadowing, a package initializer mutation and named factory results. A positive case freezes the real `openPool` constructor body, with a transparently declared parsed-only authority stub. The tests and their inputs are source-reviewed only; their runtime results remain NOT_RUN. The deferred-Close case uses a conditional nonnil-error return, and the shadowed Scan case uses an otherwise admitted builtin `int64`, so an unrelated rejection cannot mask either defect.

The isolated source worker's `f0892aee` was not accepted: independent static review found a deferred named-error reset after helper Close, a cross-file type shadowing an assumed builtin Scan destination, and a package initializer resetting a global error sentinel. Follow-up `0df27419` restricts Close to terminating factory failure, checks package-wide builtin declarations, scans initializer closures for global mutation, and conservatively retains named factory results. `79ef9479` also includes exported context wrappers in source-origin checks. These source commits were merged as `a5840120`; the test and source writers used separate worktrees and did not overlap paths. Their final commits were independently inspected before integration.

Read-only `factory_effect_audit` reviewed the three known repairs at `0df27419`, finding no remaining P0/P1 in that scoped E1 review (Humaux record `92a6543e-1b3d-442c-afc8-1700004f733a`). Its final four-line delta review of `79ef9479` and named-factory counter at `a3197998` also found no scoped P0/P1 (`3f77303d-15a3-43bc-94db-74466fc5328b`). This is source evidence, not compiler, formatter or runtime acceptance. The final GitHub scanner package, format check, required PR checks and independent final acceptance remain pending.

Final source-file SHA-256 bindings for this batch:

| File | SHA-256 |
| --- | --- |
| `go_admission_effects.go` | `2235d4113f4867b13b1c36e16a77dd3a2702dfcc5398d1387985d9d4339d9d0a` |
| `go_factory_refs.go` | `8823691b891fc6457292dc0ce96f10909ac90d070cf41edfb909d4057a8e4e4f` |
| `go_factory_effect_cli_test.go` | `e98a2f7d5310fa1271f752a7ae72352ed0548e4737b4a3d32f25c4704dd40897` |

## Implemented boundary; CI acceptance pending

The proof removes only complete, proven startup admission statement intervals from factory provenance. It keeps actual helper bodies, imports, argument definitions, construction, middleware and request-handler dependencies. Unknown mutations, callbacks, receivers or calls retain the original span. The same boundary applies to opaque dispatch and handler-expression provenance. Resolved dependency bodies are retained independently of routing signals; routing signals establish inventory.

This is a bounded routing-effect analysis, not a general purity claim. Standard-library operations and borrowed PGx resources need verified import/type/producer identities and closed operation rules; a real PGx type alone does not establish callback-free construction. Configuration-writing cleanup and callback-configured pool counterexamples remain required. No git-diff, baseline or threshold exemption is used.
