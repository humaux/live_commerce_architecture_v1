// Purpose: pin production packaging to OA1 disabled until OA2 cancel/dispatch cutover.
// Depends on: the actual production Compose service environment overrides.
// Used by: foundation CI's structural deployment acceptance (E1, not process proof).
// Invariants: env-file or caller settings cannot enable API/claims execution implicitly.

package foundation_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This packaging pin complements the actual disabled claims-worker process lane
// in TestOrderAttemptWorkerEnabledProcessNativeReady. It does not run Compose or a
// process: native execution/cleanup remains that gate's independent obligation.
func TestOrderAttemptProductionActivationRemainsDisabled(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	compose := string(body)
	for _, service := range []string{"api", "claims-worker"} {
		t.Run(service, func(t *testing.T) {
			start := strings.Index(compose, "\n  "+service+":\n")
			if start < 0 {
				t.Fatalf("production service %s missing", service)
			}
			block := compose[start+1:]
			if next := regexp.MustCompile(`\n  [a-z][a-z0-9-]*:\n`).FindStringIndex(block[1:]); next != nil {
				block = block[:next[0]+1]
			}
			environment := strings.Index(block, "\n    environment:\n")
			if environment < 0 {
				t.Fatal("production service environment missing")
			}
			overrides := block[environment+1:]
			if next := regexp.MustCompile(`\n    [a-z][a-z0-9_]*:`).FindStringIndex(overrides[1:]); next != nil {
				overrides = overrides[:next[0]+1]
			}
			// Compose environment overrides env_file; only a literal zero blocks
			// deployment knobs from activating the unfinished OA2 surface.
			flag := regexp.MustCompile(`(?m)^      LC_ORDER_ATTEMPT_ENABLED: "0"[ \t]*(?:#.*)?$`)
			declarations := regexp.MustCompile(`(?m)^      LC_ORDER_ATTEMPT_ENABLED:`)
			if !flag.MatchString(overrides) || len(declarations.FindAllStringIndex(overrides, -1)) != 1 {
				t.Fatal("OA1 production activation must be overridden to literal zero until OA2 cutover")
			}
		})
	}
}
