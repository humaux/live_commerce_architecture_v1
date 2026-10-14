// Purpose: prevent a raw C2 caller committing only its materialization phase.
// Depends on: real C1 admission, execution fence and domain-owned quote materialization.
// Used by: OA1 CI before exposing the composite prepare/finish execution entry.
// Invariants: I01/I04/I08; a lock-only PENDING Tx is legal, committed partial C2 is not.
package foundation_test

import (
	"context"
	"testing"
)

func TestOrderAttemptExecutionAtomicBoundary(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	ctx := context.Background()
	// A lock/recovery observation does not itself create order effects and may
	// persist its protected witness while the attempt remains pending.
	lockOnly, _ := oaFenceDomainTx(t, e, a, true)
	if err := lockOnly.Commit(ctx); err != nil {
		t.Fatalf("lock-only pending transaction was incorrectly forbidden: %v", err)
	}
	partial, plain := oaFenceDomainTx(t, e, a, true)
	oaStateExec(t, partial, `SET LOCAL ROLE commerce_order_attempt_owner`)
	quote, _, err := oaQuoteCall(partial, a.AttemptID, plain)
	if err != nil || quote.CartID == "" || quote.QuoteID == "" {
		t.Fatalf("real C2 materialization positive sentinel: %v", err)
	}
	// Exercise PostgreSQL's COMMIT boundary, not an application-side condition.
	// The future public prepare/finish caller must not be able to omit finish.
	if err := partial.Commit(ctx); err == nil {
		t.Fatal("partial C2 cart/quote committed while its authority remained PENDING")
	} else {
		oaFenceCode(t, nil, err, "23514")
	}
	var pending bool
	var carts, quotes, orders int
	if err := e.p.f.owner.QueryRow(ctx, `SELECT a.state='PENDING' AND a.plan_ciphertext IS NOT NULL,
	 (SELECT count(*) FROM storefront.carts x WHERE x.create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM storefront.quotes x WHERE x.create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM checkout.orders x WHERE x.create_attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID).Scan(&pending, &carts, &quotes, &orders); err != nil || !pending || carts != 0 || quotes != 0 || orders != 0 {
		t.Fatalf("refused partial commit left durable effects: pending=%v carts=%d quotes=%d orders=%d err=%v", pending, carts, quotes, orders, err)
	}
}
