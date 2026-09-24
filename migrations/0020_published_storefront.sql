-- Read-only published-store admission kernel. No existing store is published,
-- and no application role can provision DNS/TLS or publication facts here.
-- The future audited control-plane writer is a separate production gate.
CREATE TABLE control.storefront_publications (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    published boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    PRIMARY KEY (tenant_id, store_id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id)
);

CREATE TABLE control.storefront_domains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    origin text NOT NULL UNIQUE CHECK (
        octet_length(origin) BETWEEN 11 AND 261 AND right(origin,10) <> '.localhost' AND
        origin COLLATE "C" ~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'
    ),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    state text NOT NULL DEFAULT 'REQUESTED' CHECK (state IN (
        'REQUESTED','OWNERSHIP_PENDING','TLS_PENDING','ACTIVE','SUSPENDED','DETACHED'
    )),
    ownership_verified_at timestamptz CHECK (isfinite(ownership_verified_at)),
    tls_verified_at timestamptz CHECK (isfinite(tls_verified_at)),
    valid_until timestamptz CHECK (isfinite(valid_until)),
    evidence_ref text,
    FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id),
    CHECK (state <> 'ACTIVE' OR (
        ownership_verified_at IS NOT NULL AND tls_verified_at IS NOT NULL AND
        valid_until IS NOT NULL AND evidence_ref IS NOT NULL AND
        length(btrim(evidence_ref)) BETWEEN 1 AND 240 AND
        length(evidence_ref) <= 240 AND
        evidence_ref ~ '[^[:space:]]' AND evidence_ref !~ '[[:cntrl:]]' AND
        valid_until > ownership_verified_at AND valid_until > tls_verified_at
    ))
);

ALTER TABLE control.storefront_publications ENABLE ROW LEVEL SECURITY;
ALTER TABLE control.storefront_publications FORCE ROW LEVEL SECURITY;
ALTER TABLE control.storefront_domains ENABLE ROW LEVEL SECURITY;
ALTER TABLE control.storefront_domains FORCE ROW LEVEL SECURITY;
REVOKE ALL ON control.storefront_publications, control.storefront_domains FROM PUBLIC;
GRANT SELECT ON control.storefront_publications, control.storefront_domains TO commerce_buyer_writer;
CREATE POLICY publications_lookup ON control.storefront_publications
    FOR SELECT TO commerce_buyer_writer USING (true);
CREATE POLICY domains_lookup ON control.storefront_domains
    FOR SELECT TO commerce_buyer_writer USING (true);

CREATE FUNCTION buyer.resolve_published_store(p_origin text)
RETURNS TABLE(domain_id uuid, store_id uuid, domain_version bigint,
              publication_version bigint, origin text)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE route record; admitted_at timestamptz;
BEGIN
    -- Match the Go boundary exactly. No normalization, default store, forwarded
    -- host interpretation, wildcard expansion, or arbitrary client store ID.
    IF p_origin IS NULL OR octet_length(p_origin) NOT BETWEEN 11 AND 261 OR right(p_origin,10) = '.localhost' OR
       p_origin COLLATE "C" !~ '^https://([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$' THEN
        RAISE EXCEPTION 'invalid storefront origin' USING ERRCODE='PT400';
    END IF;

    -- One coherent statement snapshot. The result admits this bounded request,
    -- not future requests: every new request must resolve again after a stop.
    SELECT d.id, d.store_id, d.version AS domain_version,
           p.version AS publication_version, d.origin,
           d.ownership_verified_at, d.tls_verified_at, d.valid_until
      INTO route
      FROM control.storefront_domains d
      JOIN control.storefront_publications p USING (tenant_id, store_id)
      JOIN control.stores s ON (s.tenant_id, s.id) = (d.tenant_id, d.store_id)
      JOIN control.tenants t ON t.id = d.tenant_id
     WHERE d.origin = p_origin AND d.state = 'ACTIVE' AND p.published
       AND s.active AND t.active;
    IF NOT FOUND THEN RETURN; END IF;
    admitted_at := clock_timestamp();
    IF route.ownership_verified_at > admitted_at OR route.tls_verified_at > admitted_at
       OR route.valid_until <= admitted_at THEN RETURN; END IF;
    RETURN QUERY SELECT route.id, route.store_id, route.domain_version,
                        route.publication_version, route.origin;
END $$;
ALTER FUNCTION buyer.resolve_published_store(text) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.resolve_published_store(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.resolve_published_store(text) TO commerce_buyer_issuer;
