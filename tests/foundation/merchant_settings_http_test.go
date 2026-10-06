package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/fulfillment"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/payments"
)

// These tests cross the actual HTTP handler, runtime database role, command
// receipt and domain transaction. No provider, browser or production is used.
func settingsPath(store, market, kind, code string) string {
	return "/v1/admin/stores/" + store + "/markets/" + market + "/countries/TW/" + kind + "/" + code
}

func settingsBody(t *testing.T, in any) []byte {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func settingsRead[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("settings status %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing safe response headers")
	}
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestMerchantSettingsHTTPPaymentPersistenceAndReplay(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	p.link(t)
	path := settingsPath(f.store, p.in.MarketID, "payment-methods", p.in.Code)
	h := httpapi.NewHandler(f.base.runtime)
	before := p.counts(t)
	body, key := settingsBody(t, p.in), t04Key("settings-create")
	put := func(body []byte, key string) *httptest.ResponseRecorder {
		return adminRequest(h, "PUT", path, f.token, body, "application/json", map[string]string{"Idempotency-Key": key})
	}
	first := settingsRead[payments.Method](t, put(body, key))
	if first.Version != 1 || first.Enabled || !first.Visible || first.ConnectionID != p.in.ConnectionID || first.NameEN != p.in.NameEN || first.NameHans != p.in.NameHans || first.NameHant != p.in.NameHant {
		t.Fatalf("incorrect draft readback: %+v", first)
	}
	// A fresh handler has no memory of the request: database persistence is required.
	got := settingsRead[payments.Method](t, adminRequest(httpapi.NewHandler(f.base.runtime), "GET", path, f.token, nil, "", nil))
	if !reflect.DeepEqual(got, first) {
		t.Fatal("GET did not return persisted configuration")
	}
	changed := p.in
	changed.ExpectedVersion, changed.Visible, changed.NameHant = 1, false, "線上刷卡"
	second := settingsRead[payments.Method](t, put(settingsBody(t, changed), t04Key("settings-hide")))
	if second.Version != 2 || second.Visible || second.Enabled || second.NameHant != "線上刷卡" {
		t.Fatal("visibility update lost independent state or language")
	}
	if replay := settingsRead[payments.Method](t, put(body, key)); !reflect.DeepEqual(replay, first) {
		t.Fatal("permanent replay changed after newer revision")
	}
	assertAdminError(t, put(settingsBody(t, changed), key), 409, "conflict")
	assertAdminError(t, put(settingsBody(t, changed), t04Key("settings-stale")), 409, "conflict")
	assertAdminError(t, put(body, ""), 422, "invalid_request")
	after := p.counts(t)
	want := before
	want[3] += 2 // accepted command receipts
	want[4] += 2 // accepted audits
	want[7] += 2 // immutable revisions
	want[8]++    // current head
	if after != want {
		t.Fatalf("unexpected durable writes or provider job: got %v want %v", after, want)
	}
}

func TestMerchantSettingsHTTPPaymentCAS(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	if _, err := p.set(f.token, f.store, t04Key("settings-cas-seed"), p.in); err != nil {
		t.Fatal(err)
	}
	h, path := httpapi.NewHandler(f.base.runtime), settingsPath(f.store, p.in.MarketID, "payment-methods", p.in.Code)
	before := p.counts(t)
	responses := make(chan *httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	for _, name := range []string{"First editor", "Second editor"} {
		in := p.in
		in.ExpectedVersion, in.NameEN = 1, name
		body, key := settingsBody(t, in), t04Key("settings-cas")
		go func() {
			<-start
			responses <- adminRequest(h, "PUT", path, f.token, body, "application/json", map[string]string{"Idempotency-Key": key})
		}()
	}
	close(start)
	winners := 0
	for range 2 {
		w := <-responses
		if w.Code == 200 {
			if got := settingsRead[payments.Method](t, w); got.Version != 2 {
				t.Fatal("CAS winner did not advance one revision")
			}
			winners++
		} else {
			assertAdminError(t, w, 409, "conflict")
		}
	}
	want := before
	want[3]++
	want[4]++
	want[7]++
	if winners != 1 || p.counts(t) != want {
		t.Fatal("concurrent editors did not produce exactly one accepted update")
	}
}

func TestMerchantSettingsHTTPAuthorityAndDiagnostics(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	p.link(t)
	if _, err := p.set(f.token, f.store, t04Key("settings-auth-seed"), p.in); err != nil {
		t.Fatal(err)
	}
	h, path := httpapi.NewHandler(f.base.runtime), settingsPath(f.store, p.in.MarketID, "payment-methods", p.in.Code)
	before := p.counts(t)
	putBody := p.in
	putBody.ExpectedVersion = 1
	body := settingsBody(t, putBody)
	for _, tc := range []struct {
		name, method, token, target, code string
		status                            int
	}{
		{"no session", "GET", "", path, "unauthorized", 401},
		{"write no session", "PUT", "", path, "unauthorized", 401},
		{"read permission absent", "GET", f.missingPermission, path, "forbidden", 403},
		{"read only write", "PUT", f.otherToken, path, "forbidden", 403},
		{"foreign tenant", "GET", f.token, settingsPath(f.base.storeB, p.in.MarketID, "payment-methods", p.in.Code), "not_found", 404},
		{"invisible store", "GET", f.otherToken, settingsPath(f.otherStore, p.in.MarketID, "payment-methods", p.in.Code), "not_found", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := adminRequest(h, tc.method, tc.target, tc.token, body, "application/json", map[string]string{"Idempotency-Key": t04Key("denied-settings"), "X-Tenant-ID": f.tenant, "X-Principal-ID": f.principal, "Cookie": "session=" + f.token})
			assertAdminError(t, w, tc.status, tc.code, f.token, f.tenant)
		})
	}
	settingsRead[payments.Method](t, adminRequest(h, "GET", path, f.otherToken, nil, "", nil))
	check := p.check(1)
	for _, version := range []int64{1, 999} {
		check.ExpectedVersion = version
		got := settingsRead[payments.Availability](t, adminRequest(h, "POST", path+"/inspect", f.otherToken, settingsBody(t, check), "application/json", nil))
		if got.Available || !slices.Contains(got.Reasons, "NOT_QUALIFIED") || !slices.Contains(got.Reasons, "METHOD_DISABLED") || (version == 999 && !slices.Contains(got.Reasons, "METHOD_VERSION_CHANGED")) {
			t.Fatalf("diagnostic claimed payment capability or lost reason: %+v", got)
		}
	}
	putBody.Enabled = true
	// w4-02b: with the platform switch off (default) the refusal is the stable platform_disabled code, still 409 and still no write.
	assertAdminError(t, adminRequest(h, "PUT", path, f.token, settingsBody(t, putBody), "application/json", map[string]string{"Idempotency-Key": t04Key("unsafe-enable")}), 409, "platform_disabled")
	mustExec(t, f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:read'`, f.tenant, f.store, f.otherPrincipal)
	assertAdminError(t, adminRequest(h, "GET", path, f.otherToken, nil, "", nil), 403, "forbidden")
	if p.counts(t) != before {
		t.Fatal("denial/inspect produced configuration, audit, receipt or provider work")
	}
}

func TestMerchantSettingsHTTPValidation(t *testing.T) {
	p := pmSetup(t)
	f := p.m.f
	h, path := httpapi.NewHandler(f.base.runtime), settingsPath(f.store, p.in.MarketID, "payment-methods", p.in.Code)
	body := string(settingsBody(t, p.in))
	before := p.counts(t)
	for _, tc := range []struct {
		name, method, suffix, media, body, code string
		status                                  int
	}{
		{"media", "PUT", "", "text/plain", body, "json_required", 415},
		{"malformed", "PUT", "", "application/json", "{", "invalid_json", 400},
		{"trailing", "PUT", "", "application/json", body + "{}", "invalid_json", 400},
		{"unknown scope", "PUT", "", "application/json", strings.TrimSuffix(body, "}") + `,"tenant_id":"forged-private-value"}`, "invalid_json", 400},
		{"unknown secret", "PUT", "", "application/json", strings.TrimSuffix(body, "}") + `,"hash_key":"forged-private-value"}`, "invalid_json", 400},
		{"oversized", "PUT", "", "application/json", `{"name_en":"` + strings.Repeat("x", 65536) + `"}`, "invalid_json", 400},
		{"wrong market", "PUT", "", "application/json", strings.Replace(body, p.in.MarketID, randomUUID(), 1), "invalid_request", 422},
		{"wrong country", "PUT", "", "application/json", strings.Replace(body, `"TW"`, `"US"`, 1), "invalid_request", 422},
		{"wrong code", "PUT", "", "application/json", strings.Replace(body, p.in.Code, "payuni_atm", 1), "invalid_request", 422},
		{"missing targets", "PUT", "", "application/json", `{}`, "invalid_request", 422},
		{"query read", "GET", "?tenant_id=forged-private-value", "", "", "invalid_request", 422},
		{"empty query", "GET", "?", "", "", "invalid_request", 422},
		{"query write", "PUT", "?x=1", "application/json", body, "invalid_request", 422},
		{"wrong method", "DELETE", "", "", "", "method_not_allowed", 405},
		{"unknown route", "GET", "/unknown", "", "", "not_found", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := adminRequest(h, tc.method, path+tc.suffix, f.token, []byte(tc.body), tc.media, map[string]string{"Idempotency-Key": t04Key("invalid-settings"), "X-Request-ID": "caller-chosen"})
			out := assertAdminError(t, w, tc.status, tc.code, "forged-private-value", f.token)
			if out.RequestID == "caller-chosen" {
				t.Fatal("untrusted request ID accepted")
			}
		})
	}
	check := p.check(1)
	check.Code = "payuni_atm"
	assertAdminError(t, adminRequest(h, "POST", path+"/inspect", f.token, settingsBody(t, check), "application/json", nil), 422, "invalid_request")
	if p.counts(t) != before {
		t.Fatal("invalid transport input wrote durable facts")
	}
}

func TestMerchantSettingsHTTPDeliverySwitches(t *testing.T) {
	h, in := dsSetup(t)
	path := settingsPath(h.f.storeA1, in.MarketID, "delivery-services", in.Code)
	handler := httpapi.NewHandler(h.f.runtime)
	put := func(in fulfillment.ServiceInput, key string) *httptest.ResponseRecorder {
		return adminRequest(handler, "PUT", path, h.f.tokens["a"], settingsBody(t, in), "application/json", map[string]string{"Idempotency-Key": key})
	}
	first := settingsRead[fulfillment.Service](t, put(in, t04Key("settings-delivery")))
	if !first.Enabled || !first.Visible || first.Mode != "MANUAL" || first.Version != 1 {
		t.Fatal("manual delivery did not persist independent switches")
	}
	read := settingsRead[fulfillment.Service](t, adminRequest(httpapi.NewHandler(h.f.runtime), "GET", path, h.f.tokens["a"], nil, "", nil))
	// delivery-allocation brief (0117): the settings PUT also auto-allocates the store's default warehouse and returns it as
	// default_warehouse_id for the UI; the plain GET projection deliberately does not resolve one (fulfillment.Service doc).
	// Everything else must round-trip unchanged.
	if first.DefaultWarehouseID == "" || read.DefaultWarehouseID != "" {
		t.Fatalf("default_warehouse_id: PUT=%q GET=%q, want set only on the PUT", first.DefaultWarehouseID, read.DefaultWarehouseID)
	}
	firstNoWarehouse := first
	firstNoWarehouse.DefaultWarehouseID = ""
	if !reflect.DeepEqual(read, firstNoWarehouse) {
		t.Fatal("delivery GET not persisted")
	}
	in.ExpectedVersion, in.Visible = 1, false
	hidden := settingsRead[fulfillment.Service](t, put(in, t04Key("settings-delivery-hide")))
	if !hidden.Enabled || hidden.Visible || hidden.PolicyVersion != first.PolicyVersion {
		t.Fatal("visibility modified enable or pricing revision")
	}
	in.ExpectedVersion, in.Enabled = 2, false
	disabled := settingsRead[fulfillment.Service](t, put(in, t04Key("settings-delivery-disable")))
	if disabled.Enabled || disabled.Visible || disabled.Version != 3 {
		t.Fatal("disable not persisted")
	}
	before := dsCount(t, h)
	in.ExpectedVersion, in.Mode, in.Enabled = 3, "API", true
	assertAdminError(t, put(in, t04Key("settings-delivery-api")), 422, "invalid_request")
	in.Mode, in.Enabled, in.Country = "MANUAL", false, "US"
	assertAdminError(t, put(in, t04Key("settings-delivery-target")), 422, "invalid_request")
	assertAdminError(t, adminRequest(handler, "GET", path+"?", h.f.tokens["a"], nil, "", nil), 422, "invalid_request")
	if dsCount(t, h) != before {
		t.Fatal("invalid delivery operation committed revision")
	}
}

// Observe the actual wait before revoking. A pre-revoked token would not test
// the missing post-wait check, and an arbitrary sleep would not prove ordering.
func TestMerchantSettingsHTTPDeliveryRevokedDuringWait(t *testing.T) {
	for _, mode := range []string{"write", "replay", "read"} {
		t.Run(mode, func(t *testing.T) {
			h, in := dsSetup(t)
			key := t04Key("settings-revoke")
			if _, err := dsSet(h, key, in); err != nil {
				t.Fatal(err)
			}
			if mode == "write" {
				in.ExpectedVersion, in.Visible = 1, false
				key = t04Key("settings-revoke-update")
			}
			before := dsCount(t, h)
			ctx := context.Background()
			block, err := h.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer block.Rollback(ctx)
			if mode == "read" {
				_, err = block.Exec(ctx, `LOCK TABLE fulfillment.service_heads IN ACCESS EXCLUSIVE MODE`)
			} else {
				lockKey := "command|" + h.f.tenantA + "|" + h.f.storeA1 + "|fulfillment.service.set|" + key
				_, err = block.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey)
			}
			if err != nil {
				t.Fatal(err)
			}
			path := settingsPath(h.f.storeA1, in.MarketID, "delivery-services", in.Code)
			handler, body := httpapi.NewHandler(h.f.runtime), settingsBody(t, in)
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				method := "PUT"
				if mode == "read" {
					method = "GET"
				}
				done <- adminRequest(handler, method, path, h.f.tokens["a"], body, "application/json", map[string]string{"Idempotency-Key": key})
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
				err = h.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, block.Conn().PgConn().PID()).Scan(&waiting)
				if err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if !waiting {
				t.Fatal("did not observe HTTP request waiting inside domain operation")
			}
			permission := "integration:manage"
			if mode == "read" {
				permission = "integration:read"
			}
			mustExec(t, h.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission=$4`, h.f.tenantA, h.f.storeA1, h.f.principalA, permission)
			defer mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, h.f.tenantA, h.f.storeA1, h.f.principalA, permission)
			if err = block.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			response := <-done
			joined = true
			assertAdminError(t, response, 403, "forbidden")
			if dsCount(t, h) != before {
				t.Fatal("revoked request committed a delivery revision")
			}
		})
	}
}
