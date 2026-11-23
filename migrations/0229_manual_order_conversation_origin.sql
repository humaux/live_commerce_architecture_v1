-- Purpose: retain one contextual order per Inbox revision and fence unsettled claim reservations.
-- Depends on: 0094 manual order marks, 0165 buyer panel and 0177 legacy claim reservation authority.
-- Used by: internal/merchanttools/manual_source.go and the A13 buyer-panel projection.
CREATE TABLE inbox.manual_order_sources (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 key_hash bytea NOT NULL CHECK(octet_length(key_hash)=32),
 principal_id uuid NOT NULL,
 conversation_id uuid NOT NULL,
 bundle_ids uuid[] NOT NULL CHECK(cardinality(bundle_ids)<=5 AND array_position(bundle_ids,NULL) IS NULL),
 revision text NOT NULL CHECK(revision ~ '^[0-9a-f]{64}$'),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','placed','released')),
 buyer_id uuid,
 claim_request_id uuid,
 begin_key text CHECK(begin_key ~ '^mt-[0-9a-f]{32}$'),
 price_reason text NOT NULL DEFAULT '',
 some_catalog boolean NOT NULL DEFAULT false,
 order_id uuid,
 placed_at timestamptz,
 PRIMARY KEY(tenant_id,store_id,key_hash),
 FOREIGN KEY(tenant_id,store_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,id),
 CHECK((state='placed')=(order_id IS NOT NULL AND placed_at IS NOT NULL)),
 CHECK(begin_key IS NULL OR buyer_id IS NOT NULL)
);
CREATE UNIQUE INDEX manual_order_source_revision ON inbox.manual_order_sources(tenant_id,store_id,conversation_id,revision) WHERE state<>'released';
CREATE UNIQUE INDEX manual_order_source_pending ON inbox.manual_order_sources(tenant_id,store_id,conversation_id) WHERE state='pending';
CREATE INDEX manual_order_source_orders ON inbox.manual_order_sources(tenant_id,store_id,conversation_id,placed_at DESC) WHERE state='placed';
ALTER TABLE inbox.manual_order_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.manual_order_sources FORCE ROW LEVEL SECURITY;
REVOKE ALL ON inbox.manual_order_sources FROM PUBLIC;
GRANT SELECT ON inbox.manual_order_sources TO commerce_runtime,commerce_claims_writer,commerce_checkout_writer,commerce_integration_writer;
GRANT INSERT(tenant_id,store_id,key_hash,principal_id,conversation_id,bundle_ids,revision),
 UPDATE(buyer_id,claim_request_id,begin_key,price_reason,some_catalog) ON inbox.manual_order_sources TO commerce_runtime;
GRANT UPDATE(state) ON inbox.manual_order_sources TO commerce_claims_writer;
GRANT UPDATE(state,order_id,placed_at) ON inbox.manual_order_sources TO commerce_checkout_writer;
GRANT USAGE ON SCHEMA inbox TO commerce_checkout_writer;
GRANT EXECUTE ON FUNCTION identity.principal_holds(uuid,uuid,uuid,text[]) TO commerce_checkout_writer;
CREATE POLICY manual_order_source_read ON inbox.manual_order_sources FOR SELECT TO commerce_runtime,commerce_claims_writer,commerce_checkout_writer,commerce_integration_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY manual_order_source_admit ON inbox.manual_order_sources FOR INSERT TO commerce_runtime
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
   AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid AND nullif(current_setting('app.buyer_id',true),'') IS NULL);
CREATE POLICY manual_order_source_update ON inbox.manual_order_sources FOR UPDATE TO commerce_runtime,commerce_claims_writer,commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
   AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL AND nullif(current_setting('app.buyer_id',true),'') IS NULL)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
COMMENT ON TABLE inbox.manual_order_sources IS 'M09/M10: scoped manual command admission and order provenance. Opaque IDs only. Reads join the live conversation, so erasure hides its pointer and preserves the command binding.';

CREATE FUNCTION claims.manual_order_context(p_conversation uuid,p_requested uuid[])
RETURNS TABLE(bundle_ids uuid[],order_ids uuid[],reason text,latest_order_id uuid)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; c record; x record; v_found uuid[]; v_waiting boolean; v_order uuid;
BEGIN
 v:=claims.for_buyer_scope();
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['orders:read','inventory:reserve','inbox:read'])
 THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT s.app_id,s.object,s.asset_id,s.peer_key INTO c FROM social.conversations s
  WHERE s.tenant_id=v[1] AND s.store_id=v[2] AND s.id=p_conversation;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
 IF p_requested IS NOT NULL THEN
  IF (SELECT count(*) FROM claims.bundles b WHERE b.tenant_id=v[1] AND b.store_id=v[2]
      AND b.id=ANY(p_requested) AND b.purged_at IS NULL)<>cardinality(p_requested)
  THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
  IF claims.for_buyer_peer_state(p_conversation,p_requested)='bundle_buyer_mismatch'
  THEN RAISE EXCEPTION 'bundle_buyer_mismatch' USING ERRCODE='PT409'; END IF;
 END IF;
 bundle_ids:='{}'; order_ids:='{}'; reason:='';
 SELECT m.order_id INTO latest_order_id FROM inbox.manual_order_sources m
  WHERE m.tenant_id=v[1] AND m.store_id=v[2] AND m.conversation_id=p_conversation AND m.state='placed'
  ORDER BY m.placed_at DESC,m.order_id DESC LIMIT 1;
 IF EXISTS(SELECT 1 FROM inbox.manual_order_sources m WHERE m.tenant_id=v[1] AND m.store_id=v[2]
   AND m.conversation_id=p_conversation AND m.state='pending') THEN
  reason:='source_order_pending'; RETURN NEXT; RETURN;
 END IF;
 FOR x IN
  SELECT DISTINCT b.id FROM inbox.bundle_peers p JOIN claims.bundles b
   ON b.tenant_id=p.tenant_id AND b.store_id=p.store_id AND b.id=p.bundle_id AND b.purged_at IS NULL
  WHERE p.tenant_id=v[1] AND p.store_id=v[2] AND p.app_id=c.app_id AND p.object=c.object AND p.asset_id=c.asset_id AND p.peer_key=c.peer_key
  AND EXISTS(SELECT 1 FROM claims.lines l WHERE l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id)
  ORDER BY b.id
 LOOP
  IF claims.for_buyer_peer_state(p_conversation,ARRAY[x.id])='bundle_buyer_mismatch'
  THEN reason:='bundle_buyer_mismatch'; RETURN NEXT; RETURN; END IF;
  SELECT coalesce(array_agg(o.id ORDER BY o.id),'{}') INTO v_found
   FROM claims.orders_of_bundles(v[1],v[2],ARRAY[x.id]) src
   JOIN checkout.orders o ON o.tenant_id=v[1] AND o.store_id=v[2] AND o.id=src.order_id AND o.commercial_state<>'CANCELLED';
  SELECT true,o.id INTO v_waiting,v_order FROM inbox.order_for_buyer r
   LEFT JOIN LATERAL (SELECT k.id FROM checkout.orders k WHERE k.tenant_id=r.tenant_id AND k.store_id=r.store_id
     AND k.owner_id=r.buyer_id AND k.commercial_state<>'CANCELLED' ORDER BY k.created_at DESC LIMIT 1) o ON true
   WHERE r.tenant_id=v[1] AND r.store_id=v[2] AND r.bundle_id=x.id AND r.state='pending'
    AND (r.create_attempt_id IS NOT NULL OR r.updated_at>=clock_timestamp()-interval '15 minutes' OR o.id IS NOT NULL
         OR EXISTS(SELECT 1 FROM inbox.manual_order_sources m WHERE m.tenant_id=r.tenant_id AND m.store_id=r.store_id
           AND m.key_hash=r.idempotency_key_hash AND m.state='pending'));
  IF v_waiting AND v_order IS NULL THEN
   IF x.id=ANY(p_requested) THEN RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409'; END IF;
   reason:='unresolved_bundle'; RETURN NEXT; RETURN;
  END IF;
  IF v_order IS NOT NULL AND NOT (v_order=ANY(v_found)) THEN v_found:=array_append(v_found,v_order); END IF;
  IF cardinality(v_found)>0 THEN
   IF x.id=ANY(p_requested) THEN
    RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409',DETAIL=CASE WHEN cardinality(v_found)=1 THEN v_found[1]::text ELSE '' END;
   END IF;
   order_ids:=ARRAY(SELECT DISTINCT id FROM unnest(order_ids||v_found) id ORDER BY id);
  ELSE
   bundle_ids:=array_append(bundle_ids,x.id);
   IF cardinality(bundle_ids)>5 THEN reason:='too_many_bundles'; RETURN NEXT; RETURN; END IF;
  END IF;
 END LOOP;
 RETURN NEXT;
END $$;
ALTER FUNCTION claims.manual_order_context(uuid,uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.manual_order_context(uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.manual_order_context(uuid,uuid[]) TO commerce_runtime;

CREATE FUNCTION claims.read_manual_order_source(p_key bytea,p_principal uuid,p_own boolean)
RETURNS TABLE(conversation_id uuid,bundle_ids uuid[])
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[];
BEGIN
 v:=claims.for_buyer_scope();
 IF v[3]<>p_principal OR NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['orders:read','inbox:read']) THEN RETURN; END IF;
 RETURN QUERY SELECT m.conversation_id,m.bundle_ids FROM inbox.manual_order_sources m JOIN social.conversations c
   ON c.tenant_id=m.tenant_id AND c.store_id=m.store_id AND c.id=m.conversation_id
  WHERE m.tenant_id=v[1] AND m.store_id=v[2] AND m.key_hash=p_key AND m.state='placed' AND (NOT p_own OR m.principal_id=v[3]);
END $$;
ALTER FUNCTION claims.read_manual_order_source(bytea,uuid,boolean) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.read_manual_order_source(bytea,uuid,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.read_manual_order_source(bytea,uuid,boolean) TO commerce_runtime;

CREATE FUNCTION claims.release_manual_order_source(p_key bytea) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; r record;
BEGIN
 v:=claims.for_buyer_scope();
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve','catalog:read','orders:read','inbox:read'])
 THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 UPDATE inbox.manual_order_sources m SET state='released'
  WHERE m.tenant_id=v[1] AND m.store_id=v[2] AND m.key_hash=p_key AND m.state='pending' AND m.begin_key IS NULL
  RETURNING m.claim_request_id INTO r;
 IF NOT FOUND THEN RETURN false; END IF;
 IF r.claim_request_id IS NOT NULL THEN
  UPDATE inbox.order_for_buyer SET state='released',updated_at=clock_timestamp()
   WHERE tenant_id=v[1] AND store_id=v[2] AND request_id=r.claim_request_id AND state='pending';
  UPDATE claims.merchant_origin_grants SET expires_at=clock_timestamp()
   WHERE tenant_id=v[1] AND store_id=v[2] AND request_id=r.claim_request_id AND consumed_at IS NULL;
 END IF;
 RETURN true;
END $$;
ALTER FUNCTION claims.release_manual_order_source(bytea) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.release_manual_order_source(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.release_manual_order_source(bytea) TO commerce_runtime;

CREATE FUNCTION fulfillment.manual_conversation_checkout(p_key bytea,p_begin text) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store uuid:=nullif(current_setting('app.store_id',true),'')::uuid;
 v_principal uuid:=nullif(current_setting('app.principal_id',true),'')::uuid;
 v_out jsonb;
BEGIN
 IF nullif(current_setting('app.buyer_id',true),'') IS NOT NULL OR NOT coalesce(
  identity.principal_holds(v_tenant,v_store,v_principal,ARRAY['inventory:reserve','catalog:read','orders:read','inbox:read']),false)
 THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT r.response || jsonb_build_object('total_minor',o.total_minor,'currency',o.currency) INTO v_out
 FROM inbox.manual_order_sources m JOIN checkout.command_results r
  ON r.tenant_id=m.tenant_id AND r.store_id=m.store_id AND r.owner_id=m.buyer_id AND r.operation='checkout.begin' AND r.idempotency_key=m.begin_key
 JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.id=r.order_id AND o.owner_id=r.owner_id
 WHERE m.tenant_id=v_tenant AND m.store_id=v_store AND m.key_hash=p_key AND m.begin_key=p_begin AND m.state IN ('pending','placed');
 RETURN v_out;
END $$;
ALTER FUNCTION fulfillment.manual_conversation_checkout(bytea,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.manual_conversation_checkout(bytea,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.manual_conversation_checkout(bytea,text) TO commerce_runtime;

CREATE FUNCTION fulfillment.finish_manual_conversation_order(p_key bytea,p_order uuid,p_link bytea) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE m inbox.manual_order_sources; v_now timestamptz:=clock_timestamp();
BEGIN
 SELECT * INTO m FROM inbox.manual_order_sources s
  WHERE s.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
    AND s.store_id=nullif(current_setting('app.store_id',true),'')::uuid AND s.key_hash=p_key AND s.state='pending'
  FOR UPDATE;
 IF NOT FOUND OR p_order IS NULL OR p_link IS NULL OR octet_length(p_link)<>32 OR m.begin_key IS NULL
   OR (fulfillment.manual_conversation_checkout(p_key,m.begin_key)->>'order_id') IS DISTINCT FROM p_order::text
 THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
 UPDATE checkout.orders SET source='merchant_manual'
  WHERE tenant_id=m.tenant_id AND store_id=m.store_id AND id=p_order AND owner_id=m.buyer_id AND source='storefront';
 INSERT INTO checkout.order_links(token_hash,tenant_id,store_id,owner_id,order_id,created_at,expires_at)
  VALUES(p_link,m.tenant_id,m.store_id,m.buyer_id,p_order,v_now,v_now+interval '7 days') ON CONFLICT(token_hash) DO NOTHING;
 UPDATE inbox.manual_order_sources SET state='placed',order_id=p_order,placed_at=v_now
  WHERE tenant_id=m.tenant_id AND store_id=m.store_id AND key_hash=p_key;
END $$;
ALTER FUNCTION fulfillment.finish_manual_conversation_order(bytea,uuid,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.finish_manual_conversation_order(bytea,uuid,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.finish_manual_conversation_order(bytea,uuid,bytea) TO commerce_runtime;

CREATE FUNCTION claims.lock_manual_order_bundles(p_tenant uuid,p_store uuid,p_bundles uuid[]) RETURNS void
LANGUAGE plpgsql VOLATILE SET search_path=pg_catalog AS $$
DECLARE b uuid;
BEGIN
 FOR b IN SELECT DISTINCT id FROM unnest(p_bundles) id ORDER BY 1 LOOP
  PERFORM pg_advisory_xact_lock(hashtextextended('claims.manual_order_bundle|'||p_tenant||'|'||p_store||'|'||b,0));
 END LOOP;
END $$;
ALTER FUNCTION claims.lock_manual_order_bundles(uuid,uuid,uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.lock_manual_order_bundles(uuid,uuid,uuid[]) FROM PUBLIC;

CREATE OR REPLACE FUNCTION claims.for_buyer_begin(p_key_hash bytea, p_request_hash bytea, p_conversation uuid, p_bundles uuid[], p_buyer uuid)
RETURNS TABLE(o_request uuid, o_reason text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; v_req uuid; v_reason text; b uuid; r record; v_order uuid; v_live uuid; v_own boolean; v_hash bytea; v_now timestamptz:=clock_timestamp(); v_sorted uuid[];
BEGIN
 v:=claims.for_buyer_scope();
 IF p_key_hash IS NULL OR octet_length(p_key_hash)<>32 OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_bundles IS NULL OR cardinality(p_bundles)>5
  OR (cardinality(p_bundles)>0 AND p_buyer IS NULL) THEN
  RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023';
 END IF;
 v_sorted:=ARRAY(SELECT DISTINCT x FROM unnest(p_bundles) x ORDER BY 1);
 IF cardinality(v_sorted)<>cardinality(p_bundles) OR EXISTS (SELECT 1 FROM unnest(p_bundles) x WHERE x IS NULL) THEN
  RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023';
 END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF cardinality(v_sorted)=0 THEN
  o_request:=gen_random_uuid(); o_reason:='no_bundle'; RETURN NEXT; RETURN;   -- a plain manual order: nothing to reserve, no price
 END IF;
 IF (SELECT count(*) FROM claims.bundles x WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.id=ANY(v_sorted) AND x.purged_at IS NULL)<>cardinality(v_sorted) THEN
  RAISE EXCEPTION 'not found' USING ERRCODE='PT404';
 END IF;
 -- §5.3 fail-closed eligibility, evaluated before anything is written (a mismatch creates nothing).
 v_reason:=claims.for_buyer_peer_state(p_conversation,v_sorted);
 IF v_reason='bundle_buyer_mismatch' THEN RAISE EXCEPTION 'bundle_buyer_mismatch' USING ERRCODE='PT409'; END IF;
 IF v_reason='' AND NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['live:manage']) THEN v_reason:='permission'; END IF;
 -- The request this Idempotency-Key already started (resume), else a new one. The same key with another request body is refused (P2-4).
 SELECT x.request_id,x.request_hash INTO v_req,v_hash FROM inbox.order_for_buyer x
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.idempotency_key_hash=p_key_hash LIMIT 1;
 IF NOT FOUND THEN v_req:=gen_random_uuid();
 ELSIF v_hash<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2]
  AND x.request_id=v_req AND x.create_attempt_id IS NOT NULL) THEN
  RAISE EXCEPTION 'attempt_owned_reservation' USING ERRCODE='PT409';
 END IF;
 PERFORM claims.lock_manual_order_bundles(v[1],v[2],v_sorted);
 FOREACH b IN ARRAY v_sorted LOOP
  -- §5.1 3a: lock the live row in sorted order. Release only another request's
  -- placed exact CANCELLED order binding, including an attempt-owned binding.
  -- Native order commands never revive CANCELLED; no backwards order lock needed.
  -- Timestamp recovery remains exclusively for legacy NULL-attempt pending rows.
  SELECT x.request_id,x.state,x.order_id,x.updated_at,x.buyer_id,x.create_attempt_id,x.idempotency_key_hash INTO r FROM inbox.order_for_buyer x
   WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.state IN ('pending','placed') FOR UPDATE;
  v_own:=FOUND AND r.request_id=v_req;
  IF NOT v_own AND EXISTS(SELECT 1 FROM inbox.manual_order_sources m
    WHERE m.tenant_id=v[1] AND m.store_id=v[2] AND m.key_hash=p_key_hash AND m.state='pending') THEN
   SELECT o.id INTO v_order FROM claims.orders_of_bundles(v[1],v[2],ARRAY[b]) src
    JOIN checkout.orders o ON o.tenant_id=v[1] AND o.store_id=v[2] AND o.id=src.order_id AND o.commercial_state<>'CANCELLED'
    ORDER BY o.created_at DESC,o.id DESC LIMIT 1;
   IF v_order IS NOT NULL THEN RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409',DETAIL=v_order::text; END IF;
  END IF;
  IF r.request_id IS NOT NULL AND NOT v_own THEN
   IF r.state='placed' AND EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=v[1] AND o.store_id=v[2]
    AND o.id=r.order_id AND o.owner_id=r.buyer_id AND o.create_attempt_id IS NOT DISTINCT FROM r.create_attempt_id
    AND o.commercial_state='CANCELLED') THEN
    UPDATE inbox.order_for_buyer x SET state='released',updated_at=v_now
     WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.request_id=r.request_id AND x.state='placed';
   ELSIF r.create_attempt_id IS NOT NULL THEN
    RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409';
   ELSE
    SELECT o.id INTO v_live FROM checkout.orders o WHERE o.tenant_id=v[1] AND o.store_id=v[2] AND o.owner_id=r.buyer_id AND o.commercial_state<>'CANCELLED'
     ORDER BY o.created_at DESC LIMIT 1;
    IF r.state='pending' AND v_live IS NULL AND r.updated_at<v_now-interval '15 minutes'
     AND NOT EXISTS(SELECT 1 FROM inbox.manual_order_sources m WHERE m.tenant_id=v[1] AND m.store_id=v[2]
       AND m.key_hash=r.idempotency_key_hash AND m.state='pending') THEN
     UPDATE inbox.order_for_buyer x SET state='released',updated_at=v_now
      WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.request_id=r.request_id AND x.state='pending' AND x.create_attempt_id IS NULL;
    ELSE
     RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409', DETAIL=coalesce(r.order_id::text,v_live::text,'');
    END IF;
   END IF;
  END IF;
  IF v_own THEN
   UPDATE inbox.order_for_buyer x SET updated_at=v_now WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=b AND x.request_id=v_req AND x.state='pending';
  END IF;
  -- A bundle whose claim lines are all fully consumed (the buyer ordered through the link) is also already ordered; a resume of THIS request
  -- (its own live row) is exempt, because its own order is what consumed the lines.
  IF NOT v_own AND EXISTS (SELECT 1 FROM claims.lines l WHERE l.tenant_id=v[1] AND l.store_id=v[2] AND l.bundle_id=b)
   AND NOT EXISTS (SELECT 1 FROM claims.lines l WHERE l.tenant_id=v[1] AND l.store_id=v[2] AND l.bundle_id=b
    AND l.quantity>(SELECT coalesce(sum(u.quantity),0) FROM claims.live_price_uses u
      JOIN checkout.orders r2 ON r2.tenant_id=u.tenant_id AND r2.store_id=u.store_id AND r2.id=u.order_id AND r2.commercial_state<>'CANCELLED'
      WHERE u.tenant_id=l.tenant_id AND u.store_id=l.store_id AND u.bundle_id=l.bundle_id AND u.offer_id=l.offer_id)) THEN
   SELECT u.order_id INTO v_order FROM claims.live_price_uses u JOIN checkout.orders r2 ON r2.tenant_id=u.tenant_id AND r2.store_id=u.store_id
    AND r2.id=u.order_id AND r2.commercial_state<>'CANCELLED'
    WHERE u.tenant_id=v[1] AND u.store_id=v[2] AND u.bundle_id=b ORDER BY r2.created_at DESC LIMIT 1;
   RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409', DETAIL=coalesce(v_order::text,'');
  END IF;
  BEGIN
   INSERT INTO inbox.order_for_buyer AS x(tenant_id,store_id,bundle_id,request_id,idempotency_key_hash,request_hash,buyer_id,state,conversation_id,principal_id)
    VALUES(v[1],v[2],b,v_req,p_key_hash,p_request_hash,p_buyer,'pending',p_conversation,v[3])
    ON CONFLICT (tenant_id,store_id,request_id,bundle_id) DO UPDATE SET state='pending',updated_at=v_now WHERE x.state='released';
  EXCEPTION WHEN unique_violation THEN
   RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409', DETAIL='';   -- the partial unique index lost a race with another request
  END;
 END LOOP;
 IF v_reason='' THEN
  FOREACH b IN ARRAY v_sorted LOOP
   -- §5.3: one single-use grant per bundle for THIS buyer; a resume of an unbound, unconsumed grant renews the 15 minutes.
   INSERT INTO claims.merchant_origin_grants AS g(tenant_id,store_id,request_id,buyer_id,bundle_id,principal_id,expires_at)
    VALUES(v[1],v[2],v_req,p_buyer,b,v[3],v_now+interval '15 minutes')
    ON CONFLICT (tenant_id,store_id,request_id,bundle_id) DO UPDATE SET expires_at=v_now+interval '15 minutes'
     WHERE g.consumed_at IS NULL AND g.quote_id IS NULL AND g.buyer_id=p_buyer;
  END LOOP;
 END IF;
 IF v_reason<>'' THEN
  -- A replay whose eligibility now fails must not leave the grants an earlier run wrote (P2-5): expire them.
  UPDATE claims.merchant_origin_grants g SET expires_at=v_now
   WHERE g.tenant_id=v[1] AND g.store_id=v[2] AND g.request_id=v_req AND g.consumed_at IS NULL AND g.expires_at>v_now;
 END IF;
 o_request:=v_req; o_reason:=v_reason; RETURN NEXT;
END $$;

CREATE OR REPLACE FUNCTION claims.for_buyer_release(p_request uuid) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v uuid[]; n integer;
BEGIN
 v:=claims.for_buyer_scope();
 IF p_request IS NULL THEN RAISE EXCEPTION 'invalid for-buyer request' USING ERRCODE='22023'; END IF;
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2]
  AND x.request_id=p_request AND x.create_attempt_id IS NOT NULL) THEN
  RAISE EXCEPTION 'attempt_owned_reservation' USING ERRCODE='PT409';
 END IF;
 IF EXISTS(SELECT 1 FROM inbox.manual_order_sources m WHERE m.tenant_id=v[1] AND m.store_id=v[2]
  AND m.claim_request_id=p_request AND m.state='pending') THEN
  RAISE EXCEPTION 'source_order_pending' USING ERRCODE='PT409';
 END IF;
 UPDATE inbox.order_for_buyer x SET state='released',updated_at=clock_timestamp()
  WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.request_id=p_request AND x.state='pending'
   -- never release a bundle that has an order: Place can fail AFTER begin_hold (the manual-source mark), and the order still exists (P2-1 c)
   AND NOT EXISTS (SELECT 1 FROM checkout.orders o WHERE o.tenant_id=x.tenant_id AND o.store_id=x.store_id AND o.owner_id=x.buyer_id AND o.commercial_state<>'CANCELLED');
 GET DIAGNOSTICS n=ROW_COUNT;
 UPDATE claims.merchant_origin_grants g SET expires_at=clock_timestamp()
  WHERE g.tenant_id=v[1] AND g.store_id=v[2] AND g.request_id=p_request AND g.consumed_at IS NULL;
 RETURN n;
END $$;


CREATE OR REPLACE FUNCTION claims.capture_order_origins(p_tenant uuid,p_store uuid,p_owner uuid,p_order uuid,p_origins jsonb) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_bundles uuid[];
BEGIN
 v_bundles:=ARRAY(SELECT DISTINCT bundle_id FROM jsonb_to_recordset(p_origins) x(bundle_id uuid) WHERE bundle_id IS NOT NULL);
 PERFORM claims.lock_manual_order_bundles(p_tenant,p_store,v_bundles);
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer r JOIN inbox.manual_order_sources m
   ON m.tenant_id=r.tenant_id AND m.store_id=r.store_id AND m.key_hash=r.idempotency_key_hash
   WHERE r.tenant_id=p_tenant AND r.store_id=p_store AND r.bundle_id=ANY(v_bundles) AND r.buyer_id<>p_owner
     AND (m.state='pending' AND r.state='pending' OR m.state='placed' AND r.state='placed'
          AND EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.id=r.order_id AND o.commercial_state<>'CANCELLED')))
 THEN RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409'; END IF;
 INSERT INTO claims.order_origins
 SELECT p_tenant,p_store,p_order,b.id,e.offer_id,e.line_version,e.session_id,src.source_object_id,e.occurred_at
 FROM jsonb_to_recordset(p_origins) x(bundle_id uuid,offer_id uuid,sku_id uuid,line_version bigint,quantity bigint)
 JOIN claims.bundles b ON b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=x.bundle_id AND b.owner_id=p_owner
 JOIN claims.lines l ON l.tenant_id=p_tenant AND l.store_id=p_store AND l.bundle_id=b.id AND l.offer_id=x.offer_id AND l.sku_id=x.sku_id
 JOIN claims.events e ON e.tenant_id=p_tenant AND e.store_id=p_store AND e.bundle_id=b.id AND e.offer_id=x.offer_id
  AND e.line_version=x.line_version AND e.outcome='ACCEPTED' AND x.quantity<=e.quantity
 LEFT JOIN claims.meta_intake i ON i.tenant_id=p_tenant AND i.store_id=p_store AND i.applied_event_id=e.id AND i.inbox_event_id=e.source_event_id AND i.state='APPLIED'
 LEFT JOIN live.claim_sources src ON src.tenant_id=p_tenant AND src.store_id=p_store AND src.id=i.source_id
 ON CONFLICT DO NOTHING;
END $$;

CREATE OR REPLACE FUNCTION inbox.buyer_panel(p_conversation uuid, p_bundle uuid)
RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
        v_app text; v_obj text; v_asset text; v_peer text;
        v_platform text;
        v_bundles uuid[];
        v_facts jsonb;
        v_order_ids uuid[];
        v_orders jsonb;
        v_auto text;
        v_out jsonb;
BEGIN
    v := inbox.lcn_scope();
    IF (p_conversation IS NULL) = (p_bundle IS NULL) THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT400';
    END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    IF p_bundle IS NOT NULL THEN
        SELECT b.platform INTO v_platform
          FROM claims.bundles b
         WHERE b.tenant_id = v[1] AND b.store_id = v[2] AND b.id = p_bundle AND b.purged_at IS NULL;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
        END IF;
        v_bundles := ARRAY[p_bundle];
    ELSE
        SELECT c.app_id, c.object, c.asset_id, c.peer_key INTO v_app, v_obj, v_asset, v_peer
          FROM social.conversations c
         WHERE c.id = p_conversation AND c.tenant_id = v[1] AND c.store_id = v[2];
        IF NOT FOUND THEN
            RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
        END IF;
        v_platform := CASE v_obj WHEN 'page' THEN 'messenger' ELSE 'instagram' END;
        SELECT coalesce(array_agg(b.id), ARRAY[]::uuid[]) INTO v_bundles
          FROM inbox.bundle_peers bp
          JOIN claims.bundles b ON b.tenant_id = bp.tenant_id AND b.store_id = bp.store_id
               AND b.id = bp.bundle_id AND b.purged_at IS NULL
         WHERE bp.tenant_id = v[1] AND bp.store_id = v[2]
           AND bp.app_id = v_app AND bp.object = v_obj
           AND bp.asset_id = v_asset AND bp.peer_key = v_peer;
    END IF;
    v_facts := claims.buyer_panel_claims(v[1], v[2], v_bundles);
    SELECT coalesce(array_agg(x.order_id), ARRAY[]::uuid[]) INTO v_order_ids
      FROM claims.orders_of_bundles(v[1], v[2], v_bundles) AS x(order_id);
    IF p_conversation IS NOT NULL THEN
        v_order_ids := ARRAY(SELECT DISTINCT id FROM (
            SELECT unnest(v_order_ids) AS id
            UNION ALL
            SELECT m.order_id FROM inbox.manual_order_sources m WHERE m.tenant_id=v[1] AND m.store_id=v[2]
              AND m.conversation_id=p_conversation AND m.state='placed'
        ) AS scoped_orders);
    END IF;
    v_out := v_facts || jsonb_build_object(
        'platform', v_platform,
        'purchase_ordinal', (SELECT count(*) FROM checkout.order_panel_facts(v[1], v[2], v_order_ids) f
                              WHERE f.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION')),
        'link_pending_manual', EXISTS (SELECT 1 FROM claims.bundles b
                                        WHERE b.tenant_id = v[1] AND b.store_id = v[2]
                                          AND b.id = ANY(v_bundles) AND b.link_pending_manual));
    IF identity.principal_holds(v[1], v[2], v[3], ARRAY['orders:read']::text[]) THEN
        SELECT coalesce(jsonb_agg(jsonb_build_object(
                'order_id', f.order_id,
                'number', 'LC-' || upper(replace(f.order_id::text, '-', '')),
                'state', f.commercial_state,
                'total_minor', f.total_minor,
                'created_at', to_char(f.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))
            ORDER BY f.created_at DESC, f.order_id DESC), '[]'::jsonb) INTO v_orders
          FROM (SELECT * FROM checkout.order_panel_facts(v[1], v[2], v_order_ids)
                 ORDER BY created_at DESC, order_id DESC LIMIT 20) f;
        v_out := v_out || jsonb_build_object('orders', v_orders);
    END IF;
    SELECT o.state INTO v_auto
      FROM integration.operations o
     WHERE o.tenant_id = v[1] AND o.store_id = v[2]
       AND o.action = 'meta.private_reply'
       AND o.request->>'origin_kind' = 'auto'
       AND o.request->>'bundle_id' IN (SELECT g::text FROM unnest(v_bundles) g)
     ORDER BY o.created_at DESC, o.id DESC
     LIMIT 1;
    IF FOUND THEN
        v_out := v_out || jsonb_build_object('auto_reply_state', v_auto);
    END IF;
    RETURN v_out;
END $$;
