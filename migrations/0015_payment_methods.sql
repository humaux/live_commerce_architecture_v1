-- Merchant payment-method configuration is not provider admission. Until the
-- adapter + account entitlement gate exists, all methods are disabled drafts.
ALTER TABLE integration.merchant_accounts ADD CONSTRAINT account_method_target_unique
 UNIQUE(tenant_id,store_id,id,provider,environment);

CREATE SCHEMA payments;
REVOKE ALL ON SCHEMA payments FROM PUBLIC;
GRANT USAGE ON SCHEMA payments TO commerce_runtime;

CREATE TABLE payments.method_versions (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,market_id uuid NOT NULL,
 country text NOT NULL CHECK(country='TW'),
 code text NOT NULL CHECK(code IN ('payuni_credit','payuni_installment','payuni_atm','payuni_cvs','payuni_linepay')),
 version bigint NOT NULL CHECK(version>0),
 provider text NOT NULL CHECK(provider='payuni'),
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 connection_id uuid,binding_version bigint,
 currency text NOT NULL CHECK(currency='TWD'),
 name_hans text NOT NULL CHECK(char_length(name_hans) BETWEEN 1 AND 120 AND btrim(name_hans)<>'' AND name_hans !~ '[[:cntrl:]]'),
 name_hant text NOT NULL CHECK(char_length(name_hant) BETWEEN 1 AND 120 AND btrim(name_hant)<>'' AND name_hant !~ '[[:cntrl:]]'),
 name_en text NOT NULL CHECK(char_length(name_en) BETWEEN 1 AND 120 AND btrim(name_en)<>'' AND name_en !~ '[[:cntrl:]]'),
 enabled boolean NOT NULL CONSTRAINT method_not_admitted CHECK(NOT enabled),
 visible boolean NOT NULL,sort_order integer NOT NULL CHECK(sort_order BETWEEN 0 AND 1000),
 min_amount_minor bigint NOT NULL CHECK(min_amount_minor BETWEEN 1 AND 1000000000000),
 max_amount_minor bigint NOT NULL CHECK(max_amount_minor BETWEEN min_amount_minor AND 1000000000000),
 principal_id uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,market_id,country,code,version),
 CONSTRAINT method_connection_pair CHECK((connection_id IS NULL AND binding_version IS NULL)
  OR (connection_id IS NOT NULL AND binding_version IS NOT NULL AND binding_version>0)),
 CONSTRAINT method_market_target_fk FOREIGN KEY(tenant_id,store_id,market_id,currency)
  REFERENCES pricing.markets(tenant_id,store_id,id,currency),
 CONSTRAINT method_account_target_fk FOREIGN KEY(tenant_id,store_id,connection_id,provider,environment)
  REFERENCES integration.merchant_accounts(tenant_id,store_id,id,provider,environment),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);

CREATE TABLE payments.method_heads (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,market_id uuid NOT NULL,
 country text NOT NULL,code text NOT NULL,current_version bigint NOT NULL,
 PRIMARY KEY(tenant_id,store_id,market_id,country,code),
 FOREIGN KEY(tenant_id,store_id,market_id,country,code,current_version)
  REFERENCES payments.method_versions(tenant_id,store_id,market_id,country,code,version)
);
ALTER TABLE payments.method_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments.method_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE payments.method_heads ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments.method_heads FORCE ROW LEVEL SECURITY;

CREATE POLICY method_read ON payments.method_versions FOR SELECT TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY method_insert ON payments.method_versions FOR INSERT TO commerce_runtime
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
 AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY method_head_read ON payments.method_heads FOR SELECT TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY method_head_insert ON payments.method_heads FOR INSERT TO commerce_runtime
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY method_head_update ON payments.method_heads FOR UPDATE TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
 AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT,INSERT ON payments.method_versions,payments.method_heads TO commerce_runtime;
GRANT UPDATE(current_version) ON payments.method_heads TO commerce_runtime;
-- No direct buyer/checkout/worker grants and no mutable history. A later payment
-- attempt freezes a revision; toggling settings must never rewrite its facts.
