package live

import (
	"context"
	"errors"
	"testing"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

func TestMediaPlannerRejectsInvalidBoundary(t *testing.T) {
	if _, err := NewMediaPlanner(nil); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("nil jobs: %v", err)
	}
	var planner *MediaPlanner
	if _, err := planner.PlanStart(context.Background(), nil, platform.Scope{}, "", "", MediaStartInput{}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("nil planner: %v", err)
	}
	if _, err := (&MediaPlanner{}).PlanStart(nil, nil, platform.Scope{}, "", "", MediaStartInput{}); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("nil context: %v", err)
	}
}

func TestMediaOperationArgsFixedKind(t *testing.T) {
	if got := (mediaOperationArgs{}).Kind(); got != "live_media_operation_v1" {
		t.Fatalf("kind: %q", got)
	}
}
