# ads-tw-regulation — final local acceptance PASS

Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ads-tw-regulation`; branch `unit/ads-tw-regulation`; base `b0f835c3`; final source `b7afdf1d`.
No push, merge, deployment, credentials, real advertisements, SANDBOX or LIVE calls.
Owner confirmed migration **0112**. Final ruling is Amendment 2 (`eb2a38cf`), not either earlier Taiwan/regional-policy proposal.

## Scope and commits

| Item | State | Commit |
|---|---|---|
| TW→TAIWAN_UNIVERSAL, SG→SINGAPORE_UNIVERSAL declaration table; HK omits parameter; no identities | PASS (MOCK) | 8ce05a74 |
| Remove TH/EU prechecks, bespoke Taiwan/regional codes and our policy guidance | PASS | 8ce05a74, 11bed084 |
| Generic graph_<code>, Meta error_user_msg only; plain text, HTML removed, ≤300 Unicode code points; UNKNOWN rules unchanged | PASS (MOCK, REAL_PG, BROWSER) | 8ce05a74, 85b940f3, 11bed084 |
| Durable ads-owned refusal projection, lease-fenced Finish, readback in draft operations and settings recent20 (including insights/CAPI) | PASS (REAL_PG) | 85b940f3; migration0112 |
| AL1 task-owned process-group cleanup, original redirect assertions unchanged | PASS in final browser run | 18eca3f3 |
| Exact migration inventory47→48, no relaxed equality | PASS | 5845431c |
| Exact Finish guard; exact existing ads/CAPI loaders; composed alias/callback/loader/escape negatives | PASS (9 top-level MCI10 tests +15 fixture cases) | 769816d0, 7745eadc |
| Isolate new refusal/retry journey from frozen activation count; old assertion remains exactly1; new draft exactly1activate+1pause; prior drafts rechecked | PASS (BROWSER + REAL_PG, Graph MOCK) | b7afdf1d |

Authorized ruling cherry-picks: `2b83615d` (first amendment), `7247de9e` (final amendment).
All later changes after `5845431c` are tests/contracts only: `git diff 5845431c b7afdf1d -- apps internal migrations` is empty. This establishes product equivalence for the earlier click-sweep evidence; it does not erase historical gate failures.

## Final gate ledger

| Command | Exit | Evidence / result |
|---|---:|---|
| `go build ./...` | 0 | a5-go-build.log |
| `go vet ./...` | 0 | a5-go-vet.log |
| `git ls-files -z '*.go' \| xargs -0 gofmt -l` | 0 | a5-gofmt.log, empty |
| `bash scripts/dev/check-gates.sh` | 0 | a5-check-gates.log |
| `bash scripts/dev/test-node.sh` | 0 | a5-test-node.log;329PASS0FAIL;optional R04 binary unset |
| `pnpm --filter admin exec tsc --noEmit` | 0 | a5-admin-tsc.log |
| `bash scripts/dev/test-focused.sh '^(TestMetaAds\|TestAds\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | a5-focused-final.log;23PASS/0FAIL/4sandboxSKIP; exact required regex |
| `bash scripts/dev/test-local.sh --browser-meta-ads` | 0 | a5-browser-meta-final.log;2 Go tests PASS,30.126s;Playwright original8+new1 PASS, no scenario skipped |
| `bash scripts/dev/test-local.sh --browser-click-sweep` | 0 | a2-browser-click-sweep.log at5845431c, product-identical to final;120units,993clicksPASS/0FAIL/18SKIP,18journey stepsPASS |
| `bash scripts/dev/release-gate.sh --strict --only G07` | 0 | a5-g07.log / release-gate-a5/;6517 tests/subtests PASS,0FAIL;1965 top-level PASS,10 top-level SKIP;foundation3270.101s |
| `bash scripts/dev/release-gate.sh --strict --only G04` | 0 | final-static/results.tsv; source and staged evidence scanned, no key-shaped literal |

Queue wrapper exports `LC_TEST_LOCK_WAIT=14400`. Final G07 uses a separate evidence directory; the original failing run is preserved. `exits.tsv` is chronological, not a list of only passing attempts.
The strict runner reports G07 PASS with accepted NOT_RUN:13 skipped tests/subtests (10 top-level), listed in `release-gate-a5/G07.skipped`. These cover external SANDBOX/LIVE/SMTP prerequisites and older populated-upgrade branches; they were not executed by this unit. References to historical owner LIVE checks in the runner's accepted catalogue are not this unit's evidence. The3 dirty tracked files at gate start were generated `output/ui-click-sweep/{journeys.json,ledger.json,ledger.md}` only; product, migrations and tests stayed pinned to b7afdf1d throughout.

## Red/green and failures retained

- Adapter: pre-Amendment2 source94dfb152 overlay + new tests → exit1 (`a2-adapter-red.log`); current adapter/routes → exit0 (`a2-adapter-green.log`, `a2-routes.log`).
- Persistence: original-message PG assertion before projection → exit1 (`a2-pg-red.log`); after0112 → exit0 (`a2-pg-first-green.log`). Required focused group at5845431c → exit0,23PASS/0FAIL/4sandboxSKIP (`a2-focused-native.log`).
- First full G07 at5845431c → **exit1**,1962 top-levelPASS/2FAIL/10SKIP;foundation3683.223s (`release-gate/G07.log`). Failures: order search latency, and new Finish absent from MCI10 allowlist.
- Own MCI10 defect: exact Finish guard → exit0 (`a3-guard-green.log`). Independent review then found the old directory+loader-literal composition bypass. Two composed counterexamples → exit1 (`a4-composed-red.log`), exact loader file/function restriction → exit0 (`a4-composed-green.log`);9 top-level tests and15positive/negative cases.
- Browser fixture defect: at7745eadc all9Playwright cases passed but old PG assertion saw2activations instead of1 (`a4-browser-meta-final.log`, exit1). Two phases preserve the old assertion and independently constrain the new draft; final `a5-browser-meta-final.log` exit0.
- Order-performance issue is **not changed by this unit**: search7654 took1.233441625s in full G07 and1.107228416s in fresh focusedPG, threshold remains<1s. The latter run `a3-focused-final.log` exits1:32PASS/1FAIL/4SKIP,347.989s. Independent review confirms order code/test/related migration blobs match base;0112 is ads-only. No EXPLAIN/resource evidence establishes a cause; do not call it an environment flake.
- Final G07's unchanged order-performance test passes: telephone suffix7654 n10,p50=99.802125ms,max=104.989709ms; tracking n10,p50=76.05975ms,max=77.281667ms. This does not retroactively invalidate either red run or establish a performance fix. See `ORDERS-PERFORMANCE.md`.
- Historical harness incidents: old pre-Amendment browser was terminated after AL1 assertion passed but teardown hung; NOT a browser PASS. An evidence wrapper hot-edit caused an EOF exit2 before an early browser test began; logs are retained. Later superseded queued launchers exited143 without starting tests. Early a4G07 was interrupted130 in its verified task-owned process group74975 for the browser fixture fix; task fixture `lc-foundation-test-75663` and that group were confirmed absent. No foreign lock/process was removed. Partial a4G07 is **NOT acceptance evidence** (`release-gate-final/`).
- `SUPERSEDED-first-ruling.md` and pre-a2 logs are historical only.

## Screenshots and click ledger

`screenshots/`: all24 manifest images from final browser run20261003T163322.181847000, SHA256 manifest included. The six `meta-original-refusal-{zh-TW,zh-CN,en}-{desktop,mobile}.png` show the same Meta wording after reload (1586×992 /390×844); all six inspected. Mobile cards wrap code/text without horizontal overflow. No HTML/link rendering of provider text.
`a5-playwright-original.log`:8PASS. `a5-playwright-refusal.log`:1PASS. PG assertions run after each phase.
`click-sweep/{ledger.json,ledger.md,journeys.json,runner.log}`: final-product-equivalent click evidence generated2026-10-03T15:09:04.951Z.18 skips have individual explanations, not counted as clicked PASS. No load/new/stale/known failures.

## Independent review, ownership and boundaries

Root sole writer; no child writer worktree. Base and worktree above. Read-only reviewers: gpt-6.1-sol/high (security/adapter), gpt-6-luna/medium (fixture/performance mapping); no recursive delegation.
Root owns this unit's adapter/CAPI completion hook, migration0112, Ads UI/model/copy, associated tests, contracts/docs and output only; root runtime model/reasoning are not exposed in the session and are not inferred. No changes to other worktrees. Own implementation/test commits have the required Codex footer; authorized ruling cherry-picks retain the integrator's original authorship.
- Product/security review at18eca3f3: no openP0/P1/P2 after corrections — Humaux `e9f2286b-b887-4206-8021-a36c45043385`.
- Exact48 migration inventory delta — `9d5f9430-7a3b-45da-964a-0912464f9f5e`.
- Composed guard final actual-diff review and independent2tests/15fixtures exit0, P2closed — `2e6f8444-64fc-49b3-948c-b75ee1986267`.
- Two-phase browser diff reviewed, no original assertions weakened — Humaux title `Ads MA09a two-phase activation assertion review at 7745ead`.
- Unchanged order latency impact review — `6814e5c4-27b8-4d00-bd0d-5f84fefcfc5b`.
Reviewers did not run PG/browser/fullG07; root ran recorded gates. Code graph and memory links updated for implementation and guards.

Cleanup: the final runner exited0 and reported removal of its isolated PG fixture; the unit-private `child-pg.lock` is absent. A post-run cwd inspection found only the inspection shell/tools in this worktree, no leftover unit server/test process. The subsequently running foundation process belonged to `r5-gate-b0f835c`; it and all foreign locks/processes were left untouched.
Raw console evidence is retained byte-for-byte, including12 Next build progress lines with trailing CR/space and2 red-source snapshots with a final blank line. The optional `git diff --cached --check` on all raw evidence exits2 for those14 whitespace diagnostics only. `git diff --check b0f835c3 b7afdf1d` exits0 for the actual source changes; none of the required gates is waived.

Original messages discarded before0112 cannot be reconstructed. Missing Meta error_user_msg stays code-only; never expose private error.message or invent a translation. Settings returns latest20 current refusals; per-draft operations remain readable. Projection is current-attempt evidence, not immutable history; success clears it.

**NOT_RUN:** SANDBOX and LIVE deliberately not run; sandbox assertion is written for owner/integrator. G07's13 accepted skips and optional Node R04 binary are NOT_RUN, not passed tests. No production secrets loaded. All requested local gates passed; this subset is not a complete-release or production-readiness verdict. Historical order timing variability remains documented for the integrator.
Skills used: frontend-architect preserved read/model/command boundaries; playwright reused real-click fixtures. No production browser actions.
