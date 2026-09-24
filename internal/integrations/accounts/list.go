package accounts

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// Metadata is the public account projection. It never carries key identifiers
// or credential material, including when used outside the HTTP composition.
type Metadata struct {
	ID                string    `json:"id"`
	Provider          string    `json:"provider"`
	Environment       string    `json:"environment"`
	AccountID         string    `json:"account_id"`
	BindingID         string    `json:"binding_id"`
	BindingVersion    int64     `json:"binding_version"`
	Enabled           bool      `json:"enabled"`
	CredentialVersion int64     `json:"credential_version"`
	State             string    `json:"state"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// List checks current permission on both sides of the scoped read. A revoked
// permission while rows are pending cannot release a partial page or cursor.
func (s *Service) List(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, request pagination.Request) (pagination.Page[Metadata], error) {
	page := pagination.Page[Metadata]{Items: make([]Metadata, 0)}
	if s == nil || s.keys == nil || s.bindings == nil {
		return page, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return page, err
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "provider-accounts"}
	limit, after, err := pagination.Decode(request, binding, 1)
	if err != nil {
		return page, err
	}
	args := []any{scope.TenantID, scope.StoreID}
	query := `SELECT a.id::text,a.provider,a.environment,a.account_id,a.binding_id::text,
		b.semantic_version,b.enabled,a.credential_version,a.created_at,a.updated_at
		FROM integration.merchant_accounts a
		JOIN integration.bindings b ON b.tenant_id=a.tenant_id AND b.store_id=a.store_id AND b.id=a.binding_id
		WHERE a.tenant_id=$1 AND a.store_id=$2`
	if len(after) == 1 {
		query += ` AND a.id>$3::uuid`
		args = append(args, after[0])
	}
	query += ` ORDER BY a.id LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return pagination.Page[Metadata]{}, mapError(err)
	}
	for rows.Next() {
		var item Metadata
		if err := rows.Scan(&item.ID, &item.Provider, &item.Environment, &item.AccountID,
			&item.BindingID, &item.BindingVersion, &item.Enabled, &item.CredentialVersion,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			rows.Close()
			return pagination.Page[Metadata]{}, mapError(err)
		}
		item.State = configuredUnverified
		page.Items = append(page.Items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return pagination.Page[Metadata]{}, mapError(err)
	}
	if err := authorize(ctx, tx, scope, token, "integration:read"); err != nil {
		return pagination.Page[Metadata]{}, err
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		page.NextCursor, err = pagination.Encode(binding, []string{page.Items[len(page.Items)-1].ID})
		if err != nil {
			return pagination.Page[Metadata]{}, err
		}
	}
	return page, nil
}
