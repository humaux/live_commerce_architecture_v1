-- Foundation slice only. Migration owner != API login. No production data.
CREATE ROLE commerce_runtime NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
CREATE ROLE commerce_auth NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
CREATE SCHEMA control;
CREATE SCHEMA identity;
CREATE SCHEMA ops;
CREATE SCHEMA river;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA control, identity, ops, river FROM PUBLIC;

CREATE TABLE control.tenants (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    active boolean NOT NULL DEFAULT true
);
CREATE TABLE control.stores (
    tenant_id uuid NOT NULL REFERENCES control.tenants(id),
    id uuid NOT NULL UNIQUE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    active boolean NOT NULL DEFAULT true,
    PRIMARY KEY (tenant_id, id)
);
CREATE TABLE identity.principals (
    id uuid PRIMARY KEY,
    active boolean NOT NULL DEFAULT true
);
CREATE TABLE identity.memberships (
    tenant_id uuid NOT NULL REFERENCES control.tenants(id),
    principal_id uuid NOT NULL REFERENCES identity.principals(id),
    active boolean NOT NULL DEFAULT true,
    authz_revision bigint NOT NULL DEFAULT 1 CHECK (authz_revision > 0),
    PRIMARY KEY (tenant_id, principal_id)
);
CREATE TABLE identity.store_grants (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    principal_id uuid NOT NULL,
    permission text NOT NULL CHECK (permission IN ('store:read', 'audit:read', 'audit:write')),
    PRIMARY KEY (tenant_id, store_id, principal_id, permission),
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id),
    FOREIGN KEY (tenant_id, principal_id) REFERENCES identity.memberships(tenant_id, principal_id)
);
CREATE TABLE identity.sessions (
    id uuid PRIMARY KEY,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    principal_id uuid NOT NULL REFERENCES identity.principals(id),
    audience text NOT NULL CHECK (audience IN ('merchant', 'buyer', 'support')),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE INDEX sessions_expiry ON identity.sessions(expires_at);

CREATE TABLE ops.audit_events (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    principal_id uuid NOT NULL,
    action text NOT NULL CHECK (action ~ '^[a-z][a-z0-9_.:]{0,79}$'),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id),
    FOREIGN KEY (tenant_id, principal_id) REFERENCES identity.memberships(tenant_id, principal_id)
);
CREATE INDEX audit_recent ON ops.audit_events(tenant_id, store_id, created_at DESC, id);

ALTER TABLE control.stores ENABLE ROW LEVEL SECURITY;
ALTER TABLE control.stores FORCE ROW LEVEL SECURITY;
ALTER TABLE ops.audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE ops.audit_events FORCE ROW LEVEL SECURITY;
CREATE POLICY stores_scoped ON control.stores FOR SELECT TO commerce_runtime
USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
   AND id = nullif(current_setting('app.store_id', true), '')::uuid);
-- Only a non-login, non-inherited authentication function owner can read this projection.
CREATE POLICY stores_auth_lookup ON control.stores FOR SELECT TO commerce_auth USING (true);
CREATE POLICY audit_scoped_read ON ops.audit_events FOR SELECT TO commerce_runtime
USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
   AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);
CREATE POLICY audit_scoped_insert ON ops.audit_events FOR INSERT TO commerce_runtime
WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
   AND store_id = nullif(current_setting('app.store_id', true), '')::uuid
   AND principal_id = nullif(current_setting('app.principal_id', true), '')::uuid);

GRANT USAGE ON SCHEMA identity, control TO commerce_auth;
GRANT SELECT ON control.tenants, control.stores, identity.principals,
    identity.memberships, identity.store_grants, identity.sessions TO commerce_auth;
CREATE FUNCTION identity.resolve_scope(p_hash bytea, p_store uuid, p_permission text)
RETURNS TABLE (tenant_id uuid, principal_id uuid, authz_revision bigint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT s.tenant_id, p.id, m.authz_revision
    FROM identity.sessions AS login
    JOIN identity.principals AS p ON p.id = login.principal_id AND p.active
    JOIN identity.memberships AS m ON m.principal_id = p.id AND m.active
    JOIN control.tenants AS t ON t.id = m.tenant_id AND t.active
    JOIN control.stores AS s ON s.tenant_id = t.id AND s.id = p_store AND s.active
    JOIN identity.store_grants AS g ON g.tenant_id = s.tenant_id
        AND g.store_id = s.id AND g.principal_id = p.id AND g.permission = p_permission
    WHERE login.token_hash = p_hash AND login.audience = 'merchant'
        AND login.revoked_at IS NULL AND login.expires_at > statement_timestamp()
$$;
ALTER FUNCTION identity.resolve_scope(bytea, uuid, text) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.resolve_scope(bytea, uuid, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.resolve_scope(bytea, uuid, text) TO commerce_runtime;
GRANT USAGE ON SCHEMA identity, control, ops, river TO commerce_runtime;
GRANT SELECT ON control.stores TO commerce_runtime;
GRANT SELECT, INSERT ON ops.audit_events TO commerce_runtime;
