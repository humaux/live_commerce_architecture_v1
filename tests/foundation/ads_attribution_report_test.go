package foundation_test

import (
	"fmt"
	"testing"
	"time"
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
