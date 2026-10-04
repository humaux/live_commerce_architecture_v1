package metareply

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/integrations/core"
)

func audienceRequest() core.DispatchRequest {
	raw, _ := json.Marshal(map[string]any{"v": 1, "source_id": bundleT, "session_id": storeT, "post_id": "1234567890_99", "asset_id": "1234567890"})
	return core.DispatchRequest{OperationID: opT, Provider: "facebook", Action: "meta.live_insights", Purpose: "service", ExternalAssetID: "1234567890", Request: raw}
}

type audienceCall struct{ method, path, rawQuery, auth string }

func audienceHarness(t *testing.T, handler func(*http.Request, http.ResponseWriter)) (core.DispatchRoute, func() []audienceCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []audienceCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, audienceCall{method: r.Method, path: r.URL.Path, rawQuery: r.URL.RawQuery, auth: r.Header.Get("Authorization")})
		mu.Unlock()
		handler(r, w)
	}))
	t.Cleanup(srv.Close)
	route, err := newAudienceRoute(func(context.Context, string) (string, error) { return "", nil }, testKeyring(t), nil, Config{GraphBaseURL: srv.URL, GraphVersion: "v26.0"})
	if err != nil {
		t.Fatal(err)
	}
	return route, func() []audienceCall { mu.Lock(); defer mu.Unlock(); return append([]audienceCall(nil), calls...) }
}

func TestLiveAudienceReadOnlySnapshot(t *testing.T) {
	route, calls := audienceHarness(t, func(r *http.Request, w http.ResponseWriter) {
		if strings.HasSuffix(r.URL.Path, "/live_videos") {
			if r.URL.Query().Get("after") == "" {
				io.WriteString(w, `{"data":[{"id":"777","post_id":"1234567890_88"}],"paging":{"next":"https://untrusted.invalid/?access_token=private","cursors":{"after":"page2"}}}`)
			} else {
				io.WriteString(w, `{"data":[{"id":"888","post_id":"1234567890_99"}]}`)
			}
		} else {
			io.WriteString(w, `{"data":[{"name":"total_video_views","period":"lifetime","values":[{"value":34}]},{"name":"total_video_view_time_by_age_bucket_and_gender","period":"lifetime","values":[{"value":{"F.25-34":1234,"M.35-44":5600}}]},{"name":"total_video_view_time_by_region_id","period":"lifetime","values":[{"value":{"Taipei":7200}}]}]}`)
		}
	})
	out, err := route.DispatchWithSecret(context.Background(), audienceRequest(), core.NewSecret([]byte(fakeTok)))
	if err != nil || out.State != "SUCCEEDED" || out.Code != "graph_read" || out.Detail == nil {
		t.Fatalf("snapshot=%+v,%v", out, err)
	}
	raw, _ := json.Marshal(out.Detail)
	var snapshot struct {
		Status    string `json:"status"`
		Views     *int64 `json:"views"`
		Peak      *int64 `json:"peak_concurrent"`
		Total     *int64 `json:"total_view_time_ms"`
		AgeGender []struct {
			Bucket     string `json:"bucket"`
			ViewTimeMS int64  `json:"view_time_ms"`
		} `json:"age_gender"`
		Regions []struct {
			Bucket     string `json:"bucket"`
			ViewTimeMS int64  `json:"view_time_ms"`
		} `json:"regions"`
	}
	if json.Unmarshal(raw, &snapshot) != nil || snapshot.Status != "available" || snapshot.Views == nil || *snapshot.Views != 34 || snapshot.Peak != nil || snapshot.Total != nil || len(snapshot.AgeGender) != 2 || len(snapshot.Regions) != 1 || snapshot.AgeGender[0].ViewTimeMS != 1234 || snapshot.Regions[0].ViewTimeMS != 7200 {
		t.Fatalf("projection=%s", raw)
	}
	_ = out == out
	log := calls()
	if len(log) != 3 {
		t.Fatalf("calls=%d", len(log))
	}
	for _, call := range log {
		if call.method != http.MethodGet || call.auth != "Bearer "+fakeTok || strings.Contains(call.rawQuery, "access_token") {
			t.Fatalf("unsafe call: %+v", call)
		}
	}
	q, _ := url.ParseQuery(log[2].rawQuery)
	if log[2].path != "/v26.0/888/video_insights" || q.Get("metric") != "total_video_views,total_video_view_time_by_age_bucket_and_gender,total_video_view_time_by_region_id" {
		t.Fatalf("metric call=%+v", log[2])
	}
}

func TestLiveAudienceInsufficientDemographics(t *testing.T) {
	for _, value := range []string{`{}`, `[]`, `[ ]`, `null`} {
		t.Run(value, func(t *testing.T) {
			route, _ := audienceHarness(t, func(r *http.Request, w http.ResponseWriter) {
				if strings.HasSuffix(r.URL.Path, "/live_videos") {
					io.WriteString(w, `{"data":[{"id":"888","post_id":"1234567890_99"}]}`)
					return
				}
				io.WriteString(w, `{"data":[{"name":"total_video_views","values":[{"value":34}]},{"name":"total_video_view_time_by_age_bucket_and_gender","values":[{"value":`+value+`}]},{"name":"total_video_view_time_by_region_id","values":[{"value":`+value+`}]}]}`)
			})
			out, err := route.DispatchWithSecret(context.Background(), audienceRequest(), core.NewSecret([]byte(fakeTok)))
			snapshot, ok := out.Detail.(*LiveAudienceSnapshot)
			if err != nil || !ok || out.State != "SUCCEEDED" || snapshot.Status != "insufficient" || snapshot.Views == nil || *snapshot.Views != 34 || snapshot.AgeGender == nil || len(snapshot.AgeGender) != 0 || snapshot.Regions == nil || len(snapshot.Regions) != 0 || snapshot.TotalViewTimeMS != nil || snapshot.PeakConcurrent != nil {
				t.Fatalf("empty=%+v,%v", out, err)
			}
		})
	}
}

func TestLiveAudienceBoundVideoAndPagination(t *testing.T) {
	for _, tc := range []struct {
		name, body  string
		calls       int
		state, code string
	}{
		{"unmatched", `{"data":[{"id":"777","post_id":"1234567890_88"}]}`, 1, "UNKNOWN", codeUnproven},
		{"ambiguous", `{"data":[{"id":"777","post_id":"1234567890_99"},{"id":"888","post_id":"1234567890_99"}]}`, 1, "UNKNOWN", codeUnproven},
		{"unknown video path", `{"data":[{"id":"evil/path","post_id":"1234567890_99"}]}`, 1, "FAILED_FINAL", "bad_result"},
		{"missing cursor", `{"data":[],"paging":{"next":"https://untrusted.invalid"}}`, 1, "UNKNOWN", codeUnproven},
		{"unsafe cursor", `{"data":[],"paging":{"next":"https://untrusted.invalid","cursors":{"after":"x?token=private"}}}`, 1, "UNKNOWN", codeUnproven},
		{"cursor cycle", `{"data":[],"paging":{"next":"https://untrusted.invalid","cursors":{"after":"same"}}}`, 2, "UNKNOWN", codeUnproven},
		{"malformed list", `{"data":null}`, 1, "UNKNOWN", codeUnconfirmed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			route, calls := audienceHarness(t, func(_ *http.Request, w http.ResponseWriter) { io.WriteString(w, tc.body) })
			out, err := route.DispatchWithSecret(context.Background(), audienceRequest(), core.NewSecret([]byte(fakeTok)))
			if err != nil || out.State != tc.state || out.Code != tc.code || out.Detail != nil || len(calls()) != tc.calls {
				t.Fatalf("out=%+v,%v calls=%d", out, err, len(calls()))
			}
		})
	}
	page := 0
	route, calls := audienceHarness(t, func(_ *http.Request, w http.ResponseWriter) {
		page++
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "paging": map[string]any{"next": "https://untrusted.invalid", "cursors": map[string]string{"after": strconv.Itoa(page)}}})
	})
	if out, _ := route.DispatchWithSecret(context.Background(), audienceRequest(), core.NewSecret([]byte(fakeTok))); out.Code != codeUnproven || len(calls()) != 10 {
		t.Fatalf("page bound=%+v,%d", out, len(calls()))
	}
}

func TestLiveAudiencePolicyAndMalformedRequest(t *testing.T) {
	checked := 0
	keys := testKeyring(t)
	check := func(context.Context, string) (string, error) { checked++; return "source_off", nil }
	route, err := newAudienceRoute(check, keys, nil, Config{GraphBaseURL: GraphHost, GraphVersion: "v26.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := route.Check(context.Background(), audienceRequest()); !errors.Is(err, core.ErrPolicyDenied) || checked != 1 {
		t.Fatalf("check=%v count=%d", err, checked)
	}
	for _, mutate := range []func(map[string]any){
		func(r map[string]any) { r["source_id"] = "bad" }, func(r map[string]any) { r["session_id"] = "bad" }, func(r map[string]any) { r["post_id"] = "evil/path" }, func(r map[string]any) { r["asset_id"] = "different" }, func(r map[string]any) { r["post_id"] = "999_99" }, func(r map[string]any) { r["v"] = 2 }, func(r map[string]any) { r["extra"] = "unknown" },
	} {
		req := audienceRequest()
		var body map[string]any
		json.Unmarshal(req.Request, &body)
		mutate(body)
		req.Request, _ = json.Marshal(body)
		if err := route.Check(context.Background(), req); !errors.Is(err, core.ErrPolicyDenied) {
			t.Fatalf("accepted bad request: %v", err)
		}
		if checked != 1 {
			t.Fatal("malformed request reached SQL")
		}
		out, err := route.DispatchWithSecret(context.Background(), req, core.NewSecret([]byte(fakeTok)))
		if err != nil || out.State != "FAILED_FINAL" || out.Code != "bad_request" {
			t.Fatalf("invalid dispatch=%+v,%v", out, err)
		}
	}
	if _, err := newAudienceRoute(nil, keys, nil, Config{GraphBaseURL: GraphHost, GraphVersion: "v26.0"}); err == nil {
		t.Fatal("nil check accepted")
	}
	if _, err := AudienceRoutes(nil, keys, nil, Config{GraphBaseURL: GraphHost, GraphVersion: "v26.0"}); err == nil {
		t.Fatal("nil pool accepted")
	}
	allowed, err := newAudienceRoute(func(context.Context, string) (string, error) { return "", nil }, keys, nil, Config{GraphBaseURL: GraphHost, GraphVersion: "v26.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := allowed.Check(context.Background(), audienceRequest()); err != nil {
		t.Fatalf("allowed=%v", err)
	}
	if allowed.Provider != "facebook" || allowed.Purpose != "service" || allowed.Action != "meta.live_insights" || allowed.LoadSecret == nil || allowed.Finish == nil || allowed.ReconcileWithSecret != nil {
		t.Fatal("route custody/shape changed")
	}
}

func TestLiveAudienceCredentialScopes(t *testing.T) {
	keys := testKeyring(t)
	route, err := newAudienceRoute(func(context.Context, string) (string, error) { return "", nil }, keys, nil, Config{GraphBaseURL: GraphHost, GraphVersion: "v26.0"})
	if err != nil {
		t.Fatal(err)
	}
	scope := PageTokenScope{TenantID: tenantT, StoreID: storeT, BindingID: bindingT, Provider: "facebook", AssetID: "1234567890", Version: 4}
	k, n, c, err := keys.Seal(scope, fakeTok)
	if err != nil {
		t.Fatal(err)
	}
	claim := core.SecretClaim{OperationID: opT, Generation: 2, LeaseToken: bytes32(9)}
	for _, tc := range []struct {
		name   string
		scopes []string
		ok     bool
	}{
		{"exact readonly", []string{"read_insights", "pages_read_engagement"}, true},
		{"one missing", []string{"read_insights"}, false},
		{"other missing", []string{"pages_read_engagement"}, false},
		{"messaging alone", []string{"pages_messaging"}, false},
		{"none", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &fakeTx{row: []any{tenantT, storeT, bindingT, "facebook", "1234567890", int64(4), k, n, c, tc.scopes}}
			secret, err := route.LoadSecret(context.Background(), tx, claim)
			if tc.ok {
				if err != nil || string(secret.Reveal()) != fakeTok {
					t.Fatalf("exact read scopes denied=%v", err)
				}
			} else if !errors.Is(err, core.ErrPolicyDenied) {
				t.Fatalf("missing scope accepted=%v", err)
			}
			if !strings.Contains(tx.gotSQL, "integration.load_meta_audience_token") || len(tx.gotArg) != 3 || tx.gotArg[0] != opT || tx.gotArg[1] != int64(2) {
				t.Fatal("wrong SQL loader/fence")
			}
		})
	}
	if _, err := route.LoadSecret(context.Background(), &fakeTx{}, claim); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("missing row=%v", err)
	}
	bad := &fakeTx{row: []any{tenantT, storeT, bindingT, "instagram", "1234567890", int64(4), k, n, c, []string{"read_insights", "pages_read_engagement"}}}
	if _, err := route.LoadSecret(context.Background(), bad, claim); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("wrong provider=%v", err)
	}
}

func TestLiveAudienceFailureNoReplay(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		state, code string
	}{
		{"permission HTTP", 403, `{"error":{"message":"private"}}`, "FAILED_FINAL", "permission_denied"},
		{"permission Graph", 400, `{"error":{"code":190,"message":"private"}}`, "FAILED_FINAL", "permission_denied"},
		{"server uncertainty", 503, `{"error":{"code":100}}`, "UNKNOWN", codeUnconfirmed},
		{"refused", 400, `{"error":{"code":100}}`, "FAILED_FINAL", "graph_refused"},
		{"redirect", 302, ``, "UNKNOWN", codeUnconfirmed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			route, calls := audienceHarness(t, func(_ *http.Request, w http.ResponseWriter) {
				w.Header().Set("Location", "https://untrusted.invalid")
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			})
			out, err := route.DispatchWithSecret(context.Background(), audienceRequest(), core.NewSecret([]byte(fakeTok)))
			if err != nil || out.State != tc.state || out.Code != tc.code || out.Detail != nil || len(calls()) != 1 {
				t.Fatalf("failure=%+v,%v calls=%d", out, err, len(calls()))
			}
			reconciled, err := route.Reconcile(context.Background(), audienceRequest())
			if err != nil || reconciled.State != "UNKNOWN" || reconciled.Code != codeUnproven || len(calls()) != 1 {
				t.Fatalf("replayed=%+v,%v calls=%d", reconciled, err, len(calls()))
			}
		})
	}
	route, calls := audienceHarness(t, func(r *http.Request, w http.ResponseWriter) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	out, err := route.DispatchWithSecret(ctx, audienceRequest(), core.NewSecret([]byte(fakeTok)))
	if err != nil || out.State != "UNKNOWN" || len(calls()) != 1 {
		t.Fatalf("timeout=%+v,%v calls=%d", out, err, len(calls()))
	}
}

type audienceFinishTx struct {
	pgx.Tx
	sql  string
	args []any
	err  error
}

func (tx *audienceFinishTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx.sql = sql
	tx.args = args
	return pgconn.NewCommandTag("SELECT 1"), tx.err
}

func TestLiveAudienceFinishFence(t *testing.T) {
	claim := core.SecretClaim{OperationID: opT, Generation: 2, LeaseToken: bytes32(9), Mode: "dispatch"}
	for _, out := range []core.Outcome{{State: "UNKNOWN", Code: codeUnconfirmed}, {State: "FAILED_FINAL", Code: "permission_denied"}, {State: "SUCCEEDED", Code: "graph_read"}} {
		tx := &audienceFinishTx{}
		if err := finishAudience(context.Background(), tx, claim, out); err != nil || tx.sql != "" {
			t.Fatal("untyped/non-success wrote snapshot")
		}
	}
	snapshot := &LiveAudienceSnapshot{Status: "insufficient", AgeGender: []LiveAudienceBucket{}, Regions: []LiveAudienceBucket{}}
	tx := &audienceFinishTx{}
	if err := finishAudience(context.Background(), tx, claim, core.Outcome{State: "SUCCEEDED", Code: "graph_read", Detail: snapshot}); err != nil || !strings.Contains(tx.sql, "integration.finish_meta_audience") || len(tx.args) != 5 || tx.args[0] != opT || tx.args[1] != int64(2) || string(tx.args[2].([]byte)) != string(claim.LeaseToken) || tx.args[3] != "dispatch" {
		t.Fatalf("finish=%s,%v", tx.sql, err)
	}
	var body LiveAudienceSnapshot
	if json.Unmarshal(tx.args[4].([]byte), &body) != nil || body.Status != "insufficient" || body.Views != nil || body.AgeGender == nil || body.Regions == nil {
		t.Fatal("bad JSON projection")
	}
	tx = &audienceFinishTx{err: errors.New("fixture refusal")}
	if err := finishAudience(context.Background(), tx, claim, core.Outcome{State: "SUCCEEDED", Code: "graph_read", Detail: snapshot}); err == nil {
		t.Fatal("SQL failure swallowed")
	}
}

func TestLiveAudienceMetricValidation(t *testing.T) {
	for _, value := range []string{`-1`, `1.5`, `9223372036854775808`, `"wrong"`, `true`} {
		if _, ok := parseAudienceMetrics([]json.RawMessage{json.RawMessage(`{"name":"total_video_views","values":[{"value":` + value + `}]} `)}); ok {
			t.Fatalf("bad count accepted=%s", value)
		}
	}
	for _, raw := range []string{
		`{"name":"total_video_views","period":"day","values":[{"value":1}]}`,
		`{"name":"total_video_views","values":[{"value":1},{"value":2}]}`,
		`{"name":"unsupported_peak","values":[{"value":5}]}`,
		`{"name":"total_video_view_time_by_region_id","values":[{"value":{"<script>":12}}]}`,
		`{"name":"total_video_view_time_by_region_id","values":[{"value":{"Taipei":1.5}}]}`,
	} {
		if _, ok := parseAudienceMetrics([]json.RawMessage{json.RawMessage(raw)}); ok {
			t.Fatalf("invalid metric accepted=%s", raw)
		}
	}
	raw := json.RawMessage(`{"name":"total_video_views","values":[{"value":1}]}`)
	if _, ok := parseAudienceMetrics([]json.RawMessage{raw, raw}); ok {
		t.Fatal("duplicate metric accepted")
	}
}
