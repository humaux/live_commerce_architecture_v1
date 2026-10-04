package identityhttp

// Transport tests for /v1/identity/staff/* with a fake service (no database, no mail): rejection order (BFF key, browser
// headers, query, bearer, body), the error mapping and that rejected requests never reach the service.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"livecommerce/internal/identity"
)

type fakeStaff struct {
	calls              int
	err                error
	bearer, store, arg string
}

func (f *fakeStaff) hit(b, s, a string) { f.calls++; f.bearer, f.store, f.arg = b, s, a }
func (f *fakeStaff) List(_ context.Context, b, s string) (identity.StaffTeam, error) {
	f.hit(b, s, "")
	return identity.StaffTeam{Members: []identity.StaffMember{}, Invitations: []identity.StaffInvitation{}}, f.err
}
func (f *fakeStaff) Invite(_ context.Context, b, s, email, role, locale string) (identity.StaffInvite, error) {
	f.hit(b, s, email+"|"+role+"|"+locale)
	return identity.StaffInvite{ID: "i", ExpiresAt: time.Unix(0, 0), MailState: "SENT"}, f.err
}
func (f *fakeStaff) RevokeInvite(_ context.Context, b, s, i string) error {
	f.hit(b, s, i)
	return f.err
}
func (f *fakeStaff) SetRole(_ context.Context, b, s, p, r string) error {
	f.hit(b, s, p+"|"+r)
	return f.err
}
func (f *fakeStaff) Remove(_ context.Context, b, s, p string) error { f.hit(b, s, p); return f.err }
func (f *fakeStaff) Accept(_ context.Context, b, tok string) (identity.StaffJoined, error) {
	f.hit(b, "", tok)
	return identity.StaffJoined{StoreID: "s", Role: "viewer"}, f.err
}

func staffRequest(t *testing.T, f *fakeStaff, path, bodyText string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	h, err := NewStaffHandler(f, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(bodyText))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Commerce-BFF-Key", testSecret)
	r.Header.Set("Authorization", "Bearer "+testSecret)
	if mutate != nil {
		mutate(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const sid = "11111111-1111-4111-8111-111111111111"

func TestStaffHTTPRoutesReachService(t *testing.T) {
	f := &fakeStaff{}
	cases := []struct {
		path, body, arg string
		code            int
	}{
		{"list", `{"store_id":"` + sid + `"}`, "", 200},
		{"invite", `{"store_id":"` + sid + `","email":"a@example.test","role":"viewer","locale":"en"}`, "a@example.test|viewer|en", 201},
		{"revoke-invite", `{"store_id":"` + sid + `","invite_id":"` + sid + `"}`, sid, 204},
		{"set-role", `{"store_id":"` + sid + `","principal_id":"` + sid + `","role":"admin"}`, sid + "|admin", 204},
		{"remove", `{"store_id":"` + sid + `","principal_id":"` + sid + `"}`, sid, 204},
		{"accept", `{"token":"abc"}`, "abc", 200},
	}
	for _, c := range cases {
		w := staffRequest(t, f, "/v1/identity/staff/"+c.path, c.body, nil)
		if w.Code != c.code || f.arg != c.arg || f.bearer != testSecret {
			t.Fatalf("%s = %d arg=%q bearer ok=%v body=%s", c.path, w.Code, f.arg, f.bearer == testSecret, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s lacks no-store", c.path)
		}
	}
}

func TestStaffHTTPRejectsBeforeService(t *testing.T) {
	body := `{"store_id":"` + sid + `"}`
	for name, tc := range map[string]struct {
		mutate func(*http.Request)
		path   string
		body   string
		code   int
	}{
		"no bff key":     {func(r *http.Request) { r.Header.Del("X-Commerce-BFF-Key") }, "list", body, 401},
		"wrong bff key":  {func(r *http.Request) { r.Header.Set("X-Commerce-BFF-Key", strings.Repeat("B", 43)) }, "list", body, 401},
		"origin":         {func(r *http.Request) { r.Header.Set("Origin", "https://x.test") }, "list", body, 403},
		"cookie":         {func(r *http.Request) { r.Header.Set("Cookie", "a=b") }, "list", body, 403},
		"query":          {func(r *http.Request) { r.URL.RawQuery = "x=1" }, "list", body, 422},
		"no bearer":      {func(r *http.Request) { r.Header.Del("Authorization") }, "list", body, 401},
		"short bearer":   {func(r *http.Request) { r.Header.Set("Authorization", "Bearer abc") }, "list", body, 401},
		"unknown field":  {nil, "list", `{"store_id":"` + sid + `","tenant_id":"x"}`, 400},
		"not json":       {func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, "list", body, 415},
		"unknown route":  {nil, "promote", body, 404},
		"get not post":   {func(r *http.Request) { r.Method = http.MethodGet }, "list", body, 405},
		"trailing value": {nil, "list", body + `{}`, 400},
	} {
		f := &fakeStaff{}
		w := staffRequest(t, f, "/v1/identity/staff/"+tc.path, tc.body, tc.mutate)
		if w.Code != tc.code || f.calls != 0 {
			t.Errorf("%s: status %d (want %d) calls %d", name, w.Code, tc.code, f.calls)
		}
	}
}

func TestStaffHTTPErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
		name string
	}{
		{identity.ErrForbidden, 403, "forbidden"}, {identity.ErrNotFound, 404, "not_found"}, {identity.ErrInviteInvalid, 404, "invite_invalid"},
		{identity.ErrAlreadyMember, 409, "already_member"}, {identity.ErrLastOwner, 409, "last_owner"},
		{identity.ErrTooManyInvitations, 429, "too_many_invitations"}, {identity.ErrInvalidEmail, 422, "invalid_email"},
		{identity.ErrInvalid, 422, "invalid_request"}, {identity.ErrUnauthorized, 401, "unauthorized"}, {identity.ErrUnavailable, 503, "unavailable"},
	} {
		w := staffRequest(t, &fakeStaff{err: tc.err}, "/v1/identity/staff/accept", `{"token":"abc"}`, nil)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), `"code":"`+tc.name+`"`) {
			t.Errorf("%v => %d %s", tc.err, w.Code, w.Body.String())
		}
	}
}
