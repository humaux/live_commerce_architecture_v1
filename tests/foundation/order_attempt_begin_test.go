// Purpose: REAL_PG independent acceptance of native OA1 Begin/stock/expiry effects.
// Depends on: real C1/fence/Open, native quote/destination/allocation helpers and inventory.PlanAllocation.
// Used by: parent CI after allocation prerequisites; every partial C2 outer transaction rolls back.
// Invariants: I01/I03/I04/I08/I14; order ID=attempt, exact creator chain, per-origin frozen money,
// tracked-SKU conservation, no buyer bearer, native expiry/order/receipt share one transaction.
package foundation_test

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/inventory"
	"livecommerce/internal/orderattempt"
)

type oaBeginFacts struct {
	Selection struct {
		Customer struct {
			Email string `json:"email"`
		} `json:"customer"`
		Locale string `json:"locale"`
	} `json:"selection"`
	Catalog struct {
		Service fulfillment.Service `json:"service"`
		Payment struct {
			WindowHours    int    `json:"window_hours"`
			BankName       string `json:"bank_name"`
			Branch         string `json:"branch"`
			AccountName    string `json:"account_name"`
			AccountNumber  string `json:"account_number"`
			SurchargeMinor int64  `json:"surcharge_minor"`
			Carrier        string `json:"carrier"`
		} `json:"payment"`
	} `json:"catalog"`
}

func oaBeginSelection(t *testing.T, e *lbuEnv, mode, stock string) map[string]any {
	t.Helper()
	selection := oaFactsSelection(e)
	if mode == "pay_at_pickup" {
		selection = oaMaterialCVS(t, e, "cvs_711", "001234", mode)
	}
	if mode == "cash_on_delivery" {
		if status, _ := e.hcodSettings(0, true, 20000, 35, "black_cat"); status != 200 {
			t.Fatalf("native COD settings sentinel: %d", status)
		}
	}
	selection["payment_mode"] = mode
	if stock == "all-untracked" {
		mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET inventory_tracked=false,max_per_order=10 WHERE id=$1`, e.money)
	}
	if stock == "mixed" {
		sku := e.sku(1000, 20)
		mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET inventory_tracked=false,max_per_order=10 WHERE id=$1`, sku)
		selection["items"] = append(selection["items"].([]map[string]any), map[string]any{"sku_id": sku, "quantity": 2, "ordinal": 3, "origin_id": randomUUID()})
	}
	return selection
}

func oaBeginPrepared(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt) (pgx.Tx, []byte, oaQuoteWire, string, oaAllocationWire, []inventory.Line) {
	t.Helper()
	tx, plain, quote := oaAllocationPrefix(t, e, a)
	// Native offline-capacity serialization belongs at the service level,
	// after catalog but before pickup/destination and any warehouse/balance locks.
	ids, _, err := oaAllocationCatalog(tx, a.AttemptID, plain)
	if err != nil || len(ids) == 0 {
		t.Fatalf("catalog safety prerequisite: %v", err)
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatal("catalog IDs are not in deterministic lock order")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] == ids[i] {
			t.Fatal("catalog returned duplicate origin SKU IDs")
		}
	}
	var offline []byte
	err = tx.QueryRow(context.Background(), `SELECT checkout.order_attempt_lock_offline_safety($1::uuid,$2::text)`, a.AttemptID, string(plain)).Scan(&offline)
	if err != nil {
		t.Fatalf("native offline settings lock before destination/inventory: %v", err)
	}
	destinationWire, _, err := oaDestinationCall(tx, a.AttemptID, plain, quote.CartID)
	if err != nil || destinationWire.DestinationID == "" {
		t.Fatalf("native destination prerequisite: %v", err)
	}
	destination := destinationWire.DestinationID
	allocation, _, err := oaAllocationCall(tx, a.AttemptID, plain, quote, destination)
	if err != nil || allocation.OrderID != a.AttemptID || allocation.Demands == nil || allocation.Balances == nil {
		t.Fatalf("real native allocation prerequisite: %v", err)
	}
	lines := []inventory.Line{}
	if len(allocation.Demands) != 0 {
		lines, err = inventory.PlanAllocation(allocation.WarehouseIDs, allocation.Demands, allocation.Balances)
		if err != nil {
			t.Fatalf("existing Go allocator over native locked facts: %v", err)
		}
	}
	return tx, plain, quote, destination, allocation, lines
}

func oaBeginCall(tx pgx.Tx, attempt string, plain []byte, quote oaQuoteWire, destination string, lines []inventory.Line) (checkout.Result, []byte, error) {
	var out checkout.Result
	encoded, err := json.Marshal(lines)
	if err != nil {
		return out, nil, err
	}
	var raw []byte
	// Calls checkout.order_attempt_begin, order-attempt-v1 §§1/2/3.1/6a/6j.
	err = tx.QueryRow(context.Background(), `SELECT checkout.order_attempt_begin($1::uuid,$2::text,$3::uuid,$4::uuid,$5::uuid,$6::jsonb)`, attempt, string(plain), quote.CartID, quote.QuoteID, destination, string(encoded)).Scan(&raw)
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out, raw, err
}

func oaBeginZeroEffects(t *testing.T, e *lbuEnv, attempt string) {
	t.Helper()
	var count int
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM checkout.orders WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM checkout.command_results WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM checkout.events WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM checkout.bank_transfers WHERE order_id=$1::uuid)
	 +(SELECT count(*) FROM inventory.reservations WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM inventory.reservation_lines WHERE reservation_id=$1::uuid)
	 +(SELECT count(*) FROM inventory.ledger WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM river_expiry.river_job WHERE args->>'order_id'=($1::uuid)::text)
	 +(SELECT count(*) FROM storefront.carts WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM storefront.cart_lines WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM storefront.quotes WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM storefront.destination_snapshots WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM storefront.destination_events WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM fulfillment.pickup_versions WHERE create_attempt_id=$1::uuid)`, attempt).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("outer rollback left native Begin/materialization effects: count=%d err=%v", count, err)
	}
}

func TestOrderAttemptBeginNativeEffectsAndRollback(t *testing.T) {
	for _, mode := range []string{"bank_transfer", "pay_at_pickup", "cash_on_delivery"} {
		for _, stock := range []string{"duplicate-origins", "mixed", "all-untracked"} {
			t.Run(mode+"/"+stock, func(t *testing.T) {
				e := lbuNew(t)
				a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, mode, stock))
				tx, plain, quote, destination, allocation, lines := oaBeginPrepared(t, e, a)
				var plan orderattempt.FrozenPlan
				var facts oaBeginFacts
				var destinationFacts oaDestinationFacts
				if err := json.Unmarshal(plain, &plan); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(plan.Facts, &facts); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(plan.Facts, &destinationFacts); err != nil {
					t.Fatal(err)
				}
				if stock == "all-untracked" {
					if len(lines) != 0 || len(allocation.Demands) != 0 {
						t.Fatal("all-untracked native prerequisite entered stock plan")
					}
				} else if len(lines) != 1 || lines[0].SKUID != e.money || lines[0].Quantity != 5 {
					t.Fatal("duplicate origins must allocate tracked SKU quantity five exactly once")
				}
				oaStateExec(t, tx, `RESET ROLE`)
				var beforeBalances string
				if err := tx.QueryRow(context.Background(), `SELECT coalesce(jsonb_agg(to_jsonb(b) ORDER BY warehouse_id,sku_id),'[]')::text FROM inventory.balances b WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()).Scan(&beforeBalances); err != nil {
					t.Fatal(err)
				}
				oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
				oaStateExec(t, tx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.principal_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true),set_config('app.buyer_session_id','caller-marker',true),set_config('app.order_attempt_id','caller-marker',true)`)
				got, raw, err := oaBeginCall(tx, a.AttemptID, plain, quote, destination, lines)
				if err != nil {
					t.Fatalf("native Begin after real C1/fence/quote/destination/allocation: %v", err)
				}
				state := map[string]string{"bank_transfer": "AWAITING_TRANSFER", "pay_at_pickup": "CONFIRMED", "cash_on_delivery": "AWAITING_COLLECTION"}[mode]
				if got.OrderID != a.AttemptID || got.ReservationID != a.AttemptID || got.Generation != 1 || got.JobID < 1 || got.PaymentMode != mode || got.CommercialState != state || got.ExpiresAt.IsZero() {
					t.Fatal("Begin result lost native attempt/order/job identity")
				}
				var exact bool
				if err = tx.QueryRow(context.Background(), `SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker' AND current_setting('app.buyer_id')='caller-marker' AND current_setting('app.buyer_session_id')='caller-marker' AND current_setting('app.order_attempt_id')='caller-marker'`).Scan(&exact); err != nil || !exact {
					t.Fatalf("Begin leaked caller context: %v", err)
				}
				oaStateExec(t, tx, `RESET ROLE`)
				reservationState := "COMMITTED"
				if mode == "bank_transfer" {
					reservationState = "HELD"
				}
				err = tx.QueryRow(context.Background(), `SELECT o.id=a.attempt_id AND o.owner_id=a.buyer_owner_id AND o.creator_kind='order_attempt' AND o.creator_session_id IS NULL AND o.create_attempt_id=a.attempt_id
				 AND o.tenant_id=a.tenant_id AND o.store_id=a.store_id AND o.cart_id=$2::uuid AND o.cart_version=1 AND o.quote_id=$3::uuid AND o.destination_id=$4::uuid
				 AND o.payment_mode=$5 AND o.commercial_state=$6 AND o.fulfillment_state='MANUAL_UNASSIGNED' AND o.generation=1 AND o.expires_at=$7::timestamptz
				 AND o.currency=q.currency AND o.total_minor=(q.snapshot->'amount'->>'total_minor')::bigint AND o.snapshot->'quote'=q.snapshot
				 AND o.service_version=$8::bigint AND o.allocation_version=$9::bigint AND o.buyer_email=$10 AND o.locale=$11
				 AND r.id=o.id AND r.checkout_id=o.id AND r.buyer_owner_id=o.owner_id AND r.creator_kind='order_attempt' AND r.buyer_session_id IS NULL AND r.create_attempt_id=a.attempt_id AND r.generation=1 AND r.state=$12 AND r.expires_at=o.expires_at
				 AND j.id=$13::bigint AND j.kind='checkout_expiry_v1' AND j.queue='checkout_expiry_v1' AND j.state='scheduled' AND j.unique_key IS NULL
				 AND j.args=jsonb_build_object('order_id',o.id::text,'generation',1,'version',1) AND j.scheduled_at=o.expires_at
				 AND (SELECT count(*) FROM river_expiry.river_job x WHERE x.args->>'order_id'=o.id::text)=1
				 AND (SELECT count(*) FROM checkout.events x WHERE x.create_attempt_id=a.attempt_id)=2
				 FROM ops.order_attempts a JOIN checkout.orders o ON o.create_attempt_id=a.attempt_id JOIN storefront.quotes q ON q.id=o.quote_id
				 JOIN inventory.reservations r ON r.checkout_id=o.id JOIN river_expiry.river_job j ON j.id=o.job_id WHERE a.attempt_id=$1::uuid`, a.AttemptID, quote.CartID, quote.QuoteID, destination, mode, state, got.ExpiresAt, facts.Catalog.Service.Version, allocation.Allocation.Version, facts.Selection.Customer.Email, facts.Selection.Locale, reservationState, got.JobID).Scan(&exact)
				if err != nil || !exact {
					t.Fatalf("native order/reservation/expiry/provenance/frozen snapshot: %v", err)
				}
				var created, expires time.Time
				var snapshotRaw []byte
				if err = tx.QueryRow(context.Background(), `SELECT created_at,expires_at,snapshot FROM checkout.orders WHERE id=$1`, a.AttemptID).Scan(&created, &expires, &snapshotRaw); err != nil {
					t.Fatal(err)
				}
				hold := 15 * time.Minute
				if mode == "bank_transfer" {
					hold = time.Duration(facts.Catalog.Payment.WindowHours) * time.Hour
				}
				if !expires.Equal(created.Add(hold)) || !expires.Equal(got.ExpiresAt) {
					t.Fatal("native hold lifetime did not use frozen offline window")
				}
				var snapshot checkout.Snapshot
				if err = json.Unmarshal(snapshotRaw, &snapshot); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(snapshot.Quote.Amount, plan.Calculation) || len(snapshot.Quote.Lines) != map[bool]int{false: 2, true: 3}[stock == "mixed"] || snapshot.Quote.Lines[0].OriginID == "" || snapshot.Quote.Lines[0].OriginID == snapshot.Quote.Lines[1].OriginID || snapshot.Quote.Lines[0].LineOrdinal != 1 || snapshot.Quote.Lines[1].LineOrdinal != 2 {
					t.Fatal("order snapshot merged origins or changed frozen money")
				}
				if snapshot.Destination.ID != destination || snapshot.Destination.CartID != quote.CartID || snapshot.Destination.CartVersion != 1 || !snapshot.Destination.SelectedAt.Equal(plan.AdmittedAt) || !snapshot.Destination.ExpiresAt.Equal(plan.ExecuteBy) {
					t.Fatal("native destination identity/lifetime lost from order snapshot")
				}
				if snapshot.Destination.Version != 1 || snapshot.Destination.Kind != facts.Catalog.Service.DeliveryKind || snapshot.Destination.Country != destinationFacts.Selection.Delivery.Country || snapshot.Destination.RecipientName != destinationFacts.Selection.Customer.Name || snapshot.Destination.Phone != destinationFacts.Selection.Customer.Phone {
					t.Fatal("order snapshot lost frozen destination/contact fields")
				}
				if destinationFacts.Catalog.Pickup == nil {
					if snapshot.Destination.Pickup != nil || snapshot.Destination.HomeAddress != destinationFacts.Selection.Delivery.Home {
						t.Fatal("order snapshot replaced frozen home address")
					}
				} else {
					if snapshot.Destination.Pickup == nil || snapshot.Destination.Pickup.ID == "" {
						t.Fatal("order snapshot lost native pickup projection")
					}
					actual, frozen := *snapshot.Destination.Pickup, *destinationFacts.Catalog.Pickup
					if !actual.AttestedAt.Equal(frozen.AttestedAt) || !actual.ValidUntil.Equal(frozen.ValidUntil) {
						t.Fatal("order snapshot changed frozen pickup lifetime")
					}
					actual.ID = ""
					actual.AttestedAt = frozen.AttestedAt
					actual.ValidUntil = frozen.ValidUntil
					if !reflect.DeepEqual(actual, frozen) {
						t.Fatal("order snapshot changed frozen entered-store fields")
					}
					err = tx.QueryRow(context.Background(), `SELECT d.pickup_id=$2::uuid AND p.create_attempt_id=d.create_attempt_id AND p.buyer_owner_id=d.owner_id FROM storefront.destination_snapshots d JOIN fulfillment.pickup_versions p ON p.id=d.pickup_id WHERE d.id=$1::uuid`, destination, snapshot.Destination.Pickup.ID).Scan(&exact)
					if err != nil || !exact {
						t.Fatalf("order snapshot substituted physical pickup source: %v", err)
					}
				}
				if !reflect.DeepEqual(snapshot.Service, facts.Catalog.Service) || !reflect.DeepEqual(snapshot.Allocation, allocation.Allocation) {
					t.Fatal("order snapshot replaced frozen service or bound current allocation")
				}
				canonical, err := json.Marshal(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				var typedAgain checkout.Snapshot
				if err = json.Unmarshal(canonical, &typedAgain); err != nil {
					t.Fatal(err)
				}
				again, err := json.Marshal(typedAgain)
				if err != nil || string(again) != string(canonical) {
					t.Fatal("typed checkout order snapshot loses wire fields")
				}
				if mode == "bank_transfer" {
					err = tx.QueryRow(context.Background(), `SELECT state='AWAITING' AND bank_name=$2 AND branch=$3 AND account_name=$4 AND account_number=$5 AND window_hours=$6 AND currency=$7 FROM checkout.bank_transfers WHERE order_id=$1`, a.AttemptID, facts.Catalog.Payment.BankName, facts.Catalog.Payment.Branch, facts.Catalog.Payment.AccountName, facts.Catalog.Payment.AccountNumber, facts.Catalog.Payment.WindowHours, snapshot.Quote.Currency).Scan(&exact)
				} else if mode == "cash_on_delivery" {
					err = tx.QueryRow(context.Background(), `SELECT cod_surcharge_minor=$2 AND cod_carrier=$3 AND collection_state='PENDING' AND total_minor=$4 FROM checkout.orders WHERE id=$1`, a.AttemptID, facts.Catalog.Payment.SurchargeMinor, facts.Catalog.Payment.Carrier, plan.Calculation.TotalMinor).Scan(&exact)
					if snapshot.ExpectedCodSurchargeMinor == nil || *snapshot.ExpectedCodSurchargeMinor != facts.Catalog.Payment.SurchargeMinor {
						t.Fatal("frozen COD surcharge not preserved independently from quote total")
					}
				} else {
					err = tx.QueryRow(context.Background(), `SELECT collection_state='PENDING' AND cod_surcharge_minor IS NULL FROM checkout.orders WHERE id=$1`, a.AttemptID).Scan(&exact)
				}
				if err != nil || !exact {
					t.Fatalf("native frozen offline payment snapshot: %v", err)
				}
				var eventCount, lineCount, ledgerCount int
				if err = tx.QueryRow(context.Background(), `SELECT count(*) FROM checkout.events WHERE create_attempt_id=$1 AND order_id=$1 AND owner_id=$2 AND actor_kind='ORDER_ATTEMPT' AND creator_kind='order_attempt' AND session_id IS NULL AND generation=1 AND action=ANY($3::text[])`, a.AttemptID, plan.BuyerOwnerID, []string{"checkout.held", "checkout." + mode + "_placed"}).Scan(&eventCount); err != nil || eventCount != 2 {
					t.Fatalf("native checkout event identities: count=%d err=%v", eventCount, err)
				}
				if err = tx.QueryRow(context.Background(), `SELECT count(*) FROM inventory.reservation_lines WHERE reservation_id=$1`, a.AttemptID).Scan(&lineCount); err != nil || lineCount != len(lines) {
					t.Fatalf("tracked reservation conservation: count=%d err=%v", lineCount, err)
				}
				expectedLedger := len(lines)
				if mode != "bank_transfer" {
					expectedLedger *= 2
				}
				if err = tx.QueryRow(context.Background(), `SELECT count(*) FROM inventory.ledger WHERE create_attempt_id=$1`, a.AttemptID).Scan(&ledgerCount); err != nil || ledgerCount != expectedLedger {
					t.Fatalf("native reserve/allocate ledger multiplicity: count=%d err=%v", ledgerCount, err)
				}
				for _, line := range lines {
					err = tx.QueryRow(context.Background(), `SELECT l.quantity=$4 AND x.actor_kind='ORDER_ATTEMPT' AND x.creator_kind='order_attempt' AND x.principal_id=$5::uuid AND x.buyer_owner_id=$6::uuid AND x.buyer_session_id IS NULL AND x.checkout_id=$1::uuid AND x.create_attempt_id=$1::uuid AND x.operation='checkout.order_attempt.reserve' AND x.command_key=($1::uuid)::text AND x.delta_reserved=$4 AND x.delta_allocated=0
					 FROM inventory.reservation_lines l JOIN inventory.ledger x ON x.reservation_id=l.reservation_id AND x.warehouse_id=l.warehouse_id AND x.sku_id=l.sku_id AND x.kind='RESERVE' WHERE l.reservation_id=$1::uuid AND l.warehouse_id=$2::uuid AND l.sku_id=$3::uuid`, a.AttemptID, line.WarehouseID, line.SKUID, line.Quantity, plan.PrincipalID, plan.BuyerOwnerID).Scan(&exact)
					if err != nil || !exact {
						t.Fatalf("native reserve ledger exact provenance/quantity: %v", err)
					}
					if mode != "bank_transfer" {
						err = tx.QueryRow(context.Background(), `SELECT actor_kind='ORDER_ATTEMPT' AND creator_kind='order_attempt' AND principal_id=$5::uuid AND buyer_owner_id=$6::uuid AND buyer_session_id IS NULL AND create_attempt_id=$1::uuid AND checkout_id=$1::uuid AND operation='checkout.pay_at_pickup.commit' AND command_key=($1::uuid)::text AND delta_reserved=-$4::bigint AND delta_allocated=$4::bigint FROM inventory.ledger WHERE reservation_id=$1::uuid AND warehouse_id=$2::uuid AND sku_id=$3::uuid AND kind='ALLOCATE'`, a.AttemptID, line.WarehouseID, line.SKUID, line.Quantity, plan.PrincipalID, plan.BuyerOwnerID).Scan(&exact)
						if err != nil || !exact {
							t.Fatalf("native offline allocation exact provenance/quantity: %v", err)
						}
					}
				}
				for _, b := range allocation.Balances {
					reserved, allocated := b.Reserved, b.Allocated
					for _, line := range lines {
						if line.WarehouseID == b.WarehouseID && line.SKUID == b.SKUID {
							if mode == "bank_transfer" {
								reserved += line.Quantity
							} else {
								allocated += line.Quantity
							}
						}
					}
					err = tx.QueryRow(context.Background(), `SELECT on_hand=$3 AND reserved=$4 AND allocated=$5 AND unavailable=$6 FROM inventory.balances WHERE warehouse_id=$1 AND sku_id=$2`, b.WarehouseID, b.SKUID, b.OnHand, reserved, allocated, b.Unavailable).Scan(&exact)
					if err != nil || !exact {
						t.Fatalf("native balance conservation: %v", err)
					}
				}
				if stock == "all-untracked" {
					var unchangedBalances string
					if err = tx.QueryRow(context.Background(), `SELECT coalesce(jsonb_agg(to_jsonb(b) ORDER BY warehouse_id,sku_id),'[]')::text FROM inventory.balances b WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()).Scan(&unchangedBalances); err != nil || unchangedBalances != beforeBalances {
						t.Fatalf("untracked Begin touched native balance counters/version: %v", err)
					}
				}
				err = tx.QueryRow(context.Background(), `SELECT count(*)=1 AND bool_and(r.creator_kind='order_attempt' AND r.creator_session_id IS NULL AND r.create_attempt_id=a.attempt_id AND r.order_id=a.attempt_id AND r.owner_id=a.buyer_owner_id AND r.operation='checkout.begin' AND r.idempotency_key=a.attempt_id::text AND r.request_hash=a.request_hash AND r.response=$2::jsonb) FROM checkout.command_results r JOIN ops.order_attempts a ON a.attempt_id=r.create_attempt_id WHERE a.attempt_id=$1::uuid`, a.AttemptID, string(raw)).Scan(&exact)
				if err != nil || !exact {
					t.Fatalf("native command receipt did not bind request/order/attempt/result: %v", err)
				}
				err = tx.QueryRow(context.Background(), `SELECT NOT EXISTS(SELECT 1 FROM buyer.capability_sessions WHERE owner_id=$2::uuid) AND NOT EXISTS(SELECT 1 FROM checkout.payment_attempts WHERE order_id=$1::uuid) AND (SELECT state='PENDING' AND order_id IS NULL FROM ops.order_attempts WHERE attempt_id=$1::uuid)`, a.AttemptID, plan.BuyerOwnerID).Scan(&exact)
				if err != nil || !exact {
					t.Fatalf("Begin minted bearer/payment or falsely finalized partial C2: %v", err)
				}
				if err = tx.Rollback(context.Background()); err != nil {
					t.Fatal(err)
				}
				oaBeginZeroEffects(t, e, a.AttemptID)
				var afterBalances string
				if err = e.p.f.owner.QueryRow(context.Background(), `SELECT coalesce(jsonb_agg(to_jsonb(b) ORDER BY warehouse_id,sku_id),'[]')::text FROM inventory.balances b WHERE tenant_id=$1 AND store_id=$2`, e.tenant(), e.store()).Scan(&afterBalances); err != nil || afterBalances != beforeBalances {
					t.Fatalf("outer rollback changed balance counters/version: %v", err)
				}
			})
		}
	}
}

func TestOrderAttemptBeginRejectsForgedNativeAllocation(t *testing.T) {
	for _, change := range []string{"missing-quantity", "extra-quantity", "duplicate-pair", "foreign-warehouse", "untracked-plan", "current-stock-exhausted"} {
		t.Run(change, func(t *testing.T) {
			e := lbuNew(t)
			stock := "duplicate-origins"
			if change == "untracked-plan" {
				stock = "all-untracked"
			}
			a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, "bank_transfer", stock))
			tx, plain, quote, destination, _, lines := oaBeginPrepared(t, e, a)
			code := "PT409"
			switch change {
			case "missing-quantity":
				lines[0].Quantity--
			case "extra-quantity":
				lines[0].Quantity++
			case "duplicate-pair":
				lines = append(lines, lines[0])
				code = "PT400"
			case "foreign-warehouse":
				lines[0].WarehouseID = randomUUID()
			case "untracked-plan":
				lines = []inventory.Line{{WarehouseID: e.p.stock.warehouse.ID, SKUID: e.money, Quantity: 5}}
			case "current-stock-exhausted":
				// Disclosed owner fixture changes the locked native balance within this Tx;
				// it never edits frozen C1 or trusts fabricated returned balance facts.
				oaStateExec(t, tx, `RESET ROLE`)
				oaStateExec(t, tx, `UPDATE inventory.balances SET unavailable=on_hand-reserved-allocated,version=version+1 WHERE tenant_id=$1 AND store_id=$2 AND sku_id=$3`, e.tenant(), e.store(), e.money)
				oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
				code = "PT402"
			}
			_, raw, err := oaBeginCall(tx, a.AttemptID, plain, quote, destination, lines)
			oaFenceCode(t, raw, err, code)
			_ = tx.Rollback(context.Background())
			oaBeginZeroEffects(t, e, a.AttemptID)
		})
	}
}

func TestOrderAttemptBeginFrozenPricesFeesAndOfflineSettings(t *testing.T) {
	for _, mode := range []string{"bank_transfer", "pay_at_pickup", "cash_on_delivery"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, mode, "duplicate-origins"))
			plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
			var facts oaBeginFacts
			if err := json.Unmarshal(plan.Facts, &facts); err != nil {
				t.Fatal(err)
			}
			mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET price_minor=price_minor+123,version=version+1 WHERE id=$1`, e.money)
			service := facts.Catalog.Service.Code
			// The base home service lives in bcHarness; svcs contains only added CVS services.
			in := e.p.delivery
			if facts.Catalog.Service.DeliveryKind != "home" {
				in = e.svcs[service]
			}
			if in.Code != service || in.MarketID != facts.Catalog.Service.MarketID || in.Country != facts.Catalog.Service.Country {
				t.Fatal("native service drift fixture does not match frozen admission")
			}
			dsPolicy(t, e.p.cqHarness, in, 1, 237, true)
			in.ExpectedVersion = facts.Catalog.Service.Version
			in.PolicyVersion = 2
			in.Visible = false
			in.NameEN = "Changed display"
			if out, err := dsSet(e.p.cqHarness, t04Key("oa-begin-service-drift"), in); err != nil || out.Version != facts.Catalog.Service.Version+1 {
				t.Fatalf("native service drift sentinel: %+v %v", out, err)
			}
			if mode == "bank_transfer" {
				mustExec(t, e.p.f.owner, `UPDATE checkout.bank_transfer_settings SET bank_name='Changed synthetic bank',account_number='999000111',window_hours=6,version=version+1 WHERE store_id=$1`, e.store())
			}
			if mode == "cash_on_delivery" {
				mustExec(t, e.p.f.owner, `UPDATE checkout.cash_on_delivery_settings SET surcharge_twd=99,version=version+1 WHERE store_id=$1`, e.store())
			}
			tx, plain, quote, destination, _, lines := oaBeginPrepared(t, e, a)
			got, _, err := oaBeginCall(tx, a.AttemptID, plain, quote, destination, lines)
			if err != nil || got.OrderID != a.AttemptID {
				t.Fatalf("current price/fee/display/version revoked frozen admitted Begin: %v", err)
			}
			oaStateExec(t, tx, `RESET ROLE`)
			var exact bool
			err = tx.QueryRow(context.Background(), `SELECT total_minor=$2 AND snapshot->'quote'=(SELECT snapshot FROM storefront.quotes WHERE id=$3::uuid) AND service_version=$4 FROM checkout.orders WHERE id=$1::uuid`, a.AttemptID, plan.Calculation.TotalMinor, quote.QuoteID, facts.Catalog.Service.Version).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("order repriced frozen quote/service: %v", err)
			}
			if mode == "bank_transfer" {
				err = tx.QueryRow(context.Background(), `SELECT bank_name=$2 AND account_number=$3 AND window_hours=$4 FROM checkout.bank_transfers WHERE order_id=$1`, a.AttemptID, facts.Catalog.Payment.BankName, facts.Catalog.Payment.AccountNumber, facts.Catalog.Payment.WindowHours).Scan(&exact)
			}
			if mode == "cash_on_delivery" {
				err = tx.QueryRow(context.Background(), `SELECT cod_surcharge_minor=$2 AND total_minor=$3 FROM checkout.orders WHERE id=$1`, a.AttemptID, facts.Catalog.Payment.SurchargeMinor, plan.Calculation.TotalMinor).Scan(&exact)
			}
			if err != nil || !exact {
				t.Fatalf("current offline fee/settings replaced frozen monetary facts: %v", err)
			}
			_ = tx.Rollback(context.Background())
			oaBeginZeroEffects(t, e, a.AttemptID)
		})
	}
}

func TestOrderAttemptBeginDenialsAndExactACL(t *testing.T) {
	e := lbuNew(t)
	a := oaAllocationAdmit(t, e, oaBeginSelection(t, e, "bank_transfer", "duplicate-origins"))
	for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_buyer_runtime", "commerce_checkout_runtime", "commerce_buyer_writer", "commerce_privacy_writer"} {
		t.Run(role, func(t *testing.T) {
			tx, err := e.p.f.owner.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			oaStateExec(t, tx, `SET LOCAL ROLE `+role)
			_, raw, err := oaBeginCall(tx, a.AttemptID, []byte(`{}`), oaQuoteWire{CartID: randomUUID(), QuoteID: randomUUID()}, randomUUID(), []inventory.Line{})
			oaFenceCode(t, raw, err, "42501")
		})
	}
	for _, change := range []string{"no-fence", "changed-plan", "foreign-cart", "foreign-quote", "foreign-destination"} {
		t.Run(change, func(t *testing.T) {
			var tx pgx.Tx
			var plain []byte
			var quote oaQuoteWire
			var destination string
			var lines []inventory.Line
			if change == "no-fence" {
				plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
				var err error
				plain, err = json.Marshal(plan)
				if err != nil {
					t.Fatal(err)
				}
				tx, err = e.p.f.owner.Begin(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
				quote = oaQuoteWire{CartID: randomUUID(), QuoteID: randomUUID()}
				destination = randomUUID()
				lines = []inventory.Line{}
			} else {
				tx, plain, quote, destination, _, lines = oaBeginPrepared(t, e, a)
			}
			code := "PT409"
			switch change {
			case "changed-plan":
				plain = append(plain, ' ')
				code = "PT400"
			case "foreign-cart":
				quote.CartID = randomUUID()
			case "foreign-quote":
				quote.QuoteID = randomUUID()
			case "foreign-destination":
				destination = randomUUID()
			}
			_, raw, err := oaBeginCall(tx, a.AttemptID, plain, quote, destination, lines)
			oaFenceCode(t, raw, err, code)
			_ = tx.Rollback(context.Background())
			oaBeginZeroEffects(t, e, a.AttemptID)
		})
	}
	var exact bool
	err := e.p.f.owner.QueryRow(context.Background(), `SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype='jsonb'::regtype AND r.rolname='commerce_checkout_writer' AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolbypassrls
	 AND p.proconfig=ARRAY['search_path=pg_catalog'] AND obj_description(p.oid,'pg_proc') IS NOT NULL
	 AND (SELECT count(*) FROM aclexplode(p.proacl) x WHERE x.grantee<>p.proowner)=1
	 AND EXISTS(SELECT 1 FROM aclexplode(p.proacl) x JOIN pg_roles g ON g.oid=x.grantee WHERE g.rolname='commerce_order_attempt_owner' AND x.privilege_type='EXECUTE' AND NOT x.is_grantable)
	 AND NOT has_table_privilege('commerce_runtime','river_expiry.river_job','INSERT') AND NOT has_table_privilege('commerce_claims_worker','river_expiry.river_job','INSERT')
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid='checkout.order_attempt_begin(uuid,text,uuid,uuid,uuid,jsonb)'::regprocedure`).Scan(&exact)
	if err != nil || !exact {
		t.Fatalf("private Begin/expiry producer ACL boundary: %v", err)
	}
}
