# Merchant order reads — bounded backend acceptance

2026-09-25. **IMPLEMENTED_PENDING_FINAL_GATES**. This is not merchant UI,
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
  SQL/runner/Go prerequisite commits. Tests are independently authored, not
  counted as passing until the root repeats the final integrated tree.
- Independent review: `hosted_contract_security_review`. Contract freeze
  `9ed17103-ff27-43a3-9c4d-d593babd58dc`; SQL source
  `ca89aa52-ffe6-43aa-b8be-b3400aaa929f`; Go/HTTP source
  `5a982c61-469d-47d1-8d59-f50752451ea5`. No remaining source P0/P1/P2
  at the reviewed boundaries; this is not a substitute for runtime evidence.

## Evidence retained so far

Logs are under `/Volumes/data/output/` unless otherwise noted.

| Check | Observed result | Log |
| --- | --- | --- |
| First migration smoke | FAIL, 0027 PL/pgSQL IF/CASE syntax | `merchant-orders-migration-smoke-1.log` |
| Corrected migration + existing purchase entry | PASS, 4 top-level real PG/race tests, 4.312s, exit 0 | `merchant-orders-migration-smoke-2.log` |
| Root merchantorders/pagination/httpapi unit race | PASS, all three packages; before final empty-query-segment follow-up | `merchant-orders-root-unit-1.log` |
| Root same-package vet | PASS, exit 0; same tree as preceding row | `merchant-orders-root-vet-1.log` |
| Independent early SQL subset | PASS, 4 top-level, 9 blocked-query and 5 real-state subcases, 3.173s; not final suite | `merchant-orders-pg-subset-1.log` |
| Root final MOR focused and complete PG/race/vet | NOT_RUN | Pending final integrated test tree |

The first SQL smoke exposed a bare `CASE ... THEN` inside PL/pgSQL `IF`.
Parenthesizing the expression fixed parsing (`b371cec`); the original failure is
retained. No test was removed or weakened. Initial old-member assertions after
full setup were recognized as insufficient migration-upgrade proof; the final
gate must seed the legacy state before actually applying 0027.

## Required final gate coverage

MOR01: real forward-upgrade non-elevation, fresh onboarding and replay revocation;
exact owner/schema/column/function privileges. MOR02: two tenants, two stores and
multiple buyers, explicit token/scope boundaries and exact safe fields. MOR03:
real HTTP handler to PG, strict request metadata and filter-bound timestamp/UUID
pagination. MOR04: genuine checkout/payment/expiry transitions using local signed
provider mocks, allocation-review handling and frozen history. MOR05: witnessed
query blocking followed by permission/session changes, including empty/missing
responses, and no business mutations. MOR06: root repeat, complete regressions,
independent source/test review, docs and owned fixture cleanup.

No browser page changed in this backend increment. Earlier buyer UI evidence is
not claimed as a new merchant UI gate. No real buyer data, customer live service,
real PSP request, charge/refund or production permission grant was touched.
