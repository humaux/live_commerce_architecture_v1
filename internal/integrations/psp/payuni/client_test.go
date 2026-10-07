package payuni

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	testKey = "12345678901234567890123456789012"
	testIV  = "1234567890123456"
	// PAYUNi's published Node.js example, page 312.
	goldenPlain   = "MerID=AAA&MerTradeNO=BBB&Prod=%E5%95%86%E5%93%81%E8%AA%AA%E6%98%8E"
	goldenEncrypt = "47396636346f66735853533167396942344f587a3775696b34732b596e70452b675270564f73536b7753446c6a4d77526d4e374256514173672b6c78616d4533504d475152642b362f4530626f446e4f6356533969756c743a3a3a4b5961342f4635456965743069385a784b6277704a413d3d"
	goldenHash    = "E97180D78C8378D64A188D292938B9D2717034F292B626019B01DF160AEFC0B7"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	c, err := New(Config{Environment: "SANDBOX", MerchantID: "AAA", HashKey: testKey, HashIV: testIV,
		ReturnURL: "https://shop.example.com/pay/return", NotifyURL: "https://shop.example.com/pay/notify"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func testExpected(method string) ExpectedTrade {
	return ExpectedTrade{MerTradeNo: "ORDER_1", AmountTWD: 100, Currency: "TWD", Method: method}
}

func notification(t *testing.T, c *Client, inner url.Values, status string) []byte {
	t.Helper()
	encryptInfo, hashInfo, err := c.seal(inner.Encode())
	if err != nil {
		t.Fatal(err)
	}
	return []byte(url.Values{"Status": {status}, "MerID": {"AAA"}, "Version": {"2.0"},
		"EncryptInfo": {encryptInfo}, "HashInfo": {hashInfo}}.Encode())
}

func baseObservation() url.Values {
	return url.Values{"Status": {"SUCCESS"}, "MerID": {"AAA"}, "MerTradeNo": {"ORDER_1"},
		"TradeNo": {"P123"}, "TradeAmt": {"100"}, "TradeStatus": {"1"},
		"PaymentType": {"1"}, "Gateway": {"2"}, "AuthType": {"1"}}
}

func TestOfficialNodeVector(t *testing.T) {
	c := testClient(t)
	encryptInfo, hashInfo, err := c.seal(goldenPlain)
	if err != nil || encryptInfo != goldenEncrypt || hashInfo != goldenHash {
		t.Fatalf("official vector mismatch: err=%v encrypt=%t hash=%t", err, encryptInfo == goldenEncrypt, hashInfo == goldenHash)
	}
	plain, err := c.open(goldenEncrypt, goldenHash)
	if err != nil || plain != goldenPlain {
		t.Fatalf("official vector decrypt mismatch: err=%v", err)
	}
	if _, err := c.open(goldenEncrypt, strings.Repeat("0", 64)); !errors.Is(err, ErrAuthentication) {
		t.Fatalf("bad hash accepted: %v", err)
	}
}

func TestConfigRedactionAndValidation(t *testing.T) {
	c := testClient(t)
	for _, value := range []any{c.config, c} {
		encoded, err := json.Marshal(value)
		if err != nil || strings.Contains(fmt.Sprint(value)+fmt.Sprintf("%#v", value)+string(encoded), testKey) ||
			strings.Contains(fmt.Sprint(value)+fmt.Sprintf("%#v", value)+string(encoded), testIV) {
			t.Fatalf("secret leaked by formatting %T: %v", value, err)
		}
	}
	for _, callback := range []string{"http://shop.example.com/x", "https://127.0.0.1/x",
		"https://localhost/x", "https://shop.example.com:8443/x", "https://user@shop.example.com/x",
		"https://shop.example.com/x#fragment"} {
		config := c.config
		config.NotifyURL = callback
		if _, err := New(config); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted callback %q: %v", callback, err)
		}
	}
	for _, secret := range []string{" " + testKey[1:], testKey[:31] + " "} {
		config := c.config
		config.HashKey = secret
		if _, err := New(config); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted edge-space secret: %v", err)
		}
	}
}

func TestQueryOnlyClientCannotUsePaymentOrNotification(t *testing.T) {
	config := Config{Environment: "SANDBOX", MerchantID: "AAA", HashKey: testKey, HashIV: testIV}
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected provider request")
		return nil, nil
	})
	client, err := NewQuery(config, transport)
	if err != nil || !client.queryOnly {
		t.Fatalf("query-only constructor: %v", err)
	}
	defaultClient, err := NewQuery(config)
	if err != nil {
		t.Fatal(err)
	}
	if built, ok := defaultClient.httpClient.Transport.(*http.Transport); !ok || !built.DisableKeepAlives {
		t.Fatal("one-shot query retained idle connection")
	}
	if built, ok := testClient(t).httpClient.Transport.(*http.Transport); !ok || built.DisableKeepAlives {
		t.Fatal("hosted client transport changed")
	}
	if _, err := client.BuildHosted(HostedRequest{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("query client built hosted payment: %v", err)
	}
	if _, err := client.VerifyNotification(nil, testExpected("payuni_credit")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("query client verified notification: %v", err)
	}
	config.NotifyURL = "https://shop.example.com/notify"
	if _, err := NewQuery(config); !errors.Is(err, ErrInvalid) {
		t.Fatalf("query-only client accepted callback URL: %v", err)
	}
	config.NotifyURL = ""
	config.Environment = "LIVE"
	if _, err := NewQuery(config, transport); !errors.Is(err, ErrInvalid) {
		t.Fatalf("live query client accepted mock transport: %v", err)
	}
	if _, err := NewQuery(config); err != nil {
		t.Fatalf("live query with fixed transport rejected: %v", err)
	}
	if _, err := New(config); !errors.Is(err, ErrInvalid) {
		t.Fatalf("hosted constructor accepted empty callback URLs: %v", err)
	}
}

func TestQueryOnlyClientUsesSignedWire(t *testing.T) {
	config := Config{Environment: "SANDBOX", MerchantID: "AAA", HashKey: testKey, HashIV: testIV}
	var client *Client
	calls := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://sandbox-api.payuni.com.tw/api/trade/query" || r.Method != http.MethodPost {
			t.Fatal("query client used wrong endpoint")
		}
		row := baseObservation()
		row.Del("Status")
		row.Set("DataSource", "A")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(queryEnvelope(t, client, row))),
			Header: make(http.Header)}, nil
	})
	var err error
	client, err = NewQuery(config, transport)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Query(context.Background(), testExpected("payuni_credit"), 1760000000)
	if err != nil || calls != 1 || got.AmountTWD != 100 || got.DataSource != "A" {
		t.Fatalf("signed query failed: %+v, calls=%d, err=%v", got, calls, err)
	}
}

func TestBuildHostedBindsOnePaymentSelector(t *testing.T) {
	c := testClient(t)
	timestamp := int64(1760000000)
	for _, tc := range []struct {
		method, selector, expiry string
		installments             []int
	}{
		{"payuni_credit", "Credit", "", nil},
		{"payuni_installment", "CreditInst", "", []int{3, 6}},
		{"payuni_atm", "ATM", "2025-10-10", nil},
		{"payuni_cvs", "CVS", "2025-10-10", nil},
		{"payuni_linepay", "LinePay", "", nil},
	} {
		req := HostedRequest{MerTradeNo: "ORDER_1", AmountTWD: 100, Timestamp: timestamp,
			Description: "商品", Method: tc.method, Installments: tc.installments, ExpireDate: tc.expiry,
			PageExpirySeconds: 600, Language: "zh-tw"}
		form, err := c.BuildHosted(req)
		if err != nil {
			t.Fatalf("%s: %v", tc.method, err)
		}
		if form.Action != "https://sandbox-api.payuni.com.tw/api/upp" || len(form.Fields) != 4 {
			t.Fatalf("bad hosted envelope %s", tc.method)
		}
		plain, err := c.open(form.Fields.Get("EncryptInfo"), form.Fields.Get("HashInfo"))
		if err != nil {
			t.Fatal(err)
		}
		inner, err := url.ParseQuery(plain)
		if err != nil || inner.Get(tc.selector) == "" || inner.Get("MerTradeNo") != "ORDER_1" {
			t.Fatalf("bad hosted contents %s: %v", tc.method, err)
		}
		selectors := 0
		for _, key := range []string{"Credit", "CreditInst", "ATM", "CVS", "LinePay"} {
			if inner.Get(key) != "" {
				selectors++
			}
		}
		if selectors != 1 {
			t.Fatalf("selector count %d for %s", selectors, tc.method)
		}
	}
	bad := HostedRequest{MerTradeNo: "ORDER_1", AmountTWD: 100, Timestamp: timestamp,
		Description: "x", Method: "payuni_installment", Installments: []int{6, 3},
		PageExpirySeconds: 600, Language: "en"}
	if _, err := c.BuildHosted(bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unordered installments accepted: %v", err)
	}
	bad.Method, bad.Installments, bad.AmountTWD = "payuni_cvs", nil, 29
	bad.ExpireDate = "2025-10-10"
	if _, err := c.BuildHosted(bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("below-minimum CVS amount accepted: %v", err)
	}
}

func TestNotificationAuthenticationAndBinding(t *testing.T) {
	c := testClient(t)
	inner := baseObservation()
	got, err := c.VerifyNotification(notification(t, c, inner, "SUCCESS"), testExpected("payuni_credit"))
	if err != nil || got.TradeNo != "P123" || got.TradeStatus != "1" {
		t.Fatalf("valid notification: %+v %v", got, err)
	}
	for _, tc := range []struct {
		name string
		body []byte
		want error
	}{
		{"duplicate decoded key", append(notification(t, c, inner, "SUCCESS"), []byte("&Mer%49D=AAA")...), ErrProtocol},
		{"unsigned error", []byte("Status=ERROR&MerID=AAA&Version=2.0"), ErrUncertain},
		{"bad escape", []byte("MerID=%ZZ"), ErrProtocol},
	} {
		if _, err := c.VerifyNotification(tc.body, testExpected("payuni_credit")); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	wrong := baseObservation()
	wrong.Set("TradeAmt", "101")
	if _, err := c.VerifyNotification(notification(t, c, wrong, "SUCCESS"), testExpected("payuni_credit")); !errors.Is(err, ErrMismatch) {
		t.Fatalf("wrong amount accepted: %v", err)
	}
	unknown := baseObservation()
	unknown.Set("Status", "UNKNOWN")
	unknown.Set("TradeStatus", "0")
	got, err = c.VerifyNotification(notification(t, c, unknown, "UNKNOWN"), testExpected("payuni_credit"))
	if err != nil || got.Status != "UNKNOWN" || got.TradeStatus != "0" {
		t.Fatalf("UNKNOWN observation lost: %+v %v", got, err)
	}
	pending := baseObservation()
	pending.Set("Status", "UNAPPROVED")
	pending.Set("TradeStatus", "8")
	got, err = c.VerifyNotification(notification(t, c, pending, "Unapproved"), testExpected("payuni_credit"))
	if err != nil || got.Status != "UNAPPROVED" || got.TradeStatus != "8" {
		t.Fatalf("UNAPPROVED observation lost: %+v %v", got, err)
	}
	if _, err := c.VerifyNotification(notification(t, c, pending, "SUCCESS"), testExpected("payuni_credit")); !errors.Is(err, ErrUncertain) {
		t.Fatalf("outer status mismatch accepted: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func queryEnvelope(t *testing.T, c *Client, row url.Values) string {
	t.Helper()
	inner := url.Values{"Status": {"SUCCESS"}}
	for key, values := range row {
		inner.Set("Result[0]["+key+"]", values[0])
	}
	encryptInfo, hashInfo, err := c.seal(inner.Encode())
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := json.Marshal(map[string]string{"Status": "SUCCESS", "MerID": "AAA", "Version": "2.0",
		"EncryptInfo": encryptInfo, "HashInfo": hashInfo})
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

func TestQueryOneBoundedObservation(t *testing.T) {
	c := testClient(t)
	row := baseObservation()
	row.Del("Status")
	row.Set("DataSource", "B")
	body := queryEnvelope(t, c, row)
	calls := 0
	c.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://sandbox-api.payuni.com.tw/api/trade/query" || r.Method != http.MethodPost ||
			r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.Header.Get("User-Agent") != "payuni" {
			t.Error("unexpected query request")
		}
		wire, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err := url.ParseQuery(string(wire))
		if err != nil {
			t.Fatal(err)
		}
		plain, err := c.open(form.Get("EncryptInfo"), form.Get("HashInfo"))
		if err != nil {
			t.Fatal(err)
		}
		queryFields, err := url.ParseQuery(plain)
		if err != nil || queryFields.Has("TradeNo") || queryFields.Get("MerTradeNo") != "ORDER_1" {
			t.Error("wrong lookup key")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	got, err := c.Query(context.Background(), testExpected("payuni_credit"), 1760000000)
	if err != nil || calls != 1 || got.DataSource != "B" || got.TradeStatus != "1" {
		t.Fatalf("query observation: %+v %v calls=%d", got, err, calls)
	}
	row.Set("DataSource", "X")
	body = queryEnvelope(t, c, row)
	if _, err := c.Query(context.Background(), testExpected("payuni_credit"), 1760000000); !errors.Is(err, ErrUncertain) {
		t.Fatalf("unknown source accepted: %v", err)
	}
}

func TestQueryRejectsMultirowAndUntrustedEnvelope(t *testing.T) {
	c := testClient(t)
	row := baseObservation()
	row.Del("Status")
	row.Set("DataSource", "A")
	good := queryEnvelope(t, c, row)
	badJSON := strings.Replace(good, `"Status":"SUCCESS"`, `"Status":"SUCCESS","Status":"SUCCESS"`, 1)
	for _, body := range []string{badJSON, `{"Status":{},"MerID":"AAA"}`} {
		c.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		if _, err := c.Query(context.Background(), testExpected("payuni_credit"), 1760000000); !errors.Is(err, ErrProtocol) {
			t.Fatalf("bad JSON accepted: %v", err)
		}
	}
	inner := url.Values{"Status": {"SUCCESS"}, "Result[1][TradeNo]": {"OTHER"}}
	for key, values := range row {
		inner.Set("Result[0]["+key+"]", values[0])
	}
	encryptInfo, hashInfo, err := c.seal(inner.Encode())
	if err != nil {
		t.Fatal(err)
	}
	multi, _ := json.Marshal(map[string]string{"Status": "SUCCESS", "MerID": "AAA", "Version": "2.0",
		"EncryptInfo": encryptInfo, "HashInfo": hashInfo})
	c.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(multi))), Header: make(http.Header)}, nil
	})
	if _, err := c.Query(context.Background(), testExpected("payuni_credit"), 1760000000); !errors.Is(err, ErrUncertain) {
		t.Fatalf("multirow accepted: %v", err)
	}
}
