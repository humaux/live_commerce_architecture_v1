package live

import (
	"context"
	"errors"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestCombinedMediaWorkerRoutesOnlyOriginalQueues(t *testing.T) {
	w := &combinedMediaExecutionWorker{egress: &mediaExecutionWorker{}, input: &browserInputExecutionWorker{}}
	job := &river.Job[mediaOperationArgs]{JobRow: &rivertype.JobRow{ID: 41,
		Kind: (mediaOperationArgs{}).Kind()}, Args: mediaOperationArgs{
		OperationID: "11111111-1111-4111-8111-111111111111", Version: 1}}
	for _, queue := range []string{mediaMockQueue, mediaInputMockQueue} {
		job.Queue = queue
		if err := w.Work(context.Background(), job); !errors.Is(err, ErrMediaDatabase) {
			t.Fatalf("original queue %s did not reach its own executor: %v", queue, err)
		}
	}
	job.Queue = "other"
	if err := w.Work(context.Background(), job); !errors.Is(err, ErrMediaJob) {
		t.Fatalf("foreign queue entered an executor: %v", err)
	}
	job.Queue = mediaInputMockQueue
	job.Kind = "other_kind"
	if err := w.Work(context.Background(), job); !errors.Is(err, ErrMediaJob) {
		t.Fatalf("foreign kind entered input executor: %v", err)
	}
	if err := w.Work(context.Background(), nil); !errors.Is(err, ErrMediaJob) {
		t.Fatalf("nil job entered executor: %v", err)
	}
}
