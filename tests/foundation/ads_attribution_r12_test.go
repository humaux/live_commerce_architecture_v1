package foundation_test

import (
	"context"
	"testing"
	"time"

	"livecommerce/internal/integrations/core"
)

// R12: GET-only reads may be requested again after their bounded cooldown.
// Owner writes age only the synthetic operation clock; the UNKNOWN outcome is
// produced by the real dispatcher consuming an empty local Graph video list.
func TestAdsAttributionR12UnknownReadCooldown(t *testing.T) {
	x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
	x.g.mu.Lock()
	x.g.missingVideo = true
	x.g.mu.Unlock()
	op := x.mustPlan()
	t06StartDispatcher(t, x.pool, x.queue, []core.DispatchRoute{x.route}, mciDispatchOptions())
	x.run(op)
	x.e.awaitOp(t, op, "UNKNOWN", 8*time.Second, "cancelled")
	jobs := miCount(t, x.e.h.f.owner, `SELECT count(*) FROM river.river_job`)
	r := x.plan()
	if r.Status != 200 || r.JSON["operation_id"] != op || r.JSON["state"] != "UNKNOWN" {
		t.Fatalf("UNKNOWN within cooldown must replay: status=%d receipt=%v", r.Status, r.JSON)
	}
	if miCount(t, x.e.h.f.owner, `SELECT count(*) FROM river.river_job`) != jobs {
		t.Fatal("UNKNOWN cooldown committed another job")
	}
	mustExec(t, x.e.h.f.owner, `UPDATE integration.operations SET updated_at=clock_timestamp()-interval '10 minutes 1 second' WHERE id=$1 AND state='UNKNOWN'`, op)
	x.g.mu.Lock()
	x.g.missingVideo = false
	x.g.mu.Unlock()
	next := x.mustPlan()
	if next == op || miCount(t, x.e.h.f.owner, `SELECT count(*) FROM river.river_job`) != jobs+1 {
		t.Fatal("expired UNKNOWN must create exactly one new GET read")
	}
	x.run(next)
	x.e.awaitOp(t, next, "SUCCEEDED", 8*time.Second, "completed")
}

func TestAdsAttributionR12CheckoutDefinerACL(t *testing.T) {
	x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
	// extra: a pinned per-function setting beyond search_path (attribution_metrics: custom plans, the R11 report deadline).
	for _, fn := range []struct{ sig, owner, grantee, extra string }{
		{"claims.capture_order_origins(uuid,uuid,uuid,uuid,jsonb)", "commerce_claims_writer", "commerce_checkout_writer", ""},
		{"claims.order_comment_posts(uuid,uuid,uuid)", "commerce_claims_writer", "commerce_checkout_writer", ""},
		{"claims.attribution_session_orders(uuid,uuid,uuid)", "commerce_claims_writer", "commerce_checkout_writer", ""},
		{"ads.attribution_match(uuid,uuid,uuid,text,timestamptz)", "commerce_ads_writer", "commerce_checkout_writer", ""},
		{"ads.capi_ip_needed(uuid,uuid,uuid,uuid)", "commerce_ads_writer", "commerce_checkout_writer", ""},
		{"ads.order_signals_allowed(uuid,uuid,uuid)", "commerce_ads_writer", "commerce_checkout_writer", ""},
		{"orders.freeze_attribution(bytea,uuid,uuid,jsonb,text,jsonb)", "commerce_checkout_writer", "commerce_checkout_runtime", ""},
		{"orders.erase_ad_context(uuid,uuid,uuid)", "commerce_checkout_writer", "commerce_privacy_writer", ""},
		{"orders.attribution_metrics(uuid,uuid,date,date,uuid,uuid)", "commerce_checkout_writer", "commerce_ads_writer", "plan_cache_mode=force_custom_plan"},
		{"orders.capi_context(uuid,uuid,uuid)", "commerce_checkout_writer", "commerce_ads_writer", ""},
		{"orders.purge_capi_ip()", "commerce_checkout_writer", "commerce_ads_writer", ""},
	} {
		t.Run(fn.sig, func(t *testing.T) {
			var owner string
			var secured bool
			if err := x.e.h.f.owner.QueryRow(context.Background(), `SELECT r.rolname,p.prosecdef AND NOT r.rolcanlogin AND p.proconfig=array_remove(ARRAY['search_path=pg_catalog',$2],'') FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=$1::regprocedure`, fn.sig, fn.extra).Scan(&owner, &secured); err != nil {
				t.Fatal(err)
			}
			if owner != fn.owner || !secured {
				t.Fatalf("definer owner=%s secure=%v", owner, secured)
			}
			grants := lcStrings(t, x.e.h.f.owner, `SELECT CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_get_userbyid(a.grantee) END FROM pg_proc p,aclexplode(coalesce(p.proacl,acldefault('f',p.proowner))) a WHERE p.oid=$1::regprocedure AND a.privilege_type='EXECUTE' AND a.grantee<>p.proowner`, fn.sig)
			lcSameSet(t, "exact attribution EXECUTE", grants, []string{fn.grantee})
		})
	}
}

func TestAdsAttributionR12GrantAwareUnread(t *testing.T) {
	for _, granted := range []bool{false, true} {
		name := "missing-read-insights"
		scopes := []string{"pages_read_engagement"}
		want := "not_authorized"
		if granted {
			name, want = "granted-no-snapshot", "not_read"
			scopes = append(scopes, "read_insights")
		}
		t.Run(name, func(t *testing.T) {
			x := newATSEnv(t, scopes)
			if got := x.reportAudience(); got["status"] != want {
				t.Fatalf("audience status=%v want=%s", got["status"], want)
			}
			if !granted {
				if r := x.plan(); r.Status != 403 {
					t.Fatalf("missing grant allowed read=%d", r.Status)
				}
				return
			}
			op := x.mustPlan()
			t06StartDispatcher(t, x.pool, x.queue, []core.DispatchRoute{x.route}, mciDispatchOptions())
			x.run(op)
			x.e.awaitOp(t, op, "SUCCEEDED", 8*time.Second, "completed")
			if got := x.reportAudience(); got["status"] != "insufficient" {
				t.Fatalf("real read did not leave not_read: %v", got["status"])
			}
		})
	}
}

func TestAdsAttributionR12AudienceLeaseBoundary(t *testing.T) {
	for _, state := range []string{"READY", "DISPATCHING"} {
		t.Run(state, func(t *testing.T) {
			x := newATSEnv(t, []string{"read_insights", "pages_read_engagement"})
			op := x.mustPlan()
			if state == "DISPATCHING" {
				var disposition, mode string
				var generation int64
				if err := x.pool.QueryRow(context.Background(), `SELECT disposition,generation,mode FROM integration.claim_operation($1,30,$2)`, op, randomBytes(32)).Scan(&disposition, &generation, &mode); err != nil {
					t.Fatal(err)
				}
				if disposition != "claimed" || mode != "dispatch" {
					t.Fatal("actual operation claim failed")
				}
			}
			if r := x.plan(); r.Status != 200 || r.JSON["operation_id"] != op || r.JSON["state"] != state {
				t.Fatalf("active job/lease did not replay=%v", r.JSON)
			}
			// Disclosed local time/lifecycle fixture, no provider effects.
			if state == "READY" {
				mustExec(t, x.e.h.f.owner, `UPDATE river.river_job SET state='cancelled',finalized_at=clock_timestamp() WHERE id=(SELECT job_id FROM integration.operations WHERE id=$1)`, op)
			} else {
				mustExec(t, x.e.h.f.owner, `UPDATE integration.operations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, op)
			}
			next := x.mustPlan()
			if next == op {
				t.Fatal("expired job/lease prevented fresh GET")
			}
		})
	}
}
