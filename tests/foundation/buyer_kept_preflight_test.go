// Purpose: drive unchanged API factory retained-pool admission against two real clusters.
// Depends on: isolated mrFixture roles and buyerintegration cmd/api child gate.
// Used by: OA1 focused CI; no production settings or database are forwarded.
// Invariants: same database names cannot substitute for actual physical pool proof.

package foundation_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuyerKeptPoolPreflightActualFactory(t *testing.T) {
	f, clone := mrFixture(t), mrFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var primaryName, cloneName string
	if f.owner.QueryRow(ctx, `SELECT current_database()`).Scan(&primaryName) != nil ||
		clone.owner.QueryRow(ctx, `SELECT current_database()`).Scan(&cloneName) != nil || primaryName != cloneName {
		t.Fatal("retained-pool fixture requires equal names on distinct clusters")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-tags", "buyerintegration", "-count=1", "-run", "^TestBuyerKeptDatabasePreflightRealPG$", "-v", "./cmd/api")
	cmd.Dir = root
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "COMMERCE_") || strings.HasPrefix(name, "LC_BUYER_") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "LC_BUYER_KEPT_GATE=1", "LC_BUYER_KEPT_MAIN_DSN="+f.runtime.Config().ConnString(),
		"LC_BUYER_KEPT_OBSERVER_DSN="+f.owner.Config().ConnString(), "LC_BUYER_KEPT_CLONE_OBSERVER_DSN="+clone.owner.Config().ConnString())
	for _, role := range []struct{ name, authority string }{
		{"ISSUER", "commerce_buyer_issuer"}, {"RUNTIME", "commerce_buyer_runtime"}, {"CHECKOUT", "commerce_checkout_runtime"},
	} {
		cmd.Env = append(cmd.Env, "LC_BUYER_KEPT_"+role.name+"_DSN="+miRole(t, f, role.authority),
			"LC_BUYER_KEPT_CLONE_"+role.name+"_DSN="+miRole(t, clone, role.authority))
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("retained-pool child gate failed: %s", output.String())
	}
	if !strings.Contains(output.String(), "--- PASS: TestBuyerKeptDatabasePreflightRealPG") {
		t.Fatal("retained-pool child gate did not run")
	}
}
