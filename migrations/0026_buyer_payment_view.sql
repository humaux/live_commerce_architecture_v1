-- Read-only buyer projection, not a payment admission ticket. Keep the original
-- attempt/facts authoritative even after provider configuration changes. The
-- dedicated hosted role receives only this function, no new table privileges.
CREATE FUNCTION checkout.hosted_payment_view(p_hash bytea,p_store uuid,p_order uuid,
 p_profile text,p_config bytea) RETURNS jsonb
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; sf record; v_result jsonb; v_now timestamptz;
 v_methods_until timestamptz; v_page_until timestamptz;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
 OR p_profile IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE')
 OR p_config IS NULL OR octet_length(p_config)<>32
 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid payment view input' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true);
 PERFORM set_config('app.store_id',p_store::text,true);
 PERFORM set_config('app.buyer_id',s.owner_id::text,true);
 PERFORM set_config('app.buyer_session_id',s.session_id::text,true);
 PERFORM set_config('app.principal_id','',true);
 v_now:=clock_timestamp();

 -- One statement snapshot prevents mixing a pre-capture order with post-capture
 -- facts. Explicit ownership is required despite capture_attempt_lookup RLS.
 WITH owned AS MATERIALIZED (
  SELECT o.id,o.tenant_id,o.store_id,o.owner_id,o.creator_session_id,o.market_id,o.country,
   o.currency,o.total_minor,o.commercial_state,o.generation,o.expires_at
   FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store
   AND o.owner_id=s.owner_id AND o.id=p_order
 ), attempt AS MATERIALIZED (
  SELECT a.id,a.tenant_id,a.store_id,a.connection_id,a.execution_profile,a.environment,a.currency,a.amount_minor
   FROM checkout.payment_attempts a JOIN owned o ON a.tenant_id=o.tenant_id
   AND a.store_id=o.store_id AND a.owner_id=o.owner_id AND a.order_id=o.id
 ), page AS (
  SELECT h.config_digest,h.expires_at,h.handed_out_at,h.attempt_id
   FROM checkout.hosted_payment_pages h JOIN attempt a ON a.id=h.attempt_id
 ), candidate AS (
  SELECT m.code,m.version,m.name_hans,m.name_hant,m.name_en,
   least(o.expires_at,r.expires_at,q.expires_at) AS valid_until
  FROM owned o
  JOIN inventory.reservations r ON r.tenant_id=o.tenant_id AND r.store_id=o.store_id
   AND r.id=o.id AND r.checkout_id=o.id AND r.buyer_owner_id=o.owner_id
   AND r.buyer_session_id=o.creator_session_id AND r.generation=o.generation
  JOIN pricing.markets market ON market.tenant_id=o.tenant_id AND market.store_id=o.store_id
   AND market.id=o.market_id AND market.currency=o.currency AND market.active
  JOIN payments.method_heads head ON head.tenant_id=o.tenant_id AND head.store_id=o.store_id
   AND head.market_id=o.market_id AND head.country=o.country AND head.code='payuni_credit'
  JOIN payments.method_versions m ON m.tenant_id=head.tenant_id AND m.store_id=head.store_id
   AND m.market_id=head.market_id AND m.country=head.country AND m.code=head.code
   AND m.version=head.current_version
  JOIN integration.merchant_accounts a ON a.tenant_id=o.tenant_id AND a.store_id=o.store_id
   AND a.id=m.connection_id AND a.provider='payuni' AND a.environment=m.environment
  JOIN integration.bindings b ON b.tenant_id=o.tenant_id AND b.store_id=o.store_id
   AND b.id=a.binding_id AND b.provider=a.provider AND b.external_asset_id=a.binding_asset
   AND b.semantic_version=m.binding_version AND b.enabled
  JOIN payments.account_qualifications q ON q.tenant_id=o.tenant_id AND q.store_id=o.store_id
   AND q.id=m.qualification_id AND q.connection_id=a.id AND q.credential_version=a.credential_version
   AND q.environment=a.environment AND q.code=m.code
  WHERE NOT EXISTS(SELECT 1 FROM attempt)
   AND o.commercial_state='DRAFT' AND r.state='HELD'
   AND o.currency='TWD' AND o.total_minor BETWEEN 100 AND 19999900 AND o.total_minor%100=0
   AND m.enabled AND m.visible AND o.total_minor BETWEEN m.min_amount_minor AND m.max_amount_minor
   AND o.expires_at>v_now AND r.expires_at>v_now AND q.observed_at<=v_now AND q.expires_at>v_now
   AND q.revoked_at IS NULL
   AND q.proof_class=CASE WHEN p_profile='PROVIDER_MOCK' THEN p_profile ELSE 'REAL_'||p_profile END
   AND a.environment=CASE WHEN p_profile='PROVIDER_MOCK' THEN 'SANDBOX' ELSE p_profile END
 ), financial AS (
  SELECT f.kind FROM payments.facts f JOIN attempt a ON f.tenant_id=a.tenant_id
   AND f.store_id=a.store_id AND f.attempt_id=a.id AND f.connection_id=a.connection_id
   AND f.execution_profile=a.execution_profile AND f.environment=a.environment
   AND f.currency=a.currency AND f.amount_minor=a.amount_minor
  JOIN owned o ON o.currency=f.currency AND o.total_minor=f.amount_minor
 )
 SELECT jsonb_build_object(
  'order_id',o.id,'currency',o.currency,'total_minor',o.total_minor,'commercial_state',o.commercial_state,
  'test_mode',CASE WHEN a.id IS NULL THEN p_profile<>'LIVE'
    ELSE a.execution_profile<>'LIVE' OR a.environment<>'LIVE' END,
  'payment_state',CASE WHEN a.id IS NULL THEN 'NOT_STARTED'
    WHEN EXISTS(SELECT 1 FROM payments.review_cases rc WHERE rc.tenant_id=a.tenant_id
      AND rc.store_id=a.store_id AND rc.attempt_id=a.id) THEN 'REVIEW_REQUIRED'
    WHEN EXISTS(SELECT 1 FROM financial WHERE kind='CAPTURED') THEN 'CAPTURED'
    WHEN EXISTS(SELECT 1 FROM financial WHERE kind='AUTHORIZED') THEN 'AUTHORIZED'
    ELSE 'PENDING' END,
  'handoff_state',CASE WHEN a.execution_profile<>p_profile THEN 'UNAVAILABLE'
    WHEN h.attempt_id IS NULL THEN 'NONE' WHEN h.handed_out_at IS NOT NULL THEN 'ISSUED'
    WHEN h.config_digest<>p_config THEN 'UNAVAILABLE' WHEN h.expires_at<=v_now THEN 'EXPIRED'
    ELSE 'PREPARED' END,
  'handoff_expires_at',h.expires_at,
  'methods',CASE WHEN m.code IS NULL THEN '[]'::jsonb ELSE jsonb_build_array(jsonb_build_object(
    'code',m.code,'version',m.version,'name_hans',m.name_hans,'name_hant',m.name_hant,'name_en',m.name_en)) END
 ),m.valid_until,h.expires_at INTO v_result,v_methods_until,v_page_until
 FROM owned o LEFT JOIN attempt a ON true LEFT JOIN page h ON true LEFT JOIN candidate m ON true;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;

 SELECT * INTO sf FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND OR sf.tenant_id<>s.tenant_id OR sf.owner_id<>s.owner_id OR sf.session_id<>s.session_id THEN
  RAISE EXCEPTION 'buyer capability expired' USING ERRCODE='PT401'; END IF;
 -- A schema/capability lock can consume a short qualification window. Prune at
 -- final DB time, never extend a saved page or infer unpaid/failed from timeout.
 v_now:=clock_timestamp();
 IF v_methods_until IS NOT NULL AND v_methods_until<=v_now THEN
  v_result:=jsonb_set(v_result,'{methods}','[]'::jsonb); END IF;
 IF v_result->>'handoff_state'='PREPARED' AND v_page_until<=v_now THEN
  v_result:=jsonb_set(v_result,'{handoff_state}','"EXPIRED"'::jsonb); END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION checkout.hosted_payment_view(bytea,uuid,uuid,text,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.hosted_payment_view(bytea,uuid,uuid,text,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.hosted_payment_view(bytea,uuid,uuid,text,bytea) TO commerce_hosted_runtime;
