-- Purpose: expose the native C2 preparation stage through one protected composite entry.
-- Depends on: 0178 current-xid fence, 0180 owner prefix, 0179 quote, 0181 destination,
-- 0183 catalog/allocation, 0184 offline safety, and 0185 claim prefix for claim intents.
-- Used by: the outer Go executor on runtime/claims-worker connections; no domain DML grants.
-- Invariants: order-attempt-v1 §§1/3; exact original plan, canonical lock order and
-- a single outer transaction. 0182 forbids committing this unfinished prepare stage.

CREATE FUNCTION ops.execute_order_attempt(p_attempt uuid,p_generation bigint,p_plan_plaintext text,p_step text,p_input jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE binding jsonb; ctx jsonb; owner_state text; quote jsonb; destination jsonb; allocation jsonb; result jsonb;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 -- Calls protected ops contexts before trusting any caller identity or stage data.
 -- lock_order_attempt has taken the first row lock on this physical transaction.
 binding:=ops.order_attempt_fence_context(p_attempt,p_generation);
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 IF p_step IS DISTINCT FROM 'prepare' OR p_input IS DISTINCT FROM '{}'::jsonb THEN
  RAISE EXCEPTION 'attempt_stage_invalid' USING ERRCODE='PT400';
 END IF;
 PERFORM set_config('app.tenant_id',binding->>'tenant_id',true),set_config('app.store_id',binding->>'store_id',true),
  set_config('app.principal_id',binding->>'principal_id',true),set_config('app.buyer_id',binding->>'buyer_owner_id',true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 -- All indexed owners precede every claim, parcel or order resource. Neither
 -- this continuation nor domain helpers resolve a placement-time session again.
 owner_state:=buyer.order_attempt_lock_owners(p_attempt,p_generation);
 IF owner_state IS DISTINCT FROM 'active' THEN
  RAISE EXCEPTION 'owner_unavailable' USING ERRCODE='PT409';
 END IF;
 -- An admitted manual intent has an authenticated empty claim set. Claim intents
 -- must acquire the complete domain prefix before any order/cart materialization.
 IF jsonb_array_length(ctx->'plan'->'facts'->'claims'->'lines')>0 THEN
  IF claims.order_attempt_lock_origins(p_attempt,p_plan_plaintext) IS DISTINCT FROM 'active' THEN
   RAISE EXCEPTION 'claim_unavailable' USING ERRCODE='PT409';
  END IF;
 END IF;
 -- The new order is not yet in a parcel group; its canonical ID is the attempt.
 PERFORM pg_advisory_xact_lock(hashtextextended('checkout.order|'||(binding->>'tenant_id')||'|'||(binding->>'store_id')||'|'||p_attempt::text,0));
 quote:=storefront.order_attempt_materialize_quote(p_attempt,p_plan_plaintext);
 PERFORM checkout.order_attempt_lock_catalog(p_attempt,p_plan_plaintext);
 PERFORM checkout.order_attempt_lock_offline_safety(p_attempt,p_plan_plaintext);
 destination:=storefront.order_attempt_materialize_destination(p_attempt,p_plan_plaintext,(quote->>'cart_id')::uuid);
 allocation:=checkout.order_attempt_prepare_allocation(p_attempt,p_plan_plaintext,
  (quote->>'cart_id')::uuid,(quote->>'quote_id')::uuid,(destination->>'destination_id')::uuid);
 result:=jsonb_build_object('stage','PREPARED','cart_id',quote->'cart_id','quote_id',quote->'quote_id',
  'destination_id',destination->'destination_id','allocation',allocation);
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true),
  set_config('app.principal_id',coalesce(prior_principal,''),true),set_config('app.buyer_id',coalesce(prior_buyer,''),true),
  set_config('app.buyer_session_id',coalesce(prior_session,''),true),set_config('app.order_attempt_id',coalesce(prior_attempt,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.execute_order_attempt(uuid,bigint,text,text,jsonb) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.execute_order_attempt(uuid,bigint,text,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.execute_order_attempt(uuid,bigint,text,text,jsonb) TO commerce_runtime,commerce_claims_worker;
COMMENT ON FUNCTION ops.execute_order_attempt(uuid,bigint,text,text,jsonb) IS
 'C2 composite internal call stage, runtime/claims-worker only: exact held generation/current-xid fence and original plaintext; owner/claim/order/cart/catalog/service/destination/inventory suffix via narrow domain definers. Prepare returns native allocation facts to existing Go allocator and cannot COMMIT without full settlement under0182. No placement reauthorization, provider I/O or direct domain grants. Caller scope restored; stage data cannot supply tenant/owner/money.';
