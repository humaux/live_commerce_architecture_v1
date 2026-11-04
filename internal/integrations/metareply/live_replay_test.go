// Purpose: FEAT-LIVE-REPLAY MOCK gates for the existing official live_videos read.
// Depends on: liveVideoHarness, normalizeLiveVideos and the v26.0 LiveVideo contract.
// Used by: GitHub Foundation Go unit suite.
// Invariants: request only official fields; retain only safe Facebook permalinks; never invent a URL from an ID.
package metareply

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"livecommerce/internal/integrations/core"
)

func TestLiveReplayOfficialPermalinkRead(t *testing.T) {
	const permalink = "https://www.facebook.com/synthetic.page/videos/777/"
	route, calls := liveVideoHarness(t, func(r *http.Request, w http.ResponseWriter) {
		io.WriteString(w, `{"data":[{"id":"777","post_id":"1234567890_88","title":"測試回放","status":"VOD","creation_time":"2026-10-30T12:00:00+0000","permalink_url":"`+permalink+`"}]}`)
	})
	out, err := route.DispatchWithSecret(context.Background(), liveVideoRequest(), core.NewSecret([]byte(fakeTok)))
	if err != nil || out.State != "SUCCEEDED" {
		t.Fatalf("official read: state=%s err=%v", out.State, err)
	}
	gotCalls := calls()
	if len(gotCalls) != 1 {
		t.Fatalf("wanted one existing read, got %d calls", len(gotCalls))
	}
	u, err := url.Parse(strings.Split(gotCalls[0], " auth=")[0][4:])
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Split(u.Query().Get("fields"), ",")
	found := false
	for _, field := range fields {
		found = found || field == "permalink_url"
	}
	if !found {
		t.Fatal("FEAT-LIVE-REPLAY: existing official live_videos read must request permalink_url")
	}
	raw, err := json.Marshal(out.Detail)
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if json.Unmarshal(raw, &items) != nil || len(items) != 1 || items[0]["permalink_url"] != permalink {
		t.Fatalf("official permalink was discarded: %s", raw)
	}
}

func TestLiveReplayPermalinkTrustBoundary(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"official", "https://www.facebook.com/synthetic.page/videos/777/", "https://www.facebook.com/synthetic.page/videos/777/"},
		{"absent", "", ""},
		{"not https", "http://www.facebook.com/synthetic.page/videos/777/", ""},
		{"foreign host", "https://example.test/videos/777", ""},
		{"suffix attack", "https://www.facebook.com.example.test/videos/777", ""},
		{"credentials", "https://synthetic@www.facebook.com/videos/777", ""},
		{"token query", "https://www.facebook.com/synthetic.page/videos/777/?access_token=synthetic", ""},
		{"script", "javascript:alert(1)", ""},
		{"embedded player input", "https://www.facebook.com/plugins/video.php?href=synthetic", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, _ := json.Marshal(map[string]string{"id": "777", "post_id": "1234567890_88", "status": "VOD", "permalink_url": tc.input})
			items, ok := normalizeLiveVideos([]json.RawMessage{row}, "1234567890")
			if !ok || len(items) != 1 {
				t.Fatal("optional unusable permalink must not break the existing picker")
			}
			raw, _ := json.Marshal(items[0])
			var item map[string]any
			if json.Unmarshal(raw, &item) != nil {
				t.Fatal("invalid normalized item")
			}
			got, _ := item["permalink_url"].(string)
			if got != tc.want {
				t.Fatalf("safe permalink=%q want=%q", got, tc.want)
			}
		})
	}
}
