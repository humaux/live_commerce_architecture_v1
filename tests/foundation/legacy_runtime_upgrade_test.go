package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"livecommerce/internal/buyer"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
	"livecommerce/migrations"
)

// Reuse the exact pre-0029 fixture bytes and install only later accepted old
// versions. In particular, neither 0032 nor post/0005 exists at this boundary.
func lriPre0032Fixture(t *testing.T) *testFixture {
	t.Helper()
	f := mcPre0029Fixture(t)
	ctx := context.Background()
	mcApplyHistorical(t, f, "0029_meta_social_consumer.sql", "0030_meta_runtime.sql", "0031_meta_river_isolation.sql")
	upstream, err := rivermigrate.New(riverpgxv5.New(f.owner), &rivermigrate.Config{Schema: "river_meta", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := upstream.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatal(err)
	}
	mcApplyHistorical(t, f, "post_river/0004_meta_river_isolation.sql")
	mustExec(t, f.owner, `GRANT USAGE ON SCHEMA river_meta TO commerce_meta_worker;
	 GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA river_meta TO commerce_meta_worker;
	 REVOKE ALL ON river_meta.river_migration FROM commerce_meta_worker;
	 GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA river_meta TO commerce_meta_worker`)
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version IN ('0032_legacy_river_isolation.sql','post_river/0005_legacy_river_isolation.sql')`) != 0 {
		t.Fatal("historical fixture crossed 0032 boundary")
	}
	// taiwan-cvs-logistics-v1 (post_river/0017): the current checkout Go calls the 10-argument begin_hold, which this old boundary
	// (8-argument 0013 body, no CVS schema) cannot host. A SECURITY INVOKER shim keeps the card-order path of these historical
	// fixtures exercising the old body unchanged; the extra arguments carry nothing an old schema could act on and the two result keys are the card-order constants.
	// post_river/0017 drops this shim (DROP FUNCTION IF EXISTS) before creating the real function, so migrations.Apply upgrades these fixtures.
	mustExec(t, f.owner, `CREATE FUNCTION checkout.begin_hold(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,p_order uuid,
	 p_snapshot jsonb,p_lines jsonb,p_job_id bigint,p_payment_environment text,p_payment_mode text) RETURNS jsonb
	 LANGUAGE sql AS $$ SELECT checkout.begin_hold(p_hash,p_store,p_key,p_request_hash,p_order,p_snapshot,p_lines,p_job_id)
	 || jsonb_build_object('payment_mode','card','commercial_state','DRAFT') $$;
	 REVOKE ALL ON FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text) FROM PUBLIC;
	 GRANT EXECUTE ON FUNCTION checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint,text,text) TO commerce_checkout_runtime`)
	runtime, err := platform.OpenPool(ctx, bcRole(t, f, "commerce_runtime"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	f.runtime = runtime
	return f
}

// lriShims is the historical-schema fallback for the CURRENT Go code on a pre-0032 fixture. The Go packages name schema
// objects that later migrations added: internal/pricing SetPolicy/LockCurrent name pricing.policy_versions.
// free_shipping_threshold_minor (0088, contracts/storefront-v2.md section C); internal/storefront SetCart/applyLivePrices
// name storefront.cart_lines.claim_* (0092, live tools); internal/checkout Begin calls checkout.set_order_locale (0097).
// On the old schema the first such statement failed (42703/42883, surfaced as "checkout database unavailable") and no
// legacy upgrade gate could even seed its checkout. Product code stays unchanged: the fixture receives a minimal SHIM of
// each object (same name and signature as the real migration, no behaviour beyond what an old order can carry), only
// when it is absent, exactly like the begin_hold shim in lriPre0032Fixture. Every legacy gate upgrades through
// lriApply, which drops the shims whose migration is not yet recorded right before Apply (the real ADD COLUMN / CREATE
// FUNCTION have no IF NOT EXISTS). Column shims only ever hold NULL (no legacy fixture sets a threshold or a claim
// origin; the 0109 flag shim only ever holds its default true); lriApply refuses to drop one that holds data (held = SQL returning the count of non-default values).
var lriShims = []struct{ migration, exists, add, held, drop string }{
	{"0088_checkout_offline.sql",
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='pricing' AND table_name='policy_versions' AND column_name='free_shipping_threshold_minor')`,
		`ALTER TABLE pricing.policy_versions ADD COLUMN free_shipping_threshold_minor bigint;
		 GRANT SELECT(free_shipping_threshold_minor) ON pricing.policy_versions TO commerce_buyer_runtime`,
		`SELECT count(*) FROM pricing.policy_versions WHERE free_shipping_threshold_minor IS NOT NULL`,
		`ALTER TABLE pricing.policy_versions DROP COLUMN free_shipping_threshold_minor`},
	{"0092_live_tools.sql",
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='storefront' AND table_name='cart_lines' AND column_name='claim_bundle_id')`,
		`ALTER TABLE storefront.cart_lines ADD COLUMN claim_bundle_id uuid, ADD COLUMN claim_offer_id uuid, ADD COLUMN claim_quantity bigint`,
		`SELECT count(*) FROM storefront.cart_lines WHERE claim_bundle_id IS NOT NULL OR claim_offer_id IS NOT NULL OR claim_quantity IS NOT NULL`,
		`ALTER TABLE storefront.cart_lines DROP COLUMN claim_bundle_id, DROP COLUMN claim_offer_id, DROP COLUMN claim_quantity`},
	{"0097_order_locale.sql",
		`SELECT to_regprocedure('checkout.set_order_locale(bytea,uuid,uuid,text)') IS NOT NULL`,
		`CREATE FUNCTION checkout.set_order_locale(p_hash bytea,p_store uuid,p_order uuid,p_locale text) RETURNS void LANGUAGE sql AS 'SELECT';
		 REVOKE ALL ON FUNCTION checkout.set_order_locale(bytea,uuid,uuid,text) FROM PUBLIC;
		 GRANT EXECUTE ON FUNCTION checkout.set_order_locale(bytea,uuid,uuid,text) TO commerce_checkout_runtime`,
		``,
		`DROP FUNCTION checkout.set_order_locale(bytea,uuid,uuid,text)`},
	// internal/checkout planLocked filters the lock plan by catalog.skus.inventory_tracked (A6, 0109 product-core). The
	// shim holds only the real default (true = tracked, the pre-0109 behaviour); 0109 adds the column with its CHECK.
	{"0109_product_core.sql",
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='catalog' AND table_name='skus' AND column_name='inventory_tracked')`,
		`ALTER TABLE catalog.skus ADD COLUMN inventory_tracked boolean NOT NULL DEFAULT true`,
		`SELECT count(*) FROM catalog.skus WHERE NOT inventory_tracked`,
		`ALTER TABLE catalog.skus DROP COLUMN inventory_tracked`},
	// 0113: an old card-only fixture has no imported claim version. Unlike
	// the real column, this shim rejects every non-NULL version.
	{"0113_ads_attribution.sql",
		`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='storefront' AND table_name='cart_lines' AND column_name='claim_line_version')`,
		`ALTER TABLE storefront.cart_lines ADD COLUMN claim_line_version bigint CONSTRAINT lri_claim_version_null CHECK(claim_line_version IS NULL);
		 COMMENT ON COLUMN storefront.cart_lines.claim_line_version IS 'lri_prehead_0113_test_only';`,
		`SELECT count(*) FROM storefront.cart_lines WHERE claim_line_version IS NOT NULL`,
		`ALTER TABLE storefront.cart_lines DROP COLUMN claim_line_version`},
}

const lriAttributionMarker = "lri_prehead_0113_test_only"

// This is a test fixture adapter, not an attribution implementation. Only the
// historical schema marker and authenticated ordinary card-order scope pass.
// There is no money/stock/job/receipt change and no creation-guard replacement.
const lriAttributionStub = `CREATE FUNCTION orders.freeze_attribution(p_hash bytea,p_store uuid,p_order uuid,p_touch jsonb,p_ip text,p_signals jsonb DEFAULT NULL)
 RETURNS void LANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS $shim$
 DECLARE s record;
 BEGIN
  IF p_touch IS NOT NULL OR coalesce(p_ip,'')<>'' OR p_signals IS NOT NULL THEN
   RAISE EXCEPTION 'historical fixture cannot carry attribution input' USING ERRCODE='23514';
  END IF;
  IF (SELECT obj_description(n.oid,'pg_namespace') FROM pg_namespace n WHERE n.nspname='orders') IS DISTINCT FROM 'lri_prehead_0113_test_only'
   OR to_regclass('orders.order_attribution') IS NOT NULL
   OR to_regprocedure('claims.capture_order_origins(uuid,uuid,uuid,uuid,jsonb)') IS NOT NULL
   OR to_regprocedure('checkout.begin_hold(bytea,uuid,text,bytea,uuid,jsonb,jsonb,bigint)') IS NULL THEN
   RAISE EXCEPTION 'not a historical attribution fixture' USING ERRCODE='23514';
  END IF;
  SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
  IF NOT FOUND OR s.tenant_id::text IS DISTINCT FROM current_setting('app.tenant_id',true)
   OR s.store_id::text IS DISTINCT FROM current_setting('app.store_id',true)
   OR s.owner_id::text IS DISTINCT FROM current_setting('app.buyer_id',true)
   OR NOT EXISTS(SELECT 1 FROM checkout.orders o WHERE o.id=p_order AND o.tenant_id=s.tenant_id AND o.store_id=s.store_id AND o.owner_id=s.owner_id) THEN
   RAISE EXCEPTION 'historical fixture order unavailable' USING ERRCODE='PT404';
  END IF;
  IF EXISTS(SELECT 1 FROM storefront.cart_lines l WHERE l.tenant_id=s.tenant_id AND l.store_id=s.store_id AND l.owner_id=s.owner_id
    AND (l.claim_bundle_id IS NOT NULL OR l.claim_offer_id IS NOT NULL OR l.claim_quantity IS NOT NULL OR l.claim_line_version IS NOT NULL)) THEN
   RAISE EXCEPTION 'historical fixture cannot carry claim origins' USING ERRCODE='23514';
  END IF;
 END $shim$;
 REVOKE ALL ON FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb) FROM PUBLIC;
 GRANT EXECUTE ON FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb) TO commerce_checkout_runtime;
 COMMENT ON FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb) IS 'lri_prehead_0113_test_only';`

func lriAddAttributionStub(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var recorded bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.lc_schema_migrations WHERE version='0113_ads_attribution.sql')`).Scan(&recorded); err != nil {
		return err
	}
	if recorded {
		return nil
	} // never alter an actual 0113 schema/function/column
	var columnMarker *string
	if err = tx.QueryRow(ctx, `SELECT col_description('storefront.cart_lines'::regclass,a.attnum) FROM pg_attribute a WHERE a.attrelid='storefront.cart_lines'::regclass AND a.attname='claim_line_version' AND NOT a.attisdropped`).Scan(&columnMarker); err != nil {
		return err
	}
	if columnMarker == nil || *columnMarker != lriAttributionMarker {
		return fmt.Errorf("refusing unowned historical attribution column")
	}
	var schemaExists bool
	var marker *string
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname='orders'),(SELECT obj_description(oid,'pg_namespace') FROM pg_namespace WHERE nspname='orders')`).Scan(&schemaExists, &marker); err != nil {
		return err
	}
	if schemaExists && (marker == nil || *marker != lriAttributionMarker) {
		return fmt.Errorf("refusing existing orders schema")
	}
	if !schemaExists {
		if _, err = tx.Exec(ctx, `CREATE SCHEMA orders; REVOKE ALL ON SCHEMA orders FROM PUBLIC; GRANT USAGE ON SCHEMA orders TO commerce_checkout_runtime; COMMENT ON SCHEMA orders IS 'lri_prehead_0113_test_only'`); err != nil {
			return err
		}
	}
	var fnExists bool
	if err = tx.QueryRow(ctx, `SELECT to_regprocedure('orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb)') IS NOT NULL`).Scan(&fnExists); err != nil {
		return err
	}
	if fnExists {
		if err = tx.QueryRow(ctx, `SELECT obj_description(to_regprocedure('orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb)'),'pg_proc')`).Scan(&marker); err != nil {
			return err
		}
		if marker == nil || *marker != lriAttributionMarker {
			return fmt.Errorf("refusing existing attribution function")
		}
	} else if _, err = tx.Exec(ctx, lriAttributionStub); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Cleanup validates ALL 0113-owned shim objects first, then removes them in one
// transaction. No CASCADE: foreign tables/functions/dependencies are preserved.
func lriDropAttributionShims(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var recorded bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.lc_schema_migrations WHERE version='0113_ads_attribution.sql')`).Scan(&recorded); err != nil {
		return err
	}
	if recorded {
		return nil
	}
	var columnExists bool
	var marker *string
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='storefront.cart_lines'::regclass AND attname='claim_line_version' AND NOT attisdropped),
 (SELECT col_description(attrelid,attnum) FROM pg_attribute WHERE attrelid='storefront.cart_lines'::regclass AND attname='claim_line_version' AND NOT attisdropped)`).Scan(&columnExists, &marker); err != nil {
		return err
	}
	if columnExists {
		if marker == nil || *marker != lriAttributionMarker {
			return fmt.Errorf("refusing unowned historical attribution column")
		}
		var held int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM storefront.cart_lines WHERE claim_line_version IS NOT NULL OR claim_bundle_id IS NOT NULL OR claim_offer_id IS NOT NULL OR claim_quantity IS NOT NULL`).Scan(&held); err != nil || held != 0 {
			return fmt.Errorf("historical attribution shim holds %d values (err %v)", held, err)
		}
	}
	var schemaExists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname='orders'),(SELECT obj_description(oid,'pg_namespace') FROM pg_namespace WHERE nspname='orders')`).Scan(&schemaExists, &marker); err != nil {
		return err
	}
	if schemaExists {
		if marker == nil || *marker != lriAttributionMarker {
			return fmt.Errorf("refusing existing orders schema")
		}
		var foreign int
		if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='orders')+
 (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='orders' AND
 (p.oid<>coalesce(to_regprocedure('orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb)')::oid,0) OR obj_description(p.oid,'pg_proc') IS DISTINCT FROM 'lri_prehead_0113_test_only' OR p.prosecdef))`).Scan(&foreign); err != nil || foreign != 0 {
			return fmt.Errorf("refusing orders shim schema with %d foreign/data objects (err %v)", foreign, err)
		}
		if _, err = tx.Exec(ctx, `DROP FUNCTION IF EXISTS orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb); DROP SCHEMA orders`); err != nil {
			return err
		}
	}
	if columnExists {
		if _, err = tx.Exec(ctx, `ALTER TABLE storefront.cart_lines DROP COLUMN claim_line_version`); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// lriAddShims adds every missing shim (idempotent). Called by the shared policy writer, so any historical fixture that
// reaches pricing gets the objects the current Go names.
func lriAddShims(f *testFixture) error {
	ctx := context.Background()
	for _, sh := range lriShims {
		var has bool
		if err := f.owner.QueryRow(ctx, sh.exists).Scan(&has); err != nil {
			return err
		}
		if has {
			continue
		}
		if _, err := f.owner.Exec(ctx, sh.add); err != nil {
			return err
		}
	}
	return lriAddAttributionStub(ctx, f.owner)
}

// lriApply is migrations.Apply for the legacy upgrade gates: it first drops the shims whose migration is not yet
// recorded, refusing if a column shim ever held data. Idempotent on a retry.
func lriApply(ctx context.Context, pool *pgxpool.Pool) error {
	if err := lriDropAttributionShims(ctx, pool); err != nil {
		return err
	}
	for _, sh := range lriShims {
		var has, recorded bool
		if err := pool.QueryRow(ctx, sh.exists).Scan(&has); err != nil {
			return err
		}
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.lc_schema_migrations WHERE version=$1)`, sh.migration).Scan(&recorded); err != nil {
			return err
		}
		if !has || recorded {
			continue
		}
		if sh.held != "" {
			var held int
			if err := pool.QueryRow(ctx, sh.held).Scan(&held); err != nil || held != 0 {
				return fmt.Errorf("historical shim of %s holds %d values (err %v); refusing to drop it", sh.migration, held, err)
			}
		}
		if _, err := pool.Exec(ctx, sh.drop); err != nil {
			return err
		}
	}
	return migrations.Apply(ctx, pool)
}

// These probes exercise only the historical adapter. None calls, replaces, or
// relaxes the real 0113 creation guard.
func TestLegacyRuntimeIsolationAttributionShimsRestricted(t *testing.T) {
	f := lriPre0032Fixture(t)
	q := pwOldQuerySetupOn(t, f, pwKeys(t), "river") // actual old Begin + payment producer
	ctx := context.Background()
	hash := sha256.Sum256([]byte(q.cap.Token))
	call := func(order string, touch any, ip any) error {
		_, err := cqBuyer(q.pool, q.cap, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) (bool, error) {
			_, err := tx.Exec(ctx, `SELECT orders.freeze_attribution($1,$2,$3,$4::jsonb,$5::text)`, hash[:], q.cap.Scope.StoreID, order, touch, ip)
			return err == nil, err
		})
		return err
	}
	tables := []string{"checkout.orders", "checkout.payment_attempts", "inventory.ledger", "river.river_job"}
	before := make(map[string]string, len(tables))
	for _, table := range tables {
		before[table] = lriRows(t, f, table, "")
	}
	if err := call(q.hold.OrderID, nil, ""); err != nil {
		t.Fatal("authenticated ordinary historical order", err)
	}
	if err := call(q.hold.OrderID, nil, nil); err != nil {
		t.Fatal("NULL historical IP", err)
	}
	for _, tc := range []struct {
		name  string
		touch any
		ip    any
	}{
		{"empty touch object", `{}`, ""},
		{"JSON null is not SQL NULL", `null`, ""},
		{"nonempty touch", `{"draft_id":"synthetic"}`, ""},
		{"IP", nil, "192.0.2.44"},
		{"whitespace IP", nil, " "},
	} {
		t.Run(tc.name, func(t *testing.T) { requirePGCode(t, call(q.hold.OrderID, tc.touch, tc.ip), "23514", tc.name) })
	}
	requirePGCode(t, call(randomUUID(), nil, ""), "PT404", "missing/foreign order")
	other := mustIssue(t, q.cqHarness.service, q.cap.Scope.StoreID)
	otherHash := sha256.Sum256([]byte(other.Token))
	_, err := cqBuyer(q.pool, other, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) (bool, error) {
		_, err := tx.Exec(ctx, `SELECT orders.freeze_attribution($1,$2,$3,NULL,'')`, otherHash[:], other.Scope.StoreID, q.hold.OrderID)
		return false, err
	})
	requirePGCode(t, err, "PT404", "other buyer's order")
	_, err = q.pool.Exec(ctx, `SELECT orders.freeze_attribution($1,$2,$3,NULL,'')`, hash[:], q.cap.Scope.StoreID, q.hold.OrderID)
	requirePGCode(t, err, "PT404", "missing transaction scope")
	_, err = f.owner.Exec(ctx, `UPDATE storefront.cart_lines SET claim_line_version=1 WHERE owner_id=$1`, q.cap.Scope.OwnerID)
	requirePGCode(t, err, "23514", "shim version must remain NULL")
	mustExec(t, f.owner, `UPDATE storefront.cart_lines SET claim_bundle_id=$2 WHERE owner_id=$1`, q.cap.Scope.OwnerID, randomUUID())
	requirePGCode(t, call(q.hold.OrderID, nil, ""), "23514", "claim origin cannot be ignored")
	if err := lriDropAttributionShims(ctx, f.owner); err == nil || !strings.Contains(err.Error(), "holds") {
		t.Fatalf("cleanup must refuse claim provenance: %v", err)
	}
	mustExec(t, f.owner, `UPDATE storefront.cart_lines SET claim_bundle_id=NULL WHERE owner_id=$1`, q.cap.Scope.OwnerID)
	mustExec(t, f.owner, `COMMENT ON SCHEMA orders IS 'not_owned_by_lri'`)
	requirePGCode(t, call(q.hold.OrderID, nil, ""), "23514", "nonfixture schema")
	if err := lriAddAttributionStub(ctx, f.owner); err == nil {
		t.Fatal("installer accepted unowned schema")
	}
	if err := lriDropAttributionShims(ctx, f.owner); err == nil {
		t.Fatal("cleanup accepted unowned schema")
	}
	mustExec(t, f.owner, `COMMENT ON SCHEMA orders IS 'lri_prehead_0113_test_only'`)
	for _, table := range tables {
		if got := lriRows(t, f, table, ""); got != before[table] {
			t.Fatalf("historical attribution adapter mutated %s", table)
		}
	}
	if miCount(t, f.owner, `SELECT count(*) FROM pg_proc WHERE oid=to_regprocedure('orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb)') AND NOT prosecdef
	 AND NOT has_function_privilege('commerce_buyer_runtime',oid,'EXECUTE') AND has_function_privilege('commerce_checkout_runtime',oid,'EXECUTE')`) != 1 {
		t.Fatal("shim must remain invoker with checkout-only execution")
	}
}

func TestLegacyRuntimeIsolationAttributionShimsCleanup(t *testing.T) {
	f := lriPre0032Fixture(t)
	q := pwOldQuerySetupOn(t, f, pwKeys(t), "river")
	ctx := context.Background()
	columnAndFunction := func() bool {
		return miCount(t, f.owner, `SELECT count(*) FROM pg_attribute WHERE attrelid='storefront.cart_lines'::regclass AND attname='claim_line_version' AND NOT attisdropped
	 AND to_regprocedure('orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb)') IS NOT NULL`) == 1
	}
	ledger := lriRows(t, f, "public.lc_schema_migrations", "")
	// Explicitly defeat only the test-only CHECK to probe held-data cleanup.
	mustExec(t, f.owner, `ALTER TABLE storefront.cart_lines DROP CONSTRAINT lri_claim_version_null`)
	mustExec(t, f.owner, `UPDATE storefront.cart_lines SET claim_line_version=1 WHERE owner_id=$1`, q.cap.Scope.OwnerID)
	if err := lriDropAttributionShims(ctx, f.owner); err == nil || !strings.Contains(err.Error(), "holds") || !columnAndFunction() {
		t.Fatalf("non-NULL cleanup did not refuse atomically: %v", err)
	}
	mustExec(t, f.owner, `UPDATE storefront.cart_lines SET claim_line_version=NULL WHERE owner_id=$1`, q.cap.Scope.OwnerID)
	mustExec(t, f.owner, `ALTER TABLE storefront.cart_lines ADD CONSTRAINT lri_claim_version_null CHECK(claim_line_version IS NULL)`)
	mustExec(t, f.owner, `CREATE TABLE orders.lri_foreign_data(value integer); INSERT INTO orders.lri_foreign_data VALUES(7)`)
	if err := lriDropAttributionShims(ctx, f.owner); err == nil || !columnAndFunction() || miCount(t, f.owner, `SELECT count(*) FROM orders.lri_foreign_data WHERE value=7`) != 1 {
		t.Fatalf("foreign/data objects were not retained: %v", err)
	}
	mustExec(t, f.owner, `DROP TABLE orders.lri_foreign_data`) // only this probe's table, no CASCADE
	// The exact signature is insufficient proof of ownership; a lost marker
	// must retain the entire adapter, including its column.
	mustExec(t, f.owner, `COMMENT ON FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb) IS NULL`)
	if err := lriDropAttributionShims(ctx, f.owner); err == nil || !columnAndFunction() {
		t.Fatalf("unowned function cleanup was not rejected atomically: %v", err)
	}
	mustExec(t, f.owner, `COMMENT ON FUNCTION orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb) IS 'lri_prehead_0113_test_only'`)
	if err := lriDropAttributionShims(ctx, f.owner); err != nil {
		t.Fatal("clean empty owned adapter", err)
	}
	if miCount(t, f.owner, `SELECT count(*) FROM pg_namespace WHERE nspname='orders'`) != 0 ||
		miCount(t, f.owner, `SELECT count(*) FROM pg_attribute WHERE attrelid='storefront.cart_lines'::regclass AND attname='claim_line_version' AND NOT attisdropped`) != 0 ||
		lriRows(t, f, "public.lc_schema_migrations", "") != ledger {
		t.Fatal("cleanup left attribution adapter or changed historical ledger")
	}
	if err := lriDropAttributionShims(ctx, f.owner); err != nil {
		t.Fatal("cleanup retry", err)
	}
}

func TestLegacyRuntimeIsolationAttributionShimsHeadUntouched(t *testing.T) {
	f := pwIsolatedFixture(t) // real current migrations, not the pre-head adapter
	ctx := context.Background()
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0113_ads_attribution.sql'`) != 1 {
		t.Fatal("current fixture did not install real 0113")
	}
	objects := func() string {
		var out string
		if err := f.owner.QueryRow(ctx, `SELECT jsonb_build_object('function',pg_get_functiondef(to_regprocedure('orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb)')),
	 'relations',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='orders'),
	 'constraints',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.oid) FROM pg_constraint c WHERE c.conrelid='storefront.cart_lines'::regclass),
	 'ledger',(SELECT to_jsonb(m) FROM public.lc_schema_migrations m WHERE version='0113_ads_attribution.sql'))::text`).Scan(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	before := objects()
	if err := lriAddShims(f); err != nil {
		t.Fatal("head installer must leave real attribution unchanged", err)
	}
	if err := lriDropAttributionShims(ctx, f.owner); err != nil {
		t.Fatal("head cleanup must leave real attribution unchanged", err)
	}
	if objects() != before {
		t.Fatal("historical helper changed real 0113 function/table/constraint/ledger")
	}
}

// Historical post-River router gates must replay their original SQL bytes at
// the old cutover, not call latest Apply (which now performs the 0032 cutover).
func lriApplyHistoricalPost(f *testFixture, target string) error {
	ctx := context.Background()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	paths, err := filepath.Glob("../../migrations/[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil {
		return err
	}
	posts, err := filepath.Glob("../../migrations/post_river/[0-9][0-9][0-9][0-9]_*.sql")
	if err != nil {
		return err
	}
	known := map[string]string{}
	for _, path := range append(paths, posts...) {
		version := filepath.Base(path)
		if strings.Contains(path, "post_river/") {
			version = "post_river/" + version
			if version >= "post_river/0005" {
				continue
			}
		} else if version >= "0032" {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		known[version] = fmt.Sprintf("%x", sha256.Sum256(body))
	}
	rows, err := tx.Query(ctx, `SELECT version,checksum FROM public.lc_schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return err
		}
		if expected, ok := known[version]; !ok || checksum != expected {
			rows.Close()
			return fmt.Errorf("historical version unknown or changed: %s", version)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, ok := known[target]; !ok {
		return fmt.Errorf("historical post-River target unknown: %s", target)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.lc_schema_migrations WHERE version=$1)`, target).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		body, err := os.ReadFile(filepath.Join("../../migrations", target))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES($1,$2)`, target, known[target]); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return waResyncLegacy(f) // the replayed old SQL granted its objects to commerce_worker only
}

func lriRows(t *testing.T, f *testFixture, table, predicate string) string {
	t.Helper()
	return miIsoRows(t, f, table, predicate)
}

func lriLedger(t *testing.T, f *testFixture, predicate string) string {
	t.Helper()
	return lriRows(t, f, "public.lc_schema_migrations", predicate)
}

func lriCloseProducerPools(p psHarness) {
	p.pool.Close()
	p.worker.Close()
	p.expiry.Close()
	p.a.runtime.Close()
	p.a.issuer.Close()
	p.a.identity.Close()
}

// Both non-MOCK profiles still use the production old-schema StartPayment
// transaction. Only the local owner prepares consistent qualification facts;
// neither branch contacts a provider or purports to prove real qualification.
func lriOldProfileQuery(t *testing.T, f *testFixture, profile string) pqFixture {
	t.Helper()
	p := psSetupItemsOn(t, f, 1, "river")
	if profile == "SANDBOX" {
		qualExec(t, f.owner, `UPDATE payments.account_qualifications SET proof_class='REAL_SANDBOX',evidence_ref='local synthetic qualification; no provider call' WHERE id=$1`, p.proof)
	} else if profile == "LIVE" {
		binding, account, proof := randomUUID(), randomUUID(), randomUUID()
		ctx := context.Background()
		tx, err := f.owner.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		steps := []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO integration.bindings(id,tenant_id,store_id,principal_id,provider,external_asset_id) VALUES($1,$2,$3,$4,'payuni','LIVE:local-upgrade-account')`, []any{binding, p.f.tenantA, p.f.storeA1, p.f.principalA}},
			{`INSERT INTO integration.merchant_accounts(id,tenant_id,store_id,principal_id,provider,environment,account_id,binding_id,credential_version) VALUES($1,$2,$3,$4,'payuni','LIVE','local-upgrade-account',$5,1)`, []any{account, p.f.tenantA, p.f.storeA1, p.f.principalA, binding}},
			{`INSERT INTO integration.account_credentials(tenant_id,store_id,connection_id,version,key_id,nonce,ciphertext,principal_id) VALUES($1,$2,$3,1,'mock_key',decode(repeat('00',12),'hex'),decode(repeat('00',17),'hex'),$4)`, []any{p.f.tenantA, p.f.storeA1, account, p.f.principalA}},
			{`INSERT INTO payments.account_qualifications(id,tenant_id,store_id,connection_id,credential_version,environment,code,proof_class,evidence_ref,observed_at,expires_at) VALUES($1,$2,$3,$4,1,'LIVE','payuni_credit','REAL_LIVE','local synthetic qualification; no provider call',clock_timestamp()-interval '1 second',clock_timestamp()+interval '1 hour')`, []any{proof, p.f.tenantA, p.f.storeA1, account}},
			{`UPDATE payments.method_versions SET connection_id=$2,qualification_id=$3,environment='LIVE' WHERE qualification_id=$1`, []any{p.proof, account, proof}},
		}
		for _, step := range steps {
			if _, err := tx.Exec(ctx, step.sql, step.args...); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		p.binding, p.account, p.proof = binding, account, proof
	} else {
		t.Fatal("unsupported historical profile")
	}
	p.starter = psStarterIn(t, p.pool, profile, "river")
	result, err := p.start(t04Key("lri-old-profile"))
	if err != nil {
		t.Fatalf("old-schema %s payment admission: %v", profile, err)
	}
	return pqFixture{psHarness: p, result: result, schema: "river"}
}

func lriProfileAdmission(t *testing.T, q pqFixture, profile, environment, proof, queue string) {
	t.Helper()
	var gotProfile, gotEnv, gotProof, gotQualEnv, gotMethodEnv, gotAccountEnv, gotQueue string
	var assetMatches bool
	err := q.f.owner.QueryRow(context.Background(), `SELECT a.execution_profile,a.environment,qual.proof_class,qual.environment,m.environment,acct.environment,j.queue,b.external_asset_id=acct.binding_asset
	 FROM checkout.payment_attempts a JOIN payments.account_qualifications qual ON qual.id=a.qualification_id
	 JOIN payments.method_versions m ON m.tenant_id=a.tenant_id AND m.store_id=a.store_id AND m.market_id=a.market_id AND m.country=a.country AND m.code=a.method_code AND m.version=a.method_version
	 JOIN integration.merchant_accounts acct ON acct.id=a.connection_id JOIN integration.bindings b ON b.id=a.binding_id
	 JOIN river.river_job j ON j.id=a.job_id AND j.kind='payment_query_v1'
	 WHERE a.id=$1`, q.result.AttemptID).Scan(&gotProfile, &gotEnv, &gotProof, &gotQualEnv, &gotMethodEnv, &gotAccountEnv, &gotQueue, &assetMatches)
	if err != nil || !assetMatches || gotProfile != profile || gotEnv != environment || gotProof != proof || gotQualEnv != environment || gotMethodEnv != environment || gotAccountEnv != environment || gotQueue != queue {
		t.Fatalf("historical %s qualification/profile/job facts drifted: err=%v profile=%s env=%s proof=%s qual=%s method=%s account=%s queue=%s asset=%t", profile, err, gotProfile, gotEnv, gotProof, gotQualEnv, gotMethodEnv, gotAccountEnv, gotQueue, assetMatches)
	}
}

func lriReady(t *testing.T, f *testFixture, want bool) {
	t.Helper()
	var payment, expiry bool
	if err := f.owner.QueryRow(context.Background(), `SELECT integration.payment_queue_ready(),checkout.expiry_queue_ready()`).Scan(&payment, &expiry); err != nil {
		t.Fatal(err)
	}
	if payment != want || expiry != want {
		t.Fatalf("legacy family readiness payment=%t expiry=%t want=%t", payment, expiry, want)
	}
}

func lriNoPost(t *testing.T, f *testFixture, sourceJobs, sourceQueues, paymentDest, expiryDest, paymentQueues, expiryQueues string) {
	t.Helper()
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='post_river/0005_legacy_river_isolation.sql'`) != 0 {
		t.Fatal("failed cutover committed the post checksum")
	}
	if got := lriRows(t, f, "river.river_job", ""); got != sourceJobs {
		t.Fatal("failed cutover mutated source jobs")
	}
	if got := lriRows(t, f, "river.river_queue", ""); got != sourceQueues {
		t.Fatal("failed cutover mutated source queues")
	}
	if got := lriRows(t, f, "river_payment.river_job", ""); got != paymentDest {
		t.Fatal("failed cutover mutated payment destination")
	}
	if got := lriRows(t, f, "river_expiry.river_job", ""); got != expiryDest {
		t.Fatal("failed cutover mutated expiry destination")
	}
	if got := lriRows(t, f, "river_payment.river_queue", ""); got != paymentQueues {
		t.Fatal("failed cutover mutated payment destination queues")
	}
	if got := lriRows(t, f, "river_expiry.river_queue", ""); got != expiryQueues {
		t.Fatal("failed cutover mutated expiry destination queues")
	}
	lriReady(t, f, false)
}

// This is an actual pre-0032 cluster. Every source job is admitted by the old
// producer first; owner-only state/profile changes model persisted lifecycle
// history, not fabricated historical admission XIDs.
func TestLegacyRuntimeIsolationPopulatedUpgrade(t *testing.T) {
	f := lriPre0032Fixture(t)
	ctx := context.Background()
	keys := pwKeys(t)
	queries := [3]pqFixture{}
	queries[0] = pwOldQuerySetupOn(t, f, keys, "river")
	queries[1] = lriOldProfileQuery(t, f, "SANDBOX")
	lriCloseProducerPools(queries[1].psHarness)
	queries[2] = lriOldProfileQuery(t, f, "LIVE")
	lriCloseProducerPools(queries[2].psHarness)
	for i, want := range []struct{ profile, env, proof, queue string }{{"PROVIDER_MOCK", "SANDBOX", "PROVIDER_MOCK", "payment_mock_v1"}, {"SANDBOX", "SANDBOX", "REAL_SANDBOX", "payment_sandbox_v1"}, {"LIVE", "LIVE", "REAL_LIVE", "payment_live_v1"}} {
		lriProfileAdmission(t, queries[i], want.profile, want.env, want.proof, want.queue)
	}
	pcRecord(t, queries[0], pcFull(queries[0]))
	lriCloseProducerPools(queries[0].psHarness)
	oldClient, err := river.NewClient(riverpgxv5.New(f.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	external, err := integration.New(oldClient)
	if err != nil {
		t.Fatal(err)
	}
	err = platform.WithScope(ctx, f.runtime, queries[0].f.tokens["a"], queries[0].f.storeA1, "store:read", func(tx pgx.Tx, scope platform.Scope) error {
		_, e := external.Plan(ctx, tx, scope, queries[0].f.tokens["a"], t04Key("lri-external"), integration.PlanInput{
			BindingID: queries[0].binding, ExpectedBindingVersion: 1, Purpose: "transactional", Action: "payment.authorize", Request: json.RawMessage(`{"amount":1}`),
		})
		return e
	})
	if err != nil {
		t.Fatal("old external producer", err)
	}
	meta := mrSetup(t, f)
	asset := miAsset()
	binding := miBinding(t, meta, asset, "facebook", f.tenantA, f.storeA1, f.principalA)
	miRoute(t, meta, asset, f.tenantA, f.storeA1, binding)
	metaEvent := mcPost(t, meta, asset, miMessage(asset, "m."+randomUUID(), "legacy-family-cutover"))
	if metaEvent.job < 1 {
		t.Fatal("Meta job did not enter isolated lane")
	}
	meta.ingress.Close()
	meta.registrar.Close()
	meta.curator.Close()
	var reconcile int64
	if err := f.owner.QueryRow(ctx, `SELECT id FROM river.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, queries[0].result.OperationID).Scan(&reconcile); err != nil {
		t.Fatal(err)
	}
	// Distinct linked holds cover all retained non-running River states. The
	// existing query fixtures also contribute three linked expiry jobs.
	expiry := make([]psHarness, 4)
	for i := range expiry {
		expiry[i] = ewSetup(t, f, 1, "river")
		lriCloseProducerPools(expiry[i])
	}
	mustExec(t, f.owner, `UPDATE river.river_job SET state='scheduled',scheduled_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, queries[1].result.JobID)
	mustExec(t, f.owner, `UPDATE river.river_job SET state='retryable',attempt=1,attempted_at=clock_timestamp(),scheduled_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, queries[2].result.JobID)
	mustExec(t, f.owner, `UPDATE river.river_job SET state='completed',attempt=1,attempted_at=clock_timestamp(),finalized_at=clock_timestamp(),queue='default' WHERE id=$1`, reconcile)
	mustExec(t, f.owner, `UPDATE river.river_job SET state='available',scheduled_at=clock_timestamp() WHERE id=$1`, queries[0].hold.JobID)
	for i, state := range []string{"pending", "scheduled", "retryable", "completed"} {
		if state == "completed" {
			mustExec(t, f.owner, `UPDATE river.river_job SET state='completed',attempt=1,attempted_at=clock_timestamp(),finalized_at=clock_timestamp(),scheduled_at=clock_timestamp()+interval '1 hour',queue='default' WHERE id=$1`, expiry[i].hold.JobID)
		} else {
			mustExec(t, f.owner, `UPDATE river.river_job SET state=$2,scheduled_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, expiry[i].hold.JobID, state)
		}
	}
	mustExec(t, f.owner, `UPDATE river.river_job SET state='cancelled',finalized_at=clock_timestamp(),queue='default' WHERE id=$1`, queries[1].hold.JobID)
	mustExec(t, f.owner, `UPDATE river.river_job SET state='discarded',finalized_at=clock_timestamp(),queue='default' WHERE id=$1`, queries[2].hold.JobID)
	pruned := ewSetup(t, f, 1, "river")
	lriCloseProducerPools(pruned)
	mustExec(t, f.owner, `UPDATE river.river_job SET state='discarded',finalized_at=clock_timestamp() WHERE id=$1`, pruned.hold.JobID)
	mustExec(t, f.owner, `DELETE FROM river.river_job WHERE id=$1`, pruned.hold.JobID)
	prunedPayment := pwOldQuerySetupOn(t, f, keys, "river")
	lriCloseProducerPools(prunedPayment.psHarness)
	mustExec(t, f.owner, `UPDATE river.river_job SET state='discarded',finalized_at=clock_timestamp() WHERE id=$1`, prunedPayment.result.JobID)
	mustExec(t, f.owner, `DELETE FROM river.river_job WHERE id=$1`, prunedPayment.result.JobID)
	for _, state := range []string{"available", "pending", "scheduled", "retryable", "completed", "cancelled", "discarded"} {
		if miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE kind IN ('payment_query_v1','payment_reconcile_v1','checkout_expiry_v1') AND state::text=$1`, state) == 0 {
			t.Fatalf("retained historical state %s absent", state)
		}
	}
	for _, queue := range []string{"payment_mock_v1", "payment_sandbox_v1", "payment_live_v1", "checkout_expiry_v1"} {
		mustExec(t, f.owner, `INSERT INTO river.river_queue(name,paused_at,metadata) VALUES($1,clock_timestamp(),'{"test":"upgrade"}'::jsonb) ON CONFLICT(name) DO UPDATE SET paused_at=excluded.paused_at,metadata=excluded.metadata`, queue)
	}
	var sourceHigh int64
	if err := f.owner.QueryRow(ctx, `SELECT setval('river.river_job_id_seq',greatest((SELECT last_value FROM river.river_job_id_seq),$1::bigint,$2::bigint)+100,true)`, pruned.hold.JobID, prunedPayment.result.JobID).Scan(&sourceHigh); err != nil {
		t.Fatal(err)
	}
	oldPayment := lriRows(t, f, "river.river_job", `WHERE kind IN ('payment_query_v1','payment_reconcile_v1')`)
	oldExpiry := lriRows(t, f, "river.river_job", `WHERE kind='checkout_expiry_v1'`)
	oldExternal := lriRows(t, f, "river.river_job", `WHERE kind NOT IN ('payment_query_v1','payment_reconcile_v1','checkout_expiry_v1')`)
	oldMeta := lriRows(t, f, "river_meta.river_job", "")
	oldQueues := map[string]string{}
	for _, queue := range []string{"payment_mock_v1", "payment_sandbox_v1", "payment_live_v1", "checkout_expiry_v1", "default"} {
		oldQueues[queue] = lriRows(t, f, "river.river_queue", `WHERE name='`+queue+`'`)
	}
	const historicalLedger = `WHERE (left(version,11) <> 'post_river/' AND version < '0032') OR (left(version,11) = 'post_river/' AND version < 'post_river/0005')`
	oldChecksums := lriLedger(t, f, historicalLedger)
	business := map[string]string{}
	for _, table := range []string{"checkout.orders", "checkout.payment_attempts", "payments.provider_observations", "integration.operations", "integration.operation_events", "inventory.reservations", "inventory.ledger"} {
		business[table] = lriRows(t, f, table, "")
	}
	// Apply includes forward0035, which adds one nullable media identity column, and 0159 (W6-05B operations ledger), which adds the
	// defaulted dispatcher budget base generation_floor (DEFAULT 0, only ever raised by an operator query/retry).
	// Preserve exact full-row equality: every historical operation must retain
	// every old value and acquire precisely media_attempt_id:null and generation_floor:0, never a link.
	var expectedOperations string
	if err := f.owner.QueryRow(ctx, `SELECT coalesce(jsonb_agg(
	 value || '{"media_attempt_id":null,"generation_floor":0}'::jsonb
	 ORDER BY (value || '{"media_attempt_id":null,"generation_floor":0}'::jsonb)::text),'[]'::jsonb)::text
	 FROM jsonb_array_elements($1::jsonb)`, business["integration.operations"]).Scan(&expectedOperations); err != nil {
		t.Fatal("expected additive media identity", err)
	}
	business["integration.operations"] = expectedOperations
	// Apply also includes 0072 (taiwan-cvs C4): checkout.orders gains payment_mode (default 'card') and collection_state (NULL); every
	// historical order keeps all old values and acquires exactly those keys. Documented later additive columns (each a nullable
	// or defaulted ADD COLUMN, so a historical order acquires only its default): 0088 buyer_email NULL (storefront-v2 section C),
	// 0094 source 'storefront' (section G, merchant-created orders are the only other value), 0097 locale NULL (order-locale unit;
	// the Begin of a fixture order ran against the lriShims no-op set_order_locale, so no locale was ever stored),
	// 0107 home-cod: cod_surcharge_minor NULL and cod_carrier NULL (both set only on cash_on_delivery orders) and collected_at NULL.
	// collected_at is the one 0107 column that is not a plain ADD COLUMN: the migration runs a labelled best-effort backfill
	// (collected_at=updated_at) for rows already collection_state='COLLECTED'. Every fixture order here is a historical card order
	// (collection_state NULL), so the backfill matches no row and collected_at stays NULL; a card order that DID gain a non-null
	// collected_at, or any other changed value, still fails the equality below.
	const addedOrderKeys = `{"payment_mode":"card","collection_state":null,"buyer_email":null,"source":"storefront","locale":null,"cod_surcharge_minor":null,"cod_carrier":null,"collected_at":null}`
	var expectedOrders string
	if err := f.owner.QueryRow(ctx, `SELECT coalesce(jsonb_agg(
	 value || $2::jsonb
	 ORDER BY (value || $2::jsonb)::text),'[]'::jsonb)::text
	 FROM jsonb_array_elements($1::jsonb)`, business["checkout.orders"], addedOrderKeys).Scan(&expectedOrders); err != nil {
		t.Fatal("expected additive CVS order columns", err)
	}
	business["checkout.orders"] = expectedOrders
	// Apply also includes 0041's five nullable recovery event columns. Keep
	// comparing every historical event value, with only these new keys NULL.
	var expectedEvents string
	if err := f.owner.QueryRow(ctx, `SELECT coalesce(jsonb_agg(
	 value || '{"episode_id":null,"episode_event_kind":null,"native_job_id":null,"observation_id":null,"elapsed_ms":null}'::jsonb
	 ORDER BY (value || '{"episode_id":null,"episode_event_kind":null,"native_job_id":null,"observation_id":null,"elapsed_ms":null}'::jsonb)::text),'[]'::jsonb)::text
	 FROM jsonb_array_elements($1::jsonb)`, business["integration.operation_events"]).Scan(&expectedEvents); err != nil {
		t.Fatal("expected additive media recovery event fields", err)
	}
	business["integration.operation_events"] = expectedEvents
	if oldPayment == "[]" || oldExpiry == "[]" || oldExternal == "[]" || oldMeta == "[]" || oldQueues["payment_mock_v1"] == "[]" || oldQueues["checkout_expiry_v1"] == "[]" {
		t.Fatal("historical populated source was empty")
	}
	if err := lriApply(ctx, f.owner); err != nil {
		t.Fatal("populated legacy cutover", err)
	}
	lriReady(t, f, true)
	if got := lriRows(t, f, "river_payment.river_job", ""); got != oldPayment {
		t.Fatal("payment full job rows changed during cutover")
	}
	if got := lriRows(t, f, "river_expiry.river_job", ""); got != oldExpiry {
		t.Fatal("expiry full job rows changed during cutover")
	}
	if got := lriRows(t, f, "river.river_job", ""); got != oldExternal {
		t.Fatal("external source rows changed during cutover")
	}
	if got := lriRows(t, f, "river_meta.river_job", ""); got != oldMeta {
		t.Fatal("Meta source rows changed during legacy cutover")
	}
	for queue, before := range oldQueues {
		table := "river.river_queue"
		if queue == "checkout_expiry_v1" {
			table = "river_expiry.river_queue"
		}
		if strings.HasPrefix(queue, "payment_") {
			table = "river_payment.river_queue"
		}
		if got := lriRows(t, f, table, `WHERE name='`+queue+`'`); got != before {
			t.Fatalf("%s full queue row changed", queue)
		}
	}
	for table, before := range business {
		if got := lriRows(t, f, table, ""); got != before {
			t.Fatalf("%s business rows changed", table)
		}
	}
	if got := lriLedger(t, f, historicalLedger); got != oldChecksums {
		t.Fatal("historical migration ledger/checksum changed")
	}
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version IN ('0032_legacy_river_isolation.sql','post_river/0005_legacy_river_isolation.sql')`) != 2 {
		t.Fatal("cutover ledger missing")
	}
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0033_live_planning.sql'`) != 1 {
		t.Fatal("live planning migration missing after populated cutover")
	}
	var sourceAfter int64
	if err := f.owner.QueryRow(ctx, `SELECT last_value FROM river.river_job_id_seq`).Scan(&sourceAfter); err != nil || sourceAfter != sourceHigh {
		t.Fatalf("source sequence changed: %d -> %d: %v", sourceHigh, sourceAfter, err)
	}
	postJobs := map[string]string{"river_payment.river_job": oldPayment, "river_expiry.river_job": oldExpiry, "river.river_job": oldExternal}
	postLedger := lriLedger(t, f, "")
	if err := lriApply(ctx, f.owner); err != nil {
		t.Fatal("repeat populated Apply", err)
	}
	for table, before := range postJobs {
		if got := lriRows(t, f, table, ""); got != before {
			t.Fatalf("repeat Apply mutated %s", table)
		}
	}
	if got := lriLedger(t, f, ""); got != postLedger {
		t.Fatal("repeat Apply mutated migration ledger")
	}
	for _, schema := range []string{"river_payment", "river_expiry"} {
		var next int64
		if err := f.owner.QueryRow(ctx, `SELECT nextval('`+schema+`.river_job_id_seq')`).Scan(&next); err != nil || next <= sourceHigh || next <= pruned.hold.JobID || next <= prunedPayment.result.JobID {
			t.Fatalf("%s sequence reused source/pruned high-water: next=%d high=%d pruned expiry=%d payment=%d err=%v", schema, next, sourceHigh, pruned.hold.JobID, prunedPayment.result.JobID, err)
		}
	}
	// A migrated linked, unpaused expiry row is actually consumed by the
	// current family-bound worker after the byte-equality snapshots above.
	mustExec(t, f.owner, `UPDATE river_expiry.river_queue SET paused_at=NULL WHERE name='checkout_expiry_v1'`)
	bcDue(t, expiry[0].bcHarness, expiry[0].hold)
	mustExec(t, f.owner, `UPDATE river_expiry.river_job SET state='available',scheduled_at=clock_timestamp() WHERE id=$1`, expiry[0].hold.JobID)
	worker := waOpen(t, f, waExpiry, platform.WorkerExpiry) // the current family-bound expiry worker (T21-02: its own authority)
	ewClient(t, worker, 1)
	ewAwait(t, f.owner, expiry[0].hold.JobID, "completed")
	ewAssertOrder(t, expiry[0], "CANCELLED", "EXPIRED", 1)
}

func TestLegacyRuntimeIsolationUpgradeRejectsTamperedLedger(t *testing.T) {
	f := lriPre0032Fixture(t)
	q := pwOldQuerySetupOn(t, f, pwKeys(t), "river")
	if miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE id=$1 AND kind='payment_query_v1'`, q.result.JobID) != 1 {
		t.Fatal("historical payment producer did not admit a job")
	}
	jobs := lriRows(t, f, "river.river_job", "")
	queues := lriRows(t, f, "river.river_queue", "")
	var checksum string
	if err := f.owner.QueryRow(context.Background(), `SELECT checksum FROM public.lc_schema_migrations WHERE version='0031_meta_river_isolation.sql'`).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `UPDATE public.lc_schema_migrations SET checksum='tampered' WHERE version='0031_meta_river_isolation.sql'`)
	if err := lriApply(context.Background(), f.owner); err == nil {
		t.Fatal("current Apply accepted a changed historical checksum")
	}
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0032_legacy_river_isolation.sql'`) != 0 ||
		lriRows(t, f, "river.river_job", "") != jobs || lriRows(t, f, "river.river_queue", "") != queues {
		t.Fatal("checksum rejection mutated old lane or installed preparation")
	}
	mustExec(t, f.owner, `UPDATE public.lc_schema_migrations SET checksum=$1 WHERE version='0031_meta_river_isolation.sql'`, checksum)
	mustExec(t, f.owner, `INSERT INTO public.lc_schema_migrations(version,checksum) VALUES('0031_unknown_local.sql','unknown')`)
	if err := lriApply(context.Background(), f.owner); err == nil {
		t.Fatal("current Apply accepted an unknown historical migration")
	}
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0032_legacy_river_isolation.sql'`) != 0 ||
		lriRows(t, f, "river.river_job", "") != jobs || lriRows(t, f, "river.river_queue", "") != queues {
		t.Fatal("unknown-version rejection mutated old lane or installed preparation")
	}
	mustExec(t, f.owner, `DELETE FROM public.lc_schema_migrations WHERE version='0031_unknown_local.sql'`)
	if err := lriApply(context.Background(), f.owner); err != nil {
		t.Fatal("corrected ledger retry", err)
	}
	lriReady(t, f, true)
}

func TestLegacyRuntimeIsolationUpgradeFailClosedRetry(t *testing.T) {
	f := lriPre0032Fixture(t)
	q := pwOldQuerySetupOn(t, f, pwKeys(t), "river")
	ctx := context.Background()
	failure := func(label string) {
		t.Helper()
		sourceJobs := lriRows(t, f, "river.river_job", "")
		sourceQueues := lriRows(t, f, "river.river_queue", "")
		paymentDest := "[]"
		expiryDest := "[]"
		paymentQueues := "[]"
		expiryQueues := "[]"
		if miCount(t, f.owner, `SELECT count(*) FROM information_schema.tables WHERE table_schema='river_payment' AND table_name='river_job'`) != 0 {
			paymentDest = lriRows(t, f, "river_payment.river_job", "")
			expiryDest = lriRows(t, f, "river_expiry.river_job", "")
			paymentQueues = lriRows(t, f, "river_payment.river_queue", "")
			expiryQueues = lriRows(t, f, "river_expiry.river_queue", "")
		}
		if err := lriApply(ctx, f.owner); err == nil {
			t.Fatalf("%s accepted unsafe cutover", label)
		}
		lriNoPost(t, f, sourceJobs, sourceQueues, paymentDest, expiryDest, paymentQueues, expiryQueues)
	}
	mustExec(t, f.owner, `UPDATE river.river_job SET state='running',attempt=1,attempted_at=clock_timestamp() WHERE id=$1`, q.result.JobID)
	failure("running source")
	if miCount(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0032_legacy_river_isolation.sql'`) != 1 ||
		miCount(t, f.owner, `SELECT count(*) FROM river_payment.river_migration`) == 0 ||
		miCount(t, f.owner, `SELECT count(*) FROM river_expiry.river_migration`) == 0 {
		t.Fatal("first failure did not retain the expected partial native phase")
	}
	mustExec(t, f.owner, `UPDATE river.river_job SET state='available',attempt=0,attempted_at=NULL WHERE id=$1`, q.result.JobID)
	mustExec(t, f.owner, `UPDATE river.river_job SET args='{}'::jsonb WHERE id=$1`, q.result.JobID)
	failure("poison source")
	mustExec(t, f.owner, `UPDATE river.river_job SET args=jsonb_build_object('operation_id',$2::text,'version',1) WHERE id=$1`, q.result.JobID, q.result.OperationID)
	// Native ledgers are already committed from the first attempt; only the
	// post phase rolls back. Direct owner probes are removed before retry.
	var intruder int64
	if err := f.owner.QueryRow(ctx, `INSERT INTO river_payment.river_job(kind,args,queue,max_attempts) VALUES('intruder_v1','{}','default',1) RETURNING id`).Scan(&intruder); err != nil {
		t.Fatal(err)
	}
	failure("nonempty destination")
	mustExec(t, f.owner, `DELETE FROM river_payment.river_job WHERE id=$1`, intruder)
	mustExec(t, f.owner, `INSERT INTO river_payment.river_queue(name) VALUES('conflicting_queue')`)
	failure("conflicting destination queue")
	mustExec(t, f.owner, `DELETE FROM river_payment.river_queue WHERE name='conflicting_queue'`)
	sourceBeforeLock := lriRows(t, f, "river.river_job", "")
	queuesBeforeLock := lriRows(t, f, "river.river_queue", "")
	paymentQueuesBeforeLock := lriRows(t, f, "river_payment.river_queue", "")
	expiryQueuesBeforeLock := lriRows(t, f, "river_expiry.river_queue", "")
	lock, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.Exec(ctx, `LOCK TABLE river.river_job IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	var holderPID int
	if err := lock.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	// Keep migration acquisition independent of the session holding the
	// table lock; otherwise pool starvation can imitate a lock failure.
	migrationPool, err := pgxpool.New(ctx, f.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer migrationPool.Close()
	blocked, stop := context.WithTimeout(ctx, 15*time.Second)
	result := make(chan error, 1)
	go func() { result <- lriApply(blocked, migrationPool) }()
	observed := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_stat_activity a WHERE a.pid<>pg_backend_pid() AND $1=ANY(pg_catalog.pg_blocking_pids(a.pid)))`, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			observed = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	stop()
	err = <-result
	rollbackErr := lock.Rollback(ctx)
	if !observed || err == nil {
		t.Fatalf("cutover did not prove source lock wait: observed=%t apply=%v rollback=%v", observed, err, rollbackErr)
	}
	if rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	// Cancellation closed the attempt's hijacked lock connection client-side,
	// but its backend was still queued on the table lock; it only finishes its
	// statement, aborts and releases session advisory lock 718020260920
	// (migrations/migrate.go) when it exits after the rollback above. Observe
	// that exit instead of retrying Apply, so the no-post check and the retry
	// both run after the cancelled attempt has fully ended.
	waitAdvisoryLockReleased(t, f.owner, "cancelled migration attempt still holds the migration advisory lock", 718020260920)
	lriNoPost(t, f, sourceBeforeLock, queuesBeforeLock, "[]", "[]", paymentQueuesBeforeLock, expiryQueuesBeforeLock)
	if err := lriApply(ctx, f.owner); err != nil {
		t.Fatal("corrected partial-native retry", err)
	}
	lriReady(t, f, true)
	if miCount(t, f.owner, `SELECT count(*) FROM river_payment.river_job WHERE id=$1 AND kind='payment_query_v1'`, q.result.JobID) != 1 ||
		miCount(t, f.owner, `SELECT count(*) FROM river_expiry.river_job WHERE id=$1 AND kind='checkout_expiry_v1'`, q.hold.JobID) != 1 {
		t.Fatal("retry did not move linked jobs exactly once")
	}
}

func TestLegacyRuntimeIsolationFamilyCollidingIDs(t *testing.T) {
	f := lriPre0032Fixture(t)
	if err := lriApply(context.Background(), f.owner); err != nil {
		t.Fatal(err)
	}
	// Actual new-family producers make an expiry row at the payment sequence's
	// next ID before the valid reconciliation transaction commits.
	q := pqSetupItemsOn(t, f, pwKeys(t), false, 1)
	p := ewSetup(t, f, 1)
	lriCloseProducerPools(p)
	claim := q.claim(t)
	if err := pqRecordIn(q.worker, "river_payment", q.result.OperationID, claim, "PROVIDER_MOCK", pcFull(q)); err != nil {
		t.Fatal("valid reconciliation report did not commit", err)
	}
	var reconcile int64
	if err := f.owner.QueryRow(context.Background(), `SELECT id FROM river_payment.river_job WHERE kind='payment_reconcile_v1' AND args->>'operation_id'=$1`, q.result.OperationID).Scan(&reconcile); err != nil {
		t.Fatal(err)
	}
	if reconcile != p.hold.JobID || miCount(t, f.owner, `SELECT count(*) FROM river_expiry.river_job WHERE id=$1 AND kind='checkout_expiry_v1'`, reconcile) != 1 ||
		miCount(t, f.owner, `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1 AND source='QUERY'`, q.result.AttemptID) != 1 {
		t.Fatal("valid report did not commit across equal numeric family IDs")
	}
	lriCloseProducerPools(q.psHarness)
	wrong := pqSetupItemsOn(t, f, pwKeys(t), false, 1)
	wrongOnly := ewSetup(t, f, 1)
	lriCloseProducerPools(wrongOnly)
	if miCount(t, f.owner, `SELECT count(*) FROM river_payment.river_job WHERE id=$1`, wrongOnly.hold.JobID) != 0 ||
		miCount(t, f.owner, `SELECT count(*) FROM river_expiry.river_job WHERE id=$1 AND kind='checkout_expiry_v1'`, wrongOnly.hold.JobID) != 1 {
		t.Fatal("wrong-family-only ID precondition absent")
	}
	wrongClaim := wrong.claim(t)
	if _, err := wrong.worker.Exec(context.Background(), `SELECT integration.record_payment_query($1,$2,$3,$4,$5::jsonb,$6)`, wrong.result.OperationID, wrongClaim.Generation, wrongClaim.LeaseToken, "PROVIDER_MOCK", pqJSON(pcFull(wrong)), wrongOnly.hold.JobID); miSQLState(err) != "PT409" {
		t.Fatalf("expiry-only ID accepted as reconciliation job: state=%s", miSQLState(err))
	}
	if miCount(t, f.owner, `SELECT count(*) FROM payments.provider_observations WHERE attempt_id=$1`, wrong.result.AttemptID) != 0 {
		t.Fatal("wrong-family collision wrote a provider observation")
	}
}

func lriPrunedSequenceFixture(t *testing.T) (*testFixture, pqFixture, psHarness, int64) {
	t.Helper()
	f := lriPre0032Fixture(t)
	q := pwOldQuerySetupOn(t, f, pwKeys(t), "river")
	lriCloseProducerPools(q.psHarness)
	p := ewSetup(t, f, 1, "river")
	lriCloseProducerPools(p)
	ctx := context.Background()
	mustExec(t, f.owner, `UPDATE river.river_job SET state='running',attempt=1,attempted_at=clock_timestamp() WHERE id=$1`, q.result.JobID)
	if err := lriApply(ctx, f.owner); err == nil {
		t.Fatal("native preparation unexpectedly completed while old job running")
	}
	lriReady(t, f, false)
	for _, id := range []int64{q.result.JobID, q.hold.JobID, p.hold.JobID} {
		mustExec(t, f.owner, `UPDATE river.river_job SET state='discarded',finalized_at=clock_timestamp() WHERE id=$1`, id)
		mustExec(t, f.owner, `DELETE FROM river.river_job WHERE id=$1`, id)
	}
	var oldLow int64
	if err := f.owner.QueryRow(ctx, `SELECT setval('river.river_job_id_seq',1,true)`).Scan(&oldLow); err != nil {
		t.Fatal(err)
	}
	return f, q, p, oldLow
}

// A pruned expiry domain reference, rather than the deliberately low old
// sequence, is the expiry maximum; payment's own destination sequence wins.
func TestLegacyRuntimeIsolationUpgradeSequencesDoNotReuseRefs(t *testing.T) {
	f, q, p, oldLow := lriPrunedSequenceFixture(t)
	ctx := context.Background()
	var paymentHigh int64
	if err := f.owner.QueryRow(ctx, `SELECT setval('river_payment.river_job_id_seq',$1::bigint,true)`, q.result.JobID+500).Scan(&paymentHigh); err != nil {
		t.Fatal(err)
	}
	if oldLow >= p.hold.JobID || paymentHigh <= q.result.JobID {
		t.Fatal("test did not establish independent sequence maxima")
	}
	if err := lriApply(ctx, f.owner); err != nil {
		t.Fatal("sequence cutover", err)
	}
	lriReady(t, f, true)
	var oldAfter, paymentNext, expiryNext int64
	if err := f.owner.QueryRow(ctx, `SELECT last_value FROM river.river_job_id_seq`).Scan(&oldAfter); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT nextval('river_payment.river_job_id_seq')`).Scan(&paymentNext); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT nextval('river_expiry.river_job_id_seq')`).Scan(&expiryNext); err != nil {
		t.Fatal(err)
	}
	if oldAfter != oldLow || paymentNext <= paymentHigh || expiryNext <= p.hold.JobID {
		t.Fatalf("sequence nonreuse failed: source=%d/%d payment=%d/%d expiry=%d/%d", oldLow, oldAfter, paymentHigh, paymentNext, p.hold.JobID, expiryNext)
	}
}

// Mirror the maxima: payment's pruned permanent reference is above the old
// sequence, while expiry's own native sequence is independently highest.
func TestLegacyRuntimeIsolationUpgradeMirroredSequenceHighWater(t *testing.T) {
	f, q, p, oldLow := lriPrunedSequenceFixture(t)
	ctx := context.Background()
	var expiryHigh int64
	if err := f.owner.QueryRow(ctx, `SELECT setval('river_expiry.river_job_id_seq',$1::bigint,true)`, p.hold.JobID+500).Scan(&expiryHigh); err != nil {
		t.Fatal(err)
	}
	if oldLow >= q.result.JobID || expiryHigh <= p.hold.JobID {
		t.Fatal("mirrored independent maxima absent")
	}
	if err := lriApply(ctx, f.owner); err != nil {
		t.Fatal("mirrored sequence cutover", err)
	}
	lriReady(t, f, true)
	var oldAfter, paymentNext, expiryNext int64
	if err := f.owner.QueryRow(ctx, `SELECT last_value FROM river.river_job_id_seq`).Scan(&oldAfter); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT nextval('river_payment.river_job_id_seq')`).Scan(&paymentNext); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT nextval('river_expiry.river_job_id_seq')`).Scan(&expiryNext); err != nil {
		t.Fatal(err)
	}
	if oldAfter != oldLow || paymentNext <= q.result.JobID || expiryNext <= expiryHigh {
		t.Fatalf("mirrored sequence nonreuse failed: source=%d/%d payment=%d/%d expiry=%d/%d", oldLow, oldAfter, q.result.JobID, paymentNext, expiryHigh, expiryNext)
	}
}

func TestLegacyRuntimeIsolationUpgradeCopiedRowsAboveOldSequence(t *testing.T) {
	f := lriPre0032Fixture(t)
	q := pwOldQuerySetupOn(t, f, pwKeys(t), "river")
	lriCloseProducerPools(q.psHarness)
	p := ewSetup(t, f, 1, "river")
	lriCloseProducerPools(p)
	ctx := context.Background()
	var oldLow int64
	if err := f.owner.QueryRow(ctx, `SELECT setval('river.river_job_id_seq',1,true)`).Scan(&oldLow); err != nil {
		t.Fatal(err)
	}
	if q.result.JobID <= oldLow || p.hold.JobID <= oldLow {
		t.Fatal("copied-row-above-source-sequence precondition absent")
	}
	oldPayment := lriRows(t, f, "river.river_job", `WHERE kind='payment_query_v1'`)
	oldExpiry := lriRows(t, f, "river.river_job", `WHERE kind='checkout_expiry_v1'`)
	if err := lriApply(ctx, f.owner); err != nil {
		t.Fatal("copied-row sequence cutover", err)
	}
	if lriRows(t, f, "river_payment.river_job", "") != oldPayment || lriRows(t, f, "river_expiry.river_job", "") != oldExpiry {
		t.Fatal("copied rows changed or disappeared")
	}
	var oldAfter, paymentNext, expiryNext int64
	if err := f.owner.QueryRow(ctx, `SELECT last_value FROM river.river_job_id_seq`).Scan(&oldAfter); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT nextval('river_payment.river_job_id_seq')`).Scan(&paymentNext); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.QueryRow(ctx, `SELECT nextval('river_expiry.river_job_id_seq')`).Scan(&expiryNext); err != nil {
		t.Fatal(err)
	}
	if oldAfter != oldLow || paymentNext <= q.result.JobID || expiryNext <= p.hold.JobID {
		t.Fatalf("copied-row high-water reused: source=%d/%d payment=%d/%d expiry=%d/%d", oldLow, oldAfter, q.result.JobID, paymentNext, p.hold.JobID, expiryNext)
	}
}
