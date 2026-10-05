-- 0121 live-console message templates (contracts/live-console-v1.md §3.4/§3.5/§7.3, §11 /message-templates; FROZEN 2026-10-05).
--
-- Purpose: the merchant message-template vocabulary of W2-05B — template ids/versions with the frozen
-- {template_id, version, public_safe, kinds} shape, the two system-fixed templates (order-pay-link/v1 with the
-- {{連結}} placeholder, offer-recommend/v1 with product/variant/keyword/live-price placeholders), the publish
-- (live:manage, audit template.published) and list (inbox:reply) merchant definers, and the resolve definer the
-- LC-B4 send path uses to look up a published (fixed or merchant) template by id + version.
--
-- Non-goals: no new permission (reuses live:manage + inbox:reply from 0119), no send/planning (§4 belongs to
-- LC-B4 / 0123), no outbound_messages/send_secrets/bundle_peers (0123), no UI, no template edit/delete (published
-- versions are append-only; a correction is a new version), no consumer/classifier change.
--
-- Depends on: 0001 (ops.audit_events, ops.command_results, identity store_grants/memberships), 0119
-- (inbox.principal_holds guard, the inbox:reply permission and role bundles).
--
-- Used by: internal/msgtemplates (publish/list/resolve definer callers + the §3.5 public-safe validator),
-- internal/httpapi/templates.go (POST/GET /message-templates), and LC-B4's send path via msgtemplates.resolve.
-- Roles: commerce_runtime reaches the merchant definers only; commerce_msgtemplates_writer is the NOLOGIN definer owner.

-- ---------------------------------------------------------------------------------------
-- Preconditions. 0121 rides on 0119's guard and permission; re-deriving the permission
-- CHECK is not needed (no new permission is added here), but the guard must exist.
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_perm text;
BEGIN
    IF to_regprocedure('inbox.principal_holds(text[])') IS NULL THEN
        RAISE EXCEPTION '0121 requires inbox.principal_holds(text[]) (migration 0119)';
    END IF;
    IF NOT EXISTS(SELECT 1 FROM pg_constraint c
        WHERE c.conrelid='identity.store_grants'::regclass AND c.conname='store_grants_permission_check'
          AND pg_get_constraintdef(c.oid) LIKE '%''inbox:reply''%') THEN
        RAISE EXCEPTION '0121 requires the inbox:reply permission (migration 0119)';
    END IF;
    FOREACH v_perm IN ARRAY ARRAY['commerce_runtime','commerce_auth'] LOOP
        IF to_regrole(v_perm) IS NULL THEN RAISE EXCEPTION '0121 requires role %',v_perm; END IF;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------
-- Role and schema. commerce_msgtemplates_writer is the NOLOGIN owner of the merchant
-- template definers (the same single-authority shape as commerce_inbox_writer of 0119).
-- FORCE RLS: the definer body is the control; the runtime login never touches the tables.
-- ---------------------------------------------------------------------------------------
CREATE ROLE commerce_msgtemplates_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
COMMENT ON ROLE commerce_msgtemplates_writer IS
 '0121 NOLOGIN definer owner of the merchant message-template writes/reads (publish/list/resolve). Never a login; touches msgtemplates.templates and msgtemplates.fixed_templates only through its fixed functions.';

CREATE SCHEMA msgtemplates;
REVOKE ALL ON SCHEMA msgtemplates FROM PUBLIC;
GRANT USAGE ON SCHEMA msgtemplates TO commerce_msgtemplates_writer, commerce_runtime;

-- Merchant-published templates, append-only (a correction is a new version, never an UPDATE).
CREATE TABLE msgtemplates.templates (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    template_id text NOT NULL CHECK (template_id ~ '^[a-z0-9][a-z0-9/_-]{0,63}$'),
    version bigint NOT NULL CHECK (version > 0),
    name text NOT NULL CHECK (name <> '' AND char_length(name) <= 120),
    body text NOT NULL CHECK (body <> '' AND char_length(body) <= 2000),
    kinds text[] NOT NULL CHECK (kinds <> '{}'::text[] AND kinds <@ ARRAY['dm','private_reply','public_reply','recommend']::text[]),
    public_safe boolean NOT NULL DEFAULT false,
    created_by uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, template_id, version),
    -- A template usable for public replies or the recommend comment must be flagged public_safe (§3.5). This
    -- invariant is a DB CHECK, so even a defective caller cannot record an unsafe public template.
    CONSTRAINT templates_public_kinds_safe CHECK (NOT (kinds && ARRAY['public_reply','recommend']::text[]) OR public_safe)
);

-- System-fixed templates (store-independent; resolved before merchant templates). version is pinned at 1.
CREATE TABLE msgtemplates.fixed_templates (
    template_id text NOT NULL CHECK (template_id IN ('order-pay-link/v1','offer-recommend/v1')),
    version bigint NOT NULL CHECK (version = 1),
    name text NOT NULL CHECK (name <> ''),
    body text NOT NULL CHECK (body <> ''),
    kinds text[] NOT NULL CHECK (kinds <> '{}'::text[]),
    public_safe boolean NOT NULL,
    PRIMARY KEY (template_id, version),
    CONSTRAINT fixed_public_kinds_safe CHECK (NOT (kinds && ARRAY['public_reply','recommend']::text[]) OR public_safe)
);

ALTER TABLE msgtemplates.templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE msgtemplates.templates FORCE ROW LEVEL SECURITY;
ALTER TABLE msgtemplates.fixed_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE msgtemplates.fixed_templates FORCE ROW LEVEL SECURITY;
REVOKE ALL ON msgtemplates.templates, msgtemplates.fixed_templates FROM PUBLIC;

GRANT SELECT, INSERT ON msgtemplates.templates TO commerce_msgtemplates_writer;
CREATE POLICY templates_writer ON msgtemplates.templates FOR ALL TO commerce_msgtemplates_writer USING (true) WITH CHECK (true);

GRANT SELECT ON msgtemplates.fixed_templates TO commerce_msgtemplates_writer;
CREATE POLICY fixed_templates_reader ON msgtemplates.fixed_templates FOR SELECT TO commerce_msgtemplates_writer USING (true);

-- §3.5 guard reuse: the merchant definers re-check the server-resolved principal through the 0119 guard. USAGE on
-- schema inbox is required to reference the schema-qualified function (EXECUTE alone is not enough, same as 0119).
GRANT USAGE ON SCHEMA inbox TO commerce_msgtemplates_writer;
GRANT EXECUTE ON FUNCTION inbox.principal_holds(text[]) TO commerce_msgtemplates_writer;

-- Audit: only template.published (the fixed action of the contract's API list row).
GRANT USAGE ON SCHEMA ops TO commerce_msgtemplates_writer;
GRANT INSERT ON ops.audit_events TO commerce_msgtemplates_writer;
CREATE POLICY audit_msgtemplates_insert ON ops.audit_events FOR INSERT TO commerce_msgtemplates_writer
 WITH CHECK (action = 'template.published');

-- ---------------------------------------------------------------------------------------
-- Fixed templates. order-pay-link/v1 is a DM-only payment-link template (§5.1 step 4): the {{連結}}
-- placeholder is sealed into the send secret by LC-B4, never persisted in a display copy; public_safe is false.
-- offer-recommend/v1 is the §7.3 recommend comment (§3.5): product name, variant, keyword, live price, no link;
-- public_safe is true and it is re-validated at send. Idempotent on re-apply.
-- ---------------------------------------------------------------------------------------
INSERT INTO msgtemplates.fixed_templates(template_id, version, name, body, kinds, public_safe) VALUES
 ('order-pay-link/v1', 1, '訂單付款連結', '您的訂單已建立，請點此連結完成付款：{{連結}}', ARRAY['dm'], false),
 ('offer-recommend/v1', 1, '推薦商品', '推薦商品：{{product.name}} {{variant}}，關鍵字「{{keyword}}」，直播價 {{live_price}}', ARRAY['recommend'], true)
ON CONFLICT (template_id, version) DO NOTHING;

-- ---------------------------------------------------------------------------------------
-- §11 /message-templates merchant definers. Owner commerce_msgtemplates_writer, EXECUTE commerce_runtime.
-- publish (live:manage) appends the next version and audits template.published; list (inbox:reply) returns the
-- latest published version of each template_id; resolve (inbox:reply OR live:manage) returns one published
-- version — fixed first, then merchant — for the LC-B4 send path. §3.5 content matching itself is Go-side
-- (internal/msgtemplates); these definers enforce the structural + permission + public_safe/kind invariants.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION msgtemplates.publish(p_template_id text, p_name text, p_body text, p_kinds text[], p_public_safe boolean)
RETURNS TABLE(template_id text, version bigint, public_safe boolean, kinds text[])
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
        v_principal uuid := nullif(current_setting('app.principal_id', true), '')::uuid;
        v_version bigint;
BEGIN
    IF p_template_id IS NULL OR p_template_id !~ '^[a-z0-9][a-z0-9/_-]{0,63}$' THEN
        RAISE EXCEPTION 'invalid_template_id' USING ERRCODE = 'PT422';
    END IF;
    IF p_name IS NULL OR p_name = '' OR char_length(p_name) > 120 THEN
        RAISE EXCEPTION 'invalid_template_name' USING ERRCODE = 'PT422';
    END IF;
    IF p_body IS NULL OR p_body = '' OR char_length(p_body) > 2000 THEN
        RAISE EXCEPTION 'invalid_template_body' USING ERRCODE = 'PT422';
    END IF;
    IF p_kinds IS NULL OR p_kinds = '{}'::text[] OR array_length(p_kinds, 1) > 4
        OR NOT p_kinds <@ ARRAY['dm','private_reply','public_reply','recommend']::text[] THEN
        RAISE EXCEPTION 'invalid_kinds' USING ERRCODE = 'PT422';
    END IF;
    IF (p_kinds && ARRAY['public_reply','recommend']::text[]) AND NOT p_public_safe THEN
        RAISE EXCEPTION 'public_template_not_public_safe' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['live:manage']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    IF EXISTS(SELECT 1 FROM msgtemplates.fixed_templates f WHERE f.template_id = p_template_id) THEN
        RAISE EXCEPTION 'template_fixed' USING ERRCODE = 'PT409';
    END IF;
    SELECT coalesce(max(t.version), 0) + 1 INTO v_version
      FROM msgtemplates.templates t
     WHERE t.tenant_id = v_tenant AND t.store_id = v_store AND t.template_id = p_template_id;
    INSERT INTO msgtemplates.templates(tenant_id, store_id, template_id, version, name, body, kinds, public_safe, created_by)
     VALUES (v_tenant, v_store, p_template_id, v_version, p_name, p_body, p_kinds, p_public_safe, v_principal);
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action)
     VALUES (v_tenant, v_store, v_principal, 'template.published');
    RETURN QUERY SELECT p_template_id, v_version, p_public_safe, p_kinds;
END $$;

CREATE FUNCTION msgtemplates.list()
RETURNS TABLE(template_id text, version bigint, name text, kinds text[], public_safe boolean, created_at timestamptz)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
BEGIN
    IF NOT inbox.principal_holds(ARRAY['inbox:reply']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    RETURN QUERY
    SELECT t.template_id, t.version, t.name, t.kinds, t.public_safe, t.created_at
      FROM msgtemplates.templates t
     WHERE t.tenant_id = v_tenant AND t.store_id = v_store
       AND t.version = (SELECT max(x.version) FROM msgtemplates.templates x
                        WHERE x.tenant_id = t.tenant_id AND x.store_id = t.store_id AND x.template_id = t.template_id)
     ORDER BY t.template_id;
END $$;

CREATE FUNCTION msgtemplates.resolve(p_template_id text, p_version bigint)
RETURNS TABLE(template_id text, version bigint, name text, body text, kinds text[], public_safe boolean, fixed boolean)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
BEGIN
    IF p_template_id IS NULL OR p_version IS NULL OR p_version < 1 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['inbox:reply','live:manage']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    RETURN QUERY
    SELECT f.template_id, f.version, f.name, f.body, f.kinds, f.public_safe, true
      FROM msgtemplates.fixed_templates f
     WHERE f.template_id = p_template_id AND f.version = p_version
    UNION ALL
    SELECT t.template_id, t.version, t.name, t.body, t.kinds, t.public_safe, false
      FROM msgtemplates.templates t
     WHERE t.tenant_id = v_tenant AND t.store_id = v_store
       AND t.template_id = p_template_id AND t.version = p_version;
END $$;

DO $$
DECLARE f text;
BEGIN
    FOREACH f IN ARRAY ARRAY['msgtemplates.publish(text,text,text,text[],boolean)',
      'msgtemplates.list()','msgtemplates.resolve(text,bigint)'] LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_msgtemplates_writer', f);
        EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC', f);
        EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime', f);
    END LOOP;
END $$;
