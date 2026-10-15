// hosted_test.go proves retired credential creation fails before any SQL or provider material can be used.
package accounts

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
	"livecommerce/internal/platform"
)

// The embedded interface deliberately panics on every method. A retired write must not even authorize via SQL.
type removedAccountTx struct{ pgx.Tx }

func removedAccountScope() platform.Scope {
	return platform.Scope{TenantID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", StoreID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", PrincipalID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Revision: 1}
}

func TestRemovedAccountCreateNeverReadsOrWrites(t *testing.T) {
	s, err := New(testKeyring(t), &core.Service{})
	if err != nil {
		t.Fatal(err)
	}
	for _, environment := range []string{"SANDBOX", "LIVE"} {
		out, err := s.Create(context.Background(), removedAccountTx{}, removedAccountScope(), "token", "validkey1", CreateInput{Provider: "payuni", Environment: environment, AccountID: "mock-account", Credentials: Credentials{"01234567890123456789012345678901", "0123456789012345"}})
		if !errors.Is(err, command.ErrInvalid) || out != (Connection{}) {
			t.Fatalf("removed account created: %+v %v", out, err)
		}
	}
}
