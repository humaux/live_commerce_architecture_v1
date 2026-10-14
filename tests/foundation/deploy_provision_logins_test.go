// Purpose: pin deployed login-secret pairing and isolated A2 operator custody.
// Depends on: deployment manifests/Compose and actual API/claims-worker sources.
// Used by: foundation CI; native A2 tests separately prove the database authority.
package foundation_test

// Static deploy-packaging gate (no PG): every core login in deploy/postgres/logins.tsv must have its
// pw_<login> secret mounted into the provision-logins service and declared at top level in
// deploy/compose.yml, otherwise deploy.sh aborts after migrate with "missing secret pw_<login>".
// Found on the pilot host 2026-10-01: lc_ads_worker (meta-ads-v1) was added to logins.tsv without the
// compose mount; arm64 dev runs and the static smoke never started provision-logins against it.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDeployProvisionLoginsMountsEveryCoreSecret(t *testing.T) {
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join("..", "..", rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}
	compose := read("deploy/compose.yml")
	// The provision-logins service block: from its key to the next top-level service key.
	start := strings.Index(compose, "\n  provision-logins:\n")
	if start < 0 {
		t.Fatal("deploy/compose.yml has no provision-logins service")
	}
	block := compose[start+1:]
	if next := regexp.MustCompile(`\n  [a-z][a-z0-9-]*:\n`).FindStringIndex(block[1:]); next != nil {
		block = block[:next[0]+1]
	}
	for _, line := range strings.Split(read("deploy/postgres/logins.tsv"), "\n") {
		f := strings.Split(line, "\t")
		if strings.HasPrefix(line, "#") || len(f) < 4 || f[3] != "core" {
			continue
		}
		secret := "pw_" + f[0]
		if !regexp.MustCompile(`(?m)^\s+- ` + secret + `$`).MatchString(block) {
			t.Errorf("provision-logins does not mount %s (core login %s in logins.tsv)", secret, f[0])
		}
		if !regexp.MustCompile(`(?m)^  ` + secret + `: \{ file:`).MatchString(compose) {
			t.Errorf("deploy/compose.yml has no top-level secret %s", secret)
		}
	}
}

func TestDeployOrderAttemptKeyAdminIsExternalOnly(t *testing.T) {
	files := []string{"deploy/postgres/logins.tsv", "deploy/secrets.manifest.tsv", "deploy/compose.yml"}
	for _, dir := range []string{"cmd/api", "cmd/claims-worker"} {
		entries, err := filepath.Glob(filepath.Join("..", "..", dir, "*.go"))
		if err != nil || len(entries) < 2 {
			t.Fatalf("missing actual process source inventory for %s: %v", dir, err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry, "_test.go") {
				files = append(files, filepath.Join(dir, filepath.Base(entry)))
			}
		}
	}
	for _, file := range files {
		body, err := os.ReadFile(filepath.Join("..", "..", file))
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"lc_order_attempt_key_admin", "COMMERCE_ORDER_ATTEMPT_KEY_ADMIN_DATABASE_URL", "ops.retire_order_attempt_plan_key"} {
			if strings.Contains(string(body), forbidden) {
				t.Errorf("%s exposes external-only A2 retirement custody/entry (%s)", file, forbidden)
			}
		}
	}
}
