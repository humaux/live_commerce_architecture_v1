# Independent review — ed06573d3 (PR #70, PAY-RM2, g09)

- Reviewer: Claude Opus 5.5 (claude-opus-5-5), read-only; no tests run, no edits, no push.
- Scope: `git show ed06573d3` only (tests/foundation/expiry_runtime_test.go, tests/foundation/payment_fixture_helpers_test.go).
- Evidence tier of this review: static code reading (DESIGN). The CI log of run 38790242931 was not read; the failure text is taken from the commit/brief.

## Verdict: APPROVE (no P0/P1)

## 1. Root-cause chain — confirmed by code
- `sflNewOn` -> `sstNewEnv` builds a new `stripetest.New("acct_FakeDefault0001")` per env (tests/foundation/stripe_start_test.go:88).
- The fake numbers sessions from a per-instance counter: `s.next++; id := fmt.Sprintf("cs_test_fake_%d", s.next)` (internal/integrations/psp/stripe/stripetest/server.go:475-476; same in Inject at 564-565). A second fake reissues `cs_test_fake_1`.
- `payments.stripe_sessions.session_id text UNIQUE` (migrations/0061_stripe_psp.sql:299).
- Pin path: `integration.record_stripe_observation` does `UPDATE payments.stripe_sessions SET session_id=v_session ... WHERE attempt_id=a.id` when unpinned (migrations/post_river/0012_stripe_payment.sql:~469) -> 23505 -> `recordStripe` returns `errPaymentQueryDatabase` (internal/payments/stripe_query.go:327-329) -> `finishStripe(..., "stripe_record_failed", 5s)` (stripe_query.go:104-108), repeating until `pinned`'s 25s await times out (stripe_flow_test.go:181-185). Matches the reported `codes=[stripe_record_failed ...]`.
- Alternatives ruled out:
  - Account identity: `seed` uses `acct_T`+`t04Tag()` per store (stripe_start_test.go:135) — unique against `stripe_account_identity_unique` (0061:32); and a collision there would fail at Register, not at pin.
  - One-account-per-store: `psSetupItemsOn` re-randomizes `f.tenantA/f.storeA1` per call (payment_start_test.go:76), so each store is new.
  - Identity / account checks in record_stripe_observation (0012:~452-465) compare against the attempt's own row and the report the same fake produced; nothing env-dependent fails there.
  - Webhook endpoint uniqueness: only reached in `rm2StripePay` via `paymentEndpoint`, which is after the failing pin and cached per connection (payment_fixture_helpers_test.go:84-96).

## 2. Sharing one env — supported, assertions not weakened
- Multiple `stripeStore` per env is established practice: stripe_flow_test.go:391, payment_runtime_test.go:248, stripe_refund_webhook_test.go:102, customers_billing_list_test.go:118/123.
- Shared fake gives `cs_test_fake_1`/`_2`; both stores' accounts are registered in that one fake.
- `rm2StripePay` on `confirmed`: `session()` reads the DB by attempt; `fake.SetState` targets the same fake that created it; `paymentEndpoint` uses confirmed's own connection; `awaitFact` / `sstWake` are scoped by `operation_id` = confirmed attempt. Unchanged semantics.
- Pending attempt: its query job may be run by the confirmed-pay worker; it observes an `open/unpaid` session in the same fake, so it stays PAYMENT_PENDING — the `ewAssertOrder(pending, "AWAITING_PAYMENT", "PAYMENT_PENDING")` and generation==2 assertions are unaffected. This is strictly more correct than before: with two envs, either env's worker could process the other attempt's jobs against a fake that did not own its session.

## 3. Other two-env-on-one-fixture callers
- Only `sflEnv.start*`, browser_e2e_test.go:231 and browser_stripe_test.go:367 run a Stripe worker that pins sessions; each uses one fake per DB.
- Remaining multiple `sstNewEnv` on a shared fixture (e.g. expiry_runtime_test.go:572 in a 2-iteration loop, stripe_retained_invariants_test.go:28/88/215, stripe_untracked_quantity_test.go:76, stripe_buyer_http_test.go:189, stripe_start_test.go subtests) start no worker, so no session is pinned -> no collision today.
- `rm2StripePending(t)` uses `pwIsolatedFixture` per call — safe.

## 4. Comments
- New `rm2StripePendingIn` doc comment states the rule and reason accurately.
- File header (payment_fixture_helpers_test.go:3-10) stays accurate; the Invariant line names `rm2StripePending` and holds equally for `rm2StripePendingIn`.

## Findings
- P2-1 (latent hazard, not blocking): the rule lives only in a helper comment. Any future test that adds a worker to a second `sstNewEnv` on a shared fixture (e.g. expiry_runtime_test.go:572 loop) will hit the same UNIQUE collision. Root-level fix would be a per-instance random session-id prefix in stripetest/server.go:476/565; out of scope for this commit.
- P2-2 (evidence): this commit is E0 for the fix until g09 is green on CI at ed06573d3 or a descendant SHA; do not merge on this review alone.

No P0/P1.
