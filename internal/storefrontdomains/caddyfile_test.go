// caddyfile_test.go: the committed deploy Caddyfile must actually load on the pinned Caddy 2.11 image —
// the review (N-P0-1) found it failed `caddy adapt` on the removed on_demand_tls `interval`/`burst` knobs.
// This is a REAL parse, not a static grep: it runs `caddy validate` and `caddy adapt` in a throwaway
// container against the committed file (piped over stdin, so no host path must be Docker-shared) with dummy
// env, and asserts the catch-all site's automation policy is on_demand:true. Missing docker is a NOT_RUN
// skip — release-gate maps a `NOT_RUN:` skip to NOT_RUN, never to PASS.

package storefrontdomains

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

func TestCaddyfileLoadsOnPinnedCaddy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("NOT_RUN: the caddy gate runs the linux caddy:2.11.4 image")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("NOT_RUN: docker unavailable (deploy caddy adapt gate)")
	}
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "caddy", "Caddyfile"))
	if err != nil {
		t.Fatalf("read committed Caddyfile: %v", err)
	}
	// Caddyfile env placeholders are the only names Caddy serves; dummy values satisfy the adapt/validate pass.
	envArgs := []string{
		"-e", "ACME_EMAIL=gate@example.com",
		"-e", "LC_ADMIN_HOST=admin.localhost",
		"-e", "LC_STORE_HOST=shop.localhost",
		"-e", "LC_API_HOST=api.localhost",
		"-e", "LC_HOOKS_HOST=hooks.localhost",
	}
	run := func(args ...string) (string, error) {
		cmd := exec.Command("docker", append([]string{"run", "--rm", "--network", "none", "-i"}, append(envArgs, append([]string{"caddy:2.11.4"}, args...)...)...)...)
		cmd.Stdin = bytes.NewReader(body)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("caddy", "validate", "--config", "-", "--adapter", "caddyfile"); err != nil {
		t.Fatalf("caddy validate failed (exit %v):\n%s", err, out)
	}
	out, err := run("caddy", "adapt", "--config", "-", "--adapter", "caddyfile", "--pretty")
	if err != nil {
		t.Fatalf("caddy adapt failed (exit %v):\n%s", err, out)
	}
	// The catch-all automation policy is the boolean true; the global on_demand_tls ask is an object
	// ("on_demand": {...}), so the boolean true matches only the per-site `tls { on_demand }` directive.
	if !regexp.MustCompile(`"on_demand"\s*:\s*true`).MatchString(out) {
		t.Fatalf("adapted config lacks the on_demand:true automation policy:\n%s", out)
	}
}
