package foundation_test

// ops-polish independent gates OP1 (honest card option) and OP3 (pay-at-pickup money in finance), written from
// docs/delivery/units/ops-polish.md and the contract sentences it amends (buyer-checkout-options-v1 OP1 rule, customers-billing-v1 BD7),
// not from the implementation. Prefix `opp`. Tier REAL_PG + HTTP_PG; MOCK ECPay (ecpaytest fake) and MOCK Stripe (rfx fake).
// Owner-pool writes (disclosed fixtures): identity grants (tcvEnv), and aging checkout.orders.updated_at / payments.facts.received_at /
// fulfillment.cvs_shipments.environment with session_replication_role=replica so a day boundary can be placed exactly; the order states
// themselves (COLLECTED, REFUNDED_OFFLINE, captured) come from the real merchant routes, signed ECPay status posts and the Stripe capture path.
//
// OP1 here is the in-process half (checkout.Service.WithoutCardPayment, the one switch cmd/api turns on when COMMERCE_BUYER_PAYMENT_ENABLED
// is off). The wiring of that switch in the real cmd/api assembly is TestOpsPolishOP1APIAssembly (child go test, tag buyerintegration).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/fulfillment"
	"livecommerce/internal/storefront"
)

// oppBegin posts the buyer checkout route of h for the buyer's current cart/destination/quote.
func oppBegin(t *testing.T, h bhHarness, b *tcvBuyer, quoteID, destID string, sv, av int64, mode, key string) bhResponse {
	t.Helper()
	body := map[string]any{"quote_id": quoteID, "destination_id": destID, "cart_version": b.cartVersion(), "service_version": sv, "allocation_version": av}
	if mode != "-" { // "-" omits the field: the default mode
		body["payment_mode"] = mode
	}
	return h.request(t, "POST", "/v1/buyer/checkout", b.cap.Token, key, body, nil)
}

// oppPrepare places a buyer on a buyer-entered pickup of the service `code` and returns destination and quote.
func oppPrepare(t *testing.T, e *tcvEnv, code string, items ...storefront.Item) (*tcvBuyer, storefront.Destination, storefront.Quote) {
	t.Helper()
	b := e.newBuyer(items...)
	pickup := e.tppEntered(b, code)
	dest, err := b.destination("cvs_711", pickup, tppName, tppPhone)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := b.quote(code)
	if err != nil {
		t.Fatal(err)
	}
	return b, dest, quote
}

func oppModes(rows map[string]map[string]any, kind string) []string {
	r := rows[kind]
	if r == nil {
		return nil
	}
	return tcbModes(r)
}

func TestOpsPolishOP1CardOff(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("orders:read", "fulfillment:write")
	var code string
	var sv, av int64
	for _, kind := range []string{"cvs_711", "cvs_familymart", "cvs_hilife", "cvs_okmart"} {
		c, s, a := e.service(kind, "MANUAL", 0)
		if kind == "cvs_711" {
			code, sv, av = c, s, a
		}
	}
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	market := e.p.market.ID
	off := e.tcbServe(e.svc.WithoutCardPayment()) // what cmd/api serves when COMMERCE_BUYER_PAYMENT_ENABLED is off
	probe := e.newBuyer()

	t.Run("control: with card payment on, home and CVS rows still offer card", func(t *testing.T) {
		rows := tcbOptions(t, e.bh, probe.cap.Token, market)
		if rows["home"] == nil {
			t.Fatalf("control: the home option must be listed while card is payable, got %v", rows)
		}
		if m := oppModes(rows, "cvs_711"); len(m) != 2 || m[0] != "card" || m[1] != "pay_at_pickup" {
			t.Errorf("control: cvs_711 payment_modes %v, want [card pay_at_pickup]", m)
		}
	})

	t.Run("card off: options drop card, home rows absent, CVS rows pay_at_pickup only", func(t *testing.T) {
		rows := tcbOptions(t, off, probe.cap.Token, market)
		if rows["home"] != nil {
			t.Errorf("a home option is card-only and must not be offered without card payment: %v", rows["home"])
		}
		cvs := 0
		for kind, r := range rows {
			if kind == "home" {
				continue
			}
			cvs++
			if m := tcbModes(r); len(m) != 1 || m[0] != "pay_at_pickup" {
				t.Errorf("%s payment_modes %v, want exactly [pay_at_pickup]", kind, m)
			}
		}
		if cvs != 4 {
			t.Errorf("want the four enabled chains still listed, got %d", cvs)
		}
	})

	t.Run("card off and pay-at-pickup off: no option is offered at all", func(t *testing.T) {
		e.cvsSettings(tcvAllChains, false, "null", 500)
		defer e.cvsSettings(tcvAllChains, true, "20000", 500)
		rows := tcbOptions(t, off, probe.cap.Token, market)
		if len(rows) != 0 {
			t.Errorf("neither card nor pay-at-pickup can be completed, so nothing may be offered: %v", rows)
		}
		if rows := tcbOptions(t, e.bh, probe.cap.Token, market); rows["cvs_711"] == nil || rows["home"] == nil {
			t.Errorf("control: with card on the same store still offers its rows: %v", rows)
		}
	})

	t.Run("Begin with card is refused 422 card_unavailable with zero effects (explicit and default mode)", func(t *testing.T) {
		for _, mode := range []string{"card", "-"} {
			b, dest, quote := oppPrepare(t, e, code)
			h0, a0, o0 := e.tcbEffects(b.cap.Scope.OwnerID)
			res := oppBegin(t, off, b, quote.ID, dest.ID, sv, av, mode, t04Key("opp-card-"+mode))
			if res.status != 422 || tcvStr(tcvJSON(t, res.body), "code") != "card_unavailable" {
				t.Errorf("mode %q: want 422 card_unavailable, got %d %s", mode, res.status, res.body)
			}
			if h, a, o := e.tcbEffects(b.cap.Scope.OwnerID); h != h0 || a != a0 || o != o0 {
				t.Errorf("mode %q: a refused card Begin left rows: holds/attempts/orders %d/%d/%d -> %d/%d/%d", mode, h0, a0, o0, h, a, o)
			}
			// control: the same request on the card-capable service is accepted (the refusal is the switch, not the fixture)
			if mode == "card" {
				ok := oppBegin(t, e.bh, b, quote.ID, dest.ID, sv, av, mode, t04Key("opp-card-ctl"))
				if ok.status != 200 {
					t.Errorf("control: card Begin on the card-capable service: %d %s", ok.status, ok.body)
				}
			}
		}
	})

	t.Run("pay_at_pickup Begin still succeeds with card off", func(t *testing.T) {
		b, dest, quote := oppPrepare(t, e, code)
		res := oppBegin(t, off, b, quote.ID, dest.ID, sv, av, "pay_at_pickup", t04Key("opp-pap"))
		if res.status != 200 {
			t.Fatalf("pay_at_pickup with card off: %d %s", res.status, res.body)
		}
		id := tcvStr(tcvJSON(t, res.body), "order_id")
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND payment_mode='pay_at_pickup' AND collection_state='PENDING'`, id); n != 1 {
			t.Errorf("the order %q is not a PENDING pay-at-pickup order", id)
		}
	})

	t.Run("exact replay of an already placed card order still returns that order", func(t *testing.T) {
		b, dest, quote := oppPrepare(t, e, code)
		key := t04Key("opp-replay")
		first := oppBegin(t, e.bh, b, quote.ID, dest.ID, sv, av, "card", key) // placed while card was payable
		if first.status != 200 {
			t.Fatalf("setup: card Begin on the card-capable service: %d %s", first.status, first.body)
		}
		want := tcvStr(tcvJSON(t, first.body), "order_id")
		_, _, o0 := e.tcbEffects(b.cap.Scope.OwnerID)
		again := oppBegin(t, off, b, quote.ID, dest.ID, sv, av, "card", key) // the process was restarted without a payment service
		if again.status != 200 || tcvStr(tcvJSON(t, again.body), "order_id") != want {
			t.Errorf("an exact replay must return order %s, got %d %s", want, again.status, again.body)
		}
		if _, _, o := e.tcbEffects(b.cap.Scope.OwnerID); o != o0 {
			t.Errorf("the replay created an order: %d -> %d", o0, o)
		}
	})
}

// TestOpsPolishOP1APIAssembly runs the real cmd/api assembly (buildBuyerHandler over the isolated roles) twice, with
// COMMERCE_BUYER_PAYMENT_ENABLED=1 and =0, in a child `go test -tags buyerintegration ./cmd/api`. Parent fixture: a buyer with a quoted
// home order; child: options, card Begin, replay over the real buyer HTTP handler.
func TestOpsPolishOP1APIAssembly(t *testing.T) {
	h := bhSetup(t)
	hostedDSN := hpRole(t, h.f)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	// a quoted home order for the buyer h.cap (as TestBuyerHTTPOptionsDriveHomeCheckout does)
	var row boptItem
	// options are paginated and the shared fixture store accumulates markets across the suite: page like the sibling test
	for query, pages := "", 0; row.MarketID == "" && pages < 100; pages++ {
		page := boptRead(t, h, query)
		for _, candidate := range page.Items {
			if candidate.MarketCode == h.market.Code {
				row = candidate
			}
		}
		if page.NextCursor == "" {
			break
		}
		query = "cursor=" + url.QueryEscape(page.NextCursor)
	}
	if row.MarketID == "" {
		t.Fatal("no home option in the fixture")
	}
	c := bhRead[storefront.Cart](t, h.request(t, "GET", "/v1/buyer/cart", h.cap.Token, "", nil, nil), 200)
	q := bhRead[storefront.Quote](t, h.request(t, "POST", "/v1/buyer/quotes", h.cap.Token, t04Key("opp-quote"), storefront.QuoteInput{CartVersion: c.Version, MarketID: row.MarketID, Country: row.Country, Method: row.Method}, nil), 200)
	address := bdHome(c)
	address.ExpectedVersion = h.destination.Version
	address.Country, address.Kind = row.Country, row.DeliveryKind
	d := bhRead[storefront.Destination](t, h.request(t, "PUT", "/v1/buyer/destination", h.cap.Token, t04Key("opp-address"), address, nil), 200)
	body, _ := json.Marshal(map[string]any{"quote_id": q.ID, "destination_id": d.ID, "cart_version": c.Version, "service_version": row.ServiceVersion, "allocation_version": row.AllocationVersion})

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-tags", "buyerintegration", "-count=1", "-run", "^TestBuyerNoCardAssemblyRealPG$", "-v", "./cmd/api")
	cmd.Dir = root
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "COMMERCE_") || strings.HasPrefix(name, "LC_BUYER_") || strings.HasPrefix(name, "LC_OPP_") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "LC_BUYER_NOCARD_GATE=1",
		"LC_BUYER_TEST_ISSUER_DSN="+h.a.issuerURL, "LC_BUYER_TEST_RUNTIME_DSN="+h.a.runtimeURL, "LC_BUYER_TEST_CHECKOUT_DSN="+h.poolURL, "LC_BUYER_TEST_HOSTED_DSN="+hostedDSN,
		"LC_OPP_TOKEN="+h.cap.Token, "LC_OPP_ORIGIN="+h.origin, "LC_OPP_MARKET="+row.MarketID, "LC_OPP_COUNTRY="+row.Country, "LC_OPP_BODY="+string(body))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("no-card assembly child failed: %v\n%s", err, out.String())
	}
	for _, want := range []string{"--- PASS: TestBuyerNoCardAssemblyRealPG/payment_on_lists_home_and_accepts_card", "--- PASS: TestBuyerNoCardAssemblyRealPG/payment_off_drops_home_and_refuses_card", "--- PASS: TestBuyerNoCardAssemblyRealPG/replay_after_restart_without_payment"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("child did not execute %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "--- SKIP") {
		t.Errorf("a skipped child case is never PASS:\n%s", out.String())
	}
}

// ---- OP3 ------------------------------------------------------------------------------------------------------------------------

func TestOpsPolishOP3Finance(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	f := e.p.f
	ctx := context.Background()
	e.r.startWorker(t)
	e.grantCreator("orders:read", "orders:export", "fulfillment:write", "integration:manage", "integration:read")
	e.cvsSettings(tcvAllChains, true, "20000", 500)
	manual, _, _ := e.service("cvs_711", "MANUAL", 0)

	age := func(q string, args ...any) {
		t.Helper()
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	setUpdated := func(order, ts string) {
		t.Helper()
		age(`UPDATE checkout.orders SET updated_at=$2::timestamptz WHERE id=$1`, order, ts)
	}
	total := func(order string) int64 {
		var n int64
		if err := f.owner.QueryRow(ctx, `SELECT total_minor FROM checkout.orders WHERE id=$1`, order).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// ---- no cvs_shipments row (buyer-entered pickup + manual shipment): collected through the real merchant route ----
	seq := 0
	manualCollected := func() string {
		t.Helper()
		b := e.newBuyer()
		res, err := e.tppPlace(b, manual, e.tppEntered(b, manual), tppName, tppPhone)
		if err != nil {
			t.Fatalf("place: %v", err)
		}
		seq++
		if st, _, raw := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+res.OrderID+"/shipment", t04Key("opp-ms"), mfxShip(0, "seven_eleven_cvs", fmt.Sprintf("00%08d", 12345+seq))); st != 200 {
			t.Fatalf("manual shipment: %d %s", st, raw)
		}
		if st, _, raw := e.record(e.token(), res.OrderID, t04Key("opp-collect"), "PENDING", "collected"); st != 200 {
			t.Fatalf("collected: %d %s", st, raw)
		}
		return res.OrderID
	}
	m1, m2, m3, m5 := manualCollected(), manualCollected(), manualCollected(), manualCollected()
	if st, _, raw := e.record(e.token(), m3, t04Key("opp-refoff"), "COLLECTED", "refunded_offline"); st != 200 || e.collectionState(m3) != "REFUNDED_OFFLINE" {
		t.Fatalf("refunded_offline: %d %s", st, raw)
	}
	bPending := e.newBuyer()
	pendRes, err := e.tppPlace(bPending, manual, e.tppEntered(bPending, manual), tppName, tppPhone)
	if err != nil {
		t.Fatal(err)
	}
	m4 := pendRes.OrderID // placed, never collected

	// ---- cvs_shipments rows (ECPay API service): collected through signed status posts ----
	e.connect("C2C")
	e.startDispatcher()
	e.updateService(manual, func(in *fulfillment.ServiceInput) { in.Enabled = false })
	api711, _, _ := e.service("cvs_711", "API", 0)
	endpoint := e.endpointID()
	apiCollected := func() string {
		t.Helper()
		order, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: api711, paymentMode: "pay_at_pickup"})
		if st, _, raw := e.ship(e.token(), order, 0, "", true); st != 202 {
			t.Fatalf("request shipment: %d %s", st, raw)
		}
		e.awaitShip(order, "CREATED")
		e.tppStatuses(endpoint, order, "2030", "2073", "2067")
		if e.collectionState(order) != "COLLECTED" {
			t.Fatalf("collection %s, want COLLECTED", e.collectionState(order))
		}
		return order
	}
	s1, s2 := apiCollected(), apiCollected()
	if env := e.count(`SELECT count(*) FROM fulfillment.cvs_shipments WHERE order_id IN ($1,$2) AND environment='SANDBOX'`, s1, s2); env != 2 {
		t.Fatalf("setup: both API orders should carry a SANDBOX shipment row, got %d", env)
	}
	// the second API order's connection is LIVE (a disclosed fixture: the fake connection is SANDBOX); the row decides the environment
	age(`UPDATE fulfillment.cvs_shipments SET environment='LIVE' WHERE order_id=$1`, s2)
	// ---- a paid card order: real capture, its own money path ----
	card, _ := e.cvsOrder(tcvOrderSpec{kind: "cvs_711", code: api711})

	tm := func(o string) int64 { return total(o) }
	if tm(m1) <= 0 || tm(s1) <= 0 {
		t.Fatal("setup: zero totals")
	}

	// ---- place every transition on an exact UTC+8 boundary (instants are UTC) ----
	setUpdated(m5, "2026-01-14 15:59:59+00") // 2026-01-14 23:59:59 Taipei
	setUpdated(m1, "2026-01-15 15:59:59+00") // 2026-01-15 23:59:59 Taipei: last second of the 15th
	setUpdated(m2, "2026-01-15 16:00:00+00") // 2026-01-16 00:00:00 Taipei: first second of the 16th
	setUpdated(m3, "2026-01-16 04:00:00+00") // REFUNDED_OFFLINE: must never count
	setUpdated(m4, "2026-01-16 04:00:00+00") // PENDING: must never count
	setUpdated(s1, "2026-01-16 15:59:59+00") // 2026-01-16 23:59:59 Taipei
	setUpdated(s2, "2026-01-16 16:00:00+00") // 2026-01-17 00:00:00 Taipei
	setUpdated(card, "2026-01-16 02:00:00+00")
	age(`UPDATE payments.facts SET received_at=timestamptz '2026-01-16 02:00:00+00' WHERE kind='CAPTURED' AND attempt_id IN (SELECT id FROM checkout.payment_attempts WHERE order_id=$1)`, card)
	cardTotal := tm(card)

	fin := func(from, to string) (map[string]any, []map[string]any, []map[string]any) {
		t.Helper()
		st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
		if st != 200 {
			t.Fatalf("finance %s..%s: %d %s", from, to, st, raw)
		}
		list := func(k string) (rows []map[string]any) {
			for _, r := range out[k].([]any) {
				rows = append(rows, r.(map[string]any))
			}
			return rows
		}
		return out, list("rows"), list("totals")
	}
	find := func(rows []map[string]any, day, env string) map[string]any {
		for _, r := range rows {
			if r["day"] == day && r["environment"] == env {
				return r
			}
		}
		return nil
	}
	num := func(r map[string]any, k string) float64 {
		if r == nil {
			return -1
		}
		v, _ := r[k].(float64)
		return v
	}

	t.Run("daily rows: both environment branches, Taipei boundaries, separate from captured and net", func(t *testing.T) {
		_, rows, totals := fin("2026-01-14", "2026-01-17")
		// m5 14th LIVE (no shipment row), m1 15th LIVE, m2 16th LIVE (no row), s1 16th SANDBOX (row), s2 17th LIVE (row flipped LIVE)
		want := []struct {
			day, env string
			n, minor float64
		}{
			{"2026-01-14", "LIVE", 1, float64(tm(m5))},
			{"2026-01-15", "LIVE", 1, float64(tm(m1))},
			{"2026-01-16", "LIVE", 1, float64(tm(m2))},
			{"2026-01-16", "SANDBOX", 1, float64(tm(s1))},
			{"2026-01-17", "LIVE", 1, float64(tm(s2))},
		}
		for _, w := range want {
			r := find(rows, w.day, w.env)
			if r == nil || num(r, "pickup_collected_count") != w.n || num(r, "pickup_collected_minor") != w.minor {
				t.Errorf("%s %s: row %v, want pickup_collected_count=%v pickup_collected_minor=%v", w.day, w.env, r, w.n, w.minor)
			}
			if r != nil && w.env == "LIVE" && (num(r, "captured_minor") != 0 || num(r, "net_minor") != 0 || num(r, "captured_count") != 0) {
				t.Errorf("%s %s: pay-at-pickup money leaked into captured/net: %v", w.day, w.env, r)
			}
		}
		// the paid card order is the day's captured money; the SANDBOX pickup beside it never enters captured or net
		sb := find(rows, "2026-01-16", "SANDBOX")
		if num(sb, "captured_count") != 1 || num(sb, "captured_minor") != float64(cardTotal) || num(sb, "net_minor") != float64(cardTotal) {
			t.Errorf("2026-01-16 SANDBOX captured/net must be exactly the card capture %d: %v", cardTotal, sb)
		}
		if len(rows) != 5 {
			t.Errorf("want exactly 5 rows (REFUNDED_OFFLINE and PENDING orders add none), got %d: %v", len(rows), rows)
		}
		// no other day/env carries any pickup money
		sum := 0.0
		for _, r := range rows {
			sum += num(r, "pickup_collected_minor")
		}
		if sum != float64(tm(m5)+tm(m1)+tm(m2)+tm(s1)+tm(s2)) {
			t.Errorf("pickup minor over all rows %v, want the five collected orders only", sum)
		}
		live, sandbox := find(totals, "", "LIVE"), find(totals, "", "SANDBOX")
		if num(live, "pickup_collected_count") != 4 || num(sandbox, "pickup_collected_count") != 1 || num(sandbox, "captured_minor") != float64(cardTotal) || num(live, "captured_minor") != 0 {
			t.Errorf("totals per environment: LIVE %v SANDBOX %v", live, sandbox)
		}
	})

	t.Run("day range edges follow UTC+8: 15th excludes the 16:00Z instant, 16th includes it", func(t *testing.T) {
		_, rows, _ := fin("2026-01-15", "2026-01-15")
		if len(rows) != 1 || rows[0]["day"] != "2026-01-15" || num(rows[0], "pickup_collected_count") != 1 {
			t.Errorf("one-day range 15th: %v (23:59:59 Taipei belongs to the 15th, 00:00:00 to the 16th)", rows)
		}
		_, rows, _ = fin("2026-01-16", "2026-01-16")
		if find(rows, "2026-01-16", "LIVE") == nil || find(rows, "2026-01-16", "SANDBOX") == nil || len(rows) != 2 {
			t.Errorf("one-day range 16th: %v", rows)
		}
		_, rows, _ = fin("2026-01-17", "2026-01-17")
		if len(rows) != 1 || num(rows[0], "pickup_collected_count") != 1 {
			t.Errorf("one-day range 17th must hold only the 00:00:00 Taipei order: %v", rows)
		}
	})

	t.Run("REFUNDED_OFFLINE leaves the collected column; PENDING never entered it", func(t *testing.T) {
		if st, _, raw := e.record(e.token(), m1, t04Key("opp-refoff2"), "COLLECTED", "refunded_offline"); st != 200 {
			t.Fatalf("refunded_offline: %d %s", st, raw)
		}
		setUpdated(m1, "2026-01-15 15:59:59+00") // the transition touched updated_at; put it back so only the state differs
		_, rows, _ := fin("2026-01-15", "2026-01-15")
		if len(rows) != 0 {
			t.Errorf("a REFUNDED_OFFLINE order must not count even on its old day: %v", rows)
		}
		for _, order := range []string{m3, m4} {
			setUpdated(order, "2026-01-15 04:00:00+00")
		}
		if _, rows, _ := fin("2026-01-15", "2026-01-15"); len(rows) != 0 {
			t.Errorf("REFUNDED_OFFLINE / PENDING orders appear after being moved into the range: %v", rows)
		}
	})

	t.Run("CSV carries the two columns for the same rows (HTTP export route, audited)", func(t *testing.T) {
		audits := e.audit("finance.exported")
		st, _, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary.csv?from=2026-01-14&to=2026-01-17", "", "")
		if st != 200 {
			t.Fatalf("csv: %d %s", st, raw)
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\r\n"), "\n")
		if lines[0] != "day,currency,environment,captured_count,captured_minor,refunded_minor,net_minor,pickup_collected_count,pickup_collected_minor,bank_transfer_confirmed_count,bank_transfer_confirmed_minor,cod_collected_count,cod_collected_minor" {
			t.Fatalf("csv header %q", lines[0])
		}
		got := map[string]string{}
		for _, l := range lines[1:] {
			fields := strings.Split(strings.TrimSpace(l), ",")
			if len(fields) != 13 {
				t.Fatalf("csv row %q has %d columns, want 13", l, len(fields))
			}
			got[fields[0]+"/"+fields[2]] = fields[7] + "/" + fields[8]
		}
		want := map[string]string{
			"2026-01-14/LIVE":    fmt.Sprintf("1/%d", tm(m5)),
			"2026-01-16/LIVE":    fmt.Sprintf("1/%d", tm(m2)),
			"2026-01-16/SANDBOX": fmt.Sprintf("1/%d", tm(s1)),
			"2026-01-17/LIVE":    fmt.Sprintf("1/%d", tm(s2)),
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("csv %s pickup columns %q, want %q (all rows: %v)", k, got[k], v, got)
			}
		}
		if _, ok := got["2026-01-15/LIVE"]; ok { // m1 was refunded offline in the previous subtest
			t.Errorf("csv still lists the REFUNDED_OFFLINE day: %v", got)
		}
		if n := e.audit("finance.exported"); n != audits+1 {
			t.Errorf("export audit rows %d -> %d, want one more", audits, n)
		}
	})
}
