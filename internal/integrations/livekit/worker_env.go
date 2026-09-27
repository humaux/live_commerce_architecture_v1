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
	BrowserInput      *WorkerBrowserInputProject
}

// Browser input keeps a separate local SFU credential and pinned transport.
type WorkerBrowserInputProject struct {
	Config     Config
	Transport  http.RoundTripper
	BrowserURL string
}

func (WorkerBrowserInputProject) String() string {
	return "livekit.WorkerBrowserInputProject{redacted}"
}
func (p WorkerBrowserInputProject) GoString() string { return p.String() }
func (WorkerBrowserInputProject) MarshalJSON() ([]byte, error) {
	return []byte(`"livekit.WorkerBrowserInputProject{redacted}"`), nil
}

func (WorkerProject) String() string     { return "livekit.WorkerProject{redacted}" }
func (p WorkerProject) GoString() string { return p.String() }
func (WorkerProject) MarshalJSON() ([]byte, error) {
	return []byte(`"livekit.WorkerProject{redacted}"`), nil
}

func LoadWorkerEnvironment(getenv func(string) string) (WorkerEnvironment, error) {
	var out WorkerEnvironment
	if getenv == nil {
		return out, ErrWorkerEnvironment
	}
	switch getenv("COMMERCE_MEDIA_WORKER_ENABLED") {
	case "", "0":
		return out, nil
	case "1":
		out.Enabled = true
	default:
		return WorkerEnvironment{}, ErrWorkerEnvironment
	}
	switch getenv("COMMERCE_MEDIA_BROWSER_INPUT_ENABLED") {
	case "", "0":
	case "1":
		if getenv("COMMERCE_MEDIA_RECOVERY_SUPERVISED") != "1" {
			return WorkerEnvironment{}, ErrWorkerEnvironment
		}
		out.BrowserInputEnabled = true
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
	inputEnabled := false
	switch getenv("COMMERCE_MEDIA_BROWSER_INPUT_ENABLED") {
	case "", "0":
	case "1":
		inputEnabled = true
	default:
		return nil, ErrWorkerEnvironment
	}
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
	inputCount := 0
	for _, item := range items {
		obj, ok := item.(map[string]any)
		fields := []string{"project_id", "credential_version", "endpoint", "api_key", "api_secret", "stream_hosts", "mock_dial_address", "mock_ca_pem"}
		inputRaw, hasInput := obj["browser_input"]
		if hasInput {
			if !inputEnabled {
				return nil, ErrWorkerEnvironment
			}
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
		config := Config{Environment: "MOCK", Endpoint: endpoint, APIKey: apiKey, APISecret: apiSecret, StreamHosts: hosts}
		transport, err := workerTransport(dialAddress, caPEM)
		if err != nil {
			return nil, ErrWorkerEnvironment
		}
		if _, err := New(config, transport); err != nil {
			return nil, ErrWorkerEnvironment
		}
		project := WorkerProject{ProjectID: id, CredentialVersion: version, Config: config, Transport: transport}
		if hasInput {
			input, ok := inputRaw.(map[string]any)
			if !ok || !exactWorkerFields(input, "api_key", "api_secret", "mock_dial_address", "mock_ca_pem", "browser_url") {
				return nil, ErrWorkerEnvironment
			}
			inputKey, keyOK := input["api_key"].(string)
			inputSecret, secretOK := input["api_secret"].(string)
			inputDial, dialOK := input["mock_dial_address"].(string)
			inputCA, caOK := input["mock_ca_pem"].(string)
			browserURL, browserOK := input["browser_url"].(string)
			if !keyOK || !secretOK || !dialOK || !caOK || !browserOK {
				return nil, ErrWorkerEnvironment
			}
			inputTransport, err := workerTransport(inputDial, inputCA)
			if err != nil {
				return nil, ErrWorkerEnvironment
			}
			inputConfig := Config{Environment: "MOCK", Endpoint: endpoint, APIKey: inputKey,
				APISecret: inputSecret, StreamHosts: hosts}
			if _, err := New(inputConfig, inputTransport); err != nil {
				return nil, ErrWorkerEnvironment
			}
			project.BrowserInput = &WorkerBrowserInputProject{Config: inputConfig,
				Transport: inputTransport, BrowserURL: browserURL}
			inputCount++
		}
		projects = append(projects, project)
	}
	if inputEnabled && inputCount == 0 {
		return nil, ErrWorkerEnvironment
	}
	return projects, nil
}

func workerTransport(rawAddress, caPEM string) (http.RoundTripper, error) {
	address, ok := canonicalWorkerLoopback(rawAddress)
	if !ok || len(caPEM) == 0 || len(caPEM) > 16384 {
		return nil, ErrWorkerEnvironment
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, ErrWorkerEnvironment
	}
	// Both transports pin a literal loopback dial and verify the logical endpoint TLS name.
	return &http.Transport{Proxy: nil, DisableKeepAlives: true,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots},
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
		},
	}, nil
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
