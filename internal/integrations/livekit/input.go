package livekit

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"time"
)

var (
	publisherPattern = regexp.MustCompile(`^lcp_[0-9a-f]{32}$`)
	participantSID   = regexp.MustCompile(`^PA_[A-Za-z0-9_-]{1,100}$`)
	trackSID         = regexp.MustCompile(`^TR_[A-Za-z0-9_-]{1,100}$`)
	inputRoomSID     = regexp.MustCompile(`^RM_[A-Za-z0-9_-]{1,100}$`)
)

// PublisherGrant must come from persisted server authority, not browser input.
// Syntax and time validation here do not establish merchant access or ownership.
type PublisherGrant struct {
	RoomName, Identity  string
	IssuedAt, ExpiresAt int64
}

// PublisherToken redacts diagnostics; Bearer is the deliberate secret exit for
// the future authenticated, no-store HTTPS response. Never put it in receipts.
type PublisherToken struct{ value string }

func (t PublisherToken) Bearer() string   { return t.value }
func (PublisherToken) String() string     { return "livekit.PublisherToken{redacted}" }
func (t PublisherToken) GoString() string { return t.String() }
func (PublisherToken) MarshalJSON() ([]byte, error) {
	return []byte(`"livekit.PublisherToken{redacted}"`), nil
}

// InputTarget must identify the exact room/publisher owned by the stored attempt.
type InputTarget struct{ RoomName, Identity string }

// InputObservation reports provider track metadata, not decoded media quality.
type InputObservation struct {
	RoomName, Identity, ParticipantID, State string
	CameraPublished, CameraMuted             bool
	MicrophonePublished, MicrophoneMuted     bool
}

type InputRoomObservation struct{ RoomName, RoomID string }

func validInputTarget(t InputTarget) bool {
	return roomPattern.MatchString(t.RoomName) && publisherPattern.MatchString(t.Identity)
}

// MintPublisher signs fixed claims without I/O or replay deadline extension.
// Admission persistence, revocation and the connected lifetime belong upstream.
func (c *Client) MintPublisher(g PublisherGrant) (PublisherToken, error) {
	now := time.Now().Unix()
	if !c.ready() || !validInputTarget(InputTarget{g.RoomName, g.Identity}) ||
		g.IssuedAt <= 0 || g.IssuedAt > now || g.ExpiresAt <= now || g.ExpiresAt-g.IssuedAt > 60 {
		return PublisherToken{}, ErrInvalid
	}
	claims := struct {
		Issuer    string `json:"iss"`
		Subject   string `json:"sub"`
		IssuedAt  int64  `json:"iat"`
		NotBefore int64  `json:"nbf"`
		Expires   int64  `json:"exp"`
		Video     struct {
			Room                 string   `json:"room"`
			RoomJoin             bool     `json:"roomJoin"`
			CanPublish           bool     `json:"canPublish"`
			CanPublishSources    []string `json:"canPublishSources"`
			CanSubscribe         bool     `json:"canSubscribe"`
			CanPublishData       bool     `json:"canPublishData"`
			CanUpdateOwnMetadata bool     `json:"canUpdateOwnMetadata"`
		} `json:"video"`
	}{Issuer: c.config.APIKey, Subject: g.Identity, IssuedAt: g.IssuedAt, NotBefore: g.IssuedAt, Expires: g.ExpiresAt}
	claims.Video.Room = g.RoomName
	claims.Video.RoomJoin = true
	claims.Video.CanPublish = true
	claims.Video.CanPublishSources = []string{"camera", "microphone"}
	data, err := json.Marshal(claims)
	if err != nil {
		return PublisherToken{}, ErrInvalid
	}
	return PublisherToken{c.signClaims(data)}, nil
}

// callInput only accepts fixed method and grant choices from this package.
func (c *Client) callInput(ctx context.Context, room, method, grant string, payload any) ([]byte, bool) {
	if ctx.Err() != nil {
		return nil, false
	}
	now := time.Now().Unix()
	claims := struct {
		Issuer    string         `json:"iss"`
		IssuedAt  int64          `json:"iat"`
		NotBefore int64          `json:"nbf"`
		Expires   int64          `json:"exp"`
		Video     map[string]any `json:"video"`
	}{c.config.APIKey, now, now, now + 60, map[string]any{"room": room, grant: true}}
	data, err := json.Marshal(claims)
	if err != nil {
		return nil, false
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	return c.callAuthorized(ctx, "RoomService", method, c.signClaims(data), body)
}

func (c *Client) ObserveInput(ctx context.Context, target InputTarget) (InputObservation, error) {
	if !c.ready() || ctx == nil || !validInputTarget(target) {
		return InputObservation{}, ErrInvalid
	}
	body, ok := c.callInput(ctx, target.RoomName, "GetParticipant", "roomAdmin", struct {
		Room     string `json:"room"`
		Identity string `json:"identity"`
	}{target.RoomName, target.Identity})
	if !ok {
		return InputObservation{}, ErrUnavailable
	}
	return decodeInput(body, target)
}

// RemoveInput returns only an RPC acknowledgement; strict cached-token revocation
// requires the qualified Cloud profile and controller described in the contract.
func (c *Client) RemoveInput(ctx context.Context, target InputTarget, cutoff int64) error {
	now := time.Now().Unix()
	if !c.ready() || ctx == nil || !validInputTarget(target) || cutoff <= 0 || cutoff > now || now-cutoff > 30 {
		return ErrInvalid
	}
	body, ok := c.callInput(ctx, target.RoomName, "RemoveParticipant", "roomAdmin", struct {
		Room          string `json:"room"`
		Identity      string `json:"identity"`
		RevokeTokenTS string `json:"revoke_token_ts"`
	}{target.RoomName, target.Identity, strconv.FormatInt(cutoff, 10)})
	if !ok || !emptyAck(body) {
		return ErrUnknown
	}
	return nil
}

// DeleteInputRoom does not establish token revocation or Egress termination.
func (c *Client) DeleteInputRoom(ctx context.Context, room string) error {
	if !c.ready() || ctx == nil || !roomPattern.MatchString(room) {
		return ErrInvalid
	}
	body, ok := c.callInput(ctx, room, "DeleteRoom", "roomCreate", struct {
		Room string `json:"room"`
	}{room})
	if !ok || !emptyAck(body) {
		return ErrUnknown
	}
	return nil
}

// ObserveInputRoom reports a momentary observation; ErrNotObserved is not proof
// that a cached token cannot recreate this room. See livekit-input-protocol-v1.md.
func (c *Client) ObserveInputRoom(ctx context.Context, room string) (InputRoomObservation, error) {
	if !c.ready() || ctx == nil || !roomPattern.MatchString(room) {
		return InputRoomObservation{}, ErrInvalid
	}
	body, ok := c.callInput(ctx, room, "ListRooms", "roomList", struct {
		Names []string `json:"names"`
	}{[]string{room}})
	if !ok {
		return InputRoomObservation{}, ErrUnavailable
	}
	obj, err := decodeObject(body)
	if err != nil {
		return InputRoomObservation{}, ErrUnavailable
	}
	v, found := obj["rooms"]
	if !found {
		return InputRoomObservation{}, ErrNotObserved
	}
	rows, ok := v.([]any)
	if !ok {
		return InputRoomObservation{}, ErrUnavailable
	}
	if len(rows) == 0 {
		return InputRoomObservation{}, ErrNotObserved
	}
	if len(rows) != 1 {
		return InputRoomObservation{}, ErrUnavailable
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		return InputRoomObservation{}, ErrUnavailable
	}
	name, nameOK := row["name"].(string)
	sid, sidOK := row["sid"].(string)
	if !nameOK || name != room || !sidOK || !inputRoomSID.MatchString(sid) {
		return InputRoomObservation{}, ErrUnavailable
	}
	return InputRoomObservation{RoomName: name, RoomID: sid}, nil
}

func emptyAck(body []byte) bool {
	obj, err := decodeObject(body)
	return err == nil && len(obj) == 0
}

func decodeInput(body []byte, target InputTarget) (InputObservation, error) {
	obj, err := decodeObject(body)
	if err != nil {
		return InputObservation{}, ErrUnavailable
	}
	identity, identityOK := obj["identity"].(string)
	sid, sidOK := obj["sid"].(string)
	if !identityOK || identity != target.Identity || !sidOK || !participantSID.MatchString(sid) {
		return InputObservation{}, ErrUnavailable
	}
	state := "JOINING"
	if v, present := obj["state"]; present {
		state, err = inputEnum(v, []string{"JOINING", "JOINED", "ACTIVE", "DISCONNECTED"})
		if err != nil {
			return InputObservation{}, ErrUnavailable
		}
	}
	out := InputObservation{RoomName: target.RoomName, Identity: identity, ParticipantID: sid, State: state}
	v, present := obj["tracks"]
	if !present {
		return out, nil
	}
	tracks, ok := v.([]any)
	if !ok || len(tracks) > 2 {
		return InputObservation{}, ErrUnavailable
	}
	seenSID := map[string]bool{}
	seenSource := map[string]bool{}
	for _, raw := range tracks {
		track, ok := raw.(map[string]any)
		if !ok {
			return InputObservation{}, ErrUnavailable
		}
		trackID, ok := track["sid"].(string)
		if !ok || !trackSID.MatchString(trackID) || seenSID[trackID] {
			return InputObservation{}, ErrUnavailable
		}
		seenSID[trackID] = true
		sourceRaw, present := track["source"]
		if !present {
			return InputObservation{}, ErrUnavailable
		}
		source, err := inputEnum(sourceRaw, []string{"UNKNOWN", "CAMERA", "MICROPHONE"})
		if err != nil || source == "UNKNOWN" || seenSource[source] {
			return InputObservation{}, ErrUnavailable
		}
		seenSource[source] = true
		kind := "AUDIO"
		if typeRaw, present := track["type"]; present {
			kind, err = inputEnum(typeRaw, []string{"AUDIO", "VIDEO"})
			if err != nil {
				return InputObservation{}, ErrUnavailable
			}
		}
		if (source == "CAMERA" && kind != "VIDEO") || (source == "MICROPHONE" && kind != "AUDIO") {
			return InputObservation{}, ErrUnavailable
		}
		muted := false
		if mutedRaw, present := track["muted"]; present {
			muted, ok = mutedRaw.(bool)
			if !ok {
				return InputObservation{}, ErrUnavailable
			}
		}
		if source == "CAMERA" {
			out.CameraPublished, out.CameraMuted = true, muted
		} else {
			out.MicrophonePublished, out.MicrophoneMuted = true, muted
		}
	}
	return out, nil
}

func inputEnum(v any, names []string) (string, error) {
	switch value := v.(type) {
	case string:
		for _, name := range names {
			if value == name {
				return name, nil
			}
		}
	case json.Number:
		if len(value) == 1 && value[0] >= '0' && int(value[0]-'0') < len(names) {
			return names[int(value[0]-'0')], nil
		}
	}
	return "", errWire
}
