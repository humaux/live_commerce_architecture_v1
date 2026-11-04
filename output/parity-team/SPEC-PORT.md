# SPEC-PORT — staff-team and OC reauth specs on the signed Team layout (2026-11-04)

Authority: integrator ruling 2026-11-04 (Figma frames are the production UI; port navigation/selectors, keep every behavioral assertion and count). Frame ids are from `tests/admin/parity-team-frames.json`; visible words are pinned from `packages/i18n/src/team-parity.ts` (zh-TW / zh-CN / en), the same pinning pattern the reauth spec already uses for `lib/reauth-copy.ts`.

Not touched: `tests/foundation/*.go`, `parity-team.mjs`, `parity-team-frames.json`, product code. Every `waitForResponse`, status code (201/204), token-leak check, `nthMail` SMTP read, `team list` API readback, order-actions/orders readbacks, invitee refusal checks, `sent` replay count/body equality, dialog/focus-trap/OIDC/cookie checks are byte-identical. Local: `node --experimental-strip-types --check` exit 0 on both files. CI: NOT_RUN.

## tests/admin/staff-team.spec.ts

| Old line | New line | Why (frame) | Assertion preserved |
|---|---|---|---|
| 8 header: "data-testid hooks the brief lists (team-invite-*, member-*, invite-*)" | header names the signed layout + the same hooks | comment only | n/a |
| 25 import `type Page` | adds `type Locator` | helper takes a locator | n/a |
| 123-124 `unobscuredMemberRole(page, testId)` + `getByTestId(testId)` | `unobscuredMemberRole(page, control: Locator)` | per-member role `<select>` no longer exists; the frame's per-member role control is the 變更角色 button (list `1468:74273` / `1468:74324`) | yes: same 3-point hit test, same message, same real click + Escape |
| — (new consts after 188) | `teamUi`, `teamButton`, `roleRadio` | button names and role radios per list/invite/role-edit frames | n/a (locators only) |
| 218 `team-invite-email` visible on load | invite entry button 邀請員工 visible | empty frame `1468:74559` / `1468:74603`: landing has the entry button, form opens on click | yes: "invite control offered to the owner on load" |
| 219 `team-members` visible | `team-page` contains 目前只有你一位員工 | same empty frame: a one-member team shows the empty card, not the list | yes: owner sees the (empty) staff state; `team-members` is still asserted absent for the invitee (345-346 unchanged) |
| — (before 228) | click 邀請員工 | entry → invite form (`1468:74979` invite-fulfilment) | n/a (click to reach the control) |
| 229 `team-invite-role` selectOption("fulfilment") | radio `name=team-role value=fulfilment` check | five role radios (`1468:74979`) | yes: same role code `fulfilment` sent; 201 + no-token + mail locale/link/recipient checks unchanged |
| 236-237 notice and invites table together | notice visible, click 回到員工, then `team-invites` contains invitee | invite-sent `1468:75217` / `1468:75260` → pending `1468:77685` (回到員工 opens pending tab) | yes: both `team-notice` visible and `team-invites` contains the invitee email |
| 359 `unobscuredMemberRole(member-role-<id>)` | `memberCard = member-<id>`; hit test + click its 變更角色 | list `1468:74273` → role-edit `1468:78796` | yes (unobscured check kept, now on the real per-member role control) |
| — | radio `viewer` check, click 繼續 | role-edit `1468:75792` → role-confirm `1468:75905` | n/a (click sequence) |
| 362 select change triggers set-role | 確認變更 click triggers set-role | role-confirm `1468:75905` / `1468:75945` | yes: same `waitForResponse('/api/team/set-role')`, same `204` |
| 365 `member-role-<id>` toHaveValue("viewer") | click 回到員工, `member-<id>` contains 僅供檢視 · 加入 / Viewer · Joined | role-done `1468:75960` → list-updated `1468:76023` / `1468:76074` | yes: UI readback of the persisted viewer role after a fresh list read; invitee order-actions `{refund:false,fulfillment_write:false,orders_export:false}` and export-hidden checks unchanged |
| 377 `member-remove-<id>` click | member card 移除 click | list `1468:74273` | n/a (locator) |
| 378 `member-remove-yes-<id>` waitFor | 確認移除 waitFor | remove-confirm `1468:76100` / `1468:76140` | yes: confirmation step still required before the write |
| 381 `member-remove-yes-<id>` click | 確認移除 click | remove-confirm `1468:76100` | yes: same `waitForResponse('/api/team/remove')`, same `204` |
| — (before 384) | click 回到員工, `team-page` contains 目前只有你一位員工 | remove-done `1468:76155` → empty `1468:74559` | added only so 384 runs on the list, not the done screen (otherwise 384 would pass vacuously) |
| 384 `member-<id>` count 0 | unchanged | — | yes; invitee 401/403/404 refusals unchanged |

## tests/admin/oc/reauth.spec.ts

| Old line | New line | Why (frame) | Assertion preserved |
|---|---|---|---|
| 8 header "existing Team page hooks (team-invite-*)" | "signed Team layout (invite entry, team-invite-*, role radios)" | comment only | n/a |
| — (new consts after 46) | `teamUi` (zh-TW/zh-CN/en invite + back), `teamButton` | entry button and invite-sent action | n/a |
| — (before 230) | click 邀請員工 | empty `1468:74559` → invite form `1468:75098` | n/a (click to reach the form) |
| 230 `team-invite-email` visible | unchanged (now after the entry click) | invite-viewer `1468:75098` | yes |
| 233 `team-invite-role` selectOption("viewer") | radio `value=viewer` check | five radios `1468:75098` | yes: same request body role; `sent` length 1, dialog visible, no sign-out navigation, still signed in — unchanged |
| 242 `team-invites` contains invitee (right after replay) | `team-notice` contains invitee | replay lands on invite-sent `1468:75217` | yes: replay success observed on the page; `team-invites` check kept below |
| 243-244 replay count 2, same body | unchanged | — | yes: exactly once, original bytes |
| 245 Send toBeFocused ("focus returns to the control that started the request") | 回到員工 toBeFocused ("focus lands on the action of the screen the replay opened") | Send no longer exists after SENT; invite-sent `1468:75217` / `1468:75260` has exactly one action, 回到員工 | replaced, not deleted: the WCAG 2.4.3 focus-return check now targets the frame's only control (see Risk) |
| — (after 245) | click 回到員工, `team-invites` contains invitee | pending `1468:77813` | yes: the old 242 invites-table readback |

Callers at 281 and 384 (zh-TW desktop, zh-CN desktop, en mobile) are unchanged; the OIDC callback `Location` check (`/team?store=…#reauth-done`) is unaffected because screens are client state.

## Risk — expected RED until a product follow-up (not applied: product code is out of scope)

`apps/admin/components/ReauthDialog.tsx:66-72` returns focus only to the opener when it is still connected. On a successful replay `Team.tsx:110` switches to the `sent` screen, which unmounts Send (`:175`), so focus falls to `<body>` and the new focus assertion will fail. This is a real accessibility gap (focus loss after the dialog), not a spec artifact. Minimal product fix proposal (needs a writer card for `Team.tsx`): give the 回到員工 button on the `sent` screen (`Team.tsx:181`) focus when that screen mounts (e.g. a ref + `useEffect`, or `autoFocus`), so the dialog's fallback finds focus already placed. The same screen-change focus loss applies to role-done/remove-done; the specs do not assert focus there.

## Proposed oracle fix (not applied) — `tests/admin/parity-team.mjs:217-224`

`content()` requires a `button` named after every listed string present in `frame.text`. In four frame families the string is the screen heading, not a button: `invite-{owner,admin,live,fulfilment,viewer}` (邀請員工), `role-edit*` and `role-confirm*` (變更角色), `pending-empty` (待接受邀請). In every one of them it is `frame.text[0]`, and no frame needs a button named after its own `text[0]`. Minimal fix:

```js
// line 218
if (!names.has(text) || text === frame.text[0]) continue;
// after the loop, keep the string strict as a heading
await expect(scope.getByRole("heading", { name: translate(frame.text[0], locale), exact: true })).toBeVisible();
```

`list`/`pending` keep 員工名單/待接受邀請/邀請員工 as buttons (their `text[0]` is 員工), and the disabled 邀請員工 in `password-off`/`mail-failed` is unaffected (their `text[0]` is the error title).

Evidence: E1 (syntax check + ownership `--check`). GREEN / browser run: NOT_RUN (CI only). Author PASS = E0.

## RELAY 7 — tests/admin/parity-team.mjs (owner decisions 2026-11-04)

Authority: owner lifted the invite pause (「现在解除暂停」) and chose 「两项都按设计稿做」 for admin promotion. Both R3 tests below were added by this PR (not preserved base tests). Every other check is kept.

| Old | New | Why (frame) |
|---|---|---|
| `fixtureState(state, store)`; invite body asserts `locale: "zh-TW"` | `fixtureState(state, store, locale = "zh-TW")`; asserts `locale` | the three-locale send test now reaches the invite API; zh-TW signed tests unchanged |
| `content()` requires a button for every listed string, incl. `frame.text[0]` | `frame.text[0]` is asserted as a heading; every other string as before | `invite-*` 邀請員工, `role-edit*`/`role-confirm*` 變更角色, `pending-empty` 待接受邀請 are page headings (exactly those 15 frames; no other frame has a button name as text[0]) |
| `R3 paused send`: heading 邀請郵件未送出 + help text | `R3 invite send`: `content(invite-sent)` in the test's locale | invite-sent `1468:75217` / `1468:75260` |
| 邀請員工 disabled | 送出邀請 count 0 | the sent frame offers no resend |
| writes 0 after submit | writes `[{action:"invite", role:"admin"}]` | owner-lifted pause: one real invite call |
| click 回到員工, reload, 員工名單 visible | same + mei@example.test visible after 回到員工 (pending tab) | invite-sent → pending |
| writes 0 after refresh | writes 1 after refresh | refresh never resends |
| `R3 admin promotion unavailable`: 繼續 disabled, writes 0, 取消, reload shows 僅供檢視 | `R3 admin promotion`: fixture `role-done-admin` (expects set-role admin); 繼續 → review 僅供檢視改為管理員。 (writes 0) → 確認變更 → 角色已變更, writes `[set-role admin]` → 回到員工 → reload shows 管理員 · 加入 10/12, writes 1 | owner decision; Go's owner check stays the authority |
