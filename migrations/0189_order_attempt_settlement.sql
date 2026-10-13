-- Purpose: prove the five native order facts permitted in a C2 terminal receipt.
-- Depends on: 0178 exact-plan/current-xid fence, 0184 native Begin, 0187 pay entry,
-- and post_river/0005 + 0024 expiry job ownership (resolved only at execution).
-- Used by: the order-attempt coordinator inside its existing outer transaction.
-- Invariants: retained order lock, native provenance, frozen money/mode/state,
-- no caller facts, bearer disclosure, late prefix locks, or direct business DML.

CREATE FUNCTION checkout.order_attempt_terminal_facts(p_attempt uuid,p_plan_plaintext text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_variable
DECLARE ctx jsonb; binding jsonb; plan jsonb; tenant uuid; store uuid; buyer_id uuid;
 mode text; state text; order_key bigint; now_at timestamptz; expected_expiry timestamptz;
 o checkout.orders%ROWTYPE; result jsonb;
 prior_tenant text:=current_setting('app.tenant_id',true); prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true); prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true); prior_attempt text:=current_setting('app.order_attempt_id',true);
BEGIN
 ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 binding:=ctx->'binding'; plan:=ctx->'plan';
 tenant:=(binding->>'tenant_id')::uuid; store:=(binding->>'store_id')::uuid;
 buyer_id:=(binding->>'buyer_owner_id')::uuid; mode:=plan->'facts'->'selection'->>'payment_mode';
 IF mode IS NULL OR mode NOT IN ('bank_transfer','pay_at_pickup','cash_on_delivery') THEN
  RAISE EXCEPTION 'attempt_effects_incomplete' USING ERRCODE='PT409'; END IF;
 state:=CASE mode WHEN 'bank_transfer' THEN 'AWAITING_TRANSFER' WHEN 'pay_at_pickup' THEN 'CONFIRMED' ELSE 'AWAITING_COLLECTION' END;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',binding->>'principal_id',true),set_config('app.buyer_id',buyer_id::text,true),
  set_config('app.buyer_session_id','',true),set_config('app.order_attempt_id',p_attempt::text,true);
 -- This suffix must not repair a missing prefix by taking its lock late.
 order_key:=hashtextextended('checkout.order|'||tenant::text||'|'||store::text||'|'||p_attempt::text,0);
 IF NOT EXISTS(SELECT 1 FROM pg_locks l WHERE l.pid=pg_backend_pid() AND l.locktype='advisory'
  AND l.classid=((order_key>>32)&4294967295)::oid AND l.objid=(order_key&4294967295)::oid
  AND l.objsubid=1 AND l.mode='ExclusiveLock' AND l.granted) THEN
  RAISE EXCEPTION 'attempt_effects_incomplete' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO o FROM checkout.orders x WHERE x.tenant_id=tenant AND x.store_id=store
  AND x.owner_id=buyer_id AND x.id=p_attempt AND x.create_attempt_id=p_attempt
  AND x.creator_kind='order_attempt' AND x.creator_session_id IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'attempt_effects_incomplete' USING ERRCODE='PT409'; END IF;
 now_at:=clock_timestamp();
 expected_expiry:=o.created_at+CASE WHEN mode='bank_transfer'
  THEN make_interval(hours=>(plan->'facts'->'catalog'->'payment'->>'window_hours')::integer)
  ELSE interval '15 minutes' END;
 IF o.source IS DISTINCT FROM 'merchant_manual' OR o.generation IS DISTINCT FROM 1
  OR o.cart_version IS DISTINCT FROM 1 OR o.payment_mode IS DISTINCT FROM mode
  OR o.commercial_state IS DISTINCT FROM state OR o.fulfillment_state IS DISTINCT FROM 'MANUAL_UNASSIGNED'
  OR o.collection_state IS DISTINCT FROM (CASE WHEN mode='bank_transfer' THEN NULL::text ELSE 'PENDING' END)
  OR o.currency IS DISTINCT FROM plan->'facts'->'catalog'->'market'->>'currency'
  OR o.total_minor IS DISTINCT FROM (plan->'calculation'->>'total_minor')::bigint
  OR o.cod_surcharge_minor IS DISTINCT FROM (CASE WHEN mode='cash_on_delivery'
   THEN (plan->'facts'->'catalog'->'payment'->>'surcharge_minor')::bigint ELSE NULL::bigint END)
  OR o.cod_carrier IS DISTINCT FROM (CASE WHEN mode='cash_on_delivery'
   THEN plan->'facts'->'catalog'->'payment'->>'carrier' ELSE NULL::text END)
  OR o.snapshot->'quote'->'amount' IS DISTINCT FROM plan->'calculation'
  OR o.created_at<(binding->>'admitted_at')::timestamptz OR o.created_at>now_at
  OR o.expires_at IS DISTINCT FROM expected_expiry OR o.expires_at<=now_at
  OR EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=tenant AND a.store_id=store AND a.order_id=o.id)
  OR NOT EXISTS(SELECT 1 FROM inventory.reservations r WHERE r.tenant_id=tenant AND r.store_id=store
   AND r.id=o.id AND r.checkout_id=o.id AND r.buyer_owner_id=buyer_id AND r.buyer_session_id IS NULL
   AND r.creator_kind='order_attempt' AND r.create_attempt_id=p_attempt AND r.generation=1
   AND r.expires_at=o.expires_at AND r.state=CASE WHEN mode='bank_transfer' THEN 'HELD' ELSE 'COMMITTED' END)
  OR (mode='bank_transfer' AND NOT EXISTS(SELECT 1 FROM checkout.bank_transfers b
   WHERE b.tenant_id=tenant AND b.store_id=store AND b.owner_id=buyer_id AND b.order_id=o.id AND b.state='AWAITING'))
  OR NOT EXISTS(SELECT 1 FROM checkout.command_results c WHERE c.tenant_id=tenant AND c.store_id=store
   AND c.owner_id=buyer_id AND c.operation='checkout.begin' AND c.idempotency_key=p_attempt::text
   AND c.order_id=o.id AND c.creator_kind='order_attempt' AND c.create_attempt_id=p_attempt
   AND c.creator_session_id IS NULL AND c.request_hash=sha256(convert_to((plan->'facts'->'selection')::text,'UTF8'))
   AND c.response=jsonb_build_object('order_id',o.id,'reservation_id',o.id,'generation',1,
    'expires_at',o.expires_at,'job_id',o.job_id,'payment_mode',mode,'commercial_state',state))
  OR NOT EXISTS(SELECT 1 FROM checkout.order_pay_entries e WHERE e.tenant_id=tenant AND e.store_id=store
   AND e.owner_id=buyer_id AND e.order_id=o.id AND e.create_attempt_id=p_attempt AND e.link_version=1
   AND e.revoked_at IS NULL AND e.issued_at>=o.created_at AND e.issued_at<=now_at AND e.expires_at>now_at
   AND e.expires_at=least(o.expires_at,e.issued_at+interval '72 hours'))
  OR NOT EXISTS(SELECT 1 FROM river_expiry.river_job j WHERE j.id=o.job_id AND j.kind='checkout_expiry_v1'
   AND j.queue='checkout_expiry_v1' AND j.state='scheduled' AND j.max_attempts=25 AND j.scheduled_at=o.expires_at
   AND j.args=jsonb_build_object('order_id',o.id::text,'generation',1,'version',1)) THEN
  RAISE EXCEPTION 'attempt_effects_incomplete' USING ERRCODE='PT409'; END IF;
 result:=jsonb_build_object('order_id',o.id,'commercial_state',o.commercial_state,
  'payment_mode',o.payment_mode,'total_minor',o.total_minor,'currency',o.currency);
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
ALTER FUNCTION checkout.order_attempt_terminal_facts(uuid,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_terminal_facts(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_terminal_facts(uuid,text) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_terminal_facts(uuid,text) IS
 'Private five-field native terminal proof under current-xid exact plan and already-held canonical order lock. Requires original Begin money/mode/state, reservation, exact command receipt, current reusable v1 entry and exact scheduled expiry job. No new locks, writes, bearer/contact data, or caller-supplied facts. Restores all six caller GUCs.';

-- Only the trusted coordinator may add this native merchant audit. Its stored
-- admission principal and current-xid witness are authority; GUCs alone are not.
-- The existing membership FK remains: no late principal/scope authority recheck.
GRANT INSERT(tenant_id,store_id,id,principal_id,action,details) ON ops.audit_events TO commerce_order_attempt_owner;
CREATE POLICY order_attempt_native_audit ON ops.audit_events FOR INSERT TO commerce_order_attempt_owner
WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
 AND action='order.manual_created'
 AND EXISTS(SELECT 1 FROM ops.order_attempts a JOIN ops.order_attempt_execution_fences f
  ON f.attempt_id=a.attempt_id AND f.tenant_id=a.tenant_id AND f.store_id=a.store_id
  WHERE a.attempt_id=nullif(current_setting('app.order_attempt_id',true),'')::uuid
   AND a.tenant_id=audit_events.tenant_id AND a.store_id=audit_events.store_id
   AND a.admission_provenance->>'principal_id'=audit_events.principal_id::text
   AND a.state='PENDING' AND a.admission_finished AND f.execution_xid=pg_current_xact_id()
   AND audit_events.details=jsonb_build_object('attempt_id',a.attempt_id,'order_id',a.attempt_id,'intent_generation',a.intent_generation)));
