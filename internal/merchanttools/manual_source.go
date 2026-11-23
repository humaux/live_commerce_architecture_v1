package merchanttools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"livecommerce/internal/claims"
	"livecommerce/internal/command"
	"livecommerce/internal/loyalty"
	"livecommerce/internal/platform"
)

const manualIntentOperation = "merchanttools.order.manual_intent"

var manualRevision = regexp.MustCompile("^[0-9a-f]{64}$")

type ManualOrderOrigin struct {
	ConversationID string   `json:"conversation_id"`
	BundleIDs      []string `json:"bundle_ids"`
}

type ManualSourceContext struct {
	ManualOrderOrigin
	Revision string `json:"revision"`
}

func ValidManualSource(s ManualSourceContext) bool {
	if !command.ValidID(s.ConversationID) || !manualRevision.MatchString(s.Revision) || s.BundleIDs == nil || len(s.BundleIDs) > 5 {
		return false
	}
	seen := map[string]bool{}
	for _, id := range s.BundleIDs {
		if !command.ValidID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

type ManualOrderContext struct {
	State          string   `json:"state"`
	ConversationID string   `json:"conversation_id"`
	BundleIDs      []string `json:"bundle_ids,omitempty"`
	Revision       string   `json:"revision,omitempty"`
	LatestOrderID  *string  `json:"latest_order_id"`
	OrderID        string   `json:"order_id,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

func (o ManualOrderContext) MarshalJSON() ([]byte, error) {
	if o.State == "ready" {
		return json.Marshal(struct {
			State string `json:"state"`
			ManualSourceContext
			LatestOrderID *string `json:"latest_order_id"`
		}{o.State, ManualSourceContext{ManualOrderOrigin{o.ConversationID, o.BundleIDs}, o.Revision}, o.LatestOrderID})
	}
	type blocked struct {
		State          string `json:"state"`
		ConversationID string `json:"conversation_id"`
		OrderID        string `json:"order_id,omitempty"`
		Reason         string `json:"reason,omitempty"`
	}
	return json.Marshal(blocked{o.State, o.ConversationID, o.OrderID, o.Reason})
}

type manualSourceAttempt struct {
	keyHash [32]byte
	begun   claims.ForBuyerBegun
	buyerID string
	skipped int
	origins int
}

func requireManualSource(ctx context.Context, tx pgx.Tx, scope platform.Scope, token string) error {
	for _, permission := range []string{"orders:read", "inbox:read"} {
		if err := platform.RequirePermission(ctx, tx, scope, token, permission); err != nil {
			return err
		}
	}
	return nil
}

func readManualContext(ctx context.Context, tx pgx.Tx, conversation string, requested []string) (ManualOrderContext, error) {
	out := ManualOrderContext{ConversationID: conversation}
	var candidates []string
	var reason string
	err := tx.QueryRow(ctx, `SELECT bundle_ids,order_ids,reason,latest_order_id
		FROM claims.manual_order_context($1::uuid,$2::uuid[])`, conversation, requested).
		Scan(&out.BundleIDs, &candidates, &reason, &out.LatestOrderID)
	if err != nil {
		return out, manualSourceError(err)
	}
	switch {
	case reason != "":
		out.State, out.Reason = "review_required", reason
	case len(out.BundleIDs) == 0 && len(candidates) == 1:
		out.State, out.OrderID = "existing_order", candidates[0]
	case len(out.BundleIDs) == 0 && len(candidates) > 1:
		out.State, out.Reason = "review_required", "competing_orders"
	default:
		out.State = "ready"
		value, err := json.Marshal([]any{conversation, out.BundleIDs, out.LatestOrderID})
		if err != nil {
			return out, err
		}
		hash := sha256.Sum256(value)
		out.Revision = hex.EncodeToString(hash[:])
	}
	return out, nil
}

func manualKeyHash(store, key string) [32]byte {
	// The same merchant key also fences the legacy A16 reservation namespace.
	return sha256.Sum256([]byte("order-for-buyer|" + store + "|" + key))
}

func admitManualIntent(ctx context.Context, tx pgx.Tx, scope platform.Scope, token, key string, in ManualInput) (*manualSourceAttempt, error) {
	var binding struct{}
	var attempt *manualSourceAttempt
	if in.SourceContext != nil {
		attempt = &manualSourceAttempt{keyHash: manualKeyHash(scope.StoreID, key)}
	}
	err := command.Run(ctx, tx, scope, manualIntentOperation, key, in, &binding, func() error {
		if in.MemberAward != nil {
			if err := loyalty.PrecheckAward(ctx, tx, scope, token, *in.MemberAward); err != nil {
				return err
			}
		}
		if attempt == nil {
			return nil
		}
		source := in.SourceContext
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`,
			"merchanttools.manual_source|"+scope.TenantID+"|"+scope.StoreID+"|"+source.ConversationID); err != nil {
			return err
		}
		var linkedCustomer *string
		if err := tx.QueryRow(ctx, `SELECT linked_customer_id::text FROM social.conversation_meta($1::uuid)`, source.ConversationID).Scan(&linkedCustomer); err != nil {
			return manualConversationError(err)
		}
		var legacy bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ops.command_results
			WHERE tenant_id=$1 AND store_id=$2 AND operation=$3 AND idempotency_key=$4)`,
			scope.TenantID, scope.StoreID, forBuyerBeginOperation, key).Scan(&legacy); err != nil {
			return err
		}
		if legacy {
			return command.ErrConflict
		}
		var priorState, priorOrder string
		err := tx.QueryRow(ctx, `SELECT state,coalesce(order_id::text,'') FROM inbox.manual_order_sources
			WHERE tenant_id=$1 AND store_id=$2 AND conversation_id=$3 AND revision=$4 AND state<>'released'`,
			scope.TenantID, scope.StoreID, source.ConversationID, source.Revision).Scan(&priorState, &priorOrder)
		switch {
		case err == nil && priorState == "placed":
			return manualSourceConflict("source_already_ordered", priorOrder)
		case err == nil:
			return manualSourceConflict("source_order_pending", "")
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		current, err := readManualContext(ctx, tx, source.ConversationID, source.BundleIDs)
		if err != nil {
			return err
		}
		switch {
		case current.State == "existing_order":
			return manualSourceConflict("bundle_already_ordered", current.OrderID)
		case current.Reason == "source_order_pending":
			return manualSourceConflict(current.Reason, "")
		case current.State != "ready" || current.Revision != source.Revision || !slices.Equal(current.BundleIDs, source.BundleIDs):
			return manualSourceConflict("source_changed", "")
		}
		_, err = tx.Exec(ctx, `INSERT INTO inbox.manual_order_sources
			(tenant_id,store_id,principal_id,key_hash,conversation_id,bundle_ids,revision)
			VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.TenantID, scope.StoreID, scope.PrincipalID, attempt.keyHash[:],
			source.ConversationID, source.BundleIDs, source.Revision)
		return err
	})
	return attempt, err
}

func manualSourceConflict(code, order string) error {
	return &claims.ForBuyerError{Status: http.StatusConflict, Code: code, OrderID: order}
}

func manualSourceError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "PT404":
			return &Error{Status: http.StatusNotFound, Code: "not_found"}
		case "PT403":
			return platform.ErrForbidden
		case "PT409":
			return manualSourceConflict(pg.Message, pg.Detail)
		}
	}
	return err
}

func manualConversationError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return &Error{Status: http.StatusNotFound, Code: "not_found"}
	}
	return manualSourceError(err)
}

func (m *ManualOrders) placeFromConversation(ctx context.Context, token, capability, storeID, key string, in ManualInput, a *manualSourceAttempt) (manualReceipt, error) {
	var zero manualReceipt
	var saved *manualReceipt
	scope := func(run func(pgx.Tx, platform.Scope) error) error {
		return platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, scope platform.Scope) error {
			if err := requireManualSource(ctx, tx, scope, token); err != nil {
				return err
			}
			return run(tx, scope)
		})
	}
	beginKey := stepKey(key, "begin")
	// The immutable checkout result can be read even after the old options or quote
	// are unavailable. Only the owner and exact Begin key from this intent qualify.
	if err := scope(func(tx pgx.Tx, s platform.Scope) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT fulfillment.manual_conversation_checkout($1,$2)`, a.keyHash[:], beginKey).Scan(&raw); err != nil {
			return err
		}
		if raw != nil {
			saved = new(manualReceipt)
			if err := json.Unmarshal(raw, saved); err != nil {
				return err
			}
			saved.Discount = in.Discount
		}
		return nil
	}); err != nil {
		return zero, mapReceiptError(manualSourceError(err))
	}
	if saved != nil {
		return *saved, nil
	}

	registered, err := m.issuer.RegisterForTrustedStore(ctx, storeID, capability)
	if err != nil {
		return zero, classifyPipeline(ctx, err)
	}
	a.buyerID = registered.Scope.OwnerID
	hooks := &pipelineHooks{}
	canonical, err := json.Marshal(in)
	if err != nil {
		return zero, command.ErrInvalid
	}
	requestHash := sha256.Sum256(canonical)
	err = scope(func(tx pgx.Tx, s platform.Scope) error {
		var active bool
		if err := tx.QueryRow(ctx, `SELECT state='pending' FROM inbox.manual_order_sources
			WHERE tenant_id=$1 AND store_id=$2 AND key_hash=$3 FOR UPDATE`, s.TenantID, s.StoreID, a.keyHash[:]).Scan(&active); err != nil {
			return err
		}
		if !active {
			return manualSourceConflict("source_changed", "")
		}
		var linkedCustomer *string
		if err := tx.QueryRow(ctx, `SELECT linked_customer_id::text FROM social.conversation_meta($1::uuid)`, in.SourceContext.ConversationID).Scan(&linkedCustomer); err != nil {
			return manualConversationError(err)
		}
		if len(in.SourceContext.BundleIDs) > 0 {
			var inner error
			a.begun, inner = claims.BeginForBuyer(ctx, tx, a.keyHash[:], requestHash[:], &in.SourceContext.ConversationID, in.SourceContext.BundleIDs, a.buyerID)
			if inner != nil {
				return inner
			}
			if a.begun.Reason == "" {
				var lines []claims.ForBuyerLine
				for _, bundle := range in.SourceContext.BundleIDs {
					got, e := claims.ForBuyerLines(ctx, tx, nil, &bundle)
					if e != nil {
						return e
					}
					lines = append(lines, got...)
				}
				hooks.origins, a.skipped = pickOrigins(in.Items, lines)
				a.origins = len(hooks.origins)
				hooks.afterQuote = (&ForBuyer{manual: m}).bindGrant
			}
		} else {
			a.begun.Reason = "no_bundle"
		}
		reason := notAppliedReason(a.begun.Reason, a.origins)
		var request *string
		if a.begun.RequestID != "" {
			request = &a.begun.RequestID
		}
		tag, inner := tx.Exec(ctx, `UPDATE inbox.manual_order_sources
			SET buyer_id=$4,claim_request_id=$5,price_reason=$6,some_catalog=$7
			WHERE tenant_id=$1 AND store_id=$2 AND key_hash=$3 AND state='pending' AND (buyer_id IS NULL OR buyer_id=$4)`,
			s.TenantID, s.StoreID, a.keyHash[:], a.buyerID, request, reason, a.skipped > 0)
		if inner == nil && tag.RowsAffected() == 0 {
			return manualSourceConflict("source_changed", "")
		}
		return inner
	})
	if err != nil {
		return zero, m.refuseManualSource(ctx, token, storeID, a, mapForBuyerError(err))
	}
	hooks.beforeBegin = func(ctx context.Context) error {
		if m.FaultBeforeContextBegin != nil {
			if err := m.FaultBeforeContextBegin(); err != nil {
				return err
			}
		}
		return scope(func(tx pgx.Tx, s platform.Scope) error {
			if _, err := readManualContext(ctx, tx, in.SourceContext.ConversationID, in.SourceContext.BundleIDs); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `UPDATE inbox.manual_order_sources SET begin_key=$4
				WHERE tenant_id=$1 AND store_id=$2 AND key_hash=$3
				  AND (state='pending' AND begin_key IS NULL OR state IN ('pending','placed') AND begin_key=$4)`,
				s.TenantID, s.StoreID, a.keyHash[:], beginKey)
			if err == nil && tag.RowsAffected() == 0 {
				return manualSourceConflict("source_changed", "")
			}
			return err
		})
	}
	sent, err := m.pipeline(withPipelineHooks(ctx, hooks), token, capability, storeID, key, in)
	if err != nil {
		var source *claims.ForBuyerError
		if errors.As(err, &source) {
			return zero, m.refuseManualSource(ctx, token, storeID, a, err)
		}
		return zero, m.refuseManualSource(ctx, token, storeID, a, classifyPipeline(ctx, err))
	}
	return sent, nil
}

func (m *ManualOrders) refuseManualSource(ctx context.Context, token, storeID string, a *manualSourceAttempt, refusal error) error {
	var conflict *claims.ForBuyerError
	if !definitiveRefusal(refusal) && !(errors.As(refusal, &conflict) && conflict.Code != "idempotency_conflict") {
		return refusal
	}
	var released bool
	err := platform.WithScope(ctx, m.pool, token, storeID, manualPermission, func(tx pgx.Tx, _ platform.Scope) error {
		return tx.QueryRow(ctx, `SELECT claims.release_manual_order_source($1)`, a.keyHash[:]).Scan(&released)
	})
	if err != nil || !released {
		return manualSourceConflict("source_order_pending", "")
	}
	return refusal
}

func finishManualSource(ctx context.Context, tx pgx.Tx, scope platform.Scope, a *manualSourceAttempt, order string, linkHash []byte, receipt *manualReceipt) error {
	var request, buyerID *string
	var bundles []string
	var reason string
	var someCatalog bool
	err := tx.QueryRow(ctx, `SELECT bundle_ids,claim_request_id::text,buyer_id::text,price_reason,some_catalog
		FROM inbox.manual_order_sources WHERE tenant_id=$1 AND store_id=$2 AND key_hash=$3 AND state='pending' FOR UPDATE`,
		scope.TenantID, scope.StoreID, a.keyHash[:]).Scan(&bundles, &request, &buyerID, &reason, &someCatalog)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT fulfillment.finish_manual_conversation_order($1,$2::uuid,$3)`, a.keyHash[:], order, linkHash); err != nil {
		return err
	}
	receipt.LivePrice, receipt.LivePriceReason = "not_applied", &reason
	if request == nil {
		return nil
	}
	granted, err := claims.FinishForBuyer(ctx, tx, *request, order)
	if err != nil {
		return err
	}
	if len(granted) > 0 {
		receipt.LivePrice, reason = "applied", ""
		if someCatalog {
			reason = "some_lines_catalog"
		}
	}
	for _, bundle := range bundles {
		if detail, ok := grantAudit(bundle, *buyerID, order, granted); ok {
			if err := command.AuditDetails(ctx, tx, scope, "claims.merchant_origin_granted", detail); err != nil {
				return err
			}
		}
	}
	return nil
}

func readManualOrigin(ctx context.Context, tx pgx.Tx, storeID, principalID, key string, own bool) (*ManualOrderOrigin, error) {
	h := manualKeyHash(storeID, key)
	var out ManualOrderOrigin
	err := tx.QueryRow(ctx, `SELECT conversation_id,bundle_ids FROM claims.read_manual_order_source($1,$2::uuid,$3)`, h[:], principalID, own).
		Scan(&out.ConversationID, &out.BundleIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &out, err
}

type ManualReceiptRead struct {
	State   string                 `json:"state"`
	Receipt *ManualReceiptSnapshot `json:"receipt,omitempty"`
}

type ManualReceiptSnapshot struct {
	manualReceipt
	OrderOrigin *ManualOrderOrigin `json:"order_origin,omitempty"`
}

func (m *ManualOrders) ReadReceipt(ctx context.Context, token, storeID, key string) (ManualReceiptRead, error) {
	if m == nil {
		return ManualReceiptRead{}, ErrManualDisabled
	}
	out := ManualReceiptRead{State: "absent"}
	err := platform.WithScope(ctx, m.pool, token, storeID, "orders:read", func(tx pgx.Tx, scope platform.Scope) error {
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT r.response FROM ops.command_results r LEFT JOIN ops.command_results i
			ON i.tenant_id=r.tenant_id AND i.store_id=r.store_id AND i.idempotency_key=r.idempotency_key AND i.operation=$6
			WHERE r.tenant_id=$1 AND r.store_id=$2 AND r.idempotency_key=$3 AND r.operation=$4
			  AND coalesce(i.principal_id,r.principal_id)=$5`,
			scope.TenantID, scope.StoreID, key, manualOperation, scope.PrincipalID, manualIntentOperation).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			var pending bool
			keyHash := manualKeyHash(scope.StoreID, key)
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ops.command_results r WHERE r.tenant_id=$1 AND r.store_id=$2
				AND r.idempotency_key=$3 AND r.operation=$4 AND r.principal_id=$5
				AND NOT EXISTS(SELECT 1 FROM inbox.manual_order_sources m WHERE m.tenant_id=r.tenant_id
					AND m.store_id=r.store_id AND m.key_hash=$6 AND m.state='released'))`,
				scope.TenantID, scope.StoreID, key, manualIntentOperation, scope.PrincipalID, keyHash[:]).Scan(&pending); err != nil {
				return err
			}
			if pending {
				out.State = "pending"
			}
			return nil
		}
		if err != nil {
			return err
		}
		out.State, out.Receipt = "committed", new(ManualReceiptSnapshot)
		if err := json.Unmarshal(raw, &out.Receipt.manualReceipt); err != nil {
			return err
		}
		out.Receipt.ExpiresAt = out.Receipt.ExpiresAt.UTC()
		out.Receipt.OrderOrigin, err = readManualOrigin(ctx, tx, storeID, scope.PrincipalID, key, true)
		return err
	})
	if err != nil {
		return ManualReceiptRead{}, mapReceiptError(err)
	}
	return out, nil
}
