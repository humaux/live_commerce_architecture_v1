-- 0124 tracking-import batches (unit w3-01b, contracts/manual-fulfilment-v1.md Amendment "M-7 revoked"). PLACEHOLDER NUMBER —
-- the integrator renumbers this file; the unit brief draft named it 0128.
--
-- Purpose: the batch record and the lock-free precheck for the home-delivery bulk tracking backfill
-- (CSV upload of shipment rows). One batch row per committed file (UNIQUE per file_sha256), its per-row
-- results jsonb (no buyer PII: only order id, carrier, tracking number, outcome, code), and a SECURITY
-- DEFINER precheck the Go importer (internal/merchanttools/tracking_import.go) uses to decide each row:
-- order exists / is a CVS order / current shipment head (version, status, carrier, tracking).
--
-- Non-goals: no change to fulfillment.record_manual_shipment, the manual_shipment_versions carrier
-- CHECK, the notify shipped-mail triggers, roles, or River kinds/queues; no CVS/ECPay order handling
-- (the precheck only flags them — the Go importer refuses with cvs_order); the final authority (order
-- lock, CAS, MD6 eligibility, audit, shipped mail) stays in fulfillment.record_manual_shipment.
--
-- Depends on: 0001 (identity.memberships), 0013 (checkout.orders snapshot destination), 0063/0073/0107
-- (fulfillment.manual_shipment_heads/_versions, identity.resolve_access), 0072 (fulfillment.cvs_shipments).
--
-- Used by: internal/merchanttools/tracking_import.go (preview/commit/result.csv) via internal/httpapi/merchanttools.go.
-- Roles: commerce_runtime SELECT/INSERT/DELETE on the batches table (app.tenant_id/store_id GUC, catalog-table
--   scope_access style); commerce_checkout_writer owns the precheck definer; commerce_runtime EXECUTE only.

DO $$
DECLARE v_role text;
BEGIN
    IF to_regprocedure('fulfillment.record_manual_shipment(bytea,uuid,uuid,text,bytea,bigint,text,text,text,text,text,text,text)') IS NULL THEN
        RAISE EXCEPTION '0124 requires fulfillment.record_manual_shipment (migration 0107)';
    END IF;
    IF to_regclass('fulfillment.manual_shipment_heads') IS NULL
       OR to_regclass('fulfillment.manual_shipment_versions') IS NULL
       OR to_regclass('fulfillment.cvs_shipments') IS NULL THEN
        RAISE EXCEPTION '0124 requires fulfillment.manual_shipment_heads/_versions and fulfillment.cvs_shipments';
    END IF;
    FOREACH v_role IN ARRAY ARRAY['commerce_runtime','commerce_checkout_writer'] LOOP
        IF to_regrole(v_role) IS NULL THEN RAISE EXCEPTION '0124 requires role %',v_role; END IF;
    END LOOP;
END $$;

-- One committed import file. The UNIQUE(file_sha256) is the durable idempotency guard (I02): the same
-- bytes can only ever produce one batch; re-submitting replays the batch, never re-ships.
CREATE TABLE fulfillment.tracking_import_batches (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL,
    file_sha256 bytea NOT NULL CHECK (octet_length(file_sha256)=32),
    principal_id uuid NOT NULL,
    rows_total smallint NOT NULL CHECK (rows_total BETWEEN 0 AND 500),
    applied smallint NOT NULL CHECK (applied BETWEEN 0 AND 500),
    unchanged smallint NOT NULL CHECK (unchanged BETWEEN 0 AND 500),
    failed smallint NOT NULL CHECK (failed BETWEEN 0 AND 500),
    results jsonb NOT NULL CHECK (jsonb_typeof(results)='array' AND jsonb_array_length(results) <= 500),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, id),
    UNIQUE (tenant_id, store_id, file_sha256)
);

-- FORCE RLS: only the app.tenant_id / app.store_id GUCs (set by platform.WithScope) open the rows;
-- the runtime login touches this table directly (SELECT/INSERT/DELETE) and never a definer.
ALTER TABLE fulfillment.tracking_import_batches ENABLE ROW LEVEL SECURITY;
ALTER TABLE fulfillment.tracking_import_batches FORCE ROW LEVEL SECURITY;
CREATE POLICY tracking_import_batches_scope ON fulfillment.tracking_import_batches TO commerce_runtime
  USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
     AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
  WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
     AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT, INSERT, DELETE ON fulfillment.tracking_import_batches TO commerce_runtime;

COMMENT ON TABLE fulfillment.tracking_import_batches IS
 'internal/merchanttools (tracking_import.go): one committed tracking-import file per store, its per-row results jsonb, and the durable per-file idempotency guard (UNIQUE file_sha256). Roles: commerce_runtime SELECT/INSERT/DELETE under the app tenant/store GUCs. Non-goals: no buyer PII, no shipment write (record_manual_shipment owns that), no 30-day retention beyond the lazy commit-time cleanup (<=50 rows, I23).';
COMMENT ON COLUMN fulfillment.tracking_import_batches.file_sha256 IS 'sha256 of the exact CSV bytes (32 bytes); the UNIQUE key that makes a re-submit replay instead of re-ship.';
COMMENT ON COLUMN fulfillment.tracking_import_batches.results IS 'per-row results: {row, order_id, carrier_code, carrier_name?, tracking_number, outcome, code?} — outcome apply|unchanged|failed. No recipient, phone, address or body.';
COMMENT ON COLUMN fulfillment.tracking_import_batches.applied IS 'rows that went through fulfillment.record_manual_shipment in this commit.';
COMMENT ON COLUMN fulfillment.tracking_import_batches.unchanged IS 'rows whose shipped head already carried the same carrier/tracking (no write).';
COMMENT ON COLUMN fulfillment.tracking_import_batches.failed IS 'rows skipped with a code (parse, duplicate, not_found, cvs_order, already_shipped, or a record_manual_shipment refusal).';

-- tracking_import_precheck: the lock-free read half of the importer. It returns, per order, whether the
-- order exists in this store, whether it is a CVS order (rule 4: frozen destination pickup kind is any
-- CVS chain, or ANY fulfillment.cvs_shipments row regardless of state — manual 交貨便 and ECPay alike),
-- and the current manual-shipment head (version, status, carrier, tracking) so the importer can emit
-- unchanged / already_shipped / apply without taking the order lock. It never locks and never decides:
-- record_manual_shipment stays the only writer and the only CAS/MD6 authority.
CREATE OR REPLACE FUNCTION fulfillment.tracking_import_precheck(p_hash bytea,p_store uuid,p_orders uuid[])
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 s record;
 v_order_id uuid;
 v_found boolean;
 v_cvs boolean;
 v_head bigint;
 v_head_status text;
 v_carrier_code text;
 v_carrier_name text;
 v_tracking_number text;
 v_tracking_url text;
 arr jsonb := '[]'::jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_orders IS NULL
  OR cardinality(p_orders) > 500
  OR current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'invalid precheck request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'fulfillment:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);

 FOR v_order_id IN SELECT unnest(p_orders) LOOP
  SELECT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=v_order_id)
   INTO v_found;
  IF NOT v_found THEN
   arr := arr || jsonb_build_object('order_id',v_order_id,'found',false);
   CONTINUE;
  END IF;
  -- Rule 4: CVS-destination orders and any order with a cvs_shipments row are never bulk-shipped here
  -- (ECPay goes through its own label flow; manual 交貨便 uses the single PUT).
  v_cvs := EXISTS(SELECT 1 FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store
             AND o.id=v_order_id AND o.snapshot#>>'{destination,pickup,kind}' IN
              ('cvs_711','cvs_familymart','cvs_hilife','cvs_okmart'))
        OR EXISTS(SELECT 1 FROM fulfillment.cvs_shipments c WHERE c.tenant_id=s.tenant_id
             AND c.store_id=p_store AND c.order_id=v_order_id);
  -- Head read is lock-free: a concurrent shipment between precheck and record_manual_shipment surfaces
  -- as version_changed there (or already_shipped/unchanged on a re-run), never as a lost write.
  SELECT h.current_version,v.status,v.carrier_code,v.carrier_name,v.tracking_number,v.tracking_url
   INTO v_head,v_head_status,v_carrier_code,v_carrier_name,v_tracking_number,v_tracking_url
   FROM fulfillment.manual_shipment_heads h
   JOIN fulfillment.manual_shipment_versions v
    ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.order_id=h.order_id AND v.version=h.current_version
   WHERE h.tenant_id=s.tenant_id AND h.store_id=p_store AND h.order_id=v_order_id;
  arr := arr || jsonb_build_object('order_id',v_order_id,'found',true,'cvs',v_cvs,
   'head_version',coalesce(v_head,0),'head_status',v_head_status,
   'carrier_code',v_carrier_code,'carrier_name',v_carrier_name,
   'tracking_number',v_tracking_number,'tracking_url',v_tracking_url);
 END LOOP;
 RETURN arr;
END $$;
ALTER FUNCTION fulfillment.tracking_import_precheck(bytea,uuid,uuid[]) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.tracking_import_precheck(bytea,uuid,uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.tracking_import_precheck(bytea,uuid,uuid[]) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.tracking_import_precheck(bytea,uuid,uuid[]) IS
 'internal/merchanttools (tracking_import.go): lock-free read for the bulk tracking import — per order {found, cvs, head_version, head_status, carrier_code, carrier_name, tracking_number, tracking_url}. Owner commerce_checkout_writer, EXECUTE commerce_runtime, fulfillment:write. Never locks or writes; record_manual_shipment is the only authority.';
