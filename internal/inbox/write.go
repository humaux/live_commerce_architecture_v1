package inbox

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// MarkRead is A10. The definer honours read_seq only when the caller holds inbox:reply and returns the value actually
// applied (a read-only reader gets the assignee's unchanged value back).
func (s *Service) MarkRead(ctx context.Context, tx pgx.Tx, conversationID string, in ReadInput) (ReadOutput, error) {
	var out ReadOutput
	err := tx.QueryRow(ctx, `SELECT inbox.mark_read($1::uuid, $2::bigint)`, conversationID, in.ReadSeq).
		Scan(&out.ReadSeq)
	if err != nil {
		return out, databaseError(err)
	}
	return out, nil
}

// Takeover is A11's human transition. assignee is the server-resolved principal (never null for a successful takeover).
func (s *Service) Takeover(ctx context.Context, tx pgx.Tx, conversationID string, in TakeoverInput) (TakeoverOutput, error) {
	var out TakeoverOutput
	err := tx.QueryRow(ctx, `SELECT mode, assignee::text, takeover_generation
		FROM inbox.takeover($1::uuid, $2::bigint)`, conversationID, in.ExpectedGeneration).
		Scan(&out.Mode, &out.Assignee, &out.TakeoverGeneration)
	if err != nil {
		return out, databaseError(err)
	}
	return out, nil
}

// Release is A11's auto transition; the response assignee is always null.
func (s *Service) Release(ctx context.Context, tx pgx.Tx, conversationID string, in TakeoverInput) (TakeoverOutput, error) {
	var out TakeoverOutput
	err := tx.QueryRow(ctx, `SELECT mode, assignee::text, takeover_generation
		FROM inbox.release($1::uuid, $2::bigint)`, conversationID, in.ExpectedGeneration).
		Scan(&out.Mode, &out.Assignee, &out.TakeoverGeneration)
	if err != nil {
		return out, databaseError(err)
	}
	return out, nil
}

// CustomerLink is A14. A nil CustomerID unlinks; the definer re-checks the owner in the caller's tenant/store scope
// (CAS version_conflict PT409 on mismatch).
func (s *Service) CustomerLink(ctx context.Context, tx pgx.Tx, conversationID string, in CustomerLinkInput) (CustomerLinkOutput, error) {
	var out CustomerLinkOutput
	err := tx.QueryRow(ctx, `SELECT customer_id::text, version
		FROM inbox.customer_link($1::uuid, $2::uuid, $3::bigint)`, conversationID, in.CustomerID, in.ExpectedVersion).
		Scan(&out.CustomerID, &out.Version)
	if err != nil {
		return out, databaseError(err)
	}
	return out, nil
}
