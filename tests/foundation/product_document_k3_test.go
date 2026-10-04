package foundation_test

// product_document_k3_test.go: K3 adversarial gates for unit product-core (docs/delivery/units/product-editor.md §f
// "product-core 独立测试（K3）：并发（PE05、PE09）、幂等（PE02）、隔离（PE10）的对抗测试"). Independent of the
// implementer; written against contracts/invariants.json I01–I04 and the §f rulings. Every test here names the
// assertion it would catch if the named guard were removed (mutation-proven; evidence in output/product-core-tests/).
// Evidence label: REAL_PG (fresh PG 18.6 container via scripts/dev/test-focused.sh).

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/catalog"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

// k3Doc saves a product document through the real document command (merchant scope, catalog:write).
func k3Doc(t *testing.T, f *testFixture, key string, in catalog.ProductDocumentInput) (catalog.ProductDocument, error) {
	t.Helper()
	return t04Scoped(context.Background(), f, f.tokens["a"], f.storeA1, "catalog:write",
		func(tx pgx.Tx, s platform.Scope) (catalog.ProductDocument, error) {
			return catalog.SaveProductDocument(context.Background(), tx, s, key, in)
		})
}

// k3Edit applies a merge-patch edit through the real edit command (merchant scope, catalog:write).
func k3Edit(t *testing.T, f *testFixture, key, id string, in catalog.ProductDocumentPatch) (catalog.ProductDocument, error) {
	t.Helper()
	return t04Scoped(context.Background(), f, f.tokens["a"], f.storeA1, "catalog:write",
		func(tx pgx.Tx, s platform.Scope) (catalog.ProductDocument, error) {
			return catalog.SaveProductEdit(context.Background(), tx, s, key, id, in)
		})
}

// k3Buyer is one prepared buyer: cart + home destination + quote, ready to Begin.
func k3Buyer(t *testing.T, b bcHarness, items []storefront.Item) bcHarness {
	t.Helper()
	b.prepare(t, mustIssue(t, b.cqHarness.service, b.f.storeA1), items)
	return b
}

// k3RaceBegins fires one Begin per prepared buyer at the same instant and returns per-buyer results.
func k3RaceBegins(buyers []bcHarness, keys []string) ([]checkout.Result, []error) {
	start := make(chan struct{})
	results := make([]checkout.Result, len(buyers))
	errs := make([]error, len(buyers))
	var wg sync.WaitGroup
	for i := range buyers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = buyers[i].begin(keys[i])
		}(i)
	}
	close(start)
	wg.Wait()
	return results, errs
}

// ---- PE05 adversarial (A6): 100 concurrent orders on one untracked SKU, all within max_per_order, all succeed ----
//
// Mutation anchors: (M1) drop `AND inventory_tracked` in planLocked -> the untracked SKU enters the plan, has no
// balance row, and every order fails; (M2) drop the PT422 max_per_order guard in post_river/0021 -> the over-cap
// buyer below succeeds. Both were run red and reverted (output/product-core-tests/).

func TestProductEditorK3UntrackedConcurrentOrders(t *testing.T) {
	b := bcSetup(t)
	cap2 := int64(2)
	doc, err := k3Doc(t, b.f, t04Key("k3-doc"), catalog.ProductDocumentInput{
		Name: "k3 untracked " + t04Tag(), Status: catalog.StatusActive,
		SKUs: []catalog.DocumentSKUInput{{PriceMinor: 1250, Stock: &catalog.DocumentStock{Mode: "untracked", MaxPerOrder: &cap2}}},
	})
	if err != nil || len(doc.SKUs) != 1 {
		t.Fatalf("untracked document: %+v %v", doc, err)
	}
	sku := doc.SKUs[0]
	if sku.InventoryTracked || sku.MaxPerOrder == nil || *sku.MaxPerOrder != 2 {
		t.Fatalf("untracked flags: %+v", sku)
	}

	const n = 100
	buyers := make([]bcHarness, n)
	keys := make([]string, n)
	for i := range buyers {
		buyers[i] = k3Buyer(t, b, []storefront.Item{{SKUID: sku.ID, Quantity: 2}}) // exactly max_per_order
		keys[i] = t04Key("k3-begin")
	}
	results, errs := k3RaceBegins(buyers, keys)
	orders := make([]string, 0, n)
	for i := range buyers {
		if errs[i] != nil {
			t.Fatalf("untracked order %d refused: %v (an untracked SKU within max_per_order must never see stock refusal)", i, errs[i])
		}
		orders = append(orders, results[i].OrderID)
	}
	// 100 distinct orders placed; the untracked SKU was never locked or deducted.
	seen := map[string]bool{}
	for _, id := range orders {
		if seen[id] {
			t.Fatalf("duplicate order id %s", id)
		}
		seen[id] = true
	}
	if got := countRows(t, b.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1`, sku.ID); got != 0 {
		t.Fatalf("untracked SKU has %d ledger rows, want 0 (never locked/deducted)", got)
	}
	if got := countRows(t, b.f.owner, `SELECT count(*) FROM inventory.reservation_lines WHERE sku_id=$1`, sku.ID); got != 0 {
		t.Fatalf("untracked SKU has %d reservation lines, want 0", got)
	}
	if got := countRows(t, b.f.owner, `SELECT count(*) FROM inventory.balances WHERE sku_id=$1`, sku.ID); got != 0 {
		t.Fatalf("untracked SKU has %d balance rows, want 0", got)
	}
	if got := countRows(t, b.f.owner, `SELECT count(*) FROM inventory.reservations WHERE id=ANY($1::uuid[])`, orders); got != n {
		t.Fatalf("reservations=%d want %d (one HELD row per order, with zero lines)", got, n)
	}

	// The same SKU over max_per_order is refused by begin_hold (PT422 max_per_order_exceeded), never placed.
	over := k3Buyer(t, b, []storefront.Item{{SKUID: sku.ID, Quantity: 3}})
	_, err = over.begin(t04Key("k3-over"))
	var cvs *fulfillment.CVSError
	if !errors.As(err, &cvs) || cvs.Code != "max_per_order_exceeded" {
		t.Fatalf("over-cap order: err=%v want 422 max_per_order_exceeded", err)
	}
	if got := countRows(t, b.f.owner, `SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, over.cap.Scope.OwnerID); got != 0 {
		t.Fatalf("over-cap refusal left %d orders", got)
	}
}

// ---- PE05 adversarial: the last unit of a tracked SKU sells exactly once under 50 concurrent buyers -------------

func TestProductEditorK3TrackedLastUnitFiftyBuyers(t *testing.T) {
	b := bcSetup(t)
	mustExec(t, b.f.owner, `UPDATE inventory.balances SET on_hand=1 WHERE warehouse_id=$1 AND sku_id=$2`,
		b.stock.warehouse.ID, b.stock.skus[0].ID)
	const n = 50
	buyers := make([]bcHarness, n)
	keys := make([]string, n)
	for i := range buyers {
		buyers[i] = k3Buyer(t, b, []storefront.Item{{SKUID: b.stock.skus[0].ID, Quantity: 1}})
		keys[i] = t04Key("k3-last")
	}
	_, errs := k3RaceBegins(buyers, keys)
	wins, short := 0, 0
	for i, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, command.ErrInsufficient):
			short++
		default:
			t.Fatalf("buyer %d: unexpected error %v", i, err)
		}
	}
	if wins != 1 || short != n-1 {
		t.Fatalf("last unit: wins=%d insufficient=%d, want 1 and %d (oversell!)", wins, short, n-1)
	}
	var onHand, reserved int64
	if err := b.f.owner.QueryRow(context.Background(),
		`SELECT on_hand,reserved FROM inventory.balances WHERE warehouse_id=$1 AND sku_id=$2`,
		b.stock.warehouse.ID, b.stock.skus[0].ID).Scan(&onHand, &reserved); err != nil {
		t.Fatal(err)
	}
	if onHand != 1 || reserved != 1 {
		t.Fatalf("balance after race: on_hand=%d reserved=%d, want 1/1 (I03)", onHand, reserved)
	}
}

// ---- PE09 adversarial: two concurrent edits with the same expected_version — one wins, the other 409s -----------
//
// Mutation anchor (M3): drop the `cur.Version != in.ExpectedVersion` pre-check in applyProductEdit -> the loser
// still fails (the UPDATE's WHERE version= backstop) but as 404/ErrNotFound instead of 409, and this test goes red.

func TestProductEditorK3ConcurrentSameVersionEdits(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("K3 race"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	nameA, nameB := e.name("K3 winner A"), e.name("K3 winner B")
	edit := func(name string) pdPatch {
		return pdPatch{ID: p.ID, Name: pdStr(name), Description: pdStr(""), ExpectedVersion: p.Version,
			SKUs: &[]pdSKUPatch{{ID: p.SKUs[0].ID, PriceMinor: pdI64(1000)}}}
	}
	type outcome struct {
		status int
		body   []byte
	}
	run := func(name, key string, ch chan<- outcome) {
		status, body := e.a.call("PUT", "/products/"+p.ID+"/document", key, edit(name))
		ch <- outcome{status, body}
	}
	chA, chB := make(chan outcome, 1), make(chan outcome, 1)
	start := make(chan struct{})
	go func() { <-start; run(nameA, e.key("edit-a"), chA) }()
	go func() { <-start; run(nameB, e.key("edit-b"), chB) }()
	close(start)
	a, b := <-chA, <-chB

	byName := map[string]outcome{nameA: a, nameB: b}
	var winner string
	for name, o := range byName {
		if o.status == 200 {
			winner = name
		} else if o.status != 409 {
			t.Fatalf("edit %q: status=%d body=%s, want 200 or 409", name, o.status, o.body)
		}
	}
	if winner == "" {
		t.Fatalf("both edits lost: a=%d b=%d (a version race must not deadlock the command)", a.status, b.status)
	}
	var cur struct {
		Name    string `json:"name"`
		Version int64  `json:"version"`
	}
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if cur.Version != 2 || cur.Name != winner {
		t.Fatalf("final state: name=%q version=%d, want winner %q at version 2 (no silent overwrite)", cur.Name, cur.Version, winner)
	}
}

// ---- PE02 adversarial: the same Idempotency-Key under concurrency ----------------------------------------------

func TestProductEditorK3ConcurrentIdempotentReplay(t *testing.T) {
	e := ccNew(t)
	key := e.key("idem")
	name := e.name("K3 idem")
	doc := pdDoc{Name: name, Description: "same bytes", SKUs: []pdSKU{{PriceMinor: 1000}}}
	auditBefore := t04Count(t, e.h.f, `SELECT count(*) FROM ops.audit_events WHERE action='catalog.product.saved' AND store_id=$1`, e.h.f.storeA1)

	// 8 concurrent identical requests: all replay the one winner; exactly one product, one receipt, one audit row.
	const n = 8
	type outcome struct {
		status int
		body   []byte
	}
	ch := make(chan outcome, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			<-start
			status, body := e.a.call("POST", "/products/document", key, doc)
			ch <- outcome{status, body}
		}()
	}
	close(start)
	var firstID string
	for i := 0; i < n; i++ {
		o := <-ch
		if o.status != 200 {
			t.Fatalf("replay %d: status=%d body=%s", i, o.status, o.body)
		}
		var got pdProduct
		if err := json.Unmarshal(o.body, &got); err != nil || got.ID == "" {
			t.Fatalf("replay %d: decode %v body=%s", i, err, o.body)
		}
		if firstID == "" {
			firstID = got.ID
		} else if got.ID != firstID {
			t.Fatalf("replay %d: id=%s want %s (a key replay must return the first result)", i, got.ID, firstID)
		}
	}
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE name=$1`, name); got != 1 {
		t.Fatalf("products with the idem name: %d want 1", got)
	}
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM ops.command_results WHERE idempotency_key=$1`, key); got != 1 {
		t.Fatalf("command results for the key: %d want 1", got)
	}
	if got := t04Count(t, e.h.f, `SELECT count(*) FROM ops.audit_events WHERE action='catalog.product.saved' AND store_id=$1`, e.h.f.storeA1) - auditBefore; got != 1 {
		t.Fatalf("audit rows written by the storm: %d want 1 (a replay never re-audits)", got)
	}

	// The same key with different bytes, raced against the original bytes: exactly one side wins, the other is 409.
	key2 := e.key("idem2")
	changed := doc
	changed.Name = e.name("K3 idem changed")
	chA, chB := make(chan outcome, 1), make(chan outcome, 1)
	start2 := make(chan struct{})
	go func() { <-start2; s, b := e.a.call("POST", "/products/document", key2, doc); chA <- outcome{s, b} }()
	go func() { <-start2; s, b := e.a.call("POST", "/products/document", key2, changed); chB <- outcome{s, b} }()
	close(start2)
	oa, ob := <-chA, <-chB
	statuses := map[int]int{oa.status: 1, ob.status: 1}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatalf("same key, different bytes, concurrent: statuses %d and %d, want one 200 and one 409 (I02)", oa.status, ob.status)
	}
}

// ---- A6 switch under load: tracked -> untracked while a HELD reservation is open --------------------------------

func TestProductEditorK3TrackedToUntrackedWithOpenHold(t *testing.T) {
	b := bcSetup(t) // prepared buyer holds 2 of skus[0] (on_hand 10)
	held, err := b.begin(t04Key("k3-hold"))
	if err != nil {
		t.Fatal(err)
	}
	balance := func() (int64, int64) {
		t.Helper()
		var onHand, reserved int64
		if err := b.f.owner.QueryRow(context.Background(),
			`SELECT on_hand,reserved FROM inventory.balances WHERE warehouse_id=$1 AND sku_id=$2`,
			b.stock.warehouse.ID, b.stock.skus[0].ID).Scan(&onHand, &reserved); err != nil {
			t.Fatal(err)
		}
		return onHand, reserved
	}
	if onHand, reserved := balance(); onHand != 10 || reserved != 2 {
		t.Fatalf("before switch: on_hand=%d reserved=%d want 10/2", onHand, reserved)
	}
	sku, other := b.stock.skus[0], b.stock.skus[1]
	product := b.stock.product
	ledgerRows := func() int {
		t.Helper()
		return countRows(t, b.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1`, sku.ID)
	}
	before := ledgerRows()

	// The merchant re-axes the SKU to untracked while the hold is open. The hold must survive untouched.
	cap3 := int64(3)
	doc, err := k3Edit(t, b.f, t04Key("k3-flip"), product.ID, catalog.ProductDocumentPatch{
		Name: pdStr(product.Name), Status: pdStr(catalog.StatusActive), ExpectedVersion: product.Version,
		SKUs: &[]catalog.DocumentSKUPatch{
			{ID: sku.ID, PriceMinor: &sku.PriceMinor, Stock: &catalog.DocumentStock{Mode: "untracked", MaxPerOrder: &cap3}},
			{ID: other.ID, PriceMinor: &other.PriceMinor},
		},
	})
	if err != nil {
		t.Fatalf("tracked->untracked switch with an open hold: %v", err)
	}
	var flipped *catalog.SKU
	for i := range doc.SKUs {
		if doc.SKUs[i].ID == sku.ID {
			flipped = &doc.SKUs[i]
		}
	}
	if flipped == nil || flipped.InventoryTracked || flipped.MaxPerOrder == nil || *flipped.MaxPerOrder != 3 {
		t.Fatalf("flipped SKU: %+v", flipped)
	}
	if onHand, reserved := balance(); onHand != 10 || reserved != 2 {
		t.Fatalf("the switch touched the open hold: on_hand=%d reserved=%d want 10/2", onHand, reserved)
	}

	// New orders no longer lock stock: qty 3 (at cap) places with zero new ledger rows; qty 4 is refused.
	at := k3Buyer(t, b, []storefront.Item{{SKUID: sku.ID, Quantity: 3}})
	placed, err := at.begin(t04Key("k3-at-cap"))
	if err != nil {
		t.Fatalf("untracked order at cap after switch: %v", err)
	}
	if after := ledgerRows(); after != before {
		t.Fatalf("untracked order wrote ledger rows: before=%d after=%d", before, after)
	}
	if got := countRows(t, b.f.owner, `SELECT count(*) FROM inventory.reservation_lines WHERE reservation_id=$1`, placed.OrderID); got != 0 {
		t.Fatalf("untracked order has %d reservation lines, want 0", got)
	}
	if onHand, reserved := balance(); onHand != 10 || reserved != 2 {
		t.Fatalf("untracked order touched the balance: on_hand=%d reserved=%d want 10/2", onHand, reserved)
	}
	over := k3Buyer(t, b, []storefront.Item{{SKUID: sku.ID, Quantity: 4}})
	_, err = over.begin(t04Key("k3-over"))
	var cvs *fulfillment.CVSError
	if !errors.As(err, &cvs) || cvs.Code != "max_per_order_exceeded" {
		t.Fatalf("over-cap after switch: err=%v want max_per_order_exceeded", err)
	}

	// The open hold still works end to end: the real expiry definer (checkout.expire_held, expiry worker) releases it
	// even though the SKU flipped to untracked after the hold was taken. (The merchant ReleaseReservation is fenced off
	// checkout-owned reservations by design, migrations/0013.)
	bcDue(t, b, held)
	if got := bcExpire(t, b, held, 1); got != "EXPIRED" {
		t.Fatalf("expiry of the pre-switch hold: %s want EXPIRED", got)
	}
	if onHand, reserved := balance(); onHand != 10 || reserved != 0 {
		t.Fatalf("after expiry: on_hand=%d reserved=%d want 10/0", onHand, reserved)
	}

	// Switching back to tracked restores the invariant surface (cap gone). The merge-patch edit has no warehouse_id and the
	// fixture keeps two active warehouses, so target_qty cannot be written here; the mode-only switch never writes stock.
	back, err := k3Edit(t, b.f, t04Key("k3-back"), product.ID, catalog.ProductDocumentPatch{
		Name: pdStr(product.Name), Status: pdStr(catalog.StatusActive), ExpectedVersion: doc.Product.Version,
		SKUs: &[]catalog.DocumentSKUPatch{
			{ID: sku.ID, PriceMinor: &sku.PriceMinor, Stock: &catalog.DocumentStock{Mode: "tracked"}},
			{ID: other.ID, PriceMinor: &other.PriceMinor},
		},
	})
	if err != nil {
		t.Fatalf("untracked->tracked switch-back: %v", err)
	}
	for i := range back.SKUs {
		if back.SKUs[i].ID == sku.ID && (!back.SKUs[i].InventoryTracked || back.SKUs[i].MaxPerOrder != nil) {
			t.Fatalf("switched-back SKU: %+v", back.SKUs[i])
		}
	}
	// The mode switches never write stock: the only ledger rows are the fixture ADJUST, the hold RESERVE and its RELEASE.
	if got := countRows(t, b.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1 AND kind='ADJUST'`, sku.ID); got != 1 {
		t.Fatalf("the mode switches must never write stock: ADJUST rows=%d want 1 (the fixture opening)", got)
	}
	if after := ledgerRows(); after != before+1 { // +1 RELEASE of the pre-switch hold
		t.Fatalf("ledger rows after release: %d want %d", after, before+1)
	}
}

// ---- PE10 adversarial: cross-store 404/not_found on every new route + honest list counts + draft purchase refusal

func TestProductEditorK3IsolationNewRoutes(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("K3 isolation"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	other := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}
	// copy of a foreign product: 404, and nothing created in store B
	other.refuse(404, "POST", "/products/"+p.ID+"/copy", e.key("copy"), map[string]any{"expected_version": p.Version})
	// bulk-status naming a real foreign id: per-item not_found, the target untouched
	var items []pdBulkItem
	other.ok("POST", "/products/bulk-status", e.key("bulk"), map[string]any{"ids": []string{p.ID}, "status": "archived"}, &items)
	if len(items) != 1 || items[0].Err != "not_found" || items[0].Status != "" {
		t.Fatalf("cross-store bulk item: %+v want per-item not_found", items)
	}
	var cur struct {
		Status string `json:"status"`
	}
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if cur.Status != "draft" {
		t.Fatalf("the foreign bulk write changed the product: status=%q want draft", cur.Status)
	}
	// the merchant list and its tallies are store-scoped: store B sees nothing of store A's tag
	var listB pdList
	other.ok("GET", "/catalog-products?q="+e.tag, "", nil, &listB)
	if listB.Total != 0 || len(listB.Items) != 0 || listB.StatusCounts.Draft != 0 || listB.StatusCounts.Active != 0 || listB.StatusCounts.Archived != 0 {
		t.Fatalf("store B list leaks store A rows: %+v", listB)
	}
	// purchase entry: a buyer cannot put the draft product's SKU into a cart (PE07's 购买入口 half)
	if st := e.cartPut(e.buyerToken(), p.SKUs[0].ID); st == 200 {
		t.Fatal("a draft product's SKU entered a buyer cart")
	}
}

// ---- P2-3 pin: the idempotency key is scoped to (tenant, store, operation), NOT to the actor --------------------
// Frozen 0002 command.Run behavior shared by every command; this test pins it so a future actor-scoping amendment
// flips this test on purpose, not by accident. REVIEW-product-core.md P2-3.

func TestProductEditorK3IdempotencyKeyScopeIsStoreNotActor(t *testing.T) {
	e := ccNew(t)
	_, token2 := lcPrincipal(t, e.h.f, e.h.f.tenantA, []string{e.h.f.storeA1}, "store:read", "catalog:write")
	second := ccAdmin{t: t, h: e.srv, token: token2, store: e.h.f.storeA1}

	key := e.key("actor")
	doc := pdDoc{Name: e.name("K3 actor"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}
	var first pdProduct
	e.a.ok("POST", "/products/document", key, doc, &first)

	// A different actor of the same store with the same key and the same bytes gets the first actor's result.
	var replay pdProduct
	second.ok("POST", "/products/document", key, doc, &replay)
	if replay.ID != first.ID {
		t.Fatalf("cross-actor replay id=%s want %s (the key is store-scoped, not actor-scoped)", replay.ID, first.ID)
	}
	// Same key, different bytes, different actor: 409.
	changed := doc
	changed.Description = "different actor, different bytes"
	second.refuse(409, "POST", "/products/document", key, changed)
}

// ---- P2-1: the 0109 CHECK is 3VL-safe (untracked + NULL max_per_order is refused with 23514), and begin_hold
// ---- refuses an untracked line whose cap is NULL as max_per_order_exceeded (defence in depth) even when the row
// ---- exists out-of-band.

func TestProductEditorK3UntrackedNullCapGap(t *testing.T) {
	b := bcSetup(t)
	// The tightened CHECK: an untracked SKU can never be stored with a NULL max_per_order.
	if _, err := b.f.owner.Exec(context.Background(),
		`UPDATE catalog.skus SET inventory_tracked=false, max_per_order=NULL WHERE id=$1`, b.stock.skus[0].ID); err == nil {
		t.Fatal("P2-1: untracked + NULL max_per_order was stored; the 3VL-safe CHECK must refuse it (23514)")
	} else if pgCode(err) != "23514" {
		t.Fatalf("untracked+NULL cap: err=%v, want 23514 CHECK violation", err)
	}
	// Defence in depth: drop the CHECK (the only way to reach the state out-of-band) and prove begin_hold still
	// refuses the NULL cap instead of placing an unbounded order.
	const restore = `ALTER TABLE catalog.skus ADD CONSTRAINT skus_inventory_max_per_order CHECK (
		(inventory_tracked AND max_per_order IS NULL)
		OR (NOT inventory_tracked AND max_per_order IS NOT NULL AND max_per_order BETWEEN 1 AND 999))`
	mustExec(t, b.f.owner, `ALTER TABLE catalog.skus DROP CONSTRAINT skus_inventory_max_per_order`)
	t.Cleanup(func() {
		if _, err := b.f.owner.Exec(context.Background(), `UPDATE catalog.skus SET inventory_tracked=true, max_per_order=NULL WHERE id=$1`, b.stock.skus[0].ID); err != nil {
			t.Errorf("restore fixture SKU: %v", err)
		}
		if _, err := b.f.owner.Exec(context.Background(), restore); err != nil {
			t.Errorf("restore skus_inventory_max_per_order: %v", err)
		}
	})
	mustExec(t, b.f.owner, `UPDATE catalog.skus SET inventory_tracked=false, max_per_order=NULL WHERE id=$1`, b.stock.skus[0].ID)
	big := k3Buyer(t, b, []storefront.Item{{SKUID: b.stock.skus[0].ID, Quantity: 1_000_000}}) // far over any cap
	_, err := big.begin(t04Key("k3-nullcap"))
	var cvs *fulfillment.CVSError
	if !errors.As(err, &cvs) || cvs.Code != "max_per_order_exceeded" {
		t.Fatalf("untracked+NULL cap must be refused by begin_hold: err=%v want 422 max_per_order_exceeded", err)
	}
	if got := countRows(t, b.f.owner, `SELECT count(*) FROM checkout.orders WHERE owner_id=$1`, big.cap.Scope.OwnerID); got != 0 {
		t.Fatalf("the refused unbounded order left %d orders", got)
	}
}

// ---- P2-2: an explicitly-archived SKU (active:false) releases its keyword in the same transaction, so the
// ---- keyword becomes reusable by a fresh SKU of the same edit (no keyword_taken rollback).

func TestProductEditorK3AxisChangeReleasesKeyword(t *testing.T) {
	e := ccNew(t)
	kw := pdKeyword()
	var doc pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("K3 axis"), Description: "", Status: "active",
		SKUs: []pdSKU{{PriceMinor: 1000, Keyword: kw}, {PriceMinor: 2000}}}, &doc)
	if len(doc.SKUs) != 2 {
		t.Fatalf("create: %+v", doc)
	}
	var skuA, skuB pdSKUOut
	for _, s := range doc.SKUs {
		switch s.PriceMinor {
		case 1000:
			skuA = s
		case 2000:
			skuB = s
		}
	}
	if skuA.ID == "" || skuB.ID == "" {
		t.Fatalf("create SKUs: %+v", doc.SKUs)
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1 AND keyword=$2`, skuA.ID, kw) != 1 {
		t.Fatalf("skuA must hold keyword %s", kw)
	}
	// Re-axe: skuA is explicitly archived (merge-patch, §g.1 — omission would leave it active) and a fresh SKU reuses its
	// keyword. The archive releases the keyword in the same transaction BEFORE the new SKU is written, so no keyword_taken.
	var edited pdProduct
	e.docEdit(doc.ID, e.key("edit"), pdPatch{Name: pdStr(doc.Name), Status: pdStr("active"), ExpectedVersion: doc.Version,
		SKUs: &[]pdSKUPatch{
			{ID: skuA.ID, Active: pdBool(false)},
			{ID: skuB.ID, PriceMinor: pdI64(2000)},
			{PriceMinor: pdI64(3000), Keyword: pdStr(kw)},
		}}, &edited)
	// skuA is archived, its keyword released, and the keyword is reused by the fresh SKU.
	var status string
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT status FROM catalog.skus WHERE id=$1`, skuA.ID).Scan(&status); err != nil || status != "archived" {
		t.Fatalf("omitted skuA status=%q err=%v, want archived", status, err)
	}
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1 AND keyword=$2`, skuA.ID, kw); got != 0 {
		t.Fatalf("skuA keyword %s was not released: %d rows", kw, got)
	}
	var holder string
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT sku_id::text FROM live.keyword_library WHERE keyword=$1`, kw).Scan(&holder); err != nil {
		t.Fatalf("keyword %s holder: %v", kw, err)
	}
	if holder == skuA.ID {
		t.Fatalf("keyword %s is still held by the archived SKU %s", kw, holder)
	}
	if holder == skuB.ID {
		t.Fatalf("keyword %s moved to a referenced SKU %s, want a fresh SKU", kw, holder)
	}
}

// ---- P2-5: copy clamps the source name so name +「（复制）」 stays within the 120-rune CHECK instead of a bare 422.

func TestProductEditorK3CopyClampsLongName(t *testing.T) {
	e := ccNew(t)
	name := e.tag + strings.Repeat("x", 120-len(e.tag)) // exactly 120 runes, tag-prefixed so ccNew's cleanup catches it
	if got := utf8.RuneCountInString(name); got != 120 {
		t.Fatalf("test name is %d runes, want 120", got)
	}
	var src pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: name, Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &src)
	if src.Name != name {
		t.Fatalf("source name %q, want the 120-rune input", src.Name)
	}
	var copy pdProduct
	e.a.ok("POST", "/products/"+src.ID+"/copy", e.key("copy"), map[string]any{"expected_version": src.Version}, &copy)
	want := name[:120-utf8.RuneCountInString("（复制）")] + "（复制）" // 116 + 4 = 120 runes, not a 23514 -> 422
	if copy.Name != want {
		t.Fatalf("copy name %q (%d runes), want the 120-rune clamp %q", copy.Name, utf8.RuneCountInString(copy.Name), want)
	}
	if got := utf8.RuneCountInString(copy.Name); got != 120 {
		t.Fatalf("copy name is %d runes, want 120", got)
	}
}
