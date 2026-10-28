# INBOX-V2-IMPL — tests-only RED handoff

Status: **implementation NOT_STARTED; CI RED pending**. This is a partial, resumable handoff, not completion. No product source has changed. No local tests, browser, server, container, database or LIVE action ran.

## Authority and current scope

See `PLAN.md` for exact paths and acceptance matrix. UI authority is the integrator-supplied owner sign-off plus the current simplify/polish handoff:
`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/DELIVERY.md`
SHA-256 `a74b140f8abc0d7a50be3a55b9ee5bd180c84c4459c1ad8d2bbda4387def0d88`.

Current Fleet Bridge read: connected document, editorType figma, default file `iJ9lDdFIXgTlKkt5Tvrc0X`; read-only context `1429:39076`; fresh screenshots `1429:38826`, `1429:39076`, `1429:37361`, `1429:39424` under `figma/`. No Figma write, copy-lint mutation or bridge restart. All 43 authority frames remain the implementation target; the four captures are representatives, not whole-set acceptance.

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

Humaux preference/rejected search, coord/lock/store and canvas: tool unavailable, NOT_RUN. Library absence is not inferred.

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

All implementation/readback entries are PENDING until causal RED and the implementation push.

|Required desktop field/fact|Required mobile and every result/review presentation|Implementation / CI|
|---|---|---|
|Buyer and channel|Selected buyer/channel header, unchanged through send/recovery|PENDING|
|Store read/unread|Row/filter state and store-read metadata; persists after reload|New native-click assertion; PENDING|
|Assignment/takeover and paused automation|Always-visible disclosure label; use real assignment facts, no invented agent name|New disclosure assertion; PENDING|
|Manual identity link|Expanded details; never infer from name|Existing real A14 link/CAS tests retained; PENDING|
|Orders/cumulative spend unavailable|Expanded unavailable fact when server omits data; never an invented zero|New unavailable-fact assertion; PENDING|
|Incoming author/time/body|Header identifies author; original body/time once|Existing real-chain + new state tests; PENDING|
|Outgoing author/time/body/outcome|Original once, author from actual server facts, one result region|New queued/sent/failed/unknown assertions; PENDING|
|Recipient/channel before send|Header next to composer; remains in recovery|New three-language main path; PENDING|
|Draft/retained original|Editable before send/known rejection; one original after ACK; no hidden persisted buyer draft|Native fill/clear/send/edit; PENDING|
|Deadline/restriction/safe next step|Eligibility or one concrete result; UNKNOWN only query, never resend|New window/permission/connection/UNKNOWN assertions; PENDING|
|Phone/destination/totals/payment timing|N/A: not part of this inbox card|No field added|

## Three-language copy

The complete signed 62-string table remains at the authority path above. New additive Node assertions independently pin the 23 module keys in zh-TW/zh-CN/en, with `{time}`/`{channel}` placeholders instead of Figma sample facts. Existing consumer copy assertions remain unchanged. `packages/i18n` implementation is PENDING and must preserve all three locales.

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

- `node --check` on the new spec, modified copy/discovery suites and Playwright config: exit 0 (E1 syntax only).
- `git diff --check`: exit 0 (static whitespace only).
- `pnpm exec tsc ... tests/admin/inbox-v2-ui.spec.ts`: exit 254, `Command "tsc" not found`; typecheck NOT_RUN, dependency installation not attempted.
- `node scripts/dev/derive-browser-ownership.mjs`: exit 0, generated 53-mode ownership artifact after staging the new spec. This is static generation, not a test.
- RED/GREEN/browser/density/independent acceptance: PENDING / NOT_RUN. Exact push run IDs go in `red.log` and final WAIT_RUN handoff.
- Outside module paths: `playwright.config.ts` ADD-only test registration and generated `output/ci-select-backend-browser/browser-ownership.json`; explicitly list in PR body for integrator review. No dependency map change needed so far (no Go graph changes).
- Rollback point: clean base SHA above; this tests-only commit is isolated on the assigned branch. No rollback command was executed.
- No background process was started. Owned captures stay under this task's output directory; no other lane material was edited/deleted.
- Author assessment E0; static syntax/generation E1. **No completion or product PASS claimed.** Humaux store/canvas remains unavailable.
