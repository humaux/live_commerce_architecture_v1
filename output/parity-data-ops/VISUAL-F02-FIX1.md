# PR475 F02 inventory search — first product repair candidate

task_id: d2f8e3b8-8d0e-4434-b687-a7ce38425813-sub-1 (root owns coord task d2f8e3b8-8d0e-4434-b687-a7ce38425813).
base_commit: 7216d35ff8e0f9c45b0e23250672547a944b3e6c.
branch: codex/pr475-inventory-repair (observed via git branch --show-current); PR475.
worktree: /Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr475-inventory-repair.
role: sole F02 product author; requested Sol/high, actual model/reasoning telemetry UNKNOWN.
Allowed changed paths: apps/admin/components/inventory/InventoryView.tsx; apps/admin/components/imports/data-ops.css; output/parity-data-ops/VISUAL-F02-FIX1.md.
Evidence level: E1 static candidate only. Runtime/visual GREEN, independent acceptance and production readiness are NOT_RUN. One candidate, no recursive delegation, no separate coord claim, no push/CI/merge.

## Root cause and minimal implementation

Before: the search-state submit button had only `primary`. The existing `.data-ops button.primary` selected `--ui-primary` (#247965, rgb(36, 121, 101)). Original CI ran the independent navy/white oracle at both 1440 and 390 and reported background RED: expected rgb(20, 41, 66), received rgb(36, 121, 101). White foreground assertion is retained exactly.

After candidate: only that submit button additionally has `ops-inventory-search-submit`. A single scoped CSS rule immediately after the generic primary rule selects existing `--ds-ink` for background and border. Existing packages/ui/src/tokens.css:64 defines #142942; :82 defines `--ds-on-primary: #fff`. No new token or literal color is introduced in product code.

Cascade review: generic primary and the new selector both have specificity (0,2,1), so the later scoped rule controls its background/border. Existing `.data-ops button:hover:not(:disabled)` has (0,3,1), so it continues to control hover background. Foreground still comes from generic primary. Existing focus outline, disabled opacity, event handlers, state transitions, query/filter/reload, dimensions and layout are untouched. Rendered cascade verification remains NOT_RUN.

## Authority and evidence

Read AGENTS.md; architecture sections 0/1/2/26/27; contracts/tasks.json T04/T05 DAG; contracts/invariants.json; frozen contracts/catalog-inventory-v1.md boundary; existing source and direct Ledger caller. I03 inventory conservation, I04 transactional task intent, I14 CAS and I16 controls remain server-owned and unchanged. No API, permission, journal, amount, stock truth or capability changes.

Official design authority: Figma file iJ9lDdFIXgTlKkt5Tvrc0X, frames 840:5254 and 840:5255, enabled nonhover navy/white. Parent independently inspected archived official images and froze this exact repair scope. This author inspected the existing raw CI log and repository token source; made no fresh Figma request or pixel verdict. Existing report prose is provenance, not a replacement design authority.

Original evidence in integration root:

- output/integrator/tools/logs/ci-evidence-1238/475-visual-review-1140/REPORT.md; SHA256 1618e263ecb169b0525857594b2363b0d103fa8182af33ae615252bccf4bd4bf (includes matrix/source receipts).
- output/integrator/tools/logs/ci-evidence-1238/475-visual-ci-1238/13141517471/0-playwright.log; SHA256 5f254e8311eaf92684f3c0dfedd1e1eace8e721336d2658ba5d3c45b6ab92fb7; lines 63–77 and 101–115 carry the two actual RED assertions. Run 41443325073, job 125272277848, artifact 13141517471. This is prior RED, not candidate execution.
- Existing tests/admin/parity-data-ops.spec.ts:183–218 retains both viewport cases, background/foreground assertions and real filter/reload checks without any test edit.

## Actual static commands and results

All commands ran in the worktree above; no runtime test was started.

| Command | Exit | Result |
|---|---:|---|
| `git status --short` | 0 | Initial tracked clean; exactly five historical untracked KEEP artifacts observed and untouched. |
| `git rev-parse HEAD` | 0 | Exact base_commit above. |
| `git diff --check` | 0 | No whitespace errors in candidate. |
| `git diff --exit-code 7216d35ff8e0f9c45b0e23250672547a944b3e6c -- tests/admin/parity-data-ops.spec.ts tests/admin/parity-customer-import.spec.ts packages/ui/src/tokens.css apps/admin/app/globals.css` | 0 | Tests and global tokens/styles byte-preserved against base. |
| `git diff -- apps/admin/components/inventory/InventoryView.tsx apps/admin/components/imports/data-ops.css` | 0 | One TSX class change and one scoped CSS line. |
| `shasum -a 256 apps/admin/components/inventory/InventoryView.tsx apps/admin/components/imports/data-ops.css tests/admin/parity-data-ops.spec.ts tests/admin/parity-customer-import.spec.ts packages/ui/src/tokens.css` | 0 | Hashes below. |
| `command -v tsc` | 1 | No executable found; no local dependencies installed. Typecheck NOT_RUN; no installation attempted. |
| `git add -- apps/admin/components/inventory/InventoryView.tsx apps/admin/components/imports/data-ops.css output/parity-data-ops/VISUAL-F02-FIX1.md` | 0 | Only the assigned three paths staged. |
| `git diff --cached --check` | 0 | Staged candidate whitespace check passed. |
| `git -c core.hooksPath=/dev/null commit -m 'fix(admin): match signed inventory search button color'` | 0 | Local candidate committed; root receives the final commit ID in handoff. No git config change. |

Source SHA256:

- InventoryView.tsx: ec31ffde51a3194a86f2da63f2fce02075e026d18cf725304edcdc18235b36df.
- data-ops.css: 0643836d370aa92191abc516447e2a6cdf88953ce9d4e0db4e477811135a369e.
- unchanged parity-data-ops.spec.ts: 0cd98c9903f8792f65f143520abb0a1685c41e65d9672ce6af11b72b6aff613a.
- unchanged parity-customer-import.spec.ts: b68817c2d7e419804a622fe0645728624fcd77dec21c71b77e6e978d07d19b22.
- unchanged tokens.css: 0f70f5418534be6db9409824586b00b37baa66460501aa766ed6a2f3b8ba0383.

Humaux read coverage: rejected project 直播SaaS 11/11, no truncation; development/visual detail records fetched in full. Existing graph lookup locates InventoryView; local rg confirms Ledger imports the component and stylesheet. Skills read: figma-implement-design and ponytail. No new scripts, fixtures, gates or mirrored tests.

## Remaining acceptance / handoff

Root must review the full three-file diff, ownership and retained oracles; make any integrator-owned metadata updates; then run the established GitHub checks at the exact resulting SHA and obtain independent acceptance. Required F02 evidence: both enabled nonhover navy/white assertions turn GREEN while original native search/filter/refresh behavior remains GREEN. Existing hover/focus behavior still needs runtime verification. No assertion or threshold may be weakened.

Local Go/PG/browser/Node tests, build, tsc, new CI and independent acceptance: NOT_RUN (task restricts runtime execution to GitHub; local typecheck tooling absent). PE12/CC12 member failures: separate cause UNKNOWN, outside this fix. Full visual parity/token diff and other review findings remain unresolved. No processes, fixtures, containers or ports were created. The five pre-existing untracked KEEP artifacts remain. No product-completion claim.
