// Purpose: the W3-03B checkout-reminder HTTP adapter: POST /live-sessions/{sid}/reminders[/{bundle_id}] (the merchant triggers one reminder pass over
// every eligible buyer, or for one buyer) and GET /live-sessions/{sid}/reminders (sent / queued / failed / follow-up list). It runs the
// merchanttools.CheckoutReminders pass (one scan transaction, then one transaction per buyer) and the inbox.Service report, and maps refusals
// through inboxSendClassify; it decides no rule (the 0144 definers do), never returns a driver message and never logs a body, link or token.
// The per-store 「自動提醒」 settings routes are DEFERRED with the automatic path (keyring custody): live.reminder_settings has no route.
// Depends on: internal/merchanttools (CheckoutReminders), internal/inbox (report), internal/httpapi inboxSendScoped/claimsRoute, internal/platform.
// Used by: internal/httpapi/handler.go (registerReminderRoutes, gated on Options.Inbox with the send side enabled); cmd/api wires it.
// Invariants: trigger needs inbox:reply (+ live:manage, re-checked in SQL), report inbox:read; cross-store -> 404 by the definers.

package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/inbox"
	"livecommerce/internal/merchanttools"
	"livecommerce/internal/platform"
)

// registerReminderRoutes mounts the reminder routes. Nothing is mounted unless the inbox service has the send side enabled (a reminder is a DM:
// without the seal ring and the job client it could never be sent). manual may be nil (unpaid orders then become link_unavailable refusals).
func registerReminderRoutes(mux *http.ServeMux, pool *pgxpool.Pool, svc *inbox.Service, manual *merchanttools.ManualOrders) {
	if svc == nil || !svc.SendEnabled() {
		return
	}
	const sessionPath = "/v1/admin/stores/{store_id}/live-sessions/{session_id}/reminders"
	reminders, err := merchanttools.NewCheckoutReminders(pool, manual, svc)
	if err != nil { // a nil pool (DB-free router tests): the report route still mounts, the trigger does not
		reminders = nil
	}
	trigger := func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || strings.ContainsAny(strings.TrimPrefix(header, "Bearer "), " \t\r\n") {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if reminders == nil {
			respondError(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		// One scan transaction + one per buyer (≤ 100): a longer budget than a single scoped call.
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		out, err := reminders.Trigger(ctx, strings.TrimPrefix(header, "Bearer "), r.PathValue("store_id"), r.Header.Get("Idempotency-Key"),
			r.PathValue("session_id"), r.PathValue("bundle_id"))
		if err != nil {
			status, code := inboxSendClassify(err)
			respondErrorDetails(w, err, status, code)
			return
		}
		respond(w, http.StatusOK, out)
	}
	// Trigger: one pass over the session's unpaid buyers (or one buyer); Idempotency-Key required, no body.
	mux.HandleFunc("POST "+sessionPath, claimsRoute(http.MethodPost, false, trigger))
	mux.HandleFunc("POST "+sessionPath+"/{bundle_id}", claimsRoute(http.MethodPost, false, trigger))
	mux.HandleFunc("GET "+sessionPath, claimsRoute(http.MethodGet, false, inboxSendScoped(pool, "inbox:read",
		func(ctx context.Context, tx pgx.Tx, s platform.Scope, r *http.Request) (any, error) {
			return svc.ReminderReport(ctx, tx, r.PathValue("session_id"))
		})))
	// Methodless fallbacks keep wrong-method answers inside the same private response boundary.
	mux.HandleFunc(sessionPath, studioRoute("", false, nil))
	mux.HandleFunc(sessionPath+"/{bundle_id}", studioRoute("", false, nil))
}
