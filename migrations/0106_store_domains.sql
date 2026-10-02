-- 0106 store domains (R5 unit store-domains; docs/delivery/units/store-domains.md).
--
-- Every store gets an address automatically; merchants can bring their own domain. Six Decisions:
--   D1 store handle: control.stores.handle (lower-case ASCII, UNIQUE platform-wide, reserved list refused), assigned by a
--      BEFORE INSERT trigger from the store name, backfilled for existing stores, suffixed -2/-3 on collision.
--   D2 platform address: ensure_store_platform_domain writes an ACTIVE https://<handle>.<base> row (evidence platform-subdomain);
--      the platform owns that zone so ownership is implied and the merchant is reachable the moment they publish.
--   D3 merchant self-service domain: request_merchant_domain (REQUESTED + TXT token + CNAME/A instructions), the DNS/TLS
--      worker transitions (registrar EXECUTE), merchant suspend/detach; one primary origin per store (merchant ACTIVE else platform).
--   D4 edge TLS ask: resolve_storefront_ask answers only ACTIVE or TLS_PENDING origins (Caddy on_demand_tls ask endpoint).
--   D5 security: tokens/columns never logged by these definers (no token in audit rows).
--   D6 forward-only; merchant definers follow the 0081 p_hash + resolve_access + WithScope re-check pattern (integration:manage).
--
-- Roles reused: commerce_identity_writer (owns handle helpers/trigger), commerce_storefront_writer (owns domain definers),
-- commerce_storefront_registrar (EXECUTE the DNS/TLS worker transitions), commerce_runtime (merchant domain routes),
-- commerce_identity (onboarding login, EXECUTE ensure_store_platform_domain), commerce_buyer_issuer (301 primary lookup).
-- commerce_worker stays retired: nothing here grants to it.
-- The onboarding login (commerce_identity) calls control.suggest_store_handle and control.ensure_store_platform_domain
-- only; it needs USAGE on control to name them, and gains no table privilege from it.
GRANT USAGE ON SCHEMA control TO commerce_identity;

-- ---------------------------------------------------------------------------------------
-- D1: the store handle. Nullable so existing rows (and any foreign insert) stay valid until backfilled; the
-- BEFORE INSERT trigger below fills it. Format = exactly internal/storehandles.Valid: 3..30 lower-case ASCII,
-- letters/digits with single interior hyphens, no leading/trailing hyphen, no reserved word, no xn-- punycode.
-- ---------------------------------------------------------------------------------------
ALTER TABLE control.stores ADD COLUMN handle text
    CONSTRAINT stores_handle_format CHECK (
        handle IS NULL OR handle ~ '^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$');
CREATE UNIQUE INDEX stores_handle_unique ON control.stores(handle) WHERE handle IS NOT NULL;
COMMENT ON COLUMN control.stores.handle IS
 '0106 R5: the platform-wide store address handle (https://<handle>.<LC_STORE_BASE_DOMAIN>). Lower-case ASCII, 3..30 chars, single interior hyphens, no reserved word/xn--. Assigned by control.stores_handle_default from the store name; UNIQUE while set; changed only while never published (operator-only afterwards, D3).';

-- Reserved words are case-normalised before the comparison. `stores` is the CNAME apex; anything xn-- is punycode.
CREATE FUNCTION control.store_handle_reserved(p_handle text) RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path=pg_catalog AS $$
    SELECT p_handle = ANY (ARRAY['www','admin','api','hooks','shop','mail','static','cdn','assets','app','help','support','status','stores'])
        OR p_handle LIKE 'xn--%'
$$;
ALTER FUNCTION control.store_handle_reserved(text) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION control.store_handle_reserved(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.store_handle_reserved(text) TO commerce_identity_writer;
COMMENT ON FUNCTION control.store_handle_reserved(text) IS
 '0106 D1: true when p_handle (already lower-cased) is a reserved platform word or punycode (xn--). Immutable; called by assign/suggest/trigger.';

-- The slug from a display name: lower-case, every run of non-alphanumerics becomes one hyphen, trimmed. A non-ASCII
-- name (e.g. a Chinese store name) yields the empty string and the caller falls back to store-<id8>.
CREATE FUNCTION control.slug_store_handle(p_name text) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path=pg_catalog AS $$
    SELECT lower(regexp_replace(regexp_replace(regexp_replace(coalesce(p_name,''), '[^a-zA-Z0-9]+', '-', 'g'), '^-+', '', 'g'), '-+$', '', 'g'))
$$;
ALTER FUNCTION control.slug_store_handle(text) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION control.slug_store_handle(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.slug_store_handle(text) TO commerce_identity_writer;
COMMENT ON FUNCTION control.slug_store_handle(text) IS
 '0106 D1: slug of a store display name for the handle (lower-case ASCII, runs of non-alphanumerics -> one hyphen). Empty for non-ASCII names; callers fall back to store-<id8>.';

CREATE FUNCTION control.store_handle_valid(p_handle text) RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE SET search_path=pg_catalog AS $$
    SELECT p_handle ~ '^[a-z0-9]([a-z0-9-]{1,28}[a-z0-9])$' AND NOT control.store_handle_reserved(p_handle)
$$;
ALTER FUNCTION control.store_handle_valid(text) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION control.store_handle_valid(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.store_handle_valid(text) TO commerce_identity_writer;
COMMENT ON FUNCTION control.store_handle_valid(text) IS
 '0106 D1: format + reserved check in one predicate (parity with internal/storehandles.Valid).';

-- control.assign_store_handle: the single writer of a derived handle. SECURITY DEFINER as commerce_identity_writer
-- (which holds SELECT/INSERT on control.stores via 0004 policies), so the BEFORE INSERT trigger can always read the
-- existing handle set for the uniqueness/suffix check regardless of the inserting role. Falls back to store-<first 8 hex
-- of id> when the slug is empty or reserved; suffixes -2,-3,... (bounded) on collision. The unique index is the backstop
-- for a concurrent race; the suffix loop only resolves committed collisions.
CREATE FUNCTION control.assign_store_handle(p_name text, p_id uuid) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE base text; cand text; i int := 2;
BEGIN
    base := control.slug_store_handle(p_name);
    IF base = '' OR length(base) < 3 OR control.store_handle_reserved(base) THEN
        base := 'store-' || left(replace(p_id::text, '-', ''), 8);
    END IF;
    base := left(base, 30);
    base := regexp_replace(base, '-+$', '', 'g'); -- truncation must not leave a trailing hyphen
    IF base = '' OR length(base) < 3 OR control.store_handle_reserved(base) THEN
        base := 'store-' || left(replace(p_id::text, '-', ''), 8);
    END IF;
    cand := base;
    WHILE EXISTS (SELECT 1 FROM control.stores WHERE handle = cand) LOOP
        IF i > 9999 THEN RAISE EXCEPTION 'handle space exhausted' USING ERRCODE='PT409'; END IF;
        cand := left(base, 30 - length('-' || i::text)) || '-' || i;
        i := i + 1;
    END LOOP;
    RETURN cand;
END $$;
ALTER FUNCTION control.assign_store_handle(text,uuid) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION control.assign_store_handle(text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.assign_store_handle(text,uuid) TO commerce_identity_writer;
COMMENT ON FUNCTION control.assign_store_handle(text,uuid) IS
 '0106 D1: derive a free handle from a store name + id (slug, store-<id8> fallback, -2/-3 suffix on committed collision). SECURITY DEFINER commerce_identity_writer; called by the BEFORE INSERT trigger and the 0106 backfill. Reads control.stores.handle only (uniqueness/suffix); the unique index remains the concurrency backstop.';

CREATE FUNCTION control.stores_handle_default() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
    IF NEW.handle IS NULL THEN
        NEW.handle := control.assign_store_handle(NEW.name, NEW.id);
    ELSIF NOT control.store_handle_valid(NEW.handle) THEN
        RAISE EXCEPTION 'invalid store handle' USING ERRCODE='PT400';
    END IF;
    RETURN NEW;
END $$;
ALTER FUNCTION control.stores_handle_default() OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION control.stores_handle_default() FROM PUBLIC;
COMMENT ON FUNCTION control.stores_handle_default() IS
 '0106 D1: BEFORE INSERT trigger on control.stores; fills a missing handle from the name and refuses an explicit reserved/invalid one. SECURITY DEFINER commerce_identity_writer.';

CREATE TRIGGER stores_handle_before_insert
    BEFORE INSERT ON control.stores
    FOR EACH ROW EXECUTE FUNCTION control.stores_handle_default();
COMMENT ON TRIGGER stores_handle_before_insert ON control.stores IS
 '0106 D1: assign/validate the handle on every store insert (onboarding via identity.create_initial_store and any direct insert).';

-- Backfill handles for stores that predate 0106. Runs once at migration time (fresh databases have none).
DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT id, name FROM control.stores WHERE handle IS NULL ORDER BY id LOOP
        UPDATE control.stores SET handle = control.assign_store_handle(r.name, r.id) WHERE id = r.id;
    END LOOP;
END $$;

-- The live availability/suggest probe for onboarding: the slug the wizard previews and whether it is taken.
CREATE FUNCTION control.suggest_store_handle(p_name text, p_id text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE base text;
BEGIN
    IF p_id IS NULL OR p_id !~ '^[0-9a-f]{32}$' THEN RAISE EXCEPTION 'invalid handle suggestion' USING ERRCODE='PT400'; END IF;
    base := control.slug_store_handle(p_name);
    IF base = '' OR length(base) < 3 OR control.store_handle_reserved(base) THEN
        base := 'store-' || left(p_id, 8);
    END IF;
    base := left(base, 30);
    base := regexp_replace(base, '-+$', '', 'g');
    IF base = '' OR length(base) < 3 OR control.store_handle_reserved(base) THEN
        base := 'store-' || left(p_id, 8);
    END IF;
    RETURN jsonb_build_object('suggested', base,
        'available', NOT EXISTS (SELECT 1 FROM control.stores WHERE handle = base));
END $$;
ALTER FUNCTION control.suggest_store_handle(text,text) OWNER TO commerce_identity_writer;
REVOKE ALL ON FUNCTION control.suggest_store_handle(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.suggest_store_handle(text,text) TO commerce_identity;
COMMENT ON FUNCTION control.suggest_store_handle(text,text) IS
 '0106 D1: onboarding slug preview + live availability for a proposed store name (no write). EXECUTE commerce_identity (the onboarding login). p_id is a 32-hex caller nonce used only for the store-<id8> fallback display.';

-- ---------------------------------------------------------------------------------------
-- D3 merchant self-service domain: extra columns on control.storefront_domains for the REQUESTED lifecycle.
-- verification_token is the TXT value (_lc-verify.<host>); verify_deadline = requested_at + 72h (backoff bound).
-- ---------------------------------------------------------------------------------------
ALTER TABLE control.storefront_domains
    ADD COLUMN verification_token text CHECK (verification_token IS NULL OR verification_token ~ '^[A-Za-z0-9_-]{43}$'),
    ADD COLUMN requested_at timestamptz CHECK (isfinite(requested_at)),
    ADD COLUMN verify_deadline timestamptz CHECK (isfinite(verify_deadline)),
    ADD COLUMN last_checked_at timestamptz CHECK (isfinite(last_checked_at)),
    ADD COLUMN dns_failures integer NOT NULL DEFAULT 0 CHECK (dns_failures >= 0);
COMMENT ON COLUMN control.storefront_domains.verification_token IS
 '0106 D3: the TXT verification value for a REQUESTED merchant domain (never logged; cleared on the ACTIVE transition).';
COMMENT ON COLUMN control.storefront_domains.requested_at IS '0106 D3: when the merchant requested this origin (backoff window anchor).';
COMMENT ON COLUMN control.storefront_domains.verify_deadline IS '0106 D3: requested_at + 72h; DNS verification stops after it (the merchant re-requests for a fresh token).';
COMMENT ON COLUMN control.storefront_domains.last_checked_at IS '0106 D3: last DNS check attempt (worker backoff).';
COMMENT ON COLUMN control.storefront_domains.dns_failures IS '0106 D3: consecutive DNS verification failures since the last success.';

GRANT INSERT(verification_token,requested_at,verify_deadline,last_checked_at,dns_failures)
  ON control.storefront_domains TO commerce_storefront_writer;
GRANT UPDATE(verification_token,requested_at,verify_deadline,last_checked_at,dns_failures)
  ON control.storefront_domains TO commerce_storefront_writer;
-- The merchant definers and the platform-address ensure both need to see the store handle/name for origin building.
GRANT SELECT(handle) ON control.stores TO commerce_storefront_writer;
COMMENT ON POLICY storefront_writer_domains_insert ON control.storefront_domains IS
 '0081 + 0106: definer owner insert (operator bind ACTIVE, merchant request REQUESTED, platform ensure ACTIVE).';
COMMENT ON POLICY storefront_writer_domains_update ON control.storefront_domains IS
 '0081 + 0106: definer owner lifecycle moves (operator bind/suspend/detach, merchant suspend/detach, DNS/TLS worker transitions, merchant re-request refresh); each bumps version.';
-- Extend the 0081 audit policy (same policy, same count) with the merchant self-service domain actions.
ALTER POLICY storefront_writer_audit_insert ON ops.audit_events WITH CHECK (action IN (
  'merchant.storefront_published','merchant.storefront_unpublished',
  'operator.domain_bound','operator.domain_bound:rebind_from_detached','operator.domain_suspended','operator.domain_detached',
  'merchant.domain_requested','merchant.domain_suspended','merchant.domain_detached'));
COMMENT ON POLICY storefront_writer_audit_insert ON ops.audit_events IS
 '0081 + 0106: the nine fixed storefront audit actions (six operator/merchant publication + three merchant self-service domain).';

-- ---------------------------------------------------------------------------------------
-- D2: control.ensure_store_platform_domain. Idempotent writer of the ACTIVE platform subdomain row
-- (https://<handle>.<base>, evidence platform-subdomain). Called by the identity onboarding flow right after
-- identity.create_initial_store (same tx, commerce_identity) and by backfill_platform_domains (registrar).
-- The platform owns that zone so ownership/TLS are implied; valid_until is a 10-year formality because the
-- wildcard certificate is Caddy-managed and auto-renews (no SQL renewal path exists). No audit row (not a
-- merchant-visible write). Returns {handle, origin} (origin NULL when no base domain is configured).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION control.ensure_store_platform_domain(p_store uuid, p_base_domain text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_handle text; v_origin text; v_now timestamptz;
BEGIN
    IF p_store IS NULL THEN RAISE EXCEPTION 'invalid store request' USING ERRCODE='PT400'; END IF;
    SELECT tenant_id, handle INTO v_tenant, v_handle FROM control.stores WHERE id = p_store;
    IF NOT FOUND THEN RAISE EXCEPTION 'store not found' USING ERRCODE='PT404'; END IF;
    IF v_handle IS NULL THEN RAISE EXCEPTION 'store has no handle' USING ERRCODE='PT409'; END IF;
    IF p_base_domain IS NULL OR btrim(p_base_domain) = '' THEN
        RETURN jsonb_build_object('handle', v_handle, 'origin', NULL);
    END IF;
    v_origin := 'https://' || v_handle || '.' || lower(p_base_domain);
    IF octet_length(v_origin) NOT BETWEEN 11 AND 261 OR right(v_origin,10) = '.localhost'
       OR v_origin COLLATE "C" !~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' THEN
        RAISE EXCEPTION 'invalid base domain' USING ERRCODE='PT400';
    END IF;
    -- One writer per origin at a time, then idempotent under the lock.
    PERFORM pg_advisory_xact_lock(hashtextextended('storefront-domain:'||v_origin,0));
    IF EXISTS (SELECT 1 FROM control.storefront_domains d
               WHERE d.origin = v_origin AND d.state = 'ACTIVE' AND d.evidence_ref = 'platform-subdomain' AND d.store_id = p_store) THEN
        RETURN jsonb_build_object('handle', v_handle, 'origin', v_origin);
    END IF;
    v_now := clock_timestamp();
    INSERT INTO control.storefront_domains(tenant_id, store_id, origin, state,
        ownership_verified_at, tls_verified_at, valid_until, evidence_ref)
    VALUES (v_tenant, p_store, v_origin, 'ACTIVE', v_now, v_now, v_now + interval '3650 days', 'platform-subdomain')
    ON CONFLICT (origin) DO NOTHING;
    RETURN jsonb_build_object('handle', v_handle, 'origin', v_origin);
END $$;
ALTER FUNCTION control.ensure_store_platform_domain(uuid,text) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.ensure_store_platform_domain(uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.ensure_store_platform_domain(uuid,text) TO commerce_identity;
GRANT EXECUTE ON FUNCTION control.ensure_store_platform_domain(uuid,text) TO commerce_runtime;
GRANT EXECUTE ON FUNCTION control.ensure_store_platform_domain(uuid,text) TO commerce_storefront_registrar;
COMMENT ON FUNCTION control.ensure_store_platform_domain(uuid,text) IS
 '0106 D2: idempotent ACTIVE platform subdomain (https://<handle>.<base>, evidence platform-subdomain). EXECUTE commerce_identity (onboarding, same tx as identity.create_initial_store), commerce_runtime (lazy admin Settings) and commerce_storefront_registrar (backfill). Cross-domain call: commerce_identity -> this commerce_storefront_writer definer, so the identity login gains no direct storefront_domains write.';

CREATE FUNCTION control.backfill_platform_domains(p_base_domain text) RETURNS bigint
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r record; n bigint := 0;
BEGIN
    FOR r IN SELECT id FROM control.stores WHERE handle IS NOT NULL ORDER BY id LOOP
        PERFORM control.ensure_store_platform_domain(r.id, p_base_domain);
        n := n + 1;
    END LOOP;
    RETURN n;
END $$;
ALTER FUNCTION control.backfill_platform_domains(text) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.backfill_platform_domains(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.backfill_platform_domains(text) TO commerce_storefront_registrar;
COMMENT ON FUNCTION control.backfill_platform_domains(text) IS
 '0106 D2: one-shot backfill of the platform subdomain for every existing handled store (registrar/operator one-shot, idempotent). Returns the number of stores visited.';

-- ---------------------------------------------------------------------------------------
-- D3 merchant request / read / suspend / detach. Same p_hash + resolve_access(integration:manage) + WithScope
-- re-check as 0081 read_storefront/set_storefront_published. The token is generated by Go and passed in; these
-- definers never emit it into audit or logs.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION control.request_merchant_domain(p_hash bytea,p_store uuid,p_hostname text,p_base_domain text,p_token text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_origin text; v_host text; v_now timestamptz; v_deadline timestamptz; v_row record; v_id uuid; v_version bigint;
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_token IS NULL OR p_token !~ '^[A-Za-z0-9_-]{43}$' THEN
        RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
    SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
    IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
    IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
     OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
     OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    IF p_base_domain IS NULL OR btrim(p_base_domain)='' THEN RAISE EXCEPTION 'base domain not configured' USING ERRCODE='PT409'; END IF;
    v_host := lower(btrim(p_hostname));
    IF v_host IS NULL OR length(v_host) > 253
       OR v_host COLLATE "C" !~ '^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' THEN
        RAISE EXCEPTION 'invalid hostname' USING ERRCODE='PT400'; END IF;
    -- A merchant cannot bring a domain that lives under the platform's own base (the platform subdomain zone).
    IF v_host = lower(p_base_domain) OR v_host LIKE '%.' || lower(p_base_domain) THEN
        RAISE EXCEPTION 'reserved hostname' USING ERRCODE='PT409'; END IF;
    v_origin := 'https://' || v_host;
    IF octet_length(v_origin) NOT BETWEEN 11 AND 261 OR right(v_origin,10) = '.localhost'
       OR v_origin COLLATE "C" !~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' THEN
        RAISE EXCEPTION 'invalid hostname' USING ERRCODE='PT400'; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('storefront-domain:'||v_origin,0));
    SELECT d.id,d.tenant_id,d.store_id,d.state,d.version INTO v_row FROM control.storefront_domains d WHERE d.origin=v_origin FOR UPDATE;
    IF FOUND THEN
        IF v_row.store_id <> p_store THEN RAISE EXCEPTION 'domain_owned_elsewhere' USING ERRCODE='PT409'; END IF;
        IF v_row.state IN ('ACTIVE','SUSPENDED','DETACHED') THEN
            RAISE EXCEPTION USING MESSAGE = CASE v_row.state WHEN 'ACTIVE' THEN 'domain_active' WHEN 'SUSPENDED' THEN 'domain_suspended' ELSE 'domain_detached' END, ERRCODE='PT409';
        END IF;
        -- A REQUESTED/OWNERSHIP_PENDING/TLS_PENDING row of the same store: re-request refreshes the token and deadline.
        v_now := clock_timestamp();
        UPDATE control.storefront_domains SET verification_token=p_token, requested_at=v_now, verify_deadline=v_now + interval '72 hours',
            last_checked_at=NULL, dns_failures=0, version=version+1
         WHERE id=v_row.id RETURNING id,version INTO v_id,v_version;
    ELSE
        v_now := clock_timestamp();
        INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,verification_token,requested_at,verify_deadline)
        VALUES (s.tenant_id,p_store,v_origin,'REQUESTED',p_token,v_now,v_now + interval '72 hours') RETURNING id,version INTO v_id,v_version;
    END IF;
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'merchant.domain_requested');
    RETURN jsonb_build_object('domain_id',v_id,'version',v_version,'state','REQUESTED','origin',v_origin,
      'dns',jsonb_build_object(
        'txt_name','_lc-verify.'||v_host,'txt_value',p_token,
        'cname_target','stores.'||lower(p_base_domain),
        'apex',array_length(regexp_split_to_array(v_host,'\.'),1) < 3));
END $$;
ALTER FUNCTION control.request_merchant_domain(bytea,uuid,text,text,text) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.request_merchant_domain(bytea,uuid,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.request_merchant_domain(bytea,uuid,text,text,text) TO commerce_runtime;
COMMENT ON FUNCTION control.request_merchant_domain(bytea,uuid,text,text,text) IS
 '0106 D3: internal/storefrontdomains Request; EXECUTE commerce_runtime (admin Settings POST storefront/domains). integration:manage. Enters a REQUESTED merchant origin with a fresh TXT token + CNAME/A DNS instructions; re-requesting a pending row refreshes the token/deadline. Refuses the platform base zone, a foreign store origin (PT409 domain_owned_elsewhere) and ACTIVE/SUSPENDED/DETACHED rows. Never logs the token.';

CREATE FUNCTION control.read_store_domains(p_hash bytea,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_domains jsonb;
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL THEN
        RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
    SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:read');
    IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
    IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
     OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
     OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    SELECT coalesce(jsonb_agg(jsonb_build_object('origin',d.origin,'state',d.state,'version',d.version,
      'token',d.verification_token,
      'verify_deadline',CASE WHEN d.verify_deadline IS NULL THEN NULL ELSE to_char(d.verify_deadline AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') END,
      'serving',coalesce(d.state='ACTIVE' AND d.valid_until>clock_timestamp(),false)) ORDER BY d.origin),'[]'::jsonb) INTO v_domains
     FROM control.storefront_domains d WHERE d.tenant_id=s.tenant_id AND d.store_id=p_store;
    RETURN jsonb_build_object('domains',v_domains);
END $$;
ALTER FUNCTION control.read_store_domains(bytea,uuid) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.read_store_domains(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.read_store_domains(bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION control.read_store_domains(bytea,uuid) IS
 '0106 D3: internal/storefrontdomains Read; EXECUTE commerce_runtime (admin Settings GET storefront/domains). integration:read. Every domain row of the store (origin, state, version, token for pending rows, verify deadline, serving). The token is the merchant''s own; never logged.';

CREATE FUNCTION control.suspend_merchant_domain(p_hash bytea,p_store uuid,p_origin text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_row record; v_version bigint;
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
       OR p_origin IS NULL OR octet_length(p_origin) NOT BETWEEN 11 AND 261 OR right(p_origin,10)='.localhost'
       OR p_origin COLLATE "C" !~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' THEN
        RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
    SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
    IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
    IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
     OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
     OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('storefront-domain:'||p_origin,0));
    SELECT d.id,d.store_id,d.state,d.version INTO v_row FROM control.storefront_domains d WHERE d.origin=p_origin FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'domain not found' USING ERRCODE='PT404'; END IF;
    IF v_row.store_id <> p_store THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    IF v_row.state='DETACHED' THEN RAISE EXCEPTION 'domain_detached' USING ERRCODE='PT409'; END IF;
    IF v_row.state='SUSPENDED' THEN
        RETURN jsonb_build_object('domain_id',v_row.id,'version',v_row.version,'state','SUSPENDED','changed',false);
    END IF;
    UPDATE control.storefront_domains SET state='SUSPENDED',version=version+1 WHERE id=v_row.id RETURNING version INTO v_version;
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'merchant.domain_suspended');
    RETURN jsonb_build_object('domain_id',v_row.id,'version',v_version,'state','SUSPENDED','changed',true);
END $$;
ALTER FUNCTION control.suspend_merchant_domain(bytea,uuid,text) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.suspend_merchant_domain(bytea,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.suspend_merchant_domain(bytea,uuid,text) TO commerce_runtime;
COMMENT ON FUNCTION control.suspend_merchant_domain(bytea,uuid,text) IS
 '0106 D3: internal/storefrontdomains Suspend; EXECUTE commerce_runtime. integration:manage. Merchant suspends its own non-DETACHED origin (resolver denies on the next request). DETACHED = PT409 domain_detached; another store''s origin = PT403.';

CREATE FUNCTION control.detach_merchant_domain(p_hash bytea,p_store uuid,p_origin text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_row record; v_version bigint;
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL
       OR p_origin IS NULL OR octet_length(p_origin) NOT BETWEEN 11 AND 261 OR right(p_origin,10)='.localhost'
       OR p_origin COLLATE "C" !~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' THEN
        RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
    SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
    IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
    IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
    IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
     OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
     OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('storefront-domain:'||p_origin,0));
    SELECT d.id,d.store_id,d.state,d.version INTO v_row FROM control.storefront_domains d WHERE d.origin=p_origin FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'domain not found' USING ERRCODE='PT404'; END IF;
    IF v_row.store_id <> p_store THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
    IF v_row.state='DETACHED' THEN
        RETURN jsonb_build_object('domain_id',v_row.id,'version',v_row.version,'state','DETACHED','changed',false);
    END IF;
    UPDATE control.storefront_domains SET state='DETACHED',version=version+1 WHERE id=v_row.id RETURNING version INTO v_version;
    INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'merchant.domain_detached');
    RETURN jsonb_build_object('domain_id',v_row.id,'version',v_version,'state','DETACHED','changed',true);
END $$;
ALTER FUNCTION control.detach_merchant_domain(bytea,uuid,text) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.detach_merchant_domain(bytea,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.detach_merchant_domain(bytea,uuid,text) TO commerce_runtime;
COMMENT ON FUNCTION control.detach_merchant_domain(bytea,uuid,text) IS
 '0106 D3: internal/storefrontdomains Detach; EXECUTE commerce_runtime. integration:manage. Merchant finally detaches its own origin. DETACHED = no-op; another store''s origin = PT403.';

-- ---------------------------------------------------------------------------------------
-- D3 worker transitions (EXECUTE commerce_storefront_registrar; the operator CLI sweep cmd/store-admin domain-verify
-- is the "worker"): DNS advance (REQUESTED -> OWNERSHIP_PENDING -> TLS_PENDING), TLS completion (-> ACTIVE), and the
-- two pending batch reads. No token leaves these functions.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION control.store_domain_dns_advance(p_domain_id uuid,p_matched boolean)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_row record; v_now timestamptz; v_state text;
BEGIN
    IF p_domain_id IS NULL OR p_matched IS NULL THEN RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
    SELECT d.id,d.state,d.version,d.dns_failures,d.verify_deadline INTO v_row FROM control.storefront_domains d WHERE d.id=p_domain_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'domain not found' USING ERRCODE='PT404'; END IF;
    v_now := clock_timestamp();
    IF p_matched THEN
        IF v_row.state='REQUESTED' THEN
            v_state := 'OWNERSHIP_PENDING';
            UPDATE control.storefront_domains SET state='OWNERSHIP_PENDING',ownership_verified_at=v_now,last_checked_at=v_now,dns_failures=0,version=version+1
             WHERE id=p_domain_id;
        ELSIF v_row.state='OWNERSHIP_PENDING' THEN
            v_state := 'TLS_PENDING';
            UPDATE control.storefront_domains SET state='TLS_PENDING',last_checked_at=v_now,dns_failures=0,version=version+1
             WHERE id=p_domain_id;
        ELSE
            v_state := v_row.state;
        END IF;
    ELSE
        v_state := v_row.state;
        UPDATE control.storefront_domains SET last_checked_at=v_now,dns_failures=dns_failures+1,version=version+1 WHERE id=p_domain_id;
    END IF;
    RETURN jsonb_build_object('domain_id',p_domain_id,'state',v_state,'expired',coalesce(v_row.verify_deadline <= v_now,false),'dns_failures',CASE WHEN p_matched THEN 0 ELSE v_row.dns_failures+1 END);
END $$;
ALTER FUNCTION control.store_domain_dns_advance(uuid,boolean) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.store_domain_dns_advance(uuid,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.store_domain_dns_advance(uuid,boolean) TO commerce_storefront_registrar;
COMMENT ON FUNCTION control.store_domain_dns_advance(uuid,boolean) IS
 '0106 D3: worker DNS transition (EXECUTE commerce_storefront_registrar). match: REQUESTED -> OWNERSHIP_PENDING, OWNERSHIP_PENDING -> TLS_PENDING; miss: last_checked_at + dns_failures+1. Returns the new state and whether the verify deadline has passed (expired).';

CREATE FUNCTION control.store_domain_tls_complete(p_domain_id uuid,p_valid_until timestamptz)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_row record; v_now timestamptz;
BEGIN
    IF p_domain_id IS NULL OR p_valid_until IS NULL OR NOT isfinite(p_valid_until) THEN
        RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
    v_now := clock_timestamp();
    IF p_valid_until<=v_now OR p_valid_until>v_now+interval '400 days' THEN
        RAISE EXCEPTION 'invalid domain request' USING ERRCODE='PT400'; END IF;
    SELECT d.id,d.state,d.version INTO v_row FROM control.storefront_domains d WHERE d.id=p_domain_id FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'domain not found' USING ERRCODE='PT404'; END IF;
    IF v_row.state <> 'TLS_PENDING' THEN RAISE EXCEPTION 'domain not pending tls' USING ERRCODE='PT409'; END IF;
    UPDATE control.storefront_domains SET state='ACTIVE',tls_verified_at=v_now,valid_until=p_valid_until,
        verification_token=NULL,evidence_ref='merchant-tls',version=version+1 WHERE id=p_domain_id;
    RETURN jsonb_build_object('domain_id',p_domain_id,'state','ACTIVE','version',v_row.version+1);
END $$;
ALTER FUNCTION control.store_domain_tls_complete(uuid,timestamptz) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.store_domain_tls_complete(uuid,timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.store_domain_tls_complete(uuid,timestamptz) TO commerce_storefront_registrar;
COMMENT ON FUNCTION control.store_domain_tls_complete(uuid,timestamptz) IS
 '0106 D3: worker TLS transition (EXECUTE commerce_storefront_registrar): TLS_PENDING -> ACTIVE with the probe''s certificate notAfter as valid_until (bounded 400 days), evidence merchant-tls, token cleared.';

CREATE FUNCTION control.next_store_domain_dns_check()
RETURNS TABLE(domain_id uuid, origin text, hostname text, token text, state text, verify_deadline timestamptz, last_checked_at timestamptz, dns_failures integer)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
    SELECT d.id, d.origin, substring(d.origin from 9), d.verification_token, d.state, d.verify_deadline, d.last_checked_at, d.dns_failures
      FROM control.storefront_domains d
     WHERE d.state='REQUESTED' AND d.verify_deadline > clock_timestamp()
     ORDER BY d.id
$$;
ALTER FUNCTION control.next_store_domain_dns_check() OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.next_store_domain_dns_check() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.next_store_domain_dns_check() TO commerce_storefront_registrar;
COMMENT ON FUNCTION control.next_store_domain_dns_check() IS
 '0106 D3: worker batch read (EXECUTE commerce_storefront_registrar): REQUESTED rows still inside their 72h window. Returns the hostname (origin without the https:// scheme) and token for the DNS lookup.';

CREATE FUNCTION control.next_store_domain_tls_probe()
RETURNS TABLE(domain_id uuid, origin text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
    SELECT d.id, d.origin FROM control.storefront_domains d WHERE d.state='TLS_PENDING' ORDER BY d.id
$$;
ALTER FUNCTION control.next_store_domain_tls_probe() OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.next_store_domain_tls_probe() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.next_store_domain_tls_probe() TO commerce_storefront_registrar;
COMMENT ON FUNCTION control.next_store_domain_tls_probe() IS
 '0106 D3: worker batch read (EXECUTE commerce_storefront_registrar): TLS_PENDING rows whose certificate is being (re)issued; the TLS probe completes them.';

-- ---------------------------------------------------------------------------------------
-- D4 edge TLS ask: Caddy on_demand_tls { ask http://api:<port>/internal/tls-ask } answers 200 only for a hostname
-- that maps to an ACTIVE origin or a merchant origin in TLS_PENDING. Fail closed otherwise. EXECUTE commerce_runtime
-- (the internal-network api login; rate limit + negative cache live in internal/tlsask).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION control.resolve_storefront_ask(p_host text) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
    SELECT p_host IS NOT NULL AND length(p_host) <= 253
       AND p_host COLLATE "C" ~ '^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'
       AND EXISTS (SELECT 1 FROM control.storefront_domains d
                   WHERE d.origin = 'https://' || lower(p_host) AND d.state IN ('ACTIVE','TLS_PENDING'))
$$;
ALTER FUNCTION control.resolve_storefront_ask(text) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.resolve_storefront_ask(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.resolve_storefront_ask(text) TO commerce_runtime;
COMMENT ON FUNCTION control.resolve_storefront_ask(text) IS
 '0106 D4: internal/tlsask check (EXECUTE commerce_runtime, internal network only). True only for an ACTIVE platform/merchant origin or a TLS_PENDING merchant origin; unknown/DETACHED/SUSPENDED/REQUESTED hosts fail closed.';

-- ---------------------------------------------------------------------------------------
-- D3 301-to-primary: the primary ACTIVE origin of the store that p_origin resolves to (a merchant ACTIVE row first,
-- else the platform subdomain); NULL when p_origin is not ACTIVE or is already the primary (no redirect). Non-primary
-- ACTIVE origins 301 to it. EXECUTE commerce_buyer_runtime (the buyer runtime pool internal/buyerhttp design reads run on).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION control.resolve_primary_origin(p_origin text) RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_store uuid; v_primary text;
BEGIN
    IF p_origin IS NULL THEN RETURN NULL; END IF;
    SELECT d.store_id INTO v_store FROM control.storefront_domains d WHERE d.origin=p_origin AND d.state='ACTIVE';
    IF NOT FOUND THEN RETURN NULL; END IF;
    SELECT d.origin INTO v_primary FROM control.storefront_domains d
     WHERE d.store_id=v_store AND d.state='ACTIVE'
     ORDER BY (d.evidence_ref='platform-subdomain') ASC, d.id ASC LIMIT 1;
    IF v_primary IS NOT DISTINCT FROM p_origin THEN RETURN NULL; END IF;
    RETURN v_primary;
END $$;
ALTER FUNCTION control.resolve_primary_origin(text) OWNER TO commerce_storefront_writer;
REVOKE ALL ON FUNCTION control.resolve_primary_origin(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.resolve_primary_origin(text) TO commerce_buyer_runtime;
COMMENT ON FUNCTION control.resolve_primary_origin(text) IS
 '0106 D3: primary ACTIVE origin for the store of p_origin (merchant ACTIVE first, else platform subdomain); NULL when p_origin is not ACTIVE or is already primary (so no redirect). EXECUTE commerce_buyer_runtime (buyer design read pool) for the buyer-facing 301 lookup.';
