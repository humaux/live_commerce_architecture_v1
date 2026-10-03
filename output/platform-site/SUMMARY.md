# platform-site — FIX FIRST corrections; PS1–PS4 PASS; full smoke BLOCKED

Date: 2026-10-04 (Asia/Shanghai). Branch: `unit/platform-site`.
Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/platform-site`.
Base: `c2f41c91ac38aa2da9db0e39b0fcba207e33cc5c`. Reviewed delivery: `8f058df7`; latest implementation: `3e26a969` (includes `e360e6b8`, `bb50ec5a`).
This review-fix section supersedes the earlier acceptance status below. Original red runs remain evidence, not a current PS5 verdict.

## Independent-review corrections (2026-10-04)

Scope: the user's three P1 and four P2 requests. No push, merge, deployment, migration, production secret access or product Go/SQL change. The existing Caddy Go **test fixture** is the only Go edit. Approved A design remains intact.

| Review item | Change and result |
|---|---|
| P1 Meta disclosures | **FIXED**, `e360e6b8`: privacy and terms in all three locales describe Facebook Page / linked Instagram information, post/live comments, Messenger/IG direct messages, identifiers/name/username/text, and private comment replies containing order-claim links. Grounded in `internal/metaconnect/graph.go`, `internal/integrations/meta/normalize.go` and `internal/integrations/metareply/{routes,render}.go`. No promise of a general DM inbox, public replies, automatic deletion callback or fixed retention period. |
| P1 pinned Caddy fixture | **PASS**, `bb50ec5a`: add synthetic `LC_PLATFORM_HOST=platform.localhost`; exact requested Go test exits 0 on final Caddy source. |
| P1 smoke S07 build ordering | **FIXED; full acceptance BLOCKED**, `bb50ec5a`: export the same synthetic platform/admin/contact configuration as `make_config` before the first image build. Build validation remains fail-closed. Actual native `smoke.sh full` exits 3: EUID 501, root required for UID 999 backup ownership; no S07+ case executed. See supplementary admin-image check below. |
| P2 Host canonical comparison | **PASS**, `e360e6b8`: one strict actual-Host parser strips numeric port and one terminal dot, lowercases DNS, rejects malformed authorities; used in proxy, public layout, robots and sitemap. Canonical links still come only from configured env. 16 successful public Host-variant requests + 3 www negatives; no locale cookie/admin fallback. |
| P2 public trailing slash | **PASS**, `bb50ec5a`: known GET/HEAD documents only receive 301 to slashless path, preserving query bytes; unknown/admin/API paths, double trailing slash and POST remain 404. Pinned Caddy covers 47 requests. Uses Caddy's documented [`{?query}` placeholder](https://caddyserver.com/docs/caddyfile/concepts#placeholders). |
| P2 admin robots regression | **PASS**, `e360e6b8`: restore the old `/robots.txt` 404, without redirect or locale cookie. Tested against built Next on the admin Host. |
| P2 deletion button copy | **PASS**, `e360e6b8`: exact settings-card and button labels in all three locales, including zh-TW「中斷連接」; tested against the actual `metaConnectCopy`. |

### Current verification (all commands executed in this worktree)

| Command | Exit | Evidence / count |
|---|---:|---|
| `node --test --experimental-strip-types tests/admin/platform-site.test.ts` | 0 | `review-fix/node-final.log`: 10/10, including the supplemental image manifest regression |
| `LC_TEST_LOCK_WAIT=14400 bash scripts/dev/test-local.sh --browser-platform-site` | 0 | `review-fix/browser-green.log`: PS1/2/4, 30 page cases, 15 SSR checks, 16 tag cases, 390 actual clicks + 30 reloads; additionally 20 new Host/www/admin-robots assertions |
| `node tests/deploy/platform-edge.mjs` | 0 | `review-fix/edge-green.log`, refreshed `ps3-edge.json`: PS3, 47 real pinned-Caddy requests (MOCK upstream) |
| `bash scripts/dev/test-node.sh` | 0 | `review-fix/test-node.log`: 339 PASS / 0 FAIL; optional R04 NOT_RUN (binary unset) |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `review-fix/admin-tsc.log` |
| `bash scripts/dev/check-gates.sh` | 0 | `review-fix/check-gates.log`: 61 modes; existing warning allowlist unchanged |
| `go test -count=1 -run '^TestCaddyfileLoadsOnPinnedCaddy$' ./internal/storefrontdomains` | 0 | `review-fix/caddy-go-final.log`: final Caddy source |
| `gofmt -l internal/storefrontdomains/caddyfile_test.go`; `git diff --check` | 0 | No output / no formatting errors |
| `bash deploy/scripts/smoke.sh full` | **3 BLOCKED** | `review-fix/smoke-full.log`: static S01–S06 pass, shellcheck NOT_RUN; all full cases BLOCKED by root prerequisite. Raw result/case/command manifest retained in `review-fix/smoke-full/`. |
| `bash deploy/scripts/smoke.sh static` | 0 | `review-fix/smoke-static.log`: S01–S06 PASS, shellcheck NOT_RUN (not installed); `review-fix/smoke-static/` contains the raw manifests |
| S07 supplemental admin Docker image build with synthetic env (exact command below) | 0 | `review-fix/admin-docker-build.log`, `admin-image.txt`: real clean Linux image build; not a substitute for full smoke/all four images |

```sh
LC_PLATFORM_HOST=platform.localhost LC_ADMIN_HOST=admin.localhost \
LC_COMPANY_CONTACT_EMAIL=contact@example.invalid \
docker build --pull=false -f deploy/docker/admin.Dockerfile \
  -t lc-platform-review-admin:ps-fix-20261004 \
  --build-arg GIT_SHA=bb50ec5a2409e0af7f42be999c92c230af091832-review-dirty \
  --build-arg LC_PLATFORM_HOST --build-arg LC_ADMIN_HOST \
  --build-arg LC_COMPANY_CONTACT_EMAIL .
```

**Additional build failure caught and fixed:** the first actual admin Docker build passed public configuration and compilation, then failed on `packages/ui/src/AppShell.tsx` missing React types. The deps stage installed before copying W0 `ui` / `format` manifests, so the clean image lacked UI peer dependencies, unlike the local workspace. `3e26a969` adds exactly those two manifest COPY lines before frozen install (no dependency/lockfile/component change). The new manifest test first failed (9 pass / 1 fail, `image-manifest-red.log`) and then passed. The same Docker build then exited 0. The failed build is preserved as `admin-docker-build-red.log`. The resulting local test image is retained for review; no container or production service was started from it, and shared build caches were not pruned.

**Red evidence:** `review-fix/node-red.log` exits 1 (7 pass / 2 fail); `browser-red.log` exits 1 (Host variant writes admin locale cookie); `edge-red.log` exits 1 (slash path 404); `caddy-go-red.log` exits 1 (empty Caddy block). During repair, `edge-double-slash-red.log` caught Caddy's normalized double-slash match; fixed without removing that negative. `node-runner-error.log` is a command setup error (`tsx` absent), corrected to the repository's native Node strip-types runner, not a product failure. Native `build-images.sh --only admin` also hit an existing macOS Bash 3.2 empty-array error (`s07-admin-build.log`); product/Linux script semantics were not weakened for that host.

**Independent check:** `/root/ps_review_fix_final`, security_reviewer, gpt-6.1-sol / high, read-only, base `8f058df7`; no writable paths, no recursive delegation. No actionable scoped P1/P2; independently ran 9 Node tests and diff check, both exit 0. Follow-up review at `bb50ec5a` covered the two manifest COPY lines and new regression: 10/10 Node and diff check exit 0, no actionable P1/P2. No claim that this reviewer reran Docker/Go/browser. Research helpers were read-only gpt-6-luna / medium: `ps_review_legal_facts`, `ps_review_smoke_plan`. All findings stored in Humaux.

**Screenshots:** refreshed three-language privacy/terms/deletion screenshots at 390×844 and 1586×992 viewport settings (full-page captures), in this directory. Root visually checked English mobile privacy and zh-TW desktop deletion; legal columns/buttons remain readable. Impeccable clarify guidance was used to match instructions to actual UI labels without changing approved A.

**PS5: DEFERRED TO INTEGRATOR by explicit user instruction.** Not rerun or altered in this round. User reported the `c2f41c91` baseline sweep passed and is independently testing `8f567049`; these are user-supplied attribution updates, not this branch's new execution results.

**Legal text: 需 owner/律師審閱.** Source-aligned disclosure is not legal approval or a promise of Meta approval.

## Original delivery scope and historical evidence

## Result and scope

- Owner-approved **A「從留言到訂單」**: DaWan Live and sign-in at the top, headline plus Facebook comment → merchant storefront → order management. The flow is explicitly a product illustration; it does not promise every merchant is connected. Company details are below the home first viewport, not in the hero.
- Public `/`, `/privacy`, `/terms`, `/data-deletion`, `/contact`, each in zh-TW / zh-CN / en; server-rendered, crawlable, canonical/hreflang plus robots and 15-URL sitemap.
- One company source in `apps/admin/lib/company.ts`: exact legal names, company/BRC numbers, dates and un-translated registered address from the owner-approved brief. Full facts appear in every public footer and every legal/contact body. No directors' personal details used.
- Product brand stays untranslated. Prior commits also apply it to auth titles/entry, W0 rail and operator attribution; merchant storefront branding is untouched.
- Host/email configuration remains env-only. No real-looking default mailbox. Missing production contact config fails the build. The pilot email appears only in the authorized deploy runbook; test fixtures use `contact@example.invalid`.
- Public routing uses the actual Host, a path allowlist and a separate public root layout. It does not expose BFF/API or internal rewrite routes on the platform host, and does not expand the existing auth-body proxy matcher.
- Optional domain-verification tag is absent when unset; when set it renders exactly once on each locale's apex home page, never a legal page or admin page.
- Caddy platform block and www 301, compose/admin build arguments, preflight, smoke fixture and runbook are delivered. No migration, product Go/SQL, production configuration, DNS, Meta settings, mail delivery, push, merge or deployment.

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
| `e360e6b8` | FIX FIRST: actual Meta disclosures, exact UI labels, normalized public Host and admin robots |
| `bb50ec5a` | FIX FIRST: known-document slash 301, Caddy fixture host, S07 synthetic build configuration |
| `3e26a969` | Supplemental S07 image red-green: install W0 workspace manifests before frozen dependencies |

All commits have `Co-Authored-By: Codex <noreply@openai.com>`. Earlier proposal/checkpoint commits remain in history; they are not the current acceptance state.

## Original PS1–PS5 evidence (historical, before this review-fix)

| Gate | Status | Evidence |
|---|---|---|
| PS1 | PASS (LOCAL) | 7 focused Node cases within the 336-case Node run; 15 rendered company/SSR checks; actual production build missing email exits 1 as required |
| PS2 | PASS (LOCAL/MOCK) | 30 page/viewport/language cases, 390 actual clicks plus 30 reloads; no horizontal overflow; real document navigation, language changes, admin links, skip link, mailto activation |
| PS3 | PASS (LOCAL Caddy TLS / MOCK upstream) | Pinned Caddy adapt/validate/fmt plus 22 requests: platform pages/static, rejected API/internal paths/POST/forwarded-host spoof, HEAD, www 301 preserving path/query, admin host |
| PS4 | PASS (LOCAL) | 16 verification-tag cases, unset and set; production-Next crawler checks |
| PS5 | **BLOCKED / NOT ALL GREEN** | Node/tsc/check-gates are green. Serial full click-sweep exits 1 on existing merchant-storefront routes: 120 pages / 32 load failures, 983 controls / 960 pass / 3 fail / 20 skip; 18 journeys pass. Public-matrix standalone result is separate, not a replacement. |

## Original commands and exit codes (historical)

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
- Full production compose startup / full preflight against real hosts: **NOT_RUN**. Local Caddy/compose and the review-fix smoke attempts are distinguished above; no live deployment acceptance is claimed.
- Full strict release gate and full G07: **NOT_RUN**; no migration/SQL/ACL change in this unit. The release catalog registration was checked.
- Optional `tests/media/r04-input-runner.test.mjs`: **NOT_RUN**, `COMMERCE_R04_LIVEKIT_BINARY` unset; test-node explicitly reports it.
- No claim that Meta will approve the business or application. Owner/legal review, deployment and integrator independent acceptance remain outside local delivery.
- Final review-fix handoff is **not full release acceptance**: full Linux/root smoke remains blocked locally, and PS5 is integrator-owned this round. The historical `8f567049` first-viewport proof remains; company facts are below the hero and approved A was not changed.
- Runners closed their owned Next/browser/fixture processes. The choice server on port 58620 was already absent (`lsof` exit 1, no listener); no unrelated service was stopped. No task-owned public runner/Next process remained in the postflight process check. No shared cache was deleted.

## Handoff stop line

1. Integrator: rerun `bash deploy/scripts/smoke.sh full` on the repository's Linux/root test runner (see `.github/workflows/deploy-smoke.yml`). This unit does not push or trigger remote CI. The old documented dind smoke has no reproducible outer runner here; no unverified dind recreation or host privilege escalation was used.
2. PS5 is not waived, but explicitly belongs to the integrator. Retain old red evidence; no sweep fixture/threshold/assertion was relaxed in this repair.
3. Owner/legal review and independent integrator acceptance remain necessary before authorized deployment. Public-site green evidence is not production or Meta review approval.
