# FEAT-BUYER-RMA (PR #395) — buyerhttp.Routes pin review request

Status: **pending independent review**. The author/integrator-writer is not the approver.
Base: `origin/r3/integration` @ `573f105d4f882e5afdc38a4a9260603c76824d04` (includes #394 manual discount
and FEAT-LIVE-REPLAY). Branch `unit/feat-buyer-rma`.

## Fingerprint

- Base reviewed SHA: `a816f2699027fc26ca5fa3b7c87258e8b809ebb3c97e24baff797f88b9e9ba30`
  (`review-manual-discount-pin.md`).
- New candidate SHA: `dd3843222577142f0990853a9b3a68cb59fd8205c6ce17b74af911dbf7aa0241`.
- Source: the candidate printed by `go run ./scripts/dev/contractdrift` itself (the freeze error
  `buyerhttp.Routes <candidate> want <pin>`). The function/declaration lists were read from the
  same in-tool candidate (`goTableDispatcherFingerprint`) via a temporary local print that was
  reverted before commit. No hand-computed hash.
- With the pin applied, `go run ./scripts/dev/contractdrift` exits 0 (errors=0).
- Only the `buyerhttp.Routes` entry changes; the feed entry is untouched.

## Closure delta vs base (functions 83 -> 84, declarations 154 -> 156)

| Member | Change | Notes for reviewer |
| --- | --- | --- |
| `handler.dispatch` | Changed | One added case: the four `routeReturnRequest*` kinds call `h.returnRequest(ctx, r, selected, storeID, token, key)`. Same argument shape as `cvsRequest`/`transferRequest`; runs after the existing BFF key, trusted-origin, capability and view-only admission in `ServeHTTP`. |
| `handler.returnRequest` (`return_requests.go`) | Added | GET: rejects query and body (`noBody`). POST create: strict `decodeJSON` into `returns.RequestInput`. POST withdraw: strict `decodeJSON` of `{expected_version}` only. All work inside existing `scoped(ctx, h.pool, token, store, ...)`; tenant/store come from the resolved buyer capability (`token`, `storeID` from server-side resolution), never from body, Host or query. Calls `returns.CreateBuyerRequest` / `DecideBuyerRequest("withdraw")` / `ReadBuyerRequests` with empty `platform.Scope{}` (no merchant authority). Errors mapped to `returns.Error` status/code, `ErrUnauthorized`->401, `ErrScopeNotFound`->404 (cross-buyer/store id indistinguishable from missing). No money movement. |
| `const routeReturnRequestCreate,routeReturnRequestDetail,routeReturnRequestList,routeReturnRequestWithdraw` | Added const block | `routeKind = 230 + iota`; no overlap with existing ranges (iota block, 200+, 210+, 220, 221). |

Not in the closure anymore: `isReturnRequestRoute` (listed in the earlier FIX2 candidate, removed by FIX5
6bfb1e642 when return headers moved into the existing buyer middleware). It was a stale entry in the
branch pin; the candidate no longer references it.

## Route rows (data, excluded from hash but reviewed here)

Added to `routeRows` (52 = 46 legacy + 2 FEAT-LIVE-REPLAY + 4 FEAT-BUYER-RMA):

- `POST /v1/buyer/orders/{id}/return-requests`
- `GET /v1/buyer/return-requests`
- `GET /v1/buyer/return-requests/{id}`
- `POST /v1/buyer/return-requests/{id}/withdraw`

`TestRouteTableMatchesLegacy` keeps the frozen legacy oracle; return paths use the independent
`returnRequestOracle`/`returnRequestOracleAllowed` (return_requests_test.go) and replay paths keep
the base's independent spec. Both sides' cases retained; none dropped.

## Cross-package boundaries (not frozen; ordinary review)

`internal/returns` buyer wrappers, migration `0221_buyer_return_requests.sql` (renumbered from 0217;
definer functions, FORCE RLS, no direct runtime table grants).

## Approval

Reviewer: replace the `reviewed` string in `contracts/route-table-dispatchers.json` with
`<date> independent review (...)` only after reading the full closure above.
