-- Purpose: lock and complete only the claim origins admitted by one fenced OA1 plan.
-- Depends on: 0175 admission custody, 0178 exact current-xid context, 0179 quote and 0184 Begin.
-- Used by: commerce_order_attempt_owner after owner locks, before order/cart, then after native Begin.
-- Invariants: I01/I02/I04/I08/I14; original quantities/version/money, sorted prefix,
-- no placement/session/repricing gates, no coordinator DML and no internal commit.

-- Existing scoped SELECT policies suffice. Only the NOLOGIN domain owner needs
-- these provenance columns; neither runtime nor coordinator gains table access.
GRANT SELECT(create_attempt_id) ON checkout.orders TO commerce_claims_writer;
GRANT SELECT(create_attempt_id,creator_session_id) ON storefront.quotes TO commerce_claims_writer;

CREATE FUNCTION claims.order_attempt_lock_origins(p_attempt uuid,p_plan_plaintext text)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_ctx jsonb; v_binding jsonb; v_facts jsonb; v_selection jsonb; v_lines jsonb; v_catalog jsonb;
 v_tenant uuid; v_store uuid; v_owner uuid; v_principal uuid; v_deadline timestamptz; v_hash bytea;
 v_bundles uuid[]; v_selected uuid[]; v_count bigint;
 v_prior text[]:=ARRAY[current_setting('app.tenant_id',true),current_setting('app.store_id',true),
  current_setting('app.principal_id',true),current_setting('app.buyer_id',true),current_setting('app.buyer_session_id',true)];
BEGIN
 -- This accessor proves the exact pending attempt, plaintext and current xid,
 -- including execute_by, before any domain read or lock.
 v_ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 v_binding:=v_ctx->'binding'; v_facts:=v_ctx->'plan'->'facts'; v_selection:=v_facts->'selection';
 v_tenant:=(v_binding->>'tenant_id')::uuid; v_store:=(v_binding->>'store_id')::uuid;
 v_owner:=(v_binding->>'buyer_owner_id')::uuid; v_principal:=(v_binding->>'principal_id')::uuid;
 v_deadline:=(v_binding->>'execute_by')::timestamptz;
 -- Compare the exact opaque C1 custody, without retaining a PII selection hash.
 v_hash:=sha256(convert_to('order-attempt-custody/v1|'||v_tenant::text||'|'||v_store::text||'|'||p_attempt::text
  ||'|'||(v_binding->>'generation')::bigint::text||'|'||v_owner::text||'|'||v_principal::text,'UTF8'));
 v_lines:=v_facts->'claims'->'lines'; v_catalog:=v_facts->'catalog'->'lines';
 IF jsonb_typeof(v_lines) IS DISTINCT FROM 'array' OR jsonb_typeof(v_catalog) IS DISTINCT FROM 'array'
  OR jsonb_typeof(v_selection#>'{for,bundle_ids}') IS DISTINCT FROM 'array' THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
 BEGIN
  v_bundles:=ARRAY(SELECT (value#>>'{}')::uuid FROM jsonb_array_elements(v_selection#>'{for,bundle_ids}') ORDER BY 1);
  v_selected:=ARRAY(SELECT DISTINCT (value->>'bundle_id')::uuid FROM jsonb_array_elements(v_lines) ORDER BY 1);
  IF v_bundles IS DISTINCT FROM v_selected
   OR cardinality(v_bundles)<>(SELECT count(DISTINCT x) FROM unnest(v_bundles) x)
   OR EXISTS(SELECT 1 FROM jsonb_array_elements(v_lines) c WHERE
    coalesce(c->>'line_version','') !~ '^[1-9][0-9]{0,18}$' OR (c->>'line_version')::bigint<1
    OR (c->>'quantity')::bigint NOT BETWEEN 1 AND 1000
    OR (c->>'origin_id')::uuid='00000000-0000-0000-0000-000000000000'::uuid
    OR (SELECT count(*) FROM jsonb_array_elements(v_selection->'items') s WHERE
     s->>'ordinal'=c->>'ordinal' AND s->>'origin_id'=c->>'origin_id' AND s->>'bundle_id'=c->>'bundle_id'
     AND s->>'offer_id'=c->>'offer_id' AND s->>'sku_id'=c->>'sku_id' AND s->>'quantity'=c->>'quantity')<>1
    OR (SELECT count(*) FROM jsonb_array_elements(v_catalog) s WHERE
     s->>'ordinal'=c->>'ordinal' AND s->>'origin_id'=c->>'origin_id' AND s->>'bundle_id'=c->>'bundle_id'
     AND s->>'offer_id'=c->>'offer_id' AND s->>'sku_id'=c->>'sku_id' AND s->>'quantity'=c->>'quantity')<>1)
   OR jsonb_array_length(v_lines)<>(SELECT count(*) FROM jsonb_array_elements(v_selection->'items') s WHERE s->>'bundle_id' IS NOT NULL)
   OR (SELECT count(DISTINCT c->>'ordinal') FROM jsonb_array_elements(v_lines) c)<>jsonb_array_length(v_lines)
   OR (SELECT count(DISTINCT c->>'origin_id') FROM jsonb_array_elements(v_lines) c)<>jsonb_array_length(v_lines)
   OR EXISTS(SELECT 1 FROM jsonb_array_elements(v_lines) c GROUP BY c->>'bundle_id',c->>'offer_id'
    HAVING count(DISTINCT c->>'sku_id')<>1 OR count(DISTINCT c->>'line_version')<>1
     OR count(DISTINCT coalesce(c->>'live_price_minor','null'))<>1)
   OR (cardinality(v_bundles)>0 AND v_facts->'claims'->>'request_id' IS DISTINCT FROM p_attempt::text) THEN
   RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
 EXCEPTION WHEN invalid_text_representation OR numeric_value_out_of_range THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400';
 END;
 PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_store::text,true),
  set_config('app.principal_id',v_principal::text,true),set_config('app.buyer_id','',true),set_config('app.buyer_session_id','',true);
 -- One deterministic domain prefix. No current offer/session/window/permission
 -- read is part of continuation, and no order/cart/catalog lock is taken here.
 PERFORM b.id FROM claims.bundles b WHERE b.tenant_id=v_tenant AND b.store_id=v_store AND b.id=ANY(v_bundles) ORDER BY b.id FOR UPDATE;
 IF (SELECT count(*) FROM claims.bundles b WHERE b.tenant_id=v_tenant AND b.store_id=v_store AND b.id=ANY(v_bundles)
  AND b.purged_at IS NULL)<>cardinality(v_bundles) THEN
  RAISE EXCEPTION 'resource_unavailable' USING ERRCODE='PT404'; END IF;
 PERFORM l.offer_id FROM claims.lines l WHERE l.tenant_id=v_tenant AND l.store_id=v_store AND l.bundle_id=ANY(v_bundles)
  ORDER BY l.bundle_id,l.offer_id FOR UPDATE;
 PERFORM r.request_id FROM inbox.order_for_buyer r WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.bundle_id=ANY(v_bundles)
  ORDER BY r.bundle_id,r.request_id FOR UPDATE;
 PERFORM g.request_id FROM claims.merchant_origin_grants g WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.bundle_id=ANY(v_bundles)
  ORDER BY g.bundle_id,g.request_id FOR UPDATE;
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer r WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.bundle_id=ANY(v_bundles)
  AND r.create_attempt_id IS DISTINCT FROM p_attempt AND r.state='placed') THEN
  RAISE EXCEPTION 'bundle_order_exists' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer r WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.bundle_id=ANY(v_bundles)
  AND r.create_attempt_id IS DISTINCT FROM p_attempt AND r.state='pending') THEN
  RAISE EXCEPTION 'bundle_reserved' USING ERRCODE='PT409'; END IF;
 SELECT count(*) INTO v_count FROM inbox.order_for_buyer r WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.create_attempt_id=p_attempt;
 IF v_count<>cardinality(v_bundles) OR EXISTS(SELECT 1 FROM inbox.order_for_buyer r
  WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.create_attempt_id=p_attempt
   AND (r.request_id<>p_attempt OR NOT r.bundle_id=ANY(v_bundles) OR r.buyer_id<>v_owner OR r.principal_id<>v_principal
    OR r.request_hash IS DISTINCT FROM v_hash OR r.idempotency_key_hash IS DISTINCT FROM sha256(convert_to('order-attempt-reservation:'||p_attempt::text,'UTF8'))
    OR r.state<>'pending' OR r.order_id IS NOT NULL OR r.expires_at IS DISTINCT FROM v_deadline)) THEN
  RAISE EXCEPTION 'reservation_lost' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM claims.merchant_origin_grants g WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.create_attempt_id=p_attempt
  AND (g.request_id<>p_attempt OR NOT g.bundle_id=ANY(v_bundles) OR g.buyer_id<>v_owner OR g.principal_id<>v_principal
   OR g.expires_at IS DISTINCT FROM v_deadline OR g.quote_id IS NOT NULL OR g.consumed_at IS NOT NULL))
  OR EXISTS(SELECT 1 FROM jsonb_array_elements(v_lines) c WHERE c->>'live_price_minor' IS NOT NULL
   AND NOT EXISTS(SELECT 1 FROM claims.merchant_origin_grants g WHERE g.tenant_id=v_tenant AND g.store_id=v_store
    AND g.create_attempt_id=p_attempt AND g.request_id=p_attempt AND g.bundle_id=(c->>'bundle_id')::uuid
    AND g.buyer_id=v_owner AND g.principal_id=v_principal AND g.expires_at=v_deadline AND g.quote_id IS NULL AND g.consumed_at IS NULL)) THEN
  RAISE EXCEPTION 'origin_grant_unavailable' USING ERRCODE='PT409'; END IF;
 -- All original selected quantities count, even a catalog-priced claim. Current
 -- accepted version may advance; source audit below uses the frozen version.
 IF EXISTS(SELECT 1 FROM (SELECT (c->>'bundle_id')::uuid bundle_id,(c->>'offer_id')::uuid offer_id,
  (c->>'sku_id')::uuid sku_id,sum((c->>'quantity')::bigint) quantity FROM jsonb_array_elements(v_lines) c GROUP BY 1,2,3) d
  LEFT JOIN claims.lines l ON l.tenant_id=v_tenant AND l.store_id=v_store AND l.bundle_id=d.bundle_id AND l.offer_id=d.offer_id
  WHERE l.sku_id IS DISTINCT FROM d.sku_id OR d.quantity>l.quantity-(SELECT coalesce(sum(u.quantity),0) FROM claims.live_price_uses u
   JOIN checkout.orders o ON o.tenant_id=u.tenant_id AND o.store_id=u.store_id AND o.id=u.order_id AND o.commercial_state<>'CANCELLED'
   WHERE u.tenant_id=v_tenant AND u.store_id=v_store AND u.bundle_id=d.bundle_id AND u.offer_id=d.offer_id)) THEN
  RAISE EXCEPTION 'claim_quantity_exceeded' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM (SELECT (c->>'bundle_id')::uuid bundle_id,(c->>'offer_id')::uuid offer_id,
  (c->>'line_version')::bigint line_version,sum((c->>'quantity')::bigint) quantity FROM jsonb_array_elements(v_lines) c GROUP BY 1,2,3) d
  WHERE NOT EXISTS(SELECT 1 FROM claims.events e WHERE e.tenant_id=v_tenant AND e.store_id=v_store AND e.bundle_id=d.bundle_id
   AND e.offer_id=d.offer_id AND e.line_version=d.line_version AND e.outcome='ACCEPTED' AND e.quantity>=d.quantity)) THEN
  RAISE EXCEPTION 'resource_unavailable' USING ERRCODE='PT404'; END IF;
 IF clock_timestamp()>=v_deadline THEN RAISE EXCEPTION 'attempt_expired' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',coalesce(v_prior[1],''),true),set_config('app.store_id',coalesce(v_prior[2],''),true),
  set_config('app.principal_id',coalesce(v_prior[3],''),true),set_config('app.buyer_id',coalesce(v_prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(v_prior[5],''),true);
 RETURN 'active';
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(v_prior[1],''),true),set_config('app.store_id',coalesce(v_prior[2],''),true),
  set_config('app.principal_id',coalesce(v_prior[3],''),true),set_config('app.buyer_id',coalesce(v_prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(v_prior[5],''),true);
 RAISE;
END $$;
ALTER FUNCTION claims.order_attempt_lock_origins(uuid,text) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.order_attempt_lock_origins(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.order_attempt_lock_origins(uuid,text) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION claims.order_attempt_lock_origins(uuid,text) IS
 'Private C2 claim prefix after retained owner locks: exact current-xid plan, sorted bundle/line/reservation/grant locks, original aggregate allowance and native admission custody/deadline. Returns active including manual empty set; typed refusal for unavailable origins. No current Place, session, offer price/active, catalog, order or business writes. Coordinator EXECUTE only.';

CREATE FUNCTION claims.order_attempt_complete_origins(p_attempt uuid,p_plan_plaintext text,p_order uuid)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_ctx jsonb; v_binding jsonb; v_facts jsonb; v_lines jsonb; v_catalog jsonb; v_snapshot jsonb;
 v_tenant uuid; v_store uuid; v_owner uuid; v_principal uuid; v_deadline timestamptz; v_hash bytea;
 v_bundles uuid[]; v_used uuid[]; v_quote uuid; v_count bigint; v_expected bigint; v_line jsonb; v_native jsonb; v_price bigint;
 v_prior text[]:=ARRAY[current_setting('app.tenant_id',true),current_setting('app.store_id',true),
  current_setting('app.principal_id',true),current_setting('app.buyer_id',true),current_setting('app.buyer_session_id',true)];
BEGIN
 v_ctx:=ops.order_attempt_execution_context(p_attempt,p_plan_plaintext);
 v_binding:=v_ctx->'binding'; v_facts:=v_ctx->'plan'->'facts';
 v_tenant:=(v_binding->>'tenant_id')::uuid; v_store:=(v_binding->>'store_id')::uuid;
 v_owner:=(v_binding->>'buyer_owner_id')::uuid; v_principal:=(v_binding->>'principal_id')::uuid;
 v_deadline:=(v_binding->>'execute_by')::timestamptz;
 v_hash:=sha256(convert_to('order-attempt-custody/v1|'||v_tenant::text||'|'||v_store::text||'|'||p_attempt::text
  ||'|'||(v_binding->>'generation')::bigint::text||'|'||v_owner::text||'|'||v_principal::text,'UTF8'));
 v_lines:=v_facts->'claims'->'lines'; v_catalog:=v_facts->'catalog'->'lines';
 IF jsonb_typeof(v_lines) IS DISTINCT FROM 'array' OR jsonb_typeof(v_catalog) IS DISTINCT FROM 'array' THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
 v_bundles:=ARRAY(SELECT DISTINCT (c->>'bundle_id')::uuid FROM jsonb_array_elements(v_lines) c ORDER BY 1);
 v_used:=ARRAY(SELECT DISTINCT (c->>'bundle_id')::uuid FROM jsonb_array_elements(v_lines) c WHERE c->>'live_price_minor' IS NOT NULL ORDER BY 1);
 PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_store::text,true),
  set_config('app.principal_id',v_principal::text,true),set_config('app.buyer_id','',true),set_config('app.buyer_session_id','',true);
 -- Caller retains the entire earlier prefix from lock_origins. Never call that
 -- helper again here: these are exact reads, followed by own-row writes only.
 IF (SELECT count(*) FROM claims.bundles b WHERE b.tenant_id=v_tenant AND b.store_id=v_store AND b.id=ANY(v_bundles)
  AND b.purged_at IS NULL)<>cardinality(v_bundles) THEN
  RAISE EXCEPTION 'resource_unavailable' USING ERRCODE='PT404'; END IF;
 SELECT count(*) INTO v_count FROM inbox.order_for_buyer r WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.create_attempt_id=p_attempt;
 IF v_count<>cardinality(v_bundles) OR EXISTS(SELECT 1 FROM inbox.order_for_buyer r
  WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.create_attempt_id=p_attempt
   AND (r.request_id<>p_attempt OR NOT r.bundle_id=ANY(v_bundles) OR r.buyer_id<>v_owner OR r.principal_id<>v_principal
    OR r.request_hash IS DISTINCT FROM v_hash OR r.state<>'pending' OR r.order_id IS NOT NULL OR r.expires_at IS DISTINCT FROM v_deadline)) THEN
  RAISE EXCEPTION 'reservation_lost' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM claims.merchant_origin_grants g WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.create_attempt_id=p_attempt
  AND (g.request_id<>p_attempt OR NOT g.bundle_id=ANY(v_bundles) OR g.buyer_id<>v_owner OR g.principal_id<>v_principal
   OR g.expires_at IS DISTINCT FROM v_deadline OR g.quote_id IS NOT NULL OR g.consumed_at IS NOT NULL))
  OR EXISTS(SELECT 1 FROM unnest(v_used) b WHERE NOT EXISTS(SELECT 1 FROM claims.merchant_origin_grants g
   WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.create_attempt_id=p_attempt AND g.request_id=p_attempt
    AND g.bundle_id=b AND g.buyer_id=v_owner AND g.principal_id=v_principal AND g.expires_at=v_deadline
    AND g.quote_id IS NULL AND g.consumed_at IS NULL)) THEN
  RAISE EXCEPTION 'origin_grant_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT o.quote_id INTO v_quote FROM checkout.orders o WHERE o.tenant_id=v_tenant AND o.store_id=v_store
  AND o.id=p_order AND o.id=p_attempt AND o.create_attempt_id=p_attempt AND o.owner_id=v_owner
  AND o.creator_session_id IS NULL AND o.commercial_state<>'CANCELLED';
 IF NOT FOUND THEN RAISE EXCEPTION 'resource_unavailable' USING ERRCODE='PT404'; END IF;
 -- Quote/use policies require buyer context; this is the admitted native owner,
 -- never an invented browser session. Admission updates switch back below.
 PERFORM set_config('app.principal_id','',true),set_config('app.buyer_id',v_owner::text,true);
 SELECT q.snapshot INTO v_snapshot FROM storefront.quotes q WHERE q.tenant_id=v_tenant AND q.store_id=v_store
  AND q.id=v_quote AND q.owner_id=v_owner AND q.create_attempt_id=p_attempt AND q.creator_session_id IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'resource_unavailable' USING ERRCODE='PT404'; END IF;
 IF jsonb_typeof(v_snapshot->'lines') IS DISTINCT FROM 'array'
  OR jsonb_array_length(v_snapshot->'lines')<>jsonb_array_length(v_catalog) THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
 FOR v_line IN SELECT value FROM jsonb_array_elements(v_lines) ORDER BY (value->>'ordinal')::bigint LOOP
  SELECT count(*) INTO v_count FROM jsonb_array_elements(v_snapshot->'lines') q WHERE
   q->>'line_ordinal'=v_line->>'ordinal' AND q->>'origin_kind'='claim' AND q->>'origin_id'=v_line->>'origin_id'
   AND q->>'sku_id'=v_line->>'sku_id' AND q->>'quantity'=v_line->>'quantity';
  IF v_count<>1 THEN RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
  SELECT value INTO v_native FROM jsonb_array_elements(v_snapshot->'lines') WHERE value->>'line_ordinal'=v_line->>'ordinal';
  IF v_line->>'live_price_minor' IS NOT NULL THEN
   v_price:=(v_line->>'live_price_minor')::bigint;
   IF v_native->>'price_rule' IS DISTINCT FROM 'live_claim'
    OR v_native->>'claim_bundle_id' IS DISTINCT FROM v_line->>'bundle_id'
    OR v_native->>'claim_offer_id' IS DISTINCT FROM v_line->>'offer_id' THEN
    RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
  ELSE
   SELECT (c->>'unit_price_minor')::bigint INTO v_price FROM jsonb_array_elements(v_catalog) c WHERE c->>'ordinal'=v_line->>'ordinal';
   IF v_native->>'price_rule'='live_claim' THEN RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
  END IF;
  IF v_price IS NULL OR (v_native->>'unit_price_minor')::bigint IS DISTINCT FROM v_price THEN
   RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
 END LOOP;
 IF EXISTS(SELECT 1 FROM jsonb_array_elements(v_snapshot->'lines') q WHERE q->>'price_rule'='live_claim'
  AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(v_lines) c WHERE c->>'ordinal'=q->>'line_ordinal'
   AND c->>'origin_id'=q->>'origin_id' AND c->>'live_price_minor' IS NOT NULL)) THEN
  RAISE EXCEPTION 'plan_invalid' USING ERRCODE='PT400'; END IF;
 IF EXISTS(SELECT 1 FROM claims.live_price_uses u WHERE u.tenant_id=v_tenant AND u.store_id=v_store AND u.order_id=p_order)
  OR EXISTS(SELECT 1 FROM claims.order_origins o WHERE o.tenant_id=v_tenant AND o.store_id=v_store AND o.order_id=p_order) THEN
  RAISE EXCEPTION 'origin_already_consumed' USING ERRCODE='PT409'; END IF;
 -- Recheck full original aggregate allowance under the retained claim locks.
 -- There is one native use row per physical bundle/offer/order, not per ordinal.
 IF EXISTS(SELECT 1 FROM (SELECT (c->>'bundle_id')::uuid bundle_id,(c->>'offer_id')::uuid offer_id,
  (c->>'sku_id')::uuid sku_id,sum((c->>'quantity')::bigint) quantity FROM jsonb_array_elements(v_lines) c GROUP BY 1,2,3) d
  LEFT JOIN claims.lines l ON l.tenant_id=v_tenant AND l.store_id=v_store AND l.bundle_id=d.bundle_id AND l.offer_id=d.offer_id
  WHERE l.sku_id IS DISTINCT FROM d.sku_id OR d.quantity>l.quantity-(SELECT coalesce(sum(u.quantity),0) FROM claims.live_price_uses u
   JOIN checkout.orders o ON o.tenant_id=u.tenant_id AND o.store_id=u.store_id AND o.id=u.order_id AND o.commercial_state<>'CANCELLED'
   WHERE u.tenant_id=v_tenant AND u.store_id=v_store AND u.bundle_id=d.bundle_id AND u.offer_id=d.offer_id)) THEN
  RAISE EXCEPTION 'claim_quantity_exceeded' USING ERRCODE='PT409'; END IF;
 INSERT INTO claims.live_price_uses(tenant_id,store_id,bundle_id,offer_id,order_id,quantity,unit_price_minor)
  SELECT v_tenant,v_store,(c->>'bundle_id')::uuid,(c->>'offer_id')::uuid,p_order,sum((c->>'quantity')::bigint),
   (c->>'live_price_minor')::bigint FROM jsonb_array_elements(v_lines) c WHERE c->>'live_price_minor' IS NOT NULL
  GROUP BY (c->>'bundle_id')::uuid,(c->>'offer_id')::uuid,(c->>'live_price_minor')::bigint ORDER BY 3,4;
 -- Freeze the original ACCEPTED event, including price-neutral claim origins.
 -- Merchant provenance comes from this exact attempt's reservation/grant, not
 -- current bundle.owner_id equality or a new capability/session.
 SELECT count(*) INTO v_expected FROM (SELECT DISTINCT c->>'bundle_id',c->>'offer_id' FROM jsonb_array_elements(v_lines) c) origins;
 INSERT INTO claims.order_origins(tenant_id,store_id,order_id,bundle_id,offer_id,line_version,session_id,post_id,occurred_at)
  SELECT v_tenant,v_store,p_order,d.bundle_id,d.offer_id,d.line_version,e.session_id,src.source_object_id,e.occurred_at
  FROM (SELECT (c->>'bundle_id')::uuid bundle_id,(c->>'offer_id')::uuid offer_id,(c->>'line_version')::bigint line_version,
   sum((c->>'quantity')::bigint) quantity FROM jsonb_array_elements(v_lines) c GROUP BY 1,2,3) d
  JOIN claims.events e ON e.tenant_id=v_tenant AND e.store_id=v_store AND e.bundle_id=d.bundle_id AND e.offer_id=d.offer_id
   AND e.line_version=d.line_version AND e.outcome='ACCEPTED' AND e.quantity>=d.quantity
  LEFT JOIN claims.meta_intake i ON i.tenant_id=v_tenant AND i.store_id=v_store AND i.applied_event_id=e.id
   AND i.inbox_event_id=e.source_event_id AND i.state='APPLIED'
  LEFT JOIN live.claim_sources src ON src.tenant_id=v_tenant AND src.store_id=v_store AND src.id=i.source_id
  ORDER BY d.bundle_id,d.offer_id;
 GET DIAGNOSTICS v_count=ROW_COUNT;
 IF v_count<>v_expected THEN RAISE EXCEPTION 'resource_unavailable' USING ERRCODE='PT404'; END IF;
 UPDATE claims.merchant_origin_grants g SET quote_id=v_quote,consumed_at=clock_timestamp()
  WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.create_attempt_id=p_attempt AND g.request_id=p_attempt
   AND g.bundle_id=ANY(v_used) AND g.buyer_id=v_owner AND g.principal_id=v_principal AND g.expires_at=v_deadline
   AND g.quote_id IS NULL AND g.consumed_at IS NULL;
 GET DIAGNOSTICS v_count=ROW_COUNT;
 IF v_count<>cardinality(v_used) THEN RAISE EXCEPTION 'origin_grant_unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.principal_id',v_principal::text,true),set_config('app.buyer_id','',true);
 UPDATE inbox.order_for_buyer r SET state='placed',order_id=p_order,updated_at=clock_timestamp()
  WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.create_attempt_id=p_attempt AND r.request_id=p_attempt
   AND r.bundle_id=ANY(v_bundles) AND r.buyer_id=v_owner AND r.principal_id=v_principal AND r.state='pending' AND r.order_id IS NULL;
 GET DIAGNOSTICS v_count=ROW_COUNT;
 IF v_count<>cardinality(v_bundles) THEN RAISE EXCEPTION 'reservation_lost' USING ERRCODE='PT409'; END IF;
 UPDATE claims.merchant_origin_grants g SET expires_at=least(g.expires_at,clock_timestamp())
  WHERE g.tenant_id=v_tenant AND g.store_id=v_store AND g.create_attempt_id=p_attempt AND g.request_id=p_attempt
   AND g.buyer_id=v_owner AND g.principal_id=v_principal AND g.quote_id IS NULL AND g.consumed_at IS NULL;
 IF clock_timestamp()>=v_deadline THEN RAISE EXCEPTION 'attempt_expired' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',coalesce(v_prior[1],''),true),set_config('app.store_id',coalesce(v_prior[2],''),true),
  set_config('app.principal_id',coalesce(v_prior[3],''),true),set_config('app.buyer_id',coalesce(v_prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(v_prior[5],''),true);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(v_prior[1],''),true),set_config('app.store_id',coalesce(v_prior[2],''),true),
  set_config('app.principal_id',coalesce(v_prior[3],''),true),set_config('app.buyer_id',coalesce(v_prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(v_prior[5],''),true);
 RAISE;
END $$;
ALTER FUNCTION claims.order_attempt_complete_origins(uuid,text,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.order_attempt_complete_origins(uuid,text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.order_attempt_complete_origins(uuid,text,uuid) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION claims.order_attempt_complete_origins(uuid,text,uuid) IS
 'Private C2 claim completion after native attempt Begin under retained prefix: exact native creator/quote owner, frozen ordinal/origin/quantity/price, aggregate live uses only, original accepted-event/source audit including no-live origins, every selected reservation placed, used grants consumed and unused grants expired retaining audit. No earlier lock reacquisition, current Place/window/offer gates, internal commit, receipt or settlement. Coordinator EXECUTE only.';
