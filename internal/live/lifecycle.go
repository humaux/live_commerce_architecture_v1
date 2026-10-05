// Purpose: A7 session lifecycle (live-console-v1 §9): start/end/archive of a live session with a lifecycle_version CAS, one receipt (live.session.lifecycle) and audit rows, driving the session's claim window; plus the append-only live.offer_timeline writer (§7.3) that the recommend endpoint (LC-B4/B7) calls.
// Depends on: live.sessions (lifecycle, lifecycle_version, lifecycle_at; migration 0122), live.offer_timeline (0122), claims.ApplyWindow/ReadWindow (internal/claims/merchant.go: window transition, 5-window store cap, billing guard 0079), command.Run/Audit, authorize in draft.go.
// Used by: internal/httpapi/live_lifecycle.go (POST /live-sessions/{session_id}/lifecycle); RecordOfferFeatured by the recommend route of a later unit.
// Invariants: lock order session row (FOR NO KEY UPDATE) -> window row -> store cap advisory lock, so a transition, UpdateDraft, SetWindow and ingest cannot deadlock (§5.6). A transition never touches live.sessions.version/updated_at (0035 frozen_media_session refuses them once a media attempt exists).
package live

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

// Session lifecycle states (contract §9).
const (
	LifecycleDraft    = "draft"
	LifecycleLive     = "live"
	LifecycleEnded    = "ended"
	LifecycleArchived = "archived"
)

// LifecycleInput is the exact A7 body. OpenWindow nil means true (start also opens the claim window).
type LifecycleInput struct {
	Action          string `json:"action"`
	ExpectedVersion int64  `json:"expected_version"`
	OpenWindow      *bool  `json:"open_window,omitempty"`
}

// LifecycleResult is the A7 response: the new lifecycle, its CAS version and the session's claim window.
type LifecycleResult struct {
	Lifecycle string        `json:"lifecycle"`
	Version   int64         `json:"version"`
	Window    claims.Window `json:"window"`
}

// planLifecycle is the pure §9 transition table: start from draft or ended, end from live, archive from ended only.
// It returns the next lifecycle and the audit action (live.session.<audit>).
func planLifecycle(current, action string) (next, audit string, err error) {
	switch {
	case action == "start" && (current == LifecycleDraft || current == LifecycleEnded):
		return LifecycleLive, "started", nil
	case action == "end" && current == LifecycleLive:
		return LifecycleEnded, "ended", nil
	case action == "archive" && current == LifecycleEnded:
		return LifecycleArchived, "archived", nil
	case action == "start" || action == "end" || action == "archive":
		return "", "", claims.ErrInvalidTransition
	}
	return "", "", command.ErrInvalid
}

// Lifecycle applies one A7 transition (live:manage). Side effects, all in the caller's transaction under one receipt:
// UPDATE live.sessions lifecycle columns; start opens the claim window unless open_window=false (new generation,
// 5-window cap, billing guard); end closes an OPEN window; audit rows live.session.<started|ended|archived> and
// live.claim.window.<opened|closed>. Errors: command.ErrConflict (stale expected_version), claims.ErrInvalidTransition,
// claims.ErrTooManyOpenWindows, claims.ErrBillingRestricted, command.ErrNotFound, command.ErrInvalid.
func Lifecycle(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key, sessionID string, in LifecycleInput) (LifecycleResult, error) {
	if !command.ValidID(sessionID) || in.ExpectedVersion < 1 {
		return LifecycleResult{}, command.ErrInvalid
	}
	if in.Action != "start" && in.Action != "end" && in.Action != "archive" {
		return LifecycleResult{}, command.ErrInvalid // unknown action: 422 before any lock
	}
	openWindow := in.OpenWindow == nil || *in.OpenWindow
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return LifecycleResult{}, err
	}
	request := struct {
		PrincipalID     string `json:"principal_id"`
		SessionID       string `json:"session_id"`
		Action          string `json:"action"`
		ExpectedVersion int64  `json:"expected_version"`
		OpenWindow      bool   `json:"open_window"`
	}{scope.PrincipalID, sessionID, in.Action, in.ExpectedVersion, openWindow}
	var out LifecycleResult
	err := command.Run(ctx, tx, scope, "live.session.lifecycle", key, request, &out, func() error {
		var current string
		var version int64
		// LOCK: session row first, NO KEY UPDATE so FK checks of offers/windows (KEY SHARE) are not blocked.
		if err := tx.QueryRow(ctx, `SELECT lifecycle,lifecycle_version FROM live.sessions
			WHERE tenant_id=$1 AND store_id=$2 AND id=$3 FOR NO KEY UPDATE`,
			scope.TenantID, scope.StoreID, sessionID).Scan(&current, &version); err != nil {
			return mapError(err)
		}
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		if version != in.ExpectedVersion {
			return command.ErrConflict
		}
		next, audit, err := planLifecycle(current, in.Action)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `UPDATE live.sessions SET lifecycle=$4,lifecycle_version=lifecycle_version+1,lifecycle_at=clock_timestamp()
			WHERE tenant_id=$1 AND store_id=$2 AND id=$3 AND lifecycle_version=$5 RETURNING lifecycle,lifecycle_version`,
			scope.TenantID, scope.StoreID, sessionID, next, version).Scan(&out.Lifecycle, &out.Version); err != nil {
			return mapError(err)
		}
		// The window follows the (already updated) lifecycle: ApplyWindow refuses to open an ended/archived session,
		// so start must run after the UPDATE above. claims.ApplyWindow = §8 cap + billing guard (0079) + generation bump.
		var action string
		switch {
		case in.Action == "start" && openWindow:
			out.Window, action, err = claims.ApplyWindow(ctx, tx, scope, sessionID, claims.WindowOpen)
		case in.Action == "end":
			out.Window, action, err = claims.ApplyWindow(ctx, tx, scope, sessionID, claims.WindowClosed)
		default:
			out.Window, err = claims.ReadWindow(ctx, tx, scope, sessionID)
		}
		if err != nil {
			return err
		}
		if err := command.Audit(ctx, tx, scope, "live.session."+audit); err != nil {
			return err
		}
		if action != "" {
			return command.Audit(ctx, tx, scope, "live.claim.window."+action)
		}
		return nil
	})
	if err != nil {
		return LifecycleResult{}, mapError(err)
	}
	// A replay remains an authenticated request even after its lock wait.
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return LifecycleResult{}, err
	}
	return out, nil
}

// RecordOfferFeatured appends one 'featured' event to live.offer_timeline for an offer of the session (live:manage) and
// returns its time. The FK (session, offer) makes a foreign-session or unknown offer command.ErrNotFound; the table has
// no UPDATE/DELETE grant, so history is never rewritten. Called by the recommend route (§7.3) in its own transaction.
func RecordOfferFeatured(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID, offerID string) (time.Time, error) {
	if !command.ValidID(sessionID) || !command.ValidID(offerID) {
		return time.Time{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return time.Time{}, err
	}
	var at time.Time
	err := tx.QueryRow(ctx, `INSERT INTO live.offer_timeline(tenant_id,store_id,session_id,offer_id,kind,principal_id)
		VALUES($1,$2,$3,$4,'featured',$5) RETURNING at`,
		scope.TenantID, scope.StoreID, sessionID, offerID, scope.PrincipalID).Scan(&at)
	if err != nil {
		return time.Time{}, mapError(err)
	}
	return at, nil
}
