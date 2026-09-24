package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
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
