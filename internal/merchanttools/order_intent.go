// Purpose: own durable merchant order intents; a browser guard is only a UX hint.
// Depends on: ops.order_intents RLS, platform auth, command receipts and fulfillment.order_intent_begin (0172).
// Used by: ManualOrders/ForBuyer placement and A16R recovery, including recovery after another tab or re-login.
// Invariants: I01/I02/I03/I06; one pending intent per authenticated principal/store, immutable key/body/owner, no expiry unlock.
package merchanttools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

type orderIntent struct {
	Key, Kind, State, Owner, BeginKey, OrderID string
	Hash, Frozen                               []byte
}

// bindIntent runs while both the principal/store and original key locks are held.
// The pending row commits before any pipeline side effect; credentials are never stored in it.
func (m *ManualOrders) bindIntent(ctx context.Context, tx pgx.Tx, scope platform.Scope, store, key string, request attemptRequest, legacyOwner string) (orderIntent, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return orderIntent{}, command.ErrInvalid
	}
	hash := sha256.Sum256(encoded)
	var intent orderIntent
	err = tx.QueryRow(ctx, `SELECT idempotency_key,operation,state,buyer_owner_id::text,begin_key,coalesce(order_id::text,''),request_hash,frozen_request
		FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND idempotency_key=$4`,
		scope.TenantID, scope.StoreID, scope.PrincipalID, key).Scan(&intent.Key, &intent.Kind, &intent.State, &intent.Owner, &intent.BeginKey, &intent.OrderID, &intent.Hash, &intent.Frozen)
	if err == nil {
		if intent.Kind != request.Kind || !bytes.Equal(intent.Hash, hash[:]) {
			return orderIntent{}, command.ErrConflict
		}
		return intent, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return orderIntent{}, err
	}
	var pending bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND state='pending')`,
		scope.TenantID, scope.StoreID, scope.PrincipalID).Scan(&pending); err != nil {
		return orderIntent{}, err
	}
	if pending {
		return orderIntent{}, &Error{Status: http.StatusConflict, Code: "attempt_pending"}
	}
	owner := legacyOwner
	if owner == "" {
		// Calls buyer.register_capability_limited for a new server-private owner; no buyer bearer enters the record.
		issued, issueErr := m.issuer.RegisterForTrustedStore(ctx, store, m.capability("order", store, key))
		if issueErr != nil {
			return orderIntent{}, issueErr
		}
		if issued.Scope.TenantID != scope.TenantID || issued.Scope.StoreID != scope.StoreID {
			return orderIntent{}, platform.ErrScopeNotFound
		}
		owner = issued.Scope.OwnerID
	}
	if !command.ValidID(owner) {
		return orderIntent{}, ErrUnavailable
	}
	intent = orderIntent{Key: key, Kind: request.Kind, State: "pending", Owner: owner, BeginKey: stepKey(key, "begin"), Hash: hash[:], Frozen: encoded}
	_, err = tx.Exec(ctx, `INSERT INTO ops.order_intents(tenant_id,store_id,principal_id,idempotency_key,operation,request_hash,frozen_request,buyer_owner_id,begin_key,state)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending')`, scope.TenantID, scope.StoreID, scope.PrincipalID, key, request.Kind, hash[:], encoded, intent.Owner, intent.BeginKey)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		// A key owned by a different principal is deliberately invisible under RLS, but still permanently bound.
		return orderIntent{}, command.ErrConflict
	}
	return intent, err
}

func readIntent(ctx context.Context, tx pgx.Tx, scope platform.Scope, key string) (orderIntent, error) {
	var out orderIntent
	err := tx.QueryRow(ctx, `SELECT idempotency_key,operation,state,buyer_owner_id::text,begin_key,coalesce(order_id::text,''),request_hash,frozen_request
		FROM ops.order_intents WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND idempotency_key=$4`,
		scope.TenantID, scope.StoreID, scope.PrincipalID, key).Scan(&out.Key, &out.Kind, &out.State, &out.Owner, &out.BeginKey, &out.OrderID, &out.Hash, &out.Frozen)
	return out, err
}

// settleIntent changes only an existing same-principal intent and removes its private frozen body on settlement.
// The caller must have proved the complete upper receipt or locked absence in this transaction.
func settleIntent(ctx context.Context, tx pgx.Tx, scope platform.Scope, key, state, orderID string) error {
	intent, err := readIntent(ctx, tx, scope, key)
	if err != nil {
		return err
	}
	if intent.State == state && intent.OrderID == orderID {
		return nil
	}
	if intent.State != "pending" || (state != "created" && state != "sealed") || (state == "created" && !command.ValidID(orderID)) {
		return command.ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE ops.order_intents SET state=$5,order_id=nullif($6,'')::uuid,frozen_request=NULL,updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND store_id=$2 AND principal_id=$3 AND idempotency_key=$4 AND state='pending'`,
		scope.TenantID, scope.StoreID, scope.PrincipalID, key, state, orderID)
	return err
}

// lookupIntentBegin uses the durable server intent rather than the expiring buyer session.
// Calls fulfillment.order_intent_begin (live-console-v1 §5.1.1); caller holds the exact private Begin lock through settlement.
func lookupIntentBegin(ctx context.Context, tx pgx.Tx, token, store, key string) (manualReceipt, bool, error) {
	hash := sha256.Sum256([]byte(token))
	var out manualReceipt
	err := tx.QueryRow(ctx, `SELECT order_id::text,commercial_state,payment_mode,total_minor,currency,expires_at
		FROM fulfillment.order_intent_begin($1,$2::uuid,$3)`, hash[:], store, key).
		Scan(&out.OrderID, &out.CommercialState, &out.PaymentMode, &out.TotalMinor, &out.Currency, &out.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return manualReceipt{}, false, nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "PT409" {
		// This exact lookup uses PT409 for missing or inconsistent original checkout evidence.
		// It is UNKNOWN, unlike a verified same-key/body conflict elsewhere; never turn it into an absence seal.
		return manualReceipt{}, false, ErrUnavailable
	}
	return out, err == nil, err
}

func intentBeginLock(ctx context.Context, tx pgx.Tx, scope platform.Scope, intent orderIntent) error {
	if _, err := tx.Exec(ctx, `SELECT set_config('lock_timeout','0',true)`); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "checkout.begin|"+scope.TenantID+"|"+scope.StoreID+"|"+intent.Owner+"|"+intent.BeginKey)
	return err
}

// settleManualSuccess requires the durable manual receipt, which includes source/link/audit completion.
func (m *ManualOrders) settleManualSuccess(ctx context.Context, token, store, key string, body any) error {
	return platform.WithScope(ctx, m.pool, token, store, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
		if err := requireCatalog(ctx, tx, scope, token); err != nil {
			return err
		}
		var completed manualReceipt
		if err := command.Run(ctx, tx, scope, manualOperation, key, body, &completed, func() error { return ErrUnavailable }); err != nil {
			return err
		}
		return settleIntent(ctx, tx, scope, key, "created", completed.OrderID)
	})
}
