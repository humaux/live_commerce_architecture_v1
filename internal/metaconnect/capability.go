// Purpose: the live-console §6 capability read model (contracts/meta-connection-health-v1 §7): the CapabilityReader seam LC-B3
// introduced, plus its two implementations. SnapshotReader derives every §6 row from what connect stored (meta_connections +
// the 0122 resubscribe outcome), TableReader reads integration.binding_capabilities (the probed truth) and falls back to the
// SnapshotReader for any binding the probe has not yet written, so callers never see fewer rows or a new state during rollout.
// Depends on: Derive (derive.go), the SECURITY DEFINER integration.meta_health_snapshot() (migration 0125, the only way
// commerce_runtime reads meta_connections — it has no RLS policy) and the commerce_runtime SELECT policy on
// integration.binding_capabilities; config COMMERCE_META_ADVANCED_ACCESS / COMMERCE_META_DM_RECEIVER_CONFIRMED.
// Used by: cmd/api (the B1 route and the CapabilityReader swap §7.4), LC-B3's console read model, tests (MCH08).
// Invariants: read-only; runs inside the caller's scoped transaction (tenant/store from the GUCs, never from a request);
// SnapshotReader evidence is always DESIGN, CheckedAt zero (never probed); TableReader rows win over the fallback.
// Status: MOCK (REAL_PG + fake Graph; LIVE probe is MCH12, NOT_RUN).

package metaconnect

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/platform"
)

// Evidence labels of integration.binding_capabilities.evidence (§2, §3.4): DESIGN < MOCK < LIVE_READ < LIVE_SEND, monotonic
// (only mark_capability_evidence upgrades them). The probe starts rows at MOCK (loopback fake Graph) else DESIGN; it never
// changes an existing row's evidence.
const (
	EvidenceDesign   = "DESIGN"
	EvidenceMock     = "MOCK"
	EvidenceLiveRead = "LIVE_READ"
	EvidenceLiveSend = "LIVE_SEND"
)

// CapabilityState is one live-console §6 row. Reason is a §3.3 code; CheckedAt is zero when never probed.
type CapabilityState struct {
	BindingID, Provider, Capability, State, Reason, Evidence string
	CheckedAt                                                time.Time
}

// CapabilityReader returns the §6 rows of the scope's store (all Pages, or one binding when bindingID != "").
// Read-only; runs inside the caller's scoped transaction (RLS on tenant/store GUCs).
type CapabilityReader interface {
	Capabilities(ctx context.Context, tx pgx.Tx, scope platform.Scope, bindingID string) ([]CapabilityState, error)
}

// ReaderConfig is the deployment state both readers feed Derive: the permissions with Advanced Access (rule 8) and whether
// LC-U11 is closed (rule 7). The zero value is the safe pre-review default (no Advanced Access, DM not confirmed).
type ReaderConfig struct {
	AdvancedAccess map[string]bool // permissions with Advanced Access (empty = none: every grant is Standard Access)
	DMConfirmed    bool            // COMMERCE_META_DM_RECEIVER_CONFIRMED (LC-U11 closed)
}

// NewReaderConfig parses COMMERCE_META_ADVANCED_ACCESS (comma list; blank entries dropped) into a ReaderConfig.
func NewReaderConfig(advancedCSV string, dmConfirmed bool) ReaderConfig {
	m := make(map[string]bool)
	for _, p := range strings.Split(advancedCSV, ",") {
		if p = strings.TrimSpace(p); p != "" {
			m[p] = true
		}
	}
	return ReaderConfig{AdvancedAccess: m, DMConfirmed: dmConfirmed}
}

// SnapshotReader is LC-B3's default reader (§7.2): it derives the §6 rows from what connect stored, with evidence DESIGN.
type SnapshotReader struct {
	Config ReaderConfig
}

// Capabilities derives the §6 rows of the scope's store from the connect snapshot (token valid when status=active else
// invalid, perms = scopes, perm_source=snapshot, tasks = connect snapshot {MESSAGING, MODERATE}, fb_fields = feed plus
// messages once the 0122 resubscribe SUCCEEDED for that Page). bindingID != "" limits to that binding.
func (r SnapshotReader) Capabilities(ctx context.Context, tx pgx.Tx, scope platform.Scope, bindingID string) ([]CapabilityState, error) {
	rows, err := readSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	out := r.fromSnapshot(rows, bindingID)
	sortCapabilities(out) // same order as TableReader so an empty table and the snapshot are byte-identical (MCH08)
	return out, nil
}

// snapshotRow is one connected Page's read-side facts, returned by integration.meta_health_snapshot().
type snapshotRow struct {
	PageID    string
	PageName  string
	Status    string
	FBBinding string
	IGBinding string // "" when the Page has no Instagram binding
	Scopes    []string
	Messages  bool // the 0122 resubscribe job SUCCEEDED for this Page (messages webhook subscribed)
}

// readSnapshot runs the GUC-scoped definer (the only commerce_runtime read of meta_connections). It is STABLE: safe inside
// the caller's transaction.
func readSnapshot(ctx context.Context, tx pgx.Tx) ([]snapshotRow, error) {
	rows, err := tx.Query(ctx, `SELECT o_page, o_page_name, o_status, o_fb::text, o_ig::text, o_scopes, o_messages
		FROM integration.meta_health_snapshot()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []snapshotRow
	for rows.Next() {
		var s snapshotRow
		var ig *string
		if err := rows.Scan(&s.PageID, &s.PageName, &s.Status, &s.FBBinding, &ig, &s.Scopes, &s.Messages); err != nil {
			return nil, err
		}
		if ig != nil {
			s.IGBinding = *ig
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r SnapshotReader) fromSnapshot(rows []snapshotRow, bindingID string) []CapabilityState {
	tasks := map[string]bool{"MESSAGING": true, "MODERATE": true}
	var out []CapabilityState
	for _, row := range rows {
		reading := Reading{
			Token:       TokenValid,
			Perms:       setOf(row.Scopes),
			PermSource:  PermSourceSnapshot,
			Tasks:       tasks,
			FBFields:    map[string]bool{"feed": true},
			AppReview:   r.Config.AdvancedAccess,
			DMConfirmed: r.Config.DMConfirmed,
		}
		if row.Messages {
			reading.FBFields["messages"] = true
		}
		if row.Status != "active" {
			reading.Token = TokenInvalid // status=reauth_required: rule 1, subcode unknown -> token_invalid
		}
		for _, b := range []struct {
			provider, binding string
		}{{"facebook", row.FBBinding}, {"instagram", row.IGBinding}} {
			if b.binding == "" || (bindingID != "" && b.binding != bindingID) {
				continue
			}
			for _, capability := range Capabilities(b.provider) {
				state, reason := Derive(b.provider, capability, reading)
				out = append(out, CapabilityState{
					BindingID: b.binding, Provider: b.provider, Capability: capability,
					State: state, Reason: reason, Evidence: EvidenceDesign,
				})
			}
		}
	}
	return out
}

// TableReader is W1-01B's reader (§7.3): it reads the probed integration.binding_capabilities under RLS and, for any binding
// the probe has not yet written (connection created before the first sweep), falls back to Fallback (the SnapshotReader).
type TableReader struct {
	Fallback CapabilityReader
}

// Capabilities merges probed rows with fallback rows for not-yet-probed bindings, deterministically ordered.
func (r TableReader) Capabilities(ctx context.Context, tx pgx.Tx, scope platform.Scope, bindingID string) ([]CapabilityState, error) {
	fallback := r.Fallback
	if fallback == nil {
		fallback = SnapshotReader{}
	}
	table, err := readTable(ctx, tx, bindingID)
	if err != nil {
		return nil, err
	}
	rows, err := readSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(table))
	for _, c := range table {
		present[c.BindingID] = true
	}
	out := table
	for _, row := range rows {
		for _, b := range []struct {
			binding string
		}{{row.FBBinding}, {row.IGBinding}} {
			if b.binding == "" || (bindingID != "" && b.binding != bindingID) || present[b.binding] {
				continue
			}
			snap, err := fallback.Capabilities(ctx, tx, scope, b.binding)
			if err != nil {
				return nil, err
			}
			out = append(out, snap...)
		}
	}
	sortCapabilities(out)
	return out, nil
}

// readTable reads probed rows under RLS (commerce_runtime SELECT policy on the GUC-scoped store).
func readTable(ctx context.Context, tx pgx.Tx, bindingID string) ([]CapabilityState, error) {
	var rows pgx.Rows
	var err error
	if bindingID == "" {
		rows, err = tx.Query(ctx, `SELECT binding_id::text, provider, capability, state, reason, evidence, checked_at
			FROM integration.binding_capabilities ORDER BY binding_id, capability`)
	} else {
		rows, err = tx.Query(ctx, `SELECT binding_id::text, provider, capability, state, reason, evidence, checked_at
			FROM integration.binding_capabilities WHERE binding_id=$1::uuid ORDER BY binding_id, capability`, bindingID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CapabilityState
	for rows.Next() {
		var c CapabilityState
		if err := rows.Scan(&c.BindingID, &c.Provider, &c.Capability, &c.State, &c.Reason, &c.Evidence, &c.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// sortCapabilities orders rows deterministically so the two readers produce byte-identical fixtures (MCH08).
func sortCapabilities(rows []CapabilityState) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].BindingID != rows[j].BindingID {
			return rows[i].BindingID < rows[j].BindingID
		}
		return rows[i].Capability < rows[j].Capability
	})
}

// setOf turns a granted-scope list into the set Derive reads (nil-safe: a nil list becomes an empty set, not "unknown" —
// the connect snapshot is never unknown).
func setOf(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, s := range list {
		m[s] = true
	}
	return m
}
