package live

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const mediaMockQueue = "media_mock_v1"

var mediaKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)

type MediaStartInput struct {
	SessionID              string `json:"session_id"`
	AuthorizationID        string `json:"authorization_id"`
	ExpectedSessionVersion int64  `json:"expected_session_version"`
}

type MediaStartResult struct {
	SessionID   string `json:"session_id"`
	ProgramID   string `json:"program_id"`
	AttemptID   string `json:"attempt_id"`
	OperationID string `json:"operation_id"`
	JobID       int64  `json:"job_id"`
	RoomName    string `json:"room_name"`
	State       string `json:"state"`
}

type mediaOperationArgs struct {
	OperationID string `json:"operation_id"`
	Version     int    `json:"version"`
}

func (mediaOperationArgs) Kind() string { return "live_media_operation_v1" }

// MediaPlanner persists only an inert MOCK Start intent in a caller-owned transaction.
type MediaPlanner struct{ jobs *river.Client[pgx.Tx] }

func NewMediaPlanner(jobs *river.Client[pgx.Tx]) (*MediaPlanner, error) {
	if jobs == nil {
		return nil, command.ErrInvalid
	}
	return &MediaPlanner{jobs: jobs}, nil
}

func (p *MediaPlanner) PlanStart(ctx context.Context, tx pgx.Tx, scope platform.Scope,
	token, key string, in MediaStartInput) (out MediaStartResult, err error) {
	if ctx == nil || p == nil || p.jobs == nil || tx == nil ||
		!command.ValidID(in.SessionID) || !command.ValidID(in.AuthorizationID) ||
		in.SessionID == "00000000-0000-0000-0000-000000000000" ||
		in.AuthorizationID == "00000000-0000-0000-0000-000000000000" ||
		in.ExpectedSessionVersion < 1 || !mediaKeyPattern.MatchString(key) {
		return MediaStartResult{}, command.ErrInvalid
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return MediaStartResult{}, err
	}
	// Replay skips the command callback, but SQL still requires this revision GUC.
	if _, err := tx.Exec(ctx, `SELECT set_config('app.authz_revision',$1,true)`, strconv.FormatInt(scope.Revision, 10)); err != nil {
		return MediaStartResult{}, err
	}
	request := struct {
		PrincipalID string `json:"principal_id"`
		MediaStartInput
	}{scope.PrincipalID, in}
	err = command.Run(ctx, tx, scope, "live.media.start", key, request, &out, func() error {
		if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
			return err
		}
		var ready bool
		if err := tx.QueryRow(ctx, `SELECT live.media_plan_ready()`).Scan(&ready); err != nil {
			return err
		}
		if !ready {
			return command.ErrConflict
		}
		var operationID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&operationID); err != nil {
			return err
		}
		if !command.ValidID(operationID) || operationID == "00000000-0000-0000-0000-000000000000" {
			return command.ErrInvalid
		}
		job, err := p.jobs.InsertTx(ctx, tx, mediaOperationArgs{OperationID: operationID, Version: 1},
			&river.InsertOpts{Queue: mediaMockQueue})
		if err != nil {
			return err
		}
		hash := sha256.Sum256([]byte(token))
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT live.plan_media_start($1,$2::uuid,$3::uuid,$4::uuid,$5,$6,$7::uuid,$8)`,
			hash[:], scope.StoreID, in.SessionID, in.AuthorizationID, in.ExpectedSessionVersion,
			key, operationID, job.Job.ID).Scan(&raw); err != nil {
			return mediaPlanError(err)
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return command.ErrInvalid
		}
		return command.Audit(ctx, tx, scope, "live.media.start.planned")
	})
	if err != nil {
		return MediaStartResult{}, mediaPlanError(err)
	}
	if err := authorize(ctx, tx, scope, token, managePermission); err != nil {
		return MediaStartResult{}, err
	}
	hash := sha256.Sum256([]byte(token))
	if _, err := tx.Exec(ctx, `SELECT live.assert_media_start_login($1::bytea,$2::uuid,$3::uuid)`,
		hash[:], scope.StoreID, out.AttemptID); err != nil {
		return MediaStartResult{}, mediaPlanError(err)
	}
	return out, nil
}

func mediaPlanError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return command.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "MP400", "22001", "22023", "22P02":
			return command.ErrInvalid
		case "MP401":
			return platform.ErrUnauthorized
		case "MP403":
			return platform.ErrForbidden
		case "MP404", "23503":
			return command.ErrNotFound
		case "MP409", "23505", "23514":
			return command.ErrConflict
		}
	}
	return err
}
