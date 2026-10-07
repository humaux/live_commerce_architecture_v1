// Purpose: DB-free checks of the LC-B3b response shapes: the A13 `orders` key is omitted unless orders:read produced it,
// `auto_reply` / `display_name` are omitted when absent, and A8 rows (conversation and bundle-only) carry `link_version`.
// Depends on: encoding/json and the DTOs in service.go. Used by: go test ./internal/inbox.
package inbox

import (
	"encoding/json"
	"strings"
	"testing"
)

func marshalString(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBuyerPanelOrdersKeyOmittedWithoutPermission(t *testing.T) {
	without := marshalString(t, BuyerPanel{Platform: "messenger", Claims: []ClaimRef{}})
	if strings.Contains(without, `"orders"`) || strings.Contains(without, "auto_reply") || strings.Contains(without, "display_name") {
		t.Fatalf("absent optional keys rendered: %s", without)
	}
	empty := []OrderRef{}
	with := marshalString(t, BuyerPanel{Platform: "messenger", Claims: []ClaimRef{}, Orders: &empty, AutoReply: &AutoReply{SendState: "sent"}})
	if !strings.Contains(with, `"orders":[]`) || !strings.Contains(with, `"auto_reply":{"send_state":"sent"}`) {
		t.Fatalf("held orders:read with zero orders must render [] and auto_reply: %s", with)
	}
}

func TestConversationItemsCarryLinkVersion(t *testing.T) {
	v := int64(3)
	conv := marshalString(t, ConversationItem{ConversationID: "c", Platform: "messenger", LinkVersion: &v})
	if !strings.Contains(conv, `"link_version":3`) {
		t.Fatalf("conversation row: %s", conv)
	}
	b, s := "b", "s"
	bundle := marshalString(t, ConversationItem{BundleOnly: true, BundleID: &b, SessionID: &s, Platform: "instagram", LinkPendingManual: false})
	if !strings.Contains(bundle, `"link_version":null`) || !strings.Contains(bundle, `"link_pending_manual":false`) || !strings.Contains(bundle, `"conversation_id":null`) {
		t.Fatalf("bundle-only row: %s", bundle)
	}
}
