-- 0200 settlement payout returns, stored sync reconciliation with a close gate, and the payables list
--   (contracts/stripe-platform-account-v1.md §6.7, §6.8, §6.9; SA-1; gates PF16-PF18).
-- Purpose: (1) a payout that bounced is recorded APPEND-ONLY (settlement_payout_returns) and carried once into the store's next statement as
--   returned_in_minor; the original payout triple is never changed. (2) every sync run stores its comparison with Stripe per settlement currency
--   (settlement_sync_reconciliations), computed by SQL from a window manifest, and close refuses while the LATEST covering observation of any
--   segment of its coverage interval has a non-zero difference (sync_unreconciled). (3) read_settlement_payables lists who is owed money.
--   Nothing here calls a bank, Stripe payout, transfer or refund API: a return is a fact the owner already saw at the bank (S-I4).
-- Depends on: 0150 (statements, lines, unattributed, sync runs, record_settlement_lines, close_settlement, settlement_statement_json and the
--   scope/advisory-lock/policy patterns), 0163 (resolution patches that this migration keeps by patching IN PLACE), 0137 (stripe_platform),
--   0061 (require_stripe_registrar_scope), 0077/0137 (approve_stripe_live, set_stripe_platform_open checklist arrays), ops.audit_events.
-- Used by: internal/payments/stripeadmin (settlement.go), internal/payments/settlement (merchant DTO, CSV), cmd/stripe-admin
--   settlement-payout-return | settlement-payables | settlement-sync, deploy/scripts/ops-admin.sh (allowlist), tests/foundation
--   (settlement_returns_test.go PF16-PF18 smoke, platform_settlement_test.go ACL pins, SA-3 gate).
-- Invariants: I05/I06/I19/I20 (S-I1..S-I6 of the contract): the net identity includes returned_in; a return is carried exactly once; coverage
--   is judged by the latest observation per segment; every write shares its transaction with its audit row; PostgreSQL is the single source of truth.
-- Migration rules: forward-only, one transaction. Existing functions are patched IN PLACE with the pg_get_functiondef/prosrc + replace idiom
--   (0163): each anchor must match exactly once or the migration raises and nothing is applied. record_settlement_lines changes signature (one more
--   argument), so the ten-argument identity is created from the patched definition and the nine-argument identity is DROPPED in this same
--   transaction: no wrapper, no retained grant. guard_settlement_statement needs no patch: its row-wide comparison (0150:203-208,
--   to_jsonb(NEW) minus the payout quadruple) already covers returned_in_minor, so it is immutable like every other amount column (§6.7).
--   Section 9 adds the §8 code tw_legal_opinion to the two 0137 checklist arrays (approve_stripe_live, set_stripe_platform_open).
-- Deploy order (§9): run cmd/migrate (0200) first, then replace the runtime and stripe-admin together. Until replaced, the old runtime
--   refuses merchant settlement reads (unknown DTO keys -> ErrUnavailable) and the old stripe-admin fails its exact-signature authority
--   check (nine-argument record_settlement_lines is dropped): both fail closed, no wrong ledger row is written.
-- Status: REAL_PG + MOCK only. No Stripe call.

-- ---------------------------------------------------------------------------------------------------------
-- 1. settlement_statements.returned_in_minor and the NAMED net identity
-- ---------------------------------------------------------------------------------------------------------
ALTER TABLE payments.settlement_statements
 ADD COLUMN returned_in_minor bigint NOT NULL DEFAULT 0 CHECK(returned_in_minor BETWEEN 0 AND 100000000000);
COMMENT ON COLUMN payments.settlement_statements.returned_in_minor IS
 'payments owner; sum of the payout returns (settlement_payout_returns) carried into this statement at close (contract §6.7); frozen with the statement; 0 when nothing bounced';

DO $do$
DECLARE v_n integer; v_name text;
BEGIN
 -- the 0150 net-identity CHECK is unnamed: find it by its definition (exactly one match, else nothing is applied)
 SELECT count(*),min(c.conname) INTO v_n,v_name FROM pg_constraint c
  WHERE c.conrelid='payments.settlement_statements'::regclass AND c.contype='c'
   AND pg_get_constraintdef(c.oid) ~ 'net_payable_minor = .*captured_minor.*refunded_minor.*dispute_minor.*stripe_fee_minor.*platform_fee_minor.*carried_in_minor'
   AND pg_get_constraintdef(c.oid) !~ 'returned_in_minor';
 IF v_n<>1 THEN
  RAISE EXCEPTION 'payments.settlement_statements has % net-identity CHECK constraints, expected exactly 1',v_n;
 END IF;
 EXECUTE format('ALTER TABLE payments.settlement_statements DROP CONSTRAINT %I',v_name);
END $do$;
-- existing rows have returned_in_minor = 0, so every one still satisfies it (the ADD validates; it is not NOT VALID)
ALTER TABLE payments.settlement_statements ADD CONSTRAINT settlement_statements_net_identity
 CHECK(net_payable_minor=captured_minor-refunded_minor-dispute_minor+stripe_fee_minor-platform_fee_minor+carried_in_minor+returned_in_minor);

-- the payables list reads unpaid positive statements oldest first
CREATE INDEX settlement_statements_payable ON payments.settlement_statements(environment,closed_at,id)
 WHERE payout_ref IS NULL AND net_payable_minor>0;

-- ---------------------------------------------------------------------------------------------------------
-- 2. settlement_payout_returns: append-only, one per PAID statement, set-once carry
-- ---------------------------------------------------------------------------------------------------------
CREATE TABLE payments.settlement_payout_returns (
 statement_id uuid PRIMARY KEY,                       -- the PAID statement whose payout bounced: at most one return per statement
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,      -- copied from the statement row by SQL, never from input
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 return_ref text NOT NULL CHECK(return_ref ~ '^[A-Za-z0-9._:/-]{4,80}$'),          -- bank / owner reference; never shown to the merchant
 returned_minor bigint NOT NULL CHECK(returned_minor BETWEEN 1 AND 100000000000),
 returned_at timestamptz NOT NULL,                    -- when the money came back (operator-entered fact)
 recorded_by text NOT NULL CHECK(recorded_by ~ '^[A-Za-z0-9._:@-]{2,64}$'),
 ticket text NOT NULL CHECK(ticket ~ '^[A-Za-z0-9._:-]{8,128}$'),
 carried_into_statement_id uuid,                      -- set once by close_settlement; NULL = not yet carried
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(tenant_id,store_id,statement_id) REFERENCES payments.settlement_statements(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,carried_into_statement_id) REFERENCES payments.settlement_statements(tenant_id,store_id,id),
 CHECK(carried_into_statement_id IS DISTINCT FROM statement_id)
);
-- the FK pins tenant and store only; the ENVIRONMENT of a carry is guaranteed by close_settlement's selection (same environment), not by the FK
CREATE INDEX settlement_payout_returns_open ON payments.settlement_payout_returns(tenant_id,store_id,environment,returned_at)
 WHERE carried_into_statement_id IS NULL;
CREATE INDEX settlement_payout_returns_carried ON payments.settlement_payout_returns(tenant_id,store_id,carried_into_statement_id);

ALTER TABLE payments.settlement_payout_returns ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments.settlement_payout_returns FORCE ROW LEVEL SECURITY;
REVOKE ALL ON payments.settlement_payout_returns FROM PUBLIC;
GRANT SELECT,INSERT ON payments.settlement_payout_returns TO commerce_payment_registry_writer;
GRANT UPDATE(carried_into_statement_id) ON payments.settlement_payout_returns TO commerce_payment_registry_writer;
COMMENT ON TABLE payments.settlement_payout_returns IS
 'payments owner (SA-1, contract §6.7); only commerce_payment_registry_writer definers touch it; APPEND-ONLY: one row per paid statement whose payout bounced, only carried_into_statement_id moves once NULL to a statement id (close_settlement); no DELETE or TRUNCATE for anyone; return_ref is never shown to a merchant; moves no money';
COMMENT ON COLUMN payments.settlement_payout_returns.statement_id IS 'payments owner; the paid statement whose payout bounced (primary key: at most one return per statement)';
COMMENT ON COLUMN payments.settlement_payout_returns.tenant_id IS 'payments owner; tenant of the statement, copied from the statement row by SQL';
COMMENT ON COLUMN payments.settlement_payout_returns.store_id IS 'payments owner; store of the statement, copied from the statement row by SQL';
COMMENT ON COLUMN payments.settlement_payout_returns.environment IS 'payments owner; SANDBOX or LIVE, copied from the statement; a carry stays in the same environment (close_settlement selection)';
COMMENT ON COLUMN payments.settlement_payout_returns.return_ref IS 'payments owner; operator-entered bank or owner reference of the return; never in an audit row or a merchant response';
COMMENT ON COLUMN payments.settlement_payout_returns.returned_minor IS 'payments owner; returned amount in store-currency minor units; must equal the statement payout_minor (whole-payout returns only)';
COMMENT ON COLUMN payments.settlement_payout_returns.returned_at IS 'payments owner; operator-entered time the money came back; not before paid_at and not in the future';
COMMENT ON COLUMN payments.settlement_payout_returns.recorded_by IS 'payments owner; operator name';
COMMENT ON COLUMN payments.settlement_payout_returns.ticket IS 'payments owner; operator ticket reference';
COMMENT ON COLUMN payments.settlement_payout_returns.carried_into_statement_id IS 'payments owner; the later statement that carries this return as returned_in_minor; set once by close_settlement, NULL until then';
COMMENT ON COLUMN payments.settlement_payout_returns.recorded_at IS 'payments owner; server time of the record';

-- RLS: the definers pin the scope GUCs. Reads are own scope (or every store while app.settlement_op=on); writes name their own scope.
CREATE POLICY settlement_return_read ON payments.settlement_payout_returns FOR SELECT TO commerce_payment_registry_writer
 USING((tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
  OR current_setting('app.settlement_op',true)='on');
CREATE POLICY settlement_return_insert ON payments.settlement_payout_returns FOR INSERT TO commerce_payment_registry_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY settlement_return_update ON payments.settlement_payout_returns FOR UPDATE TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
COMMENT ON POLICY settlement_return_read ON payments.settlement_payout_returns IS 'SA-1: own scope, or every store while app.settlement_op=on (operator return/close/payables). Readers keep explicit tenant/store predicates';
COMMENT ON POLICY settlement_return_insert ON payments.settlement_payout_returns IS 'SA-1: a return is inserted in its own statement store scope, which the definer pins from the statement row';
COMMENT ON POLICY settlement_return_update ON payments.settlement_payout_returns IS 'SA-1: only the set-once carry update, in the store scope close_settlement pins; the trigger allows nothing else';

-- Append-only in depth: only NULL -> value on carried_into_statement_id with every other column equal; no DELETE, no TRUNCATE, even for the owner.
CREATE FUNCTION payments.guard_settlement_return() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 IF TG_OP='UPDATE' AND OLD.carried_into_statement_id IS NULL AND NEW.carried_into_statement_id IS NOT NULL
  AND (to_jsonb(NEW)-'carried_into_statement_id') IS NOT DISTINCT FROM (to_jsonb(OLD)-'carried_into_statement_id') THEN
  RETURN NEW; END IF;
 RAISE EXCEPTION 'settlement_return_immutable' USING ERRCODE='PT409';
END $$;
ALTER FUNCTION payments.guard_settlement_return() OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.guard_settlement_return() FROM PUBLIC;
CREATE TRIGGER guard_settlement_return BEFORE UPDATE OR DELETE ON payments.settlement_payout_returns
 FOR EACH ROW EXECUTE FUNCTION payments.guard_settlement_return();
CREATE TRIGGER guard_settlement_return_truncate BEFORE TRUNCATE ON payments.settlement_payout_returns
 FOR EACH STATEMENT EXECUTE FUNCTION payments.guard_settlement_return();
COMMENT ON FUNCTION payments.guard_settlement_return() IS
 'payments owner (SA-1); BEFORE UPDATE OR DELETE OR TRUNCATE trigger of settlement_payout_returns: the only allowed change is carried_into_statement_id from NULL to a value with every other column equal; anything else is PT409 settlement_return_immutable. Moves no money';

-- ---------------------------------------------------------------------------------------------------------
-- 3. sync observation order and the stored reconciliation
-- ---------------------------------------------------------------------------------------------------------
-- Existing runs receive lower sequence values than every later run; gaps are allowed. Order among the pre-existing runs is arbitrary and
-- irrelevant: they are all grandfathered clean.
ALTER TABLE payments.settlement_sync_runs ADD COLUMN observation_seq bigint GENERATED ALWAYS AS IDENTITY UNIQUE;
COMMENT ON COLUMN payments.settlement_sync_runs.observation_seq IS
 'payments owner (SA-1, contract §6.8); allocated while record_settlement_lines holds the per-environment advisory lock, so a later committed observation has a greater value; close_settlement chooses the latest covering run per segment by this value, never by timestamp or id';

CREATE TABLE payments.settlement_sync_reconciliations (
 sync_run_id uuid NOT NULL REFERENCES payments.settlement_sync_runs(id),
 settle_currency text NOT NULL CHECK(settle_currency ~ '^[A-Z]{3}$'),
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,      -- the platform store's scope (= the sync run's)
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 stripe_net_minor bigint NOT NULL CHECK(abs(stripe_net_minor)<=1000000000000000),      -- sum of net of the run's manifest (SQL)
 recorded_net_minor bigint NOT NULL CHECK(abs(recorded_net_minor)<=1000000000000000),  -- sum of stored nets in the window (SQL)
 stripe_txn_count integer NOT NULL CHECK(stripe_txn_count>=0),
 recorded_txn_count integer NOT NULL CHECK(recorded_txn_count>=0),
 difference_minor bigint GENERATED ALWAYS AS (recorded_net_minor-stripe_net_minor) STORED,
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(sync_run_id,settle_currency),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
ALTER TABLE payments.settlement_sync_reconciliations ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments.settlement_sync_reconciliations FORCE ROW LEVEL SECURITY;
REVOKE ALL ON payments.settlement_sync_reconciliations FROM PUBLIC;
GRANT SELECT,INSERT ON payments.settlement_sync_reconciliations TO commerce_payment_registry_writer;
COMMENT ON TABLE payments.settlement_sync_reconciliations IS
 'payments owner (SA-1, contract §6.8); only commerce_payment_registry_writer definers touch it; APPEND-ONLY: one row per sync run and settlement currency comparing the run''s Stripe manifest total with the stored ledger total of the window (difference = recorded - stripe); no UPDATE, DELETE or TRUNCATE for anyone; close_settlement refuses while the latest covering observation of a segment has a non-zero difference; moves no money';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.sync_run_id IS 'payments owner; the sync run (settlement_sync_runs.id) this comparison belongs to';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.settle_currency IS 'payments owner; settlement currency of the compared totals (a currency present on either side gets a row)';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.tenant_id IS 'payments owner; the platform store tenant, same scope as the sync run';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.store_id IS 'payments owner; the platform store, same scope as the sync run';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.environment IS 'payments owner; SANDBOX or LIVE, same as the sync run';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.stripe_net_minor IS 'payments owner; sum of net over the run''s window manifest (every balance transaction Stripe returned for the window), computed by SQL';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.recorded_net_minor IS 'payments owner; sum of stored settle_net in the window: lines plus unattributed rows that have no line yet (each transaction once), computed by SQL';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.stripe_txn_count IS 'payments owner; number of manifest entries in this currency';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.recorded_txn_count IS 'payments owner; number of stored rows counted in recorded_net_minor';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.difference_minor IS 'payments owner; recorded_net_minor minus stripe_net_minor (0 = reconciled); generated';
COMMENT ON COLUMN payments.settlement_sync_reconciliations.recorded_at IS 'payments owner; server time of the comparison';
CREATE POLICY settlement_recon_rw ON payments.settlement_sync_reconciliations TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
COMMENT ON POLICY settlement_recon_rw ON payments.settlement_sync_reconciliations IS 'SA-1: platform scope only, like settlement_sync_rw; privileges are narrowed to SELECT and INSERT by the grants';

CREATE FUNCTION payments.guard_settlement_reconciliation() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 RAISE EXCEPTION 'settlement_reconciliation_immutable' USING ERRCODE='PT409';
END $$;
ALTER FUNCTION payments.guard_settlement_reconciliation() OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.guard_settlement_reconciliation() FROM PUBLIC;
CREATE TRIGGER guard_settlement_reconciliation BEFORE UPDATE OR DELETE ON payments.settlement_sync_reconciliations
 FOR EACH ROW EXECUTE FUNCTION payments.guard_settlement_reconciliation();
CREATE TRIGGER guard_settlement_reconciliation_truncate BEFORE TRUNCATE ON payments.settlement_sync_reconciliations
 FOR EACH STATEMENT EXECUTE FUNCTION payments.guard_settlement_reconciliation();
COMMENT ON FUNCTION payments.guard_settlement_reconciliation() IS
 'payments owner (SA-1); BEFORE UPDATE OR DELETE OR TRUNCATE trigger of settlement_sync_reconciliations: a stored comparison is never edited or removed (PT409 settlement_reconciliation_immutable). Moves no money';

-- ---------------------------------------------------------------------------------------------------------
-- 4. record_settlement_lines: ten-argument identity (p_window_manifest), the nine-argument identity is DROPPED
--    Patched from the live prosrc (0150 + the 0163 resolved-row patch), so nothing of either is lost.
-- ---------------------------------------------------------------------------------------------------------
DO $do$
DECLARE v_body text; v_needle text; v_repl text;
BEGIN
 SELECT p.prosrc INTO v_body FROM pg_proc p
  WHERE p.oid='payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text)'::regprocedure;
 IF v_body IS NULL THEN RAISE EXCEPTION 'payments.record_settlement_lines (nine arguments) is missing'; END IF;

 -- RSL1: three more variables
 v_needle := $n$ v_found boolean; f record; c record; v_window jsonb;$n$;
 v_repl := $r$ v_found boolean; f record; c record; v_window jsonb; v_run uuid; v_recon jsonb; v_diff jsonb;$r$;
 IF (length(v_body)-length(replace(v_body,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.record_settlement_lines has an unexpected shape for the 0200 RSL1 patch'; END IF;
 v_body := replace(v_body,v_needle,v_repl);

 -- RSL2: manifest validation, 22023 before any write. Required iff the window pair is given; at most 5000 entries; every entry exactly
 -- {id, currency, net} with a txn id, an upper-case ISO currency and an integer net of at most 1e11; ids unique.
 v_needle := $n$  RAISE EXCEPTION 'invalid settlement lines' USING ERRCODE='22023'; END IF;$n$;
 v_repl := $r$  RAISE EXCEPTION 'invalid settlement lines' USING ERRCODE='22023'; END IF;
 IF (p_window_from IS NULL)<>(p_window_manifest IS NULL)
  OR NOT coalesce(p_window_manifest IS NULL OR CASE WHEN jsonb_typeof(p_window_manifest)='array' THEN jsonb_array_length(p_window_manifest)<=5000
   ELSE false END,false) THEN
  RAISE EXCEPTION 'invalid settlement manifest' USING ERRCODE='22023'; END IF;
 IF p_window_manifest IS NOT NULL AND (EXISTS(SELECT 1 FROM jsonb_array_elements(p_window_manifest) zm(e) WHERE NOT (
   CASE WHEN jsonb_typeof(zm.e)<>'object' THEN false
   ELSE (SELECT count(*) FROM jsonb_object_keys(zm.e))=3 AND zm.e ?& ARRAY['id','currency','net']
    AND coalesce(jsonb_typeof(zm.e->'id')='string' AND zm.e->>'id' ~ '^txn_[A-Za-z0-9]{1,255}$'
     AND jsonb_typeof(zm.e->'currency')='string' AND zm.e->>'currency' ~ '^[A-Z]{3}$'
     AND jsonb_typeof(zm.e->'net')='number'
     AND CASE WHEN zm.e->>'net' ~ '^-?[0-9]{1,15}$' THEN abs((zm.e->>'net')::bigint)<=100000000000 ELSE false END,false) END))
  OR (SELECT count(DISTINCT zm.e->>'id') FROM jsonb_array_elements(p_window_manifest) zm(e))<>jsonb_array_length(p_window_manifest)) THEN
  RAISE EXCEPTION 'invalid settlement manifest' USING ERRCODE='22023'; END IF;$r$;
 IF (length(v_body)-length(replace(v_body,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.record_settlement_lines has an unexpected shape for the 0200 RSL2 patch'; END IF;
 v_body := replace(v_body,v_needle,v_repl);

 -- RSL3: the sync run id is needed by the reconciliation rows
 v_needle := $n$VALUES(p_tenant,p_store,p_environment,p_window_from,p_window_to,p_principal);$n$;
 v_repl := $r$VALUES(p_tenant,p_store,p_environment,p_window_from,p_window_to,p_principal) RETURNING id INTO v_run;$r$;
 IF (length(v_body)-length(replace(v_body,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.record_settlement_lines has an unexpected shape for the 0200 RSL3 patch'; END IF;
 v_body := replace(v_body,v_needle,v_repl);

 -- RSL4: store the comparison per settlement currency in the same transaction as the run (before the audit row). The recorded side is the
 -- 0150 UNION ALL / anti-join exactly: lines contribute stored settle_net; an unattributed row contributes stored settle_net only while it
 -- has no line; the same rows feed net and count. Never settle_amount - settle_fee (an unsupported row may have a NULL fee).
 v_needle := $n$INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)$n$;
 v_repl := $r$v_recon:=NULL; v_diff:=NULL;
 IF p_window_from IS NOT NULL THEN
  WITH zs AS (SELECT zm.e->>'currency' AS cur,sum((zm.e->>'net')::bigint)::bigint AS net,count(*)::integer AS n
    FROM jsonb_array_elements(p_window_manifest) zm(e) GROUP BY zm.e->>'currency'),
   zr AS (SELECT u.cur,sum(u.net)::bigint AS net,count(*)::integer AS n FROM (
     SELECT x.settle_currency AS cur,x.settle_net AS net FROM payments.settlement_lines x
      WHERE x.environment=p_environment AND x.txn_created_at>=p_window_from AND x.txn_created_at<p_window_to
     UNION ALL
     SELECT y.settle_currency,y.settle_net FROM payments.settlement_unattributed y
      WHERE y.environment=p_environment AND y.txn_created_at>=p_window_from AND y.txn_created_at<p_window_to
       AND NOT EXISTS(SELECT 1 FROM payments.settlement_lines z WHERE z.balance_txn_id=y.balance_txn_id)) u GROUP BY u.cur)
  INSERT INTO payments.settlement_sync_reconciliations(sync_run_id,settle_currency,tenant_id,store_id,environment,stripe_net_minor,
   recorded_net_minor,stripe_txn_count,recorded_txn_count)
  SELECT v_run,k.cur,p_tenant,p_store,p_environment,coalesce(zs.net,0),coalesce(zr.net,0),coalesce(zs.n,0),coalesce(zr.n,0)
   FROM (SELECT zs0.cur FROM zs zs0 UNION SELECT zr0.cur FROM zr zr0) k LEFT JOIN zs ON zs.cur=k.cur LEFT JOIN zr ON zr.cur=k.cur;
  SELECT coalesce(jsonb_agg(jsonb_build_object('currency',zq.settle_currency,'stripe_net_minor',zq.stripe_net_minor,
    'recorded_net_minor',zq.recorded_net_minor,'difference_minor',zq.difference_minor,'stripe_txn_count',zq.stripe_txn_count,
    'recorded_txn_count',zq.recorded_txn_count) ORDER BY zq.settle_currency),'[]'::jsonb),
   jsonb_object_agg(zq.settle_currency,zq.difference_minor) INTO v_recon,v_diff
   FROM payments.settlement_sync_reconciliations zq WHERE zq.sync_run_id=v_run;
 END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)$r$;
 IF (length(v_body)-length(replace(v_body,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.record_settlement_lines has an unexpected shape for the 0200 RSL4 patch'; END IF;
 v_body := replace(v_body,v_needle,v_repl);

 -- RSL5: the audit details carry the per-currency difference (numbers only); the result carries the run id and the stored comparison
 v_needle := $n$'rechecked',v_rechecked,'ticket',p_ticket)));$n$;
 v_repl := $r$'rechecked',v_rechecked,'ticket',p_ticket,'differences',v_diff)));$r$;
 IF (length(v_body)-length(replace(v_body,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.record_settlement_lines has an unexpected shape for the 0200 RSL5 patch'; END IF;
 v_body := replace(v_body,v_needle,v_repl);

 v_needle := $n$'rechecked',v_rechecked,'window_net',v_window));$n$;
 v_repl := $r$'rechecked',v_rechecked,'window_net',v_window,'sync_run_id',v_run,'reconciliation',v_recon));$r$;
 IF (length(v_body)-length(replace(v_body,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.record_settlement_lines has an unexpected shape for the 0200 RSL6 patch'; END IF;
 v_body := replace(v_body,v_needle,v_repl);

 EXECUTE 'CREATE FUNCTION payments.record_settlement_lines(p_tenant uuid,p_store uuid,p_principal uuid,p_environment text,p_lines jsonb,'
  || 'p_window_from timestamptz DEFAULT NULL,p_window_to timestamptz DEFAULT NULL,p_connection uuid DEFAULT NULL,p_ticket text DEFAULT NULL,'
  || 'p_window_manifest jsonb DEFAULT NULL) RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS '
  || quote_literal(v_body);
END $do$;
-- the nine-argument identity goes in the same transaction: no wrapper, no retained grant (adding a defaulted argument would NOT replace it)
DROP FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text);
ALTER FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text,jsonb) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text,jsonb) TO commerce_payment_registrar;
COMMENT ON FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text,jsonb) IS
 'payments owner (W4-S2, resolved-row rule 0163, manifest SA-1/0200); operator definer (stripe-admin settlement-sync, registrar only, platform store scope): records Stripe balance transactions as per-store ledger lines attributed in SQL from Stripe ids, cross-checked against payments.facts and refund_facts; unmapped or foreign rows go to settlement_unattributed; an identical re-sync re-checks lines that have no statement and retries an unattributed row except one with a 0163 resolution; the last chunk records the sync window (never ending inside the last 15 minutes) and REQUIRES p_window_manifest (every balance transaction Stripe returned for the window as {id,currency,net}: 22023 invalid settlement manifest, nothing written, when it is missing or invalid), from which SQL stores the per-currency comparison with the ledger (settlement_sync_reconciliations) in the same transaction. Idempotent by balance transaction id; changed content is PT409. Never pays anyone';

-- ---------------------------------------------------------------------------------------------------------
-- 5. close_settlement, patched IN PLACE (same signature, owner and grants): the sync_unreconciled gate and the return carry
-- ---------------------------------------------------------------------------------------------------------
DO $do$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
 v_fn := pg_get_functiondef('payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text)'::regprocedure);

 -- CS1: variables
 v_needle := $n$v_net bigint; v_count integer; v_sha bytea; v_assigned integer;$n$;
 v_repl := $r$v_net bigint; v_count integer; v_sha bytea; v_assigned integer; v_returned bigint:=0; v_ret_n integer:=0; v_ret_upd integer; v_blocking uuid[];$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0200 CS1 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);

 -- CS2: after the existing sync_required walk (a segment with no observation at all refuses first, even when another segment is unreconciled):
 -- for every segment of R = [period_start - 7 d, period_end) split at the clipped window boundaries, the covering run with the GREATEST
 -- observation_seq is chosen BEFORE any cleanliness filter; it is clean only if no reconciliation row of that run has a non-zero difference.
 -- A newer discrepancy therefore supersedes older clean coverage (including pre-0200 runs, which have no rows and start clean) and only over
 -- its overlap. Nothing has been written at this point.
 v_needle := $n$IF v_cur<v_end THEN RAISE EXCEPTION 'sync_required' USING ERRCODE='PT409'; END IF;$n$;
 v_repl := $r$IF v_cur<v_end THEN RAISE EXCEPTION 'sync_required' USING ERRCODE='PT409'; END IF;
 SELECT array_agg(zc.id ORDER BY zc.observation_seq) INTO v_blocking FROM (
  SELECT g.id,g.observation_seq FROM (
   SELECT DISTINCT ON (sg.zfrom) sg.zfrom,zr.id,zr.observation_seq FROM (
    SELECT p.zp AS zfrom,lead(p.zp) OVER (ORDER BY p.zp) AS zto FROM (
     SELECT v_start-interval '7 days' AS zp
     UNION SELECT v_end
     UNION SELECT greatest(x.window_from,v_start-interval '7 days') FROM payments.settlement_sync_runs x WHERE x.tenant_id=p_tenant
      AND x.store_id=p_store AND x.environment=p_environment AND x.window_to>v_start-interval '7 days' AND x.window_from<v_end
     UNION SELECT least(x.window_to,v_end) FROM payments.settlement_sync_runs x WHERE x.tenant_id=p_tenant
      AND x.store_id=p_store AND x.environment=p_environment AND x.window_to>v_start-interval '7 days' AND x.window_from<v_end) p) sg
   JOIN (SELECT x.id,x.observation_seq,x.window_from,x.window_to FROM payments.settlement_sync_runs x WHERE x.tenant_id=p_tenant
     AND x.store_id=p_store AND x.environment=p_environment AND x.window_to>v_start-interval '7 days' AND x.window_from<v_end) zr
    ON zr.window_from<=sg.zfrom AND zr.window_to>=sg.zto
   WHERE sg.zto IS NOT NULL
   ORDER BY sg.zfrom,zr.observation_seq DESC) g
  WHERE EXISTS(SELECT 1 FROM payments.settlement_sync_reconciliations rc WHERE rc.sync_run_id=g.id AND rc.difference_minor<>0)
  GROUP BY g.id,g.observation_seq) zc;
 IF v_blocking IS NOT NULL THEN
  RAISE EXCEPTION 'sync_unreconciled' USING ERRCODE='PT409', DETAIL=array_to_string(v_blocking[1:20],','); END IF;$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0200 CS2 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);

 -- CS3: the replay reads the stored returned_in_minor ...
 v_needle := $n$x.line_count,x.net_payable_minor INTO st$n$;
 v_repl := $r$x.line_count,x.net_payable_minor,x.returned_in_minor INTO st$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0200 CS3 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);

 -- CS4: ... and proves it equals the sum of the returns carried into the statement (else settlement_changed)
 v_needle := $n$OR v_count<>st.line_count THEN RAISE EXCEPTION 'settlement_changed' USING ERRCODE='PT409'; END IF;$n$;
 v_repl := $r$OR v_count<>st.line_count THEN RAISE EXCEPTION 'settlement_changed' USING ERRCODE='PT409'; END IF;
   SELECT coalesce(sum(zr.returned_minor),0) INTO v_returned FROM payments.settlement_payout_returns zr
    WHERE zr.tenant_id=t.tenant_id AND zr.store_id=t.store_id AND zr.carried_into_statement_id=st.id;
   IF v_returned<>st.returned_in_minor THEN RAISE EXCEPTION 'settlement_changed' USING ERRCODE='PT409'; END IF;$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0200 CS4 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);

 -- CS5: returned_in_minor = the sum of this store's uncarried returns whose source statement is an EARLIER period and whose returned_at is
 -- before this period's end (Asia/Taipei midnight), in the same environment; it is added to the net BEFORE the next carry is derived
 v_needle := $n$v_net:=v_captured-v_refunded-v_dispute+v_fee-v_pfee+v_carry;$n$;
 v_repl := $r$SELECT coalesce(sum(zr.returned_minor),0),count(*) INTO v_returned,v_ret_n FROM payments.settlement_payout_returns zr
   WHERE zr.tenant_id=t.tenant_id AND zr.store_id=t.store_id AND zr.environment=p_environment AND zr.carried_into_statement_id IS NULL
    AND zr.returned_at<v_end AND EXISTS(SELECT 1 FROM payments.settlement_statements zs WHERE zs.tenant_id=zr.tenant_id
     AND zs.store_id=zr.store_id AND zs.id=zr.statement_id AND zs.period_start<p_period_start);
  v_net:=v_captured-v_refunded-v_dispute+v_fee-v_pfee+v_carry+v_returned;$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0200 CS5 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);

 -- CS6/CS7: the statement INSERT carries returned_in_minor (the named net-identity CHECK holds by construction)
 v_needle := $n$lines_sha256,closed_by)$n$;
 v_repl := $r$lines_sha256,closed_by,returned_in_minor)$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0200 CS6 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);

 v_needle := $n$sp.platform_fee_bps,v_pfee,v_carry,v_net,v_count,v_sha,p_operator);$n$;
 v_repl := $r$sp.platform_fee_bps,v_pfee,v_carry,v_net,v_count,v_sha,p_operator,v_returned);$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0200 CS7 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);

 -- CS8: carry exactly the returns counted above into the new statement, set once; a different count means something moved under the lock
 v_needle := $n$IF v_assigned<>v_count THEN RAISE EXCEPTION 'settlement_changed' USING ERRCODE='PT409'; END IF;$n$;
 v_repl := $r$IF v_assigned<>v_count THEN RAISE EXCEPTION 'settlement_changed' USING ERRCODE='PT409'; END IF;
  UPDATE payments.settlement_payout_returns zr SET carried_into_statement_id=v_new
   WHERE zr.tenant_id=t.tenant_id AND zr.store_id=t.store_id AND zr.environment=p_environment AND zr.carried_into_statement_id IS NULL
    AND zr.returned_at<v_end AND EXISTS(SELECT 1 FROM payments.settlement_statements zs WHERE zs.tenant_id=zr.tenant_id
     AND zs.store_id=zr.store_id AND zs.id=zr.statement_id AND zs.period_start<p_period_start);
  GET DIAGNOSTICS v_ret_upd=ROW_COUNT;
  IF v_ret_upd<>v_ret_n THEN RAISE EXCEPTION 'settlement_changed' USING ERRCODE='PT409'; END IF;$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.close_settlement has an unexpected shape for the 0200 CS8 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);

 EXECUTE v_fn;
END $do$;
ALTER FUNCTION payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text) OWNER TO commerce_payment_registry_writer;
COMMENT ON FUNCTION payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text) IS
 'payments owner (W4-S2, resolve step 0163, returns and reconciliation gate SA-1/0200); operator definer (stripe-admin settlement-close, registrar only): closes the weekly Asia/Taipei statement of one store or of every store with lines or an earlier statement: captured minus refunded minus disputes plus Stripe fees (negative) minus platform fee plus a negative carry-in plus the payout returns carried in (returned_in_minor: uncarried returns of an earlier statement of the same store and environment, returned before the period end, each carried exactly once), assigns the lines once, refuses while the period is not +72 h old, a sync did not cover it (sync_required), the latest covering observation of any segment of the coverage interval has a non-zero difference against Stripe (sync_unreconciled, DETAIL = up to 20 run ids), a mismatch or an UNRESOLVED unmapped source exists (a 0163 resolution row clears that refusal; the resolved assigned_to_store rows of the closed period are returned as operator_notes and move no money). Idempotent per store and period. Records what the platform owes the store; moves no money';

-- ---------------------------------------------------------------------------------------------------------
-- 6. settlement_statement_json, patched IN PLACE: three additive merchant keys (returned_in_minor, payout_returned, payout_returned_at)
--    Still no return_ref, ticket, operator, Stripe id, settlement-currency amount or other store's data (S-I6).
-- ---------------------------------------------------------------------------------------------------------
DO $do$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
 v_fn := pg_get_functiondef('payments.settlement_statement_json(uuid,uuid,uuid,boolean)'::regprocedure);
 v_needle := $n$'carried_in_minor',x.carried_in_minor,'net_payable_minor',x.net_payable_minor,'line_count',x.line_count,$n$;
 v_repl := $r$'carried_in_minor',x.carried_in_minor,'returned_in_minor',x.returned_in_minor,'net_payable_minor',x.net_payable_minor,'line_count',x.line_count,$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.settlement_statement_json has an unexpected shape for the 0200 SJ1 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);
 v_needle := $n$'paid_at',x.paid_at,$n$;
 v_repl := $r$'paid_at',x.paid_at,'payout_returned',EXISTS(SELECT 1 FROM payments.settlement_payout_returns pr WHERE pr.tenant_id=x.tenant_id
      AND pr.store_id=x.store_id AND pr.statement_id=x.id),
   'payout_returned_at',(SELECT pr2.returned_at FROM payments.settlement_payout_returns pr2 WHERE pr2.tenant_id=x.tenant_id
      AND pr2.store_id=x.store_id AND pr2.statement_id=x.id),$r$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.settlement_statement_json has an unexpected shape for the 0200 SJ2 patch'; END IF;
 v_fn := replace(v_fn,v_needle,v_repl);
 EXECUTE v_fn;
END $do$;
ALTER FUNCTION payments.settlement_statement_json(uuid,uuid,uuid,boolean) OWNER TO commerce_payment_registry_writer;
COMMENT ON FUNCTION payments.settlement_statement_json(uuid,uuid,uuid,boolean) IS
 'payments owner (W4-S2, return keys SA-1/0200); helper called by read_store_settlements and read_settlement_statement: one store''s statements as JSON in the scope the caller pinned, optional lines (order number, kind, store-currency amounts, Taipei date), plus returned_in_minor, payout_returned and payout_returned_at (never the return reference or ticket). No Stripe ids, no settlement-currency amounts, explicit tenant/store predicates. Read only';

-- ---------------------------------------------------------------------------------------------------------
-- 7. record_settlement_payout_return (RECORD ONLY: a return is a fact the owner already saw at the bank)
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.record_settlement_payout_return(p_tenant uuid,p_store uuid,p_principal uuid,p_statement uuid,p_return_ref text,
 p_returned_minor bigint,p_returned_at timestamptz,p_operator text,p_ticket text) RETURNS timestamptz
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE st payments.settlement_statements%ROWTYPE; sp payments.stripe_platform%ROWTYPE; rt payments.settlement_payout_returns%ROWTYPE;
 v_env text; v_at timestamptz; v_tenant uuid; v_store uuid;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 -- structural checks (22023) before any read
 IF p_statement IS NULL OR p_return_ref IS NULL OR p_return_ref !~ '^[A-Za-z0-9._:/-]{4,80}$'
  OR p_returned_minor IS NULL OR p_returned_minor NOT BETWEEN 1 AND 100000000000 OR p_returned_at IS NULL
  OR p_operator IS NULL OR p_operator !~ '^[A-Za-z0-9._:@-]{2,64}$'
  OR p_ticket IS NULL OR p_ticket !~ '^[A-Za-z0-9._:-]{8,128}$' THEN
  RAISE EXCEPTION 'invalid settlement payout return' USING ERRCODE='22023'; END IF;
 PERFORM set_config('app.settlement_op','on',true);
 -- the target store and environment come from the statement row (never from input): read the environment WITHOUT a lock, check it belongs to
 -- the caller's platform, take the per-environment advisory lock (the same as sync and close), then re-read the row FOR UPDATE
 SELECT x.environment INTO v_env FROM payments.settlement_statements x WHERE x.id=p_statement;
 IF NOT FOUND THEN RAISE EXCEPTION 'statement_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=v_env;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'statement_unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM pg_advisory_xact_lock(hashtextextended('lc.settlement.'||v_env,0));
 -- the row lock is taken in the statement's own store scope (the UPDATE policy of the statements table is scope-bound)
 SELECT x.tenant_id,x.store_id INTO v_tenant,v_store FROM payments.settlement_statements x WHERE x.id=p_statement;
 PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_store::text,true);
 SELECT x.* INTO st FROM payments.settlement_statements x WHERE x.id=p_statement FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'statement_unavailable' USING ERRCODE='PT409'; END IF;
 IF st.payout_ref IS NULL THEN RAISE EXCEPTION 'payout_not_recorded' USING ERRCODE='PT409'; END IF;
 -- replay: the identical record returns the stored time (no second row, no second audit); any other input is a conflict
 SELECT x.* INTO rt FROM payments.settlement_payout_returns x WHERE x.statement_id=p_statement;
 IF FOUND THEN
  IF rt.return_ref=p_return_ref AND rt.returned_minor=p_returned_minor AND rt.returned_at=p_returned_at
   AND rt.recorded_by=p_operator AND rt.ticket=p_ticket THEN
   PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
   RETURN rt.recorded_at; END IF;
  RAISE EXCEPTION 'return_already_recorded' USING ERRCODE='PT409';
 END IF;
 -- the amount only CONFIRMS the payout; it is never an amount source and partial returns are not supported in v1
 IF p_returned_minor<>st.payout_minor THEN RAISE EXCEPTION 'return_amount_mismatch' USING ERRCODE='PT409'; END IF;
 IF p_returned_at<st.paid_at OR p_returned_at>clock_timestamp() THEN
  RAISE EXCEPTION 'invalid settlement payout return' USING ERRCODE='22023'; END IF;
 INSERT INTO payments.settlement_payout_returns(statement_id,tenant_id,store_id,environment,return_ref,returned_minor,returned_at,recorded_by,ticket)
  VALUES(st.id,st.tenant_id,st.store_id,st.environment,p_return_ref,p_returned_minor,p_returned_at,p_operator,p_ticket)
  RETURNING recorded_at INTO v_at;
 PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
 VALUES(p_tenant,p_store,p_principal,'stripe.settlement.payout_return',jsonb_build_object('environment',st.environment,
  'target_store',st.store_id,'statement_id',st.id,'returned_minor',p_returned_minor,'ticket',p_ticket));
 RETURN v_at;
END $$;
ALTER FUNCTION payments.record_settlement_payout_return(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.record_settlement_payout_return(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.record_settlement_payout_return(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text) TO commerce_payment_registrar;
COMMENT ON FUNCTION payments.record_settlement_payout_return(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text) IS
 'payments owner (SA-1, contract §6.7); operator definer (stripe-admin settlement-payout-return, registrar only, platform store scope): RECORDS that the payout of a PAID statement bounced (append-only, one per statement, amount must equal payout_minor, returned_at not before paid_at and not in the future); the money is carried into the store''s next statement by close_settlement as returned_in_minor. Reads the statement environment, takes the per-environment advisory lock, then locks the row; an identical replay returns the stored time with no second row or audit; the audit row never carries the return reference. Calls no bank or Stripe API and moves no money';

-- ---------------------------------------------------------------------------------------------------------
-- 8. read_settlement_payables (read only: who is owed money, oldest first; paying stays the owner's bank action)
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.read_settlement_payables(p_tenant uuid,p_store uuid,p_principal uuid,p_environment text,p_limit integer) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE sp payments.stripe_platform%ROWTYPE; v_rows jsonb; v_n integer;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_environment IS NULL OR p_environment NOT IN ('SANDBOX','LIVE') OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 200 THEN
  RAISE EXCEPTION 'invalid settlement payables' USING ERRCODE='22023'; END IF;
 -- an environment that has no platform account cannot hold statements: nothing is owed there. Another platform's environment is not readable.
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND THEN RETURN jsonb_build_object('payables','[]'::jsonb,'truncated',false); END IF;
 IF sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.settlement_op','on',true);
 SELECT count(*) INTO v_n FROM (SELECT 1 FROM payments.settlement_statements x WHERE x.environment=p_environment
  AND x.net_payable_minor>0 AND x.payout_ref IS NULL LIMIT p_limit+1) z;
 SELECT coalesce(jsonb_agg(jsonb_build_object('statement_id',q.id,'tenant_id',q.tenant_id,'store_id',q.store_id,'period_start',q.period_start,
   'currency',q.currency,'net_payable_minor',q.net_payable_minor,'closed_at',q.closed_at) ORDER BY q.closed_at,q.id),'[]'::jsonb) INTO v_rows
  FROM (SELECT x.id,x.tenant_id,x.store_id,x.period_start,x.currency,x.net_payable_minor,x.closed_at FROM payments.settlement_statements x
   WHERE x.environment=p_environment AND x.net_payable_minor>0 AND x.payout_ref IS NULL ORDER BY x.closed_at,x.id LIMIT p_limit) q;
 RETURN jsonb_build_object('payables',v_rows,'truncated',v_n>p_limit);
END $$;
ALTER FUNCTION payments.read_settlement_payables(uuid,uuid,uuid,text,integer) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.read_settlement_payables(uuid,uuid,uuid,text,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.read_settlement_payables(uuid,uuid,uuid,text,integer) TO commerce_payment_registrar;
COMMENT ON FUNCTION payments.read_settlement_payables(uuid,uuid,uuid,text,integer) IS
 'payments owner (SA-1, contract §6.9); operator definer (stripe-admin settlement-payables, registrar only, platform store scope): READ ONLY (no audit row, no lock): the environment''s statements with net_payable_minor > 0 and no payout recorded, oldest closed first, at most p_limit (1..200), truncated=true when more exist; each entry is statement_id, tenant_id, store_id, period_start, currency, net_payable_minor, closed_at (no PII, no Stripe id, no bank data). A statement whose payout bounced keeps its payout triple and is paid through the statement that carries it. Moves no money';

-- ---------------------------------------------------------------------------------------------------------
-- 9. §8 tw_legal_opinion: the two 0137 checklist arrays, patched IN PLACE (same signatures, owners and grants)
--    approve_stripe_live accepts the base 11 codes or the platform 16 (was 15); set_stripe_platform_open on LIVE requires all five
--    platform codes. An approval stored before 0200 with the old 15 codes no longer opens LIVE: the owner re-attests (§8).
-- ---------------------------------------------------------------------------------------------------------
DO $do$
DECLARE v_fn text; v_needle text;
BEGIN
 v_fn := pg_get_functiondef('payments.approve_stripe_live(uuid,uuid,uuid,uuid,uuid,text,text,timestamptz,bigint,bigint,text[],jsonb)'::regprocedure);
 v_needle := $n$'platform_stripe_terms','tax_invoice']) c ORDER BY c COLLATE "C");$n$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.approve_stripe_live has an unexpected shape for the 0200 checklist patch'; END IF;
 EXECUTE replace(v_fn,v_needle,$r$'platform_stripe_terms','tax_invoice','tw_legal_opinion']) c ORDER BY c COLLATE "C");$r$);

 v_fn := pg_get_functiondef('payments.set_stripe_platform_open(uuid,uuid,uuid,text,boolean,integer,bigint)'::regprocedure);
 v_needle := $n$ap.checklist @> ARRAY['merchant_terms','payout_ops','platform_stripe_terms','tax_invoice']::text[]$n$;
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'payments.set_stripe_platform_open has an unexpected shape for the 0200 checklist patch'; END IF;
 EXECUTE replace(v_fn,v_needle,$r$ap.checklist @> ARRAY['merchant_terms','payout_ops','platform_stripe_terms','tax_invoice','tw_legal_opinion']::text[]$r$);
END $do$;
ALTER FUNCTION payments.approve_stripe_live(uuid,uuid,uuid,uuid,uuid,text,text,timestamptz,bigint,bigint,text[],jsonb) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.set_stripe_platform_open(uuid,uuid,uuid,text,boolean,integer,bigint) OWNER TO commerce_payment_registry_writer;
