package live

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"livecommerce/internal/command"
	"livecommerce/internal/pagination"
	"livecommerce/internal/platform"
)

const studioTime = "2006-01-02T15:04:05.000000Z"

var ErrStudioProjection = errors.New("studio projection unavailable")

type StudioDestination struct {
	Ordinal  int    `json:"ordinal"`
	Provider string `json:"provider"`
}

type StudioPrepared struct {
	AuthorizationID string              `json:"authorization_id"`
	SessionVersion  int64               `json:"session_version"`
	StartBefore     time.Time           `json:"start_before"`
	Environment     string              `json:"environment"`
	Destinations    []StudioDestination `json:"destinations"`
}

type StudioAttempt struct {
	AttemptID       string              `json:"attempt_id"`
	Environment     string              `json:"environment"`
	OperationState  string              `json:"operation_state"`
	ResourceState   string              `json:"resource_state"`
	TransportStatus string              `json:"transport_status"`
	CleanupRequired bool                `json:"cleanup_required"`
	StopRequested   bool                `json:"stop_requested"`
	StopWireCount   int                 `json:"stop_wire_count"`
	Escalated       bool                `json:"escalated"`
	UpdatedAt       time.Time           `json:"updated_at"`
	Destinations    []StudioDestination `json:"destinations"`
}

type Studio struct {
	Draft     Draft           `json:"draft"`
	Prepared  *StudioPrepared `json:"prepared"`
	Attempt   *StudioAttempt  `json:"attempt"`
	CanManage bool            `json:"can_manage"`
}

func ListDrafts(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string, page pagination.Request) (pagination.Page[Draft], error) {
	out := pagination.Page[Draft]{Items: []Draft{}}
	if ctx == nil || tx == nil {
		return out, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return out, err
	}
	binding := pagination.Binding{TenantID: scope.TenantID, StoreID: scope.StoreID, Collection: "live-sessions"}
	limit, keys, err := pagination.Decode(page, binding, 2)
	if err != nil {
		return out, err
	}
	var afterTime, afterID any
	if len(keys) == 2 {
		afterTime, afterID = keys[0], keys[1]
	}
	rows, err := tx.Query(ctx, `SELECT s.id::text,p.id::text,s.title,s.scheduled_at,p.aspect_ratio,p.state,
	 s.version,s.created_at,s.updated_at FROM live.sessions s
	 JOIN live.programs p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.session_id=s.id
	 WHERE s.tenant_id=$1 AND s.store_id=$2 AND ($3::timestamptz IS NULL
	  OR (s.created_at,s.id)<($3::timestamptz,$4::uuid))
	 ORDER BY s.created_at DESC,s.id DESC LIMIT $5`, scope.TenantID, scope.StoreID, afterTime, afterID, limit+1)
	if err != nil {
		return out, mapError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var item Draft
		var scheduled pgtype.Timestamptz
		if err := rows.Scan(&item.ID, &item.ProgramID, &item.Title, &scheduled, &item.AspectRatio, &item.State,
			&item.Version, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return pagination.Page[Draft]{Items: []Draft{}}, mapError(err)
		}
		item.ScheduledAt = scheduledTime(scheduled)
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Page[Draft]{Items: []Draft{}}, mapError(err)
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		last := out.Items[len(out.Items)-1]
		out.NextCursor, err = pagination.Encode(binding, []string{last.CreatedAt.UTC().Format(studioTime), last.ID})
		if err != nil {
			return pagination.Page[Draft]{Items: []Draft{}}, err
		}
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return pagination.Page[Draft]{Items: []Draft{}}, err
	}
	return out, nil
}

func GetStudio(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, sessionID string) (Studio, error) {
	if ctx == nil || tx == nil || !command.ValidID(sessionID) {
		return Studio{}, command.ErrInvalid
	}
	draft, err := GetDraft(ctx, tx, scope, token, sessionID)
	if err != nil {
		return Studio{}, err
	}
	hash := sha256.Sum256([]byte(token))
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT live.read_studio_media($1::bytea,$2::uuid,$3::uuid)`, hash[:], scope.StoreID, sessionID).Scan(&raw); err != nil {
		return Studio{}, mediaPlanError(err)
	}
	prepared, attempt, err := decodeStudioMedia(raw)
	if err != nil {
		return Studio{}, err
	}
	out := Studio{Draft: draft, Prepared: prepared, Attempt: attempt}
	err = platform.RequirePermission(ctx, tx, scope, token, managePermission)
	if err == nil {
		out.CanManage = true
	} else if !errors.Is(err, platform.ErrForbidden) {
		return Studio{}, err
	}
	if err := authorize(ctx, tx, scope, token, readPermission); err != nil {
		return Studio{}, err
	}
	return out, nil
}

func decodeStudioMedia(raw []byte) (*StudioPrepared, *StudioAttempt, error) {
	if len(raw) > 16<<10 || !studioExactFields(raw, "prepared", "attempt") {
		return nil, nil, ErrStudioProjection
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, nil, ErrStudioProjection
	}
	if string(fields["prepared"]) != "null" && !studioExactFields(fields["prepared"], "authorization_id", "session_version", "start_before", "environment", "destinations") {
		return nil, nil, ErrStudioProjection
	}
	if string(fields["attempt"]) != "null" && !studioExactFields(fields["attempt"], "attempt_id", "environment", "operation_state", "resource_state", "transport_status", "cleanup_required", "stop_requested", "stop_wire_count", "escalated", "updated_at", "destinations") {
		return nil, nil, ErrStudioProjection
	}
	var envelope struct {
		Prepared *StudioPrepared `json:"prepared"`
		Attempt  *StudioAttempt  `json:"attempt"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&envelope); err != nil {
		return nil, nil, ErrStudioProjection
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, nil, ErrStudioProjection
	}
	if envelope.Prepared != nil && envelope.Attempt != nil {
		return nil, nil, ErrStudioProjection
	}
	validDestinations := func(values []StudioDestination) bool {
		if len(values) < 1 || len(values) > 2 {
			return false
		}
		for i, v := range values {
			if v.Ordinal != i+1 || (v.Provider != "facebook" && v.Provider != "instagram") {
				return false
			}
		}
		return true
	}
	if p := envelope.Prepared; p != nil {
		if !studioDestinationsExact(fields["prepared"]) {
			return nil, nil, ErrStudioProjection
		}
		if !command.ValidID(p.AuthorizationID) || p.SessionVersion < 1 || p.StartBefore.IsZero() || p.Environment != "MOCK" || !validDestinations(p.Destinations) {
			return nil, nil, ErrStudioProjection
		}
	}
	if a := envelope.Attempt; a != nil {
		if !studioDestinationsExact(fields["attempt"]) {
			return nil, nil, ErrStudioProjection
		}
		validOperation := false
		for _, state := range []string{"READY", "DISPATCHING", "UNKNOWN", "ACKNOWLEDGED", "SUCCEEDED", "FAILED_FINAL", "CANCELLED", "BLOCKED_POLICY", "STALE_BINDING"} {
			if a.OperationState == state {
				validOperation = true
				break
			}
		}
		validTransport := a.TransportStatus == ""
		for _, status := range []string{"EGRESS_STARTING", "EGRESS_ACTIVE", "EGRESS_ENDING", "EGRESS_COMPLETE", "EGRESS_FAILED", "EGRESS_ABORTED", "EGRESS_LIMIT_REACHED"} {
			if a.TransportStatus == status {
				validTransport = true
				break
			}
		}
		if !command.ValidID(a.AttemptID) || a.Environment != "MOCK" || !validOperation || !validTransport ||
			(a.ResourceState != "UNOBSERVED" && a.ResourceState != "OBSERVED" && a.ResourceState != "TERMINAL") ||
			a.StopWireCount < 0 || a.StopWireCount > 2 || a.UpdatedAt.IsZero() || !validDestinations(a.Destinations) {
			return nil, nil, ErrStudioProjection
		}
	}
	return envelope.Prepared, envelope.Attempt, nil
}

func studioExactFields(raw []byte, names ...string) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != len(names) {
		return false
	}
	for _, name := range names {
		if _, ok := fields[name]; !ok {
			return false
		}
	}
	return true
}

func studioDestinationsExact(raw []byte) bool {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return false
	}
	var values []json.RawMessage
	if err := json.Unmarshal(object["destinations"], &values); err != nil || len(values) < 1 || len(values) > 2 {
		return false
	}
	for _, value := range values {
		if !studioExactFields(value, "ordinal", "provider") {
			return false
		}
	}
	return true
}
