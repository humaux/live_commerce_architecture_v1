// Purpose: REAL_PG acceptance of the private OA1 claim lock/complete seams.
// Depends on: real C1/Open/fence/owner locks, native Begin and accepted claim events.
// Used by: parent CI; partial C2 transactions always roll back, never fake settlement.
// Invariants: I01/I02/I04/I08/I14; exact original origins, aggregate allowance,
// frozen money/version audit, all selected reservations and grant custody in one Tx.
package foundation_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims"
	"livecommerce/internal/inventory"
	"livecommerce/internal/orderattempt"
)

func oaClaimExecutionLock(tx pgx.Tx, attempt string, plain []byte) (string, error) {
	var state string
	// Frozen ABI: order-attempt-v1 §§1/3.1; owner prefix precedes this call.
	err := tx.QueryRow(context.Background(), `SELECT claims.order_attempt_lock_origins($1::uuid,$2::text)`, attempt, string(plain)).Scan(&state)
	return state, err
}

func oaClaimExecutionComplete(tx pgx.Tx, attempt string, plain []byte, order string) error {
	// Frozen ABI: native Begin has created the exact order under retained locks.
	_, err := tx.Exec(context.Background(), `SELECT claims.order_attempt_complete_origins($1::uuid,$2::text,$3::uuid)`, attempt, string(plain), order)
	return err
}

func oaClaimExecutionPrefix(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt) (pgx.Tx, []byte) {
	t.Helper()
	tx, plain := oaFenceDomainTx(t, e, a, true)
	oaAbortCleanupTx(t, tx)
	oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
	var state string
	err := tx.QueryRow(context.Background(), `SELECT buyer.order_attempt_lock_owners($1::uuid,$2::bigint)`, a.AttemptID, a.Generation).Scan(&state)
	if err != nil || state != "active" {
		t.Fatalf("native owner prefix: state=%s err=%v", state, err)
	}
	return tx, plain
}

func oaClaimExecutionBegin(t *testing.T, e *lbuEnv, a orderattempt.AdmissionReceipt, tx pgx.Tx, plain []byte) oaQuoteWire {
	t.Helper()
	// The exact order-level key is a disclosed owner fixture, as in oaAllocationPrefix.
	// No earlier-level lock is acquired after this point.
	oaStateExec(t, tx, `RESET ROLE`)
	oaStateExec(t, tx, `SELECT pg_advisory_xact_lock(hashtextextended('checkout.order|'||tenant_id::text||'|'||store_id::text||'|'||attempt_id::text,0)) FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID)
	quote := oaDestinationQuote(t, tx, a, plain)
	ids, _, err := oaAllocationCatalog(tx, a.AttemptID, plain)
	if err != nil || len(ids) == 0 {
		t.Fatalf("native catalog prefix: %v", err)
	}
	var offline []byte
	err = tx.QueryRow(context.Background(), `SELECT checkout.order_attempt_lock_offline_safety($1::uuid,$2::text)`, a.AttemptID, string(plain)).Scan(&offline)
	if err != nil {
		t.Fatalf("native offline prefix: %v", err)
	}
	destination, _, err := oaDestinationCall(tx, a.AttemptID, plain, quote.CartID)
	if err != nil || destination.DestinationID == "" {
		t.Fatalf("native destination prefix: %v", err)
	}
	allocation, _, err := oaAllocationCall(tx, a.AttemptID, plain, quote, destination.DestinationID)
	if err != nil || allocation.OrderID != a.AttemptID {
		t.Fatalf("native allocation prefix: %v", err)
	}
	lines := []inventory.Line{}
	if len(allocation.Demands) != 0 {
		lines, err = inventory.PlanAllocation(allocation.WarehouseIDs, allocation.Demands, allocation.Balances)
		if err != nil {
			t.Fatal(err)
		}
	}
	order, _, err := oaBeginCall(tx, a.AttemptID, plain, quote, destination.DestinationID, lines)
	if err != nil || order.OrderID != a.AttemptID {
		t.Fatalf("real native Begin claim sentinel: order=%s err=%v", order.OrderID, err)
	}
	return quote
}

func oaClaimExecutionRestored(t *testing.T, tx pgx.Tx) {
	t.Helper()
	var restored bool
	err := tx.QueryRow(context.Background(), `SELECT current_setting('app.tenant_id')='caller-marker'
	 AND current_setting('app.store_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker'
	 AND current_setting('app.buyer_id')='caller-marker' AND current_setting('app.buyer_session_id')='caller-marker'`).Scan(&restored)
	if err != nil || !restored {
		t.Fatalf("claim helper must restore domain context: restored=%v err=%v", restored, err)
	}
}

func oaClaimExecutionMarkers(t *testing.T, tx pgx.Tx) {
	t.Helper()
	oaStateExec(t, tx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),
	 set_config('app.principal_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true),set_config('app.buyer_session_id','caller-marker',true)`)
}

func TestOrderAttemptClaimExecutionCompletesFrozenOriginsAndAllBundles(t *testing.T) {
	e := lbuNew(t)
	ctx := context.Background()
	session, first, b1, conversation := e.scenario()
	secondSession, second := e.session("B2", ltgLive+700, 5)
	accepted := e.h.accepted(t, secondSession, "", "claim-exec-"+t04Tag(), "B2+2")
	e.h.closeWindow(t, secondSession)
	b2 := accepted.BundleID
	e.linkPeer(b2, conversation)
	mustExec(t, e.p.f.owner, `UPDATE live.offers SET live_price_minor=NULL WHERE id=$1`, second.ID)
	selection := oaClaimSelection(e, []string{b1, b2}, []string{first.ID, second.ID}, conversation)
	items := selection["items"].([]map[string]any)
	items[0]["quantity"] = 1
	items[1]["ordinal"] = 3
	selection["items"] = []map[string]any{items[0], {"ordinal": 2, "origin_id": randomUUID(), "bundle_id": b1, "offer_id": first.ID, "sku_id": e.money, "quantity": 1}, items[1]}
	a := oaAllocationAdmit(t, e, selection)
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	var facts struct {
		Claims oaClaimResult `json:"claims"`
	}
	if err := json.Unmarshal(plan.Facts, &facts); err != nil || len(facts.Claims.Lines) != 3 || facts.Claims.Lines[0].LivePriceMinor == nil || facts.Claims.Lines[2].LivePriceMinor != nil {
		t.Fatalf("real C1 live/no-live duplicate origin sentinel: %v", err)
	}
	// Native accepted correction changes the current version after C1, while
	// admission provenance and money stay frozen. Current placement gates do not recur.
	e.h.open(t, session, claims.MatchExact)
	newer := e.h.accepted(t, session, b1, "", "A1+3")
	e.h.closeWindow(t, session)
	if newer.LineVersion <= facts.Claims.Lines[0].LineVersion {
		t.Fatal("accepted correction did not advance native version")
	}
	mustExec(t, e.p.f.owner, `UPDATE live.offers SET active=false,live_price_minor=live_price_minor+900 WHERE id=$1`, first.ID)
	mustExec(t, e.p.f.owner, `UPDATE live.sessions SET lifecycle='archived' WHERE id=$1`, session)
	// Isolated owner fixture supplies an unused, exact own grant for the selected
	// no-live-price bundle. Completion must expire it while retaining audit.
	mustExec(t, e.p.f.owner, `INSERT INTO claims.merchant_origin_grants(tenant_id,store_id,request_id,buyer_id,bundle_id,principal_id,expires_at,create_attempt_id)
	 SELECT tenant_id,store_id,request_id,buyer_id,$2::uuid,principal_id,expires_at,create_attempt_id
	 FROM claims.merchant_origin_grants WHERE create_attempt_id=$1::uuid AND bundle_id=$3::uuid`, a.AttemptID, b2, b1)
	tx, plain := oaClaimExecutionPrefix(t, e, a)
	oaClaimExecutionMarkers(t, tx)
	state, err := oaClaimExecutionLock(tx, a.AttemptID, plain)
	if err != nil || state != "active" {
		t.Fatalf("frozen admission must survive current price/window changes: state=%s err=%v", state, err)
	}
	oaClaimExecutionRestored(t, tx)
	quote := oaClaimExecutionBegin(t, e, a, tx, plain)
	oaClaimExecutionMarkers(t, tx)
	if err = oaClaimExecutionComplete(tx, a.AttemptID, plain, a.AttemptID); err != nil {
		t.Fatalf("complete exact native Begin: %v", err)
	}
	oaClaimExecutionRestored(t, tx)
	oaStateExec(t, tx, `RESET ROLE`)
	var placed, uses, origins, used, unused int
	var quantity, price int64
	err = tx.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM inbox.order_for_buyer WHERE create_attempt_id=$1::uuid AND request_id=$1::uuid AND state='placed' AND order_id=$1::uuid),
	 (SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1::uuid),
	 (SELECT quantity FROM claims.live_price_uses WHERE order_id=$1::uuid AND bundle_id=$2::uuid AND offer_id=$3::uuid),
	 (SELECT unit_price_minor FROM claims.live_price_uses WHERE order_id=$1::uuid AND bundle_id=$2::uuid AND offer_id=$3::uuid),
	 (SELECT count(*) FROM claims.order_origins WHERE order_id=$1::uuid),
	 (SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1::uuid AND bundle_id=$2::uuid AND quote_id=$4::uuid AND consumed_at IS NOT NULL),
	 (SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1::uuid AND bundle_id=$5::uuid AND quote_id IS NULL AND consumed_at IS NULL AND expires_at<=clock_timestamp())`,
		a.AttemptID, b1, first.ID, quote.QuoteID, b2).Scan(&placed, &uses, &quantity, &price, &origins, &used, &unused)
	if err != nil || placed != 2 || uses != 1 || quantity != 2 || price != ltgLive || origins != 2 || used != 1 || unused != 1 {
		t.Fatalf("native all-bundle completion: placed=%d uses=%d quantity=%d price=%d origins=%d used=%d unused=%d err=%v", placed, uses, quantity, price, origins, used, unused, err)
	}
	for _, index := range []int{0, 2} {
		line := facts.Claims.Lines[index]
		var version int64
		var recordedSession string
		var source *string
		var acceptedTime bool
		err = tx.QueryRow(ctx, `SELECT o.line_version,o.session_id::text,o.post_id,o.occurred_at=e.occurred_at
		 FROM claims.order_origins o JOIN claims.events e ON e.tenant_id=o.tenant_id AND e.store_id=o.store_id
		 AND e.bundle_id=o.bundle_id AND e.offer_id=o.offer_id AND e.line_version=o.line_version AND e.outcome='ACCEPTED'
		 WHERE o.order_id=$1 AND o.bundle_id=$2 AND o.offer_id=$3`, a.AttemptID, line.BundleID, line.OfferID).Scan(&version, &recordedSession, &source, &acceptedTime)
		wantSession := session
		if index == 2 {
			wantSession = secondSession
		}
		if err != nil || version != line.LineVersion || recordedSession != wantSession || source != nil || !acceptedTime {
			t.Fatalf("original accepted event audit index=%d version=%d want=%d err=%v", index, version, line.LineVersion, err)
		}
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	oaBeginZeroEffects(t, e, a.AttemptID)
	var pending, grants int
	err = e.p.f.owner.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM inbox.order_for_buyer WHERE create_attempt_id=$1::uuid AND state='pending' AND order_id IS NULL),
	 (SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1::uuid AND quote_id IS NULL AND consumed_at IS NULL AND expires_at>clock_timestamp())`, a.AttemptID).Scan(&pending, &grants)
	if err != nil || pending != 2 || grants != 2 {
		t.Fatalf("outer C2 rollback must retain untouched C1 admission: pending=%d grants=%d err=%v", pending, grants, err)
	}
}

func TestOrderAttemptClaimExecutionRetainsNativePrefixLocks(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conversation := e.scenario()
	a := oaAllocationAdmit(t, e, oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation))
	tx, plain := oaClaimExecutionPrefix(t, e, a)
	state, err := oaClaimExecutionLock(tx, a.AttemptID, plain)
	if err != nil || state != "active" {
		t.Fatalf("positive claim lock sentinel: state=%s err=%v", state, err)
	}
	for _, query := range []string{
		`SELECT id FROM claims.bundles WHERE id=$1::uuid FOR UPDATE NOWAIT`,
		`SELECT offer_id FROM claims.lines WHERE bundle_id=$1::uuid FOR UPDATE NOWAIT`,
		`SELECT request_id FROM inbox.order_for_buyer WHERE bundle_id=$1::uuid FOR UPDATE NOWAIT`,
		`SELECT request_id FROM claims.merchant_origin_grants WHERE bundle_id=$1::uuid FOR UPDATE NOWAIT`,
	} {
		probe, err := e.p.f.owner.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		oaAbortCleanupTx(t, probe)
		_, err = probe.Exec(context.Background(), query, bundle)
		oaFenceCode(t, nil, err, "55P03")
		_ = probe.Rollback(context.Background())
	}
	// The claim prefix must not get ahead of the parent cart/catalog suffix.
	probe, err := e.p.f.owner.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	oaAbortCleanupTx(t, probe)
	var sku string
	err = probe.QueryRow(context.Background(), `SELECT id::text FROM catalog.skus WHERE id=$1 FOR UPDATE NOWAIT`, e.money).Scan(&sku)
	if err != nil || sku != e.money {
		t.Fatalf("claim prefix acquired later catalog lock: %v", err)
	}
}

func TestOrderAttemptClaimExecutionAvailabilityRefusals(t *testing.T) {
	for _, kind := range []string{"quantity", "quantity-duplicate", "purged", "reservation", "request-hash", "grant-deadline", "missing-grant"} {
		t.Run(kind, func(t *testing.T) {
			e := lbuNew(t)
			session, offer, bundle, conversation := e.scenario()
			selection := oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation)
			if kind == "quantity-duplicate" {
				items := selection["items"].([]map[string]any)
				items[0]["quantity"] = 1
				selection["items"] = append(items, map[string]any{"ordinal": 2, "origin_id": randomUUID(), "bundle_id": bundle, "offer_id": offer.ID, "sku_id": e.money, "quantity": 1})
			}
			a := oaAllocationAdmit(t, e, selection)
			want := "PT409"
			switch kind {
			case "quantity", "quantity-duplicate":
				e.h.open(t, session, claims.MatchExact)
				e.h.accepted(t, session, bundle, "", "A1+1")
				e.h.closeWindow(t, session)
			case "purged":
				mustExec(t, e.p.f.owner, `UPDATE claims.bundles SET purged_at=clock_timestamp() WHERE id=$1`, bundle)
				want = "PT404"
			case "reservation":
				mustExec(t, e.p.f.owner, `UPDATE inbox.order_for_buyer SET state='released' WHERE create_attempt_id=$1`, a.AttemptID)
			case "request-hash":
				mustExec(t, e.p.f.owner, `UPDATE inbox.order_for_buyer SET request_hash=sha256('different'::bytea) WHERE create_attempt_id=$1`, a.AttemptID)
			case "grant-deadline":
				mustExec(t, e.p.f.owner, `UPDATE claims.merchant_origin_grants SET expires_at=expires_at-interval '1 second' WHERE create_attempt_id=$1`, a.AttemptID)
			case "missing-grant":
				mustExec(t, e.p.f.owner, `DELETE FROM claims.merchant_origin_grants WHERE create_attempt_id=$1`, a.AttemptID)
			}
			tx, plain := oaClaimExecutionPrefix(t, e, a)
			state, err := oaClaimExecutionLock(tx, a.AttemptID, plain)
			if state != "" {
				t.Fatalf("invalid original quantity/admission returned %q", state)
			}
			oaFenceCode(t, nil, err, want)
			_ = tx.Rollback(context.Background())
			oaBeginZeroEffects(t, e, a.AttemptID)
		})
	}
}

func TestOrderAttemptClaimExecutionExactFenceAndPrivateAuthority(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conversation := e.scenario()
	a := oaAllocationAdmit(t, e, oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation))
	seed, plain := oaClaimExecutionPrefix(t, e, a)
	if err := seed.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, complete := range []bool{false, true} {
		t.Run(map[bool]string{false: "lock", true: "complete"}[complete], func(t *testing.T) {
			for _, kind := range []string{"no-fence", "plaintext", "other-attempt", "runtime", "worker", "checkout"} {
				t.Run(kind, func(t *testing.T) {
					var tx pgx.Tx
					var err error
					want := "PT409"
					body, attempt := plain, a.AttemptID
					if kind == "runtime" || kind == "worker" || kind == "checkout" {
						role := map[string]string{"runtime": "commerce_runtime", "worker": "commerce_claims_worker", "checkout": "commerce_checkout_runtime"}[kind]
						tx, err = miPool(t, e.p.f, role).Begin(context.Background())
						if err != nil {
							t.Fatal(err)
						}
						oaAbortCleanupTx(t, tx)
						want = "42501"
					} else if kind == "no-fence" {
						tx, _ = oaFenceDomainTx(t, e, a, false)
						oaAbortCleanupTx(t, tx)
						oaStateExec(t, tx, `SET LOCAL ROLE commerce_order_attempt_owner`)
					} else {
						tx, _ = oaClaimExecutionPrefix(t, e, a)
						if kind == "plaintext" {
							body = append([]byte(" "), plain...)
							want = "PT400"
						} else {
							attempt = randomUUID()
						}
					}
					if complete {
						err = oaClaimExecutionComplete(tx, attempt, body, a.AttemptID)
					} else {
						_, err = oaClaimExecutionLock(tx, attempt, body)
					}
					oaFenceCode(t, nil, err, want)
					_ = tx.Rollback(context.Background())
				})
			}
		})
	}
}

func TestOrderAttemptClaimExecutionCompleteRequiresExactNativeOrder(t *testing.T) {
	e := lbuNew(t)
	_, offer, bundle, conversation := e.scenario()
	a := oaAllocationAdmit(t, e, oaClaimSelection(e, []string{bundle}, []string{offer.ID}, conversation))
	tx, plain := oaClaimExecutionPrefix(t, e, a)
	state, err := oaClaimExecutionLock(tx, a.AttemptID, plain)
	if err != nil || state != "active" {
		t.Fatalf("positive lock before missing-order counterexample: state=%s err=%v", state, err)
	}
	err = oaClaimExecutionComplete(tx, a.AttemptID, plain, randomUUID())
	oaFenceCode(t, nil, err, "PT404")
	_ = tx.Rollback(context.Background())
	oaBeginZeroEffects(t, e, a.AttemptID)
}

func TestOrderAttemptClaimExecutionManualEmptyOrigins(t *testing.T) {
	e := lbuNew(t)
	a := oaAllocationAdmit(t, e, oaFactsSelection(e))
	tx, plain := oaClaimExecutionPrefix(t, e, a)
	state, err := oaClaimExecutionLock(tx, a.AttemptID, plain)
	if err != nil || state != "active" {
		t.Fatalf("empty manual claim set must be active: state=%s err=%v", state, err)
	}
	oaClaimExecutionBegin(t, e, a, tx, plain)
	if err = oaClaimExecutionComplete(tx, a.AttemptID, plain, a.AttemptID); err != nil {
		t.Fatalf("empty manual claim completion: %v", err)
	}
	oaStateExec(t, tx, `RESET ROLE`)
	var effects int
	err = tx.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM claims.live_price_uses WHERE order_id=$1::uuid)
	 +(SELECT count(*) FROM claims.order_origins WHERE order_id=$1::uuid)
	 +(SELECT count(*) FROM inbox.order_for_buyer WHERE create_attempt_id=$1::uuid)
	 +(SELECT count(*) FROM claims.merchant_origin_grants WHERE create_attempt_id=$1::uuid)`, a.AttemptID).Scan(&effects)
	if err != nil || effects != 0 {
		t.Fatalf("manual completion wrote claim artifacts: count=%d err=%v", effects, err)
	}
	if err = tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	oaBeginZeroEffects(t, e, a.AttemptID)
}
