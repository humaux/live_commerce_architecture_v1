-- Purpose: FEAT-LIVE-REPLAY durable official video references and ENDED-session public projection.
-- Depends on: 0118 fenced live-video completion snapshot, 0122 lifecycle, buyer.resolve_published_store.
-- Used by: internal/live replay commands, metareply completion and internal/buyerhttp replay reads.
-- Invariants: forward-only; no historical Meta read; FORCE RLS; no buyer direct table access or video hosting.
ALTER TABLE live.sessions
 ADD COLUMN replay_public boolean NOT NULL DEFAULT false,
 ADD COLUMN replay_version bigint NOT NULL DEFAULT 1 CHECK(replay_version BETWEEN 1 AND 9007199254740991);
GRANT UPDATE(replay_public,replay_version) ON live.sessions TO commerce_runtime;
COMMENT ON COLUMN live.sessions.replay_public IS '0219: explicit live:manage publication; buyer projection additionally requires ENDED and stored official permalink.';
COMMENT ON COLUMN live.sessions.replay_version IS '0219: independent publication CAS; draft and lifecycle versions remain unchanged.';

CREATE TABLE live.replay_videos (
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,binding_id uuid NOT NULL,
 video_id text NOT NULL CHECK(video_id ~ '^[0-9]{1,40}$'),
 post_id text NOT NULL CHECK(post_id ~ '^[0-9]{1,40}_[0-9]{1,40}$'),
 permalink_url text NOT NULL DEFAULT '' CHECK(length(permalink_url)<=2048 AND
  (permalink_url='' OR permalink_url ~ '^https://(www\.)?facebook\.com/([A-Za-z0-9._-]+/)?videos/[0-9]{1,40}/?$')),
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 requested_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(tenant_id,store_id,binding_id,video_id),
 FOREIGN KEY(tenant_id,store_id,binding_id) REFERENCES integration.bindings(tenant_id,store_id,id));
ALTER TABLE live.replay_videos ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.replay_videos FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.replay_videos FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE ON live.replay_videos TO commerce_integration_writer;
CREATE POLICY replay_video_completion ON live.replay_videos TO commerce_integration_writer USING(true) WITH CHECK(true);
COMMENT ON TABLE live.replay_videos IS '0219: Graph-supplied official permalinks from future lease-fenced live_videos completion only. No backfill, tokens or viewer data. Store-owned metadata retained with its binding; no automatic purge.';

-- Trigger attaches to the existing fenced completion boundary; no duplicate dispatcher or loosened lease check.
CREATE FUNCTION live.persist_replay_videos() RETURNS trigger
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.state='succeeded' THEN
  INSERT INTO live.replay_videos(tenant_id,store_id,binding_id,video_id,post_id,permalink_url,requested_at)
  SELECT NEW.tenant_id,NEW.store_id,NEW.binding_id,x->>'video_id',x->>'post_id',
   CASE WHEN x->>'permalink_url' ~ '^https://(www\.)?facebook\.com/([A-Za-z0-9._-]+/)?videos/[0-9]{1,40}/?$'
    AND length(x->>'permalink_url')<=2048 THEN x->>'permalink_url' ELSE '' END,NEW.requested_at
  FROM jsonb_array_elements(NEW.items) x
  WHERE x->>'video_id' ~ '^[0-9]{1,40}$' AND x->>'post_id' ~ '^[0-9]{1,40}_[0-9]{1,40}$'
  ON CONFLICT(tenant_id,store_id,binding_id,video_id) DO UPDATE SET
   post_id=EXCLUDED.post_id,permalink_url=EXCLUDED.permalink_url,requested_at=EXCLUDED.requested_at,observed_at=clock_timestamp()
  WHERE live.replay_videos.requested_at<=EXCLUDED.requested_at;
 END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION live.persist_replay_videos() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION live.persist_replay_videos() FROM PUBLIC;
CREATE TRIGGER persist_replay_videos AFTER INSERT OR UPDATE ON live.page_live_video_snapshots
 FOR EACH ROW EXECUTE FUNCTION live.persist_replay_videos();
COMMENT ON FUNCTION live.persist_replay_videos() IS '0219: only future successful snapshot writes persist references, in the same transaction after existing operation/lease/permission fences.';

CREATE ROLE commerce_replay_reader NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
GRANT USAGE ON SCHEMA live,buyer,control,catalog TO commerce_replay_reader;
GRANT SELECT ON live.replay_videos TO commerce_replay_reader;
GRANT SELECT(tenant_id,store_id,id,title,lifecycle,lifecycle_at,replay_public) ON live.sessions TO commerce_replay_reader;
GRANT SELECT(tenant_id,store_id,session_id,platform,binding_id,source_object_id,active) ON live.claim_sources TO commerce_replay_reader;
GRANT SELECT(tenant_id,store_id,id,session_id,sku_id,created_at) ON live.offers TO commerce_replay_reader;
GRANT SELECT(tenant_id,store_id,id,product_id) ON catalog.skus TO commerce_replay_reader;
GRANT SELECT(tenant_id,id,active) ON control.stores TO commerce_replay_reader;
GRANT EXECUTE ON FUNCTION buyer.resolve_published_store(text) TO commerce_replay_reader;
CREATE POLICY replay_video_projection ON live.replay_videos FOR SELECT TO commerce_replay_reader USING(true);
CREATE POLICY replay_session_projection ON live.sessions FOR SELECT TO commerce_replay_reader USING(true);
CREATE POLICY replay_source_projection ON live.claim_sources FOR SELECT TO commerce_replay_reader USING(true);
CREATE POLICY replay_offer_projection ON live.offers FOR SELECT TO commerce_replay_reader USING(true);
CREATE POLICY replay_sku_projection ON catalog.skus FOR SELECT TO commerce_replay_reader USING(true);
CREATE POLICY replay_store_projection ON control.stores FOR SELECT TO commerce_replay_reader USING(true);
COMMENT ON ROLE commerce_replay_reader IS '0219: no-login projection owner; only origin-scoped public replay and GUC-scoped merchant permalink definers are executable.';

CREATE FUNCTION live.replay_permalink(p_session uuid) RETURNS text
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT coalesce((SELECT v.permalink_url FROM live.claim_sources c JOIN live.replay_videos v
  ON (v.tenant_id,v.store_id,v.binding_id,v.post_id)=(c.tenant_id,c.store_id,c.binding_id,c.source_object_id)
  WHERE c.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND c.store_id=nullif(current_setting('app.store_id',true),'')::uuid
   AND c.session_id=p_session AND c.active AND c.platform='facebook' AND v.permalink_url<>''
  ORDER BY v.observed_at DESC,v.video_id LIMIT 1),'');
$$;
ALTER FUNCTION live.replay_permalink(uuid) OWNER TO commerce_replay_reader;
REVOKE ALL ON FUNCTION live.replay_permalink(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.replay_permalink(uuid) TO commerce_runtime;
COMMENT ON FUNCTION live.replay_permalink(uuid) IS '0219: GUC-scoped merchant source lookup only; live:manage is rechecked in caller before publication.';

-- The projection supplies only product identifiers; the HTTP layer calls the existing buyer catalog
-- projection in the same read transaction for CURRENT price/stock/publication and its closed DTO.
CREATE FUNCTION live.buyer_replays(p_origin text,p_session uuid) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_store uuid;v_tenant uuid;v_items jsonb;v_detail jsonb;
BEGIN
 SELECT r.store_id INTO v_store FROM buyer.resolve_published_store(p_origin) r;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=v_store AND s.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'storefront not published' USING ERRCODE='PT404'; END IF;
 SELECT coalesce(jsonb_agg(x.item ORDER BY x.ended_at DESC,x.id),'[]'::jsonb) INTO v_items FROM (
  SELECT s.id,s.lifecycle_at AS ended_at,jsonb_build_object('session_id',s.id,'title',s.title,
   'ended_at',s.lifecycle_at,'permalink_url',v.permalink_url) AS item
  FROM live.sessions s JOIN LATERAL (
   SELECT rv.permalink_url FROM live.claim_sources c JOIN live.replay_videos rv
    ON (rv.tenant_id,rv.store_id,rv.binding_id,rv.post_id)=(c.tenant_id,c.store_id,c.binding_id,c.source_object_id)
   WHERE c.tenant_id=s.tenant_id AND c.store_id=s.store_id AND c.session_id=s.id AND c.active
    AND c.platform='facebook' AND rv.permalink_url<>'' ORDER BY rv.observed_at DESC,rv.video_id LIMIT 1
  ) v ON true WHERE s.tenant_id=v_tenant AND s.store_id=v_store AND s.replay_public
   AND s.lifecycle='ended' AND s.lifecycle_at IS NOT NULL AND (p_session IS NULL OR s.id=p_session)
  ORDER BY s.lifecycle_at DESC,s.id LIMIT 50
 ) x;
 IF p_session IS NULL THEN RETURN jsonb_build_object('items',v_items); END IF;
 IF jsonb_array_length(v_items)=0 THEN RAISE EXCEPTION 'replay not found' USING ERRCODE='PT404'; END IF;
 v_detail:=v_items->0;
 RETURN v_detail || jsonb_build_object('offers',coalesce((SELECT jsonb_agg(jsonb_build_object(
  'offer_id',o.id,'sku_id',o.sku_id,'product_id',k.product_id) ORDER BY o.created_at,o.id)
  FROM live.offers o JOIN catalog.skus k ON (k.tenant_id,k.store_id,k.id)=(o.tenant_id,o.store_id,o.sku_id)
  WHERE o.tenant_id=v_tenant AND o.store_id=v_store AND o.session_id=p_session),'[]'::jsonb));
END $$;
ALTER FUNCTION live.buyer_replays(text,uuid) OWNER TO commerce_replay_reader;
REVOKE ALL ON FUNCTION live.buyer_replays(text,uuid) FROM PUBLIC;
GRANT USAGE ON SCHEMA live TO commerce_buyer_runtime;
GRANT EXECUTE ON FUNCTION live.buyer_replays(text,uuid) TO commerce_buyer_runtime;
COMMENT ON FUNCTION live.buyer_replays(text,uuid) IS '0219: published-origin-only projection, at most 50 ENDED public sessions; no direct buyer table grants, secrets or historical money; missing/unpublished/foreign detail is PT404.';

-- Missing/unpublished catalog products are omitted without aborting the enclosing replay transaction.
CREATE FUNCTION live.buyer_replay_product(p_origin text,p_product text) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 RETURN catalog.buyer_v2_product(p_origin,p_product);
EXCEPTION WHEN SQLSTATE 'PT404' THEN RETURN NULL;
END $$;
GRANT EXECUTE ON FUNCTION catalog.buyer_v2_product(text,text) TO commerce_replay_reader;
ALTER FUNCTION live.buyer_replay_product(text,text) OWNER TO commerce_replay_reader;
REVOKE ALL ON FUNCTION live.buyer_replay_product(text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.buyer_replay_product(text,text) TO commerce_buyer_runtime;
COMMENT ON FUNCTION live.buyer_replay_product(text,text) IS '0219: same catalog read and published-origin authority, converts catalog PT404 to absent product; no money/stock copy.';
