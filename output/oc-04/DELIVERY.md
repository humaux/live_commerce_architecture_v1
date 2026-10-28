# OC-04 re-auth dialog and reauth_required handling: delivery

## Directed cookie-fixture fix (2026-10-28, attempt 1 of 2)

- Dispatched by Claude integrator to Codex; base `36b807952ca57ba651ba0bee15d214e881c4be72`, branch
  `unit/oc04-reauth-dialog`, worktree `/Volumes/data/live_commerce_architecture_v1/.claude/worktrees/oc04`.
  Runtime model/effort identifiers are not exposed. No delegation. Authorized source path: `tests/admin/oc/reauth.spec.ts`.
- Blocking CI: Gates run `40626807342`, job `122653477002` (`gate (--browser-password-auth)`, failure).
  Integrator reports the zh-TW step 7 session-cookie lookup throws at line 265; the job API confirms TestBrowserOC failed.
- Root cause: `browserFront` uses HTTP loopback for Chromium, which accepts and sends Secure `__Host-` cookies there;
  Playwright's HTTP URL cookie filter omits those stored cookies. The step 6 snapshots could therefore both be empty.
- Fix: reuse `password-auth.spec.ts`'s unfiltered cookie-jar read plus exact public-host match, assert one session cookie both
  before and after OIDC re-auth, retain the existing unchanged-cookie and callback assertions, and read the validated session
  for step 7. The step 7/8 OIDC marker fixture uses the existing admin specs' HTTPS cookie URL (same hostname, no navigation).
  Product authentication code and all step 7/8 routing, mail-hint, cancel and signed-in assertions are unchanged.
- Static checks: `node --check --experimental-strip-types tests/admin/oc/reauth.spec.ts` exit 0;
  `./node_modules/.bin/tsc --noEmit --strict --skipLibCheck --target ES2023 --module NodeNext --moduleResolution NodeNext
  --typeRoots apps/admin/node_modules/@types --types node tests/admin/oc/reauth.spec.ts` exit 0.
  The initial tsc invocation without the installed admin type root exited 1 (TS2688, missing Node type lookup).
- Evidence: E1 syntax/type structure only; no local tests or servers. CI GREEN and independent review PENDING;
  exact pushed head and WAIT_RUN IDs will be recorded in a PR comment, with no evidence-only push.
- Outside card write_paths: none. Humaux is unavailable in this runtime; memory, canvas and coordination updates were not possible.

- Role: ui_worker (Claude Opus 5.5, subagent of the integrator, no recursive delegation). Branch `unit/oc04-reauth-dialog`, worktree
  `.claude/worktrees/oc04`, base `r3/integration` f935508ba (OC-15 #229 merged). PR #246 into r3/integration; no auto-merge, not merged.
- Authority: contracts/tasks.json OC-04 (acceptance, write_paths); contracts/merchant-password-auth-v1.md §11, §16, §16.1;
  output/oc-15/DELIVERY.md; issue #244 (items owned by OC-04).
- Evidence: CI only. Locally only tsc (touched files), `go vet -tags browser ./tests/foundation`, gofmt, `node scripts/dev/check-ds-rules.mjs`
  (0 violations). The author's PASS is E0; independent review NOT_RUN.
- RED: Gates run 40588086164, job 122493869233 (gate --browser-password-auth), head 3d850bcb1 (tests only): TestBrowserOC fails on assertions (no dialog, re-auth routes never called, 0 replayed invitations); TestBrowserPasswordAuth and TestBrowserStaffTeam pass in the same job (`red.log`). GREEN: run and job ids are posted as a PR comment (a commit cannot carry the run that tests it).
- No migration (R2 pin 134 untouched).

## What changed

- `apps/admin/components/ReauthDialog.tsx` (new, `@ds-page`): the one handler of 403 `reauth_required`. Mounted once in the locale
  layout; it wraps `window.fetch` for same-origin `/api/` calls (not `/api/auth/*`). A refused call waits for the dialog; on success the
  original request (the caller's own fetch arguments: method, URL, headers including `Idempotency-Key`, body) is sent again exactly once
  and the caller gets that answer; on cancel the caller gets the original 403. No logout. Concurrent refusals share one dialog. If the
  caller's own timeout ran out during the dialog, the replay gets a fresh 16 s timeout; an explicit abort still applies.
  UI only from DS: `ConfirmDialog` (focus trap, Escape, busy lock), `Field`/`TextInput`, `Button`, `Alert`, `Stack`; no colours or sizes of
  its own (DS controls are 44 px below 768 px). Focus starts in the first field and returns to the page control that started the request.
  Password route: password -> emailed code -> complete. OIDC route: a popup to the IdP; the callback returns the popup to the opening page
  with `#reauth-done` / `#reauth-failed`, posted to the opener over BroadcastChannel.
- Fix 1 of 2 for a third blocker (Gates job 122530020662, `--browser-live-console` labels: `request.postData()` was null): the first
  send had been rebuilt as `new Request(...)` plus `clone()`, which turns the body into a stream the browser tooling cannot read. The
  first send is now the caller's own `fetch(input, init)` untouched; the replay reuses the same arguments (a Request input is cloned).
- Fix 1 of 2 for a fourth blocker (Gates job 122546542318, spec step 5: the password field was not focused when the dialog opened from a
  script-started request): `ConfirmDialog` focused its first input in the body's effect, which runs before the parent's `showModal()`;
  it now also focuses the first input right after `showModal()`. That alone did not clear it (Gates job 122566399452, same step:
  the field lost focus during the 8.5 s wait), so fix 2 of 2: while the re-auth dialog is open, focus settles on its first field on
  open and on every step, and returns there whenever it falls out of the dialog to nothing.
- `apps/admin/lib/reauth-client.ts` (new, integrator grant): the two BFF calls of the dialog (`reauthMethod`, `reauthStep`), so the
  component calls no `fetch` (G-UI3/G-UI5, tests/admin/shell-architecture.test.mjs). Fix 1 of 2 for that blocker (Foundation job
  122523457766); no allowance added, test untouched.
- `apps/admin/lib/reauth-copy.ts` (new): zh-TW, zh-CN, en. The §11 mail_unavailable OIDC hint is a separate string shown only when OIDC
  is configured and the account has an OIDC identity.
- `apps/admin/app/api/auth/password/reauth/[step]/route.ts` (new): `GET method` ({password, oidc, account}); `POST start`, `complete`
  (-> Go password re-auth, §16.1), `oidc` (-> Go `login/reauth/start`, sets the login cookie marked as a re-auth flow with the return path).
  Per-step status/code allowlists from §16.1; only an upstream 401 clears cookies (403 reauth_failed and 422 invalid_code keep the session).
- `apps/admin/app/api/auth/callback/route.ts`: on `reauth: true` it does not call `setSessionCookies`, sets and clears no session/CSRF
  cookie, and redirects to the originating page. A refused or tokenful answer to a re-auth cookie never becomes a new session. An ordinary
  OIDC login additionally sets `__Host-commerce_oidc_session` = HMAC-SHA256(fixed label "commerce-oidc-session-v1", session token), HttpOnly: the hint the dialog uses to offer the
  OIDC route to an OIDC account (PD9: password XOR OIDC). A stale marker never matches another session.
- `apps/admin/lib/auth.ts`: `reauthReturnPath`, `setOIDCHint`, `oidcSession`, `OIDC_HINT_COOKIE`; `setLoginCookie`/`loginBinding`
  carry the optional `reauth` path; messages for `reauth_required`, `reauth_failed`, `owner_role_disabled`. `setSessionCookies` and
  `clearAuthCookies` are unchanged (their two-cookie tests stay as they are).
- BFF code allowlists: `apps/admin/lib/team-model.ts` `teamCodes` gains `reauth_required`, `owner_role_disabled`, `reauth_failed` (before:
  rewritten to 503 retry_later). The other guarded BFF routes already pass any well-formed code through `safeError` and were not changed:
  `apps/admin/app/api/stores/[store]/[...resource]/route.ts` (orders CSV, finance CSV, customer erasure/export, bank-transfer decisions,
  refunds, payments/card, billing, ads), `apps/admin/app/api/stores/[store]/reports/[report]/route.ts`.
- `apps/admin/lib/transfer-copy.ts`: `bank_transfer_closed` in en, zh-CN, zh-TW (grant; #244).
- Fix 1 of 2 (Foundation job 122520630945, contractdrift `TestBFFLiveSettingsRealLeaves` / `TestBFFRealSourceMutation`: 12 UNRESOLVED):
  the scanner allows only `createHmac`/`randomBytes`/`timingSafeEqual` from node:crypto, and reads every `/auth/password/` leaf through
  `handleStep1`/`handleVerify` with literal `privateIdentity` paths. The marker is now an HMAC with a fixed label (the spec computes the same),
  the re-auth POST handler is named `handleVerify`, and the three upstream paths are literals. The scanner and its baseline are untouched.
- Tests: `tests/admin/oc/reauth.spec.ts`, `tests/foundation/browser_oc_test.go`, `playwright.config.ts` suite `oc` (was not registered).

## Outside write_paths (integrator approval needed)

- `apps/admin/app/[locale]/layout.tsx`: one import and `<ReauthDialog locale={locale} />` (the mount point).
- `packages/ui/src/ds/Dialog.tsx`: `ConfirmDialog` gains an optional `children` slot (rendered in the body) and focuses the first input
  in the body instead of only the typed-confirm input. Additive; existing callers unchanged. Needed because the DS dialog had no slot for
  a password/code field.
- `scripts/dev/test-local.sh` (`--browser-password-auth`): `TestBrowserOC` added to the `-run` regex and two no-test guards in `lc_prepare`,
  so CI runs the new gate. No other mode, receipt or gate assertion changed.

## Acceptance mapping

| Card item | Where asserted |
| --- | --- |
| 403 reauth_required opens the dialog, no logout | spec: invite chains (dialog visible, URL unchanged, `/api/stores` 200) |
| Replay once with the original idempotency key | spec step 5 (two requests, same `Idempotency-Key` and body, caller gets the replay's status); invite chains (two requests, same body); Go: every invited address exactly once in PG |
| BFF allowlists pass the three codes, one 403 assertion per guarded route | spec step 1 (team, catch-all x4, reports), step 2 (reauth_failed), step 4 (owner_role_disabled) |
| Suite `oc` + browser_oc_test.go in CI | playwright.config.ts, test-local.sh `--browser-password-auth` |
| OIDC callback reauth:true: no setSessionCookies, cookie unchanged, back to the originating page | spec step 6 (callback Set-Cookie, Location, cookie jar before/after) |
| Password -> code -> complete; OIDC-only account gets the OIDC route | spec invite chains; step 7 (`account` from the marker) and step 8 (dialog opens on the OIDC step) |
| §11 copy incl. mail_unavailable OIDC hint only for OIDC accounts | spec step 8 (MOCK 503, with and without the marker) |
| 3 locales, focus trap / return, 44 px | zh-TW desktop, zh-CN desktop, en 390 px; Tab loop; focus on the send button after the replay; height >= 44 |

## NOT_RUN / limits

- OIDC re-auth against a real IdP: MOCK only (harness answers `login/reauth/start` and the re-auth `login/complete`); the SQL side is OC-02 RA04/RA07.
- `mail_unavailable` from a real SMTP outage: MOCK 503 in the browser only.
- billing:manage still answers 403 `forbidden` (OC-15b, #244), so no dialog there until that lands; the BFF passes the code through once it does.
- payments/card, refunds, customer export and Meta ads publish: no browser assertion here (same catch-all `safeError` pass-through as the asserted erasure/CSV; Go side covered by OC-15 RH13 or NOT_RUN there).
- Under PD9 a password account never has an OIDC identity, so the mail_unavailable OIDC hint is only reachable for an OIDC account that switches to the password route (which Go then refuses). It is implemented as the contract states.
- Team page copy (`apps/admin/lib/team-copy.ts`, not in write_paths) has no text for `owner_role_disabled` / `reauth_failed`; the page shows its generic error for them. Follow-up for the Team card.
- If the browser blocks the popup, the dialog says so; the request is not replayed until the person retries.
- WebKit: NOT_RUN (`--browser-webkit` does not include suite `oc`).
- Independent review: NOT_RUN.
