package foundation_test

// storefront_integration_smoke_test.go: author smoke of unit storefront-integration (migration 0093) on real PostgreSQL through the real
// buyerhttp handler: SI01 the CVS return-path allowlist and Buy now cart binding (Go and SQL),
// SI02 free_shipping_threshold_minor on every checkout-options row, SI03 the collection id on both catalog/v2 collection reads. Evidence label:
// SANDBOX (isolated disposable PG, synthetic published origin). The browser side is TestBrowserStorefront and the ported buyer gates.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/storefront"
)

func TestStorefrontIntegrationSmoke(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.connect("C2C")
	code, _, _ := e.service("cvs_711", "API", 0)

	t.Run("SI01 CVS return paths and exact Buy now cart binding", func(t *testing.T) {
		good := []string{"/en/checkout", "/zh-TW/checkout", "/zh-CN/checkout", "/en/products/prod_1", "/zh-TW/products/x", "/zh-TW/products/" + strings.Repeat("a", 64)}
		bad := []string{"/en/checkout/", "/en/checkout/x", "/en/Checkout", "/en/CHECKOUT", "/fr/checkout", "/en/checkout?x=1", "/en/checkout#f", "//evil.example/en/checkout",
			"https://evil.example/en/checkout", "en/checkout", "/en/products/Checkout/x", "/en/products/", "/en/claim", "/en/cart"}
		b := e.newBuyer()
		newSelection := func(b *tcvBuyer) string {
			t.Helper()
			res := b.req("POST", "/v1/buyer/buy-now", t04Key("si01-buy-now"), storefront.BuyNowInput{
				SKUID: e.p.stock.skus[0].ID, Quantity: 1,
			}, nil)
			var cart storefront.Cart
			if err := json.Unmarshal(res.body, &cart); res.status != 200 || err != nil || cart.ID == "" || cart.Version != 1 {
				t.Fatalf("create Buy now cart: HTTP %d, %s (%v)", res.status, res.body, err)
			}
			return cart.ID
		}
		regular, direct, other := b.h.destination.CartID, newSelection(b), newSelection(b)
		foreign := newSelection(e.newBuyer())
		const directPath = "/en/checkout/buy-now/"
		bad = append(bad, "/en/checkout/buy-now", directPath+"bad-id", directPath+direct+"/", directPath+direct+"?x=1", directPath+direct+"#f")

		// The CHECK enforces path shape. The Go entry and definer also enforce
		// cart binding, so a well-formed path can still be refused by both.
		type pathCase struct {
			name, path, selector, cart, code, sqlState string
			status                                     int
			validShape                                 bool
		}
		var cases []pathCase
		for _, path := range good {
			cases = append(cases, pathCase{name: path, path: path, cart: regular, status: 201, validShape: true})
		}
		for _, path := range bad {
			cases = append(cases, pathCase{name: path, path: path, status: 422, code: "bad_return_path", sqlState: "PT422"})
		}
		for _, locale := range []string{"en", "zh-TW", "zh-CN"} {
			path := "/" + locale + "/checkout/buy-now/" + direct
			cases = append(cases, pathCase{name: locale + " matched Buy now", path: path, selector: direct, cart: direct, status: 201, validShape: true})
		}
		cases = append(cases,
			pathCase{name: "no selector", path: directPath + direct, status: 422, code: "bad_return_path", sqlState: "PT422", validShape: true},
			pathCase{name: "other owned cart path", path: directPath + other, selector: direct, status: 422, code: "bad_return_path", sqlState: "PT422", validShape: true},
			pathCase{name: "ordinary path with selector", path: "/en/checkout", selector: direct, status: 422, code: "bad_return_path", sqlState: "PT422", validShape: true},
			pathCase{name: "ordinary cart as Buy now", path: directPath + regular, selector: regular, status: 409, code: "conflict", sqlState: "PT409", validShape: true},
			pathCase{name: "other buyer's cart", path: directPath + foreign, selector: foreign, status: 409, code: "conflict", sqlState: "PT409", validShape: true},
		)

		assertSelection := func(t *testing.T, id string, tc pathCase) {
			t.Helper()
			var cart, path string
			err := f.owner.QueryRow(ctx, `SELECT cart_id::text,return_path FROM fulfillment.cvs_selections WHERE id=$1 AND store_id=$2 AND owner_id=$3`,
				id, f.storeA1, b.cap.Scope.OwnerID).Scan(&cart, &path)
			if err != nil || cart != tc.cart || path != tc.path {
				t.Fatalf("saved selection cart=%q path=%q, want %q %q: %v", cart, path, tc.cart, tc.path, err)
			}
		}
		var sampleID string
		var accepted int64
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				in := checkout.SelectionOpenInput{BuyNowID: tc.selector, CartVersion: b.cartVersion(), MarketID: e.p.market.ID, ServiceCode: code, ReturnPath: tc.path}
				res := b.req("POST", "/v1/buyer/cvs-selections", t04Key("si01-http"), in, nil)
				if res.status != tc.status {
					t.Fatalf("HTTP %d %s, want %d", res.status, res.body, tc.status)
				}
				body := tcvJSON(t, res.body)
				if tc.sqlState == "" {
					id := tcvStr(body, "selection_id")
					assertSelection(t, id, tc)
					if sampleID == "" {
						sampleID = id
					}
					w := e.mapReturn(id, tcvFormFields(body))
					if want := e.origin + tc.path + "?cvs_selection=" + id; w.Code != http.StatusSeeOther || w.Header().Get("Location") != want {
						t.Errorf("map return: %d %q want %q", w.Code, w.Header().Get("Location"), want)
					}
				} else if tcvStr(body, "code") != tc.code {
					t.Errorf("HTTP %d code in %s, want %s", res.status, res.body, tc.code)
				}

				// Call the definer directly under the buyer's checkout pool.
				hash := sha256.Sum256([]byte(b.cap.Token))
				nonce, req := sha256.Sum256([]byte("nonce"+tc.path)), sha256.Sum256([]byte("req"+tc.path))
				var id string
				err := buyer.WithScope(ctx, e.p.pool, b.cap.Token, f.storeA1, func(c context.Context, tx pgx.Tx, _ buyer.Scope) error {
					if _, err := tx.Exec(c, `SELECT set_config('app.buy_now_cart_id',$1,true)`, tc.selector); err != nil {
						return err
					}
					return tx.QueryRow(c, `SELECT fulfillment.open_cvs_selection($1,$2::uuid,$3,$4,$5::bigint,$6::uuid,$7,$8,$9,$10,$11)->>'selection_id'`,
						hash[:], f.storeA1, t04Key("si01-sql"), req[:], b.cartVersion(), e.p.market.ID, code, nonce[:], e.origin, tc.path, "SANDBOX").Scan(&id)
				})
				if tc.sqlState != "" {
					requirePGCode(t, err, tc.sqlState, "SQL open_cvs_selection")
					return
				}
				if err != nil {
					t.Fatalf("SQL open_cvs_selection: %v", err)
				}
				assertSelection(t, id, tc)
				accepted++
			})
		}
		if got := miCount(t, f.owner, `SELECT count(*) FROM fulfillment.cvs_selections WHERE store_id=$1 AND owner_id=$2`, f.storeA1, b.cap.Scope.OwnerID); got != 2*accepted {
			t.Errorf("HTTP and SQL created %d selections, want %d", got, 2*accepted)
		}
		if sampleID == "" {
			t.Fatal("no real selection for the CHECK probes")
		}
		for _, tc := range cases {
			t.Run("CHECK "+tc.name, func(t *testing.T) {
				tx, err := f.owner.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				result, err := tx.Exec(ctx, `UPDATE fulfillment.cvs_selections SET return_path=$1 WHERE id=$2`, tc.path, sampleID)
				if tc.validShape {
					if err != nil || result.RowsAffected() != 1 {
						t.Fatalf("return_path CHECK rejected %q: %v", tc.path, err)
					}
					return
				}
				var pg *pgconn.PgError
				if !errors.As(err, &pg) || pg.Code != "23514" || pg.ConstraintName != "cvs_selections_return_path_check" {
					t.Fatalf("return_path CHECK did not reject %q: %v", tc.path, err)
				}
			})
		}
	})

	t.Run("SI02 every checkout-options row carries free_shipping_threshold_minor (number or null, 0 reads as null)", func(t *testing.T) {
		homeCode := code // the cvs_711 API service of this environment: its policy is the row under test
		setThreshold := func(threshold *int64) {
			t.Helper()
			var version int64
			if err := f.owner.QueryRow(ctx, `SELECT current_version FROM pricing.policy_heads WHERE tenant_id=$1 AND store_id=$2 AND method=$3`, f.tenantA, f.storeA1, "delivery:"+homeCode).Scan(&version); err != nil {
				t.Fatal(err)
			}
			p := e.p.policy
			fee := int64(600)
			p.Method, p.ExpectedVersion, p.ShippingMinor, p.Enabled, p.FreeShippingThresholdMinor = "delivery:"+homeCode, version, &fee, true, threshold
			policy, err := e.p.setPolicy(p)
			if err != nil {
				t.Fatalf("set policy: %v", err)
			}
			// the service is pinned to a policy version (options read the pinned version): re-pin it to the new head
			service := e.svcs[homeCode]
			service.ExpectedVersion, service.PolicyVersion = e.svcVer[homeCode], policy.Version
			if got, err := dsSet(e.p.cqHarness, t04Key("si02-service"), service); err != nil {
				t.Fatalf("re-pin service: %v", err)
			} else {
				e.svcVer[homeCode] = got.Version
			}
		}
		rows := func() map[string]any {
			t.Helper()
			res := e.newBuyer().req("GET", "/v1/buyer/checkout-options?market_id="+e.p.market.ID+"&country=TW", "", nil, nil)
			if res.status != 200 {
				t.Fatalf("options: %d %s", res.status, res.body)
			}
			var found map[string]any
			items, _ := tcvJSON(t, res.body)["items"].([]any)
			for _, it := range items {
				row := it.(map[string]any)
				if _, ok := row["free_shipping_threshold_minor"]; !ok {
					t.Fatalf("a row without the key: %v", row)
				}
				if row["delivery_code"] == homeCode {
					found = row
				}
			}
			if found == nil {
				t.Fatalf("the home delivery row is missing: %s", res.body)
			}
			return found
		}
		if v := rows()["free_shipping_threshold_minor"]; v != nil {
			t.Errorf("no threshold must read null, got %v", v)
		}
		th := int64(150000)
		setThreshold(&th)
		if v := rows()["free_shipping_threshold_minor"]; v != float64(150000) {
			t.Errorf("threshold 150000 must read 150000, got %v", v)
		}
		zero := int64(0)
		setThreshold(&zero)
		if v := rows()["free_shipping_threshold_minor"]; v != nil {
			t.Errorf("a zero threshold (always free) must read null, got %v", v)
		}
	})

	t.Run("SI03 collection reads carry the collection id", func(t *testing.T) {
		h := e.bh
		a := v2Admin{t: t, h: httpapi.NewHandler(f.runtime), token: f.tokens["a"], store: f.storeA1}
		var made struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		}
		a.do("POST", "/collections", t04Key("si03-col"), map[string]any{"title": "SI03 " + t04Tag(), "description": "ids"}, 200, &made)
		t.Cleanup(func() { mustExec(t, f.owner, `UPDATE catalog.collections SET status='hidden' WHERE id=$1`, made.ID) })
		list := tcvJSON(t, h.request(t, "GET", "/v1/buyer/catalog/v2/collections", "", "", nil, nil).body)
		var seen bool
		for _, it := range list["collections"].([]any) {
			if row := it.(map[string]any); row["slug"] == made.Slug {
				seen = row["id"] == made.ID
			}
		}
		if !seen {
			t.Errorf("catalog/v2/collections must return id=%s for %s: %v", made.ID, made.Slug, list)
		}
		one := tcvJSON(t, h.request(t, "GET", "/v1/buyer/catalog/v2/collections/"+made.Slug, "", "", nil, nil).body)
		if one["id"] != made.ID {
			t.Errorf("catalog/v2/collections/{slug} must return id=%s: %v", made.ID, one)
		}
	})

	t.Run("SI04 no business-schema function is executable by PUBLIC (the restricted-pool authority scan depends on it)", func(t *testing.T) {
		rows, err := f.owner.Query(ctx, `SELECT n.nspname||'.'||p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
			WHERE n.nspname NOT IN ('information_schema','pg_catalog') AND n.nspname NOT LIKE 'pg\_%' AND n.nspname NOT LIKE 'river%'
			AND (p.proacl IS NULL OR has_function_privilege(0::oid,p.oid,'EXECUTE')) ORDER BY 1`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var fn string
			_ = rows.Scan(&fn)
			t.Errorf("%s is executable by PUBLIC: validateStripeAuthority would refuse the Stripe registrar login (REVOKE ALL ... FROM PUBLIC in its migration)", fn)
		}
	})
}
