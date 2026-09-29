// main_test.go: usage, environment gating and secret-hygiene tests for the Meta registrar CLI (MOCK tier).
// Non-goal: the registry SQL (REAL_PG, tests/foundation) and any Graph call.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/integrations/metareply"
)

const fakeToken = "EAAB" + "cli-sentinel-page-token-0123456789"

func env() map[string]string {
	k := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))
	return map[string]string{
		"COMMERCE_META_REGISTRAR_DATABASE_URL":   "postgres://operator:" + "pw-secret@127.0.0.1:1/lc",
		"META_PAGE_ACCESS_TOKEN":                 fakeToken,
		"COMMERCE_META_PAGE_TOKEN_ACTIVE_KEY_ID": "pt-1",
		"COMMERCE_META_PAGE_TOKEN_KEYS_JSON":     `{"keys":[{"id":"pt-1","key_base64":"` + k + `"}]}`,
	}
}

const ids = "--tenant 11111111-1111-4111-8111-111111111111 --store 22222222-2222-4222-8222-222222222222 " +
	"--principal 33333333-3333-4333-8333-333333333333 --binding 44444444-4444-4444-8444-444444444444 " +
	"--provider facebook --asset 1234567890 --expected-version 0 --scopes pages_messaging"

func do(t *testing.T, values map[string]string, line string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(context.Background(), strings.Fields(line), func(n string) string { return values[n] }, &out)
	return out.String(), err
}

func TestUsageErrorsAreFixedAndPrintNothing(t *testing.T) {
	for _, line := range []string{"", "bogus", "page-token extra", "page-token --nope=EAAB_leak",
		"page-token --expected-version=abc", strings.Replace("page-token "+ids, "--provider facebook", "--provider tiktok", 1),
		strings.Replace("page-token "+ids, "--asset 1234567890", "--asset 12ab", 1),
		strings.Replace("page-token "+ids, "pages_messaging", "Pages-Messaging", 1),
		strings.Replace("page-token "+ids, "--expected-version 0", "--expected-version -1", 1),
		strings.Replace("page-token "+ids, "--tenant 11111111-1111-4111-8111-111111111111", "--tenant nope", 1)} {
		out, err := do(t, env(), line)
		if !errors.Is(err, errUsage) || out != "" || strings.Contains(err.Error(), "leak") || strings.Contains(err.Error(), "abc") {
			t.Fatalf("%q -> %q %v", line, out, err)
		}
	}
}

func TestEnvironmentGatesBeforeAnyConnection(t *testing.T) {
	saved := register
	defer func() { register = saved }()
	register = func(context.Context, string, *metareply.PageTokenKeyring, metareply.Registration, string) (int64, error) {
		t.Fatal("connected before the environment was valid")
		return 0, nil
	}
	for name, mutate := range map[string]func(map[string]string){
		"no dsn":     func(v map[string]string) { delete(v, "COMMERCE_META_REGISTRAR_DATABASE_URL") },
		"no token":   func(v map[string]string) { delete(v, "META_PAGE_ACCESS_TOKEN") },
		"no keyring": func(v map[string]string) { delete(v, "COMMERCE_META_PAGE_TOKEN_KEYS_JSON") },
		"bad active": func(v map[string]string) { v["COMMERCE_META_PAGE_TOKEN_ACTIVE_KEY_ID"] = "zz" },
	} {
		v := env()
		mutate(v)
		if out, err := do(t, v, "page-token "+ids); !errors.Is(err, errConfig) || out != "" {
			t.Fatalf("%s: %q %v", name, out, err)
		}
	}
}

func TestSuccessPrintsVersionOnly(t *testing.T) {
	saved := register
	defer func() { register = saved }()
	var got metareply.Registration
	var gotToken string
	register = func(_ context.Context, _ string, _ *metareply.PageTokenKeyring, r metareply.Registration, token string) (int64, error) {
		got, gotToken = r, token
		return 3, nil
	}
	out, err := do(t, env(), "page-token "+strings.Replace(strings.Replace(ids, "--expected-version 0", "--expected-version 2", 1), "pages_messaging", "pages_messaging,pages_read_engagement", 1))
	if err != nil || out != "{\"version\":3}\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if got.ExpectedVersion != 2 || len(got.Scopes) != 2 || got.Provider != "facebook" || got.AssetID != "1234567890" || gotToken != fakeToken {
		t.Fatalf("registration %+v", got)
	}
	if strings.Contains(out, fakeToken) {
		t.Fatal("token printed")
	}
}

func TestDatabaseFailuresAreMasked(t *testing.T) {
	for _, line := range []string{"page-token " + ids} {
		out, err := do(t, env(), line)
		if !errors.Is(err, errRegister) || out != "" { // connection refused surfaces at the first query, masked
			t.Fatalf("%q: %q %v", line, out, err)
		}
		for _, banned := range []string{"pw-secret", "127.0.0.1", "operator", fakeToken} {
			if strings.Contains(err.Error(), banned) {
				t.Fatalf("error leaked %s", banned)
			}
		}
	}
	// A malformed DSN must not echo itself either.
	v := env()
	v["COMMERCE_META_REGISTRAR_DATABASE_URL"] = "postgres://operator:" + "pw-secret@[bad"
	if _, err := do(t, v, "page-token "+ids); !errors.Is(err, errDatabase) || strings.Contains(err.Error(), "pw-secret") {
		t.Fatalf("parse error not masked: %v", err)
	}
}
