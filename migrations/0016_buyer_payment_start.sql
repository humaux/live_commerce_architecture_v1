-- Payment admission is separate from merchant intent. No application role can
-- issue qualification; the real provider verifier remains unimplemented. Only
-- disposable test owners seed PROVIDER_MOCK, which never qualifies a real run.
CREATE TABLE payments.account_qualifications (
 id uuid PRIMARY KEY,tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 connection_id uuid NOT NULL,credential_version bigint NOT NULL,
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 code text NOT NULL CHECK(code='payuni_credit'),
 proof_class text NOT NULL CHECK(proof_class IN ('PROVIDER_MOCK','REAL_SANDBOX','REAL_LIVE')),
 evidence_ref text NOT NULL CHECK(length(evidence_ref) BETWEEN 1 AND 200 AND evidence_ref !~ '[[:cntrl:]]'),
 observed_at timestamptz NOT NULL,expires_at timestamptz NOT NULL,revoked_at timestamptz,
 CHECK(expires_at>observed_at),
 CHECK(proof_class='PROVIDER_MOCK' OR proof_class='REAL_'||environment),
 UNIQUE(tenant_id,store_id,id,connection_id,environment,code),
 FOREIGN KEY(tenant_id,store_id,connection_id,credential_version)
  REFERENCES integration.account_credentials(tenant_id,store_id,connection_id,version)
);
ALTER TABLE payments.account_qualifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments.account_qualifications FORCE ROW LEVEL SECURITY;

ALTER TABLE payments.method_versions ADD COLUMN qualification_id uuid;
ALTER TABLE payments.method_versions DROP CONSTRAINT method_not_admitted;
ALTER TABLE payments.method_versions ADD CONSTRAINT method_admission_reference
 CHECK(NOT enabled OR (qualification_id IS NOT NULL AND connection_id IS NOT NULL));
ALTER TABLE payments.method_versions ADD CONSTRAINT method_qualification_fk
 FOREIGN KEY(tenant_id,store_id,qualification_id,connection_id,environment,code)
 REFERENCES payments.account_qualifications(tenant_id,store_id,id,connection_id,environment,code);

CREATE TABLE checkout.payment_attempts (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,owner_id uuid NOT NULL,id uuid NOT NULL,
 session_id uuid NOT NULL,order_id uuid NOT NULL,market_id uuid NOT NULL,country text NOT NULL,
 method_code text NOT NULL CHECK(method_code='payuni_credit'),method_version bigint NOT NULL,
 connection_id uuid NOT NULL,credential_version bigint NOT NULL,qualification_id uuid NOT NULL,
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 execution_profile text NOT NULL CHECK(execution_profile IN ('PROVIDER_MOCK','SANDBOX','LIVE')),
 binding_id uuid NOT NULL,binding_version bigint NOT NULL,
 currency text NOT NULL CHECK(currency='TWD'),amount_minor bigint NOT NULL
  CHECK(amount_minor BETWEEN 100 AND 19999900 AND amount_minor%100=0),
 merchant_trade_no text NOT NULL UNIQUE CHECK(merchant_trade_no ~ '^[A-Za-z0-9_-]{1,25}$'),
 state text NOT NULL CHECK(state='PAYMENT_PENDING'),generation bigint NOT NULL CHECK(generation>1),
 job_id bigint NOT NULL UNIQUE CHECK(job_id>0),created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,owner_id,id),UNIQUE(id),UNIQUE(tenant_id,store_id,order_id),
 UNIQUE(tenant_id,store_id,id,owner_id,session_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,session_id) REFERENCES buyer.capability_sessions(tenant_id,store_id,owner_id,id),
 FOREIGN KEY(tenant_id,store_id,market_id,country,method_code,method_version)
  REFERENCES payments.method_versions(tenant_id,store_id,market_id,country,code,version),
 FOREIGN KEY(tenant_id,store_id,connection_id,credential_version)
  REFERENCES integration.account_credentials(tenant_id,store_id,connection_id,version),
 FOREIGN KEY(tenant_id,store_id,qualification_id,connection_id,environment,method_code)
  REFERENCES payments.account_qualifications(tenant_id,store_id,id,connection_id,environment,code),
 FOREIGN KEY(tenant_id,store_id,binding_id) REFERENCES integration.bindings(tenant_id,store_id,id)
);
ALTER TABLE checkout.payment_attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.payment_attempts FORCE ROW LEVEL SECURITY;
CREATE POLICY payment_attempt_writer ON checkout.payment_attempts TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
 AND session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
GRANT SELECT,INSERT ON checkout.payment_attempts TO commerce_checkout_writer;

-- Both ledgers reference the same attempt identity. No fabricated membership,
-- no READY state, and no dispatch lease are possible in the buyer family.
ALTER TABLE integration.operations ALTER COLUMN principal_id DROP NOT NULL;
ALTER TABLE integration.operations ADD COLUMN actor_kind text NOT NULL DEFAULT 'MERCHANT',
 ADD COLUMN payment_attempt_id uuid,ADD COLUMN buyer_owner_id uuid,ADD COLUMN buyer_session_id uuid;
ALTER TABLE integration.operations ADD CONSTRAINT operation_actor_family CHECK(
 (actor_kind='MERCHANT' AND principal_id IS NOT NULL AND payment_attempt_id IS NULL
  AND buyer_owner_id IS NULL AND buyer_session_id IS NULL)
 OR (actor_kind='BUYER_PAYMENT_QUERY' AND principal_id IS NULL AND payment_attempt_id=id
  AND payment_attempt_id IS NOT NULL AND buyer_owner_id IS NOT NULL AND buyer_session_id IS NOT NULL
  AND provider='payuni' AND action='payuni.query' AND purpose='transactional'
  AND state NOT IN ('READY','DISPATCHING','BLOCKED_POLICY','STALE_BINDING') AND lease_mode<>'dispatch'));
ALTER TABLE integration.operations ADD CONSTRAINT operation_payment_attempt_fk
 FOREIGN KEY(tenant_id,store_id,payment_attempt_id,buyer_owner_id,buyer_session_id)
 REFERENCES checkout.payment_attempts(tenant_id,store_id,id,owner_id,session_id);
ALTER TABLE checkout.payment_attempts ADD CONSTRAINT attempt_query_operation_fk
 FOREIGN KEY(tenant_id,store_id,id) REFERENCES integration.operations(tenant_id,store_id,id)
 DEFERRABLE INITIALLY DEFERRED;
GRANT USAGE ON SCHEMA integration,payments TO commerce_checkout_writer;
GRANT SELECT,INSERT ON integration.operations,integration.operation_events TO commerce_checkout_writer;
GRANT USAGE ON SEQUENCE integration.operation_events_id_seq TO commerce_checkout_writer;
CREATE POLICY checkout_payment_operation ON integration.operations TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND actor_kind='BUYER_PAYMENT_QUERY' AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND actor_kind='BUYER_PAYMENT_QUERY' AND buyer_owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
 AND buyer_session_id=nullif(current_setting('app.buyer_session_id',true),'')::uuid);
CREATE POLICY checkout_payment_event ON integration.operation_events FOR INSERT TO commerce_checkout_writer
 WITH CHECK(EXISTS(SELECT 1 FROM integration.operations o WHERE o.id=operation_id
 AND o.tenant_id=operation_events.tenant_id AND o.store_id=operation_events.store_id
 AND o.actor_kind='BUYER_PAYMENT_QUERY'));

DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['payments.method_heads','payments.method_versions',
  'payments.account_qualifications','integration.merchant_accounts','integration.bindings'] LOOP
  EXECUTE format('GRANT SELECT ON %s TO commerce_checkout_writer',r);
  EXECUTE format('CREATE POLICY payment_writer_read ON %s FOR SELECT TO commerce_checkout_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',r);
  EXECUTE format('CREATE POLICY payment_writer_lock ON %s FOR UPDATE TO commerce_checkout_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid) WITH CHECK(false)',r);
 END LOOP;
END $$;
GRANT UPDATE(current_version) ON payments.method_heads TO commerce_checkout_writer;
GRANT UPDATE(id) ON payments.account_qualifications,integration.merchant_accounts,integration.bindings TO commerce_checkout_writer;
GRANT UPDATE(generation) ON inventory.reservations TO commerce_checkout_writer;
ALTER TABLE checkout.command_results DROP CONSTRAINT command_results_operation_check;
ALTER TABLE checkout.command_results ADD CONSTRAINT command_results_operation_check
 CHECK(operation IN ('checkout.begin','checkout.payment.start'));
ALTER TABLE checkout.events DROP CONSTRAINT events_action_check;
ALTER TABLE checkout.events DROP CONSTRAINT events_check;
ALTER TABLE checkout.events ADD CONSTRAINT events_action_check CHECK(action IN ('checkout.held','checkout.expired','checkout.payment_started'));
ALTER TABLE checkout.events ADD CONSTRAINT events_actor_action CHECK(
 (action IN ('checkout.held','checkout.payment_started') AND actor_kind='BUYER')
 OR(action='checkout.expired' AND actor_kind='SYSTEM_EXPIRY'));

CREATE FUNCTION checkout.start_payment(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,
 p_order uuid,p_method text,p_version bigint,p_profile text,p_attempt uuid,p_job bigint) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; s_final record; o checkout.orders%ROWTYPE; r inventory.reservations%ROWTYPE;
 m payments.method_versions%ROWTYPE; a integration.merchant_accounts%ROWTYPE;
 b integration.bindings%ROWTYPE; q payments.account_qualifications%ROWTYPE;
 v_version bigint; v_active boolean; v_now timestamptz; v_trade text; v_result jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
 OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
 OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_attempt IS NULL
 OR p_job IS NULL OR p_job<1 OR p_method IS DISTINCT FROM 'payuni_credit'
 OR p_version IS NULL OR p_version<1 OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'invalid payment input' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true);
 PERFORM set_config('app.store_id',p_store::text,true);
 PERFORM set_config('app.buyer_id',s.owner_id::text,true);
 PERFORM set_config('app.buyer_session_id',s.session_id::text,true);
 PERFORM set_config('app.principal_id','',true);
 PERFORM pg_advisory_xact_lock(hashtextextended('checkout.payment.start|'||s.tenant_id||'|'||p_store||'|'||s.owner_id||'|'||p_key,0));
 IF EXISTS(SELECT 1 FROM checkout.command_results x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.owner_id=s.owner_id AND x.operation='checkout.payment.start' AND x.idempotency_key=p_key) THEN
  RAISE EXCEPTION 'payment already recorded' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO o FROM checkout.orders x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.owner_id=s.owner_id AND x.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO r FROM inventory.reservations x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
 IF NOT FOUND OR o.commercial_state<>'DRAFT' OR r.state<>'HELD' OR r.checkout_id<>o.id
 OR r.buyer_owner_id<>o.owner_id OR r.buyer_session_id<>o.creator_session_id OR r.generation<>o.generation THEN
  RAISE EXCEPTION 'order hold unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.active INTO v_active FROM pricing.markets x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.id=o.market_id AND x.currency=o.currency FOR SHARE;
 IF NOT FOUND OR NOT v_active THEN RAISE EXCEPTION 'payment market unavailable' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_version FROM payments.method_heads h WHERE h.tenant_id=s.tenant_id AND h.store_id=p_store
 AND h.market_id=o.market_id AND h.country=o.country AND h.code=p_method FOR SHARE;
 IF NOT FOUND OR v_version<>p_version THEN RAISE EXCEPTION 'payment method changed' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO m FROM payments.method_versions x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.market_id=o.market_id AND x.country=o.country AND x.code=p_method AND x.version=p_version;
 IF NOT FOUND OR NOT m.enabled OR NOT m.visible OR m.connection_id IS NULL OR m.qualification_id IS NULL
 OR o.currency<>'TWD' OR o.total_minor NOT BETWEEN 100 AND 19999900 OR o.total_minor%100<>0
 OR o.total_minor<m.min_amount_minor OR o.total_minor>m.max_amount_minor THEN
  RAISE EXCEPTION 'payment method unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO a FROM integration.merchant_accounts x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=m.connection_id FOR SHARE;
 IF NOT FOUND OR a.provider<>'payuni' OR a.environment<>m.environment THEN
  RAISE EXCEPTION 'payment account unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=a.binding_id FOR SHARE;
 IF NOT FOUND OR NOT b.enabled OR b.semantic_version<>m.binding_version OR b.provider<>a.provider OR b.external_asset_id<>a.binding_asset THEN
  RAISE EXCEPTION 'payment binding unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO q FROM payments.account_qualifications x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=m.qualification_id FOR SHARE;
 IF NOT FOUND OR q.connection_id<>a.id OR q.credential_version<>a.credential_version OR q.environment<>a.environment OR q.code<>p_method
 OR q.revoked_at IS NOT NULL OR q.proof_class<>(CASE WHEN p_profile='PROVIDER_MOCK' THEN p_profile ELSE 'REAL_'||p_profile END)
 OR (p_profile<>'PROVIDER_MOCK' AND p_profile<>a.environment) THEN
  RAISE EXCEPTION 'payment qualification unavailable' USING ERRCODE='PT409'; END IF;
 v_now:=clock_timestamp();
 IF o.expires_at<=v_now OR r.expires_at<=v_now OR q.observed_at>v_now OR q.expires_at<=v_now THEN
  RAISE EXCEPTION 'payment admission expired' USING ERRCODE='PT409'; END IF;
 IF NOT EXISTS(SELECT 1 FROM river.river_job j WHERE j.id=p_job AND j.kind='payment_query_v1'
 AND j.args=jsonb_build_object('operation_id',p_attempt::text,'version',1)) THEN
  RAISE EXCEPTION 'payment query job missing' USING ERRCODE='PT409'; END IF;
 -- All UUID bits survive in this 23-character provider reference; no truncation.
 v_trade:='P'||rtrim(translate(encode(uuid_send(p_attempt),'base64'),'+/','-_'),'=');
 INSERT INTO checkout.payment_attempts(tenant_id,store_id,owner_id,id,session_id,order_id,market_id,country,
 method_code,method_version,connection_id,credential_version,qualification_id,environment,execution_profile,
 binding_id,binding_version,currency,amount_minor,merchant_trade_no,state,generation,job_id)
 VALUES(s.tenant_id,p_store,s.owner_id,p_attempt,s.session_id,p_order,o.market_id,o.country,p_method,p_version,
 a.id,a.credential_version,q.id,a.environment,p_profile,b.id,b.semantic_version,o.currency,o.total_minor,v_trade,'PAYMENT_PENDING',o.generation+1,p_job);
 INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,
 purpose,action,semantic_key,request_hash,request,job_id,state,generation,actor_kind,payment_attempt_id,buyer_owner_id,buyer_session_id)
 VALUES(p_attempt,s.tenant_id,p_store,NULL,b.id,b.semantic_version,'payuni',b.external_asset_id,'transactional','payuni.query',
 'payment.query:'||p_attempt,p_request_hash,jsonb_build_object('attempt_id',p_attempt::text),p_job,'UNKNOWN',1,'BUYER_PAYMENT_QUERY',p_attempt,s.owner_id,s.session_id);
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(s.tenant_id,p_store,p_attempt,1,'UNKNOWN','','buyer_payment_started');
 UPDATE checkout.orders SET commercial_state='AWAITING_PAYMENT',generation=o.generation+1,updated_at=v_now WHERE id=p_order;
 -- Reservation provenance remains the original checkout session. The payment
 -- event/attempt separately retain the current authenticated session.
 PERFORM set_config('app.buyer_session_id',o.creator_session_id::text,true);
 UPDATE inventory.reservations SET state='PAYMENT_PENDING',generation=o.generation+1
 WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
 PERFORM set_config('app.buyer_session_id',s.session_id::text,true);
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(s.tenant_id,p_store,s.owner_id,p_order,s.session_id,o.generation+1,'checkout.payment_started','BUYER');
 v_result:=jsonb_build_object('order_id',p_order::text,'attempt_id',p_attempt::text,'operation_id',p_attempt::text,
 'job_id',p_job,'generation',o.generation+1,'merchant_trade_no',v_trade,'currency',o.currency,'amount_minor',o.total_minor,'state','PAYMENT_PENDING');
 INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,creator_session_id,request_hash,order_id,response)
 VALUES(s.tenant_id,p_store,s.owner_id,'checkout.payment.start',p_key,s.session_id,p_request_hash,p_order,v_result);
 -- Recheck after all SQL waits, including FK and fault-injection triggers.
 SELECT * INTO s_final FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND OR s_final.tenant_id<>s.tenant_id OR s_final.owner_id<>s.owner_id OR s_final.session_id<>s.session_id THEN
  RAISE EXCEPTION 'buyer capability expired' USING ERRCODE='PT401'; END IF;
 v_now:=clock_timestamp();
 IF o.expires_at<=v_now OR r.expires_at<=v_now OR q.expires_at<=v_now THEN
  RAISE EXCEPTION 'payment admission expired' USING ERRCODE='PT409'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION checkout.start_payment(bytea,uuid,text,bytea,uuid,text,bigint,text,uuid,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.start_payment(bytea,uuid,text,bytea,uuid,text,bigint,text,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.start_payment(bytea,uuid,text,bytea,uuid,text,bigint,text,uuid,bigint) TO commerce_checkout_runtime;

-- Preserve the existing generation/token fence. Only the buyer query family may
-- reconcile a historical disabled/revised binding, never dispatch to a new one.
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
