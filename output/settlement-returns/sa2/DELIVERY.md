# SA-2 settlement-return UI delivery

- Status: **PENDING_REQUIRED_CHROMIUM** (integrator ruling, 2026-10-26). Current-head Foundation has passed (partial **E3: Node / structural gates**);
  the native WebKit card preflight also proves **57/57 Node PASS** against baseline **41/57 PASS**.
  Required browser acceptance is the unchanged Chromium card mode, **STL1–STL7 including 390px**.
  WebKit acceptance is **NOT_RUN**, tracked separately in [issue #204](https://github.com/humaux/live_commerce_architecture_v1/issues/204).
  Its failed diagnostic attempt remains archived; it is not a SA-2 product regression or a required SA-2 blocker.
  No browser or full required-Gates PASS is claimed; strict admin typecheck is E1.
- Date: 2026-10-26. PR: https://github.com/humaux/live_commerce_architecture_v1/pull/201 . Base: `r3/integration`.
- Head: `ecdc21e630cb9d58c4b8c002efab8c653510236e`; base: `08ba731fc7afbcb6eb9686ababcfdc512bb09224`.
  Application bytes are unchanged from typechecked implementation `f68c3607c`. D212 groups browser cases; ecdc changes only the
  specifically authorized exact shell-registry array and its SA-2 write_path. Browser app/spec/fixture/harness bytes match d212.
- Existing worktree: `.claude/worktrees/sa2`; branch: `unit/sa2-settlement-ui`. No new branch, auto-merge or merge.
- Author: Codex-4, UI worker; exact current model subversion / effort UNKNOWN (not exposed in this runtime).
  Read-only seam inventory: explorer `sa0_scope_inventory`, requested `gpt-6.1-sol` / `medium`, same base; no file writes or tests.
- Authority: `card-sa2.md`, SA-2 in `sa-cards-20261026.md`, frozen contract §6.7–§6.9, SA-2 task entry, and integrator's explicit
  scope-extension reply for the three legacy Node suites / old settlement fixtures, and the latest explicit integrator
  ruling: require Chromium STL1–STL7/390px; WebKit NOT_RUN under issue #204; do not change brfStartAdmin or weaken assertions.
  No Figma, shared harness or backend changes.

## Changes

- `card-payments-settlements-model.ts`: required returned amount and flag, conditional valid return timestamp not before payout,
  strict unknown-key refusal, amount bound 0..100000000000, net identity includes returned funds exactly once. Original payout
  triple is retained. Returned state takes precedence over the historical paid flag; carrying a return is not a returned payout.
- `Settlements.tsx` / copy: separate positive returned-funds column in list and detail; wrapping warning text
  **撥付已退回 · 退回款會計入之後的結算單**, matching zh-CN/en copy, return time and original payout reference/time. Net always
  remains the server value. Existing guarded-read/session lifecycle is unchanged.
- Settings registry: only the `settlements` entry changes `nav:false` → `nav:true`; route, `billing:manage`, group and labels remain.
- Frozen `settlement-returned.json`: explicit MOCK DTO, with bounced historical payout, next statement carrying 125000 minor,
  net 173200, and a later zero-return/zero-net statement. It is not a claim that SA-1's new SQL has run.
- STL1–STL5 retain their real Go/PG chain and original scope; STL6/STL7 add real clicks in zh-TW, zh-CN and en at 1586px / 390px.
  The spec writes `sa2-click-ledger.json` with post-assertion native UI text plus hashed screenshots via the existing manifest.
  STL1–STL5 keep their original serial behavior inside their own group; the independent new fixture cases execute even if that
  pre-SA-1 real-chain group fails. Its failure still makes the whole gate fail; no waits, assertions or retries were relaxed.

## Authorized legacy assertion / fixture alignment

`contracts/tasks.json` changes **only** by appending the eight newly authorized concrete paths to SA-2's write_paths.
No other task field, card, acceptance threshold or timeout is changed.

| File | Old assertion / shape | New assertion / shape and reason |
| --- | --- | --- |
| `tests/admin/card-payments-model.test.ts` | Base statement omitted the two return fields; net formula and literal rows had six contributions. | Base requires `returned_in_minor:0`, `payout_returned:false`; retain every original malformed-money, line-sum, timestamp, payout and permission assertion. Literal rows keep every original value and add a zero return column; sum all seven contributions and compare the same server net at shifted index 8. Add missing/bad required fields, bounds, timestamp/flag consistency, private-key refusal and nonzero return assertions. |
| Same model suite, payout states | Four paid/pending/none assertions used `{paid,net}`. | Keep all four expected states, pass the now-required false return flag, and add the returned-state case with preserved original payout. |
| Same model suite, navigation | Both routes were nav:false; exact owner settings list was settings/team/billing. | Card page remains nav:false; settlement entry is exactly nav:true. Keep exact group/three-language/permission assertions and all three old entries, append only settlements to the exact list, as required by the SA-2 navigation change. |
| `tests/admin/card-payments-wire.test.ts` | Captured historical amounts parsed with old DTO / eight displayed cells. | Preserve every old amount, sign, fee-credit, line, count, payout-state and privacy assertion. Add required zero/false checks and the zero ninth cell; expand the literal sum to seven contributions without changing net. Add frozen returned-fixture state/amount/time/privacy checks. Header explicitly records that new envelopes are not a fresh post-SA-1 wire capture. |
| `tests/admin/card-payments-copy.test.ts` | Exact locale keys, contract terms, credential refusal, six formula column names. | Keep all existing assertions, add returned column to the formula-name check and exact three-language returned explanation / positive-credit label checks. |
| `fixtures/card-payments-wire/settlements-list.json` | Three historical server statements, return fields absent. | Each gains only `returned_in_minor:0`, `payout_returned:false`, matching 0200's frozen defaults. Original order, ids, times, amounts and payout triple unchanged. |
| `fixtures/card-payments-wire/settlement-paid.json` | Historical paid detail without return metadata. | Same required zero/false additions; original net 2400, payout triple and single line unchanged. No returned timestamp is fabricated. |
| `fixtures/card-payments-wire/settlement-carried.json` | Historical negative-net detail. | Same zero/false additions; original net −6800 and all three charge/refund/dispute lines unchanged. |
| `fixtures/card-payments-wire/settlement-pending.json` | Historical pending detail after debt carry and dispute reversal. | Same zero/false additions; original carry −6800, positive fee credit, net 4000 and both lines unchanged. |
| `tests/admin/settlements.spec.ts` (original scope) | Six signed contributions and net index 7. | Assert the same real server amounts and payout states plus returned contribution; exact net comparison shifts to index 8. Original waits, privacy checks and five cases are retained. New returned scenarios alone use the explicitly MOCK BFF fixture. |
| `tests/admin/shell-registry.test.ts:180` | Exact settings list `[settings,team,billing]`. | Exact list becomes `[settings,team,billing,settlements]`; every other byte/statement in this file is unchanged. Native RED: Foundation **40328001527**, job **121589842388**. Integrator explicitly authorized this one-line scope extension; commit **ecdc21e63** appends the test path in the same PR. No contains/subset assertion or permission relaxation. |

The four old settlement fixtures retain their captured historical money facts. Their **shape** is aligned additively with the frozen
new DTO; a fresh real Go/PG capture after SA-1 is still **NOT_RUN**. No loader silently upgrades a live response or defaults missing
return fields, and the production parser rejects the old shape.

## Commands and actual CI state

| Command / run | Source | Actual result |
| --- | --- | --- |
| `pnpm --dir apps/admin typecheck` (initial shared-dependency attempt) | implementation files | exit 1, missing workspace `@live-commerce/ui` / `format` links; retained in `typecheck-initial-dependency-gap.log`, not a product acceptance failure |
| `pnpm install --offline --frozen-lockfile --ignore-scripts` | worktree dependency preparation | exit 0; reused 49 packages, downloaded 0; no lockfile/source change, no test or application run |
| `pnpm --dir apps/admin typecheck` (private dependencies, final source) | f68c3607c executable bytes | **exit 0**, `typecheck.log`; only E1 |
| First browser-only RED dispatch `40324181093` | dd3252961 tests only | CANCELLED while queued after the expanded tests-only source superseded it; **NOT_RUN**, not RED |
| [RED 40324634792](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/40324634792), `gates.yml`, modes `["--browser-card-payments"]` | a341d70d11f233ac1527f0de96f0078cb63a6d7f, production unchanged | card job **121580160502 FAILED**, Node command exit 1: **57 tests, 41 pass, 16 fail, 0 skip/todo** at 13:01:58Z. Includes valid new DTO refused by the old parser, absent return copy and nav:false. Native `red.log`; Go/browser was not reached. Overall receipt aggregation still pending at observation time. |
| Earlier Foundation / required Gates / WebKit `40326301684` / `40326301526` / `40326330347` | f68c3607c | Superseded by the test-grouping commit; no acceptance claimed. WebKit cancellation requested. |
| [Foundation 40328001527](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/40328001527) | d2125d7a2 | **FAILED** job 121589842388 at the package-docs/gate-map step: `tests/admin/shell-registry.test.ts:180` still expects exact `[settings,team,billing]`, actual adds settlements. CI contract-drift summary errors=0; Node unit suites were skipped after this preceding failure. Native `foundation-current.log`. Scope granted and exact-list candidate fix pushed as ecdc21e63. |
| Earlier required Gates `40328001824` | d2125d7a2 | Superseded after the authorized snapshot update; not acceptance |
| [Foundation 40329587213](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/40329587213) | ecdc21e630cb9d58c4b8c002efab8c653510236e | **SUCCESS**, job **121595143023**, completed **13:34:31Z**. Package docs / gate map and Node unit suites both succeeded. The card copy/model/wire/BFF block has **51 tests, 51 pass, 0 fail/cancelled/skip/todo**; the exact shell-registry list passes. Native `foundation-green-40329587213.log` plus run/job JSON are archived; checkout is synthetic merge `8cce0397` of this head into the SA-0 base. `foundation-green-proof.json` records native-log hashes and the d212/current browser-input byte comparison. |
| [Required Gates 40329587275](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/40329587275) | ecdc21e63; PR merge ref is CI authority | QUEUED on the 13:46Z read; no result yet |
| [WebKit 40328006710](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/40328006710), same card mode with `LC_BROWSER_ENGINE=webkit` | d2125d7a2, matching app/spec/fixture/harness bytes on ecdc | **Historical diagnostic attempt FAILED** job **121591231264**, Gate exit **1** at **13:57:57Z**, Go `TestBrowserCardPayments` timed out after **40m**. Its Node preflight is **57/57 pass**, 0 fail/cancelled/skip/todo. The native Playwright log schedules 27 tests and records 13 failed login waits + 13 serial skips; the final case is interrupted with no terminal result. Zero browser passes. This is the card/settlement suite on WebKit; the final run receipts are separate from the failed Gate. Native tested head is d212, with matched current input hashes. **Acceptance item: NOT_RUN, issue #204**, per integrator ruling. |

Node suites, browser/PG modes, check-gates and other tests were **not run locally**. No local application/PG/browser process was
started. No local build/lint result is claimed. Git commits and pushes succeeded; PR #201 was created with `autoMergeRequest:null`.

## Risks / NOT_RUN / integrator handoff

- **Backend dependency:** this branch is based on SA-0 only. The new strict UI deliberately refuses pre-SA-1 Go responses lacking
  required fields. STL1–STL5's real Go/PG chain and fresh native wire capture must be revalidated after SA-1 merges. The observed WebKit failure
  is earlier, at shared login; it is **not** presented as an observed SA-1 DTO refusal. Do not erase this
  dependency with permissive defaults or browser rewriting of the existing real-chain cases.
- **Snapshot correction proved in CI:** current Foundation **40329587213** passed the exact list and the rest of its checks. The integrator approved `shell-registry.test.ts:180` after native CI RED. Ecdc changes exactly
  that expected array and appends only its path. The standing ruling permits identical settlements-route additions to other old
  exact lists only when all remaining assertions are unchanged and each occurrence is documented here; other scope changes still
  need approval. This does not authorize skipping or weakening any gate.
- **Partial Node red→green, browser still pending:** baseline Node RED exists (57 tests including the collector: 41 pass / 16 fail);
  current Foundation has the card block green (51 tests without the separate collector block: all pass, zero skips/todo).
  Those Foundation/baseline counts have different command compositions. The native WebKit preflight now also runs the same
  57-case composition and is **57/57 PASS** on d212 (same current inputs), so that is the exact Node red→green witness.
  New DTO, warning-copy, fixture and navigation assertions pass. Browser UI assertions were not reached because login fails;
  required Gates **40329587275**, including Chromium card job **121611482634**, remain queued on ecdc at handoff.
  Queued/cancelled is never PASS. Browser fixture results, when available, cover frontend rendering only.
- Cross-family independent review is pending and belongs to the integrator; the read-only inventory is not review approval.
- SANDBOX/LIVE Stripe and bank actions: NOT_RUN, outside this unit. No settlement write UI or external money action was added.
- Rollback: revert the SA-2 commits (`dd3252961`, `a341d70d1`, `f68c3607c`, `d2125d7a2`, `ecdc21e63`); no migration/data rollback.
- Native browser evidence destination: `output/playwright/card-payments-*/` (including Playwright log/results,
  `screenshots.json` and `sa2-click-ledger.json`). The failed run has native failure screenshots/traces, but no completed results JSON, successful `screenshots.json` or `sa2-click-ledger.json`; no successful UI evidence is claimed.
- Continuation **stopped at the fixed deadline 2026-10-26T14:58:00Z**; no new automation was created.
  The native automation update reported that `sa-2-pr201-ci-follow-up` no longer exists (cause UNKNOWN). A bounded read-only
  GitHub API monitor (session 34016) supplied the final observation, exited **0**, and is stopped. Required Gates continues
  on GitHub and is not cancelled. No further WebKit run or shared-harness work belongs to SA-2. No local tests or out-of-scope
  source changes were made. This is a pending handoff, not a completed unit.

## Historical WebKit diagnostic / separate issue #204

- Run **40328006710**, native source **d2125d7a233f00d1922332af604898ab6efd0425**, Gate **121591231264**.
  Current-head input equivalence is recorded in `foundation-green-proof.json`; only tasks and the old shell snapshot differ.
- Both pre-existing CPU1 and new STL6 traces show HTTP `POST /api/auth/login` → `GET /authorize` →
  `GET /api/auth/callback` → default-locale login failure. Callback Cookie header/cookie metadata are absent.
  There is **no `/api/stores` request**, so neither actual settlement DTO nor the new fixture UI is reached.
  Native snapshots show the login-incomplete alert; the failure is independent of settlement parsing.
- Source correlation (E1 diagnosis): `tests/foundation/browser_refund_fulfilment_test.go:76-79` discards the relay's
  engine-aware public origin and reconstructs an HTTP origin from its internal listener.
  `browser_admin_relay_test.go:79` supplies `browserFront` (TLS for WebKit), while `apps/admin/lib/auth.ts:378` always
  requires Secure cookies. The callback rejects a missing binding and falls back to zh-CN. Trace metadata alone cannot
  distinguish browser cookie rejection from HAR omission, but the source and redirect jointly localize the shared login boundary.
- Separate harness issue #204 owns the proposed upstream correction: have `brfStartAdmin` use **`relay.origin`**, retain Secure cookies and all existing
  waits/assertions, and prove it in native CI. This shared Go harness is **outside SA-2 write_paths**, so it was **not edited**. The integrator confirmed that this capability was never covered
  by the existing WebKit gate (896bf2497); SA-2 WebKit acceptance is **NOT_RUN**, not PASS or a remaining SA-2 blocker.
- Browser results: **13 failed waits, 13 serial skips, final STL7/en/390 interrupted, zero passes**.
  Of the 12 new STL6/STL7 cases, 11 failed before UI and the last has no terminal result. No case is claimed accepted.
  The aggregate mode receipt's spec_count is not treated as evidence of passed UI cases.
- Native archive: `webkit-40328006710-native-artifact.zip` (artifact **12656752118**) plus uncompressed job log,
  native job JSON, artifact metadata and mode receipt. `webkit-failure-proof.json` binds their hashes and observations.
  The supplied archive contains native failure screenshots and traces; custom success ledger/screenshots/results are absent.
- Chromium card job **121611482634** in current-head required Gates **40329587275** remains **QUEUED** at this handoff.
  Foundation **40329587213** remains SUCCESS. SA-2 is awaiting Chromium / backend revalidation and is not completed or independently approved.

## Latest required-run observation after issue #204 ruling

- Run **40329587275** at 14:26Z: 78 jobs registered; Chromium card job **121611482634** is **QUEUED** on
  `ecdc21e630cb9d58c4b8c002efab8c653510236e` (native CI checkout merge `8cce0397b88b9b7cb026c71bc997c24925075aad`).
- The run also has a failure in **`--browser-payment`**, job **121611479886**: `TestBrowserStripeCheckout/SU09_mobile`,
  expected cutoff text “This payment page can no longer be opened. Refresh this order for the result.” was absent;
  native readback was `REVIEW_REQUIRED`. This is a buyer checkout path; all buyer/payment source and this harness/spec
  are unchanged by SA-2. No settlement assertion has failed in that job, and no out-of-scope repair is attempted.
  This observation is not a full root-cause diagnosis or a claim that the required run is green.
- The authoritative native card result is still pending. Once it executes, diagnose any actual SA-2 failure without changing
  waits, fixtures' meaning, assertion strength or backend code; at most two targeted attempts for one blocker.
- The genuine Go/PG settlement chain still requires SA-1. Revalidate after SA-1 merges; frozen-DTO UI fixtures never substitute
  for backend proof. WebKit is NOT_RUN specifically because of #204, independently of that backend dependency.

## Fixed-deadline handoff — 2026-10-26T14:58:00Z

- Head remains **`ecdc21e630cb9d58c4b8c002efab8c653510236e`**. No source or documentation push was made during the watch.
- Required run **40329587275** is still pending. Its Chromium card job **121611482634** is **QUEUED**, conclusion absent;
  there is no current-source Chromium STL1–STL7 browser result to accept. This is not a backend refusal or UI PASS.
- The known `--browser-payment` failure is retained in the final native job snapshot; no other lane was repaired.
- `required-deadline-handoff.json` records exact status counts, failed jobs, source SHA and SHA-256 of the raw run/jobs/API
  observation log. Native CI remains running/queued; it was not cancelled.
- The bounded read-only monitor ended at the deadline (exit **0**); the automatic continuation is no longer present and
  is not recreated. Resume this same task only when the integrator requests the actual card result / subsequent run.
- Acceptance still requires unchanged Chromium **STL1–STL7 including 390px**, and the genuine Go/PG chain after **SA-1** merges.
  WebKit is **NOT_RUN**, explicitly tracked by **issue #204**. Node/structural evidence remains partial E3; whole-unit completion
  and independent approval are not claimed.

- Final native job snapshot additionally records **`--browser-cvs`**, job **121611481046**, FAILED.
  The native log locates the failure at `browser_taiwan_cvs_test.go:184` (MOCK admin browser exit 1);
  the buyer/WebKit half passed and SANDBOX was explicitly NOT_RUN. The admin Playwright error is in that job's artifact;
  no root-cause or unrelatedness claim is made from this Go-level summary, and no CVS repair was attempted after the deadline.

## 接手交接 — claude-sa2-cont（2026-10-27）

- PR #201，head `ecdc21e630cb9d58c4b8c002efab8c653510236e`（本次未推送任何源码）。必需门 run **40329587275** attempt 1：`--browser-payment`(121611479886)、`--browser-cvs`(121611481046)、`--browser-card-payments`(121611482634) 失败，其余完成。
- **payment / cvs 失败：判定与 SA-2 无关。** 失败位于买家结账 SU09_mobile（REVIEW_REQUIRED 文案缺失）与 CVS MOCK 管理端浏览器（WebKit 半段通过）；二者的 spec 不引用 settlements；SA-2 仅改管理端结算页/settings 路由/测试。基线 r3/integration 08ba731fc（run 40322461220）为绿。按指示已执行 `gh run rerun 40329587275 --failed`，现为 attempt 2，结果待定（NOT_RUN 直到完成；若仍失败，证据交 integrator，不修共享 harness）。
- **card-payments（Chromium）实际结果：22 通过 / 1 失败。** STL6、STL7（zh-TW/zh-CN/en × 1586px/390px，冻结 DTO MOCK）全部通过；唯一失败为 STL1 真实链路（settlements.spec.ts:111，期望 3 行，实得 0，页面显示「Settlements are unavailable」），STL2–STL5 因串行组被跳过。
- **根因（E2，证据：artifact gate-browser-card-payments id 12663223563 的 playwright.log / error-context.md）：** 基线 Go 后端不输出冻结 DTO 的 `payout_returned` 等字段，而 SA-2 的 model 按冻结契约严格要求这些字段，解析抛 unavailable。这是 SA-1（#198）的依赖，不是 UI 缺陷；不放宽 model 与断言。#198 run 40351516453 的同一 gate 也在 STL1 得 0 行（旧 UI + 新后端），说明真实链路需 SA-1 与 SA-2 同时在位才能全绿；任何一方单独合并都会让 STL1 红，建议由 integrator 叠栈验证（SA-2 基于 SA-1 head）。
- 范围扩展：无（未改动授权范围外任何文件）。WebKit：NOT_RUN（issue #204）。Go/PG 真实链路：待 SA-1 合并后复验，NOT_RUN。未开 auto-merge、未合并。
