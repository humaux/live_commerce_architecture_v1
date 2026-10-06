# G-UI8 defects found by the real-click sweep

**STATUS: all four defects are FIXED (unit ui-click-sweep, branch unit/ui-click-sweep).** They were found by `bash scripts/dev/test-local.sh --browser-click-sweep` at base `6e72a249` (r3/integration: 120 page/viewport/locale units, 792 control clicks with 4 failing, 18 journey steps). After the fixes `tests/ui/click-sweep-known-defects.json` is `[]` and the gate passes with an empty list: 120 units opened (0 with a page-load failure), 811 control clicks (796 pass, 0 fail, 15 skip), 18 journey steps (18 pass). The original evidence screenshots stay in `defects/` as the record of what a user saw before the fix.
Evidence = the screenshot taken by the sweep at the failing control; the full per-control ledger is `ledger.md` / `ledger.json` in this directory.

| id | sev | page / control | what a user sees | evidence | owner unit |
| --- | --- | --- | --- | --- | --- |
| D01-manual-order-cod-parse | **P1** | `/orders/new` (zh-TW, en; desktop, 390 px), page load | "此部署尚未開啟從後台建立訂單。" / "Creating orders from the admin is not turned on for this deployment." The whole manual-order form is gone as soon as cash on delivery is enabled for a home delivery. | `defects/D01-orders-new-cod-parse.png` | home-cod |
| D02-studio-request-storm | **P1** | `/studio` (all variants), page load | The page sits on "Loading scenes…" forever and fires `GET /api/stores/{store}/live-sessions?limit=20` about 370 times per second (2019-2173 requests in the sweep's 8 s settle window), each one aborting the previous. | `defects/D02-studio-request-storm.png` | studio-ui |
| D03-claims-refresh-silent | P2 | `/studio/claims` > "Refresh facts" / 「重新整理事實」 | The click re-reads six endpoints and changes nothing on screen: no spinner, no "updated at", no notice. | `defects/D03-claims-refresh-silent.png` | live-claims |
| D04-product-save-silent | P2 | `/products/{id}` > "Save changes" (en desktop; flaky across variants) | On an unchanged product the button sends no request and shows no message ("nothing to save"), and stays enabled: the click looks dead. | `defects/D04-product-save-silent.png` | catalog-core |

## Root causes and fixes (each with a red-first test)

- **D01 (fixed).** Root cause as suspected, plus two siblings. `apps/admin/lib/merchant-tools-model.ts` enumerated `bank_transfer|pay_at_pickup`, but Go's manual options (the buyer checkout options with card removed) list `cash_on_delivery` plus the offer (`cod_carrier`, `cod_max_minor`, `cod_surcharge_minor`) on a COD-enabled home row, so one unknown mode made `parseManualOptions` refuse the whole list. Go itself still refused the mode it advertised (`ValidateManual` allowed only the two old modes: 422 `invalid_request`), and the result parser knew only AWAITING_TRANSFER/CONFIRMED. Fix: Go accepts `cash_on_delivery` (Place already restricts it to the chosen row's modes, begin_hold applies the whole-TWD total, the cap and the open-order limit) and forwards the offer fields; the admin parser accepts exactly that shape (COD only on a home row, whole-NT$ cap up to 20,000 and fee up to 1,000, offer keys only with the mode), the result state per mode (COD = AWAITING_COLLECTION), a COD radio with the fee/cap/carrier note, the cash due at the door on the result, three COD refusal texts in all locales, and the hold time is shown for bank transfer only. Contract `storefront-v2.md` G3 amended. Tests: `tests/admin/merchant-tools-model.test.ts` (red before), `TestValidateManualAcceptsCashOnDelivery` (red before), and the real-click gate `--browser-manual-order` now creates a COD order through the UI and opens its link in a fresh browser.
- **D02 (fixed; the suspected effect was not the cause).** `Studio.tsx` `clear` closed over `scope` and `detailKey`; `detailKey` carries the selected scene id, which comes from the page data, and `loadPage` resets that data on every run. Page data arriving changed `clear`, which re-created `loadPage` (it depends on `clear`), whose effect reset the data again: an endless loop whenever the store has a scene and the URL has no `?scene=` (every other Studio test passes `?scene=`). Fix: `clear` reads both from the latest render through the existing `volatile` ref, so it has one identity. Test: `STU05` in `tests/admin/studio-ui.spec.ts` (red before: the first scene never opened) counts `live-sessions` GETs over four reloads (<= 6 each).
- **D03 (fixed).** The Refresh button only re-ran the six reads. Now it shows a busy label and is disabled while they run, then "Facts refreshed at hh:mm:ss" (Taipei time, `displayClock` in `packages/format`). Test: `tests/admin/claims-ui.spec.ts` (red before: the button stayed enabled), board read slowed only to make the busy state observable.
- **D04 (fixed).** `Basics.submit` returned silently when nothing changed. It still sends nothing (no version bump) but answers "No changes to save." / 沒有變更。 / 没有变更。 in a neutral notice. Test: `tests/admin/catalog-core.spec.ts` (red before) asserts the notice and that no PATCH is sent.

## Found while fixing the five red modes (not in the original four)

- The W0 registry guarded `/orders/new` with `orders:write`, a permission that exists nowhere in Go or SQL, so every role except the owner got the shell's 403 on the manual-order page. It now names `inventory:reserve` (what Go guards a manual order with, migration 0094); `tests/admin/shell-registry.test.ts` checks that every route permission is one Go or SQL knows.
- Not fixed, for the W0 owner: a signed-in role without `orders:read` (for example marketing-only or catalog-only) lands on Overview and sees the shell's 403 with a "Back to overview" button that leads to the same 403. The browser gates were adjusted to the real role contract instead.

## Not defects (classified while building the sweep; recorded so nobody re-raises them)

- Visually hidden "skip to content" links (admin shell and storefront) cannot be hit by a mouse by design; the sweep reaches them as a keyboard user does (Tab, Enter) and asserts the `#main` hash.
- A control that is already the current item (current wizard step, selected radio card, active tab, link to this very URL) changes nothing when clicked again; the ledger says "already the current item".
- `/order-link` and `/claim` opened without a token show their refused-link alert by design; `/orders/cvs-print` hands the browser to ECPay (the sweep's network edge answers a stub and asserts the attempt).
- `/live` (ui-architecture section 4, S6) has no page in this base: nothing to click; the ledger has an explicit skip row.
- Native `<select>` controls with a single option cannot be changed by `selectOption`; the ledger records them as skipped ("select has a single option").

## Known coverage limits of this gate (NOT_RUN, not hidden)

- No card-paid order exists in this stack (needs the Stripe fake worker): the refund panel and payment panels are not swept here (`--browser-refund-fulfilment`, `--browser-payment`).
- Real FB/IG dialogs, real PSP and carrier pages, WebKit/iPhone Safari (`--browser-webkit`) are outside this MOCK gate.
- Alike controls (for example 24 identical order rows) are sampled, 3 per class; every distinct control is clicked and the ledger names the class size.
