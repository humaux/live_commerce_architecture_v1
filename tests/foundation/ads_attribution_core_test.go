package foundation_test

// Independent Amendment 1 gates. All owner writes below prepare synthetic fixtures or
// inject transaction faults; Begin, capture, consent and dispatch use their real paths.
import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/attribution"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/claims"
	"livecommerce/internal/fulfillment"
	integration "livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
	"livecommerce/tests/ads/fakegraph"
)

const atFBC = "fb.1.1790000000000.synthetic-click"
const atFBP = "fb.1.1790000000000.123456789"

func atTouch(d string, age time.Duration) *checkout.AdTouch {
	fbc := atFBC
	return &checkout.AdTouch{DraftID: d, ClickedAt: time.Now().Add(-age), FBC: &fbc, FBP: atFBP}
}

// A genuine signed Meta comment flows through the intake worker, claim redemption
// and Begin. The offer has NO live price: attribution is provenance, not a discount.
func TestAdsAttributionAT3ExactComment(t *testing.T) {
	for _, which := range []string{"one boost", "simultaneous boosts", "outside window", "other post in same session", "click beats post", "line updated after redeem"} {
		t.Run(which, func(t *testing.T) {
			m := mciSetup(t, mciOpts{})
			h := m.h
			e := newAdsEnv(t, adsOpts{fx: h.f})
			// Synthetic asset registration: ads identity and comment intake refer to the same Page.
			mustExec(t, h.f.owner, `UPDATE integration.bindings SET external_asset_id=$2 WHERE id=$1`, e.idBinding, m.pageAsset)
			e.pageAsset = m.pageAsset
			d := e.newDraft(adsDraftIn{Source: m.postID})
			e.mustApprove(d)
			e.mustPublish(d)
			e.driveTo(d, "activate", 1)
			e.ownerReplica(`UPDATE ads.campaign_drafts SET starts_at=clock_timestamp()-interval '1 hour',ends_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, d)
			at := time.Now()
			post := m.postID
			if which == "simultaneous boosts" {
				d2 := e.newDraft(adsDraftIn{Source: post})
				e.mustApprove(d2)
				e.mustPublish(d2)
				e.driveTo(d2, "activate", 1)
				e.ownerReplica(`UPDATE ads.campaign_drafts SET starts_at=clock_timestamp()-interval '1 hour',ends_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, d2)
				at = time.Now()
			}
			if which == "outside window" {
				e.ownerReplica(`UPDATE ads.campaign_drafts SET starts_at=clock_timestamp()-interval '3 hours',ends_at=clock_timestamp()-interval '1 hour' WHERE id=$1`, d)
			}
			if which == "other post in same session" {
				post = m.pageAsset + "_" + mciDigits(10)
				m.mustSource(t, "page", m.pageAsset, post, false)
			}
			sent := m.postFBTo(t, post, "", "", "A1+2", &at, nil, true)
			m.apply(t)
			intake := m.mustIntake(t, "page", m.pageAsset, sent.comment)
			if intake.State != "APPLIED" {
				t.Fatalf("comment not applied: %+v", intake)
			}
			event := lcEvent(t, h.f, intake.AppliedEvent)
			if event.outcome != "ACCEPTED" {
				t.Fatalf("claim outcome=%s", event.outcome)
			}
			h.closeWindow(t, m.session)
			link := h.link(t, m.session, event.bundle, 0, false)
			cap := mustIssue(t, h.service, h.f.storeA1)
			preview, err := h.preview(cap, link.Token)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = h.redeem(cap, t04Key("at-redeem"), link.Token, preview.BundleVersion); err != nil {
				t.Fatal(err)
			}
			if which == "line updated after redeem" {
				// A later comment changes the mutable claim line and is on a different
				// post. The cart still carries the exact earlier accepted version.
				h.open(t, m.session, claims.MatchExact)
				laterPost := m.pageAsset + "_" + mciDigits(10)
				m.mustSource(t, "page", m.pageAsset, laterPost, false)
				laterAt := mciSoon() // A timestamp-less webhook is deliberately not claim-eligible.
				later := m.postFBTo(t, laterPost, "", sent.from, "A1+3", &laterAt, nil, true)
				m.apply(t)
				if state := m.mustIntake(t, "page", m.pageAsset, later.comment).State; state != "APPLIED" {
					t.Fatalf("later comment=%s", state)
				}
				h.closeWindow(t, m.session)
			}
			cq := h.cqHarness
			cq.cap = cap
			delivery := fulfillment.ServiceInput{MarketID: cq.market.ID, Country: "TW", Code: "at_home_" + t04Tag(), PolicyVersion: 1, NameHans: "测试配送", NameHant: "測試配送", NameEN: "Synthetic delivery", DeliveryKind: "home", Mode: "MANUAL", Enabled: true, Visible: true}
			dsPolicy(t, cq, delivery, 0, 50, true)
			if _, err = dsSet(cq, t04Key("at-service"), delivery); err != nil {
				t.Fatal(err)
			}
			alloc := fulfillment.AllocationInput{MarketID: cq.market.ID, Country: "TW", Code: delivery.Code, ExpectedServiceVersion: 1, WarehouseIDs: []string{cq.stock.warehouse.ID}}
			if _, err = daSet(cq, t04Key("at-allocation"), alloc); err != nil {
				t.Fatal(err)
			}
			cart := h.cartOf(t, cap)
			dest, err := bdSet(cq, t04Key("at-dest"), bdHome(cart))
			if err != nil {
				t.Fatal(err)
			}
			quote, err := cqBuyer(cq.a.runtime, cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
				return storefront.CreateQuote(ctx, tx, s, t04Key("at-quote"), storefront.QuoteInput{CartVersion: cart.Version, MarketID: cq.market.ID, Country: "TW", Method: "delivery:" + delivery.Code})
			})
			if err != nil {
				t.Fatal(err)
			}
			pool, err := platform.OpenCheckoutPool(context.Background(), bcRole(t, h.f, "commerce_checkout_runtime"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			input := checkout.Input{QuoteID: quote.ID, DestinationID: dest.ID, CartVersion: cart.Version, ServiceVersion: 1, AllocationVersion: 1}
			if which == "click beats post" {
				input.AdTouch = atTouch(e.newDraft(adsDraftIn{}), time.Minute)
			}
			pgLog := lcServerLog(t, h.f)
			result, err := bcService(t, pool).Begin(context.Background(), cap.Token, h.f.storeA1, t04Key("at-comment-begin"), input)
			if err != nil {
				for _, line := range strings.Split(pgLog(), "\n") {
					if strings.Contains(line, "ERROR:") || strings.Contains(line, "CONTEXT:") {
						t.Log(line)
					}
				}
				t.Fatal(err)
			}
			if n := miCount(t, h.f.owner, `SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1`, result.OrderID); n != 0 {
				t.Fatal("test accidentally depends on a live-price ledger")
			}
			if which == "outside window" || which == "other post in same session" {
				if n := miCount(t, h.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE order_id=$1`, result.OrderID); n != 0 {
					t.Fatal("unboosted exact comment incorrectly credited")
				}
				return
			}
			var path string
			var credited, postID *string
			if err = h.f.owner.QueryRow(h.ctx, `SELECT path,draft_id::text,post_id FROM orders.order_attribution WHERE order_id=$1`, result.OrderID).Scan(&path, &credited, &postID); err != nil {
				t.Fatal(err)
			}
			if which == "click beats post" {
				if path != "ad_click" || credited == nil || *credited != input.AdTouch.DraftID || postID != nil {
					t.Fatalf("click did not win: %s %v %v", path, credited, postID)
				}
				return
			}
			if path != "boosted_post" || postID == nil || *postID != post {
				t.Fatalf("wrong post: %s %v", path, postID)
			}
			if which == "simultaneous boosts" {
				if credited != nil {
					t.Fatal("ambiguous boost invented a draft")
				}
			} else if credited == nil || *credited != d {
				t.Fatalf("exact boost missing: %v", credited)
			}
		})
	}
}

func TestAdsAttributionAT8BreakdownDispatch(t *testing.T) {
	e := newAdsEnv(t, adsOpts{})
	d, camp := e.activeDraft(300000)
	day := taipeiDay(0)
	e.g.SetInsights(camp, day, fakegraph.Insights{Spend: "12.30", Impressions: "1000", Clicks: "9", PurchaseCount: "2", PurchaseValue: "45.90"})
	for wire, fields := range map[string]map[string]any{
		"age,gender": {"age": "25-34", "gender": "female"}, "region": {"region": "Taipei"},
		"publisher_platform,platform_position": {"publisher_platform": "facebook", "platform_position": "feed"},
		"device_platform":                      {"device_platform": "mobile"}, "hourly_stats_aggregated_by_advertiser_time_zone": {"hourly_stats_aggregated_by_advertiser_time_zone": "13:00:00 - 13:59:59"},
	} {
		fields["date_start"] = day
		fields["date_stop"] = day
		fields["spend"] = "1.23"
		fields["reach"] = "15"
		fields["impressions"] = "20"
		fields["clicks"] = "3"
		fields["actions"] = []any{map[string]any{"action_type": "post_engagement", "value": "7"}, map[string]any{"action_type": "comment", "value": "4"}, map[string]any{"action_type": "omni_purchase", "value": "2"}}
		fields["action_values"] = []any{map[string]any{"action_type": "omni_purchase", "value": "45.90"}}
		e.g.SetBreakdowns(camp, day, wire, []map[string]any{fields})
	}
	var mu sync.Mutex
	var probes []string
	e.onLoad = func(cl integration.SecretClaim) {
		var action string
		if err := e.f.owner.QueryRow(e.ctx, `SELECT action FROM integration.operations WHERE id=$1`, cl.OperationID).Scan(&action); err != nil || action != "meta.ads.read_insights" {
			return
		}
		wrong := append([]byte(nil), cl.LeaseToken...)
		wrong[0] ^= 0xff
		for _, p := range []struct {
			name  string
			gen   int64
			token []byte
		}{{"generation", cl.Generation + 1, cl.LeaseToken}, {"token", cl.Generation, wrong}} {
			_, err := e.workerPool.Exec(e.ctx, `SELECT ads.finish_insights_breakdowns($1,$2,$3,'dispatch',$4::jsonb)`, cl.OperationID, p.gen, p.token, `{"timezone_name":"Asia/Taipei","currency":"TWD","rows":[]}`)
			mu.Lock()
			probes = append(probes, p.name+":"+sqlState(err))
			mu.Unlock()
		}
	}
	e.sweep("insights")
	e.settle()
	e.onLoad = nil
	mu.Lock()
	gotProbes := append([]string(nil), probes...)
	mu.Unlock()
	if len(gotProbes) != 6 {
		t.Fatalf("lease probes=%v", gotProbes)
	}
	for _, p := range gotProbes {
		if !strings.HasSuffix(p, ":40001") {
			t.Fatalf("stale lease accepted: %s", p)
		}
	}
	want := []string{"age_gender/25-34/female", "region/Taipei", "placement/facebook/feed", "device/mobile", "hourly/13:00:00 - 13:59:59"}
	lcSameSet(t, "all five partitions", lcStrings(t, e.f.owner, `SELECT dimension||'/'||bucket FROM ads.insights_breakdowns WHERE draft_id=$1 AND day=$2`, d, day), want)
	if n := e.count(`SELECT count(*) FROM ads.insights_breakdowns WHERE draft_id=$1 AND day=$2 AND timezone_name='Asia/Taipei' AND currency='TWD' AND spend_minor=123 AND reach=15 AND impressions=20 AND clicks=3 AND engagements=7 AND comments=4 AND purchases=2 AND purchase_value_minor=4590`, d, day); n != 5 {
		t.Fatalf("wrong metrics on %d of five partitions", 5-n)
	}
	if n := e.count(`SELECT count(*) FROM ads.insights_breakdowns WHERE draft_id=$1 AND day=$2 AND dimension='hourly' AND hour_start=($2::date+time '13:00') AT TIME ZONE 'Asia/Taipei'`, d, day); n != 1 {
		t.Fatal("hour is not an absolute account-zone instant")
	}
	e.sweep("advance")
	full := e.daily(d)[day]
	if full.Spend != 1230 || full.Impressions != 1000 || full.Clicks != 9 || full.Purchases == nil || *full.Purchases != 2 {
		t.Fatalf("daily totals changed by partition data: %+v", full)
	}
	// Re-read same account-day with a replacement region and absent other buckets.
	e.ownerReplica(`UPDATE integration.operations SET semantic_key=regexp_replace(semantic_key,'[0-9]{10}$','2000010100') WHERE id IN(SELECT operation_id FROM ads.insight_reads WHERE draft_id=$1)`, d)
	for _, wire := range []string{"age,gender", "publisher_platform,platform_position", "device_platform", "hourly_stats_aggregated_by_advertiser_time_zone"} {
		e.g.SetBreakdowns(camp, day, wire, nil)
	}
	e.g.SetBreakdowns(camp, day, "region", []map[string]any{{"date_start": day, "date_stop": day, "region": "Kaohsiung", "spend": "2.00", "reach": "1", "impressions": "2", "clicks": "1"}})
	e.sweep("insights")
	e.settle()
	lcSameSet(t, "whole-day replacement without duplicates", lcStrings(t, e.f.owner, `SELECT dimension||'/'||bucket||'/'||spend_minor::text FROM ads.insights_breakdowns WHERE draft_id=$1 AND day=$2`, d, day), []string{"region/Kaohsiung/200"})
	for _, role := range []string{"commerce_runtime", "commerce_buyer_runtime", "commerce_ads_worker"} {
		pool := miPool(t, e.f, role)
		_, err := pool.Exec(e.ctx, `SELECT * FROM ads.insights_breakdowns WHERE store_id=$1`, e.store)
		if sqlState(err) != "42501" {
			t.Fatalf("%s raw aggregate access: %v", role, err)
		}
	}
	foreign := newAdsEnv(t, adsOpts{})
	if resp := e.apiOn(foreign.store, "GET", "/attribution?from="+day+"&to="+day, e.token, nil, nil); resp.Status != 404 {
		t.Fatalf("foreign store report accessible: %d", resp.Status)
	}
	if resp := foreign.api("GET", "/attribution?from="+day+"&to="+day, foreign.token, nil, nil); resp.Status != 200 || strings.Contains(string(resp.Raw), "Kaohsiung") {
		t.Fatalf("foreign report received another store's partition: %d %s", resp.Status, resp.Raw)
	}
}

func TestAdsAttributionAT1BeginFreeze(t *testing.T) {
	t.Run("valid click frozen once and replay cannot change it", func(t *testing.T) {
		b := bcSetup(t)
		pgLog := lcServerLog(t, b.f)
		e := newAdsEnv(t, adsOpts{fx: b.f})
		d := e.newDraft(adsDraftIn{})
		b.input.AdTouch = atTouch(d, time.Minute)
		b.input.ClientIP = "192.0.2.8"
		key := t04Key("at-begin")
		r, err := b.begin(key)
		if err != nil {
			for _, line := range strings.Split(pgLog(), "\n") {
				if strings.Contains(line, "ERROR:") || strings.Contains(line, "CONTEXT:") {
					t.Log(line)
				}
			}
			t.Fatal(err)
		}
		var path, draft, fbc, fbp string
		var ip *string
		if err = b.f.owner.QueryRow(context.Background(), `SELECT path,draft_id::text,fbc,fbp,host(client_ip) FROM orders.order_attribution WHERE order_id=$1`, r.OrderID).Scan(&path, &draft, &fbc, &fbp, &ip); err != nil {
			t.Fatal(err)
		}
		if path != "ad_click" || draft != d || fbc != atFBC || fbp != atFBP || ip != nil {
			t.Fatalf("wrong frozen measurement: %s %s %s %s ip=%v", path, draft, fbc, fbp, ip)
		}
		before := lcStrings(t, b.f.owner, `SELECT row_to_json(a)::text FROM orders.order_attribution a WHERE order_id=$1`, r.OrderID)
		b.input.AdTouch = atTouch(e.newDraft(adsDraftIn{}), time.Second)
		replay, err := b.begin(key)
		if err != nil || replay.OrderID != r.OrderID {
			t.Fatalf("replay %v %v", replay, err)
		}
		lcSameSet(t, "measurement frozen on replay", lcStrings(t, b.f.owner, `SELECT row_to_json(a)::text FROM orders.order_attribution a WHERE order_id=$1`, r.OrderID), before)
		var leaked bool
		if err = b.f.owner.QueryRow(context.Background(), `SELECT snapshot::text LIKE '%synthetic-click%' OR snapshot::text LIKE '%123456789%' FROM checkout.orders WHERE id=$1`, r.OrderID).Scan(&leaked); err != nil || leaked {
			t.Fatalf("pseudonym in legally retained snapshot: %v %v", leaked, err)
		}
	})
	for _, name := range []string{"expired", "future", "foreign store", "malformed"} {
		t.Run(name, func(t *testing.T) {
			b := bcSetup(t)
			e := newAdsEnv(t, adsOpts{fx: b.f})
			d := e.newDraft(adsDraftIn{})
			b.input.AdTouch = atTouch(d, time.Minute)
			switch name {
			case "expired":
				b.input.AdTouch.ClickedAt = time.Now().Add(-8 * 24 * time.Hour)
			case "future":
				b.input.AdTouch.ClickedAt = time.Now().Add(time.Hour)
			case "foreign store":
				foreign := newAdsEnv(t, adsOpts{})
				b.input.AdTouch.DraftID = foreign.newDraft(adsDraftIn{})
			case "malformed":
				b.input.AdTouch.DraftID = "invalid"
			}
			r, err := b.begin(t04Key("at-ignore"))
			if err != nil {
				t.Fatal(err)
			}
			if n := miCount(t, b.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE order_id=$1`, r.OrderID); n != 0 {
				t.Fatalf("invalid context credited: %d", n)
			}
		})
	}
}

func TestAdsAttributionAT1AtomicRollback(t *testing.T) {
	b := bcSetup(t)
	e := newAdsEnv(t, adsOpts{fx: b.f})
	b.input.AdTouch = atTouch(e.newDraft(adsDraftIn{}), time.Minute)
	before := b.facts(t)
	name := "at_fault_" + t04Tag()
	mustExec(t, b.f.owner, `CREATE SEQUENCE public.`+name+`_hits`)
	mustExec(t, b.f.owner, `GRANT USAGE ON SEQUENCE public.`+name+`_hits TO commerce_checkout_writer`)
	mustExec(t, b.f.owner, `CREATE FUNCTION public.`+name+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.store_id=`+quoteLiteral(b.f.storeA1)+`::uuid THEN PERFORM nextval('public.`+name+`_hits'); RAISE EXCEPTION 'synthetic attribution fault'; END IF; RETURN NEW; END $$`)
	mustExec(t, b.f.owner, `CREATE TRIGGER `+name+` BEFORE INSERT ON orders.order_attribution FOR EACH ROW EXECUTE FUNCTION public.`+name+`()`)
	t.Cleanup(func() {
		mustExec(t, b.f.owner, `DROP TRIGGER `+name+` ON orders.order_attribution`)
		mustExec(t, b.f.owner, `DROP FUNCTION public.`+name+`() `)
		mustExec(t, b.f.owner, `DROP SEQUENCE public.`+name+`_hits`)
	})
	if _, err := b.begin(t04Key("at-fault")); err == nil {
		t.Fatal("injected failure accepted")
	}
	var fired bool
	if err := b.f.owner.QueryRow(context.Background(), `SELECT is_called FROM public.`+name+`_hits`).Scan(&fired); err != nil || !fired {
		t.Fatalf("attribution insertion was not reached: %v %v", fired, err)
	}
	if b.facts(t) != before {
		t.Fatal("failed attribution left an order, reservation or job")
	}
	if n := miCount(t, b.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE store_id=$1`, b.f.storeA1); n != 0 {
		t.Fatalf("orphan measurement: %d", n)
	}
}

func atSeedContext(t *testing.T, c *capiEnv, d string) {
	t.Helper()
	mustExec(t, c.f.owner, `INSERT INTO orders.order_attribution(order_id,tenant_id,store_id,path,draft_id,clicked_at,fbc,fbp,client_ip) VALUES($1,$2,$3,'ad_click',$4,clock_timestamp(),$5,$6,'192.0.2.8')`, c.p.result.OrderID, c.tenant, c.store, d, atFBC, atFBP)
}

func TestAdsAttributionAT2PayloadAndTerminalIP(t *testing.T) {
	for _, consented := range []bool{true, false} {
		t.Run(map[bool]string{true: "consented", false: "refused"}[consented], func(t *testing.T) {
			c := newCapiEnv(t, adsOpts{})
			d := c.newDraft(adsDraftIn{})
			atSeedContext(t, c, d)
			mustExec(t, c.f.owner, `UPDATE checkout.orders SET buyer_email='  Buyer@Example.Test ' WHERE id=$1`, c.p.result.OrderID)
			if r := c.setCapi(true); r.Status != 200 {
				t.Fatal(string(r.Raw))
			}
			c.capture("SANDBOX", "SANDBOX")
			if r := c.consent(consented); r.status != 200 {
				t.Fatal(string(r.body))
			}
			c.sweep("capi")
			c.settle()
			evs := c.events()
			if !consented {
				if len(evs) != 0 {
					t.Fatal("nonconsented HTTP")
				}
				c.sweep("capi")
			} else {
				if len(evs) != 1 {
					t.Fatalf("events=%d", len(evs))
				}
				var payload map[string]any
				if err := json.Unmarshal(evs[0].Body, &payload); err != nil {
					t.Fatal(err)
				}
				ev := payload["data"].([]any)[0].(map[string]any)
				ud := ev["user_data"].(map[string]any)
				if ev["event_id"] != attribution.EventID(c.p.result.AttemptID) || ud["fbc"] != atFBC || ud["fbp"] != atFBP || ud["client_ip_address"] != "192.0.2.8" {
					t.Fatalf("payload=%v", ev)
				}
				if got := ud["em"].([]any)[0]; got != attribution.HashEmail("buyer@example.test") {
					t.Fatalf("email hash=%v", got)
				}
				raw, _ := json.Marshal(payload)
				if strings.Contains(string(raw), "Buyer@") {
					t.Fatal("raw email leaked")
				}
			}
			if _, err := c.workerPool.Exec(c.ctx, `SELECT ads.plan_capi_purge()`); err != nil {
				t.Fatal(err)
			}
			var ip *string
			if err := c.f.owner.QueryRow(c.ctx, `SELECT host(client_ip) FROM orders.order_attribution WHERE order_id=$1`, c.p.result.OrderID).Scan(&ip); err != nil || ip != nil {
				t.Fatalf("terminal/ineligible IP retained: %v %v", ip, err)
			}
			if n := c.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND path='ad_click' AND draft_id=$2 AND fbc=$3 AND fbp=$4`, c.p.result.OrderID, d, atFBC, atFBP); n != 1 {
				t.Fatal("aggregate/pseudonym context unexpectedly removed")
			}
		})
	}
}

func TestAdsAttributionR3Erasure(t *testing.T) {
	c := newCapiEnv(t, adsOpts{})
	other := newCapiEnv(t, adsOpts{})
	otherDraft := other.newDraft(adsDraftIn{})
	atSeedContext(t, other, otherDraft)
	d := c.newDraft(adsDraftIn{})
	atSeedContext(t, c, d)
	// The privacy domain's actual erasure definer owns redaction, not an UPDATE in this test.
	before := lcStrings(t, c.f.owner, `SELECT snapshot::text FROM checkout.orders WHERE id=$1`, c.p.result.OrderID)
	facts := lcStrings(t, c.f.owner, `SELECT path||'/'||draft_id::text||'/'||clicked_at::text||'/'||frozen_at::text FROM orders.order_attribution WHERE order_id=$1`, c.p.result.OrderID)
	if _, err := c.f.owner.Exec(c.ctx, `SELECT customers.apply_erasure($1,$2,$3)`, c.tenant, c.store, c.p.cap.Scope.OwnerID); err != nil {
		t.Fatal(err)
	}
	if n := c.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND fbc IS NULL AND fbp IS NULL AND client_ip IS NULL`, c.p.result.OrderID); n != 1 {
		t.Fatal("erasure retained tracking identifiers")
	}
	lcSameSet(t, "retained aggregate facts", lcStrings(t, c.f.owner, `SELECT path||'/'||draft_id::text||'/'||clicked_at::text||'/'||frozen_at::text FROM orders.order_attribution WHERE order_id=$1`, c.p.result.OrderID), facts)
	lcSameSet(t, "retained financial snapshot", lcStrings(t, c.f.owner, `SELECT snapshot::text FROM checkout.orders WHERE id=$1`, c.p.result.OrderID), before)
	if n := other.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND fbc=$2 AND fbp=$3 AND client_ip IS NOT NULL`, other.p.result.OrderID, atFBC, atFBP); n != 1 {
		t.Fatal("erasure crossed buyer/store boundary")
	}
}

func TestAdsAttributionR3NoOperationPurge(t *testing.T) {
	for _, reason := range []string{"no consent", "disabled", "expired", "cancelled"} {
		t.Run(reason, func(t *testing.T) {
			c := newCapiEnv(t, adsOpts{})
			d := c.newDraft(adsDraftIn{})
			atSeedContext(t, c, d)
			if r := c.setCapi(true); r.Status != 200 {
				t.Fatal(string(r.Raw))
			}
			if r := c.consent(true); r.status != 200 {
				t.Fatal(string(r.body))
			}
			switch reason {
			case "no consent":
				if r := c.consent(false); r.status != 200 {
					t.Fatal(string(r.body))
				}
			case "disabled":
				if r := c.setCapi(false); r.Status != 200 {
					t.Fatal(string(r.Raw))
				}
			case "expired":
				c.ownerReplica(`UPDATE checkout.orders SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, c.p.result.OrderID)
			case "cancelled":
				c.ownerReplica(`UPDATE checkout.orders SET commercial_state='CANCELLED' WHERE id=$1`, c.p.result.OrderID)
			}
			if len(c.capiOps()) != 0 {
				t.Fatal("fixture unexpectedly planned CAPI")
			}
			if _, err := c.workerPool.Exec(c.ctx, `SELECT ads.plan_capi_purge()`); err != nil {
				t.Fatal(err)
			}
			if n := c.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND client_ip IS NULL AND draft_id=$2 AND fbc=$3 AND fbp=$4`, c.p.result.OrderID, d, atFBC, atFBP); n != 1 {
				t.Fatal("no-operation cleanup did not clear only IP")
			}
		})
	}
}

func TestAdsAttributionR3ConsentedBeginKeepsIP(t *testing.T) {
	c := newCapiEnv(t, adsOpts{})
	if r := c.setCapi(true); r.Status != 200 {
		t.Fatal(string(r.Raw))
	}
	b := c.p.bcHarness
	b.prepare(t, mustIssue(t, b.cqHarness.service, c.store), []storefront.Item{{SKUID: b.stock.skus[0].ID, Quantity: 1}})
	c.p.cap = b.cap
	if r := c.consent(true); r.status != 200 {
		t.Fatal(string(r.body))
	}
	d := c.newDraft(adsDraftIn{})
	b.input.AdTouch = atTouch(d, time.Minute)
	b.input.ClientIP = "192.0.2.8"
	result, err := b.begin(t04Key("at-ip-begin"))
	if err != nil {
		t.Fatal(err)
	}
	var ip *string
	if err = c.f.owner.QueryRow(c.ctx, `SELECT host(client_ip) FROM orders.order_attribution WHERE order_id=$1`, result.OrderID).Scan(&ip); err != nil || ip == nil || *ip != "192.0.2.8" {
		t.Fatalf("consented Begin IP=%v %v", ip, err)
	}
	if n := c.count(`SELECT count(*) FROM ads.capi_events WHERE attempt_id IN(SELECT id FROM checkout.payment_attempts WHERE order_id=$1)`, result.OrderID); n != 0 {
		t.Fatal("Begin prematurely emitted Purchase")
	}
}

func TestAdsAttributionR3FinalFailureClearsIP(t *testing.T) {
	c := newCapiEnv(t, adsOpts{})
	d := c.newDraft(adsDraftIn{})
	atSeedContext(t, c, d)
	c.prime("SANDBOX", "SANDBOX")
	c.g.Inject(fakegraph.Fault{Route: fakegraph.RouteEvents, Kind: fakegraph.FaultGraphError, HTTP: 400, Code: 100})
	c.sweep("capi")
	c.settle()
	ops := c.capiOps()
	if len(ops) != 1 || ops[0].State != "FAILED_FINAL" || len(c.events()) != 1 {
		t.Fatalf("final failure %+v HTTP=%d", ops, len(c.events()))
	}
	var ip *string
	if err := c.f.owner.QueryRow(c.ctx, `SELECT host(client_ip) FROM orders.order_attribution WHERE order_id=$1`, c.p.result.OrderID).Scan(&ip); err != nil || ip != nil {
		t.Fatalf("final failure retained IP: %v %v", ip, err)
	}
	if n := c.count(`SELECT count(*) FROM orders.order_attribution WHERE order_id=$1 AND draft_id=$2 AND fbc=$3 AND fbp=$4`, c.p.result.OrderID, d, atFBC, atFBP); n != 1 {
		t.Fatal("terminal cleanup changed other facts")
	}
}

func TestAdsAttributionR2ScopeAndACL(t *testing.T) {
	b := bcSetup(t)
	result, err := b.begin(t04Key("at-scope"))
	if err != nil {
		t.Fatal(err)
	}
	foreign := newAdsEnv(t, adsOpts{})
	foreignDraft := foreign.newDraft(adsDraftIn{})
	tx, err := b.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	lcProbe(t, tx, "23503", "foreign-store measurement cannot reference this order", `INSERT INTO orders.order_attribution(order_id,tenant_id,store_id,path,draft_id,clicked_at) VALUES($1,$2,$3,'ad_click',$4,clock_timestamp())`, result.OrderID, foreign.tenant, foreign.store, foreignDraft)
	for _, role := range []string{"commerce_runtime", "commerce_buyer_runtime", "commerce_checkout_runtime", "commerce_ads_worker"} {
		pool := miPool(t, b.f, role)
		_, err := pool.Exec(context.Background(), `SELECT fbc,fbp,client_ip FROM orders.order_attribution WHERE order_id=$1`, result.OrderID)
		if sqlState(err) != "42501" {
			t.Fatalf("%s raw measurement access: %v", role, err)
		}
	}
	// R9: an older-than-one-minute historical order cannot acquire tracking,
	// but attribution refusal must not abort an otherwise valid transaction.
	mustExec(t, b.f.owner, `UPDATE checkout.orders SET created_at=clock_timestamp()-interval '2 minutes',expires_at=clock_timestamp()+interval '8 minutes' WHERE id=$1`, result.OrderID)
	before := atR9Rows(t, b, result.OrderID)
	var xid string
	err = buyer.WithScope(context.Background(), b.pool, b.cap.Token, b.f.storeA1, func(ctx context.Context, tx pgx.Tx, _ buyer.Scope) error {
		raw, _ := json.Marshal(atTouch(foreignDraft, time.Minute))
		_, err := tx.Exec(ctx, `SELECT orders.freeze_attribution($1,$2,$3,$4::jsonb,'192.0.2.8')`, tokenHash(b.cap.Token), b.f.storeA1, result.OrderID, string(raw))
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT pg_current_xact_id()::text`).Scan(&xid)
	})
	if err != nil {
		t.Fatalf("historical attribution refusal aborted transaction: %v", err)
	}
	atR9Committed(t, b, xid)
	lcSameSet(t, "historical freeze remains no-op", atR9Rows(t, b, result.OrderID), before)
	if n := miCount(t, b.f.owner, `SELECT count(*) FROM orders.order_attribution WHERE order_id=$1`, result.OrderID); n != 0 {
		t.Fatal("rejected mutation left tracking")
	}
}
