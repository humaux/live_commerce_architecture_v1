// Purpose: DB-free tests of the sold_out_reply branch of the meta.private_reply route (frozen text sent, no link/token logic, malformed
// requests refused before any call, Check maps through claims.check_meta_reply).
// Depends on: routes_test.go harness (graphServer, newHarness, fakeTok).
// Used by: go test ./internal/integrations/metareply.
// Status: MOCK.

package metareply

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/integrations/core"
)

func (h *harness) soldOutRequest(provider string, mutate func(map[string]any)) core.DispatchRequest {
	body := map[string]any{"v": 1, "platform": provider, "source_id": storeT, "asset_id": "1234567890", "comment_ref": "1111_2222",
		"bundle_id": bundleT, "session_id": storeT, "offer_id": storeT, "locale": "zh-TW", "template": "sold-out-reply/v1", "template_version": 1,
		"policy": "mpr-policy/v1", "message_type": "sold_out_reply", "origin_kind": "auto", "text": "抱歉，測試商品 已售完", "takeover_generation": 0,
		"deadline_at": "2030-01-01T00:00:00Z", "live_media": false}
	if mutate != nil {
		mutate(body)
	}
	raw, _ := json.Marshal(body)
	req := h.request(provider, nil)
	req.Request = raw
	return req
}

func TestSoldOutDispatchSendsFrozenTextOnce(t *testing.T) {
	var calls []graphCall
	srv := graphServer(t, 200, `{"recipient_id":"555","message_id":"m_so1"}`, &calls)
	h := newHarness(t)
	route := h.routes(Config{GraphBaseURL: srv.URL, GraphVersion: "v23.0"})["facebook"]
	out, err := route.DispatchWithSecret(context.Background(), h.soldOutRequest("facebook", nil), core.NewSecret([]byte(fakeTok)))
	if err != nil || out.State != "SUCCEEDED" || out.ProviderReference != "m_so1" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
	msg := calls[0].body["message"].(map[string]any)
	if msg["text"] != "抱歉，測試商品 已售完" || strings.Contains(calls[0].rawBody, "claim") || strings.Contains(calls[0].rawBody, "https://") {
		t.Fatalf("body = %s", calls[0].rawBody)
	}
}

func TestSoldOutDispatchNeverRepostsOnDoubt(t *testing.T) {
	var calls []graphCall
	srv := graphServer(t, 500, `boom`, &calls)
	h := newHarness(t)
	route := h.routes(Config{GraphBaseURL: srv.URL, GraphVersion: "v23.0"})["instagram"]
	out, err := route.DispatchWithSecret(context.Background(), h.soldOutRequest("instagram", nil), core.NewSecret([]byte(fakeTok)))
	if err != nil || out != (core.Outcome{State: "UNKNOWN", Code: "graph_unconfirmed"}) || len(calls) != 1 {
		t.Fatalf("SO08 out=%+v err=%v calls=%d", out, err, len(calls))
	}
}

func TestSoldOutRefusesMalformedBeforeAnyCall(t *testing.T) {
	var calls []graphCall
	srv := graphServer(t, 200, `{"message_id":"m1"}`, &calls)
	h := newHarness(t)
	h.checkFn = func(context.Context, string, []byte) (string, error) { return "OK", nil }
	route := h.routes(Config{GraphBaseURL: srv.URL, GraphVersion: "v23.0"})["facebook"]
	for name, mutate := range map[string]func(map[string]any){
		"empty text":     func(m map[string]any) { m["text"] = "" },
		"newline":        func(m map[string]any) { m["text"] = "a\nb" },
		"huge text":      func(m map[string]any) { m["text"] = strings.Repeat("好", 401) },
		"foreign asset":  func(m map[string]any) { m["asset_id"] = "999" },
		"bad bundle":     func(m map[string]any) { m["bundle_id"] = "x" },
		"bad comment":    func(m map[string]any) { m["comment_ref"] = "" },
		"bad version":    func(m map[string]any) { m["v"] = 2 },
		"missing text":   func(m map[string]any) { delete(m, "text") },
		"text not a str": func(m map[string]any) { m["text"] = 5 },
	} {
		req := h.soldOutRequest("facebook", mutate)
		if _, err := route.DispatchWithSecret(context.Background(), req, core.NewSecret([]byte(fakeTok))); !errors.Is(err, errBadRequest) {
			t.Errorf("%s: dispatch err = %v", name, err)
		}
		if err := route.Check(context.Background(), req); err == nil {
			t.Errorf("%s: Check accepted", name)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("%d calls for malformed requests", len(calls))
	}
}

func TestSoldOutCheckUsesSharedCheck(t *testing.T) {
	h := newHarness(t)
	for code, wantDeny := range map[string]bool{"OK": false, "human_takeover": true, "source_off": true, "deadline": true, "link_invalid": true} {
		h.checkFn = func(context.Context, string, []byte) (string, error) { return code, nil }
		err := h.routes(Config{GraphBaseURL: "https://graph.facebook.com", GraphVersion: "v23.0"})["facebook"].Check(context.Background(), h.soldOutRequest("facebook", nil))
		if (err != nil) != wantDeny {
			t.Errorf("code %s: err = %v", code, err)
		}
	}
}
