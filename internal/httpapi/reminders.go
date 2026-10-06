// Purpose: the W3-03B checkout-reminder HTTP adapter: POST /live-sessions/{sid}/reminders[/{bundle_id}] (merchant triggers one reminder pass over every eligible buyer, or for one buyer),
// GET /live-sessions/{sid}/reminders (sent / queued / failed / follow-up list) and GET/PUT /live-settings/reminder (the per-store automatic
// reminder setting). It decodes strict bodies, runs the inbox.Service methods in one scoped merchant transaction and maps refusals through
// inboxSendClassify; it decides no rule (the 0131 definers do), never returns a driver message and never logs a body or token.
// Depends on: internal/inbox (reminders.go), internal/httpapi inboxSendScoped/claimsRoute/claimsBody, internal/platform (WithScope).
// Used by: internal/httpapi/handler.go (registerReminderRoutes, gated on Options.Inbox with the send side enabled); cmd/api wires it.
// Invariants: permissions inbox:reply (trigger) / inbox:read (report) / live:read + live:manage (settings); cross-store -> 404 by the definers.

package httpapi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/inbox"
	"livecommerce/internal/platform"
)

// registerReminderRoutes mounts the four reminder routes. Nothing is mounted unless the inbox service has the send side enabled (a reminder
// is a DM: without the seal ring and the job client it could never be sent).
func registerReminderRoutes(mux *http.ServeMux, pool *pgxpool.Pool, svc *inbox.Service) {
	if svc == nil || !svc.SendEnabled() {
		return
	}
	const sessionPath = "/v1/admin/stores/{store_id}/live-sessions/{session_id}/reminders"
	const settingsPath = "/v1/admin/stores/{store_id}/live-settings/reminder"

	// Trigger: one pass over the session's unpaid buyers; Idempotency-Key required, no body.
	mux.HandleFunc("POST "+sessionPath, claimsRoute(http.MethodPost, false, inboxSendScoped(pool, "inbox:reply",
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.PlanCheckoutReminders(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), "")
		})))
	// Single buyer: the same pass restricted to one bundle (「提醒這位」).
	mux.HandleFunc("POST "+sessionPath+"/{bundle_id}", claimsRoute(http.MethodPost, false, inboxSendScoped(pool, "inbox:reply",
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.PlanCheckoutReminders(ctx, tx, s, r.Header.Get("Idempotency-Key"), r.PathValue("session_id"), r.PathValue("bundle_id"))
		})))
	mux.HandleFunc("GET "+sessionPath, claimsRoute(http.MethodGet, false, inboxSendScoped(pool, "inbox:read",
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.ReminderReport(ctx, tx, r.PathValue("session_id"))
		})))
	mux.HandleFunc("GET "+settingsPath, inboxRoute(http.MethodGet, false, false, inboxSendScoped(pool, "live:read",
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.GetReminderSettings(ctx, tx)
		})))
	mux.HandleFunc("PUT "+settingsPath, inboxRoute(http.MethodPut, false, false, func(w http.ResponseWriter, r *http.Request) {
		in, ok := claimsBody[inbox.ReminderSettingsInput](w, r, []string{"enabled", "delay_minutes", "expected_version"}, nil)
		if !ok {
			return
		}
		inboxSendScoped(pool, "live:manage", func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.PutReminderSettings(ctx, tx, in)
		})(w, r)
	}))
	// Methodless fallbacks keep wrong-method answers inside the same private response boundary.
	mux.HandleFunc(sessionPath, studioRoute("", false, nil))
	mux.HandleFunc(sessionPath+"/{bundle_id}", studioRoute("", false, nil))
	mux.HandleFunc(settingsPath, studioRoute("", false, nil))
}
