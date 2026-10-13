// Purpose: resolve or explicitly resume durable merchant intents using their server-frozen request and original private owner.
// Depends on: ops.order_intents, fulfillment.order_intent_begin, command receipts/audit, claims.for_buyer_request/release.
// Used by: manual/for-buyer A16R recovery and pending recovery after tab changes or re-login.
// Invariants: I01/I02/I03/I06; absent checkout is sealed atomically, partial checkout retains the same key, no buyer secrets in replies.
package merchanttools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

func recoveryPermissions(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) error {
	for _, permission := range []string{manualPermission, "catalog:read", "orders:read"} {
		if err := platform.RequirePermission(ctx, tx, scope, token, permission); err != nil {
			return err
		}
	}
	return nil
}

// recoverIntentState keeps the private Begin lock through closure, cleanup, receipt and intent settlement COMMIT.
// A current signing secret or buyer capability lifetime never participates in the authoritative lookup.
func (m *ManualOrders) recoverIntentState(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, request attemptRequest, absenceOnly bool) (OrderRecovery, error) {
	check := func() error { return recoveryPermissions(ctx, tx, scope, token) }
	if err := check(); err != nil {
		return OrderRecovery{}, err
	}
	intent, err := readIntent(ctx, tx, scope, key)
	if err != nil {
		return OrderRecovery{}, err
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return OrderRecovery{}, command.ErrInvalid
	}
	hash := sha256.Sum256(encoded)
	if request.Kind != intent.Kind || !bytes.Equal(hash[:], intent.Hash) {
		return OrderRecovery{}, command.ErrConflict
	}
	if err = intentBeginLock(ctx, tx, scope, intent); err != nil {
		return OrderRecovery{}, err
	}
	if err = check(); err != nil {
		return OrderRecovery{}, err
	}
	begin, found, err := lookupIntentBegin(ctx, tx, token, scope.StoreID, key)
	if err != nil {
		return OrderRecovery{}, err
	}
	if err = check(); err != nil {
		return OrderRecovery{}, err
	}
	resolved := OrderRecovery{State: "not_created"}
	state := "sealed"
	if found {
		if absenceOnly {
			return OrderRecovery{}, ErrUnavailable
		}
		// Begin precedes manual source/link and A16 bundle/DM effects. Only their upper receipt proves completion.
		var orderID string
		if request.Kind == forBuyerOperation {
			var complete forBuyerCompletion
			if err = command.Run(ctx, tx, scope, forBuyerCompleteOperation, key, request.Body, &complete, func() error { return ErrUnavailable }); err != nil {
				return OrderRecovery{}, err
			}
			orderID = complete.OrderID
		} else {
			var complete manualReceipt
			if err = command.Run(ctx, tx, scope, manualOperation, key, request.Body, &complete, func() error { return ErrUnavailable }); err != nil {
				return OrderRecovery{}, err
			}
			orderID = complete.OrderID
		}
		if !command.ValidID(orderID) || orderID != begin.OrderID {
			return OrderRecovery{}, command.ErrConflict
		}
		resolved = OrderRecovery{State: "created", OrderID: orderID}
		state = "created"
	} else {
		if intent.State == "created" {
			return OrderRecovery{}, command.ErrConflict
		}
		if request.Kind == forBuyerOperation {
			body, encodeErr := json.Marshal(request.Body)
			if encodeErr != nil {
				return OrderRecovery{}, command.ErrInvalid
			}
			keyHash := sha256.Sum256([]byte("order-for-buyer|" + scope.StoreID + "|" + key))
			bodyHash := sha256.Sum256(body)
			// Calls claims.for_buyer_request/release (live-console-v1 §5.1.1); absence is already proved under Begin lock.
			pending, lookupErr := claims.RequestForBuyer(ctx, tx, keyHash[:], bodyHash[:], intent.Owner)
			if lookupErr != nil {
				return OrderRecovery{}, lookupErr
			}
			if pending != nil {
				if err = claims.ReleaseForBuyer(ctx, tx, *pending); err != nil {
					return OrderRecovery{}, err
				}
			}
		}
		owner := struct {
			OwnerID string `json:"owner_id"`
		}{intent.Owner}
		var saved struct {
			OwnerID string `json:"owner_id"`
		}
		if err = command.Run(ctx, tx, scope, checkoutClosureOperation, intent.BeginKey, owner, &saved, func() error {
			if err := check(); err != nil {
				return err
			}
			saved.OwnerID = intent.Owner
			return nil
		}); err != nil {
			return OrderRecovery{}, err
		}
		if saved.OwnerID != intent.Owner {
			return OrderRecovery{}, command.ErrConflict
		}
	}
	var out OrderRecovery
	if err = command.Run(ctx, tx, scope, recoveryOperation, key, request, &out, func() error {
		if err := check(); err != nil {
			return err
		}
		out = resolved
		return command.Audit(ctx, tx, scope, "order.attempt_recovered")
	}); err != nil {
		return OrderRecovery{}, err
	}
	if out != resolved {
		return OrderRecovery{}, command.ErrConflict
	}
	if err = settleIntent(ctx, tx, scope, key, state, resolved.OrderID); err != nil {
		return OrderRecovery{}, err
	}
	return out, check()
}

func definitiveIntentRefusal(err error) bool {
	var coded *Error
	if errors.As(err, &coded) {
		return coded.Status >= 400 && coded.Status < 500 && coded.Status != 401 && coded.Status != 403 && coded.Status != 404
	}
	var forBuyer *claims.ForBuyerError
	return errors.As(err, &forBuyer) && forBuyer.Status >= 400 && forBuyer.Status < 500
}

func (m *ManualOrders) sealRefusedIntent(ctx context.Context, token, store, key string, request attemptRequest) error {
	return platform.WithScope(ctx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		_, err := m.recoverIntentState(ctx, tx, scope, token, key, request, true)
		return err
	})
}

// pendingIntent returns only the current authenticated principal's pending server request, never to the browser.
func (m *ManualOrders) pendingIntent(ctx context.Context, token, store string) (orderIntent, bool, error) {
	var intent orderIntent
	var found bool
	err := platform.WithScope(ctx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if err := recoveryPermissions(ctx, tx, scope, token); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `SELECT idempotency_key,operation,state,buyer_owner_id::text,begin_key,coalesce(order_id::text,''),request_hash,frozen_request
			FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND state='pending'`,
			scope.TenantID, scope.StoreID, scope.PrincipalID).Scan(&intent.Key, &intent.Kind, &intent.State, &intent.Owner, &intent.BeginKey, &intent.OrderID, &intent.Hash, &intent.Frozen)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return intent, found, err
}

func (m *ManualOrders) pendingHasBegin(ctx context.Context, token, store, key string) (bool, error) {
	var found bool
	err := platform.WithScope(ctx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if err := recoveryPermissions(ctx, tx, scope, token); err != nil {
			return err
		}
		intent, err := readIntent(ctx, tx, scope, key)
		if err != nil {
			return err
		}
		if err = intentBeginLock(ctx, tx, scope, intent); err != nil {
			return err
		}
		_, found, err = lookupIntentBegin(ctx, tx, token, store, key)
		return err
	})
	return found, err
}

// RecoverPending seals absence or explicitly resumes a Begin-only order from its durable original request.
// It supports both manual and for-buyer intents, never exposes their body/key, and sends only idempotent requested side effects.
func (f *ForBuyer) RecoverPending(ctx context.Context, token, store string) (OrderRecovery, error) {
	if f == nil || f.manual == nil {
		return OrderRecovery{}, ErrManualDisabled
	}
	intent, found, err := f.manual.pendingIntent(ctx, token, store)
	if err != nil {
		return OrderRecovery{}, mapForBuyerError(err)
	}
	if !found {
		return OrderRecovery{State: "none"}, nil
	}
	var envelope struct {
		Kind string          `json:"kind"`
		Body json.RawMessage `json:"body"`
	}
	if json.Unmarshal(intent.Frozen, &envelope) != nil || envelope.Kind != intent.Kind {
		return OrderRecovery{}, ErrUnavailable
	}
	var manual ManualInput
	var forBuyer ForBuyerInput
	if intent.Kind == manualOperation {
		if json.Unmarshal(envelope.Body, &manual) != nil {
			return OrderRecovery{}, ErrUnavailable
		}
	} else if intent.Kind == forBuyerOperation {
		var body forBuyerRequest
		if json.Unmarshal(envelope.Body, &body) != nil {
			return OrderRecovery{}, ErrUnavailable
		}
		forBuyer = ForBuyerInput{Items: body.Manual.Items, Customer: body.Manual.Customer, Delivery: body.Manual.Delivery, PaymentMode: body.Manual.PaymentMode,
			Locale: body.Manual.Locale, For: body.For, SendPaymentLink: body.Send}
	} else {
		return OrderRecovery{}, ErrUnavailable
	}
	var out OrderRecovery
	if intent.Kind == manualOperation {
		out, err = f.manual.Recover(ctx, token, store, intent.Key, manual)
	} else {
		out, err = f.Recover(ctx, token, store, intent.Key, forBuyer)
	}
	if err == nil || !errors.Is(err, ErrUnavailable) {
		return out, err
	}
	// Recovery's 503 can also mean unknown DB facts. Only an established original Begin permits pipeline completion.
	if exists, lookupErr := f.manual.pendingHasBegin(ctx, token, store, intent.Key); lookupErr != nil || !exists {
		return OrderRecovery{}, err
	}
	if intent.Kind == manualOperation {
		result, _, placeErr := f.manual.Place(ctx, token, store, intent.Key, manual)
		if placeErr != nil {
			return OrderRecovery{}, placeErr
		}
		return OrderRecovery{State: "created", OrderID: result.OrderID}, nil
	}
	result, _, err := f.Place(ctx, token, store, intent.Key, forBuyer)
	if err != nil {
		return OrderRecovery{}, err
	}
	return OrderRecovery{State: "created", OrderID: result.OrderID}, nil
}
