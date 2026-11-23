package foundation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

func TestConversationManualOrder(t *testing.T) {
	e := lbuNew(t)
	e.grantCreator("customers:points")
	keys, err := inbox.LoadKeyring(func(n string) string {
		switch n {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return miKeyID
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return `{"keys":[{"id":"` + miKeyID + `","key_base64":"` + base64.StdEncoding.EncodeToString(randomBytes(32)) + `"}]}`
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound, err := inbox.NewService(keys)
	if err != nil {
		t.Fatal(err)
	}
	e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: e.manual, ForBuyer: e.fb, Inbox: inbound})
	ctx := context.Background()
	var closed bool
	if err := e.p.f.owner.QueryRow(ctx, `SELECT c.relrowsecurity AND c.relforcerowsecurity
		AND NOT has_table_privilege('commerce_buyer_runtime', c.oid, 'SELECT')
		AND NOT has_function_privilege('commerce_buyer_runtime','fulfillment.finish_manual_conversation_order(bytea,uuid,bytea)','EXECUTE')
		AND NOT has_function_privilege('commerce_runtime','claims.lock_manual_order_bundles(uuid,uuid,uuid[])','EXECUTE')
		FROM pg_class c WHERE c.oid='inbox.manual_order_sources'::regclass`).Scan(&closed); err != nil || !closed {
		t.Fatalf("0229 private provenance ACL = %t err=%v", closed, err)
	}
	admin := e.admin(e.token())
	members := &lpStore{t: t, f: e.p.f, tenant: e.tenant(), store: e.store(), adminID: e.p.f.principalA, admin: e.token()}
	member := members.mustMember("Synthetic member differs from recipient", nil, nil)
	read := func(token, path, key string) (int, map[string]any) {
		t.Helper()
		w := e.admin(token).call(http.MethodGet, path, key, "", nil)
		out := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	contextFor := func(conversation string) map[string]any {
		t.Helper()
		status, got := e.get(e.token(), "?conversation_id="+conversation)
		source, ok := got["order_context"].(map[string]any)
		if status != 200 || !ok || source["state"] != "ready" || source["conversation_id"] != conversation {
			t.Fatalf("A15 = %d %v", status, got)
		}
		return map[string]any{"conversation_id": conversation, "bundle_ids": source["bundle_ids"], "revision": source["revision"]}
	}
	expect := func(status int, got map[string]any, want int, code string) {
		t.Helper()
		if status != want || code != "" && got["code"] != code {
			t.Fatalf("response = %d %v, want %d %q", status, got, want, code)
		}
	}
	plain := lcConversation(t, e.p.f, e.tenant(), e.store(), "page")
	mustExec(t, e.p.f.owner, "UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp() WHERE conversation_id=$1", plain)
	source := contextFor(plain)
	orderBody := mtBody(e.money, 1, e.homeKey, "bank_transfer", mtHome)
	orderBody["source_context"] = source
	orderBody["member_award"] = lpAward(member.ID, 17)
	bodyKey := t04Key("plain-conversation")
	status, got := e.admin(e.member()).place(bodyKey, orderBody)
	expect(status, got, 403, "forbidden")
	foreign := lcConversation(t, e.p.f, e.p.f.tenantB, e.p.f.storeB, "page")
	orderBody["source_context"] = map[string]any{"conversation_id": foreign, "bundle_ids": []string{}, "revision": source["revision"]}
	status, got = admin.place(bodyKey, orderBody)
	expect(status, got, 404, "not_found")
	sameTenantStore := randomUUID()
	mustExec(t, e.p.f.owner, "INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,$2,'Synthetic neighbouring store','TWD')", e.tenant(), sameTenantStore)
	other := lcConversation(t, e.p.f, e.tenant(), sameTenantStore, "page")
	orderBody["source_context"] = map[string]any{"conversation_id": other, "bundle_ids": []string{}, "revision": source["revision"]}
	status, got = admin.place(bodyKey, orderBody)
	expect(status, got, 404, "not_found")
	orderBody["source_context"] = source
	orderBody["delivery"] = mtBody(e.money, 1, "12345678-1234-1234-1234-123456789012|TW|unconfigured", "bank_transfer", mtHome)["delivery"]
	status, got = admin.place(t04Key("missing-delivery"), orderBody)
	expect(status, got, 422, "invalid_request")
	orderBody["delivery"] = mtBody(e.money, 1, e.homeKey, "bank_transfer", mtHome)["delivery"]
	e.manual.FaultBeforeFinal = func() error { return errors.New("fixture interruption after Begin") }
	status, got = admin.place(bodyKey, orderBody)
	if status < 500 {
		t.Fatalf("lost result = %d %v", status, got)
	}
	e.manual.FaultBeforeFinal = nil
	status, got = read(e.token(), "/orders/manual/receipt", bodyKey)
	expect(status, got, 200, "")
	if got["state"] != "pending" {
		t.Fatalf("pending = %v", got)
	}
	second := e.member("inbox:read", "customers:points")
	status, got = e.admin(second).place(t04Key("different-plain-key"), orderBody)
	expect(status, got, 409, "source_order_pending")
	delete(orderBody, "source_context")
	status, got = admin.place(bodyKey, orderBody)
	expect(status, got, 409, "idempotency_conflict")
	orderBody["source_context"] = source
	status, got = admin.place(bodyKey, orderBody)
	expect(status, got, 201, "")
	order := got["order_id"].(string)
	if got["live_price"] != "not_applied" || lpAwardOf(t, got)["state"] != "awarded" ||
		got["order_origin"].(map[string]any)["conversation_id"] != plain {
		t.Fatalf("ordinary thread receipt = %v", got)
	}
	status, got = admin.place(bodyKey, orderBody)
	expect(status, got, 200, "")
	if got["order_id"] != order {
		t.Fatalf("exact replay = %v", got)
	}
	status, got = admin.place(t04Key("same-revision-key"), orderBody)
	expect(status, got, 409, "source_already_ordered")
	status, got = read(e.token(), "/orders/manual/receipt", bodyKey)
	expect(status, got, 200, "")
	saved := got["receipt"].(map[string]any)
	if got["state"] != "committed" || saved["order_id"] != order || saved["buyer_link"] != nil {
		t.Fatalf("readback = %v", got)
	}
	status, got = read(second, "/orders/manual/receipt", bodyKey)
	expect(status, got, 200, "")
	if got["state"] != "absent" {
		t.Fatalf("foreign principal receipt = %v", got)
	}
	status, got = read(e.token(), "/inbox/buyer-panel?conversation_id="+plain, "")
	if status != 200 {
		probe := platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
			var out []byte
			return tx.QueryRow(ctx, "SELECT inbox.buyer_panel($1::uuid,NULL)", plain).Scan(&out)
		})
		t.Logf("A13 SQL discriminator: %v", probe)
	}
	expect(status, got, 200, "")
	if rows, ok := got["orders"].([]any); !ok || len(rows) != 1 || rows[0].(map[string]any)["order_id"] != order {
		t.Fatalf("A13 ordinary thread = %v", got)
	}
	if n := e.count("SELECT count(*) FROM inventory.reservations WHERE store_id=$1 AND id=$2", e.store(), order); n != 1 {
		t.Fatalf("ordinary holds = %d", n)
	}
	if n := e.count("SELECT count(*) FROM loyalty.point_entries WHERE store_id=$1 AND order_id=$2 AND reason='manual_order_award'", e.store(), order); n != 1 {
		t.Fatalf("ordinary awards = %d", n)
	}
	var recipient string
	if err := e.p.f.owner.QueryRow(ctx, "SELECT snapshot->'destination'->>'recipient_name' FROM checkout.orders WHERE id=$1", order).Scan(&recipient); err != nil || recipient != orderBody["customer"].(map[string]any)["name"] {
		t.Fatalf("destination separated from member = %q err=%v", recipient, err)
	}
	_, _, bundle, claimed := e.scenario()
	orderBody["source_context"] = contextFor(claimed)
	if ids := orderBody["source_context"].(map[string]any)["bundle_ids"].([]any); len(ids) != 1 || ids[0] != bundle {
		t.Fatalf("claim source = %v", ids)
	}
	liveKey := t04Key("live-conversation")
	e.manual.FaultBeforeContextBegin = func() error { return errors.New("fixture interruption before Begin") }
	status, got = admin.place(liveKey, orderBody)
	e.manual.FaultBeforeContextBegin = nil
	if status < 500 {
		t.Fatalf("claim attempt did not stop before Begin: %d %v", status, got)
	}
	var reservation string
	if err := e.p.f.owner.QueryRow(ctx, `UPDATE inbox.order_for_buyer SET updated_at=clock_timestamp()-interval '30 minutes'
		WHERE store_id=$1 AND bundle_id=$2 AND state='pending' RETURNING request_id`, e.store(), bundle).Scan(&reservation); err != nil {
		t.Fatal(err)
	}
	err = platform.WithScope(ctx, e.p.f.runtime, e.token(), e.store(), "inventory:reserve", func(tx pgx.Tx, _ platform.Scope) error {
		return claims.ReleaseForBuyer(ctx, tx, reservation)
	})
	if err == nil {
		t.Fatal("legacy release accepted a contextual reservation")
	}
	status, got = e.post(e.token(), t04Key("aged-claim-legacy"), e.body(1, []string{bundle}, claimed, false))
	expect(status, got, 409, "bundle_already_ordered")
	status, got = admin.place(liveKey, orderBody)
	expect(status, got, 201, "")
	if got["live_price"] != "applied" || got["order_origin"].(map[string]any)["conversation_id"] != claimed || e.plan.planned() != 0 {
		t.Fatalf("claim receipt or DM side effect = %v DM=%d", got, e.plan.planned())
	}
	status, got = e.post(e.token(), t04Key("claimed-legacy"), e.body(1, []string{bundle}, claimed, false))
	expect(status, got, 409, "bundle_already_ordered")
	status, got = e.get(e.token(), "?conversation_id="+claimed)
	expect(status, got, 200, "")
	if got["order_context"].(map[string]any)["state"] != "existing_order" {
		t.Fatalf("claimed re-entry = %v", got)
	}
	mustExec(t, e.p.f.owner, "DELETE FROM social.conversations WHERE id=$1", plain)
	orderBody["source_context"] = source
	status, got = admin.place(bodyKey, orderBody)
	expect(status, got, 200, "")
	if got["order_origin"] != nil || got["order_id"] != order {
		t.Fatalf("erased conversation replay = %v", got)
	}
}
