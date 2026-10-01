package httpapi

// design_test.go: transport guards of the store-design admin routes that run before any SQL (a nil pool is never
// reached): login required on every route, JSON content type, unknown/trailing JSON, the body cap, the multipart upload
// shape, and the 422 envelope carrying the offending path. Real-PG behaviour: tests/foundation/store_design_test.go.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/design"
	"livecommerce/internal/httperror"
)

const designBase = "/v1/admin/stores/11111111-1111-4111-8111-111111111111/design"
const designImage = "33333333-3333-4333-8333-333333333333"

func TestDesignRoutesRequireLogin(t *testing.T) {
	h := NewHandler(nil)
	for _, tt := range []struct{ method, path string }{
		{"GET", designBase + "/draft"}, {"PUT", designBase + "/draft"}, {"POST", designBase + "/publish"}, {"POST", designBase + "/rollback"},
		{"POST", designBase + "/preview-token"}, {"GET", designBase + "/versions"}, {"GET", designBase + "/media"}, {"POST", designBase + "/media"},
		{"GET", designBase + "/media/" + designImage}, {"POST", designBase + "/media/" + designImage + "/delete"},
	} {
		r := httptest.NewRequest(tt.method, tt.path, strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Errorf("%s %s: status %d", tt.method, tt.path, w.Code)
		}
	}
}

func TestDesignBodyGuards(t *testing.T) {
	h := NewHandler(nil)
	auth := "Bearer " + strings.Repeat("a", 43)
	for _, tt := range []struct {
		name, ctype, body string
		want              int
	}{
		{"not json", "text/plain", "{}", 415},
		{"unknown envelope field", "application/json", `{"expected_version":1,"document":{},"tenant_id":"x"}`, 400},
		{"trailing value", "application/json", `{"expected_version":1,"document":{}} {}`, 400},
		{"malformed", "application/json", `{`, 400},
		{"over the cap", "application/json", `{"expected_version":1,"document":"` + strings.Repeat("a", maxDesignBody) + `"}`, 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, designBase+"/draft", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.ctype)
			r.Header.Set("Authorization", auth)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestDesignUploadGuards(t *testing.T) {
	h := NewHandler(nil)
	auth := "Bearer " + strings.Repeat("a", 43)
	for _, tt := range []struct {
		name  string
		build func() (*bytes.Buffer, string)
		want  int
	}{
		{"json is not multipart", func() (*bytes.Buffer, string) { return bytes.NewBufferString("{}"), "application/json" }, 415},
		{"wrong field name", func() (*bytes.Buffer, string) { return multipartBody(t, map[string][]byte{"x": []byte("y")}, "x") }, 422},
		{"over the cap", func() (*bytes.Buffer, string) {
			return multipartBody(t, map[string][]byte{"file": make([]byte, maxUploadBody+1)}, "file")
		}, 413},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body, ctype := tt.build()
			r := httptest.NewRequest(http.MethodPost, designBase+"/media", body)
			r.Header.Set("Content-Type", ctype)
			r.Header.Set("Authorization", auth)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestValidationEnvelopeCarriesPathOnly(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("X-Request-ID", "req-1")
	writeValidation(w, &design.ValidationError{Path: "home.sections[2].heading", Reason: "too long (max 80 characters)"})
	var env httperror.Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if w.Code != 422 || env.Code != "invalid_request" || env.RequestID != "req-1" || env.Retryable ||
		env.Details["path"] != "home.sections[2].heading" || len(env.Details) != 2 || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("envelope %d %+v", w.Code, env)
	}
}
