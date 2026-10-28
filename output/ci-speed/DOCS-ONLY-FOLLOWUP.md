# Docs-only PR skip — follow-up (NOT implemented)

Origin: CI-SPEED card goal 2 (2026-10-27). PR #238 implemented it twice. Both independent review rounds (Kimi K3.1 Pro, DeepSeek) found classifier holes, so the integrator cut it from #238 on 2026-10-28. #238 now ships build-once only. A docs-only diff plans exactly what it did at base `d960cab2a`: `foundation-shards` (11 jobs).

Code history (on branch `ci/speed-build-once`, for reference):
- v1 `5771436ec`..`3b9848fc9`: candidates were docs/**, output/**, contracts/**.md and root *.md. The lexical reader scan covered untagged Go plus one level of named scripts.
- v2 `38d2cbfa4`: candidates were restricted to doc extensions. The scan covered every code file outside output/ plus transitively named output/ tools, and a browser-side reader added the browser universe.

## Why a lexical "nobody reads it" scan cannot be fail-closed

`Foundation checks` (test-node.sh, check-gates) is NOT a required status. The ruleset requires only `Gates / required` and `review/independent`. So any reader the scan misses lets `required` go green while a test that reads the doc would be red. Counterexamples found:

| # | Round / reviewer | Counterexample | Hole |
| --- | --- | --- | --- |
| 1 | r1 Kimi + DeepSeek P1-1 | `output/integrator/tools/visual_review.sh` -> `{modes: [], docsOnly: true}`; read by `tests/admin/browser-evidence.test.mjs:140,200` | non-doc files (scripts, JSON) under docs/ or output/ counted as docs |
| 2 | r1 P1-1 | `output/ci-select-backend-browser/covers-derivation.json`, read by `tests/ci/backend-coverage.test.mjs:194` | same |
| 3 | r1 P1-2a | a doc read only by a browser-tagged Go test (fixture pinned `docs/browser-only.md` inert) | browser-tagged Go not scanned |
| 4 | r1 P1-2b | `contracts/stripe-platform-account-v1.md`, read by `tests/admin/card-payments-copy.test.ts:106` | Node test readers not scanned |
| 5 | r1 P1-2c | script -> script -> doc | only one script level followed |
| 6 | r1 P2 | `hasBrowserConstraint` treated `!browser` and `browser \|\| x` as browser-only | ad-hoc constraint parsing |
| 7 | r2 Kimi P1 | `scripts/dev/depmap.sh:16` `out=docs/engineering/dependency-map.md`, compared with `cmp` at :52 | unquoted shell paths are not string literals |
| 8 | r2 Kimi P1 | `./README.md`, `../README.md` | root *.md name regex rejects a `./` or `../` prefix |
| 9 | r2 Kimi P1 | browser spec -> output/ tool -> doc | the browser bit is not propagated along the transitive chain |
| 10 | r2 Kimi P1 | the monotonicity test's docs-only exemption used the same `inert` classifier it was checking | self-proving test |
| 11 | r2 DeepSeek P1 | `tests/ci/release-tag-guard.test.mjs` scans every tracked file via `git ls-files`, so a new `docs/runbooks/sub/new.md` was classified inert | whole-tree readers cannot be seen lexically |

Class 11 is decisive. Any test that enumerates the tree (`git ls-files`, `WalkDir(root)`, `glob('**')`) reads every doc. A lexical scan cannot prove absence of such readers.

## Recommended designs (pick one; each needs its own card + RED cases 1-11)

A. **Explicit allowlist, no classifier.** A short, reviewed list of path globs that are documentation by policy. Examples: `output/<unit>/**/*.md`, `docs/delivery/units/*.md`, `docs/design/**/*.{png,jpg,svg}`. Each entry carries proof that no test reads it. That proof is a gate: `tests/ci` runs every whole-tree reader (release-tag-guard, check-headers, claims_retention_guard's crFiles, ...) against a synthetic new file in each glob and asserts it is ignored. A new whole-tree reader must be registered or the gate fails. Unknown paths default to the full plan.

B. **Docs-only plan that still runs the cheap checks inside `required`.** Instead of an empty matrix, a docs-only plan runs one `docs-only` leg. That leg runs `scripts/dev/test-node.sh` + `check-gates.sh` (the Foundation checks content, ~5 min, no PG/browsers), and its receipt feeds `required`. This drops the 11 foundation shards (~175 job-min) but keeps every Node/static reader inside the required status. Go tests that read docs (runbooks glob, check_packet.py, matrix-ledger evidence) still need either A's allowlist or the shards. So B still runs `foundation-shards` when any changed doc matches a Go reader, or simply runs B + the shards for docs/ and B alone for an allowlisted `output/<unit>/**`.

Either way, the acceptance gate must include RED cases for every row above. "Fewer saved minutes is better than lost coverage" (integrator, 2026-10-28).

## Expected value

A docs-only PR currently costs the 11 foundation shards, about 175 job-min (#233: 15 jobs, ~35 min each). Design B would cut that to roughly 10 job-min for evidence/brief-only PRs.
