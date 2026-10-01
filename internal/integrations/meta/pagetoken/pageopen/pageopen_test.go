package pageopen

import (
	"crypto/ecdh"
	"crypto/hpke"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"livecommerce/internal/integrations/meta/pagetoken"
)

// Ring builds a matching private/public key pair set for tests (id -> raw key) and the two env maps.
func rings(t *testing.T, ids ...string) (privEnv, pubEnv map[string]string) {
	t.Helper()
	var priv, pub []string
	for _, id := range ids {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			t.Fatal(err)
		}
		sk, err := hpke.DHKEM(ecdh.X25519()).NewPrivateKey(raw)
		if err != nil {
			t.Fatal(err)
		}
		priv = append(priv, `{"id":"`+id+`","private_key_base64":"`+base64.StdEncoding.EncodeToString(raw)+`"}`)
		pub = append(pub, `{"id":"`+id+`","public_key_base64":"`+base64.StdEncoding.EncodeToString(sk.PublicKey().Bytes())+`"}`)
	}
	return map[string]string{"COMMERCE_META_PAGE_HPKE_PRIVATE_KEYS": `{"keys":[` + strings.Join(priv, ",") + `]}`},
		map[string]string{"COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON": `{"keys":[` + strings.Join(pub, ",") + `]}`, "COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID": ids[len(ids)-1]}
}

func get(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

var scope = pagetoken.Scope{TenantID: "00000000-0000-4000-8000-000000000001", StoreID: "00000000-0000-4000-8000-000000000002",
	BindingID: "00000000-0000-4000-8000-000000000003", Provider: "facebook", AssetID: "123", Version: 1}

// The seal half (public keys) round-trips with the open half (private ring) and is bound to every scope field.
func TestSealOpenRoundTripAndBinding(t *testing.T) {
	privEnv, pubEnv := rings(t, "k1", "k2")
	seal, err := pagetoken.LoadSealKeys(get(pubEnv))
	if err != nil {
		t.Fatal(err)
	}
	ring, err := LoadKeyring(get(privEnv))
	if err != nil {
		t.Fatal(err)
	}
	keyID, enc, ct, err := seal.Seal(scope, []byte("SENTINEL-PAGE-TOKEN"))
	if err != nil || keyID != "k2" || len(enc) != pagetoken.EncSize {
		t.Fatalf("seal: %q %d %v", keyID, len(enc), err)
	}
	if plain, err := ring.Open(scope, keyID, enc, ct); err != nil || string(plain) != "SENTINEL-PAGE-TOKEN" {
		t.Fatalf("open: %q %v", plain, err)
	}
	for name, mod := range map[string]func(s *pagetoken.Scope){
		"tenant":   func(s *pagetoken.Scope) { s.TenantID = "00000000-0000-4000-8000-000000000009" },
		"store":    func(s *pagetoken.Scope) { s.StoreID = "00000000-0000-4000-8000-000000000009" },
		"binding":  func(s *pagetoken.Scope) { s.BindingID = "00000000-0000-4000-8000-000000000009" },
		"provider": func(s *pagetoken.Scope) { s.Provider = "instagram" },
		"asset":    func(s *pagetoken.Scope) { s.AssetID = "124" },
		"version":  func(s *pagetoken.Scope) { s.Version = 2 },
	} {
		s := scope
		mod(&s)
		if _, err := ring.Open(s, keyID, enc, ct); err == nil {
			t.Errorf("a ciphertext opened under another %s", name)
		}
	}
	if _, err := ring.Open(scope, "k1", enc, ct); err == nil {
		t.Error("opened under the wrong key id")
	}
	bad := append([]byte(nil), ct...)
	bad[0] ^= 1
	if _, err := ring.Open(scope, keyID, enc, bad); err == nil {
		t.Error("tampered ciphertext opened")
	}
}

// The seal half alone (what cmd/api holds) has no way to open, and refuses malformed config and tokens.
func TestSealKeysConfigAndTokenShape(t *testing.T) {
	_, pubEnv := rings(t, "k1")
	if _, err := pagetoken.LoadSealKeys(get(map[string]string{})); err == nil {
		t.Error("empty config accepted")
	}
	bad := map[string]string{"COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON": pubEnv["COMMERCE_META_PAGE_HPKE_PUBLIC_KEYS_JSON"], "COMMERCE_META_PAGE_HPKE_ACTIVE_KEY_ID": "nope"}
	if _, err := pagetoken.LoadSealKeys(get(bad)); err == nil {
		t.Error("active id outside the ring accepted")
	}
	seal, _ := pagetoken.LoadSealKeys(get(pubEnv))
	for _, tok := range []string{"", "has space", strings.Repeat("a", 4097)} {
		if _, _, _, err := seal.Seal(scope, []byte(tok)); err == nil {
			t.Errorf("token %q sealed", tok)
		}
	}
	if _, err := LoadKeyring(get(pubEnv)); err == nil {
		t.Error("a public ring loaded as a private ring")
	}
}
