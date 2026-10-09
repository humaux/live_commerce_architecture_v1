// Purpose: the merchant settings of the sold-out automatic private reply (W3-04B): whether a sold-out claim gets the "sold out" text and which
// template renders it. The reply itself is planned in SQL (integration.plan_claim_reply) and sent by internal/integrations/metareply.
// Depends on: SQL claims.get_sold_out_reply / claims.set_sold_out_reply (migration 0151; live:read / live:manage re-checked by the definers).
// Used by: internal/httpapi/sold_out_settings.go (W3-U2 settings route) and tests/foundation/sold_out_reply_test.go.
// Invariants: the reply consumes the comment's single private reply (mpr: key); compare-and-swap on Version (0 = never saved).

package claims

import (
	"context"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
)

// SoldOutReply is a store's sold-out reply setting. A never-saved store reads Enabled=true, the fixed template and Version 0.
type SoldOutReply struct {
	Enabled         bool   `json:"enabled"`
	TemplateID      string `json:"template_id"`
	TemplateVersion int64  `json:"template_version"`
	Version         int64  `json:"version"`
}

// GetSoldOutReply reads the caller's store setting. tx must be a platform.WithScope transaction (tenant/store/principal GUCs pinned);
// the definer requires live:read. Read only.
func GetSoldOutReply(ctx context.Context, tx pgx.Tx) (SoldOutReply, error) {
	var out SoldOutReply
	// claims.get_sold_out_reply: definer commerce_integration_writer (migration 0151).
	err := tx.QueryRow(ctx, `SELECT enabled, template_id, template_version, version FROM claims.get_sold_out_reply()`).
		Scan(&out.Enabled, &out.TemplateID, &out.TemplateVersion, &out.Version)
	return out, mapError(err)
}

// SetSoldOutReply saves the setting with a compare-and-swap on expectedVersion (0 for the first save): a stale version is
// command.ErrConflict, a template that is not a usable sold-out text (not private_reply, > 280 characters, a placeholder other than
// {{product.name}}) is command.ErrInvalid, a caller without live:manage is platform.ErrForbidden. Writes one settings row and one audit row
// (claims.sold_out_reply.set). tx must be a platform.WithScope transaction.
func SetSoldOutReply(ctx context.Context, tx pgx.Tx, enabled bool, templateID string, templateVersion, expectedVersion int64) (SoldOutReply, error) {
	if templateID == "" || templateVersion < 1 || expectedVersion < 0 {
		return SoldOutReply{}, command.ErrInvalid
	}
	var out SoldOutReply
	// claims.set_sold_out_reply: definer commerce_integration_writer (migration 0151).
	err := tx.QueryRow(ctx, `SELECT enabled, template_id, template_version, version FROM claims.set_sold_out_reply($1,$2,$3,$4)`,
		enabled, templateID, templateVersion, expectedVersion).Scan(&out.Enabled, &out.TemplateID, &out.TemplateVersion, &out.Version)
	return out, mapError(err)
}
