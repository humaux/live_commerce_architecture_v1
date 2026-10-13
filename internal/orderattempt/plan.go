// Purpose: Seal/open OA1 frozen admission plans with a separate, versioned AES-256-GCM keyring.
// Depends on: standard-library crypto/JSON/time, command UUID validation and pricing.Calculation.
// Used by: Go order-attempt C1 admission and C2 inline/River execution, with trusted PG metadata.
// Invariants: order-attempt-v1 §§1–3; I01, I08, I11. No SQL, price calculation or business-fact validation.
package orderattempt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"time"

	"livecommerce/internal/command"
	"livecommerce/internal/pricing"
)

const maxPlanBytes = 1 << 20

// PlanError carries a fixed domain code, without key versions, payloads or underlying crypto errors.
type PlanError struct {
	code string
}

// Error returns only the fixed domain code.
func (e *PlanError) Error() string { return e.code }

// Code returns the stable code for executor classification; it has no side effects.
func (e *PlanError) Code() string { return e.code }

var (
	// ErrPlanInvalid means shape, authenticated binding, ciphertext or exact plaintext digest is invalid.
	ErrPlanInvalid = &PlanError{code: "order_attempt_plan_invalid"}
	// ErrPlanKeyUnavailable means a ring or required AES-256 key is definitively absent/unusable.
	// The caller must distinguish this from temporary external key-service failure before invoking Open.
	ErrPlanKeyUnavailable = &PlanError{code: "order_attempt_plan_key_unavailable"}
	// ErrPlanSealFailed means secure randomness failed; no ciphertext is returned and there is no fallback.
	ErrPlanSealFailed = &PlanError{code: "order_attempt_plan_seal_failed"}
)

// FrozenPlan is the caller's authoritative C1 snapshot. Calculation is produced by the existing
// pricing calculator; Facts carries domain-owned facts and versions. The codec does not establish
// their business validity or recompute money. This type must never be logged or put in River args.
type FrozenPlan struct {
	Schema       int                 `json:"schema"`
	AttemptID    string              `json:"attempt_id"`
	ScopeID      string              `json:"scope_id"`
	Generation   int64               `json:"generation"`
	TenantID     string              `json:"tenant_id"`
	StoreID      string              `json:"store_id"`
	PrincipalID  string              `json:"principal_id"`
	BuyerOwnerID string              `json:"buyer_owner_id"`
	AdmittedAt   time.Time           `json:"admitted_at"`
	ExecuteBy    time.Time           `json:"execute_by"`
	RequestHash  [32]byte            `json:"request_hash"`
	FactsDigest  [32]byte            `json:"facts_digest"`
	Facts        json.RawMessage     `json:"facts"`
	Calculation  pricing.Calculation `json:"calculation"`
}

// PlanBinding contains immutable metadata loaded from the locked attempt row. Open requires this
// independent expected binding, never a binding obtained from the untrusted encrypted plan itself.
type PlanBinding struct {
	Schema       int       `json:"schema"`
	AttemptID    string    `json:"attempt_id"`
	ScopeID      string    `json:"scope_id"`
	Generation   int64     `json:"generation"`
	TenantID     string    `json:"tenant_id"`
	StoreID      string    `json:"store_id"`
	PrincipalID  string    `json:"principal_id"`
	BuyerOwnerID string    `json:"buyer_owner_id"`
	AdmittedAt   time.Time `json:"admitted_at"`
	ExecuteBy    time.Time `json:"execute_by"`
	RequestHash  [32]byte  `json:"request_hash"`
	FactsDigest  [32]byte  `json:"facts_digest"`
}

// Binding returns the plan's metadata with timestamps in UTC, without modifying the plan.
// It is convenient at C1; at C2 the expected metadata must instead come from trusted PG columns.
func (p FrozenPlan) Binding() PlanBinding {
	return PlanBinding{
		Schema: p.Schema, AttemptID: p.AttemptID, ScopeID: p.ScopeID, Generation: p.Generation,
		TenantID: p.TenantID, StoreID: p.StoreID, PrincipalID: p.PrincipalID, BuyerOwnerID: p.BuyerOwnerID,
		AdmittedAt: p.AdmittedAt.UTC(), ExecuteBy: p.ExecuteBy.UTC(), RequestHash: p.RequestHash, FactsDigest: p.FactsDigest,
	}
}

// SealedPlan is the persisted envelope: Ciphertext is nonce(12) || ciphertext || tag(16).
// Digest is SHA-256 of the exact JSON plaintext. The ciphertext still contains protected PII.
type SealedPlan struct {
	KeyVersion string   `json:"key_version"`
	Ciphertext []byte   `json:"ciphertext"`
	Digest     [32]byte `json:"digest"`
}

// PlanKeys is an immutable, order-only keyring. Retain referenced old versions during rotation.
// AES expanded keys are private; String/GoString also redact them from accidental formatting.
type PlanKeys struct {
	activeVersion string
	keys          map[string]cipher.AEAD
}

// String redacts key material in ordinary formatting.
func (k *PlanKeys) String() string { return "[order-attempt plan keys]" }

// GoString redacts key material in Go-syntax formatting.
func (k *PlanKeys) GoString() string { return k.String() }

// NewPlanKeys validates and defensively copies every supplied key, including inactive versions.
// It performs no environment reads or external calls; callers own provisioning and safe retirement.
func NewPlanKeys(activeVersion string, keys map[string][]byte) (*PlanKeys, error) {
	if !validPlanKeyVersion(activeVersion) || len(keys) == 0 {
		return nil, ErrPlanKeyUnavailable
	}
	ring := &PlanKeys{activeVersion: activeVersion, keys: make(map[string]cipher.AEAD, len(keys))}
	for version, key := range keys {
		if !validPlanKeyVersion(version) || len(key) != 32 || bytes.Equal(key, make([]byte, 32)) {
			return nil, ErrPlanKeyUnavailable
		}
		owned := bytes.Clone(key)
		block, err := aes.NewCipher(owned)
		clear(owned)
		if err != nil {
			return nil, ErrPlanKeyUnavailable
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, ErrPlanKeyUnavailable
		}
		ring.keys[version] = aead
	}
	if ring.keys[activeVersion] == nil {
		return nil, ErrPlanKeyUnavailable
	}
	return ring, nil
}

// Seal authenticates the immutable admission binding and encrypts the existing snapshot.
// It consumes crypto/rand randomness but has no database/external effects and does no repricing.
func (k *PlanKeys) Seal(plan FrozenPlan) (SealedPlan, error) {
	if k == nil || k.keys[k.activeVersion] == nil {
		return SealedPlan{}, ErrPlanKeyUnavailable
	}
	// Even the smallest JSON LineAmount exceeds 64 bytes; reject huge slices before encoding.
	if !validPlanBinding(plan.Binding()) || !validPlanFacts(plan.Facts) || len(plan.Calculation.Lines) > maxPlanBytes/64 {
		return SealedPlan{}, ErrPlanInvalid
	}
	plain, err := json.Marshal(plan)
	defer clear(plain)
	if err != nil || len(plain) > maxPlanBytes {
		return SealedPlan{}, ErrPlanInvalid
	}
	sealed := SealedPlan{KeyVersion: k.activeVersion, Digest: sha256.Sum256(plain)}
	aad, err := planAAD(plan.Binding(), sealed.KeyVersion, sealed.Digest)
	if err != nil {
		return SealedPlan{}, ErrPlanInvalid
	}
	aead := k.keys[k.activeVersion]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return SealedPlan{}, ErrPlanSealFailed
	}
	sealed.Ciphertext = aead.Seal(nonce, nonce, plain, aad)
	return sealed, nil
}

// Open decrypts only a plan matching the independently loaded admission metadata and exact digest.
// All failures return a zero plan and a fixed typed code. C2 owns clock/deadline and safety checks;
// this codec verifies only the immutable 15-minute interval, never current time or business facts.
func (k *PlanKeys) Open(binding PlanBinding, sealed SealedPlan) (FrozenPlan, error) {
	if k == nil {
		return FrozenPlan{}, ErrPlanKeyUnavailable
	}
	if !validPlanBinding(binding) || !validPlanKeyVersion(sealed.KeyVersion) || sealed.Digest == ([32]byte{}) {
		return FrozenPlan{}, ErrPlanInvalid
	}
	aead := k.keys[sealed.KeyVersion]
	if aead == nil {
		return FrozenPlan{}, ErrPlanKeyUnavailable
	}
	prefix := aead.NonceSize()
	if len(sealed.Ciphertext) <= prefix+aead.Overhead() || len(sealed.Ciphertext) > maxPlanBytes+prefix+aead.Overhead() {
		return FrozenPlan{}, ErrPlanInvalid
	}
	// Normalize time zones before building AAD: PG instants must not depend on connection timezone.
	binding.AdmittedAt, binding.ExecuteBy = binding.AdmittedAt.UTC(), binding.ExecuteBy.UTC()
	aad, err := planAAD(binding, sealed.KeyVersion, sealed.Digest)
	if err != nil {
		return FrozenPlan{}, ErrPlanInvalid
	}
	plain, err := aead.Open(nil, sealed.Ciphertext[:prefix], sealed.Ciphertext[prefix:], aad)
	if err != nil {
		return FrozenPlan{}, ErrPlanInvalid
	}
	defer clear(plain)
	if len(plain) > maxPlanBytes || sha256.Sum256(plain) != sealed.Digest {
		return FrozenPlan{}, ErrPlanInvalid
	}
	var plan FrozenPlan
	if err := json.Unmarshal(plain, &plan); err != nil || plan.Binding() != binding || !validPlanFacts(plan.Facts) {
		return FrozenPlan{}, ErrPlanInvalid
	}
	return plan, nil
}

func validPlanBinding(b PlanBinding) bool {
	return b.Schema == 1 && b.Generation > 0 && command.ValidID(b.AttemptID) && command.ValidID(b.ScopeID) &&
		command.ValidID(b.TenantID) && command.ValidID(b.StoreID) && command.ValidID(b.PrincipalID) && command.ValidID(b.BuyerOwnerID) &&
		!b.AdmittedAt.IsZero() && !b.ExecuteBy.IsZero() && b.AdmittedAt.Add(15*time.Minute).Equal(b.ExecuteBy) &&
		b.RequestHash != ([32]byte{}) && b.FactsDigest != ([32]byte{})
}

func validPlanFacts(facts json.RawMessage) bool {
	return len(facts) > 0 && len(facts) <= maxPlanBytes && json.Valid(facts)
}

func validPlanKeyVersion(version string) bool {
	if len(version) == 0 || len(version) > 64 {
		return false
	}
	for _, c := range version {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

func planAAD(binding PlanBinding, version string, digest [32]byte) ([]byte, error) {
	// Wire v1 binds the namespace, retained key version, trusted metadata and exact plaintext digest.
	return json.Marshal(struct {
		Domain     string      `json:"domain"`
		KeyVersion string      `json:"key_version"`
		Binding    PlanBinding `json:"binding"`
		Digest     [32]byte    `json:"digest"`
	}{"livecommerce/order-attempt-plan/v1", version, binding, digest})
}
