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

## PR #246 CSV review fix: RED handoff (2026-10-29, attempt 1)

- Dispatcher: Claude integrator; executor: Codex, runtime model/effort identifiers not exposed. No delegation.
  Worktree and branch unchanged; pre-RED base e13c85c67cf08e92611b3b962ff796a1c47b9617.
  `git fetch origin r3/integration` and `git merge origin/r3/integration` exit 0 (already up to date).
- Authority: current owner repair card, contracts/tasks.json OC-04, merchant-password-auth-v1.md §16/§16.1,
  and integrator review log cxf-rv-246.log (last answer). Root finding: native download navigation bypasses window.fetch.
- Tests-only commit: 51e81e7d6d48e6aea299cc49d80f2f11720dcfb0. Six independent cases use the actual orders/finance
  controls in all three locales (en at mobile width), require a real 403, dialog, password + mailed code, exactly one
  replay with identical URL/query, one completed download, valid CSV header/filename, signed-in state and no re-download on reload.
- E1: `node --check --experimental-strip-types tests/admin/oc/reauth.spec.ts` exit 0;
  `./node_modules/.bin/tsc --noEmit --strict --skipLibCheck --target ES2023 --module NodeNext --moduleResolution NodeNext
  --typeRoots apps/admin/node_modules/@types --types node tests/admin/oc/reauth.spec.ts` exit 0. No local tests/servers.
- WAIT_RUN: Gates 40749708453; Foundation 40749708075 (foundation job 123032808680).
  RED assertion and GREEN PENDING; original green at d8adead27 does not accept this head.
  Integrator relays completion to a fresh session; resume reads failed CI logs, appends raw assertion evidence to red.log,
  then makes the single implementation push. No implementation has been changed in this RED session.
- Outside card write_paths: output/ci-select-backend-browser/browser-ownership.json (generated by the owner's named
  derive-browser-ownership.mjs, exit 0; CI's backend-coverage drift gate hashes the modified spec). Includes the current branch's
  pre-existing ownership drift; source files for those entries were not changed. Integrator approval required.
- Humaux tools absent: no store/canvas/coord calls possible. This file and red.log retain the handoff; no completion claim.

### CSV click ledger (pending CI)

| Pages/locales | Control | User operation | Required visible result | Actual |
| --- | --- | --- | --- | --- |
| orders, zh-TW / zh-CN / en | orders-export | click after session fixture aging | re-auth dialog; complete password/code; one valid orders CSV | PENDING |
| finance, zh-TW / zh-CN / en | finance-csv | click after session fixture aging | re-auth dialog; complete password/code; one valid finance CSV | PENDING |
| both pages, all locales | browser reload | reload after download | signed in; export remains visible; no second download/dialog | PENDING |

No new screen, visible copy, layout or design artifact. Existing UI design/process evidence is unchanged; this repair adds behavioral
acceptance only. NOT_RUN: all local tests/builds/servers, CI RED result, implementation/GREEN, independent re-review, WebKit and LIVE.

## PR #246 continuation: outside-card gate conflict (2026-10-29)

- Source of truth: `gh run view 40749708075 --job 123032808680 --log-failed` (exit 0),
  tests-only head `51e81e7d6d48e6aea299cc49d80f2f11720dcfb0`; filtered original lines appended to `red.log`.
  The relay's description of the actual result was inaccurate: the raw log says
  `actual: [ 'foundation-shards', '--browser-password-auth' ]`, not Foundation alone.
- Existing generated ownership already includes `tests/admin/oc/reauth.spec.ts` in both `specs` and `inputs` of
  `--browser-password-auth`, with registry evidence for the actual harness. The registry invokes `TestBrowserOC`.
  `tests/ci/pr-modes.test.mjs:680-681` nevertheless uses this tracked, registered spec as an unknown-path fixture
  and requires all browser modes. These requirements conflict; changing the suite from a glob to an exact path
  cannot resolve that assertion. No such source change was made.
- STOP under the card's outside-scope gate rule and explicit prohibition on editing CI-SPEED-3 tests.
  Integrator must resolve the contradictory unknown-path fixture in its own authorized scope before this card can pass.
  No implementation, generator, local test, server, commit or push was performed in this continuation.
  `git fetch origin r3/integration` and `git merge origin/r3/integration` both exited 0 (already up to date).
- Gates run `40749708453` still queued/in progress when read; no CSV assertion RED observed. GREEN: NOT_RUN.
  WAIT_RUN: `40749708453` (Gates, not its plan job). Integrator relay is the wake-up path.
- Evidence: raw CI Foundation failure tied to the exact head; the CSV implementation remains unaccepted (author E0).
  Only existing `output/oc-04/DELIVERY.md` and `red.log` were appended; no new outside-card paths.
  Humaux tools absent: memory store, canvas and coordination updates unavailable.

## FIX-RED #246: current-head failure audit (2026-10-29)

- Current PR head independently read from GitHub: `51e81e7d6d48e6aea299cc49d80f2f11720dcfb0`.
  First action in the worktree: `git fetch origin && git merge origin/r3/integration`, exit 0.
  Integration `bf161bac9` is now in local merge `ddfc6dd02` (unpublished, Codex co-author trailer).
  Existing uncommitted delivery/red notes were preserved.
- Read `gh pr checks 246` and invoked `gh run view --log-failed --job` for EVERY failed job.
  Foundation logs returned successfully. Gates archive reads stalled (one returned `unexpected EOF`);
  read all three affected jobs through the GitHub Actions per-job raw logs API instead and stopped the stalled readers.
- Foundation run `40749708075`, job `123032808680`: the sole failing Node assertion treats the registered
  `tests/admin/oc/reauth.spec.ts` as unknown and expects all modes. Actual is `foundation-shards` plus
  `--browser-password-auth`. The conflicting fixture is STILL present in current integration at
  `tests/ci/pr-modes.test.mjs:680`. This is not generated ownership drift.
- Gates run `40749708453`, bundle:08 job `123035494287`: `TestBrowserOC` reaches its 1200-second deadline;
  the harness then reports `context deadline exceeded`. The job log alone does not establish assertion-level
  RED for the six CSV cases. Receipt job `123058993884` fails on the password-auth FAIL receipt;
  required job `123059582522` fails because gate and receipts failed. These are downstream failures.
  The Gates receipt identifies tested synthetic merge `e8667f2fad7d22b9305818c66fbd2415ec4c3d74`.
- Source readback confirms the CSV root finding: MerchantOrders and Finance use native download anchors,
  while ReauthDialog wraps only window.fetch. No speculative implementation was pushed past the STOP condition.
- STOP under the owner's explicit outside-card gate rule: `tests/ci/pr-modes.test.mjs` is a shared CI gate,
  outside the admin module's scope pre-approval. Integrator must resolve its contradictory fixture before
  this card can become green. No gate/test/threshold/fixture/exemption was weakened; no timeout was changed.
- Evidence: current-head CI failures read directly (E3 failure evidence, not completion); local merged head has
  no CI evidence. Author acceptance remains E0. Commands executed locally were reads and the requested Git merge;
  no local tests, builds, servers or generators. New outside-card edits: none; upstream merge changes only.
- Updated only this file and `red.log`. Humaux tools unavailable: no store/canvas/coord update possible.
  GREEN: NOT_RUN. WAIT_RUN: none (no new push; prior runs both concluded failure).
  NOT_RUN: CSV implementation, assertion-level CSV RED confirmation, current merged-head CI,
  independent review, local tests/servers, provider SANDBOX/LIVE. Integrator handoff is the resume path.
- Optional browser artifact `12845468900` did not finish downloading after two minutes; the reader was stopped.
  No artifact contents or assertion failure are claimed. All processes started in this continuation have ended.


## FIX-RED latest relay readback (2026-10-29)

- Independently re-read PR #246 checks for remote head `51e81e7d6d48e6aea299cc49d80f2f11720dcfb0` and all four failed jobs.
  `git fetch origin && git merge origin/r3/integration` exited 0 (already up to date); local unpublished merge remains
  `ddfc6dd02ed99ff2e8633069362e3ed42e86e3f7`, integration `bf161bac9`.
- Original CI results, not author acceptance: Foundation `40749708075/123032808680` fails the unknown-path fixture;
  Gates `40749708453/123035494287` times out TestBrowserOC after 1200.02s;
  `123058993884` and `123059582522` are the dependent receipt/required failures. Gates tested synthetic merge
  `e8667f2fad7d22b9305818c66fbd2415ec4c3d74`. Exact log excerpts remain in red.log.
- Current `origin/r3/integration:tests/ci/pr-modes.test.mjs:679-681` confirms the conflicting fixture is still present.
  No implementation push is permitted before causal CSV RED, and the card requires STOP on this outside-module gate blocker.
  Integrator handoff: correct the unknown-path fixture in the CI owner's scope, then resume CSV assertion RED and implementation.
- This relay changed only these two existing OC-04 evidence files; pre-existing edits retained. No source change, commit or push.
  No new workflow dispatched; WAIT_RUN: NONE. GREEN, causal CSV RED, local tests/servers, independent review and LIVE: NOT_RUN.
- Evidence boundary: raw CI failures and static source inspection only; no completion or author PASS claimed.
  Humaux is unavailable: no memory store, task claim, lock or canvas update could be performed.


## FIX-RED current relay audit (2026-10-29)

- `git fetch origin && git merge origin/r3/integration`: exit 0, already up to date; local HEAD
  `ddfc6dd02ed99ff2e8633069362e3ed42e86e3f7`, integration `bf161bac96252c35337e07713c08a275d48e3e17`.
- Independently read `gh pr checks 246` and all four current-head failed jobs. Foundation log command exited 0;
  each Gates `gh run view --log-failed --job` hit a 45-second read timeout, then its raw per-job logs API exited 0.
  Remote head remains `51e81e7d6d48e6aea299cc49d80f2f11720dcfb0`; run/job evidence is appended to red.log.
- STOP: current integration still tests registered `tests/admin/oc/reauth.spec.ts` as an unknown spec at
  `tests/ci/pr-modes.test.mjs:680-681`. This shared CI gate conflict is outside the admin module and is not
  generated ownership drift. The CI owner must resolve it; this card does not authorize changing that gate.
- `TestBrowserOC` times out after 1200.02 seconds; the logs do not establish assertion-level CSV RED.
  CSV implementation and GREEN remain NOT_RUN. No source edit, commit, push, test, server or generator was run.
  Existing dirty notes were preserved; only these two in-card evidence files were appended. No new outside-card edits.
- Evidence: E3 CI failure receipts only, no completion; author acceptance E0. WAIT_RUN: NONE (no new push).
  All four bounded log readers ended. Integrator handoff is the resume path; Humaux store/canvas/coord unavailable.

## FIX-RED relay: latest integration merged, gate conflict remains (2026-10-29)

- Requested `git fetch origin && git merge origin/r3/integration`: exit 0. Integration
  `313c2c36f4ac064975c1c7c170d710cb63fa457f` merged into local, unpublished
  `b500d58cb0ffa541c4d2c4500aa932201d321b08` (Codex trailer included).
  Existing dirty evidence preserved. Remote PR head independently rechecked:
  `51e81e7d6d48e6aea299cc49d80f2f11720dcfb0`.
- Read `gh pr checks 246` and requested `gh run view --log-failed --job` for all four failed jobs.
  Foundation succeeded directly; Gates reads exceeded 25 seconds. Bounded per-job raw-log API reads
  subsequently succeeded for all three Gates jobs. All readers ended; transport timeouts are not test outcomes.
- Foundation `40749708075/123032808680`: assertion at `tests/ci/pr-modes.test.mjs:681`
  expects every browser mode for `tests/admin/oc/reauth.spec.ts`, while actual is
  `foundation-shards` plus `--browser-password-auth`. The conflicting fixture at lines 680–681 remains
  in the freshly merged integration source. This is a shared CI test conflict, not ownership drift.
- Gates `40749708453/123035494287`: `TestBrowserOC (1200.02s)` and `context deadline exceeded`.
  Receipt job `123058993884`: password-auth FAIL, 47 tests, synthetic merge
  `e8667f2fad7d22b9305818c66fbd2415ec4c3d74`. Required job `123059582522`:
  `gate=failure`, `receipts=failure`. These logs do not prove CSV assertion-level RED.
- STOP per the current card's outside-module gate rule. The CI owner must replace the registered spec
  in the unknown-path fixture with a genuinely unregistered path, preserving the all-mode assertion;
  this card does not edit that shared gate. No CSV implementation or push, no weakened tests or timeouts.
- New authored edits: only `output/oc-04/DELIVERY.md` and `red.log`; merge brings upstream changes only.
  No local tests, servers, generators or static validation were run. E3 failure evidence only;
  author acceptance E0. GREEN, causal CSV RED, local merged-head CI, independent review and LIVE: NOT_RUN.
  WAIT_RUN: NONE (no push). Integrator handoff required. Humaux unavailable: no store/canvas/coord calls.
## FIX-RED fresh relay readback (2026-10-29)

- `git fetch origin && git merge origin/r3/integration`: exit 0, already up to date at
  integration `313c2c36f4ac064975c1c7c170d710cb63fa457f`. Local head remains
  `b500d58cb0ffa541c4d2c4500aa932201d321b08`; PR head rechecked as `51e81e7d6d48e6aea299cc49d80f2f11720dcfb0`.
- Independently read all four current-head failures. Foundation `40749708075/123032808680`
  fails `tests/ci/pr-modes.test.mjs:681`: registered OC spec treated as unknown. The fixture remains
  in current integration at line 680; generated ownership lists the spec under password-auth.
  This is a shared CI gate conflict outside apps/admin, not browser ownership drift.
- Gates `40749708453/123035494287`: `TestBrowserOC (1200.02s)`, context deadline exceeded.
  Receipts `123058993884` and required `123059582522` fail downstream of password-auth.
  Raw per-job API reads exited 0 after bounded `gh run view` transport timeouts.
- STOP under the card's outside-module gate rule. CI owner must repair the unknown-spec fixture
  while retaining the all-mode assertion; a CSV assertion-level RED is still required before implementation.
  E3 failure receipts only, no completion; author acceptance E0. GREEN / independent review / LIVE: NOT_RUN.
  No local tests, servers, generators, source edits, commit or push. Only these two in-card evidence files
  appended, preserving prior dirty notes; no new outside-card edits. WAIT_RUN: NONE.
  Log readers ended. Integrator handoff is the resume path; Humaux unavailable, no store/canvas/coord claimed.

## CSV assertion-first RED continuation (2026-10-29, current handoff)

- Supersedes the older STOP/pending-status notes above. Integrator fixed the unknown-spec fixture in
  `4db7c2c9f34817888c6877e4d40c2be0a7386bae`, already present locally and on PR #246.
  Fetch/merge of current `origin/r3/integration` (`c12fac9a2acafc04a7be5222c10ff2573975dde6`) exits 0, already up to date.
- Root cause of the unhelpful RED: the six CSV cases await a response event on the unfixed native-download
  path before asserting the dialog. A download navigation can omit that event, consuming the 240-second
  test budget. They now click with `noWaitAfter: true` and immediately assert the visible re-auth dialog
  within the existing 10-second assertion timeout. The 403 status assertion follows it. No assertion,
  timeout, fixture, retry, threshold or gate-adapter assertion was weakened; all replay/download checks remain.
- This push changes tests and evidence only; CSV implementation is intentionally pending causal CI RED.
  All six orders/finance × zh-TW/zh-CN/en cases are included in the single RED push.
- E1 only: `node --check --experimental-strip-types tests/admin/oc/reauth.spec.ts` and the prior strict
  standalone `tsc` command both exit 0. `git diff --check` exits 0. No local tests or servers.
- Outside write_paths: `output/ci-select-backend-browser/browser-ownership.json`, regenerated using
  `node scripts/dev/derive-browser-ownership.mjs` (exit 0, 53 modes); only this spec's source hash changes.
  No dependency/import changed, so no dependency-map or component harness edit is needed.
- Executor: dispatched Codex implementation lane, no child agents; exact runtime model/effort not exposed.
  Base is `4db7c2c9f34817888c6877e4d40c2be0a7386bae`; branch/worktree unchanged. Prior dirty evidence notes retained.
- Humaux tools became discoverable in this relay: queried task board, canvas, relevant memory, rejected
  representative set (3/9), code map and full development rules `7bdb220b-0798-4b1d-aff5-16ee94cae75f`.
  Dispatched subtask needs no new claim. Writes use an acquired coordination lock and a separate subtask canvas element.
- Current acceptance remains author E0; causal CSV RED, GREEN, CSV implementation, independent review,
  WebKit and LIVE are NOT_RUN/PENDING. Run/job IDs are appended to `red.log` and the PR after push;
  integrator relays CI completion to resume. No production operation or local process was started.
