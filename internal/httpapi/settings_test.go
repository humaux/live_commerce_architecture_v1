package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingsRoutesRejectQueriesBeforeAuthentication(t *testing.T) {
	h := NewHandler(nil)
	base := "/v1/admin/stores/11111111-1111-4111-8111-111111111111/markets/22222222-2222-4222-8222-222222222222/countries/TW"
	for _, path := range []string{
		base + "/delivery-services/manual?",
		base + "/delivery-services/manual?x=1",
		base + "/payment-methods/payuni?x=1",
		base + "/payment-methods/payuni/inspect?x=1",
	} {
		r := httptest.NewRequest("GET", path, nil)
		if strings.Contains(path, "/inspect") {
			r = httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 422 {
			t.Errorf("%s: status %d, body %s", path, w.Code, w.Body.String())
		}
	}
}

func TestSettingsBodiesDoNotAcceptProviderCredentials(t *testing.T) {
	h := NewHandler(nil)
	base := "/v1/admin/stores/11111111-1111-4111-8111-111111111111/markets/22222222-2222-4222-8222-222222222222/countries/TW"
	for _, route := range []string{
		base + "/delivery-services/manual",
		base + "/payment-methods/payuni",
		base + "/payment-methods/payuni/inspect",
	} {
		method := "PUT"
		body := `{"market_id":"22222222-2222-4222-8222-222222222222","country":"TW","code":"` + route[strings.LastIndex(route, "/")+1:] + `","access_token":"secret"}`
		if strings.HasSuffix(route, "/inspect") {
			method = "POST"
			body = `{"market_id":"22222222-2222-4222-8222-222222222222","country":"TW","code":"payuni","access_token":"secret"}`
		}
		r := httptest.NewRequest(method, route, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 || strings.Contains(w.Body.String(), "secret") {
			t.Errorf("%s: status %d, body %s", route, w.Code, w.Body.String())
		}
	}
}

func TestSettingsPathTargetMustMatchExactly(t *testing.T) {
	r := httptest.NewRequest("PUT", "/", nil)
	r.SetPathValue("market_id", "market-1")
	r.SetPathValue("country", "TW")
	r.SetPathValue("code", "payuni")
	for _, target := range [][3]string{
		{"market-1", "TW", "payuni"},
		{"market-2", "TW", "payuni"},
		{"market-1", "US", "payuni"},
		{"market-1", "TW", "other"},
		{"", "TW", "payuni"},
	} {
		want := target[0] == "market-1" && target[1] == "TW" && target[2] == "payuni"
		if got := matchesSettingsTarget(r, target[0], target[1], target[2]); got != want {
			t.Errorf("target %q: got %v, want %v", target, got, want)
		}
	}
}
