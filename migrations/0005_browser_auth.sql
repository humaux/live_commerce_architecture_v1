-- A bounded authentication projection, not a cross-tenant table grant.
-- Browser input never chooses the principal or tenant being enumerated.
CREATE FUNCTION identity.list_session_stores(p_hash bytea)
RETURNS TABLE (id uuid, name text, currency text)
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
    SELECT s.id, s.name, s.currency
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
