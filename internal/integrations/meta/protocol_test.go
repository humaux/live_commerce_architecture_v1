package meta

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSecret = "1234567890123456"

func verifier(t *testing.T) *Verifier {
	t.Helper()
	v, err := NewVerifier(Config{AppID: "123", Object: "page", AppSecret: testSecret, VerifyToken: "abcdefghijklmnop"})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func signed(raw []byte) string {
	m := hmac.New(sha256.New, []byte(testSecret))
	_, _ = m.Write(raw)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func TestVerifySignatureStrictJSONAndIdentity(t *testing.T) {
	v := verifier(t)
	first := []byte(`{"object":"page","entry":[{"id":"9","time":123,"messaging":[{"sender":{"id":"4"},"recipient":{"id":"9"},"message":{"mid":"m.1","text":"hi"}}]}]}`)
	second := []byte(`{ "entry" : [ {"messaging":[{"message":{"text":"hi","mid":"m.1"},"recipient":{"id":"9"},"sender":{"id":"4"}}],"time":999,"id":"9"}], "object":"page" }`)
	b1, err := v.Verify(first, signed(first))
	if err != nil {
		t.Fatal(err)
	}
	b2, err := v.Verify(second, signed(second))
	if err != nil {
		t.Fatal(err)
	}
	if len(b1.Events) != 1 || len(b2.Events) != 1 || b1.Events[0].Key != b2.Events[0].Key || b1.Events[0].PayloadHash != b2.Events[0].PayloadHash || b1.BodyHash == b2.BodyHash {
		t.Fatal("rebatching/key order identity or raw digest")
	}
	changed := bytes.Replace(first, []byte(`"hi"`), []byte(`"bye"`), 1)
	if _, err := v.Verify(changed, signed(first)); !errors.Is(err, ErrSignature) {
		t.Fatalf("stale signature: %v", err)
	}
	b3, err := v.Verify(changed, signed(changed))
	if err != nil {
		t.Fatal(err)
	}
	if b3.Events[0].Key != b1.Events[0].Key || b3.Events[0].PayloadHash == b1.Events[0].PayloadHash {
		t.Fatal("changed MID payload conflict identity")
	}
	for _, raw := range [][]byte{
		[]byte(`{"object":"page","obj\u0065ct":"page"}`),
		[]byte(`{"object":"page","entry":[{"id":"9","id":"8"}]}`),
		[]byte(`{"object":"page"} true`),
		[]byte(`[]`),
		{0xff},
	} {
		if _, err := v.Verify(raw, signed(raw)); !errors.Is(err, ErrJSON) {
			t.Fatalf("accepted malformed %q: %v", raw, err)
		}
	}
	depth64 := []byte(`{"x":` + strings.Repeat(`[`, 63) + `0` + strings.Repeat(`]`, 63) + `}`)
	depth65 := []byte(`{"x":` + strings.Repeat(`[`, 64) + `0` + strings.Repeat(`]`, 64) + `}`)
	if _, err := v.Verify(depth64, signed(depth64)); err != nil {
		t.Fatalf("depth 64: %v", err)
	}
	if _, err := v.Verify(depth65, signed(depth65)); !errors.Is(err, ErrJSON) {
		t.Fatalf("depth 65: %v", err)
	}
}

func TestStrictSurrogateEscapesPreserveJSONIdentity(t *testing.T) {
	v := verifier(t)
	invalid := map[string]string{
		"root member":     `{"\ud800":1,"object":"page"}`,
		"nested member":   `{"object":"page","entry":[{"id":"9","\udc00":1}]}`,
		"nested value":    `{"object":"page","entry":[{"id":"9","changes":[{"field":"feed","value":{"item":"comment","verb":"add","comment_id":"a","text":"\ud800"}}]}]}`,
		"two highs":       `{"object":"page","entry":[],"text":"\ud800\ud801"}`,
		"lone low":        `{"object":"page","entry":[],"text":"\udc00"}`,
		"high then plain": `{"object":"page","entry":[],"text":"\ud800x"}`,
		"malformed hex":   `{"object":"page","entry":[],"text":"\uGGGG"}`,
	}
	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			raw := []byte(source)
			if _, err := v.Verify(raw, signed(raw)); !errors.Is(err, ErrJSON) {
				t.Fatalf("accepted malformed escape: %v", err)
			}
		})
	}
	valid := []string{
		`{"object":"page","entry":[],"text":"\uD83D\uDE00"}`,
		`{"object":"page","entry":[],"text":"😀"}`,
		`{"object":"page","entry":[],"text":"\\uD800"}`,
		`{"object":"page","entry":[],"text":"\uFFFD"}`,
		`{"object":"page","entry":[],"text":"�"}`,
	}
	var hashes []string
	for _, source := range valid {
		raw := []byte(source)
		b, err := v.Verify(raw, signed(raw))
		if err != nil || len(b.Events) != 2 {
			t.Fatalf("rejected valid %q: %v", source, err)
		}
		hashes = append(hashes, b.Events[0].PayloadHash)
	}
	if hashes[0] != hashes[1] || hashes[3] != hashes[4] || hashes[0] == hashes[2] {
		t.Fatal("canonical Unicode identity changed")
	}
}

func TestQuarantineAccountingAndBounds(t *testing.T) {
	v := verifier(t)
	raw := []byte(`{"object":"page","standby":{},"entry":[{"id":"9","extra":true,"changes":[{"field":"feed","value":{"item":"comment","verb":"add","comment_id":"c1","created_time":300}}],"messaging":[{"sender":{"id":"9"},"recipient":{"id":"9"},"message":{"mid":"echo","is_echo":true}}]},{"id":"10"}]}`)
	b, err := v.Verify(raw, signed(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Events) != 5 {
		t.Fatalf("expected 5 accounted events, got %d", len(b.Events))
	}
	if b.Events[2].Kind != "page_comment_add" || b.Events[2].ExternalID != "c1" || b.Events[2].OccurredAt == nil || b.Events[2].OccurredAt.Unix() != 300 {
		t.Fatalf("comment: %v", b.Events[2])
	}
	if b.Events[3].QuarantineReason == "" || b.Events[4].QuarantineReason == "" {
		t.Fatal("lost quarantine")
	}
	// A full-size adversarial entry must retain O(input) bytes across records.
	var builder strings.Builder
	builder.WriteString(`{"object":"page","entry":[{"id":"9","time":1,"messaging":[`)
	for i := 0; i < 1000; i++ {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{"unknown":"x"}`)
	}
	builder.WriteString(`]}]}`)
	raw = []byte(builder.String())
	b, err = v.Verify(raw, signed(raw))
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, event := range b.Events {
		total += len(event.Payload)
	}
	if len(b.Events) != 1000 || total > 10<<20 {
		t.Fatalf("amplified quarantine: events=%d bytes=%d", len(b.Events), total)
	}
	// Malformed entry metadata is retained once, not copied into each unit.
	malformedTime := []byte(strings.Replace(builder.String(), `"time":1`, `"time":"`+strings.Repeat("q", 200000)+`"`, 1))
	malformedTime = []byte(strings.Replace(string(malformedTime), `{"unknown":"x"},`, ``, 1))
	b, err = v.Verify(malformedTime, signed(malformedTime))
	if err != nil {
		t.Fatal(err)
	}
	total = 0
	for _, event := range b.Events {
		total += len(event.Payload)
	}
	if len(b.Events) != 1000 || b.Events[0].QuarantineReason != "invalid_entry_time" || total > 10<<20 {
		t.Fatalf("malformed time amplified: events=%d bytes=%d", len(b.Events), total)
	}
	raw = []byte(strings.Replace(builder.String(), `{"unknown":"x"}`, `{"unknown":"x"},{"unknown":"y"}`, 1))
	if _, err := v.Verify(raw, signed(raw)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("1001 events: %v", err)
	}
	raw = []byte(`{"object":"page","entry":[{"id":"9","messaging":[{"sender":{"id":"4"},"recipient":{"id":"9"},"message":{"mid":"m1","is_echo":null}}]}]}`)
	b, err = v.Verify(raw, signed(raw))
	if err != nil || b.Events[0].QuarantineReason == "" {
		t.Fatalf("null echo accepted: %v", err)
	}
}

func TestRedactionAndConfig(t *testing.T) {
	if _, err := (&Verifier{}).Verify([]byte(`{}`), signed([]byte(`{}`))); !errors.Is(err, ErrConfig) {
		t.Fatal("zero verifier admitted")
	}
	if _, err := NewHandler(&Verifier{}, func(context.Context, Batch) error { return nil }); !errors.Is(err, ErrConfig) {
		t.Fatal("zero verifier handler admitted")
	}
	if _, err := NewVerifier(Config{AppID: "１２", Object: "page", AppSecret: testSecret, VerifyToken: testSecret}); !errors.Is(err, ErrConfig) {
		t.Fatal("unicode digits accepted")
	}
	if _, err := NewVerifier(Config{AppID: "1", Object: "page", AppSecret: testSecret + "\n", VerifyToken: testSecret}); !errors.Is(err, ErrConfig) {
		t.Fatal("control accepted")
	}
	v := verifier(t)
	raw := []byte(`{"object":"page","entry":[{"id":"9","changes":[{"field":"feed","value":{"item":"comment","verb":"add","comment_id":"private"}}]}]}`)
	b, err := v.Verify(raw, signed(raw))
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []any{Config{AppSecret: testSecret}, v, b, b.Events[0]} {
		for _, got := range []string{fmt.Sprint(x), fmt.Sprintf("%#v", x), string(mustJSON(t, x))} {
			if strings.Contains(got, "private") || strings.Contains(got, testSecret) {
				t.Fatalf("secret in formatting: %s", got)
			}
		}
	}
	raw[0] = 'x'
	if !bytes.Contains(b.Events[0].Payload, []byte("private")) {
		t.Fatal("payload aliases raw input")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHandlerCommitGate(t *testing.T) {
	v := verifier(t)
	called := 0
	fail := false
	h, err := NewHandler(v, func(_ context.Context, b Batch) error {
		called++
		if len(b.Events) != 1 {
			t.Fatal("partial batch")
		}
		if fail {
			return errors.New("private failure")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"object":"page","entry":[{"id":"9","changes":[{"field":"feed","value":{"item":"comment","verb":"add","comment_id":"a"}}]}]}`)
	post := func(path string, body []byte, sig string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json; charset=UTF-8")
		r.Header.Set("X-Hub-Signature-256", sig)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := post("/webhook?", raw, signed(raw)); w.Code != 400 || called != 0 {
		t.Fatal("bare query accepted")
	}
	if w := post("/webhook", raw, signed([]byte(`{}`))); w.Code != 403 || called != 0 {
		t.Fatal("bad signature admitted")
	}
	invalidMedia := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(raw))
	invalidMedia.Header.Set("Content-Type", "application/json; foo=bar")
	invalidMedia.Header.Set("X-Hub-Signature-256", signed(raw))
	invalidMediaResponse := httptest.NewRecorder()
	h.ServeHTTP(invalidMediaResponse, invalidMedia)
	if invalidMediaResponse.Code != 415 || called != 0 {
		t.Fatal("unsupported media parameter admitted")
	}
	if w := post("/webhook", raw, signed(raw)); w.Code != 200 || w.Body.String() != "EVENT_RECEIVED" || called != 1 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("commit gate: %d %q", w.Code, w.Body.String())
	}
	fail = true
	if w := post("/webhook", raw, signed(raw)); w.Code != 503 || called != 2 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("commit failure leaked or acknowledged")
	}
	q := "/webhook?hub.mode=subscribe&hub.verify_token=abcdefghijklmnop&hub.challenge=abc-9"
	r := httptest.NewRequest(http.MethodGet, q, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != "abc-9" {
		t.Fatalf("challenge: %d %q", w.Code, w.Body.String())
	}
	if w := post("/webhook", bytes.Repeat([]byte("x"), maxBody+1), signed(raw)); w.Code != 413 || called != 2 {
		t.Fatal("oversize admitted")
	}
	_ = fmt.Sprint(v)
}
