//go:build browser

// Purpose: seed LC-U2b2 A13/A14 browser data through signed intake, private-reply Finish, and real A16 checkout.
// Depends on: lbEnv, merchanttools, checkout, merchantorders, customers, and their authenticated PostgreSQL definers.
// Used by: TestBrowserInboxUIRealChain; contains synthetic MOCK data only, never a product endpoint or an RLS bypass.
// Invariants: I01, I05, I09; owner reads below discover fixture IDs only; all read assertions use runtime scope.
package foundation_test

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/customers"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inbox"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

type inboxCustomerLinkFixture struct {
	Conversation    string `json:"conversation"`
	Session         string `json:"session"`
	CustomerA       string `json:"customer_a"`
	CustomerB       string `json:"customer_b"`
	NameA           string `json:"name_a"`
	NameB           string `json:"name_b"`
	ConfirmedNumber string `json:"confirmed_number"`
	CancelledNumber string `json:"cancelled_number"`
	ClaimTotalMinor int64  `json:"claim_total_minor"`
}

// inboxPagerCustomers exceeds the BuyerPanel customer page (limit 25) so a second page exists.
const inboxPagerCustomers = 30

// seedInboxCustomerLink prepares two real claimed orders and their customer projection without fabricating domain rows.
// The inherited lbEnv store is USD; A13 has no currency field and its frozen UI contract formats the supplied minor value as TWD.
func seedInboxCustomerLink(t *testing.T, e *lbEnv) inboxCustomerLinkFixture {
	t.Helper()
	ctx, h, f := context.Background(), e.h, e.h.f
	out := inboxCustomerLinkFixture{Session: e.session, NameA: "LCU2B2 Synthetic Alpha", NameB: "LCU2B2 Synthetic Beta", ClaimTotalMinor: 2100}
	_, token := lcPerson(t, f, f.tenantA, f.storeA1, "inventory:reserve", "catalog:read", "live:manage", "live:read", "inbox:read", "inbox:reply", "customers:read", "orders:read", "integration:read", "integration:manage", "payments:refund", "fulfillment:write")
	// Fixture authority only, as dsSetup does; delivery commands below still use the real merchant runtime role.
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		VALUES($1,$2,$3,'integration:manage'),($1,$2,$3,'integration:read') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, f.principalA)

	// Signed inbound and actual successful meta.private_reply Finish establish bundle_peers (live-console-v1 I09).
	const peer = "900007320005"
	out.Conversation = e.postDM(t, peer, "LCU2B2 SYNTHETIC customer-link fixture", time.Now())
	e.g.mu.Lock()
	e.g.privateRecipient = peer
	e.g.mu.Unlock()
	defer func() { e.g.mu.Lock(); e.g.privateRecipient = ""; e.g.mu.Unlock() }()
	sku := h.stock.skus[1].ID
	h.livePriceOffer(t, e.session, "B2", sku, 5, 700)
	var bundles [2]string
	for i, text := range []string{"B2+2", "B2+1"} {
		r := e.planReply(t, false, mciDigits(14), text)
		if r.op == "" || r.bundleID == "" {
			t.Fatal("LCU2B2 private reply was not planned")
		}
		e.run(t, r.op)
		e.awaitOp(t, r.op, "SUCCEEDED", 10*time.Second, "completed")
		bundles[i] = r.bundleID
	}
	if bundles[0] == bundles[1] {
		t.Fatal("LCU2B2 requires distinct real bundles")
	}

	// Reuse the existing store's catalog/market/warehouse; configure delivery through fulfillment.SetService/SetAllocation.
	delivery := fulfillment.ServiceInput{MarketID: h.market.ID, Country: "TW", Code: "lc_inbox_" + t04Tag(), PolicyVersion: 1,
		NameHans: "测试配送", NameHant: "測試配送", NameEN: "Synthetic delivery", DeliveryKind: "home", Mode: "MANUAL", Enabled: true, Visible: true}
	dsPolicy(t, h.cqHarness, delivery, 0, 0, true)
	if _, err := dsSet(h.cqHarness, t04Key("inu-service"), delivery); err != nil {
		t.Fatal(err)
	}
	if _, err := daSet(h.cqHarness, t04Key("inu-allocation"), fulfillment.AllocationInput{MarketID: h.market.ID, Country: "TW", Code: delivery.Code, ExpectedServiceVersion: 1, WarehouseIDs: []string{h.stock.warehouse.ID}}); err != nil {
		t.Fatal(err)
	}
	pool, err := platform.OpenCheckoutPool(ctx, bcRole(t, f, "commerce_checkout_runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	service := bcService(t, pool)
	adapter := &tcvEnv{t: t, p: psHarness{bcHarness: bcHarness{cqHarness: h.cqHarness, expiry: waOpen(t, f, waExpiry, platform.WorkerExpiry)}}, svc: service}
	manual := mtManualOrders(t, adapter)
	forBuyer, err := merchanttools.NewForBuyer(manual, e.svc)
	if err != nil {
		t.Fatal(err)
	}
	admin := mtAdmin{t: t, h: httpapi.NewHandler(f.runtime, httpapi.Options{ManualOrders: manual, ForBuyer: forBuyer}), store: f.storeA1, token: token}
	seq := 0
	call := func(method, path string, body any, want int) map[string]any {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		key, ctype := "", "" // GET carries no key/body: cvsRoute rejects a key on unkeyed routes (422)
		if body != nil {
			seq++ // one key per command: a reused key with a different payload is an idempotency conflict
			key, ctype = t04Key("inu-seed-"+strconv.Itoa(seq)), "application/json"
		}
		w := admin.call(method, path, key, ctype, raw)
		if w.Code != want {
			body := w.Body.String() // a bounded error envelope (code/details); success payloads never reach here
			if len(body) > 300 {
				body = body[:300]
			}
			t.Fatalf("LCU2B2 fixture %s %s: status %d %s, want %d", method, path, w.Code, body, want)
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal("LCU2B2 fixture invalid JSON")
		}
		return result
	}
	// PR #41 review P1: >25 active customers so INU10 clicks the real "Next customers"/"First page" pager. Seeded through
	// the real merchant CSV import BEFORE the orders: the list is newest-activity first, so A and B stay on page one.
	_, importer := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "customers:read", "customers:write", "customers:privacy")
	var csv strings.Builder
	csv.WriteString("顧客編號,姓名,手機,電子郵件,同意行銷,備註\n")
	for i := 1; i <= inboxPagerCustomers; i++ {
		csv.WriteString("LCU2B2-PG-" + strconv.Itoa(i) + ",LCU2B2 Synthetic Page " + strconv.Itoa(i) + ",09001" + strconv.Itoa(10000+i) + ",,否,\n")
	}
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM customers.import_profiles WHERE tenant_id=$1 AND owner_id IN (SELECT internal_id FROM migrationimport.external_ids WHERE tenant_id=$1 AND external_id LIKE 'LCU2B2-PG-%')`,
			`DELETE FROM migrationimport.external_ids WHERE tenant_id=$1 AND external_id LIKE 'LCU2B2-PG-%'`, `DELETE FROM migrationimport.batches WHERE tenant_id=$1`} {
			mustExec(t, f.owner, q, f.tenantA)
		}
	})
	imp := mtAdmin{t: t, h: admin.h, store: f.storeA1, token: importer}
	if w := imp.call("POST", "/imports/customers/commit?expected_apply_rows="+strconv.Itoa(inboxPagerCustomers), "", "text/csv", []byte(csv.String())); w.Code != 200 {
		t.Fatalf("LCU2B2 pager import: status %d", w.Code)
	}
	// Owner fixture time travel (like cofAge): the imports sort OLDER than B's expired order (aged 73 h below), so page one
	// is A, B and the newest imports, and the remaining imports form page two.
	mustExec(t, f.owner, `UPDATE customers.import_profiles SET imported_at=clock_timestamp()-interval '100 hours'
		WHERE tenant_id=$1 AND owner_id IN (SELECT internal_id FROM migrationimport.external_ids WHERE tenant_id=$1 AND external_id LIKE 'LCU2B2-PG-%')`, f.tenantA)
	settings := call("GET", "/bank-transfer-settings", nil, 200)
	call("PUT", "/bank-transfer-settings", map[string]any{"expected_version": settings["version"], "enabled": true, "allow_cvs": false,
		"bank_name": "SYNTHETIC BANK", "branch": "MOCK", "account_name": "SYNTHETIC SHOP", "account_number": "0000000000", "window_hours": 72}, 200)
	options, err := manual.Options(ctx, token, f.storeA1)
	if err != nil {
		t.Fatal(err)
	}
	optionKey := ""
	for _, option := range options {
		if option.DeliveryCode == delivery.Code && strings.Contains(strings.Join(option.PaymentModes, ","), "bank_transfer") {
			optionKey = option.OptionKey
		}
	}
	if optionKey == "" {
		t.Fatal("LCU2B2 fixture lacks configured bank transfer option")
	}
	for i, name := range []string{out.NameA, out.NameB} {
		body := mtBody(sku, 2-i, optionKey, "bank_transfer", map[string]any{"home_address": map[string]any{"region": "台北市", "city": "中正區", "postal_code": "100", "line1": "SYNTHETIC FIXTURE ADDRESS", "line2": ""}})
		body["customer"] = map[string]any{"name": name, "phone": "0900000000", "email": ""}
		body["for"] = map[string]any{"bundle_ids": []string{bundles[i]}, "conversation_id": out.Conversation}
		body["send_payment_link"] = false
		placed := call("POST", "/orders/for-buyer", body, 201)
		order, ok := placed["order_id"].(string)
		if !ok || placed["live_price"] != "applied" {
			t.Fatal("LCU2B2 fixture did not record a real live-price order")
		}
		var customer string
		if err := f.owner.QueryRow(ctx, `SELECT owner_id::text FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, f.tenantA, f.storeA1, order).Scan(&customer); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			call("POST", "/orders/"+order+"/bank-transfer/confirm", map[string]any{}, 200)
			out.CustomerA, out.ConfirmedNumber = customer, "LC-"+strings.ToUpper(strings.ReplaceAll(order, "-", ""))
		} else {
			// An AWAITING_TRANSFER bank-transfer order is not merchant-cancellable (0162: offline refund path); the real way it
			// becomes CANCELLED is the window expiry, run exactly as the River worker does after aging the window out.
			adapter.cofAge(order, 73)
			if disposition, _ := adapter.cofExpire(order); disposition != "EXPIRED" {
				t.Fatalf("LCU2B2 fixture expiry: %s", disposition)
			}
			out.CustomerB, out.CancelledNumber = customer, "LC-"+strings.ToUpper(strings.ReplaceAll(order, "-", ""))
		}
		// Assert the product customer projection under real runtime RLS, not an owner SELECT or hand-written DTO.
		err = platform.WithScope(ctx, f.runtime, token, f.storeA1, "customers:read", func(tx pgx.Tx, scope platform.Scope) error {
			got, readErr := customers.Get(ctx, tx, scope, token, customer)
			if readErr == nil && (got.DisplayName == nil || *got.DisplayName != name) {
				t.Error("LCU2B2 customer projection lost synthetic name")
			}
			return readErr
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if out.CustomerA == out.CustomerB {
		t.Fatal("LCU2B2 requires two independently created customers")
	}
	// A13 is checked through inbox.buyer_panel under inbox:read; an incomplete seed fails before browser startup.
	err = platform.WithScope(ctx, f.runtime, token, f.storeA1, "inbox:read", func(tx pgx.Tx, _ platform.Scope) error {
		panel, readErr := e.svc.BuyerPanel(ctx, tx, out.Conversation)
		if readErr != nil {
			return readErr
		}
		assertInboxCustomerLinkPanel(t, panel, out)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertInboxCustomerLinkPanel(t *testing.T, panel inbox.BuyerPanel, want inboxCustomerLinkFixture) {
	t.Helper()
	if len(panel.Claims) != 2 || panel.ClaimTotalMinor != 2100 || panel.PurchaseOrdinal != 1 || panel.AutoReply == nil || panel.AutoReply.SendState != "sent" || panel.LinkedCustomerID != nil {
		t.Fatal("LCU2B2 incomplete A13 claims/ordinal/auto-reply seed")
	}
	quantities := map[int]int{}
	for _, claim := range panel.Claims {
		if claim.Keyword != "B2" || claim.SessionID != want.Session {
			t.Fatal("LCU2B2 claim provenance mismatch")
		}
		quantities[claim.Quantity]++
	}
	if quantities[1] != 1 || quantities[2] != 1 {
		t.Fatal("LCU2B2 claim quantities mismatch")
	}
	if panel.Orders == nil || len(*panel.Orders) != 2 {
		t.Fatal("LCU2B2 A13 orders missing")
	}
	states := map[string]string{}
	for _, order := range *panel.Orders {
		states[order.Number] = order.State
	}
	if states[want.ConfirmedNumber] != "CONFIRMED" || states[want.CancelledNumber] != "CANCELLED" {
		t.Fatal("LCU2B2 A13 order states mismatch")
	}
}
