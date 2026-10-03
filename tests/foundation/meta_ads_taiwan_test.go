package foundation_test

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"testing"

	"livecommerce/tests/ads/fakegraph"
)

// REAL_PG + MOCK Graph: the authenticated projection preserves Meta's generic
// code and bounded original public-facing text, not internal diagnostics.
func TestMetaAdsOriginalRefusalDraftReason(t *testing.T) {
	e := newAdsEnv(t, adsOpts{})
	e.g.Inject(fakegraph.Fault{Route: fakegraph.RouteCreateAdset, Kind: fakegraph.FaultGraphError, Code: 100, Subcode: 3858495, UserMessage: "<b>Meta 原文</b> &amp; details<script>private()</script>"})
	d := e.newDraft(adsDraftIn{})
	e.mustApprove(d)
	e.mustPublish(d)
	e.drive(d, 6, func() bool { o, ok := e.op(d, "adset", 1); return ok && o.State == "FAILED_FINAL" })
	o := e.mustOp(d, "adset", 1)
	if o.State != "FAILED_FINAL" || o.Code != "graph_100" {
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
			found = op["attempt"] == read.JSON["publish_attempt"] && op["state"] == "FAILED_FINAL" && op["code"] == reason && op["error_user_msg"] == "Meta 原文 & details"
		}
	}
	if !found {
		t.Fatalf("authenticated GET did not expose latest failed attempt code and original sanitized Meta text: %s", read.Raw)
	}
	settings := e.api("GET", "/settings", e.token, nil, nil)
	if settings.Status != 200 || !strings.Contains(string(settings.Raw), "Meta 原文") || strings.Contains(string(settings.Raw), "private()") || strings.Contains(string(settings.Raw), "synthetic graph error") {
		t.Fatalf("settings must expose only sanitized public refusal: status=%d", settings.Status)
	}
	foreign := e.apiOn(randomUUID(), "GET", "/settings", e.token, nil, nil)
	if foreign.Status == 200 || strings.Contains(string(foreign.Raw), "Meta 原文") {
		t.Fatal("foreign store leaked refusal")
	}
	for role, pool := range map[string]*pgxpool.Pool{"runtime": e.f.runtime, "ads_worker": e.workerPool} {
		if _, err := pool.Exec(e.ctx, `SELECT error_user_msg FROM ads.operation_refusals`); !pgcode(err, "42501") {
			t.Errorf("%s direct table read=%v", role, err)
		}
	}
	if _, err := e.workerPool.Exec(e.ctx, `SELECT ads.finish_operation_refusal($1::uuid,1,$2::bytea,'dispatch','FAILED_FINAL','graph_100','overwrite')`, o.ID, make([]byte, 32)); !pgcode(err, "40001") {
		t.Fatalf("completed/stale lease was not refused: %v", err)
	}
	var retained string
	if err := e.f.owner.QueryRow(e.ctx, `SELECT error_user_msg FROM ads.operation_refusals WHERE operation_id=$1`, o.ID).Scan(&retained); err != nil || retained != "Meta 原文 & details" {
		t.Fatalf("stale completion changed original: %v", err)
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
