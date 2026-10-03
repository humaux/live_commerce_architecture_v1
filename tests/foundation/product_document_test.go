package foundation_test

// product_document_test.go: acceptance gates PE01-PE11 (PG and Go parts) of unit product-core
// (docs/delivery/units/product-editor.md §e). Written from the contract + §f rulings by a test author who is not the
// implementer; it shares only the repo's harness (ccNew/t04Fixture/bhSetup) and exercises the merchant document command
// (POST /products/document, PUT /products/{id}/document), bulk-status, copy, the image cap and the product list through the
// real httpapi handler. The K3 adversarial passes (PE02/PE05/PE09/PE10 concurrency) are separate; these are the sequential
// PG/Go gates. Evidence label: REAL_PG.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/httperror"
)

// ---- document request/response DTOs (JSON of internal/catalog document.go) ------------------------------------------

type pdDoc struct {
	ID              string   `json:"id,omitempty"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Status          string   `json:"status,omitempty"`
	Slug            string   `json:"slug,omitempty"`
	SEOTitle        string   `json:"seo_title,omitempty"`
	SEODescription  string   `json:"seo_description,omitempty"`
	Options         []ccAxis `json:"options,omitempty"`
	SKUs            []pdSKU  `json:"skus"`
	CollectionIDs   []string `json:"collection_ids,omitempty"`
	WarehouseID     string   `json:"warehouse_id,omitempty"`
	WeightGrams     int64    `json:"weight_grams,omitempty"`
	LengthMM        int64    `json:"length_mm,omitempty"`
	WidthMM         int64    `json:"width_mm,omitempty"`
	HeightMM        int64    `json:"height_mm,omitempty"`
	ExpectedVersion int64    `json:"expected_version"`
}

type pdSKU struct {
	ID             string   `json:"id,omitempty"`
	OptionValues   []string `json:"option_values,omitempty"`
	Code           string   `json:"code,omitempty"`
	PriceMinor     int64    `json:"price_minor"`
	CompareAtMinor *int64   `json:"compare_at_minor,omitempty"`
	OriginCountry  string   `json:"origin_country,omitempty"`
	CustomsName    string   `json:"customs_name,omitempty"`
	HSCandidate    string   `json:"hs_candidate,omitempty"`
	Stock          *pdStock `json:"stock,omitempty"`
	Keyword        string   `json:"keyword,omitempty"`
	Active         *bool    `json:"active,omitempty"`
}

type pdStock struct {
	Mode        string `json:"mode,omitempty"`
	OpeningQty  *int64 `json:"opening_qty,omitempty"`
	TargetQty   *int64 `json:"target_qty,omitempty"`
	MaxPerOrder *int64 `json:"max_per_order,omitempty"`
}

type pdProduct struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Status  string     `json:"status"`
	Version int64      `json:"version"`
	Slug    string     `json:"slug"`
	Options []ccAxis   `json:"options"`
	SKUs    []pdSKUOut `json:"skus"`
}

type pdSKUOut struct {
	ID               string   `json:"id"`
	ProductID        string   `json:"product_id"`
	Code             string   `json:"code"`
	Status           string   `json:"status"`
	Currency         string   `json:"currency"`
	PriceMinor       int64    `json:"price_minor"`
	Version          int64    `json:"version"`
	OptionValues     []string `json:"option_values"`
	InventoryTracked bool     `json:"inventory_tracked"`
	MaxPerOrder      *int64   `json:"max_per_order"`
}

// pdDetail is the merchant GET /products/{id} detail shape (product-editor §g.4).
type pdDetail struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Status        string        `json:"status"`
	Version       int64         `json:"version"`
	Slug          string        `json:"slug"`
	Options       []ccAxis      `json:"options"`
	CollectionIDs []string      `json:"collection_ids"`
	WarehouseID   *string       `json:"warehouse_id"`
	SKUs          []pdDetailSKU `json:"skus"`
}

type pdDetailSKU struct {
	ID         string `json:"id"`
	PriceMinor int64  `json:"price_minor"`
	Keyword    string `json:"keyword"`
	OnHand     *int64 `json:"on_hand"`
	Committed  *int64 `json:"committed"`
	Available  int64  `json:"available"`
}

type pdBulkItem struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Err    string `json:"error"`
}

type pdList struct {
	Items []struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Status           string `json:"status"`
		Keyword          string `json:"keyword"`
		InventoryTracked bool   `json:"inventory_tracked"`
		UpdatedAt        string `json:"updated_at"`
	} `json:"items"`
	NextCursor   string `json:"next_cursor"`
	Total        int    `json:"total"`
	StatusCounts struct {
		Draft    int `json:"draft"`
		Active   int `json:"active"`
		Archived int `json:"archived"`
	} `json:"status_counts"`
}

func pdI64(v int64) *int64   { return &v }
func pdStr(v string) *string { return &v }
func pdBool(v bool) *bool    { return &v }

// ---- edit (merge-patch) DTOs (product-editor §g) ------------------------------------------------------------------

// pdPatch mirrors internal/catalog.ProductDocumentPatch: pointers mark presence, so absent keeps the value and [] clears.
type pdPatch struct {
	ID              string        `json:"id,omitempty"`
	Name            *string       `json:"name,omitempty"`
	Description     *string       `json:"description,omitempty"`
	Status          *string       `json:"status,omitempty"`
	Slug            *string       `json:"slug,omitempty"`
	SEOTitle        *string       `json:"seo_title,omitempty"`
	SEODescription  *string       `json:"seo_description,omitempty"`
	Options         *[]ccAxis     `json:"options,omitempty"`
	SKUs            *[]pdSKUPatch `json:"skus,omitempty"`
	CollectionIDs   *[]string     `json:"collection_ids,omitempty"`
	WeightGrams     *int64        `json:"weight_grams,omitempty"`
	LengthMM        *int64        `json:"length_mm,omitempty"`
	WidthMM         *int64        `json:"width_mm,omitempty"`
	HeightMM        *int64        `json:"height_mm,omitempty"`
	ExpectedVersion int64         `json:"expected_version"`
}

// pdSKUPatch mirrors internal/catalog.DocumentSKUPatch: id present patches in place, id absent creates a new SKU.
type pdSKUPatch struct {
	ID             string    `json:"id,omitempty"`
	OptionValues   *[]string `json:"option_values,omitempty"`
	PriceMinor     *int64    `json:"price_minor,omitempty"`
	CompareAtMinor *int64    `json:"compare_at_minor,omitempty"`
	OriginCountry  *string   `json:"origin_country,omitempty"`
	CustomsName    *string   `json:"customs_name,omitempty"`
	HSCandidate    *string   `json:"hs_candidate,omitempty"`
	Stock          *pdStock  `json:"stock,omitempty"`
	Keyword        *string   `json:"keyword,omitempty"`
	Active         *bool     `json:"active,omitempty"`
	WeightGrams    *int64    `json:"weight_grams,omitempty"`
	LengthMM       *int64    `json:"length_mm,omitempty"`
	WidthMM        *int64    `json:"width_mm,omitempty"`
	HeightMM       *int64    `json:"height_mm,omitempty"`
}

// pdKeyword is a store-unique keyword in the live.keyword_library grammar (^[A-Z0-9]{1,16}$).
func pdKeyword() string { return "K" + strings.ToUpper(t04Tag())[:10] }

// ---- ccEnv helpers for the document command -------------------------------------------------------------------------

func (e *ccEnv) docCreate(key string, in pdDoc, out *pdProduct) {
	e.t.Helper()
	e.a.ok("POST", "/products/document", key, in, out)
}

func (e *ccEnv) docEdit(id, key string, in pdPatch, out *pdProduct) {
	e.t.Helper()
	e.a.ok("PUT", "/products/"+id+"/document", key, in, out)
}

func (e *ccEnv) docCreateRefuse(want int, key string, in pdDoc) {
	e.t.Helper()
	e.a.refuse(want, "POST", "/products/document", key, in)
}

func (e *ccEnv) docEditRefuse(want int, id, key string, in pdPatch) {
	e.t.Helper()
	e.a.refuse(want, "PUT", "/products/"+id+"/document", key, in)
}

// refuseCode asserts the exact transport status AND the contract error code (amount_not_whole_twd, keyword_taken, ...).
func (e *ccEnv) refuseCode(want int, code, method, path, key string, payload any) {
	e.t.Helper()
	status, body := e.a.call(method, path, key, payload)
	if status != want {
		e.t.Fatalf("%s %s %v: status=%d want %d body=%s", method, path, payload, status, want, body)
	}
	var env httperror.Envelope
	if err := json.Unmarshal(body, &env); err != nil || env.Code != code {
		e.t.Fatalf("%s %s: code=%q err=%v want %q body=%s", method, path, env.Code, err, code, body)
	}
}

// pdBalance reads the tracked SKU's on_hand (present=false when the SKU has no balance row at all — untracked SKUs never do).
func (e *ccEnv) pdBalance(skuID string) (onHand int64, present bool) {
	e.t.Helper()
	err := e.h.f.owner.QueryRow(context.Background(), `SELECT on_hand FROM inventory.balances WHERE sku_id=$1`, skuID).Scan(&onHand)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return onHand, true
}

// seedLiveWindow makes one OPEN claim window with an active offer on skuID, so a bulk unlist of that product is refused
// live_window_open per item (product-editor §f ruling 3). Seeded as owner: the live write path is not under test here.
func (e *ccEnv) seedLiveWindow(skuID string) {
	e.t.Helper()
	ctx := context.Background()
	var tenant, principal string
	if err := e.h.f.owner.QueryRow(ctx, `SELECT m.tenant_id,m.principal_id
		FROM identity.sessions s JOIN identity.memberships m USING(principal_id)
		JOIN control.stores st ON st.tenant_id=m.tenant_id
		WHERE s.token_hash=$1 AND st.id=$2`, tokenHash(e.h.f.tokens["a"]), e.h.f.storeA1).Scan(&tenant, &principal); err != nil {
		e.t.Fatal(err)
	}
	var session string
	if err := e.h.f.owner.QueryRow(ctx, `INSERT INTO live.sessions(tenant_id,store_id,principal_id,title)
		VALUES($1,$2,$3,'pe11') RETURNING id::text`, tenant, e.h.f.storeA1, principal).Scan(&session); err != nil {
		e.t.Fatal(err)
	}
	mustExec(e.t, e.h.f.owner, `INSERT INTO live.claim_windows(tenant_id,store_id,session_id,state,match_mode,generation,opened_at,principal_id)
		VALUES($1,$2,$3,'OPEN','EXACT',1,clock_timestamp(),$4)`, tenant, e.h.f.storeA1, session, principal)
	// storeA1 allows one OPEN window (live_claim_window_one_open) and is shared by every gate: close it after the test.
	e.t.Cleanup(func() {
		mustExec(e.t, e.h.f.owner, `UPDATE live.claim_windows SET state='CLOSED',closed_at=clock_timestamp() WHERE session_id=$1 AND state='OPEN'`, session)
	})
	mustExec(e.t, e.h.f.owner, `INSERT INTO live.offers(tenant_id,store_id,session_id,keyword,sku_id,max_quantity_per_claim,active,principal_id)
		VALUES($1,$2,$3,'PE11',$4,5,true,$5)`, tenant, e.h.f.storeA1, session, skuID, principal)
}

// onlyWarehouse deactivates every other ACTIVE warehouse of the shared fixture store, so the store reads exactly one active
// warehouse (resolveWarehouse("")/GetProductDetail single-warehouse branches). Cleanup reactivates only the warehouses it
// deactivated: other gates deliberately leave storeA1 warehouses inactive, and the full G07 suite shares this store.
func (e *ccEnv) onlyWarehouse(keep string) {
	e.t.Helper()
	ctx := context.Background()
	var ids []string
	if err := e.h.f.owner.QueryRow(ctx, `WITH off AS (UPDATE inventory.warehouses SET active=false
		WHERE store_id=$1 AND id<>$2 AND active RETURNING id::text) SELECT coalesce(array_agg(id),'{}') FROM off`,
		e.h.f.storeA1, keep).Scan(&ids); err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() {
		mustExec(e.t, e.h.f.owner, `UPDATE inventory.warehouses SET active=true WHERE store_id=$1 AND id=ANY($2::uuid[])`, e.h.f.storeA1, ids)
	})
}

// ---- PE01 single transaction: one save writes product + SKUs + stock + keyword + collections; any failure leaves nothing

func TestProductEditorPE01SingleTransaction(t *testing.T) {
	e := ccNew(t)

	kw := pdKeyword()
	colA := e.collection("PE01 A", nil)
	colB := e.collection("PE01 B", nil)
	name := e.name("PE01 save")
	key := e.key("doc")
	doc := pdDoc{
		Name: name, Description: "one save", Status: "active",
		SKUs: []pdSKU{
			{PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(5)}, Keyword: kw},
			{PriceMinor: 1200, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(2)}},
			{PriceMinor: 1500, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(3)}},
		},
		CollectionIDs: []string{colA.ID, colB.ID},
		WarehouseID:   e.wh1,
	}
	var got pdProduct
	e.docCreate(key, doc, &got)
	if got.ID == "" || got.Version != 1 || got.Status != "active" {
		t.Fatalf("saved product: %+v", got)
	}
	if len(got.SKUs) != 3 {
		t.Fatalf("SKU count=%d want 3", len(got.SKUs))
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND status='active'`, got.ID) != 3 {
		t.Fatal("3 active SKUs must be written")
	}
	// opening stock: one balance + one ADJUST ledger row per tracked SKU (warehouse scoped)
	for i, s := range got.SKUs {
		want := []int64{5, 2, 3}[i]
		if on, ok := e.pdBalance(s.ID); !ok || on != want {
			t.Fatalf("SKU %d balance=(%d,%t) want (%d,true)", i, on, ok, want)
		}
		if countRows(t, e.h.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1 AND kind='ADJUST' AND command_key=$2`, s.ID, key) != 1 {
			t.Fatalf("SKU %d must have one opening ledger row", i)
		}
	}
	// keyword lands in live.keyword_library (the shared claims vocabulary)
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1 AND keyword=$2`, got.SKUs[0].ID, kw) != 1 {
		t.Fatal("keyword must be written for the first SKU")
	}
	// collection membership: the product is in both collections
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.collection_products WHERE product_id=$1`, got.ID) != 2 {
		t.Fatal("product must belong to both collections")
	}

	// Mid-flight failure rolls everything back: a bogus collection id fails AFTER the product/SKUs/stock/keyword writes,
	// so a pass here proves the single-transaction guarantee (nothing survives).
	rollKey := e.key("roll")
	rollName := e.name("PE01 rollback")
	rollKeyword := pdKeyword()
	e.docCreateRefuse(404, rollKey, pdDoc{
		Name: rollName, Description: "rolls back",
		SKUs: []pdSKU{
			{PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(10)}, Keyword: rollKeyword},
		},
		CollectionIDs: []string{randomUUID()},
		WarehouseID:   e.wh1,
	})
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE name=$1`, rollName) != 0 {
		t.Fatal("rollback left a product row")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM inventory.ledger WHERE command_key=$1`, rollKey) != 0 {
		t.Fatal("rollback left ledger rows")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE keyword=$1`, rollKeyword) != 0 {
		t.Fatal("rollback left a keyword row")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM ops.command_results WHERE idempotency_key=$1`, rollKey) != 0 {
		t.Fatal("a failed command must store no result")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.collection_products WHERE product_id IN (SELECT id FROM catalog.products WHERE name=$1)`, rollName) != 0 {
		t.Fatal("rollback left collection membership")
	}
}

// ---- PE02 idempotency: same key + same bytes replay; same key + different bytes is 409 ---------------------------------

func TestProductEditorPE02Idempotency(t *testing.T) {
	e := ccNew(t)
	key := e.key("idem")
	doc := pdDoc{Name: e.name("PE02 idem"), Description: "same", SKUs: []pdSKU{{PriceMinor: 1000}}}
	auditBefore := t04Count(t, e.h.f, `SELECT count(*) FROM ops.audit_events WHERE action='catalog.product.saved' AND store_id=$1`, e.h.f.storeA1)

	var first pdProduct
	e.docCreate(key, doc, &first)
	if first.ID == "" {
		t.Fatal("create produced no product")
	}
	var replay pdProduct
	e.docCreate(key, doc, &replay)
	if replay.ID != first.ID || len(replay.SKUs) != len(first.SKUs) {
		t.Fatalf("replay id=%q want %q (SKUs %d vs %d)", replay.ID, first.ID, len(replay.SKUs), len(first.SKUs))
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE name=$1`, doc.Name) != 1 {
		t.Fatal("replay must not create a second product")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM ops.command_results WHERE idempotency_key=$1`, key) != 1 {
		t.Fatal("one command result row for one key")
	}
	if t04Count(t, e.h.f, `SELECT count(*) FROM ops.audit_events WHERE action='catalog.product.saved' AND store_id=$1`, e.h.f.storeA1) != auditBefore+1 {
		t.Fatal("replay must not audit twice")
	}
	// same key, different bytes -> 409
	changed := doc
	changed.Description = "different"
	e.docCreateRefuse(409, key, changed)
}

// ---- PE03 keyword and slug conflicts roll back with the right code ----------------------------------------------------

func TestProductEditorPE03KeywordAndSlugConflict(t *testing.T) {
	e := ccNew(t)

	// keyword_taken: second product reuses the first's keyword -> 409 keyword_taken, and nothing is written.
	kw := pdKeyword()
	var first pdProduct
	e.docCreate(e.key("k1"), pdDoc{Name: e.name("PE03 first"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000, Keyword: kw}}}, &first)
	takenName := e.name("PE03 taken")
	e.refuseCode(409, "keyword_taken", "POST", "/products/document", e.key("k2"), pdDoc{
		Name: takenName, Description: "",
		SKUs: []pdSKU{
			{PriceMinor: 1000, Keyword: pdKeyword()}, // a fresh keyword for SKU 1 (would succeed)
			{PriceMinor: 2000, Keyword: kw},          // taken -> whole command rolls back
		},
	})
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE name=$1`, takenName) != 0 {
		t.Fatal("keyword conflict must roll back the product")
	}

	// slug conflict: an explicit slug that is already taken -> 409, whole command rolls back.
	e.docCreateRefuse(409, e.key("slug"), pdDoc{
		Name: e.name("PE03 slug"), Description: "", Slug: first.Slug,
		SKUs: []pdSKU{{PriceMinor: 1000}},
	})
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE slug=$1`, first.Slug) != 1 {
		t.Fatal("slug conflict must not create a product")
	}
}

// ---- PE04 TWD whole-dollar: 60 -> 6000 ok; 60.5 (6050) refused amount_not_whole_twd -----------------------------------

func TestProductEditorPE04TWDWholeDollar(t *testing.T) {
	e := ccNew(t)
	b := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}

	// whole dollar 6000 -> stored as 6000
	var ok pdProduct
	status, body := b.call("POST", "/products/document", e.key("twd"), pdDoc{
		Name: e.name("PE04 whole"), Description: "", SKUs: []pdSKU{{PriceMinor: 6000}},
	})
	if status != 200 {
		t.Fatalf("whole TWD status=%d body=%s", status, body)
	}
	if err := json.Unmarshal(body, &ok); err != nil || len(ok.SKUs) != 1 || ok.SKUs[0].PriceMinor != 6000 {
		t.Fatalf("whole TWD result=%+v err=%v", ok, err)
	}
	// 60.5 -> 6050 -> refused with amount_not_whole_twd
	e.refuseCodeTWD(t, b, 422, "amount_not_whole_twd", pdDoc{
		Name: e.name("PE04 frac"), Description: "", SKUs: []pdSKU{{PriceMinor: 6050}},
	})
	// compare-at non-whole is refused too
	e.refuseCodeTWD(t, b, 422, "amount_not_whole_twd", pdDoc{
		Name: e.name("PE04 cmp"), Description: "",
		SKUs: []pdSKU{{PriceMinor: 6000, CompareAtMinor: pdI64(6150)}},
	})
}

// refuseCodeTWD posts a document to storeB (TWD) and asserts the exact status + code.
func (e *ccEnv) refuseCodeTWD(t *testing.T, b ccAdmin, want int, code string, in pdDoc) {
	t.Helper()
	status, body := b.call("POST", "/products/document", e.key("twd"), in)
	if status != want {
		t.Fatalf("TWD status=%d want %d body=%s", status, want, body)
	}
	var env httperror.Envelope
	if err := json.Unmarshal(body, &env); err != nil || env.Code != code {
		t.Fatalf("TWD code=%q err=%v want %q body=%s", env.Code, err, code, body)
	}
}

// ---- PE05 untracked/tracked flags + the A6 CHECK (K3 does the concurrency) -------------------------------------------

func TestProductEditorPE05UntrackedTrackedFlags(t *testing.T) {
	e := ccNew(t)
	var got pdProduct
	e.docCreate(e.key("p"), pdDoc{
		Name: e.name("PE05 flags"), Description: "", WarehouseID: e.wh1,
		SKUs: []pdSKU{
			{PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(7)}},
			{PriceMinor: 1200, Stock: &pdStock{Mode: "untracked", MaxPerOrder: pdI64(5)}},
		},
	}, &got)
	if len(got.SKUs) != 2 {
		t.Fatalf("SKU count=%d", len(got.SKUs))
	}
	tracked, untracked := got.SKUs[0], got.SKUs[1]
	if !tracked.InventoryTracked || tracked.MaxPerOrder != nil {
		t.Fatalf("tracked SKU flags: %+v", tracked)
	}
	if untracked.InventoryTracked || untracked.MaxPerOrder == nil || *untracked.MaxPerOrder != 5 {
		t.Fatalf("untracked SKU flags: %+v", untracked)
	}
	if on, ok := e.pdBalance(tracked.ID); !ok || on != 7 {
		t.Fatalf("tracked SKU balance=(%d,%t) want (7,true)", on, ok)
	}
	if _, ok := e.pdBalance(untracked.ID); ok {
		t.Fatal("an untracked SKU must never have a balance row")
	}

	// Go validation: tracked SKUs carry no per-order cap; untracked require it 1..999 and never carry a quantity.
	e.docCreateRefuse(422, e.key("p"), pdDoc{Name: e.name("PE05 tracked cap"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", MaxPerOrder: pdI64(5)}}}})
	e.docCreateRefuse(422, e.key("p"), pdDoc{Name: e.name("PE05 untracked nolimit"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000, Stock: &pdStock{Mode: "untracked"}}}})
	e.docCreateRefuse(422, e.key("p"), pdDoc{Name: e.name("PE05 untracked qty"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000, Stock: &pdStock{Mode: "untracked", MaxPerOrder: pdI64(5), OpeningQty: pdI64(1)}}}})
	e.docCreateRefuse(422, e.key("p"), pdDoc{Name: e.name("PE05 cap zero"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000, Stock: &pdStock{Mode: "untracked", MaxPerOrder: pdI64(0)}}}})
	e.docCreateRefuse(422, e.key("p"), pdDoc{Name: e.name("PE05 cap big"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000, Stock: &pdStock{Mode: "untracked", MaxPerOrder: pdI64(1000)}}}})

	// DB CHECK is the authority (migration 0109): a tracked SKU must never carry a cap, and an untracked SKU's cap must
	// be inside 1..999 — both rejected as SQLSTATE 23514. (The CHECK is the §f literal; because an SQL CHECK treats
	// NULL as pass, "untracked with a NULL cap" is left to the Go validation above, exactly as the migration documents.)
	for name, sql := range map[string]string{
		"tracked with cap":           `UPDATE catalog.skus SET max_per_order=5 WHERE id=$1`,
		"untracked cap out of range": `UPDATE catalog.skus SET inventory_tracked=false, max_per_order=0 WHERE id=$1`,
	} {
		if _, err := e.h.f.owner.Exec(context.Background(), sql, tracked.ID); sqlState(err) != "23514" {
			t.Fatalf("%s: sqlstate=%q err=%v want 23514", name, sqlState(err), err)
		}
	}
}

// ---- PE06 image cap 12: the 12th upload succeeds, the 13th is refused, order persists ---------------------------------

func TestProductEditorPE06ImagesCap(t *testing.T) {
	e := ccNew(t)
	var p ccProduct
	e.a.ok("POST", "/products", e.key("p"), map[string]any{"name": e.name("PE06 images")}, &p)
	for i := 0; i < 12; i++ {
		if st, body := e.upload("/products/"+p.ID+"/images", ccPNG(uint8(i+1)), e.key("img")); st != 200 {
			t.Fatalf("image %d status=%d body=%s", i+1, st, body)
		}
	}
	if st, body := e.upload("/products/"+p.ID+"/images", ccPNG(99), e.key("img")); st != 409 {
		t.Fatalf("13th image status=%d want 409 body=%s", st, body)
	}
	var imgs struct {
		Items []struct {
			Position int `json:"position"`
		} `json:"items"`
	}
	e.a.ok("GET", "/products/"+p.ID+"/images", "", nil, &imgs)
	if len(imgs.Items) != 12 {
		t.Fatalf("image count=%d want 12", len(imgs.Items))
	}
	for i, im := range imgs.Items {
		if im.Position != i {
			t.Fatalf("image %d position=%d want %d (order must persist)", i, im.Position, i)
		}
	}
}

// ---- PE07 draft visibility: a document draft is invisible to buyers, becomes visible on publish, hidden again on unlist

func TestProductEditorPE07DraftVisibility(t *testing.T) {
	e := ccNew(t)
	doc := pdDoc{Name: e.name("PE07 draft"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}
	var p pdProduct
	e.docCreate(e.key("p"), doc, &p)
	if p.Status != "draft" {
		t.Fatalf("create without status must be a draft: %+v", p)
	}
	// merchant reads its own draft
	if st, _ := e.a.call("GET", "/products/"+p.ID, "", nil); st != 200 {
		t.Fatalf("merchant draft read: %d", st)
	}
	assertHidden := func(label string) {
		t.Helper()
		for _, key := range []string{p.ID, p.Slug} {
			if _, st := e.detail(key); st != 404 {
				t.Fatalf("%s: buyer detail by %q status=%d want 404", label, key, st)
			}
		}
		if ccHas(e.list("q="+e.tag), p.ID) {
			t.Fatalf("%s: draft must not be in the buyer list", label)
		}
	}
	assertHidden("draft")

	// publish (merge-patch edit: present fields set, the SKU price present keeps it active) -> visible
	edit := pdPatch{
		ID: p.ID, Name: pdStr(doc.Name), Description: pdStr(doc.Description), Status: pdStr("active"), ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: p.SKUs[0].ID, PriceMinor: pdI64(1000)}},
	}
	var active pdProduct
	e.docEdit(p.ID, e.key("edit"), edit, &active)
	if active.Status != "active" || active.Version != 2 {
		t.Fatalf("publish: %+v", active)
	}
	if _, st := e.detail(active.ID); st != 200 {
		t.Fatalf("active buyer detail by id: %d", st)
	}
	if _, st := e.detail(active.Slug); st != 200 {
		t.Fatalf("active buyer detail by slug: %d", st)
	}
	if !ccHas(e.list("q="+e.tag), active.ID) {
		t.Fatal("active product must be in the buyer list")
	}

	// back to draft -> hidden again
	back := edit
	back.Status = pdStr("draft")
	back.ExpectedVersion = active.Version
	back.SKUs = &[]pdSKUPatch{{ID: p.SKUs[0].ID, PriceMinor: pdI64(1000)}}
	e.docEdit(p.ID, e.key("unlist"), back, &active)
	assertHidden("re-drafted")
}

// ---- PE08 axis change archives the explicitly-retired SKUs and the freed combination can be recreated -----------------

func TestProductEditorPE08AxisChangeArchives(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{
		Name: e.name("PE08 axis"), Description: "", WarehouseID: e.wh1,
		Options: []ccAxis{{Name: "Color", Values: []string{"Red", "Blue"}}},
		SKUs: []pdSKU{
			{OptionValues: []string{"Red"}, PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(1)}},
			{OptionValues: []string{"Blue"}, PriceMinor: 1100, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(1)}},
		},
	}, &p)
	if len(p.SKUs) != 2 {
		t.Fatalf("initial SKU count=%d", len(p.SKUs))
	}
	red, blue := p.SKUs[0].ID, p.SKUs[1].ID

	// axis change to a single Green value: Red and Blue must be explicitly archived (merge-patch: omission alone keeps
	// them, and they would no longer fit the new axes, so §g.1 requires active:false — the stranded case is PE20).
	var after pdProduct
	e.docEdit(p.ID, e.key("axis"), pdPatch{
		ID: p.ID, Name: pdStr(e.name("PE08 axis")), Description: pdStr(""), ExpectedVersion: p.Version,
		Options: &[]ccAxis{{Name: "Color", Values: []string{"Green"}}},
		SKUs: &[]pdSKUPatch{
			{ID: red, Active: pdBool(false)},
			{ID: blue, Active: pdBool(false)},
			{OptionValues: &[]string{"Green"}, PriceMinor: pdI64(1200)},
		},
	}, &after)
	if len(after.Options) != 1 || len(after.Options[0].Values) != 1 || after.Options[0].Values[0] != "Green" {
		t.Fatalf("options after axis change: %+v", after.Options)
	}
	for id, label := range map[string]string{red: "Red", blue: "Blue"} {
		got := countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.skus WHERE id=$1 AND status='archived'`, id)
		if got != 1 {
			t.Fatalf("%s SKU must be archived (soft-deleted, row kept): %d", label, got)
		}
	}
	if len(after.SKUs) != 1 || after.SKUs[0].OptionValues[0] != "Green" {
		t.Fatalf("active SKUs after axis change: %+v", after.SKUs)
	}

	// the freed combination can be recreated (new SKU id, not the archived one).
	var rebuilt pdProduct
	e.docEdit(p.ID, e.key("recreate"), pdPatch{
		ID: p.ID, Name: pdStr(e.name("PE08 axis")), Description: pdStr(""), ExpectedVersion: after.Version,
		Options: &[]ccAxis{{Name: "Color", Values: []string{"Green", "Red"}}},
		SKUs: &[]pdSKUPatch{
			{ID: after.SKUs[0].ID, OptionValues: &[]string{"Green"}, PriceMinor: pdI64(1200)},
			{OptionValues: &[]string{"Red"}, PriceMinor: pdI64(1300)},
		},
	}, &rebuilt)
	if len(rebuilt.SKUs) != 2 {
		t.Fatalf("rebuilt SKU count=%d", len(rebuilt.SKUs))
	}
	for _, s := range rebuilt.SKUs {
		if s.OptionValues[0] == "Red" && s.ID == red {
			t.Fatal("the recreated Red SKU must be a new id, not the archived one")
		}
	}
}

// ---- PE09 stale expected_version is 409, never a silent overwrite -----------------------------------------------------

func TestProductEditorPE09StaleVersion(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("PE09 v1"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	var v2 pdProduct
	e.docEdit(p.ID, e.key("edit"), pdPatch{
		ID: p.ID, Name: pdStr(e.name("PE09 v2")), Description: pdStr(""), ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: p.SKUs[0].ID, PriceMinor: pdI64(1000)}},
	}, &v2)
	if v2.Version != 2 {
		t.Fatalf("edit version=%d want 2", v2.Version)
	}
	// the now-stale version 1 must be refused
	e.docEditRefuse(409, p.ID, e.key("stale"), pdPatch{
		ID: p.ID, Name: pdStr(e.name("PE09 stale")), Description: pdStr(""), ExpectedVersion: 1,
		SKUs: &[]pdSKUPatch{{ID: p.SKUs[0].ID, PriceMinor: pdI64(1000)}},
	})
	var cur struct {
		Name    string `json:"name"`
		Version int64  `json:"version"`
	}
	e.a.ok("GET", "/products/"+p.ID, "", nil, &cur)
	if cur.Version != 2 || cur.Name != e.name("PE09 v2") {
		t.Fatalf("a stale edit must not overwrite: name=%q version=%d", cur.Name, cur.Version)
	}
}

// ---- PE10 isolation: another tenant/store cannot edit or read these rows (404) ----------------------------------------

func TestProductEditorPE10Isolation(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("PE10 mine"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	other := ccAdmin{t: t, h: e.srv, token: e.h.f.tokens["b"], store: e.h.f.storeB}
	other.refuse(404, "PUT", "/products/"+p.ID+"/document", e.key("x"), pdPatch{
		ID: p.ID, Name: pdStr(e.name("PE10 foreign")), Description: pdStr(""), ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: p.SKUs[0].ID, PriceMinor: pdI64(1000)}},
	})
	other.refuse(404, "GET", "/products/"+p.ID, "", nil)
}

// ---- copy (product-editor §f ruling 5): draft, name +「（复制）」, fresh codes, no images/keywords ----------------------

func TestProductEditorCopyCommand(t *testing.T) {
	e := ccNew(t)
	var src pdProduct
	e.docCreate(e.key("p"), pdDoc{
		Name: e.name("PE copy source"), Description: "", Status: "active", WarehouseID: e.wh1,
		SKUs: []pdSKU{
			{PriceMinor: 1000, Keyword: pdKeyword(), Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(3)}},
			{PriceMinor: 1200, Stock: &pdStock{Mode: "untracked", MaxPerOrder: pdI64(4)}},
		},
	}, &src)
	if st, _ := e.upload("/products/"+src.ID+"/images", ccPNG(1), e.key("img")); st != 200 {
		t.Fatalf("source image upload: %d", st)
	}

	var copy pdProduct
	e.a.ok("POST", "/products/"+src.ID+"/copy", e.key("copy"), map[string]any{"expected_version": src.Version}, &copy)
	if copy.ID == src.ID || copy.ID == "" {
		t.Fatalf("copy id %q must be fresh (source %q)", copy.ID, src.ID)
	}
	if copy.Status != "draft" || copy.Version != 1 {
		t.Fatalf("copy must be a fresh draft: %+v", copy)
	}
	if copy.Name != src.Name+"（复制）" {
		t.Fatalf("copy name %q want %q", copy.Name, src.Name+"（复制）")
	}
	if copy.Slug == src.Slug {
		t.Fatal("copy must have a fresh slug")
	}
	// The copy response is a Product (no SKUs); assert regenerated codes from the SKU table directly.
	srcCodes := map[string]bool{}
	rows, err := e.h.f.owner.Query(context.Background(), `SELECT code FROM catalog.skus WHERE product_id=$1`, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		srcCodes[c] = true
	}
	rows.Close()
	copyRows, err := e.h.f.owner.Query(context.Background(), `SELECT code FROM catalog.skus WHERE product_id=$1`, copy.ID)
	if err != nil {
		t.Fatal(err)
	}
	var copyCount int
	for copyRows.Next() {
		var c string
		if err := copyRows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		copyCount++
		if srcCodes[c] {
			t.Fatalf("copy SKU code %q must be regenerated, not reused", c)
		}
	}
	copyRows.Close()
	if copyCount != len(srcCodes) {
		t.Fatalf("copy SKU count=%d want %d", copyCount, len(srcCodes))
	}
	// no images and no keywords are copied
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, copy.ID) != 0 {
		t.Fatal("copy must not carry images")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library kl WHERE kl.sku_id IN (SELECT id FROM catalog.skus WHERE product_id=$1)`, copy.ID) != 0 {
		t.Fatal("copy must not carry keywords")
	}
}

// ---- PE11 bulk status: 100 ids, per-item result, live_window_open per item, others proceed ----------------------------

func TestProductEditorPE11BulkStatus(t *testing.T) {
	e := ccNew(t)

	mk := func(title string) pdProduct {
		var out pdProduct
		e.docCreate(e.key("p"), pdDoc{Name: e.name(title), Description: "", Status: "active", SKUs: []pdSKU{{PriceMinor: 1000}}}, &out)
		return out
	}
	p1, p2, p3 := mk("PE11 one"), mk("PE11 two"), mk("PE11 three")
	e.seedLiveWindow(p1.SKUs[0].ID)

	var items []pdBulkItem
	e.a.ok("POST", "/products/bulk-status", e.key("bulk"), map[string]any{"ids": []string{p1.ID, p2.ID, p3.ID}, "status": "draft"}, &items)
	if len(items) != 3 {
		t.Fatalf("bulk item count=%d want 3", len(items))
	}
	byID := map[string]pdBulkItem{}
	for _, it := range items {
		byID[it.ID] = it
	}
	if byID[p1.ID].Err != "live_window_open" || byID[p1.ID].Status != "" {
		t.Fatalf("open-window product must be refused per item: %+v", byID[p1.ID])
	}
	if byID[p2.ID].Err != "" || byID[p2.ID].Status != "draft" || byID[p3.ID].Err != "" || byID[p3.ID].Status != "draft" {
		t.Fatalf("the others must proceed: %+v %+v", byID[p2.ID], byID[p3.ID])
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE id=$1 AND status='draft'`, p1.ID) != 0 {
		t.Fatal("the refused product must stay active")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.products WHERE id=$1 AND status='draft'`, p2.ID) != 1 {
		t.Fatal("p2 must be drafted")
	}

	// a foreign id is a per-item not_found, not a whole-command failure
	var mixed []pdBulkItem
	e.a.ok("POST", "/products/bulk-status", e.key("bulk"), map[string]any{"ids": []string{p2.ID, randomUUID()}, "status": "active"}, &mixed)
	if len(mixed) != 2 || mixed[0].Err != "" || mixed[1].Err != "not_found" {
		t.Fatalf("per-item not_found: %+v", mixed)
	}

	// the 100 bound: 101 ids are 422, 100 ids (all foreign) run to completion with 100 per-item results
	many := make([]string, 101)
	for i := range many {
		many[i] = randomUUID()
	}
	e.a.refuse(422, "POST", "/products/bulk-status", e.key("bulk"), map[string]any{"ids": many, "status": "draft"})
	var hundred []pdBulkItem
	e.a.ok("POST", "/products/bulk-status", e.key("bulk"), map[string]any{"ids": many[:100], "status": "draft"}, &hundred)
	if len(hundred) != 100 {
		t.Fatalf("100-id bulk returned %d items", len(hundred))
	}
	for _, it := range hundred {
		if it.Err != "not_found" {
			t.Fatalf("foreign id item=%+v want not_found", it)
		}
	}
	// duplicates and empty are invalid
	e.a.refuse(422, "POST", "/products/bulk-status", e.key("bulk"), map[string]any{"ids": []string{p2.ID, p2.ID}, "status": "draft"})
	e.a.refuse(422, "POST", "/products/bulk-status", e.key("bulk"), map[string]any{"ids": []string{}, "status": "draft"})
}

// ---- list additions (§f ruling 5): status counts, total, keyword, tracked flag, updated_at ----------------------------

func TestProductEditorListSummaries(t *testing.T) {
	e := ccNew(t)
	kw := pdKeyword()
	var active pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("PE list active"), Description: "", Status: "active", SKUs: []pdSKU{{PriceMinor: 1000, Keyword: kw}}}, &active)
	var draft pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("PE list draft"), Description: "", SKUs: []pdSKU{{PriceMinor: 2000}}}, &draft)

	list := func(q string) pdList {
		t.Helper()
		var out pdList
		e.a.ok("GET", "/catalog-products?"+q, "", nil, &out)
		return out
	}
	all := list("q=" + e.tag)
	if all.Total != 2 || all.StatusCounts.Draft != 1 || all.StatusCounts.Active != 1 || all.StatusCounts.Archived != 0 {
		t.Fatalf("tallies (search-scoped): %+v", all)
	}
	if len(all.Items) != 2 {
		t.Fatalf("items=%d want 2", len(all.Items))
	}
	byID := map[string]struct {
		Keyword          string
		InventoryTracked bool
		UpdatedAt        string
	}{}
	for _, it := range all.Items {
		byID[it.ID] = struct {
			Keyword          string
			InventoryTracked bool
			UpdatedAt        string
		}{it.Keyword, it.InventoryTracked, it.UpdatedAt}
	}
	if got := byID[active.ID]; got.Keyword != kw || !got.InventoryTracked || got.UpdatedAt == "" {
		t.Fatalf("active summary keyword/tracked/updated_at: %+v", got)
	}
	if got := byID[draft.ID]; got.Keyword != "" || !got.InventoryTracked || got.UpdatedAt == "" {
		t.Fatalf("draft summary keyword/tracked/updated_at: %+v", got)
	}
	// the status filter narrows items but not the tallies
	onlyActive := list("q=" + e.tag + "&status=active")
	if len(onlyActive.Items) != 1 || onlyActive.Items[0].ID != active.ID {
		t.Fatalf("status filter: %+v", onlyActive.Items)
	}
	if onlyActive.Total != 2 || onlyActive.StatusCounts.Active != 1 {
		t.Fatalf("tallies must ignore the status filter: %+v", onlyActive)
	}
}

// ---- PE18 merge-patch: a price-only edit leaves every other field, keyword, collection and SKU untouched ---------------

func TestProductEditorPE18PriceOnlyEditPreservesEverything(t *testing.T) {
	e := ccNew(t)
	kw := pdKeyword()
	col := e.collection("PE18 col", nil)
	doc := pdDoc{
		Name: e.name("PE18 keep"), Description: "desc", Status: "active",
		Options:       []ccAxis{{Name: "Color", Values: []string{"Red", "Blue"}}},
		CollectionIDs: []string{col.ID},
		WarehouseID:   e.wh1,
		SKUs: []pdSKU{
			{OptionValues: []string{"Red"}, PriceMinor: 1000, Keyword: kw, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(5)}, OriginCountry: "US"},
			{OptionValues: []string{"Blue"}, PriceMinor: 2000, Stock: &pdStock{Mode: "untracked", MaxPerOrder: pdI64(3)}},
		},
	}
	var p pdProduct
	e.docCreate(e.key("p"), doc, &p)
	red, blue := p.SKUs[0].ID, p.SKUs[1].ID

	var edited pdProduct
	e.docEdit(p.ID, e.key("edit"), pdPatch{ID: p.ID, ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: red, PriceMinor: pdI64(1500)}}}, &edited)
	if edited.Version != 2 {
		t.Fatalf("price-only edit version=%d want 2", edited.Version)
	}

	var d pdDetail
	e.a.ok("GET", "/products/"+p.ID, "", nil, &d)
	if d.Name != doc.Name || d.Description != "desc" || d.Status != "active" {
		t.Fatalf("price-only edit changed name/desc/status: %+v", d)
	}
	if len(d.Options) != 1 || d.Options[0].Name != "Color" || len(d.Options[0].Values) != 2 {
		t.Fatalf("price-only edit changed options: %+v", d.Options)
	}
	if len(d.CollectionIDs) != 1 || d.CollectionIDs[0] != col.ID {
		t.Fatalf("price-only edit changed collections: %+v", d.CollectionIDs)
	}
	byID := map[string]pdDetailSKU{}
	for _, s := range d.SKUs {
		byID[s.ID] = s
	}
	if byID[red].PriceMinor != 1500 {
		t.Fatalf("red price=%d want 1500", byID[red].PriceMinor)
	}
	if byID[blue].PriceMinor != 2000 || byID[blue].Keyword != "" {
		t.Fatalf("other SKU must be untouched: %+v", byID[blue])
	}
	if byID[red].Keyword != kw {
		t.Fatalf("red keyword must be preserved: %q want %q", byID[red].Keyword, kw)
	}
	// the keyword row and the other SKU's flags/stock are untouched
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1 AND keyword=$2`, red, kw) != 1 {
		t.Fatal("red keyword row must survive a price-only edit")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.skus WHERE id=$1 AND price_minor=2000 AND inventory_tracked=false AND max_per_order=3`, blue) != 1 {
		t.Fatal("the other SKU's price/flags must be untouched")
	}
	if on, ok := e.pdBalance(red); !ok || on != 5 {
		t.Fatalf("red balance=(%d,%t) want (5,true)", on, ok)
	}
}

// ---- PE19 target_qty present (incl 0) sets on-hand; below the committed amount is insufficient -------------------------

func TestProductEditorPE19TargetQtyZero(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("PE19 target"), Description: "", WarehouseID: e.wh1,
		SKUs: []pdSKU{{PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(10)}}}}, &p)
	sku := p.SKUs[0].ID
	if on, ok := e.pdBalance(sku); !ok || on != 10 {
		t.Fatalf("opening balance=(%d,%t) want (10,true)", on, ok)
	}
	e.onlyWarehouse(e.wh1) // target_qty resolves the store's single active warehouse

	// target_qty present, including 0, sets the final on-hand (§g.2 fixes the old *qty != 0 skip).
	var v2 pdProduct
	e.docEdit(p.ID, e.key("zero"), pdPatch{ID: p.ID, ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: sku, Stock: &pdStock{TargetQty: pdI64(0)}}}}, &v2)
	if on, ok := e.pdBalance(sku); !ok || on != 0 {
		t.Fatalf("after target_qty 0 balance=(%d,%t) want (0,true)", on, ok)
	}

	// Restore 10, reserve 3 out-of-band, then try to target below the committed 3 -> insufficient, whole rollback.
	e.docEdit(p.ID, e.key("up"), pdPatch{ID: p.ID, ExpectedVersion: v2.Version,
		SKUs: &[]pdSKUPatch{{ID: sku, Stock: &pdStock{TargetQty: pdI64(10)}}}}, &v2)
	mustExec(t, e.h.f.owner, `UPDATE inventory.balances SET reserved=3 WHERE warehouse_id=$1 AND sku_id=$2`, e.wh1, sku)
	e.refuseCode(409, "insufficient_inventory", "PUT", "/products/"+p.ID+"/document", e.key("low"),
		pdPatch{ID: p.ID, ExpectedVersion: v2.Version,
			SKUs: &[]pdSKUPatch{{ID: sku, Stock: &pdStock{TargetQty: pdI64(2)}}}})
	if on, _ := e.pdBalance(sku); on != 10 {
		t.Fatalf("insufficient edit must not change on_hand: %d want 10", on)
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM inventory.ledger WHERE sku_id=$1 AND command_key=$2`, sku, "low") != 0 {
		t.Fatal("the insufficient edit must leave no ledger rows")
	}
}

// ---- PE20 changing axes without retiring the stranded SKUs is invalid and rolls back -----------------------------------

func TestProductEditorPE20AxisChangeRequiresExplicitArchive(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{
		Name: e.name("PE20 axis"), Description: "", WarehouseID: e.wh1,
		Options: []ccAxis{{Name: "Color", Values: []string{"Red", "Blue"}}},
		SKUs: []pdSKU{
			{OptionValues: []string{"Red"}, PriceMinor: 1000, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(1)}},
			{OptionValues: []string{"Blue"}, PriceMinor: 1100, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(1)}},
		},
	}, &p)

	// options -> Green while Red/Blue stay active (no active:false): they no longer fit -> invalid, whole rollback.
	e.docEditRefuse(422, p.ID, e.key("axis"), pdPatch{ID: p.ID, ExpectedVersion: p.Version,
		Options: &[]ccAxis{{Name: "Color", Values: []string{"Green"}}},
		SKUs:    &[]pdSKUPatch{{OptionValues: &[]string{"Green"}, PriceMinor: pdI64(1200)}},
	})
	var d pdDetail
	e.a.ok("GET", "/products/"+p.ID, "", nil, &d)
	if len(d.Options) != 1 || len(d.Options[0].Values) != 2 || d.Options[0].Values[0] != "Red" {
		t.Fatalf("PE20 rollback must keep the old axes: %+v", d.Options)
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND status='active'`, p.ID) != 2 {
		t.Fatal("PE20 rollback must keep both active SKUs")
	}
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM catalog.skus WHERE product_id=$1 AND status='archived'`, p.ID) != 0 {
		t.Fatal("PE20 must not archive anything")
	}
}

// ---- PE21 open live window: keyword touch / active:false / unlist are refused, a price change is allowed ---------------

func TestProductEditorPE21LiveWindowEditRefusals(t *testing.T) {
	e := ccNew(t)
	kw := pdKeyword()
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("PE21 live"), Description: "", Status: "active",
		SKUs: []pdSKU{{PriceMinor: 1000, Keyword: kw}, {PriceMinor: 2000}}}, &p)
	skuA := p.SKUs[0].ID
	e.seedLiveWindow(skuA)

	// clearing the windowed SKU's keyword -> live_window_open
	e.refuseCode(409, "live_window_open", "PUT", "/products/"+p.ID+"/document", e.key("kw"),
		pdPatch{ID: p.ID, ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: skuA, Keyword: pdStr("")}}})
	// archiving the windowed SKU -> live_window_open
	e.refuseCode(409, "live_window_open", "PUT", "/products/"+p.ID+"/document", e.key("arch"),
		pdPatch{ID: p.ID, ExpectedVersion: p.Version, SKUs: &[]pdSKUPatch{{ID: skuA, Active: pdBool(false)}}})
	// unlisting the product -> live_window_open
	e.refuseCode(409, "live_window_open", "PUT", "/products/"+p.ID+"/document", e.key("unlist"),
		pdPatch{ID: p.ID, ExpectedVersion: p.Version, Status: pdStr("draft")})

	// a price change on the windowed SKU is allowed (§g.5: the live price is per-session anyway)
	var v2 pdProduct
	e.docEdit(p.ID, e.key("price"), pdPatch{ID: p.ID, ExpectedVersion: p.Version,
		SKUs: &[]pdSKUPatch{{ID: skuA, PriceMinor: pdI64(1500)}}}, &v2)
	for _, s := range v2.SKUs {
		if s.ID == skuA && s.PriceMinor != 1500 {
			t.Fatalf("windowed SKU price=%d want 1500", s.PriceMinor)
		}
	}
	// the keyword row survived the refused edits
	if countRows(t, e.h.f.owner, `SELECT count(*) FROM live.keyword_library WHERE sku_id=$1 AND keyword=$2`, skuA, kw) != 1 {
		t.Fatal("the refused edits must not have cleared the keyword")
	}
}

// ---- PE22 detail: keyword, collection_ids, warehouse_id, on_hand, committed (null when >1 warehouse) -------------------

func TestProductEditorPE22DetailFields(t *testing.T) {
	e := ccNew(t)
	kw := pdKeyword()
	col := e.collection("PE22 col", nil)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{
		Name: e.name("PE22 detail"), Description: "", Status: "active",
		CollectionIDs: []string{col.ID}, WarehouseID: e.wh1,
		SKUs: []pdSKU{{PriceMinor: 1000, Keyword: kw, Stock: &pdStock{Mode: "tracked", OpeningQty: pdI64(7)}}},
	}, &p)

	// two active warehouses (ccNew default): warehouse_id null, on_hand/committed null, available still summed.
	var d pdDetail
	e.a.ok("GET", "/products/"+p.ID, "", nil, &d)
	if len(d.CollectionIDs) != 1 || d.CollectionIDs[0] != col.ID {
		t.Fatalf("collection_ids: %+v want [%s]", d.CollectionIDs, col.ID)
	}
	if d.WarehouseID != nil {
		t.Fatalf("two warehouses must read warehouse_id null, got %q", *d.WarehouseID)
	}
	if len(d.SKUs) != 1 {
		t.Fatalf("SKU count=%d want 1", len(d.SKUs))
	}
	if d.SKUs[0].Keyword != kw {
		t.Fatalf("SKU keyword=%q want %q", d.SKUs[0].Keyword, kw)
	}
	if d.SKUs[0].OnHand != nil || d.SKUs[0].Committed != nil {
		t.Fatalf("multi-warehouse on_hand/committed must be null: %+v", d.SKUs[0])
	}
	if d.SKUs[0].Available != 7 {
		t.Fatalf("available=%d want 7", d.SKUs[0].Available)
	}

	// exactly one active warehouse: warehouse_id is that warehouse, on_hand/committed read from it.
	e.onlyWarehouse(e.wh1)
	var d1 pdDetail
	e.a.ok("GET", "/products/"+p.ID, "", nil, &d1)
	if d1.WarehouseID == nil || *d1.WarehouseID != e.wh1 {
		t.Fatalf("single warehouse must read %q, got %v", e.wh1, d1.WarehouseID)
	}
	if d1.SKUs[0].OnHand == nil || *d1.SKUs[0].OnHand != 7 {
		t.Fatalf("on_hand=%v want 7", d1.SKUs[0].OnHand)
	}
	if d1.SKUs[0].Committed == nil || *d1.SKUs[0].Committed != 0 {
		t.Fatalf("committed=%v want 0", d1.SKUs[0].Committed)
	}
}

// ---- PE23 top-level logistics in an edit request are invalid -----------------------------------------------------------

func TestProductEditorPE23TopLevelLogisticsInvalid(t *testing.T) {
	e := ccNew(t)
	var p pdProduct
	e.docCreate(e.key("p"), pdDoc{Name: e.name("PE23 logistics"), Description: "", SKUs: []pdSKU{{PriceMinor: 1000}}}, &p)

	e.docEditRefuse(422, p.ID, e.key("w"), pdPatch{ID: p.ID, ExpectedVersion: p.Version, WeightGrams: pdI64(100)})
	e.docEditRefuse(422, p.ID, e.key("l"), pdPatch{ID: p.ID, ExpectedVersion: p.Version, LengthMM: pdI64(10)})
	e.docEditRefuse(422, p.ID, e.key("wi"), pdPatch{ID: p.ID, ExpectedVersion: p.Version, WidthMM: pdI64(10)})
	e.docEditRefuse(422, p.ID, e.key("h"), pdPatch{ID: p.ID, ExpectedVersion: p.Version, HeightMM: pdI64(10)})
}
