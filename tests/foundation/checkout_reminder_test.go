package foundation_test

// Real-PG gates of W3-03B (checkout reminders; migration 0131; brief docs/delivery/units/w3-03b-checkout-reminder.md + the integrator's P1/P2 review
// rulings): CR01..CR08, the batch livelock gate (CR09), the link-completes-the-purchase gates (CR10 claim link, CR11 order link), the per-buyer outcome /
// rate-cap / young-claim / Check re-check gates and the exact ACL of the new definers/tables. Harness: live_console_send_test.go (lbSetup: the REAL
// inbox.Service planners on commerce_runtime, the REAL metareply send routes on a REAL core dispatcher over a loopback fake Graph) plus the REAL
// merchanttools.CheckoutReminders pass. Evidence label: MOCK. Every id, name and text is a synthetic sentinel; no Meta token, no non-loopback network.
// Meta LIVE = NOT_RUN. Owner-pool writes are disclosed fixtures (backdated claims, synthetic orders, peers); no product path writes those rows here.

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/inbox"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

// crBuyer is one synthetic buyer: a claimed bundle (comment A1), a Messenger conversation and the bundle<->peer link a successful private
// reply would have written (inbox.finish_send), so the bundle's thread is known. The claim is backdated 20 minutes (a buyer still in the show is not nagged).
type crBuyer struct{ psid, bundle, conv string }

func (e *lbEnv) crBuyer(t *testing.T) crBuyer {
	t.Helper()
	b := crBuyer{psid: mciDigits(15)}
	b.conv = e.postDM(t, b.psid, "hello", time.Now())
	r := e.planReply(t, false, b.psid, "A1")
	if r.bundleID == "" {
		t.Fatal("claim did not create a bundle")
	}
	b.bundle = r.bundleID
	e.linkPeer(t, b.bundle, b.conv, randomUUID())
	mustExec(t, e.h.f.owner, `UPDATE claims.bundles SET created_at=clock_timestamp()-interval '20 minutes' WHERE id=$1`, b.bundle)
	return b
}

// crManual is the REAL ManualOrders for the order-link path. Only its link derivation and regenerate call are used, so the buyer pipeline pieces are
// non-nil placeholders (NewManualOrders only requires them to exist).
func (e *lbEnv) crManual(t *testing.T) *merchanttools.ManualOrders {
	t.Helper()
	f := e.h.f
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'catalog:read'),($1,$2,$3,'inventory:reserve') ON CONFLICT DO NOTHING`,
		f.tenantA, f.storeA1, e.h.actor)
	m, err := merchanttools.NewManualOrders(f.runtime, &buyer.Service{}, f.runtime, &checkout.Service{}, randomBytes(32))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// crTriggerAs runs one reminder pass as the merchant of token (batch when bundle == ""); in lets a test interpose between the scan and the plans.
func (e *lbEnv) crTriggerAs(t *testing.T, token string, in merchanttools.ReminderInbox, bundle string) (merchanttools.ReminderResult, error) {
	t.Helper()
	r, err := merchanttools.NewCheckoutReminders(e.h.f.runtime, e.crManual(t), orDefault(in, e.svc))
	if err != nil {
		t.Fatal(err)
	}
	out, terr := r.Trigger(context.Background(), token, e.h.f.storeA1, lbKey(), e.session, bundle)
	e.crPark(t)
	return out, terr
}

func orDefault(in merchanttools.ReminderInbox, d *inbox.Service) merchanttools.ReminderInbox {
	if in == nil {
		return d
	}
	return in
}

func (e *lbEnv) crTrigger(t *testing.T) (merchanttools.ReminderResult, error) {
	t.Helper()
	return e.crTriggerAs(t, e.h.token, nil, "")
}

// crPark moves the new jobs onto a holding queue in one statement: a stray default-queue worker of this binary must not dispatch an operation the
// test means to hold back (e.run moves it from the holding queue to this harness's dispatcher).
func (e *lbEnv) crPark(t *testing.T) {
	t.Helper()
	mustExec(t, e.h.f.owner, `UPDATE river.river_job SET queue='cr_hold' WHERE kind='external_operation_v1' AND state='available' AND queue='default'
		AND args->>'operation_id' IN (SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND request->>'message_type'='checkout_reminder')`,
		e.h.f.tenantA, e.h.f.storeA1)
}

func (e *lbEnv) crReport(t *testing.T) inbox.ReminderReport {
	t.Helper()
	var out inbox.ReminderReport
	if err := e.scoped(t, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = e.svc.ReminderReport(ctx, tx, e.session)
		return err
	}); err != nil {
		t.Fatalf("reminder report: %v", err)
	}
	return out
}

// crOps returns the checkout-reminder DM operations of this test's session (the store is shared across tests), oldest first.
func (e *lbEnv) crOps(t *testing.T) []string {
	t.Helper()
	rows, err := e.h.f.owner.Query(context.Background(), `SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2
		AND action='meta.dm_send' AND request->>'message_type'='checkout_reminder' AND request->>'session_id'=$3 ORDER BY created_at`, e.h.f.tenantA, e.h.f.storeA1, e.session)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func (e *lbEnv) crCleanup(t *testing.T) {
	t.Helper()
	f := e.h.f
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)
		mustExec(t, f.owner, `DELETE FROM checkout.order_links WHERE tenant_id=$1 AND store_id=$2 AND order_id IN (SELECT id FROM checkout.orders WHERE snapshot->>'cr'='1')`, f.tenantA, f.storeA1)
		mustExec(t, f.owner, `DELETE FROM claims.order_origins WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, e.session)
		mustExec(t, f.owner, `DELETE FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND snapshot->>'cr'='1'`, f.tenantA, f.storeA1)
	})
}

// crOrder inserts one synthetic order of the bundle (disclosed fixture: the checkout pipeline needs a full buyer/cart/quote graph that this gate does
// not exercise; the reminder only reads commercial_state, expires_at and source through claims.order_origins) and returns its id.
func (e *lbEnv) crOrder(t *testing.T, bundle, state, source string, expired bool) string {
	t.Helper()
	f := e.h.f
	ctx := context.Background()
	tx, err := f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Foreign keys point at buyer/cart/quote/fulfilment rows that are irrelevant here (replica role skips only the FK triggers; CHECKs stay).
	if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
		t.Fatal(err)
	}
	order := randomUUID()
	created, expires := "now() - interval '5 minutes'", "now() + interval '10 minutes'"
	if expired {
		created, expires = "now() - interval '30 minutes'", "now() - interval '20 minutes'"
	}
	fulfil := "MANUAL_UNASSIGNED"
	if state == "CANCELLED" {
		fulfil = "CANCELLED"
	}
	if _, err := tx.Exec(ctx, `INSERT INTO checkout.orders(tenant_id,store_id,owner_id,id,creator_session_id,cart_id,cart_version,quote_id,destination_id,market_id,
		country,service_code,service_version,allocation_version,currency,total_minor,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at,source)
		VALUES($1,$2,gen_random_uuid(),$3,gen_random_uuid(),gen_random_uuid(),1,gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),
		'TW','std',1,1,'TWD',1000,$4,$5,1,`+expires+`,(random()*9e12)::bigint+1000000000000,'{"cr":"1"}'::jsonb,`+created+`,`+created+`,$6)`,
		f.tenantA, f.storeA1, order, state, fulfil, source); err != nil {
		t.Fatalf("synthetic order: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO claims.order_origins(tenant_id,store_id,order_id,bundle_id,offer_id,line_version,session_id,occurred_at)
		VALUES($1,$2,$3,$4,gen_random_uuid(),1,$5,clock_timestamp())`, f.tenantA, f.storeA1, order, bundle, e.session); err != nil {
		t.Fatalf("synthetic order origin: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return order
}

var crLink = regexp.MustCompile(`https://[^ ]+/(claim#t=|order-link#o=)[^ ]+`)

// crSentLink returns the link text of the n-th Graph POST (the bearer exists nowhere else in clear).
func (e *lbEnv) crSentLink(t *testing.T, n int) string {
	t.Helper()
	reqs := e.g.all()
	if len(reqs) <= n {
		t.Fatalf("graph requests=%d, want > %d", len(reqs), n)
	}
	msg, _ := reqs[n].body["message"].(map[string]any)
	text, _ := msg["text"].(string)
	link := crLink.FindString(text)
	if link == "" {
		t.Fatalf("no claim/order link in the DM text: %q", text)
	}
	return link
}

func crTokenOf(t *testing.T, link string) string {
	t.Helper()
	i := strings.LastIndex(link, "t=")
	if i < 0 {
		t.Fatalf("no token in %q", link)
	}
	tok, err := url.PathUnescape(link[i+2:])
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// CR01 + CR02 + the followup -> queued transition + CR10 (the claim link redeems to the bundle): only the buyer inside the window is messaged
// (RESPONSE, no tag), the closed-window and the human-takeover buyers become follow-ups, a second trigger never reminds twice.
func TestCheckoutReminderCR01WindowTakeoverAndOnce(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	open, closed, human := e.crBuyer(t), e.crBuyer(t), e.crBuyer(t)
	e.setInbound(t, closed.conv, time.Now().Add(-30*time.Hour))
	e.setHuman(t, human.conv, 1)
	var genBefore int64
	if err := f.owner.QueryRow(context.Background(), `SELECT generation FROM claims.links WHERE bundle_id=$1`, open.bundle).Scan(&genBefore); err != nil {
		t.Fatal(err)
	}

	out, err := e.crTrigger(t)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if out.Queued != 1 || out.Followup != 2 || out.AlreadyReminded != 0 || out.Refused != 0 || out.Truncated || len(out.Results) != 3 {
		t.Fatalf("first trigger: %+v", out)
	}
	ops := e.crOps(t)
	if len(ops) != 1 {
		t.Fatalf("meta.dm_send operations=%d, want 1", len(ops))
	}
	var request string
	if err := f.owner.QueryRow(context.Background(), `SELECT request::text FROM integration.operations WHERE id=$1`, ops[0]).Scan(&request); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"origin": "auto"`, `"template_id": "checkout-reminder/v1"`, `"conversation_id": "` + open.conv + `"`, `"bundle_id": "` + open.bundle + `"`, `"reminder_state": "claimed"`} {
		if !strings.Contains(request, want) {
			t.Fatalf("frozen request lacks %s: %s", want, request)
		}
	}
	if strings.Contains(request, open.psid) || strings.Contains(request, "http") {
		t.Fatalf("frozen request leaks the PSID or a link: %s", request)
	}
	// An auto send never takes the conversation over (plan_dm's implicit takeover is the human variant).
	if s := e.state(t, open.conv); s.Mode != "auto" || s.Gen != 0 {
		t.Fatalf("auto reminder changed the conversation state: %+v", s)
	}
	rep := e.crReport(t)
	reasons := map[string]string{}
	for _, it := range rep.Followup {
		reasons[it.BundleID] = it.Reason
		if !it.LinkCopyAllowed {
			t.Fatalf("followup without a copyable link: %+v", it)
		}
	}
	if len(rep.Followup) != 2 || reasons[closed.bundle] != "window_closed" || reasons[human.bundle] != "human_takeover" || rep.Queued != 1 || rep.Link == nil {
		t.Fatalf("report: %+v reasons=%v", rep, reasons)
	}
	// One audit row per pass (also when nothing is sent, asserted in the follow-up-only test).
	if n := miCount(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inbox.checkout_reminder.triggered'`, f.tenantA, f.storeA1); n < 1 {
		t.Fatalf("triggered audit rows=%d", n)
	}

	// Dispatch: RESPONSE inside the window, the buyer's PSID, a FRESH claim link; no tag material.
	e.run(t, ops[0])
	e.awaitOp(t, ops[0], "SUCCEEDED", 10*time.Second, "completed")
	reqs := e.g.all()
	if len(reqs) != 1 {
		t.Fatalf("graph traffic: %+v", reqs)
	}
	body := reqs[0].body
	rcp, _ := body["recipient"].(map[string]any)
	if body["messaging_type"] != "RESPONSE" || rcp["id"] != open.psid {
		t.Fatalf("reminder body: %v", body)
	}
	for _, banned := range []string{"tag", "message_tag", "update", "HUMAN_AGENT", "ACCOUNT_UPDATE"} {
		if strings.Contains(reqs[0].raw, banned) {
			t.Fatalf("message tag material %q in the request: %s", banned, reqs[0].raw)
		}
	}
	// CR10: the claim link in the DM IS the bundle's current link (hash match, generation +1, 72 h), so it opens the bundle that holds the claimed lines.
	link := e.crSentLink(t, 0)
	if !strings.HasPrefix(link, e.origin+"/zh-TW/claim#t=") {
		t.Fatalf("claim link shape: %s", link)
	}
	sum := sha256.Sum256([]byte(crTokenOf(t, link)))
	var bundle string
	var gen int64
	var ttl time.Duration
	if err := f.owner.QueryRow(context.Background(), `SELECT bundle_id::text, generation, expires_at-clock_timestamp() FROM claims.links WHERE token_hash=$1`, sum[:]).Scan(&bundle, &gen, &ttl); err != nil {
		t.Fatalf("the DM's claim link is not a stored link: %v", err)
	}
	if bundle != open.bundle || gen != genBefore+1 || ttl < 71*time.Hour {
		t.Fatalf("claim link: bundle=%s gen=%d (before %d) ttl=%s", bundle, gen, genBefore, ttl)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM claims.lines WHERE bundle_id=$1`, open.bundle); n < 1 {
		t.Fatal("the bundle behind the link holds no claim lines")
	}
	if e.secretCount(t, ops[0]) != 0 {
		t.Fatal("dispatch copy survived SUCCEEDED")
	}
	if rep := e.crReport(t); len(rep.Sent) != 1 || rep.Sent[0].BundleID != open.bundle || rep.Sent[0].SendState != "sent" || rep.Queued != 0 {
		t.Fatalf("report after dispatch: %+v", rep)
	}

	// CR02: the same session again: nothing new for the reminded buyer, and no second link is issued (generation unchanged).
	out, err = e.crTrigger(t)
	if err != nil || out.Queued != 0 || out.AlreadyReminded != 1 || out.Followup != 2 {
		t.Fatalf("second trigger: %+v err=%v", out, err)
	}
	if len(e.crOps(t)) != 1 || e.g.count() != 1 {
		t.Fatalf("second trigger planned or sent again: ops=%d graph=%d", len(e.crOps(t)), e.g.count())
	}
	var genAfter int64
	if err := f.owner.QueryRow(context.Background(), `SELECT generation FROM claims.links WHERE bundle_id=$1`, open.bundle).Scan(&genAfter); err != nil || genAfter != gen {
		t.Fatalf("a reminded buyer's link was rotated again: %d -> %d (%v)", gen, genAfter, err)
	}
	// The closed-window buyer writes again: the next trigger turns the follow-up into one queued reminder (and still none for the others).
	e.setInbound(t, closed.conv, time.Now())
	out, err = e.crTrigger(t)
	if err != nil || out.Queued != 1 || out.AlreadyReminded != 1 || out.Followup != 1 {
		t.Fatalf("third trigger: %+v err=%v", out, err)
	}
	if rep := e.crReport(t); len(rep.Followup) != 1 || rep.Followup[0].BundleID != human.bundle || rep.Queued != 1 {
		t.Fatalf("report after the buyer wrote again: %+v", rep)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3`, f.tenantA, f.storeA1, closed.bundle); n != 1 {
		t.Fatalf("follow-up row was duplicated instead of turned into the queued one: %d rows", n)
	}
}

// CR02 (concurrent): two merchants clicking at once plan exactly one reminder; the loser is a per-buyer refusal or already_reminded, never an error.
func TestCheckoutReminderCR02ConcurrentTriggers(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	e.crBuyer(t)
	type res struct {
		out merchanttools.ReminderResult
		err error
	}
	ch := make(chan res, 2)
	for i := 0; i < 2; i++ {
		go func() {
			out, err := e.crTrigger(t)
			ch <- res{out, err}
		}()
	}
	queued := 0
	for i := 0; i < 2; i++ {
		r := <-ch
		if r.err != nil {
			t.Fatalf("concurrent trigger: %v", r.err)
		}
		queued += r.out.Queued
	}
	if queued != 1 || len(e.crOps(t)) != 1 {
		t.Fatalf("queued=%d operations=%d, want exactly one reminder", queued, len(e.crOps(t)))
	}
}

// CR03: the window closes between planning and Check -> BLOCKED_POLICY window_closed, zero Graph calls, secret wiped, never retried; a takeover and a
// payment between planning and Check are denied too (human_takeover / not_remindable).
func TestCheckoutReminderCR03CheckReChecks(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	b := e.crBuyer(t)
	e.setInbound(t, b.conv, time.Now().Add(-(24*time.Hour - 6*time.Minute))) // inside the window by one minute
	out, err := e.crTrigger(t)
	if err != nil || out.Queued != 1 {
		t.Fatalf("trigger: %+v err=%v", out, err)
	}
	op := e.crOps(t)[0]
	before := e.g.count()
	e.setInbound(t, b.conv, time.Now().Add(-25*time.Hour))
	e.run(t, op)
	code, _ := e.awaitOp(t, op, "BLOCKED_POLICY", 10*time.Second, "completed")
	if code != "window_closed" || e.g.count() != before || e.secretCount(t, op) != 0 {
		t.Fatalf("code=%s graph %d->%d secrets=%d", code, before, e.g.count(), e.secretCount(t, op))
	}
	if rep := e.crReport(t); len(rep.Failed) != 1 || rep.Failed[0].SendState != "blocked" {
		t.Fatalf("report: %+v", rep)
	}
	b2 := e.crBuyer(t)
	if out, err := e.crTrigger(t); err != nil || out.Queued != 1 {
		t.Fatalf("second buyer trigger: %+v err=%v", out, err)
	}
	op2 := e.crOps(t)[1]
	e.setHuman(t, b2.conv, 1)
	before = e.g.count()
	e.run(t, op2)
	if code, _ := e.awaitOp(t, op2, "BLOCKED_POLICY", 10*time.Second, "completed"); code != "human_takeover" || e.g.count() != before {
		t.Fatalf("takeover before Check: code=%s graph %d->%d", code, before, e.g.count())
	}
	// P2-7: the buyer paid / ordered after the plan -> Check re-reads the bundle state and denies (zero HTTP).
	b3 := e.crBuyer(t)
	if out, err := e.crTrigger(t); err != nil || out.Queued != 1 {
		t.Fatalf("third buyer trigger: %+v err=%v", out, err)
	}
	op3 := e.crOps(t)[2]
	e.crOrder(t, b3.bundle, "CONFIRMED", "storefront", false)
	before = e.g.count()
	e.run(t, op3)
	if code, _ := e.awaitOp(t, op3, "BLOCKED_POLICY", 10*time.Second, "completed"); code != "not_remindable" || e.g.count() != before {
		t.Fatalf("paid before Check: code=%s graph %d->%d", code, before, e.g.count())
	}
}

// CR04: a Graph failure is UNKNOWN: one POST, never repeated; the report shows send_state=unknown.
func TestCheckoutReminderCR04GraphFailureIsUnknown(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	e.crBuyer(t)
	if out, err := e.crTrigger(t); err != nil || out.Queued != 1 {
		t.Fatalf("trigger: %+v err=%v", out, err)
	}
	op := e.crOps(t)[0]
	before := e.g.count()
	e.g.setMode("5xx")
	defer e.g.setMode("ok")
	e.run(t, op)
	e.awaitOp(t, op, "UNKNOWN", 10*time.Second)
	time.Sleep(1500 * time.Millisecond) // a blind retry would show up as a second POST inside this window
	if n := e.g.count() - before; n != 1 {
		t.Fatalf("%d Graph POSTs for one reminder (UNKNOWN must never be re-POSTed)", n)
	}
	if e.secretCount(t, op) != 0 {
		t.Fatal("dispatch copy survived UNKNOWN")
	}
	if rep := e.crReport(t); len(rep.Failed) != 1 || rep.Failed[0].SendState != "unknown" {
		t.Fatalf("report: %+v", rep)
	}
	if out, err := e.crTrigger(t); err != nil || out.Queued != 0 || out.AlreadyReminded != 1 {
		t.Fatalf("retrigger after UNKNOWN: %+v err=%v", out, err)
	}
}

// CR05 + CR11: the candidate set and the order link. Claimed-never-opened (claim link) and an unpaid merchant-created order (order link) are
// reminded; an unpaid storefront order and an opened-but-unordered claim are link_unavailable follow-ups; paid, cancelled and expired-unpaid orders
// are not touched; a bundle without a known thread is no_peer.
func TestCheckoutReminderCR05CandidateSetAndOrderLink(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	claimed, manualUnpaid, storefrontUnpaid, opened, paid, cancelled, expired := e.crBuyer(t), e.crBuyer(t), e.crBuyer(t), e.crBuyer(t), e.crBuyer(t), e.crBuyer(t), e.crBuyer(t)
	order := e.crOrder(t, manualUnpaid.bundle, "AWAITING_PAYMENT", "merchant_manual", false)
	e.crOrder(t, storefrontUnpaid.bundle, "AWAITING_PAYMENT", "storefront", false)
	e.crOrder(t, paid.bundle, "CONFIRMED", "merchant_manual", false)
	e.crOrder(t, cancelled.bundle, "CANCELLED", "merchant_manual", false)
	e.crOrder(t, expired.bundle, "AWAITING_PAYMENT", "merchant_manual", true)
	// "opened" = the claim link was redeemed: the bundle has an owner (disclosed fixture; FK skipped like the orders).
	mustExec(t, f.owner, `WITH o AS (INSERT INTO buyer.owners(tenant_id,store_id) VALUES($2,$3) RETURNING id)
		UPDATE claims.bundles SET owner_id=(SELECT id FROM o), bound_at=clock_timestamp() WHERE id=$1`, opened.bundle, f.tenantA, f.storeA1)
	// A stale order link of the merchant order: the re-issue must kill it.
	var tenantOwner string
	if err := f.owner.QueryRow(context.Background(), `SELECT owner_id::text FROM checkout.orders WHERE id=$1`, order).Scan(&tenantOwner); err != nil {
		t.Fatal(err)
	}
	oldHash := sha256.Sum256([]byte("old-link-" + order))
	mustExec(t, f.owner, `INSERT INTO checkout.order_links(token_hash,tenant_id,store_id,owner_id,order_id,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '6 days')`,
		oldHash[:], f.tenantA, f.storeA1, tenantOwner, order)

	out, err := e.crTrigger(t)
	if err != nil || out.Queued != 2 || out.Followup != 2 || out.AlreadyReminded != 0 || out.Refused != 0 {
		t.Fatalf("trigger: %+v err=%v", out, err)
	}
	got := map[string]string{}
	rows, err := f.owner.Query(context.Background(), `SELECT bundle_id::text, outcome||':'||coalesce(reason,reminder_state) FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var b, s string
		if err := rows.Scan(&b, &s); err != nil {
			t.Fatal(err)
		}
		got[b] = s
	}
	rows.Close()
	want := map[string]string{claimed.bundle: "queued:claimed", manualUnpaid.bundle: "queued:awaiting_payment",
		storefrontUnpaid.bundle: "followup:link_unavailable", opened.bundle: "followup:link_unavailable"}
	if len(got) != len(want) {
		t.Fatalf("reminder rows: %v", got)
	}
	for b, s := range want {
		if got[b] != s {
			t.Fatalf("bundle %s: %q, want %q (all %v)", b, got[b], s, got)
		}
	}
	// CR11: dispatch the unpaid merchant order's reminder: the DM carries the order-pay-link text and a NEW order link of THAT order.
	var opOrder string
	if err := f.owner.QueryRow(context.Background(), `SELECT operation_id::text FROM inbox.checkout_reminders WHERE bundle_id=$1`, manualUnpaid.bundle).Scan(&opOrder); err != nil {
		t.Fatal(err)
	}
	e.run(t, opOrder)
	e.awaitOp(t, opOrder, "SUCCEEDED", 10*time.Second, "completed")
	link := e.crSentLink(t, 0)
	msg, _ := e.g.all()[0].body["message"].(map[string]any)
	if !strings.Contains(msg["text"].(string), "完成付款") || !strings.HasPrefix(link, e.origin+"/zh-TW/order-link#o="+order+"&t=") {
		t.Fatalf("order-pay-link DM: %v", msg["text"])
	}
	sum := sha256.Sum256([]byte(crTokenOf(t, link)))
	var linkOrder string
	var redeemed *time.Time
	if err := f.owner.QueryRow(context.Background(), `SELECT order_id::text, redeemed_at FROM checkout.order_links WHERE token_hash=$1`, sum[:]).Scan(&linkOrder, &redeemed); err != nil || linkOrder != order || redeemed != nil {
		t.Fatalf("the DM's order link is not a live link of the unpaid order: %s %v (%v)", linkOrder, redeemed, err)
	}
	var oldDead bool
	if err := f.owner.QueryRow(context.Background(), `SELECT redeemed_at IS NOT NULL FROM checkout.order_links WHERE token_hash=$1`, oldHash[:]).Scan(&oldDead); err != nil || !oldDead {
		t.Fatalf("the previous order link stayed valid: dead=%v err=%v", oldDead, err)
	}
	// A bundle without a known thread is a no_peer follow-up (never a private-reply quota spend): drop the peer link of a fresh buyer.
	lone := e.crBuyer(t)
	mustExec(t, f.owner, `DELETE FROM inbox.bundle_peers WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3`, f.tenantA, f.storeA1, lone.bundle)
	out, err = e.crTrigger(t)
	if err != nil || out.Queued != 0 || out.Followup != 3 || out.AlreadyReminded != 2 {
		t.Fatalf("no_peer trigger: %+v err=%v", out, err)
	}
	if rep := e.crReport(t); len(rep.Followup) != 3 {
		t.Fatalf("report: %+v", rep)
	}
}

// P1-1: the batch never livelocks. Only buyers that would be SENT count against the limit, so with more than 100 sendable buyers a second pass
// reaches the rest (reminded and follow-up buyers cost nothing). Synthetic buyers are copies of one real buyer's rows (owner-pool fixture); the
// SQL scan is called directly because planning 101 DMs is the per-buyer path already covered above.
func TestCheckoutReminderCR09BatchDoesNotLivelock(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	base := e.crBuyer(t)
	ctx := context.Background()
	const extra = 104 // + the base buyer = 105 buyers; 3 of the extras are follow-up-only (window closed) and must not eat the limit: 102 sendable
	for i := 0; i < extra; i++ {
		mustExec(t, f.owner, `WITH peer AS (SELECT encode(sha256(gen_random_uuid()::text::bytea),'hex') AS pk, gen_random_uuid() AS cid, gen_random_uuid() AS bid),
		 conv AS (INSERT INTO social.conversations SELECT (jsonb_populate_record(NULL::social.conversations, to_jsonb(c)||jsonb_build_object('id',p.cid,'peer_key',p.pk))).*
		   FROM social.conversations c, peer p WHERE c.id=$1 RETURNING tenant_id,store_id,app_id,object,asset_id,peer_key),
		 st AS (INSERT INTO inbox.conversation_state SELECT (jsonb_populate_record(NULL::inbox.conversation_state, to_jsonb(s)||jsonb_build_object('conversation_id',p.cid,
		     'last_inbound_at', CASE WHEN $3::int < 3 THEN to_jsonb(now()-interval '30 hours') ELSE to_jsonb(now()) END))).*
		   FROM inbox.conversation_state s, peer p WHERE s.conversation_id=$1 RETURNING conversation_id),
		 bu AS (INSERT INTO claims.bundles SELECT (jsonb_populate_record(NULL::claims.bundles, to_jsonb(b)||jsonb_build_object('id',p.bid,'actor_key',encode(sha256(p.bid::text::bytea),'hex'),
		     'owner_id',NULL,'bound_at',NULL))).* FROM claims.bundles b, peer p WHERE b.id=$2 RETURNING id)
		INSERT INTO inbox.bundle_peers(tenant_id,store_id,bundle_id,peer_key,app_id,object,asset_id,operation_id)
		 SELECT cv.tenant_id,cv.store_id,p.bid,cv.peer_key,cv.app_id,cv.object,cv.asset_id,gen_random_uuid() FROM conv cv, peer p, st, bu`,
			base.conv, base.bundle, i)
	}
	scan := func() (sends, already, followups int, truncated bool) {
		if err := platform.WithScope(ctx, f.runtime, e.h.token, f.storeA1, "inbox:reply", func(tx pgx.Tx, _ platform.Scope) error {
			rows, err := tx.Query(ctx, `SELECT verdict, bundle_id::text FROM inbox.checkout_reminder_candidates($1::uuid,'manual',100,NULL)`, e.session)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var v string
				var b *string
				if err := rows.Scan(&v, &b); err != nil {
					return err
				}
				switch v {
				case "send":
					sends++
				case "already_reminded":
					already++
				case "truncated":
					truncated = true
				default:
					followups++
				}
			}
			return rows.Err()
		}); err != nil {
			t.Fatalf("scan: %v", err)
		}
		return
	}
	s1, a1, f1, tr1 := scan()
	if s1 != 100 || !tr1 || f1 > 3 || a1 != 0 { // follow-up buyers sorted after the 101st sendable one are reached by the next pass
		t.Logf("follow-up reasons: %v", lcStrings(t, f.owner, `SELECT reason||':'||count(*) FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 GROUP BY reason`, f.tenantA, f.storeA1, e.session))
		t.Fatalf("first pass: sends=%d already=%d followups=%d truncated=%v", s1, a1, f1, tr1)
	}
	// Mark the first 100 sendable buyers reminded exactly as the planner does (the queued row is what the scan reads).
	mustExec(t, f.owner, `INSERT INTO inbox.checkout_reminders(tenant_id,store_id,session_id,bundle_id,owner_id,peer_key,semantic_key,outcome,reminder_state,trigger,operation_id)
		SELECT b.tenant_id,b.store_id,b.session_id,b.id,b.owner_id,NULL,'crm:'||substr(encode(sha256(convert_to(b.session_id::text||'|'||b.id::text,'UTF8')),'hex'),1,48),
		       'queued','claimed','manual',gen_random_uuid()
		  FROM (SELECT x.* FROM claims.bundles x JOIN inbox.bundle_peers bp ON bp.bundle_id=x.id JOIN social.conversations cv ON cv.peer_key=bp.peer_key AND cv.store_id=bp.store_id
		         JOIN inbox.conversation_state s ON s.conversation_id=cv.id
		        WHERE x.session_id=$1 AND s.last_inbound_at>now()-interval '1 hour' ORDER BY x.created_at, x.id LIMIT 100) b`, e.session)
	s2, a2, f2, tr2 := scan()
	if s2 != 2 || tr2 || a2 != 100 { // 105 buyers - 3 follow-ups = 102 sendable: 100 + 2
		t.Fatalf("second pass: sends=%d already=%d followups=%d truncated=%v (the first 100 must not be re-counted)", s2, a2, f2, tr2)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3 AND outcome='followup'`, f.tenantA, f.storeA1, e.session); n != 3 {
		t.Fatalf("after two passes every buyer was handled: follow-up rows=%d, want 3 (+ 100 queued + 2 sendable)", n)
	}
}

// Young claims: a buyer still in the show (window OPEN, claim younger than 10 min) is not a candidate; a closed window or an old claim makes it one.
func TestCheckoutReminderYoungClaimsAreNotNagged(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	b := e.crBuyer(t)
	mustExec(t, f.owner, `UPDATE claims.bundles SET created_at=clock_timestamp()-interval '2 minutes' WHERE id=$1`, b.bundle)
	if out, err := e.crTriggerAs(t, e.h.token, nil, ""); err != nil || out.Queued != 0 || out.Followup != 0 || len(out.Results) != 0 {
		t.Fatalf("young claim, window open: %+v err=%v", out, err)
	}
	if _, err := e.crTriggerAs(t, e.h.token, nil, b.bundle); planCode(err) != "not_remindable" {
		t.Fatalf("single young claim: %v", err)
	}
	mustExec(t, f.owner, `UPDATE live.claim_windows SET state='CLOSED', generation=generation, opened_at=opened_at, closed_at=clock_timestamp() WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, e.session)
	if out, err := e.crTriggerAs(t, e.h.token, nil, ""); err != nil || out.Queued != 1 {
		t.Fatalf("young claim, window closed: %+v err=%v", out, err)
	}
}

// P2-5: a single buyer's race refuses only that buyer. Between the scan and the plans one buyer's window closes; the other buyer is still reminded.
type crRacy struct {
	*inbox.Service
	e      *lbEnv
	victim string
}

func (r crRacy) PlanCheckoutReminder(ctx context.Context, tx pgx.Tx, scope platform.Scope, session string, c inbox.ReminderCandidate, link string) error {
	if c.ConversationID == r.victim {
		mustExec(r.e.t, r.e.h.f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=clock_timestamp()-interval '30 hours' WHERE conversation_id=$1`, c.ConversationID)
	}
	return r.Service.PlanCheckoutReminder(ctx, tx, scope, session, c, link)
}

func TestCheckoutReminderPerBuyerRefusalDoesNotAbortTheBatch(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	victim, ok := e.crBuyer(t), e.crBuyer(t)
	var genBefore int64
	if err := f.owner.QueryRow(context.Background(), `SELECT generation FROM claims.links WHERE bundle_id=$1`, victim.bundle).Scan(&genBefore); err != nil {
		t.Fatal(err)
	}
	out, err := e.crTriggerAs(t, e.h.token, crRacy{Service: e.svc, e: e, victim: victim.conv}, "")
	if err != nil {
		t.Fatalf("a per-buyer refusal ended the pass: %v", err)
	}
	if out.Queued != 1 || out.Refused != 1 {
		t.Fatalf("outcome: %+v", out)
	}
	for _, r := range out.Results {
		if r.BundleID == victim.bundle && (r.Outcome != "refused" || r.Code != "window_closed") {
			t.Fatalf("victim outcome: %+v", r)
		}
		if r.BundleID == ok.bundle && r.Outcome != "queued" {
			t.Fatalf("other buyer outcome: %+v", r)
		}
	}
	// The refused buyer's link issue was rolled back with its transaction: generation unchanged, no operation, no reminder row.
	var genAfter int64
	if err := f.owner.QueryRow(context.Background(), `SELECT generation FROM claims.links WHERE bundle_id=$1`, victim.bundle).Scan(&genAfter); err != nil || genAfter != genBefore {
		t.Fatalf("refused buyer's link was rotated anyway: %d -> %d (%v)", genBefore, genAfter, err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM inbox.checkout_reminders WHERE bundle_id=$1 AND outcome='queued'`, victim.bundle); n != 0 {
		t.Fatalf("refused buyer has a queued row")
	}
	if len(e.crOps(t)) != 1 {
		t.Fatalf("operations=%d, want 1", len(e.crOps(t)))
	}
}

// A1.3 clause 4: reminders count toward the 60 sends / store / minute cap; the buyer over the cap is a per-buyer rate_limited refusal.
func TestCheckoutReminderCountsTowardTheSendCap(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	e.crBuyer(t)
	if out, err := e.crTrigger(t); err != nil || out.Queued != 1 {
		t.Fatalf("first reminder: %+v err=%v", out, err)
	}
	first := e.crOps(t)[0]
	t.Cleanup(func() {
		mustExec(t, f.owner, `DELETE FROM integration.operations WHERE semantic_key LIKE 'mdm:crcap%'`)
	})
	for i := 0; i < 59; i++ {
		mustExec(t, f.owner, `INSERT INTO integration.operations(tenant_id,store_id,id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id)
			SELECT tenant_id,store_id,gen_random_uuid(),principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,'mdm:crcap'||$2::text||$3::text,request_hash,request,job_id
			FROM integration.operations WHERE id=$1`, first, mciDigits(8), fmt.Sprint(i))
	}
	e.crBuyer(t)
	out, err := e.crTrigger(t)
	if err != nil || out.Queued != 0 || out.Refused != 1 || out.Results[0].Code != "rate_limited" {
		t.Fatalf("over the cap: %+v err=%v", out, err)
	}
}

// P2-13: a pass that only records follow-ups still audits itself; P2 permission: the scan needs inbox:reply AND live:manage (a claim link is re-issued).
func TestCheckoutReminderFollowupOnlyAuditAndPermissions(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	b := e.crBuyer(t)
	e.setInbound(t, b.conv, time.Now().Add(-30*time.Hour))
	before := miCount(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inbox.checkout_reminder.triggered'`, f.tenantA, f.storeA1)
	if out, err := e.crTrigger(t); err != nil || out.Queued != 0 || out.Followup != 1 {
		t.Fatalf("follow-up-only pass: %+v err=%v", out, err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='inbox.checkout_reminder.triggered'`, f.tenantA, f.storeA1); n != before+1 {
		t.Fatalf("audit rows %d -> %d, want +1", before, n)
	}
	_, tokReplyOnly := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:read", "inbox:reply")
	if _, err := e.crTriggerAs(t, tokReplyOnly, nil, ""); planCode(err) != "forbidden" {
		t.Fatalf("trigger without live:manage: %v", err)
	}
	_, tokManageOnly := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:read", "live:manage")
	if _, err := e.crTriggerAs(t, tokManageOnly, nil, ""); !errors.Is(err, platform.ErrForbidden) && planCode(err) != "forbidden" { // the scope gate (inbox:reply) or the definer
		t.Fatalf("trigger without inbox:reply: %v", err)
	}
}

// CR06: scope. Another store / tenant -> 404 on the foreign session; the report needs inbox:read.
func TestCheckoutReminderCR06Scope(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	e.crBuyer(t)
	_, tokReplyOnly := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:reply")
	_, tokA2 := lcPrincipal(t, f, f.tenantA, []string{f.storeA2}, "store:read", "inbox:read", "inbox:reply", "live:read", "live:manage")
	_, tokB := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "inbox:read", "inbox:reply", "live:read", "live:manage")
	for name, tc := range map[string]struct{ token, store string }{"sister store": {tokA2, f.storeA2}, "other tenant": {tokB, f.storeB}} {
		r, err := merchanttools.NewCheckoutReminders(f.runtime, nil, e.svc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.Trigger(context.Background(), tc.token, tc.store, lbKey(), e.session, ""); planCode(err) != "not_found" {
			t.Fatalf("%s trigger on a foreign session: %v", name, err)
		}
		err = platform.WithScope(context.Background(), f.runtime, tc.token, tc.store, "inbox:read", func(tx pgx.Tx, s platform.Scope) error {
			_, err := e.svc.ReminderReport(context.Background(), tx, e.session)
			return err
		})
		if planCode(err) != "not_found" {
			t.Fatalf("%s report on a foreign session: %v", name, err)
		}
	}
	if err := platform.WithScope(context.Background(), f.runtime, tokReplyOnly, f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.ReminderReport(context.Background(), tx, e.session)
		return err
	}); planCode(err) != "forbidden" {
		t.Fatalf("report without inbox:read: %v", err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1); n != 0 {
		t.Fatalf("refused triggers left %d reminder rows", n)
	}
}

// No store domain -> 409 no_storefront and nothing planned (a reminder without a link is useless).
func TestCheckoutReminderNoStorefront(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	e.crBuyer(t)
	mustExec(t, f.owner, `UPDATE control.storefront_domains SET state='SUSPENDED' WHERE tenant_id=$1 AND store_id=$2 AND state='ACTIVE'`, f.tenantA, f.storeA1)
	t.Cleanup(func() {
		mustExec(t, f.owner, `UPDATE control.storefront_domains SET state='ACTIVE' WHERE tenant_id=$1 AND store_id=$2 AND state='SUSPENDED'`, f.tenantA, f.storeA1)
	})
	if _, err := e.crTrigger(t); planCode(err) != "no_storefront" {
		t.Fatalf("trigger without a storefront domain: %v", err)
	}
	if len(e.crOps(t)) != 0 {
		t.Fatal("a reminder was planned without a link")
	}
}

// Single buyer: only that bundle is planned; a paid bundle is 409 not_remindable, an unknown one 404; repeating it is already_reminded.
func TestCheckoutReminderSingleBuyer(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	one, other, paid := e.crBuyer(t), e.crBuyer(t), e.crBuyer(t)
	e.crOrder(t, paid.bundle, "CONFIRMED", "storefront", false)
	single := func(bundle string) (merchanttools.ReminderResult, error) {
		return e.crTriggerAs(t, e.h.token, nil, bundle)
	}
	if out, err := single(one.bundle); err != nil || out.Queued != 1 || out.Followup != 0 {
		t.Fatalf("single: %+v err=%v", out, err)
	}
	if n := miCount(t, e.h.f.owner, `SELECT count(*) FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3`, e.h.f.tenantA, e.h.f.storeA1, other.bundle); n != 0 {
		t.Fatalf("the other buyer was touched: %d rows", n)
	}
	if out, err := single(one.bundle); err != nil || out.Queued != 0 || out.AlreadyReminded != 1 {
		t.Fatalf("repeat: %+v err=%v", out, err)
	}
	if _, err := single(paid.bundle); planCode(err) != "not_remindable" {
		t.Fatalf("paid bundle: %v", err)
	}
	if _, err := single(randomUUID()); planCode(err) != "not_found" {
		t.Fatalf("unknown bundle: %v", err)
	}
	// P2-9: a bundle whose claim lines are gone is refused by the planner (never a reminder for nothing).
	mustExec(t, e.h.f.owner, `UPDATE claims.bundles SET line_count=0 WHERE id=$1`, other.bundle)
	if _, err := single(other.bundle); planCode(err) != "not_remindable" {
		t.Fatalf("bundle without lines: %v", err)
	}
	if len(e.crOps(t)) != 1 {
		t.Fatalf("operations=%d, want 1", len(e.crOps(t)))
	}
}

// CR08: no PSID, bearer link or token in any persisted row of the reminder (operation request, display copy, reminder row, audit, receipts, job args)
// and the display copy carries the {{連結}} placeholder instead of the URL.
func TestCheckoutReminderCR08NoSecretsPersisted(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	b := e.crBuyer(t)
	if out, err := e.crTrigger(t); err != nil || out.Queued != 1 {
		t.Fatalf("trigger: %+v err=%v", out, err)
	}
	op := e.crOps(t)[0]
	e.run(t, op)
	e.awaitOp(t, op, "SUCCEEDED", 10*time.Second, "completed")
	token := crTokenOf(t, e.crSentLink(t, 0))
	var dump string
	err := f.owner.QueryRow(context.Background(), `SELECT (SELECT coalesce(string_agg(o.request::text||o.state||coalesce(o.result_code,''),'|'),'') FROM integration.operations o WHERE o.id=$1)
		|| (SELECT coalesce(string_agg(r::text,'|'),'') FROM inbox.checkout_reminders r WHERE r.tenant_id=$2 AND r.store_id=$3)
		|| (SELECT coalesce(string_agg(a.action,'|'),'') FROM ops.audit_events a WHERE a.tenant_id=$2 AND a.store_id=$3)
		|| (SELECT coalesce(string_agg(c::text,'|'),'') FROM ops.command_results c WHERE c.tenant_id=$2 AND c.store_id=$3)
		|| (SELECT coalesce(string_agg(j.args::text,'|'),'') FROM river.river_job j WHERE j.args->>'operation_id'=$1::text)
		|| (SELECT coalesce(string_agg(m.template_id,'|'),'') FROM inbox.outbound_messages m WHERE m.operation_id=$1)`, op, f.tenantA, f.storeA1).Scan(&dump)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, b.psid) || strings.Contains(dump, "http") || strings.Contains(dump, token) || strings.Contains(dump, "#t=") {
		t.Fatalf("persisted reminder rows leak a PSID, a link or a token: %s", dump)
	}
	var thread inbox.ThreadView
	if err := e.scoped(t, "inbox:read", func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
		var err error
		thread, err = e.svc.ReadThread(ctx, tx, b.conv, nil, 50)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range thread.Items {
		if it.Direction == "out" {
			found = true
			if strings.Contains(it.Text, "http") || strings.Contains(it.Text, token) || !strings.Contains(it.Text, "{{連結}}") {
				t.Fatalf("display copy: %q", it.Text)
			}
		}
	}
	if !found {
		t.Fatalf("the reminder is not in the thread: %+v", thread.Items)
	}
}

// Exact ACL of migration 0131: table privileges, RLS, function owners / search_path / EXECUTE lists, the new integration-writer column reads and policies.
func TestCheckoutReminderMigration0131ExactACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	if n := countRows(t, f.owner, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0131_checkout_reminders.sql'`); n != 1 {
		t.Fatalf("0131 migration rows=%d", n)
	}
	for _, table := range []string{"inbox.checkout_reminders", "live.reminder_settings"} {
		var rls, force bool
		if err := f.owner.QueryRow(ctx, `SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid=$1::regclass`, table).Scan(&rls, &force); err != nil || !rls || !force {
			t.Fatalf("%s RLS enabled=%v forced=%v err=%v", table, rls, force, err)
		}
		for _, role := range []string{"commerce_runtime", "commerce_buyer_runtime", "commerce_claims_worker", "commerce_payment_worker", "commerce_expiry_worker", "commerce_ads_worker"} {
			var can bool
			if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,$2,'SELECT') OR has_table_privilege($1,$2,'INSERT') OR has_table_privilege($1,$2,'UPDATE') OR has_table_privilege($1,$2,'DELETE')`, role, table).Scan(&can); err != nil || can {
				t.Fatalf("%s holds a table privilege on %s (err=%v)", role, table, err)
			}
		}
	}
	// live.reminder_settings is DEFERRED: not even the writer holds a privilege on it, and no definer touches it.
	if n := countRows(t, f.owner, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='live' AND p.proname LIKE '%reminder_settings%'`); n != 0 {
		t.Fatalf("settings definers exist: %d", n)
	}
	if countRows(t, f.owner, `SELECT count(*) FROM pg_proc WHERE prosrc ~ 'live\.reminder_settings'`) != 0 {
		t.Fatal("a function reads live.reminder_settings")
	}
	runtimeAndOwner := []string{"commerce_integration_writer", "commerce_runtime"}
	for _, fn := range []struct {
		signature string
		grantees  []string
		plain     bool
	}{
		{"inbox.checkout_reminder_candidates(uuid,text,int,uuid)", runtimeAndOwner, false},
		{"inbox.plan_checkout_reminder(uuid,uuid,uuid,text,bigint,text,uuid,bigint,uuid,bytea,text,bytea,bytea,bytea,bytea,text,bigint)", runtimeAndOwner, false},
		{"inbox.reminder_report(uuid)", runtimeAndOwner, false},
		{"inbox.crm_bundle_facts(uuid,uuid,uuid)", []string{"commerce_integration_writer"}, true},
		{"inbox.check_send(uuid)", []string{"commerce_claims_worker", "commerce_integration_writer"}, false},
		{"inbox.lcn_rate_check(uuid,uuid)", []string{"commerce_integration_writer"}, true},
	} {
		var owner string
		var definer, noLogin, noBypass bool
		var principals, config []string
		if err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner), p.prosecdef, NOT r.rolcanlogin, NOT r.rolbypassrls,
			 ARRAY(SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END
			   FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.privilege_type='EXECUTE' ORDER BY 1),
			 coalesce(p.proconfig,ARRAY[]::text[])
			 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, fn.signature).Scan(&owner, &definer, &noLogin, &noBypass, &principals, &config); err != nil {
			t.Fatalf("%s: %v", fn.signature, err)
		}
		if owner != "commerce_integration_writer" || (definer == fn.plain) || !noLogin || !noBypass || !slices.Equal(principals, fn.grantees) {
			t.Fatalf("%s: owner=%s definer=%v noLogin=%v noBypass=%v ACL=%v want %v", fn.signature, owner, definer, noLogin, noBypass, principals, fn.grantees)
		}
		if !slices.Equal(config, []string{"search_path=pg_catalog"}) {
			t.Fatalf("%s: proconfig=%v, want search_path=pg_catalog pinned", fn.signature, config)
		}
	}
	// The worker side gained nothing: no claims/payment/expiry/ads worker may run a reminder producer (the automatic leg is deferred).
	for _, role := range []string{"commerce_claims_worker", "commerce_payment_worker", "commerce_expiry_worker", "commerce_ads_worker"} {
		for _, sig := range []string{"inbox.checkout_reminder_candidates(uuid,text,int,uuid)", "inbox.reminder_report(uuid)"} {
			var can bool
			if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, role, sig).Scan(&can); err != nil || can {
				t.Fatalf("%s can execute %s (err=%v)", role, sig, err)
			}
		}
	}
	// The exact column reads 0131 adds for commerce_integration_writer, and the policies that scope them (inventory pinned here; KC03 pins the bundle matrix).
	got := lcStrings(t, f.owner, `SELECT p.table_schema||'.'||p.table_name||'.'||p.column_name FROM information_schema.column_privileges p
		WHERE p.grantee='commerce_integration_writer' AND p.privilege_type='SELECT'
		  AND (p.table_schema||'.'||p.table_name, p.column_name) IN (('claims.order_origins','tenant_id'),('claims.order_origins','store_id'),('claims.order_origins','order_id'),('claims.order_origins','bundle_id'),
		     ('checkout.orders','tenant_id'),('checkout.orders','store_id'),('checkout.orders','id'),('checkout.orders','commercial_state'),('checkout.orders','expires_at'),('checkout.orders','source'),
		     ('claims.links','tenant_id'),('claims.links','store_id'),('claims.links','bundle_id'),('claims.links','generation'),
		     ('live.claim_sources','session_id'),('live.claim_sources','active'),('live.claim_sources','reply_locale'))`)
	if len(got) != 17 {
		t.Fatalf("0131 column reads: %v", got)
	}
	// Never the link hash (checkout.orders carries older, unrelated grants of other units; only this unit's additions are pinned above).
	if countRows(t, f.owner, `SELECT count(*) FROM information_schema.column_privileges p WHERE p.grantee='commerce_integration_writer' AND p.table_schema='claims' AND p.table_name='links' AND p.column_name='token_hash' AND p.privilege_type='SELECT'`) != 0 {
		t.Fatal("claims.links token_hash is readable by the integration writer")
	}
	policies := lcStrings(t, f.owner, `SELECT tablename||'|'||policyname FROM pg_policies WHERE policyname IN ('reminders_rw','order_origin_reminder_read','order_reminder_read','link_generation_reminder_read','reminder_audit') ORDER BY 1`)
	wantPolicies := []string{"checkout_reminders|reminders_rw", "links|link_generation_reminder_read", "order_origins|order_origin_reminder_read", "orders|order_reminder_read", "audit_events|reminder_audit"}
	slices.Sort(wantPolicies)
	slices.Sort(policies)
	if !slices.Equal(policies, wantPolicies) {
		t.Fatalf("0131 policies: %v want %v", policies, wantPolicies)
	}
	// The fixed template exists, dm only, not public-safe, and merchants still cannot republish it.
	var kinds []string
	var safe bool
	if err := f.owner.QueryRow(ctx, `SELECT kinds, public_safe FROM msgtemplates.fixed_templates WHERE template_id='checkout-reminder/v1' AND version=1`).Scan(&kinds, &safe); err != nil || safe || !slices.Equal(kinds, []string{"dm"}) {
		t.Fatalf("checkout-reminder/v1: kinds=%v safe=%v err=%v", kinds, safe, err)
	}
}
