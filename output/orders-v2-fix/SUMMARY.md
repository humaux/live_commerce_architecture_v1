# orders-v2-fix — PASS (local acceptance)

- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/orders-v2-fix`
- Branch: `unit/orders-v2-fix`; base `395294764e1705f8c6dfa7440277a18c20fb3659`.
- Scope: W0 orders adaptation + PT400 mapping + legacy display fallback. No AppShell, transaction writes, new migration, production, secret, push or merge changes.
- Root is sole code writer (Codex; exact root model identifier is not exposed by this runtime). Read-only explorers `/root/orders_contract`, `/root/orders_tests`, and `/root/orders_final_review` use gpt-6-luna / high; final reviewer role `explorer`. Same base/worktree, read-only scope only; no recursive delegation or overlapping write paths. Root independently runs all acceptance commands.

## Audit and progress

- Baseline evidence: integrator `output/r5-wave2/orders-fix/mou07-390-before.png`, first order y=887 at 390×844; duplicate store control and four queue rows confirmed.
- UI PASS: shell-only store selection, horizontal mobile queue with selected-tab reveal, state/refresh inside secondary filters, queue retains explicit state. Native browser 10/10 passed.
- PT400: regression first failed with `merchant order read unavailable`; mapping to `command.ErrInvalid` gives HTTP 422. Targeted unit red exit 1 → green exit 0.
- Legacy parser: canonical `—` / `unknown` accepted; arbitrary display values, full name, invalid amount and unknown filter remain rejected. Node red exit 1 → green exit 0 (4/4).
- Legacy PG: red exit 1 (HTTP 503) before changing SQL/Go projection; final focused real-PG suite exit 0, 13/13 including legacy fallback, 10k search, upgrade and worker ACL. 10k phone4 max 330.781 ms; tracking max 163.324 ms.
- Original MOU07 first-order threshold remains unchanged. Added shell A/B click+reload negative, retained-state click+reload, queue reveal and 44px measurements, 1366 width coverage.

## Verified gates — all requested commands exit 0

| Requested item | Status | Verification |
| --- | --- | --- |
| A: remove page store dropdown; shell-only selection | PASS | MOU02 + MOU03 + MOU07 real shell switches, A/B negative checks and B reload |
| A: mobile single-row queue, selected visible, 44px; state in More filters | PASS | MOU07 all three locales, native wheel/click/reload, geometry and hidden state control assertions |
| A: preserve original first-order threshold | PASS | Unchanged `<844` assertion; y values below; 1366/1586 screenshots retained |
| B1: PT400 → 422 | PASS | `red/pt400.*` exit 1 → `pt400.*` exit 0; existing HTTP invalid mapping |
| B2: per-row missing recipient/unknown delivery fallback | PASS | New PG regression exit 1 (503) → exit 0; valid row and three-row SQL counts retained; TS strict sentinel tests |
| B3: queue click retains current state | PASS | MOU07 CANCELLED → cancelled queue → reload → all queue, state and URL remain CANCELLED |

| Command | Exit | Evidence/count |
| --- | --- | --- |
| `go build ./...` | 0 | `go-build.log` |
| `go vet ./...` | 0 | `go-vet.log` |
| `gofmt -l cmd deploy internal migrations tests` | 0 | `gofmt.log`, empty; every Go source root |
| `bash scripts/dev/check-gates.sh` | 0 | `check-gates.log` |
| `bash scripts/dev/test-node.sh` | 0 | `test-node.log`, 312 passed, 0 failed |
| `pnpm --filter admin exec tsc --noEmit` | 0 | `admin-tsc.log` |
| `bash scripts/dev/test-focused.sh '^(TestMerchantOrders\|TestR2IntegrationUpgradeFromReleaseHead\|TestT06WorkerAuthorityAndFunctionACL)'` | 0 | `focused-pg.log`, 13/13, no SKIP |
| `bash scripts/dev/test-local.sh --browser-admin-shell` | 0 | `browser-admin-shell.log`; 7 registry tests + 24 matrix cases |
| `bash scripts/dev/test-local.sh --browser-merchant-orders-ui` | 0 | `browser-merchant-orders-ui.log` + `merchant-orders-playwright.log`, 10/10; final run `20261003T053230.698153000`. |
| `bash scripts/dev/test-local.sh --browser-click-sweep` | 0 | `browser-click-sweep.log`; 120 page/viewport/locale units, 827 passed controls, 0 failed, 17 control skips; 18/18 journey steps |

Test runner lock wait used `LC_TEST_LOCK_WAIT=7200`; locks were not removed or bypassed.

## Commits and evidence boundary

- `2f630382`: PT400 maps to HTTP 422; unit red → green.
- `a010546f`: legacy row display fallback; Node and real-PG red → green.
- `eda5a4c4`: W0 order controls, responsive queues, retained state, real browser regression coverage.
- This delivery commit contains the final SUMMARY, gate logs/exits, click ledgers and screenshots; implementation commits above each have the required Codex co-author trailer.
- Native shell switching lands on the selected store overview, then the real Orders navigation opens that store. The delayed old-store subcase checks full-navigation cancellation and cross-store non-disclosure, not an in-page late-response fence. Existing filter/locale late-response tests remain.
- Runtime evidence is local Chromium + real PostgreSQL/Go/Next, signed MOCK IdP and synthetic merchant data, not LIVE/provider acceptance.

## Browser evidence

- `click-ledger.json`: 40 PASS records, including native horizontal wheel/click, active tab reveal after reload, all tabs same row and >=44px, and shell A→B→reload B→A with cross-store negative checks in both directions.
- Original MOU07 `expect(row!.y).toBeLessThan(height)` is unchanged. At 390×844: zh-TW 612.7890625, zh-CN 612.7890625, en 604.7890625 (integrator baseline 887).
- Screenshots: `orders-{zh-TW,zh-CN,en}-{390x844,1366x768,1586x992}.png`; corresponding `ledger-*` and `queue-scroll-*-390x844.png`. Page horizontal overflow absent in all nine size/locale combinations.
- Red iterations retained: initial exact canonical URL helper (7/10), new-helper hydration/old read synchronization (8/10), locale screenshot transition (9/10). Final 10/10. Fixes added waits for the actual canonical route, singleton Orders nav, current table read, and translated heading; no threshold, privacy/count/state assertion or production styling was loosened to pass.
- Independent static reviewer found no P0/P1; the old-store full-navigation cancellation is recorded as an architectural coverage boundary, not a fabricated in-page late response. See Humaux research `584877b3-b66e-4b75-a954-68f551792fe7`.
- Independent visual review: six required screenshots plus en 1366×768 and scrolled mobile queue; no duplicate store selector, mobile page overflow or overlap. Layout/hierarchy/readability/touch visual score 4/5 each; touch measurements come from browser assertions, not image inference. Root additionally inspected TW mobile, en desktop/mobile and CN 1366.
- Desktop limitation: at 1366×768 the full first row requires vertical scrolling. Baseline/current structure comparison shows no added desktop control row and removes the old 16px control margin, with mobile-only queue rules; there is no paired desktop baseline screenshot, so no pixel-level before/after performance claim. The hard first-row viewport gate applies to 390×844 and passed unchanged. Native date inputs may show a platform-localized placeholder even on English pages; the existing `lang={locale}` controls were not changed.
- Full click sweep: `click-sweep/ledger.json`, `ledger.md`, `journeys.json`, `runner.log`; generated 2026-10-03T05:44:29Z. Zero new/known failures and zero stale exceptions. The 17 control skips are 14 single-option selects and 3 logout controls (logout exercised by its dedicated journey). Store switching is covered by the separate 40-entry orders ledger with two real stores; it is not inferred from the sweep's single-store fixture.
- `screenshot-dimensions.log` verifies all 21 order/ledger/queue PNGs match their declared viewport; `screenshots.sha256` supplies hashes. Generic runner output overwritten during acceptance was copied here, then its previous tracked baseline was restored; old generic failure images were not presented as current failures.

## NOT_RUN / boundaries / cleanup

- No required gate is NOT_RUN or BLOCKED.
- NOT_RUN: unrelated optional R04 media-input test when `COMMERCE_R04_LIVEKIT_BINARY` is unset; the Node runner states this explicitly. The sweep separately records `/live` as an unimplemented buyer route in this base, not silently as covered.
- NOT_RUN: production deployment, live provider/payment/carrier/Meta access, actual buyer data, live merchant workflows. No push, merge, production host or credentials touched.
- Fixture runners exited normally and cleaned their owned stacks. No surviving orders/click-sweep runner processes were found. No other task's fixtures, directories, locks or services were removed.
- Humaux decision/fix records and code links accompany the delivery; canvas final state is updated after the evidence commit.
