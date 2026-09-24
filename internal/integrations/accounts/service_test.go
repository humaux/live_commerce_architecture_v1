package accounts

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/command"
	"livecommerce/internal/platform"
)

func TestServiceInputAndFingerprint(t *testing.T) {
	if _, err := New(nil, nil); !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("nil dependencies accepted: %v", err)
	}
	for _, id := range []string{"Merchant_1", strings.Repeat("A", 64)} {
		if !validAccountID(id) {
			t.Fatalf("valid account ID rejected: %q", id)
		}
	}
	for _, id := range []string{"", "a/b", "a:b", "é", strings.Repeat("A", 65)} {
		if validAccountID(id) {
			t.Fatalf("invalid account ID accepted: %q", id)
		}
	}
	if !validEnvironment("SANDBOX") || !validEnvironment("LIVE") || validEnvironment("READY") || validEnvironment("") {
		t.Fatal("environment validation mismatch")
	}
	makeService := func(replayByte byte, encryptByte byte) *Service {
		t.Helper()
		keys, err := NewKeyring("k1", map[string][]byte{"k1": bytes.Repeat([]byte{encryptByte}, 32)}, bytes.Repeat([]byte{replayByte}, 32))
		if err != nil {
			t.Fatal(err)
		}
		return &Service{keys: keys}
	}
	scope := platform.Scope{TenantID: "tenant-a", StoreID: "store-a"}
	c := Credentials{"hash-key", "hash-iv"}
	s := makeService(1, 2)
	first := s.fingerprint(scope, c)
	const golden = "8893dfddde5f8130f187c286bdf6258599583baf81034a3432f865bea5cc37c3"
	if first != golden {
		t.Fatalf("permanent replay HMAC protocol changed: got %s", first)
	}
	if len(first) != 64 || first != makeService(1, 3).fingerprint(scope, c) {
		t.Fatal("replay digest changed with encryption key")
	}
	if first == makeService(4, 2).fingerprint(scope, c) ||
		first == s.fingerprint(platform.Scope{TenantID: "tenant-b", StoreID: scope.StoreID}, c) ||
		first == s.fingerprint(platform.Scope{TenantID: scope.TenantID, StoreID: "store-b"}, c) ||
		first == s.fingerprint(scope, Credentials{c.HashKey + "x", c.HashIV}) ||
		first == s.fingerprint(scope, Credentials{c.HashKey, c.HashIV + "x"}) {
		t.Fatal("replay digest failed to bind key, scope, or full credential")
	}
	if strings.Contains(first, c.HashKey) || strings.Contains(first, c.HashIV) {
		t.Fatal("digest exposed plaintext")
	}
}
