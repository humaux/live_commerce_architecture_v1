-- Purpose: W3-05B restricted-buyer list: a merchant marks a social actor (Facebook / Instagram commenter) as restricted for ONE store; that actor's later claims are still recorded but get no claim link and no automatic private reply, and the console shows the comment as restricted.
-- Depends on: 0060/0064 (claims.bundles/meta_intake/events, claims.intake_scope), 0127 (claims.apply_actor_erasure), 0128 (inbox.bundle_peers, claim_reply_plannable), 0129 (claims.for_buyer_scope, social.conversations read), 0123 (live.console_marks), 0143 (claim_reply_plannable suspension patch), 0151 (claims.events.reply_kind and its mark policy),
--   roles commerce_claims_writer / commerce_integration_writer / commerce_retention_writer / commerce_runtime.
-- Used by: internal/claims/blocklist.go (merchant definers), internal/httpapi/blocklist.go (routes), internal/claimsintake (planReply, unchanged call shape), tests/foundation/blocklist_test.go.
-- Invariants: the block key is the app-scoped actor_key of ONE store (never cross-store/tenant); the client never sends an actor_key (the definer resolves it from a server-known reference); the internal note is merchant-only and never reaches a buyer message, audit, receipt or log;
--   claim_reply_plannable never raises on this path (a RAISE in the poller is final and would lose the claim); the restricted skip spends no mpr: quota (no operation row is written, so the merchant may still reply once by hand).
-- Status: REAL_PG gate (Meta Graph MOCK; LIVE NOT_RUN).
-- 0154 buyer blocklist (R3 unit W3-05B; docs/delivery/units/w3-05b-blocklist.md "Integrator overrides").
--
-- What changes
--   * claims.blocked_actors (PK tenant, store, actor_key; <= 5000 rows per store; note <= 200 chars, no control characters). FORCE RLS. No login role reads it:
--     commerce_claims_writer (merchant definers), commerce_integration_writer (the claim reply planner, leased intake scope), commerce_retention_writer (erasure).
--   * claims.block_actor / unblock_actor / list_blocked_actors / actor_restricted_for_bundle: merchant definers (owner commerce_claims_writer, EXECUTE commerce_runtime).
--     block_actor accepts exactly one server-known reference ({comment_ref} | {conversation_id} | {bundle_id}); unblock takes the entry id (the actor_key never leaves the database).
--   * integration.claim_actor_restricted + a patch of claim_reply_plannable (the 0143/0151 in-place pattern): a restricted actor is the FIRST audited skip
--     (claim_reply_skipped:restricted). The decision lives in claim_reply_plannable, not plan_claim_reply, because only the plannable skip path can end a claim
--     without an operation and without a RAISE; plan_claim_reply is therefore unchanged (the poller calls it only after plannable returned OK).
--     Race: claim_actor_restricted and block_actor take the same per-actor advisory key (exclusive on both sides). A claim that meets a block in flight WAITS for the key and,
--     once the block commits, sees it (every statement of a volatile plpgsql function takes a fresh READ COMMITTED snapshot); a block that arrives mid-claim waits for the claim
--     transaction (that claim was already answered; the block applies from the next comment). Only when the block holds the key longer than the poller's lock_timeout (1 s)
--     does the claim end with 55P03, which the poller records as a retryable failure (nothing is half-applied) and the retry sees the committed block.
--   * claims.events.reply_kind accepts 'restricted' (set once by the skip); live.console_marks shows such a claim with claim.reason 'restricted'.
--   * claims.apply_actor_erasure deletes the actor's blocklist entry and COUNTS it (counts key blocked_actors), so an erasure of an actor whose only remaining record is the entry
--     succeeds (claims.erase_actor treats a zero total as 'not found'); age-based retention does not purge entries (a merchant safety list; unblock or erasure removes them).
--   * Checkout reminders (0144 inbox.checkout_reminder_candidates, patched in place): a restricted buyer's bundle gets the audited follow-up reason 'restricted' (no link re-issue,
--     no DM), through the definer predicate claims.bundle_actor_restricted (the actor_key stays in the database).
-- Limits (contract Amendment W3-05B): links issued before the block stay valid (storefront checkout is anonymous and cannot identify the actor); a restricted actor's
-- later line on an existing bundle never had an automatic reply and shows no mark; FB and IG identities of one person are different actors (BL-OPEN-1).

-- ---------------------------------------------------------------------------------------
-- Table.
-- ---------------------------------------------------------------------------------------
CREATE TABLE claims.blocked_actors (
    tenant_id uuid NOT NULL,
    store_id uuid NOT NULL,
    actor_key text NOT NULL CHECK (actor_key ~ '^[0-9a-f]{64}$'),
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    platform text NOT NULL CHECK (platform IN ('facebook', 'instagram')),
    note text CHECK (note IS NULL OR (char_length(note) BETWEEN 1 AND 200 AND note !~ '[[:cntrl:]]')),
    source_bundle_id uuid,
    principal_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (tenant_id, store_id, actor_key),
    UNIQUE (tenant_id, store_id, id)
);
CREATE INDEX blocked_actors_recent ON claims.blocked_actors(tenant_id, store_id, created_at DESC, id DESC);
ALTER TABLE claims.blocked_actors ENABLE ROW LEVEL SECURITY;
ALTER TABLE claims.blocked_actors FORCE ROW LEVEL SECURITY;
REVOKE ALL ON claims.blocked_actors FROM PUBLIC;
COMMENT ON TABLE claims.blocked_actors IS
 'W3-05B (0154). Per-store restricted social actors. actor_key is the keyed meta-claim-actor/v1 hash (no name, no PSID); note is merchant-only. Readable only through the claims definers and the claim reply planner; erased by claims.apply_actor_erasure.';

GRANT SELECT, INSERT, DELETE ON claims.blocked_actors TO commerce_claims_writer;
CREATE POLICY blocked_writer_rw ON claims.blocked_actors FOR ALL TO commerce_claims_writer
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid)
    WITH CHECK (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid AND store_id = nullif(current_setting('app.store_id', true), '')::uuid);

-- The planner reads only the key columns, only inside the leased intake's store.
GRANT SELECT(tenant_id, store_id, actor_key) ON claims.blocked_actors TO commerce_integration_writer;
CREATE POLICY blocked_reply_read ON claims.blocked_actors FOR SELECT TO commerce_integration_writer
    USING ((tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s));

-- Erasure (claims.apply_actor_erasure, owner commerce_retention_writer, global like its other policies).
GRANT SELECT(actor_key), DELETE ON claims.blocked_actors TO commerce_retention_writer;
CREATE POLICY blocked_retention_read ON claims.blocked_actors FOR SELECT TO commerce_retention_writer USING (true);
CREATE POLICY blocked_retention_delete ON claims.blocked_actors FOR DELETE TO commerce_retention_writer USING (true);

-- Column reads the new code needs beyond the earlier grants (policies of 0060/0064/0113 scope them).
GRANT SELECT(platform, actor_key) ON claims.bundles TO commerce_claims_writer;       -- block_actor / actor_restricted_for_bundle resolve the actor of a bundle
GRANT SELECT(reply_kind) ON claims.events TO commerce_claims_writer;                 -- live.console_marks shows the restricted mark
GRANT SELECT(actor_key) ON claims.meta_intake TO commerce_integration_writer;        -- the leased intake's actor (intake_reply_read policy: leased row only)

-- The skip audit of the planner (written inside the intake transaction).
CREATE POLICY claim_reply_restricted_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK ((tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s)
        AND action = 'claim_reply_skipped:restricted');

-- ---------------------------------------------------------------------------------------
-- claims.events.reply_kind: closed vocabulary {sold_out, restricted}; the one-way mark policy covers both.
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE c text;
BEGIN
    SELECT conname INTO c FROM pg_constraint
     WHERE conrelid = 'claims.events'::regclass AND contype = 'c' AND pg_get_constraintdef(oid) LIKE '%reply_kind%';
    IF c IS NULL THEN RAISE EXCEPTION 'claims.events reply_kind CHECK not found (0151 missing?)'; END IF;
    EXECUTE format('ALTER TABLE claims.events DROP CONSTRAINT %I', c);
END $$;
ALTER TABLE claims.events ADD CONSTRAINT events_reply_kind_check
    CHECK (reply_kind IS NULL OR (reply_kind IN ('sold_out', 'restricted') AND outcome = 'ACCEPTED'));
DROP POLICY event_sold_out_mark ON claims.events;
CREATE POLICY event_sold_out_mark ON claims.events FOR UPDATE TO commerce_integration_writer
    USING ((tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s) AND reply_kind IS NULL)
    WITH CHECK (reply_kind IN ('sold_out', 'restricted'));

-- ---------------------------------------------------------------------------------------
-- Checkout reminders (0144). Predicate claims.bundle_actor_restricted(bundle): owner commerce_claims_writer, EXECUTE commerce_integration_writer only (the owner of
-- inbox.checkout_reminder_candidates). No permission check of its own: its only caller already requires inbox:reply AND live:manage; the tenant/store/principal GUCs pin the scope.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION claims.bundle_actor_restricted(p_bundle uuid) RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; b record;
BEGIN
    v := claims.for_buyer_scope();
    SELECT x.platform, x.actor_key INTO b FROM claims.bundles x
     WHERE x.tenant_id = v[1] AND x.store_id = v[2] AND x.id = p_bundle AND x.purged_at IS NULL;
    IF NOT FOUND OR b.platform NOT IN ('facebook', 'instagram') THEN RETURN false; END IF;
    RETURN EXISTS (SELECT 1 FROM claims.blocked_actors d WHERE d.tenant_id = v[1] AND d.store_id = v[2] AND d.actor_key = b.actor_key);
END $$;
ALTER FUNCTION claims.bundle_actor_restricted(uuid) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.bundle_actor_restricted(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.bundle_actor_restricted(uuid) TO commerce_integration_writer;
COMMENT ON FUNCTION claims.bundle_actor_restricted(uuid) IS
 'W3-05B (0154). internal/claims helper of inbox.checkout_reminder_candidates (caller commerce_integration_writer inside the merchant transaction): whether the bundle''s actor is restricted; boolean only, no permission check of its own (the caller holds inbox:reply and live:manage).';

-- The reminder's follow-up reason set gains 'restricted' (the CHECK is re-declared; existing rows are unaffected).
DO $$
DECLARE c text;
BEGIN
    SELECT conname INTO c FROM pg_constraint
     WHERE conrelid = 'inbox.checkout_reminders'::regclass AND contype = 'c' AND pg_get_constraintdef(oid) LIKE '%link_unavailable%';
    IF c IS NULL THEN RAISE EXCEPTION 'inbox.checkout_reminders reason CHECK not found (0144 missing?)'; END IF;
    EXECUTE format('ALTER TABLE inbox.checkout_reminders DROP CONSTRAINT %I', c);
END $$;
ALTER TABLE inbox.checkout_reminders ADD CONSTRAINT checkout_reminders_reason_check
    CHECK (reason IN ('window_closed', 'human_takeover', 'no_peer', 'capability', 'link_unavailable', 'restricted'));
CREATE POLICY crm_restricted_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
    WITH CHECK (inbox.lcn_in_scope(tenant_id, store_id) AND action = 'inbox.checkout_reminder.skipped:restricted');

-- inbox.checkout_reminder_candidates, patched in place from the live definition (exactly-once anchor, loud failure otherwise; ownership and ACL re-asserted): a restricted
-- buyer overrides every other verdict (including 'send' and the other follow-up reasons), is recorded as the follow-up reason 'restricted' like the other unreachable
-- buyers, and one audit row names the skip. A buyer already reminded stays 'already_reminded' (decided before the anchor).
DO $$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
    v_needle := '        IF v_verdict IS NULL THEN' || chr(10) || '            v_sends := v_sends + 1;';
    v_repl := '        -- 0154 W3-05B: a restricted buyer is never sent a claim link or DM, whatever the verdict above says.' || chr(10)
           || '        IF claims.bundle_actor_restricted(b.id) THEN' || chr(10)
           || '            v_verdict := ''restricted'';' || chr(10)
           || '            INSERT INTO ops.audit_events(tenant_id, store_id, principal_id, action) VALUES (v_t, v_s, v_p, ''inbox.checkout_reminder.skipped:restricted'');' || chr(10)
           || '        END IF;' || chr(10) || v_needle;
    v_fn := pg_get_functiondef('inbox.checkout_reminder_candidates(uuid,text,int,uuid)'::regprocedure);
    IF (length(v_fn) - length(replace(v_fn, v_needle, ''))) <> length(v_needle) THEN
        RAISE EXCEPTION 'inbox.checkout_reminder_candidates has an unexpected shape for the restricted-actor patch';
    END IF;
    EXECUTE replace(v_fn, v_needle, v_repl);
END $$;
ALTER FUNCTION inbox.checkout_reminder_candidates(uuid,text,int,uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION inbox.checkout_reminder_candidates(uuid,text,int,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.checkout_reminder_candidates(uuid,text,int,uuid) TO commerce_runtime;

-- ---------------------------------------------------------------------------------------
-- Planner side. claim_actor_restricted: owner commerce_integration_writer, no EXECUTE grant (only claim_reply_plannable of the same owner calls it).
-- It serialises on the per-actor key block_actor also takes, then answers from a fresh statement snapshot.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.claim_actor_restricted(p_intake uuid) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE i record;
BEGIN
    SELECT x.tenant_id, x.store_id, x.actor_key INTO i FROM claims.meta_intake x
     WHERE x.id = p_intake AND x.lease_xid = pg_current_xact_id() AND x.state = 'PENDING';
    IF NOT FOUND THEN RETURN false; END IF;
    -- Same key text as claims.block_actor; a block that holds it commits first or waits for this claim transaction.
    PERFORM pg_advisory_xact_lock(hashtextextended('claims-blockactor|' || i.tenant_id::text || '|' || i.store_id::text || '|' || i.actor_key, 0));
    RETURN EXISTS (SELECT 1 FROM claims.blocked_actors b
                    WHERE b.tenant_id = i.tenant_id AND b.store_id = i.store_id AND b.actor_key = i.actor_key);
END $$;
ALTER FUNCTION integration.claim_actor_restricted(uuid) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.claim_actor_restricted(uuid) FROM PUBLIC;
COMMENT ON FUNCTION integration.claim_actor_restricted(uuid) IS
 'W3-05B (0154). Private helper of claim_reply_plannable (owner-only, no EXECUTE grant): true when the leased intake''s actor is on its store''s blocklist; takes the per-actor advisory key shared with claims.block_actor. Never raises.';

-- claim_reply_plannable, patched in place from the live definition (loud failure when the shape is not the expected one; ownership re-asserted):
--   (1) restricted is the FIRST branch (before suspension/source/binding skips): the actor gets nothing automatic whatever else is true;
--   (2) a restricted skip marks the bundle-creating claim event reply_kind='restricted' before the audit insert. No link_pending_manual flag:
--       the merchant restricted this buyer on purpose, so no "send the link by hand" prompt.
DO $$
DECLARE v_fn text; v_n1 text; v_r1 text; v_n2 text; v_r2 text;
BEGIN
    v_n1 := ' IF NOT control.store_serving(i.tenant_id,i.store_id) THEN v_code:=''store_suspended'';';
    v_r1 := ' IF integration.claim_actor_restricted(p_intake) THEN v_code:=''restricted'';' || chr(10)
         || ' ELSIF NOT control.store_serving(i.tenant_id,i.store_id) THEN v_code:=''store_suspended'';';
    v_n2 := ' INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)' || chr(10)
         || '  VALUES(i.tenant_id,i.store_id,s.principal_id,''claim_reply_skipped:''||v_code);';
    v_r2 := ' IF v_code=''restricted'' THEN' || chr(10)
         || '  UPDATE claims.events x SET reply_kind=''restricted'' WHERE x.tenant_id=i.tenant_id AND x.store_id=i.store_id' || chr(10)
         || '   AND x.source_event_id=i.inbox_event_id AND x.outcome=''ACCEPTED'' AND x.bundle_version=1 AND x.reply_kind IS NULL;' || chr(10)
         || ' END IF;' || chr(10) || v_n2;
    v_fn := pg_get_functiondef('integration.claim_reply_plannable(uuid)'::regprocedure);
    IF (length(v_fn) - length(replace(v_fn, v_n1, ''))) <> length(v_n1) OR (length(v_fn) - length(replace(v_fn, v_n2, ''))) <> length(v_n2) THEN
        RAISE EXCEPTION 'integration.claim_reply_plannable has an unexpected shape for the restricted-actor patch';
    END IF;
    EXECUTE replace(replace(v_fn, v_n1, v_r1), v_n2, v_r2);
END $$;
ALTER FUNCTION integration.claim_reply_plannable(uuid) OWNER TO commerce_integration_writer;

-- ---------------------------------------------------------------------------------------
-- Console: claim.reason of a restricted claim reads 'restricted' (the claim itself stays ACCEPTED). Same output shape as 0123/0151.
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
    v_needle := 'SELECT e.outcome,e.reason,e.offer_id,e.quantity,e.bundle_id INTO v_claim';
    v_repl := 'SELECT e.outcome,CASE WHEN e.reply_kind=''restricted'' THEN ''restricted'' ELSE e.reason END AS reason,e.offer_id,e.quantity,e.bundle_id INTO v_claim';
    v_fn := pg_get_functiondef('live.console_marks(uuid,text[])'::regprocedure);
    IF (length(v_fn) - length(replace(v_fn, v_needle, ''))) <> length(v_needle) THEN
        RAISE EXCEPTION 'live.console_marks has an unexpected shape for the restricted patch';
    END IF;
    EXECUTE replace(v_fn, v_needle, v_repl);
END $$;
ALTER FUNCTION live.console_marks(uuid,text[]) OWNER TO commerce_claims_writer;

-- ---------------------------------------------------------------------------------------
-- Erasure: the actor's blocklist entry goes with the actor (claims.apply_actor_erasure, patched in place; the 0127 definition is the latest).
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_fn text; v_needle text; v_repl text; v_n2 text; v_r2 text;
BEGIN
    v_needle := ' -- RD1: random key/label (never derived from the bundle id), binding cleared, purged_at set; label only for manual.';
    v_repl := ' IF NOT v_single THEN' || chr(10)
           || '  -- 0154 W3-05B: the merchant''s restricted-buyer entry (and its internal note) names this actor_key; erasure of the person removes it and COUNTS it,' || chr(10)
           || '  -- so an actor whose only remaining record is the entry (the state after age retention) is erased, not reported as not found.' || chr(10)
           || '  DELETE FROM claims.blocked_actors d WHERE d.actor_key=p_actor_key;' || chr(10)
           || '  GET DIAGNOSTICS v_n=ROW_COUNT;' || chr(10)
           || ' ELSE' || chr(10) || '  v_n:=0;' || chr(10)
           || ' END IF;' || chr(10) || v_needle;
    v_n2 := 'CASE WHEN n_defer>0 THEN jsonb_build_object(''social_deferred'',n_defer) ELSE ''{}''::jsonb END;';
    v_r2 := 'CASE WHEN n_defer>0 THEN jsonb_build_object(''social_deferred'',n_defer) ELSE ''{}''::jsonb END'
         || ' || CASE WHEN v_n>0 THEN jsonb_build_object(''blocked_actors'',v_n) ELSE ''{}''::jsonb END;';
    v_fn := pg_get_functiondef('claims.apply_actor_erasure(text,text,uuid,uuid,uuid,text[])'::regprocedure);
    IF (length(v_fn) - length(replace(v_fn, v_needle, ''))) <> length(v_needle) OR (length(v_fn) - length(replace(v_fn, v_n2, ''))) <> length(v_n2) THEN
        RAISE EXCEPTION 'claims.apply_actor_erasure has an unexpected shape for the blocklist patch';
    END IF;
    EXECUTE replace(replace(v_fn, v_needle, v_repl), v_n2, v_r2);
END $$;
ALTER FUNCTION claims.apply_actor_erasure(text,text,uuid,uuid,uuid,text[]) OWNER TO commerce_retention_writer;

-- ---------------------------------------------------------------------------------------
-- Merchant definers (owner commerce_claims_writer; claims.for_buyer_scope pins tenant/store/principal GUCs, read committed, no buyer GUC).
-- ---------------------------------------------------------------------------------------
-- block_actor: exactly one of {comment_ref}|{conversation_id}|{bundle_id}, resolved by the definer inside the caller's store (live:manage).
-- Returns the entry (created=false when the actor was already listed: idempotent, the note of the first block stays). Raises PT409 limit_reached at 5000 rows,
-- PT409 ambiguous_actor when a conversation maps to several actors, PT404 for an unknown / foreign / actor-less reference, 22023 for malformed input.
CREATE FUNCTION claims.block_actor(p_session uuid, p_ref jsonb, p_note text)
RETURNS TABLE(id uuid, platform text, created_at timestamptz, created boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; k text; ref text; c record; v_key text; v_platform text; v_bundle uuid; v_n bigint; r record;
BEGIN
    v := claims.for_buyer_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:manage']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF p_session IS NULL OR p_ref IS NULL OR jsonb_typeof(p_ref) <> 'object' OR (SELECT count(*) FROM jsonb_object_keys(p_ref)) <> 1
       OR (p_note IS NOT NULL AND (char_length(p_note) NOT BETWEEN 1 AND 200 OR p_note ~ '[[:cntrl:]]')) THEN
        RAISE EXCEPTION 'invalid block request' USING ERRCODE = '22023';
    END IF;
    SELECT x INTO k FROM jsonb_object_keys(p_ref) x;
    IF k NOT IN ('comment_ref', 'conversation_id', 'bundle_id') OR jsonb_typeof(p_ref -> k) <> 'string' THEN
        RAISE EXCEPTION 'invalid block request' USING ERRCODE = '22023';
    END IF;
    ref := p_ref ->> k;
    PERFORM 1 FROM live.sessions s WHERE s.tenant_id = v[1] AND s.store_id = v[2] AND s.id = p_session;
    IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;

    IF k = 'comment_ref' THEN
        IF ref !~ '^[0-9_]{1,80}$' THEN RAISE EXCEPTION 'invalid block request' USING ERRCODE = '22023'; END IF;
        -- The intake row of that comment in THIS session of THIS store (any comment, claim or not, identifies its author).
        SELECT i.platform, i.actor_key, i.applied_event_id INTO c FROM claims.meta_intake i
         WHERE i.tenant_id = v[1] AND i.store_id = v[2] AND i.session_id = p_session AND i.comment_ref = ref
         ORDER BY i.created_at LIMIT 1;
        IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
        v_platform := c.platform; v_key := c.actor_key;
        SELECT e.bundle_id INTO v_bundle FROM claims.events e WHERE e.tenant_id = v[1] AND e.store_id = v[2] AND e.id = c.applied_event_id;
    ELSIF k = 'bundle_id' THEN
        IF ref !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN RAISE EXCEPTION 'invalid block request' USING ERRCODE = '22023'; END IF;
        SELECT b.platform, b.actor_key INTO c FROM claims.bundles b
         WHERE b.tenant_id = v[1] AND b.store_id = v[2] AND b.id = ref::uuid AND b.purged_at IS NULL;
        IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
        IF c.platform NOT IN ('facebook', 'instagram') THEN RAISE EXCEPTION 'invalid block request' USING ERRCODE = '22023'; END IF; -- a manual bundle has no social actor
        v_platform := c.platform; v_key := c.actor_key; v_bundle := ref::uuid;
    ELSE
        IF ref !~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN RAISE EXCEPTION 'invalid block request' USING ERRCODE = '22023'; END IF;
        SELECT x.peer_key, x.app_id, x.object, x.asset_id INTO c FROM social.conversations x
         WHERE x.id = ref::uuid AND x.tenant_id = v[1] AND x.store_id = v[2];
        IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
        -- live-console-v1 §3.7: the actors of the bundles whose recorded peer is this thread's peer.
        SELECT count(DISTINCT b.actor_key), min(b.actor_key), min(b.platform), (array_agg(b.id ORDER BY b.id))[1] INTO v_n, v_key, v_platform, v_bundle
          FROM inbox.bundle_peers p
          JOIN claims.bundles b ON b.tenant_id = p.tenant_id AND b.store_id = p.store_id AND b.id = p.bundle_id
                               AND b.purged_at IS NULL AND b.platform IN ('facebook', 'instagram')
         WHERE p.tenant_id = v[1] AND p.store_id = v[2] AND p.peer_key = c.peer_key AND p.app_id = c.app_id AND p.object = c.object AND p.asset_id = c.asset_id;
        IF v_n = 0 THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
        IF v_n > 1 THEN RAISE EXCEPTION 'ambiguous_actor' USING ERRCODE = 'PT409'; END IF;
    END IF;

    -- Lock order: store key (cap) -> actor key (shared with claim_actor_restricted, which takes the actor key only).
    PERFORM pg_advisory_xact_lock(hashtextextended('claims-blocklist|' || v[1]::text || '|' || v[2]::text, 0));
    PERFORM pg_advisory_xact_lock(hashtextextended('claims-blockactor|' || v[1]::text || '|' || v[2]::text || '|' || v_key, 0));
    SELECT d.id, d.platform, d.created_at INTO r FROM claims.blocked_actors d WHERE d.tenant_id = v[1] AND d.store_id = v[2] AND d.actor_key = v_key;
    IF FOUND THEN
        RETURN QUERY SELECT r.id, r.platform, r.created_at, false;
        RETURN;
    END IF;
    SELECT count(*) INTO v_n FROM claims.blocked_actors d WHERE d.tenant_id = v[1] AND d.store_id = v[2];
    IF v_n >= 5000 THEN RAISE EXCEPTION 'limit_reached' USING ERRCODE = 'PT409'; END IF; -- I23
    INSERT INTO claims.blocked_actors AS d(tenant_id, store_id, actor_key, platform, note, source_bundle_id, principal_id)
        VALUES (v[1], v[2], v_key, v_platform, p_note, v_bundle, v[3]) RETURNING d.id, d.platform, d.created_at INTO r;
    RETURN QUERY SELECT r.id, r.platform, r.created_at, true;
END $$;

-- unblock_actor: removes one entry by its id (live:manage). PT404 when the id is not an entry of this store.
CREATE FUNCTION claims.unblock_actor(p_id uuid) RETURNS boolean
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; v_rows bigint;
BEGIN
    v := claims.for_buyer_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:manage']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF p_id IS NULL THEN RAISE EXCEPTION 'invalid unblock request' USING ERRCODE = '22023'; END IF;
    DELETE FROM claims.blocked_actors d WHERE d.tenant_id = v[1] AND d.store_id = v[2] AND d.id = p_id;
    GET DIAGNOSTICS v_rows = ROW_COUNT;
    IF v_rows <> 1 THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
    RETURN true;
END $$;

-- list_blocked_actors: newest first, keyset (created_at, id); live:read. The note is returned (merchant-only console); the actor_key never is.
CREATE FUNCTION claims.list_blocked_actors(p_after_time timestamptz, p_after_id uuid, p_limit integer)
RETURNS TABLE(id uuid, platform text, note text, source_bundle_id uuid, created_at timestamptz)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[];
BEGIN
    v := claims.for_buyer_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:read']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 101 OR (p_after_time IS NULL) <> (p_after_id IS NULL) THEN
        RAISE EXCEPTION 'invalid blocklist page' USING ERRCODE = '22023';
    END IF;
    RETURN QUERY SELECT d.id, d.platform, d.note, d.source_bundle_id, d.created_at FROM claims.blocked_actors d
     WHERE d.tenant_id = v[1] AND d.store_id = v[2]
       AND (p_after_time IS NULL OR (d.created_at, d.id) < (p_after_time, p_after_id))
     ORDER BY d.created_at DESC, d.id DESC LIMIT p_limit;
END $$;

-- actor_restricted_for_bundle: boolean only (never the note); live:read. PT404 for an unknown / purged bundle; a manual bundle is never restricted.
CREATE FUNCTION claims.actor_restricted_for_bundle(p_bundle uuid) RETURNS boolean
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE v uuid[]; b record;
BEGIN
    v := claims.for_buyer_scope();
    IF NOT identity.principal_holds(v[1], v[2], v[3], ARRAY['live:read']) THEN RAISE EXCEPTION 'forbidden' USING ERRCODE = 'PT403'; END IF;
    IF p_bundle IS NULL THEN RAISE EXCEPTION 'invalid blocklist check' USING ERRCODE = '22023'; END IF;
    SELECT x.platform, x.actor_key INTO b FROM claims.bundles x
     WHERE x.tenant_id = v[1] AND x.store_id = v[2] AND x.id = p_bundle AND x.purged_at IS NULL;
    IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE = 'PT404'; END IF;
    RETURN b.platform IN ('facebook', 'instagram') AND EXISTS (SELECT 1 FROM claims.blocked_actors d
        WHERE d.tenant_id = v[1] AND d.store_id = v[2] AND d.actor_key = b.actor_key);
END $$;

DO $$
DECLARE f text;
BEGIN
    FOREACH f IN ARRAY ARRAY['claims.block_actor(uuid,jsonb,text)', 'claims.unblock_actor(uuid)',
                             'claims.list_blocked_actors(timestamptz,uuid,integer)', 'claims.actor_restricted_for_bundle(uuid)'] LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO commerce_claims_writer', f);
        EXECUTE format('REVOKE ALL ON FUNCTION %s FROM PUBLIC', f);
        EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO commerce_runtime', f);
    END LOOP;
END $$;
COMMENT ON FUNCTION claims.block_actor(uuid,jsonb,text) IS
 'W3-05B (0154). internal/claims BlockActor (commerce_runtime merchant transaction, live:manage re-checked): resolves exactly one server-known reference to the store''s actor_key and lists it; idempotent; <= 5000 entries per store (PT409 limit_reached). The client never supplies an actor_key.';
COMMENT ON FUNCTION claims.unblock_actor(uuid) IS
 'W3-05B (0154). internal/claims UnblockActor (commerce_runtime merchant transaction, live:manage): removes one entry by id; PT404 when it is not an entry of the store.';
COMMENT ON FUNCTION claims.list_blocked_actors(timestamptz,uuid,integer) IS
 'W3-05B (0154). internal/claims ListBlockedActors (commerce_runtime merchant transaction, live:read): entries newest first with the merchant-only note; never the actor_key.';
COMMENT ON FUNCTION claims.actor_restricted_for_bundle(uuid) IS
 'W3-05B (0154). internal/claims BlockedForBundle (commerce_runtime merchant transaction, live:read): whether the bundle''s actor is restricted; boolean only, for the order drawer and buyer panel warning.';
