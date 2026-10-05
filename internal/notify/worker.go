package notify

// worker.go is the delivery loop: claim rows (SQL decides what is due and within which caps), render, ONE SMTP attempt per message, record.
// It hosts no policy of its own: backoff, the 3-attempt ceiling, the 24 h staleness and the caps all live in notify.claim_batch /
// notify.record_result (migration 0090). Logged: kind, order id and counts only, never an address, a body or an SMTP reply.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"livecommerce/internal/mail"
)

// Mailer sends one message once. *mail.SMTP satisfies it; tests use a fake.
type Mailer interface {
	Send(ctx context.Context, m mail.Message) (reply string, err error)
}

const (
	claimLimit    = 20               // rows per claim; a pass repeats while a claim comes back full
	maxPasses     = 5                // bounds one tick so a backlog cannot starve shutdown
	sendTimeout   = 25 * time.Second // one SMTP attempt (same bound as the identity async sends)
	recordTimeout = 10 * time.Second
	storeHourly   = 30 // §E3: buyer mails per store per rolling hour
	// DefaultEvery is the polling interval of Run: a buyer mail leaves within about this long after the order moved.
	DefaultEvery = 15 * time.Second
)

// queue is the database side of the loop (pgQueue in production, a fake in unit tests).
type queue interface {
	Claim(ctx context.Context, limit int) ([]Payload, error)
	Record(ctx context.Context, batch, state string, recipientHash []byte) error
}

// Worker drains notify.outbox (buyer/merchant-new) or notify.merchant_alerts (meta health). Construct with NewWorker or
// NewMerchantWorker; Run until the context ends.
type Worker struct {
	q     queue
	m     Mailer
	every time.Duration
	// adminOrigin is the platform admin origin the meta_health CTA links to (contract meta-connection-health-v1 §10: the
	// admin origin, never a storefront or Meta URL). Empty for the buyer worker.
	adminOrigin string
}

// NewWorker binds the commerce_expiry_worker pool and the SMTP sender. dailyCap is COMMERCE_MAIL_DAILY_CAP: notify mail may use 60% of it (§E3).
func NewWorker(pool *pgxpool.Pool, m Mailer, dailyCap int) (*Worker, error) {
	if pool == nil || m == nil || dailyCap < 20 || dailyCap > 100000 {
		return nil, errors.New("notify: pool, mailer and a daily cap of 20..100000 required")
	}
	return &Worker{q: pgQueue{pool: pool, budget: dailyCap * 60 / 100}, m: m, every: DefaultEvery}, nil
}

// NewMerchantWorker drains notify.merchant_alerts (the meta connection-health owner mail, migration 0125 §5.2) through
// notify.claim_merchant_alerts / notify.record_merchant_alert, sharing the 60% notify budget. adminOrigin is the admin
// origin the reconnect CTA points at (https only, §10); it is injected into every meta_health payload, never read from SQL.
func NewMerchantWorker(pool *pgxpool.Pool, m Mailer, dailyCap int, adminOrigin string) (*Worker, error) {
	if pool == nil || m == nil || dailyCap < 20 || dailyCap > 100000 {
		return nil, errors.New("notify: pool, mailer and a daily cap of 20..100000 required")
	}
	if adminOrigin == "" || !strings.HasPrefix(adminOrigin, "https://") || strings.ContainsAny(adminOrigin, " \r\n\"<>") {
		return nil, errors.New("notify: admin origin must be an https origin")
	}
	return &Worker{q: merchantQueue{pool: pool, budget: dailyCap * 60 / 100}, m: m, every: DefaultEvery, adminOrigin: adminOrigin}, nil
}

// merchantQueue is the merchant_alerts side of the loop (the meta_health owner mail); the record key is the alert id.
type merchantQueue struct {
	pool   *pgxpool.Pool
	budget int
}

// Claim calls notify.claim_merchant_alerts (commerce_expiry_worker): housekeeping, then up to limit PENDING alerts under the 60% budget.
func (q merchantQueue) Claim(ctx context.Context, limit int) ([]Payload, error) {
	var raw []byte
	if err := q.pool.QueryRow(ctx, `SELECT notify.claim_merchant_alerts($1::integer,$2::integer)`, limit, q.budget).Scan(&raw); err != nil {
		return nil, err
	}
	var out []Payload
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Record calls notify.record_merchant_alert (commerce_expiry_worker) on a context that survives shutdown, so a send that happened is always written down.
func (q merchantQueue) Record(ctx context.Context, batch, state string, recipientHash []byte) error {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	_, err := q.pool.Exec(rctx, `SELECT notify.record_merchant_alert($1::uuid,$2,$3)`, batch, state, recipientHash)
	return err
}

type pgQueue struct {
	pool   *pgxpool.Pool
	budget int
}

// Claim calls notify.claim_batch (commerce_expiry_worker): housekeeping, then up to limit buyer rows and at most one merchant batch per store.
func (q pgQueue) Claim(ctx context.Context, limit int) ([]Payload, error) {
	var raw []byte
	if err := q.pool.QueryRow(ctx, `SELECT notify.claim_batch($1::integer,$2::integer,$3::integer)`, limit, q.budget, storeHourly).Scan(&raw); err != nil {
		return nil, err
	}
	var out []Payload
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Record calls notify.record_result (commerce_expiry_worker) on a context that survives shutdown, so a send that happened is always written down.
func (q pgQueue) Record(ctx context.Context, batch, state string, recipientHash []byte) error {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
	_, err := q.pool.Exec(rctx, `SELECT notify.record_result($1::uuid,$2,$3)`, batch, state, recipientHash)
	return err
}

// Run polls until ctx ends. A failing claim is logged and retried on the next tick; nothing is sent without a successful claim.
func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.every)
	defer t.Stop()
	for {
		for pass := 0; pass < maxPasses; pass++ {
			n, err := w.Once(ctx)
			if err != nil {
				slog.Warn("buyer_mail_claim_failed")
				break
			}
			if n < claimLimit {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Once performs one claim and the sends of its payloads; it returns how many payloads were claimed.
func (w *Worker) Once(ctx context.Context) (int, error) {
	batch, err := w.q.Claim(ctx, claimLimit)
	if err != nil {
		return 0, err
	}
	for _, p := range batch {
		w.deliver(ctx, p)
	}
	return len(batch), nil
}

// deliver makes the one attempt per recipient and records the aggregate: any UNKNOWN wins (never re-sent, I06), else any SENT (the others
// are not retried, so nobody gets a duplicate), else FAILED (every recipient certainly refused: the queue retries with backoff).
func (w *Worker) deliver(ctx context.Context, p Payload) {
	if p.Kind == KindMetaHealth && p.AdminURL == "" {
		p.AdminURL = w.adminOrigin // §10: the admin origin, injected here, never read from SQL
	}
	msgs := Render(p)
	var sent, unknown int
	for _, msg := range msgs {
		if ctx.Err() != nil {
			break // shutdown before this recipient: nothing dialled for it, so it is safe to leave it to a later attempt
		}
		sctx, cancel := context.WithTimeout(ctx, sendTimeout)
		_, err := w.m.Send(sctx, msg)
		cancel()
		switch {
		case err == nil:
			sent++
		case errors.Is(err, mail.ErrUnknown):
			// SMTP has no idempotency key: a second attempt could deliver twice. Recorded, surfaced once, never re-sent.
			unknown++
			slog.Warn("buyer_mail_unknown", "kind", p.Kind, "order_id", p.OrderID)
		default:
			slog.Warn("buyer_mail_failed", "kind", p.Kind, "order_id", p.OrderID)
		}
	}
	state := "FAILED"
	switch {
	case unknown > 0:
		state = "UNKNOWN"
	case sent > 0:
		state = "SENT"
	}
	if err := w.q.Record(ctx, p.BatchID, state, recipientHash(p)); err != nil {
		// The row stays SENDING; claim_batch turns it UNKNOWN after 15 minutes. Nothing is re-sent because of it.
		slog.Warn("buyer_mail_record_failed", "kind", p.Kind, "state", state)
	}
}

// recipientHash is sha256("order id : lowercase address") of the buyer address (§E7); a merchant batch (or the meta_health
// owner mail, migration 0125 §5.2) hashes its sorted owner list.
func recipientHash(p Payload) []byte {
	if len(p.To) == 0 {
		return nil
	}
	var key string
	if p.Kind == KindMerchantNew || p.Kind == KindMetaHealth {
		to := make([]string, len(p.To))
		for i, a := range p.To {
			to[i] = strings.ToLower(a)
		}
		sort.Strings(to)
		key = "merchant:" + strings.Join(to, ",")
	} else {
		key = p.OrderID + ":" + strings.ToLower(p.To[0])
	}
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}
