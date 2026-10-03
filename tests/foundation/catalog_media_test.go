package foundation_test

// Independent catalog-media gates (R3 test author, not the implementer). Written from docs/delivery/units/catalog-media.md
// (CM1-CM4), contracts/catalog-inventory-openapi.json (the images paths + Image/ImageList/ImageOrderInput schemas) and the
// "Amendment catalog-media" of contracts/buyer-catalog-discovery-v1.md (BCAT06-BCAT08). Real PG 18 through
// `bash scripts/dev/test-focused.sh '^TestCatalogMedia'`; evidence label REAL_PG (no browser here, no provider).
//
//	TestCatalogMediaSchemaSurface   0082 applied: FORCE RLS, role, definer owner/EXECUTE/search_path, exact grants, CHECK/UNIQUE/FK.
//	TestCatalogMediaRLSScope        commerce_runtime sees and writes only its own store's rows (policy scope_access).
//	TestCatalogMediaLifecycle       upload -> list -> merchant bytes -> buyer catalog images -> public bytes + headers -> Meta feed
//	                                image_link -> reorder -> delete renumbering -> 8-photo cap (9th = 409) -> idempotent replay.
//	TestCatalogMediaSniff           magic-byte truth: SVG/GIF/garbage/PNG-as-JPEG/2 MiB / 2 MiB+1 and the multipart grammar.
//	TestCatalogMediaBuyer404        BCAT07: unpublished, unknown, other-product, cross-store, cross-tenant and archived are one 404.
//	TestCatalogMediaBuyerAuthority  the media route takes the BFF key + origin only (Authorization 403, query/cookie/body 422).
//	TestCatalogMediaArchivedProduct archived product: upload 409, delete allowed, buyer catalog/media/feed drop it.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/attribution"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/platform"
)

// ---- synthetic files (stdlib encoders; every call with a new seed yields different bytes) ------------------------------------

func cmiPNG(t *testing.T, w, h int, seed uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{seed, uint8(x), uint8(y), 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func cmiJPEG(t *testing.T, w, h int, seed uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), seed, uint8(y), 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// cmiWebP is now a real lossless 4x3 WebP: S1 decodes uploads, not just RIFF headers.
// Original-format/NULL-metadata assertions below remain unchanged. Corrupt RIFF is tested separately by TestImageSizesRejectCorruptAndBudget.
func cmiWebP(seed uint8) []byte {
	data, _ := base64.StdEncoding.DecodeString("UklGRh4AAABXRUJQVlA4TBEAAAAvA4AAAAdQqIKUsf+BiOh/AAA=")
	return data
}

// cmiPadded is a real JPEG padded with zero bytes to exactly n bytes (stdlib DecodeConfig reads only the header).
func cmiPadded(t *testing.T, n int) []byte {
	t.Helper()
	data := cmiJPEG(t, 16, 16, 3)
	if len(data) > n {
		t.Fatal("test jpeg larger than the pad target")
	}
	return append(data, make([]byte, n-len(data))...)
}

type cmiPart struct {
	field, filename, contentType string
	data                         []byte
}

func cmiFile(filename, contentType string, data []byte) cmiPart {
	return cmiPart{"file", filename, contentType, data}
}

func cmiMultipart(t *testing.T, parts ...cmiPart) ([]byte, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="`+p.field+`"; filename="`+p.filename+`"`)
		if p.contentType != "" {
			h.Set("Content-Type", p.contentType)
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(p.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes(), w.FormDataContentType()
}

// ---- merchant API client (real httpapi handler over the isolated PG) --------------------------------------------------------

type cmiImage struct {
	ID          string `json:"id"`
	ProductID   string `json:"product_id"`
	Position    int    `json:"position"`
	ContentType string `json:"content_type"`
	SizeBytes   int    `json:"size_bytes"`
	Width       *int   `json:"width"`
	Height      *int   `json:"height"`
	Version     int64  `json:"version"`
}

type cmiMerchant struct {
	t     *testing.T
	h     http.Handler
	token string
	store string
}

func (m cmiMerchant) raw(method, path string, headers map[string]string, body []byte) *httptest.ResponseRecorder {
	m.t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	if m.token != "" {
		r.Header.Set("Authorization", "Bearer "+m.token)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func (m cmiMerchant) imagesPath(product string) string {
	return "/v1/admin/stores/" + m.store + "/products/" + product + "/images"
}

func (m cmiMerchant) upload(product, key string, parts ...cmiPart) *httptest.ResponseRecorder {
	m.t.Helper()
	body, ct := cmiMultipart(m.t, parts...)
	return m.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": ct, "Idempotency-Key": key}, body)
}

func (m cmiMerchant) mustUpload(product string, data []byte, filename, declared string) cmiImage {
	m.t.Helper()
	w := m.upload(product, t04Key("cmi-up"), cmiFile(filename, declared, data))
	if w.Code != 200 {
		m.t.Fatalf("upload %s: %d %s", filename, w.Code, w.Body.String())
	}
	var img cmiImage
	if err := json.Unmarshal(w.Body.Bytes(), &img); err != nil || img.ID == "" {
		m.t.Fatalf("upload body: %v %s", err, w.Body.String())
	}
	return img
}

func (m cmiMerchant) list(product string) []cmiImage {
	m.t.Helper()
	w := m.raw("GET", m.imagesPath(product), nil, nil)
	if w.Code != 200 {
		m.t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Items []cmiImage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Items == nil {
		m.t.Fatalf("list body is not a non-null ImageList: %v %s", err, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte(`"bytes"`)) {
		m.t.Fatal("image metadata list carries bytes")
	}
	return out.Items
}

func (m cmiMerchant) ids(product string) []string {
	m.t.Helper()
	var out []string
	for _, img := range m.list(product) {
		out = append(out, img.ID)
	}
	return out
}

func (m cmiMerchant) order(product string, ids []string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string][]string{"ids": ids})
	return m.raw("POST", m.imagesPath(product)+"/order", map[string]string{"Content-Type": "application/json", "Idempotency-Key": t04Key("cmi-order")}, body)
}

func (m cmiMerchant) remove(product, image, key string) *httptest.ResponseRecorder {
	return m.raw("POST", m.imagesPath(product)+"/"+image+"/delete", map[string]string{"Content-Type": "application/json", "Idempotency-Key": key}, []byte("{}"))
}

// archive archives the product through the existing catalog route with its current version.
func (m cmiMerchant) archive(f *testFixture, product string) {
	m.t.Helper()
	var version int64
	if err := f.owner.QueryRow(context.Background(), `SELECT version FROM catalog.products WHERE id=$1`, product).Scan(&version); err != nil {
		m.t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]int64{"expected_version": version})
	w := m.raw("POST", "/v1/admin/stores/"+m.store+"/products/"+product+"/archive", map[string]string{"Content-Type": "application/json", "Idempotency-Key": t04Key("cmi-arch")}, body)
	if w.Code != 200 {
		m.t.Fatalf("archive: %d %s", w.Code, w.Body.String())
	}
}

// cmiRequireContiguous checks the list is positions 0..n-1 in list order (the contract's "positions stay contiguous").
func cmiRequireContiguous(t *testing.T, list []cmiImage) {
	t.Helper()
	for i, img := range list {
		if img.Position != i {
			t.Fatalf("image %d has position %d, want contiguous 0..%d: %+v", i, img.Position, len(list)-1, list)
		}
	}
}

// ---- buyer side ---------------------------------------------------------------------------------------------------------------

type cmiMedia struct {
	status int
	header http.Header
	body   []byte
}

// cmiGetMedia calls GET /v1/buyer/media/p/{product}/{image} on the real buyerhttp server with only the BFF key and origin (no bearer).
func cmiGetMedia(t *testing.T, h bhHarness, origin, product, image string, edit func(*http.Request)) cmiMedia {
	t.Helper()
	r, err := http.NewRequest("GET", h.server.URL+"/v1/buyer/media/p/"+product+"/"+image, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-Commerce-Buyer-BFF-Key", h.key)
	if origin != "" {
		r.Header.Set("X-Commerce-Storefront-Origin", origin)
	}
	if edit != nil {
		edit(r)
	}
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		t.Fatal(err)
	}
	return cmiMedia{res.StatusCode, res.Header, body}
}

type cmiCatalogImage struct {
	ID     string `json:"id"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
}

// cmiCatalogImages reads the buyer catalog and returns, per product id, the images of every SKU row of that product (BCAT06).
func cmiCatalogImages(t *testing.T, h bhHarness) map[string][][]cmiCatalogImage {
	t.Helper()
	out := map[string][][]cmiCatalogImage{}
	cursor := ""
	for page := 0; page < 20; page++ {
		q := "limit=100"
		if cursor != "" {
			q += "&cursor=" + cursor
		}
		r := h.request(t, "GET", "/v1/buyer/catalog?"+q, h.cap.Token, "", nil, nil)
		if r.status != 200 {
			t.Fatalf("buyer catalog: %d %s", r.status, r.body)
		}
		var raw struct {
			Items []struct {
				ProductID string          `json:"product_id"`
				Images    json.RawMessage `json:"images"`
			} `json:"items"`
			NextCursor string `json:"next_cursor"`
		}
		if err := json.Unmarshal(r.body, &raw); err != nil {
			t.Fatal(err)
		}
		for _, item := range raw.Items {
			if len(item.Images) == 0 || string(item.Images) == "null" {
				t.Fatalf("item %s: images must always be an array, got %q", item.ProductID, item.Images)
			}
			var list []map[string]json.RawMessage
			if err := json.Unmarshal(item.Images, &list); err != nil {
				t.Fatal(err)
			}
			for _, entry := range list {
				keys := make([]string, 0, 3)
				for k := range entry {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				if !reflect.DeepEqual(keys, []string{"height", "id", "width"}) {
					t.Fatalf("buyer image keys %v, want exactly id,width,height (metadata only)", keys)
				}
			}
			var typed []cmiCatalogImage
			if err := json.Unmarshal(item.Images, &typed); err != nil {
				t.Fatal(err)
			}
			out[item.ProductID] = append(out[item.ProductID], typed)
		}
		if raw.NextCursor == "" {
			return out
		}
		cursor = raw.NextCursor
	}
	t.Fatal("buyer catalog did not end")
	return nil
}

// cmiFeedImageLink returns the image_link of the Meta feed row whose link points at the product ("" when the row has none), and
// whether the product has a row at all.
func cmiFeedImageLink(t *testing.T, h bhHarness, product string) (string, bool) {
	t.Helper()
	r := httptest.NewRequest("GET", "/v1/buyer/feeds/meta.csv", nil)
	r.Header.Set("X-Commerce-Storefront-Origin", h.origin)
	w := httptest.NewRecorder()
	attribution.FeedHandler(h.a.runtime).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("feed: %d %s", w.Code, w.Body.String())
	}
	rows, err := csv.NewReader(w.Body).ReadAll()
	if err != nil || len(rows) == 0 {
		t.Fatalf("feed csv: %v", err)
	}
	col := map[string]int{}
	for i, name := range rows[0] {
		col[name] = i
	}
	for _, name := range []string{"link", "image_link"} {
		if _, ok := col[name]; !ok {
			t.Fatalf("feed has no %s column: %v", name, rows[0])
		}
	}
	for _, row := range rows[1:] {
		if strings.HasSuffix(row[col["link"]], "/products/"+product) {
			return row[col["image_link"]], true
		}
	}
	return "", false
}

// cmiCleanup removes every image row of the products at the end (the shared foundation database outlives each test).
func cmiCleanup(t *testing.T, f *testFixture, products ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, p := range products {
			mustExec(t, f.owner, `DELETE FROM catalog.product_images WHERE product_id=$1`, p)
		}
	})
}

func cmiErrCode(t *testing.T, err error) string {
	t.Helper()
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		t.Fatalf("not a PG error: %v", err)
	}
	return pg.Code
}

// cmiEnv is the shared per-test setup: the buyer HTTP harness (storefront origin published for storeA1), the merchant API for
// storeA1, and a fresh product with SKUs in storeA1 whose image rows are removed at the end.
func cmiEnv(t *testing.T) (bhHarness, cmiMerchant, string) {
	t.Helper()
	h := bhSetup(t)
	m := cmiMerchant{t: t, h: httpapi.NewHandler(h.f.runtime), token: h.f.tokens["a"], store: h.f.storeA1}
	product := h.stock.product.ID
	cmiCleanup(t, h.f, product)
	return h, m, product
}

func cmiNormalizeRequestID(body []byte) []byte {
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return body
	}
	delete(m, "request_id")
	delete(m, "X-Request-ID")
	out, _ := json.Marshal(m)
	return out
}

// =============================================================================================================================
// 0082 applied: structure, RLS, definers, exact grants.
// =============================================================================================================================

func TestCatalogMediaSchemaSurface(t *testing.T) {
	h, _, product := cmiEnv(t)
	f := h.f
	ctx := context.Background()
	var enabled, forced bool
	if err := f.owner.QueryRow(ctx, `SELECT relrowsecurity,relforcerowsecurity FROM pg_class WHERE oid='catalog.product_images'::regclass`).Scan(&enabled, &forced); err != nil || !enabled || !forced {
		t.Fatalf("catalog.product_images must ENABLE and FORCE row level security: %v %v %v", enabled, forced, err)
	}
	var login, super, bypass bool
	if err := f.owner.QueryRow(ctx, `SELECT rolcanlogin,rolsuper,rolbypassrls FROM pg_roles WHERE rolname='commerce_catalog_media'`).Scan(&login, &super, &bypass); err != nil || login || super || bypass {
		t.Fatalf("commerce_catalog_media must be NOLOGIN NOSUPERUSER NOBYPASSRLS: login=%v super=%v bypass=%v %v", login, super, bypass, err)
	}

	// Every role (plus PUBLIC), enumerated: only the buyer runtime (and the definer owner itself) may EXECUTE the three definers.
	rows, err := f.owner.Query(ctx, `SELECT rolname FROM pg_roles WHERE rolname NOT LIKE 'pg\_%' AND NOT rolsuper UNION SELECT 'public' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var roles []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		roles = append(roles, r)
	}
	rows.Close()
	definers := map[string]string{
		"buyer_product_images": "catalog.buyer_product_images(uuid[])",
		"buyer_media_image":    "catalog.buyer_media_image(text,uuid,uuid)",
		"buyer_feed_images":    "catalog.buyer_feed_images(text)",
	}
	for name, sig := range definers {
		var secdef bool
		var owner string
		var config []string
		if err := f.owner.QueryRow(ctx, `SELECT p.prosecdef,r.rolname,coalesce(p.proconfig,'{}') FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, sig).Scan(&secdef, &owner, &config); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !secdef || owner != "commerce_catalog_media" {
			t.Errorf("%s: secdef=%v owner=%s, want SECURITY DEFINER owned by commerce_catalog_media", name, secdef, owner)
		}
		if !strings.Contains(strings.Join(config, ","), "search_path=pg_catalog") {
			t.Errorf("%s: search_path is not pinned to pg_catalog: %v", name, config)
		}
		for _, role := range roles {
			var can bool
			if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, role, sig).Scan(&can); err != nil {
				t.Fatal(err)
			}
			var member bool // a login role that is a member of the buyer runtime (or of the definer owner) inherits EXECUTE legitimately
			if err := f.owner.QueryRow(ctx, `SELECT CASE WHEN $1='public' THEN false ELSE pg_has_role($1,'commerce_buyer_runtime','USAGE') OR pg_has_role($1,'commerce_catalog_media','USAGE') END`, role).Scan(&member); err != nil {
				t.Fatal(err)
			}
			if want := member; can != want {
				t.Errorf("%s: role %s EXECUTE=%v, want %v", name, role, can, want)
			}
		}
	}

	// Table privileges, enumerated: the buyer runtime and PUBLIC have none; the merchant runtime has SELECT/INSERT/DELETE and UPDATE
	// only on position and version; the definer owner has SELECT.
	cols, err := f.owner.Query(ctx, `SELECT attname FROM pg_attribute WHERE attrelid='catalog.product_images'::regclass AND attnum>0 AND NOT attisdropped ORDER BY attnum`)
	if err != nil {
		t.Fatal(err)
	}
	var columns []string
	for cols.Next() {
		var c string
		if err := cols.Scan(&c); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, c)
	}
	cols.Close()
	if len(columns) < 10 {
		t.Fatalf("columns %v", columns)
	}
	for _, role := range roles {
		var merchantSide bool // members of the merchant runtime / definer owner hold the table grants checked below
		if err := f.owner.QueryRow(ctx, `SELECT CASE WHEN $1='public' THEN false ELSE pg_has_role($1,'commerce_runtime','USAGE') OR pg_has_role($1,'commerce_catalog_media','USAGE') END`, role).Scan(&merchantSide); err != nil {
			t.Fatal(err)
		}
		if merchantSide {
			continue
		}
		if owner := f.owner.QueryRow(ctx, `SELECT 1 FROM pg_class c JOIN pg_roles r ON r.oid=c.relowner WHERE c.oid='catalog.product_images'::regclass AND r.rolname=$1`, role).Scan(new(int)); owner == nil {
			continue // the table owner (migration role) holds everything by definition
		}
		for _, priv := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			var can bool
			if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,'catalog.product_images',$2)`, role, priv).Scan(&can); err != nil {
				t.Fatal(err)
			}
			if can {
				t.Errorf("%s holds %s on catalog.product_images", role, priv)
			}
		}
		for _, c := range columns {
			var can bool
			if err := f.owner.QueryRow(ctx, `SELECT has_column_privilege($1,'catalog.product_images',$2,'SELECT')`, role, c).Scan(&can); err != nil {
				t.Fatal(err)
			}
			if can {
				t.Errorf("%s can SELECT column %s of catalog.product_images", role, c)
			}
		}
	}
	for _, priv := range []string{"SELECT", "INSERT", "DELETE"} {
		var can bool
		_ = f.owner.QueryRow(ctx, `SELECT has_table_privilege('commerce_runtime','catalog.product_images',$1)`, priv).Scan(&can)
		if !can {
			t.Errorf("commerce_runtime lacks %s on catalog.product_images", priv)
		}
	}
	var tableUpdate bool
	_ = f.owner.QueryRow(ctx, `SELECT has_table_privilege('commerce_runtime','catalog.product_images','UPDATE')`).Scan(&tableUpdate)
	if tableUpdate {
		t.Error("commerce_runtime holds table-level UPDATE (bytes would be rewritable)")
	}
	for _, c := range columns {
		var can bool
		_ = f.owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_runtime','catalog.product_images',$1,'UPDATE')`, c).Scan(&can)
		if want := c == "position" || c == "version"; can != want {
			t.Errorf("commerce_runtime UPDATE(%s)=%v, want %v", c, can, want)
		}
	}
	var mediaRead, nameGrant, mediaName bool
	_ = f.owner.QueryRow(ctx, `SELECT has_table_privilege('commerce_catalog_media','catalog.product_images','SELECT')`).Scan(&mediaRead)
	_ = f.owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_buyer_runtime','control.stores','name','SELECT')`).Scan(&nameGrant)
	_ = f.owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_catalog_media','control.stores','name','SELECT')`).Scan(&mediaName)
	if !mediaRead {
		t.Error("the definer owner cannot SELECT catalog.product_images")
	}
	if !nameGrant {
		t.Error("buyer runtime must read control.stores(name) for store_name")
	}
	// Documented change (contracts/storefront-v2.md section A "Buyer reads": every buyer v2 list returns
	// store:{name, currency}; migrations/0086_catalog_v2.sql grants SELECT(name,currency) to this definer owner,
	// behind the same active-store policy; name is already public through the 0082 buyer grant above).
	// Before 0086 this role must NOT read name; since 0086 it must read exactly name+currency and never write.
	if !mediaName {
		t.Error("the definer owner must read control.stores(name) since 0086 (storefront-v2 section A: store:{name,currency})")
	}
	for _, priv := range []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE"} {
		var can bool
		_ = f.owner.QueryRow(ctx, `SELECT has_table_privilege('commerce_catalog_media','control.stores',$1)`, priv).Scan(&can)
		if can {
			t.Errorf("the definer owner holds %s on control.stores", priv)
		}
	}

	// The buyer runtime pool really is denied the table (privilege, not only has_*_privilege).
	_, err = h.a.runtime.Exec(ctx, `SELECT 1 FROM catalog.product_images LIMIT 1`)
	if err == nil || cmiErrCode(t, err) != "42501" {
		t.Fatalf("buyer runtime SELECT on catalog.product_images: %v, want 42501", err)
	}

	// Structural limits, as the table owner (CHECK / UNIQUE / FK are not RLS).
	insert := func(pos int, ct string, data, sha []byte, store string) error {
		_, err := f.owner.Exec(ctx, `INSERT INTO catalog.product_images(tenant_id,store_id,product_id,position,content_type,bytes,sha256) VALUES($1,$2,$3,$4,$5,$6,$7)`,
			f.tenantA, store, product, pos, ct, data, sha)
		return err
	}
	sha := bytes.Repeat([]byte{1}, 32)
	small := []byte{1, 2, 3}
	for name, c := range map[string]struct {
		pos   int
		ct    string
		data  []byte
		sha   []byte
		store string
	}{
		"position 8":                  {8, "image/png", small, sha, f.storeA1},
		"position -1":                 {-1, "image/png", small, sha, f.storeA1},
		"content type gif":            {0, "image/gif", small, sha, f.storeA1},
		"content type svg":            {0, "image/svg+xml", small, sha, f.storeA1},
		"2 MiB + 1 bytes":             {0, "image/png", make([]byte, 2<<20+1), sha, f.storeA1},
		"empty bytes":                 {0, "image/png", []byte{}, sha, f.storeA1},
		"short sha256":                {0, "image/png", small, sha[:31], f.storeA1},
		"product of other store (FK)": {0, "image/png", small, sha, f.storeA2},
	} {
		err := insert(c.pos, c.ct, c.data, c.sha, c.store)
		if err == nil {
			t.Errorf("%s: insert accepted", name)
			mustExec(t, f.owner, `DELETE FROM catalog.product_images WHERE product_id=$1`, product)
			continue
		}
		if code := cmiErrCode(t, err); code != "23514" && code != "23503" {
			t.Errorf("%s: error %s, want a CHECK (23514) or FK (23503) violation", name, code)
		}
	}
	if err := insert(7, "image/png", make([]byte, 2<<20), sha, f.storeA1); err != nil {
		t.Errorf("exactly 2 MiB at position 7 must be accepted: %v", err)
	}
	mustExec(t, f.owner, `DELETE FROM catalog.product_images WHERE product_id=$1`, product)
	// UNIQUE(position) is DEFERRABLE INITIALLY DEFERRED: a duplicate fails at COMMIT, a swap inside one transaction passes.
	for pos := 0; pos < 2; pos++ {
		if err := insert(pos, "image/png", small, sha, f.storeA1); err != nil {
			t.Fatal(err)
		}
	}
	if err := insert(1, "image/png", small, sha, f.storeA1); err == nil || cmiErrCode(t, err) != "23505" {
		t.Errorf("duplicate position: %v, want 23505", err)
	}
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE catalog.product_images SET position=1-position WHERE product_id=$1`, product); err != nil {
		t.Fatalf("a position swap inside one transaction must pass (deferred UNIQUE): %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("swap commit: %v", err)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, product); n != 2 {
		t.Fatalf("%d rows after the negative inserts, want 2", n)
	}
}

// RLS: the merchant runtime sees and writes only its own store (policy scope_access).
func TestCatalogMediaRLSScope(t *testing.T) {
	h, _, product := cmiEnv(t)
	f := h.f
	ctx := context.Background()
	other := t04CreateStock(t, f, f.tokens["a2"], f.storeA2, 3).product.ID
	foreign := t04CreateStock(t, f, f.tokens["b"], f.storeB, 3).product.ID
	cmiCleanup(t, f, other, foreign)
	sha := bytes.Repeat([]byte{7}, 32)
	seed := func(tenant, store, prod string) {
		mustExec(t, f.owner, `INSERT INTO catalog.product_images(tenant_id,store_id,product_id,position,content_type,bytes,sha256) VALUES($1,$2,$3,0,'image/png','\x01',$4)`, tenant, store, prod, sha)
	}
	seed(f.tenantA, f.storeA1, product)
	seed(f.tenantA, f.storeA2, other)
	seed(f.tenantB, f.storeB, foreign)
	visible := func(token, store string) (products []string) {
		err := platform.WithScope(ctx, f.runtime, token, store, "catalog:read", func(tx pgx.Tx, _ platform.Scope) error {
			rows, err := tx.Query(ctx, `SELECT DISTINCT product_id::text FROM catalog.product_images ORDER BY 1`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var p string
				if err := rows.Scan(&p); err != nil {
					return err
				}
				products = append(products, p)
			}
			return rows.Err()
		})
		if err != nil {
			t.Fatalf("scoped read: %v", err)
		}
		return products
	}
	// The fixture DB is shared by the whole run and the stores are fixed, so earlier tests (catalog_core_gate_test.go
	// uploads product photos as tenant B / A1) leave image rows in the same stores. The RLS invariant is therefore
	// stated against the owner's unfiltered view: a scope sees exactly the rows of its OWN store (including the seeded
	// one) and none of the other two seeded products.
	ownProducts := func(store string) (ids []string) {
		rows, err := f.owner.Query(ctx, `SELECT DISTINCT product_id::text FROM catalog.product_images WHERE store_id=$1 ORDER BY 1`, store)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, p)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return ids
	}
	seeded := []string{product, other, foreign}
	for _, c := range []struct{ token, store, want string }{{f.tokens["a"], f.storeA1, product}, {f.tokens["a2"], f.storeA2, other}, {f.tokens["b"], f.storeB, foreign}} {
		got := visible(c.token, c.store)
		if want := ownProducts(c.store); !slices.Equal(got, want) {
			t.Errorf("scope %s sees %v, want exactly its own store's %v", c.store, got, want)
		}
		for _, s := range seeded {
			if has := slices.Contains(got, s); has != (s == c.want) {
				t.Errorf("scope %s sees seeded product %s = %v, want %v", c.store, s, has, s == c.want)
			}
		}
	}
	// WITH CHECK: an insert for another store from the A1 scope is refused by RLS, not silently moved.
	err := platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(ctx, `INSERT INTO catalog.product_images(tenant_id,store_id,product_id,position,content_type,bytes,sha256) VALUES($1,$2,$3,1,'image/png','\x01',$4)`, f.tenantA, f.storeA2, other, sha)
		return err
	})
	if err == nil || cmiErrCode(t, err) != "42501" {
		t.Errorf("cross-store insert from the A1 scope: %v, want 42501 (RLS)", err)
	}
	// A DELETE of another store's row from the A1 scope removes nothing.
	_ = platform.WithScope(ctx, f.runtime, f.tokens["a"], f.storeA1, "catalog:write", func(tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(ctx, `DELETE FROM catalog.product_images WHERE product_id=$1`, other)
		return err
	})
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, other); n != 1 {
		t.Errorf("a foreign-store delete removed the row (%d left)", n)
	}
}

// =============================================================================================================================
// End to end over HTTP: merchant -> buyer catalog -> public bytes -> Meta feed.
// =============================================================================================================================

func TestCatalogMediaLifecycle(t *testing.T) {
	h, m, product := cmiEnv(t)
	f := h.f
	if n := len(m.list(product)); n != 0 {
		t.Fatalf("fresh product has %d images", n)
	}
	if link, ok := cmiFeedImageLink(t, h, product); !ok || link != "" {
		t.Fatalf("a product without photos must be in the feed with an empty image_link: %q %v", link, ok)
	}
	if imgs := cmiCatalogImages(t, h)[product]; len(imgs) == 0 || len(imgs[0]) != 0 {
		t.Fatalf("buyer catalog must show images [] for a product without photos: %+v", imgs)
	}

	pngBytes, jpgBytes, webpBytes := cmiPNG(t, 40, 30, 11), cmiJPEG(t, 64, 48, 22), cmiWebP(33)
	a := m.mustUpload(product, pngBytes, "a.png", "image/png")
	b := m.mustUpload(product, jpgBytes, "b.jpg", "image/jpeg")
	c := m.mustUpload(product, webpBytes, "c.webp", "image/webp")
	if a.Position != 0 || b.Position != 1 || c.Position != 2 {
		t.Fatalf("positions %d %d %d, want 0 1 2", a.Position, b.Position, c.Position)
	}
	if a.ProductID != product || a.ContentType != "image/png" || a.SizeBytes != len(pngBytes) || a.Width == nil || *a.Width != 40 || a.Height == nil || *a.Height != 30 || a.Version < 1 {
		t.Fatalf("png metadata wrong: %+v", a)
	}
	if b.ContentType != "image/jpeg" || b.Width == nil || *b.Width != 64 || b.Height == nil || *b.Height != 48 {
		t.Fatalf("jpeg metadata wrong: %+v", b)
	}
	if c.ContentType != "image/webp" || c.Width != nil || c.Height != nil {
		t.Fatalf("webp must be stored with null dimensions: %+v", c)
	}
	list := m.list(product)
	if len(list) != 3 || list[0].ID != a.ID || list[1].ID != b.ID || list[2].ID != c.ID {
		t.Fatalf("list order %+v", list)
	}
	cmiRequireContiguous(t, list)

	// Merchant preview bytes (CM3): exact bytes, validated type, private cache.
	for _, p := range []struct {
		img  cmiImage
		want []byte
	}{{a, pngBytes}, {b, jpgBytes}, {c, webpBytes}} {
		w := m.raw("GET", m.imagesPath(product)+"/"+p.img.ID, nil, nil)
		if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), p.want) {
			t.Fatalf("merchant bytes of %s: %d, equal=%v", p.img.ID, w.Code, bytes.Equal(w.Body.Bytes(), p.want))
		}
		if got := w.Header().Get("Content-Type"); got != p.img.ContentType {
			t.Errorf("merchant preview Content-Type %q, want %q", got, p.img.ContentType)
		}
		if got := w.Header().Get("Cache-Control"); got != "private, max-age=300" {
			t.Errorf("merchant preview Cache-Control %q, want private, max-age=300", got)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("merchant preview lacks nosniff")
		}
	}

	// Idempotent replay (same key + same bytes = same result, no second row, filename/part type are irrelevant);
	// another file under the same key is a 409.
	key := t04Key("cmi-replay")
	first := m.upload(product, key, cmiFile("r.png", "image/png", cmiPNG(t, 8, 8, 44)))
	again := m.upload(product, key, cmiFile("other-name.png", "application/octet-stream", cmiPNG(t, 8, 8, 44)))
	var d1, d2 cmiImage
	if first.Code != 200 || again.Code != 200 || json.Unmarshal(first.Body.Bytes(), &d1) != nil || json.Unmarshal(again.Body.Bytes(), &d2) != nil || d1.ID == "" || !reflect.DeepEqual(d1, d2) {
		t.Fatalf("idempotent replay: %d %d %s | %s", first.Code, again.Code, first.Body.String(), again.Body.String())
	}
	if n := len(m.list(product)); n != 4 {
		t.Fatalf("replay created a second row: %d images, want 4", n)
	}
	if w := m.upload(product, key, cmiFile("r.png", "image/png", cmiPNG(t, 8, 8, 45))); w.Code != 409 {
		t.Fatalf("same key, different file: %d, want 409", w.Code)
	}
	if n := len(m.list(product)); n != 4 {
		t.Fatalf("a conflicting replay stored a row: %d", n)
	}
	if w := m.remove(product, d1.ID, t04Key("cmi-rm-extra")); w.Code != 200 {
		t.Fatalf("remove replay fixture: %d", w.Code)
	}
	cmiRequireContiguous(t, m.list(product))

	// Buyer catalog (BCAT06): metadata only, in position order, same list on every SKU row, [] never null.
	catalog := cmiCatalogImages(t, h)[product]
	if len(catalog) == 0 {
		t.Fatal("product missing from the buyer catalog")
	}
	for _, imgs := range catalog {
		if len(imgs) != 3 || imgs[0].ID != a.ID || imgs[1].ID != b.ID || imgs[2].ID != c.ID {
			t.Fatalf("buyer catalog images %+v, want [a b c]", imgs)
		}
		if imgs[0].Width == nil || *imgs[0].Width != 40 || imgs[2].Width != nil {
			t.Fatalf("buyer catalog dimensions wrong: %+v", imgs)
		}
	}

	// Public bytes (BCAT07): exact bytes and the contract's headers.
	for _, p := range []struct {
		img  cmiImage
		want []byte
	}{{a, pngBytes}, {b, jpgBytes}, {c, webpBytes}} {
		r := cmiGetMedia(t, h, h.origin, product, p.img.ID, nil)
		if r.status != 200 || !bytes.Equal(r.body, p.want) {
			t.Fatalf("public bytes %s: %d equal=%v", p.img.ID, r.status, bytes.Equal(r.body, p.want))
		}
		if got := r.header.Get("Content-Type"); got != p.img.ContentType {
			t.Errorf("public Content-Type %q, want %q", got, p.img.ContentType)
		}
		if got := r.header.Get("Cache-Control"); got != "public, max-age=86400, immutable" {
			t.Errorf("public Cache-Control %q", got)
		}
		if got := r.header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("public X-Content-Type-Options %q", got)
		}
		if got := r.header.Get("Content-Security-Policy"); got != "default-src 'none'; sandbox" {
			t.Errorf("public CSP %q", got)
		}
		if r.header.Get("ETag") == "" {
			t.Error("public bytes have no ETag")
		}
	}

	// Meta feed image_link = origin + /media/p/<product>/<first image> (CM4).
	if link, ok := cmiFeedImageLink(t, h, product); !ok || link != h.origin+"/media/p/"+product+"/"+a.ID {
		t.Fatalf("feed image_link %q (row=%v), want %s/media/p/%s/%s", link, ok, h.origin, product, a.ID)
	}

	// Reorder: a permutation reorders and renumbers; the buyer catalog, the cover and the feed follow.
	if w := m.order(product, []string{c.ID, a.ID, b.ID}); w.Code != 200 {
		t.Fatalf("reorder: %d %s", w.Code, w.Body.String())
	}
	got := m.list(product)
	if len(got) != 3 || got[0].ID != c.ID || got[1].ID != a.ID || got[2].ID != b.ID {
		t.Fatalf("order after reorder %+v", got)
	}
	cmiRequireContiguous(t, got)
	for _, imgs := range cmiCatalogImages(t, h)[product] {
		if len(imgs) != 3 || imgs[0].ID != c.ID || imgs[1].ID != a.ID || imgs[2].ID != b.ID {
			t.Fatalf("buyer catalog did not follow the reorder: %+v", imgs)
		}
	}
	if link, _ := cmiFeedImageLink(t, h, product); link != h.origin+"/media/p/"+product+"/"+c.ID {
		t.Fatalf("feed cover did not follow the reorder: %q", link)
	}
	// A stale or foreign id list never drops or resurrects a photo (contract: anything else is 409) and changes nothing.
	for name, ids := range map[string][]string{"missing one": {c.ID, a.ID}, "extra id": {c.ID, a.ID, b.ID, randomUUID()}, "foreign id": {c.ID, a.ID, randomUUID()}} {
		if w := m.order(product, ids); w.Code != 409 {
			t.Errorf("reorder %s: %d %s, want 409", name, w.Code, w.Body.String())
		}
	}
	for name, ids := range map[string][]string{"empty": {}, "duplicate": {c.ID, c.ID, a.ID}, "not a uuid": {c.ID, a.ID, "x"}} {
		if w := m.order(product, ids); w.Code < 400 || w.Code >= 500 {
			t.Errorf("reorder %s: %d %s, want a 4xx", name, w.Code, w.Body.String())
		}
	}
	if now := m.ids(product); !reflect.DeepEqual(now, []string{c.ID, a.ID, b.ID}) {
		t.Fatalf("a refused reorder changed the order: %v", now)
	}

	// Delete the middle one: the rest are renumbered contiguously, the bytes 404 everywhere, replay is stable.
	rmKey := t04Key("cmi-rm")
	w := m.remove(product, a.ID, rmKey)
	if w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	var left struct {
		Items []cmiImage `json:"items"`
	}
	if json.Unmarshal(w.Body.Bytes(), &left) != nil || len(left.Items) != 2 || left.Items[0].ID != c.ID || left.Items[1].ID != b.ID {
		t.Fatalf("delete answer %s", w.Body.String())
	}
	cmiRequireContiguous(t, left.Items)
	cmiRequireContiguous(t, m.list(product))
	if w2 := m.remove(product, a.ID, rmKey); w2.Code != 200 || !bytes.Equal(w.Body.Bytes(), w2.Body.Bytes()) {
		t.Fatalf("delete replay: %d %s", w2.Code, w2.Body.String())
	}
	if w3 := m.remove(product, a.ID, t04Key("cmi-rm-again")); w3.Code != 404 {
		t.Fatalf("deleting a deleted photo: %d, want 404", w3.Code)
	}
	if w := m.raw("GET", m.imagesPath(product)+"/"+a.ID, nil, nil); w.Code != 404 {
		t.Fatalf("merchant bytes of a deleted photo: %d", w.Code)
	}
	if r := cmiGetMedia(t, h, h.origin, product, a.ID, nil); r.status != 404 {
		t.Fatalf("public bytes of a deleted photo: %d", r.status)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, product); n != 2 {
		t.Fatalf("%d image rows remain, want 2", n)
	}

	// Cap: 8 photos; the 9th is 409 and stores nothing; positions are exactly 0..7.
	for seed := uint8(100); len(m.list(product)) < 8; seed++ {
		m.mustUpload(product, cmiPNG(t, 6, 6, seed), "fill.png", "image/png")
	}
	full := m.list(product)
	cmiRequireContiguous(t, full)
	if w := m.upload(product, t04Key("cmi-ninth"), cmiFile("nine.png", "image/png", cmiPNG(t, 6, 6, 250))); w.Code != 409 {
		t.Fatalf("9th photo: %d %s, want 409", w.Code, w.Body.String())
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, product); n != 8 {
		t.Fatalf("%d rows after the refused 9th, want 8", n)
	}
	// Deleting one frees a slot again and renumbers.
	if w := m.remove(product, full[3].ID, t04Key("cmi-rm-mid")); w.Code != 200 {
		t.Fatalf("delete from a full set: %d", w.Code)
	}
	cmiRequireContiguous(t, m.list(product))
	m.mustUpload(product, cmiPNG(t, 6, 6, 251), "again.png", "image/png")
	if n := len(m.list(product)); n != 8 {
		t.Fatalf("%d after delete+upload, want 8", n)
	}
	cmiRequireContiguous(t, m.list(product))
}

// =============================================================================================================================
// Magic-byte truth and the multipart grammar (CM2).
// =============================================================================================================================

func TestCatalogMediaSniff(t *testing.T) {
	h, m, product := cmiEnv(t)
	f := h.f
	rows := func() int {
		return countRows(t, f.owner, `SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, product)
	}
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><script>alert(1)</script></svg>`)
	gif := append([]byte("GIF89a"), bytes.Repeat([]byte{0}, 40)...)
	refused := map[string]struct {
		file   cmiPart
		status int
	}{
		"svg as image/svg+xml":       {cmiFile("x.svg", "image/svg+xml", svg), 422},
		"svg lying image/png .png":   {cmiFile("x.png", "image/png", svg), 422},
		"gif as image/gif":           {cmiFile("x.gif", "image/gif", gif), 422},
		"gif lying image/jpeg .jpg":  {cmiFile("x.jpg", "image/jpeg", gif), 422},
		"html lying image/jpeg":      {cmiFile("x.jpg", "image/jpeg", []byte("<html><script>1</script></html>")), 422},
		"jpeg magic, garbage header": {cmiFile("x.jpg", "image/jpeg", append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{0x41}, 64)...)), 422},
		"png magic, truncated":       {cmiFile("x.png", "image/png", cmiPNG(t, 20, 20, 1)[:12]), 422},
		"RIFF but WAVE, not WEBP":    {cmiFile("x.webp", "image/webp", append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), bytes.Repeat([]byte{0}, 32)...)), 422},
		"empty file":                 {cmiFile("x.png", "image/png", nil), 422},
		"2 MiB + 1 byte":             {cmiFile("big.jpg", "image/jpeg", cmiPadded(t, 2<<20+1)), 413},
		"3 MiB":                      {cmiFile("huge.jpg", "image/jpeg", cmiPadded(t, 3<<20)), 413},
	}
	for name, c := range refused {
		w := m.upload(product, t04Key("cmi-sniff"), c.file)
		if w.Code != c.status {
			t.Errorf("%s: %d %s, want %d", name, w.Code, w.Body.String(), c.status)
		}
		if n := rows(); n != 0 {
			t.Fatalf("%s: a refused upload stored %d rows", name, n)
		}
	}

	// The type is the bytes' truth, never the client's claim: a PNG sent as image/jpeg named .jpg is stored and served as image/png.
	png1 := cmiPNG(t, 12, 9, 5)
	img := m.mustUpload(product, png1, "photo.jpg", "image/jpeg")
	if img.ContentType != "image/png" || img.Width == nil || *img.Width != 12 {
		t.Fatalf("PNG declared as JPEG: stored as %+v, want image/png 12x9", img)
	}
	if w := m.raw("GET", m.imagesPath(product)+"/"+img.ID, nil, nil); w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), png1) {
		t.Fatalf("PNG declared as JPEG served as %q", w.Header().Get("Content-Type"))
	}
	if r := cmiGetMedia(t, h, h.origin, product, img.ID, nil); r.header.Get("Content-Type") != "image/png" {
		t.Fatalf("public bytes of PNG declared as JPEG served as %q", r.header.Get("Content-Type"))
	}
	if got := m.mustUpload(product, cmiJPEG(t, 10, 10, 6), "photo.png", "image/png"); got.ContentType != "image/jpeg" {
		t.Fatalf("JPEG declared as PNG stored as %s", got.ContentType)
	}
	// Exactly 2 MiB is accepted (a JPEG with trailing padding), and is served back byte for byte.
	exact := cmiPadded(t, 2<<20)
	big := m.mustUpload(product, exact, "exact.jpg", "image/jpeg")
	if big.SizeBytes != 2<<20 || big.ContentType != "image/jpeg" {
		t.Fatalf("2 MiB boundary: %+v", big)
	}
	if w := m.raw("GET", m.imagesPath(product)+"/"+big.ID, nil, nil); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), exact) {
		t.Fatalf("2 MiB image did not round-trip: %d", w.Code)
	}
	before := rows()

	// Multipart grammar and transport authority; every refusal stores nothing.
	okPNG := cmiPNG(t, 5, 5, 9)
	check := func(name string, w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Errorf("%s: %d %s, want %d", name, w.Code, w.Body.String(), want)
		}
		if n := rows(); n != before {
			t.Fatalf("%s changed the image count %d -> %d", name, before, n)
		}
	}
	wrongField, ctWrong := cmiMultipart(t, cmiPart{"image", "x.png", "image/png", okPNG})
	check("wrong field name", m.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": ctWrong, "Idempotency-Key": t04Key("cmi-g1")}, wrongField), 422)
	two, ct2 := cmiMultipart(t, cmiFile("a.png", "image/png", okPNG), cmiFile("b.png", "image/png", cmiPNG(t, 5, 5, 10)))
	check("two file parts", m.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": ct2, "Idempotency-Key": t04Key("cmi-g2")}, two), 422)
	check("JSON body", m.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": "application/json", "Idempotency-Key": t04Key("cmi-g3")}, []byte(`{"file":"x"}`)), 415)
	one, ct1 := cmiMultipart(t, cmiFile("a.png", "image/png", okPNG))
	check("no Idempotency-Key", m.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": ct1}, one), 422)
	noAuth := cmiMerchant{t: t, h: m.h, store: m.store}
	check("no Authorization", noAuth.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": ct1, "Idempotency-Key": t04Key("cmi-g4")}, one), 401)
	bogus := cmiMerchant{t: t, h: m.h, store: m.store, token: "0123456789abcdef0123456789abcdef"}
	check("well-formed but unknown bearer with a valid file", bogus.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": ct1, "Idempotency-Key": t04Key("cmi-g4b")}, one), 401)
	buyerTok := cmiMerchant{t: t, h: m.h, store: m.store, token: f.tokens["buyer"]}
	check("buyer session token", buyerTok.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": ct1, "Idempotency-Key": t04Key("cmi-g5")}, one), 401)
	// A spoofed tenant header is ignored: the upload lands in the authenticated scope.
	spoof := m.raw("POST", m.imagesPath(product), map[string]string{"Content-Type": ct1, "Idempotency-Key": t04Key("cmi-g6"), "X-Tenant-ID": f.tenantB}, one)
	if spoof.Code != 200 {
		t.Fatalf("spoofed-header upload: %d %s", spoof.Code, spoof.Body.String())
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_images WHERE product_id=$1 AND tenant_id=$2 AND store_id=$3`, product, f.tenantA, f.storeA1); n != before+1 {
		t.Fatalf("spoofed-header upload landed outside the authenticated scope (%d in scope, want %d)", n, before+1)
	}

	// Cross-store / unknown product: invisible (404), never an empty list or a write.
	otherProduct := t04CreateStock(t, f, f.tokens["a2"], f.storeA2, 2).product.ID
	cmiCleanup(t, f, otherProduct)
	for name, w := range map[string]*httptest.ResponseRecorder{
		"upload to a product of another store": m.upload(otherProduct, t04Key("cmi-x1"), cmiFile("a.png", "image/png", okPNG)),
		"list a product of another store":      m.raw("GET", m.imagesPath(otherProduct), nil, nil),
		"upload to an unknown product":         m.upload(randomUUID(), t04Key("cmi-x2"), cmiFile("a.png", "image/png", okPNG)),
		"list an unknown product":              m.raw("GET", m.imagesPath(randomUUID()), nil, nil),
	} {
		if w.Code != 404 {
			t.Errorf("%s: %d, want 404", name, w.Code)
		}
	}
	other := cmiMerchant{t: t, h: m.h, token: f.tokens["a2"], store: f.storeA2}
	foreignImage := other.mustUpload(otherProduct, cmiPNG(t, 5, 5, 77), "o.png", "image/png")
	if w := m.raw("GET", m.imagesPath(product)+"/"+foreignImage.ID, nil, nil); w.Code != 404 {
		t.Errorf("preview of another store's image through this store's product path: %d, want 404", w.Code)
	}
	if w := m.remove(product, foreignImage.ID, t04Key("cmi-x3")); w.Code != 404 {
		t.Errorf("delete of another store's image through this store's product path: %d, want 404", w.Code)
	}
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_images WHERE id=$1`, foreignImage.ID); n != 1 {
		t.Error("a cross-store delete removed the foreign image")
	}
}

// =============================================================================================================================
// BCAT07: every non-serving case is the same 404.
// =============================================================================================================================

func TestCatalogMediaBuyer404(t *testing.T) {
	h, m, product := cmiEnv(t)
	f := h.f
	good := m.mustUpload(product, cmiPNG(t, 9, 9, 1), "g.png", "image/png")
	// a second active product of the same store, one in another store of the same tenant, one of another tenant, one archived
	sibling := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 2).product.ID
	otherStore := t04CreateStock(t, f, f.tokens["a2"], f.storeA2, 2).product.ID
	otherTenant := t04CreateStock(t, f, f.tokens["b"], f.storeB, 2).product.ID
	archived := t04CreateStock(t, f, f.tokens["a"], f.storeA1, 2).product.ID
	cmiCleanup(t, f, sibling, otherStore, otherTenant, archived)
	siblingImg := m.mustUpload(sibling, cmiPNG(t, 9, 9, 2), "s.png", "image/png")
	archivedImg := m.mustUpload(archived, cmiPNG(t, 9, 9, 3), "a.png", "image/png")
	storeImg := cmiMerchant{t: t, h: m.h, token: f.tokens["a2"], store: f.storeA2}.mustUpload(otherStore, cmiPNG(t, 9, 9, 4), "o.png", "image/png")
	tenantImg := cmiMerchant{t: t, h: m.h, token: f.tokens["b"], store: f.storeB}.mustUpload(otherTenant, cmiPNG(t, 9, 9, 5), "t.png", "image/png")
	m.archive(f, archived)

	// control: the valid request serves, so the 404s below are not a misconfiguration
	if r := cmiGetMedia(t, h, h.origin, product, good.ID, nil); r.status != 200 {
		t.Fatalf("control request: %d %s", r.status, r.body)
	}
	missing := map[string]cmiMedia{
		"unknown image id":                     cmiGetMedia(t, h, h.origin, product, randomUUID(), nil),
		"unknown product id":                   cmiGetMedia(t, h, h.origin, randomUUID(), good.ID, nil),
		"image of another product, same store": cmiGetMedia(t, h, h.origin, product, siblingImg.ID, nil),
		"correct pair of another product":      cmiGetMedia(t, h, h.origin, sibling, good.ID, nil),
		"image of another store (same tenant)": cmiGetMedia(t, h, h.origin, otherStore, storeImg.ID, nil),
		"image of another tenant":              cmiGetMedia(t, h, h.origin, otherTenant, tenantImg.ID, nil),
		"archived product":                     cmiGetMedia(t, h, h.origin, archived, archivedImg.ID, nil),
		"origin never published":               cmiGetMedia(t, h, "https://never-published.example", product, good.ID, nil),
	}
	mustExec(t, f.owner, `UPDATE control.storefront_publications SET published=false WHERE store_id=$1`, f.storeA1)
	missing["store unpublished"] = cmiGetMedia(t, h, h.origin, product, good.ID, nil)
	mustExec(t, f.owner, `UPDATE control.storefront_publications SET published=true WHERE store_id=$1`, f.storeA1)
	if r := cmiGetMedia(t, h, h.origin, product, good.ID, nil); r.status != 200 {
		t.Fatalf("publication was not restored: %d", r.status)
	}
	var reference cmiMedia
	for name, r := range missing {
		if r.status != 404 {
			t.Errorf("%s: %d %s, want 404", name, r.status, r.body)
			continue
		}
		if bytes.Contains(r.body, []byte(good.ID)) || bytes.Contains(r.body, []byte(siblingImg.ID)) {
			t.Errorf("%s: the 404 body echoes an id", name)
		}
		if reference.body == nil {
			reference = r
			continue
		}
		if !bytes.Equal(cmiNormalizeRequestID(reference.body), cmiNormalizeRequestID(r.body)) || reference.header.Get("Content-Type") != r.header.Get("Content-Type") || reference.header.Get("Cache-Control") != r.header.Get("Cache-Control") {
			t.Errorf("%s: 404 differs from the others (existence oracle): %q vs %q", name, r.body, reference.body)
		}
	}
	// Malformed input is 422, not 404.
	if r := cmiGetMedia(t, h, h.origin, "not-a-uuid", good.ID, nil); r.status != 422 {
		t.Errorf("malformed product id: %d, want 422", r.status)
	}
	if r := cmiGetMedia(t, h, "http://buyer.example/path", product, good.ID, nil); r.status != 422 {
		t.Errorf("malformed origin: %d, want 422", r.status)
	}
	// The buyer catalog never lists another store's or an archived product's photo either (BCAT06).
	catalog := cmiCatalogImages(t, h)
	for _, p := range []string{otherStore, otherTenant, archived} {
		if _, ok := catalog[p]; ok {
			t.Errorf("buyer catalog lists product %s that must be invisible", p)
		}
	}
	if got := catalog[sibling]; len(got) == 0 || len(got[0]) != 1 || got[0][0].ID != siblingImg.ID {
		t.Errorf("sibling images wrong: %+v", got)
	}
	if got := catalog[product]; len(got) == 0 || len(got[0]) != 1 || got[0][0].ID != good.ID {
		t.Errorf("product images wrong (must not include another product's photo): %+v", got)
	}
}

// =============================================================================================================================
// The media route's transport authority (contracts/buyer-catalog-discovery-v1.md amendment).
// =============================================================================================================================

func TestCatalogMediaBuyerAuthority(t *testing.T) {
	h, m, product := cmiEnv(t)
	data := cmiPNG(t, 9, 9, 8)
	img := m.mustUpload(product, data, "a.png", "image/png")
	for name, c := range map[string]struct {
		edit   func(*http.Request)
		origin string
		want   []int
	}{
		// contracts/buyer-catalog-discovery-v1.md: "an Authorization header, cookie, query string, body or Idempotency-Key is refused (403 / 422)"
		"Authorization bearer is refused": {func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+h.cap.Token) }, h.origin, []int{401, 403}},
		"cookie is refused":               {func(r *http.Request) { r.Header.Set("Cookie", "a=b") }, h.origin, []int{403, 422}},
		"query string is refused":         {func(r *http.Request) { r.URL.RawQuery = "x=1" }, h.origin, []int{403, 422}},
		"Idempotency-Key is refused":      {func(r *http.Request) { r.Header.Set("Idempotency-Key", "k") }, h.origin, []int{403, 422}},
		"request body is refused":         {func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader("x")); r.ContentLength = 1 }, h.origin, []int{403, 422}},
		"wrong BFF key":                   {func(r *http.Request) { r.Header.Set("X-Commerce-Buyer-BFF-Key", "wrong") }, h.origin, []int{401, 403, 404}},
		"missing BFF key":                 {func(r *http.Request) { r.Header.Del("X-Commerce-Buyer-BFF-Key") }, h.origin, []int{401, 403, 404}},
		"missing origin header":           {nil, "", []int{422}},
		"POST is not a media read":        {func(r *http.Request) { r.Method = "POST" }, h.origin, []int{404, 405}},
	} {
		r := cmiGetMedia(t, h, c.origin, product, img.ID, c.edit)
		ok := false
		for _, w := range c.want {
			ok = ok || r.status == w
		}
		if !ok {
			t.Errorf("%s: %d %s, want one of %v", name, r.status, r.body, c.want)
		}
		if bytes.Equal(r.body, data) {
			t.Errorf("%s: served the bytes", name)
		}
	}
	// control: the plain request still serves.
	if r := cmiGetMedia(t, h, h.origin, product, img.ID, nil); r.status != 200 || !bytes.Equal(r.body, data) {
		t.Fatalf("control: %d", r.status)
	}
}

// =============================================================================================================================
// Archived product.
// =============================================================================================================================

func TestCatalogMediaArchivedProduct(t *testing.T) {
	h, m, product := cmiEnv(t)
	a := m.mustUpload(product, cmiPNG(t, 9, 9, 21), "a.png", "image/png")
	b := m.mustUpload(product, cmiPNG(t, 9, 9, 22), "b.png", "image/png")
	if link, _ := cmiFeedImageLink(t, h, product); link == "" {
		t.Fatal("feed has no image_link before the archive")
	}
	m.archive(h.f, product)
	if w := m.upload(product, t04Key("cmi-arch-up"), cmiFile("c.png", "image/png", cmiPNG(t, 9, 9, 23))); w.Code != 409 {
		t.Errorf("upload to an archived product: %d %s, want 409", w.Code, w.Body.String())
	}
	if r := cmiGetMedia(t, h, h.origin, product, a.ID, nil); r.status != 404 {
		t.Errorf("public bytes of an archived product: %d, want 404", r.status)
	}
	if _, ok := cmiCatalogImages(t, h)[product]; ok {
		t.Error("buyer catalog still lists the archived product")
	}
	if link, ok := cmiFeedImageLink(t, h, product); ok || link != "" {
		t.Errorf("feed still lists the archived product (%q)", link)
	}
	// The merchant can still see and clean up the photos (contract: delete is allowed on an archived product).
	if n := m.ids(product); len(n) != 2 || n[0] != a.ID || n[1] != b.ID {
		t.Errorf("merchant list of an archived product: %v", n)
	}
	if w := m.remove(product, a.ID, t04Key("cmi-arch-rm")); w.Code != 200 {
		t.Errorf("delete on an archived product: %d %s", w.Code, w.Body.String())
	}
	cmiRequireContiguous(t, m.list(product))
}
