// Purpose: resolve or explicitly resume durable merchant intents using their server-frozen request and original private owner.
// Depends on: ops.order_intents, fulfillment.order_intent_begin, command receipts/audit, claims.for_buyer_request/release.
// Used by: manual/for-buyer A16R recovery and transport-key-bound recovery after tab changes or re-login.
// Invariants: I01/I02/I03/I06; absent checkout is sealed atomically, terminal replay requires original proofs, lost responses cannot retarget an intent.
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

const recoveryTargetOperation = "merchanttools.order.intent.recovery.target"

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
	if intent.State == "sealed" {
		// The Begin definer intentionally accepts only pending/created intents. Sealed state alone is never absence proof:
		// replay its exact original closure and recovery receipts, without fabricating either if it is missing.
		if intent.OrderID != "" || !command.ValidID(intent.Owner) || intent.BeginKey != stepKey(key, "begin") {
			return OrderRecovery{}, command.ErrConflict
		}
		owner := struct {
			OwnerID string `json:"owner_id"`
		}{intent.Owner}
		var closure struct {
			OwnerID string `json:"owner_id"`
		}
		if err = command.Run(ctx, tx, scope, checkoutClosureOperation, intent.BeginKey, owner, &closure, func() error { return ErrUnavailable }); err != nil {
			return OrderRecovery{}, err
		}
		if closure.OwnerID != intent.Owner {
			return OrderRecovery{}, command.ErrConflict
		}
		var out OrderRecovery
		if err = command.Run(ctx, tx, scope, recoveryOperation, key, request, &out, func() error { return ErrUnavailable }); err != nil {
			return OrderRecovery{}, err
		}
		if out != (OrderRecovery{State: "not_created"}) {
			return OrderRecovery{}, command.ErrConflict
		}
		return out, check()
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

// recoveryIntent commits a canonical transport-key -> original intent binding before any pipeline side effect.
// A repeated transport key never selects a later pending intent. A new browser key may select the latest settled intent,
// which prevents a lost created response from becoming none merely because settlement released the pending slot.
func (m *ManualOrders) recoveryIntent(ctx context.Context, token, store, recoveryKey string) (orderIntent, bool, error) {
	var intent orderIntent
	var found bool
	err := platform.WithScope(ctx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if err := recoveryPermissions(ctx, tx, scope, token); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "merchanttools.intent|"+scope.TenantID+"|"+scope.StoreID+"|"+scope.PrincipalID); err != nil {
			return err
		}
		if err := recoveryPermissions(ctx, tx, scope, token); err != nil {
			return err
		}
		request := struct {
			StoreID     string `json:"store_id"`
			PrincipalID string `json:"principal_id"`
		}{scope.StoreID, scope.PrincipalID}
		var target struct {
			Key string `json:"intent_key"`
		}
		if err := command.Run(ctx, tx, scope, recoveryTargetOperation, recoveryKey, request, &target, func() error {
			err := tx.QueryRow(ctx, `SELECT idempotency_key FROM ops.order_intents
				WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3
				ORDER BY (state='pending') DESC,updated_at DESC,created_at DESC,idempotency_key LIMIT 1`,
				scope.TenantID, scope.StoreID, scope.PrincipalID).Scan(&target.Key)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}); err != nil {
			return err
		}
		if target.Key == "" {
			return nil
		}
		var err error
		intent, err = readIntent(ctx, tx, scope, target.Key)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnavailable // A saved target disappearing is UNKNOWN, never absence.
		}
		found = err == nil
		if err != nil {
			return err
		}
		return recoveryPermissions(ctx, tx, scope, token)
	})
	return intent, found, err
}

// intentReceipt reads only this principal's original command proof. It never creates or repairs a receipt.
func intentReceipt(ctx context.Context, tx pgx.Tx, scope platform.Scope, operation, key string, expectedHash []byte, result any) ([]byte, error) {
	var hash, response []byte
	err := tx.QueryRow(ctx, `SELECT request_hash,response FROM ops.command_results
		WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND operation=$4 AND idempotency_key=$5`,
		scope.TenantID, scope.StoreID, scope.PrincipalID, operation, key).Scan(&hash, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnavailable
	}
	if err != nil {
		return nil, err
	}
	if len(hash) != sha256.Size || (expectedHash != nil && !bytes.Equal(hash, expectedHash)) {
		return nil, command.ErrConflict
	}
	if json.Unmarshal(response, result) != nil {
		return nil, ErrUnavailable
	}
	return hash, nil
}

// terminalIntent verifies the immutable server binding and exact original upper/closure receipts even after frozen PII is removed.
// No cached transport result or bare terminal state can prove created/not_created on its own.
func (m *ManualOrders) terminalIntent(ctx context.Context, token, store, key string) (OrderRecovery, error) {
	var out OrderRecovery
	err := platform.WithScope(ctx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		check := func() error { return recoveryPermissions(ctx, tx, scope, token) }
		if err := check(); err != nil {
			return err
		}
		intent, err := readIntent(ctx, tx, scope, key)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUnavailable
		}
		if err != nil {
			return err
		}
		if !command.ValidID(intent.Owner) || intent.BeginKey != stepKey(key, "begin") ||
			(intent.Kind != manualOperation && intent.Kind != forBuyerOperation) || (intent.State != "created" && intent.State != "sealed") {
			return ErrUnavailable
		}
		if err = intentBeginLock(ctx, tx, scope, intent); err != nil {
			return err
		}
		if err = check(); err != nil {
			return err
		}
		var binding struct {
			Bound bool `json:"bound"`
		}
		if _, err = intentReceipt(ctx, tx, scope, attemptOperation, key, intent.Hash, &binding); err != nil {
			return err
		}
		if !binding.Bound {
			return command.ErrConflict
		}
		if intent.State == "sealed" {
			if intent.OrderID != "" {
				return command.ErrConflict
			}
			owner := struct {
				OwnerID string `json:"owner_id"`
			}{intent.Owner}
			encoded, err := json.Marshal(owner)
			if err != nil {
				return ErrUnavailable
			}
			hash := sha256.Sum256(encoded)
			var closure struct {
				OwnerID string `json:"owner_id"`
			}
			if _, err = intentReceipt(ctx, tx, scope, checkoutClosureOperation, intent.BeginKey, hash[:], &closure); err != nil {
				return err
			}
			if closure.OwnerID != intent.Owner {
				return command.ErrConflict
			}
			if _, err = intentReceipt(ctx, tx, scope, recoveryOperation, key, intent.Hash, &out); err != nil {
				return err
			}
			if out != (OrderRecovery{State: "not_created"}) {
				return command.ErrConflict
			}
			return check()
		}
		begin, found, err := lookupIntentBegin(ctx, tx, token, store, key)
		if err != nil {
			return err
		}
		if !found || !command.ValidID(intent.OrderID) || begin.OrderID != intent.OrderID {
			return command.ErrConflict
		}
		var manual manualReceipt
		if _, err = intentReceipt(ctx, tx, scope, manualOperation, key, nil, &manual); err != nil {
			return err
		}
		if manual.OrderID != intent.OrderID {
			return command.ErrConflict
		}
		if intent.Kind == forBuyerOperation {
			var placed forBuyerReceipt
			hash, err := intentReceipt(ctx, tx, scope, forBuyerOperation, key, nil, &placed)
			if err != nil {
				return err
			}
			var complete forBuyerCompletion
			if _, err = intentReceipt(ctx, tx, scope, forBuyerCompleteOperation, key, hash, &complete); err != nil {
				return err
			}
			if placed.OrderID != intent.OrderID || complete.OrderID != intent.OrderID {
				return command.ErrConflict
			}
		}
		out = OrderRecovery{State: "created", OrderID: intent.OrderID}
		return check()
	})
	return out, err
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

// RecoverPending binds the transport key, then seals absence or resumes the exact durable intent.
// It supports pending and settled manual/for-buyer intents without exposing their frozen body/key.
func (f *ForBuyer) RecoverPending(ctx context.Context, token, store, recoveryKey string) (OrderRecovery, error) {
	if f == nil || f.manual == nil {
		return OrderRecovery{}, ErrManualDisabled
	}
	ctx, cancel := context.WithTimeout(ctx, attemptBudget)
	defer cancel()
	intent, found, err := f.manual.recoveryIntent(ctx, token, store, recoveryKey)
	if err != nil {
		return OrderRecovery{}, mapForBuyerError(err)
	}
	if !found {
		return OrderRecovery{State: "none"}, nil
	}
	terminal := func() (OrderRecovery, error) {
		out, err := f.manual.terminalIntent(ctx, token, store, intent.Key)
		return out, mapForBuyerError(err)
	}
	if intent.State == "created" || intent.State == "sealed" {
		return terminal()
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
	if err == nil {
		return terminal()
	}
	if !errors.Is(err, ErrUnavailable) {
		return out, err
	}
	// Recovery's 503 can also mean unknown DB facts. Only an established original Begin permits pipeline completion.
	if exists, lookupErr := f.manual.pendingHasBegin(ctx, token, store, intent.Key); lookupErr != nil || !exists {
		return OrderRecovery{}, err
	}
	if intent.Kind == manualOperation {
		_, _, placeErr := f.manual.Place(ctx, token, store, intent.Key, manual)
		if placeErr != nil {
			return OrderRecovery{}, placeErr
		}
		return terminal()
	}
	_, _, err = f.Place(ctx, token, store, intent.Key, forBuyer)
	if err != nil {
		return OrderRecovery{}, err
	}
	return terminal()
}
