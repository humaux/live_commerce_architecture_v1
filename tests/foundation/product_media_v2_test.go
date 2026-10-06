package foundation_test

// Purpose: REAL_PG gates of unit product-media-v2 (contracts/catalog-inventory-v1.md "Amendment — product-media-v2", migration
// 0149): roles main/detail/sku, per-role caps in SQL and Go, option-value image links, buyer detail/catalog shapes, Meta feed
// links, and the 9-image forward migration. Evidence class REAL_PG (no browser here; browser gates are PM-U).
// Depends on: the catalog-media helpers of catalog_media_test.go (cmiEnv, cmiMerchant, cmiJPEG), buyer_http_test.go (bhHarness),
//   migrations (0149), internal/httpapi merchant routes, internal/buyerhttp, internal/attribution.FeedHandler.
// Used by: bash scripts/dev/test-focused.sh '^TestPMv2' (PG lock) and the foundation shards on CI.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"livecommerce/internal/attribution"
	"livecommerce/internal/catalog"
	"livecommerce/migrations"
)

// ---- fixture: a product with two option axes (Color: Red/Blue/Green, Size: S/M) and four active SKUs --------------------------

type pmProduct struct {
	id, slug string
	skus     map[string]string // "Red/S" -> sku id
}

func pmNewProduct(t *testing.T, h bhHarness) pmProduct {
	t.Helper()
	f := h.f
	p := pmProduct{id: randomUUID(), slug: "pm-" + randomUUID()[:8], skus: map[string]string{}}
	mustExec(t, f.owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name,description,status,slug,options)
		VALUES($1,$2,$3,'PM tee','d','active',$4,$5::jsonb)`, f.tenantA, f.storeA1, p.id, p.slug,
		`[{"name":"Color","values":["Red","Blue","Green"]},{"name":"Size","values":["S","M"]}]`)
	for _, combo := range [][]string{{"Red", "S"}, {"Red", "M"}, {"Blue", "S"}, {"Green", "S"}} {
		id := randomUUID()
		mustExec(t, f.owner, `INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,status,currency,price_minor,option_values)
			VALUES($1,$2,$3,$4,$5,'active',(SELECT currency FROM control.stores WHERE id=$2),1000,$6)`,
			f.tenantA, f.storeA1, id, p.id, "PM-"+id[:8], combo)
		p.skus[strings.Join(combo, "/")] = id
	}
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM catalog.product_images WHERE product_id=$1`, p.id)
		mustExec(t, f.owner, `DELETE FROM catalog.skus WHERE product_id=$1`, p.id)
		mustExec(t, f.owner, `DELETE FROM catalog.products WHERE id=$1`, p.id)
	})
	return p
}

func pmUpload(m cmiMerchant, product, role, optionValue string, data []byte, key string) *httptest.ResponseRecorder {
	q := "?role=" + role
	if optionValue != "" {
		q += "&option_value=" + url.QueryEscape(optionValue)
	}
	body, ct := cmiMultipart(m.t, cmiFile("x.jpg", "image/jpeg", data))
	return m.raw("POST", m.imagesPath(product)+q, map[string]string{"Content-Type": ct, "Idempotency-Key": key}, body)
}

func pmMust(m cmiMerchant, product, role, optionValue string, seed uint8) cmiImage {
	m.t.Helper()
	w := pmUpload(m, product, role, optionValue, cmiJPEG(m.t, 40, 30, seed), t04Key("pm-up"))
	if w.Code != 200 {
		m.t.Fatalf("upload role=%s value=%q: %d %s", role, optionValue, w.Code, w.Body.String())
	}
	var img cmiImage
	if err := json.Unmarshal(w.Body.Bytes(), &img); err != nil {
		m.t.Fatal(err)
	}
	return img
}

func pmPost(m cmiMerchant, path string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	return m.raw("POST", "/v1/admin/stores/"+m.store+"/products/"+path, map[string]string{"Content-Type": "application/json", "Idempotency-Key": t04Key("pm-post")}, data)
}

type pmList struct {
	Items []struct {
		ID       string `json:"id"`
		Role     string `json:"role"`
		Position int    `json:"position"`
	} `json:"items"`
	ImageAxis    *string `json:"image_axis"`
	OptionImages []struct {
		OptionName  string `json:"option_name"`
		OptionValue string `json:"option_value"`
		ImageID     string `json:"image_id"`
	} `json:"option_images"`
}

func pmGet(m cmiMerchant, product string) pmList {
	m.t.Helper()
	w := m.raw("GET", m.imagesPath(product), nil, nil)
	if w.Code != 200 {
		m.t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var out pmList
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		m.t.Fatal(err)
	}
	return out
}

func (l pmList) ids(role string) (out []string) {
	for _, i := range l.Items {
		if i.Role == role {
			out = append(out, i.ID)
		}
	}
	return out
}

// ---- constants parity ------------------------------------------------------------------------------------------------------

func TestPMv2ConstantsParity(t *testing.T) {
	if catalog.MaxMainImages != 4 || catalog.MaxDetailImages != 20 || catalog.MaxOptionImages != 50 {
		t.Fatalf("caps main=%d detail=%d option=%d, want 4/20/50 (frontend parsers pin the same numbers)", catalog.MaxMainImages, catalog.MaxDetailImages, catalog.MaxOptionImages)
	}
}

// ---- migration of a populated database ---------------------------------------------------------------------------------------

// PM1: a pre-0149 database with a 9-image product becomes 4 main + 5 detail, order preserved, nothing deleted.
func TestPMv2MigrationNineImages(t *testing.T) {
	ctx := context.Background()
	owner := mciStartPG(t) // skips (NOT_RUN) without LC_TEST_DATABASE_ALLOWED=1
	files, err := filepath.Glob("../../migrations/[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil || len(files) < 100 {
		t.Fatalf("migration files: %d %v", len(files), err)
	}
	mustExec(t, owner, `CREATE TABLE public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	var held []string
	for _, f := range files {
		version := filepath.Base(f)
		if version < "0149" {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, version)
		mustExec(t, owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, version, fmt.Sprintf("%x", sha256.Sum256(body)))
	}
	if len(held) == 0 || held[0] != "0149_product_media_v2.sql" {
		t.Fatalf("held-back migrations %v, want 0149_product_media_v2.sql first", held)
	}
	waPrecreateRoles(t, owner)
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("apply every migration before 0149: %v", err)
	}
	if n := countRows(t, owner, `SELECT count(*) FROM information_schema.columns WHERE table_schema='catalog' AND table_name='product_images' AND column_name='role'`); n != 0 {
		t.Fatal("pre-0149 database already has product_images.role")
	}
	tenant, store := randomUUID(), randomUUID()
	mustExec(t, owner, `INSERT INTO control.tenants(id,name) VALUES ($1,'pm1')`, tenant)
	mustExec(t, owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES ($1,$2,'PM1 Shop','TWD')`, tenant, store)
	nine, two, none, gap := randomUUID(), randomUUID(), randomUUID(), randomUUID()
	for _, p := range []string{nine, two, none, gap} {
		mustExec(t, owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name,description,status) VALUES($1,$2,$3,'p','d','active')`, tenant, store, p)
	}
	imageIDs := map[string][]string{}
	seedAt := func(product string, positions ...int) {
		for _, pos := range positions {
			id := randomUUID()
			mustExec(t, owner, `INSERT INTO catalog.product_images(tenant_id,store_id,product_id,id,position,content_type,bytes,sha256) VALUES($1,$2,$3,$4,$5,'image/png','\x01',$6)`,
				tenant, store, product, id, pos, bytes.Repeat([]byte{byte(pos + 1)}, 32))
			imageIDs[product] = append(imageIDs[product], id)
		}
	}
	seed := func(product string, n int) {
		positions := make([]int, n)
		for i := range positions {
			positions[i] = i
		}
		seedAt(product, positions...)
	}
	seed(nine, 9)
	seed(two, 2)
	// Review P2-2: a gallery with gaps (0,1,2,5,7) must land on main 0..2 and detail 0..1, not detail 1,3 (position-4).
	seedAt(gap, 0, 1, 2, 5, 7)
	for _, version := range held {
		mustExec(t, owner, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, version)
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("0149 on the populated database: %v", err)
	}
	read := func(product, role string) (ids []string, positions []int) {
		rows, err := owner.Query(ctx, `SELECT id::text,position FROM catalog.product_images WHERE product_id=$1 AND role=$2 ORDER BY position`, product, role)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var pos int
			if err := rows.Scan(&id, &pos); err != nil {
				t.Fatal(err)
			}
			ids, positions = append(ids, id), append(positions, pos)
		}
		return ids, positions
	}
	main9, mp := read(nine, "main")
	detail9, dp := read(nine, "detail")
	if fmt.Sprint(main9) != fmt.Sprint(imageIDs[nine][:4]) || fmt.Sprint(mp) != "[0 1 2 3]" {
		t.Fatalf("main of the 9-image product = %v %v, want the first four in order", main9, mp)
	}
	if fmt.Sprint(detail9) != fmt.Sprint(imageIDs[nine][4:]) || fmt.Sprint(dp) != "[0 1 2 3 4]" {
		t.Fatalf("detail of the 9-image product = %v %v, want images 4..8 renumbered from 0", detail9, dp)
	}
	main2, _ := read(two, "main")
	if fmt.Sprint(main2) != fmt.Sprint(imageIDs[two]) {
		t.Fatalf("2-image product changed: %v", main2)
	}
	mainGap, _ := read(gap, "main")
	detailGap, gp := read(gap, "detail")
	if fmt.Sprint(mainGap) != fmt.Sprint(imageIDs[gap][:3]) || fmt.Sprint(detailGap) != fmt.Sprint(imageIDs[gap][3:]) || fmt.Sprint(gp) != "[0 1]" {
		t.Fatalf("gap gallery: main %v detail %v %v, want first three main and detail renumbered [0 1]", mainGap, detailGap, gp)
	}
	if n := countRows(t, owner, `SELECT count(*) FROM catalog.product_images`); n != 16 {
		t.Fatalf("%d image rows after the migration, want 16 (nothing deleted)", n)
	}
	if n := countRows(t, owner, `SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, none); n != 0 {
		t.Fatal("product without images gained rows")
	}
	ledger := crExplicitDigest(t, owner, "public.lc_schema_migrations", "applied_at")
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if crExplicitDigest(t, owner, "public.lc_schema_migrations", "applied_at") != ledger {
		t.Error("a second Apply changed the ledger")
	}
}

// ---- SQL structure: caps, ranges and option-value link rules -------------------------------------------------------------------

// PM2/PM3 in SQL: a 5th main and a 21st detail have no legal position; links are structurally tied to a same-product sku image of a
// real value on the effective axis; one image per value, one value per image; cross-store refused.
func TestPMv2SQLStructure(t *testing.T) {
	h, _, _ := cmiEnv(t)
	f := h.f
	ctx := context.Background()
	p := pmNewProduct(t, h)
	other := pmNewProduct(t, h) // same store, other product
	foreign := t04CreateStock(t, f, f.tokens["a2"], f.storeA2, 3).product.ID
	cmiCleanup(t, f, foreign)
	sha := bytes.Repeat([]byte{9}, 32)
	insert := func(tenant, store, product, id, role string, pos int) error {
		_, err := f.owner.Exec(ctx, `INSERT INTO catalog.product_images(tenant_id,store_id,product_id,id,role,position,content_type,bytes,sha256) VALUES($1,$2,$3,$4,$5,$6,'image/png','\x01',$7)`,
			tenant, store, product, id, role, pos, sha)
		return err
	}
	for _, c := range []struct {
		role string
		pos  int
		ok   bool
	}{{"main", 3, true}, {"main", 4, false}, {"detail", 19, true}, {"detail", 20, false}, {"sku", 49, true}, {"sku", 50, false}, {"bogus", 0, false}} {
		err := insert(f.tenantA, f.storeA1, p.id, randomUUID(), c.role, c.pos)
		if c.ok != (err == nil) {
			t.Errorf("role=%s position=%d accepted=%v err=%v", c.role, c.pos, err == nil, err)
		} else if err != nil && cmiErrCode(t, err) != "23514" {
			t.Errorf("role=%s position=%d: %s, want CHECK 23514", c.role, c.pos, cmiErrCode(t, err))
		}
	}
	mustExec(t, f.owner, `DELETE FROM catalog.product_images WHERE product_id=$1`, p.id)

	skuImg, mainImg, otherImg, foreignImg := randomUUID(), randomUUID(), randomUUID(), randomUUID()
	for _, r := range []struct{ tenant, store, product, id, role string }{
		{f.tenantA, f.storeA1, p.id, skuImg, "sku"}, {f.tenantA, f.storeA1, p.id, mainImg, "main"},
		{f.tenantA, f.storeA1, other.id, otherImg, "sku"}, {f.tenantA, f.storeA2, foreign, foreignImg, "sku"}} {
		if err := insert(r.tenant, r.store, r.product, r.id, r.role, 0); err != nil {
			t.Fatal(err)
		}
	}
	link := func(product, name, value, image string, store string) error {
		_, err := f.owner.Exec(ctx, `INSERT INTO catalog.product_option_images(tenant_id,store_id,product_id,option_name,option_value,image_id) VALUES($1,$2,$3,$4,$5,$6)`,
			f.tenantA, store, product, name, value, image)
		return err
	}
	refuse := func(name string, err error, codes ...string) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: link accepted", name)
			mustExec(t, f.owner, `DELETE FROM catalog.product_option_images WHERE product_id=$1`, p.id)
			return
		}
		code := cmiErrCode(t, err)
		for _, c := range codes {
			if code == c {
				return
			}
		}
		t.Errorf("%s: error %s, want one of %v", name, code, codes)
	}
	refuse("main-role image", link(p.id, "Color", "Red", mainImg, f.storeA1), "23503")
	refuse("image of another product", link(p.id, "Color", "Red", otherImg, f.storeA1), "23503")
	refuse("image of another store", link(p.id, "Color", "Red", foreignImg, f.storeA2), "23503")
	refuse("value not on the axis", link(p.id, "Color", "Purple", skuImg, f.storeA1), "23514")
	refuse("axis is not the image axis (Size)", link(p.id, "Size", "S", skuImg, f.storeA1), "23514")
	refuse("axis that does not exist", link(p.id, "Material", "Red", skuImg, f.storeA1), "23514")
	if err := link(p.id, "Color", "Red", skuImg, f.storeA1); err != nil {
		t.Fatalf("legal link refused: %v", err)
	}
	second := randomUUID()
	if err := insert(f.tenantA, f.storeA1, p.id, second, "sku", 1); err != nil {
		t.Fatal(err)
	}
	refuse("second image for one value", link(p.id, "Color", "Red", second, f.storeA1), "23505")
	refuse("one image for a second value", link(p.id, "Color", "Blue", skuImg, f.storeA1), "23505")
	// deleting the image removes its link (ON DELETE CASCADE)
	mustExec(t, f.owner, `DELETE FROM catalog.product_images WHERE id=$1`, skuImg)
	if n := countRows(t, f.owner, `SELECT count(*) FROM catalog.product_option_images WHERE product_id=$1`, p.id); n != 0 {
		t.Fatalf("%d links left after deleting the image", n)
	}
	// an explicit image axis moves the rule to that axis
	mustExec(t, f.owner, `UPDATE catalog.products SET image_axis='Size' WHERE id=$1`, p.id)
	if err := link(p.id, "Size", "M", second, f.storeA1); err != nil {
		t.Fatalf("link on the explicit axis refused: %v", err)
	}
	refuse("first axis after switching", link(p.id, "Color", "Blue", second, f.storeA1), "23505", "23514")
}

// ---- merchant API: caps, roles, move, reorder, delete, links -------------------------------------------------------------------

func TestPMv2MerchantRolesAndCaps(t *testing.T) {
	h, m, _ := cmiEnv(t)
	p := pmNewProduct(t, h)
	// 5th main refused (409), stored nothing
	for i := 0; i < catalog.MaxMainImages; i++ {
		pmMust(m, p.id, "main", "", uint8(10+i))
	}
	if w := pmUpload(m, p.id, "main", "", cmiJPEG(t, 40, 30, 99), t04Key("pm-5th")); w.Code != 409 {
		t.Fatalf("5th main: %d %s, want 409", w.Code, w.Body.String())
	}
	// default role is main
	if w := m.upload(p.id, t04Key("pm-default"), cmiFile("x.jpg", "image/jpeg", cmiJPEG(t, 40, 30, 98))); w.Code != 409 {
		t.Fatalf("default-role upload on a full main: %d, want 409", w.Code)
	}
	for i := 0; i < catalog.MaxDetailImages; i++ {
		pmMust(m, p.id, "detail", "", uint8(40+i))
	}
	if w := pmUpload(m, p.id, "detail", "", cmiJPEG(t, 40, 30, 97), t04Key("pm-21st")); w.Code != 409 {
		t.Fatalf("21st detail: %d %s, want 409", w.Code, w.Body.String())
	}
	if w := pmUpload(m, p.id, "gallery", "", cmiJPEG(t, 40, 30, 96), t04Key("pm-badrole")); w.Code != 422 {
		t.Fatalf("unknown role: %d, want 422", w.Code)
	}
	l := pmGet(m, p.id)
	if len(l.ids("main")) != 4 || len(l.ids("detail")) != 20 {
		t.Fatalf("lists main=%d detail=%d", len(l.ids("main")), len(l.ids("detail")))
	}
	for role, n := range map[string]int{"main": 4, "detail": 20} {
		pos := 0
		for _, i := range l.Items {
			if i.Role == role {
				if i.Position != pos {
					t.Fatalf("%s position %d at index %d", role, i.Position, pos)
				}
				pos++
			}
		}
		if pos != n {
			t.Fatalf("%s count %d", role, pos)
		}
	}
	// move: main -> detail is refused while detail is full; delete one detail then move
	main0 := l.ids("main")[0]
	if w := pmPost(m, p.id+"/images/"+main0+"/move", map[string]string{"role": "detail"}); w.Code != 409 {
		t.Fatalf("move into a full detail: %d %s, want 409", w.Code, w.Body.String())
	}
	if w := m.remove(p.id, l.ids("detail")[3], t04Key("pm-del")); w.Code != 200 {
		t.Fatalf("delete detail: %d %s", w.Code, w.Body.String())
	}
	if w := pmPost(m, p.id+"/images/"+main0+"/move", map[string]string{"role": "detail"}); w.Code != 200 {
		t.Fatalf("move: %d %s", w.Code, w.Body.String())
	}
	l = pmGet(m, p.id)
	if len(l.ids("main")) != 3 || len(l.ids("detail")) != 20 || l.ids("detail")[19] != main0 {
		t.Fatalf("after move: main=%d detail=%d last=%s want 3/20/%s", len(l.ids("main")), len(l.ids("detail")), l.ids("detail")[19], main0)
	}
	for i, id := range l.ids("main") { // source role renumbered, contiguous
		if l.Items[i].ID != id || l.Items[i].Position != i {
			t.Fatalf("main not contiguous after move: %+v", l.Items[:4])
		}
	}
	// reorder per role: reversed detail
	det := l.ids("detail")
	rev := make([]string, len(det))
	for i, id := range det {
		rev[len(det)-1-i] = id
	}
	if w := pmPost(m, p.id+"/images/order", map[string]any{"role": "detail", "ids": rev}); w.Code != 200 {
		t.Fatalf("reorder detail: %d %s", w.Code, w.Body.String())
	}
	if got := pmGet(m, p.id).ids("detail"); fmt.Sprint(got) != fmt.Sprint(rev) {
		t.Fatalf("detail order %v, want %v", got, rev)
	}
	// a permutation mixing roles is refused
	mixed := append([]string{l.ids("main")[0]}, rev[1:]...)
	if w := pmPost(m, p.id+"/images/order", map[string]any{"role": "detail", "ids": mixed}); w.Code != 409 {
		t.Fatalf("cross-role reorder: %d, want 409", w.Code)
	}
	// 6x ratio rule for detail (750x4000 passes, 100x700 is refused); main has no ratio limit
	if w := pmUpload(m, p.id, "main", "", cmiJPEG(t, 100, 700, 5), t04Key("pm-tall-main")); w.Code != 200 {
		t.Fatalf("tall main: %d %s", w.Code, w.Body.String())
	}
	if w := m.remove(p.id, l.ids("detail")[0], t04Key("pm-del2")); w.Code != 200 {
		t.Fatalf("delete: %d", w.Code)
	}
	if w := pmUpload(m, p.id, "detail", "", cmiJPEG(t, 100, 700, 6), t04Key("pm-tall-detail")); w.Code != 422 {
		t.Fatalf("detail taller than 6x: %d %s, want 422", w.Code, w.Body.String())
	}
	if w := pmUpload(m, p.id, "detail", "", cmiJPEG(t, 750, 4000, 7), t04Key("pm-750")); w.Code != 200 {
		t.Fatalf("750x4000 detail: %d %s", w.Code, w.Body.String())
	}
}

func TestPMv2OptionImages(t *testing.T) {
	h, m, _ := cmiEnv(t)
	p := pmNewProduct(t, h)
	// role=sku needs a real value of the effective axis (default = first axis, Color)
	if w := pmUpload(m, p.id, "sku", "", cmiJPEG(t, 40, 30, 1), t04Key("pm-nov")); w.Code != 422 {
		t.Fatalf("sku without option_value: %d, want 422", w.Code)
	}
	if w := pmUpload(m, p.id, "sku", "Purple", cmiJPEG(t, 40, 30, 1), t04Key("pm-badv")); w.Code != 422 {
		t.Fatalf("sku with a value off the axis: %d, want 422", w.Code)
	}
	if w := pmUpload(m, p.id, "sku", "S", cmiJPEG(t, 40, 30, 1), t04Key("pm-size")); w.Code != 422 {
		t.Fatalf("sku with a value of the other axis: %d, want 422", w.Code)
	}
	red := pmMust(m, p.id, "sku", "Red", 11)
	blue := pmMust(m, p.id, "sku", "Blue", 12)
	if w := pmUpload(m, p.id, "sku", "Red", cmiJPEG(t, 40, 30, 13), t04Key("pm-2nd")); w.Code != 409 {
		t.Fatalf("second image for Red: %d %s, want 409", w.Code, w.Body.String())
	}
	l := pmGet(m, p.id)
	if l.ImageAxis == nil || *l.ImageAxis != "Color" || len(l.OptionImages) != 2 || len(l.ids("sku")) != 2 {
		t.Fatalf("list axis=%v links=%+v skus=%d", l.ImageAxis, l.OptionImages, len(l.ids("sku")))
	}
	if len(l.ids("main")) != 0 {
		t.Fatal("sku images leaked into main")
	}
	// delete of a sku image removes its link (cascade)
	spare := pmMust(m, p.id, "sku", "Green", 14)
	if w := pmPost(m, p.id+"/images/"+spare.ID+"/delete", struct{}{}); w.Code != 200 {
		t.Fatalf("delete spare: %d", w.Code)
	}
	if got := pmGet(m, p.id).OptionImages; len(got) != 2 {
		t.Fatalf("links after deleting the Green image: %+v", got)
	}
	// a sku image never moves to main/detail
	if w := pmPost(m, p.id+"/images/"+red.ID+"/move", map[string]string{"role": "main"}); w.Code != 422 {
		t.Fatalf("move sku->main: %d, want 422", w.Code)
	}
	// link endpoint re-points an existing sku image to a FREE value; a taken value is 409; a non-sku image is 422
	if w := pmPost(m, p.id+"/option-images", map[string]string{"option_value": "Blue", "image_id": red.ID}); w.Code != 409 {
		t.Fatalf("link onto a taken value: %d %s, want 409", w.Code, w.Body.String())
	}
	if w := pmPost(m, p.id+"/option-images", map[string]string{"option_value": "Green", "image_id": red.ID}); w.Code != 200 {
		t.Fatalf("re-point Red image to Green: %d %s", w.Code, w.Body.String())
	}
	got := map[string]string{}
	for _, o := range pmGet(m, p.id).OptionImages {
		got[o.OptionValue] = o.ImageID
	}
	if len(got) != 2 || got["Green"] != red.ID || got["Blue"] != blue.ID || got["Red"] != "" {
		t.Fatalf("links after re-point: %v", got)
	}
	mainImg := pmMust(m, p.id, "main", "", 15)
	if w := pmPost(m, p.id+"/option-images", map[string]string{"option_value": "Red", "image_id": mainImg.ID}); w.Code != 422 {
		t.Fatalf("link a main image: %d %s, want 422", w.Code, w.Body.String())
	}
	if w := pmPost(m, p.id+"/option-images", map[string]string{"option_value": "Purple", "image_id": red.ID}); w.Code != 422 {
		t.Fatalf("link to a value off the axis: %d, want 422", w.Code)
	}
	// switching the image axis
	if w := pmPost(m, p.id+"/image-axis", map[string]any{"axis": "Material"}); w.Code != 422 {
		t.Fatalf("unknown axis: %d, want 422", w.Code)
	}
	if w := pmPost(m, p.id+"/image-axis", map[string]any{"axis": "Size"}); w.Code != 200 {
		t.Fatalf("set axis: %d %s", w.Code, w.Body.String())
	}
	l = pmGet(m, p.id)
	if l.ImageAxis == nil || *l.ImageAxis != "Size" || len(l.OptionImages) != 0 {
		t.Fatalf("after switching to Size: axis=%v links=%+v (Color links must be dormant, not shown)", l.ImageAxis, l.OptionImages)
	}
	if w := pmUpload(m, p.id, "sku", "S", cmiJPEG(t, 40, 30, 16), t04Key("pm-s")); w.Code != 200 {
		t.Fatalf("sku on the new axis: %d %s", w.Code, w.Body.String())
	}
	if w := pmPost(m, p.id+"/image-axis", map[string]any{"axis": nil}); w.Code != 200 {
		t.Fatalf("reset axis: %d", w.Code)
	}
	if l = pmGet(m, p.id); l.ImageAxis == nil || *l.ImageAxis != "Color" || len(l.OptionImages) != 2 {
		t.Fatalf("after reset: axis=%v links=%+v (the dormant Color links return)", l.ImageAxis, l.OptionImages)
	}
}

// Cross-store/tenant: another store's merchant cannot link, move or read this product's images.
func TestPMv2CrossStoreRefused(t *testing.T) {
	h, m, _ := cmiEnv(t)
	p := pmNewProduct(t, h)
	img := pmMust(m, p.id, "sku", "Red", 21)
	foreign := cmiMerchant{t: t, h: m.h, token: h.f.tokens["a2"], store: h.f.storeA2}
	other := t04CreateStock(t, h.f, h.f.tokens["a2"], h.f.storeA2, 3).product.ID
	cmiCleanup(t, h.f, other)
	body, _ := json.Marshal(map[string]string{"option_value": "Red", "image_id": img.ID})
	hdr := map[string]string{"Content-Type": "application/json", "Idempotency-Key": t04Key("pm-x")}
	if w := foreign.raw("POST", "/v1/admin/stores/"+foreign.store+"/products/"+other+"/option-images", hdr, body); w.Code != 404 && w.Code != 422 {
		t.Fatalf("link of a foreign image into another store: %d %s, want 404/422", w.Code, w.Body.String())
	}
	if w := foreign.raw("POST", "/v1/admin/stores/"+foreign.store+"/products/"+p.id+"/option-images", hdr, body); w.Code != 404 {
		t.Fatalf("link on a foreign product: %d, want 404", w.Code)
	}
}

// ---- buyer shapes -----------------------------------------------------------------------------------------------------------

type pmDetail struct {
	Images       []struct{ ID string } `json:"images"`
	DetailImages []struct{ ID string } `json:"detail_images"`
	ImageAxis    *string               `json:"image_axis"`
	OptionImages []struct {
		Value   string `json:"value"`
		ImageID string `json:"image_id"`
	} `json:"option_images"`
	Variants []struct {
		SKUID        string   `json:"sku_id"`
		OptionValues []string `json:"option_values"`
		ImageID      *string  `json:"image_id"`
	} `json:"variants"`
}

func TestPMv2BuyerShapes(t *testing.T) {
	h, m, _ := cmiEnv(t)
	p := pmNewProduct(t, h)
	var mainIDs, detailIDs []string
	for i := 0; i < 4; i++ {
		mainIDs = append(mainIDs, pmMust(m, p.id, "main", "", uint8(30+i)).ID)
	}
	for i := 0; i < 5; i++ {
		detailIDs = append(detailIDs, pmMust(m, p.id, "detail", "", uint8(50+i)).ID)
	}
	red := pmMust(m, p.id, "sku", "Red", 70)
	blue := pmMust(m, p.id, "sku", "Blue", 71)

	d := bhRead[pmDetail](t, h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+p.slug, "", "", nil, nil), 200)
	ids := func(in []struct{ ID string }) (out []string) {
		for _, i := range in {
			out = append(out, i.ID)
		}
		return out
	}
	if fmt.Sprint(ids(d.Images)) != fmt.Sprint(mainIDs) {
		t.Fatalf("images = %v, want the 4 main images in order %v", ids(d.Images), mainIDs)
	}
	if fmt.Sprint(ids(d.DetailImages)) != fmt.Sprint(detailIDs) {
		t.Fatalf("detail_images = %v, want %v", ids(d.DetailImages), detailIDs)
	}
	if d.ImageAxis == nil || *d.ImageAxis != "Color" || len(d.OptionImages) != 2 {
		t.Fatalf("image_axis=%v option_images=%+v", d.ImageAxis, d.OptionImages)
	}
	want := map[string]string{"Red": red.ID, "Blue": blue.ID}
	for _, o := range d.OptionImages {
		if want[o.Value] != o.ImageID {
			t.Fatalf("option image %+v", o)
		}
	}
	for _, v := range d.Variants {
		var expect *string
		if id, ok := want[v.OptionValues[0]]; ok {
			expect = &id
		}
		if (expect == nil) != (v.ImageID == nil) || (expect != nil && *expect != *v.ImageID) {
			t.Fatalf("variant %v image_id=%v, want %v", v.OptionValues, v.ImageID, expect)
		}
	}
	// the image bytes of every role are served by the public media route (same product, active)
	for _, id := range []string{mainIDs[0], detailIDs[0], red.ID} {
		if r := cmiGetMedia(t, h, h.origin, p.id, id, nil); r.status != 200 {
			t.Fatalf("media %s: %d", id, r.status)
		}
	}
	// v1 catalog: images = main only; image_id = variant image else cover
	cat := pmCatalog(t, h)
	for key, sku := range p.skus {
		got := cat[sku]
		var expect string
		switch {
		case strings.HasPrefix(key, "Red/"):
			expect = red.ID
		case strings.HasPrefix(key, "Blue/"):
			expect = blue.ID
		default:
			expect = mainIDs[0]
		}
		if got.ImageID == nil || *got.ImageID != expect {
			t.Fatalf("catalog image_id of %s = %v, want %s", key, got.ImageID, expect)
		}
		if fmt.Sprint(got.imageIDs()) != fmt.Sprint(mainIDs) {
			t.Fatalf("catalog images of %s = %v, want main only %v", key, got.imageIDs(), mainIDs)
		}
	}
	// product list card keeps the cover only
	list := bhRead[struct {
		Products []struct {
			ID    string  `json:"id"`
			Cover *string `json:"cover_image_id"`
		} `json:"products"`
	}](t, h.request(t, "GET", "/v1/buyer/catalog/v2/products?limit=48", "", "", nil, nil), 200)
	for _, c := range list.Products {
		if c.ID == p.id && (c.Cover == nil || *c.Cover != mainIDs[0]) {
			t.Fatalf("card cover = %v, want main[0]", c.Cover)
		}
	}
	// a product with only detail images has no cover
	only := pmNewProduct(t, h)
	pmMust(m, only.id, "detail", "", 80)
	d2 := bhRead[pmDetail](t, h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+only.slug, "", "", nil, nil), 200)
	if len(d2.Images) != 0 || len(d2.DetailImages) != 1 || d2.OptionImages == nil {
		t.Fatalf("detail-only product: images=%d detail=%d option_images=%v (must be [] not null)", len(d2.Images), len(d2.DetailImages), d2.OptionImages)
	}
	// an unpublished/draft product exposes nothing (same 404 as before)
	mustExec(t, h.f.owner, `UPDATE catalog.products SET status='draft' WHERE id=$1`, only.id)
	if r := h.request(t, "GET", "/v1/buyer/catalog/v2/products/"+only.slug, "", "", nil, nil); r.status != 404 {
		t.Fatalf("draft detail: %d, want 404", r.status)
	}
}

type pmCatalogItem struct {
	SKUID   string  `json:"sku_id"`
	ImageID *string `json:"image_id"`
	Images  []struct {
		ID string `json:"id"`
	} `json:"images"`
}

func (c pmCatalogItem) imageIDs() (out []string) {
	for _, i := range c.Images {
		out = append(out, i.ID)
	}
	return out
}

func pmCatalog(t *testing.T, h bhHarness) map[string]pmCatalogItem {
	t.Helper()
	out := map[string]pmCatalogItem{}
	cursor := ""
	for page := 0; page < 30; page++ {
		q := "limit=100"
		if cursor != "" {
			q += "&cursor=" + cursor
		}
		r := h.request(t, "GET", "/v1/buyer/catalog?"+q, h.cap.Token, "", nil, nil)
		if r.status != 200 {
			t.Fatalf("buyer catalog: %d %s", r.status, r.body)
		}
		var raw struct {
			Items      []pmCatalogItem `json:"items"`
			NextCursor string          `json:"next_cursor"`
		}
		if err := json.Unmarshal(r.body, &raw); err != nil {
			t.Fatal(err)
		}
		for _, i := range raw.Items {
			out[i.SKUID] = i
		}
		if raw.NextCursor == "" {
			return out
		}
		cursor = raw.NextCursor
	}
	t.Fatal("catalog did not end")
	return nil
}

// ---- Meta feed -----------------------------------------------------------------------------------------------------------

func TestPMv2FeedLinks(t *testing.T) {
	h, m, _ := cmiEnv(t)
	p := pmNewProduct(t, h)
	var mainIDs []string
	for i := 0; i < 4; i++ {
		mainIDs = append(mainIDs, pmMust(m, p.id, "main", "", uint8(30+i)).ID)
	}
	detailID := pmMust(m, p.id, "detail", "", 60).ID
	red := pmMust(m, p.id, "sku", "Red", 70)

	r := httptest.NewRequest("GET", "/v1/buyer/feeds/meta.csv", nil)
	r.Header.Set("X-Commerce-Storefront-Origin", h.origin)
	w := httptest.NewRecorder()
	attribution.FeedHandler(h.a.runtime).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("feed: %d %s", w.Code, w.Body.String())
	}
	rows, err := csv.NewReader(w.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rows[0], ",") != "id,title,description,availability,condition,price,link,image_link,additional_image_link,brand" {
		t.Fatalf("feed header %v", rows[0])
	}
	col := map[string]int{}
	for i, n := range rows[0] {
		col[n] = i
	}
	link := func(id string) string { return h.origin + "/media/p/" + p.id + "/" + id }
	byID := map[string][]string{}
	for _, row := range rows[1:] {
		byID[row[col["id"]]] = row
	}
	for key, sku := range p.skus {
		row, ok := byID[sku]
		if !ok {
			t.Fatalf("feed has no row for %s", key)
		}
		var wantImage, wantExtra string
		if strings.HasPrefix(key, "Red/") { // variant image first, all main images additional
			wantImage = link(red.ID)
			wantExtra = strings.Join([]string{link(mainIDs[0]), link(mainIDs[1]), link(mainIDs[2]), link(mainIDs[3])}, ",")
		} else {
			wantImage = link(mainIDs[0])
			wantExtra = strings.Join([]string{link(mainIDs[1]), link(mainIDs[2]), link(mainIDs[3])}, ",")
		}
		if row[col["image_link"]] != wantImage || row[col["additional_image_link"]] != wantExtra {
			t.Fatalf("%s: image_link=%q additional=%q\nwant %q / %q", key, row[col["image_link"]], row[col["additional_image_link"]], wantImage, wantExtra)
		}
	}
	if strings.Contains(w.Body.String(), detailID) { // detail images never reach the feed
		t.Fatal("a detail image leaked into the feed")
	}
}

// ---- ACL pins ---------------------------------------------------------------------------------------------------------------

func TestPMv2ACLPins(t *testing.T) {
	h, _, _ := cmiEnv(t)
	f := h.f
	ctx := context.Background()
	// the new table: FORCE RLS, buyer/PUBLIC no privilege, runtime SELECT/INSERT/DELETE only, definer owner SELECT only
	var enabled, forced bool
	if err := f.owner.QueryRow(ctx, `SELECT relrowsecurity,relforcerowsecurity FROM pg_class WHERE oid='catalog.product_option_images'::regclass`).Scan(&enabled, &forced); err != nil || !enabled || !forced {
		t.Fatalf("catalog.product_option_images must ENABLE and FORCE RLS: %v %v %v", enabled, forced, err)
	}
	priv := func(role, priv string) bool {
		var can bool
		if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,'catalog.product_option_images',$2)`, role, priv).Scan(&can); err != nil {
			t.Fatal(err)
		}
		return can
	}
	for role, want := range map[string]map[string]bool{
		"commerce_runtime":       {"SELECT": true, "INSERT": true, "DELETE": true, "UPDATE": false, "TRUNCATE": false},
		"commerce_catalog_media": {"SELECT": true, "INSERT": false, "DELETE": false, "UPDATE": false},
		"commerce_buyer_runtime": {"SELECT": false, "INSERT": false, "DELETE": false, "UPDATE": false},
		"public":                 {"SELECT": false, "INSERT": false, "DELETE": false, "UPDATE": false},
	} {
		for p, w := range want {
			if got := priv(role, p); got != w {
				t.Errorf("%s %s on product_option_images = %v, want %v", role, p, got, w)
			}
		}
	}
	// product_images: commerce_runtime may update exactly role, position, version
	for _, c := range []string{"role", "position", "version", "bytes", "sha256", "content_type", "product_id", "tenant_id"} {
		var can bool
		_ = f.owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_runtime','catalog.product_images',$1,'UPDATE')`, c).Scan(&can)
		if want := c == "role" || c == "position" || c == "version"; can != want {
			t.Errorf("commerce_runtime UPDATE(%s) on product_images = %v, want %v", c, can, want)
		}
	}
	var axis bool
	_ = f.owner.QueryRow(ctx, `SELECT has_column_privilege('commerce_runtime','catalog.products','image_axis','UPDATE') AND has_column_privilege('commerce_catalog_media','catalog.products','image_axis','SELECT') AND NOT has_column_privilege('commerce_buyer_runtime','catalog.products','image_axis','SELECT')`).Scan(&axis)
	if !axis {
		t.Error("products.image_axis grants: runtime UPDATE + definer SELECT only")
	}
	// every new SECURITY DEFINER: owned by commerce_catalog_media, search_path pinned, EXECUTE only buyer runtime (+ the definer owner)
	for _, sig := range []string{"catalog.buyer_sku_images(uuid[])", "catalog.buyer_feed_variant_images(text)"} {
		var secdef bool
		var owner string
		var cfg []string
		if err := f.owner.QueryRow(ctx, `SELECT p.prosecdef,r.rolname,coalesce(p.proconfig,'{}') FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, sig).Scan(&secdef, &owner, &cfg); err != nil {
			t.Fatalf("%s: %v", sig, err)
		}
		if !secdef || owner != "commerce_catalog_media" || !strings.Contains(strings.Join(cfg, ","), "search_path=pg_catalog") {
			t.Errorf("%s: secdef=%v owner=%s config=%v", sig, secdef, owner, cfg)
		}
		for _, role := range []string{"public", "commerce_runtime", "commerce_buyer_runtime"} {
			var can bool
			if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, role, sig).Scan(&can); err != nil {
				t.Fatal(err)
			}
			if want := role == "commerce_buyer_runtime"; can != want {
				t.Errorf("%s EXECUTE for %s = %v, want %v", sig, role, can, want)
			}
		}
	}
	// the invoker helper is not executable by PUBLIC or the runtimes
	for _, role := range []string{"public", "commerce_runtime", "commerce_buyer_runtime"} {
		var can bool
		if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,'catalog.sku_option_image(uuid,uuid,uuid)','EXECUTE')`, role).Scan(&can); err != nil || can {
			t.Errorf("catalog.sku_option_image EXECUTE for %s = %v %v, want false", role, can, err)
		}
	}
	// the buyer runtime still has no direct table access
	if _, err := h.a.runtime.Exec(ctx, `SELECT 1 FROM catalog.product_option_images LIMIT 1`); err == nil || cmiErrCode(t, err) != "42501" {
		t.Fatalf("buyer runtime SELECT on product_option_images: %v, want 42501", err)
	}
}
