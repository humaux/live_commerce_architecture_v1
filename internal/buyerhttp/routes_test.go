// Purpose: CI gates for table consumption, strict transport rejection and legacy equivalence.
// Depends on: private routeRows, a frozen pre-table matcher and httperror's normal response envelope.
// Used by: buyerhttp CI; no database, browser, server or production authority.
// Invariant: the table cannot add/lose a method or alter path precedence unnoticed.

package buyerhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
)

func routeSample(pattern string) string {
	return strings.NewReplacer("{id}", "00000000-0000-0000-0000-000000000001",
		"{image}", "00000000-0000-0000-0000-000000000002", "{slug}", "summer-dress", "{slug_or_id}", "summer-dress").Replace(pattern)
}

func TestRouteTableEntriesAreDispatched(t *testing.T) {
	if len(routeRows) != 46 {
		t.Fatalf("inventory changed: %d entries", len(routeRows))
	}
	seen := map[string]bool{}
	for _, entry := range routeRows {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			key := entry.Method + " " + entry.Path
			if seen[key] {
				t.Fatal("duplicate route")
			}
			seen[key] = true
			selected := matchRoute(routeSample(entry.Path))
			if selected.kind == unknownRoute || selected.kind != entry.kind || !allowed(selected.kind, entry.Method) {
				t.Fatalf("table entry not dispatched: %+v -> %+v", entry, selected)
			}
			// Read the visible transport boundary without resolving a domain or hitting SQL:
			// every listed path must reach the method guard before buyer authority.
			r := httptest.NewRequest("TRACE", routeSample(entry.Path), nil)
			r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
			r.Header.Set("X-Commerce-Storefront-Origin", "https://shop.example.invalid")
			w := httptest.NewRecorder()
			httperror.Middleware(&handler{bffKey: testBuyerKey}).ServeHTTP(w, r)
			if w.Code != 405 || !strings.Contains(w.Body.String(), `"code":"method_not_allowed"`) || w.Header().Get("Location") != "" {
				t.Fatalf("not dispatched to method guard: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestRouteTableUnknownPathStill404(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/buyer/not-listed", nil)
	r.Header.Set("X-Commerce-Buyer-BFF-Key", testBuyerKey)
	r.Header.Set("X-Commerce-Storefront-Origin", "https://shop.example.invalid")
	w := httptest.NewRecorder()
	httperror.Middleware(&handler{bffKey: testBuyerKey}).ServeHTTP(w, r)
	if w.Code != 404 || !strings.Contains(w.Body.String(), `"code":"not_found"`) || w.Header().Get("Location") != "" {
		t.Fatalf("unlisted path changed: %d %s", w.Code, w.Body.String())
	}
}

func TestRouteTableMatchesLegacy(t *testing.T) {
	paths := map[string]bool{"/v1/buyer": true, "/v1/buyer/": true, "/v1/buyer/not-listed": true}
	for _, entry := range routeRows {
		path := routeSample(entry.Path)
		for _, p := range []string{path, path + "/", path + "/extra", strings.Replace(path, "/v1/buyer/", "/v1//buyer/", 1),
			strings.Replace(path, "/v1/buyer/", "/v1/buyer/../buyer/", 1)} {
			paths[p] = true
		}
		for _, value := range []string{"", "x", ".", "..", "a/b", "%2F", "UPPER", "a_b", strings.Repeat("a", 81), "00000000-0000-0000-0000-000000000001"} {
			for _, param := range []string{"{id}", "{image}", "{slug}", "{slug_or_id}"} {
				paths[routeSample(strings.ReplaceAll(entry.Path, param, value))] = true
			}
		}
	}
	methods := []string{"", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT", "get"}
	for path := range paths {
		got, want := matchRoute(path), legacyMatchRoute(path)
		if got != want {
			t.Errorf("path %q: got %+v want %+v", path, got, want)
		}
		for _, method := range methods {
			if allowed(got.kind, method) != legacyAllowed(want.kind, method) {
				t.Errorf("method drift: %s %s", method, path)
			}
		}
	}
}
