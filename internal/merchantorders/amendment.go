package merchantorders

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

var (
	ErrAmendmentOnHold = errors.New("order amendment on hold")
	ErrAmendmentStock  = errors.New("insufficient stock for order amendment")
	ErrAmendmentReplay = errors.New("order amendment idempotency conflict")
)

type Addition struct {
	SKUID    string `json:"sku_id"`
	Quantity int64  `json:"quantity"`
}

type RecipientAmendment struct {
	RecipientName *string `json:"recipient_name,omitempty"`
	Phone         *string `json:"phone,omitempty"`
	Region        *string `json:"region,omitempty"`
	City          *string `json:"city,omitempty"`
	PostalCode    *string `json:"postal_code,omitempty"`
	Line1         *string `json:"line1,omitempty"`
	Line2         *string `json:"line2,omitempty"`
}

type AmendmentInput struct {
	ExpectedRevision int64               `json:"expected_revision"`
	Recipient        *RecipientAmendment `json:"recipient,omitempty"`
	Additions        []Addition          `json:"additions,omitempty"`
	Proposal         string              `json:"proposal,omitempty"`
}

func AmendHomeCOD(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, order, key string, in AmendmentInput) (json.RawMessage, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !command.ValidID(order) || (key == "") != (in.Proposal == "") ||
		(key != "" && !shipmentKey.MatchString(key)) {
		return nil, command.ErrInvalid
	}
	var recipient, additions []byte
	if in.Recipient != nil {
		recipient, _ = json.Marshal(in.Recipient)
	}
	if len(in.Additions) > 0 {
		additions, _ = json.Marshal(in.Additions)
	}
	auth := sha256.Sum256([]byte(token))
	var raw json.RawMessage
	err := tx.QueryRow(ctx, `SELECT identity.amend_order($1,$2::uuid,$3::uuid,$4,$5::jsonb,$6::jsonb,NULLIF($7,''),NULLIF($8,''))`,
		auth[:], scope.StoreID, order, in.ExpectedRevision, recipient, additions, key, in.Proposal).Scan(&raw)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "PT409" {
			switch pg.Message {
			case "amendment_on_hold":
				return nil, ErrAmendmentOnHold
			case "insufficient_stock":
				return nil, ErrAmendmentStock
			case "idempotency_conflict":
				return nil, ErrAmendmentReplay
			}
		}
		return nil, mapShipmentError(err)
	}
	return raw, nil
}

func ReadOrderDocument(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, order string, revision int64) (Detail, error) {
	if tx == nil || !validAuthorityInput(scope, token) || !command.ValidID(order) || revision < 1 {
		return Detail{}, command.ErrInvalid
	}
	auth := sha256.Sum256([]byte(token))
	if _, err := tx.Exec(ctx, `SELECT fulfillment.authorize_order_document($1,$2,$3,$4)`, auth[:], scope.StoreID, order, revision); err != nil {
		return Detail{}, mapShipmentError(err)
	}
	return Get(ctx, tx, scope, token, order)
}
