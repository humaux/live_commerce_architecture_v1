package foundation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/claims"
	"livecommerce/internal/claimsintake"
	"livecommerce/internal/httpapi"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/integrations/metareply"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
	"livecommerce/tests/ads/fakegraph"
)

// REAL_PG + MOCK Graph: synthetic custody and paid order fixtures only. The
// attribution row below is a disclosed read-projection fixture; AT1 separately
// exercises actual Begin and its transaction boundary.
func TestAdsAttributionReport(t *testing.T) {
	c := newCapiEnv(t, adsOpts{})
	d := c.newDraft(adsDraftIn{})
	c.capture("SANDBOX", "SANDBOX")
	mustExec(t, c.p.f.owner, `INSERT INTO orders.order_attribution(order_id,tenant_id,store_id,path,draft_id,clicked_at)
 VALUES($1,$2,$3,'ad_click',$4,clock_timestamp())`, c.p.result.OrderID, c.p.f.tenantA, c.p.f.storeA1, d)
	tz, _ := time.LoadLocation("Asia/Taipei")
	day := time.Now().In(tz).Format("2006-01-02")
	got := c.api("GET", "/attribution?from="+day+"&to="+day, c.token, nil, nil)
	if got.Status != 200 {
		t.Fatalf("attribution report status=%d body=%s", got.Status, got.Raw)
	}
	drafts, ok := got.JSON["drafts"].([]any)
	if !ok || len(drafts) != 1 {
		t.Fatalf("drafts missing: %s", got.Raw)
	}
	row := drafts[0].(map[string]any)
	paths := row["orders"].([]any)
	if len(paths) != 1 {
		t.Fatalf("paths: %s", got.Raw)
	}
	p := paths[0].(map[string]any)
	var total int64
	if err := c.p.f.owner.QueryRow(c.ctx, `SELECT total_minor FROM checkout.orders WHERE id=$1`, c.p.result.OrderID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(p["net_minor"]) != fmt.Sprint(total) || p["orders"] != float64(1) || p["path"] != "ad_click" {
		t.Fatalf("wrong real order metrics: %s", got.Raw)
	}
	meta := row["meta"].(map[string]any)
	if meta["purchases"] != nil || meta["purchase_value_minor"] != nil || row["roas"] != nil {
		t.Fatalf("unreported Meta data must stay unknown, not our order: %s", got.Raw)
	}
}

const atPrivateStreet = "SYNTHETIC-PRIVATE-STREET-AT9"
const atPrivateCity = "臺北市 SYNTHETIC-PRIVATE-CITY-AT9"

// The same authoritative fixture is consumed by the PG assertions and browser
// gate. No report response is used to calculate its expected values.
type atReportEnv struct {
	*adsEnv
	r                            *srfEnv
	m                            *mciEnv
	day, firstDraft, secondDraft string
	paid, returning              rfxOrder
	cod                          checkout.Result
	codCap                       buyer.Capability
	expected                     map[string]any
}

// Minimal Page intake assembly on the Stripe fixture's isolated PG cluster.
// miSetup normally uses the serial foundation cluster; crossing those clusters
// would make a superficially green but unrelated claim/order fixture.
func atReportClaims(t *testing.T, o rfxOrder) *mciEnv {
	t.Helper()
	h := cblClaims(t, o)
	f, ctx := h.f, context.Background()
	p := miTest{f: f, ingress: miPool(t, f, "commerce_meta_ingress"), registrar: miPool(t, f, "commerce_meta_registrar"), curator: miPool(t, f, "commerce_meta_curator"), key: randomBytes(32)}
	keys, err := meta.NewPayloadKeyring(miKeyID, map[string][]byte{miKeyID: p.key})
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := meta.NewInbox(ctx, p.ingress, keys)
	if err != nil {
		t.Fatal(err)
	}
	p.verifier, err = meta.NewVerifier(meta.Config{AppID: miApp, Object: "page", AppSecret: miSecret, VerifyToken: "synthetic-at9-verify-token"})
	if err != nil {
		t.Fatal("AT9 synthetic webhook verifier:", err)
	}
	p.handler, err = meta.NewInboxHandler(p.verifier, inbox)
	if err != nil {
		t.Fatal(err)
	}
	m := &mciEnv{t: t, h: h, page: p, stopConsumer: mciNoop, consumerWorkers: 2, actorRaw: randomBytes(32), linkRaw: randomBytes(32)}
	m.actor, err = meta.NewClaimsActorKey(m.actorRaw)
	if err != nil {
		t.Fatal(err)
	}
	m.link, err = claims.NewReplyLinkKey(m.linkRaw)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) SELECT $1,$2,$3,p FROM unnest(ARRAY['integration:execute','integration:manage']) p ON CONFLICT DO NOTHING`, f.tenantA, f.storeA1, h.actor)
	m.session = h.draft(t, f.storeA1)
	t.Cleanup(func() {
		// Delete this fixture's aggregate snapshot before the shared session purge.
		mustExec(t, f.owner, `DELETE FROM ads.live_audience_snapshots WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`, f.tenantA, f.storeA1, m.session)
		m.cleanup()
	})
	h.open(t, m.session, claims.MatchExact)
	m.sku = h.stock.skus[0].ID
	m.offer = h.offer(t, m.session, "A1", m.sku, 5)
	m.pageAsset = miAsset()
	m.pageBinding = miBinding(t, p, m.pageAsset, "facebook", f.tenantA, f.storeA1, h.actor)
	miRoute(t, p, m.pageAsset, f.tenantA, f.storeA1, m.pageBinding)
	m.postID = m.pageAsset + "_" + mciDigits(10)
	m.srcFB = m.mustSource(t, "page", m.pageAsset, m.postID, false)
	m.intakeLogin = miRole(t, f, "commerce_claims_intake")
	m.intakePool, err = platform.OpenClaimsIntakePool(ctx, m.intakeLogin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.intakePool.Close)
	m.poller, err = claimsintake.New(ctx, m.intakePool, m.link, claimsintake.Config{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	m.startConsumer(t) // own worker stop/pool cleanup is registered by this helper
	return m
}

func atNewReportEnv(t *testing.T) *atReportEnv {
	t.Helper()
	r := srfNew(t) // historical paid order: the returning owner's real previous purchase
	p := r.base.s.p
	r.ensureStock(t, r.base)
	m := atReportClaims(t, r.base)
	e := newAdsEnv(t, adsOpts{fx: p.f})
	x := &atReportEnv{adsEnv: e, r: r, m: m, day: taipeiDay(0)}
	mustExec(t, p.f.owner, `UPDATE integration.bindings SET external_asset_id=$2 WHERE id=$1`, e.idBinding, m.pageAsset)
	e.pageAsset = m.pageAsset
	for i := 0; i < 2; i++ {
		d := e.newDraft(adsDraftIn{Source: m.postID})
		e.mustApprove(d)
		e.mustPublish(d)
		e.driveTo(d, "activate", 1)
		e.ownerReplica(`UPDATE ads.campaign_drafts SET starts_at=clock_timestamp()-interval '1 hour',ends_at=clock_timestamp()+interval '1 hour' WHERE id=$1`, d)
		if i == 0 {
			x.firstDraft = d
		} else {
			x.secondDraft = d
		}
	}
	// Signed comments -> APPLIED intake -> bound bundles -> real cart origins.
	// Claim version/offer/session/post are captured only by actual Begin below.
	caps := []buyer.Capability{mustIssue(t, m.h.service, p.f.storeA1), p.cap, mustIssue(t, m.h.service, p.f.storeA1)}
	for i, cap := range caps {
		at := mciSoon()
		// 2 x 1250 meets Stripe's minimum and the whole-TWD contract shared
		// by card and cash-on-delivery orders; never weaken either guard.
		sent := m.postFBTo(t, m.postID, "", "", "A1+2", &at, nil, true)
		m.apply(t)
		intake := m.mustIntake(t, "page", m.pageAsset, sent.comment)
		if intake.State != "APPLIED" {
			t.Fatal("claim intake must apply")
		}
		ev := lcEvent(t, p.f, intake.AppliedEvent)
		if ev.outcome != "ACCEPTED" {
			t.Fatal("claim must be accepted")
		}
		link := m.h.link(t, m.session, ev.bundle, 0, false)
		preview, err := m.h.preview(cap, link.Token)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = m.h.redeem(cap, t04Key("at9-redeem"), link.Token, preview.BundleVersion); err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			x.codCap = cap
		}
	}
	m.h.closeWindow(t, m.session)
	offline := &tcvEnv{t: t, p: p, svc: p.bcHarness.service, merchant: httpapi.NewHandler(p.f.runtime, httpapi.Options{})}
	r.grant(t, r.base, "integration:manage", "payments:refund", "orders:read", "fulfillment:write")
	if st, _ := offline.hcodSettings(0, true, 20000, 50, "black_cat"); st != 200 {
		t.Fatalf("COD settings=%d", st)
	}
	for i, cap := range caps {
		cart := m.h.cartOf(t, cap)
		h := p.cqHarness
		h.cap = cap
		destIn := bdHome(cart)
		destIn.HomeAddress = storefront.HomeAddress{Region: "臺北市", City: "臺北市", Line1: atPrivateStreet}
		if i == 1 {
			destIn.ExpectedVersion = p.destination.Version
			destIn.HomeAddress.Region = atPrivateStreet
			destIn.HomeAddress.City = atPrivateCity
		}
		dest, err := bdSet(h, t04Key("at9-home"), destIn)
		if err != nil {
			t.Fatal(err)
		}
		q, err := cqBuyer(h.a.runtime, cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Quote, error) {
			return storefront.CreateQuote(ctx, tx, s, t04Key("at9-quote"), storefront.QuoteInput{CartVersion: cart.Version, MarketID: h.market.ID, Country: "TW", Method: "delivery:" + p.delivery.Code})
		})
		if err != nil {
			t.Fatal(err)
		}
		in := checkout.Input{QuoteID: q.ID, DestinationID: dest.ID, CartVersion: cart.Version, ServiceVersion: 1, AllocationVersion: 1}
		if i == 0 {
			in.AdTouch = atTouch(x.firstDraft, time.Minute)
		}
		if i == 2 {
			in.PaymentMode = "cash_on_delivery"
		}
		res, err := p.bcHarness.service.Begin(context.Background(), cap.Token, p.f.storeA1, t04Key("at9-begin"), in)
		if err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			x.cod = res
			continue
		}
		s := r.base.s
		s.p.cap = cap
		s.p.hold = res
		s.p.bcHarness.input = in
		o := r.pay(t, s, r.base.endpoint, r.base.secret)
		if i == 0 {
			x.paid = o
		} else {
			x.returning = o
		}
	}
	refund := r.mustRefund(t, x.paid, 100, "requested_by_customer")
	r.awaitRefundFact(t, refund, x.paid.attempt, "SUCCEEDED")
	// MOCK Insights enter through the real dispatcher and Finish transaction.
	for i, d := range []string{x.firstDraft, x.secondDraft} {
		camp := e.mustOp(d, "campaign", 1).Ref
		spend := "12.30"
		if i == 1 {
			spend = "5.00"
		}
		e.g.SetInsights(camp, x.day, fakegraph.Insights{Spend: spend, Impressions: "1000", Clicks: "9", PurchaseCount: "9", PurchaseValue: "900.00"})
		e.g.SetBreakdowns(camp, x.day, "hourly_stats_aggregated_by_advertiser_time_zone", []map[string]any{{"date_start": x.day, "date_stop": x.day, "spend": spend, "hourly_stats_aggregated_by_advertiser_time_zone": "13:00:00 - 13:59:59"}})
	}
	e.sweep("insights")
	e.settle()
	// Dispatch persists breakdowns; the real advance worker ingests completed
	// daily reads. Idle dispatch alone does not mean daily reports are ready.
	e.sweep("advance")
	if n := miCount(t, p.f.owner, `SELECT count(*) FROM ads.insights_daily WHERE tenant_id=$1 AND store_id=$2 AND day=$3 AND ((draft_id=$4 AND spend_minor=1230) OR (draft_id=$5 AND spend_minor=500))`, p.f.tenantA, p.f.storeA1, x.day, x.firstDraft, x.secondDraft); n != 2 {
		t.Fatalf("actual advance must ingest both exact daily spend snapshots: %d", n)
	}
	// Real Page custody -> lease-fenced claims dispatcher -> aggregate snapshot.
	var err error
	m.pageKeys, err = metareply.NewPageTokenKeyring("at9_page", map[string][]byte{"at9_page": randomBytes(32)})
	if err != nil {
		t.Fatal(err)
	}
	m.pageToken = "SENTINEL-READONLY-PAGE-AT9-" + t04Tag()
	m.registerToken(t, "facebook", m.pageBinding, m.pageAsset, []string{"read_insights", "pages_read_engagement"}, m.pageToken)
	mustExec(t, p.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'live:read') ON CONFLICT DO NOTHING`, p.f.tenantA, p.f.storeA1, e.creator)
	graph := newATSGraph(t, m.postID)
	graph.setAvailable()
	pool := miPool(t, p.f, waClaims)
	routes, err := metareply.AudienceRoutes(pool, m.pageKeys, nil, metareply.Config{GraphBaseURL: graph.srv.URL, GraphVersion: "v26.0"})
	if err != nil {
		t.Fatal(err)
	}
	t06StartDispatcher(t, pool, "default", routes, mciDispatchOptions())
	read := e.api("POST", "/sessions/"+m.session+"/audience-read", e.token, adsKey(), nil)
	if read.Status != 200 {
		t.Fatalf("actual audience plan=%d", read.Status)
	}
	op, _ := read.JSON["operation_id"].(string)
	if op == "" {
		t.Fatal("actual audience plan operation missing")
	}
	m.awaitOp(t, op, "SUCCEEDED", 8*time.Second, "completed")
	var pending int64
	if err := p.f.owner.QueryRow(context.Background(), `SELECT total_minor+cod_surcharge_minor FROM checkout.orders WHERE id=$1`, x.cod.OrderID).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	var productName string
	if err := p.f.owner.QueryRow(context.Background(), `SELECT snapshot#>>'{quote,lines,0,name}' FROM checkout.orders WHERE id=$1`, x.paid.order).Scan(&productName); err != nil {
		t.Fatal(err)
	}
	x.expected = map[string]any{"orders": 3, "net_minor": x.paid.captured - 100 + x.returning.captured, "pending_orders": 1, "pending_minor": pending, "spend_minor": 1730, "meta_purchases": 9, "meta_value_minor": 90000, "comments": 3, "claims": 3, "checkout_links": 3, "paid_orders": 2, "ambiguous_orders": 2, "new_buyers": 1, "returning_buyers": 1, "counties": []map[string]any{{"name": "臺北市", "orders": 1, "net_minor": x.paid.captured - 100}, {"name": "—", "orders": 1, "net_minor": x.returning.captured}}, "top_products": []map[string]any{{"name": productName, "quantity": 4}}}
	return x
}

func (x *atReportEnv) browserFixture() map[string]any {
	return map[string]any{"from": x.day, "to": x.day, "draft_id": x.firstDraft, "session_id": x.m.session, "audience_read": "queued", "expected": x.expected,
		"live_audience":     map[string]any{"status": "available", "views": 34, "peak_concurrent": nil, "total_view_time_ms": nil, "age_gender": []map[string]any{{"bucket": "F.25-34", "view_time_ms": 1234}}, "regions": []map[string]any{{"bucket": "Taipei", "view_time_ms": 4321}}},
		"forbidden_private": []string{atPrivateStreet, atPrivateCity},
		"hourly":            []map[string]any{{"day": x.day, "timezone_name": "Asia/Taipei", "dimension": "hourly", "bucket": "13:00:00 - 13:59:59", "hour_start": x.day + "T13:00:00+08:00", "spend_minor": 1230, "reach": 0, "impressions": 0, "clicks": 0, "engagements": 0, "comments": 0, "purchases": nil, "purchase_value_minor": nil}}}
}

func atNum(t *testing.T, row map[string]any, key string, want any) {
	t.Helper()
	if fmt.Sprint(row[key]) != fmt.Sprint(want) {
		t.Fatalf("%s=%v want %v", key, row[key], want)
	}
}

func (x *atReportEnv) assertReport(t *testing.T) {
	t.Helper()
	before := x.count(`SELECT count(*) FROM integration.operations WHERE store_id=$1`, x.store)
	response := x.api("GET", "/attribution?from="+x.day+"&to="+x.day, x.token, nil, nil)
	if response.Status != 200 {
		t.Fatalf("report status=%d", response.Status)
	}
	var session, first, second map[string]any
	for _, raw := range response.JSON["sessions"].([]any) {
		v := raw.(map[string]any)
		if v["session_id"] == x.m.session {
			session = v
		}
	}
	for _, raw := range response.JSON["drafts"].([]any) {
		v := raw.(map[string]any)
		if v["draft_id"] == x.firstDraft {
			first = v
		}
		if v["draft_id"] == x.secondDraft {
			second = v
		}
	}
	if session == nil || first == nil || second == nil {
		t.Fatal("exact scoped session/drafts missing")
	}
	audience := session["live_audience"].(map[string]any)
	atNum(t, audience, "status", "available")
	atNum(t, audience, "views", 34)
	if audience["peak_concurrent"] != nil || audience["total_view_time_ms"] != nil {
		t.Fatal("unreported audience values must stay null")
	}
	if audience["age_gender"].([]any)[0].(map[string]any)["view_time_ms"] != float64(1234) || audience["regions"].([]any)[0].(map[string]any)["view_time_ms"] != float64(4321) {
		t.Fatal("audience demographics must come from real MOCK Graph snapshot")
	}
	for _, key := range []string{"orders", "net_minor", "pending_orders", "pending_minor", "spend_minor", "ambiguous_orders"} {
		atNum(t, session, key, x.expected[key])
	}
	for _, key := range []string{"comments", "claims", "checkout_links", "paid_orders"} {
		atNum(t, session["funnel"].(map[string]any), key, x.expected[key])
	}
	for _, key := range []string{"new_buyers", "returning_buyers"} {
		atNum(t, session["buyers"].(map[string]any), key, x.expected[key])
	}
	for _, kind := range []string{"counties", "top_products"} {
		wanted := map[string]map[string]any{}
		for _, w := range x.expected[kind].([]map[string]any) {
			wanted[w["name"].(string)] = w
		}
		rows := session["buyers"].(map[string]any)[kind].([]any)
		if len(rows) != len(wanted) {
			t.Fatalf("%s rows=%d want %d", kind, len(rows), len(wanted))
		}
		for _, raw := range rows {
			row := raw.(map[string]any)
			want := wanted[row["name"].(string)]
			if want == nil {
				t.Fatalf("unexpected %s bucket", kind)
			}
			for key, v := range want {
				atNum(t, row, key, v)
			}
		}
	}
	path := first["orders"].([]any)
	if len(path) != 1 {
		t.Fatal("click draft must have one factual path")
	}
	row := path[0].(map[string]any)
	atNum(t, row, "orders", 1)
	atNum(t, row, "net_minor", x.paid.captured-100)
	atNum(t, row, "path", "ad_click")
	atNum(t, first, "spend_minor", 1230)
	meta := first["meta"].(map[string]any)
	atNum(t, meta, "purchases", 9)
	atNum(t, meta, "purchase_value_minor", 90000)
	if first["roas"] != math.Round(float64(x.paid.captured-100)/1230*10000)/10000 {
		t.Fatal("ROAS must use ours net, not Meta modeled revenue")
	}
	if len(second["orders"].([]any)) != 0 {
		t.Fatal("ambiguous boosted orders must not be fan-out credited")
	}
	var hourly float64
	for _, raw := range session["timeline"].([]any) {
		v := raw.(map[string]any)
		hourly += v["spend_minor"].(float64)
		if v["spend_minor"].(float64) > 0 {
			at, err := time.Parse(time.RFC3339, v["at"].(string))
			if err != nil {
				t.Fatal(err)
			}
			tz, _ := time.LoadLocation("Asia/Taipei")
			if at.In(tz).Hour() != 13 {
				t.Fatal("hour overlay must be absolute account-zone instant")
			}
		}
	}
	if hourly != 1730 {
		t.Fatal("hour overlay must independently sum both actual boost insights")
	}
	for _, needle := range []string{atPrivateStreet, atPrivateCity, x.paid.order, x.returning.order, x.cod.OrderID, x.paid.s.p.cap.Scope.OwnerID, x.returning.s.p.cap.Scope.OwnerID, x.codCap.Scope.OwnerID, "+886900000001"} {
		if strings.Contains(string(response.Raw), needle) {
			t.Fatal("report leaked non-aggregate private fixture field")
		}
	}
	if x.count(`SELECT count(*) FROM integration.operations WHERE store_id=$1`, x.store) != before {
		t.Fatal("GET report must not enqueue provider operations")
	}
	for _, order := range []string{x.paid.order, x.returning.order, x.cod.OrderID} {
		if x.count(`SELECT count(*) FROM claims.order_origins WHERE order_id=$1 AND session_id=$2 AND post_id=$3`, order, x.m.session, x.m.postID) != 1 {
			t.Fatal("fixture must have real price-neutral immutable claim origin")
		}
		if x.count(`SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1`, order) != 0 {
			t.Fatal("fixture must not rely on live-price use")
		}
	}
	encoded, err := json.Marshal(x.browserFixture())
	if err != nil || len(encoded) == 0 {
		t.Fatal("browser fixture serialization failed")
	}
}

func TestAdsAttributionAT4AT9ExactReport(t *testing.T) { x := atNewReportEnv(t); x.assertReport(t) }
