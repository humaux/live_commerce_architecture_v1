-- Purpose: OC-16 (contracts/owner-choices-20261026-v1.md OC-Q1): the SERVER closes new bank-transfer writes. A single-row policy table
--   checkout.bank_transfer_policy(closed) is inserted closed=true; BEFORE INSERT triggers on checkout.bank_transfers (a new transfer order)
--   and BEFORE INSERT/UPDATE on checkout.bank_transfer_settings (saving or editing the merchant bank details), and payments.set_bank_transfer_settings
--   itself (after authorization), raise PT422 bank_transfer_closed while closed. Reopening is a new migration (an ADR), never a runtime switch.
-- Depends on: 0088 (checkout.bank_transfers, checkout.bank_transfer_settings, payments.set_bank_transfer_settings, role commerce_checkout_writer),
--   0184 (checkout.order_attempt_begin inserts the transfer row; unchanged, the trigger refuses it and the whole begin rolls back).
-- Used by: migrations.Apply (numbered phase); tests/foundation/bank_transfer_closed_test.go (BTC gates); tests/foundation/foundation_integration_test.go
--   (the fixture flips the policy open for legacy tests, issue #210); tests/foundation/checkout_offline_gate_test.go (trigger pin).
-- Invariants: no row is changed or deleted. decide_bank_transfer, expire_held, submit_transfer_proof and every read/export are untouched, so
--   existing AWAITING_TRANSFER orders run to their terminal state and history stays readable. Only the migration owner can write the policy row:
--   no commerce_* role holds INSERT/UPDATE/DELETE/TRUNCATE on it (commerce_checkout_writer, the owner of the guard definer, has SELECT only).

CREATE TABLE checkout.bank_transfer_policy(
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 closed boolean NOT NULL
);
INSERT INTO checkout.bank_transfer_policy(closed) VALUES(true);
REVOKE ALL ON checkout.bank_transfer_policy FROM PUBLIC;
GRANT SELECT ON checkout.bank_transfer_policy TO commerce_checkout_writer;
COMMENT ON TABLE checkout.bank_transfer_policy IS 'Single row. closed=true refuses new bank_transfer orders and new/edited bank details (PT422 bank_transfer_closed). Written only by a migration (owner); app roles have no write grant.';

CREATE FUNCTION checkout.bank_transfer_closed_guard() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM checkout.bank_transfer_policy WHERE closed) THEN
  RAISE EXCEPTION 'bank_transfer_closed' USING ERRCODE='PT422'; END IF;
 RETURN NEW;
END $$;
ALTER FUNCTION checkout.bank_transfer_closed_guard() OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.bank_transfer_closed_guard() FROM PUBLIC;
COMMENT ON FUNCTION checkout.bank_transfer_closed_guard() IS 'Trigger-only (no EXECUTE grant). Refuses a new checkout.bank_transfers row and a new or edited checkout.bank_transfer_settings row while checkout.bank_transfer_policy.closed. Non-goal: never reads or writes anything else.';

CREATE TRIGGER bank_transfer_closed_guard BEFORE INSERT ON checkout.bank_transfers
 FOR EACH ROW EXECUTE FUNCTION checkout.bank_transfer_closed_guard();
CREATE TRIGGER bank_transfer_closed_guard BEFORE INSERT OR UPDATE ON checkout.bank_transfer_settings
 FOR EACH ROW EXECUTE FUNCTION checkout.bank_transfer_closed_guard();

CREATE OR REPLACE FUNCTION payments.set_bank_transfer_settings(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,p_expected_version bigint,
 p_enabled boolean,p_allow_cvs boolean,p_bank_name text,p_branch text,p_account_name text,p_account_number text,p_window_hours integer)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; cur record; v_saved bytea; v_response jsonb; v_now timestamptz; v_ver bigint;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_expected_version IS NULL OR p_expected_version<0
  OR p_expected_version>=9223372036854775807 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid bank transfer settings' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 -- 0202 (OC-Q1): no new bank details are saved while the policy is closed. Refused after authorization and before any receipt replay or write.
 IF EXISTS(SELECT 1 FROM checkout.bank_transfer_policy WHERE closed) THEN RAISE EXCEPTION 'bank_transfer_closed' USING ERRCODE='PT422'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('bank_transfer.settings|'||s.tenant_id||'|'||p_store,0));
 SELECT r.request_hash,r.response INTO v_saved,v_response FROM ops.command_results r WHERE r.tenant_id=s.tenant_id
  AND r.store_id=p_store AND r.operation='checkout.bank_transfer_settings.set' AND r.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  -- CHECK-equivalent validation first so a bad body is a 422 invalid_settings, never a 500 from a constraint.
  IF p_enabled IS NULL OR p_allow_cvs IS NULL OR p_bank_name IS NULL OR p_branch IS NULL OR p_account_name IS NULL
   OR p_account_number IS NULL OR p_window_hours IS NULL OR p_window_hours NOT BETWEEN 6 AND 168
   OR char_length(btrim(p_bank_name))>60 OR char_length(btrim(p_branch))>60 OR char_length(btrim(p_account_name))>60
   OR concat(p_bank_name,p_branch,p_account_name) ~ '[[:cntrl:]]'
   OR btrim(p_account_number) !~ '^([0-9][0-9 -]{3,31})?$'
   OR (p_enabled AND (btrim(p_bank_name)='' OR btrim(p_account_name)='' OR btrim(p_account_number)='')) THEN
   RAISE EXCEPTION 'invalid_settings' USING ERRCODE='PT422'; END IF;
  SELECT c.version INTO cur FROM checkout.bank_transfer_settings c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store FOR UPDATE;
  v_now:=clock_timestamp();
  IF NOT FOUND THEN
   IF p_expected_version<>0 THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
   v_ver:=1;
   INSERT INTO checkout.bank_transfer_settings(tenant_id,store_id,enabled,allow_cvs,bank_name,branch,account_name,account_number,window_hours,version,updated_at)
   VALUES(s.tenant_id,p_store,p_enabled,p_allow_cvs,btrim(p_bank_name),btrim(p_branch),btrim(p_account_name),btrim(p_account_number),p_window_hours,1,v_now);
  ELSE
   IF cur.version<>p_expected_version THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
   v_ver:=cur.version+1;
   UPDATE checkout.bank_transfer_settings SET enabled=p_enabled,allow_cvs=p_allow_cvs,bank_name=btrim(p_bank_name),branch=btrim(p_branch),
    account_name=btrim(p_account_name),account_number=btrim(p_account_number),window_hours=p_window_hours,version=v_ver,updated_at=v_now
    WHERE tenant_id=s.tenant_id AND store_id=p_store;
  END IF;
  -- A change never touches placed orders (each keeps its own snapshot); only future begin_hold calls read this row.
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action)
   VALUES(s.tenant_id,p_store,s.principal_id,'checkout.bank_transfer_settings_changed');
  v_response:=jsonb_build_object('version',v_ver,'enabled',p_enabled,'allow_cvs',p_allow_cvs,'bank_name',btrim(p_bank_name),'branch',btrim(p_branch),
   'account_name',btrim(p_account_name),'account_number',btrim(p_account_number),'window_hours',p_window_hours);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,'checkout.bank_transfer_settings.set',p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'integration:manage');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
