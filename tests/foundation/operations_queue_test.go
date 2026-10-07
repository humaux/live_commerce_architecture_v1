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
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

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
	pageBinding := randomUUID() // the Page binding the merchant asked to read (request.binding_id), not the operation's own binding row
	videos := e.op(t, "facebook", "meta.live_videos", "service", `{"v":1,"binding_id":"`+pageBinding+`","asset_id":"p1"}`)
	broken := e.op(t, "ecpay_logistics", "ecpay.cvs_create", "transactional", `{"order_id":"not-a-uuid","attempt":1}`)
	plain := e.mock(t)
	// The DM resolves its conversation through the send projection, not the request.
	if _, err := e.base.owner.Exec(context.Background(), `INSERT INTO inbox.outbound_messages(tenant_id,store_id,id,conversation_id,kind,operation_id,principal_id,key_id,nonce,ciphertext,body_hmac)
		VALUES($1,$2,$3,$4,'dm',$5,$6,'k1',decode('000000000000000000000000','hex'),decode('00000000000000000000000000000000ff','hex'),sha256('x'::bytea))`,
		e.tenant, e.store, randomUUID(), conv, dm.ID, e.principal); err != nil {
		t.Fatal(err)
	}
	all := []oqOp{reply, dm, ad, capi, insights, videos, broken, plain}
	for _, o := range all {
		e.set(t, o.ID, "FAILED_FINAL", 1, "", "x")
	}
	want := map[string][2]string{reply.ID: {"claim_bundle", bundle}, dm.ID: {"conversation", conv}, ad.ID: {"ad_draft", draft}, capi.ID: {"payment_attempt", attempt},
		insights.ID: {"live_session", session}, videos.ID: {"binding", pageBinding}}
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
		if tc.action == "meta.ads.pause" {
			// A pending pause is what stops spend: cancelling it would leave the ad running past its approved budget (P1-1 of the review).
			if r := e.post(t, y.ID, "cancel", 0); r.Status != 409 || r.code() != "protective_operation" {
				t.Errorf("cancel of a READY pause: %d %s, want 409 protective_operation", r.Status, r.Body)
			}
			if st, _, gen, _ := e.row(t, y.ID); st != "READY" || gen != 0 {
				t.Errorf("a refused pause cancel changed the row to %s gen=%d", st, gen)
			}
			if ok, why := oqAction(e.call(t, "GET", e.store, "/"+y.ID, e.token, "", "").M, "cancel"); ok || why != "protective_operation" {
				t.Errorf("READY pause DTO cancel = %v %q, want unavailable protective_operation", ok, why)
			}
			continue
		}
		if r := e.post(t, y.ID, "cancel", 0); r.Status != 200 || r.M["state"] != "CANCELLED" {
			t.Errorf("%s cancel of a READY ads operation: %d %s", tc.action, r.Status, r.Body)
		}
	}
}

// Stop-class (protective) operations are never cancellable from the ledger, in any state and in any lane, fail-safe by action name; ordinary kinds still are.
func TestOperationsQueueProtectiveOperationsCannotBeCancelled(t *testing.T) {
	e := newOQEnv(t)
	for _, action := range []string{"payment.pause", "campaign.pause_all", "egress.stop", "meta.ads.disable", "stripe.revoke", "meta.unsubscribe"} {
		for _, st := range []struct {
			state string
			gen   int64
		}{{"READY", 0}, {"UNKNOWN", 2}, {"FAILED_FINAL", 1}} {
			x := e.mock(t)
			e.relabel(t, x.ID, action)
			e.set(t, x.ID, st.state, st.gen, "", "x")
			if r := e.post(t, x.ID, "cancel", st.gen); r.Status != 409 || r.code() != "protective_operation" {
				t.Errorf("cancel %s/%s: %d %s, want 409 protective_operation", action, st.state, r.Status, r.Body)
			}
			if got, _, _, _ := e.row(t, x.ID); got != st.state {
				t.Errorf("refused cancel moved %s/%s to %s", action, st.state, got)
			}
		}
	}
	for _, action := range []string{"payment.authorize", "payment.restart", "meta.dm_send", "ecpay.cvs_create"} {
		x := e.mock(t)
		e.relabel(t, x.ID, action)
		if r := e.post(t, x.ID, "cancel", 0); r.Status != 200 {
			t.Errorf("cancel of an ordinary %s: %d %s", action, r.Status, r.Body)
		}
	}
}

// The per-operation hard cap on `query`: five per rolling 24 h, then 429 query_limit with Retry-After; the oldest query leaving the window lifts it.
func TestOperationsQueueQueryHardCap(t *testing.T) {
	e := newOQEnv(t)
	o := e.mock(t)
	e.set(t, o.ID, "UNKNOWN", 3, "", "reconcile_budget_exhausted")
	e.killJobs(t, o.ID)
	ctx := context.Background()
	// Four earlier queries, spaced well beyond 60 s (1..4 h ago). Events are append-only for every login but the owner, so the fixture inserts them as the owner.
	for h := 1; h <= 4; h++ {
		if _, err := e.base.owner.Exec(ctx, `INSERT INTO integration.operation_events(tenant_id,store_id,operation_id,generation,state,mode,reason_code,created_at)
			VALUES($1,$2,$3,3,'UNKNOWN','','query_requested',clock_timestamp()-make_interval(hours=>$4))`, e.tenant, e.store, o.ID, h); err != nil {
			t.Fatal(err)
		}
	}
	if r := e.post(t, o.ID, "query", 3); r.Status != 200 {
		t.Fatalf("5th query inside the cap: %d %s", r.Status, r.Body)
	}
	e.killJobs(t, o.ID)
	jobs := len(e.jobRows(t, o.ID))
	r := e.post(t, o.ID, "query", 3)
	if r.Status != 429 || r.code() != "query_limit" {
		t.Fatalf("6th query: %d %s, want 429 query_limit (the cap wins over the 60 s spacing)", r.Status, r.Body)
	}
	after, err := strconv.Atoi(r.Header.Get("Retry-After"))
	if err != nil || after < 71900 || after > 72000 { // the oldest counted query is 4 h old: 20 h left
		t.Fatalf("Retry-After = %q (%v), want about 72000 s", r.Header.Get("Retry-After"), err)
	}
	if len(e.jobRows(t, o.ID)) != jobs || e.events(t, o.ID, "query_requested") != 5 {
		t.Errorf("a capped query left a job or event behind (jobs %d->%d, events %d)", jobs, len(e.jobRows(t, o.ID)), e.events(t, o.ID, "query_requested"))
	}
	if ok, why := oqAction(e.call(t, "GET", e.store, "/"+o.ID, e.token, "", "").M, "query"); ok || why != "query_limit" {
		t.Errorf("DTO query = %v %q, want unavailable query_limit", ok, why)
	}
	// Age the oldest query and the real one out of the window/spacing: the cap lifts and the next query is allowed again.
	if _, err := e.base.owner.Exec(ctx, `UPDATE integration.operation_events SET created_at=created_at-interval '21 hours' WHERE operation_id=$1::uuid AND reason_code='query_requested'`, o.ID); err != nil {
		t.Fatal(err)
	}
	if r := e.post(t, o.ID, "query", 3); r.Status != 200 {
		t.Fatalf("query after the window moved on: %d %s", r.Status, r.Body)
	}
}

// The read definer itself is the privacy boundary: called straight on the runtime login (inside a scoped transaction) it returns exactly the documented keys,
// whatever the Go DTO struct later filters. A column added to the SQL object would turn this red even though the HTTP response would not change.
func TestOperationsQueueSQLDTOHasExactKeys(t *testing.T) {
	e := newOQEnv(t)
	ctx := context.Background()
	canary := "CANARY-SQL-DTO-SECRET"
	o := e.op(t, "ecpay_logistics", "ecpay.cvs_create", "transactional", `{"order_id":"`+randomUUID()+`","attempt":1,"access_token":"`+canary+`","phone":"`+canary+`"}`)
	e.set(t, o.ID, "FAILED_FINAL", 2, "", "ecpay.rejected")
	h := sha256.Sum256([]byte(e.readOnly))
	read := func(id any) map[string]any {
		var raw []byte
		if err := e.scoped(ctx, e.readOnly, e.store, func(tx pgx.Tx, scope platform.Scope) error {
			if id == nil {
				return tx.QueryRow(ctx, `SELECT integration.read_operation_ledger($1,$2::uuid,'attention',NULL,NULL,NULL,50)`, h[:], e.store).Scan(&raw)
			}
			return tx.QueryRow(ctx, `SELECT integration.read_operation_ledger($1,$2::uuid,'attention',$3::uuid,NULL,NULL,1)`, h[:], e.store, id).Scan(&raw)
		}); err != nil {
			t.Fatalf("read definer: %v", err)
		}
		if strings.Contains(string(raw), canary) {
			t.Fatalf("the SQL object carries the request canary: %s", raw)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	itemKeys := []string{"operation_id", "provider", "action", "purpose", "state", "reason_code", "attempts", "created_at", "updated_at", "object", "actions"}
	for name, m := range map[string]map[string]any{"list": read(nil), "detail": read(o.ID)} {
		oqSame(t, name+" envelope", oqKeys(m), []string{"items", "has_more", "events"})
		items := m["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("%s: %d items", name, len(items))
		}
		item := items[0].(map[string]any)
		oqSame(t, name+" item", oqKeys(item), append([]string(nil), itemKeys...))
		oqSame(t, name+" object", oqKeys(item["object"].(map[string]any)), []string{"kind", "id"})
		oqSame(t, name+" actions", oqKeys(item["actions"].(map[string]any)), []string{"query", "cancel", "retry"})
		for _, a := range []string{"query", "cancel", "retry"} {
			oqSame(t, name+" action "+a, oqKeys(item["actions"].(map[string]any)[a].(map[string]any)), []string{"available", "reason"})
		}
	}
	detail := read(o.ID)
	events := detail["events"].([]any)
	if len(events) == 0 {
		t.Fatal("detail has no events")
	}
	oqSame(t, "event", oqKeys(events[0].(map[string]any)), []string{"generation", "state", "reason_code", "created_at"})
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
