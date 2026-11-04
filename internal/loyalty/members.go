// Purpose: member and ledger commands and reads of contracts/loyalty-points-v1.md §1/§2 (RED stub: signatures only).
// Depends on: migration 0223 definers (loyalty.*), internal/command (Run receipts), internal/platform (Scope).
// Used by: internal/httpapi/members.go, tests/foundation/loyalty_points_test.go.
// Invariants: every write is one command.Run receipt in the caller's transaction (LPI8).

package loyalty

import (
	"context"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/platform"
)

func CreateMember(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in MemberInput) (Member, bool, error) {
	return Member{}, false, errNotImplemented
}

func UpdateMember(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, memberID string, in MemberPatch) (Member, bool, error) {
	return Member{}, false, errNotImplemented
}

func Adjust(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, memberID string, in AdjustInput) (AdjustResult, bool, error) {
	return AdjustResult{}, false, errNotImplemented
}

func Erase(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, memberID string) (Erasure, bool, error) {
	return Erasure{}, false, errNotImplemented
}

func Export(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, memberID string) ([]byte, bool, error) {
	return nil, false, errNotImplemented
}

func Search(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, in SearchInput) (SearchResult, error) {
	return SearchResult{}, errNotImplemented
}

func Get(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, memberID, ledgerCursor string) (Detail, error) {
	return Detail{}, errNotImplemented
}

func EntriesForOrder(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, orderID string) (OrderEntries, error) {
	return OrderEntries{}, errNotImplemented
}

func PrecheckAward(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, a Award) error {
	return errNotImplemented
}

func TryAward(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, orderID string, a Award) (AwardOutcome, error) {
	return AwardOutcome{}, errNotImplemented
}
