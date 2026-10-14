// Purpose: REAL_PG tests-first coverage of native safety refusal dispositions.
// Depends on: actual C1, frozen plan, native settings/catalog and concrete Executor.
// Used by: OA1 CI; fixture metadata changes never replace native domain writers.
// Invariants: I01/I03/I08/I14; HONORED money/selection stays frozen, current
// safety refusals seal zero-effects ABORTED, and replay keeps the exact receipt.
package foundation_test

import (
	"context"
	"encoding/json"
	"testing"

	"livecommerce/internal/orderattempt"
)

func oaAreaNativeAbortReplay(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, code string) {
	t.Helper()
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	first, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("native refusal must durably seal its typed disposition", err)
	}
	oaExecutorAborted(t, e, a, first, code)
	var noOrphan bool
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT NOT EXISTS(SELECT 1 FROM river.river_job WHERE kind='external_operation_v1')`).Scan(&noOrphan); err != nil || !noOrphan {
		t.Fatal("isolated native abort left an external job, including an unlinked orphan", err)
	}
	before := oaCEAttemptBytes(t, e, a)
	again, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("typed native refusal replay", err)
	}
	oaExecutorSameReceipt(t, first, again)
	oaExecutorAborted(t, e, a, again, code)
	if string(before) != string(oaCEAttemptBytes(t, e, a)) {
		t.Fatal("native refusal replay rewrote its terminal proof")
	}
}

func TestOrderAttemptNativeBankDisableAndPriceVersionHonorFrozenFacts(t *testing.T) {
	e := oaAreaIsolated(t)
	a := oaFenceAdmit(t, e)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	var facts oaBeginFacts
	if err := json.Unmarshal(plan.Facts, &facts); err != nil {
		t.Fatal("original frozen payment facts", err)
	}
	// §2 freezes payment selection, bank/window details and price/version. An
	// enabled switch for new admissions does not delete the selected settings.
	if status, _ := e.cofSettings(1, false, false, 12); status != 200 {
		t.Fatalf("actual merchant bank-settings disable: %d", status)
	}
	mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET price_minor=price_minor+123,version=version+1 WHERE id=$1`, e.money)
	var changed bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT NOT b.enabled AND b.version=2
	 AND b.window_hours=12 AND s.version>1 FROM checkout.bank_transfer_settings b
	 JOIN catalog.skus s ON s.tenant_id=b.tenant_id AND s.store_id=b.store_id
	 WHERE b.tenant_id=$1 AND b.store_id=$2 AND s.id=$3`, e.tenant(), e.store(), e.money).Scan(&changed); err != nil || !changed {
		t.Fatal("actual post-C1 bank/price-version change prerequisite", err)
	}
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	got, err := executor.Execute(context.Background(), e.token(), e.store(), a.AttemptID, a.Generation)
	if err != nil {
		t.Fatal("mutable admission options repriced/revoked frozen intent", err)
	}
	oaExecutorCommitted(t, e, a, plan, got, 5)
	var frozen bool
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT bank_name=$2 AND branch=$3 AND account_name=$4
	 AND account_number=$5 AND window_hours=$6 FROM checkout.bank_transfers WHERE order_id=$1`,
		a.AttemptID, facts.Catalog.Payment.BankName, facts.Catalog.Payment.Branch, facts.Catalog.Payment.AccountName,
		facts.Catalog.Payment.AccountNumber, facts.Catalog.Payment.WindowHours).Scan(&frozen); err != nil || !frozen {
		t.Fatal("current bank disable/version replaced the original bank/window proof", err)
	}
}

func TestOrderAttemptNativeMaxPerOrderRevocationSealsLimitExceeded(t *testing.T) {
	e := oaAreaIsolated(t)
	a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, "bank_transfer", "all-untracked"))
	oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	// Current untracked-SKU safety is REVERIFIED, independently of frozen price.
	mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET max_per_order=4,version=version+1 WHERE id=$1`, e.money)
	var changed bool
	if err := e.p.f.owner.QueryRow(context.Background(), `SELECT NOT inventory_tracked AND max_per_order=4 FROM catalog.skus WHERE id=$1`, e.money).Scan(&changed); err != nil || !changed {
		t.Fatal("current SKU safety revocation prerequisite", err)
	}
	oaAreaNativeAbortReplay(t, e, a, "limit_exceeded")
}

func TestOrderAttemptNativeOfflineCapacitySealsLimitExceeded(t *testing.T) {
	e := oaAreaIsolated(t)
	oaAreaWholeTWDPrice(t, e)
	e.cvsSettings(tcvAllChains, true, "20000", 1)
	selection := oaBeginSelection(t, e, "cash_on_delivery", "duplicate-origins")
	a := oaAllocationAdmit(t, e, selection)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	if plan.Calculation.TotalMinor <= 0 || plan.Calculation.TotalMinor%100 != 0 {
		t.Fatal("native capacity control requires a valid whole-TWD COD amount")
	}
	// A different authenticated principal admits and completes a different owner
	// after this C1. Its real native order fills the store's offline capacity.
	token, principal := e.tcvEnv.member("catalog:read", "inventory:reserve", "orders:read")
	other := *e
	base := *e.tcvEnv
	f := *e.p.f
	f.tokens, f.principalA = map[string]string{"a": token}, principal
	base.p.f = &f
	other.ltgEnv = &ltgEnv{tcvEnv: &base, money: e.money}
	competitor := oaAllocationAdmit(t, &other, selection)
	otherPlan, _ := oaMaterialPlan(t, &other, oaServiceKeys(t), competitor.AttemptID)
	if otherPlan.BuyerOwnerID == plan.BuyerOwnerID || competitor.ScopeID == a.ScopeID {
		t.Fatal("capacity control requires an independent owner and authenticated scope")
	}
	executor := oaExecutorNew(t, e.p.f.runtime, oaServiceKeys(t))
	control, err := executor.Execute(context.Background(), token, e.store(), competitor.AttemptID, competitor.Generation)
	if err != nil {
		t.Fatal("real competing native COD capacity control", err)
	}
	oaExecutorReceipt(t, control, competitor, "COMMITTED", "")
	var full bool
	if err = e.p.f.owner.QueryRow(context.Background(), `SELECT c.pay_at_pickup_max_open=1
	 AND (SELECT count(*) FROM checkout.orders o WHERE o.tenant_id=c.tenant_id AND o.store_id=c.store_id
	 AND o.payment_mode='cash_on_delivery' AND o.collection_state='PENDING' AND o.fulfillment_state='MANUAL_UNASSIGNED')=1
	 AND EXISTS(SELECT 1 FROM checkout.orders WHERE id=$3 AND create_attempt_id=$3 AND owner_id=$4)
	 FROM fulfillment.cvs_store_settings c WHERE c.tenant_id=$1 AND c.store_id=$2`, e.tenant(), e.store(), competitor.AttemptID, otherPlan.BuyerOwnerID).Scan(&full); err != nil || !full {
		t.Fatal("native offline-order capacity positive sentinel", err)
	}
	oaAreaNativeAbortReplay(t, e, a, "limit_exceeded") // native PT429/offline_order_limit
	before := oaAreaCommercialSnapshot(t, e, competitor.AttemptID)
	again, err := executor.Execute(context.Background(), token, e.store(), competitor.AttemptID, competitor.Generation)
	if err != nil {
		t.Fatal(err)
	}
	oaExecutorSameReceipt(t, control, again)
	if string(before) != string(oaAreaCommercialSnapshot(t, e, competitor.AttemptID)) {
		t.Fatal("losing capacity attempt or control replay changed the winner's proof")
	}
}
