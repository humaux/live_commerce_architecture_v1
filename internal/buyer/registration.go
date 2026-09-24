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
		FROM buyer.register_capability($1::uuid,$2,$3)`, storeID, hash[:], s.ttlSeconds).
		Scan(&capability.Scope.TenantID, &capability.Scope.StoreID, &capability.Scope.OwnerID, &capability.Scope.SessionID, &capability.ExpiresAt)
	if err != nil {
		return Capability{}, translate(callCtx, err)
	}
	capability.Token = token
	return capability, nil
}
