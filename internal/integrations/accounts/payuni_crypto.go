package accounts

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
