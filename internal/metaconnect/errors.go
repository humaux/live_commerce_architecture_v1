package metaconnect

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// errors.go maps every refusal (local validation, SQL definer, driver) to one frozen error code so the HTTP layer never
// returns a driver message. It touches no table.

// Refusal is one frozen domain refusal: an HTTP status and the error code the body carries.
type Refusal struct {
	Status int
	Code   string
}

func (r *Refusal) Error() string { return "meta connect refusal: " + r.Code }

// ErrConnectFailed is every Meta-side failure of the callback / pick (exchange, listing, subscribe): the HTTP layer answers
// 502 meta_connect_failed without echoing a cause (a Graph body or URL can carry identifiers).
var ErrConnectFailed = errors.New("meta connect failed")

// frozenStatus is the closed list of codes the SQL raises (MCnnn, nnn = status) or this package returns.
var frozenStatus = map[string]int{
	"unauthorized": http.StatusUnauthorized, "forbidden": http.StatusForbidden, "not_found": http.StatusNotFound,
	"invalid_request": http.StatusUnprocessableEntity, "not_in_pick_list": http.StatusUnprocessableEntity,
	"missing_permission": http.StatusUnprocessableEntity,
	"state_mismatch":     http.StatusConflict, "state_used": http.StatusConflict, "state_expired": http.StatusGone,
	"cap_exceeded": http.StatusConflict, "page_taken": http.StatusConflict, "binding_disabled": http.StatusConflict,
	"conflict": http.StatusConflict, "recheck_too_soon": http.StatusTooManyRequests,
}

func refusal(code string) *Refusal {
	status, ok := frozenStatus[code]
	if !ok {
		status, code = http.StatusConflict, "conflict"
	}
	return &Refusal{Status: status, Code: code}
}

// mapError converts a definer/driver error into a Refusal, a platform/command sentinel, or leaves an unknown error untouched
// for the caller's 503 classification. SQLSTATE MCnnn carries MESSAGE = a frozen code; anything else becomes "conflict".
func mapError(err error) error {
	if err == nil {
		return nil
	}
	var refused *Refusal
	if errors.As(err, &refused) || errors.Is(err, command.ErrInvalid) || errors.Is(err, command.ErrConflict) ||
		errors.Is(err, command.ErrNotFound) || errors.Is(err, platform.ErrUnauthorized) || errors.Is(err, platform.ErrForbidden) ||
		errors.Is(err, platform.ErrScopeNotFound) || errors.Is(err, ErrConnectFailed) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if strings.HasPrefix(pgErr.Code, "MC") && len(pgErr.Code) == 5 {
		if _, numeric := strconv.Atoi(pgErr.Code[2:]); numeric == nil {
			return refusal(pgErr.Message)
		}
	}
	switch pgErr.Code {
	case "40001", "23505", "PT409":
		return command.ErrConflict
	case "23503", "P0002":
		return command.ErrNotFound
	case "22001", "22007", "22008", "22023", "22P02", "23514":
		return command.ErrInvalid
	}
	return err
}
