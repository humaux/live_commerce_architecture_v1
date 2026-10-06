-- 0156_order_history_import.sql (unit W5-03B, M22 #7): merchant historical-order CSV import, a READ-ONLY ARCHIVE.
-- Purpose: table customers.historical_orders (display facts of an old SHOPLINE order attached to an IMPORTED customer), the definer
--   migrationimport.import_orders the Go importer calls, the reader customers.read_historical_orders, batch kind 'orders' in
--   migrationimport.record_batch (audit action customers.orders_imported), erasure coverage (erase_import_profile also deletes the
--   owner's historical orders) and the merchant privacy export (export_import_profile also returns them).
-- Depends on: 0152 (migrationimport schema, external_ids / erased_external_ids / store_salts, customers.import_profiles,
--   record_batch, erase_import_profile, export_import_profile, privacy_audit_insert with customers.imported),
--   0139 (tn_authority / tn_fence / tn_owner_visible), 0078 (customers schema, commerce_privacy_writer, apply_erasure).
-- Used by: internal/migrationimport/orders*.go via internal/httpapi/imports.go; internal/customers/historical.go via
--   internal/httpapi/customer_historical.go; internal/customers/privacy.go decodes the new "historical_orders" export key.
-- Invariants: I05 -- a historical order is NEVER a checkout.orders row and NO payment, inventory, finance, report, CAPI or attribution
--   object reads this table (a static test greps every such function body); it carries a display total_minor in TWD only, never a payment
--   method, card, bank account or street address (city only: one of the 22 Taiwan cities / counties); it attaches only to a customer imported earlier (external_ids
--   kind customers); an erased customer's source id fails the row as erased and erasure deletes the owner's historical orders in the
--   same transaction; results / audit rows carry row numbers and codes only (no order id, no customer id, no name).
-- Status: REAL_PG / MOCK evidence only (synthetic data).

DO $$
DECLARE v_role text; v_src text;
BEGIN
 FOREACH v_role IN ARRAY ARRAY['commerce_privacy_writer','commerce_runtime'] LOOP
  IF to_regrole(v_role) IS NULL THEN RAISE EXCEPTION '0156 requires role %',v_role; END IF;
 END LOOP;
 IF to_regclass('migrationimport.batches') IS NULL OR to_regclass('customers.import_profiles') IS NULL
  OR to_regprocedure('migrationimport.record_batch(bytea,uuid,text,bytea,jsonb,integer,integer,integer,integer,jsonb)') IS NULL
  OR to_regprocedure('customers.erase_import_profile(uuid,uuid,uuid)') IS NULL
  OR to_regprocedure('customers.export_import_profile(bytea,uuid,uuid)') IS NULL
  OR to_regprocedure('customers.tn_authority(bytea,uuid,text)') IS NULL
  OR to_regprocedure('customers.tn_owner_visible(uuid,uuid,uuid)') IS NULL THEN
  RAISE EXCEPTION '0156 requires 0152 (customer import) and 0139 (tags/notes)';
 END IF;
 -- Drift guards (P2-1 of the W5-03B review): the three functions replaced below are the 0152 text plus declared hunks, so each is guarded on
 -- the md5 of its CURRENT body being the md5 of the 0152 body (any other patch in between fails loudly and the integrator re-bases the
 -- hunk). The audit policy is guarded on its exact action SET (order-insensitive), so a parallel unit that added an action is never
 -- silently dropped: the integrator must union the lists.
 SELECT prosrc INTO v_src FROM pg_proc WHERE oid='migrationimport.record_batch(bytea,uuid,text,bytea,jsonb,integer,integer,integer,integer,jsonb)'::regprocedure;
 IF md5(v_src)<>'4a98483008173938fbd45d923b5af3e2' THEN RAISE EXCEPTION '0156 record_batch baseline drift (not the 0152 body)'; END IF;
 SELECT prosrc INTO v_src FROM pg_proc WHERE oid='customers.erase_import_profile(uuid,uuid,uuid)'::regprocedure;
 IF md5(v_src)<>'a6676b8aa3830f6ba25272dcd88c0577' THEN RAISE EXCEPTION '0156 erase_import_profile baseline drift (not the 0152 body)'; END IF;
 SELECT prosrc INTO v_src FROM pg_proc WHERE oid='customers.export_import_profile(bytea,uuid,uuid)'::regprocedure;
 IF md5(v_src)<>'2277018e00364d4b1bb8eaf3c0b03e55' THEN RAISE EXCEPTION '0156 export_import_profile baseline drift (not the 0152 body)'; END IF;
 SELECT pg_get_expr(polwithcheck,polrelid) INTO v_src FROM pg_policy WHERE polrelid='ops.audit_events'::regclass AND polname='privacy_audit_insert';
 IF v_src IS NULL OR (SELECT array_agg(m[1] ORDER BY m[1]) FROM regexp_matches(v_src,'(customers\.[a-z_]+)','g') m)
  IS DISTINCT FROM ARRAY['customers.consent_withdrawn','customers.erased','customers.exported','customers.imported','customers.note_added',
   'customers.note_deleted','customers.note_edited','customers.tag_created','customers.tag_deleted','customers.tag_renamed','customers.tagged'] THEN
  RAISE EXCEPTION '0156 privacy_audit_insert baseline drift (action set is not the 0152 set)'; END IF;
END $$;

-- ---------------------------------------------------------------------------------------
-- The archive table. Written ONLY by the commerce_privacy_writer definers below; FORCE RLS, every policy GUC-scoped to the store.
-- external_order_id is the merchant's source order number (unique per store: a re-import of the same order updates it). There is
-- deliberately no column for a payment method, card, bank account, phone, email, name or street address.
-- ---------------------------------------------------------------------------------------
CREATE TABLE customers.historical_orders (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 id uuid NOT NULL DEFAULT gen_random_uuid(),
 owner_id uuid NOT NULL,
 external_order_id text NOT NULL CHECK (char_length(external_order_id) BETWEEN 1 AND 64),
 ordered_at timestamptz NOT NULL CHECK (isfinite(ordered_at)),
 status text NOT NULL CHECK (char_length(status) BETWEEN 1 AND 40 AND status=btrim(status)),
 total_minor bigint NOT NULL CHECK (total_minor BETWEEN 0 AND 1000000000000),
 currency text NOT NULL CHECK (currency='TWD'),
 items_summary text NOT NULL CHECK (char_length(items_summary)<=500),
 -- Backstop of the importer's allowlist (internal/twcity): only Taiwan's 22 cities / counties, canonical spelling (臺), or NULL.
 city text CHECK (city IN ('臺北市','新北市','桃園市','臺中市','臺南市','高雄市','基隆市','新竹市','嘉義市','新竹縣','苗栗縣','彰化縣','南投縣','雲林縣','嘉義縣','屏東縣','宜蘭縣','花蓮縣','臺東縣','澎湖縣','金門縣','連江縣')),
 imported_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (tenant_id,store_id,id),
 UNIQUE (tenant_id,store_id,external_order_id),
 FOREIGN KEY (tenant_id,store_id,owner_id) REFERENCES buyer.owners(tenant_id,store_id,id)
);
CREATE INDEX historical_orders_by_owner ON customers.historical_orders(tenant_id,store_id,owner_id,ordered_at DESC,id DESC);

ALTER TABLE customers.historical_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.historical_orders FORCE ROW LEVEL SECURITY;
REVOKE ALL ON customers.historical_orders FROM PUBLIC;
GRANT SELECT, INSERT, DELETE ON customers.historical_orders TO commerce_privacy_writer;
GRANT UPDATE (ordered_at,status,total_minor,items_summary,city,updated_at) ON customers.historical_orders TO commerce_privacy_writer;

DO $$
DECLARE c text; scope text:='tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid';
BEGIN
 FOREACH c IN ARRAY ARRAY['SELECT','DELETE'] LOOP
  EXECUTE format('CREATE POLICY %I ON customers.historical_orders FOR %s TO commerce_privacy_writer USING (%s)','hist_orders_'||lower(c),c,scope);
 END LOOP;
 EXECUTE format('CREATE POLICY hist_orders_insert ON customers.historical_orders FOR INSERT TO commerce_privacy_writer WITH CHECK (%s)',scope);
 EXECUTE format('CREATE POLICY hist_orders_update ON customers.historical_orders FOR UPDATE TO commerce_privacy_writer USING (%s) WITH CHECK (%s)',scope,scope);
END $$;

-- Audit: the privacy writer may also insert customers.orders_imported (action only). The policy is replaced, not added to: the ACL pin
-- requires every INSERT policy of the role to carry the full action list (0139 / 0152 pattern).
DROP POLICY privacy_audit_insert ON ops.audit_events;
CREATE POLICY privacy_audit_insert ON ops.audit_events FOR INSERT TO commerce_privacy_writer
 WITH CHECK (action IN ('customers.consent_withdrawn','customers.exported','customers.erased',
   'customers.tag_created','customers.tag_renamed','customers.tag_deleted','customers.tagged',
   'customers.note_added','customers.note_edited','customers.note_deleted','customers.imported','customers.orders_imported')
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
COMMENT ON POLICY privacy_audit_insert ON ops.audit_events IS 'commerce_privacy_writer may insert only customers.consent_withdrawn, customers.exported, customers.erased, the seven W6-01B tag/note actions, (W5-02B) customers.imported and (W5-03B) customers.orders_imported, scoped to the GUCs verified from the merchant authentication result. Never a note body, name, phone, email or order id.';

-- ---------------------------------------------------------------------------------------
-- import_orders: upserts up to 500 normalised orders [{row, order_id, customer_id, ordered_at, status, total_minor, items, city?}] in the
-- caller's transaction and returns [{row, outcome, code?}] with outcome created | updated | failed. Failure codes:
--   erased                  the customer source id carries an erasure tombstone (migration-import-v1 section 5);
--   customer_not_imported   no active imported customer has that source id (an order never creates a customer);
--   order_owner_conflict    the order number already belongs to another customer of this store (never re-assigned silently);
--   order_limit             the customer already holds 2000 historical orders.
-- The owner row is locked FOR UPDATE with its active test in the WHERE (the import_customers rule): a concurrent erasure can never be
-- written to after it committed, and the tombstone check runs in a NEW statement so a commit racing an erasure reports erased.
-- A per-store advisory lock serialises order imports of one store (the limit count and the upsert see each other).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION migrationimport.import_orders(p_hash bytea,p_store uuid,p_rows jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; r jsonb; v_owner uuid; v_out jsonb:='[]'::jsonb; v_ext text; v_cust text; v_prev uuid; v_prev_owner uuid; v_outcome text;
BEGIN
 IF p_rows IS NULL OR jsonb_typeof(p_rows)<>'array' OR jsonb_array_length(p_rows)>500
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid import request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');
 PERFORM pg_advisory_xact_lock(hashtextextended('migrationimport.orders:'||a.tenant_id::text||':'||p_store::text,0));
 FOR r IN SELECT x FROM jsonb_array_elements(p_rows) x LOOP
  v_ext:=r->>'order_id'; v_cust:=r->>'customer_id'; v_owner:=NULL;
  SELECT x.internal_id INTO v_owner FROM migrationimport.external_ids x
   JOIN buyer.owners o ON o.tenant_id=x.tenant_id AND o.store_id=x.store_id AND o.id=x.internal_id AND o.active
   WHERE x.tenant_id=a.tenant_id AND x.store_id=p_store AND x.kind='customers' AND x.external_id=v_cust
   FOR UPDATE OF o;
  IF v_owner IS NULL THEN
   IF EXISTS(SELECT 1 FROM migrationimport.erased_external_ids t JOIN migrationimport.store_salts sa
     ON sa.tenant_id=t.tenant_id AND sa.store_id=t.store_id
    WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store AND t.kind='customers'
     AND t.id_digest=sha256(sa.salt||convert_to(v_cust,'UTF8'))) THEN
    v_out:=v_out||jsonb_build_object('row',(r->>'row')::integer,'outcome','failed','code','erased');
   ELSE
    v_out:=v_out||jsonb_build_object('row',(r->>'row')::integer,'outcome','failed','code','customer_not_imported');
   END IF;
   CONTINUE;
  END IF;
  SELECT h.id,h.owner_id INTO v_prev,v_prev_owner FROM customers.historical_orders h
   WHERE h.tenant_id=a.tenant_id AND h.store_id=p_store AND h.external_order_id=v_ext;
  IF v_prev IS NOT NULL AND v_prev_owner<>v_owner THEN
   v_out:=v_out||jsonb_build_object('row',(r->>'row')::integer,'outcome','failed','code','order_owner_conflict');
   CONTINUE;
  ELSIF v_prev IS NOT NULL THEN
   UPDATE customers.historical_orders h SET ordered_at=(r->>'ordered_at')::timestamptz,status=r->>'status',
    total_minor=(r->>'total_minor')::bigint,items_summary=r->>'items',city=nullif(r->>'city',''),updated_at=clock_timestamp()
    WHERE h.tenant_id=a.tenant_id AND h.store_id=p_store AND h.id=v_prev;
   v_outcome:='updated';
  ELSIF (SELECT count(*) FROM customers.historical_orders h WHERE h.tenant_id=a.tenant_id AND h.store_id=p_store AND h.owner_id=v_owner)>=2000 THEN
   v_out:=v_out||jsonb_build_object('row',(r->>'row')::integer,'outcome','failed','code','order_limit');
   CONTINUE;
  ELSE
   INSERT INTO customers.historical_orders(tenant_id,store_id,owner_id,external_order_id,ordered_at,status,total_minor,currency,items_summary,city)
    VALUES(a.tenant_id,p_store,v_owner,v_ext,(r->>'ordered_at')::timestamptz,r->>'status',(r->>'total_minor')::bigint,'TWD',r->>'items',nullif(r->>'city',''));
   v_outcome:='created';
  END IF;
  v_out:=v_out||jsonb_build_object('row',(r->>'row')::integer,'outcome',v_outcome);
  v_prev:=NULL; v_prev_owner:=NULL;
 END LOOP;
 PERFORM customers.tn_fence(p_hash,p_store,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v_out;
END $$;

-- read_historical_orders: one customer's archive newest first, keyset (ordered_at, id) DESC. customers:read; the customer must be visible
-- to the merchant list (an erased, unknown or other-store id is a plain PT404). Returns {total, items:[{order_id, ordered_at, status,
-- total_minor, currency, items_summary, city}]}; p_limit is the page size + 1 (the Go layer trims the look-ahead row).
CREATE FUNCTION customers.read_historical_orders(p_hash bytea,p_store uuid,p_owner uuid,p_limit integer,p_after_ts timestamptz,p_after_id uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_items jsonb; v_total integer;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:read');
 IF p_owner IS NULL OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 101 OR (p_after_ts IS NULL)<>(p_after_id IS NULL)
  OR (p_after_ts IS NOT NULL AND NOT isfinite(p_after_ts)) THEN
  RAISE EXCEPTION 'invalid historical orders read' USING ERRCODE='PT400'; END IF;
 IF NOT customers.tn_owner_visible(a.tenant_id,p_store,p_owner) THEN
  RAISE EXCEPTION 'customer not found' USING ERRCODE='PT404'; END IF;
 SELECT count(*) INTO v_total FROM customers.historical_orders h WHERE h.tenant_id=a.tenant_id AND h.store_id=p_store AND h.owner_id=p_owner;
 SELECT coalesce(jsonb_agg(jsonb_build_object('order_id',x.external_order_id,
   'ordered_at',to_char(x.ordered_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'status',x.status,'total_minor',x.total_minor,
   'currency',x.currency,'items_summary',x.items_summary,'city',x.city,'id',x.id) ORDER BY x.ordered_at DESC,x.id DESC),'[]'::jsonb)
  INTO v_items
  FROM (SELECT h.* FROM customers.historical_orders h WHERE h.tenant_id=a.tenant_id AND h.store_id=p_store AND h.owner_id=p_owner
   AND (p_after_id IS NULL OR (h.ordered_at,h.id)<(p_after_ts,p_after_id))
   ORDER BY h.ordered_at DESC,h.id DESC LIMIT p_limit) x;
 PERFORM customers.tn_fence(p_hash,p_store,'customers:read',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('total',v_total,'items',v_items);
END $$;

-- record_batch: the 0152 body in full with two changes (W5-03B): p_kind may also be 'orders', and the audit action follows the kind
-- (customers.imported | customers.orders_imported). Same signature, owner and ACL.
CREATE OR REPLACE FUNCTION migrationimport.record_batch(p_hash bytea,p_store uuid,p_kind text,p_sha bytea,p_mapping jsonb,
 p_total integer,p_applied integer,p_updated integer,p_failed integer,p_results jsonb)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_id uuid;
BEGIN
 IF p_kind IS NULL OR p_kind NOT IN ('customers','orders') OR p_sha IS NULL OR octet_length(p_sha)<>32 OR p_mapping IS NULL OR p_results IS NULL THEN
  RAISE EXCEPTION 'invalid import batch' USING ERRCODE='PT400'; END IF;
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');
 DELETE FROM migrationimport.batches b WHERE b.tenant_id=a.tenant_id AND b.store_id=p_store
  AND b.id IN (SELECT o.id FROM migrationimport.batches o WHERE o.tenant_id=a.tenant_id AND o.store_id=p_store
   AND o.created_at<clock_timestamp()-interval '90 days' LIMIT 50);
 INSERT INTO migrationimport.batches(tenant_id,store_id,kind,file_sha256,mapping,rows_total,applied,updated,failed,results,principal_id)
  VALUES(a.tenant_id,p_store,p_kind,p_sha,p_mapping,p_total,p_applied,p_updated,p_failed,p_results,a.principal_id) RETURNING id INTO v_id;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
  VALUES(a.tenant_id,p_store,a.principal_id,CASE p_kind WHEN 'orders' THEN 'customers.orders_imported' ELSE 'customers.imported' END);
 PERFORM customers.tn_fence(p_hash,p_store,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v_id;
END $$;

-- erase_import_profile: the 0152 body in full plus the last DELETE (W5-03B): the owner's historical orders go in the same transaction as
-- the profile. Batch results of kind orders hold row numbers and codes only, so there is nothing to scrub there.
CREATE OR REPLACE FUNCTION customers.erase_import_profile(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE x record;
BEGIN
 IF EXISTS(SELECT 1 FROM migrationimport.external_ids e WHERE e.tenant_id=p_tenant AND e.store_id=p_store AND e.kind='customers' AND e.internal_id=p_owner) THEN
  INSERT INTO migrationimport.store_salts(tenant_id,store_id,salt) VALUES(p_tenant,p_store,uuid_send(gen_random_uuid())||uuid_send(gen_random_uuid()))
   ON CONFLICT DO NOTHING;
  INSERT INTO migrationimport.erased_external_ids(tenant_id,store_id,kind,id_digest)
   SELECT p_tenant,p_store,'customers',sha256(sa.salt||convert_to(e.external_id,'UTF8'))
   FROM migrationimport.external_ids e JOIN migrationimport.store_salts sa ON sa.tenant_id=e.tenant_id AND sa.store_id=e.store_id
   WHERE e.tenant_id=p_tenant AND e.store_id=p_store AND e.kind='customers' AND e.internal_id=p_owner
   ON CONFLICT DO NOTHING;
 END IF;
 FOR x IN SELECT e.external_id FROM migrationimport.external_ids e
   WHERE e.tenant_id=p_tenant AND e.store_id=p_store AND e.kind='customers' AND e.internal_id=p_owner LOOP
  UPDATE migrationimport.batches b SET results=(
    SELECT coalesce(jsonb_agg(CASE WHEN t.e->>'external_id'=x.external_id THEN t.e-'external_id' ELSE t.e END ORDER BY t.ord),'[]'::jsonb)
    FROM jsonb_array_elements(b.results) WITH ORDINALITY t(e,ord))
   WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.kind='customers'
    AND b.results @> jsonb_build_array(jsonb_build_object('external_id',x.external_id));
 END LOOP;
 DELETE FROM migrationimport.external_ids e WHERE e.tenant_id=p_tenant AND e.store_id=p_store AND e.kind='customers' AND e.internal_id=p_owner;
 DELETE FROM customers.import_profiles p WHERE p.tenant_id=p_tenant AND p.store_id=p_store AND p.owner_id=p_owner;
 DELETE FROM customers.historical_orders h WHERE h.tenant_id=p_tenant AND h.store_id=p_store AND h.owner_id=p_owner;
END $$;

-- export_import_profile: the 0152 body in full plus 'historical_orders' (W5-03B): the customer's archive, newest first, same item shape
-- as the reader, newest 100 only, with historical_orders_total. The merchant already holds this data (it imported it); the export route needs customers:privacy, so nothing widens.
CREATE OR REPLACE FUNCTION customers.export_import_profile(p_hash bytea,p_store uuid,p_customer uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v jsonb;
BEGIN
 IF p_customer IS NULL THEN RAISE EXCEPTION 'invalid export request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');
 SELECT jsonb_build_object('display_name',p.display_name,'phone',p.phone_e164,'email',p.email,'source',p.source,
   'imported_at',to_char(p.imported_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'updated_at',to_char(p.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'external_ids',coalesce((SELECT jsonb_agg(e.external_id ORDER BY e.external_id) FROM migrationimport.external_ids e
     WHERE e.tenant_id=p.tenant_id AND e.store_id=p.store_id AND e.kind='customers' AND e.internal_id=p.owner_id),'[]'::jsonb),
   -- Bounded (P2-4 of the review): the newest 100 archive rows plus the total, so 2000 rows of 500 characters can never push the export
   -- over its 1 MiB cap. The merchant still sees every row through the reader; a data-subject request for the rest is answered by the merchant.
   'historical_orders',coalesce((SELECT jsonb_agg(jsonb_build_object('order_id',h.external_order_id,
     'ordered_at',to_char(h.ordered_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'status',h.status,'total_minor',h.total_minor,
     'currency',h.currency,'items_summary',h.items_summary,'city',h.city) ORDER BY h.ordered_at DESC,h.id DESC)
     FROM (SELECT x.* FROM customers.historical_orders x WHERE x.tenant_id=p.tenant_id AND x.store_id=p.store_id AND x.owner_id=p.owner_id
       ORDER BY x.ordered_at DESC,x.id DESC LIMIT 100) h),'[]'::jsonb),
   'historical_orders_total',(SELECT count(*) FROM customers.historical_orders t WHERE t.tenant_id=p.tenant_id AND t.store_id=p.store_id AND t.owner_id=p.owner_id))
  INTO v FROM customers.import_profiles p WHERE p.tenant_id=a.tenant_id AND p.store_id=p_store AND p.owner_id=p_customer;
 PERFORM customers.tn_fence(p_hash,p_store,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v;
END $$;

DO $$
DECLARE f text;
BEGIN
 FOREACH f IN ARRAY ARRAY['migrationimport.import_orders(bytea,uuid,jsonb)',
  'customers.read_historical_orders(bytea,uuid,uuid,integer,timestamptz,uuid)'] LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_privacy_writer',f);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
  EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime',f);
 END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------
-- Comments (every object names its owning package).
-- ---------------------------------------------------------------------------------------
COMMENT ON TABLE customers.historical_orders IS 'internal/migrationimport + internal/customers (W5-03B): READ-ONLY ARCHIVE of an old SHOPLINE order attached to an imported customer: source order number, date, status, display total (TWD), item summary, city. Never a checkout.orders row and never read by payments, inventory, finance, reports, CAPI or attribution (I05). No payment detail, no street address. Deleted by erasure (customers.erase_import_profile). Written only by migrationimport.import_orders. FORCE RLS, definers only.';
COMMENT ON COLUMN customers.historical_orders.external_order_id IS 'internal/migrationimport: the merchant source-system order number (1..64 chars); UNIQUE per store so a re-import updates the row.';
COMMENT ON COLUMN customers.historical_orders.owner_id IS 'internal/customers: the imported customer (buyer.owners row with an import profile) this order is attached to; never changed by a re-import.';
COMMENT ON COLUMN customers.historical_orders.ordered_at IS 'internal/migrationimport: order time from the file (Asia/Taipei when the cell had no zone), stored as an instant.';
COMMENT ON COLUMN customers.historical_orders.status IS 'internal/migrationimport: the source status text as exported (<= 40 chars, trimmed); display only, never interpreted.';
COMMENT ON COLUMN customers.historical_orders.total_minor IS 'internal/migrationimport: display total in TWD minor units (whole NT$ x 100); NOT revenue: no finance, report or attribution object may sum it (I05).';
COMMENT ON COLUMN customers.historical_orders.currency IS 'internal/migrationimport: always TWD in v1.';
COMMENT ON COLUMN customers.historical_orders.items_summary IS 'internal/migrationimport: item names x quantities joined into one line (<= 500 chars); no SKU link.';
COMMENT ON COLUMN customers.historical_orders.city IS 'internal/migrationimport: one of Taiwan's 22 cities / counties (canonical 臺 spelling) or NULL; CHECK-enforced, so a street, name or email can never be stored; the full address is never imported (OH-OPEN-1).';
COMMENT ON COLUMN customers.historical_orders.imported_at IS 'internal/migrationimport: first import time of this order.';
COMMENT ON COLUMN customers.historical_orders.updated_at IS 'internal/migrationimport: last time a re-import changed this order.';
COMMENT ON COLUMN customers.historical_orders.tenant_id IS 'internal/customers: tenant scope (FORCE RLS GUC).';
COMMENT ON COLUMN customers.historical_orders.store_id IS 'internal/customers: store scope (FORCE RLS GUC).';
COMMENT ON COLUMN customers.historical_orders.id IS 'internal/customers: our id of the archive row; the keyset tie-breaker of the reader.';
COMMENT ON FUNCTION migrationimport.import_orders(bytea,uuid,jsonb) IS 'internal/migrationimport (customers:privacy): upserts <=500 validated orders [{row,order_id,customer_id,ordered_at,status,total_minor,items,city}] into customers.historical_orders for already-imported customers and returns [{row,outcome created|updated|failed,code?}] (codes erased, customer_not_imported, order_owner_conflict, order_limit). Per-store advisory lock; never writes checkout.orders, payments or inventory.';
COMMENT ON FUNCTION customers.read_historical_orders(bytea,uuid,uuid,integer,timestamptz,uuid) IS 'internal/customers (customers:read): {total, items} of one customer''s historical orders newest first (keyset ordered_at,id; limit 1..101); PT404 for an unknown, erased or other-store customer.';
COMMENT ON FUNCTION migrationimport.record_batch(bytea,uuid,text,bytea,jsonb,integer,integer,integer,integer,jsonb) IS 'internal/migrationimport (customers:privacy): records the committed import batch of kind customers or orders (UNIQUE per file hash), prunes this store''s batches older than 90 days (<=50) and audits customers.imported / customers.orders_imported; returns the batch id.';
COMMENT ON FUNCTION customers.erase_import_profile(uuid,uuid,uuid) IS 'internal/customers erasure-only (called by apply_erasure, EXECUTE nobody): writes the salted tombstone digest of the owner''s customer external ids, then deletes the import profile, the external id and (W5-03B) the owner''s historical orders, and scrubs that external id from retained batch results.';
COMMENT ON FUNCTION customers.export_import_profile(bytea,uuid,uuid) IS 'internal/customers (customers:privacy): the imported profile (name, phone, email, source, timestamps, external ids) and (W5-03B) the newest 100 historical orders plus their total of one customer for the merchant privacy export, or NULL when it has none.';
