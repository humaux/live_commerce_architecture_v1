-- 0086 catalog v2 (contracts/storefront-v2.md section A; docs/delivery/units/catalog-core.md).
--
-- Owns: product draft status, slug, SEO fields and option axes; SKU option_values and compare_at_minor; the
-- merchant collections (catalog.collections, catalog.collection_products, catalog.collection_images) and the
-- buyer-facing read definers catalog.buyer_v2_products / buyer_v2_product / buyer_v2_collections /
-- buyer_v2_collection / buyer_collection_image (all owned by commerce_catalog_media, 0082).
--
-- Non-goals: no multi-language content, no bulk import, no per-SKU image, no resize/CDN for collection images (same
-- bytea ceiling as 0082), no buyer table privilege (the buyer runtime reaches catalog data only through the
-- definers below; they resolve the store from the verified origin, never from a parameter), no stock write.
--
-- Depends on: 0002 (catalog.products/skus, FORCE RLS scope_access, inventory.balances), 0007 (commerce_buyer_runtime),
-- 0020 (buyer.resolve_published_store), 0082 (commerce_catalog_media, catalog.product_images).
--
-- Callers: internal/catalog (catalog.go, collections.go; commerce_runtime, merchant routes in internal/httpapi),
-- internal/buyerhttp catalogv2.go (commerce_buyer_runtime, no buyer bearer: the storefront BFF key + origin).
--
-- ponytail: buyer list pagination is offset-based (stable enough for a storefront, ceiling = a few thousand products
-- per store); move to keyset on (sort key, id) if a store's catalog grows past that or concurrent edits cause visible
-- skips. Collection membership is capped at 500 products by the position CHECK.

-- ---------------------------------------------------------------------------------------
-- Products: draft status, slug, SEO, option axes.
-- ---------------------------------------------------------------------------------------
ALTER TABLE catalog.products DROP CONSTRAINT products_status_check;
ALTER TABLE catalog.products ADD CONSTRAINT products_status_check CHECK (status IN ('draft','active','archived'));
-- The DB column default stays 'active' on purpose: raw-SQL fixtures and legacy rows keep their meaning. The contract
-- default (new product = draft) is applied by internal/catalog.CreateProduct, the only product writer.

ALTER TABLE catalog.products
    ADD COLUMN slug text,
    ADD COLUMN seo_title text NOT NULL DEFAULT '' CHECK (char_length(seo_title) <= 70),
    ADD COLUMN seo_description text NOT NULL DEFAULT '' CHECK (char_length(seo_description) <= 160),
    ADD COLUMN options jsonb NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(options)='array' AND jsonb_array_length(options) <= 3);

-- Legacy rows get the id prefix as slug (contract: "fall back to the product id prefix"). The table is FORCE RLS and
-- the migration owner has no policy, so the backfill briefly lifts FORCE inside this migration transaction.
ALTER TABLE catalog.products NO FORCE ROW LEVEL SECURITY;
UPDATE catalog.products SET slug = left(replace(id::text,'-',''),12) WHERE slug IS NULL;
ALTER TABLE catalog.products FORCE ROW LEVEL SECURITY;

ALTER TABLE catalog.products ALTER COLUMN slug SET NOT NULL;
-- Slug shape: lowercase alnum words joined by single dashes, <= 80, and never UUID-shaped so that the buyer route
-- /products/{slug_or_id} can tell the two apart.
ALTER TABLE catalog.products ADD CONSTRAINT products_slug_check CHECK (
    char_length(slug) <= 80 AND slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'
    AND slug !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$');
CREATE UNIQUE INDEX products_slug ON catalog.products(tenant_id,store_id,slug);

-- Raw-SQL inserts without a slug (fixtures, older tooling) get the id prefix, exactly like the backfill.
CREATE FUNCTION catalog.products_default_slug() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
    IF NEW.slug IS NULL THEN NEW.slug := left(replace(NEW.id::text,'-',''),12); END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER products_default_slug BEFORE INSERT ON catalog.products
    FOR EACH ROW EXECUTE FUNCTION catalog.products_default_slug();

-- ---------------------------------------------------------------------------------------
-- SKUs: option_values aligned to the product axes, compare_at_minor (> price) for strike-through.
-- ---------------------------------------------------------------------------------------
ALTER TABLE catalog.skus
    ADD COLUMN option_values text[] NOT NULL DEFAULT '{}' CHECK (cardinality(option_values) <= 3),
    ADD COLUMN compare_at_minor bigint CHECK (compare_at_minor IS NULL OR
        (compare_at_minor > price_minor AND compare_at_minor <= 1000000000000));
-- One active SKU per option combination of a product. Legacy axis-less SKUs ('{}') are exempt.
CREATE UNIQUE INDEX skus_option_combination ON catalog.skus(tenant_id,store_id,product_id,option_values)
    WHERE status='active' AND cardinality(option_values) > 0;
GRANT INSERT(option_values,compare_at_minor), UPDATE(option_values,compare_at_minor) ON catalog.skus TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- Collections (merchant content). Membership has contiguous manual positions; the deferred UNIQUE lets one
-- statement renumber. At most one image per collection (upload replaces it, so the immutable URL id changes).
-- ---------------------------------------------------------------------------------------
CREATE TABLE catalog.collections (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    slug text NOT NULL CHECK (char_length(slug) <= 80 AND slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'
        AND slug !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'),
    title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 80),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
    sort_mode text NOT NULL DEFAULT 'manual' CHECK (sort_mode IN ('manual','newest','price_asc','price_desc')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','hidden')),
    version bigint NOT NULL DEFAULT 1 CHECK (version>0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,id),
    UNIQUE (tenant_id,store_id,slug),
    FOREIGN KEY (tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE TABLE catalog.collection_products (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    collection_id uuid NOT NULL,
    product_id uuid NOT NULL,
    position integer NOT NULL CHECK (position BETWEEN 0 AND 499),
    PRIMARY KEY (tenant_id,store_id,collection_id,product_id),
    CONSTRAINT collection_products_position UNIQUE (tenant_id,store_id,collection_id,position) DEFERRABLE INITIALLY DEFERRED,
    FOREIGN KEY (tenant_id,store_id,collection_id) REFERENCES catalog.collections(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,product_id) REFERENCES catalog.products(tenant_id,store_id,id)
);
CREATE INDEX collection_products_product ON catalog.collection_products(tenant_id,store_id,product_id);
CREATE TABLE catalog.collection_images (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    collection_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    content_type text NOT NULL CHECK (content_type IN ('image/jpeg','image/png','image/webp')),
    bytes bytea NOT NULL CHECK (octet_length(bytes) BETWEEN 1 AND 2097152),
    sha256 bytea NOT NULL CHECK (octet_length(sha256)=32),
    width integer CHECK (width IS NULL OR width BETWEEN 1 AND 100000),
    height integer CHECK (height IS NULL OR height BETWEEN 1 AND 100000),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,id),
    UNIQUE (tenant_id,store_id,collection_id),
    FOREIGN KEY (tenant_id,store_id,collection_id) REFERENCES catalog.collections(tenant_id,store_id,id)
);
REVOKE ALL ON catalog.collections, catalog.collection_products, catalog.collection_images FROM PUBLIC;
DO $$
DECLARE relation_name text;
BEGIN
    FOREACH relation_name IN ARRAY ARRAY['catalog.collections','catalog.collection_products','catalog.collection_images'] LOOP
        EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY',relation_name);
        EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY',relation_name);
        EXECUTE format('CREATE POLICY scope_access ON %s TO commerce_runtime
           USING (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
              AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)
           WITH CHECK (tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
              AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',relation_name);
    END LOOP;
END $$;
GRANT SELECT,INSERT,DELETE ON catalog.collections, catalog.collection_products, catalog.collection_images TO commerce_runtime;
GRANT UPDATE(slug,title,description,sort_mode,status,version,updated_at) ON catalog.collections TO commerce_runtime;
GRANT UPDATE(position) ON catalog.collection_products TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- Definer owner commerce_catalog_media (0082) grows the read access the v2 buyer definers need. Every grant is a
-- SELECT with its own policy (active rows only) so the role can never see drafts, archived rows or other tables.
-- ---------------------------------------------------------------------------------------
GRANT USAGE ON SCHEMA inventory TO commerce_catalog_media;
GRANT SELECT(name,description,slug,seo_title,seo_description,options,created_at) ON catalog.products TO commerce_catalog_media;
GRANT SELECT(tenant_id,store_id,id,product_id,code,status,currency,price_minor,compare_at_minor,option_values,created_at)
    ON catalog.skus TO commerce_catalog_media;
CREATE POLICY catalog_media_sku_read ON catalog.skus FOR SELECT TO commerce_catalog_media USING (status='active');
GRANT SELECT ON catalog.collections, catalog.collection_products, catalog.collection_images TO commerce_catalog_media;
CREATE POLICY catalog_media_collection_read ON catalog.collections FOR SELECT TO commerce_catalog_media USING (status='active');
CREATE POLICY catalog_media_membership_read ON catalog.collection_products FOR SELECT TO commerce_catalog_media USING (true);
CREATE POLICY catalog_media_collection_image_read ON catalog.collection_images FOR SELECT TO commerce_catalog_media USING (true);
GRANT SELECT(tenant_id,store_id,sku_id,on_hand,reserved,allocated,unavailable) ON inventory.balances TO commerce_catalog_media;
CREATE POLICY catalog_media_balance_read ON inventory.balances FOR SELECT TO commerce_catalog_media USING (true);
GRANT SELECT(name,currency) ON control.stores TO commerce_catalog_media;
COMMENT ON ROLE commerce_catalog_media IS
 'catalog definer owner (migrations/0082, extended by 0086). NOLOGIN; owns the catalog.buyer_* definers (product images, v2 product/collection reads, collection image bytes) and reads active products, active SKUs, active collections, product/collection images, stock balances (sum only, never exposed raw) and published stores only through its own FORCE RLS policies. No runtime login may reach it.';

-- ---------------------------------------------------------------------------------------
-- Buyer v2 definers. All: store from the verified origin (buyer.resolve_published_store: PT400 invalid origin, PT404
-- not published / unknown / not visible), STABLE, EXECUTE only for commerce_buyer_runtime. They return jsonb that
-- internal/buyerhttp decodes into closed Go structs (explicit projection, no pass-through).
-- ---------------------------------------------------------------------------------------

-- Product list (contract A: catalog/v2/products). Only active products of the published store that have at least one
-- active SKU in the store currency (a product with no price cannot be listed). p_sort '' = the collection's own
-- sort_mode, else newest. Stock: available = on_hand - reserved - allocated - unavailable summed over warehouses
-- (the ledger's Available, same formula as the Meta feed 0080).
CREATE FUNCTION catalog.buyer_v2_products(p_origin text, p_collection text, p_q text, p_sort text,
    p_min bigint, p_max bigint, p_offset integer, p_limit integer)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_name text; v_cur text; v_coll uuid; v_collsort text; v_sort text; v_q text; v_rows jsonb; v_n integer;
BEGIN
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 48 OR p_offset IS NULL OR p_offset < 0 OR p_offset > 100000
    OR p_sort NOT IN ('','newest','price_asc','price_desc','title') THEN
    RAISE EXCEPTION 'invalid catalog query' USING ERRCODE='PT400';
 END IF;
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id,s.name,s.currency INTO v_tenant,v_name,v_cur FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 IF p_collection <> '' THEN
    SELECT c.id,c.sort_mode INTO v_coll,v_collsort FROM catalog.collections c
     WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.slug=p_collection AND c.status='active';
    IF NOT FOUND THEN RAISE EXCEPTION 'collection not found' USING ERRCODE='PT404'; END IF;
 END IF;
 v_sort := CASE WHEN p_sort <> '' THEN p_sort WHEN v_coll IS NOT NULL THEN v_collsort ELSE 'newest' END;
 -- Literal substring match: '!' is the explicit escape character (no standard_conforming_strings drift).
 v_q := replace(replace(replace(coalesce(p_q,''),'!','!!'),'%','!%'),'_','!_');
 WITH base AS (
   SELECT p.id, p.slug, p.name, p.created_at,
          min(s.price_minor) AS pmin, max(s.price_minor) AS pmax,
          min(s.compare_at_minor) AS cmin,
          coalesce(bool_or(av.avail > 0),false) AS in_stock,
          min(cp.position) AS pos
     FROM catalog.products p
     JOIN catalog.skus s ON s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id
                        AND s.status='active' AND s.currency=v_cur
     LEFT JOIN catalog.collection_products cp ON v_coll IS NOT NULL AND cp.tenant_id=p.tenant_id AND cp.store_id=p.store_id
                        AND cp.collection_id=v_coll AND cp.product_id=p.id
     LEFT JOIN LATERAL (SELECT coalesce(sum(b.on_hand-b.reserved-b.allocated-b.unavailable),0) AS avail
                          FROM inventory.balances b
                         WHERE b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.sku_id=s.id) av ON true
    WHERE p.tenant_id=v_tenant AND p.store_id=v_store AND p.status='active'
      AND (v_coll IS NULL OR cp.product_id IS NOT NULL)
      AND (v_q = '' OR p.name ILIKE '%'||v_q||'%' ESCAPE '!' OR p.description ILIKE '%'||v_q||'%' ESCAPE '!'
           OR EXISTS (SELECT 1 FROM catalog.skus x WHERE x.tenant_id=p.tenant_id AND x.store_id=p.store_id AND x.product_id=p.id
                         AND x.status='active' AND x.code ILIKE '%'||v_q||'%' ESCAPE '!'))
    GROUP BY p.id, p.slug, p.name, p.created_at
   HAVING (p_max IS NULL OR min(s.price_minor) <= p_max) AND (p_min IS NULL OR max(s.price_minor) >= p_min)
 ), ranked AS (
   SELECT b.*, row_number() OVER (
            ORDER BY (CASE WHEN v_sort='manual' THEN b.pos END) ASC,
                     (CASE WHEN v_sort='price_asc' THEN b.pmin END) ASC,
                     (CASE WHEN v_sort='price_desc' THEN b.pmax END) DESC,
                     (CASE WHEN v_sort='title' THEN lower(b.name) END) ASC,
                     b.created_at DESC, b.id) AS rn
     FROM base b
 )
 SELECT coalesce(jsonb_agg(jsonb_build_object('id',g.id,'slug',g.slug,'title',g.name,'price_min_minor',g.pmin,
            'price_max_minor',g.pmax,'compare_at_min_minor',g.cmin,'in_stock',g.in_stock,
            'cover_image_id',(SELECT i.id FROM catalog.product_images i WHERE i.tenant_id=v_tenant AND i.store_id=v_store
                               AND i.product_id=g.id ORDER BY i.position LIMIT 1)) ORDER BY g.rn), '[]'::jsonb),
        count(*)
   INTO v_rows, v_n
   FROM ranked g WHERE g.rn > p_offset AND g.rn <= p_offset + p_limit + 1;
 RETURN jsonb_build_object('store',jsonb_build_object('name',v_name,'currency',v_cur),
    'products', CASE WHEN v_n > p_limit THEN v_rows - p_limit ELSE v_rows END,
    'next_offset', CASE WHEN v_n > p_limit THEN to_jsonb(p_offset + p_limit) ELSE 'null'::jsonb END);
END $$;
ALTER FUNCTION catalog.buyer_v2_products(text,text,text,text,bigint,bigint,integer,integer) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_v2_products(text,text,text,text,bigint,bigint,integer,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_v2_products(text,text,text,text,bigint,bigint,integer,integer) TO commerce_buyer_runtime;

-- Product detail (catalog/v2/products/{slug_or_id}). PT404 identical for unknown / draft / archived / foreign store.
CREATE FUNCTION catalog.buyer_v2_product(p_origin text, p_key text)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_cur text; p record; v_images jsonb; v_variants jsonb; v_colls jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id,s.currency INTO v_tenant,v_cur FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT x.id,x.slug,x.name,x.description,x.seo_title,x.seo_description,x.options INTO p
   FROM catalog.products x
  WHERE x.tenant_id=v_tenant AND x.store_id=v_store AND x.status='active'
    AND CASE WHEN p_key ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN x.id=p_key::uuid ELSE x.slug=p_key END;
 IF NOT FOUND THEN RAISE EXCEPTION 'product not found' USING ERRCODE='PT404'; END IF;
 SELECT coalesce(jsonb_agg(jsonb_build_object('id',i.id,'width',i.width,'height',i.height) ORDER BY i.position),'[]'::jsonb) INTO v_images
   FROM catalog.product_images i WHERE i.tenant_id=v_tenant AND i.store_id=v_store AND i.product_id=p.id;
 SELECT coalesce(jsonb_agg(jsonb_build_object('sku_id',s.id,
            'title', CASE WHEN cardinality(s.option_values)=0 THEN '預設' ELSE array_to_string(s.option_values,' / ') END,
            'option_values',to_jsonb(s.option_values),'price_minor',s.price_minor,'compare_at_minor',s.compare_at_minor,
            'stock', CASE WHEN av.avail >= 6 THEN 'in' WHEN av.avail >= 1 THEN 'low' ELSE 'out' END)
            ORDER BY s.created_at, s.id),'[]'::jsonb) INTO v_variants
   FROM catalog.skus s
   LEFT JOIN LATERAL (SELECT coalesce(sum(b.on_hand-b.reserved-b.allocated-b.unavailable),0) AS avail
                        FROM inventory.balances b WHERE b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.sku_id=s.id) av ON true
  WHERE s.tenant_id=v_tenant AND s.store_id=v_store AND s.product_id=p.id AND s.status='active' AND s.currency=v_cur;
 SELECT coalesce(jsonb_agg(jsonb_build_object('slug',c.slug,'title',c.title) ORDER BY c.created_at, c.id),'[]'::jsonb) INTO v_colls
   FROM catalog.collections c
   JOIN catalog.collection_products cp ON cp.tenant_id=c.tenant_id AND cp.store_id=c.store_id AND cp.collection_id=c.id AND cp.product_id=p.id
  WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.status='active';
 RETURN jsonb_build_object('id',p.id,'slug',p.slug,'title',p.name,'description',p.description,
    'seo',jsonb_build_object('title',p.seo_title,'description',p.seo_description),
    'images',v_images,'options',p.options,'variants',v_variants,'collections',v_colls);
END $$;
ALTER FUNCTION catalog.buyer_v2_product(text,text) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_v2_product(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_v2_product(text,text) TO commerce_buyer_runtime;

-- Active collections with the count of their active products (catalog/v2/collections).
CREATE FUNCTION catalog.buyer_v2_collections(p_origin text)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_out jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 -- ponytail: at most 200 collections per store (enforced by internal/catalog), so no pagination here.
 SELECT jsonb_build_object('collections', coalesce(jsonb_agg(jsonb_build_object('slug',c.slug,'title',c.title,
            'image_id',(SELECT ci.id FROM catalog.collection_images ci WHERE ci.tenant_id=c.tenant_id AND ci.store_id=c.store_id AND ci.collection_id=c.id),
            'product_count',(SELECT count(*) FROM catalog.collection_products cp
                               JOIN catalog.products p ON p.tenant_id=cp.tenant_id AND p.store_id=cp.store_id AND p.id=cp.product_id AND p.status='active'
                              WHERE cp.tenant_id=c.tenant_id AND cp.store_id=c.store_id AND cp.collection_id=c.id)) ORDER BY c.created_at, c.id),'[]'::jsonb))
   INTO v_out FROM catalog.collections c WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.status='active';
 RETURN v_out;
END $$;
ALTER FUNCTION catalog.buyer_v2_collections(text) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_v2_collections(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_v2_collections(text) TO commerce_buyer_runtime;

-- One active collection (catalog/v2/collections/{slug}); PT404 for unknown / hidden / foreign.
CREATE FUNCTION catalog.buyer_v2_collection(p_origin text, p_slug text)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_out jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT jsonb_build_object('slug',c.slug,'title',c.title,'description',c.description,
            'image_id',(SELECT ci.id FROM catalog.collection_images ci WHERE ci.tenant_id=c.tenant_id AND ci.store_id=c.store_id AND ci.collection_id=c.id))
   INTO v_out FROM catalog.collections c
  WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.slug=p_slug AND c.status='active';
 IF v_out IS NULL THEN RAISE EXCEPTION 'collection not found' USING ERRCODE='PT404'; END IF;
 RETURN v_out;
END $$;
ALTER FUNCTION catalog.buyer_v2_collection(text,text) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_v2_collection(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_v2_collection(text,text) TO commerce_buyer_runtime;

-- Collection image bytes for the public /media/c/<collection>/<image> route (same shape and PT codes as
-- catalog.buyer_media_image, 0082): published store, active collection, any miss is PT404.
CREATE FUNCTION catalog.buyer_collection_image(p_origin text, p_collection uuid, p_image uuid)
RETURNS TABLE(content_type text, bytes bytea, sha256 bytea)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 RETURN QUERY
  SELECT i.content_type, i.bytes, i.sha256
    FROM catalog.collection_images i
    JOIN catalog.collections c ON c.tenant_id=i.tenant_id AND c.store_id=i.store_id AND c.id=i.collection_id
    JOIN control.stores s ON s.tenant_id=i.tenant_id AND s.id=i.store_id
   WHERE s.id=v_store AND s.active AND c.id=p_collection AND i.id=p_image AND c.status='active';
 IF NOT FOUND THEN RAISE EXCEPTION 'image not found' USING ERRCODE='PT404'; END IF;
END $$;
ALTER FUNCTION catalog.buyer_collection_image(text,uuid,uuid) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_collection_image(text,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_collection_image(text,uuid,uuid) TO commerce_buyer_runtime;

-- ---------------------------------------------------------------------------------------
-- Documentation (PROCESS section 5).
-- ---------------------------------------------------------------------------------------
COMMENT ON COLUMN catalog.products.slug IS 'internal/catalog: URL handle, unique per store, lowercase alnum words joined by dashes, <= 80, never UUID-shaped. Generated from the title on create, id prefix as fallback.';
COMMENT ON COLUMN catalog.products.seo_title IS 'internal/catalog: optional <head> title override, <= 70 chars; empty string means unset.';
COMMENT ON COLUMN catalog.products.seo_description IS 'internal/catalog: optional meta description, <= 160 chars; empty string means unset.';
COMMENT ON COLUMN catalog.products.options IS 'internal/catalog: 0..3 option axes [{name <= 30, values[1..50] each <= 40}]; validated in Go (OptionAxis); active SKUs must align (same axis count, each value listed).';
COMMENT ON COLUMN catalog.skus.option_values IS 'internal/catalog: one value per product option axis, in axis order; empty for an axis-less product. Unique per product among active SKUs when non-empty.';
COMMENT ON COLUMN catalog.skus.compare_at_minor IS 'internal/catalog: optional strike-through price in minor units, strictly greater than price_minor; display only, never used by quote or checkout (I05: the server price is price_minor).';
COMMENT ON CONSTRAINT products_status_check ON catalog.products IS 'draft (merchant-only), active (buyer-visible when the store is published), archived.';
COMMENT ON FUNCTION catalog.products_default_slug() IS 'catalog-core (0086): BEFORE INSERT default of catalog.products.slug (id prefix). No other effect.';
COMMENT ON TABLE catalog.collections IS 'internal/catalog collections.go: merchant collections (title, description, sort mode, active|hidden). Written only by commerce_runtime under the collection row lock; read by buyers only through catalog.buyer_v2_*. Non-goal: no nesting, no rules-based smart collections.';
COMMENT ON COLUMN catalog.collections.slug IS 'URL handle, unique per store, same shape as products.slug.';
COMMENT ON COLUMN catalog.collections.sort_mode IS 'manual (membership position), newest, price_asc or price_desc; applied by catalog.buyer_v2_products when the buyer gives no explicit sort.';
COMMENT ON COLUMN catalog.collections.status IS 'active (buyer-visible) or hidden.';
COMMENT ON COLUMN catalog.collections.version IS 'Optimistic expected_version of every collection command (edit, membership replace, delete); bumped by edits and membership changes, not by image changes (a replaced image has a new immutable id).';
COMMENT ON TABLE catalog.collection_products IS 'internal/catalog collections.go: manual membership with contiguous positions 0..n-1 (<= 500 per collection); a product may be in many collections. Replaced as a whole by catalog.SetCollectionProducts.';
COMMENT ON TABLE catalog.collection_images IS 'internal/catalog collections.go: at most one image per collection (bytes <= 2 MiB, validated by catalog.SniffImage); upload replaces the row so the public URL /media/c/<collection>/<image> stays immutable-cacheable.';
COMMENT ON POLICY scope_access ON catalog.collections IS 'catalog-core (0086): commerce_runtime only, store scope from the app.tenant_id/app.store_id GUCs.';
COMMENT ON POLICY scope_access ON catalog.collection_products IS 'catalog-core (0086): commerce_runtime only, store scope from the GUCs.';
COMMENT ON POLICY scope_access ON catalog.collection_images IS 'catalog-core (0086): commerce_runtime only, store scope from the GUCs.';
COMMENT ON POLICY catalog_media_sku_read ON catalog.skus IS 'catalog-core (0086): commerce_catalog_media reads active SKUs (column-limited) for the buyer v2 definers. Non-goal: no weight, customs or HS data.';
COMMENT ON POLICY catalog_media_collection_read ON catalog.collections IS 'catalog-core (0086): commerce_catalog_media reads active collections only.';
COMMENT ON POLICY catalog_media_membership_read ON catalog.collection_products IS 'catalog-core (0086): commerce_catalog_media reads membership for the buyer v2 definers, which add the store and collection filters.';
COMMENT ON POLICY catalog_media_collection_image_read ON catalog.collection_images IS 'catalog-core (0086): commerce_catalog_media reads collection images for catalog.buyer_collection_image and the v2 definers.';
COMMENT ON POLICY catalog_media_balance_read ON inventory.balances IS 'catalog-core (0086): commerce_catalog_media reads the five stock columns so the buyer v2 definers can derive in/low/out; raw counts are never returned.';
COMMENT ON FUNCTION catalog.buyer_v2_products(text,text,text,text,bigint,bigint,integer,integer) IS
 'catalog-core owner commerce_catalog_media; only caller internal/buyerhttp catalogv2.go (commerce_buyer_runtime, no buyer token). Product cards of active products with an active SKU in the store currency for the verified origin: collection slug filter, literal q on title/description/SKU code, sort, price overlap filter, offset page of p_limit (1..48). PT400 bad parameters or origin, PT404 not published or unknown collection. No store parameter.';
COMMENT ON FUNCTION catalog.buyer_v2_product(text,text) IS
 'catalog-core owner commerce_catalog_media; only caller internal/buyerhttp catalogv2.go. One active product by slug or id with images, option axes, variants (derived title, price, compare-at, in/low/out) and its active collections. PT404 is identical for unknown, draft, archived and foreign store.';
COMMENT ON FUNCTION catalog.buyer_v2_collections(text) IS
 'catalog-core owner commerce_catalog_media; only caller internal/buyerhttp catalogv2.go. Active collections of the verified origin store with their active product count and image id.';
COMMENT ON FUNCTION catalog.buyer_v2_collection(text,text) IS
 'catalog-core owner commerce_catalog_media; only caller internal/buyerhttp catalogv2.go. One active collection by slug (PT404 for unknown or hidden).';
COMMENT ON FUNCTION catalog.buyer_collection_image(text,uuid,uuid) IS
 'catalog-core owner commerce_catalog_media; only caller internal/buyerhttp catalogv2.go (public /media/c route, no bearer). Bytes of one image of an active collection of the published store; PT400 invalid origin, PT404 any miss.';
