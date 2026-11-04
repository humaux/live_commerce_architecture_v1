# PARITY-TEAM design decisions (owner-delegated, 2026-11-04)

Owner, 2026-11-04: 「页内重新登录：升级管理员的确认页和完成页：你进行决定设计，符合业务。设计稿里过时的导航文字：你进行决定设计，符合业务。」

The owner delegated three design questions to the integrator. The integrator's decisions are below, with the reason for each, the exact frame ids affected and the current state in this PR. Signed sources: `FRAME-COPY.md` (SHA-256 `fd1ff3d7…ee55c0`), ux-team `DELIVERY.md` (SHA-256 `6a7c24e8…490b75`).

## 1. In-page re-auth: keep the shared ReauthDialog (DONE)

- Decision: keep the shared `ReauthDialog` (OC-04 contract: password step-up, replays the original request exactly once with its idempotency key, focus trap). Do not build in-page re-auth on the Team page.
- Reason: it works the same way on every admin page and is already security-tested (`tests/admin/oc/reauth.spec.ts`). Replaying with the idempotency key means a re-auth can never send a duplicate invitation. The in-page frames would have required a second re-auth path that does not replay the request.
- Frames superseded (both viewports, exact ids): `reauth` 1468:75538 / 1468:75582, `reauth-signin` 1468:96313 / 1468:96358, `reauth-verify` 1468:96378 / 1468:96420.
- Oracle: `tests/admin/parity-team-frames.json` `superseded` (line 6) lists these ids with the reason "owner-delegated design decision 2026-11-04: shared ReauthDialog (OC-04) replaces in-page re-auth". `tests/admin/parity-team.mjs:14-17,332`: each of these ids registers as a skipped test with that reason, and every other frame is still required. `:443-449`: the frozen-oracle test still requires all 82 frames / 164 ids, and allows only these three states, with their signed ids and this reason, to be superseded.
- `tests/admin/oc/reauth.spec.ts`: unchanged, all assertions kept.
- Note: the card named the mobile ids. Both viewport ids of each state are superseded, because the decision is about the behaviour, not the screen size.

## 2. Admin promotion confirm/done (BLOCKED: the frame copy says "no change")

- Integrator decision: role-edit → select 管理員 for a non-admin → 繼續 → `role-confirm-admin` (frame copy verbatim, explaining admin powers) → 確認變更 → existing set-role → `role-done-admin`. The backend permission check is the authority. Continue is disabled only when the role is unchanged.
- Already in place since RELAY 7 (no change in this relay): `apps/admin/components/Team.tsx:93,190`. Continue/submit refuse only an unchanged role, and promotion goes through set-role. A refusal shows the error screen. `tests/admin/parity-team.mjs` `R3 admin promotion` drives viewer → admin with real clicks: review 「僅供檢視改為管理員。」 → 確認變更 → 「角色已變更」 → refresh shows 管理員, one write.
- Not implemented, because the signed copy contradicts the premise:
  - `role-confirm-admin` 1468:78293 / 1468:78333 is verbatim 「角色不需變更 / Ari · ari@example.test / 目前已是管理員，無須變更。/ 回到員工」. It has no 確認變更 button and does not explain admin powers.
  - `role-done-admin` 1468:78348 / 1468:78392 is verbatim 「角色未變更 / … / 管理員 / 回到員工」.
  - ux-team DELIVERY:104 lists them as the "No-op role selection cannot submit" pair, and :181-183 marks all three -admin frames "client unchanged-role guard — NOT_SENT; role unchanged".
  - Showing them after a real promotion would tell the owner the role was not changed when it was, so it is not business-correct. Under the decision's own rule (Continue disabled when unchanged) they can never be reached.
- `role-edit-admin` 1468:78180 / 1468:78249 is reachable and asserted: an admin member with 管理員 selected sees 「目前已是管理員，無須變更。」 and Continue is disabled.
- Proposed decision (needs integrator/owner OK): supersede `role-confirm-admin` / `role-done-admin` (4 ids) with the reason "unchanged-role guard on role-edit-admin replaces the no-op review/result". If the promotion review should explain admin powers, show the signed role line 「管理員 · 管理業務，不含訂閱帳單與員工」 on the generic role-confirm screen when the target role is admin. That reuses existing three-locale copy, and the Figma role-confirm frame would get a variant.

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
- After the decisions on items 2 and 4: 1468:78293, 1468:78333, 1468:78348, 1468:78392, and the four invite sign-in frame pairs.

Evidence: E1 only (`node --check`, JSON parse, ownership generate + `--check`). Browser parity in CI: NOT_RUN at this head. SANDBOX/LIVE: NOT_RUN. Own PASS = E0.
