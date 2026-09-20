-- Authentication failures, invisible resources and missing operations are
-- distinct without exposing another store's identifiers to the caller.
CREATE FUNCTION identity.resolve_access(p_hash bytea, p_store uuid, p_permission text)
RETURNS TABLE (access_status text, tenant_id uuid, principal_id uuid, authz_revision bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE
    v_principal uuid;
    v_tenant uuid;
    v_revision bigint;
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
        RETURN QUERY SELECT 'not_found'::text, NULL::uuid, NULL::uuid, NULL::bigint;
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
ALTER FUNCTION identity.resolve_access(bytea, uuid, text) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.resolve_access(bytea, uuid, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.resolve_access(bytea, uuid, text) TO commerce_runtime;
