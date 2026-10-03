-- orders-v2: scoped read/filter projection only. No money/state writer is replaced.
-- Reuse the final v1 financial projection and fresh auth fence, with shape-checked
-- substitutions. This avoids forking a stale pre-COD payment/refund CASE expression.
CREATE INDEX orders_phone_suffix_read ON checkout.orders
 (tenant_id,store_id,(right(snapshot#>>'{destination,phone}',4)),created_at DESC,id DESC);
CREATE INDEX orders_method_date_read ON checkout.orders
 (tenant_id,store_id,payment_mode,created_at DESC,id DESC);
CREATE INDEX shipment_tracking_read ON fulfillment.manual_shipment_versions
 (tenant_id,store_id,tracking_number,order_id) WHERE status='SHIPPED';
COMMENT ON INDEX checkout.orders_phone_suffix_read IS 'internal/merchantorders v2 scoped exact phone-last4 read; never returns full phone or authorizes a caller.';
COMMENT ON INDEX checkout.orders_method_date_read IS 'internal/merchantorders v2 scoped payment/date keyset read; no state writes.';
COMMENT ON INDEX fulfillment.shipment_tracking_read IS 'internal/merchantorders v2 current-head tracking lookup; old/voided versions do not match.';

GRANT USAGE ON SCHEMA claims,live TO commerce_auth;
GRANT SELECT(tenant_id,store_id,order_id,bundle_id) ON claims.live_price_uses TO commerce_auth;
GRANT SELECT(tenant_id,store_id,id,session_id) ON claims.bundles TO commerce_auth;
GRANT SELECT(tenant_id,store_id,id,title,created_at) ON live.sessions TO commerce_auth;
GRANT SELECT(provider_logistics_id,shipment_no) ON fulfillment.cvs_shipments TO commerce_auth;
CREATE POLICY orders_v2_price_use_read ON claims.live_price_uses FOR SELECT TO commerce_auth USING
 (tenant_id::text=current_setting('app.tenant_id',true) AND store_id::text=current_setting('app.store_id',true));
CREATE POLICY orders_v2_bundle_read ON claims.bundles FOR SELECT TO commerce_auth USING
 (tenant_id::text=current_setting('app.tenant_id',true) AND store_id::text=current_setting('app.store_id',true));
CREATE POLICY orders_v2_session_read ON live.sessions FOR SELECT TO commerce_auth USING
 (tenant_id::text=current_setting('app.tenant_id',true) AND store_id::text=current_setting('app.store_id',true));

DO $migration$
DECLARE body text; before_owned integer; after_owned integer; needle text; replacement text;
BEGIN
 SELECT prosrc INTO STRICT body FROM pg_proc
 WHERE oid='identity.read_merchant_orders(bytea,uuid,uuid,integer,timestamptz,uuid,text)'::regprocedure;
 -- Preserve auth before and after the data SELECT. p_order stays NULL: list only.
 body:=replace(body,'DECLARE s record;', 'DECLARE p_order uuid := NULL; s record;');
 needle:='IF p_hash IS NULL';
 IF position(needle IN body)=0 OR position('cod_collect_minor' IN body)=0 THEN
  RAISE EXCEPTION 'orders-v2 baseline projection drift'; END IF;
 body:=replace(body,needle,$validation$
 IF p_state IS NULL OR p_state NOT IN ('active','all','DRAFT','AWAITING_PAYMENT','AWAITING_TRANSFER','AWAITING_COLLECTION','CONFIRMED','CANCELLED','shipped','unshipped','cvs_pending')
 OR p_bucket IS NULL OR p_bucket NOT IN ('all','unpaid','transfer_review','ready_to_ship','ready_to_consign','shipped','completed','cancelled')
 OR p_query IS NULL OR char_length(p_query)>80 OR p_query<>btrim(p_query) OR p_query ~ '[[:cntrl:]]'
 OR p_payment IS NULL OR p_payment NOT IN ('','card','bank_transfer','pay_at_pickup','cash_on_delivery')
 OR p_delivery IS NULL OR p_delivery NOT IN ('','home','cvs_711','cvs_familymart','cvs_hilife','cvs_okmart')
 OR (p_from IS NOT NULL AND NOT isfinite(p_from)) OR (p_to IS NOT NULL AND NOT isfinite(p_to))
 OR (p_from IS NOT NULL AND p_to IS NOT NULL AND p_from>=p_to) THEN
  RAISE EXCEPTION 'invalid orders-v2 filter' USING ERRCODE='PT400'; END IF;
 IF p_hash IS NULL$validation$);
 body:=replace(body,'p_state NOT IN (''all''', 'p_state NOT IN (''active'',''all''');
 before_owned:=position(' WITH owned AS MATERIALIZED (' IN body);
 after_owned:=position(' ), projected AS (' IN body);
 IF before_owned=0 OR after_owned<=before_owned THEN RAISE EXCEPTION 'orders-v2 owned projection drift'; END IF;
 replacement:=$query$
 WITH linked AS MATERIALIZED (
  -- claims.live_price_uses is placement evidence, not inferred visitor attribution.
  SELECT DISTINCT u.order_id,b.session_id,ls.title AS name,ls.created_at AS session_created_at
  FROM claims.live_price_uses u JOIN claims.bundles b
   ON b.tenant_id=u.tenant_id AND b.store_id=u.store_id AND b.id=u.bundle_id
  JOIN live.sessions ls ON ls.tenant_id=b.tenant_id AND ls.store_id=b.store_id AND ls.id=b.session_id
  WHERE u.tenant_id=s.tenant_id AND u.store_id=p_store
 ), filtered AS MATERIALIZED (
  SELECT o.id,o.tenant_id,o.store_id,o.owner_id,o.created_at,o.updated_at,o.currency,
   o.total_minor,o.commercial_state,o.fulfillment_state,o.country,o.service_code,
   o.payment_mode,o.collection_state,o.cod_surcharge_minor,o.source,
   o.snapshot#>>'{destination,pickup,verification_kind}' AS pickup_vk,
   NULL::jsonb AS snapshot,
   'LC-'||upper(replace(o.id::text,'-','')) AS order_number,
   -- Legacy display gaps must not poison otherwise valid transaction rows.
   -- btrim only trims ASCII spaces; a leading U+3000/NBSP/zero-width char would yield a mask the Go validator rejects.
   -- Start at the first non-whitespace rune; an all-whitespace name falls back to the placeholder.
   coalesce(left(nullif(regexp_replace(o.snapshot#>>'{destination,recipient_name}',
     '^[\s\u0085\u00a0\u1680\u180e\u2000-\u200f\u2028-\u202f\u205f\u2060\u3000\ufeff]+',''),''),1)||'***','—') AS recipient_masked,
   CASE WHEN o.snapshot#>>'{destination,kind}'='home' THEN 'home'
     WHEN o.snapshot#>>'{destination,pickup,kind}' IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart')
       THEN o.snapshot#>>'{destination,pickup,kind}' ELSE 'unknown' END AS delivery_kind,
   coalesce((SELECT jsonb_agg(jsonb_build_object('id',l.session_id,'name',l.name) ORDER BY l.session_id)
     FROM linked l WHERE l.order_id=o.id),'[]'::jsonb) AS live_sessions,
   o.commercial_state NOT IN ('DRAFT','CANCELLED') AND
     ((o.payment_mode='card' AND o.commercial_state<>'CONFIRMED') OR
      (o.payment_mode='bank_transfer' AND o.commercial_state='AWAITING_TRANSFER' AND bt.state<>'SUBMITTED') OR
      (o.payment_mode IN ('cash_on_delivery','pay_at_pickup') AND o.collection_state='PENDING')) AS unpaid,
   o.commercial_state='AWAITING_TRANSFER' AND bt.state='SUBMITTED' AS transfer_review,
   o.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION') AND o.fulfillment_state='MANUAL_UNASSIGNED'
     AND fulfillment.manual_shipment_eligible(o.tenant_id,o.store_id,o.id) AS ready_to_ship,
   o.fulfillment_state='PROVIDER_LABEL_CREATED' AND cs.state='CREATED' AS ready_to_consign,
   o.fulfillment_state='MERCHANT_SHIPPED' OR cs.state IN ('AT_DC','AT_STORE','PICKED_UP') AS shipped,
   o.commercial_state<>'CANCELLED' AND (o.collection_state='COLLECTED' OR
     (cs.state='PICKED_UP' AND o.payment_mode IN ('card','bank_transfer')
       AND fulfillment.order_money_shippable(o.tenant_id,o.store_id,o.id))) AS completed,
   o.commercial_state='CANCELLED' AS cancelled
  FROM checkout.orders o
  LEFT JOIN checkout.bank_transfers bt ON bt.tenant_id=o.tenant_id AND bt.store_id=o.store_id AND bt.order_id=o.id
  LEFT JOIN LATERAL (SELECT c.state,c.provider_logistics_id,c.shipment_no FROM fulfillment.cvs_shipments c
   WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND c.order_id=o.id
   ORDER BY c.attempt DESC LIMIT 1) cs ON true
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store
   AND (p_payment='' OR o.payment_mode=p_payment)
   AND (p_delivery='' OR CASE WHEN o.snapshot#>>'{destination,kind}'='home' THEN 'home'
     ELSE o.snapshot#>>'{destination,pickup,kind}' END=p_delivery)
   AND (p_session IS NULL OR EXISTS(SELECT 1 FROM linked l WHERE l.order_id=o.id AND l.session_id=p_session))
   AND (p_from IS NULL OR o.created_at>=p_from) AND (p_to IS NULL OR o.created_at<p_to)
   AND (p_query='' OR position(lower(p_query) IN lower('LC-'||replace(o.id::text,'-','')))>0
     OR o.id::text=p_query
     OR position(lower(p_query) IN lower(o.snapshot#>>'{destination,recipient_name}'))>0
     OR (p_query ~ '^[0-9]{4}$' AND right(o.snapshot#>>'{destination,phone}',4)=p_query)
     OR EXISTS(SELECT 1 FROM jsonb_array_elements(o.snapshot#>'{quote,lines}') line
       WHERE position(lower(p_query) IN lower(line->>'code'))>0)
     OR EXISTS(SELECT 1 FROM fulfillment.manual_shipment_heads h JOIN fulfillment.manual_shipment_versions v
       ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.order_id=h.order_id AND v.version=h.current_version
       WHERE h.tenant_id=o.tenant_id AND h.store_id=o.store_id AND h.order_id=o.id AND v.status='SHIPPED'
         AND v.tracking_number=p_query)
     OR (cs.state IN ('CREATED','AT_DC','AT_STORE','PICKED_UP','UNCLAIMED')
       AND (cs.shipment_no=p_query OR cs.provider_logistics_id=p_query)))
 ), matched AS MATERIALIZED (
  SELECT * FROM filtered WHERE p_state='all' OR (p_state='active' AND commercial_state<>'DRAFT')
   OR (p_state IN ('DRAFT','AWAITING_PAYMENT','AWAITING_TRANSFER','CONFIRMED','CANCELLED') AND commercial_state=p_state)
   OR (p_state='AWAITING_COLLECTION' AND commercial_state='AWAITING_COLLECTION' AND collection_state='PENDING')
   OR (p_state='shipped' AND shipped) OR (p_state='unshipped' AND ready_to_ship)
   OR (p_state='cvs_pending' AND ready_to_consign)
 ), bucketed AS MATERIALIZED (
  SELECT * FROM matched WHERE p_bucket='all' OR (p_bucket='unpaid' AND unpaid)
   OR (p_bucket='transfer_review' AND transfer_review) OR (p_bucket='ready_to_ship' AND ready_to_ship)
   OR (p_bucket='ready_to_consign' AND ready_to_consign) OR (p_bucket='shipped' AND shipped)
   OR (p_bucket='completed' AND completed) OR (p_bucket='cancelled' AND cancelled)
 ), owned AS MATERIALIZED (
  SELECT * FROM bucketed WHERE p_after_id IS NULL OR (created_at,id)<(p_after_created_at,p_after_id)
  ORDER BY created_at DESC,id DESC LIMIT p_limit
$query$;
 body:=substring(body FROM 1 FOR before_owned-1)||replacement||substring(body FROM after_owned);
 needle:='''order_id'',o.id,''created_at''';
 IF position(needle IN body)=0 THEN RAISE EXCEPTION 'orders-v2 metadata projection drift'; END IF;
 body:=replace(body,needle,$meta$'order_number',o.order_number,'recipient_masked',o.recipient_masked,
   'delivery_kind',o.delivery_kind,'source',o.source,'live_sessions',o.live_sessions,
   'order_id',o.id,'created_at'$meta$);
 needle:='SELECT coalesce(jsonb_agg(value ORDER BY created_at DESC,id DESC),''[]''::jsonb) INTO v_result FROM projected;';
 IF position(needle IN body)=0 THEN RAISE EXCEPTION 'orders-v2 aggregate drift'; END IF;
 body:=replace(body,needle,$aggregate$
 SELECT jsonb_build_object(
  'items',(SELECT coalesce(jsonb_agg(value ORDER BY created_at DESC,id DESC),'[]'::jsonb) FROM projected),
  'total',(SELECT count(*) FROM bucketed),
  'counts',(SELECT jsonb_build_object('all',count(*),'unpaid',count(*) FILTER(WHERE unpaid),
   'transfer_review',count(*) FILTER(WHERE transfer_review),'ready_to_ship',count(*) FILTER(WHERE ready_to_ship),
   'ready_to_consign',count(*) FILTER(WHERE ready_to_consign),'shipped',count(*) FILTER(WHERE shipped),
   'completed',count(*) FILTER(WHERE completed),'cancelled',count(*) FILTER(WHERE cancelled)) FROM matched),
  'sessions',(SELECT coalesce(jsonb_agg(jsonb_build_object('id',session_id,'name',name) ORDER BY session_created_at DESC,session_id DESC),'[]'::jsonb)
   FROM (SELECT DISTINCT session_id,name,session_created_at FROM linked ORDER BY session_created_at DESC,session_id DESC LIMIT 100) choices)
 ) INTO v_result;
$aggregate$);
 EXECUTE 'CREATE FUNCTION identity.read_merchant_orders_v2(p_hash bytea,p_store uuid,p_limit integer,p_after_created_at timestamptz,p_after_id uuid,p_state text,p_bucket text,p_query text,p_payment text,p_delivery text,p_session uuid,p_from timestamptz,p_to timestamptz) RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS '||quote_literal(body);
END $migration$;
ALTER FUNCTION identity.read_merchant_orders_v2(bytea,uuid,integer,timestamptz,uuid,text,text,text,text,text,uuid,timestamptz,timestamptz) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_merchant_orders_v2(bytea,uuid,integer,timestamptz,uuid,text,text,text,text,text,uuid,timestamptz,timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_merchant_orders_v2(bytea,uuid,integer,timestamptz,uuid,text,text,text,text,text,uuid,timestamptz,timestamptz) TO commerce_runtime;
COMMENT ON FUNCTION identity.read_merchant_orders_v2(bytea,uuid,integer,timestamptz,uuid,text,text,text,text,text,uuid,timestamptz,timestamptz) IS 'internal/merchantorders ListV2 only; runtime execute, commerce_auth owner. Single-statement scoped filtered page and SQL counts; final fresh orders:read fence. No PII list, money/state writes, provider calls or new transaction engine.';
