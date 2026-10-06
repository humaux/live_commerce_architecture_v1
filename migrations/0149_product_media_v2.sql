-- Purpose: product-media-v2 (docs/delivery/units/product-media-v2.md; contract contracts/catalog-inventory-v1.md "Amendment —
--   product-media-v2"): 1688-style product media. catalog.product_images gains a role (main <= 4, detail <= 20, sku <= 50) with a per-role
--   position, products gain image_axis, option-value images are linked per value of ONE option axis in catalog.product_option_images,
--   and the buyer / feed definers read the new shape.
-- Depends on: 0082 (product_images, commerce_catalog_media, buyer_* definers), 0086 (options, buyer_v2_*), 0109 (widened position CHECK),
--   0111 (product_image_sizes children, unchanged), buyer.resolve_published_store (0020).
-- Used by: internal/catalog (images.go, image_roles.go), internal/httpapi images.go, internal/storefront catalog.go (buyer_sku_images),
--   internal/buyerhttp catalogv2.go (buyer_v2_product), internal/attribution feed.go (buyer_feed_images, buyer_feed_variant_images).
-- Invariants: nothing is deleted; existing positions 0..3 stay main, positions >= 4 become detail renumbered from 0 (a 9-image product
--   keeps 4 main + 5 detail). Caps are structural (per-role CHECK + deferred UNIQUE), repeated in Go (MaxMainImages, MaxDetailImages,
--   MaxOptionImages). Original bytes stay <= 2 MiB. No new buyer table privilege: buyers reach images only through definers.
-- Status: DESIGN -> REAL_PG (tests/foundation/product_media_v2_test.go).

-- ---------------------------------------------------------------------------------------
-- (1) The image axis: the NAME of the option axis that carries option-value images (NULL = the first axis).
-- commerce_runtime already holds table-level UPDATE on catalog.products (0002); the definer owner reads the column.
-- ---------------------------------------------------------------------------------------
ALTER TABLE catalog.products ADD COLUMN image_axis text CHECK (image_axis IS NULL OR char_length(image_axis) BETWEEN 1 AND 30);
GRANT SELECT(image_axis) ON catalog.products TO commerce_catalog_media;

-- ---------------------------------------------------------------------------------------
-- (2) Roles and per-role positions. The new CHECK is added NOT VALID, the rows are migrated, then it is validated and only then
-- the old 0..11 CHECK and the old (product, position) UNIQUE go away (the old UNIQUE would clash main 0 with detail 0).
-- ---------------------------------------------------------------------------------------
ALTER TABLE catalog.product_images
    ADD COLUMN role text NOT NULL DEFAULT 'main' CHECK (role IN ('main','detail','sku'));
ALTER TABLE catalog.product_images
    ADD CONSTRAINT product_images_role_position UNIQUE (tenant_id,store_id,product_id,role,position) DEFERRABLE INITIALLY DEFERRED,
    ADD CONSTRAINT product_images_role_key UNIQUE (tenant_id,store_id,product_id,id,role);
ALTER TABLE catalog.product_images DROP CONSTRAINT product_images_position;
ALTER TABLE catalog.product_images ADD CONSTRAINT product_images_role_position_range CHECK (
    (role='main' AND position BETWEEN 0 AND 3) OR (role='detail' AND position BETWEEN 0 AND 19) OR (role='sku' AND position BETWEEN 0 AND 49)) NOT VALID;

-- Data move. The table is FORCE RLS and the migration owner has no policy, so FORCE is lifted for this transaction only (same as 0086).
ALTER TABLE catalog.product_images NO FORCE ROW LEVEL SECURITY;
DO $$
DECLARE total_before bigint; tail_before bigint; main_after bigint; detail_after bigint;
BEGIN
 SELECT count(*), count(*) FILTER (WHERE position >= 4) INTO total_before, tail_before FROM catalog.product_images;
 UPDATE catalog.product_images SET role='detail', position=position-4 WHERE role='main' AND position >= 4;
 SELECT count(*) FILTER (WHERE role='main'), count(*) FILTER (WHERE role='detail') INTO main_after, detail_after FROM catalog.product_images;
 IF main_after + detail_after <> total_before OR detail_after <> tail_before OR main_after <> total_before - tail_before THEN
    RAISE EXCEPTION 'product-media-v2 data move changed the image count: before=% (tail %) after main=% detail=%', total_before, tail_before, main_after, detail_after;
 END IF;
END $$;
SET CONSTRAINTS ALL IMMEDIATE; -- fire the deferred UNIQUE checks now: ALTER TABLE refuses a table with pending trigger events
ALTER TABLE catalog.product_images FORCE ROW LEVEL SECURITY;
ALTER TABLE catalog.product_images VALIDATE CONSTRAINT product_images_role_position_range;
ALTER TABLE catalog.product_images DROP CONSTRAINT product_images_position_check;
-- The merchant runtime may move an image between main and detail (role) besides reordering (position, version); bytes stay immutable.
GRANT UPDATE(role) ON catalog.product_images TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- (3) Option-value images: one row per value of the image axis. PK = one image per value, UNIQUE image = one value per image, the
-- composite FK pins same product + same store/tenant + role 'sku' and removes the link with the image.
-- ---------------------------------------------------------------------------------------
CREATE TABLE catalog.product_option_images (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    product_id uuid NOT NULL,
    option_name text NOT NULL CHECK (char_length(option_name) BETWEEN 1 AND 30),
    option_value text NOT NULL CHECK (char_length(option_value) BETWEEN 1 AND 40),
    image_id uuid NOT NULL,
    image_role text NOT NULL DEFAULT 'sku' CHECK (image_role = 'sku'),
    PRIMARY KEY (tenant_id,store_id,product_id,option_name,option_value),
    CONSTRAINT product_option_images_image UNIQUE (tenant_id,store_id,image_id),
    FOREIGN KEY (tenant_id,store_id,product_id) REFERENCES catalog.products(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,product_id,image_id,image_role) REFERENCES catalog.product_images(tenant_id,store_id,product_id,id,role) ON DELETE CASCADE
);
REVOKE ALL ON catalog.product_option_images FROM PUBLIC;
ALTER TABLE catalog.product_option_images ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog.product_option_images FORCE ROW LEVEL SECURITY;
CREATE POLICY scope_access ON catalog.product_option_images TO commerce_runtime
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
    WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT,INSERT,DELETE ON catalog.product_option_images TO commerce_runtime;
CREATE POLICY media_definer_read ON catalog.product_option_images FOR SELECT TO commerce_catalog_media USING (true);
GRANT SELECT ON catalog.product_option_images TO commerce_catalog_media;

-- A link names the EFFECTIVE image axis (products.image_axis when it is a current axis, else the first axis) and a value currently on it.
-- A missing product row is left to the FK (23503). Runs as the inserting role (SECURITY INVOKER); commerce_runtime reads products under RLS.
CREATE FUNCTION catalog.product_option_images_check() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$
DECLARE o jsonb; ax text; found boolean;
BEGIN
 SELECT true, p.options, p.image_axis INTO found, o, ax FROM catalog.products p
  WHERE p.tenant_id=NEW.tenant_id AND p.store_id=NEW.store_id AND p.id=NEW.product_id;
 IF NOT coalesce(found,false) THEN RETURN NEW; END IF;
 ax := coalesce((SELECT a->>'name' FROM jsonb_array_elements(o) a WHERE a->>'name'=ax), o->0->>'name');
 IF ax IS DISTINCT FROM NEW.option_name OR NOT EXISTS (
      SELECT 1 FROM jsonb_array_elements(o) a, jsonb_array_elements_text(a->'values') v WHERE a->>'name'=ax AND v=NEW.option_value) THEN
    RAISE EXCEPTION 'option image: % / % is not a value of the image axis', NEW.option_name, NEW.option_value USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION catalog.product_option_images_check() FROM PUBLIC;
CREATE TRIGGER product_option_images_check BEFORE INSERT ON catalog.product_option_images
    FOR EACH ROW EXECUTE FUNCTION catalog.product_option_images_check();

-- The image of one SKU through its value on the image axis (NULL when none). SECURITY INVOKER on purpose: only definers owned by
-- commerce_catalog_media call it, under that role's own read policies (active SKU), so it opens nothing by itself.
-- ponytail: one indexed lookup per SKU (feed: <= 20000 SKUs); inline it as a join when a feed gets slow.
CREATE FUNCTION catalog.sku_option_image(p_tenant uuid, p_store uuid, p_sku uuid) RETURNS uuid
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
  SELECT oi.image_id
    FROM catalog.skus s
    JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
    CROSS JOIN LATERAL (SELECT coalesce(
        (SELECT a.n::int FROM jsonb_array_elements(p.options) WITH ORDINALITY AS a(x,n) WHERE a.x->>'name'=p.image_axis),
        CASE WHEN jsonb_array_length(p.options) > 0 THEN 1 END) AS idx) ax
    JOIN catalog.product_option_images oi ON oi.tenant_id=s.tenant_id AND oi.store_id=s.store_id AND oi.product_id=s.product_id
         AND oi.option_name=p.options->(ax.idx-1)->>'name' AND oi.option_value=s.option_values[ax.idx]
   WHERE s.tenant_id=p_tenant AND s.store_id=p_store AND s.id=p_sku
$$;
REVOKE ALL ON FUNCTION catalog.sku_option_image(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.sku_option_image(uuid,uuid,uuid) TO commerce_catalog_media;

-- ---------------------------------------------------------------------------------------
-- (4) Buyer definers. Same owner and EXECUTE as 0082/0086; CREATE OR REPLACE keeps both. `images` everywhere means MAIN images.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION catalog.buyer_product_images(p_products uuid[])
RETURNS TABLE(product_id uuid, image_id uuid, width integer, height integer, "position" smallint)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
  SELECT i.product_id, i.id, i.width, i.height, i.position
    FROM catalog.product_images i
    JOIN catalog.products p ON p.tenant_id=i.tenant_id AND p.store_id=i.store_id AND p.id=i.product_id
   WHERE i.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
     AND i.store_id=nullif(current_setting('app.store_id',true),'')::uuid
     AND p.status='active' AND i.role='main' AND i.product_id=ANY(p_products)
   ORDER BY i.product_id, i.position
$$;

-- Per SKU of the buyer scope store: its option-value image, else the cover (main position 0), else NULL (cart/order/claim thumbnails).
CREATE FUNCTION catalog.buyer_sku_images(p_skus uuid[])
RETURNS TABLE(sku_id uuid, image_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
  SELECT s.id, coalesce(catalog.sku_option_image(s.tenant_id,s.store_id,s.id),
           (SELECT i.id FROM catalog.product_images i WHERE i.tenant_id=s.tenant_id AND i.store_id=s.store_id AND i.product_id=s.product_id
               AND i.role='main' ORDER BY i.position LIMIT 1))
    FROM catalog.skus s
    JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
   WHERE s.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
     AND s.store_id=nullif(current_setting('app.store_id',true),'')::uuid
     AND p.status='active' AND s.id=ANY(p_skus)
$$;
ALTER FUNCTION catalog.buyer_sku_images(uuid[]) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_sku_images(uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_sku_images(uuid[]) TO commerce_buyer_runtime;

-- Meta feed: every MAIN image of every active product of the published store in display order (image_link = first,
-- additional_image_link = the next three); detail and sku images never leave through here.
CREATE OR REPLACE FUNCTION catalog.buyer_feed_images(p_origin text)
RETURNS TABLE(product_id uuid, image_id uuid)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 -- ponytail: 20000 products x 4 main images per feed, same product ceiling as ads.feed_rows.
 RETURN QUERY
  SELECT i.product_id, i.id
    FROM catalog.product_images i
    JOIN catalog.products p ON p.tenant_id=i.tenant_id AND p.store_id=i.store_id AND p.id=i.product_id
    JOIN control.stores s ON s.tenant_id=i.tenant_id AND s.id=i.store_id
   WHERE s.id=v_store AND s.active AND p.status='active' AND i.role='main'
   ORDER BY i.product_id, i.position LIMIT 80000;
END $$;

-- Meta feed variant items: the option-value image of every active SKU that has one (feed item id = SKU id).
CREATE FUNCTION catalog.buyer_feed_variant_images(p_origin text)
RETURNS TABLE(sku_id uuid, image_id uuid)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 -- ponytail: 20000 SKUs per feed, same ceiling as ads.feed_rows.
 RETURN QUERY
  SELECT x.id, x.img FROM (
    SELECT k.id, catalog.sku_option_image(k.tenant_id,k.store_id,k.id) AS img
      FROM catalog.skus k
      JOIN catalog.products p ON p.tenant_id=k.tenant_id AND p.store_id=k.store_id AND p.id=k.product_id
      JOIN control.stores s ON s.tenant_id=k.tenant_id AND s.id=k.store_id
     WHERE s.id=v_store AND s.active AND p.status='active' AND k.status='active'
     ORDER BY k.id LIMIT 20000) x
   WHERE x.img IS NOT NULL;
END $$;
ALTER FUNCTION catalog.buyer_feed_variant_images(text) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_feed_variant_images(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_feed_variant_images(text) TO commerce_buyer_runtime;

-- Product list: the cover is the MAIN image at position 0 (a detail image also has position 0).
CREATE OR REPLACE FUNCTION catalog.buyer_v2_products(p_origin text, p_collection text, p_q text, p_sort text,
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
                               AND i.product_id=g.id AND i.role='main' ORDER BY i.position LIMIT 1)) ORDER BY g.rn), '[]'::jsonb),
        count(*)
   INTO v_rows, v_n
   FROM ranked g WHERE g.rn > p_offset AND g.rn <= p_offset + p_limit + 1;
 RETURN jsonb_build_object('store',jsonb_build_object('name',v_name,'currency',v_cur),
    'products', CASE WHEN v_n > p_limit THEN v_rows - p_limit ELSE v_rows END,
    'next_offset', CASE WHEN v_n > p_limit THEN to_jsonb(p_offset + p_limit) ELSE 'null'::jsonb END);
END $$;


-- Product detail: images = main, plus detail_images, image_axis (effective axis name or null), option_images (links on that axis for
-- values still on it) and variants[].image_id. Same PT404 rule as before.
CREATE OR REPLACE FUNCTION catalog.buyer_v2_product(p_origin text, p_key text)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_cur text; p record; v_images jsonb; v_detail jsonb; v_axis text; v_optimg jsonb; v_variants jsonb; v_colls jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id,s.currency INTO v_tenant,v_cur FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT x.id,x.slug,x.name,x.description,x.seo_title,x.seo_description,x.options,x.image_axis INTO p
   FROM catalog.products x
  WHERE x.tenant_id=v_tenant AND x.store_id=v_store AND x.status='active'
    AND CASE WHEN p_key ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN x.id=p_key::uuid ELSE x.slug=p_key END;
 IF NOT FOUND THEN RAISE EXCEPTION 'product not found' USING ERRCODE='PT404'; END IF;
 SELECT coalesce(jsonb_agg(jsonb_build_object('id',i.id,'width',i.width,'height',i.height) ORDER BY i.position) FILTER (WHERE i.role='main'),'[]'::jsonb),
        coalesce(jsonb_agg(jsonb_build_object('id',i.id,'width',i.width,'height',i.height) ORDER BY i.position) FILTER (WHERE i.role='detail'),'[]'::jsonb)
   INTO v_images, v_detail
   FROM catalog.product_images i WHERE i.tenant_id=v_tenant AND i.store_id=v_store AND i.product_id=p.id AND i.role IN ('main','detail');
 v_axis := coalesce((SELECT a->>'name' FROM jsonb_array_elements(p.options) a WHERE a->>'name'=p.image_axis), p.options->0->>'name');
 SELECT coalesce(jsonb_agg(jsonb_build_object('value',oi.option_value,'image_id',oi.image_id) ORDER BY i.position),'[]'::jsonb) INTO v_optimg
   FROM catalog.product_option_images oi
   JOIN catalog.product_images i ON i.tenant_id=oi.tenant_id AND i.store_id=oi.store_id AND i.id=oi.image_id
  WHERE oi.tenant_id=v_tenant AND oi.store_id=v_store AND oi.product_id=p.id AND oi.option_name=v_axis
    AND EXISTS (SELECT 1 FROM jsonb_array_elements(p.options) a, jsonb_array_elements_text(a->'values') v WHERE a->>'name'=v_axis AND v=oi.option_value);
 SELECT coalesce(jsonb_agg(jsonb_build_object('sku_id',s.id,
            'title', CASE WHEN cardinality(s.option_values)=0 THEN '預設' ELSE array_to_string(s.option_values,' / ') END,
            'option_values',to_jsonb(s.option_values),'price_minor',s.price_minor,'compare_at_minor',s.compare_at_minor,
            'image_id',catalog.sku_option_image(v_tenant,v_store,s.id),
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
    'images',v_images,'detail_images',v_detail,'image_axis',v_axis,'option_images',v_optimg,
    'options',p.options,'variants',v_variants,'collections',v_colls);
END $$;

-- ---------------------------------------------------------------------------------------
-- Documentation (PROCESS section 5).
-- ---------------------------------------------------------------------------------------
COMMENT ON TABLE catalog.product_images IS
 'internal/catalog images.go + image_roles.go: merchant product photos by role (main <= 4 at position 0..3 with 0 = cover; detail <= 20 at 0..19; sku <= 50 at 0..49 linked to option values), bytes <= 2 MiB, content_type validated by magic bytes in Go. Caps are the per-role CHECK + deferred UNIQUE (role,position). Written only by commerce_runtime under the product row lock; read by buyers only through the catalog.buyer_* definers.';
COMMENT ON COLUMN catalog.product_images.role IS 'main | detail | sku (product-media-v2). Updatable by commerce_runtime only to move an image between main and detail; a linked sku image keeps role sku (composite FK).';
COMMENT ON COLUMN catalog.product_images.position IS 'Display order INSIDE the role, contiguous from 0: main 0..3 (0 is the cover: storefront card, Meta image_link), detail 0..19, sku 0..49.';
COMMENT ON COLUMN catalog.products.image_axis IS 'product-media-v2: NAME of the option axis that carries option-value images; NULL = the first axis. Effective axis = this when it names a current axis, else the first axis, else none. Written by commerce_runtime (catalog.SetImageAxis).';
COMMENT ON TABLE catalog.product_option_images IS
 'product-media-v2: one row per value of the image axis -> a sku-role image of the same product (PK one image per value, UNIQUE image, composite FK role sku, cascade with the image, trigger: option_name is the effective axis and the value is on it). commerce_runtime SELECT/INSERT/DELETE under scope_access; read by the buyer definers through commerce_catalog_media. Links of a non-effective axis stay stored but dormant.';
COMMENT ON COLUMN catalog.product_option_images.option_name IS 'Axis NAME (<= 30), the effective image axis at link time.';
COMMENT ON COLUMN catalog.product_option_images.option_value IS 'A value of that axis (<= 40); every SKU carrying it resolves to image_id.';
COMMENT ON COLUMN catalog.product_option_images.image_id IS 'The sku-role image of the same product (composite FK); deleted with it.';
COMMENT ON COLUMN catalog.product_option_images.image_role IS 'Constant sku; exists so the composite FK can pin the role.';
COMMENT ON POLICY scope_access ON catalog.product_option_images IS 'product-media-v2 (0149): commerce_runtime only, store scope from the app.tenant_id/app.store_id GUCs.';
COMMENT ON POLICY media_definer_read ON catalog.product_option_images IS 'product-media-v2 (0149): commerce_catalog_media SELECT for the catalog.buyer_* definers and catalog.sku_option_image only.';
COMMENT ON FUNCTION catalog.product_option_images_check() IS 'product-media-v2 trigger (SECURITY INVOKER): refuses a link whose option_name is not the effective image axis or whose value is not on it (23514).';
COMMENT ON FUNCTION catalog.sku_option_image(uuid,uuid,uuid) IS 'product-media-v2: image id linked to the SKU value on the image axis, NULL when none. SECURITY INVOKER, EXECUTE only for commerce_catalog_media (called from its definers).';
COMMENT ON FUNCTION catalog.buyer_sku_images(uuid[]) IS 'product-media-v2 owner commerce_catalog_media; only caller internal/storefront ListCatalog (commerce_buyer_runtime in buyer.WithScope). Per active SKU of the scope store: its option-value image else the cover (main position 0) else NULL; no bytes.';
COMMENT ON FUNCTION catalog.buyer_feed_variant_images(text) IS 'product-media-v2 owner commerce_catalog_media; only caller internal/attribution.loadFeed. Origin-resolved (PT400/PT404 as ads.feed_rows): (sku id, option-value image id) of active SKUs that have one; feed variant items use it as image_link.';
COMMENT ON FUNCTION catalog.buyer_feed_images(text) IS 'catalog-media owner (replaced by 0149); only caller internal/attribution.loadFeed. Origin-resolved: every MAIN image id of each active product in display order (first = image_link, next three = additional_image_link). No bytes, no detail or sku images.';
COMMENT ON FUNCTION catalog.buyer_product_images(uuid[]) IS 'catalog-media owner (replaced by 0149); only caller internal/storefront.ListCatalog. Metadata (no bytes) of the MAIN images of active products in the buyer scope store, ordered by product then position.';
