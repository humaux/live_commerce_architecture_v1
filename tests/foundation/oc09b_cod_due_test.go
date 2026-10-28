package foundation_test

// OC-09b (owner choice OC-Q5; contracts/buyer-http-v1.md quote amendment). Prefix `oc9b`. Tier HTTP_PG.
//
// Owns: the buyer quote's server-computed `cod_due_minor` — equal to the placed cash-on-delivery order's cod_collect_minor for the same
// cart and quote, absent whenever Begin would refuse cash on delivery for that quote (store switch off, per-order cap), and never
// readable outside the quote's tenant/store/owner scope.
// Never: the storefront rendering (tests/storefront/cod-footer.spec.ts, home-cod-buyer.mjs) or the COD lifecycle (home_cod_test.go).
// Depends-on: tcvEnv/tcvBuyer (taiwan_cvs_env_test.go), hcodSettings (home_cod_test.go), bhRead (buyer_http_test.go), t04Key.
// Used-by: the storefront checkout bottom bar 「到貨需付」 (apps/storefront/components/CheckoutFlow.tsx) reads the same field.
// Owner-pool writes: none (settings go through the merchant route, orders through checkout.Begin).

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/storefront"
)

// oc9bCodDuer is the checkout.Service read under test, asserted dynamically so this file compiles before the method exists (RED).
type oc9bCodDuer interface {
	CodDue(ctx context.Context, token, storeID, quoteID string) (*int64, error)
}

// oc9bQuote POSTs a quote for the buyer's cart and home service through the real buyerhttp route and returns its id and raw keys.
func oc9bQuote(t *testing.T, b *tcvBuyer) (string, map[string]json.RawMessage) {
	t.Helper()
	out := bhRead[map[string]json.RawMessage](t, b.req("POST", "/v1/buyer/quotes", t04Key("oc9b-quote"), storefront.QuoteInput{
		CartVersion: b.cartVersion(), MarketID: b.e.p.market.ID, Country: "TW", Method: "delivery:" + b.h.delivery.Code}, nil), 200)
	var id string
	if json.Unmarshal(out["id"], &id) != nil || id == "" {
		t.Fatal("quote has no id")
	}
	return id, out
}

func oc9bInt(t *testing.T, raw json.RawMessage, what string) int64 {
	t.Helper()
	var v int64
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s: not an integer: %s", what, raw)
	}
	return v
}

func oc9bTotal(t *testing.T, q map[string]json.RawMessage) int64 {
	t.Helper()
	var amount map[string]json.RawMessage
	if json.Unmarshal(q["amount"], &amount) != nil {
		t.Fatal("quote has no amount")
	}
	return oc9bInt(t, amount["total_minor"], "total_minor")
}

func TestOC09bQuoteCodDue(t *testing.T) {
	e := tcvNew(t)
	ctx := context.Background()
	if st, out := e.hcodSettings(0, true, 20000, 50, "black_cat"); st != 200 {
		t.Fatalf("enable COD: %d %v", st, out)
	}

	var placedQuote string
	var placer *tcvBuyer
	t.Run("cod_due_minor equals the placed order's cod_collect_minor (same cart, same quote)", func(t *testing.T) {
		b := e.newBuyer()
		id, q := oc9bQuote(t, b)
		raw, present := q["cod_due_minor"]
		if !present {
			t.Fatalf("COD-enabled store: quote must carry cod_due_minor; keys %v", oc9bKeys(q))
		}
		due, total := oc9bInt(t, raw, "cod_due_minor"), oc9bTotal(t, q)
		if due != total+5000 {
			t.Errorf("cod_due_minor %d, want quote total %d + surcharge 5000", due, total)
		}
		// GET of the same quote reports the same due (no client arithmetic needed on recovery either).
		again := bhRead[map[string]json.RawMessage](t, b.req("GET", "/v1/buyer/quotes/"+id, "", nil, nil), 200)
		if r, ok := again["cod_due_minor"]; !ok || oc9bInt(t, r, "GET cod_due_minor") != due {
			t.Errorf("GET quote cod_due_minor %s, want %d", r, due)
		}
		in := b.h.input
		in.QuoteID, in.PaymentMode = id, "cash_on_delivery"
		res, err := e.svc.Begin(ctx, b.cap.Token, e.store(), t04Key("oc9b-begin"), in)
		if err != nil {
			t.Fatalf("COD Begin on the quoted cart: %v", err)
		}
		order := bhRead[map[string]json.RawMessage](t, b.req("GET", "/v1/buyer/orders/"+res.OrderID, "", nil, nil), 200)
		if collect := oc9bInt(t, order["cod_collect_minor"], "cod_collect_minor"); collect != due {
			t.Errorf("order cod_collect_minor %d != quote cod_due_minor %d", collect, due)
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND total_minor+cod_surcharge_minor=$2`, res.OrderID, due); n != 1 {
			t.Errorf("stored order total+surcharge differs from the quoted due %d", due)
		}
		placedQuote, placer = id, b
	})

	t.Run("scope: another owner or another store never reads a due for the quote", func(t *testing.T) {
		if placer == nil {
			t.Skip("needs the placed quote")
		}
		other := e.newBuyer()
		r := other.req("GET", "/v1/buyer/quotes/"+placedQuote, "", nil, nil)
		if r.status != 404 {
			t.Errorf("another owner's GET of the quote: %d %s, want 404", r.status, r.body)
		}
		var leaked map[string]json.RawMessage
		if json.Unmarshal(r.body, &leaked) == nil {
			if _, ok := leaked["cod_due_minor"]; ok {
				t.Errorf("refusal body carries cod_due_minor: %s", r.body)
			}
		}
		svc, ok := any(e.svc).(oc9bCodDuer)
		if !ok {
			t.Fatal("checkout.Service has no CodDue(ctx, token, storeID, quoteID) read")
		}
		if due, err := svc.CodDue(ctx, other.cap.Token, e.store(), placedQuote); due != nil || !errors.Is(err, command.ErrNotFound) {
			t.Errorf("other owner, same store: due=%v err=%v, want nil + not found", due, err)
		}
		if due, err := svc.CodDue(ctx, placer.cap.Token, e.p.f.storeA2, placedQuote); due != nil || !errors.Is(err, buyer.ErrUnauthorized) {
			t.Errorf("same token, other store of the tenant: due=%v err=%v, want nil + unauthorized", due, err)
		}
		if due, err := svc.CodDue(ctx, placer.cap.Token, e.store(), placedQuote); err != nil || due == nil || *due <= 0 {
			t.Errorf("control: the owner's own scope reads the due: %v %v", due, err)
		}
	})

	t.Run("absent when Begin would refuse cash on delivery: cap exceeded, then store switch off", func(t *testing.T) {
		// NT$70 cap < the harness total NT$25 + NT$50 surcharge.
		if st, out := e.hcodSettings(1, true, 70, 50, "black_cat"); st != 200 {
			t.Fatalf("lower cap: %d %v", st, out)
		}
		b := e.newBuyer()
		id, q := oc9bQuote(t, b)
		if r, present := q["cod_due_minor"]; present {
			t.Errorf("over the cap: cod_due_minor %s must be absent", r)
		}
		in := b.h.input
		in.QuoteID, in.PaymentMode = id, "cash_on_delivery"
		if _, err := e.svc.Begin(ctx, b.cap.Token, e.store(), t04Key("oc9b-cap"), in); err == nil {
			t.Error("control: Begin must refuse the COD order the quote reported no due for")
		}
		if st, out := e.hcodSettings(2, false, 20000, 50, "black_cat"); st != 200 {
			t.Fatalf("disable COD: %d %v", st, out)
		}
		_, q = oc9bQuote(t, e.newBuyer())
		if r, present := q["cod_due_minor"]; present {
			t.Errorf("COD off: cod_due_minor %s must be absent", r)
		}
	})
}

func oc9bKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
