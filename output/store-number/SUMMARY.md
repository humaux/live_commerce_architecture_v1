# store-number — implementation delivered; two gate failures retained

Scope: `/Volumes/data/live_commerce_architecture_v1/.worktrees/store-number`, branch `unit/store-number`, baseline `e44e58a2458102ebeb772a84c8086e0af5500c72`. No push, merge, deployment, production credentials or other-worktree changes.

## Implementation

| Item | Status | Evidence |
|---|---|---|
| Random eight-digit automatic handle; existing-store backfill uses the same allocator | IMPLEMENTED / focused PASS | `72b17e1c`; unpublished migration 0106 only |
| Reserved/existing/still-serving candidates excluded; 50 attempts then PT409; candidate transaction lock + unique index | IMPLEMENTED / focused PASS | Numeric concurrency, deterministic occupied draws, exhaustion and forced two-session collision tests |
| Name slug + suggestion SQL/Go/BFF/client removed | IMPLEMENTED / unit PASS | `72b17e1c`, `c9701022`; retired Go endpoint 404/405, absent BFF/client files |
| Three-language automatic assignment explanation; no preview; display authoritative success receipt | PASS | `c9701022`; final frozen domains + password real-click gates passed |
| Operator handle-set / custom domains / Caddy / TLS admission / verifier preserved | focused PASS / domains browser PASS | Existing grammar, reserved names, domain lifecycle/ACL and exact freed-handle ownership assertions retained |
| Deployment step 7 + identity/store-domain handle contract | PASS | `72b17e1c`, wording follow-up `da2faeed` |

The legacy internal allocator signature remains `(text, uuid)` to avoid unrelated ABI churn; both arguments are intentionally unused. The public number is not a credential. No new dependency, migration number, index, role or grant was introduced.

## Red → green

- `red-route.log`: old suggestion POST returned 200; exit 1.
- `red-node.log`: old BFF route/client still existed; exit 1.
- `red-pg.log`: old numeric/backfill assertions returned name slugs; 0 PASS / 2 FAIL / 0 SKIP, exit 1.
- `red-collision.log`: old 50-store numeric / occupied candidate / bounded exhaustion contract failed; 0 PASS / 2 FAIL / 0 SKIP, exit 1.
- `focused-green.log`: first implementation exposed two additional obsolete slug assertions (SDW02/SDW11); exit 1. They now assert numeric format and deterministic freed-number reuse, retaining ACTIVE/detach/ownership checks.
- Independent review requested a forced concurrent collision, since 50 random draws rarely collide. `red-lock-clean.log`: remove only the candidate-lock statement → expected wait assertion fails, 0 PASS / 1 FAIL / 0 SKIP, exit 1. Lock restored before final validation.
- `red-lock-mutation.log` is historical harness-failure evidence, **not** the accepted negative control: it also found deferred Rollback racing the in-flight query. Cleanup now cancels and joins before Rollback; `red-lock-clean.log` has no DATA RACE.
- `focused-final-source.log`: final source, 38 PASS / 0 FAIL / 0 SKIP, exit 0; includes deterministic collision, SDW11 exact freed-number reuse, backfill and upgrade/ACL regressions.

## Commands

Commands run from this worktree. Exact exit timeline: `exits.tsv`. `test-focused.sh` in this evidence directory is a safe wrapper for baseline runners that do not uniformly enforce the requested wait: it takes the machine PG lock, waits up to 7200 seconds, never removes a foreign lock, and only releases its own PID lock. Nested focused runner gets a task-local child lock. No foreign process was stopped.

| Command | Exit | Result |
|---|---:|---|
| `go build ./...` | 0 | PASS |
| `go vet ./...` | 0 | PASS |
| `gofmt -l` on tracked Go files | 0 | Empty output |
| `go test ./internal/identityhttp ./internal/storehandles` | 0 | PASS |
| `bash scripts/dev/check-gates.sh` | 0 | PASS |
| `bash scripts/dev/test-node.sh` | 0 | 328 PASS / 0 FAIL; optional R04 binary absent |
| `pnpm --filter admin exec tsc --noEmit` | 0 | PASS |
| `bash scripts/dev/release-gate.sh --strict --only G04` (additional artifact hygiene) | 0 | PASS including staged evidence; `artifact-secret-check/summary.txt` |
| `bash scripts/dev/test-focused.sh '^(TestStoreDomains|TestIdentity|TestR2IntegrationUpgradeFromReleaseHead|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | 38 top-level PASS / 0 FAIL / 0 SKIP |
| `bash scripts/dev/test-local.sh --browser-store-domains` | 0 | Final frozen-source replay: 22 checkpoints; `browser-store-domains-final.log` |
| `bash scripts/dev/test-local.sh --browser-password-auth` | 0 | Final clean replay: 11 password + 6 staff-team browser cases; `browser-password-auth-final.log`, `password-evidence/*-playwright.log` |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | 0 | 7 Node assertions + 24 browser matrix cases, role negative, store switch and axe audit |
| `bash scripts/dev/test-local.sh --browser-click-sweep` | 1 | BLOCKED for integrator ruling: native tel-link observation; 120/120 page loads, 978 PASS / 1 FAIL / 18 SKIP controls; 18/18 journey steps PASS |
| `bash scripts/dev/release-gate.sh --strict --only G07` | 1 | Full run finished: 1,958 top-level PASS / 1 FAIL / 10 SKIP; all test/subtest events 6,473 PASS / 2 FAIL / 13 SKIP (the failed parent and its one child are one root cause) |

## Independent review and scope

### Full G07 stop line

`g07/G07.log` and `full-g07.log`: one failing top-level test, `TestBuyerCommsCardCaptureAndStripeRefundTriggers`, whose refund-notification child inserts two separately evaluated `clock_timestamp()` values against an exact 20-hour equality CHECK. The test and migration 0062 are byte-identical to baseline; numeric handle migration 0106 neither changes them nor invokes this refund path. Root and read-only `number_tests` independently inspected the source and baseline comparison. Details: `G07-refund-fixture-analysis.md`. No refund code, fixture or assertion was changed. Full G07 remains **FAIL**, not waived or replaced by focused results.

All 24 `TestStoreDomains*` tests, the R2 release-head upgrade and T06 authority/ACL test passed inside this full run, as well as in the focused run. G07's `dirty_files=3` header records preceding click-sweep generated tracked evidence awaiting restoration, not product-code changes; these were restored after copying them to this unit. The source stayed pinned at `da2faeed` throughout.

Thirteen skipped test/subtest events all have explicit NOT_RUN explanations: ten top-level external sandbox/live prerequisites and three nested cases (two unavailable partial-migration fixtures and one real Stripe sandbox probe). See `G07-NOT_RUN.md`; none is claimed as PASS.

### Click-sweep stop line (no assertion waived)

`click-evidence/ledger.json` and `runner.log`: only failure `r00846`, storefront `/`, desktop/en, phone link `+886 2 2345 6789`, `no-effect` within 3 seconds. The two zh-TW phone clicks passed **only because the page scrolled**, not because a phone application opened. All 18 journey steps passed, including catalog creation, COD ordering/collection, domains, and session change.

`tel-baseline-repro.mjs` uses the unchanged click-sweep monitor and the exact native `tel:+886223456789` anchor in a loopback-only headless Chromium page. It checks Git blob equality at baseline `e44e58a2` versus HEAD for `ShopChrome.tsx`, `click-sweep-lib.mjs` and `click-sweep.mjs`; then reproduces `no-effect` (diagnostic command exit 0 means reproduced, **not gate PASS**). Evidence: `tel-baseline-repro.log`, failure screenshot `click-evidence/r00846.png`. This isolates an unchanged external-protocol observation gap; it is not a full baseline gate replay and not proof of an OS phone handler. No product link, fixture, timeout, skip list or assertion changed. The full gate remains FAIL until the integrator decides an explicit external-protocol acceptance rule.

Click-sweep's generated tracked baseline ledgers were copied into `click-evidence/` before restoring only those task-generated modifications from HEAD. The new failure screenshot was moved into this unit's evidence; it remains recoverable there.

Final implementation SHA: `da2faeed7f1bfe243d77e554298bb829c0b1d638` (reporting/deployment wording follow-up to the two implementation commits). The first password gate's business assertions passed (11 password + 6 team cases), but its runner log ended with `unexpected EOF` because this task edited the runner's printed description during execution. Despite the wrapper reporting exit 0, that log is **not accepted** as final PASS. The frozen runner passes `bash -n`; the entire password gate was rerun cleanly with exit 0, 11 + 6 cases and no EOF. No assertions were relaxed.

The shell runner overwrites tracked historical W0 evidence. Fresh outputs were copied to `shell-evidence/`, and only files overwritten by this task were restored from HEAD; initial worktree was clean. Final domain screenshots and exact receipt are in `screenshots/` and `store-domains-result.json`: `https://41671428.lctest.example` is the local fixture origin, not production. The real-click password signup case checks the same numeric receipt contract; its three-language registration screenshots and logs are in `password-evidence/`. The final password/team harnesses found none of their 61 / 43 canaries in logs (team tokens also absent from API URLs). Raw canary/credential fixture files are not committed.

Root is sole writer, with write paths confined to this worktree's changed files (root model/reasoning identifiers are not exposed by this session; not inferred). Read-only explorers (`number_domain`, `number_tests`, gpt-6-luna / medium) mapped authority and tests; independent `security_reviewer` (`number_review`, gpt-6.1-sol / high) reviewed final `e44e58a2..da2faeed`, no write paths and no recursion. Review found no P0/P1 in scope, and its P2 collision-test gap was fixed and verified by negative control. Follow-up memory: `52d2eede-9557-43f4-93a8-364846a5b59d`; final review title: `[research] store-number final pinned da2faeed review: P2 race-test gap closed, no new P0/P1 located`. Reviewer independently ran Go identityhttp/storehandles unit tests and final diff check, exit 0; reviewer PG/browser/G07 NOT_RUN, root runs those gates.

Skills: frontend-architect guided removal of the complete obsolete client/UI chain while retaining receipt/retry ownership; playwright guided real-click registration assertions. No new design or speculative feature was added.

## NOT_RUN / boundaries

- `tests/media/r04-input-runner.test.mjs`: optional `COMMERCE_R04_LIVEKIT_BINARY` unset, reported by test-node; unrelated to store-number.
- Production deployment, real DNS/CA, real mailbox and live domain verification: NOT_RUN / not authorized. Browser gates use local fixture PG/Go/Next with explicit external mocks.
- All requested commands were actually run; there are no queued or running gates. **Acceptance remains BLOCKED by two retained failures**, not by an unrun requested command: the unchanged refund time fixture in full G07, and the unchanged native telephone link's click-sweep observation. Both need integrator disposition; no assertion has been waived.
- All evidence stays inside the requested worktree, per the latest task-specific scope (overrides generic PROCESS main-output placement).

## Commit and handoff index

| Commit | Responsibility |
|---|---|
| `72b17e1c` | SQL allocator, retired Go suggestion path, numeric/collision/backfill/404 tests, contracts and runbook |
| `c9701022` | Retired BFF/client and preview, three-language explanation, authoritative receipt, Node and real-click browser assertions |
| `da2faeed` | Gate reporter and deployment text aligned with automatic numeric assignment; source frozen here for final gates |

Each logical commit carries `Co-Authored-By: Codex <noreply@openai.com>`. The following evidence-only commit contains this report, exact logs, screenshots and the independent stop-line analyses; it does not change tested product code. No push, merge or deployment was performed.

Additional hygiene: staged evidence contains no canary files, `runner.env`, raw credential fixture directory or key-shaped literals. `git diff --exit-code da2faeed -- apps internal migrations scripts tests contracts docs` returned 0 after all gates; the product source is unchanged. Source-only whitespace check returned 0. The all-evidence `git diff --cached --check` returned 2 solely for unmodified raw log formatting (Next progress-line CR/trailing spaces, Go failure SQL indentation, blank assertion-log lines); these bytes are deliberately retained, not falsified into clean output. All task-started gate commands have terminated, and the task-local child PG lock is absent; no foreign process/lock was removed.

Recommended next action for the integrator: adjudicate the two explicitly isolated baseline gate issues in their owning units, then rerun affected gates on the integrated candidate. Do not treat focused PASS or these failure analyses as release approval.
