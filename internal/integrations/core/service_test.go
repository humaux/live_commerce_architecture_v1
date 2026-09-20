package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"livecommerce/internal/command"
)

// Keep the parser/credential-output guard executable without a PostgreSQL fixture.
func TestCanonicalObjectAndLeaseOutput(t *testing.T) {
	_, a, err := canonicalObject(json.RawMessage(`{"b":2,"a":9007199254740993}`))
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := canonicalObject(json.RawMessage(` {"a":9007199254740993,"b":2} `))
	if err != nil || string(a) != string(b) || !strings.Contains(string(a), "9007199254740993") {
		t.Fatalf("canonical precision: %s / %s / %v", a, b, err)
	}
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`[]`), json.RawMessage(`{} {}`), {'{', '"', 'v', '"', ':', '"', 0xff, '"', '}'}} {
		if _, _, err := canonicalObject(raw); !errors.Is(err, command.ErrInvalid) {
			t.Fatalf("invalid object accepted, bytes=%d err=%v", len(raw), err)
		}
	}
	encoded, err := json.Marshal(ClaimResult{Disposition: "claimed", Generation: 1, Mode: "dispatch", LeaseToken: []byte("must-not-serialize")})
	if err != nil || strings.Contains(string(encoded), "Token") || strings.Contains(string(encoded), "must-not") || strings.Contains(string(encoded), "bXVzdC") {
		t.Fatalf("lease credential serialized: %s %v", encoded, err)
	}
}
