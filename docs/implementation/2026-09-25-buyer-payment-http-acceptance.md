# Buyer payment view and private HTTP

Status: **PASS_BOUNDED_PRIVATE_HTTP_PROVIDER_MOCK**. This is not public
payment UI, provider qualification, successful charging or deployment approval.
Contract: [buyer-payment-http-v1](../../contracts/buyer-payment-http-v1.md).
Baseline: accepted internal hosted core `18f5a2d`; frozen interface `603f237`.
Production source baseline `34a9169`; final combined source/test tree `7f4e8be`.
Subsequent author evidence commit `776c024` changes documentation only.

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
| BPH01 | PASS: exact view/3-name, 22 candidate-change cases, absent/foreign and zero-write tests. Four disconnected account/binding variants are rejected by real FK/CHECK constraints, not simulated as valid rows. |
| BPH02 | PASS: actual signed reports for PENDING/AUTHORIZED/CAPTURED/REVIEW_REQUIRED, original facts after profile changes and all page states. Exact blocker plus pre-expiry DB clock proves capability/qualification/page expiry during a real relation wait; return is unauthorized/no methods/EXPIRED respectively, without financial writes. |
| BPH03 | PASS: actual private HTTP + PG prepare/view/one-shot handoff in three locales; exact receipt and independently decoded original wire amount. No provider POST. |
| BPH04 | PASS: malformed/foreign/revoked/disabled requests, early nonretryable 503 and concurrent handoff with exactly one form. |
| BPH05 | PASS: disabled config/secret-read and unchanged parser unit gates; actual four-role API assembly and observed connection cleanup for successful close, wrong hosted role, invalid profile/key and final handler failure. |
| BPH06 | PASS: root final combined actual PG18/race/vet: 421 top-level tests, 0 FAIL/0 SKIP, foundation 171.368s, exit 0. Final independent source/evidence verdict has no unresolved P0/P1/P2 in this scope. |

Retained pre-final logs (not substitutes for the combined full gate):

- `/Volumes/data/output/buyer-payment-root-projection-smoke.log`: root `7cd793c`,
  actual isolated PG payment subset, exit 0, foundation 60.520s.
- `/Volumes/data/output/buyer-payment-view-author-1.log`: independent `d9b90e5`,
  actual PG/race payment subset, exit 0, foundation 67.856s.
- `/Volumes/data/output/buyer-payment-http-author-1.log`: retained failed test
  run. Disabled prepare was sent without its required key/body; this is not
  counted as a passing HTTP gate. Corrected runs are separately retained in
  [the independent author record](2026-09-25-buyer-payment-pg-author.md).
- `/Volumes/data/output/buyer-payment-root-private-1.log`: retained failed root
  fixture run. Its 1s expiry plus 20ms overrun exceeded the existing 1s lock timeout;
  no production defect was established. `7886604` changes only the fixture to
  450ms and retains exact blocker/pre-expiry-clock assertions and all result checks.
- `/Volumes/data/output/buyer-payment-root-private-2.log`: corrected actual PG
  subset PASS, 62.246s, including root final-clock and actual API assembly gates.
  SHA-256 `1c9492ff47ecabb2936d69b9e443a7eae8153ae741ec735b95b5d9ae28ff29f6`.
- `/Volumes/data/output/buyer-payment-root-full-1.log`: final root command
  `bash scripts/dev/test-local.sh`, actual exit 0 at `7f4e8be`; full Go `-race`,
  real isolated PG18 and `go vet ./...`. 421 top-level PASS, 0 FAIL, 0 SKIP;
  foundation 171.368s. SHA-256
  `f161c5fbc556abb5f317f03eeb341a1a369b102cf457e4a7cdf263d6d76d5477`.

## Ownership and review

Root integrator owns SQL0026, `PaymentView`, PT404 mapping, SQL smoke, the
foundation API-assembly launcher, causal final-clock test and integration docs.
Seventeen changed Go/SQL files were submitted to the code graph; it reported
16 processed files, 259 entities and 0 rejected. The read-model, causal-clock
test and assembly gate are linked to their respective design/authority reasons.

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
Model/effort labels above record delegation configuration, not an independent
attestation of the serving runtime. Independent author test record:
`e7ed2500-7378-4cf4-bfc2-5c09e587982b`; assembly author record:
`bafa1474-e097-45cd-8487-82981e37280b`.
Final independent source/evidence record:
`85bbfc20-fcf0-4139-b339-a93c74f0de3e`. The reviewer inspected the final log/hash
and changed source; it did not independently rerun the full suite. Root did run
the full combined tree independently of the authors' subset runs.

## Release boundary

Public BFF/client/payment button, safely constrained form submission, return
route, browser recovery/lost-response gates and provider sandbox acceptance are
**NOT_RUN** for this increment. The approved B product composition remains the
target. Merchant enabling, qualification issuance, real DNS/TLS, production
worker/notification assembly and whole-SaaS release remain separate gates.
No customer production data, live broadcast, payment account or service was
changed. Test-owned fixtures are disposed by the existing owner-labelled runner;
failed evidence and agent worktrees are retained.
