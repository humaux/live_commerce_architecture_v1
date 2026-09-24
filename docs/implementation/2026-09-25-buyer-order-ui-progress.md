# B inline address and unpaid order — integration evidence

Application checkpoints: `91ca230` (UI), `88dc72f` (recovery fixes),
`acf3bfe` (mobile amount layout). Independent test commit `341e519` was imported
as `48d6435`; `83a28e0` addresses the independent visual findings. This extends the approved B product-detail purchase surface; it
does not change the full SaaS delivery goal or authorize production writes.

## Boundaries and dependency map

| Component | Owns | Reuses / does not own |
| --- | --- | --- |
| `ProductPurchase.tsx` | Product/cart/quote UI, current session epoch, local route language, recovery dispatch, owned-order rendering and session-reset guard | `buyer-client.ts` capability/BFF; `purchase.ts` single mutation lock and exact-shape journal. No bearer, address snapshot or provider key in browser storage. |
| `OrderFlow.tsx` | Native in-memory address fields, observed head, explicit confirmation, expiry invalidation, current eligible MANUAL/home option, create-unpaid-order action | `currentDestination`/`writeDestination` for CAS and idempotency; `checkoutInput` for exact quote/cart/option/destination match; `writeCheckout` for uncertain-commit recovery. No alternate transaction implementation. |
| `OrderDetails` | Immutable order lines/destination/total plus freshly read commercial state; explicit refresh | `readOrder` and the server's owned projection. DRAFT is unpaid. A timer is not cancellation or payment evidence. |
| `order-copy.ts`, `globals.css` | Three locale copies; local address/order layout using the established navy/teal flat system | Existing system and B composition. No new dependency, schema, API, asset or global design token. |
| `tests/foundation/browser_order_chain_test.go`, `tests/storefront/order-gate.mjs` | Exact 15-case real UI acceptance and six buyers' database evidence | Existing Go/PG fixture, private roles, production Next, Playwright and disposable TLS edge; no mocked business responses. |

Language selection updates history/document language without a document reload,
preserving the in-memory draft. Any edit, focus or relevant cross-tab change
invalidates confirmation. The server remains the authority for amount, currency,
stock, ownership, service revisions and expiry. The UI never offers payment in
this increment. A pending/known order prevents session reset and duplicate
creation; secure expired-owner recovery and explicit subsequent purchases remain
separate required work, not a permanent one-order-per-buyer rule.

## Root causes resolved before acceptance

- **R1: destination recovery depended on session-only quote state.** Requoting
  could delete that locator; a new tab had none. Both paths left an unresolved
  destination command without a form. The form now supports quote-less explicit
  CAS recovery using only the validated non-PII country marker. Requote remains
  disabled/guarded until that command is resolved. No address is persisted to
  make recovery easier.
- **R2: late checkout-option loading rehydrated an older address.** A buyer could
  confirm B before the older loader reset the displayed fields to A. The initial
  owned-head read hydrates once; option completion updates only the option. A
  confirmed response also rechecks current quotation expiry. The browser gate
  delays the actual response and checks both displayed and ordered B.
- The mobile order amount could wrap within a monetary value. Its column now
  uses no-wrap/no-shrink; text still wraps normally. This is a local layout fix,
  not a design-system replacement.
- Independent Impeccable review `5fc38654-9f1e-474e-9a73-c8c2991387af` found two
  P2 presentation issues. `83a28e0` demotes persistent quotation navigation to
  an outline action and renders immutable delivery/tax/discount amounts above
  the order total. The real browser test now asserts navigation is not primary
  and each amount equals the authoritative quote. Final visual scoring is in
  the separate built design record.

Independent source findings `41760d8f-a815-4508-a4e7-c9d4068bffc7` were superseded
by bounded closure `e45c277b-4bd5-44a4-bcc8-490e0c3cbd38`: both P1 fixes resolved,
no remaining concrete P0/P1/P2 in the reviewed fix scope. Source review alone
was not used to claim browser acceptance.

## Root replay evidence

| Gate | Result | Exact log / evidence |
| --- | --- | --- |
| All storefront client tests | 26 PASS, no failures/skips | `/Volumes/data/output/buyer-order-ui-node-final.log` |
| New address/order actual UI | 15 scenarios, six buyers, PASS; race-enabled foundation 13.692s | `/Volumes/data/output/buyer-order-gate-root-independent.log`; `output/playwright/buyer-order-2076912888/` |
| Same gate after visual corrections | 15 scenarios, six buyers, PASS; foundation 14.954s | `/Volumes/data/output/buyer-order-gate-root-visual-fixes.log`; `output/playwright/buyer-order-491481612/` |
| Existing actual buyer regression | 13 scenarios PASS; foundation 8.779s | `/Volumes/data/output/buyer-order-ui-existing-regression-final.log`; `output/playwright/buyer-real-2097184543/` |
| Existing buyer regression after visual corrections | 13 scenarios PASS; foundation 9.455s | `/Volumes/data/output/buyer-order-ui-existing-regression-post-visual.log`; `output/playwright/buyer-real-4255746899/` |
| Full Go/isolated PostgreSQL regression, race and vet | 376 top-level tests PASS; foundation 150.085s; runner exit 0 | `/Volumes/data/output/buyer-order-ui-full-regression.log` |
| Typecheck / production build | PASS | `npx tsc --noEmit`; both browser runner logs include the production build |

The [independent browser gate record](2026-09-25-buyer-order-browser-gate.md)
lists precise cases, fixture authority, cleanup and retained failed runs. Root
checked the actual test code and reran it independently. Every buyer has exactly
one order/reservation/expiry-job/receipt/RESERVE row; global deltas reject orphan
state and payment/integration creation. The deliberate worker-expired order is
CANCELLED with inventory EXPIRED, not a guessed RELEASED state. Remaining DRAFT
orders have HELD inventory. All addresses, products and screenshot contents are
synthetic; no customer operation or money movement occurred.

## Visual scope and remaining release gates

The single detector pass is `/Volumes/data/output/buyer-order-ui-detector.json`.
Its only warning concerns the inherited Arial family, which is preserved rather
than replacing the approved world. Desktop/mobile address and order captures
plus a focused desktop address viewport are under
`.impeccable/review/buyer-order/`. Full-page fixed-footer compositing is not the
sole layout evidence. Independent visual review and its final disposition are
recorded separately in the built design record; this file does not pre-approve it.
The returned verdict `bb4c4c60-4bbb-42b9-a1d8-d60adb2ca3fc` is **ship limited to
F1/F2**: both listed findings resolved. It does not certify the whole surface,
payment or production. All five replacement captures were inspected; no second
detector or unrelated polish loop was run.

Still required for the full product: explicit next purchase with retained order
history, secure guest/account access recovery, trusted CVS pickup selection,
per-order hosted checkout and authoritative signed payment callbacks, real
provider sandbox acceptance, cross-border carrier integration, deployment/DNS/
TLS/rollback and performance/release gates. Local TLS and mock identities are
not production acceptance. No customer's live stream or current platform was
modified, stopped or migrated by this increment.
