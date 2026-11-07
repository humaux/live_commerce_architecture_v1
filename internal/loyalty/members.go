// Purpose: member commands, current-row receipt projections and reads (LP1-LP19).
// Depends on: 0223 loyalty definers, command.Run receipts, platform authority fences.
// Used by: member HTTP routes and manual-order final transaction; REAL_PG gates.
// Invariants: receipts contain ids/numbers only; no ownership or points rule in HTTP.
package loyalty

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/text/unicode/norm"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func text(s string, min, max int) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) < min || utf8.RuneCountInString(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}
func optional(s *string, max int) bool { return s == nil || text(*s, 0, max) }
func memberInput(in MemberInput) (MemberInput, error) {
	in.DisplayName = norm.NFC.String(strings.TrimSpace(in.DisplayName))
	if !text(in.DisplayName, 1, 60) || !optional(in.ContactNote, 120) || !optional(in.Notes, 1000) {
		return in, command.ErrInvalid
	}
	return in, nil
}
func dbError(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT400", "PT422":
			if pg.Message == "limit_reached" {
				return ErrLimit
			}
			return command.ErrInvalid
		case "PT401":
			return platform.ErrUnauthorized
		case "PT403":
			return platform.PT403(pg.Message, platform.ErrForbidden)
		case "PT404":
			return platform.ErrScopeNotFound
		case "PT409":
			switch pg.Message {
			case "points_insufficient":
				return ErrInsufficient
			case "member_erased":
				return ErrErased
			case "limit_reached":
				return ErrLimit
			case "export_too_large":
				return ErrExportTooLarge
			default:
				return ErrVersion
			}
		case "55P03", "40P01", "40001", "57014":
			return err
		default:
			return fmt.Errorf("%w (SQLSTATE %s)", ErrUnavailable, pg.Code)
		}
	}
	return err
}

// call is the module's parameterized JSON definer boundary; caller retains the transaction.
func call(ctx context.Context, tx pgx.Tx, s platform.Scope, token, permission, sql string, out any, args ...any) error {
	if tx == nil {
		return command.ErrInvalid
	}
	if e := platform.RequirePermission(ctx, tx, s, token, permission); e != nil {
		return dbError(e)
	}
	h := sha256.Sum256([]byte(token))
	var raw []byte
	if e := tx.QueryRow(ctx, sql, append([]any{h[:], s.StoreID}, args...)...).Scan(&raw); e != nil {
		return dbError(e)
	}
	if e := platform.RequirePermission(ctx, tx, s, token, permission); e != nil {
		return dbError(e)
	}
	if e := json.Unmarshal(raw, out); e != nil {
		return ErrUnavailable
	}
	return nil
}
func write(ctx context.Context, tx pgx.Tx, s platform.Scope, token, key, op, permission string, request, out any, sql string, args ...any) (bool, error) {
	if tx == nil {
		return false, command.ErrInvalid
	}
	if e := platform.RequirePermission(ctx, tx, s, token, permission); e != nil {
		return false, dbError(e)
	}
	replay := true
	e := command.Run(ctx, tx, s, op, key, request, out, func() error { replay = false; return call(ctx, tx, s, token, permission, sql, out, args...) })
	if errors.Is(e, command.ErrConflict) {
		e = ErrIdempotencyConflict
	}
	if e != nil {
		return false, dbError(e)
	}
	return replay, dbError(platform.RequirePermission(ctx, tx, s, token, permission))
}

type memberReceipt struct {
	ID      string `json:"member_id"`
	Version int64  `json:"version"`
}

func readMember(ctx context.Context, tx pgx.Tx, s platform.Scope, token, id, cursor, permission string) (Detail, error) {
	var out Detail
	e := call(ctx, tx, s, token, permission, `SELECT loyalty.read_member($1,$2::uuid,$3::uuid,$4,$5)`, &out, id, cursor, permission)
	return out, e
}
func CreateMember(ctx context.Context, tx pgx.Tx, s platform.Scope, token, key string, in MemberInput) (Member, bool, error) {
	in, e := memberInput(in)
	if e != nil {
		return Member{}, false, e
	}
	var receipt memberReceipt
	replay, e := write(ctx, tx, s, token, key, "loyalty.member.create", "customers:points", in, &receipt, `SELECT loyalty.create_member($1,$2::uuid,$3,$4,$5)`, in.DisplayName, in.ContactNote, in.Notes)
	if e != nil {
		return Member{}, false, e
	}
	d, e := readMember(ctx, tx, s, token, receipt.ID, "", "customers:points")
	return d.Member, replay, e
}
func UpdateMember(ctx context.Context, tx pgx.Tx, s platform.Scope, token, key, id string, in MemberPatch) (Member, bool, error) {
	if !command.ValidID(id) || in.ExpectedVersion < 1 || len(in.Fields) == 0 {
		return Member{}, false, command.ErrInvalid
	}
	in.Fields = append([]string{}, in.Fields...)
	sort.Strings(in.Fields)
	for i, k := range in.Fields {
		if (i > 0 && k == in.Fields[i-1]) || (k != "display_name" && k != "contact_note" && k != "notes") {
			return Member{}, false, command.ErrInvalid
		}
		if k == "display_name" {
			if in.DisplayName == nil {
				return Member{}, false, command.ErrInvalid
			}
			v := norm.NFC.String(strings.TrimSpace(*in.DisplayName))
			in.DisplayName = &v
			if !text(v, 1, 60) {
				return Member{}, false, command.ErrInvalid
			}
		}
	}
	if !optional(in.ContactNote, 120) || !optional(in.Notes, 1000) {
		return Member{}, false, command.ErrInvalid
	}
	var receipt memberReceipt
	request := struct {
		MemberID string      `json:"member_id"`
		Patch    MemberPatch `json:"patch"`
	}{id, in}
	replay, e := write(ctx, tx, s, token, key, "loyalty.member.update", "customers:points", request, &receipt, `SELECT loyalty.update_member($1,$2::uuid,$3::uuid,$4,$5::text[],$6,$7,$8)`, id, in.ExpectedVersion, in.Fields, in.DisplayName, in.ContactNote, in.Notes)
	if e != nil {
		return Member{}, false, e
	}
	d, e := readMember(ctx, tx, s, token, id, "", "customers:points")
	return d.Member, replay, e
}
func Adjust(ctx context.Context, tx pgx.Tx, s platform.Scope, token, key, id string, in AdjustInput) (AdjustResult, bool, error) {
	if !command.ValidID(id) || in.Delta == 0 || in.Delta < -MaxPoints || in.Delta > MaxPoints || !text(in.Note, 1, 200) || strings.TrimSpace(in.Note) == "" || (in.OrderID != nil && !command.ValidID(*in.OrderID)) {
		return AdjustResult{}, false, command.ErrInvalid
	}
	var receipt struct {
		EntryID string `json:"entry_id"`
		Balance int64  `json:"balance"`
	}
	request := struct {
		MemberID string      `json:"member_id"`
		Input    AdjustInput `json:"input"`
	}{id, in}
	replay, e := write(ctx, tx, s, token, key, "loyalty.points.adjust", "customers:points", request, &receipt, `SELECT loyalty.adjust_points($1,$2::uuid,$3::uuid,$4,$5,$6::uuid)`, id, in.Delta, in.Note, in.OrderID)
	if e != nil {
		return AdjustResult{}, false, e
	}
	var entry Entry
	e = call(ctx, tx, s, token, "customers:points", `SELECT loyalty.read_entry($1,$2::uuid,$3::uuid)`, &entry, receipt.EntryID)
	return AdjustResult{entry, receipt.Balance}, replay, e
}
func Erase(ctx context.Context, tx pgx.Tx, s platform.Scope, token, key, id string) (Erasure, bool, error) {
	if !command.ValidID(id) {
		return Erasure{}, false, command.ErrInvalid
	}
	var out Erasure
	replay, e := write(ctx, tx, s, token, key, "loyalty.member.erase", "customers:privacy", map[string]string{"member_id": id}, &out, `SELECT loyalty.erase_member($1,$2::uuid,$3::uuid)`, id)
	return out, replay, e
}
func Export(ctx context.Context, tx pgx.Tx, s platform.Scope, token, key, id string) ([]byte, bool, error) {
	if !command.ValidID(id) {
		return nil, false, command.ErrInvalid
	}
	var out json.RawMessage
	// Read before recording a receipt so the size refusal leaves no audit or receipt.
	e := call(ctx, tx, s, token, "customers:privacy", `SELECT loyalty.export_member($1,$2::uuid,$3::uuid)`, &out, id)
	if e != nil {
		return nil, false, e
	}
	if len(out) > MaxExportBytes {
		return nil, false, ErrExportTooLarge
	}
	var receipt memberReceipt
	// Export's definer is read-only; the audit is emitted only on a fresh receipt.
	replay := true
	e = command.Run(ctx, tx, s, "loyalty.member.export", key, map[string]string{"member_id": id}, &receipt, func() error {
		replay = false
		receipt.ID = id
		return command.AuditDetails(ctx, tx, s, "member.exported", map[string]any{"member_id": id})
	})
	if errors.Is(e, command.ErrConflict) {
		e = ErrIdempotencyConflict
	}
	return out, replay, dbError(e)
}
func Search(ctx context.Context, tx pgx.Tx, s platform.Scope, token string, in SearchInput) (SearchResult, error) {
	if in.Limit == 0 {
		in.Limit = 20
	}
	if in.Limit < 1 || in.Limit > 100 || (in.Q != nil && !text(*in.Q, 1, 60)) || (in.Cursor != nil && len(*in.Cursor) > 1024) {
		return SearchResult{}, command.ErrInvalid
	}
	var out SearchResult
	e := call(ctx, tx, s, token, "customers:read", `SELECT loyalty.read_members($1,$2::uuid,$3,$4,$5,$6)`, &out, in.Q, in.IncludeErased, in.Cursor, in.Limit)
	return out, e
}
func Get(ctx context.Context, tx pgx.Tx, s platform.Scope, token, id, cursor string) (Detail, error) {
	if !command.ValidID(id) || len(cursor) > 1024 {
		return Detail{}, command.ErrInvalid
	}
	return readMember(ctx, tx, s, token, id, cursor, "customers:read")
}
func EntriesForOrder(ctx context.Context, tx pgx.Tx, s platform.Scope, token, id string) (OrderEntries, error) {
	if !command.ValidID(id) {
		return OrderEntries{}, command.ErrInvalid
	}
	var out OrderEntries
	e := call(ctx, tx, s, token, "customers:read", `SELECT loyalty.read_order_entries($1,$2::uuid,$3::uuid)`, &out, id)
	return out, e
}
func PrecheckAward(ctx context.Context, tx pgx.Tx, s platform.Scope, token string, a Award) error {
	if !ValidAward(a) {
		return command.ErrInvalid
	}
	var out any
	return call(ctx, tx, s, token, "customers:points", `SELECT loyalty.precheck_award($1,$2::uuid,$3::uuid,$4)`, &out, a.MemberID, a.Points)
}
func TryAward(ctx context.Context, tx pgx.Tx, s platform.Scope, token, order string, a Award) (AwardOutcome, error) {
	if tx == nil || !ValidAward(a) || !command.ValidID(order) {
		return AwardOutcome{}, command.ErrInvalid
	}
	// LP18 deliberately bypasses the raising Go permission fence: SQL records a revoked right as not_awarded.
	h := sha256.Sum256([]byte(token))
	var raw []byte
	var out AwardOutcome
	e := tx.QueryRow(ctx, `SELECT loyalty.try_award_manual_order($1,$2::uuid,$3::uuid,$4::uuid,$5)`, h[:], s.StoreID, order, a.MemberID, a.Points).Scan(&raw)
	if e != nil {
		return out, dbError(e)
	}
	if json.Unmarshal(raw, &out) != nil {
		return out, ErrUnavailable
	}
	return out, nil
}
