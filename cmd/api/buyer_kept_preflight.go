// Purpose: refuse startup if actual retained buyer pools differ from main's database.
// Depends on: captured buyerConfig and validation-only retained ManualOrders API.
// Used by: run after factory cleanup is registered and before handler assembly.
// Invariants: disabled is a no-op; enabled fails closed; factory and pool ownership stay intact.
package main

import "context"

func buyerKeptDatabasePreflight(ctx context.Context, c buyerConfig, parts cvsParts) error {
	if !c.enabled {
		return nil
	}
	if ctx == nil || parts.Manual == nil || parts.Manual.ValidateSameDatabase(ctx) != nil {
		return errBuyerConfig
	}
	return nil
}
