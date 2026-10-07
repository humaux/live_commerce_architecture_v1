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

func TestSettlementSyncChunksAndCarriesTheWindowOnTheLastChunk(t *testing.T) {
	db := &liveDB{}
	txns := txnN(1203)
	p := &balanceFake{txns: txns}
	r := settlementRegistrar(t, db, p)
	db.replies = []any{storedReply(t, r, liveRAK),
		`{"inserted":500,"duplicate":0,"unattributed":0,"mismatch":0}`,
		`{"inserted":498,"duplicate":0,"unattributed":2,"mismatch":1}`,
		`{"inserted":100,"duplicate":100,"unattributed":3,"mismatch":0,"window_net":{"HKD":108270}}`}
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
		if len(c.args) != 9 || (c.args[5] != nil) != last || (c.args[6] != nil) != last {
			t.Fatalf("chunk %d: args %v; only the last chunk carries the window", i, c.args)
		}
		if c.args[3] != "LIVE" || c.args[7] != conn || c.args[8] != tick {
			t.Fatalf("environment/connection/ticket args = %v %v %v", c.args[3], c.args[7], c.args[8])
		}
	}
	if w := db.calls[3].args; !w[5].(time.Time).Equal(winFrom) || !w[6].(time.Time).Equal(winTo) {
		t.Fatalf("window args %v %v", w[5], w[6])
	}
	if rep.Fetched != 1203 || rep.Inserted != 1098 || rep.Duplicate != 100 || rep.Unattributed != 5 || rep.Mismatch != 1 {
		t.Fatalf("report: %+v", rep)
	}
	// 1203 txns of net 90 = 108270 on both sides: reconciled
	if rep.StripeNet["HKD"] != 108270 || rep.RecordedNet["HKD"] != 108270 || rep.Difference["HKD"] != 0 {
		t.Fatalf("reconciliation: %+v", rep)
	}
	// a persisted total that differs from Stripe's is printed as a number, not hidden
	db2 := &liveDB{}
	r2 := settlementRegistrar(t, db2, &balanceFake{txns: txnN(2)})
	db2.replies = []any{storedReply(t, r2, liveRAK), `{"inserted":1,"duplicate":0,"unattributed":0,"mismatch":0,"window_net":{"HKD":80}}`}
	rep2, err := r2.SettlementSync(context.Background(), scope, conn, 2, winFrom, winTo, tick)
	if err != nil || rep2.Difference["HKD"] != -100 {
		t.Fatalf("difference: %+v %v", rep2, err)
	}
	// an empty window still records its coverage (a JSON array, never null)
	db3 := &liveDB{}
	r3 := settlementRegistrar(t, db3, &balanceFake{})
	db3.replies = []any{storedReply(t, r3, liveRAK), `{"inserted":0,"duplicate":0,"unattributed":0,"mismatch":0,"window_net":{}}`}
	if _, err := r3.SettlementSync(context.Background(), scope, conn, 2, winFrom, winTo, tick); err != nil || len(db3.calls) != 2 || db3.calls[1].args[4].(string) != "[]" || len(db3.calls[1].args) != 9 || db3.calls[1].args[5] == nil {
		t.Fatalf("empty window: %v calls=%d", err, len(db3.calls))
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
