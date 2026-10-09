//go:build browser

// Purpose: CDC04/CDC05 real-click claim -> checkout -> pay-at-pickup acceptance.
// Depends on: ltgEnv/ltRun, production storefront Next, real buyerhttp and isolated PG.
// Used by: --browser-claim-checkout and the seventh --browser-webkit step.
// Status: BROWSER + MOCK manual claims and CVS; no Meta send, PSP, carrier or live keys.
package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/claims"
)

type cdcLink struct {
	Token   string `json:"token"`
	Bundle  string `json:"bundle"`
	SKU     string `json:"sku"`
	Code    string `json:"code"`
	SoldSKU string `json:"sold_sku,omitempty"`
}

// TestBrowserClaimDirectCheckout requires both a browser journey and authoritative order readback.
func TestBrowserClaimDirectCheckout(t *testing.T) {
	if os.Getenv("LC_BROWSER_CLAIM_CHECKOUT_ACCEPTANCE") != "1" || os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" {
		t.Fatal("use --browser-claim-checkout")
	}
	t.Run("RealClick", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 850*time.Second)
		defer cancel()
		root, _ := filepath.Abs("../..")
		evidence := brfEvidence(t, root, "claim-checkout")
		e := ltgNew(t, tcvOpts{origin: sbOrigin})
		// psSetup inside tcvNew already owns a synthetic CARD order; count only this journey's delta.
		baseOrders := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store())
		e.grantCreator("fulfillment:write", "orders:read")
		e.cvsSettings(tcvAllChains, true, "20000", 500)
		e.service("cvs_711", "MANUAL", 0)
		fixtures := map[string]cdcLink{}
		for _, name := range []string{"zh-TW", "zh-CN", "en", "merge", "second", "pending", "partial", "sold", "expired", "repriced", "race", "repeat", "conflict", "begin-race", "archived"} {
			stock := int64(100)
			if name == "sold" {
				stock = 1
			}
			sku := e.sku(30000, stock)
			if name == "sold" {
				mustExec(t, e.p.f.owner, `UPDATE inventory.balances SET on_hand=0 WHERE sku_id=$1`, sku)
			}
			session := e.h.draft(t, e.store())
			e.h.open(t, session, claims.MatchExact)
			offer := e.h.livePriceOffer(t, session, "A1", sku, 5, 20000)
			c := e.h.accepted(t, session, "", "synthetic CDC "+name, "A1+2")
			sold := ""
			if name == "partial" {
				sold = e.sku(30000, 1)
				// Owner-only zero-stock fixture; the ledger command correctly rejects delta 0.
				mustExec(t, e.p.f.owner, `UPDATE inventory.balances SET on_hand=0 WHERE sku_id=$1`, sold)
				e.h.offer(t, session, "B2", sold, 5)
				c = e.h.accepted(t, session, c.BundleID, "", "B2")
			}
			link := e.h.link(t, session, c.BundleID, 0, false)
			e.h.closeWindow(t, session)
			if name == "expired" {
				e.setLinkExpiry(c.BundleID, -time.Minute)
			}
			fixtures[name] = cdcLink{Token: string(link.Token), Bundle: c.BundleID, SKU: sku, Code: offer.SKUCode, SoldSKU: sold}
		}
		transport := &claimsTransportLog{}
		api := httptest.NewServer(transport.wrap(e.bh.server.Config.Handler))
		t.Cleanup(api.Close)
		x := &ltRun{t: t, ctx: ctx, e: e, root: root, evidence: evidence, buyerAPI: api, buyerKey: e.bh.key, secrets: map[string]string{}}
		port := x.startStorefrontNext(t)
		target, _ := url.Parse("http://127.0.0.1:" + port)
		edge := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
		t.Cleanup(edge.Close)
		proxy := connectProxy(t, "buyer.example:443", edge.Listener.Addr().String())
		controlKey := brToken()
		control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" || r.Header.Get("X-CDC-Control") != controlKey {
				http.Error(w, "forbidden", 403)
				return
			}
			var in struct {
				Name   string `json:"name"`
				Action string `json:"action"`
				Order  string `json:"order,omitempty"`
				SKU    string `json:"sku,omitempty"`
			}
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in) != nil {
				http.Error(w, "invalid", 400)
				return
			}
			link, ok := fixtures[in.Name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if in.Action == "expire" {
				_, err := e.p.f.owner.Exec(ctx, `UPDATE claims.links SET issued_at=clock_timestamp()-interval '71 hours',expires_at=clock_timestamp()-interval '1 minute' WHERE bundle_id=$1`, link.Bundle)
				if err != nil {
					http.Error(w, "fixture failed", 500)
					return
				}
			} else if in.Action == "deplete" {
				// Owner-only fault fixture: models stock disappearing between B1 and B2/Begin; never a product stock writer.
				_, err := e.p.f.owner.Exec(ctx, `UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, e.tenant(), e.store(), link.SKU)
				if err != nil {
					http.Error(w, "fixture failed", 500)
					return
				}
			} else if in.Action == "ship-previous" {
				// SETUP: exercise the real merchant manual-shipment path on a synthetic order.
				// The existing per-owner pay-at-pickup cap forbids another unshipped order.
				status, _, _ := e.mcall(e.token(), "PUT", "/v1/admin/stores/"+e.store()+"/orders/"+in.Order+"/shipment", t04Key("cdc-ship"), mfxShip(0, "seven_eleven_cvs", "0012345678"))
				if status != 200 {
					http.Error(w, "fixture shipment failed", 500)
					return
				}
			} else if in.Action == "replenish" {
				_, err := e.p.f.owner.Exec(ctx, `UPDATE inventory.balances SET on_hand=reserved+allocated+unavailable+100 WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, e.tenant(), e.store(), link.SKU)
				if err != nil {
					http.Error(w, "fixture failed", 500)
					return
				}
			} else if in.Action == "archive-other" {
				_, err := e.p.f.owner.Exec(ctx, `UPDATE catalog.skus SET status='archived' WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, e.tenant(), e.store(), in.SKU)
				if err != nil {
					http.Error(w, "fixture failed", 500)
					return
				}
			} else if in.Action != "link" {
				http.Error(w, "invalid", 400)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(link)
		}))
		t.Cleanup(control.Close)
		cmd := exec.CommandContext(ctx, "node", "tests/storefront/claim-checkout.mjs")
		cmd.Dir = root
		cmd.Env = browserEnvironment(map[string]string{"LC_CDC_ORIGIN": sbOrigin, "LC_CDC_PROXY": proxy, "LC_CDC_EVIDENCE": evidence, "LC_CDC_CONTROL": control.URL, "LC_CDC_CONTROL_KEY": controlKey, "LC_CDC_PRODUCT": e.p.stock.product.ID})
		log := browserLog(t, filepath.Join(evidence, "browser.log"))
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Run(); err != nil {
			t.Fatalf("CDC browser failed: %v; evidence=%s", err, evidence)
		}
		var result struct {
			Cases  int      `json:"cases"`
			Orders []string `json:"orders"`
		}
		raw, err := os.ReadFile(filepath.Join(evidence, "result.json"))
		if err != nil || json.Unmarshal(raw, &result) != nil || result.Cases != 19 || len(result.Orders) != 6 {
			t.Fatalf("missing CDC cases/order result: %v", err)
		}
		for _, id := range result.Orders {
			var state, mode string
			if err := e.p.f.owner.QueryRow(ctx, `SELECT commercial_state,payment_mode FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, e.tenant(), e.store(), id).Scan(&state, &mode); err != nil || state != "CONFIRMED" || mode != "pay_at_pickup" {
				t.Fatalf("CDC order readback %s: %s %s %v", id, state, mode, err)
			}
		}
		if n := e.count(`SELECT count(*) FROM checkout.orders WHERE store_id=$1`, e.store()); n-baseOrders != len(result.Orders) {
			t.Fatalf("CDC duplicate order: persisted delta=%d browser=%d (baseline=%d)", n-baseOrders, len(result.Orders), baseOrders)
		}
		var doubleOrders int
		if err := e.p.f.owner.QueryRow(ctx, `SELECT count(*) FROM checkout.orders o JOIN storefront.quotes q ON q.tenant_id=o.tenant_id AND q.store_id=o.store_id AND q.id=o.quote_id WHERE o.store_id=$1 AND EXISTS(SELECT 1 FROM jsonb_array_elements(q.snapshot->'lines') l WHERE l->>'sku_id'=$2)`, e.store(), fixtures["repeat"].SKU).Scan(&doubleOrders); err != nil || doubleOrders != 1 {
			t.Fatalf("double-click order count=%d %v", doubleOrders, err)
		}
		for _, f := range fixtures {
			u, m, _ := transport.audit(f.Token)
			if u != 0 || m != 0 {
				t.Fatal("CDC token leaked outside claim header")
			}
		}
		// Persist the token-free pointer with this run's authoritative artifacts.
		dest := filepath.Join(evidence, "claim-direct-checkout")
		_ = os.MkdirAll(dest, 0755)
		name := "chromium"
		if strings.EqualFold(os.Getenv("LC_BROWSER_ENGINE"), "webkit") {
			name = "webkit"
		}
		_ = os.WriteFile(filepath.Join(dest, name+"-evidence.txt"), []byte(evidence+"\n"), 0600)
		t.Logf("PASS CDC04/CDC05 cases=%d orders=%d engine=%s evidence=%s", result.Cases, len(result.Orders), name, evidence)
	})
}
