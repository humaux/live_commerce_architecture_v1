-- A leased query can read only the attempt's frozen credential version. The
-- private definer is non-login; ordinary workers receive no ciphertext SELECT.
GRANT USAGE ON SCHEMA checkout,payments TO commerce_integration_writer;
GRANT SELECT ON checkout.payment_attempts,integration.merchant_accounts,
 integration.account_credentials TO commerce_integration_writer;
CREATE POLICY payment_query_reader ON checkout.payment_attempts FOR SELECT
 TO commerce_integration_writer USING(true);
CREATE POLICY payment_query_reader ON integration.merchant_accounts FOR SELECT
 TO commerce_integration_writer USING(true);
CREATE POLICY payment_query_reader ON integration.account_credentials FOR SELECT
 TO commerce_integration_writer USING(true);

CREATE TABLE payments.provider_observations (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,attempt_id uuid NOT NULL,
 source text NOT NULL CHECK(source='QUERY'),
 execution_profile text NOT NULL CHECK(execution_profile IN ('PROVIDER_MOCK','SANDBOX','LIVE')),
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 first_generation bigint NOT NULL CHECK(first_generation>1),
 report jsonb NOT NULL CHECK(jsonb_typeof(report)='object' AND octet_length(report::text)<=2048),
 report_hash bytea NOT NULL CHECK(octet_length(report_hash)=32),
 received_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,attempt_id,report_hash),
 FOREIGN KEY(tenant_id,store_id,attempt_id) REFERENCES integration.operations(tenant_id,store_id,id),
 CHECK(report_hash=sha256(convert_to(report::text,'UTF8')))
);
ALTER TABLE payments.provider_observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments.provider_observations FORCE ROW LEVEL SECURITY;
CREATE POLICY payment_query_observation ON payments.provider_observations TO commerce_integration_writer
 USING(true) WITH CHECK(true);
GRANT SELECT,INSERT ON payments.provider_observations TO commerce_integration_writer;
-- Once pinned, a provider transaction must not be assigned to another attempt
-- in this account. No financial claim is inferred from this uniqueness rule.
CREATE UNIQUE INDEX payment_query_provider_reference ON integration.operations
 (tenant_id,store_id,binding_id,provider_reference)
 WHERE actor_kind='BUYER_PAYMENT_QUERY' AND provider_reference<>'';

-- Shared private guard, not executable by any login/runtime role. Lock order
-- matches Claim/Complete; holding only local row locks never spans network I/O.
CREATE FUNCTION integration.require_payment_query(p_id uuid,p_generation bigint,p_token bytea,p_profile text)
RETURNS checkout.payment_attempts LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; b integration.bindings%ROWTYPE;
 a checkout.payment_attempts%ROWTYPE; v_binding uuid;
BEGIN
 IF p_id IS NULL OR p_generation IS NULL OR p_generation<2 OR p_token IS NULL OR octet_length(p_token)<>32
 OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'invalid payment query' USING ERRCODE='22023'; END IF;
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
ALTER FUNCTION integration.require_payment_query(uuid,bigint,bytea,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.require_payment_query(uuid,bigint,bytea,text) FROM PUBLIC;

CREATE FUNCTION integration.load_payment_query(p_id uuid,p_generation bigint,p_token bytea,p_profile text)
RETURNS TABLE(tenant_id uuid,store_id uuid,connection_id uuid,provider text,environment text,
 account_id text,credential_version bigint,key_id text,nonce bytea,ciphertext bytea,
 merchant_trade_no text,currency text,amount_minor bigint,method_code text,
 provider_reference text,age_seconds double precision)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; c integration.account_credentials%ROWTYPE;
 m integration.merchant_accounts%ROWTYPE; o integration.operations%ROWTYPE;
BEGIN
 a:=integration.require_payment_query(p_id,p_generation,p_token,p_profile);
 SELECT x.* INTO m FROM integration.merchant_accounts x WHERE x.id=a.connection_id
 AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id AND x.binding_id=a.binding_id
 AND x.provider='payuni' AND x.environment=a.environment;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment account unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO c FROM integration.account_credentials x WHERE x.tenant_id=a.tenant_id
 AND x.store_id=a.store_id AND x.connection_id=a.connection_id AND x.version=a.credential_version;
 IF NOT FOUND THEN RAISE EXCEPTION 'payment credential unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=a.id;
 IF o.external_asset_id<>m.environment||':'||m.account_id THEN
  RAISE EXCEPTION 'payment account mismatch' USING ERRCODE='PT409'; END IF;
 -- Table locks/FK waits may outlive the claim. Recheck before releasing secrets.
 PERFORM integration.require_payment_query(p_id,p_generation,p_token,p_profile);
 RETURN QUERY SELECT a.tenant_id,a.store_id,a.connection_id,m.provider,a.environment,m.account_id,
 a.credential_version,c.key_id,c.nonce,c.ciphertext,a.merchant_trade_no,a.currency,a.amount_minor,
 a.method_code,o.provider_reference,extract(epoch FROM clock_timestamp()-a.created_at)::double precision;
END $$;
ALTER FUNCTION integration.load_payment_query(uuid,bigint,bytea,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.load_payment_query(uuid,bigint,bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.load_payment_query(uuid,bigint,bytea,text) TO commerce_worker;

CREATE FUNCTION integration.record_payment_query(p_id uuid,p_generation bigint,p_token bytea,p_profile text,p_report jsonb)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; v_ref text; v_lease timestamptz;
BEGIN
 a:=integration.require_payment_query(p_id,p_generation,p_token,p_profile);
 IF p_report IS NULL OR jsonb_typeof(p_report)<>'object' OR octet_length(p_report::text)>2048 THEN
  RAISE EXCEPTION 'invalid payment report' USING ERRCODE='22023'; END IF;
 IF (SELECT count(*) FROM jsonb_object_keys(p_report))<>10
 OR NOT p_report ?& ARRAY['MerTradeNo','TradeNo','AmountTWD','PaymentType','TradeStatus','Status','AuthType','CardInst','DataSource','CloseStatus']
 OR EXISTS(SELECT 1 FROM jsonb_each(p_report) e WHERE e.key NOT IN ('AmountTWD','CardInst') AND jsonb_typeof(e.value)<>'string')
 OR p_report->'AmountTWD' IS DISTINCT FROM to_jsonb(a.amount_minor/100)
 OR p_report->'CardInst' IS DISTINCT FROM '0'::jsonb
 OR p_report->>'MerTradeNo'<>a.merchant_trade_no OR a.currency<>'TWD' OR a.method_code<>'payuni_credit'
 OR p_report->>'PaymentType'<>'1' OR p_report->>'AuthType'<>'1' OR p_report->>'Status'<>'SUCCESS'
 OR p_report->>'TradeStatus' NOT IN ('0','1','2','3','4','8','9')
 OR p_report->>'DataSource' NOT IN ('A','B') OR p_report->>'CloseStatus' NOT IN ('','1','2','3','7','9')
 OR (p_report->>'TradeNo'<>'' AND p_report->>'TradeNo' !~ '^[A-Za-z0-9_-]{1,64}$') THEN
  RAISE EXCEPTION 'payment report mismatch' USING ERRCODE='PT409'; END IF;
 SELECT x.provider_reference,x.lease_until INTO v_ref,v_lease FROM integration.operations x WHERE x.id=p_id;
 IF v_ref<>'' AND v_ref<>p_report->>'TradeNo' THEN
  RAISE EXCEPTION 'payment reference changed' USING ERRCODE='PT409'; END IF;
 INSERT INTO payments.provider_observations(tenant_id,store_id,attempt_id,source,execution_profile,environment,
 first_generation,report,report_hash)
 VALUES(a.tenant_id,a.store_id,a.id,'QUERY',p_profile,a.environment,p_generation,p_report,sha256(convert_to(p_report::text,'UTF8')))
 ON CONFLICT(tenant_id,store_id,attempt_id,report_hash) DO NOTHING;
 PERFORM integration.complete_operation(p_id,p_generation,p_token,'UNKNOWN','payment_report_observed',p_report->>'TradeNo');
 -- Completion's event triggers can wait too; roll back the entire report and
 -- cleared lease if the original fencing deadline has passed during any write.
 IF clock_timestamp()>=v_lease THEN
  RAISE EXCEPTION 'payment query lease conflict' USING ERRCODE='40001'; END IF;
END $$;
ALTER FUNCTION integration.record_payment_query(uuid,bigint,bytea,text,jsonb) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.record_payment_query(uuid,bigint,bytea,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.record_payment_query(uuid,bigint,bytea,text,jsonb) TO commerce_worker;

-- Error and budget outcomes also retain the post-write clock fence. An event
-- table wait must not clear an expired lease or change its pinned reference.
CREATE FUNCTION integration.finish_payment_query(p_id uuid,p_generation bigint,p_token bytea,
 p_profile text,p_code text,p_reference text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_lease timestamptz; v_reference text;
BEGIN
 PERFORM integration.require_payment_query(p_id,p_generation,p_token,p_profile);
 IF p_code IS NULL OR p_code NOT IN ('payment_query_budget_exhausted','payment_query_timeout',
 'payment_query_material_unavailable','payment_query_panic','payment_query_wire_failed','payment_query_record_failed') THEN
  RAISE EXCEPTION 'invalid payment query completion' USING ERRCODE='22023'; END IF;
 SELECT x.lease_until,x.provider_reference INTO v_lease,v_reference FROM integration.operations x WHERE x.id=p_id;
 IF p_reference IS DISTINCT FROM v_reference THEN
  RAISE EXCEPTION 'payment query reference mismatch' USING ERRCODE='PT409'; END IF;
 PERFORM integration.complete_operation(p_id,p_generation,p_token,'UNKNOWN',p_code,p_reference);
 IF clock_timestamp()>=v_lease THEN
  RAISE EXCEPTION 'payment query lease conflict' USING ERRCODE='40001'; END IF;
END $$;
ALTER FUNCTION integration.finish_payment_query(uuid,bigint,bytea,text,text,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.finish_payment_query(uuid,bigint,bytea,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.finish_payment_query(uuid,bigint,bytea,text,text,text) TO commerce_worker;
