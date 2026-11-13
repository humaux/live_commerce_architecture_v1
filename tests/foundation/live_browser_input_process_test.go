// Responsibility: combined-client role/readiness negatives against existing PG.
// This is constructor coverage, not native dualqueue or real-SFU acceptance.
package foundation_test

import (
	"context"
	"testing"

	"livecommerce/internal/live"
)

func TestLiveBrowserInputProcessReadiness(t *testing.T) {
	f := brwWireStarted(t)
	h := f.h
	ctx := context.Background()
	construct := func() error {
		_, err := live.NewMediaClientWithBrowserInput(ctx, h.worker, h.executor, f.keys, []live.MediaProject{f.egress}, h.runtime, 1)
		return err
	}
	if err := construct(); err != nil {
		t.Fatalf("clean combined constructor: %v", err)
	}
	for _, tc := range []struct{ name, grant, revoke string }{
		{"input", `GRANT EXECUTE ON FUNCTION live.browser_input_next_action(uuid) TO PUBLIC`, `REVOKE EXECUTE ON FUNCTION live.browser_input_next_action(uuid) FROM PUBLIC`},
		{"legacy", `GRANT EXECUTE ON FUNCTION live.reserve_media_start(uuid,bigint,bytea) TO commerce_runtime`, `REVOKE EXECUTE ON FUNCTION live.reserve_media_start(uuid,bigint,bytea) FROM commerce_runtime`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExec(t, h.lp.f.owner, tc.grant)
			t.Cleanup(func() { mustExec(t, h.lp.f.owner, tc.revoke) })
			if err := construct(); err == nil {
				t.Fatal("combined constructor accepted unsafe readiness")
			}
			mustExec(t, h.lp.f.owner, tc.revoke)
			if err := construct(); err != nil {
				t.Fatalf("restored constructor: %v", err)
			}
		})
	}
	if _, err := live.NewMediaClientWithBrowserInput(ctx, h.executor, h.worker, f.keys, []live.MediaProject{f.egress}, h.runtime, 1); err == nil {
		t.Fatal("swapped native roles accepted")
	}
	if f.inputs.Load() != 0 || f.starts.Load() != 0 {
		t.Fatal("constructor performed provider I/O")
	}
}
