package foundation_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
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
	for locale, lang := range map[string]string{"zh-CN": "zh-tw", "zh-TW": "zh-tw", "en": "en"} {
		t.Run(locale, func(t *testing.T) {
			h := bphSetup(t)
			path := bphPath(h, "")
			before, beforeOther := bpCounts(t, h.hpHarness)
			initial := h.http.request(t, "GET", path, h.cap.Token, "", nil, nil)
			initialRaw := bphRaw(t, initial, 200, bpViewKeys)
			bpState(t, initialRaw, "payment_state", "NOT_STARTED")
			bpState(t, initialRaw, "handoff_state", "NONE")
			var options []map[string]json.RawMessage
			if err := json.Unmarshal(initialRaw["methods"], &options); err != nil || len(options) != 1 || !reflect.DeepEqual(bpKeys(options[0]), bpMethodKeys) {
				t.Fatal("BPH03 current method was not projected with exact safe fields")
			}
			in := map[string]any{"method_code": "payuni_credit", "method_version": 1, "locale": locale}
			key := t04Key("bph-prepare")
			preparedHTTP := h.http.request(t, "POST", path+"/prepare", h.cap.Token, key, in, nil)
			prepared := bphRaw(t, preparedHTTP, 200, []string{"amount_minor", "currency", "order_id", "state"})
			bpState(t, prepared, "state", "PAYMENT_PENDING")
			var amount int64
			if err := json.Unmarshal(prepared["amount_minor"], &amount); err != nil || amount != 2500 {
				t.Fatal("BPH03 prepare did not use the frozen original amount")
			}
			stable := h.counts(t)
			replayed := h.http.request(t, "POST", path+"/prepare", h.cap.Token, key, in, nil)
			if replayed.status != 200 || !bytes.Equal(preparedHTTP.body, replayed.body) || h.counts(t) != stable {
				t.Fatal("BPH03 HTTP prepare replay changed receipt or payment facts")
			}
			pending := bphRaw(t, h.http.request(t, "GET", path, h.cap.Token, "", nil, nil), 200, bpViewKeys)
			bpState(t, pending, "payment_state", "PENDING")
			bpState(t, pending, "handoff_state", "PREPARED")
			if string(pending["methods"]) != "[]" {
				t.Fatal("BPH03 pending payment offered another method")
			}
			first := h.http.request(t, "POST", path+"/handoff", h.cap.Token, "", nil, nil)
			issued := bphRaw(t, first, 200, []string{"disposition", "expires_at", "form", "order_id"})
			bpState(t, issued, "disposition", "ISSUED")
			var form struct {
				Action string            `json:"action"`
				Fields map[string]string `json:"fields"`
			}
			if err := json.Unmarshal(issued["form"], &form); err != nil || len(form.Fields) != 4 {
				t.Fatal("BPH03 issued handoff lacks bounded scalar form")
			}
			var attempt checkout.PaymentResult
			if err := h.f.owner.QueryRow(context.Background(), `SELECT merchant_trade_no,amount_minor,id::text FROM checkout.payment_attempts WHERE order_id=$1`, h.hold.OrderID).
				Scan(&attempt.MerchantTradeNo, &attempt.AmountMinor, &attempt.AttemptID); err != nil {
				t.Fatal(err)
			}
			_, created, _, _ := h.page(t, attempt.AttemptID)
			hpVerifyWire(t, h.hpHarness, attempt, created, form.Action, form.Fields, lang)
			second := h.http.request(t, "POST", path+"/handoff", h.cap.Token, "", nil, nil)
			noForm := bphRaw(t, second, 200, []string{"disposition", "expires_at", "order_id"})
			bpState(t, noForm, "disposition", "ALREADY_ISSUED")
			latest := bphRaw(t, h.http.request(t, "GET", path, h.cap.Token, "", nil, nil), 200, bpViewKeys)
			bpState(t, latest, "payment_state", "PENDING")
			bpState(t, latest, "handoff_state", "ISSUED")
			if h.counts(t) != stable {
				t.Fatal("BPH03 GET/take/repeat changed payment/stock/job aggregate")
			}
			_, afterOther := bpCounts(t, h.hpHarness)
			if before == stable || beforeOther != afterOther {
				t.Fatal("BPH03 prepare did not create original payment facts or touched other facts")
			}
		})
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
	h := bphSetup(t)
	path := bphPath(h, "")
	in := map[string]any{"method_code": "payuni_credit", "method_version": 1, "locale": "en"}
	if response := h.http.request(t, "POST", path+"/prepare", h.cap.Token, t04Key("bph-concurrent"), in, nil); response.status != 200 {
		t.Fatal("BPH04 concurrent case did not prepare")
	}
	var wg sync.WaitGroup
	results := make(chan bhResponse, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := bphConcurrentTake(h)
			results <- result
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	issued, replay := 0, 0
	for result := range results {
		if result.status != 200 {
			t.Fatal("BPH04 concurrent handoff failed")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(result.body, &fields); err != nil {
			t.Fatal(err)
		}
		var disposition string
		if err := json.Unmarshal(fields["disposition"], &disposition); err != nil {
			t.Fatal(err)
		}
		switch disposition {
		case "ISSUED":
			issued++
			if string(fields["form"]) == "" || string(fields["form"]) == "null" {
				t.Fatal("BPH04 issued handoff lacked form")
			}
		case "ALREADY_ISSUED":
			replay++
			if _, found := fields["form"]; found {
				t.Fatal("BPH04 repeated handoff exposed form")
			}
		default:
			t.Fatal("BPH04 unknown concurrent handoff disposition")
		}
	}
	if issued != 1 || replay != 1 {
		t.Fatal("BPH04 concurrent HTTP took more than one form")
	}
	// Service failure occurs before any hosted SQL. A handoff 503 must remain
	// nonretryable because the client cannot know whether another path committed.
	h.a.issuer.Close()
	bphFail(t, h.http.request(t, "POST", path+"/handoff", h.cap.Token, "", nil, nil), 503, "unavailable")
	_, _, _, handed := h.page(t, func() string {
		var id string
		if err := h.f.owner.QueryRow(context.Background(), `SELECT id::text FROM checkout.payment_attempts WHERE order_id=$1`, h.hold.OrderID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}())
	if handed == nil {
		t.Fatal("BPH04 early 503 erased committed one-shot marker")
	}
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
