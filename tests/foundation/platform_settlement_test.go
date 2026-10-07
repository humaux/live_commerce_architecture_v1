package foundation_test

// W4-S2 per-store platform settlement ledger (contracts/stripe-platform-account-v1.md §6; author smoke of gates PF09-PF13, the independent
// K3 gate is platform_settlement_gate_test.go). Tier: MOCK (a fixture balance-transaction list, never Stripe) + REAL_PG: payments are paid
// through the real capture path (pfEnv), refunds through the real refund path, ledger writes only through stripeadmin.Registrar (the same
// methods cmd/stripe-admin calls), merchant reads only through the real HTTP handler. SANDBOX (a real balance-transaction read with the owner's
// test key) is NOT_RUN.
//
// Disclosed owner-pool fixtures (named at their call site): the PF09 negative inserts/updates that prove CHECKs and set-once triggers, and one
// unattributed row of reason unmapped_source (the only way to prove the environment-wide close block without a Stripe read). S2-OPEN-1: that row
// is now cleared through the 0163 resolve step, exactly the way an operator clears it; the former owner-pool DELETE fixture is gone.
// Everything else goes through the product paths.

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/payments/settlement"
	"livecommerce/internal/payments/stripeadmin"
)

var pslTPE = time.FixedZone("TPE", 8*3600)

// pslTicket is the operator ticket every fixture call carries; it must reach the audit details (Opus review P2-10).
const pslTicket = "TICKET-2026-1001"

func pslDay(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, pslTPE) }

func pslS(v string) *string { return &v }
func pslI(v int64) *int64   { return &v }

// pslFee is the contract §6.3 reference with the review's symmetric rounding (P2-4): -step * sign * floor(|fee*store/settle/step| + 1/2), step 100,
// in exact rational arithmetic (half away from zero on the absolute value, then the sign), so a refund exactly reverses its charge's fee share.
func pslFee(fee, store, settle int64) int64 {
	num, den := big.NewInt(fee*store), big.NewInt(settle*100)
	negative := num.Sign()*den.Sign() < 0
	q := new(big.Rat).SetFrac(new(big.Int).Abs(num), new(big.Int).Abs(den))
	q.Add(q, big.NewRat(1, 2))
	magnitude := new(big.Int).Div(q.Num(), q.Denom()) // floor of a positive rational
	steps := -100 * magnitude.Int64()
	if negative {
		return -steps
	}
	return steps
}

func pslCharge(id, pi string, store, settle, fee int64, at time.Time) stripe.BalanceTransaction {
	rate := json.Number("0.2564")
	return stripe.BalanceTransaction{ID: id, Type: "charge", ReportingCategory: pslS("charge"), Amount: settle, Fee: pslI(fee), Net: settle - fee,
		Currency: "HKD", ExchangeRate: &rate, Created: at.Unix(), SourceID: pslS("ch_" + id[4:]), SourceObject: pslS("charge"),
		ChargePaymentIntent: pslS(pi), ChargeAmount: pslI(store), ChargeCurrency: pslS("TWD")}
}

func pslRefund(id, refund string, store, settle, fee int64, at time.Time) stripe.BalanceTransaction {
	rate := json.Number("0.2564")
	return stripe.BalanceTransaction{ID: id, Type: "refund", ReportingCategory: pslS("refund"), Amount: -settle, Fee: pslI(fee), Net: -settle - fee,
		Currency: "HKD", ExchangeRate: &rate, Created: at.Unix(), SourceID: pslS(refund), SourceObject: pslS("refund"),
		RefundID: pslS(refund), RefundAmount: pslI(store), RefundCurrency: pslS("TWD")}
}

func pslRefundFailure(id, refund string, store, settle int64, at time.Time) stripe.BalanceTransaction {
	t := pslRefund(id, refund, store, settle, 0, at)
	t.Type, t.ReportingCategory = "refund_failure", pslS("refund_failure")
	t.Amount, t.Net = settle, settle
	return t
}

// pslDispute: a dispute (reversal=false, money leaves) or its reversal (money returns, the dispute fee is credited back).
func pslDispute(id, dispute, pi string, store, settle, fee int64, reversal bool, at time.Time) stripe.BalanceTransaction {
	rate := json.Number("0.2564")
	t := stripe.BalanceTransaction{ID: id, Type: "adjustment", ReportingCategory: pslS("dispute"), Amount: -settle, Fee: pslI(fee), Net: -settle - fee,
		Currency: "HKD", ExchangeRate: &rate, Created: at.Unix(), SourceID: pslS(dispute), SourceObject: pslS("dispute"),
		DisputeID: pslS(dispute), DisputePaymentIntent: pslS(pi), DisputeAmount: pslI(store), DisputeCurrency: pslS("TWD")}
	if reversal {
		t.ReportingCategory, t.Amount, t.Fee, t.Net = pslS("dispute_reversal"), settle, pslI(-fee), settle+fee
	}
	return t
}

func pslRaw(id, typ, rc string, amount int64, fee *int64, source *string, at time.Time) stripe.BalanceTransaction {
	f := int64(0)
	if fee != nil {
		f = *fee
	}
	return stripe.BalanceTransaction{ID: id, Type: typ, ReportingCategory: pslS(rc), Amount: amount, Fee: fee, Net: amount - f, Currency: "HKD",
		Created: at.Unix(), SourceID: source}
}

// pslEnv is one isolated database: the platform store, two enrolled stores A and B, a primary store X on another Stripe account, and orders
// paid through the real capture path (A1..A3 in A, B1 in B, P1 on the platform store, X1 on X) plus a refund each on A1 and B1.
type pslEnv struct {
	*pfEnv
	a, b, c                        psHarness
	oa1, oa2, oa3, ob1             rfxOrder
	oc1, oc2                       rfxOrder
	op1, ox1                       rfxOrder
	refundA1, refundB1             string // Stripe refund ids
	refundC2, refundC2ID           string // the Stripe refund id and the refund uuid of C2's refund
	storeA, storeB, storeC, storeP string
}

func pslNew(t *testing.T) *pslEnv {
	t.Helper()
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	a, b, c := pf.merchant(t, true), pf.merchant(t, true), pf.merchant(t, true)
	pf.mustSet(t, a, pfInput(true, 0, ""))
	pf.mustSet(t, b, pfInput(true, 0, ""))
	pf.mustSet(t, c, pfInput(true, 0, ""))
	e := &pslEnv{pfEnv: pf, a: a, b: b, c: c}
	sa, sb, sc := pf.derived(t, a), pf.derived(t, b), pf.derived(t, c)
	e.oc1 = pf.pay(t, sc, pf.endpoint, pf.secret)
	e.oc2 = pf.payMore(t, e.oc1)
	e.oa1 = pf.pay(t, sa, pf.endpoint, pf.secret)
	e.oa2 = pf.payMore(t, e.oa1)
	e.oa3 = pf.payMore(t, e.oa2)
	e.ob1 = pf.pay(t, sb, pf.endpoint, pf.secret)
	e.op1 = pf.pay(t, pf.plat, pf.endpoint, pf.secret)
	x := pf.e2(t)
	xep, xsec := pf.sflEnv.endpoint(t, x)
	e.ox1 = pf.pay(t, x, xep, xsec)
	for _, o := range []rfxOrder{e.oa1, e.ob1, e.oc2} {
		pf.grant(t, o, "orders:read", "payments:refund")
	}
	ra, rb, rc := pf.mustRefund(t, e.oa1, 800, "requested_by_customer"), pf.mustRefund(t, e.ob1, 800, "requested_by_customer"), pf.mustRefund(t, e.oc2, 800, "requested_by_customer")
	pf.awaitRefundFact(t, ra, e.oa1.attempt, "SUCCEEDED")
	pf.awaitRefundFact(t, rb, e.ob1.attempt, "SUCCEEDED")
	pf.awaitRefundFact(t, rc, e.oc2.attempt, "SUCCEEDED")
	e.refundC2ID = rc
	for refund, dest := range map[string]*string{ra: &e.refundA1, rb: &e.refundB1, rc: &e.refundC2} {
		if err := pf.f.owner.QueryRow(context.Background(), `SELECT stripe_refund_id FROM payments.stripe_refunds WHERE id=$1`, refund).Scan(dest); err != nil {
			t.Fatalf("stripe refund id: %v", err)
		}
	}
	e.storeA, e.storeB, e.storeC, e.storeP = a.f.storeA1, b.f.storeA1, c.f.storeA1, pf.plat.p.f.storeA1
	return e
}

func (e *pslEnv) sync(t *testing.T, from, to time.Time, txns ...stripe.BalanceTransaction) stripeadmin.SyncReport {
	t.Helper()
	rep, err := e.reg.RecordSettlementLines(context.Background(), e.op, e.plat.connection, pslTicket, txns, from, to)
	if err != nil {
		t.Fatalf("sync [%s,%s): %v", from.Format("01-02"), to.Format("01-02"), err)
	}
	return rep
}

func (e *pslEnv) syncErr(from, to time.Time, txns ...stripe.BalanceTransaction) error {
	_, err := e.reg.RecordSettlementLines(context.Background(), e.op, e.plat.connection, pslTicket, txns, from, to)
	return err
}

func (e *pslEnv) closeWeek(start time.Time, target string) ([]stripeadmin.ClosedStatement, error) {
	res, err := e.reg.SettlementClose(context.Background(), e.op, "SANDBOX", start.Format("2006-01-02"), "op@test", target, pslTicket)
	return res.Statements, err // S2-OPEN-1: SettlementClose now returns CloseResult; operator notes are asserted in settlement_resolve_test.go
}

func (e *pslEnv) line(t *testing.T, txn string) (kind, store string, storeMinor, feeStore int64, mismatch *string, statement *string) {
	t.Helper()
	if err := e.f.owner.QueryRow(context.Background(), `SELECT kind,store_id::text,store_minor,fee_store_minor,mismatch,statement_id::text FROM payments.settlement_lines WHERE balance_txn_id=$1`,
		txn).Scan(&kind, &store, &storeMinor, &feeStore, &mismatch, &statement); err != nil {
		t.Fatalf("line %s: %v", txn, err)
	}
	return
}

func (e *pslEnv) unattributed(t *testing.T, txn string) string {
	t.Helper()
	var reason string
	if err := e.f.owner.QueryRow(context.Background(), `SELECT reason FROM payments.settlement_unattributed WHERE balance_txn_id=$1`, txn).Scan(&reason); err != nil {
		t.Fatalf("unattributed %s: %v", txn, err)
	}
	return reason
}

// fixtureExec runs SQL in one owner-pool transaction with triggers and FK checks off (session_replication_role=replica). Disclosed fixture:
// it only moves worker-written fact rows aside and back, to reproduce the webhook/worker lag that P1-1 is about.
func (e *pslEnv) fixtureExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := e.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role=replica`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("fixture %.60s: %v", sql, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// hideCaptured moves the attempt's CAPTURED fact out of payments.facts (the worker has not recorded it yet) and returns the restore.
func (e *pslEnv) hideCaptured(t *testing.T, attempt string) func() {
	t.Helper()
	e.fixtureExec(t, `CREATE TABLE IF NOT EXISTS public.psl_saved_facts (LIKE payments.facts)`)
	e.fixtureExec(t, `INSERT INTO public.psl_saved_facts SELECT * FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED'`, attempt)
	e.fixtureExec(t, `DELETE FROM payments.facts WHERE attempt_id=$1 AND kind='CAPTURED'`, attempt)
	return func() {
		e.fixtureExec(t, `INSERT INTO payments.facts SELECT * FROM public.psl_saved_facts WHERE attempt_id=$1`, attempt)
		e.fixtureExec(t, `DELETE FROM public.psl_saved_facts WHERE attempt_id=$1`, attempt)
	}
}

// hideRefundFact removes the refund's fact rows (the refund is pending as far as the database knows) and returns a restore that puts the
// fact back with the given terminal kind (SUCCEEDED, or FAILED for a refund that never succeeded).
func (e *pslEnv) hideRefundFact(t *testing.T, refund string) func(kind string) {
	t.Helper()
	e.fixtureExec(t, `CREATE TABLE IF NOT EXISTS public.psl_saved_refund_facts (LIKE payments.refund_facts)`)
	e.fixtureExec(t, `INSERT INTO public.psl_saved_refund_facts SELECT * FROM payments.refund_facts WHERE refund_id=$1`, refund)
	e.fixtureExec(t, `DELETE FROM payments.refund_facts WHERE refund_id=$1`, refund)
	return func(kind string) {
		e.fixtureExec(t, `INSERT INTO payments.refund_facts(tenant_id,store_id,refund_id,attempt_id,kind,amount_minor,currency,stripe_refund_id,failure_reason,source_report_hash,received_at)
			SELECT tenant_id,store_id,refund_id,attempt_id,$2,amount_minor,currency,stripe_refund_id,failure_reason,source_report_hash,received_at
			FROM public.psl_saved_refund_facts WHERE refund_id=$1`, refund, kind)
		e.fixtureExec(t, `DELETE FROM public.psl_saved_refund_facts WHERE refund_id=$1`, refund)
	}
}

func pslWantRefused(t *testing.T, what string, err error, token string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "rejected") || (token != "" && !strings.Contains(err.Error(), token)) {
		t.Fatalf("%s: want a rejection carrying %q, got %v", what, token, err)
	}
}

func TestPlatformSettlement(t *testing.T) {
	e := pslNew(t)
	ctx := context.Background()
	w0, w1, w2, w3, w4 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14), pslDay(2026, 9, 21), pslDay(2026, 9, 28)
	d := func(w time.Time, n int) time.Time { return w.Add(time.Duration(n)*24*time.Hour + 3*time.Hour) }
	settleOf := func(minor int64) int64 { return minor * 2564 / 10000 } // 2500 -> 641 (the fixture rate 0.2564)

	// ---------------------------------------------------------------------------------------------------------------------
	// PF09 schema: FORCE RLS, ACL inventory, CHECKs, set-once triggers, the §6.3 fee vector against the Go reference.
	// ---------------------------------------------------------------------------------------------------------------------
	t.Run("PF09_schema", func(t *testing.T) {
		o := e.f.owner
		tables := []string{"settlement_lines", "settlement_statements", "settlement_unattributed", "settlement_sync_runs",
			"settlement_unattributed_resolutions"} // 0163 (S2-OPEN-1): the append-only resolution table joins every privilege pin
		for _, tbl := range tables {
			var rls, force bool
			if err := o.QueryRow(ctx, `SELECT relrowsecurity,relforcerowsecurity FROM pg_class WHERE oid=('payments.'||$1)::regclass`, tbl).Scan(&rls, &force); err != nil || !rls || !force {
				t.Fatalf("%s: rls=%v force=%v err=%v", tbl, rls, force, err)
			}
			// only the registry writer holds any privilege (superusers aside); never PUBLIC, never DELETE, never a worker/ingress/checkout role
			over := lcStrings(t, o, `SELECT r.rolname FROM pg_roles r WHERE NOT r.rolsuper AND r.rolname NOT LIKE 'pg\_%' AND r.rolname<>'commerce_payment_registry_writer'
				AND (has_table_privilege(r.oid,('payments.'||$1)::regclass,'SELECT,INSERT,UPDATE,DELETE') OR has_any_column_privilege(r.oid,('payments.'||$1)::regclass,'SELECT,INSERT,UPDATE'))
				AND r.rolname<>(SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid=('payments.'||$1)::regclass)`, tbl)
			if len(over) != 0 {
				t.Fatalf("%s is reachable by %v", tbl, over)
			}
			var del, pub bool
			if err := o.QueryRow(ctx, `SELECT has_table_privilege('commerce_payment_registry_writer',('payments.'||$1)::regclass,'DELETE'),
				EXISTS(SELECT 1 FROM pg_class c,aclexplode(c.relacl) a WHERE c.oid=('payments.'||$1)::regclass AND a.grantee=0)`, tbl).Scan(&del, &pub); err != nil || del || pub {
				t.Fatalf("%s: registry writer DELETE=%v PUBLIC grant=%v err=%v", tbl, del, pub, err)
			}
		}
		fns := map[string][]string{
			"payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text)": {"commerce_payment_registrar", "commerce_payment_registry_writer"},
			"payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text)":                            {"commerce_payment_registrar", "commerce_payment_registry_writer"},
			"payments.record_settlement_payout(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text)":      {"commerce_payment_registrar", "commerce_payment_registry_writer"},
			"payments.read_settlement_statement(uuid,uuid,uuid,uuid)":                                       {"commerce_payment_registrar", "commerce_payment_registry_writer"},
			"payments.read_store_settlements(bytea,uuid,integer,date,uuid)":                                 {"commerce_runtime", "commerce_payment_registry_writer"},
			"payments.record_settlement_resolution(uuid,uuid,uuid,text,text,text,text,text,text,uuid,uuid)": {"commerce_payment_registrar", "commerce_payment_registry_writer"}, // 0163 (S2-OPEN-1)
			"payments.settlement_kind(text,text,text,bigint)":                                               {"commerce_payment_registry_writer"},
			"payments.settlement_fee_store(bigint,bigint,bigint)":                                           {"commerce_payment_registry_writer"},
			"payments.settlement_statement_json(uuid,uuid,uuid,boolean)":                                    {"commerce_payment_registry_writer"},
		}
		for fn, want := range fns {
			var secdef bool
			var owner, cfg string
			var acl []string
			if err := o.QueryRow(ctx, `SELECT p.prosecdef,pg_get_userbyid(p.proowner),coalesce(p.proconfig::text,''),
				coalesce((SELECT array_agg(a.grantee::regrole::text ORDER BY a.grantee::regrole::text) FROM aclexplode(p.proacl) a WHERE a.privilege_type='EXECUTE'),'{}')
				FROM pg_proc p WHERE p.oid=$1::regprocedure`, fn).Scan(&secdef, &owner, &cfg, &acl); err != nil {
				t.Fatalf("%s missing: %v", fn, err)
			}
			entry := strings.HasPrefix(fn, "payments.record_") || strings.HasPrefix(fn, "payments.close_") || strings.HasPrefix(fn, "payments.read_")
			if secdef != entry || owner != "commerce_payment_registry_writer" || !strings.Contains(cfg, "search_path=pg_catalog") {
				t.Fatalf("%s: secdef=%v owner=%s cfg=%s", fn, secdef, owner, cfg)
			}
			lcSameSet(t, fn+" EXECUTE grantees", acl, want)
		}
		// no worker, ingress, checkout or integration authority can execute any operator definer
		for fn := range fns {
			if !strings.Contains(fn, "record_") && !strings.Contains(fn, "close_") && !strings.Contains(fn, "read_s") {
				continue
			}
			over := lcStrings(t, o, `SELECT r.rolname FROM pg_roles r WHERE r.rolname LIKE 'commerce\_%' AND NOT r.rolsuper
				AND r.rolname NOT IN ('commerce_payment_registrar','commerce_payment_registry_writer','commerce_runtime') AND has_function_privilege(r.oid,$1::regprocedure,'EXECUTE')`, fn)
			if len(over) != 0 {
				t.Fatalf("%s is executable by %v", fn, over)
			}
		}
		// the merchant runtime can execute the reader only
		if n := countRows(t, o, `SELECT count(*) FROM unnest(ARRAY['payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text)',
			'payments.close_settlement(uuid,uuid,uuid,text,date,text,uuid,text)','payments.record_settlement_payout(uuid,uuid,uuid,uuid,text,bigint,timestamptz,text,text)',
			'payments.read_settlement_statement(uuid,uuid,uuid,uuid)',
			'payments.record_settlement_resolution(uuid,uuid,uuid,text,text,text,text,text,text,uuid,uuid)']) f
			WHERE has_function_privilege('commerce_runtime',f::regprocedure,'EXECUTE')`); n != 0 {
			t.Fatalf("commerce_runtime can execute %d operator definers", n)
		}

		// fixtures through the owner pool (disclosed): rows on the platform store at a period no other subtest uses
		plat := e.storeP
		tenant := e.plat.p.f.tenantA
		stmt := "5a5a5a5a-5a5a-4a5a-8a5a-5a5a5a5a5a01"
		insert := func(net, captured int64) error {
			_, err := o.Exec(ctx, `INSERT INTO payments.settlement_statements(id,tenant_id,store_id,environment,period_start,period_end,currency,captured_minor,refunded_minor,
				dispute_minor,stripe_fee_minor,platform_fee_bps,platform_fee_minor,carried_in_minor,net_payable_minor,line_count,lines_sha256,closed_by)
				VALUES($1,$2,$3,'SANDBOX','2025-01-06','2025-01-13','TWD',$4,0,0,0,0,0,0,$5,0,sha256('x'),'fixture@test')`, stmt, tenant, plat, captured, net)
			return err
		}
		if sqlState(insert(999, 1000)) != "23514" { // net identity: captured-refunded-dispute+fee-platform_fee+carried = net
			t.Fatal("net identity CHECK missing")
		}
		if err := insert(1000, 1000); err != nil {
			t.Fatalf("valid statement: %v", err)
		}
		defer func() { mustExec(t, o, `DELETE FROM payments.settlement_statements WHERE id=$1`, stmt) }()
		if _, err := o.Exec(ctx, `INSERT INTO payments.settlement_statements(id,tenant_id,store_id,environment,period_start,period_end,currency,captured_minor,refunded_minor,dispute_minor,
			stripe_fee_minor,platform_fee_bps,platform_fee_minor,carried_in_minor,net_payable_minor,line_count,lines_sha256,closed_by)
			VALUES(gen_random_uuid(),$1,$2,'SANDBOX','2025-01-07','2025-01-14','TWD',0,0,0,0,0,0,0,0,0,sha256('x'),'fixture@test')`, tenant, plat); sqlState(err) != "23514" {
			t.Fatalf("a non-Monday period must be refused: %v", err)
		}
		if _, err := o.Exec(ctx, `UPDATE payments.settlement_statements SET payout_ref='BANK-REF-1',payout_minor=999,paid_at=now(),paid_recorded_by='op@test' WHERE id=$1`, stmt); sqlState(err) != "23514" {
			t.Fatalf("payout_minor must equal net_payable: %v", err)
		}
		if _, err := o.Exec(ctx, `UPDATE payments.settlement_statements SET payout_ref='BANK-REF-1' WHERE id=$1`, stmt); sqlState(err) != "23514" {
			t.Fatalf("the payout quadruple is all-or-nothing: %v", err)
		}
		mustExec(t, o, `UPDATE payments.settlement_statements SET payout_ref='BANK-REF-1',payout_minor=1000,paid_at=now(),paid_recorded_by='op@test' WHERE id=$1`, stmt)
		if _, err := o.Exec(ctx, `UPDATE payments.settlement_statements SET payout_ref='BANK-REF-2',payout_minor=1000,paid_at=now(),paid_recorded_by='op@test' WHERE id=$1`, stmt); sqlState(err) != "PT409" {
			t.Fatalf("the payout triple is set once: %v", err)
		}
		if _, err := o.Exec(ctx, `UPDATE payments.settlement_statements SET captured_minor=2000,net_payable_minor=2000 WHERE id=$1`, stmt); sqlState(err) != "PT409" {
			t.Fatalf("a closed statement is frozen: %v", err)
		}
		// line CHECKs: Stripe's own identity, the kind/sign rule
		line := func(kind string, store, settleNet int64) error {
			_, err := o.Exec(ctx, `INSERT INTO payments.settlement_lines(balance_txn_id,tenant_id,store_id,environment,attempt_id,order_id,kind,store_currency,store_minor,settle_currency,
				settle_amount,settle_fee,settle_net,fee_store_minor,txn_created_at,payload_sha256) VALUES('txn_Fixture'||$7::text,$1,$2,'SANDBOX',$3,$4,$5,'TWD',$6,'HKD',100,10,$8,0,now(),sha256('x'))`,
				e.op1.s.p.f.tenantA, plat, e.op1.attempt, "9f1c2e4d-5a6b-4c7d-8e9f-0a1b2c3d4e5f", kind, store, t04Tag(), settleNet)
			return err
		}
		if sqlState(line("CHARGE", 100, 91)) != "23514" { // settle_amount - settle_fee <> settle_net
			t.Fatal("settle net identity CHECK missing")
		}
		if sqlState(line("CHARGE", -100, 90)) != "23514" { // a CHARGE is never negative
			t.Fatal("kind/sign CHECK missing")
		}
		// set-once statement_id on a line: a fixture line on the platform store, assigned once, never moved
		if err := line("CHARGE", 100, 90); err != nil {
			t.Fatalf("valid fixture line: %v", err)
		}
		defer func() {
			mustExec(t, o, `DELETE FROM payments.settlement_lines WHERE balance_txn_id LIKE 'txn_Fixture%'`)
		}()
		// P1-1: before a statement exists ONLY mismatch and the fee share may be re-checked; nothing else moves
		mustExec(t, o, `UPDATE payments.settlement_lines SET mismatch='no_fact',fee_store_minor=-100 WHERE balance_txn_id LIKE 'txn_Fixture%'`)
		mustExec(t, o, `UPDATE payments.settlement_lines SET mismatch=NULL,fee_store_minor=0 WHERE balance_txn_id LIKE 'txn_Fixture%'`)
		for _, bad := range []string{`store_minor=200`, `settle_fee=11,settle_net=89`, `kind='REFUND'`, `payload_sha256=sha256('y')`, `txn_created_at=now()`} {
			if _, err := o.Exec(ctx, `UPDATE payments.settlement_lines SET `+bad+` WHERE balance_txn_id LIKE 'txn_Fixture%'`); sqlState(err) != "PT409" {
				t.Fatalf("an unassigned line accepted a change of %s: %v", bad, err)
			}
		}
		// the payment-intent index is a plain lookup index, NOT unique (a unique index would make the payments recording path fail on a repeated
		// provider id, TestStripeSP12MoneyChecks); ambiguity is handled in the attribution (PF10_ambiguous_payment_intent)
		var unique bool
		if err := o.QueryRow(ctx, `SELECT indisunique FROM pg_index WHERE indexrelid='payments.stripe_sessions_payment_intent_idx'::regclass`).Scan(&unique); err != nil || unique {
			t.Fatalf("stripe_sessions_payment_intent_idx must exist and must not be UNIQUE: %v %v", unique, err)
		}
		mustExec(t, o, `UPDATE payments.settlement_lines SET statement_id=$1 WHERE balance_txn_id LIKE 'txn_Fixture%'`, stmt)
		if _, err := o.Exec(ctx, `UPDATE payments.settlement_lines SET statement_id=NULL WHERE balance_txn_id LIKE 'txn_Fixture%'`); sqlState(err) != "PT409" {
			t.Fatalf("statement_id is set once: %v", err)
		}
		if _, err := o.Exec(ctx, `UPDATE payments.settlement_lines SET store_minor=200 WHERE balance_txn_id LIKE 'txn_Fixture%'`); sqlState(err) != "PT409" {
			t.Fatalf("a ledger line is immutable: %v", err)
		}

		// P2-6 (Opus): the attribution reads 0150 grants on three existing tables are pinned to the exact columns (SELECT only), for the registry writer
		// and for nobody else beyond what those tables already granted.
		colsOf := func(table, role string) []string {
			return lcStrings(t, o, `SELECT a.attname FROM pg_attribute a, aclexplode(a.attacl) x WHERE a.attrelid=$1::regclass AND x.grantee=$2::regrole
				AND x.privilege_type='SELECT' AND a.attnum>0 AND NOT a.attisdropped ORDER BY 1`, table, role)
		}
		lcSameSet(t, "registry writer columns on payments.stripe_sessions", colsOf("payments.stripe_sessions", "commerce_payment_registry_writer"),
			[]string{"tenant_id", "store_id", "attempt_id", "environment", "account_id", "payment_intent_id"})
		lcSameSet(t, "registry writer columns on payments.stripe_refunds", colsOf("payments.stripe_refunds", "commerce_payment_registry_writer"),
			[]string{"tenant_id", "store_id", "id", "attempt_id", "environment", "stripe_refund_id", "account_id", "amount_minor", "currency"})
		lcSameSet(t, "registry writer columns on checkout.payment_attempts", colsOf("checkout.payment_attempts", "commerce_payment_registry_writer"),
			[]string{"tenant_id", "store_id", "id", "method_code", "environment", "connection_id", "currency", "qualification_id", "created_at", "order_id"})
		// §6.3 vectors (the brief's PF11 vector, with its arithmetic): HKD 25,640 settled for TWD 100000 minor, fee 1,046 ->
		// 1046 x 100000 / 25640 = 4079.56 -> /100 = 40.80 -> half-up 41 -> x100 = 4100 -> fee_store_minor = -4100.
		type vec struct{ fee, store, settle, want int64 }
		vectors := []vec{{1046, 100000, 25640, -4100}, {1, 50, 1, -100}, {49, 100, 100, 0}, {0, 100000, 25640, 0}, {-1500, 250000, 64100, 5900}}
		for k := int64(0); k < 6; k++ { // exact halves: fee*store = (k+1/2)*settle*100 -> rounds up to k+1
			vectors = append(vectors, vec{fee: 2*k + 1, store: 700, settle: 14, want: -100 * (k + 1)})
			// P2-4 (Opus): half away from zero on the absolute value. A returned fee (negative) at an exact half credits the SAME magnitude, and a
			// refund (negative store and settle amounts) reverses its charge's fee share exactly.
			vectors = append(vectors, vec{fee: -(2*k + 1), store: 700, settle: 14, want: 100 * (k + 1)})
			vectors = append(vectors, vec{fee: 2*k + 1, store: -700, settle: -14, want: -100 * (k + 1)})
			vectors = append(vectors, vec{fee: -(2*k + 1), store: -700, settle: -14, want: 100 * (k + 1)})
		}
		for _, v := range vectors {
			if got := pslFee(v.fee, v.store, v.settle); got != v.want {
				t.Fatalf("Go reference %+v = %d", v, got)
			}
			var got int64
			if err := o.QueryRow(ctx, `SELECT payments.settlement_fee_store($1,$2,$3)`, v.fee, v.store, v.settle).Scan(&got); err != nil || got != v.want {
				t.Fatalf("SQL %+v = %d (%v), want %d", v, got, err, v.want)
			}
		}
		for i := int64(1); i <= 300; i++ { // SQL numeric equals the exact Go result on a spread of inputs
			fee, store, settle := (i*37)%4000+1, ((i*91)%900+1)*100, (i*53)%90000+100
			var got int64
			if err := o.QueryRow(ctx, `SELECT payments.settlement_fee_store($1,$2,$3)`, fee, store, settle).Scan(&got); err != nil || got != pslFee(fee, store, settle) {
				t.Fatalf("SQL vs Go (%d,%d,%d): %d vs %d (%v)", fee, store, settle, got, pslFee(fee, store, settle), err)
			}
		}
	})

	// ---------------------------------------------------------------------------------------------------------------------
	// PF10 sync: attribution in SQL, unattributed reasons, replay, changed content, cross-checks, fail-closed rows.
	// ---------------------------------------------------------------------------------------------------------------------
	cap1 := func(o rfxOrder) int64 { return o.captured }
	feeA2, feeA1, feeP1, feeA3, feeC1, feeC2 := int64(33), int64(31), int64(29), int64(35), int64(31), int64(29)
	t.Run("PF10_sync", func(t *testing.T) {
		// week 0 first (the dispute of A2 in week 1 needs A2's charge line for its ratio)
		rep := e.sync(t, w0, w1, pslCharge("txn_ChargeA2", e.oa2.pi, cap1(e.oa2), settleOf(cap1(e.oa2)), feeA2, d(w0, 1)))
		if rep.Inserted != 1 || rep.Unattributed != 0 || rep.Mismatch != 0 || rep.Difference["HKD"] != 0 {
			t.Fatalf("week 0 sync: %+v", rep)
		}
		week1 := []stripe.BalanceTransaction{
			pslCharge("txn_ChargeP1", e.op1.pi, cap1(e.op1), settleOf(cap1(e.op1)), feeP1, d(w1, 1)),
			pslCharge("txn_ChargeA1", e.oa1.pi, cap1(e.oa1), settleOf(cap1(e.oa1)), feeA1, d(w1, 1)),
			pslRefund("txn_RefundA1", e.refundA1, 800, settleOf(800), 0, d(w1, 2)),
			pslDispute("txn_DisputeA2", "dp_A2", e.oa2.pi, cap1(e.oa2), settleOf(cap1(e.oa2)), 1500, false, d(w1, 3)), // its charge line is in the earlier batch
			// another primary account's charge, and the types that are never a store line
			pslCharge("txn_ChargeX1", e.ox1.pi, cap1(e.ox1), settleOf(cap1(e.ox1)), 30, d(w1, 1)),
			pslRaw("txn_Payout1", "payout", "payout", -500000, pslI(0), pslS("po_1"), d(w1, 4)),
			pslRaw("txn_StripeFee1", "stripe_fee", "fee", -100, pslI(0), nil, d(w1, 4)),
			func() stripe.BalanceTransaction { // Stripe sent no fee: fails closed
				t := pslCharge("txn_NoFee", e.oa3.pi, 2500, 641, 0, d(w1, 5))
				t.Fee = nil
				return t
			}(),
			func() stripe.BalanceTransaction { // Stripe sent no exchange rate: fails closed
				t := pslCharge("txn_NoRate", e.oa3.pi, 2500, 641, 30, d(w1, 5))
				t.ExchangeRate = nil
				return t
			}(),
			func() stripe.BalanceTransaction { // a dispute whose reporting category is not a known one: blocks close, never dropped
				t := pslDispute("txn_DisputeOdd", "dp_odd", e.oa2.pi, 100, 26, 0, false, d(w1, 5))
				t.ReportingCategory = pslS("dispute_other")
				return t
			}(),
		}
		rep = e.sync(t, w1, w2, week1...)
		if rep.Inserted != 4 || rep.Unattributed != 6 || rep.Duplicate != 0 || rep.Mismatch != 0 || rep.Fetched != 10 {
			t.Fatalf("week 1 sync: %+v", rep)
		}
		for _, cur := range []string{"HKD"} {
			if rep.StripeNet[cur] != rep.RecordedNet[cur] || rep.Difference[cur] != 0 {
				t.Fatalf("reconciliation %s: stripe=%d recorded=%d", cur, rep.StripeNet[cur], rep.RecordedNet[cur])
			}
		}
		// attribution: every line is in exactly its own store's scope
		for txn, store := range map[string]string{"txn_ChargeP1": e.storeP, "txn_ChargeA1": e.storeA, "txn_RefundA1": e.storeA, "txn_DisputeA2": e.storeA, "txn_ChargeA2": e.storeA} {
			if _, got, _, _, _, _ := e.line(t, txn); got != store {
				t.Fatalf("%s attributed to %s, want %s", txn, got, store)
			}
		}
		// amounts, kinds, signs and the fee conversion (a dispute uses the ORIGINAL charge's ratio)
		if k, _, sm, fm, mm, _ := e.line(t, "txn_ChargeA1"); k != "CHARGE" || sm != cap1(e.oa1) || fm != pslFee(feeA1, cap1(e.oa1), settleOf(cap1(e.oa1))) || mm != nil {
			t.Fatalf("A1 charge line: %s %d %d %v", k, sm, fm, mm)
		}
		if k, _, sm, fm, mm, _ := e.line(t, "txn_RefundA1"); k != "REFUND" || sm != -800 || fm != 0 || mm != nil {
			t.Fatalf("A1 refund line: %s %d %d %v", k, sm, fm, mm)
		}
		if k, _, sm, fm, mm, _ := e.line(t, "txn_DisputeA2"); k != "DISPUTE" || sm != -cap1(e.oa2) || fm != pslFee(1500, cap1(e.oa2), settleOf(cap1(e.oa2))) || mm != nil {
			t.Fatalf("A2 dispute line: %s %d %d %v", k, sm, fm, mm)
		}
		for txn, want := range map[string]string{"txn_ChargeX1": "foreign_connection", "txn_Payout1": "unsupported_type", "txn_StripeFee1": "unsupported_type",
			"txn_NoFee": "unsupported_type", "txn_NoRate": "unsupported_type", "txn_DisputeOdd": "unmapped_source"} {
			if got := e.unattributed(t, txn); got != want {
				t.Fatalf("%s reason = %s, want %s", txn, got, want)
			}
		}
		// an unknown payment intent: unmapped_source (placed in week 4 so it blocks no closable period)
		rep = e.sync(t, w4, w4.Add(7*24*time.Hour), pslCharge("txn_Unknown", "pi_unknownintent", 2500, 641, 30, d(w4, 1)))
		if rep.Unattributed != 1 || e.unattributed(t, "txn_Unknown") != "unmapped_source" {
			t.Fatalf("unmapped: %+v", rep)
		}
		// replay inserts nothing; changed content for one txn id is an integrity conflict (PT409) and writes nothing
		rep = e.sync(t, w1, w2, week1...)
		if rep.Inserted != 0 || rep.Unattributed != 0 || rep.Duplicate != 10 {
			t.Fatalf("replay: %+v", rep)
		}
		before := countRows(t, e.f.owner, `SELECT (SELECT count(*) FROM payments.settlement_lines)+(SELECT count(*) FROM payments.settlement_unattributed)`)
		changed := append([]stripe.BalanceTransaction{pslCharge("txn_ChargeP1", e.op1.pi, cap1(e.op1), settleOf(cap1(e.op1)), feeP1+1, d(w1, 1))}, pslCharge("txn_Brand", e.op1.pi, 2500, 641, 30, d(w1, 2)))
		pslWantRefused(t, "changed content", e.syncErr(w1, w2, changed...), "settlement_content_changed")
		if after := countRows(t, e.f.owner, `SELECT (SELECT count(*) FROM payments.settlement_lines)+(SELECT count(*) FROM payments.settlement_unattributed)`); after != before {
			t.Fatalf("a refused batch wrote rows: %d -> %d", before, after)
		}
		// malformed input is refused whole (22023), never partially applied
		bad := pslCharge("txn_Bad", e.op1.pi, 2500, 641, 30, d(w1, 2))
		bad.Currency = "hkd"
		if err := e.syncErr(w1, w2, bad); err == nil {
			t.Fatal("a lower-case currency was accepted")
		}
	})

	// A payment intent shared by two sessions (a provider anomaly the payments recording path tolerates) is ambiguous: never attributed by guess.
	t.Run("PF10_ambiguous_payment_intent", func(t *testing.T) {
		var original string
		if err := e.f.owner.QueryRow(ctx, `SELECT payment_intent_id FROM payments.stripe_sessions WHERE attempt_id=$1`, e.oa3.attempt).Scan(&original); err != nil {
			t.Fatal(err)
		}
		e.fixtureExec(t, `UPDATE payments.stripe_sessions SET payment_intent_id=$2 WHERE attempt_id=$1`, e.oa3.attempt, e.oa1.pi) // disclosed fixture: a repeated PI id
		defer e.fixtureExec(t, `UPDATE payments.stripe_sessions SET payment_intent_id=$2 WHERE attempt_id=$1`, e.oa3.attempt, original)
		rep := e.sync(t, w4, w4.Add(7*24*time.Hour), pslCharge("txn_Ambiguous", e.oa1.pi, cap1(e.oa1), settleOf(cap1(e.oa1)), 31, d(w4, 2)))
		if rep.Unattributed != 1 || rep.Inserted != 0 || e.unattributed(t, "txn_Ambiguous") != "unmapped_source" {
			t.Fatalf("ambiguous payment intent: %+v", rep)
		}
	})

	// P2-1 (Opus review): the connection named by the CLI must be the designated platform connection.
	t.Run("PF10_platform_connection_assertion", func(t *testing.T) {
		_, err := e.reg.RecordSettlementLines(ctx, e.op, e.ox1.s.connection, pslTicket, nil, w1, w2)
		pslWantRefused(t, "a connection that is not the platform connection", err, "not_platform_connection")
		if _, err := e.reg.RecordSettlementLines(ctx, e.op, e.plat.connection, "short", nil, w1, w2); err == nil {
			t.Fatal("an invalid ticket was accepted")
		}
	})

	// P1-1 (Opus review): a fact-lag mismatch is transient. The sync runs before the worker recorded the CAPTURED fact (webhook lag),
	// and before a refund reached a terminal state (pending, or one that only ever FAILED). A re-sync of the same transactions must
	// clear the mismatch while no statement exists, and the store must then close. Fixtures (disclosed, owner pool, replica mode):
	// the fact rows are moved aside to simulate the lag and put back (the refund fact as FAILED) to simulate the worker catching up.
	t.Run("PF10_recheck_transient_mismatch", func(t *testing.T) {
		restoreCaptured := e.hideCaptured(t, e.oc1.attempt)
		restoreRefund := e.hideRefundFact(t, e.refundC2ID)
		batch := []stripe.BalanceTransaction{
			pslCharge("txn_ChargeC1", e.oc1.pi, cap1(e.oc1), settleOf(cap1(e.oc1)), feeC1, d(w1, 1)),
			pslCharge("txn_ChargeC2", e.oc2.pi, cap1(e.oc2), settleOf(cap1(e.oc2)), feeC2, d(w1, 1)),
			pslRefund("txn_RefundC2", e.refundC2, 800, settleOf(800), 0, d(w1, 2)),
			pslRefundFailure("txn_RefundFailC2", e.refundC2, 800, settleOf(800), d(w1, 3)),
		}
		rep := e.sync(t, w1, w2, batch...)
		if rep.Inserted != 4 || rep.Mismatch != 3 {
			t.Fatalf("first sync under fact lag: %+v (want 4 inserted, 3 no_fact)", rep)
		}
		for _, txn := range []string{"txn_ChargeC1", "txn_RefundC2", "txn_RefundFailC2"} {
			if _, _, _, _, mm, _ := e.line(t, txn); mm == nil || *mm != "no_fact" {
				t.Fatalf("%s mismatch = %v, want no_fact", txn, mm)
			}
		}
		// the worker catches up: CAPTURED appears; the refund turns out to have FAILED (it never succeeded)
		restoreCaptured()
		restoreRefund("FAILED")
		rep = e.sync(t, w1, w2, batch...)
		if rep.Inserted != 0 || rep.Unattributed != 0 || rep.Mismatch != 0 || rep.Rechecked != 3 || rep.Duplicate != 1 {
			t.Fatalf("re-sync after the facts arrived: %+v (want 3 rechecked, the already-clean C2 charge a duplicate)", rep)
		}
		// a third identical sync changes nothing
		if rep = e.sync(t, w1, w2, batch...); rep.Rechecked != 0 || rep.Duplicate != 4 || rep.Inserted != 0 {
			t.Fatalf("third sync: %+v", rep)
		}
		for _, txn := range []string{"txn_ChargeC1", "txn_ChargeC2", "txn_RefundC2", "txn_RefundFailC2"} {
			if _, _, _, _, mm, st := e.line(t, txn); mm != nil || st != nil {
				t.Fatalf("%s still blocked after the facts arrived: mismatch=%v statement=%v", txn, mm, st)
			}
		}
		// a refund that only ever FAILED nets to zero: the REFUND (-800) and its REFUND_FAILURE (+800) cancel
		if _, _, sm, _, _, _ := e.line(t, "txn_RefundC2"); sm != -800 {
			t.Fatalf("refund line store_minor = %d", sm)
		}
	})

	// ---------------------------------------------------------------------------------------------------------------------
	// PF11 close
	// ---------------------------------------------------------------------------------------------------------------------
	var stmtA0, stmtA1, stmtA2, stmtA3 string
	var netA0, netA1 int64
	t.Run("PF11_close", func(t *testing.T) {
		// a period that has not reached +72 h is refused; use the week that contains now (always in the future relative to its own end)
		nowTPE := time.Now().In(pslTPE)
		thisMonday := pslDay(nowTPE.Year(), nowTPE.Month(), nowTPE.Day())
		for thisMonday.Weekday() != time.Monday {
			thisMonday = thisMonday.AddDate(0, 0, -1)
		}
		_, err := e.closeWeek(thisMonday, e.storeA)
		pslWantRefused(t, "period not closable", err, "period_not_closable")
		if _, err := e.reg.SettlementClose(ctx, e.op, "SANDBOX", "2026-09-08", "op@test", e.storeA, pslTicket); err == nil { // a Tuesday
			t.Fatal("a non-Monday period was accepted")
		}
		// P1-2: a window may not cover time that has not passed (a sync run Thursday with --to next Monday would claim the weekend)
		now := time.Now().Truncate(time.Second)
		if err := e.syncErr(now.Add(-24*time.Hour), now.Add(6*24*time.Hour)); err == nil {
			t.Fatal("a window ending in the future was accepted by the registrar")
		}
		if err := e.syncErr(now.Add(-24*time.Hour), now.Add(-5*time.Minute)); err == nil {
			t.Fatal("a window ending inside the 15-minute settling margin was accepted by the registrar")
		}
		for _, to := range []time.Time{now.Add(time.Hour), now.Add(-5 * time.Minute)} {
			var state string
			if err := e.f.owner.QueryRow(ctx, `SELECT 'x' FROM (SELECT payments.record_settlement_lines($1::uuid,$2::uuid,$3::uuid,'SANDBOX','[]'::jsonb,$4::timestamptz,$5::timestamptz)) q`,
				e.op.TenantID, e.op.StoreID, e.op.PrincipalID, to.Add(-24*time.Hour), to).Scan(&state); sqlState(err) != "22023" {
				t.Fatalf("SQL accepted a window ending at %s: %v", to.Format(time.RFC3339), err)
			}
		}
		// P2-9: the wire lists whole unix seconds, so a fractional bound would claim a sliver that was never listed
		var frac string
		if err := e.f.owner.QueryRow(ctx, `SELECT 'x' FROM (SELECT payments.record_settlement_lines($1::uuid,$2::uuid,$3::uuid,'SANDBOX','[]'::jsonb,$4::timestamptz,$5::timestamptz)) q`,
			e.op.TenantID, e.op.StoreID, e.op.PrincipalID, w0.Add(500*time.Millisecond), w1).Scan(&frac); sqlState(err) != "22023" {
			t.Fatalf("SQL accepted a fractional-second window: %v", err)
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_sync_runs WHERE window_to>now()-interval '15 minutes'`); n != 0 {
			t.Fatalf("%d coverage rows claim time that has not settled", n)
		}
		// close requires a sync that covered [period_start - 7 d, period_end): week 0 needs [08-24, 09-07) and 08-24..08-31 was never synced
		_, err = e.closeWeek(w0, e.storeA)
		pslWantRefused(t, "sync required", err, "sync_required")
		e.sync(t, w0.Add(-7*24*time.Hour), w0) // an empty coverage run
		// an unmapped source (here the dispute-shaped adjustment Stripe did not classify as a known pair) blocks every store's close
		_, err = e.closeWeek(w1, e.storeA)
		pslWantRefused(t, "unmapped source blocks", err, "settlement_unattributed")
		// S2-OPEN-1 (0163 §6.6): the sanctioned clearing path is the append-only resolve step — the exact call stripe-admin
		// settlement-resolve makes. The former owner-pool DELETE of this row is gone; the row itself stays as sync wrote it.
		if _, err := e.reg.SettlementResolve(ctx, e.op, "SANDBOX", "txn_DisputeOdd", stripeadmin.ResolveNotStoreRevenue, "", "",
			"Dispute-shaped adjustment with an unclassified category; not a store sale.", "op@test", pslTicket); err != nil {
			t.Fatalf("resolve txn_DisputeOdd: %v", err)
		}
		// week 1 for A while week 0 is still open
		_, err = e.closeWeek(w1, e.storeA)
		pslWantRefused(t, "previous period open", err, "previous_period_open")
		// week 0 for every store with unassigned lines: only A has any (A2's charge)
		got, err := e.closeWeek(w0, "")
		if err != nil || len(got) != 1 || got[0].StoreID != e.storeA || got[0].LineCount != 1 || got[0].Replayed {
			t.Fatalf("close week 0: %+v %v", got, err)
		}
		stmtA0, netA0 = got[0].StatementID, got[0].NetPayableMinor
		feeStoreA2 := pslFee(feeA2, cap1(e.oa2), settleOf(cap1(e.oa2)))
		if netA0 != cap1(e.oa2)+feeStoreA2 {
			t.Fatalf("A week 0 net = %d, want %d", netA0, cap1(e.oa2)+feeStoreA2)
		}
		// replay returns the same statement
		again, err := e.closeWeek(w0, "")
		if err != nil || len(again) != 1 || again[0].StatementID != stmtA0 || !again[0].Replayed {
			t.Fatalf("close replay: %+v %v", again, err)
		}
		// week 1 for EVERY store (the transient C mismatches were cleared by the re-sync): A, C and the platform store
		got, err = e.closeWeek(w1, "")
		by := map[string]stripeadmin.ClosedStatement{}
		for _, s := range got {
			by[s.StoreID] = s
		}
		if err != nil || len(got) != 3 || by[e.storeA].LineCount != 3 || by[e.storeC].LineCount != 4 || by[e.storeP].LineCount != 1 {
			t.Fatalf("close week 1 (all stores): %+v %v", got, err)
		}
		stmtA1, netA1 = by[e.storeA].StatementID, by[e.storeA].NetPayableMinor
		wantFees := pslFee(feeA1, cap1(e.oa1), settleOf(cap1(e.oa1))) + pslFee(1500, cap1(e.oa2), settleOf(cap1(e.oa2)))
		if want := cap1(e.oa1) - 800 - cap1(e.oa2) + wantFees; netA1 != want || netA1 >= 0 { // the net is negative and carries forward
			t.Fatalf("A week 1 net = %d, want %d (negative)", netA1, want)
		}
		wantC := cap1(e.oc1) + cap1(e.oc2) + pslFee(feeC1, cap1(e.oc1), settleOf(cap1(e.oc1))) + pslFee(feeC2, cap1(e.oc2), settleOf(cap1(e.oc2)))
		if by[e.storeC].NetPayableMinor != wantC { // refunded 800 and failure +800: net refund 0
			t.Fatalf("C week 1 net = %d, want %d", by[e.storeC].NetPayableMinor, wantC)
		}
		// a charge dated in week 1 that arrives after week 1 closed: it rolls into the next statement, never into the closed one
		e.sync(t, w1, w2, pslCharge("txn_ChargeA3", e.oa3.pi, cap1(e.oa3), settleOf(cap1(e.oa3)), feeA3, d(w1, 6)))
		if _, _, _, _, _, st := e.line(t, "txn_ChargeA3"); st != nil {
			t.Fatal("a late line joined a closed statement")
		}
		// the dispute is won in week 2
		e.sync(t, w2, w3, pslDispute("txn_ReverseA2", "dp_A2", e.oa2.pi, cap1(e.oa2), settleOf(cap1(e.oa2)), 1500, true, d(w2, 1)))
		// week 2 with a 500 bps platform fee (data on the platform row; 0 for the earlier weeks)
		v, err := e.reg.PlatformOpen(ctx, e.op, "SANDBOX", true, 500, e.version)
		if err != nil {
			t.Fatalf("set platform fee: %v", err)
		}
		e.version = v
		// P2-5 quiet week: C and the platform store have no line this week; every store that has an earlier statement still gets one, so the
		// chain never breaks (previous_period_open would otherwise abort the NEXT all-store close for everybody)
		got, err = e.closeWeek(w2, "")
		by = map[string]stripeadmin.ClosedStatement{}
		for _, s := range got {
			by[s.StoreID] = s
		}
		if err != nil || len(got) != 3 || by[e.storeA].LineCount != 2 || by[e.storeC].LineCount != 0 || by[e.storeP].LineCount != 0 ||
			by[e.storeC].NetPayableMinor != 0 || by[e.storeP].NetPayableMinor != 0 {
			t.Fatalf("close week 2 (all stores, quiet week): %+v %v", got, err)
		}
		stmtA2 = by[e.storeA].StatementID
		var captured, refunded, dispute, fee, pfee, carried, net int64
		var bps int
		if err := e.f.owner.QueryRow(ctx, `SELECT captured_minor,refunded_minor,dispute_minor,stripe_fee_minor,platform_fee_bps,platform_fee_minor,carried_in_minor,net_payable_minor
			FROM payments.settlement_statements WHERE id=$1`, stmtA2).Scan(&captured, &refunded, &dispute, &fee, &bps, &pfee, &carried, &net); err != nil {
			t.Fatal(err)
		}
		wantFee := pslFee(feeA3, cap1(e.oa3), settleOf(cap1(e.oa3))) + pslFee(-1500, cap1(e.oa2), settleOf(cap1(e.oa2)))
		wantPFee := int64(100 * ((cap1(e.oa3) * 500 / 10000) / 100)) // 2500*5% = 125 -> half-up to the NT$ step = 100
		if captured != cap1(e.oa3) || refunded != 0 || dispute != -cap1(e.oa2) || fee != wantFee || bps != 500 || pfee != wantPFee || carried != netA1 ||
			net != captured-refunded-dispute+fee-pfee+carried {
			t.Fatalf("week 2: captured=%d refunded=%d dispute=%d fee=%d(%d) bps=%d pfee=%d(%d) carried=%d(%d) net=%d", captured, refunded, dispute, fee, wantFee, bps, pfee, wantPFee, carried, netA1, net)
		}
		if _, _, _, _, _, st := e.line(t, "txn_ChargeA3"); st == nil || *st != stmtA2 {
			t.Fatal("the late line did not roll into the week 2 statement")
		}
		// the week 2 replay (every store) and the audit trail
		if r, err := e.closeWeek(w2, ""); err != nil || len(r) != 3 || !r[0].Replayed || !r[1].Replayed || !r[2].Replayed {
			t.Fatalf("week 2 replay: %+v %v", r, err)
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.close' AND details->>'target_store'=$1`, e.storeA); n != 3 {
			t.Fatalf("close audit rows for A = %d, want 3", n)
		}
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.sync'`); n < 5 {
			t.Fatalf("sync audit rows = %d", n)
		}
		// platform fee bps 0 earlier: the week 0 and week 1 statements carry none
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_statements WHERE id IN ($1,$2) AND platform_fee_bps=0 AND platform_fee_minor=0`, stmtA0, stmtA1); n != 2 {
			t.Fatal("platform fee applied while bps was 0")
		}
		// week 3: store B's lines arrive with mismatches that are NOT transient (the amount differs from the CAPTURED fact; a refund failure
		// without a FAILED fact), so the all-store close refuses, while a quiet-week statement for A alone is still allowed
		rep := e.sync(t, w3, w4,
			pslCharge("txn_ChargeB1", e.ob1.pi, cap1(e.ob1)+100, settleOf(cap1(e.ob1)+100), 40, d(w3, 1)),
			pslRefund("txn_RefundB1", e.refundB1, 800, settleOf(800), 0, d(w3, 2)),
			pslRefundFailure("txn_RefundFailB1", e.refundB1, 800, settleOf(800), d(w3, 3)))
		if rep.Inserted != 3 || rep.Mismatch != 2 {
			t.Fatalf("store B sync: %+v", rep)
		}
		if _, _, _, _, mm, _ := e.line(t, "txn_ChargeB1"); mm == nil || *mm != "amount" {
			t.Fatalf("B1 charge mismatch = %v, want amount", mm)
		}
		if _, _, _, _, mm, _ := e.line(t, "txn_RefundB1"); mm != nil { // a SUCCEEDED refund fact: amount taken from payments.stripe_refunds
			t.Fatalf("B1 refund mismatch = %v", mm)
		}
		if _, _, _, _, mm, _ := e.line(t, "txn_RefundFailB1"); mm == nil || *mm != "no_fact" {
			t.Fatalf("B1 refund failure mismatch = %v, want no_fact", mm)
		}
		_, err = e.closeWeek(w3, "")
		pslWantRefused(t, "mismatch blocks", err, "settlement_mismatch")
		got, err = e.closeWeek(w3, e.storeA)
		if err != nil || len(got) != 1 || got[0].LineCount != 0 {
			t.Fatalf("quiet week 3 (A alone): %+v %v", got, err)
		}
		stmtA3 = got[0].StatementID
	})

	// ---------------------------------------------------------------------------------------------------------------------
	// PF12 payout (record only)
	// ---------------------------------------------------------------------------------------------------------------------
	t.Run("PF12_payout", func(t *testing.T) {
		past := time.Now().Add(-time.Hour).Truncate(time.Second)
		pay := func(st string, ref string, amount int64, at time.Time) (time.Time, error) {
			return e.reg.SettlementPayout(ctx, e.op, st, ref, amount, at, "op@test", pslTicket)
		}
		_, err := pay(stmtA1, "BANK-REF-A1", 1, past) // net <= 0 can never be paid, whatever the amount
		pslWantRefused(t, "payout of a negative net", err, "payout_amount_mismatch")
		_, err = pay(stmtA0, "BANK-REF-A0", netA0+100, past)
		pslWantRefused(t, "payout amount != net", err, "payout_amount_mismatch")
		if _, err = pay(stmtA0, "BANK-REF-A0", netA0, time.Now().Add(time.Hour)); err == nil {
			t.Fatal("a payout in the future was accepted")
		}
		if _, err = pay(stmtA0, "x y", netA0, past); err == nil {
			t.Fatal("an invalid reference was accepted")
		}
		at, err := pay(stmtA0, "BANK-REF-A0", netA0, past)
		if err != nil || !at.Equal(past) {
			t.Fatalf("payout: %v %v", at, err)
		}
		if again, err := pay(stmtA0, "BANK-REF-A0", netA0, past); err != nil || !again.Equal(past) { // identical replay returns the stored time
			t.Fatalf("payout replay: %v %v", again, err)
		}
		_, err = pay(stmtA0, "BANK-REF-OTHER", netA0, past)
		pslWantRefused(t, "second payout reference", err, "payout_already_recorded")
		_, err = pay(stmtA0, "BANK-REF-A0", netA0, past.Add(time.Minute))
		pslWantRefused(t, "second payout time", err, "payout_already_recorded")
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.payout'`); n != 1 {
			t.Fatalf("payout audit rows = %d, want 1 (a replay writes none)", n)
		}
		if _, err := pay("5a5a5a5a-5a5a-4a5a-8a5a-5a5a5a5a5aff", "BANK-REF-ZZ", 100, past); err == nil {
			t.Fatal("a payout for an unknown statement was accepted")
		}
	})

	t.Run("PF12_ticket_in_audit", func(t *testing.T) {
		for _, action := range []string{"stripe.settlement.sync", "stripe.settlement.close", "stripe.settlement.payout"} {
			if n := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events WHERE action=$1 AND details->>'ticket'=$2`, action, pslTicket); n == 0 {
				t.Fatalf("no %s audit row carries the operator ticket", action)
			}
		}
	})

	// ---------------------------------------------------------------------------------------------------------------------
	// PF13 merchant read and the CSV
	// ---------------------------------------------------------------------------------------------------------------------
	t.Run("PF13_merchant_read_and_csv", func(t *testing.T) {
		tokenA, tokenB := e.a.f.tokens["a"], e.b.f.tokens["a"]
		listPath := "/v1/admin/stores/" + e.storeA + "/settlements"
		status, raw, hdr := e.call(http.MethodGet, listPath, tokenA, nil, "")
		if status != 200 || !strings.Contains(hdr.Get("Cache-Control"), "no-store") {
			t.Fatalf("list: %d %s %v", status, raw, hdr)
		}
		var list settlement.List
		if err := json.Unmarshal(raw, &list); err != nil || len(list.Statements) != 4 || list.Statements[0].StatementID != stmtA3 ||
			list.Statements[0].PeriodStart != "2026-09-21" || list.Statements[0].LineCount != 0 || len(list.Statements[0].Lines) != 0 {
			t.Fatalf("list body: %s", raw)
		}
		byID := map[string]settlement.Statement{}
		for _, s := range list.Statements {
			byID[s.StatementID] = s
		}
		if s := byID[stmtA0]; !s.Paid || s.PayoutRef == nil || *s.PayoutRef != "BANK-REF-A0" || s.NetPayableMinor != netA0 {
			t.Fatalf("paid statement: %+v", s)
		}
		if s := byID[stmtA3]; s.LineCount != 0 || s.Paid || s.PeriodStart != "2026-09-21" {
			t.Fatalf("quiet-week statement: %+v", s)
		}
		if s := byID[stmtA1]; s.Paid || s.NetPayableMinor != netA1 || s.RefundedMinor != 800 {
			t.Fatalf("open statement: %+v", s)
		}
		// before + limit
		status, raw, _ = e.call(http.MethodGet, listPath+"?before=2026-09-14&limit=1", tokenA, nil, "")
		var one settlement.List
		if err := json.Unmarshal(raw, &one); status != 200 || err != nil || len(one.Statements) != 1 || one.Statements[0].StatementID != stmtA1 {
			t.Fatalf("before/limit: %d %s", status, raw)
		}
		// detail with lines; no ids of Stripe, no settlement-currency amounts, no other store's data
		status, raw, _ = e.call(http.MethodGet, listPath+"/"+stmtA1, tokenA, nil, "")
		var detail settlement.Detail
		if err := json.Unmarshal(raw, &detail); status != 200 || err != nil || len(detail.Statement.Lines) != 3 {
			t.Fatalf("detail: %d %s", status, raw)
		}
		for _, l := range detail.Statement.Lines {
			if !strings.HasPrefix(l.OrderNumber, "LC-") || l.TxnDate == "" {
				t.Fatalf("line: %+v", l)
			}
		}
		for _, body := range [][]byte{raw, psMustJSON(t, list)} {
			s := string(body)
			for _, bad := range []string{"txn_Charge", "txn_Refund", "txn_Dispute", "txn_Reverse", e.refundA1, e.oa1.pi, e.oa2.pi, e.plat.account, "tenant", "HKD", "settle_", "exchange_rate"} {
				if strings.Contains(s, bad) {
					t.Fatalf("merchant body leaks %q: %s", bad, s)
				}
			}
		}
		// store A cannot read B's statements, and B cannot read A's statement id through its own store
		if status, _, _ = e.call(http.MethodGet, "/v1/admin/stores/"+e.storeB+"/settlements", tokenA, nil, ""); status == 200 {
			t.Fatal("A's token read store B")
		}
		if status, _, _ = e.call(http.MethodGet, "/v1/admin/stores/"+e.storeB+"/settlements/"+stmtA1, tokenB, nil, ""); status != 404 {
			t.Fatalf("B read A's statement id through B's store: %d", status)
		}
		status, raw, _ = e.call(http.MethodGet, "/v1/admin/stores/"+e.storeB+"/settlements", tokenB, nil, "")
		var bl settlement.List
		if err := json.Unmarshal(raw, &bl); status != 200 || err != nil || len(bl.Statements) != 0 {
			t.Fatalf("B's list: %d %s", status, raw)
		}
		// billing:manage is required: a store member without it is refused (403), an unknown token is 401
		memberToken, _ := e.member(t, e.oa1, "orders:read")
		if status, _, _ = e.call(http.MethodGet, listPath, memberToken, nil, ""); status != 403 {
			t.Fatalf("member without billing:manage: %d", status)
		}
		if status, _, _ = e.call(http.MethodGet, listPath, strings.Repeat("a", 43), nil, ""); status != 401 {
			t.Fatalf("unknown token: %d", status)
		}
		for _, p := range []string{"?limit=0", "?limit=53", "?before=2026-9-1", "?foo=1", "?limit=1&limit=2"} {
			if status, _, _ = e.call(http.MethodGet, listPath+p, tokenA, nil, ""); status != 422 {
				t.Fatalf("%s: %d", p, status)
			}
		}
		if status, _, _ = e.call(http.MethodGet, listPath+"/not-a-uuid", tokenA, nil, ""); status != 422 {
			t.Fatalf("bad statement id: %d", status)
		}
		if status, _, _ = e.call(http.MethodPost, listPath, tokenA, map[string]string{"Idempotency-Key": "k"}, "{}"); status != 405 {
			t.Fatalf("POST: %d", status)
		}

		// operator CSV: header without any PII column, 0600, refuses to overwrite, formula guard on text cells only
		st, err := e.reg.SettlementStatement(ctx, e.op, stmtA1)
		if err != nil || len(st.Lines) != 3 {
			t.Fatalf("operator read: %+v %v", st, err)
		}
		path := filepath.Join(t.TempDir(), "settlement-a1.csv")
		if err := settlement.WriteFile(path, st); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("export mode: %v %v", info, err)
		}
		body, _ := os.ReadFile(path)
		header, _, _ := strings.Cut(string(body), "\r\n")
		if header != "\xEF\xBB\xBFperiod_start,period_end,order_number,kind,amount,stripe_fee,txn_date" {
			t.Fatalf("csv header: %q", header)
		}
		for _, pii := range []string{"email", "phone", "name", "address", "recipient", "buyer"} {
			if strings.Contains(strings.ToLower(header), pii) {
				t.Fatalf("csv header has a PII column %q", pii)
			}
		}
		if !strings.Contains(string(body), "\r\nline_count,3\r\n") || !strings.Contains(string(body), "\r\nnet_payable,-") { // totals block, negative net stays numeric
			t.Fatalf("csv totals block: %q", string(body))
		}
		if err := settlement.WriteFile(path, st); err == nil {
			t.Fatal("the export overwrote an existing file")
		}
		if err := settlement.WriteFile("relative.csv", st); err == nil {
			t.Fatal("a relative export path was accepted")
		}
		evil := st
		evil.Lines = []settlement.Line{{OrderNumber: "=cmd|' /C calc'!A0", Kind: "CHARGE", StoreMinor: 100, FeeStoreMinor: -4100, TxnDate: "2026-09-15"}}
		var out strings.Builder
		if err := settlement.WriteCSV(&out, evil); err != nil || !strings.Contains(out.String(), "'=cmd|") || !strings.Contains(out.String(), ",-41.00,") {
			t.Fatalf("formula guard / numeric cell: %q %v", out.String(), err)
		}
	})
}

func psMustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
