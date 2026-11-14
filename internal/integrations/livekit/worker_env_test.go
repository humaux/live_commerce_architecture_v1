package livekit

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Synthetic DSN sentinels live in their own constants so no source line looks like a
// credential to secret scanners (GitGuardian false positives 2026-09-29); they are test sentinels.
const (
	dsnSentinel1 = "secret"
)

func workerTestVars(ca, dial string) map[string]string {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	return map[string]string{
		"COMMERCE_MEDIA_WORKER_ENABLED": "1", "COMMERCE_MEDIA_WORKER_DATABASE_URL": "postgres://worker:" + dsnSentinel1 + "@localhost/db",
		"COMMERCE_MEDIA_EXECUTOR_DATABASE_URL": "postgres://executor:" + dsnSentinel1 + "@localhost/db", "COMMERCE_MEDIA_WORKER_CONCURRENCY": "32",
		"COMMERCE_MEDIA_MATERIAL_ACTIVE_KEY_ID": "k1",
		"COMMERCE_MEDIA_MATERIAL_KEYS_JSON":     fmt.Sprintf(`{"keys":[{"id":"k1","key_base64":%q}]}`, key),
		"COMMERCE_MEDIA_PROJECTS_JSON":          fmt.Sprintf(`{"projects":[{"project_id":"p1","credential_version":1,"endpoint":"https://unit.livekit.cloud","api_key":"test_key","api_secret":%q,"stream_hosts":["ingest.example.com"],"mock_dial_address":%q,"mock_ca_pem":%q}]}`, strings.Repeat("s", 40), dial, ca),
	}
}

func workerLoad(v map[string]string) (WorkerEnvironment, error) {
	return LoadWorkerEnvironment(func(k string) string { return v[k] })
}

func TestWorkerEnvironmentLMW01StrictBoundaryAndRedaction(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	// A fixture server's DER leaf is a valid nonempty PEM trust root for parsing.
	ca := string(pemCert(server.TLS.Certificates[0].Certificate[0]))
	v := workerTestVars(ca, server.Listener.Addr().String())
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	duplicateKey := strings.Replace(v["COMMERCE_MEDIA_MATERIAL_KEYS_JSON"], `]}`, `,{"id":"k1","key_base64":"`+key+`"}]}`, 1)
	var projectDoc map[string]any
	if err := json.Unmarshal([]byte(v["COMMERCE_MEDIA_PROJECTS_JSON"]), &projectDoc); err != nil {
		t.Fatal(err)
	}
	item := projectDoc["projects"].([]any)[0]
	projectDoc["projects"] = []any{item, item}
	duplicateProjectBytes, _ := json.Marshal(projectDoc)
	duplicateJSONKey := strings.TrimSuffix(v["COMMERCE_MEDIA_MATERIAL_KEYS_JSON"], "}") + `,"keys":[]}`
	good, err := workerLoad(v)
	if err != nil || !good.Enabled || good.Concurrency != 32 || len(good.Projects) != 1 || good.Projects[0].Config.Environment != "MOCK" {
		t.Fatalf("valid frozen config: %v", err)
	}
	for _, value := range []any{good, good.Projects[0], good.Keys, good.Projects[0].Config} {
		for _, rendered := range []string{fmt.Sprint(value), fmt.Sprintf("%#v", value), func() string { b, _ := json.Marshal(value); return string(b) }()} {
			if strings.Contains(rendered, "secret") || strings.Contains(rendered, "test_key") || strings.Contains(rendered, "p1") {
				t.Fatalf("secret-bearing value rendered raw: %s", rendered)
			}
		}
	}
	bad := []struct{ name, key, value string }{
		{"zero concurrency", "COMMERCE_MEDIA_WORKER_CONCURRENCY", "0"},
		{"noncanonical concurrency", "COMMERCE_MEDIA_WORKER_CONCURRENCY", "01"},
		{"over concurrency", "COMMERCE_MEDIA_WORKER_CONCURRENCY", "33"},
		{"oversize dsn", "COMMERCE_MEDIA_WORKER_DATABASE_URL", strings.Repeat("x", 8193)},
		{"duplicate key", "COMMERCE_MEDIA_MATERIAL_KEYS_JSON", duplicateKey},
		{"duplicate JSON key", "COMMERCE_MEDIA_MATERIAL_KEYS_JSON", duplicateJSONKey},
		{"unknown key field", "COMMERCE_MEDIA_MATERIAL_KEYS_JSON", strings.TrimSuffix(v["COMMERCE_MEDIA_MATERIAL_KEYS_JSON"], "}") + `,"other":1}`},
		{"trailing bytes", "COMMERCE_MEDIA_PROJECTS_JSON", v["COMMERCE_MEDIA_PROJECTS_JSON"] + `{}`},
		{"fractional version", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], `"credential_version":1`, `"credential_version":1.0`, 1)},
		{"exponent version", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], `"credential_version":1`, `"credential_version":1e0`, 1)},
		{"duplicate project", "COMMERCE_MEDIA_PROJECTS_JSON", string(duplicateProjectBytes)},
		{"remote dial", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], server.Listener.Addr().String(), "192.0.2.1:443", 1)},
		{"DNS dial", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], server.Listener.Addr().String(), "localhost:443", 1)},
		{"wildcard dial", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], server.Listener.Addr().String(), "0.0.0.0:443", 1)},
		{"bad CA", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], strconv.Quote(ca), `"not-a-cert"`, 1)},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			copy := map[string]string{}
			for k, val := range v {
				copy[k] = val
			}
			copy[tc.key] = tc.value
			if _, err := workerLoad(copy); err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("accepted malformed %s: %v", tc.name, err)
			}
		})
	}
	rotated := map[string]string{}
	for k, val := range v {
		rotated[k] = val
	}
	rotated["COMMERCE_MEDIA_MATERIAL_KEYS_JSON"] = strings.Replace(v["COMMERCE_MEDIA_MATERIAL_KEYS_JSON"], `]}`, `,{"id":"k2","key_base64":"`+base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32))+`"}]}`, 1)
	rotated["COMMERCE_MEDIA_MATERIAL_ACTIVE_KEY_ID"] = "k2"
	if env, err := workerLoad(rotated); err != nil || env.Keys == nil {
		t.Fatalf("rotation preserving prior key: %v", err)
	}
}

func pemCert(der []byte) []byte {
	return []byte("-----BEGIN CERTIFICATE-----\n" + base64.StdEncoding.EncodeToString(der) + "\n-----END CERTIFICATE-----\n")
}

type workerRoundTripCount struct {
	base  http.RoundTripper
	count atomic.Int32
}

func (c *workerRoundTripCount) RoundTrip(r *http.Request) (*http.Response, error) {
	c.count.Add(1)
	return c.base.RoundTrip(r)
}

func workerTLSServer(t *testing.T, handler http.Handler) (*httptest.Server, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "unit.livekit.cloud"}, DNSNames: []string{"unit.livekit.cloud"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(pemCert(der), append([]byte("-----BEGIN EC PRIVATE KEY-----\n"), append([]byte(base64.StdEncoding.EncodeToString(private)), []byte("\n-----END EC PRIVATE KEY-----\n")...)...))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, string(pemCert(der))
}

func TestWorkerEnvironmentLMW02PinnedTLSAndNoProxy(t *testing.T) {
	var hits, proxyHits atomic.Int32
	server, ca := workerTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Host != "unit.livekit.cloud" {
			t.Error("endpoint host was rewritten")
		}
		w.Header().Set("Location", "https://escape.livekit.cloud/")
		w.WriteHeader(http.StatusFound)
	}))
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { proxyHits.Add(1); w.WriteHeader(502) }))
	defer proxy.Close()
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("HTTP_PROXY", proxy.URL)
	defaultTransport := http.DefaultTransport
	v := workerTestVars(ca, server.Listener.Addr().String())
	env, err := workerLoad(v)
	if err != nil {
		t.Fatal(err)
	}
	tr := env.Projects[0].Transport.(*http.Transport)
	if tr.Proxy != nil || !tr.DisableKeepAlives || tr.TLSClientConfig.InsecureSkipVerify || tr.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatal("transport bypassed frozen security profile")
	}
	counted := &workerRoundTripCount{base: env.Projects[0].Transport}
	client, err := New(env.Projects[0].Config, counted)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(t.Context(), Target{RoomName: "lc_0123456789abcdef0123456789abcdef", EgressID: "EG_test"})
	if err != ErrUnavailable || counted.count.Load() != 1 || hits.Load() != 1 || proxyHits.Load() != 0 || http.DefaultTransport != defaultTransport {
		t.Fatalf("pinned TLS/redirect/proxy result: err=%v roundtrips=%d hits=%d proxy=%d", err, counted.count.Load(), hits.Load(), proxyHits.Load())
	}
	// A well-formed but unrelated root cannot authenticate the same server.
	_, otherCA := workerTLSServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	wrongCA := workerTestVars(otherCA, server.Listener.Addr().String())
	otherEnv, err := workerLoad(wrongCA)
	if err != nil {
		t.Fatal(err)
	}
	otherClient, err := New(otherEnv.Projects[0].Config, otherEnv.Projects[0].Transport)
	if err != nil {
		t.Fatal(err)
	}
	_, err = otherClient.Query(t.Context(), Target{RoomName: "lc_0123456789abcdef0123456789abcdef", EgressID: "EG_test"})
	if err == nil || hits.Load() != 1 || proxyHits.Load() != 0 {
		t.Fatalf("wrong CA reached provider: %v hits=%d proxy=%d", err, hits.Load(), proxyHits.Load())
	}
	// Trusted leaf with a different hostname must fail before HTTP handling.
	wrongHost := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer wrongHost.Close()
	wrongHostEnv, err := workerLoad(workerTestVars(string(pemCert(wrongHost.TLS.Certificates[0].Certificate[0])), wrongHost.Listener.Addr().String()))
	if err != nil {
		t.Fatal(err)
	}
	wrongHostClient, err := New(wrongHostEnv.Projects[0].Config, wrongHostEnv.Projects[0].Transport)
	if err != nil {
		t.Fatal(err)
	}
	_, err = wrongHostClient.Query(t.Context(), Target{RoomName: "lc_0123456789abcdef0123456789abcdef", EgressID: "EG_test"})
	if err == nil || hits.Load() != 1 || proxyHits.Load() != 0 {
		t.Fatalf("wrong hostname reached provider: %v hits=%d proxy=%d", err, hits.Load(), proxyHits.Load())
	}
}

// This behavioral assertion compiles against the baseline loader: enabled nested
// input must load, before any new typed interface is referenced.
func TestWorkerEnvironmentBrowserInputEnabled(t *testing.T) {
	server, ca := workerTLSServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("construction performed HTTP I/O") }))
	v := workerTestVars(ca, server.Listener.Addr().String())
	v["COMMERCE_MEDIA_BROWSER_INPUT_ENABLED"] = "1"
	v["COMMERCE_MEDIA_RECOVERY_SUPERVISED"] = "1"
	nested := fmt.Sprintf(`,"browser_input":{"api_key":"input_key","api_secret":%q,"mock_dial_address":%q,"mock_ca_pem":%q,"browser_url":"ws://127.0.0.1:7880"}`, strings.Repeat("i", 40), server.Listener.Addr().String(), ca)
	v["COMMERCE_MEDIA_PROJECTS_JSON"] = strings.TrimSuffix(v["COMMERCE_MEDIA_PROJECTS_JSON"], `}]}`) + nested + `}]}`
	projects, err := LoadWorkerProjects(func(k string) string { return v[k] })
	if err != nil || len(projects) != 1 {
		t.Fatalf("enabled nested browser input must load: %v", err)
	}
}

// JSON mutation helpers are fixture preparation only; production always uses decodeObject.
func workerInputFixture(t *testing.T, ca, dial string) (map[string]string, map[string]any) {
	t.Helper()
	v := workerTestVars(ca, dial)
	v["COMMERCE_MEDIA_BROWSER_INPUT_ENABLED"] = "1"
	v["COMMERCE_MEDIA_RECOVERY_SUPERVISED"] = "1"
	var doc map[string]any
	if err := json.Unmarshal([]byte(v["COMMERCE_MEDIA_PROJECTS_JSON"]), &doc); err != nil {
		t.Fatal(err)
	}
	project := doc["projects"].([]any)[0].(map[string]any)
	project["browser_input"] = map[string]any{"api_key": "input_key", "api_secret": strings.Repeat("i", 40), "mock_dial_address": dial, "mock_ca_pem": ca, "browser_url": "wss://[::1]:7880/"}
	return v, project
}

func workerInputDocument(t *testing.T, projects ...any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"projects": projects})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWorkerEnvironmentBrowserInputFlags(t *testing.T) {
	server, ca := workerTLSServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unexpected HTTP") }))
	for _, flag := range []string{"", "0", "1", "true", "01", " 1", "2"} {
		for _, native := range []string{"", "0", "1", "true"} {
			for _, supervised := range []string{"", "0", "1", "true"} {
				t.Run(fmt.Sprintf("input=%q/native=%q/supervised=%q", flag, native, supervised), func(t *testing.T) {
					v, p := workerInputFixture(t, ca, server.Listener.Addr().String())
					v["COMMERCE_MEDIA_BROWSER_INPUT_ENABLED"], v["COMMERCE_MEDIA_WORKER_ENABLED"], v["COMMERCE_MEDIA_RECOVERY_SUPERVISED"] = flag, native, supervised
					if flag != "1" {
						delete(p, "browser_input")
					}
					v["COMMERCE_MEDIA_PROJECTS_JSON"] = workerInputDocument(t, p)
					requested := flag == "1"
					invalid := flag != "" && flag != "0" && flag != "1" || requested && (native != "1" || supervised != "1")
					projects, err := LoadWorkerProjects(func(k string) string { return v[k] })
					if (err != nil) != invalid {
						t.Fatalf("project flag rejection mismatch: %v", err)
					}
					if !invalid && (projects[0].BrowserInput != nil) != requested {
						t.Fatal("input state mismatch")
					}
					env, err := workerLoad(v)
					envInvalid := invalid || native != "" && native != "0" && native != "1"
					if (err != nil) != envInvalid {
						t.Fatalf("environment flag rejection mismatch: %v", err)
					}
					if !envInvalid && (env.BrowserInputEnabled != requested || env.Enabled != (native == "1")) {
						t.Fatal("environment enablement mismatch")
					}
				})
			}
		}
	}
	if _, err := LoadWorkerProjects(nil); err != ErrWorkerEnvironment {
		t.Fatal("nil getter accepted")
	}
	if _, err := LoadWorkerEnvironment(nil); err != ErrWorkerEnvironment {
		t.Fatal("nil environment getter accepted")
	}
}

func TestWorkerEnvironmentBrowserInputStrictGrammar(t *testing.T) {
	server, ca := workerTLSServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unexpected HTTP") }))
	type mutation struct {
		name   string
		change func(map[string]any)
	}
	cases := []mutation{
		{"no input", func(p map[string]any) { delete(p, "browser_input") }},
		{"null input", func(p map[string]any) { p["browser_input"] = nil }},
		{"array input", func(p map[string]any) { p["browser_input"] = []any{} }},
		{"string input", func(p map[string]any) { p["browser_input"] = "bad" }},
		{"unknown project", func(p map[string]any) { p["other"] = true }},
		{"bad identity", func(p map[string]any) { p["project_id"] = "bad.id" }},
		{"bad version", func(p map[string]any) { p["credential_version"] = 0 }},
		{"bad inherited endpoint", func(p map[string]any) { p["endpoint"] = "https://localhost" }},
		{"bad inherited hosts", func(p map[string]any) { p["stream_hosts"] = []string{"localhost"} }},
	}
	for _, field := range []string{"api_key", "api_secret", "mock_dial_address", "mock_ca_pem", "browser_url"} {
		for _, kind := range []string{"missing", "null", "number", "bool", "object", "array"} {
			cases = append(cases, mutation{field + "/" + kind, func(p map[string]any) {
				n := p["browser_input"].(map[string]any)
				switch kind {
				case "missing":
					delete(n, field)
				case "null":
					n[field] = nil
				case "number":
					n[field] = 1
				case "bool":
					n[field] = true
				case "object":
					n[field] = map[string]any{}
				case "array":
					n[field] = []any{}
				}
			}})
		}
	}
	for field, values := range map[string][]string{
		"api_key": {"", "invalid key"}, "api_secret": {"", "short", strings.Repeat("s", 257), strings.Repeat("s", 32) + "\n"},
		"mock_ca_pem":       {"", "not-a-cert", strings.Repeat("x", 16385)},
		"mock_dial_address": {"localhost:443", "192.168.1.1:443", "0.0.0.0:443", "127.0.0.2:443", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:0443", "127.0.0.1:+443", "[::1%lo0]:443"},
		"browser_url":       {"", "WS://127.0.0.1:7880", "https://127.0.0.1:7880", "ws://localhost:7880", "ws://127.0.0.1:07880", "ws://127.0.0.1:7880/path", "ws://127.0.0.1:7880?", "ws://127.0.0.1:7880#", "ws://user@127.0.0.1:7880", "ws://127.0.0.1:7880/%2f", "ws://[::1%25lo0]:7880", strings.Repeat("x", 257)},
	} {
		for i, value := range values {
			cases = append(cases, mutation{fmt.Sprintf("%s/bad%d", field, i), func(p map[string]any) { p["browser_input"].(map[string]any)[field] = value }})
		}
	}
	for _, field := range []string{"extra", "project_id", "credential_version", "endpoint", "stream_hosts"} {
		cases = append(cases, mutation{"nested unknown " + field, func(p map[string]any) { p["browser_input"].(map[string]any)[field] = "forbidden" }})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, p := workerInputFixture(t, ca, server.Listener.Addr().String())
			tc.change(p)
			v["COMMERCE_MEDIA_PROJECTS_JSON"] = workerInputDocument(t, p)
			if _, err := LoadWorkerProjects(func(k string) string { return v[k] }); err != ErrWorkerEnvironment {
				t.Fatalf("malformed input accepted: %v", err)
			}
		})
	}
	v, p := workerInputFixture(t, ca, server.Listener.Addr().String())
	good := workerInputDocument(t, p)
	for _, field := range []string{"api_key", "api_secret", "mock_dial_address", "mock_ca_pem", "browser_url"} {
		t.Run("duplicate "+field, func(t *testing.T) {
			v["COMMERCE_MEDIA_PROJECTS_JSON"] = strings.Replace(good, `"browser_input":{`, `"browser_input":{`+strconv.Quote(field)+`:"duplicate",`, 1)
			if _, err := LoadWorkerProjects(func(k string) string { return v[k] }); err != ErrWorkerEnvironment {
				t.Fatal("duplicate nested field accepted")
			}
		})
	}
	for name, doc := range map[string]string{"duplicate object": strings.Replace(good, `"browser_input":`, `"browser_input":{},"browser_input":`, 1), "duplicate identity": workerInputDocument(t, p, p), "empty projects": `{"projects":[]}`, "null projects": `{"projects":null}`, "unknown root": strings.TrimSuffix(good, "}") + `,"extra":1}`, "oversize": good + strings.Repeat(" ", 65537-len(good))} {
		t.Run(name, func(t *testing.T) {
			v["COMMERCE_MEDIA_PROJECTS_JSON"] = doc
			if _, err := LoadWorkerProjects(func(k string) string { return v[k] }); err != ErrWorkerEnvironment {
				t.Fatal("invalid document accepted")
			}
		})
	}
	for _, flag := range []string{"", "0"} {
		v["COMMERCE_MEDIA_BROWSER_INPUT_ENABLED"] = flag
		v["COMMERCE_MEDIA_PROJECTS_JSON"] = good
		if _, err := LoadWorkerProjects(func(k string) string { return v[k] }); err != ErrWorkerEnvironment {
			t.Fatal("disabled nested input accepted")
		}
	}
	v["COMMERCE_MEDIA_BROWSER_INPUT_ENABLED"] = "1"
	v["COMMERCE_MEDIA_PROJECTS_JSON"] = good + strings.Repeat(" ", 65536-len(good))
	if _, err := LoadWorkerProjects(func(k string) string { return v[k] }); err != nil {
		t.Fatal("exact document cap rejected")
	}
	// Each valid PEM is too large to fit 128 entries within the document cap.
	// Use compact invalid objects so 129 projects exercise the independent count guard.
	entries := make([]any, 129)
	for i := range entries {
		entries[i] = map[string]any{}
	}
	v["COMMERCE_MEDIA_PROJECTS_JSON"] = workerInputDocument(t, entries...)
	if _, err := LoadWorkerProjects(func(k string) string { return v[k] }); err != ErrWorkerEnvironment {
		t.Fatal("project cap accepted")
	}
}

func TestWorkerEnvironmentBrowserInputMappingAndRedaction(t *testing.T) {
	var connections atomic.Int32
	server, ca := workerTLSServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("constructor HTTP") }))
	server.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			connections.Add(1)
		}
	}
	v, p := workerInputFixture(t, ca, server.Listener.Addr().String())
	_, legacy := workerInputFixture(t, ca, server.Listener.Addr().String())
	delete(legacy, "browser_input")
	legacy["credential_version"] = 2
	v["COMMERCE_MEDIA_PROJECTS_JSON"] = workerInputDocument(t, legacy, p)
	env, err := workerLoad(v)
	if err != nil {
		t.Fatal(err)
	}
	if !env.BrowserInputEnabled || len(env.Projects) != 2 || env.Projects[0].BrowserInput != nil {
		t.Fatal("optional mapping lost")
	}
	got := env.Projects[1]
	in := got.BrowserInput
	if in == nil || got.ProjectID != "p1" || got.CredentialVersion != 1 || in.Config.Endpoint != got.Config.Endpoint || in.Config.Environment != "MOCK" || !reflect.DeepEqual(in.Config.StreamHosts, got.Config.StreamHosts) || in.Config.APIKey != "input_key" || in.Config.APISecret != strings.Repeat("i", 40) || got.Config.APIKey != "test_key" || got.Config.APISecret != strings.Repeat("s", 40) || in.Transport == got.Transport || in.BrowserURL != "wss://[::1]:7880/" {
		t.Fatal("explicit inherited mapping mismatch")
	}
	for _, value := range []any{env, got, *in, in, in.Config} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{fmt.Sprint(value), fmt.Sprintf("%+v", value), fmt.Sprintf("%#v", value), string(b)} {
			for _, secret := range []string{"input_key", strings.Repeat("i", 40), "unit.livekit.cloud", "7880", "p1", "BEGIN CERTIFICATE"} {
				if strings.Contains(s, secret) {
					t.Fatal("secret-bearing input rendering leaked")
				}
			}
		}
	}
	if connections.Load() != 0 {
		t.Fatal("configuration construction dialed")
	}
}

func TestWorkerEnvironmentBrowserInputDistinctTLS(t *testing.T) {
	var egressHits, inputHits, proxyHits atomic.Int32
	handler := func(hits *atomic.Int32) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.Host != "unit.livekit.cloud" {
				t.Error("logical hostname changed")
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		})
	}
	egress, egressCA := workerTLSServer(t, handler(&egressHits))
	input, inputCA := workerTLSServer(t, handler(&inputHits))
	proxy := httptest.NewServer(handler(&proxyHits))
	defer proxy.Close()
	t.Setenv("HTTPS_PROXY", proxy.URL)
	t.Setenv("HTTP_PROXY", proxy.URL)
	original := http.DefaultTransport
	v, p := workerInputFixture(t, egressCA, egress.Listener.Addr().String())
	nested := p["browser_input"].(map[string]any)
	nested["mock_ca_pem"], nested["mock_dial_address"] = inputCA, input.Listener.Addr().String()
	load := func() WorkerProject {
		v["COMMERCE_MEDIA_PROJECTS_JSON"] = workerInputDocument(t, p)
		projects, err := LoadWorkerProjects(func(k string) string { return v[k] })
		if err != nil {
			t.Fatal(err)
		}
		return projects[0]
	}
	call := func(c Config, tr http.RoundTripper) {
		t.Helper()
		transport := tr.(*http.Transport)
		defer transport.CloseIdleConnections()
		if transport.Proxy != nil || transport.TLSClientConfig.InsecureSkipVerify || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 || !transport.DisableKeepAlives {
			t.Fatal("unsafe transport")
		}
		client, err := New(c, tr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client.Query(t.Context(), Target{RoomName: "lc_0123456789abcdef0123456789abcdef", EgressID: "EG_test"}); err == nil {
			t.Fatal("fixture failure should propagate")
		}
	}
	got := load()
	if egressHits.Load() != 0 || inputHits.Load() != 0 {
		t.Fatal("constructor HTTP I/O")
	}
	call(got.Config, got.Transport)
	if egressHits.Load() != 1 || inputHits.Load() != 0 {
		t.Fatal("egress routed incorrectly")
	}
	call(got.BrowserInput.Config, got.BrowserInput.Transport)
	if egressHits.Load() != 1 || inputHits.Load() != 1 {
		t.Fatal("input routed incorrectly")
	}
	nested["mock_ca_pem"] = egressCA
	got = load()
	call(got.BrowserInput.Config, got.BrowserInput.Transport)
	wrongHost := httptest.NewTLSServer(handler(&inputHits))
	defer wrongHost.Close()
	nested["mock_ca_pem"], nested["mock_dial_address"] = string(pemCert(wrongHost.TLS.Certificates[0].Certificate[0])), wrongHost.Listener.Addr().String()
	got = load()
	call(got.BrowserInput.Config, got.BrowserInput.Transport)
	if egressHits.Load() != 1 || inputHits.Load() != 1 || proxyHits.Load() != 0 || http.DefaultTransport != original {
		t.Fatal("TLS verification or proxy isolation failed")
	}
}
