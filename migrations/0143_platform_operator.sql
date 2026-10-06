-- Purpose: platform-operator suspend/resume of a tenant or store plus the append-only control.operator_audit, and the two outbound guards (claim-reply planning, mail claiming) that the existing active predicates do not cover.
-- Depends on: control.tenants/stores (0001), identity.principal_holds/resolve_access (0064/0089 active joins), integration.claim_reply_plannable (0128), notify.claim_batch (0098), ops.audit_events policies (0064), roles commerce_integration_writer/commerce_checkout_writer/commerce_expiry_worker.
-- Used by: cmd/platform-admin (commerce_platform_operator login), integration claims intake poller, expiry-worker mail loop, tests/foundation/platform_operator*_test.go.
-- Invariants: suspension never blocks money in flight (payment notify/query, refunds, expiry untouched); the audit is append-only for every role.
-- Status: REAL_PG gate.
-- 0143 platform operator: suspend/resume a merchant (tenant) or one store, with an append-only operator audit
-- (R3 unit OPS-01B; docs/delivery/units/ops-01b-store-suspend.md "Integrator 裁决"; covers deviations A8/B7/D9).
--
-- The platform operator console is CLI-only (cmd/platform-admin). It reuses the two flags every request already honours:
-- control.tenants.active and control.stores.active (0001). identity.resolve_access, identity.principal_holds,
-- buyer.resolve_scope / issue_capability and buyer.resolve_published_store (0001/0006/0020/0064) join them, so the merchant
-- API, every stored-principal send/ads Check, buyer capabilities and the published-store resolver stop at once. This
-- migration adds ONLY what those predicates do not already cover:
--   * control.set_store_active / set_tenant_active (flag + control.operator_audit row in one transaction),
--   * control.operator_audit (append-only: a trigger rejects UPDATE/DELETE/TRUNCATE for every role, the owner included),
--   * control.platform_status / control.read_operator_audit (read definers),
--   * control.store_serving(tenant, store): the one predicate the two remaining outbound planners use,
--   * guards: integration.claim_reply_plannable (new skip code store_suspended: the comment is recorded, no automatic
--     private reply is planned) and notify.claim_batch (no buyer/merchant mail for a suspended store; rows stay PENDING).
-- Suspension never deletes data, never unbinds a PSP/Meta binding and never blocks money already in flight: payment
-- notifies/queries, refunds and expiry keep running (those definers do not read these flags and are not touched here).
--
-- Roles: commerce_platform_writer (NOLOGIN, owns the definers, no members); commerce_platform_operator (NOLOGIN, EXECUTE on the
-- operator definers only; deployment gives it exactly one LOGIN member, INHERIT TRUE / SET FALSE, same rule as the
-- store/stripe registrars; integrator wires the login).

CREATE ROLE commerce_platform_writer NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE commerce_platform_operator NOLOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;
COMMENT ON ROLE commerce_platform_writer IS
 '0143 owner of the platform-operator definers (cmd/platform-admin). NOLOGIN, no members; flips control.tenants.active / control.stores.active and appends control.operator_audit only through its fixed functions.';
COMMENT ON ROLE commerce_platform_operator IS
 '0143 operator authority: EXECUTE on control.set_store_active / set_tenant_active / platform_status / read_operator_audit only. Exactly one login used by the ops one-shot cmd/platform-admin, never a long-running service.';

GRANT USAGE ON SCHEMA control TO commerce_platform_writer, commerce_platform_operator;

-- ---------------------------------------------------------------------------------------
-- Append-only operator audit. No FK: the audit must outlive any later erasure of a tenant or store row.
-- ---------------------------------------------------------------------------------------
CREATE TABLE control.operator_audit (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  operator text NOT NULL CHECK (operator ~ '^[a-z0-9._-]{1,40}$'),
  db_user text NOT NULL,
  action text NOT NULL CHECK (action IN ('store_suspend','store_resume','tenant_suspend','tenant_resume')),
  tenant_id uuid NOT NULL,
  store_id uuid,
  ticket text NOT NULL CHECK (length(ticket) BETWEEN 1 AND 80 AND ticket !~ '[[:cntrl:]]' AND ticket ~ '[^[:space:]]'),
  reason text CHECK (reason IN ('fraud','non_payment','legal','owner_request','other')),
  detail jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(detail)='object' AND octet_length(detail::text)<=1024),
  CHECK ((action LIKE 'store_%')=(store_id IS NOT NULL))
);
CREATE INDEX operator_audit_recent ON control.operator_audit(occurred_at DESC, id);
ALTER TABLE control.operator_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE control.operator_audit FORCE ROW LEVEL SECURITY;
REVOKE ALL ON control.operator_audit FROM PUBLIC;
GRANT SELECT, INSERT ON control.operator_audit TO commerce_platform_writer;
CREATE POLICY operator_audit_writer_read ON control.operator_audit FOR SELECT TO commerce_platform_writer USING (true);
CREATE POLICY operator_audit_writer_insert ON control.operator_audit FOR INSERT TO commerce_platform_writer WITH CHECK (true);
COMMENT ON POLICY operator_audit_writer_read ON control.operator_audit IS '0143: read_operator_audit definer.';
COMMENT ON POLICY operator_audit_writer_insert ON control.operator_audit IS '0143: set_store_active / set_tenant_active append one row per call.';

CREATE FUNCTION control.operator_audit_append_only() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 RAISE EXCEPTION 'control.operator_audit is append-only' USING ERRCODE='42501';
END $$;
CREATE TRIGGER operator_audit_no_update_delete BEFORE UPDATE OR DELETE ON control.operator_audit
 FOR EACH ROW EXECUTE FUNCTION control.operator_audit_append_only();
CREATE TRIGGER operator_audit_no_truncate BEFORE TRUNCATE ON control.operator_audit
 FOR EACH STATEMENT EXECUTE FUNCTION control.operator_audit_append_only();
COMMENT ON TABLE control.operator_audit IS
 '0143 append-only platform-operator audit (who: operator + session_user, what: action, which: tenant/store, why: ticket + reason code). Written only by control.set_store_active / set_tenant_active (commerce_platform_writer); read only through control.read_operator_audit. UPDATE/DELETE/TRUNCATE are rejected by trigger for every role. OPS-02B (support grants) widens the action CHECK and reuses it.';

-- ---------------------------------------------------------------------------------------
-- Table access of the definer owner (FORCE RLS on stores stays: each grant has an explicit policy).
-- control.tenants has no RLS.
-- ---------------------------------------------------------------------------------------
GRANT SELECT(id,tenant_id,active), UPDATE(active) ON control.stores TO commerce_platform_writer;
GRANT SELECT(id,active), UPDATE(active) ON control.tenants TO commerce_platform_writer;
CREATE POLICY stores_platform_read ON control.stores FOR SELECT TO commerce_platform_writer USING (true);
CREATE POLICY stores_platform_update ON control.stores FOR UPDATE TO commerce_platform_writer USING (true) WITH CHECK (true);
COMMENT ON POLICY stores_platform_read ON control.stores IS '0143: platform-operator definers (set_store_active, platform_status, store_serving) read any store.';
COMMENT ON POLICY stores_platform_update ON control.stores IS '0143: set_store_active flips stores.active (column grant UPDATE(active) only).';

-- ---------------------------------------------------------------------------------------
-- Operator definers. The EXECUTE grant is the authority check; there is no bearer.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION control.set_store_active(p_store uuid,p_active boolean,p_operator text,p_ticket text,p_reason text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_was boolean; v_changed boolean; v_tenant_active boolean;
BEGIN
 IF p_store IS NULL OR p_active IS NULL OR p_operator IS NULL OR p_operator COLLATE "C" !~ '^[a-z0-9._-]{1,40}$'
  OR p_ticket IS NULL OR length(p_ticket) NOT BETWEEN 1 AND 80 OR p_ticket ~ '[[:cntrl:]]' OR p_ticket !~ '[^[:space:]]'
  OR (p_active AND p_reason IS NOT NULL)
  OR (NOT p_active AND (p_reason IS NULL OR p_reason NOT IN ('fraud','non_payment','legal','owner_request','other'))) THEN
  RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 SELECT s.tenant_id,s.active INTO v_tenant,v_was FROM control.stores s WHERE s.id=p_store FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'store not found' USING ERRCODE='PT404'; END IF;
 v_changed:=v_was IS DISTINCT FROM p_active;
 IF v_changed THEN UPDATE control.stores SET active=p_active WHERE id=p_store; END IF;
 SELECT t.active INTO v_tenant_active FROM control.tenants t WHERE t.id=v_tenant;
 -- Every call is audited, a no-op included (OP06): who asked, under which ticket, and that nothing changed.
 INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket,reason,detail)
 VALUES(p_operator,session_user,CASE WHEN p_active THEN 'store_resume' ELSE 'store_suspend' END,v_tenant,p_store,p_ticket,p_reason,
  jsonb_build_object('changed',v_changed,'was_active',v_was));
 RETURN jsonb_build_object('store_id',p_store,'tenant_id',v_tenant,'active',p_active,'changed',v_changed,
  'result',CASE WHEN v_changed THEN 'changed' ELSE 'unchanged' END,'tenant_active',v_tenant_active);
END $$;

CREATE FUNCTION control.set_tenant_active(p_tenant uuid,p_active boolean,p_operator text,p_ticket text,p_reason text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_was boolean; v_changed boolean;
BEGIN
 IF p_tenant IS NULL OR p_active IS NULL OR p_operator IS NULL OR p_operator COLLATE "C" !~ '^[a-z0-9._-]{1,40}$'
  OR p_ticket IS NULL OR length(p_ticket) NOT BETWEEN 1 AND 80 OR p_ticket ~ '[[:cntrl:]]' OR p_ticket !~ '[^[:space:]]'
  OR (p_active AND p_reason IS NOT NULL)
  OR (NOT p_active AND (p_reason IS NULL OR p_reason NOT IN ('fraud','non_payment','legal','owner_request','other'))) THEN
  RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 SELECT t.active INTO v_was FROM control.tenants t WHERE t.id=p_tenant FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'tenant not found' USING ERRCODE='PT404'; END IF;
 v_changed:=v_was IS DISTINCT FROM p_active;
 IF v_changed THEN UPDATE control.tenants SET active=p_active WHERE id=p_tenant; END IF;
 INSERT INTO control.operator_audit(operator,db_user,action,tenant_id,store_id,ticket,reason,detail)
 VALUES(p_operator,session_user,CASE WHEN p_active THEN 'tenant_resume' ELSE 'tenant_suspend' END,p_tenant,NULL,p_ticket,p_reason,
  jsonb_build_object('changed',v_changed,'was_active',v_was));
 RETURN jsonb_build_object('tenant_id',p_tenant,'active',p_active,'changed',v_changed,
  'result',CASE WHEN v_changed THEN 'changed' ELSE 'unchanged' END);
END $$;

-- Read-only: one store (serving = tenant AND store active) or every store of one tenant (at most 200 listed).
CREATE FUNCTION control.platform_status(p_tenant uuid,p_store uuid)
RETURNS jsonb LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid:=p_tenant; v_tenant_active boolean;
BEGIN
 IF (p_tenant IS NULL)=(p_store IS NULL) THEN RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 IF p_store IS NOT NULL THEN
  SELECT s.tenant_id INTO v_tenant FROM control.stores s WHERE s.id=p_store;
  IF NOT FOUND THEN RAISE EXCEPTION 'store not found' USING ERRCODE='PT404'; END IF;
 END IF;
 SELECT t.active INTO v_tenant_active FROM control.tenants t WHERE t.id=v_tenant;
 IF NOT FOUND THEN RAISE EXCEPTION 'tenant not found' USING ERRCODE='PT404'; END IF;
 RETURN jsonb_build_object('tenant_id',v_tenant,'tenant_active',v_tenant_active,'stores',coalesce((
  SELECT jsonb_agg(jsonb_build_object('store_id',q.id,'active',q.active,'serving',q.active AND v_tenant_active) ORDER BY q.id)
  FROM (SELECT s.id,s.active FROM control.stores s WHERE s.tenant_id=v_tenant AND (p_store IS NULL OR s.id=p_store) ORDER BY s.id LIMIT 200) q),'[]'::jsonb));
END $$;

-- Newest first. Only fields the operator wrote or the definer stamped: no merchant or buyer data.
CREATE FUNCTION control.read_operator_audit(p_since timestamptz,p_limit integer)
RETURNS TABLE(id uuid,occurred_at timestamptz,operator text,db_user text,action text,tenant_id uuid,store_id uuid,ticket text,reason text,detail jsonb)
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 500 OR (p_since IS NOT NULL AND NOT isfinite(p_since)) THEN
  RAISE EXCEPTION 'invalid platform operator request' USING ERRCODE='PT400'; END IF;
 RETURN QUERY SELECT a.id,a.occurred_at,a.operator,a.db_user,a.action,a.tenant_id,a.store_id,a.ticket,a.reason,a.detail
  FROM control.operator_audit a WHERE p_since IS NULL OR a.occurred_at>=p_since ORDER BY a.occurred_at DESC,a.id LIMIT p_limit;
END $$;

-- The one predicate the outbound planners below use (true iff the store AND its tenant are active; false when unknown).
-- Boolean only: it discloses nothing beyond what its callers already know about their own scope.
CREATE FUNCTION control.store_serving(p_tenant uuid,p_store uuid) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT coalesce((SELECT s.active AND t.active FROM control.stores s JOIN control.tenants t ON t.id=s.tenant_id
  WHERE s.tenant_id=p_tenant AND s.id=p_store),false)
$$;

ALTER FUNCTION control.set_store_active(uuid,boolean,text,text,text) OWNER TO commerce_platform_writer;
ALTER FUNCTION control.set_tenant_active(uuid,boolean,text,text,text) OWNER TO commerce_platform_writer;
ALTER FUNCTION control.platform_status(uuid,uuid) OWNER TO commerce_platform_writer;
ALTER FUNCTION control.read_operator_audit(timestamptz,integer) OWNER TO commerce_platform_writer;
ALTER FUNCTION control.store_serving(uuid,uuid) OWNER TO commerce_platform_writer;
REVOKE ALL ON FUNCTION control.set_store_active(uuid,boolean,text,text,text), control.set_tenant_active(uuid,boolean,text,text,text),
 control.platform_status(uuid,uuid), control.read_operator_audit(timestamptz,integer), control.store_serving(uuid,uuid),
 control.operator_audit_append_only() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION control.set_store_active(uuid,boolean,text,text,text), control.set_tenant_active(uuid,boolean,text,text,text),
 control.platform_status(uuid,uuid), control.read_operator_audit(timestamptz,integer) TO commerce_platform_operator;
GRANT EXECUTE ON FUNCTION control.store_serving(uuid,uuid) TO commerce_integration_writer, commerce_checkout_writer;

COMMENT ON FUNCTION control.set_store_active(uuid,boolean,text,text,text) IS
 'cmd/platform-admin store-suspend|store-resume only; EXECUTE commerce_platform_operator. Flips control.stores.active (suspend needs a reason code fraud|non_payment|legal|owner_request|other, resume none), appends control.operator_audit (operator, session_user, ticket) in the same transaction, a no-op included (result unchanged). Deletes nothing, closes no live window, touches no PSP/Meta binding.';
COMMENT ON FUNCTION control.set_tenant_active(uuid,boolean,text,text,text) IS
 'cmd/platform-admin tenant-suspend|tenant-resume only; EXECUTE commerce_platform_operator. Flips control.tenants.active (every store of the tenant stops serving; per-store flags are untouched, so a resume restores exactly the previous per-store state) and appends control.operator_audit.';
COMMENT ON FUNCTION control.platform_status(uuid,uuid) IS
 'cmd/platform-admin status only; EXECUTE commerce_platform_operator. Read-only: tenant_active and per store active/serving (store AND tenant); ids and flags only.';
COMMENT ON FUNCTION control.read_operator_audit(timestamptz,integer) IS
 'cmd/platform-admin audit only; EXECUTE commerce_platform_operator. Newest-first rows of control.operator_audit (limit 1..500, optional occurred_at lower bound); the only read path of that table.';
COMMENT ON FUNCTION control.store_serving(uuid,uuid) IS
 'Guard predicate for outbound planners: true iff the store and its tenant are active. EXECUTE commerce_integration_writer (integration.claim_reply_plannable) and commerce_checkout_writer (notify.claim_batch) only; boolean result, no data.';
COMMENT ON FUNCTION control.operator_audit_append_only() IS '0143 trigger body: rejects UPDATE/DELETE/TRUNCATE of control.operator_audit for every role.';

-- ---------------------------------------------------------------------------------------
-- Guard 1: no automatic claim reply is planned for a suspended store. The intake (comment, claim) is still recorded;
-- the skip is audited like source_off/binding_disabled. Patched in place from the live 0128 definition (like 0097/0088):
-- the patch fails loudly when the definition is not the expected one; ownership and grants are re-asserted.
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
 v_needle:=' IF NOT s.active OR NOT s.private_reply THEN v_code:=''source_off'';';
 v_repl:=' IF NOT control.store_serving(i.tenant_id,i.store_id) THEN v_code:=''store_suspended'';'||chr(10)||' ELSIF NOT s.active OR NOT s.private_reply THEN v_code:=''source_off'';';
 v_fn:=pg_get_functiondef('integration.claim_reply_plannable(uuid)'::regprocedure);
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'integration.claim_reply_plannable has an unexpected shape for the suspension patch'; END IF;
 EXECUTE replace(v_fn,v_needle,v_repl);
END $$;
ALTER FUNCTION integration.claim_reply_plannable(uuid) OWNER TO commerce_integration_writer;
CREATE POLICY claim_reply_suspended_audit ON ops.audit_events FOR INSERT TO commerce_integration_writer
 WITH CHECK ((tenant_id, store_id) = (SELECT s.tenant_id, s.store_id FROM claims.intake_scope() s)
  AND action = 'claim_reply_skipped:store_suspended');

-- ---------------------------------------------------------------------------------------
-- Guard 2: no buyer or merchant mail is claimed for a suspended store. The predicate sits in the candidate WHERE (not in
-- the loop) so a suspended store's old PENDING rows cannot fill the LIMIT window and starve every other store; the rows
-- stay PENDING and the existing 24 h stale rule applies if the suspension outlasts it. record_result is untouched: a send
-- already claimed is still recorded.
-- ---------------------------------------------------------------------------------------
DO $$
DECLARE v_fn text; v_n1 text; v_n2 text;
BEGIN
 v_n1:='AND (SELECT count(*) FROM notify.outbox h WHERE h.store_id=x.store_id';
 v_n2:='WHERE x.kind=''merchant_new'' AND x.state=''PENDING'' AND x.next_attempt_at<=v_now LOOP';
 v_fn:=pg_get_functiondef('notify.claim_batch(integer,integer,integer)'::regprocedure);
 IF (length(v_fn)-length(replace(v_fn,v_n1,'')))<>length(v_n1) OR (length(v_fn)-length(replace(v_fn,v_n2,'')))<>length(v_n2) THEN
  RAISE EXCEPTION 'notify.claim_batch has an unexpected shape for the suspension patch'; END IF;
 v_fn:=replace(v_fn,v_n1,'AND control.store_serving(x.tenant_id,x.store_id) '||v_n1);
 v_fn:=replace(v_fn,v_n2,'WHERE x.kind=''merchant_new'' AND x.state=''PENDING'' AND x.next_attempt_at<=v_now AND control.store_serving(x.tenant_id,x.store_id) LOOP');
 EXECUTE v_fn;
END $$;
ALTER FUNCTION notify.claim_batch(integer,integer,integer) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION notify.claim_batch(integer,integer,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.claim_batch(integer,integer,integer) TO commerce_expiry_worker;
