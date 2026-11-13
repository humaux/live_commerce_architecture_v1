// Worker environment loading owns strict local MOCK configuration for native and recovery callers.
// It reuses decodeObject and New; construction never performs network I/O.
package livekit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrWorkerEnvironment = errors.New("media_worker_invalid_config")

// WorkerEnvironment owns platform configuration; its formatted forms never expose DSNs or keys.
type WorkerEnvironment struct {
	Enabled                bool
	BrowserInputEnabled    bool
	WorkerDSN, ExecutorDSN string
	Concurrency            int
	Keys                   *MaterialKeyring
	Projects               []WorkerProject
}

func (WorkerEnvironment) String() string     { return "livekit.WorkerEnvironment{redacted}" }
func (c WorkerEnvironment) GoString() string { return c.String() }
func (WorkerEnvironment) MarshalJSON() ([]byte, error) {
	return []byte(`"livekit.WorkerEnvironment{redacted}"`), nil
}

type WorkerProject struct {
	ProjectID         string
	CredentialVersion int64
	Config            Config
	Transport         http.RoundTripper
	BrowserInput      *WorkerBrowserInput
}

func (WorkerProject) String() string     { return "livekit.WorkerProject{redacted}" }
func (p WorkerProject) GoString() string { return p.String() }
func (WorkerProject) MarshalJSON() ([]byte, error) {
	return []byte(`"livekit.WorkerProject{redacted}"`), nil
}

// WorkerBrowserInput binds explicit input credentials and fixed local transport
// to its owning Worker's project identity, endpoint and stream-host policy.
type WorkerBrowserInput struct {
	Config     Config
	Transport  http.RoundTripper
	BrowserURL string
}

func (WorkerBrowserInput) String() string     { return "livekit.WorkerBrowserInput{redacted}" }
func (p WorkerBrowserInput) GoString() string { return p.String() }
func (WorkerBrowserInput) MarshalJSON() ([]byte, error) {
	return []byte(`"livekit.WorkerBrowserInput{redacted}"`), nil
}

func workerBrowserInputEnabled(getenv func(string) string) (bool, error) {
	if getenv == nil {
		return false, ErrWorkerEnvironment
	}
	switch getenv("COMMERCE_MEDIA_BROWSER_INPUT_ENABLED") {
	case "", "0":
		return false, nil
	case "1":
		if getenv("COMMERCE_MEDIA_WORKER_ENABLED") == "1" && getenv("COMMERCE_MEDIA_RECOVERY_SUPERVISED") == "1" {
			return true, nil
		}
	}
	return false, ErrWorkerEnvironment
}

func LoadWorkerEnvironment(getenv func(string) string) (WorkerEnvironment, error) {
	var out WorkerEnvironment
	if getenv == nil {
		return out, ErrWorkerEnvironment
	}
	inputEnabled, flagErr := workerBrowserInputEnabled(getenv)
	if flagErr != nil {
		return out, flagErr
	}
	out.BrowserInputEnabled = inputEnabled
	switch getenv("COMMERCE_MEDIA_WORKER_ENABLED") {
	case "", "0":
		return out, nil
	case "1":
		out.Enabled = true
	default:
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	out.WorkerDSN = getenv("COMMERCE_MEDIA_WORKER_DATABASE_URL")
	out.ExecutorDSN = getenv("COMMERCE_MEDIA_EXECUTOR_DATABASE_URL")
	if !workerDSNValid(out.WorkerDSN) || !workerDSNValid(out.ExecutorDSN) {
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	var err error
	out.Concurrency, err = LoadWorkerConcurrency(getenv)
	if err != nil {
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	active := getenv("COMMERCE_MEDIA_MATERIAL_ACTIVE_KEY_ID")
	keysJSON := getenv("COMMERCE_MEDIA_MATERIAL_KEYS_JSON")
	if len(keysJSON) == 0 || len(keysJSON) > 8192 || !materialIDPattern.MatchString(active) {
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	keysObj, err := decodeObject([]byte(keysJSON))
	if err != nil || !exactWorkerFields(keysObj, "keys") {
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	keyItems, ok := keysObj["keys"].([]any)
	if !ok || len(keyItems) < 1 || len(keyItems) > 16 {
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	keys := make(map[string][]byte, len(keyItems))
	for _, item := range keyItems {
		obj, ok := item.(map[string]any)
		if !ok || !exactWorkerFields(obj, "id", "key_base64") {
			return WorkerEnvironment{}, ErrWorkerEnvironment
		}
		id, okID := obj["id"].(string)
		encoded, okKey := obj["key_base64"].(string)
		if !okID || !okKey || !materialIDPattern.MatchString(id) {
			return WorkerEnvironment{}, ErrWorkerEnvironment
		}
		if _, duplicate := keys[id]; duplicate {
			return WorkerEnvironment{}, ErrWorkerEnvironment
		}
		key, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(key) != 32 || base64.StdEncoding.EncodeToString(key) != encoded {
			return WorkerEnvironment{}, ErrWorkerEnvironment
		}
		keys[id] = key
	}
	out.Keys, err = NewMaterialKeyring(active, keys)
	if err != nil {
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	out.Projects, err = LoadWorkerProjects(getenv)
	if err != nil {
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	return out, nil
}

// LoadWorkerProjects shares the native strict project selector with recovery.
func LoadWorkerProjects(getenv func(string) string) ([]WorkerProject, error) {
	if getenv == nil {
		return nil, ErrWorkerEnvironment
	}
	inputEnabled, err := workerBrowserInputEnabled(getenv)
	if err != nil {
		return nil, err
	}
	inputCount := 0
	var projects []WorkerProject
	projectsJSON := getenv("COMMERCE_MEDIA_PROJECTS_JSON")
	if len(projectsJSON) == 0 || len(projectsJSON) > 65536 {
		return nil, ErrWorkerEnvironment
	}
	projectsObj, err := decodeObject([]byte(projectsJSON))
	if err != nil || !exactWorkerFields(projectsObj, "projects") {
		return nil, ErrWorkerEnvironment
	}
	items, ok := projectsObj["projects"].([]any)
	if !ok || len(items) < 1 || len(items) > 128 {
		return nil, ErrWorkerEnvironment
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		fields := []string{"project_id", "credential_version", "endpoint", "api_key", "api_secret", "stream_hosts", "mock_dial_address", "mock_ca_pem"}
		if _, present := obj["browser_input"]; inputEnabled && present {
			fields = append(fields, "browser_input")
		}
		if !ok || !exactWorkerFields(obj, fields...) {
			return nil, ErrWorkerEnvironment
		}
		id, okID := obj["project_id"].(string)
		versionNumber, okVersion := obj["credential_version"].(json.Number)
		endpoint, okEndpoint := obj["endpoint"].(string)
		apiKey, okAPIKey := obj["api_key"].(string)
		apiSecret, okAPISecret := obj["api_secret"].(string)
		hostValues, okHosts := obj["stream_hosts"].([]any)
		dialAddress, okDial := obj["mock_dial_address"].(string)
		caPEM, okCA := obj["mock_ca_pem"].(string)
		if !okID || !projectIDPattern.MatchString(id) || !okVersion || !okEndpoint || !okAPIKey || !okAPISecret || !okHosts || !okDial || !okCA {
			return nil, ErrWorkerEnvironment
		}
		version, err := strconv.ParseInt(string(versionNumber), 10, 64)
		if err != nil || version < 1 || strconv.FormatInt(version, 10) != string(versionNumber) {
			return nil, ErrWorkerEnvironment
		}
		identity := id + ":" + strconv.FormatInt(version, 10)
		if seen[identity] {
			return nil, ErrWorkerEnvironment
		}
		seen[identity] = true
		hosts := make([]string, 0, len(hostValues))
		for _, value := range hostValues {
			host, ok := value.(string)
			if !ok {
				return nil, ErrWorkerEnvironment
			}
			hosts = append(hosts, host)
		}
		transport, err := workerLocalTransport(dialAddress, caPEM)
		if err != nil {
			return nil, ErrWorkerEnvironment
		}
		config := Config{Environment: "MOCK", Endpoint: endpoint, APIKey: apiKey, APISecret: apiSecret, StreamHosts: hosts}
		if _, err := New(config, transport); err != nil {
			return nil, ErrWorkerEnvironment
		}
		project := WorkerProject{ProjectID: id, CredentialVersion: version, Config: config, Transport: transport}
		if raw, present := obj["browser_input"]; present {
			nested, ok := raw.(map[string]any)
			if !ok || !exactWorkerFields(nested, "api_key", "api_secret", "mock_dial_address", "mock_ca_pem", "browser_url") {
				return nil, ErrWorkerEnvironment
			}
			key, keyOK := nested["api_key"].(string)
			secret, secretOK := nested["api_secret"].(string)
			dial, dialOK := nested["mock_dial_address"].(string)
			ca, caOK := nested["mock_ca_pem"].(string)
			browserURL, urlOK := nested["browser_url"].(string)
			if !keyOK || !secretOK || !dialOK || !caOK || !urlOK || !IsCanonicalBrowserInputURL(browserURL) {
				return nil, ErrWorkerEnvironment
			}
			inputTransport, err := workerLocalTransport(dial, ca)
			if err != nil {
				return nil, ErrWorkerEnvironment
			}
			inputConfig := Config{Environment: config.Environment, Endpoint: config.Endpoint, APIKey: key, APISecret: secret, StreamHosts: append([]string(nil), config.StreamHosts...)}
			if _, err := New(inputConfig, inputTransport); err != nil {
				return nil, ErrWorkerEnvironment
			}
			project.BrowserInput = &WorkerBrowserInput{Config: inputConfig, Transport: inputTransport, BrowserURL: browserURL}
			inputCount++
		}
		projects = append(projects, project)
	}
	if inputEnabled && inputCount == 0 {
		return nil, ErrWorkerEnvironment
	}
	return projects, nil
}

func LoadWorkerConcurrency(getenv func(string) string) (int, error) {
	if getenv == nil {
		return 0, ErrWorkerEnvironment
	}
	if raw := getenv("COMMERCE_MEDIA_WORKER_CONCURRENCY"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 32 || strconv.Itoa(n) != raw {
			return 0, ErrWorkerEnvironment
		}
		return n, nil
	}
	return 4, nil
}

func workerDSNValid(dsn string) bool {
	return len(dsn) >= 1 && len(dsn) <= 8192 && strings.TrimSpace(dsn) != ""
}

func exactWorkerFields(obj map[string]any, names ...string) bool {
	if len(obj) != len(names) {
		return false
	}
	for _, name := range names {
		if _, ok := obj[name]; !ok {
			return false
		}
	}
	return true
}

func canonicalWorkerLoopback(raw string) (string, bool) {
	host, port, err := net.SplitHostPort(raw)
	if err != nil || (host != "127.0.0.1" && host != "::1") {
		return "", false
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port || net.JoinHostPort(host, port) != raw {
		return "", false
	}
	return raw, true
}

// workerLocalTransport fixes the dial route without weakening logical TLS identity.
func workerLocalTransport(dialAddress, caPEM string) (http.RoundTripper, error) {
	address, ok := canonicalWorkerLoopback(dialAddress)
	if !ok || len(caPEM) == 0 || len(caPEM) > 16384 {
		return nil, ErrWorkerEnvironment
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, ErrWorkerEnvironment
	}
	return &http.Transport{Proxy: nil, DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots},
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
		},
	}, nil
}

// IsCanonicalBrowserInputURL validates the shared local browser-input URL grammar.
func IsCanonicalBrowserInputURL(raw string) bool {
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
