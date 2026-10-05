// Purpose: the console's IG-live read-through projection (live-console-v1 §2.4 OPEN-4): one
// stored, already-verified webhook event from social.comment_events, decrypted and projected to the
// console Comment shape. from.id never leaves this package — it is collapsed to IsPage here (the
// seller's own comment) and dropped. The media id (which the caller compares to the session source's
// source_object_id) is only inside the ciphertext, so it is returned here as MediaID.
// Depends on: PayloadKeyring.open (payload.go), ParseStrict/object/stringField (protocol.go).
// Used by: internal/live/stream.go (the A2 IG fallback); cmd/api holds the PayloadKeyring.
package meta

import (
	"errors"
	"time"

	"livecommerce/internal/command"
)

// ErrCommentRead is every malformed/undecryptable IG comment event (fixed, carries no payload bytes).
var ErrCommentRead = errors.New("meta: invalid comment event")

// CommentEnvelope is the decryption input for one social.read_comment_events row (AAD + envelope).
type CommentEnvelope struct {
	EventID     string
	Kind        string // instagram_comment | instagram_live_comment
	AppID       string
	Object      string // instagram
	AssetID     string // IG business account id
	EventKey    string
	PayloadHash string
	RouteID     string
	RouteEpoch  int64
	KeyID       string
	Nonce       []byte
	Ciphertext  []byte
	OccurredAt  time.Time
	ReceivedAt  time.Time
	Seq         int64
}

// CommentRead is the projected comment. FromID is collapsed to IsPage (from.id == the asset) and never
// returned; text/author_name exist only in this value and the console response, never in the DB.
type CommentRead struct {
	Ref           string
	ParentRef     string
	CreatedAt     time.Time
	AuthorName    string
	Text          string
	IsPage        bool
	MediaID       string
	HasAttachment bool
}

// OpenComment opens and projects one stored IG comment / live_comment event. tenantID/storeID come
// from the caller's already-authorized merchant scope (they are part of the payload AAD). A mismatch
// in any envelope field, or a body that is not the expected IG change unit, fails closed.
func (k *PayloadKeyring) OpenComment(tenantID, storeID string, env CommentEnvelope) (CommentRead, error) {
	var none CommentRead
	if k == nil || !command.ValidID(tenantID) || !command.ValidID(storeID) || !command.ValidID(env.EventID) ||
		(env.Kind != "instagram_comment" && env.Kind != "instagram_live_comment") || env.Object != "instagram" ||
		!digits(env.AppID) || !validPayloadHash(env.EventKey) || !validPayloadHash(env.PayloadHash) ||
		!command.ValidID(env.RouteID) || env.RouteEpoch <= 0 || !validPayloadKeyID(env.KeyID) ||
		len(env.Nonce) != payloadNonce {
		return none, ErrCommentRead
	}
	scope := payloadContext{Class: "event", ID: env.EventID, AppID: env.AppID, Object: env.Object,
		EventKey: env.EventKey, PayloadHash: env.PayloadHash, TenantID: tenantID, StoreID: storeID,
		RouteID: env.RouteID, RouteEpoch: env.RouteEpoch}
	plaintext, err := k.open(scope, sealedPayload{KeyID: env.KeyID, Nonce: env.Nonce, Ciphertext: env.Ciphertext})
	if err != nil {
		return none, ErrCommentRead
	}
	defer clear(plaintext)

	root, err := ParseStrict(plaintext)
	if err != nil {
		return none, ErrCommentRead
	}
	value, ok := object(root["value"])
	if !ok {
		return none, ErrCommentRead
	}
	field := "comments"
	if env.Kind == "instagram_live_comment" {
		field = "live_comments"
	}
	if stringField(root, "field") != field {
		return none, ErrCommentRead
	}
	media, ok := object(value["media"])
	if !ok {
		return none, ErrCommentRead
	}
	mediaID := stringField(media, "id")
	ref := stringField(value, "id")
	if !digits(mediaID) || !metaRef.MatchString(ref) {
		return none, ErrCommentRead
	}
	from, ok := object(value["from"])
	if !ok || !digits(stringField(from, "id")) {
		return none, ErrCommentRead
	}
	parent := stringField(value, "parent_id")
	if parent != "" && !metaRef.MatchString(parent) {
		return none, ErrCommentRead
	}
	text := stringField(value, "text")
	if len(text) > maxBody {
		return none, ErrCommentRead
	}
	name := stringField(from, "username")
	if name == "" {
		name = stringField(from, "name")
	}
	created := env.OccurredAt
	if created.IsZero() {
		created = env.ReceivedAt
	}
	return CommentRead{
		Ref:        ref,
		ParentRef:  parent,
		CreatedAt:  created,
		AuthorName: name,
		Text:       text,
		IsPage:     stringField(from, "id") == env.AssetID,
		MediaID:    mediaID,
	}, nil
}
