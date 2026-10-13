// Package orderattempt owns durable merchant order admission, frozen-plan
// custody and the single-transaction local executor. It never obtains authority
// from browser storage, recomputes an admitted price, commits a partial order or
// contacts a payment/message provider.
//
// It depends on PostgreSQL domain definers, the existing pricing/stock algorithms,
// isolated encryption rings and River InsertTx. API and claims-worker composition
// roots are its callers; the frozen order-attempt-v1 contract defines its fences.
package orderattempt
