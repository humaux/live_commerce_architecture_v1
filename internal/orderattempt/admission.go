// Purpose: atomic merchant C1 admission with frozen money, encrypted plan and River job.
// Depends: ops prepare/finish definers, pgx/River on one runtime pool, pricing and PlanKeys.
// Used by: dormant OA1 library callers and real-PG acceptance; not wired into placement.
// Invariants: server authority and all-state replay precede fresh pricing; one READ
// COMMITTED transaction; no nested scope transaction, private data in jobs or blind retry.
package orderattempt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"livecommerce/internal/command"
	"livecommerce/internal/pricing"
)

// AdmissionRequest carries a server-normalized selection without money. SQL
// owns its closed schema, authenticated scope, canonical hash and CAS validation.
type AdmissionRequest struct {
	ExpectedScopeID   string          `json:"expected_scope_id"`
	ExpectedGeneration int64          `json:"expected_generation"`
	PreviousAttemptID *string         `json:"previous_attempt_id,omitempty"`
	TransportKey      string          `json:"transport_key"`
	Selection         json.RawMessage `json:"selection"`
}

// AdmissionReceipt contains only durable identifiers and the safe stored terminal
// receipt. It never contains the encrypted plan, buyer contact or pricing facts.
type AdmissionReceipt struct {
	AttemptID  string          `json:"attempt_id"`
	ScopeID    string          `json:"scope_id"`
	Generation int64           `json:"intent_generation"`
	State      string          `json:"state"`
	Replay     bool            `json:"replay"`
	Receipt    json.RawMessage `json:"receipt"`
}

// AdmissionError exposes fixed codes only. Unknown commit outcomes must be read
// back through durable key/attempt authority; this service never retries them.
type AdmissionError struct {
	code, state string
	unknown bool
}

func (e *AdmissionError) Error() string { return e.code }
func (e *AdmissionError) Code() string { return e.code }
func (e *AdmissionError) SQLState() string { return e.state }
func (e *AdmissionError) OutcomeUnknown() bool { return e.unknown }

// Service borrows its runtime pool and immutable keyring. Its insert-only River
// client uses that exact pool, so admission cannot enqueue through another DB.
type Service struct {
	pool *pgxpool.Pool
	keys *PlanKeys
	jobs *river.Client[pgx.Tx]
}

// NewService creates no workers or connections and does not own the pool lifecycle.
// The composition root opens the pool with platform.OpenPool's authority gate.
// Seal checks key availability inside C1, so an unavailable ring cannot commit a candidate.
func NewService(pool *pgxpool.Pool, keys *PlanKeys) (*Service, error) {
	if pool == nil || keys == nil { return nil, &AdmissionError{code:"invalid_request"} }
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema:"river"})
	if err != nil { return nil, &AdmissionError{code:"order_attempt_unavailable"} }
	return &Service{pool:pool, keys:keys, jobs:jobs}, nil
}

type admissionJob struct {
	AttemptID string `json:"attempt_id"`
	Generation int64 `json:"intent_generation"`
}

func (admissionJob) Kind() string { return "order_attempt_v1" }

type admissionWire struct {
	AdmissionReceipt
	TenantID string `json:"tenant_id"`
	StoreID string `json:"store_id"`
	PrincipalID string `json:"principal_id"`
	BuyerOwnerID string `json:"buyer_owner_id"`
	AdmittedAt time.Time `json:"admitted_at"`
	ExecuteBy time.Time `json:"execute_by"`
	RequestHash string `json:"request_hash"`
	FactsDigest string `json:"facts_digest"`
	Facts json.RawMessage `json:"facts"`
}

// Admit returns success only after C1 commits. A replay commits without resealing
// or minting a job. Neither path creates an order, hold, pay entry or notification.
func (s *Service) Admit(ctx context.Context, merchantToken, store string, req AdmissionRequest) (AdmissionReceipt, error) {
	if ctx == nil || s == nil || s.pool == nil || s.jobs == nil || !validAdmissionRequest(merchantToken, store, req) {
		return AdmissionReceipt{}, &AdmissionError{code:"invalid_request"}
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.pool.BeginTx(bounded, pgx.TxOptions{IsoLevel:pgx.ReadCommitted})
	if err != nil { return AdmissionReceipt{}, admissionFailure(err, false) }
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if _, err = tx.Exec(bounded, `SELECT set_config('statement_timeout','5s',true),
	 set_config('lock_timeout','1s',true),set_config('idle_in_transaction_session_timeout','5s',true)`); err != nil {
		return AdmissionReceipt{}, admissionFailure(err, false)
	}
	authHash, keyDigest := sha256.Sum256([]byte(merchantToken)), sha256.Sum256([]byte(req.TransportKey))
	var raw []byte
	err = tx.QueryRow(bounded, `SELECT ops.prepare_order_attempt_admission($1::bytea,$2::uuid,$3::uuid,$4::bigint,$5::uuid,$6::bytea,$7::jsonb)`,
		authHash[:], store, req.ExpectedScopeID, req.ExpectedGeneration, req.PreviousAttemptID, keyDigest[:], string(req.Selection)).Scan(&raw)
	if err != nil { return AdmissionReceipt{}, admissionFailure(err, false) }
	var prepared admissionWire
	if err = json.Unmarshal(raw, &prepared); err != nil || !validAdmissionReceipt(prepared.AdmissionReceipt) || prepared.ScopeID != req.ExpectedScopeID {
		return AdmissionReceipt{}, &AdmissionError{code:"order_attempt_invalid_response"}
	}
	if prepared.Replay {
		if err = tx.Commit(bounded); err != nil { return AdmissionReceipt{}, admissionFailure(err, true) }
		return prepared.AdmissionReceipt, nil
	}
	if prepared.State != "PENDING" || prepared.StoreID != store || prepared.Generation != req.ExpectedGeneration+1 {
		return AdmissionReceipt{}, &AdmissionError{code:"order_attempt_invalid_response"}
	}
	calculation, err := admissionCalculation(prepared.Facts)
	if err != nil { return AdmissionReceipt{}, err }
	plan := FrozenPlan{Schema:1, AttemptID:prepared.AttemptID, ScopeID:prepared.ScopeID, Generation:prepared.Generation,
		TenantID:prepared.TenantID, StoreID:prepared.StoreID, PrincipalID:prepared.PrincipalID, BuyerOwnerID:prepared.BuyerOwnerID,
		AdmittedAt:prepared.AdmittedAt, ExecuteBy:prepared.ExecuteBy, Facts:prepared.Facts, Calculation:calculation}
	if !admissionDigest(prepared.RequestHash, &plan.RequestHash) || !admissionDigest(prepared.FactsDigest, &plan.FactsDigest) {
		return AdmissionReceipt{}, &AdmissionError{code:"order_attempt_invalid_response"}
	}
	sealed, err := s.keys.Seal(plan)
	if err != nil { return AdmissionReceipt{}, err }
	if _, err = s.jobs.InsertTx(bounded, tx, admissionJob{AttemptID:prepared.AttemptID, Generation:prepared.Generation}, &river.InsertOpts{Queue:"claims", Priority:1}); err != nil {
		return AdmissionReceipt{}, admissionFailure(err, false)
	}
	err = tx.QueryRow(bounded, `SELECT ops.finish_order_attempt_admission($1::bytea,$2::uuid,$3::bigint,$4::bytea,$5::text,$6::bytea)`,
		authHash[:], prepared.AttemptID, prepared.Generation, sealed.Digest[:], sealed.KeyVersion, sealed.Ciphertext).Scan(&raw)
	if err != nil { return AdmissionReceipt{}, admissionFailure(err, false) }
	var finished AdmissionReceipt
	if err = json.Unmarshal(raw, &finished); err != nil || !validAdmissionReceipt(finished) || finished.AttemptID != prepared.AttemptID ||
		finished.ScopeID != prepared.ScopeID || finished.Generation != prepared.Generation || finished.State != "PENDING" || finished.Replay {
		return AdmissionReceipt{}, &AdmissionError{code:"order_attempt_invalid_response"}
	}
	if err = tx.Commit(bounded); err != nil { return AdmissionReceipt{}, admissionFailure(err, true) }
	return finished, nil
}

func validAdmissionRequest(token, store string, req AdmissionRequest) bool {
	if len(token)<32 || len(token)>512 || !command.ValidID(store) || !command.ValidID(req.ExpectedScopeID) || req.ExpectedGeneration<0 ||
		req.ExpectedGeneration==int64(^uint64(0)>>1) || len(req.TransportKey)<8 || len(req.TransportKey)>128 ||
		(req.PreviousAttemptID!=nil && !command.ValidID(*req.PreviousAttemptID)) || len(req.Selection)>65536 || !json.Valid(req.Selection) {
		return false
	}
	for _, c := range req.TransportKey {
		if !((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||strings.ContainsRune("_.:-",c)) { return false }
	}
	trimmed := bytes.TrimSpace(req.Selection)
	return len(trimmed)>0 && trimmed[0]=='{'
}

func validAdmissionReceipt(r AdmissionReceipt) bool {
	return command.ValidID(r.AttemptID) && command.ValidID(r.ScopeID) && r.Generation>0 &&
		(r.State=="PENDING" || r.State=="COMMITTED" || r.State=="ABORTED") && len(r.Receipt)<=16384
}

func admissionDigest(raw string, out *[32]byte) bool {
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded)!=len(out) { return false }
	copy(out[:],decoded)
	return *out!=([32]byte{})
}

type admissionLine struct {
	Ordinal int `json:"ordinal"`
	SKUID string `json:"sku_id"`
	OriginID string `json:"origin_id"`
	BundleID string `json:"bundle_id"`
	OfferID string `json:"offer_id"`
	Quantity int64 `json:"quantity"`
	UnitPriceMinor int64 `json:"unit_price_minor"`
	Currency string `json:"currency"`
}

type admissionClaim struct {
	admissionLine
	LivePriceMinor *int64 `json:"live_price_minor"`
	Reason string `json:"reason"`
}

// admissionCalculation reads only authenticated domain facts. It preserves ordinal
// order and applies claim prices to exact origin tuples, never to every matching SKU.
// Original facts remain unchanged in FrozenPlan for C2's independent verification.
func admissionCalculation(raw json.RawMessage) (pricing.Calculation, error) {
	invalid := &AdmissionError{code:"order_attempt_invalid_facts"}
	var facts struct {
		Catalog struct {
			Policy pricing.Policy `json:"policy"`
			Lines []admissionLine `json:"lines"`
			Payment struct { Mode string `json:"mode"`; SurchargeMinor int64 `json:"surcharge_minor"`; MaxMinor int64 `json:"max_minor"` } `json:"payment"`
		} `json:"catalog"`
		Claims struct { Lines []admissionClaim `json:"lines"` } `json:"claims"`
	}
	if len(raw)>maxPlanBytes || json.Unmarshal(raw,&facts)!=nil || len(facts.Catalog.Lines)==0 || len(facts.Catalog.Lines)>50 { return pricing.Calculation{}, invalid }
	claims := make(map[int]admissionClaim,len(facts.Claims.Lines))
	for _, claim := range facts.Claims.Lines {
		if _, duplicate := claims[claim.Ordinal]; duplicate || claim.Ordinal<=0 { return pricing.Calculation{}, invalid }
		claims[claim.Ordinal]=claim
	}
	inputs := make([]pricing.AmountLine,0,len(facts.Catalog.Lines))
	lastOrdinal := 0
	for _, line := range facts.Catalog.Lines {
		if line.Ordinal<=lastOrdinal || !command.ValidID(line.SKUID) || line.Currency!=facts.Catalog.Policy.Currency ||
			(line.OriginID!=""&&!command.ValidID(line.OriginID)) || (line.BundleID=="")!=(line.OfferID=="") { return pricing.Calculation{}, invalid }
		lastOrdinal=line.Ordinal
		price:=line.UnitPriceMinor
		claim, exists := claims[line.Ordinal]
		if line.BundleID!="" {
			if !command.ValidID(line.BundleID) || !command.ValidID(line.OfferID) || line.OriginID=="" || !exists ||
				claim.SKUID!=line.SKUID || claim.OriginID!=line.OriginID || claim.BundleID!=line.BundleID || claim.OfferID!=line.OfferID || claim.Quantity!=line.Quantity { return pricing.Calculation{}, invalid }
			if claim.LivePriceMinor!=nil {
				if claim.Reason!="" || *claim.LivePriceMinor<1 || *claim.LivePriceMinor>command.MaxMoney { return pricing.Calculation{}, invalid }
				price=*claim.LivePriceMinor
			}
			delete(claims,line.Ordinal)
		} else if exists { return pricing.Calculation{}, invalid }
		inputs=append(inputs,pricing.AmountLine{UnitPriceMinor:price,Quantity:line.Quantity})
	}
	if len(claims)!=0 { return pricing.Calculation{}, invalid }
	amount, err := pricing.CalculateWith(facts.Catalog.Policy,inputs,nil)
	if err!=nil { return pricing.Calculation{}, invalid }
	payment:=facts.Catalog.Payment
	switch payment.Mode {
	case "bank_transfer":
		if payment.SurchargeMinor!=0 { return pricing.Calculation{}, invalid }
	case "cash_on_delivery", "pay_at_pickup":
		if payment.SurchargeMinor<0 || payment.SurchargeMinor>command.MaxMoney || payment.MaxMinor<1 || payment.MaxMinor>command.MaxMoney ||
			(payment.Mode=="pay_at_pickup"&&payment.SurchargeMinor!=0) { return pricing.Calculation{}, invalid }
		// Existing checkout stores surcharge beside total; collection adds it once.
		if amount.TotalMinor>command.MaxMoney-payment.SurchargeMinor || amount.TotalMinor+payment.SurchargeMinor>payment.MaxMinor {
			return pricing.Calculation{}, &AdmissionError{code:payment.Mode+"_amount_exceeds",state:"PT422"}
		}
	default: return pricing.Calculation{}, invalid
	}
	return amount,nil
}

func admissionFailure(err error, committing bool) error {
	result:=&AdmissionError{code:"order_attempt_unavailable",unknown:committing}
	var pgerr *pgconn.PgError
	if errors.As(err,&pgerr) {
		result.state=pgerr.Code
		switch pgerr.Code {
		case "PT400": result.code="invalid_request"
		case "PT401": result.code="unauthorized"
		case "PT403": result.code="forbidden"
		case "PT404": result.code="not_found"
		case "PT409":
			result.code="order_attempt_conflict"
			switch pgerr.Message {
			case "intent_conflict","idempotency_conflict","bundle_reserved","bundle_already_ordered": result.code=pgerr.Message
			}
		case "PT422": result.code="order_attempt_unavailable"
		case "40001","40P01","55P03","57014": result.code="order_attempt_retryable"
		}
		if strings.HasPrefix(pgerr.Code,"08") || pgerr.Code=="57P01" || pgerr.Code=="57P02" || pgerr.Code=="57P03" { result.code="order_attempt_retryable" }
	} else if errors.Is(err,context.Canceled)||errors.Is(err,context.DeadlineExceeded) { result.code="order_attempt_retryable" }
	return result
}
