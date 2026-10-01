-- 0095 merchant self-serve Facebook Page / Instagram connect (contracts/meta-claims-intake-v1.md "Merchant connect (R4)",
-- unit meta-connect). Until now a Page token, binding and route could only be registered by the operator CLI
-- (cmd/meta-admin, login commerce_meta_registrar). This adds the merchant path under the SAME custody rules:
--   * the token is sealed exactly as meta-page-token-v1 (AES-256-GCM, AAD bound to tenant/store/binding/provider/asset/version);
--     only the sealed bytes reach SQL, never plaintext;
--   * every definer authenticates the merchant session by hash (identity.resolve_access, integration:manage / :read) and
--     requires the result to equal this transaction's app.* GUC scope (platform.WithScope set it from the same session);
--   * the Page -> store route is written by a helper owned by commerce_meta_writer (the owner of the 0028 route definers)
--     that skips the registrar-login check but keeps activate_route's lock order and the one-owner-per-asset rule.
-- Owning package: internal/metaconnect (callers: httpapi meta_connect.go, commerce_runtime) and internal/integrations/metareply
-- (mark_reauth, commerce_worker). Non-goals: no Graph call here, no plaintext token, no Page-token read for anyone but the
-- disconnecting merchant's own API call (see meta_connect_disconnect), the 0064 loader/registrar stay as they are.

-- ---------------------------------------------------------------------------------------
-- Tables (all FORCE RLS, PUBLIC revoked; the only reader/writer is commerce_integration_writer through the definers below).
-- ---------------------------------------------------------------------------------------
CREATE TABLE integration.meta_connect_states (
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL, store_id uuid NOT NULL, principal_id uuid NOT NULL,
 state_hash bytea NOT NULL UNIQUE CHECK(octet_length(state_hash)=32),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), used_at timestamptz,
 expires_at timestamptz NOT NULL,
 -- Pages the merchant may pick: [{page_id,name,ig_id?,ig_username?,missing:[..],ig_missing:[..]}]; ids and display names only.
 pages jsonb CHECK(pages IS NULL OR (jsonb_typeof(pages)='array' AND jsonb_array_length(pages)<=25 AND octet_length(pages::text)<=16384)),
 scopes_attested text[] CHECK(scopes_attested IS NULL OR (cardinality(scopes_attested) BETWEEN 1 AND 64
  AND array_to_string(scopes_attested,',') ~ '^[a-z_]{1,64}(,[a-z_]{1,64}){0,63}$')),
 -- The sealed USER token between callback and pick (AES-256-GCM, page-token keyring, AAD scope = this state), deleted on pick
 -- or at expiry (opportunistically by begin and by disconnect; there is no periodic job).
 pending_key_id text CHECK(pending_key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 pending_nonce bytea CHECK(octet_length(pending_nonce)=12),
 pending_ciphertext bytea CHECK(octet_length(pending_ciphertext) BETWEEN 17 AND 8192),
 done_at timestamptz,
 CHECK(used_at IS NULL OR used_at >= created_at), CHECK(expires_at > created_at),
 CHECK((pending_ciphertext IS NULL) = (pending_nonce IS NULL) AND (pending_nonce IS NULL) = (pending_key_id IS NULL)),
 UNIQUE(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id));
CREATE INDEX meta_connect_states_expiry ON integration.meta_connect_states(expires_at) WHERE pending_ciphertext IS NOT NULL;

CREATE TABLE integration.meta_connections (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, PRIMARY KEY(tenant_id,store_id),
 page_id text NOT NULL UNIQUE CHECK(page_id ~ '^[0-9]{1,40}$'),
 page_name text NOT NULL CHECK(length(page_name) BETWEEN 0 AND 200),
 fb_binding uuid NOT NULL, ig_binding uuid,
 ig_id text UNIQUE CHECK(ig_id ~ '^[0-9]{1,40}$'), ig_username text CHECK(length(ig_username) BETWEEN 0 AND 200),
 scopes text[] NOT NULL CHECK(cardinality(scopes) BETWEEN 1 AND 64),
 status text NOT NULL CHECK(status IN ('active','reauth_required')),
 connected_by uuid NOT NULL,
 connected_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 route_expires_at timestamptz NOT NULL,
 CHECK((ig_binding IS NULL) = (ig_id IS NULL)),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,fb_binding) REFERENCES integration.bindings(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,ig_binding) REFERENCES integration.bindings(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,connected_by) REFERENCES identity.memberships(tenant_id,principal_id));

ALTER TABLE integration.meta_connect_states ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.meta_connect_states FORCE ROW LEVEL SECURITY;
ALTER TABLE integration.meta_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.meta_connections FORCE ROW LEVEL SECURITY;
REVOKE ALL ON integration.meta_connect_states, integration.meta_connections FROM PUBLIC;

-- The definer owner is the control (0064 page_credential_writer pattern): its bodies decide every row.
GRANT SELECT, INSERT, UPDATE(used_at,pages,scopes_attested,pending_key_id,pending_nonce,pending_ciphertext,done_at)
 ON integration.meta_connect_states TO commerce_integration_writer;
CREATE POLICY meta_connect_states_writer ON integration.meta_connect_states FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
GRANT SELECT, INSERT, DELETE, UPDATE(page_name,fb_binding,ig_binding,ig_id,ig_username,scopes,status,connected_by,connected_at,updated_at,route_expires_at)
 ON integration.meta_connections TO commerce_integration_writer;
CREATE POLICY meta_connections_writer ON integration.meta_connections FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
-- Disconnect destroys the sealed Page token (head and every version): 0064 granted only SELECT/INSERT/UPDATE(head).
GRANT DELETE ON integration.meta_page_credentials, integration.meta_page_heads TO commerce_integration_writer;
-- The route helpers below live in meta_inbox (owner commerce_meta_writer) and are called by the integration definers.
GRANT USAGE ON SCHEMA meta_inbox TO commerce_integration_writer;
-- "Last webhook received" per asset: the card's one read of meta_inbox.events.
CREATE INDEX events_asset_recent ON meta_inbox.events(object,asset_id,created_at DESC) WHERE tenant_id IS NOT NULL;

-- ---------------------------------------------------------------------------------------
-- meta_inbox route helpers (owner commerce_meta_writer; EXECUTE commerce_integration_writer only).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION meta_inbox.connect_owner_ok(p_object text,p_asset text,p_tenant uuid,p_store uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT NOT EXISTS(SELECT 1 FROM meta_inbox.asset_owners o WHERE o.object=p_object AND o.asset_id=p_asset
  AND (o.tenant_id<>p_tenant OR o.store_id<>p_store)) $$;

CREATE FUNCTION meta_inbox.connect_activate(p_app text,p_object text,p_asset text,p_tenant uuid,p_store uuid,p_binding uuid,p_expires timestamptz)
RETURNS TABLE(route_id uuid,route_epoch bigint) LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r meta_inbox.routes%ROWTYPE; o meta_inbox.asset_owners%ROWTYPE; v_ver bigint; v_proof text;
BEGIN
 IF p_app IS NULL OR p_app !~ '^[0-9]{1,40}$' OR p_object IS NULL OR p_object NOT IN ('page','instagram') OR p_asset IS NULL OR p_asset !~ '^[0-9]{1,40}$'
  OR p_tenant IS NULL OR p_store IS NULL OR p_binding IS NULL OR p_expires IS NULL OR NOT isfinite(p_expires) OR p_expires<=clock_timestamp() THEN
  RAISE EXCEPTION 'invalid meta route' USING ERRCODE='22023'; END IF;
 -- Same lock order as 0028 activate_route / the receiver: scope -> binding -> asset owner -> route.
 PERFORM 1 FROM control.tenants WHERE id=p_tenant AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'inactive meta scope' USING ERRCODE='PT409'; END IF;
 PERFORM 1 FROM control.stores WHERE tenant_id=p_tenant AND id=p_store AND active FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'inactive meta scope' USING ERRCODE='PT409'; END IF;
 SELECT b.semantic_version INTO v_ver FROM integration.bindings b WHERE b.id=p_binding AND b.tenant_id=p_tenant AND b.store_id=p_store
  AND b.enabled AND b.external_asset_id=p_asset AND b.provider=CASE p_object WHEN 'page' THEN 'facebook' ELSE 'instagram' END FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid meta binding' USING ERRCODE='PT409'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended(jsonb_build_array('meta-owner',p_object,p_asset)::text,0));
 SELECT * INTO o FROM meta_inbox.asset_owners WHERE object=p_object AND asset_id=p_asset;
 IF FOUND AND (o.tenant_id<>p_tenant OR o.store_id<>p_store) THEN RAISE EXCEPTION 'meta owner conflict' USING ERRCODE='PT409'; END IF;
 IF o.asset_id IS NULL THEN INSERT INTO meta_inbox.asset_owners VALUES(p_object,p_asset,p_tenant,p_store); END IF;
 SELECT * INTO r FROM meta_inbox.routes WHERE app_id=p_app AND object=p_object AND asset_id=p_asset FOR UPDATE;
 IF r.id IS NOT NULL AND (r.binding_id<>p_binding OR r.tenant_id<>p_tenant OR r.store_id<>p_store) THEN
  RAISE EXCEPTION 'meta route conflict' USING ERRCODE='PT409'; END IF;
 -- The proof is the merchant's own Facebook Login (Page admin with MESSAGING + MODERATE tasks, checked by internal/metaconnect);
 -- proof_hash is a deterministic digest of the scope, proof_expires a yearly refresh (reconnect renews it).
 v_proof:=encode(sha256(convert_to('merchant-oauth|'||p_tenant||'|'||p_store||'|'||p_object||'|'||p_asset,'UTF8')),'hex');
 IF r.id IS NULL THEN
  INSERT INTO meta_inbox.routes(app_id,object,asset_id,tenant_id,store_id,binding_id,binding_version,route_epoch,proof_hash,proof_expires)
   VALUES(p_app,p_object,p_asset,p_tenant,p_store,p_binding,v_ver,1,v_proof,p_expires) RETURNING * INTO r;
 ELSE
  UPDATE meta_inbox.routes SET binding_version=v_ver,route_epoch=r.route_epoch+1,proof_hash=v_proof,proof_expires=p_expires,enabled=true
   WHERE id=r.id RETURNING * INTO r;
 END IF;
 INSERT INTO meta_inbox.audit_events(route_id,action,evidence_hash) VALUES(r.id,'activate',v_proof);
 RETURN QUERY SELECT r.id,r.route_epoch;
END $$;

CREATE FUNCTION meta_inbox.connect_disable(p_tenant uuid,p_store uuid,p_object text,p_asset text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r meta_inbox.routes%ROWTYPE;
BEGIN
 FOR r IN SELECT * FROM meta_inbox.routes x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.object=p_object AND x.asset_id=p_asset
   AND x.enabled FOR UPDATE LOOP
  UPDATE meta_inbox.routes SET enabled=false,route_epoch=r.route_epoch+1 WHERE id=r.id;
  INSERT INTO meta_inbox.audit_events(route_id,action) VALUES(r.id,'disable');
 END LOOP;
END $$;

CREATE FUNCTION meta_inbox.connect_last_event(p_tenant uuid,p_store uuid,p_object text,p_asset text) RETURNS timestamptz
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT max(e.created_at) FROM meta_inbox.events e WHERE e.object=p_object AND e.asset_id=p_asset AND e.tenant_id=p_tenant AND e.store_id=p_store $$;

DO $$
DECLARE f text;
BEGIN
 FOREACH f IN ARRAY ARRAY['meta_inbox.connect_owner_ok(text,text,uuid,uuid)',
  'meta_inbox.connect_activate(text,text,text,uuid,uuid,uuid,timestamptz)','meta_inbox.connect_disable(uuid,uuid,text,text)',
  'meta_inbox.connect_last_event(uuid,uuid,text,text)'] LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_meta_writer',f);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
  EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_integration_writer',f);
 END LOOP;
END $$;
COMMENT ON FUNCTION meta_inbox.connect_owner_ok(text,text,uuid,uuid) IS
 'meta_inbox owner (commerce_meta_writer); only caller integration.meta_connect_prepare (commerce_integration_writer). True iff the Page/IG asset has no owner or this store owns it; never names the other owner.';
COMMENT ON FUNCTION meta_inbox.connect_activate(text,text,text,uuid,uuid,uuid,timestamptz) IS
 'meta_inbox owner (commerce_meta_writer); only caller integration.meta_connect_finish. Merchant-OAuth variant of 0028 activate_route: no registrar-login check (the caller definer authenticated the merchant session), binding_version taken from the enabled binding, epoch derived under the route row lock; same lock order and one-owner-per-asset rule (PT409 on another store).';
COMMENT ON FUNCTION meta_inbox.connect_disable(uuid,uuid,text,text) IS
 'meta_inbox owner (commerce_meta_writer); only caller integration.meta_connect_disconnect. Disables the store''s enabled routes of one Page/IG asset (epoch +1, audited disable); the asset_owners row stays so the Page cannot silently move to another store.';
COMMENT ON FUNCTION meta_inbox.connect_last_event(uuid,uuid,text,text) IS
 'meta_inbox owner (commerce_meta_writer); only caller integration.meta_connect_status. Latest created_at of a ROUTED event of this store for the asset (the card''s "last webhook received"); no body, no id.';

-- ---------------------------------------------------------------------------------------
-- integration.meta_connect_* (owner commerce_integration_writer, EXECUTE commerce_runtime; mark_reauth: commerce_worker).
-- Refusals: SQLSTATE MCnnn (nnn = HTTP status), MESSAGE = a frozen code; Go (internal/metaconnect) maps it.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.meta_connect_auth(p_hash bytea,p_store uuid,p_permission text)
RETURNS TABLE(out_tenant uuid,out_principal uuid) LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_permission NOT IN ('integration:manage','integration:read')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 SELECT * INTO v FROM identity.resolve_access(p_hash,p_store,p_permission);
 IF v.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='MC401'; END IF;
 IF v.access_status='not_found' THEN RAISE EXCEPTION 'not_found' USING ERRCODE='MC404'; END IF;
 IF v.access_status<>'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='MC403'; END IF;
 -- The authenticated session must be the one platform.WithScope pinned for this transaction.
 IF v.tenant_id IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v.principal_id IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='MC403'; END IF;
 RETURN QUERY SELECT v.tenant_id,v.principal_id;
END $$;

CREATE FUNCTION integration.meta_connect_begin(p_hash bytea,p_store uuid,p_state_hash bytea)
RETURNS TABLE(out_state uuid,out_expires timestamptz) LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v_id uuid:=gen_random_uuid(); v_exp timestamptz;
BEGIN
 IF p_state_hash IS NULL OR octet_length(p_state_hash)<>32 THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 -- ponytail: the expired user-token ciphertexts are wiped here (every connect start) and at disconnect; no periodic job.
 UPDATE integration.meta_connect_states s SET pending_key_id=NULL,pending_nonce=NULL,pending_ciphertext=NULL
  WHERE s.pending_ciphertext IS NOT NULL AND s.expires_at<=clock_timestamp();
 IF (SELECT count(*) FROM integration.meta_connect_states s WHERE s.tenant_id=a.out_tenant AND s.store_id=p_store
   AND s.used_at IS NULL AND s.expires_at>clock_timestamp())>=10 THEN
  RAISE EXCEPTION 'conflict' USING ERRCODE='MC409'; END IF;
 v_exp:=clock_timestamp()+interval '10 minutes';
 INSERT INTO integration.meta_connect_states(id,tenant_id,store_id,principal_id,state_hash,expires_at)
  VALUES(v_id,a.out_tenant,p_store,a.out_principal,p_state_hash,v_exp);
 RETURN QUERY SELECT v_id,v_exp;
END $$;

CREATE FUNCTION integration.meta_connect_consume(p_hash bytea,p_store uuid,p_state_hash bytea) RETURNS uuid
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states;
BEGIN
 IF p_state_hash IS NULL OR octet_length(p_state_hash)<>32 THEN RAISE EXCEPTION 'state_mismatch' USING ERRCODE='MC409'; END IF;
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 SELECT * INTO s FROM integration.meta_connect_states x WHERE x.state_hash=p_state_hash FOR UPDATE;
 -- One uniform refusal for unknown, foreign or used state: no oracle for another principal's state.
 IF NOT FOUND OR s.tenant_id<>a.out_tenant OR s.store_id<>p_store OR s.principal_id<>a.out_principal OR s.used_at IS NOT NULL THEN
  RAISE EXCEPTION 'state_mismatch' USING ERRCODE='MC409'; END IF;
 IF s.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'state_expired' USING ERRCODE='MC410'; END IF;
 UPDATE integration.meta_connect_states x SET used_at=clock_timestamp() WHERE x.id=s.id;
 RETURN s.id;
END $$;

CREATE FUNCTION integration.meta_connect_put_result(p_hash bytea,p_store uuid,p_state uuid,p_pages jsonb,p_scopes text[],
 p_key_id text,p_nonce bytea,p_ciphertext bytea) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states; e jsonb;
BEGIN
 IF p_state IS NULL OR p_pages IS NULL OR jsonb_typeof(p_pages)<>'array' OR jsonb_array_length(p_pages)>25 OR octet_length(p_pages::text)>16384
  OR p_scopes IS NULL OR array_ndims(p_scopes)<>1 OR cardinality(p_scopes) NOT BETWEEN 1 AND 64
  OR array_to_string(p_scopes,',') !~ '^[a-z_]{1,64}(,[a-z_]{1,64}){0,63}$'
  OR p_key_id IS NULL OR p_key_id !~ '^[A-Za-z0-9_-]{1,64}$' OR p_nonce IS NULL OR octet_length(p_nonce)<>12
  OR p_ciphertext IS NULL OR octet_length(p_ciphertext) NOT BETWEEN 17 AND 8192 THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 FOR e IN SELECT * FROM jsonb_array_elements(p_pages) LOOP
  IF jsonb_typeof(e)<>'object' OR e->>'page_id' IS NULL OR e->>'page_id' !~ '^[0-9]{1,40}$'
   OR (e->>'ig_id' IS NOT NULL AND e->>'ig_id' !~ '^[0-9]{1,40}$')
   OR jsonb_typeof(e->'missing')<>'array' OR jsonb_typeof(e->'ig_missing')<>'array' THEN
   RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 END LOOP;
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 SELECT * INTO s FROM integration.meta_connect_states x WHERE x.id=p_state AND x.tenant_id=a.out_tenant AND x.store_id=p_store
  AND x.principal_id=a.out_principal FOR UPDATE;
 IF NOT FOUND OR s.used_at IS NULL OR s.pages IS NOT NULL THEN RAISE EXCEPTION 'state_mismatch' USING ERRCODE='MC409'; END IF;
 IF s.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'state_expired' USING ERRCODE='MC410'; END IF;
 UPDATE integration.meta_connect_states x SET pages=p_pages,scopes_attested=p_scopes,pending_key_id=p_key_id,pending_nonce=p_nonce,
  pending_ciphertext=p_ciphertext WHERE x.id=s.id;
END $$;

CREATE FUNCTION integration.meta_connect_get_state(p_hash bytea,p_store uuid,p_state uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states;
BEGIN
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 SELECT * INTO s FROM integration.meta_connect_states x WHERE x.id=p_state AND x.tenant_id=a.out_tenant AND x.store_id=p_store
  AND x.principal_id=a.out_principal;
 IF NOT FOUND THEN RAISE EXCEPTION 'state_mismatch' USING ERRCODE='MC409'; END IF;
 IF s.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'state_expired' USING ERRCODE='MC410'; END IF;
 IF s.pages IS NULL THEN RAISE EXCEPTION 'not_found' USING ERRCODE='MC404'; END IF;
 IF s.done_at IS NOT NULL THEN RAISE EXCEPTION 'state_used' USING ERRCODE='MC409'; END IF;
 RETURN jsonb_build_object('state_id',s.id,'expires_at',to_char(s.expires_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
  'scopes',to_jsonb(s.scopes_attested),'pages',s.pages);
END $$;

-- Pick, step 1 (before any Meta call): the state must be this principal's, consumed, unexpired, not finished, and the chosen
-- Page (and IG) must be pickable. Returns the sealed user token so the API can fetch the Page token (it never leaves memory).
CREATE FUNCTION integration.meta_connect_prepare(p_hash bytea,p_store uuid,p_state uuid,p_page text,p_with_ig boolean) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states; e jsonb; v_page jsonb; v_other text;
BEGIN
 IF p_page IS NULL OR p_page !~ '^[0-9]{1,40}$' OR p_with_ig IS NULL THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 SELECT * INTO s FROM integration.meta_connect_states x WHERE x.id=p_state AND x.tenant_id=a.out_tenant AND x.store_id=p_store
  AND x.principal_id=a.out_principal FOR UPDATE;
 IF NOT FOUND OR s.used_at IS NULL OR s.pages IS NULL THEN RAISE EXCEPTION 'state_mismatch' USING ERRCODE='MC409'; END IF;
 IF s.expires_at<=clock_timestamp() THEN RAISE EXCEPTION 'state_expired' USING ERRCODE='MC410'; END IF;
 IF s.done_at IS NOT NULL OR s.pending_ciphertext IS NULL THEN RAISE EXCEPTION 'state_used' USING ERRCODE='MC409'; END IF;
 FOR e IN SELECT * FROM jsonb_array_elements(s.pages) LOOP
  IF e->>'page_id'=p_page THEN v_page:=e; END IF;
 END LOOP;
 IF v_page IS NULL THEN RAISE EXCEPTION 'not_in_pick_list' USING ERRCODE='MC422'; END IF;
 -- Never partial-enable: a missing permission or task refuses the whole pick (the card lists what to re-grant).
 IF jsonb_array_length(v_page->'missing')>0 THEN RAISE EXCEPTION 'missing_permission' USING ERRCODE='MC422'; END IF;
 IF p_with_ig AND (v_page->>'ig_id' IS NULL OR jsonb_array_length(v_page->'ig_missing')>0) THEN
  RAISE EXCEPTION 'missing_permission' USING ERRCODE='MC422'; END IF;
 SELECT c.page_id INTO v_other FROM integration.meta_connections c WHERE c.tenant_id=a.out_tenant AND c.store_id=p_store;
 IF FOUND AND v_other<>p_page THEN RAISE EXCEPTION 'already_connected' USING ERRCODE='MC409'; END IF;
 -- One Page / IG account belongs to one store platform-wide; the refusal never names the other store.
 IF NOT meta_inbox.connect_owner_ok('page',p_page,a.out_tenant,p_store)
  OR (p_with_ig AND NOT meta_inbox.connect_owner_ok('instagram',v_page->>'ig_id',a.out_tenant,p_store)) THEN
  RAISE EXCEPTION 'page_taken' USING ERRCODE='MC409'; END IF;
 RETURN jsonb_build_object('page',v_page,'scopes',to_jsonb(s.scopes_attested),'key_id',s.pending_key_id,
  'nonce',encode(s.pending_nonce,'base64'),'ciphertext',encode(s.pending_ciphertext,'base64'));
END $$;

-- The head version of a binding's Page credential (0 = none): the CAS input of the seal (its AAD carries the version).
CREATE FUNCTION integration.meta_connect_head(p_hash bytea,p_store uuid,p_binding uuid) RETURNS bigint
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; v bigint;
BEGIN
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 SELECT h.current_version INTO v FROM integration.meta_page_heads h WHERE h.tenant_id=a.out_tenant AND h.store_id=p_store AND h.binding_id=p_binding;
 RETURN coalesce(v,0);
END $$;

-- Pick, step 2 (one transaction): credentials (CAS), routes, connection row, state done. Any refusal rolls back everything.
CREATE FUNCTION integration.meta_connect_finish(p_hash bytea,p_store uuid,p_state uuid,p_page text,
 p_fb_binding uuid,p_fb_expected bigint,p_fb_key text,p_fb_nonce bytea,p_fb_ct bytea,
 p_ig_binding uuid,p_ig_expected bigint,p_ig_key text,p_ig_nonce bytea,p_ig_ct bytea,
 p_app_page text,p_app_ig text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states; e jsonb; v_page jsonb; v_other text; b record; v_exp timestamptz;
 v_fb text[]:=ARRAY['pages_show_list','pages_manage_metadata','pages_read_engagement','pages_messaging'];
 v_ig text[]:=ARRAY['instagram_basic','instagram_manage_comments','instagram_manage_messages'];
BEGIN
 IF p_page IS NULL OR p_page !~ '^[0-9]{1,40}$' OR p_fb_binding IS NULL OR p_fb_expected IS NULL OR p_fb_expected<0 OR p_fb_expected>=1<<62
  OR p_fb_key IS NULL OR p_fb_key !~ '^[A-Za-z0-9_-]{1,64}$' OR p_fb_nonce IS NULL OR octet_length(p_fb_nonce)<>12
  OR p_fb_ct IS NULL OR octet_length(p_fb_ct) NOT BETWEEN 17 AND 8192 OR p_app_page IS NULL OR p_app_page !~ '^[0-9]{1,40}$'
  OR ((p_ig_binding IS NULL) IS DISTINCT FROM (p_ig_expected IS NULL)) OR ((p_ig_binding IS NULL) IS DISTINCT FROM (p_ig_key IS NULL))
  OR ((p_ig_binding IS NULL) IS DISTINCT FROM (p_ig_nonce IS NULL)) OR ((p_ig_binding IS NULL) IS DISTINCT FROM (p_ig_ct IS NULL))
  OR (p_ig_binding IS NOT NULL AND (p_ig_expected<0 OR p_ig_expected>=1<<62 OR p_ig_key !~ '^[A-Za-z0-9_-]{1,64}$'
   OR octet_length(p_ig_nonce)<>12 OR octet_length(p_ig_ct) NOT BETWEEN 17 AND 8192 OR p_app_ig IS NULL OR p_app_ig !~ '^[0-9]{1,40}$')) THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
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
 UPDATE integration.meta_connect_states x SET done_at=clock_timestamp(),pending_key_id=NULL,pending_nonce=NULL,pending_ciphertext=NULL WHERE x.id=s.id;
 RETURN jsonb_build_object('page_id',p_page,'instagram',p_ig_binding IS NOT NULL);
END $$;

-- Internal: one credential version under the CAS (shared by both providers of a connect). No caller EXECUTE beyond the owner.
CREATE FUNCTION integration.meta_connect_put_credential(p_tenant uuid,p_store uuid,p_principal uuid,p_binding uuid,p_provider text,
 p_asset text,p_expected bigint,p_key text,p_nonce bytea,p_ct bytea,p_scopes text[]) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_head bigint; v_next bigint;
BEGIN
 IF p_expected=0 THEN
  IF EXISTS(SELECT 1 FROM integration.meta_page_heads h WHERE h.tenant_id=p_tenant AND h.store_id=p_store AND h.binding_id=p_binding) THEN
   RAISE EXCEPTION 'conflict' USING ERRCODE='MC409'; END IF;
  INSERT INTO integration.meta_page_heads(tenant_id,store_id,binding_id,current_version) VALUES(p_tenant,p_store,p_binding,1);
  v_next:=1;
 ELSE
  SELECT h.current_version INTO v_head FROM integration.meta_page_heads h
   WHERE h.tenant_id=p_tenant AND h.store_id=p_store AND h.binding_id=p_binding FOR UPDATE;
  IF NOT FOUND OR v_head<>p_expected THEN RAISE EXCEPTION 'conflict' USING ERRCODE='MC409'; END IF;
  v_next:=p_expected+1;
  UPDATE integration.meta_page_heads h SET current_version=v_next,updated_at=clock_timestamp()
   WHERE h.tenant_id=p_tenant AND h.store_id=p_store AND h.binding_id=p_binding;
 END IF;
 INSERT INTO integration.meta_page_credentials(tenant_id,store_id,binding_id,provider,asset_id,version,key_id,nonce,ciphertext,scopes_attested,principal_id)
  VALUES(p_tenant,p_store,p_binding,p_provider,p_asset,v_next,p_key,p_nonce,p_ct,p_scopes,p_principal);
END $$;

CREATE FUNCTION integration.meta_connect_status(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; c integration.meta_connections; v_last timestamptz; v_ig timestamptz;
BEGIN
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:read');
 SELECT * INTO c FROM integration.meta_connections x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store;
 IF NOT FOUND THEN RETURN jsonb_build_object('connected',false); END IF;
 v_last:=meta_inbox.connect_last_event(a.out_tenant,p_store,'page',c.page_id);
 IF c.ig_id IS NOT NULL THEN v_ig:=meta_inbox.connect_last_event(a.out_tenant,p_store,'instagram',c.ig_id); END IF;
 RETURN jsonb_build_object('connected',true,'status',c.status,'page',jsonb_build_object('id',c.page_id,'name',c.page_name),
  'instagram',CASE WHEN c.ig_id IS NULL THEN NULL ELSE jsonb_build_object('id',c.ig_id,'username',coalesce(c.ig_username,'')) END,
  'permissions',to_jsonb(c.scopes),
  'connected_at',to_char(c.connected_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
  'route_expires_at',to_char(c.route_expires_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
  'last_event_at',CASE WHEN greatest(v_last,v_ig) IS NULL THEN NULL
   ELSE to_char(greatest(v_last,v_ig) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"') END);
END $$;

-- Disconnect: ONE transaction destroys the sealed Page tokens and disables the routes. It returns the head ciphertexts once so
-- the API can make its best-effort DELETE subscribed_apps with the in-memory token after COMMIT (the ciphertext is useless
-- without the Page-token keyring and its AAD); the caller then disables the bindings (core.SetBindingEnabled).
CREATE FUNCTION integration.meta_connect_disconnect(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; c integration.meta_connections; r record; v_creds jsonb:='[]'::jsonb;
BEGIN
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 SELECT * INTO c FROM integration.meta_connections x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='MC404'; END IF;
 FOR r IN SELECT k.binding_id,k.provider,k.asset_id,k.version,k.key_id,k.nonce,k.ciphertext FROM integration.meta_page_credentials k
   JOIN integration.meta_page_heads h ON h.tenant_id=k.tenant_id AND h.store_id=k.store_id AND h.binding_id=k.binding_id AND h.current_version=k.version
   WHERE k.tenant_id=a.out_tenant AND k.store_id=p_store AND k.binding_id IN (c.fb_binding,c.ig_binding) LOOP
  v_creds:=v_creds||jsonb_build_object('binding',r.binding_id,'provider',r.provider,'asset',r.asset_id,'version',r.version,'key_id',r.key_id,
   'nonce',encode(r.nonce,'base64'),'ciphertext',encode(r.ciphertext,'base64'));
 END LOOP;
 -- Head first, then the versions (the 0064 head FK is deferred anyway): no credential of these bindings survives.
 DELETE FROM integration.meta_page_heads h WHERE h.tenant_id=a.out_tenant AND h.store_id=p_store AND h.binding_id IN (c.fb_binding,c.ig_binding);
 DELETE FROM integration.meta_page_credentials k WHERE k.tenant_id=a.out_tenant AND k.store_id=p_store AND k.binding_id IN (c.fb_binding,c.ig_binding);
 PERFORM meta_inbox.connect_disable(a.out_tenant,p_store,'page',c.page_id);
 IF c.ig_id IS NOT NULL THEN PERFORM meta_inbox.connect_disable(a.out_tenant,p_store,'instagram',c.ig_id); END IF;
 UPDATE integration.meta_connect_states x SET pending_key_id=NULL,pending_nonce=NULL,pending_ciphertext=NULL
  WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.pending_ciphertext IS NOT NULL;
 DELETE FROM integration.meta_connections x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store;
 RETURN jsonb_build_object('page_id',c.page_id,'fb_binding',c.fb_binding,'ig_binding',c.ig_binding,'credentials',v_creds);
END $$;

-- Graph 190 (token invalid / revoked) seen by the private-reply dispatcher: flip the card to "reconnect". Only ever moves
-- active -> reauth_required, so a forged operation id can at worst ask the merchant to reconnect.
CREATE FUNCTION integration.meta_connect_mark_reauth(p_operation uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o record;
BEGIN
 IF p_operation IS NULL THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 SELECT x.tenant_id,x.store_id,x.binding_id INTO o FROM integration.operations x WHERE x.id=p_operation AND x.action='meta.private_reply';
 IF NOT FOUND THEN RETURN; END IF;
 UPDATE integration.meta_connections c SET status='reauth_required',updated_at=clock_timestamp()
  WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND c.status='active' AND o.binding_id IN (c.fb_binding,c.ig_binding);
END $$;

DO $$
DECLARE f text;
BEGIN
 FOREACH f IN ARRAY ARRAY['integration.meta_connect_auth(bytea,uuid,text)','integration.meta_connect_begin(bytea,uuid,bytea)',
  'integration.meta_connect_consume(bytea,uuid,bytea)','integration.meta_connect_put_result(bytea,uuid,uuid,jsonb,text[],text,bytea,bytea)',
  'integration.meta_connect_get_state(bytea,uuid,uuid)','integration.meta_connect_prepare(bytea,uuid,uuid,text,boolean)',
  'integration.meta_connect_head(bytea,uuid,uuid)',
  'integration.meta_connect_finish(bytea,uuid,uuid,text,uuid,bigint,text,bytea,bytea,uuid,bigint,text,bytea,bytea,text,text)',
  'integration.meta_connect_put_credential(uuid,uuid,uuid,uuid,text,text,bigint,text,bytea,bytea,text[])',
  'integration.meta_connect_status(bytea,uuid)','integration.meta_connect_disconnect(bytea,uuid)',
  'integration.meta_connect_mark_reauth(uuid)'] LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_integration_writer',f);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
 END LOOP;
 FOREACH f IN ARRAY ARRAY['integration.meta_connect_begin(bytea,uuid,bytea)','integration.meta_connect_consume(bytea,uuid,bytea)',
  'integration.meta_connect_put_result(bytea,uuid,uuid,jsonb,text[],text,bytea,bytea)','integration.meta_connect_get_state(bytea,uuid,uuid)',
  'integration.meta_connect_prepare(bytea,uuid,uuid,text,boolean)','integration.meta_connect_head(bytea,uuid,uuid)',
  'integration.meta_connect_finish(bytea,uuid,uuid,text,uuid,bigint,text,bytea,bytea,uuid,bigint,text,bytea,bytea,text,text)',
  'integration.meta_connect_status(bytea,uuid)','integration.meta_connect_disconnect(bytea,uuid)'] LOOP
  EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime',f);
 END LOOP;
 GRANT EXECUTE ON FUNCTION integration.meta_connect_mark_reauth(uuid) TO commerce_worker;
END $$;

COMMENT ON TABLE integration.meta_connect_states IS
 'internal/metaconnect: single-use Facebook Login for Business state per principal and store (10 min) holding the Page pick list and the sealed USER token until pick or expiry. Written only by integration.meta_connect_* definers (commerce_integration_writer). Non-goal: never a plaintext token, never a Page token.';
COMMENT ON TABLE integration.meta_connections IS
 'internal/metaconnect: the store''s merchant-connected Facebook Page (and optional Instagram account): ids, display names, granted permissions, status active|reauth_required, route expiry. One row per store; deleted on disconnect. Written only by integration.meta_connect_finish/disconnect/mark_reauth. Non-goal: no token or secret column.';
COMMENT ON FUNCTION integration.meta_connect_auth(bytea,uuid,text) IS
 'integration owner; internal helper of the meta_connect definers, no caller EXECUTE. identity.resolve_access for integration:manage|read plus equality with the transaction GUC scope; refusals MC401/MC403/MC404.';
COMMENT ON FUNCTION integration.meta_connect_begin(bytea,uuid,bytea) IS
 'integration owner; only caller internal/metaconnect.Service.Start (commerce_runtime, integration:manage). Stores sha256(state) for this principal and store (10 min, single use, at most 10 live per store) and wipes expired pending user tokens.';
COMMENT ON FUNCTION integration.meta_connect_consume(bytea,uuid,bytea) IS
 'integration owner; only caller internal/metaconnect.Service.Callback. Marks the state used iff it is this principal''s, this store''s, unused and unexpired (state_mismatch / state_expired); uniform refusal otherwise.';
COMMENT ON FUNCTION integration.meta_connect_put_result(bytea,uuid,uuid,jsonb,text[],text,bytea,bytea) IS
 'integration owner; only caller internal/metaconnect.Service.Callback after the code exchange. Stores the pick list (ids and names), granted permissions and the sealed user token on the consumed state, once.';
COMMENT ON FUNCTION integration.meta_connect_get_state(bytea,uuid,uuid) IS
 'integration owner; only caller GET meta-connect/states/{id} (commerce_runtime). Pick list and granted permissions of this principal''s unexpired, unfinished state; never the sealed token.';
COMMENT ON FUNCTION integration.meta_connect_prepare(bytea,uuid,uuid,text,boolean) IS
 'integration owner; only caller internal/metaconnect.Service.Pick step 1. Validates the pick (state, pickable Page/IG, one-store-per-Page via meta_inbox.connect_owner_ok) BEFORE any Meta call and returns the sealed user token so the API can fetch the Page token; refusals missing_permission / page_taken / already_connected.';
COMMENT ON FUNCTION integration.meta_connect_head(bytea,uuid,uuid) IS
 'integration owner; only caller internal/metaconnect.Service.Pick (commerce_runtime). The current Page-credential version of a binding of this store (0 = none), the CAS input of the seal.';
COMMENT ON FUNCTION integration.meta_connect_finish(bytea,uuid,uuid,text,uuid,bigint,text,bytea,bytea,uuid,bigint,text,bytea,bytea,text,text) IS
 'integration owner; only caller internal/metaconnect.Service.Pick step 2. One transaction: sealed Page credential(s) under the 0064 head CAS, merchant routes via meta_inbox.connect_activate, the connection row, state done and pending token wiped. Never receives plaintext.';
COMMENT ON FUNCTION integration.meta_connect_put_credential(uuid,uuid,uuid,uuid,text,text,bigint,text,bytea,bytea,text[]) IS
 'integration owner; internal helper of meta_connect_finish, no caller EXECUTE. One meta_page_credentials version under the head CAS (the 0064 register_meta_page_token rules).';
COMMENT ON FUNCTION integration.meta_connect_status(bytea,uuid) IS
 'integration owner; only caller GET meta-connect/status (commerce_runtime, integration:read). The card: Page, IG, permissions, status, route expiry, last routed webhook time; no token, no ciphertext.';
COMMENT ON FUNCTION integration.meta_connect_disconnect(bytea,uuid) IS
 'integration owner; only caller internal/metaconnect.Service.Disconnect (commerce_runtime, integration:manage). Deletes the sealed Page credentials (heads and versions), disables the Page/IG routes, deletes the connection row and returns the head ciphertexts once for the caller''s best-effort Graph unsubscribe.';
COMMENT ON FUNCTION integration.meta_connect_mark_reauth(uuid) IS
 'integration owner; only caller the private-reply adapter (commerce_worker) on a Graph 190. Moves the operation''s store connection active -> reauth_required; no-op for an unknown operation or an already flagged connection.';
COMMENT ON POLICY meta_connect_states_writer ON integration.meta_connect_states IS '0095: the definer owner reads/writes states; the policy body is the control (0064 page_credential_writer pattern).';
COMMENT ON POLICY meta_connections_writer ON integration.meta_connections IS '0095: the definer owner reads/writes connections; the policy body is the control.';
COMMENT ON INDEX integration.meta_connect_states_expiry IS '0095: finds expired pending user tokens for the opportunistic wipe in meta_connect_begin.';
COMMENT ON INDEX meta_inbox.events_asset_recent IS '0095: the card''s last-webhook read (meta_inbox.connect_last_event); partial on routed events.';
