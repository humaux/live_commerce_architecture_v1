package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzNeedsNoDatabase(t *testing.T) {
	response := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("healthz = %d, %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestReadyzUnavailableWithoutPool(t *testing.T) {
	response := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable || response.Body.String() != "{\"error\":\"unavailable\"}\n" {
		t.Fatalf("readyz = %d, %q", response.Code, response.Body.String())
	}
}

func TestAdminRejectsCookieAndTenantHeadersWithoutBearer(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/admin/stores/00000000-0000-0000-0000-000000000000", nil)
	request.Header.Set("Cookie", "session=not-a-token")
	request.Header.Set("X-Tenant", "other-tenant")
	response := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Body.String() != "{\"error\":\"unauthorized\"}\n" {
		t.Fatalf("admin response = %d, %q", response.Code, response.Body.String())
	}
}

func TestCanonicalUUID(t *testing.T) {
	if !isCanonicalUUID("123e4567-e89b-12d3-a456-426614174000") {
		t.Fatal("canonical UUID rejected")
	}
	if isCanonicalUUID("123E4567-E89B-12D3-A456-426614174000") || isCanonicalUUID("nope") {
		t.Fatal("non-canonical UUID accepted")
	}
}
