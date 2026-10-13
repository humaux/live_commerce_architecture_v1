// Purpose: prove an unresolved money intent is owned by a stable principal/store, not a browser session or bundle.
// Depends on: real merchant HTTP, checkout Begin receipts/holds and the existing scoped partial-commit fault seam.
// Used by: REAL_PG CI red/green gates; no browser or local test execution is substituted for that authority.
// Invariants: a fresh key cannot duplicate a pending order across re-login; RLS and erasure retain the money fence.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
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

// Schema/privacy acceptance reads PostgreSQL's effective privileges and real scoped rows, never a DDL source scan.
// All transaction callbacks return errors: t.Fatal/Goexit inside WithScope would bypass its normal rollback path.
func TestOrderIntentAuthoritySchemaPrivacyAndScope(t *testing.T) {
	e := lbuNew(t)
	var owner string
	var enabled, forced bool
	oiaRead(t, e, `SELECT pg_get_userbyid(relowner),relrowsecurity,relforcerowsecurity FROM pg_class WHERE oid='ops.order_intents'::regclass`, nil, &owner, &enabled, &forced)
	if owner != "commerce_checkout_writer" || !enabled || !forced {
		t.Fatal("intent authority must retain checkout ownership and forced RLS")
	}
	var columns []string
	oiaRead(t, e, `SELECT ARRAY(SELECT attname::text FROM pg_attribute WHERE attrelid='ops.order_intents'::regclass AND attnum>0 AND NOT attisdropped ORDER BY attname)`, nil, &columns)
	lcSameSet(t, "intent columns", columns, []string{"tenant_id", "store_id", "principal_id", "idempotency_key", "operation", "request_hash", "frozen_request", "buyer_owner_id", "begin_key", "state", "order_id", "created_at", "updated_at"})
	var foreignKeys []string
	oiaRead(t, e, `SELECT ARRAY(SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='ops.order_intents'::regclass AND contype='f' ORDER BY oid)`, nil, &foreignKeys)
	lcSameSet(t, "intent scoped foreign keys", foreignKeys, []string{
		"FOREIGN KEY (tenant_id, store_id) REFERENCES control.stores(tenant_id, id)",
		"FOREIGN KEY (tenant_id, store_id, buyer_owner_id) REFERENCES buyer.owners(tenant_id, store_id, id)",
	})
	var pendingIndex bool
	oiaRead(t, e, `SELECT EXISTS(SELECT 1 FROM pg_index x
 WHERE x.indrelid='ops.order_intents'::regclass AND x.indisunique AND x.indisvalid AND x.indnkeyatts=3
 AND ARRAY(SELECT a.attname::text FROM unnest(x.indkey::smallint[]) WITH ORDINALITY k(attnum,n)
  JOIN pg_attribute a ON a.attrelid=x.indrelid AND a.attnum=k.attnum WHERE k.n<=x.indnkeyatts ORDER BY k.n)=ARRAY['tenant_id','store_id','principal_id']
 AND pg_get_expr(x.indpred,x.indrelid)='(state = ''pending''::text)')`, nil, &pendingIndex)
	if !pendingIndex {
		t.Fatal("intent authority has no unique pending principal/store fence")
	}

	// Column-only rights must not accidentally become broad table writes or reveal private keys to erasure.
	for _, role := range []string{"commerce_runtime", "commerce_privacy_writer"} {
		var tableRights []string
		oiaRead(t, e, `SELECT ARRAY(SELECT p FROM unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER']) p
 WHERE has_table_privilege($1,'ops.order_intents',p) ORDER BY p)`, []any{role}, &tableRights)
		var want []string
		if role == "commerce_runtime" {
			want = []string{"SELECT", "INSERT"}
		}
		lcSameSet(t, role+" intent table rights", tableRights, want)
		for _, permission := range []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"} {
			var granted []string
			oiaRead(t, e, `SELECT ARRAY(SELECT attname::text FROM pg_attribute
 WHERE attrelid='ops.order_intents'::regclass AND attnum>0 AND NOT attisdropped
 AND has_column_privilege($1,'ops.order_intents',attname,$2) ORDER BY attname)`, []any{role, permission}, &granted)
			want = nil
			if role == "commerce_runtime" {
				switch permission {
				case "SELECT", "INSERT":
					want = columns
				case "UPDATE":
					want = []string{"state", "order_id", "frozen_request", "updated_at"}
				}
			} else {
				switch permission {
				case "SELECT":
					want = []string{"tenant_id", "store_id", "buyer_owner_id", "state"}
				case "UPDATE":
					want = []string{"frozen_request"}
				}
			}
			lcSameSet(t, role+" intent "+permission+" columns", granted, want)
		}
	}
	for _, fn := range []struct {
		signature, owner, volatility string
		execute                      []string
	}{
		{"fulfillment.order_intent_begin(bytea,uuid,text)", "commerce_checkout_writer", "s", []string{"commerce_checkout_writer", "commerce_runtime"}},
		{"fulfillment.mark_order_intent_manual(bytea,uuid,text,uuid,bytea)", "commerce_checkout_writer", "v", []string{"commerce_checkout_writer", "commerce_runtime"}},
		{"fulfillment.order_pay_link_valid(bytea,uuid,uuid,bytea)", "commerce_checkout_writer", "v", []string{"commerce_checkout_writer", "commerce_runtime"}},
		{"fulfillment.order_intent_legacy_owner(bytea,uuid,text,text)", "commerce_checkout_writer", "s", []string{"commerce_checkout_writer", "commerce_runtime"}},
		{"customers.erase_order_intent_payload(uuid,uuid,uuid)", "commerce_privacy_writer", "v", []string{"commerce_privacy_writer"}},
	} {
		var actualOwner, volatility string
		var definer, documented bool
		var config, execute []string
		oiaRead(t, e, `SELECT pg_get_userbyid(proowner),provolatile::text,prosecdef,proconfig,
 obj_description(oid,'pg_proc') IS NOT NULL,
 ARRAY(SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee)::text END
  FROM aclexplode(coalesce(proacl,acldefault('f',proowner))) a WHERE a.privilege_type='EXECUTE' ORDER BY a.grantee)
 FROM pg_proc WHERE oid=$1::regprocedure`, []any{fn.signature}, &actualOwner, &volatility, &definer, &config, &documented, &execute)
		if actualOwner != fn.owner || volatility != fn.volatility || !definer || !documented {
			t.Fatalf("intent definer metadata changed: %s", fn.signature)
		}
		lcSameSet(t, fn.signature+" search_path", config, []string{"search_path=pg_catalog"})
		lcSameSet(t, fn.signature+" execute", execute, fn.execute)
	}

	beforeOrders, beforeHeld := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()), e.reserved()
	key, body := t04Key("intent-erasure-pending"), mnrBody(e, 2)
	mnrPartialKill(t, e, key, body) // Real Begin committed; the manual receipt/source/link transaction was killed.
	var buyerOwner, original string
	var privatePayload bool
	name := body["customer"].(map[string]any)["name"].(string)
	email := body["customer"].(map[string]any)["email"].(string)
	oiaRead(t, e, `SELECT i.buyer_owner_id::text,r.order_id::text,
 i.state='pending' AND i.order_id IS NULL AND i.frozen_request IS NOT NULL
 AND position($5 IN i.frozen_request::text)>0 AND position($6 IN i.frozen_request::text)>0
 FROM ops.order_intents i JOIN checkout.command_results r
 ON r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.owner_id=i.buyer_owner_id
 AND r.operation='checkout.begin' AND r.idempotency_key=i.begin_key
 WHERE i.tenant_id=$1 AND i.store_id=$2 AND i.principal_id=$3 AND i.idempotency_key=$4`,
		[]any{e.tenant(), e.store(), e.p.f.principalA, key, name, email}, &buyerOwner, &original, &privatePayload)
	if !privatePayload || e.orders() != 0 || e.reserved() != beforeHeld+2 || e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+1 {
		t.Fatal("erasure fixture lacks one real private pending Begin order/hold")
	}

	// The other store commits through the public manual HTTP pipeline; settlement must atomically discard its payload.
	other := lbuNew(t)
	otherBefore, otherHeld := other.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, other.tenant(), other.store()), other.reserved()
	otherKey := t04Key("intent-erasure-created")
	status, created := other.admin(other.token()).place(otherKey, mnrBody(other, 1))
	createdOrder := lbuStr(created, "order_id")
	if status != 201 || createdOrder == "" {
		t.Fatalf("settled intent fixture placement failed: status=%d code=%v", status, created["code"])
	}
	assertSettled := func() {
		t.Helper()
		var settled bool
		oiaRead(t, other, `SELECT count(*)=1 FROM ops.order_intents i JOIN checkout.orders o
 ON o.tenant_id=i.tenant_id AND o.store_id=i.store_id AND o.id=i.order_id AND o.owner_id=i.buyer_owner_id
 JOIN ops.command_results r ON r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.principal_id=i.principal_id
 AND r.idempotency_key=i.idempotency_key AND r.operation='merchanttools.order.manual' AND r.response->>'order_id'=o.id::text
 WHERE i.tenant_id=$1 AND i.store_id=$2 AND i.principal_id=$3 AND i.idempotency_key=$4 AND i.state='created'
 AND i.frozen_request IS NULL AND o.id=$5 AND o.source='merchant_manual'`,
			[]any{other.tenant(), other.store(), other.p.f.principalA, otherKey, createdOrder}, &settled)
		if !settled || other.orders() != 1 || other.reserved() != otherHeld+1 || other.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, other.tenant(), other.store()) != otherBefore+1 {
			t.Fatal("settled intent retained a private payload or lost its exact completed order/hold")
		}
	}
	assertSettled()
	otherPrincipal := e.member()
	for _, scope := range []struct {
		e          *lbuEnv
		token, key string
		want       int
	}{
		{e, e.token(), key, 1},
		{e, otherPrincipal, key, 0},
		{other, other.token(), key, 0},
		{other, other.token(), otherKey, 1},
		{e, e.token(), otherKey, 0},
	} {
		if got := oiaVisibleIntent(t, scope.e, scope.token, scope.key); got != scope.want {
			t.Fatalf("intent runtime RLS crossed principal/store scope: got=%d want=%d", got, scope.want)
		}
	}

	// Calls customers.apply_erasure (customers-billing-v1 §5): trusted privacy fixture, real erasure, exact owner scope.
	if err := oiaEraseIntentOwner(e, buyerOwner); err != nil {
		t.Fatal("actual owner erasure failed")
	}
	assertErasedPending := func() {
		t.Helper()
		var retained bool
		oiaRead(t, e, `SELECT count(*)=1 FROM ops.order_intents i JOIN checkout.command_results r
 ON r.tenant_id=i.tenant_id AND r.store_id=i.store_id AND r.owner_id=i.buyer_owner_id
 AND r.operation='checkout.begin' AND r.idempotency_key=i.begin_key
 JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.id=r.order_id AND o.owner_id=r.owner_id
 JOIN buyer.owners b ON b.tenant_id=i.tenant_id AND b.store_id=i.store_id AND b.id=i.buyer_owner_id
 WHERE i.tenant_id=$1 AND i.store_id=$2 AND i.principal_id=$3 AND i.idempotency_key=$4
 AND i.state='pending' AND i.frozen_request IS NULL AND i.order_id IS NULL AND NOT b.active
 AND o.id=$5 AND o.source='storefront'`, []any{e.tenant(), e.store(), e.p.f.principalA, key, original}, &retained)
		if !retained || e.orders() != 0 || e.reserved() != beforeHeld+2 || e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != beforeOrders+1 {
			t.Fatal("erasure failed to remove the private input while retaining the exact unresolved order/hold")
		}
		if e.count(`SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND order_id=$3`, e.tenant(), e.store(), original) != 0 || mnrDefinitiveCount(e) != 0 {
			t.Fatal("erasure incorrectly completed or sealed an unresolved checkout")
		}
	}
	assertErasedPending()
	status, refused := e.admin(e.token()).place(t04Key("intent-erased-new-key"), body)
	if status != 409 || refused["code"] != "attempt_pending" || refused["order_id"] != nil {
		t.Fatalf("erasure released pending authority for a new key: status=%d code=%v", status, refused["code"])
	}
	w := e.admin(e.token()).call("POST", "/orders/attempt/recover", "", "application/json", []byte(`{}`))
	var recovered map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &recovered); err != nil {
		t.Fatal("generic erased-intent recovery returned invalid JSON")
	}
	if w.Code != 503 || recovered["code"] != "retry_later" || len(recovered) != 1 {
		t.Fatalf("generic recovery recreated erased input or disclosed private state: status=%d code=%v fields=%d", w.Code, recovered["code"], len(recovered))
	}
	assertErasedPending()
	assertSettled()
	if oiaVisibleIntent(t, e, e.token(), key) != 1 || oiaVisibleIntent(t, e, otherPrincipal, key) != 0 {
		t.Fatal("erasure changed pending authority visibility across principals")
	}
}

// oiaRead runs bounded fixture reads outside every product transaction; its failures cannot strand WithScope.
func oiaRead(t *testing.T, e *lbuEnv, query string, args []any, values ...any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.p.f.owner.QueryRow(ctx, query, args...).Scan(values...); err != nil {
		t.Fatal("intent authority fixture/schema read failed")
	}
}

func oiaVisibleIntent(t *testing.T, e *lbuEnv, token, key string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var count int
	err := platform.WithScope(ctx, e.p.f.runtime, token, e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM ops.order_intents WHERE idempotency_key=$1`, key).Scan(&count)
	})
	if err != nil {
		t.Fatal("intent authority runtime scope read failed")
	}
	return count
}

// A deferred rollback covers every fixture error; scope values are trusted fixture facts, not client input.
func oiaEraseIntentOwner(e *lbuEnv, owner string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, e.tenant(), e.store()); err != nil {
		return err
	}
	// Calls customers.apply_erasure (customers-billing-v1 §5); it alone owns private-payload removal.
	if _, err = tx.Exec(ctx, `SELECT customers.apply_erasure($1,$2,$3)`, e.tenant(), e.store(), owner); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
