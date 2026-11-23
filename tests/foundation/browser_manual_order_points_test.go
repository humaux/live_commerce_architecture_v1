//go:build browser

package foundation_test

// v2 (pre-publication correction 1): NON-Test helper for the M7 order-points corpus. This file defines no `Test*` function and no new
// compiled test case: the EXISTING TestBrowserManualOrderLink owns the ONE admin Next / Go API / PG lifecycle and calls these two
// helpers around its existing single JS run (root integrator hook):
//
//	points := mtPointsSeamNew(t, e, <points store>)   // before it builds the child env; merge points.env() into that env map
//	...existing single run of tests/storefront/manual-order-link-gate.mjs (old ten cases + points spec, one Next lifecycle)...
//	points.check(t, e, evidence)                      // after cmd.Run(), scoped to the points store
//
// The task fixture permission control is a runner-only endpoint that reuses the existing control pattern (local httptest server +
// random X-Gate-Key): the spec signs a FRESH context in after each level change, so ONE suite process exercises
// full -> readonly -> noread. No timeout increase, no second Next, no legacy replay, and the public business API is unchanged.
//
// SEPARATE STORE, SAME PG, NO PARENT EDIT: mtPointsSeamNew creates its own store in the same already-open PG/API and seeds a complete
// corpus through a value-copy view of tcvEnv (copy the struct + the fixture, re-point storeA1, keep the DB pools/services). Every
// existing store-scoped helper then operates on the points store, so the legacy store's final audit count (2) and order ids cannot
// move. The legacy baseline is stamped by the control AFTER the old ten cases (they create those two orders/audits), and check()
// asserts zero delta from that stamp -- never a whole-run zero delta. GAP-1 is closed inside this file.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
	"livecommerce/internal/pricing"
)

// pointsCanary is the synthetic marker every browser-created member name, contact note, adjust note and order customer name carries.
const pointsCanary = "ZqPointsCanary"

// mtPointsControlPermissions is the exact fixture grant set the control applies to the browser principal on the points store. Only the
// customer permissions vary between levels; the rest is what one manual order plus the catalog picker already need, so the control
// never widens a business rule (the real API still checks every permission on every request).
var mtPointsControlPermissions = map[string][]string{
	"full":     {"store:read", "catalog:read", "inventory:read", "inventory:reserve", "orders:read", "customers:read", "customers:points"},
	"readonly": {"store:read", "catalog:read", "inventory:read", "inventory:reserve", "orders:read", "customers:read"},
	"noread":   {"store:read", "catalog:read", "inventory:read", "inventory:reserve", "orders:read"},
}

type mtPointsBaseline struct {
	manualOrders   map[string]bool
	members        map[string]bool
	entries        int
	orderAudits    int
	orderReceipts  int
	memberReceipts int
	adjustReceipts int
}

type mtPointsSeam struct {
	t            *testing.T
	productName  string
	conversation string
	store        string
	legacy       string
	principal    string
	canary       string
	control      *httptest.Server
	key          string
	created      bool
	base         mtPointsBaseline // points store, taken BEFORE the browser run: the oracle is delta-based, never an absolute zero
	legacyBase   mtPointsBaseline
	// legacyStamped records that the control stamped the legacy baseline AFTER the old ten cases (the only correct instant: the
	// legacy gate itself creates two orders/audits). Without the stamp the legacy comparison is refused instead of guessed.
	legacyStamped bool
	missAward     atomic.Bool
}

// mtPointsSeamNew binds the points corpus to its own store in the SAME already-open PG/API, snapshots the store-scoped baseline (the
// shared fixture already holds an order, so nothing assumes absolute zero) and starts the permission control.
func mtPointsSeamNew(t *testing.T, e *tcvEnv, pointsStore string) *mtPointsSeam {
	t.Helper()
	s := &mtPointsSeam{t: t, store: pointsStore, legacy: e.store(), principal: e.p.f.principalA, canary: pointsCanary, key: randomToken()}
	if s.store == "" {
		s.store, s.created = randomUUID(), true
	}
	if s.store == s.legacy {
		t.Fatal("SETUP_FAIL: the points corpus must not share the legacy manual-order store (it would move the legacy audit/order counts)")
	}
	s.seed(t, e) // same PG/API, own store/corpus; the original e is never re-pointed
	s.conversation = lcConversation(t, e.p.f, e.p.f.tenantA, s.store, "page")
	mustExec(t, e.p.f.owner, "UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp() WHERE conversation_id=$1", s.conversation)
	mustExec(t, e.p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'inbox:read') ON CONFLICT DO NOTHING`,
		e.p.f.tenantA, s.store, s.principal)
	s.grant(e, "full")
	s.base = s.snapshotAt(context.Background(), e, s.store)
	mux := http.NewServeMux()
	// The hook calls this immediately after the legacy ten cases: their two orders/audits are then part of the legacy baseline.
	mux.HandleFunc("POST /points-baseline", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != s.key {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		s.legacyBase = s.snapshotAt(context.Background(), e, s.legacy)
		s.legacyStamped = true
		_ = json.NewEncoder(w).Encode(map[string]int{"legacy_orders": len(s.legacyBase.manualOrders), "legacy_audits": s.legacyBase.orderAudits})
	})
	mux.HandleFunc("POST /points-permissions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != s.key {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var in struct {
			Level string `json:"level"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in) != nil || mtPointsControlPermissions[in.Level] == nil {
			http.Error(w, "invalid level", http.StatusBadRequest)
			return
		}
		s.grant(e, in.Level)
		_ = json.NewEncoder(w).Encode(map[string]string{"level": in.Level, "store": s.store})
	})
	mux.HandleFunc("POST /points-miss-award", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gate-Key") != s.key {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		s.missAward.Store(true)
		w.WriteHeader(http.StatusNoContent)
	})
	s.control = httptest.NewServer(mux)
	t.Cleanup(s.control.Close)
	return s
}

// seed creates the points store in the SAME already-open PG/API and gives it a complete corpus with the ordinary domain calls. The
// value-copy view is the whole seam: copy the tcvEnv and the fixture, re-point storeA1, and every existing store-scoped helper
// (t04CreateStock, pricingScoped, dsPolicy/dsSet/daSet, cofEnsureSettings, service) then operates on the points store. DB pools,
// buyer infrastructure and services are retained by the copy and never closed here (they belong to the parent env).
func (s *mtPointsSeam) seed(t *testing.T, e *tcvEnv) {
	t.Helper()
	ctx := context.Background()
	f := *e.p.f
	f.storeA1 = s.store
	view := *e
	view.p.bcHarness.cqHarness.f = &f // struct harnesses copy by value; the fixture pointer is the only rebound authority
	view.bh.bcHarness.cqHarness.f = &f
	view.svcs, view.svcVer = map[string]fulfillment.ServiceInput{}, map[string]int64{} // independent of the legacy store's caches
	view.settingsVer = 0                                                               // the new store starts with no CVS settings version
	if s.created {
		mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'fixture-store-order-points','TWD')`, f.tenantA, f.storeA1)
	}
	// SETUP grants of the browser principal on the points store (the permission control narrows only customers:read/points later).
	for _, perm := range []string{"store:read", "catalog:read", "catalog:write", "inventory:read", "inventory:write", "inventory:reserve",
		"pricing:read", "pricing:write", "integration:manage", "integration:read", "orders:read", "fulfillment:write"} {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			f.tenantA, f.storeA1, f.principalA, perm)
	}
	stock := t04CreateStock(t, &f, f.tokens["a"], f.storeA1, 12, 20)
	s.productName = stock.product.Name
	view.p.stock = stock // re-bound to the real new stock/warehouse; the legacy stock is never reused
	market, err := pricingScoped(ctx, &f, f.tokens["a"], f.storeA1, "pricing:write", func(tx pgx.Tx, scope platform.Scope) (pricing.Market, error) {
		return pricing.CreateMarket(ctx, tx, scope, t04Key("points-market"), pricing.MarketInput{Code: "tw", Name: "Order points market", Currency: "TWD"})
	})
	if err != nil {
		t.Fatal(err)
	}
	view.p.market = market
	zero := int64(0)
	view.p.policy = pricing.PolicyInput{MarketID: market.ID, Country: "TW", Currency: "TWD", ShippingMode: "country_flat", ShippingMinor: &zero,
		TaxMode: "none", TaxBasis: "goods", TaxRateBPS: &zero, QuoteTTLSeconds: 300, Enabled: true, ConfigurationRef: "synthetic order points fixture"}
	delivery := fulfillment.ServiceInput{MarketID: market.ID, Country: "TW", Code: "home", PolicyVersion: 1, NameHans: "测试配送", NameHant: "測試配送",
		NameEN: "Home delivery", DeliveryKind: "home", Mode: "MANUAL", Enabled: true, Visible: true}
	dsPolicy(t, view.p.bcHarness.cqHarness, delivery, 0, 0, true)
	if _, err := dsSet(view.p.bcHarness.cqHarness, t04Key("points-delivery"), delivery); err != nil {
		t.Fatal(err)
	}
	allocation := fulfillment.AllocationInput{MarketID: market.ID, Country: "TW", Code: "home", ExpectedServiceVersion: 1, WarehouseIDs: []string{stock.warehouse.ID}}
	if _, err := daSet(view.p.bcHarness.cqHarness, t04Key("points-allocation"), allocation); err != nil {
		t.Fatal(err)
	}
	view.cofEnsureSettings(0, true, true, 72) // bank transfer on for this store through the ordinary merchant route
}

// env is merged into the existing child env map by the parent hook; the spec reads exactly these names.
func (s *mtPointsSeam) env() map[string]string {
	return map[string]string{
		"LC_POINTS_PRODUCT":           s.productName,
		"LC_LINK_POINTS_STORE":        s.store,
		"LC_LINK_POINTS_CONTROL":      s.control.URL,
		"LC_LINK_POINTS_CONTROL_KEY":  s.key,
		"LC_LINK_POINTS_CANARY":       s.canary,
		"LC_LINK_POINTS_CONVERSATION": s.conversation,
	}
}

// grant replaces the browser principal's fixture grants on the points store with exactly one frozen level set.
func (s *mtPointsSeam) grant(e *tcvEnv, level string) {
	s.t.Helper()
	perms := mtPointsControlPermissions[level]
	if perms == nil {
		s.t.Fatalf("unknown points level %q", level)
	}
	mustExec(s.t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission IN ('customers:read','customers:points')`,
		e.p.f.tenantA, s.store, s.principal)
	for _, perm := range perms {
		mustExec(s.t, e.p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			e.p.f.tenantA, s.store, s.principal, perm)
	}
}

func (s *mtPointsSeam) snapshotAt(ctx context.Context, e *tcvEnv, store string) mtPointsBaseline {
	s.t.Helper()
	base := mtPointsBaseline{manualOrders: map[string]bool{}, members: map[string]bool{}}
	rows, err := e.p.f.owner.Query(ctx, `SELECT id::text FROM checkout.orders WHERE store_id=$1 AND source='merchant_manual'`, store)
	if err != nil {
		s.t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			s.t.Fatal(err)
		}
		base.manualOrders[id] = true
	}
	rows.Close()
	rows, err = e.p.f.owner.Query(ctx, `SELECT id::text FROM loyalty.members WHERE store_id=$1`, store)
	if err != nil {
		s.t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			s.t.Fatal(err)
		}
		base.members[id] = true
	}
	rows.Close()
	base.entries = e.count(`SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1`, store)
	base.orderAudits = e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='order.manual_created'`, store)
	base.orderReceipts = e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation='merchanttools.order.manual'`, store)
	base.memberReceipts = e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation='loyalty.member.create'`, store)
	base.adjustReceipts = e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation='loyalty.points.adjust'`, store)
	return base
}

// pointsEvidence is the spec's artifact: ids and numbers only (no display name, contact note or ledger note).
type pointsEvidence struct {
	Phases     []string `json:"phases"`
	Cases      int      `json:"cases"`
	RealOrders []struct {
		OrderID          string  `json:"order_id"`
		MemberID         *string `json:"member_id"`
		AwardEntryID     string  `json:"award_entry_id"`
		DeductionEntryID string  `json:"deduction_entry_id"`
		BalanceAfter     int64   `json:"balance_after"`
		Recovery         bool    `json:"recovery"`
		PlacementKey     string  `json:"placement_key"`
		RecoveryKey      string  `json:"recovery_key"`
		ConversationID   string  `json:"conversation_id"`
		CustomerName     string  `json:"customer_name"`
		Permission       string  `json:"permission"`
	} `json:"real_orders"`
	RealMembers []struct {
		MemberID string `json:"member_id"`
		Kind     string `json:"kind"`
	} `json:"real_members"`
	MockAwards []struct {
		OrderID string `json:"order_id"`
		Reason  string `json:"reason"`
		Locale  string `json:"locale"`
	} `json:"mock_awards"`
	Permission map[string]struct {
		OrderID            string   `json:"order_id"`
		DirectAdjustStatus int      `json:"direct_adjust_status"`
		MemberReads        []string `json:"member_reads"`
	} `json:"permission"`
}

// check pins the REAL_PG effects of the one-lifecycle browser corpus as DELTAS from the two store-scoped snapshots: no absolute-zero
// assumption, no fake expected count, and the legacy store is asserted unchanged instead of being taken on faith.
func (s *mtPointsSeam) check(t *testing.T, e *tcvEnv, evidence string) {
	t.Helper()
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join(evidence, "order-points", "result.json"))
	if err != nil {
		t.Fatalf("the points spec wrote no evidence: %v", err)
	}
	var ev pointsEvidence
	if err := json.Unmarshal(raw, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Cases == 0 || strings.Join(ev.Phases, ",") != "full,readonly,noread" {
		t.Fatalf("points evidence: cases=%d phases=%v", ev.Cases, ev.Phases)
	}
	awarded, recovered := 0, 0
	full, readonly, noread := 0, 0, 0
	readonlyID, noreadID := "", ""
	seenOrders := make(map[string]bool, len(ev.RealOrders))
	for _, order := range ev.RealOrders {
		if order.OrderID == "" || seenOrders[order.OrderID] {
			t.Fatal("points evidence has an empty or duplicate order id")
		}
		seenOrders[order.OrderID] = true
		switch order.Permission {
		case "":
			full++
		case "customers:read only":
			readonly++
			readonlyID = order.OrderID
		case "no customers:read":
			noread++
			noreadID = order.OrderID
		default:
			t.Fatal("points evidence has an unknown permission phase")
		}
		if order.Permission != "" && order.MemberID != nil {
			t.Fatal("a permission-refusal order carries a member award")
		}
		if order.Recovery {
			recovered++
		} else if order.MemberID != nil {
			awarded++
		}
	}
	// The two recovery orders augment the existing placement and refusal corpus.
	if len(ev.RealOrders) != 11 || full != 9 || readonly != 1 || noread != 1 || len(ev.RealMembers) != 13 || len(ev.MockAwards) != 12 || awarded != 6 || recovered != 2 {
		t.Fatalf("points corpus: orders=%d phases=%d/%d/%d awarded=%d recovered=%d members=%d mock=%d", len(ev.RealOrders), full, readonly, noread, awarded, recovered, len(ev.RealMembers), len(ev.MockAwards))
	}
	if len(ev.Permission) != 2 || ev.Permission["readonly"].OrderID != readonlyID || ev.Permission["readonly"].DirectAdjustStatus != 403 ||
		ev.Permission["noread"].OrderID != noreadID || len(ev.Permission["noread"].MemberReads) != 0 {
		t.Fatal("points permission evidence does not match the two refusal orders")
	}
	after := s.snapshotAt(ctx, e, s.store)
	delta := func(before, now int, what string, want int) {
		t.Helper()
		if now-before != want {
			t.Fatalf("%s delta: %d, want %d", what, now-before, want)
		}
	}
	delta(s.base.entries, after.entries, "ledger entries", 2*awarded+recovered)
	delta(s.base.orderAudits, after.orderAudits, "manual-order audits", len(ev.RealOrders))
	delta(s.base.orderReceipts, after.orderReceipts, "manual-order receipts", len(ev.RealOrders))
	delta(s.base.memberReceipts, after.memberReceipts, "member-create receipts", len(ev.RealMembers))
	delta(s.base.adjustReceipts, after.adjustReceipts, "adjust receipts", awarded)
	for id := range after.manualOrders {
		if !s.base.manualOrders[id] && !containsOrderID(ev, id) {
			t.Fatalf("a manual order appeared in the points store without points evidence: %s", id)
		}
	}
	for _, order := range ev.RealOrders {
		if !after.manualOrders[order.OrderID] {
			t.Fatalf("evidence order %s is not a manual order of the points store", order.OrderID)
		}
	}
	for id := range after.members {
		if !s.base.members[id] && !containsMemberID(ev, id) {
			t.Fatalf("a member appeared in the points store without points evidence: %s", id)
		}
	}
	for _, member := range ev.RealMembers {
		want := 0
		var wantBalance int64
		if member.Kind == "awarded" {
			want = 2
		}
		if member.Kind == "recovered" {
			want, wantBalance = 1, 30
		}
		var balance int64
		if err := e.p.f.owner.QueryRow(ctx, `SELECT balance FROM loyalty.members WHERE store_id=$1 AND id=$2`, s.store, member.MemberID).Scan(&balance); err != nil || balance != wantBalance {
			t.Fatalf("member balance: %d, want %d, err=%v", balance, wantBalance, err)
		}
		if n := e.count(`SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1 AND member_id=$2`, s.store, member.MemberID); n != want {
			t.Fatalf("%s member entries: %d, want %d", member.Kind, n, want)
		}
	}
	for _, order := range ev.RealOrders {
		var source, state, recipient string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT source,commercial_state,coalesce(snapshot#>>'{destination,recipient_name}','') FROM checkout.orders WHERE store_id=$1 AND id=$2`,
			s.store, order.OrderID).Scan(&source, &state, &recipient); err != nil || source != "merchant_manual" || state != "AWAITING_TRANSFER" {
			t.Fatalf("order readback: source=%q state=%q err=%v", source, state, err)
		}
		if order.CustomerName != "" && recipient != order.CustomerName {
			t.Fatalf("the order snapshot must keep the merchant-edited customer name (LP17)")
		}
		if order.MemberID == nil {
			continue
		}
		var memberName string
		if err := e.p.f.owner.QueryRow(ctx, `SELECT display_name FROM loyalty.members WHERE store_id=$1 AND id=$2`, s.store, *order.MemberID).Scan(&memberName); err != nil {
			t.Fatal(err)
		}
		if order.CustomerName != "" && memberName == recipient {
			t.Fatalf("the order customer name must stay independent of the member display name (LP17)")
		}
		if n := e.count(`SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1 AND order_id=$2 AND member_id=$3 AND reason='manual_order_award' AND delta=30 AND note IS NULL`,
			s.store, order.OrderID, *order.MemberID); n != 1 {
			t.Fatalf("award entry for %s: %d", order.OrderID, n)
		}
		if order.Recovery {
			if n := e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation='merchanttools.order.manual' AND idempotency_key=$2
				AND response->>'order_id'=$3 AND response#>>'{member_award,state}'='not_awarded' AND response#>>'{member_award,reason}'='forbidden'
				AND response#>>'{member_award,member_id}'=$4 AND response#>>'{member_award,points}'='30'`,
				s.store, order.PlacementKey, order.OrderID, *order.MemberID); n != 1 {
				t.Fatalf("original failed award receipt: %d", n)
			}
			if n := e.count(`SELECT count(*) FROM ops.command_results r JOIN loyalty.point_entries p ON p.store_id=r.store_id AND p.id::text=r.response->>'entry_id'
				WHERE r.store_id=$1 AND r.operation='loyalty.order_award.recover' AND r.idempotency_key=$2
				AND p.order_id=$3 AND p.member_id=$4 AND p.id=$5 AND p.actor_principal_id=$6 AND r.response->>'state'='awarded'`,
				s.store, order.RecoveryKey, order.OrderID, *order.MemberID, order.AwardEntryID, s.principal); n != 1 {
				t.Fatalf("keyed recovery receipt and ledger: %d", n)
			}
			if order.ConversationID != "" {
				if order.ConversationID != s.conversation || e.count(`SELECT count(*) FROM inbox.manual_order_sources WHERE store_id=$1 AND conversation_id=$2 AND order_id=$3 AND state='placed'`, s.store, s.conversation, order.OrderID) != 1 {
					t.Fatal("the Inbox source must persist on the recovered original order")
				}
				if n := e.count(`SELECT count(*) FROM inbox.outbound_messages WHERE store_id=$1 AND conversation_id=$2`, s.store, s.conversation); n != 0 {
					t.Fatalf("contextual manual placement and recovery queued %d customer messages", n)
				}
			}
			continue
		}
		if n := e.count(`SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1 AND order_id=$2 AND member_id=$3 AND reason='manual_adjust' AND delta=-30 AND note IS NOT NULL AND actor_principal_id=$4`,
			s.store, order.OrderID, *order.MemberID, s.principal); n != 1 {
			t.Fatalf("deduction entry for %s: %d", order.OrderID, n)
		}
		if order.BalanceAfter != 0 {
			t.Fatalf("the deduction answer carried balance %d", order.BalanceAfter)
		}
	}
	for _, mock := range ev.MockAwards {
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND id=$2`, s.store, mock.OrderID); n != 0 {
			t.Fatalf("MOCK not_awarded order %s exists in PG", mock.OrderID)
		}
		if n := e.count(`SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1 AND order_id=$2`, s.store, mock.OrderID); n != 0 {
			t.Fatalf("MOCK not_awarded order %s has ledger entries", mock.OrderID)
		}
	}
	// PII hygiene: the canary is provably in the points store's order snapshots and provably absent from its receipts/audit details.
	var receipts, details strings.Builder
	rows, err := e.p.f.owner.Query(ctx, `SELECT response::text FROM ops.command_results WHERE store_id=$1`, s.store)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatal(err)
		}
		receipts.WriteString(text)
	}
	rows.Close()
	if strings.Contains(receipts.String(), s.canary) {
		t.Fatal("a command receipt carries the member/order canary (LPI7)")
	}
	rows, err = e.p.f.owner.Query(ctx, `SELECT coalesce(details::text,'') FROM ops.audit_events WHERE store_id=$1`, s.store)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatal(err)
		}
		details.WriteString(text)
	}
	rows.Close()
	if strings.Contains(details.String(), s.canary) {
		t.Fatal("an audit detail carries the member/order canary (LPI7)")
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1 AND snapshot::text LIKE '%'||$2||'%'`, s.store, s.canary); n != len(ev.RealOrders) {
		t.Fatalf("canary positive control: %d order snapshots, want %d", n, len(ev.RealOrders))
	}
	// Legacy store: the hook stamps the baseline AFTER the old ten cases (that gate itself creates two orders/audits), so this asserts
	// the points phase added nothing there -- never a whole-run zero delta.
	if !s.legacyStamped {
		t.Fatal("SETUP_FAIL: the hook must POST /points-baseline after the legacy ten cases and before the points spec")
	}
	var legacy struct {
		OrderID    string `json:"order_id"`
		CodOrderID string `json:"cod_order_id"`
		Cases      int    `json:"cases"`
	}
	legacyRaw, err := os.ReadFile(filepath.Join(evidence, "result.json"))
	if err != nil {
		t.Fatalf("the legacy artifact is missing: %v", err)
	}
	if json.Unmarshal(legacyRaw, &legacy) != nil || legacy.Cases != 10 || legacy.OrderID == "" || legacy.CodOrderID == "" {
		t.Fatalf("the legacy manual-order artifact changed: %s", legacyRaw)
	}
	if !s.legacyBase.manualOrders[legacy.OrderID] || !s.legacyBase.manualOrders[legacy.CodOrderID] {
		t.Fatalf("the stamped legacy baseline lacks the two legacy order ids %s/%s", legacy.OrderID, legacy.CodOrderID)
	}
	if s.legacyBase.orderAudits < 2 {
		t.Fatalf("the stamped legacy baseline has %d manual audits, want >=2", s.legacyBase.orderAudits)
	}
	legacyAfter := s.snapshotAt(ctx, e, s.legacy)
	if len(legacyAfter.manualOrders) != len(s.legacyBase.manualOrders) || legacyAfter.entries != s.legacyBase.entries ||
		legacyAfter.orderAudits != s.legacyBase.orderAudits || legacyAfter.orderReceipts != s.legacyBase.orderReceipts ||
		len(legacyAfter.members) != len(s.legacyBase.members) {
		t.Fatalf("the points phase moved the legacy store: orders %d->%d audits %d->%d entries %d->%d",
			len(s.legacyBase.manualOrders), len(legacyAfter.manualOrders), s.legacyBase.orderAudits, legacyAfter.orderAudits, s.legacyBase.entries, legacyAfter.entries)
	}
}

func containsOrderID(ev pointsEvidence, id string) bool {
	for _, order := range ev.RealOrders {
		if order.OrderID == id {
			return true
		}
	}
	return false
}

func containsMemberID(ev pointsEvidence, id string) bool {
	for _, member := range ev.RealMembers {
		if member.MemberID == id {
			return true
		}
	}
	return false
}
