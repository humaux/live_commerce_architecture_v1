# PARITY-GROWTH — implementation candidate, GitHub CI pending

Current section: **Relay 4 route-fixture repair** below. Historical RED-stage status is not current acceptance.
Evidence: implementation author E0; local static E1; causal GitHub RED E3 (MOCK). GREEN / browser parity / independent review: PENDING.

## Relay 4 route-fixture repair

Authority: current PR head `5ef4cc44857ff92e40e551edf484e851d136c182`; Gates run `41075396782`, failed job `124143813883`, tested merge `9b3ab3af402c500b69add3923c115ac1b524c713`. This relay changes no UI or Figma design. Existing signed-frame references and deferred data fields still apply.

- **Root cause / attempt 1 of 2:** artifact `12989464352` shows 15 passing ads tests and two failed desktop/mobile loading-state cases. The real read was released and interception removed immediately; the CI trace shows `setNetworkInterceptionPatterns` (empty patterns) before the delayed `Route.continue`, which then failed with `Route is already handled!`. The prior artifact-name repair succeeded: screenshot uploads completed. This is a separate fixture-lifecycle blocker, not a new assertion RED.
- Wait for the real refresh control before installing the read delay. Track every continuation promise, release the delay, await all continuations, then remove interception. Errors still propagate; no catch/ignore, timeout/retry increase, invented response, skipped assertion or disabled test. All real-click journeys and both loading screenshots remain.
- `relay4-assertions.log`, `relay4-bundle03.log` and `relay4-receipt.json` preserve the original failure and ordering. Downloaded ZIPs were removed after extracting these receipts. Static syntax, ownership regeneration/check, packet validation and whitespace checks exit 0 (E1); source hashes are in the receipt. No browser/test/server was run locally. No component imports changed, so no harness stub change is needed. No dependency-map change is needed for this browser-fixture-only edit.
- Playwright cleanup authority, checked 2026-10-30: https://playwright.dev/docs/api/class-page#page-unroute-all documents that default removal does not wait for running handlers. Here explicit promises finish before interception removal, preserving errors instead of ignoring them.
- Current candidate GitHub browser/PG acceptance, whole-PR GREEN and independent parity review remain PENDING. Humaux MCP is unavailable; no store/canvas/coord claim. The integrator completion relay is the wake mechanism. No merge/auto-merge/deploy or external ad action.

## Relay 3 targeted repair (historical)

Current baseline: `f239dbc50b89df1a0c2c622a18f269bfc0d51552`. Exact-head Gates run `41065293413`, job `124113569764` (`bundle:04`), tested merge `b42b92a81ccee6ca56f3c586dd1a3123356f1914`.

- **Root cause / attempt 1 of 2:** every bundle member returned success, but `Bundle member 1 evidence` failed: GitHub upload-artifact rejects `parity/1151:9420.png` because of the colon. This is an evidence-publication failure, not a new assertion RED and not GREEN. `relay3-upload-failure.log` preserves the filtered raw receipt. The original causal RED remains `41054278594 / 124066658713`.
- All three owned screenshot emitters now use `parity/<node>-<node>.png`; Figma ids retain their original colon in `frames.jsonl`, density measurements, the parity sheet and manifest frame fields. Each emitter asserts the safe filename grammar. Marketing now records its frame mapping too. No screenshot, control or visibility assertion was removed.
- Prior relay's open P1: operation rows now translate the seven server step kinds; unexpected kinds use a localized generic label. Error codes use the existing error copy or a localized Meta-refusal explanation. The original Meta response remains visible, escaped and unchanged. Exact server kind/code checks remain in browser assertions using `data-op-kind` / `data-op-code`; the same tests assert localized visible copy and absence of the raw-code text, including refusal persistence after reload in all three locales.
- This is one batch of card-owned repairs. No backend, contract, workflow, baseline, exemption or gate-adapter edit. No import was added/renamed in the component, so existing component harness import allowlists are unchanged.
- Static checks: node syntax, regenerated ownership (53 modes), strict admin tsc with read-only installed dependencies, packet structure and diff whitespace: exit 0, hashes in `relay3-static.json`. Temporary dependency links were removed. Initial missing-dependency attempts are recorded; no dependency install. Current candidate checks are E1 static only; CI on the next head, screenshot upload, complete own-PR GREEN, visual comparison/density review and independent acceptance remain PENDING/NOT_RUN. No local test/server/browser/PG or production action.
- Humaux MCP is unavailable; no store/canvas/coord action is claimed. These versioned receipts and the PR relay comment carry the handoff; integrator completion relay is the wake mechanism.

### Relay 3 copy additions

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| opKinds.campaign | 建立廣告活動 | 创建广告系列 | Create campaign |
| opKinds.adset | 建立廣告組合 | 创建广告组 | Create ad set |
| opKinds.creative | 準備廣告素材 | 准备广告素材 | Prepare creative |
| opKinds.ad | 建立廣告 | 创建广告 | Create ad |
| opKinds.preflight | 檢查廣告帳號 | 检查广告账户 | Check ad account |
| opKinds.activate | 啟用廣告 | 开启广告投放 | Activate ad |
| opKinds.pause | 暫停廣告 | 暂停广告 | Pause ad |
| opFallback | 廣告處理步驟 | 广告处理步骤 | Advertising step |
| metaRefusal | Meta 未能完成這個步驟，請先閱讀回應再重試。 | Meta 未能完成此步骤，请先查看反馈再重试。 | Meta could not complete this step. Review its response before trying again. |

## Relay 2 implementation

- PR #366, `unit/parity-growth` → `r3/integration`. Base `3ea359f796d36153bb4b8aa7259e4e0c27676b86`; RED head `e37bb6524349e47f1d5ed600cc4dcb765627e96a`. Final candidate head is recorded in the PR comment after push.
- Causal RED: Gates run **41054278594**, bundle job **124066658713**, tested synthetic merge **8c7cf2404ce307a2d6fc893a94c2ddd8da6815b6**. New real-route assertions failed on editor heading, signed list copy, attribution heading and marketing fallback heading. Foundation **41054278546 / 124057780561** passed on the RED head. These are not GREEN evidence for implementation.
- Losslessly compressed raw failed browser logs (`*.log.gz`) and representative screenshots: `relay2-failures/`; source artifacts `gate-browser-meta-ads` (12978428571), `gate-browser-ads-attribution` (12978886087), `gate-browser-promotions` (12978168572), all on the RED run. Full workflow URL: https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41054278594 . `red.log` binds the assertions and SHA.
- Current Figma authority refreshed through Fleet Bridge on 2026-10-30: all 38 roots read with get_design_context + get_screenshot; every PNG hash matches the signed reference. `relay2-figma-readback.json` records current context hashes and reference links. No Figma write or bridge restart.
- Root cause addressed: the production routes retained the pre-Figma stack. Ads now switches between list, asset authorization, editor, source selection, settings, saved/review/status, leave, stale selection recovery and concurrent-update recovery. Source/account/budget/date/age/version values come from the existing authenticated DTOs. Settings Apply changes local form state; Save still uses the original validated, versioned, idempotent API operation. A failed version update keeps local edits until the user reads the latest server copy.
- Attribution starts with selectable draft/session cards, then renders one signed task. Separate results use the existing report API; source metrics are never added together. Original complete audit tables and guarded audience reads remain reachable with native disclosures. Marketing uses the required off fallback; discount code management stays reachable through its own task button.
- Shared admin shell remains owned by the integration lane; this module inherits its navigation. No API, Go, migration, contract, lockfile or production operation is changed.

### Evidence and click coverage

Local checks: temporary read-only dependency links into the main checkout, with workspace source paths explicitly pointing to this worktree. `tsc -p output/parity-growth/tsconfig-static.json --pretty false` exit 0; `node --check` for changed browser entries and harnesses exit 0; `git diff --check` exit 0; `python3 scripts/check_packet.py` exit 0 (`PASS_PACKET_STRUCTURE_ONLY`). `relay2-static.json` binds source hashes and the limited meaning of those checks. No local tests, browser, server, PG or model experiments ran.

Browser CI produces `growth-click-ledger.jsonl`, parity PNGs, `parity/frames.jsonl` and read-only viewport `parity/density.jsonl`. Current clicks, refresh persistence, viewport density and screenshot parity remain **PENDING**, not PASS. Source-frame density probe is a Figma execution snippet and was not executed: the card forbids Figma writes/use_figma. The signed frames contain substantial explanatory copy; the reviewer must compare the actual task viewport measurements with the signed reference and the owner's density constraint. No threshold was weakened or declared passed.

| Existing test intent | Signed task mapping | Assertions retained |
|---|---|---|
| MA09 connect / OAuth refusals / token protection | List → account connection; callbacks reopen assets | State cookie, fixed errors, pick-list isolation, original error text and no secret exposure |
| MA09 create / edit / copy / Taipei persistence | Editor → source → settings → Apply → Save; Return list → reopen | Exact request body, whole-unit amount, UTC/Taipei conversion, persisted values, new copy id |
| MA09 approve / publish / pause / allowance | Saved draft → expanded operation detail; original actions | Approval revision, publish attempt, keyed request, fake Graph state, no resume, allowance refusal |
| MA09 reports and six locale/viewport cases | List → results task | Three groups, exact fixture values, timestamp/currency symbols, no combined ROAS |
| AT7/AT9 report facts and audience reads | Selection cards / advanced selectors → expanded audit facts | PG-equal facts, null vs zero, complete linked ids, privacy absence, real scroll, guarded keyed audience reads |
| AT5 unpaid checkout attribution | Selected draft → expanded report audit; reload/reopen disclosure | All collected/pending counts and amounts remain exactly zero; Meta is separate |
| Promotion quote/usage/decimal checks | Marketing off → Discount codes; revisit the same task after refresh | Original validation, used counts, checkout totals, persisted codes and per-buyer rules |

### Desktop → mobile → summary facts

| Required fact | Desktop/mobile | Summary/review | Evidence |
|---|---|---|---|
| Owned source and authorized identity/account | Same responsive source/settings components; existing saved-source candidates only | Server source + identity/account asset ids | Static E1; CI PENDING |
| Whole-schedule budget + currency | Same controlled input and formatting | Saved server lifetime budget | Static E1; CI save/reload PENDING |
| Start/end + Taipei timezone | Same date controls, Go validation retained | Saved formatted instants + Asia/Taipei label | Static E1; UTC/Taipei CI assertions retained |
| Country, age and automatic placements | Same settings fields | Country/age in saved/review summary; automatic-placement explanation in editor/settings | Static E1; CI PENDING |
| Draft id, revision, save vs approve vs publish | Original server fences retained | Summary id/revision/status; review explanation | Static E1; CI conflict/operation checks PENDING |
| Period/source, platform receipts, Meta delivery and purchases | Shared report source controls | Three independent API groups | Static E1; CI PENDING |
| Pending payment, unknown metrics and source timestamps | Server projection only | Separate pending card; full breakdown audit | Static E1; CI null/zero and timestamps PENDING |
| Customer segment fields and source catalogue eligibility | Exact data gaps below | No invented metrics or names | DEFERRED |

### Deferred fields / acceptance limits

- Frames **6:18113 / 6:18232**: no `segments[{id,name,rules,rule_version,member_count,updated_at,consent_eligibility}]` projection. Render the off fallback; never derive consent or manufacture segment counts from customer PII.
- Source picker **1049:6945 / 1049:7058**: existing saved draft `source_ref`/template/binding combinations can be selected; discovery and friendly display names need non-PII `sources[{source_ref,display_name,kind,eligible}]`. The existing manual owned-reference input remains available. Discovery portion is DEFERRED.
- Assets **5:17516 / 5:17633**: render only the settings response's actual authorized asset ids and connection dates. No grant scopes, sync time, display names or attested permissions are manufactured; exact missing fields are `asset_display_name`, `grant_scopes`, `last_synced_at` where absent.
- Marketing fallback has no ads-settings projection on the promotion/customer API. It therefore makes no connected-account, environment or budget-limit claim. Ads connection/results actions requiring those permissions/data remain in /ads; existing pricing actions remain in /promotions.
- Visual parity and every reachable MOCK state remain subject to the PR's own browser gates and independent side-by-side review. E1 code is not browser/production acceptance.

### Scope and tool trail

Within card: ads/promotion components and scoped CSS, per-namespace `packages/i18n/src/growth.ts` + its export, browser specs + registration artifact. Required out-of-path harness edits: **tests/admin/attribution.test.ts** and **tests/admin/timezone-ui.test.mjs** (new runtime-import stubs and updated signed H1 assertion); list these for integrator approval. Evidence files under output/parity-growth are also outside source write paths. No exemptions/baselines/gate-adapter assertions changed.

Humaux tools were not exposed in this relay runtime; no shared lock/task/canvas/store operation is claimed. Repository rg symbol/caller lookup was the equivalent for unavailable code-index tools. Persistent relay evidence is this delivery, red.log and the PR comment. No delegation. No local background server was started. The integrator's completion relay is the wake mechanism for queued GitHub runs. No merge/auto-merge/deploy.


## Historical RED-stage handoff (delivery status superseded; receipts retained)

Evidence: E1 syntax/static registration only. No local tests, builds, servers or browser runs.
Base: `3ea359f796d36153bb4b8aa7259e4e0c27676b86`. Branch: `unit/parity-growth`. Target: `r3/integration`.
Role: dispatched UI worker; model/effort runtime labels are not independently exposed. No delegation.

## Authority and project understanding

- Owner card dated 2026-10-30: Figma is the production UI, MOCK only; one batch RED push, then implementation only after assertion RED.
- AGENTS.md; 架构.md §§0/1/2/26/27: evidence separation, Go/PG authority, tenant/store authentication, isolated branch and independent acceptance.
- 架构.md §§14/15; contracts/meta-ads-v1.md; existing ads/attribution/promotions DTOs: consent boundaries, no client money calculations, no UNKNOWN replay; approval and external publication differ.
- UI authority: file `iJ9lDdFIXgTlKkt5Tvrc0X`, page `4:8829`, C3 inventory. All 38 roots read via Fleet Bridge get_design_context AND get_screenshot on 2026-10-30; truncated content subtrees fetched separately, no Figma writes.
- No ux-ads/ux-marketing worktree is registered. Existing signed design checkpoints were located under output/figma-ui-completion-20261025/ads-{draft,list,reports}-20261026/DELIVERY.md. They supplied reference pointers only. CURRENT frame receipts determine copy and state.
- Read development rule 7bdb220b-0798-4b1d-aff5-16ee94cae75f and visual rule f6273516-2c72-4f2e-b4b3-54d3625809bb. Current card's CI-only/no-merge rules override generic defaults. Rejected representative coverage 5/9; lessons coverage 5/33, not an exhaustive claim.

## Current-frame findings / deferred data

- Marketing-named 4:11612/4:11750 currently contain the ads draft list; 1194:7740/7796 contain the same list with the feature-off notice. Historical names do not justify rebuilding a different UI.
- 5:17304/17438 currently contain the recovery heading 「從列表開啟正確的草稿」. No historical daily-budget/performance detail is restored.
- DEFERRED 6:18113/18232 customer segments: promotions DTO has no segment id/name/rules/version/member_count/updated_at or consent-filtered eligibility. The feature-off fallback is tested; no six demo segment counts are fabricated.
- Source inventory gap: ads settings contain authorized identities, not an inventory of owned published posts/products with display names and eligibility. Parity tests select a source already saved by the real MA09 fixture. New source discovery beyond existing server data remains DEFERRED pending exact `sources[{source_ref,display_name,kind,eligible}]` fields. No API/contract/migration is changed in this RED stage.
- Asset grant/sync metadata is limited to existing settings; unavailable authorization dates/permissions must stay absent rather than copying the example's dates.

## BOURDIEU (design interpretation, not user research)

| Verdict | Application |
|---|---|
| Q1 | New to this advertising task: explicit source → budget → review → save/approval sequence. |
| Q2 | Experienced with this task: preserve server values, draft revision and a direct return path. |
| Q3 | Knowledge burden: explain internal approval separately from Meta publication and delivery. |
| Q4 | Recovery cost: preserve local edits on conflict; unknown external results permit reads only. |
| Q5 | No class/device inference: desktop and mobile retain the same material facts. |
| Q6 | Aesthetic preference UNKNOWN until owner; implement the signed frame without inventing a new style. |

## MOBBIN → EAGLE

Read-only Eagle item/info on 2026-10-30 confirms both references present and nondeleted in folder `MVQL9YXQLTAAR`.
Google Ads `f1889379-f1cd-41a6-9bc9-c3ca9d27c8f4` → `MVOJWMITZO1XA`: staged creation/review hierarchy.
AirOps `a0c8e522-9ff7-4787-a203-648575e28504` → `MVOT8B5BK3QQ3`: date controls before independent metrics and titled groups.
Reference structure only; no copied images, branding, numbers or product claims. Receipts: eagle-*-reference.json. No missing reference needs importing for this signed-frame implementation.

## FIGMA OFFICIAL GUIDANCE

Read the existing UX audit's FIGMA-GUIDE.md (ux-audit/output/ux-audit; the card's ux-audit-ref alias does not exist).
G1 Auto layout → responsive task panels/action dock; G2 Variables → existing semantic tokens; G3 Components/variants → shared controls/state rendering; G4/G5 Prototyping → explicit next/return and conflict/unknown paths; G6 → frame-to-route/viewport handoff.
Primary guidance links recorded there: https://help.figma.com/hc/en-us/articles/360040451373-Guide-to-auto-layout-in-Figma ; https://help.figma.com/hc/en-us/articles/14506821864087-Overview-of-variables-collections-and-modes ; https://help.figma.com/hc/en-us/articles/360038662654-Guide-to-components-in-Figma ; https://help.figma.com/hc/en-us/articles/360040314193-Guide-to-prototyping-in-Figma . No new Figma design is delivered, so design copy lint is NOT_RUN.

## Desktop → mobile → summary/review fact checklist

| Required fact | Desktop / mobile target | Summary/review target | Current evidence |
|---|---|---|---|
| Server source and authorized identity | editor/source pair | saved + review pair | NOT_RUN; existing MA09 source only |
| Lifetime budget, not daily budget | settings pair | saved/review/status | NOT_RUN; 1,500 persistence assertion |
| Start/end, Taipei timezone, schedule validity | settings pair | saved/review/status | NOT_RUN; real fill and API save |
| Country and age range | settings pair | review pair | NOT_RUN; target fact labels |
| Draft id/revision and save vs approve vs publish | editor/saved/review | conflict/status | NOT_RUN; two-tab concurrent update |
| Selected period/source and three independent metric groups | index/results/detail | Meta breakdown | NOT_RUN; real report fixture |
| Pending payment excluded; missing metrics remain unknown | detail/breakdown pair | summary groups | NOT_RUN; existing AT assertions preserved |
| Segment counts/consent and rule versions | 6:18113/18232 | no invented summary | DEFERRED; required DTO absent |

## Click ledger and acceptance

Registered via existing tests/admin/ads.spec.ts, attribution.spec.ts and tests/storefront/promotions-gate.mjs. Parent gate assertions preserved verbatim; no existing test deleted, skipped or weakened.
The new journeys write growth-click-ledger.jsonl after visible assertions and parity/frames.jsonl after real-route screenshots. Marketing verifies refresh persistence in all locales; ads save and conflict verify server persistence after reload. Two-tab edits exercise real version conflict, not a synthetic 409. A delayed REAL GET is fixture preparation for the loading frame; no write is intercepted.
Every clickable flow remains NOT_RUN until the PR's own CI executes it. Existing MA09/AT/promotions intent must be mapped to the new task layout during implementation, never deleted.

## Legacy removal targets

- /ads: old H1 「廣告」 / Ads / 广告 and stacked connection + drafts + CAPI + report panels. New task screens have one purpose; connection/report remain reachable.
- /ads/attribution: old H1 「廣告歸因與直播復盤」 / Ad attribution & session review and simultaneously open draft/session panels; replace with selection → one detail → breakdown.
- /promotions: old page H1 「優惠碼」 / 优惠码 / Discount codes; existing discount-code creation/edit/toggle intent remains required in a reachable task, with its original gate retained.

## Gate and rollback

Static checks: node --check on both new helpers and all changed parent entry points; git diff --check; derive-browser-ownership.mjs then --check (53 modes). Exit 0. tsc NOT_RUN (worktree dependencies absent); no install or local server.
CI RED/GREEN, real browser clicks, persistence, screenshot parity, density and independent review: NOT_RUN/PENDING.
No process, DB, port or container was started. Read-only Fleet Bridge calls completed. Rollback is git revert of this tests-only commit on this branch; untracked evidence is retained, not removed by git.
Outside C3 application write paths: tests/admin/{ads,attribution}.spec.ts, tests/storefront/promotions-gate.mjs (spec registration); generated output/ci-select-backend-browser/browser-ownership.json; output/parity-growth/** evidence. The initial import-only parity-growth.spec.ts and parity-growth-marketing.spec.mjs fragments were consolidated into these registered entry points in relay 1. No application implementation changed.

## Relay 1 — repair test reachability before assertion RED

- Original head `bddc4380602d63ddf0e06487f18e5ac2e3ec69b2`: Foundation run `41052985242`, job `124053376997`, failed at `Package docs and gate map are current`. Filtered original failed-step receipt: `relay1-foundation-failed.log`.
- The gate reported both imported parity fragments as `is run by no gate`. No new visible-copy/control assertion was reached; this is **not** causal RED.
- Targeted registration repair attempt **1/2**: move all added ads and attribution cases into their existing registered spec entry points and the marketing helper into its existing promotions runner. Assertion bodies, fixture data, invocation conditions and preexisting gate assertions stay unchanged. No registry/checker/exemption change.
- Fetched and merged `origin/r3/integration` before deleting the fragments: already up to date at `3ea359f796d36153bb4b8aa7259e4e0c27676b86`. Regenerated browser ownership from the staged inventory, then `--check`: exit 0. `node --check` on the three entries and `git diff --check`: exit 0. These are E1 static evidence only.
- The replacement push remains tests-only. Implementation, assertion RED, GREEN, actual clicks, persistence and parity screenshots remain pending CI. Integrator relays a new session after the exact-head Gates/Foundation run completes; no local test/server or production action is authorized.

## Frame parity sheet (all screenshots pending CI)

| Frame | Real route / state | Browser artifact | Signed reference PNG | Status |
|---|---|---|---|---|
| 5:17304 | /ads · Admin · 广告详情 · SHOPLINE | parity/5-17304.png | [figma/5-17304/get_screenshot-5-17304-1793361325193-0.png](figma/5-17304/get_screenshot-5-17304-1793361325193-0.png) | NOT_RUN |
| 5:17438 | /ads · Admin Mobile · 广告详情 · SHOPLINE | parity/5-17438.png | [figma/5-17438/get_screenshot-5-17438-1793361326127-0.png](figma/5-17438/get_screenshot-5-17438-1793361326127-0.png) | NOT_RUN |
| 5:17516 | /ads · Admin · 资产授权 · SHOPLINE | parity/5-17516.png | [figma/5-17516/get_screenshot-5-17516-1793361328107-0.png](figma/5-17516/get_screenshot-5-17516-1793361328107-0.png) | NOT_RUN |
| 5:17633 | /ads · Admin Mobile · 资产授权 · SHOPLINE | parity/5-17633.png | [figma/5-17633/get_screenshot-5-17633-1793361329143-0.png](figma/5-17633/get_screenshot-5-17633-1793361329143-0.png) | NOT_RUN |
| 6:20130 | /ads · Admin · 广告草稿 · SHOPLINE | parity/6-20130.png | [figma/6-20130/get_screenshot-6-20130-1793361330220-0.png](figma/6-20130/get_screenshot-6-20130-1793361330220-0.png) | NOT_RUN |
| 6:20255 | /ads · Admin Mobile · 广告草稿 · SHOPLINE | parity/6-20255.png | [figma/6-20255/get_screenshot-6-20255-1793361331165-0.png](figma/6-20255/get_screenshot-6-20255-1793361331165-0.png) | NOT_RUN |
| 6:23064 | /ads/attribution · Admin Desktop · 歸因報表 · DaWan Live | parity/6-23064.png | [figma/6-23064/get_screenshot-6-23064-1793361332168-0.png](figma/6-23064/get_screenshot-6-23064-1793361332168-0.png) | NOT_RUN |
| 6:23199 | /ads/attribution · Admin Mobile · 歸因報表 · DaWan Live | parity/6-23199.png | [figma/6-23199/get_screenshot-6-23199-1793361333035-0.png](figma/6-23199/get_screenshot-6-23199-1793361333035-0.png) | NOT_RUN |
| 1049:6945 | /ads · Admin Desktop · 廣告草稿 · source | parity/1049-6945.png | [figma/1049-6945/get_screenshot-1049-6945-1793361334045-0.png](figma/1049-6945/get_screenshot-1049-6945-1793361334045-0.png) | NOT_RUN |
| 1049:7058 | /ads · Admin Mobile · 廣告草稿 · source | parity/1049-7058.png | [figma/1049-7058/get_screenshot-1049-7058-1793361334851-0.png](figma/1049-7058/get_screenshot-1049-7058-1793361334851-0.png) | NOT_RUN |
| 1049:7120 | /ads · Admin Desktop · 廣告草稿 · settings | parity/1049-7120.png | [figma/1049-7120/get_screenshot-1049-7120-1793361336145-0.png](figma/1049-7120/get_screenshot-1049-7120-1793361336145-0.png) | NOT_RUN |
| 1049:7233 | /ads · Admin Mobile · 廣告草稿 · settings | parity/1049-7233.png | [figma/1049-7233/get_screenshot-1049-7233-1793361337052-0.png](figma/1049-7233/get_screenshot-1049-7233-1793361337052-0.png) | NOT_RUN |
| 1049:7295 | /ads · Admin Desktop · 廣告草稿 · review | parity/1049-7295.png | [figma/1049-7295/get_screenshot-1049-7295-1793361338469-0.png](figma/1049-7295/get_screenshot-1049-7295-1793361338469-0.png) | NOT_RUN |
| 1049:7408 | /ads · Admin Mobile · 廣告草稿 · review | parity/1049-7408.png | [figma/1049-7408/get_screenshot-1049-7408-1793361339565-0.png](figma/1049-7408/get_screenshot-1049-7408-1793361339565-0.png) | NOT_RUN |
| 1049:7470 | /ads · Admin Desktop · 廣告草稿 · saved | parity/1049-7470.png | [figma/1049-7470/get_screenshot-1049-7470-1793361341105-0.png](figma/1049-7470/get_screenshot-1049-7470-1793361341105-0.png) | NOT_RUN |
| 1049:7583 | /ads · Admin Mobile · 廣告草稿 · saved | parity/1049-7583.png | [figma/1049-7583/get_screenshot-1049-7583-1793361342801-0.png](figma/1049-7583/get_screenshot-1049-7583-1793361342801-0.png) | NOT_RUN |
| 1049:7645 | /ads · Admin Desktop · 廣告草稿 · status | parity/1049-7645.png | [figma/1049-7645/get_screenshot-1049-7645-1793361344573-0.png](figma/1049-7645/get_screenshot-1049-7645-1793361344573-0.png) | NOT_RUN |
| 1049:7758 | /ads · Admin Mobile · 廣告草稿 · status | parity/1049-7758.png | [figma/1049-7758/get_screenshot-1049-7758-1793361345566-0.png](figma/1049-7758/get_screenshot-1049-7758-1793361345566-0.png) | NOT_RUN |
| 1049:7820 | /ads · Admin Desktop · 廣告草稿 · leave | parity/1049-7820.png | [figma/1049-7820/get_screenshot-1049-7820-1793361346963-0.png](figma/1049-7820/get_screenshot-1049-7820-1793361346963-0.png) | NOT_RUN |
| 1049:7933 | /ads · Admin Mobile · 廣告草稿 · leave | parity/1049-7933.png | [figma/1049-7933/get_screenshot-1049-7933-1793361348136-0.png](figma/1049-7933/get_screenshot-1049-7933-1793361348136-0.png) | NOT_RUN |
| 1049:7995 | /ads · Admin Desktop · 廣告草稿 · conflict | parity/1049-7995.png | [figma/1049-7995/get_screenshot-1049-7995-1793361349622-0.png](figma/1049-7995/get_screenshot-1049-7995-1793361349622-0.png) | NOT_RUN |
| 1049:8108 | /ads · Admin Mobile · 廣告草稿 · conflict | parity/1049-8108.png | [figma/1049-8108/get_screenshot-1049-8108-1793361350258-0.png](figma/1049-8108/get_screenshot-1049-8108-1793361350258-0.png) | NOT_RUN |
| 1134:9548 | /ads · Admin Desktop · 廣告列表 · 狀態說明 | parity/1134-9548.png | [figma/1134-9548/get_screenshot-1134-9548-1793361351393-0.png](figma/1134-9548/get_screenshot-1134-9548-1793361351393-0.png) | NOT_RUN |
| 1134:9549 | /ads · Admin Mobile · 廣告列表 · 狀態說明 | parity/1134-9549.png | [figma/1134-9549/get_screenshot-1134-9549-1793361352015-0.png](figma/1134-9549/get_screenshot-1134-9549-1793361352015-0.png) | NOT_RUN |
| 1134:9550 | /ads · Admin Desktop · 廣告列表 · 讀取回饋 | parity/1134-9550.png | [figma/1134-9550/get_screenshot-1134-9550-1793361353077-0.png](figma/1134-9550/get_screenshot-1134-9550-1793361353077-0.png) | NOT_RUN |
| 1134:9551 | /ads · Admin Mobile · 廣告列表 · 讀取回饋 | parity/1134-9551.png | [figma/1134-9551/get_screenshot-1134-9551-1793361353601-0.png](figma/1134-9551/get_screenshot-1134-9551-1793361353601-0.png) | NOT_RUN |
| 1151:9420 | /ads/attribution · Admin Desktop · 成效總覽 · DaWan Live | parity/1151-9420.png | [figma/1151-9420/get_screenshot-1151-9420-1793361354607-0.png](figma/1151-9420/get_screenshot-1151-9420-1793361354607-0.png) | NOT_RUN |
| 1151:9421 | /ads/attribution · Admin Mobile · 成效總覽 · DaWan Live | parity/1151-9421.png | [figma/1151-9421/get_screenshot-1151-9421-1793361355231-0.png](figma/1151-9421/get_screenshot-1151-9421-1793361355231-0.png) | NOT_RUN |
| 1151:9422 | /ads/attribution · Admin Desktop · 廣告歸因明細 · DaWan Live | parity/1151-9422.png | [figma/1151-9422/get_screenshot-1151-9422-1793361356209-0.png](figma/1151-9422/get_screenshot-1151-9422-1793361356209-0.png) | NOT_RUN |
| 1151:9423 | /ads/attribution · Admin Mobile · 廣告歸因明細 · DaWan Live | parity/1151-9423.png | [figma/1151-9423/get_screenshot-1151-9423-1793361357321-0.png](figma/1151-9423/get_screenshot-1151-9423-1793361357321-0.png) | NOT_RUN |
| 1151:9428 | /ads/attribution · Admin Desktop · Meta 分組明細 · DaWan Live | parity/1151-9428.png | [figma/1151-9428/get_screenshot-1151-9428-1793361358350-0.png](figma/1151-9428/get_screenshot-1151-9428-1793361358350-0.png) | NOT_RUN |
| 1151:9429 | /ads/attribution · Admin Mobile · Meta 分組明細 · DaWan Live | parity/1151-9429.png | [figma/1151-9429/get_screenshot-1151-9429-1793361358989-0.png](figma/1151-9429/get_screenshot-1151-9429-1793361358989-0.png) | NOT_RUN |
| 4:11612 | /ads · Admin · 营销 · SHOPLINE | parity/4-11612.png | [figma/4-11612/get_screenshot-4-11612-1793361359948-0.png](figma/4-11612/get_screenshot-4-11612-1793361359948-0.png) | NOT_RUN |
| 4:11750 | /ads · Admin Mobile · 营销 · SHOPLINE | parity/4-11750.png | [figma/4-11750/get_screenshot-4-11750-1793361360696-0.png](figma/4-11750/get_screenshot-4-11750-1793361360696-0.png) | NOT_RUN |
| 6:18113 | /promotions · Admin · 客户分群 · SHOPLINE | parity/6-18113.png | [figma/6-18113/get_screenshot-6-18113-1793361361726-0.png](figma/6-18113/get_screenshot-6-18113-1793361361726-0.png) | DEFERRED: segment DTO absent |
| 6:18232 | /promotions · Admin Mobile · 客户分群 · SHOPLINE | parity/6-18232.png | [figma/6-18232/get_screenshot-6-18232-1793361362834-0.png](figma/6-18232/get_screenshot-6-18232-1793361362834-0.png) | DEFERRED: segment DTO absent |
| 1194:7740 | /promotions · Admin · 营销 · SHOPLINE · 功能關閉 | parity/1194-7740.png | [figma/1194-7740/get_screenshot-1194-7740-1793361364024-0.png](figma/1194-7740/get_screenshot-1194-7740-1793361364024-0.png) | NOT_RUN |
| 1194:7796 | /promotions · Admin Mobile · 营销 · SHOPLINE · 功能關閉 | parity/1194-7796.png | [figma/1194-7796/get_screenshot-1194-7796-1793361364571-0.png](figma/1194-7796/get_screenshot-1194-7796-1793361364571-0.png) | NOT_RUN |

## Copy table (RED targets; existing dictionaries retained)

| zh-TW | zh-CN | en |
|---|---|---|
| 廣告草稿 | 广告草稿 | Ad drafts |
| 新增草稿 | 新增草稿 | New draft |
| 建立廣告草稿 | 创建广告草稿 | Create ad draft |
| 選擇貼文或商品 | 选择帖文或商品 | Choose post or product |
| 修改預算、排期與對象 | 修改预算、排期和受众 | Edit budget, schedule and audience |
| 儲存草稿 | 保存草稿 | Save draft |
| 核准前確認 | 审批前确认 | Review before approval |
| 草稿已儲存 | 草稿已保存 | Draft saved |
| 要保留這些未儲存的修改嗎？ | 要保留这些未保存的修改吗？ | Keep these unsaved changes? |
| 另一位管理員已更新草稿 | 另一位管理员已更新草稿 | Another administrator updated this draft |
| 狀態說明 | 状态说明 | Status guide |
| 資產授權 | 资产授权 | Asset authorization |
| 歸因報表 | 归因报表 | Attribution report |
| 成效總覽 | 成效总览 | Results overview |
| 廣告歸因 | 广告归因 | Ad attribution |
| Meta 分組明細 | Meta 分组明细 | Meta breakdown |
| 重新讀取 | 重新读取 | Refresh |
| 平台啟用前，此商店的廣告功能為關閉狀態。 | 平台启用前，此店铺的广告功能为关闭状态。 | Ads are off for this store until the platform enables them. |

Full visible-string table and per-control executed ledger remain required before implementation acceptance. This RED handoff does not claim UI delivery.

## Current three-language copy table (growth namespace)

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| quickAmounts | 快速金額 | 快捷金额 | Quick amounts |
| taiwan | 台灣 TW | 台湾 TW | Taiwan TW |
| hongKong | 香港 HK | 香港 HK | Hong Kong HK |
| agesDefault | 18–65 歲 | 18–65 岁 | 18–65 years |
| agesFocused | 25–44 歲 | 25–44 岁 | 25–44 years |
| threeDays | 3 天 | 3 天 | 3 days |
| sevenDays | 7 天 | 7 天 | 7 days |
| stateTitles.DRAFT | 草稿 | 草稿 | Draft |
| stateTitles.APPROVED | 內部已核准 | 内部已审核 | Internally approved |
| stateTitles.SUBMITTING | 正在 Meta 建立 | 正在 Meta 创建 | Being created on Meta |
| stateTitles.REMOTE_PAUSED | 已建立 · 尚未啟用 | 已创建 · 尚未启用 | Created · not activated |
| stateTitles.ACTIVE | 刊登中 | 投放中 | Delivering |
| stateTitles.PAUSED | 已確認暫停 | 已确认暂停 | Confirmed paused |
| stateTitles.UNKNOWN | 外部結果待確認 | 外部结果待确认 | External result awaiting confirmation |
| stateTitles.FAILED | 建立失敗 | 创建失败 | Creation failed |
| stateTitles.ENDED | 已結束 | 已结束 | Ended |
| stateTitles.REJECTED | 遭 Meta 拒絕 | 被 Meta 拒绝 | Rejected by Meta |
| stateDescriptions.DRAFT | 尚未內部核准，沒有發布。可編輯並儲存，再核對版本。 | 尚未通过内部审核，也未发布。编辑并保存后，请重新核对版本。 | Not internally approved or published. Edit and save, then check the version. |
| stateDescriptions.APPROVED | 只表示負責人核准這個版本。尚未等同 Meta 審核通過或刊登。 | 仅代表负责人审核通过当前版本，不代表 Meta 审核通过或正在投放。 | The owner approved this version. This does not mean Meta approved it or delivery began. |
| stateDescriptions.SUBMITTING | 發布工作正在處理。收到工作回執也不能判定已投放。 | 发布任务正在处理，收到任务回执并不能证明已经投放。 | Publication is being processed. A work receipt does not prove delivery. |
| stateDescriptions.REMOTE_PAUSED | Meta 物件已建立，狀態為暫停。不能顯示為刊登中。 | Meta 广告已创建，当前暂停，尚未开始投放。 | Meta objects were created paused. They are not delivering yet. |
| stateDescriptions.ACTIVE | 啟用操作已確認成功，且無後續成功暫停。不保證實際曝光或花費，仍須核對外部狀態。 | 已确认启用，之后没有成功暂停记录。实际曝光和花费仍需核对外部状态。 | Activation was confirmed with no later successful pause. Check Meta for actual delivery and spend. |
| stateDescriptions.PAUSED | 暫停操作已確認成功；以最新外部資料核對實際狀態。不提供直接恢復的捷徑。 | 暂停已确认，请以最新外部数据核对实际状态。不提供直接恢复投放的入口。 | Pause was confirmed. Check the latest external state; there is no direct resume shortcut. |
| stateDescriptions.UNKNOWN | 目前不能確認結果，也不能認定沒有花費。重新讀取；必要時到 Ads Manager 核對。不要重建廣告。 | 尚不能确认结果或花费。请重新读取，必要时到 Ads Manager 核对，不要重新创建广告。 | The result and spend are unconfirmed. Read again or check Ads Manager; do not recreate the ad. |
| stateDescriptions.FAILED | 這次建立未成功，閱讀本次 Meta 原因。能否重送由伺服器核對，不直接重複發布。 | 本次创建失败，请阅读 Meta 返回的原因。能否重试由服务器核对，不直接重复发布。 | Read the Meta reason for this attempt. The server decides whether it can be retried. |
| stateDescriptions.ENDED | 本地已標記結束；外部暫停可能仍待確認。結束不保證花費立刻歸零。 | 本地已标记结束，外部暂停可能仍待确认，花费不一定立即停止。 | Locally marked ended; an external pause may still be awaiting confirmation. Spend may not stop immediately. |
| stateDescriptions.REJECTED | 依最近一次外部狀態顯示。閱讀 Meta 原文；需要重新設定時另建草稿並核准。 | 按最近的外部状态展示。请阅读 Meta 原文，重新配置时需新建草稿并重新审核。 | Read the original Meta message. A newly configured draft requires fresh approval. |
| drafts | 廣告草稿 | 广告草稿 | Ad drafts |
| newDraft | 新增草稿 | 新增草稿 | New draft |
| listIntro | 先核對版本，再核准與發布。內部核准不等於 Meta 審核通過。 | 先核对版本，再审核与发布。内部审核通过不代表 Meta 审核通过。 | Check the version before approval and publication. Internal approval is separate from Meta review. |
| create | 建立廣告草稿 | 创建广告草稿 | Create ad draft |
| edit | 編輯廣告草稿 | 编辑广告草稿 | Edit ad draft |
| sequence | 填寫草稿 → 儲存 → 負責人核准 → 發布 | 填写草稿 → 保存 → 负责人审核 → 发布 | Prepare draft → save → owner approval → publish |
| sourceStep | 01 · 選擇推廣內容 | 01 · 选择推广内容 | 01 · Choose content |
| budgetStep | 02 · 總預算與排期 | 02 · 总预算与投放时间 | 02 · Lifetime budget and schedule |
| audienceStep | 03 · 投放對象與身份 | 03 · 投放人群与身份 | 03 · Audience and identity |
| source | 選擇貼文或商品 | 选择帖子或商品 | Choose a post or product |
| sourceTitle | 選擇這次要推廣的內容 | 选择本次推广内容 | Choose the content to promote |
| keepSource | 保留原選擇並返回 | 保留原选择并返回 | Keep selection and return |
| useSource | 使用這個來源 | 使用此来源 | Use this source |
| noSource | 尚未選擇來源 | 尚未选择来源 | No source selected |
| savedSources | 已儲存過的來源 | 已保存过的来源 | Previously saved sources |
| sourceGap | 尚無可用來源目錄。請填寫自有貼文或已上架商品的編號。 | 暂无可用来源目录。请填写自有帖子或已上架商品的编号。 | No eligible source catalogue is available. Enter an owned post or published product reference. |
| settings | 修改預算、排期與對象 | 修改预算、投放时间与人群 | Edit budget, schedule and audience |
| apply | 套用到草稿 | 应用到草稿 | Apply to draft |
| automatic | 自動版位 | 自动版位 | Automatic placements |
| saved | 草稿已儲存 | 草稿已保存 | Draft saved |
| savedIntro | 還沒有向 Meta 建立或發布廣告 | 尚未在 Meta 创建或发布广告 | No ad has been created or published on Meta yet |
| nextApproval | 下一步由負責人核准 | 下一步由负责人审核 | Next, the owner approves this version |
| accountEnvironment | 廣告帳號、環境與預算上限 | 广告账户、环境与预算上限 | Ad account, environment and budget limit |
| review | 核准前確認 | 审核前确认 | Review before approval |
| reviewAction | 核對並核准 | 核对并审核 | Review and approve |
| content | 推廣內容 | 推广内容 | Promoted content |
| budget | 整段總預算與排期 | 整个投放周期的预算与时间 | Lifetime budget and schedule |
| audience | 投放對象 | 投放人群 | Audience |
| separate | 核准與發布分開 | 审核与发布分开 | Approval and publication are separate |
| separation | 儲存：只保留有效草稿。核准：負責人確認這個版本。發布：向 Meta 建立廣告，之後可能花費。 | 保存仅保留有效草稿。审核由负责人确认当前版本。发布会在 Meta 创建广告，之后可能产生花费。 | Saving keeps a draft. Approval confirms this version. Publication creates an ad on Meta and may incur spend. |
| budgetWarning | 金額不會自行提高 | 预算不会自动提高 | The amount will not increase automatically |
| budgetHint | 整段排期的總預算，不是每日額度。Meta 回報花費有延遲，實際花費可能略高於設定。 | 预算覆盖整个投放周期，并非每日限额。Meta 花费数据有延迟，实际花费可能略高于设置。 | The budget covers the whole schedule, not each day. Meta reporting is delayed and delivery may exceed the setting. |
| status | 廣告狀態 | 广告状态 | Ad status |
| statusAction | 查看廣告狀態 | 查看广告状态 | View ad status |
| statusUnread | 尚未讀取廣告狀態 | 尚未读取广告状态 | Ad status has not been read |
| statusHint | 先核對已儲存版本及確認後的外部狀態，再採取動作。 | 先核对已保存版本和已确认的外部状态，再进行操作。 | Read the saved version and confirmed external status before taking action. |
| leave | 離開 | 离开 | Leave |
| leaveTitle | 要保留這些未儲存的修改嗎？ | 保留这些未保存的修改吗？ | Keep these unsaved changes? |
| keepEditing | 繼續編輯 | 继续编辑 | Continue editing |
| discard | 放棄修改並離開 | 放弃修改并离开 | Discard changes and leave |
| conflict | 另一位管理員已更新草稿 | 另一位管理员已更新草稿 | Another administrator updated the draft |
| conflictHint | 這次修改尚未覆蓋伺服器內容。 | 本次修改尚未覆盖服务器内容。 | Your changes have not overwritten the server version. |
| keepChanges | 返回保留修改 | 返回并保留修改 | Return and keep changes |
| latest | 讀取最新草稿 | 读取最新草稿 | Read latest draft |
| list | 返回廣告列表 | 返回广告列表 | Back to ad list |
| goList | 前往廣告列表 | 前往广告列表 | Go to ad list |
| recovery | 從列表開啟正確的草稿 | 从列表打开正确的草稿 | Open the correct draft from the list |
| recoveryHint | 無法讀取這份已儲存草稿，請從列表選擇可查看的草稿。 | 无法读取这份已保存草稿，请从列表选择可查看的草稿。 | This saved selection could not be read. Choose an accessible draft from the list. |
| legend | 狀態說明 | 状态说明 | Status guide |
| legendTitle | 狀態各自代表什麼 | 各状态代表什么 | What each status means |
| approved | 內部已核准 | 内部已审核 | Internally approved |
| remotePaused | 已建立 · 尚未啟用 | 已创建 · 尚未启用 | Created · not activated |
| unknown | 外部結果待確認 | 外部结果待确认 | External result awaiting confirmation |
| paused | 已確認暫停 | 已确认暂停 | Confirmed paused |
| assets | 查看廣告帳號連結 | 查看广告账户连接 | View ad account connection |
| assetsTitle | 資產授權 | 资产授权 | Asset authorization |
| authorized | 已授權資產 | 已授权资产 | Authorized assets |
| reauthorize | 重新授權 | 重新授权 | Authorize again |
| refresh | 重新讀取 | 重新读取 | Refresh |
| loading | 正在讀取廣告列表 | 正在读取广告列表 | Reading ad list |
| notSaved | 尚未儲存 | 尚未保存 | Not saved |
| noDrafts | 還沒有已儲存的廣告草稿 | 暂无已保存的广告草稿 | No saved ad drafts yet |
| emptyHint | 選擇自有貼文或商品，設定總預算和排期，再儲存草稿。儲存本身不會產生廣告花費。 | 选择自有帖子或商品，设置总预算和投放时间，然后保存草稿。保存本身不会产生广告花费。 | Choose owned content, set the lifetime budget and schedule, then save. Saving alone does not incur ad spend. |
| resultsHint | 費用與成效請到報表查看 | 费用与效果请到报表查看 | See spend and results in reports |
| resultsNote | 總預算不是已花費金額。實際收款、Meta 花費與廣告帶來的購買是分開的資料，不相加。 | 总预算并非已花费金额。实际收款、Meta 花费和广告带来的购买分别展示，不相加。 | Budget is not spend. Platform receipts, Meta spend and Meta-reported purchases are separate figures. |
| attribution | 歸因報表 | 归因报表 | Attribution report |
| attributionIntro | 先選一份廣告草稿或一場直播，再核對它帶來的訂單。 | 先选择广告草稿或直播，再核对带来的订单。 | Choose an ad draft or live session, then review the orders it brought. |
| results | 成效總覽 | 成效总览 | Results overview |
| viewAttribution | 查看廣告／直播歸因 | 查看广告／直播归因 | View ad / live attribution |
| adAttribution | 廣告歸因 | 广告归因 | Ad attribution |
| backAttribution | 返回歸因列表 | 返回归因列表 | Back to attribution list |
| breakdown | Meta 分組明細 | Meta 分组明细 | Meta breakdown |
| breakdownHint | Meta 彙總，不是買家個人資料。 | Meta 汇总数据，并非买家个人资料。 | Meta aggregates, not personal buyer data. |
| backDetail | 返回廣告歸因 | 返回广告归因 | Back to ad attribution |
| pending | 待收款 · 不算入已收款 | 待收款 · 不计入已收款 | Awaiting payment · excluded from paid receipts |
| orders | 本平台 · 信用卡收付款 | 本平台 · 信用卡收付款 | Platform · card receipts and refunds |
| delivery | Meta · 刊登成效 | Meta · 投放表现 | Meta · delivery performance |
| reported | Meta · 回報購買 | Meta · 回报购买 | Meta · reported purchases |
| off | 平台啟用前，此商店的廣告功能為關閉狀態。 | 平台启用前，此店铺的广告功能为关闭状态。 | Ads are off for this store until the platform enables them. |
| codes | 優惠碼 | 优惠码 | Discount codes |
| cancel | 取消 | 取消 | Cancel |
| advanced | 進階 | 高级 | Advanced |

Existing operational/audit copy remains in the three-locale ads-copy.ts, attribution-copy.ts and promotions-copy.ts dictionaries; no error message or status code is rewritten. Dynamic server values are not illustrative copy.

## Existing ads operational and audit copy (retained catalogue)

Dynamic formatting expressions show their exact source; input values are supplied by the server/form. Source: apps/admin/lib/ads-copy.ts.

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| errors.operations_in_flight | Meta 廣告操作仍在進行中，請等待結果確認後再解除綁定。 | Meta 广告操作仍在进行中，请等待结果确认后再解除绑定。 | Meta ads operations are still in progress. Wait for confirmed results before disconnecting. |
| errors.binding_in_use | 仍有可能投放中的廣告，請先暫停並等待確認，再解除綁定。 | 仍有可能投放中的广告，请先暂停并等待确认，再解除绑定。 | Ads may still be running. Pause them first, wait for confirmation, then disconnect. |
| errors.state_mismatch | 此次連結不屬於目前的登入或商店，請重新開始。 | 此次连接不属于当前的登录或店铺，请重新开始。 | This connection attempt does not belong to this session or store. Start again. |
| errors.state_expired | 此次連結已過期，請重新開始。 | 此次连接已过期，请重新开始。 | The connection attempt expired. Start again. |
| errors.meta_connect_failed | Meta 無法完成連結，請稍後再試。 | Meta 无法完成连接，请稍后再试。 | Meta could not complete the connection. Try again in a moment. |
| errors.not_in_pick_list | Meta 沒有為此次登入提供該廣告帳號或資料集，請重新開始連結。 | Meta 没有为此次登录提供该广告账号或数据集，请重新开始连接。 | That ad account or dataset was not offered by Meta for this login. Start the connection again. |
| errors.client_business_changed | 此廣告帳號目前屬於與最初連結不同的 Meta 商家，請聯絡支援。 | 此广告账号目前属于与最初连接不同的 Meta 商家，请联系支持。 | This ad account now belongs to a different Meta business than the one first connected. Contact support. |
| errors.revision_changed | 草稿在您讀取後已變更，已重新載入；請檢查後再試。 | 草稿在您读取后已变更，已重新加载；请检查后再试。 | The draft changed since you loaded it. It was reloaded; review it and try again. |
| errors.over_allowance | 這會超過此商店的廣告預算上限。 | 这会超过此店铺的广告预算上限。 | This would exceed the ad budget allowance set for this store. |
| errors.billing_restricted | 您的帳戶有逾期款項，暫時無法核准或發布新廣告；暫停功能仍可使用。 | 您的账户有逾期款项，暂时无法审批或发布新广告；暂停功能仍可使用。 | Your account has an overdue balance, so new ads cannot be approved or published. Pausing still works. |
| errors.attempt_changed | 發布次數已變更，草稿已重新載入；請檢查後再試。 | 发布次数已变更，草稿已重新加载；请检查后再试。 | The publish attempt changed. The draft was reloaded; review it and try again. |
| errors.prior_attempt_not_paused | 先前的發布可能仍會花費，請先暫停並等待確認。 | 之前的发布可能仍会花费，请先暂停并等待确认。 | An earlier attempt may still be able to spend. Pause it and wait for confirmation first. |
| errors.draft_approved | 已核准的草稿無法編輯，請複製為新草稿。 | 已审批的草稿无法编辑，请复制为新草稿。 | An approved draft cannot be edited. Copy it into a new draft instead. |
| errors.budget_below_minimum | 預算低於 Meta 對此期間的最低要求。 | 预算低于 Meta 对此期间的最低要求。 | The budget is below what Meta requires for this schedule. |
| errors.not_whole_unit | 此幣別的預算須為整數單位（例如 NT$300，不能是 NT$300.50）。 | 此币种的预算须为整数单位（例如 NT$300，不能是 NT$300.50）。 | This currency needs a whole-unit budget (for example NT$300, not NT$300.50). |
| errors.currency_mismatch | 草稿幣別須與商店廣告幣別及廣告帳號幣別一致。 | 草稿币种须与店铺广告币种及广告账号币种一致。 | The draft currency must match the store's ad currency and the ad account's currency. |
| errors.starts_too_soon | 開始時間須在 10 分鐘之後。 | 开始时间须在 10 分钟之后。 | The start time must be at least 10 minutes from now. |
| errors.binding_disabled | 此草稿使用的連結已停用，請先重新連結。 | 此草稿使用的连接已停用，请先重新连接。 | The connection used by this draft is disabled. Reconnect it first. |
| errors.source_not_owned | 該貼文不屬於所選的粉絲專頁或 Instagram 帳號。 | 该帖文不属于所选的主页或 Instagram 账号。 | That post is not on the selected Page or Instagram account. |
| errors.product_not_published | 該商品未上架，或商店沒有啟用中的店面網域。 | 该商品未上架，或店铺没有启用中的店面域名。 | That product is not published, or the store has no active storefront domain. |
| errors.forbidden | 此帳號沒有此操作的權限。 | 此账号没有此操作的权限。 | This account does not have permission for this action. |
| errors.not_found | 所選商店中無法讀取此項目。 | 所选店铺中无法读取此项目。 | This item is unavailable in the selected store. |
| errors.invalid_request | 請求無效，請檢查欄位後重試。 | 请求无效，请检查字段后重试。 | The request was not valid. Check the fields and try again. |
| errors.unauthorized | 請重新登入後繼續。 | 请重新登录后继续。 | Sign in again to continue. |
| errors.retry_later | 暫時無法使用，請重試。 | 暂时无法使用，请重试。 | Temporarily unavailable. Try again. |
| errors.rate_limited | 請求過多，請幾秒後再試。 | 请求过多，请几秒后再试。 | Too many requests. Try again in a few seconds. |
| errors.invalid_json | 請求無法讀取，請重新載入頁面後再試。 | 请求无法读取，请重新加载页面后再试。 | The request could not be read. Reload the page and try again. |
| errors.json_required | 請求無法讀取，請重新載入頁面後再試。 | 请求无法读取，请重新加载页面后再试。 | The request could not be read. Reload the page and try again. |
| errors.method_not_allowed | 此操作不可用。 | 此操作不可用。 | This action is not available. |
| errors.conflict | 此請求與先前的請求衝突，請重新整理後再試。 | 此请求与之前的请求冲突，请刷新后再试。 | This conflicts with an earlier request. Refresh and try again. |
| errors.budget_invalid | 預算請輸入純數字。 | 预算请输入纯数字。 | Enter the budget as a plain number. |
| errors.unsupported_currency | 廣告僅支援 TWD、USD 與 HKD 的預算。 | 广告仅支持 TWD、USD 与 HKD 的预算。 | Ads support TWD, USD and HKD budgets only. |
| errors.ends_before_start | 結束時間須晚於開始時間。 | 结束时间须晚于开始时间。 | The end must be after the start. |
| errors.duration_too_long | 廣告最長可刊登 30 天。 | 广告最长可投放 30 天。 | An ad can run at most 30 days. |
| errors.age_invalid | 年齡須介於 18 到 65，且最低不得高於最高。 | 年龄须介于 18 到 65，且最低不得高于最高。 | Ages must be between 18 and 65, minimum not above maximum. |
| errors.countries_invalid | 請輸入一個以上不重複的兩位字母國家代碼，以逗號分隔。 | 请输入一个以上不重复的两位字母国家代码，以逗号分隔。 | Use one or more different two-letter country codes, separated by commas. |
| errors.source_invalid | 貼文、貼文編號或商品編號格式看起來不正確。 | 帖文、帖文编号或商品编号格式看起来不正确。 | The post, media or product id does not look valid. |
| errors.bindings_missing | 請選擇廣告帳號與粉絲專頁或 Instagram 帳號。 | 请选择广告账号与主页或 Instagram 账号。 | Choose an ad account and a Page or Instagram account. |
| errors.dates_invalid | 請選擇開始與結束時間。 | 请选择开始与结束时间。 | Choose a start and an end time. |
| statuses.DRAFT | 草稿 | 草稿 | Draft |
| statuses.APPROVED | 已核准 | 已审批 | Approved |
| statuses.SUBMITTING | 正在 Meta 建立 | 正在 Meta 创建 | Being created on Meta |
| statuses.REMOTE_PAUSED | 已建立·暫停中 | 已创建·暂停中 | Created, paused |
| statuses.ACTIVE | 刊登中 | 投放中 | Active |
| statuses.PAUSED | 已暫停 | 已暂停 | Paused |
| statuses.UNKNOWN | 請查看 Ads Manager | 请查看 Ads Manager | Check Ads Manager |
| statuses.FAILED | 失敗 | 失败 | Failed |
| statuses.ENDED | 已結束 | 已结束 | Ended |
| statuses.REJECTED | 遭 Meta 拒絕 | 被 Meta 拒绝 | Rejected by Meta |
| opStates.READY | 排隊中 | 排队中 | Queued |
| opStates.DISPATCHING | 送出中 | 发送中 | Sending |
| opStates.UNKNOWN | 結果未知 | 结果未知 | Result unknown |
| opStates.ACKNOWLEDGED | 已接受 | 已接受 | Accepted |
| opStates.SUCCEEDED | 完成 | 完成 | Done |
| opStates.FAILED_FINAL | 失敗 | 失败 | Failed |
| opStates.CANCELLED | 已取消 | 已取消 | Cancelled |
| opStates.BLOCKED_POLICY | 被規則攔下 | 被规则拦下 | Blocked by a rule |
| opStates.STALE_BINDING | 連結已變更 | 连接已变更 | Connection changed |
| providers.facebook | Facebook 粉絲專頁 | Facebook 主页 | Facebook Page |
| providers.instagram | Instagram | Instagram | Instagram |
| providers.meta_ads | Meta 廣告帳號 | Meta 广告账号 | Meta ad account |
| providers.meta_dataset | Meta 資料集 | Meta 数据集 | Meta dataset |
| templates.BOOST_POST | 加強貼文 | 加强帖文 | Boost a post |
| templates.PRODUCT_TRAFFIC | 商品導流 | 商品导流 | Product traffic |
| connectErrors.state_mismatch | 此次連結不屬於目前的登入或商店，請重新開始。 | 此次连接不属于当前的登录或店铺，请重新开始。 | This connection attempt does not belong to this session or store. Start again. |
| connectErrors.state_expired | 此次連結已過期，請重新開始。 | 此次连接已过期，请重新开始。 | The connection attempt expired. Start again. |
| connectErrors.meta_connect_failed | Meta 無法完成連結，請稍後重新開始。 | Meta 无法完成连接，请稍后重新开始。 | Meta could not complete the connection. Start again in a moment. |
| connectErrors.denied | 您已在 Meta 取消連結。 | 您已在 Meta 取消连接。 | The connection was cancelled on Meta. |
| connectErrors.invalid_request | 從 Meta 返回的資料無效，請重新開始。 | 从 Meta 返回的数据无效，请重新开始。 | The return from Meta was not valid. Start again. |
| connectErrors.forbidden | 此帳號無法為這間商店連結廣告帳號。 | 此账号无法为这家店铺连接广告账号。 | This account cannot connect ad accounts for this store. |
| connectErrors.unavailable | 目前無法完成連結，請重新開始。 | 目前无法完成连接，请重新开始。 | The connection could not be completed right now. Start again. |
| title | 廣告 | 广告 | Ads |
| subtitle | 連結 Meta 廣告帳號，建立加強貼文或商品導流廣告，核准後發布並查看成效。 | 连接 Meta 广告账号，创建加强帖文或商品导流广告，审批后发布并查看效果。 | Connect a Meta ad account, prepare a boosted post or product-traffic campaign, approve it, and read the results. |
| store | 商店 | 店铺 | Store |
| refresh | 重新整理 | 刷新 | Refresh |
| loading | 讀取廣告中… | 正在读取广告… | Loading ads… |
| signedOut | 請登入後管理廣告。 | 请登录后管理广告。 | Sign in to manage ads. |
| forbidden | 此帳號沒有管理廣告的權限。 | 此账号没有管理广告的权限。 | This account does not have permission to manage ads. |
| unavailable | 廣告功能暫時無法使用，請重試。 | 广告功能暂时无法使用，请重试。 | Ads are temporarily unavailable. Try again. |
| noStore | 沒有可用的授權商店。 | 没有可用的授权店铺。 | No authorized store is available. |
| retry | 重試 | 重试 | Try again |
| cancel | 取消 | 取消 | Cancel |
| close | 關閉 | 关闭 | Close |
| sending | 處理中… | 处理中… | Working… |
| sandboxBanner | 沙盒模式，不會投放。在此建立的內容不會觸及任何人，也不會花費任何費用。 | 沙盒模式，不会投放。在此创建的内容不会触达任何人，也不会产生任何费用。 | Sandbox — no delivery. Nothing created here reaches real people and nothing is spent. |
| allowanceOff | 平台啟用前，此商店的廣告功能為關閉狀態。 | 平台启用前，此店铺的广告功能处于关闭状态。 | Ads are off for this store until the platform enables them. |
| budgetNote | Meta 預算不是即時的硬性上限：實際花費可能略為超出，數據也會延遲更新。要停止廣告請按「暫停」。 | Meta 预算不是实时的硬性上限：实际花费可能略有超出，数据也会延迟更新。要停止广告请点“暂停”。 | A Meta budget is not a real-time hard stop: delivery can slightly overshoot and figures refresh with a delay. Pause an ad to stop it. |
| unbindButton | 解除綁定廣告帳號 | 解除绑定广告账号 | Disconnect ad account |
| unbindTitle | 解除綁定此廣告帳號？ | 解除绑定此广告账号？ | Disconnect this ad account? |
| unbindHistory | 廣告草稿、核准、成效及稽核歷史都會保留。 | 广告草稿、批准、成效及审计历史都会保留。 | Ad drafts, approvals, insights and audit history are kept. |
| unbindPause | 請先暫停所有仍可能投放中的廣告，再解除綁定。 | 请先暂停所有仍可能投放中的广告，再解除绑定。 | Pause all ads that may still be running before disconnecting. |
| unbindLocal | 僅移除本地加密廣告憑據，不會撤銷 Meta 端授權；如有需要，請到 Facebook 設定撤銷。 | 仅移除本地加密广告凭据，不会撤销 Meta 端授权；如有需要，请到 Facebook 设置撤销。 | Only local sealed ad credentials are removed. This does not revoke your Meta grant; revoke it in Facebook settings if needed. |
| unbindCapi | CAPI 及其資料集連結不受影響，購買事件另由下方設定控制。 | CAPI 及其数据集连接不受影响，购买事件另由下方设置控制。 | CAPI and its dataset connection stay connected. Purchase events are controlled separately below. |
| unbindConfirm | 確認解除綁定 | 确认解除绑定 | Disconnect account |
| unbound | 廣告帳號已解除綁定，歷史已保留。 | 广告账号已解除绑定，历史已保留。 | Ad account disconnected. History is kept. |
| feedTitle | 商品目錄 feed 網址 | 商品目录 feed 网址 | Catalog feed URL |
| feedHowTo | 在 Meta Commerce Manager 開啟商品目錄 → 資料來源 → 新增商品 → 資料 feed，選擇排程 feed 並貼上此公開網址。內容僅包含已發布的商品目錄資料。 | 在 Meta Commerce Manager 打开商品目录 → 数据来源 → 添加商品 → 数据 feed，选择定时 feed 并粘贴此公开网址。内容仅包含已发布的商品目录数据。 | In Meta Commerce Manager, open your catalog → Data sources → Add items → Data feed. Choose a scheduled feed and paste this public URL. It contains published catalog data only. |
| feedEmpty | 請先發布獨立站並啟用獨立站網域，才能取得商品目錄 feed 網址。 | 请先发布独立站并启用独立站域名，才能取得商品目录 feed 网址。 | Publish your storefront and activate a storefront domain first to get a catalog feed URL. |
| feedCopy | 複製 feed 網址 | 复制 feed 网址 | Copy feed URL |
| feedCopied | 已複製 feed 網址。 | 已复制 feed 网址。 | Feed URL copied. |
| feedCopyFailed | 無法複製，請選取網址後手動複製。 | 无法复制，请选中网址后手动复制。 | Could not copy. Select the URL and copy it manually. |
| connTitle | Meta 連結 | Meta 连接 | Meta connection |
| connEmpty | 尚未連結 Meta 廣告帳號。 | 尚未连接 Meta 广告账号。 | No Meta ad account is connected yet. |
| connectButton | 連結 Meta 廣告帳號 | 连接 Meta 广告账号 | Connect a Meta ad account |
| connecting | 正在前往 Meta… | 正在前往 Meta… | Opening Meta… |
| connectHint | 登入在 Meta 自己的頁面完成，本網站不會取得您的 Meta 密碼或存取權杖。 | 登录在 Meta 自己的页面完成，本网站不会获取您的 Meta 密码或访问令牌。 | You sign in on Meta's own page. This site never sees your Meta password or access token. |
| connAccount | 廣告帳號 | 广告账号 | Ad account |
| connDataset | 資料集 | 数据集 | Dataset |
| connBusiness | 商家 | 商家 | Business |
| connAt | 連結時間 | 连接时间 | Connected |
| connEnabled | 已啟用 | 已启用 | Enabled |
| connDisabled | 已停用 | 已停用 | Disabled |
| identitiesTitle | 可用於加強貼文的粉絲專頁與 Instagram 帳號 | 可用于加强帖文的主页与 Instagram 账号 | Pages and Instagram accounts available for boosting |
| identitiesEmpty | 此商店尚未連結任何 Facebook 粉絲專頁或 Instagram 帳號。 | 此店铺尚未连接任何 Facebook 主页或 Instagram 账号。 | No Facebook Page or Instagram account is connected for this store. |
| pickTitle | 選擇要使用的廣告帳號 | 选择要使用的广告账号 | Choose the ad account to use |
| pickAccounts | 廣告帳號 | 广告账号 | Ad accounts |
| pickDatasets | 購買事件資料集（選填） | 购买事件数据集（选填） | Dataset for purchase events (optional) |
| pickNoDataset | 不使用資料集 | 不使用数据集 | No dataset |
| pickNoAccounts | Meta 沒有為此登入回傳任何廣告帳號。 | Meta 没有为此登录返回任何广告账号。 | Meta returned no ad accounts for this login. |
| pickSubmit | 連結此帳號 | 连接此账号 | Connect this account |
| pickExpired | 此次連結已過期或已使用過，請重新開始。 | 此次连接已过期或已使用过，请重新开始。 | This connection attempt expired or was already used. Start again. |
| startAgain | 重新開始 | 重新开始 | Start again |
| accountReady | 正常 | 正常 | Active |
| connected | 已連結，廣告帳號現在可用於草稿。 | 已连接，广告账号现在可用于草稿。 | Connected. The ad account is now available to drafts. |
| draftsTitle | 廣告草稿 | 广告草稿 | Ad drafts |
| draftsEmpty | 尚無草稿。 | 暂无草稿。 | No drafts yet. |
| newDraft | 新增草稿 | 新建草稿 | New draft |
| colTemplate | 類型 | 类型 | Template |
| colBudget | 總預算 | 总预算 | Lifetime budget |
| colSchedule | 期間 | 时间 | Schedule |
| colStatus | 狀態 | 状态 | Status |
| colCreated | 建立時間 | 创建时间 | Created |
| select | 開啟 | 打开 | Open |
| formNew | 新增廣告草稿 | 新建广告草稿 | New ad draft |
| formEdit | 編輯草稿 | 编辑草稿 | Edit draft |
| formCopy | 由已暫停的廣告建立新草稿 | 由已暂停的广告创建新草稿 | New draft from a paused ad |
| fTemplate | 類型 | 类型 | Template |
| fAdAccount | 廣告帳號 | 广告账号 | Ad account |
| fIdentity | Facebook 粉絲專頁或 Instagram 帳號 | Facebook 主页或 Instagram 账号 | Facebook Page or Instagram account |
| fIdentityFacebookOnly | Facebook 粉絲專頁（商品導流必填） | Facebook 主页（商品导流必填） | Facebook Page (required for product traffic) |
| fSource | 貼文編號、Instagram 貼文編號或商品編號 | 帖文编号、Instagram 帖文编号或商品编号 | Post id, Instagram media id or product id |
| fSourceBoost | 上方已連結帳號的 Facebook 貼文編號（頁面編號_貼文編號）或 Instagram 貼文編號。 | 上方已连接账号的 Facebook 帖文编号（主页编号_帖文编号）或 Instagram 帖文编号。 | A Facebook post id (page_post) or an Instagram media id, from an account connected above. |
| fSourceProduct | 此商店已上架商品的商品編號。 | 此店铺已上架商品的商品编号。 | The product id of a published product in this store. |
| fStarts | 開始時間 | 开始时间 | Starts |
| fEnds | 結束時間 | 结束时间 | Ends |
| fDatesHint | 以 Asia/Taipei（UTC+8）計。開始時間須在 10 分鐘之後，最長刊登 30 天。 | 按 Asia/Taipei（UTC+8）计算。开始时间须在 10 分钟之后，最长投放 30 天。 | Asia/Taipei (UTC+8). Start at least 10 minutes from now; run at most 30 days. |
| fCountries | 國家／地區 | 国家／地区 | Countries |
| fCountriesHint | 兩位字母代碼，以逗號分隔；預設 TW。 | 两位字母代码，以逗号分隔；默认 TW。 | Two-letter codes separated by commas; default TW. |
| fAgeMin | 最低年齡 | 最低年龄 | Minimum age |
| fAgeMax | 最高年齡 | 最高年龄 | Maximum age |
| fPlacement | 廣告版位由 Meta 自動選擇。 | 广告版位由 Meta 自动选择。 | Placements are chosen automatically by Meta. |
| fNoAccount | 請先連結廣告帳號。 | 请先连接广告账号。 | Connect an ad account first. |
| fNoIdentity | 沒有符合此類型的已連結粉絲專頁或 Instagram 帳號。 | 没有符合此类型的已连接主页或 Instagram 账号。 | No connected Page or Instagram account fits this template. |
| save | 儲存草稿 | 保存草稿 | Save draft |
| saving | 儲存中… | 保存中… | Saving… |
| saved | 草稿已儲存，列表顯示的是伺服器上的內容。 | 草稿已保存，列表显示的是服务器上的内容。 | Draft saved. The list shows the server's copy. |
| fixFields | 請先修正以下項目： | 请先修正以下项目： | Fix these before saving: |
| detailTitle | 草稿 | 草稿 | Draft |
| budget | 總預算 | 总预算 | Lifetime budget |
| starts | 開始 | 开始 | Starts |
| ends | 結束 | 结束 | Ends |
| countries | 國家／地區 | 国家／地区 | Countries |
| ages | 年齡 | 年龄 | Ages |
| source | 來源 | 来源 | Source |
| revision | 版本 | 版本 | Revision |
| attempt | 發布次數 | 发布次数 | Publish attempt |
| remote | Meta 上的物件 | Meta 上的对象 | Meta objects |
| remoteCampaign | 廣告活動 | 广告系列 | Campaign |
| remoteAdset | 廣告組合 | 广告组 | Ad set |
| remoteCreative | 廣告素材 | 广告素材 | Creative |
| remoteAd | 廣告 | 广告 | Ad |
| remoteNone | 尚未在 Meta 建立任何物件。 | 尚未在 Meta 创建任何对象。 | Nothing created on Meta yet. |
| opsTitle | 送往 Meta 的步驟 | 发往 Meta 的步骤 | Steps sent to Meta |
| metaResponse | Meta 回覆 | Meta 回复 | Meta responses |
| opsEmpty | 尚無步驟。 | 暂无步骤。 | No steps yet. |
| opKind | 步驟 | 步骤 | Step |
| opAttempt | 次數 | 次数 | Attempt |
| opState | 狀態 | 状态 | State |
| opUpdated | 更新時間 | 更新时间 | Updated |
| adsManager | 在 Ads Manager 開啟 | 在 Ads Manager 打开 | Open in Ads Manager |
| checkAdsManager | 某個步驟的結果未知。請到 Ads Manager 確認此廣告的實際狀態，有疑慮時請在此暫停。 | 某个步骤的结果未知。请到 Ads Manager 确认此广告的实际状态，有疑虑时请在此暂停。 | The result of a step is unknown. Check Ads Manager for the real state of this ad, and pause it here if in doubt. |
| edit | 編輯 | 编辑 | Edit |
| approve | 核准 | 审批 | Approve |
| publish | 發布 | 发布 | Publish |
| republish | 重新發布 | 重新发布 | Publish again |
| pause | 暫停 | 暂停 | Pause |
| end | 結束 | 结束 | End |
| copy | 複製為新草稿 | 复制为新草稿 | Copy to new draft |
| endConfirmTitle | 要永久結束這則廣告嗎？ | 要永久结束这条广告吗？ | End this ad permanently? |
| endConfirmBody | 結束後的廣告會在 Meta 上暫停，且無法再啟動。若要再次刊登，請複製為新草稿。 | 结束后的广告会在 Meta 上暂停，且无法再启动。若要再次投放，请复制为新草稿。 | An ended ad is paused on Meta and cannot be started again. To run it again, copy it into a new draft. |
| endConfirm | 結束廣告 | 结束广告 | End ad |
| copyHint | 已暫停的廣告不會再啟動。新草稿需要重新核准才能刊登。 | 已暂停的广告不会再启动。新草稿需要重新审批才能投放。 | A paused ad never starts again. A new draft needs its own approval before it can run. |
| approveOffHint | 此商店的廣告功能為關閉狀態，無法核准。 | 此店铺的广告功能处于关闭状态，无法审批。 | Ads are off for this store, so approval is unavailable. |
| approved | 已核准此版本。 | 已审批此版本。 | Approved for this revision. |
| published | 已開始發布，下方步驟顯示伺服器的進度。 | 已开始发布，下方步骤显示服务器的进度。 | Publishing started. Steps below show the server's progress. |
| paused | 已要求暫停，下方步驟顯示伺服器的進度。 | 已请求暂停，下方步骤显示服务器的进度。 | Pause requested. Steps below show the server's progress. |
| ended | 廣告已結束。 | 广告已结束。 | The ad was ended. |
| uncertain | 結果尚未確認。重試會安全地送出相同請求，或重新整理查看伺服器狀態。 | 结果尚未确认。重试会安全地发送相同请求，或刷新查看服务器状态。 | The result is not confirmed. Retrying safely sends the same request; or refresh to see the server state. |
| retrySame | 重試相同請求 | 重试相同请求 | Retry the same request |
| reportTitle | 成效 | 效果 | Results |
| reportFrom | 起 | 起 | From |
| reportTo | 迄 | 止 | To |
| reportLoad | 顯示成效 | 显示效果 | Show results |
| reportLoading | 讀取成效中… | 正在读取效果… | Loading results… |
| reportWindow | 區間 | 区间 | Window |
| reportTimezone | 時區 | 时区 | Time zone |
| reportFetched | 擷取時間 | 获取时间 | Fetched |
| reportNotFetched | 尚未擷取 | 尚未获取 | Not fetched yet |
| reportInvalid | 請依序選擇日期，最多相隔 92 天。 | 请按先后顺序选择日期，最多相隔 92 天。 | Choose dates in order, at most 92 days apart. |
| reportRefreshNote | Meta 數據會延遲更新，且數天內仍可能變動；這不是即時的花費硬性上限。 | Meta 数据会延迟更新，且几天内仍可能变动；这不是实时的花费硬性上限。 | Meta figures refresh with a delay and can still change for a few days. They are not a real-time hard stop for spend. |
| reportSeparate | 以下是來自三個來源的三項獨立指標，不會相加，也不提供合併的報酬率數字。 | 以下是来自三个来源的三项独立指标，不会相加，也不提供合并的回报率数字。 | These are three separate measures from three sources. They are not added together and no combined return figure is shown. |
| blockOrders | 訂單（扣除退款後） | 订单（扣除退款后） | Orders, net of refunds |
| blockOrdersSub | 此區間內您自己的已付款訂單減去退款。 | 此区间内您自己的已付款订单减去退款。 | Your own paid orders in this window minus refunds. |
| blockDelivery | Meta 投放 | Meta 投放 | Meta delivery |
| blockDeliverySub | Meta 表示已投放與收費的內容。 | Meta 表示已投放与收费的内容。 | What Meta says it delivered and charged. |
| blockReported | Meta 回報的購買 | Meta 报告的购买 | Purchases reported by Meta |
| blockReportedSub | Meta 自己的歸因，可能與您的訂單不同。 | Meta 自己的归因，可能与您的订单不同。 | Meta's own attribution; it can differ from your orders. |
| notAvailable | 無法取得 | 无法获取 | Not available |
| captured | 已付款 | 已付款 | Paid |
| refunded | 已退款 | 已退款 | Refunded |
| net | 淨額 | 净额 | Net |
| spend | 花費 | 花费 | Spend |
| impressions | 曝光 | 曝光 | Impressions |
| clicks | 點擊 | 点击 | Clicks |
| purchases | 購買數 | 购买数 | Purchases |
| purchaseValue | 購買金額 | 购买金额 | Purchase value |
| finalThrough | 數據定案至 | 数据定稿至 | Final through |
| cardOnly | 僅計信用卡付款；超商取貨付款的訂單不計入。 | 仅统计银行卡付款；超商取货付款的订单不计入。 | Card payments only; pay-at-pickup orders are not counted. |
| capiTitle | 購買事件（Conversions API） | 购买事件（Conversions API） | Purchase events (Conversions API) |
| capiIntro | 將已付款訂單的購買事件傳送到 Meta 資料集，讓 Meta 衡量並最佳化廣告。 | 将已付款订单的购买事件发送到 Meta 数据集，让 Meta 衡量并优化广告。 | Send purchase events for paid orders to a Meta dataset so Meta can measure and optimize ads. |
| capiConsent | 只會傳送已記錄同意的買家的事件。 | 只会发送已记录同意的买家的事件。 | Events are sent only for buyers whose consent is recorded. |
| capiEnable | 傳送購買事件 | 发送购买事件 | Send purchase events |
| capiDataset | 資料集 | 数据集 | Dataset |
| capiNoDataset | 不使用資料集 | 不使用数据集 | No dataset |
| capiTest | 測試事件代碼（選填） | 测试事件代码（选填） | Test event code (optional) |
| capiTestHint | 僅供 Meta 測試工具使用；正式事件請留空。 | 仅供 Meta 测试工具使用；正式事件请留空。 | Only for Meta's test tool; leave empty for real events. |
| capiSave | 儲存 | 保存 | Save |
| capiSaved | 已儲存。 | 已保存。 | Saved. |
| capiInvalid | 開啟時請選擇資料集；測試代碼只能包含字母、數字、- 與 _。 | 开启时请选择数据集；测试代码只能包含字母、数字、- 与 _。 | Choose a dataset when events are on; the test code may use letters, digits, - and _. |
| capiNoneConnected | 請先連結廣告帳號與資料集才能開啟。 | 请先连接广告账号与数据集才能开启。 | Connect a dataset with an ad account to turn this on. |
| allowanceLine | (amount) => 此商店已核准廣告的總預算上限為 ${amount}。 | (amount) => 此店铺已审批广告的总预算上限为 ${amount}。 | (amount: string) => Approved lifetime budgets across this store's ads are capped at ${amount}. |
| inFlightTotal | (total: number, shown: number) => 顯示仍在進行的 ${total} 筆操作中的 ${shown} 筆。 | (total: number, shown: number) => 显示仍在进行的 ${total} 项操作中的 ${shown} 项。 | (total: number, shown: number) => Showing ${shown} of ${total} operations still in progress. |
| accountStatus | (n) => (n === 1 ? "正常" : n === 2 ? "已停用" : n === 3 ? "款項未結清" : n === 7 ? "風險審查中" : n === 9 ? "寬限期" : n === 100 \|\| n === 101 ? "關閉中或已關閉" : 狀態 ${n}) | (n) => (n === 1 ? "正常" : n === 2 ? "已停用" : n === 3 ? "款项未结清" : n === 7 ? "风险审核中" : n === 9 ? "宽限期" : n === 100 \|\| n === 101 ? "关闭中或已关闭" : 状态 ${n}) | (n: number): string => (n === 1 ? "Active" : n === 2 ? "Disabled" : n === 3 ? "Payment unsettled" : n === 7 ? "Risk review" : n === 9 ? "Grace period" : n === 100 \|\| n === 101 ? "Closing or closed" : Status ${n}) |
| fBudget | (currency) => 總預算（${currency}，整數單位） | (currency) => 总预算（${currency}，整数单位） | (currency: string) => Lifetime budget (${currency}, whole units) |
| fBudgetHint | (currency) => (currency === "TWD" ? "僅限整數新台幣，例如 300。" : "最多兩位小數，例如 12.50。") | (currency) => (currency === "TWD" ? "仅限整数新台币，例如 300。" : "最多两位小数，例如 12.50。") | (currency: string): string => (currency === "TWD" ? "Whole NT$ only; for example 300." : "Up to two decimals; for example 12.50.") |

## Existing attribution operational and audit copy (retained catalogue)

Dynamic formatting expressions show their exact source; input values are supplied by the server/form. Source: apps/admin/lib/attribution-copy.ts.

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| title | 廣告歸因與直播復盤 | 广告归因与直播复盘 | Ad attribution & session review |
| back | 返回廣告 | 返回广告 | Back to ads |
| intro | 訂單實績與 Meta 歸因採不同口徑，數字可能不同，兩者不相加。 | 订单实绩与 Meta 归因采用不同口径，数字可能不同，两者不相加。 | Order results and Meta attribution are different views; they can differ and are never added together. |
| from | 開始日期 | 开始日期 | From |
| to | 結束日期 | 结束日期 | To |
| apply | 讀取報表 | 读取报表 | Read report |
| invalid | 請選擇有效日期範圍，最多 92 天。 | 请选择有效日期范围，最多 92 天。 | Choose a valid date range of at most 92 days. |
| loading | 讀取歸因資料… | 读取归因数据… | Reading attribution… |
| retry | 重試 | 重试 | Try again |
| unavailable | 暫時無法取得歸因資料，請重試。 | 暂时无法取得归因数据，请重试。 | Attribution is unavailable. Try again. |
| forbidden | 你沒有讀取廣告的權限。 | 你没有读取广告的权限。 | You do not have permission to read ads. |
| signedOut | 工作階段已結束。 | 会话已结束。 | Your session ended. |
| signIn | 登入 | 登录 | Sign in |
| empty | 此期間沒有歸因紀錄。 | 此期间没有归因记录。 | No attribution records in this window. |
| selectDraft | 廣告草稿 | 广告草稿 | Ad draft |
| selectSession | 直播場次 | 直播场次 | Live session |
| noDraft | 沒有草稿紀錄 | 没有草稿记录 | No draft records |
| noSession | 沒有場次紀錄 | 没有场次记录 | No session records |
| drafts | 草稿報表 | 草稿报表 | Draft report |
| sessions | 直播復盤 | 直播复盘 | Session review |
| ordersTitle | 訂單實績 | 订单实绩 | Order results |
| metaTitle | Meta 回報 | Meta 回报 | Meta reported |
| orderZone | 訂單日期：Asia/Taipei | 订单日期：Asia/Taipei | Order dates: Asia/Taipei |
| metaZone | Meta 帳戶時區 | Meta 账户时区 | Meta account time zone |
| unknown | 未提供 | 未提供 | Not provided |
| provisional | Meta 數據可能仍會更新 | Meta 数据可能仍会更新 | Meta data may still change |
| spend | 廣告花費 | 广告花费 | Ad spend |
| sessionSpend | 所選期間內推廣此直播貼文的廣告花費 | 所选期间内推广此直播帖文的广告花费 | Ad spend promoting this live post within the selected period |
| truncated | 僅顯示前 100 筆 | 仅显示前 100 条 | Only the first 100 records are shown |
| breakdownsUnavailable | 無法取得分類資料 | 无法取得分类数据 | Breakdowns unavailable |
| reconnect | 重新連接 Facebook | 重新连接 Facebook | Reconnect Facebook |
| path | 來源路徑 | 来源路径 | Source path |
| adClick | 廣告點擊 | 广告点击 | Ad click |
| boosted | 受推廣貼文帶來 | 受推广帖文带来 | From promoted post |
| boostedNote | 受推廣貼文的訂單可能來自付費或自然留言，無法確認廣告曝光。 | 受推广帖文的订单可能来自付费或自然评论，无法确认广告曝光。 | Promoted-post orders may come from paid or organic comments; ad exposure is unknown. |
| orders | 已收款訂單 | 已收款订单 | Collected orders |
| net | 扣除退款後淨營收 | 扣除退款后净营收 | Net revenue after refunds |
| pending | 待收款訂單 | 待收款订单 | Uncollected orders |
| pendingValue | 待收款金額 | 待收款金额 | Uncollected amount |
| roas | 訂單 ROAS | 订单 ROAS | Order ROAS |
| purchases | 購買次數 | 购买次数 | Purchases |
| purchaseValue | 購買金額 | 购买金额 | Purchase value |
| audience | 廣告受眾（Meta 統計） | 广告受众（Meta 统计） | Ad audience (Meta statistics) |
| day | Meta 帳戶日期 | Meta 账户日期 | Meta account day |
| dimension | 分類 | 分类 | Breakdown |
| bucket | 群組 | 群组 | Bucket |
| reach | 觸及 | 触达 | Reach |
| impressions | 曝光 | 曝光 | Impressions |
| clicks | 點擊 | 点击 | Clicks |
| engagements | 互動 | 互动 | Engagements |
| comments | 留言 | 评论 | Comments |
| ageGender | 年齡與性別 | 年龄与性别 | Age & gender |
| region | 地區 | 地区 | Region |
| placement | 版位 | 版位 | Placement |
| device | 裝置 | 设备 | Device |
| hourly | 每小時 | 每小时 | Hourly |
| noAudience | Meta 尚未提供受眾分類。 | Meta 尚未提供受众分类。 | Meta has not provided audience breakdowns. |
| buyers | 買家分佈（訂單實績） | 买家分布（订单实绩） | Buyer distribution (order results) |
| county | 收件縣市 | 收件县市 | Ship-to county / city |
| newBuyers | 新買家 | 新买家 | New buyers |
| returning | 回購買家 | 回购买家 | Returning buyers |
| average | 平均訂單金額 | 平均订单金额 | Average order value |
| products | 熱銷商品 | 热销商品 | Top products |
| product | 商品 | 商品 | Product |
| quantity | 數量 | 数量 | Quantity |
| noBuyers | 此期間沒有買家分類資料。 | 此期间没有买家分类数据。 | No buyer breakdowns in this window. |
| minuteOrders | 每分鐘訂單 | 每分钟订单 | Orders per minute |
| at | 時間（Asia/Taipei） | 时间（Asia/Taipei） | Time (Asia/Taipei) |
| timeline | 同一場次時間軸 | 同一场次时间轴 | One session timeline |
| timelineNote | 每小時廣告花費以絕對時間對齊；Meta 觀眾統計不與個別買家連結。 | 每小时广告花费以绝对时间对齐；Meta 观众统计不与个人买家关联。 | Hourly ad spend is aligned using absolute timestamps. No per-person join is made between Meta audiences and buyers. |
| claims | 喊單 | 喊单 | Claims |
| checkoutLinks | 結帳連結 | 结账链接 | Checkout links |
| paid | 已付款／收款訂單 | 已付款／收款订单 | Paid / collected orders |
| funnel | 留言 → 喊單 → 結帳連結 → 已收款訂單 | 评论 → 喊单 → 结账链接 → 已收款订单 | Comments → claims → checkout links → collected orders |
| viewers | 觀眾 | 观众 | Viewers |
| liveAudience | 觀眾輪廓（Meta 統計） | 观众轮廓（Meta 统计） | Viewer profile (Meta statistics) |
| audienceRefresh | 刷新觀眾洞察 | 刷新观众洞察 | Refresh audience insights |
| audienceReading | 正在排入觀眾讀取… | 正在排入观众读取… | Queuing audience read… |
| audienceQueued | 觀眾讀取已排入佇列，請稍後重新讀取報表查看更新。 | 观众读取已排入队列，请稍后重新读取报表查看更新。 | Audience read queued. Read this report again later to see any updated insights. |
| audienceCompleted | 沿用已完成的觀眾讀取，未新增讀取請求。請重新讀取報表查看已儲存的洞察。 | 沿用已完成的观众读取，未新增读取请求。请重新读取报表查看已保存的洞察。 | Reusing a completed audience read; no new read was queued. Read the report again to view the saved insights. |
| audienceInflight | 既有觀眾讀取正在處理，請稍後重新讀取報表。 | 现有观众读取正在处理，请稍后重新读取报表。 | The existing audience read is in progress. Read the report again later. |
| audienceUnconfirmed | 既有觀眾讀取結果未知，可再次要求讀取；10 分鐘冷卻期結束前，伺服器會沿用這次讀取。 | 现有观众读取结果未知，可再次请求读取；10 分钟冷却期结束前，服务器会沿用这次读取。 | The existing audience read outcome is unknown. You can request again; the server reuses this read until its 10-minute cooldown ends. |
| audienceReadFailed | 觀眾讀取失敗，請確認 Facebook 連接後再提出請求。 | 观众读取失败，请确认 Facebook 连接后再提出请求。 | The audience read failed. Check the Facebook connection before requesting again. |
| audienceBlocked | 觀眾讀取因政策被阻擋，請確認 Facebook 授權。 | 观众读取因政策被阻止，请确认 Facebook 授权。 | The audience read was blocked by policy. Check the Facebook authorization. |
| audienceStale | Facebook 連接已變更，請重新連接後再讀取觀眾洞察。 | Facebook 连接已变更，请重新连接后再读取观众洞察。 | The Facebook connection changed. Reconnect it before requesting audience insights. |
| audienceCancelled | 觀眾讀取已取消。 | 观众读取已取消。 | The audience read was cancelled. |
| audienceUnknown | 讀取請求結果未知，請重試同一請求。 | 读取请求结果未知，请重试同一请求。 | The read request outcome is unknown. Retry the same request. |
| audienceRetry | 重試同一讀取請求 | 重试同一读取请求 | Retry same read request |
| audienceForbidden | 觀眾洞察不可用：缺少 read_insights／pages_read_engagement 權限，或授權已失效。 | 观众洞察不可用：缺少 read_insights／pages_read_engagement 权限，或授权已失效。 | Audience insights are unavailable: read_insights / pages_read_engagement permissions are missing or no longer valid. |
| audienceFailed | 無法排入觀眾讀取，請重試。 | 无法排入观众读取，请重试。 | Could not queue the audience read. Try again. |
| audienceStorage | 無法安全保留此讀取請求，請重新載入頁面後再試。 | 无法安全保留此读取请求，请重新加载页面后再试。 | Could not retain this read request safely. Reload the page before trying again. |
| views | 影片觀看次數 | 视频观看次数 | Video views |
| peak | 最高同時觀看人數 | 最高同时观看人数 | Peak concurrent viewers |
| totalTime | 總觀看時間（毫秒） | 总观看时间（毫秒） | Total view time (ms) |
| viewTime | 觀看時間（毫秒），非人數 | 观看时间（毫秒），非人数 | View time (ms), not people |
| insufficient | 觀眾數不足，Meta 未提供輪廓 | 观众数不足，Meta 未提供轮廓 | Audience too small; Meta did not provide a profile. |
| notAuthorized | 需重新連接 Facebook 以授權觀眾數據 | 需重新连接 Facebook 以授权观众数据 | Reconnect Facebook to authorize audience insights |
| notRead | 尚未讀取觀眾數據，可使用讀取按鈕取得資料。 | 尚未读取观众数据，可使用读取按钮获取数据。 | Audience insights have not been read yet. Use the read action to request them. |
| ambiguous | 多個推廣同時進行，未分配到單一廣告 | 多个推广同时进行，未分配到单一广告 | Several promotions were live; these orders were not assigned to a single ad |
| post | 綁定貼文 | 绑定帖文 | Bound posts |
| draftIds | 關聯草稿 | 关联草稿 | Linked drafts |
| start | 開始 | 开始 | Starts |
| end | 結束 | 结束 | Ends |
| noTimeline | 尚未提供時間軸觀測。 | 尚未提供时间轴观测。 | No timeline observations provided. |
| noRows | 尚未提供紀錄。 | 尚未提供记录。 | No rows provided. |
| scrollHint | 橫向捲動以查看所有欄位。 | 横向滚动以查看所有列。 | Scroll horizontally to view all columns. |
| source | 來源參照 | 来源参照 | Source reference |

## Existing promotions operational and audit copy (retained catalogue)

Dynamic formatting expressions show their exact source; input values are supplied by the server/form. Source: apps/admin/lib/promotions-copy.ts.

| Key | zh-TW | zh-CN | en |
|---|---|---|---|
| errors.promo_exists | 已存在相同的優惠碼。 | 已存在相同的优惠码。 | A discount code with this text already exists. |
| errors.invalid_promotion | 優惠碼設定無效。 | 优惠码设置无效。 | The discount code settings are not valid. |
| errors.version_changed | 此優惠碼在載入後已被更改，列表已重新整理，請重試。 | 此优惠码在载入后已被更改，列表已刷新，请重试。 | This code changed since it was loaded. The list was refreshed; try again. |
| errors.idempotency_conflict | 此請求識別碼已用於另一個請求，請重新載入後再試。 | 此请求标识已用于另一个请求，请重新载入后再试。 | This request key was already used for a different request. Reload and try again. |
| errors.forbidden | 您沒有修改優惠碼的權限。 | 您没有修改优惠码的权限。 | You do not have permission to change discount codes. |
| errors.unauthorized | 登入已失效，請重新登入。 | 登录已失效，请重新登录。 | Your session ended. Sign in again. |
| errors.retry_later | 暫時無法使用，請重試。 | 暂时无法使用，请重试。 | Temporarily unavailable. Try again. |
| errors.default | 無法儲存變更，請重試。 | 无法保存更改，请重试。 | The change could not be saved. Try again. |
| nav | 優惠碼 | 优惠码 | Discount codes |
| title | 優惠碼 | 优惠码 | Discount codes |
| subtitle | 買家在購物車或結帳時輸入的代碼。優惠碼只減商品金額，不減運費，一張訂單只能用一個。 | 买家在购物车或结账时输入的代码。优惠码只减商品金额，不减运费，一张订单只能用一个。 | Codes buyers type in the cart or at checkout. A code takes money off the goods, never off shipping, and one order uses one code. |
| store | 店鋪 | 店铺 | Store |
| loading | 正在載入優惠碼… | 正在载入优惠码… | Loading discount codes… |
| signedOut | 請登入後管理優惠碼。 | 请登录后管理优惠码。 | Sign in to manage discount codes. |
| forbidden | 您沒有管理本店優惠碼的權限。 | 您没有管理本店优惠码的权限。 | You do not have permission to manage discount codes in this store. |
| notFound | 本店暫時無法使用優惠碼。 | 本店暂时无法使用优惠码。 | Discount codes are unavailable for this store. |
| unavailable | 優惠碼暫時無法使用，請重試。 | 优惠码暂时不可用，请重试。 | Discount codes are temporarily unavailable. Try again. |
| noStore | 沒有可用的授權店鋪。 | 没有可用的授权店铺。 | No authorized store is available. |
| retry | 重試 | 重试 | Try again |
| listTitle | 代碼 | 代码 | Codes |
| empty | 還沒有優惠碼。 | 还没有优惠码。 | No discount codes yet. |
| colCode | 代碼 | 代码 | Code |
| colDiscount | 優惠 | 优惠 | Discount |
| colMinimum | 最低消費 | 最低消费 | Minimum order |
| colWindow | 有效期（台北時間） | 有效期（台北时间） | Valid (Taipei time) |
| colUsage | 已使用 | 已使用 | Used |
| colPerBuyer | 每位買家 | 每位买家 | Per buyer |
| colStatus | 狀態 | 状态 | Status |
| colActions | 操作 | 操作 | Actions |
| statusActive | 啟用中 | 启用中 | Active |
| statusPaused | 已暫停 | 已暂停 | Paused |
| noMinimum | 無 | 无 | None |
| noLimit | 不限 | 不限 | No limit |
| always | 長期有效 | 长期有效 | Always |
| from | 自 | 自 | from |
| until | 至 | 至 | until |
| pause | 暫停 | 暂停 | Pause |
| resume | 恢復 | 恢复 | Resume |
| edit | 編輯 | 编辑 | Edit |
| percentOff | % 折扣 | % 折扣 | % off |
| createTitle | 新增優惠碼 | 新增优惠码 | New discount code |
| editTitle | 編輯優惠碼 | 编辑优惠码 | Edit discount code |
| code | 代碼 | 代码 | Code |
| codeHint | 3 到 24 個字母、數字或連字號。買家輸入時不分大小寫。建立後不能修改。 | 3 到 24 个字母、数字或连字符。买家输入时不分大小写。创建后不能修改。 | 3 to 24 letters, digits or hyphens. Buyers can type it in any case. It cannot be changed later. |
| kind | 類型 | 类型 | Type |
| kindPercent | 百分比折扣 | 百分比折扣 | Percent off |
| kindFixed | 固定金額折扣 | 固定金额折扣 | Amount off |
| valuePercent | 折扣百分比（1 到 90） | 折扣百分比（1 到 90） | Percent off (1 to 90) |
| valueFixed | 折扣金額 | 折扣金额 | Amount off |
| minSubtotal | 最低訂單金額 | 最低订单金额 | Minimum order amount |
| minSubtotalHint | 與折扣前的商品總額比較。留空表示不設最低金額。 | 与折扣前的商品总额比较。留空表示不设最低金额。 | Compared with the goods total before the discount. Leave empty for no minimum. |
| startsAt | 開始時間（台北時間） | 开始时间（台北时间） | Starts (Taipei time) |
| endsAt | 結束時間（台北時間） | 结束时间（台北时间） | Ends (Taipei time) |
| windowHint | 留空表示不設開始或結束時間。 | 留空表示不设开始或结束时间。 | Leave empty for no start or end. |
| totalLimit | 總使用次數 | 总使用次数 | Total uses |
| perBuyerLimit | 每位買家使用次數 | 每位买家使用次数 | Uses per buyer |
| limitHint | 留空表示不限。已取消和已過期的訂單會釋放其使用次數。買家以手機號碼、電子郵件或裝置辨識，因此此限制是嚇阻而非保證。 | 留空表示不限。已取消和已过期的订单会释放其使用次数。买家以手机号、邮箱或设备识别，因此此限制是威慑而非保证。 | Leave empty for no limit. Cancelled and expired orders free their use. A buyer is recognised by phone number, email or device, so this limit is a deterrent, not a guarantee. |
| statusLabel | 狀態 | 状态 | Status |
| create | 建立優惠碼 | 创建优惠码 | Create code |
| save | 儲存變更 | 保存更改 | Save changes |
| cancel | 取消 | 取消 | Cancel |
| invalid | 請檢查表單：代碼、金額或百分比、日期和次數限制。 | 请检查表单：代码、金额或百分比、日期和次数限制。 | Check the form: the code, the amount or percent, the dates and the limits. |
| invalidWhole | 新台幣金額為整數元，例如 500，請去掉小數。 | 新台币金额为整数元，例如 500，请去掉小数。 | NT$ amounts are whole dollars, for example 500. Remove the decimals. |
| created | 優惠碼已建立。 | 优惠码已创建。 | Discount code created. |
| saved | 變更已儲存。 | 更改已保存。 | Changes saved. |
| toggled | 狀態已更新。 | 状态已更新。 | Status updated. |
| uncertain | 結果尚未確認。請再次點擊同一按鈕，可安全重試。 | 结果尚未确认。请再次点击同一按钮，可安全重试。 | The result is unconfirmed. Press the same button again to retry safely. |
