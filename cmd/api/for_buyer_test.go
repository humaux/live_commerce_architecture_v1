// Purpose: DB-free wiring checks of the order-for-a-buyer service (LC-B6) in the API process: without the buyer surface there is no service (the routes
// then answer 503 manual_order_unavailable), and an untyped nil inbox is never turned into a typed-nil planner.
// Depends on: buildForBuyer (inbox.go), internal/merchanttools.
// Used by: go test ./cmd/api.

package main

import "testing"

func TestBuildForBuyerWithoutTheBuyerSurfaceIsNil(t *testing.T) {
	forBuyer, err := buildForBuyer(nil, nil)
	if err != nil || forBuyer != nil {
		t.Fatalf("no manual-order pipeline must give no service: %v %v", forBuyer, err)
	}
}
