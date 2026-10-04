package foundation_test

// merchant_tools_csv_smoke_test.go: author smoke of unit merchant-tools G2 (contracts/storefront-v2.md, product CSV export and all-or-nothing
// import) through the REAL merchant handler (httpapi) on real PostgreSQL. Evidence label: REAL_PG (isolated disposable PG, synthetic store).
// The independent MT gate tests are written by the tester from the contract.
//   MTC01 TestMerchantToolsCSVImportLifecycle   preview writes nothing -> commit creates (draft default, stock only through the adjustment
//                                               path with the reason, collections appended) -> same file twice replays -> update file
//                                               (price, new variant, stock delta, rename) -> never deletes -> export round trip is a no-op
//   MTC02 TestMerchantToolsCSVAllOrNothing      every row error listed with its row; a commit with one bad row writes nothing
//   MTC03 TestMerchantToolsCSVBoundaries        BOM, formula guard round trip, 2 MiB / 5,000 row limits, media type, authority, stock below reserved
//   MTC04 TestMerchantToolsCSVScale             5,000 products in one transaction: time and result (measurement, label NOT a promise)
// Owner-pool writes (disclosed fixtures): a principal with chosen grants and a session (as tcvEnv.member does); every other write goes
// through the real routes.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/inventory"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

type mtAdmin struct {
	t     *testing.T
	h     http.Handler
	store string
	token string
	pool  *pgxpool.Pool // diagnostics only: when a route answers 503 the same call is repeated in process to print the real error
}

func (a mtAdmin) call(method, path, key, ctype string, body []byte) *httptest.ResponseRecorder {
	a.t.Helper()
	var reader io.Reader // a GET must carry no body at all (the real server hands the handler http.NoBody)
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	r := httptest.NewRequest(method, "/v1/admin/stores/"+a.store+path, reader)
	if a.token != "" {
		r.Header.Set("Authorization", "Bearer "+a.token)
	}
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, r)
	return w
}

func (a mtAdmin) with(token string) mtAdmin { a.token = token; return a }

// importCSV posts a file to preview or commit and decodes the ImportResult (also present in a 422 body).
func (a mtAdmin) importCSV(commit bool, csv string) (int, merchanttools.ImportResult) {
	a.t.Helper()
	mode := "preview"
	if commit {
		mode = "commit"
	}
	w := a.call("POST", "/products/import/"+mode, "", "text/csv; charset=utf-8", []byte(csv))
	var res merchanttools.ImportResult
	if w.Code == 503 && a.pool != nil {
		err := platform.WithScopeBudget(context.Background(), a.pool, a.token, a.store, "catalog:write", 60*time.Second, func(tx pgx.Tx, s platform.Scope) error {
			_, inner := merchanttools.ImportProducts(context.Background(), tx, s, []byte(csv), false)
			return inner
		})
		a.t.Logf("diagnose import 503: in-process preview err=%v sqlstate=%s", err, sqlState(err))
	}
	if w.Code == 200 || w.Code == 422 {
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			a.t.Fatalf("%s: %v body=%s", mode, err, w.Body.String())
		}
	}
	return w.Code, res
}

func (a mtAdmin) jsonOK(method, path, key string, payload any, want int) map[string]any {
	a.t.Helper()
	raw, _ := json.Marshal(payload)
	w := a.call(method, path, key, "application/json", raw)
	if w.Code != want {
		a.t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, want, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

// mtMember creates a merchant principal of the fixture tenant with exactly perms (plus store:read) and returns its bearer.
func mtMember(t *testing.T, p psHarness, perms ...string) string {
	t.Helper()
	f := p.f
	token, principal := randomToken(), randomUUID()
	mustExec(t, f.owner, `INSERT INTO identity.principals(id) VALUES($1)`, principal)
	mustExec(t, f.owner, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, f.tenantA, principal)
	for _, perm := range append([]string{"store:read"}, perms...) {
		mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4)`, f.tenantA, f.storeA1, principal, perm)
	}
	tx, err := f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := insertSession(context.Background(), tx, token, principal, "merchant", time.Now().Add(time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	return token
}

func mtCSVEnv(t *testing.T) (mtAdmin, psHarness) {
	t.Helper()
	p := psSetup(t)
	return mtAdmin{t: t, h: httpapi.NewHandler(p.f.runtime), store: p.f.storeA1, token: p.f.tokens["a"], pool: p.f.runtime}, p
}

func (a mtAdmin) count(p psHarness, query string, args ...any) int {
	a.t.Helper()
	return countRows(a.t, p.f.owner, query, append([]any{p.f.tenantA, p.f.storeA1}, args...)...)
}

func hasError(res merchanttools.ImportResult, row int, column, code string) bool {
	for _, e := range res.Errors {
		if e.Row == row && e.Column == column && e.Code == code {
			return true
		}
	}
	return false
}

func TestMerchantToolsCSVImportLifecycle(t *testing.T) {
	a, p := mtCSVEnv(t)
	wh := p.stock.warehouse.Name
	header := "handle,title,description,status,option1_name,option1_value,sku,price,compare_at_price,stock:" + wh + ",collections\r\n"
	a.jsonOK("POST", "/collections", t04Key("mtc-coll"), map[string]any{"title": "MT Sale", "slug": "mt-sale"}, 200)

	file1 := header +
		"mt-tee,\"Tee, Classic\",soft,,Size,S,MT-TEE-S,12.50,15,7,mt-sale\r\n" +
		"mt-tee,,,,,M,MT-TEE-M,12.50,,3,\r\n" +
		"mt-hat,Hat,,active,,,MT-HAT,5,,,\r\n"
	products := a.count(p, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2`)

	t.Run("preview computes the answer and writes nothing", func(t *testing.T) {
		code, res := a.importCSV(false, file1)
		if code != 200 || res.Committed || res.Replayed || len(res.Errors) != 0 || res.CreatedProducts != 2 || res.CreatedSKUs != 3 || res.StockAdjustments != 2 || res.Rows != 3 {
			t.Fatalf("preview: %d %+v", code, res)
		}
		if n := a.count(p, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2`); n != products {
			t.Fatalf("a preview wrote products: %d -> %d", products, n)
		}
		if n := a.count(p, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='merchanttools.csv_imported'`); n != 0 {
			t.Fatalf("a preview left an import audit row: %d", n)
		}
	})

	var firstSHA string
	t.Run("commit creates, drafts by default, adjusts stock with the reason, appends the collection", func(t *testing.T) {
		code, res := a.importCSV(true, file1)
		if code != 200 || !res.Committed || res.Replayed || res.CreatedProducts != 2 || res.CreatedSKUs != 3 || res.StockAdjustments != 2 || res.UpdatedProducts != 0 {
			t.Fatalf("commit: %d %+v", code, res)
		}
		sum := sha256.Sum256([]byte(file1))
		firstSHA = hex.EncodeToString(sum[:])
		if res.FileSHA256 != firstSHA {
			t.Fatalf("file sha %s want %s", res.FileSHA256, firstSHA)
		}
		var tee, hat string
		var teePrice int64
		var teeCompare *int64
		var teeOptions string
		if err := p.f.owner.QueryRow(context.Background(), `SELECT status,options::text FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND slug='mt-tee'`, p.f.tenantA, p.f.storeA1).Scan(&tee, &teeOptions); err != nil {
			t.Fatal(err)
		}
		if err := p.f.owner.QueryRow(context.Background(), `SELECT status FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND slug='mt-hat'`, p.f.tenantA, p.f.storeA1).Scan(&hat); err != nil {
			t.Fatal(err)
		}
		if tee != "draft" || hat != "active" || !strings.Contains(teeOptions, `"Size"`) {
			t.Fatalf("status tee=%s hat=%s options=%s", tee, hat, teeOptions)
		}
		if err := p.f.owner.QueryRow(context.Background(), `SELECT price_minor,compare_at_minor FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND code='MT-TEE-S'`, p.f.tenantA, p.f.storeA1).Scan(&teePrice, &teeCompare); err != nil {
			t.Fatal(err)
		}
		if teePrice != 1250 || teeCompare == nil || *teeCompare != 1500 {
			t.Fatalf("price/compare-at in minor units: %d %v", teePrice, teeCompare)
		}
		if n := a.count(p, `SELECT count(*) FROM inventory.ledger l JOIN catalog.skus s ON s.tenant_id=l.tenant_id AND s.store_id=l.store_id AND s.id=l.sku_id
			WHERE l.tenant_id=$1 AND l.store_id=$2 AND s.code LIKE 'MT-%' AND l.reason=$3`, "csv import "+firstSHA[:12]); n != 2 {
			t.Fatalf("stock must go through the adjustment path with the file reason: %d ledger rows", n)
		}
		if n := a.count(p, `SELECT coalesce(sum(b.on_hand),0)::int FROM inventory.balances b JOIN catalog.skus s ON s.tenant_id=b.tenant_id AND s.store_id=b.store_id AND s.id=b.sku_id
			WHERE b.tenant_id=$1 AND b.store_id=$2 AND s.code LIKE 'MT-TEE-%'`); n != 10 {
			t.Fatalf("on-hand after import: %d", n)
		}
		if n := a.count(p, `SELECT count(*) FROM catalog.collection_products cp JOIN catalog.collections c ON c.tenant_id=cp.tenant_id AND c.store_id=cp.store_id AND c.id=cp.collection_id
			JOIN catalog.products pr ON pr.tenant_id=cp.tenant_id AND pr.store_id=cp.store_id AND pr.id=cp.product_id
			WHERE cp.tenant_id=$1 AND cp.store_id=$2 AND c.slug='mt-sale' AND pr.slug='mt-tee'`); n != 1 {
			t.Fatalf("collection membership: %d", n)
		}
		if n := a.count(p, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='merchanttools.csv_imported'`); n != 1 {
			t.Fatalf("import audit rows: %d", n)
		}
	})

	t.Run("the same file again replays and changes nothing", func(t *testing.T) {
		ledger := a.count(p, `SELECT count(*) FROM inventory.ledger WHERE tenant_id=$1 AND store_id=$2`)
		code, res := a.importCSV(true, file1)
		if code != 200 || !res.Replayed || !res.Committed || res.CreatedProducts != 2 {
			t.Fatalf("replay: %d %+v", code, res)
		}
		if n := a.count(p, `SELECT count(*) FROM inventory.ledger WHERE tenant_id=$1 AND store_id=$2`); n != ledger {
			t.Fatalf("a replay wrote ledger rows: %d -> %d", ledger, n)
		}
	})

	t.Run("an update file changes price, adds a variant, applies a stock delta, renames, and deletes nothing", func(t *testing.T) {
		file2 := header +
			"mt-tee,Tee,,,Size,S,MT-TEE-S,13,,9,\r\n" +
			"mt-tee,,,,,M,MT-TEE-M,12.50,15,3,\r\n" + // compare-at 15 (> 12.50) is new on M
			"mt-tee,,,,,L,MT-TEE-L,14,,2,\r\n"
		code, res := a.importCSV(true, file2)
		if code != 200 || !res.Committed || res.CreatedProducts != 0 || res.UpdatedProducts != 1 || res.CreatedSKUs != 1 || res.UpdatedSKUs != 2 || res.StockAdjustments != 2 {
			t.Fatalf("update: %d %+v", code, res)
		}
		var name string
		var onHandS, onHandL int64
		var priceS int64
		var compareS *int64
		if err := p.f.owner.QueryRow(context.Background(), `SELECT name FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND slug='mt-tee'`, p.f.tenantA, p.f.storeA1).Scan(&name); err != nil {
			t.Fatal(err)
		}
		_ = p.f.owner.QueryRow(context.Background(), `SELECT price_minor,compare_at_minor FROM catalog.skus WHERE tenant_id=$1 AND store_id=$2 AND code='MT-TEE-S'`, p.f.tenantA, p.f.storeA1).Scan(&priceS, &compareS)
		_ = p.f.owner.QueryRow(context.Background(), `SELECT b.on_hand FROM inventory.balances b JOIN catalog.skus s ON s.tenant_id=b.tenant_id AND s.store_id=b.store_id AND s.id=b.sku_id WHERE b.tenant_id=$1 AND b.store_id=$2 AND s.code='MT-TEE-S'`, p.f.tenantA, p.f.storeA1).Scan(&onHandS)
		_ = p.f.owner.QueryRow(context.Background(), `SELECT b.on_hand FROM inventory.balances b JOIN catalog.skus s ON s.tenant_id=b.tenant_id AND s.store_id=b.store_id AND s.id=b.sku_id WHERE b.tenant_id=$1 AND b.store_id=$2 AND s.code='MT-TEE-L'`, p.f.tenantA, p.f.storeA1).Scan(&onHandL)
		if name != "Tee" || priceS != 1300 || compareS != nil || onHandS != 9 || onHandL != 2 {
			t.Fatalf("after update: name=%q price=%d compare=%v onHand S=%d L=%d", name, priceS, compareS, onHandS, onHandL)
		}
		if n := a.count(p, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2 AND slug='mt-hat' AND status='active'`); n != 1 {
			t.Fatal("a row missing from the file must leave its product untouched")
		}
	})

	t.Run("export has BOM, attachment headers, one row per active variant, and re-importing it is a no-op", func(t *testing.T) {
		w := a.call("GET", "/products/export.csv", "", "", nil)
		if w.Code != 200 || w.Header().Get("Content-Type") != "text/csv; charset=utf-8" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), `attachment; filename="products-`) {
			t.Fatalf("export: %d %v", w.Code, w.Header())
		}
		body := w.Body.Bytes()
		if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) || !bytes.Contains(body, []byte("\r\n")) {
			t.Fatal("export must be UTF-8 with BOM and CRLF rows")
		}
		parsed := merchanttools.ParseFile(body, 2)
		if len(parsed.Errors) != 0 {
			t.Fatalf("the export does not parse: %+v", parsed.Errors)
		}
		variants := a.count(p, `SELECT count(*) FROM catalog.skus s JOIN catalog.products p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.id=s.product_id
			WHERE s.tenant_id=$1 AND s.store_id=$2 AND s.status='active' AND p.status<>'archived'`)
		noSKU := a.count(p, `SELECT count(*) FROM catalog.products p WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.status<>'archived' AND NOT EXISTS(SELECT 1 FROM catalog.skus s WHERE s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id AND s.status='active')`)
		if len(parsed.Rows) != variants+noSKU {
			t.Fatalf("export rows %d, want %d variants + %d SKU-less products", len(parsed.Rows), variants, noSKU)
		}
		ledger := a.count(p, `SELECT count(*) FROM inventory.ledger WHERE tenant_id=$1 AND store_id=$2`)
		code, res := a.importCSV(true, string(body))
		if code != 200 || !res.Committed || res.CreatedProducts+res.UpdatedProducts+res.CreatedSKUs+res.UpdatedSKUs+res.StockAdjustments != 0 {
			t.Fatalf("re-importing the export must change nothing: %d %+v", code, res)
		}
		if n := a.count(p, `SELECT count(*) FROM inventory.ledger WHERE tenant_id=$1 AND store_id=$2`); n != ledger {
			t.Fatalf("round trip wrote ledger rows: %d -> %d", ledger, n)
		}
	})
}

func TestMerchantToolsCSVAllOrNothing(t *testing.T) {
	a, p := mtCSVEnv(t)
	a.jsonOK("POST", "/collections", t04Key("mtc-coll2"), map[string]any{"title": "Known", "slug": "known"}, 200)
	existing := "handle,title,sku,price\r\nown-product,Own,OWN-1,5\r\n"
	if code, res := a.importCSV(true, existing); code != 200 || !res.Committed {
		t.Fatalf("seed: %d %+v", code, res)
	}
	file := "handle,title,sku,price,collections\r\n" +
		"ok-one,Fine,OK-1,5,\r\n" + // 1 valid
		"Bad Handle,X,OK-X,5,\r\n" + // 2 invalid handle
		"ok-two,Two,OK-1,6,\r\n" + // 3 duplicate sku (in file)
		"ok-three,Three,OWN-1,6,\r\n" + // 4 sku belongs to another product (database)
		"ok-four,Four,OK-4,5,nope\r\n" + // 5 unknown collection (database)
		"ok-five,Five,OK-5,,\r\n" + // 6 new variant without a price
		"ok-six,Six,OK-6,5,known\r\n" // 7 valid
	products := a.count(p, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2`)
	code, res := a.importCSV(false, file)
	if code != 200 || res.Committed {
		t.Fatalf("preview: %d %+v", code, res)
	}
	for _, want := range []struct {
		row          int
		column, code string
	}{{2, "handle", "invalid_value"}, {3, "sku", "duplicate_sku"}, {4, "sku", "sku_in_other_product"}, {5, "collections", "unknown_collection"}, {6, "price", "required"}} {
		if !hasError(res, want.row, want.column, want.code) {
			t.Errorf("preview must list row %d %s %s: %+v", want.row, want.column, want.code, res.Errors)
		}
	}
	for _, e := range res.Errors {
		if e.Row == 1 || e.Row == 7 {
			t.Errorf("valid row %d reported: %+v", e.Row, e)
		}
	}
	for i := 1; i < len(res.Errors); i++ {
		if res.Errors[i-1].Row > res.Errors[i].Row {
			t.Fatalf("errors must be ordered by row: %+v", res.Errors)
		}
	}
	// Commit with the same defects: 422, the body lists them, and NOTHING of the valid rows (one and six) was written.
	code, res = a.importCSV(true, file)
	if code != 422 || res.Committed || len(res.Errors) == 0 {
		t.Fatalf("commit with errors: %d %+v", code, res)
	}
	if n := a.count(p, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2`); n != products {
		t.Fatalf("all-or-nothing violated: products %d -> %d", products, n)
	}
	if n := a.count(p, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='merchanttools.csv_imported'`); n != 1 {
		t.Fatalf("a refused commit must leave no audit row (only the seed import): %d", n)
	}
	// The failed commit is not remembered: after fixing the file the same store accepts it.
	fixed := "handle,title,sku,price\r\nok-one,Fine,OK-1,5\r\n"
	if code, res = a.importCSV(true, fixed); code != 200 || !res.Committed {
		t.Fatalf("fixed file: %d %+v", code, res)
	}
}

func TestMerchantToolsCSVBoundaries(t *testing.T) {
	a, p := mtCSVEnv(t)

	t.Run("formula-looking cells survive export and import unchanged", func(t *testing.T) {
		if code, res := a.importCSV(true, "handle,title,description,sku,price\r\nformula,'=1+1,'@home,'-SKU-1,5\r\n"); code != 200 || !res.Committed || res.CreatedProducts != 1 {
			t.Fatalf("guarded file: %d %+v", code, res)
		}
		var title, desc, sku string
		if err := p.f.owner.QueryRow(context.Background(), `SELECT p.name,p.description,s.code FROM catalog.products p JOIN catalog.skus s ON s.tenant_id=p.tenant_id AND s.store_id=p.store_id AND s.product_id=p.id
			WHERE p.tenant_id=$1 AND p.store_id=$2 AND p.slug='formula'`, p.f.tenantA, p.f.storeA1).Scan(&title, &desc, &sku); err != nil {
			t.Fatal(err)
		}
		if title != "=1+1" || desc != "@home" || sku != "-SKU-1" {
			t.Fatalf("stored values: %q %q %q", title, desc, sku)
		}
		w := a.call("GET", "/products/export.csv", "", "", nil)
		if !strings.Contains(w.Body.String(), "'=1+1") || !strings.Contains(w.Body.String(), "'@home") || !strings.Contains(w.Body.String(), "'-SKU-1") {
			t.Fatalf("export must guard formula cells: %s", w.Body.String())
		}
	})

	t.Run("limits, media type and authority", func(t *testing.T) {
		big := "handle,title\r\n" + strings.Repeat("a", merchanttools.MaxCSVBytes)
		if w := a.call("POST", "/products/import/preview", "", "text/csv", []byte(big)); w.Code != 413 {
			t.Errorf("a file over 2 MiB: %d", w.Code)
		}
		var rows strings.Builder
		rows.WriteString("handle,title\r\n")
		for i := 0; i <= merchanttools.MaxCSVRows; i++ {
			rows.WriteString("x,y\r\n")
		}
		if code, res := a.importCSV(false, rows.String()); code != 200 || !hasError(res, 0, "", "limit") {
			t.Errorf("one row over the limit: %d %+v", code, res.Errors)
		}
		if w := a.call("POST", "/products/import/preview", "", "application/json", []byte("{}")); w.Code != 415 {
			t.Errorf("wrong media type: %d", w.Code)
		}
		if w := a.call("POST", "/products/import/commit", "", "text/csv", nil); w.Code != 413 {
			t.Errorf("empty body: %d", w.Code)
		}
		if w := a.with("").call("GET", "/products/export.csv", "", "", nil); w.Code != 401 {
			t.Errorf("no bearer: %d", w.Code)
		}
		reader := mtMember(t, p, "catalog:read", "inventory:read")
		if w := a.with(reader).call("GET", "/products/export.csv", "", "", nil); w.Code != 200 {
			t.Errorf("a catalog+inventory reader may export: %d", w.Code)
		}
		if w := a.with(reader).call("POST", "/products/import/preview", "", "text/csv", []byte("handle,title\r\na,b\r\n")); w.Code != 403 {
			t.Errorf("a reader must not import: %d", w.Code)
		}
		onlyCatalog := mtMember(t, p, "catalog:write")
		if w := a.with(onlyCatalog).call("POST", "/products/import/preview", "", "text/csv", []byte("handle,title\r\na,b\r\n")); w.Code != 403 {
			t.Errorf("import needs inventory:write too: %d", w.Code)
		}
		noInventory := mtMember(t, p, "catalog:read")
		if w := a.with(noInventory).call("GET", "/products/export.csv", "", "", nil); w.Code != 403 {
			t.Errorf("export needs inventory:read too: %d", w.Code)
		}
		other := a
		other.store = p.f.storeA2
		if w := other.call("GET", "/products/export.csv", "", "", nil); w.Code != 404 && w.Code != 403 {
			t.Errorf("another store of the same tenant must not be reachable with this scope: %d", w.Code)
		}
	})

	t.Run("a stock target below what is reserved is a row error, never a negative balance", func(t *testing.T) {
		wh := p.stock.warehouse.Name
		sku := p.stock.skus[0].Code
		// A real reservation through inventory.Reserve (the only way reserved moves), 4 of the 10 on hand.
		if _, err := t04Scoped(context.Background(), p.f, p.f.tokens["a"], p.f.storeA1, "inventory:reserve", func(tx pgx.Tx, s platform.Scope) (inventory.Reservation, error) {
			return inventory.Reserve(context.Background(), tx, s, t04Key("mtc-res"), []inventory.Line{{WarehouseID: p.stock.warehouse.ID, SKUID: p.stock.skus[0].ID, Quantity: 4}})
		}); err != nil {
			t.Fatalf("reserve: %v", err)
		}
		file := fmt.Sprintf("handle,title,sku,price,stock:%s\r\n%s,T,%s,12.50,2\r\n", wh, p.stock.product.Slug, sku)
		code, res := a.importCSV(false, file)
		if code != 200 || len(res.Errors) != 1 || !hasError(res, 1, "stock:"+wh, "conflict") {
			t.Fatalf("below reserved: %d %+v", code, res)
		}
	})
}

// mtScaleFile builds n products with one variant, a price and a stock cell each (the heaviest ordinary row: product + variant + adjustment).
func mtScaleFile(wh string, n int, prefix string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "handle,title,option1_name,option1_value,sku,price,stock:%s\r\n", wh)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s-%d,Scale %d,,,%s-%d,%d.50,%d\r\n", prefix, i, i, strings.ToUpper(prefix), i, 1+i%90, 1+i%7)
	}
	return b.String()
}

// MTC04 measures the all-or-nothing import in ONE transaction (preview mode: same code, rolled back, so each size starts clean), commits the contract
// maximum through the real route and proves the work cap. Findings of the first runs (the reason for the limits): 11-18 ms per product+variant+stock row
// on an idle machine (4x on a loaded one), and 5,000 rows died with "out of shared memory" (every command holds an advisory lock until commit).
func TestMerchantToolsCSVScale(t *testing.T) {
	a, p := mtCSVEnv(t)
	wh := p.stock.warehouse.Name
	ctx := context.Background()
	for _, n := range []int{250, 1000} {
		file := mtScaleFile(wh, n, fmt.Sprintf("m%d", n))
		start := time.Now()
		var res merchanttools.ImportResult
		err := platform.WithScopeBudget(ctx, p.f.runtime, p.f.tokens["a"], p.f.storeA1, "catalog:write", 110*time.Second, func(tx pgx.Tx, s platform.Scope) error {
			var inner error
			res, inner = merchanttools.ImportProducts(ctx, tx, s, []byte(file), false)
			return inner
		})
		took := time.Since(start)
		t.Logf("MTC04 measure n=%d (product+variant+stock): %s (%.1f ms/row) created=%d adjustments=%d errors=%d err=%v", n, took, float64(took.Milliseconds())/float64(n), res.CreatedProducts, res.StockAdjustments, len(res.Errors), err)
		if len(res.Errors) != 0 || res.CreatedProducts != n || res.StockAdjustments != n {
			t.Fatalf("n=%d: %+v", n, res)
		}
	}
	// The contract maximum through the real route (no stock: 2 work units per row). The HTTP answer only exists if it finished inside the budget.
	n := merchanttools.MaxCSVRows
	var b strings.Builder
	b.WriteString("handle,title,sku,price\r\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "max-%d,Max %d,MAX-%d,%d\r\n", i, i, i, 1+i%90)
	}
	start := time.Now()
	code, res := a.importCSV(true, b.String())
	t.Logf("MTC04 contract maximum: %d rows (product+variant, no stock), status %d in %s (committed=%v, errors=%d)", n, code, time.Since(start), res.Committed, len(res.Errors))
	if code != 200 || !res.Committed || res.CreatedProducts != n || res.CreatedSKUs != n || len(res.Errors) != 0 {
		t.Fatalf("maximum commit: %d %+v", code, res)
	}
	start = time.Now()
	w := a.call("GET", "/products/export.csv", "", "", nil)
	t.Logf("MTC04 export of %d+ rows: %d in %s, %d bytes", n, w.Code, time.Since(start), w.Body.Len())
	if w.Code != 200 {
		t.Fatalf("export: %d", w.Code)
	}
	// The work cap: new products with stock in TWO warehouses are 6 units each, so 1,100 rows = 6,600 > 6,000 although the row count is legal.
	// Refused with `limit` at the row where it became too large, nothing written, long before the time budget.
	var second struct{ Name string }
	a.jsonOK("POST", "/warehouses", t04Key("mtc-wh2"), map[string]any{"name": "mtc-second-" + t04Tag()}, 200)
	if err := p.f.owner.QueryRow(ctx, `SELECT name FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 AND id<>$3 AND name LIKE 'mtc-second-%' LIMIT 1`, p.f.tenantA, p.f.storeA1, p.stock.warehouse.ID).Scan(&second.Name); err != nil {
		t.Fatal(err)
	}
	var cap strings.Builder
	fmt.Fprintf(&cap, "handle,title,sku,price,stock:%s,stock:%s\r\n", wh, second.Name)
	for i := 0; i < 1100; i++ {
		fmt.Fprintf(&cap, "cap-%d,Cap %d,CAP-%d,5,%d,%d\r\n", i, i, i, 1+i%5, 1+i%3)
	}
	products := a.count(p, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2`)
	start = time.Now()
	code, res = a.importCSV(true, cap.String())
	t.Logf("MTC04 work cap: 1100 rows x 2 warehouses -> status %d in %s, errors=%v", code, time.Since(start), res.Errors)
	if code != 422 || res.Committed || len(res.Errors) == 0 || res.Errors[len(res.Errors)-1].Code != "limit" {
		t.Fatalf("work cap: %d %+v", code, res)
	}
	if after := a.count(p, `SELECT count(*) FROM catalog.products WHERE tenant_id=$1 AND store_id=$2`); after != products {
		t.Fatalf("a refused file wrote products: %d -> %d", products, after)
	}
}
