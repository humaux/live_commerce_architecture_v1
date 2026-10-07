package foundation_test

// W4-U1 (docs/delivery/units/w4-u1-payment-activation-ui.md): the real-ledger fixture shared by the card-payments / settlements
// browser gate (browser_card_payments_test.go) and the wire-shape check below. Tier: MOCK balance transactions (never Stripe) + REAL_PG:
// merchant A of pslNew pays through the real capture path, then three weekly statements are produced ONLY through the operator paths
// (stripeadmin sync -> close -> payout record). Nothing is inserted into the settlement tables directly.
//
// Why a wire test: the admin parsers (apps/admin/lib/card-payments-settlements-model.ts) are strict. TestW4U1WireShapes asserts the real
// HTTP bodies carry every sign case the parsers must accept (a fee is a cost <= 0, a dispute reversal makes dispute_minor negative, a
// carried-in debt, an unpaid net <= 0) and, with LC_W4U1_WIRE_OUT=<dir>, writes the bodies verbatim so tests/admin/fixtures/card-payments-wire
// can be regenerated and re-parsed by tests/admin/card-payments-wire.test.ts (Go PG -> JSON -> TS parser, E3).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"livecommerce/internal/payments/settlement"
)

// w4u1Statements names the three statements of merchant A: Paid (net > 0, payout recorded), Carried (net <= 0: refunds and a dispute
// outran the week, never paid) and Pending (net > 0 after the carry, dispute reversal makes dispute_minor negative, not yet paid).
type w4u1Statements struct {
	Paid, Carried, Pending string
	PaidNet                int64
	PayoutRef              string
}

const w4u1PayoutRef = "BANK-REF-W4U1-0831"

// w4u1SeedStatements closes weeks 08-31, 09-07 and 09-14 (Asia/Taipei) of store A and records the payout of the first.
func w4u1SeedStatements(t *testing.T, e *pslEnv) w4u1Statements {
	t.Helper()
	w0, w1, w2, w3 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14), pslDay(2026, 9, 21)
	d := func(w time.Time, n int) time.Time { return w.Add(time.Duration(n)*24*time.Hour + 3*time.Hour) }
	settleOf := func(minor int64) int64 { return minor * 2564 / 10000 }
	capOf := func(o rfxOrder) int64 { return o.captured }
	// coverage: close needs the union of sync windows to span [period_start - 7d, period_end)
	e.sync(t, w0.Add(-7*24*time.Hour), w0)
	e.sync(t, w0, w1, pslCharge("txn_W4U1ChargeA2", e.oa2.pi, capOf(e.oa2), settleOf(capOf(e.oa2)), 33, d(w0, 1)))
	e.sync(t, w1, w2,
		pslCharge("txn_W4U1ChargeA1", e.oa1.pi, capOf(e.oa1), settleOf(capOf(e.oa1)), 31, d(w1, 1)),
		pslRefund("txn_W4U1RefundA1", e.refundA1, 800, settleOf(800), 0, d(w1, 2)),
		pslDispute("txn_W4U1DisputeA2", "dp_W4U1A2", e.oa2.pi, capOf(e.oa2), settleOf(capOf(e.oa2)), 1500, false, d(w1, 3)))
	e.sync(t, w2, w3,
		pslCharge("txn_W4U1ChargeA3", e.oa3.pi, capOf(e.oa3), settleOf(capOf(e.oa3)), 35, d(w2, 1)),
		pslDispute("txn_W4U1ReversalA2", "dp_W4U1A2", e.oa2.pi, capOf(e.oa2), settleOf(capOf(e.oa2)), 1500, true, d(w2, 2)))
	closeOne := func(week time.Time) (string, int64) {
		got, err := e.closeWeek(week, e.storeA)
		if err != nil || len(got) != 1 {
			t.Fatalf("close %s: %+v %v", week.Format("2006-01-02"), got, err)
		}
		return got[0].StatementID, got[0].NetPayableMinor
	}
	var out w4u1Statements
	out.Paid, out.PaidNet = closeOne(w0)
	out.Carried, _ = closeOne(w1)
	out.Pending, _ = closeOne(w2)
	if out.PaidNet <= 0 {
		t.Fatalf("fixture: the first statement must be payable, net=%d", out.PaidNet)
	}
	paidAt := time.Now().Add(-time.Hour).Truncate(time.Second)
	if _, err := e.reg.SettlementPayout(t.Context(), e.op, out.Paid, w4u1PayoutRef, out.PaidNet, paidAt, "op@test", pslTicket); err != nil {
		t.Fatalf("payout record: %v", err)
	}
	out.PayoutRef = w4u1PayoutRef
	return out
}

// w4u1Get calls the real merchant HTTP handler (card routes + settlement routes) with the store creator's bearer.
func w4u1Get(t *testing.T, h http.Handler, token, path string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

func TestW4U1WireShapes(t *testing.T) {
	e := pslNew(t)
	st := w4u1SeedStatements(t, e)
	token, store := e.a.f.tokens["a"], e.storeA
	base := "/v1/admin/stores/" + store

	status, raw := w4u1Get(t, e.card, token, base+"/settlements")
	var list settlement.List
	if status != http.StatusOK || json.Unmarshal(raw, &list) != nil || len(list.Statements) != 3 {
		t.Fatalf("settlement list: %d %s", status, raw)
	}
	byID := map[string]settlement.Statement{}
	for _, s := range list.Statements {
		byID[s.StatementID] = s
		if len(s.Lines) != 0 {
			t.Fatalf("the list must not carry lines: %+v", s)
		}
	}
	details := map[string][]byte{}
	for id := range byID {
		status, raw := w4u1Get(t, e.card, token, base+"/settlements/"+id)
		var detail settlement.Detail
		if status != http.StatusOK || json.Unmarshal(raw, &detail) != nil || len(detail.Statement.Lines) != detail.Statement.LineCount {
			t.Fatalf("detail %s: %d %s", id, status, raw)
		}
		details[id] = raw
	}
	paid, carried, pending := byID[st.Paid], byID[st.Carried], byID[st.Pending]
	// the sign cases the admin parsers must accept
	if !paid.Paid || paid.PayoutRef == nil || *paid.PayoutRef != st.PayoutRef || paid.PayoutMinor == nil || *paid.PayoutMinor != paid.NetPayableMinor || paid.StripeFeeMinor >= 0 {
		t.Errorf("paid statement: %+v", paid)
	}
	if carried.Paid || carried.NetPayableMinor > 0 || carried.RefundedMinor <= 0 || carried.DisputeMinor <= 0 || carried.StripeFeeMinor >= 0 {
		t.Errorf("carried statement (net <= 0, refund and dispute positive, fee a cost): %+v", carried)
	}
	if pending.Paid || pending.NetPayableMinor <= 0 || pending.CarriedInMinor >= 0 || pending.DisputeMinor >= 0 {
		t.Errorf("pending statement (positive net after a carried debt, dispute reversal negative): %+v", pending)
	}
	if dir := os.Getenv("LC_W4U1_WIRE_OUT"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		write := func(name string, body []byte) {
			if err := os.WriteFile(filepath.Join(dir, name), append(body, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write("settlements-list.json", raw)
		write("settlement-paid.json", details[st.Paid])
		write("settlement-carried.json", details[st.Carried])
		write("settlement-pending.json", details[st.Pending])
		for name, path := range map[string]string{"card-enabled.json": base + "/payments/card"} {
			status, body := w4u1Get(t, e.card, token, path)
			if status != http.StatusOK {
				t.Fatalf("%s: %d %s", path, status, body)
			}
			write(name, body)
		}
		// a store with no statement answers an empty list; the card read of a never-enabled, allowlisted store is the NONE shape
		status, body := w4u1Get(t, e.card, e.b.f.tokens["a"], "/v1/admin/stores/"+e.storeB+"/settlements")
		if status != http.StatusOK {
			t.Fatalf("empty list: %d %s", status, body)
		}
		write("settlements-empty.json", body)
	}
}
