package foundation_test

// product_document_edit_k3_test.go: K3 independent adversarial gates for unit product-core-edit
// (docs/delivery/units/product-editor.md §g merge-patch edit; REVIEW-product-core-edit.md). Written from the §g
// ruling and contracts/invariants.json, not from the implementation. The shared fixture store storeA1 is restored by
// every test (windows closed, only the warehouses a test deactivated are reactivated — ccNew/onlyWarehouse cleanups).
// Evidence label: REAL_PG (fresh PG container via scripts/dev/test-focused.sh).

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/httperror"
)

// pdeSKU reads one SKU row straight from the DB: the assertions below care about columns the detail/read DTOs do not
// all expose (row version, compare_at_minor) and about the absence of writes, which only the DB can prove.
type pdeSKU struct {
	Version    int64
	PriceMinor int64
	CompareAt  *int64
	Status     string
}

func pdeReadSKU(t *testing.T, e *ccEnv, id string) pdeSKU {
	t.Helper()
	var s pdeSKU
	if err := e.h.f.owner.QueryRow(context.Background(),
		`SELECT version,price_minor,compare_at_minor,status FROM catalog.skus WHERE id=$1`, id).
		Scan(&s.Version, &s.PriceMinor, &s.CompareAt, &s.Status); err != nil {
		t.Fatal(err)
	}
	return s
}

func pdeKeywordRows(t *testing.T, e *ccEnv, skuID string) int {
	t.Helper()
	return countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1`, skuID)
}

func pdeCollectionRows(t *testing.T, e *ccEnv, productID string) int {
	t.Helper()
	return countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.collection_products WHERE product_id=$1`, productID)
}

// ---- §g.1 concurrency: 50 concurrent edits with the same expected_version — exactly one 200, the rest 409 ---------
//
// Mutation anchors: dropping lockProduct or the `cur.Version != in.ExpectedVersion` pre-check lets a second edit
// overwrite the winner (lost update, version still 2 but a loser's name) or turns the loser's 409 into a 200/5xx.

func TestProductEditK3FiftyWaySameVersionRace(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("race50"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	const n = 50
	type outcome struct {
		status int
		body   []byte
	}
	results := make([]outcome, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			name := e.name("race50 winner")
			status, body := e.a.call("PUT", "/products/"+p.ID+"/document", e.key("race"),
				pdPatch{ExpectedVersion: p.Version, Name: pdStr(name),
					SKUs: &[]pdSKUPatch{{ID: p.SKUs[0].ID, PriceMinor: pdI64(1000)}}})
			results[i] = outcome{status, body}
		}(i)
	}
	close(start)
	wg.Wait()

	wins, conflicts := 0, 0
	for i, o := range results {
		switch o.status {
		case 200:
			wins++
		case 409:
			conflicts++
		default:
			t.Fatalf("edit %d: status=%d body=%s, want exactly one 200 and %d×409", i, o.status, o.body, n-1)
		}
	}
	if wins != 1 || conflicts != n-1 {
		t.Fatalf("race: wins=%d conflicts=%d, want 1 and %d (I14: no lost update, no deadlock)", wins, conflicts, n-1)
	}
	var cur struct {
		Name    string `json:"name"`
		Version int64  `json:"version"`
	}
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if cur.Version != 2 || cur.Name != e.name("race50 winner") {
		t.Fatalf("final state: name=%q version=%d, want the winner's name at version 2", cur.Name, cur.Version)
	}
	if v := pdeReadSKU(t, e, p.SKUs[0].ID); v.Version != 2 {
		t.Fatalf("winner SKU version=%d want 2 (exactly one edit applied)", v.Version)
	}
}

// ---- I02: the same key+bytes replays the FIRST result even after a later edit moved the product on -----------------

func TestProductEditK3IdempotentReplayAfterLaterEdit(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("idem base"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	var marker time.Time
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&marker); err != nil {
		t.Fatal(err)
	}
	nameA, nameB := e.name("idem A"), e.name("idem B")
	key := e.key("edit1")
	bodyA := pdPatch{ExpectedVersion: p.Version, Name: pdStr(nameA)}
	var v2 pdProduct
	e.docEdit(p.ID, key, bodyA, &v2)
	if v2.Version != 2 {
		t.Fatalf("first edit version=%d want 2", v2.Version)
	}
	var v3 pdProduct
	e.docEdit(p.ID, e.key("edit2"), pdPatch{ExpectedVersion: v2.Version, Name: pdStr(nameB)}, &v3)
	if v3.Version != 3 {
		t.Fatalf("second edit version=%d want 3", v3.Version)
	}

	// Replay key+bytes after the product moved to v3: the stored v2 result comes back, nothing is re-applied.
	status, body := e.a.call("PUT", "/products/"+p.ID+"/document", key, bodyA)
	if status != 200 {
		t.Fatalf("replay: status=%d body=%s, want the stored 200", status, body)
	}
	var replayed pdProduct
	if err := json.Unmarshal(body, &replayed); err != nil {
		t.Fatal(err)
	}
	if replayed.Version != 2 || replayed.Name != nameA {
		t.Fatalf("replay must return the FIRST result: name=%q version=%d, want %q at 2", replayed.Name, replayed.Version, nameA)
	}
	var cur struct {
		Name    string `json:"name"`
		Version int64  `json:"version"`
	}
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if cur.Name != nameB || cur.Version != 3 {
		t.Fatalf("the replay re-applied the edit: name=%q version=%d, want %q at 3", cur.Name, cur.Version, nameB)
	}
	// The replay never re-audits: exactly two name-edit audit rows since the marker, one per real edit.
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM ops.audit_events
		WHERE store_id=$1 AND action='catalog.product.edited.name' AND created_at>=$2`, e.h.f.storeA1, marker); got != 2 {
		t.Fatalf("name-edit audit rows since marker: %d want 2 (a replay must not audit)", got)
	}

	// The same key with different bytes is 409 (I02).
	e.docEditRefuse(409, p.ID, key, pdPatch{ExpectedVersion: p.Version, Name: pdStr(e.name("idem C"))})
}

// ---- §g.2: target_qty in a multi-warehouse store is a clear refusal — never a write to an arbitrary warehouse ------

func TestProductEditK3MultiWarehouseTargetQtyRefused(t *testing.T) {
	e := ccNew(t) // ccNew default: TWO active warehouses (wh1, wh2) — resolveWarehouse("") must refuse
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("multiwh"), Description: "", WarehouseID: e.wh1,
		SKUs: []pdSKU{{PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(5)}}}}, &p)
	sku := p.SKUs[0].ID

	key := e.key("tq")
	e.refuseCode(422, "invalid_request", "PUT", "/products/"+p.ID+"/document", key,
		pdPatch{ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: sku, Stock: &pdStock{TargetQty: pdI64(3)}}}})
	if on, _ := e.pdBalance(sku); on != 5 {
		t.Fatalf("the refused edit touched on_hand: %d want 5", on)
	}
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1 AND command_key=$2`, sku, key); got != 0 {
		t.Fatalf("the refused edit left %d ledger rows", got)
	}
	var v int64
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT version FROM catalog.products WHERE id=$1`, p.ID).Scan(&v); err != nil || v != 1 {
		t.Fatalf("the refused edit bumped the product version: %d (err %v), want 1 (whole rollback)", v, err)
	}
}

// ---- §g.1 presence matrix: absent keeps, null/"" clears, [] clears — and an unmentioned SKU is never touched -------

func TestProductEditK3NullVsAbsentMatrix(t *testing.T) {
	e := ccNew(t)
	kw1 := pdKeyword()
	col := e.collection("matrix col", nil)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{
		Name: e.name("matrix"), Description: "", Status: "active",
		CollectionIDs: []string{col.ID},
		SKUs: []pdSKU{
			{PriceMinor: 1000, CompareAtMinor: pdI64(2000), Keyword: kw1},
			{PriceMinor: 500},
		},
	}, &p)
	sku1, sku2 := p.SKUs[0].ID, p.SKUs[1].ID
	sku2v0 := p.SKUs[1].Version
	version := p.Version
	edit := func(key string, patch map[string]any) {
		t.Helper()
		patch["expected_version"] = version
		status, body := e.a.call("PUT", "/products/"+p.ID+"/document", key, patch)
		if status != 200 {
			t.Fatalf("edit %s: status=%d body=%s", key, status, body)
		}
		var out pdProduct
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		version = out.Version
	}
	sku1Patch := func(fields map[string]any) map[string]any {
		fields["id"] = sku1
		return map[string]any{"skus": []any{fields}}
	}

	// 1. name-only patch: collection membership and every SKU field kept (absent keeps).
	edit(e.key("s1"), map[string]any{"name": e.name("matrix s1")})
	if got := pdeCollectionRows(t, e, p.ID); got != 1 {
		t.Fatalf("absent collection_ids must keep membership: %d rows", got)
	}
	if s := pdeReadSKU(t, e, sku1); s.CompareAt == nil || *s.CompareAt != 2000 {
		t.Fatalf("absent compare_at_minor must keep 2000: %+v", s)
	}

	// 2. collection_ids: [] clears.
	edit(e.key("s2"), map[string]any{"collection_ids": []any{}})
	if got := pdeCollectionRows(t, e, p.ID); got != 0 {
		t.Fatalf("collection_ids [] must clear membership: %d rows", got)
	}

	// 3. compare_at_minor: null clears.
	edit(e.key("s3"), sku1Patch(map[string]any{"compare_at_minor": nil}))
	if s := pdeReadSKU(t, e, sku1); s.CompareAt != nil {
		t.Fatalf("compare_at_minor null must clear: %+v", s)
	}

	// 4. set it again, then a price-only patch: absent keeps the restored value.
	edit(e.key("s4"), sku1Patch(map[string]any{"compare_at_minor": 2200}))
	edit(e.key("s5"), sku1Patch(map[string]any{"price_minor": 1100}))
	if s := pdeReadSKU(t, e, sku1); s.CompareAt == nil || *s.CompareAt != 2200 || s.PriceMinor != 1100 {
		t.Fatalf("absent compare_at must keep 2200 across a price patch: %+v", s)
	}

	// 5. keyword "" clears; a set restores it; null clears; a price patch (absent) keeps it.
	edit(e.key("s6"), sku1Patch(map[string]any{"keyword": ""}))
	if got := pdeKeywordRows(t, e, sku1); got != 0 {
		t.Fatalf(`keyword "" must clear: %d rows`, got)
	}
	kw2 := pdKeyword()
	edit(e.key("s7"), sku1Patch(map[string]any{"keyword": kw2}))
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1 AND keyword=$2`, sku1, kw2); got != 1 {
		t.Fatalf("keyword set must write %s: %d rows", kw2, got)
	}
	edit(e.key("s8"), sku1Patch(map[string]any{"keyword": nil}))
	if got := pdeKeywordRows(t, e, sku1); got != 0 {
		t.Fatalf("keyword null must clear: %d rows", got)
	}
	kw3 := pdKeyword()
	edit(e.key("s9"), sku1Patch(map[string]any{"keyword": kw3}))
	edit(e.key("s10"), sku1Patch(map[string]any{"price_minor": 1200}))
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1 AND keyword=$2`, sku1, kw3); got != 1 {
		t.Fatalf("absent keyword must keep %s across a price patch: %d rows", kw3, got)
	}

	// The unmentioned SKU2 was never touched: no version bump, no price/keyword/stock change (§g.1).
	if s := pdeReadSKU(t, e, sku2); s.Version != sku2v0 || s.PriceMinor != 500 || s.Status != "active" {
		t.Fatalf("unmentioned SKU2 changed: %+v (want version %d, price 500, active)", s, sku2v0)
	}
	if got := pdeKeywordRows(t, e, sku2); got != 0 {
		t.Fatalf("unmentioned SKU2 gained a keyword row: %d", got)
	}
	if _, ok := e.pdBalance(sku2); ok {
		t.Fatal("unmentioned SKU2 gained a balance row")
	}
}

// ---- §g.5: a CLOSED window does not block; the same edits an OPEN window refuses go through after close ------------

func TestProductEditK3ClosedWindowDoesNotBlock(t *testing.T) {
	e := ccNew(t)
	kw := pdKeyword()
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("win close"), Description: "", Status: "active",
		SKUs: []pdSKU{{PriceMinor: 1000, Keyword: kw}, {PriceMinor: 2000}}}, &p)
	skuA := p.SKUs[0].ID
	e.seedLiveWindow(skuA) // cleanup closes the window; this test closes it explicitly mid-way

	// Sanity: while OPEN, the keyword touch is refused.
	e.refuseCode(409, "live_window_open", "PUT", "/products/"+p.ID+"/document", e.key("open"),
		pdPatch{ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: skuA, Keyword: pdStr("")}}})

	// Close the window (scoped to this test's SKU's offer): the same edits must now succeed.
	mustExec(t, e.h.f.owner, `UPDATE live.claim_windows w SET state='CLOSED',closed_at=clock_timestamp()
		FROM live.offers o
		WHERE o.tenant_id=w.tenant_id AND o.store_id=w.store_id AND o.session_id=w.session_id
			AND o.sku_id=$1 AND w.state='OPEN'`, skuA)
	kw2 := pdKeyword()
	var v2 pdProduct
	e.docEdit(p.ID, e.key("kw"), pdPatch{ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: skuA, Keyword: pdStr(kw2)}}}, &v2)
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1 AND keyword=$2`, skuA, kw2); got != 1 {
		t.Fatalf("keyword change after close: %d rows with %s", got, kw2)
	}
	var v3 pdProduct
	e.docEdit(p.ID, e.key("arch"), pdPatch{ExpectedVersion: v2.Version,
		SKUs: &[]pdSKUPatch{{ID: skuA, Active: pdBool(false)}}}, &v3)
	if s := pdeReadSKU(t, e, skuA); s.Status != "archived" {
		t.Fatalf("active:false after close: status=%q want archived", s.Status)
	}
	// Unlisting is also free once the window is closed.
	e.docEdit(p.ID, e.key("unlist"), pdPatch{ExpectedVersion: v3.Version, Status: pdStr("draft")}, &v3)
}

// ---- I01: a foreign store's detail read is 404, never a leak ------------------------------------------------------

func TestProductEditK3CrossStoreDetail404(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("xstore"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	other := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}
	other.refuse(404, "GET", "/products/"+p.ID, "", nil)
	// and the edit command on a foreign id is likewise 404, with the product untouched
	other.refuse(404, "PUT", "/products/"+p.ID+"/document", e.key("x"),
		pdPatch{ExpectedVersion: p.Version, Name: pdStr(e.name("xstore hijack"))})
	var cur struct {
		Name string `json:"name"`
	}
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if strings.Contains(cur.Name, "hijack") {
		t.Fatalf("the foreign edit changed the product: %q", cur.Name)
	}
}

// ---- §g.4: committed = reserved+allocated+unavailable from the single warehouse ------------------------------------

func TestProductEditK3DetailCommittedComposition(t *testing.T) {
	e := ccNew(t)
	e.onlyWarehouse(e.wh1)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("committed"), Description: "", WarehouseID: e.wh1,
		SKUs: []pdSKU{{PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(7)}}}}, &p)
	sku := p.SKUs[0].ID
	mustExec(t, e.h.f.owner, `UPDATE inventory.balances SET reserved=2,allocated=1,unavailable=1
		WHERE warehouse_id=$1 AND sku_id=$2`, e.wh1, sku)

	var d pdDetail
	e.a.ok("GET", "/products/"+p.ID, "", nil, &d)
	if len(d.SKUs) != 1 || d.SKUs[0].Committed == nil || *d.SKUs[0].Committed != 4 {
		t.Fatalf("committed=%v want 4 (reserved 2 + allocated 1 + unavailable 1)", d.SKUs)
	}
	if d.SKUs[0].OnHand == nil || *d.SKUs[0].OnHand != 7 || d.SKUs[0].Available != 3 {
		t.Fatalf("on_hand/available: %+v want 7/3", d.SKUs[0])
	}
}

// ---- §f.6: patched prices honour TWD whole-dollar ------------------------------------------------------------------

func TestProductEditK3TWDWholeDollarOnPatchedPrices(t *testing.T) {
	e := ccNew(t)
	b := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB} // storeB is the TWD store (PE04)
	var p pdProduct
	b.ok("POST", "/products/document", e.key("p"), pdDoc{Name: e.name("twd edit"), Description: "",
		SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)
	sku := p.SKUs[0].ID
	edit := func(key string, in pdPatch) (int, []byte) {
		return b.call("PUT", "/products/"+p.ID+"/document", key, in)
	}
	refuse := func(want int, code, key string, in pdPatch) {
		t.Helper()
		status, body := edit(key, in)
		if status != want {
			t.Fatalf("TWD edit status=%d want %d body=%s", status, want, body)
		}
		var env httperror.Envelope
		if err := json.Unmarshal(body, &env); err != nil || env.Code != code {
			t.Fatalf("TWD edit code=%q err=%v want %q body=%s", env.Code, err, code, body)
		}
	}

	refuse(422, "amount_not_whole_twd", e.key("p1"),
		pdPatch{ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: sku, PriceMinor: pdI64(1050)}}})
	refuse(422, "amount_not_whole_twd", e.key("p2"),
		pdPatch{ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: sku, CompareAtMinor: pdI64(1550)}}})
	if s := pdeReadSKU(t, e, sku); s.PriceMinor != 1000 || s.CompareAt != nil {
		t.Fatalf("the refused price edits changed the SKU: %+v", s)
	}
	// a whole-dollar patch is accepted
	status, body := edit(e.key("ok"), pdPatch{ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: sku, PriceMinor: pdI64(1500), CompareAtMinor: pdI64(2000)}}})
	if status != 200 {
		t.Fatalf("whole-dollar edit status=%d body=%s", status, body)
	}
}

// ---- §g.1 audit: field names only, never values or PII --------------------------------------------------------------

func TestProductEditK3AuditFieldNamesOnly(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("audit base"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	var marker time.Time
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&marker); err != nil {
		t.Fatal(err)
	}
	secret := e.name("audit SECRET-VALUE") // unique, tag-prefixed: must never appear in an audit action
	kw := pdKeyword()
	e.docEdit(p.ID, e.key("edit"), pdPatch{ExpectedVersion: p.Version, Name: pdStr(secret),
		SKUs: &[]pdSKUPatch{{ID: p.SKUs[0].ID, PriceMinor: pdI64(1500), Keyword: pdStr(kw)}}}, &pdProduct{})

	rows, err := e.h.f.owner.Query(context.Background(), `SELECT action FROM ops.audit_events
		WHERE store_id=$1 AND created_at>=$2 ORDER BY action`, e.h.f.storeA1, marker)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	actions := []string{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, a)
	}
	want := []string{"catalog.product.edited.name", "catalog.sku.edited.keyword", "catalog.sku.edited.price_minor"}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Fatalf("audit actions=%v want %v (one row per changed field name)", actions, want)
	}
	for _, a := range actions {
		if strings.Contains(a, e.tag) || strings.Contains(a, "SECRET") || strings.Contains(a, kw) || strings.Contains(a, "1500") {
			t.Fatalf("audit action %q carries a field VALUE (only field names are allowed, I11)", a)
		}
	}
}

// ---- REVIEW P1-1: an options-only patch (no skus key) must not strand active SKUs — invalid + whole rollback --------
//
// Red on the pre-fix code: the axes are replaced (200) because the fit check lives behind `in.SKUs != nil`.

func TestProductEditK3OptionsOnlyPatchMustNotStrandSKUs(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{
		Name: e.name("strand"), Description: "", Status: "active",
		Options: []ccAxis{{Name: "Color", Values: []string{"Red"}}},
		SKUs:    []pdSKU{{OptionValues: []string{"Red"}, PriceMinor: 1000}},
	}, &p)
	sku := p.SKUs[0].ID

	e.refuseCode(422, "invalid_request", "PUT", "/products/"+p.ID+"/document", e.key("axes"),
		pdPatch{ExpectedVersion: p.Version, Options: &[]ccAxis{{Name: "Color", Values: []string{"Crimson"}}}})

	// Whole rollback: old axes, SKU active with its combination, version unchanged.
	var d pdDetail
	e.a.ok("GET", "/products/"+p.ID, "", nil, &d)
	if len(d.Options) != 1 || len(d.Options[0].Values) != 1 || d.Options[0].Values[0] != "Red" {
		t.Fatalf("the refused options-only patch changed the axes: %+v", d.Options)
	}
	if s := pdeReadSKU(t, e, sku); s.Status != "active" || s.Version != 1 {
		t.Fatalf("the refused options-only patch touched the SKU: %+v", s)
	}
	if d.Version != 1 {
		t.Fatalf("the refused options-only patch bumped the product version: %d", d.Version)
	}
}

// ---- REVIEW P2-1: patching a SKU onto another active SKU's combination is invalid (422), never a silent write ------
//
// Red on the pre-fix code: the Go duplicate check misses existing-existing pairs and the 0086 unique index surfaces
// the refusal as 409 conflict instead of the contract's 422 invalid.

func TestProductEditK3DuplicateCombinationViaPatch(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{
		Name: e.name("dupcombo"), Description: "",
		Options: []ccAxis{{Name: "Color", Values: []string{"Red", "Blue"}}},
		SKUs: []pdSKU{
			{OptionValues: []string{"Red"}, PriceMinor: 1000},
			{OptionValues: []string{"Blue"}, PriceMinor: 1100},
		},
	}, &p)
	red, blue := p.SKUs[0].ID, p.SKUs[1].ID

	e.refuseCode(422, "invalid_request", "PUT", "/products/"+p.ID+"/document", e.key("dup"),
		pdPatch{ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: red, OptionValues: &[]string{"Blue"}}}})

	// Whole rollback: both SKUs keep their combinations, the product version is unchanged.
	if s := pdeReadSKU(t, e, red); s.Version != 1 {
		t.Fatalf("the refused duplicate patch touched the red SKU: %+v", s)
	}
	if s := pdeReadSKU(t, e, blue); s.Version != 1 {
		t.Fatalf("the refused duplicate patch touched the blue SKU: %+v", s)
	}
	var v int64
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT version FROM catalog.products WHERE id=$1`, p.ID).Scan(&v); err != nil || v != 1 {
		t.Fatalf("the refused duplicate patch bumped the product version: %d (err %v)", v, err)
	}
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND option_values='{Blue}' AND status='active'`, p.ID); got != 1 {
		t.Fatalf("active [Blue] SKUs: %d want exactly 1", got)
	}
}

// P2-2 (integrator ruling): an active:false entry carries only id + active. Combining it with another field is 422 and the
// whole patch rolls back — never a silent drop of the other fields.
func TestProductEditArchiveWithOtherFieldsInvalid(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("archive+price"), Description: "",
		SKUs: []pdSKU{{PriceMinor: 1000}, {PriceMinor: 2000}}}, &p)
	sku := p.SKUs[0].ID
	e.refuseCode(422, "invalid_request", "PUT", "/products/"+p.ID+"/document", e.key("bad"),
		pdPatch{ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: sku, Active: pdBool(false), PriceMinor: pdI64(3000)}}})
	if s := pdeReadSKU(t, e, sku); s.Version != 1 {
		t.Fatalf("the refused archive+price patch touched the SKU: %+v", s)
	}
	var archived pdProduct
	e.docEdit(p.ID, e.key("ok"), pdPatch{ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: sku, Active: pdBool(false)}}}, &archived)
	if len(archived.SKUs) != 1 {
		t.Fatalf("archive-only patch: %+v", archived.SKUs)
	}
}

// P2-3 (integrator ruling): an edit may not leave more than 100 active SKUs (the create cap, the ListSKUs contract and the
// detail read all assume it). Archiving one and adding one in the same patch stays at 100 and is accepted.
func TestProductEditActiveSKUCap(t *testing.T) {
	e := ccNew(t)
	skus := make([]pdSKU, 100)
	for i := range skus {
		// explicit codes: generated codes for axis-less SKUs try only 50 suffixes per base (freeSKUCode)
		skus[i] = pdSKU{PriceMinor: 1000, Code: fmt.Sprintf("%s-cap-%03d", e.tag, i)}
	}
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("cap"), Description: "", SKUs: skus}, &p)
	if len(p.SKUs) != 100 {
		t.Fatalf("create 100: %d", len(p.SKUs))
	}
	e.refuseCode(422, "invalid_request", "PUT", "/products/"+p.ID+"/document", e.key("101"),
		pdPatch{ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{PriceMinor: pdI64(1000)}}})
	if got := countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND status='active'`, p.ID); got != 100 {
		t.Fatalf("active SKUs after refused 101st: %d", got)
	}
	var swapped pdProduct
	e.docEdit(p.ID, e.key("swap"), pdPatch{ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: p.SKUs[0].ID, Active: pdBool(false)}, {PriceMinor: pdI64(2000)}}}, &swapped)
	if len(swapped.SKUs) != 100 {
		t.Fatalf("archive one + add one: %d active", len(swapped.SKUs))
	}
}
