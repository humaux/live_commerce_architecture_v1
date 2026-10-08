//go:build browser

// Purpose: Bound only the click-sweep fixture leases to its test deadline; production expiry policy is unchanged.
// Depends on: context/time/testing; used by the isolated browser click-sweep identity and synthetic-domain setup.
// Used by: browser_click_sweep_test.go and a DB-free budget regression; no roles/permissions or negative expiry cases change.
package foundation_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

const csSweepBudget = 85 * time.Minute

func csFixtureLeaseUntil(ctx context.Context, now time.Time) (time.Time, error) {
	deadline, ok := ctx.Deadline()
	if !ok || !deadline.After(now) {
		return time.Time{}, fmt.Errorf("fixture needs a live bounded deadline")
	}
	// A finite one-minute grace covers clock/setup skew, not unbounded renewal.
	return deadline.Add(time.Minute), nil
}

func TestBrowserClickSweepFixtureLeaseBudget(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(csSweepBudget))
	defer cancel()
	until, err := csFixtureLeaseUntil(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if !until.After(now.Add(csSweepBudget)) || !until.After(now.Add(65*time.Minute)) {
		t.Fatal("fixture authority/domain expire inside the allowed sweep budget")
	}
	if until.After(now.Add(csSweepBudget + time.Minute)) {
		t.Fatal("fixture lease exceeds the bounded deadline grace")
	}
	if !until.Equal(now.Add(csSweepBudget + time.Minute)) {
		t.Fatal("fixture lease must end at the exact bounded deadline grace")
	}
	if _, err := csFixtureLeaseUntil(context.Background(), now); err == nil {
		t.Fatal("unbounded context was admitted")
	}
	past, stop := context.WithDeadline(context.Background(), now.Add(-time.Minute))
	defer stop()
	if _, err := csFixtureLeaseUntil(past, now); err == nil {
		t.Fatal("already expired deadline was admitted")
	}
}
