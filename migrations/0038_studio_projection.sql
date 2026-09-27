-- Merchant-safe media facts. The writer already owns the private rows; callers gain
-- only this fixed, token-checked projection, never raw SELECT on those tables.
CREATE FUNCTION live.read_studio_media(p_auth_hash bytea, p_store uuid, p_session uuid)
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
   OR a.environment<>'MOCK' OR a.execution_profile<>'PROVIDER_MOCK'
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
