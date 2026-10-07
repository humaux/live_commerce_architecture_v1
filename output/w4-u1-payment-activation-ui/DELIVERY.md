# w4-u1-payment-activation-ui delivery
- Branch/commit: `unit/w4-u1-payment-activation-ui` (final commit = `git log -1`)   Base: trunk 8464eb96, merged `r3/integration` 3807a3f8 (merge a52f3266)   Model: Kimi K3 (2 commits, quota hit) finished by Claude Sonnet 5.5
- Summary: platform-Stripe card activation page (`apps/admin/components/CardPayments.tsx`), settlement statements page (`Settlements.tsx`), BFF grammar (`lib/card-payments-request.ts`, wired in `app/api/stores/[store]/[...resource]/route.ts`), strict parsers (`lib/card-payments-model.ts`, `lib/card-payments-settlements-model.ts`), three-language copy, wizard entry link, storefront §5 collector disclosure. Money-path UI, no credential input of any kind, MOCK evidence only.
- Contract/interface changes: none (contracts/ untouched; no migration, no OpenAPI, no lockfile).

## What Kimi did (70691fb9, bcc33117)
Models + BFF grammar + copy + client + both pages + routes/shell labels (`nav:false`, correct) + wizard link + storefront contract/copy/render + 33 node tests. It could not run tsc.

## What I found and finished
1. **tsc**: admin `exit 1`, one error: `settings/payments/card/page.tsx` imported `../../customers/page-data` (one `../` short). Fixed. Storefront tsc was clean.
2. **P1, every real statement was refused** (`red-wire.log`: the Kimi parsers fail all 3 real wire bodies with `unavailable`). They treated `stripe_fee_minor`, `fee_store_minor`, `refunded_minor`, `dispute_minor` as non-negative and `closed_at` as `Z`-only. Migration 0150 makes a fee a cost (<= 0; real fixture: `-100`, `-5900`, `+5900` returned fee), refunded/dispute can go negative after a refund failure / dispute reversal, carried-in is `<= 0`, and PostgreSQL emits `+00:00`. Fixed, and the parsers now enforce the 0150 CHECKs (net = captured - refunded - dispute + fee - platform_fee + carried_in; payout only for net > 0 and equal to it; line sign by kind; detail lines add up to the totals; a payout reference shaped like `acct_/sk_/rk_/pk_/whsec_` is never shown). Real bodies come from the real handlers: `TestW4U1WireShapes` (`tests/foundation/w4_u1_seed_test.go`) seeds merchant A's statements only through the operator paths (sync -> close -> payout record) and writes the verbatim HTTP bodies to `tests/admin/fixtures/card-payments-wire/`; `tests/admin/card-payments-wire.test.ts` re-parses them (Go + PG -> JSON -> TS, E3).
3. **State machine and PUT body moved into the model** (`cardView`, `buildCardInput`, `descriptorSuffixOf`) so they are unit-tested, not buried in JSX. Behaviour changes: suffix length refusal now takes precedence over the charset message (the server answers `descriptor_suffix_too_long` first); a retained suffix is pre-filled on re-enable (the GET only exposes the final preview); `platform_stripe_blocked` / `_not_allowed` now reload the page; a statement that nets <= 0 shows "No payout due (carried forward)" instead of "Pending payout"; the period label shows the last day inside the exclusive-end week.
4. **Storefront**: `collectorDisclosure()` (pure; function replacers so a `$&` in a shop name is literal) in `lib/payment-copy.ts`; `OrderPayment.tsx` renders it with the view (above every pay button, also while a read is in flight) instead of inside the `plan` branch. Tests cover presence and absence by `collector` and the source order against `pay-order`.
5. **Gate hygiene**: Kimi's three admin test files were registered in no gate (`check-gates.sh` would fail): added to `scripts/dev/test-node.sh`; two pages lacked the `Purpose:` header; `tests/foundation/browser_click_sweep_test.go` now sets `PaymentProfile: "PROVIDER_MOCK"` (outside the unit's write paths, one line) because otherwise `payments/card` is unmounted in the sweep stack, the page answers 404 "not available" and the sweep flags `/settings/payments/card` as degraded in all variants.
6. **Browser gate** (not run locally, CI): `tests/admin/card-payments.spec.ts` (CPU1-CPU8), `tests/admin/settlements.spec.ts` (STL1-STL5), `tests/storefront/collector-buyer.mjs`, harness `tests/foundation/browser_card_payments_test.go` (`//go:build browser`), mode `--browser-card-payments` in `scripts/dev/test-local.sh` + two GATES.md rows. Seeding only through real definers: platform designate/open, allowlist, block/unblock, a competing enrollment change (`w4uControl` in `w4_u1_seed_test.go`), statements through sync/close/payout. Helper names are all new (`cpbr*`, `w4u*`); it reuses `cbbrStartAdmin`, `cbbrShots`, `brfEvidence`, `brfPlaywright`.
7. **`TestW4U1StateSequence`** (PG, runs locally): replays every server interaction of the card spec through the real HTTP handler with the exact PUT bodies the page builds (disable, enable + suffix, stale CAS 409, platform CLOSED 409, BLOCKED 403 enable / 200 disable, unblock, re-enable) and pins the audit counts the browser harness asserts (2 disable, 4 enable), plus the buyer scenarios at service level (payable order carries the collector, paid order too, primary connection none).

## Brief rules, item by item
| Rule | Where | Evidence |
| --- | --- | --- |
| no credential input of any kind | CardPayments has one checkbox and one optional suffix text field, no key/secret/account field | spec CPU1: zero `input` in the page, no password input; model tests |
| never show account id / key / approval id | grep of the new UI/lib/copy: none rendered; parsers reject unknown keys; payout reference credential-shaped values refused | `card-payments-copy.test.ts` (all copy), `card-payments-wire.test.ts`, specs scan every `/api/` body, request, DOM and storage for `acct_/sk_/rk_/pk_/whsec_`, buyer script scans the view |
| enable dialog quotes contract §5 terms verbatim + `descriptor_preview` | `cardPaymentsCopy["zh-TW"].terms` is the contract sentence byte for byte; dialog shows terms, terms version, live preview | `card-payments-copy.test.ts`; spec CPU3/CPU7 (zh-TW exact text) |
| platform not OPEN: 「信用卡收款尚未開放」, toggle hidden | `cardView.notOpen`, no enable control | node state-machine tests; spec CPU5 (zh-TW exact, en) |
| BLOCKED: 「已被平台暫停，請聯絡客服」, disable only | `cardView.blocked`, `canEnable=false`, `canDisable=true` | node tests; spec CPU6 (zh-TW exact); PG test (enable 403, disable 200) |
| storefront disclosure above the Stripe pay button and on the order payment page whenever `collector` | `OrderPayment.tsx` + `collectorDisclosure` | storefront node tests; `collector-buyer.mjs` unpaid (above `pay-order`, DOM order), paid page, primary absent, zh-TW/zh-CN/en |
| shell rule: no second `nav:true` | both routes `nav:false`, entry = wizard link | `card-payments-model.test.ts` registry test (settings group still exactly settings/team/billing), `shell-registry.test.ts` via check-gates |

## Decisions the integrator should confirm
- A store that is ENABLED while the platform is not OPEN (CLOSED keeps existing stores selling) shows 「尚未開放」 and no enable control, **but keeps a disable button**: contract §3.3 says disable is never gated, and hiding it would strand a merchant who wants to stop selling. The ruling's "hide the toggle" is applied to enable.
- The brief's "NT$25 - NT$20,000" is not hard-coded: the page shows the server's min/max (SANDBOX/MOCK TWD max is 99,999,900 minor = NT$999,999; LIVE comes from the approval). The spec asserts only the min (NT$25) and parity with the server JSON.
- Kimi's page imports `loadPage` from `app/[locale]/customers/page-data` (shared helper, unchanged).

## Tests
- Red (unchanged parsers): `node --test --experimental-strip-types` model tests -> exit 1, 13 of 20 failing (`red.log`); storefront helper test against HEAD -> exit 1 (`red-storefront.log`); Kimi parsers on the real wire bodies -> exit 1, 3 of 3 failing (`red-wire.log`). Kimi's own pre-implementation red/green kept as `red-kimi.log`, `green-kimi.log`.
- Green (`green.log`): node suites 46/46 (`card-payments-{model,request,copy,wire}.test.ts`, `payment-collector.test.mjs`); `bash scripts/dev/test-focused.sh '^TestW4U1'` -> exit 0 (`TestW4U1WireShapes`, `TestW4U1StateSequence`, 12-13 s each).
- Gates run: `cd apps/admin && pnpm exec tsc --noEmit -p .` -> exit 1 before, **exit 0**; storefront tsc -> exit 0; `bash scripts/dev/test-node.sh` -> exit 0 (`green-node.log`, r04 NOT_RUN as always); `bash scripts/dev/check-gates.sh` -> exit 0 (74 modes, headers OK, shell-registry + ui-architecture pass; `check-gates.log`); `go vet ./tests/foundation` -> exit 0; `go vet -tags browser ./tests/foundation` -> exit 0; `gofmt -l tests/foundation` -> empty; `pnpm run build:admin` (env as test-local.sh) -> exit 0, both routes listed (`build-admin.log`); `pnpm run build:storefront` -> exit 0.
- Evidence class: node = MOCK data + real PG wire fixtures; PG tests = REAL_PG + MOCK balance transactions + independent Stripe fake; browser = BROWSER MOCK (CI only).

## CI gates (run on GitHub, not on the Mac)
1. `--browser-card-payments` (new: `card-payments.spec.ts`, `settlements.spec.ts`, buyer half `collector-buyer.mjs`, runs the node suites first). Never run end to end locally: the spec/harness were type-checked (`tsc --strict` on both specs, `go vet -tags browser`, `node --check` on the buyer script) and every server interaction they depend on is pinned by `TestW4U1StateSequence`, but expect first-run selector/timing fixes.
2. Regression: `--browser-click-sweep` and `--browser-visual-lint` (new routes + the sweep options change; the visual lint has never seen these two pages), `--browser-identity` (settings wizard link), `--browser-meta-connect`, `--browser-customers-billing` (settings nav group), `--browser-payment`, `--stripe-browser` (storefront `OrderPayment` change), `--browser-webkit`.
3. `focused:^TestW4U1` (also inside the foundation shard `^Test[S-Z]`), `focused:^(TestPlatformSettlement|TestPlatformStripe)` (unchanged, regression only).

## Risks
- The browser gate is unrun (above). Most likely first-run issues: the Settings wizard step on which the entry link is reachable (`settings-card-payments-link`, CPU1), Playwright text normalisation of `money()` cells, the buyer page order-history click path (copied from refund-buyer.mjs).
- Shop name in the disclosure comes from the storefront design profile (`ShopNameProvider`, layout.tsx); an unpublished design falls back to the neutral default shop name. The buyer script checks the slot is non-empty and identical in all languages, not a specific name.
- The dialog is a `role="dialog"` section, not a focus-trapping modal (same pattern as the other admin pages).
- `dl/dt/dd` blocks (limits, statement detail) use default styles; visual lint decides.

## NOT_RUN / BLOCKED
- NOT_RUN: `--browser-card-payments` and every other browser mode (owner rule: CI), WebKit, SANDBOX (real Stripe Checkout / real balance-transaction read), LIVE anything, a real merchant-owned PSP.
- BLOCKED: none.

## Integrator to-do
- Push the branch and run the CI gates above; the first `--browser-card-payments` run is the real acceptance of the browser half.
- Confirm the "ENABLED while not OPEN keeps disable" reading and the server-driven limits (above).
- No migration, no OpenAPI, no lockfile, no contract edit needed. GATES.md rows added (`--browser-card-payments` + spec table). Regenerate the wire fixtures when `settlement`/`platformstripe` JSON changes: `LC_W4U1_WIRE_OUT=$PWD/tests/admin/fixtures/card-payments-wire bash scripts/dev/test-focused.sh '^TestW4U1WireShapes$'`.
