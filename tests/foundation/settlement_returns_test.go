package foundation_test

// SA-1 smoke of the settlement returns / reconciliation / payables gates PF16-PF18 (contracts/stripe-platform-account-v1.md §6.7-§6.9,
// migration 0200). Author's own minimal reproduction; the independent vector gate is SA-3's platform_settlement_gate_test.go. Tier: MOCK balance
// transactions (never Stripe) + REAL_PG through pslEnv: payments through the real capture path, ledger rows only through stripeadmin.Registrar or
// the operator definers, merchant reads through the real HTTP handler. Test names carry "SA1" so they never collide with SA-3's names.
//
// The file compiles and its setup passes on the base (before 0200), so a failure there is behaviour, not setup:
//   - PF16: the merchant DTO must carry returned_in_minor / payout_returned (base: keys absent), then the return entry point must exist and carry;
//   - PF17: a discrepant resync must make close refuse `sync_unreconciled` (base: close succeeds), and the old nine-argument identity must be gone;
//   - PF18: the payables entry point must list positive unpaid statements only.
// Owner-pool fixtures are named at their call site: the close probe in a rolled-back transaction, and the PF16/PF17 immutability and catalog reads.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/integrations/psp/stripe"
)

const (
	srSyncOld = `payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text)`
	srSyncNew = `payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text,jsonb)`
)

func srFee(store, settle, fee int64) int64 { return pslFee(fee, store, settle) }

// srMerchantStatements reads the store's statement list through the real merchant handler as generic maps (the DTO keys are the subject).
func srMerchantStatements(t *testing.T, e *pslEnv, store, token string) map[string]map[string]any {
	t.Helper()
	status, raw, _ := e.call(http.MethodGet, "/v1/admin/stores/"+store+"/settlements", token, nil, "")
	var body struct {
		Statements []map[string]any `json:"statements"`
	}
	if status != 200 || json.Unmarshal(raw, &body) != nil {
		t.Fatalf("merchant list: %d %s", status, raw)
	}
	out := map[string]map[string]any{}
	for _, s := range body.Statements {
		out[s["statement_id"].(string)] = s
	}
	return out
}

// srRun sends one sync observation straight to the SQL definer: the projected lines may be a SUBSET of the manifest (the lines that were
// attributed), the manifest is everything Stripe returned for the window. The ten-argument identity is used when it exists (after 0200);
// before it, the nine-argument one (the manifest cannot be sent).
func (e *pslEnv) srRun(t *testing.T, from, to time.Time, lines, manifest []stripe.BalanceTransaction) string {
	t.Helper()
	ctx := context.Background()
	lj, err := json.Marshal(append([]stripe.BalanceTransaction{}, lines...))
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		ID       string `json:"id"`
		Currency string `json:"currency"`
		Net      int64  `json:"net"`
	}
	m := []entry{}
	for _, x := range manifest {
		m = append(m, entry{x.ID, x.Currency, x.Net})
	}
	mj, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var has10 bool
	if err := e.f.owner.QueryRow(ctx, `SELECT to_regprocedure($1) IS NOT NULL`, srSyncNew).Scan(&has10); err != nil {
		t.Fatal(err)
	}
	var out string
	if has10 {
		err = e.f.owner.QueryRow(ctx, `SELECT payments.record_settlement_lines($1::uuid,$2::uuid,$3::uuid,'SANDBOX',$4::jsonb,$5::timestamptz,$6::timestamptz,NULL,$7::text,$8::jsonb)::text`,
			e.op.TenantID, e.op.StoreID, e.op.PrincipalID, string(lj), from.UTC(), to.UTC(), pslTicket, string(mj)).Scan(&out)
	} else {
		err = e.f.owner.QueryRow(ctx, `SELECT payments.record_settlement_lines($1::uuid,$2::uuid,$3::uuid,'SANDBOX',$4::jsonb,$5::timestamptz,$6::timestamptz,NULL,$7::text)::text`,
			e.op.TenantID, e.op.StoreID, e.op.PrincipalID, string(lj), from.UTC(), to.UTC(), pslTicket).Scan(&out)
	}
	if err != nil {
		t.Fatalf("sync run [%s,%s): %v", from.Format("01-02"), to.Format("01-02"), err)
	}
	return out
}

// srCloseProbe closes the period inside an owner-pool transaction that is always rolled back: proof that a close is reachable (coverage passes)
// without leaving a statement or an assigned line behind.
func (e *pslEnv) srCloseProbe(t *testing.T, start time.Time, target string) (net int64, err error) {
	t.Helper()
	ctx := context.Background()
	tx, berr := e.f.owner.Begin(ctx)
	if berr != nil {
		t.Fatal(berr)
	}
	defer tx.Rollback(ctx)
	var out string
	err = tx.QueryRow(ctx, `SELECT payments.close_settlement($1::uuid,$2::uuid,$3::uuid,'SANDBOX',$4::date,'op@test',$5::uuid,$6::text)::text`,
		e.op.TenantID, e.op.StoreID, e.op.PrincipalID, start.Format("2006-01-02"), target, pslTicket).Scan(&out)
	if err != nil {
		return 0, err
	}
	var res struct {
		Statements []struct {
			NetPayableMinor int64 `json:"net_payable_minor"`
		} `json:"statements"`
	}
	if json.Unmarshal([]byte(out), &res) != nil || len(res.Statements) != 1 {
		t.Fatalf("close probe body: %s", out)
	}
	return res.Statements[0].NetPayableMinor, nil
}

func (e *pslEnv) srReturn(statement, ref string, amount int64, at time.Time, ticket string) (time.Time, error) {
	var recorded time.Time
	err := e.f.owner.QueryRow(context.Background(), `SELECT payments.record_settlement_payout_return($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::text,$6::bigint,$7::timestamptz,'op@test',$8::text)`,
		e.op.TenantID, e.op.StoreID, e.op.PrincipalID, statement, ref, amount, at.UTC(), ticket).Scan(&recorded)
	return recorded, err
}

func srWant(t *testing.T, what string, err error, state, token string) {
	t.Helper()
	if err == nil || sqlState(err) != state || (token != "" && !strings.Contains(err.Error(), token)) {
		t.Fatalf("%s: want SQLSTATE %s carrying %q, got %v", what, state, token, err)
	}
}

// TestPlatformSettlementSA1PF16Return: a payout that bounced is recorded once, carried into the next statement exactly once, and the merchant
// DTO shows it without the bank reference.
func TestPlatformSettlementSA1PF16Return(t *testing.T) {
	e := pslNew(t)
	ctx := context.Background()
	w0, w1, w2, w3 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14), pslDay(2026, 9, 21)
	d := func(w time.Time, n int) time.Time { return w.Add(time.Duration(n)*24*time.Hour + 3*time.Hour) }
	settleOf := func(minor int64) int64 { return minor * 2564 / 10000 }
	// setup (must pass on the base too): W1 = w0 with A1's charge, W2 = w1 with A2's charge, W3 = w2 quiet; 14 days of coverage each
	e.sync(t, w0.Add(-7*24*time.Hour), w0)
	e.sync(t, w0, w1, pslCharge("txn_SR16W1", e.oa1.pi, e.oa1.captured, settleOf(e.oa1.captured), 31, d(w0, 1)))
	e.sync(t, w1, w2, pslCharge("txn_SR16W2", e.oa2.pi, e.oa2.captured, settleOf(e.oa2.captured), 33, d(w1, 1)))
	e.sync(t, w2, w3)
	got, err := e.closeWeek(w0, e.storeA)
	if err != nil || len(got) != 1 {
		t.Fatalf("setup close W1: %+v %v", got, err)
	}
	stmt1, net1 := got[0].StatementID, got[0].NetPayableMinor
	if want := e.oa1.captured + srFee(e.oa1.captured, settleOf(e.oa1.captured), 31); net1 != want || net1 <= 0 {
		t.Fatalf("setup W1 net %d, want %d (> 0)", net1, want)
	}
	paidAt := w1.Add(12 * time.Hour) // inside W2, long past
	if _, err := e.reg.SettlementPayout(ctx, e.op, stmt1, "BANK-SR16-0001", net1, paidAt, "op@test", pslTicket); err != nil {
		t.Fatalf("setup payout: %v", err)
	}
	token := e.a.f.tokens["a"]

	// the DTO before any return: both keys present on every statement, nothing carried
	before := srMerchantStatements(t, e, e.storeA, token)[stmt1]
	if v, ok := before["returned_in_minor"]; !ok || v != float64(0) {
		t.Fatalf("RED DTO: returned_in_minor missing or non-zero on the paid statement: %v", before)
	}
	if v, ok := before["payout_returned"]; !ok || v != false {
		t.Fatalf("RED DTO: payout_returned missing or true before any return: %v", before)
	}
	if _, ok := before["payout_returned_at"]; ok {
		t.Fatalf("payout_returned_at present before any return: %v", before)
	}

	returnedAt := paidAt.Add(24 * time.Hour) // Sep 8 12:00, before the end of W2
	_, err = e.srReturn(stmt1, "BANKRET-0001", net1-100, returnedAt, "OPS-2026-0001")
	srWant(t, "amount mismatch", err, "PT409", "return_amount_mismatch")
	_, err = e.srReturn(stmt1, "BANKRET-0001", net1, paidAt.Add(-time.Second), "OPS-2026-0001")
	srWant(t, "returned before paid", err, "22023", "")
	at, err := e.srReturn(stmt1, "BANKRET-0001", net1, returnedAt, "OPS-2026-0001")
	if err != nil {
		t.Fatalf("record return: %v", err)
	}
	if again, err := e.srReturn(stmt1, "BANKRET-0001", net1, returnedAt, "OPS-2026-0001"); err != nil || !again.Equal(at) {
		t.Fatalf("replay: %v %v (want %v)", again, err, at)
	}
	_, err = e.srReturn(stmt1, "BANKRET-0002", net1, returnedAt, "OPS-2026-0001")
	srWant(t, "different ref", err, "PT409", "return_already_recorded")
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_payout_returns WHERE statement_id=$1`, stmt1); n != 1 {
		t.Fatalf("%d return rows, want 1", n)
	}
	var audit string
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.payout_return'`); n != 1 {
		t.Fatalf("%d return audit rows, want 1 (a replay writes none)", n)
	}
	if err := e.f.owner.QueryRow(ctx, `SELECT details::text FROM ops.audit_events WHERE action='stripe.settlement.payout_return'`).Scan(&audit); err != nil || strings.Contains(audit, "BANKRET") {
		t.Fatalf("return audit details %q (%v): the bank reference must not be in them", audit, err)
	}

	// W2 close carries the return exactly once
	got, err = e.closeWeek(w1, e.storeA)
	if err != nil || len(got) != 1 {
		t.Fatalf("close W2: %+v %v", got, err)
	}
	stmt2, net2 := got[0].StatementID, got[0].NetPayableMinor
	own2 := e.oa2.captured + srFee(e.oa2.captured, settleOf(e.oa2.captured), 33)
	if net2 != own2+net1 {
		t.Fatalf("W2 net %d, want %d (own %d + returned %d)", net2, own2+net1, own2, net1)
	}
	var carried string
	if err := e.f.owner.QueryRow(ctx, `SELECT coalesce(carried_into_statement_id::text,'') FROM payments.settlement_payout_returns WHERE statement_id=$1`, stmt1).Scan(&carried); err != nil || carried != stmt2 {
		t.Fatalf("carried_into_statement_id %q (%v), want %s", carried, err, stmt2)
	}
	if again, err := e.closeWeek(w1, e.storeA); err != nil || len(again) != 1 || !again[0].Replayed || again[0].NetPayableMinor != net2 {
		t.Fatalf("W2 replay: %+v %v", again, err)
	}
	got, err = e.closeWeek(w2, e.storeA) // W3: nothing left to carry
	if err != nil || len(got) != 1 || got[0].NetPayableMinor != 0 {
		t.Fatalf("W3: %+v %v (the return was carried into W2 already; W3 net must be 0)", got, err)
	}
	var retIn int64
	if err := e.f.owner.QueryRow(ctx, `SELECT returned_in_minor FROM payments.settlement_statements WHERE id=$1`, got[0].StatementID).Scan(&retIn); err != nil || retIn != 0 {
		t.Fatalf("W3 returned_in_minor = %d (%v), want 0", retIn, err)
	}

	// merchant DTO after the return: W1 shows it (with the time), W2 shows the carried amount, no bank reference anywhere
	status, raw, _ := e.call(http.MethodGet, "/v1/admin/stores/"+e.storeA+"/settlements", token, nil, "")
	list := srMerchantStatements(t, e, e.storeA, token)
	if s := list[stmt1]; s["payout_returned"] != true || s["payout_returned_at"] == nil {
		t.Fatalf("W1 DTO after the return: %v", s)
	}
	if s := list[stmt2]; s["returned_in_minor"] != float64(net1) || s["payout_returned"] != false {
		t.Fatalf("W2 DTO: %v (want returned_in_minor %d)", s, net1)
	}
	for _, bad := range []string{"BANKRET", "return_ref", "OPS-2026", "ticket"} {
		if status != 200 || strings.Contains(string(raw), bad) {
			t.Fatalf("merchant body leaks %q: %d %s", bad, status, raw)
		}
	}

	// the return row is append-only (owner-pool fixture: even the table owner is refused)
	_, err = e.f.owner.Exec(ctx, `UPDATE payments.settlement_payout_returns SET returned_minor=returned_minor+100 WHERE statement_id=$1`, stmt1)
	srWant(t, "UPDATE of returned_minor", err, "PT409", "settlement_return_immutable")
	_, err = e.f.owner.Exec(ctx, `DELETE FROM payments.settlement_payout_returns WHERE statement_id=$1`, stmt1)
	srWant(t, "DELETE of the return", err, "PT409", "settlement_return_immutable")
}

// TestPlatformSettlementSA1PF17Reconciliation: the sync run stores its comparison, close refuses a window whose latest observation is dirty
// until a clean resync supersedes it, and the nine-argument entry point is gone.
func TestPlatformSettlementSA1PF17Reconciliation(t *testing.T) {
	t.Run("196_item_completeness", srItemCompleteness)
	t.Run("197_failed_final_chunk", srIncompleteChunks)
	e := pslNew(t)
	ctx := context.Background()
	w0, w1, w2 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14)
	d := func(w time.Time, n int) time.Time { return w.Add(time.Duration(n)*24*time.Hour + 3*time.Hour) }
	settleOf := func(minor int64) int64 { return minor * 2564 / 10000 }
	// A: a charge of store A; B: its refund (the refund fact is SUCCEEDED); C: a Stripe fee row with no fee field (unattributed unsupported_type)
	txA := pslCharge("txn_SR17A", e.oa1.pi, e.oa1.captured, settleOf(e.oa1.captured), 31, d(w1, 1))
	txB := pslRefund("txn_SR17B", e.refundA1, 800, settleOf(800), 0, d(w1, 2))
	txC := pslRaw("txn_SR17C", "stripe_fee", "fee", -500, nil, nil, d(w1, 3))
	wantNet := e.oa1.captured - 800 + srFee(e.oa1.captured, settleOf(e.oa1.captured), 31)

	e.srRun(t, w0, w1, nil, nil)                                                                     // U: the clean complement, an empty window
	e.srRun(t, w1, w2, []stripe.BalanceTransaction{txA, txC}, []stripe.BalanceTransaction{txA, txC}) // r0: clean, B not yet visible in Stripe's list
	// a: coverage is complete and clean, so a close is reachable (probe only; nothing is kept)
	net, err := e.srCloseProbe(t, w1, e.storeA)
	if err != nil || net != e.oa1.captured+srFee(e.oa1.captured, settleOf(e.oa1.captured), 31) {
		t.Fatalf("a: clean coverage must let the close probe succeed with net %d: %d %v", e.oa1.captured+srFee(e.oa1.captured, settleOf(e.oa1.captured), 31), net, err)
	}
	// b: the later snapshot reveals B but the projected lines drop it: recorded (A+C) differs from Stripe (A+B+C)
	out := e.srRun(t, w1, w2, []stripe.BalanceTransaction{txA, txC}, []stripe.BalanceTransaction{txA, txB, txC})
	// c: the latest observation of the window is dirty, so close refuses (before any statement or assignment)
	_, err = e.closeWeek(w1, e.storeA)
	if err == nil || !strings.Contains(err.Error(), "sync_unreconciled") {
		t.Fatalf("RED c: close must refuse sync_unreconciled while the latest observation differs from Stripe, got %v", err)
	}
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_statements WHERE environment='SANDBOX' AND period_start=$1`, w1.Format("2006-01-02")); n != 0 {
		t.Fatalf("c: %d statements were written by a refused close", n)
	}
	if !strings.Contains(out, "reconciliation") {
		t.Fatalf("b: the sync result carries no reconciliation: %s", out)
	}
	// d: a clean resync of the whole window supersedes the dirty one
	e.srRun(t, w1, w2, []stripe.BalanceTransaction{txA, txB, txC}, []stripe.BalanceTransaction{txA, txB, txC})
	// e: the close now succeeds and B is assigned exactly once
	got, err := e.closeWeek(w1, e.storeA)
	if err != nil || len(got) != 1 || got[0].NetPayableMinor != wantNet || got[0].LineCount != 2 {
		t.Fatalf("e: close after the clean repair: %+v %v (want net %d, 2 lines)", got, err, wantNet)
	}

	// f: the nine-argument entry point is retired, the ten-argument one refuses a window without a manifest and writes nothing
	var oldID *string
	if err := e.f.owner.QueryRow(ctx, `SELECT to_regprocedure($1)::text`, srSyncOld).Scan(&oldID); err != nil || oldID != nil {
		t.Fatalf("RED f: the old nine-argument identity still exists: %v %v", oldID, err)
	}
	count := func() int {
		return countRows(t, e.f.owner, `SELECT (SELECT count(*) FROM payments.settlement_lines)+(SELECT count(*) FROM payments.settlement_unattributed)
			+(SELECT count(*) FROM payments.settlement_sync_runs)+(SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.sync')`)
	}
	before := count()
	var sink string
	err = e.f.owner.QueryRow(ctx, `SELECT payments.record_settlement_lines($1::uuid,$2::uuid,$3::uuid,'SANDBOX','[]'::jsonb,$4::timestamptz,$5::timestamptz,NULL,NULL,NULL)::text`,
		e.op.TenantID, e.op.StoreID, e.op.PrincipalID, w0.UTC(), w1.UTC()).Scan(&sink)
	srWant(t, "f: window without a manifest", err, "22023", "")
	if after := count(); after != before {
		t.Fatalf("f: the refused call wrote rows (%d -> %d)", before, after)
	}
}

// TestPlatformSettlementSA1PF18Payables: the owner's "who do I pay" list holds positive unpaid statements only, oldest first.
func TestPlatformSettlementSA1PF18Payables(t *testing.T) {
	e := pslNew(t)
	ctx := context.Background()
	w0, w1, w2 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14)
	d := func(w time.Time, n int) time.Time { return w.Add(time.Duration(n)*24*time.Hour + 3*time.Hour) }
	settleOf := func(minor int64) int64 { return minor * 2564 / 10000 }
	e.sync(t, w0, w1)
	e.sync(t, w1, w2,
		pslCharge("txn_SR18A", e.oa1.pi, e.oa1.captured, settleOf(e.oa1.captured), 31, d(w1, 1)),
		pslCharge("txn_SR18C", e.oc1.pi, e.oc1.captured, settleOf(e.oc1.captured), 29, d(w1, 1)),
		pslRefund("txn_SR18B", e.refundB1, 800, settleOf(800), 0, d(w1, 2))) // store B: a refund with no charge: net -800
	closeOne := func(store string) string {
		got, err := e.closeWeek(w1, store)
		if err != nil || len(got) != 1 {
			t.Fatalf("setup close %s: %+v %v", store, got, err)
		}
		return got[0].StatementID
	}
	stmtA := closeOne(e.storeA) // positive, closed first
	stmtC := closeOne(e.storeC) // positive, closed last of the payable ones
	if b := closeOne(e.storeB); b == "" {
		t.Fatal("setup: B")
	}
	if p := closeOne(e.storeP); p == "" { // the platform store: an empty statement, net 0
		t.Fatal("setup: P")
	}
	payables := func(limit int) (ids []string, truncated bool, keys []string, err error) {
		var raw string
		if err = e.f.owner.QueryRow(ctx, `SELECT payments.read_settlement_payables($1::uuid,$2::uuid,$3::uuid,$4::text,$5::integer)::text`,
			e.op.TenantID, e.op.StoreID, e.op.PrincipalID, "SANDBOX", limit).Scan(&raw); err != nil {
			return
		}
		var res struct {
			Payables  []map[string]any `json:"payables"`
			Truncated bool             `json:"truncated"`
		}
		if err = json.Unmarshal([]byte(raw), &res); err != nil {
			return
		}
		for _, p := range res.Payables {
			ids = append(ids, p["statement_id"].(string))
		}
		if len(res.Payables) > 0 {
			for k := range res.Payables[0] {
				keys = append(keys, k)
			}
		}
		return ids, res.Truncated, keys, nil
	}
	ids, trunc, keys, err := payables(50)
	if err != nil || len(ids) != 2 || ids[0] != stmtA || ids[1] != stmtC || trunc {
		t.Fatalf("a: payables %v truncated=%v (%v), want [A C] in closed_at order", ids, trunc, err)
	}
	lcSameSet(t, "payable entry keys", keys, []string{"statement_id", "tenant_id", "store_id", "period_start", "currency", "net_payable_minor", "closed_at"})
	if ids, trunc, _, err = payables(1); err != nil || len(ids) != 1 || ids[0] != stmtA || !trunc {
		t.Fatalf("b: limit 1 -> %v truncated=%v (%v), want [A] truncated", ids, trunc, err)
	}
	var netA int64
	if err := e.f.owner.QueryRow(ctx, `SELECT net_payable_minor FROM payments.settlement_statements WHERE id=$1`, stmtA).Scan(&netA); err != nil {
		t.Fatal(err)
	}
	if _, err := e.reg.SettlementPayout(ctx, e.op, stmtA, "BANK-SR18-0001", netA, time.Now().Add(-time.Hour).Truncate(time.Second), "op@test", pslTicket); err != nil {
		t.Fatalf("setup payout: %v", err)
	}
	if ids, _, _, err = payables(50); err != nil || len(ids) != 1 || ids[0] != stmtC {
		t.Fatalf("c: after the payout of A -> %v (%v), want [C]", ids, err)
	}
	for _, limit := range []int{0, 201} {
		_, _, _, err = payables(limit)
		srWant(t, "d: limit bound", err, "22023", "")
	}
	var sink string
	err = e.f.owner.QueryRow(ctx, `SELECT payments.read_settlement_payables($1::uuid,$2::uuid,$3::uuid,'PROD',50)::text`, e.op.TenantID, e.op.StoreID, e.op.PrincipalID).Scan(&sink)
	srWant(t, "d: environment", err, "22023", "")
	var runtimeCan bool
	if err := e.f.owner.QueryRow(ctx, `SELECT has_function_privilege('commerce_runtime','payments.read_settlement_payables(uuid,uuid,uuid,text,integer)','EXECUTE')`).Scan(&runtimeCan); err != nil || runtimeCan {
		t.Fatalf("e: commerce_runtime may execute the payables definer: %v %v", runtimeCan, err)
	}
	var live string
	if err := e.f.owner.QueryRow(ctx, `SELECT payments.read_settlement_payables($1::uuid,$2::uuid,$3::uuid,'LIVE',50)::text`, e.op.TenantID, e.op.StoreID, e.op.PrincipalID).Scan(&live); err != nil || !strings.Contains(live, `"payables": []`) && !strings.Contains(live, `"payables":[]`) {
		t.Fatalf("f: LIVE read with only SANDBOX statements: %q %v", live, err)
	}
}

// srNoCloseWrites checks all close effects, including the audit, after a refused call.
func srNoCloseWrites(t *testing.T, e *pslEnv) {
	t.Helper()
	for _, q := range []string{
		`SELECT count(*) FROM payments.settlement_statements`,
		`SELECT count(*) FROM payments.settlement_lines WHERE statement_id IS NOT NULL`,
		`SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.close'`,
	} {
		if n := countRows(t, e.f.owner, q); n != 0 {
			t.Fatalf("refused close wrote %d rows: %s", n, q)
		}
	}
}

// srItemCompleteness keeps aggregate net equal on purpose. Count alone cannot catch equal-size ID or per-item changes.
// Raw unsupported Stripe fee rows are valid stored history, as in PF17 above; the actual payable is a fact-backed charge.
func srItemCompleteness(t *testing.T) {
	for _, name := range []string{"offsetting_missing_pair", "equal_count_replaced_id", "offsetting_item_nets", "offsetting_currency_changes"} {
		t.Run(name, func(t *testing.T) {
			e := pslNew(t)
			w0, w1, w2 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14)
			at := w1.Add(24 * time.Hour)
			charge := pslCharge("txn_ICcharge", e.oa1.pi, e.oa1.captured, e.oa1.captured*2564/10000, 31, at)
			a := pslRaw("txn_ICplus", "stripe_fee", "fee", 100, nil, nil, at)
			b := pslRaw("txn_ICminus", "stripe_fee", "fee", -100, nil, nil, at)
			complete := []stripe.BalanceTransaction{charge, a, b}
			lines := append([]stripe.BalanceTransaction{}, complete...)
			manifest := append([]stripe.BalanceTransaction{}, complete...)
			switch name {
			case "offsetting_missing_pair":
				lines = lines[:1]
			case "equal_count_replaced_id":
				manifest[1].ID = "txn_ICreplacement"
			case "offsetting_item_nets":
				manifest[1].Net++
				manifest[2].Net--
			case "offsetting_currency_changes":
				manifest[1].Currency, manifest[2].Currency = "USD", "USD"
			}
			e.srRun(t, w0, w1, nil, nil)
			e.srRun(t, w1, w2, lines, lines) // old clean observation must not win over the newer incomplete one
			if _, err := e.srCloseProbe(t, w1, e.storeA); err != nil {
				t.Fatalf("clean control before counterexample: %v", err)
			}
			e.srRun(t, w1, w2, lines, manifest)
			if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_sync_reconciliations WHERE difference_minor<>0`); n != 0 {
				t.Fatalf("counterexample must have matching aggregate nets, got %d differences", n)
			}
			_, err := e.closeWeek(w1, e.storeA)
			if err == nil || !strings.Contains(err.Error(), "sync_unreconciled") {
				t.Fatalf("RED #196 %s: equal totals must not hide item differences; want sync_unreconciled, got %v", name, err)
			}
			srNoCloseWrites(t, e)
			// Correct the projection/manifest, then replay it: complete identical items restore coverage without duplicates.
			e.srRun(t, w1, w2, complete, complete)
			e.srRun(t, w1, w2, complete, complete)
			got, err := e.closeWeek(w1, e.storeA)
			if err != nil || len(got) != 1 || got[0].LineCount != 1 {
				t.Fatalf("complete replay must close exactly one payable line: %+v %v", got, err)
			}
		})
	}
}

// srIncompleteChunks uses the real Registrar and independently committed 500+1 SQL calls. A test-only trigger aborts the last refund,
// after the first 500 rows committed. The failed final transaction releases the environment lock, reproducing a stopped worker.
// Fixture DDL is confined to pslNew's disposable database; it never changes production functions or ledger data directly.
func srIncompleteChunks(t *testing.T) {
	e := pslNew(t)
	ctx := context.Background()
	w0, w1, w2 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14)
	at := w1.Add(24 * time.Hour)
	a := pslCharge("txn_CHold", e.oa1.pi, e.oa1.captured, e.oa1.captured*2564/10000, 31, at)
	b := pslCharge("txn_CHnew", e.oa2.pi, e.oa2.captured, e.oa2.captured*2564/10000, 31, at)
	refund := pslRefund("txn_CHlast", e.refundA1, 800, 800*2564/10000, 0, at)
	e.srRun(t, w0, w1, nil, nil)
	e.srRun(t, w1, w2, []stripe.BalanceTransaction{a}, []stripe.BalanceTransaction{a})
	if _, err := e.srCloseProbe(t, w1, e.storeA); err != nil {
		t.Fatalf("old clean coverage control: %v", err)
	}
	txns := []stripe.BalanceTransaction{a, b}
	for i := 0; i < 498; i++ {
		txns = append(txns, pslRaw(fmt.Sprintf("txn_CHzero%d", i), "stripe_fee", "fee", 0, nil, nil, at))
	}
	txns = append(txns, refund)
	mustExec(t, e.f.owner, `CREATE FUNCTION public.sr197_abort_tail() RETURNS trigger LANGUAGE plpgsql AS $$
	 BEGIN IF NEW.balance_txn_id='txn_CHlast' THEN PERFORM pg_advisory_xact_lock(196197);
	 RAISE EXCEPTION 'sr197_tail_failure' USING ERRCODE='22023'; END IF; RETURN NEW; END $$`)
	mustExec(t, e.f.owner, `CREATE TRIGGER sr197_abort_tail BEFORE INSERT ON payments.settlement_lines FOR EACH ROW EXECUTE FUNCTION public.sr197_abort_tail()`)
	barrier, err := e.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer barrier.Rollback(ctx)
	if _, err := barrier.Exec(ctx, `SELECT pg_advisory_xact_lock(196197)`); err != nil {
		t.Fatal(err)
	}
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	syncDone := make(chan error, 1)
	go func() {
		_, err := e.reg.RecordSettlementLines(syncCtx, e.op, e.plat.connection, pslTicket, txns, w1, w2)
		syncDone <- err
	}()
	// This is an actual two-chunk barrier: the first SQL call committed and the last INSERT is waiting on our fixture lock.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if n := countRows(t, e.f.owner, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database()
		 AND wait_event='advisory' AND query LIKE '%payments.record_settlement_lines%'`); n > 0 {
			break
		}
		select {
		case err := <-syncDone:
			t.Fatalf("sync exited before reaching the final-chunk barrier: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("final-chunk barrier was not reached")
		}
		time.Sleep(20 * time.Millisecond)
	}
	closeDone := make(chan error, 1)
	go func() { _, err := e.closeWeek(w1, e.storeA); closeDone <- err }()
	// Close may wait for the sync's environment lock or refuse immediately; it must never succeed with a partial run.
	closeFinished := false
	select {
	case err := <-closeDone:
		closeFinished = true
		if err == nil || !strings.Contains(err.Error(), "sync_unreconciled") {
			t.Fatalf("RED #197: close at the chunk barrier must wait or refuse, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
	}
	if err := barrier.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-syncDone:
		if err == nil {
			t.Fatal("tail failure fixture did not abort the 501st item")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("sync did not stop after the tail failure")
	}
	if !closeFinished {
		select {
		case err := <-closeDone:
			if err == nil || !strings.Contains(err.Error(), "sync_unreconciled") {
				t.Fatalf("RED #197: concurrent close after tail failure must refuse, got %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("close did not finish after failed sync released its lock")
		}
	}
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_lines WHERE balance_txn_id IN ('txn_CHold','txn_CHnew')`); n != 2 {
		t.Fatalf("first chunk must have committed both charges, got %d", n)
	}
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_unattributed WHERE balance_txn_id LIKE 'txn_CHzero%'`); n != 498 {
		t.Fatalf("first chunk must have committed all 498 zero-net rows, got %d", n)
	}
	_, err = e.closeWeek(w1, e.storeA)
	if err == nil || !strings.Contains(err.Error(), "sync_unreconciled") {
		t.Fatalf("RED #197: aborted final chunk must invalidate old clean coverage, got %v", err)
	}
	srNoCloseWrites(t, e)
	mustExec(t, e.f.owner, `DROP TRIGGER sr197_abort_tail ON payments.settlement_lines`)
	mustExec(t, e.f.owner, `DROP FUNCTION public.sr197_abort_tail()`)
	// Complete retry and a duplicate-only replay must both finish clean; no permanent dirty flag or duplicate accounting.
	for i := 0; i < 2; i++ {
		if _, err := e.reg.RecordSettlementLines(ctx, e.op, e.plat.connection, pslTicket, txns, w1, w2); err != nil {
			t.Fatalf("complete retry %d: %v", i, err)
		}
	}
	got, err := e.closeWeek(w1, e.storeA)
	if err != nil || len(got) != 1 || got[0].LineCount != 3 {
		t.Fatalf("complete retry must assign two charges and one refund once: %+v %v", got, err)
	}
	again, err := e.closeWeek(w1, e.storeA)
	if err != nil || len(again) != 1 || again[0].NetPayableMinor != got[0].NetPayableMinor || again[0].LineCount != 3 {
		t.Fatalf("close replay changed accounting: %+v %v", again, err)
	}
}
