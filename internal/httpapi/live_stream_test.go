// live_stream_test.go covers the A2/A3 console-comment routes DB-free: route registration (so a ServeMux
// ambiguity panics here instead of first use), the query parser and the error classifier. Comment text
// never appears here; the transport-level guards (401/405/400/415) run before any database.
package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/live"
)

func testCommentStream(t *testing.T) *live.CommentStream {
	t.Helper()
	bridge, err := metareply.NewBridgeClient("http://127.0.0.1:1", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cs, err := live.NewCommentStream(bridge, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

func TestLiveStreamRouteRegistration(t *testing.T) {
	mux := http.NewServeMux()
	registerStudioRoutes(mux, nil, true, nil, nil)
	registerLiveFlowRoutes(mux, nil, true, nil)
	registerLiveStreamRoutes(mux, nil, testCommentStream(t))
}

func TestLiveStreamQuery(t *testing.T) {
	for _, raw := range []string{
		"after_epoch=-1", "after_epoch=+1", "after_epoch=1.0", "after_epoch=x", "after_epoch=",
		"after_seq=-1", "after_seq=x", "after_seq=1&after_seq=2",
		"before_cursor=", "before_cursor=a%zz", "before_cursor=" + strings.Repeat("a", 1025),
		"limit=0", "limit=101", "limit=x", "limit=1&limit=2", "unknown=x", "limit=%zz",
	} {
		if _, err := liveStreamQuery(mustURL(raw)); err == nil {
			t.Fatalf("accepted bad live-stream query %q", raw)
		}
	}
	q, err := liveStreamQuery(mustURL("after_epoch=5&after_seq=6&limit=20"))
	if err != nil || q.AfterEpoch == nil || *q.AfterEpoch != 5 || q.AfterSeq == nil || *q.AfterSeq != 6 || q.Limit != 20 {
		t.Fatalf("valid query %+v %v", q, err)
	}
	q, err = liveStreamQuery(mustURL(""))
	if err != nil || q.Limit != 50 || q.AfterEpoch != nil || q.AfterSeq != nil || q.BeforeCursor != nil {
		t.Fatalf("empty query defaults wrong: %+v %v", q, err)
	}
	cur := "abc.def"
	q, err = liveStreamQuery(mustURL("before_cursor=" + cur))
	if err != nil || q.BeforeCursor == nil || *q.BeforeCursor != cur {
		t.Fatalf("before_cursor %+v %v", q, err)
	}
}

func TestLiveStreamClassify(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{live.ErrNoSource, 409, "no_source"},
		{live.ErrStreamUnavailable, 503, "stream_unavailable"},
		{live.ErrInvalidCursor, 400, "invalid_cursor"},
		{live.ErrInvalidRef, 422, "invalid_ref"},
	} {
		status, code := liveStreamClassify(tt.err)
		if status != tt.status || code != tt.code {
			t.Fatalf("classify(%v) = %d %s", tt.err, status, code)
		}
	}
}

func TestLiveStreamTransportGuards(t *testing.T) {
	h := NewHandler(nil, Options{Studio: true, CommentStream: testCommentStream(t)})
	const base = "/v1/admin/stores/11111111-1111-4111-8111-111111111111/live-sessions/22222222-2222-4222-8222-222222222222"
	for _, tt := range []struct {
		name, method, path, auth string
		want                     int
	}{
		{"a2 no auth", "GET", base + "/comments", "", 401},
		{"a2 bad cursor", "GET", base + "/comments?after_seq=-1", "Bearer " + strings.Repeat("a", 64), 400},
		{"a2 post method", "POST", base + "/comments", "", 405},
		{"a3 no auth", "POST", base + "/comments/123/print", "", 401},
		{"a3 get method", "GET", base + "/comments/123/print", "", 405},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.method == http.MethodPost {
				r.Header.Set("Content-Type", "application/json")
				r.Body = io.NopCloser(strings.NewReader("{}"))
				r.Header.Set("Idempotency-Key", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
			}
			if tt.auth != "" {
				r.Header.Set("Authorization", tt.auth)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

func mustURL(raw string) *url.URL {
	if raw == "" {
		return &url.URL{}
	}
	u, err := url.Parse("?" + raw)
	if err != nil {
		panic(err)
	}
	return u
}
