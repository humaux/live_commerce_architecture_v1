-- 0137 platform Stripe for every store (contracts/stripe-platform-account-v1.md §3, owner decision 2026-10-06).
-- Purpose: one designated platform Stripe connection per environment collects card payments for many stores through
--   per-store DERIVED connections that point at it. Every money fact stays in the store's own tenant/store scope.
-- Depends on: 0014/0016 (accounts, credentials, qualifications, methods), 0061 (Stripe registry), 0062 (refund
--   loader), 0077 (live approvals, qualification revoke-only), 0096 (worker authorities: loader grants).
-- Used by: internal/payments/platformstripe (merchant GET/PUT payments/card), cmd/stripe-admin platform-* (operator),
--   internal/payments (worker loaders read the aad_* scope), post_river/0022 (start/webhook/hosted view deltas).
-- Invariants: I01 (store from the token, never metadata), I05 (account/currency/amount per attempt), I16 (kill
--   switches stop new starts only), I19 (PostgreSQL is the single source of truth).
-- Non-goals: no Connect/on_behalf_of, no plaintext key in SQL (derived credentials are BYTE COPIES of the platform
--   envelope, never re-encrypted), no settlement ledger (0138, W4-S2), no UI.
-- Status: MOCK + REAL_PG only; SANDBOX needs the owner's test keys; LIVE is owner-run.

DO $$
DECLARE v_n bigint;
BEGIN
 -- Migration path note (brief "Migration path"): this migration moves no data. Existing LIVE approvals on a
 -- connection that is not (yet) the platform connection are only counted, never failed on.
 SELECT count(*) INTO v_n FROM payments.stripe_live_approvals;
 IF v_n>0 THEN RAISE NOTICE '0137: % stripe_live_approvals rows exist; designate the owner platform connection with stripe-admin platform-designate', v_n; END IF;
END $$;

-- ---------------------------------------------------------------------------------------------------------
-- 3.2 widened objects
-- ---------------------------------------------------------------------------------------------------------
ALTER TABLE integration.merchant_accounts
 ADD COLUMN platform_connection_id uuid REFERENCES integration.merchant_accounts(id);
ALTER TABLE integration.merchant_accounts ADD CONSTRAINT merchant_accounts_platform_stripe_check
 CHECK(platform_connection_id IS NULL OR provider='stripe');
DROP INDEX integration.stripe_account_identity_unique;
-- Two primary registrations of one Stripe account are still refused; derived rows deliberately repeat the platform
-- account id (the explicit, designated sharing of contract §0.2).
CREATE UNIQUE INDEX stripe_account_identity_unique ON integration.merchant_accounts(environment,account_id)
 WHERE provider='stripe' AND platform_connection_id IS NULL;

ALTER TABLE integration.account_credentials
 ADD COLUMN sealed_version bigint CHECK(sealed_version IS NULL OR sealed_version>0);

ALTER TABLE payments.account_qualifications
 ADD COLUMN platform_qualification_id uuid REFERENCES payments.account_qualifications(id);
ALTER TABLE payments.account_qualifications DROP CONSTRAINT stripe_qualification_live_needs_approval_check;
ALTER TABLE payments.account_qualifications ADD CONSTRAINT stripe_qualification_live_needs_approval_check
 CHECK(code<>'stripe_checkout' OR proof_class<>'REAL_LIVE' OR live_approval_id IS NOT NULL OR platform_qualification_id IS NOT NULL);
ALTER TABLE payments.account_qualifications ADD CONSTRAINT stripe_qualification_one_link_check
 CHECK(NOT (live_approval_id IS NOT NULL AND platform_qualification_id IS NOT NULL));
ALTER TABLE payments.account_qualifications ADD CONSTRAINT stripe_qualification_platform_code_check
 CHECK(platform_qualification_id IS NULL OR code='stripe_checkout');

-- ---------------------------------------------------------------------------------------------------------
-- 3.1 new tables (FORCE RLS, PUBLIC revoked, no DELETE grant to anyone)
-- ---------------------------------------------------------------------------------------------------------
CREATE TABLE payments.stripe_platform (
 environment text PRIMARY KEY CHECK(environment IN ('SANDBOX','LIVE')),
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,connection_id uuid NOT NULL,
 provider text GENERATED ALWAYS AS ('stripe'::text) STORED,
 account_id text NOT NULL,
 display_name text NOT NULL CHECK(char_length(display_name) BETWEEN 2 AND 60 AND display_name !~ '[[:cntrl:]<>]'),
 descriptor_display text NOT NULL CHECK(descriptor_display ~ '^[A-Za-z0-9 .*-]{5,22}$'),
 enrollment_open boolean NOT NULL DEFAULT false,
 terms_version text NOT NULL CHECK(terms_version ~ '^[a-z0-9.-]{3,40}$'),
 platform_fee_bps integer NOT NULL DEFAULT 0 CHECK(platform_fee_bps BETWEEN 0 AND 3000),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(connection_id),
 FOREIGN KEY(tenant_id,store_id,connection_id,provider,environment,account_id)
  REFERENCES integration.merchant_accounts(tenant_id,store_id,id,provider,environment,account_id)
);
CREATE TABLE payments.platform_stripe_enrollments (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 connection_id uuid NOT NULL UNIQUE,
 enrolled_by uuid NOT NULL,
 terms_version text NOT NULL CHECK(terms_version ~ '^[a-z0-9.-]{3,40}$'),
 accepted_at timestamptz NOT NULL,
 descriptor_suffix text CHECK(descriptor_suffix ~ '^[A-Za-z0-9][A-Za-z0-9 .-]{1,9}$' AND descriptor_suffix ~ '[A-Za-z]'),
 blocked_at timestamptz,
 blocked_ref text CHECK(blocked_ref ~ '^[A-Za-z0-9._:-]{8,128}$'),
 blocked_by text CHECK(blocked_by ~ '^[A-Za-z0-9._:@-]{2,64}$'),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,environment),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,connection_id) REFERENCES integration.merchant_accounts(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,enrolled_by) REFERENCES identity.memberships(tenant_id,principal_id),
 CHECK((blocked_at IS NULL)=(blocked_ref IS NULL) AND (blocked_at IS NULL)=(blocked_by IS NULL))
);
DO $$ DECLARE v_table text; v_col record; BEGIN
 FOREACH v_table IN ARRAY ARRAY['stripe_platform','platform_stripe_enrollments'] LOOP
  EXECUTE format('ALTER TABLE payments.%I ENABLE ROW LEVEL SECURITY',v_table);
  EXECUTE format('ALTER TABLE payments.%I FORCE ROW LEVEL SECURITY',v_table);
  EXECUTE format('REVOKE ALL ON payments.%I FROM PUBLIC',v_table);
  EXECUTE format('COMMENT ON TABLE payments.%I IS %L',v_table,
   'payments owner (internal/payments/platformstripe, stripeadmin); commerce_payment_registry_writer definers write it, the checkout writer reads display columns only; no merchant, worker or ingress access; no key or approval data');
  FOR v_col IN SELECT column_name FROM information_schema.columns WHERE table_schema='payments' AND table_name=v_table LOOP
   EXECUTE format('COMMENT ON COLUMN payments.%I.%I IS %L',v_table,v_col.column_name,
    'payments owner; platform Stripe enrollment metadata (contracts/stripe-platform-account-v1.md §3.1); never PSP authority, metadata is not authority');
  END LOOP;
 END LOOP;
END $$;

GRANT SELECT,INSERT ON payments.stripe_platform,payments.platform_stripe_enrollments TO commerce_payment_registry_writer;
GRANT UPDATE(tenant_id,store_id,connection_id,account_id,display_name,descriptor_display,enrollment_open,terms_version,
 platform_fee_bps,version,updated_at) ON payments.stripe_platform TO commerce_payment_registry_writer;
GRANT UPDATE(enrolled_by,terms_version,accepted_at,descriptor_suffix,blocked_at,blocked_ref,blocked_by,version,updated_at)
 ON payments.platform_stripe_enrollments TO commerce_payment_registry_writer;
-- Platform rows are not tenant data: every registry definer (merchant enable runs in the merchant scope) reads them.
CREATE POLICY platform_registry_read ON payments.stripe_platform FOR SELECT TO commerce_payment_registry_writer USING(true);
CREATE POLICY platform_registry_insert ON payments.stripe_platform FOR INSERT TO commerce_payment_registry_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY platform_registry_update ON payments.stripe_platform FOR UPDATE TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
-- Enrollments: the operator fan-out enumerates every store (SELECT true), but every write names its own scope.
CREATE POLICY enrollment_registry_read ON payments.platform_stripe_enrollments FOR SELECT TO commerce_payment_registry_writer USING(true);
CREATE POLICY enrollment_registry_insert ON payments.platform_stripe_enrollments FOR INSERT TO commerce_payment_registry_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY enrollment_registry_update ON payments.platform_stripe_enrollments FOR UPDATE TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
-- The buyer-facing disclosure (contract §5): checkout definers read the display text and the store's own suffix.
GRANT SELECT(environment,connection_id,display_name,descriptor_display) ON payments.stripe_platform TO commerce_checkout_writer;
CREATE POLICY platform_checkout_read ON payments.stripe_platform FOR SELECT TO commerce_checkout_writer USING(true);
GRANT SELECT(tenant_id,store_id,environment,connection_id,descriptor_suffix) ON payments.platform_stripe_enrollments TO commerce_checkout_writer;
CREATE POLICY enrollment_checkout_read ON payments.platform_stripe_enrollments FOR SELECT TO commerce_checkout_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);

-- The registry definers copy the platform envelope and mirror the platform qualification: they must READ the
-- platform's own rows although the scope GUC names the merchant store. Each policy admits only rows that belong to the
-- designated platform connection.
CREATE POLICY platform_account_read ON integration.merchant_accounts FOR SELECT TO commerce_payment_registry_writer
 USING(EXISTS(SELECT 1 FROM payments.stripe_platform p WHERE p.connection_id=merchant_accounts.id));
CREATE POLICY platform_credential_read ON integration.account_credentials FOR SELECT TO commerce_payment_registry_writer
 USING(EXISTS(SELECT 1 FROM payments.stripe_platform p WHERE p.connection_id=account_credentials.connection_id));
CREATE POLICY platform_qualification_read ON payments.account_qualifications FOR SELECT TO commerce_payment_registry_writer
 USING(EXISTS(SELECT 1 FROM payments.stripe_platform p WHERE p.connection_id=account_qualifications.connection_id));
CREATE POLICY platform_approval_read ON payments.stripe_live_approvals FOR SELECT TO commerce_payment_registry_writer
 USING(EXISTS(SELECT 1 FROM payments.stripe_platform p WHERE p.connection_id=stripe_live_approvals.connection_id));
-- Which (market,country) pairs a store sells to: the enable definer creates one card head per pair.
GRANT SELECT(tenant_id,store_id,market_id,country) ON pricing.policy_versions TO commerce_payment_registry_writer;
CREATE POLICY stripe_registry_policy_read ON pricing.policy_versions FOR SELECT TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT EXECUTE ON FUNCTION identity.resolve_access(bytea,uuid,text) TO commerce_payment_registry_writer;

-- Revoke-only now also covers derived qualifications (contract §3.2).
DROP POLICY stripe_registry_qualification_revoke ON payments.account_qualifications;
CREATE POLICY stripe_registry_qualification_revoke ON payments.account_qualifications
 FOR UPDATE TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND code='stripe_checkout' AND (live_approval_id IS NOT NULL OR platform_qualification_id IS NOT NULL))
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
  AND code='stripe_checkout' AND (live_approval_id IS NOT NULL OR platform_qualification_id IS NOT NULL));
CREATE OR REPLACE FUNCTION payments.account_qualification_revoke_only() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW IS NOT DISTINCT FROM OLD THEN RETURN NEW; END IF; -- UPDATE(id) row-lock no-ops change nothing
 -- delta (0137): a platform-derived qualification is revoke-only too (kill switches, fan-out revoke).
 IF OLD.code='stripe_checkout' AND (OLD.live_approval_id IS NOT NULL OR OLD.platform_qualification_id IS NOT NULL)
  AND OLD.revoked_at IS NULL AND NEW.revoked_at IS NOT NULL
  AND to_jsonb(NEW)-'revoked_at' IS NOT DISTINCT FROM to_jsonb(OLD)-'revoked_at' THEN
  NEW.revoked_at:=clock_timestamp(); -- the supplied value is ignored
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'Stripe qualification is revoke-only' USING ERRCODE='PT409';
END $$;

-- ---------------------------------------------------------------------------------------------------------
-- Guard triggers (SECURITY DEFINER, owner registry writer; they read platform rows through the policies above)
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION integration.guard_derived_stripe_account() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE p payments.stripe_platform%ROWTYPE;
BEGIN
 IF TG_OP='UPDATE' AND OLD.platform_connection_id IS DISTINCT FROM NEW.platform_connection_id THEN
  RAISE EXCEPTION 'derived Stripe connection pointer is immutable' USING ERRCODE='42501'; END IF;
 IF NEW.platform_connection_id IS NULL THEN RETURN NEW; END IF;
 SELECT x.* INTO p FROM payments.stripe_platform x WHERE x.connection_id=NEW.platform_connection_id;
 IF NOT FOUND OR p.environment<>NEW.environment OR p.account_id<>NEW.account_id OR NEW.provider<>'stripe' THEN
  RAISE EXCEPTION 'derived Stripe connection must mirror the designated platform account' USING ERRCODE='42501'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION integration.guard_derived_stripe_account() OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION integration.guard_derived_stripe_account() FROM PUBLIC;
CREATE TRIGGER guard_derived_stripe_account BEFORE INSERT OR UPDATE ON integration.merchant_accounts
 FOR EACH ROW EXECUTE FUNCTION integration.guard_derived_stripe_account();

CREATE FUNCTION integration.guard_derived_stripe_credential() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a integration.merchant_accounts%ROWTYPE; p payments.stripe_platform%ROWTYPE;
 c integration.account_credentials%ROWTYPE;
BEGIN
 SELECT x.* INTO a FROM integration.merchant_accounts x WHERE x.tenant_id=NEW.tenant_id
  AND x.store_id=NEW.store_id AND x.id=NEW.connection_id;
 IF a.platform_connection_id IS NULL THEN
  IF NEW.sealed_version IS NOT NULL THEN
   RAISE EXCEPTION 'sealed_version belongs to derived Stripe credentials only' USING ERRCODE='42501'; END IF;
  RETURN NEW;
 END IF;
 -- A derived credential is a BYTE COPY of the platform envelope at sealed_version (never re-encrypted, no plaintext).
 SELECT x.* INTO p FROM payments.stripe_platform x WHERE x.connection_id=a.platform_connection_id;
 SELECT x.* INTO c FROM integration.account_credentials x WHERE x.tenant_id=p.tenant_id AND x.store_id=p.store_id
  AND x.connection_id=p.connection_id AND x.version=NEW.sealed_version;
 IF NEW.sealed_version IS NULL OR NOT FOUND OR c.key_id<>NEW.key_id OR c.nonce<>NEW.nonce
  OR c.ciphertext<>NEW.ciphertext THEN
  RAISE EXCEPTION 'derived Stripe credential must equal the platform envelope' USING ERRCODE='42501'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION integration.guard_derived_stripe_credential() OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION integration.guard_derived_stripe_credential() FROM PUBLIC;
CREATE TRIGGER guard_derived_stripe_credential BEFORE INSERT ON integration.account_credentials
 FOR EACH ROW EXECUTE FUNCTION integration.guard_derived_stripe_credential();

-- ---------------------------------------------------------------------------------------------------------
-- Owner amendment 2026-10-06 (AD-PF2): the platform account may collect ONLY for stores an operator allowlisted (the
-- platform owner's own store(s), same legal entity as the Stripe account). Third-party merchants stay refused
-- (platform_stripe_not_allowed) until plan A, a merchant-owned Taiwan PSP. Disable is never gated.
-- ---------------------------------------------------------------------------------------------------------
CREATE TABLE payments.platform_stripe_allowlist (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 allowed boolean NOT NULL,
 allowed_ref text NOT NULL CHECK(allowed_ref ~ '^[A-Za-z0-9._:-]{8,128}$'),
 allowed_by text NOT NULL CHECK(allowed_by ~ '^[A-Za-z0-9._:@-]{2,64}$'),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,environment),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
ALTER TABLE payments.platform_stripe_allowlist ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments.platform_stripe_allowlist FORCE ROW LEVEL SECURITY;
REVOKE ALL ON payments.platform_stripe_allowlist FROM PUBLIC;
COMMENT ON TABLE payments.platform_stripe_allowlist IS 'payments owner (stripeadmin platform-allow); registry writer definers only. Operator-attested list of stores that may self-enable platform Stripe card payments (owner amendment 2026-10-06); no merchant access; not a payment authority';
COMMENT ON COLUMN payments.platform_stripe_allowlist.allowed IS 'payments owner; true = the store may enable; false = withdrawn (existing sales stop only through platform-block)';
GRANT SELECT,INSERT ON payments.platform_stripe_allowlist TO commerce_payment_registry_writer;
GRANT UPDATE(allowed,allowed_ref,allowed_by,version,updated_at) ON payments.platform_stripe_allowlist TO commerce_payment_registry_writer;
-- read: any registry definer (the merchant enable runs in the merchant scope); writes name their own scope
CREATE POLICY allowlist_registry_read ON payments.platform_stripe_allowlist FOR SELECT TO commerce_payment_registry_writer USING(true);
CREATE POLICY allowlist_registry_insert ON payments.platform_stripe_allowlist FOR INSERT TO commerce_payment_registry_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY allowlist_registry_update ON payments.platform_stripe_allowlist FOR UPDATE TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);

-- ---------------------------------------------------------------------------------------------------------
-- 3.3 internal helpers (owner registry writer, NO EXECUTE grant to any login; reachable only from the definers below)
-- ---------------------------------------------------------------------------------------------------------
-- The platform's current valid qualification: head credential, unexpired, unrevoked, and (LIVE) tied to an active approval.
CREATE FUNCTION payments.platform_stripe_valid_qualification(p_environment text,p_proof text DEFAULT NULL)
RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_id uuid;
BEGIN
 SELECT q.id INTO v_id
 FROM payments.stripe_platform sp
 JOIN integration.merchant_accounts pa ON pa.tenant_id=sp.tenant_id AND pa.store_id=sp.store_id AND pa.id=sp.connection_id
 JOIN payments.account_qualifications q ON q.tenant_id=pa.tenant_id AND q.store_id=pa.store_id AND q.connection_id=pa.id
  AND q.credential_version=pa.credential_version AND q.environment=sp.environment AND q.code='stripe_checkout'
 WHERE sp.environment=p_environment AND q.revoked_at IS NULL AND q.observed_at<=clock_timestamp()
  AND q.expires_at>clock_timestamp() AND (p_proof IS NULL OR q.proof_class=p_proof)
  AND (p_environment='SANDBOX' OR EXISTS(SELECT 1 FROM payments.stripe_live_approvals ap
   WHERE ap.id=q.live_approval_id AND ap.revoked_at IS NULL))
 ORDER BY q.observed_at DESC,q.id LIMIT 1;
 RETURN v_id;
END $$;

-- §2: the platform state is derived, never stored.
CREATE FUNCTION payments.platform_stripe_state(p_environment text) RETURNS text
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE sp payments.stripe_platform%ROWTYPE; ap payments.stripe_live_approvals%ROWTYPE;
BEGIN
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND THEN RETURN 'NONE'; END IF;
 IF p_environment='LIVE' THEN
  SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.connection_id=sp.connection_id AND x.revoked_at IS NULL;
  IF NOT FOUND THEN
   RETURN CASE WHEN EXISTS(SELECT 1 FROM payments.stripe_live_approvals x WHERE x.connection_id=sp.connection_id)
    THEN 'REVOKED' ELSE 'DESIGNATED' END;
  END IF;
  IF ap.canary_verified_at IS NULL THEN RETURN 'DESIGNATED'; END IF;
 END IF;
 IF payments.platform_stripe_valid_qualification(p_environment) IS NULL THEN RETURN 'DESIGNATED'; END IF;
 RETURN CASE WHEN sp.enrollment_open THEN 'OPEN' ELSE 'CLOSED' END;
END $$;

-- Refresh one store's derived rows. The CALLER sets app.tenant_id/app.store_id/app.principal_id to that store (RLS).
-- Modes: 'enable' creates the derived account chain if missing and (re)creates one enabled card head per (market,country);
-- 'follow' (fan-out) syncs the credential, mirrors the platform qualification and moves ENABLED heads; 'disable' appends
-- disabled head versions. The envelope is copied byte-for-byte (sealed_version = platform head), never re-encrypted.
CREATE FUNCTION payments.platform_stripe_refresh_store(p_tenant uuid,p_store uuid,p_environment text,
 p_principal uuid,p_qualification uuid,p_mode text) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE sp payments.stripe_platform%ROWTYPE; pa integration.merchant_accounts%ROWTYPE;
 pq payments.account_qualifications%ROWTYPE; da integration.merchant_accounts%ROWTYPE;
 b integration.bindings%ROWTYPE; ap payments.stripe_live_approvals%ROWTYPE; m payments.method_versions%ROWTYPE;
 t record; h record; v_sealed bigint; v_next bigint; v_dq uuid; v_binding uuid; v_conn uuid;
 v_currency text; v_min bigint; v_max bigint; v_ver bigint; v_found boolean; v_count integer:=0;
 v_hans text; v_hant text; v_en text; v_sort integer;
BEGIN
 IF p_mode NOT IN ('enable','follow','disable') OR p_tenant IS NULL OR p_store IS NULL OR p_principal IS NULL
  OR p_environment NOT IN ('SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'invalid platform Stripe refresh' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO pa FROM integration.merchant_accounts x WHERE x.id=sp.connection_id;
 SELECT x.* INTO da FROM integration.merchant_accounts x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
  AND x.provider='stripe' AND x.environment=p_environment;
 IF NOT FOUND THEN
  IF p_mode<>'enable' THEN RAISE EXCEPTION 'enrollment_unavailable' USING ERRCODE='PT409'; END IF;
  v_binding:=gen_random_uuid(); v_conn:=gen_random_uuid();
  INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id)
  VALUES(v_binding,p_tenant,p_store,p_principal,'stripe',p_environment||':'||sp.account_id);
  INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,
   binding_id,credential_version,platform_connection_id)
  VALUES(v_conn,p_tenant,p_store,p_principal,'stripe',p_environment,sp.account_id,v_binding,1,sp.connection_id);
  INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,
   principal_id,sealed_version)
  SELECT p_tenant,p_store,v_conn,1,c.key_id,c.nonce,c.ciphertext,p_principal,pa.credential_version
  FROM integration.account_credentials c WHERE c.tenant_id=pa.tenant_id AND c.store_id=pa.store_id
   AND c.connection_id=pa.id AND c.version=pa.credential_version;
  IF NOT FOUND THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
  SELECT x.* INTO da FROM integration.merchant_accounts x WHERE x.id=v_conn AND x.tenant_id=p_tenant AND x.store_id=p_store;
 ELSIF da.platform_connection_id IS DISTINCT FROM sp.connection_id THEN
  RAISE EXCEPTION 'stripe_store_has_own_account' USING ERRCODE='PT409';
 ELSIF p_mode<>'disable' THEN
  -- credential sync: a platform rotation appended a head; copy it as this store's next version
  SELECT c.sealed_version INTO v_sealed FROM integration.account_credentials c WHERE c.tenant_id=p_tenant
   AND c.store_id=p_store AND c.connection_id=da.id AND c.version=da.credential_version;
  IF v_sealed IS NULL OR v_sealed<pa.credential_version THEN
   v_next:=da.credential_version+1;
   INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,
    principal_id,sealed_version)
   SELECT p_tenant,p_store,da.id,v_next,c.key_id,c.nonce,c.ciphertext,p_principal,pa.credential_version
   FROM integration.account_credentials c WHERE c.tenant_id=pa.tenant_id AND c.store_id=pa.store_id
    AND c.connection_id=pa.id AND c.version=pa.credential_version;
   IF NOT FOUND THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
   UPDATE integration.merchant_accounts SET credential_version=v_next,updated_at=clock_timestamp()
    WHERE tenant_id=p_tenant AND store_id=p_store AND id=da.id;
   da.credential_version:=v_next;
  END IF;
 END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.id=da.binding_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'Stripe binding unavailable' USING ERRCODE='PT409'; END IF;
 -- mirror the platform qualification (same proof class, same expiry, evidence = the platform qualification id)
 IF p_qualification IS NOT NULL AND p_mode<>'disable' THEN
  SELECT q.* INTO pq FROM payments.account_qualifications q WHERE q.id=p_qualification;
  IF NOT FOUND OR pq.connection_id<>sp.connection_id OR pq.environment<>sp.environment
   OR pq.credential_version<>pa.credential_version OR pq.revoked_at IS NOT NULL OR pq.expires_at<=clock_timestamp() THEN
   RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
  SELECT q.id INTO v_dq FROM payments.account_qualifications q WHERE q.tenant_id=p_tenant AND q.store_id=p_store
   AND q.connection_id=da.id AND q.platform_qualification_id=pq.id AND q.credential_version=da.credential_version
   AND q.revoked_at IS NULL;
  IF NOT FOUND THEN
   v_dq:=gen_random_uuid();
   INSERT INTO payments.account_qualifications(id,tenant_id,store_id,connection_id,credential_version,environment,
    code,proof_class,evidence_ref,observed_at,expires_at,platform_qualification_id)
   VALUES(v_dq,p_tenant,p_store,da.id,da.credential_version,da.environment,'stripe_checkout',pq.proof_class,
    'platform:'||pq.id::text,pq.observed_at,pq.expires_at,pq.id);
  END IF;
 END IF;
 IF p_mode='enable' THEN
  IF v_dq IS NULL THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
  IF p_environment='LIVE' THEN
   SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.connection_id=sp.connection_id AND x.revoked_at IS NULL;
   IF NOT FOUND THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
   v_currency:=ap.currency; v_max:=ap.max_minor;
  ELSE
   SELECT s.currency INTO v_currency FROM control.stores s WHERE s.tenant_id=p_tenant AND s.id=p_store;
   v_max:=CASE v_currency WHEN 'TWD' THEN 99999900 ELSE 99999999 END;
  END IF;
  v_min:=payments.stripe_min_minor(v_currency);
  IF v_min IS NULL OR NOT payments.stripe_amount_ok(v_currency,v_min) OR NOT payments.stripe_amount_ok(v_currency,v_max) THEN
   RAISE EXCEPTION 'currency_unsupported' USING ERRCODE='PT422'; END IF;
  FOR t IN SELECT DISTINCT mk.id AS market_id,pv.country FROM pricing.markets mk
   JOIN pricing.policy_versions pv ON pv.tenant_id=mk.tenant_id AND pv.store_id=mk.store_id AND pv.market_id=mk.id
   WHERE mk.tenant_id=p_tenant AND mk.store_id=p_store AND mk.active AND mk.currency=v_currency
   ORDER BY mk.id,pv.country LOOP
   v_count:=v_count+1;
   SELECT hd.current_version INTO v_ver FROM payments.method_heads hd WHERE hd.tenant_id=p_tenant
    AND hd.store_id=p_store AND hd.market_id=t.market_id AND hd.country=t.country AND hd.code='stripe_checkout' FOR UPDATE;
   v_found:=FOUND;
   v_hans:='信用卡'; v_hant:='信用卡'; v_en:='Card'; v_sort:=0;
   IF v_found THEN
    SELECT x.* INTO m FROM payments.method_versions x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
     AND x.market_id=t.market_id AND x.country=t.country AND x.code='stripe_checkout' AND x.version=v_ver;
    IF m.enabled AND m.visible AND m.connection_id=da.id AND m.qualification_id=v_dq
     AND m.binding_version=b.semantic_version AND m.min_amount_minor=v_min AND m.max_amount_minor=v_max THEN
     CONTINUE; -- already current: a replay creates nothing
    END IF;
    v_hans:=m.name_hans; v_hant:=m.name_hant; v_en:=m.name_en; v_sort:=m.sort_order;
   END IF;
   INSERT INTO payments.method_versions(tenant_id,store_id,market_id,country,code,version,provider,environment,
    connection_id,binding_version,currency,name_hans,name_hant,name_en,enabled,visible,sort_order,min_amount_minor,
    max_amount_minor,principal_id,qualification_id)
   VALUES(p_tenant,p_store,t.market_id,t.country,'stripe_checkout',coalesce(v_ver,0)+1,'stripe',da.environment,da.id,
    b.semantic_version,v_currency,v_hans,v_hant,v_en,true,true,v_sort,v_min,v_max,p_principal,v_dq);
   IF v_found THEN
    UPDATE payments.method_heads SET current_version=v_ver+1 WHERE tenant_id=p_tenant AND store_id=p_store
     AND market_id=t.market_id AND country=t.country AND code='stripe_checkout';
   ELSE
    INSERT INTO payments.method_heads(tenant_id,store_id,market_id,country,code,current_version)
    VALUES(p_tenant,p_store,t.market_id,t.country,'stripe_checkout',1);
   END IF;
  END LOOP;
  IF v_count=0 THEN RAISE EXCEPTION 'no_payment_market' USING ERRCODE='PT422'; END IF;
 ELSE
  -- follow: ENABLED heads move to the freshly mirrored qualification; disable: ENABLED heads get a disabled version.
  IF p_mode='disable' OR v_dq IS NOT NULL THEN
   FOR h IN SELECT hd.market_id,hd.country FROM payments.method_heads hd WHERE hd.tenant_id=p_tenant
    AND hd.store_id=p_store AND hd.code='stripe_checkout' ORDER BY hd.market_id,hd.country LOOP
    SELECT hd.current_version INTO v_ver FROM payments.method_heads hd WHERE hd.tenant_id=p_tenant
     AND hd.store_id=p_store AND hd.market_id=h.market_id AND hd.country=h.country AND hd.code='stripe_checkout' FOR UPDATE;
    SELECT x.* INTO m FROM payments.method_versions x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
     AND x.market_id=h.market_id AND x.country=h.country AND x.code='stripe_checkout' AND x.version=v_ver;
    IF NOT m.enabled OR (p_mode='follow' AND m.qualification_id IS NOT DISTINCT FROM v_dq
     AND m.binding_version=b.semantic_version) THEN CONTINUE; END IF;
    INSERT INTO payments.method_versions(tenant_id,store_id,market_id,country,code,version,provider,environment,
     connection_id,binding_version,currency,name_hans,name_hant,name_en,enabled,visible,sort_order,min_amount_minor,
     max_amount_minor,principal_id,qualification_id)
    VALUES(p_tenant,p_store,h.market_id,h.country,'stripe_checkout',v_ver+1,'stripe',da.environment,da.id,
     CASE WHEN p_mode='follow' THEN b.semantic_version ELSE m.binding_version END,m.currency,m.name_hans,m.name_hant,
     m.name_en,p_mode='follow',m.visible,m.sort_order,m.min_amount_minor,m.max_amount_minor,p_principal,
     CASE WHEN p_mode='follow' THEN v_dq ELSE m.qualification_id END);
    UPDATE payments.method_heads SET current_version=v_ver+1 WHERE tenant_id=p_tenant AND store_id=p_store
     AND market_id=h.market_id AND country=h.country AND code='stripe_checkout';
   END LOOP;
  END IF;
 END IF;
 RETURN da.id;
END $$;

-- §3.3 platform_stripe_fanout: runs inside the platform operation's transaction (rotate / qualify / live-revoke).
-- Bounded to 2000 enrollments; a blocked store is skipped (its derived rows were already revoked).
CREATE FUNCTION payments.platform_stripe_fanout(p_environment text,p_reason text) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE e record; v_n integer; v_done integer:=0; v_pq uuid; v_t text; v_s text; v_p text;
BEGIN
 IF p_environment NOT IN ('SANDBOX','LIVE') OR p_reason NOT IN ('rotate','qualify','revoke') THEN
  RAISE EXCEPTION 'invalid platform Stripe fan-out' USING ERRCODE='22023'; END IF;
 SELECT count(*) INTO v_n FROM payments.platform_stripe_enrollments x
  WHERE x.environment=p_environment AND x.blocked_at IS NULL;
 IF v_n>2000 THEN RAISE EXCEPTION 'platform_fanout_too_large' USING ERRCODE='PT409'; END IF;
 v_t:=coalesce(current_setting('app.tenant_id',true),''); v_s:=coalesce(current_setting('app.store_id',true),'');
 v_p:=coalesce(current_setting('app.principal_id',true),'');
 v_pq:=CASE WHEN p_reason='qualify' THEN payments.platform_stripe_valid_qualification(p_environment) END;
 FOR e IN SELECT x.tenant_id,x.store_id,x.environment,x.connection_id,x.enrolled_by
  FROM payments.platform_stripe_enrollments x WHERE x.environment=p_environment AND x.blocked_at IS NULL
  ORDER BY x.tenant_id,x.store_id LOOP
  PERFORM set_config('app.tenant_id',e.tenant_id::text,true),set_config('app.store_id',e.store_id::text,true),
   set_config('app.principal_id',e.enrolled_by::text,true);
  -- lock per row AFTER the scope is set: the UPDATE policy of the enrollment table is scope-bound
  PERFORM 1 FROM payments.platform_stripe_enrollments x WHERE x.tenant_id=e.tenant_id AND x.store_id=e.store_id
   AND x.environment=e.environment FOR UPDATE;
  IF p_reason='revoke' THEN
   UPDATE payments.account_qualifications SET revoked_at=clock_timestamp() WHERE tenant_id=e.tenant_id
    AND store_id=e.store_id AND connection_id=e.connection_id AND platform_qualification_id IS NOT NULL
    AND revoked_at IS NULL;
  ELSE
   PERFORM payments.platform_stripe_refresh_store(e.tenant_id,e.store_id,e.environment,e.enrolled_by,v_pq,'follow');
  END IF;
  v_done:=v_done+1;
 END LOOP;
 PERFORM set_config('app.tenant_id',v_t,true),set_config('app.store_id',v_s,true),set_config('app.principal_id',v_p,true);
 RETURN v_done;
END $$;

-- Merchant-facing summary of one store (the caller's scope GUCs name that store).
CREATE FUNCTION payments.platform_stripe_summary(p_tenant uuid,p_store uuid,p_environment text) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE sp payments.stripe_platform%ROWTYPE; e payments.platform_stripe_enrollments%ROWTYPE; ap payments.stripe_live_approvals%ROWTYPE;
 v_has_sp boolean; v_has_e boolean; v_state text; v_store_state text; v_currency text; v_min bigint; v_max bigint; v_preview text;
BEGIN
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment; v_has_sp:=FOUND;
 SELECT x.* INTO e FROM payments.platform_stripe_enrollments x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
  AND x.environment=p_environment; v_has_e:=FOUND;
 v_state:=payments.platform_stripe_state(p_environment);
 v_store_state:=CASE WHEN NOT v_has_e THEN 'NONE' WHEN e.blocked_at IS NOT NULL THEN 'BLOCKED'
  WHEN EXISTS(SELECT 1 FROM payments.method_heads hd JOIN payments.method_versions mv ON mv.tenant_id=hd.tenant_id
   AND mv.store_id=hd.store_id AND mv.market_id=hd.market_id AND mv.country=hd.country AND mv.code=hd.code
   AND mv.version=hd.current_version WHERE hd.tenant_id=p_tenant AND hd.store_id=p_store AND hd.code='stripe_checkout'
   AND mv.enabled) THEN 'ENABLED' ELSE 'DISABLED' END;
 IF v_has_sp THEN
  IF p_environment='LIVE' THEN
   SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.connection_id=sp.connection_id AND x.revoked_at IS NULL;
   IF FOUND THEN v_currency:=ap.currency; v_max:=ap.max_minor; END IF;
  ELSE
   SELECT s.currency INTO v_currency FROM control.stores s WHERE s.tenant_id=p_tenant AND s.id=p_store;
   v_max:=CASE v_currency WHEN 'TWD' THEN 99999900 ELSE 99999999 END;
  END IF;
  v_min:=payments.stripe_min_minor(v_currency);
  v_preview:=sp.descriptor_display||CASE WHEN v_has_e AND e.descriptor_suffix IS NOT NULL THEN '* '||e.descriptor_suffix ELSE '' END;
 END IF;
 RETURN jsonb_build_object('platform_state',v_state,'store_state',v_store_state,
  'terms_version',CASE WHEN v_has_sp THEN sp.terms_version END,
  'accepted_terms_version',CASE WHEN v_has_e THEN e.terms_version END,
  'display_name',CASE WHEN v_has_sp THEN sp.display_name END,'descriptor_preview',v_preview,
  'currency',v_currency,'min_minor',v_min,'max_minor',v_max,'version',CASE WHEN v_has_e THEN e.version ELSE 0 END,
  'allowed',EXISTS(SELECT 1 FROM payments.platform_stripe_allowlist a WHERE a.tenant_id=p_tenant AND a.store_id=p_store
   AND a.environment=p_environment AND a.allowed));
END $$;

ALTER FUNCTION payments.platform_stripe_valid_qualification(text,text) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.platform_stripe_state(text) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.platform_stripe_refresh_store(uuid,uuid,text,uuid,uuid,text) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.platform_stripe_fanout(text,text) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.platform_stripe_summary(uuid,uuid,text) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.platform_stripe_valid_qualification(text,text),payments.platform_stripe_state(text),
 payments.platform_stripe_refresh_store(uuid,uuid,text,uuid,uuid,text),payments.platform_stripe_fanout(text,text),
 payments.platform_stripe_summary(uuid,uuid,text) FROM PUBLIC;

-- ---------------------------------------------------------------------------------------------------------
-- 3.3 merchant entry points (owner registry writer, EXECUTE commerce_runtime only). p_token is the SHA-256 of the bearer
-- token (the COD precedent, 0107): identity.resolve_access fixes tenant/store/principal, never the request.
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.set_platform_stripe(p_token bytea,p_store uuid,p_profile text,p_enabled boolean,
 p_terms_version text,p_descriptor_suffix text,p_expected_version bigint) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_env text; sp payments.stripe_platform%ROWTYPE; e payments.platform_stripe_enrollments%ROWTYPE;
 ap payments.stripe_live_approvals%ROWTYPE; v_has_sp boolean; v_has_e boolean; v_state text; v_suffix text; v_pq uuid;
 v_l integer; v_proof text; v_replay boolean:=false; v_is_enabled boolean; v_summary jsonb;
BEGIN
 IF p_token IS NULL OR octet_length(p_token)<>32 OR p_store IS NULL OR p_profile IS NULL
  OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') OR p_enabled IS NULL OR p_expected_version IS NULL
  OR p_expected_version<0 OR p_expected_version>=9223372036854775807
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid platform Stripe request' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_token,p_store,'billing:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 -- the profile only picks the environment; the platform state and every row come from the database
 v_env:=CASE WHEN p_profile='LIVE' THEN 'LIVE' ELSE 'SANDBOX' END;
 PERFORM pg_advisory_xact_lock(hashtextextended('platform.stripe.enroll|'||s.tenant_id||'|'||p_store||'|'||v_env,0));
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=v_env; v_has_sp:=FOUND;
 SELECT x.* INTO e FROM payments.platform_stripe_enrollments x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
  AND x.environment=v_env FOR UPDATE; v_has_e:=FOUND;
 IF NOT p_enabled THEN
  -- disable is ALWAYS allowed (blocked, CLOSED, REVOKED...): it only stops new starts; it keeps every row (I16)
  IF v_has_e THEN
   v_is_enabled:=EXISTS(SELECT 1 FROM payments.method_heads hd JOIN payments.method_versions mv
    ON mv.tenant_id=hd.tenant_id AND mv.store_id=hd.store_id AND mv.market_id=hd.market_id AND mv.country=hd.country
    AND mv.code=hd.code AND mv.version=hd.current_version
    WHERE hd.tenant_id=s.tenant_id AND hd.store_id=p_store AND hd.code='stripe_checkout' AND mv.enabled);
   IF v_is_enabled THEN
    IF e.version<>p_expected_version THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
    PERFORM payments.platform_stripe_refresh_store(s.tenant_id,p_store,v_env,s.principal_id,NULL,'disable');
    UPDATE payments.platform_stripe_enrollments SET version=version+1,updated_at=clock_timestamp()
     WHERE tenant_id=s.tenant_id AND store_id=p_store AND environment=v_env;
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
    VALUES(s.tenant_id,p_store,s.principal_id,'stripe.platform.disable');
   END IF;
  END IF;
 ELSE
  IF v_has_e AND e.blocked_at IS NOT NULL THEN RAISE EXCEPTION 'platform_stripe_blocked' USING ERRCODE='PT403'; END IF;
  -- AD-PF2: only operator-allowlisted stores (the platform owner's own) may enable; checked before platform state
  IF NOT EXISTS(SELECT 1 FROM payments.platform_stripe_allowlist a WHERE a.tenant_id=s.tenant_id AND a.store_id=p_store
   AND a.environment=v_env AND a.allowed) THEN
   RAISE EXCEPTION 'platform_stripe_not_allowed' USING ERRCODE='PT403'; END IF;
  IF NOT v_has_sp THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
  v_state:=payments.platform_stripe_state(v_env);
  IF v_state='CLOSED' THEN RAISE EXCEPTION 'platform_stripe_closed' USING ERRCODE='PT409'; END IF;
  IF v_state<>'OPEN' THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
  IF p_terms_version IS DISTINCT FROM sp.terms_version THEN RAISE EXCEPTION 'terms_version_stale' USING ERRCODE='PT409'; END IF;
  -- a store holds EITHER its own primary Stripe connection OR one derived connection per environment
  IF EXISTS(SELECT 1 FROM integration.merchant_accounts a WHERE a.tenant_id=s.tenant_id AND a.store_id=p_store
   AND a.provider='stripe' AND a.environment=v_env AND a.platform_connection_id IS NULL) THEN
   RAISE EXCEPTION 'stripe_store_has_own_account' USING ERRCODE='PT409'; END IF;
  v_suffix:=nullif(p_descriptor_suffix,'');
  IF v_suffix IS NOT NULL THEN
   -- PF-F4: card prefix + '* ' + suffix is at most 22 characters. L = PrefixLength (LIVE approval) or min(DescriptorLength,10);
   -- SANDBOX has no approval, so L=10.
   IF v_env='LIVE' THEN
    SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.connection_id=sp.connection_id AND x.revoked_at IS NULL;
    IF NOT FOUND THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
    v_l:=CASE WHEN (ap.account_readiness->>'PrefixLength')::integer=0
     THEN least((ap.account_readiness->>'DescriptorLength')::integer,10)
     ELSE (ap.account_readiness->>'PrefixLength')::integer END;
   ELSE v_l:=10; END IF;
   IF char_length(v_suffix)>10 OR v_l+2+char_length(v_suffix)>22 THEN
    RAISE EXCEPTION 'descriptor_suffix_too_long' USING ERRCODE='PT422'; END IF;
   IF v_suffix !~ '^[A-Za-z0-9][A-Za-z0-9 .-]{1,9}$' OR v_suffix !~ '[A-Za-z]' THEN
    RAISE EXCEPTION 'invalid_descriptor_suffix' USING ERRCODE='PT422'; END IF;
  END IF;
  IF v_has_e THEN
   v_is_enabled:=EXISTS(SELECT 1 FROM payments.method_heads hd JOIN payments.method_versions mv
    ON mv.tenant_id=hd.tenant_id AND mv.store_id=hd.store_id AND mv.market_id=hd.market_id AND mv.country=hd.country
    AND mv.code=hd.code AND mv.version=hd.current_version
    WHERE hd.tenant_id=s.tenant_id AND hd.store_id=p_store AND hd.code='stripe_checkout' AND mv.enabled);
   -- replay of the stored state: same terms and suffix while already enabled -> no CAS, no new version
   v_replay:=v_is_enabled AND e.terms_version=p_terms_version AND e.descriptor_suffix IS NOT DISTINCT FROM v_suffix;
  END IF;
  IF NOT v_replay AND ((v_has_e AND e.version<>p_expected_version) OR (NOT v_has_e AND p_expected_version<>0)) THEN
   RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
  v_proof:=CASE p_profile WHEN 'PROVIDER_MOCK' THEN 'PROVIDER_MOCK' WHEN 'SANDBOX' THEN 'REAL_SANDBOX' ELSE 'REAL_LIVE' END;
  v_pq:=payments.platform_stripe_valid_qualification(v_env,v_proof);
  IF v_pq IS NULL THEN RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
  -- one transaction: binding, derived account, credential copy, derived qualification, method heads (no-op when current)
  PERFORM payments.platform_stripe_refresh_store(s.tenant_id,p_store,v_env,s.principal_id,v_pq,'enable');
  IF NOT v_replay THEN
   IF v_has_e THEN
    UPDATE payments.platform_stripe_enrollments SET enrolled_by=s.principal_id,terms_version=p_terms_version,
     accepted_at=clock_timestamp(),descriptor_suffix=v_suffix,version=version+1,updated_at=clock_timestamp()
     WHERE tenant_id=s.tenant_id AND store_id=p_store AND environment=v_env;
   ELSE
    INSERT INTO payments.platform_stripe_enrollments(tenant_id,store_id,environment,connection_id,enrolled_by,
     terms_version,accepted_at,descriptor_suffix)
    SELECT s.tenant_id,p_store,v_env,a.id,s.principal_id,p_terms_version,clock_timestamp(),v_suffix
    FROM integration.merchant_accounts a WHERE a.tenant_id=s.tenant_id AND a.store_id=p_store AND a.provider='stripe'
     AND a.environment=v_env AND a.platform_connection_id=sp.connection_id;
   END IF;
   INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,'stripe.platform.enable');
  END IF;
 END IF;
 v_summary:=payments.platform_stripe_summary(s.tenant_id,p_store,v_env);
 SELECT * INTO v_final FROM identity.resolve_access(p_token,p_store,'billing:manage');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN jsonb_build_object('state',v_summary->>'store_state','version',(v_summary->>'version')::bigint,
  'max_minor',v_summary->'max_minor','currency',v_summary->'currency','descriptor_preview',v_summary->'descriptor_preview');
END $$;

CREATE FUNCTION payments.read_platform_stripe(p_token bytea,p_store uuid,p_profile text) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_summary jsonb;
BEGIN
 IF p_token IS NULL OR octet_length(p_token)<>32 OR p_store IS NULL OR p_profile IS NULL
  OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE') OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid platform Stripe read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_token,p_store,'integration:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 v_summary:=payments.platform_stripe_summary(s.tenant_id,p_store,CASE WHEN p_profile='LIVE' THEN 'LIVE' ELSE 'SANDBOX' END);
 SELECT * INTO v_final FROM identity.resolve_access(p_token,p_store,'integration:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_summary;
END $$;

-- ---------------------------------------------------------------------------------------------------------
-- 3.3 operator entry points (owner registry writer, EXECUTE commerce_payment_registrar only; scope = the PLATFORM store)
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.designate_stripe_platform(p_tenant uuid,p_store uuid,p_principal uuid,p_connection uuid,
 p_display_name text,p_descriptor_display text,p_terms_version text,p_expected_version bigint) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE acct integration.merchant_accounts%ROWTYPE; sp payments.stripe_platform%ROWTYPE; v_next bigint;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_connection IS NULL OR p_display_name IS NULL OR char_length(p_display_name) NOT BETWEEN 2 AND 60
  OR p_display_name ~ '[[:cntrl:]<>]' OR p_descriptor_display IS NULL OR p_descriptor_display !~ '^[A-Za-z0-9 .*-]{5,22}$'
  OR p_terms_version IS NULL OR p_terms_version !~ '^[a-z0-9.-]{3,40}$'
  OR p_expected_version IS NULL OR p_expected_version<0 THEN
  RAISE EXCEPTION 'invalid platform Stripe designation' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
  AND x.id=p_connection AND x.provider='stripe' FOR SHARE;
 IF NOT FOUND OR acct.platform_connection_id IS NOT NULL THEN
  RAISE EXCEPTION 'stripe_platform_derived' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=acct.environment;
 IF NOT FOUND THEN
  IF p_expected_version<>0 THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
  INSERT INTO payments.stripe_platform(environment,tenant_id,store_id,connection_id,account_id,display_name,
   descriptor_display,terms_version)
  VALUES(acct.environment,p_tenant,p_store,p_connection,acct.account_id,p_display_name,p_descriptor_display,p_terms_version);
  v_next:=1;
 ELSE
  -- the row belongs to one store scope: only that store's operator may change it
  IF sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
   RAISE EXCEPTION 'platform_has_enrollments' USING ERRCODE='PT409'; END IF;
  IF sp.version<>p_expected_version THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
  IF sp.connection_id<>p_connection AND EXISTS(SELECT 1 FROM payments.platform_stripe_enrollments x
   WHERE x.environment=acct.environment) THEN
   -- moving the connection would orphan every derived row
   RAISE EXCEPTION 'platform_has_enrollments' USING ERRCODE='PT409'; END IF;
  v_next:=sp.version+1;
  UPDATE payments.stripe_platform SET connection_id=p_connection,account_id=acct.account_id,display_name=p_display_name,
   descriptor_display=p_descriptor_display,terms_version=p_terms_version,version=v_next,updated_at=clock_timestamp()
   WHERE environment=acct.environment;
 END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(p_tenant,p_store,p_principal,'stripe.platform.designate');
 RETURN v_next;
END $$;

CREATE FUNCTION payments.set_stripe_platform_open(p_tenant uuid,p_store uuid,p_principal uuid,p_environment text,
 p_open boolean,p_platform_fee_bps integer,p_expected_version bigint) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE sp payments.stripe_platform%ROWTYPE; ap payments.stripe_live_approvals%ROWTYPE; v_next bigint;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_environment NOT IN ('SANDBOX','LIVE') OR p_open IS NULL OR p_expected_version IS NULL OR p_expected_version<1
  OR (p_platform_fee_bps IS NOT NULL AND p_platform_fee_bps NOT BETWEEN 0 AND 3000) THEN
  RAISE EXCEPTION 'invalid platform Stripe open request' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 IF sp.version<>p_expected_version THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
 IF p_open THEN
  -- OPEN preconditions minus the flag itself (§2); LIVE also needs the §8 attestation codes and a verified canary
  IF p_environment='LIVE' THEN
   SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.connection_id=sp.connection_id AND x.revoked_at IS NULL;
   IF NOT FOUND OR ap.canary_verified_at IS NULL
    OR NOT (ap.checklist @> ARRAY['merchant_terms','payout_ops','platform_stripe_terms','tax_invoice']::text[]) THEN
    RAISE EXCEPTION 'platform_not_ready' USING ERRCODE='PT409'; END IF;
  END IF;
  IF payments.platform_stripe_valid_qualification(p_environment) IS NULL THEN
   RAISE EXCEPTION 'platform_not_ready' USING ERRCODE='PT409'; END IF;
 END IF;
 v_next:=sp.version+1;
 UPDATE payments.stripe_platform SET enrollment_open=p_open,platform_fee_bps=coalesce(p_platform_fee_bps,platform_fee_bps),
  version=v_next,updated_at=clock_timestamp() WHERE environment=p_environment;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(p_tenant,p_store,p_principal,CASE WHEN p_open THEN 'stripe.platform.open' ELSE 'stripe.platform.close' END);
 RETURN v_next;
END $$;

CREATE FUNCTION payments.block_platform_stripe(p_tenant uuid,p_store uuid,p_principal uuid,p_target_tenant uuid,
 p_target_store uuid,p_environment text,p_blocked boolean,p_operator text,p_ref text) RETURNS timestamptz
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE sp payments.stripe_platform%ROWTYPE; e payments.platform_stripe_enrollments%ROWTYPE; v_at timestamptz;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_target_tenant IS NULL OR p_target_store IS NULL OR p_environment NOT IN ('SANDBOX','LIVE') OR p_blocked IS NULL
  OR p_operator IS NULL OR p_operator !~ '^[A-Za-z0-9._:@-]{2,64}$'
  OR (p_blocked AND (p_ref IS NULL OR p_ref !~ '^[A-Za-z0-9._:-]{8,128}$')) THEN
  RAISE EXCEPTION 'invalid platform Stripe block request' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO e FROM payments.platform_stripe_enrollments x WHERE x.tenant_id=p_target_tenant
  AND x.store_id=p_target_store AND x.environment=p_environment;
 IF NOT FOUND THEN RAISE EXCEPTION 'enrollment_unavailable' USING ERRCODE='PT409'; END IF;
 -- every row write below runs in the TARGET store's scope; the audit row returns to the platform scope
 PERFORM set_config('app.tenant_id',p_target_tenant::text,true),set_config('app.store_id',p_target_store::text,true),
  set_config('app.principal_id',e.enrolled_by::text,true);
 PERFORM 1 FROM payments.platform_stripe_enrollments x WHERE x.tenant_id=p_target_tenant AND x.store_id=p_target_store
  AND x.environment=p_environment FOR UPDATE;
 IF p_blocked THEN
  IF e.blocked_at IS NOT NULL THEN
   IF e.blocked_ref=p_ref AND e.blocked_by=p_operator THEN v_at:=e.blocked_at; ELSE
    RAISE EXCEPTION 'already_blocked' USING ERRCODE='PT409'; END IF;
  ELSE
   v_at:=clock_timestamp();
   UPDATE payments.platform_stripe_enrollments SET blocked_at=v_at,blocked_ref=p_ref,blocked_by=p_operator,
    version=version+1,updated_at=v_at WHERE tenant_id=p_target_tenant AND store_id=p_target_store AND environment=p_environment;
   -- start -> PT409 immediately: the derived qualification is revoked and the card method disabled (never deleted)
   UPDATE payments.account_qualifications SET revoked_at=v_at WHERE tenant_id=p_target_tenant AND store_id=p_target_store
    AND connection_id=e.connection_id AND platform_qualification_id IS NOT NULL AND revoked_at IS NULL;
   PERFORM payments.platform_stripe_refresh_store(p_target_tenant,p_target_store,p_environment,e.enrolled_by,NULL,'disable');
  END IF;
 ELSE
  v_at:=clock_timestamp();
  IF e.blocked_at IS NOT NULL THEN
   -- unblock clears the triple only; the merchant must enable again (which re-derives)
   UPDATE payments.platform_stripe_enrollments SET blocked_at=NULL,blocked_ref=NULL,blocked_by=NULL,
    version=version+1,updated_at=v_at WHERE tenant_id=p_target_tenant AND store_id=p_target_store AND environment=p_environment;
  END IF;
 END IF;
 PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',p_principal::text,true);
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
 VALUES(p_tenant,p_store,p_principal,CASE WHEN p_blocked THEN 'stripe.platform.block' ELSE 'stripe.platform.unblock' END,
  jsonb_build_object('target_tenant',p_target_tenant,'target_store',p_target_store,'environment',p_environment));
 RETURN v_at;
END $$;

ALTER FUNCTION payments.set_platform_stripe(bytea,uuid,text,boolean,text,text,bigint) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.read_platform_stripe(bytea,uuid,text) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.designate_stripe_platform(uuid,uuid,uuid,uuid,text,text,text,bigint) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.set_stripe_platform_open(uuid,uuid,uuid,text,boolean,integer,bigint) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.block_platform_stripe(uuid,uuid,uuid,uuid,uuid,text,boolean,text,text) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.set_platform_stripe(bytea,uuid,text,boolean,text,text,bigint),
 payments.read_platform_stripe(bytea,uuid,text),
 payments.designate_stripe_platform(uuid,uuid,uuid,uuid,text,text,text,bigint),
 payments.set_stripe_platform_open(uuid,uuid,uuid,text,boolean,integer,bigint),
 payments.block_platform_stripe(uuid,uuid,uuid,uuid,uuid,text,boolean,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.set_platform_stripe(bytea,uuid,text,boolean,text,text,bigint),
 payments.read_platform_stripe(bytea,uuid,text) TO commerce_runtime;
GRANT EXECUTE ON FUNCTION payments.designate_stripe_platform(uuid,uuid,uuid,uuid,text,text,text,bigint),
 payments.set_stripe_platform_open(uuid,uuid,uuid,text,boolean,integer,bigint),
 payments.block_platform_stripe(uuid,uuid,uuid,uuid,uuid,text,boolean,text,text) TO commerce_payment_registrar;

-- Operator allowlist (AD-PF2). Scope = the platform store; the allowlist row is written in the TARGET store's scope and an
-- audit row (target ids in details) in the platform scope. Idempotent per (target, environment): same state+ref+operator
-- replays; a changed state bumps the version.
CREATE FUNCTION payments.allow_platform_stripe(p_tenant uuid,p_store uuid,p_principal uuid,p_target_tenant uuid,
 p_target_store uuid,p_environment text,p_allowed boolean,p_operator text,p_ref text) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE sp payments.stripe_platform%ROWTYPE; a payments.platform_stripe_allowlist%ROWTYPE; v_has boolean; v_ver bigint;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_target_tenant IS NULL OR p_target_store IS NULL OR p_environment NOT IN ('SANDBOX','LIVE') OR p_allowed IS NULL
  OR p_operator IS NULL OR p_operator !~ '^[A-Za-z0-9._:@-]{2,64}$' OR p_ref IS NULL OR p_ref !~ '^[A-Za-z0-9._:-]{8,128}$'
  OR NOT EXISTS(SELECT 1 FROM control.stores x WHERE x.tenant_id=p_target_tenant AND x.id=p_target_store) THEN
  RAISE EXCEPTION 'invalid platform Stripe allowlist request' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO a FROM payments.platform_stripe_allowlist x WHERE x.tenant_id=p_target_tenant AND x.store_id=p_target_store
  AND x.environment=p_environment; v_has:=FOUND;
 PERFORM set_config('app.tenant_id',p_target_tenant::text,true),set_config('app.store_id',p_target_store::text,true);
 IF NOT v_has THEN
  INSERT INTO payments.platform_stripe_allowlist(tenant_id,store_id,environment,allowed,allowed_ref,allowed_by)
  VALUES(p_target_tenant,p_target_store,p_environment,p_allowed,p_ref,p_operator); v_ver:=1;
 ELSIF a.allowed IS DISTINCT FROM p_allowed THEN
  UPDATE payments.platform_stripe_allowlist SET allowed=p_allowed,allowed_ref=p_ref,allowed_by=p_operator,
   version=version+1,updated_at=clock_timestamp() WHERE tenant_id=p_target_tenant AND store_id=p_target_store
   AND environment=p_environment;
  v_ver:=a.version+1;
 ELSE v_ver:=a.version; END IF;
 PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
 VALUES(p_tenant,p_store,p_principal,CASE WHEN p_allowed THEN 'stripe.platform.allow' ELSE 'stripe.platform.disallow' END,
  jsonb_build_object('target_tenant',p_target_tenant,'target_store',p_target_store,'environment',p_environment));
 RETURN v_ver;
END $$;
ALTER FUNCTION payments.allow_platform_stripe(uuid,uuid,uuid,uuid,uuid,text,boolean,text,text) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.allow_platform_stripe(uuid,uuid,uuid,uuid,uuid,text,boolean,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.allow_platform_stripe(uuid,uuid,uuid,uuid,uuid,text,boolean,text,text) TO commerce_payment_registrar;

-- ---------------------------------------------------------------------------------------------------------
-- 3.4 re-created definers (same signatures, owners and ACLs; the marked "delta (0137)" lines are the only changes)
-- ---------------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.rotate_stripe_key(p_tenant uuid,p_store uuid,p_principal uuid,
 p_connection uuid,p_expected_version bigint,p_key_id text,p_nonce bytea,p_ciphertext bytea)
RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE acct integration.merchant_accounts%ROWTYPE; v_next bigint;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_connection IS NULL OR p_expected_version IS NULL OR p_expected_version<1
  OR p_key_id !~ '^[A-Za-z0-9_-]{1,40}$'
  OR octet_length(p_nonce)<>12 OR octet_length(p_ciphertext) NOT BETWEEN 17 AND 8192 THEN
  RAISE EXCEPTION 'invalid Stripe rotation input' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.id=p_connection AND x.provider='stripe' FOR UPDATE;
 IF NOT FOUND OR acct.credential_version<>p_expected_version OR acct.environment NOT IN ('SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'Stripe credential version changed' USING ERRCODE='PT409'; END IF;
 -- delta (0137): a derived connection never rotates; the platform rotation fans out below
 IF acct.platform_connection_id IS NOT NULL THEN RAISE EXCEPTION 'stripe_platform_derived' USING ERRCODE='PT409'; END IF;
 v_next:=p_expected_version+1;
 INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,
  nonce,ciphertext,principal_id)
 VALUES(p_tenant,p_store,p_connection,v_next,p_key_id,p_nonce,p_ciphertext,p_principal);
 UPDATE integration.merchant_accounts SET credential_version=v_next,updated_at=clock_timestamp()
  WHERE tenant_id=p_tenant AND store_id=p_store AND id=p_connection;
 -- delta (0137): copy the new envelope to every enrolled store in this transaction (derived qualifications follow qualify)
 IF EXISTS(SELECT 1 FROM payments.stripe_platform sp WHERE sp.connection_id=p_connection) THEN
  PERFORM payments.platform_stripe_fanout(acct.environment,'rotate'); END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(p_tenant,p_store,p_principal,'stripe.key_rotated');
 RETURN v_next;
END $$;

CREATE OR REPLACE FUNCTION payments.set_stripe_webhook_endpoint(p_tenant uuid,p_store uuid,p_principal uuid,
 p_connection uuid,p_endpoint uuid,p_profile text,p_expected_version bigint,p_enabled boolean,
 p_key_id text,p_nonce bytea,p_ciphertext bytea) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE acct integration.merchant_accounts%ROWTYPE; ep payments.stripe_webhook_endpoints%ROWTYPE;
 v_next bigint;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_connection IS NULL OR p_endpoint IS NULL OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE')
  OR p_expected_version IS NULL OR p_expected_version<0 OR p_enabled IS NULL
  OR p_key_id !~ '^[A-Za-z0-9_-]{1,40}$'
  OR octet_length(p_nonce)<>12 OR octet_length(p_ciphertext) NOT BETWEEN 17 AND 8192 THEN
  RAISE EXCEPTION 'invalid Stripe endpoint input' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.id=p_connection AND x.provider='stripe' FOR SHARE;
 IF NOT FOUND OR acct.environment NOT IN ('SANDBOX','LIVE') OR (acct.environment='LIVE')<>(p_profile='LIVE') THEN
  RAISE EXCEPTION 'Stripe endpoint account unavailable' USING ERRCODE='PT409'; END IF;
 -- delta (0137): one endpoint per platform connection serves every derived store
 IF acct.platform_connection_id IS NOT NULL THEN RAISE EXCEPTION 'stripe_platform_derived' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO ep FROM payments.stripe_webhook_endpoints x WHERE x.endpoint_id=p_endpoint FOR UPDATE;
 IF p_expected_version=0 THEN
  IF FOUND THEN RAISE EXCEPTION 'Stripe endpoint already exists' USING ERRCODE='PT409'; END IF;
  INSERT INTO payments.stripe_webhook_endpoints(endpoint_id,tenant_id,store_id,connection_id,
   environment,account_id,execution_profile,enabled,key_version,key_id,nonce,ciphertext)
  VALUES(p_endpoint,p_tenant,p_store,p_connection,acct.environment,acct.account_id,p_profile,
   p_enabled,1,p_key_id,p_nonce,p_ciphertext);
  v_next:=1;
 ELSE
  IF NOT FOUND OR ep.tenant_id<>p_tenant OR ep.store_id<>p_store
   OR ep.connection_id<>p_connection OR ep.execution_profile<>p_profile
   OR ep.key_version<>p_expected_version THEN
   RAISE EXCEPTION 'Stripe endpoint version changed' USING ERRCODE='PT409'; END IF;
  v_next:=p_expected_version+1;
  UPDATE payments.stripe_webhook_endpoints SET enabled=p_enabled,key_version=v_next,
   key_id=p_key_id,nonce=p_nonce,ciphertext=p_ciphertext,updated_at=clock_timestamp()
   WHERE endpoint_id=p_endpoint;
 END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(p_tenant,p_store,p_principal,'stripe.webhook_endpoint_set');
 RETURN v_next;
END $$;

CREATE OR REPLACE FUNCTION payments.qualify_stripe_method(p_tenant uuid,p_store uuid,p_principal uuid,
 p_qualification uuid,p_connection uuid,p_expected_version bigint,p_profile text,p_evidence_ref text,
 p_observed_at timestamptz,p_expires_at timestamptz) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE acct integration.merchant_accounts%ROWTYPE; b integration.bindings%ROWTYPE;
 ap payments.stripe_live_approvals%ROWTYPE; v_proof text; v_approval uuid;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_qualification IS NULL OR p_connection IS NULL OR p_expected_version IS NULL
  OR p_expected_version<1 OR p_profile NOT IN ('PROVIDER_MOCK','SANDBOX','LIVE')
  OR p_evidence_ref IS NULL OR length(p_evidence_ref) NOT BETWEEN 1 AND 200
  OR p_evidence_ref ~ '[[:cntrl:]]' OR p_observed_at IS NULL OR p_expires_at IS NULL
  OR p_observed_at>clock_timestamp() OR p_expires_at<=p_observed_at
  OR p_expires_at>p_observed_at+interval '30 days' THEN
  RAISE EXCEPTION 'invalid Stripe qualification input' USING ERRCODE='22023'; END IF;
 IF p_profile='SANDBOX' AND p_evidence_ref !~ '^stripe-probe:[A-Za-z0-9_]{1,255}$' THEN
  RAISE EXCEPTION 'Stripe probe evidence missing' USING ERRCODE='22023'; END IF;
 IF p_profile='LIVE' AND p_evidence_ref !~ '^stripe-probe:cs_live_[A-Za-z0-9_]{1,245}$' THEN
  RAISE EXCEPTION 'Stripe probe evidence missing' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.id=p_connection AND x.provider='stripe' FOR SHARE;
 -- (acct.environment='LIVE')=(p_profile='LIVE'): PROVIDER_MOCK and SANDBOX profiles only on SANDBOX accounts
 IF NOT FOUND OR acct.environment NOT IN ('SANDBOX','LIVE')
  OR (acct.environment='LIVE')<>(p_profile='LIVE') OR acct.credential_version<>p_expected_version THEN
  RAISE EXCEPTION 'Stripe account unavailable' USING ERRCODE='PT409'; END IF;
 -- delta (0137): derived qualifications mirror the platform's; they are never qualified directly
 IF acct.platform_connection_id IS NOT NULL THEN RAISE EXCEPTION 'stripe_platform_derived' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
  AND x.id=acct.binding_id FOR SHARE;
 IF NOT FOUND OR NOT b.enabled OR b.provider<>'stripe'
  OR b.external_asset_id<>acct.binding_asset THEN
  RAISE EXCEPTION 'Stripe binding unavailable' USING ERRCODE='PT409'; END IF;
 IF p_profile='LIVE' THEN
  SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
   AND x.connection_id=p_connection AND x.revoked_at IS NULL FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'Stripe live approval unavailable' USING ERRCODE='PT409'; END IF;
  v_approval:=ap.id;
 END IF;
 v_proof:=CASE p_profile WHEN 'PROVIDER_MOCK' THEN 'PROVIDER_MOCK' WHEN 'SANDBOX' THEN 'REAL_SANDBOX'
  ELSE 'REAL_LIVE' END;
 INSERT INTO payments.account_qualifications(id,tenant_id,store_id,connection_id,credential_version,
  environment,code,proof_class,evidence_ref,observed_at,expires_at,live_approval_id)
 VALUES(p_qualification,p_tenant,p_store,p_connection,acct.credential_version,acct.environment,
  'stripe_checkout',v_proof,p_evidence_ref,p_observed_at,p_expires_at,v_approval);
 -- delta (0137): mirror the new platform qualification into every enrolled store, same transaction
 IF EXISTS(SELECT 1 FROM payments.stripe_platform sp WHERE sp.connection_id=p_connection) THEN
  PERFORM payments.platform_stripe_fanout(acct.environment,'qualify'); END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(p_tenant,p_store,p_principal,'stripe.method_qualified');
 RETURN p_qualification;
END $$;

CREATE OR REPLACE FUNCTION payments.set_stripe_method(p_tenant uuid,p_store uuid,p_principal uuid,
 p_market uuid,p_country text,p_connection uuid,p_qualification uuid,p_expected_version bigint,
 p_enabled boolean,p_visible boolean,p_sort integer,p_min bigint,p_max bigint,
 p_name_hans text,p_name_hant text,p_name_en text) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE market pricing.markets%ROWTYPE; acct integration.merchant_accounts%ROWTYPE;
 b integration.bindings%ROWTYPE; q payments.account_qualifications%ROWTYPE;
 ap payments.stripe_live_approvals%ROWTYPE;
 v_current bigint; v_next bigint; v_now timestamptz;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_market IS NULL OR p_connection IS NULL OR p_qualification IS NULL
  OR p_country !~ '^[A-Z]{2}$' OR p_expected_version IS NULL OR p_expected_version<0
  OR p_enabled IS NULL OR p_visible IS NULL OR p_sort NOT BETWEEN 0 AND 1000
  OR p_min IS NULL OR p_max IS NULL OR p_max<p_min THEN
  RAISE EXCEPTION 'invalid Stripe method input' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO market FROM pricing.markets x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.id=p_market FOR SHARE;
 IF NOT FOUND OR NOT market.active OR NOT payments.stripe_amount_ok(market.currency,p_min)
  OR NOT payments.stripe_amount_ok(market.currency,p_max) THEN
  RAISE EXCEPTION 'Stripe market amount unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.id=p_connection AND x.provider='stripe' FOR SHARE;
 IF NOT FOUND OR acct.environment NOT IN ('SANDBOX','LIVE') THEN
  RAISE EXCEPTION 'Stripe account unavailable' USING ERRCODE='PT409'; END IF;
 -- delta (0137): a derived connection admits only the disable (operator kill switch); enabling is set_platform_stripe
 IF acct.platform_connection_id IS NOT NULL AND p_enabled THEN
  RAISE EXCEPTION 'stripe_platform_derived' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.id=acct.binding_id FOR SHARE;
 IF NOT FOUND OR NOT b.enabled OR b.provider<>'stripe'
  OR b.external_asset_id<>acct.binding_asset THEN
  RAISE EXCEPTION 'Stripe binding unavailable' USING ERRCODE='PT409'; END IF;
 -- LD5: only enabling a LIVE method needs the active approval; the approval is locked before the qualification
 IF acct.environment='LIVE' AND p_enabled THEN
  SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
   AND x.connection_id=p_connection AND x.revoked_at IS NULL FOR SHARE;
  IF NOT FOUND OR market.currency<>ap.currency THEN
   RAISE EXCEPTION 'Stripe live approval unavailable' USING ERRCODE='PT409'; END IF;
 END IF;
 SELECT x.* INTO q FROM payments.account_qualifications x WHERE x.id=p_qualification
  AND x.tenant_id=p_tenant AND x.store_id=p_store FOR SHARE;
 v_now:=clock_timestamp();
 IF NOT FOUND OR q.connection_id<>p_connection OR q.code<>'stripe_checkout' THEN
  RAISE EXCEPTION 'Stripe qualification unavailable' USING ERRCODE='PT409'; END IF;
 IF p_enabled THEN
  IF q.credential_version<>acct.credential_version OR q.environment<>acct.environment
   OR q.revoked_at IS NOT NULL OR q.observed_at>v_now OR q.expires_at<=v_now THEN
   RAISE EXCEPTION 'Stripe qualification unavailable' USING ERRCODE='PT409'; END IF;
  IF acct.environment='LIVE' THEN
   IF q.proof_class<>'REAL_LIVE' OR q.live_approval_id IS DISTINCT FROM ap.id
    OR p_max>(CASE WHEN ap.canary_verified_at IS NULL THEN ap.canary_max_minor ELSE ap.max_minor END) THEN
    RAISE EXCEPTION 'Stripe live method not admitted' USING ERRCODE='PT409'; END IF;
  END IF;
 END IF;
 SELECT h.current_version INTO v_current FROM payments.method_heads h WHERE h.tenant_id=p_tenant
  AND h.store_id=p_store AND h.market_id=p_market AND h.country=p_country
  AND h.code='stripe_checkout' FOR UPDATE;
 IF p_expected_version=0 THEN
  IF FOUND THEN RAISE EXCEPTION 'Stripe method already exists' USING ERRCODE='PT409'; END IF;
  v_next:=1;
 ELSE
  IF NOT FOUND OR v_current<>p_expected_version THEN
   RAISE EXCEPTION 'Stripe method version changed' USING ERRCODE='PT409'; END IF;
  v_next:=p_expected_version+1;
 END IF;
 INSERT INTO payments.method_versions(tenant_id,store_id,market_id,country,code,version,provider,
  environment,connection_id,binding_version,currency,name_hans,name_hant,name_en,
  enabled,visible,sort_order,min_amount_minor,max_amount_minor,principal_id,qualification_id)
 VALUES(p_tenant,p_store,p_market,p_country,'stripe_checkout',v_next,'stripe',acct.environment,
  p_connection,b.semantic_version,market.currency,p_name_hans,p_name_hant,p_name_en,
  p_enabled,p_visible,p_sort,p_min,p_max,p_principal,p_qualification);
 IF p_expected_version=0 THEN
  INSERT INTO payments.method_heads(tenant_id,store_id,market_id,country,code,current_version)
  VALUES(p_tenant,p_store,p_market,p_country,'stripe_checkout',v_next);
 ELSE
  UPDATE payments.method_heads SET current_version=v_next WHERE tenant_id=p_tenant
   AND store_id=p_store AND market_id=p_market AND country=p_country AND code='stripe_checkout';
 END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(p_tenant,p_store,p_principal,'stripe.method_set');
 RETURN v_next;
END $$;

CREATE OR REPLACE FUNCTION payments.revoke_stripe_live(p_tenant uuid,p_store uuid,p_principal uuid,
 p_approval uuid,p_revoke_ref text) RETURNS timestamptz
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ap payments.stripe_live_approvals%ROWTYPE; v_at timestamptz;
BEGIN
 -- Any principal passing the registrar scope may revoke: the kill switch must not need the owner.
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_approval IS NULL OR p_revoke_ref IS NULL OR p_revoke_ref !~ '^[A-Za-z0-9._:-]{8,128}$' THEN
  RAISE EXCEPTION 'invalid Stripe live revoke input' USING ERRCODE='22023'; END IF;
 -- The revoke below must see qualifications committed while the approval lock was awaited: each statement in
 -- READ COMMITTED takes a fresh snapshot, so this function refuses a snapshot-pinning isolation level.
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'Stripe live revoke needs read committed' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.id=p_approval AND x.tenant_id=p_tenant
  AND x.store_id=p_store FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'Stripe live approval unavailable' USING ERRCODE='PT409'; END IF;
 IF ap.revoked_at IS NOT NULL THEN
  IF ap.revoked_by=p_principal AND ap.revoke_ref=p_revoke_ref THEN RETURN ap.revoked_at; END IF;
  RAISE EXCEPTION 'Stripe live approval already revoked' USING ERRCODE='PT409'; END IF;
 -- separate statement: every non-revoked qualification of this approval (the trigger stamps revoked_at)
 UPDATE payments.account_qualifications SET revoked_at=clock_timestamp()
  WHERE tenant_id=p_tenant AND store_id=p_store AND live_approval_id=p_approval AND revoked_at IS NULL;
 v_at:=clock_timestamp();
 UPDATE payments.stripe_live_approvals SET revoked_at=v_at,revoked_by=p_principal,revoke_ref=p_revoke_ref
  WHERE id=p_approval AND tenant_id=p_tenant AND store_id=p_store;
 -- delta (0137): revoking the PLATFORM approval revokes every derived qualification in the same transaction (§7 hard switch)
 IF EXISTS(SELECT 1 FROM payments.stripe_platform sp WHERE sp.connection_id=ap.connection_id) THEN
  PERFORM payments.platform_stripe_fanout('LIVE','revoke'); END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(p_tenant,p_store,p_principal,'stripe.live.revoke');
 RETURN v_at;
END $$;

CREATE OR REPLACE FUNCTION payments.approve_stripe_live(p_tenant uuid,p_store uuid,p_principal uuid,p_approval uuid,
 p_connection uuid,p_currency text,p_approval_ref text,p_approved_at timestamptz,p_canary_max bigint,
 p_max bigint,p_checklist text[],p_readiness jsonb) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE acct integration.merchant_accounts%ROWTYPE; b integration.bindings%ROWTYPE;
 ap payments.stripe_live_approvals%ROWTYPE; v_sorted text[]; v_key text; v_min bigint; v_now timestamptz;
 v_base text[]:=ARRAY['account_active','canary_private','descriptor','dispute_notice','managed_off','payout_bank','policy_pages','radar_default','rak_live','three_ds','webhook_live']::text[];
 v_full text[]:=ARRAY(SELECT c FROM unnest(ARRAY['account_active','canary_private','descriptor','dispute_notice','managed_off','payout_bank','policy_pages','radar_default','rak_live','three_ds','webhook_live','merchant_terms','payout_ops','platform_stripe_terms','tax_invoice']) c ORDER BY c COLLATE "C");
BEGIN
 -- integration:manage + active membership + active store, and the GUC scope for RLS and the audit row
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_approval IS NULL OR p_connection IS NULL OR p_currency IS NULL OR p_currency !~ '^[A-Z]{3}$'
  OR p_approval_ref IS NULL OR p_approval_ref !~ '^[A-Za-z0-9._:-]{8,128}$'
  OR p_approved_at IS NULL OR p_canary_max IS NULL OR p_max IS NULL OR p_checklist IS NULL
  OR p_readiness IS NULL OR jsonb_typeof(p_readiness)<>'object' THEN
  RAISE EXCEPTION 'invalid Stripe live approval input' USING ERRCODE='22023'; END IF;
 -- LD2: the approver is defined by a grant predicate (0065 store-creator set): integration:manage (above) AND payments:refund
 IF NOT EXISTS(SELECT 1 FROM identity.store_grants g WHERE g.tenant_id=p_tenant AND g.store_id=p_store
   AND g.principal_id=p_principal AND g.permission='payments:refund') THEN
  RAISE EXCEPTION 'Stripe live approver lacks payments:refund' USING ERRCODE='42501'; END IF;
 -- lock order: account -> binding -> approval
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.id=p_connection AND x.provider='stripe' FOR SHARE;
 IF NOT FOUND OR acct.environment<>'LIVE' THEN
  RAISE EXCEPTION 'Stripe live account unavailable' USING ERRCODE='PT409'; END IF;
 IF acct.platform_connection_id IS NOT NULL THEN RAISE EXCEPTION 'stripe_platform_derived' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
  AND x.id=acct.binding_id FOR SHARE;
 IF NOT FOUND OR NOT b.enabled OR b.provider<>'stripe' OR b.external_asset_id<>acct.binding_asset THEN
  RAISE EXCEPTION 'Stripe binding unavailable' USING ERRCODE='PT409'; END IF;
 -- the checklist is the exact §9 code set, stored sorted (byte order, no duplicates)
 SELECT array_agg(c ORDER BY c COLLATE "C") INTO v_sorted FROM unnest(p_checklist) c;
 -- delta (0137 §8): the platform approval may carry the four platform attestation codes (set_stripe_platform_open requires them on LIVE)
 IF v_sorted IS DISTINCT FROM v_base AND v_sorted IS DISTINCT FROM v_full THEN
  RAISE EXCEPTION 'Stripe live checklist incomplete' USING ERRCODE='22023'; END IF;
 -- replay: identical row -> same id; anything else -> PT409 (checked before time-dependent rules)
 SELECT x.* INTO ap FROM payments.stripe_live_approvals x WHERE x.id=p_approval;
 IF FOUND THEN
  IF ap.tenant_id=p_tenant AND ap.store_id=p_store AND ap.connection_id=p_connection AND ap.currency=p_currency
   AND ap.approved_by=p_principal AND ap.approval_ref=p_approval_ref AND ap.approved_at=p_approved_at
   AND ap.canary_max_minor=p_canary_max AND ap.max_minor=p_max AND ap.checklist=v_sorted
   AND ap.account_readiness=p_readiness THEN
   RETURN ap.id; END IF;
  RAISE EXCEPTION 'Stripe live approval already exists' USING ERRCODE='PT409'; END IF;
 -- LD8: absence is never a pass. Every key must exist and be non-null, then have the right type.
 FOREACH v_key IN ARRAY ARRAY['AVSRule','CVCRule','ChargesEnabled','CurrentlyDueCount','DescriptorLength',
  'DetailsSubmitted','PayoutsEnabled','PrefixLength'] LOOP
  IF NOT p_readiness ? v_key OR jsonb_typeof(p_readiness->v_key)='null' THEN
   RAISE EXCEPTION 'stripe_live_readiness_unknown' USING ERRCODE='22023'; END IF;
 END LOOP;
 IF (SELECT count(*) FROM jsonb_object_keys(p_readiness))<>8 THEN
  RAISE EXCEPTION 'invalid Stripe live readiness shape' USING ERRCODE='22023'; END IF;
 FOREACH v_key IN ARRAY ARRAY['AVSRule','CVCRule','ChargesEnabled','DetailsSubmitted','PayoutsEnabled'] LOOP
  IF jsonb_typeof(p_readiness->v_key)<>'boolean' THEN
   RAISE EXCEPTION 'invalid Stripe live readiness type' USING ERRCODE='22023'; END IF;
 END LOOP;
 FOREACH v_key IN ARRAY ARRAY['CurrentlyDueCount','DescriptorLength','PrefixLength'] LOOP
  IF jsonb_typeof(p_readiness->v_key)<>'number' OR (p_readiness->>v_key) !~ '^(0|[1-9][0-9]{0,5})$' THEN
   RAISE EXCEPTION 'invalid Stripe live readiness type' USING ERRCODE='22023'; END IF;
 END LOOP;
 IF NOT ((p_readiness->>'ChargesEnabled')::boolean AND (p_readiness->>'PayoutsEnabled')::boolean
  AND (p_readiness->>'DetailsSubmitted')::boolean AND (p_readiness->>'CurrentlyDueCount')::integer=0
  AND (p_readiness->>'DescriptorLength')::integer BETWEEN 5 AND 22
  AND ((p_readiness->>'PrefixLength')::integer=0 OR (p_readiness->>'PrefixLength')::integer BETWEEN 2 AND 10)) THEN
  RAISE EXCEPTION 'stripe_live_not_ready' USING ERRCODE='22023'; END IF;
 v_now:=clock_timestamp();
 IF p_approved_at>v_now OR p_approved_at<v_now-interval '30 days' THEN
  RAISE EXCEPTION 'Stripe live approval time out of range' USING ERRCODE='22023'; END IF;
 -- I05 / LQ3: caps come from the closed table; the owner has ruled a per-order max for TWD only
 v_min:=payments.stripe_min_minor(p_currency);
 IF v_min IS NULL THEN
  RAISE EXCEPTION 'Stripe live currency unknown' USING ERRCODE='22023'; END IF;
 IF p_currency<>'TWD' THEN
  RAISE EXCEPTION 'stripe_live_max_unruled' USING ERRCODE='22023'; END IF;
 IF NOT payments.stripe_amount_ok(p_currency,p_canary_max) OR p_canary_max>2*v_min
  OR NOT payments.stripe_amount_ok(p_currency,p_max) OR p_max<p_canary_max OR p_max>2000000 THEN
  RAISE EXCEPTION 'Stripe live caps invalid' USING ERRCODE='22023'; END IF;
 IF EXISTS(SELECT 1 FROM payments.stripe_live_approvals x WHERE x.connection_id=p_connection
  AND x.revoked_at IS NULL) THEN
  RAISE EXCEPTION 'Stripe live approval already active' USING ERRCODE='PT409'; END IF;
 BEGIN
  INSERT INTO payments.stripe_live_approvals(id,tenant_id,store_id,connection_id,account_id,currency,
   approved_by,approval_ref,approved_at,canary_max_minor,max_minor,checklist,account_readiness)
  VALUES(p_approval,p_tenant,p_store,p_connection,acct.account_id,p_currency,p_principal,p_approval_ref,
   p_approved_at,p_canary_max,p_max,v_sorted,p_readiness);
 EXCEPTION WHEN unique_violation THEN
  RAISE EXCEPTION 'Stripe live approval already active' USING ERRCODE='PT409';
 END;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
 VALUES(p_tenant,p_store,p_principal,'stripe.live.approve');
 RETURN p_approval;
END $$;

-- ---------------------------------------------------------------------------------------------------------
-- 3.4 credential loaders gain the aad_* return columns (DROP + CREATE + grants restored in this migration)
-- ---------------------------------------------------------------------------------------------------------
DROP FUNCTION payments.stripe_registrar_credential(uuid,uuid,uuid,uuid,bigint);
CREATE FUNCTION payments.stripe_registrar_credential(p_tenant uuid,p_store uuid,p_principal uuid,
 p_connection uuid,p_expected_version bigint)
RETURNS TABLE(account_id text,key_id text,nonce bytea,ciphertext bytea,aad_tenant_id uuid,aad_store_id uuid,
 aad_connection_id uuid,aad_version bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE acct integration.merchant_accounts%ROWTYPE; c integration.account_credentials%ROWTYPE;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_connection IS NULL OR p_expected_version IS NULL OR p_expected_version<1 THEN
  RAISE EXCEPTION 'invalid Stripe credential read' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO acct FROM integration.merchant_accounts x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.id=p_connection AND x.provider='stripe' AND x.environment IN ('SANDBOX','LIVE');
 IF NOT FOUND OR acct.credential_version<>p_expected_version THEN
  RAISE EXCEPTION 'Stripe credential version changed' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO c FROM integration.account_credentials x WHERE x.tenant_id=p_tenant
  AND x.store_id=p_store AND x.connection_id=p_connection AND x.version=p_expected_version;
 IF NOT FOUND THEN RAISE EXCEPTION 'Stripe credential unavailable' USING ERRCODE='PT409'; END IF;
 -- delta (0137): the AAD scope the envelope was sealed under: the row's own scope, or the platform connection's for a derived row
 RETURN QUERY SELECT acct.account_id,c.key_id,c.nonce,c.ciphertext,coalesce(p.tenant_id,acct.tenant_id),
  coalesce(p.store_id,acct.store_id),coalesce(p.connection_id,acct.id),coalesce(c.sealed_version,c.version)
  FROM (SELECT 1) one LEFT JOIN payments.stripe_platform p ON p.connection_id=acct.platform_connection_id;
END $$;
ALTER FUNCTION payments.stripe_registrar_credential(uuid,uuid,uuid,uuid,bigint) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.stripe_registrar_credential(uuid,uuid,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.stripe_registrar_credential(uuid,uuid,uuid,uuid,bigint) TO commerce_payment_registrar;

DROP FUNCTION integration.load_stripe_credential(uuid,bigint,bytea,text);
CREATE FUNCTION integration.load_stripe_credential(p_id uuid,p_generation bigint,p_token bytea,p_profile text)
RETURNS TABLE(tenant_id uuid,store_id uuid,connection_id uuid,credential_version bigint,
 environment text,account_id text,key_id text,nonce bytea,ciphertext bytea,aad_tenant_id uuid,aad_store_id uuid,
 aad_connection_id uuid,aad_version bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a checkout.payment_attempts%ROWTYPE; c integration.account_credentials%ROWTYPE;
 m integration.merchant_accounts%ROWTYPE; o integration.operations%ROWTYPE; pm integration.merchant_accounts%ROWTYPE;
BEGIN
 a:=integration.require_stripe_query(p_id,p_generation,p_token,p_profile);
 SELECT x.* INTO m FROM integration.merchant_accounts x WHERE x.id=a.connection_id
  AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id AND x.binding_id=a.binding_id
  AND x.provider='stripe' AND x.environment=a.environment;
 IF NOT FOUND THEN RAISE EXCEPTION 'Stripe account unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO c FROM integration.account_credentials x WHERE x.tenant_id=a.tenant_id
  AND x.store_id=a.store_id AND x.connection_id=a.connection_id AND x.version=a.credential_version;
 IF NOT FOUND THEN RAISE EXCEPTION 'Stripe credential unavailable' USING ERRCODE='PT409'; END IF;
 -- delta (0137): a derived row's envelope is a copy sealed under the platform connection's scope (aad_*)
 SELECT x.* INTO pm FROM integration.merchant_accounts x WHERE x.id=m.platform_connection_id;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=a.id;
 IF o.external_asset_id<>m.environment||':'||m.account_id THEN
  RAISE EXCEPTION 'Stripe account mismatch' USING ERRCODE='PT409'; END IF;
 -- Recheck after waits so an expired claim cannot release secret material.
 PERFORM integration.require_stripe_query(p_id,p_generation,p_token,p_profile);
 RETURN QUERY SELECT a.tenant_id,a.store_id,a.connection_id,a.credential_version,
  a.environment,m.account_id,c.key_id,c.nonce,c.ciphertext,coalesce(pm.tenant_id,a.tenant_id),
  coalesce(pm.store_id,a.store_id),coalesce(pm.id,a.connection_id),coalesce(c.sealed_version,a.credential_version);
END $$;
ALTER FUNCTION integration.load_stripe_credential(uuid,bigint,bytea,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.load_stripe_credential(uuid,bigint,bytea,text) FROM PUBLIC;

DROP FUNCTION integration.load_stripe_refund(uuid,bigint,bytea,text);
CREATE FUNCTION integration.load_stripe_refund(p_id uuid,p_generation bigint,p_token bytea,p_profile text)
RETURNS TABLE(tenant_id uuid,store_id uuid,attempt_id uuid,order_id uuid,connection_id uuid,environment text,
 account_id text,credential_version bigint,key_id text,nonce bytea,ciphertext bytea,
 payment_intent_id text,currency text,amount_minor bigint,reason text,requested_at timestamptz,
 resend_until timestamptz,first_sent_at timestamptz,last_sent_at timestamptz,send_count integer,
 body_sha256 bytea,suppressed_at timestamptz,stripe_refund_id text,pinned_at timestamptz,
 has_succeeded boolean,has_terminal boolean,has_rejected boolean,latest_status text,db_now timestamptz,
 aad_tenant_id uuid,aad_store_id uuid,aad_connection_id uuid,aad_version bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r payments.stripe_refunds%ROWTYPE; a checkout.payment_attempts%ROWTYPE;
 m integration.merchant_accounts%ROWTYPE; c integration.account_credentials%ROWTYPE;
 o integration.operations%ROWTYPE; pm integration.merchant_accounts%ROWTYPE; v_ok boolean; v_term boolean; v_rej boolean; v_status text;
BEGIN
 r:=integration.require_stripe_refund(p_id,p_generation,p_token,p_profile);
 SELECT x.* INTO a FROM checkout.payment_attempts x WHERE x.id=r.attempt_id AND x.tenant_id=r.tenant_id
  AND x.store_id=r.store_id;
 -- RD13: the account's CURRENT head credential, and only for the frozen account.
 SELECT x.* INTO m FROM integration.merchant_accounts x WHERE x.id=a.connection_id
  AND x.tenant_id=a.tenant_id AND x.store_id=a.store_id AND x.binding_id=a.binding_id
  AND x.provider='stripe' AND x.environment=a.environment;
 IF NOT FOUND OR m.account_id<>r.account_id THEN
  RAISE EXCEPTION 'Stripe refund account unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO c FROM integration.account_credentials x WHERE x.tenant_id=m.tenant_id
  AND x.store_id=m.store_id AND x.connection_id=m.id AND x.version=m.credential_version;
 IF NOT FOUND THEN RAISE EXCEPTION 'Stripe refund credential unavailable' USING ERRCODE='PT409'; END IF;
 -- delta (0137): aad_* names the scope the (possibly derived) envelope was sealed under
 SELECT x.* INTO pm FROM integration.merchant_accounts x WHERE x.id=m.platform_connection_id;
 SELECT x.* INTO o FROM integration.operations x WHERE x.id=r.id;
 IF o.external_asset_id<>m.environment||':'||m.account_id THEN
  RAISE EXCEPTION 'Stripe refund account mismatch' USING ERRCODE='PT409'; END IF;
 SELECT EXISTS(SELECT 1 FROM payments.refund_facts f WHERE f.tenant_id=r.tenant_id AND f.store_id=r.store_id
   AND f.refund_id=r.id AND f.kind='SUCCEEDED'),
  EXISTS(SELECT 1 FROM payments.refund_facts f WHERE f.tenant_id=r.tenant_id AND f.store_id=r.store_id
   AND f.refund_id=r.id AND f.kind IN ('FAILED','CANCELED')),
  EXISTS(SELECT 1 FROM payments.refund_facts f WHERE f.tenant_id=r.tenant_id AND f.store_id=r.store_id
   AND f.refund_id=r.id AND f.kind='REJECTED') INTO v_ok,v_term,v_rej;
 SELECT x.report->>'Status' INTO v_status FROM payments.provider_observations x
  WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.attempt_id=r.attempt_id
   AND x.source='QUERY' AND x.report->>'Object'='refund' AND x.report->>'RefundRef'=r.id::text
   AND x.report->>'RefundID'<>'' ORDER BY x.received_at DESC LIMIT 1;
 -- Recheck after waits so an expired claim cannot release secret material.
 PERFORM integration.require_stripe_refund(p_id,p_generation,p_token,p_profile);
 RETURN QUERY SELECT r.tenant_id,r.store_id,r.attempt_id,r.order_id,a.connection_id,a.environment,
  m.account_id,m.credential_version,c.key_id,c.nonce,c.ciphertext,r.payment_intent_id,r.currency,
  r.amount_minor,r.reason,r.requested_at,r.resend_until,r.first_sent_at,r.last_sent_at,r.send_count,
  r.body_sha256,r.suppressed_at,r.stripe_refund_id,r.pinned_at,v_ok,v_term,v_rej,coalesce(v_status,''),
  clock_timestamp(),coalesce(pm.tenant_id,r.tenant_id),coalesce(pm.store_id,r.store_id),coalesce(pm.id,a.connection_id),
  coalesce(c.sealed_version,m.credential_version);
END $$;
ALTER FUNCTION integration.load_stripe_refund(uuid,bigint,bytea,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.load_stripe_refund(uuid,bigint,bytea,text) FROM PUBLIC;
-- 0096 moved these two from the shared commerce_worker to the payment worker authorities (SANDBOX and LIVE lanes).
GRANT EXECUTE ON FUNCTION integration.load_stripe_credential(uuid,bigint,bytea,text),
 integration.load_stripe_refund(uuid,bigint,bytea,text) TO commerce_payment_worker,commerce_payment_live;

-- ---------------------------------------------------------------------------------------------------------
-- COMMENT ON (PROCESS §5): owner package, callers, non-goals
-- ---------------------------------------------------------------------------------------------------------
COMMENT ON FUNCTION integration.guard_derived_stripe_account() IS 'integration owner (registry writer definer); a derived Stripe connection must mirror the designated platform account and its pointer is immutable; no network';
COMMENT ON FUNCTION integration.guard_derived_stripe_credential() IS 'integration owner (registry writer definer); a derived credential must be a byte copy of the platform envelope at sealed_version (never re-encrypted); no plaintext';
COMMENT ON FUNCTION payments.platform_stripe_valid_qualification(text,text) IS 'payments owner; INTERNAL helper of the platform-Stripe definers, no EXECUTE grant to any login; returns the platform current valid qualification id';
COMMENT ON FUNCTION payments.platform_stripe_state(text) IS 'payments owner; INTERNAL helper, no EXECUTE grant; derives NONE/DESIGNATED/OPEN/CLOSED/REVOKED (contract §2), never stored';
COMMENT ON FUNCTION payments.platform_stripe_refresh_store(uuid,uuid,text,uuid,uuid,text) IS 'payments owner; INTERNAL helper, no EXECUTE grant; caller sets the store scope GUCs; creates or syncs one store derived account chain (byte-copied credential), mirrored qualification and card heads; never a PSP call';
COMMENT ON FUNCTION payments.platform_stripe_fanout(text,text) IS 'payments owner; INTERNAL, no EXECUTE grant; called only from rotate_stripe_key, qualify_stripe_method and revoke_stripe_live on the platform connection; bounded to 2000 enrollments (platform_fanout_too_large); blocked stores skipped';
COMMENT ON FUNCTION payments.platform_stripe_summary(uuid,uuid,text) IS 'payments owner; INTERNAL helper, no EXECUTE grant; merchant-safe projection (no account id, key or approval data)';
COMMENT ON FUNCTION payments.set_platform_stripe(bytea,uuid,text,boolean,text,text,bigint) IS 'internal/payments/platformstripe only; EXECUTE commerce_runtime. billing:manage, GUCs from resolve_access, fresh final fence. Operator-allowlisted stores only (AD-PF2); enable derives account/credential copy/qualification/card heads in one transaction; disable is always allowed; CAS on enrollment version. Never calls Stripe.';
COMMENT ON FUNCTION payments.read_platform_stripe(bytea,uuid,text) IS 'internal/payments/platformstripe only; EXECUTE commerce_runtime. integration:read; platform state, store state, terms, descriptor preview, limits; no account id, key or approval data';
COMMENT ON FUNCTION payments.designate_stripe_platform(uuid,uuid,uuid,uuid,text,text,text,bigint) IS 'payments owner (stripeadmin platform-designate); EXECUTE commerce_payment_registrar only; marks a primary Stripe connection as the environment platform connection; CAS; refuses a move while enrollments exist';
COMMENT ON FUNCTION payments.set_stripe_platform_open(uuid,uuid,uuid,text,boolean,integer,bigint) IS 'payments owner (stripeadmin platform-open/close); EXECUTE commerce_payment_registrar only; opens or closes new enrollments; LIVE open needs verified canary and the four platform attestation codes';
COMMENT ON FUNCTION payments.block_platform_stripe(uuid,uuid,uuid,uuid,uuid,text,boolean,text,text) IS 'payments owner (stripeadmin platform-block/unblock); EXECUTE commerce_payment_registrar only; per-store kill switch: revokes the derived qualification and disables the card method; unblock clears the flag only';
COMMENT ON FUNCTION payments.allow_platform_stripe(uuid,uuid,uuid,uuid,uuid,text,boolean,text,text) IS 'payments owner (stripeadmin platform-allow); EXECUTE commerce_payment_registrar only; AD-PF2 operator allowlist of stores that may self-enable platform Stripe; audited with target ids; withdrawing does not stop sales (use platform-block)';
COMMENT ON FUNCTION payments.stripe_registrar_credential(uuid,uuid,uuid,uuid,bigint) IS 'payments owner; operator registrar reads the sealed API credential envelope at the connection head version, its registered account and the aad_* scope it was sealed under (platform connection for a derived row); ciphertext only, no writes, never plaintext';
COMMENT ON FUNCTION integration.load_stripe_credential(uuid,bigint,bytea,text) IS 'integration owner; commerce_payment_worker/commerce_payment_live read the exact frozen Stripe API credential version under scoped lease plus the aad_* scope it was sealed under (derived rows: the platform connection); never current head or global key';
COMMENT ON FUNCTION integration.load_stripe_refund(uuid,bigint,bytea,text) IS 'integration owner; commerce_payment_worker/commerce_payment_live read the frozen refund and the account current-head API credential under the refund-operation lease (RD13) plus its aad_* scope; never another account key';
