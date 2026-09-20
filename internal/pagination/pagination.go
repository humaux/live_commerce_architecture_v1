// Package pagination provides bounded, opaque positions for scoped keyset lists.
package pagination

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"livecommerce/internal/command"
)

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
	if !validBinding(binding) || (keyCount != 1 && keyCount != 2) || request.Limit < 0 || request.Limit > maxLimit {
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
		if !command.ValidID(key) {
			return 0, nil, invalid("cursor")
		}
	}
	return limit, value.Keys, nil
}

func Encode(binding Binding, keys []string) (string, error) {
	if !validBinding(binding) || (len(keys) != 1 && len(keys) != 2) {
		return "", invalid("cursor")
	}
	for _, key := range keys {
		if !command.ValidID(key) {
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
	case "products", "warehouses", "inventory":
		return b.ParentID == "" && b.Filter == ""
	case "skus":
		return command.ValidID(b.ParentID) && b.Filter == ""
	case "catalog-ledger":
		return command.ValidID(b.ParentID) && len(b.Filter) == 64
	default:
		return false
	}
}

func invalid(what string) error { return fmt.Errorf("%w: invalid %s", command.ErrInvalid, what) }
