# Buyer owned history and explicit next purchase v1

Base: `1b35842`. Scope: active buyer capability; approved B detail surface.
No guest identity renewal, PSP write, production change or new transaction engine.

## Read contract

- `GET /v1/buyer/orders?limit=1..100&cursor=<opaque>` (default 50), exposed
  only through the existing same-origin buyer BFF as `orders`.
- Response `{items: OrderSummary[], next_cursor: string}`. Summary fields:
  `order_id`, `created_at` (RFC3339), `cart_id`, `cart_version`,
  `commercial_state`, `fulfillment_state`, `currency`, `total_minor`.
- Newest first by `(created_at DESC, id DESC)`, bounded keyset with scope-bound
  canonical cursor (collection + tenant/store/owner). Cursor is a position, not
  authorization. Reuse checkout pool, buyer.WithScope, RLS and final capability
  check. Never return destination, recipient, bearer, worker/receipt IDs in list.
- Existing owned order GET additionally exposes `cart_id` and `cart_version`
  from its immutable quote. Detail retains existing scoped PII projection.
- Add only the owner-history index; no identity or order schema redesign.

## Explicit continuation

- The UI has a read-only history disclosure and a separate **Continue shopping**
  action for the known current order. Viewing an old order never changes the
  current checkout locator or cancels orders/holds.
- The action shares `commerce-purchase-write-v1`. Any pending non-continuation
  intent, especially unknown checkout, rejects continuation.
- Resolve the current locator by owned GET, then read current cart. Equal
  cart/version: reuse existing idempotent `PUT cart` with empty items and that
  exact expected version. Higher current version: preserve the newer cart.
  Different ID or lower version: fail closed. Never delete/recreate a cart.
- Persist a non-PII `next-cart` journal (same five envelope fields, CartWrite
  body with empty items) before mutation. Retry the original key/body, never
  synthesize a new checkout. A resolved CAS conflict must read a higher current
  version before releasing the old locator. Network/5xx leaves recovery pending.
- After authoritative newer-cart read, forget session quote, remove and verify
  removal of current-order locator, then clear the journal. Crash at any step
  remains recoverable. SQL order history survives removal of the convenience
  locator; it is not represented by local arrays of orders or PII snapshots.
- Other tabs recheck shared pending/locator/context and discard obsolete UI.
  An expired session with known order or continuation journal still cannot reset
  to hide unresolved work. Expired guest recovery needs a separate proof-of-owner
  contract; names, phones and order IDs are not authentication.

## Acceptance gate

Real Next -> Go -> isolated PostgreSQL, not browser mocks alone:
1. Same buyer makes A, explicitly continues, makes B; each order/hold/job/receipt
   exactly once; A's original checkout key still returns A; snapshots unchanged.
2. Lost continuation response, reload, two tabs and storage failures preserve
   original key/body and never clear a newer cart. Unknown checkout cannot continue.
3. History latest-first, exact pagination, owner/store isolation, malformed and
   foreign cursors denied, revoked/expired capability denied, no PII in summary.
4. History views do not mutate checkout pointer; three locales, desktop/mobile,
   accessible list controls, empty/loading/error and navigation back work.
5. Existing 15-case order gate and 13-case purchase gate remain valid. Independent
   source/test and bounded visual review before declaring this increment accepted.
