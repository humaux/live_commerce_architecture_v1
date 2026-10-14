package foundation_test

// Purpose: enforce browser-fixture PostgreSQL headroom without changing pool limits.
// Depends on: pgx native activity/settings reads and caller-owned isolated fixtures.
// Used by: browser harnesses; observation errors and any sampled breach remain failures.
// INV: sampled client-backend high-water includes this observer, across the cluster;
// it is not a claim about unsampled instantaneous peaks or production capacity.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const harnessCapacityMargin = 3

type harnessCapacitySample struct {
	ClientBackends, MaxConnections, ReservedConnections, SuperuserReservedConnections int
}

func (s harnessCapacitySample) limit() int {
	return s.MaxConnections - s.ReservedConnections - s.SuperuserReservedConnections - harnessCapacityMargin
}

type harnessCapacityQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func harnessReadCapacityFrom(ctx context.Context, q harnessCapacityQuerier) (harnessCapacitySample, error) {
	var s harnessCapacitySample
	err := q.QueryRow(ctx, `SELECT count(*) FILTER (WHERE backend_type='client backend'),
	 current_setting('max_connections')::int,current_setting('reserved_connections')::int,
	 current_setting('superuser_reserved_connections')::int FROM pg_stat_activity`).Scan(
		&s.ClientBackends, &s.MaxConnections, &s.ReservedConnections, &s.SuperuserReservedConnections)
	if err != nil {
		return s, errors.New("PG capacity observation failed") // never include connection configuration in evidence
	}
	return s, nil
}

func harnessReadCapacity(ctx context.Context, owner *pgxpool.Pool) (harnessCapacitySample, error) {
	return harnessReadCapacityFrom(ctx, owner)
}

type harnessCapacityState struct {
	peak, samples int
	last          harnessCapacitySample
	err           error
}

func (s *harnessCapacityState) observe(v harnessCapacitySample, err error) error {
	if err == nil {
		if v.ClientBackends < 1 || v.MaxConnections < 1 || v.ReservedConnections < 0 ||
			v.SuperuserReservedConnections < 0 || v.limit() < 1 || v.ClientBackends > v.MaxConnections {
			err = errors.New("invalid PG capacity observation")
		} else {
			s.samples++
			s.last = v
			if v.ClientBackends > s.peak {
				s.peak = v.ClientBackends
			}
			if v.ClientBackends > v.limit() {
				err = fmt.Errorf("PG capacity headroom exceeded: observed=%d limit=%d max=%d reserved=%d super=%d margin=%d",
					v.ClientBackends, v.limit(), v.MaxConnections, v.ReservedConnections, v.SuperuserReservedConnections, harnessCapacityMargin)
			}
		}
	}
	if err != nil && s.err == nil {
		s.err = err
	}
	return s.err
}

// harnessWatchCapacity owns one observer connection, not another pool. It never
// runs on the HTTP response path. Cleanup joins observation before closing the
// connection and before the fixture's previously registered teardown callbacks.
func harnessWatchCapacity(t *testing.T, owner *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	config := owner.Config().ConnConfig.Copy()
	config.RuntimeParams["application_name"] = "lc-browser-capacity"
	conn, err := pgx.ConnectConfig(ctx, config)
	cancel()
	if err != nil {
		t.Fatal("PG capacity observer connection failed")
	}
	closeObserver := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := conn.Close(ctx); err != nil {
			t.Error("PG capacity observer cleanup failed")
		}
	}
	var state harnessCapacityState
	sample := func(parent context.Context) {
		ctx, cancel := context.WithTimeout(parent, time.Second)
		defer cancel()
		v, err := harnessReadCapacityFrom(ctx, conn)
		if parent.Err() == nil {
			state.observe(v, err)
		}
	}
	sample(context.Background())
	if state.err != nil {
		closeObserver()
		t.Fatal(state.err)
	}
	watch, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-watch.Done():
				return
			case <-ticker.C:
				sample(watch)
			}
		}
	}()
	t.Cleanup(func() {
		stop()
		<-done
		sample(context.Background())
		closeObserver()
		t.Logf("HARNESS_PG_CAPACITY sampled_peak=%d samples=%d limit=%d max=%d reserved=%d super=%d margin=%d",
			state.peak, state.samples, state.last.limit(), state.last.MaxConnections,
			state.last.ReservedConnections, state.last.SuperuserReservedConnections, harnessCapacityMargin)
		if state.err != nil {
			t.Error(state.err)
		}
	})
}

func TestHarnessPoolCapacityBoundary(t *testing.T) {
	for _, n := range []int{1, 54, 55} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			var state harnessCapacityState
			err := state.observe(harnessCapacitySample{ClientBackends: n, MaxConnections: 60, SuperuserReservedConnections: 3}, nil)
			if (err != nil) != (n > 54) || state.peak != n || state.samples != 1 {
				t.Fatalf("capacity boundary: err=%v peak=%d samples=%d", err, state.peak, state.samples)
			}
		})
	}
}

func TestHarnessPoolCapacityObservationFailureIsSticky(t *testing.T) {
	for _, first := range []struct {
		name string
		v    harnessCapacitySample
		err  error
	}{
		{"query", harnessCapacitySample{}, errors.New("observation unavailable")},
		{"invalid", harnessCapacitySample{ClientBackends: 0, MaxConnections: 60}, nil},
		{"reserved", harnessCapacitySample{ClientBackends: 1, MaxConnections: 60, ReservedConnections: 60}, nil},
		{"breach", harnessCapacitySample{ClientBackends: 55, MaxConnections: 60, SuperuserReservedConnections: 3}, nil},
	} {
		t.Run(first.name, func(t *testing.T) {
			var state harnessCapacityState
			if state.observe(first.v, first.err) == nil {
				t.Fatal("bad observation was accepted")
			}
			if state.observe(harnessCapacitySample{ClientBackends: 2, MaxConnections: 60, SuperuserReservedConnections: 3}, nil) == nil {
				t.Fatal("later recovery erased a guard failure")
			}
		})
	}
}
