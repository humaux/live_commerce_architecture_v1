// reports_test.go — W6-02B (contracts/reporting-v2.md) REAL_PG gate for the product / channel / funnel / manual-order reports and their audited CSV.
// Evidence class: REAL_PG on MOCK data. The rows are DISCLOSED SYNTHETIC FIXTURES written with the owner pool under
// session_replication_role=replica (FK triggers off, every CHECK still on): no product path produces "an order paid at 10:00 with a refund at
// 12:00 on Sep 12" for a fixed date, and the point here is the arithmetic of the read definers, not the checkout chain. Every expected number is
// worked out by hand in the comments. W5-03B (historical order import) is not merged in this tree, so RP03 is NOT_RUN here.
//
// Purpose: RP01 hand-checked numbers + finance parity, RP02 offline money, RP04 nested funnel, RP05 range 422, RP06 export permission + audit,
//
//	RP07 cross-store 404, RP08 timing at 10k orders, plus read-only and live:read gates.
//
// Depends on: migrations/0147_reports.sql, internal/reporting, internal/httpapi (reports.go), lcPrincipal/lcToken (live_claims_test.go).
// Used by: go test ./tests/foundation -run '^TestReport'.
package foundation_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/reporting"
)

var (
	rpJob = atomic.Int64{}
	rpTPE = time.FixedZone("TPE", 8*3600)
)

func init() { rpJob.Store(6_100_000_000) }

type rpWorld struct {
	t                   *testing.T
	f                   *testFixture
	tenant, store       string
	h                   http.Handler
	reader, exporter    string // tokens: orders:read+live:read ; +orders:export
	ordersOnly, noOrder string // orders:read only ; live:read only
	staff               string // the principal that "created" the manual orders
}

type rpLine struct {
	sku, product, code, name string
	qty, total               int64
}

type rpOrder struct{ id, owner, attempt string }

// rpNew is a LIVE-deployment store of the fixture's tenant A.
func rpNew(t *testing.T) *rpWorld { return rpNewIn(t, "A", "LIVE") }

// rpNewIn makes a fresh store of tenant "A" or "B" served by a deployment whose payment environment is env (httpapi Options.PaymentEnvironment).
func rpNewIn(t *testing.T, which, env string) *rpWorld {
	t.Helper()
	f := fixture(t)
	tenant := f.tenantA
	if which == "B" {
		tenant = f.tenantB
	}
	w := &rpWorld{t: t, f: f, tenant: tenant, store: randomUUID(), h: httpapi.NewHandler(f.runtime, httpapi.Options{PaymentEnvironment: env})}
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'rp store','TWD')`, tenant, w.store)
	_, w.reader = lcPrincipal(t, f, tenant, []string{w.store}, "store:read", "orders:read", "live:read")
	_, w.exporter = lcPrincipal(t, f, tenant, []string{w.store}, "store:read", "orders:read", "orders:export", "live:read")
	_, w.ordersOnly = lcPrincipal(t, f, tenant, []string{w.store}, "store:read", "orders:read")
	_, w.noOrder = lcPrincipal(t, f, tenant, []string{w.store}, "store:read", "live:read")
	w.staff, _ = lcPrincipal(t, f, tenant, []string{w.store}, "inventory:reserve")
	return w
}

// exec runs one fixture statement with FK triggers disabled (CHECK constraints stay on).
func (w *rpWorld) exec(q string, args ...any) {
	w.t.Helper()
	ctx := context.Background()
	tx, err := w.f.owner.Begin(ctx)
	if err != nil {
		w.t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
		w.t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, q, args...); err != nil {
		w.t.Fatalf("fixture: %v\n%s", err, q)
	}
	if err = tx.Commit(ctx); err != nil {
		w.t.Fatal(err)
	}
}

func rpLines(lines []rpLine) (string, int64) {
	var total int64
	items := make([]map[string]any, len(lines))
	for i, l := range lines {
		total += l.total
		items[i] = map[string]any{"sku_id": l.sku, "product_id": l.product, "code": l.code, "name": l.name, "quantity": l.qty,
			"amount": map[string]any{"total_minor": l.total}}
	}
	raw, _ := json.Marshal(map[string]any{"quote": map[string]any{"lines": items}})
	return string(raw), total
}

// order inserts one order. mode: card | cash_on_delivery | bank_transfer. state: CONFIRMED | CANCELLED.
func (w *rpWorld) order(at time.Time, state, source, mode string, lines ...rpLine) rpOrder {
	snap, total := rpLines(lines)
	o := rpOrder{id: randomUUID(), owner: randomUUID(), attempt: randomUUID()}
	fulfil, collection, carrier, surcharge := "MANUAL_UNASSIGNED", any(nil), any(nil), any(nil)
	if state == "CANCELLED" {
		fulfil = "CANCELLED"
	}
	if mode == "cash_on_delivery" {
		collection, carrier, surcharge = "PENDING", "black_cat", int64(5000)
	}
	w.exec(`INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,quote_id,destination_id,market_id,country,service_code,
	  service_version,allocation_version,currency,total_minor,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at,payment_mode,source,
	  collection_state,cod_carrier,cod_surcharge_minor)
	 VALUES($1,$2,$3,$4,gen_random_uuid(),gen_random_uuid(),1,gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'TW','std',1,1,'TWD',$5,$6,$7,1,$8,$9,$10::jsonb,$11,$11,$12,$13,$14,$15,$16)`,
		w.tenant, w.store, o.owner, o.id, total, state, fulfil, at.Add(10*time.Minute), rpJob.Add(1), snap, at, mode, source, collection, carrier, surcharge)
	return o
}

// capture adds the payment attempt and its CAPTURED fact at the instant at.
func (w *rpWorld) capture(o rpOrder, minor int64, env string, at time.Time) {
	w.exec(`WITH a AS (INSERT INTO checkout.payment_attempts(tenant_id,store_id,owner_id,id,session_id,order_id,market_id,country,method_code,method_version,connection_id,credential_version,
	  qualification_id,environment,execution_profile,binding_id,binding_version,currency,amount_minor,merchant_trade_no,state,generation,job_id)
	 VALUES($1,$2,$3,$4::uuid,gen_random_uuid(),$5,gen_random_uuid(),'TW','payuni_credit',1,gen_random_uuid(),1,gen_random_uuid(),$6,'PROVIDER_MOCK',gen_random_uuid(),1,'TWD',100,
	  substr(replace($4::text,'-',''),1,25),'PAYMENT_PENDING',2,$7) RETURNING 1)
	 INSERT INTO payments.facts(tenant_id,store_id,attempt_id,kind,amount_minor,currency,provider_reference,connection_id,execution_profile,environment,source_report_hash,received_at)
	 VALUES($1,$2,$4::uuid,'CAPTURED',$8,'TWD','ref_'||substr(replace($4::text,'-',''),1,20),gen_random_uuid(),'PROVIDER_MOCK',$6,sha256('x'::bytea),$9)`,
		w.tenant, w.store, o.owner, o.attempt, o.id, env, rpJob.Add(1), minor, at)
}

// refund adds a requested refund of the order's attempt and its SUCCEEDED fact at the instant at.
func (w *rpWorld) refund(o rpOrder, minor int64, env string, at time.Time) {
	id := randomUUID()
	w.exec(`WITH r AS (INSERT INTO payments.stripe_refunds(tenant_id,store_id,id,attempt_id,order_id,owner_id,principal_id,environment,account_id,credential_version,payment_intent_id,currency,
	  amount_minor,reason,create_params,requested_at,resend_until) VALUES($1,$2,$3,$4::uuid,$5,$6,$7,$8,'acct_x1',1,'pi_x1','TWD',$9,'requested_by_customer','{}',$10,$10::timestamptz+interval '20 hours') RETURNING 1)
	 INSERT INTO payments.refund_facts(tenant_id,store_id,refund_id,attempt_id,kind,amount_minor,currency,stripe_refund_id,source_report_hash,received_at)
	 VALUES($1,$2,$3,$4,'SUCCEEDED',$9,'TWD','re_x1',sha256('x'::bytea),$10)`,
		w.tenant, w.store, id, o.attempt, o.id, o.owner, w.staff, env, minor, at)
}

// collect marks a COD order COLLECTED at the instant at (money = total + the 5000 surcharge).
func (w *rpWorld) collect(o rpOrder, at time.Time) {
	w.exec(`UPDATE checkout.orders SET collection_state='COLLECTED',collected_at=$3 WHERE tenant_id=$1 AND id=$2`, w.tenant, o.id, at)
}

// transfer confirms a bank-transfer order for minor at the instant at.
func (w *rpWorld) transfer(o rpOrder, minor int64, at time.Time) {
	w.exec(`INSERT INTO checkout.bank_transfers(tenant_id,store_id,owner_id,order_id,state,bank_name,branch,account_name,account_number,window_hours,currency,created_at,updated_at,
	  confirmed_at,confirmed_by,confirmed_amount_minor) VALUES($1,$2,$3,$4,'CONFIRMED','b','b','a','1',24,'TWD',$6,$6,$6,$5,$7)`,
		w.tenant, w.store, o.owner, o.id, w.staff, at, minor)
}

// manual records the staff receipt of a merchant-created order (the creator the manual report names).
func (w *rpWorld) manual(o rpOrder, principal string) {
	w.exec(`INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
	 VALUES($1,$2,'merchanttools.order.manual',$3,sha256('x'::bytea),jsonb_build_object('order_id',$4::text),$5)`,
		w.tenant, w.store, "rp-key-"+o.id[:12], o.id, principal)
}

func (w *rpWorld) session(title string) string {
	id := randomUUID()
	w.exec(`INSERT INTO live.sessions(tenant_id,store_id,id,principal_id,title) VALUES($1,$2,$3,$4,$5)`, w.tenant, w.store, id, w.staff, title)
	return id
}

// bundle makes a bundle of the platform in the session with one claim event at the instant at; sent adds a SUCCEEDED private-reply link,
// orders adds one order_origins row per order.
func (w *rpWorld) bundle(session, platform string, at time.Time, accepted, sent bool, orders ...rpOrder) string {
	id := randomUUID()
	w.exec(`INSERT INTO claims.bundles(tenant_id,store_id,id,session_id,platform,actor_key) VALUES($1,$2,$3::uuid,$4,$5,md5($3::uuid::text)||md5($3::uuid::text||'x'))`, w.tenant, w.store, id, session, platform)
	if accepted {
		w.exec(`INSERT INTO claims.events(tenant_id,store_id,session_id,window_generation,source_kind,source_event_id,platform,occurred_at,grammar_version,grammar_kind,match_mode,
		  outcome,offer_id,quantity,explicit_quantity,bundle_id,line_version,bundle_version) VALUES($1,$2,$4,1,'meta',gen_random_uuid(),$5,$6,'kw-v1','MATCH','EXACT','ACCEPTED',gen_random_uuid(),1,true,$3::uuid,1,1)`,
			w.tenant, w.store, id, session, platform, at)
	} else { // a REJECTED event never counts
		w.exec(`INSERT INTO claims.events(tenant_id,store_id,session_id,window_generation,source_kind,source_event_id,platform,occurred_at,grammar_version,grammar_kind,match_mode,
		  outcome,reason) VALUES($1,$2,$3,1,'meta',gen_random_uuid(),$4,$5,'kw-v1','NO_MATCH','EXACT','REJECTED','NO_MATCH')`, w.tenant, w.store, session, platform, at)
	}
	if sent {
		w.exec(`INSERT INTO integration.operations(id,tenant_id,store_id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,
		  job_id,state,generation,created_at) VALUES(gen_random_uuid(),$1,$2,$3,gen_random_uuid(),1,'facebook','page','service','meta.private_reply','mpr:'||replace($4::uuid::text,'-',''),sha256('x'::bytea),
		  jsonb_build_object('bundle_id',$4::uuid::text),$5,'SUCCEEDED',1,$6)`, w.tenant, w.store, w.staff, id, rpJob.Add(1), at) // created_at = the claim time: the funnel reads operations only up to range end + 7 days, so a now() default broke this test once the wall clock passed 2026-10-07
	}
	for _, o := range orders {
		w.exec(`INSERT INTO claims.order_origins(tenant_id,store_id,order_id,bundle_id,offer_id,line_version,session_id,occurred_at) VALUES($1,$2,$3,$4,gen_random_uuid(),1,$5,$6)`,
			w.tenant, w.store, o.id, id, session, at)
	}
	return id
}

func (w *rpWorld) get(token, path string) (int, []byte, http.Header) {
	w.t.Helper()
	r := httptest.NewRequest("GET", "/v1/admin/stores/"+w.store+"/reports/"+path, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	w.h.ServeHTTP(rec, r)
	return rec.Code, rec.Body.Bytes(), rec.Header()
}

func (w *rpWorld) must(token, path string, out any) {
	w.t.Helper()
	status, body, _ := w.get(token, path)
	if status != 200 {
		w.t.Fatalf("GET %s = %d %s", path, status, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		w.t.Fatalf("decode %s: %v %s", path, err, body)
	}
}

func rpAt(day, hour int) time.Time { return time.Date(2026, 9, day, hour, 0, 0, 0, rpTPE) }

const rpQ = "?from=2026-09-01&to=2026-09-30"

// rpSeedMain builds the hand-checked store (see the numbers beside each order) and returns the sessions.
func rpSeedMain(w *rpWorld) (s1, s2 string, sku [3]string) {
	for i := range sku {
		sku[i] = randomUUID()
	}
	A := func(q, total int64) rpLine { return rpLine{sku[0], randomUUID(), "AAA", "=Alpha", q, total} }
	B := func(q, total int64) rpLine { return rpLine{sku[1], randomUUID(), "BBB", "Beta", q, total} }
	C := func(q, total int64) rpLine { return rpLine{sku[2], randomUUID(), "CCC", "Gamma", q, total} }
	s1, s2 = w.session("rp s1"), w.session("rp s2")

	// O1 storefront: A x2 20000 + B x1 10000, captured 30000 LIVE Sep 10, refunded 9000 Sep 12.
	o1 := w.order(rpAt(10, 9), "CONFIRMED", "storefront", "card", A(2, 20000), B(1, 10000))
	w.capture(o1, 30000, "LIVE", rpAt(10, 10))
	w.refund(o1, 9000, "LIVE", rpAt(12, 12))
	// O2 facebook bundle F1 (session s1): A 3333 + B 3333 + C 3334 = 10000 lines, captured 10001 LIVE (the extra 1 goes to the largest fraction: C).
	o2 := w.order(rpAt(11, 9), "CONFIRMED", "storefront", "card", A(1, 3333), B(1, 3333), C(1, 3334))
	w.capture(o2, 10001, "LIVE", rpAt(11, 10))
	// O3 instagram bundle F5 (no link sent): B x3 30000, captured 30000 SANDBOX Sep 15.
	o3 := w.order(rpAt(15, 9), "CONFIRMED", "storefront", "card", B(3, 30000))
	w.capture(o3, 30000, "SANDBOX", rpAt(15, 10))
	// O4 manual (staff), no session: A x1 10000 cash on delivery, collected Sep 16 -> 10000 + 5000 surcharge = 15000 offline.
	o4 := w.order(rpAt(16, 9), "CONFIRMED", "merchant_manual", "cash_on_delivery", A(1, 10000))
	w.manual(o4, w.staff)
	w.collect(o4, rpAt(16, 12))
	// O5 manual with a facebook origin (bundle F2): B x1 8000 bank transfer confirmed Sep 17 for 8000 offline.
	o5 := w.order(rpAt(17, 9), "CONFIRMED", "merchant_manual", "bank_transfer", B(1, 8000))
	w.manual(o5, w.staff)
	w.transfer(o5, 8000, rpAt(17, 15))
	// O7 created before the range, captured inside it (money counts, the order does not): C x1 5000 LIVE Sep 5.
	o7 := w.order(time.Date(2026, 8, 20, 9, 0, 0, 0, rpTPE), "CONFIRMED", "storefront", "card", C(1, 5000))
	w.capture(o7, 5000, "LIVE", rpAt(5, 10))
	// O8 on the Sep 30 / Oct 1 boundary: captured 23:30 Taipei Sep 30 is in range, 00:30 Oct 1 is not (O9).
	o8 := w.order(rpAt(30, 22), "CONFIRMED", "storefront", "card", B(1, 100))
	w.capture(o8, 100, "LIVE", time.Date(2026, 9, 30, 23, 30, 0, 0, rpTPE))
	o9 := w.order(rpAt(30, 22), "CONFIRMED", "storefront", "card", B(1, 777))
	w.capture(o9, 777, "LIVE", time.Date(2026, 10, 1, 0, 30, 0, 0, rpTPE))

	o6 := w.order(rpAt(18, 9), "CANCELLED", "storefront", "card", A(1, 10000)) // cancelled in range: counted as cancelled, no money
	// Funnel (session s1): F1 facebook claim+link+O2 (paid), F2 facebook claim+link+O5 (paid), F3 claim+link only, F4 claim only,
	// F5 instagram claim, no link, +O3 (paid), F7 claim outside the range, F8 rejected event only. Session s2: F6 claim+link+O6 (cancelled).
	w.bundle(s1, "facebook", rpAt(10, 8), true, true, o2)
	w.bundle(s1, "facebook", rpAt(10, 8), true, true, o5)
	w.bundle(s1, "facebook", rpAt(10, 8), true, true)
	w.bundle(s1, "facebook", rpAt(10, 8), true, false)
	w.bundle(s1, "instagram", rpAt(10, 8), true, false, o3)
	w.bundle(s1, "facebook", time.Date(2026, 8, 1, 8, 0, 0, 0, rpTPE), true, true)
	w.bundle(s1, "facebook", rpAt(10, 8), false, false)
	w.bundle(s2, "facebook", rpAt(12, 8), true, true, o6)
	return s1, s2, sku
}

// TestReportRP01ProductsChannelsMatchHandNumbersAndFinance proves RP01/RP02: every number equals the hand calculation in rpSeedMain, the channel
// sums equal read_finance_summary of the same range, offline money is its own column, and the reads write nothing.
func TestReportRP01ProductsChannelsMatchHandNumbersAndFinance(t *testing.T) {
	w := rpNew(t)
	_, _, sku := rpSeedMain(w)
	before := map[string]int{}
	for _, tb := range []string{"checkout.orders", "payments.facts", "payments.refund_facts", "claims.order_origins", "ops.audit_events", "ops.command_results"} {
		before[tb] = countRows(t, w.f.owner, "SELECT count(*) FROM "+tb)
	}

	// ---- products (LIVE then the SANDBOX row, net descending overall) ----
	var prod reporting.ProductReport
	w.must(w.reader, "products"+rpQ, &prod)
	type key struct{ sku, env string }
	want := map[key][6]int64{ // units, captured, refunded, net, offlineUnits, offlineMinor
		// A: O1 20000 + O2 3333 captured; refund 6000 (= 9000*2/3); units 2+1; offline O4 15000, 1 unit. Net 17333.
		{sku[0], "LIVE"}: {3, 23333, 6000, 17333, 1, 15000},
		// B: O1 10000 + O2 3333 + O8 100; refund 3000; units 1+1+1; offline O5 8000, 1 unit. Net 10433. (O9 is Oct 1 Taipei: out.)
		{sku[1], "LIVE"}: {3, 13433, 3000, 10433, 1, 8000},
		// C: O2 3335 (3334 + the remainder 1) + O7 5000; units 1+1. Net 8335.
		{sku[2], "LIVE"}: {2, 8335, 0, 8335, 0, 0},
		// B in SANDBOX: O3 30000, 3 units. Never mixed with LIVE (I05).
		{sku[1], "SANDBOX"}: {3, 30000, 0, 30000, 0, 0},
	}
	if prod.Truncated || len(prod.Rows) != len(want) {
		t.Fatalf("product rows: %+v", prod)
	}
	var liveNet int64
	for i, r := range prod.Rows {
		exp, ok := want[key{r.SKUID, r.Environment}]
		got := [6]int64{r.Units, r.CapturedMinor, r.RefundedMinor, r.NetMinor, r.OfflineUnits, r.OfflineMinor}
		if !ok || got != exp || r.Currency != "TWD" {
			t.Errorf("product row %d %s/%s: got %v want %v", i, r.SKUID, r.Environment, got, exp)
		}
		if r.Environment == "LIVE" {
			liveNet += r.NetMinor
		}
	}
	if prod.Rows[0].Environment != "SANDBOX" || prod.Rows[0].NetMinor != 30000 {
		t.Errorf("rows are not net-descending: %+v", prod.Rows[0])
	}

	// ---- channels ----
	var ch reporting.ChannelReport
	w.must(w.reader, "channels"+rpQ, &ch)
	type chWant struct {
		orders, cancelled int64
		money             []reporting.Money
	}
	wantCh := []struct {
		channel string
		w       chWant
	}{
		// facebook_live: O2 + O5 (manual order with a facebook origin goes to the origin channel) and the cancelled O6 (its bundle F6 is a facebook origin). Money: O2 captured 10001 LIVE; O5 offline transfer 8000.
		{"facebook_live", chWant{2, 1, []reporting.Money{{Environment: "LIVE", CapturedCount: 1, CapturedMinor: 10001, NetMinor: 10001, OfflineCount: 1, OfflineMinor: 8000}}}},
		// instagram_live: O3 only, SANDBOX card 30000. The deployment is LIVE, so the SANDBOX-paid order is not counted (orders 0); its money still shows, split by environment.
		{"instagram_live", chWant{0, 0, []reporting.Money{{Environment: "SANDBOX", CapturedCount: 1, CapturedMinor: 30000, NetMinor: 30000}}}},
		// storefront: orders created in range = O1, O8, O9 (O7 predates the range) -> 3 orders, none cancelled. Money in range: O1 30000 + O7 5000 + O8 100,
		// refund 9000 -> captured 35100 over 3 facts, net 26100 (O9 captured Oct 1 Taipei: out).
		{"storefront", chWant{3, 0, []reporting.Money{{Environment: "LIVE", CapturedCount: 3, CapturedMinor: 35100, RefundedMinor: 9000, NetMinor: 26100}}}},
		// manual: O4 only (no origin): COD collected 10000 + 5000 surcharge = 15000, never captured.
		{"manual", chWant{1, 0, []reporting.Money{{Environment: "LIVE", OfflineCount: 1, OfflineMinor: 15000}}}},
	}
	if len(ch.Rows) != len(wantCh) {
		t.Fatalf("channel rows: %+v", ch.Rows)
	}
	for i, wc := range wantCh {
		r := ch.Rows[i]
		if r.Channel != wc.channel || r.Currency != "TWD" || r.Orders != wc.w.orders || r.CancelledOrders != wc.w.cancelled || len(r.Money) != len(wc.w.money) {
			t.Errorf("channel %d: %+v want %s %+v", i, r, wc.channel, wc.w)
			continue
		}
		for j := range r.Money {
			if r.Money[j] != wc.w.money[j] {
				t.Errorf("channel %s money: %+v want %+v", wc.channel, r.Money[j], wc.w.money[j])
			}
		}
	}

	// ---- parity with the finance summary of the same range (RP01) ----
	var fin reporting.FinanceSummary
	finReq := httptest.NewRequest("GET", "/v1/admin/stores/"+w.store+"/finance/summary"+rpQ, nil)
	finReq.Header.Set("Authorization", "Bearer "+w.reader)
	rec := httptest.NewRecorder()
	w.h.ServeHTTP(rec, finReq)
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &fin) != nil {
		t.Fatalf("finance: %d %s", rec.Code, rec.Body.String())
	}
	type tot struct{ captured, refunded, net, offline int64 }
	finTot, chTot, prodTot := map[string]tot{}, map[string]tot{}, map[string]tot{}
	for _, r := range fin.Totals {
		finTot[r.Environment] = tot{r.CapturedMinor, r.RefundedMinor, r.NetMinor, r.PickupCollectedMinor + r.BankTransferConfirmedMinor + r.CodCollectedMinor}
	}
	for _, r := range ch.Rows {
		for _, m := range r.Money {
			x := chTot[m.Environment]
			chTot[m.Environment] = tot{x.captured + m.CapturedMinor, x.refunded + m.RefundedMinor, x.net + m.NetMinor, x.offline + m.OfflineMinor}
		}
	}
	for _, r := range prod.Rows {
		x := prodTot[r.Environment]
		prodTot[r.Environment] = tot{x.captured + r.CapturedMinor, x.refunded + r.RefundedMinor, x.net + r.NetMinor, x.offline + r.OfflineMinor}
	}
	if len(finTot) != 2 || finTot["LIVE"].net != liveNet || finTot["LIVE"].offline != 23000 {
		t.Fatalf("finance totals %+v (live product net %d)", finTot, liveNet)
	}
	for env, ft := range finTot {
		if chTot[env] != ft || prodTot[env] != ft {
			t.Errorf("%s: finance %+v channels %+v products %+v", env, ft, chTot[env], prodTot[env])
		}
	}

	// ---- manual orders: both merchant-created orders, one creator, no session; O5's facebook origin has the session s1 ----
	var man reporting.ManualReport
	w.must(w.reader, "manual-orders"+rpQ, &man)
	if len(man.Rows) != 2 {
		t.Fatalf("manual rows: %+v", man.Rows)
	}
	for _, r := range man.Rows {
		if r.PrincipalID == nil || *r.PrincipalID != w.staff || r.Orders != 1 || r.Currency != "TWD" || len(r.Money) != 1 {
			t.Errorf("manual row: %+v", r)
		}
	}
	// the one with a session is O5 (transfer 8000), the other O4 (COD 15000)
	var withSession, without int
	for _, r := range man.Rows {
		if r.SessionID != nil {
			withSession++
			if r.Money[0].OfflineMinor != 8000 {
				t.Errorf("session manual row: %+v", r)
			}
		} else {
			without++
			if r.Money[0].OfflineMinor != 15000 {
				t.Errorf("no-session manual row: %+v", r)
			}
		}
	}
	if withSession != 1 || without != 1 {
		t.Errorf("manual rows by session: %d with, %d without", withSession, without)
	}

	// ---- read-only (the brief's "只读"): no table the reports read changed ----
	for tb, n := range before {
		if got := countRows(t, w.f.owner, "SELECT count(*) FROM "+tb); got != n {
			t.Errorf("%s changed from %d to %d during reads", tb, n, got)
		}
	}
}

// TestReportRP04FunnelNestedPerSessionAndStore proves RP04: four nested counts, per session and whole store, rejected/out-of-range claims excluded,
// cancelled orders not "ordered", a session of another store is 404.
func TestReportRP04FunnelNestedPerSessionAndStore(t *testing.T) {
	w := rpNew(t)
	s1, s2, _ := rpSeedMain(w)
	var all, one, two reporting.FunnelReport
	w.must(w.reader, "funnel"+rpQ, &all)
	w.must(w.reader, "funnel"+rpQ+"&session_id="+s1, &one)
	w.must(w.reader, "funnel"+rpQ+"&session_id="+s2, &two)
	// s1 in range: F1..F5 claimed (5; F7 is August, F8 only a rejected event); links sent F1,F2,F3 (3); ordered F1 (O2), F2 (O5) (2); paid both (2);
	// F5 has O3 but no link sent -> ordered_without_link 1.
	if one.Claimed != 5 || one.LinkSent != 3 || one.Ordered != 2 || one.Paid != 2 || one.OrderedWithoutLink != 1 || one.SessionID == nil || *one.SessionID != s1 {
		t.Errorf("session s1: %+v", one)
	}
	// s2: F6 claim + link + a CANCELLED order -> ordered 0.
	if two.Claimed != 1 || two.LinkSent != 1 || two.Ordered != 0 || two.Paid != 0 || two.OrderedWithoutLink != 0 {
		t.Errorf("session s2: %+v", two)
	}
	if all.Claimed != 6 || all.LinkSent != 4 || all.Ordered != 2 || all.Paid != 2 || all.OrderedWithoutLink != 1 || all.SessionID != nil {
		t.Errorf("whole store: %+v", all)
	}
	for _, f := range []reporting.FunnelReport{all, one, two} {
		if f.Paid > f.Ordered || f.Ordered > f.LinkSent || f.LinkSent > f.Claimed {
			t.Errorf("funnel grows to the right: %+v", f)
		}
	}
	// A session that is not this store's: 404 with nothing disclosed (RP07 for the funnel).
	other := rpNew(t)
	otherSession := other.session("other store")
	status, body, _ := w.get(w.reader, "funnel"+rpQ+"&session_id="+otherSession)
	if status != 404 || strings.Contains(string(body), otherSession) {
		t.Errorf("foreign session: %d %s", status, body)
	}
	// The funnel needs live:read besides orders:read.
	if status, _, _ := w.get(w.ordersOnly, "funnel"+rpQ); status != 403 {
		t.Errorf("funnel without live:read = %d", status)
	}
	if status, _, _ := w.get(w.noOrder, "products"+rpQ); status != 403 {
		t.Errorf("products without orders:read = %d", status)
	}
}

// TestReportRP05RangeAndShape proves RP05: from>to, 93 days and malformed queries are 422 on all eight routes, 92 days is fine.
func TestReportRP05RangeAndShape(t *testing.T) {
	w := rpNew(t)
	for _, slug := range []string{"products", "channels", "funnel", "manual-orders", "products.csv", "channels.csv", "funnel.csv", "manual-orders.csv"} {
		for name, q := range map[string]string{
			"93 days":  "?from=2026-01-01&to=2026-04-03",
			"reversed": "?from=2026-09-02&to=2026-09-01",
			"no query": "",
			"bad date": "?from=2026-02-30&to=2026-03-01",
		} {
			token := w.reader
			if strings.HasSuffix(slug, ".csv") {
				token = w.exporter
			}
			if status, _, _ := w.get(token, slug+q); status != 422 {
				t.Errorf("%s %s = %d want 422", slug, name, status)
			}
		}
		token := w.reader
		if strings.HasSuffix(slug, ".csv") {
			token = w.exporter
		}
		if status, body, _ := w.get(token, slug+"?from=2026-01-01&to=2026-04-02"); status != 200 {
			t.Errorf("%s 92 days = %d %s", slug, status, body)
		}
	}
}

// TestReportRP06ExportPermissionAuditAndCSV proves RP06: no orders:export -> 403 and no audit row; with it one reports.export.<report> row per
// export, the CSV carries the same rows with the formula guard, a read writes no audit row.
func TestReportRP06ExportPermissionAuditAndCSV(t *testing.T) {
	w := rpNew(t)
	rpSeedMain(w)
	audits := func() int {
		return countRows(t, w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action LIKE 'reports.export.%'`, w.store)
	}
	for _, slug := range []string{"products", "channels", "funnel", "manual-orders"} {
		if status, _, _ := w.get(w.ordersOnly, slug+".csv"+rpQ); status != 403 {
			t.Errorf("%s.csv with orders:read only = %d", slug, status)
		}
	}
	if audits() != 0 {
		t.Fatal("a refused export wrote an audit row")
	}
	w.get(w.reader, "products"+rpQ)
	if audits() != 0 {
		t.Fatal("a read wrote an audit row")
	}
	for i, slug := range []string{"products", "channels", "funnel", "manual-orders"} {
		status, body, hdr := w.get(w.exporter, slug+".csv"+rpQ)
		if status != 200 || !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") || hdr.Get("Cache-Control") != "no-store" ||
			!strings.Contains(hdr.Get("Content-Disposition"), "attachment; filename=\"report-") {
			t.Fatalf("%s.csv: %d %v %s", slug, status, hdr, body)
		}
		if got := audits(); got != i+1 {
			t.Errorf("after %s export: %d audit rows, want %d", slug, got, i+1)
		}
		// the audit action names the report (manual-orders -> reports.export.manual_orders)
		if n := countRows(t, w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action=$2`, w.store, "reports.export."+strings.ReplaceAll(slug, "-", "_")); n != 1 {
			t.Errorf("%s: %d audit rows with its own action, want 1", slug, n)
		}
		rows, err := csv.NewReader(strings.NewReader(string(body))).ReadAll()
		if err != nil || len(rows) < 2 {
			t.Fatalf("%s csv: %v %q", slug, err, body)
		}
		if slug == "products" {
			if len(rows) != 5 { // header + LIVE A,B,C + SANDBOX B
				t.Errorf("products csv rows %d: %q", len(rows), body)
			}
			if !strings.Contains(string(body), "'=Alpha") || strings.Contains(string(body), ",=Alpha") {
				t.Errorf("formula guard missing: %q", body)
			}
		}
	}
	// the audit row names the exporting principal in this store
	if n := countRows(t, w.f.owner, `SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action LIKE 'reports.export.%' AND tenant_id=$2`, w.store, w.tenant); n != 4 {
		t.Errorf("audit rows %d", n)
	}
}

// TestReportRP07CrossStoreNoLeak proves RP07: another store's token gets 404 on this store's reports with none of its numbers, and each store's
// own report holds only its own money.
func TestReportRP07CrossStoreNoLeak(t *testing.T) {
	a, b := rpNew(t), rpNew(t)
	rpSeedMain(a)
	skuB := randomUUID()
	ob := b.order(rpAt(10, 9), "CONFIRMED", "storefront", "card", rpLine{skuB, randomUUID(), "ZZZ", "Zeta", 1, 123400})
	b.capture(ob, 123400, "LIVE", rpAt(10, 10))
	for _, slug := range []string{"products", "channels", "funnel", "manual-orders", "products.csv"} {
		// b's token against a's store
		r := httptest.NewRequest("GET", "/v1/admin/stores/"+a.store+"/reports/"+slug+rpQ, nil)
		r.Header.Set("Authorization", "Bearer "+b.exporter)
		rec := httptest.NewRecorder()
		a.h.ServeHTTP(rec, r)
		if rec.Code != 404 || strings.Contains(rec.Body.String(), "35100") || strings.Contains(rec.Body.String(), "23333") {
			t.Errorf("%s cross-store: %d %s", slug, rec.Code, rec.Body.String())
		}
	}
	var pb reporting.ProductReport
	b.must(b.reader, "products"+rpQ, &pb)
	if len(pb.Rows) != 1 || pb.Rows[0].SKUID != skuB || pb.Rows[0].NetMinor != 123400 {
		t.Errorf("store b products: %+v", pb.Rows)
	}
	var cb reporting.ChannelReport
	b.must(b.reader, "channels"+rpQ, &cb)
	if len(cb.Rows) != 1 || cb.Rows[0].Orders != 1 || cb.Rows[0].Money[0].NetMinor != 123400 {
		t.Errorf("store b channels: %+v", cb.Rows)
	}
}

// TestReportRP08TimingTenThousandOrders proves RP08 at a size above the brief: 10,000 synthetic orders over 200 days (about 4,600 of them in the
// 92-day window) with a captured fact each, a third with a facebook origin; every report must answer in under 5 s. The measured times are logged.
func TestReportRP08TimingTenThousandOrders(t *testing.T) {
	w := rpNew(t)
	// A second tenant's store with the same volume: the reports must stay selective with other tenants' rows in the same tables.
	noise := rpNewIn(t, "B", "LIVE")
	for _, x := range []*rpWorld{noise, w} {
		x.perfSeed()
	}
	const q = "?from=2026-06-01&to=2026-08-31" // 92 days
	rpPerfMeasure(t, w, q)
}

// perfSeed writes 10,000 synthetic orders over 200 days with a captured fact each and a facebook origin on every third.
func (w *rpWorld) perfSeed() {
	w.exec(`WITH g AS MATERIALIZED (
	  SELECT n,gen_random_uuid() AS oid,gen_random_uuid() AS aid,gen_random_uuid() AS bid,gen_random_uuid() AS owner,
	   timestamptz '2026-03-01 10:00:00+08' + (n % 200) * interval '1 day' + (n % 600) * interval '1 minute' AS at
	  FROM generate_series(1,10000) n
	 ), sku AS MATERIALIZED (SELECT i,gen_random_uuid() AS id FROM generate_series(0,39) i),
	 o AS (INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,quote_id,destination_id,market_id,country,service_code,service_version,
	   allocation_version,currency,total_minor,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at,payment_mode,source,collection_state)
	  SELECT $1,$2,g.owner,g.oid,gen_random_uuid(),gen_random_uuid(),1,gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'TW','std',1,1,'TWD',30000,'CONFIRMED','MANUAL_UNASSIGNED',1,
	   g.at+interval '10 minutes',$4::bigint+g.n,
	   jsonb_build_object('quote',jsonb_build_object('lines',jsonb_build_array(
	    jsonb_build_object('sku_id',(SELECT id FROM sku WHERE i=g.n%40),'product_id',gen_random_uuid(),'code','C','name','N','quantity',1,'amount',jsonb_build_object('total_minor',10000)),
	    jsonb_build_object('sku_id',(SELECT id FROM sku WHERE i=(g.n+7)%40),'product_id',gen_random_uuid(),'code','C','name','N','quantity',2,'amount',jsonb_build_object('total_minor',20000))))),
	   g.at,g.at,'card',CASE WHEN g.n%10=0 THEN 'merchant_manual' ELSE 'storefront' END,NULL FROM g RETURNING id),
	 a AS (INSERT INTO checkout.payment_attempts(tenant_id,store_id,owner_id,id,session_id,order_id,market_id,country,method_code,method_version,connection_id,credential_version,
	   qualification_id,environment,execution_profile,binding_id,binding_version,currency,amount_minor,merchant_trade_no,state,generation,job_id)
	  SELECT $1,$2,g.owner,g.aid,gen_random_uuid(),g.oid,gen_random_uuid(),'TW','payuni_credit',1,gen_random_uuid(),1,gen_random_uuid(),'LIVE','PROVIDER_MOCK',gen_random_uuid(),1,'TWD',100,
	   substr(replace(g.aid::text,'-',''),1,25),'PAYMENT_PENDING',2,$4::bigint+g.n FROM g RETURNING id),
	 f AS (INSERT INTO payments.facts(tenant_id,store_id,attempt_id,kind,amount_minor,currency,provider_reference,connection_id,execution_profile,environment,source_report_hash,received_at)
	  SELECT $1,$2,g.aid,'CAPTURED',30000,'TWD','ref_'||substr(replace(g.aid::text,'-',''),1,20),gen_random_uuid(),'PROVIDER_MOCK','LIVE',sha256('x'::bytea),g.at+interval '1 minute' FROM g RETURNING 1),
	 b AS (INSERT INTO claims.bundles(tenant_id,store_id,id,session_id,platform,actor_key)
	  SELECT $1,$2,g.bid,$3,'facebook',md5(g.bid::text)||md5(g.oid::text) FROM g WHERE g.n%3=0 RETURNING 1),
	 e AS (INSERT INTO claims.events(tenant_id,store_id,session_id,window_generation,source_kind,source_event_id,platform,occurred_at,grammar_version,grammar_kind,match_mode,
	   outcome,offer_id,quantity,explicit_quantity,bundle_id,line_version,bundle_version)
	  SELECT $1,$2,$3,1,'meta',gen_random_uuid(),'facebook',g.at,'kw-v1','MATCH','EXACT','ACCEPTED',gen_random_uuid(),1,true,g.bid,1,1 FROM g WHERE g.n%3=0 RETURNING 1)
	 INSERT INTO claims.order_origins(tenant_id,store_id,order_id,bundle_id,offer_id,line_version,session_id,occurred_at)
	  SELECT $1,$2,g.oid,g.bid,gen_random_uuid(),1,$3,g.at FROM g WHERE g.n%3=0`, w.tenant, w.store, w.session("perf"), rpJob.Add(20000)-19999) // reserves job ids [base, base+10000] for this seed
	w.exec(`ANALYZE checkout.orders`)
	w.exec(`ANALYZE payments.facts`)
	w.exec(`ANALYZE claims.order_origins`)
	w.exec(`ANALYZE claims.events`)
}

func rpPerfMeasure(t *testing.T, w *rpWorld, q string) {
	t.Helper()
	for _, slug := range []string{"products", "channels", "funnel", "manual-orders"} {
		start := time.Now()
		status, body, _ := w.get(w.reader, slug+q)
		took := time.Since(start)
		t.Logf("RP08 %-13s 92 days over 10000 orders: %v (status %d, %d bytes)", slug, took.Round(time.Millisecond), status, len(body))
		if status != 200 || took > 5*time.Second {
			t.Errorf("%s: status %d in %v (limit 5s) %.200s", slug, status, took, body)
		}
	}
	var ch reporting.ChannelReport
	w.must(w.reader, "channels"+q, &ch)
	var orders int64
	for _, r := range ch.Rows {
		orders += r.Orders
	}
	if orders < 4000 || orders > 5200 { // 10000 orders * 92/200 days
		t.Errorf("orders in the window: %d", orders)
	}
}

// TestReportEnvironmentProfiles proves the deployment-environment rule (LC-B7 A1): counts (funnel paid, channel / manual-order orders and
// cancelled_orders) follow the deployment payment environment, money stays split per (currency, environment) and is the same under both profiles.
func TestReportEnvironmentProfiles(t *testing.T) {
	for _, env := range []string{"LIVE", "SANDBOX"} {
		t.Run(env, func(t *testing.T) {
			w := rpNewIn(t, "A", env)
			s1, _, _ := rpSeedMain(w)
			var ch reporting.ChannelReport
			w.must(w.reader, "channels"+rpQ, &ch)
			got := map[string][2]int64{}
			for _, r := range ch.Rows {
				got[r.Channel] = [2]int64{r.Orders, r.CancelledOrders}
			}
			// An order is in the environment of its payment attempt, else in the deployment's (offline modes, unpaid).
			// LIVE: facebook O2 (LIVE) + O5 (no attempt) = 2 and the cancelled O6 = 1; instagram O3 is SANDBOX = 0; storefront O1,O8,O9 = 3; manual O4 = 1.
			// SANDBOX: facebook only O5 = 1 and O6 cancelled = 1; instagram O3 = 1; storefront 0 (O1,O8,O9 are LIVE); manual O4 = 1.
			want := map[string][2]int64{"facebook_live": {2, 1}, "instagram_live": {0, 0}, "storefront": {3, 0}, "manual": {1, 0}}
			if env == "SANDBOX" {
				want = map[string][2]int64{"facebook_live": {1, 1}, "instagram_live": {1, 0}, "storefront": {0, 0}, "manual": {1, 0}}
			}
			for ch, wv := range want {
				if got[ch] != wv {
					t.Errorf("%s deployment, channel %s orders/cancelled = %v, want %v", env, ch, got[ch], wv)
				}
			}
			// Money is identical under both profiles: LIVE 45101 captured over the LIVE rows, 30000 SANDBOX.
			var live, sandbox int64
			for _, r := range ch.Rows {
				for _, m := range r.Money {
					if m.Environment == "LIVE" {
						live += m.CapturedMinor
					} else {
						sandbox += m.CapturedMinor
					}
				}
			}
			if live != 45101 || sandbox != 30000 {
				t.Errorf("%s deployment money split: LIVE %d SANDBOX %d", env, live, sandbox)
			}
			// Funnel paid: F1's order O2 is paid by a LIVE capture (counts only on a LIVE deployment), F2's O5 by a bank transfer (always).
			var f reporting.FunnelReport
			w.must(w.reader, "funnel"+rpQ+"&session_id="+s1, &f)
			wantPaid := int64(2)
			if env == "SANDBOX" {
				wantPaid = 1
			}
			if f.Claimed != 5 || f.LinkSent != 3 || f.Ordered != 2 || f.Paid != wantPaid {
				t.Errorf("%s deployment funnel: %+v want paid %d", env, f, wantPaid)
			}
		})
	}
	// Manual orders: a LIVE-paid and a SANDBOX-paid merchant-created order; only the deployment's one is counted, both money rows show.
	for _, env := range []string{"LIVE", "SANDBOX"} {
		w := rpNewIn(t, "A", env)
		sku := randomUUID()
		for _, e := range []string{"LIVE", "SANDBOX"} {
			o := w.order(rpAt(20, 9), "CONFIRMED", "merchant_manual", "card", rpLine{sku, randomUUID(), "M", "M", 1, 100})
			w.manual(o, w.staff)
			w.capture(o, 1000, e, rpAt(20, 10))
		}
		var man reporting.ManualReport
		w.must(w.reader, "manual-orders"+rpQ, &man)
		if len(man.Rows) != 1 || man.Rows[0].Orders != 1 || len(man.Rows[0].Money) != 2 {
			t.Errorf("%s deployment manual orders: %+v", env, man.Rows)
		}
	}
}

// TestReportInternalHelpersAreNotCallable pins identity.report_open / report_money_events: not SECURITY DEFINER, owned by commerce_auth, no
// EXECUTE for PUBLIC or any other role (they run only inside the report definers), while the five report definers are SECURITY DEFINER with
// EXECUTE for commerce_runtime only.
func TestReportInternalHelpersAreNotCallable(t *testing.T) {
	f := fixture(t)
	for _, sig := range []string{"identity.report_open(bytea,uuid,date,date,text,text)", "identity.report_money_events(uuid,uuid,timestamp with time zone,timestamp with time zone)"} {
		var owner string
		var definer bool
		if err := f.owner.QueryRow(context.Background(), `SELECT pg_get_userbyid(proowner),prosecdef FROM pg_proc WHERE oid=$1::regprocedure`, sig).Scan(&owner, &definer); err != nil {
			t.Fatalf("%s: %v", sig, err)
		}
		if owner != "commerce_auth" || definer {
			t.Errorf("%s owner=%s definer=%v, want commerce_auth and invoker rights", sig, owner, definer)
		}
		if n := countRows(t, f.owner, `SELECT count(*) FROM pg_proc p, aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE p.oid=$1::regprocedure AND a.privilege_type='EXECUTE' AND a.grantee<>p.proowner`, sig); n != 0 {
			t.Errorf("%s has %d EXECUTE grantees besides its owner (PUBLIC included)", sig, n)
		}
		for _, role := range []string{"commerce_runtime", "commerce_buyer_runtime", "commerce_claims_writer", "commerce_checkout_writer"} {
			if countRows(t, f.owner, `SELECT CASE WHEN has_function_privilege($1,$2::regprocedure,'EXECUTE') THEN 1 ELSE 0 END`, role, sig) != 0 {
				t.Errorf("%s can execute %s", role, sig)
			}
		}
	}
	for _, sig := range []string{"identity.read_report_products(bytea,uuid,date,date)", "identity.read_report_channels(bytea,uuid,date,date,text)",
		"identity.read_report_manual_orders(bytea,uuid,date,date,text)", "identity.read_report_funnel(bytea,uuid,date,date,uuid,text)",
		"identity.export_report(bytea,uuid,text,date,date,uuid,text)"} {
		if countRows(t, f.owner, `SELECT CASE WHEN has_function_privilege('commerce_runtime',$1::regprocedure,'EXECUTE') AND (SELECT prosecdef FROM pg_proc WHERE oid=$1::regprocedure) THEN 1 ELSE 0 END`, sig) != 1 {
			t.Errorf("%s is not a SECURITY DEFINER callable by commerce_runtime", sig)
		}
	}
}
