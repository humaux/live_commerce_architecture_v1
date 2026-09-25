# Buyer payment view and private HTTP

Status: **IMPLEMENTED / INDEPENDENT GATES IN PROGRESS**. This is not public
payment UI, provider qualification, successful charging or deployment approval.
Contract: [buyer-payment-http-v1](../../contracts/buyer-payment-http-v1.md).
Baseline: accepted internal hosted core `18f5a2d`; frozen interface `603f237`.

## Boundaries and dependencies

- `checkout.PaymentView` uses the dedicated hosted pool and SQL0026's single
  owned-order snapshot. It exposes only order totals, payment/handoff status,
  explicit test-mode disclosure and current method names. No stored form,
  account/credential/qualification identifiers or buyer address enters the view.
- Historical payment facts retain their original profile/environment and exact
  amount. A different runtime profile disables handoff, not history visibility.
  Authorized, captured and review-required are distinct; a redirect or pending
  attempt is never evidence of payment.
- Private `buyerhttp` routes retain BFF authentication, published-origin scope,
  capability checks, strict input and no-store envelopes. Prepare returns four
  receipt fields; handoff rejects body/replay keys and uses nonretryable errors.
- `cmd/api` defaults the nested payment feature off. Its enabled path opens a
  fourth independently validated SQL authority and reuses the historical account
  key parser. Every partial startup failure closes owned pools. River only
  enqueues within the existing transaction; no worker or PSP request is started.
- No dependency, table, index, generic retry mechanism or second transaction
  engine was added. SQL0026 grants only function execution to the hosted role.

## Gate evidence

| Gate | Evidence and current limitation |
| --- | --- |
| BPH01 | Independent exact view/3-name, candidate drift, absent/foreign and zero-write tests. Additional admission matrix in progress. |
| BPH02 | Actual signed test reports for PENDING/AUTHORIZED/CAPTURED/REVIEW_REQUIRED, original facts after profile changes and page states. Root causal final-clock gate in progress. |
| BPH03 | Actual private HTTP + PG prepare/view/one-shot handoff in three locales; author run in progress. No provider POST. |
| BPH04 | Malformed/foreign/revoked/disabled requests and concurrent handoff; author run in progress. |
| BPH05 | Disabled config/secret-read and parser unit gates passed. Actual four-role API assembly and connection cleanup in progress. |
| BPH06 | Final combined root full PG/race/vet and independent source/evidence verdict pending. |

Retained pre-final logs (not substitutes for the combined full gate):

- `/Volumes/data/output/buyer-payment-root-projection-smoke.log`: root `7cd793c`,
  actual isolated PG payment subset, exit 0, foundation 60.520s.
- `/Volumes/data/output/buyer-payment-view-author-1.log`: independent `d9b90e5`,
  actual PG/race payment subset, exit 0, foundation 67.856s.
- `/Volumes/data/output/buyer-payment-http-author-1.log`: retained failed test
  run. Disabled prepare was sent without its required key/body; this is not
  counted as a passing HTTP gate. Corrected valid-input rerun remains separate.

## Ownership and review

Root integrator owns SQL0026, `PaymentView`, PT404 mapping, SQL smoke, the
foundation API-assembly launcher, causal final-clock test and integration docs.

`hosted_go_implementation` (commerce_worker, gpt-6-sol/high), isolated
`/Volumes/data/worktrees/commerce-buyer-payment-http-20260925`, base `603f237`,
owns only `cmd/api/{accounts,buyer,buyer_payment}.go`, corresponding new focused
payment tests and `internal/buyerhttp/{handler,payment}.go`. Author commits
`ea864d3` and `8c1260f`; root projection dependency was not duplicated.
Its bounded follow-up base `4ecff05`, worktree
`/Volumes/data/worktrees/commerce-buyer-payment-assembly-gate-20260925`, owns only
new tagged `cmd/api/buyer_payment_pg_test.go` (`6fdefb4`).

`hosted_pg_gate` (test_worker, gpt-6-sol/high), isolated
`/Volumes/data/worktrees/commerce-buyer-payment-pg-20260925`, base `603f237`, owns
only new `buyer_payment_{view,http}_test.go` and its independent author record.
`hosted_contract_security_review` (security_reviewer, gpt-6-sol/high) is read-only;
frozen-design verdict `f4bbc541-6a2a-41bf-b543-02dac6f036a9` is DESIGN evidence only.

## Release boundary

Public BFF/client/payment button, safely constrained form submission, return
route, browser recovery/lost-response gates and provider sandbox acceptance are
**NOT_RUN** for this increment. The approved B product composition remains the
target. Merchant enabling, qualification issuance, real DNS/TLS, production
worker/notification assembly and whole-SaaS release remain separate gates.
No customer production data, live broadcast, payment account or service was
changed. Test-owned fixtures are disposed by the existing owner-labelled runner;
failed evidence and agent worktrees are retained.
