package foundation_test

// PGT01-PGT11 (contracts/storefront-v2.md §F, unit promotions, migration 0091): the INDEPENDENT acceptance gates, written by the test author
// from the contract, not from the implementation or the implementer's author smoke (promotions_smoke_test.go, TestPromotion*). Evidence label of
// every passing line: REAL_PG through the real merchant HTTP handler, the real buyer HTTP handler and checkout.Begin; Stripe lines are MOCK
// (stripetest fake, signed webhook, real capture path); nothing here is SANDBOX or LIVE. Prefix `TestPromoGate`.
//   PGT01 TestPromoGateMath             percent/fixed boundaries, TWD whole-dollar floor, never below zero, line allocation sums
//   PGT02 TestPromoGateWindowAndMinimum minimum subtotal is inclusive and compares the PRE-discount subtotal; Taipei (+08:00) window crossings
//   PGT03 TestPromoGateShipping         a code never discounts shipping; the free-shipping threshold compares the PRE-discount subtotal
//   PGT04 TestPromoGateOneCode          one code per order (no stacking, no list), the client never supplies an amount
//   PGT05 TestPromoGateBeginErrors      used-up / expired / edited between quote and Begin: coded 422 through the real buyer route, zero facts
//   PGT06 TestPromoGateConcurrency      N racing placements: total_limit K admits exactly K, per_buyer_limit 1 admits exactly one identity
//   PGT07 TestPromoGateBankTransfer     per-buyer identity by e-mail/phone, discounted bank-transfer money to the merchant order view and finance
//   PGT08 TestPromoGateRelease          bank-transfer expiry frees a use; a confirmed/refunded transfer order keeps counting
//   PGT09 TestPromoGateStripeRefundCap  card order paid through the real capture path: captured = discounted total, refund cap, Stripe amount
//   PGT10 TestPromoGateAdminAuthority   401/403 matrix, the five staff role bundles (fulfilment cannot create codes), audit
//   PGT11 TestPromoGateIsolation        tenant and store isolation of codes, admin routes and buyer redemption
// Owner-pool writes (disclosed fixtures, as in the other gates): aging an order for expire_held (cofAge/proAge), identity grants/members, and reading
// result tables. Codes, quotes, placements, confirmations and refunds go through the real routes.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/checkout"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/storefront"
)

var pgTaipei = time.FixedZone("TPE", 8*3600)

// pgService adds a delivery service (home) with its own policy: flat shipping fee and an optional free-shipping threshold; returns its code.
func (e *tcvEnv) pgService(fee int64, threshold *int64) string {
	e.t.Helper()
	in := e.p.delivery
	in.Code, in.ExpectedVersion = "pgx"+t04Tag(), 0
	pol := e.p.policy
	pol.Method, pol.ExpectedVersion, pol.ShippingMinor, pol.Enabled, pol.FreeShippingThresholdMinor = "delivery:"+in.Code, 0, &fee, true, threshold
	if _, err := e.p.setPolicy(pol); err != nil {
		e.t.Fatalf("set policy: %v", err)
	}
	if _, err := dsSet(e.p.cqHarness, t04Key("pg-service"), in); err != nil {
		e.t.Fatalf("service: %v", err)
	}
	if _, err := daSet(e.p.cqHarness, t04Key("pg-allocation"), fulfillment.AllocationInput{MarketID: e.p.market.ID, Country: "TW", Code: in.Code, ExpectedServiceVersion: 1, WarehouseIDs: []string{e.p.stock.warehouse.ID}}); err != nil {
		e.t.Fatalf("allocation: %v", err)
	}
	return in.Code
}

// pgBuyer is a buyer whose cart holds one line per price (one fresh SKU each, quantity 1).
func (e *tcvEnv) pgBuyer(prices ...int64) *tcvBuyer {
	e.t.Helper()
	items := make([]storefront.Item, len(prices))
	for i, p := range prices {
		items[i] = storefront.Item{SKUID: e.sku(p, 50), Quantity: 1}
	}
	return e.newBuyer(items...)
}

// pgBegin places the buyer's order with the given quote through the real checkout.Begin; edit adjusts the input (payment mode, e-mail, destination).
func (b *tcvBuyer) pgBegin(q storefront.Quote, edit func(*checkout.Input)) (checkout.Result, error) {
	in := b.h.input
	in.QuoteID = q.ID
	if edit != nil {
		edit(&in)
	}
	return b.e.svc.Begin(context.Background(), b.cap.Token, b.e.store(), t04Key("pg-begin"), in)
}

// pgCheckout is the same placement through the real buyer HTTP route (the BFF-facing contract).
func (b *tcvBuyer) pgCheckout(q storefront.Quote) bhResponse {
	in := b.h.input
	in.QuoteID = q.ID
	return b.req("POST", "/v1/buyer/checkout", t04Key("pg-http"), in, nil)
}

// pgOtherPhone gives the buyer a second destination with another phone number and returns its id (the harness gives every buyer the same phone).
func (b *tcvBuyer) pgOtherPhone(phone string) string {
	b.e.t.Helper()
	d := bdHome(storefront.Cart{Version: b.cartVersion()})
	d.Phone, d.ExpectedVersion = phone, b.h.destination.Version
	out, err := bdSet(b.h.cqHarness, t04Key("pg-dest"), d)
	if err != nil {
		b.e.t.Fatalf("destination: %v", err)
	}
	return out.ID
}

func pgRefusal(t *testing.T, r bhResponse, want string) {
	t.Helper()
	var env struct {
		Code string `json:"code"`
	}
	if r.status != 422 || json.Unmarshal(r.body, &env) != nil || env.Code != want {
		t.Fatalf("want 422 %s, got %d %s", want, r.status, r.body)
	}
}

// pgFinance sums the finance summary rows around today (Asia/Taipei days).
func (e *tcvEnv) pgFinance() (confirmedCount, confirmedMinor, captured, refunded float64) {
	e.t.Helper()
	today := time.Now().In(pgTaipei)
	from, to := today.AddDate(0, 0, -1).Format("2006-01-02"), today.AddDate(0, 0, 1).Format("2006-01-02")
	st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary?from="+from+"&to="+to, "", "")
	if st != 200 {
		e.t.Fatalf("finance: %d %s", st, raw)
	}
	rows, _ := out["totals"].([]any)
	for _, row := range rows {
		m, _ := row.(map[string]any)
		confirmedCount += m["bank_transfer_confirmed_count"].(float64)
		confirmedMinor += m["bank_transfer_confirmed_minor"].(float64)
		captured += m["captured_minor"].(float64)
		refunded += m["refunded_minor"].(float64)
	}
	return
}

// pgMemberOn creates a merchant principal of ANY tenant/store with exactly perms (plus store:read) and returns its bearer token.
func (e *tcvEnv) pgMemberOn(tenant, store string, perms ...string) string {
	e.t.Helper()
	f := e.p.f
	token, principal := randomToken(), randomUUID()
	mustExec(e.t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	mustExec(e.t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, tenant, principal)
	seen := map[string]bool{}
	for _, perm := range append([]string{"store:read"}, perms...) {
		if seen[perm] {
			continue
		}
		seen[perm] = true
		mustExec(e.t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, tenant, store, principal, perm)
	}
	tx, err := f.owner.Begin(context.Background())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := insertSession(context.Background(), tx, token, principal, "merchant", time.Now().Add(time.Hour), nil); err != nil {
		e.t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	return token
}

// pgCreateAs creates a code with an explicit bearer token and store.
func (e *tcvEnv) pgCreateAs(token, store, code string, over map[string]any) (int, map[string]any) {
	e.t.Helper()
	f := proFields(over)
	f["code"] = code
	st, out, _ := e.mcall(token, "POST", "/v1/admin/stores/"+store+"/promotions", t04Key("pg-create"), proBody(e.t, f))
	return st, out
}

func (e *tcvEnv) pgList(token, store string) (int, []any, []byte) {
	e.t.Helper()
	st, out, raw := e.mcall(token, "GET", "/v1/admin/stores/"+store+"/promotions", "", "")
	list, _ := out["promotions"].([]any)
	return st, list, raw
}

// ---- PGT01 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateMath(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	const fee = int64(6000)
	svc := e.pgService(fee, nil)
	pct := func(n int) map[string]any { return map[string]any{"percent": n} }
	fix := func(n int64) map[string]any {
		return map[string]any{"kind": "fixed", "percent": nil, "fixed_minor": n}
	}
	// want = the discount the contract prescribes: percent = floor(subtotal*p/100) then rounded DOWN to a whole TWD dollar (100 minor);
	// fixed = min(fixed, subtotal) then rounded DOWN to a whole dollar; a discount that rounds to 0 is refused promo_invalid.
	cases := []struct {
		name   string
		prices []int64
		over   map[string]any
		want   int64
		refuse string
	}{
		{"percent 1 of 100000", []int64{100000}, pct(1), 1000, ""},
		{"percent 10 of 12345 floors 1234 down to 1200", []int64{12345}, pct(10), 1200, ""},
		{"percent 1 of 12345 floors 123 down to 100 (never up to 200)", []int64{12345}, pct(1), 100, ""},
		{"percent 90 of 12345 floors 11110 down to 11100", []int64{12345}, pct(90), 11100, ""},
		{"percent 33 of 199999 floors 65999 down to 65900", []int64{199999}, pct(33), 65900, ""},
		{"percent 1 of 9900 rounds to a zero discount: refused, no use burned", []int64{9900}, pct(1), 0, "promo_invalid"},
		{"fixed 5000 of 100000", []int64{100000}, fix(5000), 5000, ""},
		{"fixed 100 (the smallest whole dollar)", []int64{100000}, fix(100), 100, ""},
		{"fixed equal to the subtotal leaves goods at exactly 0", []int64{100000}, fix(100000), 100000, ""},
		{"fixed above the subtotal is capped at the subtotal", []int64{30000}, fix(99999900), 30000, ""},
		{"capped fixed on a non-whole subtotal floors to a whole dollar: 12345 -> 12300", []int64{12345}, fix(20000), 12300, ""},
		{"two lines, percent 7 of 100000", []int64{33300, 66700}, pct(7), 7000, ""},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code := fmt.Sprintf("MATH%02d", i)
			e.proMust(code, c.over)
			b := e.pgBuyer(c.prices...)
			var subtotal int64
			for _, p := range c.prices {
				subtotal += p
			}
			base, err := b.quoteCodeFor(svc, "")
			if err != nil || base.Amount.SubtotalMinor != subtotal || base.Amount.DiscountMinor != 0 || base.Amount.ShippingMinor != fee || base.Amount.TotalMinor != subtotal+fee {
				t.Fatalf("code-less quote: %v %+v", err, base.Amount)
			}
			q, err := b.quoteCodeFor(svc, strings.ToLower(code))
			if c.refuse != "" {
				if err == nil || !strings.Contains(proCode(err), c.refuse) {
					t.Fatalf("want %s, got %v (%+v)", c.refuse, err, q.Amount)
				}
				return
			}
			if err != nil {
				t.Fatalf("quote: %v", err)
			}
			a := q.Amount
			if a.DiscountMinor != c.want || a.ShippingMinor != fee || a.TaxMinor != 0 || a.SubtotalMinor != subtotal {
				t.Errorf("amounts %+v, want discount %d, shipping %d (a code never discounts shipping)", a, c.want, fee)
			}
			if a.TotalMinor != subtotal-c.want+fee || a.TotalMinor < 0 || subtotal-a.DiscountMinor < 0 {
				t.Errorf("total %d, want %d (never below zero)", a.TotalMinor, subtotal-c.want+fee)
			}
			if a.DiscountMinor%100 != 0 {
				t.Errorf("TWD discount %d is not a whole dollar", a.DiscountMinor)
			}
			var sum int64
			for j, l := range q.Lines {
				d := l.Amount.DiscountMinor
				sum += d
				if d < 0 || d > l.Amount.SubtotalMinor {
					t.Errorf("line %d discount %d outside [0, %d]", j, d, l.Amount.SubtotalMinor)
				}
				// proportional within one unit: floor(disc*line/subtotal) <= d <= floor+1
				exact := c.want * l.Amount.SubtotalMinor / subtotal
				if d < exact || d > exact+1 {
					t.Errorf("line %d discount %d is not its proportional share (%d)", j, d, exact)
				}
			}
			if sum != c.want {
				t.Errorf("line discounts sum %d, want %d", sum, c.want)
			}
		})
	}
	t.Run("admin bounds: percent 1..90, fixed positive whole dollar", func(t *testing.T) {
		for name, tc := range map[string]struct {
			over map[string]any
			want int
		}{
			"percent 0": {pct(0), 422}, "percent 91": {pct(91), 422}, "percent -1": {pct(-1), 422},
			"percent 1": {pct(1), 200}, "percent 90": {pct(90), 200},
			"fixed 0": {fix(0), 422}, "fixed 150 (not a whole TWD dollar)": {fix(150), 422}, "fixed 100": {fix(100), 200},
		} {
			code := "BND" + strings.ToUpper(t04Tag())[:6]
			if st, out := e.proCreate(code, tc.over); st != tc.want {
				t.Errorf("%s: want %d, got %d %v", name, tc.want, st, out)
			}
		}
	})
}

// ---- PGT02 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateWindowAndMinimum(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	svc := e.pgService(0, nil)
	b := e.pgBuyer(60000, 40000) // subtotal 100000
	const sub = int64(100000)
	taipei := func(at time.Time) string { return at.In(pgTaipei).Format("2006-01-02T15:04:05+08:00") }
	quote := func(code string) error { _, err := b.quoteCodeFor(svc, code); return err }

	t.Run("minimum subtotal is inclusive and compares the PRE-discount subtotal", func(t *testing.T) {
		e.proMust("MINEQ", map[string]any{"min_subtotal_minor": sub})
		e.proMust("MINOVER", map[string]any{"min_subtotal_minor": sub + 1})
		// 50% off leaves 50000 < 60000, but the pre-discount subtotal 100000 qualifies
		e.proMust("MINPRE", map[string]any{"percent": 50, "min_subtotal_minor": 60000})
		if err := quote("mineq"); err != nil {
			t.Errorf("subtotal == minimum must qualify: %v", err)
		}
		if err := quote("minover"); err == nil || !strings.Contains(proCode(err), "promo_min_subtotal") {
			t.Errorf("subtotal one below the minimum: want promo_min_subtotal, got %v", err)
		}
		q, err := b.quoteCodeFor(svc, "minpre")
		if err != nil || q.Amount.DiscountMinor != 50000 {
			t.Errorf("minimum is checked before the discount: %v %+v", err, q.Amount)
		}
		// the authoritative placement at the equality boundary
		q, err = b.quoteCodeFor(svc, "mineq")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = b.pgBegin(q, nil); err != nil {
			t.Errorf("Begin at subtotal == minimum: %v", err)
		}
	})

	t.Run("Asia/Taipei offsets: the instant is the +08:00 wall time, not the same digits in UTC", func(t *testing.T) {
		now := time.Now()
		// valid window written in Taipei wall time: [now-1h, now+1h]. Read as UTC wall time the start would be 7 hours in the future.
		id, _ := e.proMust("TPEVALID", map[string]any{"starts_at": taipei(now.Add(-time.Hour)), "ends_at": taipei(now.Add(time.Hour))})
		if err := quote("tpevalid"); err != nil {
			t.Errorf("a window around now written in Taipei time must be open: %v", err)
		}
		// ended one minute ago (Taipei wall time); read as UTC it would still be open for 8 hours
		e.proMust("TPEOVER", map[string]any{"ends_at": taipei(now.Add(-time.Minute))})
		if err := quote("tpeover"); err == nil || !strings.Contains(proCode(err), "promo_expired") {
			t.Errorf("ended a minute ago (Taipei): want promo_expired, got %v", err)
		}
		// starts in 90 s (Taipei wall time); read as UTC it would be open already
		e.proMust("TPEFUTURE", map[string]any{"starts_at": taipei(now.Add(90 * time.Second))})
		if err := quote("tpefuture"); err == nil || !strings.Contains(proCode(err), "promo_not_started") {
			t.Errorf("starts in 90 s (Taipei): want promo_not_started, got %v", err)
		}
		// the merchant list reads the same instants back
		st, list, raw := e.pgList(e.token(), e.store())
		if st != 200 {
			t.Fatalf("list: %d %s", st, raw)
		}
		for _, it := range list {
			m := it.(map[string]any)
			if m["id"] != id {
				continue
			}
			s, _ := time.Parse(time.RFC3339, m["starts_at"].(string))
			if !s.Equal(now.Add(-time.Hour).Truncate(time.Second)) {
				t.Errorf("starts_at round-trips as %v, want the instant %v", s, now.Add(-time.Hour).Truncate(time.Second))
			}
		}
	})

	t.Run("boundaries are crossed in real time: starts_at opens, ends_at closes", func(t *testing.T) {
		edge := time.Now().Add(3 * time.Second).Truncate(time.Second).Add(time.Second) // 3..4 s ahead
		e.proMust("OPENS", map[string]any{"starts_at": taipei(edge)})
		e.proMust("CLOSES", map[string]any{"ends_at": taipei(edge)})
		if err := quote("opens"); err == nil || !strings.Contains(proCode(err), "promo_not_started") {
			t.Errorf("before the start: %v", err)
		}
		if err := quote("closes"); err != nil {
			t.Errorf("before the end: %v", err)
		}
		time.Sleep(time.Until(edge) + 400*time.Millisecond)
		if err := quote("opens"); err != nil {
			t.Errorf("after the start: %v", err)
		}
		if err := quote("closes"); err == nil || !strings.Contains(proCode(err), "promo_expired") {
			t.Errorf("after the end: %v", err)
		}
	})
}

// ---- PGT03 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateShipping(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write", "orders:read")
	const fee, threshold = int64(6000), int64(100000)
	paid := e.pgService(fee, nil)
	thr := threshold
	free := e.pgService(fee, &thr)
	e.proMust("HALF", map[string]any{"percent": 50})
	e.proMust("TEN", nil)
	e.proMust("FIXALL", map[string]any{"kind": "fixed", "percent": nil, "fixed_minor": 99999900})

	t.Run("no threshold: a code touches goods only, shipping is charged in full on top", func(t *testing.T) {
		b := e.pgBuyer(100000)
		q, err := b.quoteCodeFor(paid, "half")
		if err != nil || q.Amount.DiscountMinor != 50000 || q.Amount.ShippingMinor != fee || q.Amount.TotalMinor != 50000+fee {
			t.Fatalf("%v %+v", err, q.Amount)
		}
		q, err = b.quoteCodeFor(paid, "fixall") // even a code worth more than the cart never reaches shipping
		if err != nil || q.Amount.DiscountMinor != 100000 || q.Amount.ShippingMinor != fee || q.Amount.TotalMinor != fee {
			t.Fatalf("a code worth more than the cart: %v %+v", err, q.Amount)
		}
	})
	t.Run("threshold uses the PRE-discount subtotal (at and above it)", func(t *testing.T) {
		b := e.pgBuyer(100000) // == threshold
		none, err := b.quoteCodeFor(free, "")
		if err != nil || none.Amount.ShippingMinor != 0 {
			t.Fatalf("subtotal == threshold is free without a code: %v %+v", err, none.Amount)
		}
		q, err := b.quoteCodeFor(free, "ten") // 100000 - 10000 = 90000 < threshold, yet the cart qualified before the code
		if err != nil || q.Amount.DiscountMinor != 10000 || q.Amount.ShippingMinor != 0 || q.Amount.TotalMinor != 90000 {
			t.Fatalf("code on a free-shipping cart: %v %+v", err, q.Amount)
		}
		// the same through the authoritative placement: the order total is the discounted total, still no shipping
		res, err := b.pgBegin(q, nil)
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		var total int64
		var ship float64
		if err := e.p.f.owner.QueryRow(context.Background(), `SELECT total_minor,(snapshot#>>'{quote,amount,shipping_minor}')::bigint FROM checkout.orders WHERE id=$1`, res.OrderID).Scan(&total, &ship); err != nil || total != 90000 || ship != 0 {
			t.Errorf("order total %d shipping %v (%v)", total, ship, err)
		}
	})
	t.Run("below the threshold a code cannot buy free shipping, and the threshold is not re-read after the discount", func(t *testing.T) {
		b := e.pgBuyer(99999) // one minor unit below the threshold
		q, err := b.quoteCodeFor(free, "ten")
		if err != nil || q.Amount.ShippingMinor != fee || q.Amount.DiscountMinor != 9900 || q.Amount.TotalMinor != 99999-9900+fee {
			t.Fatalf("%v %+v", err, q.Amount)
		}
	})
}

// ---- PGT04 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateOneCode(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	svc := e.pgService(0, nil)
	e.proMust("ONE10", nil)
	e.proMust("TWO20", map[string]any{"percent": 20})
	b := e.pgBuyer(100000)

	t.Run("a second code replaces the first, never stacks", func(t *testing.T) {
		q1, err := b.quoteCodeFor(svc, "one10")
		if err != nil || q1.Amount.DiscountMinor != 10000 || q1.Promotion == nil || q1.Promotion.Code != "ONE10" {
			t.Fatalf("%v %+v", err, q1.Amount)
		}
		q2, err := b.quoteCodeFor(svc, "two20")
		if err != nil || q2.Amount.DiscountMinor != 20000 || q2.Promotion == nil || q2.Promotion.Code != "TWO20" {
			t.Fatalf("a re-quote with another code is priced by that code alone: %v %+v", err, q2.Amount)
		}
	})
	t.Run("a list or a combined string is not a code", func(t *testing.T) {
		before := e.count(`SELECT count(*) FROM storefront.quotes WHERE owner_id=$1`, b.cap.Scope.OwnerID)
		for _, bad := range []any{"ONE10,TWO20", "ONE10 TWO20", "ONE10+TWO20", []string{"ONE10", "TWO20"}} {
			r := b.req("POST", "/v1/buyer/quotes", t04Key("pg-multi"), map[string]any{"cart_version": b.cartVersion(), "market_id": e.p.market.ID, "country": "TW",
				"method": "delivery:" + svc, "promo_code": bad}, nil)
			if r.status < 400 || r.status >= 500 {
				t.Errorf("promo_code %v: want a 4xx refusal, got %d %s", bad, r.status, r.body)
			}
		}
		for _, key := range []string{"promo_codes", "codes"} {
			r := b.req("POST", "/v1/buyer/quotes", t04Key("pg-multi"), map[string]any{"cart_version": b.cartVersion(), "market_id": e.p.market.ID, "country": "TW",
				"method": "delivery:" + svc, key: []string{"ONE10", "TWO20"}}, nil)
			if r.status < 400 || r.status >= 500 {
				t.Errorf("unknown key %s: want a 4xx refusal, got %d %s", key, r.status, r.body)
			}
		}
		if after := e.count(`SELECT count(*) FROM storefront.quotes WHERE owner_id=$1`, b.cap.Scope.OwnerID); after != before {
			t.Errorf("a refused multi-code request stored %d quotes", after-before)
		}
	})
	t.Run("the client never supplies an amount", func(t *testing.T) {
		for _, key := range []string{"discount_minor", "total_minor", "amount", "tenant_id", "store_id"} {
			var forged any = int64(1)
			if key == "amount" {
				forged = map[string]any{"discount_minor": 99999900, "total_minor": 100}
			}
			if key == "tenant_id" || key == "store_id" {
				forged = randomUUID()
			}
			r := b.req("POST", "/v1/buyer/quotes", t04Key("pg-forge"), map[string]any{"cart_version": b.cartVersion(), "market_id": e.p.market.ID, "country": "TW",
				"method": "delivery:" + svc, "promo_code": "one10", key: forged}, nil)
			if r.status == 200 {
				var out struct {
					Amount struct {
						Discount int64 `json:"discount_minor"`
						Total    int64 `json:"total_minor"`
					} `json:"amount"`
				}
				if json.Unmarshal(r.body, &out) != nil || out.Amount.Discount != 10000 || out.Amount.Total != 90000 {
					t.Errorf("client key %s changed the priced amount: %s", key, r.body)
				}
			} else if r.status < 400 || r.status >= 500 {
				t.Errorf("client key %s: unexpected status %d", key, r.status)
			}
		}
	})
	t.Run("an order placed with one code has exactly one redemption; a code-less quote has none", func(t *testing.T) {
		q, err := b.quoteCodeFor(svc, "two20")
		if err != nil {
			t.Fatal(err)
		}
		res, err := b.pgBegin(q, nil)
		if err != nil {
			t.Fatal(err)
		}
		if n := e.count(`SELECT count(*) FROM promotions.redemptions WHERE order_id=$1`, res.OrderID); n != 1 {
			t.Errorf("%d redemptions for one order", n)
		}
		plain := e.pgBuyer(100000)
		pq, _ := plain.quoteCodeFor(svc, "")
		pres, err := plain.pgBegin(pq, nil)
		if err != nil {
			t.Fatal(err)
		}
		if n := e.count(`SELECT count(*) FROM promotions.redemptions WHERE order_id=$1`, pres.OrderID); n != 0 {
			t.Errorf("a code-less order has %d redemptions", n)
		}
	})
}

// ---- PGT05 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateBeginErrors(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	svc := e.pgService(0, nil)
	facts := func(b *tcvBuyer) [3]int {
		o := b.cap.Scope.OwnerID
		return [3]int{e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, o), e.count(`SELECT count(*) FROM inventory.reservations WHERE buyer_owner_id=$1`, o),
			e.count(`SELECT count(*) FROM promotions.redemptions WHERE owner_id=$1`, o)}
	}

	t.Run("used up between quote and Begin", func(t *testing.T) {
		id, _ := e.proMust("LASTONE", map[string]any{"total_limit": 1})
		a, b := e.pgBuyer(100000), e.pgBuyer(100000)
		qa, _ := a.quoteCodeFor(svc, "lastone")
		qb, err := b.quoteCodeFor(svc, "lastone")
		if err != nil {
			t.Fatalf("quote while one use is left: %v", err)
		}
		if r := a.pgCheckout(qa); r.status != 200 {
			t.Fatalf("first placement: %d %s", r.status, r.body)
		}
		pgRefusal(t, b.pgCheckout(qb), "promo_used_up")
		if f := facts(b); f != [3]int{} {
			t.Errorf("the refused placement left facts %v", f)
		}
		if e.proUsed(id) != 1 {
			t.Errorf("usage %d", e.proUsed(id))
		}
		// a fresh quote is refused up front (advisory), with the same typed code
		if _, err := b.quoteCodeFor(svc, "lastone"); err == nil || !strings.Contains(proCode(err), "promo_used_up") {
			t.Errorf("re-quote: %v", err)
		}
	})
	t.Run("expired between quote and Begin", func(t *testing.T) {
		end := time.Now().Add(3 * time.Second).Truncate(time.Second).Add(time.Second)
		e.proMust("SHORTLIFE", map[string]any{"ends_at": end.In(pgTaipei).Format("2006-01-02T15:04:05+08:00")})
		b := e.pgBuyer(100000)
		q, err := b.quoteCodeFor(svc, "shortlife")
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Until(end) + 400*time.Millisecond)
		pgRefusal(t, b.pgCheckout(q), "promo_expired")
		if f := facts(b); f != [3]int{} {
			t.Errorf("the refused placement left facts %v", f)
		}
	})
	t.Run("edited between quote and Begin: promo_changed, then a re-quote places", func(t *testing.T) {
		id, ver := e.proMust("EDITME", nil)
		b := e.pgBuyer(100000)
		q, err := b.quoteCodeFor(svc, "editme")
		if err != nil {
			t.Fatal(err)
		}
		if st, out := e.proUpdate(id, ver, map[string]any{"percent": 20}); st != 200 {
			t.Fatalf("edit: %d %v", st, out)
		}
		pgRefusal(t, b.pgCheckout(q), "promo_changed")
		if f := facts(b); f != [3]int{} {
			t.Errorf("the refused placement left facts %v", f)
		}
		q2, err := b.quoteCodeFor(svc, "editme")
		if err != nil || q2.Amount.DiscountMinor != 20000 {
			t.Fatalf("re-quote prices the new terms: %v %+v", err, q2.Amount)
		}
		if r := b.pgCheckout(q2); r.status != 200 {
			t.Errorf("placement after the re-quote: %d %s", r.status, r.body)
		}
	})
}

// ---- PGT06 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateConcurrency(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	svc := e.pgService(0, nil)

	race := func(buyers []*tcvBuyer, quotes []storefront.Quote, edit func(i int, in *checkout.Input)) (won int, errs []error) {
		errs = make([]error, len(buyers))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range buyers {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, errs[i] = buyers[i].pgBegin(quotes[i], func(in *checkout.Input) {
					if edit != nil {
						edit(i, in)
					}
				})
			}(i)
		}
		close(start)
		wg.Wait()
		for _, err := range errs {
			if err == nil {
				won++
			}
		}
		return
	}
	prep := func(n int, code string) ([]*tcvBuyer, []storefront.Quote) {
		buyers := make([]*tcvBuyer, n)
		quotes := make([]storefront.Quote, n)
		for i := range buyers {
			buyers[i] = e.pgBuyer(100000)
			q, err := buyers[i].quoteCodeFor(svc, code)
			if err != nil {
				t.Fatalf("quote %d: %v", i, err)
			}
			quotes[i] = q
		}
		return buyers, quotes
	}

	t.Run("total_limit 3 with 10 racing placements admits exactly 3", func(t *testing.T) {
		id, _ := e.proMust("THREEONLY", map[string]any{"total_limit": 3})
		buyers, quotes := prep(10, "threeonly")
		won, errs := race(buyers, quotes, nil)
		for i, err := range errs {
			if err != nil && !strings.Contains(proCode(err), "promo_used_up") {
				t.Errorf("buyer %d: want promo_used_up, got %v", i, err)
			}
		}
		if won != 3 || e.proUsed(id) != 3 {
			t.Fatalf("%d placements won, usage %d, want exactly 3 (oversubscribed or under-admitted)", won, e.proUsed(id))
		}
		if n := e.count(`SELECT count(*) FROM promotions.redemptions WHERE code_id=$1`, id); n != 3 {
			t.Errorf("%d redemption rows", n)
		}
	})
	t.Run("per_buyer_limit 1 with 6 racing placements from one identity admits exactly 1", func(t *testing.T) {
		id, _ := e.proMust("ONEPERSON", map[string]any{"per_buyer_limit": 1})
		buyers, quotes := prep(6, "oneperson") // six capabilities, the harness gives all of them the same recipient phone = one buyer
		won, errs := race(buyers, quotes, nil)
		for i, err := range errs {
			if err != nil && !strings.Contains(proCode(err), "promo_buyer_limit") {
				t.Errorf("buyer %d: want promo_buyer_limit, got %v", i, err)
			}
		}
		if won != 1 || e.proUsed(id) != 1 {
			t.Fatalf("%d placements won, usage %d, want exactly 1", won, e.proUsed(id))
		}
	})
	t.Run("a sequential per-buyer limit 2 admits two orders and refuses the third", func(t *testing.T) {
		id, _ := e.proMust("TWICE", map[string]any{"per_buyer_limit": 2})
		buyers, quotes := prep(3, "twice")
		for i := range buyers {
			_, err := buyers[i].pgBegin(quotes[i], nil)
			switch {
			case i < 2 && err != nil:
				t.Errorf("order %d: %v", i, err)
			case i == 2 && (err == nil || !strings.Contains(proCode(err), "promo_buyer_limit")):
				t.Errorf("third order: want promo_buyer_limit, got %v", err)
			}
		}
		if e.proUsed(id) != 2 {
			t.Errorf("usage %d", e.proUsed(id))
		}
	})
}

// ---- PGT07 ---------------------------------------------------------------------------------------------------------------------

// pgTransferEnv enables bank transfer for the store (through the real settings route).
func (e *tcvEnv) pgTransferEnv() {
	e.grantCreator("pricing:read", "pricing:write", "orders:read", "orders:export", "payments:refund", "fulfillment:write")
	e.cofEnsureSettings(0, true, true, 72)
}

func (e *tcvEnv) pgTransfer(email, phoneDest string) func(*checkout.Input) {
	return func(in *checkout.Input) {
		in.PaymentMode, in.BuyerEmail = "bank_transfer", email
		if phoneDest != "" {
			in.DestinationID = phoneDest
		}
	}
}

func TestPromoGateBankTransfer(t *testing.T) {
	e := tcvNew(t)
	e.pgTransferEnv()
	svc := e.pgService(6000, nil)
	f := e.p.f
	ctx := context.Background()

	t.Run("per-buyer identity: e-mail and phone each match, a new identity is a new buyer", func(t *testing.T) {
		id, _ := e.proMust("EACH", map[string]any{"per_buyer_limit": 1})
		a, b, c, d := e.pgBuyer(100000), e.pgBuyer(100000), e.pgBuyer(100000), e.pgBuyer(100000)
		qa, _ := a.quoteCodeFor(svc, "each")
		qb, _ := b.quoteCodeFor(svc, "each")
		qc, _ := c.quoteCodeFor(svc, "each")
		qd, _ := d.quoteCodeFor(svc, "each")
		if _, err := a.pgBegin(qa, e.pgTransfer("same@example.com", a.pgOtherPhone("+886911111111"))); err != nil {
			t.Fatalf("first buyer: %v", err)
		}
		// another capability, another phone, SAME e-mail (any case): the e-mail identity matches
		if _, err := b.pgBegin(qb, e.pgTransfer("Same@Example.com", b.pgOtherPhone("+886922222222"))); err == nil || !strings.Contains(proCode(err), "promo_buyer_limit") {
			t.Errorf("same e-mail: want promo_buyer_limit, got %v", err)
		}
		// another capability, another e-mail, SAME phone: the phone identity matches
		if _, err := c.pgBegin(qc, e.pgTransfer("other@example.com", c.pgOtherPhone("+886911111111"))); err == nil || !strings.Contains(proCode(err), "promo_buyer_limit") {
			t.Errorf("same phone: want promo_buyer_limit, got %v", err)
		}
		// another capability, e-mail and phone: a new buyer (documented weakness of the control), allowed
		if _, err := d.pgBegin(qd, e.pgTransfer("new@example.com", d.pgOtherPhone("+886933333333"))); err != nil {
			t.Errorf("a genuinely new identity must be admitted: %v", err)
		}
		if e.proUsed(id) != 2 {
			t.Errorf("usage %d, want 2 (first buyer + the new identity)", e.proUsed(id))
		}
	})

	t.Run("discounted money reaches the buyer's transfer amount, the merchant order view, finance and the offline refund", func(t *testing.T) {
		e.proMust("PAY10", map[string]any{"min_subtotal_minor": 50000})
		b := e.pgBuyer(40000, 60000) // subtotal 100000, shipping 6000
		q, err := b.quoteCodeFor(svc, "pay10")
		if err != nil || q.Amount.DiscountMinor != 10000 || q.Amount.TotalMinor != 96000 {
			t.Fatalf("quote: %v %+v", err, q.Amount)
		}
		before, _, _, _ := e.pgFinance()
		res, err := b.pgBegin(q, e.pgTransfer("buyer@example.com", ""))
		if err != nil || res.CommercialState != "AWAITING_TRANSFER" {
			t.Fatalf("placement: %v %+v", err, res)
		}
		order := res.OrderID
		var total, discount int64
		if err := f.owner.QueryRow(ctx, `SELECT o.total_minor,r.discount_minor FROM checkout.orders o JOIN promotions.redemptions r ON r.order_id=o.id WHERE o.id=$1`, order).Scan(&total, &discount); err != nil || total != 96000 || discount != 10000 {
			t.Fatalf("order total %d redemption discount %d (%v), want 96000 / 10000", total, discount, err)
		}
		// what the buyer is told to transfer
		view := cofJSON(t, b.cofView(order).body)
		if view["amount_minor"] != float64(96000) {
			t.Errorf("buyer is asked to transfer %v, want the discounted total 96000", view["amount_minor"])
		}
		// merchant order detail
		st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+order, "", "")
		totals, _ := out["totals"].(map[string]any)
		if st != 200 || totals["discount_minor"] != float64(10000) || totals["total_minor"] != float64(96000) || totals["shipping_minor"] != float64(6000) {
			t.Fatalf("merchant order totals: %d %s", st, raw)
		}
		var lineSum float64
		items, _ := out["items"].([]any)
		for _, l := range items {
			if amount, ok := l.(map[string]any)["amount"].(map[string]any); ok {
				lineSum += amount["discount_minor"].(float64)
			}
		}
		if lineSum != 10000 {
			t.Errorf("merchant order lines carry discount %v, want 10000: %s", lineSum, raw)
		}
		// proof for the discounted amount, then the merchant confirms
		if p := b.cofProof(order, t04Key("pg-proof"), "12345", 96000); p.status != 200 {
			t.Fatalf("proof: %d %s", p.status, p.body)
		}
		if st, out := e.cofDecide(e.token(), order, "confirm", t04Key("pg-confirm"), `{}`); st != 200 || out["state"] != "CONFIRMED" {
			t.Fatalf("confirm: %d %v", st, out)
		}
		var confirmed int64
		if err := f.owner.QueryRow(ctx, `SELECT confirmed_amount_minor FROM checkout.bank_transfers WHERE order_id=$1`, order).Scan(&confirmed); err != nil || confirmed != 96000 {
			t.Errorf("confirmed amount %d (%v), want the discounted total 96000", confirmed, err)
		}
		count, minor, captured, _ := e.pgFinance()
		if count != 1 || minor-before != 96000 || captured != 0 {
			t.Errorf("finance: count=%v confirmed_minor=%v (+%v) captured=%v, want 1 / 96000", count, minor, minor-before, captured)
		}
		// the CSV export carries the same discounted number
		today := time.Now().In(pgTaipei)
		csvStatus, _, csvRaw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/finance/summary.csv?from="+today.AddDate(0, 0, -1).Format("2006-01-02")+"&to="+today.AddDate(0, 0, 1).Format("2006-01-02"), "", "")
		if csvStatus != 200 || !strings.Contains(string(csvRaw), "96000") || strings.Contains(string(csvRaw), "106000") {
			t.Errorf("finance CSV: %d %s", csvStatus, csvRaw)
		}
		// offline refund: the recorded fact keeps the paid (discounted) amount
		if st, out := e.cofDecide(e.token(), order, "refund-offline", t04Key("pg-refund"), `{}`); st != 200 || out["state"] != "REFUNDED_OFFLINE" {
			t.Fatalf("offline refund: %d %v", st, out)
		}
		if err := f.owner.QueryRow(ctx, `SELECT confirmed_amount_minor FROM checkout.bank_transfers WHERE order_id=$1`, order).Scan(&confirmed); err != nil || confirmed != 96000 {
			t.Errorf("after the offline refund the paid amount is %d (%v), want 96000", confirmed, err)
		}
	})
}

// ---- PGT08 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateRelease(t *testing.T) {
	e := tcvNew(t)
	e.pgTransferEnv()
	svc := e.pgService(0, nil)
	id, _ := e.proMust("RELEASE1", map[string]any{"total_limit": 1})
	a, b, c := e.pgBuyer(100000), e.pgBuyer(100000), e.pgBuyer(100000)
	qa, _ := a.quoteCodeFor(svc, "release1")
	qb, _ := b.quoteCodeFor(svc, "release1")
	qc, _ := c.quoteCodeFor(svc, "release1")

	phoneA, phoneB, phoneC := a.pgOtherPhone("+886944444441"), b.pgOtherPhone("+886944444442"), c.pgOtherPhone("+886944444443")
	ra, err := a.pgBegin(qa, e.pgTransfer("a@example.com", phoneA))
	if err != nil {
		t.Fatal(err)
	}
	if e.proUsed(id) != 1 {
		t.Fatalf("usage after the first placement %d", e.proUsed(id))
	}
	if _, err = b.pgBegin(qb, e.pgTransfer("b@example.com", phoneB)); err == nil || !strings.Contains(proCode(err), "promo_used_up") {
		t.Fatalf("second placement while the first is open: %v", err)
	}
	// the transfer window ends: the order is CANCELLED/EXPIRED by the real expiry function and its use is free again with no hook
	e.cofAge(ra.OrderID, 73)
	if d, _ := e.cofExpire(ra.OrderID); d != "EXPIRED" {
		t.Fatalf("expire_held: %s", d)
	}
	if got := e.proUsed(id); got != 0 {
		t.Fatalf("usage after the window ended: %d, want 0", got)
	}
	st, list, _ := e.pgList(e.token(), e.store())
	if st != 200 || list[0].(map[string]any)["used"] != float64(0) {
		t.Errorf("merchant list still shows usage after expiry: %v", list)
	}
	rb, err := b.pgBegin(qb, e.pgTransfer("b@example.com", phoneB))
	if err != nil {
		t.Fatalf("the freed use must be placeable: %v", err)
	}
	// a confirmed order keeps its use, and so does one the merchant refunded offline (contract F: a refund is not a reason to reuse a limited code)
	if st, out := e.cofDecide(e.token(), rb.OrderID, "confirm", t04Key("pg-rel-confirm"), `{}`); st != 200 {
		t.Fatalf("confirm: %d %v", st, out)
	}
	if _, err = c.pgBegin(qc, e.pgTransfer("c@example.com", phoneC)); err == nil || !strings.Contains(proCode(err), "promo_used_up") {
		t.Errorf("a CONFIRMED order keeps counting: %v", err)
	}
	if st, out := e.cofDecide(e.token(), rb.OrderID, "refund-offline", t04Key("pg-rel-refund"), `{}`); st != 200 {
		t.Fatalf("offline refund: %d %v", st, out)
	}
	if got := e.proUsed(id); got != 1 {
		t.Errorf("a refunded order must keep counting (contract F), usage %d", got)
	}
	if _, err = c.quoteCodeFor(svc, "release1"); err == nil || !strings.Contains(proCode(err), "promo_used_up") {
		t.Errorf("quote after a refunded order: %v", err)
	}
}

// ---- PGT09 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateStripeRefundCap(t *testing.T) {
	// BASE DEFECT workaround (output/promotions/tests/DEFECTS.md B1): on r3/integration 736561a two trigger functions (catalog.products_default_slug,
	// design.refuse_history_change) keep PUBLIC EXECUTE, so platform.OpenStripeRegistrarPool refuses every Stripe login ("unsafe stripe database
	// privileges") and EVERY Stripe-based gate fails before its first line. The disposable PG of this test gets the REVOKE the owning units forgot;
	// trigger functions need no EXECUTE at fire time. Nothing in promotions is touched.
	revokeBaseDefaultExec(t)
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("pricing:read", "pricing:write", "orders:read", "payments:refund")
	ctx := context.Background()
	e.proMust("CAP10", nil)
	b := e.pgBuyer(100000)
	q, err := b.quoteCodeFor(e.p.delivery.Code, "cap10")
	if err != nil || q.Amount.DiscountMinor != 10000 || q.Amount.TotalMinor != 90000 {
		t.Fatalf("quote: %v %+v", err, q.Amount)
	}
	hold, err := b.pgBegin(q, nil)
	if err != nil {
		t.Fatal(err)
	}
	o := e.payHold(hold, b)
	if o.captured != 90000 {
		t.Fatalf("captured %d, want the discounted total 90000", o.captured)
	}
	// Stripe is asked for exactly the discounted total (one line item); a different amount would be the pre-discount 100000
	var unit, wantUnit, preUnit int64
	if err := e.p.f.owner.QueryRow(ctx, `SELECT s.unit_amount,payments.stripe_unit_amount('TWD',90000),payments.stripe_unit_amount('TWD',100000) FROM payments.stripe_sessions s WHERE s.attempt_id=$1`, o.attempt).Scan(&unit, &wantUnit, &preUnit); err != nil || unit != wantUnit || unit == preUnit {
		t.Errorf("Stripe session unit amount %d, want %d (pre-discount would be %d): %v", unit, wantUnit, preUnit, err)
	}
	if _, _, captured, refunded := e.pgFinance(); captured != 90000 || refunded != 0 {
		t.Errorf("finance captured %v refunded %v, want 90000 / 0", captured, refunded)
	}
	st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+o.order, "", "")
	if totals, _ := out["totals"].(map[string]any); st != 200 || totals["discount_minor"] != float64(10000) || totals["total_minor"] != float64(90000) {
		t.Errorf("merchant order totals: %d %s", st, raw)
	}
	// refund cap: the PAID amount, not the subtotal and not subtotal+shipping
	if got := e.r.refundable(t, o); got != 90000 {
		t.Fatalf("refundable %d, want the paid amount 90000", got)
	}
	for name, amount := range map[string]int64{"the pre-discount subtotal": 100000, "one dollar above the paid amount": 90100} {
		if status, out := e.r.request(o, o.token(), t04Key("pg-over"), rfxBody(amount, "requested_by_customer", 90000)); status != 422 || srqCode(out) != "exceeds_refundable" {
			t.Errorf("refund of %s: %d %v, want 422 exceeds_refundable", name, status, out)
		}
	}
	id := e.r.mustRefund(t, o, 90000, "requested_by_customer") // exactly the paid amount is accepted
	if id == "" {
		t.Fatal("no refund id")
	}
	if got := e.r.refundable(t, o); got != 0 {
		t.Errorf("refundable after a full-paid-amount refund %d, want 0", got)
	}
	if status, out := e.r.request(o, o.token(), t04Key("pg-more"), rfxBody(100, "requested_by_customer", 0)); status != 422 || srqCode(out) != "exceeds_refundable" {
		t.Errorf("a refund beyond the cap: %d %v", status, out)
	}
}

// ---- PGT10 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateAdminAuthority(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	f := e.p.f
	url := "/v1/admin/stores/" + e.store() + "/promotions"
	id, ver := e.proMust("AUTHZ", nil)

	t.Run("no session, a buyer session, an expired and a revoked session", func(t *testing.T) {
		for name, token := range map[string]string{"none": "", "buyer": f.tokens["buyer"], "expired": f.tokens["expired"], "revoked": f.tokens["revoked"]} {
			for _, call := range []struct{ method, path, body string }{{"GET", url, ""}, {"POST", url, proBody(t, map[string]any{"code": "NOPE1", "kind": "percent", "percent": 5, "min_subtotal_minor": 0,
				"starts_at": nil, "ends_at": nil, "total_limit": nil, "per_buyer_limit": nil, "status": "active"})}} {
				key := ""
				if call.method == "POST" {
					key = t04Key("pg-401")
				}
				if st, _, raw := e.mcall(token, call.method, call.path, key, call.body); st != 401 && st != 403 {
					t.Errorf("%s session %s %s: want 401/403, got %d %s", name, call.method, call.path, st, raw)
				}
			}
		}
		if n := e.count(`SELECT count(*) FROM promotions.codes WHERE code='NOPE1'`); n != 0 {
			t.Errorf("an unauthenticated create wrote %d rows", n)
		}
	})

	// §D role bundles (contracts/storefront-v2.md): who may read and who may write discount codes
	roles := []struct {
		role        string
		list, write bool
	}{{"owner", true, true}, {"admin", true, true}, {"viewer", true, false}, {"fulfilment", false, false}, {"live_operator", false, false}}
	for _, r := range roles {
		t.Run("staff role "+r.role, func(t *testing.T) {
			rows, err := f.owner.Query(context.Background(), `SELECT unnest(identity.staff_role_permissions($1))`, r.role)
			if err != nil {
				t.Fatal(err)
			}
			var perms []string
			for rows.Next() {
				var p string
				_ = rows.Scan(&p)
				perms = append(perms, p)
			}
			rows.Close()
			if len(perms) == 0 {
				t.Fatalf("role %s has no permission bundle", r.role)
			}
			token := e.pgMemberOn(e.tenant(), e.store(), perms...)
			st, _, raw := e.mcall(token, "GET", url, "", "")
			if (st == 200) != r.list || (!r.list && st != 403) {
				t.Errorf("list as %s: %d %s (want allowed=%v)", r.role, st, raw, r.list)
			}
			code := "ROLE" + strings.ToUpper(strings.ReplaceAll(r.role, "_", ""))
			st, out := e.pgCreateAs(token, e.store(), code, nil)
			if (st == 200) != r.write || (!r.write && st != 403) {
				t.Errorf("create as %s: %d %v (want allowed=%v)", r.role, st, out, r.write)
			}
			upd := proFields(map[string]any{"percent": 55, "status": "paused"})
			upd["expected_version"] = ver
			st, _, raw = e.mcall(token, "POST", url+"/"+id, t04Key("pg-upd"), proBody(t, upd))
			if (st == 200) != r.write || (!r.write && st != 403) {
				t.Errorf("update as %s: %d %s (want allowed=%v)", r.role, st, raw, r.write)
			}
			if r.write {
				ver++ // the successful update bumped the version; keep the next role's CAS honest
			}
		})
	}
	t.Run("the rows written by the roles that may not write do not exist", func(t *testing.T) {
		for _, role := range []string{"VIEWER", "FULFILMENT", "LIVEOPERATOR"} {
			if n := e.count(`SELECT count(*) FROM promotions.codes WHERE code=$1`, "ROLE"+role); n != 0 {
				t.Errorf("a role without pricing:write created %s", role)
			}
		}
	})
	t.Run("audit rows carry the acting principal", func(t *testing.T) {
		if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action='promotions.created' AND principal_id IS NOT NULL`, e.store()); n < 3 {
			t.Errorf("created-audit rows with a principal: %d, want at least 3 (fixture, owner, admin)", n)
		}
	})
}

// ---- PGT11 ---------------------------------------------------------------------------------------------------------------------

func TestPromoGateIsolation(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	f := e.p.f
	svc := e.pgService(0, nil)
	// psSetup gives this test its own tenant and store A1; a second store of the SAME tenant is added as an owner-pool fixture. Tenant B / store B
	// come from the shared base fixture (another tenant entirely).
	storeA2 := randomUUID()
	mustExec(t, f.owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'pg-store-a2','TWD')`, e.tenant(), storeA2)
	tokenB := e.pgMemberOn(f.tenantB, f.storeB, "pricing:read", "pricing:write")
	tokenA2 := e.pgMemberOn(e.tenant(), storeA2, "pricing:read", "pricing:write")

	idA, verA := e.proMust("SHARED10", nil)
	if st, out := e.pgCreateAs(tokenB, f.storeB, "SHARED10", map[string]any{"percent": 50}); st != 200 { // the same text in another tenant is another code
		t.Fatalf("same code text in tenant B: %d %v", st, out)
	}
	if st, out := e.pgCreateAs(tokenA2, storeA2, "SHARED10", map[string]any{"percent": 30}); st != 200 { // and in another store of the same tenant
		t.Fatalf("same code text in store A2: %d %v", st, out)
	}
	if st, out := e.pgCreateAs(tokenB, f.storeB, "ONLYB", nil); st != 200 {
		t.Fatalf("ONLYB: %d %v", st, out)
	}
	if st, out := e.pgCreateAs(tokenA2, storeA2, "ONLYA2", nil); st != 200 {
		t.Fatalf("ONLYA2: %d %v", st, out)
	}

	t.Run("each store lists only its own codes", func(t *testing.T) {
		for _, tc := range []struct {
			token, store string
			n            int
			percent      float64
		}{{e.token(), e.store(), 1, 10}, {tokenB, f.storeB, 2, 0}, {tokenA2, storeA2, 2, 0}} {
			st, list, raw := e.pgList(tc.token, tc.store)
			if st != 200 || len(list) != tc.n {
				t.Errorf("store %s lists %d codes (%d): %s", tc.store[:8], len(list), st, raw)
			}
			for _, it := range list {
				if it.(map[string]any)["id"] == idA && tc.store != e.store() {
					t.Errorf("store %s lists store A1's code id", tc.store[:8])
				}
			}
		}
	})
	t.Run("admin routes refuse another tenant's and another store's path, and never leak or change it", func(t *testing.T) {
		for name, tc := range map[string]struct{ token, store string }{
			"tenant B session on store A1": {tokenB, e.store()}, "store A2 session on store A1": {tokenA2, e.store()},
			"store A1 session on tenant B": {e.token(), f.storeB}, "store A1 session on store A2": {e.token(), storeA2},
		} {
			st, _, raw := e.mcall(tc.token, "GET", "/v1/admin/stores/"+tc.store+"/promotions", "", "")
			if st != 403 && st != 404 || strings.Contains(string(raw), "SHARED10") || strings.Contains(string(raw), "ONLYB") || strings.Contains(string(raw), "ONLYA2") {
				t.Errorf("%s list: %d %s", name, st, raw)
			}
			if st, out := e.pgCreateAs(tc.token, tc.store, "INTRUDER", nil); st == 200 {
				t.Errorf("%s create succeeded: %v", name, out)
			}
			upd := proFields(map[string]any{"percent": 90})
			upd["expected_version"] = verA
			if st, _, _ := e.mcall(tc.token, "POST", "/v1/admin/stores/"+tc.store+"/promotions/"+idA, t04Key("pg-x"), proBody(t, upd)); st == 200 {
				t.Errorf("%s updated store A1's code", name)
			}
		}
		if n := e.count(`SELECT count(*) FROM promotions.codes WHERE id=$1 AND version=$2 AND percent=10`, idA, verA); n != 1 {
			t.Error("store A1's code was changed by a foreign scope")
		}
		if n := e.count(`SELECT count(*) FROM promotions.codes WHERE code='INTRUDER'`); n != 0 {
			t.Errorf("%d INTRUDER rows exist", n)
		}
	})
	t.Run("a buyer only ever meets his own store's codes", func(t *testing.T) {
		b := e.pgBuyer(100000)
		q, err := b.quoteCodeFor(svc, "shared10") // the same text exists in B (50%) and A2 (30%): only store A1's 10% applies
		if err != nil || q.Amount.DiscountMinor != 10000 {
			t.Errorf("same text, three stores: %v %+v", err, q.Amount)
		}
		for _, code := range []string{"onlyb", "onlya2"} {
			if _, err := b.quoteCodeFor(svc, code); err == nil || !strings.Contains(proCode(err), "promo_invalid") {
				t.Errorf("code %s of another store: want promo_invalid (indistinguishable from unknown), got %v", code, err)
			}
		}
		if _, err := b.quoteCodeFor(svc, "neverexisted"); err == nil || !strings.Contains(proCode(err), "promo_invalid") {
			t.Errorf("unknown code: %v", err)
		}
		// redemption rows are scoped to the placing store
		res, err := b.pgBegin(q, nil)
		if err != nil {
			t.Fatal(err)
		}
		if n := e.count(`SELECT count(*) FROM promotions.redemptions r WHERE r.order_id=$1 AND r.store_id=$2 AND r.tenant_id=$3 AND r.code_id=$4`, res.OrderID, e.store(), e.tenant(), idA); n != 1 {
			t.Errorf("redemption is not bound to store A1's code (%d)", n)
		}
	})
}

// pgStripeRevoke makes pwIsolatedFixture (payment_runtime_test.go) REVOKE the stray PUBLIC EXECUTE grants of BASE defect B1 in its disposable database.
// Only TestPromoGateStripeRefundCap sets it, and clears it on exit, so every other test still sees the real base.
var pgStripeRevoke bool

func revokeBaseDefaultExec(t *testing.T) {
	t.Helper()
	pgStripeRevoke = true
	t.Cleanup(func() { pgStripeRevoke = false })
}
