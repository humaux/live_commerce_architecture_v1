package meta

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const consumerTimeout = 5 * time.Second

var (
	ErrConsumerJob     = errors.New("meta: invalid consumer job")
	ErrConsumerPolicy  = errors.New("meta: consumer policy unavailable")
	ErrConsumerStorage = errors.New("meta: consumer storage unavailable")
)

// ConsumerWorker borrows a dedicated consumer pool. River job lifecycle runs
// under a separately validated ordinary worker pool.
type ConsumerWorker struct {
	river.WorkerDefaults[inboxJobArgs]
	pool *pgxpool.Pool
	keys *PayloadKeyring
}

var _ river.Worker[inboxJobArgs] = (*ConsumerWorker)(nil)

func (ConsumerWorker) String() string     { return "meta.ConsumerWorker{redacted}" }
func (w ConsumerWorker) GoString() string { return w.String() }
func (ConsumerWorker) MarshalJSON() ([]byte, error) {
	return []byte(`"meta.ConsumerWorker{redacted}"`), nil
}

func NewConsumerWorker(ctx context.Context, pool *pgxpool.Pool, keys *PayloadKeyring) (*ConsumerWorker, error) {
	if ctx == nil || pool == nil || keys == nil || !validPayloadKeyID(keys.activeID) ||
		len(keys.keys) < 1 || len(keys.keys) > 16 {
		return nil, ErrConfig
	}
	if active, ok := keys.keys[keys.activeID]; !ok || active == ([32]byte{}) {
		return nil, ErrConfig
	}
	if err := platform.ValidateMetaConsumerPool(ctx, pool); err != nil {
		return nil, ErrConfig
	}
	return &ConsumerWorker{pool: pool, keys: keys}, nil
}

func (*ConsumerWorker) Timeout(*river.Job[inboxJobArgs]) time.Duration { return consumerTimeout }

type socialLoaded struct {
	outcome     string
	appID       *string
	object      *string
	assetID     *string
	kind        *string
	eventKey    *string
	payloadHash *string
	tenantID    *string
	storeID     *string
	routeID     *string
	routeEpoch  *int64
	keyID       *string
	nonce       []byte
	ciphertext  []byte
}

func (r socialLoaded) empty() bool {
	return r.appID == nil && r.object == nil && r.assetID == nil && r.kind == nil &&
		r.eventKey == nil && r.payloadHash == nil && r.tenantID == nil && r.storeID == nil &&
		r.routeID == nil && r.routeEpoch == nil && r.keyID == nil && r.nonce == nil && r.ciphertext == nil
}

func (r socialLoaded) ready() bool {
	return r.appID != nil && r.object != nil && r.assetID != nil && r.kind != nil &&
		r.eventKey != nil && r.payloadHash != nil && r.tenantID != nil && r.storeID != nil &&
		r.routeID != nil && r.routeEpoch != nil && r.keyID != nil && len(r.nonce) > 0 && len(r.ciphertext) > 0
}

func consumerDatabaseError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22023" {
		return river.JobCancel(ErrConsumerJob)
	}
	return ErrConsumerStorage
}

func (w *ConsumerWorker) Work(ctx context.Context, job *river.Job[inboxJobArgs]) error {
	if job == nil || job.JobRow == nil || job.ID <= 0 || job.Attempt <= 0 ||
		job.Args.Version != 1 || !command.ValidID(job.Args.EventID) ||
		job.Kind != (inboxJobArgs{}).Kind() || job.Queue != inboxQueue {
		return river.JobCancel(ErrConsumerJob)
	}
	if ctx == nil || w == nil || w.pool == nil || w.keys == nil {
		return ErrConsumerStorage
	}
	bounded, cancel := context.WithTimeout(ctx, consumerTimeout)
	defer cancel()
	tx, err := w.pool.BeginTx(bounded, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ErrConsumerStorage
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','5s',true),
		set_config('lock_timeout','1s',true),set_config('idle_in_transaction_session_timeout','5s',true)`); err != nil {
		return consumerDatabaseError(err)
	}
	var loaded socialLoaded
	err = tx.QueryRow(bounded, `SELECT outcome,app_id,object,asset_id,kind,event_key,payload_hash,
		tenant_id::text,store_id::text,route_id::text,route_epoch,key_id,nonce,ciphertext
		FROM meta_inbox.load_social_event($1::uuid,$2::bigint,$3::integer)`,
		job.Args.EventID, job.ID, job.Attempt).Scan(
		&loaded.outcome, &loaded.appID, &loaded.object, &loaded.assetID, &loaded.kind,
		&loaded.eventKey, &loaded.payloadHash, &loaded.tenantID, &loaded.storeID,
		&loaded.routeID, &loaded.routeEpoch, &loaded.keyID, &loaded.nonce, &loaded.ciphertext)
	if err != nil {
		return consumerDatabaseError(err)
	}
	switch loaded.outcome {
	case "ALREADY", "REVIEWED", "STALE":
		if !loaded.empty() {
			return ErrConsumerStorage
		}
		if err = tx.Commit(bounded); err != nil {
			return consumerDatabaseError(err)
		}
		if loaded.outcome == "ALREADY" {
			return nil
		}
		return river.JobCancel(ErrConsumerPolicy)
	case "READY":
		if !loaded.ready() {
			return ErrConsumerStorage
		}
	default:
		return ErrConsumerStorage
	}
	scope := payloadContext{Class: "event", ID: job.Args.EventID, AppID: *loaded.appID,
		Object: *loaded.object, EventKey: *loaded.eventKey, PayloadHash: *loaded.payloadHash,
		TenantID: *loaded.tenantID, StoreID: *loaded.storeID, RouteID: *loaded.routeID,
		RouteEpoch: *loaded.routeEpoch}
	plaintext, err := w.keys.open(scope, sealedPayload{KeyID: *loaded.keyID,
		Nonce: loaded.nonce, Ciphertext: loaded.ciphertext})
	if err != nil {
		return ErrConsumerPayload
	}
	projection, err := projectSocial(scope, *loaded.assetID, *loaded.kind, plaintext)
	if err != nil {
		return ErrConsumerPayload
	}
	if _, err = tx.Exec(bounded, `SELECT meta_inbox.finish_social_event($1::uuid,$2::bigint,$3::integer,$4::text,$5::text)`,
		job.Args.EventID, job.ID, job.Attempt, projection.family, projection.subjectKey); err != nil {
		return consumerDatabaseError(err)
	}
	if err = tx.Commit(bounded); err != nil {
		return consumerDatabaseError(err)
	}
	return nil
}
