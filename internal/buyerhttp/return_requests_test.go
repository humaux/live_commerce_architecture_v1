// Purpose: freeze FEAT-BUYER-RMA transport admission before the buyer return-request implementation.
// Depends on: existing buyer Routes, handler early guards and viewAllowed; no database or new production symbol.
// Used by: GitHub CI unit shard; all execution stays in CI.
// Invariants: exact methods/paths, private authentication, no view-only mutation or credential-bearing redirects.
package buyerhttp

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"livecommerce/internal/httperror"
)

func TestBuyerReturnRequestTransport(t *testing.T) {
	const id = "12345678-1234-4234-8234-123456789abc"
	for _, tc := range []struct{ method, path string }{
		{"POST", "/v1/buyer/orders/" + id + "/return-requests"},
		{"GET", "/v1/buyer/return-requests"},
		{"GET", "/v1/buyer/return-requests/" + id},
		{"POST", "/v1/buyer/return-requests/" + id + "/withdraw"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			selected := matchRoute(tc.path)
			if selected.kind == unknownRoute || !allowed(selected.kind, tc.method) {
				t.Fatalf("buyer return-request endpoint is not registered: %s %s", tc.method, tc.path)
			}
			if allowed(selected.kind, "DELETE") || allowed(selected.kind, "PATCH") || allowed(selected.kind, "HEAD") {
				t.Fatal("return-request transport admitted an undeclared method")
			}
			if viewAllowed(selected, tc.method, id) {
				t.Fatal("view-only guest lookup acquired buyer request authority")
			}
			for _, alias := range []string{tc.path + "/", strings.Replace(tc.path, "/v1/buyer/", "/v1/buyer//", 1)} {
				if matchRoute(alias).kind != unknownRoute {
					t.Fatalf("noncanonical alias admitted: %s", alias)
				}
			}
			// No BFF key: the real handler must refuse before touching its nil pools.
			w := httptest.NewRecorder()
			h := httperror.Middleware(&handler{bffKey: testBuyerKey})
			h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("missing private credential: got %d want 401", w.Code)
			}
		})
	}
}

// Independent 0210 oracle adds the declared routes to the legacy equivalence gate without changing its frozen matcher.
func returnRequestOracle(path string) route {
	if path == "/v1/buyer/return-requests" {
		return route{kind: routeReturnRequestList}
	}
	if parts := regexp.MustCompile(`^/v1/buyer/orders/([^/]+)/return-requests$`).FindStringSubmatch(path); parts != nil {
		return route{kind: routeReturnRequestCreate, id: parts[1]}
	}
	if parts := regexp.MustCompile(`^/v1/buyer/return-requests/([^/]+)(/withdraw)?$`).FindStringSubmatch(path); parts != nil {
		if parts[2] != "" {
			return route{kind: routeReturnRequestWithdraw, id: parts[1]}
		}
		return route{kind: routeReturnRequestDetail, id: parts[1]}
	}
	return route{}
}
func returnRequestOracleAllowed(kind routeKind, method string) bool {
	switch kind {
	case routeReturnRequestList, routeReturnRequestDetail:
		return method == http.MethodGet
	case routeReturnRequestCreate, routeReturnRequestWithdraw:
		return method == http.MethodPost
	default:
		return false
	}
}
