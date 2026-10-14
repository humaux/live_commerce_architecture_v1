//go:build browser

// Purpose: prove Stripe MOCK setup pools close while retained buyer/payment services stay live.
// Depends on: sbNew's isolated real PostgreSQL fixture, fake Stripe transport and payment worker.
// Used by: TestBrowserTrackingBackfill and the standalone lifetime gate; no SANDBOX/LIVE claim.
package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/checkout"
)

func TestHarnessStripeMockPoolLifetime(t *testing.T) {
	harnessStripeMockPoolLifetime(t)
}

func harnessStripeMockPoolLifetime(t *testing.T) {
	t.Helper()
	e := sbNew(t, "MOCK")
	// Inspect the exact released pointers, rather than a planner or a close receipt.
	for name, pool := range map[string]*pgxpool.Pool{
		"identity": e.p.a.identity,
		"worker":   e.p.worker,
		"expiry":   e.p.expiry,
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		conn, err := pool.Acquire(ctx)
		contextErr := ctx.Err()
		cancel()
		if conn != nil {
			conn.Release()
		}
		if err == nil || contextErr != nil || pool.Stat().TotalConns() != 0 {
			t.Fatalf("Stripe MOCK setup pool %s accepted an acquisition, timed out, or retained sessions after Close", name)
		}
	}

	// A fresh capability has a distinct owner; only use it to prove issuer/runtime liveness.
	fresh := mustIssue(t, e.p.bcHarness.cqHarness.service, e.p.f.storeA1)
	assertScope(t, e.p.a.runtime, fresh, e.p.f.storeA1)
	h := bhHarness{bcHarness: e.p.bcHarness, server: e.api, key: e.bffKey, origin: sbOrigin}
	order := e.p.hold.OrderID
	response := h.request(t, http.MethodGet, "/v1/buyer/orders/"+order, e.p.cap.Token, "", nil, nil)
	var got struct {
		OrderID string `json:"order_id"`
	}
	if response.status != http.StatusOK || json.Unmarshal(response.body, &got) != nil || got.OrderID != order {
		t.Fatal("Stripe MOCK retained checkout HTTP read failed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Direct controls avoid /act's unbuffered Node driver channel.
	e.setMethod(t, false)
	view, err := e.svc.PaymentView(ctx, e.p.cap.Token, e.p.f.storeA1, order)
	if err != nil || len(view.Methods) != 0 {
		t.Fatal("Stripe MOCK retained registrar did not disable payment method")
	}
	e.setMethod(t, true)
	view, err = e.svc.PaymentView(ctx, e.p.cap.Token, e.p.f.storeA1, order)
	if err != nil || len(view.Methods) != 1 || view.Methods[0].Code != "stripe_checkout" || view.Methods[0].Version != e.method {
		t.Fatal("Stripe MOCK retained registrar did not enable current payment method")
	}
	in := checkout.HostedInput{OrderID: order, MethodCode: "stripe_checkout", MethodVersion: e.method, Locale: "en"}
	if _, err := e.svc.BeginHosted(ctx, e.p.cap.Token, e.p.f.storeA1, t04Key("mock-pool-lifetime"), in); err != nil {
		t.Fatal("Stripe MOCK retained hosted service could not begin payment")
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		handoff, err := e.svc.TakeHosted(ctx, e.p.cap.Token, e.p.f.storeA1, order)
		if err == nil && handoff.Disposition == "REDIRECT" && handoff.RedirectURL != "" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Stripe MOCK retained worker did not produce hosted redirect within budget")
		case <-ticker.C:
		}
	}
	t.Log("REAL_PG+PROVIDER_MOCK: three setup pools closed; retained issuer/runtime/checkout/registrar/worker live")
}
