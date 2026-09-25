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

// Exercise the real private API assembly, including its unexported nested
// payment configuration. Reuse the existing isolated buyer fixture; never
// forward a developer's COMMERCE_* credentials to the child process.
func TestBuyerPaymentAPIAssemblyRealPoolCleanup(t *testing.T) {
	h := bhSetup(t)
	hostedDSN := hpRole(t, h.f)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-race", "-tags", "buyerintegration", "-count=1",
		"-run", "^TestBuyerPaymentPoolAssemblyRealPG$", "-v", "./cmd/api")
	cmd.Dir = root
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "COMMERCE_") || strings.HasPrefix(name, "LC_BUYER_") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "LC_BUYER_PAYMENT_ASSEMBLY_GATE=1",
		"LC_BUYER_TEST_ISSUER_DSN="+h.a.issuerURL, "LC_BUYER_TEST_RUNTIME_DSN="+h.a.runtimeURL,
		"LC_BUYER_TEST_CHECKOUT_DSN="+h.poolURL, "LC_BUYER_TEST_HOSTED_DSN="+hostedDSN)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("buyer payment API assembly gate failed: %s", output.String())
	}
	if !strings.Contains(output.String(), "--- PASS: TestBuyerPaymentPoolAssemblyRealPG") {
		t.Fatal("buyer payment API assembly child did not execute")
	}
	t.Log("actual cmd/api payment assembly and pg_stat_activity pool cleanup PASS; no provider request")
}
