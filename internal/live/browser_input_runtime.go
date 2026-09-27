package live

import (
	"net"
	"net/http"
	"net/url"
	"strconv"

	"livecommerce/internal/command"
	"livecommerce/internal/integrations/livekit"
)

// BrowserInputProject binds one platform-owned project/version to a local SFU.
// The injected transport is trusted fixture configuration; its route is not
// inspectable through http.RoundTripper.
type BrowserInputProject struct {
	ProjectID         string
	CredentialVersion int64
	Config            livekit.Config
	Transport         http.RoundTripper
	BrowserURL        string
}

func (BrowserInputProject) String() string     { return "live.BrowserInputProject{redacted}" }
func (p BrowserInputProject) GoString() string { return p.String() }
func (BrowserInputProject) MarshalJSON() ([]byte, error) {
	return []byte(`"live.BrowserInputProject{redacted}"`), nil
}

type browserInputEndpoint struct {
	identity   string
	client     *livekit.Client
	browserURL string
}

// BrowserInputRuntime holds only validated configuration. Constructing it
// performs no network call and does not register a route or queue consumer.
type BrowserInputRuntime struct {
	projects map[mediaProjectKey]browserInputEndpoint
}

func (BrowserInputRuntime) String() string     { return "live.BrowserInputRuntime{redacted}" }
func (r BrowserInputRuntime) GoString() string { return r.String() }
func (BrowserInputRuntime) MarshalJSON() ([]byte, error) {
	return []byte(`"live.BrowserInputRuntime{redacted}"`), nil
}

func NewBrowserInputRuntime(projects []BrowserInputProject) (*BrowserInputRuntime, error) {
	if len(projects) < 1 || len(projects) > 128 {
		return nil, ErrMediaConfig
	}
	selected := make(map[mediaProjectKey]browserInputEndpoint, len(projects))
	for _, project := range projects {
		if !mediaProjectPattern.MatchString(project.ProjectID) || project.CredentialVersion < 1 ||
			project.Config.Environment != "MOCK" || !canonicalBrowserInputURL(project.BrowserURL) {
			return nil, ErrMediaConfig
		}
		key := mediaProjectKey{project.ProjectID, project.CredentialVersion}
		if _, exists := selected[key]; exists {
			return nil, ErrMediaConfig
		}
		client, err := livekit.New(project.Config, project.Transport)
		if err != nil {
			return nil, ErrMediaConfig
		}
		selected[key] = browserInputEndpoint{
			identity:   project.Config.Endpoint,
			client:     client,
			browserURL: project.BrowserURL,
		}
	}
	return &BrowserInputRuntime{projects: selected}, nil
}

// MintPublisher deliberately has no transaction access. Call only after the
// scoped reservation transaction has committed; never put its token in a receipt.
func (r *BrowserInputRuntime) MintPublisher(grant MediaInputGrant) (string, livekit.PublisherToken, error) {
	if r == nil {
		return "", livekit.PublisherToken{}, command.ErrConflict
	}
	endpoint, ok := r.projects[mediaProjectKey{grant.ProjectID, grant.CredentialVersion}]
	if !ok || endpoint.identity != grant.EndpointIdentity || endpoint.client == nil || endpoint.browserURL == "" {
		return "", livekit.PublisherToken{}, command.ErrConflict
	}
	token, err := endpoint.client.MintPublisher(livekit.PublisherGrant{
		RoomName: grant.RoomName, Identity: grant.PublisherIdentity,
		IssuedAt: grant.IssuedAt, ExpiresAt: grant.ExpiresAt,
	})
	if err != nil {
		return "", livekit.PublisherToken{}, command.ErrConflict
	}
	return endpoint.browserURL, token, nil
}

func canonicalBrowserInputURL(raw string) bool {
	if len(raw) == 0 || len(raw) > 256 {
		return false
	}
	for i := range raw {
		if raw[i] <= ' ' || raw[i] >= 0x7f || raw[i] == '%' || raw[i] == '\\' {
			return false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" ||
		u.Opaque != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
		return false
	}
	hostname := u.Hostname()
	if hostname != "127.0.0.1" && hostname != "::1" {
		return false
	}
	portText := u.Port()
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portText {
		return false
	}
	host := net.JoinHostPort(hostname, portText)
	if u.Host != host {
		return false
	}
	expected := u.Scheme + "://" + host
	if u.Path == "/" {
		expected += "/"
	}
	return raw == expected
}
