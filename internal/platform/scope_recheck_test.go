// Purpose: SCOPE-404 fail-closed leg — a re-resolve that cannot answer never rewrites a denial.
// Depends on: platform.scopeLost / reResolveScopeLoss (platform.go), pgx.Row.
// Used by: go test ./internal/platform (DB-free; the REAL_PG legs live in tests/foundation/scope_revoke_race_test.go).
package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type failingRow struct{}

func (failingRow) Scan(...any) error { return errors.New("re-check failed") }

type failingQuerier struct{}

func (failingQuerier) QueryRow(context.Context, string, ...any) pgx.Row { return failingRow{} }

func TestScopeRecheckFailureKeepsDenial(t *testing.T) {
	if scopeLost(context.Background(), failingQuerier{}, make([]byte, 32), "00000000-0000-4000-8000-000000000001", "inbox:read") {
		t.Fatal("a failed re-check reported scope loss; it must keep the original denial")
	}
}
