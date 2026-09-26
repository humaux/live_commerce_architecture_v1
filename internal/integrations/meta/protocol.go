// Package meta admits signed Meta webhook events. The caller must atomically
// persist the complete batch and its jobs before acknowledging a delivery.
package meta

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxBody        = 1 << 20
	maxEvents      = 1000
	maxDepth       = 64
	maxUnixSeconds = 253402300799
)

var (
	ErrConfig    = errors.New("meta: invalid configuration")
	ErrSignature = errors.New("meta: invalid signature")
	ErrJSON      = errors.New("meta: invalid JSON")
	ErrTooLarge  = errors.New("meta: admission limit exceeded")
)

type Config struct{ AppID, Object, AppSecret, VerifyToken string }
type Batch struct {
	AppID, Object, BodyHash string
	Events                  []Event
}
type Event struct {
	AssetID, Kind, ExternalID, Key, PayloadHash, QuarantineReason string
	OccurredAt                                                    *time.Time
	Payload                                                       json.RawMessage
}

// All four types can hold credentials or provider PII, including nested values.
func (Config) String() string               { return "meta.Config{redacted}" }
func (c Config) GoString() string           { return c.String() }
func (Config) MarshalJSON() ([]byte, error) { return []byte(`"meta.Config{redacted}"`), nil }
func (Batch) String() string                { return "meta.Batch{redacted}" }
func (b Batch) GoString() string            { return b.String() }
func (Batch) MarshalJSON() ([]byte, error)  { return []byte(`"meta.Batch{redacted}"`), nil }
func (Event) String() string                { return "meta.Event{redacted}" }
func (e Event) GoString() string            { return e.String() }
func (Event) MarshalJSON() ([]byte, error)  { return []byte(`"meta.Event{redacted}"`), nil }

type Verifier struct{ appID, object, appSecret, verifyToken string }

func (Verifier) String() string               { return "meta.Verifier{redacted}" }
func (v Verifier) GoString() string           { return v.String() }
func (Verifier) MarshalJSON() ([]byte, error) { return []byte(`"meta.Verifier{redacted}"`), nil }

func NewVerifier(c Config) (*Verifier, error) {
	if !digits(c.AppID) || (c.Object != "page" && c.Object != "instagram") || !safeSecret(c.AppSecret) || !safeSecret(c.VerifyToken) {
		return nil, ErrConfig
	}
	return &Verifier{c.AppID, c.Object, c.AppSecret, c.VerifyToken}, nil
}

func (v *Verifier) valid() bool {
	return v != nil && digits(v.appID) && (v.object == "page" || v.object == "instagram") && safeSecret(v.appSecret) && safeSecret(v.verifyToken)
}

func safeSecret(s string) bool {
	if len(s) < 16 || len(s) > 512 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return false
		}
	}
	return true
}

func digits(s string) bool {
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

// Verify authenticates the original bytes before parsing any JSON. The returned
// payloads are canonical, owned bytes; neither raw nor signature is retained.
func (v *Verifier) Verify(raw []byte, signature string) (Batch, error) {
	if !v.valid() {
		return Batch{}, ErrConfig
	}
	if len(raw) > maxBody {
		return Batch{}, ErrTooLarge
	}
	if len(signature) != len("sha256=")+64 || !strings.HasPrefix(signature, "sha256=") {
		return Batch{}, ErrSignature
	}
	want, err := hex.DecodeString(signature[len("sha256="):])
	if err != nil {
		return Batch{}, ErrSignature
	}
	mac := hmac.New(sha256.New, []byte(v.appSecret))
	_, _ = mac.Write(raw)
	if !hmac.Equal(mac.Sum(nil), want) {
		return Batch{}, ErrSignature
	}
	root, err := parseStrict(raw)
	if err != nil {
		return Batch{}, ErrJSON
	}
	bodyHash := sha256.Sum256(raw)
	b := Batch{AppID: v.appID, Object: v.object, BodyHash: hex.EncodeToString(bodyHash[:])}
	if err = v.normalize(root, &b); err != nil {
		return Batch{}, err
	}
	return b, nil
}

// A token walk catches decoded duplicate names, including escaped aliases.
// json.Decoder alone replaces invalid UTF-8, so validate the bytes first.
func parseStrict(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) {
		return nil, ErrJSON
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := readValue(d, 0)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, ErrJSON
	}
	root, ok := v.(map[string]any)
	if !ok {
		return nil, ErrJSON
	}
	return root, nil
}

func readValue(d *json.Decoder, depth int) (any, error) {
	t, err := d.Token()
	if err != nil {
		return nil, ErrJSON
	}
	delim, container := t.(json.Delim)
	if !container {
		return t, nil
	}
	if depth >= maxDepth {
		return nil, ErrJSON
	}
	switch delim {
	case '{':
		m := make(map[string]any)
		for d.More() {
			name, err := d.Token()
			if err != nil {
				return nil, ErrJSON
			}
			key, ok := name.(string)
			if !ok {
				return nil, ErrJSON
			}
			if _, exists := m[key]; exists {
				return nil, ErrJSON
			}
			value, err := readValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[key] = value
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return nil, ErrJSON
		}
		return m, nil
	case '[':
		a := make([]any, 0)
		for d.More() {
			value, err := readValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, value)
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return nil, ErrJSON
		}
		return a, nil
	default:
		return nil, ErrJSON
	}
}

func object(v any) (map[string]any, bool)             { m, ok := v.(map[string]any); return m, ok }
func stringField(m map[string]any, key string) string { s, _ := m[key].(string); return s }

func canonical(v any) json.RawMessage {
	// The strict parser only produces json.Marshal-supported values. In
	// particular json.Number preserves provider integer precision.
	b, _ := json.Marshal(v)
	return b
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func tupleHash(parts ...string) string { return digest(canonical(parts)) }

func (v *Verifier) emit(b *Batch, assetID, kind, externalID, reason string, payload any, at *time.Time) error {
	if len(b.Events) == maxEvents {
		return ErrTooLarge
	}
	p := canonical(payload)
	ph := digest(p)
	keyParts := []string{"meta-event-v1", v.appID, v.object, assetID, kind, externalID, ph}
	if reason == "" && (kind == "page_message" || kind == "instagram_message") {
		keyParts = []string{"meta-message-v1", v.appID, v.object, assetID, kind, externalID}
	}
	b.Events = append(b.Events, Event{AssetID: assetID, Kind: kind, ExternalID: externalID,
		Key: tupleHash(keyParts...), PayloadHash: ph, QuarantineReason: reason,
		OccurredAt: at, Payload: p})
	return nil
}

func seconds(v any) *time.Time { return timestamp(v, false) }
func millis(v any) *time.Time  { return timestamp(v, true) }
func timestamp(v any, ms bool) *time.Time {
	n, ok := v.(json.Number)
	if !ok {
		return nil
	}
	s := string(n)
	if len(s) == 0 || strings.ContainsAny(s, ".eE") {
		return nil
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil || i <= 0 {
		return nil
	}
	if ms {
		if i/1000 > maxUnixSeconds {
			return nil
		}
		t := time.UnixMilli(i).UTC()
		return &t
	}
	if i > maxUnixSeconds {
		return nil
	}
	t := time.Unix(i, 0).UTC()
	return &t
}
