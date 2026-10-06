// Purpose: unbind.go is Amendment W6-06B (contracts/meta-ads-v1.md §A/§B) in Go: Service.Unbind (POST meta/unbind,
//   ads:manage, command receipt ads.meta.unbind) and Service.CatalogFeed (GET catalog-feed, ads:read). Unbind detaches
//   the store's enabled meta_ads bindings of one ad account: the definer integration.meta_ads_unbind (migration 0160)
//   locks the bindings, refuses 409 operations_in_flight while any operation is DISPATCHING/UNKNOWN/ACKNOWLEDGED
//   (external-operation-v1 rules 4-5, never cancelled here), destroys the sealed token copies and returns the binding
//   versions; THIS file then CAS-disables each binding as commerce_runtime (0008:115) in the same transaction, so the
//   frozen 0074 bindings_ads_disable_guard trigger can still refuse PT409 binding_in_use (R2-ADS-PAUSE-1) and the
//   rollback keeps the credentials. History is never deleted; success audits ads.account_unbound. CatalogFeed only
//   surfaces the absolute public feed URL (ads.catalog_feed_url) — no secret is involved.
// Depends on: internal/command (Run/Audit), internal/platform (scope + sentinels), errors.go (frozen refusals,
//   mapError), service.go (tokenHash, digitsPattern, queryJSON), migrations 0008/0064/0074/0095/0160.
// Used by: internal/httpapi/ads.go (the two W6-06B routes); tests internal/ads/unbind_pg_test.go.

package ads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// UnbindInput is the exact body of POST meta/unbind (Amendment W6-06B §A): the asset of the meta_ads binding.
type UnbindInput struct {
	AdAccountID string `json:"ad_account_id"`
}

// UnbindResult is the 200 body: {unbound, ad_account_id, binding_ids, already_unbound}. BindingIDs is never nil so a
// replay of the stored receipt is byte-equal to the first response.
type UnbindResult struct {
	Unbound        bool     `json:"unbound"`
	AdAccountID    string   `json:"ad_account_id"`
	BindingIDs     []string `json:"binding_ids"`
	AlreadyUnbound bool     `json:"already_unbound"`
}

// unbindDefinerResult is the jsonb of integration.meta_ads_unbind (0160): either the in-flight refusal (carrying the
// operation list as transport details), the idempotent no-op, or the destroyed bindings with their versions for the CAS.
type unbindDefinerResult struct {
	Refused         string `json:"refused"`
	OperationsTotal int    `json:"operations_total"`
	Operations      []struct {
		OperationID string `json:"operation_id"`
		Action      string `json:"action"`
		State       string `json:"state"`
	} `json:"operations"`
	Unbound  bool `json:"unbound"`
	Bindings []struct {
		BindingID            string `json:"binding_id"`
		BindingVersion       int64  `json:"binding_version"`
		CredentialsDestroyed int    `json:"credentials_destroyed"`
	} `json:"bindings"`
}

// Unbind detaches every enabled meta_ads binding of in.AdAccountID under the ads.meta.unbind command receipt
// (I02): definer first (lock + in-flight check + token destruction), then the CAS disable per binding in the SAME
// transaction, then the audit row. Idempotent: an already-unbound (or never-bound) asset is a 200 no-op without an
// audit row; a replayed key returns the stored receipt. Refusals: operations_in_flight 409 (details list the ops),
// binding_in_use 409 (frozen trigger, everything rolled back), plus the standard ADnnn auth/validation codes.
func (s *Service) Unbind(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in UnbindInput) (out UnbindResult, err error) {
	hash, err := tokenHash(token)
	if err != nil || s == nil {
		return out, platform.ErrUnauthorized
	}
	if !digitsPattern.MatchString(in.AdAccountID) {
		return out, refusal("invalid_request")
	}
	out.AdAccountID = in.AdAccountID
	out.BindingIDs = []string{}
	err = command.Run(ctx, tx, scope, "ads.meta.unbind", key, struct {
		PrincipalID string `json:"principal_id"`
		AdAccountID string `json:"ad_account_id"`
	}{scope.PrincipalID, in.AdAccountID}, &out, func() error {
		var res unbindDefinerResult
		// Calls integration.meta_ads_unbind (migration 0160, meta-ads-v1 Amendment W6-06B §A): auth + lock + in-flight check + token destruction.
		if e := tx.QueryRow(ctx, `SELECT integration.meta_ads_unbind($1,$2,$3)`, hash, scope.StoreID, in.AdAccountID).Scan(&res); e != nil {
			return e
		}
		if res.Refused == "operations_in_flight" {
			// Never cancel or blind-retry (external-operation-v1 rules 4-5); the list rides the transport
			// details (bounded at 50 by the definer), ids/actions/states only.
			ops := make([]map[string]any, 0, len(res.Operations))
			for _, o := range res.Operations {
				ops = append(ops, map[string]any{"operation_id": o.OperationID, "action": o.Action, "state": o.State})
			}
			return &Refusal{Status: http.StatusConflict, Code: "operations_in_flight",
				Details: map[string]any{"operations": ops, "operations_total": res.OperationsTotal}}
		}
		if !res.Unbound {
			out.AlreadyUnbound = true // idempotent no-op: nothing destroyed or audited (only the command receipt is stored)
			return nil
		}
		destroyed := 0
		for _, b := range res.Bindings {
			// Detach as commerce_runtime, CAS on the version the definer locked (0008:115 grant). The frozen
			// 0074 guard may raise PT409 binding_in_use here (R2-ADS-PAUSE-1); mapError turns it into the
			// frozen refusal and the caller's transaction rolls the credential destruction back with it.
			var v int64
			e := tx.QueryRow(ctx, `UPDATE integration.bindings
				SET enabled=false, semantic_version=semantic_version+1, updated_at=clock_timestamp()
				WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND semantic_version=$4 AND enabled
				RETURNING semantic_version`,
				scope.TenantID, scope.StoreID, b.BindingID, b.BindingVersion).Scan(&v)
			if errors.Is(e, pgx.ErrNoRows) {
				return command.ErrConflict // the binding moved under us between lock and CAS
			}
			if e != nil {
				return e
			}
			out.BindingIDs = append(out.BindingIDs, b.BindingID)
			destroyed += b.CredentialsDestroyed
		}
		out.Unbound = true
		// Audit details: the public ad-account id and counts only, never a token or ciphertext (AD2, no PII).
		return command.AuditDetails(ctx, tx, scope, "ads.account_unbound",
			map[string]any{"ad_account_id": in.AdAccountID, "bindings": len(res.Bindings), "credentials_destroyed": destroyed})
	})
	return out, mapError(err)
}

// CatalogFeed is GET catalog-feed (Amendment W6-06B §B): ads.catalog_feed_url (ads:read) — the absolute public feed
// URL per ACTIVE storefront domain of the published store, for Meta Commerce Manager. The feed is public and
// unsigned, so no secret and no ads:manage.
func (s *Service) CatalogFeed(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) (json.RawMessage, error) {
	hash, err := tokenHash(token)
	if err != nil {
		return nil, err
	}
	// Calls ads.catalog_feed_url (migration 0160, meta-ads-v1 Amendment W6-06B §B): pure read, ads:read.
	return queryJSON(ctx, tx, `SELECT ads.catalog_feed_url($1,$2)`, hash, scope.StoreID)
}
