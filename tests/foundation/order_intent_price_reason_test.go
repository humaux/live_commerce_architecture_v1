// Purpose: REAL_PG partial order replay preserves the original Quote's mixed catalog/live price warning.
// Depends on: lbuNew, real claims, immutable checkout Quote/live_price_uses and the ForBuyer FaultAfterPlace seam.
// Used by: foundation CI TestOrderIntentReplayPreservesOriginalPriceReason; owner requires CI-only execution.
// Invariants: I02/I05/I08; replay cannot re-Quote, infer original prices from current claims, duplicate stock holds or lose durable price provenance.
package foundation_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"livecommerce/internal/claims"
)

func TestOrderIntentReplayPreservesOriginalPriceReason(t *testing.T) {
	for _, tc := range []struct {
		name    string
		partial bool
		mixed   bool
	}{{"fresh_mixed", false, true}, {"partial_mixed", true, true}, {"partial_all_live", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			e := lbuNew(t)
			beforeOrders, beforeHeld := e.orders(), e.reserved()
			session, liveOffer, bundle, conversation := e.scenario()
			bundles := []string{bundle}
			body := e.body(2, bundles, conversation, false)
			catalogSKU, secondCatalogBundle := "", ""
			if tc.mixed {
				catalogSKU = e.sku(ltgCatalog, 500)
				e.h.open(t, session, claims.MatchExact)
				e.h.livePriceOffer(t, session, "B1", catalogSKU, 5, ltgLive)
				// One B1 origin alone covers the requested aggregate, but two distinct B1 origins force catalog pricing.
				e.h.accepted(t, session, bundle, "", "B1+2")
				secondCatalogBundle = e.h.accepted(t, session, "", "extra-"+t04Tag(), "B1+1").BundleID
				e.h.closeWindow(t, session)
				e.linkPeer(secondCatalogBundle, conversation)
				bundles = append(bundles, secondCatalogBundle)
				body = e.body(2, bundles, conversation, false)
				body["items"] = []map[string]any{{"sku_id": e.money, "quantity": 2}, {"sku_id": catalogSKU, "quantity": 2}}
			}
			key := t04Key("oir-price")
			if tc.partial {
				e.fb.FaultAfterPlace = func() error { return errors.New("injected death before for-buyer upper receipt") }
			}
			status, out := e.post(e.token(), key, body)
			e.fb.FaultAfterPlace = nil
			if (tc.partial && (status != 503 || out["code"] != "retry_later")) || (!tc.partial && status != 201) {
				t.Fatalf("real placement barrier: status=%d code=%v", status, out["code"])
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var order string
			if err := e.p.f.owner.QueryRow(ctx, `SELECT order_id::text FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2
 AND operation='merchanttools.order.manual' AND idempotency_key=$3`, e.tenant(), e.store(), key).Scan(&order); err != nil {
				t.Fatal("barrier has no original completed manual receipt")
			}
			originalSnapshot, originalQuote := oiprReadSnapshot(t, e, order)
			quoteCount := e.count(`SELECT count(*) FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store())
			if e.orders() != beforeOrders+1 || e.reserved() != beforeHeld+2 || e.plan.planned() != 0 || e.count(`SELECT count(*) FROM claims.live_price_uses WHERE tenant_id=$1 AND store_id=$2
 AND order_id=$3 AND bundle_id=$4 AND offer_id=$5 AND quantity=2 AND unit_price_minor=$6`, e.tenant(), e.store(), order, bundle, liveOffer.ID, ltgLive) != 1 {
				t.Fatal("original placement lacks its exact order, stock hold or consumed live-price provenance")
			}
			wantCatalog := 0
			wantReason := ""
			if tc.mixed {
				wantCatalog, wantReason = 1, "some_lines_catalog"
				if e.count(`SELECT count(*) FROM checkout.orders o CROSS JOIN LATERAL jsonb_array_elements(o.snapshot#>'{quote,lines}') x
 WHERE o.tenant_id=$1 AND o.store_id=$2 AND o.id=$3 AND x->>'sku_id'=$4 AND x->>'price_rule' IS DISTINCT FROM 'live_claim'
 AND (x->>'unit_price_minor')::bigint=$5 AND x->>'claim_bundle_id' IS NULL`, e.tenant(), e.store(), order, catalogSKU, ltgCatalog) != 1 {
					t.Fatal("original multi-origin line did not fail closed to catalog in the immutable Quote")
				}
				if tc.partial {
					// Disclosed bounded owner fixture: a later purge hides one current candidate; original Quote and consumption remain unchanged.
					tag, err := e.p.f.owner.Exec(ctx, `UPDATE claims.bundles SET purged_at=clock_timestamp(),owner_id=NULL,bound_at=NULL,
 label='purged-'||replace(id::text,'-','') WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND purged_at IS NULL`, e.tenant(), e.store(), secondCatalogBundle)
					if err != nil || tag.RowsAffected() != 1 {
						t.Fatal("later current-candidate fixture did not hide exactly the second catalog origin")
					}
				}
			}
			if e.count(`SELECT count(*) FROM checkout.orders o CROSS JOIN LATERAL jsonb_array_elements(o.snapshot#>'{quote,lines}') x
 WHERE o.tenant_id=$1 AND o.store_id=$2 AND o.id=$3 AND x->>'price_rule' IS DISTINCT FROM 'live_claim'`, e.tenant(), e.store(), order) != wantCatalog {
				t.Fatal("original Quote has an unexpected catalog line count")
			}
			for replay := 0; replay < 3; replay++ {
				status, out = e.post(e.token(), key, body)
				wantStatus := 200
				if tc.partial && replay == 0 {
					wantStatus = 201
				}
				if status != wantStatus || out["order_id"] != order || out["live_price"] != "applied" || out["live_price_reason"] != wantReason {
					t.Fatalf("original price reason replay %d: status=%d live=%v reason=%v code=%v", replay, status, out["live_price"], out["live_price_reason"], out["code"])
				}
				snapshot, quote := oiprReadSnapshot(t, e, order)
				if !bytes.Equal(snapshot, originalSnapshot) || quote != originalQuote || e.orders() != beforeOrders+1 || e.reserved() != beforeHeld+2 || e.plan.planned() != 0 || e.count(`SELECT count(*) FROM storefront.quotes WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()) != quoteCount {
					t.Fatal("price reason recovery rewrote its Quote or duplicated checkout, stock, Quote or DM")
				}
				if e.count(`SELECT count(*) FROM claims.live_price_uses WHERE tenant_id=$1 AND store_id=$2 AND order_id=$3 AND quantity=2 AND unit_price_minor=$4`, e.tenant(), e.store(), order, ltgLive) != 1 || e.count(`SELECT count(*) FROM ops.command_results WHERE tenant_id=$1 AND store_id=$2 AND operation='merchanttools.order.for_buyer'
 AND idempotency_key=$3 AND response->>'live_price_reason'=$4`, e.tenant(), e.store(), key, wantReason) != 1 {
					t.Fatal("upper replay did not retain the original ledger and durable price reason exactly once")
				}
			}
		})
	}
}

func oiprReadSnapshot(t *testing.T, e *lbuEnv, order string) ([]byte, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var snapshot []byte
	var quote string
	if err := e.p.f.owner.QueryRow(ctx, `SELECT snapshot,quote_id::text FROM checkout.orders WHERE tenant_id=$1 AND store_id=$2 AND id=$3`, e.tenant(), e.store(), order).Scan(&snapshot, &quote); err != nil {
		t.Fatal("original immutable Quote read failed")
	}
	return snapshot, quote
}
