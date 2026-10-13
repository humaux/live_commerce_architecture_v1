-- Purpose: authenticated scope discovery and private C1 domain context.
-- Depends on: 0170 durable attempts, identity.resolve_access and exact domain owners.
-- Used by: internal/orderattempt C1, domain admission readers and the River job guard.
-- Invariants: principal/store comes from authentication; internal helpers cannot be
--   called by a runtime role; restored GUCs never replace durable admission authority.

GRANT USAGE ON SCHEMA identity TO commerce_order_attempt_owner;
GRANT EXECUTE ON FUNCTION identity.resolve_access(bytea,uuid,text)
 TO commerce_order_attempt_owner;
GRANT EXECUTE ON FUNCTION identity.merchant_access_denied(bytea,uuid,text[],uuid,uuid,bigint)
 TO commerce_order_attempt_owner;

-- PostgreSQL requires an UPDATE column grant for FOR UPDATE. Only the existing
-- NOLOGIN authentication owner gets it; the coordinator receives EXECUTE only.
GRANT UPDATE(id) ON identity.principals TO commerce_auth;
CREATE FUNCTION identity.lock_order_attempt_principal(p_hash bytea,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; denied text;
BEGIN
 IF NOT pg_has_role(session_user,'commerce_runtime','MEMBER')
  OR p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid order attempt authority' USING ERRCODE='PT400';
 END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' OR s.authz_revision<1 THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 -- No client principal argument or GUC can select the row to be locked. Holding
 -- this lock until commit serializes activation with principal lifecycle changes.
 PERFORM 1 FROM identity.principals p WHERE p.id=s.principal_id AND p.active FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 -- This VOLATILE check uses fresh clock/snapshot after any wait, unlike a nested
 -- STABLE resolve_access call. It also preserves exact store/revision binding.
 denied:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF denied IS NOT NULL THEN RAISE EXCEPTION 'order attempt access denied' USING ERRCODE=denied; END IF;
 RETURN jsonb_build_object('tenant_id',s.tenant_id,'store_id',p_store,
  'principal_id',s.principal_id,'authz_revision',s.authz_revision);
END $$;
ALTER FUNCTION identity.lock_order_attempt_principal(bytea,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.lock_order_attempt_principal(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.lock_order_attempt_principal(bytea,uuid) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION identity.lock_order_attempt_principal(bytea,uuid) IS
 'Internal coordinator activation helper, order-attempt-v1 §2/A1: authenticate current orders:read, lock actual principal before any scope lock, fresh recheck after wait, no GUC mutation or direct table grants. Ordinary C1 and retired replay use authenticate_order_attempt without this lock.';

-- Only the admission/sweep entries mint a witness; the INSERT-only River guard
-- consumes its empty job slot. A caller cannot forge it with GUCs or UPDATE(kind),
-- and a transaction ID from a prior committed admission authorizes no new job.
ALTER TABLE ops.order_attempts
 ADD COLUMN enqueue_xid xid8,
 ADD COLUMN enqueue_job_id bigint CHECK(enqueue_job_id>0),
 ADD CONSTRAINT order_attempt_enqueue_witness CHECK(enqueue_job_id IS NULL OR enqueue_xid IS NOT NULL);
CREATE POLICY order_attempt_enqueue_locator ON ops.order_attempts FOR SELECT TO commerce_order_attempt_owner
 USING(enqueue_xid=pg_current_xact_id());
COMMENT ON COLUMN ops.order_attempts.enqueue_xid IS
 'Protected C1/sweep transaction witness; only an INSERT guard can bind its job slot; never browser or worker-supplied authority.';
COMMENT ON COLUMN ops.order_attempts.enqueue_job_id IS
 'Fresh River INSERT identity witnessed inside enqueue_xid; deferred bidirectional checks require exact attempt/generation linkage.';

CREATE FUNCTION ops.authenticate_order_attempt(p_hash bytea,p_store uuid,p_permissions text[])
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; permission text; denied text;
BEGIN
 IF NOT pg_has_role(session_user,'commerce_runtime','MEMBER')
  OR p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
  OR cardinality(p_permissions) IS NULL OR cardinality(p_permissions)<1
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid order attempt authority' USING ERRCODE='PT400';
 END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' OR s.authz_revision<1 THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 FOREACH permission IN ARRAY p_permissions LOOP
  IF permission IS NULL OR permission NOT IN ('orders:read','catalog:read','inventory:reserve',
   'inbox:read','inbox:reply','live:read','live:manage','customers:read') THEN
   RAISE EXCEPTION 'invalid order attempt permission' USING ERRCODE='PT400';
  END IF;
 END LOOP;
 denied:=identity.merchant_access_denied(p_hash,p_store,p_permissions,s.tenant_id,s.principal_id,s.authz_revision);
 IF denied IS NOT NULL THEN RAISE EXCEPTION 'order attempt access denied' USING ERRCODE=denied; END IF;
 RETURN jsonb_build_object('tenant_id',s.tenant_id,'store_id',p_store,
  'principal_id',s.principal_id,'authz_revision',s.authz_revision);
END $$;
ALTER FUNCTION ops.authenticate_order_attempt(bytea,uuid,text[]) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.authenticate_order_attempt(bytea,uuid,text[]) FROM PUBLIC;
COMMENT ON FUNCTION ops.authenticate_order_attempt(bytea,uuid,text[]) IS
 'Internal coordinator only: verifies current merchant session and exact permissions, never trusts caller GUCs or supports a worker admission.';

CREATE FUNCTION ops.read_order_attempt_scope(p_hash bytea,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE auth jsonb; scope_row ops.order_attempt_scopes%ROWTYPE; last_attempt ops.order_attempts%ROWTYPE;
 prior_tenant text; prior_store text; prior_principal text; result jsonb;
BEGIN
 prior_tenant:=current_setting('app.tenant_id',true);
 prior_store:=current_setting('app.store_id',true);
 prior_principal:=current_setting('app.principal_id',true);
 -- A1 activation takes principal before scope. This entry accepts no client CAS
 -- identity; a retired scope remains available to separate exact replay entries.
 auth:=identity.lock_order_attempt_principal(p_hash,p_store);
 PERFORM set_config('app.tenant_id',auth->>'tenant_id',true);
 PERFORM set_config('app.store_id',auth->>'store_id',true);
 PERFORM set_config('app.principal_id',auth->>'principal_id',true);
 INSERT INTO ops.order_attempt_scopes(tenant_id,store_id,principal_id)
  VALUES((auth->>'tenant_id')::uuid,p_store,(auth->>'principal_id')::uuid)
  ON CONFLICT(tenant_id,store_id,principal_id,operation_family) WHERE retired_at IS NULL DO NOTHING;
 SELECT * INTO STRICT scope_row FROM ops.order_attempt_scopes
  WHERE tenant_id=(auth->>'tenant_id')::uuid AND store_id=p_store
   AND principal_id=(auth->>'principal_id')::uuid AND operation_family='merchant_order'
   AND retired_at IS NULL FOR SHARE;
 SELECT * INTO last_attempt FROM ops.order_attempts
  WHERE scope_id=scope_row.scope_id AND intent_generation=scope_row.current_generation;
 result:=jsonb_build_object('scope_id',scope_row.scope_id,'intent_generation',scope_row.current_generation,
  'attempt_id',last_attempt.attempt_id,'state',last_attempt.state,'receipt',last_attempt.receipt);
 -- Recheck current authorization after all reads; scope discovery creates no order intent.
 PERFORM ops.authenticate_order_attempt(p_hash,p_store,ARRAY['orders:read']);
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
ALTER FUNCTION ops.read_order_attempt_scope(bytea,uuid) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.read_order_attempt_scope(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.read_order_attempt_scope(bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION ops.read_order_attempt_scope(bytea,uuid) IS
 'order-attempt-v1 §§2/5 internal Go entry: current authenticated principal/store CAS and latest all-state reference; no new attempt, price or order; no-store at the future HTTP adapter.';

-- Domain helpers are narrower than the public C1 entry. They see only the candidate
-- authenticated by C1, in its still-open transaction, with its exact normalized body.
-- Runtime callers cannot EXECUTE this accessor or acquire a domain-owner membership.
CREATE FUNCTION ops.order_attempt_admission_context(p_attempt uuid,p_selection jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; principal uuid;
BEGIN
 IF p_attempt IS NULL OR p_selection IS NULL OR jsonb_typeof(p_selection)<>'object'
  OR octet_length(p_selection::text)>65536 THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 SELECT x.* INTO a FROM ops.order_attempts x
  WHERE x.attempt_id=p_attempt
   AND x.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND x.store_id=nullif(current_setting('app.store_id',true),'')::uuid;
 IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_found' USING ERRCODE='PT404'; END IF;
 SELECT s.principal_id INTO principal FROM ops.order_attempt_scopes s WHERE s.scope_id=a.scope_id;
 IF principal IS NULL OR principal IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid THEN
  RAISE EXCEPTION 'attempt_not_found' USING ERRCODE='PT404';
 END IF;
 IF a.state<>'PENDING' OR a.admission_finished THEN
  RAISE EXCEPTION 'attempt_not_admitting' USING ERRCODE='PT409';
 END IF;
 IF a.request_hash<>sha256(convert_to(p_selection::text,'UTF8')) THEN
  RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409';
 END IF;
 RETURN jsonb_build_object('attempt_id',a.attempt_id,'scope_id',a.scope_id,'intent_generation',a.intent_generation,
  'tenant_id',a.tenant_id,'store_id',a.store_id,'buyer_owner_id',a.buyer_owner_id,'principal_id',principal,
  'admitted_at',a.admitted_at,'execute_by',a.execute_by,'request_hash',encode(a.request_hash,'hex'),
  'private_owner',a.admission_provenance->'private_owner');
END $$;
ALTER FUNCTION ops.order_attempt_admission_context(uuid,jsonb) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.order_attempt_admission_context(uuid,jsonb) FROM PUBLIC;
GRANT USAGE ON SCHEMA ops TO commerce_checkout_writer,commerce_claims_writer,commerce_buyer_writer,commerce_privacy_writer;
GRANT EXECUTE ON FUNCTION ops.order_attempt_admission_context(uuid,jsonb)
 TO commerce_checkout_writer,commerce_claims_writer,commerce_buyer_writer,commerce_privacy_writer;
COMMENT ON FUNCTION ops.order_attempt_admission_context(uuid,jsonb) IS
 'Internal C1 helper accessor only: exact authenticated candidate/request binding, scoped owner/principal and immutable DB deadline; no domain table grants to coordinator.';
