package accounts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func testKeyring(t *testing.T) *Keyring {
	t.Helper()
	k, err := NewKeyring("current", map[string][]byte{
		"current": bytes.Repeat([]byte{0x35}, 32),
		"old":     bytes.Repeat([]byte{0x71}, 32),
	}, bytes.Repeat([]byte{0xa4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func testAAD() credentialAAD {
	return credentialAAD{1, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		"cccccccc-cccc-cccc-cccc-cccccccccccc", "payuni", "SANDBOX", "Mer_ID-1", 1}
}

func TestSealRoundTripTamperAndAAD(t *testing.T) {
	k := testKeyring(t)
	aad := testAAD()
	want := Credentials{"hash-key-secret", "hash-iv-secret"}
	id, nonce, ciphertext, err := k.seal(aad, want)
	if err != nil || id != "current" || len(nonce) != 12 || len(ciphertext) < 17 {
		t.Fatalf("seal: id=%q nonce=%d ciphertext=%d err=%v", id, len(nonce), len(ciphertext), err)
	}
	got, err := k.open(aad, id, nonce, ciphertext)
	if err != nil || got != want {
		t.Fatalf("roundtrip: got=%v err=%v", got, err)
	}
	_, nonce2, ciphertext2, err := k.seal(aad, want)
	if err != nil || bytes.Equal(nonce, nonce2) || bytes.Equal(ciphertext, ciphertext2) {
		t.Fatal("nonce or ciphertext repeated")
	}
	for name, change := range map[string]func(*credentialAAD){
		"format":      func(a *credentialAAD) { a.FormatVersion++ },
		"tenant":      func(a *credentialAAD) { a.TenantID = "other" },
		"store":       func(a *credentialAAD) { a.StoreID = "other" },
		"connection":  func(a *credentialAAD) { a.ConnectionID = "other" },
		"provider":    func(a *credentialAAD) { a.Provider = "other" },
		"environment": func(a *credentialAAD) { a.Environment = "LIVE" },
		"account":     func(a *credentialAAD) { a.AccountID = "Other" },
		"version":     func(a *credentialAAD) { a.CredentialVersion++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := aad
			change(&changed)
			if _, err := k.open(changed, id, nonce, ciphertext); err == nil {
				t.Fatal("cross-AAD decryption succeeded")
			}
		})
	}
	badNonce := bytes.Clone(nonce)
	badNonce[0] ^= 1
	badCiphertext := bytes.Clone(ciphertext)
	badCiphertext[0] ^= 1
	badTag := bytes.Clone(ciphertext)
	badTag[len(badTag)-1] ^= 1
	for _, altered := range []struct{ nonce, ciphertext []byte }{
		{badNonce, ciphertext}, {nonce, badCiphertext}, {nonce, badTag}, {nonce[:11], ciphertext},
	} {
		if _, err := k.open(aad, id, altered.nonce, altered.ciphertext); err == nil {
			t.Fatal("tampered encrypted credential accepted")
		}
	}
}

func TestKeyringCopyAndOldKey(t *testing.T) {
	active := bytes.Repeat([]byte{0x42}, 32)
	old := bytes.Repeat([]byte{0x24}, 32)
	replay := bytes.Repeat([]byte{0x19}, 32)
	keys := map[string][]byte{"active": active, "old": old}
	k, err := NewKeyring("active", keys, replay)
	if err != nil {
		t.Fatal(err)
	}
	aad := testAAD()
	want := Credentials{"key", "iv"}
	k.activeID = "old"
	id, nonce, ciphertext, err := k.seal(aad, want)
	if err != nil {
		t.Fatal(err)
	}
	for i := range active {
		active[i] = 0
	}
	for i := range old {
		old[i] = 0
	}
	for i := range replay {
		replay[i] = 0
	}
	delete(keys, "old")
	if got, err := k.open(aad, id, nonce, ciphertext); err != nil || got != want {
		t.Fatalf("caller mutation affected keyring: %v %v", got, err)
	}
	kWithoutOld, err := NewKeyring("active", map[string][]byte{"active": bytes.Repeat([]byte{0x42}, 32)}, bytes.Repeat([]byte{0x19}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kWithoutOld.open(aad, id, nonce, ciphertext); !errors.Is(err, errInvalidSecret) {
		t.Fatalf("missing old key must fail closed: %v", err)
	}
}

func TestKeyringAndCredentialBounds(t *testing.T) {
	validKey := bytes.Repeat([]byte{1}, 32)
	replay := bytes.Repeat([]byte{2}, 32)
	for _, tc := range []struct {
		id   string
		keys map[string][]byte
		rkey []byte
	}{
		{"", map[string][]byte{"a": validKey}, replay},
		{"missing", map[string][]byte{"a": validKey}, replay},
		{"bad!", map[string][]byte{"bad!": validKey}, replay},
		{"a", map[string][]byte{"a": validKey}, replay[:31]},
		{"a", map[string][]byte{"a": validKey[:31]}, replay},
		{"a", map[string][]byte{"a": validKey}, validKey},
		{"a", nil, replay},
	} {
		if _, err := NewKeyring(tc.id, tc.keys, tc.rkey); err == nil {
			t.Fatalf("invalid keyring accepted: id=%q keys=%d replay=%d", tc.id, len(tc.keys), len(tc.rkey))
		}
	}
	max := strings.Repeat("X", 512)
	for _, c := range []Credentials{{"a", "b"}, {max, max}, {"!~", "\\\""}} {
		if !validCredentials(c) {
			t.Fatal("valid credential rejected")
		}
	}
	for _, c := range []Credentials{{"", "b"}, {"a", ""}, {" key", "iv"}, {"key ", "iv"},
		{strings.Repeat("x", 513), "iv"}, {"a\nb", "iv"}, {"é", "iv"}} {
		if validCredentials(c) {
			t.Fatal("invalid credential accepted")
		}
	}
}

func TestCredentialsNeverFormatOrJSONAsPlaintext(t *testing.T) {
	c := Credentials{"secret-hash-key", "secret-hash-iv"}
	for _, rendered := range []string{fmt.Sprint(c), fmt.Sprintf("%#v", c), string(mustJSON(t, c)),
		string(mustJSON(t, CreateInput{Credentials: c})), string(mustJSON(t, RotateInput{Credentials: c}))} {
		if strings.Contains(rendered, c.HashKey) || strings.Contains(rendered, c.HashIV) {
			t.Fatalf("credential leaked: %s", rendered)
		}
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
