package checkout

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/buyer"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
)

type OrderSummary struct {
	OrderID          string    `json:"order_id"`
	CreatedAt        time.Time `json:"created_at"`
	CartID           string    `json:"cart_id"`
	CartVersion      int64     `json:"cart_version"`
	CommercialState  string    `json:"commercial_state"`
	FulfillmentState string    `json:"fulfillment_state"`
	Currency         string    `json:"currency"`
	TotalMinor       int64     `json:"total_minor"`
}

type ordersCursor struct {
	Version   int       `json:"version"`
	Binding   string    `json:"binding"`
	CreatedAt time.Time `json:"created_at"`
	OrderID   string    `json:"order_id"`
}

// The digest hides scope identifiers and binds position, not authority. Every
// page still resolves the active capability and reads through owner-scoped RLS.
func ordersBinding(scope buyer.Scope) string {
	raw, _ := json.Marshal(struct {
		Collection string `json:"collection"`
		TenantID   string `json:"tenant_id"`
		StoreID    string `json:"store_id"`
		OwnerID    string `json:"owner_id"`
	}{"buyer.orders.v1", scope.TenantID, scope.StoreID, scope.OwnerID})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func decodeOrdersCursor(encoded, binding string) (ordersCursor, error) {
	if encoded == "" {
		return ordersCursor{}, nil
	}
	if len(encoded) > 1024 {
		return ordersCursor{}, command.ErrInvalid
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) == 0 || base64.RawURLEncoding.EncodeToString(raw) != encoded {
		return ordersCursor{}, command.ErrInvalid
	}
	var position ordersCursor
	if err = json.Unmarshal(raw, &position); err != nil {
		return ordersCursor{}, command.ErrInvalid
	}
	position.CreatedAt = position.CreatedAt.UTC()
	canonical, err := json.Marshal(position)
	if err != nil || !bytes.Equal(raw, canonical) || position.Version != 1 || position.Binding != binding ||
		position.CreatedAt.IsZero() || position.CreatedAt.Nanosecond()%1000 != 0 || !command.ValidID(position.OrderID) {
		return ordersCursor{}, command.ErrInvalid
	}
	return position, nil
}

func encodeOrdersCursor(binding string, order OrderSummary) string {
	raw, _ := json.Marshal(ordersCursor{1, binding, order.CreatedAt.UTC(), order.OrderID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (s *Service) ListOrders(ctx context.Context, token, storeID string, in pagination.Request) (pagination.Page[OrderSummary], error) {
	page := pagination.Page[OrderSummary]{Items: []OrderSummary{}}
	if ctx == nil || s == nil || s.pool == nil || in.Limit < 0 || in.Limit > 100 || len(in.Cursor) > 1024 {
		return page, command.ErrInvalid
	}
	limit := in.Limit
	if limit == 0 {
		limit = 50
	}
	tokenHash := sha256.Sum256([]byte(token))
	err := buyer.WithScope(ctx, s.pool, token, storeID, func(callCtx context.Context, tx pgx.Tx, scope buyer.Scope) error {
		binding := ordersBinding(scope)
		position, err := decodeOrdersCursor(in.Cursor, binding)
		if err != nil {
			return err
		}
		var afterTime, afterID any
		if position.OrderID != "" {
			afterTime, afterID = position.CreatedAt, position.OrderID
		}
		rows, err := tx.Query(callCtx, `SELECT id::text,created_at,cart_id::text,cart_version,
			commercial_state,fulfillment_state,currency,total_minor FROM checkout.orders
			WHERE tenant_id=$1 AND store_id=$2 AND owner_id=$3
			AND ($4::timestamptz IS NULL OR (created_at,id)<($4::timestamptz,$5::uuid))
			ORDER BY created_at DESC,id DESC LIMIT $6`, scope.TenantID, scope.StoreID, scope.OwnerID, afterTime, afterID, limit+1)
		if err != nil {
			return err
		}
		for rows.Next() {
			var order OrderSummary
			if err = rows.Scan(&order.OrderID, &order.CreatedAt, &order.CartID, &order.CartVersion,
				&order.CommercialState, &order.FulfillmentState, &order.Currency, &order.TotalMinor); err != nil {
				rows.Close()
				return err
			}
			order.CreatedAt = order.CreatedAt.UTC()
			page.Items = append(page.Items, order)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if err = checkCapability(callCtx, tx, tokenHash[:], storeID, scope); err != nil {
			return err
		}
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
			page.NextCursor = encodeOrdersCursor(binding, page.Items[limit-1])
		}
		return nil
	})
	if err != nil {
		return pagination.Page[OrderSummary]{Items: []OrderSummary{}}, safeError(ctx, err)
	}
	return page, nil
}
