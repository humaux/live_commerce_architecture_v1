package checkout

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestAdSignalsIndependentAndPrivate(t *testing.T) {
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, s := range []string{
		`{"fbc":"fb.1.1800000000000.synthetic","fbp":"fb.1.1800000000000.7654"}`,
		`{"fbc":null,"fbp":"fb.1.1800000000000.7654"}`,
		`{"fbc":"fb.1.1800000000000.synthetic","fbp":null}`,
	} {
		got := ParseAdSignals(encode(s))
		if got == nil {
			t.Fatalf("independent signals rejected: %s", s)
		}
		raw, err := json.Marshal(Input{AdSignals: got})
		if err != nil || strings.Contains(string(raw), "fb.") || strings.Contains(string(raw), "AdSignals") {
			t.Fatalf("private signals entered public command/digest: %s %v", raw, err)
		}
	}
	for _, s := range []string{
		`null`, `{}`, `{"fbc":null,"fbp":null}`, `[]`,
		`{"fbc":"forged","fbp":null}`, `{"fbp":"fb.1.1.bad"}`,
		`{"fbc":"fb.1.1.synthetic","draft_id":"forged"}`,
		`{"fbp":"fb.1.1.1"} {}`, `{"fbp":1}`,
	} {
		if ParseAdSignals(encode(s)) != nil {
			t.Fatalf("invalid optional signals accepted: %s", s)
		}
	}
	for _, s := range []string{"", "!", strings.Repeat("a", 2049), encode(`{"fbp":"fb.1.1.1"}`) + "\n"} {
		if ParseAdSignals(s) != nil {
			t.Fatal("malformed wire signals accepted")
		}
	}
}
