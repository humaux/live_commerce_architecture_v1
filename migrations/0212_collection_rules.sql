-- Purpose: FEAT-AUTO-COLLECTIONS saved rules and read-time SQL membership (forward only).
-- Depends on: 0086 collection RLS/grants, 0093 collection JSON, 0149 product-media-v2 readers.
-- Used by: internal/catalog and existing buyer catalog-v2 HTTP/BFF routes.
-- Invariants: auth/origin scope and active product/SKU fences remain; no membership snapshots/jobs.

CREATE FUNCTION catalog.valid_collection_rules(r jsonb) RETURNS boolean
LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog AS $$
DECLARE c jsonb;
BEGIN
 IF r IS NULL THEN RETURN true; END IF;
 IF jsonb_typeof(r)<>'object' THEN RETURN false; END IF;
 IF (r - ARRAY['match','conditions']) <> '{}'::jsonb OR NOT (r ?& ARRAY['match','conditions'])
 OR r->>'match' NOT IN ('all','any') OR jsonb_typeof(r->'match')<>'string'
 OR jsonb_typeof(r->'conditions')<>'array' THEN RETURN false; END IF;
 IF jsonb_array_length(r->'conditions') NOT BETWEEN 1 AND 20 THEN RETURN false; END IF;
 FOR c IN SELECT value FROM jsonb_array_elements(r->'conditions') LOOP
  IF jsonb_typeof(c)<>'object' THEN RETURN false; END IF;
  IF (c - ARRAY['field','operator','days']) <> '{}'::jsonb OR NOT (c ?& ARRAY['field','operator','days'])
  OR c->>'field' IS DISTINCT FROM 'created_at' OR c->>'operator' IS DISTINCT FROM 'within_days'
  OR jsonb_typeof(c->'days') IS DISTINCT FROM 'number' THEN RETURN false; END IF;
  IF (c->>'days') !~ '^[0-9]{1,4}$' THEN RETURN false; END IF;
  IF (c->>'days')::integer NOT BETWEEN 1 AND 3650 THEN RETURN false; END IF;
 END LOOP;
 RETURN true;
END $$;
REVOKE ALL ON FUNCTION catalog.valid_collection_rules(jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.valid_collection_rules(jsonb) TO commerce_runtime,commerce_catalog_media;

ALTER TABLE catalog.collections ADD COLUMN membership_mode text NOT NULL DEFAULT 'manual'
 CHECK (membership_mode IN ('manual','automatic'));
ALTER TABLE catalog.collections ADD COLUMN rules jsonb
 CHECK (catalog.valid_collection_rules(rules));
ALTER TABLE catalog.collections ADD CONSTRAINT collection_automatic_rules_required
 CHECK (membership_mode='manual' OR rules IS NOT NULL);
-- Existing FORCE RLS and scope_access/media read policies cover these columns as part of the same table.
-- Existing table SELECT/INSERT grants extend; UPDATE is column-specific, so extend only that grant.
GRANT UPDATE(membership_mode,rules) ON catalog.collections TO commerce_runtime;

CREATE FUNCTION catalog.collection_rule_matches(p_created timestamptz,r jsonb) RETURNS boolean
LANGUAGE sql STABLE STRICT SET search_path=pg_catalog AS $$
 SELECT CASE WHEN r->>'match'='all' THEN bool_and(ok) ELSE bool_or(ok) END
 FROM (SELECT p_created > statement_timestamp()-make_interval(days => (c->>'days')::integer)
 AND p_created <= statement_timestamp() AS ok FROM jsonb_array_elements(r->'conditions') c) matches
$$;
REVOKE ALL ON FUNCTION catalog.collection_rule_matches(timestamptz,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.collection_rule_matches(timestamptz,jsonb) TO commerce_runtime,commerce_catalog_media;
COMMENT ON FUNCTION catalog.valid_collection_rules(jsonb) IS 'internal/catalog rule grammar: all|any, 1..20 created_at within_days conditions, integer days 1..3650; NULL for legacy manual collections.';
COMMENT ON FUNCTION catalog.collection_rule_matches(timestamptz,jsonb) IS 'internal/catalog + buyer catalog readers: pure timestamp rule evaluation at statement time; does not read tables or bypass RLS.';
COMMENT ON COLUMN catalog.collections.membership_mode IS 'manual (legacy default) | automatic. Switching never deletes collection_products; automatic refuses list replacement.';
COMMENT ON COLUMN catalog.collections.rules IS 'Validated rule JSON retained until collection deletion, including while in manual mode. No copied automatic members.';
-- No new index: reads already constrain the products composite PK prefix (tenant_id,store_id).
-- Match supports all/any windows; no justified new independent index without query-plan/scale evidence.

CREATE OR REPLACE FUNCTION catalog.buyer_v2_products(p_origin text, p_collection text, p_q text, p_sort text,
    p_min bigint, p_max bigint, p_offset integer, p_limit integer)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_name text; v_cur text; v_coll uuid; v_collsort text; v_mode text; v_rules jsonb; v_sort text; v_q text; v_rows jsonb; v_n integer;
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
    SELECT c.id,c.sort_mode,c.membership_mode,c.rules INTO v_coll,v_collsort,v_mode,v_rules FROM catalog.collections c
     WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.slug=p_collection AND c.status='active';
    IF NOT FOUND THEN RAISE EXCEPTION 'collection not found' USING ERRCODE='PT404'; END IF;
 END IF;
 v_sort := CASE WHEN p_sort <> '' THEN p_sort WHEN v_mode='automatic' AND v_collsort='manual' THEN 'newest'
 WHEN v_coll IS NOT NULL THEN v_collsort ELSE 'newest' END;
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
     LEFT JOIN catalog.collection_products cp ON v_coll IS NOT NULL AND v_mode='manual' AND cp.tenant_id=p.tenant_id AND cp.store_id=p.store_id
                        AND cp.collection_id=v_coll AND cp.product_id=p.id
     LEFT JOIN LATERAL (SELECT coalesce(sum(b.on_hand-b.reserved-b.allocated-b.unavailable),0) AS avail
                          FROM inventory.balances b
                         WHERE b.tenant_id=s.tenant_id AND b.store_id=s.store_id AND b.sku_id=s.id) av ON true
    WHERE p.tenant_id=v_tenant AND p.store_id=v_store AND p.status='active'
      AND (v_coll IS NULL OR (v_mode='manual' AND cp.product_id IS NOT NULL)
 OR (v_mode='automatic' AND catalog.collection_rule_matches(p.created_at,v_rules)))
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

CREATE OR REPLACE FUNCTION catalog.buyer_v2_product(p_origin text, p_key text)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_cur text; p record; v_images jsonb; v_detail jsonb; v_axis text; v_optimg jsonb; v_variants jsonb; v_colls jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id,s.currency INTO v_tenant,v_cur FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT x.id,x.slug,x.name,x.description,x.seo_title,x.seo_description,x.options,x.image_axis,x.created_at INTO p
   FROM catalog.products x
  WHERE x.tenant_id=v_tenant AND x.store_id=v_store AND x.status='active'
    AND EXISTS (SELECT 1 FROM catalog.skus sk WHERE sk.tenant_id=x.tenant_id AND sk.store_id=x.store_id AND sk.product_id=x.id AND sk.status='active' AND sk.currency=v_cur)
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
  WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.status='active'
 AND ((c.membership_mode='automatic' AND catalog.collection_rule_matches(p.created_at,c.rules))
 OR (c.membership_mode='manual' AND EXISTS (SELECT 1 FROM catalog.collection_products cp WHERE cp.tenant_id=c.tenant_id AND cp.store_id=c.store_id AND cp.collection_id=c.id AND cp.product_id=p.id)));
 RETURN jsonb_build_object('id',p.id,'slug',p.slug,'title',p.name,'description',p.description,
    'seo',jsonb_build_object('title',p.seo_title,'description',p.seo_description),
    'images',v_images,'detail_images',v_detail,'image_axis',v_axis,'option_images',v_optimg,
    'options',p.options,'variants',v_variants,'collections',v_colls);
END $$;

CREATE OR REPLACE FUNCTION catalog.buyer_v2_collections(p_origin text)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
#variable_conflict use_column
DECLARE v_store uuid; v_tenant uuid; v_out jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 -- ponytail: at most 200 collections per store (enforced by internal/catalog), so no pagination here.
 SELECT jsonb_build_object('collections', coalesce(jsonb_agg(jsonb_build_object('id',c.id,'slug',c.slug,'title',c.title,
            'image_id',(SELECT ci.id FROM catalog.collection_images ci WHERE ci.tenant_id=c.tenant_id AND ci.store_id=c.store_id AND ci.collection_id=c.id),
            'product_count',(SELECT count(*) FROM catalog.products p
 WHERE p.tenant_id=c.tenant_id AND p.store_id=c.store_id AND p.status='active'
 AND EXISTS (SELECT 1 FROM catalog.skus s JOIN control.stores st ON st.tenant_id=s.tenant_id AND st.id=s.store_id
 WHERE s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id AND s.status='active' AND s.currency=st.currency)
 AND ((c.membership_mode='automatic' AND catalog.collection_rule_matches(p.created_at,c.rules))
 OR (c.membership_mode='manual' AND EXISTS (SELECT 1 FROM catalog.collection_products cp WHERE cp.tenant_id=c.tenant_id AND cp.store_id=c.store_id AND cp.collection_id=c.id AND cp.product_id=p.id))))) ORDER BY c.created_at, c.id),'[]'::jsonb))
   INTO v_out FROM catalog.collections c WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.status='active';
 RETURN v_out;
END $$;

ALTER FUNCTION catalog.buyer_v2_products(text,text,text,text,bigint,bigint,integer,integer) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_v2_products(text,text,text,text,bigint,bigint,integer,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_v2_products(text,text,text,text,bigint,bigint,integer,integer) TO commerce_buyer_runtime;
ALTER FUNCTION catalog.buyer_v2_product(text,text) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_v2_product(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_v2_product(text,text) TO commerce_buyer_runtime;
ALTER FUNCTION catalog.buyer_v2_collections(text) OWNER TO commerce_catalog_media;
REVOKE ALL ON FUNCTION catalog.buyer_v2_collections(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION catalog.buyer_v2_collections(text) TO commerce_buyer_runtime;
