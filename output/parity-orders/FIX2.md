# PARITY-ORDERS-FIX2 — retained single batch after integrator ruling

Status: authorized retained batch prepared for one push; current-head CI and GREEN PENDING. The 2026-10-31 integrator ruling supersedes every historical STOP entry below. No product-completion claim.
Date: 2026-10-31 Asia/Shanghai. PR #362, unit/parity-orders → r3/integration.

## Authority, ownership and rollback

Integrator's current FIX2 message, AGENTS.md, 架构.md §§0/1/2/26/27, contracts/merchant-orders-ui-v1.md privacy/lifecycle/MOU gates, and actual source/raw CI receipts govern. Current card supersedes the historical inline presentation with signed UX-orders. No design changes, new copy or Figma writes. Skills: ponytail, impeccable harden; Humaux development detail 7bdb220b-0798-4b1d-aff5-16ee94cae75f read. Code repo map read. Rejected subset consulted (not an exhaustive claim). Existing local records are historical, not current acceptance.

Dispatched implementation agent only; no delegation and no independent acceptance. Exact runtime model/effort UNKNOWN. Main checkout read-only; own worktree only. Existing dirty DELIVERY.md/FIX1.md/red.log and untracked material preserved.

Step 1: fetched and merged origin/r3/integration629f42152f864621a99ac2247c3baf5d751a422f. Local merge commit f9d0deb6c4799907ad6f77e26585d074079020f5; pre-merge932937e7aead8773880a977e6b2a47a717cb84ac is the rollback reference. Merge conflicts in checkout-offline.spec.ts and home-cod.spec.ts retain both the existing disclosure helper and integration's real Edit settings click; browser ownership regenerated, not hand-edited. Merge commit has required co-author trailer. Merge-wide whitespace check found existing upstream evidence-log whitespace; no upstream evidence was edited. Resolved source paths pass diff check.

## Raw RED, base and complete failure table

Gates41090484680 attempt1 is completed FAILURE. Native head932937e7aead8773880a977e6b2a47a717cb84ac; synthetic tested mergeab50d04f2c36fcc6ee6f817d9699b80147002ee1. Job logs retrieved with gh run view --job <id> --log-failed; browser artifacts downloaded/read, no local browser execution. E3 HISTORICAL FAILURE only, not evidence for the draft.

Evidence shorthand (all under output/parity-orders unless a GitHub ID is supplied):
- O: artifact12998215543, gate-browser-merchant-orders-ui; fix2-evidence/orders/playwright/merchant-orders-c-browser/20261030T171713.358280963/playwright.log (33 cases:10 failed,23 passed).
- P: artifact12995610677, gate-browser-password-auth; fix2-evidence/password/playwright/run.eCuZlMLq/oc-20261030T164550.766850856/playwright.log (OC:3 failed,6 passed). TestBrowserPasswordAuth itself succeeds; TestBrowserOC fails inside the shared mode.
- W: artifact12994344180, gate-browser-webkit; original downloaded fix1-current-webkit/playwright/run.mHtc0HL1/taiwan-cvs/20261030T164608.282762295/cvs-buyer.mjs.log:17–32; fresh wrapper job in fix2-webkit-job.log:2151–2154.

| Failing job | Failing test / assertion | Evidence | Class / retained fix under 2026-10-31 ruling |
|---|---|---|---|
|124185561381 bundle:08|OC-04 zh-TW orders; reauth.spec.ts:188 orders-export hidden after reload|P:15–42; fix2-password-job.log:2216,2735|(a) Refresh resets signed advanced disclosure; real-click it again. Preserve replay [403,200], same URL, exact CSV header, one download and no download on refresh.|
|124185561381 bundle:08|OC-04 zh-CN orders, same assertion|P:46–72|(a) Same shared orders-only reload click; finance path unchanged.|
|124185561381 bundle:08|OC-04 en orders/mobile, same assertion|P:77–100|(a) Same real disclosure click, same three-language coverage.|
|124185562324 bundle:09|PARITY-ORDERS list zh-TW/390; orders-ui.spec.ts:168 selection bar expected visible, hidden|O:39–63|(b) Legacy mobile display:none hid signed selectable-card actions. Scoped parity CSS restores wrapping bar and existing actions.|
|124185562324 bundle:09|PARITY-ORDERS list zh-CN/390, same assertion|O:119–143|(b) Same CSS root fix; no locale-dependent branching.|
|124185562324 bundle:09|PARITY-ORDERS list en/390, same assertion|O:199–223|(b) Same CSS root fix; original entry click and path assertions retained.|
|124185562324 bundle:09|MOU01/04; orders-ui.spec.ts:495 orders-refresh hidden|O:279–311|(a) Back/page change remounts collapsed signed filters. Real-click existing revealOrderFilters before Refresh; frozen-money/state/privacy assertions unchanged.|
|124185562324 bundle:09|MOU07; orders-ui.spec.ts:974 orders-list hidden on an empty queue|O:330–354|(b) Zero-row card container had zero height while empty text was its sibling. Move the existing localized empty state inside the list. Keep list visibility, all SQL totals/counts and queue/filter coverage.|
|124185562324 bundle:09|MOU08 private cursor=false; order-detail expected0 received1 at1143|O:356–379|(a) Signed standalone detail already unmounts the list, so the old list-disappearance wait returned before the20s poll. Await that exact403 response within the SAME25s poll budget, then retain all list/detail/forbidden/noPII assertions.|
|124185562324 bundle:09|MOU08 private cursor=true, same assertion|O:395–418|(a) Same403 synchronization for POST private search; no permission/privacy assertion relaxed.|
|124185562324 bundle:09|ORDERS-V2-IMPL mobile later queue zh-TW; cancelled click hidden, read timeout|O:434–482; trace call@1081/1086 isVisible, call@1088 click|(b) Shared setup helper sampled menu presence during locale navigation and skipped opening it. Harness viewport selects mobile, await visible menu, real click, assert expanded. Await requested locale URL.|
|124185562324 bundle:09|ORDERS-V2-IMPL mobile later queue zh-CN, same assertion|O:498–546|(b) Same shared helper. Real Back/refresh/active queue/target assertions retained.|
|124185562324 bundle:09|ORDERS-V2-IMPL mobile later queue en, same assertion|O:562–610|(b) Same shared helper, original30s test timeout unchanged.|
|124185561277 --browser-webkit|TestBrowserTaiwanCvs/WebKit, buyer CVS; cvs-picker.mjs:387 getByTestId('cvs-candidate-024793') expected visible5000ms, element not found|W above|(c)-candidate, OUTSIDE CARD, per integrator ruling2026-10-31. Integrator compares with base41107905131; identical base assertion remains unproved. No CVS code/test edits.|
|124214325754 receipts|three failing modes: password-auth, merchant-orders-ui AND webkit|fix2-receipts-job.log:618–620,688|Dependency failure, not receipts-only; no missing browser-spec marker repair warranted.|
|124214689095 required|gate=failure, receipts=failure; aggregate=success|fix2-required-job.log last line|Dependency failure; aggregate succeeded. No gate/status bypass.|

Base Gates41107905131 at629f42152 remains pending in a fresh GitHub API read. One job-list API read returned502 (transport failure, not CI failure). Relevant buyer CVS files have no PR diff against current integration, but source identity does NOT establish identical runtime failure on base. No base failure log line exists yet; class(c) is unproved. Never claim that this pending base is GREEN.

## Prepared diff, validation and scope

Draft files: MerchantOrders.tsx, orders/parity.css, orders-ui.spec.ts, oc/reauth.spec.ts. No new imports, routes/specs, strings or dependencies. Existing headers remain. Browser ownership regenerated with derive-browser-ownership.mjs and --check. Dependency map regenerated by scripts/dev/depmap.sh. No baseline/dispatcher/migration/contract/OpenAPI/go.mod/lockfile/R2 pin edits.

Static checks only: node --check for the two changed specs exit0; strict tsc of current admin sources plus both changed browser specs exit0 (fix2-tsc.log, empty output is success); temporary read-only dependency symlinks created only in this worktree and removed. Static ownership and depmap outputs: fix2-ownership.log and fix2-depmap.log. Local test/server/browser/PG/container/provider execution: NOT_RUN, prohibited. Own acceptanceE0. No current-draft runtime acceptance, click ledger, screenshots, density or independent review; all NOT_RUN. Existing causal RED tests-only7093800f/run41052482621/jobs124054769784+124054770244 remain recorded in red.log; no redundant RED push for this directed continuation.

Outside original source write paths needing integrator approval: tests/admin/oc/reauth.spec.ts (existing direct orders export harness); generated output/ci-select-backend-browser/browser-ownership.json. Merge-only conflict resolutions: tests/admin/checkout-offline.spec.ts, tests/admin/home-cod.spec.ts. Other upstream changes arrive through the mandated integration merge, not authored here. PR body updated with this blocker report and outside-path list; no new code push after STOP.

UI process/facts/languages: existing signed30-frame REQUIRED-fact checklist, BOURDIEU Q1–Q6, project understanding, Mobbin/Eagle reference principles, Figma official guidance and three-language copy map remain in DELIVERY.md. This patch adds no new screen, copy or fact. Dynamic mobile actions now remain available; empty-state text stays localized. New visual/click/density evidence is pending CI and independent review, never inferred from static checks.

## Historical STOP and handoff — superseded by ruling below

Integrator must adjudicate the omitted out-of-module WebKit failure or provide its actual base result/log. Do not implement storefront changes in this lane. Do not push, rerun CI or take a third approach on a repeated assertion without that ruling. The one batch is prepared for review, not verified. No new WAIT_RUN exists because no push occurred. Existing base41107905131 is pending; integrator owns relay/wakeup. No background process left running.

## Draft file hashes (E1 scope only)

- `apps/admin/components/MerchantOrders.tsx`: `ee818be89030f1bcfc719ed367996346dcc7f59914c9b1bf7a86587c6c11d161`
- `apps/admin/components/orders/parity.css`: `7e5c7ace25c902a761c975cd4af1b8fb8c44ab0c83e98b7b75aa9b825a92cb18`
- `tests/admin/orders-ui.spec.ts`: `6c1d39802057e1d5aaa7744ab28058493327973b5f70c109049c94cc46c45419`
- `tests/admin/oc/reauth.spec.ts`: `a548fa92f5485174cd3e16e6b5d21b21ad56a9d37b5b1a0645b90c53e860b9e1`
- `output/ci-select-backend-browser/browser-ownership.json`: `07a07ec6c20afa8997ca6000cd51d2a4924c8c74a16af9f98466ae14bde8df5f`

## Live recheck — 2026-10-30T18:44:50Z

Native GitHub reads in this resumed session confirm PR head remains `932937e7aead8773880a977e6b2a47a717cb84ac`; integration remains `629f42152f864621a99ac2247c3baf5d751a422f`, already an ancestor of local merge `f9d0deb6c4799907ad6f77e26585d074079020f5`. No additional merge is needed. Gates `41090484680` is completed FAILURE, with exactly the five failed jobs in the table above. Fresh `gh run view --job … --log-failed` reads confirm OC, orders and WebKit mode failures. Base `41107905131` is still QUEUED with no failed-job evidence, so class (c) remains unproved.

The existing out-of-card STOP remains in force. This session changed only this local handoff; source/test/generated draft files were preserved, with no commit, push, CI rerun or local test/server. No new WAIT_RUN. Existing base workflow `41107905131` needs the integrator's completion relay; remote failed PR workflow `41090484680` is terminal. E3 describes only the observed failure on the published head; draft acceptance remains E0 and GREEN/independent review NOT_RUN.

## Live recheck — 2026-10-30T18:46:43Z

Fresh `gh pr checks 362`, PR metadata, all three failed browser job logs, and base workflow metadata confirm the same published head `932937e7aead8773880a977e6b2a47a717cb84ac`. Remote integration remains `629f42152f864621a99ac2247c3baf5d751a422f`, already merged locally at `f9d0deb6c4799907ad6f77e26585d074079020f5`. Base `41107905131` remains nonterminal (`status=queued`, several backend jobs in progress); no identical base WebKit assertion is available. Its currently listed jobs do not include WebKit, so completion alone would not establish class (c).

The exact extra blocker remains job `124185561277`: raw job line2151 `FAIL: cvs (go test exit=1, parse verdict=1)`; downloaded original artifact log lines17–32 reports `getByTestId('cvs-candidate-024793')` not found within5000ms at `tests/storefront/cvs-picker.mjs:387`. The table above covers all five failed jobs, including downstream receipts/required. This is outside the authorized orders module, and is not proven to be a base failure. Under the card's explicit out-of-card STOP, no implementation commit, push, rerun, additional fix attempt or local runtime check was performed. Existing draft source/test/generated changes were preserved. New GREEN/WAIT_RUN: none. Integrator must relay the scope ruling or matching base browser evidence before the prepared single batch can be pushed; no agent background wait remains.

## Live recheck — 2026-10-30T18:49:26Z

Current native `gh pr checks 362` again lists five failed jobs: WebKit124185561277, OC124185561381, orders124185562324, receipts124214325754 and required124214689095. Fresh failed-job reads confirm WebKit `FAIL: cvs` at line2151, OC `FAIL: TestBrowserOC` at2216, orders `FAIL: TestBrowserMerchantOrdersUIRealChain` at1890; required reports gate=failure, aggregate=success, receipts=failure. Original buyer artifact assertion remains `cvs-candidate-024793` not visible within5000ms at cvs-picker.mjs:387. The complete classification table above remains applicable.

Remote PR head remains932937e7aead8773880a977e6b2a47a717cb84ac; remote integration remains629f42152f864621a99ac2247c3baf5d751a422f and git ancestry confirms it is already merged locally into f9d0deb6c4799907ad6f77e26585d074079020f5. Base run41107905131 is now in_progress, conclusion empty. A jobs-list request returned HTTP502, a transport failure rather than a test result. There is still no matching base WebKit assertion supporting class(c).

Explicit out-of-card STOP applies. No source/test changes, local tests/server, commit, push or CI rerun this session; existing draft preserved. Only this handoff updated. GREEN and current-draft acceptance NOT_RUN; authorE0, published CI failure observedE3. No new WAIT_RUN. Integrator owns next relay: scope adjudication or matching base browser evidence. No background process remains.

## Live recheck — 2026-10-30T18:51Z

Fresh native PR checks and failed-job logs again confirm remote head `932937e7aead8773880a977e6b2a47a717cb84ac` and the same five failed jobs. OC job124185561381 line2216, orders job124185562324 line1890 and omitted WebKit job124185561277 line2151 remain raw failures. The retained original artifact confirms `cvs-candidate-024793` absent within5000ms at `tests/storefront/cvs-picker.mjs:387`. Base41107905131 is still in_progress with no conclusion; no matching base browser evidence exists. Integration629f42152 is already an ancestor of local HEAD f9d0deb6c.

The explicit out-of-card STOP remains active: no commit, push, runtime test, server or CI rerun. Source/test/generated draft files preserved. GREEN and new WAIT_RUN: none. Integrator must adjudicate the omitted WebKit blocker or relay matching base evidence; repeating this unchanged dispatch cannot resolve it. AuthorE0; published failure evidenceE3.


## Current ruling and one-push handoff — 2026-10-31

Authority: integrator's current human message explicitly directs "Push your retained single batch now". It assigns buyer CVS job124185561277 to the integrator as class(c)-candidate, without proving an identical base failure. It permits tests/admin/oc/reauth.spec.ts as class(a), and checkout-offline/home-cod specs only for integration merge-conflict resolution. These permissions supersede the historical STOP paragraphs above; they do not authorize CVS changes or any third fix attempt.

Fresh fetch: origin/r3/integration629f42152f864621a99ac2247c3baf5d751a422f is already an ancestor of local mergef9d0deb6c4799907ad6f77e26585d074079020f5; git merge --no-edit reports Already up to date. Pre-merge rollback reference932937e7aead8773880a977e6b2a47a717cb84ac. All pre-existing task files preserved. No edits to other worktrees, main, protected contracts/baselines/dispatchers or storefront.

Fresh raw GitHub read: Gates41090484680 is completed FAILURE at native932937e7aead8773880a977e6b2a47a717cb84ac, with precisely the five failed jobs in the table. OC and orders --log-failed lines reconfirm their TestBrowser failures. Base41107905131 remains in_progress at629f42152 with empty conclusion; no base GREEN or class(c) assertion is claimed. Original causal RED remains41052482621/jobs124054769784 and124054770244 on tests-only7093800f; red.log contains actual assertion receipts.

Retained source batch unchanged: two product fixes (selection bar and empty-list structure); orders-only real disclosure clicks; shared mobile queue/locale readiness; synchronize with the denied poll before preserving every PII-removal assertion. No new import, route, locale string, dependency, fixture, timeout or threshold change. No extra approach beyond the authorized retained FIX2 batch; if its same assertion fails, BLOCKED with exact evidence, no third approach.

E1 static refresh, all exit0: node --check of both changed specs; strict tsc -p output/parity-orders/fix2-tsconfig.json; ownership regeneration then --check; bash scripts/dev/depmap.sh (unchanged generated map); python3 scripts/check_packet.py; scoped git diff --check. Logs: fix2-ownership.log, fix2-depmap.log, fix2-tsc.log, fix2-packet.log. Temporary read-only dependency links were removed. No local tests/server/browser/PG/container was run. Own acceptance E0; no current runtime/independent PASS. No new design deliverable: existing DELIVERY.md contains signed fact parity, languages and UI-process sections.

Outside original card paths this FIX2: tests/admin/oc/reauth.spec.ts (explicitly authorized class(a)); tests/admin/checkout-offline.spec.ts and tests/admin/home-cod.spec.ts (merge-conflict resolution only, retained both sides' real-click setup); generated output/ci-select-backend-browser/browser-ownership.json; output/parity-orders task reports/logs. Full PR's previous scope exceptions remain in the PR body. No newly authored storefront change.

Push one batch to unit/parity-orders, never integration/main. The resulting exact head and Gates/Foundation workflow run IDs will be posted in a PR comment and final reply, without an evidence-only commit. GREEN PENDING. Integrator owns the fresh-session wakeup; do not poll beyond10min. Humaux/coord/code-index interfaces are unavailable in this session, so no memory store, canvas or code-memory-link claim. Local evidence and the PR receipt provide the available trail. No new background process remains.
