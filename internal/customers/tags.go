// Purpose: merchant customer tags (W6-01B): the store tag catalogue (create/rename/delete/list) and the whole-set
//   replacement of one customer's tags guarded by a revision token. Tags are merchant-typed facts, never inferred.
// Depends on: SQL definers customers.create_tag / rename_tag / delete_tag / set_owner_tags / list_tags (migration 0139,
//   customers:write for writes, customers:read for the list), internal/command (Run: scoped idempotency receipts),
//   internal/platform (RequirePermission second fence).
// Used by: internal/httpapi/customer_tags.go (merchant routes), tests/foundation/customer_tags_test.go.
// Invariants: customers-billing-v1 Amendment W6-01B (<=100 tags per store, <=20 per customer, names unique case-insensitively).

package customers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/text/unicode/norm"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// Tag colours the store may use (the 0139 CHECK carries the same eight).
var tagColors = map[string]bool{"gray": true, "red": true, "orange": true, "yellow": true, "green": true, "teal": true, "blue": true, "purple": true}

var revisionPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// maxTagSet bounds the ids a request may carry; the database enforces the real cap of 20 (limit_reached), so a
// 21-tag request is a coded refusal rather than a malformed one.
const maxTagSet = 100

// Tag is one tag as shown on a customer row.
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// TagRecord is a tag of the store catalogue with its usage count.
type TagRecord struct {
	Tag
	CreatedAt      string `json:"created_at"`
	CustomersCount int64  `json:"customers_count"`
}

// TagInput is the create body: both fields required.
type TagInput struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// TagPatch is the rename/recolour body: at least one field present.
type TagPatch struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

// SetTagsInput replaces the tag set of one customer. Revision is the tags_revision of the detail the merchant read.
type SetTagsInput struct {
	TagIDs   []string `json:"tag_ids"`
	Revision string   `json:"revision"`
}

// TagSet is the result of a set replacement: the new tags and the new revision.
type TagSet struct {
	Tags         []Tag  `json:"tags"`
	TagsRevision string `json:"tags_revision"`
}

// tagSetReceipt is what the idempotency receipt of a tag-set write stores: the result plus the customer id, so erasure can
// delete the receipt of an erased customer (customers.erase_tags_notes matches operation and customer_id).
type tagSetReceipt struct {
	Tags         []Tag  `json:"tags"`
	TagsRevision string `json:"tags_revision"`
	CustomerID   string `json:"customer_id"`
}

// DeletedTag reports how many customer links went with the tag.
type DeletedTag struct {
	TagID    string `json:"tag_id"`
	Detached int64  `json:"detached"`
}

// ValidTagName is the Go half of the name rule (the database re-checks NFC): 1..20 characters, valid UTF-8, trimmed,
// no control characters and no invisible format characters (Unicode Cf, e.g. U+200B, which would let two tags look equal).
func ValidTagName(name string) bool {
	if !utf8.ValidString(name) || name == "" || utf8.RuneCountInString(name) > 20 || name != trimSpace(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

// nfc normalises a name to NFC before it is validated and sent, so a decomposed spelling is stored composed.
func nfc(name string) string { return norm.NFC.String(name) }

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end {
		r, n := utf8.DecodeRuneInString(s[start:end])
		if !unicode.IsSpace(r) {
			break
		}
		start += n
	}
	for end > start {
		r, n := utf8.DecodeLastRuneInString(s[start:end])
		if !unicode.IsSpace(r) {
			break
		}
		end -= n
	}
	return s[start:end]
}

func validTag(t Tag) bool { return command.ValidID(t.ID) && ValidTagName(t.Name) && tagColors[t.Color] }

// validTags checks a projected tag list: non-nil, at most 20, ids distinct.
func validTags(tags []Tag) bool {
	if tags == nil || len(tags) > 20 {
		return false
	}
	seen := map[string]bool{}
	for _, t := range tags {
		if !validTag(t) || seen[t.ID] {
			return false
		}
		seen[t.ID] = true
	}
	return true
}

// mapTagError is mapMerchantError plus the 0139 coded conflicts (PT409 messages tag_exists / limit_reached /
// version_changed) and PT422 (invalid input the Go validation could not see, e.g. a non-NFC name or an unknown tag id).
func mapTagError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT409":
			switch pg.Message {
			case "tag_exists":
				return ErrTagExists
			case "limit_reached":
				return ErrLimitReached
			case "version_changed":
				return ErrVersionChanged
			}
		case "PT422":
			return command.ErrInvalid
		}
	}
	return mapMerchantError(err)
}

// writeCall runs one idempotent customers:write definer call inside the caller's transaction. A replay of the same key and
// request returns the stored result (ops.command_results keeps the marshalled out); the same key with another request is
// ErrIdempotencyConflict. out must be a pointer. decode turns the definer's JSON into out; nil means a strict decode, and
// the notes pass their own so that a note BODY never reaches the stored receipt (erasure could not reach it there).
func writeCall(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, operation string, request, out any,
	decode func(raw []byte) error, sql string, args ...any) error {
	if tx == nil || !validAuthorityInput(scope, token) {
		return command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	err := command.Run(ctx, tx, scope, operation, key, request, out, func() error {
		var raw []byte
		// The definer named in sql is a customers:write SECURITY DEFINER of migration 0139: it resolves access from the
		// token hash, re-checks the GUC scope, writes, audits (no body) and fences authority again.
		if err := tx.QueryRow(ctx, sql, append([]any{hash[:], scope.StoreID}, args...)...).Scan(&raw); err != nil {
			return mapTagError(err)
		}
		// Second fence with the original Go Scope (read() pattern).
		if err := platform.RequirePermission(ctx, tx, scope, token, "customers:write"); err != nil {
			return mapMerchantError(err)
		}
		if decode != nil {
			return decode(raw)
		}
		return decodeInto(raw, out)
	})
	if errors.Is(err, command.ErrConflict) {
		return ErrIdempotencyConflict
	}
	return err
}

// decodeInto strictly decodes a definer result (unknown fields are drift, not data).
func decodeInto(raw []byte, out any) error {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return ErrUnavailable
	}
	return strict(raw, out)
}

// CreateTag adds a tag to the store catalogue (customers:write; DB: customers.create_tag). ErrTagExists for a name used
// case-insensitively, ErrLimitReached at 100 tags.
func CreateTag(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in TagInput) (TagRecord, error) {
	var out TagRecord
	in.Name = nfc(in.Name)
	if !ValidTagName(in.Name) || !tagColors[in.Color] {
		return out, command.ErrInvalid
	}
	err := writeCall(ctx, tx, scope, token, key, "customer.tag.create", in, &out, nil,
		`SELECT customers.create_tag($1,$2::uuid,$3,$4)`, in.Name, in.Color)
	return out, err
}

// RenameTag changes the name and/or colour of a tag (DB: customers.rename_tag).
func RenameTag(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, tagID string, in TagPatch) (TagRecord, error) {
	var out TagRecord
	if in.Name != nil {
		n := nfc(*in.Name)
		in.Name = &n
	}
	if !command.ValidID(tagID) || (in.Name == nil && in.Color == nil) || (in.Name != nil && !ValidTagName(*in.Name)) ||
		(in.Color != nil && !tagColors[*in.Color]) {
		return out, command.ErrInvalid
	}
	err := writeCall(ctx, tx, scope, token, key, "customer.tag.rename", struct {
		TagID string `json:"tag_id"`
		TagPatch
	}{tagID, in}, &out, nil, `SELECT customers.rename_tag($1,$2::uuid,$3::uuid,$4,$5)`, tagID, in.Name, in.Color)
	return out, err
}

// DeleteTag removes a tag and its customer links (DB: customers.delete_tag); customers are untouched.
func DeleteTag(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, tagID string) (DeletedTag, error) {
	var out DeletedTag
	if !command.ValidID(tagID) {
		return out, command.ErrInvalid
	}
	err := writeCall(ctx, tx, scope, token, key, "customer.tag.delete", map[string]string{"tag_id": tagID}, &out, nil,
		`SELECT customers.delete_tag($1,$2::uuid,$3::uuid)`, tagID)
	return out, err
}

// SetOwnerTags replaces the whole tag set of a customer (DB: customers.set_owner_tags). ErrVersionChanged when the
// revision is stale, ErrLimitReached above 20 tags, command.ErrInvalid for an unknown tag id.
func SetOwnerTags(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, customerID string, in SetTagsInput) (TagSet, error) {
	var out TagSet
	var rec tagSetReceipt
	if !command.ValidID(customerID) || !revisionPattern.MatchString(in.Revision) || len(in.TagIDs) > maxTagSet {
		return out, command.ErrInvalid
	}
	ids := in.TagIDs
	if ids == nil {
		ids = []string{}
	}
	for _, id := range ids {
		if !command.ValidID(id) {
			return out, command.ErrInvalid
		}
	}
	err := writeCall(ctx, tx, scope, token, key, "customer.tags.set", struct {
		CustomerID string `json:"customer_id"`
		SetTagsInput
	}{customerID, SetTagsInput{TagIDs: ids, Revision: in.Revision}}, &rec, func(raw []byte) error {
		if err := decodeInto(raw, &out); err != nil {
			return err
		}
		rec = tagSetReceipt{out.Tags, out.TagsRevision, customerID}
		return nil
	}, `SELECT customers.set_owner_tags($1,$2::uuid,$3::uuid,$4::uuid[],$5)`, customerID, ids, in.Revision)
	if err != nil {
		return TagSet{}, err
	}
	if !validTags(rec.Tags) || !revisionPattern.MatchString(rec.TagsRevision) {
		return TagSet{}, ErrUnavailable
	}
	return TagSet{rec.Tags, rec.TagsRevision}, nil
}

// ListTags returns the store tag catalogue, name-ordered, with usage counts (customers:read; DB: customers.list_tags).
func ListTags(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) ([]TagRecord, error) {
	if tx == nil || !validAuthorityInput(scope, token) {
		return nil, command.ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// customers.list_tags: customers:read, tenant/store from the verified GUCs, authority fenced after the read.
	if err := tx.QueryRow(ctx, `SELECT customers.list_tags($1,$2::uuid)`, hash[:], scope.StoreID).Scan(&raw); err != nil {
		return nil, mapTagError(err)
	}
	if err := platform.RequirePermission(ctx, tx, scope, token, "customers:read"); err != nil {
		return nil, mapMerchantError(err)
	}
	var out []TagRecord
	if len(raw) == 0 || len(raw) > 1<<20 || json.Unmarshal(raw, &out) != nil || out == nil || len(out) > 100 {
		return nil, ErrUnavailable
	}
	for _, t := range out {
		if !validTag(t.Tag) || t.CustomersCount < 0 {
			return nil, ErrUnavailable
		}
	}
	return out, nil
}
