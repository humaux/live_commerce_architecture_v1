package foundation_test

// PRO01-PRO05 (contracts/storefront-v2.md §F, unit promotions, migration 0091): REAL_PG author smoke of discount codes through the real
// merchant HTTP routes, the real buyer quote + checkout.Begin path and the real expiry function. Evidence label of every passing line: REAL_PG
// (no PSP is involved). Owner-pool writes (disclosed fixtures): aging an order for expire_held (as cofAge does) and reading the result tables.
//   PRO01 TestPromotionQuoteApplication   code -> quote discount (any case), snapshot freeze, every typed refusal, code-less quote unchanged
//   PRO02 TestPromotionAtomicLimit        N concurrent placements of a total_limit 1 code: exactly one order; cancel/expiry frees the use
//   PRO03 TestPromotionPerBuyerAndChange  per-buyer limit by phone identity across two capabilities; promo_changed on pause with zero facts
//   PRO04 TestPromotionOrderMoney         order total = discounted quote total, snapshot + merchant view carry the discount, free-shipping policy
//                                         still places (RevalidateQuote pointer-compare regression), refund paths read the captured amount
//   PRO05 TestPromotionAdminGuards        permissions, idempotent replay, duplicate code, stale version, rule violations, audit rows

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/storefront"
)

// proFields is the full create/update field set with defaults; overrides replace keys.
func proFields(over map[string]any) map[string]any {
	f := map[string]any{"kind": "percent", "percent": 10, "fixed_minor": nil, "min_subtotal_minor": 0, "starts_at": nil, "ends_at": nil,
		"total_limit": nil, "per_buyer_limit": nil, "status": "active"}
	for k, v := range over {
		f[k] = v
	}
	return f
}

func proBody(t *testing.T, m map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func (e *tcvEnv) proCreate(code string, over map[string]any) (int, map[string]any) {
	e.t.Helper()
	f := proFields(over)
	f["code"] = code
	st, out, _ := e.mcall(e.token(), "POST", "/v1/admin/stores/"+e.store()+"/promotions", t04Key("pro-create"), proBody(e.t, f))
	return st, out
}

func (e *tcvEnv) proUpdate(id string, version int64, over map[string]any) (int, map[string]any) {
	e.t.Helper()
	f := proFields(over)
	f["expected_version"] = version
	st, out, _ := e.mcall(e.token(), "POST", "/v1/admin/stores/"+e.store()+"/promotions/"+id, t04Key("pro-update"), proBody(e.t, f))
	return st, out
}

func (e *tcvEnv) proMust(code string, over map[string]any) (id string, version int64) {
	e.t.Helper()
	st, out := e.proCreate(code, over)
	if st != 200 {
		e.t.Fatalf("create %s: %d %v", code, st, out)
	}
	return out["id"].(string), int64(out["version"].(float64))
}

// quoteCode quotes the buyer's cart with a code through the domain function (the buyer HTTP route is exercised in PRO01).
func (b *tcvBuyer) quoteCode(code string) (storefront.Quote, error) {
	return b.quoteCodeFor(b.e.p.delivery.Code, code)
}

func (b *tcvBuyer) quoteCodeFor(service, code string) (storefront.Quote, error) {
	return cqBuyer(b.h.a.runtime, b.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.CreateQuote(ctx, tx, s, t04Key("pro-quote"), storefront.QuoteInput{CartVersion: b.cartVersion(), MarketID: b.e.p.market.ID,
			Country: "TW", Method: "delivery:" + service, PromoCode: code})
	})
}

// proAge ages a DRAFT card order past its 15-minute hold (owner-pool fixture; cofAge is for the long bank-transfer window).
func (e *tcvEnv) proAge(order string) {
	e.t.Helper()
	mustExec(e.t, e.p.f.owner, `UPDATE checkout.orders SET created_at=clock_timestamp()-interval '16 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, order)
	mustExec(e.t, e.p.f.owner, `UPDATE inventory.reservations SET created_at=clock_timestamp()-interval '16 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE id=$1`, order)
}

// beginQuote places the buyer's harness destination with the given quote through the real Begin.
func (b *tcvBuyer) beginQuote(q storefront.Quote) (checkout.Result, error) {
	in := b.h.input
	in.QuoteID = q.ID
	return b.e.svc.Begin(context.Background(), b.cap.Token, b.e.store(), t04Key("pro-begin"), in)
}

// proCode is the refusal text: a coded promotion refusal's Error() is its contract code.
func proCode(err error) string { return err.Error() }

func (e *tcvEnv) proUsed(id string) int {
	return e.count(`SELECT count(*) FROM promotions.redemptions r JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id
		AND o.owner_id=r.owner_id AND o.id=r.order_id WHERE r.code_id=$1 AND o.commercial_state<>'CANCELLED'`, id)
}

func TestPromotionQuoteApplication(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	b := e.newBuyer()
	base, err := b.quoteCode("")
	if err != nil || base.Promotion != nil || base.Amount.DiscountMinor != 0 {
		t.Fatalf("a code-less quote is the pre-0091 quote: %v %+v", err, base.Amount)
	}
	sub := base.Amount.SubtotalMinor
	e.proMust("SAVE10", nil)
	q, err := b.quoteCode("  save10 ") // buyer types any case with stray spaces
	if err != nil {
		t.Fatalf("quote with a valid code: %v", err)
	}
	want := sub * 10 / 100
	want -= want % 100 // TWD is charged in whole dollars: the discount is floored to a whole dollar (pricing.wholeStep)
	if q.Promotion == nil || q.Promotion.Code != "SAVE10" || q.Promotion.Kind != "percent" || q.Promotion.Percent != 10 {
		t.Fatalf("snapshot promotion: %+v", q.Promotion)
	}
	if q.Amount.DiscountMinor != want || q.Amount.TotalMinor != base.Amount.TotalMinor-want {
		t.Fatalf("discount %d total %d want %d / %d", q.Amount.DiscountMinor, q.Amount.TotalMinor, want, base.Amount.TotalMinor-want)
	}
	var lineDiscount int64
	for _, l := range q.Lines {
		lineDiscount += l.Amount.DiscountMinor
	}
	if lineDiscount != want || q.Amount.ShippingMinor != base.Amount.ShippingMinor {
		t.Fatalf("line discounts %d, shipping %d (a code never discounts shipping)", lineDiscount, q.Amount.ShippingMinor)
	}
	// The historical read returns the frozen snapshot, including the promotion.
	got, err := cqBuyer(b.h.a.runtime, b.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.GetQuote(ctx, tx, s, q.ID)
	})
	if err != nil || got.Promotion == nil || got.Promotion.Version != q.Promotion.Version {
		t.Fatalf("snapshot readback: %v %+v", err, got.Promotion)
	}
	// Fixed amount above the subtotal is capped at the subtotal: goods never go below zero.
	e.proMust("BIGFIX", map[string]any{"kind": "fixed", "percent": nil, "fixed_minor": 99999900})
	if q, err = b.quoteCode("bigfix"); err != nil || q.Amount.DiscountMinor != sub-sub%100 || q.Amount.TotalMinor != base.Amount.TotalMinor-(sub-sub%100) {
		t.Fatalf("capped fixed: %v %+v", err, q.Amount)
	}
	// Every typed refusal is a coded 422 through the real buyer route; nothing is stored.
	now := time.Now().UTC()
	e.proMust("PAUSED", map[string]any{"status": "paused"})
	e.proMust("OLD", map[string]any{"ends_at": now.Add(-time.Hour).Format(time.RFC3339)})
	e.proMust("SOON", map[string]any{"starts_at": now.Add(time.Hour).Format(time.RFC3339)})
	e.proMust("BIG", map[string]any{"min_subtotal_minor": sub + 1})
	before := e.count(`SELECT count(*) FROM storefront.quotes WHERE owner_id=$1`, b.cap.Scope.OwnerID)
	for code, want := range map[string]string{"nope": "promo_invalid", "paused": "promo_invalid", "bad code!": "promo_invalid", "old": "promo_expired",
		"soon": "promo_not_started", "big": "promo_min_subtotal"} {
		r := b.req("POST", "/v1/buyer/quotes", t04Key("pro-http"), map[string]any{"cart_version": b.cartVersion(), "market_id": e.p.market.ID, "country": "TW",
			"method": "delivery:" + e.p.delivery.Code, "promo_code": code}, nil)
		if r.status != 422 || !strings.Contains(string(r.body), `"`+want+`"`) {
			t.Errorf("code %q: want 422 %s, got %d %s", code, want, r.status, r.body)
		}
	}
	if after := e.count(`SELECT count(*) FROM storefront.quotes WHERE owner_id=$1`, b.cap.Scope.OwnerID); after != before {
		t.Errorf("a refused code stored a quote: %d -> %d", before, after)
	}
	r := b.req("POST", "/v1/buyer/quotes", t04Key("pro-http"), map[string]any{"cart_version": b.cartVersion(), "market_id": e.p.market.ID, "country": "TW",
		"method": "delivery:" + e.p.delivery.Code, "promo_code": "save10"}, nil)
	var body struct {
		Amount struct {
			DiscountMinor int64 `json:"discount_minor"`
		} `json:"amount"`
		Promotion struct{ Code string } `json:"promotion"`
	}
	if err := json.Unmarshal(r.body, &body); r.status != 200 || err != nil || body.Amount.DiscountMinor != want || body.Promotion.Code != "SAVE10" {
		t.Errorf("buyer quote response: %d %s", r.status, r.body)
	}
}

func TestPromotionAtomicLimit(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	id, _ := e.proMust("ONLYONE", map[string]any{"total_limit": 1})
	const n = 6
	buyers := make([]*tcvBuyer, n)
	quotes := make([]storefront.Quote, n)
	for i := range buyers {
		buyers[i] = e.newBuyer()
		var err error
		if quotes[i], err = buyers[i].quoteCode("onlyone"); err != nil {
			t.Fatalf("quote %d: %v", i, err)
		}
	}
	// All six quotes were priced while the code was still available (advisory check); the placements race.
	results := make([]checkout.Result, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range buyers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = buyers[i].beginQuote(quotes[i])
		}(i)
	}
	close(start)
	wg.Wait()
	won := -1
	for i, err := range errs {
		switch {
		case err == nil && won < 0:
			won = i
		case err == nil:
			t.Fatalf("oversubscribed: buyers %d and %d both placed a total_limit 1 code", won, i)
		case !strings.Contains(proCode(err), "promo_used_up"):
			t.Errorf("buyer %d: want promo_used_up, got %v", i, err)
		}
	}
	if won < 0 {
		t.Fatal("no placement won")
	}
	if got := e.proUsed(id); got != 1 {
		t.Fatalf("usage %d, want exactly 1", got)
	}
	// Losers left no order, hold or redemption behind (the whole placement rolled back).
	for i, b := range buyers {
		if i == won {
			continue
		}
		if c := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, b.cap.Scope.OwnerID); c != 0 {
			t.Errorf("buyer %d: refused placement left %d orders", i, c)
		}
	}
	// Quote time now refuses too (advisory), and cancelling the winner's order frees the use.
	if _, err := buyers[(won+1)%n].quoteCode("onlyone"); err == nil || !strings.Contains(proCode(err), "promo_used_up") {
		t.Errorf("quote after the limit: %v", err)
	}
	e.proAge(results[won].OrderID)
	if d, _ := e.cofExpire(results[won].OrderID); d != "EXPIRED" {
		t.Fatalf("expire_held: %s", d)
	}
	if got := e.proUsed(id); got != 0 {
		t.Fatalf("usage after expiry %d, want 0 (expiry releases the use with no hook)", got)
	}
	next := buyers[(won+1)%n]
	q, err := next.quoteCode("onlyone")
	if err != nil {
		t.Fatalf("quote after release: %v", err)
	}
	if _, err = next.beginQuote(q); err != nil {
		t.Fatalf("placement after release: %v", err)
	}
	if got := e.proUsed(id); got != 1 {
		t.Fatalf("usage after re-use %d, want 1", got)
	}
}

func TestPromotionPerBuyerAndChange(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	id, _ := e.proMust("ONCE", map[string]any{"per_buyer_limit": 1})
	a, b := e.newBuyer(), e.newBuyer() // two capabilities, the harness gives both the same recipient phone
	qa, err := a.quoteCode("once")
	if err != nil {
		t.Fatal(err)
	}
	qb, err := b.quoteCode("once") // advisory: b has no redemption yet
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.beginQuote(qa); err != nil {
		t.Fatalf("first placement: %v", err)
	}
	if _, err = b.beginQuote(qb); err == nil || !strings.Contains(proCode(err), "promo_buyer_limit") {
		t.Fatalf("same phone, other capability: want promo_buyer_limit, got %v", err)
	}
	if got := e.proUsed(id); got != 1 {
		t.Fatalf("usage %d", got)
	}
	// Pausing after the quote: Begin answers promo_changed and leaves no facts.
	id2, ver2 := e.proMust("LATE", nil)
	c := e.newBuyer()
	qc, err := c.quoteCode("late")
	if err != nil {
		t.Fatal(err)
	}
	if st, out := e.proUpdate(id2, ver2, map[string]any{"status": "paused"}); st != 200 || out["version"] != float64(ver2+1) {
		t.Fatalf("pause: %d %v", st, out)
	}
	if _, err = c.beginQuote(qc); err == nil || !strings.Contains(proCode(err), "promo_changed") {
		t.Fatalf("paused after quote: want promo_changed, got %v", err)
	}
	if n := e.count(`SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, c.cap.Scope.OwnerID); n != 0 {
		t.Fatalf("promo_changed left %d orders", n)
	}
	if n := e.count(`SELECT count(*) FROM inventory.reservations WHERE buyer_owner_id=$1`, c.cap.Scope.OwnerID); n != 0 {
		t.Fatalf("promo_changed left %d reservations", n)
	}
	// A natural expiry between quote and Begin is promo_expired (no version bump).
	id3, _ := e.proMust("SHORT", nil)
	d := e.newBuyer()
	qd, err := d.quoteCode("short")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.p.f.owner, `UPDATE promotions.codes SET ends_at=clock_timestamp()-interval '1 second',starts_at=NULL WHERE id=$1`, id3)
	if _, err = d.beginQuote(qd); err == nil || !strings.Contains(proCode(err), "promo_expired") {
		t.Fatalf("expired after quote: want promo_expired, got %v", err)
	}
}

func TestPromotionOrderMoney(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write", "orders:read")
	// A fresh delivery service whose policy carries a free-shipping threshold: RevalidateQuote used to compare the policy by pointer
	// address and refuse every such quote at Begin. (A new policy version on the harness service would break its service binding.)
	in := e.p.delivery
	in.Code, in.ExpectedVersion = "pro"+t04Tag(), 0
	threshold := int64(1)
	pol := e.p.policy
	fee := int64(6000)
	pol.Method, pol.ExpectedVersion, pol.ShippingMinor, pol.Enabled, pol.FreeShippingThresholdMinor = "delivery:"+in.Code, 0, &fee, true, &threshold
	if _, err := e.p.setPolicy(pol); err != nil {
		t.Fatalf("set policy: %v", err)
	}
	if _, err := dsSet(e.p.cqHarness, t04Key("pro-service"), in); err != nil {
		t.Fatalf("service: %v", err)
	}
	if _, err := daSet(e.p.cqHarness, t04Key("pro-allocation"), fulfillment.AllocationInput{MarketID: e.p.market.ID, Country: "TW", Code: in.Code, ExpectedServiceVersion: 1, WarehouseIDs: []string{e.p.stock.warehouse.ID}}); err != nil {
		t.Fatal(err)
	}
	id, _ := e.proMust("TEN", nil)
	b := e.newBuyer()
	q, err := b.quoteCodeFor(in.Code, "ten")
	if err != nil {
		t.Fatal(err)
	}
	if q.Amount.ShippingMinor != 0 || q.Amount.DiscountMinor == 0 {
		t.Fatalf("threshold + code: %+v", q.Amount)
	}
	res, err := b.beginQuote(q)
	if err != nil {
		t.Fatalf("placement with a threshold policy and a code: %v", err)
	}
	var total, discount int64
	var code2 string
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT o.total_minor,r.discount_minor,c.code FROM checkout.orders o JOIN promotions.redemptions r ON r.order_id=o.id
		JOIN promotions.codes c ON c.id=r.code_id WHERE o.id=$1`, res.OrderID).Scan(&total, &discount, &code2); err != nil {
		t.Fatal(err)
	}
	if total != q.Amount.TotalMinor || discount != q.Amount.DiscountMinor || code2 != "TEN" {
		t.Fatalf("order total %d redemption %d/%s vs quote %+v", total, discount, code2, q.Amount)
	}
	order, err := e.svc.Get(context.Background(), b.cap.Token, e.store(), res.OrderID)
	if err != nil || order.Snapshot.Quote.Promotion == nil || order.Snapshot.Quote.Promotion.Code != "TEN" || order.Snapshot.Quote.Amount.DiscountMinor != discount {
		t.Fatalf("order snapshot: %v %+v", err, order.Snapshot.Quote.Promotion)
	}
	// Merchant order detail shows the discount in the totals (the existing discount_minor key; no new key).
	st, out, raw := e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/orders/"+res.OrderID, "", "")
	if st != 200 || fmt.Sprint(out["totals"].(map[string]any)["discount_minor"]) != fmt.Sprint(float64(discount)) {
		t.Fatalf("merchant order view: %d %s", st, raw)
	}
	// Admin list shows the usage count; a second code on the same cart is not a thing (one code per order: the quote carries one).
	st, out, _ = e.mcall(e.token(), "GET", "/v1/admin/stores/"+e.store()+"/promotions", "", "")
	list, _ := out["promotions"].([]any)
	if st != 200 || len(list) != 1 || list[0].(map[string]any)["used"] != float64(1) || list[0].(map[string]any)["id"] != id {
		t.Fatalf("admin list: %d %v", st, out)
	}
}

func TestPromotionAdminGuards(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	url := "/v1/admin/stores/" + e.store() + "/promotions"
	reader, _ := e.member("pricing:read")
	nobody, _ := e.member("catalog:read")
	if st, _, _ := e.mcall(reader, "GET", url, "", ""); st != 200 {
		t.Errorf("pricing:read lists: %d", st)
	}
	if st, _, _ := e.mcall(nobody, "GET", url, "", ""); st != 403 {
		t.Errorf("no pricing permission: want 403, got %d", st)
	}
	f := proFields(nil)
	f["code"] = "GUARD"
	body := proBody(t, f)
	if st, _, _ := e.mcall(reader, "POST", url, t04Key("pro-g1"), body); st != 403 {
		t.Errorf("pricing:read must not create: %d", st)
	}
	key := t04Key("pro-g2")
	st, first, _ := e.mcall(e.token(), "POST", url, key, body)
	st2, replay, _ := e.mcall(e.token(), "POST", url, key, body)
	if st != 200 || st2 != 200 || first["id"] != replay["id"] {
		t.Fatalf("create + exact replay: %d %d %v %v", st, st2, first, replay)
	}
	if n := e.count(`SELECT count(*) FROM promotions.codes WHERE code='GUARD'`); n != 1 {
		t.Fatalf("replay created %d rows", n)
	}
	if st, out, _ := e.mcall(e.token(), "POST", url, key, strings.Replace(body, `"percent":10`, `"percent":11`, 1)); st != 409 || out["code"] != "idempotency_conflict" {
		t.Errorf("same key, other body: %d %v", st, out)
	}
	if st, out := e.proCreate("guard", nil); st != 409 || out["code"] != "promo_exists" { // case-insensitive uniqueness
		t.Errorf("duplicate code: %d %v", st, out)
	}
	id, ver := first["id"].(string), int64(first["version"].(float64))
	if st, out := e.proUpdate(id, ver+5, nil); st != 409 || out["code"] != "version_changed" {
		t.Errorf("stale version: %d %v", st, out)
	}
	for name, over := range map[string]map[string]any{
		"percent 91":                     {"percent": 91},
		"percent with fixed":             {"fixed_minor": 5},
		"fixed without amount":           {"kind": "fixed", "percent": nil},
		"fixed not a whole dollar (TWD)": {"kind": "fixed", "percent": nil, "fixed_minor": 150},
		"end before start":               {"starts_at": "2026-10-02T00:00:00+08:00", "ends_at": "2026-10-01T00:00:00+08:00"},
		"zero total limit":               {"total_limit": 0},
		"unknown status":                 {"status": "deleted"},
		"negative minimum":               {"min_subtotal_minor": -1},
	} {
		if st, out := e.proUpdate(id, ver, over); st != 422 || out["code"] != "invalid_promotion" {
			t.Errorf("%s: want 422 invalid_promotion, got %d %v", name, st, out)
		}
	}
	for _, bad := range []string{"ab", "has space", "waytoolongcodewaytoolongcode1", "emoji😀"} {
		if st, out := e.proCreate(bad, nil); st != 422 || out["code"] != "invalid_promotion" {
			t.Errorf("code %q: %d %v", bad, st, out)
		}
	}
	if st, out := e.proUpdate(id, ver, map[string]any{"status": "paused", "percent": 20}); st != 200 || out["status"] != "paused" || out["percent"] != float64(20) || out["version"] != float64(ver+1) {
		t.Errorf("pause + edit: %d %v", st, out)
	}
	if n := e.count(`SELECT count(*) FROM ops.audit_events WHERE store_id=$1 AND action IN ('promotions.created','promotions.updated')`, e.store()); n != 2 {
		t.Errorf("audit rows: %d, want 2 (create, update; the replay writes none)", n)
	}
}

// promotions.redeem reads the code id, version and discount from the ORDER snapshot; this test forges that snapshot (owner-pool fixture) and
// proves SQL refuses a frozen effect that is not the code row's own terms or an impossible discount, which no Go path can produce.
func TestPromotionRedeemRejectsForgedSnapshot(t *testing.T) {
	e := tcvNew(t)
	e.grantCreator("pricing:read", "pricing:write")
	id, ver := e.proMust("REAL10", nil)
	b := e.newBuyer()
	res, err := b.beginQuote(mustQuote(t, b, ""))
	if err != nil {
		t.Fatal(err)
	}
	var subtotal int64
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT (snapshot#>>'{quote,amount,subtotal_minor}')::bigint FROM checkout.orders WHERE id=$1`, res.OrderID).Scan(&subtotal); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(b.cap.Token))
	try := func(name, promotion string, discount int64, want string) {
		t.Helper()
		forged := fmt.Sprintf(`{"id":%q,"code":"REAL10","version":%d,%s}`, id, ver, promotion)
		mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET snapshot=jsonb_set(jsonb_set(snapshot,'{quote,promotion}',$2::jsonb,true),'{quote,amount,discount_minor}',to_jsonb($3::bigint)) WHERE id=$1`, res.OrderID, forged, discount)
		tx, err := e.p.bcHarness.pool.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(context.Background())
		_, err = tx.Exec(context.Background(), `SELECT promotions.redeem($1,$2::uuid,$3::uuid,NULL,NULL)`, hash[:], e.store(), res.OrderID)
		if want == "" {
			if err != nil {
				t.Errorf("%s: want success, got %v", name, err)
			}
			return
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want %s, got %v", name, want, err)
		}
	}
	try("another percent than the code's", `"kind":"percent","percent":50`, subtotal/2, "promo_changed")
	try("a fixed effect on a percent code", `"kind":"fixed","fixed_minor":100`, 100, "promo_changed")
	try("discount above the code's own percent", `"kind":"percent","percent":10`, subtotal/10+100, "promo_changed")
	try("discount above the subtotal", `"kind":"percent","percent":10`, subtotal+100, "promo_changed")
	try("zero discount", `"kind":"percent","percent":10`, 0, "promo_changed")
	try("the code's own terms", `"kind":"percent","percent":10`, subtotal/10-subtotal/10%100, "")
}

func mustQuote(t *testing.T, b *tcvBuyer, code string) storefront.Quote {
	t.Helper()
	q, err := b.quoteCode(code)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
