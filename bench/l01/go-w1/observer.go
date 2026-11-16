// Purpose: Package w1 measures a fixed, bounded Go signed-webhook pilot.
// Depends on: standard library HTTP, HMAC, JSON and monotonic time; no provider calls.
// Used by: TestL01GoW1's existing compiled API / isolated PG fixture bridge.
// Owns: bounded observation collection and verdict accounting for synthetic W1 requests.
// Invariants: I01, I04, I11, I18, I23; never infers business correctness from ACK.
// Package w1 records bounded observations; it never implements webhook business logic.
package w1

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

const (
	// Attempts freezes the per-block offered load, including negative controls.
	Attempts = 100
	// Window is the independently scheduled arrival window, not completion time.
	Window = 5 * time.Second
	// Concurrency bounds owned requests; arrivals never wait for a free slot.
	Concurrency = 8
	// Deadline bounds each request, including response body reads.
	Deadline = 2 * time.Second
)

// Request is synthetic input held only in memory; it is never serialized.
type Request struct {
	Body         []byte
	BadSignature bool
	TenantHeader string
}

// Attempt is the artifact wire shape: nanoseconds relative to block start, no payloads.
type Attempt struct {
	Index          int    `json:"index"`
	PlannedNS      int64  `json:"planned_ns"`
	StartedNS      int64  `json:"started_ns"`
	CompletedNS    int64  `json:"completed_ns"`
	LatencyNS      int64  `json:"latency_ns"`
	Status         int    `json:"status"`
	ExpectedStatus int    `json:"expected_status"`
	Started        bool   `json:"started"`
	Completed      bool   `json:"completed"`
	Error          string `json:"error_class"`
}

// Block contains every offered attempt and ACK-only latency quantiles.
// Success denominator is all planned attempts; expected 403 is a control, not an ACK.
type Block struct {
	Seed      int64     `json:"seed"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Attempts  []Attempt `json:"attempts"`
	Planned   int       `json:"planned"`
	Started   int       `json:"started"`
	Completed int       `json:"completed"`
	Rejected  int       `json:"rejected"`
	Timeout   int       `json:"timeout"`
	Dropped   int       `json:"dropped"`
	Errors    int       `json:"errors"`
	ACKs      int       `json:"acks"`
	P50NS     int64     `json:"p50_ns"`
	P95NS     int64     `json:"p95_ns"`
	P99NS     int64     `json:"p99_ns"`
}

// Run posts only to an owned literal loopback address; it never retries a request.
// Caller owns fixture preparation, business assertions and persistence of returned data.
func Run(ctx context.Context, target, secret string, requests []Request, seed int64) (Block, error) {
	b := Block{Seed: seed}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path == "" {
		return b, fmt.Errorf("owned literal loopback HTTP target required")
	}
	if len(requests) != Attempts || secret == "" {
		return b, fmt.Errorf("fixed nonempty pilot input required")
	}
	transport := &http.Transport{Proxy: nil, MaxConnsPerHost: Concurrency, MaxIdleConnsPerHost: Concurrency, DialContext: (&net.Dialer{Timeout: Deadline}).DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: Deadline, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	rng := rand.New(rand.NewSource(seed))
	b.Attempts = make([]Attempt, Attempts)
	step := Window / time.Duration(Attempts)
	for i, r := range requests {
		want := http.StatusOK
		if r.BadSignature {
			want = http.StatusForbidden
		}
		b.Attempts[i] = Attempt{Index: i, PlannedNS: int64(time.Duration(i)*step + time.Duration(rng.Int63n(int64(step)))), ExpectedStatus: want}
	}
	b.Start = time.Now().UTC()
	clockStart := time.Now()
	slots := make(chan struct{}, Concurrency)
	var wg sync.WaitGroup
	for i := range b.Attempts {
		a := &b.Attempts[i]
		if ctx.Err() != nil {
			a.Error = "cancelled_drop"
			continue
		}
		timer := time.NewTimer(time.Until(clockStart.Add(time.Duration(a.PlannedNS))))
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			a.Error = "cancelled_drop"
			continue
		}
		// Late scheduler wakeups are reported as drops rather than burst-replayed load.
		if time.Since(clockStart)-time.Duration(a.PlannedNS) > 250*time.Millisecond {
			a.Error = "scheduler_drop"
			continue
		}
		select {
		case slots <- struct{}{}:
		default:
			a.Error = "capacity_drop"
			continue
		}
		wg.Add(1)
		go func(a *Attempt, input Request) {
			defer wg.Done()
			defer func() { <-slots }()
			a.Started = true
			a.StartedNS = int64(time.Since(clockStart))
			defer func() {
				a.CompletedNS = int64(time.Since(clockStart))
				a.LatencyNS = a.CompletedNS - a.StartedNS
				a.Completed = true
			}()
			reqCtx, cancel := context.WithTimeout(ctx, Deadline)
			defer cancel()
			req, e := http.NewRequestWithContext(reqCtx, http.MethodPost, target, bytes.NewReader(input.Body))
			if e != nil {
				a.Error = "request"
				return
			}
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write(input.Body)
			signature := hex.EncodeToString(mac.Sum(nil))
			if input.BadSignature {
				signature = "0000000000000000000000000000000000000000000000000000000000000000"
			}
			req.Header.Set("X-Hub-Signature-256", "sha256="+signature)
			req.Header.Set("Content-Type", "application/json")
			if input.TenantHeader != "" {
				req.Header.Set("X-Tenant-ID", input.TenantHeader)
			}
			// Calls the production Meta HTTP route (meta-inbox-v1); never a public provider.
			resp, e := client.Do(req)
			if e != nil {
				if reqCtx.Err() != nil {
					a.Error = "timeout"
				} else {
					a.Error = "transport"
				}
				return
			}
			defer resp.Body.Close()
			a.Status = resp.StatusCode
			body, e := io.ReadAll(io.LimitReader(resp.Body, 513))
			if e != nil {
				if reqCtx.Err() != nil {
					a.Error = "timeout"
				} else {
					a.Error = "body_read"
				}
				return
			}
			if a.Status != a.ExpectedStatus {
				a.Error = "unexpected_status"
				return
			}
			if (a.Status == 200 && string(body) != "EVENT_RECEIVED") || len(body) > 512 {
				a.Error = "invalid_ack"
			}
		}(a, requests[i])
	}
	wg.Wait()
	b.End = time.Now().UTC()
	summarize(&b)
	return b, nil
}

func summarize(b *Block) {
	b.Planned = len(b.Attempts)
	var latencies []int64
	for _, a := range b.Attempts {
		if a.Started {
			b.Started++
		} else {
			b.Dropped++
		}
		if a.Completed {
			b.Completed++
		}
		if a.Status >= 400 {
			b.Rejected++
		}
		if a.Error == "timeout" {
			b.Timeout++
		}
		if a.Error != "" {
			b.Errors++
		}
		if a.Completed && a.Status == 200 && a.Error == "" {
			b.ACKs++
			latencies = append(latencies, a.LatencyNS)
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	if len(latencies) > 0 {
		q := func(p float64) int64 { return latencies[int(math.Ceil(p*float64(len(latencies))))-1] }
		b.P50NS, b.P95NS, b.P99NS = q(.50), q(.95), q(.99)
	}
}

// Validate rejects empty, incomplete, erroneous or malformed observation data.
// It is a measurement gate only; SQL correctness remains an independent prerequisite.
func Validate(b Block) error {
	if len(b.Attempts) != Attempts || b.Start.IsZero() || !b.End.After(b.Start) {
		return fmt.Errorf("invalid block envelope")
	}
	var last int64 = -1
	for i, a := range b.Attempts {
		if a.Index != i || a.PlannedNS <= last || a.PlannedNS >= int64(Window) || !a.Started || !a.Completed || a.Error != "" || a.LatencyNS <= 0 || a.StartedNS < a.PlannedNS || a.CompletedNS < a.StartedNS || a.LatencyNS != a.CompletedNS-a.StartedNS || a.Status != a.ExpectedStatus || (a.ExpectedStatus != 200 && a.ExpectedStatus != 403) {
			return fmt.Errorf("invalid attempt %d", i)
		}
		last = a.PlannedNS
	}
	expected := Block{Attempts: b.Attempts}
	summarize(&expected)
	if expected.Planned != b.Planned || expected.Started != b.Started || expected.Completed != b.Completed || expected.Rejected != b.Rejected || expected.Timeout != b.Timeout || expected.Dropped != b.Dropped || expected.Errors != b.Errors || expected.ACKs != b.ACKs || expected.P50NS != b.P50NS || expected.P95NS != b.P95NS || expected.P99NS != b.P99NS || b.ACKs == 0 || b.Errors != 0 || b.P50NS <= 0 || b.P50NS > b.P95NS || b.P95NS > b.P99NS {
		return fmt.Errorf("invalid counters/quantiles")
	}
	return nil
}
