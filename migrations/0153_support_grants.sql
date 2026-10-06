-- Purpose: time-boxed, read-only platform support access to ONE store for a designated support principal (identity.platform_support_principals registry + identity.support_grants), the five operator definers that manage them, and the one extra branch in identity.resolve_access / list_session_stores that honours a live grant of a registered, merchant-free principal.
-- Depends on: 0001 (identity.principals/memberships/store_grants, control.stores/tenants, ops.audit_events), 0003 (identity.resolve_access), 0089 (identity.list_session_stores), 0130 (ops.audit_events.details), 0143 (control.operator_audit, roles commerce_platform_writer / commerce_platform_operator).
-- Used by: cmd/platform-admin support-grant|support-revoke|support-list (commerce_platform_operator login), internal/platform withScopeContext (support = negative authz_revision), tests/foundation/support_grant_test.go.
-- Invariants: default-deny (a support grant needs an enabled registry row and a principal with no active membership and no store grant anywhere, checked at grant AND at every request); read-only (CHECK on the permission pack + ':read' guard in resolve_access + read-only transaction in Go); expiry/revocation/suspension evaluated on every request, no sweeper.
-- Status: REAL_PG gate.
-- 0153 platform support grant (R3 unit OPS-02B; docs/delivery/units/ops-02b-support-grant.md "Integrator 裁决"; covers M01 #7, deviation A8).
--
-- A support principal logs in with the ordinary merchant login (no new login method, no impersonation). The platform operator CLI
-- (cmd/platform-admin) gives that principal limited-time READ access to one store; every grant and revoke writes the append-only
-- control.operator_audit (action support_grant/support_revoke) AND the merchant-visible ops.audit_events (support.granted /
-- support.revoked; details = a fixed label and the expiry only, the ticket and operator label stay in control.operator_audit). Use is audited by the Go scope layer (support.used, at most one row per principal+store per 15 minutes).
--
-- Designated support principals (review OPS-02B P1-1): only a principal enrolled by the operator in
-- identity.platform_support_principals (add_support_principal / revoke_support_principal, audited in control.operator_audit with a NULL
-- tenant) can be granted. A support identity is separate from merchant identities: a principal with ANY active membership or ANY
-- store_grants row is refused at registration, at grant time and at every request (identity.support_principal_ok). Revoking the registry
-- row closes every open grant of that principal and ends access at once. Gaining regular access (store_grants insert, membership made
-- active: staff_accept) revokes the registry row and the grants by trigger, so a later staff_remove (which deletes store_grants) cannot revive
-- an old grant.
--
-- resolve_access change (signature, owner commerce_auth and EXECUTE grants are unchanged, CREATE OR REPLACE): copy of the 0003 body plus ONE
-- branch. The regular branch is byte-for-byte the old logic. The support branch runs only when the regular lookup found nothing AND the
-- principal is an enabled, merchant-free registry principal; it requires an unrevoked, unexpired (clock_timestamp()) grant on an active
-- store of an active tenant, the permission must end in ':read' and be inside the grant. It returns authz_revision = -1: memberships
-- CHECK authz_revision > 0, so a negative revision can only come from this branch, which is how internal/platform recognises a support
-- scope without a signature change. Definers that independently re-join an active membership (e.g. the 0073 fences) refuse a support
-- principal: that is the intended default-deny.

CREATE TABLE identity.platform_support_principals (
  principal_id uuid PRIMARY KEY REFERENCES identity.principals(id),
  added_by text NOT NULL CHECK (added_by ~ '^[a-z0-9._-]{1,40}$'),
  db_user text NOT NULL,
  ticket text NOT NULL CHECK (length(ticket) BETWEEN 1 AND 80 AND ticket !~ '[[:cntrl:]]' AND ticket ~ '[^[:space:]]'),
  added_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  revoked_at timestamptz
);
ALTER TABLE identity.platform_support_principals ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.platform_support_principals FORCE ROW LEVEL SECURITY;
REVOKE ALL ON identity.platform_support_principals FROM PUBLIC;
GRANT SELECT, INSERT ON identity.platform_support_principals TO commerce_platform_writer;
GRANT UPDATE (added_by, db_user, ticket, added_at, revoked_at) ON identity.platform_support_principals TO commerce_platform_writer;
GRANT SELECT (principal_id, revoked_at) ON identity.platform_support_principals TO commerce_auth;
CREATE POLICY support_principals_platform_read ON identity.platform_support_principals FOR SELECT TO commerce_platform_writer USING (true);
CREATE POLICY support_principals_platform_insert ON identity.platform_support_principals FOR INSERT TO commerce_platform_writer WITH CHECK (true);
CREATE POLICY support_principals_platform_update ON identity.platform_support_principals FOR UPDATE TO commerce_platform_writer USING (true) WITH CHECK (true);
CREATE POLICY support_principals_auth_read ON identity.platform_support_principals FOR SELECT TO commerce_auth USING (true);
COMMENT ON TABLE identity.platform_support_principals IS
 '0153 registry of designated platform support principals (operator-managed through identity.add_support_principal / revoke_support_principal; revoked_at NULL = enabled). A support grant, a resolve_access support branch and a list_session_stores support arm all require an enabled row. Disabled automatically when the principal gains regular store access.';
COMMENT ON POLICY support_principals_platform_read ON identity.platform_support_principals IS '0153: operator definers read the registry.';
COMMENT ON POLICY support_principals_platform_insert ON identity.platform_support_principals IS '0153: add_support_principal enrols a principal.';
COMMENT ON POLICY support_principals_platform_update ON identity.platform_support_principals IS '0153: revoke / re-enrol / automatic revoke (column grants only).';
COMMENT ON POLICY support_principals_auth_read ON identity.platform_support_principals IS '0153: support_principal_ok (commerce_auth) reads principal_id, revoked_at.';

CREATE TABLE identity.support_grants (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid NOT NULL,
  store_id uuid NOT NULL,
  principal_id uuid NOT NULL,
  permissions text[] NOT NULL CHECK (cardinality(permissions) >= 1 AND 'store:read' = ANY(permissions)
    AND permissions <@ ARRAY['store:read','orders:read','catalog:read','inventory:read','live:read','integration:read']::text[]),
  granted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  operator text NOT NULL CHECK (operator ~ '^[a-z0-9._-]{1,40}$'),
  db_user text NOT NULL,
  ticket text NOT NULL CHECK (length(ticket) BETWEEN 1 AND 80 AND ticket !~ '[[:cntrl:]]' AND ticket ~ '[^[:space:]]'),
  CHECK (expires_at > granted_at AND expires_at <= granted_at + interval '72 hours'),
  CHECK (revoked_at IS NULL OR revoked_at >= granted_at),
  FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id),
  FOREIGN KEY (tenant_id, principal_id) REFERENCES identity.memberships(tenant_id, principal_id)
);
CREATE UNIQUE INDEX support_grants_one_open ON identity.support_grants(store_id, principal_id) WHERE revoked_at IS NULL;
CREATE INDEX support_grants_principal_open ON identity.support_grants(principal_id) WHERE revoked_at IS NULL;
ALTER TABLE identity.support_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.support_grants FORCE ROW LEVEL SECURITY;
REVOKE ALL ON identity.support_grants FROM PUBLIC;
-- The definer owner is the only writer (revoked_at is the only column it may change). commerce_auth (owner of resolve_access /
-- list_session_stores) reads the six columns the access decision needs, nothing identifying the operator.
GRANT SELECT, INSERT ON identity.support_grants TO commerce_platform_writer;
GRANT UPDATE (revoked_at) ON identity.support_grants TO commerce_platform_writer;
GRANT SELECT (tenant_id, store_id, principal_id, permissions, expires_at, revoked_at) ON identity.support_grants TO commerce_auth;
CREATE POLICY support_grants_platform_read ON identity.support_grants FOR SELECT TO commerce_platform_writer USING (true);
CREATE POLICY support_grants_platform_insert ON identity.support_grants FOR INSERT TO commerce_platform_writer WITH CHECK (true);
CREATE POLICY support_grants_platform_update ON identity.support_grants FOR UPDATE TO commerce_platform_writer USING (true) WITH CHECK (true);
CREATE POLICY support_grants_auth_read ON identity.support_grants FOR SELECT TO commerce_auth USING (true);
COMMENT ON TABLE identity.support_grants IS
 '0153 platform support access: one row per (store, principal) grant, read-only permission pack (CHECK), at most 72 h (CHECK), at most one unrevoked row per (store, principal). Written only by identity.grant_support / revoke_support (commerce_platform_writer); read by resolve_access / list_session_stores (commerce_auth, six columns). Expiry is judged per request against clock_timestamp(); there is no sweeper.';
COMMENT ON POLICY support_grants_platform_read ON identity.support_grants IS '0153: operator definers read grants.';
COMMENT ON POLICY support_grants_platform_insert ON identity.support_grants IS '0153: grant_support inserts one row per grant.';
COMMENT ON POLICY support_grants_platform_update ON identity.support_grants IS '0153: revoke_support / grant_support close a grant (column grant UPDATE(revoked_at) only).';
COMMENT ON POLICY support_grants_auth_read ON identity.support_grants IS '0153: resolve_access and list_session_stores evaluate live grants.';

-- ---------------------------------------------------------------------------------------
-- Operator audit: four new actions. support_grant / support_revoke are store-scoped (tenant + store); the two support_principal_*
-- actions are platform-level (no tenant, no store: tenant_id becomes nullable and the scope CHECKs are explicit, by name).
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE c record;
BEGIN
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='control.operator_audit'::regclass AND contype='c'
   AND pg_get_constraintdef(oid) LIKE '%action%' LOOP
  EXECUTE format('ALTER TABLE control.operator_audit DROP CONSTRAINT %I', c.conname);
 END LOOP;
END $$;
ALTER TABLE control.operator_audit ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE control.operator_audit ADD CONSTRAINT operator_audit_action_check
 CHECK (action IN ('store_suspend','store_resume','tenant_suspend','tenant_resume','support_grant','support_revoke','support_principal_add','support_principal_revoke'));
ALTER TABLE control.operator_audit ADD CONSTRAINT operator_audit_store_scope_check
 CHECK ((action IN ('store_suspend','store_resume','support_grant','support_revoke')) = (store_id IS NOT NULL));
ALTER TABLE control.operator_audit ADD CONSTRAINT operator_audit_tenant_scope_check
 CHECK ((action IN ('support_principal_add','support_principal_revoke')) = (tenant_id IS NULL));

-- ---------------------------------------------------------------------------------------
-- Table access of the definer owner (all narrow; no RLS on identity.* except support_grants, ops.audit_events needs a policy).
-- ---------------------------------------------------------------------------------------
GRANT USAGE ON SCHEMA identity, ops TO commerce_platform_writer;
GRANT SELECT (id, active) ON identity.principals TO commerce_platform_writer;
GRANT SELECT (tenant_id, principal_id, active), INSERT (tenant_id, principal_id, active) ON identity.memberships TO commerce_platform_writer;
GRANT SELECT (tenant_id, store_id, principal_id, permission) ON identity.store_grants TO commerce_platform_writer;
GRANT INSERT ON ops.audit_events TO commerce_platform_writer;
CREATE POLICY support_audit_insert ON ops.audit_events FOR INSERT TO commerce_platform_writer
 WITH CHECK (action IN ('support.granted','support.revoked'));
COMMENT ON POLICY support_audit_insert ON ops.audit_events IS '0153: the two merchant-visible support audit actions written by grant_support / revoke_support.';

-- ---------------------------------------------------------------------------------------
-- Operator definers. EXECUTE grant = authority; the operator identity is session_user, --operator is a label (as in 0143).
-- ---------------------------------------------------------------------------------------
-- One predicate for "this principal may use a support grant": enabled registry row, and a merchant-free identity (no active
-- membership in any tenant, no store_grants row anywhere). Owner commerce_auth (reads the three tables column-narrow);
-- used by resolve_access / list_session_stores (same owner) and by grant_support / add_support_principal (EXECUTE
-- commerce_platform_writer only). Only evaluated for a principal that already holds a live grant, so it is off the hot path.
CREATE FUNCTION identity.support_principal_ok(p_principal uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT EXISTS (SELECT 1 FROM identity.platform_support_principals r WHERE r.principal_id=p_principal AND r.revoked_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM identity.memberships m WHERE m.principal_id=p_principal AND m.active)
  AND NOT EXISTS (SELECT 1 FROM identity.store_grants g WHERE g.principal_id=p_principal)
$$;

-- Registry enrolment. Refuses an unknown/inactive principal (PT404) and one that holds regular merchant access (PT409).
-- Idempotent: an enabled row is left alone (changed=false); a revoked row is re-enabled. Every call is audited (no tenant).
CREATE FUNCTION identity.add_support_principal(p_principal uuid,p_operator text,p_ticket text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_active boolean; v_rows integer;
BEGIN
 IF p_principal IS NULL OR p_operator IS NULL OR p_operator COLLATE "C" !~ '^[a-z0-9._-]{1,40}$'
  OR p_ticket IS NULL OR length(p_ticket) NOT BETWEEN 1 AND 80 OR p_ticket ~ '[[:cntrl:]]' OR p_ticket !~ '[^[:space:]]' THEN
  RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 SELECT p.active INTO v_active FROM identity.principals p WHERE p.id=p_principal;
 IF NOT FOUND OR NOT v_active THEN RAISE EXCEPTION 'principal not found' USING ERRCODE='PT404'; END IF;
 IF EXISTS (SELECT 1 FROM identity.memberships m WHERE m.principal_id=p_principal AND m.active)
  OR EXISTS (SELECT 1 FROM identity.store_grants g WHERE g.principal_id=p_principal) THEN
  RAISE EXCEPTION 'principal has regular merchant access' USING ERRCODE='PT409'; END IF;
 INSERT INTO identity.platform_support_principals AS r(principal_id,added_by,db_user,ticket)
 VALUES(p_principal,p_operator,session_user,p_ticket)
 ON CONFLICT (principal_id) DO UPDATE SET added_by=EXCLUDED.added_by,db_user=EXCLUDED.db_user,ticket=EXCLUDED.ticket,added_at=clock_timestamp(),revoked_at=NULL
  WHERE r.revoked_at IS NOT NULL;
 GET DIAGNOSTICS v_rows = ROW_COUNT;
 IF v_rows=1 THEN PERFORM identity.support_close_grants(p_principal,'re_enrolled'); END IF; -- defence in depth: a (re-)enrolment never inherits an old open grant
 INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket,reason,detail)
 VALUES(p_operator,session_user,'support_principal_add',NULL,NULL,p_ticket,NULL,jsonb_build_object('changed',v_rows=1,'principal_id',p_principal));
 RETURN jsonb_build_object('principal_id',p_principal,'changed',v_rows=1,'result',CASE WHEN v_rows=1 THEN 'registered' ELSE 'unchanged' END);
END $$;

-- Closes every open grant of a principal (merchant-visible support.revoked per grant, fixed label + reason code). Internal:
-- no caller EXECUTE; used by revoke_support_principal and the regular-access trigger. Returns the number of grants closed.
CREATE FUNCTION identity.support_close_grants(p_principal uuid,p_reason text) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_n integer;
BEGIN
 WITH c AS (UPDATE identity.support_grants g SET revoked_at=clock_timestamp() WHERE g.principal_id=p_principal AND g.revoked_at IS NULL
   RETURNING g.id,g.tenant_id,g.store_id,g.principal_id),
 a AS (INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
   SELECT c.tenant_id,c.store_id,c.principal_id,'support.revoked',jsonb_build_object('label','平台支援','grant_id',c.id,'reason',p_reason) FROM c RETURNING 1)
 SELECT count(*) INTO v_n FROM c;
 RETURN v_n;
END $$;

-- Registry revocation: access, store list and every open grant of the principal end now (resolve_access also re-checks the row
-- on every request). Idempotent; audited either way. A never-enrolled principal is PT404.
CREATE FUNCTION identity.revoke_support_principal(p_principal uuid,p_operator text,p_ticket text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_was timestamptz; v_closed integer := 0; v_changed boolean := false;
BEGIN
 IF p_principal IS NULL OR p_operator IS NULL OR p_operator COLLATE "C" !~ '^[a-z0-9._-]{1,40}$'
  OR p_ticket IS NULL OR length(p_ticket) NOT BETWEEN 1 AND 80 OR p_ticket ~ '[[:cntrl:]]' OR p_ticket !~ '[^[:space:]]' THEN
  RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 SELECT r.revoked_at INTO v_was FROM identity.platform_support_principals r WHERE r.principal_id=p_principal FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'support principal not found' USING ERRCODE='PT404'; END IF;
 IF v_was IS NULL THEN
  UPDATE identity.platform_support_principals SET revoked_at=clock_timestamp() WHERE principal_id=p_principal;
  v_changed := true;
 END IF;
 v_closed := identity.support_close_grants(p_principal,'registry_revoked'); -- also sweeps rows left open by an earlier partial state
 INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket,reason,detail)
 VALUES(p_operator,session_user,'support_principal_revoke',NULL,NULL,p_ticket,NULL,
  jsonb_build_object('changed',v_changed,'principal_id',p_principal,'closed_grants',v_closed));
 RETURN jsonb_build_object('principal_id',p_principal,'changed',v_changed,'closed_grants',v_closed,'result',CASE WHEN v_changed THEN 'revoked' ELSE 'unchanged' END);
END $$;

-- Trigger body: a registered support principal that gains regular merchant access (a store_grants row, or a membership made
-- active: staff_accept) is disenrolled and loses every grant. Without it a later staff_remove (which deletes the store_grants
-- rows) would make an old open grant usable again. Cheap: one PK probe of the registry per store_grants insert.
CREATE FUNCTION identity.support_regular_access_trigger() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_closed integer;
BEGIN
 UPDATE identity.platform_support_principals SET revoked_at=clock_timestamp() WHERE principal_id=NEW.principal_id AND revoked_at IS NULL;
 IF FOUND THEN
  v_closed := identity.support_close_grants(NEW.principal_id,'regular_access');
  INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket,reason,detail)
  VALUES('system',session_user,'support_principal_revoke',NULL,NULL,'regular-access',NULL,
   jsonb_build_object('changed',true,'principal_id',NEW.principal_id,'closed_grants',v_closed,'automatic',true));
 END IF;
 RETURN NULL;
END $$;

CREATE FUNCTION identity.grant_support(p_store uuid,p_principal uuid,p_hours integer,p_perms text[],p_operator text,p_ticket text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 v_pack constant text[] := ARRAY['store:read','orders:read','catalog:read','inventory:read','live:read','integration:read'];
 v_perms text[]; v_tenant uuid; v_store_active boolean; v_tenant_active boolean; v_principal_active boolean;
 v_now timestamptz; v_exp timestamptz; v_id uuid; v_old uuid; v_old_exp timestamptz;
BEGIN
 v_perms := coalesce(p_perms, v_pack);
 IF p_store IS NULL OR p_principal IS NULL OR p_hours IS NULL OR p_hours NOT BETWEEN 1 AND 72
  OR p_operator IS NULL OR p_operator COLLATE "C" !~ '^[a-z0-9._-]{1,40}$'
  OR p_ticket IS NULL OR length(p_ticket) NOT BETWEEN 1 AND 80 OR p_ticket ~ '[[:cntrl:]]' OR p_ticket !~ '[^[:space:]]'
  OR cardinality(v_perms) < 1 OR NOT (v_perms <@ v_pack) OR NOT ('store:read' = ANY(v_perms)) THEN
  RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 SELECT array_agg(DISTINCT x ORDER BY x) INTO v_perms FROM unnest(v_perms) x;
 -- The store row lock serialises grant/revoke/suspend of one store.
 SELECT s.tenant_id,s.active,t.active INTO v_tenant,v_store_active,v_tenant_active
  FROM control.stores s JOIN control.tenants t ON t.id=s.tenant_id WHERE s.id=p_store FOR UPDATE OF s;
 IF NOT FOUND THEN RAISE EXCEPTION 'store not found' USING ERRCODE='PT404'; END IF;
 IF NOT (v_store_active AND v_tenant_active) THEN RAISE EXCEPTION 'store not serving' USING ERRCODE='PT409'; END IF;
 SELECT p.active INTO v_principal_active FROM identity.principals p WHERE p.id=p_principal;
 IF NOT FOUND OR NOT v_principal_active THEN RAISE EXCEPTION 'principal not found' USING ERRCODE='PT404'; END IF;
 -- Designated + separate: only an enabled registry principal that holds no active membership and no store grant anywhere.
 -- The registry row is share-locked first: a concurrent revoke_support_principal / regular-access trigger (row UPDATE) waits for
 -- this grant to commit and then closes it, so no open grant can outlive a revocation and revive on re-enrolment.
 PERFORM 1 FROM identity.platform_support_principals r WHERE r.principal_id=p_principal AND r.revoked_at IS NULL FOR SHARE;
 IF NOT identity.support_principal_ok(p_principal) THEN
  RAISE EXCEPTION 'principal is not an eligible platform support principal' USING ERRCODE='PT409'; END IF;
 v_now := clock_timestamp();
 v_exp := v_now + make_interval(hours => p_hours);
 SELECT g.id,g.expires_at INTO v_old,v_old_exp FROM identity.support_grants g
  WHERE g.store_id=p_store AND g.principal_id=p_principal AND g.revoked_at IS NULL FOR UPDATE;
 IF FOUND THEN
  IF v_old_exp > v_now THEN RAISE EXCEPTION 'support grant already open' USING ERRCODE='PT409'; END IF;
  UPDATE identity.support_grants SET revoked_at=v_old_exp WHERE id=v_old; -- an expired, never-revoked row ended at its expiry
 END IF;
 -- Placeholder membership (active=false: never satisfies any regular active-membership join) only so ops.audit_events and
 -- support_grants can reference (tenant, principal). An existing membership is left exactly as it is.
 INSERT INTO identity.memberships(tenant_id,principal_id,active) VALUES(v_tenant,p_principal,false) ON CONFLICT DO NOTHING;
 INSERT INTO identity.support_grants(tenant_id,store_id,principal_id,permissions,granted_at,expires_at,operator,db_user,ticket)
 VALUES(v_tenant,p_store,p_principal,v_perms,v_now,v_exp,p_operator,session_user,p_ticket) RETURNING id INTO v_id;
 INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket,reason,detail)
 VALUES(p_operator,session_user,'support_grant',v_tenant,p_store,p_ticket,NULL,
  jsonb_build_object('changed',true,'grant_id',v_id,'principal_id',p_principal,'expires_at',v_exp,'permissions',v_perms));
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
 VALUES(v_tenant,p_store,p_principal,'support.granted',
  jsonb_build_object('label','平台支援','grant_id',v_id,'expires_at',v_exp)); -- ticket + operator label stay in control.operator_audit
 RETURN jsonb_build_object('grant_id',v_id,'tenant_id',v_tenant,'store_id',p_store,'principal_id',p_principal,
  'permissions',v_perms,'granted_at',v_now,'expires_at',v_exp,'result','granted');
END $$;

-- Revoke by grant id OR by (store, principal). Idempotent: nothing open = result unchanged, still audited in control.operator_audit.
CREATE FUNCTION identity.revoke_support(p_grant uuid,p_store uuid,p_principal uuid,p_operator text,p_ticket text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 v_tenant uuid; v_store uuid; v_principal uuid; v_id uuid; v_revoked timestamptz; v_changed boolean := false;
BEGIN
 IF p_operator IS NULL OR p_operator COLLATE "C" !~ '^[a-z0-9._-]{1,40}$'
  OR p_ticket IS NULL OR length(p_ticket) NOT BETWEEN 1 AND 80 OR p_ticket ~ '[[:cntrl:]]' OR p_ticket !~ '[^[:space:]]'
  OR NOT ((p_grant IS NOT NULL AND p_store IS NULL AND p_principal IS NULL) OR (p_grant IS NULL AND p_store IS NOT NULL AND p_principal IS NOT NULL)) THEN
  RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 IF p_grant IS NOT NULL THEN
  SELECT g.tenant_id,g.store_id,g.principal_id,g.id,g.revoked_at INTO v_tenant,v_store,v_principal,v_id,v_revoked
   FROM identity.support_grants g WHERE g.id=p_grant FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'support grant not found' USING ERRCODE='PT404'; END IF;
 ELSE
  SELECT s.tenant_id,s.id INTO v_tenant,v_store FROM control.stores s WHERE s.id=p_store FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'store not found' USING ERRCODE='PT404'; END IF;
  v_principal := p_principal;
  SELECT g.id,g.revoked_at INTO v_id,v_revoked FROM identity.support_grants g
   WHERE g.store_id=p_store AND g.principal_id=p_principal AND g.revoked_at IS NULL FOR UPDATE;
 END IF;
 IF v_id IS NOT NULL AND v_revoked IS NULL THEN
  UPDATE identity.support_grants SET revoked_at=clock_timestamp() WHERE id=v_id;
  v_changed := true;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
  VALUES(v_tenant,v_store,v_principal,'support.revoked',jsonb_build_object('label','平台支援','grant_id',v_id,'reason','operator'));
 END IF;
 INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket,reason,detail)
 VALUES(p_operator,session_user,'support_revoke',v_tenant,v_store,p_ticket,NULL,
  jsonb_build_object('changed',v_changed,'grant_id',v_id,'principal_id',v_principal));
 RETURN jsonb_build_object('grant_id',v_id,'store_id',v_store,'principal_id',v_principal,'changed',v_changed,
  'result',CASE WHEN v_changed THEN 'revoked' ELSE 'unchanged' END);
END $$;

-- Read-only: the newest 50 grants of one store (any state) with a derived status. No merchant or buyer data.
CREATE FUNCTION identity.list_support_grants(p_store uuid)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_store IS NULL THEN RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 PERFORM 1 FROM control.stores s WHERE s.id=p_store;
 IF NOT FOUND THEN RAISE EXCEPTION 'store not found' USING ERRCODE='PT404'; END IF;
 RETURN coalesce((SELECT jsonb_agg(jsonb_build_object('id',q.id,'principal_id',q.principal_id,'permissions',q.permissions,
   'granted_at',q.granted_at,'expires_at',q.expires_at,'revoked_at',q.revoked_at,'operator',q.operator,'ticket',q.ticket,
   'status',CASE WHEN q.revoked_at IS NOT NULL THEN 'revoked' WHEN q.expires_at<=clock_timestamp() THEN 'expired' ELSE 'active' END)
   ORDER BY q.granted_at DESC,q.id)
  FROM (SELECT g.* FROM identity.support_grants g WHERE g.store_id=p_store ORDER BY g.granted_at DESC,g.id LIMIT 50) q),'[]'::jsonb);
END $$;

ALTER FUNCTION identity.grant_support(uuid,uuid,integer,text[],text,text) OWNER TO commerce_platform_writer;
ALTER FUNCTION identity.revoke_support(uuid,uuid,uuid,text,text) OWNER TO commerce_platform_writer;
ALTER FUNCTION identity.list_support_grants(uuid) OWNER TO commerce_platform_writer;
ALTER FUNCTION identity.add_support_principal(uuid,text,text) OWNER TO commerce_platform_writer;
ALTER FUNCTION identity.revoke_support_principal(uuid,text,text) OWNER TO commerce_platform_writer;
ALTER FUNCTION identity.support_close_grants(uuid,text) OWNER TO commerce_platform_writer;
ALTER FUNCTION identity.support_regular_access_trigger() OWNER TO commerce_platform_writer;
ALTER FUNCTION identity.support_principal_ok(uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.support_principal_ok(uuid), identity.support_close_grants(uuid,text), identity.support_regular_access_trigger() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.support_principal_ok(uuid) TO commerce_platform_writer;
REVOKE ALL ON FUNCTION identity.grant_support(uuid,uuid,integer,text[],text,text), identity.revoke_support(uuid,uuid,uuid,text,text),
 identity.list_support_grants(uuid), identity.add_support_principal(uuid,text,text), identity.revoke_support_principal(uuid,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.grant_support(uuid,uuid,integer,text[],text,text), identity.revoke_support(uuid,uuid,uuid,text,text),
 identity.list_support_grants(uuid), identity.add_support_principal(uuid,text,text), identity.revoke_support_principal(uuid,text,text) TO commerce_platform_operator;
CREATE TRIGGER support_regular_access_grants AFTER INSERT ON identity.store_grants
 FOR EACH ROW EXECUTE FUNCTION identity.support_regular_access_trigger();
CREATE TRIGGER support_regular_access_membership AFTER INSERT OR UPDATE OF active ON identity.memberships
 FOR EACH ROW WHEN (NEW.active) EXECUTE FUNCTION identity.support_regular_access_trigger();
GRANT USAGE ON SCHEMA identity TO commerce_platform_operator;
COMMENT ON FUNCTION identity.support_principal_ok(uuid) IS
 '0153 internal (owner commerce_auth; EXECUTE commerce_platform_writer): enabled registry row AND no active membership AND no store_grants row anywhere. Used by grant_support, resolve_access and list_session_stores.';
COMMENT ON FUNCTION identity.add_support_principal(uuid,text,text) IS
 'cmd/platform-admin support-principal-add only; EXECUTE commerce_platform_operator. Enrols (or re-enables) a designated platform support principal; refuses an unknown principal (PT404) and one with an active membership or any store grant (PT409). Idempotent; audited in control.operator_audit (support_principal_add, no tenant).';
COMMENT ON FUNCTION identity.revoke_support_principal(uuid,text,text) IS
 'cmd/platform-admin support-principal-revoke only; EXECUTE commerce_platform_operator. Disables the registry row and closes every open grant of that principal (merchant-visible support.revoked each); idempotent; audited (support_principal_revoke).';
COMMENT ON FUNCTION identity.support_close_grants(uuid,text) IS
 '0153 internal (owner commerce_platform_writer, no caller EXECUTE): closes all open grants of a principal and writes the merchant-visible support.revoked rows.';
COMMENT ON FUNCTION identity.support_regular_access_trigger() IS
 '0153 trigger body (store_grants insert, membership made active): a registered support principal that gains regular merchant access is disenrolled and loses every grant, so a later staff_remove cannot revive them.';
COMMENT ON FUNCTION identity.grant_support(uuid,uuid,integer,text[],text,text) IS
 'cmd/platform-admin support-grant only; EXECUTE commerce_platform_operator. Opens a 1..72 h read-only grant (default pack store/orders/catalog/inventory/live/integration :read; never customers:*, inbox:*, payments:*, any write) for an enabled registry support principal with no regular merchant access anywhere; refuses a suspended store/tenant, an unknown or unregistered principal, a second open grant. Inserts an inactive placeholder membership when needed; appends control.operator_audit (support_grant) and ops.audit_events (support.granted) in the same transaction.';
COMMENT ON FUNCTION identity.revoke_support(uuid,uuid,uuid,text,text) IS
 'cmd/platform-admin support-revoke only; EXECUTE commerce_platform_operator. Closes the grant by id or by (store, principal): immediate, because resolve_access reads the row on every request. Idempotent (unchanged); appends control.operator_audit (support_revoke) and, when a grant was closed, ops.audit_events (support.revoked).';
COMMENT ON FUNCTION identity.list_support_grants(uuid) IS
 'cmd/platform-admin support-list only; EXECUTE commerce_platform_operator. Read-only: newest 50 grants of one store with derived status active|expired|revoked.';

-- ---------------------------------------------------------------------------------------
-- identity.resolve_access: 0003 body + the support branch (see header). Signature, owner and EXECUTE grants unchanged.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION identity.resolve_access(p_hash bytea, p_store uuid, p_permission text)
RETURNS TABLE (access_status text, tenant_id uuid, principal_id uuid, authz_revision bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    v_principal uuid;
    v_tenant uuid;
    v_revision bigint;
    v_support_perms text[];
BEGIN
    SELECT p.id INTO v_principal
    FROM identity.sessions AS login
    JOIN identity.principals AS p ON p.id = login.principal_id AND p.active
    WHERE login.token_hash = p_hash AND login.audience = 'merchant'
      AND login.revoked_at IS NULL AND login.expires_at > statement_timestamp();
    IF NOT FOUND THEN
        RETURN QUERY SELECT 'unauthorized'::text, NULL::uuid, NULL::uuid, NULL::bigint;
        RETURN;
    END IF;

    SELECT s.tenant_id, m.authz_revision INTO v_tenant, v_revision
    FROM control.stores AS s
    JOIN control.tenants AS t ON t.id = s.tenant_id AND t.active
    JOIN identity.memberships AS m ON m.tenant_id = s.tenant_id
      AND m.principal_id = v_principal AND m.active
    JOIN identity.store_grants AS g ON g.tenant_id = s.tenant_id
      AND g.store_id = s.id AND g.principal_id = v_principal AND g.permission = 'store:read'
    WHERE s.id = p_store AND s.active;
    IF NOT FOUND THEN
        -- 0153 support branch. Default-deny: only a live grant (unrevoked, unexpired by the statement-independent clock, store and
        -- tenant active) of a principal that is an enabled, merchant-free registry principal (no active membership, no store grant
        -- anywhere: identity.support_principal_ok), so a member or former member never gets support access. Read-only twice
        -- over: the grant CHECK holds only ':read' permissions and the requested permission must be one of them (a NULL
        -- permission is forbidden, like the regular branch).
        SELECT g.tenant_id, g.permissions INTO v_tenant, v_support_perms
        FROM identity.support_grants AS g
        JOIN control.stores AS s ON s.tenant_id = g.tenant_id AND s.id = g.store_id AND s.active
        JOIN control.tenants AS t ON t.id = s.tenant_id AND t.active
        WHERE g.store_id = p_store AND g.principal_id = v_principal
          AND g.revoked_at IS NULL AND g.expires_at > clock_timestamp();
        IF NOT FOUND OR NOT identity.support_principal_ok(v_principal) THEN
            RETURN QUERY SELECT 'not_found'::text, NULL::uuid, NULL::uuid, NULL::bigint;
            RETURN;
        END IF;
        IF NOT coalesce(p_permission ~ ':read$' AND p_permission = ANY(v_support_perms), false) THEN
            RETURN QUERY SELECT 'forbidden'::text, NULL::uuid, NULL::uuid, NULL::bigint;
            RETURN;
        END IF;
        -- authz_revision -1 marks a support scope (memberships.authz_revision > 0 always); see internal/platform.
        RETURN QUERY SELECT 'ok'::text, v_tenant, v_principal, (-1)::bigint;
        RETURN;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM identity.store_grants AS g
        WHERE g.tenant_id = v_tenant AND g.store_id = p_store
          AND g.principal_id = v_principal AND g.permission = p_permission) THEN
        RETURN QUERY SELECT 'forbidden'::text, NULL::uuid, NULL::uuid, NULL::bigint;
        RETURN;
    END IF;
    RETURN QUERY SELECT 'ok'::text, v_tenant, v_principal, v_revision;
END;
$$;

-- ---------------------------------------------------------------------------------------
-- identity.list_session_stores: 0089 body + the same live-grant predicate, so a support principal can pick the store.
-- Role 'support' and the grant's permissions; only for an enabled, merchant-free registry principal (support_principal_ok).
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION identity.list_session_stores(p_hash bytea)
RETURNS TABLE (id uuid, name text, currency text, role text, permissions text[])
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    v_principal uuid;
BEGIN
    SELECT p.id INTO v_principal
    FROM identity.sessions AS login
    JOIN identity.principals AS p ON p.id = login.principal_id AND p.active
    WHERE login.token_hash = p_hash AND login.audience = 'merchant'
      AND login.revoked_at IS NULL AND login.expires_at > statement_timestamp();
    IF NOT FOUND THEN
        RAISE EXCEPTION USING ERRCODE = 'PT401', MESSAGE = 'unauthorized';
    END IF;

    RETURN QUERY
    SELECT q.id, q.name, q.currency, q.role, q.permissions FROM (
        SELECT s.id, s.name, s.currency,
               (SELECT f.role FROM identity.store_staff AS f WHERE f.tenant_id = s.tenant_id AND f.store_id = s.id AND f.principal_id = m.principal_id) AS role,
               ARRAY(SELECT x.permission FROM identity.store_grants AS x WHERE x.tenant_id = s.tenant_id AND x.store_id = s.id
                     AND x.principal_id = m.principal_id ORDER BY x.permission) AS permissions
        FROM identity.memberships AS m
        JOIN control.tenants AS t ON t.id = m.tenant_id AND t.active
        JOIN control.stores AS s ON s.tenant_id = t.id AND s.active
        JOIN identity.store_grants AS g ON g.tenant_id = s.tenant_id
          AND g.store_id = s.id AND g.principal_id = m.principal_id
          AND g.permission = 'store:read'
        WHERE m.principal_id = v_principal AND m.active
        UNION ALL
        SELECT s.id, s.name, s.currency, 'support'::text AS role, sg.permissions
        FROM identity.support_grants AS sg
        JOIN control.stores AS s ON s.tenant_id = sg.tenant_id AND s.id = sg.store_id AND s.active
        JOIN control.tenants AS t ON t.id = s.tenant_id AND t.active
        WHERE sg.principal_id = v_principal AND sg.revoked_at IS NULL AND sg.expires_at > clock_timestamp()
          AND identity.support_principal_ok(v_principal)
    ) AS q
    ORDER BY q.id
    LIMIT 101;
END;
$$;
COMMENT ON FUNCTION identity.resolve_access(bytea,uuid,text) IS
 '0003 + 0153: derives tenant/principal/revision from the hashed merchant session. Regular branch unchanged; support branch (live unrevoked unexpired support_grants row, store+tenant active, principal passes support_principal_ok, requested permission a granted ":read", NULL forbidden) returns ok with authz_revision -1.';
COMMENT ON FUNCTION identity.list_session_stores(bytea) IS '0005 + 0089 + 0153: the caller''s active stores with the caller''s staff role (NULL for pre-0089 members) and effective permissions per store, plus stores reachable only through a live support grant (role support); bounded projection of the hashed merchant session, never a client-chosen principal.';
