// Purpose: FEAT-BUYER-RMA causal RED then REAL_PG acceptance through existing buyer and merchant HTTP handlers.
// Depends on: tcvNew, real paid/manual shipment/merchant returns paths and the assigned 0210 migration.
// Used by: GitHub Gates foundation catch-all shard; no local execution.
// Invariants: owner/store isolation, serialized line capacity, CAS/replay, immutable RMA state machine, no money movement.
package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBuyerReturnRequests(t *testing.T) {
	e := tcvNew(t, tcvOpts{stripe: true})
	e.r.startWorker(t)
	e.grantCreator("orders:read", "fulfillment:write", "inventory:write")
	ctx := context.Background()
	sku, wh := e.rtSKU()
	readOnly, _ := e.member("orders:read")
	writer, _ := e.member("fulfillment:write", "orders:read")
	orderPath := func(order string) string { return "/v1/buyer/orders/" + order + "/return-requests" }
	requestPath := func(id string) string { return "/v1/buyer/return-requests/" + id }
	merchantPath := func(id, action string) string {
		p := e.rtPath("/return-requests/" + id)
		if action != "" {
			p += "/" + action
		}
		return p
	}
	body := func(qty int) map[string]any {
		return map[string]any{"reason": "damaged", "note": "測試商品縫線需要確認", "lines": []any{map[string]any{"sku_id": sku, "warehouse_id": wh, "quantity": qty}}}
	}
	newOrder := func(t *testing.T, shipped bool) (*tcvBuyer, string) {
		t.Helper()
		b := e.newBuyer()
		order, _ := e.rtPaidOf(b)
		if shipped {
			e.rtShip(order)
		}
		return b, order
	}
	call := func(t *testing.T, b *tcvBuyer, method, path, key string, in any) bhResponse {
		t.Helper()
		return e.bh.request(t, method, path, b.cap.Token, key, in, nil)
	}
	create := func(t *testing.T, b *tcvBuyer, order, key string, qty int) map[string]any {
		t.Helper()
		out := bhRead[map[string]any](t, call(t, b, "POST", orderPath(order), key, body(qty)), 201)
		if out["status"] != "requested" || out["version"] != float64(1) || out["order_id"] != order || out["return_id"] != nil {
			t.Fatalf("request did not remain separate from merchant RMA: %v", out)
		}
		id, ok := out["id"].(string)
		if !ok || id == "" {
			t.Fatal("missing request id")
		}
		if n, ok := out["number"].(string); !ok || !strings.HasPrefix(n, "R-") {
			t.Fatal("missing R-number")
		}
		return out
	}
	merchant := func(t *testing.T, token, id, action, key string, version int, reason string) (int, map[string]any) {
		t.Helper()
		in := map[string]any{"expected_version": version}
		if action == "reject" {
			in["reason"] = reason
		}
		raw, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		st, out, _ := e.mcall(token, "POST", merchantPath(id, action), key, string(raw))
		return st, out
	}
	assertTimeline := func(t *testing.T, out map[string]any, statuses ...string) {
		t.Helper()
		events, ok := out["timeline"].([]any)
		if !ok || len(events) != len(statuses) {
			t.Fatalf("timeline length: got %v want %v", out["timeline"], statuses)
		}
		var previous time.Time
		for i, status := range statuses {
			ev, ok := events[i].(map[string]any)
			if !ok || ev["status"] != status {
				t.Fatalf("timeline[%d] status: %v", i, events[i])
			}
			at, err := time.Parse(time.RFC3339Nano, fmt.Sprint(ev["at"]))
			if err != nil || at.Before(previous) {
				t.Fatal("timeline must carry ordered authoritative timestamps")
			}
			previous = at
		}
	}

	t.Run("BR01 create read replay approve links existing RMA and never refunds", func(t *testing.T) {
		b, order := newOrder(t, true)
		money, refunds, ledger := e.pgPaymentFingerprint(), e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE store_id=$1`, e.store()), e.rtAll()
		first := create(t, b, order, "br-create-0001", 1)
		id := first["id"].(string)
		assertTimeline(t, first, "requested")
		if len(first["lines"].([]any)) != 1 {
			t.Fatal("selected line missing")
		}
		again := bhRead[map[string]any](t, call(t, b, "POST", orderPath(order), "br-create-0001", body(1)), 201)
		if !reflect.DeepEqual(first, again) {
			t.Fatal("create retry changed stored result")
		}
		bhError(t, call(t, b, "POST", orderPath(order), "br-create-0001", body(2)), 409, "conflict")
		if e.count(`SELECT count(*) FROM returns.rmas WHERE order_id=$1`, order) != 0 {
			t.Fatal("request registered an RMA before approval")
		}
		list := bhRead[map[string]any](t, call(t, b, "GET", "/v1/buyer/return-requests", "", nil), 200)
		if len(list["items"].([]any)) != 1 {
			t.Fatal("buyer list did not contain its request")
		}
		detail := bhRead[map[string]any](t, call(t, b, "GET", requestPath(id), "", nil), 200)
		if !reflect.DeepEqual(first, detail) {
			t.Fatal("persisted detail differs from create")
		}
		st, mlist, _ := e.mcall(e.token(), "GET", e.rtPath("/return-requests"), "", "")
		rtExpect(t, "merchant requests", st, mlist, 200, "")
		found := false
		for _, row := range mlist["items"].([]any) {
			if row.(map[string]any)["id"] == id {
				found = true
			}
		}
		if !found {
			t.Fatal("merchant cannot discover buyer request")
		}
		st, approved := merchant(t, writer, id, "approve", "br-approve-0001", 1, "")
		rtExpect(t, "approve", st, approved, 200, "")
		if approved["status"] != "approved" || approved["version"] != float64(2) {
			t.Fatal("approval status/CAS")
		}
		rma, ok := approved["return_id"].(string)
		if !ok || rma == "" {
			t.Fatal("approval did not link an RMA")
		}
		st, replay := merchant(t, writer, id, "approve", "br-approve-0001", 1, "")
		rtExpect(t, "approve replay", st, replay, 200, "")
		if !reflect.DeepEqual(approved, replay) {
			t.Fatal("approval retry changed stored result")
		}
		if e.count(`SELECT count(*) FROM returns.rmas WHERE order_id=$1`, order) != 1 {
			t.Fatal("approval created duplicate RMA")
		}
		st, rmas, _ := e.mcall(e.token(), "GET", e.rtPath("/orders/"+order+"/returns"), "", "")
		rtExpect(t, "legacy merchant parser shape", st, rmas, 200, "")
		rows := rmas["items"].([]any)
		if len(rows) != 1 || rows[0].(map[string]any)["id"] != rma || rows[0].(map[string]any)["state"] != "REGISTERED" {
			t.Fatal("existing RMA flow changed")
		}
		detail = bhRead[map[string]any](t, call(t, b, "GET", requestPath(id), "", nil), 200)
		if detail["return_state"] != "REGISTERED" || detail["return_id"] != rma {
			t.Fatal("buyer cannot see linked return state")
		}
		assertTimeline(t, detail, "requested", "approved")
		st, received := e.rtReceive(writer, rma, "br-receive-0001", 1, 1)
		rtExpect(t, "unchanged merchant receive", st, received, 200, "")
		detail = bhRead[map[string]any](t, call(t, b, "GET", requestPath(id), "", nil), 200)
		if detail["return_state"] != "RECEIVED" {
			t.Fatal("buyer linked state was stale")
		}
		if e.pgPaymentFingerprint() != money || e.count(`SELECT count(*) FROM payments.stripe_refunds WHERE store_id=$1`, e.store()) != refunds || e.rtAll() != ledger {
			t.Fatal("request/approve/receive moved money or stock")
		}
		for _, secret := range []string{b.cap.Token, b.cap.Scope.OwnerID, b.cap.Scope.SessionID, "principal_id", "tenant_id", "store_id"} {
			raw, _ := json.Marshal(detail)
			if strings.Contains(string(raw), secret) {
				t.Fatal("buyer detail disclosed private authority")
			}
		}
	})

	t.Run("BR02 strict enum note quantity line validation and no client authority", func(t *testing.T) {
		b, order := newOrder(t, true)
		for i, edit := range []func(map[string]any){
			func(m map[string]any) { m["reason"] = "invented_reason" },
			func(m map[string]any) { m["note"] = strings.Repeat("測", 1001) },
			func(m map[string]any) { m["lines"] = []any{} },
			func(m map[string]any) { m["lines"] = body(0)["lines"] },
			func(m map[string]any) { m["lines"] = body(-1)["lines"] },
			func(m map[string]any) { m["lines"] = []any{map[string]any{"sku_id": sku, "quantity": 1.5}} },
			func(m map[string]any) { m["lines"] = []any{body(1)["lines"].([]any)[0], body(1)["lines"].([]any)[0]} },
		} {
			in := body(1)
			edit(in)
			r := call(t, b, "POST", orderPath(order), fmt.Sprintf("br-invalid-%04d", i), in)
			if r.status < 400 || r.status >= 500 {
				t.Fatalf("invalid input %d was not refused: %d", i, r.status)
			}
		}
		bhError(t, call(t, b, "POST", orderPath(order), "br-overqty-0001", body(3)), 422, "exceeds_shipped")
		bad := body(1)
		bad["lines"] = []any{map[string]any{"sku_id": randomUUID(), "quantity": 1}}
		bhError(t, call(t, b, "POST", orderPath(order), "br-unknown-0001", bad), 422, "unknown_line")
		for _, field := range []string{"tenant_id", "store_id", "owner_id", "amount_minor", "photos"} {
			in := body(1)
			in[field] = "untrusted"
			bhError(t, call(t, b, "POST", orderPath(order), "br-authority-"+field, in), 400, "invalid_json")
		}
		// One stored request proves all refusals above left quantity/capacity untouched.
		create(t, b, order, "br-after-invalid", 2)
	})

	t.Run("BR03 own order only and tenant store permission fences", func(t *testing.T) {
		b, order := newOrder(t, true)
		first := create(t, b, order, "br-owned-0001", 1)
		id := first["id"].(string)
		other := e.newBuyer()
		bhError(t, call(t, other, "POST", orderPath(order), "br-other-owner", body(1)), 404, "not_found")
		bhError(t, call(t, other, "GET", requestPath(id), "", nil), 404, "not_found")
		bhError(t, call(t, other, "POST", requestPath(id)+"/withdraw", "br-other-withdraw", map[string]any{"expected_version": 1}), 404, "not_found")
		list := bhRead[map[string]any](t, call(t, other, "GET", "/v1/buyer/return-requests", "", nil), 200)
		if len(list["items"].([]any)) != 0 {
			t.Fatal("buyer list crossed owner")
		}
		for _, store := range []string{e.p.f.storeA2, e.p.f.storeB} {
			foreignOrigin := "https://br-" + store + ".example"
			tenant := e.p.f.tenantA
			token := e.p.f.tokens["a2"]
			if store == e.p.f.storeB {
				tenant = e.p.f.tenantB
				token = e.p.f.tokens["b"]
			}
			var principal string
			if err := e.p.f.owner.QueryRow(ctx, `SELECT principal_id::text FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND permission='store:read' ORDER BY principal_id LIMIT 1`, tenant, store).Scan(&principal); err != nil {
				t.Fatal(err)
			}
			for _, permission := range []string{"orders:read", "fulfillment:write"} {
				tag, err := e.p.f.owner.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
				 VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, tenant, store, principal, permission)
				if err != nil {
					t.Fatal(err)
				}
				if tag.RowsAffected() == 1 {
					t.Cleanup(func() {
						mustExec(t, e.p.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`, tenant, store, principal, permission)
					})
				}
			}
			bhPublish(t, e.p.bcHarness, foreignOrigin, tenant, store)
			foreign := mustIssue(t, e.p.cqHarness.service, store)
			for _, route := range []string{requestPath(id), orderPath(order)} {
				method, key, in := "GET", "", any(nil)
				if route == orderPath(order) {
					method, key, in = "POST", "br-foreign-"+store[:8], body(1)
				}
				r := e.bh.request(t, method, route, foreign.Token, key, in, func(r *http.Request) { r.Header.Set("X-Commerce-Storefront-Origin", foreignOrigin) })
				bhError(t, r, 404, "not_found")
			}
			st, out, _ := e.mcall(token, "GET", "/v1/admin/stores/"+store+"/return-requests/"+id, "", "")
			rtExpect(t, "foreign merchant detail", st, out, 404, "not_found")
			st, out, _ = e.mcall(token, "POST", "/v1/admin/stores/"+store+"/return-requests/"+id+"/approve", "br-foreign-approve-"+store[:8], `{"expected_version":1}`)
			rtExpect(t, "foreign merchant approval", st, out, 404, "not_found")
		}
		for _, action := range []string{"approve", "reject"} {
			st, out := merchant(t, readOnly, id, action, "br-permission-"+action, 1, "測試拒絕原因")
			rtExpect(t, "staff without fulfillment:write", st, out, 403, "forbidden")
		}
		st, detail, _ := e.mcall(readOnly, "GET", merchantPath(id, ""), "", "")
		rtExpect(t, "orders:read detail", st, detail, 200, "")
		if detail["status"] != "requested" {
			t.Fatal("refusals changed status")
		}
	})

	t.Run("BR04 reject and withdraw version conflicts retries free capacity", func(t *testing.T) {
		b, order := newOrder(t, true)
		first := create(t, b, order, "br-reject-create", 1)
		id := first["id"].(string)
		st, out := merchant(t, writer, id, "approve", "br-stale-approve", 2, "")
		rtExpect(t, "approval CAS", st, out, 409, "version_changed")
		st, out = merchant(t, writer, id, "reject", "br-empty-reject", 1, "")
		rtExpect(t, "reject needs reason", st, out, 422, "invalid_request")
		st, rejected := merchant(t, writer, id, "reject", "br-reject-0001", 1, "測試拒絕原因")
		rtExpect(t, "reject", st, rejected, 200, "")
		if rejected["status"] != "rejected" || rejected["rejection_reason"] != "測試拒絕原因" || rejected["version"] != float64(2) {
			t.Fatal("reject projection")
		}
		assertTimeline(t, rejected, "requested", "rejected")
		st, replay := merchant(t, writer, id, "reject", "br-reject-0001", 1, "測試拒絕原因")
		if st != 200 || !reflect.DeepEqual(rejected, replay) {
			t.Fatal("reject replay failed")
		}
		st, out = merchant(t, writer, id, "reject", "br-reject-0001", 1, "不同原因")
		rtExpect(t, "reject replay drift", st, out, 409, "conflict")
		st, out = merchant(t, writer, id, "approve", "br-rejected-approve", 2, "")
		rtExpect(t, "rejected is terminal", st, out, 409, "invalid_state")
		second := create(t, b, order, "br-withdraw-create", 2)
		secondID := second["id"].(string)
		bhError(t, call(t, b, "POST", requestPath(secondID)+"/withdraw", "br-withdraw-stale", map[string]any{"expected_version": 2}), 409, "version_changed")
		withdrawInput := map[string]any{"expected_version": 1}
		withdrawn := bhRead[map[string]any](t, call(t, b, "POST", requestPath(secondID)+"/withdraw", "br-withdraw-0001", withdrawInput), 200)
		if withdrawn["status"] != "withdrawn" || withdrawn["version"] != float64(2) {
			t.Fatal("withdraw projection")
		}
		assertTimeline(t, withdrawn, "requested", "withdrawn")
		if replay := bhRead[map[string]any](t, call(t, b, "POST", requestPath(secondID)+"/withdraw", "br-withdraw-0001", withdrawInput), 200); !reflect.DeepEqual(withdrawn, replay) {
			t.Fatal("withdraw replay failed")
		}
		create(t, b, order, "br-capacity-freed", 2)
		if e.count(`SELECT count(*) FROM returns.rmas WHERE order_id=$1`, order) != 0 {
			t.Fatal("reject/withdraw created merchant return")
		}
	})

	t.Run("BR05 exactly one open request per line and serialized capacity", func(t *testing.T) {
		b, order := newOrder(t, true)
		var wg sync.WaitGroup
		responses := make(chan bhResponse, 2)
		for _, key := range []string{"br-race-create-a", "br-race-create-b"} {
			wg.Add(1)
			go func(key string) { defer wg.Done(); responses <- call(t, b, "POST", orderPath(order), key, body(1)) }(key)
		}
		wg.Wait()
		close(responses)
		created, refused := 0, 0
		for r := range responses {
			if r.status == 201 {
				created++
			} else {
				bhError(t, r, 409, "request_open")
				refused++
			}
		}
		if created != 1 || refused != 1 {
			t.Fatalf("open-line race: %d created %d refused", created, refused)
		}
		// Existing REGISTERED and CLOSED quantities remain authoritative after request approval.
		b2, order2 := newOrder(t, true)
		first := create(t, b2, order2, "br-capacity-approve", 2)
		st, out := merchant(t, writer, first["id"].(string), "approve", "br-capacity-appr", 1, "")
		rtExpect(t, "approve all units", st, out, 200, "")
		bhError(t, call(t, b2, "POST", orderPath(order2), "br-capacity-again", body(1)), 422, "exceeds_shipped")
		b3, order3 := newOrder(t, true)
		st, out = e.rtRegister(writer, order3, "br-legacy-reg", 2)
		rtExpect(t, "existing merchant return", st, out, 201, "")
		bhError(t, call(t, b3, "POST", orderPath(order3), "br-legacy-capacity", body(1)), 422, "exceeds_shipped")
	})

	t.Run("BR06 trusted windows and unshipped refusal", func(t *testing.T) {
		b, unshipped := newOrder(t, false)
		bhError(t, call(t, b, "POST", orderPath(unshipped), "br-not-shipped", body(1)), 409, "not_shipped")
		// Owner fixture inserts a historical first shipment, preserving every append-only/state trigger.
		// No trigger is disabled and no test clock is injected into the production API.
		for _, hours := range []int{13 * 24, 14*24 + 1} {
			buyer, order := newOrder(t, false)
			tx, err := e.p.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, `INSERT INTO fulfillment.manual_shipment_versions
			 (tenant_id,store_id,owner_id,order_id,version,status,carrier_code,tracking_number,principal_id,recorded_at)
			 VALUES($1,$2,$3,$4,1,'SHIPPED','chunghwa_post','SYNTHETIC',$5,clock_timestamp()-$6*interval '1 hour')`, e.tenant(), e.store(), buyer.cap.Scope.OwnerID, order, e.p.f.principalA, hours)
			if err == nil {
				_, err = tx.Exec(ctx, `INSERT INTO fulfillment.manual_shipment_heads(tenant_id,store_id,owner_id,order_id,current_version,updated_at)
			 VALUES($1,$2,$3,$4,1,clock_timestamp()-$5*interval '1 hour')`, e.tenant(), e.store(), buyer.cap.Scope.OwnerID, order, hours)
			}
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE checkout.orders SET fulfillment_state='MERCHANT_SHIPPED' WHERE id=$1`, order)
			}
			if err != nil {
				_ = tx.Rollback(ctx)
				t.Fatalf("historical shipment fixture: %v", err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			r := call(t, buyer, "POST", orderPath(order), fmt.Sprintf("br-window-%d", hours), body(1))
			if hours < 14*24 {
				bhRead[map[string]any](t, r, 201)
			} else {
				bhError(t, r, 409, "window_closed")
				// The buyer window must never constrain the existing merchant-create path.
				st, out := e.rtRegister(writer, order, "br-merchant-no-window", 1)
				rtExpect(t, "merchant remains unrestricted by buyer deadline", st, out, 201, "")
			}
		}
		// A collected COD order is returnable under the existing returns contract.
		st, out := e.hcodSettings(0, true, 20000, 0, "black_cat")
		rtExpect(t, "COD fixture settings", st, out, 200, "")
		for _, hours := range []int{6 * 24, 7*24 + 1} {
			buyer := e.newBuyer()
			placed, err := e.hcodPlace(buyer)
			if err != nil {
				t.Fatal(err)
			}
			e.rtShip(placed.OrderID)
			st, out, _ := e.record(writer, placed.OrderID, fmt.Sprintf("br-collect-%d", hours), "PENDING", "collected")
			rtExpect(t, "COD collection", st, out, 200, "")
			// Fixture-only age of the server's already-recorded collection fact; API never accepts timestamps.
			mustExec(t, e.p.f.owner, `UPDATE checkout.orders SET collected_at=clock_timestamp()-$2*interval '1 hour' WHERE id=$1`, placed.OrderID, hours)
			r := call(t, buyer, "POST", orderPath(placed.OrderID), fmt.Sprintf("br-col-window-%d", hours), body(1))
			if hours < 7*24 {
				bhRead[map[string]any](t, r, 201)
			} else {
				bhError(t, r, 409, "window_closed")
			}
		}
	})

	t.Run("BR07 revoked and view-only sessions never gain request authority", func(t *testing.T) {
		b, order := newOrder(t, true)
		first := create(t, b, order, "br-auth-create", 1)
		id := first["id"].(string)
		if err := e.p.cqHarness.service.Revoke(ctx, b.cap.Token, e.store()); err != nil {
			t.Fatal(err)
		}
		bhError(t, call(t, b, "GET", requestPath(id), "", nil), 401, "unauthorized")
		bhError(t, call(t, b, "POST", orderPath(order), "br-auth-create", body(1)), 401, "unauthorized")
		viewBuyer, viewOrder := newOrder(t, true)
		viewToken := randomToken()
		mustExec(t, e.p.f.owner, `SELECT buyer.issue_order_capability($1::uuid,$2::uuid,$3::uuid,sha256($4::bytea),3600::bigint)`, e.store(), viewBuyer.cap.Scope.OwnerID, viewOrder, []byte(viewToken))
		bhError(t, e.bh.request(t, "POST", orderPath(viewOrder), viewToken, "br-view-mutation", body(1), nil), 403, "forbidden")
	})

	t.Run("BR08 failed approval rolls back and concurrent decisions have one winner", func(t *testing.T) {
		b, order := newOrder(t, true)
		first := create(t, b, order, "br-rollback-create", 2)
		id := first["id"].(string)
		st, out := e.rtRegister(writer, order, "br-rollback-existing", 2)
		rtExpect(t, "merchant registered before approval", st, out, 201, "")
		st, out = merchant(t, writer, id, "approve", "br-rollback-appr", 1, "")
		rtExpect(t, "approval preserves existing quantity rule", st, out, 422, "exceeds_shipped")
		detail := bhRead[map[string]any](t, call(t, b, "GET", requestPath(id), "", nil), 200)
		if detail["status"] != "requested" || detail["version"] != float64(1) || detail["return_id"] != nil {
			t.Fatal("failed approval committed partial state")
		}
		if e.count(`SELECT count(*) FROM returns.rmas WHERE order_id=$1`, order) != 1 {
			t.Fatal("failed approval created another RMA")
		}
		b2, order2 := newOrder(t, true)
		second := create(t, b2, order2, "br-decision-create", 1)
		secondID := second["id"].(string)
		type answer struct {
			status int
			out    map[string]any
		}
		answers := make(chan answer, 2)
		var wg sync.WaitGroup
		for _, action := range []string{"approve", "reject"} {
			wg.Add(1)
			go func(action string) {
				defer wg.Done()
				st, out := merchant(t, writer, secondID, action, "br-decision-"+action, 1, "測試拒絕原因")
				answers <- answer{st, out}
			}(action)
		}
		wg.Wait()
		close(answers)
		successes, conflicts := 0, 0
		for a := range answers {
			if a.status == 200 {
				successes++
			} else {
				rtExpect(t, "decision CAS race", a.status, a.out, 409, "version_changed")
				conflicts++
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatal("concurrent decisions both committed or neither did")
		}
		detail = bhRead[map[string]any](t, call(t, b2, "GET", requestPath(secondID), "", nil), 200)
		wantRMAs := 0
		if detail["status"] == "approved" {
			wantRMAs = 1
		}
		if e.count(`SELECT count(*) FROM returns.rmas WHERE order_id=$1`, order2) != wantRMAs {
			t.Fatal("decision status and linked RMA did not commit atomically")
		}
	})
}

func TestBuyerReturnRequestTableACL(t *testing.T) {
	f := fixture(t)
	for _, table := range []string{"returns.buyer_requests", "returns.buyer_request_lines"} {
		t.Run(table, func(t *testing.T) {
			var exists, rls, forced bool
			err := f.owner.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL,
			 coalesce((SELECT relrowsecurity FROM pg_class WHERE oid=to_regclass($1)),false),
			 coalesce((SELECT relforcerowsecurity FROM pg_class WHERE oid=to_regclass($1)),false)`, table).Scan(&exists, &rls, &forced)
			if err != nil {
				t.Fatal(err)
			}
			if !exists || !rls || !forced {
				t.Fatalf("buyer request relation missing forced RLS: %s exists=%v rls=%v force=%v", table, exists, rls, forced)
			}
			for _, role := range []string{"commerce_runtime", "commerce_checkout_runtime", "commerce_buyer_runtime", "commerce_buyer_issuer"} {
				var reachable bool
				err := f.owner.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1 AND
				 (has_table_privilege(oid,$2,'SELECT') OR has_table_privilege(oid,$2,'INSERT') OR has_table_privilege(oid,$2,'UPDATE') OR has_table_privilege(oid,$2,'DELETE')))`, role, table).Scan(&reachable)
				if err != nil {
					t.Fatal(err)
				}
				if reachable {
					t.Fatalf("runtime %s can access %s outside a scoped definer", role, table)
				}
			}
		})
	}
}
