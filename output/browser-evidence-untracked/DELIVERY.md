<!-- Purpose: browser evidence write-root hygiene delivery and acceptance pointers.
Depends on: base50086616, real browser harnesses and source/static/runtime preservation proofs.
Used by: integrator independent review/CI; historical unit checkpoint stays a prior snapshot. -->
# browser-evidence-untracked

- Branch `unit/browser-evidence-untracked`; base `5008661622d35a130e32d1b1fe7d0111ac03ad43`. Final tested source `bb393536dd03a88daa1b2a5d238b14ee9d9945ae`. Fetch confirmed origin/r3/integration still at that base; PR #22 was not present, so no trunk merge was needed.
- Source commits: `1225bdd9` (JS producers), `3debdb4f` (Go/shell/CI), `6b62d3b5` (ratchet/checkpoint), `db9011b1` (alias/label boundaries), `bb393536` (shared-root visual isolation). Final delivery commit adds evidence only. Product code, migrations, dependencies and business/browser assertions untouched.
- Roles: Codex-3 integrates and runs gates (parent runtime exact deployment/effort not exposed). Isolated author worktrees for JS and harness paths plus read-only reviewer configured `gpt-6.1-sol/high`. Incorporated author commits `12fd9078` and `3e3ce8e3`; no recursive delegation. Author proofs in prior `DELIVERY.json` are unit/static evidence only.

## Change

Ordinary browser modes allocate ignored `output/playwright/run.XXXXXXXX` before any build/log write. Go suite helpers and Node producers use that run root rather than copying screenshots/ledgers into committed historical directories. Explicit caller roots survive. Standalone producers use a unique ignored fallback. Committed historical evidence stays in place.

The registered ratchet follows common write sinks, literal/Join/template paths, local/import aliases, quoted/computed properties and Go path aliases. Historical reads remain legal. Helper labels cannot escape their parent. It is a bounded regression checker, not a language interpreter/security sandbox; runtime filesystem checks provide the separate preservation evidence.

A reviewer found visual-lint could overwrite click journey evidence when a caller reused an explicit root. It now uses sibling `ui-visual-sweep` fixtures while visual audit retains `ui-visual-audit/<stamp>` and its run-local LATEST. Actual Bash allocation regression went red then green; original click bytes remain intact. CI artifact upload/discovery supports nested roots. Read-only media after comparisons require `LC_MEDIA_SIZES_BASELINE`; original image/SHA/70% reduction/LCP assertions stay unchanged.

Complete source inventory: [INVENTORY.md](INVENTORY.md), 51 mode strings / 49 Go sources in `mode-inventory.json` and `path-source-inventory.json`. Source inventory is not runtime coverage.

## Red → green

- Baseline source scan: 72 candidate write destinations (`ratchet-before.log`); current scan zero unsafe destinations. Temp checkout next-env entries were classified separately from historical outputs.
- Added quoted-path/computed-writer/renamed-import/local-alias/label-escape counterexamples: `boundaries-red.log` → `boundaries-green.log`.
- Shared explicit-root visual fixture collision: `shared-root-red.log` → `shared-root-green.log`.
- Integrated mutation: change product-editor output back to `output/product-ui-v2`; the actual registered `bash scripts/dev/check-gates.sh` fails with unsafe destinations. Restore source byte-for-byte, full check-gates exits 0 (`evidence/ratchet-{inject-red,restored-green}.log`, `ratchet-proof.json`). No calibration mutation remains.

## Final gates

| Command | Exit | Evidence |
|---|---:|---|
| `bash scripts/dev/test-local.sh --browser-product-editor` | 0 | `evidence/browser-product-editor.log`; original PE12–17/review regressions plus frozen CC12, real Go/isolated PG |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | 0 | `evidence/browser-admin-shell.log`; production Next / real browser, contract-shaped fixture |
| `bash scripts/dev/test-node.sh` | 0 | `evidence/node-final.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `evidence/gates-final.log` |
| `pnpm --dir apps/admin exec tsc --noEmit -p .` | 0 | `evidence/admin-tsc-final.log` |
| `GOFLAGS=-p=1 go vet -tags browser ./tests/foundation` | 0 | `evidence/browser-vet-final.log` |
| `GOFLAGS=-p=1 go test -tags browser -run '^TestBrowserEvidencePaths$' -count=1 ./tests/foundation` | 0 | `evidence/ratchet-go.log` |

Bounded runner status JSON binds source SHA/HEAD, command, exit, elapsed time and unchanged-source check. Browser modes run serially. Each captures raw `git status --short` and the filtered tracked status immediately after exit, plus SHA-256 of all tracked historical evidence before/after. Both browser captures have empty tracked status (zero-byte `*.tracked-status.txt`) and identical pre/post historical evidence hashes. No git checkout, reset or artifact restoration was used to satisfy acceptance.

## CI gates and boundary

`node scripts/dev/pr-modes.mjs origin/r3/integration HEAD` selects **52 modes**, listed exactly in `evidence/ci-modes.json`: foundation shards plus the affected browser/mode set. `--stripe-browser` is also directly affected by the root relocation and should be included explicitly if the pre-registry selector omits it. Only the two requested browser modes were run locally per brief; all remaining selected runtime modes and full CI are NOT_RUN here. Sweep/visual aggregation is covered by Node tests, not a full local visual sweep. Media performance before/after, provider SANDBOX/LIVE and production/deployment are NOT_RUN.

Evidence class: real Chromium/Next; product-editor additionally real Go/isolated PG; synthetic identities/contract fixtures are MOCK. Independent reviewer ran 25 lightweight path tests with no P1/P2 remaining at bb393536; integrator independent batch review/CI still pending. **E3 at bb393536**: all requested gates passed in the measured environment. Runtime/curated artifact hashes are in `evidence/run-artifacts.json` and the package manifest. Byte-original logs/screenshots/traces remain in ignored run roots; committed logs normalize trailing whitespace only. Both author worktrees were clean, their validation files archived under ignored validation/authors, then removed; incorporated commits/branches remain. All processes started by this unit have exited. No push.
