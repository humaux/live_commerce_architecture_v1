-- Purpose: disposable W1 persistence with one trusted-scope Inbox and intent boundary.
-- Depends on: PostgreSQL 18 and an empty, task-owned lc_neutral_w1_test database.
-- Used by: check.mjs and the neutral W1 adapters.
CREATE ROLE l01_w1_owner NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE l01_w1_ingress NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE l01_w1_reader NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOINHERIT;
REVOKE ALL ON DATABASE lc_neutral_w1_test FROM PUBLIC;
GRANT CONNECT ON DATABASE lc_neutral_w1_test TO l01_w1_ingress, l01_w1_reader;
CREATE SCHEMA l01_w1 AUTHORIZATION l01_w1_owner;
SET ROLE l01_w1_owner;

CREATE TABLE l01_w1.binding (
    app_id text NOT NULL,
    object text NOT NULL,
    asset_id text NOT NULL,
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    binding_id uuid NOT NULL UNIQUE,
    active boolean NOT NULL,
    PRIMARY KEY (app_id, object, asset_id),
    UNIQUE (app_id, object, asset_id, tenant_id, store_id, binding_id)
);

CREATE TABLE l01_w1.inbox (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    app_id text NOT NULL,
    object text NOT NULL,
    asset_id text NOT NULL,
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    binding_id uuid NOT NULL,
    external_key text NOT NULL,
    body_sha256 text NOT NULL CHECK (body_sha256 ~ '^[0-9a-f]{64}$'),
    FOREIGN KEY (app_id, object, asset_id, tenant_id, store_id, binding_id)
        REFERENCES l01_w1.binding (app_id, object, asset_id, tenant_id, store_id, binding_id),
    UNIQUE (app_id, object, asset_id, tenant_id, store_id, external_key)
);

CREATE TABLE l01_w1.intent (
    inbox_id bigint PRIMARY KEY REFERENCES l01_w1.inbox (id)
);

CREATE TABLE l01_w1.quarantine (
    id uuid NOT NULL UNIQUE,
    app_id text NOT NULL,
    object text NOT NULL,
    asset_id text NOT NULL,
    external_key text NOT NULL,
    body_sha256 text NOT NULL CHECK (body_sha256 ~ '^[0-9a-f]{64}$'),
    key_id text NOT NULL CHECK (key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    nonce bytea NOT NULL CHECK (octet_length(nonce) = 12),
    ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) BETWEEN 17 AND 1048592),
    PRIMARY KEY (app_id, object, asset_id, external_key, body_sha256)
);

CREATE FUNCTION l01_w1.admit(
    p_app text, p_object text, p_asset text, p_external_key text, p_body_sha256 text,
    p_quarantine_id uuid DEFAULT NULL, p_key_id text DEFAULT NULL,
    p_nonce bytea DEFAULT NULL, p_ciphertext bytea DEFAULT NULL
) RETURNS text
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
    b l01_w1.binding%ROWTYPE;
    new_id bigint;
    existing_digest text;
BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'W1 requires READ COMMITTED' USING ERRCODE = '25001';
    END IF;
    IF p_app IS NULL OR p_app !~ '^[0-9]{1,40}$' OR p_object IS DISTINCT FROM 'page'
        OR p_asset IS NULL OR p_asset !~ '^[0-9]{1,40}$'
        OR p_external_key IS NULL OR octet_length(p_external_key) NOT BETWEEN 1 AND 1024
        OR p_body_sha256 IS NULL OR p_body_sha256 !~ '^[0-9a-f]{64}$'
        OR num_nonnulls(p_quarantine_id, p_key_id, p_nonce, p_ciphertext) NOT IN (0, 4)
    THEN
        RAISE EXCEPTION 'invalid W1 event' USING ERRCODE = '22023';
    END IF;

    SELECT * INTO b FROM l01_w1.binding
        WHERE app_id = p_app AND object = p_object AND asset_id = p_asset
        FOR SHARE;
    IF b.active IS NOT TRUE THEN
        IF p_quarantine_id IS NULL THEN
            RETURN 'needs_quarantine';
        END IF;
        IF p_key_id !~ '^[A-Za-z0-9_-]{1,64}$' OR octet_length(p_nonce) <> 12
            OR octet_length(p_ciphertext) NOT BETWEEN 17 AND 1048592
        THEN
            RAISE EXCEPTION 'invalid W1 quarantine envelope' USING ERRCODE = '22023';
        END IF;
        INSERT INTO l01_w1.quarantine (id, app_id, object, asset_id, external_key, body_sha256, key_id, nonce, ciphertext)
            VALUES (p_quarantine_id, p_app, p_object, p_asset, p_external_key, p_body_sha256, p_key_id, p_nonce, p_ciphertext)
            ON CONFLICT (app_id, object, asset_id, external_key, body_sha256) DO NOTHING;
        RETURN 'quarantined';
    END IF;
    IF p_quarantine_id IS NOT NULL THEN
        RAISE EXCEPTION 'sealed envelope for active W1 binding' USING ERRCODE = '22023';
    END IF;

    INSERT INTO l01_w1.inbox (app_id, object, asset_id, tenant_id, store_id, binding_id, external_key, body_sha256)
        VALUES (p_app, p_object, p_asset, b.tenant_id, b.store_id, b.binding_id, p_external_key, p_body_sha256)
        ON CONFLICT (app_id, object, asset_id, tenant_id, store_id, external_key) DO NOTHING
        RETURNING id INTO new_id;
    IF new_id IS NOT NULL THEN
        INSERT INTO l01_w1.intent (inbox_id) VALUES (new_id);
        RETURN 'routed';
    END IF;

    -- At READ COMMITTED this statement sees the winner after ON CONFLICT waits.
    SELECT body_sha256 INTO STRICT existing_digest FROM l01_w1.inbox
        WHERE app_id = p_app AND object = p_object AND asset_id = p_asset
          AND tenant_id = b.tenant_id AND store_id = b.store_id AND external_key = p_external_key;
    IF existing_digest <> p_body_sha256 THEN
        RETURN 'conflict';
    END IF;
    RETURN 'replay';
END;
$$;

REVOKE ALL ON SCHEMA l01_w1 FROM PUBLIC;
REVOKE ALL ON FUNCTION l01_w1.admit(text, text, text, text, text, uuid, text, bytea, bytea) FROM PUBLIC;
GRANT USAGE ON SCHEMA l01_w1 TO l01_w1_ingress, l01_w1_reader;
GRANT EXECUTE ON FUNCTION l01_w1.admit(text, text, text, text, text, uuid, text, bytea, bytea) TO l01_w1_ingress;
GRANT SELECT ON ALL TABLES IN SCHEMA l01_w1 TO l01_w1_reader;
RESET ROLE;
