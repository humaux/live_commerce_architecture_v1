package payments

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/payuni"
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

func TestQueryFamilyBudgetAndRetry(t *testing.T) {
	valid := queryOperation{ActorKind: "BUYER_PAYMENT_QUERY", Provider: "payuni",
		Action: "payuni.query", Purpose: "transactional"}
	if !validQueryOperation(valid) {
		t.Fatal("buyer query family rejected")
	}
	for _, op := range []queryOperation{
		{ActorKind: "MERCHANT", Provider: "payuni", Action: "payuni.query", Purpose: "transactional"},
		{ActorKind: "BUYER_PAYMENT_QUERY", Provider: "payuni", Action: "payuni.refund", Purpose: "transactional"},
	} {
		if validQueryOperation(op) {
			t.Fatal("foreign operation accepted")
		}
	}
	if queryRetryDelay(2*time.Minute, "B") != 10*time.Minute ||
		queryRetryDelay(12*time.Minute, "B") != 12*time.Minute ||
		queryRetryDelay(2*time.Minute, "A") != 2*time.Minute {
		t.Fatal("query retry delay violated provider B minimum")
	}
}

func TestQueryPanicBecomesFixedFailure(t *testing.T) {
	client, err := payuni.NewQuery(payuni.Config{Environment: "SANDBOX", MerchantID: "TEST",
		HashKey: "12345678901234567890123456789012", HashIV: "1234567890123456"},
		mockQueryTransport(func(*http.Request) (*http.Response, error) { panic("secret from transport") }))
	if err != nil {
		t.Fatal(err)
	}
	_, err, panicked := queryOnce(context.Background(), accounts.PaymentQueryMaterial{
		Client: client, Expected: payuni.ExpectedTrade{MerTradeNo: "TEST_1", AmountTWD: 25,
			Currency: "TWD", Method: "payuni_credit"},
	})
	if !panicked || err != nil {
		t.Fatalf("panic was not sanitized: panic=%t err=%v", panicked, err)
	}
}
