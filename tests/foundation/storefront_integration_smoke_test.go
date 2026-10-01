package foundation_test

// storefront_integration_smoke_test.go: author smoke of unit storefront-integration (migration 0093) on real PostgreSQL through the real
// buyerhttp handler: SI01 the widened CVS map return-path allowlist (Go AND SQL, exactly /{locale}/checkout in addition to the product form),
// SI02 free_shipping_threshold_minor on every checkout-options row, SI03 the collection id on both catalog/v2 collection reads. Evidence label:
// SANDBOX (isolated disposable PG, synthetic published origin). The browser side is TestBrowserStorefront and the ported buyer gates.

import (
	"context"
	"crypto/sha256"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/httpapi"
)

func TestStorefrontIntegrationSmoke(t *testing.T) {
	e := tcvNew(t)
	f := e.p.f
	ctx := context.Background()
	e.connect("C2C")
	code, _, _ := e.service("cvs_711", "API", 0)

	t.Run("SI01 return path allowlist: /{locale}/checkout in Go and SQL, every near miss refused", func(t *testing.T) {
		good := []string{"/en/checkout", "/zh-TW/checkout", "/zh-CN/checkout", "/en/products/prod_1", "/zh-TW/products/" + strings.Repeat("a", 64)}
		bad := []string{"/en/checkout/", "/en/checkout/x", "/en/Checkout", "/en/CHECKOUT", "/fr/checkout", "/en/checkout?x=1", "/en/checkout#f", "//evil.example/en/checkout",
			"https://evil.example/en/checkout", "en/checkout", "/en/products/Checkout/x", "/en/products/", "/en/claim", "/en/cart"}
		for _, path := range good {
			b := e.newBuyer()
			sel, res := e.tclOpen(b, code, path, nil)
			if sel.ID == "" {
				t.Errorf("return_path %q must be accepted: %d %s", path, res.status, res.body)
				continue
			}
			// The stored path is what the unauthenticated map return redirects to: origin + stored path, never a body value.
			w := e.mapReturn(sel.ID, sel.Fields)
			if want := e.origin + path + "?cvs_selection=" + sel.ID; w.Code != http.StatusSeeOther || w.Header().Get("Location") != want {
				t.Errorf("map return for %q: %d %q want %q", path, w.Code, w.Header().Get("Location"), want)
			}
		}
		for _, path := range bad {
			if res := e.newBuyer().openSelection(code, path); res.status != 422 || tcvStr(tcvJSON(t, res.body), "code") != "bad_return_path" {
				t.Errorf("return_path %q: want 422 bad_return_path, got %d %s", path, res.status, res.body)
			}
		}
		// SQL is the authority: the definer refuses a near miss even when Go's pattern is bypassed (direct call as the checkout pool).
		b := e.newBuyer()
		call := func(path string) error {
			token := b.cap.Token
			hash := sha256.Sum256([]byte(token))
			return buyer.WithScope(ctx, e.p.pool, token, f.storeA1, func(c context.Context, tx pgx.Tx, _ buyer.Scope) error {
				nonce, req := sha256.Sum256([]byte("nonce"+path)), sha256.Sum256([]byte("req"+path))
				var raw []byte
				return tx.QueryRow(c, `SELECT fulfillment.open_cvs_selection($1,$2::uuid,$3,$4,$5::bigint,$6::uuid,$7,$8,$9,$10,$11)`,
					hash[:], f.storeA1, t04Key("si01-sql"), req[:], b.cartVersion(), e.p.market.ID, code, nonce[:], e.origin, path, "SANDBOX").Scan(&raw)
			})
		}
		for _, path := range []string{"/en/Checkout", "/en/checkout/x", "/en/checkout?x=1", "/fr/checkout", "/en/products/"} {
			if pgCode(call(path)) != "PT422" {
				t.Errorf("SQL open_cvs_selection must answer PT422 for %q, got %v", path, call(path))
			}
		}
		if err := call("/en/checkout"); err != nil {
			t.Errorf("SQL open_cvs_selection must accept /en/checkout: %v", err)
		}
		// The table CHECK is the last wall: a stored row can never hold another shape.
		var def string
		if err := f.owner.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='fulfillment.cvs_selections'::regclass AND conname='cvs_selections_return_path_check'`).Scan(&def); err != nil {
			t.Fatal(err)
		}
		const pattern = `^/(zh-TW|zh-CN|en)/(products/[A-Za-z0-9_-]{1,64}|checkout)$`
		if !strings.Contains(def, pattern) {
			t.Fatalf("return_path CHECK is not the 0093 pattern: %s", def)
		}
		for path, ok := range map[string]bool{"/en/checkout": true, "/zh-TW/products/x": true, "/en/Checkout": false, "/en/checkout/x": false, "/en/claim": false} {
			var matches bool
			if err := f.owner.QueryRow(ctx, `SELECT $1::text ~ $2::text`, path, pattern).Scan(&matches); err != nil || matches != ok {
				t.Errorf("pattern vs %q = %v (err %v), want %v", path, matches, err, ok)
			}
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
