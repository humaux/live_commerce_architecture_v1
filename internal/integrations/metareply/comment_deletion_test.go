// Purpose: LC-B2-DEL actual poller counterexamples for deletion, uncertainty, pacing and fair batches.
// Depends on: Console/pollOne with an explicit test clock, shared Graph transport and loopback MOCK HTTP.
// Used by: metareply race tests; the separate LCN01 foundation test proves the real lease/bridge/A2 chain.
package metareply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"livecommerce/internal/integrations/core"
	metaoauth "livecommerce/internal/integrations/meta/oauth"
)

type deletionGraph struct {
	mu      sync.Mutex
	batches [][]string
	calls   int
	missing map[string]bool
	status  int
	body    string
	header  string
}

func (g *deletionGraph) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if r.Header.Get("Authorization") != "Bearer deletion-token-sentinel" || r.URL.Query().Has("access_token") || strings.Contains(r.URL.String(), "deletion-token-sentinel") {
		w.WriteHeader(401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if g.header != "" {
		w.Header().Set("X-App-Usage", g.header)
	}
	if r.URL.Query().Has("ids") {
		ids := strings.Split(r.URL.Query().Get("ids"), ",")
		g.batches = append(g.batches, ids)
		if r.URL.Query().Get("fields") != "id" || len(ids) > 50 {
			w.WriteHeader(400)
			return
		}
		if g.status != 0 {
			w.WriteHeader(g.status)
		}
		if g.body != "" {
			_, _ = w.Write([]byte(g.body))
			return
		}
		out := map[string]any{}
		for _, id := range ids {
			if !g.missing[id] {
				out[id] = map[string]string{"id": id}
			}
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/comments") {
		_, _ = w.Write([]byte(`{"data":[]}`))
		return
	}
	ref := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	_ = json.NewEncoder(w).Encode(map[string]any{"id": ref, "created_time": "2030-01-01T00:00:00Z", "from": map[string]string{"id": "42"}})
}
func deletionConsole(t *testing.T, g *deletionGraph) (*Console, time.Time) {
	t.Helper()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	graph, err := metaoauth.NewGraph(srv.URL, "v99.0", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	c := &Console{graph: graph, cfg: ConsoleConfig{PollInterval: 3 * time.Second, MaxBackoff: time.Minute, CallTimeout: time.Second, BufferAge: 2 * time.Hour, BufferCap: 2000}, sources: map[string]*consoleSource{}}
	return c, time.Date(2030, 1, 1, 0, 10, 0, 0, time.UTC)
}
func deletionSource(c *Console, source, asset string, now time.Time, refs ...string) *consoleSource {
	s := &consoleSource{source: source, assetID: asset, sourceObjectID: asset + "_post", byRef: map[string]consoleComment{}, owned: true, pollEpoch: 9, pollInterval: 3 * time.Second, token: core.NewSecret([]byte("deletion-token-sentinel"))}
	c.sources[source] = s
	for _, ref := range refs {
		s.seq++
		cc := consoleComment{BridgeComment: BridgeComment{Ref: ref, CreatedAt: now.Add(-time.Minute), Text: "synthetic comment"}, seq: s.seq}
		s.byRef[ref] = cc
		s.comments = append([]consoleComment{cc}, s.comments...)
	}
	return s
}
func TestCommentDeletionEvictsOnlyMissingAndKeepsSequence(t *testing.T) {
	g := &deletionGraph{missing: map[string]bool{"1_1": true}}
	c, now := deletionConsole(t, g)
	s := deletionSource(c, "source", "asset", now, "1_1", "1_2")
	c.pollOne(context.Background(), s, now)
	if _, ok := s.byRef["1_1"]; ok {
		t.Fatal("deleted comment still served")
	}
	if _, ok := s.byRef["1_2"]; !ok {
		t.Fatal("existing comment removed")
	}
	if s.seq != 2 || s.pollEpoch != 9 || len(s.comments) != 1 {
		t.Fatal("deletion changed sequence/epoch or failed ring eviction")
	}
	if len(g.batches) != 1 {
		t.Fatalf("batched checks=%d", len(g.batches))
	}
	c.pollOne(context.Background(), s, now.Add(59*time.Second))
	if len(g.batches) != 1 {
		t.Fatal("check ran before60s")
	}
	c.pollOne(context.Background(), s, now.Add(time.Minute))
	if len(g.batches) != 1 {
		t.Fatal("deletion bypassed the59s forward call's shared budget")
	}
	c.pollOne(context.Background(), s, now.Add(62*time.Second))
	if len(g.batches) != 2 {
		t.Fatal("due deletion did not resume when the shared budget reopened")
	}
}
func TestCommentDeletionUncertainResponseKeepsEveryEntry(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"429", 429, `{"error":{"code":4}}`}, {"5xx", 503, `{"error":{"code":2}}`}, {"malformed", 200, `{`},
		{"partial_error", 200, `{"1_2":{"error":{"code":17}}}`}, {"token", 200, `{"error":{"code":190}}`},
		{"null", 200, `null`}, {"array", 200, `[]`}, {"unknown", 200, `{"data":[]}`}, {"missing_id", 200, `{"1_2":{}}`},
		{"wrong_id", 200, `{"1_2":{"id":"9_9"}}`}, {"extra_field", 200, `{"1_2":{"id":"1_2","message":"unexpected"}}`},
		{"duplicate_key", 200, `{"1_2":{"id":"1_2"},"1_2":{"id":"1_2"}}`}, {"duplicate_field", 200, `{"1_2":{"id":"1_2","id":"1_2"}}`},
		{"trailing", 200, `{} {}`}, {"partial_content", 206, `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &deletionGraph{status: tc.status, body: tc.body}
			c, now := deletionConsole(t, g)
			s := deletionSource(c, "s", "asset", now, "1_1", "1_2")
			c.pollOne(context.Background(), s, now)
			if len(g.batches) != 1 {
				t.Fatal("uncertainty case did not execute actual batch")
			}
			if len(s.comments) != 2 || len(s.byRef) != 2 {
				t.Fatal("uncertain result evicted a comment")
			}
		})
	}
}
func TestCommentDeletionSharedAssetBudgetIncludesBridgeReads(t *testing.T) {
	g := &deletionGraph{}
	c, now := deletionConsole(t, g)
	a := deletionSource(c, "a", "asset", now, "1_1")
	b := deletionSource(c, "b", "asset", now, "2_2")
	c.pollOne(context.Background(), a, now)
	c.pollOne(context.Background(), b, now)
	_, _ = c.facts(context.Background(), b, "9_9", now)
	_, _, _ = c.graphOlder(context.Background(), b.sourceObjectID, b.assetID, "before", 50, b.token.Reveal())
	if g.calls != 1 {
		t.Fatalf("same asset spent %d calls in one slot", g.calls)
	}
	c.pollOne(context.Background(), b, now.Add(3*time.Second))
	if len(g.batches) != 2 || g.calls != 2 {
		t.Fatal("denied source did not receive the next shared slot")
	}
	other := deletionSource(c, "other", "other-asset", now, "3_3")
	c.pollOne(context.Background(), other, now.Add(3*time.Second))
	if g.calls != 3 {
		t.Fatal("independent asset budget coupled")
	}
}
func TestCommentDeletionSharedBackoff(t *testing.T) {
	for _, code := range []int{4, 17, 32, 613, 190} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			g := &deletionGraph{status: 429, body: fmt.Sprintf(`{"error":{"code":%d}}`, code)}
			c, now := deletionConsole(t, g)
			a := deletionSource(c, "a", "asset", now, "1_1")
			b := deletionSource(c, "b", "asset", now, "2_2")
			c.pollOne(context.Background(), a, now)
			c.pollOne(context.Background(), b, now.Add(time.Second))
			if g.calls != 1 {
				t.Fatal("source bypassed another source's Graph backoff")
			}
			if len(a.comments) != 1 || len(b.comments) != 1 {
				t.Fatal("backoff evicted comments")
			}
		})
	}
}
func TestCommentDeletionRotatesYoungEntriesInBoundedBatches(t *testing.T) {
	g := &deletionGraph{}
	c, now := deletionConsole(t, g)
	refs := []string{}
	for i := 0; i < 101; i++ {
		refs = append(refs, fmt.Sprintf("1_%d", i+1))
	}
	s := deletionSource(c, "s", "asset", now, refs...)
	old := consoleComment{BridgeComment: BridgeComment{Ref: "old_1", CreatedAt: now.Add(-31 * time.Minute)}, seq: 0}
	s.comments = append(s.comments, old)
	s.byRef[old.Ref] = old
	for i := 0; i < 3; i++ {
		c.pollOne(context.Background(), s, now.Add(time.Duration(i)*time.Minute))
	}
	if len(g.batches) != 3 {
		t.Fatalf("checks=%d", len(g.batches))
	}
	seen := map[string]bool{}
	for _, batch := range g.batches {
		if len(batch) > 50 {
			t.Fatal("batch exceeded50")
		}
		for _, id := range batch {
			if id == old.Ref {
				t.Fatal("old entry checked")
			}
			seen[id] = true
		}
	}
	if len(seen) != 101 {
		t.Fatalf("only%d/101 young entries rotated", len(seen))
	}
	if g.batches[0][0] != refs[len(refs)-1] {
		t.Fatal("first batch not newest-first")
	}
}
func TestCommentDeletionUsagePacesAllSources(t *testing.T) {
	g := &deletionGraph{header: `{"call_count":80,"total_time":2,"total_cputime":2}`}
	c, now := deletionConsole(t, g)
	a := deletionSource(c, "a", "asset", now, "1_1")
	b := deletionSource(c, "b", "asset", now, "2_2")
	started := time.Now()
	c.pollOne(context.Background(), a, now)
	completed := now.Add(time.Since(started))
	c.pollOne(context.Background(), b, completed.Add(5*time.Second))
	if g.calls != 1 {
		t.Fatal("usage80% did not pace sharedasset to15s")
	}
	c.pollOne(context.Background(), b, completed.Add(15*time.Second))
	if g.calls != 2 {
		t.Fatal("shared usage slot did not reopen")
	}
}

// Transport failures also run through the production Graph client, not a substituted deletion implementation.
type deletionRoundTrip func(*http.Request) (*http.Response, error)

func (f deletionRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCommentDeletionTransportFailureRetainsBuffer(t *testing.T) {
	g := &deletionGraph{}
	c, now := deletionConsole(t, g)
	graph, err := metaoauth.NewGraph("http://127.0.0.1:1", "v99.0", &http.Client{Transport: deletionRoundTrip(func(*http.Request) (*http.Response, error) { return nil, errors.New("synthetic failure") })})
	if err != nil {
		t.Fatal(err)
	}
	c.graph = graph
	s := deletionSource(c, "s", "asset", now, "1_1")
	c.pollOne(context.Background(), s, now)
	if len(s.comments) != 1 {
		t.Fatal("transport failure removed private state")
	}
}

func TestCommentDeletionDelayedFailureBackoffStartsAfterResponse(t *testing.T) {
	for _, status := range []int{429, 200} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			g := &deletionGraph{}
			c, now := deletionConsole(t, g)
			c.cfg.PollInterval = time.Millisecond
			c.cfg.MaxBackoff = 4 * time.Millisecond
			calls := 0
			body := `{"error":{"code":4}}`
			if status == 200 {
				body = `{"unknown":{"id":"unknown"}}`
			}
			graph, err := metaoauth.NewGraph("http://127.0.0.1:1", "v99.0", &http.Client{Transport: deletionRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				time.Sleep(20 * time.Millisecond)
				return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			c.graph = graph
			a := deletionSource(c, "a", "asset", now, "1_1")
			b := deletionSource(c, "b", "asset", now, "2_2")
			started := time.Now()
			c.pollOne(context.Background(), a, now)
			afterResponse := now.Add(time.Since(started))
			c.pollOne(context.Background(), b, afterResponse.Add(time.Millisecond))
			if calls != 1 {
				t.Fatalf("another source called %d times before response-completion + backoff", calls)
			}
			if len(a.comments) != 1 || len(b.comments) != 1 {
				t.Fatal("uncertain slow reply evicted data")
			}
		})
	}
}

// A facts404 is a confirmed absence, not a provider throttling error; requests still spend the common pacing slot.
func TestCommentDeletionFacts404DoesNotManufactureBackoff(t *testing.T) {
	g := &deletionGraph{}
	c, now := deletionConsole(t, g)
	c.cfg.PollInterval = time.Millisecond
	calls := 0
	graph, err := metaoauth.NewGraph("http://127.0.0.1:1", "v99.0", &http.Client{Transport: deletionRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("not found"))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	c.graph = graph
	s := deletionSource(c, "s", "asset", now)
	if f, err := c.facts(context.Background(), s, "1_1", now); err != nil || f.Found {
		t.Fatalf("first missing facts: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if f, err := c.facts(ctx, s, "1_2", now.Add(5*time.Millisecond)); err != nil || f.Found {
		t.Fatalf("normal pacing slot wrongly converted absence to transport failure: %v", err)
	}
	if calls != 2 {
		t.Fatalf("facts calls=%d", calls)
	}
}

func TestCommentDeletionContractDefaults(t *testing.T) {
	cfg := (ConsoleConfig{}).withDefaults()
	if cfg.PollInterval != 2*time.Second || cfg.MaxBackoff != 60*time.Second {
		t.Fatalf("contract pacing/backoff defaults=%s/%s", cfg.PollInterval, cfg.MaxBackoff)
	}
}

func TestCommentDeletionLateBatchCannotMutateReplacementSource(t *testing.T) {
	g := &deletionGraph{}
	c, now := deletionConsole(t, g)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	graph, err := metaoauth.NewGraph("http://127.0.0.1:1", "v99.0", &http.Client{Transport: deletionRoundTrip(func(r *http.Request) (*http.Response, error) {
		close(entered)
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-release:
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	c.graph = graph
	old := deletionSource(c, "s", "asset", now, "1_1")
	done := make(chan struct{})
	go func() { c.pollOne(context.Background(), old, now); close(done) }()
	<-entered
	c.mu.Lock()
	replacement := deletionSource(c, "s", "asset", now, "1_1")
	replacement.comments[0].Text = "synthetic replacement"
	replacement.byRef["1_1"] = replacement.comments[0]
	c.mu.Unlock()
	once.Do(func() { close(release) })
	<-done
	if got, ok := replacement.byRef["1_1"]; !ok || got.Text != "synthetic replacement" || replacement.pollEpoch != 9 {
		t.Fatal("late batch repainted or erased a new source")
	}
}
func TestCommentDeletionClearsDiscardedBackingSlots(t *testing.T) {
	g := &deletionGraph{missing: map[string]bool{"1_1": true}}
	c, now := deletionConsole(t, g)
	s := deletionSource(c, "s", "asset", now, "1_1", "1_2")
	backing := s.comments
	c.pollOne(context.Background(), s, now)
	if backing[len(s.comments)].Text != "" || backing[len(s.comments)].AuthorName != nil || backing[len(s.comments)].Ref != "" {
		t.Fatal("deleted private text/reference remained in backing storage")
	}
}
