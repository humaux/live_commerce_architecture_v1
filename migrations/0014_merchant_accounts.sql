-- Merchant-owned credentials are configuration, NOT provider approval. The
-- application encrypts before SQL; these tables never receive plaintext keys.
ALTER TABLE integration.bindings ADD CONSTRAINT binding_account_target_unique
 UNIQUE(tenant_id,store_id,id,provider,external_asset_id);

CREATE TABLE integration.merchant_accounts (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, principal_id uuid NOT NULL,
 provider text NOT NULL CHECK(provider='payuni'),
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 account_id text NOT NULL CHECK(account_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 binding_id uuid NOT NULL UNIQUE,
 binding_asset text GENERATED ALWAYS AS (environment || ':' || account_id) STORED,
 credential_version bigint NOT NULL CHECK(credential_version>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(tenant_id,store_id,id),
 UNIQUE(tenant_id,store_id,provider,environment,account_id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id),
 FOREIGN KEY(tenant_id,store_id,binding_id,provider,binding_asset)
  REFERENCES integration.bindings(tenant_id,store_id,id,provider,external_asset_id)
);

CREATE TABLE integration.account_credentials (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, connection_id uuid NOT NULL,
 version bigint NOT NULL CHECK(version>0),
 key_id text NOT NULL CHECK(key_id ~ '^[A-Za-z0-9_-]{1,40}$'),
 nonce bytea NOT NULL CHECK(octet_length(nonce)=12),
 ciphertext bytea NOT NULL CHECK(octet_length(ciphertext) BETWEEN 17 AND 8192),
 principal_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,connection_id,version),
 FOREIGN KEY(tenant_id,store_id,connection_id)
  REFERENCES integration.merchant_accounts(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
-- Head and encrypted version must commit together; insert head before version.
ALTER TABLE integration.merchant_accounts ADD CONSTRAINT account_current_credential_fk
 FOREIGN KEY(tenant_id,store_id,id,credential_version)
 REFERENCES integration.account_credentials(tenant_id,store_id,connection_id,version)
 DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE integration.merchant_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.merchant_accounts FORCE ROW LEVEL SECURITY;
ALTER TABLE integration.account_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.account_credentials FORCE ROW LEVEL SECURITY;

CREATE POLICY account_read ON integration.merchant_accounts FOR SELECT TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY account_insert ON integration.merchant_accounts FOR INSERT TO commerce_runtime
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY account_update ON integration.merchant_accounts FOR UPDATE TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY credential_metadata_read ON integration.account_credentials FOR SELECT TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY credential_insert ON integration.account_credentials FOR INSERT TO commerce_runtime
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);

GRANT SELECT,INSERT ON integration.merchant_accounts TO commerce_runtime;
GRANT UPDATE(credential_version,updated_at) ON integration.merchant_accounts TO commerce_runtime;
GRANT INSERT ON integration.account_credentials TO commerce_runtime;
GRANT SELECT(tenant_id,store_id,connection_id,version,key_id,principal_id,created_at)
 ON integration.account_credentials TO commerce_runtime;
-- No ciphertext/nonce SELECT, version UPDATE/DELETE, worker or buyer grants.
-- A future operation-scoped credential reader needs its own reviewed authority.
