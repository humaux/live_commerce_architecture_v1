-- Purpose: attempt-first execution fencing and narrow domain context access.
-- Depends: 0170 durable identity, 0172 merchant authentication, sealed C1 plans.
-- Used by: inline/claims execution and later domain-owned C2/abort helpers.
-- Invariants: no scope/subject/principal/owner locks; current read authority for
-- runtime disclosure; only a protected current-transaction witness admits helpers.
-- This migration does not execute, settle, abort or enqueue an order attempt.

ALTER TABLE ops.order_attempts ADD COLUMN execution_xid xid8;
COMMENT ON COLUMN ops.order_attempts.execution_xid IS
 'Protected execution fence witness: assigned only after attempt FOR UPDATE; private helpers require pg_current_xact_id(), never a caller GUC or a previous transaction witness.';
CREATE POLICY order_attempt_execution_locator ON ops.order_attempts
 FOR SELECT TO commerce_order_attempt_owner
 USING(execution_xid=pg_current_xact_id() AND state='PENDING' AND admission_finished);

-- Schema visibility is required to call the exact granted entries; it does not
-- grant table access, admission, or EXECUTE on any private context helper.
GRANT USAGE ON SCHEMA ops TO commerce_claims_worker,commerce_inbox_writer;

CREATE FUNCTION ops.order_attempt_fence_context(p_attempt uuid,p_generation bigint)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; result jsonb;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true);
BEGIN
 -- The public entry restores its caller's context. Empty it before RLS so even
 -- malformed caller UUID GUCs cannot become authority or break the xid locator.
 PERFORM set_config('app.tenant_id','',true);
 PERFORM set_config('app.store_id','',true);
 PERFORM set_config('app.principal_id','',true);
 SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=p_attempt
  AND x.intent_generation=p_generation AND x.state='PENDING' AND x.admission_finished
  AND x.execution_xid=pg_current_xact_id();
 IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_fenced' USING ERRCODE='PT409'; END IF;
 result:=jsonb_build_object('schema',a.request_schema_version,'attempt_id',a.attempt_id,
  'scope_id',a.scope_id,'generation',a.intent_generation,'tenant_id',a.tenant_id,
  'store_id',a.store_id,'principal_id',a.admission_provenance->>'principal_id',
  'buyer_owner_id',a.buyer_owner_id,'admitted_at',a.admitted_at,'execute_by',a.execute_by,
  'request_hash',ARRAY(SELECT get_byte(a.request_hash,n) FROM generate_series(0,31) n),
  'facts_digest',ARRAY(SELECT get_byte(a.facts_digest,n) FROM generate_series(0,31) n));
 -- No deadline check here: erasure, expiry and permanently unavailable plan-key
 -- aborts need this identity under the held fence without decoding a private plan.
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.order_attempt_fence_context(uuid,bigint) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.order_attempt_fence_context(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.order_attempt_fence_context(uuid,bigint)
 TO commerce_buyer_writer,commerce_checkout_writer,commerce_claims_writer,commerce_inbox_writer;
COMMENT ON FUNCTION ops.order_attempt_fence_context(uuid,bigint) IS
 'Private domain-owner accessor: exact pending admitted generation and protected current-xid fence; metadata only, no plaintext, deadline refusal, row/advisory locks or GUC authority. Enables zero-effects abort even after key loss/expiry.';

CREATE FUNCTION ops.lock_order_attempt(p_hash bytea,p_store uuid,p_attempt uuid,p_generation bigint)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; s ops.order_attempt_scopes%ROWTYPE;
 auth jsonb; fresh_auth jsonb; result jsonb; binding jsonb;
 worker boolean:=pg_has_role(session_user,'commerce_claims_worker','MEMBER');
 runtime boolean:=pg_has_role(session_user,'commerce_runtime','MEMBER');
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true);
BEGIN
 IF p_store IS NULL OR p_attempt IS NULL OR p_generation IS NULL OR p_generation<1
  OR current_setting('transaction_isolation')<>'read committed'
  OR (worker AND runtime) OR NOT (worker OR runtime)
  OR (worker AND p_hash IS NOT NULL)
  OR (runtime AND (p_hash IS NULL OR octet_length(p_hash)<>32)) THEN
  RAISE EXCEPTION 'invalid_execution_authority' USING ERRCODE='PT400';
 END IF;
 PERFORM set_config('app.tenant_id','',true);
 PERFORM set_config('app.store_id','',true);
 PERFORM set_config('app.principal_id','',true);
 IF worker THEN
  -- Actual claims session_user is the only locator lane. This unlocked read is
  -- discovery, not a fence: identity/state/generation are re-read below under lock.
  SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=p_attempt AND x.store_id=p_store;
  IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_found' USING ERRCODE='PT404'; END IF;
  PERFORM set_config('app.tenant_id',a.tenant_id::text,true);
  PERFORM set_config('app.store_id',a.store_id::text,true);
 ELSE
  auth:=ops.authenticate_order_attempt(p_hash,p_store,ARRAY['orders:read']);
  PERFORM set_config('app.tenant_id',auth->>'tenant_id',true);
  PERFORM set_config('app.store_id',p_store::text,true);
  PERFORM set_config('app.principal_id',auth->>'principal_id',true);
 END IF;
 -- FIRST locking statement. Never lock scope, principal, subject barriers or
 -- owner here, including after this statement; domain helpers own the C2 suffix.
 SELECT x.* INTO a FROM ops.order_attempts x
  WHERE x.attempt_id=p_attempt AND x.store_id=p_store
   AND x.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND (worker OR x.admission_provenance->>'principal_id'=auth->>'principal_id')
  FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_found' USING ERRCODE='PT404'; END IF;
 IF runtime THEN
  -- VOLATILE authentication checks clock and authorization after the row wait.
  -- Placement permissions are deliberately absent from execution disclosure.
  fresh_auth:=ops.authenticate_order_attempt(p_hash,p_store,ARRAY['orders:read']);
  IF fresh_auth->>'tenant_id' IS DISTINCT FROM a.tenant_id::text
   OR fresh_auth->>'principal_id' IS DISTINCT FROM a.admission_provenance->>'principal_id' THEN
   RAISE EXCEPTION 'attempt_not_found' USING ERRCODE='PT404';
  END IF;
 END IF;
 IF a.intent_generation<>p_generation OR NOT a.admission_finished THEN
  RAISE EXCEPTION 'intent_conflict' USING ERRCODE='PT409';
 END IF;
 SELECT x.* INTO s FROM ops.order_attempt_scopes x WHERE x.scope_id=a.scope_id
  AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id
  AND x.principal_id::text=a.admission_provenance->>'principal_id';
 IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_found' USING ERRCODE='PT404'; END IF;
 IF s.retired_at IS NOT NULL AND clock_timestamp()>=s.retired_at+interval '90 days' THEN
  RAISE EXCEPTION 'receipt_expired' USING ERRCODE='PT409';
 END IF;
 IF a.state IN ('COMMITTED','ABORTED') THEN
  -- Terminal replay does not update even execution_xid and needs no plan body.
  result:=jsonb_build_object('state',a.state,'receipt',a.receipt);
 ELSE
  IF a.state<>'PENDING' THEN RAISE EXCEPTION 'intent_conflict' USING ERRCODE='PT409'; END IF;
  UPDATE ops.order_attempts SET execution_xid=pg_current_xact_id() WHERE attempt_id=a.attempt_id;
  binding:=ops.order_attempt_fence_context(a.attempt_id,a.intent_generation);
  result:=jsonb_build_object('state',a.state,'receipt',a.receipt,'binding',binding,
   'envelope',jsonb_build_object('key_version',a.plan_key_version,
    'ciphertext',replace(encode(a.plan_ciphertext,'base64'),E'\n',''),
    'digest',ARRAY(SELECT get_byte(a.plan_digest,n) FROM generate_series(0,31) n)));
 END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.lock_order_attempt(bytea,uuid,uuid,bigint) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.lock_order_attempt(bytea,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.lock_order_attempt(bytea,uuid,uuid,bigint)
 TO commerce_runtime,commerce_claims_worker;
COMMENT ON FUNCTION ops.lock_order_attempt(bytea,uuid,uuid,bigint) IS
 'order-attempt-v1 §§1/3/6k: attempt-first FOR UPDATE, exact generation and post-wait current runtime orders:read; actual claims worker requires NULL auth. Returns stored pending AEAD envelope and protected xid, or immutable retained terminal receipt; no domain effects or earlier-level locks.';

CREATE FUNCTION ops.order_attempt_execution_context(p_attempt uuid,p_plan_plaintext text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; binding jsonb; plan jsonb; result jsonb;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true);
BEGIN
 PERFORM set_config('app.tenant_id','',true);
 PERFORM set_config('app.store_id','',true);
 PERFORM set_config('app.principal_id','',true);
 SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=p_attempt
  AND x.execution_xid=pg_current_xact_id() AND x.state='PENDING' AND x.admission_finished;
 IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_fenced' USING ERRCODE='PT409'; END IF;
 binding:=ops.order_attempt_fence_context(p_attempt,a.intent_generation);
 IF clock_timestamp()>=a.execute_by THEN
  RAISE EXCEPTION 'attempt_expired' USING ERRCODE='PT409';
 END IF;
 IF p_plan_plaintext IS NULL OR octet_length(p_plan_plaintext)>1048576
  OR sha256(convert_to(p_plan_plaintext,'UTF8')) IS DISTINCT FROM a.plan_digest THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400';
 END IF;
 -- Digest the exact Go JSON bytes before parsing, not jsonb's reformatted text.
 -- Timestamp offsets may differ while representing the same immutable instant.
 BEGIN
  plan:=p_plan_plaintext::jsonb;
  IF jsonb_typeof(plan) IS DISTINCT FROM 'object'
   OR (plan-ARRAY['facts','calculation','admitted_at','execute_by']::text[])
    IS DISTINCT FROM (binding-ARRAY['admitted_at','execute_by']::text[])
   OR (plan->>'admitted_at')::timestamptz IS DISTINCT FROM a.admitted_at
   OR (plan->>'execute_by')::timestamptz IS DISTINCT FROM a.execute_by
   OR jsonb_typeof(plan->'calculation') IS DISTINCT FROM 'object'
   OR jsonb_typeof(plan->'facts') IS DISTINCT FROM 'object'
   OR jsonb_typeof(plan->'facts'->'selection') IS DISTINCT FROM 'object'
   OR sha256(convert_to((plan->'facts')::text,'UTF8')) IS DISTINCT FROM a.facts_digest
   OR sha256(convert_to((plan->'facts'->'selection')::text,'UTF8')) IS DISTINCT FROM a.request_hash THEN
   RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400';
  END IF;
 EXCEPTION WHEN invalid_text_representation OR invalid_datetime_format OR datetime_field_overflow THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400';
 END;
 result:=jsonb_build_object('binding',binding,'plan',plan);
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.order_attempt_execution_context(uuid,text) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.order_attempt_execution_context(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.order_attempt_execution_context(uuid,text)
 TO commerce_buyer_writer,commerce_checkout_writer,commerce_claims_writer,commerce_inbox_writer;
COMMENT ON FUNCTION ops.order_attempt_execution_context(uuid,text) IS
 'Private C2 domain accessor: protected current-xid pending fence, unexpired deadline, exact stored Go plaintext digest, immutable binding and canonical facts/selection digests. Returns trusted binding plus parsed plan; no caller replacement identity/money, table grants, locks, or settlement.';
