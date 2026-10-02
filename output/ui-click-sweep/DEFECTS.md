# G-UI8 defects found by the real-click sweep

Found by `bash scripts/dev/test-local.sh --browser-click-sweep` at base `6e72a249` (r3/integration). Last run: 120 page/viewport/locale units opened (23 admin registry routes + 17 buyer routes, each at 1586x992 zh-TW, 390x844 zh-TW and 1586x992 en), 792 control clicks (773 pass, 4 fail, 15 skip), 18 journey steps (18 pass); failures = 10 rows, all of them the four known defects below, 0 new, 0 stale. Product code was NOT changed (brief). The gate stays red until each entry
below is fixed and removed from `tests/ui/click-sweep-known-defects.json` (an entry that no longer reproduces fails the run as "stale" unless it is marked `flaky`).
Evidence = the screenshot taken by the sweep at the failing control; the full per-control ledger is `ledger.md` / `ledger.json` in this directory.

| id | sev | page / control | what a user sees | evidence | owner unit |
| --- | --- | --- | --- | --- | --- |
| D01-manual-order-cod-parse | **P1** | `/orders/new` (zh-TW, en; desktop, 390 px), page load | "此部署尚未開啟從後台建立訂單。" / "Creating orders from the admin is not turned on for this deployment." The whole manual-order form is gone as soon as cash on delivery is enabled for a home delivery. | `defects/D01-orders-new-cod-parse.png` | home-cod |
| D02-studio-request-storm | **P1** | `/studio` (all variants), page load | The page sits on "Loading scenes…" forever and fires `GET /api/stores/{store}/live-sessions?limit=20` about 370 times per second (2019-2173 requests in the sweep's 8 s settle window), each one aborting the previous. | `defects/D02-studio-request-storm.png` | studio-ui |
| D03-claims-refresh-silent | P2 | `/studio/claims` > "Refresh facts" / 「重新整理事實」 | The click re-reads six endpoints and changes nothing on screen: no spinner, no "updated at", no notice. | `defects/D03-claims-refresh-silent.png` | live-claims |
| D04-product-save-silent | P2 | `/products/{id}` > "Save changes" (en desktop; flaky across variants) | On an unchanged product the button sends no request and shows no message ("nothing to save"), and stays enabled: the click looks dead. | `defects/D04-product-save-silent.png` | catalog-core |

## Root-cause notes (for the owner units; not verified by a fix)

- **D01.** `apps/admin/lib/merchant-tools-model.ts:92` `manualPaymentModes = ["bank_transfer", "pay_at_pickup"]`. `GET .../tools/orders/manual/options` (Go) now lists `payment_modes: ["bank_transfer","cash_on_delivery"]` for a COD-enabled home service (migration 0107); `parseManualOptions` rejects the unknown mode, `readManualOptions` fails as "unavailable" and `ManualOrder.tsx` renders `notConfigured`. The CVS options are fine; one unknown mode on one option disables the page. Fix: add `cash_on_delivery` to the model and the manual form's payment choices (and keep a per-option parse so one bad row cannot blank the page).
- **D02 (suspected cause, not fixed).** `apps/admin/components/Studio.tsx`: the history-recovery effect `useEffect([scope, scene, locale, storeID, cursor, initialError, pageReload, reveal])` calls `reveal()` -> `setPageReload(v => v+1)` / `setDetailReload(...)`, and `pageReload` is one of its own dependencies; `loadPage` aborts the in-flight read each time. Measured in a fresh context on a store with one draft scene (`/en/studio?store=...`): 2137 requests, 1424 aborted, in 6 s. It reproduces on most but not all loads (marked `flaky`).
- **D03.** `apps/admin/components/StudioClaims.tsx:464` the `studio-refresh` button only re-runs the six reads (`disabled={!!busy}` is its only state) and renders no loading state or result line. Add a visible busy state while the reads run and a "refreshed at hh:mm" notice.
- **D04.** `apps/admin/components/ProductEditor.tsx` `Basics.submit`: `if (Object.keys(body).length === 1) return; // nothing changed: no request, no version bump` returns silently. Either disable the button when nothing changed or show a "nothing to change" notice.

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
