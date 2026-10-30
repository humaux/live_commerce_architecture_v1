// Purpose: store-contact notices use the existing renderer in all supported locales.
// Depends on: Render and synthetic merchant addresses only.
// Used by: notify's existing CI package suite.
// Invariants: no buyer PII, HTML injection, order wording or extra recipients in a contact notice.
package notify

import (
	"strings"
	"testing"
)

func TestStoreContactNoticeLocalesAndPrivacy(t *testing.T) {
	for locale, subject := range map[string]string{"zh-TW": "店鋪收到新留言", "zh-CN": "店铺收到新留言", "en": "New store message"} {
		t.Run(locale, func(t *testing.T) {
			messages := Render(Payload{Kind: KindStoreContact, To: []string{"merchant@example.test"}, StoreName: "Synthetic <b>store</b>", Locale: &locale})
			if len(messages) != 1 || messages[0].To != "merchant@example.test" || !strings.Contains(messages[0].Subject, subject) {
				t.Fatal("missing localized contact notice")
			}
			for _, forbidden := range []string{"<b>", "buyer@example.test", "訂單通知", "订单通知", "order notification"} {
				if strings.Contains(messages[0].HTML, forbidden) {
					t.Fatal("contact notice contains wrong content")
				}
			}
		})
	}
	if len(Render(Payload{Kind: KindStoreContact, To: []string{"one@example.test", "two@example.test"}})) != 0 {
		t.Fatal("contact notice admitted multiple recipients")
	}
}
