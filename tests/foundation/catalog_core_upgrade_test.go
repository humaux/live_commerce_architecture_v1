package foundation_test

// catalog_core_upgrade_test.go: CC01 of unit catalog-core (contracts/storefront-v2.md section A acceptance): migration 0086
// on a POPULATED database that is at the pre-0086 schema. A throw-away PG cluster (its own labelled container, like the
// claims-retention populated-upgrade gate) takes every migration except 0086 and later, is filled with a merchant's
// catalog, then 0086 is applied on top. The contract: legacy products keep their status AND stay buyer-visible, get a
// valid unique slug (the id prefix), legacy SKUs get empty option values and no compare-at, FORCE RLS is back on
// catalog.products, nothing else in the populated rows changes, and a second Apply changes nothing.
//
// Evidence label: REAL_PG (disposable PostgreSQL 18, real migrations.Apply, owner-seeded synthetic rows).

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/migrations"
)

func TestCatalogCoreCC01PopulatedUpgrade(t *testing.T) {
	ctx := context.Background()
	owner := mciStartPG(t) // skips (NOT_RUN) without LC_TEST_DATABASE_ALLOWED=1

	// Hold back 0086 and every later migration: the database is exactly what a pre-catalog-v2 deploy looks like.
	files, err := filepath.Glob("../../migrations/[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil || len(files) < 60 {
		t.Fatalf("migration files: %d %v", len(files), err)
	}
	var held []string
	mustExec(t, owner, `CREATE TABLE public.lc_schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`)
	sums := map[string]string{}
	for _, f := range files {
		version := filepath.Base(f)
		if version < "0086" {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		sums[version] = fmt.Sprintf("%x", sha256.Sum256(body))
		held = append(held, version)
		mustExec(t, owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, version, sums[version])
	}
	if !slices.Contains(held, "0086_catalog_v2.sql") {
		t.Fatalf("0086 not found among the held-back migrations: %v", held)
	}
	waPrecreateRoles(t, owner) // 0096 (worker authorities) is held back above; the current Apply's River grants name its roles
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("apply every migration before 0086: %v", err)
	}
	// Preconditions: this really is the pre-0086 schema.
	if n := countRows(t, owner, `SELECT count(*) FROM information_schema.columns WHERE table_schema='catalog' AND table_name='products' AND column_name IN ('slug','options','seo_title')`); n != 0 {
		t.Fatalf("pre-0086 database already has %d catalog v2 product columns", n)
	}
	if n := countRows(t, owner, `SELECT count(*) FROM information_schema.tables WHERE table_schema='catalog' AND table_name='collections'`); n != 0 {
		t.Fatal("pre-0086 database already has catalog.collections")
	}

	// ---- populate a merchant's catalog (owner-seeded, synthetic) ----
	tenant, store, otherTenant, otherStore := randomUUID(), randomUUID(), randomUUID(), randomUUID()
	mustExec(t, owner, `INSERT INTO control.tenants(id,name) VALUES ($1,'cc01-a'),($2,'cc01-b')`, tenant, otherTenant)
	mustExec(t, owner, `INSERT INTO control.stores(tenant_id,id,name,currency) VALUES ($1,$2,'CC01 Shop','TWD'),($3,$4,'CC01 Other','TWD')`, tenant, store, otherTenant, otherStore)
	const origin = "https://cc01-upgrade.example"
	mustExec(t, owner, `INSERT INTO control.storefront_publications(tenant_id,store_id,published) VALUES($1,$2,true)`, tenant, store)
	mustExec(t, owner, `INSERT INTO control.storefront_domains(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
		VALUES($1,$2,$3,'ACTIVE',clock_timestamp()-interval '1 hour',clock_timestamp()-interval '1 hour',clock_timestamp()+interval '1 day','SYNTHETIC cc01')`, tenant, store, origin)
	wh1, wh2 := randomUUID(), randomUUID()
	mustExec(t, owner, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name) VALUES ($1,$2,$3,'w1'),($1,$2,$4,'w2')`, tenant, store, wh1, wh2)
	product := func(tn, st, name, status string) string {
		id := randomUUID()
		mustExec(t, owner, `INSERT INTO catalog.products(tenant_id,store_id,id,name,description,status) VALUES($1,$2,$3,$4,$5,$6)`, tn, st, id, name, "legacy "+name, status)
		return id
	}
	sku := func(tn, st, pid, code, status string, price int64) string {
		id := randomUUID()
		mustExec(t, owner, `INSERT INTO catalog.skus(tenant_id,store_id,id,product_id,code,status,currency,price_minor) VALUES($1,$2,$3,$4,$5,$6,'TWD',$7)`, tn, st, id, pid, code, status, price)
		return id
	}
	stock := func(wh, skuID string, qty int64) {
		mustExec(t, owner, `INSERT INTO inventory.balances(tenant_id,store_id,warehouse_id,sku_id,on_hand) VALUES($1,$2,$3,$4,$5)`, tenant, store, wh, skuID, qty)
	}
	live1 := product(tenant, store, "Legacy Tee", "active")
	live1a, live1b := sku(tenant, store, live1, "LEG-TEE-S", "active", 1200), sku(tenant, store, live1, "LEG-TEE-M", "active", 1500)
	stock(wh1, live1a, 3)
	stock(wh2, live1a, 4) // 3 + 4 over two warehouses
	live2 := product(tenant, store, "Legacy Sold Out", "active")
	live2a := sku(tenant, store, live2, "LEG-SOLD", "active", 800)
	stock(wh1, live2a, 0)
	live3 := product(tenant, store, "Legacy Mixed", "active")
	live3a := sku(tenant, store, live3, "LEG-MIX-OLD", "archived", 100)
	live3b := sku(tenant, store, live3, "LEG-MIX-NEW", "active", 900)
	gone := product(tenant, store, "Legacy Retired", "archived")
	goneSKU := sku(tenant, store, gone, "LEG-GONE", "active", 700)
	foreign := product(otherTenant, otherStore, "Legacy Foreign", "active")
	foreignSKU := sku(otherTenant, otherStore, foreign, "LEG-FOREIGN", "active", 300)
	_, _, _ = live1b, live3a, goneSKU
	_ = foreignSKU
	productIDs := []string{live1, live2, live3, gone, foreign}

	// What must NOT change: every populated column that existed before 0086 (explicit JSON minus the columns 0086 adds).
	digest := func(table string, minus ...string) string {
		return crExplicitDigest(t, owner, table, minus...)
	}
	productsBefore := digest("catalog.products")
	skusBefore := digest("catalog.skus")
	balancesBefore := digest("inventory.balances")
	statusBefore := map[string]string{}
	rows, err := owner.Query(ctx, `SELECT id::text,status FROM catalog.products`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, st string
		_ = rows.Scan(&id, &st)
		statusBefore[id] = st
	}
	rows.Close()

	// ---- the upgrade: release 0086 (and the later ones) ----
	for _, version := range held {
		mustExec(t, owner, `DELETE FROM public.lc_schema_migrations WHERE version=$1`, version)
	}
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("0086 on the populated database: %v", err)
	}
	ledger := crExplicitDigest(t, owner, "public.lc_schema_migrations", "applied_at")
	if err := migrations.Apply(ctx, owner); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if crExplicitDigest(t, owner, "public.lc_schema_migrations", "applied_at") != ledger {
		t.Error("a second Apply changed the migration ledger")
	}
	if n := countRows(t, owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0086_catalog_v2.sql' AND checksum=$1`, sums["0086_catalog_v2.sql"]); n != 1 {
		t.Errorf("0086 ledger rows with the file checksum: %d", n)
	}

	// 1. status kept, slug valid + unique + the id prefix; the other populated columns untouched
	slugShape := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	uuidShape := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	seen := map[string]string{}
	rows, err = owner.Query(ctx, `SELECT id::text,status,slug,seo_title,seo_description,options::text,store_id::text FROM catalog.products`)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for rows.Next() {
		var id, status, slug, seoT, seoD, options, st string
		if err := rows.Scan(&id, &status, &slug, &seoT, &seoD, &options, &st); err != nil {
			t.Fatal(err)
		}
		n++
		if status != statusBefore[id] {
			t.Errorf("product %s changed status %s -> %s: a legacy product must keep its status", id, statusBefore[id], status)
		}
		if !strings.HasPrefix(strings.ReplaceAll(id, "-", ""), slug) || len(slug) < 8 || !slugShape.MatchString(slug) || uuidShape.MatchString(slug) || len(slug) > 80 {
			t.Errorf("product %s: legacy slug %q must be a valid slug that is a prefix of the id", id, slug)
		}
		if k := st + "/" + slug; seen[k] != "" {
			t.Errorf("slug %q is not unique in store %s (%s and %s)", slug, st, seen[k], id)
		} else {
			seen[k] = id
		}
		if seoT != "" || seoD != "" || options != "[]" {
			t.Errorf("product %s: seo/options must default to empty: %q %q %s", id, seoT, seoD, options)
		}
	}
	rows.Close()
	if n != len(productIDs) {
		t.Fatalf("products after the upgrade: %d want %d", n, len(productIDs))
	}
	newCols := []string{"slug", "seo_title", "seo_description", "options"}
	if got := digest("catalog.products", newCols...); got != productsBefore {
		t.Error("0086 changed populated catalog.products columns it does not own")
	}
	if got := digest("catalog.skus", "option_values", "compare_at_minor"); got != skusBefore {
		t.Error("0086 changed populated catalog.skus columns it does not own")
	}
	if digest("inventory.balances") != balancesBefore {
		t.Error("0086 changed inventory.balances")
	}
	// 2. legacy SKUs: no axes, no compare-at
	if bad := countRows(t, owner, `SELECT count(*) FROM catalog.skus WHERE option_values <> '{}' OR compare_at_minor IS NOT NULL`); bad != 0 {
		t.Errorf("%d legacy SKUs have option values or a compare-at price", bad)
	}
	// 3. FORCE RLS is back on the table the backfill touched (and ordinary RLS stays on)
	for _, table := range []string{"catalog.products", "catalog.skus"} {
		var rls, force bool
		if err := owner.QueryRow(ctx, `SELECT relrowsecurity,relforcerowsecurity FROM pg_class WHERE oid=$1::regclass`, table).Scan(&rls, &force); err != nil || !rls || !force {
			t.Errorf("%s must have ROW LEVEL SECURITY enabled and FORCED after the upgrade: rls=%v force=%v %v", table, rls, force, err)
		}
	}
	// 4. a draft is now a legal status; garbage still is not
	mustExec(t, owner, `UPDATE catalog.products SET status='draft' WHERE id=$1`, live2)
	mustExec(t, owner, `UPDATE catalog.products SET status='active' WHERE id=$1`, live2)
	if _, err := owner.Exec(ctx, `UPDATE catalog.products SET status='bogus' WHERE id=$1`, live2); err == nil {
		t.Error("status 'bogus' must violate the CHECK")
	}

	// 5. buyer-visible: the active products with an active SKU are served by the v2 reads; archived / foreign ones are not
	var raw string
	if err := owner.QueryRow(ctx, `SELECT catalog.buyer_v2_products($1,'','','',NULL,NULL,0,48)::text`, origin).Scan(&raw); err != nil {
		t.Fatalf("buyer v2 list on the upgraded database: %v", err)
	}
	for _, want := range []string{live1, live2, live3} {
		if !strings.Contains(raw, want) {
			t.Errorf("existing active product %s is not buyer-visible after the upgrade: %s", want, raw)
		}
	}
	for _, hidden := range []string{gone, foreign} {
		if strings.Contains(raw, hidden) {
			t.Errorf("product %s must not be buyer-visible: %s", hidden, raw)
		}
	}
	var detail string
	slug := strings.ReplaceAll(live1, "-", "")[:12]
	for _, key := range []string{live1, slug} {
		if err := owner.QueryRow(ctx, `SELECT catalog.buyer_v2_product($1,$2)::text`, origin, key).Scan(&detail); err != nil || !strings.Contains(detail, `"in"`) || !strings.Contains(detail, live1a) {
			t.Errorf("detail of the legacy product by %q after the upgrade: %v %s", key, err, detail)
		}
	}
	if err := owner.QueryRow(ctx, `SELECT catalog.buyer_v2_product($1,$2)::text`, origin, gone).Scan(&detail); err == nil {
		t.Errorf("an archived legacy product must be unreachable: %s", detail)
	}
	// the pre-existing buyer reads (0007 catalog / cart, 0080 feed) still serve exactly the legacy active products
	assertIDs := func(label, query string, want []string, args ...any) {
		t.Helper()
		got := []string{}
		r, err := owner.Query(ctx, query, args...)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		for r.Next() {
			var id string
			_ = r.Scan(&id)
			got = append(got, id)
		}
		r.Close()
		slices.Sort(got)
		w := slices.Clone(want)
		slices.Sort(w)
		if !slices.Equal(got, w) {
			t.Errorf("%s: got %v want %v", label, got, w)
		}
	}
	assertIDs("Meta feed (0080) after the upgrade", `SELECT id FROM ads.feed_rows($1)`, []string{live1a, live1b, live2a, live3b}, origin)
	// the buyer runtime role (0007: SELECT on catalog.products under the scope GUCs) still sees exactly the active products
	tx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_buyer_runtime`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true)`, tenant, store); err != nil {
		t.Fatal(err)
	}
	var got []string
	r, err := tx.Query(ctx, `SELECT id::text FROM catalog.products WHERE status='active'`)
	if err != nil {
		t.Fatalf("buyer role read of catalog.products after the upgrade: %v", err)
	}
	for r.Next() {
		var id string
		_ = r.Scan(&id)
		got = append(got, id)
	}
	r.Close()
	slices.Sort(got)
	if want := []string{live1, live2, live3}; !slices.Equal(got, func() []string { w := slices.Clone(want); slices.Sort(w); return w }()) {
		t.Errorf("buyer role sees %v after the upgrade, want the three legacy active products", got)
	}
}

var _ *pgxpool.Pool
