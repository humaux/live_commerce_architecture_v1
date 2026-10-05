-- Purpose: Add the KEYWORD_QTY_CONTAINS claim-window mode (kwc-v1 grammar version) with interval-scoped match_mode, widened CHECKs and the p_version argument on intake definers (R5 A7).
-- Depends on: claims.windows/claim_window_intervals/events/meta_intake, claims.insert_meta_intake, meta_inbox.stage_claim_intake; roles commerce_claims_writer, commerce_meta_writer, commerce_meta_consumer.
-- Used by: cmd/migrate; internal/claims, internal/integrations/meta; tests/foundation KC03, MCI02, TestLiveA7ContainsMode.
-- 0115 claims KEYWORD_QTY_CONTAINS match mode (R5 A7 "contains KW+N" restricted match;
-- contracts/live-keyword-claims-v1.md §3.5, docs/delivery/units/live-a7-contains-match.md integrator rulings).
--
-- Owns: (1) a third claim-window match mode 'KEYWORD_QTY_CONTAINS' on live.claim_windows.match_mode and
-- claims.events.match_mode, plus the new kwc-v1 grammar version (kw-v1 with a restricted "contains KW+N"
-- fallback) on claims.events.grammar_version and claims.meta_intake.grammar_version; (2) the widened
-- QUANTITY_REQUIRED reason guard so a contains window with no explicit quantity also persists a
-- QUANTITY_REQUIRED event; (3) the cross-check that a kwc-v1 event only ever lands in a CONTAINS window
-- (a kwc-v1 head downgrades to kw-v1 NO_MATCH in any other mode, so it must never be stored as kwc-v1
-- outside CONTAINS); (4) a per-interval match_mode column on live.claim_window_intervals written by the
-- window-history trigger, so a late Meta webhook is judged by the mode of the comment's own interval rather
-- than the window's current mode; (5) the p_version argument on claims.insert_meta_intake and
-- meta_inbox.stage_claim_intake (the consumer SQL edge) so the staged grammar_version is the one actually
-- computed, never hardcoded.
--
-- Non-goals: no grammar data lives here (kwc-v1 parsing is Go: internal/claims/grammar.ParseContains); no
-- change to apps/ UI, OpenAPI schema, go.mod/go.sum or pnpm lockfiles; no production publish/refund/migration.
--
-- Dependencies: 0060 (live.claim_windows, claims.events CHECKs), 0064 (live.claim_window_intervals and its
-- trigger, claims.meta_intake, claims.insert_meta_intake, meta_inbox.stage_claim_intake). No later migration
-- re-derives these constraints or signatures.
--
-- Every widened/replaced constraint is drift-asserted before DROP (0099 pattern): the old definition must
-- still be present and must not yet contain the new marker, else the migration fails loudly instead of
-- silently rewriting a drifted schema.

-- ---------------------------------------------------------------------------------------
-- 1. live.claim_windows.match_mode: EXACT | KEYWORD_QTY_ONLY | KEYWORD_QTY_CONTAINS.
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_def text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='live.claim_windows'::regclass AND c.conname='claim_windows_match_mode_check';
 IF v_def IS NULL OR v_def NOT LIKE '%''KEYWORD_QTY_ONLY''%' OR v_def LIKE '%''KEYWORD_QTY_CONTAINS''%' THEN
  RAISE EXCEPTION 'live.claim_windows claim_windows_match_mode_check has an unexpected shape: %',v_def;
 END IF;
 ALTER TABLE live.claim_windows DROP CONSTRAINT claim_windows_match_mode_check;
END $$;
ALTER TABLE live.claim_windows ADD CONSTRAINT claim_windows_match_mode_check
 CHECK (match_mode IN ('EXACT','KEYWORD_QTY_ONLY','KEYWORD_QTY_CONTAINS'));

-- ---------------------------------------------------------------------------------------
-- 2. claims.events grammar_version / match_mode / QUANTITY_REQUIRED, plus the new cross-check.
--    Column-level CHECKs are matched by their deterministic {table}_{column}_check name; the
--    table-level QUANTITY_REQUIRED CHECK is matched by definition (the only events CHECK that
--    carries both the 'QUANTITY_REQUIRED' literal and the explicit_quantity column).
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_def text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='claims.events'::regclass AND c.conname='events_grammar_version_check';
 IF v_def IS NULL OR v_def NOT LIKE '%''kw-v1''%' OR v_def LIKE '%''kwc-v1''%' THEN
  RAISE EXCEPTION 'claims.events events_grammar_version_check has an unexpected shape: %',v_def;
 END IF;
 ALTER TABLE claims.events DROP CONSTRAINT events_grammar_version_check;
END $$;
ALTER TABLE claims.events ADD CONSTRAINT events_grammar_version_check
 CHECK (grammar_version IN ('kw-v1','kwc-v1'));

DO $$
DECLARE v_def text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='claims.events'::regclass AND c.conname='events_match_mode_check';
 IF v_def IS NULL OR v_def NOT LIKE '%''KEYWORD_QTY_ONLY''%' OR v_def LIKE '%''KEYWORD_QTY_CONTAINS''%' THEN
  RAISE EXCEPTION 'claims.events events_match_mode_check has an unexpected shape: %',v_def;
 END IF;
 ALTER TABLE claims.events DROP CONSTRAINT events_match_mode_check;
END $$;
ALTER TABLE claims.events ADD CONSTRAINT events_match_mode_check
 CHECK (match_mode IN ('EXACT','KEYWORD_QTY_ONLY','KEYWORD_QTY_CONTAINS'));

DO $$
DECLARE v_name text; v_def text;
BEGIN
 SELECT c.conname,pg_get_constraintdef(c.oid) INTO v_name,v_def FROM pg_constraint c
  WHERE c.conrelid='claims.events'::regclass AND c.contype='c'
   AND pg_get_constraintdef(c.oid) LIKE '%''QUANTITY_REQUIRED''%'
   AND pg_get_constraintdef(c.oid) LIKE '%explicit_quantity%'
   AND pg_get_constraintdef(c.oid) NOT LIKE '%''NO_MATCH''%';
 IF v_name IS NULL OR v_def LIKE '%''KEYWORD_QTY_CONTAINS''%' THEN
  RAISE EXCEPTION 'claims.events QUANTITY_REQUIRED CHECK has an unexpected shape: %',v_def;
 END IF;
 EXECUTE format('ALTER TABLE claims.events DROP CONSTRAINT %I',v_name);
END $$;
ALTER TABLE claims.events ADD CONSTRAINT events_quantity_required
 CHECK (reason IS DISTINCT FROM 'QUANTITY_REQUIRED' OR ((match_mode='KEYWORD_QTY_ONLY' OR match_mode='KEYWORD_QTY_CONTAINS') AND explicit_quantity IS FALSE));

-- A kwc-v1 parse only exists in a CONTAINS window (effective() downgrades it to kw-v1 NO_MATCH everywhere
-- else), so a kwc-v1 event with any other match_mode is an invariant breach and never persisted.
ALTER TABLE claims.events ADD CONSTRAINT events_contains_grammar_version
 CHECK (grammar_version='kw-v1' OR match_mode='KEYWORD_QTY_CONTAINS');

-- ---------------------------------------------------------------------------------------
-- 3. claims.meta_intake.grammar_version: widen to kw-v1 | kwc-v1 (staging stores p_version).
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_def text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='claims.meta_intake'::regclass AND c.conname='meta_intake_grammar_version_check';
 IF v_def IS NULL OR v_def NOT LIKE '%''kw-v1''%' OR v_def LIKE '%''kwc-v1''%' THEN
  RAISE EXCEPTION 'claims.meta_intake meta_intake_grammar_version_check has an unexpected shape: %',v_def;
 END IF;
 ALTER TABLE claims.meta_intake DROP CONSTRAINT meta_intake_grammar_version_check;
END $$;
ALTER TABLE claims.meta_intake ADD CONSTRAINT meta_intake_grammar_version_check
 CHECK (grammar_version IN ('kw-v1','kwc-v1'));

-- ---------------------------------------------------------------------------------------
-- 4. live.claim_window_intervals.match_mode: comment-time mode fidelity for late webhooks.
--    Backfill uses the owning window's CURRENT mode (the brief's ruling: per-generation mode
--    was never tracked historically; every interval written after this migration carries the
--    mode of its own generation via the trigger below).
-- ---------------------------------------------------------------------------------------
ALTER TABLE live.claim_window_intervals ADD COLUMN match_mode text;
UPDATE live.claim_window_intervals i
 SET match_mode=w.match_mode
 FROM live.claim_windows w
 WHERE w.tenant_id=i.tenant_id AND w.store_id=i.store_id AND w.session_id=i.session_id;
ALTER TABLE live.claim_window_intervals ALTER COLUMN match_mode SET NOT NULL;
ALTER TABLE live.claim_window_intervals ADD CONSTRAINT claim_window_intervals_match_mode_check
 CHECK (match_mode IN ('EXACT','KEYWORD_QTY_ONLY','KEYWORD_QTY_CONTAINS'));
COMMENT ON COLUMN live.claim_window_intervals.match_mode IS
 'Match mode of the claim window for this interval (EXACT | KEYWORD_QTY_ONLY | KEYWORD_QTY_CONTAINS); written by trigger live.track_claim_window_interval so a late Meta webhook is judged by its own interval''s mode (A7).';

-- ---------------------------------------------------------------------------------------
-- 5. live.track_claim_window_interval: also record NEW.match_mode on OPEN (CREATE OR REPLACE
--    keeps the trigger, owner commerce_claims_writer and the function COMMENT).
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION live.track_claim_window_interval() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_rows bigint;
BEGIN
 IF NEW.state='OPEN' AND (TG_OP='INSERT' OR OLD.state<>'OPEN') THEN
  INSERT INTO live.claim_window_intervals(tenant_id,store_id,session_id,generation,opened_at,match_mode)
   VALUES(NEW.tenant_id,NEW.store_id,NEW.session_id,NEW.generation,NEW.opened_at,NEW.match_mode);
 ELSIF TG_OP='UPDATE' AND OLD.state='OPEN' AND NEW.state<>'OPEN' THEN
  -- Exactly the open interval of the generation that was OPEN; anything else is a corrupted history.
  UPDATE live.claim_window_intervals i SET closed_at=NEW.closed_at
   WHERE i.tenant_id=OLD.tenant_id AND i.store_id=OLD.store_id AND i.session_id=OLD.session_id
    AND i.generation=OLD.generation AND i.closed_at IS NULL;
  GET DIAGNOSTICS v_rows=ROW_COUNT;
  IF v_rows<>1 THEN RAISE EXCEPTION 'claim window interval history broken' USING ERRCODE='23514'; END IF;
 END IF;
 RETURN NULL;
END $$;

-- ---------------------------------------------------------------------------------------
-- 6. claims.insert_meta_intake: add p_version (kw-v1 | kwc-v1) and store it instead of a
--    hardcoded 'kw-v1'. Signature changes, so the old definer is dropped (its only caller,
--    meta_inbox.stage_claim_intake, is dropped first) and re-created with the same
--    owner/EXECUTE/COMMENT shape plus the new argument.
-- ---------------------------------------------------------------------------------------
DROP FUNCTION meta_inbox.stage_claim_intake(uuid,bigint,integer,text,text,text,timestamptz,text,text,integer,boolean);
DROP FUNCTION claims.insert_meta_intake(uuid,uuid,uuid,timestamptz,text,text,text,text,text,text,timestamptz,text,text,integer,boolean,boolean);

CREATE FUNCTION claims.insert_meta_intake(p_tenant uuid,p_store uuid,p_event uuid,p_received timestamptz,
 p_app text,p_object text,p_asset text,p_object_id text,p_comment_ref text,p_actor_key text,
 p_occurred timestamptz,p_kind text,p_keyword text,p_quantity integer,p_explicit boolean,p_live_media boolean,
 p_version text)
RETURNS uuid LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_offer uuid; v_unknown boolean:=false; v_id uuid;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed'
  OR nullif(current_setting('app.tenant_id',true),'') IS NOT NULL OR nullif(current_setting('app.store_id',true),'') IS NOT NULL
  OR nullif(current_setting('app.principal_id',true),'') IS NOT NULL OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL
  OR p_tenant IS NULL OR p_store IS NULL OR p_event IS NULL OR p_received IS NULL OR p_occurred IS NULL OR p_live_media IS NULL
  OR p_app IS NULL OR p_app !~ '^[0-9]{1,40}$' OR p_object IS NULL OR p_object NOT IN ('page','instagram')
  OR p_asset IS NULL OR p_asset !~ '^[0-9]{1,40}$' OR p_object_id IS NULL OR p_object_id !~ '^[0-9_]{1,80}$'
  OR p_comment_ref IS NULL OR p_comment_ref !~ '^[0-9_]{1,80}$' OR p_actor_key IS NULL OR p_actor_key !~ '^[0-9a-f]{64}$'
  OR p_kind IS NULL OR p_kind NOT IN ('MATCH','NO_MATCH','INVALID_QUANTITY')
  OR p_version IS NULL OR p_version NOT IN ('kw-v1','kwc-v1')
  OR (p_kind='NO_MATCH' AND (p_keyword IS NOT NULL OR p_quantity IS NOT NULL OR p_explicit IS NOT NULL))
  OR (p_kind='INVALID_QUANTITY' AND (p_keyword IS NULL OR p_quantity IS NOT NULL OR p_explicit IS NOT NULL))
  OR (p_kind='MATCH' AND (p_keyword IS NULL OR p_quantity IS NULL OR p_quantity NOT BETWEEN 1 AND 999 OR p_explicit IS NULL))
  OR (p_keyword IS NOT NULL AND p_keyword !~ '^[A-Z0-9]{1,16}$') THEN
  RAISE EXCEPTION 'invalid meta intake' USING ERRCODE='22023';
 END IF;
 -- Source lock first (lock order §9: consumer -> claim_sources FOR NO KEY UPDATE -> offers read).
 SELECT x.id,x.tenant_id,x.store_id,x.session_id,x.platform,x.intake_count INTO s FROM live.claim_sources x
  WHERE x.object=p_object AND x.asset_id=p_asset AND x.source_object_id=p_object_id AND x.active FOR NO KEY UPDATE;
 IF NOT FOUND OR s.tenant_id<>p_tenant OR s.store_id<>p_store
  OR s.platform<>(CASE p_object WHEN 'page' THEN 'facebook' ELSE 'instagram' END) THEN
  RETURN NULL;
 END IF;
 -- Staging bound (I23): a viral post cannot grow the queue without limit; NO_MATCH comments count.
 IF s.intake_count>=50000 THEN
  UPDATE live.claim_sources SET intake_capped=intake_capped+1 WHERE tenant_id=s.tenant_id AND store_id=s.store_id AND id=s.id;
  RETURN NULL;
 END IF;
 IF p_kind<>'NO_MATCH' THEN
  SELECT o.id INTO v_offer FROM live.offers o
   WHERE o.tenant_id=p_tenant AND o.store_id=p_store AND o.session_id=s.session_id AND o.keyword=p_keyword;
  v_unknown:=v_offer IS NULL;
 END IF;
 INSERT INTO claims.meta_intake(tenant_id,store_id,inbox_event_id,source_id,session_id,platform,app_id,object,asset_id,
  comment_ref,live_media,actor_key,occurred_at,received_at,grammar_version,grammar_kind,offer_id,unknown_keyword,
  quantity,explicit_quantity,state,attempts,not_before,lease_xid)
 VALUES(p_tenant,p_store,p_event,s.id,s.session_id,s.platform,p_app,p_object,p_asset,p_comment_ref,p_live_media,p_actor_key,
  p_occurred,p_received,p_version,p_kind,v_offer,v_unknown,CASE WHEN v_unknown THEN NULL ELSE p_quantity END,
  CASE WHEN v_unknown THEN NULL ELSE p_explicit END,'PENDING',0,clock_timestamp(),NULL)
 ON CONFLICT DO NOTHING RETURNING id INTO v_id;
 IF v_id IS NULL THEN RETURN NULL; END IF;   -- one intake per comment, globally (any app, any kind)
 UPDATE live.claim_sources SET intake_count=intake_count+1 WHERE tenant_id=s.tenant_id AND store_id=s.store_id AND id=s.id;
 RETURN v_id;
END $$;
ALTER FUNCTION claims.insert_meta_intake(uuid,uuid,uuid,timestamptz,text,text,text,text,text,text,timestamptz,text,text,integer,boolean,boolean,text)
 OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.insert_meta_intake(uuid,uuid,uuid,timestamptz,text,text,text,text,text,text,timestamptz,text,text,integer,boolean,boolean,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.insert_meta_intake(uuid,uuid,uuid,timestamptz,text,text,text,text,text,text,timestamptz,text,text,integer,boolean,boolean,text)
 TO commerce_meta_writer;
COMMENT ON FUNCTION claims.insert_meta_intake(uuid,uuid,uuid,timestamptz,text,text,text,text,text,text,timestamptz,text,text,integer,boolean,boolean,text) IS
 'internal/claims; only caller meta_inbox.stage_claim_intake (owner commerce_meta_writer). Locks the active claim source, applies the 50000 staging bound, resolves the offer keyword (never stored) and inserts one text-free claims.meta_intake row carrying p_version (kw-v1 | kwc-v1), NULL when nothing is staged.';

-- ---------------------------------------------------------------------------------------
-- 7. meta_inbox.stage_claim_intake: add p_version and forward it to claims.insert_meta_intake.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION meta_inbox.stage_claim_intake(p_event uuid,p_job bigint,p_attempt integer,p_object_id text,
 p_comment_ref text,p_actor_key text,p_occurred timestamptz,p_kind text,p_keyword text,p_quantity integer,p_explicit boolean,
 p_version text)
RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE gate record; e meta_inbox.events%ROWTYPE;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_consumer');
 IF p_object_id IS NULL OR p_object_id !~ '^[0-9_]{1,80}$' OR p_comment_ref IS NULL OR p_comment_ref !~ '^[0-9_]{1,80}$'
  OR p_actor_key IS NULL OR p_actor_key !~ '^[0-9a-f]{64}$' OR p_kind IS NULL OR p_kind NOT IN ('MATCH','NO_MATCH','INVALID_QUANTITY')
  OR p_version IS NULL OR p_version NOT IN ('kw-v1','kwc-v1') THEN
  RAISE EXCEPTION 'invalid meta claim intake' USING ERRCODE='22023';
 END IF;
 SELECT * INTO gate FROM meta_inbox.social_source(p_event,p_job,p_attempt,false);
 e:=gate.source;
 IF gate.outcome<>'READY' THEN RAISE EXCEPTION 'meta claim intake policy denied' USING ERRCODE='PT409'; END IF;
 -- Only a comment that qualifies, and only the comment fact this very transaction wrote.
 IF e.kind NOT IN ('page_comment_add','instagram_comment','instagram_live_comment')
  OR NOT EXISTS(SELECT 1 FROM social.comment_events c WHERE c.event_id=e.id AND c.tenant_id=e.tenant_id AND c.store_id=e.store_id
   AND c.kind=e.kind AND c.xmin=pg_current_xact_id()::xid) THEN
  RAISE EXCEPTION 'invalid meta claim intake' USING ERRCODE='22023';
 END IF;
 -- Ruling (c): IG units carry no time (U7), so the locked event's occurred_at is the comment time.
 IF e.occurred_at IS NULL THEN RETURN NULL; END IF;
 IF p_occurred IS NOT NULL AND p_occurred<>e.occurred_at THEN
  RAISE EXCEPTION 'invalid meta claim intake' USING ERRCODE='22023';
 END IF;
 RETURN claims.insert_meta_intake(e.tenant_id,e.store_id,e.id,e.created_at,e.app_id,e.object,e.asset_id,p_object_id,
  p_comment_ref,p_actor_key,e.occurred_at,p_kind,p_keyword,p_quantity,p_explicit,e.kind='instagram_live_comment',p_version);
END $$;
ALTER FUNCTION meta_inbox.stage_claim_intake(uuid,bigint,integer,text,text,text,timestamptz,text,text,integer,boolean,text) OWNER TO commerce_meta_writer;
REVOKE ALL ON FUNCTION meta_inbox.stage_claim_intake(uuid,bigint,integer,text,text,text,timestamptz,text,text,integer,boolean,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION meta_inbox.stage_claim_intake(uuid,bigint,integer,text,text,text,timestamptz,text,text,integer,boolean,text) TO commerce_meta_consumer;
COMMENT ON FUNCTION meta_inbox.stage_claim_intake(uuid,bigint,integer,text,text,text,timestamptz,text,text,integer,boolean,text) IS
 'meta_inbox owner; only caller internal/integrations/meta ConsumerWorker (commerce_meta_consumer) after finish_social_event, before COMMIT. Re-derives scope from the locked event and running river_meta job, then calls claims.insert_meta_intake with p_version (kw-v1 | kwc-v1). NULL = not staged.';
