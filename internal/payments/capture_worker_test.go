package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
)

func TestPaymentReconcileJobShapeAndValidation(t *testing.T) {
	valid := paymentReconcileArgs{OperationID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		ReportHash: strings.Repeat("a", 64), Version: 1}
	if valid.Kind() != "payment_reconcile_v1" || !validCaptureArgs(valid) {
		t.Fatal("frozen private job rejected")
	}
	encoded, err := json.Marshal(valid)
	if err != nil || string(encoded) != `{"operation_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","report_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","version":1}` {
		t.Fatalf("private job shape changed: %s %v", encoded, err)
	}
	for _, tc := range []paymentReconcileArgs{
		{OperationID: "bad", ReportHash: valid.ReportHash, Version: 1},
		{OperationID: valid.OperationID, ReportHash: strings.Repeat("A", 64), Version: 1},
		{OperationID: valid.OperationID, ReportHash: strings.Repeat("a", 63), Version: 1},
		{OperationID: valid.OperationID, ReportHash: strings.Repeat("g", 64), Version: 1},
		{OperationID: valid.OperationID, ReportHash: valid.ReportHash, Version: 2},
	} {
		if validCaptureArgs(tc) {
			t.Fatalf("forged job accepted: %+v", tc)
		}
	}
	worker := &CaptureWorker{}
	if worker.Timeout(nil) != 5*time.Second {
		t.Fatal("capture worker deadline changed")
	}
	bad := valid
	bad.ReportHash = strings.Repeat("A", 64)
	if err := worker.Work(context.Background(), &river.Job[paymentReconcileArgs]{Args: bad}); !errors.Is(err, errCaptureJob) {
		t.Fatalf("bad job was not cancelled with fixed error: %v", err)
	}
}

func TestCaptureWorkerConstructorRejectsInvalidAuthority(t *testing.T) {
	if _, err := NewCaptureWorker(nil, nil); !errors.Is(err, errCaptureJob) {
		t.Fatalf("nil context accepted: %v", err)
	}
	if _, err := NewCaptureWorker(context.Background(), nil); !errors.Is(err, errCaptureDatabase) {
		t.Fatalf("nil worker pool accepted: %v", err)
	}
}

func TestCaptureApplyErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		cancel bool
	}{
		{"invalid input", &pgconn.PgError{Code: "22023", Message: "private-input"}, true},
		{"missing report or provenance", fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "PT409", Message: "private-provenance"}), true},
		{"deadlock", &pgconn.PgError{Code: "40P01", Message: "private-deadlock"}, false},
		{"timeout", &pgconn.PgError{Code: "57014", Message: "private-timeout"}, false},
		{"lock wait", &pgconn.PgError{Code: "55P03", Message: "private-lock"}, false},
		{"transaction fault", errors.New("private-transaction"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := captureApplyError(tc.err)
			if tc.cancel != errors.Is(got, errCaptureJob) || !tc.cancel && !errors.Is(got, errCaptureDatabase) {
				t.Fatalf("wrong classification: %v", got)
			}
			if strings.Contains(got.Error(), "private-") {
				t.Fatalf("database detail leaked: %v", got)
			}
		})
	}
}
