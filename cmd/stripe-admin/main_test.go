// main_test.go: usage, environment gating and secret-hygiene tests for the registrar CLI (MOCK tier).
// Non-goal: the registry SQL (REAL_PG SP21, blocked on pool-fix) and any real Stripe call.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/payments/stripeadmin"
)

// Synthetic DSN passwords live in their own constants so no source line looks like a
// credential to secret scanners (GitGuardian false positives 2026-09-29); they are test sentinels.
const (
	fakeDSNPassword1 = "pw-secret"
)

func env() map[string]string {
	k := func(b byte) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{b}, 32)) }
	return map[string]string{
		"COMMERCE_STRIPE_REGISTRAR_DATABASE_URL": "postgres://operator:" + fakeDSNPassword1 + "@127.0.0.1:1/lc",
		"COMMERCE_ACCOUNT_ACTIVE_KEY_ID":         "api-1",
		"COMMERCE_ACCOUNT_KEYS_JSON":             `[{"id":"api-1","key_base64":"` + k(1) + `"}]`,
		"COMMERCE_ACCOUNT_REPLAY_KEY":            k(2),
		"COMMERCE_STRIPE_WEBHOOK_ACTIVE_KEY_ID":  "sig-1",
		"COMMERCE_STRIPE_WEBHOOK_KEYS_JSON":      `[{"id":"sig-1","key_base64":"` + k(3) + `"}]`,
		"COMMERCE_STRIPE_WEBHOOK_REPLAY_KEY":     k(4),
		"STRIPE_SECRET_KEY":                      "sk_" + "test_CLISENTINELKEY000000000",
		"STRIPE_ACCOUNT_ID":                      "acct_1CliTest000000",
		"STRIPE_WEBHOOK_SECRET":                  "whsec_" + "cliSentinelSecret_0123456789",
	}
}

const ids = "--tenant 11111111-1111-4111-8111-111111111111 --store 22222222-2222-4222-8222-222222222222 --principal 33333333-3333-4333-8333-333333333333"

func do(t *testing.T, values map[string]string, line string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(context.Background(), strings.Fields(line), func(n string) string { return values[n] }, &out)
	return out.String(), err
}

func TestUsageErrorsAreFixedAndPrintNothing(t *testing.T) {
	for _, line := range []string{"", "bogus", "register extra", "register --nope=sk_test_leak", "rotate --expected-version=notanumber",
		"method --sort=abc"} {
		out, err := do(t, env(), line)
		if !errors.Is(err, errUsage) || out != "" || strings.Contains(err.Error(), "leak") || strings.Contains(err.Error(), "abc") {
			t.Fatalf("%q -> %q %v", line, out, err)
		}
	}
}

func TestEnvironmentGatesBeforeAnyConnection(t *testing.T) {
	cases := []struct {
		name, line string
		mutate     func(map[string]string)
	}{
		{"missing dsn", "register " + ids, func(v map[string]string) { delete(v, "COMMERCE_STRIPE_REGISTRAR_DATABASE_URL") }},
		{"register without api keyring", "register " + ids, func(v map[string]string) { delete(v, "COMMERCE_ACCOUNT_KEYS_JSON") }},
		{"rotate without api keyring", "rotate " + ids + " --connection x --expected-version 1", func(v map[string]string) { delete(v, "COMMERCE_ACCOUNT_REPLAY_KEY") }},
		{"webhook without signing keyring", "webhook " + ids + " --connection x --profile SANDBOX", func(v map[string]string) { delete(v, "COMMERCE_STRIPE_WEBHOOK_KEYS_JSON") }},
		{"webhook with api keyring names only", "webhook " + ids + " --connection x --profile SANDBOX", func(v map[string]string) {
			for k := range v {
				if strings.HasPrefix(k, "COMMERCE_STRIPE_WEBHOOK_") {
					delete(v, k)
				}
			}
		}},
		{"sandbox qualify without opt-in", "qualify " + ids + " --profile SANDBOX --connection x --expected-version 1", func(v map[string]string) {}},
	}
	for _, tc := range cases {
		v := env()
		tc.mutate(v)
		out, err := do(t, v, tc.line)
		if !errors.Is(err, errConfig) || out != "" {
			t.Fatalf("%s: %q %v", tc.name, out, err)
		}
	}
}

func TestValidEnvironmentReachesMaskedDatabaseError(t *testing.T) {
	for _, line := range []string{"register " + ids, "rotate " + ids + " --connection c --expected-version 1",
		"webhook " + ids + " --connection c --profile SANDBOX --enabled", "qualify " + ids + " --profile PROVIDER_MOCK --connection c --expected-version 1",
		"method " + ids + " --market m --country HK"} {
		out, err := do(t, env(), line)
		if !errors.Is(err, stripeadmin.ErrDatabase) || out != "" {
			t.Fatalf("%q: %q %v", line, out, err)
		}
		for _, banned := range []string{"pw-secret", "127.0.0.1", "sk_", "whsec"} {
			if strings.Contains(err.Error(), banned) {
				t.Fatalf("error leaked %s", banned)
			}
		}
	}
}
