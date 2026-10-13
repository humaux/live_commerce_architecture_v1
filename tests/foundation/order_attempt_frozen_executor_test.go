// Purpose: native full-Executor acceptance of frozen money and buyer-cart isolation (6j/M1).
// Depends on: real C1/codec, actual Executor and buyer SetCart, native catalog/service/claim writers.
// Used by: parent OA1 CI; no substituted domain writer, fabricated terminal state or source-scan oracle.
// Invariants: I01/I03/I04/I05/I08/I14; frozen per-origin money, aggregate stock, independent buyer cart,
// definitive safety refusal rolls back all C2 effects before immutable ABORTED settlement.
package foundation_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/orderattempt"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

func oaFrozenBuyerCart(t *testing.T, e *lbuEnv, plan orderattempt.FrozenPlan) storefront.Cart {
	t.Helper()
	// Disclosed native capability fixture for the exact already-resolved owner.
	// Cart creation and its later edit use the actual buyer login and SetCart.
	entropy := sha256.Sum256([]byte(randomUUID()))
	token := base64.RawURLEncoding.EncodeToString(entropy[:])
	digest := sha256.Sum256([]byte(token))
	mustExec(t, e.p.f.owner, `INSERT INTO buyer.capability_sessions
	 (id,tenant_id,store_id,owner_id,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,clock_timestamp()+interval '1 hour')`,
		randomUUID(), e.tenant(), e.store(), plan.BuyerOwnerID, digest[:])
	capability := buyer.Capability{Token: token, Scope: buyer.Scope{StoreID: e.store()}}
	set := func(expected, quantity int64) storefront.Cart {
		cart, err := cqBuyer(e.p.a.runtime, capability, func(ctx context.Context, tx pgx.Tx, scope buyer.Scope) (storefront.Cart, error) {
			if scope.OwnerID != plan.BuyerOwnerID {
				return storefront.Cart{}, buyer.ErrUnauthorized
			}
			return storefront.SetCart(ctx, tx, scope, t04Key("oa-frozen-buyer-cart"), storefront.CartInput{
				ExpectedVersion: expected, Items: []storefront.Item{{SKUID: e.money, Quantity: quantity}},
			})
		})
		if err != nil || cart.ID == "" || cart.Version != expected+1 || len(cart.Items) != 1 || cart.Items[0].Quantity != quantity {
			t.Fatalf("real buyer cart edit sentinel: %v", err)
		}
		return cart
	}
	first := set(0, 1)
	changed := set(first.Version, 2)
	if changed.ID != first.ID {
		t.Fatal("buyer edit replaced its original storefront cart")
	}
	return changed
}

func oaFrozenCartBytes(t *testing.T, e *lbuEnv, cart string) []byte {
	t.Helper()
	var raw []byte
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT jsonb_build_object('cart',to_jsonb(c),
	 'lines',(SELECT jsonb_agg(to_jsonb(l) ORDER BY l.line_ordinal) FROM storefront.cart_lines l WHERE l.cart_id=c.id))
	 FROM storefront.carts c WHERE c.id=$1 AND c.cart_kind='storefront'`, cart).Scan(&raw)
	if err != nil || len(raw) == 0 {
		t.Fatalf("native nonempty buyer cart snapshot: %v", err)
	}
	return raw
}

func TestOrderAttemptFrozenExecutorHonorsPricesFeesAndIndependentBuyerCart(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conversation := e.scenario()
	selection := oaBeginSelection(t, e, "cash_on_delivery", "duplicate-origins")
	items := selection["items"].([]map[string]any)
	items[0]["bundle_id"], items[0]["offer_id"] = bundle, offer.ID
	selection["for"] = map[string]any{"bundle_ids": []string{bundle}, "conversation_id": conversation}
	// Give shipping a positive pre-admission value through the native policy/service CAS.
	in := e.p.delivery
	initialPolicy := dsPolicy(t, e.p.cqHarness, in, 1, 17300, true)
	in.ExpectedVersion, in.PolicyVersion = 1, initialPolicy.Version
	initialService, err := dsSet(e.p.cqHarness, t04Key("oa-frozen-initial-service"), in)
	if err != nil || initialService.Version != 2 {
		t.Fatalf("initial native service/policy sentinel: %v", err)
	}
	if allocation, err := daSet(e.p.cqHarness, t04Key("oa-frozen-initial-allocation"), fulfillment.AllocationInput{
		MarketID: e.p.market.ID, Country: "TW", Code: in.Code, ExpectedVersion: 1,
		ExpectedServiceVersion: initialService.Version, WarehouseIDs: []string{e.p.stock.warehouse.ID},
	}); err != nil || allocation.Version != 2 {
		t.Fatalf("initial native allocation/service consistency: %v", err)
	}
	a := oaAllocationAdmit(t, e, selection)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	var facts struct {
		Catalog oaFactsResult `json:"catalog"`
		Claims  oaClaimResult `json:"claims"`
	}
	if err = json.Unmarshal(plan.Facts, &facts); err != nil || len(facts.Catalog.Lines) != 2 || len(facts.Claims.Lines) != 2 ||
		facts.Claims.Lines[0].LivePriceMinor == nil || *facts.Claims.Lines[0].LivePriceMinor != ltgLive ||
		facts.Claims.Lines[1].LivePriceMinor != nil || plan.Calculation.ShippingMinor != 17300 ||
		facts.Catalog.Payment.SurchargeMinor != 3500 || facts.Catalog.Lines[1].UnitPriceMinor != ltgCatalog {
		t.Fatalf("real C1 mixed-origin/fee positive sentinel: %v", err)
	}
	cart := oaFrozenBuyerCart(t, e, plan)
	beforeCart := oaFrozenCartBytes(t, e, cart.ID)
	// Post-C1 merchant edits use native CAS for policy/service. Owner fixtures edit
	// the source prices/settings only; frozen plan/ciphertext/attempt are untouched.
	mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET price_minor=price_minor+12000,version=version+1 WHERE id=$1`, e.money)
	mustExec(t, e.p.f.owner, `UPDATE live.offers SET live_price_minor=live_price_minor+7000 WHERE id=$1`, offer.ID)
	fee, rate := int64(23700), int64(500)
	policy := e.p.policy
	policy.Method, policy.ExpectedVersion, policy.ShippingMinor = facts.Catalog.Policy.Method, facts.Catalog.Policy.Version, &fee
	policy.TaxMode, policy.TaxBasis, policy.TaxRateBPS, policy.QuoteTTLSeconds = "exclusive", "goods_and_shipping", &rate, 60
	changedPolicy, err := e.p.setPolicy(policy)
	if err != nil || changedPolicy.Version != facts.Catalog.Policy.Version+1 {
		t.Fatalf("native post-C1 policy edit: %v", err)
	}
	in.ExpectedVersion, in.PolicyVersion, in.Visible = facts.Catalog.Service.Version, changedPolicy.Version, false
	if changed, err := dsSet(e.p.cqHarness, t04Key("oa-frozen-service-drift"), in); err != nil || changed.Version != facts.Catalog.Service.Version+1 {
		t.Fatalf("native post-C1 fee/service edit: %v", err)
	}
	mustExec(t, e.p.f.owner, `UPDATE checkout.cash_on_delivery_settings SET surcharge_twd=12,version=version+1 WHERE store_id=$1`, e.store())
	// Make M1 distinguishable: current sources imply different money, not merely
	// different version numbers. This independent arithmetic is not used to place.
	current, err := pricing.Calculate(changedPolicy, []pricing.AmountLine{{UnitPriceMinor: ltgLive + 7000, Quantity: 2}, {UnitPriceMinor: ltgCatalog + 12000, Quantity: 3}})
	if err != nil || current.TotalMinor == plan.Calculation.TotalMinor || current.ShippingMinor == plan.Calculation.ShippingMinor {
		t.Fatalf("current-price mutation oracle is ineffective: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, holder)
	var locked string
	if err = holder.QueryRow(ctx, `SELECT id::text FROM storefront.carts WHERE id=$1 FOR UPDATE`, cart.ID).Scan(&locked); err != nil || locked != cart.ID {
		t.Fatal("buyer cart row-lock positive sentinel", err)
	}
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	got, err := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("full Executor must honor C1 money while the buyer cart is locked", err)
	}
	oaExecutorReceipt(t, got, a, "COMMITTED", "")
	var receipt oaSettlementReceipt
	var fields, send map[string]json.RawMessage
	if json.Unmarshal(got.Receipt, &receipt) != nil || json.Unmarshal(got.Receipt, &fields) != nil ||
		json.Unmarshal(fields["send"], &send) != nil || len(fields) != 9 ||
		receipt.CommercialState != "AWAITING_COLLECTION" || receipt.PaymentMode != "cash_on_delivery" ||
		receipt.TotalMinor != plan.Calculation.TotalMinor || receipt.Currency != "TWD" || receipt.Send.State != "not_sent" ||
		(len(send) != 1 && !(len(send) == 2 && receipt.Send.Reason == "not_requested")) {
		t.Fatal("full committed receipt replaced frozen money/payment or leaked extra fields")
	}
	var exact bool
	var snapshotRaw []byte
	err = e.p.f.owner.QueryRow(ctx, `SELECT a.state='COMMITTED' AND a.receipt=$2::jsonb AND a.order_id=o.id
	 AND a.plan_ciphertext IS NULL AND a.plan_key_version IS NULL AND a.settled_at IS NOT NULL
	 AND o.id=a.attempt_id AND o.owner_id=a.buyer_owner_id AND o.creator_kind='order_attempt'
	 AND o.create_attempt_id=a.attempt_id AND o.creator_session_id IS NULL AND o.source='merchant_manual'
	 AND o.commercial_state='AWAITING_COLLECTION' AND o.collection_state='PENDING'
	 AND o.payment_mode='cash_on_delivery' AND o.total_minor=$3 AND o.cod_surcharge_minor=$4
	 AND r.state='COMMITTED' AND r.create_attempt_id=a.attempt_id AND r.buyer_session_id IS NULL
	 AND c.cart_kind='order_attempt' AND c.owner_id=a.buyer_owner_id AND c.id<>$5::uuid
	 AND (SELECT count(*) FROM storefront.cart_lines WHERE cart_id=c.id)=2
	 AND (SELECT sum(quantity) FROM inventory.reservation_lines WHERE reservation_id=o.id AND sku_id=$6)=5
	 AND (SELECT sum(delta_allocated) FROM inventory.ledger WHERE create_attempt_id=a.attempt_id AND sku_id=$6)=5
	 AND EXISTS(SELECT 1 FROM checkout.order_pay_entries p WHERE p.order_id=o.id AND p.create_attempt_id=a.attempt_id AND p.revoked_at IS NULL)
	 AND EXISTS(SELECT 1 FROM claims.live_price_uses u WHERE u.order_id=o.id AND u.bundle_id=$7 AND u.offer_id=$8 AND u.quantity=2 AND u.unit_price_minor=$9)
	 AND NOT EXISTS(SELECT 1 FROM ops.order_attempt_subjects s WHERE s.attempt_id=a.attempt_id),o.snapshot
	 FROM ops.order_attempts a JOIN checkout.orders o ON o.id=a.order_id
	 JOIN inventory.reservations r ON r.id=o.id JOIN storefront.carts c ON c.id=o.cart_id WHERE a.attempt_id=$1`,
		a.AttemptID, string(got.Receipt), plan.Calculation.TotalMinor, facts.Catalog.Payment.SurchargeMinor, cart.ID, e.money, bundle, offer.ID, ltgLive).
		Scan(&exact, &snapshotRaw)
	if err != nil || !exact {
		t.Fatalf("durable native C2 frozen money/origins/stock/creator sentinel: %v", err)
	}
	var snapshot checkout.Snapshot
	if json.Unmarshal(snapshotRaw, &snapshot) != nil || !reflect.DeepEqual(snapshot.Quote.Amount, plan.Calculation) ||
		!reflect.DeepEqual(snapshot.Quote.Policy, facts.Catalog.Policy) || !reflect.DeepEqual(snapshot.Service, facts.Catalog.Service) ||
		snapshot.ExpectedCodSurchargeMinor == nil || *snapshot.ExpectedCodSurchargeMinor != facts.Catalog.Payment.SurchargeMinor || len(snapshot.Quote.Lines) != 2 {
		t.Fatal("native snapshot repriced current policy/service/COD surcharge")
	}
	for i, line := range snapshot.Quote.Lines {
		want := facts.Catalog.Lines[i]
		price := want.UnitPriceMinor
		if i == 0 {
			price = *facts.Claims.Lines[i].LivePriceMinor
			if line.PriceRule != pricing.RuleLiveClaim || line.ClaimBundleID != bundle || line.ClaimOfferID != offer.ID {
				t.Fatal("live-price origin evidence lost in full Executor snapshot")
			}
		}
		if line.SKUID != e.money || line.OriginID != want.OriginID || line.LineOrdinal != int64(i+1) ||
			line.LineID == "" || line.Quantity != want.Quantity || line.UnitPriceMinor != price || !reflect.DeepEqual(line.Amount, plan.Calculation.Lines[i]) {
			t.Fatal("duplicate SKU collapsed or per-origin frozen price/quantity changed")
		}
	}
	if snapshot.Quote.Lines[0].OriginID == snapshot.Quote.Lines[1].OriginID || snapshot.Quote.Lines[0].LineID == snapshot.Quote.Lines[1].LineID ||
		!bytes.Equal(beforeCart, oaFrozenCartBytes(t, e, cart.ID)) {
		t.Fatal("attempt merged origins or mutated the same owner's real buyer cart")
	}
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, got, again)
}

func TestOrderAttemptFrozenExecutorSafetyRefusalsAbortWithoutEffects(t *testing.T) {
	// Stock exhaustion already has a full-Executor terminal/zero-effects gate in
	// TestOrderAttemptExecutorInsufficientStockAbortsWithoutEffects; do not duplicate it.
	for _, change := range []string{"sku-disabled", "service-disabled"} {
		t.Run(change, func(t *testing.T) {
			e := lbuNew(t)
			a := oaFenceAdmit(t, e)
			code := "resource_unavailable"
			if change == "sku-disabled" {
				mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET status='archived' WHERE id=$1`, e.money)
			} else {
				in := e.p.delivery
				in.ExpectedVersion, in.Enabled = 1, false
				if out, err := dsSet(e.p.cqHarness, t04Key("oa-frozen-disabled-service"), in); err != nil || out.Enabled {
					t.Fatalf("native disabled service sentinel: %v", err)
				}
				code = "invalid_destination"
			}
			executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			got, err := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
			if err != nil {
				t.Fatal("definitive native safety refusal must settle after rolling C2 back", err)
			}
			oaExecutorAborted(t, e, a, got, code)
			again, err := executor.Execute(ctx, e.token(), e.store(), a.AttemptID, a.Generation)
			if err != nil {
				t.Fatal(err)
			}
			oaExecutorSameReceipt(t, got, again)
		})
	}
}
