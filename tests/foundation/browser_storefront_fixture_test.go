package foundation_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/platform"
	"livecommerce/internal/storefrontadmin"
)

// Shared fixture step for the buyer browser gates after the storefront shell replaced the single-product purchase page (unit
// storefront-integration). The shell sells a product through catalog-v2 variants: a product with more than one SKU needs option axes, or
// only its first variant is buyable. Old fixtures create axis-less SKUs with raw SQL, so this gives the product ONE axis named "SKU" whose
// values are the SKU codes, and each active SKU the matching option value. The chip a buyer clicks is then named exactly like the SKU code,
// which is what the old radio-per-SKU page showed, so a gate keeps selecting a SKU by its code.
// Owner pool only (fixture setup, never a runtime path); call it AFTER the fixture finished renaming SKU codes.
func sfiAxisBySKUCode(t *testing.T, owner *pgxpool.Pool, productID string) {
	t.Helper()
	mustExec(t, owner, `UPDATE catalog.skus SET option_values=ARRAY[code] WHERE product_id=$1 AND status='active'`, productID)
	mustExec(t, owner, `UPDATE catalog.products SET options=jsonb_build_array(jsonb_build_object('name','SKU','values',
		(SELECT jsonb_agg(code ORDER BY created_at,id) FROM catalog.skus WHERE product_id=$1 AND status='active'))) WHERE id=$1`, productID)
}

// sfiPublishViaDefiners makes `origin` a published, ACTIVE storefront of (tenant, store) through the production writers of migration 0081
// instead of owner INSERTs (bhPublish): the merchant publishes with control.set_storefront_published (internal/storefrontadmin.SetPublished,
// runtime role, integration:manage on the merchant's own session) and the platform operator binds the domain with
// control.operator_bind_domain (storefrontadmin.BindDomain on the registrar-shaped login). The only owner-pool writes are the disclosed
// fixture rows those definers need: a merchant session for `principal` and the identity.initial_stores row the operator audit attributes to.
// The domain evidence is a reference string nobody verifies here (MOCK, synthetic TLS edge). Cleanup removes exactly what was written.
func sfiPublishViaDefiners(t *testing.T, f *testFixture, principal, origin string) (merchantToken string) {
	t.Helper()
	ctx := context.Background()
	token := randomToken()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = insertSession(ctx, tx, token, principal, "merchant", time.Now().UTC().Add(time.Hour), nil); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		SELECT $1,$2,$3,p FROM unnest(ARRAY['integration:read','integration:manage']) p ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, principal)
	var warehouse string
	if err = f.owner.QueryRow(ctx, `SELECT id::text FROM inventory.warehouses WHERE tenant_id=$1 AND store_id=$2 LIMIT 1`, f.tenantA, f.storeA1).Scan(&warehouse); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `INSERT INTO identity.initial_stores(principal_id,idempotency_key,request_hash,tenant_id,store_id,warehouse_id)
		VALUES($1,$2,decode(repeat('ef',32),'hex'),$3,$4,$5)`, principal, "sfi-"+randomUUID()[:16], f.tenantA, f.storeA1, warehouse)
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM control.storefront_domains WHERE origin=$1`,
			`DELETE FROM control.storefront_publications WHERE tenant_id=$2 AND store_id=$3`,
			`DELETE FROM ops.audit_events WHERE tenant_id=$2 AND store_id=$3 AND action LIKE ANY(ARRAY['merchant.storefront%','operator.domain%'])`,
			`DELETE FROM identity.initial_stores WHERE tenant_id=$2 AND store_id=$3`} {
			_, _ = f.owner.Exec(context.Background(), q, origin, f.tenantA, f.storeA1)
		}
	})
	if err = platform.WithScope(ctx, f.runtime, token, f.storeA1, "integration:manage", func(tx pgx.Tx, scope platform.Scope) error {
		_, e := storefrontadmin.SetPublished(ctx, tx, scope, token, true, 0)
		return e
	}); err != nil {
		t.Fatalf("merchant publish through control.set_storefront_published: %v", err)
	}
	registrar, err := pgxpool.New(ctx, miRole(t, f, "commerce_storefront_registrar"))
	if err != nil {
		t.Fatal(err)
	}
	defer registrar.Close()
	if _, err = storefrontadmin.BindDomain(ctx, registrar, f.storeA1, origin, "SYNTHETIC storefront-integration gate only", time.Now().Add(time.Hour), time.Now()); err != nil {
		t.Fatalf("operator bind through control.operator_bind_domain: %v", err)
	}
	return token
}

// sfiSetPublished flips the merchant publication through the same definer with a compare-and-set on the version it just read
// (storefrontadmin.Read -> SetPublished), as the Settings card does. The domain binding is untouched.
func sfiSetPublished(f *testFixture, token string, published bool) error {
	ctx := context.Background()
	var current storefrontadmin.State
	if err := platform.WithScope(ctx, f.runtime, token, f.storeA1, "integration:read", func(tx pgx.Tx, scope platform.Scope) error {
		var e error
		current, e = storefrontadmin.Read(ctx, tx, scope, token)
		return e
	}); err != nil {
		return err
	}
	return platform.WithScope(ctx, f.runtime, token, f.storeA1, "integration:manage", func(tx pgx.Tx, scope platform.Scope) error {
		_, e := storefrontadmin.SetPublished(ctx, tx, scope, token, published, current.Version)
		return e
	})
}
