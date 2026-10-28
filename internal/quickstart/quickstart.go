// Package quickstart owns the store's persistent quick-start completion and derives the three steps from domain data.
// Purpose: serve AD-03 state and an idempotent completion/dismissal without duplicating catalog or delivery state.
// Depends on: pgx, internal/platform authorization and internal/command validation; control.stores/quickstart_markers,
// catalog.products and fulfillment.service_heads/service_versions (read-only). Only this module writes quickstart_markers.
// Used by: internal/httpapi admin GET/POST /quickstart and tests/quickstart.
// Invariants: server-resolved scope only, scoped RLS, permission checks before/after work, immutable first completion time.
// It never stores client-supplied step flags or calls a provider.
package quickstart

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

type Steps struct {
	StoreName    bool `json:"store_name"`
	Shipping     bool `json:"shipping"`
	FirstProduct bool `json:"first_product"`
}

type State struct {
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at"`
	Steps       Steps      `json:"steps"`
}

func authorize(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, permission string) error {
	if tx == nil || !command.ValidID(scope.TenantID) || !command.ValidID(scope.StoreID) ||
		!command.ValidID(scope.PrincipalID) || scope.Revision == 0 {
		return command.ErrInvalid
	}
	return platform.RequirePermission(ctx, tx, scope, token, permission)
}

// Get reads without creating a marker or changing the derived domain records. Store readers can see their checklist.
func Get(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (State, error) {
	if err := authorize(ctx, tx, scope, token, "store:read"); err != nil {
		return State{}, err
	}
	out, err := read(ctx, tx, scope)
	if err != nil {
		return State{}, err
	}
	if err := authorize(ctx, tx, scope, token, "store:read"); err != nil {
		return State{}, err
	}
	return out, nil
}

// Complete is also dismissal: it does not claim the derived steps are done. The DB owns the first timestamp.
// The primary key and DO NOTHING make concurrent retries safe without an extra command receipt or mutable status.
func Complete(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (State, error) {
	if err := authorize(ctx, tx, scope, token, "integration:manage"); err != nil {
		return State{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO control.quickstart_markers(tenant_id,store_id)
		VALUES($1,$2) ON CONFLICT(tenant_id,store_id) DO NOTHING`, scope.TenantID, scope.StoreID); err != nil {
		return State{}, err
	}
	out, err := read(ctx, tx, scope)
	if err != nil {
		return State{}, err
	}
	if err := authorize(ctx, tx, scope, token, "integration:manage"); err != nil {
		return State{}, err
	}
	return out, nil
}

func read(ctx context.Context, tx pgx.Tx, scope platform.Scope) (State, error) {
	var out State
	err := tx.QueryRow(ctx, `SELECT q.completed_at, btrim(s.name) <> '',
		EXISTS(SELECT 1 FROM fulfillment.service_heads h
		 JOIN fulfillment.service_versions v ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id
		  AND v.market_id=h.market_id AND v.country=h.country AND v.code=h.code AND v.version=h.current_version
		 WHERE h.tenant_id=s.tenant_id AND h.store_id=s.id AND v.enabled),
		EXISTS(SELECT 1 FROM catalog.products p WHERE p.tenant_id=s.tenant_id AND p.store_id=s.id)
		FROM control.stores s LEFT JOIN control.quickstart_markers q ON q.tenant_id=s.tenant_id AND q.store_id=s.id
		WHERE s.tenant_id=$1 AND s.id=$2`, scope.TenantID, scope.StoreID).
		Scan(&out.CompletedAt, &out.Steps.StoreName, &out.Steps.Shipping, &out.Steps.FirstProduct)
	if err == pgx.ErrNoRows {
		return State{}, platform.ErrScopeNotFound
	}
	if err != nil {
		return State{}, err
	}
	out.Completed = out.CompletedAt != nil
	return out, nil
}
