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

## Deleted cases (reason)
- BPU01 "lost handoff is permanently GET-only" / "marker replacement before/after Take" / "lost prepare replay on PAYUNi": PAYUNi one-shot `TakeHosted` marker (`handoff_started`) no longer exists; replaced by the legacy-marker-is-never-replayed test (Stripe replay semantics stay covered by SU03).
- settings-real.spec.ts stale-baseline segment (method version vs second provider account, "Connect account"/"Configure methods" steps) and `provider-accounts` secret-retry mismatch (409) segment: both only existed for PAYUNi account/method binding; policy/service baseline-conflict hold tests remain.
- BPT03 `prepared receipt enforces exact original credit-card amount` (PAYUNi TWD 100–19,999,900): replaced, not dropped (Stripe table + removed-code rejection).

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
2. Phase 2 must remove the still-live pieces this unit deliberately left: Go `payuni` package + `COMMERCE_PAYMENT_NOTIFY_URL`, BFF `app/api/stores/[store]/[...resource]/route.ts` (`provider-accounts`, `payment-methods`, `/inspect` allowlist), `tests/admin/auth-real.spec.ts` provider-account/payment-method transport checks (lines ~337-540), `tests/foundation/**`, `contracts/*payuni*`, `scripts/dev/payuni-wire-check.mjs`, `scripts/dev/test-local.sh` `payuni-baseline` step (`tests/storefront/payuni-ui-baseline.mjs`).
3. Generated docs not hand-edited (CI drift check): `docs/architecture/DEPENDENCIES.md`, `docs/engineering/dependency-map.md`; `docs/engineering/dependencies.md` still lists the PAYUNi package row. Regenerate/edit in phase 2 when the package is deleted.
4. Other docs still mention PAYUNi as history and were not given a status line (outside the briefed set): `docs/delivery/**` other units, `docs/implementation/2026-09-25-*`, `2026-09-27-*`, `account-credential-configuration.md`, `docs/handoff/**` (only `.patch` files; editing would corrupt them).
5. Deploy: existing `api.env` files carrying `COMMERCE_PAYUNI_NOTIFY_ENABLED` now fail preflight P06 (unknown knob) until the line is deleted (documented in deploy/README.md). `COMMERCE_PAYMENT_NOTIFY_URL` value changed to `/payment/notify` (no receiver either way). The generic `payments-live` REAL MONEY WARN stays: it is now about Stripe LIVE only; there was no PAYUNi-specific WARN other than the removed tripwire.
6. A buyer browser still holding a legacy PAYUNi payment marker in localStorage gets `uncertain` on the payment page and cannot start a new Stripe attempt for that order until storage is cleared; the marker is unparseable by design (no PAYUNi string may remain in source). Acceptable only because no PAYUNi payment can be issued any more; revisit if phase 2 wants an explicit purge.
7. A pending uncertain `provider-accounts` journal in a merchant's localStorage is silently discarded (its write cannot be retried to anything meaningful).
