-- Purpose: native privacy erasure seals indexed OA1 intents before PII redaction.
-- Depends on: 0176 subject barriers, 0178 protected xid witness, 0180 absence/release.
-- Used by: authenticated customers.erase_owner and private restore replay only.
-- Invariants: all subject barriers -> all attempts -> all owners -> domains;
-- no plan decryption, key/principal/scope locks, privacy ops SELECT or compensation.

CREATE FUNCTION buyer.order_attempt_lock_erasure_owners(p_subjects jsonb,p_attempts jsonb)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE entry jsonb; ctx jsonb; owner_row record; owners jsonb:=p_subjects; missing boolean:=false;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
BEGIN
 IF p_subjects IS NULL OR jsonb_typeof(p_subjects)<>'array'
  OR p_attempts IS NULL OR jsonb_typeof(p_attempts)<>'array' THEN
  RAISE EXCEPTION 'invalid_erasure_subjects' USING ERRCODE='PT400';
 END IF;
 -- The coordinator supplies native subjects, never a caller-selected owner list.
 -- Every additional owner is derived through the exact protected xid accessor.
 FOR entry IN SELECT value FROM jsonb_array_elements(p_attempts) LOOP
  ctx:=ops.order_attempt_subject_context((entry->>'attempt_id')::uuid,(entry->>'generation')::bigint);
  IF ctx->>'tenant_id' IS DISTINCT FROM entry->>'tenant_id'
   OR ctx->>'store_id' IS DISTINCT FROM entry->>'store_id' THEN
   RAISE EXCEPTION 'attempt_not_fenced' USING ERRCODE='PT409';
  END IF;
  owners:=owners||coalesce((SELECT jsonb_agg(jsonb_build_object('tenant_id',ctx->>'tenant_id',
   'store_id',ctx->>'store_id','subject_id',value)) FROM jsonb_array_elements_text(ctx->'owner_ids')),'[]'::jsonb);
 END LOOP;
 FOR owner_row IN SELECT DISTINCT (value->>'tenant_id')::uuid AS tenant_id,
   (value->>'store_id')::uuid AS store_id,(value->>'subject_id')::uuid AS owner_id
  FROM jsonb_array_elements(owners) ORDER BY owner_id,tenant_id,store_id LOOP
  IF owner_row.tenant_id IS NULL OR owner_row.store_id IS NULL OR owner_row.owner_id IS NULL THEN
   RAISE EXCEPTION 'invalid_erasure_subjects' USING ERRCODE='PT400';
  END IF;
  PERFORM set_config('app.tenant_id',owner_row.tenant_id::text,true),set_config('app.store_id',owner_row.store_id::text,true);
  PERFORM 1 FROM buyer.owners o WHERE o.tenant_id=owner_row.tenant_id
   AND o.store_id=owner_row.store_id AND o.id=owner_row.owner_id FOR UPDATE;
  IF NOT FOUND THEN missing:=true; END IF;
 END LOOP;
 IF missing THEN RAISE EXCEPTION 'customer not found' USING ERRCODE='PT404'; END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true);
 RAISE;
END $$;
ALTER FUNCTION buyer.order_attempt_lock_erasure_owners(jsonb,jsonb) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.order_attempt_lock_erasure_owners(jsonb,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.order_attempt_lock_erasure_owners(jsonb,jsonb) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION buyer.order_attempt_lock_erasure_owners(jsonb,jsonb) IS
 'Private erasure union-owner prefix: scoped native targets and every current-xid indexed owner, all FOR UPDATE in UUID order before domain work. Exact coordinator caller only; no identity creation, earlier locks, PII mutation or ops SELECT.';

CREATE FUNCTION ops.order_attempt_erase_batch_prefix(p_subjects jsonb)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE entry jsonb; subject record; candidate record; a ops.order_attempts%ROWTYPE; ctx jsonb;
 subjects jsonb:='[]'; candidates jsonb:='[]'; attempts jsonb:='[]';
 prior text[]:=ARRAY[current_setting('app.tenant_id',true),current_setting('app.store_id',true),
  current_setting('app.principal_id',true),current_setting('app.buyer_id',true),
  current_setting('app.buyer_session_id',true),current_setting('app.order_attempt_id',true)];
BEGIN
 IF p_subjects IS NULL OR jsonb_typeof(p_subjects)<>'array' THEN
  RAISE EXCEPTION 'invalid_erasure_subjects' USING ERRCODE='PT400';
 END IF;
 IF jsonb_array_length(p_subjects)=0 THEN RAISE EXCEPTION 'invalid_erasure_subjects' USING ERRCODE='PT400'; END IF;
 -- Single-owner callers are checked against authenticated GUCs by the wrapper.
 -- Restore callers discover tuples from native owners/tombstones in their private
 -- definer. Normalize every scoped tuple before any locks; buyer revalidates the
 -- actual native row after ALL attempts, so a stale/mismatched tuple rolls back.
 BEGIN
  FOR entry IN SELECT value FROM jsonb_array_elements(p_subjects) LOOP
   IF jsonb_typeof(entry)<>'object' OR (entry->>'tenant_id')::uuid IS NULL
    OR (entry->>'store_id')::uuid IS NULL OR (entry->>'subject_id')::uuid IS NULL THEN
    RAISE EXCEPTION 'invalid_erasure_subjects' USING ERRCODE='PT400';
   END IF;
   subjects:=subjects||jsonb_build_array(jsonb_build_object('tenant_id',(entry->>'tenant_id')::uuid,
    'store_id',(entry->>'store_id')::uuid,'subject_id',(entry->>'subject_id')::uuid));
  END LOOP;
 EXCEPTION WHEN invalid_text_representation THEN
  RAISE EXCEPTION 'invalid_erasure_subjects' USING ERRCODE='PT400';
 END;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM subjects->0->>'tenant_id'
  OR current_setting('app.store_id',true) IS DISTINCT FROM subjects->0->>'store_id' THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 FOR subject IN SELECT DISTINCT (x.value->>'tenant_id')::uuid AS tenant_id,
   (x.value->>'store_id')::uuid AS store_id,k.kind,(x.value->>'subject_id')::uuid AS subject_id
  FROM jsonb_array_elements(subjects) x CROSS JOIN
   unnest(ARRAY['buyer_owner','customer','private_owner']::text[]) k(kind)
  ORDER BY tenant_id,store_id,kind,subject_id LOOP
  PERFORM ops.lock_order_attempt_subjects(subject.tenant_id,subject.store_id,
   jsonb_build_array(jsonb_build_object('kind',subject.kind,'id',subject.subject_id)));
 END LOOP;
 -- Discovery is unlocked. The barrier prevents a newly admitted indexed subject;
 -- each actual candidate is re-read under its native row lock below.
 FOR entry IN SELECT DISTINCT value FROM jsonb_array_elements(subjects) LOOP
  PERFORM set_config('app.tenant_id',entry->>'tenant_id',true),set_config('app.store_id',entry->>'store_id',true);
  SELECT candidates||coalesce(jsonb_agg(jsonb_build_object('attempt_id',x.attempt_id,
   'tenant_id',x.tenant_id,'store_id',x.store_id)),'[]'::jsonb) INTO candidates
  FROM ops.order_attempts x WHERE x.tenant_id=(entry->>'tenant_id')::uuid
   AND x.store_id=(entry->>'store_id')::uuid AND x.state='PENDING'
   AND EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.tenant_id=x.tenant_id
    AND s.store_id=x.store_id AND s.attempt_id=x.attempt_id
    AND s.subject_id=(entry->>'subject_id')::uuid);
 END LOOP;
 FOR candidate IN SELECT value FROM (SELECT DISTINCT value FROM jsonb_array_elements(candidates)) unique_candidates
  ORDER BY (value->>'attempt_id')::uuid LOOP
  PERFORM set_config('app.tenant_id',candidate.value->>'tenant_id',true),
   set_config('app.store_id',candidate.value->>'store_id',true);
  SELECT x.* INTO a FROM ops.order_attempts x WHERE x.attempt_id=(candidate.value->>'attempt_id')::uuid
   AND x.tenant_id=(candidate.value->>'tenant_id')::uuid AND x.store_id=(candidate.value->>'store_id')::uuid
   AND x.state='PENDING' FOR UPDATE;
  IF NOT FOUND THEN CONTINUE; END IF; -- A preceding C2 may have committed while we waited.
  IF NOT a.admission_finished THEN RAISE EXCEPTION 'attempt_not_admitting' USING ERRCODE='PT409'; END IF;
  INSERT INTO ops.order_attempt_execution_fences(attempt_id,tenant_id,store_id,execution_xid)
   VALUES(a.attempt_id,a.tenant_id,a.store_id,pg_current_xact_id())
   ON CONFLICT(attempt_id) DO UPDATE SET execution_xid=EXCLUDED.execution_xid;
  attempts:=attempts||jsonb_build_array(jsonb_build_object('attempt_id',a.attempt_id,
   'generation',a.intent_generation,'tenant_id',a.tenant_id,'store_id',a.store_id));
 END LOOP;
 PERFORM buyer.order_attempt_lock_erasure_owners(subjects,attempts);
 -- Private privacy-only caller + locked native target + indexed exact attempt +
 -- protected xid is the eraser proof. It intentionally precedes owner deactivation;
 -- ordinary public abort still requires an already-erased owner for ERASED.
 FOR entry IN SELECT value FROM jsonb_array_elements(attempts) ORDER BY (value->>'attempt_id')::uuid LOOP
  ctx:=ops.order_attempt_fence_context((entry->>'attempt_id')::uuid,(entry->>'generation')::bigint);
  PERFORM buyer.order_attempt_assert_no_effects((entry->>'attempt_id')::uuid,(entry->>'generation')::bigint);
  PERFORM checkout.order_attempt_assert_no_effects((entry->>'attempt_id')::uuid,(entry->>'generation')::bigint);
  PERFORM claims.order_attempt_release_admission((entry->>'attempt_id')::uuid,(entry->>'generation')::bigint);
  PERFORM ops.order_attempt_fence_context((entry->>'attempt_id')::uuid,(entry->>'generation')::bigint);
  PERFORM set_config('app.tenant_id',ctx->>'tenant_id',true),set_config('app.store_id',ctx->>'store_id',true);
  UPDATE ops.order_attempts x SET state='ABORTED',settled_at=clock_timestamp(),
   receipt=jsonb_build_object('state','ABORTED','attempt_id',x.attempt_id,'intent_generation',x.intent_generation,'code','ERASED'),
   plan_ciphertext=NULL,plan_key_version=NULL
   WHERE x.attempt_id=(entry->>'attempt_id')::uuid AND x.intent_generation=(entry->>'generation')::bigint
    AND x.tenant_id=(ctx->>'tenant_id')::uuid AND x.store_id=(ctx->>'store_id')::uuid
    AND x.state='PENDING' AND x.admission_finished;
  IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_fenced' USING ERRCODE='PT409'; END IF;
 END LOOP;
 PERFORM set_config('app.tenant_id',coalesce(prior[1],''),true),set_config('app.store_id',coalesce(prior[2],''),true),
  set_config('app.principal_id',coalesce(prior[3],''),true),set_config('app.buyer_id',coalesce(prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(prior[5],''),true),set_config('app.order_attempt_id',coalesce(prior[6],''),true);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior[1],''),true),set_config('app.store_id',coalesce(prior[2],''),true),
  set_config('app.principal_id',coalesce(prior[3],''),true),set_config('app.buyer_id',coalesce(prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(prior[5],''),true),set_config('app.order_attempt_id',coalesce(prior[6],''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.order_attempt_erase_batch_prefix(jsonb) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.order_attempt_erase_batch_prefix(jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.order_attempt_erase_batch_prefix(jsonb) TO commerce_privacy_writer;
COMMENT ON FUNCTION ops.order_attempt_erase_batch_prefix(jsonb) IS
 'Private privacy restore transaction prefix: native scoped tuple discovery, globally sorted subject barriers/attempts/union owners, protected xid, zero-effects and admission release, then ABORTED ERASED/body shredding before native redaction. Privacy EXECUTE only; no ops SELECT, key/scope/principal locks or changed COMMITTED proof.';

CREATE FUNCTION ops.order_attempt_erase_prefix(p_tenant uuid,p_store uuid,p_subject uuid)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_tenant IS NULL OR p_store IS NULL OR p_subject IS NULL THEN
  RAISE EXCEPTION 'invalid_erasure_subjects' USING ERRCODE='PT400';
 END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM p_tenant::text
  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 PERFORM ops.order_attempt_erase_batch_prefix(jsonb_build_array(jsonb_build_object(
  'tenant_id',p_tenant,'store_id',p_store,'subject_id',p_subject)));
END $$;
ALTER FUNCTION ops.order_attempt_erase_prefix(uuid,uuid,uuid) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.order_attempt_erase_prefix(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.order_attempt_erase_prefix(uuid,uuid,uuid) TO commerce_privacy_writer;
COMMENT ON FUNCTION ops.order_attempt_erase_prefix(uuid,uuid,uuid) IS
 'Private authenticated eraser proof: exact tenant/store GUCs, native owner revalidation and indexed pending attempts under the global privacy prefix; seal ERASED before redaction. Exact privacy-writer EXECUTE only, no ops reads or public abort authority expansion.';

-- Patch the current native entry, preserving final authentication, CD7 collection
-- guards and all later apply_erasure Ads/tag/import/claims hooks. Refuse drift.
DO $$ DECLARE body text; anchor text; BEGIN
 body:=pg_get_functiondef('customers.erase_owner(bytea,uuid,uuid,text,uuid)'::regprocedure);
 anchor:='  PERFORM 1 FROM buyer.owners o WHERE o.tenant_id=v_tenant AND o.store_id=p_store AND o.id=v_owner FOR UPDATE;';
 IF (length(body)-length(replace(body,anchor,'')))/length(anchor)<>1 THEN RAISE EXCEPTION 'erase_owner merchant prefix drift'; END IF;
 body:=replace(body,anchor,E'  -- OA1: authenticated subject/attempt/union-owner prefix before native owner/domain work.\n  PERFORM ops.order_attempt_erase_prefix(v_tenant,p_store,v_owner);\n'||anchor);
 anchor:='  PERFORM 1 FROM buyer.owners o WHERE o.tenant_id=v_sess.tenant_id AND o.store_id=p_store AND o.id=v_sess.owner_id FOR UPDATE;';
 IF (length(body)-length(replace(body,anchor,'')))/length(anchor)<>1 THEN RAISE EXCEPTION 'erase_owner buyer prefix drift'; END IF;
 body:=replace(body,anchor,E'  -- OA1: retain native session recheck after this exact private erasure prefix.\n  PERFORM ops.order_attempt_erase_prefix(v_sess.tenant_id,p_store,v_sess.owner_id);\n'||anchor);
 EXECUTE body;
END $$;

DO $$ DECLARE body text; anchor text; BEGIN
 body:=pg_get_functiondef('customers.replay_erasures(uuid[])'::regprocedure);
 anchor:='DECLARE r record; v_count integer:=0; v_summary jsonb;';
 IF (length(body)-length(replace(body,anchor,'')))/length(anchor)<>1 THEN RAISE EXCEPTION 'replay_erasures declaration drift'; END IF;
 body:=replace(body,anchor,anchor||E'\n v_subjects jsonb;');
 anchor:=E'BEGIN\n FOR r IN';
 IF (length(body)-length(replace(body,anchor,'')))/length(anchor)<>1 THEN RAISE EXCEPTION 'replay_erasures prefix drift'; END IF;
 body:=replace(body,anchor,E'BEGIN\n -- OA1 restore discovery is unlocked; acquire the entire subject/attempt/owner\n -- prefix once before any owner/domain mutation, then native scoped revalidation.\n -- This private restore definer supplies native scoped tuples, not runtime input.\n SELECT jsonb_agg(jsonb_build_object(''tenant_id'',x.tenant_id,''store_id'',x.store_id,''subject_id'',x.owner_id))\n INTO v_subjects FROM (\n  SELECT o.tenant_id,o.store_id,o.id AS owner_id FROM buyer.owners o WHERE p_owners IS NOT NULL AND o.id=ANY(p_owners)\n  UNION SELECT a.tenant_id,a.store_id,a.owner_id FROM customers.privacy_actions a WHERE p_owners IS NULL AND a.kind=''ERASURE''\n ) x;\n IF v_subjects IS NOT NULL THEN\n  PERFORM set_config(''app.tenant_id'',v_subjects->0->>''tenant_id'',true),\n   set_config(''app.store_id'',v_subjects->0->>''store_id'',true);\n  PERFORM ops.order_attempt_erase_batch_prefix(v_subjects);\n END IF;\n FOR r IN');
 anchor:=E'  SELECT o.tenant_id,o.store_id,o.id AS owner_id FROM buyer.owners o WHERE p_owners IS NOT NULL AND o.id=ANY(p_owners)\n  UNION\n  SELECT a.tenant_id,a.store_id,a.owner_id FROM customers.privacy_actions a WHERE p_owners IS NULL AND a.kind=''ERASURE''\n  ORDER BY 3';
 IF (length(body)-length(replace(body,anchor,'')))/length(anchor)<>1 THEN RAISE EXCEPTION 'replay_erasures discovery drift'; END IF;
 body:=replace(body,anchor,E'  SELECT (value->>''tenant_id'')::uuid AS tenant_id,(value->>''store_id'')::uuid AS store_id,\n   (value->>''subject_id'')::uuid AS owner_id FROM jsonb_array_elements(coalesce(v_subjects,''[]''::jsonb))\n  ORDER BY 3');
 EXECUTE body;
END $$;
