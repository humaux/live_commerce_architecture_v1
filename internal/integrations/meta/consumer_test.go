package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestConsumerWorkerInvalidJobsCancelBeforeStorage(t *testing.T) {
	w := &ConsumerWorker{}
	base := func() *river.Job[inboxJobArgs] {
		return &river.Job[inboxJobArgs]{JobRow: &rivertype.JobRow{ID: 1, Attempt: 1,
			Kind: (inboxJobArgs{}).Kind(), Queue: inboxQueue},
			Args: inboxJobArgs{EventID: payloadTestID, Version: 1}}
	}
	invalid := []struct {
		name string
		job  *river.Job[inboxJobArgs]
	}{
		{"nil", nil},
		{"nil row", &river.Job[inboxJobArgs]{}},
		{"zero id", func() *river.Job[inboxJobArgs] { j := base(); j.ID = 0; return j }()},
		{"zero attempt", func() *river.Job[inboxJobArgs] { j := base(); j.Attempt = 0; return j }()},
		{"version", func() *river.Job[inboxJobArgs] { j := base(); j.Args.Version = 2; return j }()},
		{"event id", func() *river.Job[inboxJobArgs] { j := base(); j.Args.EventID = "bad"; return j }()},
		{"kind", func() *river.Job[inboxJobArgs] { j := base(); j.Kind = "other"; return j }()},
		{"queue", func() *river.Job[inboxJobArgs] { j := base(); j.Queue = "default"; return j }()},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			err := w.Work(context.Background(), tc.job)
			var cancel *river.JobCancelError
			if !errors.As(err, &cancel) || !errors.Is(err, ErrConsumerJob) {
				t.Fatalf("invalid job did not cancel safely: %v", err)
			}
		})
	}
	if err := w.Work(context.Background(), base()); !errors.Is(err, ErrConsumerStorage) {
		t.Fatal("valid job without pool did not return retryable storage error")
	}
	if got := w.Timeout(nil); got != consumerTimeout {
		t.Fatal("worker timeout drifted")
	}
}

func TestConsumerErrorsAndFormattingAreSafe(t *testing.T) {
	if _, err := NewConsumerWorker(nil, nil, nil); !errors.Is(err, ErrConfig) {
		t.Fatal("invalid constructor accepted")
	}
	invalidSQL := &pgconn.PgError{Code: "22023", Message: "private SQL detail"}
	var cancel *river.JobCancelError
	if err := consumerDatabaseError(invalidSQL); !errors.As(err, &cancel) || !errors.Is(err, ErrConsumerJob) {
		t.Fatal("SQL argument failure did not cancel")
	}
	if err := consumerDatabaseError(&pgconn.PgError{Code: "23505", Message: "private SQL detail"}); !errors.Is(err, ErrConsumerStorage) || strings.Contains(err.Error(), "private") {
		t.Fatal("storage error leaked SQL detail or cancelled")
	}
	w := &ConsumerWorker{keys: &PayloadKeyring{activeID: "sensitive-key-name"}}
	for _, rendered := range []string{fmt.Sprint(w), fmt.Sprintf("%+v", w), fmt.Sprintf("%#v", w)} {
		if strings.Contains(rendered, "sensitive-key-name") || !strings.Contains(rendered, "redacted") {
			t.Fatal("worker formatting exposed key data")
		}
	}
	b, err := json.Marshal(w)
	if err != nil || strings.Contains(string(b), "sensitive-key-name") || !strings.Contains(string(b), "redacted") {
		t.Fatal("worker JSON exposed key data")
	}
}
