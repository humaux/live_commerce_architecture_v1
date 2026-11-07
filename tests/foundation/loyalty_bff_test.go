// Purpose: LP-1 real Node BFF -> Go HTTP -> PostgreSQL seam acceptance.
// Depends on: existing lpNewStore fixture, runtime role, Node route loader and loopback HTTP.
// Used by: focused D2 gate; synthetic sessions stay in child environment, never logs/argv.
package foundation_test

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
)

func TestLoyaltyBFFRealPG(t *testing.T) {
	s := lpNewStore(t)
	var capture bytes.Buffer
	var mu sync.Mutex
	previous, previousSlog := log.Writer(), slog.Default()
	log.SetOutput(&capture)
	slog.SetDefault(slog.New(slog.NewTextHandler(&capture, nil)))
	defer func() { log.SetOutput(previous); slog.SetDefault(previousSlog) }()
	handler := httpapi.NewHandler(s.f.runtime, httpapi.Options{SessionStoreList: true})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capture.WriteString(r.URL.RequestURI() + "\n")
		mu.Unlock()
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--experimental-strip-types", "../admin/members-real-pg.mjs")
	cmd.Env = append(os.Environ(), "LP_BFF_ORIGIN="+server.URL, "LP_BFF_STORE="+s.store, "LP_BFF_SESSION="+s.admin, "LP_BFF_READONLY="+s.principal("customers:read"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real BFF seam failed: %v\n%s", err, output)
	} else {
		t.Log(string(output))
		mu.Lock()
		defer mu.Unlock()
		for _, marker := range []string{"測試會員 seam", "synthetic contact"} {
			if strings.Contains(capture.String(), marker) {
				t.Fatal("member PII leaked into URL or Go logs")
			}
		}
	}
}
