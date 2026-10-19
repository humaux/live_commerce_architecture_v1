# Independent review — commit 22de69581 (PR #70, unit/pay-rm2-p2a)

- Reviewer: Claude Code subagent, model **Opus 5.5 (claude-opus-5-5)**, read-only
- Scope: `git show 22de69581` only (tests/foundation/stripe_start_test.go, tests/foundation/meta_ads_capi_test.go)
- Evidence level: static review (E0/E1 for the change). NOT_RUN: no tests executed locally (per instruction); CI re-run result for this SHA was not inspected.

## Verdict: APPROVE (no P0/P1; three P2 nits, optional)

## 1. SP07 constructor_negatives (stripe_start_test.go:736-768)

Honest reflection of the retired contract, not a weakening:
- `internal/checkout/hosted.go:54-56` documents "HostedProviders admits only Stripe; an empty value creates a historical read-only service" and PAYUNi as "ignored compatibility input". `NewHostedPaymentService` (hosted.go:66-86) refuses only nil ctx / nil jobs / invalid profile / bad Stripe config / hosted-pool validation. Keys are no longer consulted at all, so `payuni_without_keys` cannot be a refusal.
- `BeginHosted` (hosted.go:88-95) then `beginStripe` (stripe.go:111-114) returns `PaymentResult{}, command.ErrInvalid` on `s.stripe == nil` **before** `buyer.WithScope` opens any transaction. There is no write path to leave unchecked.
- The new block is stronger than before. It used to assert only constructor refusal; now it proves the services build, that starting fails closed with a zero result, and that the 9 `sstFacts` counters (stripe_start_test.go:191-204) stay unchanged. Those counters cover attempts, integration.operations, command_results, checkout.events, stripe_sessions, `river_payment.river_job` payment_query_v1, AWAITING_PAYMENT orders, PAYMENT_PENDING reservations and hosted pages. That covers the River lane and the attempt rows. No other lane is reachable because the transaction never opens.
- No contract in contracts/ or docs/ requires the `no_provider` or `payuni_without_keys` refusals (grep found none).

P2-1 (stripe_start_test.go:758-761): `ErrInvalid` can also come from the outer guard in hosted.go:90-92 (bad key or invalid input). Nothing in this subtest shows that the same `s`/`s.input("zh-TW")`/key passes with a Stripe-configured service, so the test cannot tell the `s.stripe == nil` guard apart from a broken seed. The fail-closed property still holds either way. Optional: begin once with the `Stripe: &e.scfg` service built at line 770 and expect success.

P2-2 (stripe_start_test.go:773, pre-existing, not in this diff): the comment "The old constructor keeps its signature and stays PAYUNi-only" is stale. `NewHostedPaymentStarter` (hosted.go:61-64) now builds the empty-providers read-only service, and NotFound for Refresh/Cancel follows from `s.stripe == nil` (stripe.go:277). Reword it in a later touch.

## 2. MA03 owner_erased (meta_ads_capi_test.go:241-270)

- **Red-first is real**: it asserts 409 + `erasure_blocked` through the real route before the fixture changes anything. That matches 0078:514-521 (`x.expires_at > clock_timestamp()` on payments.stripe_sessions) and the mapping in buyerhttp/privacy.go:176-177.
- **Column coverage**: the 0061 DDL (0061_stripe_psp.sql:285-321) has exactly 13 timestamptz columns (attempt_created_at, expires_at, send_deadline, handoff_cutoff, create_first_sent_at, create_last_sent_at, create_suppressed_at, pinned_at, url_purged_at, first_handed_out_at, cancel_requested_at, last_refresh_at, last_expire_at). All 13 are shifted. 0062:164 adds only `charge_signal_count integer`. No other migration alters stripe_sessions, so no column is missed.
- **CHECKs after the shift**: `expires_at = date_trunc('second', attempt_created_at) + 40 min` still holds because a whole-minute shift commutes with date_trunc('second'). `send_deadline = attempt_created_at + 7 min`, `handoff_cutoff = expires_at - 5 min` and `create_first_sent_at < send_deadline` hold under a uniform shift. NULL columns stay NULL, so the NULL-pairing CHECKs are unaffected. Replica mode (meta_ads_flow_test.go:163-180) bypasses only the `guard_stripe_session` trigger (0061:602), which would otherwise reject the backward move of last_*_at. CHECKs still apply. This is disclosed in the call-site comment and the file header.
- **Reusing the idempotency key after 409**: valid. `erase_owner` raises PT409 before inserting into `customers.privacy_actions`. The buyer transaction rolls back, and buyerhttp/privacy.go:120-140 stores no HTTP receipt (the erasure route bypasses buyer.RunCommand). The retry with the same key therefore reaches the CD7 branch again, which is the realistic retry semantics.
- **Other blockers**: (a) DRAFT/AWAITING_PAYMENT orders with a live hold. The order is captured by `rm2StripePay` inside `prime`, so it should not count. (b) Refunds without a terminal fact: none are created. (c) Other live sessions for the owner: `pwIsolatedFixture` plus a single `rm2StripeStart` gives one attempt, so `WHERE attempt_id=$1` covers the only session. The 200 assertion fails loudly if any blocker remains.
- Header (meta_ads_capi_test.go:8-10) was updated accurately.

P2-3 (meta_ads_capi_test.go:252-259): `ownerReplica` does not check RowsAffected. An attempt_id mismatch would show up as a 409 at the next erase instead of a clear fixture error. Cosmetic only, since the failure stays loud.

## Not checked / NOT_RUN
- No local or CI execution of TestStripeSP07Start or TestMetaAdsMA03Consent at 22de69581 was observed by this reviewer.
