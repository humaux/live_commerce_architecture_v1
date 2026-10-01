package metaads

// config.go: Graph host guard, version pin and the shared bounded HTTP helper. Every call of this
// package (routes, CAPI, OAuth) goes through graph.do so that the host rule, the no-redirect rule,
// the body cap and the "an error never carries a URL" rule live in one place.

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	metaoauth "livecommerce/internal/integrations/meta/oauth"
)

// GraphHost is the only non-loopback base URL Config accepts (meta-ads-v1 §3).
const GraphHost = metaoauth.GraphHost

var (
	// ErrConfig is every constructor/config failure; it never carries a secret.
	ErrConfig = errors.New("metaads: invalid configuration")

	// errTransport is every failed exchange (dial, timeout, TLS, redirect, oversize body). It is
	// deliberately fixed text: a *url.Error would echo the request URL, which for the OAuth code
	// exchange contains client_secret.
	errTransport = errors.New("metaads: graph exchange failed")

	versionPattern   = regexp.MustCompile(`^v[0-9]{1,3}\.[0-9]{1,2}$`)
	loopbackPattern  = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]{1,5}$`)
	partnerPattern   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,50}$`)
	numericIDPattern = regexp.MustCompile(`^[0-9]{1,40}$`)
)

// Config is the Graph adapter configuration. GraphBaseURL is "" (= GraphHost), GraphHost or a
// loopback http://127.0.0.1:<port> (MOCK). GraphVersion is required (A-9, v26.0 in production).
// PartnerAgent is the CAPI `partner_agent` constant (F7); it is only needed by Client.PostEvent.
// HTTPClient is optional and copied; redirects are never followed (a 307 would replay a POST body
// that carries the token).
type Config struct {
	GraphBaseURL string
	GraphVersion string
	PartnerAgent string
	HTTPClient   *http.Client
}

func (c Config) validate() error {
	base := c.GraphBaseURL
	if base == "" {
		base = GraphHost
	}
	if !versionPattern.MatchString(c.GraphVersion) || !(base == GraphHost || loopbackPattern.MatchString(base)) {
		return ErrConfig
	}
	if c.PartnerAgent != "" && !partnerPattern.MatchString(c.PartnerAgent) {
		return ErrConfig
	}
	return nil
}

// graph is the shared Graph transport (internal/integrations/meta/oauth.Graph; one implementation for ads and the Page
// connect). It holds no credential.
type graph struct{ g *metaoauth.Graph }

func newGraph(cfg Config) (*graph, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	g, err := metaoauth.NewGraph(cfg.GraphBaseURL, cfg.GraphVersion, cfg.HTTPClient)
	if err != nil {
		return nil, ErrConfig
	}
	return &graph{g: g}, nil
}

// reply is one bounded Graph response.
type reply struct {
	status int
	body   []byte
}

func (r reply) ok() bool { return r.status >= 200 && r.status <= 299 }

// do performs one Graph call through metaoauth.Graph.Do (host rule, no-redirect rule, body cap and "an error never carries
// a URL" live there); every transport failure is errTransport. See Graph.Do for the token placement and UNKNOWN rules.
func (g *graph) do(ctx context.Context, method, path string, query url.Values, token []byte, payload map[string]any) (reply, error) {
	rep, err := g.g.Do(ctx, method, path, query, token, payload)
	if err != nil {
		return reply{}, errTransport
	}
	return reply{status: rep.Status, body: rep.Body}, nil
}

// SecretFromEnv returns the secret named name (for example COMMERCE_META_ADS_APP_SECRET) from either
// form the deployment can deliver (O-D: secrets are files, never chat, never argv):
//   - NAME_FILE=/run/secrets/x is expanded by deploy/tools/lcentry into NAME=<contents> before the
//     process starts (and NAME_FILE is dropped), so in a container only NAME is visible;
//   - NAME_FILE=<path> read directly when the binary runs without lcentry (dev, tests, operators).
//
// Setting both, neither, a non-regular file or a value outside 1..max bytes is ErrConfig with no detail
// (never the path or the contents). One trailing CR/LF run is trimmed. The caller clears the result.
func SecretFromEnv(getenv func(string) string, name string, max int) ([]byte, error) {
	if getenv == nil || max < 1 {
		return nil, ErrConfig
	}
	inline, path := getenv(name), getenv(name+"_FILE")
	if (inline == "") == (path == "") {
		return nil, ErrConfig
	}
	raw := []byte(inline)
	if path != "" {
		info, err := os.Stat(path)
		if err != nil || len(path) > 4096 || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > int64(max) {
			return nil, ErrConfig
		}
		if raw, err = os.ReadFile(path); err != nil {
			return nil, ErrConfig
		}
	}
	trimmed := []byte(strings.TrimRight(string(raw), "\r\n"))
	clear(raw)
	if len(trimmed) < 1 || len(trimmed) > max {
		clear(trimmed)
		return nil, ErrConfig
	}
	return trimmed, nil
}
