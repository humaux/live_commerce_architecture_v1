package buyer

import (
	"context"
	"crypto/sha256"
	"time"
)

// RegisterForTrustedStore registers a trusted server's existing random token.
// The database resolves a repeated hash to the original owner and expiry;
// this method never mints or replaces a token after an uncertain response.
func (s *Service) RegisterForTrustedStore(ctx context.Context, storeID, token string) (Capability, error) {
	if ctx == nil || s == nil || s.issuerPool == nil || !validUUID(storeID) || !validToken(token) ||
		s.ttlSeconds < int64(minimumTTL/time.Second) || s.ttlSeconds > int64(maximumTTL/time.Second) {
		return Capability{}, ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var capability Capability
	err := s.issuerPool.QueryRow(callCtx, `SELECT tenant_id::text,store_id::text,owner_id::text,session_id::text,expires_at
		FROM buyer.register_capability_limited($1::uuid,$2,$3)`, storeID, hash[:], s.ttlSeconds).
		Scan(&capability.Scope.TenantID, &capability.Scope.StoreID, &capability.Scope.OwnerID, &capability.Scope.SessionID, &capability.ExpiresAt)
	if err != nil {
		return Capability{}, translate(callCtx, err)
	}
	capability.Token = token
	return capability, nil
}

// RetireForTrustedStore persists a revoked hash even when bootstrap has not
// arrived yet. It must not be replaced by unknown-token Revoke (a no-op).
func (s *Service) RetireForTrustedStore(ctx context.Context, storeID, token string) error {
	if ctx == nil || s == nil || s.issuerPool == nil || !validUUID(storeID) || !validToken(token) ||
		s.ttlSeconds < int64(minimumTTL/time.Second) || s.ttlSeconds > int64(maximumTTL/time.Second) {
		return ErrInvalid
	}
	hash := sha256.Sum256([]byte(token))
	callCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	tx, err := s.issuerPool.Begin(callCtx)
	if err != nil {
		return translate(callCtx, err)
	}
	defer rollback(tx)
	// Like revocation, retirement waits behind a previously admitted operation,
	// bounded by the request deadline instead of the runtime's 1s lock timeout.
	if _, err = tx.Exec(callCtx, `SELECT set_config('lock_timeout','0',true),
		set_config('statement_timeout',$1,true),set_config('idle_in_transaction_session_timeout',$1,true)`, requestTimeout.String()); err == nil {
		_, err = tx.Exec(callCtx, `SELECT buyer.retire_capability($1::uuid,$2,$3)`, storeID, hash[:], s.ttlSeconds)
	}
	if err == nil {
		err = tx.Commit(callCtx)
	}
	if err != nil {
		return translate(callCtx, err)
	}
	return nil
}
