-- Migration 0084 (T21-02/T21-03, unit worker-authority-split): one DB authority per worker process.
--
-- Owns: the five NOLOGIN worker authorities that replace the shared commerce_worker role --
--   commerce_payment_worker (payment-worker SANDBOX/PROVIDER_MOCK), commerce_payment_live (payment-worker LIVE),
--   commerce_expiry_worker, commerce_ads_worker, commerce_claims_worker -- and, for every object created by
--   migrations 0001-0080, the move of each commerce_worker privilege to exactly the authorities whose Go process uses it
--   (derived from the SQL each cmd/*-worker runs, see output/worker-authority-split/privileges-after.tsv); the
--   integration.operation_lane / operation_authority_ok / profile_authority_ok helpers; the claim_operation /
--   complete_operation lane gate; and the profile fence of the three private require_* guards.
-- Non-goals: no new table, no change to any definer's business logic, no new secret or login (the logins already exist,
--   deploy/postgres/logins.tsv maps them to the new authorities), no change to the Stripe live contract. Objects created by
--   post_river/*.sql and the main/payment/expiry River grants are re-pointed in post_river/0018 and migrations/migrate.go,
--   because they do not exist yet when this numbered phase runs on a fresh database.
-- commerce_worker stays as an EMPTY legacy role: it has no remaining grant, policy or membership use. It is not dropped
--   because older checksummed migrations name it (GRANT/CREATE POLICY ... commerce_worker), tests assert its emptiness, and a
--   login still in it is rejected by every platform.Validate*/Open*Pool.
-- Lanes (integration.operation_lane): payment = provider stripe|payuni or actor BUYER_PAYMENT_QUERY|PAYMENT_REFUND; ads =
--   provider meta_ads|meta_dataset; media = actor MEDIA_ATTEMPT or provider livekit (no authority here; media has its own);
--   default = everything else, i.e. the claims-worker's external_operation_v1 dispatcher lane (facebook, instagram,
--   ecpay_logistics and the T06 ledger fixtures). A compromised ads/claims/expiry worker therefore cannot see, claim or
--   complete a payment operation, and a payment worker cannot touch an ads or claims one.
-- session_user, not current_user: inside a SECURITY DEFINER function current_user is the function owner; the login that
--   connected is session_user (post_river/0015 and 0028 use the same test). Superusers are members of every role, so the
--   migration owner and test fixtures pass the membership tests; the media lane is refused for everyone.
-- Callers: the five worker processes only; nothing here is executable by the merchant, buyer or ingress authorities.

-- Idempotent on purpose: a historical-state fixture (tests/foundation, populated pre-0064/0060 databases) pre-creates these
-- roles because the current migrate.go grants River privileges to them before this file can run; production creates them here.
DO $$
DECLARE r text;
BEGIN
 FOREACH r IN ARRAY ARRAY['commerce_payment_worker','commerce_payment_live','commerce_expiry_worker','commerce_ads_worker','commerce_claims_worker'] LOOP
  IF to_regrole(r) IS NULL THEN
   EXECUTE format('CREATE ROLE %I NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION',r);
  END IF;
 END LOOP;
END $$;
COMMENT ON ROLE commerce_payment_worker IS 'worker authority of cmd/payment-worker profile SANDBOX/PROVIDER_MOCK (lc_payment_sandbox): river_payment lifecycle, Stripe/PAYUNi worker definers, payments.apply_capture. Never LIVE, never another lane. Migration 0084.';
COMMENT ON ROLE commerce_payment_live IS 'worker authority of cmd/payment-worker profile LIVE (lc_payment_live): same SQL surface as commerce_payment_worker but the profile fence only admits LIVE attempts. Migration 0084.';
COMMENT ON ROLE commerce_expiry_worker IS 'worker authority of cmd/expiry-worker (lc_expiry_worker): river_expiry lifecycle, checkout.expire_held, checkout.expiry_queue_ready. No integration or payments access. Migration 0084.';
COMMENT ON ROLE commerce_ads_worker IS 'worker authority of cmd/ads-worker (lc_ads_worker): ads/CAPI definers, integration.load_meta_ads_token, meta_ads/meta_dataset operations, main river lifecycle. Migration 0084.';
COMMENT ON ROLE commerce_claims_worker IS 'worker authority of cmd/claims-worker dispatcher (lc_claims_worker): claims reply and ECPay CVS definers, default-lane external_operation_v1 operations, main river lifecycle. Migration 0084.';
COMMENT ON ROLE commerce_worker IS 'EMPTY legacy role since migration 0084 (T21-02): holds no privilege and no login may join it; kept because earlier checksummed migrations and tests name it. Use the five commerce_*_worker/commerce_payment_live authorities.';

-- ------------------------------------------------------------------------------------------------
-- Lane helpers (integration owner; SECURITY DEFINER with the fixed pg_catalog path like every integration function, the T06 ACL gate
-- requires it; they read only pg_roles membership of session_user). operation_lane is evaluated by the worker logins inside RLS policies, so only it
-- is executable by them; the other two run inside definers (owner) and have no caller EXECUTE.
-- ------------------------------------------------------------------------------------------------
CREATE FUNCTION integration.operation_lane(p_provider text,p_actor text) RETURNS text
LANGUAGE sql IMMUTABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT CASE
  WHEN p_actor='MEDIA_ATTEMPT' OR p_provider='livekit' THEN 'media'
  WHEN p_provider IN ('stripe','payuni') OR p_actor IN ('BUYER_PAYMENT_QUERY','PAYMENT_REFUND') THEN 'payment'
  WHEN p_provider IN ('meta_ads','meta_dataset') THEN 'ads'
  ELSE 'default' END $$;
ALTER FUNCTION integration.operation_lane(text,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.operation_lane(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.operation_lane(text,text) TO commerce_payment_worker,commerce_payment_live,commerce_ads_worker,commerce_claims_worker;
COMMENT ON FUNCTION integration.operation_lane(text,text) IS 'integration owner; pure classifier payment|ads|media|default of an operation or binding by provider/actor_kind. Used by the worker RLS policies below and operation_authority_ok; EXECUTE for the four integration-reading worker authorities only (policy evaluation). Adding a provider means deciding its lane here.';

CREATE FUNCTION integration.operation_authority_ok(p_provider text,p_actor text) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT CASE integration.operation_lane(p_provider,p_actor)
  WHEN 'payment' THEN pg_has_role(session_user,'commerce_payment_worker','MEMBER') OR pg_has_role(session_user,'commerce_payment_live','MEMBER')
  WHEN 'ads' THEN pg_has_role(session_user,'commerce_ads_worker','MEMBER')
  WHEN 'default' THEN pg_has_role(session_user,'commerce_claims_worker','MEMBER')
  ELSE false END $$;
ALTER FUNCTION integration.operation_authority_ok(text,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.operation_authority_ok(text,text) FROM PUBLIC;
COMMENT ON FUNCTION integration.operation_authority_ok(text,text) IS 'integration owner; private gate of claim_operation/complete_operation: true iff the connected login (session_user, never an argument) is a member of the authority that owns the operation lane. The media lane is false for everyone. No EXECUTE for any login.';

CREATE FUNCTION integration.profile_authority_ok(p_profile text) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT coalesce((pg_has_role(session_user,'commerce_payment_live','MEMBER') AND p_profile='LIVE')
  OR (pg_has_role(session_user,'commerce_payment_worker','MEMBER') AND p_profile IN ('PROVIDER_MOCK','SANDBOX')),false) $$;
ALTER FUNCTION integration.profile_authority_ok(text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.profile_authority_ok(text) FROM PUBLIC;
COMMENT ON FUNCTION integration.profile_authority_ok(text) IS 'integration owner; private execution-profile fence (T21-03): LIVE only for a commerce_payment_live login, SANDBOX/PROVIDER_MOCK only for a commerce_payment_worker login, derived from session_user, never from the caller-supplied profile. Used by claim_operation, complete_operation and the three require_* guards. No EXECUTE for any login.';

-- ------------------------------------------------------------------------------------------------
-- Schema USAGE and integration table reads: moved off commerce_worker
-- ------------------------------------------------------------------------------------------------
REVOKE USAGE ON SCHEMA integration,checkout,payments,claims,ads FROM commerce_worker;
GRANT USAGE ON SCHEMA integration TO commerce_payment_worker,commerce_payment_live,commerce_ads_worker,commerce_claims_worker;
GRANT USAGE ON SCHEMA payments TO commerce_payment_worker,commerce_payment_live;
GRANT USAGE ON SCHEMA checkout TO commerce_expiry_worker;
GRANT USAGE ON SCHEMA claims TO commerce_claims_worker;
GRANT USAGE ON SCHEMA ads TO commerce_ads_worker;

REVOKE SELECT ON integration.bindings,integration.operations,integration.operation_events FROM commerce_worker;
GRANT SELECT ON integration.bindings,integration.operations,integration.operation_events TO commerce_payment_worker,commerce_payment_live,commerce_ads_worker,commerce_claims_worker;

-- 0008 policy worker_binding_read served commerce_worker and the integration writer; keep it for the writer only.
ALTER POLICY worker_binding_read ON integration.bindings TO commerce_integration_writer;
DROP POLICY worker_operation_read ON integration.operations;
DROP POLICY worker_event_read ON integration.operation_events;
CREATE POLICY payment_binding_read ON integration.bindings FOR SELECT TO commerce_payment_worker,commerce_payment_live USING (integration.operation_lane(provider,'')='payment');
CREATE POLICY ads_binding_read ON integration.bindings FOR SELECT TO commerce_ads_worker USING (integration.operation_lane(provider,'')='ads');
CREATE POLICY claims_binding_read ON integration.bindings FOR SELECT TO commerce_claims_worker USING (integration.operation_lane(provider,'')='default');
CREATE POLICY payment_operation_read ON integration.operations FOR SELECT TO commerce_payment_worker,commerce_payment_live USING (integration.operation_lane(provider,actor_kind)='payment');
CREATE POLICY ads_operation_read ON integration.operations FOR SELECT TO commerce_ads_worker USING (integration.operation_lane(provider,actor_kind)='ads');
CREATE POLICY claims_operation_read ON integration.operations FOR SELECT TO commerce_claims_worker USING (integration.operation_lane(provider,actor_kind)='default');
CREATE POLICY payment_event_read ON integration.operation_events FOR SELECT TO commerce_payment_worker,commerce_payment_live USING
 (EXISTS (SELECT 1 FROM integration.operations o WHERE o.id=operation_id AND o.tenant_id=operation_events.tenant_id AND o.store_id=operation_events.store_id
  AND integration.operation_lane(o.provider,o.actor_kind)='payment'));
CREATE POLICY ads_event_read ON integration.operation_events FOR SELECT TO commerce_ads_worker USING
 (EXISTS (SELECT 1 FROM integration.operations o WHERE o.id=operation_id AND o.tenant_id=operation_events.tenant_id AND o.store_id=operation_events.store_id
  AND integration.operation_lane(o.provider,o.actor_kind)='ads'));
CREATE POLICY claims_event_read ON integration.operation_events FOR SELECT TO commerce_claims_worker USING
 (EXISTS (SELECT 1 FROM integration.operations o WHERE o.id=operation_id AND o.tenant_id=operation_events.tenant_id AND o.store_id=operation_events.store_id
  AND integration.operation_lane(o.provider,o.actor_kind)='default'));

-- ------------------------------------------------------------------------------------------------
-- Lane-gated claim/complete (static forward copies of 0035; only the T21 gate after the MEDIA check is new) and the
-- profile fence of the three private guards (static forward copies of 0017/0061/0062; only the T21-03 check is new).
-- CREATE OR REPLACE keeps owner, SECURITY DEFINER, search_path and the (revoked-from-PUBLIC) ACL.
-- ------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.claim_operation(p_id uuid,p_lease_seconds integer,p_token bytea)
RETURNS TABLE(disposition text,generation bigint,mode text)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_binding_id uuid; b integration.bindings%ROWTYPE; o integration.operations%ROWTYPE;
 v_now timestamptz; v_reason text; v_changed boolean;
BEGIN
 IF p_lease_seconds IS NULL OR p_lease_seconds NOT BETWEEN 5 AND 300 OR p_token IS NULL OR octet_length(p_token)<>32 THEN
  RAISE EXCEPTION 'invalid claim' USING ERRCODE='22023'; END IF;
 SELECT x.binding_id INTO v_binding_id FROM integration.operations x WHERE x.id=p_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.id=v_binding_id FOR SHARE;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=p_id FOR UPDATE;
 IF NOT FOUND OR o.binding_id<>b.id OR o.tenant_id<>b.tenant_id OR o.store_id<>b.store_id THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 IF o.actor_kind='MEDIA_ATTEMPT' THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 -- T21-02/03: the caller's authority (session_user membership, never an argument) must own this operation's lane
 -- and, for a payment operation tied to an attempt, that attempt's execution profile. Refused before any mutation.
 IF NOT integration.operation_authority_ok(o.provider,o.actor_kind)
  OR (o.payment_attempt_id IS NOT NULL AND NOT integration.profile_authority_ok(
   (SELECT a.execution_profile FROM checkout.payment_attempts a WHERE a.id=o.payment_attempt_id))) THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 v_now:=clock_timestamp();
 IF o.lease_until>v_now THEN RETURN QUERY SELECT 'busy'::text,o.generation,''::text; RETURN; END IF;
 IF o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING') THEN
  RETURN QUERY SELECT 'terminal'::text,o.generation,''::text; RETURN; END IF;
 v_changed:=b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id
  OR (o.actor_kind='MERCHANT' AND (NOT b.enabled OR b.semantic_version<>o.binding_version));
 IF v_changed THEN
  IF o.actor_kind='BUYER_PAYMENT_QUERY' THEN RETURN QUERY SELECT 'blocked_binding'::text,o.generation,''::text; RETURN; END IF;
  IF o.state='UNKNOWN' AND o.lease_until IS NULL AND o.result_code='binding_changed' THEN
   RETURN QUERY SELECT 'blocked_binding'::text,o.generation,''::text; RETURN; END IF;
  UPDATE integration.operations x SET state=CASE WHEN o.state='READY' THEN 'STALE_BINDING' ELSE 'UNKNOWN' END,
   generation=o.generation+1,lease_mode='',lease_until=NULL,lease_token_hash=NULL,result_code='binding_changed',updated_at=v_now
   WHERE x.id=p_id RETURNING x.* INTO o;
  v_reason:='binding_changed'; disposition:=CASE WHEN o.state='STALE_BINDING' THEN 'terminal' ELSE 'blocked_binding' END;
 ELSE
  mode:=CASE WHEN o.state='READY' THEN 'dispatch' ELSE 'reconcile' END;
  UPDATE integration.operations x SET state=CASE WHEN mode='dispatch' THEN 'DISPATCHING' ELSE 'UNKNOWN' END,
   generation=o.generation+1,lease_mode=mode,lease_until=v_now+make_interval(secs=>p_lease_seconds),lease_token_hash=sha256(p_token),updated_at=v_now
   WHERE x.id=p_id RETURNING x.* INTO o;
  disposition:='claimed'; v_reason:=CASE WHEN mode='dispatch' THEN 'dispatch_claimed' ELSE 'reconcile_claimed' END;
 END IF;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation,o.state,o.lease_mode,v_reason);
 generation:=o.generation; mode:=o.lease_mode; RETURN NEXT;
END $$;
COMMENT ON FUNCTION integration.claim_operation(uuid,integer,bytea) IS 'integration owner; EXECUTE for the four lane workers (payment x2, ads, claims). Refuses with operation-not-found, before any mutation, an operation outside the caller authority lane or (payment) outside its execution profile (T21-02/03); MEDIA_ATTEMPT never claimable here.';

CREATE OR REPLACE FUNCTION integration.complete_operation(p_id uuid,p_generation bigint,p_token bytea,p_state text,p_code text,p_reference text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_binding_id uuid; b integration.bindings%ROWTYPE; o integration.operations%ROWTYPE;
 v_now timestamptz; v_reason text;
BEGIN
 IF p_token IS NULL OR octet_length(p_token)<>32 OR p_generation IS NULL OR p_generation<1
  OR p_state IS NULL OR p_state NOT IN ('SUCCEEDED','FAILED_FINAL','UNKNOWN','ACKNOWLEDGED','BLOCKED_POLICY')
  OR p_code IS NULL OR p_code !~ '^[A-Za-z0-9_.:-]{1,80}$'
  OR p_reference IS NULL OR length(p_reference)>200 OR p_reference ~ '[[:cntrl:]]' THEN
  RAISE EXCEPTION 'invalid completion' USING ERRCODE='22023';
 END IF;
 SELECT x.binding_id INTO v_binding_id FROM integration.operations x WHERE x.id=p_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.id=v_binding_id FOR SHARE;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=p_id FOR UPDATE;
 v_now:=clock_timestamp();
 IF NOT FOUND OR o.binding_id<>b.id OR o.tenant_id<>b.tenant_id OR o.store_id<>b.store_id THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002';
 END IF;
 IF o.actor_kind='MEDIA_ATTEMPT' THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 -- T21-02/03: the caller's authority (session_user membership, never an argument) must own this operation's lane
 -- and, for a payment operation tied to an attempt, that attempt's execution profile. Refused before any mutation.
 IF NOT integration.operation_authority_ok(o.provider,o.actor_kind)
  OR (o.payment_attempt_id IS NOT NULL AND NOT integration.profile_authority_ok(
   (SELECT a.execution_profile FROM checkout.payment_attempts a WHERE a.id=o.payment_attempt_id))) THEN
  RAISE EXCEPTION 'operation not found' USING ERRCODE='P0002'; END IF;
 IF o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=v_now
  OR o.lease_token_hash<>sha256(p_token) OR o.state NOT IN ('DISPATCHING','UNKNOWN') THEN
  RAISE EXCEPTION 'lease conflict' USING ERRCODE='40001';
 END IF;
 IF p_state='BLOCKED_POLICY' AND (o.lease_mode<>'dispatch' OR p_reference<>'') THEN
  RAISE EXCEPTION 'policy outcome requires undispatched claim' USING ERRCODE='22023';
 END IF;
 v_reason:=CASE WHEN NOT b.enabled OR b.semantic_version<>o.binding_version
  OR b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id
  THEN 'completed_binding_changed' ELSE p_code END;
 UPDATE integration.operations SET state=p_state,result_code=p_code,provider_reference=p_reference,
  lease_mode='',lease_until=NULL,lease_token_hash=NULL,updated_at=v_now WHERE id=p_id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation,p_state,'',v_reason);
END $$;
COMMENT ON FUNCTION integration.complete_operation(uuid,bigint,bytea,text,text,text) IS 'integration owner; EXECUTE for the four lane workers. Same lane/profile gate as claim_operation, then the lease-token fence; MEDIA_ATTEMPT is not a generic completion.';

CREATE OR REPLACE FUNCTION integration.require_payment_query(p_id uuid,p_generation bigint,p_token bytea,p_profile text)
RETURNS checkout.payment_attempts LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; b integration.bindings%ROWTYPE;
 a checkout.payment_attempts%ROWTYPE; v_binding uuid;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<2 OR p_token IS NULL OR octet_length(p_token)<>32
 OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'invalid payment query' USING ERRCODE='22023'; END IF;
 -- T21-03: the profile is the caller's authority, not its claim: LIVE only for commerce_payment_live,
 -- SANDBOX/PROVIDER_MOCK only for commerce_payment_worker (a valid but foreign profile is refused here).
 IF NOT integration.profile_authority_ok(p_profile) THEN
  RAISE EXCEPTION 'payment query profile outside the caller authority' USING ERRCODE='42501'; END IF;
 SELECT x.binding_id INTO v_binding FROM integration.operations x
 WHERE x.id=p_id AND x.actor_kind='BUYER_PAYMENT_QUERY';
 IF NOT FOUND THEN RAISE EXCEPTION 'payment query unavailable' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.id=v_binding FOR SHARE;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=p_id FOR UPDATE;
 SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=p_id
 AND x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.owner_id=o.buyer_owner_id
 AND x.session_id=o.buyer_session_id AND x.binding_id=o.binding_id;
 IF NOT FOUND OR o.actor_kind<>'BUYER_PAYMENT_QUERY' OR o.action<>'payuni.query' OR o.provider<>'payuni'
 OR o.purpose<>'transactional' OR o.payment_attempt_id<>a.id OR o.binding_version<>a.binding_version
 OR b.id IS NULL OR b.tenant_id<>a.tenant_id OR b.store_id<>a.store_id OR b.provider<>o.provider
 OR b.external_asset_id<>o.external_asset_id OR a.execution_profile<>p_profile
 OR (p_profile='PROVIDER_MOCK' AND a.environment<>'SANDBOX')
 OR (p_profile<>'PROVIDER_MOCK' AND a.environment<>p_profile) THEN
  RAISE EXCEPTION 'payment query unavailable' USING ERRCODE='PT409'; END IF;
 IF o.state<>'UNKNOWN' OR o.lease_mode<>'reconcile' OR o.generation<>p_generation
 OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
 OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'payment query lease conflict' USING ERRCODE='40001'; END IF;
 RETURN a;
END $$;
CREATE OR REPLACE FUNCTION integration.require_stripe_query(p_id uuid,p_generation bigint,p_token bytea,p_profile text)
RETURNS checkout.payment_attempts LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; b integration.bindings%ROWTYPE;
 a checkout.payment_attempts%ROWTYPE; v_binding uuid;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<2 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'invalid Stripe query' USING ERRCODE='22023'; END IF;
 -- T21-03: the profile is the caller's authority, not its claim: LIVE only for commerce_payment_live,
 -- SANDBOX/PROVIDER_MOCK only for commerce_payment_worker (a valid but foreign profile is refused here).
 IF NOT integration.profile_authority_ok(p_profile) THEN
  RAISE EXCEPTION 'Stripe query profile outside the caller authority' USING ERRCODE='42501'; END IF;
 SELECT x.binding_id INTO v_binding FROM integration.operations x
  WHERE x.id=p_id AND x.actor_kind='BUYER_PAYMENT_QUERY' AND x.provider='stripe';
 IF NOT FOUND THEN RAISE EXCEPTION 'Stripe query unavailable' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.id=v_binding FOR SHARE;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=p_id FOR UPDATE;
 SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=p_id
  AND x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.owner_id=o.buyer_owner_id
  AND x.session_id=o.buyer_session_id AND x.binding_id=o.binding_id;
 IF NOT FOUND OR o.actor_kind<>'BUYER_PAYMENT_QUERY' OR o.action<>'stripe.checkout_session'
  OR o.provider<>'stripe' OR o.purpose<>'transactional' OR o.payment_attempt_id<>a.id
  OR o.binding_version<>a.binding_version OR a.method_code<>'stripe_checkout'
  OR b.id IS NULL OR b.tenant_id<>a.tenant_id OR b.store_id<>a.store_id OR b.provider<>o.provider
  OR b.external_asset_id<>o.external_asset_id OR a.execution_profile<>p_profile
  OR (p_profile='PROVIDER_MOCK' AND a.environment<>'SANDBOX')
  OR (p_profile<>'PROVIDER_MOCK' AND a.environment<>p_profile) THEN
  RAISE EXCEPTION 'Stripe query unavailable' USING ERRCODE='PT409'; END IF;
 IF o.state<>'UNKNOWN' OR o.lease_mode<>'reconcile' OR o.generation<>p_generation
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'Stripe query lease conflict' USING ERRCODE='40001'; END IF;
 RETURN a;
END $$;
CREATE OR REPLACE FUNCTION integration.require_stripe_refund(p_id uuid,p_generation bigint,p_token bytea,p_profile text)
RETURNS payments.stripe_refunds LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; b integration.bindings%ROWTYPE;
 a checkout.payment_attempts%ROWTYPE; r payments.stripe_refunds%ROWTYPE; v_binding uuid;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<2 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'invalid Stripe refund' USING ERRCODE='22023'; END IF;
 -- T21-03: the profile is the caller's authority, not its claim: LIVE only for commerce_payment_live,
 -- SANDBOX/PROVIDER_MOCK only for commerce_payment_worker (a valid but foreign profile is refused here).
 IF NOT integration.profile_authority_ok(p_profile) THEN
  RAISE EXCEPTION 'Stripe refund profile outside the caller authority' USING ERRCODE='42501'; END IF;
 SELECT x.binding_id INTO v_binding FROM integration.operations x
  WHERE x.id=p_id AND x.actor_kind='PAYMENT_REFUND' AND x.provider='stripe';
 IF NOT FOUND THEN RAISE EXCEPTION 'Stripe refund unavailable' USING ERRCODE='P0002'; END IF;
 -- Lock order = Claim/Complete: binding FOR SHARE, then the operation FOR UPDATE.
 SELECT x.* INTO b FROM integration.bindings x WHERE x.id=v_binding FOR SHARE;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=p_id FOR UPDATE;
 SELECT x.* INTO r FROM payments.stripe_refunds x WHERE x.id=p_id AND x.tenant_id=o.tenant_id
  AND x.store_id=o.store_id;
 SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=r.attempt_id AND x.tenant_id=r.tenant_id
  AND x.store_id=r.store_id AND x.owner_id=r.owner_id AND x.order_id=r.order_id;
 IF r.id IS NULL OR a.id IS NULL OR o.actor_kind<>'PAYMENT_REFUND' OR o.action<>'stripe.refund'
  OR o.provider<>'stripe' OR o.purpose<>'transactional' OR o.payment_attempt_id<>a.id
  OR o.buyer_owner_id<>a.owner_id OR o.buyer_session_id<>a.session_id
  OR o.binding_id<>a.binding_id OR o.binding_version<>a.binding_version
  OR a.method_code<>'stripe_checkout' OR b.id IS NULL OR b.tenant_id<>a.tenant_id
  OR b.store_id<>a.store_id OR b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id
  OR a.execution_profile<>p_profile OR r.environment<>a.environment
  OR (p_profile='PROVIDER_MOCK' AND a.environment<>'SANDBOX')
  OR (p_profile<>'PROVIDER_MOCK' AND a.environment<>p_profile) THEN
  RAISE EXCEPTION 'Stripe refund unavailable' USING ERRCODE='PT409'; END IF;
 IF o.state<>'UNKNOWN' OR o.lease_mode<>'reconcile' OR o.generation<>p_generation
  OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'Stripe refund lease conflict' USING ERRCODE='40001'; END IF;
 RETURN r;
END $$;
-- ------------------------------------------------------------------------------------------------
-- Function EXECUTE: REVOKE from commerce_worker, GRANT to the authorities whose process runs it.
-- Functions created by post_river/*.sql (payment_queue_ready, record_stripe_*, load_stripe_signal, expiry_queue_ready,
-- guard_ads_job) are moved by post_river/0018.
-- ------------------------------------------------------------------------------------------------
REVOKE EXECUTE ON FUNCTION
 integration.claim_operation(uuid,integer,bytea),integration.complete_operation(uuid,bigint,bytea,text,text,text),
 checkout.expire_held(uuid,bigint),
 integration.load_payment_query(uuid,bigint,bytea,text),integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint),
 integration.finish_payment_query(uuid,bigint,bytea,text,text,text),payments.apply_capture(uuid,bytea),
 integration.load_stripe_credential(uuid,bigint,bytea,text),integration.load_stripe_session(uuid,bigint,bytea,text),
 integration.mark_stripe_create_sent(uuid,bigint,bytea,text,bytea),integration.note_stripe_expire(uuid,bigint,bytea,text),
 integration.finish_stripe_query(uuid,bigint,bytea,text,text),integration.consume_stripe_signal(uuid,uuid,bigint,bytea,text,text),
 integration.load_stripe_refund(uuid,bigint,bytea,text),integration.mark_stripe_refund_sent(uuid,bigint,bytea,text,bytea),
 integration.finish_stripe_refund(uuid,bigint,bytea,text,text),
 claims.check_meta_reply(uuid,bytea),integration.load_meta_page_token(uuid,bigint,bytea),
 integration.load_cvs_create(uuid,bigint,bytea,text),
 integration.finish_cvs_create(uuid,bigint,bytea,text,text,text,text,text,text,text),
 fulfillment.ecpay_recipient_ok(text,text),
 ads.canonical_draft(uuid),ads.purge_oauth_states(),ads.check_create(uuid),ads.check_activate(uuid),ads.check_read(uuid),
 ads.advance_candidates(integer,integer),ads.advance_next(uuid),ads.advance_plan(uuid,text,uuid,bigint),
 ads.insights_candidates(integer),ads.insights_days(uuid),ads.insights_plan(uuid,date,uuid,bigint),
 ads.pending_insight_reads(integer),ads.put_insights_day(uuid),
 ads.plan_capi_eligible(uuid),ads.plan_capi_candidates(integer),ads.plan_capi(uuid,uuid,bigint),ads.plan_capi_purge(),
 ads.check_capi(uuid),ads.capi_user_data(uuid,bigint,bytea),integration.load_meta_ads_token(uuid,bigint,bytea)
 FROM commerce_worker;

-- Lane dispatcher ledger: every worker that dispatches external operations.
GRANT EXECUTE ON FUNCTION integration.claim_operation(uuid,integer,bytea),integration.complete_operation(uuid,bigint,bytea,text,text,text)
 TO commerce_payment_worker,commerce_payment_live,commerce_ads_worker,commerce_claims_worker;
-- Expiry worker: the checkout expiry definer only.
GRANT EXECUTE ON FUNCTION checkout.expire_held(uuid,bigint) TO commerce_expiry_worker;
-- Payment workers (SANDBOX and LIVE): PAYUNi query, Stripe session/refund/signal definers, same-tx apply_capture.
GRANT EXECUTE ON FUNCTION
 integration.load_payment_query(uuid,bigint,bytea,text),integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint),
 integration.finish_payment_query(uuid,bigint,bytea,text,text,text),payments.apply_capture(uuid,bytea),
 integration.load_stripe_credential(uuid,bigint,bytea,text),integration.load_stripe_session(uuid,bigint,bytea,text),
 integration.mark_stripe_create_sent(uuid,bigint,bytea,text,bytea),integration.note_stripe_expire(uuid,bigint,bytea,text),
 integration.finish_stripe_query(uuid,bigint,bytea,text,text),integration.consume_stripe_signal(uuid,uuid,bigint,bytea,text,text),
 integration.load_stripe_refund(uuid,bigint,bytea,text),integration.mark_stripe_refund_sent(uuid,bigint,bytea,text,bytea),
 integration.finish_stripe_refund(uuid,bigint,bytea,text,text)
 TO commerce_payment_worker,commerce_payment_live;
-- Claims worker: Meta private reply check + Page-token loader, ECPay CVS create loader/finish.
GRANT EXECUTE ON FUNCTION
 claims.check_meta_reply(uuid,bytea),integration.load_meta_page_token(uuid,bigint,bytea),
 integration.load_cvs_create(uuid,bigint,bytea,text),
 integration.finish_cvs_create(uuid,bigint,bytea,text,text,text,text,text,text,text)
 TO commerce_claims_worker;
-- Ads worker: ads Check/sweeper/ingestion definers, CAPI planner and user-data loader, ads token loader.
GRANT EXECUTE ON FUNCTION
 ads.canonical_draft(uuid),ads.purge_oauth_states(),ads.check_create(uuid),ads.check_activate(uuid),ads.check_read(uuid),
 ads.advance_candidates(integer,integer),ads.advance_next(uuid),ads.advance_plan(uuid,text,uuid,bigint),
 ads.insights_candidates(integer),ads.insights_days(uuid),ads.insights_plan(uuid,date,uuid,bigint),
 ads.pending_insight_reads(integer),ads.put_insights_day(uuid),
 ads.plan_capi_eligible(uuid),ads.plan_capi_candidates(integer),ads.plan_capi(uuid,uuid,bigint),ads.plan_capi_purge(),
 ads.check_capi(uuid),ads.capi_user_data(uuid,bigint,bytea),integration.load_meta_ads_token(uuid,bigint,bytea)
 TO commerce_ads_worker;

-- COMMENT ON (PROCESS §5): a function comment that still names the shared commerce_worker would now be false. Rewrite just that
-- word, per function, to the authority(ies) that run it; comments that do not name it are untouched.
DO $$
DECLARE r record; v_comment text;
BEGIN
 FOR r IN SELECT * FROM (VALUES
  ('checkout.expire_held(uuid,bigint)','commerce_expiry_worker'),
  ('integration.load_payment_query(uuid,bigint,bytea,text)','commerce_payment_worker/commerce_payment_live'),
  ('integration.record_payment_query(uuid,bigint,bytea,text,jsonb,bigint)','commerce_payment_worker/commerce_payment_live'),
  ('integration.finish_payment_query(uuid,bigint,bytea,text,text,text)','commerce_payment_worker/commerce_payment_live'),
  ('payments.apply_capture(uuid,bytea)','commerce_payment_worker/commerce_payment_live'),
  ('integration.load_stripe_credential(uuid,bigint,bytea,text)','commerce_payment_worker/commerce_payment_live'),
  ('integration.load_stripe_session(uuid,bigint,bytea,text)','commerce_payment_worker/commerce_payment_live'),
  ('integration.mark_stripe_create_sent(uuid,bigint,bytea,text,bytea)','commerce_payment_worker/commerce_payment_live'),
  ('integration.note_stripe_expire(uuid,bigint,bytea,text)','commerce_payment_worker/commerce_payment_live'),
  ('integration.finish_stripe_query(uuid,bigint,bytea,text,text)','commerce_payment_worker/commerce_payment_live'),
  ('integration.consume_stripe_signal(uuid,uuid,bigint,bytea,text,text)','commerce_payment_worker/commerce_payment_live'),
  ('integration.load_stripe_refund(uuid,bigint,bytea,text)','commerce_payment_worker/commerce_payment_live'),
  ('integration.mark_stripe_refund_sent(uuid,bigint,bytea,text,bytea)','commerce_payment_worker/commerce_payment_live'),
  ('integration.finish_stripe_refund(uuid,bigint,bytea,text,text)','commerce_payment_worker/commerce_payment_live'),
  ('claims.check_meta_reply(uuid,bytea)','commerce_claims_worker'),
  ('integration.load_meta_page_token(uuid,bigint,bytea)','commerce_claims_worker'),
  ('integration.load_cvs_create(uuid,bigint,bytea,text)','commerce_claims_worker'),
  ('integration.finish_cvs_create(uuid,bigint,bytea,text,text,text,text,text,text,text)','commerce_claims_worker'),
  ('ads.canonical_draft(uuid)','commerce_ads_worker'),('ads.purge_oauth_states()','commerce_ads_worker'),
  ('ads.check_create(uuid)','commerce_ads_worker'),('ads.check_activate(uuid)','commerce_ads_worker'),('ads.check_read(uuid)','commerce_ads_worker'),
  ('ads.advance_candidates(integer,integer)','commerce_ads_worker'),('ads.advance_next(uuid)','commerce_ads_worker'),
  ('ads.advance_plan(uuid,text,uuid,bigint)','commerce_ads_worker'),('ads.insights_candidates(integer)','commerce_ads_worker'),
  ('ads.insights_days(uuid)','commerce_ads_worker'),('ads.insights_plan(uuid,date,uuid,bigint)','commerce_ads_worker'),
  ('ads.pending_insight_reads(integer)','commerce_ads_worker'),('ads.put_insights_day(uuid)','commerce_ads_worker'),
  ('ads.plan_capi_eligible(uuid)','commerce_ads_worker'),('ads.plan_capi_candidates(integer)','commerce_ads_worker'),
  ('ads.plan_capi(uuid,uuid,bigint)','commerce_ads_worker'),('ads.plan_capi_purge()','commerce_ads_worker'),
  ('ads.check_capi(uuid)','commerce_ads_worker'),('ads.capi_user_data(uuid,bigint,bytea)','commerce_ads_worker'),
  ('integration.load_meta_ads_token(uuid,bigint,bytea)','commerce_ads_worker')
 ) AS t(sig,role_text) LOOP
  v_comment:=obj_description(to_regprocedure(r.sig),'pg_proc');
  IF v_comment LIKE '%commerce\_worker%' THEN
   EXECUTE format('COMMENT ON FUNCTION %s IS %L',r.sig,replace(v_comment,'commerce_worker',r.role_text));
  END IF;
 END LOOP;
END $$;
