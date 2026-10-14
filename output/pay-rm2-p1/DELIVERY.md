# PAY-RM2 phase 1 — remove PAYUNi from merchant admin, storefront, deploy and docs

Role: ui_worker (write), base `r3/integration` 62554448, branch `unit/pay-rm2-p1`, worktree `.worktrees/pay-rm2-p1`.
Owner decisions: 2026-10-06 and 2026-10-14 (PAYUNi removed entirely; merchants collect through platform Stripe).
Status: DESIGN/MOCK-level only. **No test, typecheck or build was run locally** (brief: CI runs on the PR). Every result below is NOT_RUN.

## Files changed
**Storefront** (`apps/storefront`)
- `lib/payment-contract.ts`: `PaymentMethodCode` = `"stripe_checkout"`; `HostedForm` and the ISSUED/ALREADY_ISSUED dispositions removed;
  `validHostedHandoff` accepts only REDIRECT/CREATING/CLOSED/UNAVAILABLE; PAYUNi amount rule removed from `validPaymentPrepared`.
  A view or handoff that still carries a PAYUNi code/disposition now fails validation (BFF -> sanitized 503, client -> invalid_response).
- `lib/order-payment.ts`: PAYUNi one-shot form branch of `payOrder` removed, `PaymentDestination.submit` removed (destination can only `navigate` to
  `checkout.stripe.com`), child-tab CSP is `form-action 'none'`; a non-Stripe view throws `request_failed` before any POST.
- `components/OrderPayment.tsx`: PAYUNi `fresh`/`recover` gates removed; only Stripe `plan` can enable the pay button, anything else stays on the existing unavailable/status rendering.
- `lib/buyer-server.ts`: BFF prepare body accepts only `method_code: "stripe_checkout"`.
- `next.config.ts`: UPP hosts removed from CSP `form-action` (ECPay logistics hosts kept). `lib/payment-copy.ts`: comment only.

**Admin** (`apps/admin`)
- `components/SettingsWizard.tsx`: PAYUNi branch/provider card, account + credential form (HashKey/HashIV), payment-method form, availability inspect, provider-account and payment-method
  loading/saving and secret-retry UI removed. Wizard is now: 1 overview (+ platform card-payments link), 2 delivery options, 3 market and delivery, 4 status. Logistics/ECPay/policy/service code unchanged.
  A leftover localStorage journal for a removed provider write (`provider-accounts…`, `…/payment-methods…`) is discarded on restore instead of locking the wizard.
- `lib/settings-copy.ts` (3 locales), `lib/settings-draft.ts`, `lib/settings-model.ts`, `lib/settings-client.ts` (`Pending.secret/context`), `app/[locale]/layout.tsx` (comment): PAYUNi types, method codes, copy and unused keys removed.

**Deploy**: `compose.yml` (egress/profile comments; `COMMERCE_PAYMENT_NOTIFY_URL` kept because Go config still requires it, path now `/payment/notify`), `caddy/Caddyfile` (`/payuni/notify` 404 block removed; default 404 covers it),
`scripts/smoke.sh` (probe path), `scripts/preflight.sh` + `env/api.env.example` (`COMMERCE_PAYUNI_NOTIFY_ENABLED` knob, allowlist entry and tripwire removed), `README.md`, `docker/go.Dockerfile` (comment).
**Docs**: `README.md`, `架构.md` (AD-PF1 line), `docs/runbooks/{deploy,backup-restore,incident,merchant-onboarding}.md` (B3/B4 -> removed by owner decision), one "Superseded" status line on top of
`docs/delivery/units/w4-0{1b,2b,3b}-payuni-*.md` and the 6 `docs/implementation/2026-09-24-*` files that mention PAYUNi.
**Tests**: see next section. New guard `apps/storefront/tests/no-payuni.guard.test.mjs` (fs walk of `apps/admin` + `apps/storefront` outside `tests/`, equivalent to
`git grep -il payuni -- apps/admin apps/storefront ':!**/tests/**'`, which is empty on this branch as verified by git grep).

## Tests converted (inverse assertions; none weakened)
- `payment-contract.test.mjs`: PAYUNi method/receipt/ISSUED/ALREADY_ISSUED cases now assert **false**; PAYUNi TWD 100..19999900 receipt range replaced by the Stripe table plus "payuni_credit never valid";
  fresh view uses `stripe_checkout`; "PAYUNi view unchanged" cases renamed to views without `cancel_requested`.
- `order-payment.test.mjs`: the 7 BPU01 PAYUNi lifecycle tests became: legacy PAYUNi view/marker/ISSUED form -> nothing sent, no marker, no form, child closed; generic fences and context-replacement tests re-run through the Stripe path;
  SU03 marker matrix gains legacy-marker (`uncertain`) rows; handoff outcomes gain legacy ISSUED/ALREADY_ISSUED rows.
- `buyer-payment-server.test.mjs`: BFF transport/CSRF/body tests use `stripe_checkout`; `payuni_credit` body is rejected before fetch; legacy view/ISSUED/ALREADY_ISSUED upstream bodies must become the sanitized 503; valid-byte-identical list is Stripe only.
- `buyer-payment-client.test.mjs`: fetch stub payload now a Stripe REDIRECT (no assertion depended on its shape).
- `tests/admin/settings-real.spec.ts` (Playwright, real clicks): wizard asserts no radio, no HashKey/account form, no PAYUNi text/storage; clicks the card-payments link and goes back; the lost-response/retry-same-key uncertainty
  scenario now runs on the market create POST; policy/service/changed-session segments unchanged.
- `tests/deploy/deploy-prep-r3.test.mjs`: tripwire test inverted: template has no PAYUNI, a stale `COMMERCE_PAYUNI_NOTIFY_ENABLED` (0 or 1) is a P06 FAIL unknown knob.

## Deleted cases (reason) — every remaining deletion is PAYUNi-only
- BPU01 "lost handoff is permanently GET-only" / "marker replacement before/after Take" / "lost prepare replay on PAYUNi": exist only for the PAYUNi one-shot `TakeHosted` marker (`handoff_started`), which is gone; replaced by the legacy-marker-is-never-replayed test. The generic parts of the old cases were restored on Stripe (round 2, below).
- settings-real.spec.ts `provider-accounts` HashKey/HashIV secret-retry mismatch (409) segment and "Connect account"/"Configure methods" second-provider-account binding segment: PAYUNi credential/account binding only. Its generic checks (stale-version conflict + explicit reload, unsaved field across locale switch, lost response without reload) were restored on the delivery-service write.
- BPT03 `prepared receipt enforces exact original credit-card amount` (PAYUNi TWD 100–19,999,900 table): PAYUNi-only rule; replaced by Stripe table + removed-code rejection.

## Round 2 (Opus review CHANGES) — evidence
- P1-3: `tests/admin/checkout-offline.spec.ts:96,:239` and `tests/admin/home-cod.spec.ts:90` no longer tick the removed provider radio (step 1 has no radio; they click "Continue" straight to the delivery/COD cards). Remaining `git grep -i payuni -- tests apps/*/tests` hits outside Go: `settings-real.spec.ts` (absence assertions), `deploy-prep-r3.test.mjs` (stale-knob rejection), `apps/storefront/tests/**` (inverse assertions + guard), `tests/storefront/buyer-payment-browser.mjs:136` CSP assertion inverted (must NOT contain the PSP host). Left for phase 2 (needs Go backend/harness change): `tests/admin/auth-real.spec.ts:337-540` (BFF transport vs Go provider-accounts), the rest of `buyer-payment-browser.mjs` (Go-driven PAYUNi form flow, run by `tests/foundation/browser_payment_chain_test.go`), `tests/storefront/payuni-ui-baseline.mjs`, `tests/payments/*payuni*.golden.json`, `tests/admin/baselines/w0/manifest.json` (historic capture), all `tests/foundation/**`.
- P1-4: `apps/storefront/tests/order-payment.test.mjs` new "BPU01 Stripe marker has exactly the six keys and no PII/redirect URL in storage" (key set = base :40, no-PII = base :47) and "BPU01 document/child loss after prepare but before handoff leaves exact replay marker" (child closed, base :82-86).
- P1-5: `tests/admin/settings-real.spec.ts`, after the "Standard delivery" readback: unsaved English name survives zh-CN→en locale switch; concurrent write → Save shows "Configuration changed", not silently rebased, `settings-reload-current` rebases, persists after reload; lost response (write commits, reply aborted) without page reload → pending banner → Retry → same Idempotency-Key + body, persists after reload.
- P2-1: `apps/storefront/tests/buyer-payment-server.test.mjs` BPT03: duplicate key nested at `methods[0].version` in the Stripe view GET is rejected 503 (base had nested `form.fields.Version`).
- NOT_RUN: nothing executed locally; selectors in the new Playwright block were copied from the existing spec and are unverified against a live page.

## Click ledger (settings-real.spec.ts, real Playwright actions; NOT_RUN)
| Page | Control | Action | Expected |
|---|---|---|---|
| Settings step 1 | (no radio / provider card) | assert absent | `getByRole("radio")` count 0, no HashKey, no account form, no PAYUNi text |
| Settings step 1 | "Card payments (platform Stripe)" link | click, goBack | URL `/settings/payments/card`, wizard restored |
| Settings step 1 | Continue | click | step 2 delivery cards, no account form |
| Settings step 2 | Continue | click | step 3 market form |
| Settings step 3 | Market code/name, Create market | fill, click (response lost) | pending banner after reload, no HashKey |
| Settings step 3 | Retry pending command | click | same Idempotency-Key + body (2 requests), delivery form shown |
| Settings | Language select | selectOption x3 | heading localized, TWD kept, no PAYUNi text |
| Settings step 3 | Stable service code / shipping / tax / policy / service | fill/select/check/click | as before this unit |
| Settings step 4 | status panel | read after save + reload | contains `home`, no PAYUNi |

## NOT_RUN
`node --test` (storefront + guard + deploy-prep), `playwright` settings spec, `tsc`/`next build` for admin and storefront, `bash -n` of preflight/smoke, `python3 scripts/check_packet.py`. No local runs per brief; the integrator's CI is the first execution.

## Risks / follow-ups for the integrator and phase 2
1. Typecheck is unrun: `SettingsWizard.tsx` lost ~1000 lines; watch for unused imports/locals under `strict`.
2. Phase 2 must remove the still-live pieces this unit deliberately left: Go `payuni` package + `COMMERCE_PAYMENT_NOTIFY_URL`, BFF `app/api/stores/[store]/[...resource]/route.ts` (`provider-accounts`, `payment-methods`, `/inspect` allowlist), `tests/admin/auth-real.spec.ts` provider-account/payment-method transport checks (lines ~337-540), `tests/foundation/**`, `contracts/*payuni*`, `scripts/dev/payuni-wire-check.mjs`, (the `payuni-baseline` step and the buyer-payment spec were retired in round 3, below).
3. Generated docs not hand-edited (CI drift check): `docs/architecture/DEPENDENCIES.md`, `docs/engineering/dependency-map.md`; `docs/engineering/dependencies.md` still lists the PAYUNi package row. Regenerate/edit in phase 2 when the package is deleted.
4. Other docs still mention PAYUNi as history and were not given a status line (outside the briefed set): `docs/delivery/**` other units, `docs/implementation/2026-09-25-*`, `2026-09-27-*`, `account-credential-configuration.md`, `docs/handoff/**` (only `.patch` files; editing would corrupt them).
5. Deploy: existing `api.env` files carrying `COMMERCE_PAYUNI_NOTIFY_ENABLED` now fail preflight P06 (unknown knob) until the line is deleted (documented in deploy/README.md). `COMMERCE_PAYMENT_NOTIFY_URL` value changed to `/payment/notify` (no receiver either way). The generic `payments-live` REAL MONEY WARN stays: it is now about Stripe LIVE only; there was no PAYUNi-specific WARN other than the removed tripwire.
6. A buyer browser still holding a legacy PAYUNi payment marker in localStorage gets `uncertain` on the payment page and cannot start a new Stripe attempt for that order until storage is cleared; the marker is unparseable by design (no PAYUNi string may remain in source). Acceptable only because no PAYUNi payment can be issued any more; revisit if phase 2 wants an explicit purge.
7. A pending uncertain `provider-accounts` journal in a merchant's localStorage is silently discarded (its write cannot be retried to anything meaningful).

## Round 3 (Opus delta review CHANGES) — retired PAYUNi browser e2e specs
Integrator ruling: both specs exercise only the removed PAYUNi buyer UI (one-shot native POST form, `payuni_credit` method), so they are retired in phase 1. Status: NOT_RUN (no local run, per brief).

| Retired | Reason | Where its non-PAYUNi assertions went |
|---|---|---|
| `tests/storefront/buyer-payment-browser.mjs` + `TestBrowserBuyerPaymentUI` (`tests/foundation/browser_payment_chain_test.go`) | Whole flow = PAYUNi `prepare`/`handoff` native POST to the UPP host, `hosted_payment_pages` form facts, `payuni_credit` method; no Stripe equivalent of the form cases | See below; flows with a Stripe counterpart (popup blocked/closed child, double click + 2nd tab, reload while sending, lost responses, replaced buyer context) were already in `stripe-browser.mjs` SU07/SU09 |
| `tests/storefront/payuni-ui-baseline.mjs` + `TestBrowserPayuniBaseline` (`browser_stripe_test.go`) + `payuni-baseline` step of `--stripe-browser` | PAYUNi-only SU05 SHA-to-SHA capture of the PAYUNi `order-payment` DOM | Nothing to move (the Stripe `order-payment` DOM is covered by SU09 screenshots in 3 locales) |

Non-PAYUNi assertions moved into `tests/storefront/stripe-browser.mjs` `su07()` (MOCK, runs on Chromium and WebKit):
- storefront CSP has `form-action` with `'self'` and names no PSP host (was `buyer-payment-browser.mjs:136`);
- neutral `/payment/return` GET and POST: 200, `no-store`, `no-referrer`, no Location/Set-Cookie, CSP `default-src 'none'` + `style-src 'sha256-…'` + `form-action 'none'`, query/body marker not reflected, facts unchanged;
- phone profile (iPhone 15 under webkit, Pixel 7 under chromium): touch capability, zh-TW `html lang`, `付款狀態/訂單狀態: 尚未付款`, `NT$25` money format with no `.00`, Pay button visible, no horizontal overflow at 390px.
Already in Stripe-path coverage (no move needed): unavailable state (SU09 row 11), popup blocked/closed, three-locale Pay/status copy.
NOT moved: (a) `payment-test-mode` disclosure in three locales: asserted on PAYUNi sandbox orders; whether the Stripe MOCK view sets `test_mode` is unverified, so it is a follow-up for the integrator rather than a guessed assertion; (b) iOS focus-zoom: the spec never asserted it (only the cvs/home-cod/offline/buyer-comms specs do).

Registry changes:
- `scripts/dev/test-local.sh`: `--browser-payment` now runs `TestBrowserStripeCheckout/^(SU07|SU09)` (MOCK, no key; `LC_STRIPE_BROWSER_ACCEPTANCE=1`, 1500s); `--stripe-browser` default steps are `node-env,sp18,su07,su09` (`payuni-baseline` step, its prepare `test -f`, and `LC_BASELINE_*` env removed); `--browser-webkit` `payment` step now runs `^TestBrowserStripeCheckout$/^SU07` (min 1 case) so the iPhone Safari pass over the buyer payment page is kept (commit 896bf2497 intent). SU09's keyboard Tab walk is NOT run on WebKit (NOT_RUN: not known to work on Playwright WebKit).
- `tests/foundation/browser_stripe_test.go`: the top-level `STRIPE_BROWSER/STRIPE_SANDBOX` skip moved into the SP18 and SANDBOX-OBS subtests, so the MOCK SU07/SU09 subtests run without a Stripe key (needed by `--browser-payment` and the WebKit step). `--stripe-browser` behaviour with sandbox env is unchanged; its prepare still exits 2 without the sandbox vars.
- `docs/delivery/GATES.md`: rows for `--browser-payment`, `--stripe-browser`, `--browser-webkit` and the storefront-shell file list updated (modes unchanged, so check-gates' mode/row parity holds).
- `scripts/dev/shard-plan.json`: unchanged. Both retired Go tests are `//go:build browser` files, which `shard-plan.mjs` excludes, so neither was listed. `check-backend-coverage.mjs`: `lc_covers` lines untouched.
- `check-gates.sh` "every tracked test file is run": the removed files are `.mjs`/`_test.go` (not `*.spec.*`/`*.test.*` tracked-file check targets); no remaining `*.spec/*.test` file referenced them.
