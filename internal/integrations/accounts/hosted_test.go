package accounts

import (
	"bytes"
	"testing"
)

func TestHostedConfigCanonicalDigest(t *testing.T) {
	first, digest, err := (HostedConfig{ReturnURL: "https://PAY.example.com:443/return",
		NotifyURL: "https://pay.example.com/notify"}).CanonicalDigest()
	if err != nil || first.ReturnURL != "https://pay.example.com/return" {
		t.Fatalf("canonical endpoints rejected: %+v, %v", first, err)
	}
	_, same, err := (HostedConfig{ReturnURL: "https://pay.example.com/return",
		NotifyURL: "https://pay.example.com/notify"}).CanonicalDigest()
	if err != nil || !bytes.Equal(digest[:], same[:]) {
		t.Fatal("equivalent endpoints changed the request digest")
	}
	_, changed, err := (HostedConfig{ReturnURL: "https://pay.example.com/return-v2",
		NotifyURL: "https://pay.example.com/notify"}).CanonicalDigest()
	if err != nil || bytes.Equal(digest[:], changed[:]) {
		t.Fatal("changed endpoint reused the request digest")
	}
}

func TestHostedConfigRejectsUnsafeEndpoints(t *testing.T) {
	for _, bad := range []string{"", "http://pay.example.com/return", "https://pay.example.com:8443/return",
		"https://user@pay.example.com/return", "https://pay.example.com/return?x=1",
		"https://pay.example.com/return#x", "https://127.0.0.1/return",
		"https://localhost/return", "https://pay.example.com/a/../return",
		"https://pay.example.com:/return", "https://pay.example.com/return%2Fextra"} {
		if _, _, err := (HostedConfig{ReturnURL: bad, NotifyURL: "https://pay.example.com/notify"}).CanonicalDigest(); err == nil {
			t.Fatalf("unsafe callback accepted: %q", bad)
		}
	}
}
