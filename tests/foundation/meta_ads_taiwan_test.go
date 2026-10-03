package foundation_test

import (
	"testing"

	"livecommerce/tests/ads/fakegraph"
)

// REAL_PG + MOCK Graph: the existing authenticated draft projection exposes the
// terminal operation code; no new grant, event read endpoint or migration needed.
func TestMetaAdsTaiwanFailedDraftReason(t *testing.T) {
	e := newAdsEnv(t, adsOpts{})
	e.g.Inject(fakegraph.Fault{Route: fakegraph.RouteCreateAdset, Kind: fakegraph.FaultGraphError, Code: 100, Subcode: 3858495})
	d := e.newDraft(adsDraftIn{})
	e.mustApprove(d)
	e.mustPublish(d)
	e.drive(d, 6, func() bool { o, ok := e.op(d, "adset", 1); return ok && o.State == "FAILED_FINAL" })
	o := e.mustOp(d, "adset", 1)
	if o.State != "FAILED_FINAL" || o.Code != "tw_advertiser_unverified" {
		t.Fatalf("operation: %+v", o)
	}
	var reason string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT reason_code FROM integration.operation_events WHERE tenant_id=$1 AND store_id=$2 AND operation_id=$3 AND state='FAILED_FINAL' ORDER BY id DESC LIMIT 1`, e.tenant, e.store, o.ID).Scan(&reason); err != nil || reason != o.Code {
		t.Fatalf("event reason=%q err=%v", reason, err)
	}
	read := e.draft(d)
	if read.str("status") != "FAILED" {
		t.Fatalf("draft status: %s", read.Raw)
	}
	found := false
	for _, raw := range read.JSON["ops"].([]any) {
		op := raw.(map[string]any)
		if op["kind"] == "adset" {
			found = op["attempt"] == read.JSON["publish_attempt"] && op["state"] == "FAILED_FINAL" && op["code"] == reason
		}
	}
	if !found {
		t.Fatalf("authenticated GET did not expose latest failed attempt code: %s", read.Raw)
	}
	for _, kind := range []string{"creative", "ad", "activate"} {
		if _, ok := e.op(d, kind, 1); ok {
			t.Fatalf("planned %s after refusal", kind)
		}
	}
	if objects := e.g.Objects("campaign"); len(objects) != 1 || objects[0].Status != "PAUSED" {
		t.Fatalf("campaign not safely paused: %+v", objects)
	}
}
