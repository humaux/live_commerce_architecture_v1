// Purpose: the typed view of one per-store settlement statement (contracts/stripe-platform-account-v1.md §6) and the merchant read of
//   it. Read is one commerce_runtime transaction around payments.read_store_settlements; the SQL decides every rule (billing:manage,
//   the store from the token, no settlement-currency fields, no other store's rows, no unattributed rows, no Stripe ids).
// Depends on: SQL payments.read_store_settlements (0150; registry-writer definer, EXECUTE commerce_runtime only); internal/platform
//   (scope transaction), internal/command (id check), pgx.
// Used by: internal/httpapi/settlements.go (merchant GET), internal/payments/stripeadmin/settlement.go (operator statement read),
//   export.go (the CSV of the same Statement).
// Invariants: I01 (store from the token); a statement is read-only here; no money is ever computed in Go (SQL owns the arithmetic).
// Status: MOCK + REAL_PG; no Stripe call is made here.

package settlement

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// ErrUnavailable is any unclassified failure (driver error, malformed projection): a retryable 503, never a message.
var ErrUnavailable = errors.New("settlement: unavailable")

// Error is a coded refusal of the SQL definer (a PT409 message token). Status is the HTTP status the route returns.
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return e.Code }

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,59}$`)

// Line is one ledger line as a merchant sees it: order number, kind, amounts in the store currency and the Taipei date. No Stripe id.
type Line struct {
	OrderNumber   string `json:"order_number"`
	Kind          string `json:"kind"`
	StoreMinor    int64  `json:"store_minor"`
	FeeStoreMinor int64  `json:"fee_store_minor"`
	TxnDate       string `json:"txn_date"`
}

// Statement is one closed period of one store (store currency only). Lines is empty on the list route.
type Statement struct {
	StatementID      string  `json:"statement_id"`
	PeriodStart      string  `json:"period_start"`
	PeriodEnd        string  `json:"period_end"`
	Currency         string  `json:"currency"`
	CapturedMinor    int64   `json:"captured_minor"`
	RefundedMinor    int64   `json:"refunded_minor"`
	DisputeMinor     int64   `json:"dispute_minor"`
	StripeFeeMinor   int64   `json:"stripe_fee_minor"`
	PlatformFeeBPS   int     `json:"platform_fee_bps"`
	PlatformFeeMinor int64   `json:"platform_fee_minor"`
	CarriedInMinor   int64   `json:"carried_in_minor"`
	NetPayableMinor  int64   `json:"net_payable_minor"`
	LineCount        int     `json:"line_count"`
	ClosedAt         string  `json:"closed_at"`
	Paid             bool    `json:"paid"`
	PayoutRef        *string `json:"payout_ref,omitempty"`
	PayoutMinor      *int64  `json:"payout_minor,omitempty"`
	PaidAt           *string `json:"paid_at,omitempty"`
	Lines            []Line  `json:"lines,omitempty"`
}

// List is the list route body.
type List struct {
	Statements []Statement `json:"statements"`
}

// Detail is the single-statement route body.
type Detail struct {
	Statement Statement `json:"statement"`
}

// Read returns the store's statements (statement == "": newest first, period_start < before when before != "", at most limit <= 52,
// no lines) or one statement with its lines. SQL payments.read_store_settlements; read-only.
func Read(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, limit int, before, statement string) (any, error) {
	hash, ok := tokenHash(token)
	if tx == nil || !ok || !command.ValidID(scope.StoreID) || limit < 1 || limit > 52 || (statement != "" && !command.ValidID(statement)) {
		return nil, command.ErrInvalid
	}
	var beforeArg, statementArg any
	if before != "" {
		if _, err := time.Parse("2006-01-02", before); err != nil {
			return nil, command.ErrInvalid
		}
		beforeArg = before
	}
	if statement != "" {
		statementArg = statement
	}
	var raw []byte
	// payments.read_store_settlements (0150): billing:manage on the token's store, statements of that store only, final authority fence.
	if err := tx.QueryRow(ctx, `SELECT payments.read_store_settlements($1,$2::uuid,$3,$4::date,$5::uuid)`,
		hash, scope.StoreID, limit, beforeArg, statementArg).Scan(&raw); err != nil {
		return nil, mapError(err)
	}
	if statement != "" {
		var out Detail
		if err := strictDecode(raw, &out); err != nil {
			return nil, ErrUnavailable
		}
		return out, nil
	}
	var out List
	if err := strictDecode(raw, &out); err != nil {
		return nil, ErrUnavailable
	}
	if out.Statements == nil {
		out.Statements = []Statement{}
	}
	return out, nil
}

// DecodeStatement strictly decodes one statement as the operator read returns it (with lines).
func DecodeStatement(raw []byte) (Statement, error) {
	var st Statement
	if err := strictDecode(raw, &st); err != nil || st.StatementID == "" {
		return Statement{}, ErrUnavailable
	}
	return st, nil
}

// mapError turns the definer's SQLSTATEs into coded refusals: PT400/22023 invalid input, PT401/PT404 authority, PT403 forbidden, PT409 a
// coded conflict (the message is one of our own tokens).
func mapError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400", "22023":
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT403":
			return platform.ErrForbidden
		case "PT409":
			if codePattern.MatchString(pg.Message) {
				return &Error{Status: http.StatusConflict, Code: pg.Message}
			}
			return command.ErrConflict
		}
	}
	if errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) || errors.Is(err, platform.ErrScopeNotFound) {
		return err
	}
	return ErrUnavailable
}

func strictDecode(raw []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}

// tokenHash is the authority hash the SQL definers expect (sha256 of the bearer token, 32 bytes).
func tokenHash(token string) ([]byte, bool) {
	if len(token) < 32 || len(token) > 512 {
		return nil, false
	}
	h := sha256.Sum256([]byte(token))
	return h[:], true
}
