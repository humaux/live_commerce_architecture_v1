-- 0131_checkout_reminders.sql — W3-03B checkout reminders (live-console-v1 Amendment W3-03B; brief docs/delivery/units/w3-03b-checkout-reminder.md).
-- Purpose: after a live session (or when the merchant clicks 「提醒未付款」) remind buyers who claimed but did not order, or ordered but did not pay,
--   by ONE Messenger/IG DM per buyer per session, ONLY inside the 24 h window (messaging_type RESPONSE; no message tag, no UPDATE, no utility
--   template, ever). Buyers outside the window / under human takeover / without a known thread land in a follow-up list instead. Adds the fixed
--   template checkout-reminder/v1, the reminder ledger table, the per-store reminder settings and five definers.
-- Depends on: 0121 (msgtemplates.fixed_templates), 0128 (inbox.lcn_scope/lcn_emit, inbox.bundle_peers, inbox.check_send, outbound_messages,
--   send_secrets, the meta.dm_send worker route), 0119 (inbox.conversation_state), 0113 (claims.order_origins), 0013 (checkout.orders).
-- Used by: internal/inbox/reminders.go, internal/httpapi/reminders.go (POST/GET .../live-sessions/{sid}/reminders, GET/PUT .../live-settings/reminder).
-- Invariants: LCN06 (no send outside the window; re-checked at Check by inbox.check_send), I06/I07 (the send is a meta.dm_send operation of the
--   existing ledger: no second send path), one reminder per buyer per session (semantic key 'crm:' + sha256(session|owner-or-bundle)), the
--   dispatch copy and the PSID never reach SQL in plaintext (sealed in Go, plan_dm pattern).
-- Deviations from the brief (recorded in output/w3-03b-checkout-reminder/DELIVERY.md): (1) planning is two statements in ONE merchant
--   transaction (candidates, then one plan per sendable buyer) because the dispatch copy is sealed in Go with the payload keyring + the PSID
--   (API-only custody); (2) origin=auto DMs never take a conversation over (plan_dm is the human variant), so inbox.plan_checkout_reminder is
--   the auto variant of plan_dm and shares inbox.lcn_emit / inbox.check_send / the dm_send route; (3) no table is readable by commerce_runtime
--   (0128 convention: definers only); (4) the reminder link is the store's non-bearer checkout URL: claims.links / order links keep only hashes,
--   so an existing bearer link can never be re-derived and the brief's "copy the existing link" is impossible without issuing a new one.
-- Template-id CHECK note: W3-04B (sold-out reply, 0132) also extends msgtemplates.fixed_templates' template_id CHECK. Both units rewrite the
--   same constraint, so the integrator serialises them: the final list is the union of every fixed template id.
-- Status: MOCK (REAL_PG + fake Graph); Meta LIVE = NOT_RUN.

-- ---------------------------------------------------------------------------------------
-- Fixed template checkout-reminder/v1 (dm only, not public-safe: it carries the {{連結}} placeholder the Go planner fills).
-- ---------------------------------------------------------------------------------------
ALTER TABLE msgtemplates.fixed_templates DROP CONSTRAINT fixed_templates_template_id_check;
ALTER TABLE msgtemplates.fixed_templates ADD CONSTRAINT fixed_templates_template_id_check
    CHECK (template_id IN ('order-pay-link/v1', 'offer-recommend/v1', 'checkout-reminder/v1'));
INSERT INTO msgtemplates.fixed_templates(template_id, version, name, body, kinds, public_safe) VALUES
 ('checkout-reminder/v1', 1, '結帳提醒', '您在直播中的訂單尚未完成結帳，請點此連結繼續：{{連結}}', ARRAY['dm'], false)
ON CONFLICT (template_id, version) DO NOTHING;

-- ---------------------------------------------------------------------------------------
-- Tables (FORCE RLS; the definers of commerce_integration_writer are the only readers/writers, like 0128's tables).
-- ---------------------------------------------------------------------------------------
-- One row per (session, bundle): 'followup' (needs the merchant: reason) or 'queued' (a meta.dm_send operation exists). Soft references (no
-- FK): claims retention deletes bundles/operations independently. A later trigger may turn a followup row into a queued one (the buyer wrote
-- again); a queued row never changes.
CREATE TABLE inbox.checkout_reminders (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL,
    bundle_id uuid NOT NULL,
    owner_id uuid,
    peer_key text CHECK (peer_key ~ '^[0-9a-f]{64}$'),
    semantic_key text NOT NULL CHECK (semantic_key ~ '^crm:[0-9a-f]{48}$'),
    outcome text NOT NULL CHECK (outcome IN ('queued', 'followup')),
    reason text CHECK (reason IN ('window_closed', 'human_takeover', 'no_peer', 'capability')),
    reminder_state text NOT NULL CHECK (reminder_state IN ('claimed', 'awaiting_payment')),
    trigger text NOT NULL CHECK (trigger IN ('manual', 'auto')),
    operation_id uuid,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, id),
    CONSTRAINT checkout_reminders_bundle_once UNIQUE (tenant_id, store_id, session_id, bundle_id),
    CHECK ((outcome = 'queued') = (operation_id IS NOT NULL)),
    CHECK ((outcome = 'followup') = (reason IS NOT NULL))
);
-- At most one reminder per buyer per session even when one owner holds several bundles (FB + IG).
CREATE UNIQUE INDEX checkout_reminders_once ON inbox.checkout_reminders(tenant_id, store_id, semantic_key) WHERE outcome = 'queued';

CREATE TABLE live.reminder_settings (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    delay_minutes integer NOT NULL CHECK (delay_minutes BETWEEN 10 AND 1440),
    version bigint NOT NULL CHECK (version > 0),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id)
);

ALTER TABLE inbox.checkout_reminders ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.checkout_reminders FORCE ROW LEVEL SECURITY;
ALTER TABLE live.reminder_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE live.reminder_settings FORCE ROW LEVEL SECURITY;
REVOKE ALL ON inbox.checkout_reminders, live.reminder_settings FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON inbox.checkout_reminders, live.reminder_settings TO commerce_integration_writer;
CREATE POLICY reminders_rw ON inbox.checkout_reminders FOR ALL TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id)) WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id));
CREATE POLICY reminder_settings_rw ON live.reminder_settings FOR ALL TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id)) WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id));

-- ---------------------------------------------------------------------------------------
-- Scoped reads the candidate scan needs (columns only; every policy is the transaction's tenant/store GUC scope, as in 0128).
-- ---------------------------------------------------------------------------------------
GRANT USAGE ON SCHEMA checkout TO commerce_integration_writer;
GRANT SELECT(owner_id, label, line_count, purged_at) ON claims.bundles TO commerce_integration_writer;
GRANT SELECT(tenant_id, store_id, session_id, active, reply_locale) ON live.claim_sources TO commerce_integration_writer;
GRANT SELECT(tenant_id, store_id, order_id, bundle_id) ON claims.order_origins TO commerce_integration_writer;
CREATE POLICY order_origin_reminder_read ON claims.order_origins FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));
GRANT SELECT(tenant_id, store_id, id, commercial_state, expires_at) ON checkout.orders TO commerce_integration_writer;
CREATE POLICY order_reminder_read ON checkout.orders FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));
-- The two audit actions of this unit (lcn_emit inserts the planned one, the settings definer the other).
CREATE POLICY reminder_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id)
        AND action IN ('inbox.checkout_reminder.planned', 'live.reminder_settings.updated'));

-- ---------------------------------------------------------------------------------------
-- Private helper: which reminder state does this bundle have? NULL = not a candidate (ordered and paid/cancelled/expired, no lines, purged).
-- 'awaiting_payment' = an AWAITING_PAYMENT, unexpired order of the bundle; 'claimed' = claim lines and no order at all (a cancelled order
-- is an owner decision, never reminded).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.crm_bundle_state(p_tenant uuid, p_store uuid, p_bundle uuid) RETURNS text
LANGUAGE plpgsql STABLE SET search_path = pg_catalog AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM claims.order_origins o
                 JOIN checkout.orders k ON k.tenant_id = o.tenant_id AND k.store_id = o.store_id AND k.id = o.order_id
                WHERE o.tenant_id = p_tenant AND o.store_id = p_store AND o.bundle_id = p_bundle
                  AND k.commercial_state = 'AWAITING_PAYMENT' AND k.expires_at > clock_timestamp()) THEN
        RETURN 'awaiting_payment';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM claims.order_origins o WHERE o.tenant_id = p_tenant AND o.store_id = p_store AND o.bundle_id = p_bundle) THEN
        RETURN 'claimed';
    END IF;
    RETURN NULL;
END $$;
ALTER FUNCTION inbox.crm_bundle_state(uuid, uuid, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.crm_bundle_state(uuid, uuid, uuid) FROM PUBLIC;

-- ---------------------------------------------------------------------------------------
-- inbox.checkout_reminder_candidates (merchant transaction, inbox:reply): enumerate the session's candidate buyers and classify each.
-- verdict: 'send' (window open, auto mode, capability ok: the caller plans one DM), 'already_reminded' (a queued reminder exists), or a
-- followup reason (window_closed | human_takeover | no_peer | capability) which this call records in inbox.checkout_reminders.
-- Serialised per session by an advisory lock held to the end of the transaction (the plan below runs in the same transaction).
-- p_bundle NULL = the batch over every candidate; a bundle id = that one buyer only (409 not_remindable when it is no candidate).
-- At most p_limit (1..100) buyers per call; truncated tells the caller to trigger again (already-reminded buyers are skipped cheaply).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.checkout_reminder_candidates(p_session uuid, p_trigger text, p_limit int, p_bundle uuid)
RETURNS TABLE(bundle_id uuid, verdict text, reminder_state text, conversation_id uuid, takeover_generation bigint, platform text,
 locale text, truncated boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; v_now timestamptz := clock_timestamp(); v_locale text; b record; c record; v_key text;
 v_state text; v_provider text; v_bind uuid; v_n int := 0; v_emitted int := 0; v_trunc boolean := false; v_mode text; v_gen bigint; v_verdict text;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_session IS NULL OR p_trigger IS NULL OR p_trigger NOT IN ('manual', 'auto') OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 100 THEN
        RAISE EXCEPTION 'invalid reminder scan' USING ERRCODE = '22023';
    END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF NOT EXISTS (SELECT 1 FROM live.claim_windows w WHERE w.tenant_id = v_t AND w.store_id = v_s AND w.session_id = p_session) THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    IF p_bundle IS NOT NULL AND NOT EXISTS (SELECT 1 FROM claims.bundles x WHERE x.tenant_id = v_t AND x.store_id = v_s AND x.session_id = p_session AND x.id = p_bundle) THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('crm|' || v_s::text || '|' || p_session::text, 0));
    SELECT y.reply_locale INTO v_locale FROM live.claim_sources y
     WHERE y.tenant_id = v_t AND y.store_id = v_s AND y.session_id = p_session ORDER BY y.active DESC, y.updated_at DESC LIMIT 1;
    v_locale := coalesce(v_locale, 'zh-TW');
    FOR b IN SELECT x.id, x.owner_id FROM claims.bundles x
              WHERE x.tenant_id = v_t AND x.store_id = v_s AND x.session_id = p_session AND x.platform IN ('facebook', 'instagram')
                AND x.purged_at IS NULL AND x.line_count > 0 AND (p_bundle IS NULL OR x.id = p_bundle)
              ORDER BY x.created_at, x.id LOOP
        v_state := inbox.crm_bundle_state(v_t, v_s, b.id);
        IF v_state IS NULL THEN CONTINUE; END IF;
        v_n := v_n + 1;
        IF v_n > p_limit THEN v_trunc := true; EXIT; END IF;
        v_key := 'crm:' || substr(encode(sha256(convert_to(p_session::text || '|' || coalesce(b.owner_id, b.id)::text, 'UTF8')), 'hex'), 1, 48);
        IF EXISTS (SELECT 1 FROM inbox.checkout_reminders r WHERE r.tenant_id = v_t AND r.store_id = v_s AND r.semantic_key = v_key AND r.outcome = 'queued') THEN
            v_emitted := v_emitted + 1;
            RETURN QUERY SELECT b.id, 'already_reminded'::text, v_state, NULL::uuid, NULL::bigint, NULL::text, v_locale, false;
            CONTINUE;
        END IF;
        -- The buyer's thread = the newest conversation among the bundle's peers (peers exist only after a private reply reached the buyer).
        SELECT cv.id AS conv, cv.object, cv.asset_id, cv.peer_key, s.mode, s.takeover_generation, s.human_until, s.last_inbound_at INTO c
          FROM inbox.bundle_peers bp
          JOIN social.conversations cv ON cv.tenant_id = bp.tenant_id AND cv.store_id = bp.store_id AND cv.app_id = bp.app_id
           AND cv.object = bp.object AND cv.asset_id = bp.asset_id AND cv.peer_key = bp.peer_key
          JOIN inbox.conversation_state s ON s.tenant_id = cv.tenant_id AND s.store_id = cv.store_id AND s.conversation_id = cv.id
         WHERE bp.tenant_id = v_t AND bp.store_id = v_s AND bp.bundle_id = b.id
         ORDER BY s.last_inbound_at DESC NULLS LAST, bp.created_at DESC LIMIT 1
           FOR SHARE OF s;  -- held to the end of the transaction: a takeover / inbound update waits, so the plan below sees what was classified
        v_verdict := NULL;
        IF NOT FOUND THEN
            v_verdict := 'no_peer';
        ELSE
            -- §3.6 lazy takeover expiry, as in inbox.check_send / dm_window_for_bundle.
            v_mode := c.mode; v_gen := c.takeover_generation;
            IF c.mode = 'human' AND c.human_until IS NOT NULL AND v_now >= c.human_until THEN v_mode := 'auto'; v_gen := v_gen + 1; END IF;
            v_provider := CASE c.object WHEN 'page' THEN 'facebook' ELSE 'instagram' END;
            IF c.last_inbound_at IS NULL OR c.last_inbound_at + interval '24 hours' - interval '5 minutes' <= v_now THEN
                v_verdict := 'window_closed';
            ELSIF v_mode = 'human' THEN
                v_verdict := 'human_takeover';
            ELSE
                SELECT z.id INTO v_bind FROM integration.bindings z
                 WHERE z.tenant_id = v_t AND z.store_id = v_s AND z.provider = v_provider AND z.external_asset_id = c.asset_id AND z.enabled
                 ORDER BY z.id LIMIT 1;
                IF NOT FOUND OR coalesce(integration.binding_capability_state(v_t, v_s, v_bind, 'dm_session', ARRAY[]::text[]), 'unknown')
                                NOT IN ('ok', 'review_required') THEN
                    v_verdict := 'capability';
                END IF;
            END IF;
        END IF;
        v_emitted := v_emitted + 1;
        IF v_verdict IS NULL THEN
            RETURN QUERY SELECT b.id, 'send'::text, v_state, c.conv, v_gen, v_provider, v_locale, false;
        ELSE
            INSERT INTO inbox.checkout_reminders(tenant_id, store_id, session_id, bundle_id, owner_id, peer_key, semantic_key, outcome, reason,
                reminder_state, trigger, created_at, updated_at)
            VALUES (v_t, v_s, p_session, b.id, b.owner_id, c.peer_key, v_key, 'followup', v_verdict, v_state, p_trigger, v_now, v_now)
            ON CONFLICT ON CONSTRAINT checkout_reminders_bundle_once DO UPDATE
               SET reason = EXCLUDED.reason, reminder_state = EXCLUDED.reminder_state, trigger = EXCLUDED.trigger,
                   peer_key = EXCLUDED.peer_key, updated_at = EXCLUDED.updated_at
             WHERE inbox.checkout_reminders.outcome = 'followup';
            RETURN QUERY SELECT b.id, v_verdict, v_state, NULL::uuid, NULL::bigint, NULL::text, v_locale, false;
        END IF;
    END LOOP;
    -- A single-buyer request for a bundle that is not (or no longer) a candidate (paid, cancelled, no lines) is refused, not silently empty.
    IF p_bundle IS NOT NULL AND v_emitted = 0 THEN RAISE EXCEPTION 'not_remindable' USING ERRCODE = 'PT409'; END IF;
    IF v_trunc THEN
        RETURN QUERY SELECT NULL::uuid, 'truncated'::text, NULL::text, NULL::uuid, NULL::bigint, NULL::text, v_locale, true;
    END IF;
END $$;
ALTER FUNCTION inbox.checkout_reminder_candidates(uuid, text, int, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.checkout_reminder_candidates(uuid, text, int, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.checkout_reminder_candidates(uuid, text, int, uuid) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- inbox.plan_checkout_reminder (merchant transaction, inbox:reply): the origin=auto variant of inbox.plan_dm for ONE candidate. It re-validates
-- the window, the takeover state and the capability under row locks, records the queued reminder (the once-per-buyer guard), and plans the
-- DM through inbox.lcn_emit: the SAME operation / outbound row / sealed dispatch copy / River job path as every other meta.dm_send, so
-- inbox.check_send re-checks window + takeover at dispatch (BLOCKED_POLICY window_closed / human_takeover, never retried) and an UNKNOWN
-- send is never repeated. Unlike plan_dm it never takes the conversation over (origin=auto, §3.6) and has no rate/duplicate lock of its own
-- (the per-session advisory lock + the unique crm: key are the serialisers). Refusals are PT409 with the deny code as message.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.plan_checkout_reminder(p_session uuid, p_bundle uuid, p_trigger text, p_expected_generation bigint, p_operation uuid,
 p_job bigint, p_outbound uuid, p_body_hmac bytea, p_display_key_id text, p_display_nonce bytea, p_display_ciphertext bytea,
 p_secret_enc bytea, p_secret_sealed bytea, p_template_id text, p_template_version bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; v_now timestamptz := clock_timestamp(); b record; c record; st record; z record; v_state text;
 v_key text; v_provider text; v_req jsonb; v_mode text; v_gen bigint;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_session IS NULL OR p_bundle IS NULL OR p_trigger IS NULL OR p_trigger NOT IN ('manual', 'auto') OR p_expected_generation IS NULL
       OR p_expected_generation < 0 OR p_body_hmac IS NULL OR octet_length(p_body_hmac) <> 32
       OR p_template_id IS DISTINCT FROM 'checkout-reminder/v1' OR p_template_version IS DISTINCT FROM 1 THEN
        RAISE EXCEPTION 'invalid reminder plan' USING ERRCODE = '22023';
    END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('crm|' || v_s::text || '|' || p_session::text, 0));
    SELECT x.id, x.owner_id INTO b FROM claims.bundles x
     WHERE x.tenant_id = v_t AND x.store_id = v_s AND x.id = p_bundle AND x.session_id = p_session AND x.platform IN ('facebook', 'instagram')
       AND x.purged_at IS NULL;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    v_state := inbox.crm_bundle_state(v_t, v_s, p_bundle);
    IF v_state IS NULL THEN RAISE EXCEPTION 'not_remindable' USING ERRCODE = 'PT409'; END IF;
    v_key := 'crm:' || substr(encode(sha256(convert_to(p_session::text || '|' || coalesce(b.owner_id, b.id)::text, 'UTF8')), 'hex'), 1, 48);
    IF EXISTS (SELECT 1 FROM inbox.checkout_reminders r WHERE r.tenant_id = v_t AND r.store_id = v_s AND r.semantic_key = v_key AND r.outcome = 'queued') THEN
        RAISE EXCEPTION 'already_reminded' USING ERRCODE = 'PT409';
    END IF;
    SELECT cv.id AS conv, cv.object, cv.asset_id, cv.app_id, cv.peer_key INTO c
      FROM inbox.bundle_peers bp
      JOIN social.conversations cv ON cv.tenant_id = bp.tenant_id AND cv.store_id = bp.store_id AND cv.app_id = bp.app_id
       AND cv.object = bp.object AND cv.asset_id = bp.asset_id AND cv.peer_key = bp.peer_key
      JOIN inbox.conversation_state s2 ON s2.tenant_id = cv.tenant_id AND s2.store_id = cv.store_id AND s2.conversation_id = cv.id
     WHERE bp.tenant_id = v_t AND bp.store_id = v_s AND bp.bundle_id = p_bundle
     ORDER BY s2.last_inbound_at DESC NULLS LAST, bp.created_at DESC LIMIT 1;
    IF NOT FOUND THEN RAISE EXCEPTION 'conversation_gone' USING ERRCODE = 'PT409'; END IF;
    -- Share lock only: an auto send never changes the conversation state (no takeover); it must not overtake a concurrent human takeover.
    SELECT s.mode, s.takeover_generation, s.human_until, s.last_inbound_at INTO st FROM inbox.conversation_state s
     WHERE s.tenant_id = v_t AND s.store_id = v_s AND s.conversation_id = c.conv FOR SHARE;
    IF NOT FOUND THEN RAISE EXCEPTION 'conversation_gone' USING ERRCODE = 'PT409'; END IF;
    v_mode := st.mode; v_gen := st.takeover_generation;
    IF st.mode = 'human' AND st.human_until IS NOT NULL AND v_now >= st.human_until THEN v_mode := 'auto'; v_gen := v_gen + 1; END IF;
    IF st.last_inbound_at IS NULL OR st.last_inbound_at + interval '24 hours' - interval '5 minutes' <= v_now THEN
        RAISE EXCEPTION 'window_closed' USING ERRCODE = 'PT409';
    END IF;
    IF v_mode = 'human' THEN RAISE EXCEPTION 'human_takeover' USING ERRCODE = 'PT409'; END IF;
    IF v_gen <> p_expected_generation THEN RAISE EXCEPTION 'takeover_changed' USING ERRCODE = 'PT409'; END IF;
    v_provider := CASE c.object WHEN 'page' THEN 'facebook' ELSE 'instagram' END;
    SELECT y.id, y.semantic_version INTO z FROM integration.bindings y
     WHERE y.tenant_id = v_t AND y.store_id = v_s AND y.provider = v_provider AND y.external_asset_id = c.asset_id AND y.enabled
     ORDER BY y.id LIMIT 1 FOR SHARE;
    IF NOT FOUND OR coalesce(integration.binding_capability_state(v_t, v_s, z.id, 'dm_session', ARRAY[]::text[]), 'unknown')
                    NOT IN ('ok', 'review_required') THEN
        RAISE EXCEPTION 'capability' USING ERRCODE = 'PT409';
    END IF;
    -- The once-per-buyer guard: turns this bundle's followup row (if any) into the queued one; a queued row of this bundle already refused above.
    INSERT INTO inbox.checkout_reminders(tenant_id, store_id, session_id, bundle_id, owner_id, peer_key, semantic_key, outcome, reason,
        reminder_state, trigger, operation_id, created_at, updated_at)
    VALUES (v_t, v_s, p_session, p_bundle, b.owner_id, c.peer_key, v_key, 'queued', NULL, v_state, p_trigger, p_operation, v_now, v_now)
    ON CONFLICT ON CONSTRAINT checkout_reminders_bundle_once DO UPDATE
       SET outcome = 'queued', reason = NULL, reminder_state = EXCLUDED.reminder_state, trigger = EXCLUDED.trigger,
           semantic_key = EXCLUDED.semantic_key, operation_id = EXCLUDED.operation_id, updated_at = EXCLUDED.updated_at
     WHERE inbox.checkout_reminders.outcome = 'followup';
    IF NOT FOUND THEN RAISE EXCEPTION 'already_reminded' USING ERRCODE = 'PT409'; END IF;
    v_req := jsonb_build_object('v', 1, 'kind', 'dm', 'message_type', 'checkout_reminder', 'origin', 'auto', 'platform', v_provider,
        'asset_id', c.asset_id, 'app_id', c.app_id, 'conversation_id', c.conv, 'conversation_known', true,
        'peer_key', c.peer_key, 'outbound_id', p_outbound, 'body_hmac', encode(p_body_hmac, 'hex'), 'policy', 'lcn-policy/v1',
        'takeover_generation', v_gen, 'principal_id', v_p, 'bundle_id', p_bundle, 'session_id', p_session,
        'template_id', p_template_id, 'template_version', p_template_version,
        'deadline_at', to_char((st.last_inbound_at + interval '24 hours' - interval '5 minutes') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
    -- Calls inbox.lcn_emit (0128): the shared tail of every meta.dm_send (operation + READY event + outbound display row + sealed secret + audit).
    PERFORM inbox.lcn_emit(v_t, v_s, v_p, 'dm', 'meta.dm_send', z.id, z.semantic_version, v_provider, c.asset_id, v_key,
        v_req, p_operation, p_job, p_outbound, c.conv, NULL, p_body_hmac, p_display_key_id, p_display_nonce,
        p_display_ciphertext, p_secret_enc, p_secret_sealed, p_template_id, p_template_version, 'inbox.checkout_reminder.planned');
    RETURN p_operation;
END $$;
ALTER FUNCTION inbox.plan_checkout_reminder(uuid, uuid, text, bigint, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint)
    OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.plan_checkout_reminder(uuid, uuid, text, bigint, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.plan_checkout_reminder(uuid, uuid, text, bigint, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint)
    TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- inbox.reminder_report (inbox:read): the session's reminder rows with the visible send state of each queued DM (§4.4 mapping is in Go).
-- No PSID, no text, no link: bundle id, label (manual bundles only), outcome/reason and the operation's state/result code.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.reminder_report(p_session uuid)
RETURNS TABLE(bundle_id uuid, label text, outcome text, reason text, reminder_state text, operation_id uuid, op_state text, result_code text,
 updated_at timestamptz, locale text)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_locale text;
BEGIN
    v := inbox.lcn_scope();
    IF p_session IS NULL THEN RAISE EXCEPTION 'invalid reminder report' USING ERRCODE = '22023'; END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inbox:read']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF NOT EXISTS (SELECT 1 FROM live.claim_windows w WHERE w.tenant_id = v[1] AND w.store_id = v[2] AND w.session_id = p_session) THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    SELECT y.reply_locale INTO v_locale FROM live.claim_sources y
     WHERE y.tenant_id = v[1] AND y.store_id = v[2] AND y.session_id = p_session ORDER BY y.active DESC, y.updated_at DESC LIMIT 1;
    RETURN QUERY
    SELECT r.bundle_id, bu.label, r.outcome, r.reason, r.reminder_state, r.operation_id, o.state, o.result_code, r.updated_at,
           coalesce(v_locale, 'zh-TW')
      FROM inbox.checkout_reminders r
      LEFT JOIN claims.bundles bu ON bu.tenant_id = r.tenant_id AND bu.store_id = r.store_id AND bu.id = r.bundle_id
      LEFT JOIN integration.operations o ON o.tenant_id = r.tenant_id AND o.store_id = r.store_id AND o.id = r.operation_id
     WHERE r.tenant_id = v[1] AND r.store_id = v[2] AND r.session_id = p_session
     ORDER BY r.updated_at DESC, r.bundle_id LIMIT 500;
END $$;
ALTER FUNCTION inbox.reminder_report(uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.reminder_report(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.reminder_report(uuid) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- Store settings: 「場次結束後 N 分鐘自動提醒」 (default off). get = live:read; put = live:manage with an expected_version CAS
-- (0 = no row yet). The values are only stored here: the automatic send is DEFERRED (payload keyring stays API-only; see the contract amendment).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION live.get_reminder_settings()
RETURNS TABLE(enabled boolean, delay_minutes integer, version bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
BEGIN
    v := inbox.lcn_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:read']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    RETURN QUERY SELECT x.enabled, x.delay_minutes, x.version FROM live.reminder_settings x WHERE x.tenant_id = v[1] AND x.store_id = v[2];
    IF NOT FOUND THEN RETURN QUERY SELECT false, 30, 0::bigint; END IF;
END $$;
ALTER FUNCTION live.get_reminder_settings() OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION live.get_reminder_settings() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.get_reminder_settings() TO commerce_runtime;

CREATE FUNCTION live.put_reminder_settings(p_enabled boolean, p_delay_minutes integer, p_expected_version bigint) RETURNS bigint
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_version bigint;
BEGIN
    v := inbox.lcn_scope();
    IF p_enabled IS NULL OR p_delay_minutes IS NULL OR p_delay_minutes NOT BETWEEN 10 AND 1440 OR p_expected_version IS NULL OR p_expected_version < 0 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:manage']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF p_expected_version = 0 THEN
        INSERT INTO live.reminder_settings(tenant_id, store_id, enabled, delay_minutes, version) VALUES (v[1], v[2], p_enabled, p_delay_minutes, 1)
        ON CONFLICT (tenant_id, store_id) DO NOTHING RETURNING version INTO v_version;
    ELSE
        UPDATE live.reminder_settings x SET enabled = p_enabled, delay_minutes = p_delay_minutes, version = x.version + 1, updated_at = clock_timestamp()
         WHERE x.tenant_id = v[1] AND x.store_id = v[2] AND x.version = p_expected_version RETURNING x.version INTO v_version;
    END IF;
    IF v_version IS NULL THEN RAISE EXCEPTION 'version_conflict' USING ERRCODE = 'PT409'; END IF;
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (v[1], v[2], v[3], 'live.reminder_settings.updated');
    RETURN v_version;
END $$;
ALTER FUNCTION live.put_reminder_settings(boolean, integer, bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION live.put_reminder_settings(boolean, integer, bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.put_reminder_settings(boolean, integer, bigint) TO commerce_runtime;
