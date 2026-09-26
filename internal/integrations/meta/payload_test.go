package meta

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const payloadTestID = "11111111-2222-3333-4444-555555555555"

func payloadTestKey(n byte) []byte { return bytes.Repeat([]byte{n}, 32) }

func payloadTestContext(class string, body []byte) payloadContext {
	c := payloadContext{Class: class, ID: payloadTestID, AppID: "123456", Object: "page"}
	if class == "raw" {
		c.BodyHash = digest(body)
	} else {
		c.EventKey, c.PayloadHash = digest([]byte("event identity")), digest(body)
		if class == "event" {
			c.TenantID, c.StoreID, c.RouteID, c.RouteEpoch = payloadTestID, payloadTestID, payloadTestID, 1
		}
	}
	return c
}

func TestPayloadKeyringConfigCopiesAndRotation(t *testing.T) {
	base := payloadTestKey(1)
	old := payloadTestKey(2)
	keys := map[string][]byte{"active": base, "old": old}
	k, err := NewPayloadKeyring("active", keys)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("provider content")
	c := payloadTestContext("event", body)
	first, err := k.seal(c, body)
	if err != nil {
		t.Fatal(err)
	}
	copy(base, payloadTestKey(3))
	delete(keys, "active")
	opened, err := k.open(c, first)
	if err != nil || !bytes.Equal(opened, body) {
		t.Fatalf("keyring did not own key copy: %v", err)
	}
	k.activeID = "old"
	second, err := k.seal(c, body)
	if err != nil || second.KeyID != "old" {
		t.Fatalf("rotation failed: %v", err)
	}
	if _, err := k.open(c, first); err != nil {
		t.Fatalf("old key unavailable: %v", err)
	}
	for _, tc := range []struct {
		name, active string
		keys         map[string][]byte
	}{
		{"empty", "", nil},
		{"missing active", "other", map[string][]byte{"id": payloadTestKey(1)}},
		{"invalid id", "bad/id", map[string][]byte{"bad/id": payloadTestKey(1)}},
		{"short key", "id", map[string][]byte{"id": payloadTestKey(1)[:31]}},
		{"zero key", "id", map[string][]byte{"id": make([]byte, 32)}},
		{"duplicate key", "a", map[string][]byte{"a": payloadTestKey(1), "b": payloadTestKey(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewPayloadKeyring(tc.active, tc.keys); !errors.Is(err, ErrConfig) {
				t.Fatalf("wanted safe configuration error, got %v", err)
			}
		})
	}
	tooMany := make(map[string][]byte)
	for i := 1; i <= 17; i++ {
		tooMany[fmt.Sprintf("k%d", i)] = payloadTestKey(byte(i))
	}
	if _, err := NewPayloadKeyring("k1", tooMany); err == nil {
		t.Fatal("accepted more than 16 keys")
	}
}

func TestPayloadSealingAuthenticationAndBounds(t *testing.T) {
	k, _ := NewPayloadKeyring("key", map[string][]byte{"key": payloadTestKey(7)})
	for _, class := range []string{"raw", "event", "quarantine"} {
		t.Run(class, func(t *testing.T) {
			body := []byte("secret provider content")
			c := payloadTestContext(class, body)
			a, err := k.seal(c, body)
			if err != nil {
				t.Fatal(err)
			}
			b, err := k.seal(c, body)
			if err != nil || bytes.Equal(a.Nonce, b.Nonce) || bytes.Equal(a.Ciphertext, b.Ciphertext) {
				t.Fatal("seals were not independent")
			}
			got, err := k.open(c, a)
			if err != nil || !bytes.Equal(got, body) {
				t.Fatalf("round trip failed: %v", err)
			}
			for _, changed := range []payloadContext{
				func() payloadContext { x := c; x.ID = "aaaaaaaa-2222-3333-4444-555555555555"; return x }(),
				func() payloadContext { x := c; x.AppID = "654321"; return x }(),
				func() payloadContext { x := c; x.Object = "instagram"; return x }(),
			} {
				if out, err := k.open(changed, a); !errors.Is(err, ErrPayload) || out != nil {
					t.Fatal("changed AAD authenticated")
				}
			}
			if class == "event" {
				changed := c
				changed.RouteEpoch++
				if _, err := k.open(changed, a); !errors.Is(err, ErrPayload) {
					t.Fatal("route epoch not authenticated")
				}
			}
			a.Ciphertext[0] ^= 1
			if out, err := k.open(c, a); !errors.Is(err, ErrPayload) || out != nil {
				t.Fatal("tampered ciphertext returned plaintext")
			}
		})
	}
	for _, tc := range []struct {
		class string
		size  int
		pass  bool
	}{
		{"raw", 1, true}, {"raw", maxBody, true}, {"raw", maxBody + 1, false},
		{"event", maxPayload, true}, {"event", maxPayload + 1, false},
		{"quarantine", maxPayload, true},
	} {
		body := bytes.Repeat([]byte("x"), tc.size)
		c := payloadTestContext(tc.class, body)
		sealed, err := k.seal(c, body)
		if (err == nil) != tc.pass {
			t.Fatalf("%s size %d: %v", tc.class, tc.size, err)
		}
		if tc.pass {
			if _, err := k.open(c, sealed); err != nil {
				t.Fatalf("open %s size %d: %v", tc.class, tc.size, err)
			}
		}
	}
	if _, err := k.seal(payloadTestContext("raw", nil), nil); !errors.Is(err, ErrPayload) {
		t.Fatal("empty plaintext accepted")
	}
}

func TestPayloadContextAndEnvelopeRejectMalformedInput(t *testing.T) {
	k, _ := NewPayloadKeyring("key", map[string][]byte{"key": payloadTestKey(7)})
	body := []byte("secret")
	c := payloadTestContext("event", body)
	sealed, _ := k.seal(c, body)
	mutations := []func(*payloadContext){
		func(x *payloadContext) { x.Class = "unknown" },
		func(x *payloadContext) { x.ID = "AAAAAAAA-2222-3333-4444-555555555555" },
		func(x *payloadContext) { x.AppID = "1a" },
		func(x *payloadContext) { x.EventKey = strings.ToUpper(x.EventKey) },
		func(x *payloadContext) { x.PayloadHash = strings.ToUpper(x.PayloadHash) },
		func(x *payloadContext) { x.BodyHash = digest(body) },
		func(x *payloadContext) { x.TenantID = "" },
		func(x *payloadContext) { x.StoreID = "" },
		func(x *payloadContext) { x.RouteID = "" },
		func(x *payloadContext) { x.RouteEpoch = 0 },
	}
	for i, mutate := range mutations {
		bad := c
		mutate(&bad)
		if _, err := k.seal(bad, body); !errors.Is(err, ErrPayload) {
			t.Fatalf("accepted malformed context %d", i)
		}
	}
	for i, bad := range []sealedPayload{
		{KeyID: "unknown", Nonce: sealed.Nonce, Ciphertext: sealed.Ciphertext},
		{KeyID: "key", Nonce: sealed.Nonce[:11], Ciphertext: sealed.Ciphertext},
		{KeyID: "key", Nonce: sealed.Nonce, Ciphertext: sealed.Ciphertext[:16]},
		{KeyID: "key", Nonce: sealed.Nonce, Ciphertext: make([]byte, maxPayload+payloadTag+1)},
	} {
		if out, err := k.open(c, bad); !errors.Is(err, ErrPayload) || out != nil {
			t.Fatalf("accepted malformed envelope %d", i)
		}
	}
	wrongHash := c
	wrongHash.PayloadHash = digest([]byte("other"))
	if _, err := k.seal(wrongHash, body); !errors.Is(err, ErrPayload) {
		t.Fatal("sealed mismatched digest")
	}
	if out, err := k.open(wrongHash, sealed); !errors.Is(err, ErrPayload) || out != nil {
		t.Fatal("opened mismatched digest")
	}
}

func TestPayloadSecretsAreRedacted(t *testing.T) {
	k, _ := NewPayloadKeyring("secret-key-id", map[string][]byte{"secret-key-id": payloadTestKey(7)})
	c := payloadTestContext("event", []byte("secret content"))
	s := sealedPayload{KeyID: "secret-key-id", Nonce: []byte("secret nonce"), Ciphertext: []byte("secret ciphertext")}
	for _, value := range []any{k, c, s} {
		for _, rendered := range []string{fmt.Sprint(value), fmt.Sprintf("%+v", value), fmt.Sprintf("%#v", value)} {
			if strings.Contains(rendered, "secret") || strings.Contains(rendered, payloadTestID) || strings.Contains(rendered, "123456") {
				t.Fatalf("format leaked a payload field")
			}
		}
		b, err := json.Marshal(value)
		if err != nil || strings.Contains(string(b), "secret") || strings.Contains(string(b), payloadTestID) {
			t.Fatal("JSON leaked a payload field")
		}
	}
}

func TestPayloadAADFrozenWire(t *testing.T) {
	c := payloadContext{
		Class: "event", ID: "id", AppID: "12", Object: "page",
		BodyHash: "", EventKey: "event-key", PayloadHash: "payload-hash",
		TenantID: "tenant", StoreID: "store", RouteID: "route", RouteEpoch: 7,
	}
	want := `["livecommerce/meta-payload/v1","event","id","12","page","","event-key","payload-hash","tenant","store","route",7,"key-id"]`
	if got := string(c.aad("key-id")); got != want {
		t.Fatalf("AAD wire changed: %q", got)
	}
}
