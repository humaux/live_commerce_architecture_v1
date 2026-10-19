package payments

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"livecommerce/internal/integrations/accounts"
)

type mockQueryTransport func(*http.Request) (*http.Response, error)

func (f mockQueryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestQueryWorkerOptionsAndMockIsolation(t *testing.T) {
	o := DefaultQueryWorkerOptions()
	if !validQueryWorkerOptions(o) || o.LeaseSeconds != 30 || o.DBTimeout != 2*time.Second ||
		o.CallTimeout != 10*time.Second || o.RetryDelay != 2*time.Minute ||
		o.MaxAge != 24*time.Hour || o.MaxGenerations != 720 {
		t.Fatal("default query budget changed")
	}
	bad := o
	bad.CallTimeout = 25 * time.Second
	if validQueryWorkerOptions(bad) {
		t.Fatal("wire deadline can overrun lease")
	}
	bad = o
	bad.MaxAge = 0
	if validQueryWorkerOptions(bad) {
		t.Fatal("unbounded attempt age accepted")
	}
	transport := mockQueryTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("constructor attempted provider I/O")
		return nil, nil
	})
	for _, tc := range []struct {
		profile string
		mock    http.RoundTripper
	}{
		{"PROVIDER_MOCK", nil}, {"SANDBOX", transport}, {"LIVE", transport},
	} {
		in := o
		in.MockTransport = tc.mock
		if _, err := NewQueryWorker(context.Background(), nil, &accounts.Keyring{}, tc.profile, in); !errors.Is(err, errPaymentQueryJob) {
			t.Fatalf("%s mock transport fence: %v", tc.profile, err)
		}
	}
}

func TestQueryFamilyAdmitsOnlyStripe(t *testing.T) {
	valid := queryOperation{ActorKind: "BUYER_PAYMENT_QUERY", Provider: "stripe", Action: "stripe.checkout_session", Purpose: "transactional"}
	if !validQueryOperation(valid) {
		t.Fatal("Stripe query family rejected")
	}
	for _, op := range []queryOperation{
		{ActorKind: "BUYER_PAYMENT_QUERY", Provider: "payuni", Action: "payuni.query", Purpose: "transactional"},
		{ActorKind: "MERCHANT", Provider: "payuni", Action: "payuni.query", Purpose: "transactional"},
		{ActorKind: "BUYER_PAYMENT_QUERY", Provider: "payuni", Action: "payuni.refund", Purpose: "transactional"},
	} {
		if validQueryOperation(op) {
			t.Fatalf("removed operation dispatched: %+v", op)
		}
	}
}

func TestRemovedQueryProviderNeverDispatchesAnyStoredState(t *testing.T) {
	// Removal supersedes transport panic handling: this provider cannot enter a wire path at all.
	for _, state := range []string{"PENDING", "UNKNOWN", "SUCCEEDED", "FAILED_FINAL"} {
		op := queryOperation{ActorKind: "BUYER_PAYMENT_QUERY", Provider: "payuni", Action: "payuni.query", Purpose: "transactional", State: state}
		if validQueryOperation(op) {
			t.Fatalf("historical %s attempt could call a transport", state)
		}
	}
}
