-- 0150 per-store settlement ledger of the platform Stripe account (contracts/stripe-platform-account-v1.md §4.4, §6; W4-S2).
-- Purpose: money that the platform Stripe account collects for allowlisted stores is settled with each store off-Stripe from a
--   per-store statement. Lines come ONLY from Stripe balance-transaction reads (operator CLI) cross-checked against the worker facts;
--   attribution to (tenant, store, order) happens here in SQL from Stripe ids, never from input scope or metadata.
-- Depends on: 0061/0062 (stripe_sessions, stripe_refunds, refund_facts), 0018 (facts), 0016 (payment_attempts), 0137 (stripe_platform,
--   derived merchant_accounts, require_stripe_registrar_scope via 0061), 0003 (identity.resolve_access), ops.audit_events.
-- Used by: internal/payments/stripeadmin (settlement.go: sync, close, payout, statement read), internal/payments/settlement (merchant
--   read, CSV), cmd/stripe-admin settlement-* (operator), internal/httpapi/settlements.go (merchant GET).
-- Invariants: I05 (a line needs the attempt's account/currency/amount to match the worker fact, else mismatch blocks close), I06/I20
--   (ledger rows keyed by Stripe balance transaction id, replays insert nothing), I19 (PostgreSQL is the single source of truth).
-- Contract deltas (named, each listed in output/w4-s2-platform-settlement/DELIVERY.md):
--   * lines also keep order_id and payload_sha256; unattributed rows also keep tenant/store (the platform scope), payload_sha256;
--   * a sync-coverage table (settlement_sync_runs) because close requires "a sync covered [period_start-7d, period_end)" (§6.2);
--   * record_settlement_lines takes an optional window pair (default NULL) that records the coverage and returns the window net;
--   * read_store_settlements takes an optional statement id (default NULL) for the single-statement route; the list has no lines;
--   * read_settlement_statement is the operator read for settlement-export (no contract function reads lines for the operator);
--   * (fix round, Opus review) a line with no statement yet is RE-CHECKED by an identical re-sync (mismatch and its fee share may change,
--     nothing else), a REFUND is cross-checked against payments.stripe_refunds once the refund has ANY terminal fact, a sync window may not
--     end inside the last 15 minutes or in whole fractions of a second, the platform connection and the operator ticket are optional
--     arguments (assertion / audit), an all-store close gives every store with an earlier statement a (possibly empty) statement, and the
--     fee share is rounded half away from zero on the absolute value so a refund exactly reverses its charge's share.
-- Status: MOCK + REAL_PG only. No Stripe call, no bank or payout API: a payout is only RECORDED (operator-entered reference).

-- ---------------------------------------------------------------------------------------------------------
-- Tables (FORCE RLS, PUBLIC revoked, no DELETE grant to anyone; the registry writer is the only role with a privilege)
-- ---------------------------------------------------------------------------------------------------------
CREATE TABLE payments.settlement_statements (
 id uuid PRIMARY KEY,
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 period_start date NOT NULL CHECK(extract(isodow FROM period_start)=1),
 period_end date NOT NULL,
 currency text NOT NULL CHECK(currency ~ '^[A-Z]{3}$'),
 captured_minor bigint NOT NULL,refunded_minor bigint NOT NULL,dispute_minor bigint NOT NULL,stripe_fee_minor bigint NOT NULL,
 platform_fee_bps integer NOT NULL CHECK(platform_fee_bps BETWEEN 0 AND 3000),
 platform_fee_minor bigint NOT NULL CHECK(platform_fee_minor>=0),
 carried_in_minor bigint NOT NULL CHECK(carried_in_minor<=0),
 net_payable_minor bigint NOT NULL,
 line_count integer NOT NULL CHECK(line_count>=0),
 lines_sha256 bytea NOT NULL CHECK(octet_length(lines_sha256)=32),
 closed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 closed_by text NOT NULL CHECK(closed_by ~ '^[A-Za-z0-9._:@-]{2,64}$'),
 payout_ref text CHECK(payout_ref ~ '^[A-Za-z0-9._:/-]{4,80}$'),
 payout_minor bigint,paid_at timestamptz,
 paid_recorded_by text CHECK(paid_recorded_by ~ '^[A-Za-z0-9._:@-]{2,64}$'),
 UNIQUE(tenant_id,store_id,environment,period_start),
 UNIQUE(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 CHECK(period_end=period_start+7),
 CHECK(net_payable_minor=captured_minor-refunded_minor-dispute_minor+stripe_fee_minor-platform_fee_minor+carried_in_minor),
 CHECK((payout_ref IS NULL)=(paid_at IS NULL) AND (payout_ref IS NULL)=(payout_minor IS NULL)
  AND (payout_ref IS NULL)=(paid_recorded_by IS NULL)),
 CHECK(payout_minor IS NULL OR (net_payable_minor>0 AND payout_minor=net_payable_minor))
);

CREATE TABLE payments.settlement_lines (
 balance_txn_id text PRIMARY KEY CHECK(balance_txn_id ~ '^txn_[A-Za-z0-9]{1,255}$'),
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 attempt_id uuid NOT NULL,order_id uuid NOT NULL,
 refund_id uuid,
 dispute_id text CHECK(dispute_id ~ '^dp_[A-Za-z0-9]{1,255}$'),
 kind text NOT NULL CHECK(kind IN ('CHARGE','REFUND','REFUND_FAILURE','DISPUTE','DISPUTE_REVERSAL')),
 store_currency text NOT NULL CHECK(store_currency ~ '^[A-Z]{3}$'),
 store_minor bigint NOT NULL,                       -- signed presentment amount in the store currency (Stripe source object)
 settle_currency text NOT NULL CHECK(settle_currency ~ '^[A-Z]{3}$'),
 settle_amount bigint NOT NULL,settle_fee bigint NOT NULL,settle_net bigint NOT NULL,
 fee_store_minor bigint NOT NULL,                   -- contract §6.3, signed (a fee is a cost: <= 0 normally)
 txn_created_at timestamptz NOT NULL,
 synced_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 mismatch text CHECK(mismatch IN ('amount','currency','no_fact')),
 payload_sha256 bytea NOT NULL CHECK(octet_length(payload_sha256)=32),
 statement_id uuid,                                  -- set once at close
 CHECK(settle_amount-settle_fee=settle_net),         -- Stripe's own identity
 CHECK((kind IN ('CHARGE','REFUND_FAILURE','DISPUTE_REVERSAL') AND store_minor>=0)
  OR (kind IN ('REFUND','DISPUTE') AND store_minor<=0)),
 CHECK((kind IN ('REFUND','REFUND_FAILURE'))=(refund_id IS NOT NULL)),
 CHECK((kind IN ('DISPUTE','DISPUTE_REVERSAL'))=(dispute_id IS NOT NULL)),
 FOREIGN KEY(tenant_id,store_id,attempt_id) REFERENCES integration.operations(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,statement_id) REFERENCES payments.settlement_statements(tenant_id,store_id,id)
);
CREATE INDEX settlement_lines_open ON payments.settlement_lines(tenant_id,store_id,environment,txn_created_at) WHERE statement_id IS NULL;
CREATE INDEX settlement_lines_statement ON payments.settlement_lines(tenant_id,store_id,statement_id,txn_created_at,balance_txn_id);
CREATE INDEX settlement_lines_attempt ON payments.settlement_lines(tenant_id,store_id,attempt_id,kind);
CREATE INDEX settlement_lines_window ON payments.settlement_lines(environment,txn_created_at);

-- Anything the sync could not attribute to exactly one store (contract §4.4). Written in the PLATFORM store's scope.
CREATE TABLE payments.settlement_unattributed (
 balance_txn_id text PRIMARY KEY CHECK(balance_txn_id ~ '^txn_[A-Za-z0-9]{1,255}$'),
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 type text NOT NULL CHECK(char_length(type) BETWEEN 1 AND 64),
 settle_currency text NOT NULL CHECK(settle_currency ~ '^[A-Z]{3}$'),
 settle_amount bigint NOT NULL,
 settle_fee bigint,                                  -- NULL = Stripe sent none (fails closed, §6.3)
 settle_net bigint NOT NULL,
 txn_created_at timestamptz NOT NULL,
 reason text NOT NULL CHECK(reason IN ('unmapped_source','foreign_connection','unsupported_type')),
 payload_sha256 bytea NOT NULL CHECK(octet_length(payload_sha256)=32),
 synced_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);
CREATE INDEX settlement_unattributed_window ON payments.settlement_unattributed(environment,txn_created_at);

-- One row per finished sync (the final call of a run carries the window). Close requires the union of the runs to cover
-- [period_start - 7 d, period_end) (contract §6.2).
CREATE TABLE payments.settlement_sync_runs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 tenant_id uuid NOT NULL,store_id uuid NOT NULL,
 environment text NOT NULL CHECK(environment IN ('SANDBOX','LIVE')),
 window_from timestamptz NOT NULL,window_to timestamptz NOT NULL CHECK(window_to>window_from),
 synced_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 synced_by uuid NOT NULL,
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id)
);

-- A helper index for the cross-store attribution lookups below (a payment intent id maps to exactly one Stripe session).
-- NOT unique on purpose (CI regression, TestStripeSP12MoneyChecks): payments.stripe_sessions belongs to the payments unit and its recording path
-- must never start failing on a provider anomaly (a repeated PI id would make stripe_record_failed loop forever). The attribution lookup below
-- therefore treats a payment intent shared by more than one session as AMBIGUOUS: unmapped_source (blocks close, escalate), never a guess.
CREATE INDEX stripe_sessions_payment_intent_idx ON payments.stripe_sessions(environment,payment_intent_id) WHERE payment_intent_id IS NOT NULL;

DO $$ DECLARE v_table text; v_col record; BEGIN
 FOREACH v_table IN ARRAY ARRAY['settlement_statements','settlement_lines','settlement_unattributed','settlement_sync_runs'] LOOP
  EXECUTE format('ALTER TABLE payments.%I ENABLE ROW LEVEL SECURITY',v_table);
  EXECUTE format('ALTER TABLE payments.%I FORCE ROW LEVEL SECURITY',v_table);
  EXECUTE format('REVOKE ALL ON payments.%I FROM PUBLIC',v_table);
  EXECUTE format('COMMENT ON TABLE payments.%I IS %L',v_table,
   'payments owner (internal/payments/stripeadmin settlement.go, cmd/stripe-admin settlement-*); only commerce_payment_registry_writer definers touch it; no worker, ingress, checkout or integration role; no delete; never PSP authority');
  FOR v_col IN SELECT column_name FROM information_schema.columns WHERE table_schema='payments' AND table_name=v_table LOOP
   EXECUTE format('COMMENT ON COLUMN payments.%I.%I IS %L',v_table,v_col.column_name,
    'payments owner; per-store platform settlement ledger (contracts/stripe-platform-account-v1.md §6.1); money facts come from Stripe balance-transaction reads only');
  END LOOP;
 END LOOP;
END $$;

GRANT SELECT,INSERT ON payments.settlement_lines,payments.settlement_statements,payments.settlement_unattributed,
 payments.settlement_sync_runs TO commerce_payment_registry_writer;
GRANT UPDATE(statement_id,mismatch,fee_store_minor) ON payments.settlement_lines TO commerce_payment_registry_writer;
GRANT UPDATE(payout_ref,payout_minor,paid_at,paid_recorded_by) ON payments.settlement_statements TO commerce_payment_registry_writer;

-- RLS. The definers pin the scope GUCs (tenant/store). The operator definers ALSO set app.settlement_op='on' for their transaction,
-- which opens READ across stores (duplicate detection by balance txn id, close and payout lookups). The merchant reader never sets
-- it, so its rows are scope-limited by RLS as well as by its own WHERE. Writes always name their own scope.
CREATE POLICY settlement_line_read ON payments.settlement_lines FOR SELECT TO commerce_payment_registry_writer
 USING((tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
  OR current_setting('app.settlement_op',true)='on');
CREATE POLICY settlement_line_insert ON payments.settlement_lines FOR INSERT TO commerce_payment_registry_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY settlement_line_update ON payments.settlement_lines FOR UPDATE TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY settlement_statement_read ON payments.settlement_statements FOR SELECT TO commerce_payment_registry_writer
 USING((tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
  OR current_setting('app.settlement_op',true)='on');
CREATE POLICY settlement_statement_insert ON payments.settlement_statements FOR INSERT TO commerce_payment_registry_writer
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY settlement_statement_update ON payments.settlement_statements FOR UPDATE TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY settlement_unattributed_rw ON payments.settlement_unattributed TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
CREATE POLICY settlement_sync_rw ON payments.settlement_sync_runs TO commerce_payment_registry_writer
 USING(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid)
 WITH CHECK(tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
-- The unattributed and sync-run policies above grant ALL commands; narrow the table privileges instead of the policy (no UPDATE/DELETE
-- privilege was granted, so only SELECT and INSERT can ever pass).

-- Attribution reads (column-limited; the op GUC is the only way a policy admits them across stores). stripe_sessions and stripe_refunds
-- are owned by the payments unit: the registry writer gets the Stripe ids and the account only, never a URL, parameters or hash.
GRANT SELECT(tenant_id,store_id,attempt_id,environment,account_id,payment_intent_id) ON payments.stripe_sessions TO commerce_payment_registry_writer;
CREATE POLICY settlement_session_lookup ON payments.stripe_sessions FOR SELECT TO commerce_payment_registry_writer
 USING(current_setting('app.settlement_op',true)='on');
GRANT SELECT(stripe_refund_id,account_id,amount_minor,currency) ON payments.stripe_refunds TO commerce_payment_registry_writer;
CREATE POLICY settlement_refund_lookup ON payments.stripe_refunds FOR SELECT TO commerce_payment_registry_writer
 USING(current_setting('app.settlement_op',true)='on');
COMMENT ON POLICY settlement_session_lookup ON payments.stripe_sessions IS 'W4-S2: attribution lookup by payment intent for the settlement operator definers only; open only while app.settlement_op=on. That GUC is a session value any caller of a registry-writer definer could set, so EVERY reader of a settlement table must keep its explicit tenant/store predicate (settlement_statement_json and read_store_settlements do)';
COMMENT ON POLICY settlement_refund_lookup ON payments.stripe_refunds IS 'W4-S2: attribution lookup by Stripe refund id (id, account, amount, currency) for the settlement operator definers only; same app.settlement_op caveat as settlement_session_lookup';
COMMENT ON POLICY settlement_line_read ON payments.settlement_lines IS 'W4-S2: own scope, or every store while app.settlement_op=on (operator duplicate detection, close, payout). Readers must keep explicit tenant/store predicates';
COMMENT ON POLICY settlement_statement_read ON payments.settlement_statements IS 'W4-S2: own scope, or every store while app.settlement_op=on (operator close/payout/export). Readers must keep explicit tenant/store predicates';
-- The order of a line comes from the attempt (read in the line's own scope through the existing stripe_registry_attempt_read policy).
GRANT SELECT(order_id) ON checkout.payment_attempts TO commerce_payment_registry_writer;

-- ---------------------------------------------------------------------------------------------------------
-- Set-once triggers
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.guard_settlement_line() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 -- A line is frozen once it belongs to a statement. Before that, an identical re-sync may re-check it: only mismatch (a fact that was late)
 -- and fee_store_minor (a dispute whose charge line was late) may change, and statement_id moves NULL -> value exactly once (close).
 IF OLD.statement_id IS NOT NULL
  OR (to_jsonb(NEW)-'statement_id'-'mismatch'-'fee_store_minor') IS DISTINCT FROM (to_jsonb(OLD)-'statement_id'-'mismatch'-'fee_store_minor') THEN
  RAISE EXCEPTION 'settlement_line_immutable' USING ERRCODE='PT409'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_settlement_line BEFORE UPDATE ON payments.settlement_lines
 FOR EACH ROW EXECUTE FUNCTION payments.guard_settlement_line();

CREATE FUNCTION payments.guard_settlement_statement() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog AS $$
BEGIN
 -- the payout quadruple is set once; everything else is frozen at close
 IF (to_jsonb(NEW)-'payout_ref'-'payout_minor'-'paid_at'-'paid_recorded_by')
  IS DISTINCT FROM (to_jsonb(OLD)-'payout_ref'-'payout_minor'-'paid_at'-'paid_recorded_by')
  OR (OLD.payout_ref IS NOT NULL AND (NEW.payout_ref,NEW.payout_minor,NEW.paid_at,NEW.paid_recorded_by)
   IS DISTINCT FROM (OLD.payout_ref,OLD.payout_minor,OLD.paid_at,OLD.paid_recorded_by)) THEN
  RAISE EXCEPTION 'settlement_statement_immutable' USING ERRCODE='PT409'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER guard_settlement_statement BEFORE UPDATE ON payments.settlement_statements
 FOR EACH ROW EXECUTE FUNCTION payments.guard_settlement_statement();

-- ---------------------------------------------------------------------------------------------------------
-- Helpers (owner registry writer, no EXECUTE for anyone else: they are only called from the definers below)
-- ---------------------------------------------------------------------------------------------------------
-- Balance transaction -> ledger kind. Verified against docs.stripe.com 2026-10-06: card charges and refunds are type charge|refund
-- (reporting_category charge|refund); disputes and their reversals are type=adjustment whose source is a dispute. A dispute-sourced
-- adjustment whose reporting_category/sign is not the known pair is DISPUTE_UNKNOWN (an unmapped source that blocks close), never silently
-- dropped, because skipping a dispute would overpay the store.
CREATE FUNCTION payments.settlement_kind(p_type text,p_rc text,p_source_object text,p_amount bigint) RETURNS text
LANGUAGE sql IMMUTABLE SET search_path=pg_catalog AS $$
 SELECT CASE
  WHEN p_rc='charge' AND p_type IN ('charge','payment') THEN 'CHARGE'
  WHEN p_rc='refund' AND p_type IN ('refund','payment_refund') THEN 'REFUND'
  WHEN p_type='refund_failure' OR (p_type='adjustment' AND p_rc='refund_failure') THEN 'REFUND_FAILURE'
  WHEN p_type='adjustment' AND p_source_object='dispute' AND p_rc='dispute' AND p_amount<0 THEN 'DISPUTE'
  WHEN p_type='adjustment' AND p_source_object='dispute' AND p_rc='dispute_reversal' AND p_amount>0 THEN 'DISPUTE_REVERSAL'
  WHEN p_type='adjustment' AND p_source_object='dispute' THEN 'DISPUTE_UNKNOWN'
 END
$$;

-- §6.3: fee_store_minor = -step * sign(q) * floor(|q| + 1/2), q = fee * store_minor / settle_amount / step, step 100 (whole NT$): half away
-- from zero on the ABSOLUTE value, then the sign (Opus review P2-4), so a refund (negative store and settle amounts, positive ratio) reverses
-- its charge's fee share exactly and a returned (negative) fee credits the same magnitude a positive one debits. Exact integer arithmetic on
-- the absolute values: 2*|fee*store| + |settle|*step over 2*|settle|*step, truncated; no rounding of a quotient is involved.
CREATE FUNCTION payments.settlement_fee_store(p_fee bigint,p_store_minor bigint,p_settle_amount bigint) RETURNS bigint
LANGUAGE sql IMMUTABLE SET search_path=pg_catalog AS $$
 SELECT (-100 * sign(p_fee::numeric * p_store_minor::numeric * p_settle_amount::numeric)
  * div(2 * abs(p_fee::numeric * p_store_minor::numeric) + abs(p_settle_amount::numeric) * 100, 2 * abs(p_settle_amount::numeric) * 100))::bigint
$$;

-- One statement as the merchant/operator JSON. Rows are read in the CURRENT scope GUCs (the callers pin them); no txn ids, no
-- settlement-currency amounts, no other store's rows (the explicit tenant/store predicate holds even when app.settlement_op is on).
CREATE FUNCTION payments.settlement_statement_json(p_tenant uuid,p_store uuid,p_statement uuid,p_with_lines boolean) RETURNS jsonb
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT coalesce(jsonb_agg(s.j ORDER BY s.ps DESC),'[]'::jsonb) FROM (
  SELECT x.period_start AS ps,jsonb_strip_nulls(jsonb_build_object(
   'statement_id',x.id,'period_start',x.period_start,'period_end',x.period_end,'currency',x.currency,
   'captured_minor',x.captured_minor,'refunded_minor',x.refunded_minor,'dispute_minor',x.dispute_minor,
   'stripe_fee_minor',x.stripe_fee_minor,'platform_fee_bps',x.platform_fee_bps,'platform_fee_minor',x.platform_fee_minor,
   'carried_in_minor',x.carried_in_minor,'net_payable_minor',x.net_payable_minor,'line_count',x.line_count,
   'closed_at',x.closed_at,'paid',x.payout_ref IS NOT NULL,'payout_ref',x.payout_ref,'payout_minor',x.payout_minor,'paid_at',x.paid_at,
   'lines',CASE WHEN p_with_lines THEN (SELECT coalesce(jsonb_agg(jsonb_build_object(
      'order_number','LC-'||upper(replace(l.order_id::text,'-','')),'kind',l.kind,'store_minor',l.store_minor,
      'fee_store_minor',l.fee_store_minor,'txn_date',to_char((l.txn_created_at AT TIME ZONE 'Asia/Taipei')::date,'YYYY-MM-DD'))
      ORDER BY l.txn_created_at,l.balance_txn_id),'[]'::jsonb)
     FROM payments.settlement_lines l WHERE l.tenant_id=x.tenant_id AND l.store_id=x.store_id AND l.statement_id=x.id) END)) AS j
  FROM payments.settlement_statements x
  WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND (p_statement IS NULL OR x.id=p_statement)) s
$$;

ALTER FUNCTION payments.guard_settlement_line() OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.guard_settlement_statement() OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.settlement_kind(text,text,text,bigint) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.settlement_fee_store(bigint,bigint,bigint) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.settlement_statement_json(uuid,uuid,uuid,boolean) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.guard_settlement_line(),payments.guard_settlement_statement(),
 payments.settlement_kind(text,text,text,bigint),payments.settlement_fee_store(bigint,bigint,bigint),
 payments.settlement_statement_json(uuid,uuid,uuid,boolean) FROM PUBLIC;

-- ---------------------------------------------------------------------------------------------------------
-- record_settlement_lines (operator CLI, scope = the PLATFORM store)
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.record_settlement_lines(p_tenant uuid,p_store uuid,p_principal uuid,p_environment text,p_lines jsonb,
 p_window_from timestamptz DEFAULT NULL,p_window_to timestamptz DEFAULT NULL,p_connection uuid DEFAULT NULL,p_ticket text DEFAULT NULL) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 sp payments.stripe_platform%ROWTYPE; l jsonb; v_pass integer;
 v_inserted integer:=0; v_dup integer:=0; v_unattr integer:=0; v_mismatch integer:=0; v_rechecked integer:=0;
 v_n integer; v_recheck boolean; v_old_statement uuid; v_old_mismatch text; v_old_tenant uuid; v_old_store uuid; v_old_attempt uuid; v_kinds text[];
 v_keys constant text[]:=ARRAY['amount','charge_amount','charge_currency','charge_payment_intent','created','currency','dispute_amount',
  'dispute_currency','dispute_id','dispute_payment_intent','exchange_rate','fee','id','net','refund_amount','refund_currency',
  'refund_id','reporting_category','source_id','source_object','type'];
 v_id text; v_hash bytea; v_type text; v_rc text; v_src_obj text;
 v_amount bigint; v_fee bigint; v_net bigint; v_cur text; v_rate numeric; v_created timestamptz;
 v_kind text; v_reason text; v_old_hash bytea; v_old_reason text; v_pi text; v_refund text; v_dispute text;
 v_src_amount bigint; v_src_cur text; v_tenant uuid; v_target uuid; v_attempt uuid; v_refund_uuid uuid; v_acct text; v_order uuid;
 v_store_minor bigint; v_mismatch_code text; v_fee_store bigint; v_ratio_store bigint; v_ratio_settle bigint;
 v_found boolean; f record; c record; v_window jsonb;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_environment IS NULL OR p_environment NOT IN ('SANDBOX','LIVE') OR p_lines IS NULL OR jsonb_typeof(p_lines)<>'array'
  OR jsonb_array_length(p_lines)>500 OR (p_window_from IS NULL)<>(p_window_to IS NULL)
  OR (p_window_from IS NOT NULL AND (p_window_to<=p_window_from OR p_window_to-p_window_from>interval '8 days'
   -- P1-2: coverage may only claim time that has settled. A window ending in the future (or in the last 15 minutes) would let close treat
   -- balance transactions that did not exist yet as already read; they would never be credited or deducted. Whole seconds only: the wire
   -- lists created[gte]/created[lt] in unix seconds, so a fractional bound would claim a sliver that was never listed (P2-9).
   OR p_window_to>clock_timestamp()-interval '15 minutes'
   OR p_window_from<>date_trunc('second',p_window_from) OR p_window_to<>date_trunc('second',p_window_to)))
  OR (p_ticket IS NOT NULL AND p_ticket !~ '^[A-Za-z0-9._:-]{8,128}$') THEN
  RAISE EXCEPTION 'invalid settlement lines' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 -- the CLI names the connection whose STORED key read the list; it must be the designated platform connection (Opus review P2-1)
 IF p_connection IS NOT NULL AND p_connection<>sp.connection_id THEN
  RAISE EXCEPTION 'not_platform_connection' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.settlement_op','on',true);
 -- one writer per environment: a sync and a close never interleave (close totals must see a settled set of lines)
 PERFORM pg_advisory_xact_lock(hashtextextended('lc.settlement.'||p_environment,0));
 -- pass 1 records CHARGE lines, pass 2 everything else, so a dispute in the same batch finds its charge line (the list is newest first)
 FOR v_pass IN 1..2 LOOP
  FOR l IN SELECT e.value FROM jsonb_array_elements(p_lines) AS e(value) LOOP
   -- exactly the 21 projection keys (the CLI always sends all of them; a missing Stripe value is JSON null)
   IF jsonb_typeof(l)<>'object' OR NOT (l ?& v_keys) OR (SELECT count(*) FROM jsonb_object_keys(l))<>21
    OR NOT coalesce(l->>'id' ~ '^txn_[A-Za-z0-9]{1,255}$' AND jsonb_typeof(l->'type')='string' AND char_length(l->>'type') BETWEEN 1 AND 64
     AND jsonb_typeof(l->'amount')='number' AND l->>'amount' ~ '^-?[0-9]{1,15}$'
     AND jsonb_typeof(l->'net')='number' AND l->>'net' ~ '^-?[0-9]{1,15}$'
     AND jsonb_typeof(l->'created')='number' AND l->>'created' ~ '^[0-9]{9,11}$'
     AND jsonb_typeof(l->'currency')='string' AND l->>'currency' ~ '^[A-Z]{3}$'
     AND (l->'fee'='null'::jsonb OR (jsonb_typeof(l->'fee')='number' AND l->>'fee' ~ '^-?[0-9]{1,15}$'))
     AND (l->'exchange_rate'='null'::jsonb OR (jsonb_typeof(l->'exchange_rate')='number' AND l->>'exchange_rate' ~ '^[0-9]{1,6}(\.[0-9]{1,12})?$'))
     AND (l->'charge_amount'='null'::jsonb OR l->>'charge_amount' ~ '^[0-9]{1,15}$')
     AND (l->'refund_amount'='null'::jsonb OR l->>'refund_amount' ~ '^[0-9]{1,15}$')
     AND (l->'dispute_amount'='null'::jsonb OR l->>'dispute_amount' ~ '^[0-9]{1,15}$'),false) THEN
    RAISE EXCEPTION 'invalid settlement line' USING ERRCODE='22023'; END IF;
   v_id:=l->>'id'; v_type:=l->>'type'; v_rc:=l->>'reporting_category'; v_src_obj:=l->>'source_object';
   v_amount:=(l->>'amount')::bigint; v_net:=(l->>'net')::bigint; v_fee:=(l->>'fee')::bigint; v_cur:=l->>'currency';
   v_rate:=(l->>'exchange_rate')::numeric; v_created:=to_timestamp((l->>'created')::bigint);
   v_kind:=payments.settlement_kind(v_type,v_rc,v_src_obj,v_amount);
   IF (v_pass=1)<>coalesce(v_kind='CHARGE',false) THEN CONTINUE; END IF;
   v_hash:=sha256(convert_to(l::text,'UTF8'));

   -- replay: the same txn id is never inserted twice; changed content for it is an integrity conflict (nothing written).
   -- P1-1: a line that carries a mismatch and has no statement yet is RE-CHECKED below (webhook lag, a refund still pending): the fact it
   -- was compared with may have arrived since. Once it belongs to a statement it is frozen.
   v_recheck:=false;
   SELECT x.payload_sha256,x.statement_id,x.mismatch,x.tenant_id,x.store_id,x.attempt_id INTO v_old_hash,v_old_statement,v_old_mismatch,
     v_old_tenant,v_old_store,v_old_attempt FROM payments.settlement_lines x WHERE x.balance_txn_id=v_id;
   IF FOUND THEN
    IF v_old_hash<>v_hash THEN RAISE EXCEPTION 'settlement_content_changed' USING ERRCODE='PT409'; END IF;
    IF v_old_statement IS NOT NULL OR v_old_mismatch IS NULL THEN v_dup:=v_dup+1; CONTINUE; END IF;
    v_recheck:=true;
   END IF;
   v_found:=false;
   IF NOT v_recheck THEN
    SELECT x.payload_sha256,x.reason INTO v_old_hash,v_old_reason FROM payments.settlement_unattributed x WHERE x.balance_txn_id=v_id;
    v_found:=FOUND;
   END IF;
   IF v_found THEN
    IF v_old_hash<>v_hash THEN RAISE EXCEPTION 'settlement_content_changed' USING ERRCODE='PT409'; END IF;
    -- a final reason stays; an unmapped/foreign one is retried (a session or refund may have been recorded since)
    IF v_old_reason='unsupported_type' THEN v_dup:=v_dup+1; CONTINUE; END IF;
   END IF;

   v_reason:=NULL; v_attempt:=NULL; v_refund_uuid:=NULL; v_dispute:=NULL; v_order:=NULL; v_store_minor:=NULL;
   v_mismatch_code:=NULL; v_fee_store:=0; v_target:=NULL; v_tenant:=NULL;
   v_pi:=NULL; v_refund:=NULL; v_src_amount:=NULL; v_src_cur:=NULL;
   IF v_kind IS NULL OR v_fee IS NULL OR v_rate IS NULL OR v_rate<=0 OR l->>'source_id' IS NULL OR v_amount-v_fee<>v_net THEN
    -- payout, stripe_fee, unknown type, or a missing/inconsistent fee, rate or source: fail closed (contract §6.3)
    v_reason:='unsupported_type';
   ELSIF v_kind='DISPUTE_UNKNOWN' THEN
    v_reason:='unmapped_source';
   ELSE
    IF v_kind='CHARGE' THEN
     v_pi:=l->>'charge_payment_intent'; v_src_amount:=(l->>'charge_amount')::bigint; v_src_cur:=l->>'charge_currency';
    ELSIF v_kind IN ('REFUND','REFUND_FAILURE') THEN
     v_refund:=l->>'refund_id'; v_src_amount:=(l->>'refund_amount')::bigint; v_src_cur:=l->>'refund_currency';
    ELSE
     v_pi:=l->>'dispute_payment_intent'; v_dispute:=l->>'dispute_id';
     v_src_amount:=(l->>'dispute_amount')::bigint; v_src_cur:=l->>'dispute_currency';
    END IF;
    IF v_src_amount IS NULL OR v_src_cur IS NULL OR v_src_cur !~ '^[A-Z]{3}$' THEN
     v_reason:='unsupported_type'; -- the expanded source did not carry its presentment amount
    ELSIF v_kind IN ('DISPUTE','DISPUTE_REVERSAL') AND (v_dispute IS NULL OR v_dispute !~ '^dp_[A-Za-z0-9]{1,255}$') THEN
     v_reason:='unmapped_source';
    ELSIF v_kind IN ('REFUND','REFUND_FAILURE') THEN
     -- §4.4: Stripe refund id -> payments.stripe_refunds (cross-store read admitted by app.settlement_op)
     SELECT x.tenant_id,x.store_id,x.attempt_id,x.id,x.account_id INTO v_tenant,v_target,v_attempt,v_refund_uuid,v_acct
      FROM payments.stripe_refunds x WHERE x.stripe_refund_id=v_refund AND x.environment=p_environment;
     IF NOT FOUND THEN v_reason:='unmapped_source'; END IF;
    ELSE
     -- §4.4: payment intent -> payments.stripe_sessions. Exactly ONE session may carry the payment intent; zero (unknown) or several
     -- (ambiguous: the index is not unique) is unmapped_source, which blocks close until an operator resolves it. Never LIMIT 1.
     SELECT count(*) INTO v_n FROM payments.stripe_sessions x WHERE x.payment_intent_id=v_pi AND x.environment=p_environment;
     IF v_pi IS NULL OR v_n<>1 THEN v_reason:='unmapped_source'; ELSE
      SELECT x.tenant_id,x.store_id,x.attempt_id,x.account_id INTO v_tenant,v_target,v_attempt,v_acct
       FROM payments.stripe_sessions x WHERE x.payment_intent_id=v_pi AND x.environment=p_environment;
     END IF;
    END IF;
    IF v_reason IS NULL THEN
     IF v_acct IS DISTINCT FROM sp.account_id THEN
      v_reason:='foreign_connection';
     ELSE
      -- only attempts on the platform connection or a connection derived from it are eligible (§4.4); read in the line's own scope
      PERFORM set_config('app.tenant_id',v_tenant::text,true),set_config('app.store_id',v_target::text,true);
      SELECT a.order_id INTO v_order FROM checkout.payment_attempts a JOIN integration.merchant_accounts m ON m.id=a.connection_id
       WHERE a.tenant_id=v_tenant AND a.store_id=v_target AND a.id=v_attempt AND a.environment=p_environment
        AND (m.id=sp.connection_id OR m.platform_connection_id=sp.connection_id);
      IF NOT FOUND THEN v_reason:='foreign_connection'; END IF;
     END IF;
    END IF;
   END IF;

   IF v_reason IS NULL THEN
    -- cross-check against the worker facts and compute the signed store amount and the fee (all in the target scope)
    v_store_minor:=CASE v_kind WHEN 'CHARGE' THEN v_src_amount WHEN 'REFUND_FAILURE' THEN v_src_amount WHEN 'DISPUTE_REVERSAL' THEN v_src_amount
     ELSE -v_src_amount END;
    v_ratio_store:=v_store_minor; v_ratio_settle:=v_amount;
    IF v_kind='CHARGE' THEN
     SELECT x.amount_minor,x.currency INTO f FROM payments.facts x WHERE x.tenant_id=v_tenant AND x.store_id=v_target
      AND x.attempt_id=v_attempt AND x.kind='CAPTURED';
     IF NOT FOUND THEN v_mismatch_code:='no_fact';
     ELSIF f.currency<>v_src_cur OR v_src_cur<>'TWD' THEN v_mismatch_code:='currency';
     ELSIF f.amount_minor<>v_src_amount THEN v_mismatch_code:='amount'; END IF;
    ELSIF v_kind IN ('REFUND','REFUND_FAILURE') THEN
     -- P1-1: Stripe debits the balance when a refund is CREATED, so a REFUND line exists whether the refund later succeeds, fails or is
     -- canceled: any TERMINAL fact proves the worker saw the refund (a pending one has none yet: no_fact until it arrives). A REFUND_FAILURE
     -- needs the FAILED/CANCELED one. The amount and currency are the refund's own (payments.stripe_refunds), the same for every outcome.
     v_kinds:=CASE WHEN v_kind='REFUND' THEN ARRAY['SUCCEEDED','FAILED','CANCELED'] ELSE ARRAY['FAILED','CANCELED'] END;
     SELECT x.amount_minor,x.currency INTO f FROM payments.stripe_refunds x WHERE x.tenant_id=v_tenant AND x.store_id=v_target AND x.id=v_refund_uuid;
     IF NOT FOUND OR NOT EXISTS(SELECT 1 FROM payments.refund_facts y WHERE y.tenant_id=v_tenant AND y.store_id=v_target
       AND y.refund_id=v_refund_uuid AND y.kind=ANY(v_kinds)) THEN v_mismatch_code:='no_fact';
     ELSIF f.currency<>v_src_cur OR v_src_cur<>'TWD' THEN v_mismatch_code:='currency';
     ELSIF f.amount_minor<>v_src_amount THEN v_mismatch_code:='amount'; END IF;
    ELSE
     -- §6.3: a dispute uses the ORIGINAL charge's ratio; without the charge line the row is no_fact (and blocks close)
     SELECT x.store_minor,x.settle_amount,x.store_currency INTO c FROM payments.settlement_lines x WHERE x.tenant_id=v_tenant
      AND x.store_id=v_target AND x.attempt_id=v_attempt AND x.kind='CHARGE' AND x.environment=p_environment;
     IF NOT FOUND THEN v_mismatch_code:='no_fact'; v_ratio_settle:=0;
     ELSIF c.store_currency<>v_src_cur OR v_src_cur<>'TWD' THEN v_mismatch_code:='currency'; v_ratio_store:=c.store_minor; v_ratio_settle:=c.settle_amount;
     ELSE
      v_ratio_store:=c.store_minor; v_ratio_settle:=c.settle_amount;
      IF v_src_amount>c.store_minor THEN v_mismatch_code:='amount'; END IF;
     END IF;
    END IF;
    IF v_ratio_settle=0 THEN
     v_fee_store:=0; -- no usable ratio: the row carries a mismatch that blocks close (a zero settle amount is itself one)
     v_mismatch_code:=coalesce(v_mismatch_code,'amount');
    ELSE
     v_fee_store:=payments.settlement_fee_store(v_fee,v_ratio_store,v_ratio_settle);
    END IF;
    IF v_recheck THEN
     -- the same transaction attributes to the same store and attempt (else something changed under the ledger: stop)
     IF v_old_tenant<>v_tenant OR v_old_store<>v_target OR v_old_attempt<>v_attempt THEN
      RAISE EXCEPTION 'settlement_content_changed' USING ERRCODE='PT409'; END IF;
     -- only mismatch and the fee share may move, and only while no statement exists (the trigger enforces both)
     UPDATE payments.settlement_lines SET mismatch=v_mismatch_code,fee_store_minor=v_fee_store
      WHERE balance_txn_id=v_id AND statement_id IS NULL AND (mismatch IS DISTINCT FROM v_mismatch_code OR fee_store_minor<>v_fee_store);
     IF FOUND THEN v_rechecked:=v_rechecked+1; ELSE v_dup:=v_dup+1; END IF;
    ELSE
     INSERT INTO payments.settlement_lines(balance_txn_id,tenant_id,store_id,environment,attempt_id,order_id,refund_id,dispute_id,kind,
      store_currency,store_minor,settle_currency,settle_amount,settle_fee,settle_net,fee_store_minor,txn_created_at,mismatch,payload_sha256)
     VALUES(v_id,v_tenant,v_target,p_environment,v_attempt,v_order,v_refund_uuid,v_dispute,v_kind,v_src_cur,v_store_minor,v_cur,v_amount,
      v_fee,v_net,v_fee_store,v_created,v_mismatch_code,v_hash);
     v_inserted:=v_inserted+1;
     IF v_mismatch_code IS NOT NULL THEN v_mismatch:=v_mismatch+1; END IF;
    END IF;
   ELSE
    -- unattributed: written in the PLATFORM scope; an already stored row of the same content is only counted
    PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
    IF v_found OR v_recheck THEN v_dup:=v_dup+1; ELSE
     INSERT INTO payments.settlement_unattributed(balance_txn_id,tenant_id,store_id,environment,type,settle_currency,settle_amount,
       settle_fee,settle_net,txn_created_at,reason,payload_sha256)
      VALUES(v_id,p_tenant,p_store,p_environment,v_type,v_cur,v_amount,v_fee,v_net,v_created,v_reason,v_hash);
     v_unattr:=v_unattr+1;
    END IF;
   END IF;
   PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
  END LOOP;
 END LOOP;

 v_window:=NULL;
 IF p_window_from IS NOT NULL THEN
  INSERT INTO payments.settlement_sync_runs(tenant_id,store_id,environment,window_from,window_to,synced_by)
   VALUES(p_tenant,p_store,p_environment,p_window_from,p_window_to,p_principal);
  -- reconciliation for the window: the persisted net per settlement currency (lines + unattributed), next to Stripe's own total (CLI)
  SELECT coalesce(jsonb_object_agg(t.cur,t.net),'{}'::jsonb) INTO v_window FROM (
   SELECT u.cur,sum(u.net)::bigint AS net FROM (
    SELECT x.settle_currency AS cur,x.settle_net AS net FROM payments.settlement_lines x
     WHERE x.environment=p_environment AND x.txn_created_at>=p_window_from AND x.txn_created_at<p_window_to
    UNION ALL
    -- a row that has since become a line (unmapped, then attributed) is counted once, as the line
    SELECT y.settle_currency,y.settle_net FROM payments.settlement_unattributed y
     WHERE y.environment=p_environment AND y.txn_created_at>=p_window_from AND y.txn_created_at<p_window_to
      AND NOT EXISTS(SELECT 1 FROM payments.settlement_lines z WHERE z.balance_txn_id=y.balance_txn_id)) u GROUP BY u.cur) t;
 END IF;
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
 VALUES(p_tenant,p_store,p_principal,'stripe.settlement.sync',jsonb_strip_nulls(jsonb_build_object('environment',p_environment,
  'inserted',v_inserted,'duplicate',v_dup,'unattributed',v_unattr,'mismatch',v_mismatch,'rechecked',v_rechecked,'ticket',p_ticket)));
 RETURN jsonb_strip_nulls(jsonb_build_object('inserted',v_inserted,'duplicate',v_dup,'unattributed',v_unattr,'mismatch',v_mismatch,
  'rechecked',v_rechecked,'window_net',v_window));
END $$;

-- ---------------------------------------------------------------------------------------------------------
-- close_settlement
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.close_settlement(p_tenant uuid,p_store uuid,p_principal uuid,p_environment text,p_period_start date,
 p_operator text,p_target_store uuid DEFAULT NULL,p_ticket text DEFAULT NULL) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 sp payments.stripe_platform%ROWTYPE; v_start timestamptz; v_end timestamptz; v_cur timestamptz; r record; t record; st record; v_prev record;
 v_new uuid; v_out jsonb:='[]'::jsonb; v_captured bigint; v_refunded bigint; v_dispute bigint; v_fee bigint; v_pfee bigint; v_carry bigint;
 v_net bigint; v_count integer; v_sha bytea; v_assigned integer;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_environment IS NULL OR p_environment NOT IN ('SANDBOX','LIVE') OR p_period_start IS NULL
  OR extract(isodow FROM p_period_start)<>1 OR p_operator IS NULL OR p_operator !~ '^[A-Za-z0-9._:@-]{2,64}$'
  OR (p_ticket IS NOT NULL AND p_ticket !~ '^[A-Za-z0-9._:-]{8,128}$') THEN
  RAISE EXCEPTION 'invalid settlement close' USING ERRCODE='22023'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=p_environment;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.settlement_op','on',true);
 PERFORM pg_advisory_xact_lock(hashtextextended('lc.settlement.'||p_environment,0));
 -- Monday 00:00 to the next Monday 00:00 Asia/Taipei (OQ-2); closable only +72 h after the period end (late Stripe transactions)
 v_start:=(p_period_start::timestamp) AT TIME ZONE 'Asia/Taipei';
 v_end:=((p_period_start+7)::timestamp) AT TIME ZONE 'Asia/Taipei';
 IF v_end+interval '72 hours'>clock_timestamp() THEN RAISE EXCEPTION 'period_not_closable' USING ERRCODE='PT409'; END IF;
 -- a sync must have covered [period_start - 7 d, period_end); the runs are windows of at most 8 days, so take their union
 v_cur:=v_start-interval '7 days';
 FOR r IN SELECT x.window_from,x.window_to FROM payments.settlement_sync_runs x WHERE x.tenant_id=p_tenant AND x.store_id=p_store
   AND x.environment=p_environment AND x.window_to>v_cur AND x.window_from<v_end ORDER BY x.window_from,x.window_to DESC LOOP
  IF r.window_from<=v_cur AND r.window_to>v_cur THEN v_cur:=r.window_to; END IF;
 END LOOP;
 IF v_cur<v_end THEN RAISE EXCEPTION 'sync_required' USING ERRCODE='PT409'; END IF;
 -- an unmapped source might be any store's charge: the whole environment's close waits (resolved = a line now exists for it)
 IF EXISTS(SELECT 1 FROM payments.settlement_unattributed u WHERE u.environment=p_environment AND u.reason='unmapped_source'
   AND u.txn_created_at<v_end AND NOT EXISTS(SELECT 1 FROM payments.settlement_lines l WHERE l.balance_txn_id=u.balance_txn_id)) THEN
  RAISE EXCEPTION 'settlement_unattributed' USING ERRCODE='PT409'; END IF;
 IF EXISTS(SELECT 1 FROM payments.settlement_lines l WHERE l.environment=p_environment AND l.statement_id IS NULL
   AND l.txn_created_at<v_end AND l.mismatch IS NOT NULL AND (p_target_store IS NULL OR l.store_id=p_target_store)) THEN
  RAISE EXCEPTION 'settlement_mismatch' USING ERRCODE='PT409'; END IF;

 FOR t IN
  SELECT s.tenant_id,s.store_id FROM (
   SELECT x.tenant_id,x.store_id FROM payments.settlement_lines x WHERE x.environment=p_environment AND x.statement_id IS NULL
    AND x.txn_created_at<v_end AND (p_target_store IS NULL OR x.store_id=p_target_store)
   UNION
   -- a replay finds the statements this period already has (idempotent per store and period)
   SELECT x.tenant_id,x.store_id FROM payments.settlement_statements x WHERE x.environment=p_environment AND x.period_start=p_period_start
    AND (p_target_store IS NULL OR x.store_id=p_target_store)
   UNION
   -- P2-5 quiet week: every store that already has an EARLIER statement gets one for this period too (empty when it had no line), so the
   -- period chain never breaks and the next all-store close does not abort on previous_period_open for everybody
   SELECT x.tenant_id,x.store_id FROM payments.settlement_statements x WHERE x.environment=p_environment AND x.period_start<p_period_start
    AND (p_target_store IS NULL OR x.store_id=p_target_store)
   UNION
   -- an explicit --store may be closed with no lines (an empty statement keeps the period chain unbroken) when it can sell on the platform
   SELECT e.tenant_id,e.store_id FROM payments.platform_stripe_enrollments e WHERE e.environment=p_environment AND e.store_id=p_target_store
   UNION
   SELECT sp.tenant_id,sp.store_id WHERE sp.store_id=p_target_store) s
  ORDER BY s.tenant_id,s.store_id LOOP
  PERFORM set_config('app.tenant_id',t.tenant_id::text,true),set_config('app.store_id',t.store_id::text,true);
  SELECT x.id,x.captured_minor,x.refunded_minor,x.dispute_minor,x.stripe_fee_minor,x.line_count,x.net_payable_minor INTO st
   FROM payments.settlement_statements x WHERE x.tenant_id=t.tenant_id AND x.store_id=t.store_id AND x.environment=p_environment
    AND x.period_start=p_period_start;
  IF FOUND THEN
   -- idempotent replay: the stored statement, after proving its own lines still add up to it (else PT409)
   SELECT coalesce(sum(CASE WHEN l.kind='CHARGE' THEN l.store_minor END),0),
    coalesce(-sum(CASE WHEN l.kind IN ('REFUND','REFUND_FAILURE') THEN l.store_minor END),0),
    coalesce(-sum(CASE WHEN l.kind IN ('DISPUTE','DISPUTE_REVERSAL') THEN l.store_minor END),0),
    coalesce(sum(l.fee_store_minor),0),count(*) INTO v_captured,v_refunded,v_dispute,v_fee,v_count
    FROM payments.settlement_lines l WHERE l.tenant_id=t.tenant_id AND l.store_id=t.store_id AND l.statement_id=st.id;
   IF v_captured<>st.captured_minor OR v_refunded<>st.refunded_minor OR v_dispute<>st.dispute_minor OR v_fee<>st.stripe_fee_minor
    OR v_count<>st.line_count THEN RAISE EXCEPTION 'settlement_changed' USING ERRCODE='PT409'; END IF;
   v_out:=v_out||jsonb_build_object('statement_id',st.id,'store_id',t.store_id,'net_payable_minor',st.net_payable_minor,
    'line_count',st.line_count,'replayed',true);
   CONTINUE;
  END IF;
  -- the store's previous period must be closed, unless the store has nothing before this period
  SELECT x.id,x.net_payable_minor INTO v_prev FROM payments.settlement_statements x WHERE x.tenant_id=t.tenant_id
   AND x.store_id=t.store_id AND x.environment=p_environment AND x.period_start=p_period_start-7;
  v_carry:=0;
  IF FOUND THEN v_carry:=least(v_prev.net_payable_minor,0);
  ELSIF EXISTS(SELECT 1 FROM payments.settlement_lines l WHERE l.tenant_id=t.tenant_id AND l.store_id=t.store_id
    AND l.environment=p_environment AND l.txn_created_at<v_start)
   OR EXISTS(SELECT 1 FROM payments.settlement_statements x WHERE x.tenant_id=t.tenant_id AND x.store_id=t.store_id
    AND x.environment=p_environment AND x.period_start<p_period_start) THEN
   RAISE EXCEPTION 'previous_period_open' USING ERRCODE='PT409'; END IF;
  SELECT coalesce(sum(CASE WHEN l.kind='CHARGE' THEN l.store_minor END),0),
   coalesce(-sum(CASE WHEN l.kind IN ('REFUND','REFUND_FAILURE') THEN l.store_minor END),0),
   coalesce(-sum(CASE WHEN l.kind IN ('DISPUTE','DISPUTE_REVERSAL') THEN l.store_minor END),0),
   coalesce(sum(l.fee_store_minor),0),count(*),
   sha256(convert_to(coalesce(string_agg(l.balance_txn_id||'|'||l.kind||'|'||l.store_minor||'|'||l.fee_store_minor,E'\n'
    ORDER BY l.balance_txn_id),''),'UTF8')) INTO v_captured,v_refunded,v_dispute,v_fee,v_count,v_sha
   FROM payments.settlement_lines l WHERE l.tenant_id=t.tenant_id AND l.store_id=t.store_id AND l.environment=p_environment
    AND l.statement_id IS NULL AND l.txn_created_at<v_end;
  -- platform fee: bps is data (default 0), rounded half up to the currency step (TWD 100), never negative
  v_pfee:=greatest(0,(100*floor(((v_captured-v_refunded)::numeric*sp.platform_fee_bps/10000)/100+0.5))::bigint);
  v_net:=v_captured-v_refunded-v_dispute+v_fee-v_pfee+v_carry;
  v_new:=gen_random_uuid();
  INSERT INTO payments.settlement_statements(id,tenant_id,store_id,environment,period_start,period_end,currency,captured_minor,
    refunded_minor,dispute_minor,stripe_fee_minor,platform_fee_bps,platform_fee_minor,carried_in_minor,net_payable_minor,line_count,
    lines_sha256,closed_by)
   VALUES(v_new,t.tenant_id,t.store_id,p_environment,p_period_start,p_period_start+7,'TWD',v_captured,v_refunded,v_dispute,v_fee,
    sp.platform_fee_bps,v_pfee,v_carry,v_net,v_count,v_sha,p_operator);
  UPDATE payments.settlement_lines SET statement_id=v_new WHERE tenant_id=t.tenant_id AND store_id=t.store_id
   AND environment=p_environment AND statement_id IS NULL AND txn_created_at<v_end;
  GET DIAGNOSTICS v_assigned=ROW_COUNT;
  IF v_assigned<>v_count THEN RAISE EXCEPTION 'settlement_changed' USING ERRCODE='PT409'; END IF;
  v_out:=v_out||jsonb_build_object('statement_id',v_new,'store_id',t.store_id,'net_payable_minor',v_net,'line_count',v_count,
   'replayed',false);
  -- the audit row returns to the platform scope; the target store is in the details
  PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
  VALUES(p_tenant,p_store,p_principal,'stripe.settlement.close',jsonb_build_object('environment',p_environment,
   'target_store',t.store_id,'period_start',p_period_start,'statement_id',v_new,'ticket',p_ticket));
 END LOOP;
 PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
 RETURN jsonb_build_object('statements',v_out);
END $$;

-- ---------------------------------------------------------------------------------------------------------
-- record_settlement_payout (RECORD ONLY: this unit never calls a bank, Stripe payout or transfer API)
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.record_settlement_payout(p_tenant uuid,p_store uuid,p_principal uuid,p_statement uuid,p_payout_ref text,
 p_payout_minor bigint,p_paid_at timestamptz,p_operator text,p_ticket text DEFAULT NULL) RETURNS timestamptz
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE st payments.settlement_statements%ROWTYPE; sp payments.stripe_platform%ROWTYPE;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_statement IS NULL OR p_payout_ref IS NULL OR p_payout_ref !~ '^[A-Za-z0-9._:/-]{4,80}$' OR p_payout_minor IS NULL
  OR p_paid_at IS NULL OR p_operator IS NULL OR p_operator !~ '^[A-Za-z0-9._:@-]{2,64}$'
  OR (p_ticket IS NOT NULL AND p_ticket !~ '^[A-Za-z0-9._:-]{8,128}$') THEN
  RAISE EXCEPTION 'invalid settlement payout' USING ERRCODE='22023'; END IF;
 PERFORM set_config('app.settlement_op','on',true);
 SELECT x.* INTO st FROM payments.settlement_statements x WHERE x.id=p_statement;
 IF NOT FOUND THEN RAISE EXCEPTION 'statement_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=st.environment;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 IF st.payout_ref IS NOT NULL THEN
  -- identical replay returns the stored time; anything else is a conflict
  IF st.payout_ref=p_payout_ref AND st.payout_minor=p_payout_minor AND st.paid_at=p_paid_at THEN RETURN st.paid_at; END IF;
  RAISE EXCEPTION 'payout_already_recorded' USING ERRCODE='PT409';
 END IF;
 IF st.net_payable_minor<=0 OR p_payout_minor<>st.net_payable_minor THEN
  RAISE EXCEPTION 'payout_amount_mismatch' USING ERRCODE='PT409'; END IF;
 IF p_paid_at>clock_timestamp() THEN RAISE EXCEPTION 'invalid settlement payout' USING ERRCODE='22023'; END IF;
 PERFORM set_config('app.tenant_id',st.tenant_id::text,true),set_config('app.store_id',st.store_id::text,true);
 UPDATE payments.settlement_statements SET payout_ref=p_payout_ref,payout_minor=p_payout_minor,paid_at=p_paid_at,paid_recorded_by=p_operator
  WHERE id=st.id AND tenant_id=st.tenant_id AND store_id=st.store_id AND payout_ref IS NULL;
 IF NOT FOUND THEN RAISE EXCEPTION 'payout_already_recorded' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
 INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action,details)
 VALUES(p_tenant,p_store,p_principal,'stripe.settlement.payout',jsonb_strip_nulls(jsonb_build_object('environment',st.environment,
  'target_store',st.store_id,'statement_id',st.id,'ticket',p_ticket)));
 RETURN p_paid_at;
END $$;

-- ---------------------------------------------------------------------------------------------------------
-- read_settlement_statement (operator export): one statement with its lines, platform scope
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.read_settlement_statement(p_tenant uuid,p_store uuid,p_principal uuid,p_statement uuid) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE st payments.settlement_statements%ROWTYPE; sp payments.stripe_platform%ROWTYPE; v_json jsonb;
BEGIN
 PERFORM integration.require_stripe_registrar_scope(p_tenant,p_store,p_principal);
 IF p_statement IS NULL THEN RAISE EXCEPTION 'invalid settlement statement read' USING ERRCODE='22023'; END IF;
 PERFORM set_config('app.settlement_op','on',true);
 SELECT x.* INTO st FROM payments.settlement_statements x WHERE x.id=p_statement;
 IF NOT FOUND THEN RAISE EXCEPTION 'statement_unavailable' USING ERRCODE='PT409'; END IF;
 SELECT x.* INTO sp FROM payments.stripe_platform x WHERE x.environment=st.environment;
 IF NOT FOUND OR sp.tenant_id<>p_tenant OR sp.store_id<>p_store THEN
  RAISE EXCEPTION 'platform_stripe_unavailable' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',st.tenant_id::text,true),set_config('app.store_id',st.store_id::text,true);
 v_json:=payments.settlement_statement_json(st.tenant_id,st.store_id,st.id,true)->0;
 PERFORM set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true);
 RETURN v_json;
END $$;

-- ---------------------------------------------------------------------------------------------------------
-- read_store_settlements (merchant): billing:manage, the store comes from the token, no settlement-currency fields
-- ---------------------------------------------------------------------------------------------------------
CREATE FUNCTION payments.read_store_settlements(p_token bytea,p_store uuid,p_limit integer,p_before date,p_statement uuid DEFAULT NULL)
RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_list jsonb;
BEGIN
 IF p_token IS NULL OR octet_length(p_token)<>32 OR p_store IS NULL OR p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 52
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid settlement read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_token,p_store,'billing:manage');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 IF p_statement IS NOT NULL THEN
  v_list:=payments.settlement_statement_json(s.tenant_id,p_store,p_statement,true);
  IF jsonb_array_length(v_list)=0 THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
  v_list:=jsonb_build_object('statement',v_list->0);
 ELSE
  -- the list is statements only (no lines): bounded; the single-statement route returns the lines
  SELECT jsonb_build_object('statements',coalesce(jsonb_agg(j.o ORDER BY j.ps DESC),'[]'::jsonb)) INTO v_list FROM (
   SELECT x.period_start AS ps,payments.settlement_statement_json(x.tenant_id,x.store_id,x.id,false)->0 AS o
   FROM payments.settlement_statements x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store
    AND (p_before IS NULL OR x.period_start<p_before) ORDER BY x.period_start DESC LIMIT p_limit) j;
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_token,p_store,'billing:manage');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_list;
END $$;

ALTER FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.record_settlement_payout(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.read_settlement_statement(uuid,uuid,uuid,uuid) OWNER TO commerce_payment_registry_writer;
ALTER FUNCTION payments.read_store_settlements(bytea,uuid,integer,date,uuid) OWNER TO commerce_payment_registry_writer;
REVOKE ALL ON FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text),
 payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text),
 payments.record_settlement_payout(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text),
 payments.read_settlement_statement(uuid,uuid,uuid,uuid),
 payments.read_store_settlements(bytea,uuid,integer,date,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text),
 payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text),
 payments.record_settlement_payout(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text),
 payments.read_settlement_statement(uuid,uuid,uuid,uuid) TO commerce_payment_registrar;
GRANT EXECUTE ON FUNCTION payments.read_store_settlements(bytea,uuid,integer,date,uuid) TO commerce_runtime;
