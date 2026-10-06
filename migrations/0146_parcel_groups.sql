-- 0146_parcel_groups.sql — W3-07B (unit w3-07b-parcel-merge): ship several orders of one buyer to one address as ONE parcel.
-- Purpose: parcel groups (fulfillment.parcel_groups / parcel_group_orders) and the definers that create, dissolve and ship them.
--   A group merges PARCELS only: every order keeps its own payment, refund and totals; nothing here reads or writes payments.*
--   (I05) or changes an order amount. One order belongs to at most one group (PK); COD, pay-at-pickup and CVS orders are never
--   mergeable in v1 (owner ruling). Shipping a group = one record_manual_shipment per member, same tracking number (Go, one tx).
-- Depends on: 0001 (ops.audit_events, ops.command_results), 0003 (identity.resolve_access), 0063/0107 (checkout.orders,
--   fulfillment.manual_shipment_eligible, manual_shipment_heads/_versions), 0073 (fulfillment.cvs_shipments), roles
--   commerce_runtime and commerce_checkout_writer.
-- Used by: internal/fulfillment/parcels.go (Go wrappers), internal/httpapi/parcels.go (routes), internal/merchantorders
--   (single-shipment guard, pick list / export annotation), internal/merchanttools/tracking_import.go (guard).
--   Tests: TestParcelGroup*, TestMF02 (manual_fulfilment_schema_test), TestMerchantOrdersV2PickListReadAuthority, TestWAS02.
-- Invariants: I05 (no payment or amount write); group = 2..20 orders of ONE owner with ONE destination_hash computed in SQL from
--   the order snapshots (never from the client); members are locked FOR UPDATE in id order (no deadlock between creators, the
--   single-shipment guard and the group shipment); every definer is SECURITY DEFINER SET search_path=pg_catalog, REVOKE ALL
--   FROM PUBLIC, EXECUTE commerce_runtime only. Dissolving deletes the member rows (so the orders may regroup) and keeps the
--   group row as history.

DO $$
BEGIN
 IF to_regprocedure('fulfillment.manual_shipment_eligible(uuid,uuid,uuid)') IS NULL
  OR to_regclass('fulfillment.manual_shipment_heads') IS NULL OR to_regclass('fulfillment.cvs_shipments') IS NULL THEN
  RAISE EXCEPTION '0146 requires 0063/0107 (manual shipments) and 0073 (cvs_shipments)'; END IF;
END $$;

CREATE TABLE fulfillment.parcel_groups (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL, owner_id uuid NOT NULL,
 destination_hash bytea NOT NULL CHECK(octet_length(destination_hash)=32),
 state text NOT NULL CHECK(state IN ('OPEN','SHIPPED','DISSOLVED')),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_by uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,id),
 UNIQUE(tenant_id,store_id,id,owner_id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES identity.memberships(tenant_id,principal_id)
);
-- PRIMARY KEY(tenant_id,store_id,order_id) is the "one order, at most one group" rule; the composite FKs make a member
-- belong to the group's owner and to a real order of that owner.
CREATE TABLE fulfillment.parcel_group_orders (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, group_id uuid NOT NULL, owner_id uuid NOT NULL, order_id uuid NOT NULL,
 PRIMARY KEY(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,group_id,owner_id) REFERENCES fulfillment.parcel_groups(tenant_id,store_id,id,owner_id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id)
);
CREATE INDEX parcel_group_orders_group ON fulfillment.parcel_group_orders(tenant_id,store_id,group_id,order_id);

ALTER TABLE fulfillment.parcel_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE fulfillment.parcel_groups FORCE ROW LEVEL SECURITY;
ALTER TABLE fulfillment.parcel_group_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE fulfillment.parcel_group_orders FORCE ROW LEVEL SECURITY;
REVOKE ALL ON fulfillment.parcel_groups, fulfillment.parcel_group_orders FROM PUBLIC;
-- Only the definer owner touches these tables (Go never SELECTs them directly: every read goes through a definer).
GRANT SELECT,INSERT ON fulfillment.parcel_groups TO commerce_checkout_writer;
GRANT UPDATE(state,version) ON fulfillment.parcel_groups TO commerce_checkout_writer;
GRANT SELECT,INSERT,DELETE ON fulfillment.parcel_group_orders TO commerce_checkout_writer;
CREATE POLICY parcel_groups_writer ON fulfillment.parcel_groups TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY parcel_group_orders_writer ON fulfillment.parcel_group_orders TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
COMMENT ON TABLE fulfillment.parcel_groups IS 'W3-07B: one parcel shared by 2..20 orders of one buyer and one destination (state OPEN/SHIPPED/DISSOLVED). Merges parcels only, never money (I05). Written by the fulfillment.*_parcel_group definers only.';
COMMENT ON TABLE fulfillment.parcel_group_orders IS 'W3-07B: group membership; PRIMARY KEY (tenant,store,order) = one order, at most one group. Rows are deleted on dissolve and kept after shipping.';
COMMENT ON COLUMN fulfillment.parcel_groups.destination_hash IS 'sha256 of the normalized recipient + phone + home address of the member snapshots, computed by fulfillment.parcel_destination_hash; never supplied by a client.';

-- ---------------------------------------------------------------------------------------------------
-- Pure/INVOKER helpers (no grants: only the definers below, same owner, call them).
-- ---------------------------------------------------------------------------------------------------
-- Normalized destination fingerprint: NULL for anything but a home delivery with an address line.
CREATE FUNCTION fulfillment.parcel_destination_hash(p_snapshot jsonb,p_country text) RETURNS bytea
LANGUAGE sql IMMUTABLE SET search_path=pg_catalog AS $$
 SELECT CASE
  WHEN p_snapshot#>>'{destination,kind}' IS DISTINCT FROM 'home'
   OR btrim(coalesce(p_snapshot#>>'{destination,home_address,line1}',''))='' THEN NULL
  ELSE sha256(convert_to(concat_ws(chr(31),
   lower(btrim(coalesce(p_country,''))),
   lower(regexp_replace(btrim(coalesce(p_snapshot#>>'{destination,recipient_name}','')),'\s+',' ','g')),
   regexp_replace(coalesce(p_snapshot#>>'{destination,phone}',''),'[^0-9]','','g'),
   lower(regexp_replace(btrim(coalesce(p_snapshot#>>'{destination,home_address,region}','')),'\s+',' ','g')),
   lower(regexp_replace(btrim(coalesce(p_snapshot#>>'{destination,home_address,city}','')),'\s+',' ','g')),
   lower(regexp_replace(btrim(coalesce(p_snapshot#>>'{destination,home_address,postal_code}','')),'\s+',' ','g')),
   lower(regexp_replace(btrim(coalesce(p_snapshot#>>'{destination,home_address,line1}','')),'\s+',' ','g')),
   lower(regexp_replace(btrim(coalesce(p_snapshot#>>'{destination,home_address,line2}','')),'\s+',' ','g'))),'UTF8'))
 END
$$;
ALTER FUNCTION fulfillment.parcel_destination_hash(jsonb,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.parcel_destination_hash(jsonb,text) FROM PUBLIC;
COMMENT ON FUNCTION fulfillment.parcel_destination_hash(jsonb,text) IS 'W3-07B internal only: sha256 of the normalized home destination of an order snapshot (NULL when not a home delivery). Pure, no table access, no grants.';

-- The single mergeability predicate shared by create_parcel_group and read_merge_suggestions. NULL = mergeable, otherwise the
-- refusal code. COD first (owner ruling: the carrier collects ONE amount per parcel, merging would change it = money path).
CREATE FUNCTION fulfillment.parcel_merge_block(p_tenant uuid,p_store uuid,p_order uuid) RETURNS text
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT (SELECT CASE
   WHEN o.id IS NULL THEN 'order_not_found'
   WHEN o.payment_mode='cash_on_delivery' THEN 'cod_not_mergeable'
   WHEN o.payment_mode='pay_at_pickup' OR coalesce(o.snapshot#>>'{destination,kind}','') IN ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart')
    OR EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND c.order_id=o.id)
    THEN 'cvs_not_mergeable'
   WHEN NOT fulfillment.manual_shipment_eligible(o.tenant_id,o.store_id,o.id) THEN 'not_shippable'
   WHEN EXISTS(SELECT 1 FROM fulfillment.manual_shipment_heads h WHERE h.tenant_id=o.tenant_id AND h.store_id=o.store_id AND h.order_id=o.id)
    OR fulfillment.parcel_destination_hash(o.snapshot,o.country) IS NULL THEN 'not_mergeable'
   WHEN EXISTS(SELECT 1 FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=o.tenant_id AND m.store_id=o.store_id AND m.order_id=o.id)
    THEN 'already_in_group'
   END FROM (SELECT 1) one LEFT JOIN checkout.orders o ON o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_order)
$$;
ALTER FUNCTION fulfillment.parcel_merge_block(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.parcel_merge_block(uuid,uuid,uuid) FROM PUBLIC;
COMMENT ON FUNCTION fulfillment.parcel_merge_block(uuid,uuid,uuid) IS 'W3-07B internal only: NULL when the order may join a parcel group, else cod_not_mergeable | cvs_not_mergeable | not_shippable | not_mergeable | already_in_group | order_not_found. Reuses manual_shipment_eligible. No grants.';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.read_merge_suggestions (orders:read): same owner + same destination, >=2 mergeable orders, read only.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.read_merge_suggestions(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_out jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid merge suggestions request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 -- ponytail: scans the 2000 oldest unshipped card/transfer orders (orders_unshipped index) and lists at most 100 suggestions of
 -- at most 20 orders each; raise the caps (or paginate) if a store routinely holds more unshipped orders than that.
 SELECT coalesce(jsonb_agg(jsonb_build_object('recipient_name',g.recipient_name,'order_ids',to_jsonb(g.ids)) ORDER BY g.first_at,g.ids[1]),'[]'::jsonb)
  INTO v_out FROM (
  SELECT c.owner_id,c.h,min(c.created_at) AS first_at,min(c.recipient_name) AS recipient_name,
   (array_agg(c.id ORDER BY c.created_at,c.id))[1:20] AS ids
  FROM (SELECT o.id,o.owner_id,o.created_at,fulfillment.parcel_destination_hash(o.snapshot,o.country) AS h,
         coalesce(o.snapshot#>>'{destination,recipient_name}','') AS recipient_name
        FROM checkout.orders o
        WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.commercial_state='CONFIRMED' AND o.fulfillment_state='MANUAL_UNASSIGNED'
         AND o.payment_mode NOT IN ('cash_on_delivery','pay_at_pickup')
        ORDER BY o.created_at,o.id LIMIT 2000) c
  WHERE c.h IS NOT NULL AND fulfillment.parcel_merge_block(s.tenant_id,p_store,c.id) IS NULL
  GROUP BY c.owner_id,c.h HAVING count(*)>=2 ORDER BY min(c.created_at) LIMIT 100) g;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_out;
END $$;
ALTER FUNCTION fulfillment.read_merge_suggestions(bytea,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.read_merge_suggestions(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.read_merge_suggestions(bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.read_merge_suggestions(bytea,uuid) IS 'internal/fulfillment Parcels.Suggestions only; EXECUTE commerce_runtime. orders:read; [{recipient_name, order_ids}] of one owner + one destination hash, every order passing parcel_merge_block. Read only.';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.create_parcel_group (fulfillment:write, Idempotency-Key)
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.create_parcel_group(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,p_orders uuid[]) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_expiry timestamptz; s record; v_final record; v_ids uuid[]; v_id uuid; v_n int; v_owner uuid; v_hash bytea; v_o uuid; v_h bytea; v_block text;
 v_group uuid:=gen_random_uuid(); v_saved bytea; v_response jsonb; v_replay boolean:=false; v_fail_code text; v_fail_msg text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_orders IS NULL OR cardinality(p_orders) NOT BETWEEN 2 AND 20
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid parcel group request' USING ERRCODE='PT400'; END IF;
 SELECT array_agg(x ORDER BY x) INTO v_ids FROM (SELECT DISTINCT unnest(p_orders) AS x) d;
 IF cardinality(v_ids)<>cardinality(p_orders) THEN RAISE EXCEPTION 'invalid parcel group request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 -- Lock every member in id order (the same order the single-shipment guard and the group shipment use), then read state:
 -- a concurrent creator that won the race has committed its membership by the time the checks below run.
 PERFORM 1 FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=ANY(v_ids) ORDER BY o.id FOR UPDATE;
 GET DIAGNOSTICS v_n=ROW_COUNT;
 SELECT c.request_hash,c.response INTO v_saved,v_response FROM ops.command_results c
  WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store AND c.operation='fulfillment.parcel_group.create' AND c.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict'; ELSE v_replay:=true; END IF;
 ELSIF v_n<>cardinality(v_ids) THEN
  v_fail_code:='PT404'; v_fail_msg:='order not found';  -- missing and other-store ids are indistinguishable (I01)
 ELSE
  FOREACH v_id IN ARRAY v_ids LOOP
   v_block:=fulfillment.parcel_merge_block(s.tenant_id,p_store,v_id);
   IF v_block IS NOT NULL THEN
    v_fail_code:=CASE WHEN v_block='not_shippable' THEN 'PT422' ELSE 'PT409' END; v_fail_msg:=v_block; EXIT; END IF;
   SELECT o.owner_id,fulfillment.parcel_destination_hash(o.snapshot,o.country) INTO v_o,v_h FROM checkout.orders o
    WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=v_id;
   IF v_owner IS NULL THEN v_owner:=v_o; v_hash:=v_h;
   ELSIF v_o<>v_owner THEN v_fail_code:='PT409'; v_fail_msg:='owner_mismatch'; EXIT;
   ELSIF v_h<>v_hash THEN v_fail_code:='PT409'; v_fail_msg:='destination_mismatch'; EXIT; END IF;
  END LOOP;
  IF v_fail_code IS NULL THEN
   BEGIN
    INSERT INTO fulfillment.parcel_groups(tenant_id,store_id,id,owner_id,destination_hash,state,created_by)
    VALUES(s.tenant_id,p_store,v_group,v_owner,v_hash,'OPEN',s.principal_id);
    INSERT INTO fulfillment.parcel_group_orders(tenant_id,store_id,group_id,owner_id,order_id)
    SELECT s.tenant_id,p_store,v_group,v_owner,x FROM unnest(v_ids) AS x;
   EXCEPTION WHEN unique_violation THEN
    -- Belt and braces: the row locks above already serialize creators; map a lost race to the same code as a plain duplicate.
    v_fail_code:='PT409'; v_fail_msg:='already_in_group';
   END;
  END IF;
  IF v_fail_code IS NULL THEN
   INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'fulfillment.parcel_group_created');
   v_response:=jsonb_build_object('id',v_group,'state','OPEN','version',1,'order_ids',to_jsonb(v_ids));
   BEGIN
    INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
    VALUES(s.tenant_id,p_store,'fulfillment.parcel_group.create',p_key,p_request_hash,v_response,s.principal_id);
   EXCEPTION WHEN unique_violation THEN
    -- Another transaction committed the same Idempotency-Key for a different order set while this one ran (disjoint sets do not
    -- block each other on the order locks). The key is taken: refuse; the raise below aborts the transaction, so the group
    -- rows inserted above vanish with it.
    v_fail_code:='PT409'; v_fail_msg:='idempotency_conflict';
   END;
  END IF;
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 SELECT se.expires_at INTO v_expiry FROM identity.sessions se WHERE se.token_hash=p_hash AND se.audience='merchant' AND se.revoked_at IS NULL;
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=clock_timestamp() THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION fulfillment.create_parcel_group(bytea,uuid,text,bytea,uuid[]) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.create_parcel_group(bytea,uuid,text,bytea,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.create_parcel_group(bytea,uuid,text,bytea,uuid[]) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.create_parcel_group(bytea,uuid,text,bytea,uuid[]) IS 'internal/fulfillment Parcels.Create only; EXECUTE commerce_runtime. fulfillment:write, Idempotency-Key. 2..20 distinct orders locked FOR UPDATE in id order; refuses cod_not_mergeable | cvs_not_mergeable | not_mergeable | already_in_group | owner_mismatch | destination_mismatch (409) and not_shippable (422). Writes only parcel_groups/_orders, one audit row and the command receipt (I05: no payment write).';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.dissolve_parcel_group (fulfillment:write, OPEN only, CAS on version)
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.dissolve_parcel_group(p_hash bytea,p_store uuid,p_group uuid,p_expected_version bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_expiry timestamptz; s record; v_final record; g record; v_fail_code text; v_fail_msg text; v_response jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_group IS NULL OR p_expected_version IS NULL OR p_expected_version<1
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid parcel group request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT x.state,x.version INTO g FROM fulfillment.parcel_groups x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_group FOR UPDATE;
 IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='group not found';
 ELSIF g.state<>'OPEN' THEN v_fail_code:='PT409'; v_fail_msg:='group_not_open';
 ELSIF g.version<>p_expected_version THEN v_fail_code:='PT409'; v_fail_msg:='version_changed';
 ELSE
  DELETE FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=s.tenant_id AND m.store_id=p_store AND m.group_id=p_group;
  UPDATE fulfillment.parcel_groups x SET state='DISSOLVED',version=x.version+1 WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_group;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'fulfillment.parcel_group_dissolved');
  v_response:=jsonb_build_object('id',p_group,'state','DISSOLVED','version',g.version+1);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 SELECT se.expires_at INTO v_expiry FROM identity.sessions se WHERE se.token_hash=p_hash AND se.audience='merchant' AND se.revoked_at IS NULL;
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=clock_timestamp() THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION fulfillment.dissolve_parcel_group(bytea,uuid,uuid,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.dissolve_parcel_group(bytea,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.dissolve_parcel_group(bytea,uuid,uuid,bigint) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.dissolve_parcel_group(bytea,uuid,uuid,bigint) IS 'internal/fulfillment Parcels.Dissolve only; EXECUTE commerce_runtime. fulfillment:write; OPEN groups only (group_not_open 409), CAS on version (version_changed 409); deletes the member rows so the orders may regroup, keeps the group row as DISSOLVED. Locks the group row first (same order as the group shipment).';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.begin_parcel_group_shipment / mark_parcel_group_shipped: the two SQL halves around the Go loop that calls
-- record_manual_shipment once per member. begin locks the group row and returns its members in id order; mark verifies every
-- member is shipped with one carrier + tracking number and flips OPEN -> SHIPPED (idempotent on SHIPPED, for a replay).
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.begin_parcel_group_shipment(p_hash bytea,p_store uuid,p_group uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_expiry timestamptz; s record; v_final record; g record; v_ids uuid[]; v_fail_code text; v_fail_msg text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_group IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid parcel group request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT x.state,x.version INTO g FROM fulfillment.parcel_groups x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_group FOR UPDATE;
 IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='group not found';
 ELSIF g.state='DISSOLVED' THEN v_fail_code:='PT409'; v_fail_msg:='group_not_open'; END IF;
 SELECT coalesce(array_agg(m.order_id ORDER BY m.order_id),'{}'::uuid[]) INTO v_ids FROM fulfillment.parcel_group_orders m
  WHERE m.tenant_id=s.tenant_id AND m.store_id=p_store AND m.group_id=p_group;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 SELECT se.expires_at INTO v_expiry FROM identity.sessions se WHERE se.token_hash=p_hash AND se.audience='merchant' AND se.revoked_at IS NULL;
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=clock_timestamp() THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN jsonb_build_object('id',p_group,'state',g.state,'version',g.version,'order_ids',to_jsonb(v_ids));
END $$;
ALTER FUNCTION fulfillment.begin_parcel_group_shipment(bytea,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.begin_parcel_group_shipment(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.begin_parcel_group_shipment(bytea,uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.begin_parcel_group_shipment(bytea,uuid,uuid) IS 'internal/fulfillment Parcels.Ship only; EXECUTE commerce_runtime. fulfillment:write; locks the group row FOR UPDATE (blocks a concurrent dissolve) and returns {id,state,version,order_ids} with the members in id order. DISSOLVED -> group_not_open 409. Writes nothing.';

CREATE FUNCTION fulfillment.mark_parcel_group_shipped(p_hash bytea,p_store uuid,p_group uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_expiry timestamptz; s record; v_final record; g record; t record; v_fail_code text; v_fail_msg text; v_version bigint;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_group IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid parcel group request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT x.state,x.version INTO g FROM fulfillment.parcel_groups x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_group FOR UPDATE;
 IF NOT FOUND THEN v_fail_code:='PT404'; v_fail_msg:='group not found';
 ELSIF g.state='DISSOLVED' THEN v_fail_code:='PT409'; v_fail_msg:='group_not_open';
 ELSIF g.state='OPEN' THEN
  -- Every member must be MERCHANT_SHIPPED on a SHIPPED head and all heads must carry the same carrier + tracking number.
  -- LEFT JOINs + FILTER: an unshipped member must still count in total (an inner join on the shipped state would drop it).
  SELECT count(*) AS total,count(*) FILTER (WHERE o.fulfillment_state='MERCHANT_SHIPPED' AND v.order_id IS NOT NULL) AS shipped,
   count(DISTINCT v.carrier_code||'/'||v.tracking_number) AS tracked INTO t
   FROM fulfillment.parcel_group_orders m
   JOIN checkout.orders o ON o.tenant_id=m.tenant_id AND o.store_id=m.store_id AND o.id=m.order_id
   LEFT JOIN fulfillment.manual_shipment_heads h ON h.tenant_id=m.tenant_id AND h.store_id=m.store_id AND h.order_id=m.order_id
   LEFT JOIN fulfillment.manual_shipment_versions v ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.order_id=h.order_id
    AND v.version=h.current_version AND v.status='SHIPPED'
   WHERE m.tenant_id=s.tenant_id AND m.store_id=p_store AND m.group_id=p_group;
  IF t.total<2 OR t.shipped<>t.total OR t.tracked<>1 THEN v_fail_code:='PT409'; v_fail_msg:='group_incomplete';
  ELSE
   UPDATE fulfillment.parcel_groups x SET state='SHIPPED',version=x.version+1 WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_group
    RETURNING x.version INTO v_version;
   g.version:=v_version; g.state:='SHIPPED';
   INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'fulfillment.parcel_group_shipped');
  END IF;
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 SELECT se.expires_at INTO v_expiry FROM identity.sessions se WHERE se.token_hash=p_hash AND se.audience='merchant' AND se.revoked_at IS NULL;
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=clock_timestamp() THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF v_fail_code IS NOT NULL THEN RAISE EXCEPTION '%',v_fail_msg USING ERRCODE=v_fail_code; END IF;
 RETURN jsonb_build_object('id',p_group,'state',g.state,'version',g.version);
END $$;
ALTER FUNCTION fulfillment.mark_parcel_group_shipped(bytea,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.mark_parcel_group_shipped(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.mark_parcel_group_shipped(bytea,uuid,uuid) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.mark_parcel_group_shipped(bytea,uuid,uuid) IS 'internal/fulfillment Parcels.Ship only; EXECUTE commerce_runtime. fulfillment:write; OPEN -> SHIPPED once every member is MERCHANT_SHIPPED on a SHIPPED head with one shared carrier + tracking number (else group_incomplete 409, the Go transaction rolls back); idempotent on SHIPPED. Writes only the group row and one audit row (I05).';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.guard_parcel_group_orders (fulfillment:write): called BEFORE a single or bulk shipment write. Locks the named
-- orders FOR UPDATE in id order (the same order create/ship use, so a concurrent create cannot slip between this check and the
-- shipment) and returns the orders that sit in an OPEN group. fulfillment.read_parcel_group_ids (orders:read, no lock) feeds the
-- pick list / carrier export.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.guard_parcel_group_orders(p_hash bytea,p_store uuid,p_orders uuid[]) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_expiry timestamptz; s record; v_final record; v_out jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_orders IS NULL OR cardinality(p_orders)>500
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid parcel group request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 PERFORM 1 FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=ANY(p_orders) ORDER BY o.id FOR UPDATE;
 SELECT coalesce(jsonb_agg(jsonb_build_object('order_id',m.order_id,'group_id',m.group_id) ORDER BY m.order_id),'[]'::jsonb) INTO v_out
  FROM fulfillment.parcel_group_orders m JOIN fulfillment.parcel_groups g
   ON g.tenant_id=m.tenant_id AND g.store_id=m.store_id AND g.id=m.group_id AND g.state='OPEN'
  WHERE m.tenant_id=s.tenant_id AND m.store_id=p_store AND m.order_id=ANY(p_orders);
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 SELECT se.expires_at INTO v_expiry FROM identity.sessions se WHERE se.token_hash=p_hash AND se.audience='merchant' AND se.revoked_at IS NULL;
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=clock_timestamp() THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_out;
END $$;
ALTER FUNCTION fulfillment.guard_parcel_group_orders(bytea,uuid,uuid[]) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.guard_parcel_group_orders(bytea,uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.guard_parcel_group_orders(bytea,uuid,uuid[]) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.guard_parcel_group_orders(bytea,uuid,uuid[]) IS 'internal/merchantorders GuardNotGrouped (single PUT) and internal/merchanttools tracking import only; EXECUTE commerce_runtime. fulfillment:write; locks the existing named orders FOR UPDATE in id order and returns [{order_id, group_id}] of those in an OPEN group. Writes nothing.';

CREATE FUNCTION fulfillment.read_parcel_group_ids(p_hash bytea,p_store uuid,p_orders uuid[]) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_out jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_orders IS NULL OR cardinality(p_orders)>500
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid parcel group request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT coalesce(jsonb_agg(jsonb_build_object('order_id',m.order_id,'group_id',m.group_id) ORDER BY m.order_id),'[]'::jsonb) INTO v_out
  FROM fulfillment.parcel_group_orders m WHERE m.tenant_id=s.tenant_id AND m.store_id=p_store AND m.order_id=ANY(p_orders);
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_out;
END $$;
ALTER FUNCTION fulfillment.read_parcel_group_ids(bytea,uuid,uuid[]) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.read_parcel_group_ids(bytea,uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.read_parcel_group_ids(bytea,uuid,uuid[]) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.read_parcel_group_ids(bytea,uuid,uuid[]) IS 'internal/merchantorders pick list and carrier export only; EXECUTE commerce_runtime. orders:read; [{order_id, group_id}] for the named orders that are group members (OPEN or SHIPPED). Read only, no lock.';
