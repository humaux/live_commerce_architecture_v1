-- 0152_customer_import.sql (unit W5-02B, M22 #6; the minimal W5-01B import framework is folded in): merchant customer CSV import.
-- Purpose: schema migrationimport (batches + external ids, shared by the customer and the later order import), table
--   customers.import_profiles (name / phone / email of an imported customer), the three definers the Go importer calls
--   (import_customers, record_batch, read_batch_results), the audit action customers.imported, the "imported" projection and list
--   predicate of identity.read_merchant_customers, tag/note visibility of imported customers, and erasure coverage (apply_erasure
--   also deletes the profile and the external id, leaves a SALTED digest tombstone so a stale export cannot resurrect the person, and
--   scrubs the external id from retained batch results), and the merchant privacy-export reader customers.export_import_profile.
-- Depends on: 0006 (buyer.owners), 0078 (customers schema, commerce_privacy_writer, apply_erasure, privacy_audit_insert),
--   0139 (tn_authority / tn_fence / tn_lock_owner / tn_owner_visible / tn_owners_with_tag, read_merchant_customers 8-arg body,
--   privacy_audit_insert with the tag/note actions).
-- Used by: internal/migrationimport/{customers,batch}.go via internal/httpapi/imports.go; internal/customers/read.go decodes the
--   new "imported" key.
-- Invariants: imported consent is ALWAYS unknown (no definer here touches customers.consent_events); an imported owner is its own
--   buyer.owners row and is never merged with a later buyer (no cross-tenant, no cross-store match); results/audit carry no name,
--   phone or email; no login role holds a direct grant on the new tables (definers only); erasure removes every imported datum and keeps
--   only a per-store salted digest of the erased source id (no name, phone, email or owner link) so the same id is refused later.
-- Status: REAL_PG / MOCK evidence only (synthetic data).

DO $$
DECLARE v_role text; v_src text;
BEGIN
 FOREACH v_role IN ARRAY ARRAY['commerce_privacy_writer','commerce_auth','commerce_runtime'] LOOP
  IF to_regrole(v_role) IS NULL THEN RAISE EXCEPTION '0152 requires role %',v_role; END IF;
 END LOOP;
 IF to_regclass('buyer.owners') IS NULL OR to_regclass('customers.notes') IS NULL
  OR to_regprocedure('identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid)') IS NULL
  OR to_regprocedure('customers.apply_erasure(uuid,uuid,uuid)') IS NULL
  OR to_regprocedure('customers.tn_authority(bytea,uuid,text)') IS NULL
  OR to_regprocedure('customers.tn_fence(bytea,uuid,text,uuid,uuid,bigint)') IS NULL
  OR to_regprocedure('customers.tn_lock_owner(uuid,uuid,uuid)') IS NULL
  OR to_regprocedure('customers.tn_owner_visible(uuid,uuid,uuid)') IS NULL THEN
  RAISE EXCEPTION '0152 requires 0078 (customers/privacy) and 0139 (tags/notes)';
 END IF;
 -- Drift guards for the source patches below (0113 / 0139 pattern): fail loudly rather than patch a changed body.
 SELECT prosrc INTO v_src FROM pg_proc WHERE oid='customers.apply_erasure(uuid,uuid,uuid)'::regprocedure;
 IF position('-- CD4: withdraw every granted pair' IN v_src)=0 OR position('erase_import_profile' IN v_src)>0 THEN
  RAISE EXCEPTION '0152 erasure baseline drift'; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid='ops.audit_events'::regclass AND polname='privacy_audit_insert'
    AND pg_get_expr(polwithcheck,polrelid) LIKE '%customers.note_deleted%'
    AND pg_get_expr(polwithcheck,polrelid) NOT LIKE '%customers.imported%') THEN
  RAISE EXCEPTION '0152 privacy_audit_insert baseline drift'; END IF;
END $$;

-- ---------------------------------------------------------------------------------------
-- Schema and tables. Written ONLY by the commerce_privacy_writer definers below; FORCE RLS, every policy GUC-scoped to the store.
-- ---------------------------------------------------------------------------------------
CREATE SCHEMA migrationimport;
REVOKE ALL ON SCHEMA migrationimport FROM PUBLIC;
GRANT USAGE ON SCHEMA migrationimport TO commerce_privacy_writer, commerce_runtime;

-- One committed import file per (store, kind, file bytes): UNIQUE(file_sha256) is the durable idempotency guard. results holds
-- {row, outcome, code?, external_id?} per data row (external_id is the merchant's source-system id, never a name/phone/email).
CREATE TABLE migrationimport.batches (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 id uuid NOT NULL DEFAULT gen_random_uuid(),
 kind text NOT NULL CHECK (kind IN ('customers','orders')),
 file_sha256 bytea NOT NULL CHECK (octet_length(file_sha256)=32),
 mapping jsonb NOT NULL CHECK (jsonb_typeof(mapping)='object' AND octet_length(mapping::text)<=2048),
 rows_total integer NOT NULL CHECK (rows_total BETWEEN 0 AND 5000),
 applied integer NOT NULL CHECK (applied BETWEEN 0 AND 5000),
 updated integer NOT NULL CHECK (updated BETWEEN 0 AND 5000),
 failed integer NOT NULL CHECK (failed BETWEEN 0 AND 5000),
 results jsonb NOT NULL CHECK (jsonb_typeof(results)='array' AND jsonb_array_length(results)<=5000),
 principal_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (tenant_id,store_id,id),
 UNIQUE (tenant_id,store_id,kind,file_sha256),
 CHECK (applied+updated+failed<=rows_total)
);

-- The merchant's source-system id of an imported record -> our internal id (buyer.owners.id for kind customers).
CREATE TABLE migrationimport.external_ids (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('customers','orders')),
 external_id text NOT NULL CHECK (char_length(external_id) BETWEEN 1 AND 64),
 internal_id uuid NOT NULL,
 PRIMARY KEY (tenant_id,store_id,kind,external_id)
);
CREATE INDEX import_external_ids_by_internal ON migrationimport.external_ids(tenant_id,store_id,kind,internal_id);

-- Profile of an imported customer. The owner row is a plain buyer.owners row (no capability session): the customer is
-- merchant-visible through this table, not through an order or a bundle. Consent is never stored here.
CREATE TABLE customers.import_profiles (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 owner_id uuid NOT NULL,
 display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 80 AND display_name=btrim(display_name)),
 phone_e164 text CHECK (phone_e164 ~ '^\+[1-9][0-9]{7,14}$'),
 email text CHECK (char_length(email) BETWEEN 3 AND 254 AND email ~ '^[^@[:space:]]+@[^@[:space:]]+$'), -- lower-casing is the importer's job (Go), not a locale-dependent CHECK
 source text NOT NULL CHECK (source='shopline_csv'),
 imported_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (tenant_id,store_id,owner_id),
 FOREIGN KEY (tenant_id,store_id,owner_id) REFERENCES buyer.owners(tenant_id,store_id,id)
);

-- Erasure tombstone (P1-1): one random 32-byte salt per store (two v4 uuids, the 0125 precedent: pgcrypto is not installed) and the
-- salted sha256 digests of erased source ids. The salt lives apart from the digests so a leak of the digest table alone cannot be
-- enumerated; deleting the salt with the store crypto-shreds every tombstone. No UPDATE or DELETE exists for any role.
CREATE TABLE migrationimport.store_salts (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 salt bytea NOT NULL CHECK (octet_length(salt)=32),
 PRIMARY KEY (tenant_id,store_id)
);
CREATE TABLE migrationimport.erased_external_ids (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 kind text NOT NULL CHECK (kind IN ('customers','orders')),
 id_digest bytea NOT NULL CHECK (octet_length(id_digest)=32),
 erased_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (tenant_id,store_id,kind,id_digest)
);

ALTER TABLE migrationimport.batches ENABLE ROW LEVEL SECURITY;
ALTER TABLE migrationimport.batches FORCE ROW LEVEL SECURITY;
ALTER TABLE migrationimport.external_ids ENABLE ROW LEVEL SECURITY;
ALTER TABLE migrationimport.external_ids FORCE ROW LEVEL SECURITY;
ALTER TABLE customers.import_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.import_profiles FORCE ROW LEVEL SECURITY;
ALTER TABLE migrationimport.store_salts ENABLE ROW LEVEL SECURITY;
ALTER TABLE migrationimport.store_salts FORCE ROW LEVEL SECURITY;
ALTER TABLE migrationimport.erased_external_ids ENABLE ROW LEVEL SECURITY;
ALTER TABLE migrationimport.erased_external_ids FORCE ROW LEVEL SECURITY;
REVOKE ALL ON migrationimport.batches, migrationimport.external_ids, customers.import_profiles,
 migrationimport.store_salts, migrationimport.erased_external_ids FROM PUBLIC;
GRANT SELECT, INSERT ON migrationimport.store_salts, migrationimport.erased_external_ids TO commerce_privacy_writer;

GRANT SELECT, INSERT, DELETE ON migrationimport.batches TO commerce_privacy_writer;
GRANT UPDATE (results) ON migrationimport.batches TO commerce_privacy_writer;          -- erasure scrubs one external id
GRANT SELECT, INSERT, DELETE ON migrationimport.external_ids TO commerce_privacy_writer;
GRANT SELECT, INSERT, DELETE ON customers.import_profiles TO commerce_privacy_writer;
GRANT UPDATE (display_name,phone_e164,email,updated_at) ON customers.import_profiles TO commerce_privacy_writer;
-- Imported customers are real buyer.owners rows: only the two scope columns are insertable (id, active, created_at default).
GRANT INSERT (tenant_id,store_id) ON buyer.owners TO commerce_privacy_writer;
-- identity.read_merchant_customers (owner commerce_auth) reads the profile columns it projects; it filters tenant/store itself.
GRANT SELECT (tenant_id,store_id,owner_id,display_name,phone_e164,imported_at) ON customers.import_profiles TO commerce_auth;

DO $$
DECLARE t text; c text; scope text:='tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid';
BEGIN
 FOREACH t IN ARRAY ARRAY['migrationimport.batches','migrationimport.external_ids','customers.import_profiles'] LOOP
  FOREACH c IN ARRAY ARRAY['SELECT','DELETE'] LOOP
   EXECUTE format('CREATE POLICY %I ON %s FOR %s TO commerce_privacy_writer USING (%s)',
    'imp_'||split_part(t,'.',2)||'_'||lower(c),t,c,scope);
  END LOOP;
  EXECUTE format('CREATE POLICY %I ON %s FOR INSERT TO commerce_privacy_writer WITH CHECK (%s)',
   'imp_'||split_part(t,'.',2)||'_insert',t,scope);
 END LOOP;
 EXECUTE format('CREATE POLICY imp_batches_update ON migrationimport.batches FOR UPDATE TO commerce_privacy_writer USING (%s) WITH CHECK (%s)',scope,scope);
 EXECUTE format('CREATE POLICY imp_import_profiles_update ON customers.import_profiles FOR UPDATE TO commerce_privacy_writer USING (%s) WITH CHECK (%s)',scope,scope);
 EXECUTE format('CREATE POLICY imp_owner_insert ON buyer.owners FOR INSERT TO commerce_privacy_writer WITH CHECK (%s)',scope);
END $$;
DO $$
DECLARE t text; scope text:='tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid';
BEGIN
 FOREACH t IN ARRAY ARRAY['store_salts','erased_external_ids'] LOOP
  EXECUTE format('CREATE POLICY %I ON migrationimport.%I FOR SELECT TO commerce_privacy_writer USING (%s)','imp_'||t||'_select',t,scope);
  EXECUTE format('CREATE POLICY %I ON migrationimport.%I FOR INSERT TO commerce_privacy_writer WITH CHECK (%s)','imp_'||t||'_insert',t,scope);
 END LOOP;
 -- read_merchant_customers verifies app.tenant_id / app.store_id against resolve_access before it reads, so the scope holds there.
 EXECUTE format('CREATE POLICY auth_import_profile_read ON customers.import_profiles FOR SELECT TO commerce_auth USING (%s)',scope);
END $$;

-- Audit: the privacy writer may also insert customers.imported (counts only, never a row). The policy is replaced, not added to:
-- the ACL pin requires every INSERT policy of the role to carry the full action list (0139 pattern).
DROP POLICY privacy_audit_insert ON ops.audit_events;
CREATE POLICY privacy_audit_insert ON ops.audit_events FOR INSERT TO commerce_privacy_writer
 WITH CHECK (action IN ('customers.consent_withdrawn','customers.exported','customers.erased',
   'customers.tag_created','customers.tag_renamed','customers.tag_deleted','customers.tagged',
   'customers.note_added','customers.note_edited','customers.note_deleted','customers.imported')
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
COMMENT ON POLICY privacy_audit_insert ON ops.audit_events IS 'commerce_privacy_writer may insert only customers.consent_withdrawn, customers.exported, customers.erased, the seven W6-01B tag/note actions and (W5-02B) customers.imported, scoped to the GUCs verified from the merchant authentication result. Never a note body, name, phone or email.';

-- ---------------------------------------------------------------------------------------
-- Definers (owner commerce_privacy_writer, SECURITY DEFINER, search_path=pg_catalog, customers:privacy, EXECUTE commerce_runtime).
-- ---------------------------------------------------------------------------------------
-- import_customers: upserts up to 500 normalised rows [{row, external_id, name, phone?, email?}] in the caller's transaction and
-- returns [{row, outcome, code?}] with outcome created | updated | failed (code erased: the source id carries an erasure tombstone). The Go importer validates every cell first (the table CHECKs are the
-- backstop, a violation aborts the whole import). A per-store transaction advisory lock serialises concurrent imports; the existing
-- owner row is locked FOR UPDATE with its active test in the WHERE, so a concurrent erasure (which deactivates the owner and
-- deletes the external id and writes the tombstone in one transaction) can never be updated back to life: after its commit the lookup
-- finds nothing, and the create branch re-checks the tombstone in a NEW statement (READ COMMITTED sees the committed erasure), so a
-- commit racing an erasure refuses the row as erased instead of resurrecting the person. Never touches customers.consent_events.
CREATE FUNCTION migrationimport.import_customers(p_hash bytea,p_store uuid,p_rows jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; r jsonb; v_owner uuid; v_out jsonb:='[]'::jsonb; v_outcome text; v_ext text; v_phone text; v_email text;
BEGIN
 IF p_rows IS NULL OR jsonb_typeof(p_rows)<>'array' OR jsonb_array_length(p_rows)>500
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid import request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');
 PERFORM pg_advisory_xact_lock(hashtextextended('migrationimport.customers:'||a.tenant_id::text||':'||p_store::text,0));
 FOR r IN SELECT x FROM jsonb_array_elements(p_rows) x LOOP
  v_ext:=r->>'external_id'; v_phone:=nullif(r->>'phone',''); v_email:=nullif(r->>'email','');
  v_owner:=NULL;
  SELECT x.internal_id INTO v_owner FROM migrationimport.external_ids x
   JOIN buyer.owners o ON o.tenant_id=x.tenant_id AND o.store_id=x.store_id AND o.id=x.internal_id AND o.active
   WHERE x.tenant_id=a.tenant_id AND x.store_id=p_store AND x.kind='customers' AND x.external_id=v_ext
   FOR UPDATE OF o;
  IF v_owner IS NOT NULL THEN
   -- A column the file does not map (key absent) keeps its stored value; a mapped column with an empty cell clears it.
   UPDATE customers.import_profiles p SET display_name=r->>'name',
    phone_e164=CASE WHEN r ? 'phone' THEN v_phone ELSE p.phone_e164 END,
    email=CASE WHEN r ? 'email' THEN v_email ELSE p.email END,updated_at=clock_timestamp()
    WHERE p.tenant_id=a.tenant_id AND p.store_id=p_store AND p.owner_id=v_owner;
   v_outcome:='updated';
  ELSIF EXISTS(SELECT 1 FROM migrationimport.erased_external_ids t JOIN migrationimport.store_salts sa
    ON sa.tenant_id=t.tenant_id AND sa.store_id=t.store_id
   WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store AND t.kind='customers'
    AND t.id_digest=sha256(sa.salt||convert_to(v_ext,'UTF8'))) THEN
   v_out:=v_out||jsonb_build_object('row',(r->>'row')::integer,'outcome','failed','code','erased');
   CONTINUE;
  ELSE
   INSERT INTO buyer.owners(tenant_id,store_id) VALUES(a.tenant_id,p_store) RETURNING id INTO v_owner;
   INSERT INTO customers.import_profiles(tenant_id,store_id,owner_id,display_name,phone_e164,email,source)
    VALUES(a.tenant_id,p_store,v_owner,r->>'name',v_phone,v_email,'shopline_csv');
   INSERT INTO migrationimport.external_ids(tenant_id,store_id,kind,external_id,internal_id)
    VALUES(a.tenant_id,p_store,'customers',v_ext,v_owner);
   v_outcome:='created';
  END IF;
  v_out:=v_out||jsonb_build_object('row',(r->>'row')::integer,'outcome',v_outcome);
 END LOOP;
 PERFORM customers.tn_fence(p_hash,p_store,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v_out;
END $$;

-- record_batch: the commit's one batch row (UNIQUE per file hash), a lazy 90-day prune of this store's batches (<=50 per call) and
-- the audit row customers.imported (action only: counts live in the batch, rows hold no PII). Returns the batch id.
CREATE FUNCTION migrationimport.record_batch(p_hash bytea,p_store uuid,p_kind text,p_sha bytea,p_mapping jsonb,
 p_total integer,p_applied integer,p_updated integer,p_failed integer,p_results jsonb)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_id uuid;
BEGIN
 IF p_kind IS DISTINCT FROM 'customers' OR p_sha IS NULL OR octet_length(p_sha)<>32 OR p_mapping IS NULL OR p_results IS NULL THEN
  RAISE EXCEPTION 'invalid import batch' USING ERRCODE='PT400'; END IF;
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');
 DELETE FROM migrationimport.batches b WHERE b.tenant_id=a.tenant_id AND b.store_id=p_store
  AND b.id IN (SELECT o.id FROM migrationimport.batches o WHERE o.tenant_id=a.tenant_id AND o.store_id=p_store
   AND o.created_at<clock_timestamp()-interval '90 days' LIMIT 50);
 INSERT INTO migrationimport.batches(tenant_id,store_id,kind,file_sha256,mapping,rows_total,applied,updated,failed,results,principal_id)
  VALUES(a.tenant_id,p_store,p_kind,p_sha,p_mapping,p_total,p_applied,p_updated,p_failed,p_results,a.principal_id) RETURNING id INTO v_id;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(a.tenant_id,p_store,a.principal_id,'customers.imported');
 PERFORM customers.tn_fence(p_hash,p_store,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v_id;
END $$;

-- read_batch_results: the per-row results of one committed batch of this store (PT404 for any other id), for results.csv.
CREATE FUNCTION migrationimport.read_batch_results(p_hash bytea,p_store uuid,p_batch uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_results jsonb;
BEGIN
 IF p_batch IS NULL THEN RAISE EXCEPTION 'invalid import batch' USING ERRCODE='PT400'; END IF;
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');
 SELECT b.results INTO v_results FROM migrationimport.batches b
  WHERE b.tenant_id=a.tenant_id AND b.store_id=p_store AND b.id=p_batch;
 IF NOT FOUND THEN RAISE EXCEPTION 'import batch not found' USING ERRCODE='PT404'; END IF;
 PERFORM customers.tn_fence(p_hash,p_store,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v_results;
END $$;

-- erase_import_profile (erasure-only, called by apply_erasure): writes the salted tombstone digest of each customer external id of the
-- owner FIRST, then removes the profile and the external id and scrubs that id out of the retained batch results (the counts and row
-- numbers stay; the id link does not). The tombstone holds only sha256(store salt || id): no name, phone, email or owner link.
-- Known limits (migration-import-v1 section 5): the same person under a NEW source id, or a buyer who erased themselves (no external
-- id exists), is not caught; that needs the verified-contact contract (B24).
CREATE FUNCTION customers.erase_import_profile(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS void
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
END $$;

-- export_import_profile: the merchant privacy export's reader (P1-2, modelled on export_tags_notes of 0139). customers:privacy; returns the
-- profile of one imported customer {display_name, phone, email, source, imported_at, updated_at, external_ids[]} or NULL when the customer
-- has none. The merchant already holds this data (it imported it); the export route needs customers:privacy too, so nothing widens.
CREATE FUNCTION customers.export_import_profile(p_hash bytea,p_store uuid,p_customer uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v jsonb;
BEGIN
 IF p_customer IS NULL THEN RAISE EXCEPTION 'invalid export request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:privacy');
 SELECT jsonb_build_object('display_name',p.display_name,'phone',p.phone_e164,'email',p.email,'source',p.source,
   'imported_at',to_char(p.imported_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'updated_at',to_char(p.updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'external_ids',coalesce((SELECT jsonb_agg(e.external_id ORDER BY e.external_id) FROM migrationimport.external_ids e
     WHERE e.tenant_id=p.tenant_id AND e.store_id=p.store_id AND e.kind='customers' AND e.internal_id=p.owner_id),'[]'::jsonb))
  INTO v FROM customers.import_profiles p WHERE p.tenant_id=a.tenant_id AND p.store_id=p_store AND p.owner_id=p_customer;
 PERFORM customers.tn_fence(p_hash,p_store,'customers:privacy',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v;
END $$;

DO $$
DECLARE f text;
BEGIN
 FOREACH f IN ARRAY ARRAY['migrationimport.import_customers(bytea,uuid,jsonb)','customers.export_import_profile(bytea,uuid,uuid)',
  'migrationimport.record_batch(bytea,uuid,text,bytea,jsonb,integer,integer,integer,integer,jsonb)',
  'migrationimport.read_batch_results(bytea,uuid,uuid)','customers.erase_import_profile(uuid,uuid,uuid)'] LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_privacy_writer',f);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
  IF f NOT LIKE 'customers.erase%' THEN EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime',f); END IF;
 END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------
-- Erasure: apply_erasure also calls erase_import_profile (prosrc patch behind the drift guard above, 0113/0139 pattern; OR REPLACE
-- keeps owner and ACL). replay_erasures reaches it through apply_erasure, so a restore re-deletes imported data too.
-- ---------------------------------------------------------------------------------------
DO $patch$
DECLARE body text; needle text:='-- CD4: withdraw every granted pair';
BEGIN
 SELECT prosrc INTO body FROM pg_proc WHERE oid='customers.apply_erasure(uuid,uuid,uuid)'::regprocedure;
 IF position(needle IN body)=0 THEN RAISE EXCEPTION 'erasure baseline drift'; END IF;
 body:=replace(body,needle,'PERFORM customers.erase_import_profile(p_tenant,p_store,p_owner); '||needle);
 EXECUTE 'CREATE OR REPLACE FUNCTION customers.apply_erasure(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS '||quote_literal(body);
END $patch$;

-- Tag/note writes accept an imported customer: tn_lock_owner / tn_owner_visible are the "visible to the merchant list" rule of
-- 0139, and the list now also shows import_profiles owners. Bodies are the 0139 text with the one extra EXISTS (W5-02B).
CREATE OR REPLACE FUNCTION customers.tn_lock_owner(p_tenant uuid,p_store uuid,p_owner uuid)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM buyer.owners o WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_owner AND o.active FOR UPDATE;
 IF NOT FOUND OR NOT (
  EXISTS(SELECT 1 FROM checkout.orders x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.owner_id=p_owner)
  OR EXISTS(SELECT 1 FROM claims.bundles b WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.owner_id=p_owner)
  OR EXISTS(SELECT 1 FROM customers.import_profiles ip WHERE ip.tenant_id=p_tenant AND ip.store_id=p_store AND ip.owner_id=p_owner)) THEN
  RAISE EXCEPTION 'customer not found' USING ERRCODE='PT404'; END IF;
END $$;

CREATE OR REPLACE FUNCTION customers.tn_owner_visible(p_tenant uuid,p_store uuid,p_owner uuid)
RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM buyer.owners o WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_owner)
  AND (EXISTS(SELECT 1 FROM checkout.orders x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.owner_id=p_owner)
   OR EXISTS(SELECT 1 FROM claims.bundles b WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.owner_id=p_owner)
   OR EXISTS(SELECT 1 FROM customers.import_profiles ip WHERE ip.tenant_id=p_tenant AND ip.store_id=p_store AND ip.owner_id=p_owner))
$$;

-- ---------------------------------------------------------------------------------------
-- identity.read_merchant_customers: the 0139 body in full with the lines marked "W5-02B" added (same signature, owner and ACL:
-- commerce_auth, EXECUTE commerce_runtime). An imported customer is listed through its profile, shows the imported name and phone
-- tail while it has no order, and carries imported:true. Consent is untouched: the existing consent_events projection reports an
-- imported customer as not granted (absence = not granted, CD4).
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION identity.read_merchant_customers(p_hash bytea,p_store uuid,p_customer uuid,p_limit integer,
 p_after_ts timestamptz,p_after_id uuid,p_q text,p_tag uuid DEFAULT NULL)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_result jsonb; v_auth_error text; v_digits text; v_phone boolean:=false;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
  OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 101
  OR (p_after_ts IS NULL)<>(p_after_id IS NULL)
  OR (p_after_ts IS NOT NULL AND NOT isfinite(p_after_ts))
  OR (p_customer IS NOT NULL AND (p_limit<>1 OR p_after_id IS NOT NULL OR p_q IS NOT NULL OR p_tag IS NOT NULL))
  OR (p_q IS NOT NULL AND (char_length(p_q) NOT BETWEEN 1 AND 40 OR p_q<>btrim(p_q) OR p_q ~ '[[:cntrl:]]'))
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid customer read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'customers:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- D6: digits (after removing spaces, '-' and '+') = phone-digits suffix; anything else = recipient_name prefix.
 IF p_q IS NOT NULL THEN
  v_digits:=regexp_replace(p_q,'[ +-]','','g');
  v_phone:=v_digits ~ '^[0-9]+$';
 END IF;

 -- ponytail: read-time aggregates (CD3); materialize when p95 > 300 ms at a measured store size (§9).
 -- The row set is driven from owners that HAVE activity (orders / bound bundles), never from every buyer.owners row:
 -- buyer.issue_capability creates an owner per anonymous visitor, so scanning owners made each page O(visitors)
 -- (lane-close review P2, CB03 10k idle owners). The per-customer aggregates below then run over customers only.
 WITH active_owner AS MATERIALIZED (
  -- W6-01B: tag filter. The active owners are restricted to the carriers of p_tag (an index probe on
  -- customer_owner_tags_by_tag through the set-returning helper), so the search starts from the tagged set, not from every
  -- customer. An unknown or other-store tag id matches nobody.
  SELECT o.owner_id AS id FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store
   AND (p_customer IS NULL OR o.owner_id=p_customer)
   AND (p_tag IS NULL OR o.owner_id IN (SELECT customers.tn_owners_with_tag(s.tenant_id,p_store,p_tag)))
  UNION
  SELECT b.owner_id FROM claims.bundles b WHERE b.tenant_id=s.tenant_id AND b.store_id=p_store AND b.owner_id IS NOT NULL
   AND (p_customer IS NULL OR b.owner_id=p_customer)
   AND (p_tag IS NULL OR b.owner_id IN (SELECT customers.tn_owners_with_tag(s.tenant_id,p_store,p_tag)))
  UNION
  -- W5-02B: an imported customer (customers.import_profiles) is a customer without an order or a bound bundle.
  SELECT ip.owner_id FROM customers.import_profiles ip WHERE ip.tenant_id=s.tenant_id AND ip.store_id=p_store
   AND (p_customer IS NULL OR ip.owner_id=p_customer)
   AND (p_tag IS NULL OR ip.owner_id IN (SELECT customers.tn_owners_with_tag(s.tenant_id,p_store,p_tag)))
 ), base AS MATERIALIZED (
  SELECT ow.tenant_id,ow.store_id,ow.id,ow.created_at AS first_seen,ow.active,
   greatest(
    (SELECT max(o.created_at) FROM checkout.orders o WHERE o.tenant_id=ow.tenant_id AND o.store_id=ow.store_id AND o.owner_id=ow.id),
    (SELECT max(b.bound_at) FROM claims.bundles b WHERE b.tenant_id=ow.tenant_id AND b.store_id=ow.store_id AND b.owner_id=ow.id),
    -- W5-02B: imported_at never changes, so the keyset on (last_activity,id) stays stable across re-imports.
    (SELECT ip.imported_at FROM customers.import_profiles ip WHERE ip.tenant_id=ow.tenant_id AND ip.store_id=ow.store_id AND ip.owner_id=ow.id)
   ) AS last_activity
  FROM active_owner ao
  -- LATERAL ... LIMIT 1 forces one primary-key probe per customer: without it the planner (no statistics right after a
  -- bulk insert of visitors) chose a hash join that read the whole owners table again.
  CROSS JOIN LATERAL (SELECT w.tenant_id,w.store_id,w.id,w.created_at,w.active FROM buyer.owners w
    WHERE w.id=ao.id AND w.tenant_id=s.tenant_id AND w.store_id=p_store LIMIT 1) ow
  WHERE (p_customer IS NULL OR ow.id=p_customer)
   AND (p_q IS NULL OR EXISTS(SELECT 1 FROM checkout.orders qo WHERE qo.tenant_id=ow.tenant_id AND qo.store_id=ow.store_id
     AND qo.owner_id=ow.id AND CASE WHEN v_phone
      THEN right(regexp_replace(coalesce(qo.snapshot#>>'{destination,phone}',''),'[^0-9]','','g'),char_length(v_digits))=v_digits
      ELSE starts_with(lower(coalesce(qo.snapshot#>>'{destination,recipient_name}','')),lower(p_q)) END)
    -- W5-02B: the same D6 rule over the import profile.
    OR EXISTS(SELECT 1 FROM customers.import_profiles qp WHERE qp.tenant_id=ow.tenant_id AND qp.store_id=ow.store_id
     AND qp.owner_id=ow.id AND CASE WHEN v_phone
      THEN right(regexp_replace(regexp_replace(coalesce(qp.phone_e164,''),'^\+886','0'),'[^0-9]','','g'),char_length(v_digits))=v_digits
      ELSE starts_with(lower(qp.display_name),lower(p_q)) END))
 ), paged AS MATERIALIZED (
  SELECT b.* FROM base b
  WHERE b.last_activity IS NOT NULL AND (p_after_id IS NULL OR (b.last_activity,b.id)<(p_after_ts,p_after_id))
  ORDER BY b.last_activity DESC,b.id DESC LIMIT p_limit
 ), projected AS (
  SELECT pg.last_activity,pg.id,
   jsonb_build_object(
    'customer_id',pg.id,
    'first_seen_at',to_char(pg.first_seen AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
    'last_activity_at',to_char(pg.last_activity AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
    'display_name',coalesce(lo.name,ip.display_name),
    'phone_last3',coalesce(lo.last3,ip.last3),
    'orders_count',coalesce(ag.orders_count,0),'paid_orders_count',coalesce(ag.paid_orders_count,0),
    'captured_minor',coalesce(ag.captured_minor,0),'refunded_minor',coalesce(ag.refunded_minor,0),
    'currency',lo.currency,
    'claims_count',coalesce(cl.claims_count,0),'platforms',coalesce(cl.platforms,'[]'::jsonb),
    'consents',jsonb_build_object(
     'marketing_messages',coalesce((SELECT e.granted FROM customers.consent_events e WHERE e.tenant_id=pg.tenant_id
       AND e.store_id=pg.store_id AND e.owner_id=pg.id AND e.purpose='marketing_messages' AND e.channel='meta_dm'
       ORDER BY e.occurred_at DESC,e.id DESC LIMIT 1),false),
     'ads_personalization',coalesce((SELECT e.granted FROM customers.consent_events e WHERE e.tenant_id=pg.tenant_id
       AND e.store_id=pg.store_id AND e.owner_id=pg.id AND e.purpose='ads_personalization' AND e.channel='meta_ads'
       ORDER BY e.occurred_at DESC,e.id DESC LIMIT 1),false)),
    'active',pg.active,
    'imported',ip.owner_id IS NOT NULL,
    -- W6-01B: merchant-typed store tags, name-ordered; the only tag key of the list row.
    'tags',customers.tn_tags_json(pg.tenant_id,pg.store_id,pg.id))
   ||CASE WHEN p_customer IS NULL THEN '{}'::jsonb ELSE jsonb_build_object(
    -- W6-01B detail extras: CAS token for PUT .../tags and the newest 50 merchant notes (older ones: customers.list_notes).
    'tags_revision',customers.tn_tags_revision(pg.tenant_id,pg.store_id,pg.id),
    'notes',customers.tn_notes_json(pg.tenant_id,pg.store_id,pg.id,50,NULL,NULL),
    'order_ids',coalesce((SELECT jsonb_agg(x.id ORDER BY x.created_at DESC,x.id DESC)
       FROM (SELECT o.id,o.created_at FROM checkout.orders o WHERE o.tenant_id=pg.tenant_id AND o.store_id=pg.store_id
         AND o.owner_id=pg.id ORDER BY o.created_at DESC,o.id DESC LIMIT 201) x),'[]'::jsonb),
    'claims',coalesce((SELECT jsonb_agg(jsonb_build_object('session_id',b.session_id,'platform',b.platform,
       'bound_at',to_char(b.bound_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'line_count',b.line_count)
       ORDER BY b.bound_at DESC,b.id DESC)
       FROM (SELECT c.id,c.session_id,c.platform,c.bound_at,c.line_count FROM claims.bundles c
         WHERE c.tenant_id=pg.tenant_id AND c.store_id=pg.store_id
         AND c.owner_id=pg.id ORDER BY c.bound_at DESC,c.id DESC LIMIT 100) b),'[]'::jsonb),
    'consent_history',coalesce((SELECT jsonb_agg(jsonb_build_object('purpose',x.purpose,'channel',x.channel,
       'granted',x.granted,'source',x.source,'policy_version',x.policy_version,
       'occurred_at',to_char(x.occurred_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))
       ORDER BY x.occurred_at DESC,x.id DESC)
       FROM (SELECT e.* FROM customers.consent_events e WHERE e.tenant_id=pg.tenant_id AND e.store_id=pg.store_id
         AND e.owner_id=pg.id ORDER BY e.occurred_at DESC,e.id DESC LIMIT 500) x),'[]'::jsonb),
    'privacy_actions',coalesce((SELECT jsonb_agg(jsonb_build_object('kind',y.kind,'via',y.via,
       'completed_at',to_char(y.completed_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'summary',y.summary)
       ORDER BY y.completed_at DESC,y.id DESC)
       FROM (SELECT a.* FROM customers.privacy_actions a WHERE a.tenant_id=pg.tenant_id AND a.store_id=pg.store_id
         AND a.owner_id=pg.id ORDER BY a.completed_at DESC,a.id DESC LIMIT 100) y),'[]'::jsonb)) END AS value
  FROM paged pg
  -- D7: display name, phone last 3 and currency come from the LATEST order's destination (null for bundle-only owners).
  LEFT JOIN LATERAL (
   SELECT o.currency,o.snapshot#>>'{destination,recipient_name}' AS name,
    right(regexp_replace(coalesce(o.snapshot#>>'{destination,phone}',''),'[^0-9]','','g'),3) AS last3
   FROM checkout.orders o WHERE o.tenant_id=pg.tenant_id AND o.store_id=pg.store_id AND o.owner_id=pg.id
   ORDER BY o.created_at DESC,o.id DESC LIMIT 1
  ) lo ON true
  -- W5-02B: imported name and phone tail for a customer without orders (an order's destination wins when both exist).
  LEFT JOIN LATERAL (
   SELECT p.owner_id,p.display_name,
    nullif(right(regexp_replace(regexp_replace(coalesce(p.phone_e164,''),'^\+886','0'),'[^0-9]','','g'),3),'') AS last3
   FROM customers.import_profiles p WHERE p.tenant_id=pg.tenant_id AND p.store_id=pg.store_id AND p.owner_id=pg.id LIMIT 1
  ) ip ON true
  -- I05: money sums cover only the latest order's currency (single-currency stores in v1). "Captured" uses the same
  -- fact-matches-attempt-matches-order predicate as identity.read_merchant_orders; refunded = held (no FAILED/CANCELED/
  -- REJECTED fact) with a SUCCEEDED fact, the stripe-refund-v1 §7.1 rule.
  LEFT JOIN LATERAL (
   SELECT count(*) AS orders_count,count(*) FILTER (WHERE x.captured) AS paid_orders_count,
    coalesce(sum(x.total_minor) FILTER (WHERE x.captured AND x.currency=lo.currency),0)::bigint AS captured_minor,
    coalesce(sum(x.refunded) FILTER (WHERE x.captured AND x.currency=lo.currency),0)::bigint AS refunded_minor
   FROM (
    SELECT o.total_minor,o.currency,
     EXISTS(SELECT 1 FROM checkout.payment_attempts a JOIN payments.facts f ON f.tenant_id=a.tenant_id AND f.store_id=a.store_id
       AND f.attempt_id=a.id AND f.kind='CAPTURED' AND f.connection_id=a.connection_id AND f.execution_profile=a.execution_profile
       AND f.environment=a.environment AND f.currency=a.currency AND f.amount_minor=a.amount_minor
       AND f.currency=o.currency AND f.amount_minor=o.total_minor
      WHERE a.tenant_id=o.tenant_id AND a.store_id=o.store_id AND a.owner_id=o.owner_id AND a.order_id=o.id) AS captured,
     coalesce((SELECT sum(r.amount_minor) FROM checkout.payment_attempts a JOIN payments.stripe_refunds r
        ON r.tenant_id=a.tenant_id AND r.store_id=a.store_id AND r.attempt_id=a.id
       WHERE a.tenant_id=o.tenant_id AND a.store_id=o.store_id AND a.owner_id=o.owner_id AND a.order_id=o.id
        AND NOT EXISTS(SELECT 1 FROM payments.refund_facts rf WHERE rf.tenant_id=r.tenant_id AND rf.store_id=r.store_id
          AND rf.refund_id=r.id AND rf.kind IN ('FAILED','CANCELED','REJECTED'))
        AND EXISTS(SELECT 1 FROM payments.refund_facts rs WHERE rs.tenant_id=r.tenant_id AND rs.store_id=r.store_id
          AND rs.refund_id=r.id AND rs.kind='SUCCEEDED')),0) AS refunded
    FROM checkout.orders o WHERE o.tenant_id=pg.tenant_id AND o.store_id=pg.store_id AND o.owner_id=pg.id
   ) x
  ) ag ON true
  LEFT JOIN LATERAL (
   SELECT count(*) AS claims_count,coalesce(jsonb_agg(DISTINCT c.platform ORDER BY c.platform),'[]'::jsonb) AS platforms
   FROM claims.bundles c WHERE c.tenant_id=pg.tenant_id AND c.store_id=pg.store_id AND c.owner_id=pg.id
  ) cl ON true
 )
 SELECT coalesce(jsonb_agg(value ORDER BY last_activity DESC,id DESC),'[]'::jsonb) INTO v_result FROM projected;

 -- Fresh final fence AFTER the data reads, BEFORE the empty/not-found branches (no post-revocation existence oracle).
 v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['customers:read'],s.tenant_id,s.principal_id,s.authz_revision);
 IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'customer read access denied' USING ERRCODE=v_auth_error; END IF;
 IF p_customer IS NOT NULL AND jsonb_array_length(v_result)=0 THEN
  RAISE EXCEPTION 'customer not found' USING ERRCODE='PT404'; END IF;
 IF octet_length(v_result::text)>524288 THEN
  RAISE EXCEPTION 'customer read unavailable' USING ERRCODE='PT503'; END IF;
 RETURN v_result;
END $$;

-- ---------------------------------------------------------------------------------------
-- Comments (every object names its owning package).
-- ---------------------------------------------------------------------------------------
COMMENT ON SCHEMA migrationimport IS 'internal/migrationimport (W5-02B): merchant CSV import framework tables (batches, external ids) and the commerce_privacy_writer definers that write them. No login role has a table grant.';
COMMENT ON TABLE migrationimport.batches IS 'internal/migrationimport (W5-02B): one committed import file per store and kind (UNIQUE file_sha256 = per-file idempotency), its mapping (<=2 KiB) and per-row results {row,outcome,code?,external_id?}. No name, phone or email. Written by migrationimport.record_batch (lazy 90-day prune); the erasure scrub removes one external_id from results. FORCE RLS, definers only.';
COMMENT ON COLUMN migrationimport.batches.file_sha256 IS 'internal/migrationimport: sha256 of the exact uploaded CSV bytes; the UNIQUE key that makes a re-submit replay instead of re-import.';
COMMENT ON COLUMN migrationimport.batches.mapping IS 'internal/migrationimport: canonical column -> CSV header map used for this file (header names only).';
COMMENT ON COLUMN migrationimport.batches.applied IS 'internal/migrationimport: rows that created a new record in this commit.';
COMMENT ON COLUMN migrationimport.batches.updated IS 'internal/migrationimport: rows that updated an existing record (same external id).';
COMMENT ON COLUMN migrationimport.batches.failed IS 'internal/migrationimport: rows skipped with a code (parse, duplicate or validation); never written.';
COMMENT ON COLUMN migrationimport.batches.results IS 'internal/migrationimport: per-row verdicts {row, outcome created|updated|failed, code?, external_id?}; external_id is the merchant source-system id, scrubbed on erasure of the customer.';
COMMENT ON TABLE migrationimport.store_salts IS 'internal/migrationimport (W5-02B): one random 32-byte salt per store used only to digest erased source ids (tombstones). Insert-only; deleting a store deletes its salt and so every tombstone. FORCE RLS, definers only.';
COMMENT ON COLUMN migrationimport.store_salts.salt IS 'internal/migrationimport: 32 random bytes from two v4 uuids (pgcrypto is not installed).';
COMMENT ON TABLE migrationimport.erased_external_ids IS 'internal/migrationimport (W5-02B): erasure tombstones: sha256(store salt || external id) of an erased imported customer, so the same source id is refused (code erased) when a stale export is re-imported. No name, phone, email or owner link. Insert-only. FORCE RLS, definers only.';
COMMENT ON COLUMN migrationimport.erased_external_ids.id_digest IS 'internal/migrationimport: sha256(salt || external_id as stored in external_ids); never an unsalted hash.';
COMMENT ON TABLE migrationimport.external_ids IS 'internal/migrationimport (W5-02B): source-system id -> internal id per (store, kind); the key that makes a re-import update instead of duplicate. For kind customers internal_id is buyer.owners.id; deleted by erasure (customers.erase_import_profile). FORCE RLS, definers only.';
COMMENT ON COLUMN migrationimport.external_ids.external_id IS 'internal/migrationimport: the merchant source-system record id (1..64 chars), pseudonymous but linkable, so erasure deletes it.';
COMMENT ON COLUMN migrationimport.external_ids.internal_id IS 'internal/migrationimport: our id for the imported record (buyer.owners.id for customers).';
COMMENT ON TABLE customers.import_profiles IS 'internal/customers (W5-02B): name, phone (E.164) and email of an imported customer; the customer is merchant-visible through this row. Buyer personal data: deleted by erasure (apply_erasure -> erase_import_profile). Imported consent is always unknown: nothing here or in the import writes customers.consent_events. Written only by migrationimport.import_customers.';
COMMENT ON COLUMN customers.import_profiles.display_name IS 'internal/customers: imported customer name, 1..80 characters, trimmed.';
COMMENT ON COLUMN customers.import_profiles.phone_e164 IS 'internal/customers: Taiwan mobile normalised to E.164 (+8869xxxxxxxx) by the importer; NULL when the file had none.';
COMMENT ON COLUMN customers.import_profiles.email IS 'internal/customers: imported email, lower-cased, <=254; NULL when the file had none.';
COMMENT ON COLUMN customers.import_profiles.source IS 'internal/customers: import source of the row; only shopline_csv exists.';
COMMENT ON COLUMN customers.import_profiles.imported_at IS 'internal/customers: first import time; the list activity key of an imported customer (never changes).';
COMMENT ON COLUMN customers.import_profiles.updated_at IS 'internal/customers: last time a re-import changed the profile.';
COMMENT ON COLUMN customers.import_profiles.owner_id IS 'internal/customers: the buyer.owners row created for this imported customer (no capability session, never merged with a buyer).';
COMMENT ON COLUMN customers.import_profiles.tenant_id IS 'internal/customers: tenant scope (FORCE RLS GUC).';
COMMENT ON COLUMN customers.import_profiles.store_id IS 'internal/customers: store scope (FORCE RLS GUC).';
COMMENT ON FUNCTION migrationimport.import_customers(bytea,uuid,jsonb) IS 'internal/migrationimport (customers:privacy): upserts <=500 validated rows [{row,external_id,name,phone,email}] as buyer.owners + import_profiles + external_ids and returns [{row,outcome created|updated}]. Per-store advisory lock; never writes customers.consent_events.';
COMMENT ON FUNCTION migrationimport.record_batch(bytea,uuid,text,bytea,jsonb,integer,integer,integer,integer,jsonb) IS 'internal/migrationimport (customers:privacy): records the committed import batch (UNIQUE per file hash), prunes this store''s batches older than 90 days (<=50) and audits customers.imported; returns the batch id.';
COMMENT ON FUNCTION migrationimport.read_batch_results(bytea,uuid,uuid) IS 'internal/migrationimport (customers:privacy): the per-row results jsonb of one batch of this store for results.csv; PT404 otherwise.';
COMMENT ON FUNCTION customers.erase_import_profile(uuid,uuid,uuid) IS 'internal/customers erasure-only (called by apply_erasure, EXECUTE nobody): writes the salted tombstone digest of the owner''s customer external ids, then deletes the import profile and external id and scrubs that external id from retained batch results.';
COMMENT ON FUNCTION customers.export_import_profile(bytea,uuid,uuid) IS 'internal/customers (customers:privacy): the imported profile (name, phone, email, source, timestamps, external ids) of one customer for the merchant privacy export, or NULL when it has none.';
COMMENT ON FUNCTION customers.tn_lock_owner(uuid,uuid,uuid) IS 'internal/customers (W6-01B, W5-02B): Internal (EXECUTE nobody): locks an active, merchant-visible customer owner row (order, bound bundle or import profile); same lock order as erase_owner.';
COMMENT ON FUNCTION customers.tn_owner_visible(uuid,uuid,uuid) IS 'internal/customers (W6-01B, W5-02B): Internal (EXECUTE nobody): whether a customer is visible to the merchant list (order, bound bundle or import profile).';
COMMENT ON FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) IS
 'internal/customers (customers:read): list (limit 1..101, keyset on (last_activity_at,id) DESC, optional p_tag filter) or detail (p_customer). Rows carry tags and imported (W5-02B: customers with an import profile are listed and searchable); the detail adds tags_revision and the newest 50 notes.';
