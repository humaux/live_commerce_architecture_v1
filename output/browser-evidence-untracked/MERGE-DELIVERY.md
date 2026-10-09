<!-- Purpose: PR23 trunk-registry merge and meta-health evidence routing acceptance.
Depends on: merge parents, shared root guard, declarative mode registry, real browser gates and filesystem snapshots.
Used by: integrator re-review/push; E3 local acceptance only. -->
# PR23 registry merge batch

- Branch/worktree: `unit/browser-evidence-untracked`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/browser-evidence-untracked`.
- Tested merge source: `a72cb5b732db94cdaadfd6f32ad93b576563a862`; parents PR23 `ff9a770d4db1c39bd7595f6918d8a1716df885c8` and trunk `305695a7233bbb676e24f11b110d2bdeaf99dc2d`. Final delivery commit changes evidence only.
- Role/model/effort: Codex-3 root implements; runtime reports GPT-6 family but exact model/effort unavailable. Read-only reviewer `/root/pr23_workflow_merge`, integration_worker, configured gpt-6.1-sol/high. No recursive delegation.
- Explicit allowed write paths for this batch: `scripts/dev/test-local.sh`, `.github/workflows/gates.yml`, `tests/ui/sweep-shard-lib.test.mjs`, `output/browser-evidence-untracked/`. Inherited trunk edits retained. Contract/API changes: none.

## Resolution and root cause

Kept trunk's single registry. Shared allocation/explicit-root guard executes before selection/preparation/build/fixture; --list/--dry-run remain inert. The guard is byte-identical to ff9a770d: physical pwd, tracked/ignored check, refused new-root cleanup and top-level symlink-child refusal preserved. All 83 mode names/build/fixture metadata equal trunk (82 documented gates plus foundation). Ten registry bodies reapply run-root evidence destinations/messages and visual empty journeys under separate ui-visual-sweep; the other 73 bodies are unchanged. Shared fixture runtime is byte-identical to trunk. Kept selective visual-lint upload, nested ci-gates failure logs and trusted visual-review security; union imports preserve both sides' ten sweep tests.

Meta-health's ff9 evidence change made CopyFS source and target both ROOT/meta-health-ui/<timestamp>. Real browser mode reproduced RED exit 1 after UI predicates passed: Go174 reported `blocking-en-390.png: file exists`. The registry now supplies ROOT/meta-health-ui-durable; source and copy are distinct siblings under LC_BROWSER_EVIDENCE_ROOT. Test Go file is byte-identical to both parents; all assertions unchanged. Real GREEN exit 0 and all 14 copied files byte-equal (`meta-health-copy-proof.json`). No main-checkout evidence writes or historical restoration.

## Local gates and evidence

All records below are in `merge-evidence/`. Static gates ran on the resolved tree before merge commit; their exact source SHA256 e7bf55b70082ae585371ee1c1cf039cddcd344eab8de1f253f77fd5f27fc8175 equals the committed source used for every browser gate.

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-local.sh --list` | 0 | mode-list.txt; all 83 names byte-equal trunk list |
| `bash scripts/dev/test-node.sh` | 0 | node-merge.log/status; 1219 tests, including browser-evidence 7/7, registry and pr-modes |
| `bash scripts/dev/check-gates.sh` | 0 | gates-merge.log/status and gates-delivery.log after staging delivery; writer ratchet 0 unsafe destinations, 82 documented modes |
| `pnpm exec tsc --noEmit -p apps/admin` | 0 | tsc-merge.log/status |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | vet-merge.log/status |
| focused registry/evidence/pr-modes, path and sweep node tests | 0 | registry-focused.log, paths-focused.log, sweep-focused.log; sweep 10/10 |
| `bash scripts/dev/test-local.sh --browser-meta-health-ui` (bad alias) | 1 expected | meta-health-red.log/status, actual CopyFS failure |
| `bash scripts/dev/test-local.sh --browser-meta-health-ui` | 0 | meta-health-green.log/status, copy proof; 29.887 s |
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | product-editor-green.log/status; PE12-17 mobile matrices/axis removal with exact PG readback, CC12 frozen 120; 101.317 s |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | 0 | admin-shell-green.log/status; 24 matrix + 24 brand real-click/reload cases; 95.664 s |

Browser gates ran one at a time on a72cb5b7 with source frozen. Each has empty raw tracked-status files before/after and identical historical tracked-output SHA256 c11d888b1a6325ccefcc309a43348ad18ee4f6e5d76279cbc9c21397d8c0db9a; no checkout/restore was used. Runner records its 1200-s outer supervision bound; existing mode/Go/browser timeouts and thresholds remain unchanged. Original raw logs/screenshots remain in ignored run roots; committed text logs trim trailing whitespace only. Untracked integrator-owned output/ext-agents is untouched/unstaged.

## Acceptance boundary and CI gates

E3: automated local RED/GREEN and three real browser gates on committed source. Next/BFF/Go/isolated PG use synthetic/mock providers, never LIVE acceptance. Read-only final review PASS at a72cb5b7, limited to merge semantics and focused tests; reviewer did not rerun browser GREEN. Receipt: independent-review.md.

`node scripts/dev/pr-modes.mjs origin/r3/integration` selects 53 modes plus deploy-smoke; exact list is pr-modes.txt. Local acceptance was the owner's requested meta-health-ui/product-editor/admin-shell modes. Remaining required CI modes, full foundation/deploy-smoke/visual lint/artifact upload, provider and production acceptance are NOT_RUN locally; integrator runs CI after re-review/push. All owned local runners/fixtures exited; evidence preserved. No push or GitHub thread reply.
