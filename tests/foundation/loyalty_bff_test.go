// Purpose: LP-1 real Node BFF -> Go HTTP -> PostgreSQL seam acceptance.
// Depends on: existing lpNewStore fixture, runtime role, Node route loader and loopback HTTP.
// Used by: focused D2 gate; synthetic sessions stay in child environment, never logs/argv.
package foundation_test

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
)

func TestLoyaltyBFFRealPG(t *testing.T) {
	s := lpNewStore(t)
	server := httptest.NewServer(httpapi.NewHandler(s.f.runtime, httpapi.Options{SessionStoreList: true}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--experimental-strip-types", "../admin/members-real-pg.mjs")
	cmd.Env = append(os.Environ(), "LP_BFF_ORIGIN="+server.URL, "LP_BFF_STORE="+s.store, "LP_BFF_SESSION="+s.admin, "LP_BFF_READONLY="+s.principal("customers:read"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real BFF seam failed: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
