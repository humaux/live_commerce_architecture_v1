-- Purpose: time-boxed, read-only platform support access to ONE store for a named support principal (identity.support_grants), the three operator definers that manage it, and the one extra branch in identity.resolve_access / list_session_stores that honours a live grant.
-- Depends on: 0001 (identity.principals/memberships/store_grants, control.stores/tenants, ops.audit_events), 0003 (identity.resolve_access), 0089 (identity.list_session_stores), 0130 (ops.audit_events.details), 0143 (control.operator_audit, roles commerce_platform_writer / commerce_platform_operator).
-- Used by: cmd/platform-admin support-grant|support-revoke|support-list (commerce_platform_operator login), internal/platform withScopeContext (support = negative authz_revision), tests/foundation/support_grant_test.go.
-- Invariants: default-deny (a support grant never widens a regular principal; it applies only when the principal holds no store_grants row for the store); read-only (CHECK on the permission pack + ':read' guard in resolve_access + read-only transaction in Go); expiry/revocation/suspension evaluated on every request, no sweeper.
-- Status: REAL_PG gate.
-- 0153 platform support grant (R3 unit OPS-02B; docs/delivery/units/ops-02b-support-grant.md "Integrator 裁决"; covers M01 #7, deviation A8).
--
-- A support principal logs in with the ordinary merchant login (no new login method, no impersonation). The platform operator CLI
-- (cmd/platform-admin) gives that principal limited-time READ access to one store; every grant and revoke writes the append-only
-- control.operator_audit (action support_grant/support_revoke) AND the merchant-visible ops.audit_events (support.granted /
-- support.revoked). Use is audited by the Go scope layer (support.used, at most one row per principal+store per 15 minutes).
--
-- resolve_access change (signature, owner commerce_auth and EXECUTE grants are unchanged, CREATE OR REPLACE): copy of the 0003 body plus ONE
-- branch. The regular branch is byte-for-byte the old logic. The support branch runs only when the regular lookup found nothing AND the
-- principal holds no store_grants row at all for that store; it requires an unrevoked, unexpired (clock_timestamp()) grant on an active
-- store of an active tenant, the permission must end in ':read' and be inside the grant. It returns authz_revision = -1: memberships
-- CHECK authz_revision > 0, so a negative revision can only come from this branch, which is how internal/platform recognises a support
-- scope without a signature change. Definers that independently re-join an active membership (e.g. the 0073 fences) refuse a support
-- principal: that is the intended default-deny.

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
-- Operator audit: two new actions. The store-scoped CHECK (store_id present iff the action is store-scoped) now also covers support_*.
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE c record;
BEGIN
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='control.operator_audit'::regclass AND contype='c'
   AND (pg_get_constraintdef(oid) LIKE '%store_suspend%' OR pg_get_constraintdef(oid) LIKE '%store\_%') LOOP
  EXECUTE format('ALTER TABLE control.operator_audit DROP CONSTRAINT %I', c.conname);
 END LOOP;
END $$;
ALTER TABLE control.operator_audit ADD CONSTRAINT operator_audit_action_check
 CHECK (action IN ('store_suspend','store_resume','tenant_suspend','tenant_resume','support_grant','support_revoke'));
ALTER TABLE control.operator_audit ADD CONSTRAINT operator_audit_store_scope_check
 CHECK ((action LIKE 'store\_%' OR action LIKE 'support\_%') = (store_id IS NOT NULL));

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
 -- Default-deny: support access is for principals with no regular access; never layered on a member or former member.
 IF EXISTS (SELECT 1 FROM identity.store_grants g WHERE g.tenant_id=v_tenant AND g.store_id=p_store AND g.principal_id=p_principal) THEN
  RAISE EXCEPTION 'principal has regular store access' USING ERRCODE='PT409'; END IF;
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
  jsonb_build_object('grant_id',v_id,'expires_at',v_exp,'operator',p_operator,'ticket',p_ticket));
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
  VALUES(v_tenant,v_store,v_principal,'support.revoked',jsonb_build_object('grant_id',v_id,'operator',p_operator,'ticket',p_ticket));
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
REVOKE ALL ON FUNCTION identity.grant_support(uuid,uuid,integer,text[],text,text), identity.revoke_support(uuid,uuid,uuid,text,text),
 identity.list_support_grants(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.grant_support(uuid,uuid,integer,text[],text,text), identity.revoke_support(uuid,uuid,uuid,text,text),
 identity.list_support_grants(uuid) TO commerce_platform_operator;
GRANT USAGE ON SCHEMA identity TO commerce_platform_operator;
COMMENT ON FUNCTION identity.grant_support(uuid,uuid,integer,text[],text,text) IS
 'cmd/platform-admin support-grant only; EXECUTE commerce_platform_operator. Opens a 1..72 h read-only grant (default pack store/orders/catalog/inventory/live/integration :read; never customers:*, inbox:*, payments:*, any write) for a principal with no regular access to the store; refuses a suspended store/tenant, an unknown principal, a second open grant. Inserts an inactive placeholder membership when needed; appends control.operator_audit (support_grant) and ops.audit_events (support.granted) in the same transaction.';
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
        -- 0153 support branch. Default-deny: only a principal with NO store_grants row for this store (a member, or a former
        -- member whose grants linger, never gets support access on top), and only a live grant: unrevoked, unexpired by the
        -- statement-independent clock, store and tenant active. Read-only twice over: the grant CHECK holds only ':read'
        -- permissions and the requested permission must be one of them.
        SELECT g.tenant_id, g.permissions INTO v_tenant, v_support_perms
        FROM identity.support_grants AS g
        JOIN control.stores AS s ON s.tenant_id = g.tenant_id AND s.id = g.store_id AND s.active
        JOIN control.tenants AS t ON t.id = s.tenant_id AND t.active
        WHERE g.store_id = p_store AND g.principal_id = v_principal
          AND g.revoked_at IS NULL AND g.expires_at > clock_timestamp()
          AND NOT EXISTS (SELECT 1 FROM identity.store_grants AS x
                           WHERE x.store_id = p_store AND x.principal_id = v_principal);
        IF NOT FOUND THEN
            RETURN QUERY SELECT 'not_found'::text, NULL::uuid, NULL::uuid, NULL::bigint;
            RETURN;
        END IF;
        IF p_permission !~ ':read$' OR NOT (p_permission = ANY(v_support_perms)) THEN
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
-- Role 'support' and the grant's permissions; stores where the principal holds any store_grants row are excluded here too.
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
          AND NOT EXISTS (SELECT 1 FROM identity.store_grants AS x WHERE x.store_id = s.id AND x.principal_id = v_principal)
    ) AS q
    ORDER BY q.id
    LIMIT 101;
END;
$$;
COMMENT ON FUNCTION identity.resolve_access(bytea,uuid,text) IS
 '0003 + 0153: derives tenant/principal/revision from the hashed merchant session. Regular branch unchanged; support branch (no store_grants row for the store, live unrevoked unexpired support_grants row, store+tenant active, requested permission a granted ":read") returns ok with authz_revision -1.';
COMMENT ON FUNCTION identity.list_session_stores(bytea) IS '0005 + 0089 + 0153: the caller''s active stores with the caller''s staff role (NULL for pre-0089 members) and effective permissions per store, plus stores reachable only through a live support grant (role support); bounded projection of the hashed merchant session, never a client-chosen principal.';
