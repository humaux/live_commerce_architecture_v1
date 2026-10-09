# PR20 round 1 — per-thread changes

Source commit: `2b8089a9baa27e6d679018cea3e551c837bb2029`, base `66f9976122074c74138bdeae19c2b1dd0a61eeb8`.

- **4226572680 / P1 mixed quotes**: `migrations/0167_stripe_untracked_reservation.sql:160` validates every quote SKU. A line absent from reservation_lines may be legitimately unreserved only with the scoped immutable original quote and no original RESERVE history for that SKU. A tracked sibling's RESERVE is allowed; missing tracked lines, quantity mismatch and extra reserved SKUs refuse. Capture checks every such unreserved SKU for current deletion/tracking before allocating any sibling. `TestStripeUntrackedMixedDrift` covers tracking and deletion, capture/replay, zero partial allocation and no READY state.
- **4226572675 / fulfillment**: line205 writes fulfillment_state=PAID_ALLOCATION_FAILED together with the review. Commercial state stays AWAITING_PAYMENT, reservation PAYMENT_PENDING; existing general review code inserts REVIEW_REQUIRED. `TestStripeUntrackedFulfillmentState` checks the persisted aggregate.
- **4226572677 / unpaid closure**: line289 applies the same original quote/RESERVE evidence. No current catalogue read occurs in the close proof. `TestStripeUntrackedCloseAfterDrift` covers tracking/deletion, all-untracked/mixed; actual CaptureWorker returns nil, CLOSED_UNPAID persists exactly once, original tracked holds alone release, and the real query job finishes without another provider request.
- **4226572684 / presentment precedence**: capture proof is before SELECT v_review and its early return. Both PROVIDER_PRESENTMENT_DRIFT and PAID_ALLOCATION_FAILED persist, with capture evidence. `TestStripeUntrackedPresentmentDriftRefund` exercises the actual merchant refund HTTP handler: partial refund422 refund_blocked_review; full remaining201. No refund worker/provider send is started.

## §4.4 compatibility

The fulfillment update is gated by **v_new_capture**, the first valid pending same-generation capture in the transaction that records its fact. It does not run for an already captured attempt. Thus it does not alter the contract's prohibition on reviews inserted *after capture* rewriting order/work-item state. `TestStripeUntrackedPostCaptureDriftImmutable` first settles normally, changes the catalogue, then submits a later presentment report: commercial/fulfillment/updated_at/reservation/work states remain byte-equal and no retroactive allocation-failure review is inserted. No contract change is needed.

## Proof authority and unchanged boundaries

`post_river/0021_product_core_begin_hold.sql:72,293–309,380–391` already validates the original quote, equates tracked demand to the reservation plan, and atomically writes RESERVE history. The new proof uses those durable facts rather than inventing a historical inventory_tracked flag. A corrupted current snapshot cannot be excused by an untracked catalogue flag. Existing checkout-writer SELECT/RLS authority covers quotes and ledger; no ACL change.

RF12 restores one new capture-proof section to empty and the two stock-loop empty-plan sections to their original refusal, then requires the **entire** function body equal0062. Its original0061→0062 one-line refund guard assertion remains. Independent read-only Python also verified the three seam counts1/1/1 and full byte equality. No migration-number/count, owner, SECURITY DEFINER, search_path, inherited EXECUTE ACL or COMMENT change.

The independent review was E1/source-only, not an independent PG gate or K3 approval. Integrator K3 deep review remains required. Plaintext red.log has all four original top-level failures; green.log has eleven focused passes. Broader final gate results are in results.json when the batch completes.
