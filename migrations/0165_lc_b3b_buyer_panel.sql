-- File: migrations/0165_lc_b3b_buyer_panel.sql
-- Purpose: LC-B3b inbox read gaps. A13 buyer panel read model (peer-linked claims + recorded live-price totals,
--   the bundles' orders, purchase ordinal, newest automated private-reply state); A8 session_id filter,
--   filter=live_comment bundle-only rows and per-item link_version; A9 support (conversation_meta gains
--   link_version; inbox.conversation_binding resolves the send binding id). Additive contract change only:
--   the two social readers are DROP+CREATEd with a new IN parameter (session filter) and one added result
--   column each, re-running the 0119 owner/revoke/grant exactly. No permission changes; cross-tenant/store
--   reads 404 (LCN03); every result is bounded (50 claims / 20 orders / 50 rows).
-- Depends on: 0008 (integration.bindings/operations + writer access), 0013 (checkout.orders + commerce_checkout_writer),
--   0060/0064 (claims.bundles/lines, live.offers keyword grants, schema USAGE), 0105/0129 (claims.live_price_uses +
--   unit_price_minor, inbox.order_for_buyer, claims_writer fact grants), 0107 (orders state vocabulary),
--   0119 (social readers, inbox.conversation_state.version, inbox schema USAGE), 0128 (inbox.lcn_scope,
--   identity.principal_holds, inbox.bundle_peers, integration_writer grants on claims.bundles/social.conversations).
-- Used by: internal/inbox/read.go (A8/A9/A13 DTO fill), internal/httpapi/inbox.go (orders:read gating),
--   tests/foundation/live_console_buyer_panel_test.go (gates + exact-ACL pins),
--   tests/foundation/live_console_inbox_test.go (updated signature pins).
-- Invariants: I09 (identity link is ONLY inbox.bundle_peers + the explicit A14 linked_customer_id; claims.bundles.owner_id
--   never feeds the panel), IR-16-style tenant+store filtering inside every definer, 0110 cross-domain ruling (the domain
--   owns its projection; a foreign reader definer receives EXECUTE, never new table privileges beyond the two column
--   reads granted below), LCN03 (out of scope = 404), §4.4 send-state mapping stays in Go.
-- Tests: bash scripts/dev/test-focused.sh 'TestLiveConsoleBuyerPanel'; TestLiveConsoleInboxMigration0122ExactACL;
--   TestR2IntegrationUpgradeFromReleaseHead (89 -> 90).

-- ---------------------------------------------------------------------------------------
-- Preconditions: every predecessor object this file extends must exist (fail loudly, never half-apply).
-- ---------------------------------------------------------------------------------------
DO $$
BEGIN
    IF to_regprocedure('inbox.lcn_scope()') IS NULL THEN
        RAISE EXCEPTION '0165 requires 0128 (inbox.lcn_scope)';
    END IF;
    IF to_regprocedure('identity.principal_holds(uuid,uuid,uuid,text[])') IS NULL THEN
        RAISE EXCEPTION '0165 requires 0128 (identity.principal_holds)';
    END IF;
    IF to_regprocedure('social.list_conversations(text,timestamptz,uuid,int)') IS NULL THEN
        RAISE EXCEPTION '0165 requires 0119 (social.list_conversations, 4-argument form)';
    END IF;
    IF to_regprocedure('social.conversation_meta(uuid)') IS NULL THEN
        RAISE EXCEPTION '0165 requires 0119 (social.conversation_meta)';
    END IF;
    IF to_regclass('inbox.bundle_peers') IS NULL THEN
        RAISE EXCEPTION '0165 requires 0128 (inbox.bundle_peers)';
    END IF;
    IF to_regclass('inbox.order_for_buyer') IS NULL THEN
        RAISE EXCEPTION '0165 requires 0129 (inbox.order_for_buyer)';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                    WHERE table_schema = 'claims' AND table_name = 'live_price_uses' AND column_name = 'unit_price_minor') THEN
        RAISE EXCEPTION '0165 requires 0129 (claims.live_price_uses.unit_price_minor)';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                    WHERE table_schema = 'claims' AND table_name = 'bundles' AND column_name = 'purged_at') THEN
        RAISE EXCEPTION '0165 requires claims.bundles.purged_at (0127/0129 retention)';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                    WHERE table_schema = 'inbox' AND table_name = 'conversation_state' AND column_name = 'version') THEN
        RAISE EXCEPTION '0165 requires 0119 (inbox.conversation_state.version)';
    END IF;
END $$;

-- ---------------------------------------------------------------------------------------
-- IR-16 additive grants (0110 ruling: the domain owns its projection; a cross-domain reader definer gets
-- EXECUTE, and the only new table privileges are column reads its own definer genuinely needs):
--   1. claims.buyer_panel_claims orders claims by session age -> claims_writer reads live.sessions.created_at
--      (0129 already grants id/tenant_id/store_id/lifecycle and created the session_claims_read policy).
--   2. inbox.buyer_panel/live_comment_bundles read the bundle platform and exclude retention-purged bundles ->
--      integration_writer reads claims.bundles.platform/purged_at (0128 granted the other columns + bundle_send_read).
--   3. schema USAGE so the definer chain can resolve the helper functions (USAGE alone exposes no table).
-- ---------------------------------------------------------------------------------------
GRANT SELECT(created_at) ON live.sessions TO commerce_claims_writer;
GRANT SELECT(platform, purged_at) ON claims.bundles TO commerce_integration_writer;
GRANT USAGE ON SCHEMA claims, checkout TO commerce_integration_writer;
GRANT USAGE ON SCHEMA claims TO commerce_meta_writer;

-- ---------------------------------------------------------------------------------------
-- Claims-domain projections for the A13 buyer panel. Owner commerce_claims_writer (NOLOGIN definer),
-- EXECUTE commerce_integration_writer (the panel definer calls them). Tenant+store come from the caller's
-- already-validated scope; no GUC reads here because the caller (inbox.buyer_panel) enforces lcn_scope.
-- ---------------------------------------------------------------------------------------

-- A13 claims: the accepted claim lines of the given bundles, newest session first (session created_at DESC,
-- keyword ASC, offer_id ASC as deterministic tiebreaks), capped at 50 rows. claim_total_minor sums
-- quantity x the unit price the claim RECORDED (claims.live_price_uses.unit_price_minor of the newest use
-- of that (bundle, offer), newest by its order's created_at; 0 when never recorded) over the FULL set, including
-- lines beyond the 50-row cap (ruling LC-B3b: never recomputed from the current catalogue).
CREATE FUNCTION claims.buyer_panel_claims(p_tenant uuid, p_store uuid, p_bundles uuid[])
RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_out jsonb;
BEGIN
    IF p_tenant IS NULL OR p_store IS NULL OR p_bundles IS NULL OR cardinality(p_bundles) = 0 THEN
        RETURN jsonb_build_object('claims', '[]'::jsonb, 'claim_total_minor', 0);
    END IF;
    WITH ranked AS (
        SELECT b.session_id, l.offer_id, o.keyword, l.quantity, s.created_at AS session_created,
               (SELECT u.unit_price_minor
                  FROM claims.live_price_uses u
                  JOIN checkout.orders co ON co.tenant_id = u.tenant_id AND co.store_id = u.store_id AND co.id = u.order_id
                 WHERE u.tenant_id = l.tenant_id AND u.store_id = l.store_id
                   AND u.bundle_id = l.bundle_id AND u.offer_id = l.offer_id
                   AND u.unit_price_minor IS NOT NULL
                 ORDER BY co.created_at DESC, u.order_id DESC
                 LIMIT 1) AS unit_price
          FROM claims.lines l
          JOIN claims.bundles b ON b.tenant_id = l.tenant_id AND b.store_id = l.store_id AND b.id = l.bundle_id
          JOIN live.offers o ON o.tenant_id = l.tenant_id AND o.store_id = l.store_id
               AND o.session_id = b.session_id AND o.id = l.offer_id
          JOIN live.sessions s ON s.tenant_id = b.tenant_id AND s.store_id = b.store_id AND s.id = b.session_id
         WHERE l.tenant_id = p_tenant AND l.store_id = p_store AND l.bundle_id = ANY(p_bundles))
    SELECT jsonb_build_object(
               'claims', (SELECT coalesce(jsonb_agg(jsonb_build_object(
                               'session_id', x.session_id, 'offer_id', x.offer_id,
                               'keyword', x.keyword, 'quantity', x.quantity) ORDER BY x.rn), '[]'::jsonb)
                            FROM (SELECT r.*, row_number() OVER (ORDER BY r.session_created DESC, r.keyword, r.offer_id) AS rn
                                    FROM ranked r) x
                           WHERE x.rn <= 50),
               'claim_total_minor', (SELECT coalesce(sum(r.quantity::bigint * coalesce(r.unit_price, 0)), 0)::bigint
                                       FROM ranked r))
      INTO v_out;
    RETURN v_out;
END $$;
ALTER FUNCTION claims.buyer_panel_claims(uuid, uuid, uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.buyer_panel_claims(uuid, uuid, uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.buyer_panel_claims(uuid, uuid, uuid[]) TO commerce_integration_writer;
COMMENT ON FUNCTION claims.buyer_panel_claims(uuid, uuid, uuid[]) IS
 'internal/claims (0165 LC-B3b; caller: the inbox.buyer_panel definer, owner commerce_integration_writer): A13 claims projection of the given bundles. {"claims":[{session_id,offer_id,keyword,quantity}]} newest session first, at most 50; "claim_total_minor" = sum(quantity x newest recorded live_price_uses.unit_price_minor, 0 when none) over the FULL line set. I09: bundle ids come only from inbox.bundle_peers/A14 links resolved by the caller; owner_id never feeds this projection. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to integration_writer only.';

-- A13 orders of the given bundles: the claim-checkout ledgers (order_origins: every claim checkout incl. price-neutral; live_price_uses) unioned with the A16
-- for-buyer ledger (inbox.order_for_buyer), any order state (the panel filters/ordinals downstream).
CREATE FUNCTION claims.orders_of_bundles(p_tenant uuid, p_store uuid, p_bundles uuid[])
RETURNS TABLE(order_id uuid)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF p_tenant IS NULL OR p_store IS NULL OR p_bundles IS NULL OR cardinality(p_bundles) = 0 THEN
        RETURN;
    END IF;
    RETURN QUERY
    SELECT u.order_id
      FROM claims.live_price_uses u
     WHERE u.tenant_id = p_tenant AND u.store_id = p_store AND u.bundle_id = ANY(p_bundles)
    UNION
    SELECT oo.order_id
      FROM claims.order_origins oo
     WHERE oo.tenant_id = p_tenant AND oo.store_id = p_store AND oo.bundle_id = ANY(p_bundles)
    UNION
    SELECT f.order_id
      FROM inbox.order_for_buyer f
     WHERE f.tenant_id = p_tenant AND f.store_id = p_store
       AND f.bundle_id = ANY(p_bundles) AND f.order_id IS NOT NULL;
END $$;
ALTER FUNCTION claims.orders_of_bundles(uuid, uuid, uuid[]) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.orders_of_bundles(uuid, uuid, uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.orders_of_bundles(uuid, uuid, uuid[]) TO commerce_integration_writer;
COMMENT ON FUNCTION claims.orders_of_bundles(uuid, uuid, uuid[]) IS
 'internal/claims (0165 LC-B3b; caller: the inbox.buyer_panel definer, owner commerce_integration_writer): DISTINCT order ids created from the given bundles, via claims.order_origins / claims.live_price_uses (claim checkout, price-neutral included) or inbox.order_for_buyer (A16). Any state; facts come from checkout.order_panel_facts. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to integration_writer only.';

-- A8 session filter support: does this session have at least one non-purged bundle whose bundle_peers row
-- matches the conversation's peer (same app/object/asset/peer_key)? Called by the social.list_conversations
-- definer (owner commerce_meta_writer) per candidate conversation. I09: bundle_peers is the only link.
CREATE FUNCTION claims.session_peer_linked(p_tenant uuid, p_store uuid, p_session uuid,
    p_app_id text, p_object text, p_asset_id text, p_peer_key text)
RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1
          FROM inbox.bundle_peers bp
          JOIN claims.bundles b ON b.tenant_id = bp.tenant_id AND b.store_id = bp.store_id
               AND b.id = bp.bundle_id AND b.purged_at IS NULL
         WHERE bp.tenant_id = p_tenant AND bp.store_id = p_store
           AND bp.app_id = p_app_id AND bp.object = p_object
           AND bp.asset_id = p_asset_id AND bp.peer_key = p_peer_key
           AND b.session_id = p_session);
END $$;
ALTER FUNCTION claims.session_peer_linked(uuid, uuid, uuid, text, text, text, text) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.session_peer_linked(uuid, uuid, uuid, text, text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.session_peer_linked(uuid, uuid, uuid, text, text, text, text) TO commerce_meta_writer;
COMMENT ON FUNCTION claims.session_peer_linked(uuid, uuid, uuid, text, text, text, text) IS
 'internal/claims (0165 LC-B3b; caller: the social.list_conversations definer, owner commerce_meta_writer): TRUE when the session has a non-purged bundle whose inbox.bundle_peers row matches the given peer (app/object/asset/peer_key). The A8 ?session_id= filter. I09: bundle_peers only; owner_id/name matching never. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to meta_writer only.';

-- ---------------------------------------------------------------------------------------
-- Checkout-domain facts for the A13 orders/ordinal. Owner commerce_checkout_writer, EXECUTE
-- commerce_integration_writer. Zero new table grants: 0013 already gives checkout_writer the
-- writer_access USING(true) policy and full SELECT on checkout.orders (0110 ruling).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION checkout.order_panel_facts(p_tenant uuid, p_store uuid, p_orders uuid[])
RETURNS TABLE(order_id uuid, commercial_state text, total_minor bigint, created_at timestamptz)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF p_tenant IS NULL OR p_store IS NULL OR p_orders IS NULL OR cardinality(p_orders) = 0 THEN
        RETURN;
    END IF;
    RETURN QUERY
    SELECT o.id, o.commercial_state, o.total_minor, o.created_at
      FROM checkout.orders o
     WHERE o.tenant_id = p_tenant AND o.store_id = p_store AND o.id = ANY(p_orders);
END $$;
ALTER FUNCTION checkout.order_panel_facts(uuid, uuid, uuid[]) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_panel_facts(uuid, uuid, uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_panel_facts(uuid, uuid, uuid[]) TO commerce_integration_writer;
COMMENT ON FUNCTION checkout.order_panel_facts(uuid, uuid, uuid[]) IS
 'internal/checkout (0165 LC-B3b; caller: the inbox.buyer_panel definer, owner commerce_integration_writer): (id, commercial_state, total_minor, created_at) of the given orders within tenant+store. No snapshot, destination or PII columns. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to integration_writer only.';

-- ---------------------------------------------------------------------------------------
-- §3.2 read authority, extended (LC-B3b). Both 0119 readers are DROP+CREATEd: list_conversations gains the
-- p_session IN parameter and a 10th result column link_version (= inbox.conversation_state.version, the A14
-- CAS version); conversation_meta gains link_version as its 12th column. Owner commerce_meta_writer,
-- EXECUTE commerce_runtime, access checked at the start via inbox.principal_holds([''inbox:read'']),
-- tenant+store from the GUCs exactly as before. The session filter delegates to
-- claims.session_peer_linked (bundle_peers is the only identity link, I09).
-- ---------------------------------------------------------------------------------------
DROP FUNCTION social.list_conversations(text, timestamptz, uuid, int);
CREATE FUNCTION social.list_conversations(p_filter text, p_session uuid, p_cursor_last_at timestamptz, p_cursor_id uuid, p_limit int)
RETURNS TABLE(conversation_id uuid, platform text, last_at timestamptz, unread boolean, unreplied boolean,
 mode text, assignee uuid, window_open_until timestamptz, linked_customer_id uuid, link_version bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
BEGIN
    IF p_filter IS NULL OR p_filter NOT IN ('all','unreplied','messenger','instagram','live_comment') THEN
        RAISE EXCEPTION 'invalid_filter' USING ERRCODE = 'PT400';
    END IF;
    IF p_limit IS NULL OR p_limit < 1 OR p_limit > 50 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    RETURN QUERY
    SELECT c.id,
           CASE c.object WHEN 'page' THEN 'messenger' ELSE 'instagram' END,
           GREATEST(s.last_inbound_at, COALESCE(s.last_outbound_at, '-infinity'::timestamptz)),
           COALESCE(s.last_inbound_seq, 0) > s.read_seq,
           s.last_inbound_at > COALESCE(s.last_outbound_at, '-infinity'::timestamptz),
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN 'auto' ELSE s.mode END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN NULL ELSE s.assignee_principal END,
           s.last_inbound_at + interval '24 hours',
           s.customer_id,
           s.version
      FROM social.conversations c
      JOIN inbox.conversation_state s ON s.tenant_id = c.tenant_id AND s.store_id = c.store_id AND s.conversation_id = c.id
     WHERE c.tenant_id = v_tenant AND c.store_id = v_store
       AND (p_filter = 'all'
         OR (p_filter = 'messenger' AND c.object = 'page')
         OR (p_filter = 'instagram' AND c.object = 'instagram')
         OR (p_filter = 'unreplied' AND s.last_inbound_at > COALESCE(s.last_outbound_at, '-infinity'::timestamptz)))
       AND (p_session IS NULL OR claims.session_peer_linked(v_tenant, v_store, p_session,
            c.app_id, c.object, c.asset_id, c.peer_key))
       AND (p_cursor_last_at IS NULL OR p_cursor_id IS NULL OR
            (GREATEST(s.last_inbound_at, COALESCE(s.last_outbound_at, '-infinity'::timestamptz)), c.id) < (p_cursor_last_at, p_cursor_id))
     ORDER BY GREATEST(s.last_inbound_at, COALESCE(s.last_outbound_at, '-infinity'::timestamptz)) DESC, c.id DESC
     LIMIT p_limit;
END $$;

-- A9 header + A13 conversation-scoped fields. No ciphertext. link_version is additive (LC-B3b).
DROP FUNCTION social.conversation_meta(uuid);
CREATE FUNCTION social.conversation_meta(p_conversation uuid)
RETURNS TABLE(platform text, mode text, assignee uuid, takeover_generation bigint, human_until timestamptz,
 window_open_until timestamptz, last_inbound_at timestamptz, last_inbound_seq bigint, read_seq bigint,
 linked_customer_id uuid, status text, link_version bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
BEGIN
    IF NOT inbox.principal_holds(ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    RETURN QUERY
    SELECT CASE c.object WHEN 'page' THEN 'messenger' ELSE 'instagram' END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN 'auto' ELSE s.mode END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN NULL ELSE s.assignee_principal END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN s.takeover_generation + 1 ELSE s.takeover_generation END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN NULL ELSE s.human_until END,
           s.last_inbound_at + interval '24 hours',
           s.last_inbound_at, s.last_inbound_seq, s.read_seq,
           s.customer_id, s.status,
           s.version
      FROM social.conversations c
      JOIN inbox.conversation_state s ON s.tenant_id = c.tenant_id AND s.store_id = c.store_id AND s.conversation_id = c.id
     WHERE c.id = p_conversation AND c.tenant_id = v_tenant AND c.store_id = v_store;
END $$;

DO $$
DECLARE f text;
BEGIN
    FOREACH f IN ARRAY ARRAY['social.list_conversations(text,uuid,timestamptz,uuid,int)','social.conversation_meta(uuid)'] LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_meta_writer', f);
        EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC', f);
        EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime', f);
    END LOOP;
END $$;

COMMENT ON FUNCTION social.list_conversations(text, uuid, timestamptz, uuid, int) IS
 'internal/inbox ListConversations (0119, re-created by 0165 LC-B3b): A8 conversation metadata, no ciphertext. Adds p_session (keep conversations whose peer is inbox.bundle_peers-linked to a non-purged bundle of that session, via claims.session_peer_linked; I09) and the link_version result column (inbox.conversation_state.version, the A14 expected_version). inbox:read required; tenant/store from the GUCs; limit 1..50. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to runtime only.';
COMMENT ON FUNCTION social.conversation_meta(uuid) IS
 'internal/inbox conversationMeta (0119, re-created by 0165 LC-B3b): A9 header / A13 conversation fields, no ciphertext; adds link_version (inbox.conversation_state.version, the A14 expected_version). inbox:read required; zero rows outside the caller''s tenant/store. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to runtime only.';

-- ---------------------------------------------------------------------------------------
-- Integration-domain definers for the console. Owner commerce_integration_writer, EXECUTE commerce_runtime.
-- Scope comes from inbox.lcn_scope() (validates isolation + the tenant/store/principal GUCs and rejects a
-- buyer context); access via identity.principal_holds. All bounded.
-- ---------------------------------------------------------------------------------------

-- A8 filter=live_comment: bundle-only rows of the store's live comments in scope (facebook/instagram bundles,
-- non-purged), newest first. No conversation_id (comments have no DM conversation projection); link_version is
-- exposed as explicit null by the Go marshaller for these rows (Amendment 1 P2-2 envelope).
CREATE FUNCTION inbox.live_comment_bundles(p_session uuid, p_limit int)
RETURNS TABLE(bundle_id uuid, session_id uuid, platform text, created_at timestamptz, link_pending_manual boolean)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
BEGIN
    v := inbox.lcn_scope();
    IF p_limit IS NULL OR p_limit < 1 OR p_limit > 50 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    RETURN QUERY
    SELECT b.id, b.session_id, b.platform, b.created_at, b.link_pending_manual
      FROM claims.bundles b
     WHERE b.tenant_id = v[1] AND b.store_id = v[2]
       AND b.platform IN ('facebook','instagram')
       AND b.purged_at IS NULL
       AND (p_session IS NULL OR b.session_id = p_session)
     ORDER BY b.created_at DESC, b.id DESC
     LIMIT p_limit;
END $$;
ALTER FUNCTION inbox.live_comment_bundles(uuid, int) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.live_comment_bundles(uuid, int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.live_comment_bundles(uuid, int) TO commerce_runtime;
COMMENT ON FUNCTION inbox.live_comment_bundles(uuid, int) IS
 'internal/inbox (0165 LC-B3b; caller: internal/inbox ListConversations filter=live_comment): bundle-only A8 rows of the store''s live comments (facebook/instagram bundles, non-purged), newest first, optional session filter, limit 1..50. inbox:read required. Raw claims platform values (facebook/instagram); the Go layer maps facebook to the console name messenger. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to runtime only.';

-- A9 binding_id: the binding the send path would use for this conversation (plan_dm rule: provider from the
-- conversation object, matching external_asset_id, enabled; first by id). NULL when the store has no such
-- binding. Read-only echo of the capability row; send-time authority stays with plan_dm's own checks.
CREATE FUNCTION inbox.conversation_binding(p_conversation uuid) RETURNS uuid
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
        v_provider text;
        v_asset text;
        v_binding uuid;
BEGIN
    v := inbox.lcn_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    SELECT CASE c.object WHEN 'page' THEN 'facebook' ELSE 'instagram' END, c.asset_id
      INTO v_provider, v_asset
      FROM social.conversations c
     WHERE c.id = p_conversation AND c.tenant_id = v[1] AND c.store_id = v[2];
    IF NOT FOUND THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    SELECT z.id INTO v_binding
      FROM integration.bindings z
     WHERE z.tenant_id = v[1] AND z.store_id = v[2]
       AND z.provider = v_provider AND z.external_asset_id = v_asset AND z.enabled
     ORDER BY z.id
     LIMIT 1;
    RETURN v_binding;
END $$;
ALTER FUNCTION inbox.conversation_binding(uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.conversation_binding(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.conversation_binding(uuid) TO commerce_runtime;
COMMENT ON FUNCTION inbox.conversation_binding(uuid) IS
 'internal/inbox (0165 LC-B3b; caller: internal/inbox ReadThread for the A9 header): the send binding of this conversation (plan_dm provider/asset rule, enabled, first by id) or NULL. 404 outside the caller''s tenant/store (LCN03). inbox:read required. Read-only; send-time capability checks stay in plan_dm. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to runtime only.';

-- A13 buyer panel facts. Exactly one of p_conversation / p_bundle (PT400 otherwise); 404 when the target is
-- outside the caller's tenant/store or the bundle is purged (LCN03). Bundles: for a conversation, every
-- bundle whose bundle_peers row matches the conversation's peer (I09: owner_id never contributes; a
-- conversation without links answers empty, 200 — P2-7c); for a bundle id, that bundle only.
-- Returns {claims, claim_total_minor, purchase_ordinal, link_pending_manual, platform} plus "orders" ONLY
-- when the principal holds orders:read (decided HERE, not by the caller; omission is the ruling),
-- plus "auto_reply_state" (raw integration.operations.state; §4.4 mapping stays in Go) when the bundles have
-- an automated private-reply operation (origin_kind=auto, newest first; manual sends never count).
CREATE FUNCTION inbox.buyer_panel(p_conversation uuid, p_bundle uuid)
RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
        v_app text; v_obj text; v_asset text; v_peer text;
        v_platform text;
        v_bundles uuid[];
        v_facts jsonb;
        v_order_ids uuid[];
        v_orders jsonb;
        v_auto text;
        v_out jsonb;
BEGIN
    v := inbox.lcn_scope();
    IF (p_conversation IS NULL) = (p_bundle IS NULL) THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT400';
    END IF;
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    IF p_bundle IS NOT NULL THEN
        SELECT b.platform INTO v_platform
          FROM claims.bundles b
         WHERE b.tenant_id = v[1] AND b.store_id = v[2] AND b.id = p_bundle AND b.purged_at IS NULL;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
        END IF;
        v_bundles := ARRAY[p_bundle];
    ELSE
        SELECT c.app_id, c.object, c.asset_id, c.peer_key INTO v_app, v_obj, v_asset, v_peer
          FROM social.conversations c
         WHERE c.id = p_conversation AND c.tenant_id = v[1] AND c.store_id = v[2];
        IF NOT FOUND THEN
            RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
        END IF;
        v_platform := CASE v_obj WHEN 'page' THEN 'messenger' ELSE 'instagram' END;
        SELECT coalesce(array_agg(b.id), ARRAY[]::uuid[]) INTO v_bundles
          FROM inbox.bundle_peers bp
          JOIN claims.bundles b ON b.tenant_id = bp.tenant_id AND b.store_id = bp.store_id
               AND b.id = bp.bundle_id AND b.purged_at IS NULL
         WHERE bp.tenant_id = v[1] AND bp.store_id = v[2]
           AND bp.app_id = v_app AND bp.object = v_obj
           AND bp.asset_id = v_asset AND bp.peer_key = v_peer;
    END IF;
    v_facts := claims.buyer_panel_claims(v[1], v[2], v_bundles);
    SELECT coalesce(array_agg(x.order_id), ARRAY[]::uuid[]) INTO v_order_ids
      FROM claims.orders_of_bundles(v[1], v[2], v_bundles) AS x(order_id);
    v_out := v_facts || jsonb_build_object(
        'platform', v_platform,
        'purchase_ordinal', (SELECT count(*) FROM checkout.order_panel_facts(v[1], v[2], v_order_ids) f
                              WHERE f.commercial_state IN ('CONFIRMED','AWAITING_COLLECTION')),
        'link_pending_manual', EXISTS (SELECT 1 FROM claims.bundles b
                                        WHERE b.tenant_id = v[1] AND b.store_id = v[2]
                                          AND b.id = ANY(v_bundles) AND b.link_pending_manual));
    IF identity.principal_holds(v[1], v[2], v[3], ARRAY['orders:read']::text[]) THEN
        SELECT coalesce(jsonb_agg(jsonb_build_object(
                'order_id', f.order_id,
                'number', 'LC-' || upper(replace(f.order_id::text, '-', '')),
                'state', f.commercial_state,
                'total_minor', f.total_minor,
                'created_at', to_char(f.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'))
            ORDER BY f.created_at DESC, f.order_id DESC), '[]'::jsonb) INTO v_orders
          FROM (SELECT * FROM checkout.order_panel_facts(v[1], v[2], v_order_ids)
                 ORDER BY created_at DESC, order_id DESC LIMIT 20) f;
        v_out := v_out || jsonb_build_object('orders', v_orders);
    END IF;
    SELECT o.state INTO v_auto
      FROM integration.operations o
     WHERE o.tenant_id = v[1] AND o.store_id = v[2]
       AND o.action = 'meta.private_reply'
       AND o.request->>'origin_kind' = 'auto'
       AND o.request->>'bundle_id' IN (SELECT g::text FROM unnest(v_bundles) g)
     ORDER BY o.created_at DESC, o.id DESC
     LIMIT 1;
    IF FOUND THEN
        v_out := v_out || jsonb_build_object('auto_reply_state', v_auto);
    END IF;
    RETURN v_out;
END $$;
ALTER FUNCTION inbox.buyer_panel(uuid, uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.buyer_panel(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.buyer_panel(uuid, uuid) TO commerce_runtime;
COMMENT ON FUNCTION inbox.buyer_panel(uuid, uuid) IS
 'internal/inbox (0165 LC-B3b; caller: internal/inbox BuyerPanel/BuyerPanelByBundle): the A13 buyer panel facts of a conversation (via inbox.bundle_peers only — I09) or a single bundle. {claims <=50, claim_total_minor (recorded prices, full set), purchase_ordinal (CONFIRMED/AWAITING_COLLECTION over the full order set), link_pending_manual, platform} + "orders" (<=20, LC- numbers) only when the principal holds orders:read + "auto_reply_state" of the newest automated private reply. 404 outside scope (LCN03); empty links answer empty (P2-7c). STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to runtime only.';
