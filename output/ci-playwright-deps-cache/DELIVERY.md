<!-- Purpose: mode-derived Playwright dependency installation with bounded apt retries.
Depends on: registry metadata, frozen CLI, hosted Ubuntu runner and existing browser cache.
Used by: integrator pre-review/push and CI timing acceptance. -->
# ci-playwright-deps-cache

- Branch: `unit/ci-playwright-deps-cache`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ci-playwright-deps-cache`.
- Base: `3034c4073366d77e9e849fbfe77e966caf11e62f`; tested source: `2f848f0108fe14a17b5b7032ee9cd0138f7c63ee`. Delivery adds evidence only. Frozen source SHA256: `8c0a1de3c6b56f4112a30c4d06f652b1065f3e0ff47f1a8a0faf07c2a2c04391`.
- Role/model/effort: Codex-3 root implements (GPT-6 family; exact deployment/effort unavailable). Read-only explorer configured `gpt-6.1-sol/high` independently reviewed and ran ten tests.
- Allowed write paths: `.github/workflows/gates.yml`; `scripts/dev/{ci-playwright-browsers.mjs,ci-playwright-install.sh,test-local.sh,pr-modes.mjs,test-node.sh}`; `tests/ci/playwright-deps.test.mjs`; `output/ci-playwright-deps-cache/`. Contract, dependency and lockfile changes: none.

## Changes

All 51 browser arms declare installation requirements in the existing single registry: 49 Chromium only, CVS and WebKit both engines. A validated `LC_BROWSER_ENGINE=webkit` calibration takes the union and retains Chromium. Missing/unknown declarations fail closed. Coverage tests derive mixed Go roots and shell assignments, catching a future mode selecting the CVS test without WebKit metadata. No second production mode-name list. All 83 mode names, build/fixture choices, commands and test selectors remain unchanged.

Apt gets at most three attempts, each under root GNU timeout: 240 seconds plus 15 seconds kill grace; two 10-second backoffs. The dependency retry envelope is at most 785 seconds. The existing overall 15-minute step and 150-minute job bounds remain unchanged. Downloads must fit the remaining budget; failures remain nonzero. Root Playwright avoids nested sudo, allowing the root timeout monitor to signal its apt process group. The CLI resolves from the directly declared `@playwright/test/cli`; pnpm does not expose the transitive `playwright/cli` at the root.

Browser downloads run as the user after dependency success. Binary cache keys include OS, architecture, Playwright version and engine set. An exact-version dual-engine cache may warm Chromium-only jobs; Chromium-only caches cannot populate the dual key. System dependencies are installed even on binary cache hits. Optional apt/dpkg caching is deferred.

## Local evidence

| Command/check | Exit | Evidence |
|---|---:|---|
| Initial seven criteria against old all-deps behavior | 1 expected | `evidence/red.log`: 0 pass, 7 fail |
| New installer/coverage/workflow suite | 0 | Ten cases in `evidence/focused.log`; independently 10/10 |
| CI planner, registry, pr-modes and installer suites | 0 | `evidence/focused.log`: 45 pass |
| Delete CVS WebKit metadata / reduce retry loop to one | 1 then 0 each | `coverage-*`, `retry-*`, byte-exact restoration in `mutation-proof.json` |
| `bash scripts/dev/test-local.sh --list` | 0 | `modes.txt` equals `base-modes.txt`: 83 names |
| `bash scripts/dev/test-node.sh` | 0 | `node-final.log` and status: 1191 pass |
| `bash scripts/dev/check-gates.sh` | 0 | `gates-final.log` and status: 82 documented gates, header check |
| `pnpm exec tsc --noEmit -p apps/admin` | 0 | `tsc-final.log` and status |
| YAML safe parse / Bash syntax checks | 0 | Installer step remains 15 minutes; actionlint is not installed |

E3 local acceptance: actual scripts with fake external uname/sudo/timeout/CLI/pnpm/sleep edges, real frozen-package resolution, registry and workflow checks. Historical tracked output remained unchanged. All owned local gate processes ended. Raw failure logs remain in the ignored validation directory; committed text trims trailing whitespace only. No push or production/provider action.

## CI gates and limits

Hosted Linux apt, process-group cancellation, browser downloads/cache behavior and CI step timing are NOT_RUN locally. Coverage discovery uses conservative file-level/literal-selector inspection; sibling tests may require WebKit unnecessarily, and this is not a proof of arbitrary future dynamic dispatch. The 785-second dependency budget leaves 115 seconds for other work, not a guarantee of successful downloads.

After the integrator pushes, retain the normal required set. Capture installation engines and step time for `admin-legacy`, `design`, `meta-health-ui`, `ads-attribution` (Chromium only), plus `browser-cvs` and `browser-webkit` (both installed; actual WebKit subtests pass). Logs print engines, attempts, limits and elapsed seconds. No PR is opened by this unit; integrator owns push and hosted proof.
