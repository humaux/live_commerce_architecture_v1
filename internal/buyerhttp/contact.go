// Purpose: public guest contact admission at the private BFF-only buyer boundary.
// Depends on: storefront contact encryption, the verified origin resolver and migration 0211's issuer definer.
// Used by: handler.ServeHTTP for POST /v1/buyer/contact-messages.
// Invariants: one transaction owns replay/throttle/persistence/outbox; 201 only after commit; no cookie, mail or PII logging.
package buyerhttp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/integrations/meta"
	"livecommerce/internal/storefront"
)

const contactRoute routeKind = 222

func (h *handler) contactPost(ctx context.Context, w http.ResponseWriter, r *http.Request, store, key string) error {
	// decodeJSON's legacy malformed-body status is retained everywhere else. This contract rejects
	// unknown keys, duplicate keys, missing fields and invalid field types uniformly as 422.
	if mime, one := oneHeader(r, "Content-Type"); !one || mime != "application/json" && mime != "application/json; charset=utf-8" {
		return responseError{415, "json_required"}
	}
	if r.Body == nil || r.ContentLength > maxJSON {
		return responseError{422, "invalid_request"}
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxJSON+1))
	if err != nil {
		return responseError{503, "unavailable"}
	}
	defer clear(raw)
	if len(raw) > maxJSON {
		return responseError{422, "invalid_request"}
	}
	fields, err := meta.ParseStrict(raw)
	if err != nil {
		return responseError{422, "invalid_request"}
	}
	for _, v := range fields {
		if _, ok := v.(string); !ok {
			return responseError{422, "invalid_request"}
		}
	}
	var in storefront.ContactInput
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || !storefront.ValidContactInput(in) {
		return responseError{422, "invalid_request"}
	}
	ip, ok := lookupClientIP(r)
	if !ok {
		return responseError{422, "invalid_request"}
	}
	if ip == nil {
		v := sha256.Sum256([]byte("contact:no-ip"))
		ip = v[:]
	}
	keys, err := storefront.LoadContactKeys()
	if err != nil {
		return responseError{503, "unavailable"}
	}
	tx, err := h.issuerPool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	digest := sha256.Sum256([]byte(key))
	query := `SELECT coalesce(id::text,''),tenant_id::text,store_id::text,coalesce(key_id,''),nonce,ciphertext,status,version FROM storefront.admit_store_contact($1,$2,$3,$4,$5,$6,$7)`
	var e storefront.ContactEnvelope
	scan := func(row pgx.Row) error {
		return row.Scan(&e.ID, &e.TenantID, &e.StoreID, &e.KeyID, &e.Nonce, &e.Ciphertext, &e.Status, &e.Version)
	}
	err = scan(tx.QueryRow(ctx, query, r.Header.Get("X-Commerce-Storefront-Origin"), digest[:], nil, nil, nil, nil, nil))
	if err == nil && e.ID == "" && e.StoreID == store {
		e, err = keys.Seal(e.TenantID, store, in)
		if err == nil {
			err = scan(tx.QueryRow(ctx, query, r.Header.Get("X-Commerce-Storefront-Origin"), digest[:], e.ID, e.KeyID, e.Nonce, e.Ciphertext, ip))
		}
	}
	if err != nil {
		return contactDatabaseError(err)
	}
	if e.StoreID != store {
		return responseError{404, "not_found"}
	}
	original, err := keys.Open(e)
	if err != nil {
		return responseError{503, "unavailable"}
	}
	if !storefront.MatchesContact(original, in) {
		return responseError{409, "conflict"}
	}
	if err = tx.Commit(ctx); err != nil {
		return responseError{503, "unavailable"}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	return json.NewEncoder(w).Encode(struct {
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
	}{e.ID, "received"})
}

func contactDatabaseError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT404":
			return responseError{404, "not_found"}
		case "PT422", "PT400":
			return responseError{422, "invalid_request"}
		case "PT429":
			return codedResponse{responseError{429, "rate_limited"}, 600 - int(time.Now().Unix()%600)}
		case "PT409":
			return responseError{409, "conflict"}
		}
	}
	return responseError{503, "unavailable"}
}
