-- Purpose: live-session flow (R5 A5): unified session attribution (claims.session_orders / claims.order_session_counts and the
--   CREATE OR REPLACE of claims.order_live_sources to the same union), the session-results read model
--   (identity.read_live_session_results: orders + money per session), and the Page "live videos" picker read
--   (live.page_live_video_snapshots + integration.plan/check/load/finish_meta_live_videos + live.read_page_live_videos).
-- Depends on: claims.order_origins (0113), claims.live_price_uses (0105), claims.bundles (0060), live.sessions (0033),
--   identity.resolve_access (0003), identity.principal_holds (0064), identity.merchant_access_denied (0063),
--   live.order_session_labels (0110), payments.facts (0027), payments.refund_facts / payments.stripe_refunds (0062),
--   checkout.orders (0027/0073/0107), checkout.payment_attempts (0027), checkout.bank_transfers (0088),
--   fulfillment.cvs_shipments (0073/0085), integration.bindings/operations/operation_events (0008),
--   integration.meta_page_heads/meta_page_credentials (0064), river.river_job (0018); the 0113 plan/check/load/finish pattern.
-- Used by: internal/live/results.go (read_live_session_results), internal/httpapi/live_flow.go (read_page_live_videos),
--   internal/integrations/metareply/live_videos.go (plan/check/load/finish), orders-v2 session filter (order_live_sources).

-- ---------------------------------------------------------------------------------------
-- A5-1 unified attribution. A session "owns" an order through EITHER the live-price-use
-- chain (live_price_uses -> bundles.session_id, 0110) OR the price-neutral consumed origin
-- (claims.order_origins, 0113). DISTINCT absorbs an order hitting one session via both
-- paths. Domain-owned definers; commerce_auth (the only consumer of attribution) gets
-- EXECUTE, never table privileges (0110 ACL ruling).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.session_orders(p_tenant uuid,p_store uuid,p_sessions uuid[])
RETURNS TABLE(session_id uuid,order_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT x.session_id,x.order_id FROM (
  SELECT b.session_id,u.order_id
  FROM claims.live_price_uses u JOIN claims.bundles b ON b.tenant_id=u.tenant_id AND b.store_id=u.store_id AND b.id=u.bundle_id
  WHERE u.tenant_id=p_tenant AND u.store_id=p_store AND b.tenant_id=p_tenant AND b.store_id=p_store
  UNION
  SELECT o.session_id,o.order_id FROM claims.order_origins o WHERE o.tenant_id=p_tenant AND o.store_id=p_store
 ) x
 WHERE p_sessions IS NULL OR x.session_id=ANY(p_sessions)
$$;
ALTER FUNCTION claims.session_orders(uuid,uuid,uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.session_orders(uuid,uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.session_orders(uuid,uuid,uuid[]) TO commerce_auth;
COMMENT ON FUNCTION claims.session_orders(uuid,uuid,uuid[]) IS
 'internal/claims (SQL definer; only caller is identity.read_live_session_results on commerce_auth); EXECUTE: commerce_auth. Distinct (session,order) attribution from live_price_uses⋈bundles ∪ order_origins; p_sessions NULL = all store sessions. No buyer identity, actor or bundle-owner data.';

CREATE FUNCTION claims.order_session_counts(p_tenant uuid,p_store uuid,p_orders uuid[])
RETURNS TABLE(order_id uuid,sessions bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT x.order_id,count(DISTINCT x.session_id)::bigint
 FROM (
  SELECT b.session_id,u.order_id
  FROM claims.live_price_uses u JOIN claims.bundles b ON b.tenant_id=u.tenant_id AND b.store_id=u.store_id AND b.id=u.bundle_id
  WHERE u.tenant_id=p_tenant AND u.store_id=p_store AND b.tenant_id=p_tenant AND b.store_id=p_store
  UNION
  SELECT o.session_id,o.order_id FROM claims.order_origins o WHERE o.tenant_id=p_tenant AND o.store_id=p_store
 ) x
 WHERE p_orders IS NULL OR x.order_id=ANY(p_orders)
 GROUP BY x.order_id
$$;
ALTER FUNCTION claims.order_session_counts(uuid,uuid,uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.order_session_counts(uuid,uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.order_session_counts(uuid,uuid,uuid[]) TO commerce_auth;
COMMENT ON FUNCTION claims.order_session_counts(uuid,uuid,uuid[]) IS
 'internal/claims (SQL definer; only caller is identity.read_live_session_results on commerce_auth); EXECUTE: commerce_auth. Number of distinct sessions each order belongs to, bounded to the requested orders so a results read never scans the whole store. Feeds multi_session_orders.';

-- orders-v2 session filter must agree with the results read (R2): same union.
CREATE OR REPLACE FUNCTION claims.order_live_sources(p_tenant uuid,p_store uuid,p_orders uuid[])
RETURNS TABLE(order_id uuid,session_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT x.order_id,x.session_id FROM (
  SELECT b.session_id,u.order_id
  FROM claims.live_price_uses u JOIN claims.bundles b ON b.tenant_id=u.tenant_id AND b.store_id=u.store_id AND b.id=u.bundle_id
  WHERE u.tenant_id=p_tenant AND u.store_id=p_store AND b.tenant_id=p_tenant AND b.store_id=p_store
  UNION
  SELECT o.session_id,o.order_id FROM claims.order_origins o WHERE o.tenant_id=p_tenant AND o.store_id=p_store
 ) x
 WHERE x.order_id=ANY(p_orders)
$$;
COMMENT ON FUNCTION claims.order_live_sources(uuid,uuid,uuid[]) IS
 'internal/claims: commerce_auth-only scoped order provenance (0110 ACL ruling). 0114: union with claims.order_origins so orders-v2 session filtering counts price-neutral claimed orders too and agrees with read_live_session_results.';

-- ---------------------------------------------------------------------------------------
-- A5-1 read model. owner commerce_auth; runtime EXECUTE only; no new table/column grants.
-- money口径 = 0107 read_finance_summary (I05): card CAPTURED - refunded per environment,
-- pay_at_pickup COLLECTED total_minor (env = latest cvs_shipments else LIVE), cash_on_delivery
-- COLLECTED total_minor+cod_surcharge_minor (LIVE), bank_transfer CONFIRMED confirmed_amount_minor
-- (LIVE). Per-environment, never summed across environments or currencies.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION identity.read_live_session_results(p_hash bytea,p_store uuid,p_sessions uuid[]) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET jit=off SET plan_cache_mode=force_custom_plan AS $$
DECLARE s record; v_result jsonb; v_auth_error text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
  OR p_sessions IS NULL OR array_ndims(p_sessions)<>1 OR cardinality(p_sessions) NOT BETWEEN 1 AND 50
  OR EXISTS (SELECT 1 FROM unnest(p_sessions) x WHERE x IS NULL)
  OR (SELECT count(DISTINCT x) FROM unnest(p_sessions) x)<>cardinality(p_sessions)
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid live session results' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF NOT identity.principal_holds(s.tenant_id,p_store,s.principal_id,ARRAY['live:read']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- I01: every requested session must be this store's live session (domain definer; commerce_auth has no live.sessions grant).
 IF (SELECT count(*) FROM live.order_session_labels(s.tenant_id,p_store,p_sessions))<>cardinality(p_sessions) THEN
  RAISE EXCEPTION 'session not found' USING ERRCODE='PT404'; END IF;
 WITH ord AS MATERIALIZED (
  SELECT so.session_id,so.order_id FROM claims.session_orders(s.tenant_id,p_store,p_sessions) so
 ), multi AS MATERIALIZED (
  SELECT osc.order_id,osc.sessions FROM claims.order_session_counts(s.tenant_id,p_store,
   ARRAY(SELECT order_id FROM ord)) osc
 ), cap AS MATERIALIZED (
  SELECT a.order_id,f.environment,sum(f.amount_minor)::bigint AS minor
  FROM payments.facts f JOIN checkout.payment_attempts a ON a.tenant_id=f.tenant_id AND a.store_id=f.store_id AND a.id=f.attempt_id
  WHERE f.tenant_id=s.tenant_id AND f.store_id=p_store AND f.kind='CAPTURED'
   AND a.order_id IN (SELECT order_id FROM ord)
  GROUP BY a.order_id,f.environment
 ), ref AS MATERIALIZED (
  SELECT a.order_id,a.environment,sum(r.amount_minor)::bigint AS minor
  FROM payments.refund_facts rf
  JOIN payments.stripe_refunds r ON r.tenant_id=rf.tenant_id AND r.store_id=rf.store_id AND r.id=rf.refund_id
  JOIN checkout.payment_attempts a ON a.tenant_id=r.tenant_id AND a.store_id=r.store_id AND a.id=r.attempt_id
  WHERE rf.tenant_id=s.tenant_id AND rf.store_id=p_store AND rf.kind='SUCCEEDED'
   AND a.order_id IN (SELECT order_id FROM ord)
   AND NOT EXISTS(SELECT 1 FROM payments.refund_facts later WHERE later.tenant_id=rf.tenant_id AND later.store_id=rf.store_id
     AND later.refund_id=rf.refund_id AND later.kind IN ('FAILED','CANCELED') AND later.received_at>rf.received_at)
  GROUP BY a.order_id,a.environment
 ), pick AS MATERIALIZED (
  SELECT o.id AS order_id,coalesce(c.environment,'LIVE') AS environment,sum(o.total_minor)::bigint AS minor
  FROM checkout.orders o
  LEFT JOIN LATERAL (SELECT x.environment FROM fulfillment.cvs_shipments x
   WHERE x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.order_id=o.id ORDER BY x.attempt DESC LIMIT 1) c ON true
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.payment_mode='pay_at_pickup' AND o.collection_state='COLLECTED'
   AND o.id IN (SELECT order_id FROM ord)
  GROUP BY o.id,c.environment
 ), cod AS MATERIALIZED (
  SELECT o.id AS order_id,'LIVE'::text AS environment,sum(o.total_minor+coalesce(o.cod_surcharge_minor,0))::bigint AS minor
  FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.payment_mode='cash_on_delivery' AND o.collection_state='COLLECTED'
   AND o.id IN (SELECT order_id FROM ord)
  GROUP BY o.id
 ), trf AS MATERIALIZED (
  SELECT t.order_id,'LIVE'::text AS environment,sum(t.confirmed_amount_minor)::bigint AS minor
  FROM checkout.bank_transfers t
  WHERE t.tenant_id=s.tenant_id AND t.store_id=p_store AND t.state='CONFIRMED'
   AND t.order_id IN (SELECT order_id FROM ord)
  GROUP BY t.order_id
 ), paid AS MATERIALIZED (
  SELECT x.order_id,x.environment,sum(x.minor)::bigint AS minor FROM (
   SELECT ce.order_id,ce.environment,coalesce(c.minor,0)-coalesce(r.minor,0) AS minor
   FROM (SELECT order_id,environment FROM cap UNION SELECT order_id,environment FROM ref) ce
   LEFT JOIN cap c ON c.order_id=ce.order_id AND c.environment=ce.environment
   LEFT JOIN ref r ON r.order_id=ce.order_id AND r.environment=ce.environment
   UNION ALL SELECT order_id,environment,minor FROM pick
   UNION ALL SELECT order_id,environment,minor FROM cod
   UNION ALL SELECT order_id,environment,minor FROM trf
  ) x GROUP BY x.order_id,x.environment
 ), ord_paid AS MATERIALIZED (
  SELECT o.id AS order_id,o.currency,o.commercial_state,o.total_minor,o.cod_surcharge_minor,
   coalesce(pl.minor,0) AS live_minor,coalesce(ps.minor,0) AS sandbox_minor,
   (m.sessions IS NOT NULL AND m.sessions>=2) AS multi
  FROM checkout.orders o
  LEFT JOIN paid pl ON pl.order_id=o.id AND pl.environment='LIVE'
  LEFT JOIN paid ps ON ps.order_id=o.id AND ps.environment='SANDBOX'
  LEFT JOIN multi m ON m.order_id=o.id
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id IN (SELECT order_id FROM ord)
 ), per_session AS MATERIALIZED (
  SELECT so.session_id,op.currency,
   count(*) FILTER (WHERE op.commercial_state<>'CANCELLED') AS orders,
   coalesce(sum(op.total_minor+coalesce(op.cod_surcharge_minor,0)) FILTER (WHERE op.commercial_state<>'CANCELLED'),0)::bigint AS order_minor,
   coalesce(sum(op.live_minor),0)::bigint AS paid_minor,
   coalesce(sum(op.sandbox_minor),0)::bigint AS sandbox_paid_minor,
   count(*) FILTER (WHERE op.live_minor+op.sandbox_minor>0) AS paid_orders,
   count(*) FILTER (WHERE op.multi) AS multi_session_orders
  FROM ord so JOIN ord_paid op ON op.order_id=so.order_id
  GROUP BY so.session_id,op.currency
 ), agg AS MATERIALIZED (
  SELECT ps.session_id,sum(ps.orders)::bigint AS orders,sum(ps.paid_orders)::bigint AS paid_orders,
   sum(ps.multi_session_orders)::bigint AS multi_session_orders,
   coalesce(jsonb_agg(jsonb_build_object('currency',ps.currency,'order_minor',ps.order_minor,'paid_minor',ps.paid_minor,
    'sandbox_paid_minor',ps.sandbox_paid_minor) ORDER BY ps.currency),'[]'::jsonb) AS money
  FROM per_session ps GROUP BY ps.session_id
 )
 SELECT jsonb_build_object('as_of',clock_timestamp(),'items',
  coalesce(jsonb_agg(jsonb_build_object('session_id',s2.session_id,'orders',coalesce(ag.orders,0),
   'paid_orders',coalesce(ag.paid_orders,0),'multi_session_orders',coalesce(ag.multi_session_orders,0),
   'money',coalesce(ag.money,'[]'::jsonb)) ORDER BY array_position(p_sessions,s2.session_id)),'[]'::jsonb))
 INTO v_result
 FROM live.order_session_labels(s.tenant_id,p_store,p_sessions) s2
 LEFT JOIN agg ag ON ag.session_id=s2.session_id;
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['orders:read','live:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'live session results access denied' USING ERRCODE=v_auth_error; END IF;
 IF octet_length(v_result::text)>1048576 THEN RAISE EXCEPTION 'live session results unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION identity.read_live_session_results(bytea,uuid,uuid[]) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_live_session_results(bytea,uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_live_session_results(bytea,uuid,uuid[]) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_live_session_results(bytea,uuid,uuid[]) IS
 'internal/live Results read (M14): fresh orders:read auth + live:read principal check, per-session non-CANCELLED order count and order_minor (total+cod surcharge), per-order per-environment paid amounts split into paid_minor (LIVE) / sandbox_paid_minor (SANDBOX), paid_orders (any entry >0) and multi_session_orders (order spans >=2 sessions). jit=off/plan_cache_mode=force_custom_plan (0110/0113 lesson). No buyer identity, no cross-currency/environment sum (I05).';

-- ---------------------------------------------------------------------------------------
-- A5-3 Page live-video picker. Snapshot keyed (store,binding); only the lease-fenced
-- finish writes. Read goes through the definer (commerce_runtime EXECUTE); no runtime
-- table grant.
-- ---------------------------------------------------------------------------------------
CREATE TABLE live.page_live_video_snapshots (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,binding_id uuid NOT NULL,
 operation_id uuid NOT NULL REFERENCES integration.operations(id),
 requested_at timestamptz NOT NULL,fetched_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 state text NOT NULL CHECK(state IN ('succeeded','failed')),
 code text CHECK(code IS NULL OR code ~ '^[a-z][a-z0-9_]{0,63}$'),
 items jsonb NOT NULL CHECK(jsonb_typeof(items)='array' AND jsonb_array_length(items)<=25),
 PRIMARY KEY(tenant_id,store_id,binding_id),
 FOREIGN KEY(tenant_id,store_id,binding_id) REFERENCES integration.bindings(tenant_id,store_id,id));
ALTER TABLE live.page_live_video_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.page_live_video_snapshots FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.page_live_video_snapshots FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE ON live.page_live_video_snapshots TO commerce_integration_writer;
CREATE POLICY lv_snapshot_owner ON live.page_live_video_snapshots TO commerce_integration_writer USING(true) WITH CHECK(true);
COMMENT ON TABLE live.page_live_video_snapshots IS
 'internal/integrations/metareply (0114, A5-3): latest bounded (<=25) live-video list for a bound Page, written only by integration.finish_meta_live_videos on SUCCEEDED/FAILED_FINAL; read only by live.read_page_live_videos. FORCE RLS; no runtime role holds a table grant. Items carry video_id/post_id/title/status/started_at only, never a token or buyer data.';
COMMENT ON COLUMN live.page_live_video_snapshots.operation_id IS 'The operation whose completion wrote this snapshot (audit trail, never a job handle).';
COMMENT ON COLUMN live.page_live_video_snapshots.requested_at IS 'The operation.created_at of the completed read; a stale overlapping read cannot overwrite a newer result.';
COMMENT ON COLUMN live.page_live_video_snapshots.state IS 'Terminal result: succeeded (items may be empty) or failed (code carries the denial).';
COMMENT ON COLUMN live.page_live_video_snapshots.items IS 'Normalized live videos, LIVE first, <=25; no provider pagination tokens, no viewer/buyer data.';

-- Only the plan INSERT path may create a meta.live_videos operation; same defaults as audience_plan_insert (0113).
CREATE POLICY live_videos_plan_insert ON integration.operations FOR INSERT TO commerce_integration_writer
 WITH CHECK (state='READY' AND generation=0 AND actor_kind='MERCHANT' AND provider='facebook'
  AND purpose='service' AND action='meta.live_videos');

CREATE FUNCTION integration.plan_meta_live_videos(p_hash bytea,p_store uuid,p_binding uuid,p_operation uuid,p_job bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;b record;j record;req jsonb;prior integration.operations;
BEGIN
 SELECT * INTO a FROM identity.resolve_access(p_hash,p_store,'integration:execute');
 IF a.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF a.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF a.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM a.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM a.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF NOT identity.principal_holds(a.tenant_id,p_store,a.principal_id,ARRAY['live:manage','integration:execute']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT * INTO b FROM integration.bindings x WHERE x.tenant_id=a.tenant_id AND x.store_id=p_store AND x.id=p_binding FOR SHARE;
 IF NOT FOUND OR NOT b.enabled OR b.provider<>'facebook' THEN RAISE EXCEPTION 'binding not found' USING ERRCODE='PT404'; END IF;
 IF NOT EXISTS(SELECT 1 FROM integration.meta_page_heads h JOIN integration.meta_page_credentials c
  ON c.tenant_id=h.tenant_id AND c.store_id=h.store_id AND c.binding_id=h.binding_id AND c.version=h.current_version
  WHERE h.tenant_id=a.tenant_id AND h.store_id=p_store AND h.binding_id=b.id
   AND c.scopes_attested @> ARRAY['pages_read_engagement']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- I23: serialize by binding so at most one in-flight read per Page.
 PERFORM pg_advisory_xact_lock(hashtextextended(a.tenant_id::text||'/'||p_store::text||'/live_videos/'||p_binding::text,0));
 SELECT * INTO prior FROM integration.operations x WHERE x.tenant_id=a.tenant_id AND x.store_id=p_store
  AND x.action='meta.live_videos' AND x.provider='facebook' AND x.request->>'binding_id'=p_binding::text
  AND (
   (x.state='READY' AND EXISTS(SELECT 1 FROM river.river_job queued_job WHERE queued_job.id=x.job_id
    AND queued_job.kind='external_operation_v1' AND queued_job.args=jsonb_build_object('operation_id',x.id::text,'version',1)
    AND queued_job.state IN ('available','scheduled','retryable','pending','running')))
   OR (x.state='DISPATCHING' AND x.lease_until>clock_timestamp())
   OR (x.state NOT IN ('READY','DISPATCHING') AND x.updated_at>=clock_timestamp()-interval '10 minutes'))
  ORDER BY (x.state IN ('READY','DISPATCHING')) DESC,x.updated_at DESC,x.id LIMIT 1;
 IF FOUND THEN
  IF p_operation IS NOT NULL OR p_job IS NOT NULL THEN RAISE EXCEPTION 'live videos replay requires preflight' USING ERRCODE='40001'; END IF;
  RETURN jsonb_build_object('operation_id',prior.id,'state',prior.state);
 END IF;
 IF p_operation IS NULL AND p_job IS NULL THEN RETURN jsonb_build_object('plan_required',true); END IF;
 SELECT * INTO j FROM river.river_job x WHERE x.id=p_job AND x.kind='external_operation_v1' AND x.queue='default'
  AND x.args=jsonb_build_object('operation_id',p_operation::text,'version',1) AND x.xmin=pg_current_xact_id()::xid;
 IF NOT FOUND THEN RAISE EXCEPTION 'live videos job mismatch' USING ERRCODE='22023'; END IF;
 req:=jsonb_build_object('v',1,'binding_id',p_binding,'asset_id',b.external_asset_id);
 INSERT INTO integration.operations(tenant_id,store_id,id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id)
 VALUES(a.tenant_id,p_store,p_operation,a.principal_id,b.id,b.semantic_version,'facebook',b.external_asset_id,'service','meta.live_videos',
  'live_videos:'||p_operation,sha256(convert_to(req::text,'UTF8')),req,p_job);
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(a.tenant_id,p_store,p_operation,0,'READY','','operation_planned');
 RETURN jsonb_build_object('operation_id',p_operation,'state','READY');
END $$;
ALTER FUNCTION integration.plan_meta_live_videos(bytea,uuid,uuid,uuid,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.plan_meta_live_videos(bytea,uuid,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.plan_meta_live_videos(bytea,uuid,uuid,uuid,bigint) TO commerce_runtime;
COMMENT ON FUNCTION integration.plan_meta_live_videos(bytea,uuid,uuid,uuid,bigint) IS
 'internal/integrations/metareply (A5-3) plan: fresh live:manage+integration:execute auth, one enabled facebook binding, attested pages_read_engagement, per-binding in-flight dedup, default-lane same-transaction job. Read-only Graph list; no Meta mutation, no buyer fields.';
CREATE INDEX operations_live_videos_binding ON integration.operations(tenant_id,store_id,(request->>'binding_id'),updated_at DESC)
 WHERE provider='facebook' AND action='meta.live_videos';

CREATE FUNCTION integration.check_meta_live_videos(p_operation uuid) RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations;
BEGIN
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation AND x.provider='facebook' AND x.action='meta.live_videos' AND x.purpose='service' AND x.actor_kind='MERCHANT';
 IF NOT FOUND THEN RETURN 'invalid_request'; END IF;
 IF NOT identity.principal_holds(o.tenant_id,o.store_id,o.principal_id,ARRAY['live:manage','integration:execute']) THEN RETURN 'principal_revoked'; END IF;
 IF NOT EXISTS(SELECT 1 FROM integration.bindings b WHERE b.tenant_id=o.tenant_id AND b.store_id=o.store_id AND b.id=o.binding_id
  AND b.enabled AND b.provider='facebook' AND b.external_asset_id=o.external_asset_id AND b.id::text=o.request->>'binding_id')
  THEN RETURN 'binding_changed'; END IF;
 RETURN '';
END $$;
ALTER FUNCTION integration.check_meta_live_videos(uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.check_meta_live_videos(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.check_meta_live_videos(uuid) TO commerce_claims_worker;
COMMENT ON FUNCTION integration.check_meta_live_videos(uuid) IS
 'internal/integrations/metareply (A5-3) Check: principal and exact bound Page recheck before the live-video Graph GET; claims worker only.';

CREATE FUNCTION integration.load_meta_live_videos_token(p_operation uuid,p_generation bigint,p_token bytea)
RETURNS TABLE(tenant_id uuid,store_id uuid,binding_id uuid,provider text,asset_id text,version bigint,key_id text,nonce bytea,ciphertext bytea,scopes_attested text[])
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations;b integration.bindings;
BEGIN
 IF p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32 THEN RAISE EXCEPTION 'invalid live videos lease' USING ERRCODE='22023'; END IF;
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation AND x.action='meta.live_videos' AND x.provider='facebook';
 IF NOT FOUND THEN RAISE EXCEPTION 'live videos unavailable' USING ERRCODE='P0002'; END IF;
 SELECT * INTO b FROM integration.bindings x WHERE x.id=o.binding_id AND x.tenant_id=o.tenant_id AND x.store_id=o.store_id FOR SHARE;
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation FOR SHARE;
 IF o.state<>'DISPATCHING' OR o.lease_mode<>'dispatch' OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN RAISE EXCEPTION 'live videos lease conflict' USING ERRCODE='40001'; END IF;
 IF b.id IS NULL OR NOT b.enabled OR b.provider<>'facebook' OR b.external_asset_id<>o.external_asset_id OR integration.check_meta_live_videos(p_operation)<>'' THEN RETURN; END IF;
 RETURN QUERY SELECT c.tenant_id,c.store_id,c.binding_id,c.provider,c.asset_id,c.version,c.key_id,c.nonce,c.ciphertext,c.scopes_attested
 FROM integration.meta_page_heads h JOIN integration.meta_page_credentials c ON c.tenant_id=h.tenant_id AND c.store_id=h.store_id AND c.binding_id=h.binding_id AND c.version=h.current_version
 WHERE h.tenant_id=o.tenant_id AND h.store_id=o.store_id AND h.binding_id=o.binding_id AND c.provider='facebook' AND c.asset_id=o.external_asset_id
  AND c.scopes_attested @> ARRAY['pages_read_engagement'];
END $$;
ALTER FUNCTION integration.load_meta_live_videos_token(uuid,bigint,bytea) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.load_meta_live_videos_token(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.load_meta_live_videos_token(uuid,bigint,bytea) TO commerce_claims_worker;
COMMENT ON FUNCTION integration.load_meta_live_videos_token(uuid,bigint,bytea) IS
 'internal/integrations/metareply (A5-3) read-only route: current scoped Page ciphertext under the exact dispatch lease and pages_read_engagement only; token never leaves the worker process.';

CREATE FUNCTION integration.finish_meta_live_videos(p_operation uuid,p_generation bigint,p_token bytea,p_mode text,p_result jsonb) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations;v_state text;v_code text;
BEGIN
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation AND x.action='meta.live_videos' AND x.provider='facebook' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'live videos unavailable' USING ERRCODE='P0002'; END IF;
 IF p_token IS NULL OR octet_length(p_token)<>32 OR p_mode IS DISTINCT FROM 'dispatch' OR o.generation IS DISTINCT FROM p_generation
  OR o.lease_mode IS DISTINCT FROM p_mode OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) OR o.state<>'DISPATCHING' THEN RAISE EXCEPTION 'live videos lease conflict' USING ERRCODE='40001'; END IF;
 IF integration.check_meta_live_videos(p_operation)<>'' THEN RAISE EXCEPTION 'live videos binding changed' USING ERRCODE='42501'; END IF;
 PERFORM 1 FROM integration.load_meta_live_videos_token(p_operation,p_generation,p_token);
 IF NOT FOUND THEN RAISE EXCEPTION 'live videos permission revoked' USING ERRCODE='42501'; END IF;
 IF jsonb_typeof(p_result) IS DISTINCT FROM 'object' OR p_result->>'state' IS NULL OR p_result->>'state' NOT IN ('succeeded','failed')
  OR jsonb_typeof(p_result->'items') IS DISTINCT FROM 'array' OR jsonb_array_length(p_result->'items')>25
  OR (p_result->>'code' IS NOT NULL AND p_result->>'code' !~ '^[a-z][a-z0-9_]{0,63}$') THEN
  RAISE EXCEPTION 'invalid live videos result' USING ERRCODE='22023'; END IF;
 v_state:=p_result->>'state';v_code:=p_result->>'code';
 INSERT INTO live.page_live_video_snapshots(tenant_id,store_id,binding_id,operation_id,requested_at,state,code,items)
 VALUES(o.tenant_id,o.store_id,o.binding_id,o.id,o.created_at,v_state,v_code,p_result->'items')
 ON CONFLICT(tenant_id,store_id,binding_id) DO UPDATE SET operation_id=EXCLUDED.operation_id,requested_at=EXCLUDED.requested_at,
  state=EXCLUDED.state,code=EXCLUDED.code,items=EXCLUDED.items,fetched_at=clock_timestamp()
 WHERE live.page_live_video_snapshots.requested_at<=EXCLUDED.requested_at;
END $$;
ALTER FUNCTION integration.finish_meta_live_videos(uuid,bigint,bytea,text,jsonb) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.finish_meta_live_videos(uuid,bigint,bytea,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.finish_meta_live_videos(uuid,bigint,bytea,text,jsonb) TO commerce_claims_worker;
COMMENT ON FUNCTION integration.finish_meta_live_videos(uuid,bigint,bytea,text,jsonb) IS
 'internal/integrations/metareply (A5-3) Finish in the completion transaction; exact lease, latest-wins bounded snapshot on SUCCEEDED/FAILED_FINAL, no individual/viewer linkage.';

CREATE FUNCTION live.read_page_live_videos(p_binding uuid) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid;v_store uuid;snap live.page_live_video_snapshots;op integration.operations;
BEGIN
 IF p_binding IS NULL THEN RAISE EXCEPTION 'invalid binding' USING ERRCODE='PT400'; END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 IF NOT EXISTS(SELECT 1 FROM integration.bindings b WHERE b.tenant_id=v_tenant AND b.store_id=v_store AND b.id=p_binding) THEN
  RAISE EXCEPTION 'binding not found' USING ERRCODE='PT404'; END IF;
 SELECT * INTO snap FROM live.page_live_video_snapshots x WHERE x.tenant_id=v_tenant AND x.store_id=v_store AND x.binding_id=p_binding;
 IF FOUND THEN
  RETURN jsonb_build_object('binding_id',p_binding,'state',snap.state,'code',snap.code,'requested_at',snap.requested_at,
   'fetched_at',snap.fetched_at,'items',snap.items);
 END IF;
 SELECT * INTO op FROM integration.operations x WHERE x.tenant_id=v_tenant AND x.store_id=v_store AND x.provider='facebook'
  AND x.action='meta.live_videos' AND x.request->>'binding_id'=p_binding::text
  ORDER BY x.created_at DESC,x.id DESC LIMIT 1;
 IF NOT FOUND THEN
  RETURN jsonb_build_object('binding_id',p_binding,'state','none','code',NULL,'requested_at',NULL,'fetched_at',NULL,'items','[]'::jsonb);
 END IF;
 RETURN jsonb_build_object('binding_id',p_binding,
  'state',CASE op.state WHEN 'READY' THEN 'pending' WHEN 'DISPATCHING' THEN 'pending' WHEN 'UNKNOWN' THEN 'unknown'
   WHEN 'SUCCEEDED' THEN 'succeeded' WHEN 'ACKNOWLEDGED' THEN 'succeeded' ELSE 'failed' END,
  'code',CASE WHEN op.result_code='' THEN NULL ELSE op.result_code END,
  'requested_at',op.created_at,'fetched_at',NULL,'items','[]'::jsonb);
END $$;
ALTER FUNCTION live.read_page_live_videos(uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION live.read_page_live_videos(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.read_page_live_videos(uuid) TO commerce_runtime;
COMMENT ON FUNCTION live.read_page_live_videos(uuid) IS
 'internal/integrations/metareply (A5-3) GET: binding-scoped latest snapshot or the newest operation state (none/pending/succeeded/failed/unknown); items only on a terminal snapshot. No provider call, no token access (token stays in the worker).';
