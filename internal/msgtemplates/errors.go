// Purpose: the fixed transport-safe error vocabulary of the template domain. Only fixed codes leave the process:
// a *Error carries a stable status + code (never a driver message), databaseError passes the fixed definer PT codes
// through and flattens every other database failure to one sentinel.
// Depends on: internal/command (ErrNotFound, ErrInvalid, ErrConflict), pgx pgconn (definer SQLSTATE codes).
// Used by: publish.go/list.go/resolve.go/publicsafe.go, internal/httpapi/templates.go (templatesClassify).

package msgtemplates

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
)

// ErrDatabase is the fixed wrap for any definer/database failure that is not a fixed PT code; it never carries a driver message.
var ErrDatabase = errors.New("msgtemplates: database unavailable")

// Error is a fixed transport-safe refusal. Code is the HTTP error code; Status is the HTTP status.
type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string { return e.Code }

// PublicUnsafe is the §3.5 refusal — a public-safe template body that failed the content rule (§11: 422
// public_reply_forbidden_content). The specific reason stays internal to ValidatePublicSafe.
func PublicUnsafe() error {
	return &Error{http.StatusUnprocessableEntity, "public_reply_forbidden_content"}
}

// IsNotFound reports the fixed not-found class (a resolve that matched no fixed or merchant version).
func IsNotFound(err error) bool { return errors.Is(err, command.ErrNotFound) }

// databaseError passes the fixed definer codes through unchanged (the httpapi classifier maps them) and flattens
// every other database failure to ErrDatabase.
func databaseError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT403", "PT404", "PT409", "PT422":
			return err
		}
	}
	return ErrDatabase
}
