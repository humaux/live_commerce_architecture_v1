// Package orderattempt owns durable merchant order admission, frozen-plan
// custody and the single-transaction local executor. It never obtains authority
// from browser storage, recomputes an admitted price, commits a partial order or
// contacts a payment/message provider.
//
// Purpose: durable merchant admission, frozen-plan custody and atomic local execution.
// Depends on: PostgreSQL domain definers, pricing/stock algorithms, isolated encryption
// rings and River InsertTx; order-attempt-v1 defines the execution fences.
// Used by: API and claims-worker composition roots.
package orderattempt
