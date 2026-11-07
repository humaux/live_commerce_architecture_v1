// Package loyalty owns store-scoped merchant member records and their append-only
// points ledger, including manual adjustments, manual-order awards, and the
// bounded privacy export and erasure operations of contracts/loyalty-points-v1.md.
// It never matches members to buyer identities, computes discounts or automatic
// points, redeems rewards, or contacts external services. Points are a merchant
// record; balance and ledger writes share the authorized database transaction.
package loyalty
