CA-AUTH-COPY part A — #245. Password forms stay inert before hydration but currently offer no explanation or recovery when JavaScript is disabled or a page chunk fails.

Adds three-locale browser coverage for SSR loading, noscript guidance, a delayed native reload, and removal after hydration. Tests-only RED commit: `781887326`. Implementation is gated on assertion-failing PR CI RED. Existing assertions remain unchanged.

Scope: PasswordAuth, entry-copy, password-auth.spec, and output/ca-auth-copy. No out-of-scope files changed. The prior BLOCKED.md is retained as history of the original unsplit card; the revised card explicitly excludes the mail_unavailable/OIDC capability work.

Verification: GitHub CI only; no local tests or servers. RED/GREEN run and job receipts will be recorded in output/ca-auth-copy/red.log and the final PR comment. Independent review: pending with integrator. LIVE/production: NOT_RUN. No merge or auto-merge authorized by this card.

Rollback: revert this PR's commits. No schema, backend, lockfile, baseline, dispatcher or gate changes.

🤖 Generated with Codex (dispatched by Claude Code integrator).
