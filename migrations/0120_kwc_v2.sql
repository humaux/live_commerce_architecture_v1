-- Purpose: Allow the kwc-v2 contains grammar version (K3 adversarial fixes F1-F3) on claims.events, claims.meta_intake and the two intake definers (p_version).
-- Depends on: 0115 (events_grammar_version_check, meta_intake_grammar_version_check, claims.insert_meta_intake / meta_inbox.stage_claim_intake with p_version); roles commerce_claims_writer, commerce_meta_writer, commerce_meta_consumer.
-- Used by: cmd/migrate; internal/claims (events insert), internal/integrations/meta ConsumerWorker (stage_claim_intake with p_version = kwc-v2); tests/foundation KC03 / MCI / TestLiveA7ContainsMode.
-- 0120 kwc-v2 grammar version (docs/delivery/units/kwc-v2.md; contracts/live-keyword-claims-v1.md §2.5 tables are frozen, so the fix is a NEW version).
--
-- Owns: widening grammar_version to ('kw-v1','kwc-v1','kwc-v2') on claims.events and claims.meta_intake, and the p_version
-- allow-list of claims.insert_meta_intake and meta_inbox.stage_claim_intake. kwc-v1 stays valid: stored v1 events and staged
-- v1 intake rows keep their recorded version (replay determinism per stored version, I02).
-- Unchanged on purpose: events_contains_grammar_version (grammar_version='kw-v1' OR match_mode=CONTAINS) already confines
-- kwc-v2 to CONTAINS windows. Definer signature, owner, EXECUTE grants and COMMENT are preserved by CREATE OR REPLACE.
-- Non-goals: no grammar data here (Go: internal/claims/grammar.ParseContainsV2); no backfill; no change to any row.
--
-- Dependencies: 0115 only. Each widened constraint is drift-asserted before DROP (0099/0115 pattern).

DO $$
DECLARE v_def text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='claims.events'::regclass AND c.conname='events_grammar_version_check';
 IF v_def IS NULL OR v_def NOT LIKE '%''kwc-v1''%' OR v_def LIKE '%''kwc-v2''%' THEN
  RAISE EXCEPTION 'claims.events events_grammar_version_check has an unexpected shape: %',v_def;
 END IF;
 ALTER TABLE claims.events DROP CONSTRAINT events_grammar_version_check;
END $$;
ALTER TABLE claims.events ADD CONSTRAINT events_grammar_version_check
 CHECK (grammar_version IN ('kw-v1','kwc-v1','kwc-v2'));

DO $$
DECLARE v_def text;
BEGIN
 SELECT pg_get_constraintdef(c.oid) INTO v_def FROM pg_constraint c
  WHERE c.conrelid='claims.meta_intake'::regclass AND c.conname='meta_intake_grammar_version_check';
 IF v_def IS NULL OR v_def NOT LIKE '%''kwc-v1''%' OR v_def LIKE '%''kwc-v2''%' THEN
  RAISE EXCEPTION 'claims.meta_intake meta_intake_grammar_version_check has an unexpected shape: %',v_def;
 END IF;
 ALTER TABLE claims.meta_intake DROP CONSTRAINT meta_intake_grammar_version_check;
END $$;
ALTER TABLE claims.meta_intake ADD CONSTRAINT meta_intake_grammar_version_check
 CHECK (grammar_version IN ('kw-v1','kwc-v1','kwc-v2'));

-- claims.insert_meta_intake: body identical to 0115 except the p_version allow-list gains 'kwc-v2'.
CREATE OR REPLACE FUNCTION claims.insert_meta_intake(p_tenant uuid,p_store uuid,p_event uuid,p_received timestamptz,
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
  OR p_version IS NULL OR p_version NOT IN ('kw-v1','kwc-v1','kwc-v2')
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

-- meta_inbox.stage_claim_intake: body identical to 0115 except the p_version allow-list gains 'kwc-v2'.
CREATE OR REPLACE FUNCTION meta_inbox.stage_claim_intake(p_event uuid,p_job bigint,p_attempt integer,p_object_id text,
 p_comment_ref text,p_actor_key text,p_occurred timestamptz,p_kind text,p_keyword text,p_quantity integer,p_explicit boolean,
 p_version text)
RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE gate record; e meta_inbox.events%ROWTYPE;
BEGIN
 PERFORM meta_inbox.require_authority('commerce_meta_consumer');
 IF p_object_id IS NULL OR p_object_id !~ '^[0-9_]{1,80}$' OR p_comment_ref IS NULL OR p_comment_ref !~ '^[0-9_]{1,80}$'
  OR p_actor_key IS NULL OR p_actor_key !~ '^[0-9a-f]{64}$' OR p_kind IS NULL OR p_kind NOT IN ('MATCH','NO_MATCH','INVALID_QUANTITY')
  OR p_version IS NULL OR p_version NOT IN ('kw-v1','kwc-v1','kwc-v2') THEN
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
