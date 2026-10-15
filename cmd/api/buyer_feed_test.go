// Purpose: gate exact feed mounting and BFF-key authentication through assembly.
// Depends on: attribution.FeedRoutes, net/http/httptest and mountFeed.
// Used by: cmd/api CI; no database or production services.
// Invariant: the tested feed path must belong to the runtime route table.

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"livecommerce/internal/attribution"
)

// mountFeed: only the exact feed path, only with exactly one correct BFF key; everything else reaches next.
func TestMountFeed(t *testing.T) {
	const feedPath = "/v1/buyer/feeds/meta.csv"
	if !attribution.FeedRoutes.MatchesPath(feedPath) {
		t.Fatal("test feed path differs from runtime route table")
	}
	const key = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG" // 43-char neutral fixture, not a secret
	var next, feed int
	h := mountFeed(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { next++; w.WriteHeader(http.StatusTeapot) }),
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { feed++; w.WriteHeader(http.StatusOK) }), key)
	do := func(path string, keys ...string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for _, k := range keys {
			req.Header.Add("X-Commerce-Buyer-BFF-Key", k)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := do(feedPath, key); c != 200 || feed != 1 {
		t.Fatalf("right key = %d feed=%d", c, feed)
	}
	for name, keys := range map[string][]string{"none": nil, "wrong": {key[1:] + "x"}, "two": {key, key}} {
		if c := do(feedPath, keys...); c != 401 || feed != 1 {
			t.Errorf("%s = %d feed=%d", name, c, feed)
		}
	}
	if c := do("/v1/buyer/catalog", key); c != http.StatusTeapot || next != 1 {
		t.Fatalf("other path = %d next=%d", c, next)
	}
}
