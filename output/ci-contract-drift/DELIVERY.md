# CI-DRIFT delivery

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
