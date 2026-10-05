// Purpose: A5-2 "one-click copy previous session" aggregate command: creates a new session+program under the live.session.copy receipt, then seeds its claim configuration (a CLOSED generation-0 window in the source's match mode and every ACTIVE source offer WITH its live_price_minor) in the same transaction. The source version is a CAS: the merchant must pass the version they read.
// Depends on: draft.go (authorize/mapError/scheduledTime/managePermission), command.Run/Audit, platform, internal/claims CopyContent, live.sessions, live.programs.
// Used by: internal/httpapi/live_flow.go POST /live-sessions/{session_id}/copy.
package live

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// CopyInput is the exact A5-2 request body: title, scheduled_at (nullable, like DraftInput) and the
// expected version of the SOURCE session (the CAS guard).
type CopyInput struct {
	Title           string     `json:"title"`
	ScheduledAt     *time.Time `json:"scheduled_at"`
	ExpectedVersion int64      `json:"expected_version"`
}

// CopyResult is the A5-2 response envelope: the new session draft, its seeded window, the offers
// created (empty array, never null), any import conflicts, and the source version it was copied from.
type CopyResult struct {
	Session       Draft                   `json:"session"`
	Window        claims.Window           `json:"window"`
	Created       []claims.Offer          `json:"created"`
	Conflicts     []claims.ImportConflict `json:"conflicts"`
	SourceVersion int64                   `json:"source_version"`
}

// CopySession copies the source session's draft shape (aspect ratio from its program) and its claim
// configuration onto a brand-new session. The whole operation is one transaction under one receipt;
// the FOR SHARE lock on the source session is the single order every offer mutation serializes
// behind, so the copied offers and live prices are a consistent snapshot.
func CopySession(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, sourceID string, in CopyInput) (CopyResult, error) {
	if !command.ValidID(sourceID) || in.ExpectedVersion < 1 {
		return CopyResult{}, command.ErrInvalid
	}
	var err error
	in, err = canonicalCopyInput(in)
	if err != nil {
		return CopyResult{}, err
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return CopyResult{}, err
	}
	request := struct {
		PrincipalID     string `json:"principal_id"`
		SourceSessionID string `json:"source_session_id"`
		CopyInput
	}{scope.PrincipalID, sourceID, in}
	var out CopyResult
	err = command.Run(ctx, tx, scope, "live.session.copy", key, request, &out, func() error {
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		// LOCK the source session FOR SHARE before reading its version: a concurrent UpdateDraft or
		// claim mutation holds FOR UPDATE and blocks here, so the version CAS below and the offer copy
		// in claims.CopyContent both see one committed snapshot.
		var sourceVersion int64
		if err := tx.QueryRow(ctx, `SELECT version FROM live.sessions
			WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR SHARE`,
			scope.TenantID, scope.StoreID, sourceID).Scan(&sourceVersion); err != nil {
			return mapError(err)
		}
		if sourceVersion != in.ExpectedVersion {
			return command.ErrConflict
		}
		var aspectRatio string
		if err := tx.QueryRow(ctx, `SELECT aspect_ratio FROM live.programs
			WHERE tenant_id=$1 AND store_id=$2 AND session_id=$3`,
			scope.TenantID, scope.StoreID, sourceID).Scan(&aspectRatio); err != nil {
			return mapError(err)
		}
		var scheduled pgtype.Timestamptz
		err := tx.QueryRow(ctx, `INSERT INTO live.sessions(tenant_id,store_id,principal_id,title,scheduled_at)
			VALUES($1,$2,$3,$4,$5) RETURNING id::text,title,scheduled_at,version,created_at,updated_at`,
			scope.TenantID, scope.StoreID, scope.PrincipalID, in.Title, in.ScheduledAt).
			Scan(&out.Session.ID, &out.Session.Title, &scheduled, &out.Session.Version, &out.Session.CreatedAt, &out.Session.UpdatedAt)
		if err != nil {
			return mapError(err)
		}
		out.Session.ScheduledAt = scheduledTime(scheduled)
		err = tx.QueryRow(ctx, `INSERT INTO live.programs(tenant_id,store_id,session_id,principal_id,aspect_ratio)
			VALUES($1,$2,$3,$4,$5) RETURNING id::text,aspect_ratio,state`,
			scope.TenantID, scope.StoreID, out.Session.ID, scope.PrincipalID, aspectRatio).
			Scan(&out.Session.ProgramID, &out.Session.AspectRatio, &out.Session.State)
		if err != nil {
			return mapError(err)
		}
		window, created, conflicts, err := claims.CopyContent(ctx, tx, scope, sourceID, out.Session.ID)
		if err != nil {
			return err
		}
		out.Window, out.Created, out.Conflicts = window, created, conflicts
		out.SourceVersion = sourceVersion
		return command.Audit(ctx, tx, scope, "live.session.copied")
	})
	if err != nil {
		return CopyResult{}, mapError(err)
	}
	// A replay remains an authenticated request even after its lock wait.
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return CopyResult{}, err
	}
	return out, nil
}

// canonicalCopyInput applies the same title/scheduled_at rules as draft.go canonicalInput, minus the
// aspect ratio (the copy inherits the source program's ratio).
func canonicalCopyInput(in CopyInput) (CopyInput, error) {
	if !utf8.ValidString(in.Title) || strings.TrimSpace(in.Title) != in.Title ||
		utf8.RuneCountInString(in.Title) < 1 || utf8.RuneCountInString(in.Title) > 200 {
		return CopyInput{}, command.ErrInvalid
	}
	for _, r := range in.Title {
		if unicode.IsControl(r) {
			return CopyInput{}, command.ErrInvalid
		}
	}
	if in.ScheduledAt != nil {
		value := in.ScheduledAt.UTC().Truncate(time.Microsecond)
		if value.Year() < 2000 || value.Year() > 2199 {
			return CopyInput{}, command.ErrInvalid
		}
		in.ScheduledAt = &value
	}
	return in, nil
}
