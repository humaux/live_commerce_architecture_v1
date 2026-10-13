-- Purpose: bind new payment reminders to their authoritative order and scope legacy link guards to immutable bundle/order origins.
-- Depends on: 0144 reminder planner/journal/crm_bundle_facts, integration.operations, claims.order_origins and authenticated lcn_scope.
-- Used by: inbox.PlanCheckoutReminder and merchanttools.guardOrderPayLink; no dispatcher, sealed secret or existing ledger rewrites.
-- Invariants: I01/I02/I06/I11; same-order link protection, no cross-buyer batch refusal, exact definer ACL, no new table grants.

CREATE OR REPLACE FUNCTION inbox.plan_checkout_reminder(p_session uuid, p_bundle uuid, p_conversation uuid, p_trigger text, p_expected_generation bigint,
 p_reminder_state text, p_operation uuid, p_job bigint, p_outbound uuid, p_body_hmac bytea, p_display_key_id text, p_display_nonce bytea,
 p_display_ciphertext bytea, p_secret_enc bytea, p_secret_sealed bytea, p_template_id text, p_template_version bigint)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; v_now timestamptz := clock_timestamp(); b record; c record; st record; z record; f record;
 v_key text; v_provider text; v_req jsonb; v_mode text; v_gen bigint; v_tpl text;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_session IS NULL OR p_bundle IS NULL OR p_conversation IS NULL OR p_trigger IS NULL OR p_trigger NOT IN ('manual', 'auto')
       OR p_expected_generation IS NULL OR p_expected_generation < 0 OR p_body_hmac IS NULL OR octet_length(p_body_hmac) <> 32
       OR p_reminder_state IS NULL OR p_reminder_state NOT IN ('claimed', 'awaiting_payment')
       OR p_template_version IS DISTINCT FROM 1 THEN
        RAISE EXCEPTION 'invalid reminder plan' USING ERRCODE = '22023';
    END IF;
    v_tpl := CASE p_reminder_state WHEN 'claimed' THEN 'checkout-reminder/v1' ELSE 'order-pay-link/v1' END;
    IF p_template_id IS DISTINCT FROM v_tpl THEN RAISE EXCEPTION 'invalid reminder plan' USING ERRCODE = '22023'; END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('lcn-dup|' || v_s::text || '|' || p_conversation::text || '|' || encode(p_body_hmac, 'hex'), 0));
    PERFORM inbox.lcn_rate_check(v_t, v_s);
    PERFORM pg_advisory_xact_lock(hashtextextended('crm|' || v_s::text || '|' || p_session::text, 0));
    SELECT x.id, x.owner_id INTO b FROM claims.bundles x
     WHERE x.tenant_id = v_t AND x.store_id = v_s AND x.id = p_bundle AND x.session_id = p_session AND x.platform IN ('facebook', 'instagram');
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    SELECT * INTO f FROM inbox.crm_bundle_facts(v_t, v_s, p_bundle);
    IF f.state IS DISTINCT FROM p_reminder_state THEN RAISE EXCEPTION 'not_remindable' USING ERRCODE = 'PT409'; END IF;
    v_key := 'crm:' || substr(encode(sha256(convert_to(p_session::text || '|' || coalesce(b.owner_id, b.id)::text, 'UTF8')), 'hex'), 1, 48);
    IF EXISTS (SELECT 1 FROM inbox.checkout_reminders r WHERE r.tenant_id = v_t AND r.store_id = v_s AND r.semantic_key = v_key AND r.outcome = 'queued') THEN
        RAISE EXCEPTION 'already_reminded' USING ERRCODE = 'PT409';
    END IF;
    SELECT cv.id AS conv, cv.object, cv.asset_id, cv.app_id, cv.peer_key INTO c
      FROM inbox.bundle_peers bp
      JOIN social.conversations cv ON cv.tenant_id = bp.tenant_id AND cv.store_id = bp.store_id AND cv.app_id = bp.app_id
       AND cv.object = bp.object AND cv.asset_id = bp.asset_id AND cv.peer_key = bp.peer_key
     WHERE bp.tenant_id = v_t AND bp.store_id = v_s AND bp.bundle_id = p_bundle AND cv.id = p_conversation;
    IF NOT FOUND THEN RAISE EXCEPTION 'conversation_gone' USING ERRCODE = 'PT409'; END IF;
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
    INSERT INTO inbox.checkout_reminders(tenant_id, store_id, session_id, bundle_id, owner_id, peer_key, semantic_key, outcome, reason,
        reminder_state, trigger, operation_id, created_at, updated_at)
    VALUES (v_t, v_s, p_session, p_bundle, b.owner_id, c.peer_key, v_key, 'queued', NULL, p_reminder_state, p_trigger, p_operation, v_now, v_now)
    ON CONFLICT ON CONSTRAINT checkout_reminders_bundle_once DO UPDATE
       SET outcome = 'queued', reason = NULL, reminder_state = EXCLUDED.reminder_state, trigger = EXCLUDED.trigger,
           semantic_key = EXCLUDED.semantic_key, operation_id = EXCLUDED.operation_id, updated_at = EXCLUDED.updated_at
     WHERE inbox.checkout_reminders.outcome = 'followup';
    IF NOT FOUND THEN RAISE EXCEPTION 'already_reminded' USING ERRCODE = 'PT409'; END IF;
    v_req := jsonb_build_object('v', 1, 'kind', 'dm', 'message_type', 'checkout_reminder', 'origin', 'auto', 'platform', v_provider,
        'asset_id', c.asset_id, 'app_id', c.app_id, 'conversation_id', c.conv, 'conversation_known', true,
        'peer_key', c.peer_key, 'outbound_id', p_outbound, 'body_hmac', encode(p_body_hmac, 'hex'), 'policy', 'lcn-policy/v1',
        'takeover_generation', v_gen, 'principal_id', v_p, 'bundle_id', p_bundle, 'session_id', p_session, 'reminder_state', p_reminder_state,
        -- The authoritative order whose link the reminder seals; claimed reminders have NULL, never a buyer-supplied id.
        'order_id', f.order_id, 'template_id', p_template_id, 'template_version', p_template_version,
        'deadline_at', to_char((st.last_inbound_at + interval '24 hours' - interval '5 minutes') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
    -- Calls inbox.lcn_emit (0128): shared tail, with exactly the original operation/job/secret/outbound/audit writes.
    PERFORM inbox.lcn_emit(v_t, v_s, v_p, 'dm', 'meta.dm_send', z.id, z.semantic_version, v_provider, c.asset_id, v_key,
        v_req, p_operation, p_job, p_outbound, c.conv, NULL, p_body_hmac, p_display_key_id, p_display_nonce,
        p_display_ciphertext, p_secret_enc, p_secret_sealed, p_template_id, p_template_version, 'inbox.checkout_reminder.planned');
    RETURN p_operation;
END $$;
ALTER FUNCTION inbox.plan_checkout_reminder(uuid, uuid, uuid, text, bigint, text, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint)
    OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.plan_checkout_reminder(uuid, uuid, uuid, text, bigint, text, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.plan_checkout_reminder(uuid, uuid, uuid, text, bigint, text, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint)
    TO commerce_runtime;
COMMENT ON FUNCTION inbox.plan_checkout_reminder(uuid, uuid, uuid, text, bigint, text, uuid, bigint, uuid, bytea, text, bytea, bytea, bytea, bytea, text, bigint) IS
 'internal/inbox (live-console-v1 Amendment W3-03B + PR32/0171): re-verifies inbox:reply, bundle/peer/window/binding/state and plans exactly one sealed reminder with authoritative crm_bundle_facts.order_id. Caller verifies persisted order matches its issued link before commit. Signature, lock order, owner, ACL and shared lcn_emit writes unchanged.';

CREATE FUNCTION inbox.order_pay_link_pending(p_order uuid) RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
BEGIN
    v := inbox.lcn_scope();
    IF p_order IS NULL THEN RAISE EXCEPTION 'invalid order' USING ERRCODE = '22023'; END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inventory:reserve','catalog:read']) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    RETURN EXISTS (SELECT 1 FROM integration.operations o
     WHERE o.tenant_id = v[1] AND o.store_id = v[2] AND o.action = 'meta.dm_send'
       AND o.state IN ('READY','DISPATCHING','UNKNOWN','ACKNOWLEDGED')
       AND (o.request->>'order_id' = p_order::text
         OR (coalesce(o.request->>'order_id','') = '' AND o.request->>'message_type' = 'checkout_reminder'
          AND o.request->>'reminder_state' = 'awaiting_payment'
          -- Never guess the current/latest order of a legacy bundle. All of its immutable origin orders stay protected.
          AND EXISTS (SELECT 1 FROM claims.order_origins x
           WHERE x.tenant_id = v[1] AND x.store_id = v[2] AND x.order_id = p_order
             AND (x.bundle_id::text = o.request->>'bundle_id'
               OR EXISTS (SELECT 1 FROM inbox.checkout_reminders r
                WHERE r.tenant_id = v[1] AND r.store_id = v[2] AND r.operation_id = o.id AND r.bundle_id = x.bundle_id))))));
END $$;
ALTER FUNCTION inbox.order_pay_link_pending(uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.order_pay_link_pending(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.order_pay_link_pending(uuid) TO commerce_runtime;
COMMENT ON FUNCTION inbox.order_pay_link_pending(uuid) IS
 'merchanttools.guardOrderPayLink (storefront-v2 queued payment link guard; PR32/0171): authenticated tenant/store and inventory:reserve+catalog:read re-verified. Read-only same-order nonterminal DM existence; legacy awaiting-payment reminders correlate only through immutable origins using their request bundle or same-store reminder journal. No private identity, token, row or new table privilege exposed; caller holds the order-pay-link lock until commit.';
