// Purpose: signed, versioned store profile reads and replacement writes for settings v2.
// Depends on: platform.WithScope/RequirePermission, command.Run/Audit and migration 0209's restricted writer.
// Used by: NewHandler and the admin profile BFF; foundation profile HTTP/PG gates.
// Invariants: auth-only scope; one store-name source; CAS, original-result replay and audit commit atomically;
// authorization is rechecked after lock waits and replay; no provider calls or contact data in audit/logs.
package httpapi

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

type storeProfile struct {
	Name         string `json:"name"`
	ContactEmail string `json:"contact_email"`
	ContactPhone string `json:"contact_phone"`
	Version      int64  `json:"version"`
}

type storeProfileInput struct {
	Name            string  `json:"name"`
	ContactEmail    string  `json:"contact_email"`
	ContactPhone    *string `json:"contact_phone,omitempty"`
	ExpectedVersion *int64  `json:"expected_version"`
}

func registerProfileRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const path = "/v1/admin/stores/{store_id}/profile"
	read := scoped(pool, "store:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
		var out storeProfile
		err := tx.QueryRow(ctx, `SELECT name,contact_email,contact_phone,profile_version FROM control.stores WHERE tenant_id=$1 AND id=$2`, s.TenantID, s.StoreID).
			Scan(&out.Name, &out.ContactEmail, &out.ContactPhone, &out.Version)
		if err == nil {
			err = platform.RequirePermission(ctx, tx, s, bearerToken(r), "store:read")
		}
		return out, err
	})
	mux.HandleFunc("GET "+path, exactResourceRoute(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			respondError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		read(w, r)
	}))
	mux.HandleFunc("PUT "+path, claimsRoute(http.MethodPut, false, bodyRoute(pool, "integration:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in storeProfileInput) (any, error) {
		if !validProfileInput(in) {
			return nil, command.ErrInvalid
		}
		var out storeProfile
		// Principal is part of the digest: another principal cannot claim a saved command's result.
		request := struct {
			Principal string
			Input     storeProfileInput
		}{s.PrincipalID, in}
		err := command.Run(ctx, tx, s, "store.profile.put", r.Header.Get("Idempotency-Key"), request, &out, func() error {
			phone := ""
			if in.ContactPhone != nil {
				phone = *in.ContactPhone
			}
			hash := sha256.Sum256([]byte(bearerToken(r)))
			err := tx.QueryRow(ctx, `SELECT name,contact_email,contact_phone,version FROM control.set_store_profile($1,$2,$3,$4,$5,$6)`,
				hash[:], s.StoreID, in.Name, in.ContactEmail, phone, *in.ExpectedVersion).
				Scan(&out.Name, &out.ContactEmail, &out.ContactPhone, &out.Version)
			if err != nil {
				return profileDatabaseError(err)
			}
			return command.Audit(ctx, tx, s, "store.profile.put")
		})
		// A replay also waits on command.Run's advisory lock. Recheck before exposing it or committing.
		if err == nil {
			err = platform.RequirePermission(ctx, tx, s, bearerToken(r), "integration:manage")
		}
		return out, err
	})))
	mux.HandleFunc(path, studioRoute("", false, nil))
}

func profileText(s string, max int, required bool) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || (required && strings.TrimSpace(s) == "") {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func validProfileInput(in storeProfileInput) bool {
	if in.ExpectedVersion == nil || *in.ExpectedVersion < 0 || *in.ExpectedVersion >= 9007199254740991 ||
		!profileText(in.Name, 120, true) || len(in.ContactEmail) > 254 ||
		!profileText(in.ContactEmail, 254, true) || strings.ContainsAny(in.ContactEmail, " <>\"\\") {
		return false
	}
	address, err := mail.ParseAddress(in.ContactEmail)
	return err == nil && address.Name == "" && address.Address == in.ContactEmail &&
		(in.ContactPhone == nil || profileText(*in.ContactPhone, 32, false))
}

func profileDatabaseError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.ErrForbidden
		case "PT404":
			return command.ErrNotFound
		case "PT409":
			return command.ErrConflict
		case "PT422":
			return command.ErrInvalid
		}
	}
	return err
}
