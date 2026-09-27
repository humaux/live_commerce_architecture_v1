-- Forward the Studio projection without changing its public DTO. Only marked
-- input attempts enter; the kernel-only profile still fails association checks.
-- Merchant-safe media facts. The writer already owns the private rows; callers gain
-- only this fixed, token-checked projection, never raw SELECT on those tables.
CREATE OR REPLACE FUNCTION live.read_studio_media(p_auth_hash bytea, p_store uuid, p_session uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 v_access record; v_final record; v_tenant uuid; v_principal uuid; v_revision bigint;
 v_expiry timestamptz; v_now timestamptz; v_program live.programs%ROWTYPE;
 h live.prepared_media_authorizations%ROWTYPE; a live.media_attempts%ROWTYPE;
 o integration.operations%ROWTYPE; x live.media_execution_state%ROWTYPE;
 v_prepared jsonb := NULL; v_attempt jsonb := NULL; v_destinations jsonb;
 v_count integer;
BEGIN
 IF p_auth_hash IS NULL OR octet_length(p_auth_hash)<>32 OR p_store IS NULL OR p_session IS NULL
  OR p_store='00000000-0000-0000-0000-000000000000'::uuid
  OR p_session='00000000-0000-0000-0000-000000000000'::uuid
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid studio read' USING ERRCODE='MP400'; END IF;
 SELECT * INTO v_access FROM identity.resolve_access(p_auth_hash,p_store,'live:read');
 IF v_access.access_status='unauthorized' THEN
  RAISE EXCEPTION 'studio access unavailable' USING ERRCODE='MP401'; END IF;
 IF v_access.access_status<>'ok' THEN
  RAISE EXCEPTION 'studio access unavailable' USING ERRCODE='MP403'; END IF;
 v_tenant:=v_access.tenant_id; v_principal:=v_access.principal_id;
 v_revision:=v_access.authz_revision;
 IF v_tenant IS NULL OR v_principal IS NULL OR v_revision IS NULL OR v_revision<1
  OR v_tenant IS DISTINCT FROM nullif(current_setting('app.tenant_id',true),'')::uuid
  OR p_store IS DISTINCT FROM nullif(current_setting('app.store_id',true),'')::uuid
  OR v_principal IS DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  OR v_revision IS DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint THEN
  RAISE EXCEPTION 'studio access unavailable' USING ERRCODE='MP403'; END IF;
 SELECT * INTO v_program FROM live.programs
  WHERE tenant_id=v_tenant AND store_id=p_store AND session_id=p_session;
 IF NOT FOUND OR NOT EXISTS (SELECT 1 FROM live.sessions
   WHERE tenant_id=v_tenant AND store_id=p_store AND id=p_session) THEN
  RAISE EXCEPTION 'studio session unavailable' USING ERRCODE='MP404'; END IF;
 SELECT * INTO a FROM live.media_attempts
  WHERE tenant_id=v_tenant AND store_id=p_store AND session_id=p_session;
 IF a.id IS NOT NULL THEN
  SELECT * INTO h FROM live.prepared_media_authorizations
   WHERE id=a.authorization_id AND tenant_id=v_tenant AND store_id=p_store AND session_id=p_session;
  SELECT * INTO o FROM integration.operations
   WHERE id=a.start_operation_id AND tenant_id=v_tenant AND store_id=p_store;
  IF h.id IS NULL OR h.attempt_id<>a.id OR a.program_id<>v_program.id
   OR a.environment<>'MOCK' OR (a.execution_profile<>'PROVIDER_MOCK'
    AND (a.execution_profile<>'LOCAL_SFU_MOCK_EGRESS' OR NOT EXISTS (
     SELECT 1 FROM live.media_input_custody i
     JOIN live.prepared_media_input_runtime_profiles r
      ON r.authorization_id=i.authorization_id AND r.tenant_id=i.tenant_id AND r.store_id=i.store_id
     WHERE i.attempt_id=a.id AND i.operation_id=o.id AND i.authorization_id=h.id
      AND i.tenant_id=v_tenant AND i.store_id=p_store AND i.session_id=p_session
      AND i.runtime_version=1 AND r.runtime_version=1)))
   OR o.id IS NULL OR o.actor_kind<>'MEDIA_ATTEMPT' OR o.action<>'livekit.egress.start'
   OR o.media_attempt_id<>a.id OR o.principal_id<>a.original_principal_id
   OR o.job_id<1 OR o.binding_id<>h.media_binding_id
   OR o.binding_version<>h.media_binding_version OR o.external_asset_id<>h.project_id THEN
   RAISE EXCEPTION 'studio association unavailable' USING ERRCODE='MP409'; END IF;
  SELECT * INTO x FROM live.media_execution_state
   WHERE attempt_id=a.id AND tenant_id=v_tenant AND store_id=p_store;
  IF x.attempt_id IS NOT NULL AND
   (x.tenant_id,x.store_id,x.session_id,x.authorization_id,x.operation_id,x.project_id)
   IS DISTINCT FROM (a.tenant_id,a.store_id,a.session_id,h.id,o.id,h.project_id) THEN
   RAISE EXCEPTION 'studio association unavailable' USING ERRCODE='MP409'; END IF;
  SELECT count(*), coalesce(jsonb_agg(jsonb_build_object('ordinal',d.ordinal,
   'provider',d.provider) ORDER BY d.ordinal),'[]'::jsonb)
   INTO v_count,v_destinations FROM live.media_authorization_destinations d
   WHERE d.authorization_id=h.id AND d.tenant_id=v_tenant AND d.store_id=p_store;
  IF v_count NOT BETWEEN 1 AND 2 THEN
   RAISE EXCEPTION 'studio destination unavailable' USING ERRCODE='MP409'; END IF;
  v_attempt:=jsonb_build_object('attempt_id',a.id::text,'environment','MOCK',
   'operation_state',o.state,'resource_state',coalesce(x.resource_state,'UNOBSERVED'),
   'transport_status',coalesce(x.transport_status,''),
   'cleanup_required',coalesce(x.cleanup_required,false),
   'stop_requested',x.stop_requested_at IS NOT NULL,
   'stop_wire_count',coalesce(x.stop_wire_count,0),
   'escalated',x.escalated_at IS NOT NULL,
   'updated_at',coalesce(x.updated_at,o.updated_at),
   'destinations',v_destinations);
 ELSE
  SELECT * INTO v_program FROM live.programs
   WHERE tenant_id=v_tenant AND store_id=p_store AND session_id=p_session;
  IF v_program.state='DRAFT'
   AND EXISTS (SELECT 1 FROM control.tenants WHERE id=v_tenant AND active)
   AND EXISTS (SELECT 1 FROM control.stores WHERE tenant_id=v_tenant AND id=p_store AND active) THEN
   -- A candidate is advisory. PlanStart repeats the checks under its own locks.
   FOR h IN SELECT h0.* FROM live.prepared_media_authorizations h0
    JOIN live.sessions s ON s.tenant_id=h0.tenant_id AND s.store_id=h0.store_id
     AND s.id=h0.session_id
    WHERE h0.tenant_id=v_tenant AND h0.store_id=p_store AND h0.session_id=p_session
     AND h0.environment='MOCK' AND h0.evidence_type='MOCK_FIXTURE'
     AND h0.session_version=s.version AND h0.aspect_ratio=v_program.aspect_ratio
     AND h0.start_before>clock_timestamp()
     AND NOT EXISTS (SELECT 1 FROM live.prepared_media_input_profiles ip
      WHERE ip.authorization_id=h0.id)
     AND NOT EXISTS (SELECT 1 FROM live.media_authorization_revocations r
      WHERE r.authorization_id=h0.id AND r.tenant_id=v_tenant AND r.store_id=p_store)
    ORDER BY h0.created_at DESC,h0.id DESC LOOP
    IF NOT EXISTS (SELECT 1 FROM integration.bindings b WHERE b.id=h.media_binding_id
     AND b.tenant_id=v_tenant AND b.store_id=p_store AND b.enabled
     AND b.provider='livekit' AND b.external_asset_id=h.project_id
     AND b.semantic_version=h.media_binding_version) THEN CONTINUE; END IF;
    SELECT count(*),coalesce(jsonb_agg(jsonb_build_object('ordinal',d.ordinal,
     'provider',d.provider) ORDER BY d.ordinal),'[]'::jsonb)
     INTO v_count,v_destinations FROM live.media_authorization_destinations d
     WHERE d.authorization_id=h.id AND d.tenant_id=v_tenant AND d.store_id=p_store
      AND EXISTS (SELECT 1 FROM integration.bindings b WHERE b.id=d.binding_id
       AND b.tenant_id=v_tenant AND b.store_id=p_store AND b.enabled
       AND b.provider=d.provider AND b.external_asset_id=d.external_asset_id
       AND b.semantic_version=d.binding_version);
    IF v_count NOT BETWEEN 1 AND 2 OR v_count<>(SELECT count(*)
     FROM live.media_authorization_destinations WHERE authorization_id=h.id) THEN CONTINUE; END IF;
    v_prepared:=jsonb_build_object('authorization_id',h.id::text,
     'session_version',h.session_version,'start_before',h.start_before,
     'environment','MOCK','destinations',v_destinations);
    EXIT;
   END LOOP;
  END IF;
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_auth_hash,p_store,'live:read');
 SELECT expires_at INTO v_expiry FROM identity.sessions
  WHERE token_hash=p_auth_hash AND audience='merchant' AND revoked_at IS NULL;
 v_now:=clock_timestamp();
 IF v_final.access_status='unauthorized' OR v_expiry IS NULL OR v_expiry<=v_now THEN
  RAISE EXCEPTION 'studio access unavailable' USING ERRCODE='MP401'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM v_tenant
  OR v_final.principal_id IS DISTINCT FROM v_principal
  OR v_final.authz_revision IS DISTINCT FROM v_revision THEN
  RAISE EXCEPTION 'studio access unavailable' USING ERRCODE='MP403'; END IF;
 IF v_prepared IS NOT NULL AND (v_prepared->>'start_before')::timestamptz<=v_now THEN
  v_prepared:=NULL; END IF;
 RETURN jsonb_build_object('prepared',v_prepared,'attempt',v_attempt);
END $$;
ALTER FUNCTION live.read_studio_media(bytea,uuid,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.read_studio_media(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.read_studio_media(bytea,uuid,uuid) TO commerce_runtime;

-- Read-only input liability. A missing execution row before the original job
-- claims is normal; the immutable custody row, not Egress, owns admission.
CREATE FUNCTION live.read_studio_input(p_auth_hash bytea,p_store uuid,p_session uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_media jsonb; a live.media_attempts%ROWTYPE; o integration.operations%ROWTYPE;
 i live.media_input_custody%ROWTYPE; x live.media_execution_state%ROWTYPE;
 v_manage record; v_can_stop boolean;
BEGIN
 v_media:=live.read_studio_media(p_auth_hash,p_store,p_session);
 SELECT * INTO a FROM live.media_attempts WHERE tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
  AND store_id=p_store AND session_id=p_session;
 IF a.id IS NULL OR a.execution_profile='PROVIDER_MOCK' THEN
  PERFORM live.read_studio_media(p_auth_hash,p_store,p_session);
  RETURN 'null'::jsonb; END IF;
 SELECT * INTO o FROM integration.operations WHERE id=a.start_operation_id;
 SELECT * INTO i FROM live.media_input_custody WHERE attempt_id=a.id;
 SELECT * INTO x FROM live.media_execution_state WHERE attempt_id=a.id;
 IF a.execution_profile<>'LOCAL_SFU_MOCK_EGRESS' OR o.id IS NULL OR i.attempt_id IS NULL
  OR i.runtime_version<>1 OR i.operation_id<>o.id OR i.authorization_id<>a.authorization_id
  OR (i.tenant_id,i.store_id,i.session_id) IS DISTINCT FROM (a.tenant_id,a.store_id,a.session_id)
  OR (x.attempt_id IS NOT NULL AND (x.tenant_id,x.store_id,x.session_id,x.authorization_id,x.operation_id)
   IS DISTINCT FROM (a.tenant_id,a.store_id,a.session_id,a.authorization_id,o.id)) THEN
  RAISE EXCEPTION 'studio input association unavailable' USING ERRCODE='MP409'; END IF;
 SELECT * INTO v_manage FROM identity.resolve_access(p_auth_hash,p_store,'live:manage');
 v_can_stop:=coalesce(v_manage.access_status='ok'
  AND v_manage.tenant_id IS NOT DISTINCT FROM a.tenant_id
  AND v_manage.principal_id IS NOT DISTINCT FROM nullif(current_setting('app.principal_id',true),'')::uuid
  AND v_manage.authz_revision IS NOT DISTINCT FROM nullif(current_setting('app.authz_revision',true),'')::bigint
  AND x.stop_requested_at IS NULL
  AND (i.state<>'CLOSED' OR (x.wire_reserved_at IS NOT NULL AND x.resource_state<>'TERMINAL')),false);
 -- The second projection is the final read-token/revision/session check. The
 -- result remains advisory; Stop takes its own locks and checks again.
 PERFORM live.read_studio_media(p_auth_hash,p_store,p_session);
 RETURN jsonb_build_object('attempt_id',a.id::text,'state',i.state,
  'admission_closed',i.admission_closed_at IS NOT NULL,'close_reason',i.close_reason,
  'cleanup_held',i.input_cleanup_held_at IS NOT NULL,'can_stop',v_can_stop,
  'updated_at',greatest(i.updated_at,o.updated_at,x.updated_at));
END $$;
ALTER FUNCTION live.read_studio_input(bytea,uuid,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.read_studio_input(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.read_studio_input(bytea,uuid,uuid) TO commerce_runtime;

-- Prepared selection is advisory and deliberately independent of /input's
-- exact seven-field status DTO. Private mapping pins are stripped by Go.
CREATE FUNCTION live.read_studio_input_prepared(p_auth_hash bytea,p_store uuid,p_session uuid)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_media jsonb; v_program live.programs%ROWTYPE;
 h live.prepared_media_authorizations%ROWTYPE; v_destinations jsonb;
 v_count integer; v_result jsonb := 'null'::jsonb; v_version bigint;
BEGIN
 v_media:=live.read_studio_media(p_auth_hash,p_store,p_session);
 IF v_media->'attempt' IS DISTINCT FROM 'null'::jsonb THEN
  PERFORM live.read_studio_media(p_auth_hash,p_store,p_session);
  RETURN 'null'::jsonb; END IF;
 SELECT * INTO v_program FROM live.programs
  WHERE tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=p_store AND session_id=p_session;
 IF v_program.id IS NULL OR v_program.state<>'DRAFT'
  OR NOT EXISTS(SELECT 1 FROM control.tenants WHERE id=v_program.tenant_id AND active)
  OR NOT EXISTS(SELECT 1 FROM control.stores WHERE tenant_id=v_program.tenant_id AND id=p_store AND active)
  OR EXISTS(SELECT 1 FROM live.media_attempts WHERE session_id=p_session) THEN
  PERFORM live.read_studio_media(p_auth_hash,p_store,p_session);
  RETURN 'null'::jsonb; END IF;
 FOR h IN SELECT h0.* FROM live.prepared_media_authorizations h0
  JOIN live.sessions s ON s.tenant_id=h0.tenant_id AND s.store_id=h0.store_id AND s.id=h0.session_id
  JOIN live.prepared_media_input_profiles ip ON ip.authorization_id=h0.id
   AND ip.tenant_id=h0.tenant_id AND ip.store_id=h0.store_id
  JOIN live.prepared_media_input_runtime_profiles rp ON rp.authorization_id=h0.id
   AND rp.tenant_id=h0.tenant_id AND rp.store_id=h0.store_id AND rp.runtime_version=1
  WHERE h0.tenant_id=v_program.tenant_id AND h0.store_id=p_store AND h0.session_id=p_session
   AND h0.environment='MOCK' AND h0.evidence_type='MOCK_FIXTURE'
   AND h0.session_version=s.version AND h0.aspect_ratio=v_program.aspect_ratio
   AND h0.start_before>clock_timestamp()
   AND NOT EXISTS(SELECT 1 FROM live.media_authorization_revocations r
    WHERE r.authorization_id=h0.id AND r.tenant_id=h0.tenant_id AND r.store_id=h0.store_id)
  ORDER BY h0.created_at DESC,h0.id DESC LOOP
  IF NOT EXISTS(SELECT 1 FROM integration.bindings b WHERE b.id=h.media_binding_id
   AND b.tenant_id=h.tenant_id AND b.store_id=p_store AND b.enabled
   AND b.provider='livekit' AND b.external_asset_id=h.project_id
   AND b.semantic_version=h.media_binding_version) THEN CONTINUE; END IF;
  SELECT count(*),coalesce(jsonb_agg(jsonb_build_object('ordinal',d.ordinal,
   'provider',d.provider) ORDER BY d.ordinal),'[]'::jsonb)
   INTO v_count,v_destinations FROM live.media_authorization_destinations d
   WHERE d.authorization_id=h.id AND d.tenant_id=h.tenant_id AND d.store_id=p_store
    AND EXISTS(SELECT 1 FROM integration.bindings b WHERE b.id=d.binding_id
     AND b.tenant_id=h.tenant_id AND b.store_id=p_store AND b.enabled
     AND b.provider=d.provider AND b.external_asset_id=d.external_asset_id
     AND b.semantic_version=d.binding_version);
  IF v_count NOT BETWEEN 1 AND 2 OR v_count<>(SELECT count(*)
   FROM live.media_authorization_destinations WHERE authorization_id=h.id) THEN CONTINUE; END IF;
  v_result:=jsonb_build_object('prepared',jsonb_build_object(
   'authorization_id',h.id::text,'session_version',h.session_version,
   'start_before',h.start_before,'environment','MOCK','destinations',v_destinations),
   'project_id',h.project_id,'credential_version',h.credential_version,
   'endpoint_identity',h.endpoint_identity);
  EXIT;
 END LOOP;
 -- Recheck authorization and current draft snapshot after candidate selection.
 PERFORM live.read_studio_media(p_auth_hash,p_store,p_session);
 IF v_result<>'null'::jsonb THEN
  SELECT s.version INTO v_version FROM live.sessions s
   JOIN live.programs p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.session_id=s.id
   WHERE s.id=p_session AND s.tenant_id=v_program.tenant_id AND s.store_id=p_store
    AND p.id=v_program.id AND p.state='DRAFT' AND p.aspect_ratio=v_program.aspect_ratio;
  IF v_version IS DISTINCT FROM h.session_version OR h.start_before<=clock_timestamp()
   OR EXISTS(SELECT 1 FROM live.media_authorization_revocations WHERE authorization_id=h.id)
   OR EXISTS(SELECT 1 FROM live.media_attempts WHERE session_id=p_session) THEN
   RETURN 'null'::jsonb; END IF;
 END IF;
 RETURN v_result;
END $$;
ALTER FUNCTION live.read_studio_input_prepared(bytea,uuid,uuid) OWNER TO commerce_media_writer;
REVOKE ALL ON FUNCTION live.read_studio_input_prepared(bytea,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION live.read_studio_input_prepared(bytea,uuid,uuid) TO commerce_runtime;
