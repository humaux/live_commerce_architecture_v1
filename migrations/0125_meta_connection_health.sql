-- 0125_meta_connection_health.sql — W1-01B backend (contracts/meta-connection-health-v1.md, DRAFT 2026-10-05).
-- Purpose: the connection-health store for Meta Page/IG connections — integration.binding_capabilities (the probed
--   capability table the LC-B3 reader swaps to), integration.meta_health_probes (lease-fenced probe sweep state),
--   notify.merchant_alerts, the snapshot/claim/record/recheck definers and the binding_capability_state reader.
-- Depends on: 0001 (ops.audit_events, control.stores, identity.store_grants/store_staff/memberships), 0003 (identity.resolve_access),
--   0008 (integration.bindings/operations, commerce_integration_writer), 0064 (integration.meta_page_heads/credentials, commerce_integration_writer
--   USAGE on integration/control/ops/identity), 0073/0074 (resolve_access EXECUTE), 0090 (notify.outbox/claim_batch/record_result,
--   identity.staff_password_email EXECUTE commerce_checkout_writer, notify.store_settings), 0095/0100/0108 (integration.meta_connections
--   CURRENT PK (tenant_id,store_id,page_id), meta_connect_auth, page-token custody grant pattern), 0119 (integration.meta_resubscribe_jobs).
-- Used by: internal/metaconnect (Derive/SnapshotReader/TableReader/Health), internal/integrations/metareply/probe.go (the probe sweep),
--   internal/notify (merchant meta_health alerts), internal/httpapi/meta_health.go (B1/B2). File name is the placeholder 0125; the integrator
--   renumbers (the contract's 0127 is advisory — see DELIVERY.md).
-- Invariants: every new object owned by commerce_integration_writer (notify.* by commerce_checkout_writer, 0090 pattern), SECURITY DEFINER
--   SET search_path=pg_catalog, REVOKE ALL FROM PUBLIC, READ COMMITTED asserted on writers; FORCE RLS with the definer owner = control;
--   capability rows hold ids/codes/timestamps only (never a token, scopes dump or Graph body); the probe's claim is lease-fenced
--   (60 s, generation+1); record_meta_health is a lease-fenced no-op when the lease is stale or the connection vanished.
-- Status: MOCK (REAL_PG + fake Graph; LIVE probe is MCH12, NOT_RUN).
-- Privilege-delta note (§8 vs 0096): the contract lists `commerce_worker` alongside `commerce_claims_worker` for
--   report_capability_failure / mark_capability_evidence / binding_capability_state, but migration 0096 retired the shared
--   `commerce_worker` role (empty, no login may join it). These three therefore grant EXECUTE to `commerce_claims_worker`
--   only — the send/read adapters (LC-B2/B4 private-reply route) run under that authority. The integrator must reflect
--   this in the MCI02/KC03 expected privilege rows (MCH10).

-- ---------------------------------------------------------------------------------------
-- §2 integration.binding_capabilities: one row per (binding, capability), 4 per binding.
-- ---------------------------------------------------------------------------------------
CREATE TABLE integration.binding_capabilities (
    tenant_id  uuid NOT NULL,
    store_id   uuid NOT NULL,
    page_id    text NOT NULL CHECK (page_id ~ '^[0-9]{1,40}$'),
    binding_id uuid NOT NULL,
    provider   text NOT NULL CHECK (provider IN ('facebook','instagram')),
    capability text NOT NULL CHECK (capability IN ('read_comment','private_reply','dm_session','reply_public')),
    state      text NOT NULL CHECK (state IN ('ok','missing_permission','missing_task','not_subscribed','reauth_required','review_required','unsupported','unknown')),
    reason     text NOT NULL CHECK (reason ~ '^[a-z0-9_]{1,40}$'),
    evidence   text NOT NULL DEFAULT 'DESIGN' CHECK (evidence IN ('DESIGN','MOCK','LIVE_READ','LIVE_SEND')),
    checked_at timestamptz,
    PRIMARY KEY (tenant_id, store_id, binding_id, capability),
    FOREIGN KEY (tenant_id, store_id, page_id) REFERENCES integration.meta_connections(tenant_id, store_id, page_id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, store_id, binding_id) REFERENCES integration.bindings(tenant_id, store_id, id) ON DELETE CASCADE
);
ALTER TABLE integration.binding_capabilities ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.binding_capabilities FORCE ROW LEVEL SECURITY;
REVOKE ALL ON integration.binding_capabilities FROM PUBLIC;

-- ---------------------------------------------------------------------------------------
-- §4-5 integration.meta_health_probes: the per-Page schedule/severity/episode row.
-- ---------------------------------------------------------------------------------------
CREATE TABLE integration.meta_health_probes (
    tenant_id            uuid NOT NULL,
    store_id             uuid NOT NULL,
    page_id              text NOT NULL CHECK (page_id ~ '^[0-9]{1,40}$'),
    next_due_at          timestamptz NOT NULL DEFAULT clock_timestamp(),
    lease_token          bytea CHECK (lease_token IS NULL OR octet_length(lease_token)=32),
    lease_until          timestamptz,
    generation           bigint NOT NULL DEFAULT 0 CHECK (generation >= 0),
    consecutive_failures smallint NOT NULL DEFAULT 0 CHECK (consecutive_failures BETWEEN 0 AND 32767),
    last_outcome         text CHECK (last_outcome IN ('probed','rate_limited','unknown')),
    last_checked_at      timestamptz,
    perm_source          text CHECK (perm_source IN ('graph','snapshot')),
    severity             text NOT NULL DEFAULT 'none' CHECK (severity IN ('none','warning','blocking')),
    episode              bigint NOT NULL DEFAULT 0 CHECK (episode >= 0),
    episode_opened_at    timestamptz,
    episode_closed_at    timestamptz,
    last_mail_at         timestamptz,
    PRIMARY KEY (tenant_id, store_id, page_id),
    FOREIGN KEY (tenant_id, store_id, page_id) REFERENCES integration.meta_connections(tenant_id, store_id, page_id) ON DELETE CASCADE
);
ALTER TABLE integration.meta_health_probes ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.meta_health_probes FORCE ROW LEVEL SECURITY;
REVOKE ALL ON integration.meta_health_probes FROM PUBLIC;

-- ---------------------------------------------------------------------------------------
-- §4.1/§6.2 trigger: a connection INSERT or a reconnect (status back to active, scopes or bindings changed) resets the
-- probe (due now) and clears every probed capability state (the reader falls back to the snapshot until the next sweep
-- re-probes, <=5 min). Deleting (not marking) is what makes §3.4 hold: the probe owns evidence on row creation. The
-- reauth flip (active -> reauth_required) is NOT a reset: record_meta_health wrote those rows itself.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.meta_health_on_connection() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
    IF TG_OP='INSERT' OR NEW.status='active' OR NEW.scopes IS DISTINCT FROM OLD.scopes
       OR NEW.fb_binding IS DISTINCT FROM OLD.fb_binding OR NEW.ig_binding IS DISTINCT FROM OLD.ig_binding THEN
        INSERT INTO integration.meta_health_probes(tenant_id,store_id,page_id,next_due_at)
            VALUES(NEW.tenant_id,NEW.store_id,NEW.page_id,clock_timestamp())
            ON CONFLICT (tenant_id,store_id,page_id) DO UPDATE SET next_due_at=clock_timestamp();
        DELETE FROM integration.binding_capabilities x WHERE x.tenant_id=NEW.tenant_id AND x.store_id=NEW.store_id AND x.page_id=NEW.page_id;
    END IF;
    RETURN NULL;
END $$;

CREATE TRIGGER meta_health_on_connection
    AFTER INSERT OR UPDATE OF status, scopes, fb_binding, ig_binding ON integration.meta_connections
    FOR EACH ROW EXECUTE FUNCTION integration.meta_health_on_connection();

-- ---------------------------------------------------------------------------------------
-- §7.2 read model: the only commerce_runtime read of meta_connections (it has no RLS SELECT policy). GUC-scoped; the
-- GUCs are set by platform.WithScope before the read. o_messages = the 0122 resubscribe SUCCEEDED for that Page.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.meta_health_snapshot()
RETURNS TABLE(o_page text, o_page_name text, o_status text, o_fb uuid, o_ig uuid, o_scopes text[], o_messages boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
    SELECT c.page_id, c.page_name, c.status, c.fb_binding, c.ig_binding, c.scopes,
           EXISTS(SELECT 1 FROM integration.meta_resubscribe_jobs j
                  WHERE j.tenant_id=c.tenant_id AND j.store_id=c.store_id AND j.page_id=c.page_id AND j.state='SUCCEEDED')
    FROM integration.meta_connections c
    WHERE c.tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
      AND c.store_id=nullif(current_setting('app.store_id',true),'')::uuid
    ORDER BY c.page_id $$;

-- ---------------------------------------------------------------------------------------
-- §4.1 claim: lease up to p_limit due probes (60 s, generation+1, next_due_at=lease so a dead probe is re-claimed exactly
-- when its lease expires), returning per row the FB binding's current head credential (LEFT JOIN so a missing head still
-- yields the row with NULL credential fields, which the probe records as an unknown outcome). The ciphertext leaves SQL
-- only to commerce_claims_worker through this definer (0100 pattern).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.claim_meta_health_probes(p_limit integer)
RETURNS TABLE(o_tenant uuid, o_store uuid, o_page text, o_generation bigint, o_lease bytea, o_fb uuid, o_ig uuid,
              o_ig_id text, o_scopes text[], o_version bigint, o_key_id text, o_nonce bytea, o_ciphertext bytea)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE r record; v_until timestamptz; v_gen bigint; v_token bytea;
BEGIN
    IF p_limit IS NULL OR p_limit<1 OR p_limit>50 THEN RAISE EXCEPTION 'invalid probe limit' USING ERRCODE='22023'; END IF;
    IF current_setting('transaction_isolation',true) IS DISTINCT FROM 'read committed' THEN
        RAISE EXCEPTION 'read committed required' USING ERRCODE='25001'; END IF;
    FOR r IN SELECT x.tenant_id,x.store_id,x.page_id FROM integration.meta_health_probes x
             WHERE x.next_due_at<=clock_timestamp() ORDER BY x.next_due_at,x.page_id LIMIT p_limit FOR UPDATE SKIP LOCKED LOOP
        v_until:=clock_timestamp()+interval '60 seconds';
        UPDATE integration.meta_health_probes x
           SET generation=x.generation+1, lease_token=gen_random_bytes(32), lease_until=v_until, next_due_at=v_until
         WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=r.page_id
         RETURNING x.generation,x.lease_token INTO v_gen,v_token;
        RETURN QUERY
            SELECT c.tenant_id,c.store_id,c.page_id,v_gen,v_token,c.fb_binding,c.ig_binding,c.ig_id,c.scopes,
                   k.version,k.key_id,k.nonce,k.ciphertext
            FROM integration.meta_connections c
            LEFT JOIN integration.meta_page_heads h ON h.tenant_id=c.tenant_id AND h.store_id=c.store_id AND h.binding_id=c.fb_binding
            LEFT JOIN integration.meta_page_credentials k ON k.tenant_id=h.tenant_id AND k.store_id=h.store_id
                 AND k.binding_id=h.binding_id AND k.version=h.current_version AND k.provider='facebook'
            WHERE c.tenant_id=r.tenant_id AND c.store_id=r.store_id AND c.page_id=r.page_id;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------
-- §4.3 record: lease-fenced (stale lease or vanished connection => 'stale', no write). In one transaction: upserts the
-- Go-derived states (CHECKs the vocabulary and that binding_id belongs to the Page), re-applies runtime demotions
-- (runtime_task always; runtime_permission unless perm_source=graph), flips the connection on invalid/page_gone (audited),
-- recomputes severity (§5.1) over the resulting rows, opens/closes the alert episode (§5.2), sets cadence/counters.
-- p_result = {outcome, token, subcode?, perms?, perm_source, fb_fields?, graph_codes[], states[], evidence}; never a token.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.record_meta_health(p_page text, p_generation bigint, p_lease bytea, p_result jsonb) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
    r integration.meta_health_probes;
    c integration.meta_connections;
    v_now timestamptz:=clock_timestamp();
    v_outcome text; v_token text; v_sev text; v_next timestamptz; v_fail smallint; v_ep bigint; v_status text; v_mailed boolean;
    v_demotions jsonb; s jsonb; d record;
BEGIN
    IF p_page IS NULL OR p_page !~ '^[0-9]{1,40}$' OR p_generation IS NULL OR p_generation<=0 OR p_lease IS NULL
       OR octet_length(p_lease)<>32 OR p_result IS NULL OR jsonb_typeof(p_result)<>'object'
       OR p_result->>'outcome' NOT IN ('probed','rate_limited','unknown')
       OR p_result->>'token' NOT IN ('valid','invalid','page_gone','unknown') THEN
        RAISE EXCEPTION 'invalid probe result' USING ERRCODE='22023'; END IF;
    IF current_setting('transaction_isolation',true) IS DISTINCT FROM 'read committed' THEN
        RAISE EXCEPTION 'read committed required' USING ERRCODE='25001'; END IF;
    SELECT * INTO r FROM integration.meta_health_probes x WHERE x.page_id=p_page FOR UPDATE;
    IF NOT FOUND THEN RETURN 'stale'; END IF;
    IF r.generation<>p_generation OR r.lease_token IS NULL OR r.lease_token<>p_lease OR r.lease_until<v_now THEN
        RETURN 'stale'; END IF;
    SELECT * INTO c FROM integration.meta_connections x WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page;
    IF NOT FOUND THEN RETURN 'stale'; END IF;
    v_outcome:=p_result->>'outcome'; v_token:=p_result->>'token';
    -- 1. capability upsert (probed only; rate_limited/unknown change no state, §4.3)
    IF v_outcome='probed' AND jsonb_typeof(p_result->'states')='array' THEN
        SELECT coalesce(jsonb_agg(jsonb_build_object('b',x.binding_id,'c',x.capability,'r',x.reason) ORDER BY x.binding_id,x.capability),'[]'::jsonb)
          INTO v_demotions FROM integration.binding_capabilities x
         WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page AND x.reason IN ('runtime_permission','runtime_task');
        FOR s IN SELECT * FROM jsonb_array_elements(p_result->'states') LOOP
            IF s->>'binding_id' IS NULL OR s->>'provider' NOT IN ('facebook','instagram')
               OR s->>'capability' NOT IN ('read_comment','private_reply','dm_session','reply_public')
               OR s->>'state' NOT IN ('ok','missing_permission','missing_task','not_subscribed','reauth_required','review_required','unsupported','unknown')
               OR s->>'reason' IS NULL OR s->>'reason' !~ '^[a-z0-9_]{1,40}$' THEN
                RAISE EXCEPTION 'invalid capability state' USING ERRCODE='22023'; END IF;
            IF (s->>'binding_id')::uuid IS DISTINCT FROM c.fb_binding AND (s->>'binding_id')::uuid IS DISTINCT FROM c.ig_binding THEN
                RAISE EXCEPTION 'binding not of this page' USING ERRCODE='22023'; END IF;
            INSERT INTO integration.binding_capabilities(tenant_id,store_id,page_id,binding_id,provider,capability,state,reason,evidence,checked_at)
                VALUES(r.tenant_id,r.store_id,p_page,(s->>'binding_id')::uuid,s->>'provider',s->>'capability',s->>'state',s->>'reason',
                       CASE WHEN p_result->>'evidence'='MOCK' THEN 'MOCK' ELSE 'DESIGN' END,v_now)
                ON CONFLICT (tenant_id,store_id,binding_id,capability)
                DO UPDATE SET state=EXCLUDED.state, reason=EXCLUDED.reason, checked_at=EXCLUDED.checked_at;
        END LOOP;
        FOR d IN SELECT * FROM jsonb_array_elements(v_demotions) LOOP
            IF d->>'r'='runtime_task' THEN
                UPDATE integration.binding_capabilities x SET state='missing_task',reason='runtime_task',checked_at=v_now
                 WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.binding_id=(d->>'b')::uuid AND x.capability=d->>'c';
            ELSIF d->>'r'='runtime_permission' AND p_result->>'perm_source' IS DISTINCT FROM 'graph' THEN
                UPDATE integration.binding_capabilities x SET state='missing_permission',reason='runtime_permission',checked_at=v_now
                 WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.binding_id=(d->>'b')::uuid AND x.capability=d->>'c';
            END IF;
        END LOOP;
    END IF;
    -- 2. token invalid/page_gone flips the connection once (audited); it is a no-op the next sweep
    IF v_token IN ('invalid','page_gone') THEN
        UPDATE integration.meta_connections x SET status='reauth_required',updated_at=v_now
         WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page AND x.status='active';
        IF FOUND THEN
            INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
                VALUES(r.tenant_id,r.store_id,c.connected_by,'meta.connect.reauth_required');
        END IF;
    END IF;
    -- 3. severity (§5.1) over the resulting capability rows + status
    SELECT x.status INTO v_status FROM integration.meta_connections x
     WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page;
    IF v_status='reauth_required' THEN
        v_sev:='blocking';
    ELSIF EXISTS(SELECT 1 FROM integration.binding_capabilities x WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page
                 AND x.capability IN ('read_comment','private_reply') AND x.state NOT IN ('ok','review_required')) THEN
        v_sev:='blocking';
    ELSIF EXISTS(SELECT 1 FROM integration.binding_capabilities x WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page
                 AND x.capability IN ('dm_session','reply_public') AND x.state NOT IN ('ok','review_required')
                 AND NOT (x.state='unknown' AND x.reason='lc_u11_open')) THEN
        v_sev:='warning';
    ELSE
        v_sev:='none';
    END IF;
    -- 4. consecutive_failures (any non-probed outcome) and cadence (§4.1, ±10% jitter)
    v_fail:=CASE WHEN v_outcome='probed' THEN 0 ELSE r.consecutive_failures+1 END;
    IF v_outcome='unknown' AND v_fail>=6 AND v_sev='none' THEN v_sev:='warning'; END IF; -- probe_failing (banner only)
    v_next:=v_now+make_interval(secs=> CASE v_outcome
        WHEN 'rate_limited' THEN 900::double precision*power(2::double precision,LEAST(v_fail-1,4)::double precision)
        WHEN 'unknown' THEN 1800::double precision
        ELSE CASE v_sev WHEN 'none' THEN 21600::double precision ELSE 1800::double precision END
        END*(0.9::double precision+random()*0.2::double precision));
    -- 5. episode transition (§5.2): opens on ->blocking (mail), closes on ->none|warning
    IF v_sev='blocking' AND r.severity IS DISTINCT FROM 'blocking' THEN
        v_ep:=r.episode+1;
        UPDATE integration.meta_health_probes x SET episode=v_ep,episode_opened_at=v_now,episode_closed_at=NULL
         WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page;
        SELECT notify.enqueue_merchant_alert(r.tenant_id,r.store_id,'meta_health',p_page,v_ep) INTO v_mailed;
        IF v_mailed THEN
            UPDATE integration.meta_health_probes x SET last_mail_at=v_now
             WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page;
        END IF;
    ELSIF v_sev IN ('none','warning') AND r.severity='blocking' THEN
        UPDATE integration.meta_health_probes x SET episode_closed_at=v_now
         WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page;
    END IF;
    -- 6. final write (lease cleared; perm_source only refreshed on a probed outcome with a valid source)
    UPDATE integration.meta_health_probes x SET last_checked_at=v_now,next_due_at=v_next,last_outcome=v_outcome,
           perm_source=CASE WHEN p_result->>'perm_source' IN ('graph','snapshot') THEN p_result->>'perm_source' ELSE x.perm_source END,
           consecutive_failures=v_fail,severity=v_sev,lease_token=NULL,lease_until=NULL
     WHERE x.tenant_id=r.tenant_id AND x.store_id=r.store_id AND x.page_id=p_page;
    RETURN 'recorded';
END $$;

-- ---------------------------------------------------------------------------------------
-- §4.4 runtime demotion: permission errors observed by the send/read adapters. Binding taken from the operation, never the
-- caller. A later probe clears runtime_permission only with perm_source=graph; runtime_task clears only on reconnect.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.report_capability_failure(p_operation uuid, p_capability text, p_kind text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations; v_page text;
BEGIN
    IF p_operation IS NULL OR p_capability NOT IN ('read_comment','private_reply','dm_session','reply_public')
       OR p_kind NOT IN ('permission','task') OR current_setting('transaction_isolation',true) IS DISTINCT FROM 'read committed' THEN
        RAISE EXCEPTION 'invalid capability failure' USING ERRCODE='22023'; END IF;
    SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation;
    IF NOT FOUND THEN RETURN; END IF;
    SELECT c.page_id INTO v_page FROM integration.meta_connections c
     WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND o.binding_id IN (c.fb_binding,c.ig_binding);
    IF NOT FOUND THEN RETURN; END IF;
    IF p_kind='permission' THEN
        UPDATE integration.binding_capabilities x SET state='missing_permission',reason='runtime_permission',checked_at=clock_timestamp()
         WHERE x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.page_id=v_page AND x.binding_id=o.binding_id AND x.capability=p_capability;
    ELSE
        UPDATE integration.binding_capabilities x SET state='missing_task',reason='runtime_task',checked_at=clock_timestamp()
         WHERE x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.page_id=v_page AND x.binding_id=o.binding_id AND x.capability=p_capability;
    END IF;
    UPDATE integration.meta_health_probes x SET next_due_at=clock_timestamp()
     WHERE x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.page_id=v_page;
END $$;

-- ---------------------------------------------------------------------------------------
-- §3.4 evidence upgrade (called by LC-B2/B4): monotonic, binding taken from the operation. DESIGN<MOCK<LIVE_READ<LIVE_SEND.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.mark_capability_evidence(p_operation uuid, p_capability text, p_evidence text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations; v_page text;
BEGIN
    IF p_operation IS NULL OR p_capability NOT IN ('read_comment','private_reply','dm_session','reply_public')
       OR p_evidence NOT IN ('DESIGN','MOCK','LIVE_READ','LIVE_SEND')
       OR current_setting('transaction_isolation',true) IS DISTINCT FROM 'read committed' THEN
        RAISE EXCEPTION 'invalid evidence' USING ERRCODE='22023'; END IF;
    SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation;
    IF NOT FOUND THEN RETURN; END IF;
    SELECT c.page_id INTO v_page FROM integration.meta_connections c
     WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND o.binding_id IN (c.fb_binding,c.ig_binding);
    IF NOT FOUND THEN RETURN; END IF;
    UPDATE integration.binding_capabilities x SET evidence=p_evidence
     WHERE x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.page_id=v_page AND x.binding_id=o.binding_id AND x.capability=p_capability
       AND ((x.evidence='DESIGN' AND p_evidence IN ('MOCK','LIVE_READ','LIVE_SEND'))
         OR (x.evidence='MOCK' AND p_evidence IN ('LIVE_READ','LIVE_SEND'))
         OR (x.evidence='LIVE_READ' AND p_evidence='LIVE_SEND'));
END $$;

-- ---------------------------------------------------------------------------------------
-- §7.5 server-side Check: table row if present, else the snapshot derivation (§3.2 steps 1,4,8: status, scopes, Advanced
-- Access list passed by Go). Returns the state text or NULL when the binding is not a connected meta binding.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.binding_capability_state(p_tenant uuid, p_store uuid, p_binding uuid, p_capability text, p_advanced text[]) RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_prov text; v_state text; v_status text; v_scopes text[]; v_perm text;
BEGIN
    IF p_tenant IS NULL OR p_store IS NULL OR p_binding IS NULL OR p_capability NOT IN ('read_comment','private_reply','dm_session','reply_public') THEN
        RETURN NULL; END IF;
    SELECT b.provider INTO v_prov FROM integration.bindings b WHERE b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=p_binding;
    IF NOT FOUND THEN RETURN NULL; END IF;
    SELECT x.state INTO v_state FROM integration.binding_capabilities x
     WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.binding_id=p_binding AND x.capability=p_capability;
    IF FOUND THEN RETURN v_state; END IF;
    SELECT c.status,c.scopes INTO v_status,v_scopes FROM integration.meta_connections c
     WHERE c.tenant_id=p_tenant AND c.store_id=p_store AND p_binding IN (c.fb_binding,c.ig_binding);
    IF NOT FOUND THEN RETURN NULL; END IF;
    IF v_status<>'active' THEN RETURN 'reauth_required'; END IF;
    v_perm:=CASE
        WHEN v_prov='facebook' AND p_capability='read_comment' THEN 'pages_read_engagement'
        WHEN v_prov='facebook' AND p_capability IN ('private_reply','dm_session') THEN 'pages_messaging'
        WHEN v_prov='facebook' AND p_capability='reply_public' THEN 'pages_manage_engagement'
        WHEN v_prov='instagram' AND p_capability IN ('read_comment','reply_public') THEN 'instagram_manage_comments'
        WHEN v_prov='instagram' AND p_capability='private_reply' THEN 'instagram_manage_messages'
        ELSE '' END;
    IF v_perm<>'' AND NOT (v_perm=ANY(v_scopes)) THEN RETURN 'missing_permission'; END IF;
    IF v_perm<>'' AND NOT (v_perm=ANY(p_advanced)) THEN RETURN 'review_required'; END IF;
    RETURN 'ok';
END $$;

-- ---------------------------------------------------------------------------------------
-- §9 B2 manual re-check: integration:manage via identity.resolve_access; 404 not connected, 429 recheck_too_soon (<60 s).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.request_meta_health_recheck(p_hash bytea, p_store uuid, p_page text) RETURNS timestamptz
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; p integration.meta_health_probes; v_now timestamptz:=clock_timestamp();
BEGIN
    IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_page IS NULL OR p_page !~ '^[0-9]{1,40}$'
       OR current_setting('transaction_isolation',true) IS DISTINCT FROM 'read committed' THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
    SELECT * INTO a FROM integration.meta_connect_auth(p_hash,p_store,'integration:manage');
    SELECT * INTO p FROM integration.meta_health_probes x
     WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.page_id=p_page FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='MC404'; END IF;
    IF p.last_checked_at IS NOT NULL AND p.last_checked_at>=v_now-interval '60 seconds' THEN
        RAISE EXCEPTION 'recheck_too_soon' USING ERRCODE='MC429'; END IF;
    UPDATE integration.meta_health_probes x SET next_due_at=v_now
     WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.page_id=p_page;
    RETURN v_now;
END $$;

-- ---------------------------------------------------------------------------------------
-- §5.2 notify.merchant_alerts + enqueue (cool-down inside) / claim (renderer input) / record.
-- ---------------------------------------------------------------------------------------
CREATE TABLE notify.merchant_alerts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL,
    store_id        uuid NOT NULL,
    kind            text NOT NULL CHECK (kind='meta_health'),
    subject         text NOT NULL CHECK (subject ~ '^[0-9]{1,40}$'),
    episode         bigint NOT NULL CHECK (episode >= 0),
    state           text NOT NULL DEFAULT 'PENDING' CHECK (state IN ('PENDING','SENDING','SENT','UNKNOWN','FAILED','SKIPPED')),
    attempts        smallint NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 3),
    next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    batch_id        uuid,
    claimed_at      timestamptz,
    sent_at         timestamptz,
    recipient_hash  bytea CHECK (recipient_hash IS NULL OR octet_length(recipient_hash)=32),
    skip_reason     text CHECK (skip_reason IN ('no_owner','stale')),
    created_at      timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (tenant_id, store_id, kind, subject, episode),
    CHECK ((state='SKIPPED') = (skip_reason IS NOT NULL)),
    CHECK (state NOT IN ('SENDING','SENT','UNKNOWN') OR (batch_id IS NOT NULL AND claimed_at IS NOT NULL)),
    CHECK ((state='SENT') = (sent_at IS NOT NULL))
);
ALTER TABLE notify.merchant_alerts ENABLE ROW LEVEL SECURITY;
ALTER TABLE notify.merchant_alerts FORCE ROW LEVEL SECURITY;
REVOKE ALL ON notify.merchant_alerts FROM PUBLIC;

-- Cool-down inside: one mail per (Page, episode) via the UNIQUE key, and none if the same Page had a mail in the previous
-- 24 h (flapping). Returns true when a row was actually enqueued.
CREATE FUNCTION notify.enqueue_merchant_alert(p_tenant uuid, p_store uuid, p_kind text, p_subject text, p_episode bigint) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
    IF p_tenant IS NULL OR p_store IS NULL OR p_kind<>'meta_health' OR p_subject IS NULL OR p_subject !~ '^[0-9]{1,40}$'
       OR p_episode IS NULL OR p_episode<=0 OR current_setting('transaction_isolation',true) IS DISTINCT FROM 'read committed' THEN
        RAISE EXCEPTION 'invalid merchant alert' USING ERRCODE='22023'; END IF;
    IF EXISTS(SELECT 1 FROM notify.merchant_alerts a
              WHERE a.tenant_id=p_tenant AND a.store_id=p_store AND a.subject=p_subject
                AND a.state IN ('SENDING','SENT','UNKNOWN') AND a.claimed_at IS NOT NULL
                AND a.claimed_at>=clock_timestamp()-interval '24 hours') THEN
        RETURN false;
    END IF;
    INSERT INTO notify.merchant_alerts(tenant_id,store_id,kind,subject,episode)
        VALUES(p_tenant,p_store,p_kind,p_subject,p_episode)
        ON CONFLICT (tenant_id,store_id,kind,subject,episode) DO NOTHING;
    RETURN FOUND;
END $$;

-- Housekeeping (UNKNOWN for dead SENDING, stale SKIPPED, 90-day delete <=200), then claim up to p_limit PENDING alerts under
-- the shared 60% notify budget p_daily_cap. Returns the renderer input as jsonb: store_name, page_name, the stopped
-- capabilities and their reasons (from binding_capabilities) and the owners; the recipient address leaves SQL only here, in
-- memory. The mail link is the admin origin injected by the worker (never a storefront or Meta URL, §10). Nothing else is read by Go.
CREATE FUNCTION notify.claim_merchant_alerts(p_limit integer, p_daily_cap integer) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
    v_now timestamptz:=clock_timestamp(); v_day timestamptz; v_out jsonb:='[]'::jsonb; v_used integer:=0;
    a record; v_emails text[]; v_store_name text; v_page_name text; v_caps jsonb;
BEGIN
    IF p_limit IS NULL OR p_limit<1 OR p_limit>50 OR p_daily_cap IS NULL OR p_daily_cap<0 THEN
        RAISE EXCEPTION 'invalid claim' USING ERRCODE='22023'; END IF;
    -- §5.2 "sharing the 60% notify budget": count today's notify mail of BOTH tables (buyer/merchant-new outbox + these alerts),
    -- same UTC+8 day alignment as notify.claim_batch (0090), so a busy buyer day leaves no budget for the meta_health mail.
    v_day:=to_timestamp((floor((extract(epoch FROM v_now)+28800)/86400)*86400-28800)::double precision);
    SELECT count(DISTINCT coalesce(x.batch_id::text,x.order_id::text||x.kind)) INTO v_used FROM notify.outbox x
     WHERE x.state IN ('SENDING','SENT','UNKNOWN') AND x.claimed_at>=v_day;
    SELECT v_used + count(*) INTO v_used FROM notify.merchant_alerts x
     WHERE x.state IN ('SENDING','SENT','UNKNOWN') AND x.claimed_at>=v_day;
    UPDATE notify.merchant_alerts SET state='UNKNOWN' WHERE state='SENDING' AND claimed_at<v_now-interval '15 minutes';
    UPDATE notify.merchant_alerts SET state='SKIPPED',skip_reason='stale' WHERE state='PENDING' AND next_attempt_at<v_now-interval '24 hours';
    DELETE FROM notify.merchant_alerts WHERE id IN (
        SELECT x.id FROM notify.merchant_alerts x WHERE x.state<>'PENDING' AND x.created_at<v_now-interval '90 days' LIMIT 200);
    FOR a IN SELECT x.* FROM notify.merchant_alerts x WHERE x.state='PENDING' AND x.next_attempt_at<=v_now
             ORDER BY x.created_at,x.id LIMIT p_limit FOR UPDATE SKIP LOCKED LOOP
        EXIT WHEN v_used>=p_daily_cap;
        SELECT array_agg(DISTINCT e) INTO v_emails FROM (
            SELECT identity.staff_password_email(f.principal_id) AS e FROM identity.store_staff f
            JOIN identity.memberships m ON m.tenant_id=f.tenant_id AND m.principal_id=f.principal_id AND m.active
            WHERE f.tenant_id=a.tenant_id AND f.store_id=a.store_id AND f.role='owner') q WHERE e IS NOT NULL;
        IF v_emails IS NULL THEN
            UPDATE notify.merchant_alerts SET state='SKIPPED',skip_reason='no_owner' WHERE id=a.id;
            CONTINUE;
        END IF;
        SELECT s.name INTO v_store_name FROM control.stores s WHERE s.tenant_id=a.tenant_id AND s.id=a.store_id;
        SELECT c.page_name INTO v_page_name FROM integration.meta_connections c
         WHERE c.tenant_id=a.tenant_id AND c.store_id=a.store_id AND c.page_id=a.subject;
        SELECT coalesce(jsonb_agg(jsonb_build_object('capability',x.capability,'reason',x.reason) ORDER BY x.capability),'[]'::jsonb)
          INTO v_caps FROM integration.binding_capabilities x
         WHERE x.tenant_id=a.tenant_id AND x.store_id=a.store_id AND x.page_id=a.subject
           AND x.state NOT IN ('ok','review_required') AND NOT (x.state='unknown' AND x.reason='lc_u11_open');
        -- The record key is the alert's own id (one alert = one mail to N owners); batch_id mirrors it so the shared
        -- worker's record_result-style addressing stays uniform (§5.2: record_merchant_alert takes p_id = the alert id).
        UPDATE notify.merchant_alerts SET state='SENDING',attempts=attempts+1,claimed_at=v_now,batch_id=a.id WHERE id=a.id;
        v_used:=v_used+1;
        v_out:=v_out||jsonb_build_array(jsonb_build_object('batch_id',a.id,'kind','meta_health','store_name',v_store_name,
            'to',to_jsonb(v_emails),'store_id',a.store_id,'subject',a.subject,'page_name',v_page_name,
            'episode',a.episode,'meta_caps',v_caps));
    END LOOP;
    RETURN v_out;
END $$;

-- SENT / FAILED (retry 2 min, then 10 min, then give up) / UNKNOWN (final) of one alert, only from SENDING.
CREATE FUNCTION notify.record_merchant_alert(p_id uuid, p_state text, p_recipient_hash bytea) RETURNS integer
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_n integer; v_now timestamptz:=clock_timestamp();
BEGIN
    IF p_id IS NULL OR p_state NOT IN ('SENT','FAILED','UNKNOWN') OR (p_recipient_hash IS NOT NULL AND octet_length(p_recipient_hash)<>32) THEN
        RAISE EXCEPTION 'invalid record' USING ERRCODE='PT400'; END IF;
    UPDATE notify.merchant_alerts x SET
           state=CASE p_state WHEN 'SENT' THEN 'SENT' WHEN 'UNKNOWN' THEN 'UNKNOWN' WHEN 'FAILED' THEN CASE WHEN x.attempts>=3 THEN 'FAILED' ELSE 'PENDING' END END,
           sent_at=CASE WHEN p_state='SENT' THEN v_now END,
           next_attempt_at=CASE WHEN p_state='FAILED' AND x.attempts<3 THEN v_now+CASE WHEN x.attempts=1 THEN interval '2 minutes' ELSE interval '10 minutes' END ELSE x.next_attempt_at END,
           recipient_hash=p_recipient_hash
     WHERE x.id=p_id AND x.state='SENDING';
    GET DIAGNOSTICS v_n=ROW_COUNT;
    RETURN v_n;
END $$;

-- ---------------------------------------------------------------------------------------
-- Privilege delta.
-- ---------------------------------------------------------------------------------------
-- commerce_runtime read policies (GUC-scoped) + column grant on meta_health_probes (§8: five columns only).
CREATE POLICY binding_capabilities_read ON integration.binding_capabilities FOR SELECT TO commerce_runtime
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT ON integration.binding_capabilities TO commerce_runtime;
CREATE POLICY meta_health_probes_read ON integration.meta_health_probes FOR SELECT TO commerce_runtime
    USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
GRANT SELECT (page_id, severity, last_checked_at, next_due_at, episode_opened_at) ON integration.meta_health_probes TO commerce_runtime;

-- definer owner = control (0095 pattern): the bodies decide every row.
CREATE POLICY binding_capabilities_writer ON integration.binding_capabilities FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
GRANT SELECT, INSERT, UPDATE(state,reason,evidence,checked_at) ON integration.binding_capabilities TO commerce_integration_writer;
CREATE POLICY meta_health_probes_writer ON integration.meta_health_probes FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
GRANT SELECT, INSERT, UPDATE(next_due_at,lease_token,lease_until,generation,consecutive_failures,last_outcome,last_checked_at,perm_source,severity,episode,episode_opened_at,episode_closed_at,last_mail_at)
    ON integration.meta_health_probes TO commerce_integration_writer;
-- §8 "also": the reauth audit. INSERT + USAGE on ops already exist (0064:43/475, 0066:55); only the action-limited policy is new.
CREATE POLICY meta_health_reauth_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK (action='meta.connect.reauth_required'
      AND EXISTS(SELECT 1 FROM integration.meta_connections c WHERE c.tenant_id=ops.audit_events.tenant_id AND c.store_id=ops.audit_events.store_id
        AND c.status='reauth_required' AND c.connected_by=ops.audit_events.principal_id AND c.updated_at>=clock_timestamp()-interval '1 minute'));

-- notify claim reads meta_connections(page_name) and binding_capabilities for the mail content (§5.2). commerce_checkout_writer
-- already has USAGE on integration (0016:84); this is the §5.2 read delta beyond the §8 object table (reported in DELIVERY.md).
CREATE POLICY meta_connections_notify_read ON integration.meta_connections FOR SELECT TO commerce_checkout_writer USING (true);
GRANT SELECT (page_id, page_name) ON integration.meta_connections TO commerce_checkout_writer;
CREATE POLICY binding_capabilities_notify_read ON integration.binding_capabilities FOR SELECT TO commerce_checkout_writer USING (true);
GRANT SELECT ON integration.binding_capabilities TO commerce_checkout_writer;

-- notify.* owner + worker grants.
GRANT SELECT, INSERT, UPDATE(state,attempts,next_attempt_at,batch_id,claimed_at,sent_at,recipient_hash,skip_reason) ON notify.merchant_alerts TO commerce_checkout_writer;
CREATE POLICY merchant_alerts_writer ON notify.merchant_alerts FOR ALL TO commerce_checkout_writer USING (true) WITH CHECK (true);

DO $$
DECLARE f text;
BEGIN
    FOREACH f IN ARRAY ARRAY[
        'integration.meta_health_on_connection()',
        'integration.meta_health_snapshot()',
        'integration.claim_meta_health_probes(integer)',
        'integration.record_meta_health(text,bigint,bytea,jsonb)',
        'integration.report_capability_failure(uuid,text,text)',
        'integration.mark_capability_evidence(uuid,text,text)',
        'integration.binding_capability_state(uuid,uuid,uuid,text,text[])',
        'integration.request_meta_health_recheck(bytea,uuid,text)'] LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_integration_writer',f);
        EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
    END LOOP;
    FOREACH f IN ARRAY ARRAY[
        'notify.enqueue_merchant_alert(uuid,uuid,text,text,bigint)',
        'notify.claim_merchant_alerts(integer,integer)',
        'notify.record_merchant_alert(uuid,text,bytea)'] LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_checkout_writer',f);
        EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
    END LOOP;
    EXECUTE 'GRANT EXECUTE ON FUNCTION integration.meta_health_snapshot() TO commerce_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION integration.request_meta_health_recheck(bytea,uuid,text) TO commerce_runtime';
    EXECUTE 'GRANT EXECUTE ON FUNCTION integration.claim_meta_health_probes(integer) TO commerce_claims_worker';
    EXECUTE 'GRANT EXECUTE ON FUNCTION integration.record_meta_health(text,bigint,bytea,jsonb) TO commerce_claims_worker';
    EXECUTE 'GRANT EXECUTE ON FUNCTION integration.report_capability_failure(uuid,text,text) TO commerce_claims_worker';
    EXECUTE 'GRANT EXECUTE ON FUNCTION integration.mark_capability_evidence(uuid,text,text) TO commerce_claims_worker';
    EXECUTE 'GRANT EXECUTE ON FUNCTION integration.binding_capability_state(uuid,uuid,uuid,text,text[]) TO commerce_claims_worker';
    EXECUTE 'GRANT EXECUTE ON FUNCTION notify.enqueue_merchant_alert(uuid,uuid,text,text,bigint) TO commerce_integration_writer';
    EXECUTE 'GRANT EXECUTE ON FUNCTION notify.claim_merchant_alerts(integer,integer) TO commerce_expiry_worker';
    EXECUTE 'GRANT EXECUTE ON FUNCTION notify.record_merchant_alert(uuid,text,bytea) TO commerce_expiry_worker';
END $$;

-- ---------------------------------------------------------------------------------------
-- Backfill: existing connections get a probe row (due now). Capability rows are deliberately NOT backfilled: the first
-- sweep creates them (evidence MOCK/DESIGN, §3.4); until then TableReader falls back to the snapshot (§7.3).
-- ---------------------------------------------------------------------------------------
INSERT INTO integration.meta_health_probes(tenant_id,store_id,page_id,next_due_at)
    SELECT c.tenant_id,c.store_id,c.page_id,clock_timestamp() FROM integration.meta_connections c
    ON CONFLICT (tenant_id,store_id,page_id) DO NOTHING;
