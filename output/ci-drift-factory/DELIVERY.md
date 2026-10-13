# CI-DRIFT-FACTORY

Author record; runtime acceptance is pending GitHub CI and independent review.

## Scope and provenance

Base: `256e9740ced9c5de05a83f58eb9f4c79a231b7b2` (`origin/r3/integration`). Branch: `unit/ci-drift-factory`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory`.

| Participant | Role / model / reasoning | Worktree and allowed writes |
| --- | --- | --- |
| Author | Integration / `gpt-6.1-sol` / `ultra`; model and reasoning read from the local Codex configuration | Root worktree above; `scripts/dev/contractdrift/**`, `output/ci-drift-factory/**` |
| `drift_factory_boundary` | Read-only explorer / `gpt-6.1-sol` / `high`, explicit dispatch | Root worktree; no writes |
| `drift_factory_tests` | Independent test worker / `gpt-6.1-sol` / `high`, explicit dispatch | `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory-tests`, branch `unit/ci-drift-factory-tests`; initial base above, frozen-OA1 follow-up base `bfcfa5eb`; only `scripts/dev/contractdrift/go_factory_cli_test.go` and the subsequently assigned `go_factory_oa1_cli_test.go` |

Delegate model/effort records describe the actual dispatch configuration, not independently measured provider telemetry. No product, migration, dependency, contract, baseline or gate-threshold change is authorized by this unit.

## Reproduction and acceptance

OA-1 PR #53 added a startup database identity check inside `buildBuyerWithCVS`. Its historical Foundation run [38474388216](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38474388216), job `115561367494`, reported 71 TOUCHED unknown-route findings at `a047e48812139cb172794b04c8df13d2f7fea532`. The factory's complete source span enters both opaque dispatch and handler-expression provenance, then spreads to the custom mount's subroutes. Codex-3 supplied the exact reproduction at `316848601032b434885dd513b11f0a8871ef57e5`: [Gates 38475612050 / job 115565420528](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38475612050/job/115565420528) reports the same 71 errors, exit 1. The retrieved original log binds the actual synthetic merge of that source into `256e9740`; see [portable excerpt](evidence/oa1-plan.log).

The second test fixture freezes the complete `ValidateSameDatabase` body and original `buyer.go` call block from that SHA, including the named error, deferred local cleanup, advisory-lock query, fresh Scan destination, and `closePools` failure branch. Source blobs are recorded in the test. The fixture's helper and imports predate the added call block. It retains aligned Go/contract/BFF route counts and EXACT/MOUNT warning coverage, plus counterexamples for captured configuration writes and a fake PGx receiver. The scanner never executes this fixture or contacts a database.

Acceptance uses the existing compiled scanner CLI and Git-backed fixtures. A startup-only validation addition must produce zero touched routes without changing the inventory or baseline. Handler-input changes, routing edits, middleware changes and state/alias mutations must remain TOUCHED. All existing CI-DRIFT cases remain required.

## CI receipts

| Check | Status |
| --- | --- |
| Tests-first scanner regression at `bfcfa5eb` | **RED** — [Foundation 38477346748 / job 115571192630](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38477346748/job/115571192630), actual execution 22:27:46–22:30:42 UTC, 2026-10-13. Both startup-only factory names incorrectly return exit 1/errors 3. The new middleware and child-handler dependency cases also expose false negatives (exit 0). [Portable receipt](evidence/red-factory.log). This is regression evidence, not acceptance. |
| Frozen OA1 probe regression | NOT_RUN — tests-only follow-up `1962bf85`; awaiting its GitHub run |
| Full scanner package and `check-gates.sh` | RED at the scanner regression above; later Foundation checks NOT_RUN after the failing scanner batch |
| Required PR Gates / Foundation checks on final SHA | NOT_RUN |
| Independent final review | NOT_RUN |

Every local test, build and application run is NOT_RUN under the owner's CI-only rule. No local Go, Node, pnpm, gofmt, Playwright, server or PostgreSQL was executed. Production, provider LIVE/SANDBOX and deployment are outside this static scanner unit.
