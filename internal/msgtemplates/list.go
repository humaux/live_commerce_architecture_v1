// Purpose: the list read (GET /message-templates): the latest published version of each merchant template_id, scoped to
// the server-resolved tenant/store by the msgtemplates.list SECURITY DEFINER (inbox:reply). It is a read, so no
// idempotency receipt and no command.Run.
// Depends on: msgtemplates.list definer (migration 0124).
// Used by: internal/httpapi/templates.go (the GET handler) and the foundation tests.

package msgtemplates

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// List returns the latest published version of each template_id in the caller's scope (the definer re-scopes to the
// transaction's tenant/store, so a caller can never widen authority).
func (s *Service) List(ctx context.Context, tx pgx.Tx) (ListOutput, error) {
	out := ListOutput{Items: []ListItem{}}
	rows, err := tx.Query(ctx, `SELECT template_id, version, name, kinds, public_safe, created_at FROM msgtemplates.list()`)
	if err != nil {
		return out, databaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item ListItem
		if err := rows.Scan(&item.TemplateID, &item.Version, &item.Name, &item.Kinds, &item.PublicSafe, &item.CreatedAt); err != nil {
			return out, databaseError(err)
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return out, databaseError(err)
	}
	return out, nil
}
