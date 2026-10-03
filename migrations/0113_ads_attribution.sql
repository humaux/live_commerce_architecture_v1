-- ads-attribution Amendment 1: store-scoped, erasable measurement separate from retained financial snapshots.
-- No Pixel; no Meta writes; no buyer demographic collection. 0112 refusal projections are untouched.
CREATE SCHEMA orders;
REVOKE ALL ON SCHEMA orders FROM PUBLIC;
GRANT USAGE ON SCHEMA orders TO commerce_checkout_writer,commerce_checkout_runtime,commerce_ads_writer,commerce_privacy_writer;
COMMENT ON SCHEMA orders IS 'internal/attribution: narrow order measurement context only; checkout remains order authority.';
CREATE UNIQUE INDEX checkout_orders_attribution_scope ON checkout.orders(tenant_id,store_id,id);
COMMENT ON INDEX checkout.checkout_orders_attribution_scope IS 'internal/attribution: composite foreign key prevents attaching context to another store order.';
CREATE TABLE orders.order_attribution (
 order_id uuid PRIMARY KEY, tenant_id uuid NOT NULL, store_id uuid NOT NULL,
 path text NOT NULL CHECK(path IN ('ad_click','boosted_post')), draft_id uuid, post_id text,
 clicked_at timestamptz, frozen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 fbc text CHECK(fbc ~ '^fb\.1\.[1-9][0-9]{0,15}\.[A-Za-z0-9_-]{1,500}$'),
 fbp text CHECK(fbp ~ '^fb\.1\.[1-9][0-9]{0,15}\.[0-9]{1,20}$'), client_ip inet,
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,draft_id) REFERENCES ads.campaign_drafts(tenant_id,store_id,id),
 CHECK((path='ad_click' AND draft_id IS NOT NULL AND clicked_at IS NOT NULL) OR
       (path='boosted_post' AND post_id IS NOT NULL AND clicked_at IS NULL)));
CREATE INDEX order_attribution_draft ON orders.order_attribution(tenant_id,store_id,draft_id,frozen_at);
ALTER TABLE orders.order_attribution ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders.order_attribution FORCE ROW LEVEL SECURITY;
REVOKE ALL ON orders.order_attribution FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE(fbc,fbp,client_ip) ON orders.order_attribution TO commerce_checkout_writer;
CREATE POLICY attribution_owner ON orders.order_attribution TO commerce_checkout_writer USING(true) WITH CHECK(true);
COMMENT ON TABLE orders.order_attribution IS 'internal/checkout Begin writes once through orders.freeze_attribution; only scoped definers may read. Erasure clears pseudonyms, never financial facts.';
COMMENT ON INDEX orders.order_attribution_draft IS 'internal/ads: scoped factual attribution report, not a buyer identity index.';

-- Domain-owned helper reads exact accepted comment evidence; no runtime table grants.
CREATE FUNCTION claims.order_comment_posts(p_tenant uuid,p_store uuid,p_order uuid)
RETURNS TABLE(post_id text,occurred_at timestamptz,session_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT DISTINCT src.source_object_id,i.occurred_at,i.session_id
 FROM claims.live_price_uses u
 JOIN claims.lines l ON l.tenant_id=u.tenant_id AND l.store_id=u.store_id AND l.bundle_id=u.bundle_id AND l.offer_id=u.offer_id
 JOIN claims.events e ON e.tenant_id=l.tenant_id AND e.store_id=l.store_id AND e.bundle_id=l.bundle_id AND e.offer_id=l.offer_id
  AND e.line_version=l.version AND e.outcome='ACCEPTED' AND e.source_kind='meta'
 JOIN claims.meta_intake i ON i.tenant_id=e.tenant_id AND i.store_id=e.store_id AND i.applied_event_id=e.id
  AND i.inbox_event_id=e.source_event_id AND i.state='APPLIED'
 JOIN live.claim_sources src ON src.tenant_id=i.tenant_id AND src.store_id=i.store_id AND src.id=i.source_id
 WHERE u.tenant_id=p_tenant AND u.store_id=p_store AND u.order_id=p_order
$$;
ALTER FUNCTION claims.order_comment_posts(uuid,uuid,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.order_comment_posts(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.order_comment_posts(uuid,uuid,uuid) TO commerce_checkout_writer;
COMMENT ON FUNCTION claims.order_comment_posts(uuid,uuid,uuid) IS 'internal/attribution R6: checkout definer only, exact accepted claim version to intake comment to post; never session fan-out or actor data.';

GRANT USAGE ON SCHEMA ads,customers TO commerce_checkout_writer;
CREATE FUNCTION ads.attribution_match(p_tenant uuid,p_store uuid,p_draft uuid,p_post text,p_at timestamptz)
RETURNS TABLE(draft_id uuid) LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT d.id FROM ads.campaign_drafts d
 WHERE d.tenant_id=p_tenant AND d.store_id=p_store AND
 ((p_draft IS NOT NULL AND d.id=p_draft) OR
 (p_draft IS NULL AND d.template='BOOST_POST' AND d.source_ref=p_post AND p_at>=d.starts_at AND p_at<d.ends_at
  AND (d.ended_at IS NULL OR p_at<d.ended_at)
  AND EXISTS(SELECT 1 FROM ads.remote_objects r JOIN integration.operations o ON o.id=r.operation_id
   WHERE r.tenant_id=p_tenant AND r.store_id=p_store AND r.draft_id=d.id AND r.kind='activate'
    AND o.state='SUCCEEDED' AND o.updated_at<=p_at)
  AND NOT EXISTS(SELECT 1 FROM ads.remote_objects r JOIN integration.operations o ON o.id=r.operation_id
   WHERE r.tenant_id=p_tenant AND r.store_id=p_store AND r.draft_id=d.id AND r.kind='pause'
    AND o.state='SUCCEEDED' AND o.updated_at<=p_at)))
$$;
ALTER FUNCTION ads.attribution_match(uuid,uuid,uuid,text,timestamptz) OWNER TO commerce_ads_writer;
REVOKE ALL ON FUNCTION ads.attribution_match(uuid,uuid,uuid,text,timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ads.attribution_match(uuid,uuid,uuid,text,timestamptz) TO commerce_checkout_writer;
COMMENT ON FUNCTION ads.attribution_match(uuid,uuid,uuid,text,timestamptz) IS 'internal/attribution: checkout-only same-store draft existence or actual successful boost interval, no inferred paid-comment identity.';
GRANT EXECUTE ON FUNCTION customers.consent_allows(uuid,uuid,uuid,text,text) TO commerce_checkout_writer;

CREATE FUNCTION ads.capi_ip_needed(p_tenant uuid,p_store uuid,p_owner uuid,p_attempt uuid) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s ads.store_settings; e record;
BEGIN
 IF NOT customers.consent_allows(p_tenant,p_store,p_owner,'ads_personalization','meta_ads') THEN RETURN false; END IF;
 SELECT * INTO s FROM ads.store_settings x WHERE x.tenant_id=p_tenant AND x.store_id=p_store;
 IF NOT FOUND OR NOT s.capi_enabled OR s.capi_enabled_by IS NULL OR s.capi_dataset_binding IS NULL
  OR (s.environment='SANDBOX' AND s.capi_test_event_code IS NULL) THEN RETURN false; END IF;
 IF NOT identity.principal_holds(p_tenant,p_store,s.capi_enabled_by,ARRAY['ads:manage']) OR NOT EXISTS
  (SELECT 1 FROM integration.bindings b WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=s.capi_dataset_binding
   AND b.provider='meta_dataset' AND b.enabled) THEN RETURN false; END IF;
 IF NOT EXISTS(SELECT 1 FROM ads.capi_contexts c WHERE c.tenant_id=p_tenant AND c.store_id=p_store AND c.owner_id=p_owner
   AND c.captured_at>=clock_timestamp()-interval '7 days') THEN RETURN false; END IF;
 IF p_attempt IS NOT NULL THEN
  IF NOT EXISTS(SELECT 1 FROM checkout.payment_attempts a WHERE a.tenant_id=p_tenant AND a.store_id=p_store
   AND a.owner_id=p_owner AND a.id=p_attempt) THEN RETURN false; END IF;
  SELECT o.state INTO e FROM ads.capi_events c JOIN integration.operations o ON o.id=c.operation_id
   WHERE c.tenant_id=p_tenant AND c.store_id=p_store AND c.attempt_id=p_attempt;
  IF FOUND THEN RETURN e.state IN ('READY','DISPATCHING','UNKNOWN','RETRY_WAIT'); END IF;
  IF EXISTS(SELECT 1 FROM payments.facts f WHERE f.tenant_id=p_tenant AND f.store_id=p_store AND f.attempt_id=p_attempt
    AND f.kind='CAPTURED') THEN RETURN ads.plan_capi_eligible(p_attempt); END IF;
 END IF;
 RETURN true;
END $$;
ALTER FUNCTION ads.capi_ip_needed(uuid,uuid,uuid,uuid) OWNER TO commerce_ads_writer;
REVOKE ALL ON FUNCTION ads.capi_ip_needed(uuid,uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ads.capi_ip_needed(uuid,uuid,uuid,uuid) TO commerce_checkout_writer;
COMMENT ON FUNCTION ads.capi_ip_needed(uuid,uuid,uuid,uuid) IS 'internal/attribution R3 checkout-only eligibility probe; no secrets or facts exposed; terminal/unconsented/disabled contexts need no IP.';

CREATE FUNCTION orders.freeze_attribution(p_hash bytea,p_store uuid,p_order uuid,p_touch jsonb,p_ip text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; o record; d uuid; ds uuid[]; posts text[]; c record; click_at timestamptz;
 v_path text; v_post text; v_fbc text; v_fbp text; v_ip inet;
BEGIN
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 SELECT x.* INTO o FROM checkout.orders x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.owner_id=s.owner_id
  AND x.id=p_order AND x.creator_session_id=s.session_id AND x.created_at>=clock_timestamp()-interval '1 minute';
 IF NOT FOUND THEN RAISE EXCEPTION 'attribution order unavailable' USING ERRCODE='PT404'; END IF;
 IF EXISTS(SELECT 1 FROM orders.order_attribution a WHERE a.order_id=p_order) THEN RETURN; END IF;
 -- Invalid/foreign measurement is ignored; it cannot deny a legitimate order or change its financial snapshot.
 IF jsonb_typeof(p_touch)='object' AND (p_touch->>'draft_id') ~ '^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$' THEN
  BEGIN click_at:=(p_touch->>'clicked_at')::timestamptz; EXCEPTION WHEN invalid_datetime_format OR datetime_field_overflow THEN click_at:=NULL; END;
  IF click_at BETWEEN clock_timestamp()-interval '7 days' AND clock_timestamp() THEN
   SELECT m.draft_id INTO d FROM ads.attribution_match(s.tenant_id,p_store,(p_touch->>'draft_id')::uuid,NULL,NULL) m;
   IF d IS NOT NULL THEN
    v_path:='ad_click';
    IF p_touch->>'fbc' ~ '^fb\.1\.[1-9][0-9]{0,15}\.[A-Za-z0-9_-]{1,500}$' THEN v_fbc:=p_touch->>'fbc'; END IF;
    IF p_touch->>'fbp' ~ '^fb\.1\.[1-9][0-9]{0,15}\.[0-9]{1,20}$' THEN v_fbp:=p_touch->>'fbp'; END IF;
   END IF;
  END IF;
 END IF;
 IF v_path IS NULL THEN
  SELECT array_agg(DISTINCT x.post_id) INTO posts FROM claims.order_comment_posts(s.tenant_id,p_store,p_order) x;
  -- An order spanning different posts has no single factual post attribution; never choose one arbitrarily.
  IF cardinality(posts)=1 THEN
   v_post:=posts[1];
   SELECT array_agg(DISTINCT m.draft_id) INTO ds FROM claims.order_comment_posts(s.tenant_id,p_store,p_order) x
    CROSS JOIN LATERAL ads.attribution_match(s.tenant_id,p_store,NULL,x.post_id,x.occurred_at) m;
   IF cardinality(ds)>0 THEN v_path:='boosted_post'; d:=CASE WHEN cardinality(ds)=1 THEN ds[1] ELSE NULL END; click_at:=NULL; END IF;
  END IF;
 END IF;
 IF v_path IS NULL THEN RETURN; END IF;
 IF o.payment_mode='card' AND ads.capi_ip_needed(s.tenant_id,p_store,s.owner_id,NULL) THEN
  BEGIN v_ip:=nullif(p_ip,'')::inet; EXCEPTION WHEN invalid_text_representation THEN v_ip:=NULL; END;
 END IF;
 INSERT INTO orders.order_attribution(order_id,tenant_id,store_id,path,draft_id,post_id,clicked_at,fbc,fbp,client_ip)
 VALUES(p_order,s.tenant_id,p_store,v_path,d,CASE WHEN v_path='boosted_post' THEN v_post ELSE NULL END,click_at,v_fbc,v_fbp,v_ip)
 ON CONFLICT(order_id) DO NOTHING;
END $$;
ALTER FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text) TO commerce_checkout_runtime;
COMMENT ON FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text) IS 'internal/checkout Begin-only, authenticated fresh order, same transaction, click beats exact comment, ambiguous boost draft NULL. Does not alter money or immutable snapshot.';

CREATE FUNCTION orders.erase_ad_context(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS void
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 UPDATE orders.order_attribution a SET fbc=NULL,fbp=NULL,client_ip=NULL FROM checkout.orders o
 WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.owner_id=p_owner
 AND a.tenant_id=p_tenant AND a.store_id=p_store AND a.order_id=o.id
$$;
ALTER FUNCTION orders.erase_ad_context(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION orders.erase_ad_context(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orders.erase_ad_context(uuid,uuid,uuid) TO commerce_privacy_writer;
COMMENT ON FUNCTION orders.erase_ad_context(uuid,uuid,uuid) IS 'internal/customers erasure-only: redact pseudonyms/IP; preserve aggregate attribution and financial retention.';
DO $patch$
DECLARE body text; needle text:='-- CD4: withdraw every granted pair';
BEGIN
 SELECT prosrc INTO body FROM pg_proc WHERE oid='customers.apply_erasure(uuid,uuid,uuid)'::regprocedure;
 IF position(needle IN body)=0 THEN RAISE EXCEPTION 'erasure baseline drift'; END IF;
 body:=replace(body,needle,'PERFORM orders.erase_ad_context(p_tenant,p_store,p_owner); '||needle);
 EXECUTE 'CREATE OR REPLACE FUNCTION customers.apply_erasure(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS '||quote_literal(body);
END $patch$;

CREATE FUNCTION orders.clear_terminal_capi_ip() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.provider='meta_dataset' AND NEW.action='meta.capi.purchase'
 AND NEW.state IN ('SUCCEEDED','FAILED_FINAL','BLOCKED_POLICY','STALE_BINDING','CANCELLED') THEN
  UPDATE orders.order_attribution a SET client_ip=NULL FROM checkout.payment_attempts p
  WHERE p.tenant_id=NEW.tenant_id AND p.store_id=NEW.store_id AND p.id::text=NEW.request->>'attempt_id'
   AND a.tenant_id=p.tenant_id AND a.store_id=p.store_id AND a.order_id=p.order_id;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION orders.clear_terminal_capi_ip() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION orders.clear_terminal_capi_ip() FROM PUBLIC;
CREATE TRIGGER clear_terminal_capi_ip AFTER UPDATE OF state ON integration.operations
 FOR EACH ROW WHEN(OLD.state IS DISTINCT FROM NEW.state) EXECUTE FUNCTION orders.clear_terminal_capi_ip();
COMMENT ON FUNCTION orders.clear_terminal_capi_ip() IS 'internal/attribution terminal ledger hook: clears only matching order IP in completion transaction; UNKNOWN never resent and not treated as success.';

CREATE FUNCTION orders.capi_context(p_tenant uuid,p_store uuid,p_order uuid)
RETURNS TABLE(fbc text,fbp text,client_ip text,email text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT a.fbc,a.fbp,host(a.client_ip),nullif(o.buyer_email,'') FROM checkout.orders o
 LEFT JOIN orders.order_attribution a ON a.tenant_id=o.tenant_id AND a.store_id=o.store_id AND a.order_id=o.id
 WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_order
$$;
ALTER FUNCTION orders.capi_context(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION orders.capi_context(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orders.capi_context(uuid,uuid,uuid) TO commerce_ads_writer;
COMMENT ON FUNCTION orders.capi_context(uuid,uuid,uuid) IS 'internal/attribution private ads definer dependency only; not a runtime read API; consent checked by caller.';
CREATE FUNCTION ads.capi_attribution_data(p_operation uuid,p_generation bigint,p_lease_token bytea)
RETURNS TABLE(fbc text,fbp text,client_ip text,email text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o record;
BEGIN
 -- Reuse original 0080 lease guard and recheck consent at this last data boundary.
 PERFORM 1 FROM ads.capi_user_data(p_operation,p_generation,p_lease_token);
 IF NOT FOUND OR ads.check_capi(p_operation)<>'' THEN RETURN; END IF;
 SELECT p.tenant_id,p.store_id,p.order_id INTO o FROM ads.capi_events e JOIN checkout.payment_attempts p
  ON p.tenant_id=e.tenant_id AND p.store_id=e.store_id AND p.id=e.attempt_id WHERE e.operation_id=p_operation;
 IF NOT FOUND THEN RETURN; END IF;
 RETURN QUERY SELECT * FROM orders.capi_context(o.tenant_id,o.store_id,o.order_id);
END $$;
ALTER FUNCTION ads.capi_attribution_data(uuid,bigint,bytea) OWNER TO commerce_ads_writer;
REVOKE ALL ON FUNCTION ads.capi_attribution_data(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ads.capi_attribution_data(uuid,bigint,bytea) TO commerce_ads_worker;
COMMENT ON FUNCTION ads.capi_attribution_data(uuid,bigint,bytea) IS 'internal/attribution/capiroute: lease+consent fenced ephemeral CAPI match fields; raw buyer email hashed in memory, never persisted in operation.';

CREATE FUNCTION orders.purge_capi_ip() RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r record; n integer:=0;
BEGIN
 FOR r IN SELECT a.order_id,a.tenant_id,a.store_id,o.owner_id,o.commercial_state,o.expires_at,o.created_at,
  (SELECT p.id FROM checkout.payment_attempts p WHERE p.tenant_id=o.tenant_id AND p.store_id=o.store_id AND p.order_id=o.id
   ORDER BY p.created_at DESC LIMIT 1) AS attempt_id
  FROM orders.order_attribution a JOIN checkout.orders o ON o.id=a.order_id AND o.tenant_id=a.tenant_id AND o.store_id=a.store_id
  WHERE a.client_ip IS NOT NULL ORDER BY a.frozen_at LIMIT 1000
 LOOP
  IF r.created_at<clock_timestamp()-interval '7 days' OR r.commercial_state='CANCELLED'
    OR (r.commercial_state IN ('DRAFT','AWAITING_PAYMENT') AND r.expires_at<=clock_timestamp())
    OR NOT ads.capi_ip_needed(r.tenant_id,r.store_id,r.owner_id,r.attempt_id) THEN
   UPDATE orders.order_attribution SET client_ip=NULL WHERE order_id=r.order_id AND tenant_id=r.tenant_id AND store_id=r.store_id;
   n:=n+1;
  END IF;
 END LOOP;
 RETURN n;
END $$;
ALTER FUNCTION orders.purge_capi_ip() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION orders.purge_capi_ip() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orders.purge_capi_ip() TO commerce_ads_writer;
COMMENT ON FUNCTION orders.purge_capi_ip() IS 'internal/attribution R3 bounded cleanup via existing CAPI sweep: aborted/expired, ineligible or older than send window. Keeps non-PII attribution; no new job.';
DO $patch$
DECLARE body text;
BEGIN
 SELECT prosrc INTO body FROM pg_proc WHERE oid='ads.plan_capi_purge()'::regprocedure;
 body:=replace(body,'RETURN v_n;','PERFORM orders.purge_capi_ip(); RETURN v_n;');
 EXECUTE 'CREATE OR REPLACE FUNCTION ads.plan_capi_purge() RETURNS integer LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS '||quote_literal(body);
END $patch$;

-- Aggregate Meta snapshots stay separate from all buyer/order data. A dimension
-- is a partition, not another additive copy of the campaign totals.
CREATE TABLE ads.insights_breakdowns (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,draft_id uuid NOT NULL,day date NOT NULL,
 timezone_name text NOT NULL,currency text NOT NULL CHECK(currency IN ('TWD','USD','HKD')),
 dimension text NOT NULL CHECK(dimension IN ('age_gender','region','placement','device','hourly')),
 bucket text NOT NULL CHECK(char_length(bucket) BETWEEN 1 AND 160),hour_start timestamptz,
 spend_minor bigint NOT NULL CHECK(spend_minor>=0),reach bigint NOT NULL CHECK(reach>=0),
 impressions bigint NOT NULL CHECK(impressions>=0),clicks bigint NOT NULL CHECK(clicks>=0),
 engagements bigint NOT NULL CHECK(engagements>=0),comments bigint NOT NULL CHECK(comments>=0),
 purchases bigint CHECK(purchases>=0),purchase_value_minor bigint CHECK(purchase_value_minor>=0),
 source_operation_id uuid NOT NULL REFERENCES ads.insight_reads(operation_id),fetched_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id,draft_id,day,dimension,bucket),
 FOREIGN KEY(tenant_id,store_id,draft_id) REFERENCES ads.campaign_drafts(tenant_id,store_id,id),
 CHECK((dimension='hourly')=(hour_start IS NOT NULL)));
ALTER TABLE ads.insights_breakdowns ENABLE ROW LEVEL SECURITY;
ALTER TABLE ads.insights_breakdowns FORCE ROW LEVEL SECURITY;
REVOKE ALL ON ads.insights_breakdowns FROM PUBLIC;
GRANT SELECT,INSERT,DELETE ON ads.insights_breakdowns TO commerce_ads_writer;
CREATE POLICY ads_breakdown_owner ON ads.insights_breakdowns TO commerce_ads_writer USING(true) WITH CHECK(true);
COMMENT ON TABLE ads.insights_breakdowns IS 'internal/ads D9: aggregate Meta campaign/day dimensions; each dimension is separate, never person-level or added across partitions. Hour starts are absolute, day remains account timezone.';

CREATE FUNCTION ads.finish_insights_breakdowns(p_operation uuid,p_generation bigint,p_token bytea,p_mode text,p_result jsonb) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations; ir ads.insight_reads; v_tz text; v_cur text; r jsonb;
BEGIN
 IF p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32 OR p_mode<>'dispatch'
  OR jsonb_typeof(p_result)<>'object' OR jsonb_typeof(p_result->'rows')<>'array' OR jsonb_array_length(p_result->'rows')>10000 THEN
  RAISE EXCEPTION 'invalid insight projection' USING ERRCODE='22023'; END IF;
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation AND x.provider='meta_ads' AND x.action='meta.ads.read_insights';
 IF NOT FOUND THEN RAISE EXCEPTION 'insight operation unavailable' USING ERRCODE='P0002'; END IF;
 IF o.state<>'DISPATCHING' OR o.generation<>p_generation OR o.lease_until<=clock_timestamp() OR o.lease_until IS NULL
  OR o.lease_mode<>p_mode OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN
  RAISE EXCEPTION 'insight lease conflict' USING ERRCODE='40001'; END IF;
 SELECT * INTO ir FROM ads.insight_reads x WHERE x.operation_id=o.id AND x.tenant_id=o.tenant_id AND x.store_id=o.store_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'insight read unavailable' USING ERRCODE='P0002'; END IF;
 v_tz:=p_result->>'timezone_name';v_cur:=p_result->>'currency';
 IF v_tz IS NULL OR NOT EXISTS(SELECT 1 FROM pg_timezone_names WHERE name=v_tz) OR v_cur NOT IN ('TWD','USD','HKD') THEN
  RAISE EXCEPTION 'invalid insight timezone or currency' USING ERRCODE='22023'; END IF;
 -- A late old read cannot replace newer data. A final account-day is immutable.
 IF EXISTS(SELECT 1 FROM ads.insights_daily i WHERE i.tenant_id=ir.tenant_id AND i.store_id=ir.store_id AND i.draft_id=ir.draft_id AND i.day=ir.day AND i.final)
  OR EXISTS(SELECT 1 FROM ads.insights_breakdowns i WHERE i.tenant_id=ir.tenant_id AND i.store_id=ir.store_id AND i.draft_id=ir.draft_id AND i.day=ir.day AND i.fetched_at>o.updated_at) THEN RETURN; END IF;
 DELETE FROM ads.insights_breakdowns i WHERE i.tenant_id=ir.tenant_id AND i.store_id=ir.store_id AND i.draft_id=ir.draft_id AND i.day=ir.day;
 FOR r IN SELECT value FROM jsonb_array_elements(p_result->'rows') LOOP
  IF jsonb_typeof(r)<>'object' OR (r->>'dimension'='hourly' AND ((r->>'hour_start')::timestamptz AT TIME ZONE v_tz)::date<>ir.day) THEN
   RAISE EXCEPTION 'invalid insight row' USING ERRCODE='22023'; END IF;
  INSERT INTO ads.insights_breakdowns VALUES(ir.tenant_id,ir.store_id,ir.draft_id,ir.day,v_tz,v_cur,r->>'dimension',r->>'bucket',
   (r->>'hour_start')::timestamptz,(r->>'spend_minor')::bigint,(r->>'reach')::bigint,(r->>'impressions')::bigint,
   (r->>'clicks')::bigint,(r->>'engagements')::bigint,(r->>'comments')::bigint,(r->>'purchases')::bigint,
   (r->>'purchase_value_minor')::bigint,o.id,o.updated_at);
 END LOOP;
END $$;
ALTER FUNCTION ads.finish_insights_breakdowns(uuid,bigint,bytea,text,jsonb) OWNER TO commerce_ads_writer;
REVOKE ALL ON FUNCTION ads.finish_insights_breakdowns(uuid,bigint,bytea,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ads.finish_insights_breakdowns(uuid,bigint,bytea,text,jsonb) TO commerce_ads_worker;
COMMENT ON FUNCTION ads.finish_insights_breakdowns(uuid,bigint,bytea,text,jsonb) IS 'internal/integrations/meta_ads Finish hook: validated aggregate detail, exact claim lease and same transaction as completion/0112 refusal cleanup; replace whole day, idempotent and store scoped.';

DO $comments$
DECLARE c record;
BEGIN
 FOR c IN SELECT column_name FROM information_schema.columns WHERE table_schema='orders' AND table_name='order_attribution' LOOP
  EXECUTE format('COMMENT ON COLUMN orders.order_attribution.%I IS %L',c.column_name,'internal/attribution R2: private store-scoped measurement; only domain definers, never buyer/merchant raw export.');
 END LOOP;
END $comments$;
