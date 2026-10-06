// Purpose: REAL_PG gates of LC-B7 (live-console-v1 §7.1 A1 console read model, §7.2 bounded live stock edit): the closed A1 body over a real claims/order/stock fixture, its authority (live:read only, cross-store/tenant → 404), the stream/capability cells, sales that agree with identity.read_live_session_results, and the inventory:live_adjust bounds (LCN03) through the real merchant router.
// Depends on: ltgNew/tcv harness (live_tools_gate_test.go, taiwan_cvs_env_test.go), lcnEnv fake Graph + metareply bridge (live_console_comments_test.go), internal/live Console/Results/Lifecycle, httpapi.NewHandler via the ltg merchant handler, migration 0148.
// Used by: bash scripts/dev/test-local.sh --live-console (TestLiveConsoleLCN regex), test-focused.sh.
// Invariants: I01 (cross-scope 404), I03 (live operator cannot silently rewrite stock truth), I05 (SANDBOX money is never "paid"), I11 (no comment text in the A1 body).
// Status: MOCK (fake Graph, sandbox payments); owner-pool writes are disclosed where used (collection_state fixture).
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
	"livecommerce/internal/metaconnect"
	"livecommerce/internal/platform"
)

// lcbConsole GETs A1 through the real merchant router and decodes the typed body (also returning the raw bytes).
func lcbConsole(t *testing.T, e *ltgEnv, token, session string) (int, live.ConsoleSnapshot, []byte) {
	t.Helper()
	status, _, raw := e.mcall(token, "GET", "/v1/admin/stores/"+e.store()+"/live-sessions/"+session+"/console", "", "")
	var snap live.ConsoleSnapshot
	if status == http.StatusOK {
		if err := json.Unmarshal(raw, &snap); err != nil {
			t.Fatalf("A1 body: %v %s", err, raw)
		}
	}
	return status, snap, raw
}

func lcbMustConsole(t *testing.T, e *ltgEnv, session string) live.ConsoleSnapshot {
	t.Helper()
	status, snap, raw := lcbConsole(t, e, e.token(), session)
	if status != http.StatusOK {
		t.Fatalf("A1 answered %d: %s", status, raw)
	}
	return snap
}

func lcbKeys(t *testing.T, raw []byte, path ...string) []string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	for _, p := range path {
		if list, ok := v.([]any); ok {
			v = list[0]
		}
		v = v.(map[string]any)[p]
	}
	if list, ok := v.([]any); ok {
		v = list[0]
	}
	var keys []string
	for k := range v.(map[string]any) {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func lcbSameKeys(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s keys %v, want %v", what, got, want)
	}
}

// lcbResults is live.Results for one session as the orders:read merchant (the A5-1 read model A1's sales must agree with).
func lcbResults(t *testing.T, e *ltgEnv, session string) live.SessionResult {
	t.Helper()
	var out live.SessionResults
	err := platform.WithScope(context.Background(), e.p.f.runtime, e.token(), e.store(), "live:read", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = live.Results(context.Background(), tx, s, e.token(), []string{session})
		return err
	})
	if err != nil || len(out.Items) != 1 {
		t.Fatalf("Results: %v %+v", err, out)
	}
	return out.Items[0]
}

// TestLiveConsoleLCN17ReadModelStockClaimsSales walks A1 over a real session: empty, claimed, ordered (live-price use ledger) and paid
// (owner-pool fixture: pay_at_pickup COLLECTED, the LIVE-environment path of the money口径), checking the closed key set, stock, claims, sales,
// agreement with live.Results, recommended and lifecycle.
func TestLiveConsoleLCN17ReadModelStockClaimsSales(t *testing.T) {
	e := ltgNew(t)
	s, o := e.session("A1", ltgLive, 5)

	status, _, raw := lcbConsole(t, e, e.token(), s)
	if status != http.StatusOK {
		t.Fatalf("A1: %d %s", status, raw)
	}
	lcbSameKeys(t, "A1 top", lcbKeys(t, raw), "session", "window", "stats", "offers", "capabilities", "stream", "recommended")
	lcbSameKeys(t, "session", lcbKeys(t, raw, "session"), "id", "title", "lifecycle", "version", "started_at", "ended_at")
	lcbSameKeys(t, "window", lcbKeys(t, raw, "window"), "state", "generation", "opened_at", "match_mode")
	lcbSameKeys(t, "stats", lcbKeys(t, raw, "stats"), "comments", "keyword_comments", "buyers", "orders", "paid", "currency", "as_of")
	lcbSameKeys(t, "offer", lcbKeys(t, raw, "offers"), "offer_id", "keyword", "sku_id", "product_name", "variant_label", "active", "version",
		"live_price_minor", "sku_price_minor", "stock", "claimed", "ordered_qty", "paid_qty", "paid_amount_minor", "sold_out", "low_stock")
	lcbSameKeys(t, "stock", lcbKeys(t, raw, "offers", "stock"), "tracked", "sellable", "reserved", "warehouse_id", "balance_version")
	if strings.Contains(string(raw), "text") || strings.Contains(string(raw), "author") {
		t.Fatalf("A1 body carries comment fields: %s", raw)
	}
	snap := lcbMustConsole(t, e, s)
	// Opening the claim window already moved the session to lifecycle live (the fixture opens it).
	if snap.Session.ID != s || snap.Session.Lifecycle != "live" || snap.Session.Version < 1 || snap.Session.StartedAt == nil || snap.Session.EndedAt != nil || snap.Window.State != "OPEN" || snap.Window.MatchMode != "EXACT" {
		t.Fatalf("session/window %+v %+v", snap.Session, snap.Window)
	}
	if len(snap.Offers) != 1 {
		t.Fatalf("offers %+v", snap.Offers)
	}
	off := snap.Offers[0]
	if off.OfferID != o.ID || off.Keyword != "A1" || off.SKUID != e.money || off.LivePriceMinor == nil || *off.LivePriceMinor != ltgLive || off.SKUPriceMinor != ltgCatalog ||
		!off.Stock.Tracked || off.Stock.Sellable != 500 || off.Stock.Reserved != 0 || off.Stock.WarehouseID == nil || *off.Stock.WarehouseID != e.p.stock.warehouse.ID ||
		off.Stock.BalanceVersion < 1 || off.Claimed.Buyers != 0 || off.Claimed.Quantity != 0 || off.OrderedQty != 0 || off.SoldOut || off.LowStock {
		t.Fatalf("empty offer %+v", off)
	}
	if snap.Stats.Comments.Source != "unavailable" || snap.Stats.Comments.Total != nil || snap.Stats.KeywordComments != 0 || snap.Stats.Buyers != 0 ||
		snap.Stats.Orders.Count != 0 || snap.Stats.Paid.Count != 0 || len(snap.Stats.Currency) != 3 || snap.Stats.AsOf.IsZero() {
		t.Fatalf("empty stats %+v", snap.Stats)
	}
	if snap.Stream.State != "unavailable" || snap.Stream.Reason != "bridge_disabled" || len(snap.Capabilities) != 0 || snap.Recommended != nil {
		t.Fatalf("stream/capabilities/recommended %+v %+v %+v", snap.Stream, snap.Capabilities, snap.Recommended)
	}

	// A claim, then a real order at the live price (checkout.Begin writes claims.live_price_uses and holds 2 units).
	bundle, l := e.claimLink(s, "amy", "A1+2")
	b := e.redeemed(l)
	q, ln := e.line(b, "")
	e.wantLive("quote", ln, ltgLive, bundle, o.ID)
	hold, err := e.placeHome(b, q)
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	total := e.total(hold.OrderID)
	snap = lcbMustConsole(t, e, s)
	off = snap.Offers[0]
	if off.Claimed.Buyers != 1 || off.Claimed.Quantity != 2 || off.OrderedQty != 2 || off.PaidQty != 0 || off.PaidAmount != 0 || off.Stock.Reserved != 2 || off.Stock.Sellable != 498 {
		t.Fatalf("ordered offer %+v", off)
	}
	if snap.Stats.KeywordComments != 1 || snap.Stats.Buyers != 1 || snap.Stats.Orders.Count != 1 || snap.Stats.Orders.AmountMinor != total || snap.Stats.Paid.Count != 0 || snap.Stats.Paid.AmountMinor != 0 {
		t.Fatalf("ordered stats %+v (order total %d)", snap.Stats, total)
	}
	res := lcbResults(t, e, s)
	if res.Orders != 1 || len(res.Money) != 1 || res.Money[0].OrderMinor != snap.Stats.Orders.AmountMinor || res.Money[0].Currency != snap.Stats.Currency || res.PaidOrders != 0 {
		t.Fatalf("A1 orders disagree with live.Results: %+v vs %+v", snap.Stats, res)
	}

	// Paid: LIVE-environment collected money (disclosed owner-pool fixture; the card path is SANDBOX here and is covered by the next test).
	mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET payment_mode='pay_at_pickup',collection_state='COLLECTED' WHERE id=$1`, hold.OrderID)
	lineTotal := int64(e.count(`SELECT (snapshot#>>'{quote,lines,0,amount,total_minor}')::bigint FROM checkout.orders WHERE id=$1`, hold.OrderID))
	snap = lcbMustConsole(t, e, s)
	off = snap.Offers[0]
	if snap.Stats.Paid.Count != 1 || snap.Stats.Paid.AmountMinor != total || off.PaidQty != 2 || off.PaidAmount != lineTotal || off.OrderedQty != 2 {
		t.Fatalf("paid: stats %+v offer %+v (order total %d line %d)", snap.Stats, off, total, lineTotal)
	}
	res = lcbResults(t, e, s)
	if res.PaidOrders != 1 || res.Money[0].PaidMinor != snap.Stats.Paid.AmountMinor || res.Money[0].PaidMinor != total {
		t.Fatalf("A1 paid disagrees with live.Results: %+v vs %+v", snap.Stats, res)
	}

	// Recommended (§7.3 timeline) and lifecycle (A7).
	if err := platform.WithScope(context.Background(), e.p.f.runtime, e.token(), e.store(), "live:manage", func(tx pgx.Tx, sc platform.Scope) error {
		_, err := live.RecordOfferFeatured(context.Background(), tx, sc, e.token(), s, o.ID)
		return err
	}); err != nil {
		t.Fatalf("featured: %v", err)
	}
	snap = lcbMustConsole(t, e, s)
	if snap.Recommended == nil || snap.Recommended.OfferID != o.ID || snap.Recommended.At.IsZero() {
		t.Fatalf("recommended %+v", snap.Recommended)
	}
	version := snap.Session.Version
	if _, err := e.h.k3Lifecycle(e.token(), e.store(), t04Key("lcb-end"), s, live.LifecycleInput{Action: "end", ExpectedVersion: version}); err != nil {
		t.Fatalf("end: %v", err)
	}
	snap = lcbMustConsole(t, e, s)
	if snap.Session.Lifecycle != "ended" || snap.Session.Version != version+1 || snap.Session.EndedAt == nil || snap.Window.State != "CLOSED" {
		t.Fatalf("ended session %+v window %+v", snap.Session, snap.Window)
	}

	// Capabilities: the reader's rows are embedded (first row per provider/capability); a failing reader degrades to {} without
	// aborting the transaction (the console still answers).
	readWith := func(c *live.Console) live.ConsoleSnapshot {
		var out live.ConsoleSnapshot
		err := platform.WithScope(context.Background(), e.p.f.runtime, e.token(), e.store(), "live:read", func(tx pgx.Tx, sc platform.Scope) error {
			var err error
			out, err = c.Read(context.Background(), tx, sc, e.token(), s)
			return err
		})
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		return out
	}
	checked := time.Now().UTC().Truncate(time.Second)
	got := readWith(live.NewConsole(nil, lcbCaps{rows: []metaconnect.CapabilityState{
		{Provider: "facebook", Capability: "read_comment", State: "ok", Evidence: "MOCK", CheckedAt: checked},
		{Provider: "facebook", Capability: "read_comment", State: "missing_task", Reason: "ignored_duplicate", Evidence: "DESIGN"},
		{Provider: "instagram", Capability: "dm_session", State: "review_required", Reason: "standard_access", Evidence: "DESIGN"}}}))
	fb := got.Capabilities["facebook"]["read_comment"]
	ig := got.Capabilities["instagram"]["dm_session"]
	if fb.State != "ok" || fb.Evidence != "MOCK" || fb.CheckedAt == nil || !fb.CheckedAt.Equal(checked) || ig.State != "review_required" || ig.CheckedAt != nil || len(got.Capabilities["facebook"]) != 1 {
		t.Fatalf("capabilities %+v", got.Capabilities)
	}
	if got := readWith(live.NewConsole(nil, lcbCaps{err: errors.New("boom")})); len(got.Capabilities) != 0 || got.Session.ID != s {
		t.Fatalf("failing reader did not degrade: %+v", got)
	}
}

type lcbCaps struct {
	rows []metaconnect.CapabilityState
	err  error
}

func (c lcbCaps) Capabilities(ctx context.Context, tx pgx.Tx, _ platform.Scope, _ string) ([]metaconnect.CapabilityState, error) {
	if c.err != nil {
		_, _ = tx.Exec(ctx, `SELECT 1/0`) // a failed statement inside the console transaction: the savepoint must contain it
		return nil, c.err
	}
	return c.rows, nil
}

// TestLiveConsoleLCN17SandboxPaidNotCounted: a card order paid through the real (SANDBOX) capture path is an order but never "paid" in A1 (I05), while
// live.Results reports the same money as sandbox_paid_minor.
func TestLiveConsoleLCN17SandboxPaidNotCounted(t *testing.T) {
	e := ltgNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	s, o := e.session("A1", ltgLive, 5)
	bundle, l := e.claimLink(s, "amy", "A1+2")
	b := e.redeemed(l)
	q, ln := e.line(b, "")
	e.wantLive("quote", ln, ltgLive, bundle, o.ID)
	hold, err := e.placeHome(b, q)
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	if paid := e.payHold(hold, b); paid.captured != q.Amount.TotalMinor {
		t.Fatalf("captured %d want %d", paid.captured, q.Amount.TotalMinor)
	}
	snap := lcbMustConsole(t, e, s)
	if snap.Stats.Orders.Count != 1 || snap.Stats.Paid.Count != 0 || snap.Stats.Paid.AmountMinor != 0 || snap.Offers[0].PaidQty != 0 || snap.Offers[0].OrderedQty != 2 {
		t.Fatalf("sandbox-paid order must be ordered, not paid: %+v / %+v", snap.Stats, snap.Offers[0])
	}
	if res := lcbResults(t, e, s); res.PaidOrders != 1 || res.Money[0].SandboxPaidMinor != q.Amount.TotalMinor || res.Money[0].PaidMinor != 0 {
		t.Fatalf("live.Results should show the same money as sandbox: %+v", res)
	}
}

// TestLiveConsoleLCN03ConsoleAuthority: A1 needs live:read only (no orders:read), refuses a principal without it, and answers 404 for a session of
// another store, another tenant's token, an unknown session, and an unauthenticated request 401.
func TestLiveConsoleLCN03ConsoleAuthority(t *testing.T) {
	e := ltgNew(t)
	f := e.p.f
	s, _ := e.session("A1", ltgLive, 5)
	_, readOnly := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "live:read")
	_, noLive := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "orders:read")
	for _, tc := range []struct {
		name, token, session string
		want                 int
	}{
		{"live:read only", readOnly, s, 200},
		{"no live:read", noLive, s, 403},
		{"other tenant token", f.tokens["b"], s, 401},
		{"unknown session", e.token(), randomUUID(), 404},
		{"no token", "", s, 401},
	} {
		if status, _, raw := lcbConsole(t, e, tc.token, tc.session); status != tc.want {
			t.Errorf("%s: %d %s, want %d", tc.name, status, raw, tc.want)
		}
	}
	// A session of another tenant (and its store) is 404 under this store's path, and this store's session is 404 under the other store's path (I01).
	_, otherTok := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "live:read", "live:manage")
	foreign := e.h.draftAs(t, otherTok, f.storeB)
	if status, _, raw := lcbConsole(t, e, e.token(), foreign); status != 404 {
		t.Errorf("foreign-tenant session under this store: %d %s, want 404", status, raw)
	}
	if status, _, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+f.storeB+"/live-sessions/"+s+"/console", "", ""); status != 404 {
		t.Errorf("this session under the other tenant's store: %d %s, want 404", status, raw)
	}
	// The live:read-only principal still sees the sales numbers (A1 is live:read, not orders:read).
	snap := func() live.ConsoleSnapshot { _, v, _ := lcbConsole(t, e, readOnly, s); return v }()
	if snap.Session.ID != s || len(snap.Offers) != 1 {
		t.Fatalf("live:read-only snapshot %+v", snap)
	}
}

// TestLiveConsoleLCN17StreamStateAndTotal: with the bridge wired, a polled Facebook source reports the stream state and the unique-comments-seen
// total (stream_seen); a console without a stream reports unavailable. The body never carries comment text.
func TestLiveConsoleLCN17StreamStateAndTotal(t *testing.T) {
	e := lcnSetup(t)
	now := time.Now().UTC()
	ref1, ref2 := lcnRef(), lcnRef()
	text := "LCN17-sentinel-text-" + t04Tag()
	e.graph.setComments(e.postID, []map[string]any{
		lcnComment(ref1, now.Format(time.RFC3339), e.asset, "", text, "", false),
		lcnComment(ref2, now.Add(-time.Second).Format(time.RFC3339), "9"+mciDigits(10), "Buyer", text, "", false)})
	c := e.console(t, "worker-console-a1", metareply.ConsoleConfig{})
	srv := httptest.NewServer(c.Handler())
	t.Cleanup(srv.Close)
	bridge, err := metareply.NewBridgeClient(srv.URL, e.bridgeToken)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := live.NewCommentStream(bridge, nil)
	if err != nil {
		t.Fatal(err)
	}
	if page := e.demandAndPoll(t, c, e.sourceID); len(page.Items) != 2 {
		t.Fatalf("polled %d comments", len(page.Items))
	}
	read := func(console *live.Console) (live.ConsoleSnapshot, []byte) {
		var out live.ConsoleSnapshot
		err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, sc platform.Scope) error {
			var err error
			out, err = console.Read(context.Background(), tx, sc, e.h.token, e.session)
			return err
		})
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		raw, _ := json.Marshal(out)
		return out, raw
	}
	snap, raw := read(live.NewConsole(stream, nil))
	if snap.Stats.Comments.Source != "stream_seen" || snap.Stats.Comments.Total == nil || *snap.Stats.Comments.Total != 2 || snap.Stream.State != "live" || snap.Stream.SourcePlatform != "facebook" || !snap.Stream.VideoEmbeddable {
		t.Fatalf("stream cells %+v %+v", snap.Stats.Comments, snap.Stream)
	}
	if strings.Contains(string(raw), text) {
		t.Fatalf("A1 body leaked comment text")
	}
	if snap, _ := read(live.NewConsole(nil, nil)); snap.Stats.Comments.Source != "unavailable" || snap.Stream.State != "unavailable" || snap.Stream.SourcePlatform != "facebook" {
		t.Fatalf("console without a stream %+v %+v", snap.Stats.Comments, snap.Stream)
	}
	// A session with no claim source: not_started / no_source, total unavailable.
	bare := e.h.draft(t, e.f.storeA1)
	var out live.ConsoleSnapshot
	if err := platform.WithScope(context.Background(), e.f.runtime, e.h.token, e.f.storeA1, "live:read", func(tx pgx.Tx, sc platform.Scope) error {
		var err error
		out, err = live.NewConsole(stream, nil).Read(context.Background(), tx, sc, e.h.token, bare)
		return err
	}); err != nil {
		t.Fatalf("Read bare: %v", err)
	}
	if out.Stream.State != "not_started" || out.Stream.Reason != "no_source" || out.Stats.Comments.Source != "unavailable" {
		t.Fatalf("no-source session %+v %+v", out.Stream, out.Stats.Comments)
	}
}

// lcbAdjust POSTs one stock adjustment through the real router.
func lcbAdjust(e *ltgEnv, token, sku, warehouse string, delta, expected int64, reason string) (int, map[string]any, string) {
	body, _ := json.Marshal(map[string]any{"warehouse_id": warehouse, "sku_id": sku, "delta": delta, "expected_version": expected, "reason": reason})
	key := t04Key("lcb-adj")
	status, m, _ := e.mcall(token, "POST", "/v1/admin/stores/"+e.store()+"/inventory/adjustments", key, string(body))
	return status, m, key
}

// TestLiveConsoleLCN03InventoryLiveAdjustBounds (live-console-v1 §7.2): a principal holding inventory:live_adjust but not inventory:write can edit stock
// through POST /inventory/adjustments, bounded — the reason is forced to live_console_edit, |delta| ≤ 1000, never below reserved+allocated(+unavailable)
// (422 below_reserved), the audit row carries the reason — while an inventory:write principal (also one holding both) keeps the unbounded edit, a principal
// with neither is 403, and another tenant is 404. The balance/version come from A1's stock cell, exactly as the console UI drives it.
func TestLiveConsoleLCN03InventoryLiveAdjustBounds(t *testing.T) {
	e := ltgNew(t)
	f := e.p.f
	s, o := e.session("A1", ltgLive, 5)
	bundle, l := e.claimLink(s, "amy", "A1+2")
	b := e.redeemed(l)
	q, ln := e.line(b, "")
	e.wantLive("quote", ln, ltgLive, bundle, o.ID)
	if _, err := e.placeHome(b, q); err != nil { // holds 2 units: reserved=2
		t.Fatalf("hold: %v", err)
	}
	stock := func() live.ConsoleStock { return lcbMustConsole(t, e, s).Offers[0].Stock }
	st := stock()
	if st.WarehouseID == nil {
		t.Fatalf("no warehouse in the stock cell: %+v", st)
	}
	wh := *st.WarehouseID

	liveAdj, liveTok := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inventory:live_adjust")
	_, writeTok := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inventory:write")
	_, bothTok := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inventory:write", "inventory:live_adjust")
	_, noneTok := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inventory:read")
	ledgerReason := func(key string) string {
		var reason string
		if err := f.owner.QueryRow(context.Background(), `SELECT reason FROM inventory.ledger WHERE sku_id=$1 AND command_key=$2`, e.money, key).Scan(&reason); err != nil {
			t.Fatalf("ledger row for %s: %v", key, err)
		}
		return reason
	}

	// 1. In bounds: 200, reason forced whatever the client sent, audit carries it.
	status, body, key := lcbAdjust(e, liveTok, e.money, wh, 5, st.BalanceVersion, "client-chosen-lie")
	if status != 200 || int64(body["on_hand"].(float64)) != 505 {
		t.Fatalf("bounded +5: %d %v", status, body)
	}
	if got := ledgerReason(key); got != "live_console_edit" {
		t.Fatalf("bounded reason %q, want live_console_edit", got)
	}
	if n := t04Count(t, f, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND action='inventory.adjusted' AND details->>'reason'='live_console_edit'`,
		f.tenantA, f.storeA1, liveAdj); n != 1 {
		t.Fatalf("audit rows with the reason: %d, want 1", n)
	}
	st = stock()
	if st.Sellable != 503 || st.Reserved != 2 { // 505 on hand, 2 reserved by the held order
		t.Fatalf("stock after +5: %+v", st)
	}

	// 2. |delta| bound: 1001 either way is 422 invalid_request and writes nothing.
	for _, d := range []int64{1001, -1001} {
		if status, body, _ := lcbAdjust(e, liveTok, e.money, wh, d, st.BalanceVersion, "x"); status != 422 || body["code"] != "invalid_request" {
			t.Fatalf("bounded delta %d: %d %v", d, status, body)
		}
	}
	// ...but exactly +1000 and -1000 pass (the bound is inclusive).
	if status, body, _ := lcbAdjust(e, liveTok, e.money, wh, 1000, st.BalanceVersion, "x"); status != 200 {
		t.Fatalf("bounded +1000: %d %v", status, body)
	}
	st = stock()
	if status, body, _ := lcbAdjust(e, liveTok, e.money, wh, -1000, st.BalanceVersion, "x"); status != 200 {
		t.Fatalf("bounded -1000: %d %v", status, body)
	}
	st = stock() // 505 on hand, 2 reserved, sellable 503

	// 3. Never below reserved: removing one more than sellable is 422 below_reserved; exactly sellable passes (sold out, never negative).
	if status, body, _ := lcbAdjust(e, liveTok, e.money, wh, -(st.Sellable + 1), st.BalanceVersion, "x"); status != 422 || body["code"] != "below_reserved" {
		t.Fatalf("below reserved: %d %v", status, body)
	}
	if status, body, _ := lcbAdjust(e, liveTok, e.money, wh, -st.Sellable, st.BalanceVersion, "x"); status != 200 {
		t.Fatalf("down to reserved: %d %v", status, body)
	}
	if snap := lcbMustConsole(t, e, s); snap.Offers[0].Stock.Sellable != 0 || snap.Offers[0].Stock.Reserved != 2 || !snap.Offers[0].SoldOut || snap.Offers[0].LowStock {
		t.Fatalf("sold out cell %+v", snap.Offers[0])
	}
	st = stock()
	// 4. Stale expected_version: 409 conflict (the UI re-reads).
	if status, body, _ := lcbAdjust(e, liveTok, e.money, wh, 1, st.BalanceVersion-1, "x"); status != 409 {
		t.Fatalf("stale version: %d %v", status, body)
	}
	// low_stock boundary through the live edit: sellable 3 → low, 6 → not low.
	if status, body, _ := lcbAdjust(e, liveTok, e.money, wh, 3, st.BalanceVersion, "x"); status != 200 {
		t.Fatalf("restock 3: %d %v", status, body)
	}
	if o := lcbMustConsole(t, e, s).Offers[0]; !o.LowStock || o.SoldOut || o.Stock.Sellable != 3 {
		t.Fatalf("low stock cell %+v", o)
	}
	st = stock()
	if status, _, _ := lcbAdjust(e, liveTok, e.money, wh, 3, st.BalanceVersion, "x"); status != 200 {
		t.Fatal("restock to 6")
	}
	if o := lcbMustConsole(t, e, s).Offers[0]; o.LowStock || o.Stock.Sellable != 6 {
		t.Fatalf("not-low cell %+v", o)
	}
	st = stock()

	// 5. inventory:write keeps the unbounded edit (any delta, own reason); so does a principal with both.
	status, body, key = lcbAdjust(e, writeTok, e.money, wh, 5000, st.BalanceVersion, "restock-from-supplier")
	if status != 200 {
		t.Fatalf("write principal +5000: %d %v", status, body)
	}
	if got := ledgerReason(key); got != "restock-from-supplier" {
		t.Fatalf("unbounded reason %q", got)
	}
	st = stock()
	if status, body, _ = lcbAdjust(e, bothTok, e.money, wh, 2000, st.BalanceVersion, "both"); status != 200 {
		t.Fatalf("both permissions +2000 must stay unbounded: %d %v", status, body)
	}
	// 6. Neither permission: 403. Another tenant: 404. No token: 401.
	st = stock()
	if status, body, _ := lcbAdjust(e, noneTok, e.money, wh, 1, st.BalanceVersion, "x"); status != 403 || body["code"] != "forbidden" {
		t.Fatalf("no adjust permission: %d %v", status, body)
	}
	body2, _ := json.Marshal(map[string]any{"warehouse_id": wh, "sku_id": e.money, "delta": 1, "expected_version": st.BalanceVersion, "reason": "x"})
	if status, m, _ := e.mcall(e.token(), "POST", "/v1/admin/stores/"+f.storeB+"/inventory/adjustments", t04Key("lcb-adj"), string(body2)); status != 404 {
		t.Fatalf("other tenant's store: %d %v", status, m)
	}
	if status, _, _ := lcbAdjust(e, "", e.money, wh, 1, st.BalanceVersion, "x"); status != 401 {
		t.Fatalf("no token: %d", status)
	}
	// 7. A bounded edit can never take a fresh SKU below zero either (1 on hand, remove 2).
	fresh := e.sku(1000, 1)
	if status, body, _ := lcbAdjust(e, liveTok, fresh, wh, -2, 1, "x"); status != 422 || body["code"] != "below_reserved" {
		t.Fatalf("below zero: %d %v", status, body)
	}
}

// TestLiveConsoleLCN17DefinerACL pins the two 0148 definers exactly: owner, SECURITY DEFINER, no-login non-bypass owner, fixed search_path, volatility, the
// explicit EXECUTE grantees and no PUBLIC; every other commerce_* role may not call either; the runtime pool can run the sales read but not the claims helper.
func TestLiveConsoleLCN17DefinerACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	for _, fn := range []struct {
		signature, owner string
		exec             []string // explicit non-owner EXECUTE grantees
		stable           bool
	}{
		{"claims.console_session_facts(uuid,uuid,uuid)", "commerce_claims_writer", []string{"commerce_auth"}, true},
		{"identity.read_live_console_sales(bytea,uuid,uuid)", "commerce_auth", []string{"commerce_runtime"}, false},
	} {
		t.Run(fn.signature, func(t *testing.T) {
			var owner string
			var definer, noLogin, noBypass, fixedPath, stable, publicExec bool
			var grantees []string
			err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner),p.prosecdef,NOT r.rolcanlogin,NOT r.rolbypassrls,
			 p.proconfig @> ARRAY['search_path=pg_catalog']::text[], p.provolatile='s',
			 EXISTS(SELECT 1 FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'),
			 ARRAY(SELECT pg_get_userbyid(a.grantee) FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			  WHERE a.privilege_type='EXECUTE' AND a.grantee<>p.proowner ORDER BY 1)
			 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, fn.signature).
				Scan(&owner, &definer, &noLogin, &noBypass, &fixedPath, &stable, &publicExec, &grantees)
			if err != nil {
				t.Fatal(err)
			}
			if owner != fn.owner || !definer || !noLogin || !noBypass || !fixedPath || stable != fn.stable || publicExec || !reflect.DeepEqual(grantees, fn.exec) {
				t.Fatalf("definer shape: owner=%s definer=%v noLogin=%v noBypass=%v path=%v stable=%v publicExec=%v ACL=%v", owner, definer, noLogin, noBypass, fixedPath, stable, publicExec, grantees)
			}
			var others int
			if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM pg_roles WHERE rolname LIKE 'commerce\_%'
			 AND rolname <> $2 AND NOT rolname = ANY($3::text[]) AND has_function_privilege(oid,$1,'EXECUTE')`, fn.signature, fn.owner, fn.exec).Scan(&others); err != nil || others != 0 {
				t.Fatalf("unexpected callers=%d err=%v", others, err)
			}
		})
	}
}
