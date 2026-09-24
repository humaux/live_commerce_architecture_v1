# B buyer address and order UI v1

Status: protocol frozen after read-only preflight, 2026-09-25. Later bounded
implementation and real browser evidence are in
[the integration record](../docs/implementation/2026-09-25-buyer-order-ui-progress.md),
not a declaration of full checkout/payment release. Preflight memory:
`9499b780-dcf9-4c12-b717-f0bc6602f107`.
Extends the approved B inline surface, not a new wizard. Reuse the existing
buyer capability/BFF, destination CAS, Quote and checkout engine; no new DB
writer, transaction engine, provider call or client bearer. Full payment and
trusted convenience-store pickup remain required follow-up units, not waived.

## State and authority

- Extend the already-returned Quote fields (`cart_id`, `market_id`, `country`,
  `method`) and Option fields (`service_version`, `allocation_version`, `mode`)
  in the client validator. Match the current option by the quote's exact
  market/country/method and currency, never by first option or locale. Only an
  eligible MANUAL/home option can enter this address form. Other delivery kinds
  remain explicitly unavailable until the trusted pickup flow exists.
- Show the server quotation and owned current destination; an old head may be
  expired or for an earlier cart. It is recovery context, not confirmation.
  Recipient, phone and home address live only in React/JS memory. No PII, PII
  hashes, address response or order snapshot in local/session storage or URLs.
- Require explicit confirmation of the currently displayed address and total.
  Edits, product/quantity/cart, delivery or session changes invalidate it.
  Language-only changes preserve the in-memory form using the existing locale
  route with history replacement and translated local state, not a document
  reload; update the document language. Reload always requires fresh address
  inspection/confirmation from the server.
- POST checkout takes exactly the existing five IDs/versions. The server
  determines all prices, country eligibility and stock. Read `orders/{id}` after
  any receipt; render immutable order details plus current commercial state.
  DRAFT means unpaid, not payment success. Do not expose a payment button until
  the provider checkout path is actually implemented and accepted.

## One coordinator and non-PII recovery

Extend the existing `commerce-purchase-write-v1` Web Lock and same-context
purchase journal, not a parallel mutation coordinator. Validate all stored
shapes exactly and fail closed on malformed/unavailable storage.

### Destination

Persist only the idempotency key plus cart/head versions, home kind and country
before PUT. Keep the exact address body in memory; an in-page retry reuses that
same key/body. No automatic fresh-key retry and no persisted address body.

After reload, a pending destination marker cannot be replayed from stored PII.
GET the current owned head and require explicit user reconfirmation. The new
intent uses the observed head's version and a new key, not an automatic replay
of the unknown command. The observed version must not regress below the pending
marker. Replacement is recorded before send under the same lock. Older writes
have equal/lower expected head versions: once the confirmed replacement commits,
they cannot overwrite it. If an older in-flight write wins first, CAS rejects the
replacement; read again and require confirmation again. Do not auto-adopt a new
version on a conflict. A historical successful receipt must be followed by a
current-head read and must still match the confirmed snapshot before checkout.

### Checkout

Persist the exact five-field body and original key before POST. They contain
only IDs/versions. Retry/reload always replays that original intent. A pending
checkout blocks new cart, quote, address and checkout mutations for its context.
After success, persist/read back an exact-shape same-context order locator
(order ID only) before clearing the journal; GET the order before rendering.
Every tab rechecks this locator under the same lock before a new checkout, so
queued duplicate clicks reuse the existing result rather than create new keys.
If both a locator and a pending checkout remain after a storage failure, replay
that pending checkout first and require its receipt to match the locator before
clearing it. Do not treat an unrelated existing locator as its receipt.

A parsed nonretryable denial on the first, unambiguous attempt may resolve that
attempt. A denial while recovering an earlier uncertain attempt does not prove
it never committed (for example, the publication/capability was revoked later).
Retain the pending intent and show the unresolved state; never inherit blanket
non-2xx journal clearing from ordinary cart/quote writes. Storage failures after
success also retain the pending key for replay. Neither an expired hold nor an
order redirect proves payment or authorizes a replacement order.

Session reset is not checkout recovery: do not offer automatic reset when a
pending checkout or known order exists in the expired context. Show the access
problem and preserve its non-PII locator. A new owner/context cannot read the old
order and must never be shown the old snapshot. User-erased storage and expired
guest ownership are not silently repaired by creating another order; secure
customer order-history/account recovery is a separate unresolved requirement.
The first UI increment resumes a known order; it does not silently start another
purchase. Explicit subsequent-purchase navigation retaining earlier order
locators is a required follow-up, not a permanent one-order-per-buyer rule.

## Acceptance gates

| Gate | Required real evidence |
| --- | --- |
| BO01 | Native fields, three languages and history route; switching language preserves unsaved address without local/session PII. Reload fetches owned current head and requires confirmation. |
| BO02 | Exact current option/quote/country/currency match; expired or changed quote/cart/service fails closed and requires explicit re-quote/reconfirmation. |
| BO03 | Address PUT success and lost reply; current-head reload, concurrent head replacement and older late write cannot silently replace a confirmed address. |
| BO04 | Actual form creates exactly one DRAFT order and stock hold from the authoritative total; duplicate clicks/tabs and lost response replay the original key. |
| BO05 | Reload after receipt loss/locator-storage failure recovers the same order; later 401/403/404/5xx does not erase an uncertain order request or auto-reset its owner. |
| BO06 | No buyer PII/bearer in browser storage, URL, console or test artifacts; synthetic-only fixtures; cross-context responses are ignored. |
| BO07 | Existing cart/quote/transport regressions, real PG/race/vet, typecheck/build, desktop/mobile actual browser, independent source and bounded visual review. |

Known scope: home delivery and unpaid DRAFT order are this increment. Trusted
7-Eleven/FamilyMart selection, per-order hosted payment, signed callbacks,
expiry/stock reconciliation and production release gates remain required.
No customer production, existing live stream or payment configuration is changed.
