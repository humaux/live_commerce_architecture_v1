package foundation_test

// Purpose: end explicitly owned setup-pool lifetimes before browser execution.
// Depends on: caller lifetime audits and pgxpool's idempotent Close.
// Used by: browser harness boundaries; shared and future live borrowers stay protected.
// INV: validate every pointer before closing any; never infer ownership from a fixture.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func harnessSetupPoolPlan(protected, setupOnly []*pgxpool.Pool) ([]*pgxpool.Pool, error) {
	if len(protected) == 0 {
		return nil, errors.New("setup release requires protected live pools")
	}
	keep := make(map[*pgxpool.Pool]bool, len(protected))
	for _, pool := range protected {
		if pool == nil {
			return nil, errors.New("nil protected setup-release pool")
		}
		keep[pool] = true
	}
	var plan []*pgxpool.Pool
	seen := make(map[*pgxpool.Pool]bool, len(setupOnly))
	for _, pool := range setupOnly {
		if pool == nil || keep[pool] {
			return nil, errors.New("setup-only pool is nil or aliases a live pool")
		}
		if !seen[pool] {
			seen[pool] = true
			plan = append(plan, pool)
		}
	}
	return plan, nil
}

// The caller must return acquired handles and finish all setup borrowers first.
// A pool's instantaneous AcquiredConns count cannot establish that lifecycle fact.
func harnessReleaseSetupPools(t *testing.T, protected, setupOnly []*pgxpool.Pool) {
	t.Helper()
	plan, err := harnessSetupPoolPlan(protected, setupOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pool := range plan {
		pool.Close()
	}
	t.Logf("HARNESS_SETUP_POOLS released=%d", len(plan))
}

func TestHarnessPoolCapacityReleasePlan(t *testing.T) {
	// These are identity sentinels for the pure planner, never pools to Close.
	a, b, live := new(pgxpool.Pool), new(pgxpool.Pool), new(pgxpool.Pool)
	for _, tc := range []struct {
		name      string
		protected []*pgxpool.Pool
		owned     []*pgxpool.Pool
		want      []*pgxpool.Pool
		bad       bool
	}{
		{"empty-owned", []*pgxpool.Pool{live}, nil, nil, false},
		{"ordered-dedup", []*pgxpool.Pool{live}, []*pgxpool.Pool{a, a, b, a}, []*pgxpool.Pool{a, b}, false},
		{"duplicate-protected", []*pgxpool.Pool{live, live}, []*pgxpool.Pool{a}, []*pgxpool.Pool{a}, false},
		{"empty-protected", nil, []*pgxpool.Pool{a}, nil, true},
		{"nil-protected", []*pgxpool.Pool{live, nil}, []*pgxpool.Pool{a}, nil, true},
		{"late-nil", []*pgxpool.Pool{live}, []*pgxpool.Pool{a, nil}, nil, true},
		{"late-alias", []*pgxpool.Pool{live}, []*pgxpool.Pool{a, live}, nil, true},
		{"early-alias", []*pgxpool.Pool{live}, []*pgxpool.Pool{live, a}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for repeat := 0; repeat < 2; repeat++ {
				plan, err := harnessSetupPoolPlan(tc.protected, tc.owned)
				if (err != nil) != tc.bad || len(plan) != len(tc.want) || (tc.bad && plan != nil) {
					t.Fatalf("release plan: error=%v size=%d want=%d", err, len(plan), len(tc.want))
				}
				for i := range plan {
					if plan[i] != tc.want[i] {
						t.Fatal("release plan changed pointer order")
					}
				}
			}
		})
	}
}
