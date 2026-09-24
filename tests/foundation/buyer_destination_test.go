package foundation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

// All addresses and attestation evidence below are synthetic, not official
// directory verification or evidence that a carrier serves these locations.
func bdSetup(t *testing.T) (cqHarness, storefront.Cart, fulfillment.PickupInput) {
	t.Helper()
	h := cqSetup(t)
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
	 VALUES($1,$2,$3,'integration:manage') ON CONFLICT DO NOTHING`, h.f.tenantA, h.f.storeA1, h.f.principalA)
	return h, h.oneCart(t), fulfillment.PickupInput{Kind: "cvs_familymart", Namespace: "fixture." + t04Tag(), Code: "017888", Name: "Synthetic pickup", Address: "Synthetic pickup address", EvidenceRef: "fixture-only private evidence", TTLSeconds: 3600}
}

func bdAttest(h cqHarness, key string, in fulfillment.PickupInput) (fulfillment.Pickup, error) {
	return t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Pickup, error) {
		return fulfillment.AttestPickup(context.Background(), tx, s, h.f.tokens["a"], key, in)
	})
}

func bdRevoke(h cqHarness, key, id string) error {
	_, err := t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) {
		return 0, fulfillment.RevokePickup(context.Background(), tx, s, h.f.tokens["a"], key, id)
	})
	return err
}

func bdPickup(h cqHarness, cap buyer.Capability, id string, lock bool) (fulfillment.Pickup, error) {
	return cqBuyer(h.a.runtime, cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (fulfillment.Pickup, error) {
		if lock {
			return fulfillment.LockPickup(ctx, tx, s, id)
		}
		return fulfillment.ReadPickup(ctx, tx, s, id)
	})
}

func bdHome(c storefront.Cart) storefront.DestinationInput {
	return storefront.DestinationInput{CartVersion: c.Version, Kind: "home", Country: "TW", RecipientName: "Synthetic Buyer", Phone: "+886900000001", HomeAddress: storefront.HomeAddress{City: "Synthetic city", Line1: "Synthetic home address"}}
}

func bdSet(h cqHarness, key string, in storefront.DestinationInput) (storefront.Destination, error) {
	return cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Destination, error) {
		return storefront.SetDestination(ctx, tx, s, key, in)
	})
}

func bdGet(h cqHarness, cap buyer.Capability, id string) (storefront.Destination, error) {
	return cqBuyer(h.a.runtime, cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Destination, error) {
		return storefront.GetDestination(ctx, tx, s, id)
	})
}

func bdValidate(h cqHarness, d storefront.Destination) (storefront.Destination, error) {
	return cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Destination, error) {
		return storefront.RevalidateDestination(ctx, tx, s, d.ID, d.CartVersion, d.Country, d.Kind)
	})
}

func bdCVS(t *testing.T) (cqHarness, storefront.Cart, fulfillment.PickupInput, storefront.DestinationInput, storefront.Destination) {
	t.Helper()
	h, c, p := bdSetup(t)
	source, err := bdAttest(h, t04Key("source"), p)
	if err != nil {
		t.Fatal(err)
	}
	in := bdHome(c)
	in.Kind, in.PickupID, in.HomeAddress = p.Kind, source.ID, storefront.HomeAddress{}
	d, err := bdSet(h, t04Key("destination"), in)
	if err != nil {
		t.Fatal(err)
	}
	return h, c, p, in, d
}

func bdFacts(t *testing.T, h cqHarness) [4]int {
	t.Helper()
	var out [4]int
	for i, table := range []string{"storefront.destination_snapshots", "storefront.destination_heads", "storefront.destination_events", "buyer.command_results"} {
		out[i] = countRows(t, h.f.owner, `SELECT count(*) FROM `+table+` WHERE owner_id=$1`, h.cap.Scope.OwnerID)
	}
	return out
}

func TestPickupRevisionsRevocationAndScope(t *testing.T) {
	h, _, in := bdSetup(t)
	key := t04Key("attest")
	p, err := bdAttest(h, key, in)
	if err != nil {
		t.Fatal(err)
	}
	if p.Code != "017888" || p.Namespace != in.Namespace || p.Country != "TW" || p.VerificationKind != "MANUAL_ATTESTED" || p.ValidUntil.Sub(p.AttestedAt) != time.Hour {
		t.Fatalf("source projection: %+v", p)
	}
	if replay, e := bdAttest(h, key, in); e != nil || !reflect.DeepEqual(p, replay) {
		t.Fatalf("replay: %+v %v", replay, e)
	}
	if got, e := bdPickup(h, h.cap, p.ID, true); e != nil || !reflect.DeepEqual(p, got) {
		t.Fatalf("lock: %v", e)
	}
	for _, store := range []string{h.f.storeA2, h.f.storeB} {
		foreign := mustIssue(t, h.service, store)
		if _, e := bdPickup(h, foreign, p.ID, false); !errors.Is(e, command.ErrNotFound) {
			t.Fatalf("cross-store source: %v", e)
		}
	}
	for _, change := range []string{"brand", "namespace"} {
		other := in
		if change == "brand" {
			other.Kind = "cvs_711"
		} else {
			other.Namespace += ".other"
		}
		if out, e := bdAttest(h, t04Key("distinct-source"), other); e != nil || out.ID == p.ID || out.Version != 1 || out.Code != in.Code {
			t.Fatalf("namespace collision: %+v %v", out, e)
		}
	}
	for i := 0; i < 2; i++ {
		if e := bdRevoke(h, t04Key("revoke"), p.ID); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := bdPickup(h, h.cap, p.ID, true); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("revoked: %v", e)
	}
	if _, e := bdPickup(h, h.cap, p.ID, false); e != nil {
		t.Fatalf("historical revoked read: %v", e)
	}
	if _, e := bdAttest(h, key, in); e != nil {
		t.Fatal(e)
	}
	if _, e := bdPickup(h, h.cap, p.ID, true); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("replay re-enabled source: %v", e)
	}
	in.ExpectedVersion = 1
	next, err := bdAttest(h, t04Key("reattest"), in)
	if err != nil || next.ID == p.ID || next.Version != 2 {
		t.Fatalf("reattest: %+v %v", next, err)
	}
	if _, e := bdPickup(h, h.cap, next.ID, true); e != nil {
		t.Fatal(e)
	}
	if e := bdRevoke(h, t04Key("old-revoke"), p.ID); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("superseded revoke: %v", e)
	}
	mustExec(t, h.f.owner, `DELETE FROM identity.store_grants WHERE principal_id=$1 AND permission='integration:manage'`, h.f.principalA)
	if _, e := bdAttest(h, key, in); e == nil {
		t.Fatal("revoked authority replay")
	}
}

func TestBuyerDestinationHistoryReplayPrivacyAndSession(t *testing.T) {
	h, c, _ := bdSetup(t)
	in, key := bdHome(c), t04Key("home")
	d, err := bdSet(h, key, in)
	if err != nil || d.Version != 1 || d.Pickup != nil || d.ExpiresAt.Sub(d.SelectedAt) != 30*time.Minute {
		t.Fatalf("home: %+v %v", d, err)
	}
	for i := 0; i < 2; i++ {
		if got, e := bdSet(h, key, in); e != nil || !reflect.DeepEqual(d, got) {
			t.Fatalf("replay: %v", e)
		}
	}
	if _, e := bdValidate(h, d); e != nil {
		t.Fatal(e)
	}
	var receipt map[string]json.RawMessage
	var raw []byte
	if e := h.f.owner.QueryRow(context.Background(), `SELECT response FROM buyer.command_results WHERE idempotency_key=$1`, key).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(raw, &receipt); e != nil || len(receipt) != 2 || receipt["id"] == nil || receipt["version"] == nil || strings.Contains(string(raw), in.RecipientName) {
		t.Fatalf("receipt contains more than identity: %s %v", raw, e)
	}
	foreign := mustIssue(t, h.service, h.f.storeA1)
	if _, e := bdGet(h, foreign, d.ID); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("other buyer read: %v", e)
	}
	// Owner-only fixture creates a second valid capability for the same buyer;
	// this is not an exposed account-linking or token-issuance path.
	second := h.cap
	second.Token = base64.RawURLEncoding.EncodeToString(tokenHash(randomUUID()))
	second.Scope.SessionID = randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO buyer.capability_sessions(tenant_id,store_id,owner_id,id,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,clock_timestamp()+interval '1 hour')`, second.Scope.TenantID, second.Scope.StoreID, second.Scope.OwnerID, second.Scope.SessionID, tokenHash(second.Token))
	if got, e := bdGet(h, second, d.ID); e != nil || !reflect.DeepEqual(d, got) {
		t.Fatalf("same buyer new session: %v", e)
	}
	next := in
	next.ExpectedVersion, next.HomeAddress.Line1 = 1, "Changed synthetic address"
	if _, e := bdSet(h, key, next); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("changed payload replay: %v", e)
	}
	d2, err := bdSet(h, t04Key("new-home"), next)
	if err != nil || d2.Version != 2 || d2.ID == d.ID {
		t.Fatalf("second selection: %v", err)
	}
	if _, e := bdSet(h, t04Key("late-callback"), in); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("late callback overwrote head: %v", e)
	}
	if _, e := bdGet(h, h.cap, d.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := bdValidate(h, d); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("historical selection eligible: %v", e)
	}
	if _, e := bdValidate(h, d2); e != nil {
		t.Fatal(e)
	}
	if got := bdFacts(t, h); got[0] != 2 || got[1] != 1 || got[2] != 2 {
		t.Fatalf("duplicate records: %v", got)
	}
	if e := h.service.Revoke(context.Background(), h.cap.Token, h.cap.Scope.StoreID); e != nil {
		t.Fatal(e)
	}
	if _, e := bdSet(h, key, in); e == nil {
		t.Fatal("revoked capability replay")
	}
}

func TestBuyerDestinationValidationAndSQLBoundaries(t *testing.T) {
	h, _, _, in, d := bdCVS(t)
	if d.Pickup.Code != "017888" || d.Pickup.VerificationKind != "MANUAL_ATTESTED" || d.HomeAddress != (storefront.HomeAddress{}) || d.ExpiresAt.After(d.Pickup.ValidUntil) {
		t.Fatalf("CVS snapshot: %+v", d)
	}
	before := bdFacts(t, h)
	for name, mutate := range map[string]func(*storefront.DestinationInput){
		"phone": func(v *storefront.DestinationInput) { v.Phone = "invalid" }, "name": func(v *storefront.DestinationInput) { v.RecipientName = " padded" },
		"country": func(v *storefront.DestinationInput) { v.Country = "US" }, "mixed-address": func(v *storefront.DestinationInput) { v.HomeAddress.City = "invented" },
		"pickup-id": func(v *storefront.DestinationInput) { v.PickupID = "bad" }, "version": func(v *storefront.DestinationInput) { v.ExpectedVersion = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := in
			mutate(&bad)
			if _, e := bdSet(h, t04Key("invalid-destination"), bad); !errors.Is(e, command.ErrInvalid) {
				t.Fatalf("invalid: %v", e)
			}
		})
	}
	if after := bdFacts(t, h); after != before {
		t.Fatalf("invalid wrote: %v -> %v", before, after)
	}
	bad := in
	bad.ExpectedVersion = 1
	bad.Kind = "cvs_711"
	if _, e := bdSet(h, t04Key("wrong-kind"), bad); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("wrong brand accepted: %v", e)
	}
	bad = in
	bad.ExpectedVersion = 1
	bad.PickupID = randomUUID()
	if _, e := bdSet(h, t04Key("missing-source"), bad); !errors.Is(e, command.ErrNotFound) {
		t.Fatalf("missing source: %v", e)
	}
	for _, query := range []string{
		`SELECT evidence_ref FROM fulfillment.pickup_versions`,
		`SELECT principal_id FROM fulfillment.pickup_versions`,
		`UPDATE fulfillment.pickup_heads SET current_version=current_version`,
		`INSERT INTO fulfillment.pickup_versions SELECT * FROM fulfillment.pickup_versions`,
		`UPDATE storefront.destination_snapshots SET recipient_name=recipient_name`,
		`DELETE FROM storefront.destination_snapshots`,
	} {
		err := buyer.WithScope(context.Background(), h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error { _, e := tx.Exec(ctx, query); return e })
		daSQLState(t, err, "42501")
	}
	_, err := t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) {
		_, e := tx.Exec(context.Background(), `SELECT recipient_name FROM storefront.destination_snapshots`)
		return 0, e
	})
	daSQLState(t, err, "42501")
	for _, role := range []string{"commerce_worker", "commerce_buyer_issuer", "commerce_identity"} {
		tx, e := h.f.owner.Begin(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(context.Background(), `SET LOCAL ROLE `+role); e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(context.Background(), `SELECT recipient_name FROM storefront.destination_snapshots`)
		daSQLState(t, e, "42501")
		_ = tx.Rollback(context.Background())
	}
	_, err = cqBuyer(h.a.runtime, h.cap, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) (storefront.Destination, error) {
		s.OwnerID = randomUUID()
		return storefront.GetDestination(ctx, tx, s, d.ID)
	})
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("typed scope mismatch: %v", err)
	}
	_, err = h.f.owner.Exec(context.Background(), `UPDATE fulfillment.pickup_versions SET verification_kind='OFFICIAL_VERIFIED' WHERE id=$1`, d.Pickup.ID)
	daSQLState(t, err, "23514")
	_, err = h.f.owner.Exec(context.Background(), `UPDATE storefront.destination_snapshots SET kind='cvs_711' WHERE id=$1`, d.ID)
	daSQLState(t, err, "23503")
	_, err = h.f.owner.Exec(context.Background(), `UPDATE storefront.destination_heads SET destination_id=$2 WHERE cart_id=$1`, d.CartID, randomUUID())
	daSQLState(t, err, "23503")
}

func TestBuyerDestinationConcurrentReplayAndCAS(t *testing.T) {
	for _, sameKey := range []bool{true, false} {
		t.Run(map[bool]string{true: "replay", false: "CAS"}[sameKey], func(t *testing.T) {
			h, c, _ := bdSetup(t)
			in := bdHome(c)
			key := t04Key("destination-race")
			var wg sync.WaitGroup
			errs := make(chan error, 4)
			ids := make(chan string, 4)
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					k := key
					if !sameKey {
						k = t04Key("competing-selection")
					}
					d, e := bdSet(h, k, in)
					errs <- e
					ids <- d.ID
				}()
			}
			wg.Wait()
			close(errs)
			close(ids)
			pass, conflict := 0, 0
			for e := range errs {
				if e == nil {
					pass++
				} else if errors.Is(e, command.ErrConflict) {
					conflict++
				} else {
					t.Fatal(e)
				}
			}
			if (sameKey && pass != 4) || (!sameKey && (pass != 1 || conflict != 3)) {
				t.Fatalf("race pass=%d conflicts=%d", pass, conflict)
			}
			seen := map[string]bool{}
			for id := range ids {
				if id != "" {
					seen[id] = true
				}
			}
			if len(seen) != 1 {
				t.Fatalf("duplicate IDs: %d", len(seen))
			}
			facts := bdFacts(t, h)
			if facts[0] != 1 || facts[1] != 1 || facts[2] != 1 {
				t.Fatalf("duplicate durable facts: %v", facts)
			}
		})
	}
}

func TestBuyerDestinationRevalidationRejectsDrift(t *testing.T) {
	for _, change := range []string{"cart", "country", "kind", "revoked-source", "new-source", "expired", "future", "source-expiry", "source-future", "invalid-text"} {
		t.Run(change, func(t *testing.T) {
			h, c, p, _, d := bdCVS(t)
			switch change {
			case "cart":
				if _, e := h.cart(t04Key("edit-cart"), storefront.CartInput{ExpectedVersion: c.Version, Items: []storefront.Item{{SKUID: h.stock.skus[0].ID, Quantity: 1}}}); e != nil {
					t.Fatal(e)
				}
			case "country":
				d.Country = "US"
			case "kind":
				d.Kind = "home"
			case "revoked-source":
				if e := bdRevoke(h, t04Key("revoke"), d.Pickup.ID); e != nil {
					t.Fatal(e)
				}
			case "new-source":
				p.ExpectedVersion = 1
				if _, e := bdAttest(h, t04Key("new-source"), p); e != nil {
					t.Fatal(e)
				}
			case "expired":
				selected := d.Pickup.AttestedAt.Add(time.Microsecond)
				mustExec(t, h.f.owner, `UPDATE storefront.destination_snapshots SET selected_at=$2,expires_at=$3 WHERE id=$1`, d.ID, selected, selected.Add(time.Microsecond))
				bdAssertExpiredDestination(t, h, *d.Pickup, selected, selected.Add(time.Microsecond))
			case "future":
				mustExec(t, h.f.owner, `UPDATE storefront.destination_snapshots SET selected_at=clock_timestamp()+interval '1 minute',expires_at=clock_timestamp()+interval '2 minutes' WHERE id=$1`, d.ID)
			case "source-expiry":
				mustExec(t, h.f.owner, `UPDATE fulfillment.pickup_versions SET valid_until=clock_timestamp()+interval '5 minutes' WHERE id=$1`, d.Pickup.ID)
			case "source-future":
				mustExec(t, h.f.owner, `UPDATE fulfillment.pickup_versions SET attested_at=clock_timestamp()+interval '1 minute',valid_until=clock_timestamp()+interval '1 hour' WHERE id=$1`, d.Pickup.ID)
			case "invalid-text":
				mustExec(t, h.f.owner, `UPDATE storefront.destination_snapshots SET recipient_name=' padded' WHERE id=$1`, d.ID)
			}
			before := bdFacts(t, h)
			if _, e := bdValidate(h, d); !errors.Is(e, command.ErrConflict) {
				t.Fatalf("drift accepted: %v", e)
			}
			if after := bdFacts(t, h); before != after {
				t.Fatalf("revalidation wrote: %v -> %v", before, after)
			}
		})
	}
}

func TestBuyerDestinationAtomicRollback(t *testing.T) {
	h, c, p := bdSetup(t)
	in := bdHome(c)
	for _, failure := range []struct {
		table, condition string
		source           bool
	}{
		{"storefront.destination_events", "true", false}, {"buyer.command_results", "NEW.operation='destination.set'", false},
		{"ops.audit_events", "NEW.action='fulfillment.pickup.attest'", true}, {"ops.command_results", "NEW.operation='fulfillment.pickup.attest'", true},
	} {
		t.Run(failure.table, func(t *testing.T) {
			fn := "destination_fail_" + t04Tag()
			mustExec(t, h.f.owner, `CREATE FUNCTION storefront.`+fn+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF `+failure.condition+` THEN RAISE EXCEPTION 'synthetic rollback'; END IF; RETURN NEW; END $$`)
			mustExec(t, h.f.owner, `CREATE TRIGGER `+fn+` BEFORE INSERT ON `+failure.table+` FOR EACH ROW EXECUTE FUNCTION storefront.`+fn+`()`)
			defer mustExec(t, h.f.owner, `DROP FUNCTION storefront.`+fn+`() CASCADE`)
			before := bdFacts(t, h)
			key := t04Key("atomic-destination")
			if failure.source {
				if _, e := bdAttest(h, key, p); e == nil {
					t.Fatal("source fault ignored")
				}
			} else {
				if _, e := bdSet(h, key, in); e == nil {
					t.Fatal("destination fault ignored")
				}
			}
			if after := bdFacts(t, h); after != before {
				t.Fatalf("partial destination facts: %v -> %v", before, after)
			}
			for _, table := range []string{"fulfillment.pickup_versions", "fulfillment.pickup_heads"} {
				if countRows(t, h.f.owner, `SELECT count(*) FROM `+table+` WHERE namespace=$1`, p.Namespace) != 0 {
					t.Fatal("source escaped rollback")
				}
			}
			if countRows(t, h.f.owner, `SELECT count(*) FROM ops.command_results WHERE idempotency_key=$1`, key) != 0 {
				t.Fatal("source receipt escaped rollback")
			}
		})
	}
}

func TestBuyerDestinationLocksSurviveRevalidation(t *testing.T) {
	for _, target := range []string{"source", "destination", "cart"} {
		t.Run(target, func(t *testing.T) {
			h, _, _, _, d := bdCVS(t)
			release := make(chan struct{})
			ready, done := make(chan error, 1), make(chan error, 1)
			go func() {
				done <- buyer.WithScope(context.Background(), h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
					_, e := storefront.RevalidateDestination(ctx, tx, s, d.ID, d.CartVersion, d.Country, d.Kind)
					ready <- e
					if e != nil {
						return e
					}
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			}()
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			if e := waitError(t, ready); e != nil {
				t.Fatal(e)
			}
			writer, e := h.f.owner.Begin(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer writer.Rollback(context.Background())
			name := "destination-lock-" + t04Tag()
			if _, e = writer.Exec(context.Background(), `SELECT set_config('application_name',$1,true)`, name); e != nil {
				t.Fatal(e)
			}
			query, id := `UPDATE fulfillment.pickup_heads SET enabled=false WHERE pickup_id=$1`, d.Pickup.ID
			if target == "destination" {
				query, id = `UPDATE storefront.destination_heads SET current_version=current_version WHERE destination_id=$1`, d.ID
			}
			if target == "cart" {
				query, id = `UPDATE storefront.carts SET version=version+1 WHERE id=$1`, d.CartID
			}
			changed := make(chan error, 1)
			go func() { _, e := writer.Exec(context.Background(), query, id); changed <- e }()
			waitForDatabaseLock(t, h.f.owner, name)
			close(release)
			if e := waitError(t, done); e != nil {
				t.Fatal(e)
			}
			if e := waitError(t, changed); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestBuyerDestinationClockAfterActualWait(t *testing.T) {
	for _, operation := range []string{"set", "revalidate"} {
		t.Run(operation, func(t *testing.T) {
			h, _, _, in, d := bdCVS(t)
			var expiry time.Time
			if e := h.f.owner.QueryRow(context.Background(), `UPDATE fulfillment.pickup_versions SET valid_until=clock_timestamp()+interval '2 seconds' WHERE id=$1 RETURNING valid_until`, d.Pickup.ID).Scan(&expiry); e != nil {
				t.Fatal(e)
			}
			mustExec(t, h.f.owner, `UPDATE storefront.destination_snapshots SET expires_at=$2 WHERE id=$1`, d.ID, expiry)
			holder, e := h.f.owner.Begin(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer holder.Rollback(context.Background())
			if _, e = holder.Exec(context.Background(), `UPDATE storefront.destination_heads SET current_version=current_version WHERE destination_id=$1`, d.ID); e != nil {
				t.Fatal(e)
			}
			name := "destination-wait-" + t04Tag()
			pool, e := platform.OpenBuyerPool(context.Background(), withApplicationName(t, h.a.runtimeURL, name))
			if e != nil {
				t.Fatal(e)
			}
			defer pool.Close()
			before := bdFacts(t, h)
			done := make(chan error, 1)
			go func() {
				done <- buyer.WithScope(context.Background(), pool, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
					if _, e := tx.Exec(ctx, `SET LOCAL lock_timeout='3s'`); e != nil {
						return e
					}
					if operation == "set" {
						in.ExpectedVersion = 1
						_, e := storefront.SetDestination(ctx, tx, s, t04Key("wait-selection"), in)
						return e
					}
					_, e := storefront.RevalidateDestination(ctx, tx, s, d.ID, d.CartVersion, d.Country, d.Kind)
					return e
				})
			}()
			waitForDatabaseLock(t, h.f.owner, name)
			var stillValid bool
			if e = h.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()<$1`, expiry).Scan(&stillValid); e != nil || !stillValid {
				t.Fatalf("test missed expiry window: %v", e)
			}
			mustExec(t, h.f.owner, `SELECT pg_sleep(GREATEST(0,extract(epoch FROM $1::timestamptz-clock_timestamp()))+0.02)`, expiry)
			if e = holder.Commit(context.Background()); e != nil {
				t.Fatal(e)
			}
			if e = waitError(t, done); !errors.Is(e, command.ErrConflict) {
				t.Fatalf("post-wait expiry accepted: %v", e)
			}
			if after := bdFacts(t, h); after != before {
				t.Fatalf("expiry wrote partial selection: %v -> %v", before, after)
			}
		})
	}
}

func TestPickupCASReplayActorAndGUC(t *testing.T) {
	h, _, in := bdSetup(t)
	ctx := context.Background()
	key := t04Key("source-authority")
	if _, e := bdAttest(h, key, in); e != nil {
		t.Fatal(e)
	}
	if _, e := bdAttest(h, t04Key("source-stale"), in); !errors.Is(e, command.ErrConflict) {
		t.Fatalf("source stale CAS: %v", e)
	}
	other := seedPricingActor(t, h.f, h.f.tenantA, h.f.storeA1, false)
	var principal string
	if e := h.f.owner.QueryRow(ctx, `SELECT principal_id::text FROM identity.sessions WHERE token_hash=$1`, tokenHash(other)).Scan(&principal); e != nil {
		t.Fatal(e)
	}
	_, e := t04Scoped(ctx, h.f, other, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Pickup, error) {
		return fulfillment.AttestPickup(ctx, tx, s, other, key, in)
	})
	if !errors.Is(e, platform.ErrForbidden) {
		t.Fatalf("read-only replay: %v", e)
	}
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'integration:manage')`, h.f.tenantA, h.f.storeA1, principal)
	_, e = t04Scoped(ctx, h.f, other, h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Pickup, error) {
		return fulfillment.AttestPickup(ctx, tx, s, other, key, in)
	})
	if !errors.Is(e, command.ErrConflict) {
		t.Fatalf("cross-actor replay: %v", e)
	}
	for field, value := range map[string]string{"app.tenant_id": h.f.tenantB, "app.store_id": h.f.storeA2, "app.principal_id": principal} {
		_, e := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Pickup, error) {
			if _, e := tx.Exec(ctx, `SELECT set_config($1,$2,true)`, field, value); e != nil {
				return fulfillment.Pickup{}, e
			}
			return fulfillment.AttestPickup(ctx, tx, s, h.f.tokens["a"], key, in)
		})
		if !errors.Is(e, command.ErrInvalid) {
			t.Fatalf("GUC mismatch %s: %v", field, e)
		}
	}
	for _, sameKey := range []bool{true, false} {
		input := in
		input.Namespace += "." + t04Tag()
		raceKey := t04Key("source-race")
		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				k := raceKey
				if !sameKey {
					k = t04Key("source-competitor")
				}
				_, e := bdAttest(h, k, input)
				errs <- e
			}()
		}
		wg.Wait()
		close(errs)
		success, conflict := 0, 0
		for e := range errs {
			if e == nil {
				success++
			} else if errors.Is(e, command.ErrConflict) {
				conflict++
			} else {
				t.Fatal(e)
			}
		}
		if (sameKey && success != 4) || (!sameKey && (success != 1 || conflict != 3)) {
			t.Fatalf("source race: %d %d", success, conflict)
		}
		if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.pickup_versions WHERE namespace=$1`, input.Namespace); n != 1 {
			t.Fatalf("duplicate source revisions %d", n)
		}
	}
}

func TestBuyerDestinationDirectSQLIsNotAttestation(t *testing.T) {
	for _, change := range []string{"future", "overlong", "expired", "revoked-source", "beyond-source"} {
		t.Run(change, func(t *testing.T) {
			h, c, p := bdSetup(t)
			p.TTLSeconds = 120
			source, e := bdAttest(h, t04Key("direct-source"), p)
			if e != nil {
				t.Fatal(e)
			}
			if change == "revoked-source" {
				if e := bdRevoke(h, t04Key("revoke-direct-source"), source.ID); e != nil {
					t.Fatal(e)
				}
			}
			var now time.Time
			if e = h.f.owner.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&now); e != nil {
				t.Fatal(e)
			}
			selected, expires := now, now.Add(time.Minute)
			switch change {
			case "future":
				selected, expires = now.Add(10*time.Second), now.Add(time.Minute)
			case "overlong":
				expires = now.Add(31 * time.Minute)
			case "expired":
				selected = source.AttestedAt.Add(time.Microsecond)
				expires = selected.Add(time.Microsecond)
				bdAssertExpiredDestination(t, h, source, selected, expires)
			case "beyond-source":
				expires = now.Add(20 * time.Minute)
			}
			id := randomUUID()
			// Deliberately bypass Go with the actual restricted buyer role. A buyer
			// can write intent rows, but cannot turn them into trusted pickup facts.
			e = buyer.WithScope(context.Background(), h.a.runtime, h.cap.Token, h.cap.Scope.StoreID, func(ctx context.Context, tx pgx.Tx, s buyer.Scope) error {
				_, e := tx.Exec(ctx, `INSERT INTO storefront.destination_snapshots(tenant_id,store_id,owner_id,id,cart_id,cart_version,creator_session_id,version,kind,country,recipient_name,phone,pickup_id,selected_at,expires_at)
			 VALUES($1,$2,$3,$4,$5,$6,$7,1,'cvs_familymart','TW','Synthetic Buyer','+886900000001',$8,$9,$10)`, s.TenantID, s.StoreID, s.OwnerID, id, c.ID, c.Version, s.SessionID, source.ID, selected, expires)
				if e != nil {
					return e
				}
				_, e = tx.Exec(ctx, `INSERT INTO storefront.destination_heads(tenant_id,store_id,owner_id,cart_id,current_version,destination_id) VALUES($1,$2,$3,$4,1,$5)`, s.TenantID, s.StoreID, s.OwnerID, c.ID, id)
				return e
			})
			if change == "overlong" {
				daSQLState(t, e, "23514")
				return
			}
			if e != nil {
				t.Fatalf("fixture did not exercise buyer direct insert: %v", e)
			}
			d, e := bdGet(h, h.cap, id)
			if e != nil {
				t.Fatal(e)
			}
			before := bdFacts(t, h)
			if _, e = bdValidate(h, d); !errors.Is(e, command.ErrConflict) {
				t.Fatalf("buyer-forged %s accepted: %v", change, e)
			}
			if after := bdFacts(t, h); after != before {
				t.Fatal("forged-row validation wrote facts")
			}
		})
	}
}

// Isolate destination expiry: otherwise a selection that predates attestation
// would fail source chronology even if the destination expiry check were removed.
func bdAssertExpiredDestination(t *testing.T, h cqHarness, source fulfillment.Pickup, selected, expires time.Time) {
	t.Helper()
	if _, e := bdPickup(h, h.cap, source.ID, true); e != nil {
		t.Fatalf("expiry control source is not eligible: %v", e)
	}
	var isolated bool
	err := h.f.owner.QueryRow(context.Background(), `SELECT $1::timestamptz >= $2 AND $3::timestamptz > $1 AND $3 < clock_timestamp() AND $4::timestamptz > clock_timestamp()`, selected, source.AttestedAt, expires, source.ValidUntil).Scan(&isolated)
	if err != nil || !isolated {
		t.Fatalf("expiry test did not isolate the destination clock predicate: %v", err)
	}
}
