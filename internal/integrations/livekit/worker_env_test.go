package livekit

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func workerTestVars(ca, dial string) map[string]string {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	return map[string]string{
		"COMMERCE_MEDIA_WORKER_ENABLED": "1", "COMMERCE_MEDIA_WORKER_DATABASE_URL": "postgres://worker:secret@localhost/db",
		"COMMERCE_MEDIA_EXECUTOR_DATABASE_URL": "postgres://executor:secret@localhost/db", "COMMERCE_MEDIA_WORKER_CONCURRENCY": "32",
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
	ca := string(bytes.TrimSpace([]byte("")))
	// A fixture server's DER leaf is a valid nonempty PEM trust root for parsing.
	ca = string(pemCert(server.TLS.Certificates[0].Certificate[0]))
	v := workerTestVars(ca, server.Listener.Addr().String())
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
		{"duplicate key", "COMMERCE_MEDIA_MATERIAL_KEYS_JSON", `{"keys":[{"id":"k1","key_base64":"bad"},{"id":"k1","key_base64":"bad"}]}`},
		{"unknown key field", "COMMERCE_MEDIA_MATERIAL_KEYS_JSON", strings.TrimSuffix(v["COMMERCE_MEDIA_MATERIAL_KEYS_JSON"], "}") + `,"other":1}`},
		{"trailing bytes", "COMMERCE_MEDIA_PROJECTS_JSON", v["COMMERCE_MEDIA_PROJECTS_JSON"] + `{}`},
		{"fractional version", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], `"credential_version":1`, `"credential_version":1.0`, 1)},
		{"exponent version", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], `"credential_version":1`, `"credential_version":1e0`, 1)},
		{"duplicate project", "COMMERCE_MEDIA_PROJECTS_JSON", strings.Replace(v["COMMERCE_MEDIA_PROJECTS_JSON"], `}]}`, `},{"project_id":"p1"}]}`, 1)},
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

func TestWorkerEnvironmentLMW02PinnedTLSAndNoProxy(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/twirp/livekit.Egress/ListEgress" {
			w.Header().Set("Location", "https://escape.livekit.cloud/")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ca := string(pemCert(server.TLS.Certificates[0].Certificate[0]))
	v := workerTestVars(ca, server.Listener.Addr().String())
	env, err := workerLoad(v)
	if err != nil {
		t.Fatal(err)
	}
	tr := env.Projects[0].Transport.(*http.Transport)
	if tr.Proxy != nil || !tr.DisableKeepAlives || tr.TLSClientConfig.InsecureSkipVerify || tr.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatal("transport bypassed frozen security profile")
	}
	// The httptest certificate is valid for example.com/127.0.0.1, not the
	// configured Cloud hostname. A real call must fail hostname verification.
	client, err := New(env.Projects[0].Config, env.Projects[0].Transport)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(t.Context(), Target{RoomName: "lc_0123456789abcdef0123456789abcdef", EgressID: "EG_test"})
	if err == nil || hits.Load() != 0 {
		t.Fatalf("wrong hostname was accepted or reached handler: %v hits=%d", err, hits.Load())
	}
}
