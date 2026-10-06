// Purpose: exported PAYUNi notify-credential custody helpers (SealPayuni / OpenPayuniNotify) bound to the credential AAD.
// Depends on: this package's credentialAAD/seal primitives (AES-GCM); no network, no DB.
// Used by: internal/payments/payuninotify (inbox), internal/payments (payuni_activation.go: opens the scoped activation
//   material and derives the verification notify token), tests/foundation payuni fixtures, the registrar seal path.
// Invariants: AAD binds tenant/store/connection/environment/account/version so a sealed envelope opens in one scope only.

package accounts

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// PAYUNi credentials are one HashKey/HashIV per connection: the hosted, query and notify
// paths all bind to the same credentialAAD, so a notification is verified with the exact
// material the registrar sealed. These exported helpers exist for the notify ingress (which
// lives outside package accounts) and for test fixtures; the registrar continues to use the
// private seal with newAAD.

type PayuniCredentialScope struct {
	TenantID, StoreID, ConnectionID, Environment, AccountID string
	CredentialVersion                                       int64
}

// validPayuniScope mirrors validAAD's non-empty identity checks; tenant/store/connection
// identifiers are UUIDs in practice but are not re-validated here so the notify path can
// never diverge from what the registrar accepted.
func validPayuniScope(s PayuniCredentialScope) bool {
	return s.TenantID != "" && s.StoreID != "" && s.ConnectionID != "" &&
		validEnvironment(s.Environment) && validAccountID(s.AccountID) && s.CredentialVersion > 0
}

func payuniAAD(s PayuniCredentialScope) credentialAAD {
	return credentialAAD{FormatVersion: 1, TenantID: s.TenantID, StoreID: s.StoreID,
		ConnectionID: s.ConnectionID, Provider: "payuni", Environment: s.Environment,
		AccountID: s.AccountID, CredentialVersion: s.CredentialVersion}
}

// SealPayuni seals a connection's PAYUNi HashKey/HashIV for custody. It is used by tests and
// (if needed) a registrar; it never validates the key beyond custody grammar.
func (k *Keyring) SealPayuni(scope PayuniCredentialScope, credentials Credentials) (string, []byte, []byte, error) {
	if !validPayuniScope(scope) || !validCredentials(credentials) {
		return "", nil, nil, errInvalidSecret
	}
	return k.seal(payuniAAD(scope), credentials)
}

// OpenPayuniNotify opens a connection's PAYUNi credentials for one notification verification.
// The caller may retry with CredentialVersion-1 for the rotation grace window.
func (k *Keyring) OpenPayuniNotify(scope PayuniCredentialScope, keyID string, nonce, ciphertext []byte) (Credentials, error) {
	if !validPayuniScope(scope) {
		return Credentials{}, errInvalidSecret
	}
	return k.open(payuniAAD(scope), keyID, nonce, ciphertext)
}

// OpenPayuniActivation opens the sealed envelope returned by payments.load_payuni_activation_material
// (w4-02b) for exactly one verification query/hosted form or one LIVE probe; same AAD binding as the notify path.
func (k *Keyring) OpenPayuniActivation(scope PayuniCredentialScope, keyID string, nonce, ciphertext []byte) (Credentials, error) {
	return k.OpenPayuniNotify(scope, keyID, nonce, ciphertext)
}

// PayuniVerifyNotifyToken derives the notify endpoint token of one connection+profile from the keyring's independent
// replay key (HMAC-SHA256, domain separated), so the plaintext token can be rebuilt for every verification form while only
// its sha256 is stored (payments.payuni_notify_endpoints.token_hash). The token routes a callback; it is not authentication.
func (k *Keyring) PayuniVerifyNotifyToken(connectionID, profile string) (token string, tokenHash []byte, err error) {
	if k == nil || len(k.replayKey) != 32 || connectionID == "" || (profile != "PROVIDER_MOCK" && profile != "SANDBOX") {
		return "", nil, errInvalidSecret
	}
	mac := hmac.New(sha256.New, k.replayKey)
	_, _ = mac.Write([]byte("payuni-verify-notify-endpoint-v1|" + connectionID + "|" + profile))
	token = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}
