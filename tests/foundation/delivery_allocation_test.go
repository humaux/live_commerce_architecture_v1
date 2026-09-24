package foundation_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/fulfillment"
	"livecommerce/internal/platform"
)

// This suite exercises ordinary merchant SQL, not a buyer checkout or carrier.
func daSetup(t *testing.T) (cqHarness, fulfillment.ServiceInput, fulfillment.AllocationInput) {
	t.Helper()
	h, service := dsSetup(t)
	if _, err := dsSet(h, t04Key("allocation-service"), service); err != nil {
		t.Fatal(err)
	}
	second := randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name) VALUES($1,$2,$3,$4)`, h.f.tenantA, h.f.storeA1, second, "allocation-"+t04Tag())
	return h, service, fulfillment.AllocationInput{MarketID: service.MarketID, Country: service.Country, Code: service.Code,
		ExpectedServiceVersion: 1, WarehouseIDs: []string{second, h.stock.warehouse.ID}}
}

func daSet(h cqHarness, key string, in fulfillment.AllocationInput) (fulfillment.Allocation, error) {
	return t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
		return fulfillment.SetAllocation(context.Background(), tx, s, h.f.tokens["a"], key, in)
	})
}

func daGet(h cqHarness, in fulfillment.AllocationInput) (fulfillment.Allocation, error) {
	return t04Scoped(context.Background(), h.f, h.f.tokens["a"], h.f.storeA1, "integration:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
		return fulfillment.GetAllocation(context.Background(), tx, s, h.f.tokens["a"], in.MarketID, in.Country, in.Code)
	})
}

func daSQLState(t *testing.T, err error, state string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != state {
		t.Fatalf("SQLSTATE want %s, got %v", state, err)
	}
}

func TestDeliveryAllocationRevisionsAndServiceIndependence(t *testing.T) {
	h, service, in := daSetup(t)
	if _, err := daGet(h, in); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
	key := t04Key("allocation-first")
	first, err := daSet(h, key, in)
	if err != nil || first.Version != 1 || first.ServiceVersion != 1 || !reflect.DeepEqual(first.WarehouseIDs, in.WarehouseIDs) {
		t.Fatalf("first: %+v %v", first, err)
	}
	got, err := daGet(h, in)
	if err != nil || !reflect.DeepEqual(first, got) {
		t.Fatalf("read: %+v %v", got, err)
	}
	service.ExpectedVersion, service.NameEN = 1, "Renamed independently"
	if _, err = dsSet(h, t04Key("allocation-rename"), service); err != nil {
		t.Fatal(err)
	}
	got, err = daGet(h, in)
	if err != nil || !reflect.DeepEqual(first, got) {
		t.Fatalf("service rename rewrote allocation: %+v %v", got, err)
	}
	if replay, err := daSet(h, key, in); err != nil || !reflect.DeepEqual(first, replay) {
		t.Fatalf("historical replay: %+v %v", replay, err)
	}
	in.ExpectedVersion = 1
	if _, err = daSet(h, t04Key("allocation-stale-service"), in); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale service: %v", err)
	}
	in.ExpectedServiceVersion = 2
	in.WarehouseIDs = []string{in.WarehouseIDs[1], in.WarehouseIDs[0]}
	second, err := daSet(h, t04Key("allocation-reorder"), in)
	if err != nil || second.Version != 2 || second.ServiceVersion != 2 || !reflect.DeepEqual(second.WarehouseIDs, in.WarehouseIDs) {
		t.Fatalf("reorder: %+v %v", second, err)
	}
	if _, err = daSet(h, t04Key("allocation-stale-cas"), in); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("stale config: %v", err)
	}
	if _, err = daSet(h, key, in); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("changed replay: %v", err)
	}
	in.ExpectedVersion, in.WarehouseIDs = 2, nil
	emptyKey := t04Key("allocation-clear")
	empty, err := daSet(h, emptyKey, in)
	if err != nil || empty.Version != 3 || empty.WarehouseIDs == nil || len(empty.WarehouseIDs) != 0 {
		t.Fatalf("clear: %+v %v", empty, err)
	}
	in.WarehouseIDs = []string{}
	if replay, err := daSet(h, emptyKey, in); err != nil || !reflect.DeepEqual(empty, replay) {
		t.Fatalf("nil vs empty replay: %+v %v", replay, err)
	}
	if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_versions WHERE market_id=$1`, in.MarketID); n != 3 {
		t.Fatalf("history %d", n)
	}
	if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_warehouses WHERE market_id=$1`, in.MarketID); n != 4 {
		t.Fatalf("immutable ordered children %d", n)
	}
}

func TestDeliveryAllocationValidationAndAuthorityBeforeReplay(t *testing.T) {
	h, _, in := daSetup(t)
	for name, mutate := range map[string]func(*fulfillment.AllocationInput){
		"market":           func(v *fulfillment.AllocationInput) { v.MarketID = "bad" },
		"country":          func(v *fulfillment.AllocationInput) { v.Country = "tw" },
		"code":             func(v *fulfillment.AllocationInput) { v.Code = "unsafe:code" },
		"negative-version": func(v *fulfillment.AllocationInput) { v.ExpectedVersion = -1 },
		"overflow":         func(v *fulfillment.AllocationInput) { v.ExpectedVersion = math.MaxInt64 },
		"service-version":  func(v *fulfillment.AllocationInput) { v.ExpectedServiceVersion = 0 },
		"warehouse-id":     func(v *fulfillment.AllocationInput) { v.WarehouseIDs = []string{"invalid"} },
		"duplicate": func(v *fulfillment.AllocationInput) {
			v.WarehouseIDs = []string{in.WarehouseIDs[0], in.WarehouseIDs[0]}
		},
		"bound": func(v *fulfillment.AllocationInput) {
			v.WarehouseIDs = make([]string, 17)
			for i := range v.WarehouseIDs {
				v.WarehouseIDs[i] = randomUUID()
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := in
			mutate(&bad)
			if _, err := daSet(h, t04Key("allocation-invalid"), bad); !errors.Is(err, command.ErrInvalid) {
				t.Fatalf("invalid: %v", err)
			}
		})
	}
	foreign := randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name) VALUES($1,$2,$3,$4)`, h.f.tenantA, h.f.storeA2, foreign, "foreign-"+t04Tag())
	bad := in
	bad.WarehouseIDs = []string{foreign}
	if _, err := daSet(h, t04Key("allocation-foreign"), bad); !errors.Is(err, command.ErrNotFound) {
		t.Fatalf("foreign warehouse: %v", err)
	}
	mustExec(t, h.f.owner, `UPDATE inventory.warehouses SET active=false WHERE id=$1`, in.WarehouseIDs[0])
	if _, err := daSet(h, t04Key("allocation-inactive"), in); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("inactive: %v", err)
	}
	mustExec(t, h.f.owner, `UPDATE inventory.warehouses SET active=true WHERE id=$1`, in.WarehouseIDs[0])
	key := t04Key("allocation-authority")
	if _, err := daSet(h, key, in); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	other := seedPricingActor(t, h.f, h.f.tenantA, h.f.storeA1, false)
	var otherPrincipal string
	if err := h.f.owner.QueryRow(ctx, `SELECT principal_id::text FROM identity.sessions WHERE token_hash=$1`, tokenHash(other)).Scan(&otherPrincipal); err != nil {
		t.Fatal(err)
	}
	_, err := t04Scoped(ctx, h.f, other, h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
		return fulfillment.SetAllocation(ctx, tx, s, other, key, in)
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("write without permission: %v", err)
	}
	mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'integration:manage')`, h.f.tenantA, h.f.storeA1, otherPrincipal)
	_, err = t04Scoped(ctx, h.f, other, h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
		return fulfillment.SetAllocation(ctx, tx, s, other, key, in)
	})
	if !errors.Is(err, command.ErrConflict) {
		t.Fatalf("cross actor replay: %v", err)
	}
	_, err = t04Scoped(ctx, h.f, other, h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
		return fulfillment.GetAllocation(ctx, tx, s, other, in.MarketID, in.Country, in.Code)
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("read without grant: %v", err)
	}
	for _, otherScope := range []struct{ token, tenant, store string }{
		{h.f.tokens["a2"], h.f.tenantA, h.f.storeA2},
		{h.f.tokens["b"], h.f.tenantB, h.f.storeB},
	} {
		mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission)
		 SELECT $1,$2,principal_id,p FROM identity.sessions CROSS JOIN unnest(ARRAY['integration:read','integration:manage']) p
		 WHERE token_hash=$3 ON CONFLICT DO NOTHING`, otherScope.tenant, otherScope.store, tokenHash(otherScope.token))
		_, e := t04Scoped(ctx, h.f, otherScope.token, otherScope.store, "integration:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
			return fulfillment.GetAllocation(ctx, tx, s, otherScope.token, in.MarketID, in.Country, in.Code)
		})
		if !errors.Is(e, command.ErrNotFound) {
			t.Fatalf("cross scope read: %v", e)
		}
		_, e = t04Scoped(ctx, h.f, otherScope.token, otherScope.store, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
			return fulfillment.SetAllocation(ctx, tx, s, otherScope.token, key, in)
		})
		if !errors.Is(e, command.ErrNotFound) {
			t.Fatalf("cross scope write: %v", e)
		}
	}
	_, err = t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) {
		_, e := tx.Exec(ctx, `INSERT INTO fulfillment.allocation_versions(tenant_id,store_id,market_id,country,code,version,service_version,warehouse_count,principal_id)
		 VALUES($1,$2,$3,$4,$5,99,1,0,$6)`, s.TenantID, s.StoreID, in.MarketID, in.Country, in.Code, otherPrincipal)
		return 0, e
	})
	daSQLState(t, err, "42501")
	for field, value := range map[string]string{"app.tenant_id": h.f.tenantB, "app.store_id": h.f.storeA2, "app.principal_id": otherPrincipal} {
		t.Run(field, func(t *testing.T) {
			_, e := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
				if _, e := tx.Exec(ctx, `SELECT set_config($1,$2,true)`, field, value); e != nil {
					return fulfillment.Allocation{}, e
				}
				return fulfillment.SetAllocation(ctx, tx, s, h.f.tokens["a"], key, in)
			})
			if !errors.Is(e, command.ErrInvalid) {
				t.Fatalf("GUC mismatch: %v", e)
			}
		})
	}
	mustExec(t, h.f.owner, `DELETE FROM identity.store_grants WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND permission='integration:manage'`, h.f.tenantA, h.f.storeA1, h.f.principalA)
	defer mustExec(t, h.f.owner, `INSERT INTO identity.store_grants(tenant_id,store_id,principal_id,permission) VALUES($1,$2,$3,'integration:manage') ON CONFLICT DO NOTHING`, h.f.tenantA, h.f.storeA1, h.f.principalA)
	_, err = t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "store:read", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
		return fulfillment.SetAllocation(ctx, tx, s, h.f.tokens["a"], key, in)
	})
	if !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("revoked replay: %v", err)
	}
	if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_versions WHERE market_id=$1`, in.MarketID); n != 1 {
		t.Fatalf("denied writes persisted: %d", n)
	}
}

func TestDeliveryAllocationConcurrentReplayAndCAS(t *testing.T) {
	h, _, in := daSetup(t)
	key := t04Key("allocation-concurrent")
	for _, replay := range []bool{true, false} {
		var wg sync.WaitGroup
		errs := make(chan error, 6)
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				next, k := in, key
				if !replay {
					next.ExpectedVersion = 1
					next.WarehouseIDs = next.WarehouseIDs[:1]
					k = fmt.Sprintf("allocation-cas:%s:%d", t04Tag(), i)
				}
				_, err := daSet(h, k, next)
				errs <- err
			}(i)
		}
		wg.Wait()
		close(errs)
		wins := 0
		for err := range errs {
			if err == nil {
				wins++
			} else if !errors.Is(err, command.ErrConflict) {
				t.Fatalf("concurrent: %v", err)
			}
		}
		want := 1
		if replay {
			want = 6
		}
		if wins != want {
			t.Fatalf("replay=%t wins=%d", replay, wins)
		}
	}
	if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_versions WHERE market_id=$1`, in.MarketID); n != 2 {
		t.Fatalf("versions=%d", n)
	}
}

func TestDeliveryAllocationSQLCompletenessAndRoleBoundary(t *testing.T) {
	h, _, in := daSetup(t)
	ctx := context.Background()
	if _, err := daSet(h, t04Key("allocation-sql"), in); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"commerce_worker", "commerce_buyer_issuer", "commerce_identity", "commerce_buyer_runtime"} {
		for _, query := range []string{`SELECT * FROM fulfillment.allocation_versions`, `SELECT * FROM inventory.lock_warehouse('` + in.WarehouseIDs[0] + `')`} {
			tx, err := h.f.owner.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, `SET LOCAL ROLE `+role); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, query)
			daSQLState(t, err, "42501")
			_ = tx.Rollback(ctx)
		}
	}
	for _, query := range []string{
		`UPDATE fulfillment.allocation_versions SET warehouse_count=0 WHERE market_id=$1`,
		`DELETE FROM fulfillment.allocation_versions WHERE market_id=$1`,
		`UPDATE fulfillment.allocation_warehouses SET position=3 WHERE market_id=$1`,
		`DELETE FROM fulfillment.allocation_warehouses WHERE market_id=$1`,
		`UPDATE inventory.warehouses SET active=false WHERE id=$1`,
	} {
		_, err := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) { _, e := tx.Exec(ctx, query, in.MarketID); return 0, e })
		daSQLState(t, err, "42501")
	}
	// INSERTs below run as an ordinary authenticated runtime, so deferred trigger
	// reads must obey RLS and remain usable without a SECURITY DEFINER owner bypass.
	insert := func(key string, count int, positions []int, warehouse string) error {
		_, err := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) {
			_, e := tx.Exec(ctx, `INSERT INTO fulfillment.allocation_versions(tenant_id,store_id,market_id,country,code,version,service_version,warehouse_count,principal_id) VALUES($1,$2,$3,$4,$5,99,1,$6,$7)`, s.TenantID, s.StoreID, in.MarketID, in.Country, in.Code, count, s.PrincipalID)
			if e != nil {
				return 0, e
			}
			for _, pos := range positions {
				if _, e = tx.Exec(ctx, `INSERT INTO fulfillment.allocation_warehouses(tenant_id,store_id,market_id,country,code,version,position,warehouse_id) VALUES($1,$2,$3,$4,$5,99,$6,$7)`, s.TenantID, s.StoreID, in.MarketID, in.Country, in.Code, pos, warehouse); e != nil {
					return 0, e
				}
			}
			_, e = tx.Exec(ctx, `INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES($1,$2,$3,$4)`, s.TenantID, s.StoreID, s.PrincipalID, key)
			return 0, e
		})
		return err
	}
	for _, tc := range []struct {
		name      string
		count     int
		positions []int
		state     string
	}{
		{"missing", 1, nil, "23514"}, {"gap", 1, []int{2}, "23514"}, {"zero-extra", 0, []int{1}, "23514"},
		{"duplicate-warehouse", 2, []int{1, 2}, "23505"}, {"position", 1, []int{17}, "23514"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := "allocation.test." + t04Tag()
			daSQLState(t, insert(key, tc.count, tc.positions, in.WarehouseIDs[0]), tc.state)
			if n := countRows(t, h.f.owner, `SELECT count(*) FROM ops.audit_events WHERE action=$1`, key); n != 0 {
				t.Fatal("deferred error persisted audit")
			}
		})
	}
	foreign := randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name) VALUES($1,$2,$3,$4)`, h.f.tenantA, h.f.storeA2, foreign, "foreign-fk-"+t04Tag())
	daSQLState(t, insert("allocation.test.foreign", 1, []int{1}, foreign), "23503")
	_, err := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) {
		var n int
		e := tx.QueryRow(ctx, `SELECT count(*) FROM inventory.lock_warehouse($1)`, foreign).Scan(&n)
		if e == nil && n != 0 {
			return n, errors.New("cross-store warehouse lock leak")
		}
		return n, e
	})
	if err != nil {
		t.Fatal(err)
	}
	// A later INSERT into a complete nonempty revision must be checked too.
	third := randomUUID()
	mustExec(t, h.f.owner, `INSERT INTO inventory.warehouses(tenant_id,store_id,id,name) VALUES($1,$2,$3,$4)`, h.f.tenantA, h.f.storeA1, third, "late-"+t04Tag())
	_, err = t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) {
		_, e := tx.Exec(ctx, `INSERT INTO fulfillment.allocation_warehouses(tenant_id,store_id,market_id,country,code,version,position,warehouse_id) VALUES($1,$2,$3,$4,$5,1,3,$6)`, s.TenantID, s.StoreID, in.MarketID, in.Country, in.Code, third)
		return 0, e
	})
	daSQLState(t, err, "23514")
	// And an empty committed revision must remain empty.
	in.ExpectedVersion, in.WarehouseIDs = 1, nil
	if _, err = daSet(h, t04Key("allocation-empty"), in); err != nil {
		t.Fatal(err)
	}
	_, err = t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (int, error) {
		_, e := tx.Exec(ctx, `INSERT INTO fulfillment.allocation_warehouses(tenant_id,store_id,market_id,country,code,version,position,warehouse_id) VALUES($1,$2,$3,$4,$5,2,1,$6)`, s.TenantID, s.StoreID, in.MarketID, in.Country, in.Code, h.stock.warehouse.ID)
		return 0, e
	})
	daSQLState(t, err, "23514")
	if n := countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_versions WHERE market_id=$1`, in.MarketID); n != 2 {
		t.Fatalf("invalid headers persisted %d", n)
	}
}

func TestDeliveryAllocationAtomicFailure(t *testing.T) {
	h, _, in := daSetup(t)
	first, err := daSet(h, t04Key("allocation-atomic"), in)
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []struct{ table, when string }{{"ops.audit_events", "NEW.action='fulfillment.allocation.set'"}, {"ops.command_results", "NEW.operation='fulfillment.allocation.set'"}} {
		t.Run(failure.table, func(t *testing.T) {
			fn := "allocation_fail_" + t04Tag()
			mustExec(t, h.f.owner, `CREATE FUNCTION fulfillment.`+fn+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF `+failure.when+` THEN RAISE EXCEPTION 'synthetic rollback'; END IF; RETURN NEW; END $$`)
			mustExec(t, h.f.owner, `CREATE TRIGGER `+fn+` BEFORE INSERT ON `+failure.table+` FOR EACH ROW EXECUTE FUNCTION fulfillment.`+fn+`()`)
			defer mustExec(t, h.f.owner, `DROP FUNCTION fulfillment.`+fn+`() CASCADE`)
			next := in
			next.ExpectedVersion = 1
			next.WarehouseIDs = nil
			key := t04Key("allocation-fail")
			before := countRows(t, h.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND action='fulfillment.allocation.set'`, h.f.tenantA)
			if _, err := daSet(h, key, next); err == nil {
				t.Fatal("injected failure ignored")
			}
			got, err := daGet(h, in)
			if err != nil || !reflect.DeepEqual(first, got) {
				t.Fatalf("head escaped rollback %+v %v", got, err)
			}
			if countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_versions WHERE market_id=$1`, in.MarketID) != 1 || countRows(t, h.f.owner, `SELECT count(*) FROM fulfillment.allocation_warehouses WHERE market_id=$1`, in.MarketID) != 2 || countRows(t, h.f.owner, `SELECT count(*) FROM ops.command_results WHERE idempotency_key=$1`, key) != 0 || countRows(t, h.f.owner, `SELECT count(*) FROM ops.audit_events WHERE tenant_id=$1 AND action='fulfillment.allocation.set'`, h.f.tenantA) != before {
				t.Fatal("partial atomic facts")
			}
		})
	}
}

func TestDeliveryAllocationRetainsLocksAndRechecksWarehouse(t *testing.T) {
	h, _, in := daSetup(t)
	ctx := context.Background()
	// SetAllocation returns inside caller transaction; its locks must survive until
	// the caller commits. Independent connection must not deactivate or change head.
	_, err := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
		out, e := fulfillment.SetAllocation(ctx, tx, s, h.f.tokens["a"], t04Key("allocation-lock"), in)
		if e != nil {
			return out, e
		}
		for _, locked := range []struct{ query, id string }{
			{`UPDATE inventory.warehouses SET active=false WHERE id=$1`, in.WarehouseIDs[0]},
			{`UPDATE fulfillment.service_heads SET current_version=current_version WHERE market_id=$1`, in.MarketID},
		} {
			other, e := h.f.owner.Begin(ctx)
			if e != nil {
				return out, e
			}
			if _, e = other.Exec(ctx, `SET LOCAL lock_timeout='100ms'`); e != nil {
				_ = other.Rollback(ctx)
				return out, e
			}
			_, e = other.Exec(ctx, locked.query, locked.id)
			_ = other.Rollback(ctx)
			daSQLState(t, e, "55P03")
		}
		return out, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A deactivation already waiting/committing is not a stale unlocked read.
	blocker, err := h.f.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err = blocker.Exec(ctx, `UPDATE inventory.warehouses SET active=false WHERE id=$1`, in.WarehouseIDs[0]); err != nil {
		t.Fatal(err)
	}
	name := "allocation-wait-" + t04Tag()
	result := make(chan error, 1)
	in.ExpectedVersion = 1
	go func() {
		_, e := t04Scoped(ctx, h.f, h.f.tokens["a"], h.f.storeA1, "integration:manage", func(tx pgx.Tx, s platform.Scope) (fulfillment.Allocation, error) {
			if _, e := tx.Exec(ctx, `SELECT set_config('application_name',$1,true)`, name); e != nil {
				return fulfillment.Allocation{}, e
			}
			return fulfillment.SetAllocation(ctx, tx, s, h.f.tokens["a"], t04Key("allocation-after-wait"), in)
		})
		result <- e
	}()
	waitForDatabaseLock(t, h.f.owner, name)
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = waitError(t, result); !errors.Is(err, command.ErrConflict) {
		t.Fatalf("deactivated after wait: %v", err)
	}
	if got, err := daGet(h, in); err != nil || got.Version != 1 {
		t.Fatalf("historical read must remain available %+v %v", got, err)
	}
}
