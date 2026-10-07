package foundation_test

// S2-OPEN-1 (PF15): append-only resolution of payments.settlement_unattributed rows with reason unmapped_source, so a
// close blocked by a charge the system never created can proceed without a red-line DELETE
// (contracts/stripe-platform-account-v1.md §6.6, migrations/0163_settlement_resolve.sql).
// Tier: MOCK (fixture balance-transaction list, never Stripe) + REAL_PG: the blocking row is seeded through the product
// sync path (the PF10 dispute_other vector), close runs through stripeadmin.Registrar (what cmd/stripe-admin calls),
// and the resolution through the 0163 SECURITY DEFINER entry point. No owner-pool fixture writes: nothing is deleted or
// edited anywhere in this test — that is exactly the property under test.

import (
	"context"
	"testing"
	"time"
)

func TestPlatformSettlementPF15Resolve(t *testing.T) {
	e := pslNew(t)
	ctx := context.Background()
	w0, w1, w2 := pslDay(2026, 8, 31), pslDay(2026, 9, 7), pslDay(2026, 9, 14)
	d := func(w time.Time, n int) time.Time { return w.Add(time.Duration(n)*24*time.Hour + 3*time.Hour) }
	settleOf := func(minor int64) int64 { return minor * 2564 / 10000 } // the fixture rate 0.2564

	// Week 1 the way it happens in production: one attributed charge for A plus a dispute-shaped adjustment Stripe did
	// not classify as a known pair (PF10 vector) -> an unmapped_source row that blocks the whole environment's close.
	e.sync(t, w0, w1) // close of w1 requires sync coverage of [w0, w2)
	odd := pslDispute("txn_PF15Odd", "dp_pf15", e.oa2.pi, 100, 26, 0, false, d(w1, 5))
	odd.ReportingCategory = pslS("dispute_other")
	e.sync(t, w1, w2, pslCharge("txn_PF15A1", e.oa1.pi, e.oa1.captured, settleOf(e.oa1.captured), 31, d(w1, 1)), odd)
	if got := e.unattributed(t, "txn_PF15Odd"); got != "unmapped_source" {
		t.Fatalf("seed row reason = %s, want unmapped_source", got)
	}

	// Close refuses the whole environment while the row is unresolved (0150, §6.2).
	_, err := e.closeWeek(w1, "")
	pslWantRefused(t, "unmapped source blocks close", err, "settlement_unattributed")

	// The resolution path (0163 §6.6). RED state: the entry point does not exist yet (42883), so the refusal above has
	// no sanctioned remedy; GREEN: the resolve clears the close with the unattributed row untouched.
	var resolved string
	if err := e.f.owner.QueryRow(ctx,
		`SELECT payments.record_settlement_resolution($1::uuid,$2::uuid,$3::uuid,'SANDBOX','txn_PF15Odd','not_store_revenue','op@test',$4,'Stripe-side dispute adjustment, not a store sale.',NULL,NULL)::text`,
		e.op.TenantID, e.op.StoreID, e.op.PrincipalID, pslTicket).Scan(&resolved); err != nil {
		t.Fatalf("record_settlement_resolution: %v (sqlstate %s)", err, sqlState(err))
	}
	t.Logf("resolution row: %s", resolved)

	// Close now succeeds, and the totals are the charge-only week: a resolution is a note, never a line (§6.6 v1).
	got, err := e.closeWeek(w1, "")
	if err != nil {
		t.Fatalf("close after resolve: %v", err)
	}
	if len(got) != 1 || got[0].StoreID != e.storeA || got[0].LineCount != 1 {
		t.Fatalf("close after resolve: %+v", got)
	}
	if want := e.oa1.captured + pslFee(31, e.oa1.captured, settleOf(e.oa1.captured)); got[0].NetPayableMinor != want {
		t.Fatalf("net = %d, want %d (resolution moved no money)", got[0].NetPayableMinor, want)
	}
	// Append-only: the unattributed row itself is unchanged, and exactly one resolution row exists.
	if reason := e.unattributed(t, "txn_PF15Odd"); reason != "unmapped_source" {
		t.Fatalf("the unattributed row was edited: %s", reason)
	}
	if n := countRows(t, e.f.owner, `SELECT count(*) FROM payments.settlement_unattributed_resolutions`); n != 1 {
		t.Fatalf("%d resolution rows, want 1", n)
	}
}
