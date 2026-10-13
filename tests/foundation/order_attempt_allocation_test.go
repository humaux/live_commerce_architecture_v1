// Purpose: REAL_PG contracts for the attempt-owned catalog/allocation preparation seam.
// Depends on: real C1, execution fence, native quote/destination helpers, inventory.PlanAllocation.
// Used by: parent CI; every private materialization transaction rolls back.
// Invariants: I01/I03/I04/I08; original origins survive, stock demand aggregates by SKU,
// catalog precedes service and balances, no hold/order/job or SQL allocation algorithm.
package foundation_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/inventory"
	"livecommerce/internal/orderattempt"
)

type oaAllocationWire struct {
	OrderID      string                 `json:"order_id"`
	WarehouseIDs []string               `json:"warehouse_ids"`
	Demands      []inventory.Demand     `json:"demands"`
	Balances     []inventory.Balance    `json:"balances"`
	Allocation   fulfillment.Allocation `json:"allocation"`
}

func oaAllocationCatalog(tx pgx.Tx, attempt string, plain []byte) ([]string, []byte, error) {
	var raw []byte
	// Calls checkout.order_attempt_lock_catalog, order-attempt-v1 §§2/3.1.
	err := tx.QueryRow(context.Background(), `SELECT checkout.order_attempt_lock_catalog($1::uuid,$2::text)`, attempt, string(plain)).Scan(&raw)
	var out struct {
		SKUIDs []string `json:"sku_ids"`
	}
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out.SKUIDs, raw, err
}

func oaAllocationCall(tx pgx.Tx, attempt string, plain []byte, quote oaQuoteWire, destination string) (oaAllocationWire, []byte, error) {
	var raw []byte
	// Calls checkout.order_attempt_prepare_allocation, order-attempt-v1 §§2/3.1/6j.
	err := tx.QueryRow(context.Background(), `SELECT checkout.order_attempt_prepare_allocation($1::uuid,$2::text,$3::uuid,$4::uuid,$5::uuid)`, attempt, string(plain), quote.CartID, quote.QuoteID, destination).Scan(&raw)
	var out oaAllocationWire
	if err == nil {
		err = json.Unmarshal(raw, &out)
	}
	return out, raw, err
}

func oaAllocationAdmit(t *testing.T, e *lbuEnv, selection map[string]any) orderattempt.AdmissionReceipt {
	t.Helper()
	req := oaMaterialRequest(t, e, selection)
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || a.State != "PENDING" || a.AttemptID == "" {
		t.Fatalf("real C1 allocation sentinel: state=%s err=%v", a.State, err)
	}
	return a
}

func oaAllocationPrefix(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt) (pgx.Tx, []byte, oaQuoteWire) {
	t.Helper()
	tx, plain := oaFenceDomainTx(t, e, a, true)
	var plan orderattempt.FrozenPlan
	var facts struct {
		Claims struct {
			Lines []json.RawMessage `json:"lines"`
		} `json:"claims"`
	}
	if err := json.Unmarshal(plain, &plan); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(plan.Facts, &facts); err != nil || len(facts.Claims.Lines) != 0 {
		t.Fatal("manual fixture requires the actual empty claim lock set")
	}
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	var ownerState string
	// Calls buyer.order_attempt_lock_owners, order-attempt-v1 §3.1, before order key.
	if err := tx.QueryRow(context.Background(), `SELECT buyer.order_attempt_lock_owners($1,$2)`, a.AttemptID, a.Generation).Scan(&ownerState); err != nil || ownerState != "active" {
		t.Fatalf("native sorted owner prefix: state=%s err=%v", ownerState, err)
	}
	oaStateExec(t, tx, `RESET ROLE`)
	// Disclosed owner fixture holds the exact parent-frozen common order key.
	// There is no order/parcel row yet, and no claim row for this manual intent.
	oaStateExec(t, tx, `SELECT pg_advisory_xact_lock(hashtextextended('checkout.order|'||tenant_id::text||'|'||store_id::text||'|'||attempt_id::text,0)) FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID)
	quote := oaDestinationQuote(t, tx, a, plain)
	return tx, plain, quote
}

func oaAllocationDestination(t *testing.T, tx pgx.Tx, a orderattempt.AdmissionReceipt, plain []byte, quote oaQuoteWire) string {
	t.Helper()
	ids, _, err := oaAllocationCatalog(tx, a.AttemptID, plain)
	if err != nil || len(ids) == 0 {
		t.Fatalf("catalog safety before service/destination: ids=%v err=%v", ids, err)
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatal("catalog IDs are not in deterministic lock order")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] == ids[i] {
			t.Fatal("catalog returned duplicate origin SKU IDs")
		}
	}
	destination, _, err := oaDestinationCall(tx, a.AttemptID, plain, quote.CartID)
	if err != nil || destination.DestinationID == "" {
		t.Fatalf("real native destination prerequisite: %v", err)
	}
	return destination.DestinationID
}

func TestOrderAttemptAllocationAggregatesNativeStockAndRollsBack(t *testing.T) {
	for _, mode := range []string{"duplicate-origins", "mixed", "all-untracked"} {
		t.Run(mode, func(t *testing.T) {
			e := lbuNew(t)
			selection := oaFactsSelection(e)
			if mode == "all-untracked" {
				mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET inventory_tracked=false,max_per_order=10 WHERE id=$1`, e.money)
			}
			if mode == "mixed" {
				sku := e.sku(1000, 20)
				mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET inventory_tracked=false,max_per_order=10 WHERE id=$1`, sku)
				selection["items"] = append(selection["items"].([]map[string]any), map[string]any{"sku_id": sku, "quantity": 2, "ordinal": 3, "origin_id": randomUUID()})
			}
			a := oaAllocationAdmit(t, e, selection)
			tx, plain, quote := oaAllocationPrefix(t, e, a)
			oaStateExec(t, tx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.principal_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true),set_config('app.buyer_session_id','caller-marker',true),set_config('app.order_attempt_id','caller-marker',true)`)
			destination := oaAllocationDestination(t, tx, a, plain, quote)
			got, _, err := oaAllocationCall(tx, a.AttemptID, plain, quote, destination)
			if err != nil {
				t.Fatalf("prepare after real C1/fence/native prefix: %v", err)
			}
			if got.OrderID != a.AttemptID || !reflect.DeepEqual(got.WarehouseIDs, []string{e.p.stock.warehouse.ID}) || !reflect.DeepEqual(got.WarehouseIDs, got.Allocation.WarehouseIDs) || got.Allocation.Version < 1 || got.Allocation.MarketID != e.p.market.ID || got.Allocation.Country != "TW" || got.Allocation.Code != "home" {
				t.Fatal("allocation lost bound order or current warehouse priority/version")
			}
			if got.Demands == nil || got.Balances == nil {
				t.Fatal("empty demands/balances must be arrays, not null")
			}
			var restored bool
			err = tx.QueryRow(context.Background(), `SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker' AND current_setting('app.buyer_id')='caller-marker' AND current_setting('app.buyer_session_id')='caller-marker' AND current_setting('app.order_attempt_id')='caller-marker'`).Scan(&restored)
			if err != nil || !restored {
				t.Fatalf("private allocation helpers trusted or leaked caller context: %v", err)
			}
			if mode == "all-untracked" {
				if len(got.Demands) != 0 || len(got.Balances) != 0 {
					t.Fatal("untracked SKU entered stock allocation")
				}
			} else {
				if !reflect.DeepEqual(got.Demands, []inventory.Demand{{SKUID: e.money, Quantity: 5}}) || len(got.Balances) != 1 {
					t.Fatalf("two origins must produce one demand of five: %+v", got.Demands)
				}
				lines, err := inventory.PlanAllocation(got.WarehouseIDs, got.Demands, got.Balances)
				if err != nil || len(lines) != 1 || lines[0].SKUID != e.money || lines[0].Quantity != 5 || lines[0].WarehouseID != e.p.stock.warehouse.ID {
					t.Fatalf("existing Go allocator cannot consume native facts: lines=%+v err=%v", lines, err)
				}
			}
			if _, err = tx.Exec(context.Background(), `RESET ROLE`); err != nil {
				t.Fatal(err)
			}
			var exact bool
			err = tx.QueryRow(context.Background(), `SELECT
			 (SELECT count(*) FROM storefront.cart_lines WHERE create_attempt_id=$1 AND sku_id=$2)=2
			 AND (SELECT sum(quantity) FROM storefront.cart_lines WHERE create_attempt_id=$1 AND sku_id=$2)=5
			 AND NOT EXISTS(SELECT 1 FROM checkout.orders WHERE create_attempt_id=$1)
			 AND NOT EXISTS(SELECT 1 FROM inventory.reservations WHERE create_attempt_id=$1)
			 AND NOT EXISTS(SELECT 1 FROM inventory.ledger WHERE create_attempt_id=$1)
			 AND NOT EXISTS(SELECT 1 FROM river_expiry.river_job WHERE args->>'order_id'=$1::text)`, a.AttemptID, e.money).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("allocation merged origins or created Begin effects: %v", err)
			}
			for _, b := range got.Balances {
				err = tx.QueryRow(context.Background(), `SELECT on_hand=$3 AND reserved=$4 AND allocated=$5 AND unavailable=$6 AND version=$7
				 FROM inventory.balances WHERE warehouse_id=$1 AND sku_id=$2`, b.WarehouseID, b.SKUID, b.OnHand, b.Reserved, b.Allocated, b.Unavailable, b.Version).Scan(&exact)
				if err != nil || !exact {
					t.Fatalf("returned balance differs from actual native row: %v", err)
				}
			}
			if err = tx.Rollback(context.Background()); err != nil {
				t.Fatal(err)
			}
			err = e.p.f.owner.QueryRow(context.Background(), `SELECT NOT EXISTS(SELECT 1 FROM storefront.carts WHERE create_attempt_id=$1)
			 AND NOT EXISTS(SELECT 1 FROM storefront.quotes WHERE create_attempt_id=$1)
			 AND NOT EXISTS(SELECT 1 FROM storefront.destination_snapshots WHERE create_attempt_id=$1)`, a.AttemptID).Scan(&exact)
			if err != nil || !exact {
				t.Fatalf("private seam rollback left materialization: %v", err)
			}
		})
	}
}

func TestOrderAttemptAllocationCurrentSafetyAndStockDrift(t *testing.T) {
	for _, change := range []string{"sku-disabled", "aggregate-cap", "exhausted", "price-only", "allocation-version"} {
		t.Run(change, func(t *testing.T) {
			e := lbuNew(t)
			if change == "aggregate-cap" {
				mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET inventory_tracked=false,max_per_order=10 WHERE id=$1`, e.money)
			}
			a := oaAllocationAdmit(t, e, oaFactsSelection(e))
			switch change {
			case "sku-disabled":
				mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET status='archived' WHERE id=$1`, e.money)
			case "aggregate-cap":
				mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET max_per_order=4 WHERE id=$1`, e.money)
			case "exhausted":
				// Owner fixture edits native free stock, not the frozen plan or helper output.
				mustExec(t, e.p.f.owner, `UPDATE inventory.balances SET unavailable=on_hand-reserved-allocated,version=version+1 WHERE sku_id=$1`, e.money)
			case "price-only":
				mustExec(t, e.p.f.owner, `UPDATE catalog.skus SET price_minor=price_minor+100,version=version+1 WHERE id=$1`, e.money)
			case "allocation-version":
				_, err := daSet(e.p.cqHarness, t04Key("oa-allocation-drift"), fulfillment.AllocationInput{MarketID: e.p.market.ID, Country: "TW", Code: "home", ExpectedVersion: 1, ExpectedServiceVersion: 1, WarehouseIDs: []string{e.p.stock.warehouse.ID}})
				if err != nil {
					t.Fatal(err)
				}
			}
			tx, plain, quote := oaAllocationPrefix(t, e, a)
			if change == "sku-disabled" || change == "aggregate-cap" {
				_, raw, err := oaAllocationCatalog(tx, a.AttemptID, plain)
				oaFenceCode(t, raw, err, "PT422")
				return
			}
			destination := oaAllocationDestination(t, tx, a, plain, quote)
			got, _, err := oaAllocationCall(tx, a.AttemptID, plain, quote, destination)
			if err != nil {
				t.Fatalf("locked balance facts after %s: %v", change, err)
			}
			if change == "allocation-version" && got.Allocation.Version != 2 {
				t.Fatal("allocation did not reverify the current version")
			}
			_, err = inventory.PlanAllocation(got.WarehouseIDs, got.Demands, got.Balances)
			if change == "exhausted" {
				if !errors.Is(err, command.ErrInsufficient) {
					t.Fatalf("Go allocator did not observe native exhausted stock: %v", err)
				}
			} else if err != nil {
				t.Fatalf("price-only drift refused frozen allocation: %v", err)
			}
		})
	}
}

func TestOrderAttemptAllocationNativeLocksRetained(t *testing.T) {
	e := lbuNew(t)
	a := oaAllocationAdmit(t, e, oaFactsSelection(e))
	tx, plain, quote := oaAllocationPrefix(t, e, a)
	destination := oaAllocationDestination(t, tx, a, plain, quote)
	got, _, err := oaAllocationCall(tx, a.AttemptID, plain, quote, destination)
	if err != nil || len(got.Balances) != 1 {
		t.Fatalf("positive locked native balance sentinel: %v", err)
	}
	for _, check := range []struct {
		query string
		args  []any
	}{
		{`SELECT id FROM catalog.skus WHERE id=$1 FOR UPDATE NOWAIT`, []any{e.money}},
		{`SELECT p.id FROM catalog.products p JOIN catalog.skus s ON s.product_id=p.id WHERE s.id=$1 FOR UPDATE OF p NOWAIT`, []any{e.money}},
		{`SELECT current_version FROM fulfillment.service_heads WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country='TW' AND code='home' FOR UPDATE NOWAIT`, []any{e.tenant(), e.store(), e.p.market.ID}},
		{`SELECT current_version FROM fulfillment.allocation_heads WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country='TW' AND code='home' FOR UPDATE NOWAIT`, []any{e.tenant(), e.store(), e.p.market.ID}},
		{`SELECT warehouse_id FROM inventory.balances WHERE sku_id=$1 FOR UPDATE NOWAIT`, []any{e.money}},
	} {
		probe, err := e.p.f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = probe.Exec(context.Background(), check.query, check.args...)
		_ = probe.Rollback(context.Background())
		oaFenceCode(t, nil, err, "55P03")
	}
}

func TestOrderAttemptAllocationCatalogBeforeServiceBarrier(t *testing.T) {
	e := lbuNew(t)
	a := oaAllocationAdmit(t, e, oaFactsSelection(e))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	var sku string
	// NO KEY UPDATE permits quote's SKU FK KEY SHARE but blocks catalog FOR SHARE.
	if err = holder.QueryRow(ctx, `SELECT id::text FROM catalog.skus WHERE id=$1 FOR NO KEY UPDATE`, e.money).Scan(&sku); err != nil || sku != e.money {
		t.Fatalf("real SKU lock sentinel: %v", err)
	}
	tx, plain, _ := oaAllocationPrefix(t, e, a)
	var pid int
	if err = tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { defer close(done); _, _, callErr := oaAllocationCatalog(tx, a.AttemptID, plain); done <- callErr }()
	defer func() { _ = holder.Rollback(context.Background()); select { case <-done: case <-ctx.Done(): } }()
	ticker := time.NewTicker(10*time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err = e.p.f.owner.QueryRow(ctx, `SELECT $2::int=ANY(pg_blocking_pids($1))`, pid, int32(holder.Conn().PgConn().PID())).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err = <-done: t.Fatalf("catalog bypassed held SKU: %v", err)
		case <-ticker.C:
		case <-ctx.Done(): t.Fatal("catalog did not reach actual SKU barrier")
		}
	}
	probe, err := e.p.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Rollback(context.Background())
	var version int64
	err = probe.QueryRow(ctx, `SELECT current_version FROM fulfillment.service_heads WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country='TW' AND code='home' FOR UPDATE NOWAIT`, e.tenant(), e.store(), e.p.market.ID).Scan(&version)
	if err != nil || version < 1 {
		t.Fatalf("catalog helper took later service level before SKU: %v", err)
	}
	_ = probe.Rollback(ctx)
	if err = holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done: if err != nil { t.Fatalf("catalog after lock release: %v", err) }
	case <-ctx.Done(): t.Fatal("catalog did not resume after native SKU release")
	}
}

func TestOrderAttemptAllocationExactACL(t *testing.T) {
	e := lbuNew(t)
	for _, signature := range []string{"checkout.order_attempt_lock_catalog(uuid,text)", "checkout.order_attempt_prepare_allocation(uuid,text,uuid,uuid,uuid)"} {
		var safe bool
		err := e.p.f.owner.QueryRow(context.Background(), `SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype='jsonb'::regtype
		 AND r.rolname='commerce_checkout_writer' AND NOT(r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
		 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND length(obj_description(p.oid,'pg_proc'))>0
		 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.privilege_type='EXECUTE' ORDER BY 1)=ARRAY['commerce_order_attempt_owner']::text[]
		 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.is_grantable)
		 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure($1)`, signature).Scan(&safe)
		if err != nil || !safe {
			t.Fatalf("missing/unsafe allocation definer %s: safe=%v err=%v", signature, safe, err)
		}
	}
}

func TestOrderAttemptAllocationPrivateInputDenials(t *testing.T) {
	e := lbuNew(t)
	foreign := e.newBuyer()
	if foreign.h.quote.CartID == "" || foreign.h.quote.ID == "" || foreign.h.destination.ID == "" {
		t.Fatal("real foreign-owner storefront identifiers missing")
	}
	a := oaAllocationAdmit(t, e, oaFactsSelection(e))
	for _, change := range []string{"cart", "quote", "destination", "attempt", "plaintext"} {
		t.Run(change, func(t *testing.T) {
			tx, plain, quote := oaAllocationPrefix(t, e, a)
			destination := oaAllocationDestination(t, tx, a, plain, quote)
			attempt, want := a.AttemptID, "PT409"
			switch change {
			case "cart": quote.CartID = foreign.h.quote.CartID
			case "quote": quote.QuoteID = foreign.h.quote.ID
			case "destination": destination = foreign.h.destination.ID
			case "attempt": attempt = randomUUID()
			case "plaintext": plain = append([]byte(" "), plain...); want = "PT400"
			}
			_, raw, err := oaAllocationCall(tx, attempt, plain, quote, destination)
			oaFenceCode(t, raw, err, want)
		})
	}
	seed, plain, quote := oaAllocationPrefix(t, e, a)
	destination := oaAllocationDestination(t, seed, a, plain, quote)
	if err := seed.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Actual logins cannot use either private helper, even with valid prior plaintext.
	for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_buyer_runtime", "commerce_checkout_runtime"} {
		t.Run(role, func(t *testing.T) {
			pool := miPool(t, e.p.f, role)
			for _, catalog := range []bool{true, false} {
				tx, err := pool.Begin(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				var raw []byte
				if catalog {
					_, raw, err = oaAllocationCatalog(tx, a.AttemptID, plain)
				} else {
					_, raw, err = oaAllocationCall(tx, a.AttemptID, plain, quote, destination)
				}
				_ = tx.Rollback(context.Background())
				oaFenceCode(t, raw, err, "42501")
			}
		})
	}
	for _, catalog := range []bool{true, false} {
		tx, _ := oaFenceDomainTx(t, e, a, false)
		oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
		var raw []byte
		var err error
		if catalog {
			_, raw, err = oaAllocationCatalog(tx, a.AttemptID, plain)
		} else {
			_, raw, err = oaAllocationCall(tx, a.AttemptID, plain, quote, destination)
		}
		oaFenceCode(t, raw, err, "PT409")
		_ = tx.Rollback(context.Background())
	}
}
