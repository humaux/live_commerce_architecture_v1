// Purpose: member points v1 (contracts/loyalty-points-v1.md LP1-LP19) — the types, sentinel errors, input rules and the HTTP
//   refusal mapping of merchant-kept members and their append-only points ledger.
// Depends on: internal/command (ValidID, ErrInvalid), internal/platform (authority sentinels).
// Used by: members.go and award.go (this package), internal/httpapi/members.go, internal/merchanttools/manual.go,
//   tests/foundation/loyalty_points_test.go.
// Invariants: LPI7 no member PII in receipts, audit details, logs or URLs (receipt types below hold ids, numbers, versions, states);
//   refusal codes are exactly the contract §2 set; a backstop CHECK (23514) is never a refusal.

package loyalty

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
)

// Sentinel refusals of contract §2 (Classify maps them to status + code).
var (
	ErrInsufficient        = errors.New("points_insufficient")
	ErrErased              = errors.New("member_erased")
	ErrLimit               = errors.New("limit_reached")
	ErrVersion             = errors.New("conflict")
	ErrExportTooLarge      = errors.New("export_too_large")
	ErrIdempotencyConflict = errors.New("idempotency_conflict")
	ErrUnavailable         = errors.New("loyalty unavailable")
)

// Award bounds and states (LP7, LP18).
const (
	MaxPoints             = 1_000_000
	MaxBalance      int64 = 1_000_000_000_000
	StateAwarded          = "awarded"
	StateNotAwarded       = "not_awarded"
	MaxExportBytes        = 1 << 20
)

// Member is the member row as the admin sees it.
type Member struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	ContactNote *string `json:"contact_note"`
	Notes       *string `json:"notes"`
	Balance     int64   `json:"balance"`
	Version     int64   `json:"version"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	ErasedAt    *string `json:"erased_at"`
}

// MemberInput is the POST members body.
type MemberInput struct {
	DisplayName string  `json:"display_name"`
	ContactNote *string `json:"contact_note"`
	Notes       *string `json:"notes"`
}

// MemberPatch is the PATCH members/{id} body. Fields lists the keys present in the request (an explicit null clears an optional
// field); it is part of the hashed request so "absent" and "null" are different requests.
type MemberPatch struct {
	ExpectedVersion int64    `json:"expected_version"`
	DisplayName     *string  `json:"display_name"`
	ContactNote     *string  `json:"contact_note"`
	Notes           *string  `json:"notes"`
	Fields          []string `json:"fields"`
}

// AdjustInput is the POST members/{id}/adjustments body (LP10).
type AdjustInput struct {
	Delta   int64   `json:"delta"`
	Note    string  `json:"note"`
	OrderID *string `json:"order_id"`
}

// Entry is one ledger entry.
type Entry struct {
	ID               string  `json:"id"`
	MemberID         string  `json:"member_id"`
	Delta            int64   `json:"delta"`
	Reason           string  `json:"reason"`
	OrderID          *string `json:"order_id"`
	Note             *string `json:"note"`
	ActorPrincipalID string  `json:"actor_principal_id"`
	CreatedAt        string  `json:"created_at"`
}

// AdjustResult is the 201 body of an adjust: the entry (read from the current row) and the balance after it.
type AdjustResult struct {
	Entry   Entry `json:"entry"`
	Balance int64 `json:"balance"`
}

// Erasure is the erasure summary (also the stored receipt).
type Erasure struct {
	MemberID        string `json:"member_id"`
	ErasedAt        string `json:"erased_at"`
	EntriesRedacted int64  `json:"entries_redacted"`
}

// SearchInput is the POST members/search body (the search term never travels in a URL).
type SearchInput struct {
	Q             *string `json:"q"`
	IncludeErased bool    `json:"include_erased"`
	Cursor        *string `json:"cursor"`
	Limit         int     `json:"limit"`
}

// SearchItem is one search row.
type SearchItem struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	ContactNote *string `json:"contact_note"`
	Balance     int64   `json:"balance"`
	UpdatedAt   string  `json:"updated_at"`
	Erased      bool    `json:"erased"`
}

// SearchResult is one search page.
type SearchResult struct {
	Members    []SearchItem `json:"members"`
	NextCursor *string      `json:"next_cursor"`
}

// Actor names the staff principal of an entry. DisplayName is null: no staff display-name store exists (contract Q8).
type Actor struct {
	PrincipalID string  `json:"principal_id"`
	DisplayName *string `json:"display_name"`
}

// DetailEntry is one ledger row of the member detail.
type DetailEntry struct {
	ID        string  `json:"id"`
	Delta     int64   `json:"delta"`
	Reason    string  `json:"reason"`
	OrderID   *string `json:"order_id"`
	Note      *string `json:"note"`
	Actor     Actor   `json:"actor"`
	CreatedAt string  `json:"created_at"`
}

// Detail is the GET members/{id} body.
type Detail struct {
	Member           Member        `json:"member"`
	Entries          []DetailEntry `json:"entries"`
	NextLedgerCursor *string       `json:"next_ledger_cursor"`
}

// OrderEntry is one entry referencing an order (LP19).
type OrderEntry struct {
	ID                string  `json:"id"`
	MemberID          string  `json:"member_id"`
	MemberDisplayName string  `json:"member_display_name"`
	Delta             int64   `json:"delta"`
	Reason            string  `json:"reason"`
	Note              *string `json:"note"`
	CreatedAt         string  `json:"created_at"`
}

// OrderEntries is the GET members/entries?order_id= body.
type OrderEntries struct {
	Entries   []OrderEntry `json:"entries"`
	Truncated bool         `json:"truncated,omitempty"`
}

// Award is the optional member_award key of POST orders/manual (LP7).
type Award struct {
	MemberID string `json:"member_id"`
	Points   int64  `json:"points"`
}

// AwardOutcome is what the manual-order receipt and answer carry (ids, numbers, state only; LP18).
type AwardOutcome struct {
	MemberID string `json:"member_id"`
	Points   int64  `json:"points"`
	State    string `json:"state"`
	EntryID  string `json:"entry_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// ValidAward is the Go half of the award input rule.
func ValidAward(a Award) bool {
	return command.ValidID(a.MemberID) && a.Points >= 1 && a.Points <= MaxPoints
}

// Classify maps a loyalty error to its contract §2 status and code; ok is false for errors this package does not own.
func Classify(err error) (status int, code string, ok bool) {
	for _, refusal := range []struct {
		err    error
		status int
		code   string
	}{
		{ErrInsufficient, 409, "points_insufficient"},
		{ErrErased, 409, "member_erased"},
		{ErrLimit, 422, "limit_reached"},
		{ErrVersion, 409, "conflict"},
		{ErrExportTooLarge, 409, "export_too_large"},
		{ErrIdempotencyConflict, 409, "idempotency_conflict"},
		{ErrUnavailable, 503, "unavailable"},
	} {
		if errors.Is(err, refusal.err) {
			return refusal.status, refusal.code, true
		}
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		if len(pg.Code) == 5 && pg.Code[:2] == "PT" {
			return 0, "", false
		} // Other domains retain their own refusal mapping.
		// Retry only transient transaction failures. Never expose database messages:
		// CHECK failures are implementation defects and can contain member PII.
		switch pg.Code {
		case "55P03", "40P01", "40001", "57014":
			return 503, "retry_later", true
		default:
			return 503, "unavailable", true
		}
	}
	return 0, "", false
}
