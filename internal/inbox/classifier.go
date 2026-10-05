// Purpose: the frozen-classifier replay of one stored inbound message unit (A9): replayMessage re-runs the frozen
// Verifier.message rules on the exact plaintext the consumer sealed and projected, extracting text and attachment types.
// A tag/hash/classifier failure renders the message unreadable (never dropped silently).
// Depends on: livecommerce/internal/integrations/meta (ParseStrict) and the frozen message shape of 0029/normalize.go.
// Used by: internal/inbox/read.go (openMessage), internal/inbox tests.

package inbox

import (
	"errors"

	"livecommerce/internal/integrations/meta"
)

// attachmentView is the one field of a message attachment the read exposes (the frozen projection's shape; the raw
// payload, url and other provider fields never leave the response body).
type attachmentView struct {
	Type string `json:"type"`
}

// messageView is the frozen-classifier extraction of one inbound message unit. sender_is_page is always false for
// inbound rows: the classifier quarantines senderID == assetID (the page talking), so no stored row can be the page.
type messageView struct {
	Text        string           `json:"text"`
	Attachments []attachmentView `json:"attachments"`
	// senderID is the page-scoped sender id (PSID); it never leaves the API process except inside the sealed dispatch copy
	// (live-console-v1 §3.4), so it is not part of any JSON view.
	senderID string
}

var errUnreadable = errors.New("inbox: message unreadable")

// replayMessage re-runs the frozen Verifier.message classifier on one stored message unit and extracts its response
// fields. The unit is the exact plaintext the consumer sealed and projected (the single sender/recipient/message object,
// not the webhook body), so a tag/hash/classifier failure — including one introduced by a later classifier change —
// renders the message unreadable instead of being dropped silently.
func replayMessage(plaintext []byte, assetID string) (messageView, error) {
	m, err := meta.ParseStrict(plaintext)
	if err != nil {
		return messageView{}, errUnreadable
	}
	sender, senderOK := m["sender"].(map[string]any)
	recipient, recipientOK := m["recipient"].(map[string]any)
	message, messageOK := m["message"].(map[string]any)
	if !senderOK || !recipientOK || !messageOK {
		return messageView{}, errUnreadable
	}
	senderID := stringField(sender, "id")
	recipientID := stringField(recipient, "id")
	mid := stringField(message, "mid")
	echo, echoPresent := message["is_echo"]
	if !digits(senderID) || !digits(recipientID) || senderID == assetID || recipientID != assetID ||
		!messageID(mid) || (echoPresent && echo != false) {
		return messageView{}, errUnreadable
	}
	view := messageView{Text: stringField(message, "text"), senderID: senderID}
	if raw, ok := message["attachments"].([]any); ok {
		view.Attachments = make([]attachmentView, 0, len(raw))
		for _, item := range raw {
			if obj, ok := item.(map[string]any); ok {
				view.Attachments = append(view.Attachments, attachmentView{Type: stringField(obj, "type")})
			}
		}
	}
	return view, nil
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func messageID(s string) bool {
	if len(s) < 1 || len(s) > 1024 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// senderName extracts the sender's display name (name, else username) from a stored message unit; nil when absent. Best effort: the
// Messenger/IG message units normally carry only ids, in which case the UI shows no name (never a guess).
func senderName(plaintext []byte) *string {
	m, err := meta.ParseStrict(plaintext)
	if err != nil {
		return nil
	}
	sender, ok := m["sender"].(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range []string{"name", "username"} {
		if v := stringField(sender, key); v != "" && len(v) <= 200 {
			return &v
		}
	}
	return nil
}
