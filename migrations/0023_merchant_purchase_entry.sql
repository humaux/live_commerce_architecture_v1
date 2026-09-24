-- Merchant catalog -> published storefront projection. This reader verifies
-- the merchant token itself: a fabricated app.* scope is not authority.
-- No runtime gets direct domain/proof reads or any publication write.
GRANT SELECT (tenant_id,store_id,published)
    ON control.storefront_publications TO commerce_auth;
GRANT SELECT (tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until)
    ON control.storefront_domains TO commerce_auth;
CREATE POLICY publications_merchant_lookup ON control.storefront_publications
    FOR SELECT TO commerce_auth USING (true);
CREATE POLICY domains_merchant_lookup ON control.storefront_domains
    FOR SELECT TO commerce_auth USING (true);
CREATE INDEX storefront_domains_store_origin
    ON control.storefront_domains(tenant_id,store_id,origin);

CREATE FUNCTION identity.resolve_storefront_origin(p_hash bytea, p_store uuid)
RETURNS TABLE(state text, origin text)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
    access_row record;
    candidate record;
    candidates integer := 0;
    t0 timestamptz;
    t1 timestamptz;
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash) <> 32 OR p_store IS NULL THEN
        RAISE EXCEPTION 'invalid storefront request' USING ERRCODE='PT400';
    END IF;
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'storefront temporarily unavailable' USING ERRCODE='PT503';
    END IF;
    SELECT * INTO access_row
      FROM identity.resolve_access(p_hash,p_store,'catalog:read');
    CASE access_row.access_status
        WHEN 'unauthorized' THEN
            RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401';
        WHEN 'not_found' THEN
            RAISE EXCEPTION 'not found' USING ERRCODE='PT404';
        WHEN 'forbidden' THEN
            RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
        WHEN 'ok' THEN NULL;
        ELSE RAISE EXCEPTION 'storefront temporarily unavailable' USING ERRCODE='PT503';
    END CASE;
    -- Compare text, without casting untrusted GUCs to UUID. Missing/malformed
    -- values deny safely instead of leaking a cast error or widening the scope.
    IF NULLIF(current_setting('app.tenant_id',true),'') IS DISTINCT FROM access_row.tenant_id::text OR
       NULLIF(current_setting('app.store_id',true),'') IS DISTINCT FROM p_store::text OR
       NULLIF(current_setting('app.principal_id',true),'') IS DISTINCT FROM access_row.principal_id::text THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
    END IF;

    t0 := clock_timestamp();
    -- One coherent SELECT filters eligibility BEFORE truncating. Two admitted
    -- candidates are always ambiguous, even if one expires before we return.
    -- A current-price change never rewrites this derived, amount-free URL.
    FOR candidate IN
        SELECT d.origin,d.ownership_verified_at,d.tls_verified_at,d.valid_until
          FROM control.storefront_domains d
          JOIN control.storefront_publications p USING (tenant_id,store_id)
          JOIN control.stores s ON (s.tenant_id,s.id)=(d.tenant_id,d.store_id)
          JOIN control.tenants t ON t.id=d.tenant_id
         WHERE d.tenant_id=access_row.tenant_id AND d.store_id=p_store
           AND s.active AND t.active AND p.published AND d.state='ACTIVE'
           AND d.ownership_verified_at<=t0 AND d.tls_verified_at<=t0
           AND d.valid_until>t0
         ORDER BY d.origin COLLATE "C" LIMIT 2
    LOOP
        candidates := candidates+1;
        IF candidates=2 THEN
            RETURN QUERY SELECT 'domain_selection_required'::text,''::text;
            RETURN;
        END IF;
    END LOOP;
    IF candidates=0 THEN
        RETURN QUERY SELECT 'storefront_unavailable'::text,''::text;
        RETURN;
    END IF;
    -- Do not requery for a replacement when a sole candidate expires. The
    -- captured row proves only this bounded admission, never future requests.
    t1 := clock_timestamp();
    IF candidate.ownership_verified_at>t1 OR candidate.tls_verified_at>t1 OR candidate.valid_until<=t1 THEN
        RETURN QUERY SELECT 'storefront_unavailable'::text,''::text;
        RETURN;
    END IF;
    RETURN QUERY SELECT 'configured'::text,candidate.origin;
END $$;
ALTER FUNCTION identity.resolve_storefront_origin(bytea,uuid) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION identity.resolve_storefront_origin(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.resolve_storefront_origin(bytea,uuid) TO commerce_runtime;
