// Purpose: PR24 actual REAL_PG SweepOnce sequential multi-asset request-clock acceptance.
// Depends on: LCN real Page-token/lease fixtures, injected ConsoleConfig.Now and MOCK Graph transport.
// Used by: LCN01 focused/foundation gates; logical delays only, no provider traffic or clock sleeps.
package foundation_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"livecommerce/internal/integrations/metareply"
)

type lcnClockTrip func(*http.Request) (*http.Response, error)

func (f lcnClockTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLiveConsoleLCN01SequentialClockBudget(t *testing.T) {
	for _, mode := range []string{"429", "usage80"} {
		t.Run(mode, func(t *testing.T) {
			e := lcnSetup(t)
			asset := miAsset()
			binding := miBinding(t, e.m, asset, "facebook", e.f.tenantA, e.f.storeA1, e.f.principalA)
			miRoute(t, e.m, asset, e.f.tenantA, e.f.storeA1, binding)
			second := e.mustSource(t, e.session, asset, asset+"_"+mciDigits(10), true)
			if _, err := metareply.RegisterPageToken(context.Background(), e.m.registrar, e.pageKeys, metareply.Registration{
				TenantID: e.f.tenantA, StoreID: e.f.storeA1, PrincipalID: e.h.actor, BindingID: binding, Provider: "facebook",
				AssetID: asset, ExpectedVersion: 0, Scopes: []string{"pages_read_engagement"}}, e.pageToken); err != nil {
				t.Fatal(err)
			}
			var clock atomic.Int64
			clock.Store(time.Now().UnixNano())
			readClock := func() time.Time { return time.Unix(0, clock.Load()) }
			firstPath, secondPath := "", ""
			counts := map[string]int{}
			client := &http.Client{Transport: lcnClockTrip(func(r *http.Request) (*http.Response, error) {
				if !strings.HasSuffix(r.URL.Path, "/comments") || r.URL.Query().Has("access_token") || r.Header.Get("Authorization") != "Bearer "+e.pageToken {
					t.Fatal("synthetic forward-read/token fixture violated")
				}
				path := r.URL.Path
				if firstPath == "" {
					firstPath = path
				}
				counts[path]++
				body, status, header := `{"data":[]}`, 200, http.Header{}
				if path == firstPath {
					if counts[path] == 1 {
						clock.Add(int64(10 * time.Second))
					}
				} else {
					if secondPath == "" {
						secondPath = path
					}
					if path != secondPath {
						t.Fatal("unexpected third asset")
					}
					if counts[path] == 1 {
						clock.Add(int64(3 * time.Second))
						if mode == "429" {
							status, body = 429, `{"error":{"code":4}}`
						} else {
							header.Set("X-App-Usage", `{"call_count":80}`)
						}
					}
				}
				return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			c := e.console(t, "worker-clock", metareply.ConsoleConfig{Now: readClock, PollInterval: 2 * time.Second, Graph: metareply.Config{HTTPClient: client}})
			// Actual console page demand keeps both buffers out of the intentional idle-drop path.
			for _, source := range []string{e.sourceID, second} {
				if status, code, _ := e.page(t, c.Handler(), source, nil, nil, 50); status != http.StatusOK {
					t.Fatalf("source demand status=%d code=%s", status, code)
				}
			}
			c.SweepOnce(context.Background())
			completed := readClock()
			if firstPath == "" || secondPath == "" || firstPath == secondPath || counts[secondPath] != 1 {
				t.Fatal("two real sequential source polls not exercised")
			}
			for _, source := range []string{e.sourceID, second} {
				_, _, owned := e.lease(t, source)
				if !owned {
					t.Fatal("actual source lease was not acquired")
				}
			}
			delay := 2 * time.Second
			if mode == "usage80" {
				delay = 15 * time.Second
			}
			clock.Store(completed.Add(delay - time.Nanosecond).UnixNano())
			c.SweepOnce(context.Background())
			if counts[secondPath] != 1 {
				t.Fatal("later asset re-polled inside its full post-completion wait")
			}
			clock.Store(completed.Add(delay).UnixNano())
			c.SweepOnce(context.Background())
			if counts[secondPath] != 2 {
				t.Fatal("later asset did not reopen at its actual completion deadline")
			}
		})
	}
}
