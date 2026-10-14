// Purpose: exact native erased-owner classification counterexamples for C1.
// Depends on: admissionFailure and PostgreSQL's typed domain error.
// Used by: parent CI before the mapping repair; native erasure race remains the PG oracle.
// Invariants: only proven PT422/ERASED becomes ERASED; other refusals stay opaque,
// and commit ambiguity is retained rather than interpreted as a terminal result.
package orderattempt

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestAdmissionFailureExactErasedOwner(t *testing.T) {
	for _, tc := range []struct {
		state   string
		message string
		want    string
	}{
		{"PT422", "ERASED", "ERASED"},
		{"PT422", "erased", "order_attempt_unavailable"},
		{"PT422", "owner_erased", "order_attempt_unavailable"},
		{"PT422", "ERASED ", "order_attempt_unavailable"},
		{"PT422", "sku_inactive", "order_attempt_unavailable"},
		{"PT422", "delivery_service_unavailable", "order_attempt_unavailable"},
		{"PT422", "payment_mode_unavailable", "order_attempt_unavailable"},
		{"PT422", "unknown_future_reason", "order_attempt_unavailable"},
		{"PT404", "ERASED", "not_found"},
		{"PT403", "ERASED", "forbidden"},
		{"PT409", "ERASED", "order_attempt_conflict"},
	} {
		t.Run(tc.state+"/"+tc.message, func(t *testing.T) {
			for _, committing := range []bool{false, true} {
				mapped := admissionFailure(&pgconn.PgError{Code: tc.state, Message: tc.message}, committing)
				var got *AdmissionError
				if !errors.As(mapped, &got) || got.Code() != tc.want || got.SQLState() != tc.state || got.OutcomeUnknown() != committing {
					t.Fatalf("exact native classification committing=%v: got=%v want=%s/%s", committing, mapped, tc.state, tc.want)
				}
			}
		})
	}
}
