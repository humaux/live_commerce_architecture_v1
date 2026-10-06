package foundation_test

// Real-PG gates of W3-03B (checkout reminders; migration 0131; brief docs/delivery/units/w3-03b-checkout-reminder.md): CR01..CR08 plus the exact
// ACL of the new definers/tables and the reminder settings CAS. Harness: live_console_send_test.go (lbSetup: the REAL inbox.Service planners on
// commerce_runtime, the REAL metareply send routes on a REAL core dispatcher over a loopback fake Graph). Evidence label: MOCK. Every id, name
// and text is a synthetic sentinel; no Meta token, no non-loopback network. Meta LIVE = NOT_RUN.

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

// crBuyer is one synthetic buyer: a claimed bundle (comment A1), a Messenger conversation and the bundle<->peer link a successful private
// reply would have written (inbox.finish_send), so the bundle's thread is known.
type crBuyer struct{ psid, bundle, conv string }

// crBuyer claims A1 as a fresh buyer, opens a conversation (window open now) and links the bundle to it.
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
	return b
}

// crTrigger runs one reminder pass as the harness merchant (inbox:reply).
func (e *lbEnv) crTrigger(t *testing.T) (inbox.ReminderOutput, error) {
	t.Helper()
	var out inbox.ReminderOutput
	err := e.scoped(t, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = e.svc.PlanCheckoutReminders(ctx, tx, s, lbKey(), e.session, "")
		return err
	})
	// Park the new jobs at once (one statement): a stray default-queue worker of this binary must not dispatch an operation the test means to
	// hold back (e.run moves it from the parking queue to this harness's dispatcher).
	mustExec(t, e.h.f.owner, `UPDATE river.river_job SET queue='cr_hold' WHERE kind='external_operation_v1' AND state='available' AND queue='default'
		AND args->>'operation_id' IN (SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND request->>'message_type'='checkout_reminder')`,
		e.h.f.tenantA, e.h.f.storeA1)
	return out, err
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

// crOps returns the checkout-reminder DM operations of this store, oldest first.
func (e *lbEnv) crOps(t *testing.T) []string {
	t.Helper()
	rows, err := e.h.f.owner.Query(context.Background(), `SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2
		AND action='meta.dm_send' AND request->>'message_type'='checkout_reminder' ORDER BY created_at`, e.h.f.tenantA, e.h.f.storeA1)
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
		mustExec(t, f.owner, `DELETE FROM live.reminder_settings WHERE tenant_id=$1 AND store_id=$2`, f.tenantA, f.storeA1)
		mustExec(t, f.owner, `DELETE FROM claims.order_origins WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, e.session)
		mustExec(t, f.owner, `DELETE FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND snapshot->>'cr'='1'`, f.tenantA, f.storeA1)
	})
}

// crOrder inserts one synthetic order of the bundle in the given commercial state (disclosed fixture: the checkout pipeline needs a full
// buyer/cart/quote graph that this gate does not exercise; the reminder only reads commercial_state + expires_at through claims.order_origins).
func (e *lbEnv) crOrder(t *testing.T, bundle, state string, expired bool) {
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
		country,service_code,service_version,allocation_version,currency,total_minor,commercial_state,fulfillment_state,generation,expires_at,job_id,snapshot,created_at,updated_at)
		VALUES($1,$2,gen_random_uuid(),$3,gen_random_uuid(),gen_random_uuid(),1,gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),
		'TW','std',1,1,'TWD',1000,$4,$5,1,`+expires+`,(random()*9e12)::bigint+1000000000000,'{"cr":"1"}'::jsonb,`+created+`,`+created+`)`,
		f.tenantA, f.storeA1, order, state, fulfil); err != nil {
		t.Fatalf("synthetic order: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO claims.order_origins(tenant_id,store_id,order_id,bundle_id,offer_id,line_version,session_id,occurred_at)
		VALUES($1,$2,$3,$4,gen_random_uuid(),1,$5,clock_timestamp())`, f.tenantA, f.storeA1, order, bundle, e.session); err != nil {
		t.Fatalf("synthetic order origin: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// CR01 + CR02 + the followup -> queued transition + the Graph body: only the buyer inside the window is messaged (RESPONSE, no tag), the
// closed-window and the human-takeover buyers become follow-ups, and a second trigger never reminds twice.
func TestCheckoutReminderCR01WindowTakeoverAndOnce(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	open, closed, human := e.crBuyer(t), e.crBuyer(t), e.crBuyer(t)
	e.setInbound(t, closed.conv, time.Now().Add(-30*time.Hour))
	e.setHuman(t, human.conv, 1)

	out, err := e.crTrigger(t)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if out.Queued != 1 || out.Followup != 2 || out.AlreadyReminded != 0 || out.Skipped != 0 || out.Truncated {
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
	for _, want := range []string{`"origin": "auto"`, `"template_id": "checkout-reminder/v1"`, `"conversation_id": "` + open.conv + `"`, `"bundle_id": "` + open.bundle + `"`} {
		if !strings.Contains(request, want) {
			t.Fatalf("frozen request lacks %s: %s", want, request)
		}
	}
	if strings.Contains(request, open.psid) || strings.Contains(request, "checkout") && strings.Contains(request, "http") {
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

	// Dispatch: RESPONSE inside the window, the buyer's PSID, the store's non-bearer checkout link; no tag material.
	e.run(t, ops[0])
	e.awaitOp(t, ops[0], "SUCCEEDED", 10*time.Second, "completed")
	reqs := e.g.all()
	if len(reqs) != 1 {
		t.Fatalf("graph traffic: %+v", reqs)
	}
	body := reqs[0].body
	rcp, _ := body["recipient"].(map[string]any)
	msg, _ := body["message"].(map[string]any)
	text, _ := msg["text"].(string)
	if body["messaging_type"] != "RESPONSE" || rcp["id"] != open.psid || !strings.Contains(text, e.origin+"/zh-TW/checkout") {
		t.Fatalf("reminder body: %v (origin %s)", body, e.origin)
	}
	for _, banned := range []string{"tag", "message_tag", "update", "HUMAN_AGENT", "ACCOUNT_UPDATE"} {
		if strings.Contains(reqs[0].raw, banned) {
			t.Fatalf("message tag material %q in the request: %s", banned, reqs[0].raw)
		}
	}
	if e.secretCount(t, ops[0]) != 0 {
		t.Fatal("dispatch copy survived SUCCEEDED")
	}
	if rep := e.crReport(t); len(rep.Sent) != 1 || rep.Sent[0].BundleID != open.bundle || rep.Sent[0].SendState != "sent" || rep.Queued != 0 {
		t.Fatalf("report after dispatch: %+v", rep)
	}

	// CR02: the same session again (manual after manual here; the key is per buyer, not per trigger): nothing new for the reminded buyer.
	out, err = e.crTrigger(t)
	if err != nil || out.Queued != 0 || out.AlreadyReminded != 1 || out.Followup != 2 {
		t.Fatalf("second trigger: %+v err=%v", out, err)
	}
	if len(e.crOps(t)) != 1 || e.g.count() != 1 {
		t.Fatalf("second trigger planned or sent again: ops=%d graph=%d", len(e.crOps(t)), e.g.count())
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

// CR02 (concurrent): two merchants clicking at once plan exactly one reminder (per-session advisory lock + unique crm: key).
func TestCheckoutReminderCR02ConcurrentTriggers(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	e.crBuyer(t)
	type res struct {
		out inbox.ReminderOutput
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

// CR03: the window closes between planning and Check -> BLOCKED_POLICY window_closed, zero Graph calls, secret wiped, never retried.
func TestCheckoutReminderCR03WindowClosesBeforeCheck(t *testing.T) {
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
	// A takeover between planning and Check is denied too (origin=auto, §3.6).
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
	// And a new trigger does not remind the same buyer again.
	if out, err := e.crTrigger(t); err != nil || out.Queued != 0 || out.AlreadyReminded != 1 {
		t.Fatalf("retrigger after UNKNOWN: %+v err=%v", out, err)
	}
}

// CR05: the candidate set. Claimed-not-ordered and ordered-unpaid are reminded; paid, cancelled, expired-unpaid and a no-lines bundle are not.
func TestCheckoutReminderCR05CandidateSet(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	claimed, unpaid, paid, cancelled, expired := e.crBuyer(t), e.crBuyer(t), e.crBuyer(t), e.crBuyer(t), e.crBuyer(t)
	e.crOrder(t, unpaid.bundle, "AWAITING_PAYMENT", false)
	e.crOrder(t, paid.bundle, "CONFIRMED", false)
	e.crOrder(t, cancelled.bundle, "CANCELLED", false)
	e.crOrder(t, expired.bundle, "AWAITING_PAYMENT", true)
	out, err := e.crTrigger(t)
	if err != nil || out.Queued != 2 || out.Followup != 0 || out.AlreadyReminded != 0 {
		t.Fatalf("trigger: %+v err=%v", out, err)
	}
	got := map[string]string{}
	rows, err := e.h.f.owner.Query(context.Background(), `SELECT bundle_id::text, reminder_state FROM inbox.checkout_reminders WHERE tenant_id=$1 AND store_id=$2`, e.h.f.tenantA, e.h.f.storeA1)
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
	if len(got) != 2 || got[claimed.bundle] != "claimed" || got[unpaid.bundle] != "awaiting_payment" {
		t.Fatalf("reminded set: %v", got)
	}
	// A bundle without a known thread is a no_peer follow-up (never a private-reply quota spend): drop the peer link of a fresh buyer.
	lone := e.crBuyer(t)
	mustExec(t, e.h.f.owner, `DELETE FROM inbox.bundle_peers WHERE tenant_id=$1 AND store_id=$2 AND bundle_id=$3`, e.h.f.tenantA, e.h.f.storeA1, lone.bundle)
	out, err = e.crTrigger(t)
	if err != nil || out.Queued != 0 || out.Followup != 1 || out.AlreadyReminded != 2 {
		t.Fatalf("no_peer trigger: %+v err=%v", out, err)
	}
	if rep := e.crReport(t); len(rep.Followup) != 1 || rep.Followup[0].Reason != "no_peer" || rep.Followup[0].BundleID != lone.bundle {
		t.Fatalf("report: %+v", rep)
	}
}

// CR06: permission and scope. No inbox:reply -> 403; another store / tenant -> 404; settings need live:manage.
func TestCheckoutReminderCR06PermissionAndScope(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	e.crBuyer(t)
	_, tokNoReply := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:read", "live:read", "live:manage")
	_, tokReplyOnly := lcPrincipal(t, f, f.tenantA, []string{f.storeA1}, "store:read", "inbox:reply")
	_, tokA2 := lcPrincipal(t, f, f.tenantA, []string{f.storeA2}, "store:read", "inbox:read", "inbox:reply", "live:read", "live:manage")
	_, tokB := lcPrincipal(t, f, f.tenantB, []string{f.storeB}, "store:read", "inbox:read", "inbox:reply", "live:read", "live:manage")
	run := func(token, store, permission string, fn func(ctx context.Context, tx pgx.Tx, s platform.Scope) error) error {
		return platform.WithScope(context.Background(), f.runtime, token, store, permission, func(tx pgx.Tx, s platform.Scope) error { return fn(context.Background(), tx, s) })
	}
	trigger := func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.PlanCheckoutReminders(ctx, tx, s, lbKey(), e.session, "")
		return err
	}
	// The definer, not just the HTTP gate, requires inbox:reply: the scope check passes with store:read, the planner refuses.
	if err := run(tokNoReply, f.storeA1, "store:read", trigger); planCode(err) != "forbidden" {
		t.Fatalf("trigger without inbox:reply: %v", err)
	}
	for name, tc := range map[string]struct{ token, store string }{"sister store": {tokA2, f.storeA2}, "other tenant": {tokB, f.storeB}} {
		if err := run(tc.token, tc.store, "inbox:reply", trigger); planCode(err) != "not_found" {
			t.Fatalf("%s trigger on a foreign session: %v", name, err)
		}
		err := run(tc.token, tc.store, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			_, err := e.svc.ReminderReport(ctx, tx, e.session)
			return err
		})
		if planCode(err) != "not_found" {
			t.Fatalf("%s report on a foreign session: %v", name, err)
		}
	}
	// Report needs inbox:read (the reply-only principal holds none).
	if err := run(tokReplyOnly, f.storeA1, "store:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.ReminderReport(ctx, tx, e.session)
		return err
	}); planCode(err) != "forbidden" {
		t.Fatalf("report without inbox:read: %v", err)
	}
	// Settings: live:manage to write, live:read to read.
	put := func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.PutReminderSettings(ctx, tx, inbox.ReminderSettingsInput{Enabled: true, DelayMinutes: 30, ExpectedVersion: 0})
		return err
	}
	if err := run(tokReplyOnly, f.storeA1, "store:read", put); planCode(err) != "forbidden" {
		t.Fatalf("settings put without live:manage: %v", err)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM inbox.checkout_reminders WHERE tenant_id=$1`, f.tenantA); n != 0 {
		t.Fatalf("refused triggers left %d reminder rows", n)
	}
}

// Settings: default off / 30 min / version 0; CAS on expected_version; delay bounds 10..1440; the store scope isolates rows.
func TestCheckoutReminderSettingsCAS(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	get := func() inbox.ReminderSettings {
		var s inbox.ReminderSettings
		if err := e.scoped(t, "live:read", func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			var err error
			s, err = e.svc.GetReminderSettings(ctx, tx)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return s
	}
	put := func(enabled bool, delay int, version int64) (inbox.ReminderSettings, error) {
		var s inbox.ReminderSettings
		err := e.scoped(t, "live:manage", func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
			var err error
			s, err = e.svc.PutReminderSettings(ctx, tx, inbox.ReminderSettingsInput{Enabled: enabled, DelayMinutes: delay, ExpectedVersion: version})
			return err
		})
		return s, err
	}
	if s := get(); s.Enabled || s.DelayMinutes != 30 || s.Version != 0 {
		t.Fatalf("default: %+v", s)
	}
	if s, err := put(true, 45, 0); err != nil || s.Version != 1 || !s.Enabled || s.DelayMinutes != 45 {
		t.Fatalf("create: %+v err=%v", s, err)
	}
	if _, err := put(true, 45, 0); planCode(err) != "version_conflict" {
		t.Fatalf("second create with version 0: %v", err)
	}
	if _, err := put(false, 60, 5); planCode(err) != "version_conflict" {
		t.Fatalf("stale version: %v", err)
	}
	if s, err := put(false, 60, 1); err != nil || s.Version != 2 || s.Enabled {
		t.Fatalf("update: %+v err=%v", s, err)
	}
	for _, delay := range []int{9, 1441} {
		if _, err := e.svc.PutReminderSettings(context.Background(), nil, inbox.ReminderSettingsInput{DelayMinutes: delay}); err == nil {
			t.Fatalf("delay %d accepted by the Go guard", delay)
		}
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND store_id=$2 AND action='live.reminder_settings.updated'`, f.tenantA, f.storeA1); n < 2 {
		t.Fatalf("settings audit rows=%d", n)
	}
	if err := e.scoped(t, "live:manage", func(ctx context.Context, tx pgx.Tx, _ platform.Scope) error {
		_, err := tx.Exec(ctx, `SELECT live.put_reminder_settings(true, 5, 2)`)
		return err
	}); planCode(err) != "invalid_request" {
		t.Fatalf("delay 5 at the definer: %v", err)
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

// CR08: no PSID, link, token or body in any persisted row of the reminder (operation request, display copy, reminder row, audit) and the
// display copy carries the {{連結}} placeholder instead of the URL.
func TestCheckoutReminderCR08NoSecretsPersisted(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	f := e.h.f
	b := e.crBuyer(t)
	if out, err := e.crTrigger(t); err != nil || out.Queued != 1 {
		t.Fatalf("trigger: %+v err=%v", out, err)
	}
	op := e.crOps(t)[0]
	var dump string
	err := f.owner.QueryRow(context.Background(), `SELECT (SELECT coalesce(string_agg(o.request::text||o.state||coalesce(o.result_code,''),'|'),'') FROM integration.operations o WHERE o.id=$1)
		|| (SELECT coalesce(string_agg(r::text,'|'),'') FROM inbox.checkout_reminders r WHERE r.tenant_id=$2 AND r.store_id=$3)
		|| (SELECT coalesce(string_agg(a.action,'|'),'') FROM ops.audit_events a WHERE a.tenant_id=$2 AND a.store_id=$3)
		|| (SELECT coalesce(string_agg(m.template_id,'|'),'') FROM inbox.outbound_messages m WHERE m.operation_id=$1)`, op, f.tenantA, f.storeA1).Scan(&dump)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, b.psid) || strings.Contains(dump, "http") || strings.Contains(dump, "/checkout") || strings.Contains(dump, "#t=") {
		t.Fatalf("persisted reminder rows leak a PSID or a link: %s", dump)
	}
	// The display copy opens to a link-free text through the A9 merge of the conversation thread.
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
			if strings.Contains(it.Text, "http") || !strings.Contains(it.Text, "{{連結}}") {
				t.Fatalf("display copy: %q", it.Text)
			}
		}
	}
	if !found {
		t.Fatalf("the reminder is not in the thread: %+v", thread.Items)
	}
}

// Exact ACL of migration 0131: table privileges, RLS, function owners and EXECUTE lists.
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
	runtimeAndOwner := []string{"commerce_integration_writer", "commerce_runtime"}
	for _, fn := range []struct {
		signature string
		grantees  []string
		plain     bool
	}{
		{"inbox.checkout_reminder_candidates(uuid,text,int,uuid)", runtimeAndOwner, false},
		{"inbox.plan_checkout_reminder(uuid,uuid,text,bigint,uuid,bigint,uuid,bytea,text,bytea,bytea,bytea,bytea,text,bigint)", runtimeAndOwner, false},
		{"inbox.reminder_report(uuid)", runtimeAndOwner, false},
		{"live.get_reminder_settings()", runtimeAndOwner, false},
		{"live.put_reminder_settings(boolean,integer,bigint)", runtimeAndOwner, false},
		{"inbox.crm_bundle_state(uuid,uuid,uuid)", []string{"commerce_integration_writer"}, true},
	} {
		var owner string
		var definer, noLogin, noBypass bool
		var principals []string
		if err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner), p.prosecdef, NOT r.rolcanlogin, NOT r.rolbypassrls,
			 ARRAY(SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END
			   FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.privilege_type='EXECUTE' ORDER BY 1)
			 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, fn.signature).Scan(&owner, &definer, &noLogin, &noBypass, &principals); err != nil {
			t.Fatalf("%s: %v", fn.signature, err)
		}
		if owner != "commerce_integration_writer" || (definer == fn.plain) || !noLogin || !noBypass || !slices.Equal(principals, fn.grantees) {
			t.Fatalf("%s: owner=%s definer=%v noLogin=%v noBypass=%v ACL=%v want %v", fn.signature, owner, definer, noLogin, noBypass, principals, fn.grantees)
		}
	}
	// The worker side gained nothing: no claims/payment/expiry/ads worker may run a reminder producer (the auto trigger is not in this unit).
	for _, role := range []string{"commerce_claims_worker", "commerce_payment_worker", "commerce_expiry_worker", "commerce_ads_worker"} {
		for _, sig := range []string{"inbox.checkout_reminder_candidates(uuid,text,int,uuid)", "inbox.reminder_report(uuid)", "live.put_reminder_settings(boolean,integer,bigint)"} {
			var can bool
			if err := f.owner.QueryRow(ctx, `SELECT has_function_privilege($1,$2::regprocedure,'EXECUTE')`, role, sig).Scan(&can); err != nil || can {
				t.Fatalf("%s can execute %s (err=%v)", role, sig, err)
			}
		}
	}
	// The fixed template exists, dm only, not public-safe, and merchants still cannot republish it.
	var kinds []string
	var safe bool
	if err := f.owner.QueryRow(ctx, `SELECT kinds, public_safe FROM msgtemplates.fixed_templates WHERE template_id='checkout-reminder/v1' AND version=1`).Scan(&kinds, &safe); err != nil || safe || !slices.Equal(kinds, []string{"dm"}) {
		t.Fatalf("checkout-reminder/v1: kinds=%v safe=%v err=%v", kinds, safe, err)
	}
}

// Single-buyer trigger (「提醒這位」): only that bundle is planned; a paid bundle is 409 not_remindable, an unknown one 404; repeating it is
// already_reminded and plans nothing.
func TestCheckoutReminderSingleBuyer(t *testing.T) {
	e := lbSetup(t)
	e.crCleanup(t)
	one, other, paid := e.crBuyer(t), e.crBuyer(t), e.crBuyer(t)
	e.crOrder(t, paid.bundle, "CONFIRMED", false)
	single := func(bundle string) (inbox.ReminderOutput, error) {
		var out inbox.ReminderOutput
		err := e.scoped(t, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			var err error
			out, err = e.svc.PlanCheckoutReminders(ctx, tx, s, lbKey(), e.session, bundle)
			return err
		})
		return out, err
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
	if len(e.crOps(t)) != 1 {
		t.Fatalf("operations=%d, want 1", len(e.crOps(t)))
	}
}
