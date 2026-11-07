//go:build browser

// Purpose: execute the actual storefront child environment and Node activation without PG or browser.
// Depends on: storefrontBrowserCommand, Node, and the actual shop-real-gate activation declaration.
// Used by: focused browser-tag Go unit checks; secrets must never reach child serialization.
package foundation_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorefrontObserverChildEnvironment(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal("root unavailable")
	}
	for _, tc := range []struct {
		name, value string
		absent      bool
	}{
		{"absent", "", true}, {"empty", "", false}, {"invalid", "invalid", false}, {"zero", "0", false}, {"one", "1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LC_C7_RSC_DIAGNOSTIC", tc.value)
			if tc.absent {
				if err := os.Unsetenv("LC_C7_RSC_DIAGNOSTIC"); err != nil {
					t.Fatal("cannot unset diagnostic opt-in")
				}
			}
			// The legacy alias must not override this launcher's normalized authority.
			t.Setenv("C7_RSC_DIAGNOSTIC", "1")
			for _, key := range []string{"LC_PRIVATE_CANARY", "COMMERCE_PRIVATE_CANARY", "DATABASE_URL", "POSTGRES_PASSWORD"} {
				t.Setenv(key, "synthetic-private-c7-5819")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := storefrontBrowserCommand(ctx, root, map[string]string{"LC_SFR_FACTS": "fixture-authorized"})
			// Replace only the heavyweight driver entry with its exact activation declaration; preserve actual cmd.Env.
			cmd.Args = []string{"node", "--input-type=module", "-e", `
import {readFileSync} from 'node:fs'; import vm from 'node:vm';
const s=readFileSync('tests/storefront/shop-real-gate.mjs','utf8');
const a=s.indexOf('const rscDiagnostic ='), b=s.indexOf('const observedAt',a);
if(a<0||b<a) process.exit(21);
const active=vm.runInNewContext(s.slice(a,b)+'; rscDiagnostic !== null', {process});
const want=process.argv[1]==='1';
if(active!==want) process.exit(22);
for(const key of ['LC_PRIVATE_CANARY','COMMERCE_PRIVATE_CANARY','DATABASE_URL','POSTGRES_PASSWORD']) if(process.env[key]) process.exit(23);
if(process.env.LC_C7_RSC_DIAGNOSTIC!== (want?'1':'0')) process.exit(24);
if(process.env.LC_SFR_FACTS!=='fixture-authorized') process.exit(25);
if(process.env.C7_RSC_DIAGNOSTIC!=='0') process.exit(27);
`, tc.value}
			cmd.Stdout, cmd.Stderr = nil, nil // Never print hostile values on failure.
			if err := cmd.Run(); err != nil {
				t.Fatalf("actual child contract failed: %v", err)
			}
		})
	}
}
