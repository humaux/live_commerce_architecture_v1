// customer_tags_test.go is the DB-free test of the W6-01B tag/note routes: it builds the FULL router (NewHandler, so a
// pattern conflict with any other route panics here, before PostgreSQL is needed), the transport rules that hold before
// any database work, the ?tag= list grammar and the error-code table. Real-PG outcomes are CT01-CT09 in
// tests/foundation/customer_tags_test.go.

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"livecommerce/internal/customers"
	"livecommerce/internal/httperror"
)

const (
	tagID      = "55555555-5555-4555-8555-555555555555"
	noteID     = "66666666-6666-4666-8666-666666666666"
	tagRev     = "0000000000000000000000000000000000000000000000000000000000000000"
	keyHeader  = "Idempotency-Key"
	keyValue   = "tag-key-0001"
	customerNs = custBase + "/" + custID + "/notes"
)

func TestCustomerTagRoutesTransportRules(t *testing.T) {
	h := NewHandler(nil) // the whole router: route conflicts panic at registration
	withKey := func(r *http.Request) { r.Header.Set(keyHeader, keyValue) }
	noKey := func(r *http.Request) { r.Header.Del(keyHeader) }
	tags, tag := custBase+"/tags", custBase+"/tags/"+tagID
	owned, note := custBase+"/"+custID+"/tags", customerNs+"/"+noteID
	for _, tc := range []struct {
		name, method, path, body string
		edit                     func(*http.Request)
		status                   int
		code                     string
	}{
		// Admitted requests reach platform.WithScope (nil pool -> 401 unauthorized).
		{"list tags", "GET", tags, "", nil, 401, "unauthorized"},
		{"create tag", "POST", tags, `{"name":"VIP","color":"red"}`, nil, 401, "unauthorized"},
		{"patch tag", "PATCH", tag, `{"name":"Regular"}`, withKey, 401, "unauthorized"},
		{"delete tag", "DELETE", tag, "", withKey, 401, "unauthorized"},
		{"set tags", "PUT", owned, `{"tag_ids":["` + tagID + `"],"revision":"` + tagRev + `"}`, withKey, 401, "unauthorized"},
		{"list notes", "GET", customerNs + "?limit=5", "", nil, 401, "unauthorized"},
		{"add note", "POST", customerNs, `{"body":"only 7-11"}`, nil, 401, "unauthorized"},
		{"edit note", "PATCH", note, `{"body":"x","version":1}`, withKey, 401, "unauthorized"},
		{"delete note", "DELETE", note, "", withKey, 401, "unauthorized"},
		{"list by tag", "GET", custBase + "?tag=" + tagID, "", nil, 401, "unauthorized"},
		// Idempotency-Key: exactly one on every write, none on reads.
		{"create tag no key", "POST", tags, `{"name":"VIP","color":"red"}`, noKey, 422, "invalid_request"},
		{"patch tag no key", "PATCH", tag, `{"name":"x"}`, nil, 422, "invalid_request"},
		{"delete tag no key", "DELETE", tag, "", nil, 422, "invalid_request"},
		{"set tags no key", "PUT", owned, `{"tag_ids":[],"revision":"` + tagRev + `"}`, nil, 422, "invalid_request"},
		{"add note short key", "POST", customerNs, `{"body":"x"}`, func(r *http.Request) { r.Header.Set(keyHeader, "short") }, 422, "invalid_request"},
		{"list tags with key", "GET", tags, "", withKey, 422, "invalid_request"},
		// Ids, query, methods.
		{"bad tag id", "DELETE", custBase + "/tags/NOT-A-UUID", "", withKey, 422, "invalid_request"},
		{"bad note id", "DELETE", customerNs + "/NOT-A-UUID", "", withKey, 422, "invalid_request"},
		{"bad customer id on notes", "GET", custBase + "/NOT-A-UUID/notes", "", nil, 422, "invalid_request"},
		{"tags query", "GET", tags + "?x=1", "", nil, 422, "invalid_request"},
		{"notes q rejected", "GET", customerNs + "?q=Alice", "", nil, 422, "invalid_request"},
		{"notes tag rejected", "GET", customerNs + "?tag=" + tagID, "", nil, 422, "invalid_request"},
		{"bad tag filter", "GET", custBase + "?tag=nope", "", nil, 422, "invalid_request"},
		{"put tags collection", "PUT", tags, `{}`, withKey, 405, ""},
		{"get tag item", "GET", tag, "", nil, 405, ""},
		{"post owner tags", "POST", owned, `{}`, nil, 405, ""},
		// Strict bodies.
		{"create unknown key", "POST", tags, `{"name":"VIP","color":"red","x":1}`, nil, 400, "invalid_json"},
		{"create missing color", "POST", tags, `{"name":"VIP"}`, nil, 422, "invalid_request"},
		{"create null name", "POST", tags, `{"name":null,"color":"red"}`, nil, 400, "invalid_json"},
		{"create bad color", "POST", tags, `{"name":"VIP","color":"pink"}`, nil, 401, "unauthorized"}, // Go validation runs inside the scope
		{"set tags missing revision", "PUT", owned, `{"tag_ids":[]}`, withKey, 422, "invalid_request"},
		{"add note missing body", "POST", customerNs, `{}`, nil, 422, "invalid_request"},
		{"edit note missing version", "PATCH", note, `{"body":"x"}`, withKey, 422, "invalid_request"},
		{"delete note with body", "DELETE", note, `{}`, withKey, 422, "invalid_request"},
		{"delete tag with body", "DELETE", tag, `{}`, withKey, 422, "invalid_request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := customerCall(h, tc.method, tc.path, tc.body, tc.edit)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
			if tc.code != "" {
				var envelope httperror.Envelope
				if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Code != tc.code {
					t.Fatalf("code=%q want=%q", envelope.Code, tc.code)
				}
			}
			if tc.status != 405 && w.Header().Get("Cache-Control") != "no-store, private" {
				t.Fatalf("Cache-Control=%q", w.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestParseCustomersQueryTag(t *testing.T) {
	in, err := parseCustomersQuery(&url.URL{RawQuery: "tag=" + tagID + "&limit=5"})
	if err != nil || in.Tag != tagID || in.Page.Limit != 5 {
		t.Fatalf("%+v %v", in, err)
	}
	for _, bad := range []string{"tag=", "tag=x", "tag=AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", "tag=" + tagID + "&tag=" + tagID} {
		if _, err := parseCustomersQuery(&url.URL{RawQuery: bad}); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	if _, err := parseNotesQuery(&url.URL{RawQuery: "limit=3&after=abc"}); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerTagErrorCodesAreInTheTable(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{customers.ErrTagExists, 409, "tag_exists"},
		{customers.ErrLimitReached, 409, "limit_reached"},
		{customers.ErrVersionChanged, 409, "version_changed"},
	} {
		status, code := customersClassify(tc.err)
		if status != tc.status || code != tc.code {
			t.Fatalf("%v: %d %s want %d %s", tc.err, status, code, tc.status, tc.code)
		}
		// A code missing from httperror's message table is rewritten to "internal": the table must know it.
		rec := httptest.NewRecorder()
		respondError(rec, status, code)
		var envelope httperror.Envelope
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil || envelope.Code != tc.code || envelope.Message == "" {
			t.Fatalf("%s is not in the httperror table: %+v", tc.code, envelope)
		}
	}
}
