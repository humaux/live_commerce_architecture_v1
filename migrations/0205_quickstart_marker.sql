-- Purpose: OC-05 persistent per-store quick-start completion; existing stores are opted out once at upgrade.
-- Depends on: control.stores, commerce_runtime and server-authenticated transaction-local app tenant/store GUCs.
-- Used by: internal/quickstart through internal/httpapi; contracts/merchant-settings-wizard-v1.md AD-03.
-- Invariants: derive steps from existing domain data; only completion is stored. Runtime cannot update/delete a
-- marker or choose its timestamp. No SECURITY DEFINER is needed: normal scoped runtime RLS is retained.

-- Serialize the upgrade boundary with store creation, so no store can slip between backfill and commit.
LOCK TABLE control.stores IN SHARE ROW EXCLUSIVE MODE;

CREATE TABLE control.quickstart_markers (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    completed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id)
);

-- Migration-owner-only backfill. A missing row after this transaction means a newly created store.
INSERT INTO control.quickstart_markers(tenant_id, store_id)
SELECT tenant_id, id FROM control.stores;

ALTER TABLE control.quickstart_markers ENABLE ROW LEVEL SECURITY;
ALTER TABLE control.quickstart_markers FORCE ROW LEVEL SECURITY;
CREATE POLICY quickstart_scoped_read ON control.quickstart_markers FOR SELECT TO commerce_runtime
USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
   AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);
CREATE POLICY quickstart_scoped_insert ON control.quickstart_markers FOR INSERT TO commerce_runtime
WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
        AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);

GRANT SELECT ON control.quickstart_markers TO commerce_runtime;
GRANT INSERT(tenant_id, store_id) ON control.quickstart_markers TO commerce_runtime;

COMMENT ON TABLE control.quickstart_markers IS
 'internal/quickstart OC-05: one immutable completion/dismissal timestamp per store. Old stores backfilled by 0205; missing row means incomplete. GET store:read; POST integration:manage, checked before/after work by the service. No client step state, no runtime timestamp/update/delete authority; scoped RLS.';
