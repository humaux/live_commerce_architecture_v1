-- Purpose: freeze C1 claim origins and reserve bundles/grants for one durable attempt.
-- Depends on: 0170 creator identity, 0172 candidate accessor, 0129 peer rules and 0158 prices.
-- Used by: the C1 coordinator before catalog facts; C2 settlement is a separate fenced helper.
-- Invariants: I01/I02/I08/I14; explicit origins never merge, one live reservation per bundle,
-- immutable execute_by is the only attempt TTL, and legacy paths cannot take attempt grants.

ALTER TABLE inbox.order_for_buyer ADD COLUMN create_attempt_id uuid,
 ADD COLUMN expires_at timestamptz,
 ADD CONSTRAINT order_for_buyer_attempt_identity FOREIGN KEY(tenant_id,store_id,buyer_id,create_attempt_id)
 REFERENCES ops.order_attempt_creators(tenant_id,store_id,buyer_owner_id,attempt_id),
 ADD CONSTRAINT order_for_buyer_attempt_request CHECK(create_attempt_id IS NULL
  OR (request_id=create_attempt_id AND expires_at IS NOT NULL));
ALTER TABLE claims.merchant_origin_grants ADD COLUMN create_attempt_id uuid,
 ADD CONSTRAINT merchant_origin_grant_attempt_identity FOREIGN KEY(tenant_id,store_id,buyer_id,create_attempt_id)
 REFERENCES ops.order_attempt_creators(tenant_id,store_id,buyer_owner_id,attempt_id),
 ADD CONSTRAINT merchant_origin_grant_attempt_request CHECK(create_attempt_id IS NULL OR request_id=create_attempt_id);
COMMENT ON COLUMN inbox.order_for_buyer.create_attempt_id IS
 'claims C1 creator identity; claims writer only; legacy begin/finish/release cannot mutate attempt reservations.';
COMMENT ON COLUMN inbox.order_for_buyer.expires_at IS
 'claims C1 immutable attempt execute_by; elapsed time alone never releases a pending attempt reservation.';
COMMENT ON COLUMN claims.merchant_origin_grants.create_attempt_id IS
 'claims C1 creator identity; grants are exclusively consumed/released by fenced attempt execution, never storefront.';

-- FOR SHARE needs UPDATE privilege and a matching policy. Only the domain owner gains
-- one lock column; the coordinator receives no table access or domain-owner membership.
GRANT UPDATE(active) ON live.offers TO commerce_claims_writer;
CREATE POLICY offer_claims_admission_lock ON live.offers FOR UPDATE TO commerce_claims_writer
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL
  AND nullif(current_setting('app.buyer_id',true),'') IS NULL)
 WITH CHECK(false);

CREATE FUNCTION claims.order_attempt_admission_origins(p_attempt uuid,p_selection jsonb) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; v uuid[]; v_owner uuid; v_deadline timestamptz; v_request_hash bytea;
 v_bundles uuid[]; v_selected uuid[]; v_conversation uuid; item jsonb; v_bundle uuid;
 v_reason text; line_reason text; v_lines jsonb:='[]'::jsonb; r record; v_quantity bigint;
 v_offer uuid; v_sku uuid; v_origin uuid; v_ordinal integer; v_price bigint;
 v_uuid constant text:='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
BEGIN
 -- Exact request hash, authenticated principal and unfinished candidate come from ops,
 -- not client tenant/owner fields. This function changes no caller GUC on any path.
 ctx:=ops.order_attempt_admission_context(p_attempt,p_selection);
 v:=claims.for_buyer_scope();
 v_owner:=(ctx->>'buyer_owner_id')::uuid;
 v_deadline:=(ctx->>'execute_by')::timestamptz;
 -- Retained business audit binds only opaque admission authority, never the
 -- canonical selection hash (which includes buyer contact/address material).
 v_request_hash:=sha256(convert_to('order-attempt-custody/v1|'||v[1]::text||'|'||v[2]::text||'|'||p_attempt::text
  ||'|'||(ctx->>'intent_generation')::bigint::text||'|'||v_owner::text||'|'||v[3]::text,'UTF8'));
 IF NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['inventory:reserve']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 IF jsonb_typeof(p_selection->'items') IS DISTINCT FROM 'array'
  OR jsonb_array_length(p_selection->'items') NOT BETWEEN 1 AND 50
  OR jsonb_typeof(p_selection->'for') IS DISTINCT FROM 'object'
  OR jsonb_typeof(p_selection#>'{for,bundle_ids}') IS DISTINCT FROM 'array' THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF jsonb_array_length(p_selection#>'{for,bundle_ids}')>5
  OR EXISTS(SELECT 1 FROM jsonb_array_elements(p_selection#>'{for,bundle_ids}') x
   WHERE jsonb_typeof(x)<>'string' OR (x#>>'{}') !~ v_uuid) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 v_bundles:=ARRAY(SELECT (x#>>'{}')::uuid FROM jsonb_array_elements(p_selection#>'{for,bundle_ids}') x ORDER BY 1);
 IF cardinality(v_bundles)<>(SELECT count(DISTINCT x) FROM unnest(v_bundles) x) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF p_selection#>>'{for,conversation_id}' IS NOT NULL THEN
  IF p_selection#>>'{for,conversation_id}' !~ v_uuid THEN RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400'; END IF;
  v_conversation:=(p_selection#>>'{for,conversation_id}')::uuid;
 END IF;
 FOR item IN SELECT value FROM jsonb_array_elements(p_selection->'items') LOOP
  IF jsonb_typeof(item)<>'object' OR coalesce(item->>'sku_id','') !~ v_uuid
   OR jsonb_typeof(item->'quantity') IS DISTINCT FROM 'number' OR coalesce(item->>'quantity','') !~ '^([1-9][0-9]{0,2}|1000)$'
   OR jsonb_typeof(item->'ordinal') IS DISTINCT FROM 'number' OR coalesce(item->>'ordinal','') !~ '^[1-9][0-9]{0,8}$'
   OR (item->>'origin_id' IS NOT NULL AND item->>'origin_id' !~ v_uuid)
   OR ((item->>'bundle_id' IS NULL)<>(item->>'offer_id' IS NULL))
   OR (item->>'bundle_id' IS NOT NULL AND (item->>'bundle_id' !~ v_uuid OR item->>'offer_id' !~ v_uuid OR item->>'origin_id' IS NULL)) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
 END LOOP;
 IF (SELECT count(DISTINCT x->>'ordinal') FROM jsonb_array_elements(p_selection->'items') x)<>jsonb_array_length(p_selection->'items')
  OR EXISTS(SELECT 1 FROM jsonb_array_elements(p_selection->'items') x WHERE x->>'origin_id' IS NOT NULL
   GROUP BY x->>'origin_id' HAVING count(*)>1) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 v_selected:=ARRAY(SELECT DISTINCT (x->>'bundle_id')::uuid FROM jsonb_array_elements(p_selection->'items') x
  WHERE x->>'bundle_id' IS NOT NULL ORDER BY 1);
 IF v_bundles<>v_selected THEN RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400'; END IF;
 IF cardinality(v_bundles)=0 THEN
  RETURN jsonb_build_object('request_id',NULL,'reason','no_bundle','lines',v_lines);
 END IF;

 -- Lock every bundle, then its claim lines, reservations and grants in deterministic
 -- order. All eligibility and quantity facts below are re-read after these locks.
 PERFORM b.id FROM claims.bundles b WHERE b.tenant_id=v[1] AND b.store_id=v[2] AND b.id=ANY(v_bundles) ORDER BY b.id FOR UPDATE;
 IF (SELECT count(*) FROM claims.bundles b WHERE b.tenant_id=v[1] AND b.store_id=v[2] AND b.id=ANY(v_bundles) AND b.purged_at IS NULL)<>cardinality(v_bundles) THEN
  RAISE EXCEPTION 'not found' USING ERRCODE='PT404';
 END IF;
 PERFORM l.offer_id FROM claims.lines l WHERE l.tenant_id=v[1] AND l.store_id=v[2] AND l.bundle_id=ANY(v_bundles)
  ORDER BY l.bundle_id,l.offer_id FOR UPDATE;
 PERFORM x.request_id FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=ANY(v_bundles)
  ORDER BY x.bundle_id,x.request_id FOR UPDATE;
 PERFORM g.request_id FROM claims.merchant_origin_grants g WHERE g.tenant_id=v[1] AND g.store_id=v[2] AND g.bundle_id=ANY(v_bundles)
  ORDER BY g.bundle_id,g.request_id FOR UPDATE;
 PERFORM o.id FROM live.offers o WHERE o.tenant_id=v[1] AND o.store_id=v[2]
  AND EXISTS(SELECT 1 FROM claims.lines l WHERE l.tenant_id=v[1] AND l.store_id=v[2] AND l.bundle_id=ANY(v_bundles) AND l.offer_id=o.id)
  ORDER BY o.id FOR SHARE;

 v_reason:=claims.for_buyer_peer_state(v_conversation,v_bundles);
 IF v_reason='bundle_buyer_mismatch' THEN RAISE EXCEPTION 'bundle_buyer_mismatch' USING ERRCODE='PT409'; END IF;
 IF v_reason='' AND NOT identity.principal_holds(v[1],v[2],v[3],ARRAY['live:manage']) THEN v_reason:='permission'; END IF;
 FOR item IN SELECT value FROM jsonb_array_elements(p_selection->'items') ORDER BY (value->>'ordinal')::integer LOOP
  IF item->>'bundle_id' IS NULL THEN CONTINUE; END IF;
  v_bundle:=(item->>'bundle_id')::uuid; v_offer:=(item->>'offer_id')::uuid; v_sku:=(item->>'sku_id')::uuid;
  v_origin:=(item->>'origin_id')::uuid; v_ordinal:=(item->>'ordinal')::integer; v_quantity:=(item->>'quantity')::bigint;
  SELECT l.quantity,o.live_price_minor,o.active,l.applied_version,l.version,z.lifecycle,
   l.quantity-(SELECT coalesce(sum(u.quantity),0) FROM claims.live_price_uses u
    JOIN checkout.orders ord ON ord.tenant_id=u.tenant_id AND ord.store_id=u.store_id AND ord.id=u.order_id AND ord.commercial_state<>'CANCELLED'
    WHERE u.tenant_id=l.tenant_id AND u.store_id=l.store_id AND u.bundle_id=l.bundle_id AND u.offer_id=l.offer_id) AS remaining
   INTO r FROM claims.lines l JOIN claims.bundles bb ON bb.tenant_id=l.tenant_id AND bb.store_id=l.store_id AND bb.id=l.bundle_id
   JOIN live.offers o ON o.tenant_id=l.tenant_id AND o.store_id=l.store_id AND o.id=l.offer_id AND o.sku_id=l.sku_id AND o.session_id=bb.session_id
   JOIN live.sessions z ON z.tenant_id=bb.tenant_id AND z.store_id=bb.store_id AND z.id=bb.session_id
   WHERE l.tenant_id=v[1] AND l.store_id=v[2] AND l.bundle_id=v_bundle AND l.offer_id=v_offer AND l.sku_id=v_sku;
  IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
  -- Same physical origin may have separate client line identities; its combined
  -- quantity still cannot exceed the actual claim ledger allowance.
  IF (SELECT sum((x->>'quantity')::bigint) FROM jsonb_array_elements(p_selection->'items') x
   WHERE x->>'bundle_id'=v_bundle::text AND x->>'offer_id'=v_offer::text)>r.remaining THEN
   RAISE EXCEPTION 'claim_quantity_exceeded' USING ERRCODE='PT409';
  END IF;
  line_reason:=v_reason;
  IF line_reason='' AND r.lifecycle='archived' THEN line_reason:='session_archived'; END IF;
  IF line_reason='' AND EXISTS(SELECT 1 FROM claims.lines x JOIN live.offers xo ON xo.tenant_id=x.tenant_id AND xo.store_id=x.store_id AND xo.id=x.offer_id
   WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=v_bundle AND x.sku_id=v_sku AND x.offer_id<>v_offer
    AND (ROW(xo.active,x.applied_version IS NOT DISTINCT FROM x.version)>ROW(r.active,r.applied_version IS NOT DISTINCT FROM r.version)
     OR (ROW(xo.active,x.applied_version IS NOT DISTINCT FROM x.version)=ROW(r.active,r.applied_version IS NOT DISTINCT FROM r.version) AND x.offer_id<v_offer))) THEN
   line_reason:='superseded_origin';
  END IF;
  IF line_reason='' AND r.live_price_minor IS NULL THEN line_reason:='no_live_price'; END IF;
  v_price:=CASE WHEN line_reason='' THEN r.live_price_minor ELSE NULL END;
  v_lines:=v_lines||jsonb_build_array(jsonb_build_object('ordinal',v_ordinal,'origin_id',v_origin,'bundle_id',v_bundle,
   'offer_id',v_offer,'sku_id',v_sku,'quantity',v_quantity,'line_version',r.version,'live_price_minor',v_price,'reason',line_reason));
 END LOOP;
 IF clock_timestamp()>=v_deadline THEN RAISE EXCEPTION 'attempt_expired' USING ERRCODE='PT409'; END IF;
 FOREACH v_bundle IN ARRAY v_bundles LOOP
  -- The existing bundle/reservation locks serialize fresh intents. A committed
  -- CANCELLED order cannot regain a live commercial state through native order
  -- commands, so this read needs no backwards order lock. Retain the exact old
  -- order/creator binding and every origin, use and grant as historical audit.
  UPDATE inbox.order_for_buyer x SET state='released',updated_at=clock_timestamp()
   WHERE x.tenant_id=v[1] AND x.store_id=v[2] AND x.bundle_id=v_bundle AND x.state='placed'
    AND x.create_attempt_id IS DISTINCT FROM p_attempt
    AND EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=x.tenant_id AND o.store_id=x.store_id
     AND o.id=x.order_id AND o.owner_id=x.buyer_id AND o.create_attempt_id IS NOT DISTINCT FROM x.create_attempt_id
     AND o.commercial_state='CANCELLED');
  -- No stale timestamp recovery here. Only the fenced terminal settlement/sweep may
  -- release a pending attempt. The original partial unique index arbitrates races
  -- with legacy callers that do not lock the bundle row.
  IF EXISTS(SELECT 1 FROM inbox.order_for_buyer x WHERE x.tenant_id=v[1] AND x.store_id=v[2]
   AND x.bundle_id=v_bundle AND x.state IN ('pending','placed') AND x.create_attempt_id IS DISTINCT FROM p_attempt) THEN
   RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409';
  END IF;
  BEGIN
   INSERT INTO inbox.order_for_buyer(tenant_id,store_id,bundle_id,request_id,idempotency_key_hash,request_hash,buyer_id,state,
    conversation_id,principal_id,create_attempt_id,expires_at)
    VALUES(v[1],v[2],v_bundle,p_attempt,sha256(convert_to('order-attempt-reservation:'||p_attempt::text,'UTF8')),v_request_hash,
     v_owner,'pending',v_conversation,v[3],p_attempt,v_deadline)
    ON CONFLICT(tenant_id,store_id,request_id,bundle_id) DO NOTHING;
  EXCEPTION WHEN unique_violation THEN
   RAISE EXCEPTION 'bundle_already_ordered' USING ERRCODE='PT409';
  END;
  IF EXISTS(SELECT 1 FROM jsonb_array_elements(v_lines) x WHERE x->>'bundle_id'=v_bundle::text AND x->>'live_price_minor' IS NOT NULL) THEN
   INSERT INTO claims.merchant_origin_grants(tenant_id,store_id,request_id,buyer_id,bundle_id,principal_id,expires_at,create_attempt_id)
    VALUES(v[1],v[2],p_attempt,v_owner,v_bundle,v[3],v_deadline,p_attempt)
    ON CONFLICT(tenant_id,store_id,request_id,bundle_id) DO NOTHING;
  END IF;
 END LOOP;
 IF v_reason='' THEN
  SELECT coalesce(string_agg(DISTINCT x->>'reason',',' ORDER BY x->>'reason'),'') INTO v_reason
   FROM jsonb_array_elements(v_lines) x WHERE x->>'reason'<>'';
 END IF;
 RETURN jsonb_build_object('request_id',p_attempt,'reason',v_reason,'lines',v_lines);
END $$;
ALTER FUNCTION claims.order_attempt_admission_origins(uuid,jsonb) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.order_attempt_admission_origins(uuid,jsonb) FROM PUBLIC;
GRANT USAGE ON SCHEMA claims TO commerce_order_attempt_owner;
GRANT EXECUTE ON FUNCTION claims.order_attempt_admission_origins(uuid,jsonb) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION claims.order_attempt_admission_origins(uuid,jsonb) IS
 'claims C1 internal helper: coordinator only, exact candidate/hash and peer/permission checks, sorted domain locks, server live prices per origin, creator-bound reservation/grant execute_by; no C2 effects or provider calls.';
