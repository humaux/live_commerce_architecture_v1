package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

func TestBuyerCheckoutOwnersMayReusePublicKey(t *testing.T) {
	b := bcSetup(t)
	left, e := b.begin("owner-shared-key")
	if e != nil {
		t.Fatal(e)
	}
	b.prepare(t, mustIssue(t, b.cqHarness.service, b.f.storeA1), []storefront.Item{{SKUID: b.stock.skus[0].ID, Quantity: 2}})
	right, e := b.begin("owner-shared-key")
	if e != nil {
		t.Fatal(e)
	}
	if left.OrderID == right.OrderID {
		t.Fatal("owners shared a hold")
	}
	if got := countRows(t, b.f.owner, `SELECT count(DISTINCT command_key) FROM inventory.ledger WHERE checkout_id IN ($1,$2)`, left.OrderID, right.OrderID); got != 2 {
		t.Fatalf("ledger key collision: %d", got)
	}
}

func bcCVS(t *testing.T) bcHarness {
	t.Helper()
	b := bcSetup(t)
	in := b.delivery
	in.ExpectedVersion = 1
	in.DeliveryKind = "cvs_familymart"
	if _, e := dsSet(b.cqHarness, t04Key("bc-cvs-service"), in); e != nil {
		t.Fatal(e)
	}
	p, e := bdAttest(b.cqHarness, t04Key("bc-pickup"), fulfillment.PickupInput{Kind: "cvs_familymart", Namespace: "fixture." + t04Tag(), Code: "017888", Name: "Synthetic convenience store", Address: "Synthetic convenience address", EvidenceRef: "synthetic only", TTLSeconds: 3600})
	if e != nil {
		t.Fatal(e)
	}
	b.destination, e = bdSet(b.cqHarness, t04Key("bc-cvs-select"), storefront.DestinationInput{ExpectedVersion: 1, CartVersion: b.input.CartVersion, Kind: "cvs_familymart", Country: "TW", RecipientName: "Synthetic Recipient", Phone: "+886900000002", PickupID: p.ID})
	if e != nil {
		t.Fatal(e)
	}
	b.input.DestinationID = b.destination.ID
	b.input.ServiceVersion = 2
	return b
}

func TestBuyerCheckoutCarrierNeutralPickupAndRevocation(t *testing.T) {
	t.Run("selection", func(t *testing.T) {
		b := bcCVS(t)
		r, e := b.begin(t04Key("bc-cvs"))
		if e != nil {
			t.Fatal(e)
		}
		o, e := b.service.Get(context.Background(), b.cap.Token, b.f.storeA1, r.OrderID)
		if e != nil || o.Snapshot.Destination.Pickup == nil || o.Snapshot.Destination.Pickup.Code != "017888" || o.Snapshot.Service.BindingID != "" || o.FulfillmentState != "MANUAL_UNASSIGNED" {
			t.Fatalf("carrier-neutral snapshot: %v", e)
		}
		if o.Snapshot.Allocation.ServiceVersion != 1 || o.Snapshot.Service.Version != 2 {
			t.Fatal("observed service version must remain provenance, not force allocation rewrite")
		}
	})
	t.Run("revoked", func(t *testing.T) {
		b := bcCVS(t)
		before := b.facts(t)
		if e := bdRevoke(b.cqHarness, t04Key("bc-revoke"), b.destination.Pickup.ID); e != nil {
			t.Fatal(e)
		}
		if _, e := b.begin(t04Key("bc-revoked")); e == nil {
			t.Fatal("revoked pickup held inventory")
		}
		if b.facts(t) != before {
			t.Fatal("revoked source left partial hold")
		}
	})
}

func TestBuyerCheckoutReceiptWaitReauthenticates(t *testing.T) {
	b := bcSetup(t)
	key := t04Key("bc-replay-expiry")
	if _, e := b.begin(key); e != nil {
		t.Fatal(e)
	}
	name := "bc-replay-wait-" + t04Tag()
	pool, e := platform.OpenCheckoutPool(context.Background(), withApplicationName(t, b.poolURL, name))
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	b.service = bcService(t, pool)
	holder, e := b.f.owner.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer holder.Rollback(context.Background())
	lockKey := "checkout.begin|" + b.cap.Scope.TenantID + "|" + b.cap.Scope.StoreID + "|" + b.cap.Scope.OwnerID + "|" + key
	if _, e = holder.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); e != nil {
		t.Fatal(e)
	}
	var expiry time.Time
	if e = b.f.owner.QueryRow(context.Background(), `UPDATE buyer.capability_sessions SET expires_at=clock_timestamp()+interval '700 milliseconds' WHERE id=$1 RETURNING expires_at`, b.cap.Scope.SessionID).Scan(&expiry); e != nil {
		t.Fatal(e)
	}
	before := b.facts(t)
	done := make(chan error, 1)
	go func() { _, e := b.begin(key); done <- e }()
	waitForDatabaseLock(t, b.f.owner, name)
	var valid bool
	if e = b.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()<$1`, expiry).Scan(&valid); e != nil || !valid {
		t.Fatalf("missed replay wait window: %v", e)
	}
	mustExec(t, b.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
	if e = holder.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = waitError(t, done); !errors.Is(e, buyer.ErrUnauthorized) {
		t.Fatalf("expired replay accepted: %v", e)
	}
	if b.facts(t) != before {
		t.Fatal("replay changed facts")
	}
}

func TestBuyerCheckoutFinalQuoteClockAfterBalanceWait(t *testing.T) {
	b := bcSetup(t)
	name := "bc-quote-wait-" + t04Tag()
	pool, e := platform.OpenCheckoutPool(context.Background(), withApplicationName(t, b.poolURL, name))
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	b.service = bcService(t, pool)
	holder, e := b.f.owner.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer holder.Rollback(context.Background())
	if _, e = holder.Exec(context.Background(), `SELECT 1 FROM inventory.balances WHERE warehouse_id=$1 AND sku_id=$2 FOR UPDATE`, b.stock.warehouse.ID, b.stock.skus[0].ID); e != nil {
		t.Fatal(e)
	}
	var expiry time.Time
	if e = b.f.owner.QueryRow(context.Background(), `WITH timing AS(SELECT clock_timestamp()+interval '700 milliseconds' AS until)
	 UPDATE storefront.quotes SET created_at=timing.until-interval '60 seconds',expires_at=timing.until,
	 snapshot=jsonb_set(jsonb_set(snapshot,'{created_at}',to_jsonb(timing.until-interval '60 seconds')),'{expires_at}',to_jsonb(timing.until))
	 FROM timing WHERE id=$1 RETURNING expires_at`, b.quote.ID).Scan(&expiry); e != nil {
		t.Fatal(e)
	}
	before := b.facts(t)
	done := make(chan error, 1)
	go func() { _, e := b.begin(t04Key("bc-quote-expiry")); done <- e }()
	waitForDatabaseLock(t, b.f.owner, name)
	var valid bool
	if e = b.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()<$1`, expiry).Scan(&valid); e != nil || !valid {
		t.Fatalf("missed quote wait window: %v", e)
	}
	mustExec(t, b.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
	if e = holder.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = waitError(t, done); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("expired quote accepted: %v", e)
	}
	if b.facts(t) != before {
		t.Fatal("expired quote left hold")
	}
}

func TestBuyerCheckoutExpiryRollbackAndRetry(t *testing.T) {
	for _, table := range []string{"checkout.events", "inventory.ledger"} {
		t.Run(table, func(t *testing.T) {
			b := bcSetup(t)
			r, e := b.begin(t04Key("bc-expiry-fault"))
			if e != nil {
				t.Fatal(e)
			}
			bcDue(t, b, r)
			name := "bc_expiry_fault_" + t04Tag()
			mustExec(t, b.f.owner, fmt.Sprintf(`CREATE FUNCTION public.%s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF current_setting('app.buyer_id',true)=%s THEN RAISE EXCEPTION 'synthetic expiry fault'; END IF; RETURN NEW; END $$`, name, quoteLiteral(b.cap.Scope.OwnerID)))
			mustExec(t, b.f.owner, `CREATE TRIGGER `+name+` BEFORE INSERT ON `+table+` FOR EACH ROW EXECUTE FUNCTION public.`+name+`()`)
			t.Cleanup(func() {
				mustExec(t, b.f.owner, `DROP TRIGGER IF EXISTS `+name+` ON `+table)
				mustExec(t, b.f.owner, `DROP FUNCTION public.`+name+`()`)
			})
			before := b.facts(t)
			if _, e = b.expiry.Exec(context.Background(), `SELECT * FROM checkout.expire_held($1,1)`, r.OrderID); e == nil {
				t.Fatal("expiry fault did not fire")
			}
			if b.facts(t) != before {
				t.Fatal("failed expiry left partial facts")
			}
			var state string
			var reserved int64
			if e = b.f.owner.QueryRow(context.Background(), `SELECT r.state,b.reserved FROM inventory.reservations r JOIN inventory.reservation_lines l ON l.tenant_id=r.tenant_id AND l.store_id=r.store_id AND l.reservation_id=r.id JOIN inventory.balances b ON b.tenant_id=l.tenant_id AND b.store_id=l.store_id AND b.warehouse_id=l.warehouse_id AND b.sku_id=l.sku_id WHERE r.id=$1`, r.OrderID).Scan(&state, &reserved); e != nil || state != "HELD" || reserved != 2 {
				t.Fatalf("failed expiry released stock: %s/%d %v", state, reserved, e)
			}
			mustExec(t, b.f.owner, `DROP TRIGGER `+name+` ON `+table)
			if got := bcExpire(t, b, r, 1); got != "EXPIRED" {
				t.Fatalf("retry=%s", got)
			}
		})
	}
}

func TestBuyerCheckoutJobInvisibleUntilCommit(t *testing.T) {
	b := bcSetup(t)
	name := "bc_commit_" + t04Tag()
	lockKey := "checkout.commit.fixture." + name
	pool, e := platform.OpenCheckoutPool(context.Background(), withApplicationName(t, b.poolURL, name))
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	b.service = bcService(t, pool)
	mustExec(t, b.f.owner, `CREATE FUNCTION public.`+name+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.owner_id=`+quoteLiteral(b.cap.Scope.OwnerID)+`::uuid THEN PERFORM pg_advisory_xact_lock(hashtextextended(`+quoteLiteral(lockKey)+`,0)); END IF; RETURN NEW; END $$`)
	mustExec(t, b.f.owner, `CREATE TRIGGER `+name+` BEFORE INSERT ON checkout.orders FOR EACH ROW EXECUTE FUNCTION public.`+name+`()`)
	t.Cleanup(func() {
		mustExec(t, b.f.owner, `DROP TRIGGER `+name+` ON checkout.orders`)
		mustExec(t, b.f.owner, `DROP FUNCTION public.`+name+`()`)
	})
	holder, e := b.f.owner.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer holder.Rollback(context.Background())
	if _, e = holder.Exec(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); e != nil {
		t.Fatal(e)
	}
	before := b.facts(t)
	done := make(chan error, 1)
	go func() { _, e := b.begin(t04Key("bc-commit")); done <- e }()
	waitForDatabaseLock(t, b.f.owner, name)
	// The gate is after River InsertTx and before the first order INSERT. A
	// different connection must still see zero new orders, stock facts AND jobs.
	if got := b.facts(t); got != before {
		t.Fatalf("uncommitted checkout visible: %v -> %v", before, got)
	}
	if e = holder.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = waitError(t, done); e != nil {
		t.Fatal(e)
	}
	after := b.facts(t)
	for i := range after {
		if after[i]-before[i] != 1 {
			t.Fatalf("committed facts[%d] missing", i)
		}
	}
}

func TestBuyerCheckoutFinalPickupClockAfterBalanceWait(t *testing.T) {
	b := bcCVS(t)
	name := "bc-source-wait-" + t04Tag()
	pool, e := platform.OpenCheckoutPool(context.Background(), withApplicationName(t, b.poolURL, name))
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	b.service = bcService(t, pool)
	holder, e := b.f.owner.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer holder.Rollback(context.Background())
	if _, e = holder.Exec(context.Background(), `SELECT 1 FROM inventory.balances WHERE warehouse_id=$1 AND sku_id=$2 FOR UPDATE`, b.stock.warehouse.ID, b.stock.skus[0].ID); e != nil {
		t.Fatal(e)
	}
	var expiry time.Time
	if e = b.f.owner.QueryRow(context.Background(), `UPDATE fulfillment.pickup_versions SET valid_until=clock_timestamp()+interval '700 milliseconds' WHERE id=$1 RETURNING valid_until`, b.destination.Pickup.ID).Scan(&expiry); e != nil {
		t.Fatal(e)
	}
	mustExec(t, b.f.owner, `UPDATE storefront.destination_snapshots SET expires_at=$2 WHERE id=$1`, b.destination.ID, expiry)
	before := b.facts(t)
	done := make(chan error, 1)
	go func() { _, e := b.begin(t04Key("bc-pickup-expiry")); done <- e }()
	waitForDatabaseLock(t, b.f.owner, name)
	var valid bool
	if e = b.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()<$1`, expiry).Scan(&valid); e != nil || !valid {
		t.Fatalf("missed source wait window: %v", e)
	}
	mustExec(t, b.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
	if e = holder.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = waitError(t, done); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("expired source accepted: %v", e)
	}
	if b.facts(t) != before {
		t.Fatal("expired source left hold")
	}
}

func TestBuyerCheckoutForeignInputsCannotCreateHold(t *testing.T) {
	b := bcSetup(t)
	foreign := b
	foreign.prepare(t, mustIssue(t, b.cqHarness.service, b.f.storeA1), []storefront.Item{{SKUID: b.stock.skus[0].ID, Quantity: 1}})
	for _, kind := range []string{"quote", "destination", "store"} {
		t.Run(kind, func(t *testing.T) {
			in := b.input
			store := b.f.storeA1
			switch kind {
			case "quote":
				in.QuoteID = foreign.input.QuoteID
			case "destination":
				in.DestinationID = foreign.input.DestinationID
			case "store":
				store = b.f.storeA2
			}
			before := b.facts(t)
			if _, e := b.service.Begin(context.Background(), b.cap.Token, store, t04Key("bc-foreign"), in); e == nil {
				t.Fatal("foreign input accepted")
			}
			if b.facts(t) != before {
				t.Fatal("foreign input left facts")
			}
		})
	}
}
