// Purpose: REAL_PG tests-first acceptance of native attempt-order expiry compatibility.
// Depends on: real C1 and public prepare/begin/pay/finish, the existing expiry worker and native stock.
// Used by: parent OA1 CI; no fabricated COMMITTED attempt, queue consumer on a shared database, or provider call.
// Invariants: I01/I03/I04/I08/I14; exact creator provenance, canonical order serialization,
// one release only, finalized offline stock retained, immutable terminal attempt receipt and legacy behavior.
package foundation_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/checkout"
	"livecommerce/internal/orderattempt"
)

// The River consumer below owns this isolated fixture. Its existing storefront
// seed is kept in the future; no other task's expiry queue is consumed.
func oaExpiryIsolated(t *testing.T) *lbuEnv {
	t.Helper()
	p := psSetupItemsOn(t, pwIsolatedFixture(t), 1)
	base := &tcvEnv{t: t, p: p}
	base.grantCreator("orders:read")
	mustExec(t, p.f.owner, `INSERT INTO checkout.bank_transfer_settings
	 (tenant_id,store_id,enabled,allow_cvs,bank_name,account_name,account_number,window_hours,version,updated_at)
	 VALUES($1,$2,true,false,'Synthetic bank','Synthetic merchant','12345678',6,1,clock_timestamp())`, base.tenant(), base.store())
	return &lbuEnv{ltgEnv: &ltgEnv{tcvEnv: base, money: p.stock.skus[0].ID}}
}

// This fixture performs the real public stages and outer commit. The terminal
// state is produced by finish, never owner-injected to bypass the deferred guard.
func oaExpiryCommitted(t *testing.T, e *lbuEnv, mode string) (orderattempt.AdmissionReceipt, checkout.Result) {
	t.Helper()
	a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, mode, "duplicate-origins"))
	tx, plain := oaCompositeTx(t, e, a, false)
	oaAbortCleanupTx(t, tx)
	prepared, lines := oaCompositePrepare(t, tx, a, plain)
	raw, err := oaCompositeCall(tx, a, plain, "begin", oaCompositeBeginInput(prepared, lines))
	var order checkout.Result
	if err != nil || json.Unmarshal(raw, &order) != nil || order.OrderID != a.AttemptID || order.JobID <= 0 {
		t.Fatalf("native composite Begin before expiry: %v", err)
	}
	material, err := payEntryTestKeys(t).Issue(orderattempt.PayEntryBinding{TenantID: e.tenant(), StoreID: e.store(), OrderID: order.OrderID, Version: 1})
	if err != nil {
		t.Fatal("synthetic credential encryption prerequisite")
	}
	raw, err = oaCompositeCall(tx, a, plain, "pay", map[string]any{
		"key_version": material.KeyVersion, "token_hash_hex": hex.EncodeToString(material.Digest[:]),
		"credential_ciphertext_hex": hex.EncodeToString(material.Ciphertext),
	})
	var pay struct {
		Stage string `json:"stage"`
		Notification struct {
			State string `json:"state"`
		} `json:"notification"`
	}
	if err != nil || json.Unmarshal(raw, &pay) != nil || pay.Stage != "PAY_PREPARED" || pay.Notification.State != "NOT_REQUESTED" {
		t.Fatalf("native pay/notification prerequisite: %v", err)
	}
	raw, err = oaCompositeCall(tx, a, plain, "finish", map[string]any{"state": "NOT_REQUESTED"})
	var finish struct {
		State string `json:"state"`
		Receipt struct {
			State   string `json:"state"`
			OrderID string `json:"order_id"`
		} `json:"receipt"`
	}
	var wrapper map[string]json.RawMessage
	if err != nil || json.Unmarshal(raw, &finish) != nil || json.Unmarshal(raw, &wrapper) != nil || len(wrapper) != 2 ||
		finish.State != "COMMITTED" || finish.Receipt.State != "COMMITTED" || finish.Receipt.OrderID != order.OrderID {
		t.Fatalf("full native C2 finish before expiry: %v", err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatalf("complete C2 commit prerequisite: %v", err)
	}
	var exact bool
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT a.state='COMMITTED' AND a.order_id=o.id
	 AND a.receipt->>'order_id'=o.id::text AND o.creator_kind='order_attempt' AND o.create_attempt_id=a.attempt_id
	 AND o.creator_session_id IS NULL AND o.source='merchant_manual'
	 AND EXISTS(SELECT 1 FROM checkout.order_pay_entries p WHERE p.order_id=o.id AND p.create_attempt_id=a.attempt_id)
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id WHERE a.attempt_id=$1`, a.AttemptID).Scan(&exact); err != nil || !exact {
		t.Fatalf("durable terminal/native order/pay sentinel: %v", err)
	}
	return a, order
}

func oaExpiryCall(t *testing.T, pool *pgxpool.Pool, order string, generation int64) (string, *time.Time) {
	t.Helper()
	var disposition string
	var retry *time.Time
	if err := pool.QueryRow(context.Background(), `SELECT disposition,retry_at FROM checkout.expire_held($1::uuid,$2::bigint)`, order, generation).Scan(&disposition, &retry); err != nil {
		t.Fatalf("actual expiry authority native transition: %v", err)
	}
	return disposition, retry
}

func oaExpiryReceipt(t *testing.T, e *lbuEnv, attempt string) string {
	t.Helper()
	var receipt string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT receipt::text FROM ops.order_attempts WHERE attempt_id=$1 AND state='COMMITTED'`, attempt).Scan(&receipt); err != nil {
		t.Fatal(err)
	}
	return receipt
}

func oaExpiryBalance(t *testing.T, e *lbuEnv) [4]int64 {
	t.Helper()
	var balance [4]int64
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT on_hand,reserved,allocated,unavailable FROM inventory.balances
	 WHERE tenant_id=$1 AND store_id=$2 AND warehouse_id=$3 AND sku_id=$4`, e.tenant(), e.store(), e.p.stock.warehouse.ID, e.money).
		Scan(&balance[0], &balance[1], &balance[2], &balance[3]); err != nil {
		t.Fatal(err)
	}
	return balance
}

func TestOrderAttemptExpiryActualWorkerReleasesBankExactlyOnce(t *testing.T) {
	e := oaExpiryIsolated(t)
	a, order := oaExpiryCommitted(t, e, "bank_transfer")
	before, receipt := oaExpiryBalance(t, e), oaExpiryReceipt(t, e, a.AttemptID)
	if disposition, retry := oaExpiryCall(t, e.p.expiry, order.OrderID, 1); disposition != "NOT_DUE" || retry == nil || !retry.After(time.Now()) {
		t.Fatal("fresh bank hold must snooze with its native deadline")
	}
	if disposition, _ := oaExpiryCall(t, e.p.expiry, order.OrderID, 2); disposition != "STALE" {
		t.Fatal("wrong generation released current hold")
	}
	// Disclosed clock fixture: age native order/reservation only after complete
	// C2; immutable attempt/admission/receipt and all guards remain untouched.
	bcDue(t, e.p.bcHarness, order)
	mustExec(t, e.p.f.owner, `UPDATE river_expiry.river_job SET scheduled_at=clock_timestamp() WHERE id=$1`, order.JobID)
	ewClient(t, e.p.expiry, 1)
	ewAwait(t, e.p.f.owner, order.JobID, "completed")
	var exact bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT
	 EXISTS(SELECT 1 FROM checkout.orders WHERE id=$1 AND commercial_state='CANCELLED' AND fulfillment_state='CANCELLED'
	  AND creator_kind='order_attempt' AND create_attempt_id=$1 AND creator_session_id IS NULL)
	 AND EXISTS(SELECT 1 FROM inventory.reservations WHERE id=$1 AND state='EXPIRED' AND creator_kind='order_attempt' AND create_attempt_id=$1 AND buyer_session_id IS NULL)
	 AND EXISTS(SELECT 1 FROM checkout.bank_transfers WHERE order_id=$1 AND state='EXPIRED')
	 AND (SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='RELEASE')=1
	 AND EXISTS(SELECT 1 FROM inventory.ledger WHERE checkout_id=$1 AND kind='RELEASE' AND delta_reserved=-5 AND delta_allocated=0
	  AND operation='checkout.expire' AND command_key=$1::uuid::text AND actor_kind='SYSTEM_EXPIRY' AND principal_id IS NULL
	  AND creator_kind='order_attempt' AND create_attempt_id=$1 AND buyer_session_id IS NULL
	  AND buyer_owner_id=(SELECT buyer_owner_id FROM ops.order_attempts WHERE attempt_id=$1))
	 AND (SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action='checkout.expired')=1
	 AND EXISTS(SELECT 1 FROM checkout.events WHERE order_id=$1 AND action='checkout.expired' AND actor_kind='SYSTEM_EXPIRY'
	  AND creator_kind='order_attempt' AND create_attempt_id=$1 AND session_id IS NULL
	  AND owner_id=(SELECT buyer_owner_id FROM ops.order_attempts WHERE attempt_id=$1) AND generation=1)`,
		order.OrderID).Scan(&exact); err != nil || !exact {
		t.Fatalf("expiry creator, ledger/event and transfer evidence: %v", err)
	}
	want := before
	want[1] -= 5
	if got := oaExpiryBalance(t, e); got != want {
		t.Fatalf("native stock release got=%v want=%v", got, want)
	}
	for i := 0; i < 2; i++ {
		if disposition, _ := oaExpiryCall(t, e.p.expiry, order.OrderID, 1); disposition != "STALE" {
			t.Fatal("completed expiry must be a stale no-op")
		}
	}
	if oaExpiryBalance(t, e) != want || oaExpiryReceipt(t, e, a.AttemptID) != receipt ||
		e.count(`SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='RELEASE'`, order.OrderID) != 1 {
		t.Fatal("replay released twice or changed the terminal attempt receipt")
	}
}

func TestOrderAttemptExpiryFinalizedPickupAndCODKeepStock(t *testing.T) {
	for _, mode := range []string{"pay_at_pickup", "cash_on_delivery"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			a, order := oaExpiryCommitted(t, e, mode)
			before, receipt := oaExpiryBalance(t, e), oaExpiryReceipt(t, e, a.AttemptID)
			bcDue(t, e.p.bcHarness, order)
			for i := 0; i < 2; i++ {
				if disposition, retry := oaExpiryCall(t, e.p.expiry, order.OrderID, 1); disposition != "STALE" || retry != nil {
					t.Fatal("finalized offline reservation was treated as an unpaid hold")
				}
			}
			if oaExpiryBalance(t, e) != before || oaExpiryReceipt(t, e, a.AttemptID) != receipt ||
				e.count(`SELECT count(*) FROM inventory.ledger WHERE checkout_id=$1 AND kind='RELEASE'`, order.OrderID) != 0 ||
				e.count(`SELECT count(*) FROM checkout.events WHERE order_id=$1 AND action='checkout.expired'`, order.OrderID) != 0 ||
				e.count(`SELECT count(*) FROM inventory.reservations WHERE id=$1 AND state='COMMITTED'`, order.OrderID) != 1 {
				t.Fatal("offline stale expiry changed stock, reservation or terminal evidence")
			}
		})
	}
}

func TestOrderAttemptExpirySharesCanonicalOrderLock(t *testing.T) {
	e := oaExpiryIsolated(t)
	_, order := oaExpiryCommitted(t, e, "bank_transfer")
	holder, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	// Only the shared C2 order key is held, not the order row or any stock row.
	oaStateExec(t, holder, `SELECT pg_advisory_xact_lock(hashtextextended('checkout.order|'||$1::uuid::text||'|'||$2::uuid::text||'|'||$3::uuid::text,0))`, e.tenant(), e.store(), order.OrderID)
	contender, err := e.p.expiry.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, contender)
	oaStateExec(t, contender, `SET LOCAL lock_timeout='750ms'`)
	var disposition string
	err = contender.QueryRow(context.Background(), `SELECT disposition FROM checkout.expire_held($1::uuid,1)`, order.OrderID).Scan(&disposition)
	oaFenceCode(t, nil, err, "55P03")
	_ = contender.Rollback(context.Background())
	if err = holder.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if disposition, retry := oaExpiryCall(t, e.p.expiry, order.OrderID, 1); disposition != "NOT_DUE" || retry == nil {
		t.Fatal("released canonical order barrier did not permit the native transition")
	}
}

func TestOrderAttemptExpiryPreservesStorefrontAndExactAuthority(t *testing.T) {
	e := oaExpiryIsolated(t)
	legacy := e.p.hold
	bcDue(t, e.p.bcHarness, legacy)
	if disposition, _ := oaExpiryCall(t, e.p.expiry, legacy.OrderID, 1); disposition != "EXPIRED" {
		t.Fatal("existing storefront expiry behavior changed")
	}
	var exact bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT
	 EXISTS(SELECT 1 FROM inventory.ledger WHERE checkout_id=$1 AND kind='RELEASE' AND creator_kind='storefront'
	  AND create_attempt_id IS NULL AND buyer_session_id IS NOT NULL AND actor_kind='SYSTEM_EXPIRY')
	 AND EXISTS(SELECT 1 FROM checkout.events WHERE order_id=$1 AND action='checkout.expired' AND creator_kind='storefront'
	  AND create_attempt_id IS NULL AND session_id IS NOT NULL AND actor_kind='SYSTEM_EXPIRY')
	 AND has_function_privilege('commerce_expiry_worker','checkout.expire_held(uuid,bigint)','EXECUTE')
	 AND NOT has_function_privilege('commerce_worker','checkout.expire_held(uuid,bigint)','EXECUTE')`, legacy.OrderID).Scan(&exact); err != nil || !exact {
		t.Fatalf("legacy creator and exact worker ACL sentinel: %v", err)
	}
	for _, role := range []string{"commerce_runtime", "commerce_checkout_runtime", "commerce_claims_worker", "commerce_buyer_runtime"} {
		pool := miPool(t, e.p.f, role)
		var raw []byte
		err := pool.QueryRow(context.Background(), `SELECT to_jsonb(e) FROM checkout.expire_held($1::uuid,1) e`, legacy.OrderID).Scan(&raw)
		oaFenceCode(t, raw, err, "42501")
	}
}
