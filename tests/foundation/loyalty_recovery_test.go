package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/loyalty"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

func recoveryCall(a mtAdmin, method, order, key string) (int, map[string]any) {
	var body []byte
	if method == "POST" {
		body = []byte(`{}`)
	}
	w := a.call(method, "/orders/"+order+"/member-award-recovery", key, "application/json", body)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func recoveryWant(t *testing.T, a mtAdmin, method, order, key string, status int, field, value string) map[string]any {
	t.Helper()
	got, out := recoveryCall(a, method, order, key)
	if got != status || out[field] != value {
		t.Fatalf("%s: %d %v, want %d %s=%s", method, got, out, status, field, value)
	}
	return out
}

// Cause an actual LP18 failure between the successful precheck and the final placement transaction.
func recoveryMissed(t *testing.T, o *lpOrderEnv, member string, points int64) (order, originalKey string) {
	t.Helper()
	return recoveryMissedBody(t, o, o.body(lpAward(member, points)))
}

func recoveryMissedBody(t *testing.T, o *lpOrderEnv, body map[string]any) (order, originalKey string) {
	t.Helper()
	ctx := context.Background()
	o.manual.FaultBeforeFinal = func() error {
		_, err := o.s.f.owner.Exec(ctx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='customers:points'`, o.s.tenant, o.s.store, o.s.adminID)
		return err
	}
	defer func() { o.manual.FaultBeforeFinal = nil; o.e.grantCreator("customers:points") }()
	key := t04Key("original-award-missed")
	status, out := o.a.place(key, body)
	if status != 201 || lpAwardOf(t, out)["state"] != "not_awarded" || lpAwardOf(t, out)["reason"] != "forbidden" {
		t.Fatalf("original order: %d %v", status, out)
	}
	return out["order_id"].(string), key
}

func recoveryFacts(t *testing.T, o *lpOrderEnv, order, originalKey, member string, original map[string]any, balance int64, entries, receipts int) {
	t.Helper()
	if found, after := o.receipt(originalKey); !found || !reflect.DeepEqual(original, after) {
		t.Fatalf("original receipt changed: %v vs %v", original, after)
	}
	if stored, sum := o.s.balance(member); stored != balance || sum != balance {
		t.Fatalf("member balance=%d ledger sum=%d, want %d", stored, sum, balance)
	}
	if got := o.awardEntries(order); got != entries {
		t.Fatalf("canonical ledger entries=%d, want %d", got, entries)
	}
	if got := o.e.count(`SELECT count(*) FROM ops.command_results WHERE store_id=$1 AND operation='loyalty.order_award.recover' AND response->>'order_id'=$2`, o.s.store, order); got != receipts {
		t.Fatalf("recovery receipts=%d, want %d", got, receipts)
	}
}

func TestLoyaltyAwardRecovery(t *testing.T) {
	t.Run("lost HTTP response re-reads the original order", func(t *testing.T) {
		o := lpNewOrderEnv(t)
		m := o.s.mustMember("測試會員-回應中斷", nil, nil)
		order, originalKey := recoveryMissed(t, o, m.ID, 12)
		_, original := o.receipt(originalKey)
		key := t04Key("recovery-response-lost")
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			o.a.h.ServeHTTP(httptest.NewRecorder(), r)
			panic(http.ErrAbortHandler)
		}))
		defer server.Close()
		path := "/v1/admin/stores/" + o.s.store + "/orders/" + order + "/member-award-recovery"
		request, err := http.NewRequest("POST", server.URL+path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+o.s.admin)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", key)
		response, err := server.Client().Do(request)
		if response != nil {
			response.Body.Close()
		}
		if !errors.Is(err, io.EOF) || response != nil {
			t.Fatalf("client should lose the entire response: response=%v error=%v", response, err)
		}
		get := recoveryWant(t, o.a, "GET", order, "", 200, "state", "already_awarded")
		entries, err := o.s.orderEntries(o.s.admin, order)
		if err != nil || len(entries.Entries) != 1 || entries.Entries[0].ID != get["entry_id"] {
			t.Fatalf("ledger after EOF: %+v %v", entries, err)
		}
		replay := recoveryWant(t, o.a, "POST", order, key, 200, "state", "awarded")
		if replay["entry_id"] != get["entry_id"] {
			t.Fatalf("replay after EOF: %v vs %v", replay, get)
		}
		recoveryFacts(t, o, order, originalKey, m.ID, original, 12, 1, 1)
	})

	t.Run("concurrent keys and principals", func(t *testing.T) {
		o := lpNewOrderEnv(t)
		m := o.s.mustMember("測試會員-補登", nil, nil)
		order, originalKey := recoveryMissed(t, o, m.ID, 24)
		_, original := o.receipt(originalKey)
		before := recoveryWant(t, o.a, "GET", order, "", 200, "state", "eligible")
		if before["member_id"] != m.ID || before["points"] != float64(24) || len(before) != 4 {
			t.Fatalf("eligibility projection %v", before)
		}
		staff, principal := o.e.member("orders:read", "customers:read", "customers:points")
		firstKey := t04Key("recovery-staff-one")
		keys := []string{firstKey, t04Key("recovery-staff-two"), firstKey}
		admins := []mtAdmin{o.a, o.a.with(staff), o.a}
		actors := []string{o.s.adminID, principal, o.s.adminID}
		start := make(chan struct{})
		type response struct{ index, status int }
		done := make(chan response, len(admins))
		for i, a := range admins {
			go func() {
				<-start
				status, _ := recoveryCall(a, "POST", order, keys[i])
				done <- response{i, status}
			}()
		}
		close(start)
		winner := -1
		for range admins {
			r := <-done
			if r.status == 201 && winner == -1 {
				winner = r.index
			} else if r.status != 200 {
				t.Fatalf("concurrent recovery status %d", r.status)
			}
		}
		if winner < 0 {
			t.Fatal("no recovery created")
		}
		get := recoveryWant(t, o.a, "GET", order, "", 200, "state", "already_awarded")
		if get["member_id"] != m.ID || get["points"] != float64(24) || get["entry_id"] == nil || len(get) != 6 {
			t.Fatalf("readback: %v", get)
		}
		for i, a := range admins[:2] {
			replay := recoveryWant(t, a, "POST", order, keys[i], 200, "state", "awarded")
			if replay["entry_id"] != get["entry_id"] || replay["order_id"] != order || replay["member_id"] != m.ID || replay["points"] != float64(24) || len(replay) != 5 {
				t.Fatalf("replay %v vs GET %v", replay, get)
			}
		}
		entries, err := o.s.orderEntries(o.s.admin, order)
		if err != nil || len(entries.Entries) != 1 || entries.Entries[0].ID != get["entry_id"] {
			t.Fatalf("ledger readback: %+v %v", entries, err)
		}
		var actor string
		if err := o.s.f.owner.QueryRow(context.Background(), `SELECT actor_principal_id FROM loyalty.point_entries WHERE id=$1`, get["entry_id"]).Scan(&actor); err != nil || actor != actors[winner] {
			t.Fatalf("ledger actor=%q, want %q, error=%v", actor, actors[winner], err)
		}
		recoveryWant(t, o.a, "POST", randomUUID(), keys[0], 409, "code", "idempotency_conflict")
		mustExec(t, o.s.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='customers:points'`, o.s.tenant, o.s.store, principal)
		recoveryWant(t, admins[1], "POST", order, keys[1], 403, "code", "forbidden")
		recoveryWant(t, admins[1], "GET", order, "", 200, "state", "already_awarded")
		recoveryFacts(t, o, order, originalKey, m.ID, original, 24, 1, 2)
	})

	t.Run("original authority and refusal boundaries", func(t *testing.T) {
		o := lpNewOrderEnv(t)
		m := o.s.mustMember("測試會員-核查", nil, nil)
		order, key := recoveryMissed(t, o, m.ID, 15)
		_, original := o.receipt(key)
		readOnly, _ := o.e.member("orders:read", "customers:read")
		pointsOnly, _ := o.e.member("customers:points", "customers:read")
		ordersOnly, _ := o.e.member("orders:read")
		other := lpNewStore(t)
		foreign := o.a.with(other.principal("orders:read", "customers:read", "customers:points"))
		foreign.store = other.store
		for _, tc := range []struct {
			a             mtAdmin
			method, order string
			status        int
			code          string
		}{
			{o.a.with(""), "GET", order, 401, "unauthorized"},
			{o.a.with(readOnly), "POST", order, 403, "forbidden"},
			{o.a.with(pointsOnly), "POST", order, 403, "forbidden"},
			{o.a.with(ordersOnly), "GET", order, 403, "forbidden"},
			{foreign, "GET", order, 404, "not_found"},
			{foreign, "POST", order, 404, "not_found"},
			{o.a, "GET", randomUUID(), 404, "not_found"},
		} {
			requestKey := ""
			if tc.method == "POST" {
				requestKey = t04Key("recovery-refused")
			}
			recoveryWant(t, tc.a, tc.method, tc.order, requestKey, tc.status, "code", tc.code)
		}
		// A malformed history includes a second candidate receipt regardless of its amount.
		duplicate := t04Key("duplicate-original")
		mustExec(t, o.s.f.owner, `INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
			SELECT tenant_id,store_id,operation,$1,request_hash,response,principal_id FROM ops.command_results WHERE store_id=$2 AND idempotency_key=$3`, duplicate, o.s.store, key)
		recoveryWant(t, o.a, "GET", order, "", 200, "state", "review_required")
		recoveryWant(t, o.a, "POST", order, t04Key("recovery-duplicate"), 409, "code", "award_review_required")
		mustExec(t, o.s.f.owner, `DELETE FROM ops.command_results WHERE store_id=$1 AND idempotency_key=$2`, o.s.store, duplicate)
		if _, _, err := o.s.erase(o.s.admin, t04Key("recovery-erased"), m.ID); err != nil {
			t.Fatal(err)
		}
		recoveryWant(t, o.a, "POST", order, t04Key("recovery-erased"), 409, "code", "member_erased")
		recoveryFacts(t, o, order, key, m.ID, original, 0, 0, 0)

		otherMember := o.s.mustMember("測試會員-已贈", nil, nil)
		status, out := o.place(o.s.admin, t04Key("original-already-awarded"), lpAward(otherMember.ID, 7))
		if status != 201 || lpAwardOf(t, out)["state"] != "awarded" {
			t.Fatalf("awarded order: %d %v", status, out)
		}
		awardedOrder := out["order_id"].(string)
		recoveryWant(t, o.a, "GET", awardedOrder, "", 200, "state", "already_awarded")
		recoveryWant(t, o.a, "POST", awardedOrder, t04Key("recovery-already-awarded"), 409, "code", "award_already_recorded")
		noAward := t04Key("original-no-intent")
		status, out = o.place(o.s.admin, noAward, nil)
		if status != 201 {
			t.Fatalf("no award intent: %d %v", status, out)
		}
		historicalOrder := out["order_id"].(string)
		recoveryWant(t, o.a, "POST", historicalOrder, t04Key("recovery-no-intent"), 409, "code", "award_not_eligible")
		mustExec(t, o.s.f.owner, `DELETE FROM ops.command_results WHERE store_id=$1 AND idempotency_key=$2`, o.s.store, noAward)
		recoveryWant(t, o.a, "GET", historicalOrder, "", 200, "state", "review_required")
		recoveryWant(t, o.a, "POST", historicalOrder, t04Key("recovery-missing-receipt"), 409, "code", "award_review_required")
		recoveryWant(t, o.a, "POST", o.e.p.hold.OrderID, t04Key("recovery-storefront"), 409, "code", "award_not_eligible")

		// The pipeline produced an order, then failed before the definitive final receipt.
		o.manual.FaultBeforeFinal = func() error { return errors.New("synthetic placement transaction failure") }
		failedKey := t04Key("original-unknown")
		status, out = o.place(o.s.admin, failedKey, lpAward(otherMember.ID, 9))
		o.manual.FaultBeforeFinal = nil
		if status != 503 {
			t.Fatalf("faulted placement: %d %v", status, out)
		}
		var failedOrder string
		if err := o.s.f.owner.QueryRow(context.Background(), `SELECT id FROM checkout.orders WHERE store_id=$1 AND source='storefront' AND id<>$2`, o.s.store, o.e.p.hold.OrderID).Scan(&failedOrder); err != nil {
			t.Fatal(err)
		}
		recoveryWant(t, o.a, "POST", failedOrder, t04Key("recovery-unknown-placement"), 409, "code", "award_not_eligible")
		if found, _ := o.receipt(failedKey); found || o.awardEntries(failedOrder) != 0 {
			t.Fatal("an unresolved original command cannot authorize a recovery")
		}

		// A conflicting historical canonical entry must hold for investigation.
		m = o.s.mustMember("測試會員-衝突回執", nil, nil)
		order, key = recoveryMissed(t, o, m.ID, 16)
		_, original = o.receipt(key)
		mustExec(t, o.s.f.owner, `INSERT INTO loyalty.point_entries(tenant_id,store_id,member_id,delta,reason,order_id,actor_principal_id)
			VALUES($1,$2,$3,3,'manual_order_award',$4,$5)`, o.s.tenant, o.s.store, otherMember.ID, order, o.s.adminID)
		mustExec(t, o.s.f.owner, `UPDATE loyalty.members SET balance=balance+3,version=version+1 WHERE id=$1`, otherMember.ID)
		recoveryWant(t, o.a, "GET", order, "", 200, "state", "review_required")
		recoveryWant(t, o.a, "POST", order, t04Key("recovery-conflict"), 409, "code", "award_review_required")
		recoveryFacts(t, o, order, key, m.ID, original, 0, 1, 0)
	})

	t.Run("uncertain ledger including credits outside the display cap", func(t *testing.T) {
		o := lpNewOrderEnv(t)
		m := o.s.mustMember("測試會員-歷史", nil, nil)
		other := o.s.mustMember("測試會員-不同會員", nil, nil)
		o.s.mustAdjust(other.ID, 150, "測試期初")
		order, key := recoveryMissed(t, o, m.ID, 5)
		_, original := o.receipt(key)
		for i := range 101 {
			delta := int64(-1)
			if i == 100 {
				delta = 5
			}
			if _, _, err := o.s.adjust(o.s.admin, t04Key("recovery-lp10"), other.ID, delta, "測試關聯調整", &order); err != nil {
				t.Fatal(err)
			}
		}
		visible, err := o.s.orderEntries(o.s.admin, order)
		if err != nil || !visible.Truncated || len(visible.Entries) != 100 {
			t.Fatalf("LP19 cap: %+v %v", visible, err)
		}
		for _, entry := range visible.Entries {
			if entry.Delta >= 0 {
				t.Fatalf("the sole credit must be past the LP19 display cap: %+v", entry)
			}
		}
		recoveryWant(t, o.a, "GET", order, "", 200, "state", "review_required")
		recoveryWant(t, o.a, "POST", order, t04Key("recovery-linked-history"), 409, "code", "award_review_required")
		recoveryFacts(t, o, order, key, m.ID, original, 0, 0, 0)

		m = o.s.mustMember("測試會員-未關聯歷史", nil, nil)
		o.s.mustAdjust(m.ID, 10, "測試期初")
		order, key = recoveryMissed(t, o, m.ID, 4)
		_, original = o.receipt(key)
		if _, _, err := o.s.adjust(o.s.admin, t04Key("recovery-negative"), m.ID, -1, "測試扣點", &order); err != nil {
			t.Fatal(err)
		}
		recoveryWant(t, o.a, "GET", order, "", 200, "state", "eligible")
		o.s.mustAdjust(m.ID, 4, "測試手動入點")
		recoveryWant(t, o.a, "POST", order, t04Key("recovery-unlinked-history"), 409, "code", "award_review_required")
		recoveryFacts(t, o, order, key, m.ID, original, 13, 0, 0)
	})

	t.Run("rollback after entry and after balance", func(t *testing.T) {
		o := lpNewOrderEnv(t)
		m := o.s.mustMember("測試會員-交易回滾", nil, nil)
		order, originalKey := recoveryMissed(t, o, m.ID, 8)
		_, original := o.receipt(originalKey)
		key := t04Key("recovery-rollback")
		for _, fault := range []struct{ table, predicate string }{
			{"loyalty.members", fmt.Sprintf("id<>'%s' OR balance=0", m.ID)},
			{"ops.command_results", fmt.Sprintf("operation<>'loyalty.order_award.recover' OR idempotency_key<>'%s'", key)},
		} {
			func() {
				mustExec(t, o.s.f.owner, "ALTER TABLE "+fault.table+" ADD CONSTRAINT recovery_fixture_fault CHECK ("+fault.predicate+") NOT VALID")
				defer mustExec(t, o.s.f.owner, "ALTER TABLE "+fault.table+" DROP CONSTRAINT recovery_fixture_fault")
				recoveryWant(t, o.a, "POST", order, key, 503, "code", "retry_later")
				recoveryWant(t, o.a, "GET", order, "", 200, "state", "eligible")
				recoveryFacts(t, o, order, originalKey, m.ID, original, 0, 0, 0)
			}()
		}
		recoveryWant(t, o.a, "POST", order, key, 201, "state", "awarded")
		recoveryFacts(t, o, order, originalKey, m.ID, original, 8, 1, 1)
	})
}

func TestLoyaltyAwardRecoveryOrderLock(t *testing.T) {
	for _, first := range []string{"cancel", "recover", "adjust", "revoke"} {
		t.Run(first+" first", func(t *testing.T) {
			o := lpNewOrderEnv(t)
			m := o.s.mustMember("測試會員-訂單鎖", nil, nil)
			award := lpAward(m.ID, 11)
			body := o.body(award)
			if first == "cancel" || first == "recover" {
				// Native merchant cancellation accepts an unshipped, pending pay-at-pickup order.
				code, _, _ := o.e.service("cvs_711", "MANUAL", 0)
				o.e.cvsSettings(tcvAllChains, true, "20000", 20)
				options := o.a.options()
				i := slices.IndexFunc(options, func(option merchanttools.ManualOption) bool {
					return option.DeliveryCode == code && slices.Contains(option.PaymentModes, "pay_at_pickup")
				})
				if i < 0 {
					t.Fatal("no native pay-at-pickup option")
				}
				body = mtBody(o.sku, 2, options[i].OptionKey, "pay_at_pickup", mtCVS)
				body["member_award"] = award
			}
			order, originalKey := recoveryMissedBody(t, o, body)
			_, original := o.receipt(originalKey)
			key := t04Key("recovery-order-lock")
			adjustedMember := o.s.mustMember("測試會員-調整", nil, nil)
			staff, staffID := o.e.member("orders:read", "customers:points")
			permission := map[string]string{"cancel": "fulfillment:write", "recover": "orders:read", "adjust": "customers:points", "revoke": "orders:read"}[first]
			ready := make(chan uint32, 1)
			release := make(chan struct{})
			defer func() {
				if release != nil {
					close(release)
				}
			}()
			unblock := release
			committed := make(chan error, 1)
			ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			go func() {
				committed <- platform.WithScope(ctx, o.s.f.runtime, o.s.admin, o.s.store, permission, func(tx pgx.Tx, scope platform.Scope) error {
					var err error
					switch first {
					case "cancel":
						_, err = fulfillment.CancelOrder(ctx, tx, scope, o.s.admin, t04Key("cancel-before-recovery"), order, "CONFIRMED", "synthetic test")
					case "recover":
						_, _, err = loyalty.RecoverAward(ctx, tx, scope, o.s.admin, key, order)
					case "adjust":
						_, _, err = loyalty.Adjust(ctx, tx, scope, o.s.admin, t04Key("credit-before-recovery"), adjustedMember.ID, loyalty.AdjustInput{Delta: 2, Note: "測試補償", OrderID: &order})
					case "revoke":
						// The recovery read holds a shared order lock while the writer loses authority.
						_, err = tx.Exec(ctx, `SELECT loyalty.manual_order_award_recovery($1,$2,$3,false)`, lpHash(o.s.admin), o.s.store, order)
					}
					if err != nil {
						return err
					}
					ready <- tx.Conn().PgConn().PID()
					select {
					case <-unblock:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			}()
			var pid uint32
			select {
			case pid = <-ready:
			case err := <-committed:
				t.Fatalf("%s did not take the order lock: %v", first, err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			type response struct {
				status int
				body   map[string]any
			}
			second := make(chan response, 1)
			go func() {
				if first == "recover" {
					w := o.a.call("POST", "/orders/"+order+"/cancel", t04Key("cancel-after-recovery"), "application/json", []byte(`{"expected_state":"CONFIRMED","reason":"synthetic test"}`))
					var body map[string]any
					_ = json.Unmarshal(w.Body.Bytes(), &body)
					second <- response{w.Code, body}
					return
				}
				admin := o.a
				if first == "revoke" {
					admin = admin.with(staff)
				}
				status, body := recoveryCall(admin, "POST", order, key)
				second <- response{status, body}
			}()
			deadline := time.Now().Add(900 * time.Millisecond)
			for o.e.count(`SELECT count(*) FROM pg_stat_activity WHERE $1::integer=ANY(pg_blocking_pids(pid))`, int(pid)) == 0 {
				select {
				case r := <-second:
					t.Fatalf("competing request did not wait for the order lock: %d %v", r.status, r.body)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("no competing request blocked on the order lock")
				}
				time.Sleep(5 * time.Millisecond)
			}
			if first == "revoke" {
				mustExec(t, o.s.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='customers:points'`, o.s.tenant, o.s.store, staffID)
			}
			close(release)
			release = nil
			if err := <-committed; err != nil {
				t.Fatalf("first commit: %v", err)
			}
			r := <-second
			switch first {
			case "recover":
				if r.status != 200 || r.body["commercial_state"] != "CANCELLED" {
					t.Fatalf("cancel after recovery: %d %v", r.status, r.body)
				}
				get := recoveryWant(t, o.a, "GET", order, "", 200, "state", "already_awarded")
				replay := recoveryWant(t, o.a, "POST", order, key, 200, "state", "awarded")
				if get["entry_id"] != replay["entry_id"] {
					t.Fatalf("readback changed after cancellation: %v vs %v", get, replay)
				}
				newKey := recoveryWant(t, o.a, "POST", order, t04Key("recovery-after-cancellation"), 200, "state", "awarded")
				if !reflect.DeepEqual(replay, newKey) {
					t.Fatalf("cancelled-order recovery replay differs by key: %v vs %v", replay, newKey)
				}
				recoveryFacts(t, o, order, originalKey, m.ID, original, 11, 1, 2)
			case "cancel", "adjust", "revoke":
				code := map[string]string{"cancel": "award_not_eligible", "adjust": "award_review_required", "revoke": "forbidden"}[first]
				status := 409
				if first == "revoke" {
					status = 403
				}
				if r.status != status || r.body["code"] != code {
					t.Fatalf("recovery after %s: %d %v", first, r.status, r.body)
				}
				state := map[string]string{"cancel": "not_eligible", "adjust": "review_required", "revoke": "eligible"}[first]
				recoveryWant(t, o.a, "GET", order, "", 200, "state", state)
				recoveryFacts(t, o, order, originalKey, m.ID, original, 0, 0, 0)
			}
		})
	}
}

func TestLoyaltyAwardRecoveryACL(t *testing.T) {
	o := lpNewOrderEnv(t)
	m := o.s.mustMember("測試會員-鎖定權限", nil, nil)
	order, key := recoveryMissed(t, o, m.ID, 6)
	_, original := o.receipt(key)
	other := lpNewStore(t)
	ctx := context.Background()
	const definer = "loyalty.manual_order_award_recovery(bytea,uuid,uuid,boolean)"
	var name, settings string
	var securityDefiner, runtimeExec, publicExec, runtimeOrderWrite, runtimeLedgerWrite, privacyWriteReceipt bool
	err := o.s.f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(proowner),array_to_string(proconfig,','),prosecdef,
		has_function_privilege('commerce_runtime',oid,'EXECUTE'),has_function_privilege('public',oid,'EXECUTE'),
		has_any_column_privilege('commerce_runtime','checkout.orders','INSERT,UPDATE') OR has_table_privilege('commerce_runtime','checkout.orders','DELETE'),
		has_any_column_privilege('commerce_runtime','loyalty.point_entries','INSERT,UPDATE') OR has_table_privilege('commerce_runtime','loyalty.point_entries','DELETE'),
		has_table_privilege('commerce_privacy_writer','ops.command_results','INSERT,UPDATE')
		FROM pg_proc WHERE oid=$1::regprocedure`, definer).Scan(&name, &settings, &securityDefiner, &runtimeExec, &publicExec, &runtimeOrderWrite, &runtimeLedgerWrite, &privacyWriteReceipt)
	if err != nil || name != "commerce_privacy_writer" || settings != "search_path=pg_catalog" || !securityDefiner || !runtimeExec || publicExec || runtimeOrderWrite || runtimeLedgerWrite || privacyWriteReceipt {
		t.Fatalf("recovery ACL owner=%s settings=%s definer=%v runtimeExec=%v publicExec=%v runtimeOrderWrite=%v runtimeLedgerWrite=%v privacyReceiptWrite=%v err=%v",
			name, settings, securityDefiner, runtimeExec, publicExec, runtimeOrderWrite, runtimeLedgerWrite, privacyWriteReceipt, err)
	}
	const locker = "checkout.lock_member_award_order(bytea,uuid,uuid,text)"
	var grantees []string
	var privacyOrderWrite, privacyOrderCreatedAt bool
	err = o.s.f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner),array_to_string(p.proconfig,','),p.prosecdef,
		ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a
			WHERE a.grantee<>p.proowner AND a.privilege_type='EXECUTE' ORDER BY a.grantee),
		has_any_column_privilege('commerce_privacy_writer','checkout.orders','UPDATE'),
		has_column_privilege('commerce_privacy_writer','checkout.orders','created_at','SELECT')
		FROM pg_proc p WHERE p.oid=$1::regprocedure`, locker).Scan(&name, &settings, &securityDefiner, &grantees, &privacyOrderWrite, &privacyOrderCreatedAt)
	if err != nil || name != "commerce_checkout_writer" || settings != "search_path=pg_catalog" || !securityDefiner || !slices.Equal(grantees, []string{"commerce_privacy_writer"}) || privacyOrderWrite || privacyOrderCreatedAt {
		t.Fatalf("lock ACL owner=%s settings=%s definer=%v grantees=%v privacy order update=%v created_at read=%v err=%v",
			name, settings, securityDefiner, grantees, privacyOrderWrite, privacyOrderCreatedAt, err)
	}
	const callLocker = `SELECT source,commercial_state,created_at FROM checkout.lock_member_award_order($1,$2,$3,$4)`
	var denied any
	if err = o.s.f.runtime.QueryRow(ctx, `SELECT 1 FROM checkout.lock_member_award_order($1,$2,$3,'recover')`, lpHash(o.s.admin), o.s.store, order).Scan(&denied); sqlState(err) != "42501" {
		t.Fatalf("runtime invoked private checkout lock: %v", err)
	}
	foreignActor, foreignToken := lcPrincipal(t, o.s.f, other.tenant, []string{other.store}, "store:read", "orders:read", "customers:points")
	pointsToken, pointsActor := o.e.member("customers:points")
	for _, tc := range []struct {
		name, tenant, store, actor, token, purpose, denial string
	}{
		{"same store", o.s.tenant, o.s.store, o.s.adminID, o.s.admin, "recover", ""},
		{"foreign order", other.tenant, other.store, foreignActor, foreignToken, "recover", ""},
		{"points-only recovery", o.s.tenant, o.s.store, pointsActor, pointsToken, "recover", "PT403"},
		{"points-only adjustment", o.s.tenant, o.s.store, pointsActor, pointsToken, "adjust", ""},
		{"forged principal scope", o.s.tenant, o.s.store, randomUUID(), o.s.admin, "recover", "PT403"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := o.s.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, `SET LOCAL ROLE commerce_privacy_writer`); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),set_config('app.principal_id',$3,true)`, tc.tenant, tc.store, tc.actor); err != nil {
				t.Fatal(err)
			}
			var source, state *string
			var created *time.Time
			err = tx.QueryRow(ctx, callLocker, lpHash(tc.token), tc.store, order, tc.purpose).Scan(&source, &state, &created)
			if tc.denial != "" {
				if sqlState(err) != tc.denial {
					t.Fatalf("checkout lock authority state=%s err=%v, want %s", sqlState(err), err, tc.denial)
				}
				return
			}
			if tc.purpose == "adjust" {
				if err != nil || source != nil || state != nil || created != nil {
					t.Fatalf("LP10 order lock returned order facts or failed: %v %v %v %v", source, state, created, err)
				}
				return
			}
			var receipts int
			if countErr := tx.QueryRow(ctx, `SELECT count(*) FROM ops.command_results WHERE operation='merchanttools.order.manual' AND idempotency_key=$1`, key).Scan(&receipts); countErr != nil {
				t.Fatal(countErr)
			}
			if tc.store != o.s.store {
				if err != pgx.ErrNoRows || receipts != 0 {
					t.Fatalf("foreign scope saw lock or receipt: %v, %d, %v", source, receipts, err)
				}
				return
			}
			if err != nil || source == nil || *source != "merchant_manual" || state == nil || *state != "AWAITING_TRANSFER" || created == nil || receipts != 1 {
				t.Fatalf("same-store scoped lock and receipt: %v %v %v receipts=%d err=%v", source, state, created, receipts, err)
			}
			_, err = tx.Exec(ctx, `UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id=$1`, order)
			if sqlState(err) != "42501" {
				t.Fatalf("lock role must be unable to change the order; state=%s err=%v", sqlState(err), err)
			}
		})
	}
	recoveryWant(t, o.a, "GET", order, "", 200, "state", "eligible")
	recoveryFacts(t, o, order, key, m.ID, original, 0, 0, 0)
}
