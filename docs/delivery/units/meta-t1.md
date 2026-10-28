# META-T1 — independent Meta messaging / claims / order-attempt acceptance

Status: DESIGN / NOT_RUN (META-00 R7, 2026-10-28); continues LC-T1 acceptance, adds no product feature.
Purpose: independently verify A–D: inbound DM/comments, keyword claims/private links, manual replies and orders.
Depends on: each implementation unit's own exact-head CI GREEN and frozen DTOs; current integration SHA supplied
by integrator. Reviewer must not be the implementation author or treat historical GREEN as current acceptance.
Used by: integrator acceptance; owner separately controls all external/LIVE approval.
Invariants: immutable existing assertions, deterministic counterexamples, privacy/money boundaries and honest evidence.
Authority: `contracts/live-console-v1.md` §13/META-00, intake §3 and `order-attempt-v1.md` §6.

## Scope and write paths

- Audit the new Inbox/console/manual-order/pay-entry chain and existing A–D regressions; do not reimplement
  already-merged modules. Existing LC-T1/LCN and OA acceptance obligations remain; record any missing selector.
- Write paths: `tests/admin/inbox*`, `tests/admin/live-console*`, `tests/storefront/*pay-entry*`,
  `tests/foundation/*meta*gate*`, `tests/foundation/*live_console*gate*`, `output/meta-t1/**`.
  Go counterexamples outside those paths require integrator/test-owner assignment first. No product code,
  shared selectors, fixtures, baseline, exemptions or dependency changes to force acceptance.
- Separate test author/reviewer ownership from implementation; one assigned worktree/branch/PR. No recursive
  delegation. Put actual role/model/reasoning if exposed, base/head/merge SHA and allowed paths in DELIVERY.md;
  unknown runtime metadata stays UNKNOWN. Integrator owns acceptance and any later merge decision.

## Batch RED / mutation list (GitHub CI only)

1. Inject private-thread cleanup/stale-response failure → specific leak assertion red; wrong recipient/source
   or all-store DM in session feed → exact recipient/membership assertion red.
2. Skip private quota or DM window validation separately → forbidden send/call-count assertion red; infer a
   window from a comment/payment or consent → original authority assertion red.
3. Cancel still dispatches, missing lease sweep or retry UNKNOWN → separate zero-call/lease/no-second-send
   assertions red with real barriers. Preserve OA M2a/M2b/M4/M7 and other required mutation receipts by identity.
4. Copy rotates/extends TTL, forwarded bearer leaks PII, mobile/review drops a required fact or pause ends the
   session → designated copy/privacy/parity/lifecycle assertion red.

Batch all new tests/mutations in one RED push; distinguish baseline assertion failures from injected-defect
calibrations. Then restore all mutations and make one implementation/test-harness push for PR-own GREEN.
Tests must reach their assertions; setup failure is not RED proof. Never weaken fixtures/thresholds/timeouts or
existing assertions (including verbatim ordered next-startup gate-adapter assertions).

## Acceptance gate and handoff

Exact-SHA GitHub CI: affected inbox/live-console/live-claims/claim-checkout/manual-order/pay-entry/e2e browser
modes, REAL_PG/fake Graph foundation assertions, contract parity and required suite. Click/fill/select/keyboard
every actionable control, assert visible results and refresh persistence, and attach page/control/action/
expected/actual/result ledger. API seed, DOM mutation, forced click and screenshot are not user-action proof.
**Mobile/summary parity gate:** recipient, channel, product/qty, assignee, window and send result across desktop,
mobile and every summary/review state; order flows also name/phone/destination/totals/payment timing. Inspect
actual exports/rendered states, 1440/390 and zh-TW/en/ja; bearer pages must consistently withhold private fields.
Independent privacy/money/visual review, mutation-to-assertion table and run/job/artifact/exit/source/merge-SHA
receipts are required. Classify E0 self-report, E1 structure, E2 representative run, E3 automated tested environment,
E4 independent verification; no MOCK proof upgrades a provider capability to LIVE.

No local tests/servers; use CI only. On long queues record Gates/Foundation workflow run IDs (not plan-job IDs)
and hand off after ≤10 minutes with a named integrator wakeup; never silently abandon a running gate.
NOT_RUN: all independent gates until exact receipts; Meta/Stripe SANDBOX/LIVE, App Review, real messages/payments,
real-device Meta browser and production. LC-X1 remains separately owner-authorized; no connection or send here.
