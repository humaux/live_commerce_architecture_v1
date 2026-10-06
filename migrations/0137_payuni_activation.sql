-- Purpose: PAYUNi merchant self-serve activation (w4-02b): NT$1 SANDBOX verification attempts, LIVE credential probe
--   receipts, the qualification issuers (worker-only for SANDBOX/MOCK, runtime for the LIVE probe), the scoped
--   credential reader, the enabled-method definer and the runtime fence that stops merchants inserting enabled methods.
-- Depends on: roles commerce_payment_registry_writer, commerce_payment_worker, commerce_runtime; payments.account_qualifications,
--   payments.method_versions/method_heads (0015/0016), payments.payuni_notify_endpoints/receipts (0136),
--   integration.merchant_accounts/account_credentials/bindings, ops.audit_events.
-- Used by: internal/payments (payuni_activation.go, methods.go), internal/httpapi/payment_activation.go, cmd/payment-worker (sweep).
-- Invariants: a qualification is only issued from evidence rows (never a caller-supplied proof class); SANDBOX/MOCK needs a
--   CAPTURED query AND a signed notify receipt (two independent channels); LIVE needs a signature-verified read-only probe;
--   enabled=true rows are written only by payments.enable_payuni_method; no transaction, no order, no stock, no finance row.
-- Status: SANDBOX/MOCK evidence only; LIVE path exists, evidence NOT_RUN (no real money, no live key in any test).

-- ------------------------------------------------------------------------------------------------
-- 1. Verification attempts (merchant-initiated NT$1 SANDBOX trade; deliberately NOT checkout.payment_attempts:
--    that table is buyer-owned and FK-bound to orders, sessions and the query job, none of which a verification has).
-- ------------------------------------------------------------------------------------------------
CREATE TABLE payments.payuni_verifications (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,connection_id uuid NOT NULL,
 credential_version bigint NOT NULL CHECK(credential_version>0),
 environment text NOT NULL CHECK(environment='SANDBOX'),
 code text NOT NULL CHECK(code='payuni_credit'),
 execution_profile text NOT NULL CHECK(execution_profile IN ('PROVIDER_MOCK','SANDBOX')),
 idempotency_key text NOT NULL CHECK(idempotency_key ~ '^[A-Za-z0-9_.:-]{8,128}$'),
 merchant_trade_no text NOT NULL UNIQUE CHECK(merchant_trade_no ~ '^V[A-Za-z0-9_-]{1,24}$'),
 currency text NOT NULL DEFAULT 'TWD' CHECK(currency='TWD'),
 amount_minor bigint NOT NULL DEFAULT 100 CHECK(amount_minor=100),
 principal_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL,
 query_report jsonb,query_captured boolean,query_recorded_at timestamptz,
 qualification_id uuid,
 CHECK(expires_at>created_at AND expires_at<=created_at+interval '1 day'),
 CHECK((query_report IS NULL)=(query_captured IS NULL) AND (query_report IS NULL)=(query_recorded_at IS NULL)),
 CHECK(query_report IS NULL OR (jsonb_typeof(query_report)='object' AND length(query_report::text)<=4096)),
 UNIQUE(tenant_id,store_id,idempotency_key),
 FOREIGN KEY(tenant_id,store_id,connection_id,credential_version)
  REFERENCES integration.account_credentials(tenant_id,store_id,connection_id,version),
 FOREIGN KEY(tenant_id,store_id,qualification_id,connection_id,environment,code)
  REFERENCES payments.account_qualifications(tenant_id,store_id,id,connection_id,environment,code),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);

-- One signature-verified read-only LIVE probe (query of a trade that must not exist). signature_verified is pinned true:
-- an unverified probe leaves no row at all.
CREATE TABLE payments.payuni_live_probes (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,connection_id uuid NOT NULL,
 credential_version bigint NOT NULL CHECK(credential_version>0),
 environment text NOT NULL CHECK(environment='LIVE'),
 code text NOT NULL CHECK(code='payuni_credit'),
 probe_trade_no text NOT NULL CHECK(probe_trade_no ~ '^[A-Za-z0-9_-]{1,25}$'),
 signature_verified boolean NOT NULL CHECK(signature_verified),
 principal_id uuid NOT NULL,
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 qualification_id uuid,
 FOREIGN KEY(tenant_id,store_id,connection_id,credential_version)
  REFERENCES integration.account_credentials(tenant_id,store_id,connection_id,version),
 FOREIGN KEY(tenant_id,store_id,qualification_id,connection_id,environment,code)
  REFERENCES payments.account_qualifications(tenant_id,store_id,id,connection_id,environment,code),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);

DO $$ DECLARE v_table text; v_col record; BEGIN
 FOREACH v_table IN ARRAY ARRAY['payuni_verifications','payuni_live_probes'] LOOP
  EXECUTE format('ALTER TABLE payments.%I ENABLE ROW LEVEL SECURITY',v_table);
  EXECUTE format('ALTER TABLE payments.%I FORCE ROW LEVEL SECURITY',v_table);
  EXECUTE format('REVOKE ALL ON payments.%I FROM PUBLIC',v_table);
  EXECUTE format('COMMENT ON TABLE payments.%I IS %L',v_table,
   'payments owner (w4-02b); PAYUNi activation evidence read and written only by the registry-writer definers; no merchant, buyer or worker table grant');
  FOR v_col IN SELECT column_name FROM information_schema.columns
   WHERE table_schema='payments' AND table_name=v_table LOOP
   EXECUTE format('COMMENT ON COLUMN payments.%I.%I IS %L',v_table,v_col.column_name,
    'payments owner; PAYUNi activation evidence; no card data, no credential, no raw provider body');
  END LOOP;
 END LOOP;
END $$;

-- The definers below run as the registry writer and pin scope in their own WHERE clauses (the SANDBOX issuer must read across
-- stores, so the policy is open; the role is NOLOGIN and owns nothing reachable except through these functions).
GRANT SELECT,INSERT ON payments.payuni_verifications,payments.payuni_live_probes TO commerce_payment_registry_writer;
GRANT UPDATE(query_report,query_captured,query_recorded_at,qualification_id) ON payments.payuni_verifications
 TO commerce_payment_registry_writer;
GRANT UPDATE(qualification_id) ON payments.payuni_live_probes TO commerce_payment_registry_writer;
CREATE POLICY payuni_verification_registry ON payments.payuni_verifications TO commerce_payment_registry_writer
 USING(true) WITH CHECK(true);
CREATE POLICY payuni_probe_registry ON payments.payuni_live_probes TO commerce_payment_registry_writer
 USING(true) WITH CHECK(true);
-- The issuer proves the notify channel by reading receipts and the endpoint profile they arrived on.
GRANT SELECT ON payments.payuni_notify_receipts TO commerce_payment_registry_writer;
CREATE POLICY payuni_receipt_registry ON payments.payuni_notify_receipts FOR SELECT TO commerce_payment_registry_writer
 USING(true);

-- ------------------------------------------------------------------------------------------------
-- 2. Merchants (commerce_runtime) can no longer write an enabled PAYUNi method row themselves: enabled=true goes through
--    payments.enable_payuni_method, which validates the qualification. Disabled drafts keep the plain INSERT.
-- ------------------------------------------------------------------------------------------------
CREATE POLICY payuni_method_enabled_definer_only ON payments.method_versions AS RESTRICTIVE
 FOR INSERT TO commerce_runtime WITH CHECK(NOT enabled);

-- ------------------------------------------------------------------------------------------------
-- 3. Definers. Scope (tenant/store/principal) comes from the transaction GUCs the server sets after authentication,
--    never from a parameter. Errors: P0002 unknown/cross-scope, PT409 state conflict, PT412 not qualified, 22023 bad input.
-- ------------------------------------------------------------------------------------------------

-- Start (or replay) one NT$1 SANDBOX verification and make sure the connection has its notify endpoint. The endpoint token
-- hash is derived server-side; an operator-registered endpoint with another hash is never overwritten (PT409).
CREATE FUNCTION payments.start_payuni_verification(p_connection uuid,p_key text,p_profile text,p_token_hash bytea)
RETURNS TABLE(verification_id uuid,merchant_trade_no text,credential_version bigint,created_at timestamptz,
 expires_at timestamptz,replay boolean)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_tenant uuid; v_store uuid; v_principal uuid;
 acct integration.merchant_accounts%ROWTYPE; ver payments.payuni_verifications%ROWTYPE;
 ep payments.payuni_notify_endpoints%ROWTYPE; v_id uuid:=gen_random_uuid();
BEGIN
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 v_principal:=nullif(current_setting('app.principal_id',true),'')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR v_principal IS NULL THEN
  RAISE EXCEPTION 'scope required' USING ERRCODE='PT401'; END IF;
 IF p_connection IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX')
  OR p_token_hash IS NULL OR octet_length(p_token_hash)<>32 THEN
  RAISE EXCEPTION 'invalid PAYUNi verification input' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=p_connection AND x.provider='payuni' FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'PAYUNi connection not found' USING ERRCODE='P0002'; END IF;
 IF acct.environment<>'SANDBOX' THEN
  RAISE EXCEPTION 'verification needs a SANDBOX connection' USING ERRCODE='PT409'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('payuni.verify|'||v_tenant||'|'||v_store||'|'||p_key,0));
 SELECT x.* INTO ver FROM payments.payuni_verifications x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.idempotency_key=p_key;
 IF FOUND THEN
  IF ver.connection_id<>p_connection OR ver.execution_profile<>p_profile THEN
   RAISE EXCEPTION 'idempotency key reused for another verification' USING ERRCODE='PT409'; END IF;
  RETURN QUERY SELECT ver.id,ver.merchant_trade_no,ver.credential_version,ver.created_at,ver.expires_at,true;
  RETURN;
 END IF;
 INSERT INTO payments.payuni_notify_endpoints(endpoint_id,tenant_id,store_id,connection_id,environment,account_id,
  execution_profile,enabled,token_hash)
 VALUES(gen_random_uuid(),v_tenant,v_store,p_connection,acct.environment,acct.account_id,p_profile,true,p_token_hash)
 ON CONFLICT(tenant_id,store_id,connection_id,execution_profile) DO NOTHING;
 SELECT x.* INTO ep FROM payments.payuni_notify_endpoints x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.connection_id=p_connection AND x.execution_profile=p_profile;
 IF NOT FOUND OR NOT ep.enabled OR ep.token_hash<>p_token_hash THEN
  RAISE EXCEPTION 'notify endpoint registered elsewhere' USING ERRCODE='PT409'; END IF;
 -- 23 chars, all uuid bits: V + base64url(uuid).
 INSERT INTO payments.payuni_verifications(id,tenant_id,store_id,connection_id,credential_version,environment,code,
  execution_profile,idempotency_key,merchant_trade_no,principal_id,expires_at)
 VALUES(v_id,v_tenant,v_store,p_connection,acct.credential_version,acct.environment,'payuni_credit',p_profile,p_key,
  'V'||rtrim(translate(encode(uuid_send(v_id),'base64'),'+/','-_'),'='),v_principal,clock_timestamp()+interval '2 hours')
 RETURNING * INTO ver;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(v_tenant,v_store,v_principal,'payuni.verification_started');
 RETURN QUERY SELECT ver.id,ver.merchant_trade_no,ver.credential_version,ver.created_at,ver.expires_at,false;
END $$;
ALTER FUNCTION payments.start_payuni_verification(uuid,text,text,bytea) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.start_payuni_verification(uuid,text,text,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.start_payuni_verification(uuid,text,text,bytea) TO commerce_runtime;
COMMENT ON FUNCTION payments.start_payuni_verification(uuid,text,text,bytea) IS 'payments owner (w4-02b); merchant starts or replays one NT$1 SANDBOX verification for its own PAYUNi connection and its notify endpoint; no order, stock or finance row';

-- Operation-scoped credential reader (the "future reviewed authority" of 0014): the sealed envelope of the exact credential
-- version a verification pinned (SANDBOX) or of a LIVE connection's current version (probe, p_verification NULL). The Go caller
-- opens it with the keyring; this function never returns plaintext.
CREATE FUNCTION payments.load_payuni_activation_material(p_connection uuid,p_verification uuid)
RETURNS TABLE(tenant_id uuid,store_id uuid,connection_id uuid,environment text,account_id text,credential_version bigint,
 key_id text,nonce bytea,ciphertext bytea,verification_id uuid,merchant_trade_no text,execution_profile text,
 amount_minor bigint,created_at timestamptz,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_tenant uuid; v_store uuid;
 acct integration.merchant_accounts%ROWTYPE; c integration.account_credentials%ROWTYPE; ver payments.payuni_verifications%ROWTYPE;
BEGIN
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR nullif(current_setting('app.principal_id',true),'') IS NULL THEN
  RAISE EXCEPTION 'scope required' USING ERRCODE='PT401'; END IF;
 IF p_connection IS NULL THEN RAISE EXCEPTION 'invalid PAYUNi material request' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=p_connection AND x.provider='payuni';
 IF NOT FOUND THEN RAISE EXCEPTION 'PAYUNi connection not found' USING ERRCODE='P0002'; END IF;
 IF p_verification IS NOT NULL THEN
  SELECT x.* INTO ver FROM payments.payuni_verifications x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
   AND x.id=p_verification AND x.connection_id=p_connection;
  IF NOT FOUND THEN RAISE EXCEPTION 'verification not found' USING ERRCODE='P0002'; END IF;
  IF acct.environment<>'SANDBOX' OR ver.credential_version<>acct.credential_version OR ver.expires_at<=clock_timestamp() THEN
   RAISE EXCEPTION 'verification no longer current' USING ERRCODE='PT409'; END IF;
 ELSIF acct.environment<>'LIVE' THEN
  RAISE EXCEPTION 'probe needs a LIVE connection' USING ERRCODE='PT409';
 END IF;
 SELECT x.* INTO c FROM integration.account_credentials x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.connection_id=p_connection AND x.version=acct.credential_version;
 IF NOT FOUND THEN RAISE EXCEPTION 'PAYUNi credential unavailable' USING ERRCODE='PT409'; END IF;
 RETURN QUERY SELECT v_tenant,v_store,acct.id,acct.environment,acct.account_id,acct.credential_version,c.key_id,c.nonce,c.ciphertext,
  ver.id,ver.merchant_trade_no,ver.execution_profile,ver.amount_minor,ver.created_at,ver.expires_at;
END $$;
ALTER FUNCTION payments.load_payuni_activation_material(uuid,uuid) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.load_payuni_activation_material(uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.load_payuni_activation_material(uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION payments.load_payuni_activation_material(uuid,uuid) IS 'payments owner (w4-02b); sealed credential envelope for one own-store verification (SANDBOX) or LIVE probe; ciphertext only, opened by the keyring in the API process';

-- Record the authenticated, projected query report of a verification trade (no raw body, no card data). CAPTURED is
-- re-derived here with the payments.apply_capture rule (TradeNo, DataSource A, TradeStatus 1, CloseStatus 2, CloseAmt = NT$1)
-- and a report carrying refund hints never counts. Not issuance: the worker issues when the notify channel agrees.
CREATE FUNCTION payments.record_payuni_verification_query(p_verification uuid,p_report jsonb) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; ver payments.payuni_verifications%ROWTYPE;
 acct integration.merchant_accounts%ROWTYPE; v_captured boolean;
BEGIN
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR nullif(current_setting('app.principal_id',true),'') IS NULL THEN
  RAISE EXCEPTION 'scope required' USING ERRCODE='PT401'; END IF;
 IF p_verification IS NULL OR p_report IS NULL OR jsonb_typeof(p_report)<>'object' OR length(p_report::text)>4096
  OR p_report->>'MerTradeNo' IS NULL THEN
  RAISE EXCEPTION 'invalid PAYUNi verification report' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO ver FROM payments.payuni_verifications x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=p_verification FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'verification not found' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=ver.connection_id FOR SHARE;
 IF NOT FOUND OR acct.credential_version<>ver.credential_version OR ver.expires_at<=clock_timestamp() THEN
  RAISE EXCEPTION 'verification no longer current' USING ERRCODE='PT409'; END IF;
 IF ver.qualification_id IS NOT NULL OR ver.query_captured IS TRUE THEN RETURN true; END IF; -- evidence is write-once once captured
 v_captured:=p_report->>'MerTradeNo'=ver.merchant_trade_no
  AND p_report->'AmountTWD'=to_jsonb(ver.amount_minor/100)
  AND coalesce(p_report->>'TradeNo','')<>'' AND p_report->>'DataSource'='A' AND p_report->>'TradeStatus'='1'
  AND p_report->>'PaymentType'='1' AND p_report->>'AuthType'='1' AND p_report->>'CloseStatus'='2'
  AND p_report->'CloseAmountTWD'=to_jsonb(ver.amount_minor/100)
  AND coalesce(p_report->>'CardRefundType','')='' AND coalesce(p_report->>'CardRefundStatus','')=''
  AND coalesce(p_report->>'CardRefundDay','')='' AND coalesce((p_report->>'CardRefundAmountTWD')::bigint,0)=0;
 UPDATE payments.payuni_verifications SET query_report=p_report,query_captured=v_captured,query_recorded_at=clock_timestamp()
  WHERE id=ver.id;
 RETURN v_captured;
END $$;
ALTER FUNCTION payments.record_payuni_verification_query(uuid,jsonb) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.record_payuni_verification_query(uuid,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.record_payuni_verification_query(uuid,jsonb) TO commerce_runtime;
COMMENT ON FUNCTION payments.record_payuni_verification_query(uuid,jsonb) IS 'payments owner (w4-02b); stores the projected query report of an own-store verification and derives CAPTURED in SQL; issues nothing';

-- SANDBOX/MOCK issuer (payment worker only). proof_class is derived from the verification's execution profile, never passed in.
-- Issues iff: CAPTURED query evidence AND a signed notify receipt (SUCCESS, TradeStatus 1, NT$1, same connection, trade and
-- endpoint profile) AND the connection still has the credential version the verification pinned. Otherwise returns NULL.
CREATE FUNCTION payments.issue_payuni_qualification(p_verification uuid) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ver payments.payuni_verifications%ROWTYPE; acct integration.merchant_accounts%ROWTYPE; v_id uuid:=gen_random_uuid();
BEGIN
 IF p_verification IS NULL THEN RAISE EXCEPTION 'invalid PAYUNi verification' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO ver FROM payments.payuni_verifications x WHERE x.id=p_verification FOR UPDATE;
 IF NOT FOUND THEN RETURN NULL; END IF;
 PERFORM set_config('app.tenant_id',ver.tenant_id::text,true);
 PERFORM set_config('app.store_id',ver.store_id::text,true);
 PERFORM set_config('app.principal_id',ver.principal_id::text,true);
 IF ver.qualification_id IS NOT NULL THEN RETURN ver.qualification_id; END IF;
 IF ver.query_captured IS NOT TRUE OR ver.query_recorded_at>ver.expires_at THEN RETURN NULL; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=ver.tenant_id AND x.store_id=ver.store_id
  AND x.id=ver.connection_id AND x.provider='payuni' FOR SHARE;
 IF NOT FOUND OR acct.environment<>'SANDBOX' OR acct.credential_version<>ver.credential_version THEN RETURN NULL; END IF;
 IF NOT EXISTS(SELECT 1 FROM payments.payuni_notify_receipts r
   JOIN payments.payuni_notify_endpoints e ON e.endpoint_id=r.endpoint_id
  WHERE r.connection_id=ver.connection_id AND r.merchant_trade_no=ver.merchant_trade_no AND r.status='SUCCESS'
   AND r.trade_status='1' AND r.amount_twd=ver.amount_minor/100 AND r.environment=ver.environment
   AND r.received_at>=ver.created_at AND e.tenant_id=ver.tenant_id AND e.store_id=ver.store_id
   AND e.connection_id=ver.connection_id AND e.execution_profile=ver.execution_profile) THEN
  RETURN NULL; END IF;
 INSERT INTO payments.account_qualifications(id,tenant_id,store_id,connection_id,credential_version,environment,code,
  proof_class,evidence_ref,observed_at,expires_at)
 VALUES(v_id,ver.tenant_id,ver.store_id,ver.connection_id,ver.credential_version,ver.environment,'payuni_credit',
  CASE ver.execution_profile WHEN 'PROVIDER_MOCK' THEN 'PROVIDER_MOCK' ELSE 'REAL_SANDBOX' END,
  'payuni-verify:'||ver.merchant_trade_no,clock_timestamp(),clock_timestamp()+interval '180 days');
 UPDATE payments.payuni_verifications SET qualification_id=v_id WHERE id=ver.id;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(ver.tenant_id,ver.store_id,ver.principal_id,'payuni.qualification_issued');
 RETURN v_id;
END $$;
ALTER FUNCTION payments.issue_payuni_qualification(uuid) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.issue_payuni_qualification(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.issue_payuni_qualification(uuid) TO commerce_payment_worker;
COMMENT ON FUNCTION payments.issue_payuni_qualification(uuid) IS 'payments owner (w4-02b); payment worker issues PROVIDER_MOCK or REAL_SANDBOX only from CAPTURED query evidence plus a signed notify receipt; proof class is derived, never supplied';

-- Worker sweep: issue for every verification whose query evidence is complete. Idempotent; returns the number issued.
CREATE FUNCTION payments.sweep_payuni_verifications(p_limit integer) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r record; v_issued integer:=0;
BEGIN
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 200 THEN
  RAISE EXCEPTION 'invalid sweep limit' USING ERRCODE='22023'; END IF;
 FOR r IN SELECT x.id FROM payments.payuni_verifications x WHERE x.qualification_id IS NULL AND x.query_captured
   AND x.expires_at>clock_timestamp()-interval '1 day' ORDER BY x.created_at LIMIT p_limit FOR UPDATE SKIP LOCKED LOOP
  IF payments.issue_payuni_qualification(r.id) IS NOT NULL THEN v_issued:=v_issued+1; END IF;
 END LOOP;
 RETURN v_issued;
END $$;
ALTER FUNCTION payments.sweep_payuni_verifications(integer) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.sweep_payuni_verifications(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.sweep_payuni_verifications(integer) TO commerce_payment_worker;
COMMENT ON FUNCTION payments.sweep_payuni_verifications(integer) IS 'payments owner (w4-02b); payment worker loop over verifications with CAPTURED evidence, calling the issuer; no provider call';

-- LIVE: record one signature-verified read-only probe (the Go side authenticated the response with the LIVE HashKey/HashIV).
CREATE FUNCTION payments.record_payuni_live_probe(p_connection uuid,p_credential_version bigint,p_probe_trade_no text,
 p_signature_verified boolean) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_principal uuid; acct integration.merchant_accounts%ROWTYPE; v_id uuid:=gen_random_uuid();
BEGIN
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 v_principal:=nullif(current_setting('app.principal_id',true),'')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR v_principal IS NULL THEN
  RAISE EXCEPTION 'scope required' USING ERRCODE='PT401'; END IF;
 IF p_connection IS NULL OR p_credential_version IS NULL OR p_credential_version<1 OR p_signature_verified IS NOT TRUE
  OR p_probe_trade_no IS NULL OR p_probe_trade_no !~ '^[A-Za-z0-9_-]{1,25}$' THEN
  RAISE EXCEPTION 'invalid PAYUNi probe record' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=p_connection AND x.provider='payuni' FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'PAYUNi connection not found' USING ERRCODE='P0002'; END IF;
 IF acct.environment<>'LIVE' OR acct.credential_version<>p_credential_version THEN
  RAISE EXCEPTION 'probe no longer current' USING ERRCODE='PT409'; END IF;
 INSERT INTO payments.payuni_live_probes(id,tenant_id,store_id,connection_id,credential_version,environment,code,
  probe_trade_no,signature_verified,principal_id)
 VALUES(v_id,v_tenant,v_store,p_connection,p_credential_version,'LIVE','payuni_credit',p_probe_trade_no,true,v_principal);
 RETURN v_id;
END $$;
ALTER FUNCTION payments.record_payuni_live_probe(uuid,bigint,text,boolean) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.record_payuni_live_probe(uuid,bigint,text,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.record_payuni_live_probe(uuid,bigint,text,boolean) TO commerce_runtime;
COMMENT ON FUNCTION payments.record_payuni_live_probe(uuid,bigint,text,boolean) IS 'payments owner (w4-02b); stores one signature-verified read-only LIVE probe of the own-store LIVE connection; starts no transaction';

-- LIVE issuer: REAL_LIVE only from a fresh verified probe row of the same credential version.
CREATE FUNCTION payments.issue_payuni_live_qualification(p_probe uuid) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_principal uuid; pr payments.payuni_live_probes%ROWTYPE;
 acct integration.merchant_accounts%ROWTYPE; v_id uuid:=gen_random_uuid();
BEGIN
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 v_principal:=nullif(current_setting('app.principal_id',true),'')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR v_principal IS NULL THEN
  RAISE EXCEPTION 'scope required' USING ERRCODE='PT401'; END IF;
 IF p_probe IS NULL THEN RAISE EXCEPTION 'invalid PAYUNi probe' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO pr FROM payments.payuni_live_probes x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=p_probe FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'probe not found' USING ERRCODE='P0002'; END IF;
 IF pr.qualification_id IS NOT NULL THEN RETURN pr.qualification_id; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=pr.connection_id AND x.provider='payuni' FOR SHARE;
 IF NOT FOUND OR NOT pr.signature_verified OR acct.environment<>'LIVE' OR acct.credential_version<>pr.credential_version
  OR pr.observed_at<clock_timestamp()-interval '10 minutes' THEN
  RAISE EXCEPTION 'probe no longer current' USING ERRCODE='PT409'; END IF;
 INSERT INTO payments.account_qualifications(id,tenant_id,store_id,connection_id,credential_version,environment,code,
  proof_class,evidence_ref,observed_at,expires_at)
 VALUES(v_id,v_tenant,v_store,pr.connection_id,pr.credential_version,'LIVE','payuni_credit','REAL_LIVE',
  'live-probe:'||pr.id,pr.observed_at,pr.observed_at+interval '180 days');
 UPDATE payments.payuni_live_probes SET qualification_id=v_id WHERE id=pr.id;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(v_tenant,v_store,v_principal,'payuni.live_qualification_issued');
 RETURN v_id;
END $$;
ALTER FUNCTION payments.issue_payuni_live_qualification(uuid) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.issue_payuni_live_qualification(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.issue_payuni_live_qualification(uuid) TO commerce_runtime;
COMMENT ON FUNCTION payments.issue_payuni_live_qualification(uuid) IS 'payments owner (w4-02b); issues REAL_LIVE only from a fresh signature-verified probe row of the current credential version; proof class derived';

-- Read model for the settings page and the Go gate: latest verification of a connection and its current valid qualification.
CREATE FUNCTION payments.payuni_activation_status(p_connection uuid)
RETURNS TABLE(verification_id uuid,merchant_trade_no text,verification_created_at timestamptz,verification_expires_at timestamptz,
 query_captured boolean,notify_seen boolean,verification_qualification_id uuid,
 qualification_id uuid,proof_class text,qualification_observed_at timestamptz,qualification_expires_at timestamptz,
 credential_version bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_tenant uuid; v_store uuid; acct integration.merchant_accounts%ROWTYPE;
 ver payments.payuni_verifications%ROWTYPE; q payments.account_qualifications%ROWTYPE; v_notify boolean:=false;
BEGIN
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR nullif(current_setting('app.principal_id',true),'') IS NULL THEN
  RAISE EXCEPTION 'scope required' USING ERRCODE='PT401'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=p_connection AND x.provider='payuni';
 IF NOT FOUND THEN RAISE EXCEPTION 'PAYUNi connection not found' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO ver FROM payments.payuni_verifications x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.connection_id=p_connection ORDER BY x.created_at DESC LIMIT 1;
 IF FOUND THEN
  v_notify:=EXISTS(SELECT 1 FROM payments.payuni_notify_receipts r WHERE r.connection_id=ver.connection_id
   AND r.merchant_trade_no=ver.merchant_trade_no AND r.status='SUCCESS' AND r.trade_status='1');
 END IF;
 SELECT x.* INTO q FROM payments.account_qualifications x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.connection_id=p_connection AND x.credential_version=acct.credential_version AND x.environment=acct.environment
  AND x.code='payuni_credit' AND x.revoked_at IS NULL AND x.expires_at>clock_timestamp()
  ORDER BY x.observed_at DESC LIMIT 1;
 RETURN QUERY SELECT ver.id,ver.merchant_trade_no,ver.created_at,ver.expires_at,ver.query_captured,v_notify,ver.qualification_id,
  q.id,q.proof_class,q.observed_at,q.expires_at,acct.credential_version;
END $$;
ALTER FUNCTION payments.payuni_activation_status(uuid) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.payuni_activation_status(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.payuni_activation_status(uuid) TO commerce_runtime;
COMMENT ON FUNCTION payments.payuni_activation_status(uuid) IS 'payments owner (w4-02b); own-store read model of the latest verification and the current valid qualification; no credential, no raw report';

-- Qualification facts of one method revision's qualification, for InspectMethod (commerce_runtime has no table grant on
-- payments.account_qualifications and does not get one).
CREATE FUNCTION payments.method_qualification_state(p_qualification uuid)
RETURNS TABLE(connection_id uuid,credential_version bigint,environment text,code text,proof_class text,
 observed_at timestamptz,expires_at timestamptz,revoked_at timestamptz)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_tenant uuid; v_store uuid;
BEGIN
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR nullif(current_setting('app.principal_id',true),'') IS NULL THEN
  RAISE EXCEPTION 'scope required' USING ERRCODE='PT401'; END IF;
 RETURN QUERY SELECT q.connection_id,q.credential_version,q.environment,q.code,q.proof_class,q.observed_at,q.expires_at,q.revoked_at
  FROM payments.account_qualifications q WHERE q.tenant_id=v_tenant AND q.store_id=v_store AND q.id=p_qualification;
END $$;
ALTER FUNCTION payments.method_qualification_state(uuid) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.method_qualification_state(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.method_qualification_state(uuid) TO commerce_runtime;
COMMENT ON FUNCTION payments.method_qualification_state(uuid) IS 'payments owner (w4-02b); own-store qualification facts for the merchant diagnostic; no evidence reference';

-- The only writer of an enabled PAYUNi method revision. The qualification is found, never supplied: the latest valid row for
-- this connection at its CURRENT credential version, in the connection's environment (LIVE needs REAL_LIVE; SANDBOX accepts
-- REAL_SANDBOX or PROVIDER_MOCK, whose use is fenced by the payment profile at start_payment/buyer view). Else PT412.
-- Head movement stays with the Go caller's existing CAS (merchant head grants are unchanged).
CREATE FUNCTION payments.enable_payuni_method(p_market uuid,p_country text,p_code text,p_version bigint,p_environment text,
 p_connection uuid,p_binding_version bigint,p_name_hans text,p_name_hant text,p_name_en text,p_visible boolean,
 p_sort integer,p_min bigint,p_max bigint) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_principal uuid; market pricing.markets%ROWTYPE;
 acct integration.merchant_accounts%ROWTYPE; q payments.account_qualifications%ROWTYPE;
BEGIN
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 v_principal:=nullif(current_setting('app.principal_id',true),'')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR v_principal IS NULL THEN
  RAISE EXCEPTION 'scope required' USING ERRCODE='PT401'; END IF;
 IF p_market IS NULL OR p_country IS DISTINCT FROM 'TW' OR p_code IS DISTINCT FROM 'payuni_credit' OR p_version IS NULL OR p_version<1
  OR p_environment IS NULL OR p_environment NOT IN ('SANDBOX','LIVE') OR p_connection IS NULL OR p_binding_version IS NULL
  OR p_binding_version<1 OR p_visible IS NULL OR p_sort IS NULL OR p_min IS NULL OR p_max IS NULL THEN
  RAISE EXCEPTION 'invalid enabled PAYUNi method' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO market FROM pricing.markets x WHERE x.tenant_id=v_tenant AND x.store_id=v_store AND x.id=p_market FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'market not found' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.id=p_connection AND x.provider='payuni' AND x.environment=p_environment FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'PAYUNi connection not found' USING ERRCODE='P0002'; END IF;
 SELECT x.* INTO q FROM payments.account_qualifications x WHERE x.tenant_id=v_tenant AND x.store_id=v_store
  AND x.connection_id=p_connection AND x.credential_version=acct.credential_version AND x.environment=acct.environment
  AND x.code=p_code AND x.revoked_at IS NULL AND x.observed_at<=clock_timestamp() AND x.expires_at>clock_timestamp()
  AND x.proof_class=ANY(CASE acct.environment WHEN 'LIVE' THEN ARRAY['REAL_LIVE'] ELSE ARRAY['REAL_SANDBOX','PROVIDER_MOCK'] END)
  ORDER BY x.observed_at DESC LIMIT 1;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment method not qualified' USING ERRCODE='PT412'; END IF;
 INSERT INTO payments.method_versions(tenant_id,store_id,market_id,country,code,version,provider,environment,
  connection_id,binding_version,currency,name_hans,name_hant,name_en,enabled,visible,sort_order,
  min_amount_minor,max_amount_minor,principal_id,qualification_id)
 VALUES(v_tenant,v_store,p_market,p_country,p_code,p_version,'payuni',p_environment,p_connection,p_binding_version,
  market.currency,p_name_hans,p_name_hant,p_name_en,true,p_visible,p_sort,p_min,p_max,v_principal,q.id);
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(v_tenant,v_store,v_principal,'payuni.method_enabled');
 RETURN q.id;
END $$;
ALTER FUNCTION payments.enable_payuni_method(uuid,text,text,bigint,text,uuid,bigint,text,text,text,boolean,integer,bigint,bigint)
 OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.enable_payuni_method(uuid,text,text,bigint,text,uuid,bigint,text,text,text,boolean,integer,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.enable_payuni_method(uuid,text,text,bigint,text,uuid,bigint,text,text,text,boolean,integer,bigint,bigint)
 TO commerce_runtime;
COMMENT ON FUNCTION payments.enable_payuni_method(uuid,text,text,bigint,text,uuid,bigint,text,text,text,boolean,integer,bigint,bigint) IS 'payments owner (w4-02b); the only writer of an enabled PAYUNi method revision; finds the current valid qualification itself, PT412 when none; head CAS stays with SetMethod';
