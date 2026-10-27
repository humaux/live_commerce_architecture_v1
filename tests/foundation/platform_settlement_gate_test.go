package foundation_test

// SA-3 independent settlement gate (contracts/stripe-platform-account-v1.md §6.9 and §10 PF09-PF18; author is NOT SA-1).
// Purpose: prove the frozen SA-0 vectors PF16 (payout returns), PF17 (sync reconciliation) and PF18 (payables) from the
//   contract text alone, without reading the SA-1 branch or its implementation.
// Depends on: the existing platform-settlement harness (pfNew/merchant/derived/pay/mustRefund), the real capture and refund
//   paths, and the SQL entry points named by §6.9. Tier: MOCK (fixture balance-transaction lists, never Stripe) + REAL_PG.
// Used by: .github/workflows/gates.yml foundation shards; no test is run locally by the author (owner rule).
// Invariants: every assertion cites a §6.9 vector; owner-pool fixtures are named at their call site and never replace a
//   product entry point where §6.9 names one. The new SQL signatures are discovered from pg_proc so this file compiles and
//   runs on the pre-SA-1 baseline (where the functions are missing), making the baseline RED behavioural rather than a
//   compile failure that would take PF09-PF13 down with it.
//
// Baseline expectation (no SA-1 implementation): PF16-PF18 fail because payments.record_settlement_payout_return,
// payments.read_settlement_payables and the ten-argument payments.record_settlement_lines do not exist yet. PF09-PF13 stay
// green in platform_settlement_test.go. After SA-1 lands these vectors must be green.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/catalog"
	"livecommerce/internal/integrations/psp/stripe"
	"livecommerce/internal/platform"
)

const gateTicket = "OPS-2026-0001"

// gateEnv is one isolated database (pfNew creates a fresh PG) plus a registrar-role pool. The registrar pool is the same
// role cmd/stripe-admin logs in as; no owner connection is used to call a §6.9 entry point.
type gateEnv struct {
	*pfEnv
	reg                  *pgxpool.Pool
	tenant, store, princ string
}

func gateNew(t *testing.T) *gateEnv {
	t.Helper()
	pf := pfNew(t)
	pf.designate(t)
	pf.setOpen(t, true)
	e := &gateEnv{pfEnv: pf, tenant: pf.op.TenantID, store: pf.op.StoreID, princ: pf.op.PrincipalID}
	url := sstLogin(t, pf.f, "commerce_payment_registrar")
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("registrar pool: %v", err)
	}
	t.Cleanup(pool.Close)
	e.reg = pool
	return e
}

// reprice changes the store's only SKU price through catalog.SetSKUPrice (the real merchant path) and returns the updated
// harness. The §6.9 vectors fix captured TWD amounts, so the fixture must create orders at those prices.
func (g *gateEnv) reprice(t *testing.T, m psHarness, price int64) psHarness {
	t.Helper()
	sku := m.stock.skus[0]
	next, err := t04Scoped(context.Background(), g.f, m.f.tokens["a"], m.f.storeA1, "catalog:write",
		func(tx pgx.Tx, s platform.Scope) (catalog.SKU, error) {
			return catalog.SetSKUPrice(context.Background(), tx, s, t04Key("gate-price"), sku.ID,
				catalog.PriceInput{PriceMinor: price, ExpectedVersion: sku.Version})
		})
	if err != nil {
		t.Fatalf("set SKU price %d: %v", price, err)
	}
	m.stock.skus[0] = next
	return m
}

// gateStore creates one allowlisted merchant store on the platform account, makes a fresh 2-unit hold at price, and pays it
// through the real capture path. It returns the store and its paid order.
func (g *gateEnv) gateStore(t *testing.T, price int64) (psHarness, sstStore, rfxOrder) {
	t.Helper()
	m := g.merchant(t, true)
	g.mustSet(t, m, pfInput(true, 0, ""))
	s := g.derived(t, m)
	if price > 0 {
		m = g.reprice(t, m, price)
	}
	m = sstMoreHold(t, m)
	s.p = m
	o := g.pay(t, s, g.endpoint, g.secret)
	return m, s, o
}

func (g *gateEnv) refundID(t *testing.T, refund string) string {
	t.Helper()
	var id string
	if err := g.f.owner.QueryRow(context.Background(), `SELECT stripe_refund_id FROM payments.stripe_refunds WHERE id=$1`, refund).Scan(&id); err != nil {
		t.Fatalf("stripe refund id: %v", err)
	}
	return id
}

// ---------------------------------------------------------------------------------------------------------------------
// §6.9 entry points: discovered from pg_proc, called with named notation.
// ---------------------------------------------------------------------------------------------------------------------

type gateQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type gateArg struct{ name, typ string }

func (g *gateEnv) fn(t *testing.T, name string, wantArgs int) string {
	t.Helper()
	var oid string
	var n int
	if err := g.f.owner.QueryRow(context.Background(), `SELECT p.oid::text::text,count(*) OVER () FROM pg_proc p
		WHERE p.pronamespace='payments'::regnamespace AND p.proname=$1`, name).Scan(&oid, &n); err != nil {
		t.Fatalf("payments.%s is missing: %v", name, err)
	}
	if n != 1 {
		t.Fatalf("payments.%s has %d overloads, want exactly 1", name, n)
	}
	var got int
	if err := g.f.owner.QueryRow(context.Background(), `SELECT pronargs FROM pg_proc WHERE oid=$1::oid`, oid).Scan(&got); err != nil {
		t.Fatalf("pronargs %s: %v", name, err)
	}
	if wantArgs > 0 && got != wantArgs {
		t.Fatalf("payments.%s has %d arguments, want %d", name, got, wantArgs)
	}
	return oid
}

func (g *gateEnv) fnArgs(t *testing.T, oid string) []gateArg {
	t.Helper()
	rows, err := g.f.owner.Query(context.Background(), `SELECT coalesce(p.proargnames[a.ord],''),format_type(p.proargtypes[a.ord-1],NULL)
		FROM pg_proc p, generate_series(1,p.pronargs) a(ord) WHERE p.oid=$1::oid ORDER BY a.ord`, oid)
	if err != nil {
		t.Fatalf("pg_proc arguments: %v", err)
	}
	defer rows.Close()
	var out []gateArg
	for rows.Next() {
		var a gateArg
		if err := rows.Scan(&a.name, &a.typ); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// gateKey maps the contract's parameter names to the test's canonical value keys. The wildcard manifest branch keeps the
// call independent of where SA-1 places the new argument.
func gateKey(name string) string {
	n := strings.TrimPrefix(name, "p_")
	switch {
	case n == "tenant", n == "store", n == "principal", n == "environment", n == "lines", n == "from",
		n == "to", n == "connection", n == "ticket", n == "statement", n == "return_ref", n == "amount",
		n == "returned_at", n == "operator", n == "period", n == "limit", n == "target", n == "payout_ref",
		n == "payout_minor", n == "paid_at":
		return n
	case strings.Contains(n, "manifest"):
		return "manifest"
	case strings.Contains(n, "window_from"):
		return "from"
	case strings.Contains(n, "window_to"):
		return "to"
	case strings.Contains(n, "period_start"):
		return "period"
	case strings.Contains(n, "return_ref"):
		return "return_ref"
	case strings.Contains(n, "amount_minor"):
		return "amount"
	case strings.Contains(n, "returned_at"):
		return "returned_at"
	case strings.Contains(n, "target_store"):
		return "target"
	default:
		return n
	}
}

func (g *gateEnv) callOn(ctx context.Context, q gateQuerier, name string, vals map[string]any) (string, error) {
	oid, err := g.lookupFn(name)
	if err != nil {
		return "", err
	}
	args, err := g.lookupArgs(oid)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(args))
	argv := make([]any, 0, len(args))
	for i, a := range args {
		key := gateKey(a.name)
		v, ok := vals[key]
		if !ok {
			return "", fmt.Errorf("no value for payments.%s argument %q (canonical %q)", name, a.name, key)
		}
		parts = append(parts, fmt.Sprintf("%s => $%d::%s", a.name, i+1, a.typ))
		argv = append(argv, v)
	}
	sql := "SELECT to_jsonb(payments." + name + "(" + strings.Join(parts, ",") + "))::text"
	var out *string
	err = q.QueryRow(ctx, sql, argv...).Scan(&out)
	if out == nil {
		return "", err
	}
	return *out, err
}

func (g *gateEnv) lookupFn(name string) (string, error) {
	var oid string
	var n int
	err := g.f.owner.QueryRow(context.Background(), `SELECT p.oid::text,count(*) OVER () FROM pg_proc p
		WHERE p.pronamespace='payments'::regnamespace AND p.proname=$1`, name).Scan(&oid, &n)
	if err != nil {
		return "", fmt.Errorf("payments.%s is missing: %w", name, err)
	}
	if n != 1 {
		return "", fmt.Errorf("payments.%s has %d overloads, want exactly 1", name, n)
	}
	return oid, nil
}

func (g *gateEnv) lookupArgs(oid string) ([]gateArg, error) {
	rows, err := g.f.owner.Query(context.Background(), `SELECT coalesce(p.proargnames[a.ord],''),format_type(p.proargtypes[a.ord-1],NULL)
		FROM pg_proc p, generate_series(1,p.pronargs) a(ord) WHERE p.oid=$1::oid ORDER BY a.ord`, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []gateArg
	for rows.Next() {
		var a gateArg
		if err := rows.Scan(&a.name, &a.typ); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (g *gateEnv) callFn(t *testing.T, name string, vals map[string]any) (map[string]any, error) {
	t.Helper()
	g.fn(t, name, 0)
	raw, err := g.callOn(context.Background(), g.reg, name, vals)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if strings.HasPrefix(raw, "{") {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out, nil
}

func (g *gateEnv) mustCall(t *testing.T, what, name string, vals map[string]any) map[string]any {
	t.Helper()
	out, err := g.callFn(t, name, vals)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	return out
}

func gateSQLState(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code
	}
	return fmt.Sprint(err)
}

// ---------------------------------------------------------------------------------------------------------------------
// Fixture balance transactions (§6.9 vectors; the contract's numbers, never Stripe).
// ---------------------------------------------------------------------------------------------------------------------

func gateCharge(id, pi string, store, settle, fee int64, rate string, at time.Time) stripe.BalanceTransaction {
	r := json.Number(rate)
	return stripe.BalanceTransaction{ID: id, Type: "charge", ReportingCategory: pslS("charge"), Amount: settle, Fee: pslI(fee),
		Net: settle - fee, Currency: "HKD", ExchangeRate: &r, Created: at.Unix(), SourceID: pslS("ch_" + id[4:]),
		SourceObject: pslS("charge"), ChargePaymentIntent: pslS(pi), ChargeAmount: pslI(store), ChargeCurrency: pslS("TWD")}
}

func gateRefund(id, refund string, store, settle, fee int64, rate string, at time.Time) stripe.BalanceTransaction {
	r := json.Number(rate)
	return stripe.BalanceTransaction{ID: id, Type: "refund", ReportingCategory: pslS("refund"), Amount: -settle, Fee: pslI(fee),
		Net: -settle - fee, Currency: "HKD", ExchangeRate: &r, Created: at.Unix(), SourceID: pslS(refund), SourceObject: pslS("refund"),
		RefundID: pslS(refund), RefundAmount: pslI(store), RefundCurrency: pslS("TWD")}
}

func gateRaw(id, typ string, amount int64, fee *int64, at time.Time) stripe.BalanceTransaction {
	net := amount
	if fee != nil {
		net = amount - *fee
	}
	return stripe.BalanceTransaction{ID: id, Type: typ, ReportingCategory: pslS(typ), Amount: amount, Fee: fee, Net: net,
		Currency: "HKD", Created: at.Unix(), SourceID: pslS("src_" + id[4:]), SourceObject: pslS(typ)}
}

// gateManifestEntry is the §6.8 manifest element: exactly {"id","currency","net"}; any other key is 22023.
type gateManifestEntry struct {
	ID       string `json:"id"`
	Currency string `json:"currency"`
	Net      int64  `json:"net"`
}

func gateManifest(txs []stripe.BalanceTransaction) []gateManifestEntry {
	out := make([]gateManifestEntry, 0, len(txs))
	for _, tx := range txs {
		out = append(out, gateManifestEntry{ID: tx.ID, Currency: tx.Currency, Net: tx.Net})
	}
	return out
}

// sync calls the ten-argument §6.9 entry point: projected lines plus the full observed manifest.
func (g *gateEnv) sync(t *testing.T, from, to time.Time, lines, manifest []stripe.BalanceTransaction) map[string]any {
	t.Helper()
	if lines == nil {
		lines = []stripe.BalanceTransaction{}
	}
	if manifest == nil {
		manifest = []stripe.BalanceTransaction{}
	}
	l, err := json.Marshal(lines)
	if err != nil {
		t.Fatal(err)
	}
	m, err := json.Marshal(gateManifest(manifest))
	if err != nil {
		t.Fatal(err)
	}
	out, err := g.callFn(t, "record_settlement_lines", map[string]any{
		"tenant": g.tenant, "store": g.store, "principal": g.princ, "environment": "SANDBOX",
		"lines": string(l), "manifest": string(m), "from": from, "to": to, "connection": g.plat.connection, "ticket": gateTicket,
	})
	if err != nil {
		t.Fatalf("sync [%s,%s): %v", from.Format(time.RFC3339), to.Format(time.RFC3339), err)
	}
	return out
}

func (g *gateEnv) close(t *testing.T, period string) map[string]any {
	t.Helper()
	return g.mustCall(t, "close "+period, "close_settlement", map[string]any{
		"tenant": g.tenant, "store": g.store, "principal": g.princ, "environment": "SANDBOX",
		"period": period, "operator": "op@test", "target": nil, "ticket": gateTicket,
	})
}

// closeProbe runs close inside a transaction and rolls it back (keeping the committed coverage runs; no statement or line).
func (g *gateEnv) closeProbe(t *testing.T, period string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := g.reg.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = g.callOn(ctx, tx, "close_settlement", map[string]any{
		"tenant": g.tenant, "store": g.store, "principal": g.princ, "environment": "SANDBOX",
		"period": period, "operator": "op@test", "target": nil, "ticket": gateTicket,
	})
	if err != nil {
		return err
	}
	return tx.Rollback(ctx)
}

func (g *gateEnv) latestRun(t *testing.T) string {
	t.Helper()
	var id string
	if err := g.f.owner.QueryRow(context.Background(), `SELECT id::text FROM payments.settlement_sync_runs
		ORDER BY synced_at DESC,ctid DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("latest sync run: %v", err)
	}
	return id
}

// runOrder reports the run's observation order (observation_seq when SA-1 adds it, else its synced_at).
func (g *gateEnv) runOrder(t *testing.T, run string) string {
	t.Helper()
	if g.hasColumn(t, "payments", "settlement_sync_runs", "observation_seq") {
		var seq int64
		if err := g.f.owner.QueryRow(context.Background(), `SELECT observation_seq FROM payments.settlement_sync_runs WHERE id=$1`, run).Scan(&seq); err != nil {
			t.Fatalf("observation_seq: %v", err)
		}
		return fmt.Sprintf("%020d", seq)
	}
	var at time.Time
	if err := g.f.owner.QueryRow(context.Background(), `SELECT synced_at FROM payments.settlement_sync_runs WHERE id=$1`, run).Scan(&at); err != nil {
		t.Fatalf("synced_at: %v", err)
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func (g *gateEnv) hasColumn(t *testing.T, schema, table, column string) bool {
	t.Helper()
	var ok bool
	if err := g.f.owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM information_schema.columns
		WHERE table_schema=$1 AND table_name=$2 AND column_name=$3)`, schema, table, column).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

// reconColumns discovers the §6.8 reconciliation table by its two invariants (a sync_run_id and a difference column)
// instead of hard-coding a name this test is not allowed to read from §6.8.
type reconTable struct{ table, diff, stripeNet, recordedNet, stripeCount, recordedCount, currency string }

func (g *gateEnv) recon(t *testing.T) reconTable {
	t.Helper()
	rows, err := g.f.owner.Query(context.Background(), `SELECT c.relname,a.attname FROM pg_class c
		JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
		WHERE n.nspname='payments' AND c.relkind='r' ORDER BY c.relname,a.attnum`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols := map[string]map[string]bool{}
	for rows.Next() {
		var tbl, col string
		if err := rows.Scan(&tbl, &col); err != nil {
			t.Fatal(err)
		}
		if cols[tbl] == nil {
			cols[tbl] = map[string]bool{}
		}
		cols[tbl][col] = true
	}
	var out reconTable
	for tbl, c := range cols {
		if !c["sync_run_id"] {
			continue
		}
		for col := range c {
			lc := strings.ToLower(col)
			switch {
			case strings.Contains(lc, "difference"):
				out.diff = col
			case strings.Contains(lc, "stripe") && strings.Contains(lc, "net") && !strings.Contains(lc, "count"):
				out.stripeNet = col
			case strings.Contains(lc, "recorded") && strings.Contains(lc, "net") && !strings.Contains(lc, "count"):
				out.recordedNet = col
			case strings.Contains(lc, "stripe") && strings.Contains(lc, "count"):
				out.stripeCount = col
			case strings.Contains(lc, "recorded") && strings.Contains(lc, "count"):
				out.recordedCount = col
			case lc == "currency":
				out.currency = col
			}
		}
		if out.diff != "" {
			out.table = tbl
			break
		}
	}
	if out.table == "" || out.diff == "" || out.stripeNet == "" || out.recordedNet == "" || out.stripeCount == "" || out.recordedCount == "" {
		t.Fatalf("reconciliation table not found (table=%q cols=%+v)", out.table, out)
	}
	return out
}

type reconRow struct {
	diff, stripe, recorded     int64
	stripeCount, recordedCount int
}

func (g *gateEnv) reconOf(t *testing.T, run string) reconRow {
	t.Helper()
	r := g.recon(t)
	q := fmt.Sprintf(`SELECT %s::bigint,%s::bigint,%s::bigint,%s::int,%s::int FROM payments.%s WHERE sync_run_id=$1`,
		r.diff, r.stripeNet, r.recordedNet, r.stripeCount, r.recordedCount, r.table)
	args := []any{run}
	if r.currency != "" {
		q += " AND " + r.currency + "='HKD'"
	}
	q += " LIMIT 1"
	var out reconRow
	if err := g.f.owner.QueryRow(context.Background(), q, args...).Scan(&out.diff, &out.stripe, &out.recorded, &out.stripeCount, &out.recordedCount); err != nil {
		t.Fatalf("reconciliation of run %s: %v", run, err)
	}
	return out
}

func (g *gateEnv) fixtureExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := g.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role=replica`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("fixture %.80s: %v", sql, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func (g *gateEnv) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := g.f.owner.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %.80s: %v", sql, err)
	}
	return n
}

func (g *gateEnv) statement(t *testing.T, store, period string) (id string, net, captured, refunded, fee int64) {
	t.Helper()
	if err := g.f.owner.QueryRow(context.Background(), `SELECT id::text,net_payable_minor,captured_minor,refunded_minor,stripe_fee_minor
		FROM payments.settlement_statements WHERE store_id=$1 AND environment='SANDBOX' AND period_start=$2::date`,
		store, period).Scan(&id, &net, &captured, &refunded, &fee); err != nil {
		t.Fatalf("statement %s %s: %v", store, period, err)
	}
	return
}

// returnedIn read the statement's returned_in_minor. The frozen contract requires the value, not a particular storage: use the
// column when SA-1 stores it, otherwise sum the returns carried into the statement.
func (g *gateEnv) returnedIn(t *testing.T, statement string) int64 {
	t.Helper()
	if g.hasColumn(t, "payments", "settlement_statements", "returned_in_minor") {
		var n int64
		if err := g.f.owner.QueryRow(context.Background(), `SELECT returned_in_minor FROM payments.settlement_statements WHERE id=$1`, statement).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	var n int64
	if err := g.f.owner.QueryRow(context.Background(), `SELECT coalesce(sum(returned_minor),0) FROM payments.`+g.returnTable(t)+` WHERE carried_into_statement_id=$1`, statement).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// gateFee is the §6.3 Go reference used only to document the fixture arithmetic in failure messages.
func gateFee(fee, store, settle int64) int64 {
	num, den := big.NewInt(fee*store), big.NewInt(settle*100)
	negative := num.Sign()*den.Sign() < 0
	q := new(big.Rat).SetFrac(new(big.Int).Abs(num), new(big.Int).Abs(den))
	q.Add(q, big.NewRat(1, 2))
	mag := new(big.Int).Div(q.Num(), q.Denom())
	steps := -100 * mag.Int64()
	if negative {
		return -steps
	}
	return steps
}

func gateAt(y int, m time.Month, d, hour int) time.Time {
	return time.Date(y, m, d, hour, 0, 0, 0, pslTPE)
}

func gateWeek(y int, m time.Month, d int) time.Time { return pslDay(y, m, d) }

// returnTable discovers the §6.7 return table by the set-once carry column named in §6.9 (S-I2), so the gate does not depend
// on a table name it is not allowed to read from §6.7.
func (g *gateEnv) returnTable(t *testing.T) string {
	t.Helper()
	var tbl string
	if err := g.f.owner.QueryRow(context.Background(), `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname='payments' AND c.relkind='r' AND EXISTS(SELECT 1 FROM pg_attribute a WHERE a.attrelid=c.oid
		AND a.attname='carried_into_statement_id' AND a.attnum>0 AND NOT a.attisdropped) LIMIT 1`).Scan(&tbl); err != nil {
		t.Fatalf("settlement return table (carried_into_statement_id) missing: %v", err)
	}
	return tbl
}

func (g *gateEnv) callRaw(t *testing.T, name string, vals map[string]any) (string, error) {
	t.Helper()
	g.fn(t, name, 0)
	return g.callOn(context.Background(), g.reg, name, vals)
}

func gateTime(t *testing.T, raw string) time.Time {
	t.Helper()
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("recorded_at json %q: %v", raw, err)
	}
	out, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("recorded_at %q: %v", s, err)
	}
	return out
}

func gateFind(v any, key string) (any, bool) {
	switch x := v.(type) {
	case map[string]any:
		if got, ok := x[key]; ok {
			return got, true
		}
		for _, child := range x {
			if got, ok := gateFind(child, key); ok {
				return got, true
			}
		}
	case []any:
		for _, child := range x {
			if got, ok := gateFind(child, key); ok {
				return got, true
			}
		}
	}
	return nil, false
}

func gateNum(t *testing.T, v any, key string) int64 {
	t.Helper()
	got, ok := gateFind(v, key)
	if !ok {
		t.Fatalf("merchant body has no %q: %v", key, v)
	}
	n, ok := got.(float64)
	if !ok {
		t.Fatalf("%q is %T, want a number", key, got)
	}
	return int64(n)
}

// insertStatement is the disclosed owner-pool fixture for a closed statement that exists only to be read (the unpaid
// second store in PF16a and the four PF18 payables): no money fact is invented, only an already-closed ledger row.
// insertStatement writes a line-less statement whose whole net is carried_in, so a later close replay over the same period
// (0150: stored sums must equal the sums of its lines) still holds; a captured amount with no lines is settlement_changed.
func (g *gateEnv) insertStatement(t *testing.T, tenant, store string, net int64, period string, closed time.Time) string {
	t.Helper()
	col, val := "", ""
	if g.hasColumn(t, "payments", "settlement_statements", "returned_in_minor") {
		col, val = ",returned_in_minor", ",0"
	}
	var id string
	if err := g.f.owner.QueryRow(context.Background(), `INSERT INTO payments.settlement_statements(id,tenant_id,store_id,environment,
		period_start,period_end,currency,captured_minor,refunded_minor,dispute_minor,stripe_fee_minor,platform_fee_bps,platform_fee_minor,
		carried_in_minor`+col+`,net_payable_minor,line_count,lines_sha256,closed_at,closed_by)
		VALUES(gen_random_uuid(),$1,$2,'SANDBOX',$3::date,$3::date+7,'TWD',0,0,0,0,0,0,$4`+val+`,$4,0,sha256('x'),$5,'fixture@test')
		RETURNING id::text`, tenant, store, period, net, closed).Scan(&id); err != nil {
		t.Fatalf("statement fixture %s/%s: %v", tenant, period, err)
	}
	return id
}

func TestPlatformSettlementPF16PayoutReturn(t *testing.T) {
	g := gateNew(t)
	// §6.9 PF16 entry point: on the pre-SA-1 baseline this is the behavioural RED (the function does not exist yet); it is
	// never a compile failure, so PF09-PF13 keep running.
	g.fn(t, "record_settlement_payout_return", 0)
	w1, w2, w3 := gateWeek(2026, 9, 7), gateWeek(2026, 9, 14), gateWeek(2026, 9, 21)
	u := gateWeek(2026, 8, 31)
	at := func(w time.Time, d, hour int) time.Time {
		return w.Add(time.Duration(d)*24*time.Hour + time.Duration(hour)*time.Hour)
	}
	paid := w1.Add(11*24*time.Hour + 12*time.Hour) // fixed T = 2026-09-18T12:00+08:00, after W1's close gate

	// one store: W1 charge 130000 (price 65000 x 2), then W2 charge 50000 (price 25000 x 2)
	m, s, o1 := g.gateStore(t, 65000)
	if o1.captured != 130000 {
		t.Fatalf("W1 captured = %d, want 130000", o1.captured)
	}
	m = g.reprice(t, m, 25000)
	m = sstMoreHold(t, m)
	s.p = m
	o2 := g.pay(t, s, g.endpoint, g.secret)
	if o2.captured != 50000 {
		t.Fatalf("W2 captured = %d, want 50000", o2.captured)
	}

	// §6.9 PF16 setup: the W1 line is store 130000 / fee -5000 (settle 26000, Stripe fee 1000 -> -5000 at step 100).
	a1 := gateCharge("txn_PF16W1", o1.pi, 130000, 26000, 1000, "0.2", at(w1, 1, 12))
	a2 := gateCharge("txn_PF16W2", o2.pi, 50000, 20000, 720, "0.4", at(w2, 1, 12))
	if got := gateFee(1000, 130000, 26000); got != -5000 {
		t.Fatalf("fixture fee reference: %d, want -5000", got)
	}
	if got := gateFee(720, 50000, 20000); got != -1800 {
		t.Fatalf("fixture fee reference: %d, want -1800", got)
	}

	// clean coverage before every close (review P2-1/P2-2): U + W1 cover W1, W1 + W2 cover W2, W2 + W3 cover W3.
	g.sync(t, u, w1, nil, nil)
	g.sync(t, w1, w2, []stripe.BalanceTransaction{a1}, []stripe.BalanceTransaction{a1})
	g.sync(t, w2, w3, []stripe.BalanceTransaction{a2}, []stripe.BalanceTransaction{a2})
	g.sync(t, w3, w3.AddDate(0, 0, 7), nil, nil)

	tbl := g.returnTable(t)
	rows := func() int { return g.count(t, "SELECT count(*) FROM payments."+tbl) }
	if n := rows(); n != 0 {
		t.Fatalf("return rows before any return = %d", n)
	}

	// a second store with a closed, still-unpaid W1 statement (owner-pool fixture: only a statement, no money fact).
	b := g.merchant(t, false)
	bStmt := g.insertStatement(t, b.f.tenantA, b.f.storeA1, 125000, w1.Format("2006-01-02"), time.Now().Add(-time.Hour))

	// close W1, then record payout T.
	closed := g.close(t, w1.Format("2006-01-02"))
	if len(closed) == 0 {
		t.Fatal("W1 close returned no statements")
	}
	w1id, w1net, w1cap, _, w1fee := g.statement(t, m.f.storeA1, w1.Format("2006-01-02"))
	if w1net != 125000 || w1cap != 130000 || w1fee != -5000 {
		t.Fatalf("W1 totals net=%d captured=%d fee=%d, want 125000/130000/-5000", w1net, w1cap, w1fee)
	}
	payout := map[string]any{"tenant": g.tenant, "store": g.store, "principal": g.princ, "statement": w1id,
		"payout_ref": "BANK-PAY-W1", "payout_minor": int64(125000), "paid_at": paid, "operator": "op@test", "ticket": gateTicket}
	g.mustCall(t, "record W1 payout", "record_settlement_payout", payout)

	ret := func(statement, ref string, amount int64, when time.Time) (string, error) {
		return g.callRaw(t, "record_settlement_payout_return", map[string]any{
			"tenant": g.tenant, "store": g.store, "principal": g.princ, "statement": statement,
			"return_ref": ref, "amount": amount, "returned_at": when, "operator": "op@test", "ticket": gateTicket})
	}

	t.Run("PF16a_unpaid_statement", func(t *testing.T) {
		_, err := ret(bStmt, "BANKRET-UNPAID", 125000, paid.Add(24*time.Hour))
		if gateSQLState(err) != "PT409" || !strings.Contains(fmt.Sprint(err), "payout_not_recorded") {
			t.Fatalf("return on an unpaid statement: %v", err)
		}
		if n := rows(); n != 0 {
			t.Fatalf("refused return wrote %d rows", n)
		}
	})

	t.Run("PF16b_amount_mismatch", func(t *testing.T) {
		_, err := ret(w1id, "BANKRET-0001", 124900, paid.Add(24*time.Hour))
		if gateSQLState(err) != "PT409" || !strings.Contains(fmt.Sprint(err), "return_amount_mismatch") {
			t.Fatalf("amount 124900: %v", err)
		}
		if n := rows(); n != 0 {
			t.Fatalf("refused return wrote %d rows", n)
		}
	})

	t.Run("PF16c_returned_before_paid", func(t *testing.T) {
		_, err := ret(w1id, "BANKRET-0001", 125000, paid.Add(-time.Second))
		if gateSQLState(err) != "22023" || !strings.Contains(fmt.Sprint(err), "invalid settlement payout return") {
			t.Fatalf("returned_at T-1s: %v", err)
		}
		if n := rows(); n != 0 {
			t.Fatalf("refused return wrote %d rows", n)
		}
	})

	var recorded time.Time
	t.Run("PF16d_record", func(t *testing.T) {
		raw, err := ret(w1id, "BANKRET-0001", 125000, paid.Add(24*time.Hour))
		if err != nil {
			t.Fatalf("valid return: %v", err)
		}
		recorded = gateTime(t, raw)
		if recorded.IsZero() {
			t.Fatal("recorded_at is zero")
		}
		if n := rows(); n != 1 {
			t.Fatalf("return rows = %d, want 1", n)
		}
		if n := g.count(t, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.payout_return'`); n != 1 {
			t.Fatalf("payout_return audit rows = %d, want 1", n)
		}
		if n := g.count(t, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.payout_return' AND details::text LIKE '%BANKRET-0001%'`); n != 0 {
			t.Fatalf("audit details leak the return reference")
		}
	})

	t.Run("PF16e_replay", func(t *testing.T) {
		raw, err := ret(w1id, "BANKRET-0001", 125000, paid.Add(24*time.Hour))
		if err != nil {
			t.Fatalf("identical replay: %v", err)
		}
		if again := gateTime(t, raw); !again.Equal(recorded) {
			t.Fatalf("replay recorded_at %v, want %v", again, recorded)
		}
		if n := rows(); n != 1 {
			t.Fatalf("return rows after replay = %d, want 1", n)
		}
		if n := g.count(t, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.payout_return'`); n != 1 {
			t.Fatalf("payout_return audit rows after replay = %d, want 1", n)
		}
	})

	t.Run("PF16f_second_return", func(t *testing.T) {
		_, err := ret(w1id, "BANKRET-0002", 125000, paid.Add(24*time.Hour))
		if gateSQLState(err) != "PT409" || !strings.Contains(fmt.Sprint(err), "return_already_recorded") {
			t.Fatalf("second reference: %v", err)
		}
	})

	var w2id string
	t.Run("PF16g_carry_once", func(t *testing.T) {
		g.close(t, w2.Format("2006-01-02"))
		var w2net int64
		w2id, w2net, _, _, _ = g.statement(t, m.f.storeA1, w2.Format("2006-01-02"))
		if w2net != 173200 {
			t.Fatalf("W2 net = %d, want 173200 (50000 - 1800 + 125000)", w2net)
		}
		if returned := g.returnedIn(t, w2id); returned != 125000 {
			t.Fatalf("W2 returned_in_minor = %d, want 125000", returned)
		}
		var carried *string
		if err := g.f.owner.QueryRow(context.Background(), `SELECT carried_into_statement_id::text FROM payments.`+tbl+` LIMIT 1`).Scan(&carried); err != nil {
			t.Fatal(err)
		}
		if carried == nil || *carried != w2id {
			t.Fatalf("return carried_into = %v, want W2 %s", carried, w2id)
		}
	})

	t.Run("PF16h_replay_and_quiet_week", func(t *testing.T) {
		g.close(t, w2.Format("2006-01-02")) // replay
		id, net, _, _, _ := g.statement(t, m.f.storeA1, w2.Format("2006-01-02"))
		if id != w2id || net != 173200 {
			t.Fatalf("W2 replay changed the statement: %s %d", id, net)
		}
		g.close(t, w3.Format("2006-01-02"))
		w3id, net3, _, _, _ := g.statement(t, m.f.storeA1, w3.Format("2006-01-02"))
		if net3 != 0 {
			t.Fatalf("W3 net = %d, want 0 (a carried return is carried once)", net3)
		}
		if ret3 := g.returnedIn(t, w3id); ret3 != 0 {
			t.Fatalf("W3 returned_in_minor = %d, want 0", ret3)
		}
	})

	t.Run("PF16i_merchant_dto", func(t *testing.T) {
		for _, tc := range []struct {
			id   string
			key  string
			want int64
		}{{w1id, "returned_in_minor", 0}, {w2id, "returned_in_minor", 125000}} {
			status, raw, _ := g.call(http.MethodGet, "/v1/admin/stores/"+m.f.storeA1+"/settlements/"+tc.id, m.f.tokens["a"], nil, "")
			if status != 200 {
				t.Fatalf("merchant read %s: %d %s", tc.id, status, raw)
			}
			var body any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("merchant body: %v", err)
			}
			if got := gateNum(t, body, tc.key); got != tc.want {
				t.Fatalf("%s = %d, want %d (%s)", tc.key, got, tc.want, raw)
			}
			if tc.id == w1id {
				if v, ok := gateFind(body, "payout_returned"); !ok || v != true {
					t.Fatalf("W1 payout_returned = %v (%v)", v, ok)
				}
				if _, ok := gateFind(body, "payout_returned_at"); !ok {
					t.Fatal("W1 payout_returned_at is absent")
				}
			}
			for _, bad := range []string{"return_ref", "BANKRET-0001", "ticket", "HKD", "txn_", "settle_", "recorded_by"} {
				if strings.Contains(string(raw), bad) {
					t.Fatalf("merchant body leaks %q: %s", bad, raw)
				}
			}
		}
	})

	t.Run("PF16j_immutability", func(t *testing.T) {
		if _, err := g.f.owner.Exec(context.Background(), `UPDATE payments.`+tbl+` SET returned_minor=returned_minor+1`); gateSQLState(err) != "PT409" ||
			!strings.Contains(fmt.Sprint(err), "settlement_return_immutable") {
			t.Fatalf("UPDATE of returned_minor: %v", err)
		}
		if _, err := g.f.owner.Exec(context.Background(), `DELETE FROM payments.`+tbl); gateSQLState(err) != "PT409" ||
			!strings.Contains(fmt.Sprint(err), "settlement_return_immutable") {
			t.Fatalf("DELETE of a return: %v", err)
		}
		if n := rows(); n != 1 {
			t.Fatalf("return rows after refused mutations = %d, want 1", n)
		}
	})
}

// gatePF17Store builds the §6.9 PF17 fixture: a 100000 TWD capture and its 20000 TWD refund, so the MOCK list's A/B/C
// attribute through real SQL to real payment facts.
func (g *gateEnv) pf17Store(t *testing.T) (psHarness, sstStore, rfxOrder, string) {
	t.Helper()
	m, s, o := g.gateStore(t, 50000)
	refund := g.mustRefund(t, o, 20000, "requested_by_customer")
	g.awaitRefundFact(t, refund, o.attempt, "SUCCEEDED")
	return m, s, o, g.refundID(t, refund)
}

func TestPlatformSettlementPF17Reconciliation(t *testing.T) {
	g := gateNew(t)
	// §6.9 says the old nine-argument entry point is retired and the ten-argument one carries the manifest; a baseline that
	// still has only nine arguments is a contract failure, not a skipped test.
	g.fn(t, "record_settlement_lines", 10)
	w, u := gateWeek(2026, 9, 7), gateWeek(2026, 8, 31)
	at := func(d, hour int) time.Time {
		return w.Add(time.Duration(d)*24*time.Hour + time.Duration(hour)*time.Hour)
	}
	m, _, o, refund := g.pf17Store(t)
	_ = m
	a := gateCharge("txn_PF17A", o.pi, 100000, 10000, 400, "0.1", at(1, 12))
	b := gateRefund("txn_PF17B", refund, 20000, 2000, 0, "0.1", at(2, 12))
	c := gateRaw("txn_PF17C", "stripe_fee", -500, nil, at(3, 12))

	// U is empty and clean; W is the indispensable complement (both 7 days, inside the 8-day cap).
	g.sync(t, u, w, nil, nil)

	// a — clean coverage: r0 {A,C}, then a close probe inside a transaction that is rolled back.
	r0 := g.sync(t, w, w.AddDate(0, 0, 7), []stripe.BalanceTransaction{a, c}, []stripe.BalanceTransaction{a, c})
	r0id := g.latestRun(t)
	if got := g.reconOf(t, r0id); got.diff != 0 || got.stripe != 9100 || got.recorded != 9100 || got.stripeCount != 2 || got.recordedCount != 2 {
		t.Fatalf("r0 reconciliation %+v, want 0/9100/9100 counts 2/2 (return=%v)", got, r0)
	}
	if err := g.closeProbe(t, w.Format("2006-01-02")); err != nil {
		t.Fatalf("clean-coverage close probe: %v", err)
	}
	if n := g.count(t, `SELECT count(*) FROM payments.settlement_statements WHERE store_id=$1 AND period_start=$2::date`, o.store(), w.Format("2006-01-02")); n != 0 {
		t.Fatalf("the rolled-back probe left %d statements", n)
	}

	// a (continued) — U/r0 genuinely pre-02xx: strip r0's reconciliation rows (the migration cannot be un-applied per test)
	// and prove the grandfathered run is still treated as clean, not as a fabricated zero row.
	rt := g.recon(t)
	g.fixtureExec(t, `DELETE FROM payments.`+rt.table+` WHERE sync_run_id=$1`, r0id)
	if err := g.closeProbe(t, w.Format("2006-01-02")); err != nil {
		t.Fatalf("grandfathered clean coverage probe: %v", err)
	}

	// b — discrepant resync: r1 projects {A,C} but the full manifest is {A,B,C}.
	g.sync(t, w, w.AddDate(0, 0, 7), []stripe.BalanceTransaction{a, c}, []stripe.BalanceTransaction{a, b, c})
	r1id := g.latestRun(t)
	if got := g.reconOf(t, r1id); got.diff != 2000 || got.stripe != 7100 || got.recorded != 9100 || got.stripeCount != 3 || got.recordedCount != 2 {
		t.Fatalf("r1 reconciliation %+v, want +2000 7100/9100 counts 3/2", got)
	}
	if g.runOrder(t, r1id) <= g.runOrder(t, r0id) {
		t.Fatalf("r1 observation order %s is not newer than r0 %s", g.runOrder(t, r1id), g.runOrder(t, r0id))
	}
	if n := g.count(t, `SELECT count(*) FROM payments.`+rt.table+` WHERE sync_run_id=$1`, r0id); n != 0 {
		t.Fatalf("the grandfathered r0 gained %d reconciliation rows", n)
	}

	// c — refuse close: new dirty observation, not sync_required/period_not_closable/ACL.
	if err := g.closeProbe(t, w.Format("2006-01-02")); gateSQLState(err) != "PT409" || !strings.Contains(fmt.Sprint(err), "sync_unreconciled") {
		t.Fatalf("dirty resync close: %v", err)
	} else {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && !strings.Contains(pg.Detail, r1id) {
			t.Fatalf("sync_unreconciled DETAIL %q does not name r1 %s", pg.Detail, r1id)
		}
	}
	if n := g.count(t, `SELECT count(*) FROM payments.settlement_statements WHERE store_id=$1 AND period_start=$2::date`, o.store(), w.Format("2006-01-02")); n != 0 {
		t.Fatalf("a refused close wrote %d statements", n)
	}
	if n := g.count(t, `SELECT count(*) FROM payments.settlement_lines WHERE store_id=$1 AND statement_id IS NOT NULL`, o.store()); n != 0 {
		t.Fatalf("a refused close assigned %d lines", n)
	}
	if n := g.count(t, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.close'`); n != 0 {
		t.Fatalf("a refused close wrote %d close audit rows", n)
	}

	// d — clean repair: r2 {A,B,C} in both projections.
	g.sync(t, w, w.AddDate(0, 0, 7), []stripe.BalanceTransaction{a, b, c}, []stripe.BalanceTransaction{a, b, c})
	r2id := g.latestRun(t)
	if got := g.reconOf(t, r2id); got.diff != 0 || got.stripe != 7100 || got.recorded != 7100 || got.stripeCount != 3 || got.recordedCount != 3 {
		t.Fatalf("r2 reconciliation %+v, want 0 7100/7100 counts 3/3", got)
	}
	if g.runOrder(t, r2id) <= g.runOrder(t, r1id) {
		t.Fatalf("r2 observation order %s is not newer than r1 %s", g.runOrder(t, r2id), g.runOrder(t, r1id))
	}
	if got := g.reconOf(t, r1id); got.diff != 2000 {
		t.Fatalf("r1 changed after r2: %+v", got)
	}

	// e — successful close and the §6.9 totals.
	g.close(t, w.Format("2006-01-02"))
	wid, net, captured, refunded, fee := g.statement(t, o.store(), w.Format("2006-01-02"))
	if net != 76000 || captured != 100000 || refunded != 20000 || fee != -4000 {
		t.Fatalf("statement net=%d captured=%d refunded=%d fee=%d, want 76000/100000/20000/-4000", net, captured, refunded, fee)
	}
	if n := g.count(t, `SELECT count(*) FROM payments.settlement_lines WHERE balance_txn_id='txn_PF17B' AND statement_id=$1`, wid); n != 1 {
		t.Fatalf("B is assigned %d times, want exactly 1", n)
	}

	// f — retired nine-argument identity, ten-argument owner/ACL, NULL manifest and duplicate-id manifest with zero writes.
	t.Run("PF17f_entry_point", func(t *testing.T) {
		var old bool
		if err := g.f.owner.QueryRow(context.Background(), `SELECT to_regprocedure('payments.record_settlement_lines(uuid,uuid,uuid,text,jsonb,timestamptz,timestamptz,uuid,text)') IS NULL`).Scan(&old); err != nil || !old {
			t.Fatalf("old nine-argument record_settlement_lines still exists: %v", err)
		}
		oid := g.fn(t, "record_settlement_lines", 10)
		var secdef bool
		var owner, cfg string
		var acl []string
		if err := g.f.owner.QueryRow(context.Background(), `SELECT p.prosecdef,pg_get_userbyid(p.proowner),coalesce(p.proconfig::text,''),
			coalesce((SELECT array_agg(a.grantee::regrole::text ORDER BY a.grantee::regrole::text) FROM aclexplode(p.proacl) a
			WHERE a.privilege_type='EXECUTE'),'{}') FROM pg_proc p WHERE p.oid=$1::oid`, oid).Scan(&secdef, &owner, &cfg, &acl); err != nil {
			t.Fatalf("new function properties: %v", err)
		}
		if !secdef || owner != "commerce_payment_registry_writer" || !strings.Contains(cfg, "search_path=pg_catalog") {
			t.Fatalf("new function secdef=%v owner=%s cfg=%s", secdef, owner, cfg)
		}
		lcSameSet(t, "record_settlement_lines EXECUTE grantees", acl, []string{"commerce_payment_registrar", "commerce_payment_registry_writer"})

		before := g.gateWriteCounts(t)
		line, _ := json.Marshal([]stripe.BalanceTransaction{a})
		bad := []struct {
			name string
			val  any
		}{{"null-manifest", nil}, {"duplicate-id", string(gateJSON(t, gateManifest([]stripe.BalanceTransaction{a, a})))}}
		for _, tc := range bad {
			_, err := g.callFn(t, "record_settlement_lines", map[string]any{
				"tenant": g.tenant, "store": g.store, "principal": g.princ, "environment": "SANDBOX",
				"lines": string(line), "manifest": tc.val, "from": at(14, 0), "to": at(21, 0), "connection": g.plat.connection, "ticket": gateTicket})
			if gateSQLState(err) != "22023" || !strings.Contains(fmt.Sprint(err), "invalid settlement manifest") {
				t.Fatalf("%s: %v", tc.name, err)
			}
		}
		if after := g.gateWriteCounts(t); after != before {
			t.Fatalf("invalid manifests wrote rows: before=%v after=%v", before, after)
		}
	})

	// g — reconciliation immutability. difference_minor is GENERATED (§6.8), so PG refuses it before any trigger (428C9);
	// the trigger is exercised through a plain column.
	t.Run("PF17g_immutability", func(t *testing.T) {
		if _, err := g.f.owner.Exec(context.Background(), `UPDATE payments.`+rt.table+` SET `+rt.stripeNet+`=0 WHERE sync_run_id=$1`, r1id); gateSQLState(err) != "PT409" ||
			!strings.Contains(fmt.Sprint(err), "settlement_reconciliation_immutable") {
			t.Fatalf("UPDATE reconciliation: %v", err)
		}
		if _, err := g.f.owner.Exec(context.Background(), `DELETE FROM payments.`+rt.table+` WHERE sync_run_id=$1`, r1id); gateSQLState(err) != "PT409" ||
			!strings.Contains(fmt.Sprint(err), "settlement_reconciliation_immutable") {
			t.Fatalf("DELETE reconciliation: %v", err)
		}
		if got := g.reconOf(t, r1id); got.diff != 2000 {
			t.Fatalf("r1 changed after refused mutations: %+v", got)
		}
	})

	// Unmapped then attributed (txn_D): the anti-join must keep the recorded net/count at one row.
	t.Run("PF17_unmapped_then_attributed", func(t *testing.T) {
		_, _, dOrder := g.gateStore(t, 7500) // captured 15000
		d := gateCharge("txn_PF17D", "pi_gatePF17D", 15000, 1500, 60, "0.1", gateAt(2026, 9, 21, 12))
		from, to := gateAt(2026, 9, 21, 0), gateAt(2026, 9, 22, 0)
		g.sync(t, from, to, []stripe.BalanceTransaction{d}, []stripe.BalanceTransaction{d})
		run := g.latestRun(t)
		if got := g.reconOf(t, run); got.diff != 0 || got.stripe != 1440 || got.recorded != 1440 || got.stripeCount != 1 || got.recordedCount != 1 {
			t.Fatalf("D before attribution %+v, want 0 1440/1440 1/1", got)
		}
		if reason := g.count(t, `SELECT count(*) FROM payments.settlement_unattributed WHERE balance_txn_id='txn_PF17D' AND reason='unmapped_source'`); reason != 1 {
			t.Fatalf("D unattributed row = %d, want 1 unmapped_source", reason)
		}
		// The later payment mapping arrives on the already-paid session (owner-pool fixture, disclosed: the worker fact and
		// the provider id are recorded in production; here the sync must observe the same provider id in both directions).
		g.fixtureExec(t, `UPDATE payments.stripe_sessions SET payment_intent_id=$1 WHERE attempt_id=$2`, "pi_gatePF17D", dOrder.attempt)
		g.sync(t, from, to, []stripe.BalanceTransaction{d}, []stripe.BalanceTransaction{d})
		run2 := g.latestRun(t)
		got := g.reconOf(t, run2)
		if got.diff != 0 || got.recorded != 1440 || got.recordedCount != 1 {
			t.Fatalf("D after attribution %+v, want difference 0 recorded 1440 count 1 (anti-join)", got)
		}
		if n := g.count(t, `SELECT count(*) FROM payments.settlement_lines WHERE balance_txn_id='txn_PF17D'`); n != 1 {
			t.Fatalf("D lines = %d, want 1", n)
		}
		if n := g.count(t, `SELECT count(*) FROM payments.settlement_unattributed WHERE balance_txn_id='txn_PF17D'`); n != 1 {
			t.Fatalf("D unattributed history = %d, want 1 (never deleted)", n)
		}
	})

	// Unsupported NULL fee (txn_C alone, fresh id so the isolated window is not a replay).
	t.Run("PF17_unsupported_null_fee", func(t *testing.T) {
		c2 := gateRaw("txn_PF17C2", "stripe_fee", -500, nil, gateAt(2026, 9, 22, 12))
		from, to := gateAt(2026, 9, 22, 0), gateAt(2026, 9, 23, 0)
		g.sync(t, from, to, []stripe.BalanceTransaction{c2}, []stripe.BalanceTransaction{c2})
		run := g.latestRun(t)
		if got := g.reconOf(t, run); got.diff != 0 || got.stripe != -500 || got.recorded != -500 || got.stripeCount != 1 || got.recordedCount != 1 {
			t.Fatalf("C alone %+v, want 0 -500/-500 1/1 (NULL or zero recorded net is a failure)", got)
		}
		var feeNull bool
		var net int64
		if err := g.f.owner.QueryRow(context.Background(), `SELECT settle_fee IS NULL,settle_net FROM payments.settlement_unattributed WHERE balance_txn_id='txn_PF17C2'`).Scan(&feeNull, &net); err != nil {
			t.Fatal(err)
		}
		if !feeNull || net != -500 {
			t.Fatalf("C2 settle_fee NULL=%v settle_net=%d, want NULL/-500", feeNull, net)
		}
	})

	// Overlap supersession in its own database: a wider legacy clean run cannot mask r1, and a partial clean repair cannot
	// erase r1 over the remaining dirty segment.
	t.Run("PF17_overlap_supersession", func(t *testing.T) {
		og := gateNew(t)
		ow := gateWeek(2026, 9, 7)
		ou := gateWeek(2026, 8, 31)
		om, _, oo, oref := og.pf17Store(t)
		_ = om
		oa := gateCharge("txn_PF17OA", oo.pi, 100000, 10000, 400, "0.1", ow.Add(24*time.Hour+12*time.Hour))
		ob := gateRefund("txn_PF17OB", oref, 20000, 2000, 0, "0.1", ow.Add(2*24*time.Hour+12*time.Hour))
		oc := gateRaw("txn_PF17OC", "stripe_fee", -500, nil, ow.Add(3*24*time.Hour+12*time.Hour))
		ox := gateRaw("txn_PF17OX", "stripe_fee", 1000, nil, ow.Add(4*24*time.Hour+12*time.Hour)) // 09-11, only ever in r1's manifest
		og.sync(t, ou, ow, nil, nil)
		// wider clean legacy window [09-06,09-14) (8 days), then strip its reconciliation rows (pre-02xx).
		og.sync(t, ow.AddDate(0, 0, -1), ow.AddDate(0, 0, 7), []stripe.BalanceTransaction{oa, ob, oc}, []stripe.BalanceTransaction{oa, ob, oc})
		wide := og.latestRun(t)
		ort := og.recon(t)
		og.fixtureExec(t, `DELETE FROM payments.`+ort.table+` WHERE sync_run_id=$1`, wide)
		// r1 on W drops the 09-11 object from the projection but observes it in the manifest: dirty and newer than the
		// legacy wide run.
		og.sync(t, ow, ow.AddDate(0, 0, 7), []stripe.BalanceTransaction{oa, oc}, []stripe.BalanceTransaction{oa, ob, oc, ox})
		if err := og.closeProbe(t, ow.Format("2006-01-02")); gateSQLState(err) != "PT409" || !strings.Contains(fmt.Sprint(err), "sync_unreconciled") {
			t.Fatalf("wide legacy clean run masked r1: %v", err)
		}
		// a clean run over only [09-07,09-10) leaves [09-10,09-14) dirty.
		og.sync(t, ow, gateAt(2026, 9, 10, 0), []stripe.BalanceTransaction{oa, ob}, []stripe.BalanceTransaction{oa, ob})
		if err := og.closeProbe(t, ow.Format("2006-01-02")); gateSQLState(err) != "PT409" || !strings.Contains(fmt.Sprint(err), "sync_unreconciled") {
			t.Fatalf("partial clean repair erased the remaining dirty segment: %v", err)
		}
		// clean repair of the remaining segment closes.
		og.sync(t, gateAt(2026, 9, 10, 0), ow.AddDate(0, 0, 7), []stripe.BalanceTransaction{oc, ox}, []stripe.BalanceTransaction{oc, ox})
		if err := og.closeProbe(t, ow.Format("2006-01-02")); err != nil {
			t.Fatalf("full clean repair still refuses: %v", err)
		}
	})
}

func gateJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// gateWriteCounts fingerprints the five write surfaces a rejected manifest must not touch.
func (g *gateEnv) gateWriteCounts(t *testing.T) string {
	t.Helper()
	rt := g.recon(t)
	return fmt.Sprintf("lines=%d unattributed=%d runs=%d recon=%d audits=%d",
		g.count(t, `SELECT count(*) FROM payments.settlement_lines`),
		g.count(t, `SELECT count(*) FROM payments.settlement_unattributed`),
		g.count(t, `SELECT count(*) FROM payments.settlement_sync_runs`),
		g.count(t, `SELECT count(*) FROM payments.`+rt.table),
		g.count(t, `SELECT count(*) FROM ops.audit_events WHERE action='stripe.settlement.sync'`))
}

func TestPlatformSettlementPF18Payables(t *testing.T) {
	g := gateNew(t)
	g.fn(t, "read_settlement_payables", 5)
	ctx := context.Background()
	p := g.payables(t)
	if len(p) < 4 {
		t.Fatalf("payables fixture stores = %d", len(p))
	}

	// §6.9 PF18: four closed SANDBOX statements; A closes first, D last, B net 0 and C net -3000 are not payable.
	base := time.Now().Add(-4 * time.Hour).Truncate(time.Second)
	insert := func(name string, net int64, closed time.Time) string {
		t.Helper()
		st := p[name]
		return g.insertStatement(t, st.tenant, st.store, net, "2026-09-07", closed)
	}
	aID := insert("A", 80000, base)
	insert("B", 0, base.Add(30*time.Second))
	insert("C", -3000, base.Add(60*time.Second))
	dID := insert("D", 50000, base.Add(90*time.Second))

	read := func(environment string, limit int) ([]map[string]any, bool, error) {
		out, err := g.callFn(t, "read_settlement_payables", map[string]any{
			"tenant": g.tenant, "store": g.store, "principal": g.princ, "environment": environment, "limit": int64(limit)})
		if err != nil {
			return nil, false, err
		}
		raw, ok := out["payables"].([]any)
		if !ok {
			return nil, false, fmt.Errorf("read_settlement_payables returned %v", out)
		}
		list := make([]map[string]any, 0, len(raw))
		for _, v := range raw {
			entry, ok := v.(map[string]any)
			if !ok {
				return nil, false, fmt.Errorf("payable entry is %T", v)
			}
			list = append(list, entry)
		}
		truncated, _ := out["truncated"].(bool)
		return list, truncated, nil
	}

	t.Run("PF18a_order_and_shape", func(t *testing.T) {
		list, truncated, err := read("SANDBOX", 50)
		if err != nil {
			t.Fatal(err)
		}
		if truncated {
			t.Fatal("truncated=true for 2 of 50")
		}
		if len(list) != 2 || list[0]["statement_id"] != aID || list[1]["statement_id"] != dID {
			t.Fatalf("payables = %v, want [A D]", list)
		}
		for _, entry := range list {
			if len(entry) != 7 {
				t.Fatalf("entry has %d keys, want exactly 7: %v", len(entry), entry)
			}
			for _, k := range []string{"statement_id", "tenant_id", "store_id", "period_start", "currency", "net_payable_minor", "closed_at"} {
				if _, ok := entry[k]; !ok {
					t.Fatalf("entry is missing %q: %v", k, entry)
				}
			}
			if entry["currency"] != "TWD" {
				t.Fatalf("currency = %v", entry["currency"])
			}
		}
	})

	t.Run("PF18b_truncation", func(t *testing.T) {
		list, truncated, err := read("SANDBOX", 1)
		if err != nil {
			t.Fatal(err)
		}
		if !truncated || len(list) != 1 || list[0]["statement_id"] != aID {
			t.Fatalf("limit 1 = %v truncated=%v, want [A] true", list, truncated)
		}
	})

	t.Run("PF18c_paid_disappears", func(t *testing.T) {
		g.mustCall(t, "record A payout", "record_settlement_payout", map[string]any{
			"tenant": g.tenant, "store": g.store, "principal": g.princ, "statement": aID,
			"payout_ref": "BANK-PF18-A", "payout_minor": int64(80000), "paid_at": time.Now().Add(-time.Hour).Truncate(time.Second),
			"operator": "op@test", "ticket": gateTicket})
		list, _, err := read("SANDBOX", 50)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0]["statement_id"] != dID {
			t.Fatalf("after paying A the list = %v, want [D] (payout_ref IS NULL)", list)
		}
	})

	t.Run("PF18d_bounds", func(t *testing.T) {
		for _, tc := range []struct {
			env   string
			limit int
		}{{"SANDBOX", 0}, {"SANDBOX", 201}, {"PROD", 50}} {
			_, _, err := read(tc.env, tc.limit)
			if gateSQLState(err) != "22023" || !strings.Contains(fmt.Sprint(err), "invalid settlement payables") {
				t.Fatalf("%s/%d: %v", tc.env, tc.limit, err)
			}
		}
	})

	t.Run("PF18e_acl", func(t *testing.T) {
		for _, role := range []string{"commerce_runtime", "commerce_payment_worker", "commerce_worker"} {
			var granted bool
			if err := g.f.owner.QueryRow(ctx, `SELECT has_function_privilege($1::text,'payments.read_settlement_payables(uuid,uuid,uuid,text,integer)','EXECUTE')`, role).Scan(&granted); err != nil {
				t.Fatalf("%s privilege: %v", role, err)
			}
			if granted {
				t.Fatalf("%s has EXECUTE on read_settlement_payables", role)
			}
			tx, err := g.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize()); err != nil {
				tx.Rollback(ctx)
				t.Fatalf("SET ROLE %s: %v", role, err)
			}
			_, err = tx.Exec(ctx, `SELECT count(*) FROM payments.read_settlement_payables($1,$2,$3,'SANDBOX',10)`, g.tenant, g.store, g.princ)
			tx.Rollback(ctx)
			if gateSQLState(err) != "42501" {
				t.Fatalf("%s EXECUTE: %v", role, err)
			}
		}
	})

	t.Run("PF18f_live_empty", func(t *testing.T) {
		list, truncated, err := read("LIVE", 50)
		if err != nil {
			t.Fatalf("LIVE read: %v", err)
		}
		if len(list) != 0 || truncated {
			t.Fatalf("LIVE payables = %v truncated=%v, want [] false", list, truncated)
		}
	})
}

type payStore struct{ tenant, store string }

func (g *gateEnv) payables(t *testing.T) map[string]payStore {
	t.Helper()
	out := map[string]payStore{}
	for _, name := range []string{"A", "B", "C", "D"} {
		m := g.merchant(t, true)
		out[name] = payStore{tenant: m.f.tenantA, store: m.f.storeA1}
	}
	return out
}
