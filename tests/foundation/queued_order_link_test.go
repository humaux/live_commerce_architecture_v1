// Purpose: preserve the sealed URL of a planned for-buyer payment DM across real HTTP regeneration/reminder calls.
// Depends on: disposable PG, real inbox planner/River job/sealed secrets, lbSetup loopback Graph and HTTP router.
// Used by: focused regression acceptance; synthetic order rows are disclosed crOrder fixtures, Meta LIVE NOT_RUN.
// Invariants: I01/I02/I06/I11; same-order planning and link invalidation serialize, no new link/secret/job on refusal.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/inbox"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

func qlHTTP(e *lbEnv, m *merchanttools.ManualOrders, path, body string) (int, map[string]any) {
	h := httpapi.NewHandler(e.h.f.runtime, httpapi.Options{Inbox: e.svc, ManualOrders: m})
	req := httptest.NewRequest("POST", "/v1/admin/stores/"+e.h.f.storeA1+path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+e.h.token)
	req.Header.Set("Idempotency-Key", lbKey())
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func qlLinkCount(t *testing.T, e *lbEnv, order string) int {
	t.Helper()
	var n int
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM checkout.order_links WHERE order_id=$1`, order).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func qlAssertLink(t *testing.T, e *lbEnv, order, link string) {
	t.Helper()
	hash := sha256.Sum256([]byte(crTokenOf(t, link)))
	var live bool
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT order_id=$2::uuid AND redeemed_at IS NULL AND expires_at>clock_timestamp() FROM checkout.order_links WHERE token_hash=$1`, hash[:], order).Scan(&live); err != nil || !live {
		t.Fatalf("sealed order link invalid: live=%v err=%v", live, err)
	}
}
func qlPlan(t *testing.T, e *lbEnv, order, conv, link string) inbox.SendOutput {
	t.Helper()
	var out inbox.SendOutput
	if err := e.scoped(t, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = e.svc.PlanOrderPayLink(ctx, tx, s, lbKey(), conv, order, link)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// Exact real operation/job/secret graph, held off the dispatcher.
	var jobs int
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT count(*) FROM integration.operations o JOIN river.river_job j ON j.id=o.job_id WHERE o.id=$1 AND o.state='READY' AND o.request->>'order_id'=$2`, out.OperationID, order).Scan(&jobs); err != nil || jobs != 1 || e.secretCount(t, out.OperationID) != 1 {
		t.Fatalf("planned graph jobs=%d err=%v", jobs, err)
	}
	return out
}
func qlSetup(t *testing.T) (*lbEnv, *merchanttools.ManualOrders, crBuyer, string, string) {
	t.Helper()
	e := lbSetup(t)
	e.crCleanup(t)
	b := e.crBuyer(t)
	order := e.crOrder(t, b.bundle, "AWAITING_PAYMENT", "merchant_manual", false)
	// Give the disclosed synthetic order a real active owner so its sealed URL must redeem through the actual issuer.
	mustExec(t, e.h.f.owner, `INSERT INTO buyer.owners(tenant_id,store_id,id) SELECT tenant_id,store_id,owner_id FROM checkout.orders WHERE id=$1`, order)
	m := e.crManual(t)
	out, _, err := m.RegenerateLink(context.Background(), e.h.token, e.h.f.storeA1, lbKey(), merchanttools.ManualRegenerateInput{OrderID: order, Locale: "zh-TW"})
	if err != nil {
		t.Fatal(err)
	}
	return e, m, b, order, out.BuyerLink
}
func TestQueuedOrderLinkHTTPRegeneration(t *testing.T) {
	e, m, b, order, link := qlSetup(t)
	op := qlPlan(t, e, order, b.conv, link)
	before := qlLinkCount(t, e, order)
	code, out := qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, order))
	if code != 409 || out["code"] != "conflict" {
		t.Errorf("queued regeneration: %d %v", code, out)
	}
	if qlLinkCount(t, e, order) != before || e.secretCount(t, op.OperationID) != 1 {
		t.Error("refused regeneration changed links or sealed secret")
	}
	qlAssertLink(t, e, order, link)
	// Completion releases the guard; the original queued URL is exactly what dispatch sends.
	e.run(t, op.OperationID)
	e.awaitOp(t, op.OperationID, "SUCCEEDED", 10*time.Second, "completed")
	if got := e.crSentLink(t, 0); got != link {
		t.Fatal("dispatch URL differs from planned URL")
	}
	qlRedeem(t, e, order, link)
	code, out = qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, order))
	if code != 201 {
		t.Fatalf("completed regeneration: %d %v", code, out)
	}
	if qlLinkCount(t, e, order) != before+1 {
		t.Fatal("completed regeneration must issue exactly one link")
	}
}
func TestQueuedOrderLinkHTTPReminder(t *testing.T) {
	e, m, b, order, link := qlSetup(t)
	op := qlPlan(t, e, order, b.conv, link)
	before := qlLinkCount(t, e, order)
	// Human planning takes over; restore auto only to make the reminder's dynamic candidate eligible.
	mustExec(t, e.h.f.owner, `UPDATE inbox.conversation_state SET mode='auto',assignee_principal=NULL,human_until=NULL WHERE conversation_id=$1`, b.conv)
	code, out := qlHTTP(e, m, "/live-sessions/"+e.session+"/reminders/"+b.bundle, "")
	e.crPark(t)
	if code != 200 || out["refused"] != float64(1) || out["queued"] != float64(0) {
		t.Fatalf("queued reminder: %d %v", code, out)
	}
	rows := out["results"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["code"] != "link_unavailable" {
		t.Fatalf("refusal code: %v", rows)
	}
	if qlLinkCount(t, e, order) != before || len(e.crOps(t)) != 0 || e.secretCount(t, op.OperationID) != 1 {
		t.Fatal("refused reminder created a link, operation, or secret")
	}
	qlAssertLink(t, e, order, link)
}
func TestQueuedOrderLinkPlanningSerializesHTTP(t *testing.T) {
	e, m, b, order, link := qlSetup(t)
	f := e.h.f
	ctx := context.Background()
	gate := make(chan struct{})
	planned := make(chan inbox.SendOutput, 1)
	plannedDone := make(chan error, 1)
	go func() {
		plannedDone <- platform.WithScope(ctx, f.runtime, e.h.token, f.storeA1, "inbox:reply", func(tx pgx.Tx, scope platform.Scope) error {
			op, err := e.svc.PlanOrderPayLink(ctx, tx, scope, lbKey(), b.conv, order, link)
			planned <- op
			<-gate
			return err
		})
	}()
	op := <-planned
	defer func() {
		select {
		case <-gate:
		default:
			close(gate)
		}
	}()
	type result struct {
		code int
		body map[string]any
	}
	done := make(chan result, 1)
	go func() {
		c, b := qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, order))
		done <- result{c, b}
	}()
	select {
	case r := <-done:
		done <- r
		t.Errorf("regeneration escaped uncommitted planner: %d %v", r.code, r.body)
	case <-time.After(150 * time.Millisecond):
	}
	close(gate)
	if err := <-plannedDone; err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		if r.code != 409 || r.body["code"] != "conflict" {
			t.Fatalf("serialized refusal: %d %v", r.code, r.body)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("regeneration did not finish")
	}
	if e.secretCount(t, op.OperationID) != 1 {
		t.Fatal("missing planned secret")
	}
	qlAssertLink(t, e, order, link)
}

func TestQueuedOrderLinkReminderSealHTTP(t *testing.T) {
	e, m, b, order, _ := qlSetup(t)
	code, out := qlHTTP(e, m, "/live-sessions/"+e.session+"/reminders/"+b.bundle, "")
	e.crPark(t)
	if code != 200 || out["queued"] != float64(1) {
		t.Fatalf("plan reminder: %d %v", code, out)
	}
	ops := e.crOps(t)
	if len(ops) != 1 || e.secretCount(t, ops[0]) != 1 {
		t.Fatal("missing real reminder operation/secret")
	}
	var before []byte
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT token_hash FROM checkout.order_links WHERE order_id=$1 AND redeemed_at IS NULL`, order).Scan(&before); err != nil {
		t.Fatal(err)
	}
	n := qlLinkCount(t, e, order)
	code, out = qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, order))
	if code != 409 || out["code"] != "conflict" {
		t.Errorf("queued reminder regeneration: %d %v", code, out)
	}
	var still bool
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT redeemed_at IS NULL FROM checkout.order_links WHERE token_hash=$1`, before).Scan(&still); err != nil || !still || qlLinkCount(t, e, order) != n {
		t.Fatal("queued reminder seal invalidated")
	}
	e.run(t, ops[0])
	e.awaitOp(t, ops[0], "SUCCEEDED", 10*time.Second, "completed")
	link := e.crSentLink(t, 0)
	sealedHash := sha256.Sum256([]byte(crTokenOf(t, link)))
	if string(before) != string(sealedHash[:]) {
		t.Fatal("planned reminder URL identity changed")
	}
	qlAssertLink(t, e, order, link)
	qlRedeem(t, e, order, link)
	code, out = qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, order))
	if code != 201 || qlLinkCount(t, e, order) != n+1 {
		t.Fatalf("completed reminder regeneration: %d %v", code, out)
	}
}

// qlPausingPlanner is an explicit planner seam for the ForBuyer transaction-gap gate.
// The separate queued-link gates above prove real operation/job/seal behavior; this gate pins the HTTP order issuance boundary.
type qlPausingPlanner struct {
	delegate *lbuPlanner
	ready    chan string
	resume   chan struct{}
}

func (p *qlPausingPlanner) PlanOrderPayLink(ctx context.Context, tx pgx.Tx, s platform.Scope, key, conv, order, link string) (inbox.SendOutput, error) {
	if link != "" {
		p.ready <- order
		<-p.resume
	}
	return p.delegate.PlanOrderPayLink(ctx, tx, s, key, conv, order, link)
}
func TestQueuedOrderLinkForBuyerAtomicHTTP(t *testing.T) {
	e := lbuNew(t)
	_, _, bundle, conv := e.scenario()
	planner := &qlPausingPlanner{delegate: e.plan, ready: make(chan string, 1), resume: make(chan struct{})}
	fb, err := merchanttools.NewForBuyer(e.manual, planner)
	if err != nil {
		t.Fatal(err)
	}
	e.handler = httpapi.NewHandler(e.p.f.runtime, httpapi.Options{CVS: e.cvs, ManualOrders: e.manual, ForBuyer: fb})
	type result struct {
		code int
		out  map[string]any
	}
	done := make(chan result, 1)
	key := lbKey()
	body := e.body(1, []string{bundle}, conv, true)
	go func() { c, b := e.post(e.token(), key, body); done <- result{c, b} }()
	var order string
	select {
	case order = <-planner.ready:
	case r := <-done:
		t.Fatalf("placement refused before planning: %d %v", r.code, r.out)
	case <-time.After(20 * time.Second):
		t.Fatal("planner barrier timeout")
	}
	defer func() {
		select {
		case <-planner.resume:
		default:
			close(planner.resume)
		}
	}()
	regen := make(chan result, 1)
	go func() {
		raw, _ := json.Marshal(map[string]string{"order_id": order, "locale": "zh-TW"})
		w := e.admin(e.token()).call("POST", "/orders/manual/regenerate-link", lbKey(), "application/json", raw)
		var b map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &b)
		regen <- result{w.Code, b}
	}()
	escaped := false
	select {
	case r := <-regen:
		regen <- r
		escaped = true
		t.Errorf("regen escaped issue/plan transaction: %d %v", r.code, r.out)
	case <-time.After(150 * time.Millisecond):
	}
	close(planner.resume)
	r := <-done
	if r.code != 201 {
		t.Fatalf("placement: %d %v", r.code, r.out)
	}
	// The seam does not persist integration.operations; after commit regeneration is allowed. Before commit it must wait.
	rr := <-regen
	if rr.code != 201 {
		t.Fatalf("post-commit regeneration: %d %v", rr.code, rr.out)
	}
	if escaped {
		return
	}
	if got := e.plan.links[0]; got != r.out["buyer_link"].(string) {
		t.Fatal("HTTP returned URL differs from planner URL")
	}
	if n := e.count(`SELECT count(*) FROM checkout.order_links WHERE order_id=$1`, order); n != 3 {
		t.Fatalf("exact original + atomic reissue + postcommit regeneration link count: %d", n)
	}
	replayCode, replayed := e.post(e.token(), key, body)
	if replayCode != 200 || replayed["send"].(map[string]any)["operation_id"] != r.out["send"].(map[string]any)["operation_id"] || e.plan.planned() != 1 {
		t.Fatalf("explicit replay: %d %v", replayCode, replayed)
	}
}

func TestQueuedOrderLinkUnknownHTTP(t *testing.T) {
	e, m, b, order, link := qlSetup(t)
	op := qlPlan(t, e, order, b.conv, link)
	before := qlLinkCount(t, e, order)
	// Owner-only unknown/acknowledged fixtures; no send/replay follows an UNKNOWN result.
	for _, state := range []string{"UNKNOWN", "ACKNOWLEDGED"} {
		mustExec(t, e.h.f.owner, `UPDATE integration.operations SET state=$2,generation=1 WHERE id=$1`, op.OperationID, state)
		c, b := qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, order))
		if c != 409 || b["code"] != "conflict" || qlLinkCount(t, e, order) != before {
			t.Fatalf("%s regeneration: %d %v", state, c, b)
		}
		qlAssertLink(t, e, order, link)
	}
}

// qlRedeem exercises the real buyer-issuer definer after a refused regeneration; a live hash alone is insufficient proof.
func qlRedeem(t *testing.T, e *lbEnv, order, link string) {
	t.Helper()
	ctx := context.Background()
	tx, err := e.h.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_buyer_issuer`); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(crTokenOf(t, link)))
	capHash := sha256.Sum256([]byte(randomToken()))
	ip := sha256.Sum256([]byte(randomToken()))
	var got string
	if err := tx.QueryRow(ctx, `SELECT order_id::text FROM checkout.redeem_order_link($1::uuid,$2::uuid,$3,$4,3600,$5)`, e.h.f.storeA1, order, hash[:], capHash[:], ip[:]).Scan(&got); err != nil || got != order {
		t.Fatalf("sealed link redemption: order=%s err=%v", got, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
