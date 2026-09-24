package foundation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/fulfillment"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/pagination"
	"livecommerce/internal/payments"
	"livecommerce/internal/pricing"
)

// Independent transport acceptance: actual scoped runtime PG, not provider mocks.
func wizardFixture(t *testing.T) *pmFixture {
	t.Helper()
	p := pmSetup(t)
	f := p.m.f
	mustExec(t, f.base.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
	 VALUES($1,$2,$3,'pricing:read'),($1,$2,$3,'pricing:write'),($1,$2,$4,'pricing:read'),
	 ($1,$5,$3,'pricing:read')`, f.tenant, f.store, f.principal, f.otherPrincipal, f.otherStore)
	t.Cleanup(func() {
		for _, table := range []string{"fulfillment.service_heads", "fulfillment.service_versions", "pricing.policy_heads", "pricing.policy_versions"} {
			mustExec(t, f.base.owner, "DELETE FROM "+table+" WHERE tenant_id=$1", f.tenant)
		}
	})
	return p
}

func wizardMarkets(store string) string { return "/v1/admin/stores/" + store + "/markets" }
func wizardCollection(store, market, country, kind string) string {
	return wizardMarkets(store) + "/" + market + "/countries/" + country + "/" + kind
}
func wizardPolicy(p *pmFixture, code string) pricing.PolicyInput {
	shipping, tax := int64(6000), int64(0)
	return pricing.PolicyInput{MarketID: p.in.MarketID, Country: "TW", Method: "delivery:" + code, Currency: "TWD",
		ShippingMode: "country_flat", ShippingMinor: &shipping, TaxMode: "none", TaxBasis: "goods", TaxRateBPS: &tax,
		QuoteTTLSeconds: 300, Enabled: true, ConfigurationRef: "isolated fixture explicit merchant rule"}
}
func wizardWrite(t *testing.T, h http.Handler, method, path, token, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return adminRequest(h, method, path, token, settingsBody(t, body), "application/json", map[string]string{"Idempotency-Key": key})
}
func wizardCounts(t *testing.T, p *pmFixture) [12]int64 {
	t.Helper()
	var out [12]int64
	v := p.counts(t)
	copy(out[:], v[:])
	f := p.m.f
	err := f.base.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM pricing.markets WHERE tenant_id=$1),
	 (SELECT count(*) FROM pricing.policy_versions WHERE tenant_id=$1),
	 (SELECT count(*) FROM fulfillment.service_versions WHERE tenant_id=$1)`, f.tenant).Scan(&out[9], &out[10], &out[11])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestMerchantWizardMarketTransportAndPagination(t *testing.T) {
	p := wizardFixture(t)
	f := p.m.f
	h := httpapi.NewHandler(f.base.runtime)
	path := wizardMarkets(f.store)
	in := pricing.MarketInput{Code: "second", Name: "Second market", Currency: "TWD"}
	key := t04Key("wizard-market")
	first := settingsRead[pricing.Market](t, wizardWrite(t, h, "POST", path, f.token, key, in))
	if !first.Active || first.Currency != "TWD" || first.Version != 1 {
		t.Fatal("market receipt incorrect")
	}
	if got := settingsRead[pricing.Market](t, wizardWrite(t, h, "POST", path, f.token, key, in)); !reflect.DeepEqual(got, first) {
		t.Fatal("market replay differs")
	}
	in.Name = "Changed"
	assertAdminError(t, wizardWrite(t, h, "POST", path, f.token, key, in), 409, "conflict")
	before := wizardCounts(t, p)
	page1 := settingsRead[pagination.Page[pricing.Market]](t, adminRequest(h, "GET", path+"?limit=1", f.otherToken, nil, "", nil))
	if len(page1.Items) != 1 || page1.NextCursor == "" {
		t.Fatal("missing market page")
	}
	page2 := settingsRead[pagination.Page[pricing.Market]](t, adminRequest(h, "GET", path+"?limit=1&cursor="+url.QueryEscape(page1.NextCursor), f.otherToken, nil, "", nil))
	if len(page2.Items) != 1 || page2.NextCursor != "" || page1.Items[0].ID >= page2.Items[0].ID {
		t.Fatal("UUID paging broken")
	}
	assertAdminError(t, adminRequest(h, "GET", wizardMarkets(f.otherStore)+"?cursor="+url.QueryEscape(page1.NextCursor), f.token, nil, "", nil), 422, "invalid_request")
	assertAdminError(t, adminRequest(h, "GET", path, f.missingPermission, nil, "", nil), 403, "forbidden")
	assertAdminError(t, adminRequest(h, "GET", path, "", nil, "", nil), 401, "unauthorized")
	assertAdminError(t, adminRequest(h, "GET", wizardMarkets(f.base.storeB), f.token, nil, "", nil), 404, "not_found")
	assertAdminError(t, wizardWrite(t, h, "POST", path, f.otherToken, t04Key("reader-create"), in), 403, "forbidden")
	if wizardCounts(t, p) != before {
		t.Fatal("reads or denied writes produced effects")
	}
}

func TestMerchantWizardPolicyPersistenceAndValidation(t *testing.T) {
	p := wizardFixture(t)
	f := p.m.f
	h := httpapi.NewHandler(f.base.runtime)
	path := settingsPath(f.store, p.in.MarketID, "delivery-services", "home") + "/policy"
	assertAdminError(t, adminRequest(h, "GET", path, f.token, nil, "", nil), 404, "not_found")
	in := wizardPolicy(p, "home")
	in.Enabled = false
	key := t04Key("wizard-policy")
	first := settingsRead[pricing.Policy](t, wizardWrite(t, h, "PUT", path, f.token, key, in))
	if first.Enabled || first.Version != 1 {
		t.Fatal("disabled policy save incorrect")
	}
	read := adminRequest(h, "GET", path, f.otherToken, nil, "", nil)
	if got := settingsRead[pricing.Policy](t, read); !reflect.DeepEqual(got, first) {
		t.Fatal("disabled policy was hidden or changed")
	}
	for _, private := range []string{"configuration_ref", in.ConfigurationRef, "principal_id", f.principal} {
		if strings.Contains(read.Body.String(), private) {
			t.Fatal("private policy column exposed")
		}
	}
	in.ExpectedVersion, in.Enabled = 1, true
	second := settingsRead[pricing.Policy](t, wizardWrite(t, h, "PUT", path, f.token, t04Key("policy-enable"), in))
	if !second.Enabled || second.Version != 2 {
		t.Fatal("policy CAS did not persist")
	}
	before := wizardCounts(t, p)
	assertAdminError(t, wizardWrite(t, h, "PUT", path, f.token, t04Key("policy-stale"), in), 409, "conflict")
	assertAdminError(t, wizardWrite(t, h, "PUT", path, f.otherToken, t04Key("policy-reader"), in), 403, "forbidden")
	for _, mutate := range []func(*pricing.PolicyInput){
		func(v *pricing.PolicyInput) { v.ConfigurationRef = "" },
		func(v *pricing.PolicyInput) { v.Country = "US" },
		func(v *pricing.PolicyInput) { v.Method = "delivery:other" },
		func(v *pricing.PolicyInput) { v.MarketID = randomUUID() },
		func(v *pricing.PolicyInput) { v.ShippingMinor = nil },
		func(v *pricing.PolicyInput) { v.Currency = "USD" },
	} {
		bad := in
		bad.ExpectedVersion = 2
		mutate(&bad)
		assertAdminError(t, wizardWrite(t, h, "PUT", path, f.token, t04Key("policy-invalid"), bad), 422, "invalid_request")
	}
	if wizardCounts(t, p) != before {
		t.Fatal("invalid policies mutated durable state")
	}
}

func TestMerchantWizardDeliveryDiscoveryStableCursor(t *testing.T) {
	p := wizardFixture(t)
	f := p.m.f
	h := httpapi.NewHandler(f.base.runtime)
	base := wizardCollection(f.store, p.in.MarketID, "TW", "delivery-services")
	empty := settingsRead[pagination.Page[fulfillment.Service]](t, adminRequest(h, "GET", base, f.token, nil, "", nil))
	if empty.Items == nil || len(empty.Items) != 0 || empty.NextCursor != "" {
		t.Fatal("empty delivery list not explicit")
	}
	for index, code := range []string{"alpha", "beta", "gamma"} {
		settingsRead[pricing.Policy](t, wizardWrite(t, h, "PUT", base+"/"+code+"/policy", f.token, t04Key("discovery-policy"), wizardPolicy(p, code)))
		in := fulfillment.ServiceInput{MarketID: p.in.MarketID, Country: "TW", Code: code, PolicyVersion: 1,
			NameHans: "配送", NameHant: "配送", NameEN: "Delivery", Mode: "MANUAL", DeliveryKind: "home", Visible: true, SortOrder: 30 - index}
		settingsRead[fulfillment.Service](t, wizardWrite(t, h, "PUT", base+"/"+code, f.token, t04Key("discovery-service"), in))
	}
	before := wizardCounts(t, p)
	page1 := settingsRead[pagination.Page[fulfillment.Service]](t, adminRequest(h, "GET", base+"?limit=2", f.otherToken, nil, "", nil))
	if len(page1.Items) != 2 || page1.Items[0].Code != "alpha" || page1.Items[1].Code != "beta" || page1.NextCursor == "" {
		t.Fatal("delivery cursor sorted by unstable display order")
	}
	page2 := settingsRead[pagination.Page[fulfillment.Service]](t, adminRequest(h, "GET", base+"?limit=2&cursor="+url.QueryEscape(page1.NextCursor), f.otherToken, nil, "", nil))
	if len(page2.Items) != 1 || page2.Items[0].Code != "gamma" || page2.NextCursor != "" {
		t.Fatal("delivery second page incorrect")
	}
	for _, target := range []string{strings.Replace(base, "/TW/", "/US/", 1), wizardMarkets(f.store), strings.Replace(base, p.in.MarketID, randomUUID(), 1)} {
		response := adminRequest(h, "GET", target+"?cursor="+url.QueryEscape(page1.NextCursor), f.token, nil, "", nil)
		// Target absence may precede cursor validation; neither may disclose rows.
		if response.Code != 422 && response.Code != 404 {
			t.Fatalf("cross target cursor accepted: %d", response.Code)
		}
	}
	other := settingsRead[pagination.Page[fulfillment.Service]](t, adminRequest(h, "GET", strings.Replace(base, "/TW/", "/US/", 1), f.token, nil, "", nil))
	if len(other.Items) != 0 {
		t.Fatal("country leaked")
	}
	if wizardCounts(t, p) != before {
		t.Fatal("discovery created effects")
	}
}

func TestMerchantWizardPaymentListBoundsAndExactResources(t *testing.T) {
	p := wizardFixture(t)
	f := p.m.f
	h := httpapi.NewHandler(f.base.runtime)
	base := wizardCollection(f.store, p.in.MarketID, "TW", "payment-methods")
	for _, code := range []string{"payuni_linepay", "payuni_credit", "payuni_atm", "payuni_cvs", "payuni_installment"} {
		in := p.in
		in.Code = code
		settingsRead[payments.Method](t, wizardWrite(t, h, "PUT", base+"/"+code, f.token, t04Key("wizard-paymethod"), in))
	}
	before := wizardCounts(t, p)
	got := settingsRead[pagination.Page[payments.Method]](t, adminRequest(h, "GET", base, f.otherToken, nil, "", nil))
	if len(got.Items) != 5 || got.NextCursor != "" || got.Items[0].Code != "payuni_atm" || got.Items[4].Code != "payuni_linepay" {
		t.Fatal("fixed method discovery incorrect")
	}
	for _, item := range got.Items {
		if item.Enabled {
			t.Fatal("draft method enabled")
		}
	}
	for _, suffix := range []string{"?", "?limit=1", "?unknown=1"} {
		assertAdminError(t, adminRequest(h, "GET", base+suffix, f.token, nil, "", nil), 422, "invalid_request")
		assertAdminError(t, adminRequest(h, "GET", settingsPath(f.store, p.in.MarketID, "delivery-services", "home")+"/policy"+suffix, f.token, nil, "", nil), 422, "invalid_request")
	}
	for _, path := range []string{wizardMarkets(f.store), wizardCollection(f.store, p.in.MarketID, "TW", "delivery-services")} {
		for _, query := range []string{"?limit=0", "?limit=101", "?limit=1&limit=2", "?extra=1", "?cursor=invalid"} {
			assertAdminError(t, adminRequest(h, "GET", path+query, f.token, nil, "", nil), 422, "invalid_request")
		}
	}
	assertAdminError(t, adminRequest(h, "GET", strings.Replace(base, p.in.MarketID, randomUUID(), 1), f.token, nil, "", nil), 404, "not_found")
	if wizardCounts(t, p) != before {
		t.Fatal("diagnostic reads caused effects")
	}
}

func TestMerchantWizardPricingRechecksAfterLockWait(t *testing.T) {
	for _, mode := range []string{"market-write", "market-replay", "policy-write", "policy-replay", "market-read", "policy-read"} {
		t.Run(mode, func(t *testing.T) {
			p := wizardFixture(t)
			f := p.m.f
			h := httpapi.NewHandler(f.base.runtime)
			key, method, path, permission, operation := t04Key("wizard-revoke"), "POST", wizardMarkets(f.store), "pricing:write", "pricing.market.create"
			var body any = pricing.MarketInput{Code: "blocked", Name: "Blocked fixture", Currency: "TWD"}
			if strings.HasPrefix(mode, "policy") {
				method, path, operation = "PUT", settingsPath(f.store, p.in.MarketID, "delivery-services", "home")+"/policy", "pricing.policy.set"
				body = wizardPolicy(p, "home")
			}
			if strings.HasSuffix(mode, "replay") || mode == "policy-read" {
				settingsRead[map[string]any](t, wizardWrite(t, h, method, path, f.token, key, body))
			}
			if strings.HasSuffix(mode, "read") {
				method, permission = "GET", "pricing:read"
			}
			before, ctx := wizardCounts(t, p), context.Background()
			block, err := f.base.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer block.Rollback(ctx)
			if method == "GET" {
				table := "pricing.markets"
				if mode == "policy-read" {
					table = "pricing.policy_heads"
				}
				_, err = block.Exec(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE")
			} else {
				_, err = block.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "command|"+f.tenant+"|"+f.store+"|"+operation+"|"+key)
			}
			if err != nil {
				t.Fatal(err)
			}
			payload := settingsBody(t, body)
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				done <- adminRequest(h, method, path, f.token, payload, "application/json", map[string]string{"Idempotency-Key": key})
			}()
			joined := false
			defer func() {
				_ = block.Rollback(ctx)
				if !joined {
					<-done
				}
			}()
			waiting := false
			deadline := time.Now().Add(700 * time.Millisecond)
			for time.Now().Before(deadline) {
				if err = f.base.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, block.Conn().PgConn().PID()).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if !waiting {
				t.Fatal("request did not observably wait in PG")
			}
			mustExec(t, f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`, f.tenant, f.store, f.principal, permission)
			if err = block.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			response := <-done
			joined = true
			assertAdminError(t, response, 403, "forbidden")
			if wizardCounts(t, p) != before {
				t.Fatal("revoked pricing request committed effects")
			}
		})
	}
}
