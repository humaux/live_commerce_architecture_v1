// Package pagination provides bounded, opaque positions for scoped keyset lists.
package pagination

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"livecommerce/internal/command"
)

var deliveryCode = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,39}$`)

const (
	defaultLimit = 50
	maxLimit     = 100
	maxCursor    = 1024
)

type Request struct {
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"`
}

type Binding struct {
	TenantID   string `json:"tenant_id"`
	StoreID    string `json:"store_id"`
	Collection string `json:"collection"`
	ParentID   string `json:"parent_id"`
	Filter     string `json:"filter,omitempty"`
}

type cursor struct {
	Version int      `json:"version"`
	Binding Binding  `json:"binding"`
	Keys    []string `json:"keys"`
}

func Decode(request Request, binding Binding, keyCount int) (int, []string, error) {
	if !validBinding(binding) || !validKeyCount(binding.Collection, keyCount) || request.Limit < 0 || request.Limit > maxLimit {
		return 0, nil, invalid("request")
	}
	limit := request.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if request.Cursor == "" {
		return limit, nil, nil
	}
	if len(request.Cursor) > maxCursor {
		return 0, nil, invalid("cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(request.Cursor)
	if err != nil || len(raw) == 0 {
		return 0, nil, invalid("cursor")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value cursor
	if err := decoder.Decode(&value); err != nil {
		return 0, nil, invalid("cursor")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return 0, nil, invalid("cursor")
	}
	if value.Version != 1 || value.Binding != binding || len(value.Keys) != keyCount {
		return 0, nil, invalid("cursor")
	}
	for _, key := range value.Keys {
		if !validKey(binding.Collection, key) {
			return 0, nil, invalid("cursor")
		}
	}
	return limit, value.Keys, nil
}

func Encode(binding Binding, keys []string) (string, error) {
	if !validBinding(binding) || !validKeyCount(binding.Collection, len(keys)) {
		return "", invalid("cursor")
	}
	for _, key := range keys {
		if !validKey(binding.Collection, key) {
			return "", invalid("cursor")
		}
	}
	raw, err := json.Marshal(cursor{Version: 1, Binding: binding, Keys: keys})
	if err != nil || len(raw) > maxCursor {
		return "", invalid("cursor")
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if len(encoded) > maxCursor {
		return "", invalid("cursor")
	}
	return encoded, nil
}

func validBinding(b Binding) bool {
	if !command.ValidID(b.TenantID) || !command.ValidID(b.StoreID) {
		return false
	}
	switch b.Collection {
	case "products", "warehouses", "inventory", "provider-accounts", "markets":
		return b.ParentID == "" && b.Filter == ""
	case "delivery-services":
		return command.ValidID(b.ParentID) && len(b.Filter) == 2 &&
			b.Filter[0] >= 'A' && b.Filter[0] <= 'Z' && b.Filter[1] >= 'A' && b.Filter[1] <= 'Z'
	case "skus":
		return command.ValidID(b.ParentID) && b.Filter == ""
	case "catalog-ledger":
		return command.ValidID(b.ParentID) && len(b.Filter) == 64
	default:
		return false
	}
}

func validKey(collection, key string) bool {
	if collection == "delivery-services" {
		return deliveryCode.MatchString(key)
	}
	return command.ValidID(key)
}

func validKeyCount(collection string, count int) bool {
	if collection == "delivery-services" || collection == "markets" {
		return count == 1
	}
	return count == 1 || count == 2
}

func invalid(what string) error { return fmt.Errorf("%w: invalid %s", command.ErrInvalid, what) }
