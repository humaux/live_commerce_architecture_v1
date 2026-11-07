// Purpose: document the member points domain ownership and non-goals.
// Depends on: contracts/loyalty-points-v1.md and PostgreSQL migration 0223.
// Used by: Go package documentation and the generated dependency map.

// Package loyalty owns store-scoped merchant member records and their append-only
// points ledger, including manual adjustments, manual-order awards, and the
// bounded privacy export and erasure operations of contracts/loyalty-points-v1.md.
// It never matches members to buyer identities, computes discounts or automatic
// points, redeems rewards, or contacts external services. Points are a merchant
// record; balance and ledger writes share the authorized database transaction.
package loyalty
