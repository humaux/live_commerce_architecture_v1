package platform

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// WithScopeBudget (merchant-tools CSV import): the budget is bounded to (5 s, 120 s], and its timeouts are spelled in a unit PostgreSQL accepts
// (Duration.String() gives "1m0s" for a minute, which a set_config of statement_timeout refuses: found by the first REAL_PG run).
func TestWithScopeBudgetBoundsAndPostgresUnits(t *testing.T) {
	noop := func(pgx.Tx, Scope) error { return nil }
	for _, budget := range []time.Duration{0, time.Second, requestTimeout, 121 * time.Second, time.Hour} {
		if err := WithScopeBudget(context.Background(), nil, "t", "s", "p", budget, noop); err != ErrUnauthorized {
			t.Errorf("budget %s: %v, want ErrUnauthorized before any database work", budget, err)
		}
	}
	for _, c := range []struct {
		in   time.Duration
		want string
	}{{5 * time.Second, "5000ms"}, {time.Minute, "60000ms"}, {90 * time.Second, "90000ms"}, {1500 * time.Millisecond, "1500ms"}} {
		if got := pgMillis(c.in); got != c.want {
			t.Errorf("pgMillis(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}
