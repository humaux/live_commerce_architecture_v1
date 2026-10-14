package foundation_test

// Purpose: enforce browser-fixture PostgreSQL headroom without changing pool limits.
// Depends on: pgx native activity/settings reads and caller-owned isolated fixtures.
// Used by: browser harnesses; observation errors and any sampled breach remain failures.
// INV: sampled client-backend high-water includes this observer, across the cluster;
// it is not a claim about unsampled instantaneous peaks or production capacity.
// Coverage begins at each caller's post-release browser/retained-service boundary;
// setup and fixture teardown are outside this watcher (the native release test is separate).

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const harnessCapacityMargin = 3

var errHarnessCapacityTimeout = errors.New("PG capacity observation timed out")

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
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return s, errHarnessCapacityTimeout
		}
		return s, errors.New("PG capacity observation failed") // never include connection configuration in evidence
	}
	return s, nil
}

func harnessReadCapacity(ctx context.Context, owner *pgxpool.Pool) (harnessCapacitySample, error) {
	return harnessReadCapacityFrom(ctx, owner)
}

type harnessCapacityState struct {
	peak, samples  int
	timeoutRetries int
	last           harnessCapacitySample
	err            error
}

// Only a timed-out observation gets one fresh-connection read. A valid breach,
// invalid reading, or other observation error goes directly to the sticky gate.
func (s *harnessCapacityState) sample(read func() (harnessCapacitySample, error), reconnect func() error) {
	v, err := read()
	if errors.Is(err, errHarnessCapacityTimeout) {
		s.timeoutRetries++
		if err = reconnect(); err == nil {
			v, err = read()
		}
	}
	s.observe(v, err)
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
	read := func() (harnessCapacitySample, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return harnessReadCapacityFrom(ctx, conn)
	}
	reconnect := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := conn.Close(ctx)
		cancel()
		if err != nil {
			return errors.New("PG capacity observer replacement cleanup failed")
		}
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		replacement, err := pgx.ConnectConfig(ctx, config.Copy())
		if err != nil {
			return errors.New("PG capacity observer replacement connection failed")
		}
		conn = replacement
		return nil
	}
	sample := func(context.Context) {
		state.sample(read, reconnect)
	}
	sample(context.Background())
	if state.err != nil {
		closeObserver()
		t.Fatal(state.err)
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	stop, done := harnessStartCapacitySampler(ticker.C, sample)
	t.Cleanup(func() {
		stop()
		<-done
		ticker.Stop()
		sample(context.Background())
		closeObserver()
		t.Logf("HARNESS_PG_CAPACITY sampled_peak=%d samples=%d limit=%d max=%d reserved=%d super=%d margin=%d observer_timeout_retries=%d",
			state.peak, state.samples, state.last.limit(), state.last.MaxConnections,
			state.last.ReservedConnections, state.last.SuperuserReservedConnections, harnessCapacityMargin, state.timeoutRetries)
		if state.err != nil {
			t.Error(state.err)
		}
	})
}

// harnessStartCapacitySampler shares the guard loop with controlled-tick tests.
func harnessStartCapacitySampler(ticks <-chan time.Time, sample func(context.Context)) (requestStop func(), done <-chan struct{}) {
	stop := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		for {
			select {
			case <-stop:
				return
			default:
			}
			select {
			case <-stop:
				return
			case <-ticks:
				sample(context.Background())
			}
		}
	}()
	return sync.OnceFunc(func() { close(stop) }), joined
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
	t.Run("timeout_reconnect_once", func(t *testing.T) {
		// MODEL_ONLY: injected reads verify recovery policy, not native PG timing.
		safe := harnessCapacitySample{ClientBackends: 2, MaxConnections: 60, SuperuserReservedConnections: 3}
		breach := safe
		breach.ClientBackends = 55
		for _, tc := range []struct {
			name       string
			first      harnessCapacitySample
			firstErr   error
			secondErr  error
			connectErr error
			reads      int
			reconnects int
			wantErr    bool
		}{
			{name: "timeout_recovers", firstErr: errHarnessCapacityTimeout, reads: 2, reconnects: 1},
			{name: "timeout_stays_error", firstErr: errHarnessCapacityTimeout, secondErr: errHarnessCapacityTimeout, reads: 2, reconnects: 1, wantErr: true},
			{name: "reconnect_fails", firstErr: errHarnessCapacityTimeout, connectErr: errors.New("replacement unavailable"), reads: 1, reconnects: 1, wantErr: true},
			{name: "saturation_never_retries", first: breach, reads: 1, wantErr: true},
			{name: "invalid_never_retries", reads: 1, wantErr: true},
			{name: "query_error_never_retries", firstErr: errors.New("query unavailable"), reads: 1, wantErr: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var state harnessCapacityState
				reads, reconnects := 0, 0
				state.sample(func() (harnessCapacitySample, error) {
					reads++
					if reads == 1 {
						return tc.first, tc.firstErr
					}
					return safe, tc.secondErr
				}, func() error {
					reconnects++
					return tc.connectErr
				})
				if reads != tc.reads || reconnects != tc.reconnects || state.timeoutRetries != tc.reconnects || (state.err != nil) != tc.wantErr {
					t.Fatalf("observation recovery: reads=%d reconnects=%d retries=%d failed=%t", reads, reconnects, state.timeoutRetries, state.err != nil)
				}
				if tc.wantErr && state.observe(safe, nil) == nil {
					t.Fatal("later recovery erased a guard failure")
				}
			})
		}
	})
	t.Run("stop_mid_query_final_sample", func(t *testing.T) {
		// MODEL_ONLY: cancellation closes this fake observer, modeling native
		// pgconn cancellation fallout without claiming a real PostgreSQL run.
		ticks := make(chan time.Time, 1)
		entered := make(chan struct{})
		release := make(chan struct{})
		var state harnessCapacityState
		var closed, queryCanceled bool
		first := true
		read := func(ctx context.Context) (harnessCapacitySample, error) {
			if first {
				first = false
				close(entered)
				<-release
				queryCanceled = ctx.Err() != nil
				if queryCanceled {
					closed = true
					return harnessCapacitySample{}, ctx.Err()
				}
			}
			if closed {
				return harnessCapacitySample{}, errors.New("fake observer closed after canceled query")
			}
			return harnessCapacitySample{ClientBackends: 2, MaxConnections: 60, SuperuserReservedConnections: 3}, nil
		}
		sample := func(parent context.Context) {
			// The barrier owns query completion; no wall-clock deadline can
			// create the cancellation this regression is testing.
			ctx, cancel := context.WithCancel(parent)
			defer cancel()
			v, err := read(ctx)
			if parent.Err() == nil {
				state.observe(v, err)
			}
		}
		requestStop, done := harnessStartCapacitySampler(ticks, sample)
		ticks <- time.Time{}
		<-entered
		requestStop() // synchronous stop while the query is gated
		requestStop() // stop is idempotent and never cancels an observation
		close(release)
		<-done
		if queryCanceled {
			t.Error("stopping sampler canceled the in-flight query context")
		}
		if state.samples != 1 || state.peak != 2 || state.err != nil {
			t.Errorf("in-flight observation was not retained: samples=%d peak=%d err=%v", state.samples, state.peak, state.err)
		}
		sample(context.Background()) // final sample must reuse the same observer
		if state.samples != 2 || state.err != nil || closed {
			t.Errorf("final sample failed after sampler stop: samples=%d closed=%t err=%v", state.samples, closed, state.err)
		}
	})
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
