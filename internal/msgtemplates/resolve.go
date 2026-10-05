// Purpose: the internal resolver the LC-B4 send path uses to look up one published template by id + version — fixed
// templates first, then the caller's merchant templates — returning the body (and its kinds/public_safe/fixed flags)
// to render. The msgtemplates.resolve SECURITY DEFINER re-checks inbox:reply OR live:manage and re-scopes to the
// transaction's tenant/store. This unit owns the resolver so LC-B4 never reaches into the template tables directly.
// Depends on: msgtemplates.resolve definer (migration 0124), internal/command (ErrNotFound).
// Used by: LC-B4's send path; the foundation tests exercise it directly.

package msgtemplates

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
)

// Resolve returns the fixed or merchant template with the given id and version. A fixed id wins even if a merchant
// happened to publish the same id (the definer never allows that, so a fixed id has no merchant rows).
func (s *Service) Resolve(ctx context.Context, tx pgx.Tx, templateID string, version int64) (Resolved, error) {
	var out Resolved
	err := tx.QueryRow(ctx, `SELECT template_id, version, name, body, kinds, public_safe, fixed FROM msgtemplates.resolve($1,$2)`,
		templateID, version).
		Scan(&out.TemplateID, &out.Version, &out.Name, &out.Body, &out.Kinds, &out.PublicSafe, &out.Fixed)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, command.ErrNotFound
	}
	if err != nil {
		return out, databaseError(err)
	}
	return out, nil
}
