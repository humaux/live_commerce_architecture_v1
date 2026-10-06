# fix-inbox-test-isolation

Status: DONE. Code commit 6322637560ba5fa9af37a05eb696719da3bba4da (base 161d5d34). Evidence level: MOCK-free real-PG focused run (E3, bound to that SHA).

## Root cause
`TestLiveConsoleInboxLCN10TakeoverExpiryCustomerLink` and `TestLiveConsoleInboxCrossStoreIsolation` (and latently `LCN03PermissionSplit`) looked up a freshly seeded conversation in page 1 (limit 50) of `social.list_conversations` on the shared fixture store f.storeA1. Seeded rows have no timestamps, so last_at = -infinity and they sort by `c.id DESC`; once other tests leave >50 conversations on that store, the new one's random id falls off page 1.

## Fix (test only)
`lcOwnStores` seeds a private tenant + 2 stores; the three tests use it (CrossStore uses two such tenants). Assertions unchanged. File: tests/foundation/live_console_inbox_test.go.

## Product ordering
Not a product bug: migrations/0119_live_console_inbox.sql:325 orders `GREATEST(last_inbound_at, last_outbound_at) DESC, c.id DESC` — deterministic (red.log shows ids strictly id DESC). The "random tiebreak" in the diagnosis was the random UUID, not non-determinism. A real conversation always has an inbound timestamp; a null-timestamp row exists only in these seeds. No product change.

## Proof
- red.log: unfixed + 300-row flood on storeA1 -> 2 FAIL.
- green.log: same flood + all TestLiveConsoleInbox* -> PASS=5 FAIL=0.
- Note: with no flood, `^TestLiveConsole` batch passes (47 PASS) — the failure needs >50 conversations on storeA1 from other tests, so it is order/volume dependent.

## CI gates
None extra (focused only).
