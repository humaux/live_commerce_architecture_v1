# CA-AUTH-COPY part A — implementation handoff

Status: implementation submitted for PR CI; GREEN PENDING. Author assessment E0; check_packet structure E1; assertion-failing RED E3. No completion or functional PASS claim.

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
