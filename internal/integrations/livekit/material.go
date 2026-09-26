package livekit

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var (
	ErrMaterial       = errors.New("livekit: material unavailable")
	materialIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	projectIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
	uuidPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// MaterialScope must come from trusted persisted identity and versions; it is not authorization.
type MaterialScope struct {
	TenantID, StoreID, SessionID, AttemptID, ProjectID string
	CredentialVersion, MaterialVersion                 int64
}

// SealedMaterial persists through its explicit fields; its JSON representation is redacted.
type SealedMaterial struct {
	KeyID             string
	Nonce, Ciphertext []byte
}

func (SealedMaterial) String() string     { return "livekit.SealedMaterial{redacted}" }
func (s SealedMaterial) GoString() string { return s.String() }
func (SealedMaterial) MarshalJSON() ([]byte, error) {
	return []byte(`"livekit.SealedMaterial{redacted}"`), nil
}

type MaterialKeyring struct {
	activeID string
	keys     map[string][]byte
}

func (MaterialKeyring) String() string     { return "livekit.MaterialKeyring{redacted}" }
func (k MaterialKeyring) GoString() string { return k.String() }
func (MaterialKeyring) MarshalJSON() ([]byte, error) {
	return []byte(`"livekit.MaterialKeyring{redacted}"`), nil
}

// NewMaterialKeyring owns key copies; retain old IDs during rotation to open older envelopes.
func NewMaterialKeyring(activeID string, keys map[string][]byte) (*MaterialKeyring, error) {
	if len(keys) < 1 || len(keys) > 16 || !materialIDPattern.MatchString(activeID) {
		return nil, ErrMaterial
	}
	copied := make(map[string][]byte, len(keys))
	for id, key := range keys {
		if !materialIDPattern.MatchString(id) || len(key) != 32 {
			return nil, ErrMaterial
		}
		copied[id] = append([]byte(nil), key...)
	}
	if _, ok := copied[activeID]; !ok {
		return nil, ErrMaterial
	}
	return &MaterialKeyring{activeID: activeID, keys: copied}, nil
}

// Format 1 authenticates this exact field order; changing it requires a new format and migration.
type materialAAD struct {
	Domain            string `json:"domain"`
	Format            int    `json:"format"`
	KeyID             string `json:"key_id"`
	TenantID          string `json:"tenant_id"`
	StoreID           string `json:"store_id"`
	SessionID         string `json:"session_id"`
	AttemptID         string `json:"attempt_id"`
	ProjectID         string `json:"project_id"`
	CredentialVersion int64  `json:"credential_version"`
	MaterialVersion   int64  `json:"material_version"`
	Environment       string `json:"environment"`
	Endpoint          string `json:"endpoint"`
}

func (s MaterialScope) valid() bool {
	for _, id := range [...]string{s.TenantID, s.StoreID, s.SessionID, s.AttemptID} {
		if !uuidPattern.MatchString(id) || id == "00000000-0000-0000-0000-000000000000" {
			return false
		}
	}
	return projectIDPattern.MatchString(s.ProjectID) && s.CredentialVersion > 0 && s.MaterialVersion > 0
}

func (s MaterialScope) room() string { return "lc_" + strings.ReplaceAll(s.AttemptID, "-", "") }

func (s MaterialScope) aad(c *Client, keyID string) ([]byte, error) {
	return json.Marshal(materialAAD{
		Domain: "livecommerce.livekit.media", Format: 1, KeyID: keyID,
		TenantID: s.TenantID, StoreID: s.StoreID, SessionID: s.SessionID,
		AttemptID: s.AttemptID, ProjectID: s.ProjectID,
		CredentialVersion: s.CredentialVersion, MaterialVersion: s.MaterialVersion,
		Environment: c.config.Environment, Endpoint: strings.TrimSuffix(c.config.Endpoint, "/"),
	})
}

// Seal performs no I/O and grants no authority to dispatch the material.
func (k *MaterialKeyring) Seal(scope MaterialScope, client *Client, in StartInput) (SealedMaterial, error) {
	if k == nil || client == nil || !client.ready() || !scope.valid() || in.RoomName != scope.room() || !client.validStart(in) {
		return SealedMaterial{}, ErrMaterial
	}
	key, ok := k.keys[k.activeID]
	if !ok || len(key) != 32 || !materialIDPattern.MatchString(k.activeID) {
		return SealedMaterial{}, ErrMaterial
	}
	plain, err := json.Marshal(struct {
		RoomName    string   `json:"room_name"`
		AspectRatio string   `json:"aspect_ratio"`
		StreamURLs  []string `json:"stream_urls"`
	}{in.RoomName, in.AspectRatio, in.StreamURLs})
	if err != nil || len(plain) > 16<<10 {
		return SealedMaterial{}, ErrMaterial
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return SealedMaterial{}, ErrMaterial
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return SealedMaterial{}, ErrMaterial
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return SealedMaterial{}, ErrMaterial
	}
	aad, err := scope.aad(client, k.activeID)
	if err != nil {
		return SealedMaterial{}, ErrMaterial
	}
	return SealedMaterial{KeyID: k.activeID, Nonce: nonce, Ciphertext: gcm.Seal(nil, nonce, plain, aad)}, nil
}

// Open performs no I/O; the caller must separately revalidate authorization and lease.
func (k *MaterialKeyring) Open(scope MaterialScope, client *Client, sealed SealedMaterial) (StartInput, error) {
	if k == nil || client == nil || !client.ready() || !scope.valid() || !materialIDPattern.MatchString(sealed.KeyID) ||
		len(sealed.Nonce) != 12 || len(sealed.Ciphertext) < 16 || len(sealed.Ciphertext) > 16400 {
		return StartInput{}, ErrMaterial
	}
	key, ok := k.keys[sealed.KeyID]
	if !ok || len(key) != 32 {
		return StartInput{}, ErrMaterial
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return StartInput{}, ErrMaterial
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return StartInput{}, ErrMaterial
	}
	aad, err := scope.aad(client, sealed.KeyID)
	if err != nil {
		return StartInput{}, ErrMaterial
	}
	plain, err := gcm.Open(nil, sealed.Nonce, sealed.Ciphertext, aad)
	if err != nil || len(plain) > 16<<10 {
		return StartInput{}, ErrMaterial
	}
	obj, err := decodeObject(plain)
	if err != nil || len(obj) != 3 {
		return StartInput{}, ErrMaterial
	}
	room, roomOK := obj["room_name"].(string)
	aspect, aspectOK := obj["aspect_ratio"].(string)
	values, urlsOK := obj["stream_urls"].([]any)
	if !roomOK || !aspectOK || !urlsOK || len(values) < 1 || len(values) > 2 {
		return StartInput{}, ErrMaterial
	}
	urls := make([]string, len(values))
	for i, value := range values {
		url, ok := value.(string)
		if !ok {
			return StartInput{}, ErrMaterial
		}
		urls[i] = url
	}
	in := StartInput{RoomName: room, AspectRatio: aspect, StreamURLs: urls}
	if in.RoomName != scope.room() || !client.validStart(in) {
		return StartInput{}, ErrMaterial
	}
	return in, nil
}
