-- The trusted hosted signer is a separate login authority, not the ordinary
-- checkout pool. It owns neither schema nor tables and cannot SET a writer role.
CREATE ROLE commerce_hosted_runtime NOLOGIN INHERIT NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT commerce_checkout_runtime TO commerce_hosted_runtime WITH INHERIT TRUE, SET FALSE;

-- ponytail: derive scope/order from the immutable attempt instead of duplicating
-- ownership columns. Its existing unique order constraint also prevents a second
-- hosted page for that order. No placeholder or mutable form is permitted.
CREATE TABLE checkout.hosted_payment_pages (
 attempt_id uuid PRIMARY KEY REFERENCES checkout.payment_attempts(id),
 locale text NOT NULL CHECK(locale IN ('zh-CN','zh-TW','en')),
 config_digest bytea NOT NULL CHECK(octet_length(config_digest)=32),
 prepared_at timestamptz NOT NULL,expires_at timestamptz NOT NULL,
 form jsonb NOT NULL CHECK(jsonb_typeof(form)='object' AND octet_length(form::text)<=32768),
 handed_out_at timestamptz,
 CHECK(expires_at>prepared_at AND expires_at<=prepared_at+interval '60 seconds'),
 CHECK(handed_out_at IS NULL OR (handed_out_at>=prepared_at AND handed_out_at<expires_at))
);
ALTER TABLE checkout.hosted_payment_pages ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.hosted_payment_pages FORCE ROW LEVEL SECURITY;
CREATE POLICY hosted_read ON checkout.hosted_payment_pages FOR SELECT TO commerce_checkout_writer
 USING(EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.id=attempt_id
 AND a.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND a.store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND a.owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid));
CREATE POLICY hosted_insert ON checkout.hosted_payment_pages FOR INSERT TO commerce_checkout_writer
 WITH CHECK(handed_out_at IS NULL AND EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.id=attempt_id
 AND a.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND a.store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND a.owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid));
CREATE POLICY hosted_take ON checkout.hosted_payment_pages FOR UPDATE TO commerce_checkout_writer
 USING(handed_out_at IS NULL AND EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.id=attempt_id
 AND a.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND a.store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND a.owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid))
 WITH CHECK(handed_out_at IS NOT NULL AND EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.id=attempt_id
 AND a.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND a.store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND a.owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid));
GRANT SELECT,INSERT ON checkout.hosted_payment_pages TO commerce_checkout_writer;
GRANT UPDATE(handed_out_at) ON checkout.hosted_payment_pages TO commerce_checkout_writer;

-- Only an explicitly owned frozen attempt can authorize this historical
-- ciphertext read. Do not rely on capture_attempt_lookup's broader SELECT RLS.
GRANT SELECT(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext)
 ON integration.account_credentials TO commerce_checkout_writer;
CREATE POLICY hosted_credential_read ON integration.account_credentials FOR SELECT TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND EXISTS(SELECT 1 FROM checkout.payment_attempts a
 WHERE a.tenant_id=account_credentials.tenant_id AND a.store_id=account_credentials.store_id
 AND a.connection_id=account_credentials.connection_id AND a.credential_version=account_credentials.version
 AND a.owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid));

-- All callers re-resolve the capability before touching an owned order. Lock
-- order first, as payment start/capture do; never lock integration operations.
-- p_live=false permits a no-form replay after payment/expiry/config disable.
CREATE FUNCTION checkout.hosted_guard(p_hash bytea,p_store uuid,p_order uuid,p_profile text,p_live boolean)
RETURNS checkout.payment_attempts LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; sf record; o checkout.orders%ROWTYPE; r inventory.reservations%ROWTYPE;
 a checkout.payment_attempts%ROWTYPE; m payments.method_versions%ROWTYPE;
 c integration.merchant_accounts%ROWTYPE; b integration.bindings%ROWTYPE;
 q payments.account_qualifications%ROWTYPE; v_version bigint; v_active boolean; v_now timestamptz;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
 OR p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
 OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') OR p_live IS NULL THEN
  RAISE EXCEPTION 'invalid hosted request' USING ERRCODE='22023'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true);
 PERFORM set_config('app.store_id',p_store::text,true);
 PERFORM set_config('app.buyer_id',s.owner_id::text,true);
 PERFORM set_config('app.buyer_session_id',s.session_id::text,true);
 PERFORM set_config('app.principal_id','',true);
 SELECT x.* INTO o FROM checkout.orders x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.owner_id=s.owner_id AND x.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'hosted order unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
 AND x.owner_id=s.owner_id AND x.order_id=o.id;
 IF NOT FOUND OR a.execution_profile<>p_profile OR a.currency<>o.currency OR a.amount_minor<>o.total_minor
 OR a.market_id<>o.market_id OR a.country<>o.country THEN
  RAISE EXCEPTION 'hosted attempt unavailable' USING ERRCODE='PT409'; END IF;
 IF p_live THEN
  SELECT x.* INTO r FROM inventory.reservations x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.id=o.id FOR UPDATE;
  IF NOT FOUND OR o.commercial_state<>'AWAITING_PAYMENT' OR r.state<>'PAYMENT_PENDING'
  OR r.checkout_id IS DISTINCT FROM o.id OR r.buyer_owner_id IS DISTINCT FROM o.owner_id
  OR r.buyer_session_id IS DISTINCT FROM o.creator_session_id OR r.generation<>o.generation
  OR a.generation<>o.generation THEN
   RAISE EXCEPTION 'hosted hold unavailable' USING ERRCODE='PT409'; END IF;
  IF EXISTS(SELECT 1 FROM payments.facts x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.attempt_id=a.id)
  OR EXISTS(SELECT 1 FROM payments.review_cases x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.attempt_id=a.id) THEN
   RAISE EXCEPTION 'hosted financial review required' USING ERRCODE='PT409'; END IF;
  SELECT x.active INTO v_active FROM pricing.markets x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.id=o.market_id AND x.currency=o.currency FOR SHARE;
  IF NOT FOUND OR NOT v_active THEN RAISE EXCEPTION 'hosted market unavailable' USING ERRCODE='PT409'; END IF;
  SELECT x.current_version INTO v_version FROM payments.method_heads x WHERE x.tenant_id=s.tenant_id
  AND x.store_id=p_store AND x.market_id=a.market_id AND x.country=a.country AND x.code=a.method_code FOR SHARE;
  IF NOT FOUND OR v_version<>a.method_version THEN RAISE EXCEPTION 'hosted method changed' USING ERRCODE='PT409'; END IF;
  SELECT x.* INTO m FROM payments.method_versions x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.market_id=a.market_id AND x.country=a.country AND x.code=a.method_code AND x.version=a.method_version;
  IF NOT FOUND OR NOT m.enabled OR NOT m.visible OR m.connection_id IS DISTINCT FROM a.connection_id
  OR m.qualification_id IS DISTINCT FROM a.qualification_id OR m.environment<>a.environment
  OR m.binding_version IS DISTINCT FROM a.binding_version OR a.amount_minor<m.min_amount_minor OR a.amount_minor>m.max_amount_minor THEN
   RAISE EXCEPTION 'hosted method unavailable' USING ERRCODE='PT409'; END IF;
  SELECT x.* INTO c FROM integration.merchant_accounts x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.id=a.connection_id FOR SHARE;
  IF NOT FOUND OR c.provider<>'payuni' OR c.environment<>a.environment OR c.credential_version<>a.credential_version
  OR c.binding_id<>a.binding_id THEN RAISE EXCEPTION 'hosted account changed' USING ERRCODE='PT409'; END IF;
  SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.id=a.binding_id FOR SHARE;
  IF NOT FOUND OR NOT b.enabled OR b.semantic_version<>a.binding_version OR b.provider<>c.provider
  OR b.external_asset_id<>c.binding_asset THEN RAISE EXCEPTION 'hosted binding changed' USING ERRCODE='PT409'; END IF;
  SELECT x.* INTO q FROM payments.account_qualifications x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.id=a.qualification_id FOR SHARE;
  IF NOT FOUND OR q.connection_id<>a.connection_id OR q.credential_version<>a.credential_version
  OR q.environment<>a.environment OR q.code<>a.method_code OR q.revoked_at IS NOT NULL
  OR q.proof_class<>(CASE WHEN p_profile='PROVIDER_MOCK' THEN p_profile ELSE 'REAL_'||p_profile END)
  OR (p_profile='PROVIDER_MOCK' AND a.environment<>'SANDBOX')
  OR (p_profile<>'PROVIDER_MOCK' AND p_profile<>a.environment) THEN
   RAISE EXCEPTION 'hosted qualification unavailable' USING ERRCODE='PT409'; END IF;
  v_now:=clock_timestamp();
  IF a.created_at>v_now OR q.observed_at>v_now OR least(a.created_at+interval '60 seconds',q.expires_at)<=v_now THEN
   RAISE EXCEPTION 'hosted release expired' USING ERRCODE='PT409'; END IF;
 END IF;
 SELECT * INTO sf FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND OR sf.tenant_id<>s.tenant_id OR sf.owner_id<>s.owner_id OR sf.session_id<>s.session_id THEN
  RAISE EXCEPTION 'buyer capability expired' USING ERRCODE='PT401'; END IF;
 RETURN a;
END $$;
ALTER FUNCTION checkout.hosted_guard(bytea,uuid,uuid,text,boolean) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.hosted_guard(bytea,uuid,uuid,text,boolean) FROM PUBLIC;

CREATE FUNCTION checkout.load_hosted_material(p_hash bytea,p_store uuid,p_order uuid,p_profile text,p_locale text)
RETURNS TABLE(tenant_id uuid,store_id uuid,connection_id uuid,provider text,environment text,account_id text,
 credential_version bigint,key_id text,nonce bytea,ciphertext bytea,merchant_trade_no text,currency text,
 amount_minor bigint,method_code text,prepared_at timestamptz,expires_at timestamptz)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE;
BEGIN
 IF p_locale IS NULL OR p_locale NOT IN ('zh-CN','zh-TW','en') THEN
  RAISE EXCEPTION 'invalid hosted locale' USING ERRCODE='22023'; END IF;
 a:=checkout.hosted_guard(p_hash,p_store,p_order,p_profile,true);
 IF EXISTS(SELECT 1 FROM checkout.hosted_payment_pages x WHERE x.attempt_id=a.id) THEN
  RAISE EXCEPTION 'hosted page already prepared' USING ERRCODE='PT409'; END IF;
 RETURN QUERY SELECT a.tenant_id,a.store_id,a.connection_id,c.provider,c.environment,c.account_id,
 a.credential_version,k.key_id,k.nonce,k.ciphertext,a.merchant_trade_no,a.currency,a.amount_minor,a.method_code,
 a.created_at,least(a.created_at+interval '60 seconds',q.expires_at)
 FROM integration.merchant_accounts c JOIN integration.account_credentials k
 ON k.tenant_id=c.tenant_id AND k.store_id=c.store_id AND k.connection_id=c.id AND k.version=a.credential_version
 JOIN payments.account_qualifications q ON q.tenant_id=a.tenant_id AND q.store_id=a.store_id AND q.id=a.qualification_id
 WHERE c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.id=a.connection_id;
 -- Recheck after all reads, so late waits do not leak stale scoped material.
 PERFORM checkout.hosted_guard(p_hash,p_store,p_order,p_profile,true);
END $$;
ALTER FUNCTION checkout.load_hosted_material(bytea,uuid,uuid,text,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.load_hosted_material(bytea,uuid,uuid,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.load_hosted_material(bytea,uuid,uuid,text,text) TO commerce_hosted_runtime;

CREATE FUNCTION checkout.save_hosted_page(p_hash bytea,p_store uuid,p_order uuid,p_profile text,p_locale text,
 p_config bytea,p_form jsonb) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; v_account text; v_deadline timestamptz; f jsonb;
BEGIN
 IF p_locale IS NULL OR p_locale NOT IN ('zh-CN','zh-TW','en') OR p_config IS NULL OR octet_length(p_config)<>32
 OR p_form IS NULL OR jsonb_typeof(p_form)<>'object' OR octet_length(p_form::text)>32768 THEN
  RAISE EXCEPTION 'invalid hosted form' USING ERRCODE='22023'; END IF;
 f:=p_form->'fields';
 IF jsonb_typeof(f) IS DISTINCT FROM 'object' OR jsonb_typeof(p_form->'action') IS DISTINCT FROM 'string' THEN
  RAISE EXCEPTION 'invalid hosted form' USING ERRCODE='22023'; END IF;
 IF (SELECT count(*) FROM jsonb_object_keys(p_form))<>2 OR (SELECT count(*) FROM jsonb_object_keys(f))<>4
 OR NOT (f ?& ARRAY['MerID','Version','EncryptInfo','HashInfo'])
 OR EXISTS(SELECT 1 FROM jsonb_each(f) x WHERE jsonb_typeof(x.value)<>'string') THEN
  RAISE EXCEPTION 'invalid hosted form' USING ERRCODE='22023'; END IF;
 a:=checkout.hosted_guard(p_hash,p_store,p_order,p_profile,true);
 SELECT c.account_id INTO v_account FROM integration.merchant_accounts c
 WHERE c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.id=a.connection_id;
 SELECT least(a.created_at+interval '60 seconds',q.expires_at) INTO v_deadline
 FROM payments.account_qualifications q WHERE q.tenant_id=a.tenant_id AND q.store_id=a.store_id AND q.id=a.qualification_id;
 IF p_form->>'action'<>(CASE WHEN a.environment='SANDBOX' THEN 'https://sandbox-api.payuni.com.tw/api/upp'
 ELSE 'https://api.payuni.com.tw/api/upp' END) OR f->>'MerID'<>v_account OR f->>'Version'<>'2.0'
 OR f->>'EncryptInfo' !~ '^[0-9a-f]{16,24576}$' OR length(f->>'EncryptInfo')%2<>0
 OR f->>'HashInfo' !~ '^[0-9A-F]{64}$' THEN
  RAISE EXCEPTION 'invalid hosted form' USING ERRCODE='22023'; END IF;
 -- SQL authenticates the scope/outer shape. Only the trusted Go keyring can
 -- construct and verify the encrypted trade/amount/URLs; never accept buyer JSON.
 INSERT INTO checkout.hosted_payment_pages(attempt_id,locale,config_digest,prepared_at,expires_at,form)
 VALUES(a.id,p_locale,p_config,a.created_at,v_deadline,p_form);
 PERFORM checkout.hosted_guard(p_hash,p_store,p_order,p_profile,true);
END $$;
ALTER FUNCTION checkout.save_hosted_page(bytea,uuid,uuid,text,text,bytea,jsonb) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.save_hosted_page(bytea,uuid,uuid,text,text,bytea,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.save_hosted_page(bytea,uuid,uuid,text,text,bytea,jsonb) TO commerce_hosted_runtime;

CREATE FUNCTION checkout.take_hosted_page(p_hash bytea,p_store uuid,p_order uuid,p_profile text,p_config bytea)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; h checkout.hosted_payment_pages%ROWTYPE;
BEGIN
 IF p_config IS NULL OR octet_length(p_config)<>32 THEN
  RAISE EXCEPTION 'invalid hosted config' USING ERRCODE='22023'; END IF;
 a:=checkout.hosted_guard(p_hash,p_store,p_order,p_profile,false);
 SELECT x.* INTO h FROM checkout.hosted_payment_pages x WHERE x.attempt_id=a.id;
 IF NOT FOUND OR h.config_digest<>p_config THEN RAISE EXCEPTION 'hosted page unavailable' USING ERRCODE='PT409'; END IF;
 IF h.handed_out_at IS NOT NULL THEN
  PERFORM checkout.hosted_guard(p_hash,p_store,p_order,p_profile,false);
  RETURN jsonb_build_object('order_id',p_order,'disposition','ALREADY_ISSUED','expires_at',h.expires_at);
 END IF;
 PERFORM checkout.hosted_guard(p_hash,p_store,p_order,p_profile,true);
 IF h.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'hosted release expired' USING ERRCODE='PT409'; END IF;
 UPDATE checkout.hosted_payment_pages SET handed_out_at=clock_timestamp() WHERE attempt_id=a.id AND handed_out_at IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'hosted release conflict' USING ERRCODE='PT409'; END IF;
 -- The result must not leave the Go transaction boundary until commit succeeds.
 -- Final fences catch expiry/revocation during triggers and FK waits.
 PERFORM checkout.hosted_guard(p_hash,p_store,p_order,p_profile,true);
 IF h.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'hosted release expired' USING ERRCODE='PT409'; END IF;
 RETURN jsonb_build_object('order_id',p_order,'disposition','ISSUED','expires_at',h.expires_at,'form',h.form);
END $$;
ALTER FUNCTION checkout.take_hosted_page(bytea,uuid,uuid,text,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.take_hosted_page(bytea,uuid,uuid,text,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.take_hosted_page(bytea,uuid,uuid,text,bytea) TO commerce_hosted_runtime;
