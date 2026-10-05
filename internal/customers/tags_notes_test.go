package customers

// DB-free tests of the W6-01B tag/note validation, the definer error mapping and the strict projection drift checks.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"livecommerce/internal/command"
)

func TestValidTagNameAndNoteBody(t *testing.T) {
	for _, ok := range []string{"VIP", "熟客", "愛殺價", strings.Repeat("a", 20), "7-11 only"} {
		if !ValidTagName(ok) {
			t.Fatalf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", " VIP", "VIP ", strings.Repeat("a", 21), "a\nb", "a\x00b", "\xff", "a\u200bb", "\ufeffVIP", "a\u00adb"} {
		if ValidTagName(bad) {
			t.Fatalf("%q accepted", bad)
		}
	}
	for _, ok := range []string{"x", "只收 7-11\n上次少寄一件已補", strings.Repeat("字", 1000)} {
		if !ValidNoteBody(ok) {
			t.Fatalf("note %q refused", ok)
		}
	}
	for _, bad := range []string{"", " \n\t ", strings.Repeat("字", 1001), "a\x00b", "\xff"} {
		if ValidNoteBody(bad) {
			t.Fatalf("note %q accepted", bad)
		}
	}
}

func TestMapTagError(t *testing.T) {
	pg := func(code, msg string) error { return &pgconn.PgError{Code: code, Message: msg} }
	for _, tc := range []struct {
		err  error
		want error
	}{
		{pg("PT409", "tag_exists"), ErrTagExists},
		{pg("PT409", "limit_reached"), ErrLimitReached},
		{pg("PT409", "version_changed"), ErrVersionChanged},
		{pg("PT409", "idempotency_conflict"), ErrIdempotencyConflict},
		{pg("PT422", "invalid tag name"), command.ErrInvalid},
		{pg("PT403", "forbidden"), nil},
	} {
		got := mapTagError(tc.err)
		if tc.want != nil && !errors.Is(got, tc.want) {
			t.Fatalf("%v -> %v want %v", tc.err, got, tc.want)
		}
		if tc.want == nil && (got == nil || strings.Contains(got.Error(), "forbidden")) == false {
			t.Fatalf("PT403 -> %v", got)
		}
	}
}

func TestDecodeRejectsTagAndNoteDrift(t *testing.T) {
	good := func(mut func(map[string]any)) json.RawMessage { return detailJSON(t, mut) }
	if _, err := decodeDetail(good(nil)); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	for name, mut := range map[string]func(map[string]any){
		"tags null": func(m map[string]any) { m["tags"] = nil },
		"tag bad colour": func(m map[string]any) {
			m["tags"] = []map[string]string{{"id": testUUID, "name": "VIP", "color": "pink"}}
		},
		"tag duplicate": func(m map[string]any) {
			m["tags"] = []map[string]string{{"id": testUUID, "name": "A", "color": "red"}, {"id": testUUID, "name": "B", "color": "red"}}
		},
		"revision malformed": func(m map[string]any) { m["tags_revision"] = "abc" },
		"notes null":         func(m map[string]any) { m["notes"] = nil },
		"note blank": func(m map[string]any) {
			m["notes"] = []map[string]any{{"id": testUUID, "body": " ", "author_id": testUUID, "created_at": testTS, "edited_at": nil, "version": 1}}
		},
		"note extra key": func(m map[string]any) {
			m["notes"] = []map[string]any{{"id": testUUID, "body": "x", "author_id": testUUID, "created_at": testTS, "edited_at": nil, "version": 1, "owner_id": testUUID}}
		},
		"note version 0": func(m map[string]any) {
			m["notes"] = []map[string]any{{"id": testUUID, "body": "x", "author_id": testUUID, "created_at": testTS, "edited_at": nil, "version": 0}}
		},
		"missing notes key": func(m map[string]any) { delete(m, "notes") },
	} {
		if _, err := decodeDetail(good(mut)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
