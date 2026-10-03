# platform-site — approved A implemented; PS1–PS4 PASS; PS5 BLOCKED

Date: 2026-10-04 (Asia/Shanghai). Branch: `unit/platform-site`.
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/platform-site`.
Base: `c2f41c91ac38aa2da9db0e39b0fcba207e33cc5c`. Final implementation: `8f567049`.
This replaces the historical pre-approval checkpoint; its earlier evidence remains in Git and the named logs.

## Result and scope

- Owner-approved **A「從留言到訂單」**: DaWan Live and sign-in at the top, headline plus Facebook comment → merchant storefront → order management. The flow is explicitly a product illustration; it does not promise every merchant is connected. Company details are below the home first viewport, not in the hero.
- Public `/`, `/privacy`, `/terms`, `/data-deletion`, `/contact`, each in zh-TW / zh-CN / en; server-rendered, crawlable, canonical/hreflang plus robots and 15-URL sitemap.
- One company source in `apps/admin/lib/company.ts`: exact legal names, company/BRC numbers, dates and un-translated registered address from the owner-approved brief. Full facts appear in every public footer and every legal/contact body. No directors' personal details used.
- Product brand stays untranslated. Prior commits also apply it to auth titles/entry, W0 rail and operator attribution; merchant storefront branding is untouched.
- Host/email configuration remains env-only. No real-looking default mailbox. Missing production contact config fails the build. The pilot email appears only in the authorized deploy runbook; test fixtures use `contact@example.invalid`.
- Public routing uses the actual Host, a path allowlist and a separate public root layout. It does not expose BFF/API or internal rewrite routes on the platform host, and does not expand the existing auth-body proxy matcher.
- Optional domain-verification tag is absent when unset; when set it renders exactly once on each locale's apex home page, never a legal page or admin page.
- Caddy platform block and www 301, compose/admin build arguments, preflight, smoke fixture and runbook are delivered. No migration, Go/SQL, production configuration, DNS, Meta settings, mail delivery, push, merge or deployment.

**Legal text: 需 owner/律師審閱.** It describes current account/Meta/buyer/payment processing, manual email deletion requests and retention obligations. It invents no signed-request callback or fixed deletion SLA. This is a draft, not a certification of legal compliance.

## Logical commits

| Commit | Change |
|---|---|
| `5d4efcc7`, `f38a673e` | Company/config/route contract and contact validation |
| `2e91c529` | DaWan Live auth and W0 branding; merchant brand unchanged |
| `9fa55342`, `5baf3176` | Owner-confirmed mailbox runbook/example and review evidence |
| `b12c4e7f` | Approved A, three-locale public/legal pages, routing, metadata, synthetic illustrative garment |
| `e7ce99f1` | Caddy, compose/build/preflight/smoke/runbook and PS3 fixture |
| `38cee4d8` | Formal public browser mode, strict release-gate registration, shared test queue |
| `f626bb71` | Source-backed public-surface DESIGN and sidecar, without replacing the W0 design system |
| `8f567049` | First-viewport proof in addition to full-page captures |

All commits have `Co-Authored-By: Codex <noreply@openai.com>`. Earlier proposal/checkpoint commits remain in history; they are not the current acceptance state.

## PS1–PS5 evidence

| Gate | Status | Evidence |
|---|---|---|
| PS1 | PASS (LOCAL) | 7 focused Node cases within the 336-case Node run; 15 rendered company/SSR checks; actual production build missing email exits 1 as required |
| PS2 | PASS (LOCAL/MOCK) | 30 page/viewport/language cases, 390 actual clicks plus 30 reloads; no horizontal overflow; real document navigation, language changes, admin links, skip link, mailto activation |
| PS3 | PASS (LOCAL Caddy TLS / MOCK upstream) | Pinned Caddy adapt/validate/fmt plus 22 requests: platform pages/static, rejected API/internal paths/POST/forwarded-host spoof, HEAD, www 301 preserving path/query, admin host |
| PS4 | PASS (LOCAL) | 16 verification-tag cases, unset and set; production-Next crawler checks |
| PS5 | **BLOCKED / NOT ALL GREEN** | Node/tsc/check-gates are green. Serial full click-sweep exits 1 on existing merchant-storefront routes: 120 pages / 32 load failures, 983 controls / 960 pass / 3 fail / 20 skip; 18 journeys pass. Public-matrix standalone result is separate, not a replacement. |

## Commands and exit codes

All commands below run in this worktree. Browser queue uses `LC_TEST_LOCK_WAIT=14400`; no foreign lock was removed or process terminated.

| Command | Exit | Count / artifact |
|---|---:|---|
| `pnpm install --offline` (workspace setup) | 0 | Lockfile unchanged |
| `bash scripts/dev/test-node.sh` (final implementation) | 0 | 336 PASS / 0 FAIL / 0 SKIP across executed suites; `test-node.log` |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0 | 61 documented modes, tracked tests routed; existing allowlist warnings retained; `check-gates.log` |
| `pnpm --filter admin build` with synthetic required host/contact env | 0 | `build.log` |
| `pnpm --filter admin build` with `LC_COMPANY_CONTACT_EMAIL` unset | **1 (expected)** | Fails on `LC_COMPANY_CONTACT_EMAIL`; `ps1-missing-env.log` |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-platform-site` | 0 | `browser-platform-site-final.log`, `ps-browser-result.json`, `click-ledger.json` |
| `node tests/deploy/platform-edge.mjs` | 0 | 22 requests; `ps3.log`, `ps3-edge.json`, Caddy artifacts |
| `LC_COMPANY_CONTACT_EMAIL=contact@example.invalid docker compose --env-file deploy/env/compose.env.example -f deploy/compose.yml config --no-env-resolution --quiet` | 0 | `compose-config.log` |
| `bash scripts/dev/release-gate.sh --list` | 0 | Includes `B-browser-platform-site` and `B-browser-click-sweep`; listing, not full strict acceptance |
| Shell syntax checks for modified dev/deploy scripts; Node syntax checks for new runners | 0 | LOCAL static validation |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-click-sweep` (serial final) | **1** | `click-sweep-final.log`; 711.03 s; 120 pages / 32 load failures, 983 controls / 960 pass / 3 fail / 20 skip; 18 journeys PASS |
| Impeccable changed-public-target detector (once) | 0 | `design-detect.json`: [] |
| Raster provenance scan | 0 | One generated raster, no missing sidecar; `raster-provenance.log` |
| `git diff --check` (source) | 0 | No source whitespace errors |

The earlier brand-only `--browser-admin-shell` run also exited 0: W0 24-case matrix, role negative/store switch/axe, plus 24 brand click/reload cases. See `brand-browser-gate.log` and `brand-browser/`. This is prior-commit regression evidence, not a rerun of the final public source.

## Red → green / retained failures

- PS1 initially failed on missing implementation; contact-format negative test then caught a malformed dot-address; branded-title contract and uppercase hardcoded-domain witness each failed before their respective fixes. Logs are retained (`ps1-red.log`, `config-review-red.log`, `brand-red.log`, `brand-domain-red.log`).
- PS3 fixture initially returned empty 200 because the mock upstream matched a loopback Host rather than the preserved platform Host; a second run hit a stale Docker Desktop bind-mounted file after rewrite. Read-only independent diagnosis confirmed both. Hostless loopback fixture and a distinct immutable runtime Caddyfile fixed the fixture; final 22 checks passed. Red logs: `red/ps3-fixture.log`, `red/caddy-fixture.log`.
- First full click-sweep exited 1: 120 pages / 1 load failure; 984 controls / 957 pass / 1 fail / 26 skip; 18 journeys passed. **Author-caused same-worktree Next rebuild overlap** invalidated active static chunks. Evidence is under `red/click-sweep-concurrent-build/`. Public test mode now joins the same machine-level queue; final rerun is strictly serial. No test assertion or threshold was relaxed.
- Corrected the public click ledger's source reset after independent review. Reloads are reported separately, not inflated into click count. Mailto activation is not proof of OS mail handling or email delivery.
- Serial rerun also failed, with a different signature: merchant-storefront 503s for collections, cart and a static JS chunk; the admin matrix and 18 journey steps passed. Complete red logs/ledger and all 35 failure captures are under `red/click-sweep-serial/`. There was no parallel Next build in this rerun. `git diff c2f41c91..HEAD -- apps/storefront internal tests/ui/click-sweep.mjs tests/foundation/browser_click_sweep_test.go` is empty; this does not by itself prove a baseline failure, and a baseline full sweep was NOT_RUN.
- Diagnostic boundary: `apps/storefront/lib/primary-origin.ts` returns 503 when its server-authoritative origin resolver fails; the synthetic relay's catch returns 502 instead. Storefront Next output has startup only, so the precise failing upstream response/timeout is not captured. The sweep's reused storefront monitor reads `since(0)` for each page and therefore propagates earlier errors into later page records. **32 failed page records are not evidence of 32 independent faults.** Do not weaken that gate or the canonical-host security check to make this unit pass. The integrator must adjudicate the existing-path failure/instrumentation before PS5 can be green.
- The combined sweep stops before its final public stage when the legacy suite fails. The formal standalone public mode was rerun on final source; its result is recorded separately above. No assertion or frozen known-defect list was changed.

## Screenshots and visual review

- 30 final full-page captures in this directory: `{home,privacy,terms,data-deletion,contact}-{zh-TW,zh-CN,en}-{390,1586}.png`.
- Browser viewports are 390×844 and 1586×992; full-page image heights can exceed the viewport.
- First-viewport proof: `.impeccable/review/hero-repro.png`. Initial pre-finish hero retained as `hero-initial-before-finish.png`.
- Two static garment previews are generated illustrative material, not merchant/product evidence. Source prompt and hashes live next to `apps/admin/components/platform/example-shirt.jpg`.
- Fresh read-only visual review inspected all 30 captures, then only the six affected home captures after one bounded correction batch. Final disposition **ship** for those findings; no new visual regression reported. No independent Quality Bar card was available, so no design-ceiling claim is made.
- Scoped security review found no P0/P1; its P2 click-source issue was corrected and closed. See `REVIEW.md`.

Impeccable influenced the approved-A implementation, audit-first scope, bounded visual finish and separate public design record. Frontend-architect guided shared configuration/component boundaries. Playwright supplied real browser evidence. Specialized Impeccable reviewer/documenter roles were unavailable; fresh generic read-only agents used the role references and are explicitly identified, not represented as specialized agents.

## Collaboration and ownership

Root owns all writes in this isolated worktree; base SHA above. No code edits in other checkouts. Read-only agents had no write paths and did not run independent shared builds:
- `ps_routes`, `ps_gates`: explorer, gpt-6-luna/medium, route and fixture diagnostics.
- `ps_security_final`: security_reviewer, gpt-6.1-sol/high, scoped security/source review.
- `ps_visual_finish`: default, gpt-6.1-sol/medium, fresh bounded visual review.
- `ps_documenter`: default, gpt-6-luna/medium, source-backed reusable-rule extraction.
- `ps_evidence_check`: test_worker, gpt-6-luna/medium, read-only artifact/count consistency check; no mismatch in PS1–PS4.
Their findings were stored in Humaux. The integrator's separate final acceptance is still to be scheduled.

## NOT_RUN / release boundary

- LIVE/SANDBOX deployment, real DNS/TLS issuance, Meta domain/business/App Review configuration, real registrations, emails or provider operations: **NOT_RUN**.
- Full Docker application image build / full production compose startup / full preflight against real hosts: **NOT_RUN**. Local actual Caddy and compose syntax/config are separately passed above.
- Full strict release gate and full G07: **NOT_RUN**; no migration/SQL/ACL change in this unit. The release catalog registration was checked.
- Optional `tests/media/r04-input-runner.test.mjs`: **NOT_RUN**, `COMMERCE_R04_LIVEKIT_BINARY` unset; test-node explicitly reports it.
- No claim that Meta will approve the business or application. Owner/legal review, deployment and integrator independent acceptance remain outside local delivery.
- Final local delivery remains **blocked only on PS5**, not represented as complete release acceptance. Independent integrator review and a green complete sweep remain required. The last standalone public retest on `8f567049` exited 0 with the same 30 pages / 390 clicks / 30 reloads, and refreshed the first-viewport proof. Root inspected that proof: company facts remain below the first viewport.
- Runners closed their owned Next/browser/fixture processes. The choice server on port 58620 was already absent (`lsof` exit 1, no listener); no unrelated service was stopped. No task-owned public runner/Next process remained in the postflight process check. No shared cache was deleted.

## Handoff stop line

1. PS5 is not waived. Preserve both red sweeps; before another full rerun, collect the first primary-origin resolver status/latency/error in the existing merchant-storefront test fixture. It is not available in the retained startup-only Next log.
2. The integrator should separately decide the correction to the existing monitor's per-page event attribution while retaining the first real 503 as a failure. No changes to that old runner or the canonical security guard were made in this unit.
3. Re-run the complete click-sweep after the underlying failure is understood. Standalone PS1–PS4 green results do not replace it. Then perform the integrator's independent review and owner/legal review before any authorized deployment.
