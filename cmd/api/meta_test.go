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

func metaTestEnv() map[string]string {
	return map[string]string{
		"COMMERCE_META_WEBHOOK_ENABLED":       "1",
		"COMMERCE_META_INGRESS_DATABASE_URL":  "postgres://synthetic.invalid/meta",
		"COMMERCE_META_APPS_JSON":             `{"apps":[{"app_id":"123","object":"page","app_secret":"1234567890abcdef","verify_token":"abcdef1234567890"}]}`,
		"COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID": "active",
		"COMMERCE_META_PAYLOAD_KEYS_JSON":     `{"keys":[{"id":"active","key_base64":"` + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)) + `"}]}`,
	}
}

func TestMetaAPIDisabledAndPrivateBeforeSecrets(t *testing.T) {
	for _, flagValue := range []string{"", "0"} {
		c, err := loadMetaConfig(func(name string) string {
			if name != "COMMERCE_META_WEBHOOK_ENABLED" {
				t.Fatal("disabled Meta API read secret setting")
			}
			return flagValue
		}, "0.0.0.0:8080")
		if err != nil || c.enabled {
			t.Fatal("disabled Meta API changed")
		}
		h, closePools, err := buildMetaHandler(context.Background(), nil, c)
		if err != nil || h != nil || closePools == nil {
			t.Fatal("disabled Meta API opened authority")
		}
		closePools()
	}
	for _, addr := range []string{"0.0.0.0:8080", "localhost:8080", ":8080", "127.0.0.1:0"} {
		_, err := loadMetaConfig(func(name string) string {
			if name != "COMMERCE_META_WEBHOOK_ENABLED" {
				t.Fatal("public Meta listener read authority setting")
			}
			return "1"
		}, addr)
		if !errors.Is(err, errMetaConfig) {
			t.Fatal("public listener accepted")
		}
	}
	for _, enabled := range []string{"true", "2", " 1"} {
		if _, err := loadMetaConfig(func(string) string { return enabled }, "127.0.0.1:8080"); !errors.Is(err, errMetaConfig) {
			t.Fatal("noncanonical enabled flag accepted")
		}
	}
}

func TestMetaAPIConfigAndRedaction(t *testing.T) {
	values := metaTestEnv()
	get := func(name string) string { return values[name] }
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080"} {
		c, err := loadMetaConfig(get, addr)
		if err != nil || !c.enabled || len(c.endpoints) != 1 {
			t.Fatal("valid private Meta configuration rejected")
		}
		for _, rendered := range []string{fmt.Sprint(c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c)} {
			if strings.Contains(rendered, c.dsn) || strings.Contains(rendered, "1234567890abcdef") || !strings.Contains(rendered, "redacted") {
				t.Fatal("Meta API config formatting leaked secret")
			}
		}
		encoded, err := json.Marshal(c)
		if err != nil || strings.Contains(string(encoded), c.dsn) || !strings.Contains(string(encoded), "redacted") {
			t.Fatal("Meta API config JSON leaked secret")
		}
	}
	for _, value := range []string{"", " ", strings.Repeat("x", 8193)} {
		values["COMMERCE_META_INGRESS_DATABASE_URL"] = value
		if _, err := loadMetaConfig(get, "127.0.0.1:8080"); !errors.Is(err, errMetaConfig) {
			t.Fatal("invalid ingress URL accepted")
		}
	}
	if _, err := loadMetaConfig(nil, "127.0.0.1:8080"); !errors.Is(err, errMetaConfig) {
		t.Fatal("nil environment accepted")
	}
}

func TestMetaMountedBeforeServeMuxCleaning(t *testing.T) {
	fallback := http.NewServeMux()
	fallback.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) })
	called := false
	metaHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNotFound)
	})
	for _, target := range []string{
		"/v1/meta/webhooks/123/page", "/v1//meta/webhooks/123/page",
		"/v1/../v1/meta/webhooks/123/page", "//v1/meta/webhooks/123/page",
		"/v1/meta/../identity", "/v1/%6deta/webhooks/123/page",
	} {
		called = false
		w := httptest.NewRecorder()
		mountMeta(fallback, metaHandler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		if !called || w.Code != http.StatusNotFound || w.Header().Get("Location") != "" {
			t.Fatal("Meta prefix bypassed outer router")
		}
	}
	for _, h := range []http.Handler{mountMeta(fallback, metaHandler), mountMeta(fallback, nil)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if w.Code != 418 {
			t.Fatal("non Meta route changed")
		}
	}
}
