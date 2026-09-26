package meta

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

var ErrInboxStorage = errors.New("meta: inbox unavailable")

const inboxQueue = "meta_inbox"

// Inbox borrows its pool. Only NewInboxHandler can admit verified webhook bytes.
type Inbox struct {
	pool *pgxpool.Pool
	keys *PayloadKeyring
	jobs *river.Client[pgx.Tx]
}

type inboxJobArgs struct {
	EventID string `json:"event_id"`
	Version int    `json:"version"`
}

func (inboxJobArgs) Kind() string { return "meta_inbox_v1" }

func (Inbox) String() string               { return "meta.Inbox{redacted}" }
func (i Inbox) GoString() string           { return i.String() }
func (Inbox) MarshalJSON() ([]byte, error) { return []byte(`"meta.Inbox{redacted}"`), nil }

func NewInbox(ctx context.Context, pool *pgxpool.Pool, keys *PayloadKeyring) (*Inbox, error) {
	if ctx == nil || pool == nil || keys == nil || !validPayloadKeyID(keys.activeID) ||
		len(keys.keys) < 1 || len(keys.keys) > 16 {
		return nil, ErrConfig
	}
	if active, ok := keys.keys[keys.activeID]; !ok || active == ([32]byte{}) {
		return nil, ErrConfig
	}
	if err := platform.ValidateMetaIngressPool(ctx, pool); err != nil {
		return nil, ErrConfig
	}
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: "river"})
	if err != nil {
		return nil, ErrConfig
	}
	return &Inbox{pool: pool, keys: keys, jobs: jobs}, nil
}

func NewInboxHandler(v *Verifier, inbox *Inbox) (http.Handler, error) {
	if !v.valid() || inbox == nil || inbox.pool == nil || inbox.keys == nil || inbox.jobs == nil {
		return nil, ErrConfig
	}
	return newRawHandler(v, func(ctx context.Context, batch Batch, raw []byte) error {
		if batch.AppID != v.appID || batch.Object != v.object {
			return ErrInboxStorage
		}
		return inbox.commit(ctx, batch, raw)
	})
}

func (i *Inbox) commit(ctx context.Context, batch Batch, raw []byte) error {
	if i == nil || i.pool == nil || i.keys == nil || i.jobs == nil || ctx == nil ||
		len(raw) < 1 || len(raw) > maxBody || digest(raw) != batch.BodyHash ||
		!digits(batch.AppID) || (batch.Object != "page" && batch.Object != "instagram") ||
		len(batch.Events) < 1 || len(batch.Events) > maxEvents {
		return ErrInboxStorage
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := i.pool.BeginTx(bounded, pgx.TxOptions{})
	if err != nil {
		return ErrInboxStorage
	}
	committed := false
	defer func() {
		if !committed {
			cleanup, done := context.WithTimeout(context.Background(), 2*time.Second)
			defer done()
			_ = tx.Rollback(cleanup)
		}
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','9s',true),
		set_config('lock_timeout','2s',true),set_config('idle_in_transaction_session_timeout','10s',true)`); err != nil {
		return ErrInboxStorage
	}
	var batchID string
	var replay bool
	if err = tx.QueryRow(bounded, `SELECT batch_id::text,replay FROM meta_inbox.begin_batch($1,$2,$3,$4)`,
		batch.AppID, batch.Object, batch.BodyHash, len(batch.Events)).Scan(&batchID, &replay); err != nil || !command.ValidID(batchID) {
		return ErrInboxStorage
	}
	if replay {
		if err = tx.Commit(bounded); err != nil {
			return ErrInboxStorage
		}
		committed = true
		return nil
	}

	// SQL takes key-level locks; all transactions take them in the same order.
	for _, position := range sortedEventOrder(batch.Events) {
		event := batch.Events[position]
		if !validPayloadHash(event.Key) || !validPayloadHash(event.PayloadHash) ||
			len(event.Payload) < 1 || len(event.Payload) > maxPayload || digest(event.Payload) != event.PayloadHash {
			return ErrInboxStorage
		}
		var eventID string
		var needsBody bool
		var bodyClass *string
		var tenantID, storeID, routeID *string
		var routeEpoch *int64
		err = tx.QueryRow(bounded, `SELECT event_id::text,needs_body,body_class,
			tenant_id::text,store_id::text,route_id::text,route_epoch
			FROM meta_inbox.prepare_event($1::uuid,$2::int,$3::text,$4::text,$5::text,$6::text,$7::text,$8::timestamptz)`,
			batchID, position+1, event.Key, event.PayloadHash, event.AssetID,
			event.Kind, event.QuarantineReason, event.OccurredAt).
			Scan(&eventID, &needsBody, &bodyClass, &tenantID, &storeID, &routeID, &routeEpoch)
		if err != nil || !command.ValidID(eventID) {
			return ErrInboxStorage
		}
		if !needsBody {
			continue
		}
		if bodyClass == nil {
			return ErrInboxStorage
		}
		payloadScope := payloadContext{Class: *bodyClass, ID: eventID, AppID: batch.AppID,
			Object: batch.Object, EventKey: event.Key, PayloadHash: event.PayloadHash}
		var jobID any
		switch *bodyClass {
		case "event":
			if event.QuarantineReason != "" || tenantID == nil || storeID == nil || routeID == nil || routeEpoch == nil {
				return ErrInboxStorage
			}
			payloadScope.TenantID, payloadScope.StoreID = *tenantID, *storeID
			payloadScope.RouteID, payloadScope.RouteEpoch = *routeID, *routeEpoch
		case "quarantine":
			if tenantID != nil || storeID != nil || routeID != nil || (routeEpoch != nil && *routeEpoch != 0) {
				return ErrInboxStorage
			}
		default:
			return ErrInboxStorage
		}
		sealed, sealErr := i.keys.seal(payloadScope, event.Payload)
		if sealErr != nil {
			return ErrInboxStorage
		}
		if *bodyClass == "event" {
			job, insertErr := i.jobs.InsertTx(bounded, tx, inboxJobArgs{EventID: eventID, Version: 1},
				&river.InsertOpts{Queue: inboxQueue})
			if insertErr != nil || job == nil || job.Job == nil {
				return ErrInboxStorage
			}
			jobID = job.Job.ID
		}
		if _, err = tx.Exec(bounded, `SELECT meta_inbox.complete_event($1::uuid,$2::uuid,$3::text,$4::bytea,$5::bytea,$6::bigint)`,
			batchID, eventID, sealed.KeyID, sealed.Nonce, sealed.Ciphertext, jobID); err != nil {
			return ErrInboxStorage
		}
	}
	rawBody, err := i.keys.seal(payloadContext{Class: "raw", ID: batchID, AppID: batch.AppID,
		Object: batch.Object, BodyHash: batch.BodyHash}, raw)
	if err != nil {
		return ErrInboxStorage
	}
	if _, err = tx.Exec(bounded, `SELECT meta_inbox.complete_batch($1::uuid,$2::text,$3::bytea,$4::bytea)`,
		batchID, rawBody.KeyID, rawBody.Nonce, rawBody.Ciphertext); err != nil {
		return ErrInboxStorage
	}
	if err = tx.Commit(bounded); err != nil {
		return ErrInboxStorage
	}
	committed = true
	return nil
}

func sortedEventOrder(events []Event) []int {
	order := make([]int, len(events))
	for n := range order {
		order[n] = n
	}
	sort.Slice(order, func(a, b int) bool {
		x, y := events[order[a]], events[order[b]]
		if x.Key != y.Key {
			return x.Key < y.Key
		}
		if x.PayloadHash != y.PayloadHash {
			return x.PayloadHash < y.PayloadHash
		}
		return order[a] < order[b]
	})
	return order
}
