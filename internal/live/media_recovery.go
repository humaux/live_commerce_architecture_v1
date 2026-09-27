package live

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/platform"
)

var ErrMediaRecovery = errors.New("media_recovery_unavailable")

type RecoveryMember struct {
	Disposition, EpisodeID, OperationID string
	JobID, BaselineGeneration           int64
	DeadlineAt                          time.Time
	CandidateCount                      int
	CoverageKnown                       bool
	BlockedByEpisodeID                  string
}

type RecoveryReadback struct {
	EpisodeID, ScopeStatus, OperationID, Disposition string
	CoverageKnown                                    bool
	CandidateCount                                   int
	BlockedByEpisodeID                               string
	BaselineGeneration, ObservationGeneration        int64
	ObservationID, ObservationSource                 string
	WitnessElapsedMS                                 int64
	TimeoutAt                                        time.Time
	CleanupRequired                                  bool
}

type MediaRecoveryObserver struct {
	pool     *pgxpool.Pool
	projects map[mediaProjectKey]mediaEndpoint
}

// The diagnostic ledger remains usable when native/project configuration is
// invalid; it never enables provider I/O by itself.
func NewMediaRecoveryLedger(pool *pgxpool.Pool) *MediaRecoveryObserver {
	return &MediaRecoveryObserver{pool: pool}
}

// The observer has no River client, material keyring, Start or Stop method.
func NewMediaRecoveryObserver(ctx context.Context, pool *pgxpool.Pool, projects []MediaProject) (*MediaRecoveryObserver, error) {
	if ctx == nil || pool == nil || len(projects) < 1 || len(projects) > 128 {
		return nil, ErrMediaRecovery
	}
	if platform.ValidateMediaRecoveryPool(ctx, pool) != nil {
		return nil, ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var ready bool
	if pool.QueryRow(bounded, `SELECT live.media_recovery_ready()`).Scan(&ready) != nil || !ready {
		return nil, ErrMediaRecovery
	}
	selected := make(map[mediaProjectKey]mediaEndpoint, len(projects))
	for _, p := range projects {
		if !mediaProjectPattern.MatchString(p.ProjectID) || p.CredentialVersion < 1 ||
			p.Config.Environment != "MOCK" || p.Transport == nil {
			return nil, ErrMediaRecovery
		}
		key := mediaProjectKey{p.ProjectID, p.CredentialVersion}
		if _, exists := selected[key]; exists {
			return nil, ErrMediaRecovery
		}
		client, err := livekit.New(p.Config, p.Transport)
		if err != nil {
			return nil, ErrMediaRecovery
		}
		selected[key] = mediaEndpoint{identity: p.Config.Endpoint, client: client}
	}
	return &MediaRecoveryObserver{pool: pool, projects: selected}, nil
}

func (r *MediaRecoveryObserver) Begin(ctx context.Context, episode string, elapsedMS int64, capacity int, coverageKnown bool) ([]RecoveryMember, error) {
	if r == nil || r.pool == nil || ctx == nil || capacity < 1 || capacity > 32 || elapsedMS < 0 {
		return nil, ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := r.pool.Query(bounded, `SELECT disposition,episode_id::text,operation_id::text,job_id,
	 baseline_generation,deadline_at,candidate_count,coverage_known,blocked_by_episode_id::text
	 FROM live.begin_media_recovery_episode($1::uuid,$2::bigint,$3::integer,$4::boolean)`,
		episode, elapsedMS, capacity, coverageKnown)
	if err != nil {
		return nil, ErrMediaRecovery
	}
	defer rows.Close()
	var result []RecoveryMember
	for rows.Next() {
		var m RecoveryMember
		var operation, blocked sql.NullString
		var job, generation sql.NullInt64
		if rows.Scan(&m.Disposition, &m.EpisodeID, &operation, &job, &generation,
			&m.DeadlineAt, &m.CandidateCount, &m.CoverageKnown, &blocked) != nil {
			return nil, ErrMediaRecovery
		}
		m.OperationID, m.BlockedByEpisodeID = operation.String, blocked.String
		m.JobID, m.BaselineGeneration = job.Int64, generation.Int64
		result = append(result, m)
	}
	if rows.Err() != nil || len(result) == 0 || len(result) > 32 && result[0].Disposition == "pending" {
		return nil, ErrMediaRecovery
	}
	return result, nil
}

func (r *MediaRecoveryObserver) Read(ctx context.Context, episode string) ([]RecoveryReadback, error) {
	if r == nil || r.pool == nil || ctx == nil {
		return nil, ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := r.pool.Query(bounded, `SELECT episode_id::text,scope_status,coverage_known,candidate_count,
	 blocked_by_episode_id::text,operation_id::text,disposition,baseline_generation,
	 observation_id::text,observation_source,observation_generation,witness_elapsed_ms,timeout_at,cleanup_required
	 FROM live.read_media_recovery_episode($1::uuid)`, episode)
	if err != nil {
		return nil, ErrMediaRecovery
	}
	defer rows.Close()
	var result []RecoveryReadback
	for rows.Next() {
		var x RecoveryReadback
		var blocked, operation, disposition, observation, source sql.NullString
		var baseline, generation, witness sql.NullInt64
		var timeout sql.NullTime
		var cleanup sql.NullBool
		if rows.Scan(&x.EpisodeID, &x.ScopeStatus, &x.CoverageKnown, &x.CandidateCount,
			&blocked, &operation, &disposition, &baseline, &observation, &source,
			&generation, &witness, &timeout, &cleanup) != nil {
			return nil, ErrMediaRecovery
		}
		x.BlockedByEpisodeID, x.OperationID, x.Disposition = blocked.String, operation.String, disposition.String
		x.BaselineGeneration, x.ObservationGeneration = baseline.Int64, generation.Int64
		x.ObservationID, x.ObservationSource = observation.String, source.String
		x.WitnessElapsedMS, x.TimeoutAt, x.CleanupRequired = witness.Int64, timeout.Time, cleanup.Bool
		result = append(result, x)
	}
	if rows.Err() != nil || len(result) == 0 {
		return nil, ErrMediaRecovery
	}
	return result, nil
}

func (r *MediaRecoveryObserver) Witness(ctx context.Context, episode, operation, observation string, elapsedMS int64) (string, error) {
	if r == nil || r.pool == nil || ctx == nil || elapsedMS < 0 || elapsedMS > 90000 {
		return "", ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result string
	if r.pool.QueryRow(bounded, `SELECT live.witness_media_recovery_episode($1::uuid,$2::uuid,$3::uuid,$4::bigint)`,
		episode, operation, observation, elapsedMS).Scan(&result) != nil {
		return "", ErrMediaRecovery
	}
	return result, nil
}

func (r *MediaRecoveryObserver) Timeout(ctx context.Context, episode string, elapsedMS int64) (string, int, error) {
	if r == nil || r.pool == nil || ctx == nil || elapsedMS < 90000 {
		return "", 0, ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result string
	var count int
	if r.pool.QueryRow(bounded, `SELECT disposition,affected_count FROM live.timeout_media_recovery_episode($1::uuid,$2::bigint)`,
		episode, elapsedMS).Scan(&result, &count) != nil {
		return "", 0, ErrMediaRecovery
	}
	return result, count, nil
}

// Observe performs at most one fenced Query/FindByRoom; the parent owns retries
// and samples the post-commit readback before it may attest a witness.
func (r *MediaRecoveryObserver) Observe(ctx context.Context, episode string, member RecoveryMember) (string, error) {
	if r == nil || r.pool == nil || ctx == nil || member.Disposition != "pending" || member.OperationID == "" || member.JobID < 1 {
		return "", ErrMediaRecovery
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", ErrMediaRecovery
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	var status string
	var projectID, endpoint, room, egress sql.NullString
	var generation, version sql.NullInt64
	err := r.pool.QueryRow(bounded, `SELECT disposition,generation,project_id,credential_version,
	 endpoint_identity,room_name,egress_id FROM live.claim_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea)`,
		episode, member.OperationID, member.JobID, token[:]).Scan(&status, &generation,
		&projectID, &version, &endpoint, &room, &egress)
	cancel()
	if err != nil {
		return "", ErrMediaRecovery
	}
	if status != "claimed" {
		return status, nil
	}
	if !generation.Valid || generation.Int64 < 1 {
		return "", ErrMediaRecovery
	}
	finish := func(code string) {
		short, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		var released string
		_ = r.pool.QueryRow(short, `SELECT live.finish_recovery_observation($1::uuid,$2::uuid,$3::bigint,$4::bytea,$5::text)`,
			episode, member.OperationID, generation.Int64, token[:], code).Scan(&released)
	}
	project, found := r.projects[mediaProjectKey{projectID.String, version.Int64}]
	if !found || !projectID.Valid || !version.Valid || project.identity != endpoint.String || !room.Valid {
		finish("credential_unavailable")
		return "credential_unavailable", nil
	}
	provider, done := context.WithTimeout(ctx, 10*time.Second)
	var observation livekit.Observation
	var providerErr error
	source := "ROOM"
	if egress.Valid {
		source = "QUERY"
		observation, providerErr = project.client.Query(provider, livekit.Target{RoomName: room.String, EgressID: egress.String})
	} else {
		observation, providerErr = project.client.FindByRoom(provider, room.String)
	}
	done()
	if providerErr != nil {
		code := "remote_unknown"
		if errors.Is(providerErr, livekit.ErrNotObserved) {
			code = "not_observed"
		} else if errors.Is(providerErr, livekit.ErrInvalid) {
			code = "invalid_observation"
		}
		finish(code)
		return code, nil
	}
	bounded, cancel = context.WithTimeout(ctx, 5*time.Second)
	var disposition, observationID string
	err = r.pool.QueryRow(bounded, `SELECT disposition,observation_id::text FROM live.record_recovery_observation(
	 $1::uuid,$2::uuid,$3::bigint,$4::bytea,$5::text,$6::text,$7::text,$8::text,$9::bigint,$10::bigint,$11::bigint)`,
		episode, member.OperationID, generation.Int64, token[:], source, observation.EgressID,
		observation.RoomName, observation.Status, observation.StartedAtNS,
		observation.UpdatedAtNS, observation.EndedAtNS).Scan(&disposition, &observationID)
	cancel()
	if err != nil || observationID == "" {
		finish("invalid_observation")
		return "", ErrMediaRecovery
	}
	return disposition, nil
}
