# w3-02b-picklist plan

1. Migration 0130: `fulfillment.read_pick_list(p_hash,p_store,p_orders,p_session)` (owner commerce_checkout_writer, EXECUTE commerce_runtime only) + `claims.pick_list_session_orders` helper (owner commerce_claims_writer, EXECUTE commerce_checkout_writer; session = live_price_uses ∪ order_origins). No tables/columns/roles.
2. Go merchantorders: `picklist.go` (PickList + strict decode + PT422 too_many) and `carrier_export.go` (4 template constants, reuse read_pick_list rows, writeCSVLine+guardFormula+BOM, orders:export + audit orders.carrier_export).
3. Go fulfillment: `cvs_batch.go` — extract `requestOne` from `Request`, `Batch` via command.Run + per-order SAVEPOINT; outcomes queued/failed:code.
4. httpapi `picklist.go`: 3 routes + picklistClassify; wire in handler.go (pick-list/export need pool only; cvs-batch needs Options.CVS).
5. ACL pins: manual_fulfilment_schema_test.go MF02 + merchant_orders_v2_acl_test.go (read_pick_list EXECUTE commerce_runtime) + worker_authority_split WAS02 deny.
6. `option_label` always null (no variant model). Tests PL01–PL08 red→green.
