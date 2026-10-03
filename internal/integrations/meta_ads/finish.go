package metaads

import (
	"context"
	"github.com/jackc/pgx/v5"
	"livecommerce/internal/integrations/core"
	"regexp"
)

var graphCodePattern = regexp.MustCompile(`^graph_[1-9][0-9]{0,5}$`)

// FinishRefusal only projects Meta's public wording. It is part of the existing
// lease-fenced completion transaction, never a state transition or retry.
func FinishRefusal(ctx context.Context, tx pgx.Tx, claim core.SecretClaim, out core.Outcome) error {
	detail, ok := out.Detail.(GraphRefusal)
	var message any
	if ok && graphCodePattern.MatchString(out.Code) && (out.State == "FAILED_FINAL" || out.State == "UNKNOWN") {
		if detail.UserMessage != "" {
			message = detail.UserMessage
		}
	} else if out.State != "SUCCEEDED" {
		return nil
	}
	if message == nil && out.State != "SUCCEEDED" {
		return nil
	}
	_, err := tx.Exec(ctx, `SELECT ads.finish_operation_refusal($1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text,$7::text)`,
		claim.OperationID, claim.Generation, claim.LeaseToken, claim.Mode, out.State, out.Code, message)
	return err
}
