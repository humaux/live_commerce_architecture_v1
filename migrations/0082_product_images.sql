-- 0082 product images (docs/delivery/units/catalog-media.md CM1; output/r3-readiness/REPORT.md gap 3).
--
-- Owns: catalog.product_images (merchant-uploaded product photos stored as bytea, at most 8 per product,
-- JPEG/PNG/WebP, 2 MiB each), the definer owner role commerce_catalog_media and the three buyer-facing read
-- definers catalog.buyer_product_images / catalog.buyer_media_image / catalog.buyer_feed_images.
--
-- Non-goals: no resize, thumbnail, re-encode or CDN (bytes are stored exactly as validated and uploaded); no
-- video; no image on SKUs; no buyer table privilege (the buyer runtime reaches bytes only through the definers
-- below); no merchant access to another store's images (policy scope_access).
--
-- Depends on: 0002 (catalog.products, role commerce_runtime, FORCE RLS pattern), 0007 (role
-- commerce_buyer_runtime, app.tenant_id / app.store_id GUCs set by buyer.WithScope), 0020
-- (buyer.resolve_published_store, the only origin -> store mapping).
--
-- Callers: internal/catalog (images.go; commerce_runtime, merchant routes), internal/storefront ListCatalog
-- (buyer_product_images), internal/buyerhttp mediaGet (buyer_media_image), internal/attribution loadFeed
-- (buyer_feed_images); all buyer-side callers run as commerce_buyer_runtime.
--
-- ponytail: bytea in PG is the pilot ceiling (single host, small catalogs). Move to object storage + CDN when total
-- image bytes pass ~2 GB or a second host serves the storefront; the id changes whenever content changes, so the
-- public URL /media/p/<product>/<image> is already cache-immutable and survives that move.

CREATE ROLE commerce_catalog_media NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
COMMENT ON ROLE commerce_catalog_media IS
 'catalog-media definer owner (migrations/0082). NOLOGIN; owns catalog.buyer_product_images, catalog.buyer_media_image and catalog.buyer_feed_images and reads product_images, active products and published stores only through its own FORCE RLS policies. No runtime login may reach it.';
GRANT USAGE ON SCHEMA catalog, control, buyer TO commerce_catalog_media;

-- ---------------------------------------------------------------------------------------
-- Table. position 0..7 plus a deferred UNIQUE makes "at most 8 per product, contiguous order" structural: a ninth
-- insert has no free position, and a reorder/renumber inside one transaction may pass through duplicates.
-- ---------------------------------------------------------------------------------------
CREATE TABLE catalog.product_images (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    product_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    position smallint NOT NULL CHECK (position BETWEEN 0 AND 7),
    content_type text NOT NULL CHECK (content_type IN ('image/jpeg','image/png','image/webp')),
    bytes bytea NOT NULL CHECK (octet_length(bytes) BETWEEN 1 AND 2097152),
    sha256 bytea NOT NULL CHECK (octet_length(sha256)=32),
    width integer CHECK (width IS NULL OR width BETWEEN 1 AND 100000),
    height integer CHECK (height IS NULL OR height BETWEEN 1 AND 100000),
    version bigint NOT NULL DEFAULT 1 CHECK (version>0),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,product_id) REFERENCES catalog.products(tenant_id,store_id,id),
    CONSTRAINT product_images_position UNIQUE (tenant_id,store_id,product_id,position) DEFERRABLE INITIALLY DEFERRED
);
REVOKE ALL ON catalog.product_images FROM PUBLIC;
ALTER TABLE catalog.product_images ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog.product_images FORCE ROW LEVEL SECURITY;

-- Merchant runtime: store-scoped like catalog.products (0002 scope_access). It reads bytes for the admin preview,
-- inserts a validated upload, deletes one image and renumbers positions; nothing else is updatable.
CREATE POLICY scope_access ON catalog.product_images TO commerce_runtime
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
       AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
    WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
       AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT,INSERT,DELETE ON catalog.product_images TO commerce_runtime;
GRANT UPDATE(position,version) ON catalog.product_images TO commerce_runtime;

-- Definer owner: reads every row, but each definer below filters to one published store / one buyer scope.
CREATE POLICY media_definer_read ON catalog.product_images FOR SELECT TO commerce_catalog_media USING (true);
GRANT SELECT ON catalog.product_images TO commerce_catalog_media;
GRANT SELECT(tenant_id,store_id,id,status) ON catalog.products TO commerce_catalog_media;
CREATE POLICY catalog_media_product_read ON catalog.products FOR SELECT TO commerce_catalog_media USING (status='active');
GRANT SELECT(tenant_id,id,active) ON control.stores TO commerce_catalog_media;
CREATE POLICY catalog_media_store_read ON control.stores FOR SELECT TO commerce_catalog_media USING (active);
GRANT EXECUTE ON FUNCTION buyer.resolve_published_store(text) TO commerce_catalog_media;

-- The storefront home shows the published store name; the buyer already reads its own store row (buyer_read,
-- 0007) and the name is public (it is the Meta feed brand), so the column is granted, nothing else.
GRANT SELECT(name) ON control.stores TO commerce_buyer_runtime;

-- ---------------------------------------------------------------------------------------
-- Buyer definers. REVOKE FROM PUBLIC, EXECUTE only for commerce_buyer_runtime.
-- ---------------------------------------------------------------------------------------

-- Image metadata (never bytes) of active products, in display order, for the products of one catalog page. The
-- store is the buyer scope's own (GUCs set by buyer.WithScope); a caller-supplied store does not exist.
CREATE FUNCTION catalog.buyer_product_images(p_products uuid[])
RETURNS TABLE(product_id uuid, image_id uuid, width integer, height integer, "position" smallint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
  SELECT i.product_id, i.id, i.width, i.height, i.position
    FROM catalog.product_images i
    JOIN catalog.products p ON p.tenant_id=i.tenant_id AND p.store_id=i.store_id AND p.id=i.product_id
   WHERE i.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
     AND i.store_id=nullif(current_setting('app.store_id',true),'')::uuid
     AND p.status='active' AND i.product_id=ANY(p_products)
   ORDER BY i.product_id, i.position
$$;
ALTER FUNCTION catalog.buyer_product_images(uuid[]) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_product_images(uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_product_images(uuid[]) TO commerce_buyer_runtime;

-- One image's bytes for the public /media/p/<product>/<image> route. The store comes from the verified origin
-- (buyer.resolve_published_store, 0020: PT400 invalid origin, PT404 not published) and the product must be
-- active; any miss is PT404 so an unpublished, archived, foreign or unknown image is indistinguishable.
CREATE FUNCTION catalog.buyer_media_image(p_origin text, p_product uuid, p_image uuid)
RETURNS TABLE(content_type text, bytes bytea, sha256 bytea)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 RETURN QUERY
  SELECT i.content_type, i.bytes, i.sha256
    FROM catalog.product_images i
    JOIN catalog.products p ON p.tenant_id=i.tenant_id AND p.store_id=i.store_id AND p.id=i.product_id
    JOIN control.stores s ON s.tenant_id=i.tenant_id AND s.id=i.store_id
   WHERE s.id=v_store AND s.active AND i.product_id=p_product AND i.id=p_image AND p.status='active';
 IF NOT FOUND THEN RAISE EXCEPTION 'image not found' USING ERRCODE='PT404'; END IF;
END $$;
ALTER FUNCTION catalog.buyer_media_image(text,uuid,uuid) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_media_image(text,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_media_image(text,uuid,uuid) TO commerce_buyer_runtime;

-- First image of every active product of the published store, for the Meta feed image_link (meta-ads-v1 §7).
CREATE FUNCTION catalog.buyer_feed_images(p_origin text)
RETURNS TABLE(product_id uuid, image_id uuid)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 -- ponytail: 20000 products per feed, same ceiling as ads.feed_rows.
 RETURN QUERY
  SELECT DISTINCT ON (i.product_id) i.product_id, i.id
    FROM catalog.product_images i
    JOIN catalog.products p ON p.tenant_id=i.tenant_id AND p.store_id=i.store_id AND p.id=i.product_id
    JOIN control.stores s ON s.tenant_id=i.tenant_id AND s.id=i.store_id
   WHERE s.id=v_store AND s.active AND p.status='active'
   ORDER BY i.product_id, i.position LIMIT 20000;
END $$;
ALTER FUNCTION catalog.buyer_feed_images(text) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_feed_images(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_feed_images(text) TO commerce_buyer_runtime;

-- ---------------------------------------------------------------------------------------
-- Documentation (PROCESS section 5).
-- ---------------------------------------------------------------------------------------
COMMENT ON TABLE catalog.product_images IS
 'internal/catalog images.go: merchant product photos, at most 8 per product (position 0..7, deferred UNIQUE), bytes <= 2 MiB, content_type validated by magic bytes in Go. Written only by commerce_runtime through catalog.UploadImage / DeleteImage / ReorderImages under the product row lock; read by buyers only through the catalog.buyer_* definers. Non-goal: no resize, no CDN, no per-SKU image.';
COMMENT ON COLUMN catalog.product_images.tenant_id IS 'Scope (FK catalog.products); server auth only.';
COMMENT ON COLUMN catalog.product_images.store_id IS 'Scope (FK catalog.products); server auth only.';
COMMENT ON COLUMN catalog.product_images.product_id IS 'Owning product (FK); an archived product hides its images from every buyer definer.';
COMMENT ON COLUMN catalog.product_images.id IS 'Random image id; part of the immutable public URL, so a replaced photo is a new row with a new id.';
COMMENT ON COLUMN catalog.product_images.position IS 'Display order 0..7, contiguous; position 0 is the cover (storefront card, Meta feed image_link).';
COMMENT ON COLUMN catalog.product_images.content_type IS 'image/jpeg, image/png or image/webp, derived from magic bytes by internal/catalog.SniffImage, never from the client header.';
COMMENT ON COLUMN catalog.product_images.bytes IS 'Exactly the uploaded file, 1 byte .. 2 MiB; never re-encoded. Selected only by the merchant preview route and catalog.buyer_media_image.';
COMMENT ON COLUMN catalog.product_images.sha256 IS 'SHA-256 of bytes; used as the idempotency fingerprint of the upload command.';
COMMENT ON COLUMN catalog.product_images.width IS 'Pixels from image.DecodeConfig for JPEG/PNG; NULL for WebP (no decoder dependency).';
COMMENT ON COLUMN catalog.product_images.height IS 'Pixels from image.DecodeConfig for JPEG/PNG; NULL for WebP (no decoder dependency).';
COMMENT ON COLUMN catalog.product_images.version IS 'Bumped by each reorder that moves this row.';
COMMENT ON COLUMN catalog.product_images.created_at IS 'Upload time.';
COMMENT ON POLICY scope_access ON catalog.product_images IS
 'catalog-media (migrations/0082): commerce_runtime only, store scope from the app.tenant_id/app.store_id GUCs; removing it makes every merchant image route read zero rows.';
COMMENT ON POLICY media_definer_read ON catalog.product_images IS
 'catalog-media (migrations/0082): commerce_catalog_media SELECT for the catalog.buyer_* definers only, which add their own store/product filters. Non-goal: no merchant or buyer access.';
COMMENT ON POLICY catalog_media_product_read ON catalog.products IS
 'catalog-media (migrations/0082): commerce_catalog_media reads active products only (id, status, scope columns) so an archived product hides its images. Non-goal: no name, description or price.';
COMMENT ON POLICY catalog_media_store_read ON control.stores IS
 'catalog-media (migrations/0082): commerce_catalog_media reads active stores (tenant_id, id, active) to bind a published store id to its tenant. Non-goal: no name, currency or owner data.';
COMMENT ON FUNCTION catalog.buyer_product_images(uuid[]) IS
 'catalog-media owner; only caller internal/storefront.ListCatalog (commerce_buyer_runtime inside buyer.WithScope). Metadata (no bytes) of images of active products in the buyer scope store, ordered by product then position. No store parameter.';
COMMENT ON FUNCTION catalog.buyer_media_image(text,uuid,uuid) IS
 'catalog-media owner; only caller internal/buyerhttp mediaGet (commerce_buyer_runtime, no buyer token). Resolves the store from the verified storefront origin via buyer.resolve_published_store (PT400 invalid origin, PT404 not published or no such image of an active product) and returns content_type, bytes, sha256. No store parameter.';
COMMENT ON FUNCTION catalog.buyer_feed_images(text) IS
 'catalog-media owner; only caller internal/attribution.loadFeed (commerce_buyer_runtime). Resolves the store from the verified origin (PT400/PT404 as ads.feed_rows) and returns the first image id of each active product, for the Meta feed image_link. No bytes.';
