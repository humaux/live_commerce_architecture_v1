# PARITY-TEAM design decisions (owner-delegated, 2026-11-04)

Owner, 2026-11-04: 「页内重新登录：升级管理员的确认页和完成页：你进行决定设计，符合业务。设计稿里过时的导航文字：你进行决定设计，符合业务。」

The owner delegated three design questions to the integrator. The integrator's decisions are below, with the reason for each, the exact frame ids affected and the current state in this PR. Signed sources: `FRAME-COPY.md` (SHA-256 `fd1ff3d7…ee55c0`), ux-team `DELIVERY.md` (SHA-256 `6a7c24e8…490b75`).

## 1. In-page re-auth: keep the shared ReauthDialog (DONE)

- Decision: keep the shared `ReauthDialog` (OC-04 contract: password step-up, replays the original request exactly once with its idempotency key, focus trap). Do not build in-page re-auth on the Team page.
- Reason: it works the same way on every admin page and is already security-tested (`tests/admin/oc/reauth.spec.ts`). Replaying with the idempotency key means a re-auth can never send a duplicate invitation. The in-page frames would have required a second re-auth path that does not replay the request.
- Frames superseded (both viewports, exact ids): `reauth` 1468:75538 / 1468:75582, `reauth-signin` 1468:96313 / 1468:96358, `reauth-verify` 1468:96378 / 1468:96420.
- Oracle: `tests/admin/parity-team-frames.json` `superseded` (line 6) lists these ids with the reason "owner-delegated design decision 2026-11-04: shared ReauthDialog (OC-04) replaces in-page re-auth". `tests/admin/parity-team.mjs:14-18,333`: each of these ids registers as a skipped test with that reason, and every other frame is still required. `:446-455`: the frozen-oracle test still requires all 82 frames / 164 ids, and pins the superseded list to these three states plus the two in §2, each with its signed ids and reason.
- `tests/admin/oc/reauth.spec.ts`: unchanged, all assertions kept.
- Note: the card named the mobile ids. Both viewport ids of each state are superseded, because the decision is about the behaviour, not the screen size.

## 2. Admin promotion confirm/done: the no-op frames are superseded (DONE)

- Integrator's first plan: role-edit → select 管理員 for a non-admin → 繼續 → `role-confirm-admin` → 確認變更 → set-role → `role-done-admin`. This was revised because the signed copy of those two frames describes a no-op, not a promotion:
  - `role-confirm-admin` 1468:78293 / 1468:78333 reads 「角色不需變更 / … / 目前已是管理員，無須變更。/ 回到員工」, with no 確認變更.
  - `role-done-admin` 1468:78348 / 1468:78392 reads 「角色未變更 / … / 管理員 / 回到員工」.
  - ux-team DELIVERY:104 lists the pair as "No-op role selection cannot submit", and :181-183 marks them "NOT_SENT; role unchanged".
- Integrator ruling (2026-11-04, under the owner's delegation):
  - (a) Supersede exactly those 4 ids. Reason: "owner-delegated design decision 2026-11-04: no-op admin change is blocked at role-edit (Continue disabled, 目前已是管理員，無須變更); no confirm/done screen for an unsent request".
  - (b) `role-edit-admin` 1468:78180 / 1468:78249 stays required.
  - (c) The generic role-confirm screen shows the signed admin line when the target role is 管理員.
- Reason: a request that is never sent has no review or result. Showing 「角色未變更」 after a real promotion would be false. The guard on role-edit already tells the owner why Continue is disabled.
- Oracle:
  - `tests/admin/parity-team-frames.json` `superseded` (lines 31-46) holds both entries.
  - `tests/admin/parity-team.mjs:446-455` pins the superseded list to exactly the five states (3 re-auth + 2 no-op admin), each with its signed ids and reason, 10 ids in total.
  - (b) `parity-team.mjs:186-191,230`: real clicks on 變更角色 for the admin member Ari. 管理員 stays selected, 「目前已是管理員，無須變更。」 is shown, 繼續 is disabled and nothing is written.
- Product (c): `apps/admin/components/Team.tsx:196` shows `c.roleLines.admin` on the role review when the target is admin. That is 「管理員 · 管理業務，不含訂閱帳單與員工」 in zh-TW, with the signed zh-CN/en lines in those locales.
  - The `R3 admin promotion` test (`parity-team.mjs:423`) asserts this line verbatim on the viewer → admin review.
  - Promotion itself is unchanged since RELAY 7: Continue is refused only for an unchanged role (`Team.tsx:93,190`), it goes through set-role, the backend decides, and a refusal shows the error screen.

## 3. Stale nav label: the shared shell is correct (DONE)

- Decision: the shared shell breadcrumb 設定 > 員工 is correct. `設定與員工` is a stale label from the old module shell.
- Reason: one shell across all admin pages. The signed frame's own breadcrumb already reads 「DaWan Live / 設定 / 員工」.
- Frames: `nav` 1466:74190 / 1468:74244. Only that frame's expected text changes in `tests/admin/parity-team-frames.json`: `設定與員工` is removed, and `設定` and `員工` stay expected. There is an inline `note` at line 703. The locale entry is kept unchanged.

## 4. Invite sign-in copy (PROPOSAL only, not implemented)

- Conflict: `signin` 1468:76904 / 1468:76925, `signup` 1468:76946 / 1468:76968, `verify-signin` 1468:76990 / 1468:77006 and `verify-signup` 1468:77022 / 1468:77038 use 驗證碼 / 查看信箱驗證碼 / 傳送驗證碼 / 使用收到邀請的信箱。. The invite page's links lead to the shared sign-in page, whose signed entry copy is 確認碼 / 請查看信箱 / 輸入確認碼 (`packages/i18n/src/entry-parity.ts:56-67`).
- Proposed: one sign-in page, one vocabulary. Keep the entry page's signed copy, and change only these four frames' sign-in/verify strings to the entry page's text (same pattern as decision 3). Keep the invite-specific checks: 加入團隊, email prefill, and no auto-accept after verify. Move 「使用收到邀請的信箱。」 to the invite page above its 登入 / 建立帳號 links.
- Separate follow-up: ReauthDialog (`apps/admin/lib/reauth-copy.ts`) says 驗證碼 while entry says 確認碼. That should be unified in one product-wide copy card.

## Follow-up: Figma frames to be updated to match

- 1466:74190, 1468:74244: replace 設定與員工 with the shared shell (設定 > 員工).
- 1468:75538, 1468:75582, 1468:96313, 1468:96358, 1468:96378, 1468:96420: mark as superseded by the shared ReauthDialog, or replace them with the dialog.
- 1468:78293, 1468:78333, 1468:78348, 1468:78392: mark as superseded by the role-edit-admin guard.
- 1468:75905 / 1468:75945 (role-confirm): add a variant where the target role is 管理員, showing 「管理員 · 管理業務，不含訂閱帳單與員工」.
- After the decision on item 4: the four invite sign-in frame pairs.

Evidence: E1 only (`node --check`, JSON parse, ownership generate + `--check`). Browser parity in CI: NOT_RUN at this head. SANDBOX/LIVE: NOT_RUN. Own PASS = E0.
