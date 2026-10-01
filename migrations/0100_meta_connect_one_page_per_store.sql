-- 0100 D1 fix (kimi-evidence/DEFECTS.md, contracts/meta-claims-intake-v1.md "Merchant connect (R4)"): one store - one Page
-- under concurrency. 0095's meta_connect_finish read integration.meta_connections ... FOR UPDATE, but when the store has no
-- row yet there is nothing to lock, so two concurrent picks of DIFFERENT Pages both saw "no other Page"; the INSERT's
-- ON CONFLICT (tenant_id,store_id) DO UPDATE then let the second overwrite the first's connection row, both answered 201 and
-- the losing Page's bindings/routes/credentials stayed live. Fix at the root, forward-only (0095 is never edited):
--   1. meta_connect_finish is CREATE OR REPLACE'd (identical signature/owner/search_path/grants/COMMENT) and takes a
--      transaction-scoped advisory lock keyed on (tenant, store) — the hashtextextended key pattern of 0066/0095 — before
--      reading existing bindings, so the second pick serializes, reads the committed row and raises already_connected
--      (MC409, the contract's typed conflict), never a 500.
--   2. Backstop: a partial unique index enforces at most one ACTIVE connection (Page binding) per store, guarded below so
--      the migration fails with a clear message when existing data already violates the invariant. (One Page per store
--      platform-wide stays enforced by meta_connections.page_id UNIQUE + meta_inbox.asset_owners.)
-- Owning package: internal/metaconnect. Non-goals: no change to prepare/disconnect/mark_reauth, no Graph call, no token.

-- Guard: refuse to install the backstop over data that already violates it (the operator resolves the duplicates first).
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM integration.meta_connections WHERE status='active'
  GROUP BY tenant_id,store_id HAVING count(*)>1) THEN
  RAISE EXCEPTION '0100 meta_connect_one_page_per_store: existing data violates the one-active-Page-binding-per-store invariant (a store has more than one active meta_connections row); disconnect the duplicates, then re-run this migration'
   USING ERRCODE='23505';
 END IF;
END $$;

CREATE UNIQUE INDEX meta_connections_one_active_per_store ON integration.meta_connections(tenant_id,store_id)
 WHERE status='active';
COMMENT ON INDEX integration.meta_connections_one_active_per_store IS
 '0100 (D1): backstop of the meta_connect_finish per-store advisory lock — at most one active Page binding (connection row) per store; written only by integration.meta_connect_finish/disconnect/mark_reauth (internal/metaconnect).';

-- Identical to 0095's meta_connect_finish except the per-store advisory lock after authentication (see the header).
CREATE OR REPLACE FUNCTION integration.meta_connect_finish(p_hash bytea,p_store uuid,p_state uuid,p_page text,
 p_fb_binding uuid,p_fb_expected bigint,p_fb_key text,p_fb_nonce bytea,p_fb_ct bytea,
 p_ig_binding uuid,p_ig_expected bigint,p_ig_key text,p_ig_nonce bytea,p_ig_ct bytea,
 p_app_page text,p_app_ig text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states; e jsonb; v_page jsonb; v_other text; b record; v_exp timestamptz;
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
 -- D1: serialize every finish of one store BEFORE any binding/connection read. With no connection row yet, FOR UPDATE below
 -- locks nothing and two concurrent picks of different Pages both committed (ON CONFLICT overwrote the row, the losing
 -- Page's route/credentials stayed live). The loser now blocks here, then reads the committed row -> already_connected.
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
 SELECT c.page_id INTO v_other FROM integration.meta_connections c WHERE c.tenant_id=a.out_tenant AND c.store_id=p_store FOR UPDATE;
 IF FOUND AND v_other<>p_page THEN RAISE EXCEPTION 'already_connected' USING ERRCODE='MC409'; END IF;
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
 INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,ig_binding,ig_id,ig_username,scopes,status,connected_by,route_expires_at)
  VALUES(a.out_tenant,p_store,p_page,coalesce(v_page->>'name',''),p_fb_binding,p_ig_binding,
   CASE WHEN p_ig_binding IS NULL THEN NULL ELSE v_page->>'ig_id' END,
   CASE WHEN p_ig_binding IS NULL THEN NULL ELSE coalesce(v_page->>'ig_username','') END,
   s.scopes_attested,'active',a.out_principal,v_exp)
  ON CONFLICT (tenant_id,store_id) DO UPDATE SET page_name=EXCLUDED.page_name,fb_binding=EXCLUDED.fb_binding,ig_binding=EXCLUDED.ig_binding,
   ig_id=EXCLUDED.ig_id,ig_username=EXCLUDED.ig_username,scopes=EXCLUDED.scopes,status='active',connected_by=EXCLUDED.connected_by,
   connected_at=clock_timestamp(),updated_at=clock_timestamp(),route_expires_at=EXCLUDED.route_expires_at;
 UPDATE integration.meta_connect_states x SET done_at=clock_timestamp() WHERE x.id=s.id;
 RETURN jsonb_build_object('page_id',p_page,'instagram',p_ig_binding IS NOT NULL);
END $$;

COMMENT ON FUNCTION integration.meta_connect_finish(bytea,uuid,uuid,text,uuid,bigint,text,bytea,bytea,uuid,bigint,text,bytea,bytea,text,text) IS
 'integration owner; only caller internal/metaconnect.Service.Pick step 2. One transaction: sealed Page credential(s) under the 0064 head CAS, merchant routes via meta_inbox.connect_activate, the connection row, state done. Receives only sealed (HPKE v2) bytes, never plaintext.';
