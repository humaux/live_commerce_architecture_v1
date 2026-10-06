-- post_river/0022 platform Stripe attribution (contracts/stripe-platform-account-v1.md §4, §5; owner decision 2026-10-06).
-- Purpose: server-side attribution of every PaymentIntent/charge/refund to exactly one tenant/store/order when many stores
--   share the platform Stripe account: the create key set gains the store tag (+ the per-store descriptor suffix), the webhook
--   prepare maps events of derived connections to the attempt's own scope, and the hosted view discloses the collector.
-- Depends on: migration 0137 (platform tables, merchant_accounts.platform_connection_id), post_river/0016 (start body),
--   0062 (webhook prepare body), 0077 (hosted view body).
-- Used by: internal/checkout (start, hosted view), internal/payments/stripewebhook (ingress), integration workers.
-- Invariants: I01 (store from the attempt row, never from metadata), I05, I06 (idempotency keys unchanged).
-- Non-goals: start/worker/refund admission are NOT rewritten; metadata is a cross-check only, never authority.
-- Same signatures, owners and ACLs as the sources except stripe_webhook_prepare (+1 trailing DEFAULT argument).

CREATE OR REPLACE FUNCTION checkout.start_stripe_payment(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,
 p_order uuid,p_method text,p_version bigint,p_profile text,p_attempt uuid,p_job bigint,
 p_locale text,p_config_digest bytea,p_return_url text) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; sf record; o checkout.orders%ROWTYPE; r inventory.reservations%ROWTYPE;
 m payments.method_versions%ROWTYPE; a integration.merchant_accounts%ROWTYPE;
 b integration.bindings%ROWTYPE; q payments.account_qualifications%ROWTYPE;
 v_version bigint; v_active boolean; v_now timestamptz; v_trade text;
 v_result jsonb; v_params jsonb; v_expiry timestamptz; v_locale text; v_suffix text;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
 OR p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
 OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
 OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_attempt IS NULL
 OR p_job IS NULL OR p_job<1 OR p_method IS DISTINCT FROM 'stripe_checkout'
 OR p_version IS NULL OR p_version<1 OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE')
 OR p_locale NOT IN ('zh-CN','zh-TW','en')
 OR p_config_digest IS NULL OR octet_length(p_config_digest)<>32
 OR p_return_url IS NULL OR octet_length(p_return_url)>2048
 OR p_return_url !~ '^https://[^[:space:]]+$' THEN
  RAISE EXCEPTION 'invalid Stripe payment input' USING ERRCODE='PT400'; END IF;
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
 SELECT x.* INTO r FROM inventory.reservations x WHERE x.tenant_id=s.tenant_id
  AND x.store_id=p_store AND x.id=p_order FOR UPDATE;
 IF NOT FOUND OR o.commercial_state<>'DRAFT' OR r.state<>'HELD' OR r.checkout_id<>o.id
  OR r.buyer_owner_id<>o.owner_id OR r.buyer_session_id<>o.creator_session_id
  OR r.generation<>o.generation THEN
  RAISE EXCEPTION 'order hold unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.active INTO v_active FROM pricing.markets x WHERE x.tenant_id=s.tenant_id
  AND x.store_id=p_store AND x.id=o.market_id AND x.currency=o.currency FOR SHARE;
 IF NOT FOUND OR NOT v_active THEN RAISE EXCEPTION 'payment market unavailable' USING ERRCODE='PT409'; END IF;
 SELECT h.current_version INTO v_version FROM payments.method_heads h WHERE h.tenant_id=s.tenant_id
  AND h.store_id=p_store AND h.market_id=o.market_id AND h.country=o.country AND h.code=p_method FOR SHARE;
 IF NOT FOUND OR v_version<>p_version THEN RAISE EXCEPTION 'payment method changed' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO m FROM payments.method_versions x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.market_id=o.market_id AND x.country=o.country AND x.code=p_method AND x.version=p_version;
 IF NOT FOUND OR NOT m.enabled OR NOT m.visible OR m.connection_id IS NULL OR m.qualification_id IS NULL
  OR m.provider<>'stripe' OR m.currency<>o.currency OR NOT payments.stripe_amount_ok(o.currency,o.total_minor)
  OR o.total_minor<m.min_amount_minor OR o.total_minor>m.max_amount_minor THEN
  RAISE EXCEPTION 'payment method unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO a FROM integration.merchant_accounts x WHERE x.tenant_id=s.tenant_id
  AND x.store_id=p_store AND x.id=m.connection_id FOR SHARE;
 IF NOT FOUND OR a.provider<>'stripe' OR a.environment<>m.environment
  OR a.environment<>(CASE WHEN p_profile='LIVE' THEN 'LIVE' ELSE 'SANDBOX' END) THEN
  RAISE EXCEPTION 'payment account unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.id=a.binding_id FOR SHARE;
 IF NOT FOUND OR NOT b.enabled OR b.semantic_version<>m.binding_version OR b.provider<>a.provider
  OR b.external_asset_id<>a.binding_asset THEN
  RAISE EXCEPTION 'payment binding unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO q FROM payments.account_qualifications x WHERE x.tenant_id=s.tenant_id
  AND x.store_id=p_store AND x.id=m.qualification_id FOR SHARE;
 IF NOT FOUND OR q.connection_id<>a.id OR q.credential_version<>a.credential_version
  OR q.environment<>a.environment OR q.code<>p_method OR q.revoked_at IS NOT NULL
  OR q.proof_class<>(CASE p_profile WHEN 'PROVIDER_MOCK' THEN 'PROVIDER_MOCK' WHEN 'SANDBOX' THEN 'REAL_SANDBOX'
   ELSE 'REAL_LIVE' END) THEN
  RAISE EXCEPTION 'payment qualification unavailable' USING ERRCODE='PT409'; END IF;
 v_now:=clock_timestamp();
 IF o.expires_at<=v_now OR r.expires_at<=v_now OR q.observed_at>v_now OR q.expires_at<=v_now THEN
  RAISE EXCEPTION 'payment admission expired' USING ERRCODE='PT409'; END IF;
 IF NOT EXISTS(SELECT 1 FROM river_payment.river_job j WHERE j.id=p_job AND j.kind='payment_query_v1'
  AND j.args=jsonb_build_object('operation_id',p_attempt::text,'version',1)) THEN
  RAISE EXCEPTION 'payment query job missing' USING ERRCODE='PT409'; END IF;
 v_trade:='P'||rtrim(translate(encode(uuid_send(p_attempt),'base64'),'+/','-_'),'=');
 v_expiry:=date_trunc('second',v_now)+interval '40 minutes';
 v_locale:=CASE p_locale WHEN 'zh-CN' THEN 'zh' ELSE p_locale END;
 v_params:=jsonb_build_object(
  'adaptive_pricing[enabled]','false','managed_payments[enabled]','false',
  'automatic_tax[enabled]','false','metadata[lc_attempt]',p_attempt::text,
  'cancel_url',p_return_url,'metadata[lc_order]',p_order::text,
  'client_reference_id',p_attempt::text,'metadata[lc_profile]',p_profile,
  'expires_at',extract(epoch FROM v_expiry)::bigint::text,'metadata[lc_v]','1',
  'line_items[0][price_data][currency]',lower(o.currency),'mode','payment',
  'line_items[0][price_data][product_data][name]','Order '||upper(substr(replace(p_order::text,'-',''),1,8)),
  'line_items[0][price_data][unit_amount]',payments.stripe_unit_amount(o.currency,o.total_minor)::text,
  'line_items[0][quantity]','1','payment_intent_data[metadata][lc_attempt]',p_attempt::text,
  'locale',v_locale,'payment_intent_data[metadata][lc_order]',p_order::text,
  'payment_method_types[0]','card','submit_type','pay',
  'success_url',p_return_url,'ui_mode','hosted_page',
  -- delta (0137/0022): the store tag is sent for EVERY attempt (one key set); it is readable context, never authority (§4.1)
  'metadata[lc_store]',p_store::text,'payment_intent_data[metadata][lc_store]',p_store::text);
 -- delta: a derived connection with an enrollment suffix gets the per-charge statement_descriptor_suffix (PF-F4)
 IF a.platform_connection_id IS NOT NULL THEN
  SELECT e.descriptor_suffix INTO v_suffix FROM payments.platform_stripe_enrollments e WHERE e.tenant_id=s.tenant_id
   AND e.store_id=p_store AND e.connection_id=a.id;
  IF v_suffix IS NOT NULL THEN
   v_params:=v_params||jsonb_build_object('payment_intent_data[statement_descriptor_suffix]',v_suffix); END IF;
 END IF;
 INSERT INTO checkout.payment_attempts(tenant_id,store_id,owner_id,id,session_id,order_id,market_id,country,
  method_code,method_version,connection_id,credential_version,qualification_id,environment,execution_profile,
  binding_id,binding_version,currency,amount_minor,merchant_trade_no,state,generation,job_id,created_at)
 VALUES(s.tenant_id,p_store,s.owner_id,p_attempt,s.session_id,p_order,o.market_id,o.country,p_method,p_version,
  a.id,a.credential_version,q.id,a.environment,p_profile,b.id,b.semantic_version,o.currency,o.total_minor,
  v_trade,'PAYMENT_PENDING',o.generation+1,p_job,v_now);
 INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,
  external_asset_id,purpose,action,semantic_key,request_hash,request,job_id,state,generation,
  actor_kind,payment_attempt_id,buyer_owner_id,buyer_session_id)
 VALUES(p_attempt,s.tenant_id,p_store,NULL,b.id,b.semantic_version,'stripe',b.external_asset_id,
  'transactional','stripe.checkout_session','payment.stripe:'||p_attempt,p_request_hash,
  jsonb_build_object('attempt_id',p_attempt::text),p_job,'UNKNOWN',1,
  'BUYER_PAYMENT_QUERY',p_attempt,s.owner_id,s.session_id);
 INSERT INTO payments.stripe_sessions(tenant_id,store_id,owner_id,attempt_id,environment,account_id,
  locale,config_digest,unit_amount,create_params,attempt_created_at,expires_at,send_deadline,handoff_cutoff)
 VALUES(s.tenant_id,p_store,s.owner_id,p_attempt,a.environment,a.account_id,
  p_locale,p_config_digest,o.total_minor,v_params,v_now,v_expiry,v_now+interval '7 minutes',
  v_expiry-interval '5 minutes');
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(s.tenant_id,p_store,p_attempt,1,'UNKNOWN','','buyer_payment_started');
 UPDATE checkout.orders SET commercial_state='AWAITING_PAYMENT',generation=o.generation+1,
  updated_at=v_now WHERE id=p_order;
 PERFORM set_config('app.buyer_session_id',o.creator_session_id::text,true);
 UPDATE inventory.reservations SET state='PAYMENT_PENDING',generation=o.generation+1
  WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_order;
 PERFORM set_config('app.buyer_session_id',s.session_id::text,true);
 INSERT INTO checkout.events(tenant_id,store_id,owner_id,order_id,session_id,generation,action,actor_kind)
 VALUES(s.tenant_id,p_store,s.owner_id,p_order,s.session_id,o.generation+1,'checkout.payment_started','BUYER');
 v_result:=jsonb_build_object('order_id',p_order::text,'attempt_id',p_attempt::text,
  'operation_id',p_attempt::text,'job_id',p_job,'generation',o.generation+1,
  'merchant_trade_no',v_trade,'currency',o.currency,'amount_minor',o.total_minor,'state','PAYMENT_PENDING');
 INSERT INTO checkout.command_results(tenant_id,store_id,owner_id,operation,idempotency_key,
  creator_session_id,request_hash,order_id,response)
 VALUES(s.tenant_id,p_store,s.owner_id,'checkout.payment.start',p_key,s.session_id,p_request_hash,p_order,v_result);
 SELECT * INTO sf FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND OR sf.tenant_id<>s.tenant_id OR sf.owner_id<>s.owner_id OR sf.session_id<>s.session_id THEN
  RAISE EXCEPTION 'buyer capability expired' USING ERRCODE='PT401'; END IF;
 v_now:=clock_timestamp();
 IF o.expires_at<=v_now OR r.expires_at<=v_now OR q.expires_at<=v_now THEN
  RAISE EXCEPTION 'payment admission expired' USING ERRCODE='PT409'; END IF;
 RETURN v_result;
END $$;

CREATE OR REPLACE FUNCTION checkout.hosted_payment_view_v2(p_hash bytea,p_store uuid,p_order uuid,
 p_profile text,p_payuni_digest bytea,p_stripe_digest bytea) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_base jsonb; scope record; ord checkout.orders%ROWTYPE;
 a checkout.payment_attempts%ROWTYPE; s payments.stripe_sessions%ROWTYPE;
 v_stripe_method jsonb; v_now timestamptz; v_handoff text;
 v_refunded bigint:=0; v_refund_pending bigint:=0; v_refund_activity boolean:=false; v_conn uuid; v_collector jsonb;
BEGIN
 IF (p_payuni_digest IS NOT NULL AND octet_length(p_payuni_digest)<>32)
  OR (p_stripe_digest IS NOT NULL AND octet_length(p_stripe_digest)<>32)
  OR (p_payuni_digest IS NULL AND p_stripe_digest IS NULL) THEN
  RAISE EXCEPTION 'invalid payment view digest' USING ERRCODE='PT400'; END IF;
 v_base:=checkout.hosted_payment_view(p_hash,p_store,p_order,p_profile,
  coalesce(p_payuni_digest,decode(repeat('00',32),'hex')));
 IF p_payuni_digest IS NULL THEN v_base:=jsonb_set(v_base,'{methods}','[]'::jsonb); END IF;
 SELECT * INTO scope FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 SELECT x.* INTO ord FROM checkout.orders x WHERE x.tenant_id=scope.tenant_id AND x.store_id=p_store
  AND x.owner_id=scope.owner_id AND x.id=p_order;
 SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.tenant_id=scope.tenant_id
  AND x.store_id=p_store AND x.owner_id=scope.owner_id AND x.order_id=p_order;
 v_now:=clock_timestamp();
 IF a.id IS NULL AND p_stripe_digest IS NOT NULL AND p_profile IN ('PROVIDER_MOCK','SANDBOX','LIVE') THEN
  SELECT jsonb_build_object('code',m.code,'version',m.version,'name_hans',m.name_hans,
   'name_hant',m.name_hant,'name_en',m.name_en) INTO v_stripe_method
  FROM payments.method_heads h
  JOIN payments.method_versions m ON m.tenant_id=h.tenant_id AND m.store_id=h.store_id
   AND m.market_id=h.market_id AND m.country=h.country AND m.code=h.code AND m.version=h.current_version
  JOIN integration.merchant_accounts acct ON acct.tenant_id=m.tenant_id AND acct.store_id=m.store_id
   AND acct.id=m.connection_id AND acct.provider='stripe'
   AND acct.environment=CASE WHEN p_profile='LIVE' THEN 'LIVE' ELSE 'SANDBOX' END
  JOIN integration.bindings b ON b.tenant_id=acct.tenant_id AND b.store_id=acct.store_id
   AND b.id=acct.binding_id AND b.provider='stripe' AND b.external_asset_id=acct.binding_asset
   AND b.semantic_version=m.binding_version AND b.enabled
  JOIN payments.account_qualifications q ON q.tenant_id=m.tenant_id AND q.store_id=m.store_id
   AND q.id=m.qualification_id AND q.connection_id=acct.id AND q.credential_version=acct.credential_version
   AND q.environment=acct.environment AND q.code='stripe_checkout'
  JOIN inventory.reservations r ON r.tenant_id=ord.tenant_id AND r.store_id=ord.store_id
   AND r.id=ord.id AND r.state='HELD' AND r.generation=ord.generation
  JOIN pricing.markets market ON market.tenant_id=ord.tenant_id AND market.store_id=ord.store_id
   AND market.id=ord.market_id AND market.currency=ord.currency AND market.active
  WHERE h.tenant_id=ord.tenant_id AND h.store_id=ord.store_id AND h.market_id=ord.market_id
   AND h.country=ord.country AND h.code='stripe_checkout' AND ord.commercial_state='DRAFT'
   AND m.enabled AND m.visible AND m.currency=ord.currency
   AND payments.stripe_amount_ok(ord.currency,ord.total_minor)
   AND ord.total_minor BETWEEN m.min_amount_minor AND m.max_amount_minor
   AND ord.expires_at>v_now AND r.expires_at>v_now
   AND q.observed_at<=v_now AND q.expires_at>v_now AND q.revoked_at IS NULL
   AND q.proof_class=CASE p_profile WHEN 'PROVIDER_MOCK' THEN 'PROVIDER_MOCK' WHEN 'SANDBOX' THEN 'REAL_SANDBOX'
    ELSE 'REAL_LIVE' END;
  IF v_stripe_method IS NOT NULL THEN
   v_base:=jsonb_set(v_base,'{methods}',(v_base->'methods')||jsonb_build_array(v_stripe_method));
  END IF;
 END IF;
 IF a.id IS NOT NULL AND a.method_code='stripe_checkout' THEN
  SELECT x.* INTO s FROM payments.stripe_sessions x WHERE x.attempt_id=a.id;
  IF s.attempt_id IS NULL THEN RAISE EXCEPTION 'Stripe session unavailable' USING ERRCODE='PT409'; END IF;
  -- stripe-refund-v1 D8/§7.2: buyer-safe refund totals; no ids, reasons or provider strings. A refund
  -- holds money unless it has a FAILED, CANCELED or REJECTED fact; SUCCEEDED counts only while not failed.
  SELECT coalesce(sum(r.amount_minor) FILTER (WHERE ok.hit AND NOT bad.hit),0),
   coalesce(sum(r.amount_minor) FILTER (WHERE NOT ok.hit AND NOT bad.hit AND NOT rej.hit),0),
   count(*)>0 INTO v_refunded,v_refund_pending,v_refund_activity
  FROM payments.stripe_refunds r
  CROSS JOIN LATERAL (SELECT EXISTS(SELECT 1 FROM payments.refund_facts f WHERE f.tenant_id=r.tenant_id
   AND f.store_id=r.store_id AND f.refund_id=r.id AND f.kind='SUCCEEDED') AS hit) ok
  CROSS JOIN LATERAL (SELECT EXISTS(SELECT 1 FROM payments.refund_facts f WHERE f.tenant_id=r.tenant_id
   AND f.store_id=r.store_id AND f.refund_id=r.id AND f.kind IN ('FAILED','CANCELED')) AS hit) bad
  CROSS JOIN LATERAL (SELECT EXISTS(SELECT 1 FROM payments.refund_facts f WHERE f.tenant_id=r.tenant_id
   AND f.store_id=r.store_id AND f.refund_id=r.id AND f.kind='REJECTED') AS hit) rej
  WHERE r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.attempt_id=a.id;
  IF EXISTS(SELECT 1 FROM payments.review_cases c WHERE c.tenant_id=a.tenant_id
   AND c.store_id=a.store_id AND c.attempt_id=a.id) THEN
   v_base:=jsonb_set(v_base,'{payment_state}','"REVIEW_REQUIRED"'::jsonb);
  ELSIF EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=a.tenant_id
   AND f.store_id=a.store_id AND f.attempt_id=a.id AND f.kind='CLOSED_UNPAID') THEN
   v_base:=jsonb_set(v_base,'{payment_state}','"CLOSED_UNPAID"'::jsonb);
  ELSIF v_refunded>0 THEN
   -- RD7: derived, never stored. Precedence REVIEW_REQUIRED > REFUNDED > PARTIALLY_REFUNDED > CAPTURED.
   v_base:=jsonb_set(v_base,'{payment_state}',
    to_jsonb(CASE WHEN v_refunded>=a.amount_minor THEN 'REFUNDED' ELSE 'PARTIALLY_REFUNDED' END));
  END IF;
  IF v_refund_activity THEN
   v_base:=jsonb_set(v_base,'{refund}',jsonb_build_object('refunded_minor',v_refunded,'pending_minor',v_refund_pending));
  END IF;
  IF EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=a.tenant_id
   AND f.store_id=a.store_id AND f.attempt_id=a.id AND f.kind IN ('CAPTURED','CLOSED_UNPAID')) THEN
   v_handoff:='CLOSED';
  ELSIF a.execution_profile<>p_profile OR p_stripe_digest IS NULL
   OR s.config_digest<>p_stripe_digest OR v_now>=s.handoff_cutoff THEN
   v_handoff:='UNAVAILABLE';
  ELSIF s.session_url IS NULL THEN v_handoff:='CREATING';
  ELSE v_handoff:='READY'; END IF;
  v_base:=jsonb_set(v_base,'{handoff_state}',to_jsonb(v_handoff));
  v_base:=jsonb_set(v_base,'{handoff_expires_at}',to_jsonb(s.handoff_cutoff));
  v_base:=jsonb_set(v_base,'{cancel_requested}',to_jsonb(s.cancel_requested_at IS NOT NULL));
 ELSE
  v_base:=jsonb_set(v_base,'{cancel_requested}','false'::jsonb);
 END IF;
 -- delta (0137/0022 §5): buyer disclosure. collector = {display_name, descriptor_preview} when the card method / attempt runs on a
 -- derived (platform) connection, JSON null for a primary connection or when no Stripe method applies. No account id, key or id.
 v_conn:=CASE WHEN a.id IS NOT NULL AND a.method_code='stripe_checkout' THEN a.connection_id
  WHEN a.id IS NULL AND p_stripe_digest IS NOT NULL THEN (SELECT m.connection_id FROM payments.method_heads h
   JOIN payments.method_versions m ON m.tenant_id=h.tenant_id AND m.store_id=h.store_id AND m.market_id=h.market_id
    AND m.country=h.country AND m.code=h.code AND m.version=h.current_version
   WHERE h.tenant_id=ord.tenant_id AND h.store_id=ord.store_id AND h.market_id=ord.market_id AND h.country=ord.country
    AND h.code='stripe_checkout' AND m.enabled) END;
 SELECT jsonb_build_object('display_name',p.display_name,'descriptor_preview',
   p.descriptor_display||CASE WHEN e.descriptor_suffix IS NOT NULL THEN '* '||e.descriptor_suffix ELSE '' END)
  INTO v_collector
  FROM integration.merchant_accounts d JOIN payments.stripe_platform p ON p.connection_id=d.platform_connection_id
  LEFT JOIN payments.platform_stripe_enrollments e ON e.tenant_id=d.tenant_id AND e.store_id=d.store_id AND e.connection_id=d.id
  WHERE d.tenant_id=ord.tenant_id AND d.store_id=ord.store_id AND d.id=v_conn AND d.platform_connection_id IS NOT NULL;
 v_base:=jsonb_set(v_base,'{collector}',coalesce(v_collector,'null'::jsonb),true);
 RETURN v_base;
END $$;

-- stripe_webhook_prepare: the 18-argument signature is replaced by the 19-argument one (trailing DEFAULT NULL metadata.lc_store).
DROP FUNCTION payments.stripe_webhook_prepare(uuid,bigint,text,text,bigint,text,text,text,text,text,boolean,boolean,boolean,boolean,bytea,bigint,text,text);
CREATE FUNCTION payments.stripe_webhook_prepare(p_endpoint uuid,p_key_version bigint,
 p_event_id text,p_event_type text,p_event_created bigint,p_api_version text,p_object_type text,
 p_session_id text,p_client_reference text,p_metadata_attempt text,p_account_present boolean,
 p_livemode boolean,p_probe boolean,p_malformed boolean,p_body_sha256 bytea,p_signed_at bigint,
 p_payment_intent text DEFAULT NULL,p_metadata_refund text DEFAULT NULL,p_metadata_store text DEFAULT NULL)
RETURNS TABLE(disposition text,receipt_id uuid,attempt_id uuid,session_id text,signal_id uuid,refund_id uuid,object_type text)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ep payments.stripe_webhook_endpoints%ROWTYPE; a checkout.payment_attempts%ROWTYPE;
 sess payments.stripe_sessions%ROWTYPE; existing payments.stripe_webhook_receipts%ROWTYPE;
 v_receipt uuid; v_signal uuid; v_disposition text; v_reason text; v_attempt uuid;
 v_tenant uuid; v_store uuid; v_session text; rf payments.stripe_refunds%ROWTYPE; v_refund uuid;
 v_conn_ok boolean;
BEGIN
 IF p_endpoint IS NULL OR p_key_version IS NULL OR p_key_version<1
  OR p_body_sha256 IS NULL OR octet_length(p_body_sha256)<>32
  OR p_signed_at IS NULL OR p_signed_at<=0 OR p_account_present IS NULL
  OR p_livemode IS NULL OR p_probe IS NULL OR p_malformed IS NULL THEN
  RAISE EXCEPTION 'invalid Stripe webhook metadata' USING ERRCODE='22023'; END IF;
 SELECT e.* INTO ep FROM payments.stripe_webhook_endpoints e WHERE e.endpoint_id=p_endpoint FOR SHARE;
 IF NOT FOUND OR NOT ep.enabled OR ep.key_version<>p_key_version THEN
  RAISE EXCEPTION 'Stripe webhook endpoint unavailable' USING ERRCODE='40001'; END IF;
 IF p_malformed THEN
  PERFORM pg_advisory_xact_lock(hashtextextended('stripe.webhook.malformed|'
   ||ep.endpoint_id||'|'||encode(p_body_sha256,'hex'),0));
  SELECT x.* INTO existing FROM payments.stripe_webhook_receipts x
   WHERE x.endpoint_id=ep.endpoint_id
    AND x.event_id IS NULL AND x.body_sha256=p_body_sha256 FOR UPDATE;
  IF FOUND THEN
   UPDATE payments.stripe_webhook_receipts SET redelivery_count=redelivery_count+1,
    last_redelivered_at=clock_timestamp() WHERE id=existing.id;
   RETURN QUERY SELECT 'DUPLICATE'::text,existing.id,existing.attempt_id,existing.session_id,existing.signal_id,
    existing.refund_id,existing.object_type;
   RETURN;
  END IF;
  v_receipt:=gen_random_uuid();
  INSERT INTO payments.stripe_webhook_receipts(id,endpoint_id,environment,account_id,event_id,body_sha256,
   signed_at,disposition,reason)
   VALUES(v_receipt,ep.endpoint_id,ep.environment,ep.account_id,NULL,p_body_sha256,p_signed_at,'MALFORMED','malformed_json');
  RETURN QUERY SELECT 'MALFORMED'::text,v_receipt,NULL::uuid,NULL::text,NULL::uuid,NULL::uuid,NULL::text;
  RETURN;
 END IF;
 IF p_event_id IS NULL OR p_event_id !~ '^[A-Za-z0-9_]{1,255}$'
  OR p_event_type IS NULL OR p_event_type !~ '^[a-z0-9_.]{1,100}$'
  OR p_session_id IS NULL OR (p_session_id<>'' AND p_session_id !~ '^[A-Za-z0-9_]{1,255}$')
  OR p_event_created IS NULL OR p_event_created<=0 THEN
  RAISE EXCEPTION 'invalid Stripe webhook projection' USING ERRCODE='22023'; END IF;
 IF (p_payment_intent IS NOT NULL AND p_payment_intent<>'' AND p_payment_intent !~ '^[A-Za-z0-9_]{1,255}$')
  OR (p_metadata_refund IS NOT NULL AND octet_length(p_metadata_refund)>64) THEN
  RAISE EXCEPTION 'invalid Stripe webhook refund projection' USING ERRCODE='22023'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('stripe.webhook.event|'
  ||ep.endpoint_id||'|'||p_event_id,0));
 SELECT x.* INTO existing FROM payments.stripe_webhook_receipts x
  WHERE x.endpoint_id=ep.endpoint_id AND x.event_id=p_event_id FOR UPDATE;
 IF FOUND THEN
  UPDATE payments.stripe_webhook_receipts SET redelivery_count=redelivery_count+1,
   last_redelivered_at=clock_timestamp() WHERE id=existing.id;
  RETURN QUERY SELECT 'DUPLICATE'::text,existing.id,existing.attempt_id,existing.session_id,existing.signal_id,
    existing.refund_id,existing.object_type;
  RETURN;
 END IF;
 v_disposition:='QUARANTINED'; v_reason:='unknown_session';
 IF p_account_present THEN v_reason:='connect_event';
 ELSIF p_livemode<>(ep.environment='LIVE') THEN v_reason:='livemode_mismatch';
 ELSIF p_event_type NOT IN ('checkout.session.completed','checkout.session.async_payment_succeeded',
  'checkout.session.async_payment_failed','checkout.session.expired',
  'refund.created','refund.updated','refund.failed','charge.refunded') THEN
  v_disposition:='IGNORED'; v_reason:='unsubscribed_type';
 ELSIF p_probe THEN v_disposition:='IGNORED'; v_reason:='probe_session';
 ELSIF (p_event_type LIKE 'checkout.session.%' AND p_object_type<>'checkout.session')
  OR (p_event_type LIKE 'refund.%' AND p_object_type<>'refund')
  OR (p_event_type='charge.refunded' AND p_object_type<>'charge') THEN v_reason:='object_mismatch';
 ELSIF p_event_type LIKE 'refund.%' THEN
  -- stripe-refund-v1 §7.3: a refund object maps by its pinned id, else by metadata.lc_refund of an
  -- unpinned refund whose lc_attempt also matches, always scoped to this endpoint's account.
  v_disposition:='IGNORED'; v_reason:='unknown_refund';
  SELECT x.* INTO rf FROM payments.stripe_refunds x WHERE x.stripe_refund_id=p_session_id
   AND x.account_id=ep.account_id AND x.environment=ep.environment;
  IF rf.id IS NULL
   AND p_metadata_refund ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
   AND p_metadata_attempt ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
   SELECT x.* INTO rf FROM payments.stripe_refunds x WHERE x.id=p_metadata_refund::uuid
    AND x.attempt_id=p_metadata_attempt::uuid AND x.stripe_refund_id IS NULL
    AND x.account_id=ep.account_id AND x.environment=ep.environment;
  END IF;
  IF rf.id IS NOT NULL THEN
   SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=rf.attempt_id
    AND x.tenant_id=rf.tenant_id AND x.store_id=rf.store_id;
   -- delta (0137/0022 §4.2): the attempt's connection is the endpoint's OR a derived connection pointing at it
   v_conn_ok:=a.id IS NOT NULL AND (a.connection_id=ep.connection_id OR EXISTS(SELECT 1 FROM integration.merchant_accounts d
    WHERE d.id=a.connection_id AND d.platform_connection_id=ep.connection_id));
   IF a.id IS NULL OR a.execution_profile<>ep.execution_profile
    OR NOT v_conn_ok OR a.method_code<>'stripe_checkout' THEN
    v_disposition:='QUARANTINED'; v_reason:='profile_mismatch';
   ELSE
    SELECT x.* INTO rf FROM payments.stripe_refunds x WHERE x.id=rf.id FOR UPDATE;
    IF rf.signal_count>=64 THEN v_disposition:='IGNORED'; v_reason:='signal_cap';
    ELSE
     v_disposition:='ACCEPTED'; v_reason:='accepted'; v_attempt:=rf.attempt_id; v_refund:=rf.id;
     v_tenant:=rf.tenant_id; v_store:=rf.store_id; v_session:=p_session_id;
     v_signal:=gen_random_uuid();
    END IF;
   END IF;
  END IF;
 ELSIF p_event_type='charge.refunded' THEN
  -- A charge object maps by payment_intent to a pinned, captured Stripe attempt of this account.
  v_disposition:='IGNORED'; v_reason:='unknown_charge';
  IF p_payment_intent IS NOT NULL AND p_payment_intent<>'' THEN
   SELECT x.* INTO sess FROM payments.stripe_sessions x WHERE x.payment_intent_id=p_payment_intent
    AND x.account_id=ep.account_id AND x.environment=ep.environment
    AND EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=x.tenant_id AND f.store_id=x.store_id
     AND f.attempt_id=x.attempt_id AND f.kind='CAPTURED');
  END IF;
  IF sess.attempt_id IS NOT NULL THEN
   SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=sess.attempt_id
    AND x.tenant_id=sess.tenant_id AND x.store_id=sess.store_id;
   -- delta (0137/0022 §4.2): the attempt's connection is the endpoint's OR a derived connection pointing at it
   v_conn_ok:=a.id IS NOT NULL AND (a.connection_id=ep.connection_id OR EXISTS(SELECT 1 FROM integration.merchant_accounts d
    WHERE d.id=a.connection_id AND d.platform_connection_id=ep.connection_id));
   IF a.id IS NULL OR a.execution_profile<>ep.execution_profile
    OR NOT v_conn_ok OR a.method_code<>'stripe_checkout' THEN
    v_disposition:='QUARANTINED'; v_reason:='profile_mismatch';
   ELSIF p_metadata_store IS NOT NULL AND p_metadata_store<>'' AND p_metadata_store<>a.store_id::text THEN
    v_disposition:='QUARANTINED'; v_reason:='reference_mismatch'; -- delta: a crossed lc_store never selects a row
   ELSE
    SELECT x.* INTO sess FROM payments.stripe_sessions x WHERE x.attempt_id=a.id FOR UPDATE;
    IF sess.charge_signal_count>=64 THEN v_disposition:='IGNORED'; v_reason:='signal_cap';
    ELSE
     v_disposition:='ACCEPTED'; v_reason:='accepted'; v_attempt:=a.id;
     v_tenant:=a.tenant_id; v_store:=a.store_id; v_session:=p_session_id;
     v_signal:=gen_random_uuid();
    END IF;
   END IF;
  END IF;
 ELSE
  SELECT x.* INTO sess FROM payments.stripe_sessions x
   WHERE x.session_id=p_session_id AND x.account_id=ep.account_id AND x.environment=ep.environment;
  IF NOT FOUND AND p_client_reference=p_metadata_attempt
   AND p_client_reference ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN
   SELECT x.* INTO sess FROM payments.stripe_sessions x
    WHERE x.attempt_id=p_client_reference::uuid AND x.account_id=ep.account_id
     AND x.environment=ep.environment;
  END IF;
  IF sess.attempt_id IS NOT NULL THEN
   SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=sess.attempt_id
    AND x.tenant_id=sess.tenant_id AND x.store_id=sess.store_id;
   -- delta (0137/0022 §4.2): the attempt's connection is the endpoint's OR a derived connection pointing at it
   v_conn_ok:=a.id IS NOT NULL AND (a.connection_id=ep.connection_id OR EXISTS(SELECT 1 FROM integration.merchant_accounts d
    WHERE d.id=a.connection_id AND d.platform_connection_id=ep.connection_id));
   IF a.id IS NULL OR a.execution_profile<>ep.execution_profile
    OR NOT v_conn_ok OR a.method_code<>'stripe_checkout' THEN
    v_reason:='profile_mismatch';
   ELSIF p_client_reference IS DISTINCT FROM a.id::text
    OR p_metadata_attempt IS DISTINCT FROM a.id::text
    -- delta: a present lc_store that is not the attempt's store is a forged or crossed reference (PF05); absent = pre-amendment session
    OR (p_metadata_store IS NOT NULL AND p_metadata_store<>'' AND p_metadata_store<>a.store_id::text) THEN
    v_reason:='reference_mismatch';
   ELSE
    SELECT x.* INTO sess FROM payments.stripe_sessions x WHERE x.attempt_id=a.id FOR UPDATE;
    IF sess.signal_count>=64 THEN v_disposition:='IGNORED'; v_reason:='signal_cap';
    ELSE
     v_disposition:='ACCEPTED'; v_reason:='accepted'; v_attempt:=a.id;
     v_tenant:=a.tenant_id; v_store:=a.store_id; v_session:=p_session_id;
     v_signal:=gen_random_uuid();
    END IF;
   END IF;
  ELSIF p_client_reference IS DISTINCT FROM p_metadata_attempt THEN
   v_reason:='reference_mismatch';
  END IF;
 END IF;
 v_receipt:=gen_random_uuid();
 INSERT INTO payments.stripe_webhook_receipts(id,endpoint_id,environment,account_id,event_id,event_type,
  event_created,api_version,object_type,session_id,body_sha256,signed_at,disposition,reason,
  attempt_id,tenant_id,store_id,signal_id,refund_id)
 VALUES(v_receipt,ep.endpoint_id,ep.environment,ep.account_id,p_event_id,p_event_type,p_event_created,
  p_api_version,p_object_type,nullif(p_session_id,''),p_body_sha256,p_signed_at,
  v_disposition,v_reason,v_attempt,v_tenant,v_store,v_signal,v_refund);
 RETURN QUERY SELECT CASE WHEN v_disposition='ACCEPTED' THEN 'ACCEPT_PENDING' ELSE v_disposition END,
  v_receipt,v_attempt,v_session,v_signal,v_refund,p_object_type;
END $$;
ALTER FUNCTION payments.stripe_webhook_prepare(uuid,bigint,text,text,bigint,text,text,text,text,text,boolean,boolean,boolean,boolean,bytea,bigint,text,text,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION payments.stripe_webhook_prepare(uuid,bigint,text,text,bigint,text,text,text,text,text,boolean,boolean,boolean,boolean,bytea,bigint,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.stripe_webhook_prepare(uuid,bigint,text,text,bigint,text,text,text,text,text,boolean,boolean,boolean,boolean,bytea,bigint,text,text,text) TO commerce_stripe_ingress;
COMMENT ON FUNCTION payments.stripe_webhook_prepare(uuid,bigint,text,text,bigint,text,text,text,text,text,boolean,boolean,boolean,boolean,bytea,bigint,text,text,text) IS 'payments owner; signed ingress metadata dedupe and scoped signal preallocation for checkout, refund and charge objects. One endpoint serves the platform connection and every derived store connection: the receipt scope is the ATTEMPT row scope, metadata.lc_store only cross-checks (reference_mismatch), never selects a row; no body retention';
COMMENT ON FUNCTION checkout.start_stripe_payment(bytea,uuid,text,bytea,uuid,text,bigint,text,uuid,bigint,text,bytea,text) IS 'checkout owner; buyer payment start for Stripe: server-priced, per-store attempt on the store own (possibly derived) connection; create params carry metadata lc_store and the enrollment descriptor suffix (contract stripe-platform-account-v1 §4.1); no PSP call';
