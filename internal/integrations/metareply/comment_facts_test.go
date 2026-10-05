// Purpose: DB-free table test of the platform-branched comment-facts derivation (Amendment 1 A1.2 1a): the
// field sets asked of Graph and the created_at / is_page / is_reply / author-missing rules.
// Depends on: comment_poll.go (factsFields, parseCommentFacts).
// Used by: go test ./internal/integrations/metareply.
// Status: MOCK (IG field set pending probe R3 / LC-U12).
package metareply

import (
	"testing"
	"time"
)

func TestCommentFactsFieldsAndDerivation(t *testing.T) {
	if got := factsFields("page"); got != "created_time,from{id},parent{id}" {
		t.Fatalf("FB fields %q", got)
	}
	if got := factsFields("instagram"); got != "timestamp,from{id},parent_id" {
		t.Fatalf("IG fields %q", got)
	}
	const asset, ref = "1789", "1790_55"
	want := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, object, body string
		ok, page, reply    bool
	}{
		{"fb buyer", "page", `{"id":"1790_55","created_time":"2026-10-05T12:00:00+0000","from":{"id":"42"}}`, true, false, false},
		{"fb page reply", "page", `{"id":"1790_55","created_time":"2026-10-05T12:00:00+0000","from":{"id":"1789"},"parent":{"id":"1790_1"}}`, true, true, true},
		{"ig buyer", "instagram", `{"id":"1790_55","timestamp":"2026-10-05T12:00:00+0000","from":{"id":"42"}}`, true, false, false},
		{"ig page", "instagram", `{"id":"1790_55","timestamp":"2026-10-05T12:00:00+0000","from":{"id":"1789"}}`, true, true, false},
		{"ig reply", "instagram", `{"id":"1790_55","timestamp":"2026-10-05T12:00:00+0000","from":{"id":"42"},"parent_id":"1790_1"}`, true, false, true},
		{"ig uses timestamp not created_time", "instagram", `{"id":"1790_55","created_time":"2026-10-05T12:00:00+0000","from":{"id":"42"}}`, false, false, false},
		{"author missing is not found", "instagram", `{"id":"1790_55","timestamp":"2026-10-05T12:00:00+0000"}`, false, false, false},
		{"other id", "page", `{"id":"1790_99","created_time":"2026-10-05T12:00:00+0000","from":{"id":"42"}}`, false, false, false},
		{"bad time", "page", `{"id":"1790_55","created_time":"yesterday","from":{"id":"42"}}`, false, false, false},
		{"data page is not a single comment", "page", `{"data":[{"id":"1790_55"}]}`, false, false, false},
	} {
		f, ok := parseCommentFacts([]byte(tc.body), tc.object, asset, ref)
		if ok != tc.ok {
			t.Fatalf("%s: ok=%v want %v", tc.name, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if !f.Found || f.IsPage != tc.page || f.IsReply != tc.reply || f.CreatedAt == nil || !f.CreatedAt.Equal(want) {
			t.Fatalf("%s: %+v", tc.name, f)
		}
	}
}
