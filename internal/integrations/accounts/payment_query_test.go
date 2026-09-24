package accounts

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type queryTransport func(*http.Request) (*http.Response, error)

func (f queryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPaymentQueryProfileFences(t *testing.T) {
	for _, tc := range []struct {
		profile, environment string
		valid                bool
	}{
		{"PROVIDER_MOCK", "SANDBOX", true}, {"SANDBOX", "SANDBOX", true},
		{"LIVE", "LIVE", true}, {"PROVIDER_MOCK", "LIVE", false},
		{"LIVE", "SANDBOX", false}, {"SANDBOX", "LIVE", false},
	} {
		if got := validQueryEnvironment(tc.profile, tc.environment); got != tc.valid {
			t.Errorf("%s/%s: got %t", tc.profile, tc.environment, got)
		}
	}
	if validQueryProfile("MOCK") || validQueryProfile("") {
		t.Fatal("unknown execution profile accepted")
	}
}

func TestLoadPaymentQueryRejectsUnscopedInputs(t *testing.T) {
	k := testKeyring(t)
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	token := make([]byte, 32)
	transport := queryTransport(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected wire call"); return nil, nil })
	for _, tc := range []struct {
		id, profile string
		generation  int64
		token       []byte
		mock        []http.RoundTripper
	}{
		{"invalid", "SANDBOX", 1, token, nil},
		{id, "SANDBOX", 0, token, nil},
		{id, "SANDBOX", 1, token[:31], nil},
		{id, "LIVE", 1, token, []http.RoundTripper{transport}},
		{id, "PROVIDER_MOCK", 1, token, []http.RoundTripper{nil}},
		{id, "PROVIDER_MOCK", 1, token, []http.RoundTripper{transport, transport}},
	} {
		if _, err := k.LoadPaymentQuery(context.Background(), nil, tc.id, tc.generation,
			tc.token, tc.profile, tc.mock...); !errors.Is(err, ErrPaymentQueryMaterial) {
			t.Fatalf("invalid material input returned %v", err)
		}
	}
}
