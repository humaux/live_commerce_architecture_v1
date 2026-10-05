package foundation_test

// Real-PG gates LCN10 (takeover), LCN11 (UNKNOWN is never re-POSTed; the dispatch copy is wiped on every terminal path), LCN13's display-copy
// half (no link persists; PSID and text appear in no table) and the A9 merge of the merchant's own sends. Harness: live_console_send_test.go.
// The retention halves of LCN13 (C5/C5c/C7/RD4 deletes) belong to the retention unit and are NOT_RUN here.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

func (e *lbEnv) takeover(t *testing.T, conv string, gen int64, release bool) (inbox.TakeoverOutput, error) {
	t.Helper()
	var out inbox.TakeoverOutput
	err := e.scoped(t, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var err error
		if release {
			out, err = e.svc.Release(ctx, tx, conv, inbox.TakeoverInput{ExpectedGeneration: gen})
		} else {
			out, err = e.svc.Takeover(ctx, tx, conv, inbox.TakeoverInput{ExpectedGeneration: gen})
		}
		return err
	})
	return out, err
}

// autoOp plans one automatic claim-link reply (a new bundle) and returns it.
func (e *lbEnv) autoOp(t *testing.T) mciReply {
	t.Helper()
	r := e.planReply(t, false, "", "A1")
	if r.op == "" || r.bundleID == "" {
		t.Fatal("no automatic reply planned")
	}
	return r
}

// linkPeer links a bundle to a conversation exactly as the Finish hook does after a SUCCEEDED private reply.
func (e *lbEnv) linkPeer(t *testing.T, bundle, conv, op string) {
	t.Helper()
	mustExec(t, e.h.f.owner, `INSERT INTO inbox.bundle_peers(tenant_id,store_id,bundle_id,peer_key,app_id,object,asset_id,operation_id)
		SELECT tenant_id,store_id,$2::uuid,peer_key,app_id,object,asset_id,$3::uuid FROM social.conversations WHERE id=$1`, conv, bundle, op)
}

func (e *lbEnv) freezeRequest(t *testing.T, op, patch string) {
	t.Helper()
	mustExec(t, e.h.f.owner, `UPDATE integration.operations SET request=request||$2::jsonb, request_hash=sha256(convert_to((request||$2::jsonb)::text,'UTF8')) WHERE id=$1`, op, patch)
}

func (e *lbEnv) setHuman(t *testing.T, conv string, gen int64) {
	t.Helper()
	mustExec(t, e.h.f.owner, `UPDATE inbox.conversation_state SET mode='human', assignee_principal=$2, takeover_generation=$3, human_until=clock_timestamp()+interval '6 hours' WHERE conversation_id=$1`, conv, e.h.actor, gen)
}

func TestLiveConsoleSendLCN10Takeover(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f

	// Implicit takeover on the first DM only; a second DM by the same staff member keeps the generation.
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	first, err := e.sendDM(conv, "one", 0)
	if err != nil || *first.TakeoverGeneration != 1 {
		t.Fatalf("first DM: %+v %v", first, err)
	}
	second, err := e.sendDM(conv, "two", 1)
	if err != nil || *second.TakeoverGeneration != 1 {
		t.Fatalf("second DM by the assignee must keep the generation: %+v %v", second, err)
	}
	if n := e.auditCount(t, "inbox.takeover"); n != 1 {
		t.Fatalf("takeover audit rows=%d, want exactly the first DM", n)
	}
	if _, err := e.sendDM(conv, "three", 0); planCode(err) != "takeover_changed" {
		t.Fatalf("stale expected_generation: %v", err)
	}

	// Human DM queued then released before dispatch: still sent (no generation check for origin=human).
	convR := e.postDM(t, mciDigits(15), "hello", time.Now())
	queued, err := e.sendDM(convR, "queued before release", 0)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := e.takeover(t, convR, 1, true); err != nil || out.Mode != "auto" || out.TakeoverGeneration != 2 {
		t.Fatalf("release: %+v %v", out, err)
	}
	before := e.g.count()
	e.run(t, queued.OperationID)
	e.awaitOp(t, queued.OperationID, "SUCCEEDED", 10*time.Second, "completed")
	if e.g.count() != before+1 {
		t.Fatalf("released-before-dispatch human DM was not sent (graph calls %d -> %d)", before, e.g.count())
	}

	// Takeover expires: the effective generation is stored+1 even before the lazy write; planners freeze it (P2-3).
	convX := e.postDM(t, mciDigits(15), "hello", time.Now())
	if _, err := e.sendDM(convX, "taking over", 0); err != nil {
		t.Fatal(err)
	}
	if s := e.state(t, convX); s.HumanUntil == nil || s.HumanUntil.Before(time.Now().Add(5*time.Hour+50*time.Minute)) || s.HumanUntil.After(time.Now().Add(6*time.Hour+time.Minute)) {
		t.Fatalf("human_until is not 6 h after the human DM: %+v", s.HumanUntil)
	}
	mustExec(t, f.owner, `UPDATE inbox.conversation_state SET human_until=clock_timestamp()-interval '1 second' WHERE conversation_id=$1`, convX)
	var mode string
	var gen int64
	if err := f.owner.QueryRow(context.Background(), `SELECT mode, takeover_generation FROM inbox.dm_window($1,$2,$3)`, f.tenantA, f.storeA1, convX).Scan(&mode, &gen); err != nil || mode != "auto" || gen != 2 {
		t.Fatalf("effective window state mode=%s gen=%d err=%v", mode, gen, err)
	}
	if _, err := e.sendDM(convX, "stale", 1); planCode(err) != "takeover_changed" {
		t.Fatalf("expired takeover, stale generation: %v", err)
	}
	if out, err := e.sendDM(convX, "after expiry", 2); err != nil || *out.TakeoverGeneration != 3 {
		t.Fatalf("expired takeover, effective generation: %+v %v", out, err)
	}

	// Automated sends. (a) conversation_known=false is resolved at Check through the bundle's peers: human -> human_takeover, zero HTTP.
	convA := e.postDM(t, mciDigits(15), "hello", time.Now())
	a := e.autoOp(t)
	e.linkPeer(t, a.bundleID, convA, a.op)
	e.setHuman(t, convA, 1)
	before = e.g.count()
	e.run(t, a.op)
	if code, _ := e.awaitOp(t, a.op, "BLOCKED_POLICY", 10*time.Second, "completed"); code != "human_takeover" || e.g.count() != before {
		t.Fatalf("auto reply to a human-owned thread: code=%s graph calls %d -> %d", code, before, e.g.count())
	}

	// (b) known at generation N, then a human takeover -> human_takeover.
	convB := e.postDM(t, mciDigits(15), "hello", time.Now())
	b := e.autoOp(t)
	e.linkPeer(t, b.bundleID, convB, b.op)
	e.freezeRequest(t, b.op, `{"conversation_known":true,"takeover_generation":0}`)
	e.setHuman(t, convB, 1)
	e.run(t, b.op)
	if code, _ := e.awaitOp(t, b.op, "BLOCKED_POLICY", 10*time.Second, "completed"); code != "human_takeover" {
		t.Fatalf("known thread taken over: %s", code)
	}

	// (c) known at generation N, takeover then release -> generation+2 -> takeover_changed.
	convC := e.postDM(t, mciDigits(15), "hello", time.Now())
	c := e.autoOp(t)
	e.linkPeer(t, c.bundleID, convC, c.op)
	e.freezeRequest(t, c.op, `{"conversation_known":true,"takeover_generation":0}`)
	if _, err := e.takeover(t, convC, 0, false); err != nil {
		t.Fatal(err)
	}
	if out, err := e.takeover(t, convC, 1, true); err != nil || out.TakeoverGeneration != 2 {
		t.Fatalf("release: %+v %v", out, err)
	}
	before = e.g.count()
	e.run(t, c.op)
	if code, _ := e.awaitOp(t, c.op, "BLOCKED_POLICY", 10*time.Second, "completed"); code != "takeover_changed" || e.g.count() != before {
		t.Fatalf("released thread: code=%s graph calls %d -> %d", code, before, e.g.count())
	}

	// (d) an unlinked bundle (no peers) is unaffected: the automatic reply is sent.
	d := e.autoOp(t)
	e.run(t, d.op)
	e.awaitOp(t, d.op, "SUCCEEDED", 10*time.Second, "completed")
	// The SUCCEEDED automatic reply links its bundle to the buyer's thread through recipient_id (§3.7, ON CONFLICT DO NOTHING).
	if n := miCount(t, f.owner, `SELECT count(*) FROM inbox.bundle_peers WHERE bundle_id=$1 AND operation_id=$2`, d.bundleID, d.op); n != 1 {
		t.Fatalf("bundle_peers rows after a SUCCEEDED automatic reply: %d", n)
	}
}

// LCN11: 5xx / garbled / timeout -> UNKNOWN, query-only, never a second POST; the dispatch copy is wiped on every terminal path.
func TestLiveConsoleSendLCN11Unknown(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	for _, mode := range []string{"5xx", "garbled", "hang"} {
		t.Run(mode, func(t *testing.T) {
			conv := e.postDM(t, mciDigits(15), "hello", time.Now())
			out, err := e.sendDM(conv, "unknown-"+mode, 0)
			if err != nil {
				t.Fatal(err)
			}
			if e.secretCount(t, out.OperationID) != 1 {
				t.Fatal("no dispatch copy before the send")
			}
			before := e.g.count()
			e.g.setMode(mode)
			defer e.g.setMode("ok")
			e.run(t, out.OperationID)
			// UNKNOWN, then Reconcile (query-only: zero Graph calls) until the reconcile budget is spent.
			code, _ := e.awaitOp(t, out.OperationID, "UNKNOWN", 25*time.Second, "cancelled")
			if code != "reconcile_budget_exhausted" {
				t.Fatalf("final code %s", code)
			}
			if n := e.g.count() - before; n != 1 {
				t.Fatalf("%s: %d Graph POSTs for one operation (UNKNOWN must never be re-POSTed)", mode, n)
			}
			if e.secretCount(t, out.OperationID) != 0 {
				t.Fatalf("%s: send secret survived UNKNOWN", mode)
			}
		})
	}
	// SUCCEEDED, FAILED_FINAL and BLOCKED_POLICY wipes are asserted in LCN06; STALE_BINDING (claim-time, no Finish) is covered by the trigger:
	conv := e.postDM(t, mciDigits(15), "hello", time.Now())
	out, err := e.sendDM(conv, "stale binding", 0)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.h.f.owner, `UPDATE integration.operations SET state='STALE_BINDING', generation=1 WHERE id=$1`, out.OperationID)
	if e.secretCount(t, out.OperationID) != 0 {
		t.Fatal("send secret survived STALE_BINDING")
	}
}

// LCN13 (display-copy half) and the A9 merge: no link persists in a display copy; PSID and text appear in no table; the thread shows the
// merchant's own sends with their visible state.
func TestLiveConsoleSendLCN13DisplayCopyAndThread(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	psid := mciDigits(15)
	conv := e.postDM(t, psid, "buyer asks", time.Now())
	tag := t04Tag()
	typed := "付款請點 https://shop.example.test/pay/TOKEN" + tag + " 或 ｗｗｗ．ｆｕｌｌｗｉｄｔｈ．ｃｏｍ／ｘ 謝謝"
	out, err := e.sendDM(conv, typed, 0)
	if err != nil {
		t.Fatal(err)
	}
	e.run(t, out.OperationID)
	e.awaitOp(t, out.OperationID, "SUCCEEDED", 10*time.Second, "completed")
	reqs := e.g.all()
	if got, _ := reqs[len(reqs)-1].body["message"].(map[string]any)["text"].(string); got != typed {
		t.Fatalf("the dispatch copy must carry the merchant's original text, got %q", got)
	}
	// The thread: inbound row plus the outbound display copy with every link replaced, state sent.
	var view inbox.ThreadView
	if err := e.scoped(t, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var err error
		view, err = e.svc.ReadThread(ctx, tx, conv, nil, 50)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var in, outItems int
	for _, it := range view.Items {
		switch it.Direction {
		case "in":
			in++
		case "out":
			outItems++
			if it.Unreadable != nil || it.SendState == nil || *it.SendState != "sent" || it.Kind == nil || *it.Kind != "dm" || it.PrincipalID == nil {
				t.Fatalf("outbound item: %+v", it)
			}
			if strings.Contains(it.Text, "https") || strings.Contains(it.Text, "TOKEN"+tag) || strings.Contains(it.Text, "ｗｗｗ") || !strings.Contains(it.Text, "{{連結}}") || !strings.Contains(it.Text, "謝謝") {
				t.Fatalf("display copy keeps a link: %q", it.Text)
			}
		}
	}
	if in != 1 || outItems != 1 || len(view.Items) != 2 || view.Items[0].Direction != "out" {
		t.Fatalf("merged thread: in=%d out=%d order=%v", in, outItems, view.Items)
	}
	// An UNKNOWN send renders 「不確定是否送達」 (send_state unknown) in the thread.
	e.g.setMode("5xx")
	u, err := e.sendDM(conv, "this one will be unknown "+tag, 1)
	if err != nil {
		t.Fatal(err)
	}
	e.run(t, u.OperationID)
	e.awaitOp(t, u.OperationID, "UNKNOWN", 15*time.Second)
	e.g.setMode("ok")
	if err := e.scoped(t, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var err error
		view, err = e.svc.ReadThread(ctx, tx, conv, nil, 50)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	unknown := 0
	for _, it := range view.Items {
		if it.SendState != nil && *it.SendState == "unknown" {
			unknown++
		}
	}
	if unknown != 1 {
		t.Fatalf("unknown send_state items=%d in %d items", unknown, len(view.Items))
	}

	// A8 / A13 read model: the conversation lists (display names come from social.conversation_heads; none is stored for this synthetic
	// sender) and the buyer panel reports link_pending_manual=false for a conversation without flagged bundles.
	if err := e.scoped(t, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		list, err := e.svc.ListConversations(ctx, tx, inbox.ListRequest{Filter: "all", Limit: 50})
		if err != nil {
			return err
		}
		found := false
		for _, it := range list.Items {
			if it.ConversationID == conv {
				found = true
				if it.Mode != "human" || it.Assignee == nil || *it.Assignee != e.h.actor || it.DisplayName != nil {
					return fmt.Errorf("list item: %+v", it)
				}
			}
		}
		if !found {
			return fmt.Errorf("conversation missing from A8 (%d items)", len(list.Items))
		}
		panel, err := e.svc.BuyerPanel(ctx, tx, conv)
		if err != nil || panel.LinkPendingManual || panel.Platform != "messenger" {
			return fmt.Errorf("panel: %+v %v", panel, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.scoped(t, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		_, err := e.svc.BuyerPanelByBundle(ctx, tx, "00000000-0000-4000-8000-000000000000")
		return err
	}); planCode(err) != "not_found" {
		t.Fatalf("an unknown bundle must be 404: %v", err)
	}

	// Leak scan: the PSID, the typed text and the link token appear in no operation request, event, River job, audit row or command receipt.
	for _, needle := range []string{psid, "TOKEN" + tag, "this one will be unknown", "付款請點"} {
		for _, q := range []string{
			`SELECT count(*) FROM integration.operations WHERE request::text LIKE '%'||$1||'%' OR result_code LIKE '%'||$1||'%'`,
			`SELECT count(*) FROM river.river_job WHERE args::text LIKE '%'||$1||'%' OR metadata::text LIKE '%'||$1||'%'`,
			`SELECT count(*) FROM ops.audit_events WHERE action LIKE '%'||$1||'%'`,
			`SELECT count(*) FROM ops.command_results WHERE response::text LIKE '%'||$1||'%'`,
		} {
			if n := miCount(t, f.owner, q, needle); n != 0 {
				t.Fatalf("%q found %d times by %.60s", needle, n, q)
			}
		}
	}
	// The dispatch copy is opaque at rest (HPKE ciphertext of the JSON; the PSID is not a substring).
	var leaked int64
	if err := f.owner.QueryRow(context.Background(), `SELECT count(*) FROM inbox.outbound_messages WHERE position(convert_to($1,'UTF8') in ciphertext)>0`, "https").Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("display ciphertext contains a link marker: %d %v", leaked, err)
	}
	_ = fmt.Sprint
}
