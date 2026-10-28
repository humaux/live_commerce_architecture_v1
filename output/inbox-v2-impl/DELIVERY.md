# INBOX-V2-IMPL — implementation / CI relay handoff

Status: **implementation written; exact-head GREEN and independent acceptance PENDING**. Author assessment E0. No local test, browser, server, PG, container or LIVE action ran.

PR [#294](https://github.com/humaux/live_commerce_architecture_v1/pull/294), draft into `r3/integration`. Verified tests-only RED head `2b63813f3d9c826c4ed0da9517d3ba4ab643f137`; Foundation run `40772119429`, job `123111778889`, synthetic merge `dfebcc0daa0d38fcd845ced00a11db6c79e449d8`. Three NEW signed `simpleTitle` assertions fail at `tests/admin/inbox-copy.test.ts:101/104` (undefined versus signed copy), exit 1. E3 proves this copy RED only. Browser RED in Gates `40772119497` remained pending when implementation began. Raw filtered receipt: `red.log`.

One whole-card implementation push follows this record. Its Foundation/Gates RUN ids are the next WAIT_RUN handoff; final run/job evidence belongs in a PR comment, without an evidence-only commit. The integrator relays a fresh session when the jobs finish. No completion is claimed.

## Authority and current scope

See `PLAN.md` for exact paths and acceptance matrix. UI authority is the integrator-supplied owner sign-off plus the current simplify/polish handoff:
`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/DELIVERY.md`
SHA-256 `a74b140f8abc0d7a50be3a55b9ee5bd180c84c4459c1ad8d2bbda4387def0d88`.

Initial tests-only Fleet Bridge read: connected document, editorType figma, default file `iJ9lDdFIXgTlKkt5Tvrc0X`; read-only context `1429:39076`; fresh screenshots `1429:38826`, `1429:39076`, `1429:37361`, `1429:39424` under `figma/`. No Figma write, copy-lint mutation or bridge restart. All 43 authority frames remain the implementation target; the four captures are representatives, not whole-set acceptance.

## BOURDIEU — Q1–Q6

- Q1: novice/expert denotes recent experience replying to messages; no class, income, device or demographic inference. First task is choose a buyer, then reply or recover safely.
- Q2: preserve selection → thread → send/status/recovery → back; controls must reveal a visible result through native clicks.
- Q3: keep channel, store read state, takeover/paused automation, original text, reply deadline and delivery outcome; move repetition/customer facts into disclosure.
- Q4: identities come from server facts/manual links. Never infer a customer from a display name or replace an absent agent name with sample A/B.
- Q5: desktop/mobile retain every required fact and safe recovery action; parity checklist below. Real comprehension remains NOT_RUN.
- Q6: aesthetics are separate from comprehension. Owner acceptance of the simplified frames is supplied in this card (2026-10-28). No further preference is inferred.

## PROJECT UNDERSTANDING

|Read authority|Business rule imposed on UI|
|---|---|
|`架构.md` §§0/1/2|MOCK UI evidence cannot prove LIVE delivery; UI only, Go/PostgreSQL and authenticated store remain authoritative.|
|`架构.md` §10|Channel eligibility, reply window, consent and human takeover are separate facts. Human takeover pauses automation.|
|`架构.md` §§26/27 + `AGENTS.md`|One owned worktree/card; native interactions and independent acceptance; no delegation from this dispatched lane.|
|`contracts/live-console-v1.md` §3.3|Sending obeys server window/permission/idempotency; no channel fallback or blind UNKNOWN retry.|
|Same contract §3.6|Store read state persists server-side; operator read/takeover permissions and generation remain enforced.|
|Same contract §4.4, existing A8/A9/A13/A14 code|Distinct queued/sent/failed/blocked/unknown outcomes; customer link/CAS, privacy clearing and uncertain receipts cannot be simplified away.|
|Current implementation card|No local tests/servers; one whole-card RED push before implementation; three locales; fixed chip order; no new settings/API/contract.|

Relay: Humaux search/get and rejected representative enumeration ran after tools became available. Read current development rule 7bdb220b-0798-4b1d-aff5-16ee94cae75f. Dispatch ownership is retained; no new claim or recursive delegation. Shared writes use coord_lock. The card is the UI/authorization authority; recalled summaries are leads only.

## MOBBIN → EAGLE

Fresh read of the existing module folder `MVQL9Z23MS9BM` returned success and eight items; bounded receipt `eagle-reference-read.json` stores only the two used candidates' metadata. No import/download or copied image/brand/product fact.

|Existing candidate|Structural principle applied to implementation plan/tests|
|---|---|
|`MVNUXG8K4FQ39` · Quo|Separate list selection, thread/composer and customer context; mobile switches view rather than squeezing three columns.|
|`MVOVLMVQ5ZSCB` · Chime|Short list rows and one concrete result/recovery region; no repeated draft/status copy. No Undo feature added.|

Additional Mobbin query needed: none for this signed implementation. Eagle membership is freshly observed; source imagery was inherited from the signed handoff, not independently re-imported.

## FIGMA OFFICIAL GUIDANCE

Read the project guide at `/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-audit-ref/FIGMA-GUIDE.md` (guide date 2026-10-28, no fixed upstream version). The main-checkout copy was absent; the design worktree contains the authoritative referenced guide.

- G1 [Auto layout](https://help.figma.com/hc/en-us/articles/360040451373-Guide-to-auto-layout-in-Figma): preserve fill-width rows and symmetric insets with responsive CSS and min-width guards.
- G2 [Variables](https://help.figma.com/hc/en-us/articles/15343107263511-Apply-variables-to-designs): reuse existing semantic colors/spacing; customer disclosure defaults closed; language does not alter market facts.
- G3 [Components](https://help.figma.com/hc/en-us/articles/360038662654-Guide-to-components-in-Figma): reuse Inbox/InboxThread/BuyerPanel; no new framework or duplicate backend authority.
- G4/G5 [Prototyping](https://help.figma.com/hc/en-us/articles/360040314193-Guide-to-prototyping-in-Figma): every branch has a visible result/back path; UNKNOWN query is distinct from send; test with native actions.

This records application of the supplied guide; no current upstream documentation re-verification or Figma write is claimed.

## Desktop → mobile / summary-result parity checklist

The source locations below are author-reviewed mappings, E0. Runtime click, refresh and density results remain PENDING exact-head CI; source mapping is not an acceptance receipt.

|Required desktop field/fact|Required mobile and every result/review presentation|Implementation / CI|
|---|---|---|
|Buyer and channel|Selected buyer/channel header, unchanged through send/recovery|`InboxThread` header / same header around result; CI PENDING|
|Store read/unread|Row/filter state and store-read metadata; persists after reload|New native-click assertion; PENDING|
|Assignment/takeover and paused automation|Always-visible disclosure label; use real assignment facts, no invented agent name|`InboxThread` disclosure label / result states; CI PENDING|
|Manual identity link|Expanded details; never infer from name|Expanded `BuyerPanel`; real A14/CAS assertions retained; CI PENDING|
|Orders/cumulative spend unavailable|Expanded unavailable fact when server omits data; never an invented zero|`BuyerPanel compact` unavailable fact; CI PENDING|
|Incoming author/time/body|Header identifies author; original body/time once|Existing real-chain + new state tests; PENDING|
|Outgoing author/time/body/outcome|Original once, author from actual server facts, one result region|New queued/sent/failed/unknown assertions; PENDING|
|Recipient/channel before send|Header next to composer; remains in recovery|New three-language main path; PENDING|
|Draft/retained original|Editable before send/known rejection; one original after ACK; no hidden persisted buyer draft|`InboxThread` composer/result/edit; native fill/clear/send/edit CI PENDING|
|Deadline/restriction/safe next step|Eligibility or one concrete result; UNKNOWN only query, never resend|`InboxThread` eligibility/result/help; CI PENDING|
|Phone/destination/totals/payment timing|N/A: not part of this inbox card|No field added|

## Three-language copy

The complete signed table remains at the authority path above. `packages/i18n/src/inbox.ts` now owns the typed zh-TW/zh-CN/en entries, exported through the locale package and resolved by the existing inbox copy/error mapper. Independent additive copy assertions use the signed literals and `{time}`/`{channel}` placeholders. Legacy copy keys stay intact for standalone consumers. Browser tests cover the main screen in all three locales.

|zh-TW|zh-CN|en|
|---|---|---|
|全店訊息|全店消息|Store inbox|
|全部|全部|All|
|未讀|未读|Unread|
|本店分派|店铺分配|Assigned in store|
|顧客資料|客户资料|Customer details|
|傳送|发送|Send|
|清除草稿|清空草稿|Clear draft|
|查詢狀態|查询状态|Check status|
|編輯原文|编辑原文|Edit original text|
|已鎖定|发送已锁定|Sending locked|
|傳送中；尚未確認送達，勿重送。|发送中；尚未确认送达，不要重发。|Sending; delivery unconfirmed. Do not resend.|
|已送出，未代表已讀。|已发送，不代表已读。|Sent does not mean read.|
|傳送遭拒；原文保留，可編輯。|发送被拒；原文已保留，可以编辑。|Send rejected; original text saved for editing.|
|送達待確認；只查詢，不重送。|送达情况待确认；只查询，不重发。|Delivery unconfirmed; check status without resending.|
|仍待確認；到 {channel} 查看，勿重送。|尚未确认；请到 {channel} 查看，不要重发。|Still unconfirmed; check {channel}. Do not resend.|
|連接待確認；暫不能回覆。|连接待确认；暂时无法回复。|Connection unconfirmed; replies are unavailable.|
|缺少回覆權限。|缺少回复权限。|Reply permission is missing.|
|已超過回覆時間。|已超过可回复时间。|The reply window has ended.|
|自動回覆已停|自动回复已停|Auto-replies paused|
|店內已讀|店内已读|Read by store|
|訂單與累計：未提供|订单与累计消费：未提供|Orders and lifetime spend: not provided|
|{time} 前回覆|请于 {time} 前回复|Reply by {time}|

## Click ledger and gate status

New `tests/admin/inbox-v2-ui.spec.ts` registers six cases: three locales × (main at 1440/390; mobile result-state matrix). Native click/fill/select and reload assert fixed filters, read persistence, folded customer facts, clear draft/back/channel, send once, query→sent/reload, rejection→edit, UNKNOWN→query/reload, blocked, closed window, connection/permission restrictions. `page.evaluate` only reads text density and geometry; routes explicitly prepare isolated MOCK responses. The existing real BFF/Go/PG suite is retained. MOCK persistence assertions are not real-PG evidence.

Runtime click ledger will append to the existing suite's `click-ledger.json`; no unexecuted control is marked PASS here. New tests preserve the old discovery counts/calibration by adding a separately counted six-case matrix.

- Allowed static validation: `node --check` on changed TS locale/copy/harness/spec files, `git diff --check`, and `node scripts/dev/derive-browser-ownership.mjs`. Exact exits are recorded in `static.log`. These are E1 only; JSX/type/runtime acceptance belongs to CI.
- Local `tsc`: NOT_RUN, no installed compiler/dependencies (`Command "tsc" not found` in prior RED preparation). No dependency install was attempted.
- GREEN/browser/density/refresh/independent acceptance: PENDING / NOT_RUN. Six INV2 cases retain the unchanged density thresholds and add actual clicks on both restriction explanations. Existing real BFF/Go/PG suites retain their permission, CAS, read, idempotency, privacy and boundary assertions.
- Required harness adaptation: every `Inbox.tsx` / `InboxThread.tsx` / `BuyerPanel.tsx` importer was found with `rg -l`. They share `inbox-review-host.test.ts`; it now supplies `STORE_TIME_ZONE` and resolves icon buttons by accessible name as well as visible text. No timeout or waiting budget changed.
- Additional direct-consumer paths outside the initial PLAN: `tests/admin/inbox-unknown.test.ts` (signed copy; stricter pending/policy locks; four new-inbound counterexamples) and `tests/admin/inbox-window.test.ts` (two signed closing-copy literals). Register with integrator in PR body. Tests-only registration already includes `playwright.config.ts` and generated `output/ci-select-backend-browser/browser-ownership.json`.
- No API, contract, migration, pin, baseline, exemption, dependency or lockfile change. No other app/backend edit. No Go dependency edge changed, so no depmap update is needed.
- Rollback reference: assigned branch base `9a5a2f765c29fc0ab59fbb71e68ba468b31041e8`; source changes are isolated in the implementation commit. Integrator can revert that commit through its normal PR gate. No rollback ran.
- No background local process, temporary fixture, container or port was started. Owned Figma captures are retained as read evidence.

## Implementation details and limits

- Fixed All / Unread / Assigned-in-store slots, Instagram filter and full-width rows use A8 facts. Assignment filters only loaded authorized A8 rows, with existing cursor pagination; there is no invented server filter or claimed global assignment total. Existing uncommon filters remain under one overflow disclosure; this preserves the real-chain filter assertions while keeping the primary view simple.
- `InboxThread` owns the shared desktop/mobile customer disclosure and folds its existing BuyerPanel, takeover/release, published-template and older-history controls. No setting was added. Read state changes only after successful A10; privacy/session fences remain authoritative.
- Known sending outcomes use one result region and retain the original outgoing text once. Rejected text can be restored for editing. Queued/blocked/UNKNOWN results refuse a fresh send, including captured-form bypass; uncertain transport acknowledgment still permits only the existing exact-body/exact-key receipt retry. UNKNOWN remains latched if a later page omits it. A newer inbound reopens the signed composer only after a confirmed sent outcome.
- Sender names are not exposed in current A9/A8 responses (principal/assignee UUIDs only). Show the real human-takeover/paused-automation role, not invented sample Agent A/B names or raw UUIDs. This DTO limitation is disclosed; no API expansion was authorized.
- Orders/cumulative-spend omission remains unavailable, never a synthetic zero. Standalone console BuyerPanel behavior is retained; inbox-only unavailable copy is additive.
- `BOURDIEU`, `PROJECT UNDERSTANDING`, `MOBBIN → EAGLE`, and `FIGMA OFFICIAL GUIDANCE` sections above document the required process. The pinned brief controls; local skill setup scripts/tests were not run under the card's static-only restriction. No Figma mutation or design hand-in occurred; current design-copy lint is N/A to this code-only change.
- Fresh relay read: Fleet Bridge connected; `get_design_context` / `get_screenshot` re-read `1429:39076`, and screenshot `1429:39424`; captures in `figma-current/`. No uncertain write/restart.
- Humaux: causal RED store accepted for background indexing; own canvas `integrator-inbox-v2-impl-sub-1` updated successfully (1 element, no conflicts), under coord_lock and released. Shared parent task/canvas untouched.
- Author E0; static checks E1; causal copy RED E3 only. Product completion, LIVE and independent E4 are **not claimed**.
