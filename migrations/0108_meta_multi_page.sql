-- 0108 meta-multi-page (docs/delivery/units/meta-multi-page.md, contracts/meta-claims-intake-v1.md "Merchant connect (R4)"):
-- one store connects up to 10 Facebook Pages instead of exactly one. Forward-only: 0095 and 0100 are never edited.
--
-- Decision 1: connections are per (store, page_id). The PRIMARY KEY of integration.meta_connections is re-keyed from
--   (tenant_id,store_id) to (tenant_id,store_id,page_id); page_id keeps its platform-wide UNIQUE (one Page never feeds two
--   stores) and ig_id its UNIQUE. No column changes and no data moves: every existing row already carries its page_id, so the
--   re-key backfills the existing (single) row by simply re-indexing it under the new key -- nothing is dropped or rewritten.
-- Decision 2: connect adds or refreshes a Page, never replaces another. The D1 "one store - one Page" refusal (already_connected,
--   MC409) is replaced by a cap of 10 checked under the per-store advisory lock in meta_connect_finish (0100 D1's lock, kept) and
--   by a soft check in meta_connect_prepare for a precise refusal before any Meta call. Disconnect is per Page and enqueues the
--   D2 unsubscribe job for that Page only (kept; the job table was already keyed by page_id).
-- Decision 3: claim sources bind one Page post/live; intake routing stays keyed by page_id (meta_inbox.routes, live.claim_sources),
--   so nothing here changes -- the Go resolveBinding honours ref.Page (internal/claims/claim_source.go).
-- Decision 4: Page-token custody is unchanged (HPKE meta-page-token-v2, only claims-worker opens; one sealed token per Page).
-- Decision 5: the API (meta_connect_status) now returns the list of connected Pages (id, name, IG, status, permissions,
--   last_event_at) plus the count and cap so the Settings card can show "N / 10".
-- Owning package: internal/metaconnect (SQL + Go service); callers unchanged (commerce_runtime, commerce_claims_worker for the job).

-- ---------------------------------------------------------------------------------------
-- D1: re-key meta_connections (PK (tenant_id,store_id) -> (tenant_id,store_id,page_id)).
-- ---------------------------------------------------------------------------------------
ALTER TABLE integration.meta_connections DROP CONSTRAINT meta_connections_pkey;
ALTER TABLE integration.meta_connections ADD PRIMARY KEY (tenant_id,store_id,page_id);

COMMENT ON TABLE integration.meta_connections IS
 'internal/metaconnect: the store''s merchant-connected Facebook Pages (and their optional Instagram accounts), up to 10 per store (0108). ids, display names, granted permissions, status active|reauth_required, route expiry; one row per (store, page_id), page_id UNIQUE platform-wide. Written only by integration.meta_connect_finish/disconnect/mark_reauth. Non-goal: no token or secret column.';

-- ---------------------------------------------------------------------------------------
-- D2: meta_connect_prepare -- replace already_connected with a soft cap check (precise refusal before any Meta call).
-- The authoritative cap is meta_connect_finish under the store advisory lock.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.meta_connect_prepare(p_hash bytea,p_store uuid,p_state uuid,p_page text,p_with_ig boolean) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states; e jsonb; v_page jsonb; v_n bigint;
BEGIN
 IF p_page IS NULL OR p_page !~ '^[0-9]{1,40}$' OR p_with_ig IS NULL THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 SELECT * INTO s FROM integration.meta_connect_states x WHERE x.id=p_state AND x.tenant_id=a.out_tenant AND x.store_id=p_store
  AND x.principal_id=a.out_principal FOR UPDATE;
 IF NOT FOUND OR s.used_at IS NULL OR s.pages IS NULL THEN RAISE EXCEPTION 'state_mismatch' USING ERRCODE='MC409'; END IF;
 IF s.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'state_expired' USING ERRCODE='MC410'; END IF;
 IF s.done_at IS NOT NULL THEN RAISE EXCEPTION 'state_used' USING ERRCODE='MC409'; END IF;
 FOR e IN SELECT * FROM jsonb_array_elements(s.pages) LOOP
  IF e->>'page_id'=p_page THEN v_page:=e; END IF;
 END LOOP;
 IF v_page IS NULL THEN RAISE EXCEPTION 'not_in_pick_list' USING ERRCODE='MC422'; END IF;
 -- Never partial-enable: a missing permission or task refuses the whole pick (the card lists what to re-grant).
 IF jsonb_array_length(v_page->'missing')>0 THEN RAISE EXCEPTION 'missing_permission' USING ERRCODE='MC422'; END IF;
 IF p_with_ig AND (v_page->>'ig_id' IS NULL OR jsonb_array_length(v_page->'ig_missing')>0) THEN
  RAISE EXCEPTION 'missing_permission' USING ERRCODE='MC422'; END IF;
 -- D2 multi-page: a store may hold up to 10 Pages. Reconnecting an already-connected Page is a refresh and never refused;
 -- connecting a Page beyond the cap is refused before any Meta call.
 SELECT count(*) INTO v_n FROM integration.meta_connections c WHERE c.tenant_id=a.out_tenant AND c.store_id=p_store AND c.page_id<>p_page;
 IF v_n>=10 THEN RAISE EXCEPTION 'cap_exceeded' USING ERRCODE='MC409'; END IF;
 -- One Page / IG account belongs to one store platform-wide; the refusal never names the other store.
 IF NOT meta_inbox.connect_owner_ok('page',p_page,a.out_tenant,p_store)
  OR (p_with_ig AND NOT meta_inbox.connect_owner_ok('instagram',v_page->>'ig_id',a.out_tenant,p_store)) THEN
  RAISE EXCEPTION 'page_taken' USING ERRCODE='MC409'; END IF;
 RETURN jsonb_build_object('page',v_page,'scopes',to_jsonb(s.scopes_attested));
END $$;

COMMENT ON FUNCTION integration.meta_connect_prepare(bytea,uuid,uuid,text,boolean) IS
 'integration owner; only caller internal/metaconnect.Service.Pick step 1. Validates the pick (state, pickable Page/IG, one-store-per-Page via meta_inbox.connect_owner_ok, the 10-Page cap as a soft check) BEFORE any Meta call and returns the pick row; refusals missing_permission / page_taken / cap_exceeded (0108 multi-page).';

-- ---------------------------------------------------------------------------------------
-- D2: meta_connect_finish -- cap of 10 under the store advisory lock (kept from 0100 D1);
-- the write targets the new PK (tenant_id,store_id,page_id), so a different Page inserts and the
-- SAME Page refreshes (never repoints another Page). Identical otherwise to 0100.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.meta_connect_finish(p_hash bytea,p_store uuid,p_state uuid,p_page text,
 p_fb_binding uuid,p_fb_expected bigint,p_fb_key text,p_fb_nonce bytea,p_fb_ct bytea,
 p_ig_binding uuid,p_ig_expected bigint,p_ig_key text,p_ig_nonce bytea,p_ig_ct bytea,
 p_app_page text,p_app_ig text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states; e jsonb; v_page jsonb; b record; v_exp timestamptz; v_n bigint;
 v_fb text[]:=ARRAY['pages_show_list','pages_manage_metadata','pages_read_engagement','pages_messaging'];
 v_ig text[]:=ARRAY['instagram_basic','instagram_manage_comments','instagram_manage_messages'];
BEGIN
 IF p_page IS NULL OR p_page !~ '^[0-9]{1,40}$' OR p_fb_binding IS NULL OR p_fb_expected IS NULL OR p_fb_expected<0 OR p_fb_expected>=1<<62
  OR p_fb_key IS NULL OR p_fb_key !~ '^[A-Za-z0-9_-]{1,64}$' OR p_fb_nonce IS NULL OR octet_length(p_fb_nonce)<>32
  OR p_fb_ct IS NULL OR octet_length(p_fb_ct) NOT BETWEEN 17 AND 8192 OR p_app_page IS NULL OR p_app_page !~ '^[0-9]{1,40}$'
  OR ((p_ig_binding IS NULL) IS DISTINCT FROM (p_ig_expected IS NULL)) OR ((p_ig_binding IS NULL) IS DISTINCT FROM (p_ig_key IS NULL))
  OR ((p_ig_binding IS NULL) IS DISTINCT FROM (p_ig_nonce IS NULL)) OR ((p_ig_binding IS NULL) IS DISTINCT FROM (p_ig_ct IS NULL))
  OR (p_ig_binding IS NOT NULL AND (p_ig_expected<0 OR p_ig_expected>=1<<62 OR p_ig_key !~ '^[A-Za-z0-9_-]{1,64}$'
   OR octet_length(p_ig_nonce)<>32 OR octet_length(p_ig_ct) NOT BETWEEN 17 AND 8192 OR p_app_ig IS NULL OR p_app_ig !~ '^[0-9]{1,40}$')) THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 -- D1 (0100): serialize every finish of one store BEFORE any binding/connection read. With no connection row yet, FOR UPDATE
 -- below locks nothing and two concurrent picks of different Pages would both commit past the cap check; the lock serializes them.
 PERFORM pg_advisory_xact_lock(hashtextextended(jsonb_build_array('meta-connect-store',a.out_tenant,p_store)::text,0));
 SELECT * INTO s FROM integration.meta_connect_states x WHERE x.id=p_state AND x.tenant_id=a.out_tenant AND x.store_id=p_store
  AND x.principal_id=a.out_principal FOR UPDATE;
 IF NOT FOUND OR s.used_at IS NULL OR s.pages IS NULL THEN RAISE EXCEPTION 'state_mismatch' USING ERRCODE='MC409'; END IF;
 IF s.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'state_expired' USING ERRCODE='MC410'; END IF;
 IF s.done_at IS NOT NULL THEN RAISE EXCEPTION 'state_used' USING ERRCODE='MC409'; END IF;
 FOR e IN SELECT * FROM jsonb_array_elements(s.pages) LOOP
  IF e->>'page_id'=p_page THEN v_page:=e; END IF;
 END LOOP;
 IF v_page IS NULL THEN RAISE EXCEPTION 'not_in_pick_list' USING ERRCODE='MC422'; END IF;
 -- Re-check the attested permissions here as well (defence in depth; Go computed the same lists at the callback).
 IF jsonb_array_length(v_page->'missing')>0 OR NOT (v_fb <@ s.scopes_attested)
  OR (p_ig_binding IS NOT NULL AND (v_page->>'ig_id' IS NULL OR jsonb_array_length(v_page->'ig_missing')>0 OR NOT (v_ig <@ s.scopes_attested))) THEN
  RAISE EXCEPTION 'missing_permission' USING ERRCODE='MC422'; END IF;
 -- D2 multi-page: the cap of 10 under the store lock. Reconnecting an already-connected Page does not add a row, so it is always
 -- allowed; a different Page past the cap is refused (cap_exceeded, MC409).
 SELECT count(*) INTO v_n FROM integration.meta_connections c WHERE c.tenant_id=a.out_tenant AND c.store_id=p_store;
 IF v_n>=10 AND NOT EXISTS(SELECT 1 FROM integration.meta_connections c WHERE c.tenant_id=a.out_tenant AND c.store_id=p_store AND c.page_id=p_page) THEN
  RAISE EXCEPTION 'cap_exceeded' USING ERRCODE='MC409'; END IF;
 -- Bindings: this store's, enabled, for exactly this Page / IG asset (the caller created or re-enabled them in this transaction).
 SELECT x.id INTO b FROM integration.bindings x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.id=p_fb_binding
  AND x.provider='facebook' AND x.external_asset_id=p_page AND x.enabled FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'binding_disabled' USING ERRCODE='MC409'; END IF;
 IF p_ig_binding IS NOT NULL THEN
  SELECT x.id INTO b FROM integration.bindings x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.id=p_ig_binding
   AND x.provider='instagram' AND x.external_asset_id=v_page->>'ig_id' AND x.enabled FOR SHARE;
  IF NOT FOUND THEN RAISE EXCEPTION 'binding_disabled' USING ERRCODE='MC409'; END IF;
 END IF;
 -- Credentials with the 0064 head CAS (expected 0 creates the head; otherwise it must equal the current head).
 PERFORM integration.meta_connect_put_credential(a.out_tenant,p_store,a.out_principal,p_fb_binding,'facebook',p_page,
  p_fb_expected,p_fb_key,p_fb_nonce,p_fb_ct,s.scopes_attested);
 IF p_ig_binding IS NOT NULL THEN
  PERFORM integration.meta_connect_put_credential(a.out_tenant,p_store,a.out_principal,p_ig_binding,'instagram',v_page->>'ig_id',
   p_ig_expected,p_ig_key,p_ig_nonce,p_ig_ct,s.scopes_attested);
 END IF;
 v_exp:=clock_timestamp()+interval '365 days';
 BEGIN
  PERFORM meta_inbox.connect_activate(p_app_page,'page',p_page,a.out_tenant,p_store,p_fb_binding,v_exp);
  IF p_ig_binding IS NOT NULL THEN
   PERFORM meta_inbox.connect_activate(p_app_ig,'instagram',v_page->>'ig_id',a.out_tenant,p_store,p_ig_binding,v_exp);
  END IF;
 EXCEPTION WHEN SQLSTATE 'PT409' THEN
  RAISE EXCEPTION 'page_taken' USING ERRCODE='MC409';
 END;
 -- D2 (0100): a pending unsubscribe of THIS Page (a quick disconnect -> reconnect) must not unsubscribe the Page being connected again.
 UPDATE integration.meta_unsubscribe_jobs j SET state='SUPERSEDED',key_id=NULL,nonce=NULL,ciphertext=NULL,code='reconnected',updated_at=clock_timestamp()
  WHERE j.tenant_id=a.out_tenant AND j.store_id=p_store AND j.page_id=p_page AND j.state='PENDING';
 -- The row is now (tenant_id,store_id,page_id)-unique: a different Page inserts a new row, the SAME Page refreshes in place.
 INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,ig_binding,ig_id,ig_username,scopes,status,connected_by,route_expires_at)
  VALUES(a.out_tenant,p_store,p_page,coalesce(v_page->>'name',''),p_fb_binding,p_ig_binding,
   CASE WHEN p_ig_binding IS NULL THEN NULL ELSE v_page->>'ig_id' END,
   CASE WHEN p_ig_binding IS NULL THEN NULL ELSE coalesce(v_page->>'ig_username','') END,
   s.scopes_attested,'active',a.out_principal,v_exp)
  ON CONFLICT (tenant_id,store_id,page_id) DO UPDATE SET page_name=EXCLUDED.page_name,fb_binding=EXCLUDED.fb_binding,ig_binding=EXCLUDED.ig_binding,
   ig_id=EXCLUDED.ig_id,ig_username=EXCLUDED.ig_username,scopes=EXCLUDED.scopes,status='active',connected_by=EXCLUDED.connected_by,
   connected_at=clock_timestamp(),updated_at=clock_timestamp(),route_expires_at=EXCLUDED.route_expires_at;
 UPDATE integration.meta_connect_states x SET done_at=clock_timestamp() WHERE x.id=s.id;
 RETURN jsonb_build_object('page_id',p_page,'instagram',p_ig_binding IS NOT NULL);
END $$;

COMMENT ON FUNCTION integration.meta_connect_finish(bytea,uuid,uuid,text,uuid,bigint,text,bytea,bytea,uuid,bigint,text,bytea,bytea,text,text) IS
 'integration owner; only caller internal/metaconnect.Service.Pick step 2. One transaction under the per-store advisory lock (0100 D1): sealed Page credential(s) under the 0064 head CAS, merchant routes via meta_inbox.connect_activate, the connection row (insert a new Page / refresh the SAME Page, 10-Page cap, never repoint another Page), state done, a pending unsubscribe of the same Page superseded. Receives only sealed (HPKE v2) bytes, never plaintext.';

-- ---------------------------------------------------------------------------------------
-- D5: meta_connect_status -- the card now lists every connected Page (count + cap for "N / 10").
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.meta_connect_status(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; c record; v_pages jsonb:='[]'::jsonb; v_count bigint:=0; v_last timestamptz; v_ig timestamptz;
BEGIN
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:read');
 FOR c IN SELECT * FROM integration.meta_connections x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store
  ORDER BY x.connected_at,x.page_id LOOP
  v_last:=meta_inbox.connect_last_event(a.out_tenant,p_store,'page',c.page_id);
  v_ig:=NULL;
  IF c.ig_id IS NOT NULL THEN v_ig:=meta_inbox.connect_last_event(a.out_tenant,p_store,'instagram',c.ig_id); END IF;
  v_pages:=v_pages||jsonb_build_object('id',c.page_id,'name',c.page_name,'status',c.status,
   'instagram',CASE WHEN c.ig_id IS NULL THEN NULL ELSE jsonb_build_object('id',c.ig_id,'username',coalesce(c.ig_username,'')) END,
   'permissions',to_jsonb(c.scopes),
   'connected_at',to_char(c.connected_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'route_expires_at',to_char(c.route_expires_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
   'last_event_at',CASE WHEN greatest(v_last,v_ig) IS NULL THEN NULL
    ELSE to_char(greatest(v_last,v_ig) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') END);
  v_count:=v_count+1;
 END LOOP;
 RETURN jsonb_build_object('connected',v_count>0,'count',v_count,'cap',10,'pages',v_pages);
END $$;

COMMENT ON FUNCTION integration.meta_connect_status(bytea,uuid) IS
 'integration owner; only caller GET meta-connect/status (commerce_runtime, integration:read). The card: every connected Page (id, name, IG, permissions, status, route expiry, last routed webhook time) plus count and cap (10); no token, no ciphertext.';

-- ---------------------------------------------------------------------------------------
-- D2: disconnect is per Page. Drop the single-Page definer and recreate with a page_id argument:
-- one transaction destroys THAT Page's sealed credentials, disables its routes, deletes its row, and
-- enqueues ONE D2 unsubscribe job for it (the job table is keyed by page_id).
-- ---------------------------------------------------------------------------------------
DROP FUNCTION integration.meta_connect_disconnect(bytea,uuid);

CREATE FUNCTION integration.meta_connect_disconnect(p_hash bytea,p_store uuid,p_page text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; c integration.meta_connections;
BEGIN
 IF p_page IS NULL OR p_page !~ '^[0-9]{1,40}$' THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 -- D2 (0100, fixed 0108): serialize with meta_connect_finish on the SAME per-store advisory lock. Without it, a disconnect
 -- racing a reconnect of the same Page could enqueue its unsubscribe job AFTER finish already superseded the pending jobs but
 -- BEFORE finish's INSERT: the worker would then DELETE /{page}/subscribed_apps for a Page the DB shows as connected.
 PERFORM pg_advisory_xact_lock(hashtextextended(jsonb_build_array('meta-connect-store',a.out_tenant,p_store)::text,0));
 SELECT * INTO c FROM integration.meta_connections x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.page_id=p_page FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='MC404'; END IF;
 -- D2 (0100): hand this Page's head sealed token to the claims-worker (the only process that can open it) before it is destroyed here.
 INSERT INTO integration.meta_unsubscribe_jobs(tenant_id,store_id,principal_id,binding_id,page_id,version,key_id,nonce,ciphertext)
  SELECT k.tenant_id,k.store_id,a.out_principal,k.binding_id,c.page_id,k.version,k.key_id,k.nonce,k.ciphertext
  FROM integration.meta_page_heads h JOIN integration.meta_page_credentials k
   ON k.tenant_id=h.tenant_id AND k.store_id=h.store_id AND k.binding_id=h.binding_id AND k.version=h.current_version
  WHERE h.tenant_id=a.out_tenant AND h.store_id=p_store AND h.binding_id=c.fb_binding;
 -- Head first, then the versions (the 0064 head FK is deferred anyway): no credential of these bindings survives.
 DELETE FROM integration.meta_page_heads h WHERE h.tenant_id=a.out_tenant AND h.store_id=p_store AND h.binding_id IN (c.fb_binding,c.ig_binding);
 DELETE FROM integration.meta_page_credentials k WHERE k.tenant_id=a.out_tenant AND k.store_id=p_store AND k.binding_id IN (c.fb_binding,c.ig_binding);
 PERFORM meta_inbox.connect_disable(a.out_tenant,p_store,'page',c.page_id);
 IF c.ig_id IS NOT NULL THEN PERFORM meta_inbox.connect_disable(a.out_tenant,p_store,'instagram',c.ig_id); END IF;
 DELETE FROM integration.meta_connections x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.page_id=p_page;
 RETURN jsonb_build_object('page_id',c.page_id,'fb_binding',c.fb_binding,'ig_binding',c.ig_binding);
END $$;

ALTER FUNCTION integration.meta_connect_disconnect(bytea,uuid,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.meta_connect_disconnect(bytea,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.meta_connect_disconnect(bytea,uuid,text) TO commerce_runtime;

COMMENT ON FUNCTION integration.meta_connect_disconnect(bytea,uuid,text) IS
 'integration owner; caller internal/metaconnect.Service.Disconnect. One transaction for ONE Page, serialized with meta_connect_finish on the per-store advisory lock (0100 D1): enqueues that Page''s unsubscribe job (0100 D2, a copy of the head sealed token for the claims-worker), destroys every credential head/version of its bindings, disables its routes, deletes its connection row; the caller then disables the bindings. Refusals: not_found (MC404), invalid_request (MC422).';

-- ---------------------------------------------------------------------------------------
-- D3 (B1): live.put_claim_source (0064) — a deactivation may retire a source whose route was
-- disabled by a Page disconnect. 0064 is released, so the function is CREATE OR REPLACE'd here
-- (0108, unreleased, edited in place). The only change: an ACTIVE source still requires an
-- enabled (object, asset) route of this very store, but a deactivation (p_active=false) resolves
-- to this store's binding even when that route is already disabled — it never relaxes the
-- tenant/store scope, the principal_holds(live:manage,integration:execute) check, the version CAS
-- or the private_reply page-credential check, and a NEW (active) source still needs an enabled route.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION live.put_claim_source(p_session uuid,p_object text,p_asset text,p_object_id text,
 p_private_reply boolean,p_locale text,p_active boolean,p_expected_version bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_principal uuid; v_bindings uuid[]; b record; v_id uuid; v_version bigint;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.principal_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL
  OR p_session IS NULL OR p_object IS NULL OR p_object NOT IN ('page','instagram')
  OR p_asset IS NULL OR p_asset !~ '^[0-9]{1,40}$' OR p_object_id IS NULL OR p_object_id !~ '^[0-9_]{1,80}$'
  OR p_private_reply IS NULL OR p_locale IS NULL OR p_locale NOT IN ('zh-TW','zh-CN','en')
  OR p_active IS NULL OR p_expected_version IS NULL OR p_expected_version<0 OR p_expected_version>=9223372036854775807 THEN
  RAISE EXCEPTION 'invalid claim source' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 v_principal:=current_setting('app.principal_id')::uuid;
 IF NOT identity.principal_holds(v_tenant,v_store,v_principal,ARRAY['live:manage','integration:execute']) THEN
  RAISE EXCEPTION 'claim source access unavailable' USING ERRCODE='PT403';
 END IF;
 -- (object, asset) must resolve to this very store's binding; several apps may route one asset, but they must all point at
 -- one binding. An ACTIVE source additionally requires the route to be enabled; a deactivation (p_active=false) must still
 -- resolve to this store's binding, but the route may already have been disabled by a Page disconnect (0108 multi-page), so
 -- its enabled flag is not required then. Scope, ownership, version CAS and private_reply checks are unchanged below.
 IF p_active THEN
  SELECT array_agg(DISTINCT r.binding_id) INTO v_bindings FROM meta_inbox.routes r
   WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.object=p_object AND r.asset_id=p_asset AND r.enabled;
 ELSE
  SELECT array_agg(DISTINCT r.binding_id) INTO v_bindings FROM meta_inbox.routes r
   WHERE r.tenant_id=v_tenant AND r.store_id=v_store AND r.object=p_object AND r.asset_id=p_asset;
 END IF;
 IF v_bindings IS NULL OR cardinality(v_bindings)<>1 THEN
  RAISE EXCEPTION 'claim source route mismatch' USING ERRCODE='PT409';
 END IF;
 SELECT x.id,x.provider,x.external_asset_id,x.semantic_version,x.enabled INTO b FROM integration.bindings x
  WHERE x.tenant_id=v_tenant AND x.store_id=v_store AND x.id=v_bindings[1];
 IF NOT FOUND OR b.provider<>(CASE p_object WHEN 'page' THEN 'facebook' ELSE 'instagram' END)
  OR b.external_asset_id<>p_asset OR (p_active AND NOT b.enabled) THEN
  RAISE EXCEPTION 'claim source binding mismatch' USING ERRCODE='PT409';
 END IF;
 IF p_private_reply AND NOT EXISTS(SELECT 1 FROM integration.meta_page_heads h
  WHERE h.tenant_id=v_tenant AND h.store_id=v_store AND h.binding_id=b.id) THEN
  RAISE EXCEPTION 'claim source has no page credential' USING ERRCODE='PT409';
 END IF;
 SELECT s.id,s.version INTO v_id,v_version FROM live.claim_sources s
  WHERE s.tenant_id=v_tenant AND s.store_id=v_store AND s.session_id=p_session AND s.object=p_object
   AND s.asset_id=p_asset AND s.source_object_id=p_object_id FOR UPDATE;
 IF NOT FOUND THEN
  IF p_expected_version<>0 THEN RAISE EXCEPTION 'claim source version changed' USING ERRCODE='PT409'; END IF;
  INSERT INTO live.claim_sources(tenant_id,store_id,session_id,platform,binding_id,binding_version,object,asset_id,
   source_object_id,private_reply,reply_locale,active,version,principal_id)
  VALUES(v_tenant,v_store,p_session,b.provider,b.id,b.semantic_version,p_object,p_asset,p_object_id,p_private_reply,
   p_locale,p_active,1,v_principal) RETURNING id INTO v_id;
 ELSE
  IF v_version<>p_expected_version THEN RAISE EXCEPTION 'claim source version changed' USING ERRCODE='PT409'; END IF;
  UPDATE live.claim_sources s SET binding_id=b.id,binding_version=b.semantic_version,private_reply=p_private_reply,
   reply_locale=p_locale,active=p_active,version=s.version+1,principal_id=v_principal,updated_at=clock_timestamp()
   WHERE s.tenant_id=v_tenant AND s.store_id=v_store AND s.id=v_id;
 END IF;
 RETURN v_id;
END $$;
ALTER FUNCTION live.put_claim_source(uuid,text,text,text,boolean,text,boolean,bigint) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION live.put_claim_source(uuid,text,text,text,boolean,text,boolean,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.put_claim_source(uuid,text,text,text,boolean,text,boolean,bigint) TO commerce_runtime;
COMMENT ON FUNCTION live.put_claim_source(uuid,text,text,text,boolean,text,boolean,bigint) IS
 'internal/claims (T12 admin route); EXECUTE commerce_runtime only. Merchant-transaction guard, principal_holds(live:manage,integration:execute), route/binding checks (0108: an ACTIVE source needs an enabled route; a deactivation may retire a source whose route a Page disconnect already disabled), CAS on version (0 creates). One active source per (object, asset, object id) globally.';
