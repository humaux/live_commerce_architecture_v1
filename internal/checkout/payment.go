// payment.go defines shared payment DTOs and Stripe idempotency locking.
// Called by hosted checkout; the retired start API is fail-closed and performs no writes.
package checkout

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
)

const paymentOperation = "checkout.payment.start"

type PaymentStarter struct {
	pool    *pgxpool.Pool
	jobs    *river.Client[pgx.Tx]
	profile string
}

type PaymentInput struct {
	OrderID       string `json:"order_id"`
	MethodCode    string `json:"method_code"`
	MethodVersion int64  `json:"method_version"`
}

type PaymentResult struct {
	OrderID         string `json:"order_id"`
	AttemptID       string `json:"attempt_id"`
	OperationID     string `json:"operation_id"`
	JobID           int64  `json:"job_id"`
	Generation      int64  `json:"generation"`
	MerchantTradeNo string `json:"merchant_trade_no"`
	Currency        string `json:"currency"`
	AmountMinor     int64  `json:"amount_minor"`
	State           string `json:"state"`
}

type paymentQueryArgs struct {
	OperationID string `json:"operation_id"`
	Version     int    `json:"version"`
}

func (paymentQueryArgs) Kind() string { return "payment_query_v1" }

func NewPaymentStarter(ctx context.Context, checkoutPool *pgxpool.Pool, jobs *river.Client[pgx.Tx], profile string) (*PaymentStarter, error) {
	if !validPaymentProfile(profile) {
		return nil, command.ErrInvalid
	}
	s, err := New(ctx, checkoutPool, jobs)
	if err != nil {
		return nil, err
	}
	return &PaymentStarter{pool: s.pool, jobs: s.jobs, profile: profile}, nil
}

func validPaymentProfile(profile string) bool {
	return profile == "PROVIDER_MOCK" || profile == "SANDBOX" || profile == "LIVE"
}

// StartPayment is the retired non-hosted entry point. Only BeginHosted admits card payments.
// It refuses all inputs before accessing PG or River, including durable legacy replays.
func (s *PaymentStarter) StartPayment(ctx context.Context, token, storeID, key string, in PaymentInput) (PaymentResult, error) {
	return PaymentResult{}, command.ErrInvalid
}

func lockPaymentKey(ctx context.Context, tx pgx.Tx, scope buyer.Scope, key string) error {
	var timeout string
	if err := tx.QueryRow(ctx, `SHOW lock_timeout`).Scan(&timeout); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
		return err
	}
	if err := advisory(ctx, tx, "checkout.payment.start|"+scope.TenantID+"|"+scope.StoreID+"|"+scope.OwnerID+"|"+key); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT set_config('lock_timeout',$1,true)`, timeout)
	return err
}
