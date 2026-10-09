// Purpose: PR24 sequential multi-asset request-clock and slot-admission regressions.
// Depends on: actual pollOne/Graph transport, injected ConsoleConfig.Now, shared budget and parse fences.
// Used by: metareply race gates; MOCK logical delays, no provider calls or real waiting.
package metareply

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	metaoauth "livecommerce/internal/integrations/meta/oauth"
)

func TestCommentSequentialAssetsUseActualCallClock(t *testing.T) {
	for _, mode := range []string{"429", "usage50", "usage80", "bad-forward", "bad-deletion"} {
		t.Run(mode, func(t *testing.T) {
			c, sweepAt := deletionConsole(t, &deletionGraph{})
			c.cfg.PollInterval = 2 * time.Second
			c.cfg.CallTimeout = 15 * time.Second // Both logical response delays fit the production per-call budget.
			var clock atomic.Int64
			clock.Store(sweepAt.UnixNano())
			c.cfg.Now = func() time.Time { return time.Unix(0, clock.Load()) }
			callsA, callsB := 0, 0
			graph, err := metaoauth.NewGraph("http://127.0.0.1:1", "v99.0", &http.Client{Transport: deletionRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Bearer deletion-token-sentinel" || r.URL.Query().Has("access_token") {
					t.Fatal("token transport changed")
				}
				body, status, header := `{"data":[]}`, 200, http.Header{}
				if strings.Contains(r.URL.Path, "asset-a_post") {
					callsA++
					if callsA == 1 {
						clock.Add(int64(10 * time.Second))
					}
				} else {
					callsB++
					if callsB == 1 {
						clock.Add(int64(3 * time.Second)) // B's own response delay also belongs before the deadline.
						switch mode {
						case "429":
							status, body = 429, `{"error":{"code":4}}`
						case "usage50":
							header.Set("X-App-Usage", `{"call_count":50}`)
						case "usage80":
							header.Set("X-App-Usage", `{"call_count":80}`)
						case "bad-forward":
							body = `{"data":{}}`
						default:
							body = `{"unexpected":true}`
						}
					}
				}
				return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			c.graph = graph
			a := deletionSource(c, "a", "asset-a", sweepAt)
			b := deletionSource(c, "b", "asset-b", sweepAt)
			if mode == "bad-deletion" {
				b = deletionSource(c, "b", "asset-b", sweepAt, "2_2")
			}
			// Same actual sequential pollOne/Graph chain as SweepOnce; B must observe A's elapsed logical time.
			c.pollOne(context.Background(), a)
			c.pollOne(context.Background(), b)
			completed := c.cfg.Now()
			delay := 2 * time.Second
			if mode == "usage50" {
				delay = 5 * time.Second
			}
			if mode == "usage80" {
				delay = 15 * time.Second
			}
			if callsA != 1 || callsB != 1 || !completed.Equal(sweepAt.Add(13*time.Second)) {
				t.Fatal("sequential delay fixture not exercised")
			}
			clock.Store(completed.Add(delay - time.Nanosecond).UnixNano())
			c.pollOne(context.Background(), b)
			if callsB != 1 {
				t.Fatalf("B called %d times before full completion + %s wait", callsB, delay)
			}
			clock.Store(completed.Add(delay).UnixNano())
			c.pollOne(context.Background(), b)
			if callsB != 2 {
				t.Fatalf("B did not reopen at exact completion + %s: calls=%d", delay, callsB)
			}
		})
	}
}

func TestCommentAdmissionFollowsInjectedClock(t *testing.T) {
	c, start := deletionConsole(t, &deletionGraph{})
	var clock atomic.Int64
	clock.Store(start.UnixNano())
	c.cfg.Now = func() time.Time { return time.Unix(0, clock.Load()) }
	c.cfg.PollInterval = 2 * time.Second
	c.rates = map[string]*consoleRate{"asset": {next: start.Add(2 * time.Second)}}
	s := deletionSource(c, "s", "asset", start)
	g := &deletionGraph{}
	other, _ := deletionConsole(t, g)
	c.graph = other.graph
	c.pollOne(context.Background(), s)
	if g.calls != 0 {
		t.Fatal("admission bypassed the actual clock's slot deadline")
	}
	clock.Store(start.Add(2 * time.Second).UnixNano())
	c.pollOne(context.Background(), s)
	if g.calls != 1 {
		t.Fatal("admission did not reopen at the actual clock's slot deadline")
	}
	if !c.rates["asset"].next.Equal(start.Add(4 * time.Second)) {
		t.Fatal("new slot was not anchored to actual admission")
	}
}
