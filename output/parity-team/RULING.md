# PARITY-TEAM ROUND 3 ruling implementation

## RELAY 7 — oracle fix, invite send test, invitee page, owner decision D (2026-11-04)

Owner decisions 2026-11-04 (relayed by the coordinator): invite pause lifted (「现在解除暂停」, RELAY 6); for the two blocked groups 「两项都按设计稿做」. Backend, BFF, migrations, contracts and the ReauthDialog are untouched. Old→new test lines: SPEC-PORT.md "RELAY 7".

Done:
- A. `tests/admin/parity-team.mjs:376` `R3 invite send` asserts the signed invite-sent frame via `content()` in all three locales, one invite write, no resend, pending list after 回到員工, no write on refresh. Fixture takes the test locale (`:30`).
- B. `parity-team.mjs:221`: a frame's `text[0]` is asserted as a heading (15 frames: invite-*, role-edit*, role-confirm*, pending-empty); all other text/button checks unchanged.
- C. `apps/admin/components/TeamInvite.tsx` renders the signed invitee copy (`packages/i18n/src/team-parity.ts:89-103,188-202,287-301`, verbatim oracle translations) over the existing accept API: signed out → link (登入 / 建立帳號, invite-next fragment); signed in → 接受邀請 + 重新登入 (ends the session with the shell's existing `logoutWorkspace`, then sign-in with invite-next); 200 → joined (role from the accept answer); 404 invite_invalid / unauthorized → 邀請無法使用 (one generic text for expired/revoked/used/wrong-account, as before); 409 already_member → invitee-member; 404 not_found (BFF: email sign-in off) → invitee-off with 接受邀請 disabled; anything else → 確認中, no accept button, never re-accepted. Existing testids kept for staff-team.spec.ts.
- D1. Admin promotion: `apps/admin/components/Team.tsx:93,190` Continue/submit refuse only an unchanged role; promotion goes through the existing set-role; Go's owner check decides and a refusal shows the existing error screen. `parity-team.mjs:405` `R3 admin promotion` now drives viewer → admin through review, 確認變更 and refresh.

Still blocked (not faked):
1. `role-confirm-admin` / `role-done-admin` (1468:78293/78333, 1468:78348/78392): the signed role-edit-admin frame shows Continue disabled for an unchanged role, ux-team DELIVERY marks all three "client unchanged-role guard — NOT_SENT", and `ux-team/prototype-links.json` has no link into either frame. With admin promotion enabled the fixture's admin member selecting admin is still a no-op, so no real click reaches them. Needs a ux-team ruling on how a no-op selection reaches these frames.
2. In-page re-auth `reauth` / `reauth-signin` / `reauth-verify` (1468:75538/75582, 96313/96358, 96378/96420): NOT implemented. The two acceptance sources contradict on behaviour, not locators: the parity oracle signs in with email+password through `/api/auth/password/login` → `verify` and requires `writes == 1` after verify ("fresh auth returns to a list read without automatically repeating the write", `parity-team.mjs` recovery `reauth-verify`); OC-04 (`tests/admin/oc/reauth.spec.ts:232-257,292-300,324-347`) requires password-only `/api/auth/password/reauth/start|complete`, a replay of the original invite exactly once with the same body, focus trapped in `dialog[open]`, and the OIDC route returning to `/team#reauth-done`. No locator-only port of the OC spec can keep "replayed once" while the oracle forbids the replay. Owner/integrator must pick one behaviour.
3. `/invite/[token]` `signin`, `signup`, `verify-signin`, `verify-signup`: after the invite page's links the frames are the shared PasswordAuth entry page, which renders the signed entry-parity copy (確認碼, 請查看信箱) that differs from these team frames (驗證碼, 查看信箱驗證碼, 傳送驗證碼). The invite page itself now passes the `加入團隊` step; the rest needs an entry/team copy ruling on the shared component.
4. `nav` (1466:74190 / 1468:74244): unchanged from RELAY 6 item 5 (`設定與員工` in the signed frame vs the approved shared shell).

E1 only: `node --check tests/admin/parity-team.mjs` exit 0; i18n key sets equal across locales; ownership regenerate + `--check` exit 0. No local tests/tsc/server. GREEN, parity, SANDBOX and LIVE are NOT_RUN. Own PASS = E0.

## RELAY 6 — owner lifted invite pause 2026-11-04

Owner decision 2026-11-04 (relayed via AskUserQuestion: 「現在解除暫停」): the staff-invite pause is lifted. Backend, outbox, migrations, BFF route, tests and oracle files are untouched.

What changed:
- `apps/admin/components/Team.tsx:103-114` `sendInvite()` calls the existing `inviteMember` (`apps/admin/lib/team-client.ts:59`, BFF `/api/team/invite`) as base does; `:169` the form submits there instead of the paused view; `:175` Send is disabled while busy. SENT → `sent` screen (`:177-183`, `data-testid="team-notice"`; 回到員工 opens the pending tab), FAILED → existing mail-failed screen, UNKNOWN mail / `retry_later` / `unavailable` → checking (read-only reconcile, never resent; `:115-129` matches the invitation by email+role).
- `:134-136` error copy per code (`invalid_email`, `already_member`, `too_many_invitations`, `owner_role_disabled`, re-auth codes); `:209` invite errors offer 返回 back to the filled form.
- Re-auth: like base, 403 `reauth_required` is handled by the shared `ReauthDialog` fetch wrapper (one dialog, one replay); Team adds no own re-auth flow.
- `packages/i18n/src/team-parity.ts:78-88,162-172,246-256`: 11 keys per locale, copied verbatim from the signed oracle translations (FRAME-COPY).
- Unchanged: admin promotion Continue stays disabled (`Team.tsx:92,188`); this decision does not cover it.

Now reachable by real clicks (CI NOT_RUN): `invite-sent`, `invite-sent-{owner,live,fulfilment,viewer}`, `mail-failed` (real FAILED answer), `invalid-email`, `already-member`, `invite-limit`, `owner-disabled`, `invite-checking`, `checking-unresolved`.

Still blocked:
1. Admin `role-confirm-admin` / `role-done-admin` (4 frames): still blocked by the Continue-disabled ruling.
2. `reauth`, `reauth-signin`, `reauth-verify`: the shared ReauthDialog (OC-04, required by `tests/admin/oc/reauth.spec.ts`) catches 403 `reauth_required` before Team does, so the in-page 重新登入 *button* + sign-in copy 「重新登入後，先查看最新名單。」 that the oracle expects never show. The oracle and the OC-04 contract conflict here.
3. `/invite/[token]` states (`link`…`invitee-off`): `TeamInvite.tsx` has none of the signed copy (no 加入團隊). That is a separate parity unit outside this card's Team.tsx scope, so it is not attempted here.
4. Real-chain specs: the restored invite API covers the SMTP/replay counts *only* once the specs can reach it. `tests/admin/staff-team.spec.ts:218,229,236-237,362,377-381` and `tests/admin/oc/reauth.spec.ts:230-245` drive the legacy layout: an always-visible email field on the landing page, `<select data-testid="team-invite-role">`, `member-role-*` select, `member-remove(-yes)-*`, the notice and the invites table shown together, and focus returning to Send after the replay. `parity-team.mjs:112-125,227,231` require the signed flow instead: an entry button, five radios, and no 角色 combobox. One page cannot satisfy both without gaming one of them, so `browser_staff_team_test.go:197-217` / `browser_oc_test.go:259,279` are expected to stay red until the integrator reconciles the specs. No spec or oracle was edited.
5. `parity-team.mjs:214` nav: the signed Figma frame copy itself contains `設定與員工` (FRAME-COPY `1466:74190` and `1468:74244`), next to `設定` / `員工`. It is a stale module-shell label left in the signed frame. The approved shared shell (`設定` > `員工`) is correct, so the frame copy and the oracle that inherits it are the side that is wrong.
6. `parity-team.mjs:217-224` heading-as-button lookup: unchanged from RELAY 5.

E1 only: ownership regenerate + `--check` exit 0. No local tests/tsc/server. GREEN, parity, SANDBOX and LIVE are NOT_RUN. Own PASS = E0.

## RELAY 5 — integration merge, overflow hardening, oracle items (2026-11-04)

Merged `origin/r3/integration` (merge `d06f6c925`): kept #531 seeded fixture data and `registerTeamParity` in `tests/admin/parity-shell-browser.mjs` (only the header comment conflicted); `browser-ownership.json` regenerated, `--check` exit 0. Team uses its own `page.route("**/api/team/*")`, so seeding does not touch Team frames.

- bundle:09 `shell-architecture.test.mjs:11` and bundle:01 `ds-hex-ratchet.test.mjs:88`: already fixed by `98b265b46` (no `Intl`/`toLocale*` in Team.tsx; 0 hex literals in team.css). Static grep only.
- Mobile overflow (`parity-team.mjs:238`): every failing frame renders the two-store selector `Team.tsx:43`. The CI run was on `3eb0e63a`, before `98b265b46` let the select shrink. This relay also makes `.team-store` (team.css:32-33) wrap and gives it `min-width: 0`. NOT_RUN locally (CI only).
- Invite still submits to the paused view (`Team.tsx:148`) and admin Continue stays disabled (`:89,161`).

### Needs separate reviewed oracle correction (not edited here)

1. `tests/admin/parity-team.mjs:214` with `tests/admin/parity-team-frames.json:467,665`: the nav frame requires the old module-shell text `設定與員工` inside `[data-ui-shell]`. The approved shared shell shows `設定` > `員工`.
2. `tests/admin/parity-team.mjs:217-224`: `content()` requires a `button` role for every matching frame string, including the headings `邀請員工` (invite entry), `變更角色` (role editor/review) and `待接受邀請` (empty pending).

Suite SHA-256 is unchanged: `7ff876032ca732452330e152eed4513d45cc7b504eb4920ef9656d7e5e8d18c6`.

## RELAY 4 — acceptance conflict revalidated; no push (2026-11-01)

Fetched origin and merged origin/r3/integration first: exit 0, already current at `c52b65283a5ac738d0c527a464d0cccc9523b1f4`. Entry/local head `4fad602dbce4139b568502780fd28de7bc7eafc6`; PR #476 head remains `3eb0e63a0d34dd1e351cf2b09961e170e240bb15`. Existing fix `98b265b46` remains unpushed. No additional product fix attempt, commit or CI push.

Fresh `gh pr checks 476`, workflow headSha/conclusion and filtered raw failed-job reads confirm E3 failure evidence on that PR head: Gates `41289265846` / `124798201364` still requires SMTP invitations >=4 (`browser_staff_team_test.go:217`); `124798201256` still evaluates deferred invitation/admin frames. Foundation `41289265756` / `124796913759` remains failed on the pre-fix formatter. This is an implementation push failure, not the original tests-only RED. Original causal RED remains `41279053547` / `124769729983`, missing 員工名單 at parity-team.mjs:171.

Current source retains the explicit rulings: `Team.tsx:148` submits only to the paused view; `:89,161` reject/disable admin promotion. Preserved test `parity-team.mjs:179-185` cannot enter the admin confirmation/result states that its content oracle still demands. Foundation staff/OC tests require successful mail issuance/replay. No implementation can satisfy those assertions while preserving both rulings. STOP under the outside-card gate rule; integrator must reconcile acceptance scope. No test/fixture/threshold/exemption, backend, migration or contract edit. Suite SHA-256 stays `7ff876032ca732452330e152eed4513d45cc7b504eb4920ef9656d7e5e8d18c6`.

E1: browser ownership generation then --check exit 0. This relay changes only this report and PR-BODY.md; inherited untracked files remain. No new outside-card source path. Existing shared i18n scope disclosure and DEFERRED table remain applicable. Humaux is available this relay; locked subordinate store/canvas updates are recorded there. No local test/server/background task. Own PASS = E0. NOT_RUN: GREEN for local fixes, successful click/persistence/parity/density acceptance, independent review, SANDBOX, LIVE. WAIT_RUN: none (no push).

## RELAY 3 — current dispatch cannot clear the preserved gates (2026-11-01)

This section is the current handoff; earlier sections are historical. No new product fix attempt or CI push in this relay. `git fetch origin r3/integration` and `git merge --no-edit origin/r3/integration` both exit 0 (already up to date at `c52b65283a5ac738d0c527a464d0cccc9523b1f4`). Entry head is `887dbc885ef49f5fb585a81dfc2c4448634fb81d`; PR #476 remote head remains `3eb0e63a0d34dd1e351cf2b09961e170e240bb15`. The existing unpushed fix `98b265b465791f76f1415f04b29fcb933909555e` is not CI-verified.

Authority: this dispatch retains the invitation pause, forbids admin confirmation/result flows and forbids test weakening. Current source enforces these rulings (`Team.tsx:89,148-161`). The original RED suite still demands mutually incompatible outcomes:

| Exact-head E3 failure receipt | Preserved requirement / conflict |
|---|---|
| Gates `41289265846`, job `124798201364` | `browser_oc_test.go:259`: invitation replay >=3. `browser_staff_team_test.go:217`: SMTP mail >=4. Both require the send path this UI card must keep paused; backend/outbox changes are expressly forbidden. |
| Gates `41289265846`, job `124798201256` | `parity-team.mjs:179-185` refuses an admin transition, while `content()` still requires the unreachable admin confirmation/result frames. The same helper infers buttons from signed headings at lines 217-224. No inert duplicate controls or fake result screens were added. |
| Foundation `41289265756`, job `124796913759` | `Team.tsx: format 1 > 0`; already addressed in the unpushed fix. This old failure does not verify that fix. |

Fresh workflow metadata confirms both failed runs have headSha `3eb0e63a0d34dd1e351cf2b09961e170e240bb15`. Read `gh pr checks 476` and filtered `gh run view --log-failed --job` for those three jobs. Causal tests-only RED remains Gates `41279053547`, job `124769729983`, missing signed 員工名單 at `parity-team.mjs:171`. No new RED/GREEN claim. Suite SHA-256 remains `7ff876032ca732452330e152eed4513d45cc7b504eb4920ef9656d7e5e8d18c6`.

STOP under the outside-card gate rule. Repeating the two product rulings does not reconcile these preserved assertions. Integrator must supply acceptance-scope reconciliation consistent with the pause and disabled Continue; this lane must not send mail, change backend/outbox, skip tests or manufacture success. DEFERRED table below and PR body remain applicable.

E1: browser ownership regenerated, then `--check`, both exit 0, no generated diff. No local tests/server or dependency install. Humaux/code-index tools are unavailable in this runtime; memory store, coord/canvas and indexing are NOT_RUN, not represented as persisted. Entry untracked files preserved. This relay changes only this report and the card PR body. Own PASS = E0. NOT_RUN: GREEN for local fixes, successful click/persistence/parity/density acceptance, independent review, SANDBOX, LIVE. WAIT_RUN: none (STOP before push; integrator owns reconciliation/re-dispatch). No local background process started.

## RELAY 2 — verified stop (2026-11-01)

Re-read the current dispatch, both original cards, AGENTS.md and architecture §§0,1,2,26,27. `git fetch origin r3/integration unit/parity-team` and `git merge --no-edit origin/r3/integration` exit 0; already current at integration `c52b65283a5ac738d0c527a464d0cccc9523b1f4`. Remote PR head is still `3eb0e63a0d34dd1e351cf2b09961e170e240bb15`. Existing local implementation fix is `98b265b465791f76f1415f04b29fcb933909555e`; it has not been pushed or CI-verified. No new product/test change or fix attempt in this relay.

Fresh `gh pr checks 476` and raw failed-job reads confirm the following on that exact remote head (both workflow metadata headSha values checked):

- Gates `41289265846`, browser job `124798201256`: list `1468:74273`, pending `1468:74492` and empty `1468:74559` pass. Admin confirmation/result frames `1468:78293`, `1468:78348`, `1468:78333`, `1468:78392` still fail, although this ruling explicitly defers them. The invite-entry assertion also demands a button named `邀請員工` where the signed entry has a heading (`tests/admin/parity-team.mjs:217-224`).
- The same browser job requires module-shell text `設定與員工`; the original card explicitly authorizes the approved shared shell. Its source/content oracle has not been changed.
- Gates `41289265846`, bundle:08 job `124798201364`: `TestBrowserStaffTeam`, `browser_staff_team_test.go:217`, fails with `invitation mails received by the real SMTP server: 1, want >= 4`; `TestBrowserOC` also fails. The no-send ruling cannot satisfy the invitation issuance/replay/mail assertions. No SMTP action was attempted here.
- Foundation `41289265756`, job `124796913759`: `Team.tsx: format 1 > 0`. This and the CSS/mobile fixes are already in the unpushed local fix described below; no current GREEN receipt exists.

STOP under the dispatch's outside-card gate rule. Integrator must reconcile preserved acceptance scope with the two explicit rulings and shared-shell exception. No test deletion, skip, fixture/oracle/threshold change, inert duplicate control, enabling admin promotion, backend/outbox change, or mail send was used to pass. The immutable suite SHA-256 is still `7ff876032ca732452330e152eed4513d45cc7b504eb4920ef9656d7e5e8d18c6`.

Browser ownership regenerated, then `--check`, both exit 0; no working-tree change. No local tests/server, dependency installation or new CI. Inherited untracked files retained. This relay only updates this report; existing outside-path disclosure and DEFERRED table below remain applicable. E3 = exact-head observed CI failure only; E1 = static ownership structure only; own PASS = E0. NOT_RUN: GREEN for the local fix, successful click/persistence/parity/density acceptance, independent review, SANDBOX, LIVE. WAIT_RUN: none; integrator owns reconciliation/re-dispatch, no local background task remains.

## RELAY 1 — current fix batch (2026-11-01)

This section supersedes the CI-pending and unavailable-tool statements below; the original ROUND 3 record is retained as history. Current remote head inspected: `3eb0e63a0d34dd1e351cf2b09961e170e240bb15`. Merged `origin/r3/integration` first, checkpoint `d60545a3870aef3dec814414f2fb2fb2241f72f1`. Existing untracked FIX1, reference PNG and r3-ci-handoff were present on entry and preserved. No delegation or new task claim; this is the integrator's assigned lane.

### CI failures and targeted fixes (attempt 1)

| Exact-head receipt | Root cause / action |
|---|---|
| Foundation `41289265756` / `124796913759`; Gates `41289265846` / `124798201506` (bundle:09) | `Team.tsx: format 1 > 0`: use existing `instantToTaipei` from the shared format package and slice its stable wall-clock representation; remove component Intl. No allowance edit. |
| Gates `41289265846` / `124798201299` (bundle:01) | 13 literal hex colors in new team.css: use existing DS semantic tokens, no literal replacements in another notation, no baseline/token/exemption change. |
| Gates `41289265846` / `124798201256` (admin parity) | Mobile viewport overflow with long authorized-store names: allow Team's select to shrink below min-content. Keep facts and full option text; no overflow-hiding patch. Role review cancel returns to the role editor with its selection retained. |
| Gates `41289265846` / `124798201364` (bundle:08) | OC and staff-team browser journeys require invitation issuance/re-auth replay and >=4 SMTP messages. These conflict with the explicit retained pause; no invitation send or test rewrite. |

### Remaining acceptance conflicts — no bypass

1. Deferred invitation sends/acceptances and role-confirm-admin/role-done-admin are still required by unchanged RED assertions. The signed disabled Continue and no-mail rulings remain enforced. Existing OC/staff browser tests also assume the removed always-open invite form/select and a successful send. Changing setup selectors alone cannot satisfy their send/replay/SMTP assertions under the ruling.
2. `tests/admin/parity-team.mjs:217-224` infers a button for every matching string, even headings: the invite entry's heading `邀請員工`, role editor/review heading `變更角色`, and empty-pending heading `待接受邀請` are required as buttons. The signed frames instead use headings in these states. CI shows the correct heading/form/radios while failing the inferred button locator. No duplicate inert button was added to pass.
3. `parity-team.mjs:214` requires the old module shell's `設定與員工`, while the owner-approved shared shell uses `設定`. Card common rules explicitly authorize the shared-shell substitution. No shared shell or frozen oracle was edited.

Integrator reconciliation of these assertions is required before this PR can be GREEN. The lane preserves them exactly (SHA-256 `7ff876032ca732452330e152eed4513d45cc7b504eb4920ef9656d7e5e8d18c6` before/after this fix). No new tests, exemptions, fixture changes, skips or threshold changes. `rg -l 'Team\\.tsx' tests/` found only a descriptive comment in staff-team.spec.ts, no component compilation harness needing a new import adapter.

### Static checks and handoff

- E1: `node --check tests/admin/parity-team.mjs`, `python3 scripts/check_packet.py` (structure only), depmap regeneration and browser ownership generation/`--check`: exit 0. Packet check changed only its generated timestamp; restored that unrelated artifact. Depmap and ownership have no new diff after the integration merge.
- Local tsc NOT_VERIFIED: first compiler launch lacked local node_modules; two dependency-resolution attempts with temporary read-only shared dependency links ended with missing/outdated workspace exports (exit 1). Stopped rather than changing manifests/lockfiles or other modules; both links removed. CI is the type-check authority. No local test/server or app build was run.
- Humaux search/rules/rejected/map/source lookup and locked subordinate canvas are available in this relay. Stored `[fix] PARITY-TEAM #476 RELAY1 local fix batch; preserved assertion/ruling conflict STOP before push` (recalled id `19bd3769-bec2-4af1-b1e4-1683c7995005`); Team source submitted to code index. No platform/local memory read or write. Runtime model/effort identifiers remain unexposed.
- Raw CI logs retained locally under ignored `output/parity-team/relay2-failures/`, with authoritative run/job pointers above. No bulk log commit. No local process remains; CI monitoring returns to the integrator via WAIT_RUN.
- New source changes are only `apps/admin/components/Team.tsx` and `team.css`. Existing shared copy path `packages/i18n/src/team-parity.ts` still needs integrator review as disclosed below. No backend, contract, migration, lockfile, immutable gate file or other-app edit.
- GREEN, mobile click/persistence confirmation, independent review, density/screenshot comparison, SANDBOX and LIVE remain NOT_RUN for this fix. Own PASS = E0; previous CI failures are E3 evidence of failure only.
- STOP before another push under the card's gate/scope rule. Local source fix and report are committed for integrator review; remote PR head stays `3eb0e63a`. WAIT_RUN: none (no new CI). Do not repeat the generic relay until the acceptance conflict is reconciled; no request to authorize sending invitations or enabling admin promotion.

---

Date: 2026-11-01. PR #476, `unit/parity-team` → `r3/integration`.
Role: integrator-dispatched UI implementation worker. Runtime identity: Codex/GPT-6; exact configured model/effort is not exposed. No delegation.
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/parity-team` only.
Merged base: `87c67e33c329a43bb6fc0d51d9160ac4c9826b98`; local merge checkpoint: `3f00f1fe2ab5e86bfa3b09586f6032295f49d7af`.

## Authority and trail

- Human: ROUND 3 dispatch, two explicit rulings in this session. They supersede ROUND 2's pause-lift/outbox and no-op-role blockers.
- Signed copy: `output/parity-team/FRAME-COPY.md`, SHA-256 `fd1ff3d7c2f1eceb2a6aec157886925fd5bf31e2ea307a0e79f70bc97fee55c0`.
- Signed source: read-only ux-team DELIVERY, current 164-frame section, SPEC + three-language table; no other worktree was written.
- Read AGENTS.md, architecture §§0,1,2,26,27, original card, identity Q-TEAM contract, existing Team DTO/client/BFF and owner limits. New code uses only those DTOs/commands.
- Humaux and code-index tools are absent from this runtime. Targeted tool discovery returned no search/get/coord/canvas/store/code-index tools. Shared task/store/canvas/index/link writes are NOT_RUN; this report is a local handoff, not a substitute claimed as Humaux persistence.
- No production action, server, local test, invitation send, migration/contract/backend edit, merge to integration, force-push or auto-merge.

## Product behavior

- Signed member/pending tabs, identity-first cards, role/join/owner protection, email-derived label and null-email fallback. Five described role radios are the permissions view; server authorization remains authoritative.
- Review → submit once → result for reachable role changes, member removal and invitation withdrawal. Successful commands re-read server state; reload never replays a mutation.
- Unknown result → one read-only reconciliation → confirmed result or unresolved notice; no resubmit button. Identity/scope state unmounts when the existing guarded read closes.
- Invitation entry retains five role choices and email. Submit only opens signed unavailable copy; it never calls inviteMember or creates a success receipt. Q-TEAM pause and all backend entry points are unchanged.
- Continue is disabled for admin and unchanged roles; the unchanged-admin signed copy is shown. No admin confirmation/result route is built. Additive CI assertions cover promotion from viewer as well as the existing unchanged-admin assertions.
- zh-TW, zh-CN and en signed copy resides in `packages/i18n/src/team-parity.ts`. Legacy copy exports remain intact for existing mail-label contract tests.

## DEFERRED

| State | Desktop / mobile | Reason |
|---|---|---|
| invite-sent | `1468:75217` / `1468:75260` | staff invite outbox: separate backend card |
| invalid-email | `1468:75278` / `1468:75326` | staff invite outbox: separate backend card |
| already-member | `1468:75349` / `1468:75393` | staff invite outbox: separate backend card |
| invite-limit | `1468:75412` / `1468:75456` | staff invite outbox: separate backend card |
| owner-disabled | `1468:75475` / `1468:75519` | staff invite outbox: separate backend card |
| reauth | `1468:75538` / `1468:75582` | staff invite outbox: separate backend card |
| invite-checking | `1468:76405` / `1468:76453` | staff invite outbox: separate backend card |
| checking-unresolved | `1468:76689` / `1468:76734` | staff invite outbox: separate backend card |
| link | `1468:76880` / `1468:76892` | staff invite outbox: separate backend card |
| signin | `1468:76904` / `1468:76925` | staff invite outbox: separate backend card |
| signup | `1468:76946` / `1468:76968` | staff invite outbox: separate backend card |
| verify-signin | `1468:76990` / `1468:77006` | staff invite outbox: separate backend card |
| verify-signup | `1468:77022` / `1468:77038` | staff invite outbox: separate backend card |
| accept | `1468:77054` / `1468:77066` | staff invite outbox: separate backend card |
| accept-checking | `1468:77078` / `1468:77097` | staff invite outbox: separate backend card |
| joined | `1468:77116` / `1468:77134` | staff invite outbox: separate backend card |
| expired | `1468:77152` / `1468:77170` | staff invite outbox: separate backend card |
| revoked | `1468:77188` / `1468:77206` | staff invite outbox: separate backend card |
| used | `1468:77224` / `1468:77242` | staff invite outbox: separate backend card |
| wrong-account | `1468:77260` / `1468:77278` | staff invite outbox: separate backend card |
| invitee-member | `1468:77296` / `1468:77314` | staff invite outbox: separate backend card |
| invitee-off | `1468:77332` / `1468:77350` | staff invite outbox: separate backend card |
| invite-sent-owner | `1468:77368` / `1468:77411` | staff invite outbox: separate backend card |
| invite-sent-live | `1468:77496` / `1468:77539` | staff invite outbox: separate backend card |
| invite-sent-fulfilment | `1468:77624` / `1468:77667` | staff invite outbox: separate backend card |
| invite-sent-viewer | `1468:77752` / `1468:77795` | staff invite outbox: separate backend card |
| mail-failed send/history journey (unavailable content implemented) | `1468:75601` / `1468:75647` | staff invite outbox: separate backend card |
| role-confirm-admin | `1468:78293` / `1468:78333` | signed frame disables Continue |
| role-done-admin | `1468:78348` / `1468:78392` | signed frame disables Continue |
| reauth-signin | `1468:96313` / `1468:96358` | staff invite outbox: separate backend card |
| reauth-verify | `1468:96378` / `1468:96420` | staff invite outbox: separate backend card |

Entry frames `invite-owner/admin/live/fulfilment/viewer`, and the unavailable `mail-failed`/`password-off` content, are implemented. Existing pending invitations can still be read/withdrawn; that does not issue mail. Deferred frames remain in the frozen RED suite with all assertions intact; no skipped tests, oracle edits, threshold changes or exemption additions.

## Acceptance conflict requiring integrator reconciliation

The preserved tests `tests/admin/parity-team.mjs` still require invite writes, successful send/accept journeys, and reaching role-confirm-admin/role-done-admin after disabled Continue. These cannot pass while obeying this ruling. They were NOT rewritten, skipped or weakened. The two module-shell nav expectations also remain unchanged under the shared-shell exception. Do not classify these deferred paths as passed parity. Integrator must reconcile signed test scope separately; this lane cannot fabricate transitions or send mail for GREEN.

## Evidence

- RED (E3, failure only): head `89452970349f92219e1b1217868611f1a73057fe`; Gates run `41279053547`, browser job `124769729983`, missing signed 員工名單 at parity-team.mjs:171. Build and Foundation `41279053499` / `124767027990` passed on that head. Current source is not validated by these old receipts.
- E1 static: TypeScript strict `node node_modules/typescript/bin/tsc --project apps/admin/tsconfig.json --noEmit --incremental false`, exit 0; `node --check tests/admin/parity-team.mjs`, exit 0; `python3 scripts/check_packet.py`, exit 0 (structure only); depmap generation exit 0, no diff; browser ownership generation + --check exit 0.
- Tests-only RED assertions are preserved verbatim and in order. New ruling assertions are additive and await their first CI execution; no separate causal RED claim for these additions.
- CI supplies per-frame screenshots, SHA-bound manifest and real-click ledger. No local browser/screenshot/density parity or successful persistence result is claimed. Runtime response fixtures are MOCK, not Go/PG/provider acceptance.
- GREEN/independent review/density measurement/successful click acceptance: pending CI. SANDBOX/LIVE: NOT_RUN. Own PASS would be E0.

## Paths outside the original dedicated team surface

- `packages/i18n/src/team-parity.ts`: required shared three-locale copy module, consumed only by Team through team-copy; listed for integrator approval.
- `apps/admin/components/team.css`: scoped Team styles within the owned app/module (scope pre-approved).
- Generated `output/ci-select-backend-browser/browser-ownership.json`: expressly authorized. Depmap regenerated with no change.
- No migration, contract, OpenAPI, go.mod/sum, lockfile, baseline, dispatcher, R2 pin or other application/backend source change.

## Handoff

One implementation push follows the already observed tests-only RED; no evidence-only commit after CI. Gates/Foundation run IDs will be recorded in a PR comment once registered, and returned as WAIT_RUN. CI queue ownership returns to the integrator for re-dispatch, with no unattended local process.

STATUS: BLOCKED_ASSERTION_RULING_CONFLICT_RELAY4_NO_PUSH
