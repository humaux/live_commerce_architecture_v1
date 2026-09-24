# B inline purchase — address/order recovery client

Historical checkpoint: source/client-unit accepted at `438ba4c`, before UI wiring.
The later [UI integration evidence](2026-09-25-buyer-order-ui-progress.md) records
the native form and real browser acceptance; the client-only evidence below is
preserved at its original scope. No customer system, live stream, provider or funds were touched.
This is an increment toward the original full SaaS goal, not a reduced goal.

## Contract and dependencies

[Frozen protocol](../../contracts/buyer-order-ui-v1.md). Independent read-only
preflight: `9499b780-dcf9-4c12-b717-f0bc6602f107`. Independent source review:
`9c66d508-a92b-4a29-bcc2-e121e73558a7`, no remaining concrete P0/P1/P2 at the
reviewed boundary. No package, schema or backend API changes.

`apps/storefront/lib/purchase.ts` reuses `buyer-client.ts`'s `buyerRequest` and
`readBuyerSession`. It retains one `commerce-purchase-write-v1` Web Lock for
cart, quote, destination and checkout writes. The separate session lock is
unchanged. The browser never handles a bearer; the existing BFF/Go capability
checks, not a local journal, authorize every private request.

| Client entry | Existing private authority | Recovery rule |
| --- | --- | --- |
| `validQuote`, `validOption`, `checkoutInput` | `internal/buyerhttp/projections.go`, current cart/options/quote | Require already-returned cart/market/country/method and service/allocation versions; exact MANUAL/home/currency/cart match. Server still revalidates stock, prices, versions and expiry. |
| `currentDestination`, `writeDestination` | `internal/storefront/destination.go` current owned head, idempotent receipt and head CAS | Store key/cart/head/kind/country only. Address body stays in memory. Reload needs current-head inspection and explicit new CAS intent; no automatic version bump. Historical receipt must match current head and confirmed fields. |
| `writeCheckout` | `internal/checkout/checkout.go` permanent receipt, current quote/destination/service checks and transactional order/hold/job | Persist original key plus five IDs/versions before POST. Replay that intent; later denials do not erase an uncertain commit. Store/read back order ID before clearing the pending command. |
| `knownOrderID`, `readOrder`, `orderRecoveryRequired` | Owned `GET orders/{id}` and current capability | Every mutation sees the locator under the same lock. Read current order state after receipt, never infer payment/cancellation from a historical receipt or countdown. UI must consult the recovery guard before a session reset. |

Only cart/quote and non-PII order/destination bookkeeping can enter browser
storage. Destination and order snapshots, contact details, address hashes and
bearers cannot. Storage errors and malformed shapes fail closed. When both a
locator and pending checkout exist, receipt replay must agree with the locator
before journal removal; an unrelated locator is not a receipt.

## Evidence

- `node --test --test-timeout=10000 --experimental-strip-types
  apps/storefront/tests/*.test.mjs`: **26 PASS, 0 FAIL/0 SKIP**.
  `/Volumes/data/output/buyer-order-recovery-node-2.log`. Thirteen new tests
  cover authority fields, native address validation, every storage write's PII
  exclusion, destination lost reply/reload/late CAS/historical head, checkout
  lost reply/queued calls, later 401/403/404/5xx, malformed receipts, definitive
  initial denial, locator write/readback and journal-removal failures, and late
  changed context. These use deterministic fake fetch/storage, not PostgreSQL.
- Independent reviewer ran the first 11 new tests and six existing purchase
  tests, exit 0. Root then added per-write PII checks, explicit late CAS and
  readback/no-op removal cases; final 26-test suite above includes them.
- `npx tsc --noEmit` in `apps/storefront`: exit 0.
- `GOFLAGS=-p=1 bash scripts/dev/test-local.sh --browser-buyer`: **exit 0**,
  existing 13 real Chromium/production Next/Go/isolated PG cases; foundation
  8.698s. `/Volumes/data/output/buyer-order-client-browser-regression.log`;
  `output/playwright/buyer-real-1158718199/`. Product/cart/quote UI is real;
  destination/order in this existing test is a transport-driven flow, not a
  newly implemented address/order form.
- Joint merchant/buyer test independently passed separately; see
  [joint acceptance](2026-09-25-merchant-buyer-joint-gate.md).

The first new unit run exposed a **test-only lock stub defect**: one promise
queue serialized all lock names, deadlocking the nested session read under a
purchase lock. The stub now keeps a queue per lock name, matching Web Locks;
production lock behavior was not weakened. Only those owned hung processes were
terminated. Review independently confirmed this fix. The first successful
24-test log remains `/Volumes/data/output/buyer-order-recovery-node-1.log`.

## Required next gates

Connect the existing approved B surface to inline native address fields,
explicit confirmation and the unpaid/current-state order view. Language changes
must preserve in-memory draft/confirmation without a document reload; edits and
context/cart/delivery changes must invalidate confirmation. No automatic reset
when a pending/known order exists.

Then run BO01–07 with actual UI actions: two tabs, both lost responses, late CAS,
storage failure, access revocation, exact one order/hold/job/receipt and no PII
storage. Source review and fake-fetch tests do not substitute for those gates.
Explicit subsequent purchases with retained earlier locators, secure expired
guest/order-history recovery, trusted CVS selection, hosted per-order payment,
signed callbacks, stock expiry/reconciliation and production release all remain
required. No new payment button or blanket release authorization is implied.
