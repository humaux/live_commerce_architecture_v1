// Purpose: pin the executor's closed definitive-abort classification boundary.
// Depends on: native SQLSTATE/domain pairs, plan errors and the existing allocator.
// Used by: OA1 tests-first CI; absence of the helper is API evidence only.
// Invariants: order-attempt-v1 §3.2; no substring classification or generic SQLSTATE abort.
package orderattempt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
)

func TestOrderAttemptExecutorAbortClassification(t *testing.T) {
	for _, tc := range []struct {
		state, message, want string
	}{
		{"PT402", "insufficient_stock", "insufficient_inventory"},
		{"PT409", "attempt_expired", "EXPIRED"},
		{"PT410", "owner_erased", "ERASED"},
		{"PT400", "plan_invalid", "PLAN_INVALID"},
		{"PT400", "invalid_order_attempt_plan", "PLAN_INVALID"},
		{"PT404", "resource_unavailable", "resource_unavailable"},
		{"PT404", "store_unavailable", "resource_unavailable"},
		{"PT404", "sku_not_found", "resource_unavailable"},
		{"PT409", "owner_unavailable", "resource_unavailable"},
		{"PT409", "claim_unavailable", "resource_unavailable"},
		{"PT409", "allocation_unavailable", "resource_unavailable"},
		{"PT409", "warehouse_unavailable", "resource_unavailable"},
		{"PT409", "claim_quantity_exceeded", "resource_unavailable"},
		{"PT409", "bundle_reserved", "bundle_reserved"},
		{"PT409", "bundle_order_exists", "bundle_order_exists"},
		{"PT409", "offline_settings_unavailable", "payment_unavailable"},
		{"PT422", "delivery_service_unavailable", "invalid_destination"},
		{"PT422", "pay_at_pickup_unavailable", "payment_unavailable"},
		{"PT422", "cash_on_delivery_unavailable", "payment_unavailable"},
		{"PT422", "sku_inactive", "resource_unavailable"},
		{"PT422", "sku_changed", "resource_unavailable"},
		{"PT422", "sku_unavailable", "resource_unavailable"},
		{"PT422", "max_per_order_exceeded", "limit_exceeded"},
		{"PT429", "offline_order_limit", "limit_exceeded"},
	} {
		t.Run(tc.state+"/"+tc.message, func(t *testing.T) {
			native := &pgconn.PgError{Code: tc.state, Message: tc.message}
			for _, err := range []error{native, fmt.Errorf("execute: %w", native)} {
				if got := executionAbortCode(err); got != tc.want {
					t.Fatalf("closed native pair: got %q want %q", got, tc.want)
				}
			}
			for _, wrong := range []*pgconn.PgError{
				{Code: "XX000", Message: tc.message},
				{Code: tc.state, Message: "prefix_" + tc.message},
				{Code: tc.state, Message: tc.message + "_suffix"},
			} {
				if got := executionAbortCode(wrong); got != "" {
					t.Fatalf("wrong pair or substring must remain retryable: got %q", got)
				}
			}
		})
	}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"allocator insufficient", command.ErrInsufficient, "insufficient_inventory"},
		{"missing local plan key stays retryable", ErrPlanKeyUnavailable, ""},
		{"invalid authenticated plan", ErrPlanInvalid, "PLAN_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := executionAbortCode(fmt.Errorf("wrapped: %w", tc.err)); got != tc.want {
				t.Fatalf("typed domain error: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestOrderAttemptExecutorUnknownFailuresNeverProveAbsence(t *testing.T) {
	for _, tc := range []struct{ state, message string }{
		{"40001", "serialization"}, {"40P01", "deadlock"}, {"55P03", "lock timeout"},
		{"57014", "statement timeout"}, {"08006", "connection lost"}, {"57P01", "shutdown"},
		{"23503", "membership missing"}, {"23505", "duplicate"}, {"23514", "constraint"},
		{"42501", "ledger actor mismatch"}, {"PT401", "unauthorized"}, {"PT403", "forbidden"},
		{"PT400", "attempt_stage_invalid"}, {"PT400", "invalid_stock_plan"},
		{"PT409", "attempt_not_fenced"}, {"PT409", "attempt_cart_conflict"},
		{"PT409", "attempt_quote_conflict"}, {"PT409", "attempt_destination_conflict"},
		{"PT409", "reservation_lost"}, {"PT409", "origin_grant_unavailable"},
		{"PT409", "origin_already_consumed"}, {"PT409", "stock_plan_differs_from_quote"},
		{"PT409", "pay_entry_unavailable"}, {"PT422", "unknown_future_code"},
		{"PT422", "insufficient_stock"}, {"PT404", "owner_erased"},
		{"PT503", "unavailable"}, {"PT2RP", "shipment replay"},
	} {
		t.Run(tc.state+"/"+tc.message, func(t *testing.T) {
			if got := executionAbortCode(&pgconn.PgError{Code: tc.state, Message: tc.message}); got != "" {
				t.Fatalf("dependency/identity/transport fault cannot authorize abort: %q", got)
			}
		})
	}
	for _, err := range []error{nil, context.Canceled, context.DeadlineExceeded, io.EOF,
		command.ErrInvalid, ErrPlanSealFailed, ErrPayEntryKeyUnavailable, ErrPayEntryInvalid, ErrPayEntrySealFailed,
		errors.New("insufficient_stock"), errors.New(ErrPlanKeyUnavailable.Error())} {
		if got := executionAbortCode(err); got != "" {
			t.Fatalf("untyped or unrelated failure cannot authorize abort: %T -> %q", err, got)
		}
	}
}
