package main

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"livecommerce/internal/integrations/livekit"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Synthetic DSN sentinels live in their own constants so no source line looks like a
// credential to secret scanners (GitGuardian false positives 2026-09-29); they are test sentinels.
const (
	dsnSentinel1 = "private-pass"
)

func TestMediaWorkerLMW01DisabledReadsOnlyFlags(t *testing.T) {
	for _, flag := range []string{"", "0"} {
		var reads []string
		err := run(nil, func(name string) string {
			reads = append(reads, name)
			if name == "COMMERCE_MEDIA_BROWSER_INPUT_ENABLED" {
				return ""
			}
			if name == "COMMERCE_MEDIA_WORKER_ENABLED" {
				return flag
			}
			return "must-not-read"
		})
		if err != nil || len(reads) != 2 || reads[0] != "COMMERCE_MEDIA_BROWSER_INPUT_ENABLED" || reads[1] != "COMMERCE_MEDIA_WORKER_ENABLED" {
			t.Fatalf("disabled %q read=%v err=%v", flag, reads, err)
		}
	}
	for _, flag := range []string{"false", "2", " 1", "1 "} {
		var reads []string
		err := run(context.Background(), func(name string) string { reads = append(reads, name); return flag })
		if !errors.Is(err, errWorkerConfig) || len(reads) != 1 {
			t.Fatalf("noncanonical flag %q read=%v err=%v", flag, reads, err)
		}
	}
}

func TestMediaWorkerLMW01ConfigBeforeDatabase(t *testing.T) {
	vars := map[string]string{
		"COMMERCE_MEDIA_WORKER_ENABLED":         "1",
		"COMMERCE_MEDIA_WORKER_DATABASE_URL":    "postgres://private-user:" + dsnSentinel1 + "@127.0.0.1:1/test",
		"COMMERCE_MEDIA_EXECUTOR_DATABASE_URL":  "postgres://private-user:" + dsnSentinel1 + "@127.0.0.1:1/test",
		"COMMERCE_MEDIA_WORKER_CONCURRENCY":     "1",
		"COMMERCE_MEDIA_MATERIAL_ACTIVE_KEY_ID": "bad",
		"COMMERCE_MEDIA_MATERIAL_KEYS_JSON":     "private-malformed-key",
		"COMMERCE_MEDIA_PROJECTS_JSON":          "private-malformed-project",
	}
	get := func(name string) string { return vars[name] }
	if err := run(context.Background(), get); !errors.Is(err, errWorkerConfig) || strings.Contains(err.Error(), "private") {
		t.Fatalf("malformed config reached DB or exposed input: %v", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("nil context made network request") }))
	defer server.Close()
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]}))
	vars["COMMERCE_MEDIA_MATERIAL_ACTIVE_KEY_ID"] = "k1"
	vars["COMMERCE_MEDIA_MATERIAL_KEYS_JSON"] = fmt.Sprintf(`{"keys":[{"id":"k1","key_base64":%q}]}`, key)
	vars["COMMERCE_MEDIA_PROJECTS_JSON"] = fmt.Sprintf(`{"projects":[{"project_id":"p1","credential_version":1,"endpoint":"https://unit.livekit.cloud","api_key":"test_key","api_secret":%q,"stream_hosts":["ingest.example.com"],"mock_dial_address":%q,"mock_ca_pem":%q}]}`, strings.Repeat("s", 40), server.Listener.Addr().String(), ca)
	if err := run(nil, get); !errors.Is(err, errWorkerConfig) {
		t.Fatalf("valid enabled config with nil context: %v", err)
	}
}

func TestMediaProcessMappingAndChildEnvironment(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("configuration performed provider I/O") }))
	defer server.Close()
	ca := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[0].Certificate[0]}))
	doc := fmt.Sprintf(`{"projects":[{"project_id":"p1","credential_version":7,"endpoint":"https://unit.livekit.cloud","api_key":"egress_key","api_secret":%q,"stream_hosts":["ingest.example.com"],"mock_dial_address":%q,"mock_ca_pem":%q,"browser_input":{"api_key":"input_key","api_secret":%q,"mock_dial_address":%q,"mock_ca_pem":%q,"browser_url":"ws://127.0.0.1:7880"}}]}`, strings.Repeat("e", 40), server.Listener.Addr().String(), ca, strings.Repeat("i", 40), server.Listener.Addr().String(), ca)
	for k, v := range map[string]string{"COMMERCE_MEDIA_BROWSER_INPUT_ENABLED": "1", "COMMERCE_MEDIA_WORKER_ENABLED": "1", "COMMERCE_MEDIA_RECOVERY_SUPERVISED": "1", "COMMERCE_MEDIA_PROJECTS_JSON": doc, "COMMERCE_MEDIA_RECOVERY_DATABASE_URL": "private-recovery-sentinel"} {
		t.Setenv(k, v)
	}
	child := map[string]string{}
	for _, entry := range sanitizedNativeEnvironment() {
		p := strings.SplitN(entry, "=", 2)
		child[p[0]] = p[1]
	}
	if child["COMMERCE_MEDIA_BROWSER_INPUT_ENABLED"] != "1" || child["COMMERCE_MEDIA_PROJECTS_JSON"] != doc || child["COMMERCE_MEDIA_RECOVERY_DATABASE_URL"] != "" {
		t.Fatal("child environment lost mapping or leaked recovery authority")
	}
	for _, get := range []func(string) string{os.Getenv, func(k string) string { return child[k] }} {
		loaded, err := livekit.LoadWorkerProjects(get)
		if err != nil {
			t.Fatal(err)
		}
		projects, input, err := workerRuntimes(loaded, true)
		if err != nil || input == nil || len(projects) != 1 || projects[0].ProjectID != "p1" || projects[0].CredentialVersion != 7 || !reflect.DeepEqual(projects[0].Config, loaded[0].Config) || loaded[0].BrowserInput.Config.APIKey == loaded[0].Config.APIKey {
			t.Fatal("shared loader mapping lost identity or distinct credentials")
		}
		if _, _, err := workerRuntimes(loaded, false); err == nil {
			t.Fatal("disabled mapping accepted input")
		}
	}
	if _, _, err := workerRuntimes(nil, true); err == nil {
		t.Fatal("missing input runtime accepted")
	}
}

func TestMediaProcessRequestedInvalidScope(t *testing.T) {
	for _, flag := range []string{"", "0", "1", "true", " 1", "2"} {
		get := func(k string) string {
			if k == "COMMERCE_MEDIA_BROWSER_INPUT_ENABLED" {
				return flag
			}
			return ""
		}
		want := flag != "" && flag != "0"
		if requestedBrowserInput(get) != want {
			t.Fatalf("scope %q", flag)
		}
		if want {
			if _, err := livekit.LoadWorkerProjects(get); err == nil {
				t.Fatal("requested invalid config accepted")
			}
		}
	}
}
