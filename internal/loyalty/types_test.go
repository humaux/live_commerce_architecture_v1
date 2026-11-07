// Purpose: DB-free checks of the loyalty refusal mapping (contract §2) and the award input rule.
// Depends on: Classify, ValidAward, the sentinel errors.
// Used by: go test ./internal/loyalty.

package loyalty

import (
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/platform"
)

func TestClassifyContractCodes(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
		ok     bool
	}{
		{ErrInsufficient, 409, "points_insufficient", true},
		{fmt.Errorf("wrapped: %w", ErrErased), 409, "member_erased", true},
		{ErrLimit, 422, "limit_reached", true},
		{ErrVersion, 409, "conflict", true},
		{ErrExportTooLarge, 409, "export_too_large", true},
		{ErrIdempotencyConflict, 409, "idempotency_conflict", true},
		{ErrUnavailable, 503, "unavailable", true},
		// Lock waits and deadlocks are a same-key retry (§2: 503 retry_later), never a refusal of the request.
		{&pgconn.PgError{Code: "55P03"}, 503, "retry_later", true},
		{&pgconn.PgError{Code: "40P01"}, 503, "retry_later", true},
		// A backstop CHECK reaching Classify is a bug: it is never shown as a refusal code (LPG01b).
		{&pgconn.PgError{Code: "23514"}, 503, "unavailable", true},
		{platform.ErrForbidden, 0, "", false},
		{&pgconn.PgError{Code: "PT403", Message: "forbidden"}, 0, "", false},
		{&pgconn.PgError{Code: "PT409", Message: "conflict"}, 0, "", false},
	} {
		status, code, ok := Classify(c.err)
		if status != c.status || code != c.code || ok != c.ok {
			t.Errorf("Classify(%v) = %d %q %v, want %d %q %v", c.err, status, code, ok, c.status, c.code, c.ok)
		}
	}
}

func TestValidAward(t *testing.T) {
	const id = "33333333-3333-4333-8333-333333333333"
	for _, c := range []struct {
		a  Award
		ok bool
	}{{Award{id, 1}, true}, {Award{id, MaxPoints}, true}, {Award{id, 0}, false}, {Award{id, MaxPoints + 1}, false}, {Award{"x", 5}, false}} {
		if ValidAward(c.a) != c.ok {
			t.Errorf("ValidAward(%+v) != %v", c.a, c.ok)
		}
	}
}
