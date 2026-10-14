// Purpose: reject invalid retained buyer preflight dependencies without PostgreSQL.
// Depends on: buyerKeptDatabasePreflight startup seam.
// Used by: OA1 cmd/api unit CI.
// Invariants: disabled needs no dependencies; enabled cannot accept absent held pools.
package main

import (
	"context"
	"errors"
	"testing"
)

func TestBuyerKeptDatabasePreflightMissingParts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, candidate := range []context.Context{nil, context.Background(), ctx} {
		if err := buyerKeptDatabasePreflight(candidate, buyerConfig{}, cvsParts{}); err != nil {
			t.Fatal("disabled retained preflight required dependencies")
		}
		err := buyerKeptDatabasePreflight(candidate, buyerConfig{enabled: true}, cvsParts{})
		if !errors.Is(err, errBuyerConfig) || err.Error() != errBuyerConfig.Error() {
			t.Fatal("enabled retained preflight accepted absent dependencies")
		}
	}
}
