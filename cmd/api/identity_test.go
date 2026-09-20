package main

import (
	"context"
	"strings"
	"testing"
)

func TestIdentityDisabledDoesNotReadAuthorityConfiguration(t *testing.T) {
	for _, value := range []string{"", "0"} {
		c, err := loadIdentityConfig(func(key string) string {
			if key != "COMMERCE_IDENTITY_ENABLED" {
				t.Fatal("disabled identity read another configuration key")
			}
			return value
		})
		if err != nil || c.enabled {
			t.Fatal("disabled config was not disabled")
		}
		h, closeFn, err := buildIdentityHandler(context.Background(), c)
		if err != nil || h != nil || closeFn == nil {
			t.Fatal("disabled identity was constructed")
		}
		closeFn()
	}
}

func TestIdentityEnabledConfigurationFailClosed(t *testing.T) {
	base := map[string]string{
		"COMMERCE_IDENTITY_ENABLED": "1", "COMMERCE_PUBLIC_ORIGIN": "https://merchant.example",
		"COMMERCE_IDENTITY_DATABASE_URL": "postgres://config-not-connected.invalid/identity", "COMMERCE_BFF_KEY": strings.Repeat("A", 43),
		"COMMERCE_OIDC_ISSUER": "https://idp.example", "COMMERCE_OIDC_CLIENT_ID": "test-client",
		"COMMERCE_IDENTITY_PROVIDER_KEY": "test-provider", "COMMERCE_SESSION_TTL": "1h",
		"COMMERCE_ONBOARDING_ENABLED": "1", "COMMERCE_ONBOARDING_CURRENCIES": "TWD,USD",
	}
	c, err := loadIdentityConfig(func(key string) string { return base[key] })
	if err != nil || !c.enabled || c.provider.RedirectURL != "https://merchant.example/api/auth/callback" || len(c.policy.Currencies) != 2 {
		t.Fatal("valid configuration rejected")
	}
	for _, key := range []string{"COMMERCE_PUBLIC_ORIGIN", "COMMERCE_IDENTITY_DATABASE_URL", "COMMERCE_BFF_KEY", "COMMERCE_OIDC_ISSUER", "COMMERCE_OIDC_CLIENT_ID", "COMMERCE_IDENTITY_PROVIDER_KEY", "COMMERCE_SESSION_TTL", "COMMERCE_ONBOARDING_CURRENCIES"} {
		t.Run("missing_"+key, func(t *testing.T) {
			if _, err := loadIdentityConfig(func(k string) string {
				if k == key {
					return ""
				}
				return base[k]
			}); err == nil {
				t.Fatal("missing required configuration accepted")
			}
		})
	}
	for key, values := range map[string][]string{
		"COMMERCE_IDENTITY_ENABLED": {"true", "2"}, "COMMERCE_SESSION_TTL": {"1m", "25h", "forever"},
		"COMMERCE_BFF_KEY": {"short", strings.Repeat("A", 42) + "B"}, "COMMERCE_FIXTURE_ENABLED": {"1"},
		"COMMERCE_ONBOARDING_ENABLED": {"true"}, "COMMERCE_ONBOARDING_CURRENCIES": {"USD,", "twd", "TWD, US1"},
	} {
		for _, value := range values {
			t.Run("invalid_"+key+"_"+value, func(t *testing.T) {
				if _, err := loadIdentityConfig(func(k string) string {
					if k == key {
						return value
					}
					return base[k]
				}); err == nil {
					t.Fatal("invalid configuration accepted")
				}
			})
		}
	}
}

func TestIdentityPublicOriginAndPrivateListener(t *testing.T) {
	for _, raw := range []string{"", "https://u:p@merchant.example", "https://merchant.example/path", "https://merchant.example?x=1", "https://merchant.example#", "http://merchant.example", "http://127.0.0.1:3100", "javascript:alert(1)", "//merchant.example", "https://merchant.example:99999"} {
		if _, err := publicOrigin(raw, false); err == nil {
			t.Errorf("unsafe public origin accepted: %q", raw)
		}
	}
	for _, raw := range []string{"http://localhost:3100", "http://127.0.0.1:3100", "http://[::1]:3100"} {
		if _, err := publicOrigin(raw, true); err != nil {
			t.Fatal("explicit loopback test origin rejected")
		}
	}
	if _, err := publicOrigin("http://127.0.0.1.attacker.example", true); err == nil {
		t.Fatal("loopback prefix spoof accepted")
	}
	if got, err := publicOrigin("https://Merchant.Example:443/", false); err != nil || got != "https://merchant.example" {
		t.Fatal("canonical origin mismatch")
	}
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080"} {
		if !privateIdentityAddress(addr) {
			t.Fatal("literal loopback listener rejected")
		}
	}
	for _, addr := range []string{":8080", "0.0.0.0:8080", "[::]:8080", "localhost:8080", "127.0.0.1:0", "127.0.0.1:65536", "10.0.0.1:8080"} {
		if privateIdentityAddress(addr) {
			t.Errorf("unsafe listener accepted: %s", addr)
		}
	}
}
