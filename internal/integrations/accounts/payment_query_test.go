// payment_query_test.go proves retired credential rotation cannot restore a removed payment execution path.
package accounts

import (
	"context"
	"errors"
	"testing"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
)

func TestRemovedAccountRotateNeverReadsOrWrites(t *testing.T) {
	s, err := New(testKeyring(t), &core.Service{})
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []int64{1, 2} {
		out, err := s.Rotate(context.Background(), removedAccountTx{}, removedAccountScope(), "token", "validkey1", RotateInput{ConnectionID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", ExpectedVersion: version, Credentials: Credentials{"01234567890123456789012345678901", "0123456789012345"}})
		if !errors.Is(err, command.ErrInvalid) || out != (Connection{}) {
			t.Fatalf("retired credential rotated: %+v %v", out, err)
		}
	}
}
