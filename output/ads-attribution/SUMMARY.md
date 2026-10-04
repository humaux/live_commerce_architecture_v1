# ads-attribution — R10 + R11 review fix delivery

## Current verdict: LOCAL REVIEW FIXES VERIFIED — external prerequisites NOT_RUN

Final verified source: **`4acbad5353814fc3940812e9e399fdb1253ed40f`**.
The report browser at checkpoint `40128fe5` found a real READY-only UI parser
rejecting R11's completed SUCCEEDED replay. This candidate fixes strict receipt
states and persisted feedback; new READY and same-id SUCCEEDED are tested
separately. Independent follow-up also fixed the cached-UNKNOWN misleading
retry and the 100-UUID mobile text wall, without discarding any identifiers.
Final static, browser, focused PG, sweep and edge checks pass. Full G07 compiled
from this commit also **PASS, exit 0: 6,725 tests/subtests, 0 failures**. Its 13
accepted prerequisite skips are listed separately below. This is a scoped local
review-fix handoff, not an entire release verdict or live provider acceptance.
Branch `unit/ads-attribution`; authorized `373462d4` merge is `8f419664`.
Migration **0113** only; 0112 refusal fields retained. No push, deployment,
external Meta mutation, production credentials or actual buyer data.

Old `068874fc` G07 is **not** evidence for this source. Its records and every
intermediate RED run remain in [historical checkpoints](r13-review/HISTORICAL-CHECKPOINTS.md)
and the previous `r9/`–`r12-review/` evidence directories.

## Review findings

| Requirement | Source / verification |
|---|---|
| R10 paid-only order counts | Paid card, confirmed transfer and collected COD count; unpaid/draft/expired/cancelled do not. Pending COD remains separate. Actual mixed-cohort and exact report targeted PG PASS. |
| R10 independent signals + R11 consent | Begin may capture valid fbc/fbp/IP without a touch, only with ads_personalization consent. Signals-only NULL-path rows excluded from reports. |
| R11 I23 | Fresh scope/source/grant check; one in-flight read per store/video, 10-minute replay cooldown; no extra River job on replay. |
| R11 I12 | Missing Meta metrics, spend, ROAS and local-only timeline spend are NULL/—. No fabricated zero. |
| R11 optional breakdown failure | Failed dimension unavailable; other dimensions and D7 persist. Existing hourly lane retries bounded account-days with a non-extending 24-hour deadline; final daily facts immutable. |
| R11 privacy / email | All three identifiers cleared at final CAPI states including UNKNOWN, withdrawal, erasure and bounded purge. No purge starvation. Worker receives SQL-normalized SHA-256 email only. |
| R11 live-window credit | lc_ad only in successfully activated draft window. Comment fallback retains no-fan-out ambiguity handling. |
| R11 report bound | 100 rows plus 101st sentinel / truncated notice. REAL_PG 100-draft, 10k-order cold HTTP within unchanged 5-second deadline. |
| UI R10/R11 | Three-language audience/timezone/provisional/promoted-post markers, selected-period session spend, two-decimal ROAS, reconnect, cap and unknown states. |
| Integrity / trust | Patch needle asserted; exact private EXECUTE inventory; stable same-fbclid timestamp; host-only secure cookies and pinned-edge XFF/Host regression. |
| Page scopes | Optional granted read_insights preserved; missing grant offers reconnect. Remote config_id update is an external prerequisite, not a scope URL override or weakened base eligibility. |

Implementation, logical commit mapping, independent provenance, allowed paths,
model provenance and rejected experiments: [IMPLEMENTATION.md](r13-review/IMPLEMENTATION.md).

## Commands and actual exit codes

All commands run here with `LC_TEST_LOCK_WAIT=14400`. Source-stamped logs are in
`r13-review/`; `results.tsv` and `boundary-results.tsv` record exits.
No failed assertion was removed or relaxed.
An extra `git diff --cached --check` over raw artifacts returned **2** for
captured trailing spaces/terminal CRs in command and RED-test logs; those original
logs are deliberately preserved. Source-only staged and unstaged whitespace
checks returned **0**. This auxiliary artifact check is not a required gate PASS.

| Command | Exit / counts | Evidence |
|---|---|---|
| `go build ./...` | 0 | `go-build.log` |
| `go vet ./...` | 0 | `go-vet.log` |
| tracked `gofmt -l` + assert empty | 0 | `gofmt.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates.log` |
| `bash scripts/dev/depmap.sh --check` | 0 | `depmap.log` |
| `bash scripts/dev/test-node.sh` | 0 / 384 PASS, 0 FAIL (4acbad53) | `test-node.log` |
| admin `tsc --noEmit` | 0 | `admin-tsc.log` |
| storefront `tsc --noEmit` | 0 | `storefront-tsc.log` |
| `release-gate.sh --strict --only G04` | 0 | `G04.log`, `g04-final/` |
| Same G04 after staging final evidence | 0 | `g04-post-evidence/` |
| `go test -race ./internal/attribution/capiroute ./internal/integrations/meta_ads ./internal/metaconnect` | 0 / 3 packages | `adapter-race.log` |
| Targeted mixed-cohort/report/signals-only/ACL PG | CHECKPOINT 40128fe5: 0 / 4 PASS, 0 FAIL, 0 SKIP | `fixture-contract-green.log` |
| Broad focused PG: attribution/Ads/MetaAds/MetaConnect/MCI10/KC03/upgrade/T06 | 0 / 92 PASS, 0 FAIL, 4 prerequisite SANDBOX SKIP; 544.664s | `focused.log` |
| Focused customer consent/erasure/schema PG | 0 / 5 PASS, 0 FAIL, 0 SKIP | `focused-customer-boundaries.log` |
| `test-local.sh --browser-ads-attribution` | 0 / report 6/6, checkout PASS; Go 2 PASS / 85.145s | `browser-attribution.log` |
| `test-local.sh --browser-click-sweep` | 0 / 123 pages, 0 load failures; controls 1004 PASS / 0 FAIL / 22 SKIP; journeys 18/18 | `click-sweep.log` |
| `node tests/deploy/platform-edge.mjs` (pinned Caddy / mock upstream) | 0 / 47 platform requests + 4 storefront edge cases | `platform-edge.log` |
| `release-gate.sh --strict --only G07` | 0 / 6,725 tests/subtests PASS, 0 FAIL; 2,027 top-level PASS; 13 accepted prerequisite SKIP | `G07-console.log`, `g07-final/`, `g07-machine-load.log` |

Exact focused regexes and runnable command wrappers: `run-final.sh` and
`run-boundaries.sh`. Do not edit an evidence script while it is running.
`start-quiet-g07.sh` requires three samples 30 seconds apart with both 1-minute
and 5-minute load below the machine's logical CPU count. It checks the final SHA
and source cleanliness before launching G07; the gate also samples load throughout.
G07 ran **04:44:52–05:50:00 UTC** (foundation package 3,670.196s). The source
remained unchanged. Preflight passed; the run was **not continuously idle**:
131 samples, 1-minute load median 5.53 / peak 20.87, six samples at or above 10;
5-minute load stayed below 10 (peak 8.51). See [G07-LOAD.md](r13-review/G07-LOAD.md).
[RUNNER-NOTE.md](r13-review/RUNNER-NOTE.md) records an earlier wrapper error;
it never disguises a failed gate as green.

## AT1–AT9 / evidence scope

AT1–AT5 and AT7–AT9 final focused/browser checks, plus final full G07, have passed.
AT6 SANDBOX and AT9 LIVE remain explicitly NOT_RUN.
REAL_PG timing is cold (ANALYZE after synthetic bulk seed, no report warmup).
On final `4acbad53`, 100 drafts / 10,000 synthetic collected COD orders returned
HTTP 200 in **2.561327167 seconds** (101 drafts: **2.514677458s**; direct runtime
SQL: **2.813659166s**). `TestAdsAttributionR11ReportCapAndDeadline` in `focused.log`
records exact data sizes, truncation and timing; the 5-second deadline was not increased.
The independent full G07 rerun also returned HTTP 200 in **3.008085583s** for
100 drafts / 10k orders and **3.1868625s** for 101 drafts.
Real-click matrix: **390×844 / 1586×992 × zh-TW / zh-CN / en**.
Final report screenshots and six click manifests:
`output/playwright/ads-attribution-report/20261004T042014.931920000/`.
Portable evidence copies: [report](r13-review/browser-final/report/),
[buyer checkout](r13-review/browser-final/checkout/). Checkout original:
`output/playwright/ads-attribution-checkout/20261004T042100.787874000/`.
Independent visual rescore resolved the ID-wall P1, no new blocking regression;
the reviewer did not rerun the browser. Root also inspected TW mobile / EN desktop.
Only provider edges use MOCK; PG, transaction guards, workers and Next UI are real.

## NOT_RUN / external prerequisites

- **AT6 SANDBOX**: owner's dataset/test-event prerequisites; not authorized/run here.
- **AT9 LIVE**: suitable live video and actual read_insights grant/App Review prerequisite.
- Owner/integrator must add optional read_insights to the remote Meta Login for
  Business configuration selected by config_id, then reconnect and verify it.
  The frozen contract forbids URL-scope override; this unit makes no Meta mutation.
- Optional R04 Node binary suite: `COMMERCE_R04_LIVEKIT_BINARY` unset.
- G07+ recorded 13 accepted SANDBOX/LIVE/migrator prerequisites, not PASS:
  exact names in `r13-review/g07-final/G07.skipped`, inherited acceptance reasons
  in `G07-console.log` / `g07-final/results.tsv`. No historical owner LIVE result
  printed by that ledger is claimed as a live test performed in this unit.
- Accepted P3-10 owner-role RLS and P3-12 order-level approximation remain
  documented. No buyer age/gender collection; no Pixel installed.

Frontend-architect preserved strict API models/shared formatting; Playwright
kept acceptance as actual clicks and persisted readbacks rather than DOM mocks.
Impeccable's bounded hardening pass addressed the large-ID mobile case while
retaining every identifier; its mechanical detector returned no findings.
