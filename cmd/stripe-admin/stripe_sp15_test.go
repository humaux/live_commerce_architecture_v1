package main

// SP15 no-PG half for cmd/stripe-admin (contracts/stripe-psp-v1.md §12/§13; frozen
// seam: func run(ctx, args, getenv, stdout) error in stripe-b1-ingress-assembly).
// The CLI is the only reader of STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET[_NEXT],
// and each subcommand reads only its own variables. The exact flag names of
// `method` are not frozen in the brief; a wrong guess degrades that case to the
// flag-parse path (still proving no leak) and is reported as an ambiguity.

import (
	"bytes"
	"context"
	"encoding/base64"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const (
	adminDSN    = "postgres://registrar-sentinel:pw-sentinel@127.0.0.1:1/x"
	adminSecret = "sk_test_sentinel0123456789abcdef"
	adminWhsec  = "whsec_sentinel0123456789abcdef"
	adminWhNext = "whsec_sentinel_next_0123456789"
	uuidA       = "3f2b8c1e-0d4a-4b6f-9a7e-5c1d2e3f4a5b"
	uuidB       = "4a3c9d2f-1e5b-4c70-8b8f-6d2e3f4a5b6c"
	uuidC       = "5b4d0e3a-2f6c-4d81-9c90-7e3f4a5b6c7d"
)

func adminEnv() map[string]string {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	replay := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))
	ring := `[{"id":"k1","key_base64":"` + key + `"}]`
	return map[string]string{
		"COMMERCE_STRIPE_REGISTRAR_DATABASE_URL": adminDSN,
		"COMMERCE_ACCOUNT_ACTIVE_KEY_ID":         "k1", "COMMERCE_ACCOUNT_KEYS_JSON": ring, "COMMERCE_ACCOUNT_REPLAY_KEY": replay,
		"COMMERCE_STRIPE_WEBHOOK_ACTIVE_KEY_ID": "k1", "COMMERCE_STRIPE_WEBHOOK_KEYS_JSON": ring, "COMMERCE_STRIPE_WEBHOOK_REPLAY_KEY": replay,
		"STRIPE_SECRET_KEY": adminSecret, "STRIPE_ACCOUNT_ID": "acct_SentinelAcct1",
		"STRIPE_WEBHOOK_SECRET": adminWhsec, "STRIPE_WEBHOOK_SECRET_NEXT": adminWhNext,
	}
}

func adminCommon() []string {
	return []string{"--tenant", uuidA, "--store", uuidB, "--principal", uuidC}
}

func adminArgs(sub string, extra ...string) []string {
	return append(append([]string{sub}, adminCommon()...), extra...)
}

func adminCases() map[string][]string {
	return map[string][]string{
		"register": adminArgs("register"),
		"rotate":   adminArgs("rotate", "--connection", uuidA, "--expected-version", "1"),
		"webhook":  adminArgs("webhook", "--connection", uuidA, "--profile", "PROVIDER_MOCK", "--expected-version", "0", "--enabled=true"),
		"qualify": adminArgs("qualify", "--connection", uuidA, "--expected-version", "1", "--profile", "SANDBOX", "--currency", "TWD",
			"--amount-minor", "100", "--return-url", "https://checkout.example.test/payment/return"),
		"method": adminArgs("method", "--market", uuidA, "--country", "TW", "--connection", uuidB, "--qualification", uuidC, "--expected-version", "0",
			"--enabled=true", "--visible=true", "--sort", "1", "--min-minor", "100", "--max-minor", "99999900", "--name-hans", "Stripe", "--name-hant", "Stripe", "--name-en", "Stripe"),
	}
}

func set(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

// each subcommand may read only its own variables (contract §12 table + brief env list).
var adminAllowed = map[string]map[string]bool{
	"register": set("COMMERCE_STRIPE_REGISTRAR_DATABASE_URL", "COMMERCE_ACCOUNT_ACTIVE_KEY_ID", "COMMERCE_ACCOUNT_KEYS_JSON", "COMMERCE_ACCOUNT_REPLAY_KEY", "STRIPE_SECRET_KEY", "STRIPE_ACCOUNT_ID"),
	"rotate":   set("COMMERCE_STRIPE_REGISTRAR_DATABASE_URL", "COMMERCE_ACCOUNT_ACTIVE_KEY_ID", "COMMERCE_ACCOUNT_KEYS_JSON", "COMMERCE_ACCOUNT_REPLAY_KEY", "STRIPE_SECRET_KEY", "STRIPE_ACCOUNT_ID"),
	"webhook":  set("COMMERCE_STRIPE_REGISTRAR_DATABASE_URL", "COMMERCE_STRIPE_WEBHOOK_ACTIVE_KEY_ID", "COMMERCE_STRIPE_WEBHOOK_KEYS_JSON", "COMMERCE_STRIPE_WEBHOOK_REPLAY_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_WEBHOOK_SECRET_NEXT"),
	// qualify/method seal nothing, so they may need the registrar pool and (qualify SANDBOX) the API key + account only.
	"qualify": set("COMMERCE_STRIPE_REGISTRAR_DATABASE_URL", "STRIPE_SECRET_KEY", "STRIPE_ACCOUNT_ID", "COMMERCE_ACCOUNT_ACTIVE_KEY_ID", "COMMERCE_ACCOUNT_KEYS_JSON", "COMMERCE_ACCOUNT_REPLAY_KEY",
		"COMMERCE_STRIPE_WEBHOOK_ACTIVE_KEY_ID", "COMMERCE_STRIPE_WEBHOOK_KEYS_JSON", "COMMERCE_STRIPE_WEBHOOK_REPLAY_KEY"),
	"method": set("COMMERCE_STRIPE_REGISTRAR_DATABASE_URL", "COMMERCE_ACCOUNT_ACTIVE_KEY_ID", "COMMERCE_ACCOUNT_KEYS_JSON", "COMMERCE_ACCOUNT_REPLAY_KEY",
		"COMMERCE_STRIPE_WEBHOOK_ACTIVE_KEY_ID", "COMMERCE_STRIPE_WEBHOOK_KEYS_JSON", "COMMERCE_STRIPE_WEBHOOK_REPLAY_KEY"),
}

func TestStripeSP15Process(t *testing.T) {
	ctx := context.Background()
	sentinels := []string{"registrar-sentinel", "pw-sentinel", adminSecret, adminWhsec, adminWhNext, "acct_SentinelAcct1"}

	t.Run("no_args_and_unknown_subcommand_read_nothing_and_print_nothing", func(t *testing.T) {
		for name, args := range map[string][]string{"none": nil, "unknown": {"bogus"}, "help_flag": {"--help"}, "empty": {""}} {
			var out bytes.Buffer
			read := []string{}
			err := run(ctx, args, func(n string) string { read = append(read, n); return adminEnv()[n] }, &out)
			if err == nil || len(read) != 0 || out.Len() != 0 {
				t.Fatalf("%s: err=%v reads=%v stdout=%q", name, err, read, out.String())
			}
		}
	})

	t.Run("each_subcommand_reads_only_its_own_variables_and_leaks_nothing", func(t *testing.T) {
		for name, args := range adminCases() {
			var out bytes.Buffer
			read := []string{}
			env := adminEnv()
			err := run(ctx, args, func(n string) string { read = append(read, n); return env[n] }, &out)
			if err == nil { // the registrar DSN points at a closed port: success is impossible
				t.Fatalf("%s: run succeeded without a database", name)
			}
			for _, n := range read {
				if !adminAllowed[name][n] {
					t.Fatalf("%s read %q", name, n)
				}
			}
			if out.Len() != 0 {
				t.Fatalf("%s wrote to stdout on failure: %q", name, out.String())
			}
			for _, s := range sentinels {
				if strings.Contains(err.Error(), s) {
					t.Fatalf("%s leaked a secret in its error", name)
				}
			}
			if strings.Contains(err.Error(), "postgres://") {
				t.Fatalf("%s leaked the DSN", name)
			}
		}
	})

	t.Run("register_and_rotate_never_read_signing_material_and_webhook_never_reads_api_keys", func(t *testing.T) {
		env := adminEnv()
		for _, tc := range []struct {
			cmd       string
			forbidden []string
		}{
			{"register", []string{"STRIPE_WEBHOOK_SECRET", "STRIPE_WEBHOOK_SECRET_NEXT"}},
			{"rotate", []string{"STRIPE_WEBHOOK_SECRET", "STRIPE_WEBHOOK_SECRET_NEXT"}},
			{"webhook", []string{"STRIPE_SECRET_KEY", "STRIPE_ACCOUNT_ID"}},
			{"method", []string{"STRIPE_SECRET_KEY", "STRIPE_ACCOUNT_ID", "STRIPE_WEBHOOK_SECRET", "STRIPE_WEBHOOK_SECRET_NEXT"}},
			{"qualify", []string{"STRIPE_WEBHOOK_SECRET", "STRIPE_WEBHOOK_SECRET_NEXT"}},
		} {
			read := []string{}
			_ = run(ctx, adminCases()[tc.cmd], func(n string) string { read = append(read, n); return env[n] }, &bytes.Buffer{})
			for _, n := range read {
				for _, f := range tc.forbidden {
					if n == f {
						t.Fatalf("%s read %s", tc.cmd, f)
					}
				}
			}
		}
	})

	t.Run("live_is_refused_by_the_cli", func(t *testing.T) {
		env := adminEnv()
		env["STRIPE_SECRET_KEY"] = "sk_live_sentinel0123456789abcdef"
		var out bytes.Buffer
		if err := run(ctx, adminCases()["register"], func(n string) string { return env[n] }, &out); err == nil || out.Len() != 0 || strings.Contains(err.Error(), "sk_live_") {
			t.Fatalf("register with a live key: %v %q", err, out.String())
		}
		swap := func(args []string, from, to string) []string {
			out := append([]string{}, args...)
			for i, a := range out {
				if a == from {
					out[i] = to
				}
			}
			return out
		}
		for name, args := range map[string][]string{
			"qualify_live": swap(adminCases()["qualify"], "SANDBOX", "LIVE"),
			"webhook_live": swap(adminCases()["webhook"], "PROVIDER_MOCK", "LIVE"),
		} {
			out.Reset()
			if err := run(ctx, args, func(n string) string { return adminEnv()[n] }, &out); err == nil || out.Len() != 0 {
				t.Fatalf("%s: %v %q", name, err, out.String())
			}
		}
	})

	t.Run("bad_flags_and_ids_fail_without_leaking", func(t *testing.T) {
		env := adminEnv()
		for name, args := range map[string][]string{
			"missing_tenant":     {"register", "--store", uuidB, "--principal", uuidC},
			"bad_uuid":           {"register", "--tenant", "not-a-uuid", "--store", uuidB, "--principal", uuidC},
			"uppercase_uuid":     {"register", "--tenant", strings.ToUpper(uuidA), "--store", uuidB, "--principal", uuidC},
			"unknown_flag":       adminArgs("register", "--password", "sentinel-flag-value"),
			"negative_version":   adminArgs("rotate", "--connection", uuidA, "--expected-version", "-1"),
			"non_numeric":        adminArgs("rotate", "--connection", uuidA, "--expected-version", "one"),
			"positional_garbage": adminArgs("register", "extra-positional"),
		} {
			var out bytes.Buffer
			err := run(ctx, args, func(n string) string { return env[n] }, &out)
			if err == nil || out.Len() != 0 || strings.Contains(err.Error(), "sentinel") || strings.Contains(err.Error(), "postgres://") {
				t.Fatalf("%s: err=%v stdout=%q", name, err, out.String())
			}
		}
	})

	t.Run("source_guard_stripe_secrets_only_in_the_registrar_and_the_adapter", func(t *testing.T) {
		// String literals in production Go files only (a comment naming a variable reads nothing).
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		envName := regexp.MustCompile(`^STRIPE_[A-Z0-9_]+$`)
		type rule struct {
			name    string
			match   func(string) bool
			allowed []string // repo-relative directories that may hold such a literal
		}
		rules := []rule{
			{"a STRIPE_* environment name", envName.MatchString, []string{"cmd/stripe-admin", "internal/integrations/psp/stripe"}},
			{"COMMERCE_STRIPE_REGISTRAR_DATABASE_URL", func(v string) bool { return v == "COMMERCE_STRIPE_REGISTRAR_DATABASE_URL" }, []string{"cmd/stripe-admin"}},
			{"a signing-keyring name", func(v string) bool {
				return strings.HasPrefix(v, "COMMERCE_STRIPE_WEBHOOK_") && v != "COMMERCE_STRIPE_WEBHOOK_ENABLED"
			},
				[]string{"cmd/api", "cmd/stripe-admin"}},
		}
		checked := 0
		fset := token.NewFileSet()
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			if d.IsDir() {
				switch d.Name() {
				case ".git", "node_modules", "output", ".worktrees", "apps", "docs", "contracts":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			checked++
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				for _, r := range rules {
					if !r.match(value) {
						continue
					}
					allowed := false
					for _, dir := range r.allowed {
						allowed = allowed || strings.HasPrefix(rel, dir+string(filepath.Separator))
					}
					if !allowed {
						t.Errorf("%s holds %s outside %v", rel, r.name, r.allowed)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if checked < 50 {
			t.Fatalf("source guard walked only %d files; wrong root?", checked)
		}
	})
}
