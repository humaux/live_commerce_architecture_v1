# Merchant order reads — bounded backend acceptance

2026-09-25. **PASS_BOUNDED_LOCAL_MERCHANT_READ** at source/test tree
`25d43035f55497a782499b6fc745fa63faec9dfa`. This is not merchant UI,
shipment handling, provider qualification or production deployment acceptance.
Frozen contract: [MOR01–06](../../contracts/merchant-orders-v1.md).
Authority/rollout: [runbook](merchant-order-reads.md).

## Source and independent ownership

- Integrator: `codex-commerce-build-20260920`, main repository. Owns migration
  0027, contract, runner and documentation; initial SQL `32d55b2`, corrected
  `b371cec`. No prior accepted migration checksum changed.
- Go author: `merchant_orders_go`, `commerce_worker`, gpt-6-sol/high,
  `/Volumes/data/live-commerce-merchant-orders-go`, frozen base `bd0b11e`.
  Commits `c7dc62b`, `beed14c`; integrated as `8528ddf`, `3b8fb92`.
  Owned only merchantorders, order HTTP/tests, minimal handler registration and
  pagination/tests. No database or frontend edits, no additional dependency.
- Independent PG author: `merchant_orders_pg`, `test_worker`, gpt-6-sol/high,
  `/Volumes/data/live-commerce-merchant-orders-pg`, base `32d55b2` plus explicit
  SQL/runner/Go prerequisite commits. Independently authored `09cca02` and
  test-strengthening `ba57152`, integrated as `0536a2c` and `e5f399f`.
  Root repeated the final integrated tests on its own disposable PG fixture.
- Independent review: `hosted_contract_security_review`. Contract freeze
  `9ed17103-ff27-43a3-9c4d-d593babd58dc`; SQL source
  `ca89aa52-ffe6-43aa-b8be-b3400aaa929f`; Go/HTTP source
  `5a982c61-469d-47d1-8d59-f50752451ea5`. Final source/test/log closure
  `3aa55596-f0eb-4bba-bbd3-21d265e6cdae`: no open P0/P1/P2 at the bounded
  reviewed scope. Earlier three test P2 findings were fixed, not waived.

## Observed evidence

Logs are under `/Volumes/data/output/` unless otherwise noted.

| Check | Observed result | Log |
| --- | --- | --- |
| First migration smoke | FAIL, 0027 PL/pgSQL IF/CASE syntax | `merchant-orders-migration-smoke-1.log` |
| Corrected migration + existing purchase entry | PASS, 4 top-level real PG/race tests, 4.312s, exit 0 | `merchant-orders-migration-smoke-2.log` |
| Root merchantorders/pagination/httpapi unit race | PASS, all three packages; before final empty-query-segment follow-up | `merchant-orders-root-unit-1.log` |
| Root same-package vet | PASS, exit 0; same tree as preceding row | `merchant-orders-root-vet-1.log` |
| Independent early SQL subset | PASS, 4 top-level, 9 blocked-query and 5 real-state subcases, 3.173s; not final suite | `merchant-orders-pg-subset-1.log` |
| Root final focused MOR | PASS, 7 top-level tests / 12 blocked-query / 6 financial-state subcases, 7.589s, exit 0 | `merchant-orders-root-focused-2.log` |
| Root complete regression, first run | FAIL, 457 PASS / 1 FAIL / 0 SKIP, foundation 283.808s, exit 1; legacy exact grant expectation omitted intentional new grant | `merchant-orders-root-full-1.log` |
| Root complete regression, final `25d4303` | PASS, 458 top-level / 0 FAIL / 0 SKIP, actual PG18, race and vet, foundation 263.909s, process exit 0 | `merchant-orders-root-full-2.log` |

Final full log SHA256:
`ae7075f1b871c388b3fdb74bfe47acad13f4fcc160082a2708e8d5fa56645b61`.
Final focused log SHA256:
`afa7f804026f67986dfb9b1fa7d2e4d1d8b7d1d4f8437868acedbedaa3065099`.
Failed full-1 SHA256:
`3e461d939df4d261ede3ca26329c2ae074a758032f14f5e802df278f52069a47`.

The first SQL smoke exposed a bare `CASE ... THEN` inside PL/pgSQL `IF`.
Parenthesizing the expression fixed parsing (`b371cec`); the original failure is
retained. No test was removed or weakened. Initial old-member assertions after
full setup were recognized as insufficient migration-upgrade proof; the final
gate seeds the legacy state before actually applying 0027 and proves no grant
backfill. The full-1 failure was only the pre-existing initial-store test's
expected permission array. `25d4303` inserts `orders:read` into that sorted array;
exact array equality and warehouse/audit assertions remain. No product-code
change or assertion relaxation was used to make the final run pass.

## Accepted gate coverage

| Gate | Observed witness |
| --- | --- |
| MOR01 | Actual pre-0027 legacy state followed by forward migration does not elevate old members; fresh owner gets grant, revoked grant is not restored on replay. Exact function ACL includes owner and runtime; hosted/buyer/worker/identity denied. Schema/column grants and RLS checked. |
| MOR02 | Real buyer/quote/stock/Begin orders across two tenants and two same-tenant stores. Foreign rows excluded; independently authorized second store sees only its own order. Forged GUC, owner and token boundaries tested. |
| MOR03 | Actual HTTP handler → PG (`httptest`, not a browser or network proxy): strict query/body/method handling, timestamp/UUID ties, bound cursors, sanitized errors and no-store headers. |
| MOR04 | Real checkout/payment/expiry functions with signed local provider mocks: pending, authorized, captured, review, expired draft and allocation failure. Original item/total/recipient/CVS history unchanged after current data changes, including leading-zero store code `017888`. |
| MOR05 | Twelve direct SQL cases first observe `pg_blocking_pids`, then revoke/expire authority before unblocking, including empty/missing/real foreign order. Tests cannot pass merely because Go's final permission fence denied the result. Read paths leave business row counts and order/inventory row fingerprints unchanged. |
| MOR06 | Independent authors and source/test/log reviewer; root final focused and full PG/race/vet repeats; failed logs retained, contract/runbook/dependency notes updated and owned fixtures removed. |

Allocation failure is **controlled fixture-owner reservation-release fault
injection**, followed by real payment-apply facts/review/work generation. It is
not evidence that the normal expiry worker releases uncertain payments.
No production data is seeded, repaired or changed by that test.

The final script reported owned fixture removal and the subsequent Docker
`livecommerce.fixture` label query returned no running fixtures. Logs and author
worktrees are retained. Go/test entities have code-graph rationale links; this
graph does not index PL/pgSQL symbols, so migration rationale remains in the
contract, runbook and recorded SQL review rather than a claimed SQL symbol link.

No browser page changed in this backend increment. Earlier buyer UI evidence is
not claimed as a new merchant UI gate. No real buyer data, customer live service,
real PSP request, charge/refund or production permission grant was touched.
