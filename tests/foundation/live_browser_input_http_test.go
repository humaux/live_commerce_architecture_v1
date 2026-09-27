package foundation_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/httpapi"
	"livecommerce/internal/live"
)

const brwHTTPURL = "ws://127.0.0.1:7880" // Go transport only; not an HTTPS browser gate.

func brwHTTPCall(t *testing.T, handler http.Handler, method, path, bearer, key, body string, want int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != want {
		// Never print a response body here: a broken negative gate may contain JWT.
		t.Fatalf("%s %s status=%d want=%d", method, path, w.Code, want)
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("%s %s cache policy=%q", method, path, w.Header().Get("Cache-Control"))
	}
	return w
}

func brwHTTPFields(t *testing.T, w *httptest.ResponseRecorder, fields ...string) map[string]json.RawMessage {
	t.Helper()
	if w.Body.Len() > 8192 {
		t.Fatalf("response exceeds token ceiling: %d", w.Body.Len())
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out) != len(fields) {
		t.Fatalf("response shape: count=%d err=%v", len(out), err)
	}
	for _, field := range fields {
		if _, ok := out[field]; !ok {
			t.Fatalf("response missing %s", field)
		}
	}
	return out
}

func brwHTTPNoToken(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code < 400 || bytes.Contains(bytes.ToLower(w.Body.Bytes()), []byte("\"token\"")) ||
		bytes.Contains(w.Body.Bytes(), []byte("eyJ")) || w.Header().Get("Location") != "" {
		t.Fatalf("failed token action exposed credential or redirect: status=%d", w.Code)
	}
}

func brwHTTPJWT(t *testing.T, token, identity, room string, issued, expires int64) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatal("publisher token is not a three-part JWT")
	}
	decode := func(part string) map[string]any {
		t.Helper()
		raw, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil {
			t.Fatal("publisher JWT segment invalid")
		}
		var out map[string]any
		if json.Unmarshal(raw, &out) != nil {
			t.Fatal("publisher JWT JSON invalid")
		}
		return out
	}
	if !reflect.DeepEqual(decode(parts[0]), map[string]any{"alg": "HS256", "typ": "JWT"}) {
		t.Fatal("publisher JWT header changed")
	}
	mac := hmac.New(sha256.New, []byte(strings.Repeat("s", 40))) // lmeConfig fixture secret only
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		t.Fatal("publisher JWT signature invalid")
	}
	want := map[string]any{
		"iss": "lme_test_key", "sub": identity, "iat": float64(issued), "nbf": float64(issued), "exp": float64(expires),
		"video": map[string]any{"room": room, "roomJoin": true, "canPublish": true,
			"canPublishSources": []any{"camera", "microphone"}, "canSubscribe": false,
			"canPublishData": false, "canUpdateOwnMetadata": false},
	}
	if !reflect.DeepEqual(decode(parts[1]), want) {
		t.Fatal("publisher JWT claims are not the exact fixed grant")
	}
}

func brwHTTPPaths(h *brwHarness) (string, string) {
	base := "/v1/admin/stores/" + h.lp.f.storeA1 + "/live-sessions/" + h.session + "/input/"
	return base + "start", base + "token"
}

func brwHTTPStart(t *testing.T, h *brwHarness, handler http.Handler) string {
	t.Helper()
	start, _ := brwHTTPPaths(h)
	body := `{"authorization_id":"` + h.input.AuthorizationID + `","expected_session_version":1}`
	w := brwHTTPCall(t, handler, "POST", start, h.logins.a, t04Key("brw-http-start"), body, 200)
	out := brwHTTPFields(t, w, "session_id", "attempt_id", "state")
	var attempt, session string
	if json.Unmarshal(out["attempt_id"], &attempt) != nil || json.Unmarshal(out["session_id"], &session) != nil ||
		session != h.session || attempt == "" {
		t.Fatal("marked HTTP start lost safe original receipt")
	}
	return attempt
}

func brwHTTPTokenBody(attempt string) string {
	return `{"attempt_id":"` + attempt + `","expected_session_version":1}`
}

func TestLiveBrowserInputBRW05GoHTTPTokenCommitAndReplay(t *testing.T) {
	h := brwRegistered(t)
	handler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
	_, tokenPath := brwHTTPPaths(h)
	attempt := brwHTTPStart(t, h, handler)
	key := t04Key("brw-http-token")
	body := brwHTTPTokenBody(attempt)
	w := brwHTTPCall(t, handler, "POST", tokenPath, h.logins.a, key, body, 200)
	brwHTTPFields(t, w, "attempt_id", "room_name", "publisher_identity", "url", "token", "expires_at")
	var response struct {
		AttemptID         string `json:"attempt_id"`
		RoomName          string `json:"room_name"`
		PublisherIdentity string `json:"publisher_identity"`
		URL               string `json:"url"`
		Token             string `json:"token"`
		ExpiresAt         int64  `json:"expires_at"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.AttemptID != attempt || response.URL != brwHTTPURL ||
		response.Token == "" || w.Header().Get("Cache-Control") != "private, no-store" ||
		w.Header().Get("Content-Type") != "application/json" ||
		w.Header().Get("Location") != "" || w.Header().Get("Set-Cookie") != "" {
		t.Fatal("successful token response changed fixed DTO or transport")
	}
	var issued, expires int64
	var room, publisher, project, endpoint string
	var version int64
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT grant_iat,grant_exp,room_name,publisher_identity,
		project_id,endpoint_identity,credential_version FROM live.media_input_custody WHERE attempt_id=$1`, attempt).
		Scan(&issued, &expires, &room, &publisher, &project, &endpoint, &version); err != nil {
		t.Fatal(err)
	}
	if response.RoomName != room || response.PublisherIdentity != publisher || response.ExpiresAt != expires ||
		project != "project_lma" || endpoint != "https://unit.livekit.cloud" || version != 1 ||
		issued <= 0 || expires <= issued || expires-issued > 60 || expires <= time.Now().Unix() {
		t.Fatal("HTTP token not bound to persisted nonsecret grant and <=60s expiry")
	}
	brwHTTPJWT(t, response.Token, publisher, room, issued, expires)
	// The same login may replay, but cannot extend or create a second grant.
	for _, replayKey := range []string{key, t04Key("brw-http-other-key")} {
		replay := brwHTTPCall(t, handler, "POST", tokenPath, h.logins.a, replayKey, body, 200)
		if !bytes.Equal(replay.Body.Bytes(), w.Body.Bytes()) {
			t.Fatal("fixed browser grant changed on token replay")
		}
	}
	if got := bicOwnedFacts(t, h.bicHarness); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 2} {
		t.Fatalf("token replay duplicated original artifacts: %v", got)
	}
	var durable, custody, audit string
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT
		coalesce((SELECT string_agg(response::text,'') FROM ops.command_results
		 WHERE principal_id=$1 AND operation IN ('live.media.input.start','live.media.input.reserve')),''),
		to_jsonb(c)::text,
		coalesce((SELECT string_agg(to_jsonb(e)::text,'') FROM ops.audit_events e
		 WHERE e.principal_id=$1 AND e.action LIKE 'live.media.input.%'),'')
		FROM live.media_input_custody c WHERE c.attempt_id=$2`, h.lp.actor, attempt).
		Scan(&durable, &custody, &audit); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(durable, response.Token) || strings.Contains(custody, response.Token) ||
		strings.Contains(audit, response.Token) || strings.Contains(durable, `"token"`) ||
		strings.Contains(custody, `"token"`) || strings.Contains(audit, `"token"`) {
		t.Fatal("durable command receipt retained publisher JWT")
	}
	wrongLogin := brwHTTPCall(t, handler, "POST", tokenPath, h.logins.b, key, body, 403)
	brwHTTPNoToken(t, wrongLogin)
	if _, err := h.registrar.Exec(context.Background(),
		`SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,'operator_revoke')`,
		h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID); err != nil {
		t.Fatal(err)
	}
	denied := brwHTTPCall(t, handler, "POST", tokenPath, h.logins.a, key, body, 409)
	brwHTTPNoToken(t, denied)
}

func TestLiveBrowserInputBRW05GoHTTPStrictAndAuthority(t *testing.T) {
	t.Run("nil-options-and-kernel-marker", func(t *testing.T) {
		h := brwRegistered(t)
		start, tokenPath := brwHTTPPaths(h)
		nilHandler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner})
		for _, path := range []string{start, tokenPath} {
			brwHTTPNoToken(t, brwHTTPCall(t, nilHandler, "POST", path, h.logins.a,
				t04Key("brw-http-disabled"), `{}`, 404))
		}
		kernel, plan, _ := bicStarted(t)
		runtime, _ := brwPlanNoIOMap(t)
		kernelHandler := httpapi.NewHandler(kernel.lp.f.runtime, httpapi.Options{Live: kernel.planner, BrowserInput: runtime})
		kernelBase := "/v1/admin/stores/" + kernel.lp.f.storeA1 + "/live-sessions/" + kernel.session + "/input/"
		brwHTTPNoToken(t, brwHTTPCall(t, kernelHandler, "POST", kernelBase+"token", kernel.logins.a,
			t04Key("brw-http-kernel-token"), brwHTTPTokenBody(plan.AttemptID), 409))
		brwHTTPNoToken(t, brwHTTPCall(t, kernelHandler, "POST", kernelBase+"start", kernel.logins.a,
			t04Key("brw-http-kernel-start"), `{"authorization_id":"`+kernel.input.AuthorizationID+`","expected_session_version":1}`, 409))
	})

	t.Run("strict-route-and-access", func(t *testing.T) {
		h := brwRegistered(t)
		handler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
		start, tokenPath := brwHTTPPaths(h)
		startBody := `{"authorization_id":"` + h.input.AuthorizationID + `","expected_session_version":1}`
		for _, item := range []struct {
			method, path, bearer, key, body string
			status                          int
		}{
			{"POST", start + "?", h.logins.a, t04Key("brw-bad-query"), startBody, 422},
			{"POST", tokenPath + "?x=1", h.logins.a, t04Key("brw-bad-query"), `{}`, 422},
			{"POST", strings.Replace(start, h.session, "invalid-session-id", 1), h.logins.a, t04Key("brw-bad-session"), startBody, 422},
			{"GET", tokenPath, h.logins.a, "", "", 405},
			{"POST", start, h.logins.a, "", startBody, 422},
			{"POST", start, h.logins.a, t04Key("brw-bad-json"), `{"authorization_id":"` + h.input.AuthorizationID + `","authorization_id":"` + h.input.AuthorizationID + `","expected_session_version":1}`, 400},
			{"POST", start, h.logins.a, t04Key("brw-bad-json"), `{"authorization_id":"` + h.input.AuthorizationID + `","expected_session_version":1,"extra":1}`, 400},
			{"POST", start, h.logins.a, t04Key("brw-bad-json"), `{"Authorization_ID":"` + h.input.AuthorizationID + `","expected_session_version":1}`, 400},
			{"POST", start, h.logins.a, t04Key("brw-bad-json"), startBody + `{}`, 400},
			{"POST", start, "", t04Key("brw-no-auth"), startBody, 401},
			{"POST", start, h.lp.limitedToken, t04Key("brw-no-manage"), startBody, 403},
			{"POST", strings.Replace(start, h.lp.f.storeA1, h.lp.f.storeA2, 1), h.logins.a, t04Key("brw-other-store"), startBody, 404},
		} {
			brwHTTPNoToken(t, brwHTTPCall(t, handler, item.method, item.path, item.bearer, item.key, item.body, item.status))
		}
		if got := bicOwnedFacts(t, h.bicHarness); got != [8]int64{1} {
			t.Fatalf("strict HTTP denial wrote media artifacts: %v", got)
		}
		attempt := brwHTTPStart(t, h, handler)
		tokenBody := brwHTTPTokenBody(attempt)
		for _, body := range []string{
			`{"attempt_id":"` + attempt + `","attempt_id":"` + attempt + `","expected_session_version":1}`,
			`{"attempt_id":"` + attempt + `","expected_session_version":1,"unknown":1}`,
		} {
			brwHTTPNoToken(t, brwHTTPCall(t, handler, "POST", tokenPath, h.logins.a,
				t04Key("brw-token-bad-json"), body, 400))
		}
		brwHTTPNoToken(t, brwHTTPCall(t, handler, "POST", tokenPath, h.logins.a,
			"", tokenBody, 422))
		brwHTTPNoToken(t, brwHTTPCall(t, handler, "POST", tokenPath, h.logins.a,
			t04Key("brw-bad-attempt"), brwHTTPTokenBody("invalid-attempt-id"), 422))
		brwHTTPNoToken(t, brwHTTPCall(t, handler, "POST", tokenPath, h.logins.b,
			t04Key("brw-other-login"), tokenBody, 403))
		var grant *int64
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT grant_iat FROM live.media_input_custody WHERE attempt_id=$1`, attempt).
			Scan(&grant); err != nil || grant != nil {
			t.Fatalf("denied HTTP token request persisted grant: %v %v", grant, err)
		}
	})

	t.Run("runtime-binding-mismatch", func(t *testing.T) {
		h := brwRegistered(t)
		config := lmeConfig()
		config.Endpoint = "https://other.livekit.cloud"
		mismatch, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{{
			ProjectID: "project_lma", CredentialVersion: 1, Config: config,
			Transport: lmeTransport("127.0.0.1:1"), BrowserURL: brwHTTPURL,
		}})
		if err != nil {
			t.Fatal(err)
		}
		bad := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: mismatch})
		start, tokenPath := brwHTTPPaths(h)
		startBody := `{"authorization_id":"` + h.input.AuthorizationID + `","expected_session_version":1}`
		brwHTTPNoToken(t, brwHTTPCall(t, bad, "POST", start, h.logins.a,
			t04Key("brw-mismatch-start"), startBody, 409))
		if got := bicOwnedFacts(t, h.bicHarness); got != [8]int64{1} {
			t.Fatalf("mismatched runtime planned attempt: %v", got)
		}
		plan, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
			h.lp.f.storeA1, t04Key("brw-mismatch-fixture-plan"), h.input)
		if err != nil {
			t.Fatal(err)
		}
		brwHTTPNoToken(t, brwHTTPCall(t, bad, "POST", tokenPath, h.logins.a,
			t04Key("brw-mismatch-token"), brwHTTPTokenBody(plan.AttemptID), 409))
		var issued *int64
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT grant_iat FROM live.media_input_custody WHERE attempt_id=$1`, plan.AttemptID).
			Scan(&issued); err != nil || issued != nil {
			t.Fatalf("mismatch issued grant: %v %v", issued, err)
		}
	})

	t.Run("revoked-initiating-login", func(t *testing.T) {
		h := brwRegistered(t)
		plan, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
			h.lp.f.storeA1, t04Key("brw-http-login-plan"), h.input)
		if err != nil {
			t.Fatal(err)
		}
		if err := h.logins.service.Logout(context.Background(), h.logins.a); err != nil {
			t.Fatal(err)
		}
		handler := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
		_, tokenPath := brwHTTPPaths(h)
		brwHTTPNoToken(t, brwHTTPCall(t, handler, "POST", tokenPath, h.logins.a,
			t04Key("brw-http-logged-out"), brwHTTPTokenBody(plan.AttemptID), 401))
		var issued *int64
		if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT grant_iat FROM live.media_input_custody WHERE attempt_id=$1`, plan.AttemptID).
			Scan(&issued); err != nil || issued != nil {
			t.Fatalf("revoked login persisted publisher grant: %v %v", issued, err)
		}
	})
}

func TestLiveBrowserInputBRW05GoHTTPCommittedTokenAckLoss(t *testing.T) {
	h := brwRegistered(t)
	plan, err := brwPlan(context.Background(), h, h.lp.f.runtime, h.logins.a,
		h.lp.f.storeA1, t04Key("brw-http-ack-plan"), h.input)
	if err != nil {
		t.Fatal(err)
	}
	fault, loss := bicCommitAckPool(t, h.lp.f.runtime)
	handler := httpapi.NewHandler(fault, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
	_, tokenPath := brwHTTPPaths(h)
	key, body := t04Key("brw-http-ack-token"), brwHTTPTokenBody(plan.AttemptID)
	loss.armed.Store(true)
	denied := brwHTTPCall(t, handler, "POST", tokenPath, h.logins.a, key, body, 500)
	brwHTTPNoToken(t, denied)
	if !loss.committed.Load() {
		t.Fatal("token route did not lose a truly committed reservation ACK")
	}
	var issued, expires int64
	if err := h.lp.f.owner.QueryRow(context.Background(), `SELECT grant_iat,grant_exp FROM live.media_input_custody WHERE attempt_id=$1`, plan.AttemptID).
		Scan(&issued, &expires); err != nil || issued <= 0 || expires <= issued {
		t.Fatalf("ACK loss was not after durable grant: issued=%d expires=%d err=%v", issued, expires, err)
	}
	if got := bicOwnedFacts(t, h.bicHarness); got != [8]int64{1, 1, 1, 1, 1, 1, 1, 1} {
		t.Fatalf("ACK loss duplicated durable input grant: %v", got)
	}
	healthy := httpapi.NewHandler(h.lp.f.runtime, httpapi.Options{Live: h.planner, BrowserInput: h.runtime})
	replay := brwHTTPCall(t, healthy, "POST", tokenPath, h.logins.a, key, body, 200)
	out := brwHTTPFields(t, replay, "attempt_id", "room_name", "publisher_identity", "url", "token", "expires_at")
	var replayExpiry int64
	if json.Unmarshal(out["expires_at"], &replayExpiry) != nil || replayExpiry != expires {
		t.Fatal("ACK-loss replay extended fixed publisher expiry")
	}
}
