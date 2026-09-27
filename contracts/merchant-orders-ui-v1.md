# Merchant order workspace v1

2026-09-27. **Behavior preflight reviewed; user approved C 表格原位展开.**
Independent preflight `977f18dd-da68-4cfc-9817-cb412070f2e2` identified two
concrete gaps; the final verdict scored both resolved. This is not UI approval
or MOU acceptance. Composition is the user-selected full-width table with details
expanded immediately below its selected row, not a separate side inspector.
Baseline `722bde4`. Consumes the accepted [MOR](merchant-orders-v1.md) and
[MBT](merchant-orders-bff-v1.md) contracts; neither backend acceptance proves
this page exists. Buyer **B 商品详情直接选购** remains approved and unchanged.

## Scope and reuse

Provide `/{locale}/orders` for authorized merchants to filter and page through
orders, select one, and read its frozen items, totals and delivery details.
Reuse `WorkspaceFrame`, the existing locale routes, control tokens, currency
formatter, cookie authentication and exact GET order BFF. No new dependency,
database, generic proxy, cache, state framework or second transaction model.
Add an orders navigation item without breaking existing ledger/settings routes.
Page context uses optional canonical `store` UUID, `state` (the commercial
filter below), `order` UUID and opaque `cursor` matching the accepted BFF grammar.
Reject malformed, duplicate or unknown context fields; never silently switch
store. The list requests limit10. Keeping the current cursor in the URL permits
locale changes on later pages to retain the selected order; previous-page history
may remain in component memory. Cursor/IDs are not recipient data or authority.
Do not add shipping, cancellation, refunds, exports, manual-paid buttons or
claims that provider onboarding is complete.

The C comp `.impeccable/mocks/decision/merchant-orders-inline.png` is approved;
A and B merchant comps remain unapproved. This is independent of buyer B.
Retain the existing visual world. Generated sample
labels are illustrative, not new domain states: no unsupported refunded/shipped
label, global order count, sales-channel field or live-ready claim may ship.
Show page item count rather than pretending a cursor response supplies a total.

## Identity, transport and privacy

- Resolve merchant session server-side using the existing exact HttpOnly-cookie
  path and `authenticatedStores`. A store selector is never authority. Accept
  only an authenticated listed store; reject a supplied malformed/unlisted ID
  instead of falling back to another store. Without a selector, choose the
  existing deterministic first-store behavior and display its actual name.
- Do not use `workspaceData` for order admission: it selects a catalog store
  and loads warehouses; catalog/inventory grants do not imply `orders:read`.
  Reuse settings' scoped store-selection pattern without weakening the BFF.
- Never load order data through the localhost shared fixture. Use the real
  cookie BFF; no browser bearer, buyer capability or tenant ID. List/detail are
  GET only and private/no-store. No command journal or idempotency key is needed.
- Fetch only the selected order's detail. Do not prefetch recipient PII for
  every list row or store response bodies in local/session storage, IndexedDB,
  URL parameters, analytics, logs or service-worker caches. Render all strings
  as React text, not HTML; phone/address are text, not auto-generated links.
- Parse the expected DTO shape before rendering. Missing fields, unsafe numeric
  amounts, invalid states, malformed dates/IDs or mismatched requested detail ID
  fail as unavailable, never partial success. Do not invent zero values.

## Read lifecycle and honest states

- Initial, loading, empty, forbidden, signed-out, not-found and unavailable are
  separate localized states. Keep one deliberate refresh/retry action; do not
  poll or silently retry indefinitely. Disable duplicate in-flight actions.
- Use the backend commercial filter only: all/DRAFT/AWAITING_PAYMENT/CONFIRMED/
  CANCELLED. Keyset pagination uses returned `next_cursor`, not offsets or a
  fabricated page count. Previous-page history may be kept in component memory.
  Changing filter/store clears pagination and selected detail before fetching.
- One request generation is bound to session/store/filter/page/selected order.
  Abort obsolete reads where possible and discard late responses even if abort
  loses the race. Clear old detail immediately on selection/store change;
  list/detail may not mix contexts. An older error may not overwrite new data.
- Reuse the existing session-boundary helper where appropriate, without
  treating the readable CSRF cookie as authority. Session change, logout or
  401 clears all order/recipient state and invalidates in-flight requests.
  Synchronously clear protected DOM/state and invalidate requests on `pagehide`
  before a page can be frozen; clearing only after `pageshow` is insufficient.
  Clear on hiding as well. On visibility/pageshow return, restore order data
  only after a fresh authorized BFF read, never from a prior user's cached DOM.
  Same-app
  cross-tab logout invalidation carries no PII and only clears data; it cannot
  grant access. The common merchant shell must expose the existing authenticated
  logout operation: merchants with stores render Ledger rather than Entry, so
  Entry's onboarding-only logout is not an available merchant control. Reuse the
  same endpoint, CSRF/session fence and clear-only signal; no alternate session
  mechanism or synthetic logout substitute. A 403 clears protected data and explains
  missing permission; 404 never distinguishes missing from other-store orders.
- Locale switching preserves store, selected order and commercial filter, but
  never changes currency, monetary values or permissions. Opaque identifiers
  may be route context; recipient information may not. Use the current locale
  formatter and visibly identify the time zone used for timestamps.
- Display commercial, payment, fulfillment and work state independently from
  the server projection. `READY` is not shipped, `AUTHORIZED` is not captured,
  and `REVIEW_REQUIRED` is not successful payment. `test_mode=false` with
  `NOT_STARTED` is not evidence of live-payment readiness. Do not recalculate
  historical totals from current SKU prices. Preserve pickup code `017888`.
- Delivery data is the frozen checkout snapshot. Explain that historical store
  selection/merchant attestation does not confirm current carrier eligibility.

## Acceptance gates (MOU01–06)

Current evidence: [2026-09-27 local replay](../docs/implementation/2026-09-27-merchant-orders-ui-acceptance.md).
Local read-only MOU01–06 acceptance combines the existing source/visual and
dependency receipts with root `05cb8ee` seven-case browser PASS, including real
trusted hidden/visible and fresh authorized restoration. Root full PG/race/vet
on unchanged ordinary inputs passed 696 tests. Actual history returned
`pageshow.persisted=false`: native BFCache restoration is not proved, and
privacy headers remain unchanged. Independent scope adjudication:
`f393fffb-1154-46c5-85b3-6a4f93bafd39`. No Cloud or production acceptance.

1. **Actual user flow:** browser login via signed-mock OIDC → production Next
   page → existing BFF → real Go → disposable PG18. Orders are created through
   existing business APIs. Navigate from workspace, filter, next/previous page,
   select detail, refresh and return. No route-mocked happy-path order responses.
2. **Authority/privacy:** two authorized stores and two principals; no grant,
   unlisted/foreign store/order, expired/revoked/malformed session and fixture-only
   negatives. Verify no foreign PII, private/no-store and cookie clearing;
   no order bodies in browser persistent storage or credential-bearing URLs.
3. **Lifecycle failures:** controlled delayed/out-of-order reads for selection,
   store, filter, locale, logout/login and hidden-page return. Also exercise
   same-app cross-tab logout, actual history back/forward and pageshow restore;
   assert synchronous PII removal on hide/pagehide and no reappearance until
   fresh authorized read, including a delayed old response. Record
   whether native `pageshow.persisted` was observed; a synthetic event alone
   is not proof of native bfcache restoration. Old success/error cannot repaint
   data. Exercise non-JSON, invalid DTO, network failure and retry recovery.
   Fault injection is labeled separately from real-chain tests.
4. **Financial/history truth:** actual DRAFT, pending, authorized, captured,
   sticky review, expired/cancelled and paid-allocation-failure fixtures. For the
   last case, reuse the existing explicitly labeled controlled SQL fault setup
   after business creation (reservation release/order-state mutation), then the
   real capture worker's outcome; do not call it a pure business-API path or
   fabricate the final read response. Preserve
   separate states, test indicator, frozen item/totals/address and leading-zero
   pickup codes. Reads leave business tables, inventory, events and queues
   unchanged; idle worker effects must be excluded or separately accounted for.
5. **Visual/accessibility:** approved comp at its native 1586×992; mobile390px,
   zh-CN/zh-TW/en; keyboard/focus, readable error/status, no document horizontal
   overflow or overlapping controls. Scoped tables may scroll. Minimum44px
   mobile targets. One filter/list/detail structure, no repeated CTA/KPI wall.
   Independent visual review must accept the actual screenshots.
6. **Regression/record:** strict TS, existing package build, independent source
   and test review, root focused browser replay and relevant ledger/settings/
   BFF/buyer regression gates. Record exact commands, SHA, exits and evidence;
   retain failures, clean only task-owned fixtures, update dependencies and
   Humaux graph/memory/canvas. No production acceptance claim from local tests.

## Limits and upgrade signals

This is a read-only merchant order workspace, not full order operations or whole
SaaS release. Server-side search/export/bulk actions require a separate contract
when real merchant demand warrants them. Do not add a global cache/query library
for two bounded reads; revisit only after a measured, repeated cross-page need.
