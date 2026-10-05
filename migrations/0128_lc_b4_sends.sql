-- 0128_lc_b4_sends.sql — LC-B4 (live-console-v1 §3.3-3.7, §4, Amendment 1 A1.1/A1.3/P2-1..5/P2-7c). File name is the number the
-- integrator assigned (0127 = LC-R1).
-- Purpose: the four manual-send producers (DM, manual private reply, public reply, offer-recommend comment), the worker-side
--   Check / secret loader / Finish hook, outbound storage (display copy + sealed dispatch copy), bundle<->conversation peers,
--   the claims.bundles.link_pending_manual flag, the auto-reply deltas (reply_used skip, takeover deny codes) and the read
--   definers for A8/A9/A13 (conversation_heads, read_outbound, link-pending bundles).
-- Depends on: 0008/0064 (integration.operations, plan_claim_reply, commerce_integration_writer), 0119 (inbox.conversation_state, dm_window),
--   0121 (msgtemplates), 0123 (comment facts / marks), 0125 (integration.binding_capability_state, mark_capability_evidence).
-- Used by: internal/inbox (send*.go), internal/integrations/metareply (send_dm.go, public_reply.go, manual_reply.go),
--   internal/claimsintake (reply_used), internal/httpapi (A4/A5/A6/A12, A8/A9/A13 deltas).
-- Invariants: producers run in ONE merchant transaction and make no network call; refusals are SQLSTATE PT409 (message = deny code),
--   PT422 invalid_text, PT429 rate_limited; advisory locks (lcn-dup|, lcn-rec|) are taken FIRST; the unique mpr: operation key stays the
--   final serialiser of the one private reply per comment; plaintext never reaches SQL (display copy and dispatch copy are sealed in Go).
-- Deviations from the frozen text (recorded in output/lc-b4-sends/DELIVERY.md): (1) p_template_id is text, p_template_version bigint
--   (LC-B5 ids are 'order-pay-link/v1' strings); (2) mdm:/mpub:/mrec: semantic keys derive from the operation id (the producer signature
--   carries no Idempotency-Key; HTTP replay is already absorbed by command.Run); (3) the mpr: unique index is unchanged (it already
--   admits the ':m1' class because its predicate is action='meta.private_reply').
-- Status: MOCK (REAL_PG + fake Graph).

GRANT USAGE ON SCHEMA inbox, social, meta_inbox, catalog TO commerce_integration_writer;
GRANT USAGE ON SCHEMA inbox TO commerce_claims_writer;

-- ---------------------------------------------------------------------------------------
-- Tables
-- ---------------------------------------------------------------------------------------
CREATE TABLE inbox.outbound_messages (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL,
    conversation_id uuid,
    comment_ref text CHECK (comment_ref ~ '^[0-9_]{1,80}$'),
    kind text NOT NULL CHECK (kind IN ('dm','private_reply','public_reply','recommend')),
    operation_id uuid NOT NULL,
    principal_id uuid NOT NULL,
    template_id text CHECK (template_id ~ '^[a-z0-9][a-z0-9/_-]{0,63}$'),
    template_version bigint CHECK (template_version > 0),
    key_id text NOT NULL CHECK (key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
    nonce bytea NOT NULL CHECK (octet_length(nonce) = 12),
    ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) BETWEEN 17 AND 16384),
    body_hmac bytea NOT NULL CHECK (octet_length(body_hmac) = 32),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, id),
    UNIQUE (tenant_id, store_id, operation_id),
    CHECK ((template_id IS NULL) = (template_version IS NULL)),
    CHECK ((kind = 'dm') = (conversation_id IS NOT NULL)),
    CHECK (kind NOT IN ('private_reply','public_reply') OR comment_ref IS NOT NULL)
);
-- Soft references (no FK): C5/C5c retention deletes conversations and old operations independently (§10).
CREATE INDEX outbound_messages_conversation ON inbox.outbound_messages(tenant_id, store_id, conversation_id, created_at DESC)
    WHERE conversation_id IS NOT NULL;

CREATE TABLE inbox.send_secrets (
    operation_id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    sealed bytea NOT NULL CHECK (octet_length(sealed) BETWEEN 17 AND 16384),
    enc bytea NOT NULL CHECK (octet_length(enc) = 32),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX send_secrets_age ON inbox.send_secrets(created_at);

CREATE TABLE inbox.bundle_peers (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    bundle_id uuid NOT NULL,
    peer_key text NOT NULL CHECK (peer_key ~ '^[0-9a-f]{64}$'),
    app_id text NOT NULL CHECK (app_id ~ '^[0-9]{1,40}$'),
    object text NOT NULL CHECK (object IN ('page','instagram')),
    asset_id text NOT NULL CHECK (asset_id ~ '^[0-9]{1,40}$'),
    operation_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, bundle_id, peer_key)
);
CREATE INDEX bundle_peers_peer ON inbox.bundle_peers(tenant_id, store_id, app_id, object, asset_id, peer_key);

ALTER TABLE inbox.outbound_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.outbound_messages FORCE ROW LEVEL SECURITY;
ALTER TABLE inbox.send_secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.send_secrets FORCE ROW LEVEL SECURITY;
ALTER TABLE inbox.bundle_peers ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.bundle_peers FORCE ROW LEVEL SECURITY;
REVOKE ALL ON inbox.outbound_messages, inbox.send_secrets, inbox.bundle_peers FROM PUBLIC;

-- Owner of every send definer; the definer bodies (scope GUCs or the frozen operation row) are the control.
GRANT SELECT, INSERT ON inbox.outbound_messages TO commerce_integration_writer;
GRANT SELECT, INSERT, DELETE ON inbox.send_secrets TO commerce_integration_writer;
GRANT SELECT, INSERT ON inbox.bundle_peers TO commerce_integration_writer;
CREATE POLICY outbound_send_rw ON inbox.outbound_messages FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
CREATE POLICY send_secret_rw ON inbox.send_secrets FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
CREATE POLICY bundle_peers_rw ON inbox.bundle_peers FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
-- dm_window_for_bundle (owner commerce_meta_writer) joins the peer to its conversation.
GRANT SELECT ON inbox.bundle_peers TO commerce_meta_writer;
CREATE POLICY bundle_peers_meta_read ON inbox.bundle_peers FOR SELECT TO commerce_meta_writer USING (true);

-- §4.2: the bundle whose claim link could not be sent because the comment's one private reply was already used.
ALTER TABLE claims.bundles ADD COLUMN link_pending_manual boolean NOT NULL DEFAULT false;

-- The flag clears when a claim link is issued for the bundle (system or merchant).
CREATE FUNCTION claims.clear_link_pending_manual() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    UPDATE claims.bundles b SET link_pending_manual = false
     WHERE b.tenant_id = NEW.tenant_id AND b.store_id = NEW.store_id AND b.id = NEW.bundle_id AND b.link_pending_manual;
    RETURN NULL;
END $$;
ALTER FUNCTION claims.clear_link_pending_manual() OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.clear_link_pending_manual() FROM PUBLIC;
CREATE TRIGGER links_clear_link_pending AFTER INSERT OR UPDATE ON claims.links
    FOR EACH ROW EXECUTE FUNCTION claims.clear_link_pending_manual();

-- ---------------------------------------------------------------------------------------
-- Privileges of commerce_integration_writer for the merchant-transaction readers/producers. Every read is scoped to the
-- transaction's tenant/store GUCs (inbox.lcn_in_scope); the worker-side definers pin those GUCs from the frozen operation.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.lcn_in_scope(p_tenant uuid, p_store uuid) RETURNS boolean
LANGUAGE sql STABLE SET search_path = pg_catalog AS $$
    SELECT p_tenant = nullif(current_setting('app.tenant_id', true), '')::uuid
       AND p_store = nullif(current_setting('app.store_id', true), '')::uuid
$$;
ALTER FUNCTION inbox.lcn_in_scope(uuid, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.lcn_in_scope(uuid, uuid) FROM PUBLIC;

GRANT SELECT(id, tenant_id, store_id, app_id, object, asset_id, peer_key) ON social.conversations TO commerce_integration_writer;
CREATE POLICY conversation_send_read ON social.conversations FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));

GRANT SELECT ON inbox.conversation_state TO commerce_integration_writer;
GRANT UPDATE(mode, assignee_principal, takeover_generation, human_until, last_human_outbound_at, last_outbound_at, version, updated_at)
    ON inbox.conversation_state TO commerce_integration_writer;
CREATE POLICY conversation_state_send ON inbox.conversation_state FOR ALL TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id)) WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id));

GRANT SELECT(source_object_id, created_at) ON live.claim_sources TO commerce_integration_writer;
CREATE POLICY source_send_read ON live.claim_sources FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));
GRANT SELECT(tenant_id, store_id, session_id, state) ON live.claim_windows TO commerce_integration_writer;
CREATE POLICY window_send_read ON live.claim_windows FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));

GRANT SELECT(applied_event_id, app_id) ON claims.meta_intake TO commerce_integration_writer;
CREATE POLICY intake_send_read ON claims.meta_intake FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));
-- Row lock for FOR SHARE only (§4.2); WITH CHECK false forbids any mutation.
GRANT UPDATE(updated_at) ON claims.meta_intake TO commerce_integration_writer;
CREATE POLICY intake_send_lock ON claims.meta_intake FOR UPDATE TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id)) WITH CHECK (false);
CREATE POLICY event_send_read ON claims.events FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));

GRANT SELECT(tenant_id, store_id, id, session_id, link_pending_manual, created_at) ON claims.bundles TO commerce_integration_writer;
GRANT UPDATE(link_pending_manual) ON claims.bundles TO commerce_integration_writer;
CREATE POLICY bundle_send_read ON claims.bundles FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id)
        OR (tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s));
CREATE POLICY bundle_send_flag ON claims.bundles FOR UPDATE TO commerce_integration_writer
    USING ((tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s))
    WITH CHECK ((tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s));

GRANT SELECT(tenant_id, store_id, id, session_id, keyword, sku_id, active, version) ON live.offers TO commerce_integration_writer;
CREATE POLICY offer_send_read ON live.offers FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));
GRANT UPDATE(updated_at) ON live.offers TO commerce_integration_writer;
CREATE POLICY offer_send_lock ON live.offers FOR UPDATE TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id)) WITH CHECK (false);

GRANT SELECT(app_id, object, asset_id, tenant_id, store_id, enabled) ON meta_inbox.routes TO commerce_integration_writer;
CREATE POLICY route_send_read ON meta_inbox.routes FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));

-- New actions: only these producers insert them (commerce_runtime has no INSERT on integration.operations).
CREATE POLICY lcn_send_insert ON integration.operations FOR INSERT TO commerce_integration_writer
    WITH CHECK (state = 'READY' AND generation = 0 AND actor_kind = 'MERCHANT' AND purpose = 'service'
        AND provider IN ('facebook','instagram') AND action IN ('meta.dm_send','meta.public_reply','meta.offer_recommend','meta.private_reply'));
CREATE POLICY lcn_send_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id)
        AND action IN ('inbox.dm.planned','inbox.private_reply.planned','inbox.private_reply.preempt_confirmed',
                       'inbox.public_reply.planned','live.offer.recommended','inbox.takeover'));
-- plan_claim_reply / claim_reply_plannable audit one more skip code (§14.1 clause 2).
CREATE POLICY claim_reply_used_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK ((tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s)
        AND action = 'claim_reply_skipped:reply_used');

-- ---------------------------------------------------------------------------------------
-- Private helpers (owner commerce_integration_writer; no caller EXECUTE besides the owner).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.lcn_scope() RETURNS uuid[]
LANGUAGE plpgsql STABLE SET search_path = pg_catalog AS $$
BEGIN
    IF current_setting('transaction_isolation') <> 'read committed'
       OR coalesce(current_setting('app.tenant_id', true), '') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
       OR coalesce(current_setting('app.store_id', true), '') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
       OR coalesce(current_setting('app.principal_id', true), '') !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
       OR nullif(current_setting('app.buyer_id', true), '') IS NOT NULL THEN
        RAISE EXCEPTION 'invalid send scope' USING ERRCODE = '22023';
    END IF;
    RETURN ARRAY[current_setting('app.tenant_id')::uuid, current_setting('app.store_id')::uuid, current_setting('app.principal_id')::uuid];
END $$;
ALTER FUNCTION inbox.lcn_scope() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.lcn_scope() FROM PUBLIC;

-- Rate cap (I23): <= 60 human sends per store per minute (approximate, no lock, A1.3 clause 4).
CREATE FUNCTION inbox.lcn_rate_check(p_tenant uuid, p_store uuid) RETURNS void
LANGUAGE plpgsql STABLE SET search_path = pg_catalog AS $$
BEGIN
    IF (SELECT count(*) FROM integration.operations o
         WHERE o.tenant_id = p_tenant AND o.store_id = p_store
           AND o.action IN ('meta.dm_send','meta.public_reply','meta.offer_recommend','meta.private_reply')
           AND o.request->>'origin' = 'human' AND o.created_at > clock_timestamp() - interval '1 minute') >= 60 THEN
        RAISE EXCEPTION 'rate_limited' USING ERRCODE = 'PT429';
    END IF;
END $$;
ALTER FUNCTION inbox.lcn_rate_check(uuid, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.lcn_rate_check(uuid, uuid) FROM PUBLIC;

-- The common planning tail (§4.1): operation + READY event + outbound display row + sealed dispatch copy + audit. The River job
-- must be exactly this transaction's external_operation_v1 row (plan_claim_reply pattern).
CREATE FUNCTION inbox.lcn_emit(p_tenant uuid, p_store uuid, p_principal uuid, p_kind text, p_action text, p_binding uuid,
 p_binding_version bigint, p_provider text, p_asset text, p_semantic_key text, p_request jsonb, p_operation uuid, p_job bigint,
 p_outbound uuid, p_conversation uuid, p_comment_ref text, p_body_hmac bytea, p_display_key_id text, p_display_nonce bytea,
 p_display_ciphertext bytea, p_secret_enc bytea, p_secret_sealed bytea, p_template_id text, p_template_version bigint, p_audit text)
RETURNS void LANGUAGE plpgsql VOLATILE SET search_path = pg_catalog AS $$
DECLARE j record;
BEGIN
    IF p_operation IS NULL OR p_job IS NULL OR p_job <= 0 OR p_outbound IS NULL OR octet_length(p_request::text) > 2048
       OR p_body_hmac IS NULL OR octet_length(p_body_hmac) <> 32
       OR p_display_key_id IS NULL OR p_display_key_id !~ '^[A-Za-z0-9_-]{1,64}$'
       OR p_display_nonce IS NULL OR octet_length(p_display_nonce) <> 12
       OR p_display_ciphertext IS NULL OR octet_length(p_display_ciphertext) NOT BETWEEN 17 AND 16384
       OR p_secret_enc IS NULL OR octet_length(p_secret_enc) <> 32
       OR p_secret_sealed IS NULL OR octet_length(p_secret_sealed) NOT BETWEEN 17 AND 16384 THEN
        RAISE EXCEPTION 'invalid send plan' USING ERRCODE = '22023';
    END IF;
    SELECT r.id INTO j FROM river.river_job r WHERE r.id = p_job AND r.kind = 'external_operation_v1' AND r.queue = 'default'
       AND r.unique_key IS NULL AND r.args = jsonb_build_object('operation_id', p_operation::text, 'version', 1)
       AND r.xmin = pg_current_xact_id()::xid;
    IF NOT FOUND THEN RAISE EXCEPTION 'invalid send plan' USING ERRCODE = '22023'; END IF;
    INSERT INTO integration.operations(tenant_id, store_id, id, principal_id, binding_id, binding_version, provider, external_asset_id,
        purpose, action, semantic_key, request_hash, request, job_id)
    VALUES (p_tenant, p_store, p_operation, p_principal, p_binding, p_binding_version, p_provider, p_asset,
        'service', p_action, p_semantic_key, sha256(convert_to(p_request::text, 'UTF8')), p_request, p_job);
    INSERT INTO integration.operation_events(tenant_id, store_id, operation_id, generation, state, mode, reason_code)
    VALUES (p_tenant, p_store, p_operation, 0, 'READY', '', 'operation_planned');
    INSERT INTO inbox.outbound_messages(tenant_id, store_id, id, conversation_id, comment_ref, kind, operation_id, principal_id,
        template_id, template_version, key_id, nonce, ciphertext, body_hmac)
    VALUES (p_tenant, p_store, p_outbound, p_conversation, p_comment_ref, p_kind, p_operation, p_principal,
        p_template_id, p_template_version, p_display_key_id, p_display_nonce, p_display_ciphertext, p_body_hmac);
    INSERT INTO inbox.send_secrets(operation_id, tenant_id, store_id, sealed, enc)
    VALUES (p_operation, p_tenant, p_store, p_secret_sealed, p_secret_enc);
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (p_tenant, p_store, p_principal, p_audit);
END $$;
ALTER FUNCTION inbox.lcn_emit(uuid, uuid, uuid, text, text, uuid, bigint, text, text, text, jsonb, uuid, bigint, uuid, uuid, text,
 bytea, text, bytea, bytea, bytea, bytea, text, bigint, text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.lcn_emit(uuid, uuid, uuid, text, text, uuid, bigint, text, text, text, jsonb, uuid, bigint, uuid, uuid, text,
 bytea, text, bytea, bytea, bytea, bytea, text, bigint, text) FROM PUBLIC;

-- ---------------------------------------------------------------------------------------
-- inbox.plan_dm (A12; A1.3 lock order: lcn-dup -> conversation_state FOR UPDATE -> binding FOR SHARE)
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.plan_dm(p_conversation uuid, p_expected_generation bigint, p_order uuid, p_operation uuid, p_job bigint,
 p_outbound uuid, p_body_hmac bytea, p_display_key_id text, p_display_nonce bytea, p_display_ciphertext bytea, p_secret_enc bytea,
 p_secret_sealed bytea, p_template_id text, p_template_version bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; c record; st record; b record; v_now timestamptz := clock_timestamp();
 v_provider text; v_hmac text; v_gen bigint; v_req jsonb; v_changed boolean := false;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_conversation IS NULL OR p_expected_generation IS NULL OR p_expected_generation < 0 OR p_body_hmac IS NULL
       OR octet_length(p_body_hmac) <> 32 OR (p_template_id IS NULL) <> (p_template_version IS NULL) THEN
        RAISE EXCEPTION 'invalid dm plan' USING ERRCODE = '22023';
    END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply']) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    v_hmac := encode(p_body_hmac, 'hex');
    -- A1.3 clause 1: serialise same-body sends before reading for duplicates and before any insert.
    PERFORM pg_advisory_xact_lock(hashtextextended('lcn-dup|' || v_s::text || '|' || p_conversation::text || '|' || v_hmac, 0));
    IF EXISTS (SELECT 1 FROM integration.operations o WHERE o.tenant_id = v_t AND o.store_id = v_s AND o.action = 'meta.dm_send'
                AND o.request->>'conversation_id' = p_conversation::text AND o.request->>'body_hmac' = v_hmac
                AND o.created_at > v_now - interval '30 seconds') THEN
        RAISE EXCEPTION 'duplicate_recent' USING ERRCODE = 'PT409';
    END IF;
    PERFORM inbox.lcn_rate_check(v_t, v_s);
    SELECT x.id, x.object, x.asset_id, x.app_id, x.peer_key INTO c FROM social.conversations x
     WHERE x.id = p_conversation AND x.tenant_id = v_t AND x.store_id = v_s;
    IF NOT FOUND THEN RAISE EXCEPTION 'conversation_gone' USING ERRCODE = 'PT409'; END IF;
    SELECT s.mode, s.assignee_principal, s.takeover_generation, s.human_until, s.last_inbound_at INTO st FROM inbox.conversation_state s
     WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.conversation_id = p_conversation FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'conversation_gone' USING ERRCODE = 'PT409'; END IF;
    -- §3.6 lazy takeover expiry, applied by this write.
    IF st.mode = 'human' AND st.human_until IS NOT NULL AND v_now >= st.human_until THEN
        st.mode := 'auto'; st.assignee_principal := NULL; st.takeover_generation := st.takeover_generation + 1; st.human_until := NULL;
    END IF;
    IF p_expected_generation <> st.takeover_generation THEN RAISE EXCEPTION 'takeover_changed' USING ERRCODE = 'PT409'; END IF;
    IF st.last_inbound_at IS NULL OR st.last_inbound_at + interval '24 hours' - interval '5 minutes' <= v_now THEN
        RAISE EXCEPTION 'window_closed' USING ERRCODE = 'PT409';
    END IF;
    v_provider := CASE c.object WHEN 'page' THEN 'facebook' ELSE 'instagram' END;
    SELECT z.id, z.semantic_version INTO b FROM integration.bindings z
     WHERE z.tenant_id = v_t AND z.store_id = v_s AND z.provider = v_provider AND z.external_asset_id = c.asset_id AND z.enabled
     ORDER BY z.id LIMIT 1 FOR SHARE;
    IF NOT FOUND OR coalesce(integration.binding_capability_state(v_t, v_s, b.id, 'dm_session', ARRAY[]::text[]), 'unknown')
                    NOT IN ('ok', 'review_required') THEN
        RAISE EXCEPTION 'capability' USING ERRCODE = 'PT409';
    END IF;
    -- §3.6 implicit takeover: only a real transition (auto -> human, or another assignee) bumps the generation.
    v_gen := st.takeover_generation;
    IF st.mode <> 'human' OR st.assignee_principal IS DISTINCT FROM v_p THEN v_gen := v_gen + 1; v_changed := true; END IF;
    UPDATE inbox.conversation_state s SET mode = 'human', assignee_principal = v_p, takeover_generation = v_gen,
           last_human_outbound_at = v_now, human_until = v_now + interval '6 hours', version = s.version + 1, updated_at = v_now
     WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.conversation_id = p_conversation;
    IF v_changed THEN
        INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (v_t, v_s, v_p, 'inbox.takeover');
    END IF;
    v_req := jsonb_build_object('v', 1, 'kind', 'dm', 'message_type', 'dm', 'origin', 'human', 'platform', v_provider,
        'asset_id', c.asset_id, 'app_id', c.app_id, 'conversation_id', p_conversation, 'conversation_known', true,
        'peer_key', c.peer_key, 'outbound_id', p_outbound, 'body_hmac', v_hmac, 'policy', 'lcn-policy/v1',
        'takeover_generation', v_gen, 'principal_id', v_p,
        'deadline_at', to_char((st.last_inbound_at + interval '24 hours' - interval '5 minutes') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
    IF p_template_id IS NOT NULL THEN v_req := v_req || jsonb_build_object('template_id', p_template_id, 'template_version', p_template_version); END IF;
    IF p_order IS NOT NULL THEN v_req := v_req || jsonb_build_object('order_id', p_order); END IF;
    PERFORM inbox.lcn_emit(v_t, v_s, v_p, 'dm', 'meta.dm_send', b.id, b.semantic_version, v_provider, c.asset_id,
        'mdm:' || substr(encode(sha256(convert_to(p_conversation::text || '|' || p_operation::text, 'UTF8')), 'hex'), 1, 48),
        v_req, p_operation, p_job, p_outbound, p_conversation, NULL, p_body_hmac, p_display_key_id, p_display_nonce,
        p_display_ciphertext, p_secret_enc, p_secret_sealed, p_template_id, p_template_version, 'inbox.dm.planned');
    RETURN p_operation;
END $$;
ALTER FUNCTION inbox.plan_dm(uuid, bigint, uuid, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.plan_dm(uuid, bigint, uuid, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.plan_dm(uuid, bigint, uuid, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- inbox.plan_manual_private_reply (A4; locks: lcn-dup -> intake FOR SHARE -> lcn-mpr -> conversation_state -> binding)
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.plan_manual_private_reply(p_session uuid, p_comment_ref text, p_comment_created_at timestamptz,
 p_confirm_preempt boolean, p_operation uuid, p_job bigint, p_outbound uuid, p_body_hmac bytea, p_display_key_id text,
 p_display_nonce bytea, p_display_ciphertext bytea, p_secret_enc bytea, p_secret_sealed bytea, p_template_id text, p_template_version bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; src record; ir record; ex record; b record; st record; cv record;
 v_now timestamptz := clock_timestamp(); v_hmac text; v_key text; v_deadline timestamptz; v_app text; v_bundle uuid;
 v_conv uuid; v_gen bigint := 0; v_known boolean := false; v_req jsonb; v_preempt boolean := false; v_live boolean;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_session IS NULL OR p_comment_ref IS NULL OR p_comment_ref !~ '^[0-9_]{1,80}$' OR p_comment_created_at IS NULL
       OR p_confirm_preempt IS NULL OR p_body_hmac IS NULL OR octet_length(p_body_hmac) <> 32
       OR (p_template_id IS NULL) <> (p_template_version IS NULL) THEN
        RAISE EXCEPTION 'invalid private reply plan' USING ERRCODE = '22023';
    END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply']) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    SELECT s.id, s.platform, s.object, s.asset_id, s.binding_id, s.private_reply INTO src FROM live.claim_sources s
     WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.session_id = p_session AND s.active ORDER BY s.created_at DESC LIMIT 1;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    v_hmac := encode(p_body_hmac, 'hex');
    PERFORM pg_advisory_xact_lock(hashtextextended('lcn-dup|' || v_s::text || '|' || p_comment_ref || '|' || v_hmac, 0));
    IF EXISTS (SELECT 1 FROM integration.operations o WHERE o.tenant_id = v_t AND o.store_id = v_s AND o.action = 'meta.private_reply'
                AND o.request->>'comment_ref' = p_comment_ref AND o.request->>'body_hmac' = v_hmac
                AND o.created_at > v_now - interval '30 seconds') THEN
        RAISE EXCEPTION 'duplicate_recent' USING ERRCODE = 'PT409';
    END IF;
    PERFORM inbox.lcn_rate_check(v_t, v_s);
    -- §4.2: the intake apply takes FOR UPDATE on this row; whichever of the two proceeds first wins.
    SELECT i.id, i.state, i.applied_event_id INTO ir FROM claims.meta_intake i
     WHERE i.object = src.object AND i.asset_id = src.asset_id AND i.comment_ref = p_comment_ref
       AND i.tenant_id = v_t AND i.store_id = v_s FOR SHARE;
    IF FOUND AND ir.state = 'PENDING' THEN RAISE EXCEPTION 'auto_pending' USING ERRCODE = 'PT409'; END IF;
    -- The one-private-reply-per-comment key; the manual and the auto planner serialise on the same advisory key.
    v_key := 'mpr:' || substr(encode(sha256(convert_to(src.object || '|' || src.asset_id || '|' || p_comment_ref, 'UTF8')), 'hex'), 1, 48);
    PERFORM pg_advisory_xact_lock(hashtextextended('lcn-mpr|' || v_key, 0));
    SELECT o.state INTO ex FROM integration.operations o WHERE o.tenant_id = v_t AND o.store_id = v_s AND o.semantic_key = v_key;
    IF FOUND THEN
        IF ex.state IN ('BLOCKED_POLICY', 'STALE_BINDING', 'CANCELLED') THEN
            -- provably unsent (zero HTTP calls): one manual reply may use the second key class, at most once per comment.
            v_key := v_key || ':m1';
            IF EXISTS (SELECT 1 FROM integration.operations o WHERE o.tenant_id = v_t AND o.store_id = v_s AND o.semantic_key = v_key) THEN
                RAISE EXCEPTION 'used' USING ERRCODE = 'PT409';
            END IF;
        ELSE
            RAISE EXCEPTION 'used' USING ERRCODE = 'PT409';
        END IF;
    END IF;
    v_live := src.platform = 'instagram';
    -- A1.1 clause 3: the intake row may not be staged yet inside the first 120 s, so the FOR SHARE yield cannot fire.
    IF src.private_reply AND p_comment_created_at > v_now - interval '120 seconds'
       AND EXISTS (SELECT 1 FROM live.claim_windows w WHERE w.tenant_id = v_t AND w.store_id = v_s AND w.session_id = p_session AND w.state = 'OPEN') THEN
        IF NOT p_confirm_preempt THEN RAISE EXCEPTION 'auto_pending_confirm' USING ERRCODE = 'PT409'; END IF;
        v_preempt := true;
    END IF;
    v_deadline := p_comment_created_at + interval '7 days' - interval '1 hour';
    IF v_deadline <= v_now THEN RAISE EXCEPTION 'expired_7d' USING ERRCODE = 'PT409'; END IF;
    IF v_live THEN
        IF p_comment_created_at + interval '15 minutes' <= v_now
           OR NOT EXISTS (SELECT 1 FROM live.claim_windows w WHERE w.tenant_id = v_t AND w.store_id = v_s AND w.session_id = p_session AND w.state = 'OPEN') THEN
            RAISE EXCEPTION 'ig_live_ended' USING ERRCODE = 'PT409';
        END IF;
        v_deadline := least(v_deadline, p_comment_created_at + interval '15 minutes');
    END IF;
    -- §3.6: a manual private reply to a comment whose peer already has a conversation takes that conversation over.
    IF ir.applied_event_id IS NOT NULL THEN
        SELECT e.bundle_id INTO v_bundle FROM claims.events e WHERE e.tenant_id = v_t AND e.store_id = v_s AND e.id = ir.applied_event_id;
        IF v_bundle IS NOT NULL THEN
            SELECT c.id INTO v_conv FROM inbox.bundle_peers bp JOIN social.conversations c
              ON c.tenant_id = bp.tenant_id AND c.store_id = bp.store_id AND c.app_id = bp.app_id AND c.object = bp.object
             AND c.asset_id = bp.asset_id AND c.peer_key = bp.peer_key
             WHERE bp.tenant_id = v_t AND bp.store_id = v_s AND bp.bundle_id = v_bundle AND bp.object = src.object AND bp.asset_id = src.asset_id
             ORDER BY bp.created_at DESC LIMIT 1;
        END IF;
    END IF;
    IF v_conv IS NOT NULL THEN
        SELECT s.mode, s.assignee_principal, s.takeover_generation, s.human_until INTO st FROM inbox.conversation_state s
         WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.conversation_id = v_conv FOR UPDATE;
        IF FOUND THEN
            v_known := true;
            IF st.mode = 'human' AND st.human_until IS NOT NULL AND v_now >= st.human_until THEN
                st.mode := 'auto'; st.assignee_principal := NULL; st.takeover_generation := st.takeover_generation + 1;
            END IF;
            v_gen := st.takeover_generation;
            IF st.mode <> 'human' OR st.assignee_principal IS DISTINCT FROM v_p THEN
                v_gen := v_gen + 1;
                INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (v_t, v_s, v_p, 'inbox.takeover');
            END IF;
            UPDATE inbox.conversation_state s SET mode = 'human', assignee_principal = v_p, takeover_generation = v_gen,
                   last_human_outbound_at = v_now, human_until = v_now + interval '6 hours', version = s.version + 1, updated_at = v_now
             WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.conversation_id = v_conv;
        ELSE
            v_conv := NULL;
        END IF;
    END IF;
    SELECT z.id, z.semantic_version INTO b FROM integration.bindings z
     WHERE z.tenant_id = v_t AND z.store_id = v_s AND z.id = src.binding_id AND z.enabled FOR SHARE;
    IF NOT FOUND OR coalesce(integration.binding_capability_state(v_t, v_s, b.id, 'private_reply', ARRAY[]::text[]), 'unknown')
                    NOT IN ('ok', 'review_required') THEN
        RAISE EXCEPTION 'capability' USING ERRCODE = 'PT409';
    END IF;
    SELECT r.app_id INTO v_app FROM meta_inbox.routes r
     WHERE r.tenant_id = v_t AND r.store_id = v_s AND r.object = src.object AND r.asset_id = src.asset_id AND r.enabled ORDER BY r.app_id LIMIT 1;
    v_req := jsonb_build_object('v', 1, 'kind', 'private_reply', 'message_type', 'manual_private_reply', 'origin', 'human',
        'platform', src.platform, 'asset_id', src.asset_id, 'comment_ref', p_comment_ref,
        'comment_created_at', to_char(p_comment_created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
        'session_id', p_session, 'source_id', src.id, 'outbound_id', p_outbound, 'body_hmac', v_hmac, 'policy', 'lcn-policy/v1',
        'takeover_generation', v_gen, 'conversation_known', v_known, 'principal_id', v_p, 'live_media', v_live,
        'deadline_at', to_char(v_deadline AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
    IF v_app IS NOT NULL THEN v_req := v_req || jsonb_build_object('app_id', v_app); END IF;
    IF v_conv IS NOT NULL THEN v_req := v_req || jsonb_build_object('conversation_id', v_conv); END IF;
    IF v_bundle IS NOT NULL THEN v_req := v_req || jsonb_build_object('bundle_id', v_bundle); END IF;
    IF p_template_id IS NOT NULL THEN v_req := v_req || jsonb_build_object('template_id', p_template_id, 'template_version', p_template_version); END IF;
    IF v_preempt THEN
        INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (v_t, v_s, v_p, 'inbox.private_reply.preempt_confirmed');
    END IF;
    BEGIN
        PERFORM inbox.lcn_emit(v_t, v_s, v_p, 'private_reply', 'meta.private_reply', b.id, b.semantic_version, src.platform, src.asset_id,
            v_key, v_req, p_operation, p_job, p_outbound, NULL, p_comment_ref, p_body_hmac, p_display_key_id, p_display_nonce,
            p_display_ciphertext, p_secret_enc, p_secret_sealed, p_template_id, p_template_version, 'inbox.private_reply.planned');
    EXCEPTION WHEN unique_violation THEN
        -- The unique mpr: index is the final serialiser (a writer that bypassed the advisory key).
        RAISE EXCEPTION 'used' USING ERRCODE = 'PT409';
    END;
    RETURN p_operation;
END $$;
ALTER FUNCTION inbox.plan_manual_private_reply(uuid, text, timestamptz, boolean, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.plan_manual_private_reply(uuid, text, timestamptz, boolean, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.plan_manual_private_reply(uuid, text, timestamptz, boolean, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- inbox.plan_public_reply (A5; locks: lcn-dup -> binding FOR SHARE). Content is validated in Go before sealing.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.plan_public_reply(p_session uuid, p_comment_ref text, p_operation uuid, p_job bigint, p_outbound uuid,
 p_body_hmac bytea, p_display_key_id text, p_display_nonce bytea, p_display_ciphertext bytea, p_secret_enc bytea, p_secret_sealed bytea,
 p_template_id text, p_template_version bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; src record; b record; v_now timestamptz := clock_timestamp(); v_hmac text; v_req jsonb; v_app text;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_session IS NULL OR p_comment_ref IS NULL OR p_comment_ref !~ '^[0-9_]{1,80}$' OR p_body_hmac IS NULL
       OR octet_length(p_body_hmac) <> 32 OR (p_template_id IS NULL) <> (p_template_version IS NULL) THEN
        RAISE EXCEPTION 'invalid public reply plan' USING ERRCODE = '22023';
    END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    SELECT s.id, s.platform, s.object, s.asset_id, s.binding_id INTO src FROM live.claim_sources s
     WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.session_id = p_session AND s.active ORDER BY s.created_at DESC LIMIT 1;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    v_hmac := encode(p_body_hmac, 'hex');
    PERFORM pg_advisory_xact_lock(hashtextextended('lcn-dup|' || v_s::text || '|' || p_comment_ref || '|' || v_hmac, 0));
    IF EXISTS (SELECT 1 FROM integration.operations o WHERE o.tenant_id = v_t AND o.store_id = v_s AND o.action = 'meta.public_reply'
                AND o.request->>'comment_ref' = p_comment_ref AND o.request->>'body_hmac' = v_hmac
                AND o.created_at > v_now - interval '30 seconds') THEN
        RAISE EXCEPTION 'duplicate_recent' USING ERRCODE = 'PT409';
    END IF;
    PERFORM inbox.lcn_rate_check(v_t, v_s);
    IF src.platform = 'instagram' THEN RAISE EXCEPTION 'ig_live_unsupported' USING ERRCODE = 'PT409'; END IF;
    SELECT z.id, z.semantic_version INTO b FROM integration.bindings z
     WHERE z.tenant_id = v_t AND z.store_id = v_s AND z.id = src.binding_id AND z.enabled FOR SHARE;
    IF NOT FOUND OR coalesce(integration.binding_capability_state(v_t, v_s, b.id, 'reply_public', ARRAY[]::text[]), 'unknown')
                    NOT IN ('ok', 'review_required') THEN
        RAISE EXCEPTION 'capability' USING ERRCODE = 'PT409';
    END IF;
    v_req := jsonb_build_object('v', 1, 'kind', 'public_reply', 'message_type', 'public_reply', 'origin', 'human',
        'platform', src.platform, 'asset_id', src.asset_id, 'comment_ref', p_comment_ref, 'session_id', p_session,
        'outbound_id', p_outbound, 'body_hmac', v_hmac, 'policy', 'lcn-policy/v1', 'principal_id', v_p,
        'deadline_at', to_char((v_now + interval '15 minutes') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
    IF p_template_id IS NOT NULL THEN v_req := v_req || jsonb_build_object('template_id', p_template_id, 'template_version', p_template_version); END IF;
    PERFORM inbox.lcn_emit(v_t, v_s, v_p, 'public_reply', 'meta.public_reply', b.id, b.semantic_version, src.platform, src.asset_id,
        'mpub:' || substr(encode(sha256(convert_to(src.object || '|' || src.asset_id || '|' || p_comment_ref || '|' || p_operation::text, 'UTF8')), 'hex'), 1, 48),
        v_req, p_operation, p_job, p_outbound, NULL, p_comment_ref, p_body_hmac, p_display_key_id, p_display_nonce,
        p_display_ciphertext, p_secret_enc, p_secret_sealed, p_template_id, p_template_version, 'inbox.public_reply.planned');
    RETURN p_operation;
END $$;
ALTER FUNCTION inbox.plan_public_reply(uuid, text, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.plan_public_reply(uuid, text, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.plan_public_reply(uuid, text, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- live.plan_offer_recommend (A6 post_comment; locks: lcn-rec -> offer FOR SHARE -> binding FOR SHARE). FB only.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION live.plan_offer_recommend(p_session uuid, p_offer uuid, p_expected_version bigint, p_operation uuid, p_job bigint,
 p_outbound uuid, p_body_hmac bytea, p_display_key_id text, p_display_nonce bytea, p_display_ciphertext bytea, p_secret_enc bytea,
 p_secret_sealed bytea, p_template_id text, p_template_version bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; o record; src record; b record; v_now timestamptz := clock_timestamp(); v_hmac text; v_req jsonb;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_session IS NULL OR p_offer IS NULL OR p_expected_version IS NULL OR p_expected_version < 1 OR p_body_hmac IS NULL
       OR octet_length(p_body_hmac) <> 32 OR (p_template_id IS NULL) <> (p_template_version IS NULL) THEN
        RAISE EXCEPTION 'invalid recommend plan' USING ERRCODE = '22023';
    END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['live:manage', 'inbox:reply']) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    -- A1.3 clause 2: at most one recommend comment per offer per 10 min, serialised in the database.
    PERFORM pg_advisory_xact_lock(hashtextextended('lcn-rec|' || v_s::text || '|' || p_offer::text, 0));
    IF EXISTS (SELECT 1 FROM integration.operations x WHERE x.tenant_id = v_t AND x.store_id = v_s AND x.action = 'meta.offer_recommend'
                AND x.request->>'offer_id' = p_offer::text AND x.created_at > v_now - interval '10 minutes') THEN
        RAISE EXCEPTION 'duplicate_recent' USING ERRCODE = 'PT409';
    END IF;
    PERFORM inbox.lcn_rate_check(v_t, v_s);
    SELECT f.id, f.active, f.version INTO o FROM live.offers f
     WHERE f.tenant_id = v_t AND f.store_id = v_s AND f.session_id = p_session AND f.id = p_offer FOR SHARE;
    IF NOT FOUND OR NOT o.active THEN RAISE EXCEPTION 'offer_unavailable' USING ERRCODE = 'PT409'; END IF;
    IF o.version <> p_expected_version THEN RAISE EXCEPTION 'version_conflict' USING ERRCODE = 'PT409'; END IF;
    SELECT s.id, s.platform, s.asset_id, s.binding_id, s.source_object_id INTO src FROM live.claim_sources s
     WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.session_id = p_session AND s.active ORDER BY s.created_at DESC LIMIT 1;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    IF src.platform = 'instagram' THEN RAISE EXCEPTION 'ig_live_unsupported' USING ERRCODE = 'PT422'; END IF;
    SELECT z.id, z.semantic_version INTO b FROM integration.bindings z
     WHERE z.tenant_id = v_t AND z.store_id = v_s AND z.id = src.binding_id AND z.enabled FOR SHARE;
    IF NOT FOUND OR coalesce(integration.binding_capability_state(v_t, v_s, b.id, 'reply_public', ARRAY[]::text[]), 'unknown')
                    NOT IN ('ok', 'review_required') THEN
        RAISE EXCEPTION 'capability' USING ERRCODE = 'PT409';
    END IF;
    v_hmac := encode(p_body_hmac, 'hex');
    v_req := jsonb_build_object('v', 1, 'kind', 'recommend', 'message_type', 'offer_recommend', 'origin', 'human',
        'platform', src.platform, 'asset_id', src.asset_id, 'session_id', p_session, 'offer_id', p_offer,
        'live_object_id', src.source_object_id, 'outbound_id', p_outbound, 'body_hmac', v_hmac, 'policy', 'lcn-policy/v1',
        'principal_id', v_p,
        'deadline_at', to_char((v_now + interval '15 minutes') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
    IF p_template_id IS NOT NULL THEN v_req := v_req || jsonb_build_object('template_id', p_template_id, 'template_version', p_template_version); END IF;
    PERFORM inbox.lcn_emit(v_t, v_s, v_p, 'recommend', 'meta.offer_recommend', b.id, b.semantic_version, src.platform, src.asset_id,
        'mrec:' || substr(encode(sha256(convert_to(p_session::text || '|' || p_offer::text || '|' || p_operation::text, 'UTF8')), 'hex'), 1, 48),
        v_req, p_operation, p_job, p_outbound, NULL, NULL, p_body_hmac, p_display_key_id, p_display_nonce,
        p_display_ciphertext, p_secret_enc, p_secret_sealed, p_template_id, p_template_version, 'live.offer.recommended');
    RETURN p_operation;
END $$;
ALTER FUNCTION live.plan_offer_recommend(uuid, uuid, bigint, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION live.plan_offer_recommend(uuid, uuid, bigint, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.plan_offer_recommend(uuid, uuid, bigint, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) TO commerce_runtime;

-- Product facts for the fixed offer-recommend/v1 render (product name, keyword, price); no buyer data.
GRANT SELECT(tenant_id, store_id, id, name) ON catalog.products TO commerce_integration_writer;
GRANT SELECT(tenant_id, store_id, id, product_id, code, price_minor, currency) ON catalog.skus TO commerce_integration_writer;
CREATE POLICY product_send_read ON catalog.products FOR SELECT TO commerce_integration_writer USING (inbox.lcn_in_scope(tenant_id, store_id));
CREATE POLICY sku_send_read ON catalog.skus FOR SELECT TO commerce_integration_writer USING (inbox.lcn_in_scope(tenant_id, store_id));
CREATE FUNCTION live.offer_recommend_facts(p_session uuid, p_offer uuid)
RETURNS TABLE(keyword text, product_name text, sku_code text, price_minor bigint, currency text, version bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
BEGIN
    v := inbox.lcn_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:manage']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    RETURN QUERY
    SELECT f.keyword, p.name, k.code, k.price_minor, k.currency, f.version
      FROM live.offers f JOIN catalog.skus k ON k.tenant_id = f.tenant_id AND k.store_id = f.store_id AND k.id = f.sku_id
      JOIN catalog.products p ON p.tenant_id = k.tenant_id AND p.store_id = k.store_id AND p.id = k.product_id
     WHERE f.tenant_id = v[1] AND f.store_id = v[2] AND f.session_id = p_session AND f.id = p_offer;
END $$;
ALTER FUNCTION live.offer_recommend_facts(uuid, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION live.offer_recommend_facts(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.offer_recommend_facts(uuid, uuid) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- Worker side (commerce_claims_worker, no GUCs): Check, secret loader, Finish.
-- ---------------------------------------------------------------------------------------
-- §4.3 Check: lock-free, no network. Returns '' ... no: OK or one fixed deny code. The frozen operation row is the only input; the
-- tenant/store GUCs are pinned for this statement so the scoped table policies of commerce_integration_writer apply.
CREATE FUNCTION inbox.check_send(p_operation uuid) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE o record; r jsonb; v_now timestamptz := clock_timestamp(); v_cap text; st record; w record; v_key text; v_conv uuid;
 v_perms text[]; v_deadline timestamptz; v_ref text; v_object text;
BEGIN
    SELECT x.tenant_id, x.store_id, x.action, x.provider, x.binding_id, x.external_asset_id, x.request INTO o
      FROM integration.operations x WHERE x.id = p_operation AND x.actor_kind = 'MERCHANT'
       AND x.action IN ('meta.dm_send', 'meta.private_reply', 'meta.public_reply', 'meta.offer_recommend');
    IF NOT FOUND THEN RETURN 'invalid_request'; END IF;
    r := o.request;
    IF jsonb_typeof(r) <> 'object' OR r->>'policy' IS DISTINCT FROM 'lcn-policy/v1' THEN RETURN 'invalid_request'; END IF;
    PERFORM set_config('app.tenant_id', o.tenant_id::text, true), set_config('app.store_id', o.store_id::text, true);
    v_deadline := (r->>'deadline_at')::timestamptz;
    IF v_now >= v_deadline THEN
        RETURN CASE o.action WHEN 'meta.dm_send' THEN 'window_closed' ELSE 'deadline' END;
    END IF;
    v_cap := CASE o.action WHEN 'meta.dm_send' THEN 'dm_session' WHEN 'meta.private_reply' THEN 'private_reply' ELSE 'reply_public' END;
    IF coalesce(integration.binding_capability_state(o.tenant_id, o.store_id, o.binding_id, v_cap, ARRAY[]::text[]), 'unknown')
       NOT IN ('ok', 'review_required') THEN
        RETURN 'capability';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM inbox.outbound_messages m WHERE m.tenant_id = o.tenant_id AND m.store_id = o.store_id AND m.operation_id = p_operation) THEN
        RETURN 'outbound_missing';
    END IF;
    IF r->>'origin' = 'human' THEN
        v_perms := CASE WHEN o.action = 'meta.offer_recommend' THEN ARRAY['live:manage', 'inbox:reply'] ELSE ARRAY['inbox:reply'] END;
        IF NOT identity.principal_holds(o.tenant_id, o.store_id, (r->>'principal_id')::uuid, v_perms) THEN RETURN 'principal_revoked'; END IF;
    END IF;
    IF o.action = 'meta.dm_send' THEN
        v_conv := (r->>'conversation_id')::uuid;
        SELECT s.last_inbound_at,
               CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND v_now >= s.human_until THEN 'auto' ELSE s.mode END AS mode,
               CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND v_now >= s.human_until THEN s.takeover_generation + 1 ELSE s.takeover_generation END AS gen
          INTO st FROM inbox.conversation_state s WHERE s.tenant_id = o.tenant_id AND s.store_id = o.store_id AND s.conversation_id = v_conv;
        IF NOT FOUND THEN RETURN 'conversation_gone'; END IF;
        IF NOT EXISTS (SELECT 1 FROM social.conversations c WHERE c.tenant_id = o.tenant_id AND c.store_id = o.store_id AND c.id = v_conv) THEN
            RETURN 'conversation_gone';
        END IF;
        IF st.last_inbound_at IS NULL OR st.last_inbound_at + interval '24 hours' - interval '5 minutes' <= v_now THEN RETURN 'window_closed'; END IF;
        IF r->>'origin' = 'auto' THEN
            IF st.mode = 'human' THEN RETURN 'human_takeover'; END IF;
            IF st.gen <> (r->>'takeover_generation')::bigint THEN RETURN 'takeover_changed'; END IF;
        END IF;
    ELSIF o.action = 'meta.private_reply' THEN
        IF r->>'message_type' IS DISTINCT FROM 'manual_private_reply' THEN RETURN 'invalid_request'; END IF;
        v_object := CASE r->>'platform' WHEN 'facebook' THEN 'page' ELSE 'instagram' END;
        v_ref := r->>'comment_ref';
        v_key := 'mpr:' || substr(encode(sha256(convert_to(v_object || '|' || (r->>'asset_id') || '|' || v_ref, 'UTF8')), 'hex'), 1, 48);
        -- comment budget: no OTHER operation of this comment may have used the one private reply.
        IF EXISTS (SELECT 1 FROM integration.operations y WHERE y.tenant_id = o.tenant_id AND y.store_id = o.store_id AND y.id <> p_operation
                    AND y.semantic_key IN (v_key, v_key || ':m1') AND y.state IN ('SUCCEEDED', 'ACKNOWLEDGED', 'UNKNOWN', 'FAILED_FINAL')) THEN
            RETURN 'used';
        END IF;
        IF (r->>'live_media')::boolean AND NOT EXISTS (SELECT 1 FROM live.claim_windows cw WHERE cw.tenant_id = o.tenant_id AND cw.store_id = o.store_id
                AND cw.session_id = (r->>'session_id')::uuid AND cw.state = 'OPEN') THEN
            RETURN 'ig_live_ended';
        END IF;
    ELSIF o.action = 'meta.offer_recommend' THEN
        IF NOT EXISTS (SELECT 1 FROM live.offers f WHERE f.tenant_id = o.tenant_id AND f.store_id = o.store_id
                        AND f.id = (r->>'offer_id')::uuid AND f.active) THEN
            RETURN 'offer_unavailable';
        END IF;
    END IF;
    RETURN 'OK';
END $$;
ALTER FUNCTION inbox.check_send(uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.check_send(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.check_send(uuid) TO commerce_claims_worker;

-- §3.4 lease-fenced loader of the sealed dispatch copy; zero rows when the operation has none (auto claim-link replies).
CREATE FUNCTION inbox.load_send_secret(p_operation uuid, p_generation bigint, p_lease_token bytea)
RETURNS TABLE(tenant_id uuid, store_id uuid, sealed bytea, enc bytea)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE o record;
BEGIN
    IF p_operation IS NULL OR p_generation IS NULL OR p_generation < 1 OR p_lease_token IS NULL OR octet_length(p_lease_token) <> 32 THEN
        RAISE EXCEPTION 'invalid send secret load' USING ERRCODE = '22023';
    END IF;
    SELECT x.tenant_id, x.store_id, x.state, x.lease_mode, x.generation, x.lease_until, x.lease_token_hash INTO o
      FROM integration.operations x WHERE x.id = p_operation
       AND x.action IN ('meta.dm_send', 'meta.private_reply', 'meta.public_reply', 'meta.offer_recommend') FOR SHARE;
    IF NOT FOUND OR o.state <> 'DISPATCHING' OR o.lease_mode <> 'dispatch' OR o.generation <> p_generation OR o.lease_until IS NULL
       OR o.lease_until <= clock_timestamp() OR o.lease_token_hash IS DISTINCT FROM sha256(p_lease_token) THEN
        RAISE EXCEPTION 'send secret lease conflict' USING ERRCODE = '40001';
    END IF;
    RETURN QUERY SELECT k.tenant_id, k.store_id, k.sealed, k.enc FROM inbox.send_secrets k
     WHERE k.operation_id = p_operation AND k.tenant_id = o.tenant_id AND k.store_id = o.store_id;
END $$;
ALTER FUNCTION inbox.load_send_secret(uuid, bigint, bytea) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.load_send_secret(uuid, bigint, bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.load_send_secret(uuid, bigint, bytea) TO commerce_claims_worker;

-- §4.3 Finish hook (runs inside the completion transaction on EVERY completion path): wipes the dispatch copy (SUCCEEDED, FAILED_FINAL,
-- BLOCKED_POLICY, STALE_BINDING, UNKNOWN), advances last_outbound_at (DM) and records the bundle<->peer link of a SUCCEEDED private reply.
-- p_peer_key is computed in Go (meta.SocialPeerKey) from the Send API's recipient_id; NULL when absent.
CREATE FUNCTION inbox.finish_send(p_operation uuid, p_generation bigint, p_lease_token bytea, p_outcome text, p_peer_key text)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE; r jsonb; v_bundle uuid; v_object text;
BEGIN
    IF p_operation IS NULL OR p_generation IS NULL OR p_generation < 1 OR p_lease_token IS NULL OR octet_length(p_lease_token) <> 32
       OR p_outcome IS NULL OR p_outcome NOT IN ('SUCCEEDED','FAILED_FINAL','UNKNOWN','BLOCKED_POLICY','CANCELLED','ACKNOWLEDGED','STALE_BINDING')
       OR (p_peer_key IS NOT NULL AND p_peer_key !~ '^[0-9a-f]{64}$') THEN
        RAISE EXCEPTION 'invalid send finish' USING ERRCODE = '22023';
    END IF;
    SELECT x.* INTO o FROM integration.operations x WHERE x.id = p_operation
       AND x.action IN ('meta.dm_send', 'meta.private_reply', 'meta.public_reply', 'meta.offer_recommend');
    IF NOT FOUND THEN RAISE EXCEPTION 'send operation unavailable' USING ERRCODE = 'P0002'; END IF;
    IF o.generation <> p_generation OR o.lease_until IS NULL OR o.lease_until <= clock_timestamp()
       OR o.lease_token_hash IS DISTINCT FROM sha256(p_lease_token) OR o.state NOT IN ('DISPATCHING', 'UNKNOWN') THEN
        RAISE EXCEPTION 'send finish lease conflict' USING ERRCODE = '40001';
    END IF;
    PERFORM set_config('app.tenant_id', o.tenant_id::text, true), set_config('app.store_id', o.store_id::text, true);
    r := o.request;
    DELETE FROM inbox.send_secrets k WHERE k.operation_id = p_operation;
    IF p_outcome <> 'SUCCEEDED' THEN RETURN; END IF;
    IF o.action = 'meta.dm_send' THEN
        UPDATE inbox.conversation_state s SET last_outbound_at = clock_timestamp(), updated_at = clock_timestamp()
         WHERE s.tenant_id = o.tenant_id AND s.store_id = o.store_id AND s.conversation_id = (r->>'conversation_id')::uuid;
    ELSIF o.action = 'meta.private_reply' AND p_peer_key IS NOT NULL AND r->>'app_id' ~ '^[0-9]{1,40}$' THEN
        v_object := CASE r->>'platform' WHEN 'facebook' THEN 'page' ELSE 'instagram' END;
        v_bundle := nullif(r->>'bundle_id', '')::uuid;
        IF v_bundle IS NOT NULL THEN
            INSERT INTO inbox.bundle_peers(tenant_id, store_id, bundle_id, peer_key, app_id, object, asset_id, operation_id)
            VALUES (o.tenant_id, o.store_id, v_bundle, p_peer_key, r->>'app_id', v_object, r->>'asset_id', p_operation)
            ON CONFLICT (tenant_id, store_id, bundle_id, peer_key) DO NOTHING;
        END IF;
    END IF;
END $$;
ALTER FUNCTION inbox.finish_send(uuid, bigint, bytea, text, text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.finish_send(uuid, bigint, bytea, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.finish_send(uuid, bigint, bytea, text, text) TO commerce_claims_worker;

-- P2-5: automated sends resolve the thread through the bundle's peers; only peers of the operation's own platform/asset match.
CREATE FUNCTION inbox.dm_window_for_bundle(p_tenant uuid, p_store uuid, p_bundle uuid, p_app_id text, p_object text, p_asset_id text)
RETURNS TABLE(last_inbound_at timestamptz, mode text, takeover_generation bigint, human_until timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT s.last_inbound_at,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until THEN 'auto' ELSE s.mode END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN s.takeover_generation + 1 ELSE s.takeover_generation END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until THEN NULL ELSE s.human_until END
      FROM inbox.bundle_peers bp
      JOIN social.conversations c ON c.tenant_id = bp.tenant_id AND c.store_id = bp.store_id AND c.app_id = bp.app_id
       AND c.object = bp.object AND c.asset_id = bp.asset_id AND c.peer_key = bp.peer_key
      JOIN inbox.conversation_state s ON s.tenant_id = c.tenant_id AND s.store_id = c.store_id AND s.conversation_id = c.id
     WHERE bp.tenant_id = p_tenant AND bp.store_id = p_store AND bp.bundle_id = p_bundle
       AND bp.app_id = p_app_id AND bp.object = p_object AND bp.asset_id = p_asset_id
     ORDER BY bp.created_at DESC LIMIT 1
$$;
ALTER FUNCTION inbox.dm_window_for_bundle(uuid, uuid, uuid, text, text, text) OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION inbox.dm_window_for_bundle(uuid, uuid, uuid, text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.dm_window_for_bundle(uuid, uuid, uuid, text, text, text)
    TO commerce_claims_worker, commerce_claims_writer, commerce_integration_writer;

-- ---------------------------------------------------------------------------------------
-- §14.1 clauses 2-3: the automatic claim-link reply. claim_reply_plannable gains skip code reply_used (an existing manual /
-- out-of-stock mpr: operation), serialised with the manual planner on the same advisory key; plan_claim_reply freezes the
-- takeover fields; claims.check_meta_reply denies human_takeover / takeover_changed.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION integration.claim_reply_plannable(p_intake uuid) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE i record; s0 record; s record; b record; v_code text; v_key text; v_bundle uuid; v_principal uuid;
BEGIN
 IF p_intake IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid claim reply check' USING ERRCODE='22023';
 END IF;
 SELECT x.tenant_id,x.store_id,x.source_id,x.object,x.asset_id,x.comment_ref,x.inbox_event_id INTO i FROM claims.meta_intake x
  WHERE x.id=p_intake AND x.lease_xid=pg_current_xact_id() AND x.state='PENDING';
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply check' USING ERRCODE='22023'; END IF;
 -- The intake row is already locked by this transaction (lease); the same advisory key the manual planner takes after its
 -- FOR SHARE on this row (live-console-v1 §4.2): order intake -> lcn-mpr -> binding -> source on both sides.
 v_key:='mpr:'||substr(encode(sha256(convert_to(i.object||'|'||i.asset_id||'|'||i.comment_ref,'UTF8')),'hex'),1,48);
 PERFORM pg_advisory_xact_lock(hashtextextended('lcn-mpr|'||v_key,0));
 SELECT y.binding_id INTO s0 FROM live.claim_sources y WHERE y.tenant_id=i.tenant_id AND y.store_id=i.store_id AND y.id=i.source_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply check' USING ERRCODE='22023'; END IF;
 -- Binding before source (lock order §9, as the consumer).
 SELECT z.id,z.provider,z.external_asset_id,z.enabled INTO b FROM integration.bindings z
  WHERE z.tenant_id=i.tenant_id AND z.store_id=i.store_id AND z.id=s0.binding_id FOR SHARE;
 SELECT y.principal_id,y.binding_id,y.asset_id,y.platform,y.active,y.private_reply INTO s FROM live.claim_sources y
  WHERE y.tenant_id=i.tenant_id AND y.store_id=i.store_id AND y.id=i.source_id FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply check' USING ERRCODE='22023'; END IF;
 IF NOT s.active OR NOT s.private_reply THEN v_code:='source_off';
 ELSIF b.id IS NULL OR NOT b.enabled THEN v_code:='binding_disabled';
 ELSIF s.binding_id<>s0.binding_id OR b.provider<>s.platform OR b.external_asset_id<>s.asset_id THEN v_code:='binding_changed';
 ELSIF NOT EXISTS(SELECT 1 FROM control.storefront_domains d JOIN control.storefront_publications p
   ON p.tenant_id=d.tenant_id AND p.store_id=d.store_id AND p.published
   WHERE d.tenant_id=i.tenant_id AND d.store_id=i.store_id AND d.state='ACTIVE') THEN v_code:='no_storefront';
 ELSIF EXISTS(SELECT 1 FROM integration.operations o WHERE o.tenant_id=i.tenant_id AND o.store_id=i.store_id AND o.semantic_key=v_key
   AND o.request->>'message_type' IN ('manual_private_reply','out_of_stock_reply')) THEN
  -- §14.1 clause 2: the comment's one private reply was already used by a manual / out-of-stock reply. The claim commits, no
  -- operation, and the bundle is flagged so the console asks the merchant to send the link in a DM (A1.1).
  v_code:='reply_used';
  SELECT e.bundle_id INTO v_bundle FROM claims.events e WHERE e.tenant_id=i.tenant_id AND e.store_id=i.store_id
   AND e.source_event_id=i.inbox_event_id AND e.outcome='ACCEPTED' AND e.bundle_version=1;
  IF v_bundle IS NOT NULL THEN
   UPDATE claims.bundles k SET link_pending_manual=true WHERE k.tenant_id=i.tenant_id AND k.store_id=i.store_id AND k.id=v_bundle;
  END IF;
 ELSE RETURN 'OK';
 END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
  VALUES(i.tenant_id,i.store_id,s.principal_id,'claim_reply_skipped:'||v_code);
 RETURN v_code;
END $$;

CREATE OR REPLACE FUNCTION integration.plan_claim_reply(p_intake uuid,p_operation uuid,p_link_hash bytea,p_link_key_id text,p_job bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE i record; ev record; src record; b record; j record; v_domain uuid; v_origin text; v_expires timestamptz;
 v_deadline timestamptz; v_request jsonb; v_key text; w record; v_known boolean:=false; v_gen bigint:=0;
BEGIN
 IF p_intake IS NULL OR p_operation IS NULL OR p_link_hash IS NULL OR octet_length(p_link_hash)<>32
  OR p_link_key_id IS NULL OR p_link_key_id !~ '^[0-9a-f]{16}$' OR p_job IS NULL OR p_job<=0
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023';
 END IF;
 SELECT x.tenant_id,x.store_id,x.inbox_event_id,x.source_id,x.session_id,x.platform,x.object,x.asset_id,x.comment_ref,
  x.live_media,x.occurred_at,x.received_at,x.app_id INTO i FROM claims.meta_intake x
  WHERE x.id=p_intake AND x.lease_xid=pg_current_xact_id() AND x.state='PENDING';
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 -- Only the ACCEPTED event that CREATED the bundle plans a reply (bundle_version 1, IR-3).
 SELECT e.id,e.bundle_id INTO ev FROM claims.events e WHERE e.tenant_id=i.tenant_id AND e.store_id=i.store_id
  AND e.source_event_id=i.inbox_event_id AND e.outcome='ACCEPTED' AND e.bundle_version=1;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 SELECT y.id,y.binding_id,y.platform,y.asset_id,y.reply_locale,y.principal_id,y.active,y.private_reply INTO src
  FROM live.claim_sources y WHERE y.tenant_id=i.tenant_id AND y.store_id=i.store_id AND y.id=i.source_id;
 SELECT z.id,z.provider,z.external_asset_id,z.semantic_version,z.enabled INTO b FROM integration.bindings z
  WHERE z.tenant_id=i.tenant_id AND z.store_id=i.store_id AND z.id=src.binding_id;
 -- claim_reply_plannable held the locks since; any failure here is an invariant breach, not a normal path.
 IF src.id IS NULL OR NOT src.active OR NOT src.private_reply OR b.id IS NULL OR NOT b.enabled
  OR b.provider<>src.platform OR b.external_asset_id<>src.asset_id OR b.provider<>i.platform THEN
  RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023';
 END IF;
 SELECT d.id,d.origin INTO v_domain,v_origin FROM control.storefront_domains d JOIN control.storefront_publications p
  ON p.tenant_id=d.tenant_id AND p.store_id=d.store_id AND p.published
  WHERE d.tenant_id=i.tenant_id AND d.store_id=i.store_id AND d.state='ACTIVE' ORDER BY d.id LIMIT 1;
 IF v_domain IS NULL THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 -- The River job must be exactly the row inserted in this transaction (xmin needs table-level SELECT).
 SELECT r.id INTO j FROM river.river_job r WHERE r.id=p_job AND r.kind='external_operation_v1' AND r.queue='default'
  AND r.unique_key IS NULL AND r.args=jsonb_build_object('operation_id',p_operation::text,'version',1)
  AND r.xmin=pg_current_xact_id()::xid;
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 -- §3.6 / P2-3: freeze the EFFECTIVE takeover generation of the actor's known conversation (via bundle peers), else 0.
 SELECT t.takeover_generation INTO w FROM inbox.dm_window_for_bundle(i.tenant_id,i.store_id,ev.bundle_id,i.app_id,i.object,i.asset_id) t;
 IF FOUND THEN v_known:=true; v_gen:=w.takeover_generation; END IF;
 v_expires:=claims.issue_system_link(p_intake,p_link_hash);
 v_deadline:=least(i.occurred_at+interval '7 days'-interval '1 hour',v_expires-interval '10 minutes',
  CASE WHEN i.live_media THEN i.received_at+interval '15 minutes' END);
 v_request:=jsonb_build_object('v',1,'platform',i.platform,'source_id',i.source_id,'asset_id',i.asset_id,
  'comment_ref',i.comment_ref,'bundle_id',ev.bundle_id,'session_id',i.session_id,'link_generation',1,
  'link_key_id',p_link_key_id,'locale',src.reply_locale,'template','claim-link/v1','policy','mpr-policy/v1',
  'message_type','first_private_reply','origin_kind','auto','conversation_known',v_known,'takeover_generation',v_gen,
  'app_id',i.app_id,'origin_ref',v_domain,'origin',v_origin,
  'deadline_at',to_char(v_deadline AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),'live_media',i.live_media);
 IF octet_length(v_request::text)>2048 THEN RAISE EXCEPTION 'invalid claim reply plan' USING ERRCODE='22023'; END IF;
 v_key:='mpr:'||substr(encode(sha256(convert_to(i.object||'|'||i.asset_id||'|'||i.comment_ref,'UTF8')),'hex'),1,48);
 INSERT INTO integration.operations(tenant_id,store_id,id,principal_id,binding_id,binding_version,provider,external_asset_id,
  purpose,action,semantic_key,request_hash,request,job_id)
 VALUES(i.tenant_id,i.store_id,p_operation,src.principal_id,b.id,b.semantic_version,b.provider,b.external_asset_id,
  'service','meta.private_reply',v_key,sha256(convert_to(v_request::text,'UTF8')),v_request,p_job);
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(i.tenant_id,i.store_id,p_operation,0,'READY','','operation_planned');
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
  VALUES(i.tenant_id,i.store_id,src.principal_id,'meta.private_reply.planned');
 RETURN p_operation;
END $$;
ALTER FUNCTION integration.claim_reply_plannable(uuid) OWNER TO commerce_integration_writer;
ALTER FUNCTION integration.plan_claim_reply(uuid,uuid,bytea,text,bigint) OWNER TO commerce_integration_writer;

-- §14.1 clause 3: the automatic reply's Check gains human_takeover / takeover_changed (only requests planned by this migration
-- carry origin_kind; older READY operations skip the branch).
CREATE OR REPLACE FUNCTION claims.check_meta_reply(p_operation uuid,p_hash bytea) RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o record; src record; v_deadline timestamptz; v_live boolean; v_ok boolean; w record;
BEGIN
 IF p_operation IS NULL OR p_hash IS NULL OR octet_length(p_hash)<>32
  OR nullif(current_setting('app.tenant_id',true),'') IS NOT NULL OR nullif(current_setting('app.store_id',true),'') IS NOT NULL
  OR nullif(current_setting('app.principal_id',true),'') IS NOT NULL OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL THEN
  RAISE EXCEPTION 'invalid meta reply check' USING ERRCODE='22023';
 END IF;
 SELECT x.tenant_id,x.store_id,x.request INTO o FROM integration.operations x WHERE x.id=p_operation AND x.action='meta.private_reply';
 IF NOT FOUND OR jsonb_typeof(o.request)<>'object' THEN
  RAISE EXCEPTION 'invalid meta reply check' USING ERRCODE='22023';
 END IF;
 v_deadline:=(o.request->>'deadline_at')::timestamptz;
 v_live:=(o.request->>'live_media')::boolean;
 IF clock_timestamp()>=v_deadline THEN RETURN 'deadline'; END IF;
 SELECT s.active,s.private_reply,s.principal_id,s.session_id INTO src FROM live.claim_sources s
  WHERE s.tenant_id=o.tenant_id AND s.store_id=o.store_id AND s.id=(o.request->>'source_id')::uuid;
 IF NOT FOUND OR NOT src.active OR NOT src.private_reply THEN RETURN 'source_off'; END IF;
 IF NOT identity.principal_holds(o.tenant_id,o.store_id,src.principal_id,ARRAY['live:manage','integration:execute']) THEN
  RETURN 'principal_revoked';
 END IF;
 SELECT true INTO v_ok FROM claims.links k WHERE k.tenant_id=o.tenant_id AND k.store_id=o.store_id
  AND k.bundle_id=(o.request->>'bundle_id')::uuid AND k.generation=1 AND k.token_hash=p_hash
  AND k.expires_at>clock_timestamp()+interval '10 minutes';
 IF v_ok IS NOT TRUE THEN RETURN 'link_invalid'; END IF;
 IF v_live AND NOT EXISTS(SELECT 1 FROM live.claim_windows w WHERE w.tenant_id=o.tenant_id AND w.store_id=o.store_id
  AND w.session_id=src.session_id AND w.state='OPEN') THEN
  RETURN 'live_closed';
 END IF;
 -- §3.6: resolve the actor's conversation through the bundle's peers (conversation_known=false) or the frozen generation.
 IF o.request->>'origin_kind'='auto' AND o.request->>'app_id' ~ '^[0-9]{1,40}$' THEN
  SELECT t.mode,t.takeover_generation INTO w FROM inbox.dm_window_for_bundle(o.tenant_id,o.store_id,(o.request->>'bundle_id')::uuid,
   o.request->>'app_id',CASE o.request->>'platform' WHEN 'facebook' THEN 'page' ELSE 'instagram' END,o.request->>'asset_id') t;
  IF FOUND THEN
   IF w.mode='human' THEN RETURN 'human_takeover'; END IF;
   IF (o.request->>'conversation_known')::boolean AND w.takeover_generation<>(o.request->>'takeover_generation')::bigint THEN
    RETURN 'takeover_changed';
   END IF;
  END IF;
 END IF;
 RETURN 'OK';
END $$;
ALTER FUNCTION claims.check_meta_reply(uuid,bytea) OWNER TO commerce_claims_writer;

-- ---------------------------------------------------------------------------------------
-- Read side for A8/A9/A13 (merchant transaction, inbox:read).
-- ---------------------------------------------------------------------------------------
-- P2-1: the newest inbound envelope + AAD columns of up to 50 conversations (display_name is derived in the API).
CREATE FUNCTION social.conversation_heads(p_ids uuid[])
RETURNS TABLE(conversation_id uuid, event_id uuid, key_id text, nonce bytea, ciphertext bytea, app_id text, object text,
 asset_id text, event_key text, payload_hash text, tenant_id uuid, store_id uuid, route_id uuid, route_epoch bigint, server_seq bigint,
 occurred_at timestamptz)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
BEGIN
    IF p_ids IS NULL OR array_ndims(p_ids) <> 1 OR cardinality(p_ids) NOT BETWEEN 1 AND 50 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['inbox:read']::text[]) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    RETURN QUERY
    SELECT DISTINCT ON (m.conversation_id) m.conversation_id, m.event_id, m.key_id, m.nonce, m.ciphertext,
           e.app_id, e.object, e.asset_id, e.event_key, e.payload_hash, m.tenant_id, m.store_id, e.route_id, e.route_epoch,
           m.server_seq, COALESCE(m.occurred_at, m.received_at)
      FROM social.messages m
      JOIN meta_inbox.events e ON e.id = m.event_id AND e.tenant_id = m.tenant_id AND e.store_id = m.store_id
     WHERE m.conversation_id = ANY(p_ids) AND m.tenant_id = v_tenant AND m.store_id = v_store
     ORDER BY m.conversation_id, m.server_seq DESC;
END $$;
ALTER FUNCTION social.conversation_heads(uuid[]) OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION social.conversation_heads(uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION social.conversation_heads(uuid[]) TO commerce_runtime;

-- A9 merge: the merchant's own sent messages of a thread (display copies) with their operation state.
CREATE FUNCTION inbox.read_outbound(p_conversation uuid, p_limit int)
RETURNS TABLE(outbound_id uuid, kind text, principal_id uuid, template_id text, template_version bigint, key_id text, nonce bytea,
 ciphertext bytea, created_at timestamptz, send_state text, result_code text, tenant_id uuid, store_id uuid)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
BEGIN
    v := inbox.lcn_scope();
    IF p_limit IS NULL OR p_limit < 1 OR p_limit > 50 THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422'; END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inbox:read']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF NOT EXISTS (SELECT 1 FROM social.conversations c WHERE c.id = p_conversation AND c.tenant_id = v[1] AND c.store_id = v[2]) THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    RETURN QUERY
    SELECT m.id, m.kind, m.principal_id, m.template_id, m.template_version, m.key_id, m.nonce, m.ciphertext, m.created_at,
           o.state, o.result_code, m.tenant_id, m.store_id
      FROM inbox.outbound_messages m
      LEFT JOIN integration.operations o ON o.tenant_id = m.tenant_id AND o.store_id = m.store_id AND o.id = m.operation_id
     WHERE m.tenant_id = v[1] AND m.store_id = v[2] AND m.conversation_id = p_conversation
     ORDER BY m.created_at DESC, m.id DESC LIMIT p_limit;
END $$;
ALTER FUNCTION inbox.read_outbound(uuid, int) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.read_outbound(uuid, int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.read_outbound(uuid, int) TO commerce_runtime;

-- P2-2: bundles whose claim link could not be sent (A8 bundle-only items), newest first.
GRANT SELECT(platform) ON claims.bundles TO commerce_integration_writer;
CREATE FUNCTION inbox.link_pending_bundles(p_limit int)
RETURNS TABLE(bundle_id uuid, session_id uuid, created_at timestamptz)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
BEGIN
    v := inbox.lcn_scope();
    IF p_limit IS NULL OR p_limit < 1 OR p_limit > 50 THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422'; END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inbox:read']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    RETURN QUERY SELECT b.id, b.session_id, b.created_at FROM claims.bundles b
     WHERE b.tenant_id = v[1] AND b.store_id = v[2] AND b.link_pending_manual ORDER BY b.created_at DESC, b.id DESC LIMIT p_limit;
END $$;
ALTER FUNCTION inbox.link_pending_bundles(int) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.link_pending_bundles(int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.link_pending_bundles(int) TO commerce_runtime;

-- A13 by bundle or by conversation: platform + link_pending_manual (conversation -> its peers' bundles; none -> false).
CREATE FUNCTION inbox.link_pending_for(p_conversation uuid, p_bundle uuid)
RETURNS TABLE(platform text, link_pending_manual boolean)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_obj text;
BEGIN
    v := inbox.lcn_scope();
    IF (p_conversation IS NULL) = (p_bundle IS NULL) THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT400'; END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inbox:read']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF p_bundle IS NOT NULL THEN
        RETURN QUERY SELECT b.platform, b.link_pending_manual FROM claims.bundles b
         WHERE b.tenant_id = v[1] AND b.store_id = v[2] AND b.id = p_bundle;
        IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
        RETURN;
    END IF;
    SELECT c.object INTO v_obj FROM social.conversations c WHERE c.id = p_conversation AND c.tenant_id = v[1] AND c.store_id = v[2];
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    RETURN QUERY SELECT CASE v_obj WHEN 'page' THEN 'messenger' ELSE 'instagram' END,
      EXISTS (SELECT 1 FROM social.conversations c
        JOIN inbox.bundle_peers bp ON bp.tenant_id = c.tenant_id AND bp.store_id = c.store_id AND bp.app_id = c.app_id
         AND bp.object = c.object AND bp.asset_id = c.asset_id AND bp.peer_key = c.peer_key
        JOIN claims.bundles b ON b.tenant_id = bp.tenant_id AND b.store_id = bp.store_id AND b.id = bp.bundle_id
       WHERE c.id = p_conversation AND c.tenant_id = v[1] AND c.store_id = v[2] AND b.link_pending_manual);
END $$;
ALTER FUNCTION inbox.link_pending_for(uuid, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.link_pending_for(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.link_pending_for(uuid, uuid) TO commerce_runtime;

-- The Page-token loader and the Graph-190 reauth mark serve the four send actions as well (same lease fence, same custody;
-- only the action filter widens). Signatures, owner and EXECUTE grants are unchanged.
CREATE OR REPLACE FUNCTION integration.load_meta_page_token(p_operation uuid,p_generation bigint,p_lease_token bytea)
RETURNS TABLE(tenant_id uuid,store_id uuid,binding_id uuid,provider text,asset_id text,version bigint,key_id text,
 nonce bytea,ciphertext bytea,scopes_attested text[])
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_binding uuid; b record; o record; v_head bigint;
BEGIN
 IF p_operation IS NULL OR p_generation IS NULL OR p_generation<1 OR p_lease_token IS NULL OR octet_length(p_lease_token)<>32 THEN
  RAISE EXCEPTION 'invalid Meta credential load' USING ERRCODE='22023';
 END IF;
 SELECT x.binding_id INTO v_binding FROM integration.operations x
  WHERE x.id=p_operation AND x.action IN ('meta.private_reply','meta.dm_send','meta.public_reply','meta.offer_recommend') AND x.actor_kind='MERCHANT';
 IF NOT FOUND THEN RAISE EXCEPTION 'Meta credential unavailable' USING ERRCODE='P0002'; END IF;
 -- Binding before operation (dispatcher lock order).
 SELECT z.id,z.provider,z.external_asset_id INTO b FROM integration.bindings z WHERE z.id=v_binding FOR SHARE;
 SELECT x.tenant_id,x.store_id,x.binding_id,x.provider,x.external_asset_id,x.state,x.lease_mode,x.generation,x.lease_until,
  x.lease_token_hash INTO o FROM integration.operations x WHERE x.id=p_operation FOR SHARE;
 IF NOT FOUND OR o.state<>'DISPATCHING' OR o.lease_mode<>'dispatch' OR o.generation<>p_generation OR o.lease_until IS NULL
  OR o.lease_until<=clock_timestamp() OR o.lease_token_hash IS DISTINCT FROM sha256(p_lease_token) THEN
  RAISE EXCEPTION 'Meta credential lease conflict' USING ERRCODE='40001';
 END IF;
 IF b.id IS NULL OR b.id<>o.binding_id OR b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id THEN
  RAISE EXCEPTION 'Meta credential binding mismatch' USING ERRCODE='PT409';
 END IF;
 SELECT h.current_version INTO v_head FROM integration.meta_page_heads h
  WHERE h.tenant_id=o.tenant_id AND h.store_id=o.store_id AND h.binding_id=o.binding_id;
 IF NOT FOUND THEN RETURN; END IF;   -- no credential: zero rows -> BLOCKED_POLICY at the dispatcher
 -- Recheck after waits so an expired claim cannot release secret material.
 IF o.lease_until<=clock_timestamp() THEN RAISE EXCEPTION 'Meta credential lease conflict' USING ERRCODE='40001'; END IF;
 RETURN QUERY SELECT c.tenant_id,c.store_id,c.binding_id,c.provider,c.asset_id,c.version,c.key_id,c.nonce,c.ciphertext,c.scopes_attested
  FROM integration.meta_page_credentials c WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id
   AND c.binding_id=o.binding_id AND c.version=v_head;
END $$;

CREATE OR REPLACE FUNCTION integration.meta_connect_mark_reauth(p_operation uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o record;
BEGIN
 IF p_operation IS NULL THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='MC422'; END IF;
 SELECT x.tenant_id,x.store_id,x.binding_id INTO o FROM integration.operations x WHERE x.id=p_operation
  AND x.action IN ('meta.private_reply','meta.dm_send','meta.public_reply','meta.offer_recommend');
 IF NOT FOUND THEN RETURN; END IF;
 UPDATE integration.meta_connections c SET status='reauth_required',updated_at=clock_timestamp()
  WHERE c.tenant_id=o.tenant_id AND c.store_id=o.store_id AND c.status='active' AND o.binding_id IN (c.fb_binding,c.ig_binding);
END $$;
