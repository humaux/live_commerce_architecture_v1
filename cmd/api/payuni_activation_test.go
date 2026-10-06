// Purpose: MOCK-tier tests of the API's PAYUNi activation wiring (w4-02b): the LC_PAYUNI_ENABLED switch, the env-gated service, and
//   proof that the built service is what main hands to httpapi (routes answer 401, not 404, through the real handler).
// Depends on: cmd/api payuni_activation.go, internal/httpapi, internal/payments.
// Used by: go test ./cmd/api.
// Status: MOCK.

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/payments"
)

func payuniActivationEnv() map[string]string {
	env := payuniNotifyTestEnv()
	return map[string]string{
		"LC_PAYUNI_ENABLED": "0", "LC_PAYUNI_NOTIFY_BASE_URL": "https://hooks.example.com",
		"LC_PAYUNI_VERIFY_RETURN_URL": "https://admin.example.com/settings/payments", "COMMERCE_PAYMENT_PROFILE": "PROVIDER_MOCK",
		"COMMERCE_ACCOUNT_ACTIVE_KEY_ID": env["COMMERCE_ACCOUNT_ACTIVE_KEY_ID"], "COMMERCE_ACCOUNT_KEYS_JSON": env["COMMERCE_ACCOUNT_KEYS_JSON"],
		"COMMERCE_ACCOUNT_REPLAY_KEY": env["COMMERCE_ACCOUNT_REPLAY_KEY"],
	}
}

func TestPayuniActivationSwitchDefaultsOffAndSurfaceIsEnvGated(t *testing.T) {
	t.Cleanup(func() { payments.SetPayuniEnabled(false) })
	payments.SetPayuniEnabled(true)
	read := []string{}
	svc, err := loadPayuniActivation(func(name string) string { read = append(read, name); return "" })
	if err != nil || svc != nil || payments.PayuniEnabled() {
		t.Fatalf("unset env: svc=%v err=%v enabled=%v, want nil/nil/off", svc, err, payments.PayuniEnabled())
	}
	if strings.Join(read, ",") != "LC_PAYUNI_ENABLED,LC_PAYUNI_NOTIFY_BASE_URL" {
		t.Fatalf("surface off read %v, want only the switch and the base url", read)
	}
	env := payuniActivationEnv()
	env["LC_PAYUNI_ENABLED"] = "1"
	if svc, err = loadPayuniActivation(func(n string) string { return env[n] }); err != nil || svc == nil || !payments.PayuniEnabled() {
		t.Fatalf("configured: svc=%v err=%v enabled=%v", svc, err, payments.PayuniEnabled())
	}
	for name, mutate := range map[string]func(){
		"noncanonical switch": func() { env["LC_PAYUNI_ENABLED"] = "true" },
		"unknown profile":     func() { env["COMMERCE_PAYMENT_PROFILE"] = "STAGING" },
		"http return url":     func() { env["LC_PAYUNI_VERIFY_RETURN_URL"] = "http://admin.example.com/x" },
		"http notify base":    func() { env["LC_PAYUNI_NOTIFY_BASE_URL"] = "http://hooks.example.com" },
		"no keyring":          func() { env["COMMERCE_ACCOUNT_KEYS_JSON"] = "" },
	} {
		env = payuniActivationEnv()
		mutate()
		if _, err := loadPayuniActivation(func(n string) string { return env[n] }); !errors.Is(err, errPayuniActivationConfig) {
			t.Errorf("%s accepted: %v", name, err)
		}
	}
}

// The wiring proof: the service loadPayuniActivation builds, passed as main does, mounts the activation routes in the real handler.
func TestPayuniActivationServiceReachesTheHandlerAndMain(t *testing.T) {
	t.Cleanup(func() { payments.SetPayuniEnabled(false) })
	env := payuniActivationEnv()
	svc, err := loadPayuniActivation(func(n string) string { return env[n] })
	if err != nil || svc == nil {
		t.Fatal(err)
	}
	h := httpapi.NewHandler(nil, httpapi.Options{PayuniActivation: svc})
	r := httptest.NewRequest(http.MethodPost, "/v1/admin/stores/11111111-1111-4111-8111-111111111111/payments/payuni/live-probe", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("activation route status=%d, want 401 (mounted)", w.Code)
	}
	src, err := os.ReadFile("main.go")
	if err != nil || !strings.Contains(string(src), "PayuniActivation: payuniActivation") || !strings.Contains(string(src), "loadPayuniActivation(os.Getenv)") {
		t.Fatal("main.go no longer builds and passes the PAYUNi activation service")
	}
}
