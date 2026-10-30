// Purpose: contact envelope tamper, scope and old-key replay counterexamples.
// Depends on: contact.go and synthetic payload-key configuration only.
// Used by: the existing Go package CI suite; never a provider or production key.
// Invariants: wrong store/id/key/nonce/ciphertext cannot release plaintext; rotation retains original comparisons.
package storefront

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestContactEnvelopeAuthenticationAndRotation(t *testing.T) {
	key := func(n byte) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{n}, 32)) }
	config := func(active string) {
		t.Setenv("COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID", active)
		raw, _ := json.Marshal(map[string]any{"keys": []map[string]string{{"id": "old", "key_base64": key(17)}, {"id": "new", "key_base64": key(18)}}})
		t.Setenv("COMMERCE_META_PAYLOAD_KEYS_JSON", string(raw))
	}
	config("old")
	k, err := LoadContactKeys()
	if err != nil {
		t.Fatal(err)
	}
	store := "00000000-0000-4000-8000-000000000211"
	in := ContactInput{Name: "Synthetic buyer", Email: "buyer@example.test", Message: "<b>literal</b>"}
	e, err := k.Seal(store, store, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := k.Seal(store, store, in)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == e.ID || bytes.Equal(second.Nonce, e.Nonce) {
		t.Fatal("envelope reused identity or nonce")
	}
	config("new")
	rotated, err := LoadContactKeys()
	if err != nil {
		t.Fatal(err)
	}
	detail, err := rotated.Open(e)
	if err != nil || !MatchesContact(detail, in) {
		t.Fatal("old-key replay lost original fields")
	}
	changed := in
	changed.Message = "different"
	if MatchesContact(detail, changed) {
		t.Fatal("changed command matched")
	}
	for _, change := range []func(*ContactEnvelope){
		func(e *ContactEnvelope) { e.TenantID = "00000000-0000-4000-8000-000000000214" },
		func(e *ContactEnvelope) { e.StoreID = "00000000-0000-4000-8000-000000000212" },
		func(e *ContactEnvelope) { e.ID = "00000000-0000-4000-8000-000000000213" },
		func(e *ContactEnvelope) { e.KeyID = "new" },
		func(e *ContactEnvelope) { e.Nonce[0] ^= 1 },
		func(e *ContactEnvelope) { e.Ciphertext[0] ^= 1 },
	} {
		bad := e
		bad.Nonce = bytes.Clone(e.Nonce)
		bad.Ciphertext = bytes.Clone(e.Ciphertext)
		change(&bad)
		if _, err := rotated.Open(bad); err == nil {
			t.Fatal("tampered envelope opened")
		}
	}
}
