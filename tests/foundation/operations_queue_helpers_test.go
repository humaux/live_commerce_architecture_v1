// Purpose: shared fixtures and HTTP helpers of the W6-05B operations-ledger real-PG gates: oqEnv (t06 fixture + the real merchant handler + a read-only merchant), oqOp planning/forcing helpers, owner-level
//   state/job/audit probes and the JSON key assertions. No test lives here.
// Depends on: external_operation_test.go (t06GoFixture), internal/httpapi, internal/integrations/core, River (insert-only client), migration 0159.
// Used by: operations_queue_test.go, operations_queue_flow_test.go.
// Invariants: fixtures write operation state only as the database owner (never through a worker path); every row they create is removed by the t06 fixture cleanup or this env's own cleanup.
// Status: REAL_PG + MOCK (no network).

package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/httpapi"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

// oqEnv is the t06 fixture plus the real merchant handler (with the insert-only River client the query/retry routes need).
type oqEnv struct {
	*t06GoFixture
	handler  http.Handler
	jobs     *river.Client[pgx.Tx]
	readOnly string // token of a principal holding only store:read + integration:read
}

func newOQEnv(t *testing.T) *oqEnv {
	t.Helper()
	f := newT06GoFixture(t)
	jobs, err := river.NewClient[pgx.Tx](riverpgxv5.New(f.base.runtime), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	e := &oqEnv{t06GoFixture: f, jobs: jobs, handler: httpapi.NewHandler(f.base.runtime, httpapi.Options{OperationJobs: jobs}), readOnly: randomToken()}
	ctx := context.Background()
	reader := randomUUID()
	tx, err := f.base.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO identity.principals(id) VALUES($1)`, reader); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.memberships(tenant_id,principal_id) VALUES($1,$2)`, f.tenant, reader); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'store:read'),($1,$2,$3,'integration:read')`, f.tenant, f.store, reader); err != nil {
		t.Fatal(err)
	}
	if err = insertSession(ctx, tx, e.readOnly, reader, "merchant", t06GoFuture(), nil); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = f.base.owner.Exec(c, `DELETE FROM inbox.outbound_messages WHERE tenant_id=$1`, f.tenant)
		_, _ = f.base.owner.Exec(c, `DELETE FROM identity.sessions WHERE principal_id=$1`, reader)
		_, _ = f.base.owner.Exec(c, `DELETE FROM identity.store_grants WHERE principal_id=$1`, reader)
		_, _ = f.base.owner.Exec(c, `DELETE FROM identity.memberships WHERE principal_id=$1`, reader)
		_, _ = f.base.owner.Exec(c, `DELETE FROM identity.principals WHERE id=$1`, reader)
	})
	return e
}

// oqOp is a planned operation fixture.
type oqOp struct {
	ID      string
	Job     int64
	Binding integration.Binding
}

// op registers a binding of provider and plans one READY operation of (action, purpose) with the given frozen request, in the fixture's first store.
func (e *oqEnv) op(t *testing.T, provider, action, purpose, request string) oqOp {
	t.Helper()
	ctx := context.Background()
	var b integration.Binding
	err := e.scoped(ctx, e.token, e.store, func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		b, err = e.service.RegisterBinding(ctx, tx, scope, e.token, uniqueAction("oq.bind"), provider, "asset:"+uniqueAction("oq"))
		return err
	})
	if err != nil {
		t.Fatalf("register binding %s: %v", provider, err)
	}
	var p integration.PlanResult
	err = e.scoped(ctx, e.token, e.store, func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		p, err = e.service.Plan(ctx, tx, scope, e.token, uniqueAction("oq.plan"), integration.PlanInput{
			BindingID: b.ID, ExpectedBindingVersion: b.SemanticVersion, Purpose: purpose, Action: action, Request: json.RawMessage(request)})
		return err
	})
	if err != nil {
		t.Fatalf("plan %s/%s: %v", provider, action, err)
	}
	return oqOp{ID: p.OperationID, Job: p.JobID, Binding: b}
}

// lane plans an operation of the default lane and re-labels it (and its binding) as provider/action in another lane, owner-level fixture only: Plan itself refuses to
// enqueue an ads-lane operation's job on the default queue.
func (e *oqEnv) lane(t *testing.T, provider, action, purpose, request string) oqOp {
	t.Helper()
	o := e.op(t, "mock_provider", "payment.authorize", purpose, request)
	if _, err := e.base.owner.Exec(context.Background(), `UPDATE integration.bindings SET provider=$2 WHERE id=$1`, o.Binding.ID, provider); err != nil {
		t.Fatal(err)
	}
	if _, err := e.base.owner.Exec(context.Background(), `UPDATE integration.operations SET provider=$2,action=$3 WHERE id=$1`, o.ID, provider, action); err != nil {
		t.Fatal(err)
	}
	return o
}

// mock plans a mock_provider/payment.authorize operation (an unregistered, effect-class kind).
func (e *oqEnv) mock(t *testing.T) oqOp {
	t.Helper()
	return e.op(t, "mock_provider", "payment.authorize", "transactional", `{"v":1}`)
}

// live plans a facebook/meta.live_videos operation (a registered read-only retry kind).
func (e *oqEnv) live(t *testing.T) oqOp {
	t.Helper()
	return e.op(t, "facebook", "meta.live_videos", "service", `{"v":1,"binding_id":"`+randomUUID()+`","asset_id":"p1"}`)
}

// set forces the owner-level state of an operation (fixture only). lease is "", "active" or "expired".
func (e *oqEnv) set(t *testing.T, id, state string, generation int64, lease, code string) {
	t.Helper()
	mode := ""
	if lease != "" {
		mode = "reconcile"
		if state == "DISPATCHING" {
			mode = "dispatch"
		}
	}
	if _, err := e.base.owner.Exec(context.Background(), `UPDATE integration.operations SET state=$2,generation=$3,generation_floor=0,lease_mode=$4,
		lease_until=CASE $5 WHEN 'active' THEN clock_timestamp()+interval '10 minutes' WHEN 'expired' THEN clock_timestamp()-interval '10 minutes' END,
		lease_token_hash=CASE WHEN $5='' THEN NULL ELSE sha256('x'::bytea) END,result_code=$6 WHERE id=$1`, id, state, generation, mode, lease, code); err != nil {
		t.Fatalf("force state %s: %v", state, err)
	}
}

// killJobs finishes every live job of the operation (as River would after completion/discard/cancel).
func (e *oqEnv) killJobs(t *testing.T, id string) {
	t.Helper()
	if _, err := e.base.owner.Exec(context.Background(), `UPDATE river.river_job SET state='completed',finalized_at=clock_timestamp()
		WHERE kind='external_operation_v1' AND args->>'operation_id'=$1 AND state IN ('available','scheduled','retryable','pending')`, id); err != nil {
		t.Fatal(err)
	}
}

type oqJob struct {
	ID       int64
	Queue    string
	Priority int
	State    string
	Args     string
}

func (e *oqEnv) jobRows(t *testing.T, id string) []oqJob {
	t.Helper()
	rows, err := e.base.owner.Query(context.Background(), `SELECT id,queue,priority,state::text,args::text FROM river.river_job WHERE kind='external_operation_v1' AND args->>'operation_id'=$1 ORDER BY id`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []oqJob
	for rows.Next() {
		var j oqJob
		if err := rows.Scan(&j.ID, &j.Queue, &j.Priority, &j.State, &j.Args); err != nil {
			t.Fatal(err)
		}
		out = append(out, j)
	}
	return out
}

func (e *oqEnv) row(t *testing.T, id string) (state, code string, generation, floor int64) {
	t.Helper()
	if err := e.base.owner.QueryRow(context.Background(), `SELECT state,result_code,generation,generation_floor FROM integration.operations WHERE id=$1`, id).Scan(&state, &code, &generation, &floor); err != nil {
		t.Fatal(err)
	}
	return
}

func (e *oqEnv) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := e.base.owner.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *oqEnv) events(t *testing.T, id, reason string) int {
	return e.count(t, `SELECT count(*) FROM integration.operation_events WHERE operation_id=$1 AND reason_code=$2`, id, reason)
}

func (e *oqEnv) audits(t *testing.T, action string) int {
	return e.count(t, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action=$3`, e.tenant, e.store, action)
}

type oqResp struct {
	Status int
	Body   []byte
	M      map[string]any
}

func (r oqResp) code() string {
	if e, ok := r.M["code"].(string); ok {
		return e
	}
	return ""
}

// call drives the real handler. body "" sends no body (GET); key is the Idempotency-Key of a POST.
func (e *oqEnv) call(t *testing.T, method, store, path, token, key, body string) oqResp {
	t.Helper()
	url := "/v1/admin/stores/" + store + "/operations" + path
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, url, nil)
	} else {
		req = httptest.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if method == http.MethodPost {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	out := oqResp{Status: rec.Code, Body: rec.Body.Bytes()}
	_ = json.Unmarshal(out.Body, &out.M)
	return out
}

func (e *oqEnv) post(t *testing.T, id, action string, attempts int64) oqResp {
	t.Helper()
	return e.call(t, "POST", e.store, "/"+id+"/"+action, e.token, uniqueAction("oq-key"), fmt.Sprintf(`{"expected_attempts":%d}`, attempts))
}

func oqKeys(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func oqSame(t *testing.T, what string, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s keys = %v, want exactly %v", what, got, want)
	}
}

func oqAction(m map[string]any, name string) (bool, string) {
	a := m["actions"].(map[string]any)[name].(map[string]any)
	return a["available"].(bool), a["reason"].(string)
}

func oqItems(t *testing.T, r oqResp) []map[string]any {
	t.Helper()
	if r.Status != 200 {
		t.Fatalf("list status %d: %s", r.Status, r.Body)
	}
	raw, _ := r.M["items"].([]any)
	var out []map[string]any
	for _, x := range raw {
		out = append(out, x.(map[string]any))
	}
	return out
}

func oqIDs(items []map[string]any) []string {
	var out []string
	for _, i := range items {
		out = append(out, i["operation_id"].(string))
	}
	return out
}
