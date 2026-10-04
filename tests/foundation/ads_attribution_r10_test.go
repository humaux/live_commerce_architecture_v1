package foundation_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/claims"
)

// atR10Order retains genuine Begin receipts and financial-route results. None
// of these tests insert captures, collection flags, attribution, or origins.
type atR10Order struct {
	kind   string
	cap    buyer.Capability
	result checkout.Result
	net    int64
}

func (x *atReportEnv) r10AfterPause(t *testing.T) time.Time {
	t.Helper()
	var paused time.Time
	if err := x.f.owner.QueryRow(x.ctx, `SELECT max(o.updated_at) FROM ads.remote_objects r JOIN integration.operations o ON o.id=r.operation_id WHERE r.draft_id=$1 AND r.kind='pause' AND o.state='SUCCEEDED'`, x.secondDraft).Scan(&paused); err != nil {
		t.Fatal(err)
	}
	// Meta webhook time is whole seconds. A comment must be causally later
	// than the actual pause, never a guessed future timestamp crossing it.
	if wait := time.Until(paused.Truncate(time.Second).Add(time.Second)); wait > 0 {
		time.Sleep(wait)
	}
	return time.Now()
}

func (x *atReportEnv) r10FinishOrder(t *testing.T, offline *tcvEnv, cap buyer.Capability, res checkout.Result, index int) atR10Order {
	t.Helper()
	o := atR10Order{cap: cap, result: res}
	switch index {
	case 3:
		o.kind = "confirmed_transfer"
		// I05: only the authenticated merchant confirmation makes a transfer
		// a paid fact. An unsubmitted proof is valid under the frozen K304 gate.
		if st, out := offline.cofDecide(offline.token(), res.OrderID, "confirm", t04Key("at-r10-confirm"), `{}`); st != 200 || out["state"] != "CONFIRMED" {
			t.Fatalf("R10 actual transfer confirmation=%d %v", st, out)
		}
	case 4:
		o.kind = "collected_cod"
		if st, _, raw := offline.mcall(offline.token(), "PUT", "/v1/admin/stores/"+offline.store()+"/orders/"+res.OrderID+"/shipment", t04Key("at-r10-ship"), mfxShip(0, "sf_express", "R10SYNTHETICCOD")); st != 200 {
			t.Fatalf("R10 actual COD shipment=%d %s", st, raw)
		}
		if st, out, raw := offline.record(offline.token(), res.OrderID, t04Key("at-r10-collect"), "PENDING", "collected"); st != 200 || out["collection_state"] != "COLLECTED" {
			t.Fatalf("R10 actual COD collection=%d %s", st, raw)
		}
	case 5:
		o.kind = "unpaid_transfer"
	case 6:
		o.kind = "expired_draft"
		// Time travel only the fixture clock; the real expiry function must
		// release its hold. Do not manufacture CANCELLED or rewrite a paid fact.
		bcDue(t, x.r.base.s.p.bcHarness, res)
		if got := bcExpire(t, x.r.base.s.p.bcHarness, res, 1); got != "EXPIRED" {
			t.Fatalf("R10 actual card hold expiry=%s", got)
		}
	case 7:
		o.kind = "cancelled_cod"
		if st, out, raw := offline.release(offline.token(), res.OrderID, t04Key("at-r10-cancel"), "cancel", "PENDING"); st != 200 || out["commercial_state"] != "CANCELLED" {
			t.Fatalf("R10 actual COD cancellation=%d %s", st, raw)
		}
	case 8:
		o.kind = "draft_card"
	default:
		t.Fatalf("unknown R10 fixture index=%d", index)
	}
	if index == 3 || index == 4 {
		if err := x.r.base.s.p.f.owner.QueryRow(context.Background(), `SELECT total_minor+cod_surcharge_minor FROM checkout.orders WHERE id=$1`, res.OrderID).Scan(&o.net); err != nil {
			t.Fatal(err)
		}
	}
	return o
}

// REAL_PG + local Stripe/Meta MOCK when run: R10 forbids neutral-label
// workarounds. The shared browser fixture must render precisely these same
// collected-only aggregates after an unpaid/refunded/cancelled mixed cohort.
func TestAdsAttributionR10PaidOnlyMixedCohort(t *testing.T) {
	x := atNewReportEnv(t)
	f := x.r.base.s.p.f
	for _, o := range x.r10 {
		var state, mode string
		var collection *string
		if err := f.owner.QueryRow(context.Background(), `SELECT commercial_state,payment_mode,collection_state FROM checkout.orders WHERE id=$1`, o.result.OrderID).Scan(&state, &mode, &collection); err != nil {
			t.Fatal(err)
		}
		wantState := map[string]string{"confirmed_transfer": "CONFIRMED", "collected_cod": "AWAITING_COLLECTION", "unpaid_transfer": "AWAITING_TRANSFER", "expired_draft": "CANCELLED", "cancelled_cod": "CANCELLED", "draft_card": "DRAFT"}[o.kind]
		if o.kind == "collected_cod" {
			// Collection leaves the commercial state AWAITING_COLLECTION; it must carry
			// the independently recorded collection fact as well.
			if mode != "cash_on_delivery" || collection == nil || *collection != "COLLECTED" {
				t.Fatalf("R10 collected fixture missing money fact: %s/%v", mode, collection)
			}
		}
		if state != wantState {
			t.Fatalf("R10 %s state=%s want=%s", o.kind, state, wantState)
		}
	}
	x.assertReport(t)
}

func (x *atReportEnv) r10AssertBuyerCounts(t *testing.T, buyers map[string]any, orders, net, newBuyers, returningBuyers, quantity int64) {
	t.Helper()
	atNum(t, buyers, "new_buyers", newBuyers)
	atNum(t, buyers, "returning_buyers", returningBuyers)
	if orders == 0 {
		if buyers["average_order_minor"] != nil {
			t.Fatal("empty paid cohort average must stay NULL")
		}
	} else {
		atNum(t, buyers, "average_order_minor", math.Round(float64(net)/float64(orders)))
	}
	var minutesOrders, minutesNet, countyOrders, countyNet, productQuantity float64
	for _, raw := range buyers["orders_per_minute"].([]any) {
		row := raw.(map[string]any)
		if row["orders"].(float64) <= 0 {
			t.Fatal("unpaid-only minute leaked into buyer timeline")
		}
		minutesOrders += row["orders"].(float64)
		minutesNet += row["net_minor"].(float64)
	}
	for _, raw := range buyers["counties"].([]any) {
		row := raw.(map[string]any)
		countyOrders += row["orders"].(float64)
		countyNet += row["net_minor"].(float64)
	}
	for _, raw := range buyers["top_products"].([]any) {
		productQuantity += raw.(map[string]any)["quantity"].(float64)
	}
	if minutesOrders != float64(orders) || countyOrders != float64(orders) || minutesNet != float64(net) || countyNet != float64(net) || productQuantity != float64(quantity) {
		t.Fatalf("R10 buyer minute/county/product counts diverged: minutes=%v/%v counties=%v/%v quantity=%v want=%d/%d/%d", minutesOrders, minutesNet, countyOrders, countyNet, productQuantity, orders, net, quantity)
	}
}

type atR10Graph struct{ srv *httptest.Server }

func (x *atReportEnv) r10AudienceStates(t *testing.T) *atR10Graph {
	t.Helper()
	m, f := x.m, x.m.h.f
	x.stateSessions = map[string]string{}
	posts := map[string]string{m.pageAsset: m.postID}
	for _, state := range []string{"insufficient", "not_authorized"} {
		// Actual same-store sessions and registered Page custody; only the
		// missing-scope variant lacks read_insights. No fake report JSON.
		v := &mciEnv{t: t, h: m.h, page: m.page, pageKeys: m.pageKeys, stopConsumer: mciNoop, session: m.h.draft(t, f.storeA1), pageAsset: miAsset()}
		v.pageBinding = miBinding(t, v.page, v.pageAsset, "facebook", f.tenantA, f.storeA1, m.h.actor)
		miRoute(t, v.page, v.pageAsset, f.tenantA, f.storeA1, v.pageBinding)
		v.pageToken = "SENTINEL-R10-READONLY-PAGE-" + t04Tag()
		scopes := []string{"pages_read_engagement"}
		if state == "insufficient" {
			scopes = append(scopes, "read_insights")
		}
		v.registerToken(t, "facebook", v.pageBinding, v.pageAsset, scopes, v.pageToken)
		m.h.open(t, v.session, claims.MatchExact)
		v.postID = v.pageAsset + "_" + mciDigits(10)
		v.srcFB = v.mustSource(t, "page", v.pageAsset, v.postID, false)
		m.h.closeWindow(t, v.session)
		x.stateSessions[state] = v.session
		if state == "insufficient" {
			posts[v.pageAsset] = v.postID
		}
		t.Cleanup(func() {
			mustExec(t, f.owner, `DELETE FROM ads.live_audience_snapshots WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, v.session)
			v.cleanup()
		})
	}
	g := &atR10Graph{}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A GET-only local Graph mock discloses exactly what the provider
		// returns. Empty data -> insufficient with NULL facts, not zero people.
		if r.Method != http.MethodGet || r.URL.Query().Has("access_token") || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		asset := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/live_videos"), "/v26.0/")
		if post, ok := posts[asset]; ok && strings.HasSuffix(r.URL.Path, "/live_videos") {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"id": asset, "post_id": post}}})
			return
		}
		video := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/video_insights"), "/v26.0/")
		if _, ok := posts[video]; !ok || !strings.HasSuffix(r.URL.Path, "/video_insights") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		data := []any{}
		if video == m.pageAsset {
			data = []any{
				map[string]any{"name": "total_video_views", "period": "lifetime", "values": []any{map[string]any{"value": 34}}},
				map[string]any{"name": "total_video_view_time_by_age_bucket_and_gender", "period": "lifetime", "values": []any{map[string]any{"value": map[string]int64{"F.25-34": 1234}}}},
				map[string]any{"name": "total_video_view_time_by_region_id", "period": "lifetime", "values": []any{map[string]any{"value": map[string]int64{"Taipei": 4321}}}},
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (x *atReportEnv) r10AssertAudienceStates(t *testing.T, report map[string]any) {
	t.Helper()
	found := map[string]bool{}
	for _, raw := range report["sessions"].([]any) {
		s := raw.(map[string]any)
		for state, id := range x.stateSessions {
			if s["session_id"] != id {
				continue
			}
			found[state] = true
			a := s["live_audience"].(map[string]any)
			atNum(t, a, "status", state)
			for _, field := range []string{"views", "peak_concurrent", "total_view_time_ms"} {
				if a[field] != nil {
					t.Fatalf("R10 %s %s must be NULL", state, field)
				}
			}
			for _, field := range []string{"age_gender", "regions"} {
				if len(a[field].([]any)) != 0 {
					t.Fatalf("R10 %s %s must be empty", state, field)
				}
			}
			for _, field := range []string{"orders", "net_minor", "pending_orders", "pending_minor", "ambiguous_orders", "spend_minor"} {
				atNum(t, s, field, 0)
			}
			for _, field := range []string{"comments", "claims", "checkout_links", "paid_orders"} {
				atNum(t, s["funnel"].(map[string]any), field, 0)
			}
		}
	}
	if len(found) != 2 {
		t.Fatal("R10 actual insufficient/not_authorized sessions missing")
	}
}
