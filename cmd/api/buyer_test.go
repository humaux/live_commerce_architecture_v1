package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuyerDisabledDoesNotReadCredentials(t *testing.T) {
	for _, flag := range []string{"", "0"} {
		c, err := loadBuyerConfig(func(k string) string {
			if k != "COMMERCE_BUYER_ENABLED" {
				t.Fatal("disabled buyer read authority configuration")
			}
			return flag
		}, "0.0.0.0:8080")
		if err != nil || c.enabled {
			t.Fatal("disabled buyer gate changed")
		}
		h, closePools, err := buildBuyerHandler(context.Background(), c)
		if err != nil || h != nil || closePools == nil {
			t.Fatal("disabled buyer opened a dependency")
		}
		closePools()
	}
}

func TestBuyerConfigurationFailClosed(t *testing.T) {
	values := map[string]string{
		"COMMERCE_BUYER_ENABLED": "1", "COMMERCE_BUYER_BFF_KEY": strings.Repeat("A", 43),
		"COMMERCE_BUYER_SESSION_TTL": "1h", "COMMERCE_BUYER_DATABASE_URL": "postgres://test.invalid/buyer",
		"COMMERCE_BUYER_ISSUER_DATABASE_URL": "postgres://test.invalid/issuer", "COMMERCE_CHECKOUT_DATABASE_URL": "postgres://test.invalid/checkout",
	}
	get := func(k string) string { return values[k] }
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080"} {
		c, err := loadBuyerConfig(get, addr)
		if err != nil || !c.enabled || c.ttl != time.Hour {
			t.Fatal("valid private configuration rejected")
		}
	}
	for key, candidates := range map[string][]string{
		"COMMERCE_BUYER_ENABLED":             {"true", "2"},
		"COMMERCE_BUYER_BFF_KEY":             {"", "short", strings.Repeat("A", 42) + "B"},
		"COMMERCE_BUYER_SESSION_TTL":         {"", "59s", "720h1s", "1m0.5s", "forever"},
		"COMMERCE_BUYER_DATABASE_URL":        {"", "  "},
		"COMMERCE_BUYER_ISSUER_DATABASE_URL": {"", "  "},
		"COMMERCE_CHECKOUT_DATABASE_URL":     {"", "  "},
		"COMMERCE_BFF_KEY":                   {values["COMMERCE_BUYER_BFF_KEY"]},
	} {
		for i, value := range candidates {
			t.Run(key+string(rune('a'+i)), func(t *testing.T) {
				_, err := loadBuyerConfig(func(k string) string {
					if k == key {
						return value
					}
					return get(k)
				}, "127.0.0.1:8080")
				if !errors.Is(err, errBuyerConfig) {
					t.Fatal("invalid buyer configuration accepted")
				}
			})
		}
	}
	for _, addr := range []string{":8080", "localhost:8080", "0.0.0.0:8080", "[::]:8080", "127.0.0.1:0"} {
		if _, err := loadBuyerConfig(get, addr); !errors.Is(err, errBuyerConfig) {
			t.Fatal("public or unresolved listener accepted")
		}
	}
	for _, ttl := range []string{"60s", "720h"} {
		values["COMMERCE_BUYER_SESSION_TTL"] = ttl
		if _, err := loadBuyerConfig(get, "127.0.0.1:8080"); err != nil {
			t.Fatal("TTL boundary rejected")
		}
	}
}

func TestBuyerMountedBeforePathCleaning(t *testing.T) {
	fallback := http.NewServeMux()
	fallback.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) })
	var seen string
	buyerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path
		w.WriteHeader(404)
	})
	for _, target := range []string{"/v1/buyer/session", "/v1//buyer/session", "/v1/../v1/buyer/session", "//v1/buyer/session", "/v1/buyer/../session", "/v1/buyer", "/v1/buyer%2fsession"} {
		r := httptest.NewRequest("GET", target, nil)
		w := httptest.NewRecorder()
		seen = ""
		mountBuyer(fallback, buyerHandler).ServeHTTP(w, r)
		if w.Code != 404 || w.Header().Get("Location") != "" || seen != r.URL.Path {
			t.Fatal("buyer path bypassed strict router or was rewritten")
		}
	}
	for _, h := range []http.Handler{mountBuyer(fallback, buyerHandler), mountBuyer(fallback, nil)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
		if w.Code != 418 {
			t.Fatal("nonbuyer route changed")
		}
	}
	w := httptest.NewRecorder()
	mountBuyer(fallback, nil).ServeHTTP(w, httptest.NewRequest("GET", "/v1/buyer/session", nil))
	if w.Code != 418 {
		t.Fatal("disabled buyer route mounted")
	}
}
