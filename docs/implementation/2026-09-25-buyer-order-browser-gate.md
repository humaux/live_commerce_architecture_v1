# Buyer address and unpaid-order browser gate

Scope: independent `test_worker`, base `438ba4c`, branch
`commerce/buyer-order-gate-20260925`, isolated worktree
`/Volumes/data/live-commerce-worktrees/buyer-order-gate-20260925`.
The runtime model/effort identifiers are not exposed to this worker; none are
invented here. The integrator owns application changes and independently reruns
this gate. The worker owns only this document and these two new files:

- `tests/foundation/browser_order_chain_test.go`
- `tests/storefront/order-gate.mjs`

The frozen boundary is `contracts/buyer-order-ui-v1.md`. Application checkpoints
imported for execution: `91ca230`, `88dc72f`, `acf3bfe`. This gate adds no
application route, credential, database migration, payment adapter or production
write. Ponytail reuse: `bhSetup`, the existing fixture's role pools, the existing
browser environment filter, installed Playwright, and the established disposable
TLS/CONNECT pattern. There is no second business implementation.

## Executable gate

```sh
bash scripts/dev/test-local.sh --browser-order
```

`TestBrowserBuyerOrderUI` requires `LC_BROWSER_ORDER_ACCEPTANCE=1` and the
existing isolated-database opt-in. The runner production-builds Next and starts
its own PostgreSQL container. Chromium submits the actual product/address/order
UI through production Next, private Go and real PostgreSQL. Catalog, quotation,
destination, checkout and order responses are never mocked.

The local test edge can hold or lose a real response after backend commit. A
separate ephemeral loopback control listener requires a random runner-only key
and provides bounded order facts, fixture publication revocation, quote expiry,
merchant service revision and the real `checkout.expire_held` worker function.
The control key is removed from Next's environment and never reaches Chromium;
Node receives no database owner URL. All products, recipients, addresses, DNS and
TLS evidence are synthetic and screenshots visibly identify the test environment.

## Assertions

The suite requires exactly 15 completed causal scenarios and six distinct order
IDs. Each buyer has exactly one order, reservation, expiry job, checkout receipt
and RESERVE ledger row. DRAFT orders have HELD reservations; the deliberately
expired order is CANCELLED with an EXPIRED reservation. Global deltas additionally
reject orphan jobs/holds/receipts and any payment-attempt/integration-operation
creation. UI totals/currency/country must equal the real quote and database row.

| Gate | Evidence exercised here |
| --- | --- |
| BO01 | Native named fields and phone input; English, simplified and traditional Chinese route/history changes preserve unsaved fields without a document reload; reload fetches current owned address and requires explicit confirmation. |
| BO02 | Expired quote and changed merchant service revision each return actual 409 and create no order; field edits invalidate confirmation; order amount/currency/country match server authority. |
| BO03 | Lost destination reply retries identical key/body in-page; reload explicitly replaces metadata intent using observed newer CAS; concurrent head replacement and a late lower-CAS write fail; expired pending quote cannot remove recovery; a new same-context tab with no session quote recovers explicitly. |
| BO03 regression | Delay the real options reply after an initial head read, confirm edited address B, then release the reply: both displayed fields and the resulting order retain B. |
| BO04 | Actual form creates DRAFT. While its reply is held, the other tab disables/removes duplicate creation; clicking its real recovery button waits on the same Web Lock, then both show one order with only one checkout POST. No disabled button is forced enabled. |
| BO05 | Lost checkout reply and locator-write failure recover after reload with the original five-field body/key. Later publication revocation preserves the uncertain intent and original owner. Real worker expiry followed by the UI Refresh order button displays current CANCELLED state rather than the historical receipt. |
| BO06 | Audit every attempted local/session `setItem`, final storage, request URLs and browser console for synthetic recipient/address values and actual cookie/bearer secrets. Another buyer's real order GET is 404 and no snapshot renders. No application page errors. |
| BO07 contribution | Production build/typecheck plus this race-enabled real browser/Go/PG test; actual desktop 1440×900 and mobile 390×844 captures. Full foundation/vet, existing transport regression, independent source and visual verdict remain separate integrator gates. |

Only intended IDs/counts/case names are serialized into `result.json`; captured
request bodies and storage-write audits stay in runner memory. No browser trace
or unredacted credential dump is produced. The test cleans its children, sockets,
certificate directory and the runner's labelled PG fixture. Evidence is retained.

## Evidence and limits

Author execution: `bash scripts/dev/test-local.sh --browser-order`, exit 0,
exactly 15 scenarios and six buyers. `TestBrowserBuyerOrderUI` 12.37 seconds;
race-enabled foundation package 13.835 seconds. Log:
`/Volumes/data/output/buyer-order-gate-agent-final.log`. Artifacts:
`/Volumes/data/live-commerce-worktrees/buyer-order-gate-20260925/output/playwright/buyer-order-2886457026`.
Additional checks: `node --check tests/storefront/order-gate.mjs`,
`go vet -tags browser ./tests/foundation` with Go 1.27.1, and staged
`git diff --check` all exit 0. SHA-256:

- Final runner log: `cf750f5eb0aa0dc856e29627aef6320b87f82893719aa6d1f934ef4a51cd8008`
- `result.json`: `76a319b6ad36822f283aef5c6e4fe41262e93e0caa82e923e57f66c1de00933a`
- `browser.log`: `c701b1c6b0dd393f7962bdb5d44e10ee27865f2ceda306ec83a40807588c3830`

Intermediate failures were retained, not reported as product acceptance: initial
recovery locators incorrectly assumed a pending marker after a definitive 409;
the quote-expiry fixture initially violated the relational/snapshot timestamp
constraint; a new tab needed ordinary opener semantics to inherit session quote
state for the duplicate case; a browser clock installed after the effect timer
could not advance that timer; and the final hold assertion initially used
RELEASED rather than the worker's actual EXPIRED enum. These were corrected by
reading the actual UI/schema and by installing the timer before page load.
No business rejection threshold or disabled UI state was weakened.

Each run emits `browser.log`, `next.log`, `result.json`, four desktop/mobile
address/order screenshots and one desktop address viewport screenshot under
`output/playwright/buyer-order-*`. The five current visual artifacts are also
copied to `.impeccable/review/buyer-order/` for the independent reviewer; full-page
sticky-footer compositing is not used as the sole desktop layout judgment.

This proves the bounded home-address to unpaid-order slice in the stated test
environment. It does not prove hosted payment, carrier pickup, live DNS/TLS,
production deployment, secure account recovery after user-erased storage, or a
subsequent independent purchase flow. Existing transport gates separately cover
stale-context response rejection; the cross-owner case here is an actual HTTP
authorization check, not a claim of exhaustive context-race coverage.
