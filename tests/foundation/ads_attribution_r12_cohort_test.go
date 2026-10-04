package foundation_test

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/checkout"
	"livecommerce/internal/storefront"
)

// A distinct unboosted post and session, signed comment -> claim -> cart ->
// consent -> actual Begin -> signed Stripe capture. No attribution-row UPDATE.
func (x *atReportEnv) r12OrganicClaim(t *testing.T) map[string]any {
	t.Helper()
	r, p := x.r, x.r.base.s.p
	r.ensureStock(t, r.base)
	// Both consumers use river_meta/meta_inbox; retire the first fixture's
	// worker before using a second synthetic payload keyring on that queue.
	x.m.stopConsumer()
	m := atReportClaims(t, r.base)
	cap := mustIssue(t, m.h.service, p.f.storeA1)
	// Webhooks carry whole seconds; use a real time after the open window,
	// never a missing/future occurred_at fixture.
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	at := time.Now()
	sent := m.postFBTo(t, m.postID, "", "", "A1+2", &at, nil, true)
	m.apply(t)
	intake := m.mustIntake(t, "page", m.pageAsset, sent.comment)
	if intake.State != "APPLIED" {
		t.Fatal("organic comment did not apply")
	}
	ev := lcEvent(t, p.f, intake.AppliedEvent)
	if ev.outcome != "ACCEPTED" {
		t.Fatal("organic claim not accepted")
	}
	link := m.h.link(t, m.session, ev.bundle, 0, false)
	preview, err := m.h.preview(cap, link.Token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.h.redeem(cap, t04Key("r12-organic-redeem"), link.Token, preview.BundleVersion); err != nil {
		t.Fatal(err)
	}
	m.h.closeWindow(t, m.session)
	// Use the existing real buyer consent API with this new owner's capability.
	bh := bhHarness{bcHarness: p.bcHarness, key: base64.RawURLEncoding.EncodeToString(randomBytes(32)), origin: "https://r12-" + t04Tag() + ".example.test"}
	bhPublish(t, p.bcHarness, bh.origin, p.f.tenantA, p.f.storeA1)
	handler, err := buyerhttp.New(context.Background(), p.a.issuer, p.a.runtime, p.bcHarness.service, bh.key, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	bh.server = httptest.NewServer(handler)
	t.Cleanup(bh.server.Close)
	consent := bh.request(t, "PUT", "/v1/buyer/consents", cap.Token, t04Key("r12-consent"), map[string]any{"purpose": "ads_personalization", "channel": "meta_ads", "granted": true, "context": "settings"}, nil)
	if consent.status != 200 {
		t.Fatalf("organic consent=%d", consent.status)
	}
	cart := m.h.cartOf(t, cap)
	h := p.cqHarness
	h.cap = cap
	destIn := bdHome(cart)
	destIn.HomeAddress = storefront.HomeAddress{Region: "臺北市", City: "臺北市", Line1: atPrivateStreet}
	dest, err := bdSet(h, t04Key("r12-organic-home"), destIn)
	if err != nil {
		t.Fatal(err)
	}
	q, err := cqBuyer(h.a.runtime, cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
		return storefront.CreateQuote(ctx, tx, s, t04Key("r12-organic-quote"), storefront.QuoteInput{CartVersion: cart.Version, MarketID: h.market.ID, Country: "TW", Method: "delivery:" + p.delivery.Code})
	})
	if err != nil {
		t.Fatal(err)
	}
	fbc, fbp := atFBC, atFBP
	in := checkout.Input{QuoteID: q.ID, DestinationID: dest.ID, CartVersion: cart.Version, ServiceVersion: 1, AllocationVersion: 1, AdSignals: &checkout.AdSignals{FBC: &fbc, FBP: &fbp}, ClientIP: "192.0.2.12"}
	res, err := p.bcHarness.service.Begin(context.Background(), cap.Token, p.f.storeA1, t04Key("r12-organic-begin"), in)
	if err != nil {
		t.Fatal(err)
	}
	if n := x.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND path IS NULL AND draft_id IS NULL AND post_id IS NULL AND clicked_at IS NULL AND fbc=$2 AND fbp=$3 AND client_ip='192.0.2.12'::inet`, res.OrderID, fbc, fbp); n != 1 {
		t.Fatalf("actual organic Begin did not freeze exactly signals-only row=%d", n)
	}
	if n := x.count(`SELECT count(*) FROM claims.attribution_session_orders($1,$2,$3) AS origin(order_id) WHERE order_id=$4`, x.tenant, x.store, m.session, res.OrderID); n != 1 {
		t.Fatalf("actual organic claim origin=%d", n)
	}
	s := r.base.s
	s.p.cap, s.p.hold, s.p.bcHarness.input = cap, res, in
	paid := r.pay(t, s, r.base.endpoint, r.base.secret)
	return map[string]any{"session_id": m.session, "order_id": res.OrderID, "net_minor": paid.captured, "orders": 1}
}

func (x *atReportEnv) r12AssertOrganic(t *testing.T, organic map[string]any) {
	t.Helper()
	response := x.api("GET", "/attribution?from="+x.day+"&to="+x.day, x.token, nil, nil)
	if response.Status != 200 {
		t.Fatalf("organic report=%d", response.Status)
	}
	found := false
	for _, raw := range response.JSON["sessions"].([]any) {
		s := raw.(map[string]any)
		if s["session_id"] != organic["session_id"] {
			continue
		}
		found = true
		atNum(t, s, "orders", 1)
		atNum(t, s, "net_minor", organic["net_minor"])
		atNum(t, s, "pending_orders", 0)
		if len(s["draft_ids"].([]any)) != 0 {
			t.Fatal("organic signals were credited to a draft")
		}
		if n := x.count(`SELECT jsonb_array_length(orders.attribution_metrics($1,$2,$3::date,$3::date,NULL,$4)->'paths')`, x.tenant, x.store, x.day, organic["session_id"]); n != 0 {
			t.Fatalf("organic path credit=%d", n)
		}
		x.r10AssertBuyerCounts(t, s["buyers"].(map[string]any), 1, organic["net_minor"].(int64), 1, 0, 2)
		atNum(t, s["funnel"].(map[string]any), "paid_orders", 1)
	}
	if !found {
		t.Fatal("organic session excluded")
	}
}

func TestAdsAttributionR12PaymentSignalMinimization(t *testing.T) {
	for _, mode := range []string{"card", "cash_on_delivery", "bank_transfer", "pay_at_pickup"} {
		t.Run(mode, func(t *testing.T) {
			e := tcvNew(t, tcvOpts{stripe: true})
			e.grantCreator("integration:manage", "orders:read", "fulfillment:write")
			a := newAdsEnv(t, adsOpts{fx: e.p.f})
			touch := atLiveClick(t, a)
			b := e.newBuyer()
			if consent := b.req("PUT", "/v1/buyer/consents", t04Key("r12-consent"), map[string]any{"purpose": "ads_personalization", "channel": "meta_ads", "granted": true, "context": "settings"}, nil); consent.status != 200 {
				t.Fatalf("actual consent=%d", consent.status)
			}
			in := b.h.input
			in.PaymentMode = mode
			switch mode {
			case "cash_on_delivery":
				if st, _ := e.hcodSettings(0, true, 20000, 50, "black_cat"); st != 200 {
					t.Fatalf("COD settings=%d", st)
				}
			case "bank_transfer":
				if st, _ := e.cofSettings(0, true, false, 72); st != 200 {
					t.Fatalf("transfer settings=%d", st)
				}
			case "pay_at_pickup":
				code, version, allocation := e.service("cvs_711", "MANUAL", 0)
				e.cvsSettings(tcvAllChains, true, "20000", 500)
				pickup := e.tppEntered(b, code)
				dest, err := b.destination("cvs_711", pickup, tppName, tppPhone)
				if err != nil {
					t.Fatal(err)
				}
				quote, err := b.quote(code)
				if err != nil {
					t.Fatal(err)
				}
				in = checkout.Input{QuoteID: quote.ID, DestinationID: dest.ID, CartVersion: b.cartVersion(), ServiceVersion: version, AllocationVersion: allocation, PaymentMode: mode}
			}
			fbc, fbp := atFBC, atFBP
			in.AdTouch, in.AdSignals, in.ClientIP = touch, &checkout.AdSignals{FBC: &fbc, FBP: &fbp}, "192.0.2.12"
			key := t04Key("r12-payment-begin")
			res, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), key, in)
			if err != nil {
				t.Fatal(err)
			}
			if n := miCount(t, e.p.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND path='ad_click' AND draft_id=$2`, res.OrderID, touch.DraftID); n != 1 {
				t.Fatalf("payment mode lost factual attribution=%d", n)
			}
			signalRows := miCount(t, e.p.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND fbc=$2 AND fbp=$3 AND client_ip='192.0.2.12'::inet`, res.OrderID, fbc, fbp)
			if mode == "card" && signalRows != 1 {
				t.Fatal("card positive control lost consented signals")
			}
			if mode != "card" && miCount(t, e.p.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND (fbc IS NOT NULL OR fbp IS NOT NULL OR client_ip IS NOT NULL)`, res.OrderID) != 0 {
				t.Fatal("non-CAPI payment stored unnecessary matching signals")
			}
			replay, err := e.svc.Begin(context.Background(), b.cap.Token, e.store(), key, in)
			if err != nil || replay.OrderID != res.OrderID {
				t.Fatal("same-key Begin lost replay safety")
			}
		})
	}
}
