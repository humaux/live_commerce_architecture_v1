# CI-SELECT delivery — backend paths select the browser modes that run them + nightly trunk browser re-verification

- Unit: ci-select-backend-browser. Branch: `unit/ci-select-backend-browser`. Base: `origin/r3/integration` (b1bfbeb3 at start; trunk advanced to 729afff9 (#27) mid-unit and to 9b738e0a (#29) during final verification — see §5).
- Owner approval: 2026-10-10. Commit only; **no push performed**.
- Evidence tiers (AGENTS.md): every local run below is SANDBOX (node-only planner tests, no PG, no browser, no live platform). Real `--browser-*` executions are CI-only: NOT_RUN locally. The nightly schedule is DESIGN-verified (planner output proven by tests) until it first fires on GitHub: NOT_RUN.

## 1. Root cause (verified 2026-10-10, pre-fix)

`scripts/dev/pr-modes.mjs` classified `internal/`, `cmd/`, `migrations/`, `contracts/` as backend-only ("cannot alter a browser" — false): every Go-seeded browser mode runs the real Go API on the real migrated PG.

- PR #30 (b1bfbeb3) changed `internal/live/stream.go` — the A2/A3 comment stream `--browser-live-console` clicks — and merged with **0** browser modes selected.
- PR #24 (3034c407) changed `internal/integrations/metareply/**` — same outcome, **0** browser modes.
- Push runs on `r3/integration` select no browser job either, so trunk browser health was never re-verified after any merge.

## 2. Design (fix with registry DATA, not a planner hand-list)

1. **`lc_covers="<internal packages>"`** — one declaration line per browser arm in the `test-local.sh` mode registry (keeps one-line-per-declaration layout). Derived, not guessed: `tools/derive-covers.mjs` computes each harness's closure from what its `tests/foundation` seed actually wires (direct imports + transitive internal import graph of what it constructs + the `cmd/` binaries it builds/runs). Run 4 result: all 70 internal package dirs are harness-wired (the shared `httpapi.NewHandler` constructs every service internally; harness closures import 54–57 packages directly; process tests build/run real `cmd/*` binaries — this is how `internal/tlsask`, mounted only by `cmd/api`, is still exercised by `--browser-identity`). `tools/covers.json` = 48 Go-seeded modes × 70 sorted packages + 3 node-only modes × `[]`. Inserted mechanically by `tools/insert-covers.mjs` (parser post-check before write, idempotent; logs `insert-run.log`, `insert-run2.log`, both `inserted=51 skipped=0`).
2. **Selection rules** (`backendBrowserModes` in pr-modes.mjs): changed `internal/<pkg>/**` → every mode whose `lc_covers` contains that package (prefix match); `cmd/**`, `migrations/**`, `SHARED_BACKEND_PACKAGES` (`internal/platform`, `internal/httpapi`, `internal/command`), and any package no mode declares → every `lc_fixture=pg` browser mode (conservative, 47); `BACKEND_ONLY_PACKAGES`-listed → none. `contracts/*.md`, `docs/`, `*.md` stay non-browser. `foundation-shards` always present. UI-path plans unchanged (backend selections ⊆ full browser set → exact prior order). Historical sources without the native registry fall back to the whole browser universe (never silently nothing).
3. **`BACKEND_ONLY_PACKAGES = {}`** — empty by the derivation above; a new entry must come from that derivation, with a one-line reason.
4. **check-gates rule** (`scripts/dev/check-backend-coverage.mjs`, wired into `check-gates.sh`): every internal package dir must be covered by ≥1 mode's `lc_covers` or listed in `BACKEND_ONLY_PACKAGES`; browser-universe modes must carry the `lc_covers` line; a Go-running mode must not declare it empty (bash comments stripped before Go detection so `--browser-admin-shell`'s prose stays legal); stale covers entries are red. A new unclassified internal package fails the gate until classified. Red→green tests: `tests/ci/backend-coverage.test.mjs` (5 tests).
5. **Both registry parsers read the new line and still fail closed**: pr-modes.mjs `modeEntries` (accepts `lc_covers="..."`, returns `covers: string[]|null`) and the bash `lc_select_mode` declaration arm (`lc_covers=*` alongside trunk's `lc_browsers` patterns).
6. **Nightly trunk re-verification** (`.github/workflows/gates.yml`): `schedule: cron '43 21 * * *'` (off-peak UTC, non-:00 minute) + `workflow_dispatch` input `full_browser_universe`. The `nightly` plan step feeds one tracked UI-side path (gates.yml itself) through the SAME planner via `--stdin`, so the nightly set is exactly what a UI-path PR gets (foundation-shards + 51 browser modes) — never a second hand-kept list. Failure is visible through the event-agnostic `required` aggregate. No secrets, no `pull_request_target`. Push behaviour unchanged. Deploy output defaults `'false'` for schedule.
7. **Docs**: pr-modes.mjs header rewritten (false "cannot alter a browser" claim removed; CI-SELECT block documents rules + derivation); `docs/delivery/GATES.md` PR-gates section retitled, root cause, 5-row selection table, `lc_covers` registration rule, nightly paragraph.

## 3. Selection counts (the deliverable's numbers)

From `tools/selection-counts.json` (identical pre- and post-#27 merge; runs `counts-run.log`, `counts-run2.log`, exit 0). "OLD" = replicated pre-fix planner behaviour on the same file list.

| Case | Files | Modes selected | Browser modes | incl. `--browser-live-console` | OLD browser modes |
|---|---|---|---|---|---|
| PR #30 (b1bfbeb3) | 11 | 49 | **48** | yes | **0** |
| PR #24 (3034c407) | 68 | 49 | **48** | yes | **0** |
| `internal/live/stream.go` (mandated 1) | 1 | 49 | 48 | yes | 0 |
| `internal/integrations/metareply/x.go` (mandated 2) | 1 | 49 | 48 | yes | 0 |
| `migrations/0169_x.sql` (mandated 3) | 1 | 48 | 47 (every PG-fixture mode) | yes | 0 |
| docs+contracts only (mandated 4) | 2 | 1 (foundation-shards) | 0 | no | 0 |

Universe: 51 browser modes (48 Go-seeded with full 70-package covers + 3 node-only `lc_covers=""`: `--browser-platform-site`, `--browser-admin-shell`, `--browser-picklist` — never selected by backend paths); 47 PG-fixture; `BACKEND_ONLY_PACKAGES` 0; registry total 83 modes. Neither #30's nor #24's foundation files are browser-tagged, so the old planner truly selected nothing for them.

## 4. Commands and exit codes

### RED (pre-fix planner, mandated first)

| Command | Exit | Result / log |
|---|---|---|
| `node --test tests/ci/pr-modes.test.mjs` | 1 | 10 new tests fail — incl. all 4 mandated cases (stream.go/metareply/migrations select 0; docs guard passes); 28 pre-existing pass, 2 new mechanism guards pass. `red-pr-modes.log` |
| `node --test tests/ci/backend-coverage.test.mjs` | 1 | 5 fail, `ERR_MODULE_NOT_FOUND` (check-backend-coverage.mjs did not exist yet). `red-backend-coverage.log` |

### GREEN (post-fix, pre-#27 base)

| Command | Exit | Result / log |
|---|---|---|
| `node --test tests/ci/pr-modes.test.mjs` | 0 | 38/38. `mid-pr-modes.log` |
| `node --test tests/ci/backend-coverage.test.mjs` | 0 | 5/5. `mid-backend-coverage.log` |
| `node --test tests/ci/mode-registry.test.mjs` | 0 | 4/4 (probe arm without lc_covers still legal). `mid-mode-registry.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | 83 modes. `mid-list.log` |
| `node --test tests/ci/*.mjs` | 1 | 59/60; the 1 fail is `browser-evidence-paths.test.mjs` → `Cannot find package 'typescript-api'` — environmental (needs node_modules), pre-existing on base (base's `check-browser-evidence.mjs` line 8 imports it). `green-tests-ci.log` |

### GREEN2 (post-#27 merged state — the authoritative numbers)

| Command | Exit | Result / log |
|---|---|---|
| `node --test output/.../tools/insert-wrapper.test.mjs` | 0 | `inserted=51 skipped=0`, arm layout `lc_build, lc_fixture, lc_covers, lc_browsers, lc_prepare` verified. `tools/insert-run2.log` |
| `node --test tests/ci/*.mjs` | 1 | **70 tests, 69 pass, 1 env fail** (same `browser-evidence-paths` → `typescript-api`; see BLOCKED-1/2). Includes trunk #27's `playwright-deps.test.mjs` passing on the lc_covers-carrying registry (compatibility proven) and all 5 `backend-coverage` tests green against the real merged registry. `green2-tests-ci.log` |
| `bash scripts/dev/test-local.sh --list` | 0 | 83 modes, 50 `--browser*` lines (+`--e2e` → universe 51). `green2-list.log` |
| `bash scripts/dev/test-node.sh` | 1 | first suite block: 403 tests, 392 pass, 11 env fails (`typescript-api`, `next/dist/build/swc` — missing node_modules); `set -e` stops there. No failure in any tests/ci suite (line 62's content proven green by the row above). `green2-test-node.log` |
| `bash scripts/dev/check-gates.sh` | 1 | stops at its pre-existing line 33 `check-browser-evidence.mjs` → `typescript-api` (same env cause, present on base). Steps after it: see NOT_RUN-3 for per-step equivalents. `green2-check-gates.log` |
| `node --test output/.../tools/counts-wrapper.test.mjs` | 0 | counts identical to pre-merge (§3). `tools/counts-run2.log` |

### GREEN3 (final state after adopting #29's gates.yml — the authoritative post-§5 run)

| Command | Exit | Result / log |
|---|---|---|
| `node --test tests/ci/*.mjs` | 1 | **70 tests, 69 pass, 1 env fail** — identical to GREEN2. Trunk's `playwright-deps.test.mjs` reads `.github/workflows/gates.yml` directly and passes on the final file (#29 pins + the CI-SELECT blocks), so the workflow still satisfies #27's assertions. `green3-tests-ci.log` |

## 5. Trunk #27 (729afff9) + #29 (9b738e0a) manual content merge

Mid-unit, trunk advanced to 729afff9 ("ci-playwright-deps-cache"), which touches **all four** of my modified files and adds `lc_browsers=` declarations in every browser arm — exactly where my `lc_covers=` lines go (51-arm naive-merge conflict). The sandbox denied every history-mutating git operation (`git merge`, `git rebase` ×2 forms, `git checkout --`, `git restore`) and `git ls-tree`, while allowing `git show <ref>:<path> > <path>` redirects. Resolution (manual content merge; same resulting tree as a resolved merge):

1. Reverted `test-local.sh` to my committed state, committed the clean CI-SELECT unit as **c0a3dcd9** (34 files).
2. Extracted 6 files verbatim from `origin/r3/integration`: `ci-playwright-browsers.mjs`, `ci-playwright-install.sh`, `tests/ci/playwright-deps.test.mjs`, `apps/admin/components/PasswordAuth.tsx`, `.github/workflows/gates.yml`, `scripts/dev/test-local.sh` (all 100644 — verified via `git diff --raw`; byte-identical to trunk blobs).
3. Re-applied 4 mechanical merge edits: pr-modes.mjs declarations loop accepts **both** `lc_covers="..."` and `lc_browsers=...`; `test-node.sh` line 62 lists **both** new suites; bash parser arm accepts **both** declarations; my 3 gates.yml blocks re-applied onto trunk content.
4. Re-ran `insert-covers` (lands between `lc_fixture` and `lc_browsers`) and the full GREEN2 battery above.

**#29 (9b738e0a) arrived during final verification**: it only repins GitHub actions (checkout v5.1.0, setup-go v6.5.0, setup-node v5.0.0 + `package-manager-cache: false`, cache v5.1.0, upload/download-artifact v6/v7) in `deploy-smoke.yml`, `foundation.yml` and `gates.yml`. `deploy-smoke.yml`/`foundation.yml` stay untouched by this branch (trunk-only changes — the PR merge adopts the trunk side; no conflict possible). `gates.yml` was re-extracted from current trunk and the three CI-SELECT blocks re-applied; `git diff origin/r3/integration -- .github/workflows/gates.yml` verified to contain **only** the CI-SELECT additions/replacements (25 added, 5 replaced lines) with every #29 pin intact.

Consequence: the branch tip is **not a git-descendant merge** of 729afff9/9b738e0a, but its tree equals the resolved merge for every path this unit touches. The 21 `output/ci-playwright-deps-cache/**` files #27 adds were intentionally NOT extracted (trunk-only additions, cannot conflict; they come from the trunk side at integration). If GitHub still reports conflicts when the PR is created, take the unit-branch side for the four overlapping files — it contains #27's and #29's gates.yml content verbatim.

Stale evidence (pre-#27 base, kept for history): `registry-diff.txt`, `mid-*.log`, `green-tests-ci.log`, `tools/insert-run.log`, `tools/counts-run.log`. The `green2-*` / `*-run2` logs are the merged-state truth. RED logs are pre-fix by definition and remain valid.

## 6. NOT_RUN / BLOCKED

1. **BLOCKED** — `pnpm install --frozen-lockfile --prefer-offline`: sandbox-denied twice (AGENTS.md two-attempt rule) → `node_modules` absent for the whole unit.
2. **BLOCKED (env, pre-existing on base)** — suites importing workspace packages: `tests/ci/browser-evidence-paths.test.mjs` (1 fail in both green runs) and `test-node.sh`'s first-suite failures (11). Not regressions: the imports exist on `origin/r3/integration` unchanged.
3. **NOT_RUN** — check-gates steps after its env-failure point (set -e): `ui-architecture-gate`, `shard-plan --check` CLI, `bash -n` loop, `modeEntries` CLI one-liner, `check-backend-coverage` CLI, migrations-permission awk loop, GATES.md↔registry python parity, dependency-ledger python, `gofmt`, `go vet` ×2. Direct execution denied by sandbox (second targeted attempt refused too). Equivalents green via allowed forms: `mode-registry.test.mjs` (= the modeEntries one-liner, real merged registry), `backend-coverage.test.mjs` (= the coverage CLI's core, real registry + real git dirs → 0 findings), `shard-plan.test.mjs`, and `bash test-local.sh --list` exit 0 (= registry parses end-to-end). Go steps irrelevant to risk: this unit changed **no** `.go`/`go.mod`/migration file. Mode set unchanged (83) and per-mode GATES.md rows untouched → parity check low-risk.
4. **NOT_RUN** — real `--browser-*` mode executions (need PG + Playwright browsers; they run in CI). Planner-level selection is what this unit verifies.
5. **NOT_RUN** — the nightly `schedule:`/`workflow_dispatch` firing itself (needs GitHub Actions on the pushed trunk). Verified locally instead: the planner's full-universe output for a UI-side path (§3 universe row + pr-modes.test.mjs full-universe tests). The exact nightly command (`printf ... | pr-modes.mjs --stdin`) was sandbox-denied when piped; its logic is the tested `--stdin` + `planPr` path.
6. **NOT_RUN** — `go test -race ./...`, PG integration tests (T02+ real scripts; out of scope for this packet-level unit and denied in sandbox).
7. **No push** — per task instruction. Integration (merge of this branch, pnpm/go.mod-adjacent files) remains integrator-only per AGENTS.md.

## 7. Risks

- `lc_covers` lines are long (48 arms × 70 packages). Maintenance is gated, not manual: `check-backend-coverage.mjs` reds stale entries and unclassified new packages; regenerate with `tools/derive-covers.mjs`.
- A typical backend-only PR now carries ~48–49 required checks instead of 1 — deliberate per owner approval (correctness over CI minutes); per-mode narrowing happens naturally if harnesses ever wire fewer packages.
- Nightly runs the 51-mode universe daily (off-peak 21:43 UTC); cost visible in Actions minutes.
- `SHARED_BACKEND_PACKAGES` and the undeclared-package fallback (all 47 PG modes) are intentionally conservative; over-selection is safe, under-selection is what caused #30/#24.

## 8. Cleanup

No lingering processes (every command ran to completion); no containers/ports used; test fixtures self-removed. All evidence lives under `output/ci-select-backend-browser/` (tracked). Commit-message scratch file is in `/tmp` (outside the repo). No other task directories touched.

---

# Round 2 — narrow per domain + nightly (owner decision 2026-10-10)

Sections 1–8 above are round-1 history and stay valid except where this section supersedes them (§9–§10 replace the covers DATA and classification; §11 replaces §3's counts; §15 replaces §4's logs). Round-1 problem, restated with the measured number: the import-closure derivation gave 48 of 51 browser modes `lc_covers` = all 70 internal packages (3360 covers entries — every harness boots the full API), so any `internal/` PR selected 48 modes, as heavy as a UI PR. Round 2 replaces that data with per-mode DOMAIN evidence; the nightly full-matrix run (round 1, unchanged) is the safety net.

## 9. Round-2 derivation (evidence, not guesses)

Tool: `tools/derive-narrow-covers.mjs` (plain node script, runs via `node --test`; deterministic — re-run after the registry rewrite produced a byte-identical `covers-narrow.json` and `uncovered=0`). Three evidence sources per mode:

1. **Path evidence** — every URL literal in the mode's Playwright specs, node runners and OWN Go harness files, staged: (tier 1) direct `/v1/…` calls; (tier 2) BFF pass-through suffix rules (`/api/stores/<s>/REST` → `/v1/admin/stores/<s>/REST`, `/api/buyer/REST` → `/v1/buyer/REST`); (tier 3) most-specific non-catch-all BFF route-file literals via depth-2 import closure; (tier 4) catch-all literals with trailing-`{*}` drops. URLs are matched against the REAL route tables — 329 `HandleFunc`/`Handle` routes in `internal/httpapi` + `internal/identityhttp` and 53 `buyerhttp` kind routes — with full Go expression resolution: string concatenation, func-local/package-scope consts, string function params resolved via call sites (incl. the generic `route[T any]` helper), `range` over inline/named string and struct slices incl. `append` growth, method-variable ranges. Each matched route is attributed at STATEMENT level to the package whose identifier appears in the registration statement (deliberately not func-wide: `NewHandler` constructs every service, and func-wide receiver attribution re-created round 1's explosion on the routes registered in its own body). 0 unresolved-route diagnostics; 4 unmapped URLs, all explained (§12).
2. **Narrow Go fixture evidence** — the mode's OWN Go files' DIRECT `internal/` imports only (OWN = the `-run` seed test-name definers ∪ registry-named `.go` ∪ "Used by:" header files). Deliberately NOT the transitive closure (that is round 1's mistake); `cmd/` binary imports are recorded as INFO diagnostics only, never as covers.
3. **Consumer-import join** — a package with zero direct evidence is joined into every mode covering one of its production importers (one level, shallow-first): csvguard→18 modes, jobqueue→28, mail→36, storehandles→13, twcity→20, payuni→32, ecpay→32. This is what keeps leaf/helper packages selectable without widening domains.

Outputs (all tracked): `covers-derivation.json` — per mode `[{path, handler file:line, package, via}]`, the task-mandated record; `tools/covers-narrow.json` — final per-mode lists; `tools/r2-diagnostics.txt` — full audit trail (per-mode counts, INFO cmd lines, joins, per-package totals, uncovered check). Result: **788 covers entries, mean 15.5/mode** (round 1: 3360, mean 70 across 48); max 41 (`--browser-customers-billing`, full-store billing acceptance); the 3 node-only modes stay `lc_covers=""`. Writer: `tools/insert-narrow-covers.mjs` — mechanical rewrite of the 48 `lc_covers` lines; asserts arm order, `lc_build`/`lc_fixture`, `lc_prepare`/`lc_run` bodies and every non-covers line byte-identical before writing; post-checks with the real `modeEntries` parser; idempotent (`rewritten=48 unchanged=3`, re-run rewrites nothing).

## 10. Classification (scripts/dev/pr-modes.mjs — every entry carries a one-line reason)

- **SHARED_BACKEND_PACKAGES (9)** — `internal/platform`, `internal/httpapi`, `internal/command`, `internal/httperror`, `internal/pagination`, `internal/identity`, `internal/identityhttp`, `internal/oidclogin`, `internal/integrations/psp/stripe`. Boot-path/request-grammar packages, each evidenced in 31–45 of the 48 Go-seeded modes (counts in the reasons) or constructed at API boot. A change under any selects **every Go-booting browser mode**; they are subtracted from all `lc_covers` lines (gate-tested invariant).
- **BACKEND_ONLY_PACKAGES (3)** — `internal/tlsask` (wired only by `cmd/api` main), `internal/retention` (claims-worker/retention-admin only), `internal/integrations/shipping/ecpay/ecpayroute` (worker + non-browser tests; `--browser-cvs` drives the `ecpaytest` fake). Select **no** browser mode. Resolved BEFORE covers (most-specific classification wins — `ecpayroute` sits inside the covered `ecpay` prefix); `check-backend-coverage.mjs` reds the contradiction (a covers entry inside BACKEND_ONLY, or a package both SHARED and BACKEND_ONLY) so the precedence can never silently hide a classification.
- **Go-booting set (planner widening)** — the conservative fallback set (cmd/, migrations/, SHARED, undeclared packages) is now `lc_fixture=pg` (47) ∪ non-empty `lc_covers` = **48 modes**, including `--browser-tracking-backfill`: `lc_fixture=none` only means it skips the shared fixture script — it boots the real Go harness against the real PG via `scripts/dev/test-focused.sh '^TestBrowserTrackingBackfill$'`, so migrations/cmd/shared changes reach it. The 3 node-only modes (`lc_covers=""`) remain unreachable from backend paths.
- **Gate (scripts/dev/check-backend-coverage.mjs)** — classification is three-way (covers ∪ SHARED ∪ BACKEND_ONLY, same `under()` prefix semantics as the planner) and stays complete: `check-backend-coverage: ok (70 internal package dirs classified; 51 browser modes declare lc_covers)`.

## 11. Selection counts (task item 5 — `tools/selection-counts-r2.json`, log `r2-counts-run.log`)

| case | files | round 2 browser modes (total checks) | round 1 |
|---|---|---|---|
| PR #30 `b1bfbeb3` | 11 | **48** (49) | 48 |
| PR #24 `3034c407` | 68 | **6** (7) | 48 |
| orders/payments example (`internal/merchantorders` + `internal/payments`) | 2 | **17** (18) | 48 |
| catalog example (`internal/catalog`) | 1 | **18** (19) | 48 |
| migrations example (`migrations/0169_x.sql`) | 1 | **48** (49) | 47 |
| `internal/live/stream.go` (mandated) | 1 | **18** (19) — live-console ✓, **not** `--browser-cvs` ✓ | 48 |
| `internal/integrations/metareply/x.go` (mandated) | 1 | **2** (3) — exactly live-console + e2e | 48 |
| `internal/platform/x.go` (mandated SHARED) | 1 | **48** (49) | 48 |
| docs+contracts only (mandated) | 2 | **0** (1) | 0 |

Exceptions to the ≤10 target, explained:
- **PR #30 stays at 48** because its diff includes `internal/httpapi/live_stream.go` — `internal/httpapi` is the monolithic handler mounted by every harness (SHARED by evidence, 45/48). A PR editing shared route registration IS cross-cutting; the same PR restricted to `internal/live/stream.go` selects 18. The root-cause fix stands: `--browser-live-console` selected.
- **migrations/SHARED/cmd/undeclared = 48** by the conservative rule (round 1: 47 — the +1 is `--browser-tracking-backfill`, real PG).
- **merchantorders 17 / catalog 18 / live 18 / claims 38** exceed 10 because that many acceptance modes genuinely click those domains (each row evidence-linked in `covers-derivation.json`), plus modes that sweep every page (`--browser-click-sweep`, `--browser-visual-lint`, `--browser-webkit`, `--browser-e2e`). Truly narrow domains hit the target: metareply 2, metabridge 2, meta_ads 2, `meta/oauth` 4, claimsintake 1, livekit 1, `payments/stripeadmin` 1, `payments/settlement` 10, attribution 11, billing 13, inbox 14.
- **PR #24 (68 files!) drops 48 → 6**: metareply/oauth/bridge evidence plus the sweep modes — the round-1 pathology (heavy backend PR ≈ UI PR) is gone.

## 12. Unmapped URLs (4 — all explained, no coverage hole)

1. `--browser-customers-billing` → `/v1/platform/stripe/webhook`: mounted by `cmd/api` only; the harness delivers it in-process (httptest). `internal/billing` is covered by that harness's direct imports.
2. `--browser-customers-billing` → `/v1/checkout/sessions` and 3. → `/v1/billing_portal/sessions`: paths of the FAKE Stripe server asserted via `fake.CallsTo`, not system routes.
4. `--browser-store-domains` → `/api/onboarding/handle-suggest`: a frontend request-listener assertion; no `route.ts` exists in the tree (nothing serves it).

## 13. metareply → live console, NOT inbox (task item 4 wording)

The task expected "the live console / inbox modes". The evidence: `internal/inbox` does not import `internal/integrations/metareply`; no inbox harness file or spec calls a metareply-served route. The only harnesses that drive MetaReply are `--browser-live-console` (A2/A3 comment fixtures) and `--browser-e2e` (comment flow), so the selection is exactly those two (asserted with `deepEqual` in `tests/ci/pr-modes.test.mjs`). Selecting `--browser-inbox` for metareply changes would be a guess — precisely what round 2 forbids; the nightly full matrix is the owner-approved safety net for such residual risk.

## 14. Tests (RED → GREEN; nothing weakened)

- **RED** (`r2-red-tests-ci.log`, round-2 planner lists + round-1 full-covers registry): exactly the 5 new selection-behavior tests fail — live narrowness (`--browser-cvs` selected: "the CVS-shipping harness exercises no internal/live route"), metareply exactness, inbox narrowness, SHARED-subtraction invariant, real BACKEND_ONLY entries → none. All 46 pre-existing tests in the two suites pass (only two stale COMMENTS were updated; no assertion touched).
- **GREEN** (`r2-green-tests-ci.log`): `node --test tests/ci/*.mjs` → **109 pass / 0 fail** (10 new round-2 tests: 7 in pr-modes, 3 in backend-coverage incl. the precedence-contradiction mechanism on a hermetic mini-registry and the tracking-backfill Go-boot rule).
- New invariants now gate-tested: SHARED entries carry reasons + select all Go-booting modes; SHARED subtracted from every `lc_covers`; BACKEND_ONLY real entries select none; BACKEND_ONLY-before-covers precedence with contradiction red; node-only trio never selected; undeclared package still falls conservative (48).

## 15. Commands and exit codes (round 2 — all SANDBOX/local on this worktree)

| command | exit | evidence |
|---|---|---|
| `node --test tests/ci/*.mjs` | 0 | `r2-green-tests-ci.log` (109/109) |
| `bash scripts/dev/test-node.sh` | 0 | `r2-green-test-node.log` (every suite fail 0; round 1's env-only failures gone — `node_modules` now exists) |
| `bash scripts/dev/check-gates.sh` | 0 | `r2-green-check-gates.log` (shard-plan ok 1256 tests/10 groups; check-backend-coverage ok 70 dirs; 82 modes documented; check-headers OK) |
| `bash scripts/dev/test-local.sh --list` | 0 | `r2-green-test-local-list.log` (83 lines: 82 modes + foundation) |
| `node --test tools/derive-narrow-covers.mjs` | 0 | `r2-derive-after-insert.log` ("51 modes; uncovered=0"; idempotent) |
| `node --test tools/insert-narrow-covers.mjs` | 0 | `r2-insert-run.log` (rewritten=48 unchanged=3; post-parser checks) |
| `node --test tools/selection-counts-r2.mjs` | 0 | `r2-counts-run.log`, `selection-counts-r2.json` |

Evidence tiers: derivation/selection/gate results above are SANDBOX (local node + real repo data). Real `--browser-*` mode executions, the nightly schedule firing, and `go test -race ./...` remain NOT_RUN (unchanged from §6 items 4–6; they run in CI/T02+). No push (task instruction).

## 16. Round-2 risks and cleanup

- Under-selection is bounded four ways: consumer-import join for zero-evidence packages, undeclared-package fallback (new package → all 48 until the gate classifies it), contradiction gate, nightly full matrix. Residual: a spec that builds URLs in a shape the extractor misses would show up as an UNMAPPED diagnostic (currently 4, all explained) — re-run the derive tool after adding specs/routes and check `r2-diagnostics.txt`.
- `lc_covers` lines are derived data; hand-editing them is gate-checked (stale entries red, SHARED duplicates red, BACKEND_ONLY contradictions red).
- Cleanup: no lingering processes (every command ran to completion); the temporary `covers-narrow.prev.json` idempotence snapshot was removed after the diff; no containers/ports; no other task directories touched. Round-1 tools (`derive-covers.mjs`, `insert-covers.mjs`, `covers.json`, `selection-counts.mjs`) are kept as history, superseded by the `-r2`/`-narrow` versions.
