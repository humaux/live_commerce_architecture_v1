// Purpose: routing of the manual private reply (live-console-v1 §3.3 row 2, §4.3) on the existing meta.private_reply route: the
// automatic claim-link reply and the manual reply differ only by the frozen request's message_type, so the route's Check / loader /
// Dispatch pick the send adapter (send_dm.go) for message_type manual_private_reply and keep the original claim-link path otherwise.
// Depends on: send_dm.go (sendAdapter), routes.go (adapter).
// Used by: routes.go (RoutesV2 builds the route with the send adapter).
// Invariants: the one-private-reply-per-comment key (mpr:) and its :m1 class live in SQL (inbox.plan_manual_private_reply).

package metareply

import "encoding/json"

// isManualReply reports whether a frozen operation request is a manual private reply. A request that does not parse as an object is
// not manual (the original path then refuses it as malformed).
func isManualReply(raw []byte) bool {
	var t struct {
		MessageType string `json:"message_type"`
	}
	return json.Unmarshal(raw, &t) == nil && t.MessageType == manualType
}
