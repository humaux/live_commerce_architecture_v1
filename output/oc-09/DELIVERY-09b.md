# OC-09b delivery: server cod_due_minor + checkout bottom bar 「到貨需付」

Role: commerce_worker (sub-agent of integrator). Model: Claude Opus 5.5. Base r3/integration @ 79f95784c, branch unit/oc09b-cod-due,
worktree .claude/worktrees/oc09b. PR #250. No migration (R2 pin untouched; 0206 not used).

## Server
- `checkout.Service.CodDue(ctx, token, storeID, quoteID)` (internal/checkout/options.go) re-reads the quote under the buyer's
  tenant/store/owner scope (storefront.GetQuote), reads `checkout.read_cod_offer` (EXECUTE already granted to the checkout runtime pool,
  0107) and returns total_minor + surcharge_twd*100, the same number begin_hold stores as total_minor + cod_surcharge_minor (=
  cod_collect_minor). `codDue` mirrors begin_hold's amount checks (post_river/0021 lines 233-249): it returns nil when the switch is off,
  the currency is not TWD or the country not TW, the total is not whole TWD, it is outside 100..2,000,000, or it is over max_twd.
- buyerhttp: POST /v1/buyer/quotes and GET /v1/buyer/quotes/{id} add `cod_due_minor` (omitempty) after `projectQuote` through
  `quoteWithCodDue`. The exact keys of `projectQuote` are unchanged, so the existing projection tests are untouched.
- Contract: contracts/buyer-http-v1.md, the "Quote workflow" projection row amended (`cod_due_minor` optional; amendment dated 2026-10-28).
  Contract drift is route-level and this adds no routes; the baseline was not touched.
- Limitation: delivery kind (home only) and the open-orders limit are not known to the quote. The option row's payment_modes and Begin
  still decide those, so the field can be present for a quote whose row is CVS. The UI shows it only when COD is the selected mode.

## Storefront
- lib/cod-copy.ts `codFooter(locale, codDueMinor, fee, money)`: label `due` (到貨需付), the server amount verbatim, and
  `（含貨到付款手續費 {fee}）` omitted when the fee is 0. It returns null for card, or when the server omitted the due.
- CheckoutFlow.tsx: the bottom bar renders codFooter from `quote.cod_due_minor` and the fee that OrderFlow reports through `onCodFee`
  (null unless COD is the selected, offered mode); otherwise the bar is as before. OrderFlow.tsx: the inline COD summary now uses
  `quote.cod_due_minor` (the old `total_minor + cod_surcharge_minor` sum is removed).
- lib/purchase.ts validQuote accepts optional whole-TWD `cod_due_minor`; lib/purchase-types.ts Quote gains the field.
- Money goes only through formatMoney via the existing `money` helper.

## #226 batch
(1) cod-footer.spec.ts Depends-on header fixed. (2) CodOrderStatus.tsx Depends-on header fixed (i18n Locale in, cvs-contract out).
(3) RED run recorded for the per-state codAmountLabel assertion (mutation commit a134f1ae0; red-09b.log). (4) Browser case for the
`label ??` precedence in home-cod-buyer.mjs: PENDING shows the approved label and not `due`; terminal mocks show `recorded`.

## Tests
- tests/foundation/oc09b_cod_due_test.go (HTTP_PG): due == the placed order's cod_collect_minor and the DB total+surcharge; GET returns
  the same value; absent when over the cap (and Begin refuses) and when COD is off; scope: another owner gets 404 and CodDue not found,
  the same token on another store of the tenant is unauthorized.
- internal/checkout/options_test.go TestCodDueMirrorsBeginHold (unit; added with the implementation, no separate RED).
- tests/storefront/cod-footer.spec.ts: codFooter per locale (COD shows, card/no-due hide, no-fee bracket), source pins.
- tests/storefront/home-cod-buyer.mjs: footer shows/hides the due per mode, quote.cod_due_minor == total + surcharge.

RED: see red-09b.log (runs 40600188972, 40600189231, 40612298275). GREEN: ids are in a PR comment (a file cannot name its own head).

## Outside OC-09 write_paths (integrator approval)
internal/checkout/options.go, internal/checkout/options_test.go, internal/buyerhttp/handler.go, internal/buyerhttp/projections.go,
contracts/buyer-http-v1.md, apps/storefront/components/OrderFlow.tsx, apps/storefront/components/CodOrderStatus.tsx,
apps/storefront/lib/purchase.ts, apps/storefront/lib/purchase-types.ts, tests/foundation/oc09b_cod_due_test.go,
tests/storefront/home-cod-buyer.mjs.

## NOT_RUN / risks
- No local tests (CI only). Local runs: go build, go vet, gofmt, tsc --noEmit (storefront) and node --check.
- 390px footer layout with the longer fee note was not inspected; the existing --browser-home-cod no-wrap/overflow assertions cover it in CI.
- `offeredPaymentModes` (lib/purchase.ts) still sums total + surcharge for display eligibility against the cap. That is pre-existing and
  not a displayed amount. It could switch to `cod_due_minor !== undefined`; left out to keep the diff small.
- Independent review: NOT_RUN. This author's PASS is E0.
