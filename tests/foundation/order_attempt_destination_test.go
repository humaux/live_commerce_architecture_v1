// Purpose: REAL_PG acceptance of frozen OA1 local entered-store/destination materialization.
// Depends on: real C1 Service.Admit, AEAD PlanKeys.Open, public execution fence and 0179 quote helper.
// Used by: parent-owned CI; all private helper executions roll back, no complete C2 is claimed.
// Invariants: I01/I04/I08/I14; exact creator/cart identity, frozen contact/store facts,
// service safety before pickup/destination locks, no browser capability or order/payment/hold effects.
package foundation_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/checkout"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/orderattempt"
	"livecommerce/internal/storefront"
)

type oaDestinationWire struct {
	DestinationID string `json:"destination_id"`
	Destination storefront.Destination `json:"destination"`
}

type oaDestinationFacts struct {
	Selection struct {
		Customer struct { Name string `json:"name"`; Phone string `json:"phone"` } `json:"customer"`
		Delivery struct { Country string `json:"country"`; Home storefront.HomeAddress `json:"home_address"` } `json:"delivery"`
	} `json:"selection"`
	Catalog struct {
		Service fulfillment.Service `json:"service"`
		Pickup *fulfillment.Pickup `json:"pickup"`
	} `json:"catalog"`
}

func oaDestinationAdmit(t *testing.T, e *lbuEnv, kind, mode string) orderattempt.AdmissionReceipt {
	t.Helper()
	selection := oaFactsSelection(e)
	if kind != "home" {
		code := map[string]string{"cvs_711":"001234", "cvs_familymart":"006543", "cvs_hilife":"012", "cvs_okmart":"0012"}[kind]
		selection = oaMaterialCVS(t, e, kind, code, mode)
	}
	req := oaMaterialRequest(t, e, selection)
	svc, err := orderattempt.NewService(e.p.f.runtime, oaServiceKeys(t))
	if err != nil { t.Fatal(err) }
	a, err := svc.Admit(context.Background(), e.token(), e.store(), req)
	if err != nil || a.State != "PENDING" || a.AttemptID == "" { t.Fatalf("real C1 destination sentinel: state=%s err=%v", a.State, err) }
	return a
}

func oaDestinationCall(tx pgx.Tx, attempt string, plain []byte, cart string) (oaDestinationWire, []byte, error) {
	var raw []byte
	// Calls storefront.order_attempt_materialize_destination, order-attempt-v1 §§2/3/6j.
	err := tx.QueryRow(context.Background(), `SELECT storefront.order_attempt_materialize_destination($1::uuid,$2::text,$3::uuid)`, attempt, string(plain), cart).Scan(&raw)
	var out oaDestinationWire
	if err == nil { err = json.Unmarshal(raw, &out) }
	return out, raw, err
}

func oaDestinationQuote(t *testing.T, tx pgx.Tx, a orderattempt.AdmissionReceipt, plain []byte) oaQuoteWire {
	t.Helper()
	if _, err := tx.Exec(context.Background(), `SET LOCAL ROLE commerce_order_attempt_owner`); err != nil { t.Fatal(err) }
	quote, _, err := oaQuoteCall(tx, a.AttemptID, plain)
	if err != nil || quote.CartID == "" { t.Fatalf("real quote prerequisite: %v", err) }
	return quote
}

func TestOrderAttemptDestinationFrozenReadbackAndRollback(t *testing.T) {
	for _, kind := range []string{"home", "cvs_711", "cvs_familymart", "cvs_hilife", "cvs_okmart"} {
		t.Run(kind, func(t *testing.T) {
			e := lbuNew(t)
			a := oaDestinationAdmit(t, e, kind, "bank_transfer")
			ctx := context.Background()
			browserCart, browserBefore := "", ""
			if kind == "home" {
				browserCart = randomUUID()
				session := randomUUID()
				// Positive same-owner browser-cart sentinel, independent of the attempt cart.
				mustExec(t, e.p.f.owner, `INSERT INTO buyer.capability_sessions(id,tenant_id,store_id,owner_id,token_hash,expires_at)
				 SELECT $2::uuid,tenant_id,store_id,buyer_owner_id,sha256(convert_to(($2::uuid)::text,'UTF8')),clock_timestamp()+interval '1 hour' FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, session)
				mustExec(t, e.p.f.owner, `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,id,creator_session_id,currency,version)
				 SELECT tenant_id,store_id,buyer_owner_id,$2::uuid,$3::uuid,'TWD',7 FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, browserCart, session)
				if err := e.p.f.owner.QueryRow(ctx, `SELECT to_jsonb(c)::text FROM storefront.carts c WHERE id=$1`, browserCart).Scan(&browserBefore); err != nil { t.Fatal(err) }
			}
			tx, plain := oaFenceDomainTx(t, e, a, true)
			quote := oaDestinationQuote(t, tx, a, plain)
			var plan orderattempt.FrozenPlan
			var facts oaDestinationFacts
			if err := json.Unmarshal(plain, &plan); err != nil { t.Fatal(err) }
			if err := json.Unmarshal(plan.Facts, &facts); err != nil { t.Fatal(err) }
			if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id','caller-marker',true),set_config('app.store_id','caller-marker',true),set_config('app.buyer_id','caller-marker',true),set_config('app.buyer_session_id','caller-marker',true),set_config('app.principal_id','caller-marker',true),set_config('app.order_attempt_id','caller-marker',true)`); err != nil { t.Fatal(err) }
			got, _, err := oaDestinationCall(tx, a.AttemptID, plain, quote.CartID)
			if err != nil { t.Fatalf("materialize frozen %s destination after real C1/fence/quote: %v", kind, err) }
			d := got.Destination
			if got.DestinationID == "" || got.DestinationID != d.ID || d.Version != 1 || d.CartID != quote.CartID || d.CartVersion != 1 || d.Kind != kind || d.Country != facts.Selection.Delivery.Country || d.RecipientName != facts.Selection.Customer.Name || d.Phone != facts.Selection.Customer.Phone || !d.SelectedAt.Equal(plan.AdmittedAt) || !d.ExpiresAt.Equal(plan.ExecuteBy) {
				t.Fatal("destination differs from frozen identity/contact/lifetime")
			}
			if kind == "home" {
				if d.Pickup != nil || d.HomeAddress != facts.Selection.Delivery.Home { t.Fatal("frozen home address lost") }
			} else {
				if d.Pickup == nil || facts.Catalog.Pickup == nil || d.Pickup.ID == "" || d.HomeAddress != (storefront.HomeAddress{}) { t.Fatal("entered pickup projection missing") }
				actual, frozen := *d.Pickup, *facts.Catalog.Pickup
				actual.ID = ""
				if !reflect.DeepEqual(actual, frozen) || actual.Namespace != "attempt."+a.AttemptID || actual.VerificationKind != "BUYER_ENTERED" || !actual.AttestedAt.Equal(plan.AdmittedAt) || !actual.ValidUntil.Equal(plan.AdmittedAt.Add(24*time.Hour)) { t.Fatal("pickup changed frozen source, namespace or lifetime") }
			}
			var restored bool
			if err = tx.QueryRow(ctx, `SELECT current_setting('app.tenant_id')='caller-marker' AND current_setting('app.store_id')='caller-marker' AND current_setting('app.buyer_id')='caller-marker' AND current_setting('app.buyer_session_id')='caller-marker' AND current_setting('app.principal_id')='caller-marker' AND current_setting('app.order_attempt_id')='caller-marker'`).Scan(&restored); err != nil || !restored { t.Fatalf("helper leaked context: %v", err) }
			if _, err = tx.Exec(ctx, `RESET ROLE`); err != nil { t.Fatal(err) }
			var bound bool
			err = tx.QueryRow(ctx, `SELECT d.creator_kind='order_attempt' AND d.creator_session_id IS NULL AND d.create_attempt_id=a.attempt_id AND d.owner_id=a.buyer_owner_id AND d.cart_id=$2::uuid AND d.cart_version=1 AND d.version=1 AND d.selected_at=a.admitted_at AND d.expires_at=a.execute_by
			 AND h.destination_id=d.id AND h.current_version=1 AND ev.creator_kind='order_attempt' AND ev.session_id IS NULL AND ev.create_attempt_id=a.attempt_id AND ev.action='destination.selected' AND ev.created_at=a.admitted_at
			 FROM ops.order_attempts a JOIN storefront.destination_snapshots d ON d.create_attempt_id=a.attempt_id JOIN storefront.destination_heads h ON h.destination_id=d.id JOIN storefront.destination_events ev ON ev.destination_id=d.id WHERE a.attempt_id=$1 AND d.id=$3::uuid`, a.AttemptID, quote.CartID, d.ID).Scan(&bound)
			if err != nil || !bound { t.Fatalf("destination/head/event creator identity: %v", err) }
			if d.Pickup != nil {
				err = tx.QueryRow(ctx, `SELECT p.buyer_owner_id=a.buyer_owner_id AND p.create_attempt_id=a.attempt_id AND p.principal_id IS NULL AND p.verification_kind='BUYER_ENTERED' AND p.namespace='attempt.'||a.attempt_id::text AND p.version=1 AND p.attested_at=a.admitted_at AND p.valid_until=a.admitted_at+interval '24 hours' AND h.pickup_id=p.id AND h.current_version=1 AND h.enabled
				 FROM ops.order_attempts a JOIN fulfillment.pickup_versions p ON p.create_attempt_id=a.attempt_id JOIN fulfillment.pickup_heads h ON h.pickup_id=p.id WHERE a.attempt_id=$1 AND p.id=$2::uuid`, a.AttemptID, d.Pickup.ID).Scan(&bound)
				if err != nil || !bound { t.Fatalf("pickup exact immutable provenance/head: %v", err) }
			}
			encoded, err := json.Marshal(checkout.Snapshot{Destination:d})
			if err != nil { t.Fatal(err) }
			var decoded checkout.Snapshot
			if err = json.Unmarshal(encoded, &decoded); err != nil || !reflect.DeepEqual(decoded.Destination, d) { t.Fatal("typed checkout destination snapshot loses fields") }
			var effects int
			if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM checkout.orders WHERE create_attempt_id=$1)+(SELECT count(*) FROM inventory.reservations WHERE create_attempt_id=$1)`, a.AttemptID).Scan(&effects); err != nil || effects != 0 { t.Fatalf("destination acquired order/hold effects: count=%d err=%v", effects, err) }
			var sessions int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM buyer.capability_sessions WHERE owner_id=$1`, plan.BuyerOwnerID).Scan(&sessions); err != nil || sessions != map[bool]int{false:0,true:1}[kind=="home"] { t.Fatalf("destination minted browser session: count=%d err=%v", sessions, err) }
			if browserCart != "" {
				var browserAfter string
				if err = tx.QueryRow(ctx, `SELECT to_jsonb(c)::text FROM storefront.carts c WHERE id=$1`, browserCart).Scan(&browserAfter); err != nil || browserBefore != browserAfter { t.Fatalf("successful attempt materialization changed browser cart: %v", err) }
			}
			if err = tx.Rollback(ctx); err != nil { t.Fatal(err) }
			if err = e.p.f.owner.QueryRow(ctx, `SELECT (SELECT count(*) FROM storefront.destination_snapshots WHERE create_attempt_id=$1)+(SELECT count(*) FROM storefront.destination_events WHERE create_attempt_id=$1)+(SELECT count(*) FROM fulfillment.pickup_versions WHERE create_attempt_id=$1)+(SELECT count(*) FROM storefront.carts WHERE create_attempt_id=$1)+(SELECT count(*) FROM storefront.quotes WHERE create_attempt_id=$1)`, a.AttemptID).Scan(&effects); err != nil || effects != 0 { t.Fatalf("outer rollback left destination/store/cart/quote effects: count=%d err=%v", effects, err) }
		})
	}
}

func TestOrderAttemptDestinationServiceSafetyAndFrozenDisplayDrift(t *testing.T) {
	for _, change := range []string{"fee-version-visibility", "disabled", "kind-no-longer-supported"} {
		t.Run(change, func(t *testing.T) {
			e := lbuNew(t)
			a := oaDestinationAdmit(t, e, "cvs_711", "pay_at_pickup")
			plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
			var facts oaDestinationFacts
			if err := json.Unmarshal(plan.Facts, &facts); err != nil { t.Fatal(err) }
			code := facts.Catalog.Service.Code
			if change == "fee-version-visibility" {
				in := e.svcs[code]
				dsPolicy(t, e.p.cqHarness, in, 1, 237, true)
				e.updateService(code, func(in *fulfillment.ServiceInput) { in.PolicyVersion=2; in.Visible=false; in.NameEN="Changed display" })
			} else {
				e.updateService(code, func(in *fulfillment.ServiceInput) { if change == "disabled" { in.Enabled=false } else { in.DeliveryKind="cvs_familymart" } })
			}
			tx, plain := oaFenceDomainTx(t, e, a, true)
			quote := oaDestinationQuote(t, tx, a, plain)
			got, raw, err := oaDestinationCall(tx, a.AttemptID, plain, quote.CartID)
			if change != "fee-version-visibility" { oaFenceCode(t, raw, err, "PT422"); return }
			if err != nil || got.Destination.Pickup == nil || got.Destination.Pickup.Name != facts.Catalog.Pickup.Name { t.Fatalf("display/fee/version drift revoked accepted source: %v", err) }
			var frozenQuote storefront.Quote
			if err = json.Unmarshal(quote.Quote, &frozenQuote); err != nil || !reflect.DeepEqual(frozenQuote.Amount, plan.Calculation) { t.Fatal("current delivery fee repriced accepted quote") }
		})
	}
}

func TestOrderAttemptDestinationWorkerWithoutBearer(t *testing.T) {
	e := lbuNew(t)
	a := oaDestinationAdmit(t, e, "cvs_711", "bank_transfer")
	worker := miPool(t, e.p.f, "commerce_claims_worker")
	ctx := context.Background()
	var login string
	if err := worker.QueryRow(ctx, `SELECT session_user::text`).Scan(&login); err != nil { t.Fatal(err) }
	tx, err := e.p.f.owner.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL SESSION AUTHORIZATION `+pgx.Identifier{login}.Sanitize()); err != nil { t.Fatal(err) }
	raw, err := oaFenceCall(ctx, tx, nil, e.store(), a.AttemptID, a.Generation)
	if err != nil { t.Fatal(err) }
	opened := oaFenceOpen(t, raw, a)
	plan, err := oaServiceKeys(t).Open(*opened.Binding, *opened.Envelope)
	if err != nil { t.Fatal(err) }
	plain, err := json.Marshal(plan)
	if err != nil { t.Fatal(err) }
	if _, err = tx.Exec(ctx, `SET LOCAL SESSION AUTHORIZATION DEFAULT`); err != nil { t.Fatal(err) }
	quote := oaDestinationQuote(t, tx, a, plain)
	got, _, err := oaDestinationCall(tx, a.AttemptID, plain, quote.CartID)
	if err != nil || got.Destination.Pickup == nil { t.Fatalf("actual no-bearer worker fence cannot materialize local store: %v", err) }
	if _, err = tx.Exec(ctx, `RESET ROLE`); err != nil { t.Fatal(err) }
	var sessions int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM buyer.capability_sessions WHERE owner_id=$1`, plan.BuyerOwnerID).Scan(&sessions); err != nil || sessions != 0 { t.Fatalf("worker minted bearer session: count=%d err=%v", sessions, err) }
}

func TestOrderAttemptDestinationDenialsAndExactACL(t *testing.T) {
	e := lbuNew(t)
	a := oaDestinationAdmit(t, e, "home", "bank_transfer")
	ctx := context.Background()
	for _, role := range []string{"commerce_runtime", "commerce_claims_worker", "commerce_buyer_runtime", "commerce_checkout_writer", "commerce_privacy_writer"} {
		t.Run(role, func(t *testing.T) {
			tx, plain := oaFenceDomainTx(t, e, a, true)
			quote := oaDestinationQuote(t, tx, a, plain)
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+pgx.Identifier{role}.Sanitize()); err != nil { t.Fatal(err) }
			_, raw, err := oaDestinationCall(tx, a.AttemptID, plain, quote.CartID)
			oaFenceCode(t, raw, err, "42501")
		})
	}
	seed, exact := oaFenceDomainTx(t, e, a, true)
	if err := seed.Rollback(ctx); err != nil { t.Fatal(err) }
	for _, mode := range []string{"no-fence", "changed-plan-bytes", "foreign-attempt", "foreign-cart"} {
		t.Run(mode, func(t *testing.T) {
			tx, plain := oaFenceDomainTx(t, e, a, mode != "no-fence")
			id, cart, want := a.AttemptID, randomUUID(), "PT409"
			if mode == "no-fence" { plain=exact } else { cart=oaDestinationQuote(t, tx, a, plain).CartID }
			if mode == "changed-plan-bytes" { plain=append([]byte(" "), plain...); want="PT400" }
			if mode == "foreign-attempt" { id=randomUUID() }
			if mode == "foreign-cart" { cart=randomUUID() }
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_order_attempt_owner`); err != nil { t.Fatal(err) }
			_, raw, err := oaDestinationCall(tx, id, plain, cart)
			oaFenceCode(t, raw, err, want)
		})
	}
	var safe bool
	err := e.p.f.owner.QueryRow(ctx, `SELECT p.prosecdef AND p.provolatile='v' AND p.prorettype='jsonb'::regtype
	 AND r.rolname='commerce_buyer_writer' AND NOT(r.rolcanlogin OR r.rolsuper OR r.rolbypassrls)
	 AND p.proconfig=ARRAY['search_path=pg_catalog']::text[] AND length(obj_description(p.oid,'pg_proc'))>0
	 AND ARRAY(SELECT pg_get_userbyid(a.grantee)::text FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.privilege_type='EXECUTE' ORDER BY 1)=ARRAY['commerce_order_attempt_owner']::text[]
	 AND NOT EXISTS(SELECT 1 FROM aclexplode(p.proacl) a WHERE a.grantee<>p.proowner AND a.is_grantable)
	 FROM pg_proc p JOIN pg_roles r ON r.oid=p.proowner WHERE p.oid=to_regprocedure('storefront.order_attempt_materialize_destination(uuid,text,uuid)')`).Scan(&safe)
	if err != nil || !safe { t.Fatalf("missing/unsafe exact destination ACL: safe=%v err=%v", safe, err) }
}

func TestOrderAttemptDestinationExistingCartCannotAuthorizeMaterialization(t *testing.T) {
	for _, browser := range []bool{false, true} {
		t.Run(map[bool]string{false:"earlier-transaction-attempt-cart", true:"browser-cart"}[browser], func(t *testing.T) {
			e := lbuNew(t)
			a := oaDestinationAdmit(t, e, "home", "bank_transfer")
			ctx := context.Background()
			cart, session := randomUUID(), randomUUID()
			// A browser cart may exist independently. An earlier PENDING attempt
			// cart must now be impossible at the native commit boundary (0182).
			if browser {
				mustExec(t, e.p.f.owner, `INSERT INTO buyer.capability_sessions(id,tenant_id,store_id,owner_id,token_hash,expires_at)
				 SELECT $2::uuid,tenant_id,store_id,buyer_owner_id,sha256(convert_to(($2::uuid)::text,'UTF8')),clock_timestamp()+interval '1 hour' FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, session)
				mustExec(t, e.p.f.owner, `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,id,creator_session_id,currency,version)
				 SELECT tenant_id,store_id,buyer_owner_id,$2::uuid,$3::uuid,'TWD',7 FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, cart, session)
			} else {
				prior, err := e.p.f.owner.Begin(ctx)
				if err != nil { t.Fatal(err) }
				defer prior.Rollback(context.Background())
				oaStateExec(t, prior, `INSERT INTO storefront.carts(tenant_id,store_id,owner_id,id,creator_session_id,currency,version,cart_kind,create_attempt_id)
				 SELECT tenant_id,store_id,buyer_owner_id,$2::uuid,NULL,'TWD',1,'order_attempt',attempt_id FROM ops.order_attempts WHERE attempt_id=$1`, a.AttemptID, cart)
				oaFenceCode(t, nil, prior.Commit(ctx), "23514")
				var absent bool
				if err = e.p.f.owner.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM storefront.carts WHERE id=$1)`, cart).Scan(&absent); err != nil || !absent { t.Fatalf("earlier pending cart escaped commit guard: %v", err) }
			}
			tx, plain := oaFenceDomainTx(t, e, a, true)
			if _, err := tx.Exec(ctx, `SET LOCAL ROLE commerce_order_attempt_owner`); err != nil { t.Fatal(err) }
			_, raw, err := oaDestinationCall(tx, a.AttemptID, plain, cart)
			oaFenceCode(t, raw, err, "PT409")
			if err = tx.Rollback(ctx); err != nil { t.Fatal(err) }
			var unchanged bool
			err = e.p.f.owner.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM storefront.destination_snapshots WHERE cart_id=$1)
			 AND CASE WHEN $2::boolean THEN EXISTS(SELECT 1 FROM storefront.carts WHERE id=$1 AND version=7)
			 ELSE NOT EXISTS(SELECT 1 FROM storefront.carts WHERE id=$1) END`, cart, browser).Scan(&unchanged)
			if err != nil || !unchanged { t.Fatalf("rejected source cart changed: %v", err) }
		})
	}
}

func TestOrderAttemptDestinationServiceBeforePickupBarrier(t *testing.T) {
	e := lbuNew(t)
	a := oaDestinationAdmit(t, e, "cvs_711", "bank_transfer")
	plan, _ := oaMaterialPlan(t, e, oaServiceKeys(t), a.AttemptID)
	var facts oaDestinationFacts
	if err := json.Unmarshal(plan.Facts, &facts); err != nil { t.Fatal(err) }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := e.p.f.owner.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer holder.Rollback(context.Background())
	var version int64
	err = holder.QueryRow(ctx, `SELECT current_version FROM fulfillment.service_heads WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country='TW' AND code=$4 FOR UPDATE`, e.tenant(), e.store(), e.p.market.ID, facts.Catalog.Service.Code).Scan(&version)
	if err != nil || version < 1 { t.Fatalf("held service positive sentinel: %v", err) }
	tx, plain := oaFenceDomainTx(t, e, a, true)
	quote := oaDestinationQuote(t, tx, a, plain)
	var pid int
	if err = tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil { t.Fatal(err) }
	// The helper is genuinely blocked by the existing service head. Only inspect
	// the pickup advisory key after PG reports that exact backend waiting on a lock.
	done := make(chan error, 1)
	go func() { defer close(done); _, _, callErr := oaDestinationCall(tx, a.AttemptID, plain, quote.CartID); done <- callErr }()
	defer func() { _ = holder.Rollback(context.Background()); select { case <-done: case <-ctx.Done(): } }()
	ticker := time.NewTicker(10*time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err = e.p.f.owner.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock' AND query LIKE '%order_attempt_materialize_destination%')`, pid).Scan(&blocked)
		if err != nil { t.Fatal(err) }
		if blocked { break }
		select {
		case callErr := <-done: t.Fatalf("helper bypassed held service safety lock: %v", callErr)
		case <-ticker.C:
		case <-ctx.Done(): t.Fatal("service barrier never reached")
		}
	}
	probe, err := e.p.f.owner.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer probe.Rollback(context.Background())
	key := "fulfillment.pickup|"+e.tenant()+"|"+e.store()+"|cvs_711|attempt."+a.AttemptID+"|"+facts.Catalog.Pickup.Code
	var free bool
	if err = probe.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1,0))`, key).Scan(&free); err != nil || !free { t.Fatalf("helper acquired pickup before blocked service: free=%v err=%v", free, err) }
	if err = probe.Rollback(ctx); err != nil { t.Fatal(err) }
	if err = holder.Rollback(ctx); err != nil { t.Fatal(err) }
	select {
	case err = <-done: if err != nil { t.Fatalf("helper failed after service release: %v", err) }
	case <-ctx.Done(): t.Fatal("helper did not finish after service release")
	}
	// Successful materialization must retain both graph levels until outer rollback.
	probe, err = e.p.f.owner.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer probe.Rollback(context.Background())
	if err = probe.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1,0))`, key).Scan(&free); err != nil || free { t.Fatalf("pickup serialization not retained: free=%v err=%v", free, err) }
	_, err = probe.Exec(ctx, `SELECT current_version FROM fulfillment.service_heads WHERE tenant_id=$1 AND store_id=$2 AND market_id=$3 AND country='TW' AND code=$4 FOR UPDATE NOWAIT`, e.tenant(), e.store(), e.p.market.ID, facts.Catalog.Service.Code)
	oaFenceCode(t, nil, err, "55P03")
}
