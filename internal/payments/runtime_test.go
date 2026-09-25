package payments

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/jobqueue"
)

func TestWorkerClientRejectsInvalidInputsBeforeDatabase(t *testing.T) {
	opts := DefaultQueryWorkerOptions()
	keys := &accounts.Keyring{}
	for _, tc := range []struct {
		profile     string
		concurrency int
		transport   http.RoundTripper
	}{
		{"", 4, nil}, {"SANDBOX ", 4, nil}, {"SANDBOX", 0, nil},
		{"LIVE", 17, nil}, {"PROVIDER_MOCK", 4, nil},
		{"LIVE", 4, mockQueryTransport(func(*http.Request) (*http.Response, error) { return nil, nil })},
	} {
		config := opts
		config.MockTransport = tc.transport
		if _, err := NewWorkerClient(context.Background(), nil, keys, tc.profile, tc.concurrency, config); !errors.Is(err, errPaymentWorkerConfig) {
			t.Fatalf("invalid worker config accepted: profile=%q concurrency=%d err=%v", tc.profile, tc.concurrency, err)
		}
	}
	if jobqueue.ForProfile("PROVIDER_MOCK") != jobqueue.PaymentMock ||
		jobqueue.ForProfile("SANDBOX") != jobqueue.PaymentSandbox ||
		jobqueue.ForProfile("LIVE") != jobqueue.PaymentLive ||
		jobqueue.ForProfile("sandbox") != "" {
		t.Fatal("profile queue mapping changed")
	}
}
