//go:build browser

// Purpose: retain a deterministic native connection demand across the setup/runtime boundary.
// Depends on: isolated PG, two psSetupItemsOn families and the shared capacity observer.
// Used by: TestBrowserTrackingBackfill; never changes cluster limits or production pool policy.
// Invariant: release every acquired handle before observing idle setup-pool lifetimes.
package foundation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/puddle/v2"
)

func harnessPoolHeadroomRegression(t *testing.T) {
	t.Helper()
	f := pwIsolatedFixture(t)
	first := psSetupItemsOn(t, f, 1)
	second := psSetupItemsOn(t, f, 1)
	// Only these twelve pointers belong to the two setup families. In particular,
	// copied f.owner/f.runtime aliases are retained for observation and teardown.
	pools := []*pgxpool.Pool{
		first.a.runtime, first.a.issuer, first.a.identity, first.pool, first.worker, first.expiry,
		second.a.runtime, second.a.issuer, second.a.identity, second.pool, second.worker, second.expiry,
	}
	seen := make(map[*pgxpool.Pool]bool, len(pools))
	for _, pool := range pools {
		if pool == nil || pool == f.owner || pool == f.runtime || seen[pool] {
			t.Fatal("headroom workload requires twelve distinct family-owned pools")
		}
		seen[pool] = true
		if max := pool.Config().MaxConns; max < 1 || max > 8 {
			t.Fatal("headroom workload requires native family pool bounds from 1 through 8")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	read := func() harnessCapacitySample {
		sample, err := harnessReadCapacity(ctx, f.owner)
		if err != nil {
			t.Fatal("headroom workload cannot observe native cluster capacity")
		}
		return sample
	}
	initial := read()
	usable := initial.MaxConnections - initial.ReservedConnections - initial.SuperuserReservedConnections
	// This is a real native-backend demand, one above the guard's fixed margin3.
	// MaxConns sums do not establish either the initial or final backend count.
	target := usable - 3 + 1
	if target <= initial.ClientBackends || target >= usable {
		t.Fatal("headroom workload requires positive bounded demand below usable capacity")
	}
	held := make([]*pgxpool.Conn, 0, len(pools)*8)
	release := func() {
		for _, conn := range held {
			conn.Release()
		}
		held = nil
	}
	defer release() // also runs on Fatal before fixture pool cleanup
	sample := initial
	for round := 0; round < 8 && sample.ClientBackends < target; round++ {
		for _, pool := range pools {
			if round >= int(pool.Config().MaxConns) {
				continue
			}
			conn, err := pool.Acquire(ctx)
			if err != nil {
				t.Fatal("headroom workload could not acquire bounded family demand")
			}
			held = append(held, conn)
			sample = read()
			if sample.ClientBackends >= target {
				break
			}
		}
	}
	if sample.ClientBackends != target {
		t.Fatalf("headroom workload backend barrier: observed=%d target=%d", sample.ClientBackends, target)
	}
	t.Logf("headroom workload barrier: client_backends=%d usable=%d margin=3 owned_pools=%d held=%d", sample.ClientBackends, usable, len(pools), len(held))
	release()
	// Phase boundary: acquisition handles are returned, but native idle backends
	// remain owned by the setup pools until their lifetimes are correctly ended.
	protected := []*pgxpool.Pool{f.owner, f.runtime}
	harnessReleaseSetupPools(t, protected, pools)
	harnessReleaseSetupPools(t, protected, pools) // real pools: Close remains idempotent
	for i, pool := range pools {
		conn, err := pool.Acquire(ctx)
		if conn != nil {
			conn.Release()
		}
		if !errors.Is(err, puddle.ErrClosedPool) {
			t.Fatalf("setup pool %d did not report closed", i)
		}
	}
	for _, pool := range protected {
		var one int
		if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
			t.Fatal("setup release closed a protected live pool")
		}
	}
	after := read()
	t.Logf("headroom after release: client_backends=%d limit=%d", after.ClientBackends, after.limit())
	harnessWatchCapacity(t, f.owner)
}
