-- Purpose: A2 durable, global plan-key retirement and version-bound C1 admission.
-- Depends on: 0170 durable identities, 0176 C1, protected River xid witness.
-- Used by: runtime C1 and separately provisioned key-admin login; never C2/startup.
-- Invariants: registry before all lower C1 locks; permanent non-reused markers;
-- exact authorized replay before fresh gates; admin-only global reference visibility.

CREATE ROLE commerce_order_attempt_key_admin NOLOGIN NOSUPERUSER NOBYPASSRLS
 NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA ops TO commerce_order_attempt_key_admin;

CREATE TABLE ops.order_attempt_plan_key_versions (
 key_version text PRIMARY KEY CHECK(key_version ~ '^[A-Za-z0-9_.-]{1,64}$'),
 retired_at timestamptz
);
ALTER TABLE ops.order_attempt_plan_key_versions OWNER TO commerce_order_attempt_owner;
ALTER TABLE ops.order_attempt_plan_key_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.order_attempt_plan_key_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY order_attempt_plan_key_owner ON ops.order_attempt_plan_key_versions
 TO commerce_order_attempt_owner USING(true) WITH CHECK(true);
REVOKE ALL ON ops.order_attempt_plan_key_versions FROM PUBLIC;
COMMENT ON TABLE ops.order_attempt_plan_key_versions IS
 'A2 global opaque never-reused version IDs only, no key material. Private C1/admin row-lock protocol; never acquired by C2.';

CREATE FUNCTION ops.guard_order_attempt_plan_key_version() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  RAISE EXCEPTION 'immutable_plan_key_version' USING ERRCODE='23514';
 END IF;
 IF NEW.key_version IS DISTINCT FROM OLD.key_version
  OR (OLD.retired_at IS NOT NULL AND NEW.retired_at IS DISTINCT FROM OLD.retired_at) THEN
  RAISE EXCEPTION 'immutable_plan_key_version' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION ops.guard_order_attempt_plan_key_version() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.guard_order_attempt_plan_key_version() FROM PUBLIC;
CREATE TRIGGER order_attempt_plan_key_immutable BEFORE UPDATE OR DELETE
 ON ops.order_attempt_plan_key_versions FOR EACH ROW
 EXECUTE FUNCTION ops.guard_order_attempt_plan_key_version();

-- Backfill only existing explicit pending versions. OA1 has no legacy quarantine
-- carrying plan keys; a later such record must join this refusal predicate.
INSERT INTO ops.order_attempt_plan_key_versions(key_version)
 SELECT DISTINCT plan_key_version FROM ops.order_attempts
 WHERE state='PENDING' AND plan_key_version IS NOT NULL ON CONFLICT(key_version) DO NOTHING;

-- This global SELECT policy is tied to the actual dedicated single-membership login,
-- never to tenant/store or a caller-supplied administrative GUC.
CREATE POLICY order_attempt_key_admin_locator ON ops.order_attempts
 FOR SELECT TO commerce_order_attempt_owner USING(
 pg_has_role(session_user,'commerce_order_attempt_key_admin','MEMBER')
 AND (SELECT count(*)=1 AND bool_and(r.rolname='commerce_order_attempt_key_admin'
  AND m.inherit_option AND NOT m.set_option)
  FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid
  JOIN pg_roles login ON login.oid=m.member WHERE login.rolname=session_user));

CREATE FUNCTION ops.retire_order_attempt_plan_key(p_version text) RETURNS text
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE retired timestamptz;
BEGIN
 IF NOT pg_has_role(session_user,'commerce_order_attempt_key_admin','MEMBER')
  OR NOT (SELECT count(*)=1 AND bool_and(r.rolname='commerce_order_attempt_key_admin'
   AND m.inherit_option AND NOT m.set_option)
   FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid
   JOIN pg_roles login ON login.oid=m.member WHERE login.rolname=session_user) THEN
  RAISE EXCEPTION 'invalid plan key authority' USING ERRCODE='42501';
 END IF;
 IF current_setting('transaction_isolation')<>'read committed'
  OR p_version IS NULL OR p_version !~ '^[A-Za-z0-9_.-]{1,64}$' THEN
  RAISE EXCEPTION 'invalid_plan_key' USING ERRCODE='PT400';
 END IF;
 INSERT INTO ops.order_attempt_plan_key_versions(key_version) VALUES(p_version)
  ON CONFLICT(key_version) DO NOTHING;
 SELECT v.retired_at INTO retired FROM ops.order_attempt_plan_key_versions v
  WHERE v.key_version=p_version FOR UPDATE;
 IF retired IS NOT NULL THEN RETURN 'retired'; END IF;
 -- The retained key row prevents new C1 references. C2 only removes references;
 -- no attempt lock is needed, avoiding key-row-after-attempt cycles with workers.
 IF EXISTS(SELECT 1 FROM ops.order_attempts a
  WHERE a.state='PENDING' AND a.plan_key_version=p_version) THEN
  RAISE EXCEPTION 'plan_key_referenced' USING ERRCODE='PT409';
 END IF;
 UPDATE ops.order_attempt_plan_key_versions SET retired_at=clock_timestamp()
  WHERE key_version=p_version;
 RETURN 'retired';
END $$;
ALTER FUNCTION ops.retire_order_attempt_plan_key(text) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.retire_order_attempt_plan_key(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.retire_order_attempt_plan_key(text)
 TO commerce_order_attempt_key_admin;
COMMENT ON FUNCTION ops.retire_order_attempt_plan_key(text) IS
 'Private A2 key-admin entry: permanent global version retirement under the C1 key row, fixed retired/plan_key_referenced only; no identifiers or counts.';

-- The chosen version is immutable from candidate INSERT, before the broader
-- admission_finished guard freezes provenance. This guard acquires no row lock.
CREATE FUNCTION ops.guard_order_attempt_admission_plan_key() RETURNS trigger
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.admission_provenance->>'plan_key_version'
  IS DISTINCT FROM OLD.admission_provenance->>'plan_key_version' THEN
  RAISE EXCEPTION 'immutable_admission_plan_key' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION ops.guard_order_attempt_admission_plan_key() OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.guard_order_attempt_admission_plan_key() FROM PUBLIC;
CREATE TRIGGER order_attempt_admission_plan_key_immutable BEFORE UPDATE ON ops.order_attempts
 FOR EACH ROW EXECUTE FUNCTION ops.guard_order_attempt_admission_plan_key();

-- Remove the old callable shape entirely; finish cannot seal an unbound candidate.
DROP FUNCTION ops.prepare_order_attempt_admission(bytea,uuid,uuid,bigint,uuid,bytea,jsonb);

CREATE FUNCTION ops.prepare_order_attempt_admission(p_hash bytea,p_store uuid,p_scope uuid,
 p_generation bigint,p_previous uuid,p_key bytea,p_selection jsonb,p_plan_key_version text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 auth jsonb; fresh_auth jsonb; sources jsonb; subjects jsonb; claim_facts jsonb; catalog_facts jsonb; facts jsonb;
 a ops.order_attempts%ROWTYPE; s ops.order_attempt_scopes%ROWTYPE;
 owner_id uuid; new_attempt_id uuid; auth_principal uuid; auth_tenant uuid; prior_attempt uuid;
 body_hash bytea; admitted timestamptz; result jsonb; replay boolean:=false; scope_locked boolean:=false;
 key_locked boolean:=false; key_retired_at timestamptz;
 permissions text[]:=ARRAY['orders:read','catalog:read','inventory:reserve'];
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true);
 prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true);
BEGIN
 auth:=ops.authenticate_order_attempt(p_hash,p_store,ARRAY['orders:read']);
 auth_tenant:=(auth->>'tenant_id')::uuid; auth_principal:=(auth->>'principal_id')::uuid;
 IF p_scope IS NULL OR p_generation IS NULL OR p_generation<0 OR p_generation=9223372036854775807
  OR p_key IS NULL OR octet_length(p_key)<>32 OR p_selection IS NULL
  OR jsonb_typeof(p_selection)<>'object' OR octet_length(p_selection::text)>65536 THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 body_hash:=sha256(convert_to(p_selection::text,'UTF8'));
 PERFORM set_config('app.tenant_id',auth_tenant::text,true);
 PERFORM set_config('app.store_id',p_store::text,true);
 PERFORM set_config('app.principal_id',auth_principal::text,true);
 PERFORM set_config('app.buyer_id','',true);
 PERFORM set_config('app.buyer_session_id','',true);
 LOOP
  SELECT x.* INTO a FROM ops.order_attempts x
   WHERE x.tenant_id=auth_tenant AND x.operation_family='merchant_order' AND x.transport_key_digest=p_key;
  IF FOUND THEN
   -- Never query only PENDING or require the supplied scope to still be active.
   -- Original CAS is persisted; a later active generation cannot change replay.
   IF a.store_id<>p_store OR a.scope_id<>p_scope OR a.intent_generation<>p_generation+1
    OR a.request_hash<>body_hash
    OR a.admission_provenance->>'principal_id' IS DISTINCT FROM auth_principal::text
    OR a.admission_provenance->>'previous_attempt_id' IS DISTINCT FROM p_previous::text THEN
    RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409';
   END IF;
   SELECT x.* INTO s FROM ops.order_attempt_scopes x WHERE x.scope_id=a.scope_id;
   IF NOT FOUND OR s.principal_id<>auth_principal THEN
    RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409';
   END IF;
   IF s.retired_at IS NOT NULL AND clock_timestamp()>=s.retired_at+interval '90 days' THEN
    RAISE EXCEPTION 'receipt_expired' USING ERRCODE='PT409';
   END IF;
   replay:=true;
   EXIT;
  END IF;
  EXIT WHEN scope_locked;
  IF NOT key_locked THEN
   IF p_plan_key_version IS NULL OR p_plan_key_version !~ '^[A-Za-z0-9_.-]{1,64}$' THEN
    RAISE EXCEPTION 'invalid_plan_key' USING ERRCODE='PT400';
   END IF;
   -- Serialize concurrent insertion of an unknown version before every lower C1 lock.
   INSERT INTO ops.order_attempt_plan_key_versions(key_version) VALUES(p_plan_key_version)
    ON CONFLICT(key_version) DO NOTHING;
   SELECT v.retired_at INTO key_retired_at FROM ops.order_attempt_plan_key_versions v
    WHERE v.key_version=p_plan_key_version FOR SHARE;
   -- Unrelated C1 transactions may share a deployed version. Retirement's
   -- exclusive row lock still waits for every retained admission reference.
   key_locked:=true;
   -- A key-row waiter may now see the same transport winner. Reread before
   -- retirement/placement checks so retained exact replay still precedes gates.
   CONTINUE;
  END IF;
  -- A2-M1: remove only this in-lock retired-version refusal for the native mutation.
  IF key_retired_at IS NOT NULL THEN
   RAISE EXCEPTION 'plan_key_retired' USING ERRCODE='PT409';
  END IF;
  -- Mutable placement checks run only after proving this is a fresh transport key.
  IF p_selection->'send_payment_link'='true'::jsonb THEN permissions:=permissions||'inbox:reply'::text; END IF;
  fresh_auth:=ops.authenticate_order_attempt(p_hash,p_store,permissions);
  IF fresh_auth IS DISTINCT FROM auth THEN RAISE EXCEPTION 'authorization_changed' USING ERRCODE='PT403'; END IF;
  sources:=customers.order_attempt_sources(p_selection);
  owner_id:=(sources->>'buyer_owner_id')::uuid;
  subjects:=sources->'subjects';
  IF owner_id IS NULL THEN
   owner_id:=gen_random_uuid();
   subjects:=jsonb_build_array(jsonb_build_object('kind','private_owner','id',owner_id));
  END IF;
  PERFORM ops.lock_order_attempt_subjects(auth_tenant,p_store,subjects);
  SELECT x.* INTO s FROM ops.order_attempt_scopes x
   WHERE x.scope_id=p_scope AND x.tenant_id=auth_tenant AND x.store_id=p_store
    AND x.principal_id=auth_principal AND x.operation_family='merchant_order' FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'intent_conflict' USING ERRCODE='PT409'; END IF;
  scope_locked:=true;
  -- A waiter must re-read the transport authority before checking the active CAS.
 END LOOP;
 IF NOT replay THEN
  IF s.retired_at IS NOT NULL OR s.current_generation<>p_generation THEN
   RAISE EXCEPTION 'intent_conflict' USING ERRCODE='PT409';
  END IF;
  IF p_generation=0 THEN
   IF p_previous IS NOT NULL THEN RAISE EXCEPTION 'intent_conflict' USING ERRCODE='PT409'; END IF;
  ELSE
   SELECT x.attempt_id INTO prior_attempt FROM ops.order_attempts x
    WHERE x.scope_id=s.scope_id AND x.intent_generation=p_generation AND x.state IN ('COMMITTED','ABORTED');
   IF prior_attempt IS NULL OR prior_attempt IS DISTINCT FROM p_previous THEN
    RAISE EXCEPTION 'intent_conflict' USING ERRCODE='PT409';
   END IF;
  END IF;
  IF customers.order_attempt_sources(p_selection) IS DISTINCT FROM sources THEN
   RAISE EXCEPTION 'subject_binding_changed' USING ERRCODE='PT409';
  END IF;
  new_attempt_id:=gen_random_uuid(); admitted:=clock_timestamp();
  INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id)
   VALUES(new_attempt_id,auth_tenant,p_store,owner_id);
  BEGIN
  INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
   transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,
   admitted_at,execute_by,state,enqueue_xid)
   VALUES(new_attempt_id,auth_tenant,p_store,s.scope_id,p_generation+1,p_key,body_hash,1,owner_id,
    jsonb_build_object('private_owner',sources->>'buyer_owner_id' IS NULL,'source_subjects',subjects,
     'authz_revision',auth->'authz_revision','principal_id',auth_principal,'previous_attempt_id',p_previous,
     'send_payment_link',p_selection->'send_payment_link','plan_key_version',p_plan_key_version),admitted,admitted+interval '15 minutes','PENDING',pg_current_xact_id())
   RETURNING * INTO a;
  EXCEPTION WHEN unique_violation THEN
   -- A concurrent different-scope admission may win the tenant-wide key.
   -- Only that proven key collision is a typed conflict; other faults propagate.
   IF EXISTS(SELECT 1 FROM ops.order_attempts x WHERE x.tenant_id=auth_tenant
    AND x.operation_family='merchant_order' AND x.transport_key_digest=p_key) THEN
    RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409';
   END IF;
   RAISE;
  END;
  UPDATE ops.order_attempt_scopes SET current_generation=p_generation+1 WHERE scope_id=s.scope_id;
  PERFORM buyer.order_attempt_bind_owner(a.attempt_id,p_selection);
  INSERT INTO ops.order_attempt_subjects(tenant_id,store_id,subject_kind,subject_id,attempt_id)
   SELECT auth_tenant,p_store,x->>'kind',(x->>'id')::uuid,a.attempt_id FROM jsonb_array_elements(subjects) x;
  claim_facts:=claims.order_attempt_admission_origins(a.attempt_id,p_selection);
  catalog_facts:=checkout.order_attempt_admission_facts(a.attempt_id,p_selection);
  facts:=jsonb_build_object('selection',p_selection,'catalog',catalog_facts,'claims',claim_facts);
  UPDATE ops.order_attempts SET facts_digest=sha256(convert_to(facts::text,'UTF8'))
   WHERE ops.order_attempts.attempt_id=a.attempt_id RETURNING * INTO a;
 END IF;
 IF replay THEN
  -- A concurrent winner can become visible only after a subject/scope wait.
  -- Refresh read authority at disclosure; placement revocation/revision drift
  -- must not invalidate an existing attempt. This read takes no principal lock.
  fresh_auth:=ops.authenticate_order_attempt(p_hash,p_store,ARRAY['orders:read']);
  IF (fresh_auth->>'tenant_id')::uuid IS DISTINCT FROM a.tenant_id
   OR (fresh_auth->>'store_id')::uuid IS DISTINCT FROM a.store_id
   OR (fresh_auth->>'principal_id')::uuid IS DISTINCT FROM auth_principal THEN
   RAISE EXCEPTION 'authorization_changed' USING ERRCODE='PT403';
  END IF;
 END IF;
 result:=jsonb_build_object('attempt_id',a.attempt_id,'scope_id',a.scope_id,'intent_generation',a.intent_generation,
  'tenant_id',a.tenant_id,'store_id',a.store_id,'principal_id',auth_principal,'buyer_owner_id',a.buyer_owner_id,
  'admitted_at',a.admitted_at,'execute_by',a.execute_by,'request_hash',encode(a.request_hash,'hex'),
  'facts_digest',encode(a.facts_digest,'hex'),'facts',facts,'state',a.state,'replay',replay,'receipt',a.receipt);
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 PERFORM set_config('app.buyer_id',coalesce(prior_buyer,''),true);
 PERFORM set_config('app.buyer_session_id',coalesce(prior_session,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 PERFORM set_config('app.buyer_id',coalesce(prior_buyer,''),true);
 PERFORM set_config('app.buyer_session_id',coalesce(prior_session,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.prepare_order_attempt_admission(bytea,uuid,uuid,bigint,uuid,bytea,jsonb,text) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.prepare_order_attempt_admission(bytea,uuid,uuid,bigint,uuid,bytea,jsonb,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.prepare_order_attempt_admission(bytea,uuid,uuid,bigint,uuid,bytea,jsonb,text) TO commerce_runtime;
COMMENT ON FUNCTION ops.prepare_order_attempt_admission(bytea,uuid,uuid,bigint,uuid,bytea,jsonb,text) IS
 'C1 runtime entry: all-state exact replay before placement gates, then explicit CAS under subject/scope fences; unfinished effects cannot commit. No synthetic buyer capability.';

CREATE OR REPLACE FUNCTION ops.finish_order_attempt_admission(p_hash bytea,p_attempt uuid,p_generation bigint,
 p_plan_digest bytea,p_key_version text,p_ciphertext bytea)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a ops.order_attempts%ROWTYPE; auth jsonb; principal uuid; result jsonb;
 permissions text[]:=ARRAY['orders:read','catalog:read','inventory:reserve'];
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
 prior_principal text:=current_setting('app.principal_id',true);
 prior_buyer text:=current_setting('app.buyer_id',true);
 prior_session text:=current_setting('app.buyer_session_id',true);
BEGIN
 IF NOT pg_has_role(session_user,'commerce_runtime','MEMBER')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid order attempt authority' USING ERRCODE='PT400';
 END IF;
 -- Protected xid, not ambient tenant GUCs, locates this transaction's candidate.
 SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=p_attempt AND x.enqueue_xid=pg_current_xact_id();
 IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_admitting' USING ERRCODE='PT409'; END IF;
 IF a.admission_provenance->'send_payment_link'='true'::jsonb THEN permissions:=permissions||'inbox:reply'::text; END IF;
 auth:=ops.authenticate_order_attempt(p_hash,a.store_id,permissions);
 IF a.tenant_id IS DISTINCT FROM (auth->>'tenant_id')::uuid
  OR a.admission_provenance->>'principal_id' IS DISTINCT FROM auth->>'principal_id'
  OR a.admission_provenance->'authz_revision' IS DISTINCT FROM auth->'authz_revision' THEN
  RAISE EXCEPTION 'authorization_changed' USING ERRCODE='PT403';
 END IF;
 PERFORM set_config('app.tenant_id',a.tenant_id::text,true);
 PERFORM set_config('app.store_id',a.store_id::text,true);
 PERFORM set_config('app.principal_id',auth->>'principal_id',true);
 PERFORM set_config('app.buyer_id','',true);
 PERFORM set_config('app.buyer_session_id','',true);
 SELECT s.principal_id INTO principal FROM ops.order_attempt_scopes s WHERE s.scope_id=a.scope_id;
 IF principal IS DISTINCT FROM (auth->>'principal_id')::uuid THEN
  RAISE EXCEPTION 'attempt_not_admitting' USING ERRCODE='PT409';
 END IF;
 IF a.state<>'PENDING' OR a.admission_finished OR a.intent_generation IS DISTINCT FROM p_generation
  OR a.facts_digest IS NULL OR a.enqueue_job_id IS NULL THEN
  RAISE EXCEPTION 'attempt_not_admitting' USING ERRCODE='PT409';
 END IF;
 IF p_plan_digest IS NULL OR octet_length(p_plan_digest)<>32 OR p_key_version IS NULL
  OR p_key_version !~ '^[A-Za-z0-9_.-]{1,64}$' OR p_ciphertext IS NULL
  OR octet_length(p_ciphertext) NOT BETWEEN 29 AND 1048604 THEN
  RAISE EXCEPTION 'invalid_plan' USING ERRCODE='PT400';
 END IF;
 -- The private same-xid candidate records the version whose row prepare retains.
 -- Never acquire a key row here or in C2 after the attempt/subject/scope fence.
 IF a.admission_provenance->>'plan_key_version' IS DISTINCT FROM p_key_version THEN
  RAISE EXCEPTION 'plan_key_version_mismatch' USING ERRCODE='PT409';
 END IF;
 IF NOT ops.order_attempt_job_matches(a.enqueue_job_id,a.attempt_id,a.intent_generation) THEN
  RAISE EXCEPTION 'invalid_enqueue_witness' USING ERRCODE='23514';
 END IF;
 IF clock_timestamp()>=a.execute_by THEN RAISE EXCEPTION 'admission_expired' USING ERRCODE='PT409'; END IF;
 UPDATE ops.order_attempts SET admission_finished=true,plan_digest=p_plan_digest,
  plan_key_version=p_key_version,plan_ciphertext=p_ciphertext,execution_job_id=a.enqueue_job_id
  WHERE attempt_id=a.attempt_id RETURNING * INTO a;
 result:=jsonb_build_object('attempt_id',a.attempt_id,'scope_id',a.scope_id,'intent_generation',a.intent_generation,
  'tenant_id',a.tenant_id,'store_id',a.store_id,'principal_id',principal,'buyer_owner_id',a.buyer_owner_id,
  'admitted_at',a.admitted_at,'execute_by',a.execute_by,'request_hash',encode(a.request_hash,'hex'),
  'facts_digest',encode(a.facts_digest,'hex'),'facts',NULL,'state',a.state,'replay',false,'receipt',a.receipt);
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 PERFORM set_config('app.buyer_id',coalesce(prior_buyer,''),true);
 PERFORM set_config('app.buyer_session_id',coalesce(prior_session,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 PERFORM set_config('app.principal_id',coalesce(prior_principal,''),true);
 PERFORM set_config('app.buyer_id',coalesce(prior_buyer,''),true);
 PERFORM set_config('app.buyer_session_id',coalesce(prior_session,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.finish_order_attempt_admission(bytea,uuid,bigint,bytea,text,bytea) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.finish_order_attempt_admission(bytea,uuid,bigint,bytea,text,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.finish_order_attempt_admission(bytea,uuid,bigint,bytea,text,bytea) TO commerce_runtime;
COMMENT ON FUNCTION ops.finish_order_attempt_admission(bytea,uuid,bigint,bytea,text,bytea) IS
 'C1 same-xid finish after real River InsertTx: recheck current merchant admission authority, exact generation/deadline and private enqueue witness; seal plan atomically.';
