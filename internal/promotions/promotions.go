// Package promotions owns merchant discount codes (storefront-v2 section F): the code rows, the quote-time check that turns a typed code into
// the frozen pricing.Promo effect, and the BeginCheckout redemption that counts usage atomically.
//
// It never computes money (internal/pricing CalculateWith is the only calculator; this package hands it a Promo and nothing else), never
// decides a price from a client value, never touches stock or payment state, and never releases usage by hand (usage is derived from order
// state in SQL, so expiry and cancellation need no hook). It calls no external host.
//
// Every database call is one SQL definer of migration 0091 (promotions.quote_check / redeem / admin_list / admin_create / admin_update) inside
// a caller-owned pgx.Tx: SQL owns every usage rule, permission and the idempotent receipt; Go validates shape and maps refusals.
package promotions

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/command"
	"livecommerce/internal/pricing"
)

var (
	codePattern  = regexp.MustCompile(`^[A-Z0-9-]{3,24}$`)
	refusalCode  = regexp.MustCompile(`^promo_[a-z_]{3,40}$`)
	contractCode = regexp.MustCompile(`^[a-z][a-z0-9_]{2,59}$`)
)

// Coded is a refusal with its HTTP status and contract code (internal/httperror table). Buyer refusals are 422 promo_*; merchant refusals
// are 409 version_changed / promo_exists / idempotency_conflict and 422 invalid_promotion.
type Coded struct {
	Status int
	Code   string
}

func (e *Coded) Error() string { return e.Code }

// Normalize trims and upper-cases a typed code. ok is false when it cannot be a code, which the caller reports as promo_invalid.
func Normalize(code string) (string, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	return code, codePattern.MatchString(code)
}

// mapError turns the definers' SQLSTATEs into domain errors. PT422 with a promo_* message is a buyer refusal; PT409/PT422 with a contract
// code is a merchant refusal; PT400 is invalid input; everything else is returned unchanged for the caller's classifier.
func mapError(err error) error {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		return err
	}
	switch pg.Code {
	case "PT400":
		return command.ErrInvalid
	case "PT422":
		if refusalCode.MatchString(pg.Message) || pg.Message == "invalid_promotion" {
			return &Coded{Status: http.StatusUnprocessableEntity, Code: pg.Message}
		}
		return command.ErrInvalid
	case "PT409":
		if contractCode.MatchString(pg.Message) {
			return &Coded{Status: http.StatusConflict, Code: pg.Message}
		}
		return command.ErrConflict
	case "PT404":
		return command.ErrNotFound
	}
	return err
}

// Check validates a typed code against the PRE-discount merchandise subtotal inside the buyer scoped transaction (tenant/store/owner come
// from buyer.Scope, never the request) and returns the effect to freeze in the quote. It is advisory: no lock, no write. BeginCheckout's
// Redeem is authoritative. An empty code is the caller's "no promotion" and never reaches here.
func Check(ctx context.Context, tx pgx.Tx, tenantID, storeID, ownerID, code string, subtotalMinor int64) (pricing.Promo, error) {
	if tx == nil || !command.ValidID(tenantID) || !command.ValidID(storeID) || !command.ValidID(ownerID) || subtotalMinor < 0 || subtotalMinor > command.MaxMoney {
		return pricing.Promo{}, command.ErrInvalid
	}
	normal, ok := Normalize(code)
	if !ok {
		return pricing.Promo{}, &Coded{Status: http.StatusUnprocessableEntity, Code: "promo_invalid"}
	}
	var raw []byte
	// promotions.quote_check (0091): verifies the buyer GUCs equal these arguments, then status/window/minimum/total limit/owner limit.
	if err := tx.QueryRow(ctx, `SELECT promotions.quote_check($1::uuid,$2::uuid,$3::uuid,$4,$5)`, tenantID, storeID, ownerID, normal, subtotalMinor).Scan(&raw); err != nil {
		return pricing.Promo{}, mapError(err)
	}
	var out pricing.Promo
	if err := json.Unmarshal(raw, &out); err != nil || !command.ValidID(out.ID) || out.Code != normal || out.Version < 1 || (out.Kind != "percent" && out.Kind != "fixed") {
		return pricing.Promo{}, command.ErrConflict // a malformed definer answer is a conflicting fact, never a price
	}
	return out, nil
}

// identityHash is the pseudonymous per-buyer limit identity: sha256(store | kind | value). The store id salts it so the same e-mail hashes
// differently in two stores; nil means "unknown" and never matches.
func identityHash(storeID, kind, value string) []byte {
	if value == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(storeID + "|" + kind + "|" + value))
	return sum[:]
}

// phoneDigits keeps only the digits of a phone number, so "0912-345-678" and "0912345678" are one buyer.
func phoneDigits(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) && r < unicode.MaxASCII {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Redeem is BeginCheckout's authoritative step, after checkout.begin_hold in the same transaction. SQL reads the code id, version and
// discount from the order's own stored snapshot; email/phone only feed the per-buyer limit (hashed here, never sent in clear). Any
// refusal is a PT422 promo_* that rolls the placement back; promotions.redeem locks the code row, so concurrent placements cannot
// oversubscribe a limit.
func Redeem(ctx context.Context, tx pgx.Tx, tokenHash []byte, storeID, orderID, buyerEmail, phone string) error {
	if tx == nil || len(tokenHash) != 32 || !command.ValidID(storeID) || !command.ValidID(orderID) {
		return command.ErrInvalid
	}
	email := identityHash(storeID, "email", strings.ToLower(strings.TrimSpace(buyerEmail)))
	digits := phoneDigits(phone)
	if len(digits) < 6 { // not a phone: never let a placeholder collide two buyers
		digits = ""
	}
	// promotions.redeem (0091): code row FOR UPDATE, version + usage rules, redemption insert; the only writer of promotions.redemptions.
	_, err := tx.Exec(ctx, `SELECT promotions.redeem($1,$2::uuid,$3::uuid,$4,$5)`, tokenHash, storeID, orderID, email, identityHash(storeID, "phone", digits))
	return mapError(err)
}
