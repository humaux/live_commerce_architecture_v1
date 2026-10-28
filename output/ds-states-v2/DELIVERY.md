# DS-STATES-V2 · PR #274 · review fix attempt 1

Status: **E0 / CI PENDING**. This is the RED negative-control handoff, not completed acceptance.
Assigned branch/worktree: `unit/ds-states-v2`, `/Volumes/data/live_commerce_architecture_v1/.worktrees/ds-states`.
Merged `origin/r3/integration` first: `736b0e63f` (integration parent `7ce347b70`; same tree as initial merge `836b80dd9`, commit trailer corrected before pushing).
Executor: Codex, UI implementation role; runtime model/effort not exposed (UNKNOWN). No delegation.
Authority: current implementation card; final review in `output/integrator/tools/logs/cxf-rv-274.log` in the read-only main checkout; `AGENTS.md`; `docs/delivery/GATES.md` gallery contract.
Humaux search/store/canvas/coord: unavailable, no matching tools. Integrator must retain the handoff; no shared-state writes claimed.

## Changes and RED restoration

Adds a fixture to the existing `LC_UI_GALLERY=1` developer gallery, using the actual shared components and three-locale copy in `packages/i18n`. It does not add a route or a merchant setting. Synthetic operation `ds-states-example` is read through intercepted GETs only. Session storage persists fixture display state, never business truth.

Three marked RED sites must be restored after assertion-level CI failure: remove the early legacy returns in `StatePanel` and `InlineError`; restore `GalleryForbidden` body to `c.forbiddenBody`. Keep every assertion and fixture payload intact. Old assertions remain; new denial assertion exercises the actual rendered caller with a synthetic raw diagnostic. No tests/gates/baselines/exemptions were relaxed.

Local static checks: `node --check` on both click modules and `ds-rework.test.mjs`, exit 0; `git diff --check`, exit 0. Admin `tsc --noEmit` did not execute: `tsc: command not found`, missing worktree node_modules. No dependency installation, local tests, builds, server or browser.

## Acceptance gate and click ledger

CI Foundation must fail on the new/retained behavioral assertions at the RED head, then pass after restoration. The PR's own Gates must run `--browser-ui-gallery`; historical GREEN on `e3fe0f2d1` does not accept this revision. Its existing runner records every new step in `ui-gallery/click-ledger.{json,md}`. PENDING until run/job receipts and actual ledger are available.

Each row runs at zh-TW / zh-CN / en and 1440 / 390; every action is a real Playwright click. Measurements only use boundingBox; no DOM mutation, force-click or direct API call substitutes for an action.

| Page / control | Operation | Expected visible effect and persistence | Actual |
|---|---|---|---|
| gallery / empty primary | click link, reload | destination tab b remains selected; URL persists | PENDING CI |
| gallery / forbidden primary | click link, reload | same destination, no diagnostic/resource identifier in rendered copy | PENDING CI |
| gallery / expired primary | click link, reload | destination tab b and URL persist | PENDING CI |
| gallery / offline recovery | click callback, reload | one synthetic GET; loaded message persists, no replay on refresh | PENDING CI |
| gallery / unknown check | click callback, reload | GET same operation only; still awaiting confirmation; no submission/replay | PENDING CI |
| gallery / InlineError callback | click retry, reload | GET succeeds; loaded message persists | PENDING CI |
| gallery / InlineError link | click link, reload | destination tab b and URL persist | PENDING CI |
| all above | read hit rectangle | width and height >=44px on actual rendered action | PENDING CI |

## BOURDIEU · Q1–Q6

Q1 task: recover from a failed read or leave an inaccessible page. Q2 novice/expert means recent experience recovering this task, never class/device. Q3 novice sees cause and one next action. Q4 experienced user uses the same single action. Q5 no social/device-based capability assumptions; all locales and widths share semantics. Q6 aesthetics separate, UNKNOWN until owner; this card preserves the signed design and adds acceptance fixtures.

## PROJECT UNDERSTANDING

Read `架构.md` §0 (MODEL_ONLY/NOT_RUN are not product acceptance), §1 (TypeScript strict/React; business truth stays in Go/PG), §2 (do not invent merchant facts), §26 (isolated worktree and independent acceptance), §27 (slice gates and evidence). Read task DAG `contracts/tasks.json`, DS-3 baseline `docs/design/ds/DS-2.md`, gallery gate `docs/delivery/GATES.md`, shared States/Controls and existing gallery runner. Owner card requires UNKNOWN re-read only, three languages and real clicks; review requires assertion-level RED and denial-copy-path coverage. No business contract changed.

## MOBBIN → EAGLE

No new visual pattern or reference imported for this review fix. Preserves the approved `output/ux-states-ref/DELIVERY.md` handoff: Copilot `MVOVLMVSA15MW` single first action, Bevel `MVOVLO8WKRBD6` recovery in the current task, Toggl `MVRSAPZL4DT5D` permission exit. These are inherited reference receipts, not a new live Eagle observation. No brands/images/product facts copied. New Mobbin query needed: none for this fixture-only correction.

## FIGMA OFFICIAL GUIDANCE

No Figma writes or design hand-in. Reuses signed StatePanel `1429:34298` / InlineError `1429:34299`, their nested auto layout, single shared action and DS-3 variables recorded by the approved handoff. Requested `output/ux-audit-ref/FIGMA-GUIDE.md` is absent in this worktree and main checkout; no new official-guidance verification claimed. Live design parity/copy lint is NOT_RUN because this fix changes testing fixtures, not Figma frames.

## Desktop/mobile required-fact parity

| Required fact | Desktop fixture | Mobile fixture / refresh |
|---|---|---|
| title and body for each of five states | same shared props | same shared props; visible locale checks in CI |
| forbidden non-disclosing body/note | safe localized caller copy after RED restoration | same source; rendered no-ID assertions |
| UNKNOWN same-operation check, awaiting confirmation | shared read callback and status | same callback/status, refresh persistence asserted |
| one primary action | one link/button | same DOM, 44px measurement |
| InlineError message and recovery | shared component | same component; real-click effect and refresh checked |

## Visible copy · zh-TW | zh-CN | en

| zh-TW | zh-CN | en |
|---|---|---|
| 還沒有商品 | 还没有商品 | No products yet |
| 先新增第一件商品。 | 先添加第一件商品。 | Add your first product. |
| 目前無法開啟此頁 | 暂时无法打开此页面 | This page cannot be opened |
| 請返回店鋪繼續操作。 | 请返回店铺继续操作。 | Return to your store to continue. |
| 如需存取，請洽店鋪擁有者。 | 如需访问，请联系店铺所有者。 | Ask the store owner if you need access. |
| 暫時無法載入列表 | 暂时无法加载列表 | The list could not be loaded |
| 請檢查網路連線，再重新載入。 | 请检查网络连接，再重新加载。 | Check your connection and try loading it again. |
| 此連結已無法使用 | 此链接已无法使用 | This link is no longer available |
| 操作結果尚未確認 | 操作结果尚未确认 | The result is not confirmed |
| 請先查詢同一筆操作的狀態，確認前請勿再次送出。 | 请先查询同一笔操作的状态，确认前请勿再次提交。 | Check the same operation before trying again. |
| 等待確認 | 等待确认 | Waiting for confirmation |
| 已查詢狀態，仍在等待確認。 | 已查询状态，仍在等待确认。 | Status checked. Still waiting for confirmation. |
| 暫時無法載入最新狀態 | 暂时无法加载最新状态 | The latest status could not be loaded |
| 已重新載入列表。 | 已重新加载列表。 | The list has been loaded. |
| 重新載入 | 重新加载 | Reload |
| 查詢狀態 | 查询状态 | Check status |
| 返回店鋪 | 返回店铺 | Return to store |

## Scope and handoff

Original four implementation paths: States.tsx/index.ts/states.module.css/ds-rework.test.mjs. Additional acceptance paths need integrator approval: `apps/admin/components/{UiGallery.tsx,UiGallerySections.tsx,UiGalleryRecovery.tsx}`, `packages/i18n/src/{index.ts,state-recovery.ts}`, `tests/admin/{ui-gallery-clicks.mjs,ds-states-clicks.mjs}`, generated `output/ci-select-backend-browser/browser-ownership.json`, evidence `output/ds-states-v2/`. New browser helper requires the ownership generator in the same push; no manual generated-file edits. No Go dependency changes require depmap regeneration.

NOT_RUN: local tests/server/browser/build, TypeScript check (tool absent), GREEN for review fix, live/provider/PG acceptance, WebKit, independent review, Figma writes/copy lint/density probe. No task-owned processes or temporary fixtures left running. Existing untracked `output/ux-states-ref/` remains untouched.
