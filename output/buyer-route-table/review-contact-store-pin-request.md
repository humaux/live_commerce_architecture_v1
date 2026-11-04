# FEAT-CONTACT-STORE (PR #375) — buyerhttp.Routes pin review request

Status: **pending independent review**. The author/integrator-writer is not the approver.
Base: `origin/r3/integration` @ `f42e1e9bd3aa80318cc085fdac5f5b12bce8eaa0` (includes #394 manual discount 0220,
#395 buyer return requests 0221 and its reviewed pin `dd3843222577…`, FEAT-LIVE-REPLAY 0219, #365, #473).
Branch `unit/feat-contact-store`.

## Fingerprint

- Base reviewed SHA: `dd3843222577142f0990853a9b3a68cb59fd8205c6ce17b74af911dbf7aa0241`
  (`review-buyer-rma-pin-request.md`; reviewed `output/issue-107/review-395-35c4a9a6c.md` + `-delta.md`).
- New candidate SHA: `592b3c50a7298250c073d368757686783fa8eeec8ece8fe33eb82c3f5dc5672d`.
- Source: the candidate printed by `go run ./scripts/dev/contractdrift` itself (freeze error
  `buyerhttp.Routes 592b3c50… want dd384322…`). The function/declaration lists were read from the same
  in-tool candidate (`goTableDispatcherFingerprint`) via a temporary local print in
  `scripts/dev/contractdrift/go_route_tables.go` that was restored from a byte copy before commit
  (`git diff -- scripts/dev/contractdrift/go_route_tables.go` empty). No hand-computed hash.
- With the pin applied, `go run ./scripts/dev/contractdrift` exits 0 (`errors=0`).
- Only the `buyerhttp.Routes` entry changes; the `attribution.FeedRoutes` entry is untouched. Scanner, baseline
  and allow-lists unchanged.

## Closure delta vs base (functions 84 -> 86, declarations 156 -> 159)

| Member | Change | Notes for reviewer |
| --- | --- | --- |
| `handler.ServeHTTP` | Changed | One added branch at the head of the credential chain: `selected.kind == contactRoute` admits a guest; an `Authorization` header, when present, must parse via the existing `bearer(r)` else 401, and the token is **discarded** (`token` stays `""`, so `authorizeView` and every bearer path are skipped). BFF key, trusted-origin, method guard, ID validation and `keyFor` (contact is a keyed POST: `write` true, key required) run unchanged before it. Store comes from `h.resolver.Resolve(ctx, origin)` as for every other route. |
| `handler.dispatch` | Changed | One added early return `if selected.kind == contactRoute { return h.contactPost(ctx, w, r, storeID, key) }`, placed before #395's replay early return. Both are exclusive kind checks; neither alters other routes. The `routeReturnRequest*` case from #395 is unchanged. |
| `handler.contactPost` (`contact.go`) | Added | Exact `Content-Type` (`application/json` with optional `; charset=utf-8`) else 415; body ≤ `maxJSON` else 422; `meta.ParseStrict` (dup/unknown keys refused) + all values strings + `DisallowUnknownFields` decode + `storefront.ValidContactInput`. Client IP via existing `lookupClientIP`; missing IP uses one shared hashed bucket. One `issuerPool` READ COMMITTED tx: `storefront.admit_store_contact` resolves tenant/store from the verified origin header (never body/Host/query), first call is replay lookup by SHA-256 of the idempotency key, second persists the AES-GCM envelope sealed with `(tenant, store)` from that resolution. Result store must equal the resolver's `storeID` else 404. Replay compares decrypted original vs submitted fields (409 on change). 201 `{message_id,status:"received"}` only after `tx.Commit`; no cookie, no PII in response or logs. |
| `contactDatabaseError` (`contact.go`) | Added | Maps SQLSTATE `PT404`→404, `PT422`/`PT400`→422, `PT429`→429 `rate_limited` with `Retry-After` (window remainder, via existing `codedResponse`), `PT409`→409; anything else 503 `unavailable` (no blind retry). |
| `const contactRoute routeKind = 222` (`contact.go`) | Added const | Single const; no overlap with existing ranges (iota block, 200+ CVS, 210+ transfer, 220 lookup, 221 order link, 230..233 return requests). |

Already in the base closure and only *referenced* (not changed) by the new code: `oneHeader`, `maxJSON`,
`lookupClientIP`, `bearer`, `responseError`, `codedResponse`.

## Route rows (data, excluded from hash but reviewed here)

`routeRows` = 53 = 46 frozen legacy + 2 FEAT-LIVE-REPLAY + 4 FEAT-BUYER-RMA + **1 FEAT-CONTACT-STORE**:

- `POST /v1/buyer/contact-messages` → `contactRoute` (first row; exact path, no parameters).

Union kept from base: the four `/v1/buyer/(orders/{id}/)return-requests…` rows and `GET /v1/buyer/replays`,
`GET /v1/buyer/replays/{id}`. None dropped or reordered relative to each other.

Tests: `TestRouteTableEntriesAreDispatched` count 52 → 53; `TestRouteTableMatchesLegacy` keeps the frozen
`legacyMatchRoute` oracle, #395's `returnRequestOracle`/`returnRequestOracleAllowed` and the replay spec, and adds
the contact spec only for the exact path `/v1/buyer/contact-messages` (kind `contactRoute`, POST only); its cleaning
aliases (`/`, `/extra`, `//`, `/../`) keep the legacy oracle. `scripts/dev/contractdrift/go_extract_test.go`
`TestGoRepositoryInventory` buyer rows 52 → 53.

## Cross-package boundaries (not frozen; ordinary review)

`internal/storefront` contact encryption/validation, `internal/integrations/meta.ParseStrict`, `internal/httpapi/contact.go`
(merchant list/detail/handled), `internal/notify` `store_contact` render, migration `0222_store_contact_messages.sql`
(renumbered 0211 → 0218 → 0222; restricted definers, FORCE RLS on contact tables).

## Approval

Reviewer: replace the `reviewed` string in `contracts/route-table-dispatchers.json` with
`<date> independent review (...)` only after reading the full closure above.
