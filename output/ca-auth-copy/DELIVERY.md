# CA-AUTH-COPY part A — implementation handoff

Status: FIX 1/2 submitted for renewed PR CI; GREEN PENDING. Author assessment E0; prior check_packet structure E1; assertion-failing RED E3. Historical implementation handoff appears below; the FIX 1/2 section supersedes its pending run observations. No completion or functional PASS claim.

- Role: delegated implementation worker; Codex (runtime model/effort identifiers are not exposed).
- Base: `79f95784c9dee419346fe3119d4d4f56720d4a3c`.
- Branch: `unit/ca-auth-copy`; worktree: `.worktrees/ca-auth-copy`; PR #251 into `r3/integration`.
- Tests-only RED source: `781887326d96ae0e291e3ce3ab2a87a2e28636f9`.
- Authority: revised integrator card, issue #245, `contracts/merchant-password-auth-v1.md` §7.3 and repository DS components/tokens at the base SHA. No mail_unavailable/OIDC changes.
- Allowed paths: PasswordAuth.tsx, entry-copy.ts, password-auth.spec.ts, output/ca-auth-copy/**. Out-of-card changes: none.
- `BLOCKED.md` remains historical evidence of the original unsplit card; the revised part A resolves that scope blocker. Its network error is historical: current gh/push access worked.
- Humaux search/store/canvas/coord/code-index tools are not exposed in this worker session. No Humaux write, canvas update, lease claim or code-memory link is claimed. The dispatched assignment owns this branch; evidence is in this card directory and GitHub for the integrator to record.

## Implementation and acceptance

The existing mounted guard remains intact. SSR includes localized loading status (`role=status`, explicit polite live region), a noscript explanation and a DS ButtonLink rendering a plain anchor with empty href (current resource path/query). A CSS-only 10-second visibility deadline works even if every page chunk fails. Hydration removes the recovery subtree. The status occupies existing space above the panel, outside flow, so removing it cannot shift the form. New visual elements use DS Badge, Alert and ButtonLink, with DS spacing tokens. No new dependency or fetch call. The pre-existing direct-fetch password helper is unchanged; moving it is outside this UI card.

Gate: the PR's own GitHub CI, including password browser acceptance and check-ds-rules. Existing tests/assertions are untouched; the RED commit only added three tests for each locale. No thresholds, fixtures, baselines, exemptions, gate adapters or dispatchers changed.

## Evidence

- RED Gates run/job: `40612180005` / `122576061200`, failure at the new noscript assertion, `password-auth.spec.ts:189`. See red.log and red-playwright.log. One failed; twenty tests skipped by serial mode. This proves the missing no-JS guidance, not all new cases independently red.
- Official job logs and artifact were inspected with gh. Artifact download initially returned TLS handshake timeout; one retry via the same gh API succeeded. No denial or workaround.
- Local allowed static command: `python3 scripts/check_packet.py`, exit 0, check-packet.log. This validates the architecture packet only.
- `git diff --check`: exit 0.
- Local tests, builds, servers, browsers, PG, containers: NOT_RUN per card. Typecheck: NOT_RUN (no installed worktree node_modules). Browser/DS/build verification belongs to CI.
- GREEN implementation run/job IDs and current head will be registered after push in a PR comment and local handoff snapshot, avoiding an evidence-only commit after GREEN.
- Monitoring handoff: the dispatching integrator owns completion notification/resume for the recorded CI jobs; this worker does not poll beyond ten minutes. GitHub jobs have workflow timeouts. On failure inspect logs, stop for unrelated gates, and permit at most two targeted fixes per blocker. No merge or auto-merge.

## Click ledger (implementation execution PENDING CI)

| Page | Control | Operation | Expected | Actual |
| --- | --- | --- | --- | --- |
| Three locales, sign-in/signup/reset; JS disabled | Inputs and submit | Inspect native disabled state | Inert controls and localized noscript explanation | RED zh-CN failed at missing noscript; GREEN NOT_RUN |
| Three locales, reset; JS disabled | Delayed reload | Native click and full navigation | Same path/query, noscript remains, input disabled | NOT_RUN |
| Three locales, sign-in; aborted chunks | Delayed reload | Native click after lifting abort | Same path/query, hydration succeeds, inputs enabled | NOT_RUN |
| Three locales, hydrated sign-in/signup/reset | Status and reload | Observe immediately and after ten seconds | Both removed | NOT_RUN |

Risks/limits: browser timing, no-JS parsing and desktop/mobile layout await CI; independent visual review remains NOT_RUN. Native empty href reloads the current resource path/query; an invite URL fragment is not available to SSR. No backend/auth capability, schema or production behavior changed. Rollback: revert this card's commits. No local server or background watcher was started in this continuation. Downloaded archive and full job log were removed after retaining the failed assertion; historical failure receipts remain.

## Current-head CI registration (local post-push handoff; no evidence-only commit)

- Implementation source: `7cf853a1b874b920346705c47bb9d9c6f14366a1` (pushed; PR head confirmed through gh).
- Gates run `40619949465`: PENDING; jobs endpoint currently returns no jobs, so password-browser job ID is not assigned yet. Resume on this run once jobs exist; required acceptance includes gate (--browser-password-auth).
- Foundation run `40619949514`, job `122591286226`: IN_PROGRESS at handoff.
- `gh pr checks 251 --json name,state,link` snapshot: green-checks-at-handoff.json. No current-head GREEN receipt exists.
- PR body updated to implementation scope and RED evidence; the local PR_BODY.md mirrors that update and is intentionally uncommitted so CI stays bound to the implementation head.
- check_packet changed only the timestamp in experiments/results/packet-check.json; that generated change was restored immediately and is absent from the commit/worktree diff.
- No local watcher/server remains. CI completion/resume is handed to the integrator as required; this session ends without a long CI poll.

## FIX 1/2 — WebKit no-JS matcher root (2026-10-28)

- Starting head: `7cf853a1b874b920346705c47bb9d9c6f14366a1`; branch and PR unchanged.
- Existing RED: Gates run `40619949465`, job `122622312675`, attempt 1, `gate (--browser-webkit)` failed on the card's new noscript text assertion. The tested synthetic merge was `ac3addbe7d09a6003325e1859bdced827c74ab93` (base `5365b5701a0e99ef9fd8fbb4e64a3a9f87aaf248`). Official job logs and artifact `12793052251` were read; retained receipts are `fix1-red.log`, `fix1-webkit-no-js.png`, and the appended entry in `red.log`.
- Root cause: the actual failing WebKit screenshot already shows the entire localized JavaScript explanation above disabled controls. Playwright 1.63.0 `shouldSkipForTextMatching` explicitly ignores a NOSCRIPT root: https://github.com/microsoft/playwright/blob/v1.63.0/packages/injected/src/selectorUtils.ts#L57-L60. Empty matcher output does not establish empty DOM content or an invisible product message. The repository currently uses the Desktop Safari profile for admin WebKit, per `playwright.config.ts`; no profile change is made here.
- Decision: preserve the working product fallback. Retarget only the card's new `toContainText` root to the rendered DS Alert (`noscript` → descendant `role=status`), retaining the exact complete localized expected string. Add explicit visibility of that status. The existing exact visible-text, inert controls, native click/reload, path/query persistence, locale and engine requirements remain intact. Add no-script explanation absence checks after normal hydration and after the existing ten-second deadline. No assertion, timeout or engine coverage is weakened; no gate adapter changed.
- This is a narrow correction to the newly added card test's selector, justified by the current fix instruction to check user-visible text; older pre-card tests remain untouched. No product change is warranted by this failure's evidence. Acceptance still requires this PR's renewed Chromium and WebKit CI, including the later click/reload cases that serial execution had not reached.
- Local checks/tests/builds/servers/browser/PG/containers in FIX 1/2: NOT_RUN. All inspected browser results came from GitHub CI. No local background process started. The downloaded artifact directory was removed after retaining the scoped failure receipt and screenshot. Pre-existing handoff files were preserved and updated within the card's output directory.
- New head and Gates/Foundation run IDs will be registered after the single push in the PR comment and local handoff, with no evidence-only follow-up commit. Integrator owns CI completion notification and a fresh-session resume; no merge/auto-merge. Out-of-card paths: none. Humaux store/canvas/coord/code tools remain unavailable; repository evidence is ready for integrator recording, and no Humaux persistence is claimed.
