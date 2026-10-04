package checkout

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"regexp"
	"time"

	"livecommerce/internal/command"
)

// AdTouch is trusted only after the private BFF authenticates its host-signed cookie.
// The SQL writer independently verifies store ownership and the seven-day window.
type AdTouch struct {
	DraftID   string    `json:"draft_id"`
	ClickedAt time.Time `json:"clicked_at"`
	FBC       *string   `json:"fbc"`
	FBP       string    `json:"fbp"`
}

// AdSignals are separately authenticated by the BFF's host-bound, 90-day cookies.
// They improve consented CAPI matching, but never establish an attribution path.
type AdSignals struct {
	FBC *string `json:"fbc"`
	FBP *string `json:"fbp"`
}

// ParseAdSignals discards malformed optional measurement without denying checkout.
func ParseAdSignals(raw string) *AdSignals {
	if len(raw) == 0 || len(raw) > 2048 {
		return nil
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(b) != raw {
		return nil
	}
	var s AdSignals
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&s) != nil || d.Decode(new(any)) != io.EOF || (s.FBC == nil && s.FBP == nil) ||
		(s.FBC != nil && !metaClickID.MatchString(*s.FBC)) || (s.FBP != nil && !metaBrowserID.MatchString(*s.FBP)) {
		return nil
	}
	return &s
}

var metaClickID = regexp.MustCompile(`^fb\.1\.[1-9][0-9]{0,15}\.[A-Za-z0-9_-]{1,500}$`)
var metaBrowserID = regexp.MustCompile(`^fb\.1\.[1-9][0-9]{0,15}\.[0-9]{1,20}$`)

// ParseAdTouch bounds and validates the authenticated BFF-only header; invalid tracking never prevents purchase.
func ParseAdTouch(raw string, now time.Time) *AdTouch {
	if len(raw) == 0 || len(raw) > 2048 {
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil
	}
	var t AdTouch
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&t) != nil || d.Decode(new(any)) != io.EOF || !command.ValidID(t.DraftID) ||
		t.ClickedAt.After(now) || t.ClickedAt.Before(now.Add(-7*24*time.Hour)) ||
		!metaBrowserID.MatchString(t.FBP) || (t.FBC != nil && !metaClickID.MatchString(*t.FBC)) {
		return nil
	}
	return &t
}

// ValidClientIP accepts one literal, never a proxy chain or an address with a port/zone.
func ValidClientIP(raw string) string {
	if net.ParseIP(raw) == nil {
		return ""
	}
	return raw
}
