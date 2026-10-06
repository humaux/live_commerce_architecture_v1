-- Purpose: W6-05B merchant failed/UNKNOWN operations ledger: read model + three audited actions (query, cancel, retry) over integration.operations.
-- Depends on: 0008/0096 (integration.operations, claim/complete, operation_lane), 0035 (integration_writer read policies), 0128 (inbox.outbound_messages), river.river_job
--   (SELECT for commerce_integration_writer, post_river/0014), identity.resolve_access, contracts/external-operation-v1.md "Amendment W6-05B".
-- Used by: internal/integrations/core/ledger.go (cmd/api merchant routes, internal/httpapi/operations.go); internal/integrations/core/dispatcher.go reads generation_floor.
-- Invariants: I06/I07 (UNKNOWN is never blindly retried; a duplicate effect is impossible), contracts/invariants.json; one reviewed transition set, no new provider client.

-- ---------------------------------------------------------------------------------------------------
-- Reconcile/dispatch budget floor (external-dispatcher-v1 budget amendment). The dispatcher counts claimed generations as
-- generation - generation_floor, so an operator `query` or `retry` can give an exhausted (or re-opened) operation a fresh bounded budget
-- without ever lowering the monotonic generation that fences leases. DEFAULT 0 keeps every existing operation unchanged.
-- ---------------------------------------------------------------------------------------------------
ALTER TABLE integration.operations ADD COLUMN generation_floor bigint NOT NULL DEFAULT 0;
ALTER TABLE integration.operations ADD CONSTRAINT operation_generation_floor CHECK (generation_floor>=0 AND generation_floor<=generation);
GRANT UPDATE(generation_floor) ON integration.operations TO commerce_integration_writer;

-- The default ledger list (attention states, newest first) per store without scanning the (much larger) SUCCEEDED history.
CREATE INDEX operations_ledger_attention ON integration.operations(tenant_id,store_id,created_at DESC,id DESC)
 WHERE actor_kind='MERCHANT' AND state IN ('READY','UNKNOWN','ACKNOWLEDGED','FAILED_FINAL','BLOCKED_POLICY','STALE_BINDING');

-- ---------------------------------------------------------------------------------------------------
-- Private helpers (owner commerce_integration_writer, no EXECUTE for any login; called only inside the four public definers below).
-- Refusals: SQLSTATE PTnnn (nnn = HTTP status), MESSAGE = a frozen machine code that internal/integrations/core/ledger.go maps.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION integration.ledger_auth(p_hash bytea,p_store uuid,p_permission text) RETURNS uuid
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_permission IS NULL OR p_permission NOT IN ('integration:read','integration:execute')
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 SELECT * INTO a FROM identity.resolve_access(p_hash,p_store,p_permission);
 IF a.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF a.access_status='not_found' THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
 IF a.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 -- The authenticated session must be the one platform.WithScope pinned for this transaction (same fence as meta_connect_auth).
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM a.tenant_id::text OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM a.principal_id::text THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN a.tenant_id;
END $$;
ALTER FUNCTION integration.ledger_auth(bytea,uuid,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.ledger_auth(bytea,uuid,text) FROM PUBLIC;
COMMENT ON FUNCTION integration.ledger_auth(bytea,uuid,text) IS
 'integration owner; private ledger authentication (W6-05B): resolve_access for integration:read|execute, then the session must equal the app.* scope WithScope pinned. No caller EXECUTE; used by read_operation_ledger and ledger_open.';

-- Why an action is unavailable for this locked/read operation row ('' = available). The single source of truth for both the DTO flags
-- and the action refusals (contract Amendment W6-05B "Actions"). p_exclude_job is the job the caller just inserted in this transaction.
CREATE FUNCTION integration.ledger_capability(o integration.operations,p_kind text,p_exclude_job bigint) RETURNS text
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_now timestamptz:=clock_timestamp(); b integration.bindings%ROWTYPE; v_changed boolean; v_active boolean;
BEGIN
 IF o.actor_kind<>'MERCHANT' OR integration.operation_lane(o.provider,o.actor_kind) NOT IN ('default','ads') THEN RETURN 'lane_unsupported'; END IF;
 IF p_kind='cancel' THEN
  -- READY = no dispatch claim since it last became READY; DISPATCHING/UNKNOWN/ACKNOWLEDGED may already have an effect upstream.
  IF o.state='READY' THEN RETURN ''; END IF;
  RETURN CASE WHEN o.state IN ('DISPATCHING','UNKNOWN','ACKNOWLEDGED') THEN 'already_dispatched' ELSE 'operation_closed' END;
 END IF;
 -- The ads lane (meta_ads, meta_dataset) is read + cancel only: post_river/0015 guard_ads_job_link admits exactly one job per ads operation (the one in operations.job_id,
 -- an immutable column), so a follow-up query/retry job could not commit. Opening it would mean amending that guard (contract Amendment W6-05B "Scope").
 IF integration.operation_lane(o.provider,o.actor_kind)='ads' THEN RETURN 'lane_unsupported'; END IF;
 v_active:=o.lease_until IS NOT NULL AND o.lease_until>v_now;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=o.tenant_id AND x.store_id=o.store_id AND x.id=o.binding_id;
 v_changed:=NOT FOUND OR NOT b.enabled OR b.semantic_version<>o.binding_version OR b.provider<>o.provider OR b.external_asset_id<>o.external_asset_id;
 IF p_kind='query' THEN
  IF o.state NOT IN ('UNKNOWN','ACKNOWLEDGED','DISPATCHING') THEN RETURN 'not_in_doubt'; END IF;
  IF v_active THEN RETURN 'lease_active'; END IF;
  IF v_changed OR o.result_code='binding_changed' THEN RETURN 'binding_changed'; END IF;
  -- Another live external_operation_v1 job of this operation (River keeps snoozed/retrying jobs in these states).
  IF EXISTS(SELECT 1 FROM river.river_job j WHERE j.kind='external_operation_v1' AND j.args @> jsonb_build_object('operation_id',o.id::text)
   AND j.state IN ('available','scheduled','retryable','pending','running') AND j.id IS DISTINCT FROM p_exclude_job) THEN RETURN 'query_in_progress'; END IF;
  IF EXISTS(SELECT 1 FROM integration.operation_events e WHERE e.tenant_id=o.tenant_id AND e.store_id=o.store_id AND e.operation_id=o.id
   AND e.reason_code='query_requested' AND e.created_at>v_now-interval '60 seconds') THEN RETURN 'query_too_soon'; END IF;
  RETURN '';
 END IF;
 -- p_kind='retry'
 IF o.state='READY' THEN
  IF v_changed THEN RETURN 'binding_changed'; END IF;
  RETURN CASE WHEN EXISTS(SELECT 1 FROM river.river_job j WHERE j.kind='external_operation_v1' AND j.args @> jsonb_build_object('operation_id',o.id::text)
   AND j.state IN ('available','scheduled','retryable','pending','running') AND j.id IS DISTINCT FROM p_exclude_job) THEN 'already_queued' ELSE '' END;
 ELSIF o.state='FAILED_FINAL' THEN
  -- Retry registry (contract Amendment W6-05B): only read-only kinds whose Finish is an idempotent latest-wins upsert may be re-opened.
  IF (o.provider,o.action) NOT IN (('facebook','meta.live_videos'),('facebook','meta.live_insights')) THEN RETURN 'retry_not_supported'; END IF;
  RETURN CASE WHEN v_changed THEN 'binding_changed' ELSE '' END;
 ELSIF o.state IN ('DISPATCHING','UNKNOWN','ACKNOWLEDGED') THEN
  RETURN CASE WHEN v_active THEN 'lease_active' ELSE 'reconcile_first' END;
 ELSIF o.state='SUCCEEDED' THEN RETURN 'already_succeeded';
 END IF;
 RETURN 'operation_closed';
END $$;
ALTER FUNCTION integration.ledger_capability(integration.operations,text,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.ledger_capability(integration.operations,text,bigint) FROM PUBLIC;
COMMENT ON FUNCTION integration.ledger_capability(integration.operations,text,bigint) IS
 'integration owner; private (W6-05B): the reason code an action (query|cancel|retry) is unavailable for an operation row, or empty. Includes the retry registry (facebook meta.live_videos / meta.live_insights). No caller EXECUTE.';

-- Authenticates (integration:execute), locks binding FOR SHARE -> operation FOR UPDATE (the worker lock order), compare-and-swaps the caller's expected
-- generation, applies ledger_capability, and for query/retry verifies the River job the API inserted in this very transaction. Returns the locked row;
-- the row locks outlive this call inside the caller's transaction, so the public definers decide on exactly this state.
CREATE FUNCTION integration.ledger_open(p_hash bytea,p_store uuid,p_id uuid,p_expected bigint,p_kind text,p_job bigint) RETURNS integration.operations
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_binding uuid; b integration.bindings%ROWTYPE; o integration.operations; v_why text;
BEGIN
 IF p_id IS NULL OR p_expected IS NULL OR p_expected<0 OR p_kind IS NULL OR p_kind NOT IN ('query','cancel','retry')
  OR (p_kind='cancel')<>(p_job IS NULL) OR (p_job IS NOT NULL AND p_job<=0) THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 v_tenant:=integration.ledger_auth(p_hash,p_store,'integration:execute');
 -- Locator read; the immutable binding id is revalidated after the locks. Store scope is explicit: the writer role's policies are store-blind.
 SELECT x.binding_id INTO v_binding FROM integration.operations x WHERE x.tenant_id=v_tenant AND x.store_id=p_store AND x.id=p_id AND x.actor_kind='MERCHANT'
  AND integration.operation_lane(x.provider,x.actor_kind) IN ('default','ads');
 IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
 SELECT x.* INTO b FROM integration.bindings x WHERE x.tenant_id=v_tenant AND x.store_id=p_store AND x.id=v_binding FOR SHARE;
 SELECT x.* INTO o FROM integration.operations x WHERE x.tenant_id=v_tenant AND x.store_id=p_store AND x.id=p_id AND x.actor_kind='MERCHANT' FOR UPDATE;
 IF NOT FOUND OR o.binding_id<>b.id THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
 IF o.generation<>p_expected THEN RAISE EXCEPTION 'operation_changed' USING ERRCODE='PT409'; END IF;
 v_why:=integration.ledger_capability(o,p_kind,p_job);
 IF v_why<>'' THEN RAISE EXCEPTION '%',v_why USING ERRCODE='PT409'; END IF;
 IF p_kind<>'cancel' THEN
  -- The follow-up job rides the default lane exactly as Plan creates it (queue default, River default priority 1); the ads lane never reaches here (ledger_capability).
  PERFORM 1 FROM river.river_job r WHERE r.id=p_job AND r.kind='external_operation_v1' AND r.queue='default' AND r.priority=1 AND r.unique_key IS NULL
   AND r.args=jsonb_build_object('operation_id',o.id::text,'version',1) AND r.xmin=pg_current_xact_id()::xid;
  IF NOT FOUND THEN RAISE EXCEPTION 'invalid_job' USING ERRCODE='PT422'; END IF;
 END IF;
 RETURN o;
END $$;
ALTER FUNCTION integration.ledger_open(bytea,uuid,uuid,bigint,text,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.ledger_open(bytea,uuid,uuid,bigint,text,bigint) FROM PUBLIC;
COMMENT ON FUNCTION integration.ledger_open(bytea,uuid,uuid,bigint,text,bigint) IS
 'integration owner; private (W6-05B): auth + binding FOR SHARE -> operation FOR UPDATE + generation CAS + ledger_capability + same-transaction job check; returns the locked row. No caller EXECUTE.';

-- ---------------------------------------------------------------------------------------------------
-- Public definers (owner commerce_integration_writer, EXECUTE commerce_runtime only).
-- ---------------------------------------------------------------------------------------------------
-- Read: the list (p_id NULL) or one operation with its newest 50 events. Builds the exact DTO keys in SQL so no other column (request, semantic key,
-- asset id, provider reference, principal, lease) can reach the API layer. p_filter: attention|FAILED_FINAL|UNKNOWN|ACKNOWLEDGED|BLOCKED_POLICY|STALE_BINDING|READY.
CREATE FUNCTION integration.read_operation_ledger(p_hash bytea,p_store uuid,p_filter text,p_id uuid,p_after_at timestamptz,p_after_id uuid,p_limit integer) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; o integration.operations; v_rows integration.operations[]; v_items jsonb:='[]'::jsonb; v_events jsonb:='[]'::jsonb; v_more boolean:=false; v_n integer:=0;
 v_kind text; v_oid text; v_uuid constant text:='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
 v_actions jsonb; v_why text; v_name text;
BEGIN
 IF p_limit IS NULL OR p_limit<1 OR p_limit>100 OR (p_id IS NULL AND (p_filter IS NULL OR p_filter NOT IN ('attention','FAILED_FINAL','UNKNOWN','ACKNOWLEDGED','BLOCKED_POLICY','STALE_BINDING','READY')))
  OR (p_after_at IS NULL)<>(p_after_id IS NULL) OR (p_id IS NOT NULL AND p_after_at IS NOT NULL) THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 v_tenant:=integration.ledger_auth(p_hash,p_store,'integration:read');
 -- Detail by id (any visible state) or the list (partial-index states only, literal predicate so the planner can prove the index applies).
 IF p_id IS NOT NULL THEN
  SELECT array_agg(y.r) INTO v_rows FROM (SELECT x AS r FROM integration.operations x WHERE x.tenant_id=v_tenant AND x.store_id=p_store AND x.id=p_id
   AND x.actor_kind='MERCHANT' AND integration.operation_lane(x.provider,x.actor_kind) IN ('default','ads')) y;
 ELSE
  SELECT array_agg(y.r ORDER BY (y.r).created_at DESC,(y.r).id DESC) INTO v_rows FROM (SELECT x AS r FROM integration.operations x
   WHERE x.tenant_id=v_tenant AND x.store_id=p_store AND x.actor_kind='MERCHANT'
    AND x.state IN ('READY','UNKNOWN','ACKNOWLEDGED','FAILED_FINAL','BLOCKED_POLICY','STALE_BINDING')
    AND (p_filter='attention' OR x.state=p_filter)
    AND (p_after_at IS NULL OR (x.created_at,x.id)<(p_after_at,p_after_id))
    AND integration.operation_lane(x.provider,x.actor_kind) IN ('default','ads')
   ORDER BY x.created_at DESC,x.id DESC LIMIT p_limit+1) y;
 END IF;
 FOREACH o IN ARRAY coalesce(v_rows,ARRAY[]::integration.operations[]) LOOP
  v_n:=v_n+1;
  IF v_n>p_limit THEN v_more:=true; EXIT; END IF;
  -- Business object from internal ids only (contract Amendment W6-05B "Read model"); a DM resolves its conversation through the send projection.
  v_kind:=NULL; v_oid:=NULL;
  IF o.action='ecpay.cvs_create' THEN v_kind:='order'; v_oid:=o.request->>'order_id';
  ELSIF o.action IN ('meta.dm_send','meta.private_reply','meta.public_reply','meta.offer_recommend') THEN
   SELECT m.conversation_id::text INTO v_oid FROM inbox.outbound_messages m
    WHERE m.tenant_id=o.tenant_id AND m.store_id=o.store_id AND m.operation_id=o.id AND m.conversation_id IS NOT NULL;
   IF v_oid IS NOT NULL THEN v_kind:='conversation'; ELSE v_kind:='claim_bundle'; v_oid:=o.request->>'bundle_id'; END IF;
  ELSIF o.action LIKE 'meta.ads.%' THEN v_kind:='ad_draft'; v_oid:=o.request->>'draft_id';
  ELSIF o.action='meta.capi.purchase' THEN v_kind:='payment_attempt'; v_oid:=o.request->>'attempt_id';
  ELSIF o.action='meta.live_videos' THEN v_kind:='binding'; v_oid:=o.request->>'binding_id';
  ELSIF o.action='meta.live_insights' THEN v_kind:='live_session'; v_oid:=o.request->>'session_id';
  END IF;
  IF v_oid IS NULL OR v_oid !~ v_uuid THEN v_kind:=NULL; v_oid:=NULL; END IF;
  v_actions:='{}'::jsonb;
  FOREACH v_name IN ARRAY ARRAY['query','cancel','retry'] LOOP
   v_why:=integration.ledger_capability(o,v_name,NULL);
   v_actions:=v_actions||jsonb_build_object(v_name,jsonb_build_object('available',v_why='','reason',v_why));
  END LOOP;
  v_items:=v_items||jsonb_build_array(jsonb_build_object('operation_id',o.id,'provider',o.provider,'action',o.action,'purpose',o.purpose,'state',o.state,
   'reason_code',o.result_code,'attempts',o.generation,'created_at',o.created_at,'updated_at',o.updated_at,
   'object',CASE WHEN v_kind IS NULL THEN NULL ELSE jsonb_build_object('kind',v_kind,'id',v_oid) END,'actions',v_actions));
 END LOOP;
 IF p_id IS NOT NULL THEN
  IF v_n=0 THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
  SELECT coalesce(jsonb_agg(jsonb_build_object('generation',e.generation,'state',e.state,'reason_code',e.reason_code,'created_at',e.created_at) ORDER BY e.id DESC),'[]'::jsonb)
   INTO v_events FROM (SELECT y.* FROM integration.operation_events y WHERE y.tenant_id=v_tenant AND y.store_id=p_store AND y.operation_id=p_id ORDER BY y.id DESC LIMIT 50) e;
 END IF;
 RETURN jsonb_build_object('items',v_items,'has_more',v_more,'events',v_events);
END $$;
ALTER FUNCTION integration.read_operation_ledger(bytea,uuid,text,uuid,timestamptz,uuid,integer) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.read_operation_ledger(bytea,uuid,text,uuid,timestamptz,uuid,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.read_operation_ledger(bytea,uuid,text,uuid,timestamptz,uuid,integer) TO commerce_runtime;
COMMENT ON FUNCTION integration.read_operation_ledger(bytea,uuid,text,uuid,timestamptz,uuid,integer) IS
 'internal/integrations/core ledger read (W6-05B): fresh integration:read auth, store-scoped MERCHANT default/ads-lane operations, exact DTO keys built here (no request/semantic key/asset/provider reference/principal/lease), business object from internal ids, per-action availability from ledger_capability, newest 50 events for a single read. No provider call, no write.';

CREATE FUNCTION integration.request_operation_query(p_hash bytea,p_store uuid,p_id uuid,p_expected bigint,p_job bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations; v_now timestamptz:=clock_timestamp();
BEGIN
 IF p_job IS NULL THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 o:=integration.ledger_open(p_hash,p_store,p_id,p_expected,'query',p_job);
 -- State and generation are untouched: the new job's claim takes the usual reconcile lease (never dispatch). The raised floor restarts the bounded reconcile budget.
 UPDATE integration.operations x SET generation_floor=o.generation,updated_at=v_now WHERE x.id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation,o.state,'','query_requested');
 RETURN jsonb_build_object('operation_id',o.id,'state',o.state,'attempts',o.generation);
END $$;
ALTER FUNCTION integration.request_operation_query(bytea,uuid,uuid,bigint,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.request_operation_query(bytea,uuid,uuid,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.request_operation_query(bytea,uuid,uuid,bigint,bigint) TO commerce_runtime;
COMMENT ON FUNCTION integration.request_operation_query(bytea,uuid,uuid,bigint,bigint) IS
 'internal/integrations/core ledger query (W6-05B): integration:execute, UNKNOWN/ACKNOWLEDGED/expired-DISPATCHING only, no active lease or other live job, 60 s spacing; raises generation_floor and records query_requested. The same-transaction job (checked by ledger_open) can only be claimed in reconcile mode. No state change.';

CREATE FUNCTION integration.cancel_operation(p_hash bytea,p_store uuid,p_id uuid,p_expected bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations; v_now timestamptz:=clock_timestamp();
BEGIN
 o:=integration.ledger_open(p_hash,p_store,p_id,p_expected,'cancel',NULL);
 -- READY only (ledger_capability): never dispatched, so CANCELLED is a local fact, not a remote reversal. Like claim_operation's READY -> STALE_BINDING it takes the next
 -- generation (the table CHECK requires generation>0 outside READY, and every state change is fenced by a new generation).
 UPDATE integration.operations x SET state='CANCELLED',generation=o.generation+1,result_code='cancelled_by_merchant',updated_at=v_now WHERE x.id=o.id;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation+1,'CANCELLED','','cancelled_by_merchant');
 RETURN jsonb_build_object('operation_id',o.id,'state','CANCELLED','attempts',o.generation+1);
END $$;
ALTER FUNCTION integration.cancel_operation(bytea,uuid,uuid,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.cancel_operation(bytea,uuid,uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.cancel_operation(bytea,uuid,uuid,bigint) TO commerce_runtime;
COMMENT ON FUNCTION integration.cancel_operation(bytea,uuid,uuid,bigint) IS
 'internal/integrations/core ledger cancel (W6-05B): integration:execute, READY -> CANCELLED under binding FOR SHARE -> operation FOR UPDATE, so a concurrent dispatch claim and a cancel cannot both win. Refused for DISPATCHING/UNKNOWN/ACKNOWLEDGED (already_dispatched) and terminal states.';

CREATE FUNCTION integration.retry_operation(p_hash bytea,p_store uuid,p_id uuid,p_expected bigint,p_job bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations; v_now timestamptz:=clock_timestamp(); v_state text; v_reason text;
BEGIN
 IF p_job IS NULL THEN RAISE EXCEPTION 'invalid_request' USING ERRCODE='PT422'; END IF;
 o:=integration.ledger_open(p_hash,p_store,p_id,p_expected,'retry',p_job);
 IF o.state='FAILED_FINAL' THEN
  -- Re-open a registered read-only kind: same operation id, semantic key and lc:<operation_id> provider key; generation stays monotonic.
  v_state:='READY'; v_reason:='retry_authorized';
  UPDATE integration.operations x SET state='READY',generation_floor=o.generation,result_code='retry_authorized',updated_at=v_now WHERE x.id=o.id;
 ELSE
  -- READY without a live job (River exhausted its attempts): re-queue only; the operation was never claimed for dispatch.
  v_state:='READY'; v_reason:='requeue_authorized';
  UPDATE integration.operations x SET generation_floor=o.generation,updated_at=v_now WHERE x.id=o.id;
 END IF;
 INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code)
  VALUES(o.tenant_id,o.store_id,o.id,o.generation,v_state,'',v_reason);
 RETURN jsonb_build_object('operation_id',o.id,'state',v_state,'attempts',o.generation);
END $$;
ALTER FUNCTION integration.retry_operation(bytea,uuid,uuid,bigint,bigint) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.retry_operation(bytea,uuid,uuid,bigint,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.retry_operation(bytea,uuid,uuid,bigint,bigint) TO commerce_runtime;
COMMENT ON FUNCTION integration.retry_operation(bytea,uuid,uuid,bigint,bigint) IS
 'internal/integrations/core ledger retry (W6-05B): integration:execute. FAILED_FINAL -> READY only for the registered read-only kinds (ledger_capability); READY without a live job -> re-queue. UNKNOWN/ACKNOWLEDGED/expired DISPATCHING answer reconcile_first and are never re-opened. One same-transaction job (checked by ledger_open); no new semantic key.';
