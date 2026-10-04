package metaoauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type seen struct {
	method, path, rawQuery, auth, body string
}

func fakeGraph(t *testing.T, h func(seen, http.ResponseWriter)) (*Graph, *[]seen) {
	t.Helper()
	var log []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		s := seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), string(raw)}
		log = append(log, s)
		h(s, w)
	}))
	t.Cleanup(srv.Close)
	g, err := NewGraph(srv.URL, "v99.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	return g, &log
}

// The host allowlist is the only thing keeping a client_secret away from another host.
func TestNewGraphHostAndVersionAllowlist(t *testing.T) {
	for _, base := range []string{"", GraphHost, "http://127.0.0.1:8080"} {
		if _, err := NewGraph(base, "v26.0", nil); err != nil {
			t.Errorf("base %q refused: %v", base, err)
		}
	}
	for _, base := range []string{"https://graph.facebook.com.evil.test", "http://graph.facebook.com", "https://example.com", "http://localhost:80", "http://127.0.0.1", "http://10.0.0.1:80"} {
		if _, err := NewGraph(base, "v26.0", nil); !errors.Is(err, ErrConfig) {
			t.Errorf("base %q accepted", base)
		}
	}
	for _, v := range []string{"", "26.0", "v26", "v26.0.1", "latest"} {
		if _, err := NewGraph("", v, nil); !errors.Is(err, ErrConfig) {
			t.Errorf("version %q accepted", v)
		}
	}
}

// Token placement: GET and DELETE use the Authorization header, POST the JSON body; never the URL.
func TestDoTokenPlacement(t *testing.T) {
	g, log := fakeGraph(t, func(_ seen, w http.ResponseWriter) { _, _ = io.WriteString(w, `{"success":true}`) })
	tok := []byte("SENTINEL-TOKEN")
	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		if _, err := g.Do(context.Background(), m, "123/subscribed_apps", url.Values{"subscribed_fields": {"feed"}}, tok, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range *log {
		if strings.Contains(s.rawQuery, "SENTINEL") || strings.Contains(s.path, "SENTINEL") {
			t.Fatalf("%s put the token in the URL", s.method)
		}
		inBody, inHeader := strings.Contains(s.body, "SENTINEL-TOKEN"), s.auth == "Bearer SENTINEL-TOKEN"
		if (s.method == http.MethodPost) != inBody || (s.method != http.MethodPost) != inHeader || (inBody && inHeader) {
			t.Fatalf("%s: body=%v header=%v", s.method, inBody, inHeader)
		}
		if s.path != "/v99.0/123/subscribed_apps" || s.rawQuery != "subscribed_fields=feed" {
			t.Fatalf("request line %s ?%s", s.path, s.rawQuery)
		}
	}
}

// A redirect is never followed (a 307 would replay a POST body carrying the token) and the error carries no URL.
func TestDoNeverFollowsRedirectsAndFlattensErrors(t *testing.T) {
	g, log := fakeGraph(t, func(_ seen, w http.ResponseWriter) {
		w.Header().Set("Location", "http://127.0.0.1:1/elsewhere")
		w.WriteHeader(307)
	})
	rep, err := g.Do(context.Background(), http.MethodPost, "x", nil, []byte("T"), nil)
	if err != nil || rep.Status != 307 || len(*log) != 1 {
		t.Fatalf("redirect: %v %d calls=%d", err, rep.Status, len(*log))
	}
	dead, _ := NewGraph("http://127.0.0.1:1", "v99.0", nil)
	_, err = dead.Exchange(context.Background(), App{ID: "1", RedirectURI: "https://a.test/cb", Secret: []byte("SENTINEL-SECRET")}, "code")
	if !errors.Is(err, ErrTransport) || strings.Contains(err.Error(), "SENTINEL") || strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("exchange error must be the fixed text, got %v", err)
	}
}

func TestExchangeAndExtend(t *testing.T) {
	g, log := fakeGraph(t, func(s seen, w http.ResponseWriter) {
		q, _ := url.ParseQuery(s.rawQuery)
		switch {
		case q.Get("grant_type") == "fb_exchange_token" && q.Get("fb_exchange_token") == "SHORT":
			_, _ = io.WriteString(w, `{"access_token":"LONG"}`)
		case q.Get("code") == "GOOD":
			_, _ = io.WriteString(w, `{"access_token":"SHORT"}`)
		default:
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"error":{"code":100,"message":"leaky id 999"}}`)
		}
	})
	app := App{ID: "42", RedirectURI: "https://a.test/cb", Secret: []byte("SECRET")}
	short, err := g.Exchange(context.Background(), app, "GOOD")
	if err != nil || string(short) != "SHORT" {
		t.Fatalf("exchange: %q %v", short, err)
	}
	long, err := g.Extend(context.Background(), app, short)
	if err != nil || string(long) != "LONG" {
		t.Fatalf("extend: %q %v", long, err)
	}
	q, _ := url.ParseQuery((*log)[0].rawQuery)
	if q.Get("client_id") != "42" || q.Get("client_secret") != "SECRET" || q.Get("redirect_uri") != "https://a.test/cb" || q.Get("code") != "GOOD" {
		t.Fatalf("exchange query: %v", q)
	}
	if _, err := g.Exchange(context.Background(), app, "BAD"); !errors.Is(err, ErrTransport) {
		t.Fatalf("bad code: %v", err)
	}
	if _, err := g.Extend(context.Background(), app, []byte("NOPE")); !errors.Is(err, ErrTransport) {
		t.Fatalf("bad extend: %v", err)
	}
}

func TestGrantedKeepsOnlyGrantedWellFormed(t *testing.T) {
	g, _ := fakeGraph(t, func(_ seen, w http.ResponseWriter) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{
			{"permission": "pages_messaging", "status": "granted"}, {"permission": "pages_show_list", "status": "declined"},
			{"permission": "Bad-Name", "status": "granted"}, {"permission": "pages_messaging", "status": "granted"}, {"permission": "ads_read", "status": "granted"}}})
	})
	got, err := g.Granted(context.Background(), []byte("T"), 10)
	if err != nil || strings.Join(got, ",") != "ads_read,pages_messaging" {
		t.Fatalf("granted = %v %v", got, err)
	}
}

func TestDialogURLAndStateDerivation(t *testing.T) {
	d := Dialog{AppID: "1", ConfigID: "2", RedirectURI: "https://admin.test/api/meta/callback", GraphVersion: "v26.0"}
	u, _ := url.Parse(d.URL("STATE"))
	q := u.Query()
	if u.Host != "www.facebook.com" || u.Path != "/v26.0/dialog/oauth" || q.Get("config_id") != "2" || q.Get("response_type") != "code" ||
		q.Get("override_default_response_type") != "true" || q.Get("redirect_uri") != d.RedirectURI || q.Get("scope") != "" {
		t.Fatalf("dialog: %s", u)
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	a, ha := StateParam(key, "dom/a", "t", "s", "p", "idem")
	b, _ := StateParam(key, "dom/a", "t", "s", "p", "idem")
	c, _ := StateParam(key, "dom/b", "t", "s", "p", "idem")
	e, _ := StateParam(key, "dom/a", "t", "s", "p2", "idem")
	if a != b || a == c || a == e || len(a) != 43 || string(ha) != string(StateHash(a)) {
		t.Fatal("state must be deterministic per (domain, scope, key) and differ across domains and principals")
	}
}
