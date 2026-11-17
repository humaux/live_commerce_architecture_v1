// Purpose: Package neutral builds and verifies the deterministic language-neutral L01-W1 webhook corpus.
// Depends on: Go standard library only (bytes, crypto/hmac, crypto/sha256, encoding/hex, encoding/json, errors, fmt, io, math/rand, os, strings, unicode/utf8); no DB, HTTP or provider calls.
// Used by: bench/l01/neutral tests and future Go, Rust and Node L01-W1 benchmark adapters.
// Owns: synthetic raw-byte signed webhook fixtures, externalized replay/classification metadata and persisted JSON corpus verification.
// Invariants: I01, I02, I11, I18; expected classifications stay outside signed raw bytes; offline synthetic data only.
// Package neutral owns deterministic L01-W1 corpus generation and verification; it never executes adapters, touches databases, or uses production secrets.
package neutral

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	// Schema identifies the portable JSON wire contract for Go, Rust and Node consumers.
	Schema = "livecommerce/bench-l01-w1-neutral-corpus/v1"
	// EvidenceClass marks this artifact as offline synthetic input, never runtime or Go+River acceptance.
	EvidenceClass = "SYNTHETIC_OFFLINE_CORPUS"
	// DefaultFilename is the committed persisted corpus filename in bench/l01/neutral.
	DefaultFilename = "w1_corpus.json"
	// DefaultSeed matches block_1901101 in tests/foundation/l01_go_w1_test.go for reproducible shuffling.
	DefaultSeed int64 = 1901101

	// Synthetic protocol fixtures aligned with miApp/miSecret/miMessage in tests/foundation/meta_inbox_test.go.
	DefaultAppID                 = "123456789012345"
	DefaultObject                = "page"
	DefaultHMACSecret            = "meta-inbox-test-secret-only"
	DefaultAuthoritativeAssetID  = "1000000001901101"
	DefaultAuthoritativeTenantID = "00000000-0000-4000-8000-0000000000a1"
	DefaultSpoofedTenantHeader   = "00000000-0000-4000-8000-0000000000b2"
	FixedBodyText                = "synthetic-w1-fixed-body"

	UniqueValidCount      = 60
	DuplicateReplayCount  = 20
	InvalidSignatureCount = 20
	TotalCount            = UniqueValidCount + DuplicateReplayCount + InvalidSignatureCount

	ClassUniqueValid      = "unique_valid"
	ClassDuplicateReplay  = "duplicate_replay"
	ClassInvalidSignature = "invalid_signature"

	maxCorpusBytes = 1 << 20
)

var (
	ErrInvalidCorpus        = errors.New("neutral: invalid corpus envelope or counts")
	ErrBodyDigestMismatch   = errors.New("neutral: raw_body length or sha256 mismatch")
	ErrSignatureMismatch    = errors.New("neutral: signature header does not match classification")
	ErrClassificationInBody = errors.New("neutral: expected classification leaked into signed body")
	ErrSignedBodyShape      = errors.New("neutral: signed raw_body violates miMessage wire shape")
	ErrReplayMismatch       = errors.New("neutral: duplicate_replay is not byte-identical to causal primary")
	ErrDuplicateEventID     = errors.New("neutral: unexpected duplicate event_id")
)

// Counts summarizes expected request classifications outside the signed payload bytes.
type Counts struct {
	Total            int `json:"total"`
	UniqueValid      int `json:"unique_valid"`
	DuplicateReplay  int `json:"duplicate_replay"`
	InvalidSignature int `json:"invalid_signature"`
}

// Case holds one offered webhook request and its externalized oracle expectations.
// RawBody holds the exact UTF-8 wire bytes authenticated by SignatureHeader.
type Case struct {
	Index                 int    `json:"index"`
	Classification        string `json:"classification"`
	ReplayOfIndex         *int   `json:"replay_of_index,omitempty"`
	ExpectedAdmit         bool   `json:"expected_admit"`
	ExpectedIntentDelta   int    `json:"expected_intent_delta"`
	EventID               string `json:"event_id"`
	ContentType           string `json:"content_type"`
	SignatureHeader       string `json:"signature_header"`
	UntrustedTenantHeader string `json:"untrusted_tenant_header"`
	BodyBytes             int    `json:"body_bytes"`
	BodySHA256            string `json:"body_sha256"`
	RawBody               string `json:"raw_body"`
}

// Corpus is the portable language-neutral W1 request fixture.
type Corpus struct {
	Schema                string `json:"schema"`
	EvidenceClass         string `json:"evidence_class"`
	Seed                  int64  `json:"seed"`
	SyntheticOnly         bool   `json:"synthetic_only"`
	AppID                 string `json:"app_id"`
	Object                string `json:"object"`
	HMACSecret            string `json:"hmac_secret"`
	AuthoritativeAssetID  string `json:"authoritative_asset_id"`
	AuthoritativeTenantID string `json:"authoritative_tenant_id"`
	SpoofedTenantHeader   string `json:"spoofed_tenant_header"`
	Counts                Counts `json:"counts"`
	Cases                 []Case `json:"cases"`
}

type rawCandidate struct {
	eventID      string
	rawBody      []byte
	signature    string
	badSignature bool
}

// Build deterministically constructs a 100-request W1 corpus (60 unique valid, 20 byte-identical replays, 20 invalid signatures).
func Build(seed int64) Corpus {
	candidates := make([]rawCandidate, 0, TotalCount)
	for i := 0; i < UniqueValidCount; i++ {
		eventID := fmt.Sprintf("m.w1.%d.%03d", seed, i)
		raw := buildMIMessage(DefaultAuthoritativeAssetID, eventID, FixedBodyText)
		candidates = append(candidates, rawCandidate{
			eventID:   eventID,
			rawBody:   raw,
			signature: signBody(DefaultHMACSecret, raw),
		})
	}
	for i := 0; i < DuplicateReplayCount; i++ {
		orig := candidates[i]
		bodyCopy := append([]byte(nil), orig.rawBody...)
		candidates = append(candidates, rawCandidate{
			eventID:   orig.eventID,
			rawBody:   bodyCopy,
			signature: orig.signature,
		})
	}
	for i := 0; i < InvalidSignatureCount; i++ {
		eventID := fmt.Sprintf("m.w1.%d.%03d", seed, UniqueValidCount+i)
		raw := buildMIMessage(DefaultAuthoritativeAssetID, eventID, FixedBodyText)
		candidates = append(candidates, rawCandidate{
			eventID:      eventID,
			rawBody:      raw,
			signature:    invalidSignature(DefaultHMACSecret, raw, i),
			badSignature: true,
		})
	}

	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	firstValid := make(map[string]int, UniqueValidCount)
	cases := make([]Case, TotalCount)
	for i, cand := range candidates {
		sum := sha256.Sum256(cand.rawBody)
		tc := Case{
			Index:                 i,
			EventID:               cand.eventID,
			ContentType:           "application/json",
			SignatureHeader:       cand.signature,
			UntrustedTenantHeader: DefaultSpoofedTenantHeader,
			BodyBytes:             len(cand.rawBody),
			BodySHA256:            hex.EncodeToString(sum[:]),
			RawBody:               string(cand.rawBody),
		}
		if cand.badSignature {
			tc.Classification = ClassInvalidSignature
			tc.ExpectedAdmit = false
			tc.ExpectedIntentDelta = 0
		} else if origIdx, seen := firstValid[cand.eventID]; seen {
			ref := origIdx
			tc.Classification = ClassDuplicateReplay
			tc.ReplayOfIndex = &ref
			tc.ExpectedAdmit = true
			tc.ExpectedIntentDelta = 0
		} else {
			firstValid[cand.eventID] = i
			tc.Classification = ClassUniqueValid
			tc.ExpectedAdmit = true
			tc.ExpectedIntentDelta = 1
		}
		cases[i] = tc
	}

	return Corpus{
		Schema:                Schema,
		EvidenceClass:         EvidenceClass,
		Seed:                  seed,
		SyntheticOnly:         true,
		AppID:                 DefaultAppID,
		Object:                DefaultObject,
		HMACSecret:            DefaultHMACSecret,
		AuthoritativeAssetID:  DefaultAuthoritativeAssetID,
		AuthoritativeTenantID: DefaultAuthoritativeTenantID,
		SpoofedTenantHeader:   DefaultSpoofedTenantHeader,
		Counts: Counts{
			Total:            TotalCount,
			UniqueValid:      UniqueValidCount,
			DuplicateReplay:  DuplicateReplayCount,
			InvalidSignature: InvalidSignatureCount,
		},
		Cases: cases,
	}
}

// Encode validates c and serializes deterministic RFC 8259 JSON with one compact line per request case.
func Encode(c Corpus) ([]byte, error) {
	if err := Verify(c); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("{\n")
	writeJSONField(&buf, "schema", c.Schema, true)
	writeJSONField(&buf, "evidence_class", c.EvidenceClass, true)
	writeJSONField(&buf, "seed", c.Seed, true)
	writeJSONField(&buf, "synthetic_only", c.SyntheticOnly, true)
	writeJSONField(&buf, "app_id", c.AppID, true)
	writeJSONField(&buf, "object", c.Object, true)
	writeJSONField(&buf, "hmac_secret", c.HMACSecret, true)
	writeJSONField(&buf, "authoritative_asset_id", c.AuthoritativeAssetID, true)
	writeJSONField(&buf, "authoritative_tenant_id", c.AuthoritativeTenantID, true)
	writeJSONField(&buf, "spoofed_tenant_header", c.SpoofedTenantHeader, true)
	writeJSONField(&buf, "counts", c.Counts, true)
	buf.WriteString("  \"cases\": [\n")
	for i, tc := range c.Cases {
		line, err := json.Marshal(tc)
		if err != nil {
			return nil, err
		}
		buf.WriteString("    ")
		buf.Write(line)
		if i+1 < len(c.Cases) {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("  ]\n}\n")
	return buf.Bytes(), nil
}

// Parse strictly decodes serialized JSON corpus bytes and runs Verify before returning.
func Parse(raw []byte) (Corpus, error) {
	if len(raw) == 0 || len(raw) > maxCorpusBytes || !utf8.Valid(raw) {
		return Corpus{}, fmt.Errorf("%w: invalid raw size or UTF-8", ErrInvalidCorpus)
	}
	if err := validateCorpusJSON(raw); err != nil {
		return Corpus{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var c Corpus
	if err := dec.Decode(&c); err != nil {
		return Corpus{}, fmt.Errorf("%w: %v", ErrInvalidCorpus, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Corpus{}, fmt.Errorf("%w: trailing JSON data", ErrInvalidCorpus)
	}
	if err := Verify(c); err != nil {
		return Corpus{}, err
	}
	return c, nil
}

// LoadFile reads persisted serialized corpus bytes from path and validates them via Parse.
func LoadFile(path string) (Corpus, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Corpus{}, err
	}
	return Parse(raw)
}

// Verify checks envelope metadata, raw-byte SHA-256 digests, HMAC-SHA256 signatures, body isolation and causal replay invariants.
func Verify(c Corpus) error {
	if c.Schema != Schema || c.EvidenceClass != EvidenceClass || !c.SyntheticOnly {
		return fmt.Errorf("%w: schema/evidence_class/synthetic_only mismatch", ErrInvalidCorpus)
	}
	if !isDigits(c.AppID) || c.Object != DefaultObject || !isSafePrintableASCII(c.HMACSecret, 16, 512) {
		return fmt.Errorf("%w: invalid app, object or synthetic secret bounds", ErrInvalidCorpus)
	}
	if !isDigits(c.AuthoritativeAssetID) || c.AuthoritativeAssetID == "4" {
		return fmt.Errorf("%w: authoritative_asset_id must be digits distinct from sender 4", ErrInvalidCorpus)
	}
	if !isSafePrintableASCII(c.AuthoritativeTenantID, 1, 128) || !isSafePrintableASCII(c.SpoofedTenantHeader, 1, 128) || c.AuthoritativeTenantID == c.SpoofedTenantHeader {
		return fmt.Errorf("%w: authoritative and spoofed tenant identifiers must be distinct non-empty strings", ErrInvalidCorpus)
	}
	wantCounts := Counts{
		Total:            TotalCount,
		UniqueValid:      UniqueValidCount,
		DuplicateReplay:  DuplicateReplayCount,
		InvalidSignature: InvalidSignatureCount,
	}
	if c.Counts != wantCounts || len(c.Cases) != TotalCount {
		return fmt.Errorf("%w: expected 100 total (60/20/20), got counts=%+v len=%d", ErrInvalidCorpus, c.Counts, len(c.Cases))
	}

	var gotUnique, gotReplay, gotInvalid, gotAdmit, gotIntentSum int
	firstValid := make(map[string]int, UniqueValidCount)
	replayedEvents := make(map[string]int, DuplicateReplayCount)
	invalidEvents := make(map[string]int, InvalidSignatureCount)

	for i, tc := range c.Cases {
		if tc.Index != i {
			return fmt.Errorf("%w: case %d index=%d", ErrInvalidCorpus, i, tc.Index)
		}
		if tc.ContentType != "application/json" || tc.UntrustedTenantHeader != c.SpoofedTenantHeader {
			return fmt.Errorf("%w: case %d invalid header metadata", ErrInvalidCorpus, i)
		}
		rawBytes := []byte(tc.RawBody)
		if len(rawBytes) == 0 || len(rawBytes) != tc.BodyBytes || !utf8.Valid(rawBytes) {
			return fmt.Errorf("%w: case %d body_bytes=%d actual=%d", ErrBodyDigestMismatch, i, tc.BodyBytes, len(rawBytes))
		}
		sum := sha256.Sum256(rawBytes)
		if !isLowerHex64(tc.BodySHA256) || tc.BodySHA256 != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("%w: case %d sha256 mismatch", ErrBodyDigestMismatch, i)
		}

		sigBytes, ok := parseSignatureHeader(tc.SignatureHeader)
		if !ok {
			return fmt.Errorf("%w: case %d malformed X-Hub-Signature-256 header", ErrSignatureMismatch, i)
		}
		wantMAC := computeHMAC(c.HMACSecret, rawBytes)
		macValid := hmac.Equal(sigBytes, wantMAC)

		switch tc.Classification {
		case ClassUniqueValid:
			if !macValid {
				return fmt.Errorf("%w: case %d unique_valid failed HMAC verification", ErrSignatureMismatch, i)
			}
			if tc.ReplayOfIndex != nil || !tc.ExpectedAdmit || tc.ExpectedIntentDelta != 1 {
				return fmt.Errorf("%w: case %d unique_valid oracle metadata mismatch", ErrInvalidCorpus, i)
			}
			gotUnique++
		case ClassDuplicateReplay:
			if !macValid {
				return fmt.Errorf("%w: case %d duplicate_replay failed HMAC verification", ErrSignatureMismatch, i)
			}
			if tc.ReplayOfIndex == nil || !tc.ExpectedAdmit || tc.ExpectedIntentDelta != 0 {
				return fmt.Errorf("%w: case %d duplicate_replay oracle metadata mismatch", ErrInvalidCorpus, i)
			}
			gotReplay++
		case ClassInvalidSignature:
			if macValid {
				return fmt.Errorf("%w: case %d invalid_signature authenticated under HMAC secret", ErrSignatureMismatch, i)
			}
			if tc.ReplayOfIndex != nil || tc.ExpectedAdmit || tc.ExpectedIntentDelta != 0 {
				return fmt.Errorf("%w: case %d invalid_signature oracle metadata mismatch", ErrInvalidCorpus, i)
			}
			gotInvalid++
		default:
			return fmt.Errorf("%w: case %d unknown classification %q", ErrInvalidCorpus, i, tc.Classification)
		}

		if containsClassificationToken(tc.EventID) || containsClassificationToken(tc.RawBody) {
			return fmt.Errorf("%w: case %d signed payload or event_id contains classification token", ErrClassificationInBody, i)
		}
		if !isSafePrintableASCII(tc.EventID, 1, 1024) {
			return fmt.Errorf("%w: case %d invalid event_id", ErrSignedBodyShape, i)
		}
		expectedRaw := buildMIMessage(c.AuthoritativeAssetID, tc.EventID, FixedBodyText)
		if !bytes.Equal(rawBytes, expectedRaw) {
			return fmt.Errorf("%w: case %d raw_body diverges from canonical miMessage wire bytes", ErrSignedBodyShape, i)
		}

		switch tc.Classification {
		case ClassUniqueValid:
			if _, exists := firstValid[tc.EventID]; exists {
				return fmt.Errorf("%w: case %d unique_valid repeats event_id %q", ErrDuplicateEventID, i, tc.EventID)
			}
			if _, exists := invalidEvents[tc.EventID]; exists {
				return fmt.Errorf("%w: case %d unique_valid collides with invalid_signature event_id %q", ErrDuplicateEventID, i, tc.EventID)
			}
			firstValid[tc.EventID] = i
		case ClassDuplicateReplay:
			ref := *tc.ReplayOfIndex
			if ref < 0 || ref >= i {
				return fmt.Errorf("%w: case %d replay_of_index=%d out of causal range", ErrReplayMismatch, i, ref)
			}
			orig := c.Cases[ref]
			if orig.Classification != ClassUniqueValid {
				return fmt.Errorf("%w: case %d replay_of_index=%d targets %s", ErrReplayMismatch, i, ref, orig.Classification)
			}
			if firstIdx, ok := firstValid[tc.EventID]; !ok || firstIdx != ref {
				return fmt.Errorf("%w: case %d replay_of_index=%d does not match primary index for %q", ErrReplayMismatch, i, ref, tc.EventID)
			}
			if _, dup := replayedEvents[tc.EventID]; dup {
				return fmt.Errorf("%w: case %d replays event_id %q more than once", ErrReplayMismatch, i, tc.EventID)
			}
			if tc.RawBody != orig.RawBody || tc.BodyBytes != orig.BodyBytes || tc.BodySHA256 != orig.BodySHA256 ||
				tc.SignatureHeader != orig.SignatureHeader || tc.ContentType != orig.ContentType ||
				tc.UntrustedTenantHeader != orig.UntrustedTenantHeader || tc.EventID != orig.EventID {
				return fmt.Errorf("%w: case %d differs from primary case %d", ErrReplayMismatch, i, ref)
			}
			replayedEvents[tc.EventID] = i
		case ClassInvalidSignature:
			if _, exists := firstValid[tc.EventID]; exists {
				return fmt.Errorf("%w: case %d invalid_signature collides with valid event_id %q", ErrDuplicateEventID, i, tc.EventID)
			}
			if _, exists := invalidEvents[tc.EventID]; exists {
				return fmt.Errorf("%w: case %d duplicate invalid_signature event_id %q", ErrDuplicateEventID, i, tc.EventID)
			}
			invalidEvents[tc.EventID] = i
		}

		if tc.ExpectedAdmit {
			gotAdmit++
		}
		gotIntentSum += tc.ExpectedIntentDelta
	}

	if gotUnique != UniqueValidCount || gotReplay != DuplicateReplayCount || gotInvalid != InvalidSignatureCount ||
		gotAdmit != UniqueValidCount+DuplicateReplayCount || gotIntentSum != UniqueValidCount {
		return fmt.Errorf("%w: summary mismatch unique=%d replay=%d invalid=%d admit=%d intents=%d",
			ErrInvalidCorpus, gotUnique, gotReplay, gotInvalid, gotAdmit, gotIntentSum)
	}
	return nil
}

// buildMIMessage preserves the exact byte layout of miMessage in tests/foundation/meta_inbox_test.go:118.
func buildMIMessage(asset, mid, text string) []byte {
	return []byte(fmt.Sprintf(
		`{"object":"page","entry":[{"id":%q,"time":123,"messaging":[{"sender":{"id":"4"},"recipient":{"id":%q},"message":{"mid":%q,"text":%q}}]}]}`,
		asset, asset, mid, text,
	))
}

func computeHMAC(secret string, raw []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return mac.Sum(nil)
}

// signBody preserves the exact X-Hub-Signature-256 format of miSignature in tests/foundation/meta_inbox_test.go:392.
func signBody(secret string, raw []byte) string {
	return "sha256=" + hex.EncodeToString(computeHMAC(secret, raw))
}

func invalidSignature(secret string, raw []byte, ordinal int) string {
	if ordinal == 0 {
		return "sha256=" + strings.Repeat("0", 64)
	}
	digest := computeHMAC(secret, raw)
	digest[ordinal%len(digest)] ^= 0xff
	return "sha256=" + hex.EncodeToString(digest)
}

func parseSignatureHeader(h string) ([]byte, bool) {
	const prefix = "sha256="
	if len(h) != len(prefix)+64 || !strings.HasPrefix(h, prefix) {
		return nil, false
	}
	hexPart := h[len(prefix):]
	if !isLowerHex64(hexPart) {
		return nil, false
	}
	b, err := hex.DecodeString(hexPart)
	return b, err == nil && len(b) == sha256.Size
}

func containsClassificationToken(s string) bool {
	lower := strings.ToLower(s)
	for _, tok := range []string{
		ClassUniqueValid,
		ClassDuplicateReplay,
		ClassInvalidSignature,
		"replay",
		"invalid",
		".bad.",
	} {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	return false
}

func isDigits(s string) bool {
	if len(s) < 1 || len(s) > 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isSafePrintableASCII(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func isLowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func writeJSONField(buf *bytes.Buffer, name string, val any, comma bool) {
	encoded, _ := json.Marshal(val)
	buf.WriteString("  \"")
	buf.WriteString(name)
	buf.WriteString("\": ")
	buf.Write(encoded)
	if comma {
		buf.WriteByte(',')
	}
	buf.WriteByte('\n')
}

var (
	envelopeKeySet = makeKeySet(
		"schema", "evidence_class", "seed", "synthetic_only",
		"app_id", "object", "hmac_secret", "authoritative_asset_id",
		"authoritative_tenant_id", "spoofed_tenant_header", "counts", "cases",
	)
	countsKeySet = makeKeySet(
		"total", "unique_valid", "duplicate_replay", "invalid_signature",
	)
	caseKeySet = makeKeySet(
		"index", "classification", "expected_admit", "expected_intent_delta",
		"event_id", "content_type", "signature_header", "untrusted_tenant_header",
		"body_bytes", "body_sha256", "raw_body",
	)
	caseOptionalKeySet = makeKeySet("replay_of_index")
)

func makeKeySet(keys ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return m
}

// validateCorpusJSON enforces exact canonical member names, no duplicate keys, mandatory field presence
// and non-null scalar/container tokens before encoding/json struct decoding (which otherwise case-folds
// keys and defaults omitted or null scalars to Go zero values).
func validateCorpusJSON(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := validateEnvelopeObject(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("%w: trailing token", ErrInvalidCorpus)
	}
	return nil
}

func validateEnvelopeObject(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return fmt.Errorf("%w: expected root JSON object", ErrInvalidCorpus)
	}
	seen := make(map[string]struct{}, len(envelopeKeySet))
	for dec.More() {
		key, err := readObjectKey(dec, seen)
		if err != nil {
			return err
		}
		if _, ok := envelopeKeySet[key]; !ok {
			return fmt.Errorf("%w: unknown or non-canonical JSON key %q", ErrInvalidCorpus, key)
		}
		switch key {
		case "counts":
			if err := validateObjectScalars(dec, countsKeySet, nil); err != nil {
				return err
			}
		case "cases":
			if err := validateCasesArray(dec); err != nil {
				return err
			}
		default:
			if err := validateNonNullScalar(dec, key); err != nil {
				return err
			}
		}
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') {
		return fmt.Errorf("%w: malformed root object end", ErrInvalidCorpus)
	}
	if len(seen) != len(envelopeKeySet) {
		return fmt.Errorf("%w: missing required envelope JSON key", ErrInvalidCorpus)
	}
	return nil
}

func validateCasesArray(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('[') {
		return fmt.Errorf("%w: cases must be a JSON array", ErrInvalidCorpus)
	}
	for dec.More() {
		if err := validateObjectScalars(dec, caseKeySet, caseOptionalKeySet); err != nil {
			return err
		}
	}
	if end, err := dec.Token(); err != nil || end != json.Delim(']') {
		return fmt.Errorf("%w: malformed cases array end", ErrInvalidCorpus)
	}
	return nil
}

func validateObjectScalars(dec *json.Decoder, required, optional map[string]struct{}) error {
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return fmt.Errorf("%w: expected JSON object", ErrInvalidCorpus)
	}
	seen := make(map[string]struct{}, len(required)+len(optional))
	reqSeen := 0
	for dec.More() {
		key, err := readObjectKey(dec, seen)
		if err != nil {
			return err
		}
		_, isReq := required[key]
		_, isOpt := optional[key]
		if !isReq && !isOpt {
			return fmt.Errorf("%w: unknown or non-canonical JSON key %q", ErrInvalidCorpus, key)
		}
		if isReq {
			reqSeen++
		}
		if err := validateNonNullScalar(dec, key); err != nil {
			return err
		}
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') {
		return fmt.Errorf("%w: malformed object end", ErrInvalidCorpus)
	}
	if reqSeen != len(required) {
		return fmt.Errorf("%w: missing required JSON key", ErrInvalidCorpus)
	}
	return nil
}

func readObjectKey(dec *json.Decoder, seen map[string]struct{}) (string, error) {
	kTok, err := dec.Token()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidCorpus, err)
	}
	key, ok := kTok.(string)
	if !ok {
		return "", fmt.Errorf("%w: non-string object key", ErrInvalidCorpus)
	}
	if _, dup := seen[key]; dup {
		return "", fmt.Errorf("%w: duplicate JSON key %q", ErrInvalidCorpus, key)
	}
	seen[key] = struct{}{}
	return key, nil
}

func validateNonNullScalar(dec *json.Decoder, key string) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCorpus, err)
	}
	if tok == nil {
		return fmt.Errorf("%w: null JSON value for key %q", ErrInvalidCorpus, key)
	}
	if _, isDelim := tok.(json.Delim); isDelim {
		return fmt.Errorf("%w: non-scalar JSON value for key %q", ErrInvalidCorpus, key)
	}
	return nil
}
