package foundation_test

// R11 independent contract tests: REAL_PG + local MOCK only. Synthetic owner
// writes prepare task-scoped clock/history fixtures; no product function or
// acceptance threshold is replaced. Root must independently rerun after merge.
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
	"livecommerce/tests/ads/fakegraph"
)

func atsElapsed(t *testing.T, x *atsEnv, op string) {
	t.Helper()
	// Disclosed clock fixture: only this completed synthetic operation ages.
	if n := miCount(t, x.e.h.f.owner, `SELECT count(*) FROM integration.operations WHERE id=$1 AND state='SUCCEEDED'`, op); n != 1 {
		t.Fatal("cooldown fixture requires real successful completion")
	}
	mustExec(t, x.e.h.f.owner, `UPDATE integration.operations SET updated_at=clock_timestamp()-interval '10 minutes 1 second' WHERE id=$1 AND tenant_id=$2 AND store_id=$3 AND state='SUCCEEDED'`, op, x.e.h.f.tenantA, x.e.h.f.storeA1)
}

func TestAdsAttributionR11AudienceBoundReplay(t *testing.T) {
	x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
	jobs := miCount(t, x.e.h.f.owner, `SELECT count(*) FROM river.river_job`)
	op := x.mustPlan()
	var wg sync.WaitGroup
	responses := make(chan adsResp, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); responses <- x.plan() }()
	}
	wg.Wait()
	close(responses)
	for r := range responses {
		if r.Status != 200 || r.JSON["operation_id"] != op {
			t.Errorf("fresh idempotency keys must replay in-flight op: status=%d op=%v", r.Status, r.JSON["operation_id"])
		}
	}
	if got := miCount(t, x.e.h.f.owner, `SELECT count(*) FROM integration.operations WHERE tenant_id=$1 AND store_id=$2 AND action='meta.live_insights'`, x.e.h.f.tenantA, x.e.h.f.storeA1); got != 1 {
		t.Errorf("one in-flight operation required, got=%d", got)
	}
	if got := miCount(t, x.e.h.f.owner, `SELECT count(*) FROM river.river_job`); got != jobs+1 {
		t.Errorf("replay must not commit orphan jobs: before=%d after=%d", jobs, got)
	}
	if t.Failed() {
		return
	}
	// Bound is the video, not the session. Register a second scoped session
	// with the same public Page post through the real source API.
	prior := x.e.session
	x.e.session = x.e.h.draft(t, x.e.h.f.storeA1)
	x.e.mustSource(t, "page", x.e.pageAsset, x.e.postID, false)
	r := x.plan()
	x.e.session = prior
	if r.Status != 200 || r.JSON["operation_id"] != op {
		t.Fatalf("same video in another session must replay: %d %v", r.Status, r.JSON["operation_id"])
	}
	t06StartDispatcher(t, x.pool, x.queue, []core.DispatchRoute{x.route}, mciDispatchOptions())
	x.run(op)
	x.e.awaitOp(t, op, "SUCCEEDED", 8*time.Second, "completed")
	jobs = miCount(t, x.e.h.f.owner, `SELECT count(*) FROM river.river_job`)
	r = x.plan()
	if r.Status != 200 || r.JSON["operation_id"] != op || r.JSON["state"] != "SUCCEEDED" {
		t.Fatalf("final operation must replay during cooldown: %d %v", r.Status, r.JSON)
	}
	if miCount(t, x.e.h.f.owner, `SELECT count(*) FROM river.river_job`) != jobs || x.g.count() != 2 {
		t.Fatal("final replay queued a job or called Graph")
	}
	atsElapsed(t, x, op)
	next := x.mustPlan()
	if next == op {
		t.Fatal("elapsed cooldown must allow a new operation")
	}
	if miCount(t, x.e.h.f.owner, `SELECT count(*) FROM river.river_job`) != jobs+1 {
		t.Fatal("new post-cooldown operation must have exactly one job")
	}
}

func TestAdsAttributionR11ClickLiveWindow(t *testing.T) {
	e := newAdsEnv(t, adsOpts{})
	d := e.newDraft(adsDraftIn{})
	match := func(at time.Time) int {
		return int(miCount(t, e.f.owner, `SELECT count(*) FROM ads.attribution_match($1,$2,$3,NULL,$4)`, e.f.tenantA, e.store, d, at))
	}
	if n := match(time.Now()); n != 0 {
		t.Errorf("never launched public lc_ad credited=%d", n)
	}
	e.mustApprove(d)
	e.mustPublish(d)
	e.driveTo(d, "activate", 1)
	e.ownerReplica(`UPDATE ads.campaign_drafts SET starts_at=clock_timestamp()-interval '1 hour',ends_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, d)
	at := atAfterActivationSecond(t, e, d)
	if n := match(at); n != 1 {
		t.Errorf("during real successful activation credited=%d", n)
	}
	var start, end time.Time
	if err := e.f.owner.QueryRow(e.ctx, `SELECT starts_at,ends_at FROM ads.campaign_drafts WHERE id=$1`, d).Scan(&start, &end); err != nil {
		t.Fatal(err)
	}
	if match(start.Add(-time.Nanosecond)) != 0 || match(end) != 0 {
		t.Error("click outside starts/ends credited")
	}
	if pause := e.pause(d); pause.Status != 200 {
		t.Fatal("pause fixture failed")
	}
	e.driveTo(d, "pause", 1)
	var paused time.Time
	if err := e.f.owner.QueryRow(e.ctx, `SELECT max(o.updated_at) FROM ads.remote_objects r JOIN integration.operations o ON o.id=r.operation_id WHERE r.draft_id=$1 AND r.kind='pause' AND o.state='SUCCEEDED'`, d).Scan(&paused); err != nil {
		t.Fatal(err)
	}
	if n := match(paused); n != 0 {
		t.Errorf("copied ended lc_ad credited=%d", n)
	}
	if n := match(at); n != 1 {
		t.Error("historical valid click lost after end")
	}
}

// r11DraftCopies prepares never-published drafts only. All generated IDs are
// kept for exact fixture cleanup; real activation and Insights are untouched.
func r11DraftCopies(t *testing.T, e *adsEnv, template string, n int) []string {
	t.Helper()
	rows, err := e.f.owner.Query(e.ctx, `INSERT INTO ads.campaign_drafts SELECT (jsonb_populate_record(NULL::ads.campaign_drafts,to_jsonb(d)||jsonb_build_object('id',gen_random_uuid(),'publish_attempt',0,'created_at',d.created_at-interval '10 seconds'-g*interval '1 microsecond'))).* FROM ads.campaign_drafts d CROSS JOIN generate_series(1,$2::int) g WHERE d.id=$1 RETURNING id::text`, template, n)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(ids) != n {
		t.Fatalf("draft fixture count=%d err=%v", len(ids), err)
	}
	t.Cleanup(func() {
		mustExec(t, e.f.owner, `DELETE FROM ads.campaign_drafts WHERE tenant_id=$1 AND store_id=$2 AND id=ANY($3::uuid[])`, e.f.tenantA, e.store, ids)
	})
	return ids
}

func TestAdsAttributionR11UnknownDraft(t *testing.T) {
	x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
	x.a = newAdsEnv(t, adsOpts{fx: x.e.h.f, noWorker: true})
	d := x.a.newDraft(adsDraftIn{})
	r := x.a.api("GET", "/attribution?from="+taipeiDay(0)+"&to="+taipeiDay(0), x.e.h.token, nil, nil)
	if r.Status != 200 {
		t.Fatalf("report status=%d", r.Status)
	}
	if r.JSON["truncated"] != false {
		t.Errorf("uncapped report must explicitly return truncated:false")
	}
	foundDraft, foundSession := false, false
	for _, raw := range r.JSON["drafts"].([]any) {
		v := raw.(map[string]any)
		if v["draft_id"] == d {
			foundDraft = true
			for _, key := range []string{"spend_minor", "roas"} {
				value, present := v[key]
				if !present || value != nil {
					t.Errorf("no insights draft %s must be explicit null: present=%v value=%v", key, present, value)
				}
			}
		}
	}
	for _, raw := range r.JSON["sessions"].([]any) {
		v := raw.(map[string]any)
		if v["session_id"] == x.e.session {
			foundSession = true
			for _, key := range []string{"spend_minor", "roas"} {
				value, present := v[key]
				if !present || value != nil {
					t.Errorf("no insights session %s must be explicit null: present=%v value=%v", key, present, value)
				}
			}
		}
	}
	if !foundDraft || !foundSession {
		t.Fatal("unknown fixture draft or session not projected")
	}
}

func TestAdsAttributionR11ReportCapAndDeadline(t *testing.T) {
	b := bcSetup(t)
	res, err := b.begin(t04Key("r11-scale-seed"))
	if err != nil {
		t.Fatal(err)
	}
	e := newAdsEnv(t, adsOpts{noWorker: true}) // private store, unaffected by prior tests' retained drafts
	d := e.newDraft(adsDraftIn{})
	drafts := append([]string{d}, r11DraftCopies(t, e, d, 99)...)
	// Synthetic COLLECTED COD read-projection rows, not financial acceptance.
	// Replica applies only inside this owner transaction (same existing scale
	// fixture pattern); no runtime role, grant, function or gate is modified.
	tx, err := b.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(context.Background(), `SET LOCAL session_replication_role=replica`); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(context.Background(), `INSERT INTO checkout.orders SELECT (jsonb_populate_record(NULL::checkout.orders,to_jsonb(o)||jsonb_build_object('id',gen_random_uuid(),'store_id',$2::uuid,'commercial_state','CONFIRMED','payment_mode','cash_on_delivery','collection_state','COLLECTED','collected_at',clock_timestamp(),'cod_surcharge_minor',0,'cod_carrier','black_cat','cart_version',10000+g,'job_id',910000000+g))).* FROM checkout.orders o CROSS JOIN generate_series(1,10000) g WHERE o.id=$1 RETURNING id::text`, res.OrderID, e.store)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	allOrders := ids
	if _, err = tx.Exec(context.Background(), `INSERT INTO orders.order_attribution(order_id,tenant_id,store_id,path,draft_id,clicked_at) SELECT o.id,o.tenant_id,o.store_id,'ad_click',($2::uuid[])[1+((u.n-1)%100)::int],o.created_at FROM unnest($1::uuid[]) WITH ORDINALITY u(id,n) JOIN checkout.orders o ON o.id=u.id`, allOrders, drafts); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		mustExec(t, b.f.owner, `DELETE FROM orders.order_attribution WHERE tenant_id=$1 AND store_id=$2 AND order_id=ANY($3::uuid[])`, e.tenant, e.store, allOrders)
		mustExec(t, b.f.owner, `DELETE FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=ANY($3::uuid[])`, e.tenant, e.store, ids)
		mustExec(t, b.f.owner, `VACUUM checkout.orders`)
	})
	if n := miCount(t, b.f.owner, `SELECT count(*) FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2`, e.tenant, e.store); n != 10000 {
		t.Fatalf("scale fixture orders=%d", n)
	}
	for _, n := range []int{100, 101} {
		if n == 101 {
			r11DraftCopies(t, e, d, 1)
		}
		started := time.Now()
		r := e.api("GET", "/attribution?from="+taipeiDay(0)+"&to="+taipeiDay(0), e.token, nil, nil)
		elapsed := time.Since(started)
		t.Logf("REAL_PG MOCK synthetic collected COD orders=10000 drafts=%d API elapsed=%s status=%d bytes=%d", n, elapsed, r.Status, len(r.Raw))
		if r.Status != 200 || elapsed >= 5*time.Second {
			t.Errorf("report 5-second API deadline violated: status=%d elapsed=%s", r.Status, elapsed)
			continue
		}
		if len(r.JSON["drafts"].([]any)) != 100 || r.JSON["truncated"] != (n > 100) {
			t.Errorf("cap/flag drafts=%d truncated=%v", len(r.JSON["drafts"].([]any)), r.JSON["truncated"])
		}
		var paid float64
		for _, raw := range r.JSON["drafts"].([]any) {
			for _, p := range raw.(map[string]any)["orders"].([]any) {
				paid += p.(map[string]any)["orders"].(float64)
			}
		}
		wantPaid := float64(10000)
		if n == 101 {
			wantPaid = 9900
		} // the capped oldest draft owns 100 disclosed fixture orders
		if paid != wantPaid {
			t.Errorf("capped paid aggregate rows=%v want=%v", paid, wantPaid)
		}
	}
	// Runtime authentication and definer, with the same request deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var raw json.RawMessage
	started := time.Now()
	err = platform.WithScope(ctx, e.f.runtime, e.token, e.store, "ads:read", func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(ctx, `SELECT ads.attribution_report($1,$2,$3::date,$3::date)`, tokenHash(e.token), e.store, taipeiDay(0)).Scan(&raw)
	})
	t.Logf("REAL_PG direct runtime SQL elapsed=%s bytes=%d error=%v", time.Since(started), len(raw), err)
	if err != nil {
		t.Fatal(fmt.Sprintf("runtime SQL deadline: %v", err))
	}
}

func TestAdsAttributionR11SessionCap(t *testing.T) {
	x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
	for _, n := range []int{100, 101} {
		copies := 99
		if n == 101 {
			copies = 1
		}
		rows, err := x.e.h.f.owner.Query(context.Background(), `INSERT INTO live.sessions SELECT (jsonb_populate_record(NULL::live.sessions,to_jsonb(s)||jsonb_build_object('id',gen_random_uuid()))).* FROM live.sessions s CROSS JOIN generate_series(1,$2::int) g WHERE s.id=$1 RETURNING id::text`, x.e.session, copies)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			mustExec(t, x.e.h.f.owner, `DELETE FROM live.sessions WHERE tenant_id=$1 AND store_id=$2 AND id=ANY($3::uuid[])`, x.e.h.f.tenantA, x.e.h.f.storeA1, ids)
		})
		r := x.a.api("GET", "/attribution?from="+taipeiDay(0)+"&to="+taipeiDay(0), x.e.h.token, nil, nil)
		if r.Status != 200 {
			t.Fatalf("session cap report status=%d", r.Status)
		}
		if len(r.JSON["sessions"].([]any)) != 100 || r.JSON["truncated"] != (n > 100) {
			t.Errorf("session cap n=%d rows=%d truncated=%v", n, len(r.JSON["sessions"].([]any)), r.JSON["truncated"])
		}
	}
}

func TestAdsAttributionR11OptionalBreakdownAndNull(t *testing.T) {
	e := newAdsEnv(t, adsOpts{})
	d, camp := e.activeDraft(300000)
	day := taipeiDay(0)
	e.g.SetInsights(camp, day, fakegraph.Insights{Spend: "12.30", Impressions: "1000", Clicks: "9"})
	e.g.SetBreakdowns(camp, day, "age,gender", []map[string]any{{"date_start": day, "date_stop": day, "age": "<malformed>", "gender": "female", "spend": "12.30"}})
	e.g.SetBreakdowns(camp, day, "hourly_stats_aggregated_by_advertiser_time_zone", []map[string]any{{"date_start": day, "date_stop": day, "spend": "12.30", "hourly_stats_aggregated_by_advertiser_time_zone": "13:00:00 - 13:59:59"}})
	e.sweep("insights")
	e.settle()
	e.sweep("advance")
	if n := e.count(`SELECT count(*) FROM ads.insights_daily WHERE tenant_id=$1 AND store_id=$2 AND draft_id=$3 AND day=$4 AND spend_minor=1230`, e.tenant, e.store, d, day); n != 1 {
		t.Fatalf("optional breakdown failure blocked genuine D7 daily row: rows=%d", n)
	}
	r := e.api("GET", "/attribution?from="+day+"&to="+day, e.token, nil, nil)
	if r.Status != 200 {
		t.Fatalf("nullable report status=%d", r.Status)
	}
	foundDraft, foundHourly := false, false
	for _, raw := range r.JSON["drafts"].([]any) {
		v := raw.(map[string]any)
		if v["draft_id"] != d {
			continue
		}
		foundDraft = true
		missing, ok := v["breakdowns_unavailable"].([]any)
		if !ok || len(missing) != 1 {
			t.Fatalf("failed optional age_gender must be marked unavailable=%v", v["breakdowns_unavailable"])
		}
		m := missing[0].(map[string]any)
		dims := m["dimensions"].([]any)
		if m["day"] != day || len(dims) != 1 || dims[0] != "age_gender" {
			t.Fatalf("unavailable dimension lost: %v", m)
		}
		for _, row := range v["breakdowns"].([]any) {
			b := row.(map[string]any)
			if b["dimension"] != "hourly" {
				continue
			}
			foundHourly = true
			atNum(t, b, "spend_minor", 1230)
			for _, key := range []string{"reach", "impressions", "clicks", "engagements", "comments", "purchases", "purchase_value_minor"} {
				value, present := b[key]
				if !present || value != nil {
					t.Errorf("omitted metric %s must be explicit null: present=%v value=%v", key, present, value)
				}
			}
		}
	}
	if !foundDraft || !foundHourly {
		t.Fatal("successful hourly snapshot lost with optional dimension failure")
	}
}

func TestAdsAttributionR11PurgeNoStarvation(t *testing.T) {
	c := newCapiEnv(t, adsOpts{noWorker: true})
	if r := c.setCapi(true); r.Status != 200 {
		t.Fatal("CAPI enable fixture failed")
	}
	if r := c.consent(true); r.status != 200 {
		t.Fatal("eligible buyer consent fixture failed")
	}
	withdrawn := mustIssue(t, c.p.cqHarness.service, c.store)
	for _, grant := range []bool{true, false} {
		r := c.bh.request(t, "PUT", "/v1/buyer/consents", withdrawn.Token, t04Key("r11-withdraw"), map[string]any{"purpose": "ads_personalization", "channel": "meta_ads", "granted": grant, "context": "settings"}, func(req *http.Request) { req.Header.Set("User-Agent", c.ua) })
		if r.status != 200 {
			t.Fatalf("actual distinct buyer consent/withdrawal=%d", r.status)
		}
	}
	ctx := context.Background()
	tx, err := c.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL session_replication_role=replica`); err != nil {
		t.Fatal(err)
	}
	// Disclosed historical read fixture: recreate a missed withdrawal batch.
	// First 1000 rows are still eligible; row 1001 belongs to a distinct buyer
	// who withdrew through the real route. Replica is transaction-local only.
	rows, err := tx.Query(ctx, `INSERT INTO checkout.orders SELECT (jsonb_populate_record(NULL::checkout.orders,to_jsonb(o)||jsonb_build_object('id',gen_random_uuid(),'cart_version',20000+g,'job_id',920000000+g,'owner_id',CASE WHEN g=1001 THEN $2::uuid ELSE o.owner_id END,'creator_session_id',CASE WHEN g=1001 THEN $3::uuid ELSE o.creator_session_id END))).* FROM checkout.orders o CROSS JOIN generate_series(1,1001) g WHERE o.id=$1 RETURNING id::text,owner_id::text`, c.p.result.OrderID, withdrawn.Scope.OwnerID, withdrawn.Scope.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	var target string
	for rows.Next() {
		var id, owner string
		if err = rows.Scan(&id, &owner); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		if owner == withdrawn.Scope.OwnerID {
			target = id
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(ids) != 1001 || target == "" {
		t.Fatalf("starvation fixture rows=%d error=%v", len(ids), err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO orders.order_attribution(order_id,tenant_id,store_id,fbc,fbp,client_ip,frozen_at) SELECT o.id,o.tenant_id,o.store_id,$2,$3,'192.0.2.8',clock_timestamp()-interval '1 minute'+u.n*interval '1 microsecond' FROM unnest($1::uuid[]) WITH ORDINALITY u(id,n) JOIN checkout.orders o ON o.id=u.id`, ids, atFBC, atFBP); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		mustExec(t, c.f.owner, `DELETE FROM orders.order_attribution WHERE tenant_id=$1 AND store_id=$2 AND order_id=ANY($3::uuid[])`, c.tenant, c.store, ids)
		mustExec(t, c.f.owner, `DELETE FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=ANY($3::uuid[])`, c.tenant, c.store, ids)
		mustExec(t, c.f.owner, `VACUUM checkout.orders`)
	})
	if n := c.count(`SELECT count(*) FROM orders.order_attribution a JOIN checkout.orders o ON o.id=a.order_id WHERE a.order_id=ANY($1::uuid[]) AND ads.capi_ip_needed(a.tenant_id,a.store_id,o.owner_id,NULL)`, ids); n != 1000 {
		t.Fatalf("expected exactly 1000 eligible blockers, got=%d", n)
	}
	if _, err = c.workerPool.Exec(ctx, `SELECT ads.plan_capi_purge()`); err != nil {
		t.Fatal(err)
	}
	if n := c.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND fbc IS NULL AND fbp IS NULL AND client_ip IS NULL`, target); n != 1 {
		t.Error("withdrawn row starved behind 1000 still-eligible signals")
	}
	if n := c.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=ANY($1::uuid[]) AND order_id<>$2 AND fbc=$3 AND fbp=$4 AND client_ip IS NOT NULL`, ids, target, atFBC, atFBP); n != 1000 {
		t.Errorf("still-needed send-once signals removed: retained=%d", n)
	}
}

func TestAdsAttributionR11AgedRetryAndFinalDaily(t *testing.T) {
	e := newAdsEnv(t, adsOpts{})
	d, camp := e.activeDraft(300000)
	today, old := taipeiDay(0), taipeiDay(-4)
	e.g.SetInsights(camp, today, fakegraph.Insights{Spend: "12.30", Impressions: "1000", Clicks: "9"})
	e.sweep("insights")
	e.settle()
	e.sweep("advance")
	var op string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT source_operation_id::text FROM ads.insights_daily WHERE tenant_id=$1 AND store_id=$2 AND draft_id=$3 AND day=$4`, e.tenant, e.store, d, today).Scan(&op); err != nil {
		t.Fatal(err)
	}
	// Scoped historical clock fixture: a final D7 row remains immutable, but
	// its optional failure still has a 24-hour retry window outside the normal
	// three-day D7 lookback. No live day generator is patched.
	e.ownerReplica(`UPDATE ads.campaign_drafts SET starts_at=clock_timestamp()-interval '10 days',ends_at=clock_timestamp()+interval '1 hour' WHERE id=$1 AND tenant_id=$2 AND store_id=$3`, d, e.tenant, e.store)
	mustExec(t, e.f.owner, `INSERT INTO ads.insights_daily SELECT (jsonb_populate_record(NULL::ads.insights_daily,to_jsonb(i)||jsonb_build_object('day',$2::date,'final',true))).* FROM ads.insights_daily i WHERE i.draft_id=$1 AND i.day=$3::date`, d, old, today)
	for offset := 4; offset <= 9; offset++ {
		mustExec(t, e.f.owner, `INSERT INTO ads.insights_breakdown_status(tenant_id,store_id,draft_id,day,unavailable,source_operation_id,fetched_at,retry_until) VALUES($1,$2,$3,$4::date,ARRAY['age_gender'],$5,clock_timestamp(),clock_timestamp()+interval '24 hours')`, e.tenant, e.store, d, taipeiDay(-offset), op)
	}
	t.Cleanup(func() {
		mustExec(t, e.f.owner, `DELETE FROM ads.insights_breakdown_status WHERE tenant_id=$1 AND store_id=$2 AND draft_id=$3`, e.tenant, e.store, d)
	})
	before := lcStrings(t, e.f.owner, `SELECT to_jsonb(i)::text FROM ads.insights_daily i WHERE draft_id=$1 AND day=$2::date`, d, old)
	var expires time.Time
	if err := e.f.owner.QueryRow(e.ctx, `SELECT retry_until FROM ads.insights_breakdown_status WHERE draft_id=$1 AND day=$2::date`, d, old).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	var days []time.Time
	if err := e.workerPool.QueryRow(e.ctx, `SELECT ads.insights_days($1)`, d).Scan(&days); err != nil {
		t.Fatal(err)
	}
	failedDays := 0
	found := false
	for _, day := range days {
		label := day.Format("2006-01-02")
		if label == old {
			found = true
		}
		if label < today && label <= taipeiDay(-4) {
			failedDays++
		}
	}
	if !found || failedDays != 4 {
		t.Fatalf("bounded aged retry days=%v found old=%v count=%d", days, found, failedDays)
	}
	if n := miCount(t, e.workerPool, `SELECT count(*) FROM ads.insights_candidates(500) x WHERE x=$1::uuid`, d); n != 1 {
		t.Fatal("aged retry draft missing from actual worker candidate reader")
	}
	// Fail again at the old date. The first failure's retry deadline must not
	// slide forward forever; the final D7 snapshot cannot be overwritten.
	e.g.SetInsights(camp, old, fakegraph.Insights{Spend: "99.99", Impressions: "9999", Clicks: "99"})
	e.g.SetBreakdowns(camp, old, "age,gender", []map[string]any{{"date_start": old, "date_stop": old, "age": "<malformed>", "gender": "female", "spend": "99.99"}})
	e.sweep("insights")
	e.settle()
	e.sweep("advance")
	if n := e.count(`SELECT count(*) FROM ads.insight_reads WHERE draft_id=$1 AND day=$2::date`, d, old); n != 1 {
		t.Fatalf("next sweep did not really dispatch aged retry: %d", n)
	}
	lcSameSet(t, "final daily immutable through optional retry", lcStrings(t, e.f.owner, `SELECT to_jsonb(i)::text FROM ads.insights_daily i WHERE draft_id=$1 AND day=$2::date`, d, old), before)
	var after time.Time
	if err := e.f.owner.QueryRow(e.ctx, `SELECT retry_until FROM ads.insights_breakdown_status WHERE draft_id=$1 AND day=$2::date`, d, old).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !after.Equal(expires) {
		t.Fatal("repeated optional failure extended the 24-hour lifecycle")
	}
	mustExec(t, e.f.owner, `UPDATE ads.insights_breakdown_status SET fetched_at=clock_timestamp()-interval '24 hours 1 second',retry_until=clock_timestamp()-interval '1 second' WHERE tenant_id=$1 AND store_id=$2 AND draft_id=$3`, e.tenant, e.store, d)
	if err := e.workerPool.QueryRow(e.ctx, `SELECT ads.insights_days($1)`, d).Scan(&days); err != nil {
		t.Fatal(err)
	}
	for _, day := range days {
		if day.Format("2006-01-02") <= old {
			t.Fatalf("expired aged retry remained queued: %s", day)
		}
	}
}
