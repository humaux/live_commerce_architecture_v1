// Purpose: DB-free checks of the manual-send text rules and sealing (live-console-v1 §3.3-3.5, §3.4): per-platform limits, the §3.5
// public-reply patterns (LCN08), the display-copy link scrub (LCN13), display-copy seal/open and the keyed body_hmac.
// Depends on: internal/inbox (send_text.go, keyring.go), internal/msgtemplates (via checkText).
// Used by: go test ./internal/inbox.

package inbox

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
)

func sendErr(t *testing.T, err error) *SendError {
	t.Helper()
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("want *SendError, got %v", err)
	}
	return se
}

func TestCheckTextLimitsPerPlatform(t *testing.T) {
	if _, err := checkText(KindDM, "messenger", strings.Repeat("好", 2000)); err != nil {
		t.Fatalf("2000 runes allowed on Messenger: %v", err)
	}
	if se := sendErr(t, mustErr(checkText(KindDM, "messenger", strings.Repeat("好", 2001)))); se.Code != "invalid_text" || se.Max != 2000 {
		t.Fatalf("2001 runes: %+v", se)
	}
	// Instagram counts BYTES: 333 CJK characters = 999 bytes passes, 334 = 1002 does not.
	if _, err := checkText(KindDM, "instagram", strings.Repeat("好", 333)); err != nil {
		t.Fatalf("333 CJK chars fit 1000 bytes: %v", err)
	}
	if se := sendErr(t, mustErr(checkText(KindDM, "instagram", strings.Repeat("好", 334)))); se.Max != 1000 {
		t.Fatalf("334 CJK chars: %+v", se)
	}
	if _, err := checkText(KindPublic, "facebook", strings.Repeat("好", 300)); err != nil {
		t.Fatalf("300 runes public: %v", err)
	}
	if se := sendErr(t, mustErr(checkText(KindPublic, "facebook", strings.Repeat("好", 301)))); se.Max != 300 {
		t.Fatalf("301 runes public: %+v", se)
	}
	for _, bad := range []string{"", "   ", "a\x00b", "a\u0007b", "\xff"} {
		if _, err := checkText(KindDM, "messenger", bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, err := checkText(KindDM, "messenger", "line1\nline2"); err != nil {
		t.Fatalf("newline allowed: %v", err)
	}
}

func mustErr(_ string, err error) error { return err }

// LCN08: every §3.5 pattern is rejected server side, including the full-width and split forms.
func TestPublicReplyForbiddenContent(t *testing.T) {
	for name, text := range map[string]string{
		"url":              "看 https://shop.example.com/x",
		"fullwidth www":    "ｗｗｗ．ａｂｃ．ｃｏｍ",
		"fullwidth digits": "０９１２３４５６７８",
		"dashed phone":     "0912-345-678",
		"spaced line":      "l i n e",
		"line id":          "加我 line id abc",
		"zero width":       "w​ww.ab​c.com",
		"t.me":             "t.me/abc",
		"wa.me":            "wa.me/123",
		"handle":           "@someone",
		"email":            "a@b.com",
		"pay link var":     "付款連結{{連結}}",
		"order var":        "訂單 {{order.id}}",
	} {
		_, err := checkText(KindPublic, "facebook", text)
		se := sendErr(t, err)
		if se.Code != "public_reply_forbidden_content" || se.Status != 422 || se.Reason == "" {
			t.Fatalf("%s: %+v", name, se)
		}
	}
	if _, err := checkText(KindPublic, "facebook", "謝謝支持，歡迎私訊"); err != nil {
		t.Fatalf("plain public reply rejected: %v", err)
	}
	// A DM may carry a link (§12); only public kinds run the content rule.
	if _, err := checkText(KindDM, "messenger", "付款 https://shop.example.com/pay/abc"); err != nil {
		t.Fatalf("DM link rejected: %v", err)
	}
}

// LCN13: no bearer link persists in a display copy, typed, pasted or full-width.
func TestScrubLinks(t *testing.T) {
	cases := map[string]string{
		"請點 https://shop.example.com/pay/abc 付款": "請點 {{連結}} 付款",
		"請點https://shop.example.com/pay/abc付款":   "請點{{連結}}付款",
		"ｈｔｔｐｓ：／／ｓｈｏｐ．ｅｘａｍｐｌｅ．ｃｏｍ／ｐ 好":           "{{連結}} 好",
		"www.example.com":     "{{連結}}",
		"shop.example.tw/p/1": "{{連結}}",
		"沒有連結的一句話":            "沒有連結的一句話",
		"已有 {{連結}} 佔位":        "已有 {{連結}} 佔位",
		"價格 1,200 元 v1.2":     "價格 1,200 元 v1.2",
	}
	for in, want := range cases {
		if got := scrubLinks(in); got != want {
			t.Fatalf("scrubLinks(%q) = %q, want %q", in, got, want)
		}
	}
}

func testKeyring(t *testing.T) *Keyring {
	t.Helper()
	k, err := newKeyring(testKeyID, map[string][]byte{testKeyID: bytes.Repeat([]byte{7}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestOutboundSealOpenRoundTripAndBinding(t *testing.T) {
	k := testKeyring(t)
	id := "99999999-aaaa-4bbb-8ccc-dddddddddddd"
	keyID, nonce, ct, err := k.sealOutbound(testTenant2, testStore2, id, KindDM, "你好")
	if err != nil {
		t.Fatal(err)
	}
	got, err := k.openOutbound(testTenant2, testStore2, id, KindDM, keyID, nonce, ct)
	if err != nil || got != "你好" {
		t.Fatalf("open: %q %v", got, err)
	}
	// AAD binds tenant, store, id and kind: a copied row fails to open.
	for name, args := range map[string][3]string{
		"other tenant": {testStore2, testStore2, KindDM},
		"other kind":   {testTenant2, testStore2, KindPublic},
		"other store":  {testTenant2, testTenant2, KindDM},
	} {
		if _, err := k.openOutbound(args[0], args[1], id, args[2], keyID, nonce, ct); !errors.Is(err, ErrPayload) {
			t.Fatalf("%s opened", name)
		}
	}
	if _, err := k.openOutbound(testTenant2, testStore2, "99999999-aaaa-4bbb-8ccc-eeeeeeeeeeee", KindDM, keyID, nonce, ct); !errors.Is(err, ErrPayload) {
		t.Fatal("other id opened")
	}
	// A nonce is random per seal.
	_, nonce2, _, _ := k.sealOutbound(testTenant2, testStore2, id, KindDM, "你好")
	if bytes.Equal(nonce, nonce2) {
		t.Fatal("nonce reused")
	}
}

const (
	testTenant2 = "11111111-1111-4111-8111-111111111111"
	testStore2  = "22222222-2222-4222-8222-222222222222"
)

func TestBodyHMACIsKeyedAndTenantSeparated(t *testing.T) {
	k := testKeyring(t)
	a, _ := k.bodyHMAC(testTenant2, "謝謝")
	b, _ := k.bodyHMAC(testTenant2, "謝謝")
	c, _ := k.bodyHMAC(testStore2, "謝謝")
	d, _ := k.bodyHMAC(testTenant2, "謝謝!")
	if len(a) != 32 || !bytes.Equal(a, b) || bytes.Equal(a, c) || bytes.Equal(a, d) {
		t.Fatalf("hmac not deterministic/separated: %x %x %x %x", a, b, c, d)
	}
	// Never an unsalted hash of the text.
	if bytes.Equal(a, sha256Sum("謝謝")) {
		t.Fatal("body_hmac is an unsalted sha256")
	}
}

func sha256Sum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}
