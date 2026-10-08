// Purpose: private merchant notes about a customer (W6-01B): add, edit (version-guarded), delete and a keyset-paged list.
//   Notes are buyer personal data: erasure deletes them and the buyer self-service export includes them.
// Depends on: SQL definers customers.add_note / edit_note / delete_note (customers:write) and customers.list_notes
//   (customers:read) of migration 0139; internal/command (Run), internal/pagination (collection "customer-notes").
// Used by: internal/httpapi/customer_tags.go (merchant routes), tests/foundation/customer_tags_test.go.
// Invariants: customers-billing-v1 Amendment W6-01B (<=200 notes per customer, body 1..1000 characters; audit rows and
//   logs never carry a body; only the author or a customers:privacy holder edits/deletes a note).

package customers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

// Note is one private merchant note. AuthorID is the staff principal who wrote it; EditedAt is null until edited.
// Own is computed by Go (AuthorID == the caller's principal) after the definer JSON is decoded, never stored or taken
// from the database, so a staff member without customers:privacy can tell which notes they may edit or delete.
type Note struct {
	ID        string  `json:"id"`
	Body      string  `json:"body"`
	AuthorID  string  `json:"author_id"`
	CreatedAt string  `json:"created_at"`
	EditedAt  *string `json:"edited_at"`
	Version   int64   `json:"version"`
	Own       bool    `json:"own"`
}

// markOwn sets Own on every note for the calling principal.
func markOwn(notes []Note, scope platform.Scope) {
	for i := range notes {
		notes[i].Own = notes[i].AuthorID == scope.PrincipalID
	}
}

// NoteInput is the create body.
type NoteInput struct {
	Body string `json:"body"`
}

// EditNoteInput is the edit body: the new text and the version the merchant read.
type EditNoteInput struct {
	Body    string `json:"body"`
	Version int64  `json:"version"`
}

// DeletedNote confirms a deletion.
type DeletedNote struct {
	NoteID string `json:"note_id"`
}

// ValidNoteBody is the Go half of the body rule: valid UTF-8, 1..1000 characters, not blank, no control characters
// other than line breaks and tabs (NUL would also be refused by the database).
func ValidNoteBody(body string) bool {
	if !utf8.ValidString(body) || utf8.RuneCountInString(body) < 1 || utf8.RuneCountInString(body) > 1000 || trimSpace(body) == "" {
		return false
	}
	for _, r := range body {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func validNote(n Note) bool {
	if !command.ValidID(n.ID) || !command.ValidID(n.AuthorID) || !ValidNoteBody(n.Body) || n.Version < 1 {
		return false
	}
	if _, err := canonicalTime(n.CreatedAt); err != nil {
		return false
	}
	if n.EditedAt != nil {
		if _, err := canonicalTime(*n.EditedAt); err != nil {
			return false
		}
	}
	return true
}

// noteReceipt is what the idempotency receipt of a note write stores: the note WITHOUT its body. ops.command_results keeps
// the marshalled result, and a body there would survive erasure; the request body (hashed, never stored) is the replay body.
type noteReceipt struct {
	CustomerID string  `json:"customer_id"` // lets erasure find and delete this receipt (customers.erase_tags_notes)
	ID         string  `json:"id"`
	AuthorID   string  `json:"author_id"`
	CreatedAt  string  `json:"created_at"`
	EditedAt   *string `json:"edited_at"`
	Version    int64   `json:"version"`
}

// decodeNote strictly decodes the definer's note into a receipt.
func decodeNote(rec *noteReceipt, customerID string) func(raw []byte) error {
	return func(raw []byte) error {
		var n Note
		if len(raw) == 0 || len(raw) > 1<<16 || strict(raw, &n) != nil || !validNote(n) {
			return ErrUnavailable
		}
		*rec = noteReceipt{customerID, n.ID, n.AuthorID, n.CreatedAt, n.EditedAt, n.Version}
		return nil
	}
}

// AddNote appends a note (DB: customers.add_note); ErrLimitReached at 200 notes, command.ErrInvalid for a bad body.
func AddNote(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, customerID string, in NoteInput) (Note, error) {
	var rec noteReceipt
	if !command.ValidID(customerID) || !ValidNoteBody(in.Body) {
		return Note{}, command.ErrInvalid
	}
	err := writeCall(ctx, tx, scope, token, key, "customer.note.add", struct {
		CustomerID string `json:"customer_id"`
		NoteInput
	}{customerID, in}, &rec, decodeNote(&rec, customerID), `SELECT customers.add_note($1,$2::uuid,$3::uuid,$4)`, customerID, in.Body)
	if err != nil {
		return Note{}, err
	}
	return Note{rec.ID, in.Body, rec.AuthorID, rec.CreatedAt, rec.EditedAt, rec.Version, rec.AuthorID == scope.PrincipalID}, nil
}

// EditNote replaces a note body (DB: customers.edit_note). ErrVersionChanged on a stale version; platform.ErrForbidden
// when the caller is neither the author nor a customers:privacy holder.
func EditNote(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, customerID, noteID string, in EditNoteInput) (Note, error) {
	var rec noteReceipt
	if !command.ValidID(customerID) || !command.ValidID(noteID) || !ValidNoteBody(in.Body) || in.Version < 1 {
		return Note{}, command.ErrInvalid
	}
	err := writeCall(ctx, tx, scope, token, key, "customer.note.edit", struct {
		CustomerID string `json:"customer_id"`
		NoteID     string `json:"note_id"`
		EditNoteInput
	}{customerID, noteID, in}, &rec, decodeNote(&rec, customerID), `SELECT customers.edit_note($1,$2::uuid,$3::uuid,$4::uuid,$5,$6)`,
		customerID, noteID, in.Body, in.Version)
	if err != nil {
		return Note{}, err
	}
	return Note{rec.ID, in.Body, rec.AuthorID, rec.CreatedAt, rec.EditedAt, rec.Version, rec.AuthorID == scope.PrincipalID}, nil
}

// DeleteNote removes a note (DB: customers.delete_note); same authorship rule as EditNote.
func DeleteNote(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, customerID, noteID string) (DeletedNote, error) {
	var rec struct {
		DeletedNote
		CustomerID string `json:"customer_id"` // receipt-only, see noteReceipt
	}
	if !command.ValidID(customerID) || !command.ValidID(noteID) {
		return DeletedNote{}, command.ErrInvalid
	}
	err := writeCall(ctx, tx, scope, token, key, "customer.note.delete", map[string]string{"customer_id": customerID, "note_id": noteID},
		&rec, func(raw []byte) error {
			if err := decodeInto(raw, &rec.DeletedNote); err != nil {
				return err
			}
			rec.CustomerID = customerID
			return nil
		}, `SELECT customers.delete_note($1,$2::uuid,$3::uuid,$4::uuid)`, customerID, noteID)
	return rec.DeletedNote, err
}

// ListNotes pages one customer's notes newest first (customers:read; DB: customers.list_notes). The cursor is bound to
// tenant, store and customer; an unknown or other-store customer is the not-found class.
func ListNotes(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, customerID string, req pagination.Request) (pagination.Page[Note], error) {
	empty := pagination.Page[Note]{Items: []Note{}}
	if tx == nil || !validAuthorityInput(scope, token) || !command.ValidID(customerID) {
		return empty, command.ErrInvalid
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "customer-notes", ParentID: customerID}
	limit, keys, err := pagination.Decode(req, binding, 2)
	if err != nil {
		return empty, err
	}
	var beforeTime, beforeID any
	if len(keys) == 2 {
		beforeTime, beforeID = keys[0], keys[1]
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	// customers.list_notes: customers:read, customer must be visible to the merchant list, authority fenced after the read.
	if err = tx.QueryRow(ctx, `SELECT customers.list_notes($1,$2::uuid,$3::uuid,$4,$5::timestamptz,$6::uuid)`,
		hash[:], scope.StoreID, customerID, limit+1, beforeTime, beforeID).Scan(&raw); err != nil {
		return empty, mapTagError(err)
	}
	if err = platform.RequirePermission(ctx, tx, scope, token, "customers:read"); err != nil {
		return empty, mapMerchantError(err)
	}
	var notes []Note
	if len(raw) == 0 || len(raw) > 1<<20 || json.Unmarshal(raw, &notes) != nil || notes == nil || len(notes) > limit+1 {
		return empty, ErrUnavailable
	}
	for _, n := range notes {
		if !validNote(n) {
			return empty, ErrUnavailable
		}
	}
	markOwn(notes, scope)
	empty.Items = notes
	if len(notes) > limit {
		empty.Items = notes[:limit]
		last := empty.Items[limit-1]
		if empty.NextCursor, err = pagination.Encode(binding, []string{last.CreatedAt, last.ID}); err != nil {
			return pagination.Page[Note]{Items: []Note{}}, ErrUnavailable
		}
	}
	return empty, nil
}
