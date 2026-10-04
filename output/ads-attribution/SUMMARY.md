# ads-attribution — R9 delivery evidence

## Current verdict

**LOCAL GATES PASS — ready for integrator review; not a production release verdict.** The PT404 blocker in checkpoint `4b338a89` is resolved. Final G07 on `068874fc` exits0: **2007 top-level PASS,0 FAIL,10 top-level SKIP;6687 tests/subtests PASS,13 total prerequisite skips**. Exact0113 ACL inventory and D9 AST guard contract drift are corrected without new authority. The English desktop header collision has independently verified browser RED→GREEN evidence. No source changed during the final run. Final G04 exits0; the delivery commit changes evidence only, not the tested source.

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/ads-attribution`; branch `unit/ads-attribution`.
- Final product/test source: **`068874fc62a6f580ccd97ce55d7eb3b17534c641`**. Backend product source is unchanged from `492aaa30`; later source changes register exact migration/ACL/AST guard contracts and isolate report table column sizing. Final-source evidence is under `r11/`; runtime/test source remains frozen throughout final gates.
- Authorized merge `9efcb15d` contains `r3/integration cd61add5` (and the previous Amendment 1 baseline). Migration **0113** is this unit; **0112 refusal fields remain unchanged**.
- Frozen068874fc final static gates,39focusedPG,2browser gates,click sweep and fullG07 all exit0. The new layout negative fails on old CSS with41.453px overlap and passes with24px separation.
- Final068874fc G07 began **2026-10-04T01:38:00Z**, after three quiet samples (load1=6.08/5.73/4.34, zero other foundation.test processes), and returned exit0 at02:37:12Z. `r11/G07-start.log` and G07 header record the exact source; `r11/source-postflight.log` confirms the same HEAD and runtime/test diff exit0. Only output artifacts changed. `r11/machine-load.log` has119 samples from the run (min2.03,max16.31); the separate live monitor observed18.50. A concurrent unrelated Rust test/system load spike was observed; **this was quiet at launch, not continuously idle**. See `r11/G07-LOAD-NOTE.md`. No foreign process was stopped. Historical28097ecf RED remains under `r9/g07-final/`.
- No push, deployment, production operation, Meta mutation, sandbox event, real buyer data or credential access. Graph/PSP interactions exercised by this unit are local **MOCK**.
- AT6 SANDBOX and AT9 LIVE remain the explicitly accepted prerequisite **NOT_RUN** items, not claimed passes.

## Decisions / implementation

| Decision or ruling | Implemented behavior | Evidence/status |
|---|---|---|
| D1/D7 | Order facts and Meta figures have separate labels and are never added; report remains read-only; recent Meta figures are labelled provisional | Exact populated PG and six-matrix browser PASS |
| D2/D3, R2/R5 | First-party signed touch, host-only HttpOnly Secure Lax cookies; last valid click within 7 days; rolling 90-day fbp independent of touch; fbc replaced only on new fbclid; no initial ad cookies without ad parameters or touch without lc_ad | Cookie tests + final PG/browser PASS |
| R2/R9 | Narrow `orders.order_attribution`, not an edit of the immutable financial snapshot; Begin freezes once using the existing 0088 scope/session/one-minute guard | R9 REAL_PG PASS |
| D4/R4/R6 | Exact claim → intake comment → post identity, not a session-wide source fan-out; click wins; multiple simultaneous boosts keep draft_id NULL without duplicated/split revenue | AT3/AT4 PG PASS |
| D5/R3 | CAPI pseudonyms, IP and hashed email only under the existing consent/policy/lease gates; event_id unchanged; non-consented orders still count internally | AT2 PG + pure policy tests PASS |
| D6 | Refunds reduce net revenue; collected COD counts once, uncollected COD remains separate | Exact refund/COD report PG PASS |
| D8/R3 | Erasure nulls fbc/fbp/client_ip; terminal CAPI clears IP; bounded purge covers no-operation retention; county aggregation rejects free-text PII | Privacy/terminal/no-operation negative PG tests PASS |
| D9/R7 | Five breakdown dimensions stored as replacement snapshots; account timezone persisted; hourly buckets become absolute time; daily figures retain Meta account day and label | AT8 PG/MOCK + browser PASS |
| D9/R8 | Scoped Page-token aggregate GET route, explicit refresh with idempotency fence, exact lease and permission checks; insufficient data is unknown rather than zero; demographic buckets are view time, never buyer demographics | AT9 PG/MOCK/browser PASS; LIVE NOT_RUN |
| R1 | 0112 retained, attribution is 0113; upgrade inventory and exact new function ACLs registered | R2/T06 final PG PASS |

R7 fetches timezone once per independently queued draft/day read and reuses it for the five breakdown requests; this is not a global cache. No buyer age/gender collection, Pixel or third-party tracking script was added.

### R9 specifics

`e2d22d0a` removes both attempted xmin/transaction-lock creation guards. No transaction marker was added. `buyer.resolve_scope` plus tenant/store/owner/order/creator-session and the one-minute predicate gates the insert; `ON CONFLICT(order_id) DO NOTHING` prevents replacement. Ineligible orders and malformed/expired/foreign/missing optional web touches without another valid origin produce no attribution row and do not deny Begin. Malformed server parameters alone retain PT400.

Tests cover legitimate Begin, old order, another session, another store, second freeze, invalid/expired/foreign/missing touch, preserved committed checkout and PT400 internal-parameter negatives. Optional touch JSON marshal failure also discards measurement instead of checkout. Actual database/transaction errors remain atomic errors, not silently swallowed.

Nonblocking wording boundary: D4/AT3's independently verified boosted-comment origin remains eligible when a web click touch is absent/invalid. R9's no-row phrase is interpreted as no valid source, not deletion of the separate comment path. This interpretation was sent for clarification; no explicit reply is claimed. Independent scoped review found no source P0/P1, but recorded this P2 wording ambiguity.

## Logical commits

All implementation/test commits carry `Co-Authored-By: Codex <noreply@openai.com>`.

| Commit(s) | Change |
|---|---|
| `9efcb15d` | Merge integrator R9 cd61add5 |
| `e2d22d0a` | R9 creating-session guard and optional-touch marshal no-op |
| `3cdb7558`, `8863b8ce` | Independent R9 negatives; AT3 waits for the genuine next second instead of changing time assertions |
| `cf271943`, `a4494706`, `e541eea6`, `51de1c22`, `43b4aa79` | Real-contract synthetic fixtures: verifier, legal inventory/whole-TWD quantities, exact report grants/cleanup and actual daily Insights ingestion |
| `8b71956d`, `68025dd0`, `492aaa30` | Browser observes actual PUT destination; real mobile-nav clicks; date error scoped away from Next route announcer |
| `8684cf23` | Actual BFF fix: exact audience-read command uses the existing bodyless transport; Idempotency-Key, CSRF and store authorization retained |
| `18795560` | Prior accepted browser/static evidence, including historical reds |
| `28097ecf` | Exact 0113 upgrade count49 and four new audience helper ACL tuples; every prior privilege equality/denial retained |
| `8f81b4f7` | Register13 exact0113 privilege facts in§4.4 and MA02; retain all existing denies, require each new fact |
| `7933d449` | Exact synchronous fenced Finish SQL and two constant Page-loader callsites; retain old14 negatives, add escape/alias/rebind/async/nested and1+1-inventory adversarial fixtures |
| `068874fc` | Scoped intrinsic report table sizing; nonempty text Range nonoverlap and actual mobile focus/keyboard-scroll browser assertions; no financial/assertion threshold change |

Earlier complete implementation history is preserved in the old checkpoint and Git history: cookie/Begin/CAPI/schema/report, breakdown adapter, UI, audience Page-token custody, privacy and historical adapters. `e530bfdc` removed unnecessary direct checkout consent authority rather than widening frozen CB02 assertions. `781fa6e7` confines old-release attribution shims to historical test fixtures; final R2, KC03 and shim-boundary PG tests pass.

## Final068874fc commands

`r11/exit-codes.tsv` is the sequential gate ledger: all static, expanded focused PG, attribution browser, click-sweep and finalG07 phases exit0. Only output evidence files changed during the final run. Pre-freeze `layout-red` exit1 and `layout-green` exit0 rows are intentional regression evidence, not final gate failures.

| Final-source command | Exit | Evidence/count |
|---|---:|---|
| `go build ./...` | 0 | `r11/build.log` |
| `go vet ./...` | 0 | `r11/vet.log` |
| `git ls-files -z '*.go' \| xargs -0 gofmt -l`, assert empty | 0 | `r11/gofmt-files.txt` empty |
| `bash scripts/dev/check-gates.sh` | 0 | `r11/check-gates.log` |
| `bash scripts/dev/test-node.sh` | 0 | `r11/node.log`:359PASS,0FAIL |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `r11/admin-tsc.log` |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | `r11/storefront-tsc.log` |
| `bash scripts/dev/depmap.sh --check` | 0 | `r11/depmap.log` |
| `bash scripts/dev/test-focused.sh '^(TestAdsAttribution\|TestLegacyRuntimeIsolationAttributionShims\|TestLiveClaimsKC03Schema\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL\|TestMetaAdsMA02Schema\|TestMetaClaimsMCI10)'` | 0 | `r11/focused.log`:39PASS,0FAIL,0SKIP;101.536s |
| `bash scripts/dev/test-local.sh --browser-ads-attribution` | 0 | `r11/browser-attribution.log`:2GoPASS;59.135s; report and checkout-admin matrices each6/6 |
| `bash scripts/dev/test-local.sh --browser-click-sweep` | 0 | `r11/click-sweep.log`:123page cases,0load failures;978PASS/0FAIL/23SKIP controls;18/18journey steps;666.472s |
| Included platform runner in click-sweep | 0 | 30page cases,15SSR,16tag,390real clicks,30reloads |
| `LC_RELEASE_GATE_OUT=output/ads-attribution/r11/g07 bash scripts/dev/release-gate.sh --strict --only G07` | 0 | `r11/g07/G07.log`, `r11/G07-console.log`;2007top-levelPASS/0FAIL/10top-levelSKIP;6687tests/subtestsPASS,13totalSKIP; compiled068874fc; workload caveat above |
| `LC_RELEASE_GATE_OUT=output/ads-attribution/r11/g04 bash scripts/dev/release-gate.sh --strict --only G04` | 0 | `r11/g04/results.tsv`; final source068874fc and staged evidence; no key-shaped literal |
| `git diff --cached --check` | 0 | Generated Next progress trailing whitespace normalized only; historical diagnostic retained in `r11/staged-diff-check.log`; no assertion/result changes |

Final sweep raw directory: `output/playwright/click-sweep/20261004T012432.964267000/`; `output/ui-click-sweep/ledger.json` generated2026-10-04T01:35:37.282Z records1001 controls (978PASS/0FAIL/23SKIP), zero known/new failures or stale exceptions. SKIP reasons remain explicit. The outer pre-G07 orchestration was stopped only after this harness completed all cleanup; G07 was then separately launched with an exact process-name quiet check. No test was interrupted.

Local layout regression: `r11/layout-red.log` exit1 and `r11/red/` catch the exact English1586 text overlap. `r11/layout-green.log` exit0:2Go tests,53.517s; both report matrices6/6. Independent visual audit (`09ff3973-4a32-4417-8116-b8963fb02551`) confirms source068874fc fixes that P1, with no new390px page overflow. Impeccable's local layout audit influenced only intrinsic table columns; the existing design/scroll boundary remains unchanged. These pre-final visual runs do not replace the frozen-source gates above.

7933d449 history: all11 gates in `r10/exit-codes.tsv` finished exit0; no G07 ran on that source. The owned orchestration parent was stopped after successful click-sweep cleanup, before G07, to fix the newly observed local header collision. See `r10/PIPELINE-PAUSE.md`; no running test was interrupted.

## Previous frozen28097ecf commands (retained history)

All paths are relative to `output/ads-attribution/` unless otherwise stated. `LC_TEST_LOCK_WAIT=14400` was used for PG/browser/release commands.

| Command | Exit | Evidence / count |
|---|---:|---|
| `go build ./...` | 0 | `r9/build-28097ecf.log` |
| `go vet ./...` | 0 | `r9/vet-28097ecf.log` |
| `git ls-files -z '*.go' \| xargs -0 gofmt -l`, assert empty | 0 | `r9/gofmt-28097ecf.log` empty |
| `bash scripts/dev/check-gates.sh` | 0 | `r9/check-gates-28097ecf.log` |
| `bash scripts/dev/test-node.sh` | 0 | `r9/node-28097ecf.log`:359 PASS,0 FAIL; optional media binary suite NOT_RUN |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `r9/admin-tsc-28097ecf.log` |
| `pnpm --filter storefront exec tsc --noEmit` | 0 | `r9/storefront-tsc-28097ecf.log` |
| `bash scripts/dev/test-focused.sh '^(TestAdsAttribution\|TestLegacyRuntimeIsolationAttributionShims\|TestLiveClaimsKC03Schema\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | `r9/focused-final-green.log`:27 PASS,0 FAIL,0 SKIP;98.141s |
| `bash scripts/dev/test-local.sh --browser-ads-attribution` | 0 | `r9/browser-attribution-28097ecf.log`:2 Go browser tests PASS;54.308s; both report Playwright matrices6/6 |
| `bash scripts/dev/test-local.sh --browser-click-sweep` | 0 | `r9/click-sweep-28097ecf.log`:123 page cases,0 load failures;977 PASS/0 FAIL/20 SKIP controls;18/18 journey steps |
| Included platform runner in click-sweep | 0 | 30 page cases,15 SSR cases,16 tag cases,390 real clicks,30 reloads |
| `LC_RELEASE_GATE_OUT=output/ads-attribution/r9/g07-final bash scripts/dev/release-gate.sh --strict --only G07` | 1 | Historical red: `r9/g07-final/G07.log`, `r9/G07-final-console.log`; source28097ecf,2002PASS/3FAIL/10SKIP |
| `bash scripts/dev/depmap.sh --check` | 0 | `r9/depmap-final.log`; no dependency graph change after this run |
| `release-gate.sh --strict --only G04` | 0 | `r9/g04-final/results.tsv`,492aaa30; historical scan, superseded by final r11 G04 |

Click counts vary with visible controls in the synthetic run; no source assertion or threshold was changed. Historical28097ecf raw sweep: `output/playwright/click-sweep/20261003T234137.909560000/`. The current full ledger is generated at `output/ui-click-sweep/ledger.{json,md}` and `journeys.json` by each run. The attribution route is included. Destructive controls stop at confirmation; SKIP rows name the unexercised action.

## AT1–AT9

| Gate | Final local result |
|---|---|
| AT1 | PASS — cookie/BFF, scope/lifetime, R9 REAL_PG and six real checkout orders |
| AT2 | PASS REAL_PG/MOCK — consented signals, no-consent exclusion, unchanged event ID, erasure and terminal IP |
| AT3 | PASS REAL_PG — exact post, window, click precedence and ambiguous-draft negatives |
| AT4 | PASS REAL_PG/MOCK — exact sums with real test capture/refund facts and pending COD; Meta kept separate |
| AT5 | PASS BROWSER — real clicks,3locales×390/1586,6distinct orders |
| AT6 | **NOT_RUN SANDBOX** — owner dataset id/test_event_code and Events Manager evidence required |
| AT7 | Final068874fc click sweep PASS and fullG07 PASS exit0; accepted prerequisite skips remain NOT_RUN; no full release claim. Quiet launch with measured transient load spikes |
| AT8 | PASS REAL_PG/MOCK/BROWSER — dimensions, timezone, replacement and cross-store isolation |
| AT9 | PASS REAL_PG/MOCK/BROWSER — exact funnel, buyers, hourly timeline and audience panels; **LIVE NOT_RUN** pending read_insights |

## Screenshots / independent audit

Final068874fc delivery copies are under `r11/browser-final/{report,checkout,checkout-admin}/`. Exact raw sources under `output/playwright/`:

| Directory | PNGs | Coverage |
|---|---:|---|
| `ads-attribution-report/20261004T012338.726834000/` | 6 | zh-TW/zh-CN/en ×390/1586; includes nonoverlap/keyboard-scroll assertions |
| `ads-attribution-checkout/20261004T012359.129467000/` | 24 | same matrix × landing/checkout/placed/order |
| `ads-attribution-checkout-admin/20261004T012410.208980000/` | 6 | same matrix; readback of real checkout orders |

Copy identity is recorded in `r11/browser-copy-verification.json`; no runtime configuration, trace credentials or screenshot edits are part of the bundle. Root and independent auditor inspected the source-identical green English desktop and Chinese mobile report images.

Independent frozen-source evidence audit (`a456cf1f-04fc-4975-b510-9bcfe05aa2f9`) verifies59/59 copies,36/36 PNG hashes, six distinct orders through Begin200→placed→owned URL→same-draft report,72 checkout action records, and six sets of admin orders=6/net=0 readbacks. Six layout JSON files cover294 nonempty headers and222 adjacent pairs; minimum text separation24px. Both admin matrices are6/6. The independent audit did not launch another browser or PG process.

Historical28097ecf evidence (retained, not substituted for final068874fc):

The 36 PNGs and accompanying JSON manifests/ledgers are also copied byte-for-byte into `output/ads-attribution/r9/browser-final/{report,checkout,checkout-admin}/` for the committed delivery bundle; the raw directories below retain the original runner outputs. Generated runtime configuration is not included in that bundle.

| Directory | PNGs | Coverage |
|---|---:|---|
| `ads-attribution-report/20261003T234044.665264000/` | 6 | zh-TW/zh-CN/en ×390/1586 |
| `ads-attribution-checkout/20261003T234104.025024000/` | 24 | same matrix × landing/checkout/placed/order |
| `ads-attribution-checkout-admin/20261003T234115.687990000/` | 6 | same matrix |

Read-only independent auditor verified **36/36 SHA256 manifests**, all six cases and six distinct committed ad-click orders matched to Begin200 and persisted buyer-order URLs. Ledgers contain **168 mixed action/observation records**, not168 literal clicks. Mobile storefront PNG width1024 is expected device pixels:390 CSS pixels×Pixel7 DPR2.625; every shot first checks `scrollWidth <= innerWidth+1`. Report/admin PNGs use DPR1. Root inspected the final zh-TW desktop report and previous identical-source en mobile report. No screenshot was edited to remove overflow.

Independent source review covers R9, optional-touch failure behavior, exact Page audience custody, BFF bodyless/key/CSRF boundary and final ACL inventory update. This is separate from root runtime gate execution; the author is not the only reviewer.

Final read-only SUMMARY/evidence audit (`c6a7c23c-594e-463e-be32-b4b8906b925d`) is CLEAR: independently counted2007 top-level PASS/0FAIL/10SKIP and6687 total PASS/13SKIP, verified finalG04 exit0 and output-only staged changes, and confirmed that integrator-review readiness does not claim production approval, continuous machine idleness, AT6 SANDBOX or AT9 LIVE acceptance. The reviewer did not edit files or run competing tests.

## History, risks, NOT_RUN

- Old `4b338a89` BLOCKED guard checkpoint is superseded, not deleted. All old red logs remain. The lifetime red→green, BFF bodyless red→green and exact-inventory25PASS/2FAIL→27PASS/0FAIL evidence are retained.
- Earlier G07 compiled3dab4b9f was invalidated by in-flight source changes; `G07-INTERRUPTED.md` records it. It is **not** the current final-source run.
- `r9/focused-final-source.log` was deliberately interrupted because it overlapped browser/foreign PG. Its harness printed a misleading exit0 after SIGTERM. **INVALID**, never counted as a pass; see `r9/FOCUSED-INTERRUPTED.md`.
- Repository test-lock coverage is incomplete outside Stripe/platform browser branches. This task did not modify the shared harness or delete another lock; final PG modes were serialized. Current load evidence is `r11/machine-load.log` / `r11/G07-LOAD-NOTE.md`; `r9/quiet-machine-load.log` is history. A continuously idle full-run condition was not achieved on this shared machine; the functional gate still passed without changed assertions.
- Older committed Next progress log trailing carriage returns/spaces were normalized for Git whitespace checks; assertions/results were not changed.
- AT6 SANDBOX; AT9 LIVE; production rollout/provider write acceptance are NOT_RUN. Optional `tests/media/r04-input-runner.test.mjs` is NOT_RUN without `COMMERCE_R04_LIVEKIT_BINARY`. Final G07's13 prerequisite skips are in `r11/g07/G07.skipped`: ten top-level cases (`TestStripeSP16Sandbox`, `TestCustomersBillingCB10Sandbox`, `TestMailPA12SMTPProbe`, `TestMetaAdsSandboxS1`–`S4`, `TestMetaClaimsMCI11LiveReadOnlyProbes`, `TestStripeSL08RestrictedKey`, `TestStripeRF10Sandbox`) and three nested cases (`TestManualFulfilmentMF02Schema/populated_upgrade_after_0062`, `TestStripeRF03Schema/populated_upgrade_from_0061`, `TestStripeSP21Registrar/sandbox_probe_against_real_stripe`). The runner labels these accepted NOT_RUN; inherited references to owner's previous LIVE work are not this unit's verification.
- No buyer age/gender collection; no cross-device or modelled attribution; cookie loss may under-count our click path. Buyer reports and Meta audience are never joined at individual level.
- Root task `6d5e18a9-696a-4671-b9bf-6c154f066042`, agent `codex-ads-attribution`; root owns only this worktree. Independent authors used ads-attribution-insights/ui/tests worktrees; reviewer `ps_security_final` was read-only. No model override; runtime model identifiers are not exposed in the live collaboration inventory.
- Skills used: frontend-architect for existing BFF/authority boundaries, Playwright for genuine browser interaction; existing UI checkpoint used impeccable audit-first. Incremental code indexing and Humaux links record the guard, fixture and BFF rationale.
- Humaux/canvas records finalG07 PASS and the evidence-only handoff; no push/deploy is authorized.

### First full G07 repair

`r9/g07-final/` is preserved as a completed RED run:2002PASS,3FAIL,10SKIP,exit1. Failures were `TestMetaAdsMA02Schema`, `TestMetaClaimsMCI10SecretClaimOnlyInLoadSecret`, `TestMetaClaimsMCI10AdsRefusalFinishException`. Independent review traced these to13 concrete0113 domain-owner seam grants absent from the old exact inventory, and D9 callbacks that no longer fit the old single-SQL/function-local-literal AST guard. No SQL/production changes were needed. Corrected exact signature/required sets and contract§4.4 preserve all former denials. The new AST guard preserves14 original refusal negatives and enforces exact SQL, synchronous assignment, ordered lease fields, no parameter escape/rebind, and exactly one call at each authorized LoadSecret site. Reviewer counterexamples first failed (`r9/guard-inventory-red.log`), then passed. Final pure MCI10:11top-levelPASS,47fixturePASS (`r9/guard-final-green.log`); independent reviewer cleared the scoped P2 without weakening assertions. Repair focusedPG39PASS (`r9/focused-guard-repair.log`) precedes final freeze, and is not substituted for finalG07.

## Handoff / remaining acceptance

- Runtime/test source is frozen at068874fc. Evidence-only commits after it do not change the inputs compiled by final G07. Inspect this branch's final Git log for the evidence commit, and compare it to068874fc excluding `output/`.
- Task-owned PG/browser/sweep processes ended and their harnesses completed cleanup. Read-only postflight found no running foundation.test or ads-attribution Next process; surviving Next servers have other worktree CWDs and were left untouched. Docker has only the pre-existing Humaux containers, not an LC test container. No locks were deleted and no foreign process was killed. See `r11/POSTFLIGHT.md`.
- Integrator review remains required. External AT6 SANDBOX and AT9 LIVE remain NOT_RUN until the owner prerequisites are available. The shared-machine continuously-idle condition was not achieved; the measured quiet launch, transient spikes and functional PASS are disclosed separately. No further local source fix is pending.
