package inbox

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	testEventID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	testTenant  = "11111111-2222-3333-4444-555555555555"
	testStore   = "22222222-3333-4444-5555-666666666666"
	testRoute   = "33333333-4444-5555-6666-777777777777"
	testKeyID   = "test-key-1"
)

func testKey() [32]byte {
	var k [32]byte
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func testContext(payloadHash string) eventContext {
	return eventContext{
		EventID: testEventID, AppID: "1234567890", Object: "page",
		EventKey: strings.Repeat("ab", 32), PayloadHash: payloadHash,
		TenantID: testTenant, StoreID: testStore, RouteID: testRoute, RouteEpoch: 1,
	}
}

// sealForTest seals exactly like the frozen meta consumer: AES-256-GCM over the positional AAD.
func sealForTest(t *testing.T, key [32]byte, keyID string, c eventContext, plaintext []byte) (nonce, ciphertext []byte) {
	t.Helper()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	nonce = make([]byte, payloadNonce)
	return nonce, aead.Seal(nil, nonce, plaintext, c.aad(keyID))
}

func TestNewServiceRequiresKeyring(t *testing.T) {
	if _, err := NewService(nil); !errors.Is(err, ErrConfig) {
		t.Fatalf("nil keyring: want ErrConfig, got %v", err)
	}
	if _, err := NewService(&Keyring{}); err != nil {
		t.Fatalf("non-nil keyring should construct: %v", err)
	}
}

func TestKeyringOpenRoundTrip(t *testing.T) {
	key := testKey()
	k, err := newKeyring(testKeyID, map[string][]byte{testKeyID: key[:]})
	if err != nil {
		t.Fatalf("newKeyring: %v", err)
	}
	plaintext := []byte(`{"sender":{"id":"111"},"recipient":{"id":"1234567890"},"message":{"mid":"m_1","text":"hello"}}`)
	c := testContext(digest(plaintext))
	nonce, ct := sealForTest(t, key, testKeyID, c, plaintext)
	got, err := k.open(c, testKeyID, nonce, ct)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("plaintext mismatch: %q != %q", got, plaintext)
	}
}

func TestKeyringOpenRejectsTamper(t *testing.T) {
	key := testKey()
	k, _ := newKeyring(testKeyID, map[string][]byte{testKeyID: key[:]})
	plaintext := []byte(`{"x":1}`)
	c := testContext(digest(plaintext))
	nonce, ct := sealForTest(t, key, testKeyID, c, plaintext)
	ct[len(ct)-1] ^= 0x01
	if _, err := k.open(c, testKeyID, nonce, ct); !errors.Is(err, ErrPayload) {
		t.Fatalf("tampered ciphertext: want ErrPayload, got %v", err)
	}
}

func TestKeyringOpenRejectsWrongKey(t *testing.T) {
	key := testKey()
	other := testKey()
	other[0] ^= 0xff
	k, _ := newKeyring(testKeyID, map[string][]byte{testKeyID: key[:]})
	plaintext := []byte(`{"x":1}`)
	c := testContext(digest(plaintext))
	nonce, ct := sealForTest(t, other, testKeyID, c, plaintext)
	if _, err := k.open(c, testKeyID, nonce, ct); !errors.Is(err, ErrPayload) {
		t.Fatalf("wrong key: want ErrPayload, got %v", err)
	}
}

func TestKeyringOpenRejectsDigestMismatch(t *testing.T) {
	key := testKey()
	k, _ := newKeyring(testKeyID, map[string][]byte{testKeyID: key[:]})
	plaintext := []byte(`{"x":1}`)
	nonce, ct := sealForTest(t, key, testKeyID, testContext(digest(plaintext)), plaintext)
	c := testContext(digest([]byte("other")))
	if _, err := k.open(c, testKeyID, nonce, ct); !errors.Is(err, ErrPayload) {
		t.Fatalf("digest mismatch: want ErrPayload, got %v", err)
	}
}

func TestKeyringOpenRejectsBadContext(t *testing.T) {
	key := testKey()
	k, _ := newKeyring(testKeyID, map[string][]byte{testKeyID: key[:]})
	plaintext := []byte(`{"x":1}`)
	c := testContext(digest(plaintext))
	c.RouteEpoch = 0 // event context requires RouteEpoch > 0
	nonce, ct := sealForTest(t, key, testKeyID, testContext(digest(plaintext)), plaintext)
	if _, err := k.open(c, testKeyID, nonce, ct); !errors.Is(err, ErrPayload) {
		t.Fatalf("bad context: want ErrPayload, got %v", err)
	}
}

func TestLoadKeyring(t *testing.T) {
	key := testKey()
	encoded := base64.StdEncoding.EncodeToString(key[:])
	env := map[string]string{
		"COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID": testKeyID,
		"COMMERCE_META_PAYLOAD_KEYS_JSON":     `{"keys":[{"id":"test-key-1","key_base64":"` + encoded + `"}]}`,
	}
	k, err := LoadKeyring(func(name string) string { return env[name] })
	if err != nil {
		t.Fatalf("valid env: %v", err)
	}
	if _, ok := k.keys[testKeyID]; !ok {
		t.Fatal("active key not loaded")
	}
	if _, err := LoadKeyring(nil); !errors.Is(err, ErrConfig) {
		t.Fatalf("nil getenv: want ErrConfig, got %v", err)
	}
	bad := map[string]string{
		"COMMERCE_META_PAYLOAD_ACTIVE_KEY_ID": testKeyID,
		"COMMERCE_META_PAYLOAD_KEYS_JSON":     `{"keys":[{"id":"test-key-1","key_base64":"not-b64"}]}`,
	}
	if _, err := LoadKeyring(func(name string) string { return bad[name] }); !errors.Is(err, ErrConfig) {
		t.Fatalf("malformed key: want ErrConfig, got %v", err)
	}
}

func TestReplayMessage(t *testing.T) {
	const assetID = "1234567890"
	valid := []byte(`{"sender":{"id":"55555"},"recipient":{"id":"1234567890"},"message":{"mid":"m_xyz","text":"hello","attachments":[{"type":"image"}]}}`)
	view, err := replayMessage(valid, assetID)
	if err != nil {
		t.Fatalf("valid message: %v", err)
	}
	if view.Text != "hello" || len(view.Attachments) != 1 || view.Attachments[0].Type != "image" {
		t.Fatalf("unexpected extraction: %+v", view)
	}

	cases := []struct {
		name string
		body string
	}{
		{"sender is page", `{"sender":{"id":"1234567890"},"recipient":{"id":"1234567890"},"message":{"mid":"m_1","text":"x"}}`},
		{"recipient not asset", `{"sender":{"id":"55555"},"recipient":{"id":"99999"},"message":{"mid":"m_1","text":"x"}}`},
		{"echo true", `{"sender":{"id":"55555"},"recipient":{"id":"1234567890"},"message":{"mid":"m_1","text":"x","is_echo":true}}`},
		{"bad mid", `{"sender":{"id":"55555"},"recipient":{"id":"1234567890"},"message":{"mid":"","text":"x"}}`},
		{"missing message", `{"sender":{"id":"55555"},"recipient":{"id":"1234567890"}}`},
	}
	for _, c := range cases {
		if _, err := replayMessage([]byte(c.body), assetID); !errors.Is(err, errUnreadable) {
			t.Errorf("%s: want errUnreadable, got %v", c.name, err)
		}
	}
}

func TestDatabaseError(t *testing.T) {
	for _, code := range []string{"PT400", "PT403", "PT404", "PT409", "PT422"} {
		pg := &pgconn.PgError{Code: code, Message: "x"}
		if got := databaseError(pg); got != pg {
			t.Errorf("%s: want passthrough of the same error, got %v", code, got)
		}
	}
	if got := databaseError(errors.New("boom")); !errors.Is(got, ErrDatabase) {
		t.Fatalf("generic error: want ErrDatabase, got %v", got)
	}
}
