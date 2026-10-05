package metareply

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/integrations/core"
)

// A5-3 (MOCK) worker-side tests: the read-only Graph GET is dispatched with pages_read_engagement
// custody and normalized into a bounded, LIVE-first snapshot; no Meta mutation, no token in the URL.

func liveVideoRequest() core.DispatchRequest {
	raw, _ := json.Marshal(map[string]any{"v": 1, "binding_id": bindingT, "asset_id": "1234567890"})
	return core.DispatchRequest{OperationID: opT, Provider: "facebook", Action: "meta.live_videos", Purpose: "service",
		ExternalAssetID: "1234567890", BindingID: bindingT, Request: raw}
}

func liveVideoHarness(t *testing.T, handler func(*http.Request, http.ResponseWriter)) (core.DispatchRoute, func() []string) {
	t.Helper()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" auth="+r.Header.Get("Authorization"))
		handler(r, w)
	}))
	t.Cleanup(srv.Close)
	route, err := newLiveVideoRoute(func(context.Context, string) (string, error) { return "", nil }, testKeyring(t), nil,
		Config{GraphBaseURL: srv.URL, GraphVersion: "v26.0"})
	if err != nil {
		t.Fatal(err)
	}
	return route, func() []string { return append([]string(nil), calls...) }
}

func TestLiveVideosDispatchReadOnly(t *testing.T) {
	route, calls := liveVideoHarness(t, func(r *http.Request, w http.ResponseWriter) {
		io.WriteString(w, `{"data":[
			{"id":"777","post_id":"1234567890_88","title":"recap","status":"VOD","creation_time":"2026-10-04T12:00:00+0000"},
			{"id":"888","post_id":"1234567890_99","title":"live now","status":"LIVE","creation_time":"2026-10-05T12:00:00+0000"}
		]}`)
	})
	out, err := route.DispatchWithSecret(context.Background(), liveVideoRequest(), core.NewSecret([]byte(fakeTok)))
	if err != nil || out.State != "SUCCEEDED" || out.Code != "graph_read" {
		t.Fatalf("dispatch=%+v,%v", out, err)
	}
	items, ok := out.Detail.([]liveVideoItem)
	if !ok || len(items) != 2 {
		t.Fatalf("items=%+v", out.Detail)
	}
	// LIVE first even though it is listed second; post_id stays <page>_<post>.
	if items[0].Status != "LIVE" || items[0].VideoID != "888" || items[0].PostID != "1234567890_99" ||
		items[1].Status != "VOD" || items[1].PostID != "1234567890_88" {
		t.Fatalf("normalized=%+v", items)
	}
	log := calls()
	if len(log) != 1 {
		t.Fatalf("calls=%d", len(log))
	}
	call := log[0]
	if !strings.HasPrefix(call, "GET /v26.0/1234567890/live_videos?") ||
		!strings.HasSuffix(call, " auth=Bearer "+fakeTok) || strings.Contains(call, "access_token") {
		t.Fatalf("unsafe call: %s", call)
	}
	if !strings.Contains(call, "fields=id%2Cpost_id%2Ctitle%2Cstatus%2Ccreation_time") || !strings.Contains(call, "limit=25") {
		t.Fatalf("call query: %s", call)
	}
}

func TestNormalizeLiveVideos(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rows  []json.RawMessage
		ok    bool
		first string // video_id of the first item, "" when !ok
	}{
		{"empty", nil, true, ""},
		{"bare post prefixed", []json.RawMessage{json.RawMessage(`{"id":"1","post_id":"99","title":"t","status":"VOD","creation_time":"2026-01-01T00:00:00+0000"}`)}, true, "1"},
		{"mismatched prefix", []json.RawMessage{json.RawMessage(`{"id":"1","post_id":"9999999999_99","title":"t","status":"VOD","creation_time":"2026-01-01T00:00:00+0000"}`)}, false, ""},
		{"non numeric id", []json.RawMessage{json.RawMessage(`{"id":"x","post_id":"99","title":"t","status":"VOD","creation_time":"2026-01-01T00:00:00+0000"}`)}, false, ""},
		{"live first", []json.RawMessage{
			json.RawMessage(`{"id":"1","post_id":"1","title":"a","status":"VOD","creation_time":"2026-01-01T00:00:00+0000"}`),
			json.RawMessage(`{"id":"2","post_id":"2","title":"b","status":"LIVE","creation_time":"2025-01-01T00:00:00+0000"}`),
		}, true, "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, ok := normalizeLiveVideos(tc.rows, "1234567890")
			if ok != tc.ok {
				t.Fatalf("ok=%t want %t", ok, tc.ok)
			}
			if !ok {
				return
			}
			if len(items) != len(tc.rows) || (len(items) > 0 && items[0].VideoID != tc.first) {
				t.Fatalf("items=%+v first=%s", items, tc.first)
			}
			for _, it := range items {
				if !strings.HasPrefix(it.PostID, "1234567890_") {
					t.Fatalf("post_id not prefixed: %+v", it)
				}
			}
		})
	}
}

func TestSanitizeLiveToken(t *testing.T) {
	if got := sanitizeLiveToken("a\nb\x00c", 10); got != "abc" {
		t.Fatalf("control chars: %q", got)
	}
	if got := sanitizeLiveToken(string([]byte{0xff, 0xfe}), 10); got != "" {
		t.Fatalf("invalid utf8: %q", got)
	}
	if got := sanitizeLiveToken("ééé", 2); len([]rune(got)) != 2 {
		t.Fatalf("rune bound: %q", got)
	}
}

func TestLiveVideosBadResultAndPolicy(t *testing.T) {
	route, _ := liveVideoHarness(t, func(r *http.Request, w http.ResponseWriter) {
		io.WriteString(w, `{"data":[{"id":"1","post_id":"9999999999_99","title":"t","status":"VOD","creation_time":"2026-01-01T00:00:00+0000"}]}`)
	})
	out, _ := route.DispatchWithSecret(context.Background(), liveVideoRequest(), core.NewSecret([]byte(fakeTok)))
	if out.State != "FAILED_FINAL" || out.Code != "bad_result" {
		t.Fatalf("bad result=%+v", out)
	}
}
