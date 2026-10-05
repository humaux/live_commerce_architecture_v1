-- 0122 live-console inbox (contracts/live-console-v1.md §3.1/§3.2/§3.6/§3.7, §11 A8-A11/A13/A14, §14.8; FROZEN 2026-10-02).
--
-- Owns: the inbound read authority (social.read_thread / social.list_conversations / social.conversation_meta /
-- social.unread_conversation_count), the conversation-state table inbox.conversation_state plus its AFTER INSERT
-- trigger on social.messages, the merchant write definers (inbox.mark_read / inbox.takeover / inbox.release /
-- inbox.customer_link / inbox.thread_opened), the claims-worker window read inbox.dm_window, the permission
-- vocabulary inbox:read / inbox:reply / inventory:live_adjust, the role-bundle change (live_operator += three new
-- permissions; viewer excludes inbox:read), and the one-shot resubscribe job integration.meta_resubscribe_jobs with
-- claim/finish definers and the migration backfill.
--
-- Non-goals: no send, no outbound_messages / send_secrets / bundle_peers (0123 = LC-B4), no comment read-through
-- (LC-B2), no capability table write (W1-01B), no template vocabulary (W2-05B / LC-B5), no consumer/classifier
-- change (the reaction-exclusion of §3.6 is a documented boundary owned by LC-B4's --inbox-send gate, LCN06), no
-- dm_window_for_bundle (needs inbox.bundle_peers = 0123).
--
-- Depends on: 0001 (ops.audit_events, identity store_grants/memberships), 0006 (buyer.owners), 0028/0029
-- (meta_inbox.events, social.conversations/messages), 0064/0066/0095/0100 (integration roles, meta_page_heads/
-- credentials, meta_connections, the unsubscribe-job pattern), 0079/0089 (permission CHECK re-derivation,
-- staff_role_permissions, the staff-team backfill).
--
-- Callers: internal/inbox (read side + definer callers), internal/httpapi/inbox.go (A8-A11/A13/A14), cmd/claims-worker
-- via internal/integrations/metareply (Resubscriber) and the send path (dm_window, LC-B4). Roles: commerce_runtime
-- reaches the merchant definers only; commerce_claims_worker reaches dm_window and the resubscribe claim/finish only.

-- ---------------------------------------------------------------------------------------
-- Preconditions and permission vocabulary. Re-derive the CURRENT store_grants_permission_check
-- (0089 is the last writer) instead of rebuilding it, so a later value can never be silently
-- dropped (0063/0079 pattern), then append the three new permissions when absent.
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_def text; v_list text[]; v_perm text;
BEGIN
 IF to_regclass('social.conversations') IS NULL OR to_regclass('social.messages') IS NULL
  OR to_regclass('meta_inbox.events') IS NULL OR to_regclass('integration.meta_connections') IS NULL
  OR to_regclass('integration.meta_page_heads') IS NULL OR to_regclass('integration.meta_page_credentials') IS NULL
  OR to_regclass('identity.store_staff') IS NULL OR to_regclass('buyer.owners') IS NULL THEN
  RAISE EXCEPTION '0122 requires 0006, 0028, 0029, 0089, 0095 (social/meta/connections/staff/owners tables)';
 END IF;
 FOREACH v_perm IN ARRAY ARRAY['commerce_meta_writer','commerce_auth','commerce_runtime','commerce_claims_worker',
  'commerce_staff_writer','commerce_integration_writer'] LOOP
  IF to_regrole(v_perm) IS NULL THEN RAISE EXCEPTION '0122 requires role %',v_perm; END IF;
 END LOOP;
 SELECT pg_get_constraintdef(c.oid) INTO STRICT v_def FROM pg_constraint c
  WHERE c.conrelid='identity.store_grants'::regclass AND c.conname='store_grants_permission_check';
 SELECT array_agg(t.m[1] ORDER BY t.ord) INTO v_list
  FROM regexp_matches(v_def,'''([a-z_]+:[a-z_]+)''::text','g') WITH ORDINALITY AS t(m,ord);
 IF v_list IS NULL OR NOT ('billing:manage'=ANY(v_list) AND 'ads:approve'=ANY(v_list)
   AND 'customers:privacy'=ANY(v_list) AND 'customers:read'=ANY(v_list)) THEN
  RAISE EXCEPTION '0122 applied out of order: store_grants_permission_check lacks 0078/0079/0089 permissions: %',v_def;
 END IF;
 FOREACH v_perm IN ARRAY ARRAY['inbox:read','inbox:reply','inventory:live_adjust'] LOOP
  IF NOT v_perm=ANY(v_list) THEN v_list:=v_list||v_perm; END IF;
 END LOOP;
 ALTER TABLE identity.store_grants DROP CONSTRAINT store_grants_permission_check;
 EXECUTE format('ALTER TABLE identity.store_grants ADD CONSTRAINT store_grants_permission_check CHECK (permission IN (%s))',
  (SELECT string_agg(quote_literal(p),',' ORDER BY ord) FROM unnest(v_list) WITH ORDINALITY AS u(p,ord)));
END $$;

-- ---------------------------------------------------------------------------------------
-- Role bundles (contracts/storefront-v2.md §D + live-console-v1 §14.8 OPEN-7/17). The
-- catalogue now carries inbox:read/inbox:reply/inventory:live_adjust, so owner/admin pick them up
-- automatically; live_operator is the fixed list plus the three; viewer excludes inbox:read
-- explicitly (DM text is buyer PII, not an analytics read; LCN03 asserts it). fulfilment unchanged.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION identity.staff_role_permissions(p_role text)
RETURNS text[] LANGUAGE sql STABLE SET search_path = pg_catalog AS $$
    SELECT CASE p_role
      WHEN 'owner' THEN identity.staff_permission_catalogue()
      WHEN 'admin' THEN ARRAY(SELECT x FROM unnest(identity.staff_permission_catalogue()) x WHERE x <> 'billing:manage')
      WHEN 'live_operator' THEN ARRAY['store:read','live:read','live:manage','catalog:read','orders:read','inventory:read',
        'inbox:read','inbox:reply','inventory:live_adjust']
      WHEN 'fulfilment' THEN ARRAY['store:read','orders:read','fulfillment:write','orders:export','inventory:read','inventory:write','inventory:reserve']
      WHEN 'viewer' THEN ARRAY(SELECT x FROM unnest(identity.staff_permission_catalogue()) x WHERE x LIKE '%:read' AND x <> 'inbox:read')
    END
$$;

-- Additive backfill of the three new permissions for existing owner/admin/live_operator staff
-- (the 0089 D1 pattern; nothing is removed; viewer and fulfilment keep their bundles).
INSERT INTO identity.store_grants(tenant_id, store_id, principal_id, permission)
SELECT t.tenant_id, t.store_id, t.principal_id, x FROM identity.store_staff t, unnest(identity.staff_role_permissions(t.role)) x
WHERE t.role IN ('owner','admin','live_operator')
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------------------
-- Role, schema, tables. The NOLOGIN owner of the merchant write definers (the read definers
-- stay owned by commerce_meta_writer, the meta projection owner). conversation_id is a SOFT
-- reference to social.conversations (indexed with tenant_id,store_id; no FK, so C5b can delete
-- conversations first). FORCE RLS: the definer body is the control.
-- ---------------------------------------------------------------------------------------
CREATE ROLE commerce_inbox_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
COMMENT ON ROLE commerce_inbox_writer IS
 '0122 NOLOGIN definer owner of the merchant inbox writes (mark_read/takeover/release/customer_link/thread_opened). Never a login; reads/writes inbox.conversation_state and the thread-open marks only through its fixed functions.';

CREATE SCHEMA inbox;
REVOKE ALL ON SCHEMA inbox FROM PUBLIC;
GRANT USAGE ON SCHEMA inbox TO commerce_meta_writer, commerce_inbox_writer, commerce_auth, commerce_runtime, commerce_claims_worker;
GRANT USAGE ON SCHEMA social TO commerce_runtime;

CREATE TABLE inbox.conversation_state (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    mode text NOT NULL DEFAULT 'auto' CHECK (mode IN ('auto','human')),
    assignee_principal uuid,
    takeover_generation bigint NOT NULL DEFAULT 0 CHECK (takeover_generation >= 0),
    human_until timestamptz,
    last_human_outbound_at timestamptz,
    read_seq bigint NOT NULL DEFAULT 0 CHECK (read_seq >= 0),
    last_inbound_seq bigint CHECK (last_inbound_seq > 0),
    last_inbound_at timestamptz,
    last_outbound_at timestamptz,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','done')),
    customer_id uuid,
    customer_link_principal uuid,
    version bigint NOT NULL DEFAULT 0 CHECK (version >= 0),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, conversation_id)
);
-- Soft reference to social.conversations (indexed with tenant_id, store_id; no FK per §3.6).
CREATE INDEX conversation_state_store ON inbox.conversation_state(tenant_id, store_id);

-- Coalescing of the A9 audit inbox.thread_opened (≤ 1 per principal per conversation per hour;
-- ops.audit_events has no conversation_id column and the action grammar forbids UUID hyphens).
CREATE TABLE inbox.thread_open_marks (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    principal_id uuid NOT NULL,
    last_opened_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, conversation_id, principal_id)
);

ALTER TABLE inbox.conversation_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.conversation_state FORCE ROW LEVEL SECURITY;
ALTER TABLE inbox.thread_open_marks ENABLE ROW LEVEL SECURITY;
ALTER TABLE inbox.thread_open_marks FORCE ROW LEVEL SECURITY;
REVOKE ALL ON inbox.conversation_state, inbox.thread_open_marks FROM PUBLIC;

-- commerce_meta_writer: the AFTER INSERT trigger (create state row + advance last_inbound_*) and the
-- read definers / dm_window (SELECT). The trigger body is the control for the wide policy.
GRANT SELECT, INSERT ON inbox.conversation_state TO commerce_meta_writer;
GRANT UPDATE (last_inbound_seq, last_inbound_at, updated_at) ON inbox.conversation_state TO commerce_meta_writer;
CREATE POLICY conversation_state_meta ON inbox.conversation_state FOR ALL TO commerce_meta_writer USING (true) WITH CHECK (true);

-- commerce_inbox_writer: the merchant definers (mark_read/takeover/release/customer_link/thread_opened)
-- SELECT FOR UPDATE and UPDATE the state row; the definer body is the control.
GRANT SELECT ON inbox.conversation_state TO commerce_inbox_writer;
GRANT UPDATE (mode, assignee_principal, takeover_generation, human_until, read_seq, status, customer_id, customer_link_principal, version, updated_at)
 ON inbox.conversation_state TO commerce_inbox_writer;
CREATE POLICY conversation_state_inbox ON inbox.conversation_state FOR ALL TO commerce_inbox_writer USING (true) WITH CHECK (true);

GRANT SELECT, INSERT ON inbox.thread_open_marks TO commerce_inbox_writer;
GRANT UPDATE (last_opened_at) ON inbox.thread_open_marks TO commerce_inbox_writer;
CREATE POLICY thread_open_marks_inbox ON inbox.thread_open_marks FOR ALL TO commerce_inbox_writer USING (true) WITH CHECK (true);

GRANT INSERT ON ops.audit_events TO commerce_inbox_writer;
CREATE POLICY audit_inbox_insert ON ops.audit_events FOR INSERT TO commerce_inbox_writer
 WITH CHECK (action IN ('inbox.thread_opened','inbox.takeover','inbox.release','inbox.customer_linked','inbox.customer_unlinked'));

-- A14 customer validation: only the three scope/id columns of buyer.owners are visible to the inbox
-- definer, and only the current server-resolved tenant/store (the policy, not the caller, decides
-- visibility). A foreign or missing customer is therefore indistinguishable -> PT404 not_found.
GRANT USAGE ON SCHEMA buyer TO commerce_inbox_writer;
GRANT SELECT(tenant_id, store_id, id) ON buyer.owners TO commerce_inbox_writer;
CREATE POLICY owners_inbox_lookup ON buyer.owners FOR SELECT TO commerce_inbox_writer
 USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
    AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);

-- ---------------------------------------------------------------------------------------
-- Principal-holds guard (owner commerce_auth, which has SELECT on identity.store_grants and
-- identity.memberships per 0001). The tenant/store/principal are the server-set GUCs, never a
-- request value. Called by the read and write definers as defence in depth.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.principal_holds(p_permissions text[]) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT EXISTS(
      SELECT 1 FROM identity.store_grants g
      JOIN identity.memberships m ON m.tenant_id = g.tenant_id AND m.principal_id = g.principal_id AND m.active
      WHERE g.tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
        AND g.store_id = nullif(current_setting('app.store_id', true), '')::uuid
        AND g.principal_id = nullif(current_setting('app.principal_id', true), '')::uuid
        AND g.permission = ANY(p_permissions))
$$;
ALTER FUNCTION inbox.principal_holds(text[]) OWNER TO commerce_auth;
REVOKE ALL ON FUNCTION inbox.principal_holds(text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.principal_holds(text[]) TO commerce_meta_writer, commerce_inbox_writer;

-- ---------------------------------------------------------------------------------------
-- §3.6 window maintenance. The AFTER INSERT trigger on social.messages creates the state row on
-- the first message and advances last_inbound_*. All stored social.messages rows are direction='in'
-- and is_echo=false by construction (echoes, read receipts and delivery events are quarantined by
-- the frozen classifier upstream, never projected). An occurred_at more than 5 min ahead of server
-- time is clamped to server time. Boundary (documented in DELIVERY.md): reactions reach
-- social.messages indistinguishable from user messages at the SQL layer (no is_echo/kind marker is
-- persisted and no consumer/classifier change is in LC-B3's write paths); reaction exclusion is
-- owned by LC-B4's --inbox-send gate (LCN06).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.advance_conversation_state() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_at timestamptz;
BEGIN
    INSERT INTO inbox.conversation_state(tenant_id, store_id, conversation_id)
    VALUES (NEW.tenant_id, NEW.store_id, NEW.conversation_id)
    ON CONFLICT (tenant_id, store_id, conversation_id) DO NOTHING;
    v_at := COALESCE(NEW.occurred_at, NEW.received_at);
    IF v_at > clock_timestamp() + interval '5 minutes' THEN
        v_at := clock_timestamp();
    END IF;
    UPDATE inbox.conversation_state s
       SET last_inbound_seq = GREATEST(COALESCE(s.last_inbound_seq, 0), NEW.server_seq),
           last_inbound_at = GREATEST(COALESCE(s.last_inbound_at, '-infinity'::timestamptz), v_at),
           updated_at = clock_timestamp()
     WHERE s.tenant_id = NEW.tenant_id AND s.store_id = NEW.store_id AND s.conversation_id = NEW.conversation_id;
    RETURN NULL;
END $$;
ALTER FUNCTION inbox.advance_conversation_state() OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION inbox.advance_conversation_state() FROM PUBLIC;
CREATE TRIGGER conversation_state_advance AFTER INSERT ON social.messages
 FOR EACH ROW EXECUTE FUNCTION inbox.advance_conversation_state();

-- ---------------------------------------------------------------------------------------
-- §3.6 window/takeover read for the claims-worker Check path. tenant/store come from the frozen
-- request (explicit params), never GUCs. Lazy takeover expiry is applied on read: an expired human
-- thread reads as auto with generation+1 even before it is written back.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.dm_window(p_tenant uuid, p_store uuid, p_conversation uuid)
RETURNS TABLE(last_inbound_at timestamptz, mode text, takeover_generation bigint, human_until timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
    SELECT s.last_inbound_at,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN 'auto' ELSE s.mode END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN s.takeover_generation + 1 ELSE s.takeover_generation END,
           CASE WHEN s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until
                THEN NULL ELSE s.human_until END
      FROM inbox.conversation_state s
     WHERE s.tenant_id = p_tenant AND s.store_id = p_store AND s.conversation_id = p_conversation
$$;
ALTER FUNCTION inbox.dm_window(uuid, uuid, uuid) OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION inbox.dm_window(uuid, uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.dm_window(uuid, uuid, uuid) TO commerce_claims_worker;

-- ---------------------------------------------------------------------------------------
-- §3.2 read authority. Owner commerce_meta_writer, EXECUTE commerce_runtime, requires an M merchant
-- transaction (GUCs) and principal_holds(['inbox:read']). read_thread returns the envelope plus the
-- immutable AAD context of its source event (direction is the literal 'in'); list_conversations and
-- conversation_meta return metadata only (no ciphertext). Lazy expiry is applied consistently.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION social.read_thread(p_conversation uuid, p_before_seq bigint, p_limit int)
RETURNS TABLE(event_id uuid, key_id text, nonce bytea, ciphertext bytea, app_id text, object text,
 asset_id text, event_key text, payload_hash text, tenant_id uuid, store_id uuid, route_id uuid,
 route_epoch bigint, server_seq bigint, occurred_at timestamptz, direction text)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
BEGIN
    IF p_limit IS NULL OR p_limit < 1 OR p_limit > 50 OR (p_before_seq IS NOT NULL AND p_before_seq < 1) THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    IF NOT EXISTS(SELECT 1 FROM social.conversations c
        WHERE c.id = p_conversation AND c.tenant_id = v_tenant AND c.store_id = v_store) THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    RETURN QUERY
    SELECT m.event_id, m.key_id, m.nonce, m.ciphertext,
           e.app_id, e.object, e.asset_id, e.event_key, e.payload_hash,
           m.tenant_id, m.store_id, e.route_id, e.route_epoch,
           m.server_seq, COALESCE(m.occurred_at, m.received_at), 'in'::text
      FROM social.messages m
      JOIN meta_inbox.events e ON e.id = m.event_id AND e.tenant_id = m.tenant_id AND e.store_id = m.store_id
      JOIN social.conversations c ON c.id = m.conversation_id AND c.tenant_id = m.tenant_id AND c.store_id = m.store_id
     WHERE m.conversation_id = p_conversation AND m.tenant_id = v_tenant AND m.store_id = v_store
       AND (p_before_seq IS NULL OR m.server_seq < p_before_seq)
     ORDER BY m.server_seq DESC
     LIMIT p_limit;
END $$;

CREATE FUNCTION social.list_conversations(p_filter text, p_cursor_last_at timestamptz, p_cursor_id uuid, p_limit int)
RETURNS TABLE(conversation_id uuid, platform text, last_at timestamptz, unread boolean, unreplied boolean,
 mode text, assignee uuid, window_open_until timestamptz, linked_customer_id uuid)
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
           s.customer_id
      FROM social.conversations c
      JOIN inbox.conversation_state s ON s.tenant_id = c.tenant_id AND s.store_id = c.store_id AND s.conversation_id = c.id
     WHERE c.tenant_id = v_tenant AND c.store_id = v_store
       AND (p_filter = 'all'
         OR (p_filter = 'messenger' AND c.object = 'page')
         OR (p_filter = 'instagram' AND c.object = 'instagram')
         OR (p_filter = 'unreplied' AND s.last_inbound_at > COALESCE(s.last_outbound_at, '-infinity'::timestamptz)))
       AND (p_cursor_last_at IS NULL OR p_cursor_id IS NULL OR
            (GREATEST(s.last_inbound_at, COALESCE(s.last_outbound_at, '-infinity'::timestamptz)), c.id) < (p_cursor_last_at, p_cursor_id))
     ORDER BY GREATEST(s.last_inbound_at, COALESCE(s.last_outbound_at, '-infinity'::timestamptz)) DESC, c.id DESC
     LIMIT p_limit;
END $$;

-- A9 header + A13 conversation-scoped fields. No ciphertext.
CREATE FUNCTION social.conversation_meta(p_conversation uuid)
RETURNS TABLE(platform text, mode text, assignee uuid, takeover_generation bigint, human_until timestamptz,
 window_open_until timestamptz, last_inbound_at timestamptz, last_inbound_seq bigint, read_seq bigint,
 linked_customer_id uuid, status text)
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
           s.customer_id, s.status
      FROM social.conversations c
      JOIN inbox.conversation_state s ON s.tenant_id = c.tenant_id AND s.store_id = c.store_id AND s.conversation_id = c.id
     WHERE c.id = p_conversation AND c.tenant_id = v_tenant AND c.store_id = v_store;
END $$;

CREATE FUNCTION social.unread_conversation_count() RETURNS bigint
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
BEGIN
    IF NOT inbox.principal_holds(ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    RETURN (SELECT count(*) FROM inbox.conversation_state s
        WHERE s.tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
          AND s.store_id = nullif(current_setting('app.store_id', true), '')::uuid
          AND COALESCE(s.last_inbound_seq, 0) > s.read_seq);
END $$;

DO $$
DECLARE f text;
BEGIN
    FOREACH f IN ARRAY ARRAY['social.read_thread(uuid,bigint,int)','social.list_conversations(text,timestamptz,uuid,int)',
      'social.conversation_meta(uuid)','social.unread_conversation_count()'] LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_meta_writer', f);
        EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC', f);
        EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime', f);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------
-- §3.6/§3.7 merchant write definers. Owner commerce_inbox_writer, EXECUTE commerce_runtime.
-- A10 read: honoured only for inbox:reply holders (a read-only role leaves the assignee's unread
-- mark unchanged and gets the unchanged value back). A11 takeover/release and A14 customer-link are
-- CAS-guarded (takeover_changed / version_conflict, PT409).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION inbox.mark_read(p_conversation uuid, p_read_seq bigint) RETURNS bigint
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE s inbox.conversation_state; v_new bigint;
BEGIN
    IF p_read_seq IS NULL OR p_read_seq < 0 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    SELECT * INTO s FROM inbox.conversation_state x
     WHERE x.conversation_id = p_conversation
       AND x.tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
       AND x.store_id = nullif(current_setting('app.store_id', true), '')::uuid
     FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    v_new := LEAST(GREATEST(s.read_seq, p_read_seq), COALESCE(s.last_inbound_seq, 0));
    IF inbox.principal_holds(ARRAY['inbox:reply']::text[]) AND v_new <> s.read_seq THEN
        UPDATE inbox.conversation_state x SET read_seq = v_new, updated_at = clock_timestamp()
         WHERE x.tenant_id = s.tenant_id AND x.store_id = s.store_id AND x.conversation_id = s.conversation_id;
        RETURN v_new;
    END IF;
    RETURN s.read_seq;
END $$;

CREATE FUNCTION inbox.takeover(p_conversation uuid, p_expected_generation bigint)
RETURNS TABLE(mode text, assignee uuid, takeover_generation bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE s inbox.conversation_state; v_gen bigint; v_principal uuid;
BEGIN
    IF p_expected_generation IS NULL OR p_expected_generation < 0 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['inbox:reply']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    v_principal := nullif(current_setting('app.principal_id', true), '')::uuid;
    SELECT * INTO s FROM inbox.conversation_state x
     WHERE x.conversation_id = p_conversation
       AND x.tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
       AND x.store_id = nullif(current_setting('app.store_id', true), '')::uuid
     FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    -- Lazy expiry first: an expired human thread is auto with generation+1 before the CAS.
    IF s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until THEN
        s.mode := 'auto';
        s.takeover_generation := s.takeover_generation + 1;
        s.human_until := NULL;
    END IF;
    IF s.takeover_generation <> p_expected_generation THEN
        RAISE EXCEPTION 'takeover_changed' USING ERRCODE = 'PT409';
    END IF;
    v_gen := s.takeover_generation + 1;
    UPDATE inbox.conversation_state x SET mode = 'human', assignee_principal = v_principal, human_until = NULL,
        takeover_generation = v_gen, updated_at = clock_timestamp()
     WHERE x.tenant_id = s.tenant_id AND x.store_id = s.store_id AND x.conversation_id = s.conversation_id;
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action)
     VALUES (s.tenant_id, s.store_id, v_principal, 'inbox.takeover');
    RETURN QUERY SELECT 'human', v_principal, v_gen;
END $$;

CREATE FUNCTION inbox.release(p_conversation uuid, p_expected_generation bigint)
RETURNS TABLE(mode text, assignee uuid, takeover_generation bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE s inbox.conversation_state; v_gen bigint; v_principal uuid;
BEGIN
    IF p_expected_generation IS NULL OR p_expected_generation < 0 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['inbox:reply']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    v_principal := nullif(current_setting('app.principal_id', true), '')::uuid;
    SELECT * INTO s FROM inbox.conversation_state x
     WHERE x.conversation_id = p_conversation
       AND x.tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid
       AND x.store_id = nullif(current_setting('app.store_id', true), '')::uuid
     FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    IF s.mode = 'human' AND s.human_until IS NOT NULL AND clock_timestamp() >= s.human_until THEN
        s.mode := 'auto';
        s.takeover_generation := s.takeover_generation + 1;
        s.human_until := NULL;
    END IF;
    IF s.takeover_generation <> p_expected_generation THEN
        RAISE EXCEPTION 'takeover_changed' USING ERRCODE = 'PT409';
    END IF;
    v_gen := s.takeover_generation + 1;
    UPDATE inbox.conversation_state x SET mode = 'auto', assignee_principal = NULL, human_until = NULL,
        takeover_generation = v_gen, updated_at = clock_timestamp()
     WHERE x.tenant_id = s.tenant_id AND x.store_id = s.store_id AND x.conversation_id = s.conversation_id;
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action)
     VALUES (s.tenant_id, s.store_id, v_principal, 'inbox.release');
    RETURN QUERY SELECT 'auto', NULL::uuid, v_gen;
END $$;

CREATE FUNCTION inbox.customer_link(p_conversation uuid, p_customer_id uuid, p_expected_version bigint)
RETURNS TABLE(customer_id uuid, version bigint)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE s inbox.conversation_state; v_tenant uuid; v_store uuid; v_principal uuid;
BEGIN
    IF p_expected_version IS NULL OR p_expected_version < 0 THEN
        RAISE EXCEPTION 'invalid_request' USING ERRCODE = 'PT422';
    END IF;
    IF NOT inbox.principal_holds(ARRAY['inbox:reply','customers:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    v_tenant := nullif(current_setting('app.tenant_id', true), '')::uuid;
    v_store := nullif(current_setting('app.store_id', true), '')::uuid;
    v_principal := nullif(current_setting('app.principal_id', true), '')::uuid;
    SELECT * INTO s FROM inbox.conversation_state x
     WHERE x.conversation_id = p_conversation AND x.tenant_id = v_tenant AND x.store_id = v_store
     FOR UPDATE;
    IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404'; END IF;
    IF s.version <> p_expected_version THEN
        RAISE EXCEPTION 'version_conflict' USING ERRCODE = 'PT409';
    END IF;
    IF p_customer_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM buyer.owners o
        WHERE o.tenant_id = v_tenant AND o.store_id = v_store AND o.id = p_customer_id) THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    UPDATE inbox.conversation_state x SET customer_id = p_customer_id, customer_link_principal = v_principal,
        version = s.version + 1, updated_at = clock_timestamp()
     WHERE x.tenant_id = s.tenant_id AND x.store_id = s.store_id AND x.conversation_id = s.conversation_id;
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action)
     VALUES (s.tenant_id, s.store_id, v_principal,
             CASE WHEN p_customer_id IS NULL THEN 'inbox.customer_unlinked' ELSE 'inbox.customer_linked' END);
    RETURN QUERY SELECT p_customer_id, s.version + 1;
END $$;

-- A9 audit, coalesced ≤ 1 per principal per conversation per hour (no content in the audit row;
-- the conversation id lives only in inbox.thread_open_marks).
CREATE FUNCTION inbox.thread_opened(p_conversation uuid) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v_tenant uuid := nullif(current_setting('app.tenant_id', true), '')::uuid;
        v_store uuid := nullif(current_setting('app.store_id', true), '')::uuid;
        v_principal uuid := nullif(current_setting('app.principal_id', true), '')::uuid;
        m inbox.thread_open_marks;
BEGIN
    IF NOT inbox.principal_holds(ARRAY['inbox:read']::text[]) THEN
        RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403';
    END IF;
    IF NOT EXISTS(SELECT 1 FROM social.conversations c
        WHERE c.id = p_conversation AND c.tenant_id = v_tenant AND c.store_id = v_store) THEN
        RAISE EXCEPTION 'not_found' USING ERRCODE = 'PT404';
    END IF;
    SELECT * INTO m FROM inbox.thread_open_marks x
     WHERE x.tenant_id = v_tenant AND x.store_id = v_store AND x.conversation_id = p_conversation
       AND x.principal_id = v_principal FOR UPDATE;
    IF FOUND AND m.last_opened_at > clock_timestamp() - interval '1 hour' THEN
        RETURN;
    END IF;
    INSERT INTO inbox.thread_open_marks(tenant_id, store_id, conversation_id, principal_id, last_opened_at)
     VALUES (v_tenant, v_store, p_conversation, v_principal, clock_timestamp())
     ON CONFLICT (tenant_id, store_id, conversation_id, principal_id)
     DO UPDATE SET last_opened_at = clock_timestamp();
    INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action)
     VALUES (v_tenant, v_store, v_principal, 'inbox.thread_opened');
END $$;

DO $$
DECLARE f text;
BEGIN
    FOREACH f IN ARRAY ARRAY['inbox.mark_read(uuid,bigint)','inbox.takeover(uuid,bigint)','inbox.release(uuid,bigint)',
      'inbox.customer_link(uuid,uuid,bigint)','inbox.thread_opened(uuid)'] LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_inbox_writer', f);
        EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC', f);
        EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime', f);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------------------
-- §3.1 resubscribe job (integration.meta_resubscribe_jobs). Same lifecycle and audit pattern as
-- meta_unsubscribe_jobs of 0100, minus SUPERSEDED: PENDING -> LEASED -> SUCCEEDED | FAILED |
-- UNKNOWN; a definite not-applied answer (429/503) goes back to PENDING with exponential backoff
-- (at most 5 attempts); every terminal state wipes the sealed token copy and writes one audit row.
-- Enqueued once per active connection by the backfill below (POST /{page}/subscribed_apps with
-- subscribed_fields=feed,messages); the read-back + capability write is W1-01B.
-- ---------------------------------------------------------------------------------------
CREATE TABLE integration.meta_resubscribe_jobs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, principal_id uuid NOT NULL, binding_id uuid NOT NULL,
 page_id text NOT NULL CHECK(page_id ~ '^[0-9]{1,40}$'),
 version bigint NOT NULL CHECK(version>0),
 key_id text CHECK(key_id ~ '^[A-Za-z0-9_-]{1,64}$'),
 nonce bytea CHECK(octet_length(nonce) IN (12,32)),
 ciphertext bytea CHECK(octet_length(ciphertext) BETWEEN 17 AND 8192),
 state text NOT NULL DEFAULT 'PENDING' CHECK(state IN ('PENDING','LEASED','SUCCEEDED','FAILED','UNKNOWN')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 lease_until timestamptz,
 code text CHECK(code ~ '^[a-z0-9_]{1,40}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK((state IN ('PENDING','LEASED')) = (key_id IS NOT NULL AND nonce IS NOT NULL AND ciphertext IS NOT NULL)),
 CHECK((state='LEASED') = (lease_until IS NOT NULL)),
 UNIQUE(tenant_id, store_id, page_id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(tenant_id,principal_id) REFERENCES identity.memberships(tenant_id,principal_id));
CREATE INDEX meta_resubscribe_jobs_open ON integration.meta_resubscribe_jobs(next_attempt_at) WHERE state IN ('PENDING','LEASED');
ALTER TABLE integration.meta_resubscribe_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE integration.meta_resubscribe_jobs FORCE ROW LEVEL SECURITY;
REVOKE ALL ON integration.meta_resubscribe_jobs FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE(state,attempts,next_attempt_at,lease_until,code,key_id,nonce,ciphertext,updated_at)
 ON integration.meta_resubscribe_jobs TO commerce_integration_writer;
CREATE POLICY meta_resubscribe_jobs_writer ON integration.meta_resubscribe_jobs FOR ALL TO commerce_integration_writer USING (true) WITH CHECK (true);
CREATE POLICY meta_resubscribe_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
 WITH CHECK (action IN ('meta.connect.resubscribed','meta.connect.resubscribe_failed','meta.connect.resubscribe_unknown')
  AND EXISTS(SELECT 1 FROM integration.meta_resubscribe_jobs j WHERE j.tenant_id=ops.audit_events.tenant_id AND j.store_id=ops.audit_events.store_id
   AND j.principal_id=ops.audit_events.principal_id AND j.state IN ('SUCCEEDED','FAILED','UNKNOWN') AND j.updated_at>=clock_timestamp()-interval '1 minute'));
COMMENT ON TABLE integration.meta_resubscribe_jobs IS
 '0122 (§3.1): durable best-effort "POST /{page}/subscribed_apps subscribed_fields=feed,messages" of an already-connected Page. Holds a COPY of the head sealed Page token only while PENDING/LEASED; every terminal state wipes it. Written by the 0122 backfill and integration.claim_meta_resubscribe/finish_meta_resubscribe only; opened only by the claims-worker (private ring).';

CREATE FUNCTION integration.finish_meta_resubscribe(p_id uuid, p_outcome text, p_code text) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j integration.meta_resubscribe_jobs; v_state text;
BEGIN
 IF p_id IS NULL OR p_outcome IS NULL OR p_outcome NOT IN ('SUCCEEDED','FAILED','UNKNOWN','RETRY') OR p_code IS NULL OR p_code !~ '^[a-z0-9_]{1,40}$' THEN
  RAISE EXCEPTION 'invalid resubscribe outcome' USING ERRCODE='22023'; END IF;
 SELECT * INTO j FROM integration.meta_resubscribe_jobs x WHERE x.id=p_id AND x.state='LEASED' FOR UPDATE;
 IF NOT FOUND THEN RETURN false; END IF;
 IF p_outcome='RETRY' AND j.attempts<5 THEN
  UPDATE integration.meta_resubscribe_jobs x SET state='PENDING',lease_until=NULL,code=p_code,updated_at=clock_timestamp(),
   next_attempt_at=clock_timestamp()+make_interval(secs=>30*power(2,j.attempts-1)) WHERE x.id=p_id;
  RETURN true;
 END IF;
 v_state:=CASE WHEN p_outcome='RETRY' THEN 'FAILED' ELSE p_outcome END;
 UPDATE integration.meta_resubscribe_jobs x SET state=v_state,lease_until=NULL,key_id=NULL,nonce=NULL,ciphertext=NULL,updated_at=clock_timestamp(),
  code=CASE WHEN p_outcome='RETRY' THEN 'retries_exhausted' ELSE p_code END WHERE x.id=p_id;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(j.tenant_id,j.store_id,j.principal_id,
  CASE v_state WHEN 'SUCCEEDED' THEN 'meta.connect.resubscribed' WHEN 'FAILED' THEN 'meta.connect.resubscribe_failed' ELSE 'meta.connect.resubscribe_unknown' END);
 RETURN true;
END $$;

CREATE FUNCTION integration.claim_meta_resubscribe() RETURNS TABLE(o_job uuid,o_tenant uuid,o_store uuid,o_binding uuid,o_page text,
 o_version bigint,o_key_id text,o_nonce bytea,o_ciphertext bytea,o_attempt integer)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE j integration.meta_resubscribe_jobs; e uuid;
BEGIN
 FOR e IN SELECT x.id FROM integration.meta_resubscribe_jobs x WHERE x.state='LEASED' AND x.lease_until<=clock_timestamp() FOR UPDATE SKIP LOCKED LOOP
  PERFORM integration.finish_meta_resubscribe(e,'UNKNOWN','lease_expired');
 END LOOP;
 SELECT * INTO j FROM integration.meta_resubscribe_jobs x WHERE x.state='PENDING' AND x.next_attempt_at<=clock_timestamp()
  ORDER BY x.next_attempt_at,x.id LIMIT 1 FOR UPDATE SKIP LOCKED;
 IF NOT FOUND THEN RETURN; END IF;
 UPDATE integration.meta_resubscribe_jobs x SET state='LEASED',attempts=x.attempts+1,lease_until=clock_timestamp()+interval '60 seconds',updated_at=clock_timestamp()
  WHERE x.id=j.id;
 RETURN QUERY SELECT j.id,j.tenant_id,j.store_id,j.binding_id,j.page_id,j.version,j.key_id,j.nonce,j.ciphertext,j.attempts+1;
END $$;

DO $$
DECLARE f text;
BEGIN
 FOREACH f IN ARRAY ARRAY['integration.finish_meta_resubscribe(uuid,text,text)','integration.claim_meta_resubscribe()'] LOOP
  EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_integration_writer',f);
  EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC',f);
  EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_claims_worker',f);
 END LOOP;
END $$;

-- Backfill: one job per active connection, carrying the head sealed FB Page token (the only
-- credential the claims-worker can open to subscribe the Page). Idempotent on re-run.
INSERT INTO integration.meta_resubscribe_jobs(tenant_id,store_id,principal_id,binding_id,page_id,version,key_id,nonce,ciphertext)
SELECT c.tenant_id, c.store_id, c.connected_by, k.binding_id, c.page_id, k.version, k.key_id, k.nonce, k.ciphertext
  FROM integration.meta_connections c
  JOIN integration.meta_page_heads h ON h.tenant_id=c.tenant_id AND h.store_id=c.store_id AND h.binding_id=c.fb_binding
  JOIN integration.meta_page_credentials k ON k.tenant_id=h.tenant_id AND k.store_id=h.store_id AND k.binding_id=h.binding_id AND k.version=h.current_version
 WHERE c.status='active'
ON CONFLICT (tenant_id, store_id, page_id) DO NOTHING;
