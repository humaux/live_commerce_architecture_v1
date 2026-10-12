// Purpose: PR #31 review — the template resolver's fixed PT codes keep their meaning on the claims-classified sold-out PUT.
// Depends on: soldOutResolveError (sold_out_settings.go) and claimsClassify (claims.go).
// Used by: go test ./internal/httpapi.
package httpapi

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSoldOutResolveErrorKeepsDefinerMeaning(t *testing.T) {
	// Counterexample: the raw resolver PT403 (live:manage revoked mid-request) is a 503 under claimsClassify, so the
	// client would treat a rolled-back write as UNKNOWN and keep a futile retry receipt.
	if status, _ := claimsClassify(&pgconn.PgError{Code: "PT403"}); status != http.StatusServiceUnavailable {
		t.Fatalf("precondition: raw PT403 under claimsClassify = %d", status)
	}
	for code, want := range map[string]int{"PT403": http.StatusForbidden, "PT404": http.StatusNotFound, "PT409": http.StatusUnprocessableEntity, "PT422": http.StatusUnprocessableEntity} {
		if status, _ := claimsClassify(soldOutResolveError(&pgconn.PgError{Code: code})); status != want {
			t.Errorf("%s -> %d, want %d", code, status, want)
		}
	}
}
