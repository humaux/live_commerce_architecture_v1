package livekit_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	livekit "livecommerce/internal/integrations/livekit"
)

const materialKeyID = "material_key_1"

var materialKey = []byte("0123456789abcdef0123456789abcdef")

func materialScope() livekit.MaterialScope {
	return livekit.MaterialScope{
		TenantID: "11111111-1111-1111-1111-111111111111", StoreID: "22222222-2222-2222-2222-222222222222",
		SessionID: "33333333-3333-3333-3333-333333333333", AttemptID: "01234567-89ab-cdef-0123-456789abcdef",
		ProjectID: "project_1", CredentialVersion: 3, MaterialVersion: 5,
	}
}

func materialRing(t *testing.T, active string, keys map[string][]byte) *livekit.MaterialKeyring {
	t.Helper()
	k, err := livekit.NewMaterialKeyring(active, keys)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

type materialField struct {
	name  string
	value any
}

// Independent ordered JSON builder: it never calls production AAD or wire helpers.
func materialObject(t *testing.T, fields ...materialField) []byte {
	t.Helper()
	var out bytes.Buffer
	out.WriteByte('{')
	for i, field := range fields {
		if i != 0 {
			out.WriteByte(',')
		}
		name, err := json.Marshal(field.name)
		if err != nil {
			t.Fatal(err)
		}
		value, err := json.Marshal(field.value)
		if err != nil {
			t.Fatal(err)
		}
		out.Write(name)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes()
}

func materialAAD(t *testing.T, scope livekit.MaterialScope, cfg livekit.Config, keyID string) []byte {
	t.Helper()
	return materialObject(t,
		materialField{"domain", "livecommerce.livekit.media"}, materialField{"format", 1}, materialField{"key_id", keyID},
		materialField{"tenant_id", scope.TenantID}, materialField{"store_id", scope.StoreID},
		materialField{"session_id", scope.SessionID}, materialField{"attempt_id", scope.AttemptID},
		materialField{"project_id", scope.ProjectID}, materialField{"credential_version", scope.CredentialVersion},
		materialField{"material_version", scope.MaterialVersion}, materialField{"environment", cfg.Environment},
		materialField{"endpoint", strings.TrimSuffix(cfg.Endpoint, "/")},
	)
}

func materialPlain(t *testing.T, in livekit.StartInput) []byte {
	t.Helper()
	return materialObject(t, materialField{"room_name", in.RoomName}, materialField{"aspect_ratio", in.AspectRatio}, materialField{"stream_urls", in.StreamURLs})
}

func materialGCM(t *testing.T, key []byte) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return gcm
}

func materialExternalEnvelope(t *testing.T, scope livekit.MaterialScope, cfg livekit.Config, keyID string, key, plain []byte) livekit.SealedMaterial {
	t.Helper()
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	ciphertext := materialGCM(t, key).Seal(nil, nonce, plain, materialAAD(t, scope, cfg, keyID))
	return livekit.SealedMaterial{KeyID: keyID, Nonce: nonce, Ciphertext: ciphertext}
}

func materialNoIOClient(t *testing.T) (*livekit.Client, func() int32) {
	t.Helper()
	c, _, calls := fixtureClient(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("material operation performed network I/O")
		http.Error(w, "unexpected material I/O", http.StatusInternalServerError)
	})
	return c, calls.Load
}

func materialFailure(t *testing.T, got livekit.StartInput, err error) {
	t.Helper()
	requireErr(t, err, livekit.ErrMaterial)
	if !reflect.DeepEqual(got, livekit.StartInput{}) {
		t.Fatalf("material failure returned partial StartInput: %+v", got)
	}
}

func materialSealFailure(t *testing.T, got livekit.SealedMaterial, err error) {
	t.Helper()
	requireErr(t, err, livekit.ErrMaterial)
	if got.KeyID != "" || got.Nonce != nil || got.Ciphertext != nil {
		t.Fatal("material failure returned partial envelope")
	}
}

func TestLKM01IndependentAADAndAESGCMInteroperability(t *testing.T) {
	c, calls := materialNoIOClient(t)
	k := materialRing(t, materialKeyID, map[string][]byte{materialKeyID: materialKey})
	scope := materialScope()
	for _, aspect := range []string{"16:9", "9:16"} {
		in := input()
		in.AspectRatio = aspect
		first, err := k.Seal(scope, c, in)
		if err != nil || first.KeyID != materialKeyID || len(first.Nonce) != 12 || len(first.Ciphertext) < 16 || len(first.Ciphertext) > 16400 {
			t.Fatalf("Seal envelope: %+v %v", first, err)
		}
		second, err := k.Seal(scope, c, in)
		if err != nil || bytes.Equal(first.Nonce, second.Nonce) || bytes.Equal(first.Ciphertext, second.Ciphertext) {
			t.Fatalf("random nonce/ciphertext reuse: %v", err)
		}
		plain, err := materialGCM(t, materialKey).Open(nil, first.Nonce, first.Ciphertext, materialAAD(t, scope, config(), first.KeyID))
		if err != nil {
			t.Fatalf("independent AES-GCM/AAD open failed: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(plain, &decoded); err != nil || !reflect.DeepEqual(decoded, map[string]any{
			"room_name": in.RoomName, "aspect_ratio": in.AspectRatio, "stream_urls": []any{streamOne, streamTwo},
		}) {
			t.Fatalf("private plaintext shape mismatch: %v", err)
		}
		got, err := k.Open(scope, c, first)
		if err != nil || !reflect.DeepEqual(got, in) {
			t.Fatalf("package round trip: %+v %v", got, err)
		}
		external := materialExternalEnvelope(t, scope, config(), materialKeyID, materialKey, materialPlain(t, in))
		got, err = k.Open(scope, c, external)
		if err != nil || !reflect.DeepEqual(got, in) {
			t.Fatalf("independently sealed envelope rejected: %+v %v", got, err)
		}
	}
	if calls() != 0 {
		t.Fatalf("material round trip caused %d provider calls", calls())
	}
}

func TestLKM02EveryScopeAndContextBinding(t *testing.T) {
	c, calls := materialNoIOClient(t)
	k := materialRing(t, materialKeyID, map[string][]byte{materialKeyID: materialKey, "same_bytes_alias": materialKey})
	scope := materialScope()
	sealed, err := k.Seal(scope, c, input())
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*livekit.MaterialScope){
		"tenant":             func(s *livekit.MaterialScope) { s.TenantID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" },
		"store":              func(s *livekit.MaterialScope) { s.StoreID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" },
		"session":            func(s *livekit.MaterialScope) { s.SessionID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" },
		"attempt":            func(s *livekit.MaterialScope) { s.AttemptID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" },
		"project":            func(s *livekit.MaterialScope) { s.ProjectID = "other_project" },
		"credential version": func(s *livekit.MaterialScope) { s.CredentialVersion++ },
		"material version":   func(s *livekit.MaterialScope) { s.MaterialVersion++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := scope
			mutate(&changed)
			got, err := k.Open(changed, c, sealed)
			materialFailure(t, got, err)
		})
	}
	changed := sealed
	changed.KeyID = "same_bytes_alias"
	got, err := k.Open(scope, c, changed)
	materialFailure(t, got, err)
	changed = sealed
	changed.Nonce = append([]byte(nil), sealed.Nonce...)
	changed.Nonce[0] ^= 1
	got, err = k.Open(scope, c, changed)
	materialFailure(t, got, err)
	changed = sealed
	changed.Ciphertext = append([]byte(nil), sealed.Ciphertext...)
	changed.Ciphertext[0] ^= 1
	got, err = k.Open(scope, c, changed)
	materialFailure(t, got, err)
	changed = sealed
	changed.Ciphertext = append([]byte(nil), sealed.Ciphertext...)
	changed.Ciphertext[len(changed.Ciphertext)-1] ^= 1
	got, err = k.Open(scope, c, changed)
	materialFailure(t, got, err)
	changed = sealed
	changed.KeyID = "retired"
	got, err = k.Open(scope, c, changed)
	materialFailure(t, got, err)
	wrongKey := materialRing(t, materialKeyID, map[string][]byte{materialKeyID: bytes.Repeat([]byte{'X'}, 32)})
	got, err = wrongKey.Open(scope, c, sealed)
	materialFailure(t, got, err)

	liveCfg := config()
	liveCfg.Environment = "LIVE"
	liveClient, err := livekit.New(liveCfg)
	if err != nil {
		t.Fatal(err)
	}
	got, err = k.Open(scope, liveClient, sealed)
	materialFailure(t, got, err)
	otherCfg := config()
	otherCfg.Endpoint = "https://other.livekit.cloud"
	otherClient, err := livekit.New(otherCfg, http.RoundTripper(funcTransport(func(*http.Request) (*http.Response, error) {
		t.Error("material operation performed network I/O")
		return nil, fmt.Errorf("unexpected material I/O")
	})))
	if err != nil {
		t.Fatal(err)
	}
	got, err = k.Open(scope, otherClient, sealed)
	materialFailure(t, got, err)
	slashCfg := config()
	slashCfg.Endpoint += "/"
	slashClient, err := livekit.New(slashCfg, http.RoundTripper(funcTransport(func(*http.Request) (*http.Response, error) {
		t.Error("material operation performed network I/O")
		return nil, fmt.Errorf("unexpected material I/O")
	})))
	if err != nil {
		t.Fatal(err)
	}
	got, err = k.Open(scope, slashClient, sealed)
	if err != nil || !reflect.DeepEqual(got, input()) {
		t.Fatalf("canonical trailing slash changed AAD: %+v %v", got, err)
	}
	if calls() != 0 {
		t.Fatalf("binding tests caused %d provider calls", calls())
	}
}

type funcTransport func(*http.Request) (*http.Response, error)

func (f funcTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLKM02RotationAndRetirement(t *testing.T) {
	c, calls := materialNoIOClient(t)
	scope := materialScope()
	old := materialRing(t, "old", map[string][]byte{"old": materialKey})
	oldEnvelope, err := old.Seal(scope, c, input())
	if err != nil {
		t.Fatal(err)
	}
	newKey := bytes.Repeat([]byte{'N'}, 32)
	rotated := materialRing(t, "new", map[string][]byte{"old": materialKey, "new": newKey})
	if got, err := rotated.Open(scope, c, oldEnvelope); err != nil || !reflect.DeepEqual(got, input()) {
		t.Fatalf("retained old key could not open: %+v %v", got, err)
	}
	newEnvelope, err := rotated.Seal(scope, c, input())
	if err != nil || newEnvelope.KeyID != "new" {
		t.Fatalf("rotation did not select active key: %+v %v", newEnvelope, err)
	}
	if got, err := old.Open(scope, c, newEnvelope); err != livekit.ErrMaterial || !reflect.DeepEqual(got, livekit.StartInput{}) {
		t.Fatalf("old ring opened new key: %+v %v", got, err)
	}
	retired := materialRing(t, "new", map[string][]byte{"new": newKey})
	got, err := retired.Open(scope, c, oldEnvelope)
	materialFailure(t, got, err)
	if calls() != 0 {
		t.Fatalf("rotation caused %d provider calls", calls())
	}
}

func TestLKM03AuthenticatedMalformedPlaintextAndEnvelopeBounds(t *testing.T) {
	c, calls := materialNoIOClient(t)
	k := materialRing(t, materialKeyID, map[string][]byte{materialKeyID: materialKey})
	scope := materialScope()
	valid := materialPlain(t, input())
	for name, plain := range map[string][]byte{
		"malformed":      []byte(`{"room_name":`),
		"duplicate":      bytes.Replace(valid, []byte(`"room_name":`), []byte(`"room_name":"`+room+`","room_name":`), 1),
		"unknown":        append(bytes.TrimSuffix(append([]byte(nil), valid...), []byte("}")), []byte(`,"future":1}`)...),
		"missing":        materialObject(t, materialField{"room_name", room}, materialField{"aspect_ratio", "16:9"}),
		"wrong type":     bytes.Replace(valid, []byte(`"stream_urls":["`+streamOne+`","`+streamTwo+`"]`), []byte(`"stream_urls":"bad"`), 1),
		"alias":          bytes.Replace(valid, []byte(`"room_name":`), []byte(`"roomName":`), 1),
		"trailing":       append(append([]byte(nil), valid...), []byte(`{}`)...),
		"invalid UTF8":   bytes.Replace(valid, []byte(room), []byte{'l', 'c', '_', 0xff}, 1),
		"wrong room":     bytes.Replace(valid, []byte(room), []byte("lc_ffffffffffffffffffffffffffffffff"), 1),
		"wrong aspect":   bytes.Replace(valid, []byte(`"16:9"`), []byte(`"4:3"`), 1),
		"untrusted host": bytes.Replace(valid, []byte("ingest.example.com"), []byte("evil.example.com"), 1),
		"duplicate URL":  materialPlain(t, livekit.StartInput{RoomName: room, AspectRatio: "16:9", StreamURLs: []string{streamOne, streamOne}}),
	} {
		t.Run(name, func(t *testing.T) {
			sealed := materialExternalEnvelope(t, scope, config(), materialKeyID, materialKey, plain)
			got, err := k.Open(scope, c, sealed)
			materialFailure(t, got, err)
		})
	}
	largePlain := bytes.Repeat([]byte{'x'}, 16385)
	large := materialExternalEnvelope(t, scope, config(), materialKeyID, materialKey, largePlain)
	if len(large.Ciphertext) != 16401 {
		t.Fatal("oversize fixture did not cross exact ciphertext cap")
	}
	got, err := k.Open(scope, c, large)
	materialFailure(t, got, err)
	for _, changed := range []livekit.SealedMaterial{
		{KeyID: materialKeyID, Nonce: []byte{1}, Ciphertext: bytes.Repeat([]byte{1}, 16)},
		{KeyID: materialKeyID, Nonce: bytes.Repeat([]byte{1}, 12), Ciphertext: bytes.Repeat([]byte{1}, 15)},
		{KeyID: materialKeyID, Nonce: bytes.Repeat([]byte{1}, 12), Ciphertext: bytes.Repeat([]byte{1}, 16401)},
	} {
		got, err := k.Open(scope, c, changed)
		materialFailure(t, got, err)
	}
	if calls() != 0 {
		t.Fatalf("malformed material caused %d provider calls", calls())
	}
}

func TestLKM03InvalidScopeAndInputBeforeSeal(t *testing.T) {
	c, calls := materialNoIOClient(t)
	k := materialRing(t, materialKeyID, map[string][]byte{materialKeyID: materialKey})
	for name, mutate := range map[string]func(*livekit.MaterialScope){
		"zero tenant":        func(s *livekit.MaterialScope) { s.TenantID = "00000000-0000-0000-0000-000000000000" },
		"upper store":        func(s *livekit.MaterialScope) { s.StoreID = "AAAAAAAA-aaaa-aaaa-aaaa-aaaaaaaaaaaa" },
		"bad session":        func(s *livekit.MaterialScope) { s.SessionID = "bad" },
		"bad attempt":        func(s *livekit.MaterialScope) { s.AttemptID = "bad" },
		"empty project":      func(s *livekit.MaterialScope) { s.ProjectID = "" },
		"project length":     func(s *livekit.MaterialScope) { s.ProjectID = strings.Repeat("a", 81) },
		"credential version": func(s *livekit.MaterialScope) { s.CredentialVersion = 0 },
		"material version":   func(s *livekit.MaterialScope) { s.MaterialVersion = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			s := materialScope()
			mutate(&s)
			sealed, err := k.Seal(s, c, input())
			materialSealFailure(t, sealed, err)
		})
	}
	for name, mutate := range map[string]func(*livekit.StartInput){
		"room mismatch":  func(in *livekit.StartInput) { in.RoomName = "lc_ffffffffffffffffffffffffffffffff" },
		"aspect":         func(in *livekit.StartInput) { in.AspectRatio = "4:3" },
		"zero URLs":      func(in *livekit.StartInput) { in.StreamURLs = nil },
		"duplicate URLs": func(in *livekit.StartInput) { in.StreamURLs = []string{streamOne, streamOne} },
		"host":           func(in *livekit.StartInput) { in.StreamURLs = []string{"rtmps://evil.example.com/live/key"} },
		"scheme":         func(in *livekit.StartInput) { in.StreamURLs = []string{"rtmp://ingest.example.com/live/key"} },
	} {
		t.Run(name, func(t *testing.T) {
			in := input()
			mutate(&in)
			sealed, err := k.Seal(materialScope(), c, in)
			materialSealFailure(t, sealed, err)
		})
	}
	if calls() != 0 {
		t.Fatalf("invalid material caused %d provider calls", calls())
	}
}

func TestLKM04KeyringValidationOwnershipRedactionAndParallelUse(t *testing.T) {
	for name, tc := range map[string]struct {
		active string
		keys   map[string][]byte
	}{
		"nil map":        {active: materialKeyID},
		"missing active": {active: "missing", keys: map[string][]byte{materialKeyID: materialKey}},
		"invalid ID":     {active: "bad key", keys: map[string][]byte{"bad key": materialKey}},
		"short key":      {active: materialKeyID, keys: map[string][]byte{materialKeyID: []byte("short")}},
		"too many keys": {active: materialKeyID, keys: func() map[string][]byte {
			m := map[string][]byte{}
			for i := 0; i < 17; i++ {
				m[fmt.Sprintf("id%d", i)] = materialKey
			}
			m[materialKeyID] = materialKey
			return m
		}()},
	} {
		t.Run(name, func(t *testing.T) {
			k, err := livekit.NewMaterialKeyring(tc.active, tc.keys)
			if k != nil {
				t.Fatal("invalid keyring returned nonnil")
			}
			requireErr(t, err, livekit.ErrMaterial)
		})
	}
	c, calls := materialNoIOClient(t)
	scope := materialScope()
	var nilRing *livekit.MaterialKeyring
	var zeroRing livekit.MaterialKeyring
	var zeroClient livekit.Client
	for _, k := range []*livekit.MaterialKeyring{nilRing, &zeroRing} {
		sealed, err := k.Seal(scope, c, input())
		materialSealFailure(t, sealed, err)
		got, err := k.Open(scope, c, livekit.SealedMaterial{})
		materialFailure(t, got, err)
	}
	k := materialRing(t, materialKeyID, map[string][]byte{materialKeyID: materialKey})
	for _, client := range []*livekit.Client{nil, &zeroClient} {
		sealed, err := k.Seal(scope, client, input())
		materialSealFailure(t, sealed, err)
		got, err := k.Open(scope, client, livekit.SealedMaterial{})
		materialFailure(t, got, err)
	}
	keyCopy := append([]byte(nil), materialKey...)
	keys := map[string][]byte{materialKeyID: keyCopy}
	owned := materialRing(t, materialKeyID, keys)
	sealed, err := owned.Seal(scope, c, input())
	if err != nil {
		t.Fatal(err)
	}
	for i := range keyCopy {
		keyCopy[i] = 0
	}
	delete(keys, materialKeyID)
	got, err := owned.Open(scope, c, sealed)
	if err != nil || !reflect.DeepEqual(got, input()) {
		t.Fatalf("constructor aliased caller key/map: %+v %v", got, err)
	}
	original := input()
	sealed, err = k.Seal(scope, c, original)
	if err != nil {
		t.Fatal(err)
	}
	original.StreamURLs[0] = "rtmps://ingest.example.com/live/changed"
	got, err = k.Open(scope, c, sealed)
	if err != nil || got.StreamURLs[0] != streamOne {
		t.Fatalf("input slice aliased: %+v %v", got, err)
	}
	got.StreamURLs[0] = "mutated"
	again, err := k.Open(scope, c, sealed)
	if err != nil || again.StreamURLs[0] != streamOne {
		t.Fatalf("returned slice aliased: %+v %v", again, err)
	}
	for _, value := range []any{k, sealed} {
		for _, display := range []string{fmt.Sprint(value), fmt.Sprintf("%#v", value)} {
			for _, secret := range []string{string(materialKey), materialKeyID, streamOne} {
				if strings.Contains(display, secret) {
					t.Fatal("secret-bearing material display leaked")
				}
			}
		}
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{string(materialKey), materialKeyID, streamOne} {
			if bytes.Contains(b, []byte(secret)) {
				t.Fatal("secret-bearing material JSON leaked")
			}
		}
	}
	var wg sync.WaitGroup
	errors := make(chan error, 32)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			envelope, err := k.Seal(scope, c, input())
			if err == nil {
				var back livekit.StartInput
				back, err = k.Open(scope, c, envelope)
				if err == nil && !reflect.DeepEqual(back, input()) {
					err = fmt.Errorf("parallel round trip mismatch")
				}
			}
			if err != nil {
				errors <- err
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Errorf("parallel material operation: %v", err)
	}
	if calls() != 0 {
		t.Fatalf("material operations caused %d provider calls", calls())
	}
}

// The contract's seal/open authority never implies permission to start media.
func TestLKM05NoProviderCallForMaterialOnly(t *testing.T) {
	c, calls := materialNoIOClient(t)
	k := materialRing(t, materialKeyID, map[string][]byte{materialKeyID: materialKey})
	scope := materialScope()
	sealed, err := k.Seal(scope, c, input())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.Open(scope, c, sealed); err != nil {
		t.Fatal(err)
	}
	if calls() != 0 {
		t.Fatalf("material custody dispatched %d provider calls", calls())
	}
}
