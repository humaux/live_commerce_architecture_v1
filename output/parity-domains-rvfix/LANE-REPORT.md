# RVFIX-407 lane report (push 1: tests only, RED)

| Finding | Class | Change |
|---|---|---|
| P1-1 journal after definitive failure | tests only | tests/admin/domains-parity.spec.ts: cases (a) add 409, (b) suspend 403; mock `failStatus` |
| P1-2 DNS rows by IP family | tests only | same file: cases (c) v6-first order, (d) v4-only |
| P1-3 receipts | deferred to implementation push | red.log records expected failures; DELIVERY.md / PR-BODY.md rewritten after GREEN with real run ids |

Merge of origin/r3/integration: already up to date. Browser ownership regeneration deferred to the implementation push. Nothing run locally (owner rule).

STATUS: DONE "RED pushed"
