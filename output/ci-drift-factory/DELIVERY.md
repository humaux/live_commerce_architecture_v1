# CI-DRIFT-FACTORY

Author record; runtime acceptance is pending GitHub CI and independent review.

## Scope and provenance

Base: `256e9740ced9c5de05a83f58eb9f4c79a231b7b2` (`origin/r3/integration`). Branch: `unit/ci-drift-factory`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory`.

| Participant | Role / model / reasoning | Worktree and allowed writes |
| --- | --- | --- |
| Author | Integration / `gpt-6.1-sol` / `ultra`; model and reasoning read from the local Codex configuration | Root worktree above; `scripts/dev/contractdrift/**`, `output/ci-drift-factory/**` |
| `drift_factory_boundary` | Read-only explorer / `gpt-6.1-sol` / `high`, explicit dispatch | Root worktree; no writes |
| `drift_factory_tests` | Independent test worker / `gpt-6.1-sol` / `high`, explicit dispatch | `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-drift-factory-tests`, branch `unit/ci-drift-factory-tests`, same base; only `scripts/dev/contractdrift/go_factory_cli_test.go` |

Delegate model/effort records describe the actual dispatch configuration, not independently measured provider telemetry. No product, migration, dependency, contract, baseline or gate-threshold change is authorized by this unit.

## Reproduction and acceptance

OA-1 PR #53 added a startup database identity check inside `buildBuyerWithCVS`. Its historical Foundation run [38474388216](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/38474388216), job `115561367494`, reported 71 TOUCHED unknown-route findings at `a047e48812139cb172794b04c8df13d2f7fea532`. The factory's complete source span enters both opaque dispatch and handler-expression provenance, then spreads to the custom mount's subroutes. The current PR53 source was statically checked at `316848601032b434885dd513b11f0a8871ef57e5`; the relevant factory and probe files match that reproduction.

Acceptance uses the existing compiled scanner CLI and Git-backed fixtures. A startup-only validation addition must produce zero touched routes without changing the inventory or baseline. Handler-input changes, routing edits, middleware changes and state/alias mutations must remain TOUCHED. All existing CI-DRIFT cases remain required.

## CI receipts

| Check | Status |
| --- | --- |
| Tests-first scanner regression | NOT_RUN — awaiting the red test commit and GitHub run |
| Full scanner package and `check-gates.sh` | NOT_RUN — Foundation checks executes both |
| Required PR Gates / Foundation checks on final SHA | NOT_RUN |
| Independent final review | NOT_RUN |

Every local test, build and application run is NOT_RUN under the owner's CI-only rule. No local Go, Node, pnpm, gofmt, Playwright, server or PostgreSQL was executed. Production, provider LIVE/SANDBOX and deployment are outside this static scanner unit.
