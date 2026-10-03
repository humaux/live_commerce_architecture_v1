package metaads

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/integrations/core"
)

var graphCodePattern = regexp.MustCompile(`^graph_[1-9][0-9]{0,5}$`)

// FinishRefusal preserves 0112's public wording projection and persists D9
// insights in the existing lease-fenced completion transaction, never a retry.
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
	if err != nil {
		return err
	}
	breakdowns, ok := out.Detail.(*InsightsBreakdowns)
	if out.State != "SUCCEEDED" || out.Code != "graph_read" || !ok || breakdowns == nil {
		return nil
	}
	if _, err := ParseInsights(out.ProviderReference); err != nil {
		return err
	}
	raw, err := json.Marshal(breakdowns)
	if err != nil {
		return err
	}
	// ads.finish_insights_breakdowns: stores only aggregate rows for the operation's
	// authenticated draft/day, atomically with its generation-fenced completion.
	_, err = tx.Exec(ctx, `SELECT ads.finish_insights_breakdowns($1::uuid,$2::bigint,$3::bytea,$4::text,$5::jsonb)`,
		claim.OperationID, claim.Generation, claim.LeaseToken, claim.Mode, raw)
	return err
}
