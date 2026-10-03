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
 fbc text CHECK(fbc ~ '^fb\.1\.[1-9][0-9]{0,15}\.[A-Za-z0-9_-]+$' AND char_length(split_part(fbc,'.',4))<=500),
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

-- Every redeemed claim has provenance even without a discounted price. Keep its
-- exact imported version, then freeze an anonymous order/source association at Begin.
ALTER TABLE storefront.cart_lines ADD COLUMN claim_line_version bigint CHECK(claim_line_version IS NULL OR (claim_line_version>0 AND claim_bundle_id IS NOT NULL));
COMMENT ON COLUMN storefront.cart_lines.claim_line_version IS 'internal/claims RedeemLink only: exact imported version, server input only; old carts null and cannot invent a comment source.';
CREATE TABLE claims.order_origins (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,order_id uuid NOT NULL,bundle_id uuid NOT NULL,offer_id uuid NOT NULL,
 line_version bigint NOT NULL CHECK(line_version>0),session_id uuid NOT NULL,post_id text,occurred_at timestamptz NOT NULL,
 PRIMARY KEY(order_id,bundle_id,offer_id),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,id));
ALTER TABLE claims.order_origins ENABLE ROW LEVEL SECURITY;
ALTER TABLE claims.order_origins FORCE ROW LEVEL SECURITY;
REVOKE ALL ON claims.order_origins FROM PUBLIC;
GRANT SELECT,INSERT ON claims.order_origins TO commerce_claims_writer;
GRANT SELECT(offer_id,line_version,quantity,occurred_at) ON claims.events TO commerce_claims_writer;
GRANT SELECT(issued_at) ON claims.links TO commerce_claims_writer;
-- Private attribution helpers run in an authenticated merchant/buyer scope, not
-- the principal-less intake worker scope. No login gains new table privileges.
CREATE POLICY attribution_event_scope ON claims.events FOR SELECT TO commerce_claims_writer
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY claims_origin_owner ON claims.order_origins TO commerce_claims_writer USING(true) WITH CHECK(true);
COMMENT ON TABLE claims.order_origins IS 'internal/claims: immutable consumed cart origin to order association independent of live discounts; anonymous event version/post/session facts, no buyer identity or comment text. No claim FK so retention can erase source bindings.';
CREATE FUNCTION claims.capture_order_origins(p_tenant uuid,p_store uuid,p_owner uuid,p_order uuid,p_origins jsonb) RETURNS void
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 INSERT INTO claims.order_origins
 SELECT p_tenant,p_store,p_order,b.id,e.offer_id,e.line_version,e.session_id,src.source_object_id,e.occurred_at
 FROM jsonb_to_recordset(p_origins) x(bundle_id uuid,offer_id uuid,sku_id uuid,line_version bigint,quantity bigint)
 JOIN claims.bundles b ON b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=x.bundle_id AND b.owner_id=p_owner
 JOIN claims.lines l ON l.tenant_id=p_tenant AND l.store_id=p_store AND l.bundle_id=b.id AND l.offer_id=x.offer_id AND l.sku_id=x.sku_id
 JOIN claims.events e ON e.tenant_id=p_tenant AND e.store_id=p_store AND e.bundle_id=b.id AND e.offer_id=x.offer_id
  AND e.line_version=x.line_version AND e.outcome='ACCEPTED' AND x.quantity<=e.quantity
 LEFT JOIN claims.meta_intake i ON i.tenant_id=p_tenant AND i.store_id=p_store AND i.applied_event_id=e.id AND i.inbox_event_id=e.source_event_id AND i.state='APPLIED'
 LEFT JOIN live.claim_sources src ON src.tenant_id=p_tenant AND src.store_id=p_store AND src.id=i.source_id
 ON CONFLICT DO NOTHING
$$;
ALTER FUNCTION claims.capture_order_origins(uuid,uuid,uuid,uuid,jsonb) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.capture_order_origins(uuid,uuid,uuid,uuid,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.capture_order_origins(uuid,uuid,uuid,uuid,jsonb) TO commerce_checkout_writer;
GRANT USAGE ON SCHEMA claims TO commerce_checkout_writer;
COMMENT ON FUNCTION claims.capture_order_origins(uuid,uuid,uuid,uuid,jsonb) IS 'internal/checkout same creation transaction only: scoped server cart origins proved against exact accepted claim event, no runtime caller or client fields; price-neutral.';

-- Domain-owned helper reads exact frozen comment evidence; no runtime table grants.
CREATE FUNCTION claims.order_comment_posts(p_tenant uuid,p_store uuid,p_order uuid)
RETURNS TABLE(post_id text,occurred_at timestamptz,session_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT DISTINCT x.post_id,x.occurred_at,x.session_id FROM claims.order_origins x
 WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.order_id=p_order AND x.post_id IS NOT NULL
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
  AND x.id=p_order AND x.creator_session_id=s.session_id
  -- begin_hold uses a PL/pgSQL exception subtransaction. Its xmin can differ
  -- from the top-level xid; the creating backend retains that subxid lock
  -- until commit. A committed historical order cannot pass this test.
  AND EXISTS(SELECT 1 FROM pg_catalog.pg_locks l WHERE l.pid=pg_backend_pid()
   AND l.locktype='transactionid' AND l.mode='ExclusiveLock' AND l.granted AND l.transactionid=x.xmin);
 IF NOT FOUND THEN RAISE EXCEPTION 'attribution order unavailable' USING ERRCODE='PT404'; END IF;
 IF EXISTS(SELECT 1 FROM orders.order_attribution a WHERE a.order_id=p_order) THEN RETURN; END IF;
 PERFORM claims.capture_order_origins(s.tenant_id,p_store,s.owner_id,p_order,
  coalesce((SELECT jsonb_agg(jsonb_build_object('bundle_id',l.claim_bundle_id,'offer_id',l.claim_offer_id,'sku_id',l.sku_id,
   'line_version',l.claim_line_version,'quantity',l.quantity)) FROM storefront.cart_lines l
   WHERE l.tenant_id=s.tenant_id AND l.store_id=p_store AND l.owner_id=s.owner_id AND l.cart_id=o.cart_id AND l.claim_line_version IS NOT NULL),'[]'::jsonb));
 -- Invalid/foreign measurement is ignored; it cannot deny a legitimate order or change its financial snapshot.
 IF jsonb_typeof(p_touch)='object' AND (p_touch->>'draft_id') ~ '^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$' THEN
  BEGIN click_at:=(p_touch->>'clicked_at')::timestamptz; EXCEPTION WHEN invalid_datetime_format OR datetime_field_overflow THEN click_at:=NULL; END;
  IF click_at BETWEEN clock_timestamp()-interval '7 days' AND clock_timestamp() THEN
   SELECT m.draft_id INTO d FROM ads.attribution_match(s.tenant_id,p_store,(p_touch->>'draft_id')::uuid,NULL,NULL) m;
   IF d IS NOT NULL THEN
    v_path:='ad_click';
    IF p_touch->>'fbc' ~ '^fb\.1\.[1-9][0-9]{0,15}\.[A-Za-z0-9_-]+$' AND char_length(split_part(p_touch->>'fbc','.',4))<=500 THEN v_fbc:=p_touch->>'fbc'; END IF;
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

CREATE FUNCTION claims.attribution_session_orders(p_tenant uuid,p_store uuid,p_session uuid) RETURNS SETOF uuid
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT DISTINCT u.order_id FROM claims.order_origins u
 WHERE u.tenant_id=p_tenant AND u.store_id=p_store AND u.session_id=p_session
$$;
ALTER FUNCTION claims.attribution_session_orders(uuid,uuid,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.attribution_session_orders(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.attribution_session_orders(uuid,uuid,uuid) TO commerce_checkout_writer;
COMMENT ON FUNCTION claims.attribution_session_orders(uuid,uuid,uuid) IS 'internal/attribution D9: consumed claim order IDs of exact session, no post fan-out, no actor/line data; checkout aggregate definer only.';

CREATE FUNCTION orders.attribution_metrics(p_tenant uuid,p_store uuid,p_from date,p_to date,p_draft uuid,p_session uuid) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 WITH cohort AS MATERIALIZED (
 SELECT o.id,o.owner_id,o.created_at,o.currency,o.snapshot,a.path,a.draft_id,a.post_id,
  CASE WHEN o.payment_mode='card' THEN coalesce(f.captured,0)-coalesce(f.refunded,0)
   WHEN o.payment_mode IN ('pay_at_pickup','cash_on_delivery') AND o.collection_state='COLLECTED' THEN o.total_minor+coalesce(o.cod_surcharge_minor,0)
   WHEN o.payment_mode='bank_transfer' AND bt.state='CONFIRMED' THEN o.total_minor ELSE 0 END::bigint AS net_minor,
  (coalesce(f.captured,0)>0 OR o.collected_at IS NOT NULL OR bt.confirmed_at IS NOT NULL) AS paid,
  (o.collection_state='PENDING' AND o.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION')) IS TRUE AS pending,
  o.total_minor+coalesce(o.cod_surcharge_minor,0) AS due,
  coalesce(
   substring(btrim(o.snapshot#>>'{destination,home_address,region}') FROM '^(臺北市|台北市|新北市|桃園市|臺中市|台中市|臺南市|台南市|高雄市|基隆市|新竹市|嘉義市|新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|臺東縣|台東縣|澎湖縣|金門縣|連江縣)$'),
   substring(btrim(o.snapshot#>>'{destination,home_address,city}') FROM '^(臺北市|台北市|新北市|桃園市|臺中市|台中市|臺南市|台南市|高雄市|基隆市|新竹市|嘉義市|新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|臺東縣|台東縣|澎湖縣|金門縣|連江縣)$'),
   substring(o.snapshot#>>'{destination,pickup,address}' FROM '^(臺北市|台北市|新北市|桃園市|臺中市|台中市|臺南市|台南市|高雄市|基隆市|新竹市|嘉義市|新竹縣|苗栗縣|彰化縣|南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|臺東縣|台東縣|澎湖縣|金門縣|連江縣)'), '—') AS county,
  EXISTS(SELECT 1 FROM checkout.orders old WHERE old.tenant_id=o.tenant_id AND old.store_id=o.store_id AND old.owner_id=o.owner_id
   AND (old.created_at,old.id)<(o.created_at,o.id) AND (old.commercial_state='CONFIRMED' OR old.collected_at IS NOT NULL)) AS is_returning
 FROM checkout.orders o LEFT JOIN orders.order_attribution a ON a.tenant_id=o.tenant_id AND a.store_id=o.store_id AND a.order_id=o.id
 LEFT JOIN checkout.bank_transfers bt ON bt.tenant_id=o.tenant_id AND bt.store_id=o.store_id AND bt.order_id=o.id
 LEFT JOIN LATERAL (SELECT
  (SELECT sum(x.amount_minor) FROM checkout.payment_attempts p JOIN payments.facts x ON x.tenant_id=p.tenant_id AND x.store_id=p.store_id AND x.attempt_id=p.id AND x.kind='CAPTURED'
   WHERE p.tenant_id=o.tenant_id AND p.store_id=o.store_id AND p.order_id=o.id) AS captured,
  (SELECT sum(x.amount_minor) FROM checkout.payment_attempts p JOIN payments.refund_facts x ON x.tenant_id=p.tenant_id AND x.store_id=p.store_id AND x.attempt_id=p.id AND x.kind='SUCCEEDED'
   WHERE p.tenant_id=o.tenant_id AND p.store_id=o.store_id AND p.order_id=o.id) AS refunded) f ON true
 WHERE o.tenant_id=p_tenant AND o.store_id=p_store
  AND o.created_at>=p_from::timestamp AT TIME ZONE 'Asia/Taipei' AND o.created_at<(p_to+1)::timestamp AT TIME ZONE 'Asia/Taipei'
  AND ((p_draft IS NOT NULL AND a.draft_id=p_draft) OR (p_session IS NOT NULL AND o.id IN
   (SELECT * FROM claims.attribution_session_orders(p_tenant,p_store,p_session))))
 ), counts AS (
 SELECT count(*)::bigint orders,coalesce(sum(net_minor),0)::bigint net_minor,count(*) FILTER(WHERE pending)::bigint pending_orders,
  coalesce(sum(due) FILTER(WHERE pending),0)::bigint pending_minor,count(*) FILTER(WHERE paid)::bigint paid_orders,
  count(*) FILTER(WHERE path='boosted_post' AND draft_id IS NULL)::bigint ambiguous_orders FROM cohort
 ), customer_groups AS (SELECT owner_id,bool_or(is_returning) AS is_returning FROM cohort WHERE paid GROUP BY owner_id)
 SELECT to_jsonb(counts)||jsonb_build_object(
 'paths',coalesce((SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT path,count(*)::bigint orders,sum(net_minor)::bigint net_minor,
  count(*) FILTER(WHERE pending)::bigint pending_orders,coalesce(sum(due) FILTER(WHERE pending),0)::bigint pending_minor
  FROM cohort WHERE path IS NOT NULL GROUP BY path ORDER BY path) x),'[]'::jsonb),
 'buyers',jsonb_build_object(
  'counties',coalesce((SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT county AS name,count(*)::bigint orders,sum(net_minor)::bigint net_minor FROM cohort WHERE paid GROUP BY county ORDER BY count(*) DESC,county) x),'[]'::jsonb),
  'new_buyers',(SELECT count(*) FROM customer_groups WHERE NOT is_returning),'returning_buyers',(SELECT count(*) FROM customer_groups WHERE is_returning),
  'average_order_minor',(SELECT CASE WHEN count(*) FILTER(WHERE paid)>0 THEN round(sum(net_minor)::numeric/(count(*) FILTER(WHERE paid)))::bigint ELSE NULL END FROM cohort),
  'top_products',coalesce((SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT line->>'product_id' AS product_id,min(line->>'name') AS name,sum((line->>'quantity')::bigint)::bigint quantity
   FROM cohort CROSS JOIN LATERAL jsonb_array_elements(snapshot#>'{quote,lines}') line WHERE paid GROUP BY line->>'product_id' ORDER BY sum((line->>'quantity')::bigint) DESC,line->>'product_id' LIMIT 20) x),'[]'::jsonb),
  'orders_per_minute',coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY at) FROM (SELECT date_trunc('minute',created_at) AS at,count(*)::bigint orders,sum(net_minor)::bigint net_minor FROM cohort GROUP BY date_trunc('minute',created_at)) x),'[]'::jsonb))) FROM counts
$$;
ALTER FUNCTION orders.attribution_metrics(uuid,uuid,date,date,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION orders.attribution_metrics(uuid,uuid,date,date,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orders.attribution_metrics(uuid,uuid,date,date,uuid,uuid) TO commerce_ads_writer;
COMMENT ON FUNCTION orders.attribution_metrics(uuid,uuid,date,date,uuid,uuid) IS 'internal/ads D1/D6/D9 narrow aggregate; Taipei order-created cohort with all known refunds, COD pending separate. No buyer identity/phone/address/age/gender exposed; only county, new/returning and purchased product aggregates.';

CREATE FUNCTION claims.attribution_funnel(p_tenant uuid,p_store uuid,p_session uuid,p_from date,p_to date) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 WITH comments AS MATERIALIZED (SELECT i.occurred_at FROM claims.meta_intake i WHERE i.tenant_id=p_tenant AND i.store_id=p_store AND i.session_id=p_session
  AND i.occurred_at>=p_from::timestamp AT TIME ZONE 'Asia/Taipei' AND i.occurred_at<(p_to+1)::timestamp AT TIME ZONE 'Asia/Taipei'),
 accepted AS MATERIALIZED (SELECT e.occurred_at,e.bundle_id FROM claims.events e WHERE e.tenant_id=p_tenant AND e.store_id=p_store AND e.session_id=p_session AND e.outcome='ACCEPTED'
  AND e.occurred_at>=p_from::timestamp AT TIME ZONE 'Asia/Taipei' AND e.occurred_at<(p_to+1)::timestamp AT TIME ZONE 'Asia/Taipei')
 SELECT jsonb_build_object('comments',(SELECT count(*) FROM comments),'claims',(SELECT count(*) FROM accepted),
  'checkout_links',(SELECT count(*) FROM claims.links l JOIN claims.bundles b ON b.tenant_id=l.tenant_id AND b.store_id=l.store_id AND b.id=l.bundle_id
   WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.session_id=p_session AND l.issued_at>=p_from::timestamp AT TIME ZONE 'Asia/Taipei' AND l.issued_at<(p_to+1)::timestamp AT TIME ZONE 'Asia/Taipei'),
  'timeline',coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY at) FROM (SELECT at,sum(comments)::bigint comments,sum(claims)::bigint claims FROM
   (SELECT date_trunc('minute',occurred_at) at,1 comments,0 claims FROM comments UNION ALL SELECT date_trunc('minute',occurred_at),0,1 FROM accepted) m GROUP BY at) x),'[]'::jsonb))
$$;
ALTER FUNCTION claims.attribution_funnel(uuid,uuid,uuid,date,date) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.attribution_funnel(uuid,uuid,uuid,date,date) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.attribution_funnel(uuid,uuid,uuid,date,date) TO commerce_ads_writer;
COMMENT ON FUNCTION claims.attribution_funnel(uuid,uuid,uuid,date,date) IS 'internal/ads D9 session-only aggregate of received comment intake, accepted claim events and current issued checkout links; no actor identifiers or message text.';

CREATE FUNCTION claims.attribution_sources(p_tenant uuid,p_store uuid,p_session uuid)
RETURNS TABLE(id uuid,session_id uuid,binding_id uuid,asset_id text,source_object_id text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT s.id,s.session_id,s.binding_id,s.asset_id,s.source_object_id FROM live.claim_sources s
 WHERE s.tenant_id=p_tenant AND s.store_id=p_store AND s.session_id=p_session AND s.object='page' AND s.active
$$;
ALTER FUNCTION claims.attribution_sources(uuid,uuid,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.attribution_sources(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.attribution_sources(uuid,uuid,uuid) TO commerce_media_writer,commerce_integration_writer,commerce_ads_writer;
GRANT USAGE ON SCHEMA claims TO commerce_media_writer,commerce_integration_writer;
COMMENT ON FUNCTION claims.attribution_sources(uuid,uuid,uuid) IS 'internal/ads D9 narrow source projection for aggregate reads only; domain owner retains table rights; exact tenant/store/session and active Page sources.';

CREATE FUNCTION live.attribution_sessions(p_tenant uuid,p_store uuid,p_from date,p_to date)
RETURNS TABLE(session_id uuid,title text,starts_at timestamptz,ends_at timestamptz,post_ids text[])
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT s.id,s.title,coalesce(s.scheduled_at,s.created_at),NULL::timestamptz,
  ARRAY(SELECT DISTINCT src.source_object_id FROM claims.attribution_sources(p_tenant,p_store,s.id) src ORDER BY src.source_object_id)
 FROM live.sessions s WHERE s.tenant_id=p_tenant AND s.store_id=p_store
  AND coalesce(s.scheduled_at,s.created_at)>=p_from::timestamp AT TIME ZONE 'Asia/Taipei'
  AND coalesce(s.scheduled_at,s.created_at)<(p_to+1)::timestamp AT TIME ZONE 'Asia/Taipei'
 ORDER BY coalesce(s.scheduled_at,s.created_at) DESC,s.id LIMIT 100
$$;
ALTER FUNCTION live.attribution_sessions(uuid,uuid,date,date) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.attribution_sessions(uuid,uuid,date,date) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.attribution_sessions(uuid,uuid,date,date) TO commerce_ads_writer;
GRANT USAGE ON SCHEMA claims,live TO commerce_ads_writer;
COMMENT ON FUNCTION live.attribution_sessions(uuid,uuid,date,date) IS 'internal/ads D9: at most 100 scoped live sessions with bound public post IDs, not stream keys or credentials. Unknown end stays null.';

CREATE FUNCTION ads.attribution_report(p_hash bytea,p_store uuid,p_from date,p_to date) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; d record; s record; metrics jsonb; drafts jsonb:='[]'; sessions jsonb:='[]'; bd jsonb; mr record;
 f jsonb; ids uuid[]; timeline jsonb; v_currency text;
BEGIN
 SELECT * INTO a FROM ads.auth(p_hash,p_store,ARRAY['ads:read']);
 IF p_from IS NULL OR p_to IS NULL OR p_to<p_from OR p_to-p_from>91 THEN PERFORM ads.deny('invalid_request'); END IF;
 FOR d IN SELECT * FROM ads.campaign_drafts x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store ORDER BY x.created_at DESC,x.id LIMIT 100 LOOP
  metrics:=orders.attribution_metrics(a.out_tenant,p_store,p_from,p_to,d.id,NULL);
  SELECT coalesce(sum(i.spend_minor),0)::bigint AS spend,sum(i.meta_purchases)::bigint AS purchases,sum(i.meta_purchase_value_minor)::bigint AS value,
   CASE WHEN count(DISTINCT i.account_timezone)=1 THEN min(i.account_timezone) WHEN count(*)>0 THEN 'mixed' ELSE NULL END tz,
   coalesce(bool_or(i.day>=(clock_timestamp() AT TIME ZONE i.account_timezone)::date-2),false) provisional
  INTO mr FROM ads.insights_daily i WHERE i.tenant_id=a.out_tenant AND i.store_id=p_store AND i.draft_id=d.id AND i.currency=d.currency AND i.day BETWEEN p_from AND p_to;
  SELECT coalesce(jsonb_agg(to_jsonb(i)-'tenant_id'-'store_id'-'draft_id'-'source_operation_id'-'fetched_at' ORDER BY i.day,i.dimension,i.bucket),'[]'::jsonb)
   INTO bd FROM ads.insights_breakdowns i WHERE i.tenant_id=a.out_tenant AND i.store_id=p_store AND i.draft_id=d.id AND i.day BETWEEN p_from AND p_to;
  drafts:=drafts||jsonb_build_array(jsonb_build_object('draft_id',d.id,'source_ref',d.source_ref,'template',d.template,'currency',d.currency,
   'meta_account_timezone',mr.tz,'spend_minor',mr.spend,'orders',metrics->'paths','meta',jsonb_build_object('purchases',mr.purchases,'purchase_value_minor',mr.value),
   'roas',CASE WHEN mr.spend>0 THEN round((metrics->>'net_minor')::numeric/mr.spend,4) ELSE NULL END,
   'provisional',mr.provisional,'breakdowns',bd,'buyers',metrics->'buyers'));
 END LOOP;
 SELECT coalesce((SELECT st.currency FROM control.stores st WHERE st.tenant_id=a.out_tenant AND st.id=p_store),'TWD') INTO v_currency;
 FOR s IN SELECT * FROM live.attribution_sessions(a.out_tenant,p_store,p_from,p_to) LOOP
  SELECT array_agg(x.id) INTO ids FROM ads.campaign_drafts x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.template='BOOST_POST' AND x.source_ref=ANY(s.post_ids);
  metrics:=orders.attribution_metrics(a.out_tenant,p_store,p_from,p_to,NULL,s.session_id);
  f:=claims.attribution_funnel(a.out_tenant,p_store,s.session_id,p_from,p_to);
  SELECT coalesce(sum(i.spend_minor),0)::bigint spend INTO mr FROM ads.insights_daily i
   WHERE i.tenant_id=a.out_tenant AND i.store_id=p_store AND i.draft_id=ANY(ids) AND i.currency=v_currency AND i.day BETWEEN p_from AND p_to;
  SELECT coalesce(jsonb_agg(to_jsonb(q) ORDER BY at),'[]'::jsonb) INTO timeline FROM
   (SELECT at,sum(spend_minor)::bigint spend_minor,sum(orders)::bigint orders,sum(net_minor)::bigint net_minor,sum(comments)::bigint comments,sum(claims)::bigint claims,NULL::bigint viewers
    FROM (SELECT i.hour_start at,i.spend_minor,0::bigint orders,0::bigint net_minor,0::bigint comments,0::bigint claims FROM ads.insights_breakdowns i
      WHERE i.tenant_id=a.out_tenant AND i.store_id=p_store AND i.draft_id=ANY(ids) AND i.currency=v_currency AND i.day BETWEEN p_from AND p_to AND i.dimension='hourly'
     UNION ALL SELECT x.at,0,x.orders,x.net_minor,0,0 FROM jsonb_to_recordset(metrics#>'{buyers,orders_per_minute}') x(at timestamptz,orders bigint,net_minor bigint)
     UNION ALL SELECT x.at,0,0,0,x.comments,x.claims FROM jsonb_to_recordset(f->'timeline') x(at timestamptz,comments bigint,claims bigint)) z GROUP BY at) q;
  sessions:=sessions||jsonb_build_array(jsonb_build_object('session_id',s.session_id,'title',s.title,'starts_at',s.starts_at,'ends_at',s.ends_at,
   'post_ids',to_jsonb(s.post_ids),'draft_ids',to_jsonb(coalesce(ids,'{}'::uuid[])),'currency',v_currency,'spend_minor',mr.spend,
   'orders',metrics->'orders','net_minor',metrics->'net_minor','pending_orders',metrics->'pending_orders','pending_minor',metrics->'pending_minor',
   'ambiguous_orders',metrics->'ambiguous_orders','funnel',(f-'timeline')||jsonb_build_object('paid_orders',metrics->'paid_orders'),
   'buyers',metrics->'buyers','timeline',timeline,
   'live_audience',jsonb_build_object('status','not_authorized','views',NULL,'peak_concurrent',NULL,'total_view_time_ms',NULL,'age_gender','[]'::jsonb,'regions','[]'::jsonb)));
 END LOOP;
 RETURN jsonb_build_object('window',jsonb_build_object('from',p_from,'to',p_to),'order_timezone','Asia/Taipei','drafts',drafts,'sessions',sessions);
END $$;
ALTER FUNCTION ads.attribution_report(bytea,uuid,date,date) OWNER TO commerce_ads_writer;
REVOKE ALL ON FUNCTION ads.attribution_report(bytea,uuid,date,date) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ads.attribution_report(bytea,uuid,date,date) TO commerce_runtime;
COMMENT ON FUNCTION ads.attribution_report(bytea,uuid,date,date) IS 'internal/ads GET attribution: auth ads:read, bounded dates, factual local aggregates and Meta account-day snapshots never blended; session hourly timestamps already absolute. No provider call or mutation.';

-- A merchant may request a Page aggregate READ; only the existing claims worker
-- opens Page credentials. The ads worker never gains Page token custody.
GRANT USAGE ON SCHEMA ads TO commerce_integration_writer;
GRANT EXECUTE ON FUNCTION ads.auth(bytea,uuid,text[]) TO commerce_integration_writer;
CREATE TABLE ads.live_audience_snapshots (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,session_id uuid NOT NULL,source_id uuid NOT NULL,
 operation_id uuid NOT NULL REFERENCES integration.operations(id),snapshot jsonb NOT NULL CHECK(jsonb_typeof(snapshot)='object' AND octet_length(snapshot::text)<=65536),
 requested_at timestamptz NOT NULL,fetched_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(tenant_id,store_id,session_id),
 FOREIGN KEY(tenant_id,store_id,session_id) REFERENCES live.sessions(tenant_id,store_id,id));
ALTER TABLE ads.live_audience_snapshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE ads.live_audience_snapshots FORCE ROW LEVEL SECURITY;
REVOKE ALL ON ads.live_audience_snapshots FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE ON ads.live_audience_snapshots TO commerce_ads_writer;
CREATE POLICY ads_audience_owner ON ads.live_audience_snapshots TO commerce_ads_writer USING(true) WITH CHECK(true);
COMMENT ON TABLE ads.live_audience_snapshots IS 'internal/ads D9 latest bounded aggregate for an exactly bound video; no buyer or person-level demographic join; only lease-fenced completion writes.';
CREATE POLICY audience_plan_insert ON integration.operations FOR INSERT TO commerce_integration_writer
 WITH CHECK (state='READY' AND generation=0 AND actor_kind='MERCHANT' AND provider='facebook'
  AND purpose='service' AND action='meta.live_insights');

CREATE FUNCTION integration.plan_meta_audience(p_hash bytea,p_store uuid,p_session uuid,p_operation uuid,p_job bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;s record;b record;j record;n integer;req jsonb;
BEGIN
 SELECT * INTO a FROM ads.auth(p_hash,p_store,ARRAY['ads:read','live:read']);
 SELECT count(*) INTO n FROM claims.attribution_sources(a.out_tenant,p_store,p_session);
 IF n<>1 THEN RAISE EXCEPTION 'source_not_owned' USING ERRCODE='AD422'; END IF;
 SELECT * INTO s FROM claims.attribution_sources(a.out_tenant,p_store,p_session);
 SELECT * INTO b FROM integration.bindings x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.id=s.binding_id FOR SHARE;
 IF NOT FOUND OR NOT b.enabled OR b.provider<>'facebook' OR b.external_asset_id<>s.asset_id THEN RAISE EXCEPTION 'source_not_owned' USING ERRCODE='AD422'; END IF;
 IF NOT EXISTS(SELECT 1 FROM integration.meta_page_heads h JOIN integration.meta_page_credentials c
  ON c.tenant_id=h.tenant_id AND c.store_id=h.store_id AND c.binding_id=h.binding_id AND c.version=h.current_version
  WHERE h.tenant_id=a.out_tenant AND h.store_id=p_store AND h.binding_id=b.id
   AND c.scopes_attested @> ARRAY['read_insights','pages_read_engagement']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='AD403'; END IF;
 SELECT * INTO j FROM river.river_job x WHERE x.id=p_job AND x.kind='external_operation_v1' AND x.queue='default'
  AND x.args=jsonb_build_object('operation_id',p_operation::text,'version',1) AND x.xmin=pg_current_xact_id()::xid;
 IF NOT FOUND THEN RAISE EXCEPTION 'live audience job mismatch' USING ERRCODE='22023'; END IF;
 req:=jsonb_build_object('v',1,'source_id',s.id,'session_id',p_session,'post_id',s.source_object_id,'asset_id',s.asset_id);
 INSERT INTO integration.operations(tenant_id,store_id,id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id)
 VALUES(a.out_tenant,p_store,p_operation,a.out_principal,b.id,b.semantic_version,'facebook',b.external_asset_id,'service','meta.live_insights',
  'audience:'||p_operation,sha256(convert_to(req::text,'UTF8')),req,p_job);
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
 VALUES(a.out_tenant,p_store,p_operation,0,'READY','','operation_planned');
 RETURN jsonb_build_object('operation_id',p_operation,'state','READY');
END $$;
ALTER FUNCTION integration.plan_meta_audience(bytea,uuid,uuid,uuid,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.plan_meta_audience(bytea,uuid,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.plan_meta_audience(bytea,uuid,uuid,uuid,bigint) TO commerce_runtime;
COMMENT ON FUNCTION integration.plan_meta_audience(bytea,uuid,uuid,uuid,bigint) IS 'internal/ads merchant aggregate-read plan: fresh ads:read/live:read auth, one bound Page source, attested read scopes, default lane same-transaction job. No Meta mutation or buyer fields.';

CREATE FUNCTION integration.check_meta_audience(p_operation uuid) RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations;
BEGIN
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation AND x.provider='facebook' AND x.action='meta.live_insights' AND x.purpose='service' AND x.actor_kind='MERCHANT';
 IF NOT FOUND THEN RETURN 'invalid_request'; END IF;
 IF NOT identity.principal_holds(o.tenant_id,o.store_id,o.principal_id,ARRAY['ads:read','live:read']) THEN RETURN 'principal_revoked'; END IF;
 IF NOT EXISTS(SELECT 1 FROM claims.attribution_sources(o.tenant_id,o.store_id,(o.request->>'session_id')::uuid) s WHERE s.id::text=o.request->>'source_id'
  AND s.binding_id=o.binding_id AND s.source_object_id=o.request->>'post_id')
  THEN RETURN 'source_changed'; END IF;
 RETURN '';
END $$;
ALTER FUNCTION integration.check_meta_audience(uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.check_meta_audience(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.check_meta_audience(uuid) TO commerce_claims_worker;
COMMENT ON FUNCTION integration.check_meta_audience(uuid) IS 'internal/integrations/metareply audience Check: principal and exact bound source recheck before Page aggregate GET; claims worker only.';

CREATE FUNCTION integration.load_meta_audience_token(p_operation uuid,p_generation bigint,p_token bytea)
RETURNS TABLE(tenant_id uuid,store_id uuid,binding_id uuid,provider text,asset_id text,version bigint,key_id text,nonce bytea,ciphertext bytea,scopes_attested text[])
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations;b integration.bindings;
BEGIN
 IF p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32 THEN RAISE EXCEPTION 'invalid audience lease' USING ERRCODE='22023'; END IF;
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation AND x.action='meta.live_insights' AND x.provider='facebook';
 IF NOT FOUND THEN RAISE EXCEPTION 'audience unavailable' USING ERRCODE='P0002'; END IF;
 SELECT * INTO b FROM integration.bindings x WHERE x.id=o.binding_id AND x.tenant_id=o.tenant_id AND x.store_id=o.store_id FOR SHARE;
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation FOR SHARE;
 IF o.state<>'DISPATCHING' OR o.lease_mode<>'dispatch' OR o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) THEN RAISE EXCEPTION 'audience lease conflict' USING ERRCODE='40001'; END IF;
 IF b.id IS NULL OR NOT b.enabled OR b.provider<>'facebook' OR b.external_asset_id<>o.external_asset_id OR integration.check_meta_audience(p_operation)<>'' THEN RETURN; END IF;
 RETURN QUERY SELECT c.tenant_id,c.store_id,c.binding_id,c.provider,c.asset_id,c.version,c.key_id,c.nonce,c.ciphertext,c.scopes_attested
 FROM integration.meta_page_heads h JOIN integration.meta_page_credentials c ON c.tenant_id=h.tenant_id AND c.store_id=h.store_id AND c.binding_id=h.binding_id AND c.version=h.current_version
 WHERE h.tenant_id=o.tenant_id AND h.store_id=o.store_id AND h.binding_id=o.binding_id AND c.provider='facebook' AND c.asset_id=o.external_asset_id
  AND c.scopes_attested @> ARRAY['read_insights','pages_read_engagement'];
END $$;
ALTER FUNCTION integration.load_meta_audience_token(uuid,bigint,bytea) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.load_meta_audience_token(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.load_meta_audience_token(uuid,bigint,bytea) TO commerce_claims_worker;
COMMENT ON FUNCTION integration.load_meta_audience_token(uuid,bigint,bytea) IS 'internal/integrations/metareply read-only audience route: current scoped Page ciphertext, exact dispatch lease and read_insights/pages_read_engagement; never private_reply or ads-worker access.';

CREATE FUNCTION integration.finish_meta_audience(p_operation uuid,p_generation bigint,p_token bytea,p_mode text,p_result jsonb) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations;
BEGIN
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation AND x.action='meta.live_insights' AND x.provider='facebook' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'audience unavailable' USING ERRCODE='P0002'; END IF;
 IF p_token IS NULL OR octet_length(p_token)<>32 OR p_mode IS DISTINCT FROM 'dispatch' OR o.generation IS DISTINCT FROM p_generation
  OR o.lease_mode IS DISTINCT FROM p_mode OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) OR o.state<>'DISPATCHING' THEN RAISE EXCEPTION 'audience lease conflict' USING ERRCODE='40001'; END IF;
 IF integration.check_meta_audience(p_operation)<>'' THEN RAISE EXCEPTION 'audience source changed' USING ERRCODE='42501'; END IF;
 PERFORM 1 FROM integration.load_meta_audience_token(p_operation,p_generation,p_token);
 IF NOT FOUND THEN RAISE EXCEPTION 'audience permission revoked' USING ERRCODE='42501'; END IF;
 PERFORM ads.store_live_audience_snapshot(o.tenant_id,o.store_id,(o.request->>'session_id')::uuid,(o.request->>'source_id')::uuid,o.id,o.created_at,p_result);
END $$;
ALTER FUNCTION integration.finish_meta_audience(uuid,bigint,bytea,text,jsonb) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.finish_meta_audience(uuid,bigint,bytea,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.finish_meta_audience(uuid,bigint,bytea,text,jsonb) TO commerce_claims_worker;
COMMENT ON FUNCTION integration.finish_meta_audience(uuid,bigint,bytea,text,jsonb) IS 'internal/integrations/metareply Finish in completion transaction; aggregate-only latest snapshot, exact lease, no individual linkage.';

CREATE FUNCTION ads.store_live_audience_snapshot(p_tenant uuid,p_store uuid,p_session uuid,p_source uuid,p_operation uuid,p_requested_at timestamptz,p_result jsonb) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF jsonb_typeof(p_result) IS DISTINCT FROM 'object' OR coalesce(p_result->>'status','') NOT IN ('available','insufficient')
  OR jsonb_typeof(p_result->'age_gender') IS DISTINCT FROM 'array' OR jsonb_typeof(p_result->'regions') IS DISTINCT FROM 'array'
  OR jsonb_array_length(p_result->'age_gender')>200 OR jsonb_array_length(p_result->'regions')>200 THEN RAISE EXCEPTION 'invalid audience result' USING ERRCODE='22023'; END IF;
 INSERT INTO ads.live_audience_snapshots(tenant_id,store_id,session_id,source_id,operation_id,requested_at,snapshot)
 VALUES(p_tenant,p_store,p_session,p_source,p_operation,p_requested_at,p_result)
 ON CONFLICT(tenant_id,store_id,session_id) DO UPDATE SET source_id=EXCLUDED.source_id,operation_id=EXCLUDED.operation_id,requested_at=EXCLUDED.requested_at,snapshot=EXCLUDED.snapshot,fetched_at=clock_timestamp()
 WHERE ads.live_audience_snapshots.requested_at<=EXCLUDED.requested_at;
END $$;
ALTER FUNCTION ads.store_live_audience_snapshot(uuid,uuid,uuid,uuid,uuid,timestamptz,jsonb) OWNER TO commerce_ads_writer;
REVOKE ALL ON FUNCTION ads.store_live_audience_snapshot(uuid,uuid,uuid,uuid,uuid,timestamptz,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ads.store_live_audience_snapshot(uuid,uuid,uuid,uuid,uuid,timestamptz,jsonb) TO commerce_integration_writer;
COMMENT ON FUNCTION ads.store_live_audience_snapshot(uuid,uuid,uuid,uuid,uuid,timestamptz,jsonb) IS 'internal/integrations lease-fenced private aggregate persistence seam; no runtime table access, stale overlapping read cannot overwrite newer result.';

-- Patch only the audience projection, keeping the already checked report shape.
DO $patch$
DECLARE body text;needle text:='jsonb_build_object(''status'',''not_authorized'',''views'',NULL,''peak_concurrent'',NULL,''total_view_time_ms'',NULL,''age_gender'',''[]''::jsonb,''regions'',''[]''::jsonb)';
BEGIN
 SELECT prosrc INTO body FROM pg_proc WHERE oid='ads.attribution_report(bytea,uuid,date,date)'::regprocedure;
 IF strpos(body,needle)=0 THEN RAISE EXCEPTION 'audience report patch shape changed'; END IF;
 body:=replace(body,needle,'coalesce((SELECT x.snapshot FROM ads.live_audience_snapshots x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.session_id=s.session_id AND EXISTS(SELECT 1 FROM claims.attribution_sources(a.out_tenant,p_store,s.session_id) src WHERE src.id=x.source_id)),'||needle||')');
 EXECUTE 'CREATE OR REPLACE FUNCTION ads.attribution_report(p_hash bytea,p_store uuid,p_from date,p_to date) RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS '||quote_literal(body);
END $patch$;

DO $comments$
DECLARE c record;
BEGIN
 FOR c IN SELECT table_schema,table_name,column_name FROM information_schema.columns
  WHERE (table_schema,table_name) IN (('orders','order_attribution'),('claims','order_origins'),('ads','insights_breakdowns'),('ads','live_audience_snapshots')) LOOP
  EXECUTE format('COMMENT ON COLUMN %I.%I.%I IS %L',c.table_schema,c.table_name,c.column_name,'internal/attribution: domain-owned scoped measurement; raw identifiers never exported, merchant reads aggregate projections only.');
 END LOOP;
END $comments$;
