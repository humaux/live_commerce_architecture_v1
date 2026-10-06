-- 0131_checkout_reminders.sql — W3-03B checkout reminders (live-console-v1 Amendment W3-03B; brief docs/delivery/units/w3-03b-checkout-reminder.md).
-- Purpose: after a live session (or when the merchant clicks 「提醒未付款」) remind buyers who claimed but did not order, or ordered but did not pay,
--   by ONE Messenger/IG DM per buyer per session, ONLY inside the 24 h window (messaging_type RESPONSE; no message tag, no UPDATE, no utility
--   template, ever). The DM carries a link that really completes the purchase: the buyer's unpaid merchant-created order -> a re-issued order link
--   (order-pay-link/v1, fulfillment.regenerate_order_link rules), a never-opened claim -> a re-issued claim link (checkout-reminder/v1,
--   claims.issue_link). Buyers outside the window / under human takeover / without a known thread / without a re-issuable link land in a
--   follow-up list instead. Adds the fixed template checkout-reminder/v1, the reminder ledger table, the (unused) per-store settings table and the definers.
-- Depends on: 0121 (msgtemplates.fixed_templates), 0128 (inbox.lcn_scope/lcn_emit/lcn_rate_check, inbox.bundle_peers, inbox.check_send, outbound_messages,
--   send_secrets, the meta.dm_send worker route), 0119 (inbox.conversation_state), 0113 (claims.order_origins), 0013/0094 (checkout.orders, source),
--   0060 (claims.links generation; the link itself is issued by claims.issue_link / fulfillment.regenerate_order_link in the same merchant transaction).
-- Used by: internal/inbox/reminders.go, internal/merchanttools/checkout_reminders.go, internal/httpapi/reminders.go (POST/GET .../live-sessions/{sid}/reminders).
-- Invariants: LCN06 (no send outside the window; re-checked at Check by inbox.check_send), I06/I07 (the send is a meta.dm_send operation of the
--   existing ledger: no second send path), one reminder per buyer per session (semantic key 'crm:' + sha256(session|owner-or-bundle)), the
--   dispatch copy, the PSID and every bearer link never reach SQL in plaintext (sealed in Go, plan_dm pattern), A1.3 (lcn-dup lock first, the 60/min store
--   cap counts reminders too).
-- Deviations from the brief (recorded in output/w3-03b-checkout-reminder/DELIVERY.md): (1) planning is a candidate scan, then one transaction PER BUYER
--   (issue link + plan), because the dispatch copy is sealed in Go with the payload keyring + the PSID (API-only custody) and one buyer's race must not
--   refuse the batch; (2) origin=auto DMs never take a conversation over (plan_dm is the human variant), so inbox.plan_checkout_reminder is the auto variant
--   of plan_dm and shares inbox.lcn_emit / inbox.check_send / the dm route; (3) own bundle_peers join instead of dm_window_for_bundle (needs the conversation
--   id); (4) no table is readable by commerce_runtime (0128 convention: definers only); (5) the automatic (River) leg and the settings routes are DEFERRED
--   (inbox payload ring stays API-only): live.reminder_settings is kept as an unused table so the settings UI can be added with the automatic path.
-- Template-id CHECK note: W3-04B (sold-out reply, 0132) also extends msgtemplates.fixed_templates' template_id CHECK. Both units rewrite the
--   same constraint, so the integrator serialises them: the final list is the union of every fixed template id.
-- Status: MOCK (REAL_PG + fake Graph); Meta LIVE = NOT_RUN.

-- ---------------------------------------------------------------------------------------
-- Fixed template checkout-reminder/v1 (dm only, not public-safe: it carries the {{連結}} placeholder the Go planner fills with a CLAIM link).
-- The awaiting-payment state reuses the existing fixed order-pay-link/v1 (LC-B6), whose link is an ORDER link.
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
    reason text CHECK (reason IN ('window_closed', 'human_takeover', 'no_peer', 'capability', 'link_unavailable')),
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

-- DEFERRED: the 「場次結束後 N 分鐘自動提醒」 setting. Nothing reads or writes it (no definer, no route): the automatic send needs a process that may open the
-- inbox payload ring, which stays API-only. Kept (empty) so the automatic path and its settings UI can land without another table migration.
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
GRANT SELECT, INSERT, UPDATE ON inbox.checkout_reminders TO commerce_integration_writer;
CREATE POLICY reminders_rw ON inbox.checkout_reminders FOR ALL TO commerce_integration_writer
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
GRANT SELECT(tenant_id, store_id, id, commercial_state, expires_at, source) ON checkout.orders TO commerce_integration_writer;
CREATE POLICY order_reminder_read ON checkout.orders FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));
-- The current claim-link generation (the merchant link issue is a compare-and-swap on it); the token hash is never readable.
GRANT SELECT(tenant_id, store_id, bundle_id, generation) ON claims.links TO commerce_integration_writer;
CREATE POLICY link_generation_reminder_read ON claims.links FOR SELECT TO commerce_integration_writer
    USING (inbox.lcn_in_scope(tenant_id, store_id));
-- The audit actions of this unit (lcn_emit inserts the planned one, the scan the triggered one).
CREATE POLICY reminder_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id)
        AND action IN ('inbox.checkout_reminder.planned', 'inbox.checkout_reminder.triggered'));

-- ---------------------------------------------------------------------------------------
-- Private helper: what is the bundle's reminder state? state NULL = not a candidate (ordered and paid/cancelled/expired, no claim lines, purged).
-- 'awaiting_payment' = an AWAITING_PAYMENT, unexpired order of the bundle (order_id / order_source name it); 'claimed' = claim lines and no order at
-- all (a cancelled order is an owner decision, never reminded). bound = the claim was opened (the bundle has an owner): its claim link only works in
-- the owner's own browser, so it cannot be re-issued to a thread; link_generation = the current claim-link generation (0 = none yet).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.crm_bundle_facts(p_tenant uuid, p_store uuid, p_bundle uuid)
RETURNS TABLE(state text, order_id uuid, order_source text, bound boolean, link_generation bigint)
LANGUAGE plpgsql STABLE SET search_path = pg_catalog AS $$
DECLARE b record;
BEGIN
    SELECT x.owner_id, x.line_count, x.purged_at, x.platform INTO b FROM claims.bundles x
     WHERE x.tenant_id = p_tenant AND x.store_id = p_store AND x.id = p_bundle;
    IF NOT FOUND OR b.purged_at IS NOT NULL OR b.line_count <= 0 OR b.platform NOT IN ('facebook', 'instagram') THEN
        RETURN QUERY SELECT NULL::text, NULL::uuid, NULL::text, false, 0::bigint;
        RETURN;
    END IF;
    SELECT k.id, k.source INTO order_id, order_source FROM claims.order_origins o
      JOIN checkout.orders k ON k.tenant_id = o.tenant_id AND k.store_id = o.store_id AND k.id = o.order_id
     WHERE o.tenant_id = p_tenant AND o.store_id = p_store AND o.bundle_id = p_bundle
       AND k.commercial_state = 'AWAITING_PAYMENT' AND k.expires_at > clock_timestamp()
     ORDER BY k.expires_at DESC, k.id LIMIT 1;
    bound := b.owner_id IS NOT NULL;
    link_generation := coalesce((SELECT l.generation FROM claims.links l WHERE l.tenant_id = p_tenant AND l.store_id = p_store AND l.bundle_id = p_bundle), 0);
    IF order_id IS NOT NULL THEN
        state := 'awaiting_payment';
    ELSIF NOT EXISTS (SELECT 1 FROM claims.order_origins o WHERE o.tenant_id = p_tenant AND o.store_id = p_store AND o.bundle_id = p_bundle) THEN
        state := 'claimed';
    ELSE
        state := NULL;
    END IF;
    RETURN NEXT;
END $$;
ALTER FUNCTION inbox.crm_bundle_facts(uuid, uuid, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.crm_bundle_facts(uuid, uuid, uuid) FROM PUBLIC;

-- ---------------------------------------------------------------------------------------
-- A1.3 clause 4: reminders count toward the same 60 sends / store / minute as human sends (a 100-buyer batch must not flood Meta).
-- Same signature, owner and ACL as 0128.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION inbox.lcn_rate_check(p_tenant uuid, p_store uuid) RETURNS void
LANGUAGE plpgsql STABLE SET search_path = pg_catalog AS $$
BEGIN
    IF (SELECT count(*) FROM integration.operations o
         WHERE o.tenant_id = p_tenant AND o.store_id = p_store
           AND o.action IN ('meta.dm_send','meta.public_reply','meta.offer_recommend','meta.private_reply')
           AND (o.request->>'origin' = 'human' OR o.request->>'message_type' = 'checkout_reminder')
           AND o.created_at > clock_timestamp() - interval '1 minute') >= 60 THEN
        RAISE EXCEPTION 'rate_limited' USING ERRCODE = 'PT429';
    END IF;
END $$;

-- ---------------------------------------------------------------------------------------
-- inbox.check_send: 0128's Check plus the reminder re-check (same signature, owner and ACL).
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION inbox.check_send(p_operation uuid) RETURNS text
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
        -- 0131 (W3-03B): a checkout reminder is only valid while its buyer is still in the state it was planned for (paid / cancelled / expired /
        -- ordered since planning -> BLOCKED_POLICY not_remindable, zero HTTP, never retried).
        IF r->>'message_type' = 'checkout_reminder' THEN
            SELECT f.state INTO v_key FROM inbox.crm_bundle_facts(o.tenant_id, o.store_id, (r->>'bundle_id')::uuid) f;
            IF v_key IS DISTINCT FROM r->>'reminder_state' THEN RETURN 'not_remindable'; END IF;
        END IF;
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

-- ---------------------------------------------------------------------------------------
-- inbox.checkout_reminder_candidates (merchant transaction, inbox:reply AND live:manage: a claim link is re-issued): enumerate the session's candidate
-- buyers and classify each. verdict: 'send' (window open, auto mode, capability ok, a link can be issued: the caller issues it and plans one DM),
-- 'already_reminded' (a queued reminder exists), or a followup reason (window_closed | human_takeover | no_peer | capability | link_unavailable) which
-- this call records in inbox.checkout_reminders. A bundle is a candidate only when the session's claim window is CLOSED or its claim is at least 10 minutes
-- old (a buyer still shopping in a live show is not nagged). Only 'send' verdicts count against p_limit (1..100), so a later pass always reaches the
-- buyers an earlier pass left (reminded and follow-up buyers cost nothing). p_bundle NULL = the batch; a bundle id = that buyer only (409 not_remindable
-- when it is no candidate). One audit row per call, also when nothing is sent. No row lock is held: inbox.plan_checkout_reminder re-checks under its own.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.checkout_reminder_candidates(p_session uuid, p_trigger text, p_limit int, p_bundle uuid)
RETURNS TABLE(bundle_id uuid, verdict text, reminder_state text, order_id uuid, link_generation bigint, conversation_id uuid,
 takeover_generation bigint, platform text, locale text, truncated boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_t uuid; v_s uuid; v_p uuid; v_now timestamptz := clock_timestamp(); v_locale text; b record; c record; f record; v_key text;
 v_provider text; v_bind uuid; v_sends int := 0; v_emitted int := 0; v_trunc boolean := false; v_mode text; v_gen bigint; v_verdict text;
 v_closed boolean;
BEGIN
    v := inbox.lcn_scope(); v_t := v[1]; v_s := v[2]; v_p := v[3];
    IF p_session IS NULL OR p_trigger IS NULL OR p_trigger NOT IN ('manual', 'auto') OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 100 THEN
        RAISE EXCEPTION 'invalid reminder scan' USING ERRCODE = '22023';
    END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply', 'live:manage']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    SELECT w.state = 'CLOSED' INTO v_closed FROM live.claim_windows w WHERE w.tenant_id = v_t AND w.store_id = v_s AND w.session_id = p_session;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    IF p_bundle IS NOT NULL AND NOT EXISTS (SELECT 1 FROM claims.bundles x WHERE x.tenant_id = v_t AND x.store_id = v_s AND x.session_id = p_session AND x.id = p_bundle) THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    PERFORM pg_advisory_xact_lock(hashtextextended('crm|' || v_s::text || '|' || p_session::text, 0));
    SELECT y.reply_locale INTO v_locale FROM live.claim_sources y
     WHERE y.tenant_id = v_t AND y.store_id = v_s AND y.session_id = p_session ORDER BY y.active DESC, y.updated_at DESC LIMIT 1;
    v_locale := coalesce(v_locale, 'zh-TW');
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (v_t, v_s, v_p, 'inbox.checkout_reminder.triggered');
    FOR b IN SELECT x.id, x.owner_id, x.created_at FROM claims.bundles x
              WHERE x.tenant_id = v_t AND x.store_id = v_s AND x.session_id = p_session AND x.platform IN ('facebook', 'instagram')
                AND x.purged_at IS NULL AND x.line_count > 0 AND (p_bundle IS NULL OR x.id = p_bundle)
              ORDER BY x.created_at, x.id LOOP
        SELECT * INTO f FROM inbox.crm_bundle_facts(v_t, v_s, b.id);
        IF f.state IS NULL THEN CONTINUE; END IF;
        -- Not nagging a buyer who is still in the show: window open and the claim younger than 10 minutes.
        IF NOT v_closed AND b.created_at > v_now - interval '10 minutes' THEN CONTINUE; END IF;
        v_key := 'crm:' || substr(encode(sha256(convert_to(p_session::text || '|' || coalesce(b.owner_id, b.id)::text, 'UTF8')), 'hex'), 1, 48);
        IF EXISTS (SELECT 1 FROM inbox.checkout_reminders r WHERE r.tenant_id = v_t AND r.store_id = v_s AND r.semantic_key = v_key AND r.outcome = 'queued') THEN
            v_emitted := v_emitted + 1;
            RETURN QUERY SELECT b.id, 'already_reminded'::text, f.state, NULL::uuid, NULL::bigint, NULL::uuid, NULL::bigint, NULL::text, v_locale, false;
            CONTINUE;
        END IF;
        -- The buyer's thread = the newest conversation among the bundle's peers (peers exist only after a private reply reached the buyer).
        SELECT cv.id AS conv, cv.object, cv.asset_id, cv.peer_key, s.mode, s.takeover_generation, s.human_until, s.last_inbound_at INTO c
          FROM inbox.bundle_peers bp
          JOIN social.conversations cv ON cv.tenant_id = bp.tenant_id AND cv.store_id = bp.store_id AND cv.app_id = bp.app_id
           AND cv.object = bp.object AND cv.asset_id = bp.asset_id AND cv.peer_key = bp.peer_key
          JOIN inbox.conversation_state s ON s.tenant_id = cv.tenant_id AND s.store_id = cv.store_id AND s.conversation_id = cv.id
         WHERE bp.tenant_id = v_t AND bp.store_id = v_s AND bp.bundle_id = b.id
         ORDER BY s.last_inbound_at DESC NULLS LAST, bp.created_at DESC LIMIT 1;
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
                -- Only a link that completes the purchase may be sent: a merchant-created order (order link re-issue rules) or a claim nobody opened.
                ELSIF (f.state = 'awaiting_payment' AND f.order_source IS DISTINCT FROM 'merchant_manual') OR (f.state = 'claimed' AND f.bound) THEN
                    v_verdict := 'link_unavailable';
                END IF;
            END IF;
        END IF;
        IF v_verdict IS NULL THEN
            v_sends := v_sends + 1;
            IF v_sends > p_limit THEN v_trunc := true; EXIT; END IF;
            v_emitted := v_emitted + 1;
            RETURN QUERY SELECT b.id, 'send'::text, f.state, f.order_id, f.link_generation, c.conv, v_gen, v_provider, v_locale, false;
        ELSE
            v_emitted := v_emitted + 1;
            INSERT INTO inbox.checkout_reminders(tenant_id, store_id, session_id, bundle_id, owner_id, peer_key, semantic_key, outcome, reason,
                reminder_state, trigger, created_at, updated_at)
            VALUES (v_t, v_s, p_session, b.id, b.owner_id, c.peer_key, v_key, 'followup', v_verdict, f.state, p_trigger, v_now, v_now)
            ON CONFLICT ON CONSTRAINT checkout_reminders_bundle_once DO UPDATE
               SET reason = EXCLUDED.reason, reminder_state = EXCLUDED.reminder_state, trigger = EXCLUDED.trigger,
                   peer_key = EXCLUDED.peer_key, updated_at = EXCLUDED.updated_at
             WHERE inbox.checkout_reminders.outcome = 'followup';
            RETURN QUERY SELECT b.id, v_verdict, f.state, NULL::uuid, NULL::bigint, NULL::uuid, NULL::bigint, NULL::text, v_locale, false;
        END IF;
    END LOOP;
    -- A single-buyer request for a bundle that is not (or no longer) a candidate (paid, cancelled, no lines, too young) is refused, not silently empty.
    IF p_bundle IS NOT NULL AND v_emitted = 0 THEN RAISE EXCEPTION 'not_remindable' USING ERRCODE = 'PT409'; END IF;
    IF v_trunc THEN
        RETURN QUERY SELECT NULL::uuid, 'truncated'::text, NULL::text, NULL::uuid, NULL::bigint, NULL::uuid, NULL::bigint, NULL::text, v_locale, true;
    END IF;
END $$;
ALTER FUNCTION inbox.checkout_reminder_candidates(uuid, text, int, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.checkout_reminder_candidates(uuid, text, int, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.checkout_reminder_candidates(uuid, text, int, uuid) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- inbox.plan_checkout_reminder (merchant transaction, inbox:reply): the origin=auto variant of inbox.plan_dm for ONE candidate. The caller has just
-- issued the link inside this same transaction (claims.issue_link / fulfillment.regenerate_order_link), so a refusal here rolls the link back.
-- It re-validates the bundle state (same state as scanned), the CALLER-NAMED conversation (must be a peer of the bundle: no re-pick), the window,
-- the takeover state and the capability under row locks, records the queued reminder (the once-per-buyer guard), and plans the DM through
-- inbox.lcn_emit: the SAME operation / outbound row / sealed dispatch copy / River job path as every other meta.dm_send, so inbox.check_send re-checks
-- window + takeover + bundle state at dispatch (BLOCKED_POLICY window_closed / human_takeover / not_remindable, never retried) and an UNKNOWN send is
-- never repeated. Unlike plan_dm it never takes the conversation over (origin=auto, §3.6). Locks: lcn-dup (A1.3, first), the per-session crm lock,
-- the conversation state FOR SHARE, the binding FOR SHARE. Refusals are PT409 with the deny code as message.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.plan_checkout_reminder(p_session uuid, p_bundle uuid, p_conversation uuid, p_trigger text, p_expected_generation bigint,
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
    -- The template is the one of the buyer's state (a CASE inside an IF condition would confuse the plpgsql parser, hence the separate assignment).
    v_tpl := CASE p_reminder_state WHEN 'claimed' THEN 'checkout-reminder/v1' ELSE 'order-pay-link/v1' END;
    IF p_template_id IS DISTINCT FROM v_tpl THEN RAISE EXCEPTION 'invalid reminder plan' USING ERRCODE = '22023'; END IF;
    IF NOT identity.principal_holds(v_t, v_s, v_p, ARRAY['inbox:reply']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    -- A1.3 clause 1: the lcn-dup lock comes first, before any read for duplicates and before any insert (same key shape as inbox.plan_dm).
    PERFORM pg_advisory_xact_lock(hashtextextended('lcn-dup|' || v_s::text || '|' || p_conversation::text || '|' || encode(p_body_hmac, 'hex'), 0));
    PERFORM inbox.lcn_rate_check(v_t, v_s);
    PERFORM pg_advisory_xact_lock(hashtextextended('crm|' || v_s::text || '|' || p_session::text, 0));
    SELECT x.id, x.owner_id INTO b FROM claims.bundles x
     WHERE x.tenant_id = v_t AND x.store_id = v_s AND x.id = p_bundle AND x.session_id = p_session AND x.platform IN ('facebook', 'instagram');
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    -- crm_bundle_facts also refuses a purged bundle or one without claim lines (state NULL).
    SELECT * INTO f FROM inbox.crm_bundle_facts(v_t, v_s, p_bundle);
    IF f.state IS DISTINCT FROM p_reminder_state THEN RAISE EXCEPTION 'not_remindable' USING ERRCODE = 'PT409'; END IF;
    v_key := 'crm:' || substr(encode(sha256(convert_to(p_session::text || '|' || coalesce(b.owner_id, b.id)::text, 'UTF8')), 'hex'), 1, 48);
    IF EXISTS (SELECT 1 FROM inbox.checkout_reminders r WHERE r.tenant_id = v_t AND r.store_id = v_s AND r.semantic_key = v_key AND r.outcome = 'queued') THEN
        RAISE EXCEPTION 'already_reminded' USING ERRCODE = 'PT409';
    END IF;
    -- The conversation the caller named must be one of this bundle's own peers (the scan picked it; it is verified, never re-picked).
    SELECT cv.id AS conv, cv.object, cv.asset_id, cv.app_id, cv.peer_key INTO c
      FROM inbox.bundle_peers bp
      JOIN social.conversations cv ON cv.tenant_id = bp.tenant_id AND cv.store_id = bp.store_id AND cv.app_id = bp.app_id
       AND cv.object = bp.object AND cv.asset_id = bp.asset_id AND cv.peer_key = bp.peer_key
     WHERE bp.tenant_id = v_t AND bp.store_id = v_s AND bp.bundle_id = p_bundle AND cv.id = p_conversation;
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
        'template_id', p_template_id, 'template_version', p_template_version,
        'deadline_at', to_char((st.last_inbound_at + interval '24 hours' - interval '5 minutes') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'));
    -- Calls inbox.lcn_emit (0128): the shared tail of every meta.dm_send (operation + READY event + outbound display row + sealed secret + audit).
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
