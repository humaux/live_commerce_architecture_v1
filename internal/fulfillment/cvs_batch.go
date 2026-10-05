// cvs_batch.go owns the batch label-request command (contract amendment W3-02B §3): one client
// Idempotency-Key replays the whole batch through command.Run, and each order reuses the existing
// single-order entry (Request -> requestOne -> fulfillment.request_cvs_shipment, the same SQL plan
// function and the same external_operation_v1 River kind).
//
// Why per-order top-level transactions instead of one transaction with per-order savepoints (the
// brief's literal wording): integration.plan_cvs_create verifies the River job by
// r.xmin = pg_current_xact_id() (0073:1046-1048, the same check as 0064:861-864). A row INSERTed
// inside a SAVEPOINT carries the sub-transaction XID as its xmin, while pg_current_xact_id() always
// returns the TOP-level XID, so every pickable order would be refused as 22023 invalid cvs plan.
// Reusing the single-order transaction keeps the job's xmin top-level and lets a per-order refusal
// roll the whole per-order transaction back orphan-free (post_river/0014 refuses to commit an orphan
// job). Each per-order Request is therefore its own short READ COMMITTED transaction, serialised by
// the outer command.Run advisory lock; no new River kind and no concurrent ECPay dispatch (the River
// queue serialises as today).

package fulfillment

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

const cvsBatchMaxOrders = 100

// batchBudget is the outer command's idle-in-transaction budget: the outer transaction only holds the
// command.Run advisory lock while each order runs in its own short transaction, so it must not trip
// WithScope's 5 s interactive idle timeout across up to 100 per-order round trips.
const batchBudget = 60 * time.Second

// BatchInput is the exact POST body of .../shipments/cvs-batch.
type BatchInput struct {
	OrderIDs []string `json:"order_ids"`
}

// BatchResultItem is one order's outcome. queued = the single-order entry planned the request and
// queued the River job; failed carries the refusal code; already is reserved (the single-order SQL
// refuses a live attempt as not_shippable, and the aggregate replay returns the stored result whole).
type BatchResultItem struct {
	OrderID string  `json:"order_id"`
	Outcome string  `json:"outcome"` // queued | already | failed
	Code    *string `json:"code,omitempty"`
}

// BatchResult is the 200 body; the order of Results matches the input order_ids.
type BatchResult struct {
	Results []BatchResultItem `json:"results"`
}

// Batch plans one label request per order. Same key + same order_ids replays the stored result without
// inserting any new River job (PL06).
func (c *CVS) Batch(ctx context.Context, token, storeID, key string, in BatchInput) (BatchResult, error) {
	if !cvsKey.MatchString(key) || len(in.OrderIDs) == 0 || len(in.OrderIDs) > cvsBatchMaxOrders {
		return BatchResult{}, command.ErrInvalid
	}
	for _, id := range in.OrderIDs {
		if !command.ValidID(id) {
			return BatchResult{}, command.ErrInvalid
		}
	}
	if !c.cfg.ECPay.Enabled {
		return BatchResult{}, &CVSError{Status: 422, Code: "connection_unavailable"}
	}
	var out BatchResult
	err := platform.WithScopeBudget(ctx, c.pool, token, storeID, "fulfillment:write", batchBudget,
		func(tx pgx.Tx, s platform.Scope) error {
			return command.Run(ctx, tx, s, "fulfillment.cvs_batch", key, in, &out, func() error {
				out.Results = make([]BatchResultItem, 0, len(in.OrderIDs))
				for _, orderID := range in.OrderIDs {
					item, err := c.batchOne(ctx, token, storeID, orderID)
					if err != nil {
						return err
					}
					out.Results = append(out.Results, item)
				}
				return nil
			})
		})
	return out, mapCVSError(err)
}

// batchOne runs one order through the existing single-order entry in its OWN top-level transaction
// (see the file header for why a savepoint cannot work). A nil error is final (queued or a coded
// failed); a non-nil error is fatal and aborts the whole batch.
func (c *CVS) batchOne(ctx context.Context, token, storeID, orderID string) (BatchResultItem, error) {
	// The per-order idempotency key is fresh per attempt: the aggregate batch command (command.Run
	// above) is the only replay authority, so a replayed batch never re-enters this loop. Expected
	// version 0 = no CAS, the same shape the batch has no per-order version for.
	perKey, err := newUUIDv4()
	if err != nil {
		return BatchResultItem{}, ErrCVSProjection
	}
	if _, err := c.Request(ctx, token, storeID, perKey, orderID, RequestInput{ExpectedVersion: 0}); err != nil {
		code, ok := batchRefusalCode(err)
		if !ok {
			return BatchResultItem{}, err
		}
		return BatchResultItem{OrderID: orderID, Outcome: "failed", Code: &code}, nil
	}
	return BatchResultItem{OrderID: orderID, Outcome: "queued"}, nil
}

// batchRefusalCode maps a per-order refusal to the batch's code table. no_cvs_destination is renamed
// not_cvs (PL06); every other coded refusal keeps its contract code. ok=false means the error is fatal.
func batchRefusalCode(err error) (string, bool) {
	var refusal *CVSError
	if errors.As(err, &refusal) {
		if refusal.Code == "no_cvs_destination" {
			return "not_cvs", true
		}
		return refusal.Code, true
	}
	switch {
	case errors.Is(err, command.ErrNotFound):
		return "order_not_found", true
	case errors.Is(err, command.ErrInvalid):
		return "invalid_order", true
	}
	return "", false
}
