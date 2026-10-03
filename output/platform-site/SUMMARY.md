# platform-site — checkpoint, NOT COMPLETE

Base: `c2f41c91ac38aa2da9db0e39b0fcba207e33cc5c` (`r3/integration`).
Branch: `unit/platform-site`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/platform-site`.

Preparation commit: `5d4efcc7`; contact-validator review fix: `f38a673e`.

## Status

- Read current platform-site brief including DaWan Live owner ruling, company facts, env-only contact email and host requirements.
- Created the requested isolated worktree; `pnpm install --offline`: exit 0.
- Prepared a single company identity/config module and four focused tests. It is **not yet wired to pages or build**.
- Audited route conflict: existing admin locale layout is non-indexable and session-oriented. Public pages need their own root layout under a public group plus a host-specific internal rewrite. Keep streaming BFF body limits intact; do not blindly expand the proxy matcher across auth endpoints.
- Three composition proposals generated using built-in image_gen, referenced to approved W0 comp 01. **None approved yet.** Exact prompts are in JSON sidecars and embedded PNG metadata. They are composition references only: generated demonstration data, copyright years, and absolute delivery claims are not approved product copy.
- Waiting for visual direction selection (A workflow / B product + operator / C task guide); the existing impeccable workflow requires comp approval before UI implementation. No new public page, deployment, DNS or Meta configuration is claimed.

## Commands and evidence

| Command | Exit | Evidence / scope |
|---|---:|---|
| `pnpm install --offline` | 0 | 49 offline packages; lockfile unchanged |
| `node --test --experimental-strip-types tests/admin/platform-site.test.ts` before module | 1 | `ps1-red.log`: missing implementation; prerequisite red only |
| same focused test after module | 0 | `ps1-green.log`: 4 PASS, 0 FAIL, 0 SKIP; config and route parsing only |
| same test with email-format review negatives, pre-fix | 1 | `config-review-red.log`: malformed dot-address accepted |
| same test after validator fix | 0 | `config-review-green.log`: 4 PASS, 0 FAIL, 0 SKIP |

PS1 full rendered identity/build-failure acceptance: **NOT_RUN**. PS2 real browser: **NOT_RUN**. PS3 Caddy: **NOT_RUN**. PS4 rendered metadata: **NOT_RUN**. PS5 check-gates/test-node/admin tsc/click-sweep: **NOT_RUN**.

## Next

1. Record chosen comp and approved sidecar; implement pages with actual contract-derived copy.
2. Add public host allowlist/routing, metadata/robots/sitemap, production configuration validation and admin brand touchpoints.
3. Caddy/compose/preflight/runbook changes; register and run PS1–PS5 and click-sweep; independent review.
4. Final logical commits, evidence and Humaux/canvas update. Do not push, merge or deploy.

Legal content on eventual delivery: **需 owner/律師審閱**. This checkpoint contains no drafted legal policy.

## Collaboration

Root author Codex, base SHA above, only this worktree source write paths; no other checkout code changes.
Read-only explorers `ps_routes` and `ps_gates`: gpt-6-luna / medium; same base and worktree; allowed write paths none. Their findings are research, not gate acceptance.
Independent preparatory-source review by `ps_routes`: no P0/P1 within the two files only; email-format caveat addressed with red/green evidence above. Production-build and rendered-page tests remain pending.
The local design-choice server is retained intentionally for the pending selection: `http://127.0.0.1:58620/`, key `880ef0da`. No app/test/database process was started.
