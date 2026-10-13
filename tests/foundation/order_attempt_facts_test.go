// Purpose: REAL_PG acceptance of the order-attempt-v1 C1 locked admission facts seam.
// Depends on: lbuNew fixtures, native ops candidates, checkout admission definer and pgx.
// Used by: OA1 focused CI; the fixture owner prepares candidates, the coordinator calls facts.
// Invariants: I01/I02/I08; catalog prices preserve each explicit origin; runtime cannot call;
// helper scope is restored and its payment facts contain each fee exactly once.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/pricing"
	"livecommerce/internal/storefront"
)

type oaFactsLine struct {
	storefront.QuoteLine
	Ordinal  int    `json:"ordinal"`
	OriginID string `json:"origin_id"`
}

type oaFactsResult struct {
	Market     pricing.Market         `json:"market"`
	Policy     pricing.Policy         `json:"policy"`
	Lines      []oaFactsLine          `json:"lines"`
	Service    fulfillment.Service    `json:"service"`
	Allocation fulfillment.Allocation `json:"allocation"`
	Payment    struct {
		Mode           string `json:"mode"`
		WindowHours    int    `json:"window_hours"`
		Version        int64  `json:"version"`
		SurchargeMinor int64  `json:"surcharge_minor"`
		MaxMinor       int64  `json:"max_minor"`
		Carrier        string `json:"carrier"`
	} `json:"payment"`
}

func oaFactsSelection(e *lbuEnv) map[string]any {
	return map[string]any{
		"schema_version": 1,
		"items": []map[string]any{
			{"sku_id": e.money, "quantity": 2, "ordinal": 1, "origin_id": randomUUID()},
			{"sku_id": e.money, "quantity": 3, "ordinal": 2, "origin_id": randomUUID()},
		},
		"delivery": map[string]any{"market_id": e.p.market.ID, "country": "TW", "service_code": "home",
			"home_address": map[string]any{"region": "Synthetic", "city": "Synthetic", "postal_code": "100", "line1": "Synthetic address", "line2": ""}},
		"customer": map[string]any{"name": "Synthetic Buyer", "phone": "0912345678", "email": "synthetic@example.invalid"},
		"payment_mode": "bank_transfer", "locale": "en",
		"for": map[string]any{"bundle_ids": []string{}, "conversation_id": nil}, "send_payment_link": false,
	}
}

func oaFactsCandidate(t *testing.T, e *lbuEnv, selection map[string]any) (pgx.Tx, string) {
	t.Helper()
	tx, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	attempt, scope, owner := randomUUID(), randomUUID(), randomUUID()
	raw, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id',$1,true),set_config('app.store_id',$2,true),
	 set_config('app.principal_id',$3,true),set_config('app.buyer_id','',true),set_config('app.buyer_session_id','',true)`, e.tenant(), e.store(), e.p.f.principalA)
	oaStateExec(t, tx, `INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES($1,$2,$3)`, owner, e.tenant(), e.store())
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_creators(attempt_id,tenant_id,store_id,buyer_owner_id) VALUES($1,$2,$3,$4)`, attempt, e.tenant(), e.store(), owner)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempt_scopes(scope_id,tenant_id,store_id,principal_id,current_generation)
	 VALUES($1,$2,$3,$4,1)`, scope, e.tenant(), e.store(), e.p.f.principalA)
	oaStateExec(t, tx, `INSERT INTO ops.order_attempts(attempt_id,tenant_id,store_id,scope_id,intent_generation,
	 transport_key_digest,request_hash,request_schema_version,buyer_owner_id,admission_provenance,admitted_at,execute_by,state)
	 VALUES($1,$2,$3,$4,1,sha256($1::uuid::text::bytea),sha256(convert_to(($6::jsonb)::text,'UTF8')),1,$5,'{}',
	 statement_timestamp(),statement_timestamp()+interval '15 minutes','PENDING')`, attempt, e.tenant(), e.store(), scope, owner, string(raw))
	// No sealing or constraint disabling: the candidate and facts remain in C1's one Tx.
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	return tx, attempt
}

func oaFactsCall(tx pgx.Tx, attempt string, selection map[string]any) (oaFactsResult, error) {
	raw, err := json.Marshal(selection)
	if err != nil {
		return oaFactsResult{}, err
	}
	var output []byte
	// Calls checkout.order_attempt_admission_facts, order-attempt-v1 §2 C1.
	err = tx.QueryRow(context.Background(), `SELECT checkout.order_attempt_admission_facts($1::uuid,$2::jsonb)`, attempt, string(raw)).Scan(&output)
	var out oaFactsResult
	if err == nil {
		err = json.Unmarshal(output, &out)
	}
	return out, err
}

func oaFactsReject(t *testing.T, tx pgx.Tx, attempt string, selection map[string]any, code, message string) {
	t.Helper()
	probe, err := tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, err = oaFactsCall(probe, attempt, selection)
	rollbackErr := probe.Rollback(context.Background())
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != code || (message != "" && pe.Message != message) || rollbackErr != nil {
		t.Fatalf("facts refusal want %s/%s: err=%v rollback=%v", code, message, err, rollbackErr)
	}
}

func TestOrderAttemptFactsCatalogOriginsAndPayment(t *testing.T) {
	e := lbuNew(t)
	selection := oaFactsSelection(e)
	tx, attempt := oaFactsCandidate(t, e, selection)
	out, err := oaFactsCall(tx, attempt, selection)
	if err != nil {
		t.Fatal(err)
	}
	if out.Market.ID != e.p.market.ID || out.Market.Currency != "TWD" || !out.Market.Active ||
		out.Policy.Method != "delivery:home" || out.Policy.Version != 1 || out.Policy.ShippingMinor != 0 || out.Policy.TaxRateBPS != 0 ||
		out.Service.Code != "home" || !out.Service.Enabled || out.Allocation.Version != 1 || len(out.Allocation.WarehouseIDs) != 1 ||
		out.Allocation.WarehouseIDs[0] != e.p.stock.warehouse.ID || out.Payment.Mode != "bank_transfer" || out.Payment.WindowHours != 72 || out.Payment.Version < 1 {
		t.Fatalf("facts DTO/payment/fee sentinel: %+v", out)
	}
	items := selection["items"].([]map[string]any)
	if len(out.Lines) != 2 {
		t.Fatalf("duplicate origins collapsed: %+v", out.Lines)
	}
	for i, line := range out.Lines {
		if line.SKUID != e.money || line.Quantity != int64(2+i) || line.UnitPriceMinor != 30000 || line.PriceRule != "" ||
			line.Ordinal != i+1 || line.OriginID != items[i]["origin_id"] || line.ProductID == "" || line.SKUVersion < 1 {
			t.Fatalf("origin/catalog price %d: %+v", i, line)
		}
	}
	var tenant, store, principal, buyer, session string
	err = tx.QueryRow(context.Background(), `SELECT current_setting('app.tenant_id'),current_setting('app.store_id'),
	 current_setting('app.principal_id'),current_setting('app.buyer_id'),current_setting('app.buyer_session_id')`).Scan(&tenant, &store, &principal, &buyer, &session)
	if err != nil || tenant != e.tenant() || store != e.store() || principal != e.p.f.principalA || buyer != "" || session != "" {
		t.Fatalf("facts scope not restored: err=%v", err)
	}
}

func TestOrderAttemptFactsProvenanceAndRuntime(t *testing.T) {
	e := lbuNew(t)
	selection := oaFactsSelection(e)
	tx, attempt := oaFactsCandidate(t, e, selection)
	changed := oaFactsSelection(e)
	oaFactsReject(t, tx, attempt, changed, "PT409", "idempotency_conflict")
	oaStateExec(t, tx, `SELECT set_config('app.store_id',$1,true)`, randomUUID())
	oaFactsReject(t, tx, attempt, selection, "PT404", "attempt_not_found")
	oaStateExec(t, tx, `SELECT set_config('app.store_id',$1,true)`, e.store())
	if _, err := oaFactsCall(tx, attempt, selection); err != nil {
		t.Fatalf("post-refusal scope recovery: %v", err)
	}
	for _, role := range []string{"commerce_runtime", "commerce_checkout_runtime", "commerce_claims_writer"} {
		probe, err := tx.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		oaStateExec(t, probe, `RESET ROLE`)
		oaStateExec(t, probe, `SET LOCAL ROLE `+role)
		oaFactsReject(t, probe, attempt, selection, "42501", "")
		_ = probe.Rollback(context.Background())
	}
}

func TestOrderAttemptFactsEscapedCanonicalInput(t *testing.T) {
	e := lbuNew(t)
	selection := oaFactsSelection(e)
	selection["customer"].(map[string]any)["name"] = `Synthetic "Buyer"`
	selection["delivery"].(map[string]any)["home_address"].(map[string]any)["line1"] = `Synthetic \\ address`
	// Canonical UTF8 hashing must preserve JSON escapes. Casting text to bytea
	// would interpret those escapes before hashing or refuse legitimate input.
	tx, attempt := oaFactsCandidate(t, e, selection)
	out, err := oaFactsCall(tx, attempt, selection)
	if err != nil || len(out.Lines) != 2 || out.Payment.Mode != "bank_transfer" {
		t.Fatalf("escaped canonical selection refused: err=%v", err)
	}
}

func TestOrderAttemptFactsRefusals(t *testing.T) {
	for _, name := range []string{"extra_price", "bad_quantity", "missing_sku", "missing_market", "unsupported_payment", "duplicate_ordinal", "zero_ordinal", "currency", "inactive", "bank_disabled"} {
		t.Run(name, func(t *testing.T) {
			e := lbuNew(t)
			selection := oaFactsSelection(e)
			items := selection["items"].([]map[string]any)
			code, message := "PT400", "invalid_selection"
			switch name {
			case "extra_price":
				items[0]["unit_price_minor"] = 1
			case "bad_quantity":
				items[0]["quantity"] = 0
			case "missing_sku":
				items[0]["sku_id"] = randomUUID()
				code, message = "PT404", "sku_not_found"
			case "missing_market":
				selection["delivery"].(map[string]any)["market_id"] = randomUUID()
				code, message = "PT404", "market_not_found"
			case "unsupported_payment":
				selection["payment_mode"] = "card"
				code, message = "PT422", "payment_mode_unavailable"
			case "duplicate_ordinal":
				items[1]["ordinal"] = 1
			case "zero_ordinal":
				items[0]["ordinal"] = 0
			case "currency":
				code, message = "PT422", "currency_mismatch"
			case "inactive":
				code, message = "PT422", "sku_inactive"
			case "bank_disabled":
				code, message = "PT422", "bank_transfer_unavailable"
			}
			tx, attempt := oaFactsCandidate(t, e, selection)
			if name == "currency" || name == "inactive" || name == "bank_disabled" {
				oaStateExec(t, tx, `RESET ROLE`)
				if name == "currency" {
					oaStateExec(t, tx, `UPDATE catalog.skus SET currency='USD' WHERE id=$1`, e.money)
				}
				if name == "inactive" {
					oaStateExec(t, tx, `UPDATE catalog.skus SET status='archived' WHERE id=$1`, e.money)
				}
				if name == "bank_disabled" {
					oaStateExec(t, tx, `UPDATE checkout.bank_transfer_settings SET enabled=false WHERE store_id=$1`, e.store())
				}
				oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
			}
			oaFactsReject(t, tx, attempt, selection, code, message)
		})
	}
}

func TestOrderAttemptFactsCODFee(t *testing.T) {
	e := lbuNew(t)
	status, _ := e.hcodSettings(0, true, 20000, 35, "black_cat")
	if status != 200 {
		t.Fatalf("COD fixture status %d", status)
	}
	selection := oaFactsSelection(e)
	selection["payment_mode"] = "cash_on_delivery"
	tx, attempt := oaFactsCandidate(t, e, selection)
	out, err := oaFactsCall(tx, attempt, selection)
	if err != nil {
		t.Fatal(err)
	}
	if out.Payment.Mode != "cash_on_delivery" || out.Payment.SurchargeMinor != 3500 || out.Payment.MaxMinor != 2000000 || out.Payment.Carrier != "black_cat" {
		t.Fatalf("COD frozen fees %+v", out.Payment)
	}
	var amountLines []pricing.AmountLine
	for _, line := range out.Lines {
		amountLines = append(amountLines, pricing.AmountLine{UnitPriceMinor: line.UnitPriceMinor, Quantity: line.Quantity})
	}
	calculation, err := pricing.CalculateWith(out.Policy, amountLines, nil)
	if err != nil || calculation.TotalMinor != 150000 || calculation.TotalMinor+out.Payment.SurchargeMinor != 153500 {
		t.Fatalf("COD fee must be added once: quote=%+v err=%v", calculation, err)
	}
}

func TestOrderAttemptFactsShippingTaxAndLocks(t *testing.T) {
	e := lbuNew(t)
	h := e.p.cqHarness
	code := "oa" + t04Tag()
	fee, rate := int64(1200), int64(500)
	policy := h.policy
	policy.Method, policy.ShippingMinor, policy.TaxRateBPS = "delivery:"+code, &fee, &rate
	policy.TaxMode, policy.TaxBasis = "exclusive", "goods_and_shipping"
	if _, err := h.setPolicy(policy); err != nil {
		t.Fatal(err)
	}
	service := fulfillment.ServiceInput{MarketID: h.market.ID, Country: "TW", Code: code, PolicyVersion: 1,
		NameHans: "Synthetic", NameHant: "Synthetic", NameEN: "Synthetic", DeliveryKind: "home", Mode: "MANUAL", Enabled: true, Visible: true}
	if _, err := dsSet(h, t04Key("oa-facts-service"), service); err != nil {
		t.Fatal(err)
	}
	if _, err := daSet(h, t04Key("oa-facts-allocation"), fulfillment.AllocationInput{MarketID: h.market.ID, Country: "TW", Code: code,
		ExpectedServiceVersion: 1, WarehouseIDs: []string{h.stock.warehouse.ID}}); err != nil {
		t.Fatal(err)
	}
	selection := oaFactsSelection(e)
	selection["delivery"].(map[string]any)["service_code"] = code
	tx, attempt := oaFactsCandidate(t, e, selection)
	out, err := oaFactsCall(tx, attempt, selection)
	if err != nil {
		t.Fatal(err)
	}
	var lines []pricing.AmountLine
	for _, line := range out.Lines {
		lines = append(lines, pricing.AmountLine{UnitPriceMinor: line.UnitPriceMinor, Quantity: line.Quantity})
	}
	calculation, err := pricing.CalculateWith(out.Policy, lines, nil)
	if err != nil || calculation.SubtotalMinor != 150000 || calculation.ShippingMinor != 1200 ||
		calculation.TaxMinor != 7560 || calculation.ShippingTaxMinor != 60 || calculation.TotalMinor != 158760 {
		t.Fatalf("all frozen fee inputs: %+v err=%v", calculation, err)
	}
	other, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback(context.Background())
	// NOWAIT makes this a deterministic lock oracle: each independent transaction
	// must see the C1 fact lock before C1 returns to its caller for pricing/sealing.
	for _, query := range []string{
		`SELECT id FROM pricing.markets WHERE id=$1::uuid FOR UPDATE NOWAIT`,
		`SELECT market_id FROM pricing.policy_heads WHERE market_id=$1::uuid AND method='delivery:'||$2::text FOR UPDATE NOWAIT`,
		`SELECT market_id FROM fulfillment.service_heads WHERE market_id=$1::uuid AND code=$2::text FOR UPDATE NOWAIT`,
		`SELECT market_id FROM fulfillment.allocation_heads WHERE market_id=$1::uuid AND code=$2::text FOR UPDATE NOWAIT`,
	} {
		args := []any{h.market.ID}
		if strings.Contains(query, "$2") {
			args = append(args, code)
		}
		oaStateReject(t, other, "55P03", query, args...)
	}
	oaStateReject(t, other, "55P03", `SELECT id FROM catalog.skus WHERE id=$1 FOR UPDATE NOWAIT`, e.money)
	oaStateReject(t, other, "55P03", `SELECT id FROM catalog.products WHERE id=(SELECT product_id FROM catalog.skus WHERE id=$1) FOR UPDATE NOWAIT`, e.money)
	oaStateReject(t, other, "55P03", `SELECT store_id FROM checkout.bank_transfer_settings WHERE store_id=$1 FOR UPDATE NOWAIT`, e.store())
}

func TestOrderAttemptFactsFunctionACL(t *testing.T) {
	f := fixture(t)
	var owner, config string
	var definer, runtime, coordinator bool
	err := f.owner.QueryRow(context.Background(), `SELECT pg_get_userbyid(proowner),prosecdef,coalesce(array_to_string(proconfig,','),''),
	 has_function_privilege('commerce_runtime',oid,'EXECUTE'),has_function_privilege('commerce_order_attempt_owner',oid,'EXECUTE')
	 FROM pg_proc WHERE oid='checkout.order_attempt_admission_facts(uuid,jsonb)'::regprocedure`).Scan(&owner, &definer, &config, &runtime, &coordinator)
	if err != nil || owner != "commerce_checkout_writer" || !definer || !strings.Contains(config, "search_path=pg_catalog") || runtime || !coordinator {
		t.Fatalf("facts ACL owner=%s definer=%v config=%s runtime=%v coordinator=%v err=%v", owner, definer, config, runtime, coordinator, err)
	}
}
