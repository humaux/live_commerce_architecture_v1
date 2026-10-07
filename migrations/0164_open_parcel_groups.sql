-- 0164_open_parcel_groups.sql — W3-U4 (unit w3-u4-parcel-merge-ui): read the store's OPEN parcel groups so the orders page can show
--   their ship/dissolve panels after a reload (0146 gave the UI no way to list groups; a reload used to strand an OPEN group).
-- Purpose: one read-only SECURITY DEFINER, fulfillment.read_open_parcel_groups(p_hash, p_store) -> jsonb
--   [{group_id, version, created_at, members:[{order_id, order_number, recipient_masked}]}], newest group first, at most 200
--   groups (2..20 members each, so at most 4000 member rows). Display fields only: the recipient is MASKED with the exact
--   expression the orders list uses (0110 `recipient_masked`: first non-whitespace character + '***', '—' for a blank name);
--   no full name, phone, address, amount or payment field is ever returned. SHIPPED and DISSOLVED groups are not listed.
--   Also amends fulfillment.read_merge_suggestions (0146) IN PLACE (CREATE OR REPLACE; 0164 is unmerged and applied to no shared
--   DB): it returned the full recipient_name on a read the orders page fires on every load, so it now returns
--   {recipient_masked, order_ids} with the same list mask, computed after the GROUP BY (P1-A of the independent review).
-- Depends on: 0146 (fulfillment.parcel_groups / parcel_group_orders and their commerce_checkout_writer SELECT + RLS policies),
--   0110 (the recipient_masked / order_number display rule it mirrors), identity.resolve_access (orders:read), roles
--   commerce_runtime and commerce_checkout_writer.
-- Used by: internal/merchantorders/parcels.go (OpenParcelGroups), internal/httpapi/parcels.go (GET /parcel-groups), the admin
--   orders page (both reads). Tests: TestParcelGroupOpenRead and TestParcelGroupMergeSuggestionsMasked (REAL_PG),
--   TestMerchantOrdersV2PickListReadAuthority, MF02, WAS02, r2_integration_upgrade_test (count 87 -> 88; the replace adds none).
-- Invariants: orders:read only (no write, no lock, no GUC left behind: set_config is transaction-local); tenant and store come
--   from identity.resolve_access, never from the caller; SECURITY DEFINER SET search_path=pg_catalog, REVOKE ALL FROM PUBLIC,
--   EXECUTE commerce_runtime only (same shape as read_merge_suggestions / read_parcel_group_ids); I05 (reads no payments.*).
-- Status: REAL_PG (forward-only; adds one function and replaces one with identical attributes/ACL, no table/grant/policy delta).

DO $$
BEGIN
 IF to_regclass('fulfillment.parcel_groups') IS NULL OR to_regclass('fulfillment.parcel_group_orders') IS NULL THEN
  RAISE EXCEPTION '0164 requires 0146 (parcel groups)'; END IF;
END $$;

CREATE FUNCTION fulfillment.read_open_parcel_groups(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_out jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid open parcel groups request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 -- ponytail: lists the 200 newest OPEN groups; paginate if a store routinely holds more unshipped groups than that.
 SELECT coalesce(jsonb_agg(jsonb_build_object('group_id',g.id,'version',g.version,'created_at',g.created_at,'members',g.members)
   ORDER BY g.created_at DESC,g.id DESC),'[]'::jsonb) INTO v_out
 FROM (
  SELECT x.id,x.version,x.created_at,(
   -- order_number and recipient_masked are the 0110 orders-list expressions verbatim (leading Unicode whitespace is skipped,
   -- an all-blank or missing name falls back to the placeholder), so a member reads exactly like its list row.
   SELECT coalesce(jsonb_agg(jsonb_build_object('order_id',o.id,'order_number','LC-'||upper(replace(o.id::text,'-','')),
     'recipient_masked',coalesce(left(nullif(regexp_replace(o.snapshot#>>'{destination,recipient_name}',
      '^[\s\u0085\u00a0\u1680\u180e\u2000-\u200f\u2028-\u202f\u205f\u2060\u3000\ufeff]+',''),''),1)||'***','—'))
     ORDER BY o.created_at,o.id),'[]'::jsonb)
   FROM fulfillment.parcel_group_orders m
   JOIN checkout.orders o ON o.tenant_id=m.tenant_id AND o.store_id=m.store_id AND o.id=m.order_id
   WHERE m.tenant_id=x.tenant_id AND m.store_id=x.store_id AND m.group_id=x.id) AS members
  FROM fulfillment.parcel_groups x
  WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.state='OPEN'
  ORDER BY x.created_at DESC,x.id DESC LIMIT 200) g;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'orders:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_out;
END $$;
ALTER FUNCTION fulfillment.read_open_parcel_groups(bytea,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.read_open_parcel_groups(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.read_open_parcel_groups(bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.read_open_parcel_groups(bytea,uuid) IS 'internal/merchantorders OpenParcelGroups only; EXECUTE commerce_runtime. orders:read; [{group_id, version, created_at, members:[{order_id, order_number, recipient_masked}]}] of the store''s OPEN groups, newest first, at most 200; recipient masked like the orders list, no name/phone/address/amount. Read only, no lock.';

-- ---------------------------------------------------------------------------------------------------
-- fulfillment.read_merge_suggestions: 0146 returned the FULL recipient_name, and the orders page fetches this on every load, so a
-- list-level read leaked a detail-level field to the browser (P1-A of the W3-U4 review). Replace it with a masked projection.
-- CREATE OR REPLACE drops attributes it does not repeat: SECURITY DEFINER + search_path are repeated, then the ALTER OWNER /
-- REVOKE / GRANT / COMMENT of 0146 are re-run so the final ACL is byte-identical (MF02 pins prosecdef / proconfig / grantees).
-- ---------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION fulfillment.read_merge_suggestions(p_hash bytea,p_store uuid) RETURNS jsonb
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
 -- The recipient is masked AFTER the GROUP BY (min(c.recipient_name) is the group's display name) with the 0110 orders-list
 -- expression verbatim, so the full name never leaves the definer: this read fires on every orders-page load (list level).
 SELECT coalesce(jsonb_agg(jsonb_build_object('recipient_masked',
   coalesce(left(nullif(regexp_replace(g.recipient_name,
    '^[\s\u0085\u00a0\u1680\u180e\u2000-\u200f\u2028-\u202f\u205f\u2060\u3000\ufeff]+',''),''),1)||'***','—'),
   'order_ids',to_jsonb(g.ids)) ORDER BY g.first_at,g.ids[1]),'[]'::jsonb)
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
COMMENT ON FUNCTION fulfillment.read_merge_suggestions(bytea,uuid) IS 'internal/fulfillment Parcels.Suggestions only; EXECUTE commerce_runtime. orders:read; [{recipient_masked, order_ids}] of one owner + one destination hash, every order passing parcel_merge_block. recipient_masked is the orders-list mask (first non-blank character + ***, or the dash placeholder), never the full name or phone (0164). Read only.';
