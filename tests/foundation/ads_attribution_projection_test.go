package foundation_test

import (
	"testing"
	"time"
)

// This independent empty-cohort probe deliberately does not create a checkout;
// it validates every SQL read/ACL boundary without concealing a Begin failure.
func TestAdsAttributionEmptyProjection(t *testing.T) {
	e := newAdsEnv(t, adsOpts{})
	e.newDraft(adsDraftIn{})
	tz, _ := time.LoadLocation("Asia/Taipei")
	day := time.Now().In(tz).Format("2006-01-02")
	r := e.api("GET", "/attribution?from="+day+"&to="+day, e.token, nil, nil)
	if r.Status != 200 {
		t.Fatalf("empty scoped report: status=%d body=%s", r.Status, r.Raw)
	}
	drafts, ok := r.JSON["drafts"].([]any)
	if !ok || len(drafts) != 1 {
		t.Fatalf("draft projection: %s", r.Raw)
	}
	d := drafts[0].(map[string]any)
	if len(d["orders"].([]any)) != 0 || d["meta"].(map[string]any)["purchases"] != nil || d["roas"] != nil {
		t.Fatalf("empty cohort invented data: %s", r.Raw)
	}
}
