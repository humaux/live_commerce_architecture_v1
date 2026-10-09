-- 0169 LC-R2 C3x purge (unit lc-r2-c3-print-purge; contracts/live-console-v1.md §7.4 + §10 "C3 (extended)";
-- amends contracts/claims-retention-purge-v1.md §1 "Not purged here").
--
-- Purpose: nothing deleted live.comment_prints, so the A3 comment-label print fact and its ops.command_results
-- receipt (operation live.comment.print) outlived intake_days, breaching live-console-v1 §10. This recreates
-- claims.run_retention from its newest definition (0127) and adds a C3x step that DELETEs, at intake_days: the
-- print fact (aged by first_printed_at, its creation stamp — the table has no created_at), the print receipt
-- (aged by created_at, scoped to operation='live.comment.print' so the shared ledger's other operations are never
-- touched) and inbox.bundle_peers (aged by created_at). It also wires the two other §10 bundle_peers rules: delete
-- on bundle de-identify (C2, inside run_retention) and on actor erasure (RD4, peer_key-scoped inside
-- claims.apply_actor_erasure, recreated here from 0127 + the 0154 blocked_actors patch). New numeric count keys
-- prints / print_receipts / bundle_peers join the run log and the erasure counts (numbers only).
--
-- Depends on: 0127 (claims.run_retention / apply_actor_erasure being recreated), 0154 (the blocked_actors patch of
-- apply_actor_erasure, preserved here), 0123 (live.comment_prints), 0002 (ops.command_results), 0128
-- (inbox.bundle_peers), 0071 (the retention roles and the §4 grant/policy pattern).
--
-- Used by: the claims retention definers claims.run_retention (River job claims_retention_v1 / retention-admin run)
-- and claims.apply_actor_erasure (erase_actor / replay_actor_erasures); internal/retention (allowedCounts);
-- tests/foundation/claims_retention_test.go (CRP02 matrix/RLS and the C3x purge gate).

DO $$
BEGIN
 IF to_regprocedure('claims.run_retention(integer)') IS NULL
  OR to_regprocedure('claims.apply_actor_erasure(text,text,uuid,uuid,uuid,text[])') IS NULL
  OR to_regclass('live.comment_prints') IS NULL
  OR to_regclass('ops.command_results') IS NULL
  OR to_regclass('inbox.bundle_peers') IS NULL THEN
  RAISE EXCEPTION '0169 requires 0127 (retention definers), 0123 (live.comment_prints), 0002 (ops.command_results), 0128 (inbox.bundle_peers)' USING ERRCODE='55000';
 END IF;
END $$;

-- ---------------------------------------------------------------------------------------
-- §4 privileges of commerce_retention_writer on the three C3x targets (least privilege; the definer bodies are the
-- control). "lock-only" = UPDATE(col) with a FOR UPDATE policy USING(...) WITH CHECK(false): PostgreSQL needs an
-- UPDATE privilege for the FOR UPDATE SKIP LOCKED row lock and any real update then fails. Every policy is TO
-- commerce_retention_writer only.
-- ---------------------------------------------------------------------------------------
GRANT USAGE ON SCHEMA inbox, ops TO commerce_retention_writer;

-- live.comment_prints (C3x age): the PK columns key the DELETE, first_printed_at ages/selects (the row's creation
-- stamp; comment_prints has no created_at), plus a table DELETE and a lock-only UPDATE for FOR UPDATE SKIP LOCKED.
GRANT SELECT(tenant_id,store_id,session_id,comment_ref,first_printed_at) ON live.comment_prints TO commerce_retention_writer;
GRANT DELETE ON live.comment_prints TO commerce_retention_writer;
GRANT UPDATE(first_printed_at) ON live.comment_prints TO commerce_retention_writer;
CREATE POLICY comment_print_retention_read ON live.comment_prints FOR SELECT TO commerce_retention_writer USING (true);
CREATE POLICY comment_print_retention_delete ON live.comment_prints FOR DELETE TO commerce_retention_writer USING (true);
CREATE POLICY comment_print_retention_lock ON live.comment_prints FOR UPDATE TO commerce_retention_writer USING (true) WITH CHECK (false);
CREATE INDEX comment_print_retention ON live.comment_prints(first_printed_at);

-- ops.command_results (C3x age): the shared receipt ledger holds every operation, so all three policies are scoped to
-- operation='live.comment.print' — the retention role may not read, lock or delete any other operation's receipt.
GRANT SELECT(tenant_id,store_id,operation,idempotency_key,created_at) ON ops.command_results TO commerce_retention_writer;
GRANT DELETE ON ops.command_results TO commerce_retention_writer;
GRANT UPDATE(created_at) ON ops.command_results TO commerce_retention_writer;
CREATE POLICY command_print_retention_read ON ops.command_results FOR SELECT TO commerce_retention_writer USING (operation='live.comment.print');
CREATE POLICY command_print_retention_delete ON ops.command_results FOR DELETE TO commerce_retention_writer USING (operation='live.comment.print');
CREATE POLICY command_print_retention_lock ON ops.command_results FOR UPDATE TO commerce_retention_writer USING (operation='live.comment.print') WITH CHECK (false);
CREATE INDEX command_print_retention ON ops.command_results(created_at) WHERE operation='live.comment.print';

-- inbox.bundle_peers (C3x age + C2 de-identify + RD4 peer_key): the PK columns key the deletes, created_at
-- ages/selects the C3x batch, peer_key/bundle_id scope the RD4/C2 deletes; lock-only UPDATE for FOR UPDATE SKIP LOCKED.
GRANT SELECT(tenant_id,store_id,bundle_id,peer_key,created_at) ON inbox.bundle_peers TO commerce_retention_writer;
GRANT DELETE ON inbox.bundle_peers TO commerce_retention_writer;
GRANT UPDATE(created_at) ON inbox.bundle_peers TO commerce_retention_writer;
CREATE POLICY bundle_peer_retention_read ON inbox.bundle_peers FOR SELECT TO commerce_retention_writer USING (true);
CREATE POLICY bundle_peer_retention_delete ON inbox.bundle_peers FOR DELETE TO commerce_retention_writer USING (true);
CREATE POLICY bundle_peer_retention_lock ON inbox.bundle_peers FOR UPDATE TO commerce_retention_writer USING (true) WITH CHECK (false);
CREATE INDEX bundle_peer_retention ON inbox.bundle_peers(created_at);

COMMENT ON POLICY comment_print_retention_read ON live.comment_prints IS
 '0169 LC-R2: TO commerce_retention_writer (NOLOGIN definer owner, internal/retention) only; read access for the C3x print-fact purge (claims.run_retention).';
COMMENT ON POLICY comment_print_retention_delete ON live.comment_prints IS
 '0169 LC-R2: TO commerce_retention_writer only; delete access for the C3x print-fact purge (first_printed_at older than intake_days).';
COMMENT ON POLICY comment_print_retention_lock ON live.comment_prints IS
 '0169 LC-R2: TO commerce_retention_writer only; lock-only UPDATE (WITH CHECK false) for the C3x FOR UPDATE SKIP LOCKED row selection.';
COMMENT ON POLICY command_print_retention_read ON ops.command_results IS
 '0169 LC-R2: TO commerce_retention_writer only; read access for the C3x receipt purge, scoped to operation=live.comment.print (no other receipt is visible).';
COMMENT ON POLICY command_print_retention_delete ON ops.command_results IS
 '0169 LC-R2: TO commerce_retention_writer only; delete access for the C3x receipt purge, scoped to operation=live.comment.print and created_at older than intake_days.';
COMMENT ON POLICY command_print_retention_lock ON ops.command_results IS
 '0169 LC-R2: TO commerce_retention_writer only; lock-only UPDATE (WITH CHECK false) for the C3x FOR UPDATE SKIP LOCKED row selection, scoped to operation=live.comment.print.';
COMMENT ON POLICY bundle_peer_retention_read ON inbox.bundle_peers IS
 '0169 LC-R2: TO commerce_retention_writer only; read access for the C3x age purge and the C2/RD4 bundle_peers deletes.';
COMMENT ON POLICY bundle_peer_retention_delete ON inbox.bundle_peers IS
 '0169 LC-R2: TO commerce_retention_writer only; delete access for the C3x age purge (created_at older than intake_days), the C2 de-identify hook and the RD4 peer_key hook.';
COMMENT ON POLICY bundle_peer_retention_lock ON inbox.bundle_peers IS
 '0169 LC-R2: TO commerce_retention_writer only; lock-only UPDATE (WITH CHECK false) for the C3x FOR UPDATE SKIP LOCKED row selection.';

-- ---------------------------------------------------------------------------------------
-- claims.run_retention (job, operator): recreated from 0127 (the newest definition) with the C3x step added.
-- C1-C6 are byte-for-byte the 0127 behaviour; C3x (live.comment_prints, live.comment.print receipts,
-- inbox.bundle_peers) is new, and the C2 loop now also deletes a de-identified bundle's peer links.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION claims.run_retention(p_limit integer) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET lock_timeout='2s' AS $$
DECLARE pol claims.retention_policy%ROWTYPE; v_now timestamptz:=clock_timestamp();
 n_links integer:=0; n_bundles integer:=0; n_intake integer:=0; n_ops integer:=0; n_comments integer:=0;
 n_messages integer:=0; n_conv integer:=0; n_prints integer:=0; n_print_receipts integer:=0; n_bundle_peers integer:=0;
 v_more boolean:=false; r record; v_counts jsonb;
BEGIN
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 1000 THEN
  RAISE EXCEPTION 'invalid retention limit' USING ERRCODE='22023';
 END IF;
 -- busy: another run (or an erasure) holds the single advisory key; the next hour retries.
 IF NOT pg_try_advisory_xact_lock(hashtextextended('claims-retention',0)) THEN
  RETURN jsonb_build_object('busy',1);
 END IF;
 -- The policy row is read under FOR SHARE so set_retention_policy (row FOR UPDATE) cannot flip mid-batch;
 -- a policy write in flight makes this batch busy, the next one sees the new value.
 SELECT * INTO pol FROM claims.retention_policy WHERE id FOR SHARE SKIP LOCKED;
 IF NOT FOUND THEN RETURN jsonb_build_object('busy',1); END IF;

 -- C1 links: expired long enough. Not enforced: count only.
 IF pol.enforced THEN
  WITH c AS (SELECT l.tenant_id,l.store_id,l.bundle_id FROM claims.links l
    WHERE l.expires_at < v_now - make_interval(days=>pol.link_days) ORDER BY l.expires_at LIMIT p_limit
    FOR UPDATE SKIP LOCKED),
  d AS (DELETE FROM claims.links k USING c WHERE k.tenant_id=c.tenant_id AND k.store_id=c.store_id
    AND k.bundle_id=c.bundle_id RETURNING 1)
  SELECT count(*) INTO n_links FROM d;
 ELSE
  SELECT count(*) INTO n_links FROM (SELECT 1 FROM claims.links l
   WHERE l.expires_at < v_now - make_interval(days=>pol.link_days) LIMIT p_limit) s;
 END IF;

 -- C2 claim identity: RD1 de-identify in place. Window CLOSED for claims_days (rechecked under FOR SHARE: a
 -- reopen in between skips the bundle); lock order windows -> bundles -> lines -> links.
 FOR r IN SELECT b.tenant_id,b.store_id,b.id,b.session_id,b.platform FROM claims.bundles b
   JOIN live.claim_windows w ON w.tenant_id=b.tenant_id AND w.store_id=b.store_id AND w.session_id=b.session_id
   WHERE b.purged_at IS NULL AND w.state='CLOSED' AND w.closed_at < v_now - make_interval(days=>pol.claims_days)
   ORDER BY w.closed_at, b.id LIMIT p_limit LOOP
  IF NOT pol.enforced THEN n_bundles:=n_bundles+1; CONTINUE; END IF;
  PERFORM 1 FROM live.claim_windows w WHERE w.tenant_id=r.tenant_id AND w.store_id=r.store_id AND w.session_id=r.session_id
   AND w.state='CLOSED' AND w.closed_at < v_now - make_interval(days=>pol.claims_days) FOR SHARE SKIP LOCKED;
  IF NOT FOUND THEN CONTINUE; END IF;
  PERFORM 1 FROM claims.bundles b WHERE b.tenant_id=r.tenant_id AND b.store_id=r.store_id AND b.id=r.id
   AND b.purged_at IS NULL FOR NO KEY UPDATE SKIP LOCKED;
  IF NOT FOUND THEN CONTINUE; END IF;
  UPDATE claims.lines l SET applied_version=NULL WHERE l.tenant_id=r.tenant_id AND l.store_id=r.store_id AND l.bundle_id=r.id;
  DELETE FROM claims.links k WHERE k.tenant_id=r.tenant_id AND k.store_id=r.store_id AND k.bundle_id=r.id;
  -- C3x (0169 LC-R2, live-console-v1 §10): a de-identified bundle's DM peer links go with it, like its links above.
  -- Uncounted (C2 is a de-identify, not a purge class); the age rule below counts the intake_days purge separately.
  DELETE FROM inbox.bundle_peers bp WHERE bp.tenant_id=r.tenant_id AND bp.store_id=r.store_id AND bp.bundle_id=r.id;
  -- RD1: random key and random label (never derived from the bundle id), binding cleared, purged_at set.
  UPDATE claims.bundles b SET
    actor_key=replace(gen_random_uuid()::text,'-','')||replace(gen_random_uuid()::text,'-',''),
    label=CASE WHEN b.platform='manual' THEN 'purged-'||replace(gen_random_uuid()::text,'-','') END,
    owner_id=NULL, bound_at=NULL, purged_at=v_now, updated_at=v_now
   WHERE b.tenant_id=r.tenant_id AND b.store_id=r.store_id AND b.id=r.id;
  n_bundles:=n_bundles+1;
 END LOOP;

 -- C3 intake: terminal rows older than intake_days (>= 8, RD5).
 IF pol.enforced THEN
  WITH c AS (SELECT x.tenant_id,x.store_id,x.id FROM claims.meta_intake x
    WHERE x.state IN ('APPLIED','DROPPED','FAILED') AND x.received_at < v_now - make_interval(days=>pol.intake_days)
    ORDER BY x.received_at LIMIT p_limit FOR UPDATE SKIP LOCKED),
  d AS (DELETE FROM claims.meta_intake m USING c WHERE m.tenant_id=c.tenant_id AND m.store_id=c.store_id
    AND m.id=c.id AND m.state<>'PENDING' RETURNING 1)
  SELECT count(*) INTO n_intake FROM d;
 ELSE
  SELECT count(*) INTO n_intake FROM (SELECT 1 FROM claims.meta_intake x
   WHERE x.state IN ('APPLIED','DROPPED','FAILED') AND x.received_at < v_now - make_interval(days=>pol.intake_days)
   LIMIT p_limit) s;
 END IF;

 -- C4 send ledger: redact comment_ref/conversation_id/peer_key of the four send actions once they are terminal,
 -- or UNKNOWN with an expired (or no) lease; request_hash keeps the original's hash. offer_recommend never holds
 -- one of the ids, so it is never eligible (A1.4 clause 2). A leased reconciling UNKNOWN row is skipped.
 IF pol.enforced THEN
  WITH c AS (SELECT o.id FROM integration.operations o
    WHERE o.action IN ('meta.private_reply','meta.dm_send','meta.public_reply','meta.offer_recommend')
     AND o.created_at < v_now - make_interval(days=>pol.intake_days)
     AND o.request ?| array['comment_ref','conversation_id','peer_key']
     AND (o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING')
      OR (o.state='UNKNOWN' AND (o.lease_until IS NULL OR o.lease_until < v_now)))
    ORDER BY o.created_at LIMIT p_limit FOR UPDATE SKIP LOCKED),
  d AS (UPDATE integration.operations o SET request=(o.request - 'comment_ref' - 'conversation_id' - 'peer_key') || '{"redacted":true}'::jsonb,
    semantic_key=CASE o.action WHEN 'meta.private_reply' THEN 'mpr-purged:' WHEN 'meta.dm_send' THEN 'mdm-purged:'
      WHEN 'meta.public_reply' THEN 'mpub-purged:' ELSE 'mrec-purged:' END || o.id::text, updated_at=v_now
    FROM c WHERE o.id=c.id RETURNING 1)
  SELECT count(*) INTO n_ops FROM d;
 ELSE
  SELECT count(*) INTO n_ops FROM (SELECT 1 FROM integration.operations o
   WHERE o.action IN ('meta.private_reply','meta.dm_send','meta.public_reply','meta.offer_recommend')
    AND o.created_at < v_now - make_interval(days=>pol.intake_days)
    AND o.request ?| array['comment_ref','conversation_id','peer_key']
    AND (o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING')
     OR (o.state='UNKNOWN' AND (o.lease_until IS NULL OR o.lease_until < v_now))) LIMIT p_limit) s;
 END IF;

 -- C3x (0169 LC-R2, live-console-v1 §10 "C3 (extended)"): the comment-label print fact, its A3 receipt and the DM
 -- peer links age out at intake_days. Each is a batched DELETE with FOR UPDATE SKIP LOCKED (mirrors C1/C3). The
 -- print fact is keyed by first_printed_at (its creation stamp; live.comment_prints has no created_at), the receipt
 -- by created_at and scoped to operation='live.comment.print' (the shared ops.command_results ledger holds every
 -- operation, so no other receipt is ever read or deleted), the peer link by created_at. Not enforced: count only.
 IF pol.enforced THEN
  WITH c AS (SELECT p.tenant_id,p.store_id,p.session_id,p.comment_ref FROM live.comment_prints p
    WHERE p.first_printed_at < v_now - make_interval(days=>pol.intake_days)
    ORDER BY p.first_printed_at LIMIT p_limit FOR UPDATE SKIP LOCKED),
  d AS (DELETE FROM live.comment_prints k USING c WHERE k.tenant_id=c.tenant_id AND k.store_id=c.store_id
    AND k.session_id=c.session_id AND k.comment_ref=c.comment_ref RETURNING 1)
  SELECT count(*) INTO n_prints FROM d;
 ELSE
  SELECT count(*) INTO n_prints FROM (SELECT 1 FROM live.comment_prints p
   WHERE p.first_printed_at < v_now - make_interval(days=>pol.intake_days) LIMIT p_limit) s;
 END IF;
 IF pol.enforced THEN
  WITH c AS (SELECT o.tenant_id,o.store_id,o.operation,o.idempotency_key FROM ops.command_results o
    WHERE o.operation='live.comment.print' AND o.created_at < v_now - make_interval(days=>pol.intake_days)
    ORDER BY o.created_at LIMIT p_limit FOR UPDATE SKIP LOCKED),
  d AS (DELETE FROM ops.command_results k USING c WHERE k.tenant_id=c.tenant_id AND k.store_id=c.store_id
    AND k.operation=c.operation AND k.idempotency_key=c.idempotency_key RETURNING 1)
  SELECT count(*) INTO n_print_receipts FROM d;
 ELSE
  SELECT count(*) INTO n_print_receipts FROM (SELECT 1 FROM ops.command_results o
   WHERE o.operation='live.comment.print' AND o.created_at < v_now - make_interval(days=>pol.intake_days) LIMIT p_limit) s;
 END IF;
 IF pol.enforced THEN
  WITH c AS (SELECT b.tenant_id,b.store_id,b.bundle_id,b.peer_key FROM inbox.bundle_peers b
    WHERE b.created_at < v_now - make_interval(days=>pol.intake_days)
    ORDER BY b.created_at LIMIT p_limit FOR UPDATE SKIP LOCKED),
  d AS (DELETE FROM inbox.bundle_peers k USING c WHERE k.tenant_id=c.tenant_id AND k.store_id=c.store_id
    AND k.bundle_id=c.bundle_id AND k.peer_key=c.peer_key RETURNING 1)
  SELECT count(*) INTO n_bundle_peers FROM d;
 ELSE
  SELECT count(*) INTO n_bundle_peers FROM (SELECT 1 FROM inbox.bundle_peers b
   WHERE b.created_at < v_now - make_interval(days=>pol.intake_days) LIMIT p_limit) s;
 END IF;

 -- C5 social ciphertext: old enough AND the inbox event and its River job are terminal (lock_purgeable holds the job
 -- row through the delete). A non-terminal consumer job keeps its row, else its retry would hit social_terminal XX000.
 -- ponytail: the scan reads at most 10*p_limit candidates per class so a few stuck rows do not starve the queue;
 -- more than 10*p_limit old non-terminal rows would stall it (raise the limit or fix the stuck jobs).
 FOR r IN SELECT c.event_id FROM social.comment_events c WHERE c.received_at < v_now - make_interval(days=>pol.social_days)
   ORDER BY c.received_at LIMIT p_limit*10 LOOP
  EXIT WHEN n_comments>=p_limit;
  IF meta_inbox.lock_purgeable(r.event_id) THEN
   IF pol.enforced THEN DELETE FROM social.comment_events c WHERE c.event_id=r.event_id; END IF;
   n_comments:=n_comments+1;
  END IF;
 END LOOP;
 FOR r IN SELECT m.event_id FROM social.messages m WHERE m.received_at < v_now - make_interval(days=>pol.social_days)
   ORDER BY m.received_at LIMIT p_limit*10 LOOP
  EXIT WHEN n_messages>=p_limit;
  IF meta_inbox.lock_purgeable(r.event_id) THEN
   IF pol.enforced THEN DELETE FROM social.messages m WHERE m.event_id=r.event_id; END IF;
   n_messages:=n_messages+1;
  END IF;
 END LOOP;

 -- C5b conversations without messages. Lock the row first, then re-check with a fresh statement: a consumer that
 -- committed a message in between wins (READ COMMITTED sees it) and one still holding next_seq makes us skip.
 FOR r IN SELECT v.id FROM social.conversations v WHERE v.created_at < v_now - make_interval(days=>pol.social_days)
   AND NOT EXISTS(SELECT 1 FROM social.messages m WHERE m.conversation_id=v.id) ORDER BY v.created_at LIMIT p_limit LOOP
  IF NOT pol.enforced THEN n_conv:=n_conv+1; CONTINUE; END IF;
  PERFORM 1 FROM social.conversations v WHERE v.id=r.id FOR UPDATE SKIP LOCKED;
  IF NOT FOUND THEN CONTINUE; END IF;
  IF EXISTS(SELECT 1 FROM social.messages m WHERE m.conversation_id=r.id) THEN CONTINUE; END IF;
  DELETE FROM social.conversations v WHERE v.id=r.id;
  n_conv:=n_conv+1;
 END LOOP;

 -- C6 run log: only in enforced mode, uncounted; erasure and policy rows are kept.
 IF pol.enforced THEN
  DELETE FROM claims.retention_log g WHERE g.id IN (SELECT x.id FROM claims.retention_log x
   WHERE x.kind='run' AND x.created_at < v_now - interval '400 days' ORDER BY x.created_at LIMIT p_limit);
 END IF;

 -- more=1 only when enforced: a report-only run removes nothing, so its backlog never shrinks and more=1 would make the
 -- job rerun 20 batches an hour and trip the runbook's last_run_more escalation (r2-close-retention).
 v_more:=pol.enforced AND greatest(n_links,n_bundles,n_intake,n_ops,n_comments,n_messages,n_conv,n_prints,n_print_receipts,n_bundle_peers)>=p_limit;
 v_counts:=jsonb_build_object('enforced',pol.enforced::int,'links',n_links,'bundles',n_bundles,'intake',n_intake,
  'operations',n_ops,'comment_events',n_comments,'messages',n_messages,'conversations',n_conv,
  'prints',n_prints,'print_receipts',n_print_receipts,'bundle_peers',n_bundle_peers,'more',v_more::int);
 INSERT INTO claims.retention_log(kind,counts) VALUES('run',v_counts);
 RETURN v_counts;
END $$;
ALTER FUNCTION claims.run_retention(integer) OWNER TO commerce_retention_writer;
REVOKE ALL ON FUNCTION claims.run_retention(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.run_retention(integer) TO commerce_retention_job, commerce_retention_operator;

-- ---------------------------------------------------------------------------------------
-- claims.apply_actor_erasure (internal): recreated from 0127 + the 0154 blocked_actors patch, with the RD4
-- bundle_peers delete added (live-console-v1 §10: "for every peer_key it resolves, it also deletes that peer's
-- ... inbox.bundle_peers rows"). Peer-key scoped, inside the existing p_peer_keys block (so a manual-bundle erasure,
-- which has no peer keys, does not touch bundle_peers); COUNTED, so an actor whose only remaining record is a peer
-- link is erased, not reported as not found (the 0154 blocked_actors rationale). No EXECUTE grant: erase_actor and
-- replay_actor_erasures (same owner) call it.
-- ---------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION claims.apply_actor_erasure(p_platform text, p_actor_key text, p_tenant uuid, p_store uuid,
 p_bundle uuid, p_peer_keys text[]) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog SET lock_timeout='2s' AS $$
DECLARE v_now timestamptz:=clock_timestamp(); v_single boolean:=p_bundle IS NOT NULL;
 n_bundles integer:=0; n_lines integer:=0; n_links integer:=0; n_intake integer:=0; n_comments integer:=0;
 n_ops integer:=0; n_messages integer:=0; n_conv integer:=0; n_defer integer:=0; n_peers integer:=0; r record;
 v_comments text[]; v_n integer;
 v_replay boolean:=coalesce(current_setting('lc.retention_replay',true),'')='on';
BEGIN
 IF (v_single AND (p_tenant IS NULL OR p_store IS NULL OR p_actor_key IS NOT NULL))
  OR (NOT v_single AND (p_platform IS NULL OR p_platform NOT IN ('facebook','instagram')
   OR p_actor_key IS NULL OR p_actor_key !~ '^[0-9a-f]{64}$'))
  OR (p_peer_keys IS NOT NULL AND (v_single OR array_ndims(p_peer_keys)<>1 OR cardinality(p_peer_keys)>8
   OR EXISTS(SELECT 1 FROM unnest(p_peer_keys) x WHERE x IS NULL OR x !~ '^[0-9a-f]{64}$'))) THEN
  RAISE EXCEPTION 'invalid actor erasure' USING ERRCODE='22023';
 END IF;
 -- Lock order: windows FOR SHARE (a reopen waits) -> bundles FOR NO KEY UPDATE (fixed order, blocking: the caller
 -- holds the advisory key and lock_timeout turns a stuck row into 55P03).
 PERFORM 1 FROM live.claim_windows w WHERE (w.tenant_id,w.store_id,w.session_id) IN (
   SELECT b.tenant_id,b.store_id,b.session_id FROM claims.bundles b WHERE b.purged_at IS NULL AND
    CASE WHEN v_single THEN b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=p_bundle
     ELSE b.platform=p_platform AND b.actor_key=p_actor_key END)
  ORDER BY w.tenant_id,w.store_id,w.session_id FOR SHARE;
 PERFORM 1 FROM claims.bundles b WHERE b.purged_at IS NULL AND
   CASE WHEN v_single THEN b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=p_bundle
    ELSE b.platform=p_platform AND b.actor_key=p_actor_key END
  ORDER BY b.tenant_id,b.store_id,b.id FOR NO KEY UPDATE;

 UPDATE claims.lines l SET applied_version=NULL FROM claims.bundles b
  WHERE l.tenant_id=b.tenant_id AND l.store_id=b.store_id AND l.bundle_id=b.id AND b.purged_at IS NULL AND
   CASE WHEN v_single THEN b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=p_bundle
    ELSE b.platform=p_platform AND b.actor_key=p_actor_key END;
 GET DIAGNOSTICS n_lines=ROW_COUNT;
 DELETE FROM claims.links k USING claims.bundles b
  WHERE k.tenant_id=b.tenant_id AND k.store_id=b.store_id AND k.bundle_id=b.id AND b.purged_at IS NULL AND
   CASE WHEN v_single THEN b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=p_bundle
    ELSE b.platform=p_platform AND b.actor_key=p_actor_key END;
 GET DIAGNOSTICS n_links=ROW_COUNT;

 IF NOT v_single THEN
  -- Every add/edit/remove of the actor's comments: comment_key is shared by the events of one comment, and the actor's
  -- intake rows name one event of each comment (a comment id maps to one sender).
  FOR r IN SELECT ce.event_id FROM social.comment_events ce WHERE (ce.tenant_id,ce.store_id,ce.comment_key) IN (
    SELECT e.tenant_id,e.store_id,e.comment_key FROM social.comment_events e WHERE e.event_id IN (
     SELECT i.inbox_event_id FROM claims.meta_intake i WHERE i.actor_key=p_actor_key)) LOOP
   IF meta_inbox.lock_purgeable(r.event_id) THEN
    DELETE FROM social.comment_events ce WHERE ce.event_id=r.event_id;
    n_comments:=n_comments+1;
   ELSE
    n_defer:=n_defer+1;
   END IF;
  END LOOP;
  -- Capture the actor's comment refs BEFORE the intake delete: the public_reply C4 redaction below joins on them.
  SELECT array_agg(DISTINCT m.comment_ref) INTO v_comments FROM claims.meta_intake m WHERE m.actor_key=p_actor_key;
  -- replay (restore): a PENDING row of an erased actor goes too, else the consumer would re-claim and reply to them.
  DELETE FROM claims.meta_intake m WHERE m.actor_key=p_actor_key AND (m.state<>'PENDING' OR v_replay);
  GET DIAGNOSTICS n_intake=ROW_COUNT;
  -- C4: redact the ids of the four send actions bound to the actor; non-terminal ones were a hold (RD5) or, in replay,
  -- are redacted too and keep their state: the adapter then fails pre-send (errBadRequest, zero calls) and the
  -- dispatcher records UNKNOWN. offer_recommend carries no person id, so it is never eligible.
  n_ops:=0;
  UPDATE integration.operations o SET request=(o.request - 'comment_ref' - 'conversation_id' - 'peer_key') || '{"redacted":true}'::jsonb,
    semantic_key='mpr-purged:'||o.id::text, updated_at=v_now FROM claims.bundles b
   WHERE o.action='meta.private_reply' AND (v_replay OR o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING'))
    AND o.request ? 'comment_ref' AND o.tenant_id=b.tenant_id AND o.store_id=b.store_id
    AND o.request->>'bundle_id'=b.id::text AND b.purged_at IS NULL AND b.platform=p_platform AND b.actor_key=p_actor_key;
  GET DIAGNOSTICS v_n=ROW_COUNT; n_ops:=n_ops+v_n;
  UPDATE integration.operations o SET request=(o.request - 'comment_ref' - 'conversation_id' - 'peer_key') || '{"redacted":true}'::jsonb,
    semantic_key='mdm-purged:'||o.id::text, updated_at=v_now
   WHERE o.action='meta.dm_send' AND (v_replay OR o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING'))
    AND o.request ? 'peer_key' AND o.request->>'peer_key'=ANY(p_peer_keys);
  GET DIAGNOSTICS v_n=ROW_COUNT; n_ops:=n_ops+v_n;
  UPDATE integration.operations o SET request=(o.request - 'comment_ref' - 'conversation_id' - 'peer_key') || '{"redacted":true}'::jsonb,
    semantic_key='mpub-purged:'||o.id::text, updated_at=v_now
   WHERE o.action='meta.public_reply' AND (v_replay OR o.state IN ('SUCCEEDED','FAILED_FINAL','CANCELLED','BLOCKED_POLICY','STALE_BINDING'))
    AND o.request ? 'comment_ref' AND o.request->>'comment_ref'=ANY(v_comments);
  GET DIAGNOSTICS v_n=ROW_COUNT; n_ops:=n_ops+v_n;
  IF p_peer_keys IS NOT NULL AND cardinality(p_peer_keys)>0 THEN
   PERFORM 1 FROM social.conversations c WHERE c.peer_key=ANY(p_peer_keys) ORDER BY c.id FOR UPDATE;
   FOR r IN SELECT m.event_id FROM social.messages m JOIN social.conversations c ON c.id=m.conversation_id
     WHERE c.peer_key=ANY(p_peer_keys) LOOP
    IF meta_inbox.lock_purgeable(r.event_id) THEN
     DELETE FROM social.messages m WHERE m.event_id=r.event_id;
     n_messages:=n_messages+1;
    ELSE
     n_defer:=n_defer+1;
    END IF;
   END LOOP;
   DELETE FROM social.conversations c WHERE c.peer_key=ANY(p_peer_keys)
    AND NOT EXISTS(SELECT 1 FROM social.messages m WHERE m.conversation_id=c.id);
   GET DIAGNOSTICS n_conv=ROW_COUNT;
   -- C3x (0169 LC-R2, live-console-v1 §10 RD4): the peer's DM peer links go with the actor, peer_key-scoped like the
   -- conversations above and COUNTED, so an actor whose only remaining record is a peer link is erased, not PT404.
   DELETE FROM inbox.bundle_peers bp WHERE bp.peer_key=ANY(p_peer_keys);
   GET DIAGNOSTICS n_peers=ROW_COUNT;
  END IF;
 END IF;

 IF NOT v_single THEN
  -- 0154 W3-05B: the merchant's restricted-buyer entry (and its internal note) names this actor_key; erasure of the person removes it and COUNTS it,
  -- so an actor whose only remaining record is the entry (the state after age retention) is erased, not reported as not found.
  DELETE FROM claims.blocked_actors d WHERE d.actor_key=p_actor_key;
  GET DIAGNOSTICS v_n=ROW_COUNT;
 ELSE
  v_n:=0;
 END IF;
 -- RD1: random key/label (never derived from the bundle id), binding cleared, purged_at set; label only for manual.
 UPDATE claims.bundles b SET
   actor_key=replace(gen_random_uuid()::text,'-','')||replace(gen_random_uuid()::text,'-',''),
   label=CASE WHEN b.platform='manual' THEN 'erased-'||replace(gen_random_uuid()::text,'-','') END,
   owner_id=NULL, bound_at=NULL, purged_at=v_now, updated_at=v_now
  WHERE b.purged_at IS NULL AND
   CASE WHEN v_single THEN b.tenant_id=p_tenant AND b.store_id=p_store AND b.id=p_bundle
    ELSE b.platform=p_platform AND b.actor_key=p_actor_key END;
 GET DIAGNOSTICS n_bundles=ROW_COUNT;
 RETURN jsonb_build_object('bundles',n_bundles,'lines',n_lines,'links',n_links,'intake',n_intake,
  'comment_events',n_comments,'operations',n_ops,'messages',n_messages,'conversations',n_conv)
  || CASE WHEN n_defer>0 THEN jsonb_build_object('social_deferred',n_defer) ELSE '{}'::jsonb END
  || CASE WHEN v_n>0 THEN jsonb_build_object('blocked_actors',v_n) ELSE '{}'::jsonb END
  || CASE WHEN n_peers>0 THEN jsonb_build_object('bundle_peers',n_peers) ELSE '{}'::jsonb END;
END $$;
ALTER FUNCTION claims.apply_actor_erasure(text,text,uuid,uuid,uuid,text[]) OWNER TO commerce_retention_writer;
REVOKE ALL ON FUNCTION claims.apply_actor_erasure(text,text,uuid,uuid,uuid,text[]) FROM PUBLIC;

COMMENT ON FUNCTION claims.run_retention(integer) IS
 'internal/retention (Worker.Work on commerce_retention_job, RunOnce on commerce_retention_operator). One batch of the hourly purge, C1-C6 of the contract plus C3x (0169 LC-R2), <= p_limit rows per class, SKIP LOCKED, advisory key hashtextextended(''claims-retention'',0) (busy => {"busy":1}). C4 (0127 LC-R1) redacts comment_ref/conversation_id/peer_key and renames semantic_key to <prefix>-purged:<id> for the four send actions that are terminal, or UNKNOWN with an expired/no lease; request_hash is kept. C3x (0169 LC-R2, live-console-v1 §10) DELETEs live.comment_prints (first_printed_at), the live.comment.print receipts in ops.command_results (created_at) and inbox.bundle_peers (created_at) at intake_days, and the C2 loop deletes a de-identified bundle''s peer links. Report-only unless claims.retention_policy.enforced (report-only never returns more=1). Returns and logs numeric counts only. Non-goals: erasing one actor, choosing rows by caller input.';
COMMENT ON FUNCTION claims.apply_actor_erasure(text,text,uuid,uuid,uuid,text[]) IS
 'internal/retention internal helper of erase_actor and replay_actor_erasures; no EXECUTE grant. Idempotent RD1 de-identification plus deletion of the actor''s links, intake, comment events, and (0127 LC-R1) redaction of the four send-action operations bound to the actor (private_reply via its bundle, dm_send via peer_key, public_reply via the actor''s comment refs; offer_recommend carries no person id), plus (peer keys) conversations, (0154 W3-05B) the restricted-buyer entry, and (0169 LC-R2, live-console-v1 §10 RD4) the peer''s inbox.bundle_peers rows. Never applies the RD5 hold; in replay mode (lc.retention_replay=on, set only by replay_actor_erasures) it also deletes PENDING intake rows and redacts non-terminal send operations.';
