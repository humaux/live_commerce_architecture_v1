// balance_test.go: unit tests of the balance-transaction read (contract §6.5, PF10 wire half) against a mock transport.
// Purpose: the exact projection, pagination, the 50-page and 8-day bounds, and fail-closed JSON handling.
// Depends on: balance.go, helpers_test.go (roundTripFunc, sandboxConfig). Used by: go test ./internal/integrations/psp/stripe/...
// Status: MOCK. No network; every id here is a fixture.

package stripe

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const (
	btCharge  = `{"id":"txn_charge01","object":"balance_transaction","amount":25640,"fee":1046,"net":24594,"currency":"hkd","exchange_rate":0.2564,"created":1790000100,"reporting_category":"charge","type":"charge","source":{"id":"ch_1","object":"charge","amount":100000,"currency":"twd","payment_intent":"pi_1","billing_details":{"email":"buyer@example.test"},"payment_method_details":{"card":{"last4":"4242"}}}}`
	btRefund  = `{"id":"txn_refund01","object":"balance_transaction","amount":-2564,"fee":0,"net":-2564,"currency":"hkd","exchange_rate":0.2564,"created":1790000200,"reporting_category":"refund","type":"refund","source":{"id":"re_1","object":"refund","amount":10000,"currency":"twd","payment_intent":"pi_1"}}`
	btDispute = `{"id":"txn_dispute01","object":"balance_transaction","amount":-25640,"fee":1500,"net":-27140,"currency":"hkd","exchange_rate":0.2564,"created":1790000300,"reporting_category":"dispute","type":"adjustment","source":{"id":"dp_1","object":"dispute","amount":100000,"currency":"twd","payment_intent":"pi_1","charge":"ch_1"}}`
	btPayout  = `{"id":"txn_payout01","object":"balance_transaction","amount":-500000,"fee":0,"net":-500000,"currency":"hkd","exchange_rate":null,"created":1790000400,"reporting_category":"payout","type":"payout","source":"po_1"}`
	btFeeTxn  = `{"id":"txn_stripefee01","object":"balance_transaction","amount":-100,"fee":0,"net":-100,"currency":"hkd","exchange_rate":null,"created":1790000500,"reporting_category":"fee","type":"stripe_fee","source":null}`
)

func listBody(hasMore bool, items ...string) string {
	return `{"object":"list","url":"/v1/balance_transactions","has_more":` + strconv.FormatBool(hasMore) + `,"data":[` + strings.Join(items, ",") + `]}`
}

func balanceClient(t *testing.T, reply func(*http.Request) (int, string)) (*Client, *[]*http.Request) {
	t.Helper()
	var seen []*http.Request
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r)
		status, body := reply(r)
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	c, err := NewWithMockTransport(sandboxConfig(), rt)
	if err != nil {
		t.Fatal(err)
	}
	return c, &seen
}

func TestBalanceProjectionIsExactAndSourceOnlyListedFields(t *testing.T) {
	c, seen := balanceClient(t, func(*http.Request) (int, string) {
		return 200, listBody(false, btCharge, btRefund, btDispute, btPayout, btFeeTxn)
	})
	got, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790600000)
	if err != nil || len(got) != 5 {
		t.Fatalf("list: %d %v", len(got), err)
	}
	r := (*seen)[0]
	q, _ := url.ParseQuery(r.URL.RawQuery)
	if r.Method != http.MethodGet || r.URL.Path != "/v1/balance_transactions" || q.Get("created[gte]") != "1790000000" ||
		q.Get("created[lt]") != "1790600000" || q.Get("limit") != "100" || q.Get("expand[]") != "data.source" {
		t.Fatalf("request: %s %s", r.Method, r.URL.String())
	}
	if r.Header.Get("Idempotency-Key") != "" {
		t.Fatal("a read must not carry an idempotency key")
	}
	ch := got[0]
	if ch.ID != "txn_charge01" || ch.Type != "charge" || *ch.ReportingCategory != "charge" || ch.Amount != 25640 || *ch.Fee != 1046 ||
		ch.Net != 24594 || ch.Currency != "HKD" || ch.ExchangeRate.String() != "0.2564" || ch.Created != 1790000100 ||
		*ch.SourceID != "ch_1" || *ch.SourceObject != "charge" || *ch.ChargePaymentIntent != "pi_1" || *ch.ChargeAmount != 100000 ||
		*ch.ChargeCurrency != "TWD" || ch.RefundID != nil || ch.DisputeID != nil {
		t.Fatalf("charge projection: %+v", ch)
	}
	if rf := got[1]; *rf.RefundID != "re_1" || *rf.RefundAmount != 10000 || *rf.RefundCurrency != "TWD" || rf.ChargePaymentIntent != nil {
		t.Fatalf("refund projection: %+v", rf)
	}
	if d := got[2]; d.Type != "adjustment" || *d.DisputeID != "dp_1" || *d.DisputePaymentIntent != "pi_1" || *d.DisputeAmount != 100000 ||
		*d.DisputeCurrency != "TWD" || *d.SourceObject != "dispute" {
		t.Fatalf("dispute projection: %+v", d)
	}
	if p := got[3]; p.Type != "payout" || *p.SourceID != "po_1" || *p.SourceObject != "payout" || p.ExchangeRate != nil {
		t.Fatalf("payout projection: %+v", p)
	}
	if f := got[4]; f.SourceID != nil || f.Type != "stripe_fee" {
		t.Fatalf("stripe_fee projection: %+v", f)
	}
	// The wire shape carries exactly the 21 keys (JSON null for a value Stripe did not send) and no buyer data.
	raw, _ := json.Marshal(ch)
	var keys map[string]any
	if err := json.Unmarshal(raw, &keys); err != nil || len(keys) != 21 {
		t.Fatalf("keys=%d %s", len(keys), raw)
	}
	if strings.Contains(string(raw), "buyer@example.test") || strings.Contains(string(raw), "4242") {
		t.Fatalf("projection leaks buyer data: %s", raw)
	}
}

func TestBalanceMissingFeeAndSourceStayNull(t *testing.T) {
	noFee := strings.Replace(btCharge, `"fee":1046,`, ``, 1)
	c, _ := balanceClient(t, func(*http.Request) (int, string) { return 200, listBody(false, noFee) })
	got, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790600000)
	if err != nil || got[0].Fee != nil {
		t.Fatalf("absent fee must stay nil (SQL fails the row closed): %+v %v", got, err)
	}
}

func TestBalancePagination(t *testing.T) {
	c, seen := balanceClient(t, func(r *http.Request) (int, string) {
		if r.URL.Query().Get("starting_after") == "" {
			return 200, listBody(true, btCharge)
		}
		return 200, listBody(false, btRefund)
	})
	got, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790600000)
	if err != nil || len(got) != 2 || len(*seen) != 2 || (*seen)[1].URL.Query().Get("starting_after") != "txn_charge01" {
		t.Fatalf("pages: %d txns, %d calls, %v", len(got), len(*seen), err)
	}
}

func TestBalanceFiftyFirstPageIsUncertainAndReturnsNothing(t *testing.T) {
	pages := 0
	c, _ := balanceClient(t, func(*http.Request) (int, string) {
		pages++
		return 200, listBody(true, strings.Replace(btCharge, "txn_charge01", "txn_p"+strconv.Itoa(pages), 1))
	})
	got, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790600000)
	if !errors.Is(err, ErrUncertain) || got != nil || pages != maxBalancePages {
		t.Fatalf("pages=%d got=%d err=%v", pages, len(got), err)
	}
}

func TestBalanceWindowBounds(t *testing.T) {
	c, seen := balanceClient(t, func(*http.Request) (int, string) { return 200, listBody(false) })
	for _, w := range [][2]int64{{0, 100}, {200, 200}, {300, 200}, {1790000000, 1790000000 + 8*86400 + 1}} {
		if _, _, err := c.ListBalanceTransactions(t.Context(), w[0], w[1]); !errors.Is(err, ErrInvalid) {
			t.Fatalf("window %v: %v", w, err)
		}
	}
	if len(*seen) != 0 {
		t.Fatal("an invalid window must not reach the network")
	}
	if _, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790000000+8*86400); err != nil {
		t.Fatalf("exactly 8 days is allowed: %v", err)
	}
}

func TestBalanceMalformedBodiesFailClosed(t *testing.T) {
	bad := map[string]string{
		"duplicate key":  strings.Replace(btCharge, `"amount":25640,`, `"amount":25640,"amount":1,`, 1),
		"string amount":  strings.Replace(btCharge, `"amount":25640,"fee"`, `"amount":"25640","fee"`, 1),
		"float amount":   strings.Replace(btCharge, `"amount":25640,"fee"`, `"amount":256.4,"fee"`, 1),
		"wrong object":   strings.Replace(btCharge, `"object":"balance_transaction"`, `"object":"charge"`, 1),
		"bad id":         strings.Replace(btCharge, `txn_charge01`, `ch_notatxn`, 1),
		"source number":  strings.Replace(btCharge, `"source":{"id":"ch_1"`, `"source":{"id":5,"x":"ch_1"`, 1),
		"currency upper": strings.Replace(btCharge, `"currency":"hkd"`, `"currency":"HKD"`, 1),
		"no created":     strings.Replace(btCharge, `"created":1790000100,`, ``, 1),
	}
	for name, item := range bad {
		c, _ := balanceClient(t, func(*http.Request) (int, string) { return 200, listBody(false, btRefund, item) })
		got, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790600000)
		if !errors.Is(err, ErrUncertain) || got != nil {
			t.Errorf("%s: err=%v got=%d (nothing may be returned)", name, err, len(got))
		}
	}
	for _, body := range []string{`{"object":"list","has_more":true,"data":[]}`, `{"object":"list","data":[]}`, `[]`, `not json`,
		`{"object":"list","has_more":false,"data":[],"data":[]}`} {
		c, _ := balanceClient(t, func(*http.Request) (int, string) { return 200, body })
		if _, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790600000); !errors.Is(err, ErrUncertain) {
			t.Errorf("body %q: %v", body, err)
		}
	}
}

func TestBalanceTransportAndStatusClassification(t *testing.T) {
	c, _ := balanceClient(t, func(*http.Request) (int, string) { return 401, `{"error":{"type":"authentication_error"}}` })
	if _, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790600000); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("401: %v", err)
	}
	c, _ = balanceClient(t, func(*http.Request) (int, string) { return 500, `{}` })
	if _, _, err := c.ListBalanceTransactions(t.Context(), 1790000000, 1790600000); !errors.Is(err, ErrUncertain) {
		t.Fatalf("500: %v", err)
	}
}
