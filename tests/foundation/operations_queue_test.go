// Purpose: REAL_PG gates of the W6-05B failed/UNKNOWN operations ledger (read model + query/cancel/retry), driven through the full merchant HTTP handler on the ordinary
//   runtime login: store isolation, exact DTO keys without secrets/PII, business-object mapping, per-action refusal reasons, audit/event rows, SQL-level job and authority fences.
// Depends on: external_operation_test.go (t06GoFixture: tenant, two stores, merchant sessions, claims-lane worker), migration 0159, internal/httpapi, internal/integrations/core.
// Used by: scripts/dev/test-local.sh --operations-queue (go test -run '^TestOperationsQueue'), CI foundation gate.
// Invariants: contracts/external-operation-v1.md "Amendment W6-05B"; I06/I07. The provider-effect end-to-end and concurrency gates are in operations_queue_flow_test.go.
// Status: REAL_PG + MOCK provider (no network).

package foundation_test

import (
	"context"
	"crypto/sha256"
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

// The list is store-scoped, attention-only by default, newest first, and another store/tenant's operation is invisible and unreachable.
func TestOperationsQueueListIsStoreScopedAndFiltered(t *testing.T) {
	e := newOQEnv(t)
	a, b, c, d, ok := e.mock(t), e.mock(t), e.mock(t), e.mock(t), e.mock(t)
	e.set(t, a.ID, "FAILED_FINAL", 2, "", "provider_rejected")
	e.set(t, b.ID, "UNKNOWN", 3, "", "reconcile_budget_exhausted")
	e.set(t, c.ID, "BLOCKED_POLICY", 1, "", "policy_denied")
	e.set(t, ok.ID, "SUCCEEDED", 1, "", "mock_observed")
	// d stays READY. A second store of the same tenant holds one UNKNOWN operation the first store must never see.
	var other oqOp
	other.ID = randomUUID()
	var bindingB integration.Binding
	if err := e.scoped(context.Background(), e.token, e.otherStore, func(tx pgx.Tx, scope platform.Scope) error {
		var err error
		bindingB, err = e.service.RegisterBinding(context.Background(), tx, scope, e.token, uniqueAction("oq.otherbind"), "mock_provider", "asset:other")
		if err != nil {
			return err
		}
		p, err := e.service.Plan(context.Background(), tx, scope, e.token, uniqueAction("oq.otherplan"), integration.PlanInput{
			BindingID: bindingB.ID, ExpectedBindingVersion: 1, Purpose: "transactional", Action: "payment.authorize", Request: json.RawMessage(`{"v":1}`)})
		other.ID = p.OperationID
		return err
	}); err != nil {
		t.Fatal(err)
	}
	e.set(t, other.ID, "UNKNOWN", 2, "", "callback_failed")

	list := oqItems(t, e.call(t, "GET", e.store, "", e.token, "", ""))
	got := oqIDs(list)
	want := []string{d.ID, c.ID, b.ID, a.ID} // newest first; SUCCEEDED excluded; the other store's operation excluded
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("attention list = %v, want %v", got, want)
	}
	if r := oqItems(t, e.call(t, "GET", e.store, "?state=FAILED", e.token, "", "")); len(r) != 1 || r[0]["operation_id"] != a.ID || r[0]["state"] != "FAILED_FINAL" {
		t.Fatalf("state=FAILED: %v", r)
	}
	if r := oqItems(t, e.call(t, "GET", e.store, "?state=UNKNOWN", e.token, "", "")); len(r) != 1 || r[0]["operation_id"] != b.ID || r[0]["reason_code"] != "reconcile_budget_exhausted" || r[0]["attempts"].(float64) != 3 {
		t.Fatalf("state=UNKNOWN: %v", r)
	}
	for _, bad := range []string{"?state=SUCCEEDED", "?state=all", "?state=failed", "?state=UNKNOWN&state=READY", "?limit=0", "?limit=101", "?foo=1"} {
		if r := e.call(t, "GET", e.store, bad, e.token, "", ""); r.Status != 422 {
			t.Errorf("%s -> %d, want 422", bad, r.Status)
		}
	}
	// Cross-store: the other store's id is 404 in this store, this store's id is 404 in the other store (same tenant, same merchant), and every action agrees.
	if r := e.call(t, "GET", e.store, "/"+other.ID, e.token, "", ""); r.Status != 404 {
		t.Errorf("detail of another store's operation = %d, want 404", r.Status)
	}
	if r := e.call(t, "GET", e.otherStore, "/"+b.ID, e.token, "", ""); r.Status != 404 {
		t.Errorf("detail through another store = %d, want 404", r.Status)
	}
	for _, act := range []string{"query", "cancel", "retry"} {
		if r := e.call(t, "POST", e.store, "/"+other.ID+"/"+act, e.token, uniqueAction("oq-xs"), `{"expected_attempts":2}`); r.Status != 404 {
			t.Errorf("%s of another store's operation = %d (%s), want 404", act, r.Status, r.Body)
		}
	}
	if st, _, _, _ := e.row(t, other.ID); st != "UNKNOWN" {
		t.Errorf("cross-store action touched the operation: %s", st)
	}
	// The second store lists only its own.
	if r := oqItems(t, e.call(t, "GET", e.otherStore, "", e.token, "", "")); len(r) != 1 || r[0]["operation_id"] != other.ID {
		t.Errorf("other store list: %v", r)
	}
	// A principal with no grant on the store is refused; so is one with integration:read only on a POST.
	if r := e.call(t, "GET", e.store, "", e.missingPermission, "", ""); r.Status != 403 {
		t.Errorf("no integration:read -> %d, want 403", r.Status)
	}
	if r := e.call(t, "GET", e.store, "/"+b.ID, e.readOnly, "", ""); r.Status != 200 {
		t.Errorf("integration:read detail -> %d: %s", r.Status, r.Body)
	}
	for _, act := range []string{"query", "cancel", "retry"} {
		if r := e.call(t, "POST", e.store, "/"+b.ID+"/"+act, e.readOnly, uniqueAction("oq-ro"), `{"expected_attempts":3}`); r.Status != 403 {
			t.Errorf("%s without integration:execute = %d, want 403", act, r.Status)
		}
	}
	if r := e.call(t, "GET", e.store, "", "not-a-real-token-not-a-real-token-not-a-real-token", "", ""); r.Status != 401 {
		t.Errorf("bad token -> %d, want 401", r.Status)
	}
}

// Keyset paging returns every row once, newest first, and the cursor is bound to its filter.
func TestOperationsQueuePaging(t *testing.T) {
	e := newOQEnv(t)
	var ids []string
	for i := 0; i < 5; i++ {
		o := e.mock(t)
		e.set(t, o.ID, "FAILED_FINAL", 1, "", "provider_rejected")
		ids = append([]string{o.ID}, ids...) // newest first
	}
	var seen []string
	cursor := ""
	for page := 0; page < 4; page++ {
		q := "?limit=2"
		if cursor != "" {
			q += "&cursor=" + cursor
		}
		r := e.call(t, "GET", e.store, q, e.token, "", "")
		items := oqItems(t, r)
		seen = append(seen, oqIDs(items)...)
		next, _ := r.M["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	if strings.Join(seen, ",") != strings.Join(ids, ",") {
		t.Fatalf("paged ids = %v, want %v", seen, ids)
	}
	if cursor != "" {
		if r := e.call(t, "GET", e.store, "?limit=2&state=UNKNOWN&cursor="+cursor, e.token, "", ""); r.Status != 422 {
			t.Errorf("cursor reused with another filter = %d, want 422", r.Status)
		}
	}
}

// Exact DTO keys; no request, secret or PII canary, no internal identifier.
func TestOperationsQueueDTOHasExactKeysAndNoSecrets(t *testing.T) {
	e := newOQEnv(t)
	order := randomUUID()
	canaries := []string{"CANARY-ACCESS-TOKEN-91c3", "CANARY-BUYER-PHONE-0912", "CANARY-ADDRESS-LINE", "CANARY-COMMENT-TEXT"}
	o := e.op(t, "ecpay_logistics", "ecpay.cvs_create", "transactional",
		`{"order_id":"`+order+`","attempt":1,"access_token":"`+canaries[0]+`","phone":"`+canaries[1]+`","address":"`+canaries[2]+`","comment":"`+canaries[3]+`"}`)
	e.set(t, o.ID, "FAILED_FINAL", 2, "", "ecpay.rejected")
	if _, err := e.base.owner.Exec(context.Background(), `UPDATE integration.operations SET provider_reference='PROVIDER-REF-CANARY' WHERE id=$1`, o.ID); err != nil {
		t.Fatal(err)
	}
	var semantic string
	if err := e.base.owner.QueryRow(context.Background(), `SELECT semantic_key FROM integration.operations WHERE id=$1`, o.ID).Scan(&semantic); err != nil {
		t.Fatal(err)
	}
	list := e.call(t, "GET", e.store, "", e.token, "", "")
	detail := e.call(t, "GET", e.store, "/"+o.ID, e.token, "", "")
	items := oqItems(t, list)
	if len(items) != 1 || detail.Status != 200 {
		t.Fatalf("list=%d detail=%d: %s", len(items), detail.Status, detail.Body)
	}
	item := []string{"operation_id", "provider", "action", "purpose", "state", "reason_code", "attempts", "created_at", "updated_at", "object", "actions"}
	oqSame(t, "list item", oqKeys(items[0]), append([]string(nil), item...))
	oqSame(t, "detail", oqKeys(detail.M), append(append([]string(nil), item...), "events"))
	for _, m := range []map[string]any{items[0], detail.M} {
		oqSame(t, "object", oqKeys(m["object"].(map[string]any)), []string{"kind", "id"})
		oqSame(t, "actions", oqKeys(m["actions"].(map[string]any)), []string{"query", "cancel", "retry"})
		for _, a := range []string{"query", "cancel", "retry"} {
			oqSame(t, "action "+a, oqKeys(m["actions"].(map[string]any)[a].(map[string]any)), []string{"available", "reason"})
		}
	}
	events := detail.M["events"].([]any)
	if len(events) == 0 {
		t.Fatal("detail has no events")
	}
	oqSame(t, "event", oqKeys(events[0].(map[string]any)), []string{"generation", "state", "reason_code", "created_at"})
	if items[0]["object"].(map[string]any)["kind"] != "order" || items[0]["object"].(map[string]any)["id"] != order {
		t.Errorf("object = %v, want order %s", items[0]["object"], order)
	}
	forbidden := append([]string{"PROVIDER-REF-CANARY", semantic, o.Binding.ID, e.principal, "asset:", "request_hash", "lease", "semantic_key", "external_asset_id", "provider_reference", "principal_id", "binding_id"}, canaries...)
	for _, body := range [][]byte{list.Body, detail.Body} {
		for _, f := range forbidden {
			if strings.Contains(string(body), f) {
				t.Errorf("response leaks %q: %s", f, body)
			}
		}
	}
}

// The business object comes from internal ids only; unmapped kinds and malformed ids are null.
func TestOperationsQueueObjectMapping(t *testing.T) {
	e := newOQEnv(t)
	bundle, draft, attempt, conv, session := randomUUID(), randomUUID(), randomUUID(), randomUUID(), randomUUID()
	reply := e.op(t, "facebook", "meta.public_reply", "service", `{"v":1,"bundle_id":"`+bundle+`"}`)
	dm := e.op(t, "facebook", "meta.dm_send", "service", `{"v":1,"bundle_id":"`+bundle+`"}`)
	ad := e.lane(t, "meta_ads", "meta.ads.pause", "marketing", `{"v":1,"draft_id":"`+draft+`","attempt":1}`)
	capi := e.lane(t, "meta_dataset", "meta.capi.purchase", "marketing", `{"v":1,"attempt_id":"`+attempt+`"}`)
	insights := e.op(t, "facebook", "meta.live_insights", "service", `{"v":1,"session_id":"`+session+`"}`)
	broken := e.op(t, "ecpay_logistics", "ecpay.cvs_create", "transactional", `{"order_id":"not-a-uuid","attempt":1}`)
	plain := e.mock(t)
	// The DM resolves its conversation through the send projection, not the request.
	if _, err := e.base.owner.Exec(context.Background(), `INSERT INTO inbox.outbound_messages(tenant_id,store_id,id,conversation_id,kind,operation_id,principal_id,key_id,nonce,ciphertext,body_hmac)
		VALUES($1,$2,$3,$4,'dm',$5,$6,'k1',decode('000000000000000000000000','hex'),decode('00000000000000000000000000000000ff','hex'),sha256('x'::bytea))`,
		e.tenant, e.store, randomUUID(), conv, dm.ID, e.principal); err != nil {
		t.Fatal(err)
	}
	all := []oqOp{reply, dm, ad, capi, insights, broken, plain}
	for _, o := range all {
		e.set(t, o.ID, "FAILED_FINAL", 1, "", "x")
	}
	want := map[string][2]string{reply.ID: {"claim_bundle", bundle}, dm.ID: {"conversation", conv}, ad.ID: {"ad_draft", draft}, capi.ID: {"payment_attempt", attempt},
		insights.ID: {"live_session", session}}
	for _, i := range oqItems(t, e.call(t, "GET", e.store, "", e.token, "", "")) {
		id := i["operation_id"].(string)
		if w, ok := want[id]; ok {
			obj, _ := i["object"].(map[string]any)
			if obj == nil || obj["kind"] != w[0] || obj["id"] != w[1] {
				t.Errorf("%s object = %v, want %v", i["action"], i["object"], w)
			}
		} else if i["object"] != nil {
			t.Errorf("%s object = %v, want null", i["action"], i["object"])
		}
	}
}

// Cancel: READY only. Audit, event, replay, CAS and the after-dispatch refusals.
func TestOperationsQueueCancelRules(t *testing.T) {
	e := newOQEnv(t)
	o := e.mock(t)
	if r := e.post(t, o.ID, "cancel", 1); r.Status != 409 || r.code() != "operation_changed" {
		t.Fatalf("stale attempts: %d %s", r.Status, r.Body)
	}
	if st, _, _, _ := e.row(t, o.ID); st != "READY" {
		t.Fatalf("a refused cancel changed state to %s", st)
	}
	key := uniqueAction("oq-cancel")
	r := e.call(t, "POST", e.store, "/"+o.ID+"/cancel", e.token, key, `{"expected_attempts":0}`)
	if r.Status != 200 || r.M["state"] != "CANCELLED" || r.M["operation_id"] != o.ID || r.M["attempts"].(float64) != 1 {
		t.Fatalf("cancel: %d %s", r.Status, r.Body)
	}
	st, code, gen, _ := e.row(t, o.ID)
	if st != "CANCELLED" || code != "cancelled_by_merchant" || gen != 1 {
		t.Fatalf("row after cancel: %s/%s/%d", st, code, gen)
	}
	if e.events(t, o.ID, "cancelled_by_merchant") != 1 || e.audits(t, "integration.operation_cancelled") != 1 {
		t.Fatalf("events=%d audits=%d, want 1/1", e.events(t, o.ID, "cancelled_by_merchant"), e.audits(t, "integration.operation_cancelled"))
	}
	var details string
	if err := e.base.owner.QueryRow(context.Background(), `SELECT details::text FROM ops.audit_events WHERE tenant_id=$1 AND action='integration.operation_cancelled'`, e.tenant).Scan(&details); err != nil || !strings.Contains(details, o.ID) || strings.Contains(details, "request") {
		t.Fatalf("audit details %q err=%v", details, err)
	}
	// Idempotent replay: same key + same body = same answer and no second event/audit; same key + another body conflicts.
	again := e.call(t, "POST", e.store, "/"+o.ID+"/cancel", e.token, key, `{"expected_attempts":0}`)
	if again.Status != 200 || string(again.Body) != string(r.Body) || e.events(t, o.ID, "cancelled_by_merchant") != 1 || e.audits(t, "integration.operation_cancelled") != 1 {
		t.Fatalf("replay: %d %s", again.Status, again.Body)
	}
	if c := e.call(t, "POST", e.store, "/"+o.ID+"/cancel", e.token, key, `{"expected_attempts":7}`); c.Status != 409 {
		t.Fatalf("same key other body: %d", c.Status)
	}
	// Once finished, and for anything that may have reached the provider, cancel is refused.
	for _, tc := range []struct {
		state, lease, reason string
		gen                  int64
	}{
		{"CANCELLED", "", "operation_closed", 1}, {"SUCCEEDED", "", "operation_closed", 1}, {"FAILED_FINAL", "", "operation_closed", 1},
		{"DISPATCHING", "active", "already_dispatched", 1}, {"DISPATCHING", "expired", "already_dispatched", 1},
		{"UNKNOWN", "", "already_dispatched", 2}, {"UNKNOWN", "active", "already_dispatched", 2}, {"ACKNOWLEDGED", "", "already_dispatched", 2},
		{"BLOCKED_POLICY", "", "operation_closed", 1}, {"STALE_BINDING", "", "operation_closed", 1},
	} {
		x := e.mock(t)
		e.set(t, x.ID, tc.state, tc.gen, tc.lease, "x")
		r := e.post(t, x.ID, "cancel", tc.gen)
		if r.Status != 409 || r.code() != tc.reason {
			t.Errorf("cancel of %s/%s = %d %s, want 409 %s", tc.state, tc.lease, r.Status, r.Body, tc.reason)
		}
		if st, _, _, _ := e.row(t, x.ID); st != tc.state {
			t.Errorf("refused cancel moved %s to %s", tc.state, st)
		}
	}
	if r := e.call(t, "POST", e.store, "/"+randomUUID()+"/cancel", e.token, uniqueAction("oq-x"), `{"expected_attempts":0}`); r.Status != 404 {
		t.Errorf("unknown id = %d, want 404", r.Status)
	}
}

// Query: only an operation in doubt with no active lease/live job/recent query; it restarts the budget and enqueues exactly one job.
func TestOperationsQueueQueryRules(t *testing.T) {
	e := newOQEnv(t)
	for _, tc := range []struct {
		name, state, lease string
		gen                int64
		reason             string
	}{
		{"ready", "READY", "", 0, "not_in_doubt"}, {"failed", "FAILED_FINAL", "", 2, "not_in_doubt"}, {"succeeded", "SUCCEEDED", "", 1, "not_in_doubt"},
		{"cancelled", "CANCELLED", "", 1, "not_in_doubt"}, {"blocked", "BLOCKED_POLICY", "", 1, "not_in_doubt"},
		{"dispatching active", "DISPATCHING", "active", 1, "lease_active"}, {"unknown active lease", "UNKNOWN", "active", 2, "lease_active"},
	} {
		x := e.mock(t)
		e.set(t, x.ID, tc.state, tc.gen, tc.lease, "x")
		e.killJobs(t, x.ID)
		before := len(e.jobRows(t, x.ID))
		if r := e.post(t, x.ID, "query", tc.gen); r.Status != 409 || r.code() != tc.reason {
			t.Errorf("%s: %d %s, want 409 %s", tc.name, r.Status, r.Body, tc.reason)
		}
		if len(e.jobRows(t, x.ID)) != before {
			t.Errorf("%s: a refused query left a job behind", tc.name)
		}
	}
	// A live job of the operation blocks a second concurrent reconcile stream.
	live := e.mock(t)
	e.set(t, live.ID, "UNKNOWN", 2, "", "callback_failed")
	if r := e.post(t, live.ID, "query", 2); r.Status != 409 || r.code() != "query_in_progress" {
		t.Errorf("live job: %d %s", r.Status, r.Body)
	}
	// Binding disabled: the claim would only answer blocked_binding, so the action says so up front.
	gone := e.mock(t)
	e.set(t, gone.ID, "UNKNOWN", 2, "", "callback_failed")
	e.killJobs(t, gone.ID)
	if _, err := e.base.owner.Exec(context.Background(), `UPDATE integration.bindings SET enabled=false,semantic_version=semantic_version+1 WHERE id=$1`, gone.Binding.ID); err != nil {
		t.Fatal(err)
	}
	if r := e.post(t, gone.ID, "query", 2); r.Status != 409 || r.code() != "binding_changed" {
		t.Errorf("binding disabled: %d %s", r.Status, r.Body)
	}
	// Success: UNKNOWN, ACKNOWLEDGED and an expired DISPATCHING all qualify; state/generation untouched, floor raised, one job, one event, one audit.
	for _, tc := range []struct {
		state, lease string
		gen          int64
	}{{"UNKNOWN", "", 4}, {"ACKNOWLEDGED", "", 2}, {"DISPATCHING", "expired", 1}} {
		x := e.mock(t)
		e.set(t, x.ID, tc.state, tc.gen, tc.lease, "reconcile_budget_exhausted")
		e.killJobs(t, x.ID)
		before := len(e.jobRows(t, x.ID))
		r := e.post(t, x.ID, "query", tc.gen)
		if r.Status != 200 || r.M["state"] != tc.state || r.M["attempts"].(float64) != float64(tc.gen) {
			t.Fatalf("query %s: %d %s", tc.state, r.Status, r.Body)
		}
		st, _, gen, floor := e.row(t, x.ID)
		if st != tc.state || gen != tc.gen || floor != tc.gen {
			t.Errorf("%s after query: state=%s generation=%d floor=%d, want %s/%d/%d", tc.state, st, gen, floor, tc.state, tc.gen, tc.gen)
		}
		jobs := e.jobRows(t, x.ID)
		if len(jobs) != before+1 {
			t.Fatalf("%s: jobs %d -> %d, want one more", tc.state, before, len(jobs))
		}
		nj := jobs[len(jobs)-1]
		var args map[string]any
		_ = json.Unmarshal([]byte(nj.Args), &args)
		if nj.Queue != "default" || nj.State != "available" || len(args) != 2 || args["operation_id"] != x.ID || args["version"] != float64(1) {
			t.Errorf("%s: new job %+v", tc.state, nj)
		}
		if e.events(t, x.ID, "query_requested") != 1 {
			t.Errorf("%s: query_requested events = %d", tc.state, e.events(t, x.ID, "query_requested"))
		}
		// A second query is refused: the new job is live, and once it is gone the 60 s spacing still applies.
		if r := e.post(t, x.ID, "query", tc.gen); r.Status != 409 || r.code() != "query_in_progress" {
			t.Errorf("%s second query while job live: %d %s", tc.state, r.Status, r.Body)
		}
		e.killJobs(t, x.ID)
		if r := e.post(t, x.ID, "query", tc.gen); r.Status != 409 || r.code() != "query_too_soon" {
			t.Errorf("%s second query within 60 s: %d %s", tc.state, r.Status, r.Body)
		}
	}
	if got := e.audits(t, "integration.operation_query_requested"); got != 3 {
		t.Errorf("query audits = %d, want 3", got)
	}
}

// The ads lane (meta_ads, meta_dataset) is read + cancel only: post_river/0015 admits exactly one job per ads operation, so query/retry are refused up front
// (lane_unsupported) and leave no job behind; cancel of a READY ads operation still works.
func TestOperationsQueueAdsLaneIsReadAndCancelOnly(t *testing.T) {
	e := newOQEnv(t)
	for _, tc := range []struct{ provider, action string }{{"meta_ads", "meta.ads.pause"}, {"meta_ads", "meta.ads.activate"}, {"meta_dataset", "meta.capi.purchase"}} {
		x := e.lane(t, tc.provider, tc.action, "marketing", `{"v":1}`)
		e.set(t, x.ID, "UNKNOWN", 2, "", "reconcile_budget_exhausted")
		e.killJobs(t, x.ID)
		before := len(e.jobRows(t, x.ID))
		for _, act := range []string{"query", "retry"} {
			if r := e.post(t, x.ID, act, 2); r.Status != 409 || r.code() != "lane_unsupported" {
				t.Errorf("%s %s: %d %s, want 409 lane_unsupported", tc.action, act, r.Status, r.Body)
			}
		}
		if len(e.jobRows(t, x.ID)) != before || e.events(t, x.ID, "query_requested") != 0 {
			t.Errorf("%s: a refused action left a job or event behind", tc.action)
		}
		detail := e.call(t, "GET", e.store, "/"+x.ID, e.token, "", "")
		if detail.Status != 200 {
			t.Fatalf("ads detail %d", detail.Status)
		}
		for _, act := range []string{"query", "retry"} {
			if ok, why := oqAction(detail.M, act); ok || why != "lane_unsupported" {
				t.Errorf("%s DTO %s = %v %q", tc.action, act, ok, why)
			}
		}
		y := e.lane(t, tc.provider, tc.action, "marketing", `{"v":1}`)
		if r := e.post(t, y.ID, "cancel", 0); r.Status != 200 || r.M["state"] != "CANCELLED" {
			t.Errorf("%s cancel of a READY ads operation: %d %s", tc.action, r.Status, r.Body)
		}
	}
}

// Retry: never UNKNOWN; FAILED_FINAL only for the registered read-only kinds; READY only when it has no live job.
func TestOperationsQueueRetryRules(t *testing.T) {
	e := newOQEnv(t)
	// UNKNOWN-class states always answer reconcile_first (or lease_active while a lease runs); nothing changes, nothing is enqueued.
	for _, tc := range []struct {
		state, lease string
		gen          int64
		reason       string
	}{
		{"UNKNOWN", "", 3, "reconcile_first"}, {"ACKNOWLEDGED", "", 2, "reconcile_first"}, {"DISPATCHING", "expired", 1, "reconcile_first"},
		{"DISPATCHING", "active", 1, "lease_active"}, {"UNKNOWN", "active", 2, "lease_active"},
		{"SUCCEEDED", "", 1, "already_succeeded"}, {"CANCELLED", "", 1, "operation_closed"}, {"BLOCKED_POLICY", "", 1, "operation_closed"}, {"STALE_BINDING", "", 1, "operation_closed"},
	} {
		for _, kind := range []string{"effect", "read"} {
			var x oqOp
			if kind == "effect" {
				x = e.mock(t)
			} else {
				x = e.live(t)
			}
			e.set(t, x.ID, tc.state, tc.gen, tc.lease, "x")
			e.killJobs(t, x.ID)
			before := len(e.jobRows(t, x.ID))
			if r := e.post(t, x.ID, "retry", tc.gen); r.Status != 409 || r.code() != tc.reason {
				t.Errorf("retry %s/%s on %s kind: %d %s, want 409 %s", tc.state, tc.lease, kind, r.Status, r.Body, tc.reason)
			}
			if st, _, gen, _ := e.row(t, x.ID); st != tc.state || gen != tc.gen || len(e.jobRows(t, x.ID)) != before {
				t.Errorf("refused retry changed %s: state=%s generation=%d jobs=%d->%d", tc.state, st, gen, before, len(e.jobRows(t, x.ID)))
			}
		}
	}
	// FAILED_FINAL of an unregistered (effect) kind is not re-opened.
	eff := e.mock(t)
	e.set(t, eff.ID, "FAILED_FINAL", 2, "", "provider_rejected")
	e.killJobs(t, eff.ID)
	if r := e.post(t, eff.ID, "retry", 2); r.Status != 409 || r.code() != "retry_not_supported" {
		t.Fatalf("effect kind retry: %d %s", r.Status, r.Body)
	}
	// FAILED_FINAL of a registered read-only kind is re-opened under the same operation id, with a fresh budget and one new job.
	rd := e.live(t)
	e.set(t, rd.ID, "FAILED_FINAL", 2, "", "provider_rejected")
	e.killJobs(t, rd.ID)
	before := len(e.jobRows(t, rd.ID))
	if r := e.post(t, rd.ID, "retry", 1); r.Status != 409 || r.code() != "operation_changed" {
		t.Fatalf("stale attempts: %d %s", r.Status, r.Body)
	}
	r := e.post(t, rd.ID, "retry", 2)
	if r.Status != 200 || r.M["state"] != "READY" || r.M["operation_id"] != rd.ID || r.M["attempts"].(float64) != 2 {
		t.Fatalf("retry registered kind: %d %s", r.Status, r.Body)
	}
	st, code, gen, floor := e.row(t, rd.ID)
	if st != "READY" || code != "retry_authorized" || gen != 2 || floor != 2 {
		t.Fatalf("row after retry: %s/%s gen=%d floor=%d", st, code, gen, floor)
	}
	if jobs := e.jobRows(t, rd.ID); len(jobs) != before+1 || jobs[len(jobs)-1].State != "available" {
		t.Fatalf("retry jobs: %+v", jobs)
	}
	if e.events(t, rd.ID, "retry_authorized") != 1 || e.audits(t, "integration.operation_retry_authorized") != 1 {
		t.Errorf("events=%d audits=%d", e.events(t, rd.ID, "retry_authorized"), e.audits(t, "integration.operation_retry_authorized"))
	}
	// A binding that was changed since is refused (the claim would turn it STALE_BINDING anyway).
	chg := e.live(t)
	e.set(t, chg.ID, "FAILED_FINAL", 1, "", "provider_rejected")
	e.killJobs(t, chg.ID)
	if _, err := e.base.owner.Exec(context.Background(), `UPDATE integration.bindings SET enabled=false,semantic_version=semantic_version+1 WHERE id=$1`, chg.Binding.ID); err != nil {
		t.Fatal(err)
	}
	if r := e.post(t, chg.ID, "retry", 1); r.Status != 409 || r.code() != "binding_changed" {
		t.Errorf("binding changed: %d %s", r.Status, r.Body)
	}
	// READY with a live job is already queued; READY whose job died (River gave up) is re-queued for every kind, with no state change.
	queued := e.mock(t)
	if r := e.post(t, queued.ID, "retry", 0); r.Status != 409 || r.code() != "already_queued" {
		t.Errorf("READY with live job: %d %s", r.Status, r.Body)
	}
	e.killJobs(t, queued.ID)
	before = len(e.jobRows(t, queued.ID))
	if r := e.post(t, queued.ID, "retry", 0); r.Status != 200 || r.M["state"] != "READY" {
		t.Fatalf("re-queue: %d %s", r.Status, r.Body)
	}
	if st, _, gen, _ := e.row(t, queued.ID); st != "READY" || gen != 0 || len(e.jobRows(t, queued.ID)) != before+1 || e.events(t, queued.ID, "requeue_authorized") != 1 {
		t.Errorf("re-queue: state=%s gen=%d jobs=%d events=%d", st, gen, len(e.jobRows(t, queued.ID)), e.events(t, queued.ID, "requeue_authorized"))
	}
}

// SQL-level fences behind the HTTP layer: the job must be this transaction's own exact job; authority comes from the session token; helpers are private.
func TestOperationsQueueSQLFences(t *testing.T) {
	e := newOQEnv(t)
	ctx := context.Background()
	hash := func(token string) []byte { h := sha256.Sum256([]byte(token)); return h[:] }
	o := e.mock(t)
	e.set(t, o.ID, "UNKNOWN", 2, "", "callback_failed")
	e.killJobs(t, o.ID)
	other := e.mock(t)
	// (1) a job of ANOTHER operation, (2) a job from an EARLIER committed transaction, (3) a job on the wrong queue: all refused with PT422 and nothing written.
	cases := map[string]func(tx pgx.Tx) (int64, error){
		"other operation's job": func(tx pgx.Tx) (int64, error) { return integration.InsertOperationJob(ctx, e.jobs, tx, other.ID) },
		"earlier committed job": func(pgx.Tx) (int64, error) { return other.Job, nil },
		"wrong queue": func(tx pgx.Tx) (int64, error) {
			return integration.InsertOperationJobOn(ctx, e.jobs, tx, o.ID, "ads", 3)
		},
	}
	for name, mk := range cases {
		err := e.scoped(ctx, e.token, e.store, func(tx pgx.Tx, scope platform.Scope) error {
			job, err := mk(tx)
			if err != nil {
				return err
			}
			var raw []byte
			return tx.QueryRow(ctx, `SELECT integration.request_operation_query($1,$2::uuid,$3::uuid,2,$4)`, hash(e.token), e.store, o.ID, job).Scan(&raw)
		})
		if err == nil || !strings.Contains(err.Error(), "invalid_job") {
			t.Errorf("%s: err=%v, want invalid_job", name, err)
		}
	}
	if e.events(t, o.ID, "query_requested") != 0 {
		t.Error("a refused query wrote an event")
	}
	// Called straight on the runtime login (no WithScope, so no pinned app.* scope), the definers refuse: a valid token alone is not enough, the session must be the pinned one.
	for name, token := range map[string]string{"owner token": e.token, "read-only token": e.readOnly, "no grant": e.missingPermission} {
		var raw []byte
		err := e.base.runtime.QueryRow(ctx, `SELECT integration.cancel_operation($1,$2::uuid,$3::uuid,2)`, hash(token), e.store, o.ID).Scan(&raw)
		if err == nil {
			t.Errorf("%s: direct definer call outside WithScope succeeded", name)
		}
	}
	if st, _, _, _ := e.row(t, o.ID); st != "UNKNOWN" {
		t.Errorf("direct definer call changed the state to %s", st)
	}
	// The private helpers have no EXECUTE for the runtime or any worker authority; the four public ones belong to the runtime only.
	for _, sig := range []string{"integration.ledger_auth(bytea,uuid,text)", "integration.ledger_capability(integration.operations,text,bigint)", "integration.ledger_open(bytea,uuid,uuid,bigint,text,bigint)"} {
		for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_ads_worker", "commerce_payment_worker", "commerce_buyer_runtime", "commerce_worker"} {
			var ok bool
			if err := e.base.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2,'EXECUTE')`, role, sig).Scan(&ok); err != nil || ok {
				t.Errorf("%s may execute private %s (err=%v)", role, sig, err)
			}
		}
	}
	for _, sig := range []string{"integration.read_operation_ledger(bytea,uuid,text,uuid,timestamptz,uuid,integer)", "integration.request_operation_query(bytea,uuid,uuid,bigint,bigint)",
		"integration.cancel_operation(bytea,uuid,uuid,bigint)", "integration.retry_operation(bytea,uuid,uuid,bigint,bigint)"} {
		for role, want := range map[string]bool{"commerce_runtime": true, "commerce_claims_worker": false, "commerce_ads_worker": false, "commerce_payment_worker": false, "commerce_buyer_runtime": false, "commerce_buyer_issuer": false} {
			var ok bool
			if err := e.base.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2,'EXECUTE')`, role, sig).Scan(&ok); err != nil || ok != want {
				t.Errorf("%s EXECUTE %s = %v, want %v (err=%v)", role, sig, ok, want, err)
			}
		}
	}
	// generation_floor is written only by the integration writer: neither the merchant runtime nor a worker authority may update it directly.
	for _, q := range []string{`UPDATE integration.operations SET generation_floor=0 WHERE false`} {
		if _, err := e.base.runtime.Exec(ctx, q); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("runtime update of generation_floor: %v", err)
		}
		if _, err := e.worker.Exec(ctx, q); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("worker update of generation_floor: %v", err)
		}
	}
}
