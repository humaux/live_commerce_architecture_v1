package foundation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/buyerhttp"
	"livecommerce/internal/checkout"
	"livecommerce/internal/httperror"
)

type bphHarness struct {
	hpHarness
	http bhHarness
}

func bphSetup(t *testing.T) bphHarness {
	t.Helper()
	h := hpSetup(t)
	b := bhHarness{bcHarness: h.bcHarness, key: base64.RawURLEncoding.EncodeToString(randomBytes(32)), origin: "https://buyer-payment.example"}
	bhPublish(t, h.bcHarness, b.origin, h.f.tenantA, h.f.storeA1)
	handler, err := buyerhttp.New(context.Background(), h.a.issuer, h.a.runtime, h.service, b.key, time.Hour, h.api.(*checkout.HostedPaymentStarter))
	if err != nil {
		t.Fatal("BPH03 private payment handler construction failed")
	}
	b.server = httptest.NewServer(handler)
	t.Cleanup(b.server.Close)
	return bphHarness{hpHarness: h, http: b}
}

func bphPath(h bphHarness, suffix string) string {
	return "/v1/buyer/orders/" + h.hold.OrderID + "/payment" + suffix
}

func bphRaw(t *testing.T, response bhResponse, status int, want []string) map[string]json.RawMessage {
	t.Helper()
	if response.status != status {
		t.Fatalf("BPH03/04 HTTP status=%d want=%d", response.status, status)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(response.body, &fields); err != nil {
		t.Fatal("BPH03/04 invalid JSON response")
	}
	if want != nil && !reflect.DeepEqual(bpKeys(fields), want) {
		t.Fatal("BPH03/04 response included private or missing fields")
	}
	return fields
}

func bphFail(t *testing.T, response bhResponse, status int, code string) {
	t.Helper()
	envelope := bhRead[httperror.Envelope](t, response, status)
	if envelope.Code != code || envelope.Retryable || envelope.RequestID != response.header.Get("X-Request-ID") || envelope.Details == nil {
		t.Fatal("BPH04 handoff failure is retryable or unsafe")
	}
}

func bphRawBody(raw string) func(*http.Request) {
	return func(request *http.Request) {
		request.Body = io.NopCloser(strings.NewReader(raw))
		request.ContentLength = int64(len(raw))
		request.Header.Set("Content-Type", "application/json")
	}
}

func TestBuyerPaymentHTTPPrepareViewHandoffThreeLocales(t *testing.T) {
	for _, locale := range []string{"zh-CN", "zh-TW", "en"} {
		t.Run(locale, func(t *testing.T) { rm2AssertRemovedHTTP(t, "prepare", locale) })
	}
}

func TestBuyerPaymentHTTPNegativeAdmissionAndScope(t *testing.T) {
	h := bphSetup(t)
	path := bphPath(h, "")
	prepare := path + "/prepare"
	take := path + "/handoff"
	in := map[string]any{"method_code": "payuni_credit", "method_version": 1, "locale": "en"}
	before := h.counts(t)
	for _, trial := range []struct {
		name, method, path, token, key string
		input                          any
		edit                           func(*http.Request)
		status                         int
		code                           string
	}{
		{"bad-bff", "POST", take, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Set("X-Commerce-Buyer-BFF-Key", "bad") }, 401, "unauthorized"},
		{"duplicate-bff", "POST", take, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Add("X-Commerce-Buyer-BFF-Key", h.http.key) }, 401, "unauthorized"},
		{"missing-origin", "POST", take, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Del("X-Commerce-Storefront-Origin") }, 422, "invalid_request"},
		{"duplicate-origin", "POST", take, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Add("X-Commerce-Storefront-Origin", h.http.origin) }, 422, "invalid_request"},
		{"forbidden-cookie", "POST", take, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Set("Cookie", "synthetic=1") }, 403, "forbidden"},
		{"missing-token", "POST", take, "", "", nil, nil, 401, "unauthorized"},
		{"duplicate-token", "POST", take, h.cap.Token, "", nil, func(r *http.Request) { r.Header.Add("Authorization", "Bearer "+h.cap.Token) }, 401, "unauthorized"},
		{"handoff-key", "POST", take, h.cap.Token, t04Key("bph-forbidden-key"), nil, nil, 422, "invalid_request"},
		{"handoff-body", "POST", take, h.cap.Token, "", struct{}{}, nil, 422, "invalid_request"},
		{"handoff-query", "POST", take + "?x=1", h.cap.Token, "", nil, nil, 403, "forbidden"},
		{"bad-path", "POST", "/v1/buyer/orders/not-a-uuid/payment/handoff", h.cap.Token, "", nil, nil, 422, "invalid_request"},
		{"prepare-key-missing", "POST", prepare, h.cap.Token, "", in, nil, 422, "invalid_request"},
		{"prepare-unknown", "POST", prepare, h.cap.Token, t04Key("bph-unknown"), map[string]any{"method_code": "payuni_credit", "method_version": 1, "locale": "en", "provider": "payuni"}, nil, 400, "invalid_json"},
		{"prepare-null", "POST", prepare, h.cap.Token, t04Key("bph-null"), nil, bphRawBody("null"), 400, "invalid_json"},
		{"prepare-nested", "POST", prepare, h.cap.Token, t04Key("bph-nested"), nil, bphRawBody(`{"method_code":{"value":"payuni_credit"},"method_version":1,"locale":"en"}`), 400, "invalid_json"},
		{"prepare-locale", "POST", prepare, h.cap.Token, t04Key("bph-locale"), map[string]any{"method_code": "payuni_credit", "method_version": 1, "locale": "fr"}, nil, 422, "invalid_request"},
		{"prepare-method", "POST", prepare, h.cap.Token, t04Key("bph-method"), map[string]any{"method_code": "payuni_atm", "method_version": 1, "locale": "en"}, nil, 422, "invalid_request"},
		{"view-body", "GET", path, h.cap.Token, "", struct{}{}, nil, 422, "invalid_request"},
		{"view-key", "GET", path, h.cap.Token, t04Key("bph-view-key"), nil, nil, 422, "invalid_request"},
		{"view-query", "GET", path + "?x=1", h.cap.Token, "", nil, nil, 403, "forbidden"},
	} {
		t.Run(trial.name, func(t *testing.T) {
			response := h.http.request(t, trial.method, trial.path, trial.token, trial.key, trial.input, trial.edit)
			if strings.Contains(trial.path, "/handoff") {
				bphFail(t, response, trial.status, trial.code)
			} else {
				bhError(t, response, trial.status, trial.code)
			}
		})
	}
	other := mustIssue(t, h.cqHarness.service, h.f.storeA1)
	bphFail(t, h.http.request(t, "POST", take, other.Token, "", nil, nil), 409, "conflict")
	bhError(t, h.http.request(t, "GET", path, other.Token, "", nil, nil), 404, "not_found")
	bhError(t, h.http.request(t, "GET", "/v1/buyer/orders/"+randomUUID()+"/payment", h.cap.Token, "", nil, nil), 404, "not_found")
	if h.counts(t) != before {
		t.Fatal("BPH04 denied requests created payment/stock/job facts")
	}
	mustExec(t, h.f.owner, `UPDATE buyer.capability_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, h.cap.Scope.SessionID)
	bphFail(t, h.http.request(t, "POST", take, h.cap.Token, "", nil, nil), 401, "unauthorized")
}

func bphConcurrentTake(h bphHarness) (bhResponse, error) {
	r, err := http.NewRequest(http.MethodPost, h.http.server.URL+bphPath(h, "/handoff"), nil)
	if err != nil {
		return bhResponse{}, err
	}
	r.Header.Set("X-Commerce-Buyer-BFF-Key", h.http.key)
	r.Header.Set("X-Commerce-Storefront-Origin", h.http.origin)
	r.Header.Set("Authorization", "Bearer "+h.cap.Token)
	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(r)
	if err != nil {
		return bhResponse{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return bhResponse{status: response.StatusCode, header: response.Header, body: body}, err
}

func TestBuyerPaymentHTTPConcurrentHandoffAndEarlyFailures(t *testing.T) {
	rm2AssertRemovedHTTP(t, "handoff", "en")
}

func TestBuyerPaymentHTTPDisabledFeatureNoRelease(t *testing.T) {
	h := bhSetup(t)
	order, err := h.begin(t04Key("bph-disabled"))
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/buyer/orders/" + order.OrderID + "/payment"
	for _, suffix := range []string{"", "/prepare", "/handoff"} {
		method := "POST"
		var input any
		key := ""
		if suffix == "" {
			method = "GET"
		} else if suffix == "/prepare" {
			input = map[string]any{"method_code": "payuni_credit", "method_version": 1, "locale": "en"}
			key = t04Key("bph-disabled")
		}
		response := h.request(t, method, path+suffix, h.cap.Token, key, input, nil)
		if suffix == "/handoff" {
			bphFail(t, response, 404, "not_found")
		} else {
			bhError(t, response, 404, "not_found")
		}
	}
}
