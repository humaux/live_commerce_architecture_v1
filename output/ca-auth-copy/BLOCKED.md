# CA-AUTH-COPY — scope blocker

Status: BLOCKED before RED; no implementation or tests changed. Evidence: E0 author static findings, NOT_RUN CI.

Assignment: delegated implementation worker, Codex (GPT-6; runtime model/effort identifier not exposed).
Branch: `unit/ca-auth-copy`. Base/head: `79f95784c9dee419346fe3119d4d4f56720d4a3c`.
Worktree: `.worktrees/ca-auth-copy`. Target: `r3/integration`.
Authority: dispatched CA-AUTH-COPY card; `contracts/merchant-password-auth-v1.md` §6/§11/§16.1 at that SHA; issue #245 read through the connected GitHub tool.

## Blocking paths outside the card

1. `apps/admin/app/api/auth/password/step1.ts:80-105` rebuilds every upstream error with `localError`. Its parsed shape and allowlist retain only password-policy `reason`; `handleStep1` calls it at line 120 for a 503. A Go `oidc_available` field cannot reach PasswordAuth through this production path. This BFF file is outside the write paths.
2. `internal/identity/challenge.go:292-294` returns an empty Challenge plus the plain `ErrMailUnavailable` sentinel after SMTP FAILED. `internal/identity/reauth.go:161-162` likewise returns a zero expiry plus that sentinel. Neither result carries verified-principal OIDC capability. The allowed `internal/identityhttp/password.go:28-32,198-199` has no principal/capability lookup seam and cannot securely invent the boolean. Identity service changes and their tests are outside the write paths. Any required SQL capability lookup/migration also needs an integrator-assigned design and migration number; none is assigned by this card.

Integrator action: revise/split the card to authorize the BFF propagation and the identity-owned post-password capability contract, including tests and any assigned migration. Do not infer capability from an email match, expose it before password proof, or add a fake optional interface with no production implementation.

## Verification / handoff

- Local tests/servers: NOT_RUN, as instructed. No processes started.
- RED/GREEN run and job IDs: none; implementation remains gated on RED.
- PR: none; no commits or pushes attempted.
- `gh issue view 245 --json title,body`: failed because the sandbox denied the configured proxy connection. Connected GitHub `fetch_issue` succeeded and returned the original issue body. This is separate from the scope blocker.
- Files changed: only this allowed output report. No out-of-scope file changed.
- No test, threshold, fixture, gate assertion, baseline, dispatcher, lockfile or migration changed.
- Resume with the revised scope, write tests first, obtain assertion-failing RED in the PR CI, then implement. Independent review, UI click ledger, CI GREEN and LIVE remain NOT_RUN.
