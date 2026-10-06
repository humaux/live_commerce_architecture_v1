// Purpose: the sold-out automatic private reply (W3-04B, contract live-keyword-claims-v1 Amendment W3-04B): parsing of the frozen
// message_type sold_out_reply request and its Check, which shares the meta.private_reply route with the claim-link reply but carries no
// link, no token and no origin; the text was rendered and frozen into the request by integration.plan_claim_reply.
// Depends on: routes.go (adapter, graph POST), internal/integrations/core (DispatchRequest, DenyPolicy), SQL claims.check_meta_reply (the
// sold_out_reply branch skips the link proof).
// Used by: routes.go (checkRoute / dispatch branch).
// Invariants: one private reply per comment (the mpr: key is planned in SQL; this file never retries a send); no buyer data in the text.
// Status: MOCK (Graph is a loopback fake in tests; LIVE NOT_RUN).

package metareply

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"unicode"
	"unicode/utf8"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/core"
)

const soldOutType = "sold_out_reply"

// maxSoldOutText bounds the frozen text; plan_claim_reply keeps it under 400 characters (template <= 280 + product name <= 60).
const maxSoldOutText = 400

// soldOutRequest is the part of the frozen operation request the sold-out dispatch reads.
type soldOutRequest struct {
	V           int    `json:"v"`
	AssetID     string `json:"asset_id"`
	CommentRef  string `json:"comment_ref"`
	BundleID    string `json:"bundle_id"`
	MessageType string `json:"message_type"`
	Text        string `json:"text"`
}

// isSoldOutReply reports whether a frozen operation request is a sold-out reply; a request that does not parse is not (the claim-link path
// then refuses it as malformed).
func isSoldOutReply(raw []byte) bool {
	var t struct {
		MessageType string `json:"message_type"`
	}
	return json.Unmarshal(raw, &t) == nil && t.MessageType == soldOutType
}

// parseSoldOut validates a sold-out request against the operation's own asset; the text must satisfy soldOutTextOK (the same rule SQL applies
// before freezing it; this is the second fence before text reaches Graph).
func parseSoldOut(req core.DispatchRequest) (soldOutRequest, error) {
	var r soldOutRequest
	if err := json.Unmarshal(req.Request, &r); err != nil || r.V != 1 || r.MessageType != soldOutType || !command.ValidID(r.BundleID) ||
		r.AssetID != req.ExternalAssetID || !validCommentRef(r.CommentRef) || !soldOutTextOK(r.Text) {
		return soldOutRequest{}, errBadRequest
	}
	return r, nil
}

// soldOutTextOK is the ONE text rule shared with SQL (msgtemplates.sold_out_body / plan_claim_reply): 1..400 characters and no control character
// (Unicode Cc: C0, DEL, C1, which includes newline). Full-width space, ZWJ and NBSP are ordinary text. Keeping both fences identical is what
// guarantees a frozen text is never refused here after SQL has spent the comment's one private reply.
func soldOutTextOK(s string) bool {
	n := utf8.RuneCountInString(s)
	if !utf8.ValidString(s) || n < 1 || n > maxSoldOutText {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// checkSoldOut is Check for a sold-out reply: the same claims.check_meta_reply as the claim-link reply (deadline, source, principal, live window,
// takeover), which does not look for a link for this message type. The hash argument is a fixed digest the SQL ignores for it.
func (a *adapter) checkSoldOut(ctx context.Context, req core.DispatchRequest) error {
	if _, err := parseSoldOut(req); err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(soldOutType))
	return a.runCheck(ctx, req, hash[:])
}
