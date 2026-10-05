-- 0139_customer_tags_notes.sql (unit W6-01B, M16 #5): merchant-typed customer tags and private notes.
-- Purpose: tables customers.tags / owner_tags / notes, the customers:write permission, the tag/note write definers, the
--   tag filter + tag/note projection of identity.read_merchant_customers (NEW 8-argument signature, old one dropped), and
--   erasure/export coverage (apply_erasure deletes tags links + notes; buyer_read_privacy(detail) exports them).
-- Depends on: 0078 (customers schema, commerce_privacy_writer, apply_erasure, buyer_read_privacy, read_merchant_customers),
--   0089/0119 (store_grants_permission_check re-derivation pattern, staff role bundles), 0113 (apply_erasure patch pattern).
-- Used by: internal/customers/{tags,notes,read,privacy}.go, internal/httpapi/customer_tags.go; W5-02B extends the
--   read_merchant_customers body defined below (keep its new text comment-marked "W6-01B").
-- Invariants: tags are merchant-typed facts, never inferred (architecture 14.3 inferred tags are a separate contract); notes are
--   buyer personal data (erasure deletes them, the buyer self-service export includes them, CT-OPEN-1 default ruling);
--   no login role has a direct grant on the new tables (definers only); audit rows never carry a note body.
-- Status: REAL_PG / MOCK evidence only.

DO $$
DECLARE v_def text; v_src text; v_role text;
BEGIN
 IF to_regclass('buyer.owners') IS NULL OR to_regclass('customers.privacy_actions') IS NULL
  OR to_regprocedure('identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text)') IS NULL
  OR to_regprocedure('customers.apply_erasure(uuid,uuid,uuid)') IS NULL
  OR to_regprocedure('customers.buyer_read_privacy(bytea,uuid,boolean)') IS NULL
  OR to_regprocedure('identity.staff_role_permissions(text)') IS NULL THEN
  RAISE EXCEPTION '0139 requires 0078 (customers/privacy), 0089 (staff bundles)';
 END IF;
 FOREACH v_role IN ARRAY ARRAY['commerce_privacy_writer','commerce_auth','commerce_runtime'] LOOP
  IF to_regrole(v_role) IS NULL THEN RAISE EXCEPTION '0139 requires role %',v_role; END IF;
 END LOOP;
 -- Drift guards for the two source patches below (0113 pattern): fail loudly rather than patch a changed body.
 SELECT prosrc INTO v_src FROM pg_proc WHERE oid='customers.apply_erasure(uuid,uuid,uuid)'::regprocedure;
 IF position('-- CD4: withdraw every granted pair' IN v_src)=0 THEN RAISE EXCEPTION '0139 erasure baseline drift'; END IF;
 SELECT prosrc INTO v_src FROM pg_proc WHERE oid='customers.buyer_read_privacy(bytea,uuid,boolean)'::regprocedure;
 IF (length(v_src)-length(replace(v_src,E' RETURN v_result;\nEND',''))) <> length(E' RETURN v_result;\nEND') THEN
  RAISE EXCEPTION '0139 buyer_read_privacy baseline drift'; END IF;
END $$;

-- ---------------------------------------------------------------------------------------
-- Permission vocabulary: re-derive the CURRENT store_grants_permission_check (0119 is the last writer) and append
-- customers:write when absent, so no later value is dropped (0063/0079/0119 pattern).
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_def text; v_list text[];
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO STRICT v_def FROM pg_constraint c
  WHERE c.conrelid='identity.store_grants'::regclass AND c.conname='store_grants_permission_check';
 SELECT array_agg(t.m[1] ORDER BY t.ord) INTO v_list
  FROM regexp_matches(v_def,'''([a-z_]+:[a-z_]+)''::text','g') WITH ORDINALITY AS t(m,ord);
 IF v_list IS NULL OR NOT ('customers:read'=ANY(v_list) AND 'customers:privacy'=ANY(v_list) AND 'inbox:reply'=ANY(v_list)) THEN
  RAISE EXCEPTION '0139 applied out of order: store_grants_permission_check lacks 0078/0119 permissions: %',v_def;
 END IF;
 IF NOT 'customers:write'=ANY(v_list) THEN v_list:=array_append(v_list,'customers:write'::text); END IF;
 ALTER TABLE identity.store_grants DROP CONSTRAINT store_grants_permission_check;
 EXECUTE format('ALTER TABLE identity.store_grants ADD CONSTRAINT store_grants_permission_check CHECK (permission IN (%s))',
  (SELECT string_agg(quote_literal(p),',' ORDER BY ord) FROM unnest(v_list) WITH ORDINALITY AS u(p,ord)));
END $$;

-- Role bundles: owner/admin follow the live catalogue (staff_role_permissions of 0089/0119 is unchanged), so they gain
-- customers:write by the catalogue. viewer is :read only, live_operator/fulfilment are fixed lists: none gets it
-- (ruling in DELIVERY.md: the live_operator bundle does not hold customers:read either, so writing without reading makes no sense).
-- Additive backfill for existing owner/admin staff (the 0089 D1 / 0119 pattern; nothing is removed).
INSERT INTO identity.store_grants(tenant_id, store_id, principal_id, permission)
SELECT t.tenant_id, t.store_id, t.principal_id, x FROM identity.store_staff t, unnest(identity.staff_role_permissions(t.role)) x
WHERE t.role IN ('owner','admin')
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------------------
-- Tables. Written ONLY by the commerce_privacy_writer definers below; read by those definers and by the helper readers
-- that identity.read_merchant_customers (commerce_auth) calls. FORCE RLS; every policy is GUC-scoped to the store.
-- ---------------------------------------------------------------------------------------
CREATE TABLE customers.tags (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL DEFAULT gen_random_uuid(),
 name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 20 AND name=normalize(name,NFC) AND name=btrim(name)
  AND name !~ '[[:cntrl:]]'),
 color text NOT NULL CHECK (color IN ('gray','red','orange','yellow','green','teal','blue','purple')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id));
-- Case-insensitive unique name per store (CT02).
CREATE UNIQUE INDEX customer_tags_name ON customers.tags(tenant_id,store_id,lower(name));

CREATE TABLE customers.owner_tags (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, owner_id uuid NOT NULL, tag_id uuid NOT NULL,
 PRIMARY KEY (tenant_id,store_id,owner_id,tag_id),
 FOREIGN KEY (tenant_id,store_id,owner_id) REFERENCES buyer.owners(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,store_id,tag_id) REFERENCES customers.tags(tenant_id,store_id,id));
CREATE INDEX customer_owner_tags_by_tag ON customers.owner_tags(tenant_id,store_id,tag_id,owner_id);

CREATE TABLE customers.notes (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL DEFAULT gen_random_uuid(),
 owner_id uuid NOT NULL,
 body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000 AND btrim(body)<>''),
 author_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 edited_at timestamptz,
 version bigint NOT NULL DEFAULT 1 CHECK (version>=1),
 PRIMARY KEY (tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,store_id,owner_id) REFERENCES buyer.owners(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,author_id) REFERENCES identity.memberships(tenant_id,principal_id));
CREATE INDEX customer_notes_by_owner ON customers.notes(tenant_id,store_id,owner_id,created_at DESC,id DESC);

ALTER TABLE customers.tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.tags FORCE ROW LEVEL SECURITY;
ALTER TABLE customers.owner_tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.owner_tags FORCE ROW LEVEL SECURITY;
ALTER TABLE customers.notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.notes FORCE ROW LEVEL SECURITY;

-- Column-level grants (no table-wide UPDATE). Policies name one command each and are GUC-scoped (the 0078 writer pattern).
GRANT SELECT, INSERT, DELETE ON customers.tags, customers.owner_tags, customers.notes TO commerce_privacy_writer;
GRANT UPDATE (name,color) ON customers.tags TO commerce_privacy_writer;
GRANT UPDATE (body,edited_at,version) ON customers.notes TO commerce_privacy_writer;
DO $$
DECLARE t text; c text;
BEGIN
 FOREACH t IN ARRAY ARRAY['tags','owner_tags','notes'] LOOP
  FOREACH c IN ARRAY ARRAY['SELECT','DELETE'] LOOP
   EXECUTE format('CREATE POLICY %I ON customers.%I FOR %s TO commerce_privacy_writer USING (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',
    'tn_'||t||'_'||lower(c),t,c);
  END LOOP;
  EXECUTE format('CREATE POLICY %I ON customers.%I FOR INSERT TO commerce_privacy_writer WITH CHECK (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',
   'tn_'||t||'_insert',t);
 END LOOP;
 FOREACH t IN ARRAY ARRAY['tags','notes'] LOOP
  EXECUTE format('CREATE POLICY %I ON customers.%I FOR UPDATE TO commerce_privacy_writer USING (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid) WITH CHECK (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',
   'tn_'||t||'_update',t);
 END LOOP;
END $$;

-- Audit: the privacy writer may now also insert the seven tag/note actions (names only, never a body). The policy is
-- replaced (not added to) because the ACL pin requires every INSERT policy of the role to carry the full action list.
DROP POLICY privacy_audit_insert ON ops.audit_events;
CREATE POLICY privacy_audit_insert ON ops.audit_events FOR INSERT TO commerce_privacy_writer
 WITH CHECK (action IN ('customers.consent_withdrawn','customers.exported','customers.erased',
   'customers.tag_created','customers.tag_renamed','customers.tag_deleted','customers.tagged',
   'customers.note_added','customers.note_edited','customers.note_deleted')
  AND tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);

-- ---------------------------------------------------------------------------------------
-- Internal helpers (owner commerce_privacy_writer). EXECUTE: nobody for the authority/lock helpers (only same-owner
-- definers call them); commerce_auth for the three JSON readers identity.read_merchant_customers uses.
-- ---------------------------------------------------------------------------------------
-- tn_authority: the 0078 merchant prologue in one place (resolve_access, GUC equality, never set_config). Same PT codes.
CREATE FUNCTION customers.tn_authority(p_hash bytea,p_store uuid,p_permission text)
RETURNS TABLE(tenant_id uuid,principal_id uuid,authz_revision bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_permission IS NULL THEN
  RAISE EXCEPTION 'invalid request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,p_permission);
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN QUERY SELECT s.tenant_id,s.principal_id,s.authz_revision;
END $$;

-- tn_fence: authority re-read AFTER the writes and lock waits (no post-revocation success), as 0078 does.
CREATE FUNCTION customers.tn_fence(p_hash bytea,p_store uuid,p_permission text,p_tenant uuid,p_principal uuid,p_revision bigint)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE f record;
BEGIN
 SELECT * INTO f FROM identity.resolve_access(p_hash,p_store,p_permission);
 IF f.access_status IS DISTINCT FROM 'ok' OR f.tenant_id IS DISTINCT FROM p_tenant OR f.principal_id IS DISTINCT FROM p_principal
  OR f.authz_revision IS DISTINCT FROM p_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
END $$;

-- tn_lock_owner: serialises tag/note writes of one customer with each other AND with erasure (erase_owner takes the same
-- row lock first). A customer is writable only while active and only if it is visible to the merchant list (>=1 order or
-- bound bundle, the read_merchant_customers rule), so a stranger's or an erased owner id is a plain PT404.
CREATE FUNCTION customers.tn_lock_owner(p_tenant uuid,p_store uuid,p_owner uuid)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 PERFORM 1 FROM buyer.owners o WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_owner AND o.active FOR UPDATE;
 IF NOT FOUND OR NOT (
  EXISTS(SELECT 1 FROM checkout.orders x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.owner_id=p_owner)
  OR EXISTS(SELECT 1 FROM claims.bundles b WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.owner_id=p_owner)) THEN
  RAISE EXCEPTION 'customer not found' USING ERRCODE='PT404'; END IF;
END $$;

-- tn_owner_visible: read-side twin of tn_lock_owner (no lock, erased owners stay visible only through the list rule).
CREATE FUNCTION customers.tn_owner_visible(p_tenant uuid,p_store uuid,p_owner uuid)
RETURNS boolean LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS(SELECT 1 FROM buyer.owners o WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.id=p_owner)
  AND (EXISTS(SELECT 1 FROM checkout.orders x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.owner_id=p_owner)
   OR EXISTS(SELECT 1 FROM claims.bundles b WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.owner_id=p_owner))
$$;

CREATE FUNCTION customers.tn_check_tag(p_name text,p_color text) RETURNS void
LANGUAGE plpgsql IMMUTABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_name IS NOT NULL AND (char_length(p_name) NOT BETWEEN 1 AND 20 OR p_name<>normalize(p_name,NFC)
   OR p_name<>btrim(p_name) OR p_name ~ '[[:cntrl:]]') THEN
  RAISE EXCEPTION 'invalid tag name' USING ERRCODE='PT422'; END IF;
 IF p_color IS NOT NULL AND p_color NOT IN ('gray','red','orange','yellow','green','teal','blue','purple') THEN
  RAISE EXCEPTION 'invalid tag color' USING ERRCODE='PT422'; END IF;
END $$;

-- JSON readers. The tags of one customer, name-ordered; the CAS token of that set; the notes newest first (keyset on
-- (created_at,id)). Callers have already authorised and scoped (GUCs) the request; the policies re-check tenant/store.
CREATE FUNCTION customers.tn_tags_json(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT coalesce(jsonb_agg(jsonb_build_object('id',t.id,'name',t.name,'color',t.color) ORDER BY lower(t.name),t.id),'[]'::jsonb)
 FROM customers.owner_tags ot JOIN customers.tags t ON t.tenant_id=ot.tenant_id AND t.store_id=ot.store_id AND t.id=ot.tag_id
 WHERE ot.tenant_id=p_tenant AND ot.store_id=p_store AND ot.owner_id=p_owner
$$;

-- Revision = sha256 of the sorted tag-id list: equal sets have equal revisions, so a replaced-by-same set is not a conflict.
CREATE FUNCTION customers.tn_tags_revision(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT encode(sha256(convert_to(coalesce((SELECT string_agg(ot.tag_id::text,',' ORDER BY ot.tag_id) FROM customers.owner_tags ot
  WHERE ot.tenant_id=p_tenant AND ot.store_id=p_store AND ot.owner_id=p_owner),''),'UTF8')),'hex')
$$;

CREATE FUNCTION customers.tn_note_json(n customers.notes) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object('id',n.id,'body',n.body,'author_id',n.author_id,
  'created_at',to_char(n.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
  'edited_at',CASE WHEN n.edited_at IS NULL THEN NULL ELSE to_char(n.edited_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') END,
  'version',n.version)
$$;

CREATE FUNCTION customers.tn_notes_json(p_tenant uuid,p_store uuid,p_owner uuid,p_limit integer,p_before_ts timestamptz,p_before_id uuid)
RETURNS jsonb LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT coalesce(jsonb_agg(n.j ORDER BY n.created_at DESC,n.id DESC),'[]'::jsonb)
 FROM (SELECT customers.tn_note_json(x) AS j,x.created_at,x.id FROM customers.notes x
   WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.owner_id=p_owner
   AND (p_before_id IS NULL OR (x.created_at,x.id)<(p_before_ts,p_before_id))
   ORDER BY x.created_at DESC,x.id DESC LIMIT p_limit) n
$$;

-- ---------------------------------------------------------------------------------------
-- Tag definers (customers:write). Errors: PT422 invalid input, PT409 messages tag_exists / limit_reached / version_changed,
-- PT404 unknown tag or customer. One advisory lock per store serialises name uniqueness and the 100-tag cap.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION customers.create_tag(p_hash bytea,p_store uuid,p_name text,p_color text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_row customers.tags%ROWTYPE;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:write');
 IF p_name IS NULL OR p_color IS NULL THEN RAISE EXCEPTION 'invalid tag' USING ERRCODE='PT422'; END IF;
 PERFORM customers.tn_check_tag(p_name,p_color);
 PERFORM pg_advisory_xact_lock(hashtextextended('lc:customer-tags:'||p_store::text,0));
 IF EXISTS(SELECT 1 FROM customers.tags t WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store AND lower(t.name)=lower(p_name)) THEN
  RAISE EXCEPTION 'tag_exists' USING ERRCODE='PT409'; END IF;
 IF (SELECT count(*) FROM customers.tags t WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store)>=100 THEN
  RAISE EXCEPTION 'limit_reached' USING ERRCODE='PT409'; END IF;
 INSERT INTO customers.tags(tenant_id,store_id,name,color) VALUES(a.tenant_id,p_store,p_name,p_color) RETURNING * INTO v_row;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(a.tenant_id,p_store,a.principal_id,'customers.tag_created');
 PERFORM customers.tn_fence(p_hash,p_store,'customers:write',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('id',v_row.id,'name',v_row.name,'color',v_row.color,
  'created_at',to_char(v_row.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'customers_count',0);
END $$;

-- rename_tag: p_name and/or p_color (NULL = unchanged, at least one); the same case-insensitive uniqueness as create.
CREATE FUNCTION customers.rename_tag(p_hash bytea,p_store uuid,p_tag uuid,p_name text,p_color text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_row customers.tags%ROWTYPE;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:write');
 IF p_tag IS NULL OR (p_name IS NULL AND p_color IS NULL) THEN RAISE EXCEPTION 'invalid tag' USING ERRCODE='PT422'; END IF;
 PERFORM customers.tn_check_tag(p_name,p_color);
 PERFORM pg_advisory_xact_lock(hashtextextended('lc:customer-tags:'||p_store::text,0));
 SELECT t.* INTO v_row FROM customers.tags t WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store AND t.id=p_tag FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'tag not found' USING ERRCODE='PT404'; END IF;
 IF p_name IS NOT NULL AND EXISTS(SELECT 1 FROM customers.tags t WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store
   AND lower(t.name)=lower(p_name) AND t.id<>p_tag) THEN
  RAISE EXCEPTION 'tag_exists' USING ERRCODE='PT409'; END IF;
 UPDATE customers.tags t SET name=coalesce(p_name,t.name),color=coalesce(p_color,t.color)
  WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store AND t.id=p_tag RETURNING * INTO v_row;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(a.tenant_id,p_store,a.principal_id,'customers.tag_renamed');
 PERFORM customers.tn_fence(p_hash,p_store,'customers:write',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('id',v_row.id,'name',v_row.name,'color',v_row.color,
  'created_at',to_char(v_row.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
  'customers_count',(SELECT count(*) FROM customers.owner_tags ot WHERE ot.tenant_id=a.tenant_id AND ot.store_id=p_store AND ot.tag_id=p_tag));
END $$;

-- delete_tag: the links go with the tag (CT08); customers, orders and notes are untouched.
CREATE FUNCTION customers.delete_tag(p_hash bytea,p_store uuid,p_tag uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_detached integer;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:write');
 IF p_tag IS NULL THEN RAISE EXCEPTION 'invalid tag' USING ERRCODE='PT422'; END IF;
 PERFORM 1 FROM customers.tags t WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store AND t.id=p_tag FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'tag not found' USING ERRCODE='PT404'; END IF;
 DELETE FROM customers.owner_tags ot WHERE ot.tenant_id=a.tenant_id AND ot.store_id=p_store AND ot.tag_id=p_tag;
 GET DIAGNOSTICS v_detached=ROW_COUNT;
 DELETE FROM customers.tags t WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store AND t.id=p_tag;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(a.tenant_id,p_store,a.principal_id,'customers.tag_deleted');
 PERFORM customers.tn_fence(p_hash,p_store,'customers:write',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('tag_id',p_tag,'detached',v_detached);
END $$;

-- set_owner_tags: whole-set replacement guarded by the revision the merchant last read (CT04). <= 20 tags per customer.
-- Lock order owner -> tag rows (FOR SHARE), the order delete_tag/erase_owner can never invert into a held-lock cycle with it
-- except by a detected deadlock (40P01, mapped to a retry by the HTTP layer).
CREATE FUNCTION customers.set_owner_tags(p_hash bytea,p_store uuid,p_owner uuid,p_tag_ids uuid[],p_expected_revision text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_found integer;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:write');
 IF p_owner IS NULL OR p_tag_ids IS NULL OR array_ndims(p_tag_ids)>1 OR array_position(p_tag_ids,NULL) IS NOT NULL
  OR p_expected_revision IS NULL OR p_expected_revision !~ '^[0-9a-f]{64}$'
  OR cardinality(p_tag_ids)<>(SELECT count(DISTINCT x) FROM unnest(p_tag_ids) x) THEN
  RAISE EXCEPTION 'invalid tag set' USING ERRCODE='PT422'; END IF;
 IF cardinality(p_tag_ids)>20 THEN RAISE EXCEPTION 'limit_reached' USING ERRCODE='PT409'; END IF;
 PERFORM customers.tn_lock_owner(a.tenant_id,p_store,p_owner);
 IF customers.tn_tags_revision(a.tenant_id,p_store,p_owner) IS DISTINCT FROM p_expected_revision THEN
  RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
 SELECT count(*) INTO v_found FROM (SELECT 1 FROM customers.tags t WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store
   AND t.id=ANY(p_tag_ids) ORDER BY t.id FOR SHARE) x;
 IF v_found<>cardinality(p_tag_ids) THEN RAISE EXCEPTION 'unknown tag' USING ERRCODE='PT422'; END IF;
 DELETE FROM customers.owner_tags ot WHERE ot.tenant_id=a.tenant_id AND ot.store_id=p_store AND ot.owner_id=p_owner
  AND ot.tag_id<>ALL(p_tag_ids);
 INSERT INTO customers.owner_tags(tenant_id,store_id,owner_id,tag_id)
  SELECT a.tenant_id,p_store,p_owner,x FROM unnest(p_tag_ids) x ON CONFLICT DO NOTHING;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(a.tenant_id,p_store,a.principal_id,'customers.tagged');
 PERFORM customers.tn_fence(p_hash,p_store,'customers:write',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('tags',customers.tn_tags_json(a.tenant_id,p_store,p_owner),
  'tags_revision',customers.tn_tags_revision(a.tenant_id,p_store,p_owner));
END $$;

-- ---------------------------------------------------------------------------------------
-- Note definers (customers:write). Author or a customers:privacy holder (owner/admin) may edit or delete a note.
-- Audit rows carry the action only, never the body (CT06/CT09).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION customers.add_note(p_hash bytea,p_store uuid,p_owner uuid,p_body text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_row customers.notes%ROWTYPE;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:write');
 IF p_owner IS NULL OR p_body IS NULL OR char_length(p_body) NOT BETWEEN 1 AND 1000 OR btrim(p_body)='' THEN
  RAISE EXCEPTION 'invalid note' USING ERRCODE='PT422'; END IF;
 PERFORM customers.tn_lock_owner(a.tenant_id,p_store,p_owner);
 IF (SELECT count(*) FROM customers.notes n WHERE n.tenant_id=a.tenant_id AND n.store_id=p_store AND n.owner_id=p_owner)>=200 THEN
  RAISE EXCEPTION 'limit_reached' USING ERRCODE='PT409'; END IF;
 INSERT INTO customers.notes(tenant_id,store_id,owner_id,body,author_id) VALUES(a.tenant_id,p_store,p_owner,p_body,a.principal_id)
  RETURNING * INTO v_row;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(a.tenant_id,p_store,a.principal_id,'customers.note_added');
 PERFORM customers.tn_fence(p_hash,p_store,'customers:write',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN customers.tn_note_json(v_row);
END $$;

CREATE FUNCTION customers.edit_note(p_hash bytea,p_store uuid,p_owner uuid,p_note uuid,p_body text,p_expected_version bigint)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_row customers.notes%ROWTYPE;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:write');
 IF p_owner IS NULL OR p_note IS NULL OR p_expected_version IS NULL OR p_expected_version<1 OR p_body IS NULL
  OR char_length(p_body) NOT BETWEEN 1 AND 1000 OR btrim(p_body)='' THEN
  RAISE EXCEPTION 'invalid note' USING ERRCODE='PT422'; END IF;
 PERFORM customers.tn_lock_owner(a.tenant_id,p_store,p_owner);
 SELECT n.* INTO v_row FROM customers.notes n WHERE n.tenant_id=a.tenant_id AND n.store_id=p_store AND n.owner_id=p_owner
  AND n.id=p_note FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'note not found' USING ERRCODE='PT404'; END IF;
 IF v_row.author_id<>a.principal_id AND (SELECT x.access_status FROM identity.resolve_access(p_hash,p_store,'customers:privacy') x)
   IS DISTINCT FROM 'ok' THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF v_row.version<>p_expected_version THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
 UPDATE customers.notes n SET body=p_body,edited_at=clock_timestamp(),version=n.version+1
  WHERE n.tenant_id=a.tenant_id AND n.store_id=p_store AND n.id=p_note RETURNING * INTO v_row;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(a.tenant_id,p_store,a.principal_id,'customers.note_edited');
 PERFORM customers.tn_fence(p_hash,p_store,'customers:write',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN customers.tn_note_json(v_row);
END $$;

CREATE FUNCTION customers.delete_note(p_hash bytea,p_store uuid,p_owner uuid,p_note uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_row customers.notes%ROWTYPE;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:write');
 IF p_owner IS NULL OR p_note IS NULL THEN RAISE EXCEPTION 'invalid note' USING ERRCODE='PT422'; END IF;
 PERFORM customers.tn_lock_owner(a.tenant_id,p_store,p_owner);
 SELECT n.* INTO v_row FROM customers.notes n WHERE n.tenant_id=a.tenant_id AND n.store_id=p_store AND n.owner_id=p_owner
  AND n.id=p_note FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'note not found' USING ERRCODE='PT404'; END IF;
 IF v_row.author_id<>a.principal_id AND (SELECT x.access_status FROM identity.resolve_access(p_hash,p_store,'customers:privacy') x)
   IS DISTINCT FROM 'ok' THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 DELETE FROM customers.notes n WHERE n.tenant_id=a.tenant_id AND n.store_id=p_store AND n.id=p_note;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(a.tenant_id,p_store,a.principal_id,'customers.note_deleted');
 PERFORM customers.tn_fence(p_hash,p_store,'customers:write',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN jsonb_build_object('note_id',p_note);
END $$;

-- ---------------------------------------------------------------------------------------
-- Read definers (customers:read): the store tag catalogue with usage counts, and one customer's notes page.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION customers.list_tags(p_hash bytea,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_result jsonb;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:read');
 SELECT coalesce(jsonb_agg(jsonb_build_object('id',t.id,'name',t.name,'color',t.color,
   'created_at',to_char(t.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'customers_count',(SELECT count(*) FROM customers.owner_tags ot WHERE ot.tenant_id=t.tenant_id AND ot.store_id=t.store_id
     AND ot.tag_id=t.id)) ORDER BY lower(t.name),t.id),'[]'::jsonb) INTO v_result
  FROM customers.tags t WHERE t.tenant_id=a.tenant_id AND t.store_id=p_store;
 PERFORM customers.tn_fence(p_hash,p_store,'customers:read',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v_result;
END $$;

CREATE FUNCTION customers.list_notes(p_hash bytea,p_store uuid,p_owner uuid,p_limit integer,p_before_ts timestamptz,p_before_id uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_result jsonb;
BEGIN
 SELECT * INTO a FROM customers.tn_authority(p_hash,p_store,'customers:read');
 IF p_owner IS NULL OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 101 OR (p_before_ts IS NULL)<>(p_before_id IS NULL)
  OR (p_before_ts IS NOT NULL AND NOT isfinite(p_before_ts)) THEN
  RAISE EXCEPTION 'invalid notes read' USING ERRCODE='PT400'; END IF;
 IF NOT customers.tn_owner_visible(a.tenant_id,p_store,p_owner) THEN
  RAISE EXCEPTION 'customer not found' USING ERRCODE='PT404'; END IF;
 v_result:=customers.tn_notes_json(a.tenant_id,p_store,p_owner,p_limit,p_before_ts,p_before_id);
 PERFORM customers.tn_fence(p_hash,p_store,'customers:read',a.tenant_id,a.principal_id,a.authz_revision);
 RETURN v_result;
END $$;

-- ---------------------------------------------------------------------------------------
-- Erasure and export coverage. erase_tags_notes is called by apply_erasure (also by replay_erasures, so a restore re-deletes);
-- export_tags_notes is called by buyer_read_privacy(detail) and returns names/bodies/timestamps only (no principal ids, D8).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION customers.erase_tags_notes(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS void
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 DELETE FROM customers.owner_tags ot WHERE ot.tenant_id=p_tenant AND ot.store_id=p_store AND ot.owner_id=p_owner;
 DELETE FROM customers.notes n WHERE n.tenant_id=p_tenant AND n.store_id=p_store AND n.owner_id=p_owner
$$;

CREATE FUNCTION customers.export_tags_notes(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS jsonb
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object(
  'tags',coalesce((SELECT jsonb_agg(jsonb_build_object('name',t.name,'color',t.color) ORDER BY lower(t.name),t.id)
    FROM customers.owner_tags ot JOIN customers.tags t ON t.tenant_id=ot.tenant_id AND t.store_id=ot.store_id AND t.id=ot.tag_id
    WHERE ot.tenant_id=p_tenant AND ot.store_id=p_store AND ot.owner_id=p_owner),'[]'::jsonb),
  'notes',coalesce((SELECT jsonb_agg(jsonb_build_object('body',n.body,
     'created_at',to_char(n.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
     'edited_at',CASE WHEN n.edited_at IS NULL THEN NULL ELSE to_char(n.edited_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') END)
     ORDER BY n.created_at DESC,n.id DESC)
    FROM customers.notes n WHERE n.tenant_id=p_tenant AND n.store_id=p_store AND n.owner_id=p_owner),'[]'::jsonb))
$$;

DO $$
DECLARE f text;
BEGIN
 FOREACH f IN ARRAY ARRAY[
  'customers.tn_authority(bytea,uuid,text)','customers.tn_fence(bytea,uuid,text,uuid,uuid,bigint)',
  'customers.tn_lock_owner(uuid,uuid,uuid)','customers.tn_owner_visible(uuid,uuid,uuid)','customers.tn_check_tag(text,text)',
  'customers.tn_tags_json(uuid,uuid,uuid)','customers.tn_tags_revision(uuid,uuid,uuid)','customers.tn_note_json(customers.notes)',
  'customers.tn_notes_json(uuid,uuid,uuid,integer,timestamptz,uuid)',
  'customers.create_tag(bytea,uuid,text,text)','customers.rename_tag(bytea,uuid,uuid,text,text)','customers.delete_tag(bytea,uuid,uuid)',
  'customers.set_owner_tags(bytea,uuid,uuid,uuid[],text)','customers.add_note(bytea,uuid,uuid,text)',
  'customers.edit_note(bytea,uuid,uuid,uuid,text,bigint)','customers.delete_note(bytea,uuid,uuid,uuid)',
  'customers.list_tags(bytea,uuid)','customers.list_notes(bytea,uuid,uuid,integer,timestamptz,uuid)',
  'customers.erase_tags_notes(uuid,uuid,uuid)','customers.export_tags_notes(uuid,uuid,uuid)'] LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_privacy_writer',f);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
 END LOOP;
 -- The merchant-facing definers: commerce_runtime only. Nothing here takes a boolean (the "a merchant can never grant" pin).
 FOREACH f IN ARRAY ARRAY['customers.create_tag(bytea,uuid,text,text)','customers.rename_tag(bytea,uuid,uuid,text,text)',
  'customers.delete_tag(bytea,uuid,uuid)','customers.set_owner_tags(bytea,uuid,uuid,uuid[],text)','customers.add_note(bytea,uuid,uuid,text)',
  'customers.edit_note(bytea,uuid,uuid,uuid,text,bigint)','customers.delete_note(bytea,uuid,uuid,uuid)',
  'customers.list_tags(bytea,uuid)','customers.list_notes(bytea,uuid,uuid,integer,timestamptz,uuid)'] LOOP
  EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime',f);
 END LOOP;
 -- identity.read_merchant_customers is owned by commerce_auth: it needs the three JSON readers (no table grant for it).
 FOREACH f IN ARRAY ARRAY['customers.tn_tags_json(uuid,uuid,uuid)','customers.tn_tags_revision(uuid,uuid,uuid)',
  'customers.tn_notes_json(uuid,uuid,uuid,integer,timestamptz,uuid)'] LOOP
  EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_auth',f);
 END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------
-- Erasure and export patches of the 0078 definers (0113 pattern: prosrc replace behind a drift guard, OR REPLACE keeps
-- owner and ACL). apply_erasure deletes links and notes first; buyer_read_privacy appends tags+notes to the detail export only.
-- ---------------------------------------------------------------------------------------
DO $patch$
DECLARE body text; needle text:='-- CD4: withdraw every granted pair';
BEGIN
 SELECT prosrc INTO body FROM pg_proc WHERE oid='customers.apply_erasure(uuid,uuid,uuid)'::regprocedure;
 IF position(needle IN body)=0 THEN RAISE EXCEPTION 'erasure baseline drift'; END IF;
 body:=replace(body,needle,'PERFORM customers.erase_tags_notes(p_tenant,p_store,p_owner); '||needle);
 EXECUTE 'CREATE OR REPLACE FUNCTION customers.apply_erasure(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS '||quote_literal(body);
END $patch$;

DO $patch$
DECLARE body text; needle text:=E' RETURN v_result;\nEND';
BEGIN
 SELECT prosrc INTO body FROM pg_proc WHERE oid='customers.buyer_read_privacy(bytea,uuid,boolean)'::regprocedure;
 IF (length(body)-length(replace(body,needle,'')))<>length(needle) THEN RAISE EXCEPTION 'buyer_read_privacy baseline drift'; END IF;
 body:=replace(body,needle,E' IF p_detail THEN v_result:=v_result||customers.export_tags_notes(v_scope.tenant_id,p_store,v_scope.owner_id); END IF;\n RETURN v_result;\nEND');
 EXECUTE 'CREATE OR REPLACE FUNCTION customers.buyer_read_privacy(p_hash bytea,p_store uuid,p_detail boolean) RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS '||quote_literal(body);
END $patch$;

-- ---------------------------------------------------------------------------------------
-- identity.read_merchant_customers, NEW signature (adds p_tag uuid DEFAULT NULL). The body is the 0078 body in full with the
-- lines marked "W6-01B" added; W5-02B extends THIS definition. The 7-argument function is dropped: a call with seven
-- arguments resolves to this one through the default. Same owner and ACL as before (commerce_auth, EXECUTE commerce_runtime).
-- List cap 262144 -> 524288 bytes: a page of 100 customers may now carry up to 20 tags each.
-- ---------------------------------------------------------------------------------------
DROP FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text);
CREATE FUNCTION identity.read_merchant_customers(p_hash bytea,p_store uuid,p_customer uuid,p_limit integer,
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
  SELECT o.owner_id AS id FROM checkout.orders o WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store
   AND (p_customer IS NULL OR o.owner_id=p_customer)
  UNION
  SELECT b.owner_id FROM claims.bundles b WHERE b.tenant_id=s.tenant_id AND b.store_id=p_store AND b.owner_id IS NOT NULL
   AND (p_customer IS NULL OR b.owner_id=p_customer)
 ), base AS MATERIALIZED (
  SELECT ow.tenant_id,ow.store_id,ow.id,ow.created_at AS first_seen,ow.active,
   greatest(
    (SELECT max(o.created_at) FROM checkout.orders o WHERE o.tenant_id=ow.tenant_id AND o.store_id=ow.store_id AND o.owner_id=ow.id),
    (SELECT max(b.bound_at) FROM claims.bundles b WHERE b.tenant_id=ow.tenant_id AND b.store_id=ow.store_id AND b.owner_id=ow.id)
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
      ELSE starts_with(lower(coalesce(qo.snapshot#>>'{destination,recipient_name}','')),lower(p_q)) END))
   -- W6-01B: tag filter. An unknown or other-store tag id simply matches nobody (RLS-free: tenant/store are in the join).
   AND (p_tag IS NULL OR EXISTS(SELECT 1 FROM customers.owner_tags ot WHERE ot.tenant_id=ow.tenant_id AND ot.store_id=ow.store_id
     AND ot.owner_id=ow.id AND ot.tag_id=p_tag))
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
    'display_name',lo.name,
    'phone_last3',lo.last3,
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
ALTER FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) TO commerce_runtime;

COMMENT ON TABLE customers.tags IS 'internal/customers (W6-01B): merchant-typed store tags (<=100 per store, name 1..20 NFC unique case-insensitively, 8 colours). Written only by customers.create_tag/rename_tag/delete_tag; never inferred.';
COMMENT ON TABLE customers.owner_tags IS 'internal/customers (W6-01B): tag links of one customer (<=20). Replaced as a set by customers.set_owner_tags; deleted by erasure (apply_erasure -> erase_tags_notes) and with their tag.';
COMMENT ON TABLE customers.notes IS 'internal/customers (W6-01B): private merchant notes about a customer (<=200, body 1..1000). Buyer personal data: deleted by erasure, included in the buyer self-service export. Audit rows never contain the body.';
COMMENT ON FUNCTION identity.read_merchant_customers(bytea,uuid,uuid,integer,timestamptz,uuid,text,uuid) IS
 'internal/customers (customers:read): list (limit 1..101, keyset on (last_activity_at,id) DESC, optional p_tag filter) or detail (p_customer). Rows carry tags; the detail adds tags_revision and the newest 50 notes. Replaces the 0078 7-argument function (W6-01B); W5-02B extends this body.';
