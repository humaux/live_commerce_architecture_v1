// Purpose: reject a reminder whose rechecked order differs from the real order link issued after the candidate scan.
// Depends on: disposable PG, qlSetup/crOrder fixtures, real CheckoutReminders, inbox planner, River and sealed dispatch copy.
// Used by: PR32 focused REAL_PG regression acceptance; synthetic order B is disclosed, Meta LIVE is NOT_RUN.
// Invariants: I01/I02/I06/I11; candidate identity and bearer target agree; refusal rolls back link rotation, secret and job.
// Transaction seams return errors; never call testing.Fatal/Goexit inside WithScope, so failures release the transaction.
package foundation_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// qltAfterIssue changes only the disclosed fixture between the real link issuer and the real planner.
// Every scan and plan still delegates to inbox.Service; no operation, secret or job is synthesized here.
type qltAfterIssue struct {
	*inbox.Service
	target string
	before func(context.Context, inbox.ReminderCandidate, string) error
}

func (p *qltAfterIssue) PlanCheckoutReminder(ctx context.Context, tx pgx.Tx, scope platform.Scope, session string, c inbox.ReminderCandidate, link string) error {
	if p.before != nil && c.BundleID == p.target {
		f := p.before
		p.before = nil
		if err := f(ctx, c, link); err != nil {
			return err
		}
	}
	return p.Service.PlanCheckoutReminder(ctx, tx, scope, session, c, link)
}

// qltNewerOrder is the disclosed crOrder fixture with an error-only API for an in-transaction seam.
// Its owner-pool acquisition, statements and commit share the caller's deadline; no testing helper can invoke Goexit.
func qltNewerOrder(ctx context.Context, e *lbEnv, bundle, orderA string) (string, error) {
	f := e.h.f
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin concurrent synthetic order: %w", err)
	}
	defer tx.Rollback(ctx)
	// As with crOrder: omit the irrelevant commerce FK graph, retaining CHECKs, real origins, active owner and link issuer.
	if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
		return "", fmt.Errorf("prepare concurrent synthetic order: %w", err)
	}
	order := randomUUID()
	if _, err := tx.Exec(ctx, `INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,quote_id,destination_id,market_id,
		country,service_code,service_version,allocation_version,currency,total_minor,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at,source)
		VALUES($1,$2,gen_random_uuid(),$3,gen_random_uuid(),gen_random_uuid(),1,gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),
		'TW','std',1,1,'TWD',1000,'AWAITING_PAYMENT','MANUAL_UNASSIGNED',1,
		(SELECT expires_at+interval '1 minute' FROM checkout.orders WHERE id=$4),
		(random()*9e12)::bigint+1000000000000,'{"cr":"1"}'::jsonb,now()-interval '5 minutes',now()-interval '5 minutes','merchant_manual')`,
		f.tenantA, f.storeA1, order, orderA); err != nil {
		return "", fmt.Errorf("insert concurrent synthetic order: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO buyer.owners(tenant_id,store_id,id) SELECT tenant_id,store_id,owner_id FROM checkout.orders WHERE id=$1`, order); err != nil {
		return "", fmt.Errorf("insert concurrent synthetic owner: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO claims.order_origins(tenant_id,store_id,order_id,bundle_id,offer_id,line_version,session_id,occurred_at)
		VALUES($1,$2,$3,$4,gen_random_uuid(),1,$5,clock_timestamp())`, f.tenantA, f.storeA1, order, bundle, e.session); err != nil {
		return "", fmt.Errorf("insert concurrent synthetic origin: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit concurrent synthetic order: %w", err)
	}
	return order, nil
}

func TestQueuedOrderLinkReminderCandidateIdentity(t *testing.T) {
	e, m, b, orderA, original := qlSetup(t)
	ctx := context.Background()
	f := e.h.f
	other := e.crBuyer(t)
	orderC := e.crOrder(t, other.bundle, "AWAITING_PAYMENT", "merchant_manual", false)
	mustExec(t, f.owner, `INSERT INTO buyer.owners(tenant_id,store_id,id) SELECT tenant_id,store_id,owner_id FROM checkout.orders WHERE id=$1`, orderC)
	// Own the seal only so the actual planned reminder can be inspected without dispatching a buyer message.
	sealKeys, open := mcnPageRing(t)
	e.svc.EnableSend(sealKeys, e.jobs, msgtemplates.NewService())
	originalHash := sha256.Sum256([]byte(crTokenOf(t, original)))
	linksBefore := qlLinkCount(t, e, orderA)
	secretsBefore := miCount(t, f.owner, `SELECT count(*) FROM inbox.send_secrets WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)
	// Count unjoined jobs too: an orphaned preparation must not escape the rollback assertion.
	jobsBefore := miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1'`)
	var orderB string
	callbacks := 0
	planner := &qltAfterIssue{Service: e.svc, target: b.bundle}
	planner.before = func(callCtx context.Context, c inbox.ReminderCandidate, issued string) error {
		callCtx, cancel := context.WithTimeout(callCtx, 10*time.Second)
		defer cancel()
		callbacks++
		if c.BundleID != b.bundle || c.OrderID != orderA || c.State != inbox.StateAwaitingPayment || !strings.Contains(issued, "/order-link#o="+orderA+"&t=") {
			return fmt.Errorf("interposition must occur after issuing the real scanned order A link")
		}
		parsed, err := url.Parse(issued)
		if err != nil {
			return fmt.Errorf("issued A link has an invalid URL") // never include the bearer-containing parser error
		}
		fragment, err := url.ParseQuery(parsed.Fragment)
		if err != nil || fragment.Get("o") != orderA || fragment.Get("t") == "" {
			return fmt.Errorf("issued A link has an invalid target/token fragment")
		}
		issuedHash := sha256.Sum256([]byte(fragment.Get("t")))
		if issuedHash == originalHash {
			return fmt.Errorf("the real issuer must have attempted a new A bearer before planning")
		}
		// Disclosed concurrent synthetic checkout of the SAME bundle. The real fact query chooses the latest expiry,
		// so make B unambiguously newer; random UUID ordering and clock resolution must not decide this race.
		orderB, err = qltNewerOrder(callCtx, e, b.bundle, orderA)
		return err
	}
	r, err := merchanttools.NewCheckoutReminders(f.runtime, m, planner)
	if err != nil {
		t.Fatal(err)
	}
	// A batch scanned A and C. Rechecking the first buyer sees B although its URL targets A: roll back that buyer,
	// but do not abort the batch or hide the other buyer's successful real plan (regardless of candidate ordering).
	out, err := r.Trigger(ctx, e.h.token, f.storeA1, lbKey(), e.session, "")
	e.crPark(t)
	if err != nil || callbacks != 1 || orderB == "" || out.Queued != 1 || out.Refused != 1 || out.AlreadyReminded != 0 || out.Followup != 0 || out.Restricted != 0 || out.Truncated || len(out.Results) != 2 {
		t.Fatalf("changed candidate: callbacks=%d B_created=%v queued=%d refused=%d other=%d results=%d err=%v", callbacks, orderB != "", out.Queued, out.Refused, out.AlreadyReminded+out.Followup+out.Restricted, len(out.Results), err)
	}
	seen := make(map[string]bool)
	for _, result := range out.Results {
		if seen[result.BundleID] {
			t.Fatal("batch repeated one buyer and omitted the other")
		}
		seen[result.BundleID] = true
		switch result.BundleID {
		case b.bundle:
			if result.Outcome != "refused" || result.Code != "not_remindable" {
				t.Fatalf("changed candidate must refuse not_remindable: outcome=%s code=%s", result.Outcome, result.Code)
			}
		case other.bundle:
			if result.Outcome != "queued" || result.Code != "" {
				t.Fatalf("distinct buyer must still queue: outcome=%s code=%s", result.Outcome, result.Code)
			}
		default:
			t.Fatal("batch returned an unrelated buyer")
		}
	}
	if miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND request->>'session_id'=$3 AND request->>'bundle_id'=$4 AND request->>'message_type'='checkout_reminder'`, f.tenantA, f.storeA1, e.session, b.bundle) != 0 || miCount(t, f.owner, `SELECT count(*) FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND bundle_id=$4 AND outcome='queued'`, f.tenantA, f.storeA1, e.session, b.bundle) != 0 {
		t.Fatal("changed candidate committed a reminder operation or queued row")
	}
	if miCount(t, f.owner, `SELECT count(*) FROM inbox.send_secrets WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1) != secretsBefore+1 || miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1'`) != jobsBefore+1 {
		t.Fatal("changed candidate committed a sealed secret or River preparation job")
	}
	if qlLinkCount(t, e, orderA) != linksBefore || qlLinkCount(t, e, orderB) != 0 || qlLinkCount(t, e, orderC) != 1 {
		t.Fatal("changed candidate committed link rotation")
	}
	firstOps := e.crOps(t)
	if len(firstOps) != 1 || e.secretCount(t, firstOps[0]) != 1 || miCount(t, f.owner, `SELECT count(*) FROM integration.operations o JOIN river.river_job j ON j.id=o.job_id WHERE o.id=$1 AND o.state='READY' AND o.request->>'order_id'=$2 AND o.request->>'bundle_id'=$3 AND j.state='available' AND j.queue='cr_hold'`, firstOps[0], orderC, other.bundle) != 1 {
		t.Fatal("distinct buyer must keep its one real READY operation, seal and held River job")
	}
	qlAssertLink(t, e, orderA, original)
	if e.g.count() != 0 {
		t.Fatal("paused dispatcher called Graph during the rejected batch")
	}

	// A fresh ordinary pass must scan B, issue B and record B in the real plan; A must remain independently regenerable.
	ordinary, err := merchanttools.NewCheckoutReminders(f.runtime, m, e.svc)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := ordinary.Trigger(ctx, e.h.token, f.storeA1, lbKey(), e.session, b.bundle)
	e.crPark(t)
	if err != nil || queued.Queued != 1 || queued.Refused != 0 || queued.AlreadyReminded != 0 || queued.Followup != 0 || queued.Restricted != 0 || queued.Truncated || len(queued.Results) != 1 || queued.Results[0].BundleID != b.bundle || queued.Results[0].Outcome != "queued" {
		t.Fatalf("fresh B reminder: queued=%d refused=%d results=%d err=%v", queued.Queued, queued.Refused, len(queued.Results), err)
	}
	ops := e.crOps(t)
	if len(ops) != 2 {
		t.Fatal("fresh B reminder must have exactly one real operation and sealed secret")
	}
	var op string
	if err := f.owner.QueryRow(ctx, `SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND request->>'session_id'=$3 AND request->>'bundle_id'=$4 AND request->>'message_type'='checkout_reminder'`, f.tenantA, f.storeA1, e.session, b.bundle).Scan(&op); err != nil {
		t.Fatal(err)
	}
	if e.secretCount(t, op) != 1 || miCount(t, f.owner, `SELECT count(*) FROM integration.operations o JOIN river.river_job j ON j.id=o.job_id WHERE o.id=$1 AND o.state='READY' AND o.request->>'order_id'=$2 AND o.request->>'bundle_id'=$3 AND o.request->>'reminder_state'='awaiting_payment' AND j.state='available' AND j.queue='cr_hold'`, op, orderB, b.bundle) != 1 || miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1'`) != jobsBefore+2 {
		t.Fatal("fresh B reminder must record B and exactly one held READY job")
	}
	var enc, sealed []byte
	if err := f.owner.QueryRow(ctx, `SELECT enc,sealed FROM inbox.send_secrets WHERE operation_id=$1`, op).Scan(&enc, &sealed); err != nil {
		t.Fatal(err)
	}
	plain, err := open.OpenSend(f.tenantA, f.storeA1, op, enc, sealed)
	if err != nil {
		t.Fatal("open test-owned reminder seal", err)
	}
	var message struct {
		Text string `json:"text"`
	}
	err = json.Unmarshal(plain, &message)
	clear(plain)
	if err != nil {
		t.Fatal("decode real sealed reminder", err)
	}
	plannedLink := crLink.FindString(message.Text)
	message.Text = ""
	if !strings.Contains(plannedLink, "/order-link#o="+orderB+"&t=") {
		t.Fatal("fresh reminder sealed URL must target B, matching operation metadata")
	}
	qlAssertLink(t, e, orderB, plannedLink)
	qlAssertLink(t, e, orderA, original)
	if qlLinkCount(t, e, orderA) != linksBefore || qlLinkCount(t, e, orderB) != 1 {
		t.Fatal("fresh B reminder rotated a different order or issued more than one link")
	}
	code, refused := qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, orderB))
	if code != 409 || refused["code"] != "conflict" || qlLinkCount(t, e, orderB) != 1 {
		t.Fatalf("B guard: status=%d code=%v", code, refused["code"])
	}
	code, regenerated := qlHTTP(e, m, "/orders/manual/regenerate-link", fmt.Sprintf(`{"order_id":%q,"locale":"zh-TW"}`, orderA))
	if code != 201 || qlLinkCount(t, e, orderA) != linksBefore+1 {
		t.Fatalf("B guard must allow unrelated A regeneration: status=%d", code)
	}
	currentA, ok := regenerated["buyer_link"].(string)
	if !ok || currentA == "" {
		t.Fatal("successful A regeneration must issue its real current link")
	}
	qlAssertLink(t, e, orderA, currentA)
	qlAssertLink(t, e, orderB, plannedLink)
	if len(e.crOps(t)) != 2 || e.secretCount(t, op) != 1 || e.secretCount(t, firstOps[0]) != 1 || miCount(t, f.owner, `SELECT count(*) FROM inbox.send_secrets WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1) != secretsBefore+2 || miCount(t, f.owner, `SELECT count(*) FROM river.river_job WHERE kind='external_operation_v1'`) != jobsBefore+2 || e.g.count() != 0 {
		t.Fatal("guard checks altered the queued reminder graph or dispatched a buyer message")
	}
}
