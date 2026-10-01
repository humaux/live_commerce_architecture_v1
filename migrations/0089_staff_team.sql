-- 0089 staff team (contracts/storefront-v2.md §D, R4 unit staff-team): role bundles over the existing permission list,
-- email invitations (hashed one-time token, 72 h, single use, bound to store + role + email), role change, revoke.
-- Owning package: internal/identity (staff.go). Only the commerce_identity login (internal/platform.OpenIdentityPool, the
-- same pool as password auth) may EXECUTE the public definers below; the owner role commerce_staff_writer is NOLOGIN.
--
-- Design decisions (binding for readers of this file):
--   * A "role" is a label in identity.store_staff PLUS a fixed permission bundle materialised in identity.store_grants.
--     Authorization everywhere else is unchanged: identity.resolve_access reads memberships.active + store_grants on EVERY
--     request, so removing the grants and deactivating the membership (revoke) stops authorizing on the very next request;
--     no session needs to be touched.
--   * Staff management is owner-only. It is gated by the role row (store_staff.role = 'owner'), NOT by a new permission:
--     the permission CHECK (store_grants_permission_check) is rewritten by almost every migration and a new value would
--     have to be backfilled to every existing owner. "admin = all but staff management and billing" therefore means the
--     owner bundle minus billing:manage.
--   * Owner floor ("at least one owner always remains") is enforced in SQL twice: each mutating definer checks it after
--     its change under a per-store advisory lock, and a DEFERRED constraint trigger on identity.store_staff re-checks at
--     commit under the same lock, so even a direct DML by any role holding table privileges cannot leave a store ownerless.
--   * Invitation tokens are 256-bit random values; only sha256(token text) is stored (token_hash). The plaintext exists in the
--     inviter's process memory for one mail send and in the email; it is never stored, logged or returned by SQL.
--   * Accept is by the SIGNED-IN principal: its verified password email must equal the invited email. Every failure that
--     could reveal whether a token, an invitation or an account exists (unknown token, expired, revoked, already accepted,
--     other email, OIDC-only principal, inactive store) raises the same PT404 'invite_invalid'.
--   * Mail is sent by Go after commit, once (SMTP has no idempotency key, I06); the outcome is recorded in mail_state and a
--     resend is a NEW invitation (new token) that revokes the previous open one.
-- Non-goals: no role other than the five bundles, no custom per-user permissions, no multi-store invitation, no ownership
-- transfer workflow beyond "promote another owner then demote/revoke", no invite for an email that is already a member.

DO $$
DECLARE v_def text; p text;
BEGIN
  SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='identity.store_grants'::regclass AND c.conname='store_grants_permission_check';
  IF v_def IS NULL THEN RAISE EXCEPTION 'store_grants_permission_check missing'; END IF;
  -- Every permission named in a bundle must be accepted by the constraint; a missing one means 0089 ran before the
  -- migration that introduces it (0078 customers, 0079 billing, 0074 ads).
  FOREACH p IN ARRAY ARRAY['store:read','audit:read','audit:write','catalog:read','catalog:write','inventory:read','inventory:write',
    'inventory:reserve','pricing:read','pricing:write','integration:read','integration:manage','integration:execute','orders:read',
    'live:read','live:manage','payments:refund','fulfillment:write','orders:export','customers:read','customers:privacy',
    'billing:manage','ads:read','ads:manage','ads:approve'] LOOP
    IF position(quote_literal(p) IN v_def)=0 THEN
      RAISE EXCEPTION '0089 applied out of order: store_grants_permission_check lacks %',p;
    END IF;
  END LOOP;
END $$;

CREATE ROLE commerce_staff_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
COMMENT ON ROLE commerce_staff_writer IS
 '0089 owner of the staff-team definers (internal/identity staff.go). NOLOGIN, no members; writes identity.store_staff, staff_invitations, memberships, store_grants and ops.audit_events only through its fixed functions.';
GRANT USAGE ON SCHEMA identity, control, ops TO commerce_staff_writer;

-- ---------------------------------------------------------------------------------------------
-- Tables
-- ---------------------------------------------------------------------------------------------
CREATE TABLE identity.store_staff (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    principal_id uuid NOT NULL,
    role text NOT NULL CHECK (role IN ('owner','admin','live_operator','fulfilment','viewer')),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, principal_id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id),
    FOREIGN KEY (tenant_id, principal_id) REFERENCES identity.memberships(tenant_id, principal_id)
);
CREATE INDEX store_staff_owner ON identity.store_staff(tenant_id, store_id) WHERE role = 'owner';

CREATE TABLE identity.staff_invitations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    email text NOT NULL CHECK (length(email) BETWEEN 3 AND 254 AND email ~ '^[!-~]+$' AND email = lower(email) AND email ~ '^[^@]+@[^@]+$'),
    role text NOT NULL CHECK (role IN ('owner','admin','live_operator','fulfilment','viewer')),
    locale text NOT NULL CHECK (locale IN ('zh-CN','zh-TW','en')),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    invited_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    accepted_by uuid REFERENCES identity.principals(id),
    revoked_at timestamptz,
    mail_state text NOT NULL DEFAULT 'PENDING' CHECK (mail_state IN ('PENDING','SENT','FAILED','UNKNOWN')),
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id),
    FOREIGN KEY (tenant_id, invited_by) REFERENCES identity.memberships(tenant_id, principal_id),
    CONSTRAINT staff_invitations_ttl CHECK (expires_at = created_at + interval '72 hours'),
    CONSTRAINT staff_invitations_accept_shape CHECK ((accepted_at IS NULL) = (accepted_by IS NULL)),
    CONSTRAINT staff_invitations_one_end CHECK (accepted_at IS NULL OR revoked_at IS NULL)
);
-- One open invitation per (store, email): a resend revokes the previous one in the same transaction.
CREATE UNIQUE INDEX staff_invitations_open ON identity.staff_invitations(store_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE INDEX staff_invitations_store ON identity.staff_invitations(tenant_id, store_id, created_at DESC);

-- ---------------------------------------------------------------------------------------------
-- Role bundles (contracts/storefront-v2.md §D). store:read is in every bundle: resolve_access requires it first.
-- owner/admin/viewer follow the live catalogue; live_operator and fulfilment are fixed lists (contract §D).
-- ---------------------------------------------------------------------------------------------
-- The permission catalogue is the live store_grants_permission_check (every migration that adds a permission rewrites it), so
-- the owner bundle is "all permissions" at GRANT time, including any permission introduced by a later migration (integrator ruling D1).
CREATE FUNCTION identity.staff_permission_catalogue() RETURNS text[]
LANGUAGE sql STABLE SET search_path = pg_catalog AS $$
    SELECT COALESCE(array_agg(m[1] ORDER BY m[1]), '{}'::text[])
      FROM pg_constraint c, regexp_matches(pg_get_constraintdef(c.oid), '''([a-z_]+:[a-z_]+)''', 'g') m
     WHERE c.conrelid = 'identity.store_grants'::regclass AND c.conname = 'store_grants_permission_check'
$$;
CREATE FUNCTION identity.staff_role_permissions(p_role text)
RETURNS text[] LANGUAGE sql STABLE SET search_path = pg_catalog AS $$
    SELECT CASE p_role
      WHEN 'owner' THEN identity.staff_permission_catalogue()
      -- admin = owner minus billing (staff management is the owner role itself, see header).
      WHEN 'admin' THEN ARRAY(SELECT x FROM unnest(identity.staff_permission_catalogue()) x WHERE x <> 'billing:manage')
      WHEN 'live_operator' THEN ARRAY['store:read','live:read','live:manage','catalog:read','orders:read','inventory:read']
      WHEN 'fulfilment' THEN ARRAY['store:read','orders:read','fulfillment:write','orders:export','inventory:read','inventory:write','inventory:reserve']
      -- viewer = every :read permission of the catalogue.
      WHEN 'viewer' THEN ARRAY(SELECT x FROM unnest(identity.staff_permission_catalogue()) x WHERE x LIKE '%:read')
    END
$$;
ALTER FUNCTION identity.staff_permission_catalogue() OWNER TO commerce_staff_writer;
REVOKE ALL ON FUNCTION identity.staff_permission_catalogue() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.staff_permission_catalogue() TO commerce_staff_writer;
COMMENT ON FUNCTION identity.staff_permission_catalogue() IS 'Owner: internal/identity. Every permission accepted by store_grants_permission_check right now (parsed from the live constraint). Internal to the staff bundles.';
ALTER FUNCTION identity.staff_role_permissions(text) OWNER TO commerce_staff_writer;
REVOKE ALL ON FUNCTION identity.staff_role_permissions(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.staff_role_permissions(text) TO commerce_staff_writer;

-- ---------------------------------------------------------------------------------------------
-- Grants of the owner role (column-level; no table-wide UPDATE anywhere).
-- ---------------------------------------------------------------------------------------------
GRANT EXECUTE ON FUNCTION identity.resolve_access(bytea,uuid,text) TO commerce_staff_writer;
GRANT SELECT, INSERT, DELETE ON identity.store_staff TO commerce_staff_writer;
GRANT UPDATE (role, updated_at) ON identity.store_staff TO commerce_staff_writer;
GRANT SELECT, INSERT ON identity.staff_invitations TO commerce_staff_writer;
GRANT UPDATE (accepted_at, accepted_by, revoked_at, mail_state) ON identity.staff_invitations TO commerce_staff_writer;
GRANT SELECT, INSERT ON identity.memberships TO commerce_staff_writer;
GRANT UPDATE (active, authz_revision) ON identity.memberships TO commerce_staff_writer;
GRANT SELECT, INSERT, DELETE ON identity.store_grants TO commerce_staff_writer;
GRANT SELECT (id, active) ON identity.principals TO commerce_staff_writer;
GRANT SELECT (token_hash, principal_id, audience, expires_at, revoked_at) ON identity.sessions TO commerce_staff_writer;
-- password_credentials stays readable by commerce_identity_writer ONLY (0070, gate PA03): the two lookups below are definers of THAT role.
CREATE FUNCTION identity.staff_password_email(p_principal uuid) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT c.email FROM identity.password_credentials c WHERE c.principal_id = p_principal
$$;
CREATE FUNCTION identity.staff_principal_of_email(p_email text) RETURNS uuid
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT c.principal_id FROM identity.password_credentials c WHERE c.email = p_email
$$;
ALTER FUNCTION identity.staff_password_email(uuid) OWNER TO commerce_identity_writer;
ALTER FUNCTION identity.staff_principal_of_email(text) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION identity.staff_password_email(uuid), identity.staff_principal_of_email(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.staff_password_email(uuid), identity.staff_principal_of_email(text) TO commerce_staff_writer;
COMMENT ON FUNCTION identity.staff_password_email(uuid) IS '0089 internal (owner commerce_identity_writer, EXECUTE commerce_staff_writer only): the verified password email of a principal, or NULL. Lets the staff definers bind an invitation to an email without any table privilege on password_credentials (PA03).';
COMMENT ON FUNCTION identity.staff_principal_of_email(text) IS '0089 internal (owner commerce_identity_writer, EXECUTE commerce_staff_writer only): the principal holding a verified password email, or NULL. Used only to refuse inviting an existing member of the same store.';
GRANT SELECT (id, active) ON control.tenants TO commerce_staff_writer;
GRANT SELECT (tenant_id, id, active) ON control.stores TO commerce_staff_writer;
GRANT INSERT ON ops.audit_events TO commerce_staff_writer;
CREATE POLICY staff_writer_stores_read ON control.stores FOR SELECT TO commerce_staff_writer USING (true);
CREATE POLICY staff_writer_audit_insert ON ops.audit_events FOR INSERT TO commerce_staff_writer
 WITH CHECK (action IN ('staff.invited:owner','staff.invited:admin','staff.invited:live_operator','staff.invited:fulfilment','staff.invited:viewer',
   'staff.invite_revoked','staff.accepted:owner','staff.accepted:admin','staff.accepted:live_operator','staff.accepted:fulfilment','staff.accepted:viewer',
   'staff.role_changed:owner','staff.role_changed:admin','staff.role_changed:live_operator','staff.role_changed:fulfilment','staff.role_changed:viewer',
   'staff.removed'));
COMMENT ON POLICY staff_writer_stores_read ON control.stores IS '0089: staff definers resolve a store''s tenant and active flag (columns tenant_id,id,active only).';
COMMENT ON POLICY staff_writer_audit_insert ON ops.audit_events IS '0089: the 17 fixed staff audit actions only (role suffix names the new role; target principal is in identity.store_staff / staff_invitations).';

-- ---------------------------------------------------------------------------------------------
-- Owner floor. Same advisory lock key as every mutating definer, so two concurrent demotions serialize and the second
-- sees the first's committed state. Deferred: a transaction may swap owners (promote B, then demote A) in any order.
-- ---------------------------------------------------------------------------------------------
CREATE FUNCTION identity.staff_lock_store(p_store uuid) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT pg_advisory_xact_lock(hashtextextended('lc:staff:' || p_store::text, 0))
$$;
CREATE FUNCTION identity.staff_assert_owner_floor(p_tenant uuid, p_store uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM identity.store_staff s WHERE s.tenant_id = p_tenant AND s.store_id = p_store AND s.role = 'owner') THEN
        RAISE EXCEPTION 'last_owner' USING ERRCODE = 'PT409';
    END IF;
END $$;
CREATE FUNCTION identity.staff_owner_floor_trigger() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF OLD.role = 'owner' THEN
        PERFORM identity.staff_lock_store(OLD.store_id);
        PERFORM identity.staff_assert_owner_floor(OLD.tenant_id, OLD.store_id);
    END IF;
    RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER store_staff_owner_floor AFTER UPDATE OR DELETE ON identity.store_staff
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION identity.staff_owner_floor_trigger();

-- The store creator is the first owner: a trigger (not a rewrite of create_initial_store, which several migrations
-- redefine) so every future definition keeps working. Backfill: creators of existing stores and every principal that
-- already holds the owner-only permission billing:manage. Other pre-0089 grants (fixtures) get no role and cannot manage staff.
CREATE FUNCTION identity.staff_creator_trigger() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    INSERT INTO identity.store_staff(tenant_id, store_id, principal_id, role) VALUES (NEW.tenant_id, NEW.store_id, NEW.principal_id, 'owner')
    ON CONFLICT DO NOTHING;
    -- D1: the creator holds the full owner bundle (create_initial_store's own list predates ads:* and later permissions).
    PERFORM identity.staff_apply_role(NEW.tenant_id, NEW.store_id, NEW.principal_id, 'owner');
    RETURN NULL;
END $$;
INSERT INTO identity.store_staff(tenant_id, store_id, principal_id, role)
SELECT i.tenant_id, i.store_id, i.principal_id, 'owner' FROM identity.initial_stores i
UNION
SELECT g.tenant_id, g.store_id, g.principal_id, 'owner' FROM identity.store_grants g WHERE g.permission = 'billing:manage'
ON CONFLICT DO NOTHING;
ALTER FUNCTION identity.staff_lock_store(uuid) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_assert_owner_floor(uuid,uuid) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_owner_floor_trigger() OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_creator_trigger() OWNER TO commerce_staff_writer;
REVOKE ALL ON FUNCTION identity.staff_lock_store(uuid), identity.staff_assert_owner_floor(uuid,uuid),
  identity.staff_owner_floor_trigger(), identity.staff_creator_trigger() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.staff_lock_store(uuid), identity.staff_assert_owner_floor(uuid,uuid) TO commerce_staff_writer;
CREATE TRIGGER initial_stores_first_owner AFTER INSERT ON identity.initial_stores
    FOR EACH ROW EXECUTE FUNCTION identity.staff_creator_trigger();

-- ---------------------------------------------------------------------------------------------
-- staff_owner_ctx: the caller's authority for every owner-only definer. Verifies the bearer (resolve_access, store:read),
-- takes the store lock, then verifies again and requires the owner role row, so a caller demoted while waiting for the
-- lock loses authority before it writes. Internal: no EXECUTE for the login.
-- ---------------------------------------------------------------------------------------------
CREATE FUNCTION identity.staff_owner_ctx(p_hash bytea, p_store uuid)
RETURNS TABLE(tenant_id uuid, principal_id uuid) LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE s record;
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash) <> 32 OR p_store IS NULL THEN RAISE EXCEPTION 'invalid staff request' USING ERRCODE = 'PT400'; END IF;
    SELECT * INTO s FROM identity.resolve_access(p_hash, p_store, 'store:read');
    IF s.access_status = 'unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE = 'PT401'; END IF;
    IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
    PERFORM identity.staff_lock_store(p_store);
    SELECT * INTO s FROM identity.resolve_access(p_hash, p_store, 'store:read');
    IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE = 'PT401'; END IF;
    IF NOT EXISTS (SELECT 1 FROM identity.store_staff t WHERE t.tenant_id = s.tenant_id AND t.store_id = p_store
                   AND t.principal_id = s.principal_id AND t.role = 'owner') THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    RETURN QUERY SELECT s.tenant_id, s.principal_id;
END $$;
ALTER FUNCTION identity.staff_owner_ctx(bytea,uuid) OWNER TO commerce_staff_writer;
REVOKE ALL ON FUNCTION identity.staff_owner_ctx(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.staff_owner_ctx(bytea,uuid) TO commerce_staff_writer;

-- ---------------------------------------------------------------------------------------------
-- Public definers (EXECUTE commerce_identity). p_hash = sha256 of the merchant bearer, as in create_initial_store.
-- ---------------------------------------------------------------------------------------------

-- staff_list: any member sees its own role; only an owner receives members and invitations.
CREATE FUNCTION identity.staff_list(p_hash bytea, p_store uuid) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE s record; v_role text; v_members jsonb := '[]'::jsonb; v_invites jsonb := '[]'::jsonb;
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash) <> 32 OR p_store IS NULL THEN RAISE EXCEPTION 'invalid staff request' USING ERRCODE = 'PT400'; END IF;
    SELECT * INTO s FROM identity.resolve_access(p_hash, p_store, 'store:read');
    IF s.access_status = 'unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE = 'PT401'; END IF;
    IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
    SELECT t.role INTO v_role FROM identity.store_staff t WHERE t.tenant_id = s.tenant_id AND t.store_id = p_store AND t.principal_id = s.principal_id;
    IF v_role = 'owner' THEN
        SELECT COALESCE(jsonb_agg(jsonb_build_object('principal_id', t.principal_id, 'email', identity.staff_password_email(t.principal_id), 'role', t.role,
                 'joined_at', t.created_at, 'is_me', t.principal_id = s.principal_id) ORDER BY t.created_at, t.principal_id), '[]'::jsonb)
          INTO v_members
          FROM identity.store_staff t JOIN identity.memberships m ON m.tenant_id = t.tenant_id AND m.principal_id = t.principal_id AND m.active
         WHERE t.tenant_id = s.tenant_id AND t.store_id = p_store;
        SELECT COALESCE(jsonb_agg(jsonb_build_object('id', i.id, 'email', i.email, 'role', i.role, 'created_at', i.created_at,
                 'expires_at', i.expires_at, 'expired', i.expires_at <= clock_timestamp(), 'mail_state', i.mail_state) ORDER BY i.created_at DESC, i.id), '[]'::jsonb)
          INTO v_invites
          FROM identity.staff_invitations i
         WHERE i.tenant_id = s.tenant_id AND i.store_id = p_store AND i.accepted_at IS NULL AND i.revoked_at IS NULL;
    END IF;
    RETURN jsonb_build_object('my_role', v_role, 'members', v_members, 'invitations', v_invites);
END $$;

CREATE FUNCTION identity.staff_invite(p_hash bytea, p_store uuid, p_id uuid, p_email text, p_role text, p_locale text, p_token_hash bytea)
RETURNS TABLE(invite_id uuid, expires_at timestamptz) LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE c record; v_now timestamptz := clock_timestamp(); v_expiry timestamptz;
BEGIN
    IF p_id IS NULL OR p_email IS NULL OR length(p_email) NOT BETWEEN 3 AND 254 OR p_email !~ '^[!-~]+$' OR p_email <> lower(p_email) OR p_email !~ '^[^@]+@[^@]+$'
       OR p_role IS NULL OR p_role NOT IN ('owner','admin','live_operator','fulfilment','viewer')
       OR p_locale IS NULL OR p_locale NOT IN ('zh-CN','zh-TW','en') OR p_token_hash IS NULL OR octet_length(p_token_hash) <> 32 THEN
        RAISE EXCEPTION 'invalid staff request' USING ERRCODE = 'PT400';
    END IF;
    SELECT * INTO c FROM identity.staff_owner_ctx(p_hash, p_store);
    -- Already a member of THIS store: the owner can see the member list, so naming the conflict leaks nothing.
    IF EXISTS (SELECT 1 FROM identity.store_staff t WHERE t.tenant_id = c.tenant_id AND t.store_id = p_store
               AND t.principal_id = identity.staff_principal_of_email(p_email)) THEN
        RAISE EXCEPTION 'already_member' USING ERRCODE = 'PT409';
    END IF;
    -- Mail-abuse ceilings per store: 20 live invitations, 50 created per 24 h (a resend counts as a creation).
    IF (SELECT count(*) FROM identity.staff_invitations i WHERE i.store_id = p_store AND i.accepted_at IS NULL AND i.revoked_at IS NULL
          AND i.expires_at > v_now AND i.email <> p_email) >= 20
       OR (SELECT count(*) FROM identity.staff_invitations i WHERE i.store_id = p_store AND i.created_at > v_now - interval '24 hours') >= 50 THEN
        RAISE EXCEPTION 'too_many_invitations' USING ERRCODE = 'PT429';
    END IF;
    UPDATE identity.staff_invitations i SET revoked_at = v_now
     WHERE i.store_id = p_store AND i.email = p_email AND i.accepted_at IS NULL AND i.revoked_at IS NULL;
    v_expiry := v_now + interval '72 hours';
    INSERT INTO identity.staff_invitations(id, tenant_id, store_id, email, role, locale, token_hash, invited_by, created_at, expires_at)
    VALUES (p_id, c.tenant_id, p_store, p_email, p_role, p_locale, p_token_hash, c.principal_id, v_now, v_expiry);
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (c.tenant_id, p_store, c.principal_id, 'staff.invited:' || p_role);
    RETURN QUERY SELECT p_id, v_expiry;
END $$;

-- Go passes sha256(token text) to staff_invite and staff_accept: the plaintext token never reaches SQL, so it cannot
-- appear in statement logs, pg_stat_activity or an error context.

CREATE FUNCTION identity.staff_record_invite_mail(p_id uuid, p_state text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF p_id IS NULL OR p_state IS NULL OR p_state NOT IN ('SENT','FAILED','UNKNOWN') THEN RAISE EXCEPTION 'invalid staff request' USING ERRCODE = 'PT400'; END IF;
    UPDATE identity.staff_invitations i SET mail_state = p_state WHERE i.id = p_id AND i.mail_state = 'PENDING';
END $$;

CREATE FUNCTION identity.staff_revoke_invite(p_hash bytea, p_store uuid, p_invite uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE c record;
BEGIN
    IF p_invite IS NULL THEN RAISE EXCEPTION 'invalid staff request' USING ERRCODE = 'PT400'; END IF;
    SELECT * INTO c FROM identity.staff_owner_ctx(p_hash, p_store);
    UPDATE identity.staff_invitations i SET revoked_at = clock_timestamp()
     WHERE i.id = p_invite AND i.tenant_id = c.tenant_id AND i.store_id = p_store AND i.accepted_at IS NULL AND i.revoked_at IS NULL;
    IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (c.tenant_id, p_store, c.principal_id, 'staff.invite_revoked');
END $$;

-- Replaces the principal's grants in this store with the role bundle (all-or-nothing in one transaction).
CREATE FUNCTION identity.staff_apply_role(p_tenant uuid, p_store uuid, p_principal uuid, p_role text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    DELETE FROM identity.store_grants g WHERE g.tenant_id = p_tenant AND g.store_id = p_store AND g.principal_id = p_principal;
    INSERT INTO identity.store_grants(tenant_id, store_id, principal_id, permission)
    SELECT p_tenant, p_store, p_principal, x FROM unnest(identity.staff_role_permissions(p_role)) x;
END $$;
ALTER FUNCTION identity.staff_apply_role(uuid,uuid,uuid,text) OWNER TO commerce_staff_writer;
REVOKE ALL ON FUNCTION identity.staff_apply_role(uuid,uuid,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.staff_apply_role(uuid,uuid,uuid,text) TO commerce_staff_writer;

CREATE FUNCTION identity.staff_set_role(p_hash bytea, p_store uuid, p_principal uuid, p_role text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE c record; v_old text;
BEGIN
    IF p_principal IS NULL OR p_role IS NULL OR p_role NOT IN ('owner','admin','live_operator','fulfilment','viewer') THEN
        RAISE EXCEPTION 'invalid staff request' USING ERRCODE = 'PT400';
    END IF;
    SELECT * INTO c FROM identity.staff_owner_ctx(p_hash, p_store);
    SELECT t.role INTO v_old FROM identity.store_staff t WHERE t.tenant_id = c.tenant_id AND t.store_id = p_store AND t.principal_id = p_principal FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
    IF v_old = p_role THEN RETURN; END IF;
    UPDATE identity.store_staff t SET role = p_role, updated_at = clock_timestamp()
     WHERE t.tenant_id = c.tenant_id AND t.store_id = p_store AND t.principal_id = p_principal;
    PERFORM identity.staff_apply_role(c.tenant_id, p_store, p_principal, p_role);
    UPDATE identity.memberships m SET authz_revision = m.authz_revision + 1 WHERE m.tenant_id = c.tenant_id AND m.principal_id = p_principal;
    PERFORM identity.staff_assert_owner_floor(c.tenant_id, p_store);
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (c.tenant_id, p_store, c.principal_id, 'staff.role_changed:' || p_role);
END $$;

-- Revoke: grants of this store are deleted and the membership is deactivated when no grant remains in the tenant, so
-- resolve_access answers not_found on the member's very next request. The membership row itself stays (audit FK).
CREATE FUNCTION identity.staff_remove(p_hash bytea, p_store uuid, p_principal uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE c record;
BEGIN
    IF p_principal IS NULL THEN RAISE EXCEPTION 'invalid staff request' USING ERRCODE = 'PT400'; END IF;
    SELECT * INTO c FROM identity.staff_owner_ctx(p_hash, p_store);
    PERFORM 1 FROM identity.store_staff t WHERE t.tenant_id = c.tenant_id AND t.store_id = p_store AND t.principal_id = p_principal FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
    DELETE FROM identity.store_staff t WHERE t.tenant_id = c.tenant_id AND t.store_id = p_store AND t.principal_id = p_principal;
    DELETE FROM identity.store_grants g WHERE g.tenant_id = c.tenant_id AND g.store_id = p_store AND g.principal_id = p_principal;
    UPDATE identity.memberships m SET authz_revision = m.authz_revision + 1,
           active = EXISTS (SELECT 1 FROM identity.store_grants g WHERE g.tenant_id = m.tenant_id AND g.principal_id = m.principal_id)
     WHERE m.tenant_id = c.tenant_id AND m.principal_id = p_principal;
    PERFORM identity.staff_assert_owner_floor(c.tenant_id, p_store);
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (c.tenant_id, p_store, c.principal_id, 'staff.removed');
END $$;

-- Accept by the signed-in principal. One generic PT404 'invite_invalid' for every reason (see header); PT409 only
-- when THIS principal is already staff of the store (it learns nothing about anyone else).
CREATE FUNCTION identity.staff_accept(p_hash bytea, p_token_hash bytea)
RETURNS TABLE(store_id uuid, role text) LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
#variable_conflict use_column
DECLARE v_principal uuid; v_email text; i record;
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash) <> 32 OR p_token_hash IS NULL OR octet_length(p_token_hash) <> 32 THEN RAISE EXCEPTION 'invalid staff request' USING ERRCODE = 'PT400'; END IF;
    SELECT p.id INTO v_principal FROM identity.sessions l JOIN identity.principals p ON p.id = l.principal_id AND p.active
     WHERE l.token_hash = p_hash AND l.audience = 'merchant' AND l.revoked_at IS NULL AND l.expires_at > clock_timestamp();
    IF NOT FOUND THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE = 'PT401'; END IF;
    -- Preliminary lookup only to learn the store for the lock; the authoritative read is the FOR UPDATE below.
    SELECT n.store_id INTO i FROM identity.staff_invitations n WHERE n.token_hash = p_token_hash;
    IF NOT FOUND THEN RAISE EXCEPTION 'invite_invalid' USING ERRCODE = 'PT404'; END IF;
    PERFORM identity.staff_lock_store(i.store_id);
    SELECT n.* INTO i FROM identity.staff_invitations n WHERE n.token_hash = p_token_hash FOR UPDATE;
    v_email := identity.staff_password_email(v_principal);
    IF v_email IS NULL OR v_email IS DISTINCT FROM i.email OR i.accepted_at IS NOT NULL OR i.revoked_at IS NOT NULL OR i.expires_at <= clock_timestamp()
       OR NOT EXISTS (SELECT 1 FROM control.stores s JOIN control.tenants t ON t.id = s.tenant_id AND t.active
                      WHERE s.id = i.store_id AND s.tenant_id = i.tenant_id AND s.active) THEN
        RAISE EXCEPTION 'invite_invalid' USING ERRCODE = 'PT404';
    END IF;
    IF EXISTS (SELECT 1 FROM identity.store_staff t WHERE t.tenant_id = i.tenant_id AND t.store_id = i.store_id AND t.principal_id = v_principal) THEN
        RAISE EXCEPTION 'already_member' USING ERRCODE = 'PT409';
    END IF;
    INSERT INTO identity.memberships(tenant_id, principal_id) VALUES (i.tenant_id, v_principal)
    ON CONFLICT (tenant_id, principal_id) DO UPDATE SET active = true, authz_revision = identity.memberships.authz_revision + 1;
    INSERT INTO identity.store_staff(tenant_id, store_id, principal_id, role) VALUES (i.tenant_id, i.store_id, v_principal, i.role);
    PERFORM identity.staff_apply_role(i.tenant_id, i.store_id, v_principal, i.role);
    UPDATE identity.staff_invitations n SET accepted_at = clock_timestamp(), accepted_by = v_principal WHERE n.id = i.id;
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (i.tenant_id, i.store_id, v_principal, 'staff.accepted:' || i.role);
    RETURN QUERY SELECT i.store_id, i.role;
END $$;

ALTER FUNCTION identity.staff_owner_ctx(bytea,uuid) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_list(bytea,uuid) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_invite(bytea,uuid,uuid,text,text,text,bytea) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_record_invite_mail(uuid,text) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_revoke_invite(bytea,uuid,uuid) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_set_role(bytea,uuid,uuid,text) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_remove(bytea,uuid,uuid) OWNER TO commerce_staff_writer;
ALTER FUNCTION identity.staff_accept(bytea,bytea) OWNER TO commerce_staff_writer;
REVOKE ALL ON FUNCTION identity.staff_list(bytea,uuid), identity.staff_invite(bytea,uuid,uuid,text,text,text,bytea),
  identity.staff_record_invite_mail(uuid,text), identity.staff_revoke_invite(bytea,uuid,uuid), identity.staff_set_role(bytea,uuid,uuid,text),
  identity.staff_remove(bytea,uuid,uuid), identity.staff_accept(bytea,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.staff_list(bytea,uuid), identity.staff_invite(bytea,uuid,uuid,text,text,text,bytea),
  identity.staff_record_invite_mail(uuid,text), identity.staff_revoke_invite(bytea,uuid,uuid), identity.staff_set_role(bytea,uuid,uuid,text),
  identity.staff_remove(bytea,uuid,uuid), identity.staff_accept(bytea,bytea) TO commerce_identity;

-- D1 backfill: every existing owner holds the full owner bundle (additive; nothing is removed).
INSERT INTO identity.store_grants(tenant_id, store_id, principal_id, permission)
SELECT t.tenant_id, t.store_id, t.principal_id, x FROM identity.store_staff t, unnest(identity.staff_role_permissions('owner')) x
WHERE t.role = 'owner'
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------------------------
-- Role-aware navigation (integrator ruling D2): the session store list also returns the caller's staff role and effective
-- permissions per store, so the admin UI can hide entries it cannot use. The server stays the authority on every request.
-- Same authority as 0005 (owner commerce_auth, SECURITY DEFINER, search_path pg_catalog); only two columns are added.
-- ---------------------------------------------------------------------------------------------
GRANT SELECT ON identity.store_staff TO commerce_auth;
DROP FUNCTION identity.list_session_stores(bytea);
CREATE FUNCTION identity.list_session_stores(p_hash bytea)
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
    SELECT s.id, s.name, s.currency,
           (SELECT f.role FROM identity.store_staff AS f WHERE f.tenant_id = s.tenant_id AND f.store_id = s.id AND f.principal_id = m.principal_id),
           ARRAY(SELECT x.permission FROM identity.store_grants AS x WHERE x.tenant_id = s.tenant_id AND x.store_id = s.id
                 AND x.principal_id = m.principal_id ORDER BY x.permission)
    FROM identity.memberships AS m
    JOIN control.tenants AS t ON t.id = m.tenant_id AND t.active
    JOIN control.stores AS s ON s.tenant_id = t.id AND s.active
    JOIN identity.store_grants AS g ON g.tenant_id = s.tenant_id
      AND g.store_id = s.id AND g.principal_id = m.principal_id
      AND g.permission = 'store:read'
    WHERE m.principal_id = v_principal AND m.active
    ORDER BY s.id
    LIMIT 101;
END;
$$;
ALTER FUNCTION identity.list_session_stores(bytea) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.list_session_stores(bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.list_session_stores(bytea) TO commerce_runtime;
COMMENT ON FUNCTION identity.list_session_stores(bytea) IS '0005 + 0089: the caller''s active stores with the caller''s staff role (NULL for pre-0089 members) and effective permissions per store; bounded projection of the hashed merchant session, never a client-chosen principal.';

-- ---------------------------------------------------------------------------------------------
-- COMMENT ON (PROCESS §5)
-- ---------------------------------------------------------------------------------------------
COMMENT ON TABLE identity.store_staff IS 'Owner: internal/identity (staff.go). One role label per member per store; the permissions live in identity.store_grants (materialised by staff_apply_role). Written only by the staff definers and the initial_stores trigger. Owner floor: deferred trigger store_staff_owner_floor. Non-goal: not an authorization source by itself (resolve_access reads store_grants).';
COMMENT ON COLUMN identity.store_staff.role IS 'owner | admin | live_operator | fulfilment | viewer (contracts/storefront-v2.md §D). owner alone may manage staff.';
COMMENT ON COLUMN identity.store_staff.created_at IS 'When the principal joined the store team (first owner: store creation or 0089 backfill time).';
COMMENT ON COLUMN identity.store_staff.updated_at IS 'Last role change.';
COMMENT ON TABLE identity.staff_invitations IS 'Owner: internal/identity (staff.go). One emailed staff invitation bound to store + role + email; token_hash = sha256 of the 43-character base64url text of a 256-bit token, single use (accepted_at), 72 h (CHECK), revocable. Written only by the staff definers. Non-goals: not a mail outbox (one send after commit, never retried, I06), keeps no plaintext token.';
COMMENT ON COLUMN identity.staff_invitations.id IS 'Go-generated invitation id; the handle for revoke and for recording the mail outcome.';
COMMENT ON COLUMN identity.staff_invitations.email IS 'Invited address, ASCII lower case; must equal the accepting principal''s verified password email.';
COMMENT ON COLUMN identity.staff_invitations.role IS 'Role granted on accept; may include owner.';
COMMENT ON COLUMN identity.staff_invitations.locale IS 'Mail language = the inviter''s admin locale.';
COMMENT ON COLUMN identity.staff_invitations.token_hash IS 'sha256 of the 43-character base64url token text (Go: digest(token)); UNIQUE lookup key. The plaintext is only in the email link.';
COMMENT ON COLUMN identity.staff_invitations.invited_by IS 'Owner principal that created it (FK to its membership).';
COMMENT ON COLUMN identity.staff_invitations.expires_at IS 'created_at + 72 hours (CHECK staff_invitations_ttl).';
COMMENT ON COLUMN identity.staff_invitations.accepted_at IS 'Set once by staff_accept; the invitation can never be accepted again.';
COMMENT ON COLUMN identity.staff_invitations.accepted_by IS 'Principal that accepted.';
COMMENT ON COLUMN identity.staff_invitations.revoked_at IS 'Set by the owner revoking it or by a resend to the same email; mutually exclusive with accepted_at.';
COMMENT ON COLUMN identity.staff_invitations.mail_state IS 'PENDING -> SENT | FAILED | UNKNOWN, set once by staff_record_invite_mail; UNKNOWN is never retried (I06).';
COMMENT ON FUNCTION identity.staff_role_permissions(text) IS 'Owner: internal/identity. The five role bundles of contracts/storefront-v2.md §D as permission arrays; owner = whole catalogue, admin = catalogue minus billing:manage, viewer = every :read, evaluated when grants are written.';
COMMENT ON FUNCTION identity.staff_lock_store(uuid) IS 'Internal: transaction-scoped advisory lock keyed by store; serializes every staff mutation and the owner-floor trigger of one store.';
COMMENT ON FUNCTION identity.staff_assert_owner_floor(uuid,uuid) IS 'Internal: raises PT409 last_owner when the store has no owner row.';
COMMENT ON FUNCTION identity.staff_owner_floor_trigger() IS 'Internal: deferred constraint-trigger body; re-checks the owner floor at commit under the store lock so direct DML cannot leave a store ownerless.';
COMMENT ON FUNCTION identity.staff_creator_trigger() IS 'Internal: makes the creator in identity.initial_stores the first owner (works with any create_initial_store definition).';
COMMENT ON FUNCTION identity.staff_owner_ctx(bytea,uuid) IS 'Internal: bearer + store -> (tenant, principal) of an OWNER, under the store lock; PT401 unauthorized, PT404 not a member, PT403 not owner. Not executable by the login.';
COMMENT ON FUNCTION identity.staff_apply_role(uuid,uuid,uuid,text) IS 'Internal: replaces the principal''s store grants with the role bundle.';
COMMENT ON FUNCTION identity.staff_list(bytea,uuid) IS 'Owner: internal/identity. Role: commerce_identity only. {my_role, members, invitations}; members/invitations only for an owner (token_hash never returned).';
COMMENT ON FUNCTION identity.staff_invite(bytea,uuid,uuid,text,text,text,bytea) IS 'Owner: internal/identity. Role: commerce_identity only. Owner creates an invitation (revoking an open one for the same email), PT409 already_member, PT429 over 20 live / 50 per 24 h. Audit staff.invited:<role>.';
COMMENT ON FUNCTION identity.staff_record_invite_mail(uuid,text) IS 'Owner: internal/identity. Role: commerce_identity only. Sets mail_state once from PENDING; never triggers a resend.';
COMMENT ON FUNCTION identity.staff_revoke_invite(bytea,uuid,uuid) IS 'Owner: internal/identity. Role: commerce_identity only. Owner revokes an open invitation; PT404 when none. Audit staff.invite_revoked.';
COMMENT ON FUNCTION identity.staff_set_role(bytea,uuid,uuid,text) IS 'Owner: internal/identity. Role: commerce_identity only. Owner changes a member''s role (grants replaced, authz_revision bumped); PT409 last_owner when it would remove the last owner.';
COMMENT ON FUNCTION identity.staff_remove(bytea,uuid,uuid) IS 'Owner: internal/identity. Role: commerce_identity only. Owner revokes a member: role row and store grants deleted, membership deactivated when no grant remains; effective on the next request. PT409 last_owner.';
COMMENT ON FUNCTION identity.staff_accept(bytea,bytea) IS 'Owner: internal/identity. Role: commerce_identity only. The signed-in principal accepts by token: single use, 72 h, email must equal its verified email; every refusal reason is the same PT404 invite_invalid; PT409 already_member.';
