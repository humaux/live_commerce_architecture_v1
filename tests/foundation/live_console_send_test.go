package foundation_test

// Real-PG gates of LC-B4 (contracts/live-console-v1.md §3.3-3.7, §4, Amendment 1 A1.1/A1.3/P2-1..5): LCN06 (DM), LCN07 (private-reply
// quota and the auto/manual race), LCN08 (public reply + recommend replay), LCN10 (takeover), LCN11 (UNKNOWN, secret wipe), LCN13 (display
// copy / secrecy halves; the retention halves belong to the retention unit), the exact ACL of migration 0128 and the A8/A9/A13 read model.
//
// Harness: the shared mciEnv (signed webhook -> consumer -> intake -> claims), the REAL inbox.Service planners against commerce_runtime, the
// REAL metareply routes on a REAL core dispatcher (claims-worker authority pool) against a loopback fake Graph. Evidence label: MOCK. Every
// id, name and text is a synthetic sentinel; no Meta token, no non-loopback network.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/inbox"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/msgtemplates"
	"livecommerce/internal/platform"
)

// ---------------------------------------------------------------------------------------
// fake Graph
// ---------------------------------------------------------------------------------------

type lbReq struct {
	method, path string
	body         map[string]any
	raw          string
}

type lbGraph struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []lbReq
	mode string // ok | 100 | 5xx | garbled | hang
	seq  int
}

func newLbGraph(t *testing.T) *lbGraph {
	t.Helper()
	g := &lbGraph{mode: "ok"}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		body := map[string]any{}
		_ = json.Unmarshal(raw, &body)
		g.mu.Lock()
		g.seq++
		n := g.seq
		mode := g.mode
		g.reqs = append(g.reqs, lbReq{method: r.Method, path: r.URL.Path, body: body, raw: string(raw)})
		g.mu.Unlock()
		switch mode {
		case "100":
			http.Error(w, `{"error":{"message":"synthetic invalid parameter","type":"OAuthException","code":100}}`, http.StatusBadRequest)
		case "5xx":
			http.Error(w, `{"error":{"message":"synthetic outage"}}`, http.StatusServiceUnavailable)
		case "garbled":
			_, _ = w.Write([]byte("<html>not json"))
		case "hang":
			<-r.Context().Done()
		default:
			if strings.HasSuffix(r.URL.Path, "/messages") {
				rcp, _ := body["recipient"].(map[string]any)
				id, _ := rcp["id"].(string)
				if id == "" {
					id = "9000" + fmt.Sprint(n) // a comment_id private reply answers with the buyer's PSID
				}
				_, _ = w.Write([]byte(fmt.Sprintf(`{"recipient_id":%q,"message_id":"m_SYNTH_%d"}`, id, n)))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"1111_%d"}`, n)))
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *lbGraph) setMode(m string) { g.mu.Lock(); g.mode = m; g.mu.Unlock() }
func (g *lbGraph) all() []lbReq {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]lbReq(nil), g.reqs...)
}
func (g *lbGraph) count() int { return len(g.all()) }

// ---------------------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------------------

type lbEnv struct {
	*mciEnv
	svc    *inbox.Service
	jobs   *river.Client[pgx.Tx]
	g      *lbGraph
	worker *pgxpool.Pool
	queue  string
	pageID string
}

func lbSetup(t *testing.T) *lbEnv {
	t.Helper()
	e := &lbEnv{mciEnv: mciSetup(t, mciOpts{private: true})}
	f := e.h.f
	ctx := context.Background()
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		VALUES($1,$2,$3,'inbox:read'),($1,$2,$3,'inbox:reply') ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, e.h.actor)
	// A token version with the send scopes (the registered v1 only attests pages_messaging).
	e.pageToken += "-v2"
	e.registerTokenVersion(t, "facebook", e.pageBinding, e.pageAsset, 1, []string{"pages_messaging", "pages_manage_engagement"}, e.pageToken)
	// Capability snapshot (§6): one active connection naming both bindings with the scopes the four capabilities need.
	e.pageID = e.pageAsset
	scopes := []string{"pages_messaging", "pages_manage_engagement", "pages_read_engagement", "instagram_manage_comments", "instagram_manage_messages"}
	mustExec(t, f.owner, `INSERT INTO integration.meta_connections(tenant_id,store_id,page_id,page_name,fb_binding,ig_binding,ig_id,ig_username,scopes,status,connected_by,route_expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'active',$10,$11)`,
		f.tenantA, f.storeA1, e.pageID, "lb-page-"+t04Tag(), e.pageBinding, e.igBinding, e.igAsset, "lb_ig_"+t04Tag(), scopes, e.h.actor, time.Now().Add(24*time.Hour))
	t.Cleanup(func() {
		for _, q := range []string{
			`UPDATE river.river_job SET queue='lb_dead' WHERE kind='external_operation_v1' AND state IN ('available','retryable','scheduled')
			   AND args->>'operation_id' IN (SELECT id::text FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action IN ('meta.dm_send','meta.public_reply','meta.offer_recommend','meta.private_reply'))`,
			`DELETE FROM integration.meta_connections WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`,
			`DELETE FROM inbox.bundle_peers WHERE tenant_id=$1 AND store_id=$2`,
			`DELETE FROM inbox.send_secrets WHERE tenant_id=$1 AND store_id=$2`,
			`DELETE FROM inbox.outbound_messages WHERE tenant_id=$1 AND store_id=$2`,
		} {
			args := []any{f.tenantA, f.storeA1}
			if strings.Contains(q, "$3") {
				args = append(args, e.pageID)
			}
			if _, err := f.owner.Exec(ctx, q, args...); err != nil {
				t.Logf("lb cleanup: %v", err)
			}
		}
	})

	keyJSON := `{"keys":[{"id":"` + miKeyID + `","key_base64":"` + base64.StdEncoding.EncodeToString(e.page.key) + `"}]}`
	kr, err := inbox.LoadKeyring(func(n string) string {
		switch n {
		case "COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID":
			return miKeyID
		case "COMMERCE_META_PAYLOAD_KEYS_JSON":
			return keyJSON
		}
		return ""
	})
	if err != nil {
		t.Fatal("inbox keyring", err)
	}
	if e.svc, err = inbox.NewService(kr); err != nil {
		t.Fatal(err)
	}
	sealKeys, open := mcnPageRing(t)
	if e.jobs, err = river.NewClient[pgx.Tx](riverpgxv5.New(f.runtime), &river.Config{Schema: "river"}); err != nil {
		t.Fatal(err)
	}
	e.svc.EnableSend(sealKeys, e.jobs, msgtemplates.NewService())

	e.g = newLbGraph(t)
	e.worker = miPool(t, f, waClaims)
	cfg := metareply.Config{GraphBaseURL: e.g.srv.URL, GraphVersion: "v99.0", HTTPClient: e.g.srv.Client()}
	routes, err := metareply.RoutesV2(e.worker, e.link, e.pageKeys, open, cfg)
	if err != nil {
		t.Fatal("RoutesV2", err)
	}
	sendRoutes, err := metareply.SendRoutes(e.worker, e.pageKeys, open, cfg)
	if err != nil {
		t.Fatal("SendRoutes", err)
	}
	opts := mciDispatchOptions()
	opts.CallTimeout = 1500 * time.Millisecond
	e.queue = "lb_dispatch_" + t04Tag()
	t06StartDispatcher(t, e.worker, e.queue, append(routes, sendRoutes...), opts)
	return e
}

// scoped runs fn in a real merchant transaction (platform.WithScope on commerce_runtime) of the actor.
func (e *lbEnv) scoped(t *testing.T, permission string, fn func(ctx context.Context, tx pgx.Tx, s platform.Scope) error) error {
	t.Helper()
	return platform.WithScope(context.Background(), e.h.f.runtime, e.h.token, e.h.f.storeA1, permission, func(tx pgx.Tx, s platform.Scope) error {
		return fn(context.Background(), tx, s)
	})
}

func lbKey() string { return "lb-" + strings.ReplaceAll(randomUUID(), "-", "")[:20] }

// postDM admits one signed Messenger message from psid and returns the conversation id.
func (e *lbEnv) postDM(t *testing.T, psid, text string, at time.Time) string {
	t.Helper()
	raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":%q,"text":%q}}]}]}`,
		e.pageAsset, psid, e.pageAsset, at.UnixMilli(), "m."+randomUUID(), text))
	ev := mcPost(t, e.page, e.pageAsset, raw)
	mcAwait(t, e.page, ev)
	return e.convOf(t, "page", e.pageAsset, psid)
}

func (e *lbEnv) postIGDM(t *testing.T, psid, text string, at time.Time) string {
	t.Helper()
	raw := []byte(fmt.Sprintf(`{"object":"instagram","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":%q,"text":%q}}]}]}`,
		e.igAsset, psid, e.igAsset, at.UnixMilli(), "m."+randomUUID(), text))
	ev := mcPost(t, e.ig, e.igAsset, raw)
	mcAwait(t, e.ig, ev)
	return e.convOf(t, "instagram", e.igAsset, psid)
}

func (e *lbEnv) convOf(t *testing.T, object, asset, psid string) string {
	t.Helper()
	var id string
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT id::text FROM social.conversations WHERE tenant_id=$1 AND store_id=$2 AND object=$3 AND asset_id=$4 AND peer_key=$5`,
		e.h.f.tenantA, e.h.f.storeA1, object, asset, meta.SocialPeerKey(miApp, object, asset, psid)).Scan(&id); err != nil {
		t.Fatalf("conversation of the posted DM: %v", err)
	}
	return id
}

type lbState struct {
	Mode                 string
	Gen                  int64
	Assignee             *string
	HumanUntil           *time.Time
	LastInbound, LastOut *time.Time
}

func (e *lbEnv) state(t *testing.T, conv string) lbState {
	t.Helper()
	var s lbState
	if err := e.h.f.owner.QueryRow(context.Background(), `SELECT mode,takeover_generation,assignee_principal::text,human_until,last_inbound_at,last_outbound_at FROM inbox.conversation_state WHERE conversation_id=$1`, conv).
		Scan(&s.Mode, &s.Gen, &s.Assignee, &s.HumanUntil, &s.LastInbound, &s.LastOut); err != nil {
		t.Fatalf("conversation_state: %v", err)
	}
	return s
}

func (e *lbEnv) setInbound(t *testing.T, conv string, at time.Time) {
	t.Helper()
	mustExec(t, e.h.f.owner, `UPDATE inbox.conversation_state SET last_inbound_at=$2 WHERE conversation_id=$1`, conv, at)
}

func (e *lbEnv) sendDM(conv, text string, gen int64) (inbox.SendOutput, error) {
	var out inbox.SendOutput
	err := platform.WithScope(context.Background(), e.h.f.runtime, e.h.token, e.h.f.storeA1, "inbox:reply", func(tx pgx.Tx, s platform.Scope) error {
		var err error
		out, err = e.svc.SendDM(context.Background(), tx, s, lbKey(), conv, inbox.DMInput{TextInput: inbox.TextInput{Text: text}, ExpectedGeneration: &gen}, nil)
		return err
	})
	return out, err
}

// run moves the planned operation's River job onto this harness's dispatcher queue.
func (e *lbEnv) run(t *testing.T, op string) {
	t.Helper()
	tag, err := e.h.f.owner.Exec(context.Background(), `UPDATE river.river_job SET queue=$1 WHERE state='available' AND id=(SELECT job_id FROM integration.operations WHERE id=$2)`, e.queue, op)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("operation job not available for the dispatcher queue: %v", err)
	}
}

func (e *lbEnv) secretCount(t *testing.T, op string) int64 {
	return miCount(t, e.h.f.owner, `SELECT count(*) FROM inbox.send_secrets WHERE operation_id=$1`, op)
}

// planCode returns the planner deny code (PT409 message) of err, "" when err is nil or not a planner refusal.
func planCode(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && strings.HasPrefix(pg.Code, "PT") {
		return pg.Message
	}
	var se *inbox.SendError
	if errors.As(err, &se) {
		return se.Code
	}
	return ""
}

// ---------------------------------------------------------------------------------------
// exact ACL of migration 0128
// ---------------------------------------------------------------------------------------

func TestLiveConsoleSendMigration0128ExactACL(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	var n int
	if err := f.owner.QueryRow(ctx, `SELECT count(*) FROM public.lc_schema_migrations WHERE version='0128_lc_b4_sends.sql'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("0128 migration rows=%d err=%v", n, err)
	}
	for _, table := range []string{"inbox.outbound_messages", "inbox.send_secrets", "inbox.bundle_peers"} {
		var rls, force bool
		if err := f.owner.QueryRow(ctx, `SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid=$1::regclass`, table).Scan(&rls, &force); err != nil || !rls || !force {
			t.Fatalf("%s RLS enabled=%v forced=%v err=%v", table, rls, force, err)
		}
		// No merchant, buyer or worker login reads these tables directly (the definers are the only path).
		for _, role := range []string{"commerce_runtime", "commerce_buyer_runtime", "commerce_claims_worker", "commerce_payment_worker"} {
			var can bool
			if err := f.owner.QueryRow(ctx, `SELECT has_table_privilege($1,$2,'SELECT') OR has_table_privilege($1,$2,'INSERT') OR has_table_privilege($1,$2,'UPDATE') OR has_table_privilege($1,$2,'DELETE')`, role, table).Scan(&can); err != nil || can {
				t.Fatalf("%s holds a table privilege on %s (err=%v)", role, table, err)
			}
		}
	}
	runtimeAndOwner := []string{"commerce_integration_writer", "commerce_runtime"}
	workerAndOwner := []string{"commerce_claims_worker", "commerce_integration_writer"}
	ownerOnly := []string{"commerce_integration_writer"}
	for _, fn := range []struct {
		signature, owner string
		grantees         []string
	}{
		{"inbox.plan_dm(uuid,bigint,uuid,uuid,bigint,uuid,bytea,text,bytea,bytea,bytea,bytea,text,bigint)", "commerce_integration_writer", runtimeAndOwner},
		{"inbox.plan_manual_private_reply(uuid,text,timestamptz,boolean,uuid,bigint,uuid,bytea,text,bytea,bytea,bytea,bytea,text,bigint)", "commerce_integration_writer", runtimeAndOwner},
		{"inbox.plan_public_reply(uuid,text,uuid,bigint,uuid,bytea,text,bytea,bytea,bytea,bytea,text,bigint)", "commerce_integration_writer", runtimeAndOwner},
		{"live.plan_offer_recommend(uuid,uuid,bigint,uuid,bigint,uuid,bytea,text,bytea,bytea,bytea,bytea,text,bigint)", "commerce_integration_writer", runtimeAndOwner},
		{"live.offer_recommend_facts(uuid,uuid)", "commerce_integration_writer", runtimeAndOwner},
		{"inbox.read_outbound(uuid,int)", "commerce_integration_writer", runtimeAndOwner},
		{"inbox.link_pending_bundles(int)", "commerce_integration_writer", runtimeAndOwner},
		{"inbox.link_pending_for(uuid,uuid)", "commerce_integration_writer", runtimeAndOwner},
		{"inbox.check_send(uuid)", "commerce_integration_writer", workerAndOwner},
		{"inbox.load_send_secret(uuid,bigint,bytea)", "commerce_integration_writer", workerAndOwner},
		{"inbox.finish_send(uuid,bigint,bytea,text,text)", "commerce_integration_writer", workerAndOwner},
		{"inbox.dm_window_for_bundle(uuid,uuid,uuid,text,text,text)", "commerce_meta_writer",
			[]string{"commerce_claims_worker", "commerce_claims_writer", "commerce_integration_writer", "commerce_meta_writer"}},
		{"social.conversation_heads(uuid[])", "commerce_meta_writer", []string{"commerce_meta_writer", "commerce_runtime"}},
		{"inbox.store_origins()", "commerce_integration_writer", runtimeAndOwner},
		{"inbox.clear_link_pending_manual()", "commerce_integration_writer", ownerOnly},
		{"inbox.lcn_scope()", "commerce_integration_writer", ownerOnly},
		{"inbox.lcn_in_scope(uuid,uuid)", "commerce_integration_writer", ownerOnly},
		{"inbox.lcn_rate_check(uuid,uuid)", "commerce_integration_writer", ownerOnly},
		{"inbox.wipe_send_secret()", "commerce_integration_writer", ownerOnly},
	} {
		var owner string
		var definer, noLogin, noBypass bool
		var principals []string
		err := f.owner.QueryRow(ctx, `SELECT pg_get_userbyid(p.proowner), p.prosecdef, NOT r.rolcanlogin, NOT r.rolbypassrls,
			 ARRAY(SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END
			   FROM aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE a.privilege_type='EXECUTE' ORDER BY 1)
			 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, fn.signature).Scan(&owner, &definer, &noLogin, &noBypass, &principals)
		if err != nil {
			t.Fatalf("%s: %v", fn.signature, err)
		}
		// lcn_scope / lcn_in_scope / lcn_rate_check are plain (invoker) helpers; every other function is SECURITY DEFINER.
		plain := strings.HasPrefix(fn.signature, "inbox.lcn_")
		if owner != fn.owner || (!plain && !definer) || !noLogin || !noBypass || !slices.Equal(principals, fn.grantees) {
			t.Fatalf("%s: owner=%s definer=%v noLogin=%v noBypass=%v ACL=%v want owner=%s %v", fn.signature, owner, definer, noLogin, noBypass, principals, fn.owner, fn.grantees)
		}
	}
	// Per-statement runtime proofs: a merchant login cannot call a worker-side definer and a worker cannot call a producer.
	for _, q := range []string{
		`SELECT inbox.check_send(gen_random_uuid())`,
		`SELECT * FROM inbox.load_send_secret(gen_random_uuid(),1,'\x00')`,
		`SELECT inbox.finish_send(gen_random_uuid(),1,'\x00','SUCCEEDED',NULL)`,
	} {
		if _, err := f.runtime.Exec(ctx, q); sqlState(err) != "42501" {
			t.Fatalf("merchant runtime ran a worker definer: %s: %v", q, err)
		}
	}
	if _, err := miPool(t, f, waClaims).Exec(ctx, `SELECT inbox.plan_public_reply(gen_random_uuid(),'1',gen_random_uuid(),1,gen_random_uuid(),'\x00','k','\x00','\x00','\x00','\x00',NULL,NULL)`); sqlState(err) != "42501" {
		t.Fatalf("claims worker ran a producer: %v", err)
	}
	// The operation lane stays "default" (claims worker) for facebook/instagram: the four actions need no new lane.
	var lane string
	if err := f.owner.QueryRow(ctx, `SELECT integration.operation_lane('facebook','MERCHANT')`).Scan(&lane); err != nil || lane != "default" {
		t.Fatalf("lane=%q err=%v", lane, err)
	}
}

// ---------------------------------------------------------------------------------------
// LCN06: DM
// ---------------------------------------------------------------------------------------

func TestLiveConsoleSendLCN06DM(t *testing.T) {
	e := lbSetup(t)
	f := e.h.f
	ctx := context.Background()
	psid := mciDigits(15)
	secretText := "SENTINEL-DM-TEXT-" + t04Tag()
	conv := e.postDM(t, psid, "buyer says hi", time.Now())
	st := e.state(t, conv)
	if st.LastInbound == nil || st.Mode != "auto" || st.Gen != 0 {
		t.Fatalf("fresh conversation state: %+v", st)
	}

	// Inside the window: exactly one operation; implicit takeover (§3.6).
	out, err := e.sendDM(conv, secretText, 0)
	if err != nil {
		t.Fatalf("send DM: %v", err)
	}
	if out.SendState != "queued" || out.OperationID == "" || out.OutboundID == "" || out.TakeoverGeneration == nil || *out.TakeoverGeneration != 1 {
		t.Fatalf("output: %+v", out)
	}
	if n := miCount(t, f.owner, `SELECT count(*) FROM integration.operations WHERE action='meta.dm_send' AND request->>'conversation_id'=$1`, conv); n != 1 {
		t.Fatalf("operations=%d, want 1", n)
	}
	after := e.state(t, conv)
	if after.Mode != "human" || after.Gen != 1 || after.Assignee == nil || *after.Assignee != e.h.actor || after.HumanUntil == nil {
		t.Fatalf("implicit takeover: %+v", after)
	}
	// The frozen request holds no PSID and no text (external-operation "no unredacted conversation").
	var request string
	var state string
	var secrets, outbound int64
	if err := f.owner.QueryRow(ctx, `SELECT request::text,state FROM integration.operations WHERE id=$1`, out.OperationID).Scan(&request, &state); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(request, psid) || strings.Contains(request, secretText) || state != "READY" || len(request) > 2048 {
		t.Fatalf("request leaks or is oversize: state=%s %s", state, request)
	}
	secrets = e.secretCount(t, out.OperationID)
	outbound = miCount(t, f.owner, `SELECT count(*) FROM inbox.outbound_messages WHERE operation_id=$1`, out.OperationID)
	if secrets != 1 || outbound != 1 {
		t.Fatalf("secrets=%d outbound=%d", secrets, outbound)
	}

	// Dispatch: RESPONSE, no tag, the buyer's PSID and the text reach Graph; Check made no Graph call of its own.
	e.run(t, out.OperationID)
	e.awaitOp(t, out.OperationID, "SUCCEEDED", 10*time.Second, "completed")
	reqs := e.g.all()
	if len(reqs) != 1 || reqs[0].path != "/v99.0/"+e.pageAsset+"/messages" {
		t.Fatalf("graph traffic: %+v", reqs)
	}
	body := reqs[0].body
	rcp, _ := body["recipient"].(map[string]any)
	msg, _ := body["message"].(map[string]any)
	if body["messaging_type"] != "RESPONSE" || rcp["id"] != psid || msg["text"] != secretText {
		t.Fatalf("DM body: %v", body)
	}
	for _, banned := range []string{"tag", "message_tag", "update", "HUMAN_AGENT", "ACCOUNT_UPDATE"} {
		if strings.Contains(reqs[0].raw, banned) {
			t.Fatalf("message tag material %q in request body: %s", banned, reqs[0].raw)
		}
	}
	// Finish: dispatch copy wiped, last_outbound_at advanced.
	if e.secretCount(t, out.OperationID) != 0 {
		t.Fatal("send secret survived SUCCEEDED")
	}
	if s := e.state(t, conv); s.LastOut == nil {
		t.Fatal("last_outbound_at not advanced")
	}

	// window − 4 min: refused at plan (window − 5 min is the cut).
	conv2 := e.postDM(t, mciDigits(15), "second buyer", time.Now())
	e.setInbound(t, conv2, time.Now().Add(-(24*time.Hour - 4*time.Minute)))
	if _, err := e.sendDM(conv2, "x", 0); planCode(err) != "window_closed" {
		t.Fatalf("window-4min: %v", err)
	}
	// Window closing between plan and Check -> BLOCKED_POLICY with zero HTTP.
	conv3 := e.postDM(t, mciDigits(15), "third buyer", time.Now())
	e.setInbound(t, conv3, time.Now().Add(-(24*time.Hour - 6*time.Minute)))
	closing, err := e.sendDM(conv3, "closing soon", 0)
	if err != nil {
		t.Fatalf("window-6min must plan: %v", err)
	}
	before := e.g.count()
	e.setInbound(t, conv3, time.Now().Add(-25*time.Hour))
	e.run(t, closing.OperationID)
	code, _ := e.awaitOp(t, closing.OperationID, "BLOCKED_POLICY", 10*time.Second, "completed")
	if code != "window_closed" || e.g.count() != before {
		t.Fatalf("closing window: code=%s graph calls %d->%d", code, before, e.g.count())
	}
	if e.secretCount(t, closing.OperationID) != 0 {
		t.Fatal("send secret survived BLOCKED_POLICY")
	}

	// Per-platform text limits (planner: 422 invalid_text {max}).
	var se *inbox.SendError
	if _, err := e.sendDM(conv, strings.Repeat("好", 2001), 1); !errors.As(err, &se) || se.Code != "invalid_text" || se.Max != 2000 {
		t.Fatalf("Messenger limit: %v", err)
	}
	igConv := e.postIGDM(t, mciDigits(15), "ig hi", time.Now())
	if _, err := e.sendDM(igConv, strings.Repeat("好", 334), 0); !errors.As(err, &se) || se.Max != 1000 {
		t.Fatalf("Instagram byte limit: %v", err)
	}
	if _, err := e.sendDM(igConv, strings.Repeat("好", 333), 0); err != nil {
		t.Fatalf("333 CJK chars on Instagram: %v", err)
	}

	// duplicate_recent: the same body to the same conversation inside 30 s under another key.
	if _, err := e.sendDM(conv, secretText, 1); planCode(err) != "duplicate_recent" {
		t.Fatalf("duplicate_recent: %v", err)
	}
	// The same body but a stale generation (another tab) is takeover_changed before anything is planned.
	if _, err := e.sendDM(conv, "another body", 0); planCode(err) != "takeover_changed" {
		t.Fatalf("stale generation: %v", err)
	}
	// Rate cap: <= 60 human sends per store per minute.
	t.Cleanup(func() { mustExec(t, f.owner, `DELETE FROM integration.operations WHERE semantic_key LIKE 'mdm:rate%'`) })
	for i := 0; i < 60; i++ {
		mustExec(t, f.owner, `INSERT INTO integration.operations(tenant_id,store_id,id,principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,semantic_key,request_hash,request,job_id)
			SELECT tenant_id,store_id,gen_random_uuid(),principal_id,binding_id,binding_version,provider,external_asset_id,purpose,action,'mdm:rate'||$2::text||$3::text,request_hash,request,job_id
			FROM integration.operations WHERE id=$1`, out.OperationID, mciDigits(8), fmt.Sprint(i))
	}
	if _, err := e.sendDM(conv, "rate limited body", 1); planCode(err) != "rate_limited" {
		t.Fatalf("rate cap: %v", err)
	}
}

// 400 code 100 without a message_id is a request Meta provably did not accept: FAILED_FINAL invalid_request, secret wiped.
func TestLiveConsoleSendLCN06GraphInvalidRequest(t *testing.T) {
	e := lbSetup(t)
	conv := e.postDM(t, mciDigits(15), "hi", time.Now())
	out, err := e.sendDM(conv, "will be refused by Graph", 0)
	if err != nil {
		t.Fatal(err)
	}
	e.g.setMode("100")
	e.run(t, out.OperationID)
	code, _ := e.awaitOp(t, out.OperationID, "FAILED_FINAL", 10*time.Second, "completed")
	if code != "invalid_request" || e.g.count() != 1 || e.secretCount(t, out.OperationID) != 0 {
		t.Fatalf("code=%s calls=%d secrets=%d", code, e.g.count(), e.secretCount(t, out.OperationID))
	}
}

// Echoes, reactions, read receipts and delivery units never reach social.messages, so they never open or extend the window.
func TestLiveConsoleSendLCN06NonMessageUnitsNeverOpenAWindow(t *testing.T) {
	e := lbSetup(t)
	psid := mciDigits(15)
	// Post the non-opening units FIRST, then one real inbound: only that one projects.
	at := time.Now().UnixMilli()
	for name, unit := range map[string]string{
		"echo":     fmt.Sprintf(`{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"message":{"mid":"m.echo.%s","is_echo":true,"text":"echo"}}`, e.pageAsset, psid, at, mciDigits(6)),
		"reaction": fmt.Sprintf(`{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"reaction":{"mid":"m.x","action":"react","reaction":"love"}}`, psid, e.pageAsset, at),
		"read":     fmt.Sprintf(`{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"read":{"watermark":%d}}`, psid, e.pageAsset, at, at),
		"delivery": fmt.Sprintf(`{"sender":{"id":%q},"recipient":{"id":%q},"timestamp":%d,"delivery":{"mids":["m.y"],"watermark":%d}}`, psid, e.pageAsset, at, at),
	} {
		raw := []byte(fmt.Sprintf(`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[%s]}]}`, e.pageAsset, unit))
		if code, body := miPost(t, e.page, raw); code != 200 {
			t.Fatalf("%s unit refused at ingress: %d %s", name, code, body)
		}
	}
	conv := e.postDM(t, psid, "the one real message", time.Now().Add(-2*time.Hour))
	st := e.state(t, conv)
	if st.LastInbound == nil || time.Since(*st.LastInbound) < 100*time.Minute || time.Since(*st.LastInbound) > 140*time.Minute {
		t.Fatalf("window was moved by a non-message unit: last_inbound=%v", st.LastInbound)
	}
	if n := miCount(t, e.h.f.owner, `SELECT count(*) FROM social.messages WHERE conversation_id=$1`, conv); n != 1 {
		t.Fatalf("social.messages rows=%d, want only the real message", n)
	}
	// A message timestamped more than 5 minutes ahead is clamped to server time.
	conv2 := e.postDM(t, mciDigits(15), "from the future", time.Now().Add(6*time.Hour))
	if s := e.state(t, conv2); s.LastInbound == nil || s.LastInbound.After(time.Now().Add(time.Minute)) {
		t.Fatalf("future occurred_at not clamped: %v", s.LastInbound)
	}
}
