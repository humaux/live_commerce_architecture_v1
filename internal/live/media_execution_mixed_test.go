// Responsibility: exact mixed dispatcher and constructor boundary regressions.
// Dependencies: existing worker guards. No DB or provider execution evidence.
package live

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"livecommerce/internal/integrations/livekit"
	"testing"
	"time"
)

func TestMediaProcessExactQueueDispatch(t *testing.T) {
	job := func(q string) *river.Job[mediaOperationArgs] {
		return &river.Job[mediaOperationArgs]{JobRow: &rivertype.JobRow{ID: 1, Kind: (mediaOperationArgs{}).Kind(), Queue: q}, Args: mediaOperationArgs{Version: 1, OperationID: "01234567-89ab-4def-8123-0123456789ab"}}
	}
	for _, queue := range []string{mediaMockQueue, mediaInputMockQueue} {
		w := &mediaQueueDispatcher{}
		if queue == mediaMockQueue {
			w.egress = &mediaExecutionWorker{}
		} else {
			w.input = &browserInputExecutionWorker{}
		}
		if err := w.Work(context.Background(), job(queue)); !errors.Is(err, ErrMediaDatabase) {
			t.Fatalf("exact queue did not reach its worker guard: %v", err)
		}
		other := mediaMockQueue
		if queue == other {
			other = mediaInputMockQueue
		}
		if err := w.Work(context.Background(), job(other)); !errors.Is(err, ErrMediaJob) {
			t.Fatalf("cross route: %v", err)
		}
		if w.Timeout(nil) != 30*time.Second {
			t.Fatal("timeout changed")
		}
	}
	w := &mediaQueueDispatcher{egress: &mediaExecutionWorker{}, input: &browserInputExecutionWorker{}}
	bad := job(mediaMockQueue)
	bad.Kind = "other"
	wrongVersion := job(mediaInputMockQueue)
	wrongVersion.Args.Version = 2
	for _, j := range []*river.Job[mediaOperationArgs]{nil, {}, job("media_mock_v1_extra"), job(""), bad, wrongVersion} {
		if err := w.Work(context.Background(), j); !errors.Is(err, ErrMediaJob) {
			t.Fatalf("malformed job admitted: %v", err)
		}
	}
}
func TestMediaProcessMapIdentityBeforeDatabase(t *testing.T) {
	rt := &recoveryTransport{}
	_, project, input := recoveryFixture(t, rt)
	keys, err := livekit.NewMaterialKeyring("k1", map[string][]byte{"k1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*MediaProject){func(p *MediaProject) { p.ProjectID = "other" }, func(p *MediaProject) { p.CredentialVersion++ }, func(p *MediaProject) { p.Config.Endpoint = "https://other.livekit.cloud" }} {
		p := project
		mutate(&p)
		c, err := NewMediaClientWithBrowserInput(context.Background(), new(pgxpool.Pool), new(pgxpool.Pool), keys, []MediaProject{p}, input, 1)
		if c != nil || !errors.Is(err, ErrMediaConfig) {
			t.Fatalf("mismatch reached database: %v", err)
		}
	}
	if len(rt.calls) != 0 {
		t.Fatal("mapping performed I/O")
	}
}
