-- T08's first durable slice is planning only. No stream URL, provider effect,
-- worker grant or live-state claim exists here. See contracts/live-planning-v1.md.
ALTER TABLE identity.store_grants DROP CONSTRAINT store_grants_permission_check;
ALTER TABLE identity.store_grants ADD CONSTRAINT store_grants_permission_check CHECK
 (permission IN ('store:read','audit:read','audit:write','catalog:read','catalog:write',
 'inventory:read','inventory:write','inventory:reserve','pricing:read','pricing:write',
 'integration:manage','integration:execute','integration:read','orders:read',
 'live:read','live:manage'));
-- Deliberately no membership backfill or change to create_initial_store. Future
-- merchant live routes require an explicit provisioning decision, not elevation.

CREATE SCHEMA live;
REVOKE ALL ON SCHEMA live FROM PUBLIC;
GRANT USAGE ON SCHEMA live TO commerce_runtime;

CREATE TABLE live.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    principal_id uuid NOT NULL,
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200
        AND title=btrim(title) AND title !~ '[[:cntrl:]]'),
    scheduled_at timestamptz CHECK (scheduled_at >= TIMESTAMPTZ '2000-01-01 00:00:00+00'
        AND scheduled_at < TIMESTAMPTZ '2200-01-01 00:00:00+00'),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
    FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);

CREATE TABLE live.programs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    session_id uuid NOT NULL,
    principal_id uuid NOT NULL,
    aspect_ratio text NOT NULL CHECK (aspect_ratio IN ('16:9','9:16')),
    -- Configuration is not readiness or audience visibility. A later reviewed
    -- migration must introduce the start/attempt lifecycle before lifting this.
    state text NOT NULL DEFAULT 'DRAFT' CHECK (state='DRAFT'),
    UNIQUE (tenant_id,store_id,session_id),
    FOREIGN KEY (tenant_id,store_id,session_id) REFERENCES live.sessions(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);

ALTER TABLE live.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.sessions FORCE ROW LEVEL SECURITY;
ALTER TABLE live.programs ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.programs FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.sessions,live.programs FROM PUBLIC;

CREATE POLICY session_read ON live.sessions FOR SELECT TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY session_insert ON live.sessions FOR INSERT TO commerce_runtime WITH CHECK
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY session_update ON live.sessions FOR UPDATE TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);

CREATE POLICY program_read ON live.programs FOR SELECT TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY program_insert ON live.programs FOR INSERT TO commerce_runtime WITH CHECK
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid AND state='DRAFT');
CREATE POLICY program_update ON live.programs FOR UPDATE TO commerce_runtime USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND state='DRAFT');

-- Immutable IDs, ownership and creation timestamps cannot be changed by runtime.
-- No delete grant; no anonymous, buyer, integration-worker or auth-definer access.
GRANT SELECT,INSERT ON live.sessions,live.programs TO commerce_runtime;
GRANT UPDATE(title,scheduled_at,version,updated_at) ON live.sessions TO commerce_runtime;
GRANT UPDATE(aspect_ratio) ON live.programs TO commerce_runtime;
