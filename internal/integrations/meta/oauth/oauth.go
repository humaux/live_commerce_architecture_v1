// Package metaoauth owns the Meta OAuth plumbing shared by the merchant ads connect (internal/ads +
// internal/integrations/meta_ads) and the merchant Page / Instagram connect (internal/metaconnect): the bounded Graph
// transport with its host allowlist, the Facebook Login for Business dialog URL, the HMAC OAuth state derivation, the
// server-to-server code exchange and the granted-permission read.
//
// It never stores or logs a token, never decides what a connect may bind (callers own that), never touches PostgreSQL,
// and never follows a redirect. External service: graph.facebook.com (Graph API) and, only inside the URL it builds for the
// merchant's browser, www.facebook.com (the Login dialog). The base URL is exactly GraphHost, or a loopback
// http://127.0.0.1:<port> in MOCK tests.
package metaoauth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
)

// GraphHost is the only non-loopback base URL a Graph accepts (meta-ads-v1 §3, meta-claims-intake-v1 §6.3).
const GraphHost = "https://graph.facebook.com"

// MaxBody caps every Graph response (bounded bodies <= 1 MiB).
const MaxBody = 1 << 20

var (
	// ErrConfig is every constructor failure; it never carries a secret.
	ErrConfig = errors.New("metaoauth: invalid configuration")
	// ErrTransport is every failed exchange (dial, timeout, TLS, redirect, oversize body). Fixed text on purpose: a
	// *url.Error would echo the request URL, which for the code exchange contains client_secret.
	ErrTransport = errors.New("metaoauth: graph exchange failed")

	versionPattern  = regexp.MustCompile(`^v[0-9]{1,3}\.[0-9]{1,2}$`)
	loopbackPattern = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]{1,5}$`)
	cursorPattern   = regexp.MustCompile(`^[A-Za-z0-9_=+/.-]{1,512}$`)
	scopePattern    = regexp.MustCompile(`^[a-z_]{1,64}$`)
)

// LoopbackURL reports whether s is an http://127.0.0.1:<port> origin (the MOCK Graph / dev redirect form).
func LoopbackURL(s string) bool { return loopbackPattern.MatchString(s) }

// Graph is the shared Graph transport. It holds no credential.
type Graph struct {
	base, version string
	hc            *http.Client
}

// Reply is one bounded Graph response.
type Reply struct {
	Status int
	Body   []byte
}

// OK reports a 2xx status.
func (r Reply) OK() bool { return r.Status >= 200 && r.Status <= 299 }

// NewGraph validates the base ("" = GraphHost, GraphHost or a loopback origin) and the API version (required, no default:
// probe U5) and copies hc (nil = a fresh client). Redirects are never followed: a 307 would replay a POST body that carries
// the token.
func NewGraph(base, version string, hc *http.Client) (*Graph, error) {
	if base == "" {
		base = GraphHost
	}
	if !versionPattern.MatchString(version) || !(base == GraphHost || loopbackPattern.MatchString(base)) {
		return nil, ErrConfig
	}
	c := &http.Client{}
	if hc != nil {
		copied := *hc
		c = &copied
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Graph{base: base, version: version, hc: c}, nil
}

// Do performs one Graph call: path is relative to /{version}/ (no leading slash). A GET or DELETE carries the
// token in the Authorization header (never the URL); a POST carries it as `access_token` in the JSON body and never in the URL.
// // UNKNOWN until MA-S1: that Graph accepts `Authorization: Bearer` on GET reads for BISU tokens; fallback is the
// documented access_token query parameter, which would need a reviewed change because it puts the token in a URL.
// The only secret allowed in a URL is the OAuth exchange's client_secret (contract F3, GET form). A non-nil error means "no
// usable response": the request may or may not have been processed, so callers of mutating calls must treat it as UNKNOWN.
func (g *Graph) Do(ctx context.Context, method, path string, query url.Values, token []byte, payload map[string]any) (Reply, error) {
	target := g.base + "/" + g.version + "/" + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var body io.Reader
	if method == http.MethodPost {
		if payload == nil {
			payload = map[string]any{}
		}
		if len(token) > 0 {
			payload["access_token"] = string(token) // ponytail: Go strings cannot be zeroed; process-memory only, short-lived
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return Reply{}, ErrTransport
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return Reply{}, ErrTransport
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+string(token))
	}
	resp, err := g.hc.Do(req)
	if err != nil {
		return Reply{}, ErrTransport
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil || len(raw) > MaxBody {
		return Reply{}, ErrTransport
	}
	return Reply{Status: resp.StatusCode, Body: raw}, nil
}

// App is the Meta app used for the code exchange. RedirectURI must equal the dialog value and the dashboard setting (U10).
// Every formatter is redacted; Secret comes from a secret file / the app registry, never a flag.
type App struct {
	ID, RedirectURI string
	Secret          []byte
}

func (App) String() string               { return "[redacted]" }
func (App) GoString() string             { return "[redacted]" }
func (App) Format(f fmt.State, _ rune)   { _, _ = f.Write([]byte("[redacted]")) }
func (App) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }
func (App) MarshalText() ([]byte, error) { return []byte(`"[redacted]"`), nil }

// Exchange swaps an OAuth code for an access token, server to server. Contract F3 documents GET with client_secret in the
// query; this is the only call whose URL holds a secret, over TLS to graph.facebook.com, never logged, and its errors are
// flattened (ErrTransport). // UNKNOWN until MA-S3 (U10): redirect_uri may be unneeded. The caller zeroes the result.
// Retry rule: never retried (a code is single-use at Meta); every failure is the caller's "connect failed".
func (g *Graph) Exchange(ctx context.Context, app App, code string) ([]byte, error) {
	rep, err := g.Do(ctx, http.MethodGet, "oauth/access_token", url.Values{
		"client_id": {app.ID}, "client_secret": {string(app.Secret)}, "redirect_uri": {app.RedirectURI}, "code": {code}}, nil, nil)
	if err != nil || !rep.OK() {
		return nil, ErrTransport
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	err = json.Unmarshal(rep.Body, &tok)
	clear(rep.Body)
	token := []byte(tok.AccessToken)
	tok.AccessToken = ""
	if err != nil || len(token) == 0 {
		clear(token)
		return nil, ErrTransport
	}
	return token, nil
}

// Extend swaps a short-lived user token for a long-lived one (grant_type=fb_exchange_token). A Page token read with a
// long-lived user token does not expire, which is what unattended private replies need; a Page token read with the short-lived
// one dies within hours. Same secret-in-URL, flattened-error and never-retried rules as Exchange. The caller zeroes both tokens.
// Source: https://developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived (retrieved 2026-10-01).
func (g *Graph) Extend(ctx context.Context, app App, short []byte) ([]byte, error) {
	rep, err := g.Do(ctx, http.MethodGet, "oauth/access_token", url.Values{
		"grant_type": {"fb_exchange_token"}, "client_id": {app.ID}, "client_secret": {string(app.Secret)},
		"fb_exchange_token": {string(short)}}, nil, nil)
	if err != nil || !rep.OK() {
		return nil, ErrTransport
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	err = json.Unmarshal(rep.Body, &tok)
	clear(rep.Body)
	long := []byte(tok.AccessToken)
	tok.AccessToken = ""
	if err != nil || len(long) == 0 {
		clear(long)
		return nil, ErrTransport
	}
	return long, nil
}

// Edge walks one Graph edge with cursors, at most limit pages, calling each on every item until it returns false.
func (g *Graph) Edge(ctx context.Context, path string, query url.Values, token []byte, limit int, each func(json.RawMessage) bool) error {
	after := ""
	for i := 0; i < limit; i++ {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		if after != "" {
			q.Set("after", after)
		}
		rep, err := g.Do(ctx, http.MethodGet, path, q, token, nil)
		if err != nil || !rep.OK() {
			return ErrTransport
		}
		var doc struct {
			Data   []json.RawMessage `json:"data"`
			Paging struct {
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
				Next string `json:"next"`
			} `json:"paging"`
		}
		if json.Unmarshal(rep.Body, &doc) != nil {
			return ErrTransport
		}
		for _, item := range doc.Data {
			if !each(item) {
				return nil
			}
		}
		if doc.Paging.Next == "" || !cursorPattern.MatchString(doc.Paging.Cursors.After) {
			return nil
		}
		after = doc.Paging.Cursors.After
	}
	return nil
}

// Granted returns the sorted, de-duplicated names of the permissions /me/permissions reports as "granted" (at most max).
func (g *Graph) Granted(ctx context.Context, token []byte, max int) ([]string, error) {
	seen := map[string]bool{}
	err := g.Edge(ctx, "me/permissions", nil, token, 2, func(raw json.RawMessage) bool {
		var p struct {
			Permission string `json:"permission"`
			Status     string `json:"status"`
		}
		if json.Unmarshal(raw, &p) == nil && p.Status == "granted" && scopePattern.MatchString(p.Permission) && len(seen) < max {
			seen[p.Permission] = true
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

// Dialog is the Facebook Login for Business dialog configuration (contract meta-ads-v1 §2 step 1).
type Dialog struct{ AppID, ConfigID, RedirectURI, GraphVersion string }

// URL builds the dialog URL: config_id replaces scope, response_type=code with override_default_response_type=true yields a
// server-exchangeable code and a BISU token (F3).
// Source: https://developers.facebook.com/documentation/facebook-login/facebook-login-for-business (retrieved 2026-09-30).
func (d Dialog) URL(state string) string {
	q := url.Values{}
	q.Set("client_id", d.AppID)
	q.Set("config_id", d.ConfigID)
	q.Set("response_type", "code")
	q.Set("override_default_response_type", "true")
	q.Set("redirect_uri", d.RedirectURI)
	q.Set("state", state)
	u := url.URL{Scheme: "https", Host: "www.facebook.com", Path: "/" + d.GraphVersion + "/dialog/oauth", RawQuery: q.Encode()}
	return u.String()
}

// StateParam derives the OAuth state deterministically from the caller's scope and Idempotency-Key, so a replay rebuilds the
// same dialog URL without the receipt storing it. It is an HMAC-SHA256 under the server-side key and a per-flow domain label
// (so an ads state can never be replayed as a Page-connect state): the Idempotency-Key sits in clear in
// ops.command_results, so an unkeyed hash of it would be recomputable by a reader of that table. Only its SHA-256 is stored.
func StateParam(key []byte, domain, tenantID, storeID, principalID, idemKey string) (param string, hash []byte) {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(domain + "|" + tenantID + "|" + storeID + "|" + principalID + "|" + idemKey))
	param = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	return param, StateHash(param)
}

// StateHash is the digest stored for a state parameter (SHA-256 of its text).
func StateHash(param string) []byte {
	d := sha256.Sum256([]byte(param))
	return d[:]
}
