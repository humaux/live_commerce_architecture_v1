// Purpose: enforce disabled and invalid startup preflight boundaries without PostgreSQL.
// Depends on: buyerDatabasePreflight and buyerConfig startup seam.
// Used by: cmd/api unit tests and OA1 CI acceptance.
// Invariants: disabled mode needs no authority; enabled mode rejects invalid dependencies.
package main

import (
	"context"
	"errors"
	"testing"
)

func TestBuyerDatabasePreflightDisabledNeedsNoDependencies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, disabledCtx := range []context.Context{nil, ctx} {
		if err := buyerDatabasePreflight(disabledCtx, buyerConfig{issuerDSN: "invalid", buyerDSN: "invalid", checkoutDSN: "invalid"}, nil); err != nil {
			t.Fatal("disabled preflight required a context, pool or authority")
		}
	}
}

func TestBuyerDatabasePreflightEnabledRejectsMissingReference(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, invalidCtx := range []context.Context{nil, context.Background(), ctx} {
		err := buyerDatabasePreflight(invalidCtx, buyerConfig{enabled: true}, nil)
		if !errors.Is(err, errBuyerConfig) || err.Error() != errBuyerConfig.Error() {
			t.Fatal("enabled preflight admitted a missing reference dependency")
		}
	}
}
