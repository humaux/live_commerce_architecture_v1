package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/identity"
	"livecommerce/internal/identityhttp"
	"livecommerce/internal/platform"
)

type sessionStore struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

func sessionStoreList(t *testing.T, token string, status int) []sessionStore {
	t.Helper()
	w := adminRequest(httpapi.NewHandler(fixture(t).runtime, httpapi.Options{SessionStoreList: true}), "GET", "/v1/admin/stores", token, nil, "", nil)
	if w.Code != status {
		t.Fatalf("store list status=%d want=%d", w.Code, status)
	}
	assertSafeHTTPResponse(t, w, token)
	if status != 200 {
		return nil
	}
	var out struct {
		Items []sessionStore `json:"items"`
	}
	if json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Items == nil {
		t.Fatal("invalid store list projection")
	}
	return out.Items
}

func TestBrowserSessionStoreListUsesLiveAuthority(t *testing.T) {
	f := fixture(t)
	for _, key := range []string{"expired", "revoked", "buyer"} {
		sessionStoreList(t, f.tokens[key], 401)
	}
	sessionStoreList(t, randomToken(), 401)
	s, _, _ := identityFixture(t)
	session := identityLogin(t, s)
	if got := sessionStoreList(t, session.Token, 200); len(got) != 0 {
		t.Fatal("new identity received another user's store")
	}
	created, err := s.CreateInitialStore(context.Background(), session.Token, "store-list-create", firstStoreRequest())
	if err != nil {
		t.Fatal(err)
	}
	got := sessionStoreList(t, session.Token, 200)
	if len(got) != 1 || got[0].ID != created.StoreID || got[0].Currency != "TWD" {
		t.Fatal("authorized store projection mismatch")
	}
	// A second independently issued principal must not enumerate this store.
	otherService, _, _ := identityFixture(t)
	otherSession := identityLogin(t, otherService)
	if len(sessionStoreList(t, otherSession.Token, 200)) != 0 {
		t.Fatal("cross-principal store leaked")
	}
	for _, tc := range []struct {
		name, off, on string
		args          []any
		status        int
	}{
		{"tenant", `UPDATE control.tenants SET active=false WHERE id=$1`, `UPDATE control.tenants SET active=true WHERE id=$1`, []any{created.TenantID}, 200},
		{"store", `UPDATE control.stores SET active=false WHERE id=$1`, `UPDATE control.stores SET active=true WHERE id=$1`, []any{created.StoreID}, 200},
		{"membership", `UPDATE identity.memberships SET active=false WHERE tenant_id=$1`, `UPDATE identity.memberships SET active=true WHERE tenant_id=$1`, []any{created.TenantID}, 200},
		{"grant", `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND principal_id=$2 AND permission='store:read'`, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,'` + created.StoreID + `',$2,'store:read')`, []any{created.TenantID, session.PrincipalID}, 200},
		{"principal", `UPDATE identity.principals SET active=false WHERE id=$1`, `UPDATE identity.principals SET active=true WHERE id=$1`, []any{session.PrincipalID}, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.owner.Exec(context.Background(), tc.off, tc.args...); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := f.owner.Exec(context.Background(), tc.on, tc.args...); err != nil {
					t.Fatal(err)
				}
			}()
			if got := sessionStoreList(t, session.Token, tc.status); len(got) != 0 {
				t.Fatal("inactive authority still visible")
			}
		})
	}
	if err := s.Logout(context.Background(), session.Token); err != nil {
		t.Fatal(err)
	}
	sessionStoreList(t, session.Token, 401)
}

func TestBrowserSessionStoreListLimitsAndRuntimePrivileges(t *testing.T) {
	f := fixture(t)
	s, _, _ := identityFixture(t)
	session := identityLogin(t, s)
	created, err := s.CreateInitialStore(context.Background(), session.Token, "store-list-limit", firstStoreRequest())
	if err != nil {
		t.Fatal(err)
	}
	// Independent synthetic tenant, not the shared catalog fixture.
	if _, err := f.owner.Exec(context.Background(), `WITH added AS (
	 INSERT INTO control.stores(tenant_id,id,name,currency)
	 SELECT $1,gen_random_uuid(),'extra-'||n,'TWD' FROM generate_series(1,99) n RETURNING tenant_id,id)
	 INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
	 SELECT tenant_id,id,$2,'store:read' FROM added`, created.TenantID, session.PrincipalID); err != nil {
		t.Fatal(err)
	}
	got := sessionStoreList(t, session.Token, 200)
	if len(got) != 100 {
		t.Fatal("boundary count mismatch")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].ID >= got[i].ID {
			t.Fatal("list ordering unstable")
		}
	}
	if _, err := f.owner.Exec(context.Background(), `WITH added AS (
	 INSERT INTO control.stores(tenant_id,id,name,currency) VALUES($1,gen_random_uuid(),'overflow','TWD') RETURNING tenant_id,id)
	 INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) SELECT tenant_id,id,$2,'store:read' FROM added`, created.TenantID, session.PrincipalID); err != nil {
		t.Fatal(err)
	}
	sessionStoreList(t, session.Token, 409)
	if _, err := f.runtime.Exec(context.Background(), `SELECT * FROM identity.sessions`); sqlState(err) != "42501" {
		t.Fatal("runtime gained direct session access")
	}
	var owner string
	var definer, publicExec bool
	var config []string
	if err := f.owner.QueryRow(context.Background(), `SELECT pg_get_userbyid(proowner),prosecdef,proconfig,
	 EXISTS(SELECT 1 FROM aclexplode(COALESCE(proacl,acldefault('f',proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')
	 FROM pg_proc WHERE oid='identity.list_session_stores(bytea)'::regprocedure`).Scan(&owner, &definer, &config, &publicExec); err != nil {
		t.Fatal(err)
	}
	if owner != "commerce_auth" || !definer || publicExec || len(config) != 1 || config[0] != "search_path=pg_catalog" {
		t.Fatal("unsafe store-list function authority")
	}
	w := adminRequest(httpapi.NewHandler(f.runtime, httpapi.Options{SessionStoreList: true}), "GET", "/v1/admin/stores?tenant_id="+created.TenantID, session.Token, nil, "", nil)
	assertAdminError(t, w, 422, "invalid_request", session.Token)
}

func TestBrowserSessionStoreListDisabledWithValidToken(t *testing.T) {
	f := fixture(t)
	s, _, _ := identityFixture(t)
	session := identityLogin(t, s)
	if _, err := s.CreateInitialStore(context.Background(), session.Token, "disabled-list-create", firstStoreRequest()); err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.Handler{httpapi.NewHandler(f.runtime), platform.NewHandler(f.runtime), httpapi.NewHandler(nil), platform.NewHandler(nil)} {
		// nil pools additionally prove default-off never attempts the SQL query.
		w := adminRequest(handler, "GET", "/v1/admin/stores", session.Token, nil, "", nil)
		assertAdminError(t, w, 404, "not_found", session.Token)
	}
	if len(sessionStoreList(t, session.Token, 200)) != 1 {
		t.Fatal("enabled projection did not recover")
	}
}

func TestPrivateIdentityHTTPWithRealDatabase(t *testing.T) {
	s, p, _ := identityFixture(t)
	key := randomToken()
	h, err := identityhttp.NewHandler(s, key)
	if err != nil {
		t.Fatal(err)
	}
	request := func(path, body, token, idempotency string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v1/identity/"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Commerce-BFF-Key", key)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if idempotency != "" {
			r.Header.Set("Idempotency-Key", idempotency)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("login/start", "{}", "", "")
	if w.Code != 200 {
		t.Fatalf("start status=%d", w.Code)
	}
	var flow struct {
		URL       string    `json:"authorization_url"`
		Binding   string    `json:"binding"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &flow); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(flow.URL)
	if err != nil {
		t.Fatal("invalid authorization URL")
	}
	complete := func(binding string) string {
		b, _ := json.Marshal(map[string]string{"state": u.Query().Get("state"), "binding": binding, "code": "local-code"})
		return string(b)
	}
	w = request("login/complete", complete(randomToken()), "", "")
	if w.Code != 401 || p.exchanges.Load() != 0 {
		t.Fatal("invalid binding reached provider")
	}
	w = request("login/complete", complete(flow.Binding), "", "")
	if w.Code != 200 {
		t.Fatalf("complete status=%d", w.Code)
	}
	var session struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if json.Unmarshal(w.Body.Bytes(), &session) != nil || session.Token == "" {
		t.Fatal("missing session")
	}
	w = request("login/complete", complete(flow.Binding), "", "")
	if w.Code != 401 || p.exchanges.Load() != 1 {
		t.Fatal("callback replay exchanged code again")
	}
	input, _ := json.Marshal(firstStoreRequest())
	w = request("initial-store", string(input), session.Token, "http-onboard-1")
	if w.Code != 200 {
		t.Fatalf("create status=%d", w.Code)
	}
	var created identity.Store
	if json.Unmarshal(w.Body.Bytes(), &created) != nil || created.StoreID == "" {
		t.Fatal("missing store receipt")
	}
	again := request("initial-store", string(input), session.Token, "http-onboard-1")
	if again.Code != 200 || again.Body.String() != w.Body.String() {
		t.Fatal("idempotent HTTP receipt changed")
	}
	if got := sessionStoreList(t, session.Token, 200); len(got) != 1 || got[0].ID != created.StoreID {
		t.Fatal("HTTP-created store not visible")
	}
	w = request("initial-store", strings.Replace(string(input), "Test merchant", "Changed merchant", 1), session.Token, "http-onboard-1")
	if w.Code != 409 {
		t.Fatal("changed idempotent replay accepted")
	}
	w = request("logout", "{}", session.Token, "")
	if w.Code != http.StatusNoContent {
		t.Fatal("logout failed")
	}
	sessionStoreList(t, session.Token, 401)
	w = request("logout", "{}", session.Token, "")
	if w.Code != http.StatusNoContent {
		t.Fatal("repeat logout unsafe")
	}
}
