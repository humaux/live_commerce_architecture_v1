-- Merchant-only delivery settings. Carrier API drafts cannot be enabled yet.
-- Reuse pricing's single calculator with a distinct policy key per service.
ALTER TABLE pricing.policy_versions DROP CONSTRAINT policy_versions_method_check;
ALTER TABLE pricing.policy_versions ADD CONSTRAINT policy_versions_method_check CHECK
 (method IN ('home','cvs_711','cvs_familymart') OR method ~ '^delivery:[a-z][a-z0-9_-]{0,39}$');

CREATE SCHEMA fulfillment;
REVOKE ALL ON SCHEMA fulfillment FROM PUBLIC;
GRANT USAGE ON SCHEMA fulfillment TO commerce_runtime;

CREATE TABLE fulfillment.service_versions (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,market_id uuid NOT NULL,
 country text NOT NULL CHECK(country ~ '^[A-Z]{2}$'),
 code text NOT NULL CHECK(code ~ '^[a-z][a-z0-9_-]{0,39}$'),
 version bigint NOT NULL CHECK(version>0),
 policy_method text GENERATED ALWAYS AS ('delivery:' || code) STORED,
 policy_version bigint NOT NULL CHECK(policy_version>0),currency text NOT NULL,
 name_hans text NOT NULL CHECK(char_length(btrim(name_hans)) BETWEEN 1 AND 120 AND name_hans !~ '[[:cntrl:]]'),
 name_hant text NOT NULL CHECK(char_length(btrim(name_hant)) BETWEEN 1 AND 120 AND name_hant !~ '[[:cntrl:]]'),
 name_en text NOT NULL CHECK(char_length(btrim(name_en)) BETWEEN 1 AND 120 AND name_en !~ '[[:cntrl:]]'),
 delivery_kind text NOT NULL CHECK(delivery_kind IN ('home','cvs_711','cvs_familymart')),
 mode text NOT NULL CHECK(mode IN ('MANUAL','API')),
 enabled boolean NOT NULL,visible boolean NOT NULL,
 sort_order integer NOT NULL CHECK(sort_order BETWEEN 0 AND 1000),
 binding_id uuid,binding_version bigint,
 principal_id uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,market_id,country,code,version),
 CHECK(delivery_kind='home' OR country='TW'),
 CHECK((binding_id IS NULL AND binding_version IS NULL) OR (binding_id IS NOT NULL AND binding_version>0)),
 CHECK(mode<>'MANUAL' OR binding_id IS NULL),
 -- This is capability absence, not a configurable "pretend-ready" bit.
 CHECK(mode<>'API' OR NOT enabled),
 FOREIGN KEY(tenant_id,store_id,market_id,country,policy_method,policy_version,currency)
  REFERENCES pricing.policy_versions(tenant_id,store_id,market_id,country,method,version,currency),
 FOREIGN KEY(tenant_id,store_id,binding_id) REFERENCES integration.bindings(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);

CREATE TABLE fulfillment.service_heads (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,market_id uuid NOT NULL,
 country text NOT NULL,code text NOT NULL,current_version bigint NOT NULL,
 PRIMARY KEY(tenant_id,store_id,market_id,country,code),
 FOREIGN KEY(tenant_id,store_id,market_id,country,code,current_version)
  REFERENCES fulfillment.service_versions(tenant_id,store_id,market_id,country,code,version)
);

ALTER TABLE fulfillment.service_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE fulfillment.service_versions FORCE ROW LEVEL SECURITY;
ALTER TABLE fulfillment.service_heads ENABLE ROW LEVEL SECURITY;
ALTER TABLE fulfillment.service_heads FORCE ROW LEVEL SECURITY;

CREATE POLICY merchant_read ON fulfillment.service_versions FOR SELECT TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_insert ON fulfillment.service_versions FOR INSERT TO commerce_runtime
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
CREATE POLICY merchant_read ON fulfillment.service_heads FOR SELECT TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_insert ON fulfillment.service_heads FOR INSERT TO commerce_runtime
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY merchant_update ON fulfillment.service_heads FOR UPDATE TO commerce_runtime
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);

GRANT SELECT,INSERT ON fulfillment.service_versions,fulfillment.service_heads TO commerce_runtime;
GRANT UPDATE(current_version) ON fulfillment.service_heads TO commerce_runtime;
-- No buyer/worker/issuer authority and no history mutation/delete grants.
