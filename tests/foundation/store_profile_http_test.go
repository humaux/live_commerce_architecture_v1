// Purpose: STORE-PROFILE-API acceptance through the actual merchant HTTP handler and scoped PostgreSQL runtime.
// Depends on: foundation fixtures, httpapi.NewHandler, real command receipts and authorization fences.
// Used by: the CI foundation catch-all shard; no provider, browser or production traffic.
// Invariants: server-derived tenant/store, durable replay, optimistic versioning, atomic audit and no read effects.
package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
)

// Test-owned wire types keep RED compilable before the production profile API exists.
type profileWire struct {
	Name         string `json:"name"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
	Version      int64  `json:"version"`
}

func profilePath(store string) string { return "/v1/admin/stores/" + store + "/profile" }

func profileInput(version int64) map[string]any {
	return map[string]any{"name": "測試商店", "contact_email": "merchant@example.test", "contact_phone": "+886 2 0000 0000", "expected_version": version}
}

func profilePut(t *testing.T, h http.Handler, f *t06GoFixture, key string, in any) *httptest.ResponseRecorder {
	return adminRequest(h, "PUT", profilePath(f.store), f.token, settingsBody(t, in), "application/json", map[string]string{"Idempotency-Key": key})
}

// Profile PUT uses claimsRoute's private boundary; GET uses the ordinary settings boundary.
// Keep exact header expectations and every generic envelope assertion without changing shared helpers.
func profileWriteRead(t *testing.T, response *httptest.ResponseRecorder) profileWire {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("settings status %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Request-ID") == "" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing private profile write response headers")
	}
	var out profileWire
	if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertProfileError(t *testing.T, method string, response *httptest.ResponseRecorder, status int, code string, forbidden ...string) adminErrorEnvelope {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d body=%s, want %d", response.Code, response.Body.String(), status)
	}
	cache := "no-store"
	if method != http.MethodGet {
		cache = "private, no-store"
	}
	if response.Header().Get("Cache-Control") != cache || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unsafe profile error headers: %v, want Cache-Control %q", response.Header(), cache)
	}
	var envelope adminErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v body=%q", err, response.Body.String())
	}
	if envelope.Code != code || envelope.Message == "" || envelope.RequestID == "" || envelope.Details == nil {
		t.Fatalf("incomplete envelope: %+v", envelope)
	}
	if response.Header().Get("X-Request-ID") != envelope.RequestID {
		t.Fatalf("request id header/body mismatch: %q / %q", response.Header().Get("X-Request-ID"), envelope.RequestID)
	}
	text := strings.ToLower(response.Body.String())
	for _, value := range forbidden {
		if value != "" && strings.Contains(text, strings.ToLower(value)) {
			t.Fatalf("error leaked %q: %s", value, response.Body.String())
		}
	}
	return envelope
}

func profileFacts(t *testing.T, f *t06GoFixture) [3]int64 {
	t.Helper()
	var out [3]int64
	if err := f.base.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM ops.command_results WHERE tenant_id=$1),
	 (SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1),
	 (SELECT count(*) FROM integration.operations WHERE tenant_id=$1)`, f.tenant).Scan(&out[0], &out[1], &out[2]); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStoreProfilePersistenceReplayAndVersion(t *testing.T) {
	f := newT06GoFixture(t)
	h := httpapi.NewHandler(f.base.runtime)
	before := profileFacts(t, f)
	initial := settingsRead[profileWire](t, adminRequest(h, "GET", profilePath(f.store), f.token, nil, "", nil))
	if initial.Name != "t06-go-store" || initial.ContactEmail != "" || initial.ContactPhone != "" || initial.Version != 0 {
		t.Fatalf("legacy profile must be a read-only version-zero projection: %+v", initial)
	}
	if profileFacts(t, f) != before {
		t.Fatal("profile GET produced durable effects")
	}
	key, in := t04Key("profile-create"), profileInput(0)
	first := profileWriteRead(t, profilePut(t, h, f, key, in))
	if first.Name != in["name"] || first.ContactEmail != in["contact_email"] || first.ContactPhone != in["contact_phone"] || first.Version != 1 {
		t.Fatalf("profile PUT lost required/optional fields or initial version: %+v", first)
	}
	secondInput := profileInput(1)
	secondInput["name"] = "Updated store"
	delete(secondInput, "contact_phone") // Optional phone is cleared by a complete replacement PUT.
	second := profileWriteRead(t, profilePut(t, h, f, t04Key("profile-update"), secondInput))
	if second.Version != 2 || second.Name != "Updated store" || second.ContactPhone != "" {
		t.Fatalf("replacement PUT failed: %+v", second)
	}
	// Replaying an earlier successful request returns its ORIGINAL result, even after a later version exists.
	if replay := profileWriteRead(t, profilePut(t, h, f, key, in)); !reflect.DeepEqual(replay, first) {
		t.Fatal("retry did not recover the original successful command")
	}
	changed := profileInput(0)
	changed["name"] = "Different body"
	assertProfileError(t, "PUT", profilePut(t, h, f, key, changed), 409, "conflict")
	assertProfileError(t, "PUT", profilePut(t, h, f, t04Key("profile-stale"), in), 409, "conflict")
	assertProfileError(t, "PUT", profilePut(t, h, f, "", profileInput(2)), 422, "invalid_request")
	got := settingsRead[profileWire](t, adminRequest(httpapi.NewHandler(f.base.runtime), "GET", profilePath(f.store), f.token, nil, "", nil))
	if !reflect.DeepEqual(got, second) {
		t.Fatal("a fresh handler did not read the latest database profile")
	}
	var storeName string
	if err := f.base.owner.QueryRow(context.Background(), `SELECT name FROM control.stores WHERE tenant_id=$1 AND id=$2`, f.tenant, f.store).Scan(&storeName); err != nil || storeName != got.Name {
		t.Fatalf("store list and profile names diverged: %q, %v", storeName, err)
	}
	after := profileFacts(t, f)
	if after != [3]int64{before[0] + 2, before[1] + 2, before[2]} {
		t.Fatalf("accepted PUTs must atomically add exactly one receipt/audit each, with no provider operation: before=%v after=%v", before, after)
	}
}

func TestStoreProfileConcurrentEditors(t *testing.T) {
	f := newT06GoFixture(t)
	h := httpapi.NewHandler(f.base.runtime)
	profileWriteRead(t, profilePut(t, h, f, t04Key("profile-seed"), profileInput(0)))
	before := profileFacts(t, f)
	start, done := make(chan struct{}), make(chan *httptest.ResponseRecorder, 2)
	for _, name := range []string{"Editor one", "Editor two"} {
		in, key := profileInput(1), t04Key("profile-cas")
		in["name"] = name
		go func() { <-start; done <- profilePut(t, h, f, key, in) }()
	}
	close(start)
	var winner profileWire
	counts := map[int]int{}
	for range 2 {
		r := <-done
		counts[r.Code]++
		if r.Code == 200 {
			winner = profileWriteRead(t, r)
		} else {
			assertProfileError(t, "PUT", r, 409, "conflict")
		}
	}
	if counts[200] != 1 || counts[409] != 1 || winner.Version != 2 {
		t.Fatalf("CAS allowed lost updates: counts=%v winner=%+v", counts, winner)
	}
	if got := settingsRead[profileWire](t, adminRequest(h, "GET", profilePath(f.store), f.token, nil, "", nil)); !reflect.DeepEqual(got, winner) {
		t.Fatal("database head differs from the winning editor")
	}
	if after := profileFacts(t, f); after != [3]int64{before[0] + 1, before[1] + 1, before[2]} {
		t.Fatalf("losing editor committed effects: %v -> %v", before, after)
	}
	// Concurrent identical retries must also consume only ONE version and receipt.
	before = profileFacts(t, f)
	key, in := t04Key("profile-duplicate"), profileInput(2)
	for range 2 {
		go func() { done <- profilePut(t, h, f, key, in) }()
	}
	a := profileWriteRead(t, <-done)
	b := profileWriteRead(t, <-done)
	if a.Version != 3 || !reflect.DeepEqual(a, b) || profileFacts(t, f) != [3]int64{before[0] + 1, before[1] + 1, before[2]} {
		t.Fatal("concurrent identical requests were applied twice")
	}
}

func TestStoreProfileScopeAndReadOnlyAuthority(t *testing.T) {
	f := newT06GoFixture(t)
	h := httpapi.NewHandler(f.base.runtime)
	before := profileFacts(t, f)
	for _, tc := range []struct {
		method, token, store, code string
		status                     int
	}{
		{"GET", "", f.store, "unauthorized", 401},
		{"PUT", "", f.store, "unauthorized", 401},
		{"GET", f.token, f.base.storeB, "not_found", 404},
		{"PUT", f.token, f.base.storeB, "not_found", 404},
		{"GET", f.otherToken, f.otherStore, "not_found", 404},
		{"PUT", f.otherToken, f.store, "forbidden", 403},
	} {
		r := adminRequest(h, tc.method, profilePath(tc.store), tc.token, settingsBody(t, profileInput(0)), "application/json", map[string]string{
			"Idempotency-Key": t04Key("profile-denied"), "X-Tenant-ID": f.tenant, "X-Store-ID": f.store,
			"X-Principal-ID": f.principal, "X-Forwarded-Host": "forged.example.test",
		})
		assertProfileError(t, tc.method, r, tc.status, tc.code, f.token, f.tenant)
	}
	// A store:read-only identity may read the profile without inventory/integration grants.
	settingsRead[profileWire](t, adminRequest(h, "GET", profilePath(f.store), f.missingPermission, nil, "", nil))
	if profileFacts(t, f) != before {
		t.Fatal("denied/readonly requests produced effects")
	}
	key := t04Key("profile-revoked-replay")
	profileWriteRead(t, profilePut(t, h, f, key, profileInput(0)))
	before = profileFacts(t, f)
	mustExec(t, f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:manage'`, f.tenant, f.store, f.principal)
	assertProfileError(t, "PUT", profilePut(t, h, f, key, profileInput(0)), 403, "forbidden")
	if profileFacts(t, f) != before {
		t.Fatal("revoked replay changed durable facts")
	}
}

func TestStoreProfileValidationAndTransport(t *testing.T) {
	f := newT06GoFixture(t)
	h := httpapi.NewHandler(f.base.runtime)
	initial := settingsRead[profileWire](t, adminRequest(h, "GET", profilePath(f.store), f.token, nil, "", nil))
	before := profileFacts(t, f)
	for _, tc := range []struct {
		field string
		value any
	}{
		{"name", ""}, {"name", "   "}, {"name", strings.Repeat("店", 121)}, {"name", "name\nline"},
		{"contact_email", ""}, {"contact_email", "not-an-email"}, {"contact_email", "Name <merchant@example.test>"},
		{"contact_email", "a@example.test\r\nBcc: other@example.test"}, {"contact_email", strings.Repeat("a", 255) + "@example.test"},
		{"contact_phone", strings.Repeat("1", 33)}, {"contact_phone", "phone\nline"},
		{"expected_version", -1}, {"expected_version", nil},
	} {
		in := profileInput(0)
		in[tc.field] = tc.value
		assertProfileError(t, "PUT", profilePut(t, h, f, t04Key("profile-invalid"), in), 422, "invalid_request")
	}
	for _, field := range []string{"name", "contact_email", "expected_version"} {
		in := profileInput(0)
		delete(in, field)
		assertProfileError(t, "PUT", profilePut(t, h, f, t04Key("profile-missing"), in), 422, "invalid_request")
	}
	for _, tc := range []struct {
		method, suffix, media, body, code string
		status                            int
	}{
		{"PUT", "", "text/plain", `{}`, "json_required", 415},
		{"PUT", "", "application/json", `{`, "invalid_json", 400},
		{"PUT", "", "application/json", `{} {}`, "invalid_json", 400},
		{"PUT", "", "application/json", `{"tenant_id":"forged-private-value"}`, "invalid_json", 400},
		{"PUT", "", "application/json", `{"store_id":"forged-private-value"}`, "invalid_json", 400},
		{"PUT", "", "application/json", `{"name":"` + strings.Repeat("x", 65536) + `"}`, "invalid_json", 400},
		{"GET", "?", "", "", "invalid_request", 422},
		{"GET", "?tenant_id=forged-private-value", "", "", "invalid_request", 422},
		{"PUT", "?x=1", "application/json", `{}`, "invalid_request", 422},
		{"DELETE", "", "", "", "method_not_allowed", 405},
		{"GET", "/unknown", "", "", "not_found", 404},
	} {
		r := adminRequest(h, tc.method, profilePath(f.store)+tc.suffix, f.token, []byte(tc.body), tc.media, map[string]string{"Idempotency-Key": t04Key("profile-transport"), "X-Request-ID": "caller-chosen"})
		err := assertProfileError(t, tc.method, r, tc.status, tc.code, "forged-private-value", f.token)
		if err.RequestID == "caller-chosen" {
			t.Fatal("request ID came from client")
		}
	}
	if got := settingsRead[profileWire](t, adminRequest(h, "GET", profilePath(f.store), f.token, nil, "", nil)); !reflect.DeepEqual(got, initial) || profileFacts(t, f) != before {
		t.Fatal("invalid request changed profile or command/audit facts")
	}
	// Inclusive name limit counts characters rather than UTF-8 bytes; optional phone can be absent.
	in := profileInput(0)
	in["name"] = strings.Repeat("店", 120)
	delete(in, "contact_phone")
	if got := profileWriteRead(t, profilePut(t, h, f, t04Key("profile-boundary"), in)); got.Name != in["name"] || got.ContactPhone != "" {
		t.Fatal("valid boundary/optional phone was rejected or changed")
	}
}

func TestStoreProfileRevocationDuringWriteWait(t *testing.T) {
	f := newT06GoFixture(t)
	h := httpapi.NewHandler(f.base.runtime)
	initial := settingsRead[profileWire](t, adminRequest(h, "GET", profilePath(f.store), f.token, nil, "", nil))
	before := profileFacts(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	block, err := f.base.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer block.Rollback(context.Background())
	if _, err = block.Exec(ctx, `SELECT id FROM control.stores WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, f.tenant, f.store); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- profilePut(t, h, f, t04Key("profile-lock-revoke"), profileInput(0)) }()
	joined := false
	defer func() {
		_ = block.Rollback(context.Background())
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
		t.Fatal("profile PUT did not reach the real row-lock wait")
	}
	mustExec(t, f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:manage'`, f.tenant, f.store, f.principal)
	if err = block.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	r := <-done
	joined = true
	assertProfileError(t, "PUT", r, 403, "forbidden")
	got := settingsRead[profileWire](t, adminRequest(h, "GET", profilePath(f.store), f.token, nil, "", nil))
	if !reflect.DeepEqual(got, initial) || profileFacts(t, f) != before {
		t.Fatal("permission revoked during lock wait did not roll back profile/audit/receipt")
	}
}

func TestStoreProfileRestrictedDatabaseWriter(t *testing.T) {
	f := newT06GoFixture(t)
	var restricted bool
	err := f.base.owner.QueryRow(context.Background(), `SELECT
	 NOT has_column_privilege('commerce_runtime','control.stores','name','UPDATE') AND
	 NOT has_column_privilege('commerce_runtime','control.stores','contact_email','UPDATE') AND
	 NOT has_column_privilege('commerce_runtime','control.stores','profile_version','UPDATE') AND
	 NOT has_column_privilege('commerce_store_profile_writer','control.stores','active','UPDATE') AND
	 NOT has_column_privilege('commerce_store_profile_writer','control.stores','tenant_id','UPDATE') AND
	 NOT has_table_privilege('commerce_store_profile_writer','control.stores','DELETE') AND
	 has_function_privilege('commerce_runtime','control.set_store_profile(bytea,uuid,text,text,text,bigint)','EXECUTE') AND
	 NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='commerce_store_profile_writer'
	  AND (rolcanlogin OR rolsuper OR rolbypassrls OR rolcreaterole OR rolcreatedb)) AND
	 NOT pg_has_role('commerce_runtime','commerce_store_profile_writer','MEMBER') AND
	 EXISTS(SELECT 1 FROM pg_proc WHERE oid='control.set_store_profile(bytea,uuid,text,text,text,bigint)'::regprocedure
	  AND prosecdef AND proowner='commerce_store_profile_writer'::regrole
	  AND 'search_path=pg_catalog'=ANY(proconfig)
	  AND NOT EXISTS(SELECT 1 FROM aclexplode(proacl) WHERE grantee=0 AND privilege_type='EXECUTE'))`).Scan(&restricted)
	if err != nil || !restricted {
		t.Fatalf("profile writer escaped its restricted capability: restricted=%v err=%v", restricted, err)
	}
}
