-- 0204 OC-12 inbox unread filter (contracts/tasks.json OC-12, owner choice OC-Q12 Option B; definition output/oc-12/UNREAD.md).
--
-- Purpose: add the A8 filter value 'unread' to social.list_conversations: a conversation is unread when
--   COALESCE(last_inbound_seq, 0) > read_seq (contracts/live-console-v1.md §3.6 / OPEN-9: store-level read_seq, cleared
--   only by inbox:reply holders through inbox.mark_read, re-opened by the social.messages trigger). Same predicate as the
--   per-row unread flag and social.unread_conversation_count(); no new state.
-- Depends on: 0119 (inbox.conversation_state, inbox.principal_holds), 0165 (the current social.list_conversations shape,
--   claims.session_peer_linked).
-- Used by: internal/inbox ListConversations (A8, internal/httpapi/inbox.go) and apps/admin Inbox.tsx through the BFF.
-- Non-goals: no table/column/index/role/permission change; signature, result type, owner (commerce_meta_writer) and
--   EXECUTE (commerce_runtime only) are unchanged, re-asserted below. Tenant/store stay the server-set GUCs.

CREATE OR REPLACE FUNCTION social.list_conversations(p_filter text, p_session uuid, p_cursor_last_at timestamptz, p_cursor_id uuid, p_limit int)
RETURNS TABLE(conversation_id uuid, platform text, last_at timestamptz, unread boolean, unreplied boolean,
 mode text, assignee uuid, window_open_until timestamptz, linked_customer_id uuid, link_version bigint)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
BEGIN
    IF p_filter IS NULL OR p_filter NOT IN ('all','unread','unreplied','messenger','instagram','live_comment') THEN
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
         OR (p_filter = 'unread' AND COALESCE(s.last_inbound_seq, 0) > s.read_seq)
         OR (p_filter = 'unreplied' AND s.last_inbound_at > COALESCE(s.last_outbound_at, '-infinity'::timestamptz)))
       AND (p_session IS NULL OR claims.session_peer_linked(v_tenant, v_store, p_session,
            c.app_id, c.object, c.asset_id, c.peer_key))
       AND (p_cursor_last_at IS NULL OR p_cursor_id IS NULL OR
            (GREATEST(s.last_inbound_at, COALESCE(s.last_outbound_at, '-infinity'::timestamptz)), c.id) < (p_cursor_last_at, p_cursor_id))
     ORDER BY GREATEST(s.last_inbound_at, COALESCE(s.last_outbound_at, '-infinity'::timestamptz)) DESC, c.id DESC
     LIMIT p_limit;
END $$;

ALTER FUNCTION social.list_conversations(text, uuid, timestamptz, uuid, int) OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION social.list_conversations(text, uuid, timestamptz, uuid, int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION social.list_conversations(text, uuid, timestamptz, uuid, int) TO commerce_runtime;

COMMENT ON FUNCTION social.list_conversations(text, uuid, timestamptz, uuid, int) IS
 'internal/inbox ListConversations (0119, re-created by 0165 LC-B3b, filter unread added by 0204 OC-12): A8 conversation metadata, no ciphertext. Filters all|unread|unreplied|messenger|instagram|live_comment; unread = COALESCE(last_inbound_seq,0) > read_seq (store-level, live-console-v1 §3.6). p_session keeps conversations whose peer is inbox.bundle_peers-linked to a non-purged bundle of that session (claims.session_peer_linked; I09); link_version = inbox.conversation_state.version. inbox:read required; tenant/store from the GUCs; limit 1..50. STABLE SECURITY DEFINER search_path=pg_catalog; EXECUTE to runtime only.';
