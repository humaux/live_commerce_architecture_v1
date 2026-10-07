-- 0163 append-only resolution of unmapped_source settlement rows (contracts/stripe-platform-account-v1.md §6.6; S2-OPEN-1).
-- Purpose: a charge on the platform Stripe account that the system never created blocks the WHOLE environment's close
--   (0150 §6.2) and had no sanctioned remedy (the W4-S2 test deleted the row through the owner pool). This adds the
--   resolve step: one append-only resolution row per unattributed balance transaction, and an in-place patch of
--   payments.close_settlement so a resolved row no longer refuses close. v1 moves NO money: an assigned_to_store
--   resolution is recorded and printed by close as an operator note; the owner pays the store out of band.
-- Depends on: 0150 (settlement_unattributed, close_settlement, scope/advisory-lock/policy patterns), 0137 (stripe_platform,
--   platform_stripe_enrollments), 0061 (require_stripe_registrar_scope, control.stores read), ops.audit_events.
-- Used by: internal/payments/stripeadmin (settlement.go: SettlementResolve, SettlementClose operator_notes),
--   cmd/stripe-admin settlement-resolve (operator CLI), deploy/scripts/ops-admin.sh (allowlist),
--   tests/foundation/settlement_resolve_test.go (PF15).
-- Invariants: I02/I06/I20 (idempotent by balance transaction id: an identical replay returns the stored row, a different
--   payload is PT409), I19 (PostgreSQL is the single source of truth), I05 (close still refuses unresolved and mismatched rows).
-- Status: REAL_PG + MOCK only. No Stripe call: a resolution is operator-entered, like the payout record (0150).

-- ---------------------------------------------------------------------------------------------------------
-- The resolution table (FORCE RLS, PUBLIC revoked, append-only: SELECT,INSERT to the registry writer, never UPDATE/DELETE)
-- ---------------------------------------------------------------------------------------------------------
CREATE TABLE payments.settlement_unattributed_resolutions (
 balance_txn_id text PRIMARY KEY REFERENCES payments.settlement_unattributed(balance_txn_id),
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,            -- the platform store's scope (the row is written by the operator definer)
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 resolution text NOT NULL CHECK(resolution IN ('not_store_revenue','assigned_to_store')),
 target_tenant_id uuid,target_store_id uuid,                -- both set iff resolution='assigned_to_store'
 operator text NOT NULL CHECK(operator ~ '^[A-Za-z0-9._:@-]{2,64}$'),
 ticket text NOT NULL CHECK(ticket ~ '^[A-Za-z0-9._:-]{8,128}$'),
 note text NOT NULL CHECK(char_length(note) BETWEEN 1 AND 500),
 resolved_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 FOREIGN KEY(target_tenant_id,target_store_id) REFERENCES control.stores(tenant_id,id),
 CHECK((resolution='assigned_to_store')=(target_tenant_id IS NOT NULL AND target_store_id IS NOT NULL)),
 CHECK((target_tenant_id IS NULL)=(target_store_id IS NULL))
);

ALTER TABLE payments.settlement_unattributed_resolutions ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments.settlement_unattributed_resolutions FORCE ROW LEVEL SECURITY;
REVOKE ALL ON payments.settlement_unattributed_resolutions FROM PUBLIC;
GRANT SELECT,INSERT ON payments.settlement_unattributed_resolutions TO commerce_payment_registry_writer;
COMMENT ON TABLE payments.settlement_unattributed_resolutions IS
 'payments owner (internal/payments/stripeadmin settlement.go, cmd/stripe-admin settlement-resolve); only commerce_payment_registry_writer definers touch it; APPEND-ONLY: no UPDATE or DELETE grant to anyone, the unattributed row itself is never touched; no worker, ingress, checkout or integration role; never PSP authority';
DO $$ DECLARE v_col record; BEGIN
 FOR v_col IN SELECT column_name FROM information_schema.columns WHERE table_schema='payments' AND table_name='settlement_unattributed_resolutions' LOOP
  EXECUTE format('COMMENT ON COLUMN payments.settlement_unattributed_resolutions.%I IS %L',v_col.column_name,
   'payments owner; append-only resolution of one unmapped_source row (contracts/stripe-platform-account-v1.md §6.6); operator-entered via settlement-resolve only; moves no money');
 END LOOP;
END $$;

-- Same policy shape as settlement_unattributed (0150): own (platform) scope only; privileges are narrowed by the grants above.
CREATE POLICY settlement_resolution_rw ON payments.settlement_unattributed_resolutions TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
COMMENT ON POLICY settlement_resolution_rw ON payments.settlement_unattributed_resolutions IS
 'S2-OPEN-1: own (platform) scope only, like settlement_unattributed_rw; the operator definers pin the scope GUCs and set app.settlement_op for the cross-store statement lookups, but resolution rows always carry the platform scope';

-- ---------------------------------------------------------------------------------------------------------
-- record_settlement_resolution (RECORD ONLY: never calls Stripe or a bank; v1 moves no money, §6.6)
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.record_settlement_resolution(p_tenant uuid,p_store uuid,p_principal uuid,p_environment text,p_balance_txn text,
 p_resolution text,p_operator text,p_ticket text,p_note text,p_target_tenant uuid DEFAULT NULL,p_target_store uuid DEFAULT NULL)
 RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 sp payments.stripe_platform%ROWTYPE; u payments.settlement_unattributed%ROWTYPE; v_old record; v_monday date; v_at timestamptz;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 -- structural validation first (22023): a malformed call never reads a row
 IF p_environment IS NULL OR p_environment NOT IN ('SANDBOX','LIVE')
  OR p_balance_txn IS NULL OR p_balance_txn !~ '^txn_[A-Za-z0-9]{1,255}$'
  OR p_resolution IS NULL OR p_resolution NOT IN ('not_store_revenue','assigned_to_store')
  OR p_operator IS NULL OR p_operator !~ '^[A-Za-z0-9._:@-]{2,64}$'
  OR p_ticket IS NULL OR p_ticket !~ '^[A-Za-z0-9._:-]{8,128}$'
  OR p_note IS NULL OR char_length(p_note) NOT BETWEEN 1 AND 500
  OR (p_target_tenant IS NULL)<>(p_target_store IS NULL)
  OR (p_resolution='assigned_to_store')<>(p_target_tenant IS NOT NULL) THEN
  RAISE EXCEPTION 'invalid settlement resolution' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.settlement_op','on',true);
 -- one writer per environment: a resolution, a sync and a close never interleave (the 0150 lock)
 PERFORM pg_advisory_xact_lock(hashtextextended('lc.settlement.'||p_environment,0));
 SELECT r.* INTO u FROM payments.settlement_unattributed r WHERE r.balance_txn_id=p_balance_txn;
 IF NOT FOUND OR u.environment<>p_environment THEN
  RAISE EXCEPTION 'unattributed_unavailable' USING ERRCODE='PT409'; END IF;
 -- a foreign_connection or unsupported_type row is not an attribution question: it never blocks close and is not resolvable here
 IF u.reason<>'unmapped_source' THEN
  RAISE EXCEPTION 'unresolvable_reason' USING ERRCODE='PT409'; END IF;
 -- idempotent (I02/I06): the same canonical payload replays the stored row; a different payload for the same txn is a conflict
 SELECT r.resolution,r.target_tenant_id,r.target_store_id,r.operator,r.ticket,r.note,r.resolved_at INTO v_old
  FROM payments.settlement_unattributed_resolutions r WHERE r.balance_txn_id=p_balance_txn;
 IF FOUND THEN
  IF v_old.resolution<>p_resolution OR v_old.target_tenant_id IS DISTINCT FROM p_target_tenant
   OR v_old.target_store_id IS DISTINCT FROM p_target_store OR v_old.operator<>p_operator OR v_old.ticket<>p_ticket
   OR v_old.note<>p_note THEN
   RAISE EXCEPTION 'resolution_conflict' USING ERRCODE='PT409'; END IF;
  RETURN jsonb_build_object('balance_txn_id',p_balance_txn,'resolution',v_old.resolution,'target_tenant_id',v_old.target_tenant_id,
   'target_store_id',v_old.target_store_id,'operator',v_old.operator,'ticket',v_old.ticket,'note',v_old.note,
   'resolved_at',v_old.resolved_at,'replayed',true);
 END IF;
 -- the row's own weekly Asia/Taipei period (§6.2). A period already closed for ANY store is frozen: resolving a row that
 -- belongs to it would silently rewrite what that close had to refuse, so it fails closed and escalates to the owner (§9).
 v_monday:=date_trunc('week',u.txn_created_at AT TIME ZONE 'Asia/Taipei')::date;
 IF EXISTS(SELECT 1 FROM payments.settlement_statements s WHERE s.environment=p_environment AND s.period_start=v_monday) THEN
  RAISE EXCEPTION 'period_already_closed' USING ERRCODE='PT409'; END IF;
 -- assigned_to_store: the target must be a store that has USED the platform account in this environment (an enrollment),
 -- or the platform store itself. Anything else (unknown, foreign, never enrolled) is refused; the FK proves existence.
 IF p_resolution='assigned_to_store'
  AND NOT ((p_target_tenant=sp.tenant_id AND p_target_store=sp.store_id)
   OR EXISTS(SELECT 1 FROM payments.platform_stripe_enrollments e WHERE e.tenant_id=p_target_tenant
     AND e.store_id=p_target_store AND e.environment=p_environment)) THEN
  RAISE EXCEPTION 'unknown_target_store' USING ERRCODE='PT409'; END IF;
 INSERT INTO payments.settlement_unattributed_resolutions(balance_txn_id,tenant_id,store_id,environment,resolution,
   target_tenant_id,target_store_id,operator,ticket,note)
  VALUES(p_balance_txn,p_tenant,p_store,p_environment,p_resolution,p_target_tenant,p_target_store,p_operator,p_ticket,p_note)
  RETURNING resolved_at INTO v_at;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
  VALUES(p_tenant,p_store,p_principal,'stripe.settlement.resolve',jsonb_strip_nulls(jsonb_build_object('environment',p_environment,
   'balance_txn_id',p_balance_txn,'resolution',p_resolution,'target_tenant',p_target_tenant,'target_store',p_target_store,
   'ticket',p_ticket)));
 RETURN jsonb_build_object('balance_txn_id',p_balance_txn,'resolution',p_resolution,'target_tenant_id',p_target_tenant,
  'target_store_id',p_target_store,'operator',p_operator,'ticket',p_ticket,'note',p_note,'resolved_at',v_at,'replayed',false);
END $$;
ALTER FUNCTION payments.record_settlement_resolution(uuid,uuid,uuid,text,text,text,text,text,text,uuid,uuid) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.record_settlement_resolution(uuid,uuid,uuid,text,text,text,text,text,text,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.record_settlement_resolution(uuid,uuid,uuid,text,text,text,text,text,text,uuid,uuid) TO commerce_payment_registrar;
COMMENT ON FUNCTION payments.record_settlement_resolution(uuid,uuid,uuid,text,text,text,text,text,text,uuid,uuid) IS
 'payments owner (S2-OPEN-1, 0163); operator definer (stripe-admin settlement-resolve, registrar only, platform store scope): appends ONE resolution row for one unmapped_source row of settlement_unattributed so the environment''s close can proceed (§6.6): not_store_revenue (no target) or assigned_to_store (the target must be enrolled on the platform account in the environment, or be the platform store; v1 records the assignment as an operator note printed by close and moves NO money). Never edits or deletes the unattributed row; idempotent by balance transaction id (an identical replay returns the stored row, a different payload is PT409); refuses foreign_connection/unsupported_type rows and rows whose weekly period is closed for any store. Calls no Stripe or bank API; moves no money';

-- ---------------------------------------------------------------------------------------------------------
-- close_settlement, patched IN PLACE from the live definition (the 0143/0151 pattern: pg_get_functiondef, an exactly-once
-- anchor per needle, loud failure on any other shape; ownership re-asserted, signature/owner/grants unchanged).
--   1. the refusal comment names the resolution row;
--   2. the unmapped_source refusal skips rows that have a resolution (the ONLY clearing path; totals never change);
--   3. close returns operator_notes: the window's assigned_to_store resolutions, printed by the CLI (§6.6 v1).
-- ---------------------------------------------------------------------------------------------------------
DO $$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
 v_needle := '-- an unmapped source might be any store''s charge: the whole environment''s close waits (resolved = a line now exists for it)';
 v_repl := '-- an unmapped source might be any store''s charge: the whole environment''s close waits (resolved = a line exists for it, or a 0163 resolution row)';
 v_fn := pg_get_functiondef('payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text)'::regprocedure);
 IF (length(v_fn) - length(replace(v_fn, v_needle, ''))) <> length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0163 comment patch';
 END IF;
 EXECUTE replace(v_fn, v_needle, v_repl);

 v_needle := 'AND NOT EXISTS(SELECT 1 FROM payments.settlement_lines l WHERE l.balance_txn_id=u.balance_txn_id)) THEN';
 -- alias res, NOT r: close_settlement's DECLARE block owns r/t/st as record variables and plpgsql would raise
 -- "column reference is ambiguous" on r.balance_txn_id (#variable_conflict error at plan time)
 v_repl := 'AND NOT EXISTS(SELECT 1 FROM payments.settlement_lines l WHERE l.balance_txn_id=u.balance_txn_id)' || chr(10)
  || '   AND NOT EXISTS(SELECT 1 FROM payments.settlement_unattributed_resolutions res WHERE res.balance_txn_id=u.balance_txn_id)) THEN';
 v_fn := pg_get_functiondef('payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text)'::regprocedure);
 IF (length(v_fn) - length(replace(v_fn, v_needle, ''))) <> length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0163 resolution patch';
 END IF;
 EXECUTE replace(v_fn, v_needle, v_repl);

 v_needle := ' RETURN jsonb_build_object(''statements'',v_out);';
 v_repl := ' RETURN jsonb_build_object(''statements'',v_out,''operator_notes'',coalesce((SELECT jsonb_agg(jsonb_build_object('
  || '''balance_txn_id'',u.balance_txn_id,''target_tenant_id'',res.target_tenant_id,''target_store_id'',res.target_store_id,'
  || '''operator'',res.operator,''ticket'',res.ticket,''note'',res.note) ORDER BY u.balance_txn_id)'
  || ' FROM payments.settlement_unattributed u JOIN payments.settlement_unattributed_resolutions res ON res.balance_txn_id=u.balance_txn_id'
  || ' WHERE u.environment=p_environment AND u.reason=''unmapped_source'' AND u.txn_created_at<v_end'
  || ' AND res.resolution=''assigned_to_store''),''[]''::jsonb));';
 v_fn := pg_get_functiondef('payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text)'::regprocedure);
 IF (length(v_fn) - length(replace(v_fn, v_needle, ''))) <> length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0163 operator-notes patch';
 END IF;
 EXECUTE replace(v_fn, v_needle, v_repl);
END $$;
ALTER FUNCTION payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text) OWNER TO commerce_payment_registry_writer;
COMMENT ON FUNCTION payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text) IS
 'payments owner (W4-S2, resolve step S2-OPEN-1/0163); operator definer (stripe-admin settlement-close, registrar only): closes the weekly Asia/Taipei statement of one store or of every store with lines or an earlier statement: captured minus refunded minus disputes plus Stripe fees (negative) minus platform fee plus a negative carry-in, assigns the lines once, refuses while the period is not +72 h old, a sync did not cover it, a mismatch or an UNRESOLVED unmapped source exists (a 0163 resolution row clears the refusal; resolved assigned_to_store rows are returned as operator_notes and move no money). Idempotent per store and period. Records what the platform owes the store; moves no money';
