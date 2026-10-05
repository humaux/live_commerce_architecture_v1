-- Purpose: live-console-v1 §2 / §2.5 / §7.4 (unit LC-B2 comment read-through, backend DeepSeek): the poller
--   lease table and print-record table plus their definers. Comment text/names are never stored; the only new
--   persistence keyed on a comment is the plain Meta comment id in live.comment_prints and in reply operations.
-- Depends on: live.claim_sources (0064), live.claim_windows (0060), live.claim_window_intervals (0064),
--   live.sessions (0033), live.offers (0060), claims.meta_intake / claims.events (0060/0064),
--   integration.bindings/operations/meta_page_heads/meta_page_credentials (0008/0064), social.comment_events
--   (0029), meta_inbox.events (0028), identity.principal_holds (0064), roles commerce_integration_writer /
--   commerce_claims_writer / commerce_meta_writer / commerce_claims_worker / commerce_runtime.
-- Used by: internal/integrations/metareply (comment_poll.go, bridge.go), internal/live/stream.go,
--   internal/httpapi/live_stream.go (routes A2/A3).
-- Invariants: I11 (no comment text/name persisted), I01 (tenant/store scope server-side), I23 (bounded pollers).
-- Status: DESIGN + MOCK (Graph-facing paths are loopback httptest only until live-console-v1 §13.3 passes).
-- Placeholder migration number 0121: the integrator renumbers it at merge (LC-B2 unit table).

-- The poller role must be able to resolve the live.* definer names below. 0096 gave it USAGE on
-- integration + claims only; it never reads the live.* tables directly (the definers do).
GRANT USAGE ON SCHEMA live TO commerce_claims_worker;

-- ---------------------------------------------------------------------------------------
-- live.comment_poll_leases (§2.2): one lease row per active claim source; at most one Graph
-- poller per source fleet-wide (I23). Written only through the acquire definer below by
-- commerce_integration_writer; the poller (commerce_claims_worker) never writes it directly.
-- ---------------------------------------------------------------------------------------
CREATE TABLE live.comment_poll_leases (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 source_id uuid NOT NULL,
 generation bigint NOT NULL CHECK (generation>0),
 holder_id text NOT NULL CHECK (holder_id ~ '^[a-z0-9_-]{1,64}$'),
 poll_epoch bigint NOT NULL DEFAULT 0 CHECK (poll_epoch>=0),
 lease_token_hash bytea NOT NULL CHECK (octet_length(lease_token_hash)=32),
 lease_until timestamptz NOT NULL,
 demand_until timestamptz NOT NULL,
 PRIMARY KEY (tenant_id,store_id,source_id),
 FOREIGN KEY (tenant_id,store_id,source_id) REFERENCES live.claim_sources(tenant_id,store_id,id) ON DELETE CASCADE
);
ALTER TABLE live.comment_poll_leases ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.comment_poll_leases FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.comment_poll_leases FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON live.comment_poll_leases TO commerce_integration_writer;
CREATE POLICY comment_poll_lease_rw ON live.comment_poll_leases FOR ALL TO commerce_integration_writer
 USING (true) WITH CHECK (true);
-- live.comment_poll_sources (owner commerce_claims_writer) reads only the demand column; the write
-- path stays commerce_integration_writer-only.
GRANT SELECT(tenant_id,store_id,source_id,demand_until) ON live.comment_poll_leases TO commerce_claims_writer;
CREATE POLICY comment_poll_lease_read ON live.comment_poll_leases FOR SELECT TO commerce_claims_writer USING (true);

-- ---------------------------------------------------------------------------------------
-- live.comment_prints (§7.4): the fact of printing a comment label, never the label content.
-- Retention class C3 (intake_days); the retention amendment adds the DELETE row (§10).
-- ---------------------------------------------------------------------------------------
CREATE TABLE live.comment_prints (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 session_id uuid NOT NULL,
 comment_ref text NOT NULL CHECK (comment_ref ~ '^[0-9_]{1,80}$'),
 print_count bigint NOT NULL DEFAULT 1 CHECK (print_count>=1),
 first_printed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 last_printed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 last_principal_id uuid NOT NULL,
 PRIMARY KEY (tenant_id,store_id,session_id,comment_ref),
 FOREIGN KEY (tenant_id,store_id,session_id) REFERENCES live.sessions(tenant_id,store_id,id),
 FOREIGN KEY (tenant_id,last_principal_id) REFERENCES identity.memberships(tenant_id,principal_id)
);
ALTER TABLE live.comment_prints ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.comment_prints FORCE ROW LEVEL SECURITY;
REVOKE ALL ON live.comment_prints FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON live.comment_prints TO commerce_claims_writer;
CREATE POLICY comment_print_writer ON live.comment_prints FOR ALL TO commerce_claims_writer
 USING (true) WITH CHECK (true);

-- ---------------------------------------------------------------------------------------
-- live.acquire_comment_poll_lease (§2.2): lease take-over / renewal, one row per source.
-- generation increments on every take-over (new holder or expired lease) and fences
-- integration.load_meta_page_token_for_poll; poll_epoch increments only when the caller starts a
-- fresh empty ring buffer (p_new_buffer=true), so a real buffer loss surfaces as reset:true and a
-- same-holder renewal keeps the epoch (seq stays monotonic). Never writes comment text.
-- ---------------------------------------------------------------------------------------
-- The poller runs outside any merchant transaction (no app.tenant_id/store_id/session_id GUCs), so the
-- 0064 source_reply_read policy (intake_scope) can never match. Give the integration owner the same
-- no-GUC "system read" the codebase already grants commerce_claims_writer for windows/offers/links; the
-- bodies below still fence tenant/store/source/active themselves.
CREATE POLICY source_poll_read ON live.claim_sources FOR SELECT TO commerce_integration_writer
 USING (nullif(current_setting('app.tenant_id',true),'') IS NULL
  AND nullif(current_setting('app.store_id',true),'') IS NULL
  AND nullif(current_setting('app.principal_id',true),'') IS NULL
  AND nullif(current_setting('app.buyer_id',true),'') IS NULL);

CREATE FUNCTION live.acquire_comment_poll_lease(p_tenant uuid,p_store uuid,p_source uuid,p_holder text,
 p_lease_token_hash bytea,p_lease_until timestamptz,p_demand_until timestamptz,p_new_buffer boolean)
RETURNS TABLE(generation bigint,poll_epoch bigint,acquired boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v record;
BEGIN
 IF p_tenant IS NULL OR p_store IS NULL OR p_source IS NULL OR p_holder IS NULL OR p_holder !~ '^[a-z0-9_-]{1,64}$'
  OR p_lease_token_hash IS NULL OR octet_length(p_lease_token_hash)<>32
  OR p_lease_until IS NULL OR p_lease_until<=clock_timestamp()
  OR p_demand_until IS NULL OR p_new_buffer IS NULL THEN
  RAISE EXCEPTION 'invalid comment poll lease' USING ERRCODE='22023';
 END IF;
 -- The source must exist and belong to this tenant/store (fence: the poller never trusts a source id alone).
 IF NOT EXISTS(SELECT 1 FROM live.claim_sources s
   WHERE s.tenant_id=p_tenant AND s.store_id=p_store AND s.id=p_source AND s.active) THEN
  RAISE EXCEPTION 'comment poll source mismatch' USING ERRCODE='PT409';
 END IF;
 SELECT x.generation,x.poll_epoch,x.holder_id,x.lease_until INTO v
  FROM live.comment_poll_leases x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.source_id=p_source FOR UPDATE;
 IF NOT FOUND THEN
  INSERT INTO live.comment_poll_leases(tenant_id,store_id,source_id,generation,holder_id,poll_epoch,
   lease_token_hash,lease_until,demand_until)
  VALUES(p_tenant,p_store,p_source,1,p_holder,CASE WHEN p_new_buffer THEN 1 ELSE 0 END,
   p_lease_token_hash,p_lease_until,p_demand_until);
  RETURN QUERY SELECT 1::bigint, CASE WHEN p_new_buffer THEN 1::bigint ELSE 0::bigint END, true;
 ELSIF v.holder_id=p_holder AND v.lease_until>clock_timestamp() THEN
  -- Same holder, unexpired: renew without bumping generation/poll_epoch (seq stays monotonic).
  UPDATE live.comment_poll_leases x SET lease_until=p_lease_until,
   demand_until=GREATEST(x.demand_until,p_demand_until)
   WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.source_id=p_source;
  RETURN QUERY SELECT v.generation, v.poll_epoch, true;
 ELSIF v.lease_until>clock_timestamp() THEN
  -- Another holder owns an unexpired lease: this replica must not take over (I23: at most one
  -- Graph poller per source fleet-wide). acquired=false leaves the incumbent running.
  RETURN QUERY SELECT v.generation, v.poll_epoch, false;
 ELSE
  -- Take-over of an expired lease: new empty buffer (process memory) and a new generation for the token fence.
  UPDATE live.comment_poll_leases x SET generation=x.generation+1,
   poll_epoch=CASE WHEN p_new_buffer THEN x.poll_epoch+1 ELSE x.poll_epoch END,
   holder_id=p_holder, lease_token_hash=p_lease_token_hash, lease_until=p_lease_until,
   demand_until=p_demand_until
   WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.source_id=p_source
   RETURNING x.generation, x.poll_epoch INTO v.generation, v.poll_epoch;
  RETURN QUERY SELECT v.generation, v.poll_epoch, true;
 END IF;
END $$;
ALTER FUNCTION live.acquire_comment_poll_lease(uuid,uuid,uuid,text,bytea,timestamptz,timestamptz,boolean)
 OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION live.acquire_comment_poll_lease(uuid,uuid,uuid,text,bytea,timestamptz,timestamptz,boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.acquire_comment_poll_lease(uuid,uuid,uuid,text,bytea,timestamptz,timestamptz,boolean)
 TO commerce_claims_worker;
COMMENT ON FUNCTION live.acquire_comment_poll_lease(uuid,uuid,uuid,text,bytea,timestamptz,timestamptz,boolean) IS
 'integration owner; only caller internal/integrations/metareply comment poller (commerce_claims_worker). One lease row per source; take-over bumps generation and, on p_new_buffer, poll_epoch; renewal keeps both. Never comment text.';

-- ---------------------------------------------------------------------------------------
-- live.comment_poll_sources (§2.2): the candidate sources a poller must serve — every active
-- claim source whose session window is OPEN or whose lease still has demand (a console is asking).
-- Draft/archived sessions have neither, so they are never polled (the 0120 lifecycle column, LC-B1,
-- is folded into window-state+demand here). No session, no side effect.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION live.comment_poll_sources()
RETURNS TABLE(tenant_id uuid,store_id uuid,session_id uuid,source_id uuid,platform text,object text,
 asset_id text,source_object_id text,binding_id uuid,binding_version bigint,window_open boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT s.tenant_id,s.store_id,s.session_id,s.id,s.platform,s.object,s.asset_id,s.source_object_id,
  s.binding_id,s.binding_version,
  EXISTS(SELECT 1 FROM live.claim_windows w
   WHERE w.tenant_id=s.tenant_id AND w.store_id=s.store_id AND w.session_id=s.session_id AND w.state='OPEN')
 FROM live.claim_sources s
 WHERE s.active
   AND (EXISTS(SELECT 1 FROM live.claim_windows w
         WHERE w.tenant_id=s.tenant_id AND w.store_id=s.store_id AND w.session_id=s.session_id AND w.state='OPEN')
     OR EXISTS(SELECT 1 FROM live.comment_poll_leases l
         WHERE l.tenant_id=s.tenant_id AND l.store_id=s.store_id AND l.source_id=s.id
           AND l.demand_until>clock_timestamp()));
$$;
ALTER FUNCTION live.comment_poll_sources() OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION live.comment_poll_sources() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.comment_poll_sources() TO commerce_claims_worker;
COMMENT ON FUNCTION live.comment_poll_sources() IS
 'internal/live (SQL definer; only caller the comment poller commerce_claims_worker). Active sources with an OPEN window or live demand; never draft/archived (no OPEN window, no demand). No comment text.';

-- ---------------------------------------------------------------------------------------
-- live.console_source (§2.3): the bridge re-check before it touches a ring buffer. Zero rows on a
-- tenant/store/session/source mismatch → the bridge answers 404. No permission check here: the API
-- already authorized live:read for the store (I01); this only pins the tuple.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION live.console_source(p_tenant uuid,p_store uuid,p_session uuid,p_source uuid)
RETURNS TABLE(tenant_id uuid,store_id uuid,session_id uuid,source_id uuid,platform text,object text,
 asset_id text,source_object_id text,active boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT s.tenant_id,s.store_id,s.session_id,s.id,s.platform,s.object,s.asset_id,s.source_object_id,s.active
 FROM live.claim_sources s
 WHERE s.tenant_id=p_tenant AND s.store_id=p_store AND s.session_id=p_session AND s.id=p_source AND s.active;
$$;
ALTER FUNCTION live.console_source(uuid,uuid,uuid,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION live.console_source(uuid,uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.console_source(uuid,uuid,uuid,uuid) TO commerce_claims_worker;
COMMENT ON FUNCTION live.console_source(uuid,uuid,uuid,uuid) IS
 'internal/live (SQL definer; only caller the bridge commerce_claims_worker). Pins the (tenant,store,session,source) tuple to one active claim source; zero rows = mismatch (404). No comment text.';

-- ---------------------------------------------------------------------------------------
-- integration.load_meta_page_token_for_poll (§2.2): the poll-lease-fenced clone of
-- load_meta_page_token. Returns the current head credential of the source's binding only while the
-- lease is held at the given generation; zero rows when the binding is disabled/re-pointed or the
-- credential head is gone (→ the poller stops with stream.state=unavailable). Never plaintext.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.load_meta_page_token_for_poll(p_source uuid,p_generation bigint,p_lease_token bytea)
RETURNS TABLE(tenant_id uuid,store_id uuid,binding_id uuid,provider text,asset_id text,version bigint,
 key_id text,nonce bytea,ciphertext bytea,scopes_attested text[])
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; l record; b record; v_head bigint;
BEGIN
 IF p_source IS NULL OR p_generation IS NULL OR p_generation<1 OR p_lease_token IS NULL OR octet_length(p_lease_token)<>32 THEN
  RAISE EXCEPTION 'invalid poll credential load' USING ERRCODE='22023';
 END IF;
 SELECT x.tenant_id,x.store_id,x.id,x.binding_id,x.binding_version,x.object,x.asset_id INTO s
  FROM live.claim_sources x WHERE x.id=p_source AND x.active;
 IF NOT FOUND THEN RAISE EXCEPTION 'poll credential source unavailable' USING ERRCODE='P0002'; END IF;
 SELECT y.generation,y.holder_id,y.lease_until,y.lease_token_hash INTO l
  FROM live.comment_poll_leases y WHERE y.tenant_id=s.tenant_id AND y.store_id=s.store_id AND y.source_id=s.id
  FOR SHARE;
 IF NOT FOUND OR l.generation<>p_generation OR l.lease_until IS NULL OR l.lease_until<=clock_timestamp()
  OR l.lease_token_hash IS DISTINCT FROM sha256(p_lease_token) THEN
  RAISE EXCEPTION 'poll credential lease conflict' USING ERRCODE='40001';
 END IF;
 -- Binding identity + enabled gate (a re-pointed or disabled binding stops the poller within one renewal).
 SELECT z.id,z.provider,z.external_asset_id,z.enabled INTO b FROM integration.bindings z WHERE z.id=s.binding_id FOR SHARE;
 IF NOT FOUND OR b.id IS NULL OR NOT b.enabled OR b.provider<>(CASE s.object WHEN 'page' THEN 'facebook' ELSE 'instagram' END)
  OR b.external_asset_id<>s.asset_id THEN
  RETURN;
 END IF;
 SELECT h.current_version INTO v_head FROM integration.meta_page_heads h
  WHERE h.tenant_id=s.tenant_id AND h.store_id=s.store_id AND h.binding_id=s.binding_id;
 IF NOT FOUND THEN RETURN; END IF;
 IF l.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'poll credential lease conflict' USING ERRCODE='40001'; END IF;
 RETURN QUERY SELECT c.tenant_id,c.store_id,c.binding_id,c.provider,c.asset_id,c.version,c.key_id,c.nonce,c.ciphertext,c.scopes_attested
  FROM integration.meta_page_credentials c WHERE c.tenant_id=s.tenant_id AND c.store_id=s.store_id
   AND c.binding_id=s.binding_id AND c.version=v_head;
END $$;
ALTER FUNCTION integration.load_meta_page_token_for_poll(uuid,bigint,bytea) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.load_meta_page_token_for_poll(uuid,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.load_meta_page_token_for_poll(uuid,bigint,bytea) TO commerce_claims_worker;
COMMENT ON FUNCTION integration.load_meta_page_token_for_poll(uuid,bigint,bytea) IS
 'integration owner; only caller the comment poller (commerce_claims_worker). Lease-fenced like load_meta_page_token but on live.comment_poll_leases: returns the current head credential of the source''s binding at the held generation; zero rows = disabled/re-pointed/no credential; never plaintext.';

-- ---------------------------------------------------------------------------------------
-- social.read_comment_events (§2.4 OPEN-4 fallback): the console's IG-live read from the existing
-- encrypted webhook copy, no new storage. Returns instagram_live_comment/instagram_comment events of
-- the session's active IG source received since the session's first window opened_at. The API decrypts
-- with the payload keyring and discards events whose media id differs from source_object_id (the media
-- id is only inside the ciphertext). seq is a received_at microsecond proxy (comment_events has no
-- server_seq); the caller dedupes on event_id.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION social.read_comment_events(p_session uuid,p_after_seq bigint,p_limit int)
RETURNS TABLE(event_id uuid,comment_key text,kind text,occurred_at timestamptz,received_at timestamptz,
 key_id text,nonce bytea,ciphertext bytea,app_id text,object text,asset_id text,event_key text,
 payload_hash text,route_id uuid,route_epoch bigint,seq bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_principal uuid; v_first timestamptz;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.principal_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL
  OR p_session IS NULL OR p_after_seq IS NULL OR p_after_seq<0 OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 100 THEN
  RAISE EXCEPTION 'invalid comment events read' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 v_principal:=current_setting('app.principal_id')::uuid;
 IF NOT identity.principal_holds(v_tenant,v_store,v_principal,ARRAY['live:read']) THEN
  RAISE EXCEPTION 'comment events access unavailable' USING ERRCODE='PT403';
 END IF;
 -- Session must belong to this store and own an active IG source, else zero rows.
 IF NOT EXISTS(SELECT 1 FROM live.claim_sources x
   WHERE x.tenant_id=v_tenant AND x.store_id=v_store AND x.session_id=p_session
    AND x.object='instagram' AND x.active) THEN
  RETURN;
 END IF;
 SELECT min(i.opened_at) INTO v_first FROM live.claim_window_intervals i
  WHERE i.tenant_id=v_tenant AND i.store_id=v_store AND i.session_id=p_session;
 IF v_first IS NULL THEN RETURN; END IF;
 RETURN QUERY
 SELECT c.event_id,c.comment_key,c.kind,c.occurred_at,c.received_at,c.key_id,c.nonce,c.ciphertext,
  e.app_id,e.object,e.asset_id,e.event_key,e.payload_hash,e.route_id,e.route_epoch,
  (extract(epoch FROM c.received_at)*1000000)::bigint
 FROM social.comment_events c JOIN meta_inbox.events e ON e.id=c.event_id AND e.tenant_id=c.tenant_id AND e.store_id=c.store_id
 WHERE c.tenant_id=v_tenant AND c.store_id=v_store
  AND c.kind IN ('instagram_live_comment','instagram_comment')
  AND c.received_at>=v_first
  AND (extract(epoch FROM c.received_at)*1000000)::bigint>p_after_seq
 ORDER BY c.received_at, c.event_id
 LIMIT p_limit;
END $$;
ALTER FUNCTION social.read_comment_events(uuid,bigint,int) OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION social.read_comment_events(uuid,bigint,int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION social.read_comment_events(uuid,bigint,int) TO commerce_runtime;
COMMENT ON FUNCTION social.read_comment_events(uuid,bigint,int) IS
 'social owner; only caller internal/live (commerce_runtime, merchant transaction). IG-live fallback read of the encrypted webhook copy for one session''s active IG source since its first window opened_at; live:read via principal_holds. Envelope + AAD only, never plaintext.';

-- Additive cross-domain reads for social.read_comment_events (commerce_meta_writer has none today).
GRANT SELECT(tenant_id,store_id,id) ON live.sessions TO commerce_meta_writer;
CREATE POLICY session_meta_read ON live.sessions FOR SELECT TO commerce_meta_writer USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT(tenant_id,store_id,session_id,object,active) ON live.claim_sources TO commerce_meta_writer;
CREATE POLICY claim_source_meta_read ON live.claim_sources FOR SELECT TO commerce_meta_writer USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT(tenant_id,store_id,session_id,opened_at) ON live.claim_window_intervals TO commerce_meta_writer;
CREATE POLICY window_interval_meta_read ON live.claim_window_intervals FOR SELECT TO commerce_meta_writer USING
 (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT EXECUTE ON FUNCTION identity.principal_holds(uuid,uuid,uuid,text[]) TO commerce_meta_writer;

-- ---------------------------------------------------------------------------------------
-- live.console_marks (§2.5): join one comment page's refs to claims/replies/prints with no text
-- join. Every ref is resolved through the session's claim_sources (object, asset_id), never by
-- comment_ref alone. Nullable fields; LC-B4 (0123) adds the public/private-reply expression index.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION live.console_marks(p_session uuid,p_refs text[])
RETURNS TABLE(ref text,intake_state text,intake_drop_reason text,claim_outcome text,claim_reason text,
 offer_id uuid,keyword text,quantity integer,bundle_id uuid,private_reply_kind text,private_reply_state text,
 private_reply_blocked_reason text,public_replies bigint,printed_count bigint,printed_last_at timestamptz,
 private_reply_available boolean,private_reply_unavailable_reason text)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_principal uuid; v_src record; v_ref text;
 v_intake record; v_claim record; v_reply record; v_printed record; v_public bigint;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.principal_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL
  OR p_session IS NULL OR p_refs IS NULL OR array_ndims(p_refs)<>1 OR cardinality(p_refs) NOT BETWEEN 1 AND 100
  OR EXISTS(SELECT 1 FROM unnest(p_refs) r WHERE r IS NULL OR r !~ '^[0-9_]{1,80}$') THEN
  RAISE EXCEPTION 'invalid console marks' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 v_principal:=current_setting('app.principal_id')::uuid;
 IF NOT identity.principal_holds(v_tenant,v_store,v_principal,ARRAY['live:read']) THEN
  RAISE EXCEPTION 'console marks access unavailable' USING ERRCODE='PT403';
 END IF;
 -- The session must own at least one active source in this store (else the page would be empty anyway).
 SELECT s.object,s.asset_id INTO v_src FROM live.claim_sources s
  WHERE s.tenant_id=v_tenant AND s.store_id=v_store AND s.session_id=p_session AND s.active LIMIT 1;
 IF NOT FOUND THEN RETURN; END IF;
 FOREACH v_ref IN ARRAY p_refs LOOP
  v_intake:=NULL; v_claim:=NULL; v_reply:=NULL; v_printed:=NULL; v_public:=0;
  SELECT i.state,i.drop_reason,i.applied_event_id INTO v_intake FROM claims.meta_intake i
   WHERE i.object=v_src.object AND i.asset_id=v_src.asset_id AND i.comment_ref=v_ref;
  -- Always assign (a no-row SELECT INTO sets v_claim to an assigned NULL record); the
  -- conditional assignment would leave it "not assigned yet" when there is no applied event.
  SELECT e.outcome,e.reason,e.offer_id,e.quantity,e.bundle_id INTO v_claim
   FROM claims.events e
   WHERE e.tenant_id=v_tenant AND e.store_id=v_store AND e.id=v_intake.applied_event_id
    AND v_intake.applied_event_id IS NOT NULL;
  SELECT o.state, o.request->>'message_type' AS message_type, o.result_code INTO v_reply
   FROM integration.operations o
   WHERE o.tenant_id=v_tenant AND o.store_id=v_store AND o.action='meta.private_reply'
    AND o.semantic_key IN (
      'mpr:'||substr(encode(sha256(convert_to(v_src.object||'|'||v_src.asset_id||'|'||v_ref,'UTF8')),'hex'),1,48),
      'mpr:'||substr(encode(sha256(convert_to(v_src.object||'|'||v_src.asset_id||'|'||v_ref,'UTF8')),'hex'),1,48)||':m1')
   ORDER BY o.created_at DESC LIMIT 1;
  SELECT count(*) INTO v_public FROM integration.operations o
   WHERE o.tenant_id=v_tenant AND o.store_id=v_store AND o.action='meta.public_reply'
    AND o.request->>'comment_ref'=v_ref;
  SELECT p.print_count,p.last_printed_at INTO v_printed FROM live.comment_prints p
   WHERE p.tenant_id=v_tenant AND p.store_id=v_store AND p.session_id=p_session AND p.comment_ref=v_ref;
  RETURN QUERY SELECT
   v_ref,
   v_intake.state, v_intake.drop_reason,
   v_claim.outcome, v_claim.reason, v_claim.offer_id,
   (SELECT k.keyword FROM live.offers k WHERE k.tenant_id=v_tenant AND k.store_id=v_store AND k.id=v_claim.offer_id),
   v_claim.quantity, v_claim.bundle_id,
   CASE WHEN v_reply.message_type='manual_private_reply' THEN 'manual'
        WHEN v_reply.message_type='out_of_stock_reply' THEN 'out_of_stock'
        ELSE 'auto' END,
   v_reply.state, CASE WHEN v_reply.state IN ('FAILED_FINAL','BLOCKED_POLICY') THEN v_reply.result_code ELSE NULL END,
   v_public,
   COALESCE(v_printed.print_count,0), v_printed.last_printed_at,
   CASE WHEN v_reply.state IS NULL THEN true
        WHEN v_reply.state IN ('SUCCEEDED','ACKNOWLEDGED') THEN false
        WHEN v_reply.state IN ('READY','DISPATCHING','UNKNOWN') THEN false
        ELSE true END,
   CASE WHEN v_reply.state IS NULL THEN NULL
        WHEN v_reply.state IN ('SUCCEEDED','ACKNOWLEDGED') THEN 'used'
        WHEN v_reply.state IN ('READY','DISPATCHING','UNKNOWN') THEN 'auto_pending'
        ELSE NULL END;
 END LOOP;
END $$;
ALTER FUNCTION live.console_marks(uuid,text[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION live.console_marks(uuid,text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.console_marks(uuid,text[]) TO commerce_runtime;
COMMENT ON FUNCTION live.console_marks(uuid,text[]) IS
 'internal/live (SQL definer; only caller the console comment page commerce_runtime). Joins comment refs to intake/claim/private-reply/public-reply/print facts through the session''s claim_sources, never by ref alone and never with comment text. live:read via principal_holds.';

-- Additive column reads for live.console_marks (reason/offer_id/quantity on claims.events and
-- semantic_key/result_code/created_at on integration.operations are not in the 0064 column grants).
GRANT SELECT(reason,offer_id,quantity) ON claims.events TO commerce_claims_writer;
GRANT SELECT(semantic_key,result_code,created_at) ON integration.operations TO commerce_claims_writer;

-- ---------------------------------------------------------------------------------------
-- live.comment_print (§7.4 A3): idempotent-per-key print fact upsert. The browser renders the label
-- from the comment it already holds; the server stores only the fact. No text, no idempotency replay
-- beyond the row itself (the row IS the record).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION live.comment_print(p_session uuid,p_comment_ref text,p_principal uuid)
RETURNS TABLE(print_count bigint,last_printed_at timestamptz)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid; v_count bigint; v_last timestamptz;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR coalesce(current_setting('app.tenant_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR coalesce(current_setting('app.store_id',true),'') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL
  OR p_session IS NULL OR p_comment_ref IS NULL OR p_comment_ref !~ '^[0-9_]{1,80}$'
  OR p_principal IS NULL THEN
  RAISE EXCEPTION 'invalid comment print' USING ERRCODE='22023';
 END IF;
 v_tenant:=current_setting('app.tenant_id')::uuid;
 v_store:=current_setting('app.store_id')::uuid;
 IF NOT identity.principal_holds(v_tenant,v_store,p_principal,ARRAY['live:manage']) THEN
  RAISE EXCEPTION 'comment print access unavailable' USING ERRCODE='PT403';
 END IF;
 -- The session existence fence is the FK to live.sessions(tenant_id,store_id,id): a foreign or
 -- cross-store session id raises 23503 → the API maps it to 404. The comment_ref itself is an
 -- opaque Meta comment id (never verifiable server-side), so only its shape is checked above.
 INSERT INTO live.comment_prints(tenant_id,store_id,session_id,comment_ref,print_count,last_principal_id)
 VALUES(v_tenant,v_store,p_session,p_comment_ref,1,p_principal)
 ON CONFLICT (tenant_id,store_id,session_id,comment_ref)
 DO UPDATE SET print_count=live.comment_prints.print_count+1, last_printed_at=clock_timestamp(),
  last_principal_id=p_principal
 RETURNING live.comment_prints.print_count, live.comment_prints.last_printed_at INTO v_count, v_last;
 RETURN QUERY SELECT v_count, v_last;
END $$;
ALTER FUNCTION live.comment_print(uuid,text,uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION live.comment_print(uuid,text,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.comment_print(uuid,text,uuid) TO commerce_runtime;
COMMENT ON FUNCTION live.comment_print(uuid,text,uuid) IS
 'internal/live (SQL definer; only caller route A3 commerce_runtime). Idempotent print-fact upsert; live:manage via principal_holds; the comment ref must belong to an active source of the session. No label text is stored.';
