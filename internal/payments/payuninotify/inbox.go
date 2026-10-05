// inbox.go: the PG side of PAYUNi notify admission. The store never sees an unverified callback:
// record takes only a payuni.NotificationAuth that the handler already authenticated with the
// connection's own HashKey/HashIV, and it writes no money fact — it records a receipt and wakes the
// attempt's existing query job (a second query job cannot exist: payment_job_queue pins each
// attempt to one job by a.job_id=j.id, see migrations/0136).

package payuninotify

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/integrations/accounts"
	"livecommerce/internal/integrations/psp/payuni"
	"livecommerce/internal/platform"
)

// ErrConfig: constructor input is invalid. ErrDatabase: PG refused; nothing was ACKed.
var (
	ErrConfig   = errors.New("payuninotify: invalid config")
	ErrDatabase = errors.New("payuninotify: database unavailable")
)

// dbTimeout is the per-transaction budget; the whole request has 5 s (requestBudget).
const dbTimeout = 2 * time.Second

// material is what payments.payuni_resolve_endpoint returns for one enabled endpoint token:
// the scope plus the current credential envelope and (for the rotation grace window) the previous
// one. No plaintext key is ever returned; the handler decrypts in Go with the keyring.
type material struct {
	TenantID, StoreID, ConnectionID, Environment, AccountID, Profile string
	CurVersion                                                       int64
	CurKeyID                                                         string
	CurNonce, CurCiphertext                                          []byte
	PrevVersion                                                      *int64
	PrevKeyID                                                        *string
	PrevNonce, PrevCiphertext                                        []byte
}

// receipt is the record outcome; disposition is QUEUED, MISMATCH, UNKNOWN_TRADE or DUPLICATE.
type receipt struct {
	Disposition string
	ReceiptID   string
	AttemptID   string
	TenantID    string
	StoreID     string
	JobID       int64
}

// store is the PG seam. pgStore is the only production implementation; the interface exists so
// handler logic (ordering, status codes, no-ACK rules) is unit-testable without a database.
type store interface {
	material(ctx context.Context, tokenHash []byte) (m material, found bool, err error)
	record(ctx context.Context, tokenHash, payloadSHA256 []byte, auth payuni.NotificationAuth) (receipt, error)
}

// Inbox borrows the ingress pool; closing it stays the caller's job.
type Inbox struct {
	store   store
	keys    *accounts.Keyring // signing keyring only; never the payment API-key keyring's plaintext
	profile string
	now     func() time.Time
}

func (Inbox) String() string               { return "payuninotify.Inbox{redacted}" }
func (i Inbox) GoString() string           { return i.String() }
func (Inbox) MarshalJSON() ([]byte, error) { return []byte(`"payuninotify.Inbox{redacted}"`), nil }

// NewInbox validates the borrowed ingress pool (platform.ValidatePayuniIngressPool: no merchant or
// worker pool may be substituted). profile is the only profile this process admits and is limited
// to PROVIDER_MOCK|SANDBOX — LIVE is never admitted for PAYUNi notify.
func NewInbox(ctx context.Context, ingressPool *pgxpool.Pool, signingKeys *accounts.Keyring,
	profile string) (*Inbox, error) {
	if ctx == nil || ingressPool == nil || signingKeys == nil || !validProfile(profile) {
		return nil, ErrConfig
	}
	if err := platform.ValidatePayuniIngressPool(ctx, ingressPool); err != nil {
		return nil, ErrDatabase
	}
	return &Inbox{store: &pgStore{pool: ingressPool}, keys: signingKeys, profile: profile, now: time.Now}, nil
}

type pgStore struct {
	pool *pgxpool.Pool
}

func (s *pgStore) material(ctx context.Context, tokenHash []byte) (material, bool, error) {
	bounded, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	var m material
	// payments.payuni_resolve_endpoint (integration_writer definer, EXECUTE ingress only): returns
	// one enabled endpoint's scope and current/previous credential envelopes; the token hash is not
	// tenant authority.
	rows, err := s.pool.Query(bounded, `SELECT tenant_id::text,store_id::text,connection_id::text,environment,
		account_id,execution_profile,cur_version,cur_key_id,cur_nonce,cur_ciphertext,
		prev_version,prev_key_id,prev_nonce,prev_ciphertext
		FROM payments.payuni_resolve_endpoint($1::bytea)`, tokenHash)
	if err != nil {
		return m, false, ErrDatabase
	}
	defer rows.Close()
	if !rows.Next() {
		return m, false, errOrNil(rows.Err())
	}
	if err := rows.Scan(&m.TenantID, &m.StoreID, &m.ConnectionID, &m.Environment, &m.AccountID,
		&m.Profile, &m.CurVersion, &m.CurKeyID, &m.CurNonce, &m.CurCiphertext,
		&m.PrevVersion, &m.PrevKeyID, &m.PrevNonce, &m.PrevCiphertext); err != nil {
		return m, false, ErrDatabase
	}
	rows.Close()
	return m, true, errOrNil(rows.Err())
}

func errOrNil(err error) error {
	if err != nil {
		return ErrDatabase
	}
	return nil
}

func (s *pgStore) record(ctx context.Context, tokenHash, payloadSHA256 []byte, auth payuni.NotificationAuth) (r receipt, err error) {
	bounded, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()
	tx, err := s.pool.BeginTx(bounded, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return r, ErrDatabase
	}
	done := false
	defer func() {
		if !done {
			cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
			defer stop()
			_ = tx.Rollback(cleanup)
		}
	}()
	// The definer re-resolves the endpoint FOR SHARE and does the whole receipt in one transaction;
	// statement_timeout is only the server-side backstop inside the same 2 s.
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','1800ms',true),
		set_config('idle_in_transaction_session_timeout','3s',true)`); err != nil {
		return r, ErrDatabase
	}
	var attemptID, tenantID, storeID *string
	var jobID *int64
	// payments.payuni_record_notify (integration_writer definer): dedups per
	// (connection_id,payload_sha256), maps MerTradeNo to the connection's attempt and either wakes
	// the existing query job (QUEUED), opens a NOTIFY_MISMATCH review case (MISMATCH), or records a
	// receipt only (UNKNOWN_TRADE). DUPLICATE returns the original receipt and bumps redelivery.
	if err = tx.QueryRow(bounded, `SELECT disposition,receipt_id::text,attempt_id::text,tenant_id::text,store_id::text,job_id
		FROM payments.payuni_record_notify($1::bytea,$2::bytea,$3::text,$4::text,$5::bigint,$6::text,$7::text)`,
		tokenHash, payloadSHA256, auth.MerTradeNo, auth.TradeNo, auth.AmountTWD, auth.Status, auth.TradeStatus).
		Scan(&r.Disposition, &r.ReceiptID, &attemptID, &tenantID, &storeID, &jobID); err != nil {
		return r, ErrDatabase
	}
	if attemptID != nil {
		r.AttemptID = *attemptID
	}
	if tenantID != nil {
		r.TenantID = *tenantID
	}
	if storeID != nil {
		r.StoreID = *storeID
	}
	if jobID != nil {
		r.JobID = *jobID
	}
	if err = tx.Commit(bounded); err != nil {
		return r, ErrDatabase
	}
	done = true
	return r, nil
}

func validProfile(profile string) bool {
	return profile == "PROVIDER_MOCK" || profile == "SANDBOX"
}
