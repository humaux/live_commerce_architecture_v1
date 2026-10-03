# platform-site — admin branding delivered; public website NOT COMPLETE

Base: `c2f41c91ac38aa2da9db0e39b0fcba207e33cc5c` (`r3/integration`).
Branch: `unit/platform-site`. Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/platform-site`.

Preparation commit: `5d4efcc7`; contact-validator review fix: `f38a673e`; unapproved composition checkpoint: `44a33705`; admin branding implementation/tests: `2e91c529`.

## Status

- Read current platform-site brief including DaWan Live owner ruling, company facts, env-only contact email and host requirements.
- Created the requested isolated worktree; `pnpm install --offline`: exit 0.
- Single `company.ts` identity source now drives admin Entry, signup/reset titles and W0 rail branding. Reusable operator footer uses the exact English attribution. Product name remains untranslated. Removed obsolete translated entry-brand keys. Merchant storefront source was not changed.
- Signup/reset titles use server route metadata; root login uses Entry's title only when that entry surface is mounted. Exact single-title assertions cover browser navigation and reload. Authenticated dashboard titles remain unchanged.
- Latest owner supplement: public contact email **awaits confirmation**. `LC_COMPANY_CONTACT_EMAIL` has no default. Config helper validates env-only hosts/email, but production-build/public routing integration is **not yet implemented**; no claim of complete env/build acceptance.
- Added fail-closed, case-insensitive UI/legal domain grep to check-gates. A temporary uppercase domain literal caused exit 1; after removing that exact test file the final gate passed. Tests and documentation examples remain outside this UI/legal scan.
- Audited route conflict: existing admin locale layout is non-indexable and session-oriented. Public pages need their own root layout under a public group plus a host-specific internal rewrite. Keep streaming BFF body limits intact; do not blindly expand the proxy matcher across auth endpoints.
- Three composition proposals generated using built-in image_gen, referenced to approved W0 comp 01. **None approved yet.** Exact prompts are in JSON sidecars and embedded PNG metadata. They are composition references only: generated demonstration data, copyright years, and absolute delivery claims are not approved product copy.
- Waiting for visual direction selection (A workflow / B product + operator / C task guide); impeccable requires approval before implementing this new public surface. The narrow owner-brand amendment reused the incumbent admin structure via frontend-architect/impeccable refinement; it did not require a new admin layout. No public page, deployment, DNS or Meta configuration is claimed.

## Commands and evidence

| Command | Exit | Evidence / scope |
|---|---:|---|
| `pnpm install --offline` | 0 | 49 offline packages; lockfile unchanged |
| `node --test --experimental-strip-types tests/admin/platform-site.test.ts` before module | 1 | `ps1-red.log`: missing implementation; prerequisite red only |
| same focused test after module | 0 | `ps1-green.log`: 4 PASS, 0 FAIL, 0 SKIP; config and route parsing only |
| same test with email-format review negatives, pre-fix | 1 | `config-review-red.log`: malformed dot-address accepted |
| same test after validator fix | 0 | `config-review-green.log`: 4 PASS, 0 FAIL, 0 SKIP |
| focused test with new title contract, before implementation | 1 | `brand-red.log`: missing brandedTitle export |
| same focused test after implementation | 0 | `brand-green.log`: 5 PASS, 0 FAIL, 0 SKIP |
| `bash scripts/dev/check-gates.sh` with temporary uppercase domain violation | 1 | `brand-domain-red.log`: hardcoded host detected; witness then removed |
| `bash scripts/dev/check-gates.sh` final source | 0 | `brand-check-gates.log`: 60 modes, all documented; architecture ratchet PASS |
| `bash scripts/dev/test-node.sh` final source | 0 | `brand-test-node.log`: 334 PASS, 0 FAIL, 0 SKIP across executed suites; LiveKit binary-specific file NOT_RUN |
| `pnpm --filter admin exec tsc --noEmit` final source | 0 | `brand-tsc.log` (empty successful output) |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-admin-shell` first run | 1 | `brand-browser-red.log`: old exact reset title expected no brand; updated exact assertion per owner contract, invitation title untouched |
| same browser command, final source | 0 | `brand-browser-gate.log`: W0 24 matrix cases + role negative/store switch/axe; branding 24 real-click/reload cases |
| impeccable changed-target detector (once) | 2 | `brand-design-detect.json`: one warning on pre-existing Arial at globals.css:10; retained incumbent font, no new finding |
| `git diff --cached --check` implementation | 0 | Source/test staged diff clean; generated build logs may contain terminal CR progress formatting |

## Scope of verification / NOT_RUN

- **LOCAL/MOCK**, not SANDBOX or LIVE. Browser runner uses existing MOCK identity and packaged Next app. No emails, form submission, real registration, provider operations or production processes.
- `brand-browser/brand-click-ledger.json`: 18 page navigation/reload checks plus 6 real language-selector changes; 18 screenshots (signin/signup/reset × zh-TW/zh-CN/en × 390×844/1586×992).
- W0 regression screenshots/logs remain under `output/ui-w0-shell/`; first red run is preserved under its `red/` directory. They are fresh brand-state evidence, not modifications to frozen `tests/admin/baselines/w0/`.
- PS1 full public rendered identity and missing-config production-build rejection: **NOT_RUN**. PS2 public website browser: **NOT_RUN**. PS3 Caddy: **NOT_RUN**. PS4 public verification tag: **NOT_RUN**.
- PS5 static/Node/admin tsc subset PASS; full public click-sweep: **NOT_RUN**. The passing auth-brand browser is not a replacement for PS2.
- `tests/media/r04-input-runner.test.mjs`: **NOT_RUN**, `COMMERCE_R04_LIVEKIT_BINARY` unset (test-node explicitly reports it).
- No migrations or Go/SQL changes; full G07 not run for this UI-only slice. No push, merge, deployment, host/DNS or secret changes.

## Next

1. Record chosen comp and approved sidecar; implement pages with actual contract-derived copy.
2. Add public host allowlist/routing, metadata/robots/sitemap and production configuration validation. Reuse the delivered company identity/footer in public pages.
3. Caddy/compose/preflight/runbook changes; register and run PS1–PS5 and click-sweep; independent review.
4. Final logical commits, evidence and Humaux/canvas update. Do not push, merge or deploy.

Legal content on eventual delivery: **需 owner/律師審閱**. This checkpoint contains no drafted legal policy.

## Collaboration

Root author Codex, base SHA above, only this worktree source write paths; no other checkout code changes.
Read-only explorers `ps_routes` and `ps_gates`: gpt-6-luna / medium; same base and worktree; allowed write paths none. Their findings are research, not gate acceptance.
Independent preparatory-source review by `ps_routes`: no P0/P1 within the two files only; email-format caveat addressed with red/green evidence above.
Independent brand-diff review by `platform_brand_review` (explorer, gpt-6-luna / medium; same base/worktree, no write paths): no evidence-backed P1/P2 in scoped source. Supplemental visual pass inspected all 18 auth images: correct dimensions/page names, nonblank, brand/operator readable, no clipping or overlap; mobile attribution wraps cleanly. Reviewer did not rerun the browser suite and did not approve the deferred homepage.
Root inspected six representative auth captures across all three locales and both sizes: correct pages, readable attribution, no overlap/clipping. Browser asserts no horizontal overflow and one exact title after reload for every auth case.
The local design-choice server is retained intentionally for pending selection: `http://127.0.0.1:58620/`, key `880ef0da`. The browser runner closed its owned Next, fixture and browser processes; no database process was started by this slice.
