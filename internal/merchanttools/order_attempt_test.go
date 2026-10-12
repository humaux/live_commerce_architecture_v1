// Purpose: reject merchant pools that cannot hold the attempt lock and an inner pipeline transaction.
// Depends on: NewManualOrders and a lazy pgxpool with all connection attempts refused.
// Used by: Go merchanttools tests; no database or credentials required.
// Invariants: I02/I03; admitted attempts have capacity for their serialized inner work.
package merchanttools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/buyer"
	"livecommerce/internal/checkout"
	"livecommerce/internal/command"
)

func TestManualOrdersAttemptPoolCapacity(t *testing.T) {
	for _, size := range []int32{1, 2, 8} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			config, err := pgxpool.ParseConfig("postgres://fixture@127.0.0.1:1/fixture?sslmode=disable")
			if err != nil {
				t.Fatal(err)
			}
			config.MaxConns, config.MinConns, config.MinIdleConns = size, 0, 0
			var connections atomic.Int32
			config.BeforeConnect = func(context.Context, *pgx.ConnConfig) error {
				connections.Add(1)
				return errors.New("DB-free test refuses all connections")
			}
			pool, err := pgxpool.NewWithConfig(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			manual, err := NewManualOrders(pool, &buyer.Service{}, pool, &checkout.Service{}, []byte(strings.Repeat("x", 32)))
			if size < 2 {
				if manual != nil || !errors.Is(err, command.ErrInvalid) {
					t.Fatalf("single-connection pool accepted: service=%v error=%v", manual != nil, err)
				}
			} else if err != nil || manual == nil || cap(manual.attempts) != int(size)/2 {
				t.Fatalf("pool=%d service=%v error=%v", size, manual != nil, err)
			}
			if connections.Load() != 0 {
				t.Fatal("constructor attempted a database connection")
			}
		})
	}
}
