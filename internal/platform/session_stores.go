// Purpose: project active merchant stores and a stable opaque order guard identity.
// Depends on: live session list/resolve_access definers and Go SHA256; used by admin identity-enabled store list.
// Invariant: scopes derive only from authenticated tenant/store/principal, never credentials or browser identity hints.
package platform

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is the only unscoped store projection. Its SQL function derives the
// active merchant principal and grants from a hashed opaque session, never IDs.
func sessionStoresHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
		if !ok || len(r.Header.Values("Authorization")) != 1 || len(token) != 43 || err != nil || len(decoded) != 32 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if r.URL.RawQuery != "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request")
			return
		}
		if pool == nil {
			writeError(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		hash := sha256.Sum256([]byte(token))
		rows, err := pool.Query(ctx, `SELECT s.id::text,s.name,s.currency,s.role,s.permissions,a.tenant_id::text,a.principal_id::text
 FROM identity.list_session_stores($1) s
 CROSS JOIN LATERAL identity.resolve_access($1,s.id,'store:read') a WHERE a.access_status='ok'`, hash[:])
		if err != nil {
			sessionStoresError(w, err)
			return
		}
		defer rows.Close()
		type store struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Currency string `json:"currency"`
			// Role/Permissions (0089, role-aware navigation): the caller's own staff role (null for pre-0089 members) and effective
			// permissions in this store. Display hint only: every request is still authorized by resolve_access.
			Role            *string  `json:"role"`
			Permissions     []string `json:"permissions"`
			OrderGuardScope string   `json:"order_guard_scope"`
		}
		items := make([]store, 0)
		for rows.Next() {
			var item store
			var tenant, principal string
			if err := rows.Scan(&item.ID, &item.Name, &item.Currency, &item.Role, &item.Permissions, &tenant, &principal); err != nil {
				sessionStoresError(w, err)
				return
			}
			guard := sha256.Sum256([]byte("merchant-order-guard/v1|" + tenant + "|" + item.ID + "|" + principal))
			item.OrderGuardScope = hex.EncodeToString(guard[:])
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			sessionStoresError(w, err)
			return
		}
		// ponytail: first-store onboarding needs no pagination. Explicitly fail
		// above 100, never silently drop a grant; add keyset paging with multi-store.
		if len(items) > 100 {
			writeError(w, http.StatusConflict, "conflict")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func sessionStoresError(w http.ResponseWriter, err error) {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "PT401" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeError(w, http.StatusServiceUnavailable, "unavailable")
}
