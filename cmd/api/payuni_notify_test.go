// payuni_notify_test.go: MOCK-tier unit tests for the API's PAYUNi notify config, redaction and
// mount seam. Non-goal: pool admission and SQL (REAL_PG foundation gates, NOT_RUN here).
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func payuniNotifyTestEnv() map[string]string {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))
	replay := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32))
	return map[string]string{
		"COMMERCE_PAYUNI_NOTIFY_ENABLED":      "1",
		"COMMERCE_PAYUNI_INGRESS_DATABASE_URL": "postgres://synthetic-secret@synthetic.invalid/payuni",
		"COMMERCE_PAYMENT_PROFILE":             "SANDBOX",
		"COMMERCE_ACCOUNT_ACTIVE_KEY_ID":       "acct-1",
		"COMMERCE_ACCOUNT_KEYS_JSON":           `[{"id":"acct-1","key_base64":"` + key + `"}]`,
		"COMMERCE_ACCOUNT_REPLAY_KEY":          replay,
	}
}

func TestPayuniNotifyDisabledReadsOnlyFlag(t *testing.T) {
	for _, value := range []string{"", "0"} {
		c, err := loadPayuniNotifyConfig(func(name string) string {
			if name != "COMMERCE_PAYUNI_NOTIFY_ENABLED" {
				t.Fatalf("disabled PAYUNi notify read %s", name)
			}
			return value
		}, "0.0.0.0:8080")
		if err != nil || c.enabled {
			t.Fatal("disabled config changed")
		}
		h, closePools, err := buildPayuniNotifyHandler(context.Background(), nil, c)
		if err != nil || h != nil || closePools == nil {
			t.Fatal("disabled notify opened authority")
		}
		closePools()
	}
	for _, bad := range []string{"true", "2", " 1"} {
		if _, err := loadPayuniNotifyConfig(func(string) string { return bad }, "127.0.0.1:8080"); !errors.Is(err, errPayuniNotifyConfig) {
			t.Fatalf("noncanonical flag %q accepted", bad)
		}
	}
}

func TestPayuniNotifyConfigStrictAndRedacted(t *testing.T) {
	values := payuniNotifyTestEnv()
	read := []string{}
	get := func(name string) string { read = append(read, name); return values[name] }
	for _, addr := range []string{"0.0.0.0:8080", "localhost:8080", ":8080", "127.0.0.1:0"} {
		if _, err := loadPayuniNotifyConfig(get, addr); !errors.Is(err, errPayuniNotifyConfig) {
			t.Fatalf("listener %s accepted", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080"} {
		read = read[:0]
		c, err := loadPayuniNotifyConfig(get, addr)
		if err != nil || !c.enabled || c.keys == nil || c.profile != "SANDBOX" {
			t.Fatal("valid private configuration rejected")
		}
		for _, name := range read {
			if strings.HasPrefix(name, "STRIPE_") || strings.HasPrefix(name, "COMMERCE_PAYUNI_") && name != "COMMERCE_PAYUNI_NOTIFY_ENABLED" && name != "COMMERCE_PAYUNI_INGRESS_DATABASE_URL" {
				t.Fatalf("API read forbidden variable %s", name)
			}
		}
		for _, rendered := range []string{fmt.Sprint(c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c)} {
			if strings.Contains(rendered, "synthetic-secret") || !strings.Contains(rendered, "redacted") {
				t.Fatal("config formatting leaked")
			}
		}
		encoded, err := json.Marshal(c)
		if err != nil || strings.Contains(string(encoded), "synthetic-secret") {
			t.Fatal("config JSON leaked")
		}
	}
	for _, tc := range []struct{ name, value string }{
		{"COMMERCE_PAYUNI_INGRESS_DATABASE_URL", ""}, {"COMMERCE_PAYUNI_INGRESS_DATABASE_URL", " "},
		{"COMMERCE_PAYUNI_INGRESS_DATABASE_URL", strings.Repeat("x", 8193)},
		{"COMMERCE_PAYMENT_PROFILE", "LIVE"}, {"COMMERCE_PAYMENT_PROFILE", ""}, {"COMMERCE_PAYMENT_PROFILE", "sandbox"},
		{"COMMERCE_ACCOUNT_KEYS_JSON", "not-json"}, {"COMMERCE_ACCOUNT_REPLAY_KEY", ""},
	} {
		v := payuniNotifyTestEnv()
		v[tc.name] = tc.value
		_, err := loadPayuniNotifyConfig(func(n string) string { return v[n] }, "127.0.0.1:8080")
		if !errors.Is(err, errPayuniNotifyConfig) || (tc.value != "" && strings.Contains(err.Error(), tc.value)) {
			t.Fatalf("%s=%q accepted or leaked: %v", tc.name, tc.value, err)
		}
	}
	// The Stripe webhook signing keyring names must NOT satisfy the shared account keyring: PAYUNi
	// notify opens the same custody that sealed each connection's HashKey/HashIV.
	stripe := payuniNotifyTestEnv()
	for k := range stripe {
		if strings.HasPrefix(k, "COMMERCE_ACCOUNT_") {
			stripe["COMMERCE_STRIPE_WEBHOOK_"+strings.TrimPrefix(k, "COMMERCE_ACCOUNT_")] = stripe[k]
			delete(stripe, k)
		}
	}
	if _, err := loadPayuniNotifyConfig(func(n string) string { return stripe[n] }, "127.0.0.1:8080"); !errors.Is(err, errPayuniNotifyConfig) {
		t.Fatal("COMMERCE_STRIPE_WEBHOOK_* keyring accepted as PAYUNi notify custody")
	}
	if _, closePools, err := buildPayuniNotifyHandler(context.Background(), nil, payuniNotifyConfig{enabled: true}); !errors.Is(err, errPayuniNotifyConfig) || closePools != nil {
		t.Fatal("enabled build without pool/keys accepted")
	}
}

func TestMountPayuniNotifyReservesNamespace(t *testing.T) {
	hits := ""
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits += "F" })
	payuni := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits += "P" })
	mountPayuniNotify(fallback, nil).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/v1/hooks/payuni/notify/x", nil))
	if hits != "F" {
		t.Fatal("disabled mount did not fall through")
	}
	hits = ""
	h := mountPayuniNotify(fallback, payuni)
	token := strings.Repeat("A", 43)
	for _, p := range []string{"/v1/hooks/payuni/notify/" + token, "/v1/hooks/payuni",
		"/v1//hooks/payuni/notify/" + token, "/v1/x/../hooks/payuni/notify/" + token, "/v1/hooks/payunix"} {
		req := httptest.NewRequest("POST", "/", nil)
		req.URL.Path = p
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	if hits != "PPPPP" {
		t.Fatalf("namespace leaked to fallback: %s", hits)
	}
	hits = ""
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/v1/stripe/webhook/x", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/v1/meta/x", nil))
	if hits != "FF" {
		t.Fatalf("unrelated routes captured: %s", hits)
	}
}
