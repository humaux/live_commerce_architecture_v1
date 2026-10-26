// settlement_unit_test.go: MOCK-tier tests of the settlement registrar steps (contracts/stripe-platform-account-v1.md §6, PF10 CLI half)
// against the recording DB fake and a fake provider behind the package seam. Purpose: a failed or oversized Stripe read writes nothing,
// the 500-line chunking carries the window on the LAST chunk only, bad windows never reach Stripe or SQL, and a refusal keeps only our
// own coded token (never a driver message). Non-goals: the SQL definers (tests/foundation/platform_settlement_test.go), real Stripe.
// Depends on: live_test.go fakes (liveDB, fakeProvider, liveRegistrar, storedReply). Callers: go test ./internal/payments/stripeadmin.

package stripeadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/integrations/psp/stripe"
)

type balanceFake struct {
	fakeProvider
	txns     []stripe.BalanceTransaction
	err      error
	gte, lt  int64
	listed   int
	lastMeta stripe.CallMeta
}

func (b *balanceFake) ListBalanceTransactions(_ context.Context, gte, lt int64) ([]stripe.BalanceTransaction, stripe.CallMeta, error) {
	b.listed++
	b.gte, b.lt = gte, lt
	return b.txns, b.lastMeta, b.err
}

func settlementRegistrar(t *testing.T, db *liveDB, p *balanceFake) *Registrar {
	t.Helper()
	r := newRegistrar(db, ring(t, 1), ring(t, 2))
	r.live = livePair
	r.newProvider = func(cfg stripe.Config) (provider, error) {
		p.cfg = cfg
		return p, nil
	}
	return r
}

func txnN(n int) []stripe.BalanceTransaction {
	out := make([]stripe.BalanceTransaction, n)
	for i := range out {
		out[i] = stripe.BalanceTransaction{ID: "txn_N" + strings.Repeat("x", 3) + string(rune('A'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676)),
			Type: "charge", Amount: 100, Net: 90, Currency: "HKD", Created: 1790000000}
	}
	return out
}

const tick = "TICKET-2026-2002"

var (
	winFrom = time.Date(2026, 9, 14, 0, 0, 0, 0, time.FixedZone("TPE", 8*3600))
	winTo   = winFrom.Add(7 * 24 * time.Hour)
)

func TestSettlementSyncFailedReadWritesNothing(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want error
	}{"uncertain (50+ pages, bad body, transport)": {stripe.ErrUncertain, ErrProvider}, "authentication": {stripe.ErrAuthentication, ErrRejected}} {
		db := &liveDB{}
		p := &balanceFake{err: c.err}
		r := settlementRegistrar(t, db, p)
		db.replies = []any{storedReply(t, r, liveRAK)}
		_, err := r.SettlementSync(context.Background(), scope, conn, 2, winFrom, winTo, tick)
		if !errors.Is(err, c.want) {
			t.Fatalf("%s: %v", name, err)
		}
		if len(db.calls) != 1 || !strings.Contains(db.calls[0].sql, "stripe_registrar_credential") {
			t.Fatalf("%s: SQL calls after a failed read: %d (only the credential read is allowed)", name, len(db.calls))
		}
		if p.listed != 1 || p.gte != winFrom.Unix() || p.lt != winTo.Unix() {
			t.Fatalf("%s: window sent to Stripe %d..%d", name, p.gte, p.lt)
		}
	}
}

func reconReply(inserted, dup, unattr, mismatch int, run string, rows ...string) string {
	return fmt.Sprintf(`{"inserted":%d,"duplicate":%d,"unattributed":%d,"mismatch":%d,"sync_run_id":%q,"reconciliation":[%s]}`,
		inserted, dup, unattr, mismatch, run, strings.Join(rows, ","))
}

func TestSettlementSyncChunksAndCarriesTheWindowAndManifestOnTheLastChunk(t *testing.T) {
	db := &liveDB{}
	txns := txnN(1203)
	p := &balanceFake{txns: txns}
	r := settlementRegistrar(t, db, p)
	db.replies = []any{storedReply(t, r, liveRAK),
		`{"inserted":500,"duplicate":0,"unattributed":0,"mismatch":0}`,
		`{"inserted":498,"duplicate":0,"unattributed":2,"mismatch":1}`,
		reconReply(100, 100, 3, 0, conn, `{"currency":"HKD","stripe_net_minor":108270,"recorded_net_minor":108270,"difference_minor":0,"stripe_txn_count":1203,"recorded_txn_count":1203}`)}
	rep, err := r.SettlementSync(context.Background(), scope, conn, 2, winFrom, winTo, tick)
	if err != nil {
		t.Fatal(err)
	}
	if len(db.calls) != 4 {
		t.Fatalf("SQL calls = %d, want 1 credential + 3 chunks", len(db.calls))
	}
	sizes := []int{500, 500, 203}
	for i, want := range sizes {
		c := db.calls[1+i]
		var got []stripe.BalanceTransaction
		if err := json.Unmarshal([]byte(c.args[4].(string)), &got); err != nil || len(got) != want {
			t.Fatalf("chunk %d: %d lines (%v), want %d", i, len(got), err, want)
		}
		last := i == len(sizes)-1
		// §6.8: ten arguments; only the LAST chunk carries the window pair AND the manifest (intermediate calls have all three NULL)
		if len(c.args) != 10 || (c.args[5] != nil) != last || (c.args[6] != nil) != last || (c.args[9] != nil) != last {
			t.Fatalf("chunk %d: args %v; only the last chunk carries the window and the manifest", i, c.args)
		}
		if c.args[3] != "LIVE" || c.args[7] != conn || c.args[8] != tick {
			t.Fatalf("environment/connection/ticket args = %v %v %v", c.args[3], c.args[7], c.args[8])
		}
		if !strings.Contains(c.sql, "$10::jsonb") {
			t.Fatalf("chunk %d does not pass the manifest argument: %s", i, c.sql)
		}
	}
	if w := db.calls[3].args; !w[5].(time.Time).Equal(winFrom) || !w[6].(time.Time).Equal(winTo) {
		t.Fatalf("window args %v %v", w[5], w[6])
	}
	// the manifest is EVERY balance transaction of the run (not only the last chunk), {id,currency,net} only
	var manifest []map[string]any
	if err := json.Unmarshal([]byte(db.calls[3].args[9].(string)), &manifest); err != nil || len(manifest) != 1203 {
		t.Fatalf("manifest: %d entries (%v), want 1203", len(manifest), err)
	}
	if m := manifest[0]; len(m) != 3 || m["id"] != txns[0].ID || m["currency"] != "HKD" || m["net"] != float64(90) {
		t.Fatalf("manifest entry: %v", m)
	}
	if rep.Fetched != 1203 || rep.Inserted != 1098 || rep.Duplicate != 100 || rep.Unattributed != 5 || rep.Mismatch != 1 || rep.SyncRunID != conn {
		t.Fatalf("report: %+v", rep)
	}
	// the totals are READ BACK from SQL, Go sums nothing: 1203 txns of net 90 = 108270 on both sides
	if rep.StripeNet["HKD"] != 108270 || rep.RecordedNet["HKD"] != 108270 || rep.Difference["HKD"] != 0 || !rep.Reconciled() || len(rep.Reconciliation) != 1 {
		t.Fatalf("reconciliation: %+v", rep)
	}
	// a stored total that differs from Stripe's is printed as a number, not hidden, and the report is not reconciled
	db2 := &liveDB{}
	r2 := settlementRegistrar(t, db2, &balanceFake{txns: txnN(2)})
	db2.replies = []any{storedReply(t, r2, liveRAK), reconReply(1, 0, 0, 0, conn,
		`{"currency":"HKD","stripe_net_minor":180,"recorded_net_minor":80,"difference_minor":-100,"stripe_txn_count":2,"recorded_txn_count":1}`)}
	rep2, err := r2.SettlementSync(context.Background(), scope, conn, 2, winFrom, winTo, tick)
	if err != nil || rep2.Difference["HKD"] != -100 || rep2.RecordedNet["HKD"] != 80 || rep2.StripeNet["HKD"] != 180 || rep2.Reconciled() {
		t.Fatalf("difference: %+v %v", rep2, err)
	}
	// an empty window still records its coverage (a JSON array, never null) with an EMPTY manifest, and writes no reconciliation row
	db3 := &liveDB{}
	r3 := settlementRegistrar(t, db3, &balanceFake{})
	db3.replies = []any{storedReply(t, r3, liveRAK), reconReply(0, 0, 0, 0, conn)}
	rep3, err := r3.SettlementSync(context.Background(), scope, conn, 2, winFrom, winTo, tick)
	if err != nil || len(db3.calls) != 2 || db3.calls[1].args[4].(string) != "[]" || len(db3.calls[1].args) != 10 || db3.calls[1].args[5] == nil ||
		db3.calls[1].args[9] != "[]" || !rep3.Reconciled() || len(rep3.Reconciliation) != 0 {
		t.Fatalf("empty window: %v calls=%d %+v", err, len(db3.calls), rep3)
	}
	// a final reply without a sync run id is not a recorded window: nothing is reported as reconciled
	db4 := &liveDB{}
	r4 := settlementRegistrar(t, db4, &balanceFake{})
	db4.replies = []any{storedReply(t, r4, liveRAK), `{"inserted":0,"duplicate":0,"unattributed":0,"mismatch":0}`}
	if _, err := r4.SettlementSync(context.Background(), scope, conn, 2, winFrom, winTo, tick); !errors.Is(err, ErrDatabase) {
		t.Fatalf("a final reply with no sync_run_id: %v", err)
	}
	// more than the 50 pages the wire can return never reaches SQL
	if _, err := r.RecordSettlementLines(context.Background(), scope, "", "", txnN(5001), winFrom, winTo); !errors.Is(err, ErrConfig) {
		t.Fatalf("5001 transactions: %v", err)
	}
}

func TestSettlementBadArgumentsNeverReachStripeOrSQL(t *testing.T) {
	db := &liveDB{}
	p := &balanceFake{}
	r := settlementRegistrar(t, db, p)
	ctx := context.Background()
	for name, f := range map[string]func() error{
		"window over 8 days": func() error {
			_, e := r.SettlementSync(ctx, scope, conn, 2, winFrom, winFrom.Add(8*24*time.Hour+time.Second), tick)
			return e
		},
		"empty window":    func() error { _, e := r.SettlementSync(ctx, scope, conn, 2, winFrom, winFrom, tick); return e },
		"reversed window": func() error { _, e := r.SettlementSync(ctx, scope, conn, 2, winTo, winFrom, tick); return e },
		"version 0":       func() error { _, e := r.SettlementSync(ctx, scope, conn, 0, winFrom, winTo, tick); return e },
		"bad connection":  func() error { _, e := r.SettlementSync(ctx, scope, "x", 2, winFrom, winTo, tick); return e },
		"record: huge window": func() error {
			_, e := r.RecordSettlementLines(ctx, scope, "", "", nil, winFrom, winFrom.Add(9*24*time.Hour))
			return e
		},
		"record: bad connection": func() error { _, e := r.RecordSettlementLines(ctx, scope, "x", "", nil, winFrom, winTo); return e },
		"close: bad environment": func() error { _, e := r.SettlementClose(ctx, scope, "PROD", "2026-09-14", "op@test", "", ""); return e },
		"close: bad target": func() error {
			_, e := r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", "nope", "")
			return e
		},
		"resolve: bad environment": func() error {
			_, e := r.SettlementResolve(ctx, scope, "PROD", "txn_R1", ResolveNotStoreRevenue, "", "", "note", "op@test", tick)
			return e
		},
		"resolve: malformed target uuid": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveAssignedToStore, "nope", store, "note", "op@test", tick)
			return e
		},
		"resolve: half a target pair": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveAssignedToStore, store, "", "note", "op@test", tick)
			return e
		},
		"payout: bad statement": func() error {
			_, e := r.SettlementPayout(ctx, scope, "nope", "BANK-REF-1", 100, winFrom, "op@test", "")
			return e
		},
		"export: bad statement": func() error { _, e := r.SettlementStatement(ctx, scope, "nope"); return e },
	} {
		if err := f(); !errors.Is(err, ErrConfig) {
			t.Errorf("%s: %v, want ErrConfig", name, err)
		}
	}
	for name, f := range map[string]func() error{
		"close: bad date": func() error {
			_, e := r.SettlementClose(ctx, scope, "SANDBOX", "14/09/2026", "op@test", "", "")
			return e
		},
		"close: bad operator": func() error { _, e := r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "o", "", ""); return e },
		"payout: bad ref": func() error {
			_, e := r.SettlementPayout(ctx, scope, conn, "x y", 100, winFrom, "op@test", "")
			return e
		},
		"payout: zero amount": func() error {
			_, e := r.SettlementPayout(ctx, scope, conn, "BANK-REF-1", 0, winFrom, "op@test", "")
			return e
		},
		"payout: bad ticket": func() error {
			_, e := r.SettlementPayout(ctx, scope, conn, "BANK-REF-1", 100, winFrom, "op@test", "short")
			return e
		},
		"close: bad ticket": func() error {
			_, e := r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", "", "short")
			return e
		},
		"sync: bad ticket": func() error { _, e := r.SettlementSync(ctx, scope, conn, 2, winFrom, winTo, "short"); return e },
		"sync: window ends now": func() error {
			_, e := r.SettlementSync(ctx, scope, conn, 2, time.Now().Add(-24*time.Hour), time.Now(), tick)
			return e
		},
		"sync: window ends in the future": func() error {
			_, e := r.SettlementSync(ctx, scope, conn, 2, time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour), tick)
			return e
		},
		"sync: window ends inside the 15 minute margin": func() error {
			_, e := r.SettlementSync(ctx, scope, conn, 2, time.Now().Add(-24*time.Hour), time.Now().Add(-14*time.Minute), tick)
			return e
		},
		"record: window ends in the future": func() error {
			_, e := r.RecordSettlementLines(ctx, scope, "", "", nil, time.Now().Add(-24*time.Hour), time.Now().Add(time.Hour))
			return e
		},
		"payout: zero paid_at": func() error {
			_, e := r.SettlementPayout(ctx, scope, conn, "BANK-REF-1", 100, time.Time{}, "op@test", "")
			return e
		},
		"resolve: bad txn id": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "bal_R1", ResolveNotStoreRevenue, "", "", "note", "op@test", tick)
			return e
		},
		"resolve: unknown resolution value": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", "write_off", "", "", "note", "op@test", tick)
			return e
		},
		"resolve: assigned_to_store without target": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveAssignedToStore, "", "", "note", "op@test", tick)
			return e
		},
		"resolve: not_store_revenue with target": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveNotStoreRevenue, store, store, "note", "op@test", tick)
			return e
		},
		"resolve: missing ticket": func() error { // the escalation step must stay traceable: the ticket is required
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveNotStoreRevenue, "", "", "note", "op@test", "")
			return e
		},
		"resolve: bad operator": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveNotStoreRevenue, "", "", "note", "o", tick)
			return e
		},
		"resolve: empty note": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveNotStoreRevenue, "", "", "", "op@test", tick)
			return e
		},
		"resolve: note with a newline": func() error { // P2-2: one printable line, the note is printed in CLI output
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveNotStoreRevenue, "", "", "two\nlines", "op@test", tick)
			return e
		},
		"resolve: note with a tab or a C1 control": func() error {
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveNotStoreRevenue, "", "", "tab\there\u0085", "op@test", tick)
			return e
		},
		"resolve: note over 500 runes": func() error { // runes, not bytes (§6.6)
			_, e := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveNotStoreRevenue, "", "", strings.Repeat("漢", 501), "op@test", tick)
			return e
		},
	} {
		if err := f(); !errors.Is(err, ErrRejected) {
			t.Errorf("%s: %v, want ErrRejected", name, err)
		}
	}
	if len(db.calls) != 0 || p.listed != 0 {
		t.Fatalf("bad arguments reached SQL (%d) or Stripe (%d)", len(db.calls), p.listed)
	}
}

func TestSettlementRefusalKeepsOnlyOurCodedToken(t *testing.T) {
	db := &liveDB{}
	r := settlementRegistrar(t, db, &balanceFake{})
	ctx := context.Background()
	pg := func(code, msg string) error {
		return &pgconn.PgError{Code: code, Message: msg, Detail: "row (secret=abc)"}
	}
	db.replies = []any{pg("PT409", "settlement_mismatch"), pg("PT409", "driver said: password=hunter2"), pg("42501", "settlement_mismatch"), pg("XX000", "settlement_mismatch")}
	_, err := r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", "", "")
	if !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "settlement_mismatch") {
		t.Fatalf("coded refusal: %v", err)
	}
	_, err = r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", "", "")
	if !errors.Is(err, ErrRejected) || strings.Contains(err.Error(), "hunter2") || strings.Contains(err.Error(), "password") {
		t.Fatalf("a driver message leaked: %v", err)
	}
	if _, err = r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", "", ""); !errors.Is(err, ErrRejected) || strings.Contains(err.Error(), "settlement_mismatch") {
		t.Fatalf("a non-PT409 state kept a token: %v", err)
	}
	if _, err = r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", "", ""); !errors.Is(err, ErrDatabase) {
		t.Fatalf("unclassified SQL state: %v", err)
	}
}

// P2-1: the settlement_unattributed refusal passes on its blocking balance transaction ids (Stripe ids, no PII) and NOTHING else of the
// DETAIL: an id list that is not exactly 1..20 well-formed ids, or the same list under another token, is dropped.
func TestSettlementUnattributedRefusalCarriesOnlyTheBlockingIDs(t *testing.T) {
	db := &liveDB{}
	r := settlementRegistrar(t, db, &balanceFake{})
	ctx := context.Background()
	ids := func(n int) string {
		out := make([]string, n)
		for i := range out {
			out[i] = "txn_B" + strings.Repeat("x", i)
		}
		return strings.Join(out, ",")
	}
	pg := func(msg, detail string) error { return &pgconn.PgError{Code: "PT409", Message: msg, Detail: detail} }
	for name, tc := range map[string]struct {
		err  error
		want string // the exact error suffix; "" = the token only
	}{
		"one id":                  {pg("settlement_unattributed", "txn_A1"), "settlement_unattributed: txn_A1"},
		"two ids":                 {pg("settlement_unattributed", "txn_A1,txn_B2"), "settlement_unattributed: txn_A1,txn_B2"},
		"twenty ids":              {pg("settlement_unattributed", ids(20)), "settlement_unattributed: " + ids(20)},
		"twenty-one ids":          {pg("settlement_unattributed", ids(21)), ""},
		"free text":               {pg("settlement_unattributed", "row (secret=abc)"), ""},
		"an id and a driver tail": {pg("settlement_unattributed", "txn_A1,password=hunter2"), ""},
		"empty detail":            {pg("settlement_unattributed", ""), ""},
		"ids under another token": {pg("settlement_mismatch", "txn_A1"), ""},
	} {
		db.replies = []any{tc.err}
		_, err := r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", "", "")
		token := tc.err.(*pgconn.PgError).Message
		want := ErrRejected.Error() + ": " + token
		if tc.want != "" {
			want = ErrRejected.Error() + ": " + tc.want
		}
		if !errors.Is(err, ErrRejected) || err.Error() != want {
			t.Errorf("%s: %q, want %q", name, err, want)
		}
	}
}

func TestSettlementCloseAndPayoutPassOnlyTheContractArguments(t *testing.T) {
	db := &liveDB{}
	r := settlementRegistrar(t, db, &balanceFake{})
	ctx := context.Background()
	paid := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	db.replies = []any{`{"statements":[{"statement_id":"` + conn + `","store_id":"` + store +
		`","net_payable_minor":1200,"line_count":3,"replayed":false}],"operator_notes":[{"balance_txn_id":"txn_N1","target_tenant_id":"` +
		tenant + `","target_store_id":"` + store + `","operator":"op@test","ticket":"` + tick + `","note":"assigned","period_start":"2026-09-14",` +
		`"type":"adjustment","settle_amount":-641,"settle_net":-667,"settle_currency":"HKD"}]}`, paid,
		`{"balance_txn_id":"txn_R1","resolution":"not_store_revenue","target_tenant_id":null,"target_store_id":null,"operator":"op@test","ticket":"` +
			tick + `","note":"Stripe dispute fee","resolved_at":"2026-10-01 10:00:00+00","replayed":false}`,
		`{"balance_txn_id":"txn_R2","resolution":"assigned_to_store","target_tenant_id":"` + tenant + `","target_store_id":"` + store +
			`","operator":"op@test","ticket":"` + tick + `","note":"store sale","resolved_at":"2026-10-01 10:00:00+00","replayed":true}`}
	got, err := r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", store, tick)
	if err != nil || len(got.Statements) != 1 || got.Statements[0].NetPayableMinor != 1200 || got.Statements[0].LineCount != 3 {
		t.Fatalf("close: %+v %v", got, err)
	}
	// §6.6: close now carries the window's assigned_to_store resolutions as operator notes (v1 moves no money)
	if len(got.OperatorNotes) != 1 || got.OperatorNotes[0].BalanceTxnID != "txn_N1" || got.OperatorNotes[0].TargetStoreID != store {
		t.Fatalf("operator notes: %+v", got.OperatorNotes)
	}
	// P1-2: the note carries the period and the SIGNED settlement-currency amounts (a negative row must stay negative)
	if n := got.OperatorNotes[0]; n.PeriodStart != "2026-09-14" || n.Type != "adjustment" || n.SettleAmount != -641 || n.SettleNet != -667 || n.SettleCurrency != "HKD" {
		t.Fatalf("operator note amounts: %+v", n)
	}
	a := db.calls[0].args
	if a[3] != "SANDBOX" || a[4] != "2026-09-14" || a[5] != "op@test" || a[6] != store || a[7] != tick || len(a) != 8 {
		t.Fatalf("close args: %v", a)
	}
	at, err := r.SettlementPayout(ctx, scope, conn, "BANK-REF-1", 1200, paid, "op@test", tick)
	if err != nil || !at.Equal(paid) {
		t.Fatalf("payout: %v %v", at, err)
	}
	if b := db.calls[1].args; b[3] != conn || b[4] != "BANK-REF-1" || b[5] != int64(1200) || b[7] != "op@test" || b[8] != tick || len(b) != 9 || strings.Contains(db.calls[1].sql, "transfer") {
		t.Fatalf("payout args: %v", b)
	}
	res, err := r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R1", ResolveNotStoreRevenue, "", "", "Stripe dispute fee", "op@test", tick)
	if err != nil || res.Replayed || res.BalanceTxnID != "txn_R1" || res.Resolution != ResolveNotStoreRevenue || res.ResolvedAt == "" {
		t.Fatalf("resolve: %+v %v", res, err)
	}
	c := db.calls[2].args
	if c[3] != "SANDBOX" || c[4] != "txn_R1" || c[5] != ResolveNotStoreRevenue || c[6] != "op@test" || c[7] != tick ||
		c[8] != "Stripe dispute fee" || c[9] != nil || c[10] != nil || len(c) != 11 ||
		!strings.Contains(db.calls[2].sql, "record_settlement_resolution") || strings.Contains(db.calls[2].sql, "settlement_lines") {
		t.Fatalf("resolve args: %v", c)
	}
	res, err = r.SettlementResolve(ctx, scope, "SANDBOX", "txn_R2", ResolveAssignedToStore, tenant, store, "store sale", "op@test", tick)
	if err != nil || !res.Replayed || res.TargetTenantID != tenant || res.TargetStoreID != store {
		t.Fatalf("resolve assigned: %+v %v", res, err)
	}
	if d := db.calls[3].args; d[5] != ResolveAssignedToStore || d[9] != tenant || d[10] != store || len(d) != 11 {
		t.Fatalf("resolve target args: %v", d)
	}
}

// §6.7/§6.9: the return and the payables pass exactly the contract arguments, every malformed argument is refused before SQL, and the answer
// of the payables read is decoded strictly (an unknown key or a missing list is a database fault, never a partial list).
func TestSettlementPayoutReturnAndPayablesPassOnlyTheContractArguments(t *testing.T) {
	db := &liveDB{}
	r := settlementRegistrar(t, db, &balanceFake{})
	ctx := context.Background()
	at := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	db.replies = []any{at,
		`{"payables":[{"statement_id":"` + conn + `","tenant_id":"` + tenant + `","store_id":"` + store + `","period_start":"2026-09-07","currency":"TWD",` +
			`"net_payable_minor":80000,"closed_at":"2026-10-01T10:00:00.123456+00:00"}],"truncated":true}`,
		`{"payables":[],"truncated":false,"extra":1}`, `{"truncated":false}`}
	got, err := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", 125000, at.In(time.FixedZone("TPE", 8*3600)), "op@test", tick)
	if err != nil || !got.Equal(at) {
		t.Fatalf("payout return: %v %v", got, err)
	}
	a := db.calls[0].args
	if len(a) != 9 || a[3] != conn || a[4] != "BANKRET-0001" || a[5] != int64(125000) || !a[6].(time.Time).Equal(at) || a[6].(time.Time).Location() != time.UTC ||
		a[7] != "op@test" || a[8] != tick || !strings.Contains(db.calls[0].sql, "record_settlement_payout_return") || strings.Contains(db.calls[0].sql, "transfer") {
		t.Fatalf("return args: %v %s", a, db.calls[0].sql)
	}
	res, err := r.SettlementPayables(ctx, scope, "SANDBOX", 50)
	if err != nil || !res.Truncated || len(res.Payables) != 1 || res.Payables[0].NetPayableMinor != 80000 || res.Payables[0].PeriodStart != "2026-09-07" ||
		res.Payables[0].StatementID != conn || res.Payables[0].ClosedAt.IsZero() {
		t.Fatalf("payables: %+v %v", res, err)
	}
	if b := db.calls[1].args; len(b) != 5 || b[3] != "SANDBOX" || b[4] != 50 || !strings.Contains(db.calls[1].sql, "read_settlement_payables") {
		t.Fatalf("payables args: %v %s", b, db.calls[1].sql)
	}
	if _, err := r.SettlementPayables(ctx, scope, "LIVE", 10); !errors.Is(err, ErrDatabase) {
		t.Fatalf("a payables answer with an unknown key: %v", err)
	}
	if _, err := r.SettlementPayables(ctx, scope, "LIVE", 10); !errors.Is(err, ErrDatabase) {
		t.Fatalf("a payables answer with no list: %v", err)
	}

	before := len(db.calls)
	for name, f := range map[string]func() error{
		"return: bad statement": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, "nope", "BANKRET-0001", 100, at, "op@test", tick)
			return e
		},
		"payables: bad environment": func() error { _, e := r.SettlementPayables(ctx, scope, "PROD", 50); return e },
		"payables: limit 0":         func() error { _, e := r.SettlementPayables(ctx, scope, "SANDBOX", 0); return e },
		"payables: limit 201":       func() error { _, e := r.SettlementPayables(ctx, scope, "SANDBOX", 201); return e },
	} {
		if err := f(); !errors.Is(err, ErrConfig) {
			t.Errorf("%s: %v, want ErrConfig", name, err)
		}
	}
	for name, f := range map[string]func() error{
		"return: bad ref": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, conn, "x y", 100, at, "op@test", tick)
			return e
		},
		"return: zero amount": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", 0, at, "op@test", tick)
			return e
		},
		"return: negative": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", -5, at, "op@test", tick)
			return e
		},
		"return: over the bound": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", maxReturnMinor+1, at, "op@test", tick)
			return e
		},
		"return: zero time": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", 100, time.Time{}, "op@test", tick)
			return e
		},
		"return: bad operator": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", 100, at, "o", tick)
			return e
		},
		"return: missing ticket": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", 100, at, "op@test", "")
			return e
		},
		"return: short ticket": func() error {
			_, e := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", 100, at, "op@test", "short")
			return e
		},
	} {
		if err := f(); !errors.Is(err, ErrRejected) {
			t.Errorf("%s: %v, want ErrRejected", name, err)
		}
	}
	if len(db.calls) != before {
		t.Fatalf("bad arguments reached SQL (%d calls)", len(db.calls)-before)
	}
}

// §6.7/§6.8: the new refusal tokens keep only OUR token; the sync_unreconciled DETAIL (up to 20 sync run ids) is the one detail passed on.
func TestSettlementReturnAndReconciliationRefusalsKeepOnlyOurCodedToken(t *testing.T) {
	db := &liveDB{}
	r := settlementRegistrar(t, db, &balanceFake{})
	ctx := context.Background()
	pg := func(msg, detail string) error { return &pgconn.PgError{Code: "PT409", Message: msg, Detail: detail} }
	runs := func(n int) string {
		out := make([]string, n)
		for i := range out {
			out[i] = conn
		}
		return strings.Join(out, ",")
	}
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"one run":                  {pg("sync_unreconciled", conn), "sync_unreconciled: " + conn},
		"twenty runs":              {pg("sync_unreconciled", runs(20)), "sync_unreconciled: " + runs(20)},
		"twenty-one runs":          {pg("sync_unreconciled", runs(21)), "sync_unreconciled"},
		"free text":                {pg("sync_unreconciled", "row (secret=abc)"), "sync_unreconciled"},
		"a run and a driver tail":  {pg("sync_unreconciled", conn+",password=hunter2"), "sync_unreconciled"},
		"empty detail":             {pg("sync_unreconciled", ""), "sync_unreconciled"},
		"runs under another token": {pg("settlement_mismatch", conn), "settlement_mismatch"},
	} {
		db.replies = []any{tc.err}
		_, err := r.SettlementClose(ctx, scope, "SANDBOX", "2026-09-14", "op@test", "", "")
		if want := ErrRejected.Error() + ": " + tc.want; !errors.Is(err, ErrRejected) || err.Error() != want {
			t.Errorf("close/%s: %q, want %q", name, err, want)
		}
	}
	for _, token := range []string{"return_amount_mismatch", "return_already_recorded", "payout_not_recorded", "statement_unavailable"} {
		db.replies = []any{pg(token, "secret detail")}
		_, err := r.SettlementPayoutReturn(ctx, scope, conn, "BANKRET-0001", 100, time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC), "op@test", tick)
		if want := ErrRejected.Error() + ": " + token; !errors.Is(err, ErrRejected) || err.Error() != want {
			t.Errorf("return/%s: %q, want %q", token, err, want)
		}
	}
	// a missing manifest is 22023: rejected, no token, nothing of the driver message
	db.replies = []any{&pgconn.PgError{Code: "22023", Message: "invalid settlement manifest", Detail: "x"}}
	if _, err := r.RecordSettlementLines(ctx, scope, "", "", nil, winFrom, winTo); !errors.Is(err, ErrRejected) || strings.Contains(err.Error(), "manifest") {
		t.Fatalf("invalid manifest refusal: %v", err)
	}
}

// §6.8 "before any write": a multi-chunk manifest whose LAST chunk holds a bad entry is refused before the first chunk (SQL would only see
// it on the last call, after the earlier chunks committed). Zero SQL calls means zero writes.
func TestSettlementManifestLastChunkInvalidWritesNothing(t *testing.T) {
	for name, mutate := range map[string]func(*stripe.BalanceTransaction, []stripe.BalanceTransaction){
		"duplicate id":         func(x *stripe.BalanceTransaction, all []stripe.BalanceTransaction) { x.ID = all[0].ID },
		"lower-case currency":  func(x *stripe.BalanceTransaction, _ []stripe.BalanceTransaction) { x.Currency = "hkd" },
		"net above 1e11":       func(x *stripe.BalanceTransaction, _ []stripe.BalanceTransaction) { x.Net = maxManifestNet + 1 },
		"net below -1e11":      func(x *stripe.BalanceTransaction, _ []stripe.BalanceTransaction) { x.Net = -maxManifestNet - 1 },
		"not a balance txn id": func(x *stripe.BalanceTransaction, _ []stripe.BalanceTransaction) { x.ID = "ch_NotATxn" },
	} {
		db := &liveDB{}
		r := settlementRegistrar(t, db, &balanceFake{})
		txns := txnN(1203) // three chunks: 500, 500, 203
		mutate(&txns[len(txns)-1], txns)
		if _, err := r.RecordSettlementLines(context.Background(), scope, conn, tick, txns, winFrom, winTo); !errors.Is(err, ErrRejected) {
			t.Fatalf("%s: %v, want ErrRejected", name, err)
		}
		if len(db.calls) != 0 {
			t.Fatalf("%s: %d SQL calls, want 0 (nothing written)", name, len(db.calls))
		}
	}
	edge := txnN(2)
	edge[0].Net, edge[1].Net = maxManifestNet, -maxManifestNet
	if !validManifest(edge) || !validManifest(nil) {
		t.Fatal("abs(net) == 1e11 and an empty manifest are valid")
	}
}

// §8 tw_legal_opinion (SA-1): the platform approval is the base 11 codes plus five; the pre-0200 set of 15 is refused before any SQL.
// (Lives here because live_test.go is outside SA-1's write_paths.)
func TestPlatformChecklistRequiresTWLegalOpinion(t *testing.T) {
	full := append(append([]string{}, LiveChecklist...), PlatformChecklist...)
	if !validChecklist(LiveChecklist) || !validChecklist(full) || len(full) != 16 {
		t.Fatalf("base 11 and platform 16 must be valid (got %d codes)", len(full))
	}
	var old []string
	for _, c := range full {
		if c != "tw_legal_opinion" {
			old = append(old, c)
		}
	}
	if len(old) != 15 || validChecklist(old) {
		t.Fatal("the platform set without tw_legal_opinion must be refused")
	}
}
