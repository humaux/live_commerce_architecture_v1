# G07 unrelated refund fixture failure

Source pinned at `da2faeed7f1bfe243d77e554298bb829c0b1d638`; baseline `e44e58a2458102ebeb772a84c8086e0af5500c72`.

- Actual full G07 failure: `TestBuyerCommsCardCaptureAndStripeRefundTriggers/Stripe_refund_facts_enqueue_one_refunded_row_per_order_however_many_partial_refunds`.
- `tests/foundation/buyer_comms_smoke_test.go:809–811` inserts `requested_at=clock_timestamp()` and `resend_until=clock_timestamp()+interval '20 hours'` using two separate volatile evaluations.
- `migrations/0062_stripe_refund.sql:107` requires exact equality: `CHECK(resend_until=requested_at+interval '20 hours')`. Different clock values violate `stripe_refunds_check` with SQLSTATE 23514, observed in `g07/G07.log`.
- `git diff --exit-code e44e58a2..da2faeed -- tests/foundation/buyer_comms_smoke_test.go migrations/0062_stripe_refund.sql` returned **0**. Both files are byte-identical to baseline. The only migration edited by this unit is 0106; no refund logic or constraint changed.
- Read-only independent reviewer `number_tests` and root inspected this evidence. The failure is in the pre-existing fixture's time construction, not the numeric store allocator. This is not a full baseline suite replay.

No fixture, refund implementation or frozen assertion has been changed. Full G07 retains its actual nonzero exit. A separately authorized fixture correction should derive both values from one timestamp; rerunning until the two clock evaluations happen to match would not fix the root cause.

Optional narrow reproduction, only after the shared lock becomes available: run `bash scripts/dev/test-focused.sh '^TestBuyerCommsCardCaptureAndStripeRefundTriggers$'` under this unit's safe lock wrapper. NOT_RUN here: the full gate already provides the actual failure, and another PG process must not be started while it runs.
