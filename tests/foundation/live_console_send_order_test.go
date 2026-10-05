package foundation_test

// Red/green for the integrator follow-up on LC-B4: the recommend comment is rendered from catalog facts, so a content-forbidden product name must
// not mask a severed connection. Precedence is the same as the other three planners: capability first, then content.

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/platform"
)

func TestLiveConsoleSendRecommendCapabilityBeforeContent(t *testing.T) {
	e := lbSetup(t)
	e.onlySource(t, "facebook")
	f := e.h.f
	var old string
	if err := f.owner.QueryRow(context.Background(), `SELECT p.name FROM catalog.products p JOIN catalog.skus k ON k.tenant_id=p.tenant_id AND k.store_id=p.store_id AND k.product_id=p.id WHERE k.id=$1`, e.offer.SKUID).Scan(&old); err != nil {
		t.Fatal(err)
	}
	set := func(name string) {
		mustExec(t, f.owner, `UPDATE catalog.products SET name=$2 WHERE (tenant_id,store_id,id)=(SELECT p.tenant_id,p.store_id,p.id FROM catalog.products p JOIN catalog.skus k ON k.tenant_id=p.tenant_id AND k.store_id=p.store_id AND k.product_id=p.id WHERE k.id=$1)`, e.offer.SKUID, name)
	}
	t.Cleanup(func() { set(old) })
	set("shop.example.com 0912345678 https://x.example.com") // content-forbidden once rendered (§3.5)
	plan := func() error {
		return e.scoped(t, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
			_, err := e.svc.PlanRecommend(ctx, tx, s, e.session, e.offer.ID, e.offer.Version)
			return err
		})
	}
	// connected: the content rule applies
	if err := plan(); planCode(err) != "public_reply_forbidden_content" {
		t.Fatalf("connected, forbidden product name: %v", err)
	}
	// severed: capability wins
	mustExec(t, f.owner, `UPDATE integration.meta_connections SET status='reauth_required', updated_at=clock_timestamp() WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, f.tenantA, f.storeA1, e.pageID)
	defer mustExec(t, f.owner, `UPDATE integration.meta_connections SET status='active', updated_at=clock_timestamp()+interval '0 seconds' WHERE tenant_id=$1 AND store_id=$2 AND page_id=$3`, f.tenantA, f.storeA1, e.pageID)
	if err := plan(); planCode(err) != "capability" {
		t.Fatalf("severed, forbidden product name: %v (want capability)", err)
	}
	_ = time.Second
}
