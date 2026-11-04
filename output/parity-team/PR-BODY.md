PARITY-TEAM: the staff page (`/{locale}/team`) and the invitee page (`/{locale}/invite/[token]`) built to the signed Figma frames, over the existing team DTOs and commands. Frontend and browser tests only: no backend, contract, migration or lockfile change.

## What the code does now (owner decisions 2026-11-04)

- **Invite restored** (owner lifted the invitation pause): 邀請員工 → email + role → 送出邀請 calls the existing invite API (`Team.tsx:104-115`). SENT / FAILED mail are server facts; an UNKNOWN mail state or a lost answer only re-reads the list (取得最新狀態) and is never resent.
- **Admin promotion** (owner: build to the design): Continue is disabled only when the role is unchanged (`Team.tsx:190`). Promotion goes through set-role; the review screen restates the signed admin powers line (`Team.tsx:196`).
- **Invitee page** (`TeamInvite.tsx`): signed states over the existing accept API. 200 → joined; `invite_invalid`/`unauthorized` → one generic 邀請無法使用; `already_member` → member; `not_found` → off; anything else → 確認中 with no accept control. Sign-in / sign-up use the shared identity pages; the invite rides in the `#next=` fragment, never a query, Referer or log.
- **Owner-only management** comes from the server read (`my_role === "owner"`), never from client state or the URL.

## Superseded frames (integrator, owner-delegated 2026-11-04)

Exactly 9 states / 18 ids, each with its reason in `tests/admin/parity-team-frames.json` `superseded` and pinned by `tests/admin/parity-team.mjs:483-494`. All other frames of the 82-frame / 164-id oracle are required. Details and the Figma follow-up list: `output/parity-team/DESIGN-DECISIONS.md`.

| Decision | States superseded |
|---|---|
| Shared ReauthDialog (OC-04: password step-up, exact replay) replaces in-page re-auth | reauth, reauth-signin, reauth-verify |
| No-op admin change is blocked at role-edit; no confirm/done for an unsent request | role-confirm-admin, role-done-admin |
| Shared identity sign-in/sign-up/verify pages replace invite-local auth frames | signin, signup, verify-signin, verify-signup |

Shell nav: the shared breadcrumb 設定 > 員工 replaces the stale module label 設定與員工 (frame note at `parity-team-frames.json:703`).

No state is DEFERRED any more: invite-sent, the invitee states, role-edit-admin and the generic role review are required and gated.

## Tests

- `tests/admin/parity-team.mjs`: real-click signed parity for all required frames, 3 locales × 2 viewports, plus R3 invite send, admin promotion and the invite sign-in round-trip.
- `tests/admin/staff-team.spec.ts`, `tests/admin/oc/reauth.spec.ts`: locators ported to the signed layout; status, token-hygiene, SMTP capture and reuse-refusal assertions unchanged. On the SENT screen the replay's starting control (送出邀請) unmounts, so focus is asserted on that screen's only action; the dialog's focus-return to a still-mounted control is a separate follow-up (review P2).

## Evidence

- CI: all checks green at `652face84` (45 pass, 1 skipping).
- Independent review: Opus 5.5 APPROVE at `652face84` (`output/issue-107/review-476-652face84.md`), P0/P1 none. Codex review pending.
- SANDBOX / LIVE: NOT_RUN. No production action.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
