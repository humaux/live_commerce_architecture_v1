package catalog

// options.go owns the pure catalog-v2 vocabulary shared by products and collections (contracts/storefront-v2.md
// section A): option axes and how SKU option values align with them, the derived SKU title, URL slugs (validation,
// generation from a title, a free-slug probe) and Opt, the absent/null/value JSON field. No SQL beyond freeSlug.
// It never decides visibility or price; internal/catalog's commands and the buyer definers (migrations/0086) do.

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// OptionAxis is one product option (for example "Size": S, M, L). A product has 0..3 axes.
type OptionAxis struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

const (
	maxAxes        = 3
	maxAxisName    = 30
	maxAxisValues  = 50
	maxAxisValue   = 40
	maxSlug        = 80
	defaultSKUName = "預設" // contract: derived title of an axis-less SKU
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validSlug mirrors the SQL CHECK: lowercase words, <= 80, never UUID-shaped (the buyer route tells slug from id
// by the UUID shape, so a UUID-shaped slug would be ambiguous).
func validSlug(s string) bool {
	return len(s) <= maxSlug && slugPattern.MatchString(s) && !command.ValidID(s)
}

// Slugify turns a title into a slug candidate: ASCII letters and digits lowercased, every other run becomes one
// dash. A title with no ASCII letters or digits (for example Chinese only) returns "": the caller falls back to the
// id prefix (pinyin is not required, contract A).
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
	}
	out := b.String()
	if len(out) > maxSlug {
		out = strings.TrimRight(out[:maxSlug], "-")
	}
	if !validSlug(out) {
		return ""
	}
	return out
}

// freeSlug returns a slug not yet used in the caller's store for the given table (catalog.products or
// catalog.collections), base, base-2, base-3 ...; nil (empty base, or 50 taken) lets the database default apply
// (products: the id prefix trigger) or the caller fall back. The table name is a compile-time constant of the two
// callers, never input.
func freeSlug(ctx context.Context, tx pgx.Tx, scope platform.Scope, table, base string) (*string, error) {
	if base == "" {
		return nil, nil
	}
	for n := 1; n <= 50; n++ {
		candidate := base
		if n > 1 {
			suffix := "-" + strconv.Itoa(n)
			if len(base)+len(suffix) > maxSlug {
				candidate = strings.TrimRight(base[:maxSlug-len(suffix)], "-") + suffix
			} else {
				candidate = base + suffix
			}
		}
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE tenant_id=$1 AND store_id=$2 AND slug=$3)`,
			scope.TenantID, scope.StoreID, candidate).Scan(&taken); err != nil {
			return nil, err
		}
		if !taken {
			return &candidate, nil
		}
	}
	return nil, nil
}

func plainText(s string, limit int) bool {
	n := utf8.RuneCountInString(s)
	if n < 1 || n > limit || strings.TrimSpace(s) != s {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// validOptions: at most 3 axes, each with a unique non-empty name <= 30 and 1..50 unique values of <= 40 characters.
func validOptions(axes []OptionAxis) bool {
	if len(axes) > maxAxes {
		return false
	}
	names := map[string]bool{}
	for _, a := range axes {
		if !plainText(a.Name, maxAxisName) || names[a.Name] || len(a.Values) < 1 || len(a.Values) > maxAxisValues {
			return false
		}
		names[a.Name] = true
		seen := map[string]bool{}
		for _, v := range a.Values {
			if !plainText(v, maxAxisValue) || seen[v] {
				return false
			}
			seen[v] = true
		}
	}
	return true
}

// valuesFit reports whether SKU option values align with the axes: one value per axis, each listed on its axis.
func valuesFit(axes []OptionAxis, values []string) bool {
	if len(values) != len(axes) {
		return false
	}
	for i, v := range values {
		found := false
		for _, candidate := range axes[i].Values {
			if candidate == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// skuTitle is the derived variant title: values joined " / ", or "預設" without axes. It is never stored.
func skuTitle(values []string) string {
	if len(values) == 0 {
		return defaultSKUName
	}
	return strings.Join(values, " / ")
}

// Opt is a JSON field that tells absent (leave unchanged) from null (clear) from a value (set). The zero value is
// absent; with `omitzero` it is also omitted when marshalled, so the idempotency hash of "unchanged" differs from
// the hash of "clear".
type Opt[T any] struct {
	Set bool
	Val *T
}

func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Val = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Val = &v
	return nil
}

func (o Opt[T]) MarshalJSON() ([]byte, error) {
	if o.Val == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*o.Val)
}

// IsZero is the omitzero hook: an absent field is not marshalled.
func (o Opt[T]) IsZero() bool { return !o.Set }

func utf8Count(s string) int { return utf8.RuneCountInString(s) }
