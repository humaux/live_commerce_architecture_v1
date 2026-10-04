-- 0100 meta-connect fixes D1 and D2 (kimi-evidence/DEFECTS.md; contracts/meta-claims-intake-v1.md "Merchant connect (R4)"). Forward-only: 0095 is never edited.
--
-- D1 one store - one Page under concurrency. 0095's meta_connect_finish read integration.meta_connections ... FOR UPDATE, but when the
--   store has no row yet there is nothing to lock, so two concurrent picks of DIFFERENT Pages both saw "no other Page"; the INSERT's
--   ON CONFLICT (tenant_id,store_id) DO UPDATE then let the second overwrite the first's row, both answered 201 and the losing Page's
--   bindings/routes/credentials stayed live. Fix at the root:
--   1. meta_connect_finish (CREATE OR REPLACE, identical signature/owner/search_path/grants/COMMENT) takes a transaction-scoped advisory
--      lock keyed on (tenant, store) -- the hashtextextended key pattern of 0066/0095 -- before reading existing bindings, so the second
--      pick serializes, reads the committed row and raises already_connected (MC409, the contract's typed conflict), never a 500.
--   2. Backstop in the write itself: meta_connections is already PRIMARY KEY(tenant_id,store_id) (a second unique index would be a copy of
--      it), so the hole was the ON CONFLICT DO UPDATE that repointed the row at another Page. It now updates only when page_id matches
--      (reconnect / reauth of the SAME Page) and raises already_connected when it would have touched nothing.
--
-- D2 disconnect leaves the Page subscribed at Meta. The API seals Page tokens to HPKE public keys (meta-page-token-v2) and can never open
--   one, so it cannot call DELETE /{page}/subscribed_apps. Disconnect now enqueues a durable job in the SAME transaction that destroys the
--   credentials: integration.meta_unsubscribe_jobs carries a copy of the head sealed token (same custody: HPKE/AES bytes only) until the
--   claims-worker -- the only holder of the private ring -- opens it and makes the DELETE, best effort. States: PENDING -> LEASED ->
--   SUCCEEDED | FAILED (Meta refused: 4xx) | UNKNOWN (transport error, timeout, 5xx, expired lease: never repeated, the DELETE may have
--   landed) | SUPERSEDED (the same Page was connected again first); a definite not-applied answer (429/503) goes back to PENDING with
--   exponential backoff, at most 5 attempts. Every terminal state wipes the sealed bytes and writes one audit row. The binding, route and
--   connection are disabled/deleted immediately regardless of the job.
-- Owning package: internal/metaconnect (SQL), internal/integrations/metareply (Unsubscriber, run by cmd/claims-worker).
-- Callers: commerce_runtime (disconnect/finish, unchanged grants); commerce_claims_worker (claim/finish of a job, new).

-- ---------------------------------------------------------------------------------------
-- D2 table. FORCE RLS, the definer owner is the control (0064/0095 pattern).
-- ---------------------------------------------------------------------------------------
CREATE TABLE integration.meta_unsubscribe_jobs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, principal_id uuid NOT NULL, binding_id uuid NOT NULL,
 page_id text NOT NULL CHECK(page_id ~ '^[0-9]{1,40}$'),
 version bigint NOT NULL CHECK(version>0),
 key_id text CHECK(key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 nonce bytea CHECK(octet_length(nonce) IN (12,32)),
 ciphertext bytea CHECK(octet_length(ciphertext) BETWEEN 17 AND 8192),
 state text NOT NULL DEFAULT 'PENDING' CHECK(state IN ('PENDING','LEASED','SUCCEEDED','FAILED','UNKNOWN','SUPERSEDED')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 lease_until timestamptz,
 code text CHECK(code ~ '^[a-z0-9_]{1,40}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 -- The sealed token exists exactly while the job is open.
 CHECK((state IN ('PENDING','LEASED')) = (key_id IS NOT NULL AND nonce IS NOT NULL AND ciphertext IS NOT NULL)),
 CHECK((state='LEASED') = (lease_until IS NOT NULL)),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id));
CREATE INDEX meta_unsubscribe_jobs_open ON integration.meta_unsubscribe_jobs(next_attempt_at) WHERE state IN ('PENDING','LEASED');
CREATE INDEX meta_unsubscribe_jobs_page ON integration.meta_unsubscribe_jobs(tenant_id,store_id,page_id) WHERE state='PENDING';
ALTER TABLE integration.meta_unsubscribe_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.meta_unsubscribe_jobs FORCE ROW LEVEL SECURITY;
REVOKE ALL ON integration.meta_unsubscribe_jobs FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE(state,attempts,next_attempt_at,lease_until,code,key_id,nonce,ciphertext,updated_at)
 ON integration.meta_unsubscribe_jobs TO commerce_integration_writer;
CREATE POLICY meta_unsubscribe_jobs_writer ON integration.meta_unsubscribe_jobs FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
-- The finish definer audits the outcome (fixed actions only, as 0064 claim_reply_audit).
CREATE POLICY meta_unsubscribe_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
 WITH CHECK (action IN ('meta.connect.unsubscribed','meta.connect.unsubscribe_failed','meta.connect.unsubscribe_unknown')
  AND EXISTS(SELECT 1 FROM integration.meta_unsubscribe_jobs j WHERE j.tenant_id=ops.audit_events.tenant_id AND j.store_id=ops.audit_events.store_id
   AND j.principal_id=ops.audit_events.principal_id AND j.state IN ('SUCCEEDED','FAILED','UNKNOWN') AND j.updated_at>=clock_timestamp()-interval '1 minute'));
COMMENT ON TABLE integration.meta_unsubscribe_jobs IS
 '0100 (D2): durable best-effort "DELETE /{page}/subscribed_apps" of a disconnected Page. Holds a COPY of the head sealed Page token (HPKE v2 or AES v1 bytes, never plaintext) only while PENDING/LEASED; every terminal state wipes it. Written by integration.meta_connect_disconnect/finish and integration.claim_meta_unsubscribe/finish_meta_unsubscribe only; opened only by the claims-worker (private ring).';
COMMENT ON POLICY meta_unsubscribe_jobs_writer ON integration.meta_unsubscribe_jobs IS '0100: the definer owner reads/writes jobs; the policy body is the control.';
COMMENT ON POLICY meta_unsubscribe_audit ON ops.audit_events IS '0100: the audit row of integration.finish_meta_unsubscribe (fixed actions, only right after its job reached a terminal state).';

-- ---------------------------------------------------------------------------------------
-- D1 (+ D2 supersede): meta_connect_finish. Identical to 0095 except the marked lines.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.meta_connect_finish(p_hash bytea,p_store uuid,p_state uuid,p_page text,
 p_fb_binding uuid,p_fb_expected bigint,p_fb_key text,p_fb_nonce bytea,p_fb_ct bytea,
 p_ig_binding uuid,p_ig_expected bigint,p_ig_key text,p_ig_nonce bytea,p_ig_ct bytea,
 p_app_page text,p_app_ig text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; s integration.meta_connect_states; e jsonb; v_page jsonb; v_other text; b record; v_exp timestamptz; v_n bigint;
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
 -- D2: a pending unsubscribe of THIS Page (a quick disconnect -> reconnect) must not unsubscribe the Page that is being connected again.
 UPDATE integration.meta_unsubscribe_jobs j SET state='SUPERSEDED',key_id=NULL,nonce=NULL,ciphertext=NULL,code='reconnected',updated_at=clock_timestamp()
  WHERE j.tenant_id=a.out_tenant AND j.store_id=p_store AND j.page_id=p_page AND j.state='PENDING';
 -- D1 backstop: the row is (tenant_id,store_id)-unique, so the write itself may only refresh the SAME Page (reconnect / reauth); it
 -- can never repoint a store's connection at another Page, whatever the caller's locking did.
 INSERT INTO integration.meta_connections AS mc(tenant_id,store_id,page_id,page_name,fb_binding,ig_binding,ig_id,ig_username,scopes,status,connected_by,route_expires_at)
  VALUES(a.out_tenant,p_store,p_page,coalesce(v_page->>'name',''),p_fb_binding,p_ig_binding,
   CASE WHEN p_ig_binding IS NULL THEN NULL ELSE v_page->>'ig_id' END,
   CASE WHEN p_ig_binding IS NULL THEN NULL ELSE coalesce(v_page->>'ig_username','') END,
   s.scopes_attested,'active',a.out_principal,v_exp)
  ON CONFLICT (tenant_id,store_id) DO UPDATE SET page_name=EXCLUDED.page_name,fb_binding=EXCLUDED.fb_binding,ig_binding=EXCLUDED.ig_binding,
   ig_id=EXCLUDED.ig_id,ig_username=EXCLUDED.ig_username,scopes=EXCLUDED.scopes,status='active',connected_by=EXCLUDED.connected_by,
   connected_at=clock_timestamp(),updated_at=clock_timestamp(),route_expires_at=EXCLUDED.route_expires_at
   WHERE mc.page_id=EXCLUDED.page_id;
 GET DIAGNOSTICS v_n=ROW_COUNT;
 IF v_n<>1 THEN RAISE EXCEPTION 'already_connected' USING ERRCODE='MC409'; END IF;
 UPDATE integration.meta_connect_states x SET done_at=clock_timestamp() WHERE x.id=s.id;
 RETURN jsonb_build_object('page_id',p_page,'instagram',p_ig_binding IS NOT NULL);
END $$;

COMMENT ON FUNCTION integration.meta_connect_finish(bytea,uuid,uuid,text,uuid,bigint,text,bytea,bytea,uuid,bigint,text,bytea,bytea,text,text) IS
 'integration owner; only caller internal/metaconnect.Service.Pick step 2. One transaction under a per-store advisory lock (0100 D1): sealed Page credential(s) under the 0064 head CAS, merchant routes via meta_inbox.connect_activate, the connection row (never repointed at another Page), state done, a pending unsubscribe of the same Page superseded. Receives only sealed (HPKE v2) bytes, never plaintext.';

-- ---------------------------------------------------------------------------------------
-- D2: disconnect = 0095's body + one INSERT of the unsubscribe job BEFORE the credentials are deleted.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.meta_connect_disconnect(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; c integration.meta_connections;
BEGIN
 SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
 SELECT * INTO c FROM integration.meta_connections x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='MC404'; END IF;
 -- D2: hand the head sealed Page token to the claims-worker (the only process that can open it) before it is destroyed here.
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
 DELETE FROM integration.meta_connections x WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store;
 RETURN jsonb_build_object('page_id',c.page_id,'fb_binding',c.fb_binding,'ig_binding',c.ig_binding);
END $$;
COMMENT ON FUNCTION integration.meta_connect_disconnect(bytea,uuid) IS
 'integration owner; caller internal/metaconnect.Service.Disconnect. One transaction: enqueues the Page unsubscribe job (0100 D2, a copy of the head sealed token for the claims-worker), destroys every credential head/version, disables the routes, deletes the connection row; the caller then disables the bindings.';

-- ---------------------------------------------------------------------------------------
-- D2 worker side (owner commerce_integration_writer; EXECUTE commerce_claims_worker only).
-- ---------------------------------------------------------------------------------------
-- Terminal or retry transition of a LEASED job. false = the job was not LEASED any more (another claim already closed it).
CREATE FUNCTION integration.finish_meta_unsubscribe(p_id uuid,p_outcome text,p_code text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j integration.meta_unsubscribe_jobs; v_state text;
BEGIN
 IF p_id IS NULL OR p_outcome IS NULL OR p_outcome NOT IN ('SUCCEEDED','FAILED','UNKNOWN','RETRY') OR p_code IS NULL OR p_code !~ '^[a-z0-9_]{1,40}$' THEN
  RAISE EXCEPTION 'invalid unsubscribe outcome' USING ERRCODE='22023'; END IF;
 SELECT * INTO j FROM integration.meta_unsubscribe_jobs x WHERE x.id=p_id AND x.state='LEASED' FOR UPDATE;
 IF NOT FOUND THEN RETURN false; END IF;
 IF p_outcome='RETRY' AND j.attempts<5 THEN
  UPDATE integration.meta_unsubscribe_jobs x SET state='PENDING',lease_until=NULL,code=p_code,updated_at=clock_timestamp(),
   next_attempt_at=clock_timestamp()+make_interval(secs=>30*power(2,j.attempts-1)) WHERE x.id=p_id;
  RETURN true;
 END IF;
 v_state:=CASE WHEN p_outcome='RETRY' THEN 'FAILED' ELSE p_outcome END;
 UPDATE integration.meta_unsubscribe_jobs x SET state=v_state,lease_until=NULL,key_id=NULL,nonce=NULL,ciphertext=NULL,updated_at=clock_timestamp(),
  code=CASE WHEN p_outcome='RETRY' THEN 'retries_exhausted' ELSE p_code END WHERE x.id=p_id;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(j.tenant_id,j.store_id,j.principal_id,
  CASE v_state WHEN 'SUCCEEDED' THEN 'meta.connect.unsubscribed' WHEN 'FAILED' THEN 'meta.connect.unsubscribe_failed' ELSE 'meta.connect.unsubscribe_unknown' END);
 RETURN true;
END $$;

-- Lease at most one due job (60 s). A job whose lease expired mid-call is closed UNKNOWN first: the DELETE may have reached Meta, so it is
-- never repeated. Returns zero rows when nothing is due.
CREATE FUNCTION integration.claim_meta_unsubscribe() RETURNS TABLE(o_job uuid,o_tenant uuid,o_store uuid,o_binding uuid,o_page text,
 o_version bigint,o_key_id text,o_nonce bytea,o_ciphertext bytea,o_attempt integer)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j integration.meta_unsubscribe_jobs; e uuid;
BEGIN
 FOR e IN SELECT x.id FROM integration.meta_unsubscribe_jobs x WHERE x.state='LEASED' AND x.lease_until<=clock_timestamp() FOR UPDATE SKIP LOCKED LOOP
  PERFORM integration.finish_meta_unsubscribe(e,'UNKNOWN','lease_expired');
 END LOOP;
 SELECT * INTO j FROM integration.meta_unsubscribe_jobs x WHERE x.state='PENDING' AND x.next_attempt_at<=clock_timestamp()
  ORDER BY x.next_attempt_at,x.id LIMIT 1 FOR UPDATE SKIP LOCKED;
 IF NOT FOUND THEN RETURN; END IF;
 UPDATE integration.meta_unsubscribe_jobs x SET state='LEASED',attempts=x.attempts+1,lease_until=clock_timestamp()+interval '60 seconds',updated_at=clock_timestamp()
  WHERE x.id=j.id;
 RETURN QUERY SELECT j.id,j.tenant_id,j.store_id,j.binding_id,j.page_id,j.version,j.key_id,j.nonce,j.ciphertext,j.attempts+1;
END $$;

DO $$
DECLARE f text;
BEGIN
 FOREACH f IN ARRAY ARRAY['integration.finish_meta_unsubscribe(uuid,text,text)','integration.claim_meta_unsubscribe()'] LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_integration_writer',f);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
  EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_claims_worker',f);
 END LOOP;
END $$;
COMMENT ON FUNCTION integration.finish_meta_unsubscribe(uuid,text,text) IS
 'integration owner; EXECUTE commerce_claims_worker only. Closes a LEASED unsubscribe job as SUCCEEDED/FAILED/UNKNOWN (sealed token wiped, one audit row) or schedules a bounded retry (RETRY: exponential backoff, 5 attempts then FAILED retries_exhausted). Never decides what Meta answered; the caller passes the classification.';
COMMENT ON FUNCTION integration.claim_meta_unsubscribe() IS
 'integration owner; EXECUTE commerce_claims_worker only (the only holder of the private HPKE ring). Leases one due job and returns its sealed token; an expired lease is closed UNKNOWN (never repeated).';
