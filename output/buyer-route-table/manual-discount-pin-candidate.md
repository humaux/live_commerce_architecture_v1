# FEAT-MANUAL-DISCOUNT pin candidate: buyerhttp.Routes

Candidate only; pending independent review, not approval. Authority: integrator's PIN card (2026-11-01),
`docs/engineering/contractdrift.md`, and the original source/CI receipts below. PR #394, branch
`unit/feat-manual-discount`, target `r3/integration`. Author: Codex implementation lane; model/reasoning
configuration is not exposed by this runtime. No delegation or independent reviewer is claimed.

## Base and hash

- Fetched and merged `origin/r3/integration` at `fde11c1310fce20b576fc98023bd3003c66c205b`; merge exit 0,
  Already up to date. Source HEAD before this pin-only commit: `3a4e5f724edf9302766258f22a75d1dc8227758f`.
- Old dispatcher hash: `bfbb4441b77afb0b48f0c3bd0c326b75e276a528fa54e588a645cbe0f814c43a`.
- Candidate dispatcher hash: `a8b1feb94a2439eb02b09a2d42e10c0e273abacfd53dc31708725147d9a2590a`.
  This is the actual hash printed by [Gates run 41275855308, plan job 124757772578](https://github.com/humaux/live_commerce_architecture_v1/actions/runs/41275855308/job/124757772578),
  whose `headSha` is the source HEAD above. No local scanner/test was run. Integration introduced no newer
  buyerhttp source; the closure source is unchanged from that CI head.
- `reviewed`: `pending independent review (FEAT-MANUAL-DISCOUNT adjustments projection)`.
- Function list: 82 -> 82; declaration list: 151 -> 151. Added functions/declarations: none. Removed: none.
  Both lists remain byte-for-byte unchanged and sorted. `pricing.Adjustment` is imported, so it does not
  introduce a same-package closure member. No other pin entry changes.

## Whole closure delta, declaration by declaration

`git diff fde11c1310fce20b576fc98023bd3003c66c205b -- internal/buyerhttp` has exactly these two changes,
both in `internal/buyerhttp/projections.go`; all other package files are unchanged.

| Frozen declaration | Current file:line | Exact change | Required reason |
| --- | --- | --- | --- |
| `type orderQuoteResponse` | `internal/buyerhttp/projections.go:285` (new field :290) | Adds `Adjustments []pricing.Adjustment` with JSON tag `adjustments,omitempty`; existing fields only realign under gofmt. | `contracts/cart-quote-v1.md:205` requires buyer order/link GETs to retain the frozen `snapshot.quote.adjustments`. The closed DTO previously dropped it; empty slices retain omission. |
| `func projectOrder` | `internal/buyerhttp/projections.go:366` (assignment :382) | Adds `Adjustments: quote.Adjustments` beside the existing promotion projection. | Copies the stored quote adjustment without repricing, taking client money, or exposing merchant authorization fields. No other statement changes. |

Every other function/declaration already listed under `buyerhttp.Routes` in the pin is unchanged.
The imported type is defined at `internal/pricing/manual_discount.go:22`: kind and currency plus embedded
`ManualDiscount` (:17), containing name and amount_minor only. It has no staff identifier, internal reason,
customer contact, or credential. Its implementation is outside the same-package freeze and remains subject
to the normal feature review and CI gates, rather than this pin's approval.

**No URL, method, auth or tenant-scope logic in the closure changed.** The full buyerhttp diff above contains
only an output DTO field and its assignment. `routes.go`, `routeRows`, `matchRoute`, `matchPattern`, `allowed`,
`handler.ServeHTTP`, `handler.dispatch`, `handler.authorizeView`, `viewAllowed`, `bearer`, `forbiddenInput`,
origin resolution and all other closure declarations are unchanged. There is no route addition/removal.

## Reviewer readback (no local runtime verification)

Read the entire current closure, not only this delta, before approving. Its exact 82 functions and 151
declarations are enumerated in `contracts/route-table-dispatchers.json` under `buyerhttp.Routes`.

```sh
git diff fde11c1310fce20b576fc98023bd3003c66c205b -- internal/buyerhttp
git diff 3a4e5f724edf9302766258f22a75d1dc8227758f -- contracts/route-table-dispatchers.json
git ls-files 'internal/buyerhttp/*.go' ':!:internal/buyerhttp/*_test.go'
gh run view 41275855308 --log-failed | grep -m 1 'buyerhttp.Routes'
gh pr checks 394
```

Source binding: `internal/buyerhttp/projections.go` Git blob `16c1f89d9d05b99b0fbb460ac953e4f4785d19b5`,
SHA-256 `68d5cd2348fcdacfcec0314fac5698272041cdb17b0511d6030543a09b51e6e1`.
Candidate pin file SHA-256: `081463c1e82f8dfb2fc9571df7f10c0f4ca3c6c41826b1f4c0f228b5878c0b29`.
Static generation/check of browser ownership and packet structure exit 0 (E1); ownership artifact has no diff.
Existing Foundation run 41275855282 / job 124757772556 fails `TestGoRepositoryInventory` with missing buyer
table rows (E3 negative, same pre-pin source HEAD). Current candidate GREEN is pending its own PR CI.
Only the integrator may change `reviewed` after independent whole-closure approval.
