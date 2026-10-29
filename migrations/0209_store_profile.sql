-- Purpose: store-profile contact fields and CAS with one authoritative control.stores.name.
-- Depends on: control.stores RLS, identity.resolve_access and server-scoped GUCs (0153).
-- Used by: internal/httpapi/profile.go through control.set_store_profile; runtime has no direct UPDATE.
-- Invariants: authenticated scope verified before/after row lock; restricted column privileges; no external effects.
ALTER TABLE control.stores
 ADD COLUMN contact_email text NOT NULL DEFAULT '' CHECK (octet_length(contact_email)<=254 AND contact_email !~ '[[:cntrl:]]'),
 ADD COLUMN contact_phone text NOT NULL DEFAULT '' CHECK (length(contact_phone)<=32 AND contact_phone !~ '[[:cntrl:]]'),
 ADD COLUMN profile_version bigint NOT NULL DEFAULT 0 CHECK (profile_version BETWEEN 0 AND 9007199254740991);

CREATE ROLE commerce_store_profile_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA control,identity TO commerce_store_profile_writer;
GRANT SELECT(tenant_id,id,name,contact_email,contact_phone,profile_version) ON control.stores TO commerce_store_profile_writer;
GRANT UPDATE(name,contact_email,contact_phone,profile_version) ON control.stores TO commerce_store_profile_writer;
GRANT EXECUTE ON FUNCTION identity.resolve_access(bytea,uuid,text) TO commerce_store_profile_writer;
CREATE POLICY stores_profile_read ON control.stores FOR SELECT TO commerce_store_profile_writer
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY stores_profile_update ON control.stores FOR UPDATE TO commerce_store_profile_writer
 USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND id=nullif(current_setting('app.store_id',true),'')::uuid);
COMMENT ON POLICY stores_profile_read ON control.stores IS '0209: scoped profile row lock/read inside authenticated definer.';
COMMENT ON POLICY stores_profile_update ON control.stores IS '0209: scoped profile CAS; writer has only four column UPDATE grants.';

CREATE FUNCTION control.set_store_profile(p_hash bytea,p_store uuid,p_name text,p_email text,p_phone text,p_expected bigint)
RETURNS TABLE(name text,contact_email text,contact_phone text,version bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; fresh record; head bigint;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL THEN
  RAISE EXCEPTION 'invalid request' USING ERRCODE='PT422'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' OR
  current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text OR
  current_setting('app.store_id',true) IS DISTINCT FROM p_store::text OR
  current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF p_name IS NULL OR length(p_name) NOT BETWEEN 1 AND 120 OR p_name !~ '[^[:space:]]' OR p_name ~ '[[:cntrl:]]' OR
  p_email IS NULL OR octet_length(p_email) NOT BETWEEN 3 AND 254 OR p_email !~ '^[^[:space:]<>"@]+@[^[:space:]<>"@]+$' OR p_email ~ '[[:cntrl:]]' OR
  p_phone IS NULL OR length(p_phone)>32 OR p_phone ~ '[[:cntrl:]]' OR
  p_expected IS NULL OR p_expected<0 OR p_expected>=9007199254740991 THEN
  RAISE EXCEPTION 'invalid request' USING ERRCODE='PT422'; END IF;
 SELECT st.profile_version INTO head FROM control.stores st WHERE st.tenant_id=s.tenant_id AND st.id=p_store FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 -- VOLATILE: this statement sees revocation committed while SELECT FOR UPDATE was waiting.
 SELECT * INTO fresh FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF fresh.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF fresh.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF fresh.access_status IS DISTINCT FROM 'ok' OR fresh.tenant_id IS DISTINCT FROM s.tenant_id OR
  fresh.principal_id IS DISTINCT FROM s.principal_id OR fresh.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF head<>p_expected THEN RAISE EXCEPTION 'conflict' USING ERRCODE='PT409'; END IF;
 RETURN QUERY UPDATE control.stores st SET name=p_name,contact_email=p_email,contact_phone=p_phone,profile_version=head+1
  WHERE st.tenant_id=s.tenant_id AND st.id=p_store RETURNING st.name,st.contact_email,st.contact_phone,st.profile_version;
END $$;
ALTER FUNCTION control.set_store_profile(bytea,uuid,text,text,text,bigint) OWNER TO commerce_store_profile_writer;
REVOKE ALL ON FUNCTION control.set_store_profile(bytea,uuid,text,text,text,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.set_store_profile(bytea,uuid,text,text,text,bigint) TO commerce_runtime;
COMMENT ON FUNCTION control.set_store_profile(bytea,uuid,text,text,text,bigint) IS '0209: authenticated profile CAS; caller transaction owns receipt/audit and final permission check. No runtime direct store UPDATE grant.';
