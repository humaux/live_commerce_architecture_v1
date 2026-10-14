-- Purpose: expose all native C2 stages through one protected composite entry.
-- Depends on: 0178 current-xid fence, 0180 owner prefix, 0179 quote, 0181 destination,
-- 0183 allocation, 0184 Begin, 0185 claims, 0187 pay entry, 0188 notification and 0189 proof.
-- Used by: the outer Go executor on runtime/claims-worker connections; no domain DML grants.
-- Invariants: order-attempt-v1 §§1/3; exact original plan, canonical lock order and
-- a single outer transaction. 0182 forbids committing this unfinished prepare stage.

CREATE FUNCTION ops.execute_order_attempt(p_attempt uuid,p_generation bigint,p_plan_plaintext text,p_step text,p_input jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE binding jsonb; ctx jsonb; owner_state text; quote jsonb; destination jsonb; allocation jsonb; result jsonb; field text;
 pay_entry jsonb; notification jsonb; native jsonb; send_result jsonb; v_receipt jsonb;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 -- Calls protected ops contexts before trusting any caller identity or stage data.
 -- lock_order_attempt has taken the first row lock on this physical transaction.
 binding:=ops.order_attempt_fence_context(p_attempt,p_generation);
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 IF p_step IS NULL OR p_step NOT IN ('prepare','begin','pay','finish') OR jsonb_typeof(p_input) IS DISTINCT FROM 'object' THEN
  RAISE EXCEPTION 'attempt_stage_invalid' USING ERRCODE='PT400';
 END IF;
 IF p_step='prepare' AND p_input<>'{}'::jsonb THEN
  RAISE EXCEPTION 'attempt_stage_invalid' USING ERRCODE='PT400';
 ELSIF p_step='begin' THEN
  IF (SELECT count(*) FROM jsonb_object_keys(p_input))<>4
   OR EXISTS(SELECT 1 FROM jsonb_object_keys(p_input) k WHERE k NOT IN ('cart_id','quote_id','destination_id','allocation'))
   OR jsonb_typeof(p_input->'allocation') IS DISTINCT FROM 'array' THEN
   RAISE EXCEPTION 'attempt_stage_invalid' USING ERRCODE='PT400';
  END IF;
  FOREACH field IN ARRAY ARRAY['cart_id','quote_id','destination_id'] LOOP
   IF jsonb_typeof(p_input->field) IS DISTINCT FROM 'string'
    OR (p_input->>field)!~'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
    RAISE EXCEPTION 'attempt_stage_invalid' USING ERRCODE='PT400';
   END IF;
  END LOOP;
 ELSIF p_step='pay' THEN
  IF (SELECT count(*) FROM jsonb_object_keys(p_input))<>3
   OR EXISTS(SELECT 1 FROM jsonb_object_keys(p_input) k WHERE k NOT IN ('key_version','token_hash_hex','credential_ciphertext_hex'))
   OR jsonb_typeof(p_input->'key_version') IS DISTINCT FROM 'string'
   OR coalesce(p_input->>'key_version','')!~'^[A-Za-z0-9_.-]{1,64}$'
   OR jsonb_typeof(p_input->'token_hash_hex') IS DISTINCT FROM 'string'
   OR coalesce(p_input->>'token_hash_hex','')!~'^[0-9a-f]{64}$'
   OR jsonb_typeof(p_input->'credential_ciphertext_hex') IS DISTINCT FROM 'string'
   OR coalesce(p_input->>'credential_ciphertext_hex','')!~'^[0-9a-f]{142}$' THEN
   RAISE EXCEPTION 'attempt_stage_invalid' USING ERRCODE='PT400';
  END IF;
 ELSIF p_step='finish' THEN
  -- The private notification definer verifies every field of READY/VOID against
  -- native preparation. Reject unsupported artifacts before inspecting effects.
  IF coalesce(p_input->>'state','') NOT IN ('NOT_REQUESTED','READY','VOID')
   OR (p_input->>'state'='NOT_REQUESTED' AND p_input<>jsonb_build_object('state','NOT_REQUESTED')) THEN
   RAISE EXCEPTION 'attempt_stage_invalid' USING ERRCODE='PT400';
  END IF;
 END IF;
 PERFORM set_config('app.tenant_id',binding->>'tenant_id',true),set_config('app.store_id',binding->>'store_id',true),
  set_config('app.principal_id',binding->>'principal_id',true),set_config('app.buyer_id',binding->>'buyer_owner_id',true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 IF p_step='begin' THEN
  -- No earlier prefix locks recur here. The domain helper requires the exact
  -- native cart/quote/destination from this fenced attempt and validates the
  -- existing Go allocation against the locked native inventory facts.
  BEGIN
   result:=checkout.order_attempt_begin(p_attempt,p_plan_plaintext,
    (p_input->>'cart_id')::uuid,(p_input->>'quote_id')::uuid,
    (p_input->>'destination_id')::uuid,p_input->'allocation');
  EXCEPTION WHEN SQLSTATE 'PT409' THEN
   -- A caller changed this transaction's materialized identities/allocation.
   -- Classify only these exact internal-input failures as invalid stage input;
   -- stock, capacity, deadline and other dynamic refusals keep their own codes.
   IF SQLERRM IN ('attempt_cart_conflict','attempt_quote_conflict',
    'attempt_destination_conflict','stock_plan_differs_from_quote','invalid_stock_plan') THEN
    RAISE EXCEPTION 'attempt_stage_invalid' USING ERRCODE='PT400';
   END IF;
   RAISE;
  END;
 ELSIF p_step='pay' THEN
  pay_entry:=checkout.order_attempt_create_pay_entry(p_attempt,p_plan_plaintext,
   p_input->>'key_version',decode(p_input->>'token_hash_hex','hex'),decode(p_input->>'credential_ciphertext_hex','hex'));
  notification:=inbox.order_attempt_prepare_notification(p_attempt,p_plan_plaintext);
  result:=jsonb_build_object('stage','PAY_PREPARED','pay_entry',pay_entry,'notification',notification);
 ELSIF p_step='finish' THEN
  -- Read exact native facts under the locks already acquired by prepare/Begin.
  -- No client order ID, amount or receipt can finalize an incomplete pipeline.
  native:=checkout.order_attempt_terminal_facts(p_attempt,p_plan_plaintext);
  IF jsonb_array_length(ctx->'plan'->'facts'->'claims'->'lines')>0 THEN
   PERFORM claims.order_attempt_complete_origins(p_attempt,p_plan_plaintext,p_attempt);
  END IF;
  notification:=inbox.order_attempt_finish_notification(p_attempt,p_plan_plaintext,p_input);
  IF notification->>'state'='READY' THEN
   send_result:=jsonb_build_object('state','queued','operation_id',notification->'operation_id','notification_id',notification->'notification_id');
  ELSIF notification->>'state'='VOID' THEN
   send_result:=jsonb_build_object('state','not_sent','reason',notification->'void_code','notification_id',notification->'notification_id');
  ELSIF notification->>'state'='NOT_REQUESTED' THEN
   send_result:=jsonb_build_object('state','not_sent');
  ELSE
   RAISE EXCEPTION 'notification_result_invalid' USING ERRCODE='PT409';
  END IF;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
   VALUES((binding->>'tenant_id')::uuid,(binding->>'store_id')::uuid,(binding->>'principal_id')::uuid,
    'order.manual_created',jsonb_build_object('attempt_id',p_attempt,'order_id',p_attempt,'intent_generation',p_generation));
  v_receipt:=native||jsonb_build_object('state','COMMITTED','attempt_id',p_attempt,'intent_generation',p_generation,'send',send_result);
  -- The accessor checks wall-clock execute_by again after every local effect.
  -- The sole attempt-row UPDATE avoids acquiring the scope FK after domain locks.
  PERFORM ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
  UPDATE ops.order_attempts a SET state='COMMITTED',order_id=p_attempt,receipt=v_receipt,
   settled_at=clock_timestamp(),plan_ciphertext=NULL,plan_key_version=NULL
   WHERE a.tenant_id=(binding->>'tenant_id')::uuid AND a.store_id=(binding->>'store_id')::uuid
    AND a.attempt_id=p_attempt AND a.intent_generation=p_generation AND a.state='PENDING' AND a.admission_finished;
  IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_fenced' USING ERRCODE='PT409'; END IF;
  -- The native terminal trigger removes every subject index in this same Tx.
  result:=jsonb_build_object('state','COMMITTED','receipt',v_receipt);
 ELSE
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
 END IF;
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
 'C2 composite internal stages, runtime/claims-worker only: exact held generation/current-xid fence and original plaintext; canonical native domain suffix, Go allocation and credential/notification crypto. Pay stores the initial entry and prepares notification. Finish proves native order/hold/receipt/pay facts, completes claims and notification, writes redacted audit and terminal receipt, cryptoshreds plan and removes subject indexes atomically. Only the outer Go Tx commits;0182 rejects partials. No placement reauthorization, provider I/O or direct domain grants. Caller scope restored; stage data cannot supply tenant/owner/money.';
