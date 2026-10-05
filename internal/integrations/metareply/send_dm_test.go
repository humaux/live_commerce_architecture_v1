// Purpose: DB-free checks of the manual-send adapter (live-console-v1 §3.3/§4.3): Graph request shapes (RESPONSE DM without any message
// tag, comment_id private reply, public reply edge per platform, recommend comment), the outcome mapping (2xx → SUCCEEDED, 400/100 with no
// id → FAILED_FINAL, everything else UNKNOWN and never repeated), query-only Reconcile with zero HTTP, Check mapping and request parsing.
// Depends on: send_dm.go, an httptest Graph on loopback (MOCK), core.DispatchRequest.
// Used by: go test ./internal/integrations/metareply.
// Status: MOCK (LCN06 / LCN11 adapter halves; the real-PG halves live in tests/foundation/live_console_send_test.go).

package metareply

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"livecommerce/internal/integrations/core"
)

type graphLog struct {
	calls atomic.Int32
	paths []string
	last  map[string]any
}

// fakeGraph answers every POST with status/body and records the request body.
func fakeGraph(t *testing.T, status int, body string) (*httptest.Server, *graphLog) {
	t.Helper()
	log := &graphLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		log.calls.Add(1)
		log.paths = append(log.paths, r.URL.Path)
		log.last = map[string]any{}
		_ = json.Unmarshal(raw, &log.last)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, log
}

func newTestSendAdapter(t *testing.T, base string, check func(context.Context, string) (string, error)) *sendAdapter {
	t.Helper()
	if check == nil {
		check = func(context.Context, string) (string, error) { return "OK", nil }
	}
	a, err := newSendAdapter(check, nil, testKeyring(t), nil, Config{GraphBaseURL: base, GraphVersion: "v23.0"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func sendReq(provider, action string, mutate func(map[string]any)) core.DispatchRequest {
	kind := map[string]string{actionDM: "dm", actionPrivate: "private_reply", actionPublic: "public_reply", actionRecommend: "recommend"}[action]
	body := map[string]any{"v": 1, "kind": kind, "origin": "human", "platform": provider, "asset_id": "1234567890", "policy": sendPolicy,
		"deadline_at": "2030-01-01T00:00:00Z", "conversation_id": storeT, "comment_ref": "1111_2222", "live_object_id": "5555_6666"}
	if action == actionPrivate {
		body["message_type"] = manualType
	}
	if mutate != nil {
		mutate(body)
	}
	raw, _ := json.Marshal(body)
	return core.DispatchRequest{OperationID: opT, TenantID: tenantT, StoreID: storeT, PrincipalID: storeT, BindingID: bindingT, BindingVersion: 1,
		Provider: provider, ExternalAssetID: "1234567890", Purpose: "service", Action: action, Request: raw, Mode: "dispatch"}
}

func packed(psid, text string) core.Secret {
	raw, _ := json.Marshal(packedSecret{Token: "TOKEN-abc", PSID: psid, Text: text})
	return core.NewSecret(raw)
}

func TestSendRequestShapes(t *testing.T) {
	srv, log := fakeGraph(t, 200, `{"recipient_id":"9988","message_id":"m_1"}`)
	a := newTestSendAdapter(t, srv.URL, nil)
	ctx := context.Background()

	out, err := a.dispatch(ctx, sendReq("facebook", actionDM, nil), packed("9988", "你好"))
	if err != nil || out.State != "SUCCEEDED" || out.ProviderReference != "m_1" {
		t.Fatalf("dm: %+v %v", out, err)
	}
	if d, ok := out.Detail.(sendDetail); !ok || d.recipient != "9988" {
		t.Fatalf("recipient detail lost: %+v", out.Detail)
	}
	if log.paths[0] != "/v23.0/1234567890/messages" || log.last["messaging_type"] != "RESPONSE" || log.last["access_token"] != "TOKEN-abc" {
		t.Fatalf("dm request: %v %v", log.paths, log.last)
	}
	// LCN06: never a tag, never an UPDATE type.
	for _, key := range []string{"tag", "message_tag", "update"} {
		if _, found := log.last[key]; found {
			t.Fatalf("message tag key %q in body", key)
		}
	}
	if strings.Contains(strings.ToLower(mustJSON(log.last)), "human_agent") || strings.Contains(strings.ToLower(mustJSON(log.last)), "message_tag") {
		t.Fatalf("tag in body: %v", log.last)
	}
	if rcp := log.last["recipient"].(map[string]any); rcp["id"] != "9988" {
		t.Fatalf("recipient: %v", rcp)
	}

	if out, _ = a.dispatch(ctx, sendReq("facebook", actionPrivate, nil), packed("", "hello")); out.State != "SUCCEEDED" {
		t.Fatalf("private: %+v", out)
	}
	if rcp := log.last["recipient"].(map[string]any); rcp["comment_id"] != "1111_2222" || log.paths[1] != "/v23.0/1234567890/messages" {
		t.Fatalf("private request: %v %v", log.paths, log.last)
	}

	srvPub, logPub := fakeGraph(t, 200, `{"id":"1111_3333"}`)
	pub := newTestSendAdapter(t, srvPub.URL, nil)
	if out, _ = pub.dispatch(ctx, sendReq("facebook", actionPublic, nil), packed("", "thanks")); out.State != "SUCCEEDED" || out.ProviderReference != "1111_3333" {
		t.Fatalf("public: %+v", out)
	}
	if out, _ = pub.dispatch(ctx, sendReq("instagram", actionPublic, nil), packed("", "thanks")); out.State != "SUCCEEDED" {
		t.Fatalf("ig public: %+v", out)
	}
	if out, _ = pub.dispatch(ctx, sendReq("facebook", actionRecommend, nil), packed("", "recommended")); out.State != "SUCCEEDED" {
		t.Fatalf("recommend: %+v", out)
	}
	want := []string{"/v23.0/1111_2222/comments", "/v23.0/1111_2222/replies", "/v23.0/5555_6666/comments"}
	if strings.Join(logPub.paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths %v want %v", logPub.paths, want)
	}
	if logPub.last["message"] != "recommended" {
		t.Fatalf("recommend body: %v", logPub.last)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// LCN06: 400 code 100 with no message_id is FAILED_FINAL; LCN11: everything else is UNKNOWN and exactly one POST happened.
func TestSendOutcomeMapping(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		state      string
		code       string
	}{
		{"invalid request 100", `{"error":{"code":100,"message":"x"}}`, 400, "FAILED_FINAL", "invalid_request"},
		{"invalid request 2018278", `{"error":{"code":2018278}}`, 400, "FAILED_FINAL", "invalid_request"},
		{"100 but body carries id", `{"error":{"code":100},"message_id":"m_9"}`, 400, "UNKNOWN", codeUnconfirmed},
		{"other 4xx before LC-U9", `{"error":{"code":10}}`, 403, "UNKNOWN", codeUnconfirmed},
		{"400 other code", `{"error":{"code":551}}`, 400, "UNKNOWN", codeUnconfirmed},
		{"5xx", `oops`, 502, "UNKNOWN", codeUnconfirmed},
		{"429", `{"error":{"code":4}}`, 429, "UNKNOWN", codeUnconfirmed},
		{"2xx garbled", `<html>`, 200, "UNKNOWN", codeUnconfirmed},
		{"2xx without id", `{"recipient_id":"1"}`, 200, "UNKNOWN", codeUnconfirmed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, log := fakeGraph(t, tc.status, tc.body)
			a := newTestSendAdapter(t, srv.URL, nil)
			out, err := a.dispatch(context.Background(), sendReq("facebook", actionDM, nil), packed("9988", "hi"))
			if err != nil || out.State != tc.state || out.Code != tc.code {
				t.Fatalf("%s: %+v %v", tc.name, out, err)
			}
			if log.calls.Load() != 1 {
				t.Fatalf("calls=%d", log.calls.Load())
			}
		})
	}
	// Transport failure (server gone): UNKNOWN, never an error, never a retry.
	srv, _ := fakeGraph(t, 200, `{}`)
	a := newTestSendAdapter(t, srv.URL, nil)
	srv.Close()
	if out, err := a.dispatch(context.Background(), sendReq("facebook", actionDM, nil), packed("9988", "hi")); err != nil || out.State != "UNKNOWN" {
		t.Fatalf("transport: %+v %v", out, err)
	}
}

func TestSendReconcileIsQueryOnlyAndRedactedSafe(t *testing.T) {
	srv, log := fakeGraph(t, 200, `{}`)
	a := newTestSendAdapter(t, srv.URL, nil)
	for _, req := range []core.DispatchRequest{
		sendReq("facebook", actionDM, nil),
		sendReq("facebook", actionDM, func(m map[string]any) { delete(m, "conversation_id"); m["redacted"] = true }),
	} {
		req.Mode = "reconcile"
		out, err := a.reconcile(context.Background(), req)
		if err != nil || out.State != "UNKNOWN" || out.Code != codeUnproven {
			t.Fatalf("reconcile: %+v %v", out, err)
		}
	}
	if log.calls.Load() != 0 {
		t.Fatalf("reconcile made %d HTTP calls", log.calls.Load())
	}
}

func TestSendCheckMapping(t *testing.T) {
	var asked atomic.Int32
	a := newTestSendAdapter(t, "http://127.0.0.1:1", func(_ context.Context, op string) (string, error) {
		asked.Add(1)
		return map[string]string{opT: "window_closed"}[op], nil
	})
	err := a.checkRoute(context.Background(), sendReq("facebook", actionDM, nil))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("deny code not a policy denial: %v", err)
	}
	// OK passes; an infrastructure error is a plain error (UNKNOWN policy_check_failed), not a denial.
	ok := newTestSendAdapter(t, "http://127.0.0.1:1", nil)
	if err := ok.checkRoute(context.Background(), sendReq("facebook", actionDM, nil)); err != nil {
		t.Fatalf("OK denied: %v", err)
	}
	broken := newTestSendAdapter(t, "http://127.0.0.1:1", func(context.Context, string) (string, error) { return "", errors.New("db down") })
	if err := broken.checkRoute(context.Background(), sendReq("facebook", actionDM, nil)); err == nil || errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("infrastructure error became a denial: %v", err)
	}
	// A malformed/redacted request is denied before any database call.
	asked.Store(0)
	if err := a.checkRoute(context.Background(), sendReq("facebook", actionDM, func(m map[string]any) { m["policy"] = "x" })); !errors.Is(err, core.ErrPolicyDenied) || asked.Load() != 0 {
		t.Fatalf("bad policy: %v asked=%d", err, asked.Load())
	}
}

func TestParseSendRejectsMismatches(t *testing.T) {
	good := sendReq("facebook", actionDM, nil)
	if _, err := parseSend(good); err != nil {
		t.Fatal(err)
	}
	for name, req := range map[string]core.DispatchRequest{
		"asset mismatch":     func() core.DispatchRequest { r := good; r.ExternalAssetID = "999"; return r }(),
		"kind mismatch":      sendReq("facebook", actionDM, func(m map[string]any) { m["kind"] = "public_reply" }),
		"platform mismatch":  sendReq("facebook", actionDM, func(m map[string]any) { m["platform"] = "instagram" }),
		"private not manual": sendReq("facebook", actionPrivate, func(m map[string]any) { m["message_type"] = "first_private_reply" }),
		"bad comment ref":    sendReq("facebook", actionPublic, func(m map[string]any) { m["comment_ref"] = "a/b" }),
		"bad live object":    sendReq("facebook", actionRecommend, func(m map[string]any) { m["live_object_id"] = "x y" }),
		"redacted":           sendReq("facebook", actionDM, func(m map[string]any) { m["redacted"] = true }),
		"v2":                 sendReq("facebook", actionDM, func(m map[string]any) { m["v"] = 2 }),
	} {
		if _, err := parseSend(req); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestSendRoutesCoverTheActions(t *testing.T) {
	a := newTestSendAdapter(t, "http://127.0.0.1:1", nil)
	seen := map[string]bool{}
	for _, r := range a.routes() {
		if r.Purpose != "service" || r.Check == nil || r.LoadSecret == nil || r.DispatchWithSecret == nil || r.Reconcile == nil || r.Finish == nil {
			t.Fatalf("incomplete route %+v", r)
		}
		seen[r.Provider+"/"+r.Action] = true
	}
	for _, want := range []string{"facebook/meta.dm_send", "instagram/meta.dm_send", "facebook/meta.public_reply", "instagram/meta.public_reply", "facebook/meta.offer_recommend"} {
		if !seen[want] {
			t.Fatalf("missing route %s (have %v)", want, seen)
		}
	}
	if seen["instagram/meta.offer_recommend"] {
		t.Fatal("IG recommend must not be routed (FB only)")
	}
}

func TestIsManualReply(t *testing.T) {
	if !isManualReply([]byte(`{"message_type":"manual_private_reply"}`)) || isManualReply([]byte(`{"message_type":"first_private_reply"}`)) || isManualReply([]byte(`nope`)) {
		t.Fatal("isManualReply")
	}
}
