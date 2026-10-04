package foundation_test

// D9 author integration evidence: actual isolated PG + merchant API + Page-key
// custody + dispatcher + local GET-only Graph. This adapter author cannot be the
// sole acceptance reviewer. Owner SQL below only prepares synthetic scoped
// fixtures, routes their jobs to a private test queue and reads private state.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/claims"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/platform"
)

type atsGraph struct {
	mu        sync.Mutex
	srv       *httptest.Server
	post      string
	available bool
	calls     []string
}

func newATSGraph(t *testing.T, post string) *atsGraph {
	t.Helper()
	g := &atsGraph{post: post}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		available := g.available
		g.calls = append(g.calls, r.Method+" "+r.URL.Path)
		g.mu.Unlock()
		if r.Method != http.MethodGet || r.URL.Query().Has("access_token") || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			w.WriteHeader(400)
			io.WriteString(w, `{"error":{"code":100}}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/live_videos") {
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"id": "77778888", "post_id": g.post}}})
			return
		}
		if r.URL.Path != "/v26.0/77778888/video_insights" {
			w.WriteHeader(404)
			return
		}
		var ages, regions any = map[string]int64{}, map[string]int64{}
		if available {
			ages = map[string]int64{"F.25-34": 1234}
			regions = map[string]int64{"Taipei": 4321}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{
			map[string]any{"name": "total_video_views", "period": "lifetime", "values": []any{map[string]any{"value": 34}}},
			map[string]any{"name": "total_video_view_time_by_age_bucket_and_gender", "period": "lifetime", "values": []any{map[string]any{"value": ages}}},
			map[string]any{"name": "total_video_view_time_by_region_id", "period": "lifetime", "values": []any{map[string]any{"value": regions}}},
		}})
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *atsGraph) count() int    { g.mu.Lock(); defer g.mu.Unlock(); return len(g.calls) }
func (g *atsGraph) setAvailable() { g.mu.Lock(); g.available = true; g.mu.Unlock() }

type atsEnv struct {
	t     *testing.T
	e     *mciEnv
	a     *adsEnv
	g     *atsGraph
	pool  *pgxpool.Pool
	route integration.DispatchRoute
	queue string
}

func newATSEnv(t *testing.T, scopes []string) *atsEnv {
	t.Helper()
	h := lcSetup(t)
	f := h.f
	// Explicit synthetic merchant rights: no implicit ads/live permission or token scopes.
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
 SELECT $1,$2,$3,p FROM unnest(ARRAY['ads:read','integration:execute','integration:manage']) p ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, h.actor)
	keys, err := metareply.NewPageTokenKeyring("ats_key", map[string][]byte{"ats_key": randomBytes(32)})
	if err != nil {
		t.Fatal(err)
	}
	e := &mciEnv{t: t, h: h, page: miSetup(t), pageKeys: keys, session: h.draft(t, f.storeA1), pageAsset: mciDigits(14), pageToken: "SENTINEL-READONLY-PAGE-" + t04Tag() + t04Tag()}
	h.open(t, e.session, claims.MatchExact)
	e.pageBinding = miBinding(t, e.page, e.pageAsset, "facebook", f.tenantA, f.storeA1, h.actor)
	miRoute(t, e.page, e.pageAsset, f.tenantA, f.storeA1, e.pageBinding)
	e.postID = e.pageAsset + "_" + mciDigits(10)
	e.registerToken(t, "facebook", e.pageBinding, e.pageAsset, scopes, e.pageToken)
	e.srcFB = e.mustSource(t, "page", e.pageAsset, e.postID, false)
	a := newAdsEnv(t, adsOpts{fx: f, noWorker: true, noConnect: true})
	g := newATSGraph(t, e.postID)
	pool := miPool(t, f, waClaims)
	routes, err := metareply.AudienceRoutes(pool, keys, nil, metareply.Config{GraphBaseURL: g.srv.URL, GraphVersion: "v26.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("audience route count")
	}
	x := &atsEnv{t: t, e: e, a: a, g: g, pool: pool, route: routes[0], queue: "ats_" + t04Tag()}
	// New snapshot FK must be retired before lcPurgeSessions. Only this task's session.
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM ads.live_audience_snapshots WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, e.session)
	})
	return x
}

func (x *atsEnv) plan() adsResp {
	return x.a.api(http.MethodPost, "/sessions/"+x.e.session+"/audience-read", x.e.h.token, map[string]string{"Idempotency-Key": "ats-" + t04Tag()}, nil)
}

// Diagnostic calls retain the same runtime role/auth scope; rollback on every
// path. They expose only SQLSTATE/message for synthetic fixtures, never tokens.
func (x *atsEnv) diagnose(plan bool, day string) {
	x.t.Helper()
	err := x.e.h.do(x.e.h.token, x.e.h.f.storeA1, func(tx pgx.Tx, _ platform.Scope) error {
		var out json.RawMessage
		if !plan {
			return tx.QueryRow(context.Background(), `SELECT ads.attribution_report($1,$2,$3::date,$3::date)`, tokenHash(x.e.h.token), x.e.h.f.storeA1, day).Scan(&out)
		}
		var op string
		if err := tx.QueryRow(context.Background(), `SELECT gen_random_uuid()::text`).Scan(&op); err != nil {
			return err
		}
		jobs, err := newInsertOnlyClient(x.e.h.f)
		if err != nil {
			return err
		}
		job, err := integration.InsertOperationJob(context.Background(), jobs, tx, op)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(context.Background(), `SELECT integration.plan_meta_audience($1,$2,$3,$4,$5)`, tokenHash(x.e.h.token), x.e.h.f.storeA1, x.e.session, op, job).Scan(&out); err != nil {
			return err
		}
		return errors.New("diagnostic unexpectedly allowed; rollback")
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		x.t.Logf("runtime diagnostic SQLSTATE=%s message=%s", pg.Code, pg.Message)
	} else {
		x.t.Logf("runtime diagnostic non-PG error=%v", err)
	}
}

func (x *atsEnv) mustPlan() string {
	x.t.Helper()
	r := x.plan()
	if r.Status != http.StatusOK {
		x.t.Fatalf("audience plan status=%d body=%s", r.Status, r.Raw)
	}
	op, ok := r.JSON["operation_id"].(string)
	if !ok || op == "" || r.JSON["state"] != "READY" {
		x.t.Fatalf("plan shape=%s", r.Raw)
	}
	var provider, action, purpose, queue string
	var request json.RawMessage
	if err := x.e.h.f.owner.QueryRow(context.Background(), `SELECT o.provider,o.action,o.purpose,o.request,j.queue FROM integration.operations o JOIN river.river_job j ON j.id=o.job_id WHERE o.id=$1`, op).Scan(&provider, &action, &purpose, &request, &queue); err != nil {
		x.t.Fatal(err)
	}
	var body map[string]any
	json.Unmarshal(request, &body)
	if provider != "facebook" || action != "meta.live_insights" || purpose != "service" || queue != "default" || body["source_id"] != x.e.srcFB || body["post_id"] != x.e.postID || body["asset_id"] != x.e.pageAsset || body["session_id"] != x.e.session {
		x.t.Fatalf("frozen plan scope=%s", request)
	}
	return op
}

func (x *atsEnv) run(op string) {
	x.t.Helper()
	mustExec(x.t, x.e.h.f.owner, `UPDATE river.river_job SET queue=$1 WHERE state='available' AND id=(SELECT job_id FROM integration.operations WHERE id=$2)`, x.queue, op)
}

func (x *atsEnv) reportAudience() map[string]any {
	x.t.Helper()
	tz, _ := time.LoadLocation("Asia/Taipei")
	day := time.Now().In(tz).Format("2006-01-02")
	r := x.a.api(http.MethodGet, "/attribution?from="+day+"&to="+day, x.e.h.token, nil, nil)
	if r.Status != 200 {
		x.diagnose(false, day)
		x.t.Fatalf("report status=%d body=%s", r.Status, r.Raw)
	}
	sessions, ok := r.JSON["sessions"].([]any)
	if !ok {
		x.t.Fatalf("sessions=%s", r.Raw)
	}
	for _, raw := range sessions {
		s := raw.(map[string]any)
		if s["session_id"] == x.e.session {
			a, ok := s["live_audience"].(map[string]any)
			if !ok {
				x.t.Fatalf("audience=%s", r.Raw)
			}
			return a
		}
	}
	x.t.Fatalf("bound session missing: %s", r.Raw)
	return nil
}

func TestAdsAttributionSessionAudiencePipeline(t *testing.T) {
	x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
	op := x.mustPlan()
	// Observes actual Page custody + claimed lease in the dispatcher's SQL loader.
	loaded := make(chan integration.SecretClaim, 2)
	route := x.route
	realLoad := route.LoadSecret
	route.LoadSecret = func(ctx context.Context, tx pgx.Tx, c integration.SecretClaim) (integration.Secret, error) {
		s, err := realLoad(ctx, tx, c)
		if err == nil {
			loaded <- c
		}
		return s, err
	}
	t06StartDispatcher(t, x.pool, x.queue, []integration.DispatchRoute{route}, mciDispatchOptions())
	x.run(op)
	x.e.awaitOp(t, op, "SUCCEEDED", 8*time.Second, "completed")
	select {
	case c := <-loaded:
		if c.OperationID != op || c.Generation < 1 || c.Mode != "dispatch" {
			t.Fatal("wrong secret lease")
		}
	default:
		t.Fatal("real token loader not reached")
	}
	if x.g.count() != 2 {
		t.Fatalf("Graph calls=%d", x.g.count())
	}
	var persisted json.RawMessage
	if err := x.e.h.f.owner.QueryRow(context.Background(), `SELECT snapshot FROM ads.live_audience_snapshots WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND operation_id=$4`, x.e.h.f.tenantA, x.e.h.f.storeA1, x.e.session, op).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	a := x.reportAudience()
	if a["status"] != "insufficient" || a["views"] != float64(34) || a["peak_concurrent"] != nil || a["total_view_time_ms"] != nil || len(a["age_gender"].([]any)) != 0 || len(a["regions"].([]any)) != 0 {
		t.Fatalf("insufficient projection=%s", persisted)
	}
	// A new operation replaces the latest snapshot, never adds a duplicate session.
	x.g.setAvailable()
	// R11: synthetic task-owned clock advancement only; the real planner must
	// permit a new read after the completed operation's ten-minute cooldown.
	atsElapsed(t, x, op)
	op2 := x.mustPlan()
	x.run(op2)
	x.e.awaitOp(t, op2, "SUCCEEDED", 8*time.Second, "completed")
	a = x.reportAudience()
	if a["status"] != "available" || a["age_gender"].([]any)[0].(map[string]any)["view_time_ms"] != float64(1234) || a["regions"].([]any)[0].(map[string]any)["view_time_ms"] != float64(4321) {
		t.Fatalf("available projection=%+v", a)
	}
	var n int
	if err := x.e.h.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM ads.live_audience_snapshots WHERE session_id=$1`, x.e.session).Scan(&n); err != nil || n != 1 {
		t.Fatalf("replacement rows=%d err=%v", n, err)
	}
	// Cross-store read cannot project this Page's snapshot.
	_, token := lcPrincipal(t, x.e.h.f, x.e.h.f.tenantA, []string{x.e.h.f.storeA2}, "store:read", "ads:read", "live:read")
	tz, _ := time.LoadLocation("Asia/Taipei")
	day := time.Now().In(tz).Format("2006-01-02")
	r := x.a.apiOn(x.e.h.f.storeA2, http.MethodGet, "/attribution?from="+day+"&to="+day, token, nil, nil)
	if r.Status != 200 || strings.Contains(string(r.Raw), x.e.session) || strings.Contains(string(r.Raw), x.e.postID) {
		t.Fatalf("cross-store audience leak status=%d", r.Status)
	}
}

func TestAdsAttributionSessionAudiencePermissionDenials(t *testing.T) {
	for _, scopes := range [][]string{{"pages_read_engagement"}, {"read_insights"}, {"pages_messaging"}} {
		t.Run(strings.Join(scopes, "+"), func(t *testing.T) {
			x := newATSEnv(t, scopes)
			r := x.plan()
			if r.Status != 403 || x.g.count() != 0 {
				x.diagnose(true, "")
				t.Fatalf("missing readonly scope status=%d body=%s calls=%d", r.Status, r.Raw, x.g.count())
			}
			var n int
			err := x.e.h.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM integration.operations WHERE request->>'session_id'=$1 AND action='meta.live_insights'`, x.e.session).Scan(&n)
			if err != nil || n != 0 {
				t.Fatalf("denied plan wrote operation n=%d,%v", n, err)
			}
		})
	}
	x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
	mustExec(t, x.e.h.f.owner, `UPDATE integration.bindings SET enabled=false WHERE id=$1`, x.e.pageBinding)
	r := x.plan()
	if r.Status != 422 || x.g.count() != 0 {
		x.diagnose(true, "")
		t.Fatalf("unauthorized Page status=%d body=%s", r.Status, r.Raw)
	}
}

func TestAdsAttributionSessionAudienceExactLease(t *testing.T) {
	x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
	op := x.mustPlan()
	token := randomBytes(32)
	var disposition, mode string
	var generation int64
	tx, err := x.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = tx.QueryRow(context.Background(), `SELECT disposition,generation,mode FROM integration.claim_operation($1,30,$2)`, op, token).Scan(&disposition, &generation, &mode)
	if err != nil {
		tx.Rollback(context.Background())
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if disposition != "claimed" || mode != "dispatch" {
		t.Fatalf("claim=%s/%s", disposition, mode)
	}
	claim := integration.SecretClaim{OperationID: op, Generation: generation, LeaseToken: token, Mode: mode}
	tx, err = x.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	secret, err := x.route.LoadSecret(context.Background(), tx, claim)
	tx.Rollback(context.Background())
	if err != nil || string(secret.Reveal()) != x.e.pageToken {
		t.Fatalf("exact custody unavailable: %v", err)
	}
	for _, bad := range []integration.SecretClaim{
		{OperationID: op, Generation: generation + 1, LeaseToken: token, Mode: mode},
		{OperationID: op, Generation: generation, LeaseToken: randomBytes(32), Mode: mode},
	} {
		tx, err = x.pool.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = x.route.LoadSecret(context.Background(), tx, bad)
		tx.Rollback(context.Background())
		if err == nil {
			t.Fatal("stale token/generation loaded Page credentials")
		}
		tx, err = x.pool.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		err = x.route.Finish(context.Background(), tx, bad, integration.Outcome{State: "SUCCEEDED", Code: "graph_read", Detail: &metareply.LiveAudienceSnapshot{Status: "insufficient", AgeGender: []metareply.LiveAudienceBucket{}, Regions: []metareply.LiveAudienceBucket{}}})
		tx.Rollback(context.Background())
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "40001" {
			t.Fatalf("stale Finish must conflict, got=%v", err)
		}
	}
	// Rotation preserves binding identity but revokes this read capability; loader refuses before GET.
	x.e.registerTokenVersion(t, "facebook", x.e.pageBinding, x.e.pageAsset, 1, []string{"pages_messaging"}, "SENTINEL-NOSCOPE-"+t04Tag())
	tx, err = x.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = x.route.LoadSecret(context.Background(), tx, claim)
	tx.Rollback(context.Background())
	if !errors.Is(err, integration.ErrPolicyDenied) {
		t.Fatalf("revoked capability=%v", err)
	}
	if x.g.count() != 0 {
		t.Fatal("direct lease checks called Graph")
	}
	var n int
	err = x.e.h.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM ads.live_audience_snapshots WHERE session_id=$1`, x.e.session).Scan(&n)
	if err != nil || n != 0 {
		t.Fatalf("stale writes=%d %v", n, err)
	}
}

func TestAdsAttributionSessionAudienceOverlappingReads(t *testing.T) {
	for _, newFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "AthenB", true: "BthenA"}[newFirst], func(t *testing.T) {
			x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
			a := x.mustPlan()
			// New planning cannot overlap in R11. Recreate an already-existing
			// legacy overlap only after one real completed cycle and cooldown;
			// retain the adversarial out-of-order snapshot projection assertion.
			t06StartDispatcher(t, x.pool, x.queue, []integration.DispatchRoute{x.route}, mciDispatchOptions())
			x.run(a)
			x.e.awaitOp(t, a, "SUCCEEDED", 8*time.Second, "completed")
			atsElapsed(t, x, a)
			b := x.mustPlan()
			mustExec(t, x.e.h.f.owner, `UPDATE integration.operations SET state='READY',lease_token_hash=NULL,lease_mode='',lease_until=NULL WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, a, x.e.h.f.tenantA, x.e.h.f.storeA1)
			mustExec(t, x.e.h.f.owner, `UPDATE river.river_job SET state='available',finalized_at=NULL,scheduled_at=clock_timestamp(),queue='default' WHERE id=(SELECT job_id FROM integration.operations WHERE id=$1)`, a)
			first, second := a, b
			if newFirst {
				first, second = b, a
			}
			x.run(first)
			x.e.awaitOp(t, first, "SUCCEEDED", 8*time.Second, "completed")
			x.run(second)
			x.e.awaitOp(t, second, "SUCCEEDED", 8*time.Second, "completed")
			var winner string
			if err := x.e.h.f.owner.QueryRow(context.Background(), `SELECT operation_id::text FROM ads.live_audience_snapshots WHERE session_id=$1`, x.e.session).Scan(&winner); err != nil {
				t.Fatal(err)
			}
			if winner != b {
				t.Fatalf("completion order changed newer-request winner: got=%s want=%s", winner, b)
			}
			if x.g.count() != 6 {
				t.Fatalf("overlapping Graph calls=%d", x.g.count())
			}
		})
	}
}
