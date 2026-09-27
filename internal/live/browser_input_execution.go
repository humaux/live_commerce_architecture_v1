package live

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/platform"
)

const browserInputHeldDelay = 60 * time.Second

type browserInputExecutionWorker struct {
	river.WorkerDefaults[mediaOperationArgs]
	egress  *mediaExecutionWorker
	runtime *BrowserInputRuntime
}

func (*browserInputExecutionWorker) Timeout(*river.Job[mediaOperationArgs]) time.Duration {
	return 30 * time.Second
}

// NewBrowserInputMediaClient consumes only the frozen input queue. The ordinary
// Egress client and its queue remain separately configured and unchanged.
func NewBrowserInputMediaClient(ctx context.Context, workerPool, executorPool *pgxpool.Pool,
	keys *livekit.MaterialKeyring, projects []MediaProject, runtime *BrowserInputRuntime, concurrency int,
) (*river.Client[pgx.Tx], error) {
	if ctx == nil || workerPool == nil || executorPool == nil || keys == nil || runtime == nil ||
		len(runtime.projects) == 0 || concurrency < 1 || concurrency > 32 {
		return nil, ErrMediaConfig
	}
	selected, err := selectMediaProjects(projects)
	if err != nil {
		return nil, err
	}
	for key, input := range runtime.projects {
		egress, ok := selected[key]
		if !ok || egress.identity != input.identity {
			return nil, ErrMediaConfig
		}
	}
	preflight, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if platform.ValidateMediaWorkerPool(preflight, workerPool) != nil ||
		platform.ValidateMediaExecutorPool(preflight, executorPool) != nil ||
		platform.ValidateSameDatabase(preflight, workerPool, executorPool) != nil {
		return nil, ErrMediaDatabase
	}
	var ready bool
	if executorPool.QueryRow(preflight, `SELECT live.media_browser_input_worker_ready()`).Scan(&ready) != nil || !ready {
		return nil, ErrMediaDatabase
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &browserInputExecutionWorker{
		egress: &mediaExecutionWorker{pool: executorPool, keys: keys, projects: selected}, runtime: runtime,
	})
	client, err := river.NewClient(riverpgxv5.New(workerPool), &river.Config{
		Schema: mediaSchema, Workers: workers,
		Queues: map[string]river.QueueConfig{mediaInputMockQueue: {MaxWorkers: concurrency}},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		return nil, ErrMediaDatabase
	}
	return client, nil
}

type browserInputCustody struct {
	AttemptID         string `json:"attempt_id"`
	OperationID       string `json:"operation_id"`
	SessionID         string `json:"session_id"`
	ExecutionProfile  string `json:"execution_profile"`
	State             string `json:"state"`
	RoomName          string `json:"room_name"`
	PublisherIdentity string `json:"publisher_identity"`
	ProjectID         string `json:"project_id"`
	EndpointIdentity  string `json:"endpoint_identity"`
	CredentialVersion int64  `json:"credential_version"`
	SessionVersion    int64  `json:"session_version"`
	IssuedAt          int64  `json:"issued_at"`
	ExpiresAt         int64  `json:"expires_at"`
	StartBefore       int64  `json:"start_before"`
	LifetimeDeadline  int64  `json:"lifetime_deadline"`
	AdmissionClosed   bool   `json:"admission_closed"`
	CloseReason       string `json:"close_reason"`
	EgressState       string `json:"egress_state"`
	WireReserved      bool   `json:"wire_reserved"`
	Held              bool   `json:"held"`
}

type browserInputCleanupStep struct {
	Ordinal           int    `json:"ordinal"`
	Action            string `json:"action"`
	RoomName          string `json:"room_name"`
	PublisherIdentity string `json:"publisher_identity"`
	ProjectID         string `json:"project_id"`
	EndpointIdentity  string `json:"endpoint_identity"`
	CredentialVersion int64  `json:"credential_version"`
	RevokeBefore      *int64 `json:"revoke_before"`
}

func (w *browserInputExecutionWorker) queryJSON(ctx context.Context, sql string, args ...any) ([]byte, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var raw []byte
	if err := w.egress.pool.QueryRow(bounded, sql, args...).Scan(&raw); err != nil {
		return nil, ErrMediaDatabase
	}
	return raw, nil
}

func (w *browserInputExecutionWorker) finish(ctx context.Context, operation string, generation int64,
	token []byte, reason string) error {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result string
	if err := w.egress.pool.QueryRow(bounded, `SELECT live.finish_media_input_turn($1::uuid,$2::bigint,$3::bytea,$4::text)`,
		operation, generation, token, reason).Scan(&result); err != nil {
		return ErrMediaDatabase
	}
	return browserInputDisposition(result)
}

func browserInputDisposition(result string) error {
	switch result {
	case "observe":
		return river.JobSnooze(mediaObservationDelay)
	case "held":
		return river.JobSnooze(browserInputHeldDelay)
	case "terminal":
		return nil
	default:
		return ErrMediaDatabase
	}
}

func (w *browserInputExecutionWorker) Work(ctx context.Context, job *river.Job[mediaOperationArgs]) error {
	if job == nil || job.JobRow == nil || job.ID < 1 || job.Args.Version != 1 ||
		!command.ValidID(job.Args.OperationID) || job.Kind != (mediaOperationArgs{}).Kind() ||
		job.Queue != mediaInputMockQueue {
		return river.JobCancel(ErrMediaJob)
	}
	if ctx == nil || w == nil || w.egress == nil || w.egress.pool == nil || w.egress.keys == nil || w.runtime == nil {
		return ErrMediaDatabase
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return ErrMediaDatabase
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	var disposition, mode string
	var generation int64
	err := w.egress.pool.QueryRow(bounded, `SELECT disposition,generation,mode FROM live.claim_browser_input_operation(
		$1::uuid,$2::bigint,$3::integer,$4::bytea)`, job.Args.OperationID, job.ID, 30, token[:]).Scan(
		&disposition, &generation, &mode)
	cancel()
	if err != nil {
		return ErrMediaDatabase
	}
	switch disposition {
	case "kernel_only", "held":
		return river.JobSnooze(browserInputHeldDelay)
	case "await_admission", "busy":
		return river.JobSnooze(mediaObservationDelay)
	case "terminal":
		return nil
	case "claimed":
		if mode != "reconcile" {
			return ErrMediaDatabase
		}
	default:
		return ErrMediaDatabase
	}
	bounded, cancel = context.WithTimeout(ctx, 5*time.Second)
	var turn string
	err = w.egress.pool.QueryRow(bounded, `SELECT live.next_media_input_turn($1::uuid,$2::bigint,$3::bytea)`,
		job.Args.OperationID, generation, token[:]).Scan(&turn)
	cancel()
	if err != nil {
		return ErrMediaDatabase
	}
	switch turn {
	case "INPUT_OBSERVE":
		return w.observeAndStart(ctx, job.Args.OperationID, generation, token[:])
	case "INPUT_CLEANUP":
		return w.cleanup(ctx, job.Args.OperationID, generation, token[:])
	case "EGRESS":
		raw, err := w.queryJSON(ctx, `SELECT live.load_media_input_material($1::uuid,$2::bigint,$3::bytea)`,
			job.Args.OperationID, generation, token[:])
		if err != nil {
			return err
		}
		return w.egress.runLoaded(ctx, job.Args.OperationID, generation, token[:], "reconcile", raw, false)
	case "HELD":
		return w.finish(ctx, job.Args.OperationID, generation, token[:], "cleanup_unknown")
	default:
		return ErrMediaDatabase
	}
}

func (w *browserInputExecutionWorker) loadCustody(ctx context.Context, operation string, generation int64,
	token []byte) (browserInputCustody, *livekit.Client, error) {
	raw, err := w.queryJSON(ctx, `SELECT live.load_media_input_custody($1::uuid,$2::bigint,$3::bytea)`,
		operation, generation, token)
	if err != nil {
		return browserInputCustody{}, nil, err
	}
	var fields map[string]json.RawMessage
	var custody browserInputCustody
	if len(raw) > 8192 || json.Unmarshal(raw, &fields) != nil || len(fields) != 20 ||
		json.Unmarshal(raw, &custody) != nil || custody.OperationID != operation ||
		!command.ValidID(custody.AttemptID) || !command.ValidID(custody.SessionID) ||
		custody.ExecutionProfile != "LOCAL_SFU_MOCK_EGRESS" || custody.ProjectID == "" ||
		custody.CredentialVersion < 1 || custody.EndpointIdentity == "" {
		return browserInputCustody{}, nil, ErrMediaJob
	}
	endpoint, ok := w.runtime.projects[mediaProjectKey{custody.ProjectID, custody.CredentialVersion}]
	if !ok || endpoint.identity != custody.EndpointIdentity {
		return custody, nil, ErrMediaConfig
	}
	return custody, endpoint.client, nil
}

func (w *browserInputExecutionWorker) observeAndStart(ctx context.Context, operation string, generation int64,
	token []byte) error {
	custody, client, err := w.loadCustody(ctx, operation, generation, token)
	if err == ErrMediaConfig {
		return w.finish(ctx, operation, generation, token, "credential_unavailable")
	}
	if err == ErrMediaJob {
		return w.finish(ctx, operation, generation, token, "material_invalid")
	}
	if err != nil {
		return err
	}
	if custody.State != "RESERVED" || custody.AdmissionClosed || custody.WireReserved || custody.Held {
		return w.finish(ctx, operation, generation, token, "input_unknown")
	}
	obs, err := client.ObserveInput(ctx, livekit.InputTarget{RoomName: custody.RoomName, Identity: custody.PublisherIdentity})
	if err != nil {
		return w.finish(ctx, operation, generation, token, "input_unknown")
	}
	if (obs.State != "JOINED" && obs.State != "ACTIVE") || !obs.CameraPublished || obs.CameraMuted ||
		!obs.MicrophonePublished || obs.MicrophoneMuted {
		return w.finish(ctx, operation, generation, token, "input_not_ready")
	}
	// This SQL commit is the sole Start wire permission. An ACK loss returns an
	// error and does not fall through to provider I/O.
	raw, err := w.queryJSON(ctx, `SELECT live.reserve_media_input_start(
		$1::uuid,$2::bigint,$3::bytea,$4::text,$5::text,$6::text,$7::text,
		$8::boolean,$9::boolean,$10::boolean,$11::boolean)`, operation, generation, token,
		obs.RoomName, obs.Identity, obs.ParticipantID, obs.State, obs.CameraPublished, obs.CameraMuted,
		obs.MicrophonePublished, obs.MicrophoneMuted)
	if err != nil {
		return err
	}
	return w.egress.runLoaded(ctx, operation, generation, token, "dispatch", raw, false)
}

func (w *browserInputExecutionWorker) cleanup(ctx context.Context, operation string, generation int64,
	token []byte) error {
	raw, err := w.queryJSON(ctx, `SELECT live.reserve_media_input_cleanup($1::uuid,$2::bigint,$3::bytea)`,
		operation, generation, token)
	if err != nil {
		return err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return w.finish(ctx, operation, generation, token, "cleanup_unknown")
	}
	var fields map[string]json.RawMessage
	var step browserInputCleanupStep
	if len(raw) > 8192 || json.Unmarshal(raw, &fields) != nil || len(fields) != 8 ||
		json.Unmarshal(raw, &step) != nil || step.Ordinal < 1 || step.Ordinal > 8 ||
		step.ProjectID == "" || step.CredentialVersion < 1 || step.EndpointIdentity == "" {
		return ErrMediaDatabase
	}
	endpoint, ok := w.runtime.projects[mediaProjectKey{step.ProjectID, step.CredentialVersion}]
	if !ok || endpoint.identity != step.EndpointIdentity {
		// Reservation already consumed. No substitute credential or retry.
		return w.recordCleanup(ctx, operation, generation, token, step.Ordinal, "UNKNOWN", "")
	}
	target := livekit.InputTarget{RoomName: step.RoomName, Identity: step.PublisherIdentity}
	result, sid := "UNKNOWN", ""
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if ctx.Err() == nil {
		switch step.Action {
		case "REMOVE":
			if (step.Ordinal == 1 || step.Ordinal == 5) && step.RevokeBefore != nil &&
				endpoint.client.RemoveInput(callCtx, target, *step.RevokeBefore) == nil {
				result = "ACK"
			}
		case "DELETE_ROOM":
			if (step.Ordinal == 2 || step.Ordinal == 6) && step.RevokeBefore == nil &&
				endpoint.client.DeleteInputRoom(callCtx, step.RoomName) == nil {
				result = "ACK"
			}
		case "READ_PARTICIPANT":
			if (step.Ordinal == 3 || step.Ordinal == 7) && step.RevokeBefore == nil {
				if obs, err := endpoint.client.ObserveInput(callCtx, target); err == nil {
					result, sid = "PRESENT", obs.ParticipantID
				}
			}
		case "READ_ROOM":
			if (step.Ordinal == 4 || step.Ordinal == 8) && step.RevokeBefore == nil {
				if obs, err := endpoint.client.ObserveInputRoom(callCtx, step.RoomName); err == nil {
					result, sid = "PRESENT", obs.RoomID
				} else if err == livekit.ErrNotObserved {
					result = "ABSENT"
				}
			}
		default:
			return ErrMediaDatabase
		}
	}
	return w.recordCleanup(ctx, operation, generation, token, step.Ordinal, result, sid)
}

func (w *browserInputExecutionWorker) recordCleanup(ctx context.Context, operation string, generation int64,
	token []byte, ordinal int, result, sid string) error {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var disposition string
	if err := w.egress.pool.QueryRow(bounded, `SELECT live.record_media_input_cleanup(
		$1::uuid,$2::bigint,$3::bytea,$4::integer,$5::text,$6::text)`,
		operation, generation, token, ordinal, result, sid).Scan(&disposition); err != nil {
		return ErrMediaDatabase
	}
	return browserInputDisposition(disposition)
}
