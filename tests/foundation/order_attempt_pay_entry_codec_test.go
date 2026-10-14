// Purpose: independent counterexamples for the order-only pay-entry credential codec.
// Depends: the codec's exported API and standard-library AES-GCM/JSON, never its AAD helpers.
// Used by: OA1 CI before SQL storage, redemption or process wiring is enabled.
// Invariants: dedicated key custody, exact binding/digest, canonical random 256-bit tokens,
// retained-key replay, and redacted failures. Lifetime and validity belong to SQL.
package foundation_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"livecommerce/internal/orderattempt"
)

type PayEntryBinding = orderattempt.PayEntryBinding
type PayEntryMaterial = orderattempt.PayEntryMaterial
type PayEntryKeys = orderattempt.PayEntryKeys

func payEntryTestBinding() PayEntryBinding {
	return PayEntryBinding{
		TenantID: "aaaaaaaa-1111-4111-8111-111111111111",
		StoreID:  "bbbbbbbb-2222-4222-8222-222222222222",
		OrderID:  "cccccccc-3333-4333-8333-333333333333",
		Version:  1,
	}
}

func payEntryTestKeys(t *testing.T) *PayEntryKeys {
	t.Helper()
	k, err := orderattempt.NewPayEntryKeys("v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x41}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// The wire oracle specifies ordered compact JSON independently of implementation helpers.
func payEntryOracleAAD(t *testing.T, b PayEntryBinding, version string, digest [32]byte) []byte {
	t.Helper()
	raw := fmt.Sprintf(`{"domain":"order-pay-entry-v1","key_version":%q,"binding":{"tenant_id":%q,"store_id":%q,"order_id":%q,"version":%d},"digest":`, version, b.TenantID, b.StoreID, b.OrderID, b.Version)
	d, err := json.Marshal(digest)
	if err != nil {
		t.Fatal(err)
	}
	return append(append([]byte(raw), d...), '}')
}

func payEntryOracleAEAD(t *testing.T) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(bytes.Repeat([]byte{0x41}, 32))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return aead
}

func payEntryOracleMaterial(t *testing.T, b PayEntryBinding, plain string, digest [32]byte) PayEntryMaterial {
	t.Helper()
	aead := payEntryOracleAEAD(t)
	nonce := bytes.Repeat([]byte{0x19}, aead.NonceSize())
	ciphertext := aead.Seal(bytes.Clone(nonce), nonce, []byte(plain), payEntryOracleAAD(t, b, "v1", digest))
	return PayEntryMaterial{KeyVersion: "v1", Digest: digest, Ciphertext: ciphertext}
}

func payEntryRequireError(t *testing.T, err error) {
	t.Helper()
	var coded interface {
		error
		Code() string
	}
	if err == nil || !errors.As(err, &coded) {
		t.Fatal("expected a fixed typed codec error")
	}
	switch coded.Code() {
	case "order_pay_entry_invalid", "order_pay_entry_key_unavailable", "order_pay_entry_seal_failed":
	default:
		t.Fatal("unexpected or potentially unredacted error code")
	}
	if err.Error() != coded.Code() {
		t.Fatal("error included details beyond its fixed code")
	}
}

func TestOrderAttemptPayEntryRandomCredentialAndIndependentCipher(t *testing.T) {
	k, b := payEntryTestKeys(t), payEntryTestBinding()
	seen := make(map[string]bool)
	for i := 0; i < 8; i++ {
		m, err := k.Issue(b)
		if err != nil {
			t.Fatal(err)
		}
		token, err := k.Open(b, m)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
		if err != nil || len(raw) != 32 || len(token) != 43 || base64.RawURLEncoding.EncodeToString(raw) != token || seen[token] {
			t.Fatal("credential must be independent canonical random 256-bit base64url")
		}
		seen[token] = true
		if m.KeyVersion != "v1" || m.Digest != sha256.Sum256([]byte(token)) || len(m.Ciphertext) != 12+43+16 {
			t.Fatal("wrong envelope shape or credential digest")
		}
		plain, err := payEntryOracleAEAD(t).Open(nil, m.Ciphertext[:12], m.Ciphertext[12:], payEntryOracleAAD(t, b, "v1", m.Digest))
		if err != nil || string(plain) != token {
			t.Fatal("independent AES-GCM oracle could not open envelope")
		}
		for _, value := range []any{k, m, &m} {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			for _, output := range []string{fmt.Sprint(value), fmt.Sprintf("%+v", value), fmt.Sprintf("%#v", value), string(encoded)} {
				if strings.Contains(output, token) || strings.Contains(output, strings.Repeat("A", 32)) {
					t.Fatal("formatting disclosed credential or key")
				}
			}
		}
	}
}

func TestOrderAttemptPayEntryRejectsEachBindingAndEnvelopeMutation(t *testing.T) {
	k, b := payEntryTestKeys(t), payEntryTestBinding()
	m, err := k.Issue(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tenant", "store", "order", "version"} {
		t.Run(field, func(t *testing.T) {
			changed := b
			switch field {
			case "tenant":
				changed.TenantID = b.StoreID
			case "store":
				changed.StoreID = b.OrderID
			case "order":
				changed.OrderID = b.TenantID
			case "version":
				changed.Version++
			}
			plain, err := k.Open(changed, m)
			payEntryRequireError(t, err)
			if plain != "" {
				t.Fatal("binding mismatch returned plaintext")
			}
		})
	}
	for _, field := range []string{"nonce", "ciphertext", "tag", "digest", "key_version", "empty", "oversize"} {
		t.Run(field, func(t *testing.T) {
			changed := m
			changed.Ciphertext = bytes.Clone(m.Ciphertext)
			switch field {
			case "nonce":
				changed.Ciphertext[0] ^= 1
			case "ciphertext":
				changed.Ciphertext[12] ^= 1
			case "tag":
				changed.Ciphertext[len(changed.Ciphertext)-1] ^= 1
			case "digest":
				changed.Digest[0] ^= 1
			case "key_version":
				changed.KeyVersion = "unknown-secret-version"
			case "empty":
				changed.Ciphertext = nil
			case "oversize":
				changed.Ciphertext = append(changed.Ciphertext, 0)
			}
			plain, err := k.Open(b, changed)
			payEntryRequireError(t, err)
			if plain != "" {
				t.Fatal("tampered envelope returned plaintext")
			}
		})
	}
}

func TestOrderAttemptPayEntryRejectsBadKeysAndInvalidBindings(t *testing.T) {
	good := bytes.Repeat([]byte{0x41}, 32)
	for _, keys := range []map[string][]byte{nil, {}, {"v1": nil}, {"v1": make([]byte, 32)}, {"v1": good[:31]}, {"v2": good}, {"v1": good, "bad/version": good}, {"v1": good, "retired": nil}} {
		_, err := orderattempt.NewPayEntryKeys("v1", keys)
		payEntryRequireError(t, err)
	}
	for _, version := range []string{"", "bad/version", strings.Repeat("x", 65)} {
		_, err := orderattempt.NewPayEntryKeys(version, map[string][]byte{version: good})
		payEntryRequireError(t, err)
	}
	k, b := payEntryTestKeys(t), payEntryTestBinding()
	for _, bad := range []PayEntryBinding{{}, {TenantID: b.TenantID, StoreID: b.StoreID, OrderID: b.OrderID}, {TenantID: b.TenantID, StoreID: b.StoreID, OrderID: b.OrderID, Version: -1}, {TenantID: strings.ToUpper(b.TenantID), StoreID: b.StoreID, OrderID: b.OrderID, Version: 1}, {TenantID: b.TenantID, StoreID: "bad", OrderID: b.OrderID, Version: 1}, {TenantID: b.TenantID, StoreID: b.StoreID, OrderID: "", Version: 1}} {
		_, err := k.Issue(bad)
		payEntryRequireError(t, err)
	}
	var absent *PayEntryKeys
	_, err := absent.Issue(b)
	payEntryRequireError(t, err)
	plain, err := absent.Open(b, PayEntryMaterial{})
	payEntryRequireError(t, err)
	if plain != "" {
		t.Fatal("nil ring returned plaintext")
	}
}

func TestOrderAttemptPayEntryRotationCopiesKeysAndRetainsOriginalCredential(t *testing.T) {
	b := payEntryTestBinding()
	raw := bytes.Repeat([]byte{0x41}, 32)
	input := map[string][]byte{"v1": raw}
	k, err := orderattempt.NewPayEntryKeys("v1", input)
	if err != nil {
		t.Fatal(err)
	}
	clear(raw)
	delete(input, "v1")
	m, err := k.Issue(b)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := orderattempt.NewPayEntryKeys("v2", map[string][]byte{"v1": bytes.Repeat([]byte{0x41}, 32), "v2": bytes.Repeat([]byte{0x42}, 32), "alias": bytes.Repeat([]byte{0x41}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	original, err := k.Open(b, m)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := rotated.Open(b, m)
	if err != nil || original != replayed {
		t.Fatal("retained version failed to return exact original credential")
	}
	newMaterial, err := rotated.Issue(b)
	if err != nil || newMaterial.KeyVersion != "v2" {
		t.Fatal("active key version was not used")
	}
	newToken, err := rotated.Open(b, newMaterial)
	if err != nil || newToken == original {
		t.Fatal("new issuance reused old credential")
	}
	// Same-key alias proves the version is authenticated, beyond selecting a key.
	for _, version := range []string{"v2", "alias"} {
		m.KeyVersion = version
		plain, err := rotated.Open(b, m)
		payEntryRequireError(t, err)
		if plain != "" {
			t.Fatal("changed retained key version returned plaintext")
		}
	}
}

func TestOrderAttemptPayEntryRejectsAuthenticatedMalformedCredentialAndDigest(t *testing.T) {
	k, b := payEntryTestKeys(t), payEntryTestBinding()
	valid := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x41}, 32))
	// Final 'F' has nonzero unused bits but lenient base64 decodes it like valid 'E'.
	for _, plain := range []string{"", "secret-not-a-token", valid + "=", valid + "\n", valid[:42] + "F", base64.RawURLEncoding.EncodeToString(make([]byte, 31)), base64.RawURLEncoding.EncodeToString(make([]byte, 33))} {
		m := payEntryOracleMaterial(t, b, plain, sha256.Sum256([]byte(plain)))
		got, err := k.Open(b, m)
		payEntryRequireError(t, err)
		if got != "" {
			t.Fatal("authenticated malformed credential returned plaintext")
		}
	}
	m := payEntryOracleMaterial(t, b, valid, sha256.Sum256([]byte("different-credential")))
	got, err := k.Open(b, m)
	payEntryRequireError(t, err)
	if got != "" {
		t.Fatal("valid tag with mismatched plaintext digest returned plaintext")
	}
	m = payEntryOracleMaterial(t, b, valid, sha256.Sum256([]byte(valid)))
	got, err = k.Open(b, m)
	if err != nil || got != valid {
		t.Fatal("independently sealed valid credential failed")
	}
}
