# ads-attribution — R12 round 2 review fixes

## Verdict: R12 local fixes verified — external prerequisites remain NOT_RUN

Frozen source: **`6c6c3fb397dd8c6ae8c291f3d751c1eedbc6ac89`**.
Branch `unit/ads-attribution`; authorized R12 `664db345` merge is `506e5f53`.
Migration **0113** only; 0112 refusal fields retained. No push, deployment,
Meta mutation, production credentials or actual buyer data.

Previous `4acbad53` and intermediate `2817f4ce` PASS results are historical,
not final-source acceptance. All final logs below are source-stamped and kept
under [r12-final](r12-final/). Every RED log remains in
[r12-round2](r12-round2/); older R10/R11 delivery is retained in
[the historical summary](r12-final/HISTORICAL-R10-R11-SUMMARY.md).

## Review closure and logical commits

| Finding | Change and evidence |
|---|---|
| **F1 P1: signals-only cohort** | Order remains in session totals; NULL path/draft never credits an ad. Real unboosted comment → claim → consented card Begin → signed capture yields exactly one paid order, correct net, no path/draft. The misleading R10 test is corrected per R12, not weakened. |
| **F2 P2: bounded GET recovery** | READY requires a matching live River job; DISPATCHING requires a valid lease. UNKNOWN and finals only replay for 10 minutes from updated_at. Test drives actual UNKNOWN, verifies same operation during cooldown, ages it, then plans and completes a fresh GET. |
| **F3 P2: grant-aware UI** | Missing stored read_insights: reconnect. Granted but unread: not_read and read action. Three locales, reload and acknowledged UNKNOWN recovery tested through real clicks. |
| **P3: offline minimization / ACL** | COD, transfer and pay-at-pickup never freeze fbc/fbp/IP; card positive control does. Same-key replay preserves factual attribution and NULL offline identifiers. Exact owner/search_path/EXECUTE inventory covers 11 checkout-side definers; extra runtime EXECUTE negative control fails. |
| **Browser-discovered ordering** | Unordered array_agg returned different linked-ID order across reads. One-line ORDER BY UUID fixes it; the strict array-equality browser assertion is unchanged. |
| **Retained R10/R11 controls** | Paid-only counts, pending COD separation, unknown Meta metrics, consent/final/erasure/purge cleanup, SQL email hash, live-window credit, bounded reports, timezone labels, two-decimal ROAS and trust-boundary tests remain. |

Logical commits:
- `a33bf127d3519ebeb10be55fb7f1a1943003f3fc`: R12 regression tests and actual-click fixtures.
- `2817f4ce0f33dd1bf85d1a733b6c4f51adb7f04b`: cohort, lease/cooldown, UI and minimization fixes.
- `6c6c3fb397dd8c6ae8c291f3d751c1eedbc6ac89`: deterministic linked draft order.

[CHANGES.md](r12-round2/CHANGES.md) records genuine RED → GREEN provenance,
diagnostic fixture failures, exact allowed paths, roles/models and independent
review scope. READY has no operation lease before dispatch under the existing
queue contract, so its actual queued job is checked rather than inventing a TTL.
Acknowledged UNKNOWN permits an explicit fresh-key GET intention; transport
uncertainty still preserves its original-key receipt fence.

## Final-source commands and exit codes

All commands run in this worktree with `LC_TEST_LOCK_WAIT=14400`.
Exact commands, source SHAs and exits: [results.tsv](r12-final/results.tsv),
[runner](r12-final/run-final.sh). No frozen assertion or threshold was loosened.

| Command | Actual result | Evidence in r12-final/ |
|---|---|---|
| `go build ./...` | exit 0 | go-build.log |
| `go vet ./...` | exit 0 | go-vet.log |
| tracked `gofmt -l` + assert empty | exit 0 | gofmt.log |
| `bash scripts/dev/check-gates.sh` | exit 0 | check-gates.log |
| `bash scripts/dev/depmap.sh --check` | exit 0 | depmap.log |
| `bash scripts/dev/test-node.sh` | exit 0; 387 PASS, 0 FAIL | test-node.log |
| `pnpm --filter admin exec tsc --noEmit` | exit 0 | admin-tsc.log |
| `pnpm --filter storefront exec tsc --noEmit` | exit 0 | storefront-tsc.log |
| `bash scripts/dev/release-gate.sh --strict --only G04` | exit 0 | G04.log, g04-final/ |
| Same G04 after staging all final evidence | exit 0; no key-shaped literal | g04-post-evidence/ |
| Broad focused PG (exact regex below) | exit 0; 102 top-level PASS, 0 FAIL, 4 external prerequisite SKIP; 601.804s | focused.log |
| `bash scripts/dev/test-local.sh --browser-ads-attribution` | exit 0; report 6/6, buyer checkout PASS; Go 2 PASS, 81.346s | browser-attribution.log |
| `bash scripts/dev/test-local.sh --browser-click-sweep` | exit 0; 123 page cases, 0 load failures; 981 controls PASS / 0 FAIL / 22 SKIP; 18/18 journey steps | click-sweep.log, click-sweep/ledger.json |
| `bash scripts/dev/release-gate.sh --strict --only G07` | exit 0; 6,749 test/subtest PASS (2,032 top-level), 0 FAIL; 13 accepted prerequisite SKIP | G07-console.log, g07-final/, G07-STATS.json |

Focused command:
```sh
bash scripts/dev/test-focused.sh '^(TestAdsAttribution|TestAds|TestMetaAds|TestMetaConnect|TestMetaClaimsMCI10|TestLiveClaimsKC03Schema|TestCustomersBillingCB02|TestCustomersBillingCB04|TestCustomersBillingCB05|TestR2IntegrationUpgradeFromReleaseHead|TestT06WorkerAuthorityAndFunctionACL)'
```

Full G07 compiled from the final source above, after focused/browser/sweep,
not from an earlier checkpoint. It ran 07:28:13–08:35:12 UTC on 2026-10-04,
including `go test -race ./...` and `go vet`; the isolated PG fixture was removed
by the runner on exit. [Source freeze evidence](r12-final/SOURCE-LOCK.md)
explains the six output-only dirty files in its header. [Quiet preflight](r12-final/start-quiet-g07.sh)
checks the source SHA and three 30-second-spaced samples with 1m/5m load below
logical CPU count; the runner also records load throughout. No shared lock is
removed and no other agent's process is stopped.
Quiet preflight passed at 07:27–07:28 UTC, but the run was **not continuously
quiet**: around 07:42 UTC the sampled 1-minute load reached 37.61 and 5-minute
load 13.20 on a 10-logical-CPU machine, then declined; a later spike also occurred.
Across 134 samples, 1m median/peak was 5.21/37.61 (11 samples ≥10), and 5m
median/peak was 5.785/13.20 (10 samples ≥10). The original run finished PASS
without changing assertions/timeouts or stopping another task. **Only the start
was confirmed quiet; continuous quiet-machine acceptance is not claimed.**
Raw load samples and [G07 statistics](r12-final/G07-STATS.json) preserve this
qualification for integrator review. `--only G07` is a subset, not a whole-release verdict.

## Real-click and screenshot evidence

Matrix: **390 / 1586 widths × zh-TW / zh-CN / en**. Report viewport height is
992, with full-page captures; this run is not described as 390×844.
Portable copies: [report](r12-final/browser-final/report/) (42 screenshots,
6 click ledgers), [buyer checkout](r12-final/browser-final/checkout/) (24
screenshots). All 66 screenshot SHA-256 values were checked against manifests.
The supplementary verifier also checks all six ledgers (102 PASS actions) and
PNG widths: exit 0, `r12-final/browser-evidence-verification.log`. Its first
attempt incorrectly assumed all screenshots were 1× pixels (exit 1, retained).
The checkout driver actually uses Pixel 7 at DPR 2.625: 390 CSS pixels produce
a 1024-pixel PNG. The corrected verifier accounts for that explicit profile;
no product code, frozen assertion or screenshot was altered.
Root inspected TW390 not_read, CN390 UNKNOWN recovery and EN1586 organic cohort.
Original report: `output/playwright/ads-attribution-report/20261004T070213.204533000/`.
Original checkout: `output/playwright/ads-attribution-checkout/20261004T070253.474693000/`.
Provider edges are MOCK; PG, transaction guards, worker and Next UI are real.
Independent non-author read-only source and evidence reviews found no new
blocker; they verified the original strict ID assertion was retained and the
replay privacy / ACL-negative-control gaps closed. These reviewers did not
personally rerun the full gate suite.
After G07 completed, the non-author reviewer independently recomputed its
PASS/SKIP counts and all 134 load samples, checked final command exits and the
browser/sweep ledgers, and found no concrete SUMMARY inconsistency. Read-only
audit task: `c40e93f9-2b5f-4d4b-9c2c-fb4e479aa641`; Humaux title:
`R12 6c6c3fb3 final SUMMARY G07 handoff consistency audit`.

## AT1–AT9 / acceptance limits

AT1–AT4, AT8 and AT9 local PG checks PASS on the frozen final source.
AT5 and AT9 actual-click browser PASS. AT7 final sweep and full G07 PASS.
Report timing uses 100 drafts / 10,000 synthetic orders, cold HTTP and unchanged
5-second deadline. Final-source HTTP 200: **2.96114525s** for 100 drafts,
**2.766498917s** for 101 drafts; direct runtime SQL **2.770404208s**.
The full race-enabled G07 repeated these cases: 100-draft cold HTTP
**2.832202625s**, 101-draft HTTP **2.751367083s**, direct SQL **2.769253292s**.

- **AT6 SANDBOX: NOT_RUN** — owner's dataset and test-event prerequisites.
- **AT9 LIVE: NOT_RUN** — actual live video/read_insights grant/App Review.
- Optional read_insights still needs owner/integrator configuration in Meta
  Login for Business selected by config_id, then reconnect; no URL scope override
  and no Meta mutation was performed by this unit.
- Optional R04 Node suite NOT_RUN: `COMMERCE_R04_LIVEKIT_BINARY` unset.
- G07 accepted 13 prerequisite skips, not counted as PASS: Stripe SP16,
  customers CB10, mail PA12, MF02 populated upgrade after 0062; Meta Ads sandbox
  S1–S4; Meta claims MCI11 live probes; Stripe SL08, RF10, RF03 populated upgrade
  from 0061, and SP21 real sandbox probe. Exact names and raw reasons are in
  [G07-STATS.json](r12-final/G07-STATS.json) and
  [G07.skipped](r12-final/g07-final/G07.skipped). Historical prerequisite
  justifications printed by the release runner are not live calls performed
  during this task. The focused run skips only Meta Ads S1–S4.
- Accepted P3-10 owner-role RLS and P3-12 order-level approximation remain.
  No buyer age/gender collection; no Pixel.

Frontend-architect preserved strict API models and explicit state boundaries.
Playwright kept acceptance as actual actions and persisted readbacks, not DOM
mutation or simulated success. No entire-release or live-provider claim is made.

## Evidence-only handoff

The delivery commit after `6c6c3fb3` contains output evidence only. It also
refreshes the three generated `output/ui-click-sweep` ledgers to this final run;
their portable copies are kept under this unit. The two generated platform-site
server logs were preserved in the portable evidence and restored to their
pre-run content, so this unit does not churn unrelated platform evidence.
The final G07 runner confirmed fixture removal at exit; a post-run process
inventory found no remaining task-owned gate runner or sampler. No shared
process, lock file, cache or other task's fixture was deleted.
