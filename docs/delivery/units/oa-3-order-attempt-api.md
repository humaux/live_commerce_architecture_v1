# OA-3 — versioned admission, recovery and order-only pay entry

Status: DESIGN / NOT_RUN (META-00 R6, 2026-10-28).
Purpose: finish public cutover after OA-1 core and OA-2 notification authority, including PLAN's API/pay-entry lanes.
Depends on: accepted META-00 and OA-2; frozen command/transport DTOs; coordinate buyer checkout PR #273 and orders
PR #272 / copy PR #267 on current integration, not their inventory-time heads.
Used by: LC-U3-ATTEMPT, ManualOrder, merchant recovery and the buyer pay-entry page.
Invariants: stable server scope/CAS, zero old-client writes, exact selected set, order-only authority, private no-store.
Authority: `contracts/order-attempt-v1.md` §§2–6, especially §4/§5.1/§6h,m; META-00 R5/R6.

## Scope and write paths

1. Merchant API lane: scope/admit/exact-key lookup/recover/terminal envelope; A15 complete selected-set snapshot,
   50 lines/5 bundles, selection revision and per-origin lines. Reject old version/missing generation with zero
   C1/effects; never fetch the latest generation to repair an old client. Same-principal recovery uses durable
   intent despite lost placement permissions; CANCELLED is a valid committed terminal result.
2. Pay-entry lane after DTO freeze: public order-only exchange/page and merchant read-only copy / explicit CAS
   rotation commands backed by OA-2. Platform Stripe pay entry only; no bank-transfer orders/account setup.
   Separate 「複製訂單查看連結」 from payable entry; neither creates full-owner authority or silently rotates.
3. Fence legacy writers before conversion; prove complete pipeline vs Begin-only vs ambiguous/erased state;
   retain exact owner/provenance, expiry and UNKNOWN evidence. Implement 90-day retention/denial-marker behavior
   and old-link adapter. Never delete history or report an ambiguous scope as NOT_ACCEPTED.
- Write paths, merchant lane: `internal/merchanttools/manual*.go`, `internal/merchanttools/order_for_buyer*.go`,
  `internal/httpapi/merchanttools*.go`, `tests/foundation/order_attempt*_http_test.go`.
- Write paths, pay-entry lane: `internal/httpapi/order_pay_entry*.go`, `apps/storefront/app/**/pay/**`,
  `apps/storefront/lib/pay-entry*.ts`, `tests/storefront/pay-entry*`, `output/oa-3/**`.
- These are sequential seams within OA-3; integrator assigns ownership before any parallel work. Shared DTO,
  OpenAPI, route composition/BFF mapping, SQL/retention helpers and old-link adapter paths need explicit integrator
  assignment before edits. No existing checkout refactor or UI form rebase (LC-U3 owns that).
- Root/post-River migrations and R2 pin: **integrator assigns**. Preserve exact count/COMMENT/ACL pins.
  `LC_ORDER_ATTEMPT_ENABLED` stays `"0"` until full OA-3 acceptance including both lanes and independent security
  review. This card is not permission to flip the deployment pin or release production.

## Batch RED list (GitHub CI only)

1. Missing/unsupported version, missing/stale generation/previous terminal reference: typed refusal, zero rows.
   Cross-principal/store lookup/recovery discloses nothing; relogin, ACK loss and terminal replay keep one order.
2. A15 >50-line/>5-bundle/incomplete selection blocks admission. Selected change rejects; unrelated claim churn
   keeps revision valid. Preserve duplicate-SKU origins and snapshot-complete counts, no client money/tenant.
3. Recovery after lost inbox/catalog/reserve authority remains independently accessible under current read
   authority; cancelled/erased terminal result never renews holds, entry or DM. Every success/error is no-store,
   including prefill/restriction/recovery and authorization recheck after lock waits.
4. Forwarded bearer reveals zero buyer name/email/phone/address/history; exchange grants one order only, ≤15min
   and capped by 72h/payment deadline. Expiry/cancel/rotation/redemption/old-link states and rate limits hold.
5. Copy never extends TTL/rotates/supersedes a queued DM; invalid/unestablished path returns no payable candidate.
   Explicit CAS rotation works after expiry, revokes the old version and does not resend UNKNOWN.
6. Legacy Begin-only regardless of age, erased/ambiguous owner, retention/reactivation and denial markers cannot
   release an unresolved fence or revive effects/links. Stripe option is server-derived; bank writes refused.

One tests-only RED push covers both lanes, then one implementation push; preserve all §6h/m/§4 and old assertions.

## Acceptance gate and handoff

PR-own REAL_PG/API CI, contract drift zero, migration/upgrade/exact ACL pins and affected legacy/erasure/checkout
regressions. Buyer pay-entry uses real clicks → visible result → refresh/readback; independent security_reviewer
sign-off on forwarding/PII, order-only exchange, expiry, rate limits, key custody and old-link cutover, plus money
review. **Mobile/summary parity gate for UI:** recipient, channel, product/qty, assignee, window and send result
are mapped in DELIVERY.md across desktop/mobile/every summary/review; bearer page marks private/non-applicable
facts as intentionally withheld and proves they are absent on every viewport (never adds PII to satisfy parity).
Merchant order summaries additionally preserve name/phone/destination/totals/payment timing. Record click ledger,
current SHA, CI run/job/fault/exit/artifact and integrator DTO freeze before LC-U3 starts. No local tests/servers.

NOT_RUN: implementation/CI/REAL_PG/buyer browser/security until exact receipts; LC-U3 form adoption, real-device
Meta browser, Meta/Stripe SANDBOX/LIVE, real messages, App Review and production. Fake Graph/PSP remains MOCK.
