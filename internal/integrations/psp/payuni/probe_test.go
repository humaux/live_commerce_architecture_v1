// Purpose: PA07 client-level cases for Client.Probe: signed "no such trade" proves the key; everything else fails closed.
// Depends on: client_test.go helpers (testClient, roundTripFunc, queryEnvelope); no network (MOCK transport).
// Used by: go test ./internal/integrations/psp/payuni.
// Status: MOCK.

package payuni

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func probeWith(t *testing.T, c *Client, body string, transportErr error) error {
	t.Helper()
	calls := 0
	c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://sandbox-api.payuni.com.tw/api/trade/query" || r.Method != http.MethodPost {
			t.Fatalf("probe must be exactly one query call, got %s %s", r.Method, r.URL)
		}
		if transportErr != nil {
			return nil, transportErr
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	err := c.Probe(context.Background(), "PROBE_unused_1", 1760000000)
	if calls != 1 {
		t.Fatalf("wire calls = %d, want 1 (UNKNOWN is never auto-retried)", calls)
	}
	return err
}

// signedNoTrade is a reply authenticated with c's key whose inner part carries a non-SUCCESS status and no Result row.
func signedNoTrade(t *testing.T, c *Client, status string) string {
	t.Helper()
	inner := url.Values{"Status": {status}, "Message": {"no such trade"}}
	encryptInfo, hashInfo, err := c.seal(inner.Encode())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]string{"Status": status, "MerID": "AAA", "Version": "2.0", "EncryptInfo": encryptInfo, "HashInfo": hashInfo})
	return string(b)
}

func TestProbeSignedNoTradeProvesCredentials(t *testing.T) {
	c := testClient(t)
	if err := probeWith(t, c, signedNoTrade(t, c, "TRADE_NOT_EXIST"), nil); err != nil {
		t.Fatalf("signed no-trade reply must verify the key: %v", err)
	}
}

func TestProbeFailsClosed(t *testing.T) {
	c := testClient(t)
	other, err := New(Config{Environment: "SANDBOX", MerchantID: "AAA", HashKey: strings.Repeat("k", 32), HashIV: strings.Repeat("i", 16),
		ReturnURL: "https://shop.example.com/pay/return", NotifyURL: "https://shop.example.com/pay/notify"})
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		body string
		net  error
		want error
	}{
		"signed with the wrong key": {signedNoTrade(t, other, "TRADE_NOT_EXIST"), nil, ErrAuthentication},
		"unsigned reply":            {`{"Status":"TRADE_NOT_EXIST","Message":"x"}`, nil, ErrUncertain},
		"trade row present":         {queryEnvelope(t, c, baseObservation()), nil, ErrProbeTradeFound},
		"SUCCESS without a row":     {signedNoTrade(t, c, "SUCCESS"), nil, ErrUncertain},
		"transport failure":         {"", errors.New("boom"), ErrTransport},
	} {
		t.Run(name, func(t *testing.T) {
			got := probeWith(t, c, tc.body, tc.net)
			if got == nil || !errors.Is(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("timeout is UNKNOWN", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		})
		if err := c.Probe(ctx, "PROBE_unused_1", 1760000000); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout must surface as the context error, got %v", err)
		}
	})
	if err := c.Probe(context.Background(), "bad trade!", 1760000000); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid trade id: %v", err)
	}
}
