// Purpose: prove an unresolved money intent is owned by a stable principal/store, not a browser session or bundle.
// Depends on: real merchant HTTP, checkout Begin receipts/holds and the existing scoped partial-commit fault seam.
// Used by: REAL_PG CI red/green gates; no browser or local test execution is substituted for that authority.
// Invariants: a fresh key cannot duplicate a pending order across re-login; other principals/stores remain isolated.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"livecommerce/internal/buyer"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/merchanttools"
)

func TestOrderIntentAuthorityPendingAcrossSessions(t *testing.T) {
	e := lbuNew(t)
	beforeOrders := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
	beforeHeld := e.reserved()
	key := t04Key("intent-pending")
	body := e.body(2, nil, "", false) // No bundle: the server, not the drawer's buyer guard, owns this intent.
	lcrPartialKill(t, e, key, body, "merchanttools.order.manual")
	digest := sha256.Sum256([]byte("mt|begin|" + key))
	beginKey := fmt.Sprintf("mt-%x", digest[:16])
	var original string
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT order_id::text FROM checkout.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='checkout.begin' AND idempotency_key=$3`, e.tenant(), e.store(), beginKey).Scan(&original); err != nil {
		t.Fatal("partial checkout receipt is missing")
	}
	assertPending := func() {
		t.Helper()
		if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+1 || e.reserved() != beforeHeld+2 {
			t.Fatal("a new key duplicated the unresolved checkout order or its inventory hold")
		}
		if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND source='storefront'`, e.tenant(), e.store(), original) != 1 || e.orders() != 0 || e.plan.planned() != 0 {
			t.Fatal("partial fixture did not remain at the actual committed Begin boundary")
		}
		if e.count(`SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND order_id=$3`, e.tenant(), e.store(), original) != 0 || e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3 AND operation IN ('merchanttools.order.manual','merchanttools.order.for_buyer','merchanttools.order.for_buyer.completed')`, e.tenant(), e.store(), key) != 0 {
			t.Fatal("incomplete intent acquired an upper pipeline receipt or buyer link")
		}
	}
	assertPending()
	status, out := e.post(e.token(), t04Key("intent-new-key"), body)
	if status != 409 || out["code"] != "attempt_pending" || out["order_id"] != nil {
		t.Fatalf("same principal/store new key must refuse a pending intent: status=%d code=%v", status, out["code"])
	}
	assertPending()

	// Disclosed owner-only session fixture models a real logout/login: a different bearer, the same principal.
	freshToken := oiaFreshSession(t, e, e.p.f.principalA)
	oldHash := sha256.Sum256([]byte(e.token()))
	mustExec(t, e.p.f.owner, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE principal_id=$1 AND token_hash=$2`, e.p.f.principalA, oldHash[:])
	if status, _ := e.post(e.token(), t04Key("intent-old-session"), body); status != 401 {
		t.Fatalf("revoked fixture session still authenticates: status=%d", status)
	}
	status, out = e.post(freshToken, t04Key("intent-relogin-key"), body)
	if status != 409 || out["code"] != "attempt_pending" || out["order_id"] != nil {
		t.Fatalf("re-login lost stable pending authority: status=%d code=%v", status, out["code"])
	}
	assertPending()

	// A different principal in this store has an independent intent; the original pending checkout stays untouched.
	otherToken := e.member()
	status, other := e.post(otherToken, t04Key("intent-other-principal"), e.body(1, nil, "", false))
	if status != 201 || lbuStr(other, "order_id") == "" || other["order_id"] == original {
		t.Fatalf("pending authority crossed principal boundary: status=%d code=%v", status, other["code"])
	}
	if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+2 || e.orders() != 1 || e.reserved() != beforeHeld+3 {
		t.Fatal("independent principal did not create exactly its own order/hold")
	}

	// The same principal belongs to a second synthetic store through ordinary membership/grants, not a forged scope.
	otherStore := lbuNew(t)
	otherBefore := otherStore.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, otherStore.tenant(), otherStore.store())
	otherHeld := otherStore.reserved()
	mustExec(t, e.p.f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, otherStore.tenant(), e.p.f.principalA)
	mustExec(t, e.p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) SELECT $1,$2,$3,p FROM unnest(ARRAY['store:read','inventory:reserve','catalog:read','orders:read']) p`, otherStore.tenant(), otherStore.store(), e.p.f.principalA)
	status, separate := otherStore.post(freshToken, t04Key("intent-other-store"), otherStore.body(1, nil, "", false))
	if status != 201 || lbuStr(separate, "order_id") == "" || separate["order_id"] == original {
		t.Fatalf("pending authority crossed store boundary: status=%d code=%v", status, separate["code"])
	}
	if otherStore.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, otherStore.tenant(), otherStore.store()) != otherBefore+1 || otherStore.orders() != 1 || otherStore.reserved() != otherHeld+1 {
		t.Fatal("independent store did not create exactly its own order/hold")
	}
	if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+2 || e.reserved() != beforeHeld+3 {
		t.Fatal("other store changed the original store's orders/holds")
	}

	status, replay := e.post(freshToken, key, body)
	if status != 201 || replay["order_id"] != original {
		t.Fatalf("same principal's original key could not complete the exact checkout after re-login: status=%d code=%v", status, replay["code"])
	}
	if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+2 || e.orders() != 2 || e.reserved() != beforeHeld+3 {
		t.Fatal("authoritative original-key completion duplicated an order or hold")
	}
}

func TestOrderIntentAuthoritySurvivesDerivationSecretRotation(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "Created"
		if partial {
			name = "BeginPartial"
		}
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			beforeOrders := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
			beforeHeld := e.reserved()
			key := t04Key("intent-secret-rotate")
			body := mnrBody(e, 2)
			if partial {
				mnrPartialKill(t, e, key, body)
			} else if status, _ := e.admin(e.token()).place(key, body); status != 201 {
				t.Fatalf("real manual placement fixture failed: status=%d", status)
			}
			digest := sha256.Sum256([]byte("mt|begin|" + key))
			beginKey := fmt.Sprintf("mt-%x", digest[:16])
			var original string
			if err := e.p.f.owner.QueryRow(context.Background(), `SELECT order_id::text FROM checkout.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='checkout.begin' AND idempotency_key=$3`, e.tenant(), e.store(), beginKey).Scan(&original); err != nil {
				t.Fatal("secret-rotation fixture has no exact original Begin receipt")
			}
			if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+1 || e.reserved() != beforeHeld+2 {
				t.Fatal("secret-rotation fixture did not create exactly one checkout/hold")
			}
			// Real process reconfiguration: same pools/issuer/checkout, freshly generated derivation secret.
			// No owner ID or receipt is fabricated; recovery must find the originally persisted authority.
			issuer, err := buyer.New(e.p.a.issuer, 30*24*time.Hour)
			if err != nil {
				t.Fatal("rotated service issuer construction failed")
			}
			rotated, err := merchanttools.NewManualOrders(e.p.f.runtime, issuer, e.p.a.runtime, e.svc, randomBytes(32))
			if err != nil {
				t.Fatal("rotated manual service construction failed")
			}
			e.manual = rotated
			e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: rotated})
			status, recovered := mnrRecover(t, e, e.token(), key, body)
			if partial {
				if status != 503 || recovered["code"] != "retry_later" || recovered["order_id"] != nil {
					t.Fatalf("secret rotation falsely sealed the original partial checkout: status=%d state=%v code=%v", status, recovered["state"], recovered["code"])
				}
				status, refused := e.admin(e.token()).place(t04Key("intent-rotated-new-key"), body)
				if status != 409 || refused["code"] != "attempt_pending" || refused["order_id"] != nil {
					t.Fatalf("rotated service allowed a fresh key beside the original partial checkout: status=%d code=%v", status, refused["code"])
				}
				if e.orders() != 0 || e.count(`SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND order_id=$3`, e.tenant(), e.store(), original) != 0 {
					t.Fatal("partial rotation fixture acquired manual source or a buyer link before explicit replay")
				}
			} else if status != 200 || recovered["state"] != "created" || recovered["order_id"] != original {
				t.Fatalf("secret rotation lost a fully committed original order: status=%d state=%v code=%v", status, recovered["state"], recovered["code"])
			}
			if e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3 AND operation='merchanttools.order.checkout.closed'`, e.tenant(), e.store(), key) != 0 || e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+1 || e.reserved() != beforeHeld+2 {
				t.Fatal("secret rotation wrote an absence seal or a duplicate order/hold")
			}
			status, replay := e.admin(e.token()).place(key, body)
			wantStatus := 200
			if partial {
				wantStatus = 201
			}
			if status != wantStatus || replay["order_id"] != original {
				t.Fatalf("rotated original-key replay failed to preserve checkout identity: status=%d code=%v", status, replay["code"])
			}
			status, recovered = mnrRecover(t, e, e.token(), key, body)
			if status != 200 || recovered["state"] != "created" || recovered["order_id"] != original {
				t.Fatal("rotated original-key completion did not recover as the exact created order")
			}
			if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+1 || e.orders() != 1 || e.reserved() != beforeHeld+2 || e.count(`SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND order_id=$3`, e.tenant(), e.store(), original) != 1 {
				t.Fatal("rotated replay duplicated order, hold, source or buyer link")
			}
			if e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='merchanttools.order.manual' AND idempotency_key=$3`, e.tenant(), e.store(), key) != 1 || e.count(`SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='order.manual_created'`, e.tenant(), e.store()) != 1 {
				t.Fatal("rotated replay lost or duplicated the exact manual receipt/audit")
			}
		})
	}
}

// oiaFreshSession changes only authentication material outside every product transaction callback.
// Its bounded owner fixture transaction has a deferred rollback even when an outer-test assertion fails.
func oiaFreshSession(t *testing.T, e *lbuEnv, principal string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	token := randomToken()
	tx, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal("fresh session fixture could not acquire its transaction")
	}
	defer tx.Rollback(ctx)
	if err = insertSession(ctx, tx, token, principal, "merchant", time.Now().Add(time.Hour), nil); err != nil {
		t.Fatal("fresh session fixture insert failed")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal("fresh session fixture commit failed")
	}
	return token
}
