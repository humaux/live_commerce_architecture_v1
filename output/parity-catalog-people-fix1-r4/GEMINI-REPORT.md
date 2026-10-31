# GEMINI-REPORT PARITY-CATALOG-PEOPLE-FIX1 relay 4 (PR #363 head 6b392c3b, run 41181967784)

- **Merge `origin/r3/integration`**: Fetched and checked merge with `origin/r3/integration` (`37e097399`); already up to date.
- **Failing job on `6b392c3b` (run `41181967784`)**: `foundation` (`124481042992`) → `bash scripts/dev/check-gates.sh` → `node scripts/dev/check-browser-evidence.mjs` failed with `Error: spawnSync git ENOBUFS` at `checkBrowserEvidence` (`scripts/dev/check-browser-evidence.mjs:140:16`).
- **Root cause & fix**: `checkBrowserEvidence` executed `execFileSync('git', ['ls-files', '-z'], {cwd: root, encoding: 'utf8'})` without a `maxBuffer` parameter, leaving Node's default buffer limit at 1MB (1,048,576 bytes). As the repository grew to 15,963 tracked files (1,072,218 bytes of NUL-delimited paths), `execFileSync` exceeded the default buffer and threw `ENOBUFS`. Fixed by specifying `maxBuffer: 64 << 20` on line 140, matching the convention established in `scripts/dev/pr-modes.mjs`, `scripts/dev/derive-browser-ownership.mjs`, and `scripts/dev/check-backend-coverage.mjs`.
- **Browser ownership**: Ran `node scripts/dev/derive-browser-ownership.mjs` and verified with `--check` (exit 0, 54 modes).
- **Go formatting & ratchets**: `gofmt -l .` verified clean (exit 0).

| failing job / test | class | what changed (file:line) |
|---|---|---|
| `foundation` (`124481042992`) / `check-gates.sh` (`checkBrowserEvidence`, `scripts/dev/check-browser-evidence.mjs:140`) | b | `scripts/dev/check-browser-evidence.mjs:140` — supply `maxBuffer: 64 << 20` to `execFileSync('git', ['ls-files', '-z'])` to prevent `ENOBUFS` when repository file inventory exceeds 1MB; `output/PARITY-CATALOG-PEOPLE/FIX1.md:1-8` — updated failure classification table with relay 4 findings |
| `gate (bundle:03)` (`124474795328`, run `41179510955` on head `f7f55343`) / `TestBrowserCatalogCore/PE12-17-TWD` (`CATV2 mobile density`) | b | Previously fixed in `0e0ea3a79` (`apps/admin/components/ProductList.tsx:100,139-143`) by marking decorative text spans `aria-hidden="true"` and consolidating `c.selected` into `.cat-select-all` label |
| `gate (bundle:03)` (`124468182631`, run `41177298991` on head `424e7ba5c`) / `PARITY catalog signed controls` | a + b | Previously fixed in `e547175ee` and `47131fdb9` (verified passing in run `41179510955`) |
| `receipts`, `required` | dependent | None (fail only because upstream jobs failed) |

STATUS: DONE
