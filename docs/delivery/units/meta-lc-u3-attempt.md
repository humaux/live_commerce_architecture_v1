# LC-U3-ATTEMPT — shared merchant order-attempt form

Status: DESIGN / NOT_RUN (META-00 R7, 2026-10-28).
Purpose: let a merchant create/recover an order from conversation/comment or the standalone ManualOrder page.
Depends on: META-INBOX-V2, META-CONSOLE-V3 and both OA-3 API/pay-entry lanes accepted with frozen DTOs.
Used by: BuyerPanel/CommentStream/Inbox entry controls; this is 代建訂單, not manual fulfilment.
Invariants: server-priced order, immutable admitted intent, exact recipient/channel, explicit new-generation action.
Authority: `contracts/order-attempt-v1.md` §§2–6 and `contracts/live-console-v1.md` §5/META-00 amendment.

## Scope and write paths

- Share one versioned attempt form across ManualOrder and CreateOrderDrawer: authorized A15 prefill, selected
  origins, quantity/delivery edits, review, placement, retained recovery and separate copy fallback.
- No contact/destination prefill without an explicit authorized customer link. Placement guards include
  catalog/read/reserve permissions; no authority means no prefill request. Accepted-intent Recovery remains
  reachable under current order-read permission even when Place is disabled.
- Keep the admitted body/key/scope/generation on uncertain outcomes. Lookup before assuming absent; only proven
  terminal/NOT_ACCEPTED semantics allow the explicit next action. Closed window may create an order but cannot DM.
- Platform Stripe pay entry only; separate order-view link from payable entry, no bank-account setup. Explain
  payment timing and server total. Do not implement generation, price, window or notification authority in JS.
- Write paths: `apps/admin/components/{ManualOrder,CreateOrderDrawer}.tsx`,
  `apps/admin/lib/merchant-tools-{model,client,copy}.ts`,
  `apps/admin/app/api/stores/[store]/tools/[...resource]/route.ts`,
  `apps/admin/components/{BuyerPanel,CommentStream,Inbox}.tsx` (entry props only, serialized after prior units),
  `tests/admin/manual-order*.{test,spec}.ts`, `tests/admin/inbox*.spec.ts`,
  `tests/admin/live-console*.spec.ts`, `output/meta-lc-u3-attempt/**`.
  No Go/SQL/OpenAPI/DS changes; missing backend grammar is BLOCKED for integrator, never a legacy bypass.

## Batch RED list (GitHub CI only)

1. Real conversation click → correct authorized prefill → quantity/delivery → review → confirm → reload/detail.
   Missing permission does zero fetch; bundle without verified peer never obtains live price. Unselected claim
   changes do not invalidate the selection; truncation/selected revision mismatch blocks placement visibly.
2. Two tabs/relogin/ACK loss for one intent create one order, fixed receipt; no fresh key/generation after
   ambiguous 409/422 or transport loss. Recovery remains available after placement permission/option loss.
3. Closed window creates order with definitive not-sent result, no DM. UNKNOWN permits original readback only;
   CANCELLED committed recovery is viewable, not payable, and does not recreate resources.
4. Copy is read-only, no TTL extension/rotation; explicit rotation is separate CAS with clear consequences.
   Form and BFF reject stale session/CSRF before new admission; error responses remain private/no-store.
5. Mobile/review preserve recipient name/phone, channel, product/qty/origin, assignee, destination, totals,
   payment timing, message window and send result; picker/error/busy states recover without losing authority.

One tests-only batched RED covers form + drawer + BFF; then one implementation push. New drawer assertions
must prove current gates reach it and fail when the behavior is mutated; no screenshot/API-only acceptance.

## Acceptance gate and handoff

PR-own `--browser-manual-order` (including manual-order-link), affected inbox/console/e2e, Node/typecheck,
contract-drift and no-store/privacy guards. Real clicks on every control with visible outcome, refresh and PG
readback for durable results. **Mobile/summary parity gate:** DELIVERY.md table maps recipient, channel,
product/qty, assignee, window and send result across desktop/mobile/all summary/review states, plus required
name/phone/destination/totals/payment timing. Missing fact = P1. Three locales zh-TW/zh-CN/en, 1440/390 all states,
click ledger, exact SHA/run/job IDs and independent money/privacy/visual review. No raw codes/review scaffolding
in seller copy; Figma changes belong to separately authorized design work with its own parity/copy-lint receipt.

NOT_RUN: implementation/CI/click/recovery/parity/review until exact receipts; Meta/Stripe SANDBOX/LIVE, App Review,
real messages/payments, real-device browser and production. MOCK only; activation remains separately assigned.
