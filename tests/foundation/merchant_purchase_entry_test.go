package foundation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/catalog"
	"livecommerce/internal/command"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/platform"
)

func mpeRead(ctx context.Context, tf adminTransportFixture, product, locale string) (catalog.PurchaseEntry, error) {
	return t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (catalog.PurchaseEntry, error) {
		return catalog.ReadPurchaseEntry(ctx, tx, scope, tf.token, product, locale)
	})
}

func mpePublished(t *testing.T, tf adminTransportFixture) string {
	t.Helper()
	origin := "https://entry-" + strings.ReplaceAll(tf.store, "-", "") + ".test"
	publishedFixture(t, tf.f, tf.tenant, tf.store, origin)
	return origin
}

func mpeExtraDomain(t *testing.T, tf adminTransportFixture, origin, state string) {
	t.Helper()
	mustExec(t, tf.f.owner, `INSERT INTO control.storefront_domains
		(tenant_id,store_id,origin,state,ownership_verified_at,tls_verified_at,valid_until,evidence_ref)
		VALUES($1,$2,$3,$4,clock_timestamp()-interval '1 hour',clock_timestamp()-interval '1 hour',
		clock_timestamp()+interval '1 hour','SYNTHETIC_LOCAL_TEST')`, tf.tenant, tf.store, origin, state)
}

func TestMerchantPurchaseEntryStatesAndIsolation(t *testing.T) {
	tf := newAdminTransportFixture(t)
	ctx := context.Background()
	assertState := func(product, state string) {
		t.Helper()
		out, err := mpeRead(ctx, tf, product, "zh-TW")
		if err != nil || out.State != state || (state != "configured" && out.URL != "") {
			t.Fatalf("want %s: %+v %v", state, out, err)
		}
	}
	assertState(tf.parent, "storefront_unavailable")
	origin := mpePublished(t, tf)
	for _, locale := range []string{"zh-CN", "zh-TW", "en"} {
		out, err := mpeRead(ctx, tf, tf.parent, locale)
		if err != nil || out != (catalog.PurchaseEntry{ProductID: tf.parent, Locale: locale, State: "configured", URL: origin + "/" + locale + "/products/" + tf.parent}) {
			t.Fatalf("locale projection: %+v %v", out, err)
		}
	}
	assertState(tf.otherParent, "no_active_sku")
	for _, product := range []string{randomUUID(), newAdminTransportFixture(t).parent} {
		if _, err := mpeRead(ctx, tf, product, "en"); !errors.Is(err, command.ErrNotFound) {
			t.Fatalf("unknown/foreign product: %v", err)
		}
	}
	mustExec(t, tf.f.owner, `UPDATE catalog.products SET status='archived' WHERE id=$1`, tf.parent)
	assertState(tf.parent, "product_inactive")
	mustExec(t, tf.f.owner, `UPDATE catalog.products SET status='active' WHERE id=$1`, tf.parent)
	mustExec(t, tf.f.owner, `UPDATE catalog.skus SET status='archived' WHERE product_id=$1`, tf.parent)
	assertState(tf.parent, "no_active_sku")
	// A zero price is valid catalog data, not a claim that a PSP can collect it.
	mustExec(t, tf.f.owner, `UPDATE catalog.skus SET status='active',price_minor=0 WHERE id=$1`, tf.skuIDs[0])
	assertState(tf.parent, "configured")
	mustExec(t, tf.f.owner, `UPDATE control.stores SET currency='TWD' WHERE id=$1`, tf.store)
	assertState(tf.parent, "no_active_sku")
	mustExec(t, tf.f.owner, `UPDATE control.stores SET currency='USD' WHERE id=$1`, tf.store)
	for _, state := range []string{"REQUESTED", "OWNERSHIP_PENDING", "TLS_PENDING", "SUSPENDED", "DETACHED"} {
		mustExec(t, tf.f.owner, `UPDATE control.storefront_domains SET state=$2 WHERE origin=$1`, origin, state)
		assertState(tf.parent, "storefront_unavailable")
	}
	mustExec(t, tf.f.owner, `UPDATE control.storefront_domains SET state='ACTIVE' WHERE origin=$1`, origin)
	for _, mutation := range []string{
		`ownership_verified_at=clock_timestamp()+interval '30 minutes'`,
		`tls_verified_at=clock_timestamp()+interval '30 minutes'`,
		`valid_until=clock_timestamp()-interval '1 second'`,
	} {
		mustExec(t, tf.f.owner, `UPDATE control.storefront_domains SET `+mutation+` WHERE origin=$1`, origin)
		assertState(tf.parent, "storefront_unavailable")
		mustExec(t, tf.f.owner, `UPDATE control.storefront_domains SET ownership_verified_at=clock_timestamp()-interval '2 hours',
			tls_verified_at=clock_timestamp()-interval '2 hours',valid_until=clock_timestamp()+interval '1 hour' WHERE origin=$1`, origin)
	}
	mustExec(t, tf.f.owner, `UPDATE control.storefront_publications SET published=false WHERE store_id=$1`, tf.store)
	assertState(tf.parent, "storefront_unavailable")
	mustExec(t, tf.f.owner, `UPDATE control.storefront_publications SET published=true WHERE store_id=$1`, tf.store)
	assertState(tf.parent, "configured")
	mustExec(t, tf.f.owner, `UPDATE control.stores SET active=false WHERE id=$1`, tf.store)
	if _, err := mpeRead(ctx, tf, tf.parent, "en"); !errors.Is(err, platform.ErrScopeNotFound) {
		t.Fatalf("inactive store: %v", err)
	}
	mustExec(t, tf.f.owner, `UPDATE control.stores SET active=true WHERE id=$1`, tf.store)
	mustExec(t, tf.f.owner, `UPDATE control.tenants SET active=false WHERE id=$1`, tf.tenant)
	_, inactiveTenantErr := mpeRead(ctx, tf, tf.parent, "en")
	mustExec(t, tf.f.owner, `UPDATE control.tenants SET active=true WHERE id=$1`, tf.tenant)
	if !errors.Is(inactiveTenantErr, platform.ErrScopeNotFound) {
		t.Fatalf("inactive tenant: %v", inactiveTenantErr)
	}
	// Ineligible rows sort first. LIMIT must apply after all eligibility filters.
	mpeExtraDomain(t, tf, "https://a-"+tf.store+".test", "SUSPENDED")
	mpeExtraDomain(t, tf, "https://b-"+tf.store+".test", "DETACHED")
	assertState(tf.parent, "configured")
	mpeExtraDomain(t, tf, "https://z-"+tf.store+".test", "ACTIVE")
	assertState(tf.parent, "domain_selection_required")
}

func TestMerchantPurchaseEntryAuthorityAndACL(t *testing.T) {
	tf := newAdminTransportFixture(t)
	mpePublished(t, tf)
	ctx := context.Background()
	var owner, volatility string
	var secure bool
	if err := tf.f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(proowner),provolatile::text,prosecdef
		FROM pg_proc WHERE oid='identity.resolve_storefront_origin(bytea,uuid)'::regprocedure`).Scan(&owner, &volatility, &secure); err != nil || owner != "commerce_auth" || volatility != "v" || !secure {
		t.Fatalf("function ownership/volatility: %s %s %v %v", owner, volatility, secure, err)
	}
	var restricted bool
	if err := tf.f.owner.QueryRow(ctx, `SELECT p.proconfig=ARRAY['search_path=pg_catalog']::text[]
		AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolbypassrls
		AND NOT EXISTS (SELECT 1 FROM aclexplode(p.proacl) a WHERE a.privilege_type='EXECUTE'
		AND a.grantee NOT IN ('commerce_auth'::regrole,'commerce_runtime'::regrole))
		FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner
		WHERE p.oid='identity.resolve_storefront_origin(bytea,uuid)'::regprocedure`).Scan(&restricted); err != nil || !restricted {
		t.Fatalf("search_path/owner/PUBLIC execute boundary: %v", err)
	}
	for _, role := range []string{"commerce_runtime", "commerce_buyer_runtime", "commerce_buyer_issuer", "commerce_checkout_runtime", waPayment, waLive, waExpiry, waAds, waClaims, waLegacy, "commerce_identity"} {
		var execute, tables, member bool
		if err := tf.f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,'identity.resolve_storefront_origin(bytea,uuid)','EXECUTE'),
			has_any_column_privilege($1,'control.storefront_domains','SELECT') OR has_any_column_privilege($1,'control.storefront_publications','SELECT'),
			pg_has_role($1,'commerce_auth','MEMBER')`, role).Scan(&execute, &tables, &member); err != nil || execute != (role == "commerce_runtime") || tables || member {
			t.Fatalf("role %s widened: execute=%v tables=%v member=%v err=%v", role, execute, tables, member, err)
		}
	}
	for _, table := range []string{"control.storefront_domains", "control.storefront_publications"} {
		var forced, writes bool
		if err := tf.f.owner.QueryRow(ctx, `SELECT relrowsecurity AND relforcerowsecurity,
			has_table_privilege('commerce_auth',oid,'INSERT,UPDATE,DELETE,TRUNCATE') FROM pg_class WHERE oid=$1::regclass`, table).Scan(&forced, &writes); err != nil || !forced || writes {
			t.Fatalf("RLS/write boundary %s: %v %v %v", table, forced, writes, err)
		}
	}
	for _, tc := range []struct {
		name, code string
		hash       []byte
		store      string
		guc        string
		iso        pgx.TxIsoLevel
	}{
		{"bad hash", "PT400", []byte{1}, tf.store, "", pgx.ReadCommitted},
		{"unknown token", "PT401", tokenHash(randomToken()), tf.store, "", pgx.ReadCommitted},
		{"foreign store", "PT404", tokenHash(tf.token), tf.f.storeB, "", pgx.ReadCommitted},
		{"missing GUC", "PT403", tokenHash(tf.token), tf.store, "", pgx.ReadCommitted},
		{"malformed GUC", "PT403", tokenHash(tf.token), tf.store, "malformed", pgx.ReadCommitted},
		{"repeatable read", "PT503", tokenHash(tf.token), tf.store, "", pgx.RepeatableRead},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := tf.f.runtime.BeginTx(ctx, pgx.TxOptions{IsoLevel: tc.iso})
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$1,true),set_config('app.principal_id',$1,true)`, tc.guc); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, `SELECT * FROM identity.resolve_storefront_origin($1,$2)`, tc.hash, tc.store)
			requirePGCode(t, err, tc.code, tc.name)
		})
	}
	// Authorization succeeds first, then changes before the kernel is called.
	for _, change := range []string{"permission", "session", "store"} {
		token := ledgerPermissionToken(t, tf, "catalog:read")
		err := platform.WithScope(ctx, tf.f.runtime, token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) error {
			if change == "session" {
				mustExec(t, tf.f.owner, `UPDATE identity.sessions SET revoked_at=clock_timestamp() WHERE token_hash=$1`, tokenHash(token))
			} else if change == "store" {
				mustExec(t, tf.f.owner, `DELETE FROM identity.store_grants WHERE principal_id=$1 AND permission='store:read'`, scope.PrincipalID)
			} else {
				mustExec(t, tf.f.owner, `DELETE FROM identity.store_grants WHERE principal_id=$1 AND permission='catalog:read'`, scope.PrincipalID)
			}
			_, err := catalog.ReadPurchaseEntry(ctx, tx, scope, token, tf.parent, "en")
			return err
		})
		want := platform.ErrForbidden
		if change == "session" {
			want = platform.ErrUnauthorized
		} else if change == "store" {
			want = platform.ErrScopeNotFound
		}
		if !errors.Is(err, want) {
			t.Fatalf("causal reauthorization: %v want %v", err, want)
		}
	}
	for _, mutation := range []string{"expires_at=clock_timestamp()-interval '1 second'", "audience='buyer'"} {
		token := ledgerPermissionToken(t, tf, "catalog:read")
		mustExec(t, tf.f.owner, `UPDATE identity.sessions SET `+mutation+` WHERE token_hash=$1`, tokenHash(token))
		err := platform.WithScope(ctx, tf.f.runtime, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) error {
			_, err := catalog.ReadPurchaseEntry(ctx, tx, scope, token, tf.parent, "en")
			return err
		})
		if !errors.Is(err, platform.ErrUnauthorized) {
			t.Fatalf("expired/wrong audience: %v", err)
		}
	}
	for _, guc := range []string{"app.tenant_id", "app.store_id", "app.principal_id"} {
		err := platform.WithScope(ctx, tf.f.runtime, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) error {
			if _, err := tx.Exec(ctx, `SELECT set_config($1,$2,true)`, guc, randomUUID()); err != nil {
				return err
			}
			_, err := catalog.ReadPurchaseEntry(ctx, tx, scope, tf.token, tf.parent, "en")
			return err
		})
		if !errors.Is(err, platform.ErrForbidden) {
			t.Fatalf("forged %s: %v", guc, err)
		}
	}
	tx, err := tf.f.runtime.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = catalog.ReadPurchaseEntry(ctx, tx, platform.Scope{TenantID: tf.tenant, StoreID: tf.store, PrincipalID: tf.principal}, tf.token, tf.parent, "en")
	if !errors.Is(err, catalog.ErrPurchaseEntryUnavailable) {
		t.Fatalf("non-RC Go sentinel: %v", err)
	}
}

func TestMerchantPurchaseEntryClockAfterObservedLock(t *testing.T) {
	for _, multiple := range []bool{false, true} {
		t.Run(map[bool]string{false: "sole expires", true: "ambiguity survives expiry"}[multiple], func(t *testing.T) {
			tf := newAdminTransportFixture(t)
			origin := mpePublished(t, tf)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			if multiple {
				mpeExtraDomain(t, tf, "https://z-"+tf.store+".test", "ACTIVE")
			}
			mustExec(t, tf.f.owner, `UPDATE control.storefront_domains SET valid_until=clock_timestamp()+interval '750 milliseconds' WHERE origin=$1`, origin)
			if _, err := mpeRead(ctx, tf, tf.parent, "en"); err != nil {
				t.Fatal(err)
			}
			holder, err := tf.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback(context.Background())
			if _, err = holder.Exec(ctx, `LOCK control.storefront_domains IN ACCESS EXCLUSIVE MODE`); err != nil {
				t.Fatal(err)
			}
			type result struct {
				entry catalog.PurchaseEntry
				err   error
			}
			done, pids := make(chan result, 1), make(chan int, 1)
			go func() {
				out, err := t04Scoped(ctx, tf.f, tf.token, tf.store, "catalog:read", func(tx pgx.Tx, scope platform.Scope) (catalog.PurchaseEntry, error) {
					var pid int
					if err := tx.QueryRow(ctx, `SELECT pg_backend_pid() FROM set_config('lock_timeout','0',true)`).Scan(&pid); err != nil {
						return catalog.PurchaseEntry{}, err
					}
					pids <- pid
					return catalog.ReadPurchaseEntry(ctx, tx, scope, tf.token, tf.parent, "en")
				})
				done <- result{out, err}
			}()
			var pid int
			select {
			case pid = <-pids:
			case r := <-done:
				t.Fatalf("reader stopped early: %v", r.err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			for {
				var waiting bool
				if err = tf.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock' AND wait_event='relation')`, pid).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case r := <-done:
					t.Fatalf("no observed wait: %v", r.err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(10 * time.Millisecond):
				}
			}
			for {
				var expired bool
				if err = holder.QueryRow(ctx, `SELECT valid_until<=clock_timestamp() FROM control.storefront_domains WHERE origin=$1`, origin).Scan(&expired); err != nil {
					t.Fatal(err)
				}
				if expired {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(10 * time.Millisecond):
				}
			}
			if err = holder.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			want := "storefront_unavailable"
			if multiple {
				want = "domain_selection_required"
			}
			select {
			case r := <-done:
				if r.err != nil || r.entry.State != want || r.entry.URL != "" {
					t.Fatalf("clock result: %+v %v want %s", r.entry, r.err, want)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}

func TestMerchantPurchaseEntryHTTPReplayAndNoPurchaseEffects(t *testing.T) {
	tf := newAdminTransportFixture(t)
	origin := mpePublished(t, tf)
	h := httpapi.NewHandler(tf.f.runtime)
	base := "/v1/admin/stores/" + tf.store
	request := func(method, path, token, key string, body []byte, status int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-Host", "attacker.invalid")
		r.Host = "attacker.invalid"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("HTTP %s status %d want %d: %s", path, w.Code, status, w.Body.String())
		}
		return w.Body.Bytes()
	}
	key := t04Key("entry-create")
	body := []byte(`{"name":"Entry HTTP product","description":"Isolated fixture"}`)
	created := request("POST", base+"/products", tf.token, key, body, 200)
	var product catalog.Product
	if err := json.Unmarshal(created, &product); err != nil {
		t.Fatal(err)
	}
	skuBody, _ := json.Marshal(catalog.SKUInput{ProductID: product.ID, Code: "ENTRY-" + t04Tag(), PriceMinor: 1000, WeightGrams: 10, OriginCountry: "CN", CustomsName: "fixture"})
	skuKey := t04Key("entry-sku")
	skuCreated := request("POST", base+"/skus", tf.token, skuKey, skuBody, 200)
	var sku catalog.SKU
	if err := json.Unmarshal(skuCreated, &sku); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, table := range []string{"buyer.owners", "buyer.capability_sessions", "buyer.capability_events", "buyer.command_results", "storefront.carts", "storefront.quotes", "storefront.events", "checkout.orders", "checkout.command_results", "checkout.payment_attempts", "integration.operations", "integration.operation_events", "inventory.reservations", "ops.command_results"} {
		counts[table] = countRows(t, tf.f.owner, "SELECT count(*) FROM "+table)
	}
	path := base + "/products/" + product.ID + "/purchase-entry"
	for _, locale := range []string{"zh-CN", "zh-TW", "en"} {
		var fields map[string]any
		if err := json.Unmarshal(request("GET", path+"?locale="+locale, tf.token, "", nil, 200), &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 4 || fields["url"] != origin+"/"+locale+"/products/"+product.ID || fields["state"] != "configured" || fields["locale"] != locale || fields["product_id"] != product.ID {
			t.Fatalf("bad projection: %+v", fields)
		}
	}
	for _, query := range []string{"", "?locale=", "?locale=fr", "?locale=en&locale=en", "?locale=en&extra=x", "?locale=%zz", "?locale=en;x=1", "?locale=" + strings.Repeat("a", 65)} {
		request("GET", path+query, tf.token, "", nil, 422)
	}
	for _, method := range []string{"POST", "PATCH", "DELETE", "HEAD", "PUT"} {
		request(method, path+"?locale=en", tf.token, "", nil, 405)
	}
	request("GET", path+"?locale=en", tf.token, "", []byte(`{}`), 422)
	request("GET", path+"?locale=en", tf.token, "read-key", nil, 422)
	request("GET", path+"?locale=en", randomToken(), "", nil, 401)
	request("GET", path+"?locale=en", ledgerPermissionToken(t, tf), "", nil, 403)
	if replay := request("POST", base+"/products", tf.token, key, body, 200); !bytes.Equal(created, replay) {
		t.Fatal("product receipt changed")
	}
	if replay := request("POST", base+"/skus", tf.token, skuKey, skuBody, 200); !bytes.Equal(skuCreated, replay) {
		t.Fatal("SKU receipt changed")
	}
	for table, before := range counts {
		if after := countRows(t, tf.f.owner, "SELECT count(*) FROM "+table); before != after {
			t.Fatalf("unexpected %s effects %d->%d", table, before, after)
		}
	}
	priceBody, _ := json.Marshal(catalog.PriceInput{PriceMinor: 2000, ExpectedVersion: sku.Version})
	request("POST", base+"/skus/"+sku.ID+"/price", tf.token, t04Key("entry-price"), priceBody, 200)
	if out, err := mpeRead(context.Background(), tf, product.ID, "en"); err != nil || out.URL != origin+"/en/products/"+product.ID {
		t.Fatalf("price changed entry: %+v %v", out, err)
	}
	if replay := request("POST", base+"/skus", tf.token, skuKey, skuBody, 200); !bytes.Equal(skuCreated, replay) {
		t.Fatal("updated price rewrote original SKU receipt")
	}
}
