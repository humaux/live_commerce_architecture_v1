# B buyer payment UI v1

2026-09-25. Implementation boundary on accepted public transport `ed90f9f`.
This extends, not replaces, `buyer-payment-public-v1.md`. No provider enablement,
new dependency, payment engine, purchase journal kind or production write.

## Shared order entry

Keep approved B composition. Both current and historical owned `OrderDetails`
render one payment section and one explicit Pay/Recover original payment action.
No Pay in history rows or the product sticky footer. Existing Refresh order also
refreshes payment. Three locales retain server currency and method names. Show
test_mode from the payment snapshot even after merchant execution-profile change.
CAPTURED means trusted capture recorded, not bank settlement. Never infer payment
from order commercial state, local intent, handoff, URL, POST return or redirects.

Before Pay and before Take, match payment view order_id, currency and total_minor
to the selected immutable order snapshot. Prepared order_id/currency/amount_minor
must match too. A mismatch closes writes and requires GET/manual investigation.
Payment and commercial state shown together in the payment section come from ONE
PaymentView snapshot, not a merge of different-time GETs.

## Controller ownership

`lib/order-payment.ts` reuses buyerRequest, readPurchase, assertPurchaseContext,
pendingPurchase and frozen DTO guards. Export:

- `readOrderPayment(context, orderID): Promise<OrderPayment>`.
- `pendingOrderPayment(context, orderID): PaymentMarker | null`.
- `payOrder({context, order, locale, method, destination, isCurrent}): Promise<void>`.
  order is existing Order, method is PaymentView.methods[0] or undefined on replay;
  locale is zh-CN/zh-TW/en. destination is the synchronously preopened target below.
- `openPaymentDestination(locale): PaymentDestination` is synchronous. Interface
  has `ready(): boolean`, `submit(form: HostedForm): void`, `close(): void`.

Use the existing origin-scoped `commerce-purchase-write-v1` Web Lock. Check active
buyer context, no unresolved purchase journal, selected-screen epoch and target
readiness inside it before every irreversible step. Recheck after awaited calls.
Do not overwrite/clear purchase journal, current-order locator, address or cart.
Historical payment must not displace a newer purchase's recovery locator.

One exact non-PII localStorage record per buyer context/order:
`commerce-order-payment-v1:<context>:<orderUUID>` stores only v=1, context, order_id,
UUID key, body {method_code,method_version,locale}, stage `prepare` or
`handoff_started`. Validate exact own keys, bounds and readback; unavailable or
malformed storage fails closed. No form/token/amount/PII in persistent storage.

Persist original prepare key/body BEFORE prepare. Only an explicit next click
may replay that exact prepare (including old locale/version), never mount/focus/
reload. Re-read server payment on entry. Fresh prepare only when NOT_STARTED,
DRAFT, NONE and the selected method/version is still available. Existing prepare
marker may replay while NOT_STARTED/DRAFT/NONE or PENDING/AWAITING_PAYMENT/PREPARED.
All other states are read-only; no new key on rejection or uncertainty.

After valid prepare and all fences, persist `handoff_started` BEFORE exactly one
bodyless/keyless Take. Every error, loss or crash after that marker is GET-only;
never retry Take or return a cached form. Do not clear marker to manufacture a
retry. A server PREPARED without the original local marker is conservatively
read-only in this increment; cross-device continuation is a separate design gate.
Before submit revalidate active context, selected-screen epoch and exact owned
window/document. Form exists in memory/ephemeral DOM only, is submitted once and
removed immediately. ALREADY_ISSUED carries no form and stays GET-only.

## Native form and return

Explicit click synchronously opens a fresh about:blank child BEFORE any await;
immediately set and verify child.opener=null. A blocked/closed/unowned child fails
before prepare/Take where observable. Pin child document identity and about:blank
URL, never reuse a named window or navigate an arbitrary target. Use createElement
and textContent, no innerHTML or document.write. The blank page tells the user to
wait and keep the original store tab. Use native POST in that same child and only
the exact action/four fields accepted by HostedForm. Only close the still-owned
blank child on error; never close a provider window. No popup fallback that
silently consumes another Take. Real Chromium desktop/mobile gates required;
other actual mobile/browser engines remain an explicit coverage limit.

Storefront CSP adds exact form-action self + the two allowed PSP action URLs,
never wildcard. Child gets no-referrer policy and pinned form policy. Fixed global
`/payment/return` GET and POST emit identical neutral, no-store, tri-language HTML.
Ignore request body/query/cookies and never echo callback data, redirect, resolve
tenant or write payment/order state. It instructs returning to the original store
tab and refreshing; losing that tab does not magically recover its tenant.

## Gates

BPU01 independent Node controller tests: fresh/replay/locking/storage failure,
amount/context/screen fences, malformed form and one-shot loss/no retry.
BPU02 actual Next->Go->PG->mock PSP native POST: fresh and history, desktop/mobile,
three languages, blocked/closed/navigated popup, duplicate clicks/tabs, prepare
loss and exact replay, handoff loss + reload/focus GET-only, no persistent form,
neutral POST/GET return cannot set paid. No external provider request or charge.
BPU03 existing Node/typecheck/build + relevant PG/race/vet/order regressions;
precise CSP and neutral return headers/body tests; production direction contract.
BPU04 bounded visual audit desktop/mobile, one detector run, fresh finish review
and local built record. Global DESIGN.md and its sidecar remain unchanged.
These gates do not satisfy real-provider qualification/notify/query/capture,
merchant onboarding, production DNS/TLS or the full SaaS release gate.
