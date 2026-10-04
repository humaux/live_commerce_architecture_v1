package foundation_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/checkout"
	"livecommerce/internal/storefront"
)

func TestAdsAttributionR10IndependentBeginSignals(t *testing.T) {
	for _, kind := range []string{"no touch", "expired touch", "foreign touch", "no consent", "garbage signals", "duplicate header", "IP only"} {
		t.Run(kind, func(t *testing.T) {
			c := newCapiEnv(t, adsOpts{})
			b := c.p.bcHarness
			b.prepare(t, mustIssue(t, b.cqHarness.service, c.store), []storefront.Item{{SKUID: b.stock.skus[0].ID, Quantity: 1}})
			c.p.cap = b.cap
			if r := c.consent(kind != "no consent"); r.status != 200 {
				t.Fatalf("real consent route=%d", r.status)
			}
			fbc, fbp := atFBC, atFBP
			signals, _ := json.Marshal(checkout.AdSignals{FBC: &fbc, FBP: &fbp})
			wire := base64.RawURLEncoding.EncodeToString(signals)
			var touch *checkout.AdTouch
			if kind == "expired touch" {
				touch = atTouch(c.newDraft(adsDraftIn{}), 8*24*time.Hour)
			} else if kind == "foreign touch" {
				other := newAdsEnv(t, adsOpts{})
				touch = atTouch(other.newDraft(adsDraftIn{}), time.Minute)
			}
			touchJSON, _ := json.Marshal(touch)
			mutate := func(r *http.Request) {
				r.Header.Set("X-Commerce-Ad-Signals", wire)
				if touch != nil {
					r.Header.Set("X-Commerce-Ad-Touch", base64.RawURLEncoding.EncodeToString(touchJSON))
				}
				switch kind {
				case "garbage signals":
					r.Header.Set("X-Commerce-Ad-Signals", "malformed")
				case "duplicate header":
					r.Header.Add("X-Commerce-Ad-Signals", wire)
				case "IP only":
					r.Header.Del("X-Commerce-Ad-Signals")
					r.Header.Set("X-Commerce-Client-IP", "192.0.2.8")
				}
			}
			key := t04Key("r10-signals-begin")
			response := c.bh.request(t, "POST", "/v1/buyer/checkout", b.cap.Token, key, b.input, mutate)
			out := bhRead[struct {
				OrderID string `json:"order_id"`
			}](t, response, 200)
			want := kind != "no consent" && kind != "garbage signals" && kind != "duplicate header"
			if n := c.count(`SELECT count(*) FROM checkout.orders WHERE id=$1 AND creator_session_id=$2`, out.OrderID, b.cap.Scope.SessionID); n != 1 {
				t.Fatal("optional measurement prevented actual Begin commit")
			}
			rows := c.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=$1`, out.OrderID)
			if (rows == 1) != want {
				t.Fatalf("signals-only rows=%d want_present=%v", rows, want)
			}
			if !want {
				return
			}
			var path, draft, post, gotFBC, gotFBP, ip *string
			if err := c.f.owner.QueryRow(c.ctx, `SELECT path,draft_id::text,post_id,fbc,fbp,host(client_ip) FROM orders.order_attribution WHERE order_id=$1`, out.OrderID).Scan(&path, &draft, &post, &gotFBC, &gotFBP, &ip); err != nil {
				t.Fatal(err)
			}
			if path != nil || draft != nil || post != nil {
				t.Fatal("signals invented an attribution path")
			}
			if kind == "IP only" {
				if ip == nil || *ip != "192.0.2.8" || gotFBC != nil || gotFBP != nil {
					t.Fatal("IP-only context changed")
				}
			} else if gotFBC == nil || *gotFBC != fbc || gotFBP == nil || *gotFBP != fbp {
				t.Fatal("independent signed IDs not frozen")
			}
			before := lcStrings(t, c.f.owner, `SELECT row_to_json(a)::text FROM orders.order_attribution a WHERE order_id=$1`, out.OrderID)
			replay := c.bh.request(t, "POST", "/v1/buyer/checkout", b.cap.Token, key, b.input, nil)
			if replay.status != 200 {
				t.Fatal("Begin replay failed")
			}
			lcSameSet(t, "signals write once", lcStrings(t, c.f.owner, `SELECT row_to_json(a)::text FROM orders.order_attribution a WHERE order_id=$1`, out.OrderID), before)
			if strings.Contains(string(response.body), "fb.1.") || strings.Contains(string(replay.body), "192.0.2.8") {
				t.Fatal("private context leaked to buyer DTO")
			}
		})
	}
}

func TestAdsAttributionR10SignalsOnlyExcludedFromReports(t *testing.T) {
	x := atNewReportEnv(t)
	// R12 clarifies R10: exclude only path/draft CREDIT, never the order cohort.
	organic := x.r12OrganicClaim(t)
	x.r12AssertOrganic(t, organic)
	x.assertReport(t) // Neither existing draft's totals can absorb the organic order.
}
