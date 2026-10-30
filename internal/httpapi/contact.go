// Purpose: merchant contact list/detail and the one versioned mark-handled command.
// Depends on: platform scope, command receipts/audit, storefront encryption and migration 0211's restricted definers.
// Used by: NewHandler and the authenticated admin contact BFF.
// Invariants: inbox permissions rechecked after waits/replay; receipts contain opaque metadata only; private no-store responses.
package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"livecommerce/internal/storefront"
)

type contactPage struct {
	Items      []storefront.ContactDetail `json:"items"`
	NextCursor *string                    `json:"next_cursor"`
}

func contactRows(ctx context.Context, tx pgx.Tx, r *http.Request, store, id string, after any, limit int) ([]storefront.ContactDetail, error) {
	keys, err := storefront.LoadContactKeys()
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(bearerToken(r)))
	var selected any
	if id != "" {
		selected = id
	}
	rows, err := tx.Query(ctx, `SELECT id::text,tenant_id::text,store_id::text,key_id,nonce,ciphertext,status,version FROM storefront.read_store_contacts($1,$2,$3,$4,$5)`, hash[:], store, selected, after, limit)
	if err != nil {
		return nil, profileDatabaseError(err)
	}
	defer rows.Close()
	out := []storefront.ContactDetail{}
	for rows.Next() {
		var e storefront.ContactEnvelope
		if err = rows.Scan(&e.ID, &e.TenantID, &e.StoreID, &e.KeyID, &e.Nonce, &e.Ciphertext, &e.Status, &e.Version); err != nil {
			return nil, err
		}
		if e.StoreID != store {
			return nil, command.ErrNotFound
		}
		detail, err := keys.Open(e)
		if err != nil {
			return nil, err
		}
		out = append(out, detail)
	}
	return out, profileDatabaseError(rows.Err())
}

func registerContactRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	const base = "/v1/admin/stores/{store_id}/contact-messages"
	read := func(detail bool) http.HandlerFunc {
		return scoped(pool, "inbox:read", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			id, limit := r.PathValue("contact_id"), 50
			var after any
			if detail && !command.ValidID(id) {
				return nil, command.ErrInvalid
			}
			if r.URL.ForceQuery || len(r.URL.RawQuery) > 2048 {
				return nil, command.ErrInvalid
			}
			q, err := url.ParseQuery(r.URL.RawQuery)
			if err != nil {
				return nil, command.ErrInvalid
			}
			for k, v := range q {
				if detail || len(v) != 1 {
					return nil, command.ErrInvalid
				}
				switch k {
				case "limit":
					limit, err = strconv.Atoi(v[0])
					if err != nil || limit < 1 || limit > 100 {
						return nil, command.ErrInvalid
					}
				case "cursor":
					b, e := base64.RawURLEncoding.Strict().DecodeString(v[0])
					if e != nil || !command.ValidID(string(b)) {
						return nil, command.ErrInvalid
					}
					after = string(b)
				default:
					return nil, command.ErrInvalid
				}
			}
			items, err := contactRows(ctx, tx, r, s.StoreID, id, after, limit+1)
			if err != nil {
				return nil, err
			}
			if err = platform.RequirePermission(ctx, tx, s, bearerToken(r), "inbox:read"); err != nil {
				return nil, err
			}
			if detail {
				if len(items) == 0 {
					return nil, command.ErrNotFound
				}
				return items[0], nil
			}
			out := contactPage{Items: items}
			if len(items) > limit {
				out.Items = items[:limit]
				c := base64.RawURLEncoding.EncodeToString([]byte(out.Items[limit-1].ID))
				out.NextCursor = &c
			}
			return out, nil
		})
	}
	mux.HandleFunc("GET "+base, claimsRoute(http.MethodGet, true, read(false)))
	mux.HandleFunc("GET "+base+"/{contact_id}", claimsRoute(http.MethodGet, false, read(true)))
	mux.HandleFunc("POST "+base+"/{contact_id}/handled", claimsRoute(http.MethodPost, false, bodyRoute(pool, "inbox:reply", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request, in struct {
		ExpectedVersion *int64 `json:"expected_version"`
	}) (any, error) {
		id := r.PathValue("contact_id")
		if !command.ValidID(id) || in.ExpectedVersion == nil || *in.ExpectedVersion < 0 {
			return nil, command.ErrInvalid
		}
		var receipt struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Version int64  `json:"version"`
		}
		request := struct {
			ID, Principal   string
			ExpectedVersion int64
		}{id, s.PrincipalID, *in.ExpectedVersion}
		err := command.Run(ctx, tx, s, "store.contact.handled", r.Header.Get("Idempotency-Key"), request, &receipt, func() error {
			hash := sha256.Sum256([]byte(bearerToken(r)))
			err := tx.QueryRow(ctx, `SELECT id::text,status,version FROM storefront.handle_store_contact($1,$2,$3,$4)`, hash[:], s.StoreID, id, *in.ExpectedVersion).Scan(&receipt.ID, &receipt.Status, &receipt.Version)
			if err != nil {
				return profileDatabaseError(err)
			}
			return command.Audit(ctx, tx, s, "store.contact.handled")
		})
		if err != nil {
			return nil, err
		}
		if err = platform.RequirePermission(ctx, tx, s, bearerToken(r), "inbox:reply"); err != nil {
			return nil, err
		}
		items, err := contactRows(ctx, tx, r, s.StoreID, id, nil, 1)
		if err != nil {
			return nil, err
		}
		if len(items) != 1 {
			return nil, command.ErrNotFound
		}
		out := items[0]
		out.Status = receipt.Status
		out.Version = receipt.Version
		return out, nil
	})))
}
