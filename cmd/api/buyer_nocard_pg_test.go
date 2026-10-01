//go:build buyerintegration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestBuyerNoCardAssemblyRealPG is the ops-polish OP1 process-level gate: the real buildBuyerHandler assembly (the code main.go runs) over the
// isolated roles, once with COMMERCE_BUYER_PAYMENT_ENABLED=1 and once with =0, driven through its real HTTP handler. Written from the unit
// brief (OP1: no payable card -> options drop card, home rows absent, Begin with card is 422 card_unavailable, an exact replay of an order
// that already exists still answers). Invoked only by tests/foundation TestOpsPolishOP1APIAssembly after the fixture exists; the fixture
// passes a buyer capability token, its published storefront origin and one quoted home checkout body.
func TestBuyerNoCardAssemblyRealPG(t *testing.T) {
	if os.Getenv("LC_TEST_DATABASE_ALLOWED") != "1" || os.Getenv("LC_BUYER_NOCARD_GATE") != "1" {
		t.Fatal("use the isolated foundation no-card assembly gate (tests/foundation TestOpsPolishOP1APIAssembly)")
	}
	var in = map[string]string{}
	for _, name := range []string{"LC_BUYER_TEST_ISSUER_DSN", "LC_BUYER_TEST_RUNTIME_DSN", "LC_BUYER_TEST_CHECKOUT_DSN", "LC_BUYER_TEST_HOSTED_DSN",
		"LC_OPP_TOKEN", "LC_OPP_ORIGIN", "LC_OPP_MARKET", "LC_OPP_COUNTRY", "LC_OPP_BODY"} {
		if in[name] = os.Getenv(name); in[name] == "" {
			t.Fatalf("missing fixture setting %s", name)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	serve := func(t *testing.T, paymentEnabled string) *httptest.Server {
		t.Helper()
		values := buyerPaymentTestEnv()
		values["COMMERCE_BUYER_ISSUER_DATABASE_URL"] = in["LC_BUYER_TEST_ISSUER_DSN"]
		values["COMMERCE_BUYER_DATABASE_URL"] = in["LC_BUYER_TEST_RUNTIME_DSN"]
		values["COMMERCE_CHECKOUT_DATABASE_URL"] = in["LC_BUYER_TEST_CHECKOUT_DSN"]
		values["COMMERCE_HOSTED_DATABASE_URL"] = in["LC_BUYER_TEST_HOSTED_DSN"]
		values["COMMERCE_BUYER_PAYMENT_ENABLED"] = paymentEnabled
		config, err := loadBuyerConfig(func(name string) string { return values[name] }, "127.0.0.1:8080")
		if err != nil || !config.enabled || config.payment.enabled != (paymentEnabled == "1") {
			t.Fatalf("fixture buyer configuration rejected (payment=%s): %v", paymentEnabled, err)
		}
		h, closePools, err := buildBuyerHandler(ctx, config)
		if err != nil || h == nil {
			t.Fatalf("buildBuyerHandler(payment=%s): %v", paymentEnabled, err)
		}
		srv := httptest.NewServer(h)
		t.Cleanup(func() { srv.Close(); closePools() })
		return srv
	}
	call := func(t *testing.T, srv *httptest.Server, method, path, key string, body []byte) (int, map[string]any, []byte) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, srv.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Commerce-Buyer-BFF-Key", strings.Repeat("A", 43)) // buyerPaymentTestEnv's COMMERCE_BUYER_BFF_KEY
		req.Header.Set("X-Commerce-Storefront-Origin", in["LC_OPP_ORIGIN"])
		req.Header.Set("Authorization", "Bearer "+in["LC_OPP_TOKEN"])
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := (&http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return res.StatusCode, out, raw
	}
	homeListed := func(t *testing.T, srv *httptest.Server) (listed bool, raw []byte) {
		t.Helper()
		st, out, raw := call(t, srv, "GET", "/v1/buyer/checkout-options?market_id="+in["LC_OPP_MARKET"]+"&country="+in["LC_OPP_COUNTRY"], "", nil)
		if st != 200 {
			t.Fatalf("checkout-options: %d %s", st, raw)
		}
		items, _ := out["items"].([]any)
		for _, it := range items {
			if row, _ := it.(map[string]any); row != nil && row["delivery_kind"] == "home" {
				return true, raw
			}
		}
		return false, raw
	}
	body := []byte(in["LC_OPP_BODY"])
	var placed string

	t.Run("payment_on_lists_home_and_accepts_card", func(t *testing.T) {
		srv := serve(t, "1")
		if listed, raw := homeListed(t, srv); !listed {
			t.Fatalf("control: with payment on the home option must be listed: %s", raw)
		}
		st, out, raw := call(t, srv, "POST", "/v1/buyer/checkout", "opp-noc-first", body)
		if st != 200 {
			t.Fatalf("control: card Begin with payment on: %d %s", st, raw)
		}
		if placed, _ = out["order_id"].(string); placed == "" {
			t.Fatalf("no order id: %s", raw)
		}
	})

	t.Run("payment_off_drops_home_and_refuses_card", func(t *testing.T) {
		srv := serve(t, "0")
		if listed, raw := homeListed(t, srv); listed {
			t.Fatalf("with COMMERCE_BUYER_PAYMENT_ENABLED=0 a card-only home option must not be offered: %s", raw)
		}
		// a different key = a new order attempt: the card refusal is typed and final (never a retryable 5xx)
		st, out, raw := call(t, srv, "POST", "/v1/buyer/checkout", "opp-noc-second", body)
		if st != 422 || out["code"] != "card_unavailable" {
			t.Fatalf("card Begin with payment off: want 422 card_unavailable, got %d %s", st, raw)
		}
	})

	t.Run("replay_after_restart_without_payment", func(t *testing.T) {
		if placed == "" {
			t.Fatal("the first subtest did not place an order")
		}
		srv := serve(t, "0")
		st, out, raw := call(t, srv, "POST", "/v1/buyer/checkout", "opp-noc-first", body)
		if st != 200 || out["order_id"] != placed {
			t.Fatalf("an exact replay must still return order %s with payment off, got %d %s", placed, st, raw)
		}
	})
}
