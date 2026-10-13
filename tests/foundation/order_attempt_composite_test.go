// Purpose: exercise the public C2 prepare stage with actual runtime and worker logins.
// Depends on: native C1, execution fence, frozen plan codec and existing Go allocation.
// Used by: OA1 CI; no owner role switch is available to the composite caller.
// Invariants: one physical transaction, protected exact plan, no placement re-gating,
// rollback of every partial effect, and exact public/private execution authority.
package foundation_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/inventory"
	"livecommerce/internal/orderattempt"
)

type oaCompositePrepared struct {
	Stage         string           `json:"stage"`
	CartID        string           `json:"cart_id"`
	QuoteID       string           `json:"quote_id"`
	DestinationID string           `json:"destination_id"`
	Allocation    oaAllocationWire `json:"allocation"`
}

func oaCompositePrepare(t *testing.T, tx pgx.Tx, a orderattempt.AdmissionReceipt, plain []byte) (oaCompositePrepared, []inventory.Line) {
	t.Helper()
	raw, err := oaCompositeCall(tx, a, plain, "prepare", map[string]any{})
	var prepared oaCompositePrepared
	if err != nil || json.Unmarshal(raw, &prepared) != nil || prepared.Stage != "PREPARED" || prepared.Allocation.OrderID != a.AttemptID {
		t.Fatalf("actual composite prepare sentinel: %v", err)
	}
	lines := []inventory.Line{}
	if len(prepared.Allocation.Demands) > 0 {
		lines, err = inventory.PlanAllocation(prepared.Allocation.WarehouseIDs, prepared.Allocation.Demands, prepared.Allocation.Balances)
		if err != nil {
			t.Fatalf("native locked allocation facts: %v", err)
		}
	}
	return prepared, lines
}

func oaCompositeBeginInput(prepared oaCompositePrepared, lines []inventory.Line) map[string]any {
	return map[string]any{"cart_id": prepared.CartID, "quote_id": prepared.QuoteID,
		"destination_id": prepared.DestinationID, "allocation": lines}
}

func TestOrderAttemptCompositeBeginActualLoginsAndCommitBoundary(t *testing.T) {
	for _, worker := range []bool{false, true} {
		name := "runtime"
		if worker {
			name = "claims-worker"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			tx, plain := oaCompositeTx(t, e, a, worker)
			prepared, lines := oaCompositePrepare(t, tx, a, plain)
			raw, err := oaCompositeCall(tx, a, plain, "begin", oaCompositeBeginInput(prepared, lines))
			var got checkout.Result
			if err != nil || json.Unmarshal(raw, &got) != nil || got.OrderID != a.AttemptID || got.ReservationID != a.AttemptID || got.CommercialState != "AWAITING_TRANSFER" {
				t.Fatalf("actual composite Begin after prepare: %v", err)
			}
			// A native Begin receipt alone is not a complete C2 receipt. The
			// runtime must finish source, pay entry, claims and outbox first.
			oaFenceCode(t, nil, tx.Commit(context.Background()), "23514")
			oaCompositeNoEffects(t, e, a)
			oaBeginZeroEffects(t, e, a.AttemptID)
		})
	}
}

func TestOrderAttemptCompositeBeginRejectsChangedStageIdentity(t *testing.T) {
	for _, field := range []string{"cart_id", "quote_id", "destination_id", "tenant_id", "allocation"} {
		t.Run(field, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			tx, plain := oaCompositeTx(t, e, a, false)
			prepared, lines := oaCompositePrepare(t, tx, a, plain)
			input := oaCompositeBeginInput(prepared, lines)
			input[field] = randomUUID()
			if field == "allocation" {
				forged := append([]inventory.Line(nil), lines...)
				forged[0].Quantity++
				input[field] = forged
			}
			raw, err := oaCompositeCall(tx, a, plain, "begin", input)
			oaFenceCode(t, raw, err, "PT400")
			_ = tx.Rollback(context.Background())
			oaCompositeNoEffects(t, e, a)
			oaBeginZeroEffects(t, e, a.AttemptID)
		})
	}
}

func oaCompositeCall(tx pgx.Tx, a orderattempt.AdmissionReceipt, plain []byte, step string, input any) ([]byte, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var raw []byte
	// Calls ops.execute_order_attempt, order-attempt-v1 §1: stages share this Tx.
	err = tx.QueryRow(context.Background(), `SELECT ops.execute_order_attempt($1::uuid,$2::bigint,$3::text,$4::text,$5::jsonb)`,
		a.AttemptID, a.Generation, string(plain), step, string(encoded)).Scan(&raw)
	return raw, err
}

func oaCompositeTx(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, worker bool) (pgx.Tx, []byte) {
	t.Helper()
	ctx := context.Background()
	pool := e.p.f.runtime
	hash := tokenHash(e.token())
	if worker {
		pool = miPool(t, e.p.f, "commerce_claims_worker")
		hash = nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	var actual bool
	if err = tx.QueryRow(ctx, `SELECT session_user=current_user AND NOT (SELECT rolsuper FROM pg_roles WHERE rolname=current_user)`).Scan(&actual); err != nil || !actual {
		t.Fatal("composite fixture requires an actual nonsuperuser runtime login")
	}
	raw, err := oaFenceCall(ctx, tx, hash, e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	fence := oaFenceOpen(t, raw, a)
	plan, err := oaServiceKeys(t).Open(*fence.Binding, *fence.Envelope)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	return tx, plain
}

func oaCompositeNoEffects(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt) {
	t.Helper()
	var pending bool
	var carts, quotes, destinations, reservations, orders int
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='PENDING' AND a.plan_ciphertext IS NOT NULL,
	 (SELECT count(*) FROM storefront.carts c WHERE c.create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM storefront.quotes q WHERE q.create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM storefront.destination_snapshots d WHERE d.create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM inventory.reservations r WHERE r.create_attempt_id=a.attempt_id),
	 (SELECT count(*) FROM checkout.orders o WHERE o.create_attempt_id=a.attempt_id)
	 FROM ops.order_attempts a WHERE a.attempt_id=$1`, a.AttemptID).
		Scan(&pending, &carts, &quotes, &destinations, &reservations, &orders)
	if err != nil || !pending || carts != 0 || quotes != 0 || destinations != 0 || reservations != 0 || orders != 0 {
		t.Fatalf("partial stage persisted: pending=%v carts=%d quotes=%d destinations=%d reservations=%d orders=%d err=%v",
			pending, carts, quotes, destinations, reservations, orders, err)
	}
}

func TestOrderAttemptCompositePrepareActualLoginsAndCommitBoundary(t *testing.T) {
	for _, worker := range []bool{false, true} {
		name := "runtime"
		if worker {
			name = "claims-worker"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			tx, plain := oaCompositeTx(t, e, a, worker)
			oaStateExec(t, tx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.principal_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true)`)
			raw, err := oaCompositeCall(tx, a, plain, "prepare", map[string]any{})
			if err != nil {
				t.Fatalf("actual login prepare after C1/fence: %v", err)
			}
			var got oaCompositePrepared
			if err = json.Unmarshal(raw, &got); err != nil || got.Stage != "PREPARED" || got.CartID == "" || got.QuoteID == "" || got.DestinationID == "" || got.Allocation.OrderID != a.AttemptID {
				t.Fatalf("prepare omitted native identity: %v", err)
			}
			lines, err := inventory.PlanAllocation(got.Allocation.WarehouseIDs, got.Allocation.Demands, got.Allocation.Balances)
			if err != nil || len(lines) != 1 || lines[0].SKUID != e.money || lines[0].Quantity != 5 {
				t.Fatalf("prepare must return actual stock for the existing Go allocator: %v", err)
			}
			var restored bool
			if err = tx.QueryRow(context.Background(), `SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker' AND current_setting('app.buyer_id')='caller-marker'`).Scan(&restored); err != nil || !restored {
				t.Fatal("composite trusts or leaks caller scope")
			}
			// The runtime can call prepare but cannot turn that into a committed
			// partial checkout. The assertion is the native COMMIT, not a mock.
			oaFenceCode(t, nil, tx.Commit(context.Background()), "23514")
			oaCompositeNoEffects(t, e, a)
		})
	}
}

func TestOrderAttemptCompositePrepareNoPlacementGateReplay(t *testing.T) {
	e := lbuNew(t)
	a := oaFenceAdmit(t, e)
	// Read authority remains; placement-time catalog and reserve grants do not.
	mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE store_id=$1 AND principal_id=$2 AND permission IN ('catalog:read','inventory:reserve')`, e.store(), e.p.f.principalA)
	tx, plain := oaCompositeTx(t, e, a, false)
	raw, err := oaCompositeCall(tx, a, plain, "prepare", map[string]any{})
	var got oaCompositePrepared
	if err != nil || json.Unmarshal(raw, &got) != nil || got.Stage != "PREPARED" || got.CartID == "" {
		t.Fatalf("admitted continuation incorrectly reapplied placement grants: %v", err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	oaCompositeNoEffects(t, e, a)
}

func TestOrderAttemptCompositePrepareRejectsUnboundInput(t *testing.T) {
	for _, mode := range []string{"no-fence", "old-fence", "generation", "plain", "step", "input"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			tx, plain := oaCompositeTx(t, e, a, false)
			if mode == "no-fence" || mode == "old-fence" {
				var err error
				if mode == "old-fence" {
					err = tx.Commit(context.Background())
				} else {
					err = tx.Rollback(context.Background())
				}
				if err != nil {
					t.Fatal(err)
				}
				tx, err = e.p.f.runtime.Begin(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
			}
			step, code := "prepare", "PT409"
			input := map[string]any{}
			switch mode {
			case "generation":
				a.Generation++
			case "plain":
				var forged map[string]any
				if err := json.Unmarshal(plain, &forged); err != nil {
					t.Fatal(err)
				}
				forged["generation"] = 999
				plain, _ = json.Marshal(forged)
				code = "PT400"
			case "step":
				step, code = "arbitrary", "PT400"
			case "input":
				input["tenant_id"], code = e.store(), "PT400"
			}
			raw, err := oaCompositeCall(tx, a, plain, step, input)
			oaFenceCode(t, raw, err, code)
			_ = tx.Rollback(context.Background())
			oaCompositeNoEffects(t, e, a)
		})
	}
}
