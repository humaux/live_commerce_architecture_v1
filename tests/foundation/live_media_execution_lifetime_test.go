package foundation_test

import (
	"context"
	"net/http"
	"testing"
)

func TestLiveMediaExecutionLME08ReservedRevocationAndDeadlineSemantics(t *testing.T) {
	for _, change := range []string{"principal", "membership", "grant", "tenant", "store", "binding-disable", "binding-version", "prepared-revoke", "start-deadline-only"} {
		t.Run(change, func(t *testing.T) {
			h := lmeSetup(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "direct SQL only", 500) })
			lease := h.claim(t, 30)
			if lease.mode != "dispatch" {
				t.Fatalf("not a reserved dispatch fixture: %+v", lease)
			}
			if err := h.reserve(t, lease); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			var err error
			switch change {
			case "principal":
				_, err = h.lp.f.owner.Exec(ctx, `UPDATE identity.principals SET active=false WHERE id=$1`, h.lp.actor)
				t.Cleanup(func() {
					_, _ = h.lp.f.owner.Exec(context.Background(), `UPDATE identity.principals SET active=true WHERE id=$1`, h.lp.actor)
				})
			case "membership":
				_, err = h.lp.f.owner.Exec(ctx, `UPDATE identity.memberships SET active=false,authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, h.lp.f.tenantA, h.lp.actor)
				t.Cleanup(func() {
					_, _ = h.lp.f.owner.Exec(context.Background(), `UPDATE identity.memberships SET active=true,authz_revision=authz_revision+1 WHERE tenant_id=$1 AND principal_id=$2`, h.lp.f.tenantA, h.lp.actor)
				})
			case "grant":
				_, err = h.lp.f.owner.Exec(ctx, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='live:manage'`, h.lp.f.tenantA, h.lp.f.storeA1, h.lp.actor)
			case "tenant":
				_, err = h.lp.f.owner.Exec(ctx, `UPDATE control.tenants SET active=false WHERE id=$1`, h.lp.f.tenantA)
				t.Cleanup(func() {
					_, _ = h.lp.f.owner.Exec(context.Background(), `UPDATE control.tenants SET active=true WHERE id=$1`, h.lp.f.tenantA)
				})
			case "store":
				_, err = h.lp.f.owner.Exec(ctx, `UPDATE control.stores SET active=false WHERE tenant_id=$1 AND id=$2`, h.lp.f.tenantA, h.lp.f.storeA1)
				t.Cleanup(func() {
					_, _ = h.lp.f.owner.Exec(context.Background(), `UPDATE control.stores SET active=true WHERE tenant_id=$1 AND id=$2`, h.lp.f.tenantA, h.lp.f.storeA1)
				})
			case "binding-disable":
				_, err = h.lp.f.owner.Exec(ctx, `UPDATE integration.bindings SET enabled=false WHERE id=$1`, h.media)
			case "binding-version":
				_, err = h.lp.f.owner.Exec(ctx, `UPDATE integration.bindings SET semantic_version=semantic_version+1 WHERE id=$1`, h.media)
			case "prepared-revoke":
				_, err = h.registrar.Exec(ctx, `SELECT live.revoke_prepared_media($1::uuid,$2::uuid,$3::uuid,$4::text)`, h.lp.f.tenantA, h.lp.f.storeA1, h.input.AuthorizationID, "operator_revoke")
			case "start-deadline-only":
				_, err = h.lp.f.owner.Exec(ctx, `UPDATE live.prepared_media_authorizations SET start_before=clock_timestamp()-interval '1 second' WHERE id=$1`, h.input.AuthorizationID)
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := h.record(t, lease, "START", "EG_lifetime", "EGRESS_ACTIVE", 100, 110, 0)
			if err != nil || out != "observe" {
				t.Fatalf("reserved fact rejected after %s: %s %v", change, out, err)
			}
			f := h.facts(t)
			wantCleanup := change != "start-deadline-only"
			if f.cleanup != wantCleanup || f.operation != "UNKNOWN" || f.resource != "OBSERVED" || f.egress != "EG_lifetime" || f.observations != 1 || h.starts.Load()+h.lists.Load()+h.queries.Load()+h.stops.Load() != 0 {
				t.Fatalf("lifetime %s lost fact or cleanup semantics: %+v wantCleanup=%t", change, f, wantCleanup)
			}
		})
	}
}
