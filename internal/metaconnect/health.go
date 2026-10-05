// Purpose: the merchant health read model behind B1 GET /meta/health and B2 POST /meta/health/recheck (contract
// meta-connection-health-v1 §9): per-Page severity + capability rows from the CapabilityReader, and the manual re-check
// that only pulls next_due_at earlier (naturally idempotent, no command). The banner never gates anything: any read failure
// shows nothing (the route's 5xx stays server-side).
// Depends on: the CapabilityReader (TableReader/SnapshotReader), integration.meta_health_probes (commerce_runtime SELECT
// policy), integration.meta_health_snapshot() and integration.request_meta_health_recheck (0125), tokenHash (service.go).
// Used by: internal/httpapi/meta_health.go (B1/B2); LC-B1/A5-3 may call Recheck pre-live (§5.3).
// Invariants: server-resolved scope only (I01); a page never probed reads severity "none" (no banner on no data); the
// response never carries a token, scopes dump or Graph body.
// Status: MOCK (REAL_PG + fake Graph; LIVE probe MCH12 NOT_RUN).

package metaconnect

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/platform"
)

// Health is the read + recheck service for the meta connection-health banner API.
type Health struct {
	Reader CapabilityReader
}

func (h Health) reader() CapabilityReader {
	if h.Reader != nil {
		return h.Reader
	}
	return TableReader{Fallback: SnapshotReader{}}
}

// HealthCapability is one CapabilityState rendered for B1 (CheckedAt as RFC3339).
type HealthCapability struct {
	BindingID string `json:"binding_id"`
	Provider  string `json:"provider"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
	Evidence  string `json:"evidence"`
	CheckedAt string `json:"checked_at,omitempty"`
}

// HealthPage is one connected Page's row of the B1 response.
type HealthPage struct {
	PageID          string             `json:"page_id"`
	PageName        string             `json:"page_name"`
	Status          string             `json:"status"`
	Severity        string             `json:"severity"`
	CheckedAt       string             `json:"checked_at,omitempty"`
	NextCheckAt     string             `json:"next_check_at,omitempty"`
	EpisodeOpenedAt string             `json:"episode_opened_at,omitempty"`
	Capabilities    []HealthCapability `json:"capabilities"`
}

// HealthStatus is the B1 body: store severity = worst Page severity (none when every Page is fine or never probed).
type HealthStatus struct {
	Severity string       `json:"severity"`
	Pages    []HealthPage `json:"pages"`
}

// Status reads the store's health banner model in one scoped transaction.
func (h Health) Status(ctx context.Context, tx pgx.Tx, scope platform.Scope) (*HealthStatus, error) {
	probes, err := readProbes(ctx, tx)
	if err != nil {
		return nil, err
	}
	rows, err := readSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	caps, err := h.reader().Capabilities(ctx, tx, scope, "")
	if err != nil {
		return nil, err
	}
	byBinding := make(map[string][]CapabilityState, len(caps))
	for _, c := range caps {
		byBinding[c.BindingID] = append(byBinding[c.BindingID], c)
	}
	out := &HealthStatus{Severity: "none", Pages: make([]HealthPage, 0, len(rows))}
	for _, row := range rows {
		probe := probes[row.PageID]
		page := HealthPage{
			PageID:          row.PageID,
			PageName:        row.PageName,
			Status:          row.Status,
			Severity:        probe.severity,
			CheckedAt:       probe.checkedAt,
			NextCheckAt:     probe.nextDue,
			EpisodeOpenedAt: probe.episodeOpenedAt,
			Capabilities:    []HealthCapability{},
		}
		if page.Severity == "" {
			page.Severity = "none"
		}
		for _, b := range []string{row.FBBinding, row.IGBinding} {
			if b == "" {
				continue
			}
			for _, c := range byBinding[b] {
				page.Capabilities = append(page.Capabilities, HealthCapability{
					BindingID: c.BindingID, Provider: c.Provider, State: c.State, Reason: c.Reason,
					Evidence: c.Evidence, CheckedAt: formatTime(c.CheckedAt),
				})
			}
		}
		if page.Severity == "blocking" || out.Severity == "blocking" {
			out.Severity = "blocking"
		} else if page.Severity == "warning" {
			out.Severity = "warning"
		}
		out.Pages = append(out.Pages, page)
	}
	return out, nil
}

// probeRow is the B1-relevant slice of integration.meta_health_probes (already RLS-scoped).
type probeRow struct {
	severity        string
	checkedAt       string
	nextDue         string
	episodeOpenedAt string
}

func readProbes(ctx context.Context, tx pgx.Tx) (map[string]probeRow, error) {
	rows, err := tx.Query(ctx, `SELECT page_id, severity, last_checked_at, next_due_at, episode_opened_at
		FROM integration.meta_health_probes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]probeRow)
	for rows.Next() {
		var pageID string
		var p probeRow
		var severity *string
		var checkedAt, nextDue, episode *time.Time
		if err := rows.Scan(&pageID, &severity, &checkedAt, &nextDue, &episode); err != nil {
			return nil, err
		}
		if severity != nil {
			p.severity = *severity
		}
		p.checkedAt = formatTimePtr(checkedAt)
		p.nextDue = formatTimePtr(nextDue)
		p.episodeOpenedAt = formatTimePtr(episode)
		out[pageID] = p
	}
	return out, rows.Err()
}

// Recheck pulls the Page's next probe to now (integration:manage inside the definer; the route already scoped it). It
// returns the new next_check_at, or a refusal the route maps (not_found 404, recheck_too_soon 429).
func (h Health) Recheck(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, pageID string) (time.Time, error) {
	hash, err := tokenHash(token)
	if err != nil {
		return time.Time{}, err
	}
	var next time.Time
	if err := tx.QueryRow(ctx, `SELECT integration.request_meta_health_recheck($1, $2, $3)`,
		hash, scope.StoreID, pageID).Scan(&next); err != nil {
		return time.Time{}, err
	}
	return next, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
