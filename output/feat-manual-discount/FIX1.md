# FEAT-MANUAL-DISCOUNT-FIX1 — 2026-11-01 directed recovery fix

STATUS: WAIT_CI; author acceptance E0, static E1, existing failed CI E3 negative. No GREEN or completion claimed.
This section supersedes the older FIX1 dispatch report preserved below as historical context.

Authority: current integrator FIX1 dispatch; AGENTS.md; 架构.md §§0,1,2,26,27;
contracts/storefront-v2.md §G3 and manual-discount amendment; contracts/cart-quote-v1.md amendment;
current repository source and the GitHub run/job/artifact receipts below. Read contracts/tasks.json DAG.
Worker: Codex dispatched implementation lane, same worktree/branch; no recursive delegation.
Actual model/effort is not exposed by the runtime. Humaux memory/coord/canvas tools are not exposed;
no shared-memory read/write, claim, lock or canvas update is asserted. Installed ponytail guidance read.

Fetched and merged origin/r3/integration **4b8c04c7213025c173e260db6f9c81cb5b818c61** first.
Merge commit **769f1f9a6957eec99f06b2c5295c8c72829349a2**; prior PR head
**fdfec87c9d760c8d70f9b13f77f7de37f85d8e3b** is the rollback/reference point.
Preexisting dirty FIX4.md/PIN.md and all untracked raw evidence remain unstaged and preserved.
Base Gates **41277975588** is queued at readback; base GREEN/current class-(c) failure is UNKNOWN.
The prior base Gates **41270881361** at fde11c131 completed SUCCESS; that is historical evidence only.

## Every failed job → test → evidence → classification

Failed run **41277742564**, head fdfec87c9; synthetic merge tested **72b40a2485923dfd2fe40260834145e45785cafb**.

| Failed job | Failing test / assertion | Original evidence | Class / action |
| --- | --- | --- | --- |
| **124763804297**, gate --browser-webkit | **TestBrowserBuyerOrderUI**, BO05 publication revocation/restoration, order-gate.mjs:639 → created(:153): getByTestId('order-section') not visible after unchanged 5000 ms | Job log: order pass=0 fail=1, exit=1; artifact gate-browser-webkit (13072148027), playwright/buyer-order-2036392307/browser.log and failure-7.png. Visible state remains “Checking order status”; other six WebKit suites pass. | **(b), shared browser interaction/setup candidate**: recovery becomes visible during initialization, before the restored review and order-lock scroll settle. Add shared idle/enabled/focus preparation before the original pointer click. This is a source-supported race candidate; the artifact does not prove which request/handler failed. Root-cause acceptance remains pending real WebKit CI. |
| **124767027462**, receipts | matrix-receipts: RED (1 problem); receipt --browser-webkit FAIL, test_count=140 | Original failed-job log at 2026-11-01T06:01:17.0187706Z; same tested SHA/run/attempt 1 | Downstream of the WebKit member above; no independent receipts defect. |
| **124767335440**, required | gate=failure receipts=failure; plan/build/aggregate=success | Original failed-job log at 2026-11-01T06:02:21.2535512Z | Downstream; no separate product fix. |

Foundation **41277742565 / 124763245081** succeeded for the old head only. Aggregate
**124767027485** succeeded, so it is not listed as another failing job. No class-(a)
assertion replacement: the BO05 intent remains valid. No class-(c) claim: no identical
failure on the specified base has been observed. Known inbox failures are not present in this run
and the excluded inbox support/source files remain untouched.

## Change and acceptance gate

- tests/storefront/shop-helpers.mjs: one shared prepareCheckoutRecovery helper waits for the existing
  purchase-main aria-busy flag to clear, checks the recovery action enabled, focuses it and checks focus.
  It does not click, submit, retry, modify storage/DOM, create a key or call an API.
- tests/storefront/order-gate.mjs: invoke that preparation at all **8 existing recovery-click sites**;
  each original real click, business assertion, fixture, timeout, key/body check and ordering is retained.
  This applies the same native focus preparation already used for the quote-less confirmation and
  locale history actions in this suite (61951609f); it does not rewrite a signed UI expectation.
- output/ci-select-backend-browser/browser-ownership.json: regenerate from the merged source; --check exit 0.
- docs/engineering/dependency-map.md regenerated with scripts/dev/depmap.sh; no diff.

Attempt budget: **first directed recovery-readiness fix**, not a third approach. Prior PIN relays made
zero repairs to this blocker. If the same BO05 assertion remains RED after this push, stop with the exact
new assertion/job/artifact; do not add another speculative focus/retry/timeout change.
Acceptance: this PR head's own Foundation + Gates GREEN, including real Chromium/WebKit BO05 and gate
assertion parity, followed by integrator independent review. Author self-review is E0. No local runtime
check substitutes for the CI result. No product UI, copy, locale, Figma frame or business API changed.

## Verification and scope

E1 static checks, all exit 0: node --check on both edited mjs files;
python3 scripts/check_packet.py (PASS_PACKET_STRUCTURE_ONLY; timestamp-only generated change restored);
node scripts/dev/derive-browser-ownership.mjs and --check; bash scripts/dev/depmap.sh; git diff --check.
Evidence-retention gate will be run against origin/r3/integration before the single push.
The card's existing tests-only RED is retained in red.log: Foundation **41093578253 / 124193423117**
and Gates **41093578464 / 124197225702,124197225711,124197225869,124197225752,124197225919**,
head **fc28de173547886f74900c5caf4ab1f7a675b451**. Current recovery RED is the run/table above.
No redundant tests-only push: the directed card requires one batched fixes push after the existing RED.

Outside original feature source paths, for integrator review: **tests/storefront/order-gate.mjs** and
**tests/storefront/shop-helpers.mjs**, under this FIX1's shared-helper authorization. Generated ownership
and output/feat-manual-discount reports are explicitly permitted. No protected baseline/dispatcher pin,
migration, contract, OpenAPI, lockfile, other app/backend, exemption or gate policy was edited this round.
Evidence manifest is DELIVERY.md; raw downloaded logs/screenshots/artifacts are not staged.

NOT_RUN: all local runtime/unit/integration/browser/model tests and servers; CI acceptance of the new head
(until GitHub finishes); independent/E4 review; provider SANDBOX/LIVE and production deployment.
No task-owned runtime process, port, database, container or fixture was started.
After push, record Gates/Foundation WAIT_RUN IDs in a PR comment and final handoff; stop polling within
10 minutes. GitHub completion → integrator → fresh session is the wakeup path. No evidence-only push.

---

## Historical prior-dispatch FIX1 report (not current state)

# FEAT-MANUAL-DISCOUNT-FIX1 — BLOCKED_SCOPE

Date: 2026-10-31 (Asia/Shanghai). Author: Codex, dispatched by Claude integrator.
Evidence: E3 negative CI evidence for the old PR head; E1 source inspection only for the local merge.
No completion, GREEN, or independent acceptance claimed.

## Exact state and authority

- PR #394: unit/feat-manual-discount → r3/integration.
- Remote PR head: fc28de173547886f74900c5caf4ab1f7a675b451 (tests-only).
- Merged origin/r3/integration f59e5c5402a26021ca498d8323bc0e7ee8693c36 first.
- Local merge HEAD: 5e1b019d7e8c445521637572218f7a5023d46da3, not pushed.
- Gates RED run: **41093578464**, attempt 1; tested synthetic merge 80217e07380c45b66d35d61f96bbfa62e966dc10.
- Foundation RED run: **41093578253**, job 124193423117.
- FIX1's listed Gates job IDs belong to 41093578464, not 41093578253.
- Latest base readback: run 41098492538 at f59e5c540 has no conclusion; overall status queued, multiple gate jobs in_progress. Base GREEN and class (c) remain UNKNOWN.
- Sources: current FIX1 dispatch, original integrator card cxf-feat-manual-discount.md, AGENTS.md,
  架构.md §§0,1,2,26,27, contracts/storefront-v2.md §G3, contracts/cart-quote-v1.md,
  contracts/invariants.json, source and raw GitHub job logs.
- Humaux: original dispatched tests-only decision d1461ad3-cae7-423e-a892-ebf2a1be2c93;
  development detail 7bdb220b-0798-4b1d-aff5-16ee94cae75f. No new claim: dispatched-worker exemption.
- Rejected enumeration returned 5/9 (coverage 0.5556); not represented as a full inventory.
- Skills: ponytail and ci-only-evidence; card-authorized ownership generator overrides the latter's generator default.

## Failure classification

Class (a): no obsolete signed-frame assertions found; original card explicitly has no UI pages.
Class (b): product implementation required. Here these are deliberate new feature REDs, not regressions from an implementation.
Class (c): none proven. No completed base failure evidence is available; the base workflow has no conclusion.
Expected negative child output and downstream receipt failures are called out separately rather than misclassified.

| Job | Failing test / group | Exact evidence | Class and disposition |
| --- | --- | --- | --- |
| 124193423117 Foundation | picker thumbnail; named discount BFF grammar (three locales); strict manual-result parser | merchant-tools-model.test.ts:21 undefined vs authenticated thumbnail; :30 false !== true; :49 Error: unavailable. 501/504 pass, 3 fail. [log](fix1-evidence/job-124193423117.log) | (b) Feature not implemented; preserve all assertions. |
| 124197225711 bundle:02 / --browser-manual-order | Same three Node model tests | bundle member failure; receipt FAIL test_count=18, spec_count=0; [log](fix1-evidence/job-124197225711.log), [downloaded receipt](fix1-evidence/manual-order-receipt/receipt-browser-manual-order.json) | (b) Same model/BFF gap; this member failed before any browser spec ran. No click or screenshot acceptance inferred. |
| 124197225702 shard:unit | TestManualDiscountValidation: trimmed, thirty_unicode_characters, empty, whitespace, over_thirty, control, zero, negative, fraction, string_amount, money_overflow | manual_discount_test.go:78 normalized input lost discount; :57 must be 422 invalid_request, got nil. [assertions](fix1-evidence/unit-validation.log), [fail list](fix1-evidence/job-124197225702.log) | (b) ManualInput has no discount field, so JSON ignores it in this unit boundary; validation is absent. |
| 124197225869 shard:g05 | TestMerchantToolsManualOrder/FEAT-MANUAL-DISCOUNT: happy path; receipt/audit/replay; coded validation (empty, too_long, zero, negative, over_goods); shipping exclusion; permissions/scope; proportional allocation/stale version; pickup; cheap-last-line/overflow; COD cap; offline/report/refund cap | manual_discount_test.go:110,157,206,224,238,264,311,331,364,378: HTTP 400 invalid_json (expected creation, coded refusal or authority outcome). [all assertion lines](fix1-evidence/manual-http.log), [fail list](fix1-evidence/job-124197225869.log) | (b) Existing strict HTTP schema rejects the new field before new money behavior. Need full product implementation, not fixture/helper weakening. |
| 124197225752 shard:g06 | TestLiveConsoleOrderForBuyerLivePrice/FEAT-MANUAL-DISCOUNT | manual_discount_test.go:437: shared for-buyer pipeline must accept discount: HTTP 400 code=invalid_json. [log](fix1-evidence/g06.log) | (b) Same missing shared product path. Existing live-price assertions pass. |
| 124197225752 shard:g06 | Reported HistoricalPolicy matching/drift/tampered FAIL text | catalog_matrix_evidence_test.go:247 logs child ci-shallow-stays-enforcing exit=1; parent TestProductEditorMatrixEvidenceShallowClone PASS at 17:11:57.1027973Z. Source :229 expects exit 1. [context](fix1-evidence/g06.log) | **Not a failing gate/test**: expected negative child evidence. Leave unchanged; do not classify as (c) or use it as an unrelated blocker. |
| 124197225919 shard:g10 | TestCatalogV2Smoke/FEAT-MANUAL-DISCOUNT_picker_thumbnail | catalog_v2_smoke_test.go:258: picker list must add nullable cover_image_url. [log](fix1-evidence/job-124197225919.log) | (b) Catalog projection not implemented. |
| 124214127123 receipts | Matrix receipt verification | FAIL shard:unit, shard:g05, shard:g06, shard:g10, --browser-manual-order; matrix-receipts: RED (5 problems). [log](fix1-evidence/receipts.log) | Downstream of the five members; no independent receipts-only defect observed. |
| 124214304243 required | Required workflow summary | plan=success build=success gate=failure aggregate=skipped deploy=skipped receipts=failure. [log](fix1-evidence/job-124214304243.log) | Downstream; aggregate was skipped, not an additional failing job. |

All raw logs can be re-read using gh run view --repo humaux/live_commerce_architecture_v1 --job <id> --log-failed.
The manual-order artifact is on Gates 41093578464. It is a receipt, not browser execution evidence.

## Root cause and exact scope stop

The remote diff is seven test/generated files with **no production implementation**.
It is therefore incorrect to rewrite these new tests as obsolete UI expectations.

Original card item 3 says “Go handler + SQL in the owning module only”.
FIX1 hard scope rule says “Still STOP for ... other apps or backend packages you do not own”.
The latest dispatch supplies no shared backend assignment; PR comments/review comments were empty on readback.

At the local merged HEAD:
- internal/merchanttools/manual.go:540 creates a storefront quote; :573 calls checkout.Begin.
- internal/storefront/quote.go:50-65 has no adjustment line; :127 calculates only the existing promotion.
- internal/storefront/revalidate.go:96 recalculates the frozen quote with pricing.CalculateWith(policy, inputs, out.Promotion).
- internal/checkout/checkout.go:280 revalidates that quote, then marshals the typed quote into its order snapshot.
- migrations/post_river/0021_product_core_begin_hold.sql:72 requires supplied quote and stored immutable quote to match.

Changing only merchanttools would either lose the adjustment on typed reads or fail revalidation/snapshot equality.
Changing totals after Begin violates the existing money and immutable-quote gates.

Required integrator assignment before implementation: storefront quote creation/revalidation, pricing allocation,
catalog thumbnail projection and merchantorders read projection; confirm checkout/buyer readers as needed.
The contract amendments, assigned migration 0215 and R2 pin assignment also need to be covered by the resumed implementation card.
No numeric R2 pin is assigned in FIX1; do not choose one.

## Actions and acceptance boundary

- Integration merged locally once without conflicts; merge commit has required Co-Authored-By trailer.
- Ownership regenerated and --check exited 0; generated file has no additional working-tree diff.
- No source/test assertions, fixtures, exemptions, thresholds, baselines or dispatchers changed.
- No product fix attempted (0 targeted approaches), no implementation/evidence-only push, no new CI dispatch.
- Local logs and downloaded receipt retained under this task's output directory.
- No background process, server, fixture, port or container started; no temporary runtime cleanup needed.
- NOT_RUN: all local tests/servers; implementation/migration; current local merge runtime CI; GREEN; independent review; SANDBOX/LIVE; deployment.
- WAIT_RUN: none for this unpushed local merge. Integrator must resolve scope and relay a revised implementation card; do not relay the same old RED as a new failure.


## Current dispatch readback 2026-10-30T17:41:28.280669+00:00

- gh pr checks 394 reconfirmed the remote head is tests-only fc28de173547886f74900c5caf4ab1f7a675b451; all listed failures remain on that head.
- git fetch origin r3/integration and git merge --no-edit origin/r3/integration: exit 0, Already up to date. No additional merge or source edit.
- Re-read the original integrator card, architecture sections and owning contracts; no shared backend ownership assignment found. PR comments API returned [].
- Re-read current raw unit and required job logs; validation fails its own assertions, and required is downstream. One HTTP 502 reading the required job was retried once successfully; transport failure was not treated as a test result.
- node scripts/dev/derive-browser-ownership.mjs and --check: both exit 0, no tracked diff. No local tests or servers.
- Existing failure table and downloaded manual-order receipt retained; receipt spec_count=0 is not browser acceptance.
- Humaux tools unavailable in this session: memory/coord/canvas NOT_RUN; no memory-derived fact is asserted as current authority.
- BLOCKED_SCOPE remains under the explicit STOP rule for unowned backend packages; no fix attempt, push or new run. WAIT_RUN: NONE.
