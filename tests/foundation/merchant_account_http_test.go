package foundation_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/pagination"
)

// The boundary under test is real HTTP -> runtime role -> encrypted PG records.
// All secrets below are synthetic; no provider request or business payment runs.
func mahPath(store string) string { return "/v1/admin/stores/" + store + "/provider-accounts" }
func mahBody(t *testing.T, in accounts.CreateInput) []byte {
	t.Helper()
	// Production Credentials intentionally cannot marshal plaintext. Construct
	// an independent wire fixture rather than weakening that safety contract.
	return settingsBody(t, map[string]any{"provider": in.Provider, "environment": in.Environment,
		"account_id": in.AccountID, "credentials": map[string]string{"hash_key": in.Credentials.HashKey, "hash_iv": in.Credentials.HashIV}})
}
func mahRequest(h http.Handler, method, path, token, key string, body []byte) *httptest.ResponseRecorder {
	return adminRequest(h, method, path, token, body, "application/json", map[string]string{"Idempotency-Key": key})
}
func mahRead(t *testing.T, w *httptest.ResponseRecorder, secret accounts.Credentials) accounts.Connection {
	t.Helper()
	for _, forbidden := range []string{secret.HashKey, secret.HashIV, `"key_id"`, `"ciphertext"`, `"nonce"`} {
		if forbidden != "" && strings.Contains(w.Body.String(), forbidden) {
			t.Fatal("secret or encryption metadata escaped HTTP")
		}
	}
	return settingsRead[accounts.Connection](t, w)
}

func TestMerchantAccountHTTPRoundTripAndPermanentReplay(t *testing.T) {
	m := maSetup(t)
	h := httpapi.NewHandler(m.f.base.runtime, httpapi.Options{Accounts: m.service})
	path, in, key := mahPath(m.f.store), maInput(), t04Key("account-http")
	body, before := mahBody(t, in), m.facts(t)
	first := mahRead(t, mahRequest(h, "POST", path, m.f.token, key, body), in.Credentials)
	if first.KeyID != "" || first.State != "CONFIGURED_UNVERIFIED" || first.CredentialVersion != 1 {
		t.Fatal("unsafe account state")
	}
	maAssertPersistedSecret(t, m, first, 1, in.Credentials)
	after := m.facts(t)
	if after[0] != before[0]+1 || after[1] != before[1]+1 || after[2] != before[2]+1 || after[5] != before[5] || after[6] != before[6] {
		t.Fatal("wrong account aggregate or external operation")
	}
	fresh := httpapi.NewHandler(m.f.base.runtime, httpapi.Options{Accounts: m.service})
	got := mahRead(t, mahRequest(fresh, "GET", path+"/"+first.ID, m.f.token, "", nil), in.Credentials)
	if !reflect.DeepEqual(first, got) {
		t.Fatal("fresh handler did not read persisted account")
	}
	rotatedSecret := accounts.Credentials{HashKey: strings.Repeat("N", 32), HashIV: strings.Repeat("W", 16)}
	rotation := settingsBody(t, map[string]any{"expected_version": 1, "credentials": map[string]string{"hash_key": rotatedSecret.HashKey, "hash_iv": rotatedSecret.HashIV}})
	rotateKey := t04Key("account-http-rotate")
	rotated := mahRead(t, mahRequest(h, "POST", path+"/"+first.ID+"/rotate", m.f.token, rotateKey, rotation), rotatedSecret)
	if rotated.CredentialVersion != 2 || rotated.ID != first.ID || rotated.BindingID != first.BindingID || rotated.State != first.State {
		t.Fatal("rotation rebound or falsely verified account")
	}
	maAssertPersistedSecret(t, m, rotated, 2, rotatedSecret)
	maAssertPersistedSecret(t, m, first, 1, in.Credentials)
	stable := m.facts(t)
	if replay := mahRead(t, mahRequest(h, "POST", path, m.f.token, key, body), in.Credentials); !reflect.DeepEqual(replay, first) {
		t.Fatal("historical create replay changed")
	}
	if replay := mahRead(t, mahRequest(h, "POST", path+"/"+first.ID+"/rotate", m.f.token, rotateKey, rotation), rotatedSecret); !reflect.DeepEqual(replay, rotated) {
		t.Fatal("rotation replay changed")
	}
	in.Credentials = rotatedSecret
	assertAdminError(t, mahRequest(h, "POST", path, m.f.token, key, mahBody(t, in)), 409, "conflict")
	assertAdminError(t, mahRequest(h, "POST", path+"/"+first.ID+"/rotate", m.f.token, t04Key("stale-rotate"), rotation), 409, "conflict")
	if m.facts(t) != stable {
		t.Fatal("replay/conflict had durable effects")
	}
}

func TestMerchantAccountHTTPPaginationAndAuthority(t *testing.T) {
	m := maSetup(t)
	h := httpapi.NewHandler(m.f.base.runtime, httpapi.Options{Accounts: m.service})
	path := mahPath(m.f.store)
	ids := map[string]bool{}
	for range 3 {
		in := maInput()
		a := mahRead(t, mahRequest(h, "POST", path, m.f.token, t04Key("account-page"), mahBody(t, in)), in.Credentials)
		ids[a.ID] = true
	}
	stable := m.facts(t)
	cursor, previous, firstCursor := "", "", ""
	for range 3 {
		page := settingsRead[pagination.Page[accounts.Connection]](t, mahRequest(h, "GET", path+"?limit=1&cursor="+url.QueryEscape(cursor), m.f.token, "", nil))
		if len(page.Items) != 1 || !ids[page.Items[0].ID] || page.Items[0].ID <= previous || page.Items[0].KeyID != "" {
			t.Fatal("invalid scoped page")
		}
		previous, cursor = page.Items[0].ID, page.NextCursor
		delete(ids, previous)
		if firstCursor == "" {
			firstCursor = cursor
		}
	}
	if cursor != "" || len(ids) != 0 {
		t.Fatal("pagination omitted or duplicated account")
	}
	assertAdminError(t, mahRequest(h, "GET", mahPath(m.f.otherStore)+"?cursor="+url.QueryEscape(firstCursor), m.f.token, "", nil), 422, "invalid_request")
	for _, suffix := range []string{"", "/" + previous} {
		assertAdminError(t, mahRequest(h, "GET", path+suffix, "", "", nil), 401, "unauthorized")
		assertAdminError(t, mahRequest(h, "GET", path+suffix, m.f.missingPermission, "", nil), 403, "forbidden")
		assertAdminError(t, mahRequest(h, "GET", mahPath(randomUUID())+suffix, m.f.token, "", nil), 404, "not_found")
	}
	assertAdminError(t, mahRequest(h, "GET", mahPath(m.f.otherStore)+"/"+previous, m.f.token, "", nil), 404, "not_found")
	readOnly := mahRequest(h, "POST", path, m.f.otherToken, t04Key("readonly-account"), mahBody(t, maInput()))
	assertAdminError(t, readOnly, 403, "forbidden")
	if m.facts(t) != stable {
		t.Fatal("reads/denials mutated account state")
	}
}

func TestMerchantAccountHTTPValidationAndDisabledService(t *testing.T) {
	m := maSetup(t)
	h := httpapi.NewHandler(m.f.base.runtime, httpapi.Options{Accounts: m.service})
	path, in := mahPath(m.f.store), maInput()
	body, before := mahBody(t, in), m.facts(t)
	for _, tc := range []struct {
		method, suffix string
		body           []byte
		status         int
		code           string
	}{
		{"POST", "?", body, 422, "invalid_request"}, {"POST", "?x=1", body, 422, "invalid_request"},
		{"POST", "", []byte(`{"credentials":{"hash_key":"secret","hash_iv":"fixture","unexpected":1}}`), 400, "invalid_json"},
		{"POST", "", append(append([]byte{}, body...), []byte(` {}`)...), 400, "invalid_json"},
		{"POST", "", []byte(`{"provider":"payuni","environment":"SANDBOX","account_id":"x","credentials":null}`), 422, "invalid_request"},
		{"POST", "", []byte(`{"tenant_id":"forged"}`), 400, "invalid_json"},
		{"POST", "", []byte(`{"credentials":{"hash_key":"` + strings.Repeat("X", 70<<10) + `"}}`), 400, "invalid_json"},
		{"GET", "?limit=101", nil, 422, "invalid_request"}, {"GET", "?limit=1&limit=2", nil, 422, "invalid_request"},
		{"GET", "?tenant_id=x", nil, 422, "invalid_request"}, {"GET", "/not-a-uuid", nil, 422, "invalid_request"},
		{"DELETE", "", nil, 405, "method_not_allowed"},
	} {
		w := mahRequest(h, tc.method, path+tc.suffix, m.f.token, t04Key("account-bad"), tc.body)
		assertAdminError(t, w, tc.status, tc.code)
		if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), in.Credentials.HashKey) {
			t.Fatal("invalid input leaked")
		}
	}
	assertAdminError(t, mahRequest(h, "POST", path, m.f.token, "", body), 422, "invalid_request")
	nilHandler := httpapi.NewHandler(m.f.base.runtime)
	for _, method := range []string{"POST", "GET"} {
		assertAdminError(t, mahRequest(nilHandler, method, path, m.f.token, t04Key("account-off"), body), 503, "unavailable")
		assertAdminError(t, mahRequest(nilHandler, method, path, "", t04Key("account-off-no-auth"), body), 401, "unauthorized")
	}
	if m.facts(t) != before {
		t.Fatal("invalid account requests committed data")
	}
}

func TestMerchantAccountHTTPRateLimitIsScoped(t *testing.T) {
	m := maSetup(t)
	h := httpapi.NewHandler(m.f.base.runtime, httpapi.Options{Accounts: m.service})
	path, in, key := mahPath(m.f.store), maInput(), t04Key("account-limit")
	body := mahBody(t, in)
	first := mahRead(t, mahRequest(h, "POST", path, m.f.token, key, body), in.Credentials)
	stable := m.facts(t)
	for i := 1; i < 60; i++ {
		mahRead(t, mahRequest(h, "POST", path, m.f.token, key, body), in.Credentials)
	}
	w := mahRequest(h, "POST", path, m.f.token, key, body)
	assertAdminError(t, w, 429, "rate_limited")
	if w.Header().Get("Retry-After") != "60" || m.facts(t) != stable {
		t.Fatal("limiter altered data or omitted backoff")
	}
	mahRead(t, mahRequest(h, "GET", path+"/"+first.ID, m.f.token, "", nil), in.Credentials)
	// The SAME handler/process cannot let one store exhaust a different store.
	other := mahRead(t, mahRequest(h, "POST", mahPath(m.f.otherStore), m.f.token, key, body), in.Credentials)
	if first.ID == other.ID {
		t.Fatal("stores shared an account")
	}
}

func TestMerchantAccountHTTPConcurrentRotateCAS(t *testing.T) {
	m := maSetup(t)
	h := httpapi.NewHandler(m.f.base.runtime, httpapi.Options{Accounts: m.service})
	in := maInput()
	a := mahRead(t, mahRequest(h, "POST", mahPath(m.f.store), m.f.token, t04Key("account-cas"), mahBody(t, in)), in.Credentials)
	before, done := m.facts(t), make(chan *httptest.ResponseRecorder, 2)
	start := make(chan struct{})
	for _, ch := range []string{"N", "M"} {
		body := settingsBody(t, map[string]any{"expected_version": 1, "credentials": map[string]string{"hash_key": strings.Repeat(ch, 32), "hash_iv": strings.Repeat("V", 16)}})
		go func() {
			<-start
			done <- mahRequest(h, "POST", mahPath(m.f.store)+"/"+a.ID+"/rotate", m.f.token, t04Key("rotate-cas"), body)
		}()
	}
	close(start)
	statuses := map[int]int{}
	for range 2 {
		statuses[(<-done).Code]++
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatalf("CAS statuses: %v", statuses)
	}
	after := m.facts(t)
	if after[2] != before[2]+1 || after[3] != before[3]+1 || after[4] != before[4]+1 || after[5] != before[5] || after[6] != before[6] {
		t.Fatal("CAS wrote more than one credential revision")
	}
}

func TestMerchantAccountHTTPListRechecksAfterLockWait(t *testing.T) {
	m := maSetup(t)
	h := httpapi.NewHandler(m.f.base.runtime, httpapi.Options{Accounts: m.service})
	in := maInput()
	mahRead(t, mahRequest(h, "POST", mahPath(m.f.store), m.f.token, t04Key("account-revoke"), mahBody(t, in)), in.Credentials)
	ctx, before := context.Background(), m.facts(t)
	block, err := m.f.base.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer block.Rollback(ctx)
	if _, err = block.Exec(ctx, `LOCK TABLE integration.merchant_accounts IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- mahRequest(h, "GET", mahPath(m.f.store), m.f.token, "", nil) }()
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
		if err = m.f.base.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, block.Conn().PgConn().PID()).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("account list did not actually wait inside PostgreSQL")
	}
	mustExec(t, m.f.base.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:read'`, m.f.tenant, m.f.store, m.f.principal)
	if err = block.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	w := <-done
	joined = true
	assertAdminError(t, w, 403, "forbidden")
	var result map[string]any
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result["items"]; ok {
		t.Fatal("revoked read returned account list")
	}
	if m.facts(t) != before {
		t.Fatal("revoked read mutated data")
	}
}
