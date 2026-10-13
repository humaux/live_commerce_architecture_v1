// Purpose: Exercise the public OA1 frozen-plan codec with independent tamper, swap and key-loss cases.
// Depends on: orderattempt public codec, existing pricing calculator and standard-library crypto/JSON.
// Used by: foundation focused CI gates; all fixtures are synthetic, with no PG or external calls.
// Invariants: order-attempt-v1 §§1–3; I01, I08, I11, I18. Local execution NOT_RUN by owner instruction.
package foundation_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/orderattempt"
	"livecommerce/internal/pricing"
)

func oaPlanFixture(t *testing.T) orderattempt.FrozenPlan {
	t.Helper()
	calculation, err := pricing.Calculate(pricing.Policy{MarketID: "00000000-0000-4000-8000-000000000010", Country: "US", Method: "home", Currency: "USD", Version: 1, Enabled: true, QuoteTTLSeconds: 300, ShippingMode: "country_flat", TaxMode: "exclusive", TaxBasis: "goods", ShippingMinor: 50}, []pricing.AmountLine{{UnitPriceMinor: 100, Quantity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	admitted := time.Date(2026, 10, 13, 12, 0, 0, 123456000, time.UTC)
	return orderattempt.FrozenPlan{
		Schema: 1, AttemptID: "00000000-0000-4000-8000-000000000001", ScopeID: "00000000-0000-4000-8000-000000000002", Generation: 1,
		TenantID: "00000000-0000-4000-8000-000000000003", StoreID: "00000000-0000-4000-8000-000000000004", PrincipalID: "00000000-0000-4000-8000-000000000005", BuyerOwnerID: "00000000-0000-4000-8000-000000000006",
		AdmittedAt: admitted, ExecuteBy: admitted.Add(15 * time.Minute), RequestHash: sha256.Sum256([]byte("synthetic normalized request")), FactsDigest: sha256.Sum256([]byte("synthetic authoritative facts")),
		Facts: json.RawMessage(`{ "currency": "USD", "unit_price_minor": 100, "quantity": 2 }`), Calculation: calculation,
	}
}

func oaPlanKeys(t *testing.T, version string, keys map[string][]byte) *orderattempt.PlanKeys {
	t.Helper()
	ring, err := orderattempt.NewPlanKeys(version, keys)
	if err != nil {
		t.Fatal(err)
	}
	return ring
}

func TestOrderAttemptPlanRoundTrip(t *testing.T) {
	plan := oaPlanFixture(t)
	ring := oaPlanKeys(t, "v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x41}, 32)})
	sealed, err := ring.Seal(plan)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.KeyVersion != "v1" || sealed.Digest != sha256.Sum256(encoded) || len(sealed.Ciphertext) != len(encoded)+12+16 {
		t.Fatal("wrong version, exact-plaintext digest or AES-GCM envelope")
	}
	opened, err := ring.Open(plan.Binding(), sealed)
	if err != nil {
		t.Fatal(err)
	}
	var expected orderattempt.FrozenPlan
	if err := json.Unmarshal(encoded, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opened, expected) || !reflect.DeepEqual(opened.Calculation, plan.Calculation) {
		t.Fatal("frozen facts, identity or existing pricing result changed")
	}
	again, err := ring.Seal(plan)
	if err != nil || bytes.Equal(sealed.Ciphertext[:12], again.Ciphertext[:12]) || sealed.Digest != again.Digest {
		t.Fatal("nonce reused or immutable digest changed")
	}
	// A codec has no wall-clock authority: an old admitted plan still decrypts.
	if _, err := ring.Open(plan.Binding(), again); err != nil {
		t.Fatal("codec substituted a current-time expiry check")
	}
	binding := plan.Binding()
	zone := time.FixedZone("synthetic-pg-zone", 8*60*60)
	binding.AdmittedAt = binding.AdmittedAt.In(zone)
	binding.ExecuteBy = binding.ExecuteBy.In(zone)
	if _, err := ring.Open(binding, sealed); err != nil {
		t.Fatal("same PG instants in a different session timezone failed")
	}
}

func TestOrderAttemptPlanTamperAndSwap(t *testing.T) {
	plan := oaPlanFixture(t)
	ring := oaPlanKeys(t, "v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x41}, 32), "same-key-other-version": bytes.Repeat([]byte{0x41}, 32)})
	sealed, err := ring.Seal(plan)
	if err != nil {
		t.Fatal(err)
	}
	otherID := "00000000-0000-4000-8000-000000000099"
	changes := map[string]func(*orderattempt.PlanBinding){
		"attempt": func(b *orderattempt.PlanBinding) { b.AttemptID = otherID },
		"scope": func(b *orderattempt.PlanBinding) { b.ScopeID = otherID },
		"tenant": func(b *orderattempt.PlanBinding) { b.TenantID = otherID },
		"store": func(b *orderattempt.PlanBinding) { b.StoreID = otherID },
		"principal": func(b *orderattempt.PlanBinding) { b.PrincipalID = otherID },
		"owner": func(b *orderattempt.PlanBinding) { b.BuyerOwnerID = otherID },
		"generation": func(b *orderattempt.PlanBinding) { b.Generation++ },
		"request": func(b *orderattempt.PlanBinding) { b.RequestHash[0] ^= 1 },
		"facts": func(b *orderattempt.PlanBinding) { b.FactsDigest[0] ^= 1 },
		"deadline": func(b *orderattempt.PlanBinding) { b.AdmittedAt = b.AdmittedAt.Add(time.Minute); b.ExecuteBy = b.ExecuteBy.Add(time.Minute) },
		"schema": func(b *orderattempt.PlanBinding) { b.Schema++ },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			binding := plan.Binding()
			change(&binding)
			oaPlanMustReject(t, ring, binding, sealed, orderattempt.ErrPlanInvalid)
		})
	}
	for _, offset := range []int{0, 12, len(sealed.Ciphertext)-1} {
		t.Run(fmt.Sprintf("cipher-byte-%d", offset), func(t *testing.T) {
			bad := sealed
			bad.Ciphertext = bytes.Clone(sealed.Ciphertext)
			bad.Ciphertext[offset] ^= 1
			oaPlanMustReject(t, ring, plan.Binding(), bad, orderattempt.ErrPlanInvalid)
		})
	}
	bad := sealed
	bad.Digest[0] ^= 1
	oaPlanMustReject(t, ring, plan.Binding(), bad, orderattempt.ErrPlanInvalid)
	bad = sealed
	bad.KeyVersion = "same-key-other-version"
	oaPlanMustReject(t, ring, plan.Binding(), bad, orderattempt.ErrPlanInvalid)
	for _, ciphertext := range [][]byte{nil, sealed.Ciphertext[:12], make([]byte, 1024*1024+29)} {
		bad = sealed
		bad.Ciphertext = ciphertext
		oaPlanMustReject(t, ring, plan.Binding(), bad, orderattempt.ErrPlanInvalid)
	}
}

func oaPlanMustReject(t *testing.T, ring *orderattempt.PlanKeys, binding orderattempt.PlanBinding, sealed orderattempt.SealedPlan, want error) {
	t.Helper()
	got, err := ring.Open(binding, sealed)
	if !errors.Is(err, want) || !reflect.DeepEqual(got, orderattempt.FrozenPlan{}) {
		t.Fatalf("failure did not close with zero plan and fixed error: %v", err)
	}
	var typed *orderattempt.PlanError
	if !errors.As(err, &typed) || typed.Code() != err.Error() {
		t.Fatal("plan error is not a typed fixed code")
	}
}

func TestOrderAttemptPlanRotationAndKeyLoss(t *testing.T) {
	plan := oaPlanFixture(t)
	original := bytes.Repeat([]byte{0x41}, 32)
	input := map[string][]byte{"v1": original}
	ring := oaPlanKeys(t, "v1", input)
	sealed, err := ring.Seal(plan)
	if err != nil {
		t.Fatal(err)
	}
	clear(original)
	delete(input, "v1")
	if _, err := ring.Open(plan.Binding(), sealed); err != nil {
		t.Fatal("caller mutation changed retained key material")
	}
	for _, formatted := range []string{fmt.Sprintf("%v", ring), fmt.Sprintf("%+v", ring), fmt.Sprintf("%#v", ring)} {
		if formatted != "[order-attempt plan keys]" {
			t.Fatal("keyring formatting exposed key internals")
		}
	}
	rotated := oaPlanKeys(t, "v2", map[string][]byte{"v1": bytes.Repeat([]byte{0x41}, 32), "v2": bytes.Repeat([]byte{0x42}, 32)})
	if _, err := rotated.Open(plan.Binding(), sealed); err != nil {
		t.Fatal("retained historical key failed")
	}
	newSeal, err := rotated.Seal(plan)
	if err != nil || newSeal.KeyVersion != "v2" {
		t.Fatal("rotation did not seal with active key")
	}
	missing := oaPlanKeys(t, "v2", map[string][]byte{"v2": bytes.Repeat([]byte{0x42}, 32)})
	oaPlanMustReject(t, missing, plan.Binding(), sealed, orderattempt.ErrPlanKeyUnavailable)
	wrong := oaPlanKeys(t, "v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x43}, 32)})
	oaPlanMustReject(t, wrong, plan.Binding(), sealed, orderattempt.ErrPlanInvalid)
	var absent *orderattempt.PlanKeys
	oaPlanMustReject(t, absent, plan.Binding(), sealed, orderattempt.ErrPlanKeyUnavailable)
	if got, err := absent.Seal(plan); !errors.Is(err, orderattempt.ErrPlanKeyUnavailable) || !reflect.DeepEqual(got, orderattempt.SealedPlan{}) {
		t.Fatal("nil sealing ring did not fail closed")
	}
}

func TestOrderAttemptPlanValidationAndBounds(t *testing.T) {
	for name, input := range map[string]map[string][]byte{
		"nil": nil, "empty": {}, "absent-active": {"v2": bytes.Repeat([]byte{0x41}, 32)},
		"nil-key": {"v1": nil}, "short-key": {"v1": make([]byte, 31)}, "long-key": {"v1": make([]byte, 33)}, "zero-key": {"v1": make([]byte, 32)},
		"bad-old-key": {"v1": bytes.Repeat([]byte{0x41}, 32), "v0": nil},
	} {
		t.Run(name, func(t *testing.T) {
			if ring, err := orderattempt.NewPlanKeys("v1", input); !errors.Is(err, orderattempt.ErrPlanKeyUnavailable) || ring != nil {
				t.Fatal("invalid ring accepted")
			}
		})
	}
	for _, version := range []string{"", "with space", strings.Repeat("v", 65)} {
		if ring, err := orderattempt.NewPlanKeys(version, map[string][]byte{version: bytes.Repeat([]byte{0x41}, 32)}); err == nil || ring != nil {
			t.Fatal("invalid key version accepted")
		}
	}
	ring := oaPlanKeys(t, "v1", map[string][]byte{"v1": bytes.Repeat([]byte{0x41}, 32)})
	changes := map[string]func(*orderattempt.FrozenPlan){
		"schema": func(p *orderattempt.FrozenPlan) { p.Schema = 0 },
		"identity": func(p *orderattempt.FrozenPlan) { p.AttemptID = "" },
		"generation": func(p *orderattempt.FrozenPlan) { p.Generation = 0 },
		"admitted": func(p *orderattempt.FrozenPlan) { p.AdmittedAt = time.Time{} },
		"deadline": func(p *orderattempt.FrozenPlan) { p.ExecuteBy = p.ExecuteBy.Add(time.Second) },
		"request": func(p *orderattempt.FrozenPlan) { p.RequestHash = [32]byte{} },
		"facts-hash": func(p *orderattempt.FrozenPlan) { p.FactsDigest = [32]byte{} },
		"facts-nil": func(p *orderattempt.FrozenPlan) { p.Facts = nil },
		"facts-malformed": func(p *orderattempt.FrozenPlan) { p.Facts = json.RawMessage(`{"synthetic-secret":`) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			plan := oaPlanFixture(t)
			change(&plan)
			got, err := ring.Seal(plan)
			if !errors.Is(err, orderattempt.ErrPlanInvalid) || !reflect.DeepEqual(got, orderattempt.SealedPlan{}) || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("invalid plan accepted or leaked")
			}
		})
	}
	plan := oaPlanFixture(t)
	plan.Facts = json.RawMessage(`{"padding":"` + strings.Repeat("x", 1024*1024) + `"}`)
	if got, err := ring.Seal(plan); !errors.Is(err, orderattempt.ErrPlanInvalid) || !reflect.DeepEqual(got, orderattempt.SealedPlan{}) {
		t.Fatal("oversized plaintext accepted")
	}
	// RawMessage can fit while JSON's HTML escaping pushes the actual encoded plan past 1 MiB.
	plan.Facts = json.RawMessage(`{"padding":"` + strings.Repeat("<", 200000) + `"}`)
	if _, err := ring.Seal(plan); !errors.Is(err, orderattempt.ErrPlanInvalid) {
		t.Fatal("encoded plaintext expansion bypassed the bound")
	}
	plan.Facts = json.RawMessage(`{"padding":""}`)
	base, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Facts = json.RawMessage(`{"padding":"` + strings.Repeat("x", 1024*1024-len(base)) + `"}`)
	boundary, err := ring.Seal(plan)
	if err != nil || len(boundary.Ciphertext) != 1024*1024+28 {
		t.Fatal("exact 1 MiB plaintext boundary rejected")
	}
	if _, err := ring.Open(plan.Binding(), boundary); err != nil {
		t.Fatal("bounded boundary ciphertext did not round trip")
	}
}

func TestOrderAttemptPlanIndependentEnvelope(t *testing.T) {
	plan := oaPlanFixture(t)
	key := bytes.Repeat([]byte{0x41}, 32)
	ring := oaPlanKeys(t, "v1", map[string][]byte{"v1": key})
	sealed, err := ring.Seal(plan)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	// Independent wire definition: key version and digest are authenticated with the trusted PG binding.
	aad, err := json.Marshal(struct {
		Domain string `json:"domain"`
		KeyVersion string `json:"key_version"`
		Binding orderattempt.PlanBinding `json:"binding"`
		Digest [32]byte `json:"digest"`
	}{"livecommerce/order-attempt-plan/v1", "v1", plan.Binding(), sealed.Digest})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := aead.Open(nil, sealed.Ciphertext[:12], sealed.Ciphertext[12:], aad)
	if err != nil || sha256.Sum256(plain) != sealed.Digest {
		t.Fatal("independent AES256-GCM open or exact-plaintext digest failed")
	}
	// Correctly encrypted but wrong digest must fail even when GCM authentication succeeds.
	wrongDigest := sealed
	wrongDigest.Digest = sha256.Sum256([]byte("different plaintext"))
	wrongAAD, err := json.Marshal(struct {
		Domain string `json:"domain"`
		KeyVersion string `json:"key_version"`
		Binding orderattempt.PlanBinding `json:"binding"`
		Digest [32]byte `json:"digest"`
	}{"livecommerce/order-attempt-plan/v1", "v1", plan.Binding(), wrongDigest.Digest})
	if err != nil {
		t.Fatal(err)
	}
	wrongDigest.Ciphertext = aead.Seal(bytes.Clone(sealed.Ciphertext[:12]), sealed.Ciphertext[:12], plain, wrongAAD)
	oaPlanMustReject(t, ring, plan.Binding(), wrongDigest, orderattempt.ErrPlanInvalid)
	// Injection with valid GCM and digest: the plaintext must still match the independently held row.
	plan.BuyerOwnerID = "00000000-0000-4000-8000-000000000099"
	wrongPlain, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	forged := sealed
	forged.Digest = sha256.Sum256(wrongPlain)
	originalPlan := oaPlanFixture(t)
	forgedAAD, err := json.Marshal(struct {
		Domain string `json:"domain"`
		KeyVersion string `json:"key_version"`
		Binding orderattempt.PlanBinding `json:"binding"`
		Digest [32]byte `json:"digest"`
	}{"livecommerce/order-attempt-plan/v1", "v1", originalPlan.Binding(), forged.Digest})
	if err != nil {
		t.Fatal(err)
	}
	forged.Ciphertext = aead.Seal(bytes.Clone(sealed.Ciphertext[:12]), sealed.Ciphertext[:12], wrongPlain, forgedAAD)
	oaPlanMustReject(t, ring, originalPlan.Binding(), forged, orderattempt.ErrPlanInvalid)
}
