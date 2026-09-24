package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/httpapi"
)

func accountTestEnv() map[string]string {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	replay := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	return map[string]string{
		"COMMERCE_ACCOUNTS_ENABLED":      "1",
		"COMMERCE_ACCOUNT_ACTIVE_KEY_ID": "key-1",
		"COMMERCE_ACCOUNT_KEYS_JSON":     `[{"id":"key-1","key_base64":"` + key + `"}]`,
		"COMMERCE_ACCOUNT_REPLAY_KEY":    replay,
	}
}

func TestDisabledAccountsReadNoSecrets(t *testing.T) {
	read := []string{}
	config, err := loadAccountConfig(func(name string) string {
		read = append(read, name)
		return ""
	}, false, "0.0.0.0:8080")
	if err != nil || config.enabled || len(read) != 1 || read[0] != "COMMERCE_ACCOUNTS_ENABLED" {
		t.Fatalf("disabled config read %+v: %v", read, err)
	}
	if service, err := buildAccountsService(nil, config); err != nil || service != nil {
		t.Fatalf("disabled build: service=%v err=%v", service, err)
	}
}

// Configuration/constructor smoke only; real authenticated persistence is
// covered independently by foundation HTTP/PG and the signed-IdP browser gate.
func TestEnabledAccountsEnvironmentAssemblyStartsNoWorker(t *testing.T) {
	for name, value := range accountTestEnv() {
		t.Setenv(name, value)
	}
	config, err := loadAccountConfig(os.Getenv, true, "127.0.0.1:8080")
	if err != nil || !config.enabled {
		t.Fatal("enabled environment did not load")
	}
	poolConfig, err := pgxpool.ParseConfig("postgres://fixture@127.0.0.1:1/fixture")
	if err != nil {
		t.Fatal(err)
	}
	var connects atomic.Int32
	poolConfig.BeforeConnect = func(context.Context, *pgx.ConnConfig) error {
		connects.Add(1)
		return errors.New("constructor must not open a database connection")
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service, err := buildAccountsService(pool, config)
	if err != nil || service == nil {
		t.Fatal("enabled account service did not assemble")
	}
	handler := httpapi.NewHandler(pool, httpapi.Options{SessionStoreList: true, Accounts: service})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/admin/stores/11111111-1111-4111-8111-111111111111/provider-accounts", nil))
	if w.Code != http.StatusUnauthorized || connects.Load() != 0 {
		t.Fatal("unauthenticated assembly performed work or bypassed authority")
	}
}

func TestAccountConfigStrictBoundary(t *testing.T) {
	base := accountTestEnv()
	read := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	if config, err := loadAccountConfig(read(base), true, "127.0.0.1:8080"); err != nil || !config.enabled || config.keys == nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	for _, tc := range []struct {
		name, field, value string
	}{
		{"bad flag", "COMMERCE_ACCOUNTS_ENABLED", "yes"},
		{"unknown field", "COMMERCE_ACCOUNT_KEYS_JSON", `[{"id":"key-1","key_base64":"` + key + `","extra":1}]`},
		{"trailing value", "COMMERCE_ACCOUNT_KEYS_JSON", base["COMMERCE_ACCOUNT_KEYS_JSON"] + ` {}`},
		{"duplicate IDs", "COMMERCE_ACCOUNT_KEYS_JSON", `[{"id":"key-1","key_base64":"` + key + `"},{"id":"key-1","key_base64":"` + key + `"}]`},
		{"noncanonical base64", "COMMERCE_ACCOUNT_KEYS_JSON", `[{"id":"key-1","key_base64":"` + strings.TrimRight(key, "=") + `"}]`},
		{"wrong length", "COMMERCE_ACCOUNT_REPLAY_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 31))},
		{"replay reused", "COMMERCE_ACCOUNT_REPLAY_KEY", key},
		{"missing active", "COMMERCE_ACCOUNT_ACTIVE_KEY_ID", "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := accountTestEnv()
			values[tc.field] = tc.value
			_, err := loadAccountConfig(read(values), true, "127.0.0.1:8080")
			if !errors.Is(err, errAccountConfig) || strings.Contains(err.Error(), tc.value) {
				t.Fatalf("unsafe config error: %v", err)
			}
		})
	}
}

func TestAccountConfigRequiresIdentityAndLiteralLoopbackBeforeSecrets(t *testing.T) {
	for _, tc := range []struct {
		identity bool
		addr     string
	}{{false, "127.0.0.1:8080"}, {true, "localhost:8080"}, {true, "0.0.0.0:8080"}} {
		read := []string{}
		_, err := loadAccountConfig(func(name string) string {
			read = append(read, name)
			return accountTestEnv()[name]
		}, tc.identity, tc.addr)
		if !errors.Is(err, errAccountConfig) || len(read) != 1 || read[0] != "COMMERCE_ACCOUNTS_ENABLED" {
			t.Fatalf("unsafe gate: %+v err=%v", read, err)
		}
	}
}
