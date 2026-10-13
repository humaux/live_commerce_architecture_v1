// Purpose: REAL_PG an unbound historical Begin stays UNKNOWN after the manual derivation secret changes.
// Depends on: lbuNew real placement, owner-only legacy-gap fixture, buyer/merchanttools reconfiguration and real merchant HTTP routes.
// Used by: foundation CI and focused TestOrderIntentLegacyUnboundSecretRotationReturnsUnknown; no local test execution.
// Invariants: I01/I02/I03/I06; unknown old bodies cannot adopt a new owner, falsely seal absence or duplicate an existing checkout.
package foundation_test

import (
	"context"
	"testing"
	"time"

	"livecommerce/internal/buyer"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/merchanttools"
)

func TestOrderIntentLegacyUnboundSecretRotationReturnsUnknown(t *testing.T) {
	for _, action := range []string{"place", "recover"} {
		t.Run(action, func(t *testing.T) {
			e := lbuNew(t)
			key := t04Key("oir-unbound")
			status, placed := e.post(e.token(), key, e.body(2, nil, "", false))
			if status != 201 {
				t.Fatalf("real legacy checkout fixture: status=%d code=%v", status, placed["code"])
			}
			original := placed["order_id"]
			if e.count(`SELECT count(*) FROM checkout.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='checkout.begin' AND order_id=$3`, e.tenant(), e.store(), original) != 1 {
				t.Fatal("legacy fixture has no exact durable Begin")
			}
			// Disclosed owner-only gap models old code lacking a canonical attempt or upper receipt. The real checkout/owner/hold remain.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tag, err := e.p.f.owner.Exec(ctx, `DELETE FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, e.tenant(), e.store(), key)
			if err != nil || tag.RowsAffected() != 1 {
				t.Fatal("legacy fixture did not remove exactly the admitted intent")
			}
			tag, err = e.p.f.owner.Exec(ctx, `DELETE FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3 AND operation IN ('merchanttools.order.manual','merchanttools.order.for_buyer','merchanttools.order.for_buyer.completed','merchanttools.order.attempt')`, e.tenant(), e.store(), key)
			if err != nil || tag.RowsAffected() != 4 {
				t.Fatal("legacy fixture did not remove every original upper/canonical receipt")
			}
			owners := e.count(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
			sessions := e.count(`SELECT count(*) FROM buyer.capability_sessions WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
			orders, held := e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()), e.reserved()
			links := e.count(`SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND order_id=$3`, e.tenant(), e.store(), original)
			issuer, err := buyer.New(e.p.a.issuer, 30*24*time.Hour)
			if err != nil {
				t.Fatal("rotated issuer construction failed")
			}
			rotated, err := merchanttools.NewManualOrders(e.p.f.runtime, issuer, e.p.a.runtime, e.svc, randomBytes(32))
			if err != nil {
				t.Fatal("rotated manual service construction failed")
			}
			forBuyer, err := merchanttools.NewForBuyer(rotated, e.plan)
			if err != nil {
				t.Fatal("rotated for-buyer service construction failed")
			}
			e.manual, e.fb = rotated, forBuyer
			e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: rotated, ForBuyer: forBuyer})
			changed := e.body(1, nil, "", false)
			var out map[string]any
			if action == "place" {
				status, out = e.post(e.token(), key, changed)
			} else {
				status, out = lcrRecover(t, e, e.token(), key, changed)
			}
			if status != 503 || out["code"] != "retry_later" || out["state"] != nil || out["order_id"] != nil {
				t.Fatalf("rotated unknown old body %s: status=%d code=%v state=%v", action, status, out["code"], out["state"])
			}
			if e.count(`SELECT count(*) FROM buyer.owners WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != owners || e.count(`SELECT count(*) FROM buyer.capability_sessions WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != sessions {
				t.Fatal("unknown legacy body registered a replacement buyer owner/session")
			}
			if e.count(`SELECT count(*) FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND idempotency_key=$3`, e.tenant(), e.store(), key) != 0 || e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation IN ('merchanttools.order.attempt','merchanttools.order.attempt.recovery','merchanttools.order.checkout.closed')`, e.tenant(), e.store()) != 0 {
				t.Fatal("unknown legacy body was admitted or falsely sealed as not_created")
			}
			if e.count(`SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != orders || e.reserved() != held || e.count(`SELECT count(*) FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND order_id=$3`, e.tenant(), e.store(), original) != links || e.plan.planned() != 0 {
				t.Fatal("unknown legacy body changed checkout/hold/link/DM")
			}
		})
	}
}
