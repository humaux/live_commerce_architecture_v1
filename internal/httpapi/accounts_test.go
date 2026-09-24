package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/platform"
)

func TestAccountWireAndMetadataRedaction(t *testing.T) {
	secret := accountSecretWire{HashKey: "sensitive-key", HashIV: "sensitive-iv"}
	create := accountCreateWire{Credentials: secret}
	rotate := accountRotateWire{Credentials: secret}
	for _, value := range []any{secret, create, rotate} {
		encoded, err := json.Marshal(value)
		if err != nil || strings.Contains(string(encoded), "sensitive") || strings.Contains(fmt.Sprint(value), "sensitive") || strings.Contains(fmt.Sprintf("%#v", value), "sensitive") {
			t.Fatalf("secret escaped representation: %T", value)
		}
	}
	var decoded accountCreateWire
	if err := json.Unmarshal([]byte(`{"provider":"payuni","credentials":{"hash_key":"sensitive-key","hash_iv":"sensitive-iv"}}`), &decoded); err != nil || decoded.Credentials.HashKey != secret.HashKey {
		t.Fatalf("wire decode failed: %v", err)
	}
	metadata, err := json.Marshal(accountMetadata(accounts.Connection{ID: "connection", KeyID: "never-public", State: "CONFIGURED_UNVERIFIED"}))
	if err != nil || strings.Contains(string(metadata), "key_id") || strings.Contains(string(metadata), "never-public") {
		t.Fatal("public metadata exposed key ID")
	}
}

func TestAccountRoutesRejectQueriesAndUnknownSecretFields(t *testing.T) {
	h := NewHandler(nil)
	base := "/v1/admin/stores/11111111-1111-4111-8111-111111111111/provider-accounts"
	for _, path := range []string{base + "/22222222-2222-4222-8222-222222222222?", base + "/22222222-2222-4222-8222-222222222222?x=1", base + "/22222222-2222-4222-8222-222222222222/rotate?x=1"} {
		method := http.MethodGet
		if strings.Contains(path, "/rotate") {
			method = http.MethodPost
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != 422 {
			t.Errorf("%s: got %d", path, w.Code)
		}
	}
	for _, body := range []string{
		`{"provider":"payuni","environment":"SANDBOX","account_id":"merchant","credentials":{"hash_key":"secret","hash_iv":"secret","extra":"secret"}}`,
		`{"provider":"payuni","environment":"SANDBOX","account_id":"merchant","credentials":{"hash_key":"secret","hash_iv":"secret"}} {}`,
	} {
		r := httptest.NewRequest(http.MethodPost, base, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 || strings.Contains(w.Body.String(), "secret") {
			t.Errorf("body guard: status=%d body=%s", w.Code, w.Body.String())
		}
	}
}

func TestAccountLimiterScopesResetCapacityAndRace(t *testing.T) {
	now := time.Now()
	l := &accountLimiter{windows: make(map[accountScopeKey]accountWindow), now: func() time.Time { return now }}
	a := platform.Scope{TenantID: "tenant-a", StoreID: "store-a"}
	b := platform.Scope{TenantID: "tenant-b", StoreID: "store-b"}
	for i := 0; i < 60; i++ {
		if err := l.admit(a); err != nil {
			t.Fatalf("early limit at %d: %v", i, err)
		}
	}
	if !errors.Is(l.admit(a), errAccountRateLimited) || l.admit(b) != nil {
		t.Fatal("one scope consumed another's budget")
	}
	now = now.Add(time.Minute)
	if err := l.admit(a); err != nil {
		t.Fatalf("window did not reset: %v", err)
	}
	if err := l.admit(b); err != nil {
		t.Fatalf("second scope did not reset: %v", err)
	}
	for i := 0; i < 4094; i++ {
		if err := l.admit(platform.Scope{TenantID: fmt.Sprintf("tenant-%d", i), StoreID: "store"}); err != nil {
			t.Fatalf("capacity early at %d: %v", i, err)
		}
	}
	if !errors.Is(l.admit(platform.Scope{TenantID: "new", StoreID: "store"}), errAccountCapacity) {
		t.Fatal("new scope exceeded capacity")
	}
	if l.admit(b) != nil {
		t.Fatal("existing scope was blocked by full map")
	}
	now = now.Add(time.Minute)
	if err := l.admit(platform.Scope{TenantID: "new", StoreID: "store"}); err != nil {
		t.Fatalf("expired windows not pruned: %v", err)
	}

	concurrent := newAccountLimiter()
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if concurrent.admit(a) == nil {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := admitted.Load(); got != 60 {
		t.Fatalf("concurrent admissions = %d, want 60", got)
	}
}

func TestAccountLimiterErrorsAreSafe(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{{errAccountRateLimited, 429, "rate_limited"}, {errAccountCapacity, 503, "unavailable"}, {errAccountUnavailable, 503, "unavailable"}} {
		status, code := classify(tc.err)
		if status != tc.status || code != tc.code {
			t.Fatalf("classify %v: %d %s", tc.err, status, code)
		}
	}
}
