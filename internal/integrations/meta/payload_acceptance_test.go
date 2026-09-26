package meta

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const (
	payloadID       = "11111111-1111-1111-1111-111111111111"
	otherPayloadID  = "22222222-2222-2222-2222-222222222222"
	payloadTenantID = "33333333-3333-3333-3333-333333333333"
	payloadStoreID  = "44444444-4444-4444-4444-444444444444"
	payloadRouteID  = "55555555-5555-5555-5555-555555555555"
)

func payloadDigest(plaintext []byte) string {
	sum := sha256.Sum256(plaintext)
	return hex.EncodeToString(sum[:])
}

func acceptanceKeyring(t *testing.T) *PayloadKeyring {
	t.Helper()
	k, err := NewPayloadKeyring("active", map[string][]byte{
		"active": bytes.Repeat([]byte{0x31}, 32),
		"old":    bytes.Repeat([]byte{0x72}, 32),
	})
	if err != nil {
		t.Fatalf("valid keyring: %v", err)
	}
	return k
}

func rawPayloadContext(plaintext []byte) payloadContext {
	return payloadContext{Class: "raw", ID: payloadID, AppID: "123456", Object: "page", BodyHash: payloadDigest(plaintext)}
}

func eventPayloadContext(plaintext []byte) payloadContext {
	return payloadContext{
		Class: "event", ID: payloadID, AppID: "123456", Object: "page",
		EventKey: strings.Repeat("a", 64), PayloadHash: payloadDigest(plaintext),
		TenantID: payloadTenantID, StoreID: payloadStoreID, RouteID: payloadRouteID, RouteEpoch: 7,
	}
}

func quarantinePayloadContext(plaintext []byte) payloadContext {
	return payloadContext{
		Class: "quarantine", ID: payloadID, AppID: "123456", Object: "page",
		EventKey: strings.Repeat("a", 64), PayloadHash: payloadDigest(plaintext),
	}
}

func requireNoPlaintext(t *testing.T, k *PayloadKeyring, ctx payloadContext, env sealedPayload) {
	t.Helper()
	got, err := k.open(ctx, env)
	if err == nil || len(got) != 0 {
		t.Fatalf("invalid envelope returned plaintext=%t error=%t", len(got) != 0, err != nil)
	}
}

// This builds the frozen wire AAD from the contract, never from the implementation.
func contractAAD(t *testing.T, ctx payloadContext, keyID string) []byte {
	t.Helper()
	encoded, err := json.Marshal([]any{
		"livecommerce/meta-payload/v1", ctx.Class, ctx.ID, ctx.AppID, ctx.Object,
		ctx.BodyHash, ctx.EventKey, ctx.PayloadHash, ctx.TenantID, ctx.StoreID,
		ctx.RouteID, ctx.RouteEpoch, keyID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestPayloadKeyringBoundsOwnershipAndRotation(t *testing.T) {
	good := bytes.Repeat([]byte{0x31}, 32)
	for _, tc := range []struct {
		name, active string
		keys         map[string][]byte
	}{
		{"no keys", "active", nil},
		{"missing active", "missing", map[string][]byte{"active": good}},
		{"empty active", "", map[string][]byte{"active": good}},
		{"invalid id", "bad!", map[string][]byte{"bad!": good}},
		{"long id", strings.Repeat("a", 65), map[string][]byte{strings.Repeat("a", 65): good}},
		{"short key", "active", map[string][]byte{"active": good[:31]}},
		{"long key", "active", map[string][]byte{"active": append(bytes.Clone(good), 0)}},
		{"zero key", "active", map[string][]byte{"active": make([]byte, 32)}},
		{"reused bytes", "active", map[string][]byte{"active": good, "alias": bytes.Clone(good)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewPayloadKeyring(tc.active, tc.keys); err == nil {
				t.Fatal("invalid key inventory accepted")
			}
		})
	}
	seventeen := map[string][]byte{}
	for i := 0; i < 17; i++ {
		seventeen[fmt.Sprintf("k%d", i)] = bytes.Repeat([]byte{byte(i + 1)}, 32)
	}
	if _, err := NewPayloadKeyring("k0", seventeen); err == nil {
		t.Fatal("17-key inventory accepted")
	}
	sixteen := map[string][]byte{}
	for key, value := range seventeen {
		if key != "k16" {
			sixteen[key] = value
		}
	}
	if _, err := NewPayloadKeyring("k0", sixteen); err != nil {
		t.Fatalf("16-key inventory rejected: %v", err)
	}
	if _, err := NewPayloadKeyring("a_-09", map[string][]byte{"a_-09": good}); err != nil {
		t.Fatalf("valid key ID rejected: %v", err)
	}

	oldBytes := bytes.Repeat([]byte{0x72}, 32)
	callerKeys := map[string][]byte{"old": oldBytes}
	oldRing, err := NewPayloadKeyring("old", callerKeys)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("synthetic confidential raw bytes")
	ctx := rawPayloadContext(plaintext)
	env, err := oldRing.seal(ctx, plaintext)
	if err != nil || env.KeyID != "old" {
		t.Fatalf("old-key seal: %v", err)
	}
	for i := range oldBytes {
		oldBytes[i] = 0
	}
	delete(callerKeys, "old")
	if got, err := oldRing.open(ctx, env); err != nil || !bytes.Equal(got, plaintext) {
		t.Fatal("caller key/map mutation changed owned key material")
	}
	rotated, err := NewPayloadKeyring("active", map[string][]byte{
		"active": good,
		"old":    bytes.Repeat([]byte{0x72}, 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := rotated.open(ctx, env); err != nil || !bytes.Equal(got, plaintext) {
		t.Fatal("retained old key could not decrypt after rotation")
	}
	withoutOld, err := NewPayloadKeyring("active", map[string][]byte{"active": good})
	if err != nil {
		t.Fatal(err)
	}
	requireNoPlaintext(t, withoutOld, ctx, env)
}

func TestPayloadAESGCMRoundTripClassesAndRandomNonce(t *testing.T) {
	k := acceptanceKeyring(t)
	for _, tc := range []struct {
		name      string
		plaintext []byte
		context   func([]byte) payloadContext
	}{
		{"raw", []byte(`{"object":"page","entry":[]}`), rawPayloadContext},
		{"event", []byte(`{"message":{"mid":"synthetic-1","text":"hello"}}`), eventPayloadContext},
		{"quarantine", []byte(`{"unknown":{"reason":"synthetic"}}`), quarantinePayloadContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.context(tc.plaintext)
			first, err := k.seal(ctx, tc.plaintext)
			if err != nil {
				t.Fatalf("seal: %v", err)
			}
			second, err := k.seal(ctx, tc.plaintext)
			if err != nil {
				t.Fatalf("second seal: %v", err)
			}
			if first.KeyID != "active" || len(first.Nonce) != 12 || len(first.Ciphertext) != len(tc.plaintext)+16 {
				t.Fatal("AES-256-GCM envelope shape wrong")
			}
			if bytes.Equal(first.Nonce, second.Nonce) || bytes.Equal(first.Ciphertext, second.Ciphertext) {
				t.Fatal("same plaintext produced deterministic nonce/ciphertext")
			}
			if bytes.Contains(first.Ciphertext, tc.plaintext) {
				t.Fatal("ciphertext contains plaintext")
			}
			for _, env := range []sealedPayload{first, second} {
				got, err := k.open(ctx, env)
				if err != nil || !bytes.Equal(got, tc.plaintext) {
					t.Fatal("exact encrypted bytes did not round-trip")
				}
			}
		})
	}
}

func TestPayloadAADBindingAndTamperFailClosed(t *testing.T) {
	k := acceptanceKeyring(t)
	plaintext := []byte(`{"message":{"mid":"synthetic-1","text":"hello"}}`)
	ctx := eventPayloadContext(plaintext)
	env, err := k.seal(ctx, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*payloadContext){
		"class": func(c *payloadContext) {
			c.Class = "quarantine"
			c.TenantID, c.StoreID, c.RouteID, c.RouteEpoch = "", "", "", 0
		},
		"id":           func(c *payloadContext) { c.ID = otherPayloadID },
		"app":          func(c *payloadContext) { c.AppID = "654321" },
		"object":       func(c *payloadContext) { c.Object = "instagram" },
		"event key":    func(c *payloadContext) { c.EventKey = strings.Repeat("b", 64) },
		"payload hash": func(c *payloadContext) { c.PayloadHash = strings.Repeat("b", 64) },
		"tenant":       func(c *payloadContext) { c.TenantID = otherPayloadID },
		"store":        func(c *payloadContext) { c.StoreID = otherPayloadID },
		"route":        func(c *payloadContext) { c.RouteID = otherPayloadID },
		"route epoch":  func(c *payloadContext) { c.RouteEpoch++ },
	} {
		t.Run(name, func(t *testing.T) {
			altered := ctx
			change(&altered)
			requireNoPlaintext(t, k, altered, env)
		})
	}
	raw := []byte(`{"object":"page"}`)
	rawCtx := rawPayloadContext(raw)
	rawEnv, err := k.seal(rawCtx, raw)
	if err != nil {
		t.Fatal(err)
	}
	rawCtx.BodyHash = strings.Repeat("b", 64)
	requireNoPlaintext(t, k, rawCtx, rawEnv)
	q := []byte(`{"unknown":true}`)
	qCtx := quarantinePayloadContext(q)
	qEnv, err := k.seal(qCtx, q)
	if err != nil {
		t.Fatal(err)
	}
	qCtx.EventKey = strings.Repeat("b", 64)
	requireNoPlaintext(t, k, qCtx, qEnv)
	for _, altered := range []sealedPayload{
		{KeyID: "old", Nonce: env.Nonce, Ciphertext: env.Ciphertext},
		{KeyID: "missing", Nonce: env.Nonce, Ciphertext: env.Ciphertext},
		{KeyID: "", Nonce: env.Nonce, Ciphertext: env.Ciphertext},
		{KeyID: env.KeyID, Nonce: env.Nonce[:11], Ciphertext: env.Ciphertext},
		{KeyID: env.KeyID, Nonce: append(bytes.Clone(env.Nonce), 0), Ciphertext: env.Ciphertext},
		{KeyID: env.KeyID, Nonce: env.Nonce, Ciphertext: env.Ciphertext[:16]},
		{KeyID: env.KeyID, Nonce: env.Nonce, Ciphertext: nil},
	} {
		requireNoPlaintext(t, k, ctx, altered)
	}
	badNonce := bytes.Clone(env.Nonce)
	badNonce[0] ^= 1
	requireNoPlaintext(t, k, ctx, sealedPayload{KeyID: env.KeyID, Nonce: badNonce, Ciphertext: env.Ciphertext})
	badBody := bytes.Clone(env.Ciphertext)
	badBody[0] ^= 1
	requireNoPlaintext(t, k, ctx, sealedPayload{KeyID: env.KeyID, Nonce: env.Nonce, Ciphertext: badBody})
	badTag := bytes.Clone(env.Ciphertext)
	badTag[len(badTag)-1] ^= 1
	requireNoPlaintext(t, k, ctx, sealedPayload{KeyID: env.KeyID, Nonce: env.Nonce, Ciphertext: badTag})
	requireNoPlaintext(t, k, ctx, sealedPayload{KeyID: env.KeyID, Nonce: env.Nonce, Ciphertext: make([]byte, (4<<20)+17)})
	requireNoPlaintext(t, k, rawPayloadContext(raw), sealedPayload{KeyID: "active", Nonce: rawEnv.Nonce, Ciphertext: make([]byte, (1<<20)+17)})
}

func TestPayloadIndependentAESGCMInteropAndPostOpenHash(t *testing.T) {
	k := acceptanceKeyring(t)
	knownKey := bytes.Repeat([]byte{0x31}, 32)
	block, err := aes.NewCipher(knownKey)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	for i, tc := range []struct {
		name  string
		plain []byte
		ctx   func([]byte) payloadContext
	}{
		{"raw", []byte(`{"object":"page","entry":[]}`), rawPayloadContext},
		{"event", []byte(`{"message":{"mid":"synthetic-1"}}`), eventPayloadContext},
		{"quarantine", []byte(`{"unsupported":true}`), quarantinePayloadContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.ctx(tc.plain)
			aad := contractAAD(t, ctx, "active")
			if !bytes.HasPrefix(aad, []byte(`["livecommerce/meta-payload/v1",`)) || bytes.ContainsAny(aad, "\n\t") {
				t.Fatal("independent AAD fixture was not compact JSON")
			}
			sealed, err := k.seal(ctx, tc.plain)
			if err != nil {
				t.Fatal(err)
			}
			fromImplementation, err := aead.Open(nil, sealed.Nonce, sealed.Ciphertext, aad)
			if err != nil || !bytes.Equal(fromImplementation, tc.plain) {
				t.Fatal("stdlib AES-GCM could not open implementation ciphertext with frozen AAD")
			}
			nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, byte(i)}
			independent := sealedPayload{KeyID: "active", Nonce: nonce, Ciphertext: aead.Seal(nil, nonce, tc.plain, aad)}
			opened, err := k.open(ctx, independent)
			if err != nil || !bytes.Equal(opened, tc.plain) {
				t.Fatal("implementation could not open independently sealed AES-GCM bytes")
			}
			wrongDigest := ctx
			if ctx.Class == "raw" {
				wrongDigest.BodyHash = strings.Repeat("b", 64)
			} else {
				wrongDigest.PayloadHash = strings.Repeat("b", 64)
			}
			wrongAAD := contractAAD(t, wrongDigest, "active")
			wrongNonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 20, byte(i)}
			validTagWrongDigest := sealedPayload{KeyID: "active", Nonce: wrongNonce, Ciphertext: aead.Seal(nil, wrongNonce, tc.plain, wrongAAD)}
			requireNoPlaintext(t, k, wrongDigest, validTagWrongDigest)
		})
	}
	plain := []byte("synthetic key-id binding")
	ctx := eventPayloadContext(plain)
	oldBlock, err := aes.NewCipher(bytes.Repeat([]byte{0x72}, 32))
	if err != nil {
		t.Fatal(err)
	}
	oldAEAD, err := cipher.NewGCM(oldBlock)
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte{11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}
	wrongKeyIDAAD := sealedPayload{KeyID: "old", Nonce: nonce,
		Ciphertext: oldAEAD.Seal(nil, nonce, plain, contractAAD(t, ctx, "active"))}
	requireNoPlaintext(t, k, ctx, wrongKeyIDAAD)
}

func TestPayloadContextsHashesAndBounds(t *testing.T) {
	k := acceptanceKeyring(t)
	plain := []byte("synthetic private bytes")
	for _, tc := range []struct {
		name string
		ctx  payloadContext
	}{
		{"wrong class", func() payloadContext { c := rawPayloadContext(plain); c.Class = "Raw"; return c }()},
		{"bad UUID", func() payloadContext {
			c := rawPayloadContext(plain)
			c.ID = "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"
			return c
		}()},
		{"bad app", func() payloadContext { c := rawPayloadContext(plain); c.AppID = "１２３"; return c }()},
		{"bad object", func() payloadContext { c := rawPayloadContext(plain); c.Object = "Page"; return c }()},
		{"bad body hash", func() payloadContext {
			c := rawPayloadContext(plain)
			c.BodyHash = strings.ToUpper(c.BodyHash)
			return c
		}()},
		{"raw with event", func() payloadContext { c := rawPayloadContext(plain); c.EventKey = strings.Repeat("a", 64); return c }()},
		{"raw with scope", func() payloadContext { c := rawPayloadContext(plain); c.TenantID = payloadTenantID; return c }()},
		{"raw wrong plaintext hash", func() payloadContext { c := rawPayloadContext(plain); c.BodyHash = strings.Repeat("b", 64); return c }()},
		{"event without scope", func() payloadContext { c := eventPayloadContext(plain); c.TenantID = ""; return c }()},
		{"event without route", func() payloadContext { c := eventPayloadContext(plain); c.RouteID = ""; return c }()},
		{"event zero epoch", func() payloadContext { c := eventPayloadContext(plain); c.RouteEpoch = 0; return c }()},
		{"event with body hash", func() payloadContext { c := eventPayloadContext(plain); c.BodyHash = payloadDigest(plain); return c }()},
		{"event wrong plaintext hash", func() payloadContext {
			c := eventPayloadContext(plain)
			c.PayloadHash = strings.Repeat("b", 64)
			return c
		}()},
		{"quarantine with scope", func() payloadContext { c := quarantinePayloadContext(plain); c.StoreID = payloadStoreID; return c }()},
		{"quarantine without key", func() payloadContext { c := quarantinePayloadContext(plain); c.EventKey = ""; return c }()},
		{"quarantine wrong plaintext hash", func() payloadContext {
			c := quarantinePayloadContext(plain)
			c.PayloadHash = strings.Repeat("b", 64)
			return c
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := k.seal(tc.ctx, plain); err == nil {
				t.Fatal("invalid context/hash accepted")
			}
		})
	}
	if _, err := k.seal(rawPayloadContext(nil), nil); err == nil {
		t.Fatal("empty raw plaintext accepted")
	}
	if _, err := k.seal(eventPayloadContext(nil), nil); err == nil {
		t.Fatal("empty event plaintext accepted")
	}
	for _, tc := range []struct {
		name  string
		bytes []byte
		ctx   func([]byte) payloadContext
		valid bool
	}{
		{"raw exactly 1 MiB", bytes.Repeat([]byte{'r'}, 1<<20), rawPayloadContext, true},
		{"raw over 1 MiB", bytes.Repeat([]byte{'r'}, 1<<20+1), rawPayloadContext, false},
		{"event exactly 4 MiB", bytes.Repeat([]byte{'e'}, 4<<20), eventPayloadContext, true},
		{"event over 4 MiB", bytes.Repeat([]byte{'e'}, 4<<20+1), eventPayloadContext, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, err := k.seal(tc.ctx(tc.bytes), tc.bytes)
			if tc.valid {
				if err != nil {
					t.Fatalf("boundary payload rejected: %v", err)
				}
				got, err := k.open(tc.ctx(tc.bytes), env)
				if err != nil || !bytes.Equal(got, tc.bytes) {
					t.Fatal("boundary payload did not round-trip")
				}
			} else if err == nil {
				t.Fatal("oversize plaintext admitted")
			}
		})
	}
}

func TestPayloadFormattingAndJSONRedaction(t *testing.T) {
	const marker = "synthetic-secret-key-id"
	keyBytes := bytes.Repeat([]byte{0x5a}, 32)
	k, err := NewPayloadKeyring(marker, map[string][]byte{marker: keyBytes})
	if err != nil {
		t.Fatal(err)
	}
	env := sealedPayload{KeyID: marker, Nonce: []byte("nonce-marker"), Ciphertext: []byte("ciphertext-marker")}
	for _, value := range []any{k, &k, env, &env} {
		for _, formatted := range []string{fmt.Sprintf("%v", value), fmt.Sprintf("%+v", value), fmt.Sprintf("%#v", value)} {
			for _, forbidden := range []string{
				marker, "ciphertext-marker", "nonce-marker",
				fmt.Sprintf("%v", keyBytes), fmt.Sprintf("%#v", keyBytes),
				fmt.Sprintf("%v", env.Nonce), fmt.Sprintf("%#v", env.Nonce),
				fmt.Sprintf("%v", env.Ciphertext), fmt.Sprintf("%#v", env.Ciphertext),
			} {
				if strings.Contains(formatted, forbidden) {
					t.Fatalf("format leaked synthetic marker %q", forbidden)
				}
			}
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("redacted JSON marshal: %v", err)
		}
		for _, forbidden := range []string{marker, base64.StdEncoding.EncodeToString(keyBytes), base64.StdEncoding.EncodeToString(env.Nonce), base64.StdEncoding.EncodeToString(env.Ciphertext)} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("JSON leaked synthetic marker %q", forbidden)
			}
		}
	}
}
