package foundation_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

func TestZZDebugTags(t *testing.T) {
	c := ctSetup(t)
	c.bundleCustomer()
	err := c.run(c.adminTok, c.f.storeA1, func(ctx context.Context, tx pgx.Tx, s platform.Scope) error {
		var raw []byte
		return tx.QueryRow(ctx, `SELECT identity.read_merchant_customers(sha256(convert_to($1,'UTF8')),$2::uuid,NULL,5,NULL,NULL,NULL,NULL)`, c.adminTok, c.f.storeA1).Scan(&raw)
	})
	t.Logf("DEBUG list err: %v", err)
}
