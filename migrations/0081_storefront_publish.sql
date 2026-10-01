-- 0081 storefront publication + domain binding writer (R3 unit storefront-publish; closes R3 readiness gap 1).
-- contracts/published-storefront-resolver-v1.md "Writer (R3)". 0020 created control.storefront_publications and
-- control.storefront_domains with no production writer; until now only tests (as the migration owner) filled them.
--
-- Two separate consents (ruling SP1):
--   * PUBLICATION (the store is visible to buyers) is the MERCHANT's act: control.set_storefront_published /
--     control.read_storefront, EXECUTE commerce_runtime (the API login behind admin Settings), permissions
--     integration:manage / integration:read (the narrowest ones the settings routes already require).
--   * DOMAIN BINDING (ownership + TLS proof) is the PLATFORM OPERATOR's act: control.operator_bind_domain /
--     operator_suspend_domain / operator_detach_domain / operator_storefront_status, EXECUTE commerce_storefront_registrar
--     only (login lc_store_registrar, used by the `ops` one-shot cmd/store-admin). No merchant HTTP route reaches them.
-- The resolver (buyer.resolve_published_store, 0020) is unchanged and has no cache: every write here bumps `version`
-- and the very next resolve sees it, so there is nothing to invalidate.
--
-- Roles: commerce_storefront_writer (NOLOGIN) owns the definers; it is never a login and no application role is a member.
-- commerce_storefront_registrar (NOLOGIN) holds EXECUTE on the operator definers only.

CREATE ROLE commerce_storefront_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_storefront_registrar NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
COMMENT ON ROLE commerce_storefront_writer IS
 '0081 owner of the storefront publication/domain definers (internal/storefrontadmin). NOLOGIN, no members; writes control.storefront_publications / storefront_domains and ops.audit_events only through its fixed functions.';
COMMENT ON ROLE commerce_storefront_registrar IS
 '0081 operator authority: EXECUTE on control.operator_* storefront definers only. Exactly one login (lc_store_registrar, deploy/postgres/logins.tsv) used by the ops one-shot cmd/store-admin, never a long-running service.';

GRANT USAGE ON SCHEMA control, ops, identity TO commerce_storefront_writer;
GRANT USAGE ON SCHEMA control TO commerce_storefront_registrar;
-- The merchant definers verify the bearer through identity.resolve_access (owner commerce_auth); same grant as 0078's writer.
GRANT EXECUTE ON FUNCTION identity.resolve_access(bytea,uuid,text) TO commerce_storefront_writer;

-- Table access of the definer owner (FORCE RLS stays on: each grant has an explicit policy below).
GRANT SELECT ON control.storefront_publications, control.storefront_domains TO commerce_storefront_writer;
GRANT INSERT(tenant_id,store_id,published) ON control.storefront_publications TO commerce_storefront_writer;
GRANT UPDATE(published,version) ON control.storefront_publications TO commerce_storefront_writer;
GRANT INSERT(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
  ON control.storefront_domains TO commerce_storefront_writer;
-- tenant_id/store_id are updatable for exactly one path: operator_bind_domain re-binding a DETACHED origin (integrator ruling).
GRANT UPDATE(tenant_id,store_id,state,version,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
  ON control.storefront_domains TO commerce_storefront_writer;
GRANT SELECT(tenant_id,id,active) ON control.stores TO commerce_storefront_writer;
-- Operator audit attribution: ops.audit_events.principal_id is NOT NULL, an operator has no principal, so the
-- operator definers attribute to the principal that created the store (identity.initial_stores), action prefix operator.*.
GRANT SELECT(principal_id,tenant_id,store_id) ON identity.initial_stores TO commerce_storefront_writer;
GRANT INSERT ON ops.audit_events TO commerce_storefront_writer;

CREATE POLICY storefront_writer_publications_read ON control.storefront_publications FOR SELECT TO commerce_storefront_writer USING (true);
CREATE POLICY storefront_writer_publications_insert ON control.storefront_publications FOR INSERT TO commerce_storefront_writer WITH CHECK (true);
CREATE POLICY storefront_writer_publications_update ON control.storefront_publications FOR UPDATE TO commerce_storefront_writer USING (true) WITH CHECK (true);
CREATE POLICY storefront_writer_domains_read ON control.storefront_domains FOR SELECT TO commerce_storefront_writer USING (true);
CREATE POLICY storefront_writer_domains_insert ON control.storefront_domains FOR INSERT TO commerce_storefront_writer WITH CHECK (true);
CREATE POLICY storefront_writer_domains_update ON control.storefront_domains FOR UPDATE TO commerce_storefront_writer USING (true) WITH CHECK (true);
CREATE POLICY storefront_writer_stores_read ON control.stores FOR SELECT TO commerce_storefront_writer USING (true);
CREATE POLICY storefront_writer_audit_insert ON ops.audit_events FOR INSERT TO commerce_storefront_writer
 WITH CHECK (action IN ('merchant.storefront_published','merchant.storefront_unpublished',
   'operator.domain_bound','operator.domain_bound:rebind_from_detached','operator.domain_suspended','operator.domain_detached'));
COMMENT ON POLICY storefront_writer_publications_read ON control.storefront_publications IS '0081: definer owner read (merchant read/toggle, operator status).';
COMMENT ON POLICY storefront_writer_publications_insert ON control.storefront_publications IS '0081: first publish of a store (set_storefront_published inserts version 1).';
COMMENT ON POLICY storefront_writer_publications_update ON control.storefront_publications IS '0081: publish/unpublish by compare-and-set on version (set_storefront_published).';
COMMENT ON POLICY storefront_writer_domains_read ON control.storefront_domains IS '0081: operator bind/suspend/detach/status read.';
COMMENT ON POLICY storefront_writer_domains_insert ON control.storefront_domains IS '0081: operator_bind_domain inserts a new origin directly ACTIVE with proof.';
COMMENT ON POLICY storefront_writer_domains_update ON control.storefront_domains IS '0081: operator lifecycle moves (bind/renew, suspend, detach); each bumps version.';
COMMENT ON POLICY storefront_writer_stores_read ON control.stores IS '0081: operator definers resolve tenant of a store id (tenant_id,id,active columns only).';
COMMENT ON POLICY storefront_writer_audit_insert ON ops.audit_events IS '0081: the six fixed storefront audit actions only (operator.domain_bound:rebind_from_detached marks a re-bind of a DETACHED origin).';

-- ---------------------------------------------------------------------------------------
-- control.read_storefront: merchant read of the publication state and the store's domains. integration:read.
-- Merchant definers VERIFY the GUCs WithScope set (0078 pattern), never set them.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION control.read_storefront(p_hash bytea,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_pub record; v_domains jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL THEN
  RAISE EXCEPTION 'invalid storefront request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 SELECT p.published,p.version INTO v_pub FROM control.storefront_publications p
  WHERE p.tenant_id=s.tenant_id AND p.store_id=p_store;
 -- Only ACTIVE domains are shown to the merchant; `serving` says the proof window is still open (valid_until).
 SELECT coalesce(jsonb_agg(jsonb_build_object('origin',d.origin,
   'valid_until',to_char(d.valid_until AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
   'serving',d.valid_until>clock_timestamp()) ORDER BY d.origin),'[]'::jsonb) INTO v_domains
  FROM control.storefront_domains d WHERE d.tenant_id=s.tenant_id AND d.store_id=p_store AND d.state='ACTIVE';
 RETURN jsonb_build_object('published',coalesce(v_pub.published,false),'version',coalesce(v_pub.version,0),'domains',v_domains);
END $$;
ALTER FUNCTION control.read_storefront(bytea,uuid) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.read_storefront(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.read_storefront(bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION control.read_storefront(bytea,uuid) IS
 'internal/storefrontadmin Read; EXECUTE commerce_runtime (admin Settings GET storefront). integration:read. Returns {published,version(0 = never published),domains:[{origin,valid_until,serving}]} of ACTIVE domains only; no proof, evidence or PII. Never writes.';

-- ---------------------------------------------------------------------------------------
-- control.set_storefront_published: the merchant toggle (SP1). integration:manage. Compare-and-set on version:
-- expected 0 = no row yet (publish inserts version 1); anything else must equal the stored version (PT409).
-- Setting the state the row already has is a no-op (no version bump, no audit). One audit row per real change.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION control.set_storefront_published(p_hash bytea,p_store uuid,p_published boolean,p_expected bigint)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_row record; v_version bigint; v_changed boolean:=true;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_published IS NULL
  OR p_expected IS NULL OR p_expected<0 OR p_expected>=4611686018427387904 THEN
  RAISE EXCEPTION 'invalid storefront request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- Row lock serializes concurrent toggles of the same store; the first publish races on the primary key instead.
 SELECT p.published,p.version INTO v_row FROM control.storefront_publications p
  WHERE p.tenant_id=s.tenant_id AND p.store_id=p_store FOR UPDATE;
 IF NOT FOUND THEN
  IF p_expected<>0 THEN RAISE EXCEPTION 'version_conflict' USING ERRCODE='PT409'; END IF;
  IF NOT p_published THEN
   v_changed:=false; v_version:=0;                       -- never published and asked to stay unpublished
  ELSE
   BEGIN
    INSERT INTO control.storefront_publications(tenant_id,store_id,published) VALUES(s.tenant_id,p_store,true);
   EXCEPTION WHEN unique_violation THEN RAISE EXCEPTION 'version_conflict' USING ERRCODE='PT409';
   END;
   v_version:=1;
  END IF;
 ELSE
  IF v_row.version<>p_expected THEN RAISE EXCEPTION 'version_conflict' USING ERRCODE='PT409'; END IF;
  IF v_row.published=p_published THEN
   v_changed:=false; v_version:=v_row.version;
  ELSE
   UPDATE control.storefront_publications SET published=p_published,version=version+1
    WHERE tenant_id=s.tenant_id AND store_id=p_store RETURNING version INTO v_version;
  END IF;
 END IF;
 IF v_changed THEN
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
  VALUES(s.tenant_id,p_store,s.principal_id,CASE WHEN p_published THEN 'merchant.storefront_published' ELSE 'merchant.storefront_unpublished' END);
 END IF;
 -- Final authority after every lock wait and write (no post-revocation success).
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF v_final.access_status IS DISTINCT FROM 'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN jsonb_build_object('published',p_published,'version',v_version,'changed',v_changed);
END $$;
ALTER FUNCTION control.set_storefront_published(bytea,uuid,boolean,bigint) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.set_storefront_published(bytea,uuid,boolean,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.set_storefront_published(bytea,uuid,boolean,bigint) TO commerce_runtime;
COMMENT ON FUNCTION control.set_storefront_published(bytea,uuid,boolean,bigint) IS
 'internal/storefrontadmin SetPublished; EXECUTE commerce_runtime (admin Settings POST storefront/publication). integration:manage, compare-and-set on version (PT409 version_conflict), audit merchant.storefront_published|unpublished once per real change. Publishing alone serves nothing: the resolver also needs an ACTIVE domain bound by the operator (operator_bind_domain). Never touches domains.';

-- ---------------------------------------------------------------------------------------
-- Operator definers (commerce_storefront_registrar). The EXECUTE grant is the authority check; there is no bearer.
-- Origin grammar = 0020 CHECK = buyer.resolve_published_store = internal/domains.ValidOrigin (parity tests PR01).
-- ---------------------------------------------------------------------------------------
-- control.operator_bind_domain: creates the origin directly ACTIVE, or advances an existing non-DETACHED row of the SAME
-- store to ACTIVE (also the renewal path: re-binding an ACTIVE origin restamps the proof and bumps version).
-- A DETACHED origin is never re-bound by a plain update. Integrator ruling: it may be re-bound only as a renewed-proof bind
-- inside this definer: evidence_ref must differ from the row's current one (else PT409 domain_detached), both verification
-- stamps and valid_until are renewed, tenant/store become the target's, version bumps and the audit action is
-- operator.domain_bound:rebind_from_detached. A row in any other state belonging to a different store is PT409
-- domain_owned_elsewhere. This is the only path that ever changes store_id.
CREATE FUNCTION control.operator_bind_domain(p_store uuid,p_origin text,p_evidence text,p_valid_until timestamptz)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_principal uuid; v_now timestamptz; v_row record; v_id uuid; v_version bigint; v_renewed boolean:=false; v_rebound boolean:=false;
BEGIN
 IF p_store IS NULL OR p_origin IS NULL OR octet_length(p_origin) NOT BETWEEN 11 AND 261 OR right(p_origin,10)='.localhost'
  OR p_origin COLLATE "C" !~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'
  OR p_evidence IS NULL OR length(btrim(p_evidence)) NOT BETWEEN 1 AND 240 OR length(p_evidence)>240
  OR p_evidence !~ '[^[:space:]]' OR p_evidence ~ '[[:cntrl:]]'
  OR p_valid_until IS NULL OR NOT isfinite(p_valid_until) THEN
  RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
 v_now:=clock_timestamp();
 -- valid_until is the TLS certificate notAfter: it must be in the future and within a public-CA certificate lifetime
 -- (398 days, rounded up to 400) so a typo cannot mint an ACTIVE proof that never expires.
 IF p_valid_until<=v_now OR p_valid_until>v_now+interval '400 days' THEN
  RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=p_store;
 IF NOT FOUND THEN RAISE EXCEPTION 'store not found' USING ERRCODE='PT404'; END IF;
 SELECT i.principal_id INTO v_principal FROM identity.initial_stores i WHERE i.tenant_id=v_tenant AND i.store_id=p_store;
 IF NOT FOUND THEN RAISE EXCEPTION 'store has no owner principal' USING ERRCODE='PT409'; END IF;
 -- One writer per origin at a time (the unique index alone would turn the loser into an error after the proof work).
 PERFORM pg_advisory_xact_lock(hashtextextended('storefront-domain:'||p_origin,0));
 SELECT d.id,d.tenant_id,d.store_id,d.state,d.version,d.evidence_ref INTO v_row FROM control.storefront_domains d WHERE d.origin=p_origin FOR UPDATE;
 IF FOUND THEN
  IF v_row.state='DETACHED' THEN
   -- renewed proof required: the same evidence reference cannot revive a detached origin
   IF v_row.evidence_ref IS NOT DISTINCT FROM p_evidence THEN RAISE EXCEPTION 'domain_detached' USING ERRCODE='PT409'; END IF;
   v_rebound:=true;
  ELSIF v_row.tenant_id<>v_tenant OR v_row.store_id<>p_store THEN
   RAISE EXCEPTION 'domain_owned_elsewhere' USING ERRCODE='PT409';
  END IF;
  v_renewed:=v_row.state='ACTIVE';
  UPDATE control.storefront_domains SET tenant_id=v_tenant,store_id=p_store,state='ACTIVE',version=version+1,ownership_verified_at=v_now,tls_verified_at=v_now,
   valid_until=p_valid_until,evidence_ref=p_evidence WHERE id=v_row.id RETURNING id,version INTO v_id,v_version;
 ELSE
  INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
  VALUES(v_tenant,p_store,p_origin,'ACTIVE',v_now,v_now,p_valid_until,p_evidence) RETURNING id,version INTO v_id,v_version;
 END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(v_tenant,p_store,v_principal,CASE WHEN v_rebound THEN 'operator.domain_bound:rebind_from_detached' ELSE 'operator.domain_bound' END);
 RETURN jsonb_build_object('domain_id',v_id,'version',v_version,'state','ACTIVE','renewed',v_renewed,'rebound',v_rebound);
END $$;

-- control.operator_suspend_domain / operator_detach_domain: by origin. Suspend is reversible (bind again with new proof);
-- detach is final for the origin. Re-applying the target state is a no-op without version bump or audit.
CREATE FUNCTION control.operator_suspend_domain(p_origin text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_row record; v_principal uuid; v_version bigint; v_changed boolean:=true;
BEGIN
 IF p_origin IS NULL OR octet_length(p_origin) NOT BETWEEN 11 AND 261 OR right(p_origin,10)='.localhost'
  OR p_origin COLLATE "C" !~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' THEN
  RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('storefront-domain:'||p_origin,0));
 SELECT d.id,d.tenant_id,d.store_id,d.state,d.version INTO v_row FROM control.storefront_domains d WHERE d.origin=p_origin FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'domain not found' USING ERRCODE='PT404'; END IF;
 IF v_row.state='DETACHED' THEN RAISE EXCEPTION 'domain_detached' USING ERRCODE='PT409'; END IF;
 IF v_row.state='SUSPENDED' THEN
  v_changed:=false; v_version:=v_row.version;
 ELSE
  SELECT i.principal_id INTO v_principal FROM identity.initial_stores i WHERE i.tenant_id=v_row.tenant_id AND i.store_id=v_row.store_id;
  IF NOT FOUND THEN RAISE EXCEPTION 'store has no owner principal' USING ERRCODE='PT409'; END IF;
  UPDATE control.storefront_domains SET state='SUSPENDED',version=version+1 WHERE id=v_row.id RETURNING version INTO v_version;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_row.tenant_id,v_row.store_id,v_principal,'operator.domain_suspended');
 END IF;
 RETURN jsonb_build_object('domain_id',v_row.id,'version',v_version,'state','SUSPENDED','changed',v_changed);
END $$;

CREATE FUNCTION control.operator_detach_domain(p_origin text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_row record; v_principal uuid; v_version bigint; v_changed boolean:=true;
BEGIN
 IF p_origin IS NULL OR octet_length(p_origin) NOT BETWEEN 11 AND 261 OR right(p_origin,10)='.localhost'
  OR p_origin COLLATE "C" !~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' THEN
  RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('storefront-domain:'||p_origin,0));
 SELECT d.id,d.tenant_id,d.store_id,d.state,d.version INTO v_row FROM control.storefront_domains d WHERE d.origin=p_origin FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'domain not found' USING ERRCODE='PT404'; END IF;
 IF v_row.state='DETACHED' THEN
  v_changed:=false; v_version:=v_row.version;
 ELSE
  SELECT i.principal_id INTO v_principal FROM identity.initial_stores i WHERE i.tenant_id=v_row.tenant_id AND i.store_id=v_row.store_id;
  IF NOT FOUND THEN RAISE EXCEPTION 'store has no owner principal' USING ERRCODE='PT409'; END IF;
  UPDATE control.storefront_domains SET state='DETACHED',version=version+1 WHERE id=v_row.id RETURNING version INTO v_version;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(v_row.tenant_id,v_row.store_id,v_principal,'operator.domain_detached');
 END IF;
 RETURN jsonb_build_object('domain_id',v_row.id,'version',v_version,'state','DETACHED','changed',v_changed);
END $$;

-- control.operator_storefront_status: read-only, no PII: publication state and every domain row of the store.
CREATE FUNCTION control.operator_storefront_status(p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_pub record; v_domains jsonb;
BEGIN
 IF p_store IS NULL THEN RAISE EXCEPTION 'invalid storefront request' USING ERRCODE='PT400'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=p_store;
 IF NOT FOUND THEN RAISE EXCEPTION 'store not found' USING ERRCODE='PT404'; END IF;
 SELECT p.published,p.version INTO v_pub FROM control.storefront_publications p WHERE p.tenant_id=v_tenant AND p.store_id=p_store;
 SELECT coalesce(jsonb_agg(jsonb_build_object('origin',d.origin,'state',d.state,'version',d.version,
   'valid_until',CASE WHEN d.valid_until IS NULL THEN NULL ELSE to_char(d.valid_until AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') END,
   'serving',coalesce(d.state='ACTIVE' AND d.valid_until>clock_timestamp(),false)) ORDER BY d.origin),'[]'::jsonb) INTO v_domains
  FROM control.storefront_domains d WHERE d.tenant_id=v_tenant AND d.store_id=p_store;
 RETURN jsonb_build_object('store_id',p_store,'published',coalesce(v_pub.published,false),'version',coalesce(v_pub.version,0),'domains',v_domains);
END $$;

ALTER FUNCTION control.operator_bind_domain(uuid,text,text,timestamptz) OWNER TO commerce_storefront_writer;
ALTER FUNCTION control.operator_suspend_domain(text) OWNER TO commerce_storefront_writer;
ALTER FUNCTION control.operator_detach_domain(text) OWNER TO commerce_storefront_writer;
ALTER FUNCTION control.operator_storefront_status(uuid) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.operator_bind_domain(uuid,text,text,timestamptz) FROM PUBLIC;
REVOKE ALL ON FUNCTION control.operator_suspend_domain(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION control.operator_detach_domain(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION control.operator_storefront_status(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.operator_bind_domain(uuid,text,text,timestamptz) TO commerce_storefront_registrar;
GRANT EXECUTE ON FUNCTION control.operator_suspend_domain(text) TO commerce_storefront_registrar;
GRANT EXECUTE ON FUNCTION control.operator_detach_domain(text) TO commerce_storefront_registrar;
GRANT EXECUTE ON FUNCTION control.operator_storefront_status(uuid) TO commerce_storefront_registrar;
COMMENT ON FUNCTION control.operator_bind_domain(uuid,text,text,timestamptz) IS
 'internal/storefrontadmin BindDomain, cmd/store-admin domain-bind only; EXECUTE commerce_storefront_registrar. Creates the origin ACTIVE or advances/renews a non-DETACHED row of the same store (stamps ownership/tls proof now, valid_until = TLS notAfter, evidence_ref), bumps version, audits operator.domain_bound. A DETACHED origin re-binds only with a different evidence_ref (any store; audit operator.domain_bound:rebind_from_detached, same evidence = PT409 domain_detached); a non-DETACHED origin of another store is PT409; the operator attests the proof, SQL cannot verify DNS/TLS. Never publishes the store (the merchant does).';
COMMENT ON FUNCTION control.operator_suspend_domain(text) IS
 'internal/storefrontadmin SuspendDomain, cmd/store-admin domain-suspend only; EXECUTE commerce_storefront_registrar. Moves a non-DETACHED origin to SUSPENDED (resolver denies on the next request), bumps version, audits operator.domain_suspended; already SUSPENDED is a no-op.';
COMMENT ON FUNCTION control.operator_detach_domain(text) IS
 'internal/storefrontadmin DetachDomain, cmd/store-admin domain-detach only; EXECUTE commerce_storefront_registrar. Final DETACHED for the origin (never re-bound by update), bumps version, audits operator.domain_detached; already DETACHED is a no-op.';
COMMENT ON FUNCTION control.operator_storefront_status(uuid) IS
 'internal/storefrontadmin Status, cmd/store-admin status only; EXECUTE commerce_storefront_registrar. Read-only: publication state and every domain row (origin, state, version, valid_until) of one store; no evidence_ref, proof or PII.';
