// Package stripetest owns fake Stripe wire self-tests.
// It never calls api.stripe.com or admits production credentials.
// Depends on: Go httptest client and the frozen Stripe verifier.
// Used by: the MOCK gate before integration tests.
package stripetest

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"livecommerce/internal/integrations/psp/stripe"
)

func postCreate(t *testing.T, client *http.Client, endpoint, key string, values url.Values) (int, http.Header, []byte, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, endpoint+"/v1/checkout/sessions", bytes.NewBufferString(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, body, err
}

func testParams() url.Values {
	return url.Values{
		"mode": {"payment"}, "payment_method_types[0]": {"card"},
		"client_reference_id":                    {"0b5e3c1a-7d2f-4e8a-9c61-2f4d8e7a1b30"},
		"line_items[0][price_data][currency]":    {"usd"},
		"line_items[0][price_data][unit_amount]": {"500"},
		"expires_at":                             {"1790002400"},
		"metadata[lc_attempt]":                   {"0b5e3c1a-7d2f-4e8a-9c61-2f4d8e7a1b30"},
	}
}

func TestFakeStripeIdempotencyAndSignature(t *testing.T) {
	s := New("acct_FakeUnit1")
	defer s.Close()
	c := s.http.Client()
	p := testParams()
	status, _, first, err := postCreate(t, c, s.URL(), "same-key", p)
	if err != nil || status != 200 {
		t.Fatalf("first create status=%d err=%v", status, err)
	}
	status, h, replay, err := postCreate(t, c, s.URL(), "same-key", p)
	if err != nil || status != 200 || h.Get("Idempotent-Replayed") != "true" || !bytes.Equal(first, replay) || len(s.SessionIDs()) != 1 {
		t.Fatalf("same-key replay status=%d err=%v", status, err)
	}
	p.Set("line_items[0][price_data][unit_amount]", "600")
	status, _, body, err := postCreate(t, c, s.URL(), "same-key", p)
	if err != nil || status != 400 || !bytes.Contains(body, []byte("idempotency_error")) || len(s.SessionIDs()) != 1 {
		t.Fatalf("parameter mismatch status=%d err=%v", status, err)
	}
	secret := "whsec_fake_secret_not_real_0123456789"
	now := time.Unix(1790000000, 0)
	event := []byte(`{"id":"evt_fake1","object":"event","created":1790000000,"livemode":false,"type":"checkout.session.completed","data":{"object":{"object":"checkout.session","id":"cs_test_fake_1","client_reference_id":"0b5e3c1a-7d2f-4e8a-9c61-2f4d8e7a1b30","metadata":{"lc_attempt":"0b5e3c1a-7d2f-4e8a-9c61-2f4d8e7a1b30"}}}}`)
	verifier, err := stripe.NewWebhookVerifier(stripe.WebhookConfig{Secrets: []string{secret}, AccountID: "acct_FakeUnit1", Environment: "SANDBOX"})
	if err != nil {
		t.Fatal(err)
	}
	sig := SignWebhook(secret, event, now)
	got, err := verifier.Verify(event, sig, now)
	if err != nil || got.Malformed || got.ID != "evt_fake1" {
		t.Fatalf("signed event: malformed=%v err=%v", got.Malformed, err)
	}
	changed := append([]byte(nil), event...)
	changed[len(changed)-2] = 'x'
	if _, err = verifier.Verify(changed, sig, now); !errors.Is(err, stripe.ErrSignature) {
		t.Fatalf("modified raw body: %v", err)
	}
}

func TestFakeStripeCacheAndPreExecutionFaults(t *testing.T) {
	s := New("acct_FakeUnit1")
	defer s.Close()
	c := s.http.Client()
	p := testParams()
	s.SetNextFault(Fault{RateLimit: true})
	status, _, _, err := postCreate(t, c, s.URL(), "rate-key", p)
	if err != nil || status != 429 || len(s.SessionIDs()) != 0 {
		t.Fatalf("429 cached or executed: %d %v", status, err)
	}
	status, _, _, err = postCreate(t, c, s.URL(), "rate-key", p)
	if err != nil || status != 200 || len(s.SessionIDs()) != 1 {
		t.Fatalf("same key after 429: %d %v", status, err)
	}
	s.SetNextFault(Fault{Cached500: true})
	status, _, first, err := postCreate(t, c, s.URL(), "500-key", p)
	if err != nil || status != 500 {
		t.Fatalf("first 500: %d %v", status, err)
	}
	status, h, second, err := postCreate(t, c, s.URL(), "500-key", p)
	if err != nil || status != 500 || h.Get("Idempotent-Replayed") != "true" || !bytes.Equal(first, second) || len(s.SessionIDs()) != 2 {
		t.Fatalf("cached 500: %d %v", status, err)
	}
	s.SetNextFault(Fault{DropAfterExecute: true})
	c.CloseIdleConnections() // avoid net/http's transparent retry on a reused idempotent connection
	_, _, _, err = postCreate(t, c, s.URL(), "drop-key", p)
	if err == nil {
		t.Fatal("drop-after-execute returned a response")
	}
	status, h, _, err = postCreate(t, c, s.URL(), "drop-key", p)
	if err != nil || status != 200 || h.Get("Idempotent-Replayed") != "true" || len(s.SessionIDs()) != 3 {
		t.Fatalf("drop replay: %d %v", status, err)
	}
}
