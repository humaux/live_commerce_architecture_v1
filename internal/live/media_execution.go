package live

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/platform"
)

const mediaSchema = "river_media"
const mediaObservationDelay = 5 * time.Second

var (
	ErrMediaConfig      = errors.New("live: media runtime configuration unavailable")
	ErrMediaDatabase    = errors.New("live: media database unavailable")
	ErrMediaJob         = errors.New("live: invalid media job")
	mediaProjectPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
)

// MediaProject is a platform-owned MOCK endpoint, not merchant-supplied authority.
type MediaProject struct {
	ProjectID         string
	CredentialVersion int64
	Config            livekit.Config
	Transport         http.RoundTripper
}

func (MediaProject) String() string     { return "live.MediaProject{redacted}" }
func (p MediaProject) GoString() string { return p.String() }
func (MediaProject) MarshalJSON() ([]byte, error) {
	return []byte(`"live.MediaProject{redacted}"`), nil
}

type mediaProjectKey struct {
	id      string
	version int64
}
type mediaEndpoint struct {
	identity string
	client   *livekit.Client
}
type mediaExecutionWorker struct {
	river.WorkerDefaults[mediaOperationArgs]
	pool     *pgxpool.Pool
	keys     *livekit.MaterialKeyring
	projects map[mediaProjectKey]mediaEndpoint
}

func (mediaExecutionWorker) String() string     { return "live.mediaExecutionWorker{redacted}" }
func (w mediaExecutionWorker) GoString() string { return w.String() }
func (mediaExecutionWorker) MarshalJSON() ([]byte, error) {
	return []byte(`"live.mediaExecutionWorker{redacted}"`), nil
}
func (*mediaExecutionWorker) Timeout(*river.Job[mediaOperationArgs]) time.Duration {
	return 30 * time.Second
}

// NewMediaClient starts no job; caller owns both pool lifecycles and River Start/Stop.
func NewMediaClient(ctx context.Context, workerPool, executorPool *pgxpool.Pool,
	keys *livekit.MaterialKeyring, projects []MediaProject, concurrency int,
) (*river.Client[pgx.Tx], error) {
	if ctx == nil || workerPool == nil || executorPool == nil || keys == nil ||
		concurrency < 1 || concurrency > 32 || len(projects) < 1 || len(projects) > 128 {
		return nil, ErrMediaConfig
	}
	selected := make(map[mediaProjectKey]mediaEndpoint, len(projects))
	for _, p := range projects {
		if !mediaProjectPattern.MatchString(p.ProjectID) || p.CredentialVersion < 1 ||
			p.Config.Environment != "MOCK" || p.Transport == nil {
			return nil, ErrMediaConfig
		}
		k := mediaProjectKey{p.ProjectID, p.CredentialVersion}
		if _, exists := selected[k]; exists {
			return nil, ErrMediaConfig
		}
		client, err := livekit.New(p.Config, p.Transport)
		if err != nil {
			return nil, ErrMediaConfig
		}
		selected[k] = mediaEndpoint{identity: p.Config.Endpoint, client: client}
	}
	preflight, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if platform.ValidateMediaWorkerPool(preflight, workerPool) != nil ||
		platform.ValidateMediaExecutorPool(preflight, executorPool) != nil ||
		platform.ValidateSameDatabase(preflight, workerPool, executorPool) != nil {
		return nil, ErrMediaDatabase
	}
	var ready bool
	if executorPool.QueryRow(preflight, `SELECT live.media_worker_ready()`).Scan(&ready) != nil || !ready {
		return nil, ErrMediaDatabase
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &mediaExecutionWorker{pool: executorPool, keys: keys, projects: selected})
	client, err := river.NewClient(riverpgxv5.New(workerPool), &river.Config{
		Schema: mediaSchema, Workers: workers,
		Queues: map[string]river.QueueConfig{mediaMockQueue: {MaxWorkers: concurrency}},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		return nil, ErrMediaDatabase
	}
	return client, nil
}

type mediaLoaded struct {
	TenantID          string `json:"tenant_id"`
	StoreID           string `json:"store_id"`
	SessionID         string `json:"session_id"`
	AttemptID         string `json:"attempt_id"`
	ProjectID         string `json:"project_id"`
	EndpointIdentity  string `json:"endpoint_identity"`
	CredentialVersion int64  `json:"credential_version"`
	MaterialVersion   int64  `json:"material_version"`
	RoomName          string `json:"room_name"`
	AspectRatio       string `json:"aspect_ratio"`
	EgressID          string `json:"egress_id"`
	Mode              string `json:"mode"`
	KeyID             string `json:"key_id"`
	NonceHex          string `json:"nonce_hex"`
	CiphertextHex     string `json:"ciphertext_hex"`
}

func (w *mediaExecutionWorker) call(ctx context.Context, sql string, args ...any) error {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := w.pool.Exec(bounded, sql, args...)
	if err != nil {
		return ErrMediaDatabase
	}
	return nil
}

func (w *mediaExecutionWorker) finish(ctx context.Context, operation string, generation int64, token []byte, code string) error {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result string
	if w.pool.QueryRow(bounded, `SELECT live.finish_media_uncertain($1::uuid,$2::bigint,$3::bytea,$4::text)`,
		operation, generation, token, code).Scan(&result) != nil {
		return ErrMediaDatabase
	}
	switch result {
	case "observe":
		return river.JobSnooze(mediaObservationDelay)
	case "terminal", "escalated":
		return nil
	default:
		return ErrMediaDatabase
	}
}

func (w *mediaExecutionWorker) record(ctx context.Context, operation string, generation int64, token []byte, source string, obs livekit.Observation) error {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result string
	if w.pool.QueryRow(bounded, `SELECT live.record_media_observation($1::uuid,$2::bigint,$3::bytea,
		$4::text,$5::text,$6::text,$7::text,$8::bigint,$9::bigint,$10::bigint)`,
		operation, generation, token, source, obs.EgressID, obs.RoomName, obs.Status,
		obs.StartedAtNS, obs.UpdatedAtNS, obs.EndedAtNS).Scan(&result) != nil {
		return ErrMediaDatabase
	}
	switch result {
	case "observe":
		return river.JobSnooze(mediaObservationDelay)
	case "terminal", "escalated":
		return nil
	default:
		return ErrMediaDatabase
	}
}

// A Query and the Stop permission it may earn are committed atomically. A
// missing COMMIT acknowledgement never authorizes a network Stop call.
func (w *mediaExecutionWorker) recordCleanupQuery(ctx context.Context, operation string, generation int64,
	token []byte, obs livekit.Observation) (string, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result string
	if w.pool.QueryRow(bounded, `SELECT live.record_media_cleanup_query($1::uuid,$2::bigint,$3::bytea,
		$4::text,$5::text,$6::text,$7::bigint,$8::bigint,$9::bigint)`,
		operation, generation, token, obs.EgressID, obs.RoomName, obs.Status,
		obs.StartedAtNS, obs.UpdatedAtNS, obs.EndedAtNS).Scan(&result) != nil {
		return "", ErrMediaDatabase
	}
	switch result {
	case "observe", "terminal", "escalated", "stop_reserved":
		return result, nil
	default:
		return "", ErrMediaDatabase
	}
}

func (w *mediaExecutionWorker) Work(ctx context.Context, job *river.Job[mediaOperationArgs]) error {
	if job == nil || job.JobRow == nil || job.ID < 1 || job.Args.Version != 1 ||
		!command.ValidID(job.Args.OperationID) || job.Kind != (mediaOperationArgs{}).Kind() ||
		job.Queue != mediaMockQueue {
		return river.JobCancel(ErrMediaJob)
	}
	if ctx == nil || w == nil || w.pool == nil || w.keys == nil {
		return ErrMediaDatabase
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return ErrMediaDatabase
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	var disposition, mode string
	var generation int64
	err := w.pool.QueryRow(bounded, `SELECT disposition,generation,mode FROM live.claim_media_operation(
		$1::uuid,$2::bigint,$3::integer,$4::bytea)`, job.Args.OperationID, job.ID, 30, token[:]).Scan(
		&disposition, &generation, &mode)
	cancel()
	if err != nil {
		return ErrMediaDatabase
	}
	switch disposition {
	case "busy":
		return river.JobSnooze(mediaObservationDelay)
	case "terminal", "escalated":
		return nil
	case "claimed":
		if mode != "dispatch" && mode != "reconcile" {
			return ErrMediaDatabase
		}
	default:
		return ErrMediaDatabase
	}
	bounded, cancel = context.WithTimeout(ctx, 5*time.Second)
	var raw []byte
	err = w.pool.QueryRow(bounded, `SELECT live.load_media_material($1::uuid,$2::bigint,$3::bytea)`,
		job.Args.OperationID, generation, token[:]).Scan(&raw)
	cancel()
	if err != nil {
		return ErrMediaDatabase
	}
	var fields map[string]json.RawMessage
	var m mediaLoaded
	if len(raw) > 32768 || json.Unmarshal(raw, &fields) != nil || len(fields) != 15 || json.Unmarshal(raw, &m) != nil ||
		m.Mode != mode || !mediaProjectPattern.MatchString(m.ProjectID) ||
		m.CredentialVersion < 1 || m.MaterialVersion < 1 || m.EndpointIdentity == "" ||
		!command.ValidID(m.AttemptID) || !command.ValidID(m.SessionID) ||
		!command.ValidID(m.TenantID) || !command.ValidID(m.StoreID) {
		return w.finish(ctx, job.Args.OperationID, generation, token[:], "material_invalid")
	}
	project, ok := w.projects[mediaProjectKey{m.ProjectID, m.CredentialVersion}]
	if !ok || project.identity != m.EndpointIdentity {
		return w.finish(ctx, job.Args.OperationID, generation, token[:], "credential_unavailable")
	}
	if mode == "dispatch" {
		if m.EgressID != "" || m.KeyID == "" || m.NonceHex == "" || m.CiphertextHex == "" {
			return w.finish(ctx, job.Args.OperationID, generation, token[:], "material_invalid")
		}
		nonce, nErr := hex.DecodeString(m.NonceHex)
		cipher, cErr := hex.DecodeString(m.CiphertextHex)
		if nErr != nil || cErr != nil {
			return w.finish(ctx, job.Args.OperationID, generation, token[:], "material_invalid")
		}
		input, openErr := w.keys.Open(livekit.MaterialScope{TenantID: m.TenantID, StoreID: m.StoreID,
			SessionID: m.SessionID, AttemptID: m.AttemptID, ProjectID: m.ProjectID,
			CredentialVersion: m.CredentialVersion, MaterialVersion: m.MaterialVersion}, project.client,
			livekit.SealedMaterial{KeyID: m.KeyID, Nonce: nonce, Ciphertext: cipher})
		if openErr != nil || input.RoomName != m.RoomName || input.AspectRatio != m.AspectRatio {
			return w.finish(ctx, job.Args.OperationID, generation, token[:], "material_invalid")
		}
		if err := w.call(ctx, `SELECT live.reserve_media_start($1::uuid,$2::bigint,$3::bytea)`,
			job.Args.OperationID, generation, token[:]); err != nil {
			return err
		}
		obs, startErr := project.client.Start(ctx, input)
		if startErr != nil {
			return w.finish(ctx, job.Args.OperationID, generation, token[:], "remote_unknown")
		}
		return w.record(ctx, job.Args.OperationID, generation, token[:], "START", obs)
	}
	if m.KeyID != "" || m.NonceHex != "" || m.CiphertextHex != "" {
		return w.finish(ctx, job.Args.OperationID, generation, token[:], "material_invalid")
	}
	var obs livekit.Observation
	var observeErr error
	source := "ROOM"
	if m.EgressID == "" {
		obs, observeErr = project.client.FindByRoom(ctx, m.RoomName)
	} else {
		source = "QUERY"
		obs, observeErr = project.client.Query(ctx, livekit.Target{RoomName: m.RoomName, EgressID: m.EgressID})
	}
	if observeErr != nil {
		code := "remote_unknown"
		if errors.Is(observeErr, livekit.ErrNotObserved) {
			code = "not_observed"
		}
		if errors.Is(observeErr, livekit.ErrInvalid) {
			code = "invalid_observation"
		}
		return w.finish(ctx, job.Args.OperationID, generation, token[:], code)
	}
	if source == "QUERY" {
		result, err := w.recordCleanupQuery(ctx, job.Args.OperationID, generation, token[:], obs)
		if err != nil {
			return err
		}
		switch result {
		case "terminal", "escalated":
			return nil
		case "observe":
			return river.JobSnooze(mediaObservationDelay)
		case "stop_reserved":
			// The reservation was committed with this exact lease and target.
			// A cancelled context consumes budget but never sends detached I/O.
			if err := ctx.Err(); err != nil {
				return err
			}
			stopObservation, stopErr := project.client.Stop(ctx, livekit.Target{
				RoomName: m.RoomName, EgressID: m.EgressID,
			})
			if stopErr != nil {
				return w.finish(ctx, job.Args.OperationID, generation, token[:], "remote_unknown")
			}
			return w.record(ctx, job.Args.OperationID, generation, token[:], "STOP", stopObservation)
		}
		return ErrMediaDatabase
	}
	return w.record(ctx, job.Args.OperationID, generation, token[:], source, obs)
}
