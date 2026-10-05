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
	view := messageView{Text: stringField(message, "text")}
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
