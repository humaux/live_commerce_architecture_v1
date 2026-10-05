package msgtemplates

import (
	"strings"
	"testing"
)

// TestValidatePublicSafe is the §3.5 table: every forbidden pattern (plus the normalization obfuscations the gate
// must catch — full-width, zero-width splits, spaced digits/letters, dashes) maps to its fixed reason, and the
// legitimate bodies (including the fixed offer-recommend template) pass.
func TestValidatePublicSafe(t *testing.T) {
	cases := []struct {
		name        string
		text        string
		storeOrigin string
		want        string
	}{
		{"empty string", "", "", reasonEmpty},
		{"whitespace only", "   \n\t", "", reasonEmpty},
		{"zero-width only", "​​", "", reasonEmpty},
		{"over 300 runes after NFC", strings.Repeat("好", 301), "", reasonLength},

		{"plain comment allowed", "新品上架，歡迎選購！", "", ""},
		{"short digit run allowed", "滿1000送好禮", "", ""},
		{"seven digits allowed", "訂單編號1234567", "", ""},
		{"online is not line", "online 購物超方便", "", ""},
		{"timeline is not line", "timeline 活動回顧", "", ""},
		{"product placeholder allowed", "推薦商品：{{product.name}} {{variant}}，關鍵字「{{keyword}}」，直播價 {{live_price}}", "", ""},
		{"keyword placeholder allowed", "{{keyword}}", "", ""},

		{"https url", "看這裡 https://example.com/a", "", reasonURL},
		{"http url", "看這裡 http://example.com/a", "", reasonURL},
		{"fullwidth url", "ｈｔｔｐ：／／ｅｘａｍｐｌｅ．ｃｏｍ", "", reasonURL},
		{"www", "www.example.com 好康", "", reasonDomain},
		{"fullwidth www", "ｗｗｗ．ａｂｃ．ｃｏｍ", "", reasonDomain},
		{"bare domain", "到 example.com 看", "", reasonDomain},
		{"spaced domain", "example . com 好康", "", reasonDomain},
		{"t dot me", "加我 t.me/join", "", reasonDomain},
		{"wa dot me", "加我 wa.me/12345", "", reasonDomain},

		{"lineid contiguous", "我的lineid是abc", "", reasonLineID},
		{"line id spaced", "加我 line id", "", reasonLineID},
		{"l i n e spaced letters", "l i n e", "", reasonLineID},
		{"line bare token", "加好友 line 洽詢", "", reasonLineID},

		{"email", "聯絡 email@example.com", "", reasonEmail},
		{"at handle", "找我 @merchant", "", reasonHandle},
		{"at handle zero-width", "找我 @​merchant", "", reasonHandle},

		{"phone 8 digits", "0912345678", "", reasonPhone},
		{"fullwidth phone", "０９１２３４５６７８", "", reasonPhone},
		{"dashed phone", "0912-345-678", "", reasonPhone},
		{"spaced phone", "0912 345 678", "", reasonPhone},

		{"order variable", "您的 {{order.id}} 已建立", "", reasonBuyerVar},
		{"spaced buyer variable", "{{ buyer . name }}", "", reasonBuyerVar},
		{"link variable", "點 {{link.url}} 看", "", reasonBuyerVar},
		{"link placeholder", "點此 {{連結}} 付款", "", reasonLinkPlace},
		{"spaced link placeholder", "付款 連結 {{ 連結 }}", "", reasonLinkPlace},

		{"store origin", "來 mystore.xyz 逛逛", "mystore.xyz", reasonStoreOrigin},
		{"store origin not flagged when absent", "mystore.xyz", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidatePublicSafe(c.text, c.storeOrigin); got != c.want {
				t.Fatalf("ValidatePublicSafe(%q, %q) = %q, want %q", c.text, c.storeOrigin, got, c.want)
			}
		})
	}
}

// TestFixedTemplateBodies pins the two seeded fixed templates to their §3.5/public-safe contract: offer-recommend/v1
// is public-safe (recommend), while order-pay-link/v1 carries the {{連結}} placeholder and is DM-only, so it must NOT
// pass the public-safe rule.
func TestFixedTemplateBodies(t *testing.T) {
	if got := ValidatePublicSafe("推薦商品：{{product.name}} {{variant}}，關鍵字「{{keyword}}」，直播價 {{live_price}}", ""); got != "" {
		t.Fatalf("offer-recommend/v1 body: want safe, got %q", got)
	}
	if got := ValidatePublicSafe("您的訂單已建立，請點此連結完成付款：{{連結}}", ""); got != reasonLinkPlace {
		t.Fatalf("order-pay-link/v1 body: want %q, got %q", reasonLinkPlace, got)
	}
}
