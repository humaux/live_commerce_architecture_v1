package live_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"livecommerce/internal/integrations/livekit"
	"livecommerce/internal/live"
)

type countingTransport struct{ calls int }

func (t *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return nil, errors.New("unexpected network call")
}

func browserInputProject(rt http.RoundTripper) live.BrowserInputProject {
	return live.BrowserInputProject{
		ProjectID:         "project_1",
		CredentialVersion: 1,
		Config: livekit.Config{
			Environment: "MOCK",
			Endpoint:    "https://test-project.livekit.cloud",
			APIKey:      "test_key_1",
			APISecret:   strings.Repeat("S", 32),
			StreamHosts: []string{"stream.example.com"},
		},
		Transport:  rt,
		BrowserURL: "ws://127.0.0.1:7880",
	}
}

func TestBrowserInputRuntimeProjectBoundary(t *testing.T) {
	rt := &countingTransport{}
	project := browserInputProject(rt)
	for _, n := range []int{1, 128} {
		projects := make([]live.BrowserInputProject, n)
		for i := range projects {
			projects[i] = project
			projects[i].ProjectID = fmt.Sprintf("project_%d", i)
		}
		got, err := live.NewBrowserInputRuntime(projects)
		if err != nil || got == nil {
			t.Fatalf("%d valid projects: runtime=%v err=%v", n, got, err)
		}
	}
	if rt.calls != 0 {
		t.Fatalf("constructor called transport %d times", rt.calls)
	}

	invalid := []struct {
		name     string
		projects []live.BrowserInputProject
	}{
		{"nil", nil},
		{"empty", []live.BrowserInputProject{}},
		{"too many", make([]live.BrowserInputProject, 129)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) { requireBrowserInputConfigError(t, tc.projects, rt) })
	}

	second := project
	second.CredentialVersion = 2
	if got, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{project, second}); err != nil || got == nil {
		t.Fatalf("same project with different versions: runtime=%v err=%v", got, err)
	}
	second.CredentialVersion = 1
	second.Config.Endpoint = "https://other-project.livekit.cloud"
	requireBrowserInputConfigError(t, []live.BrowserInputProject{project, second}, rt)
	if rt.calls != 0 {
		t.Fatalf("constructor called transport %d times", rt.calls)
	}
}

func TestBrowserInputRuntimeRejectsInvalidProjectsAndConfig(t *testing.T) {
	rt := &countingTransport{}
	base := browserInputProject(rt)
	bad := []struct {
		name   string
		change func(*live.BrowserInputProject)
	}{
		{"empty project", func(p *live.BrowserInputProject) { p.ProjectID = "" }},
		{"project punctuation", func(p *live.BrowserInputProject) { p.ProjectID = "bad.project" }},
		{"project whitespace", func(p *live.BrowserInputProject) { p.ProjectID = " project" }},
		{"project over 80", func(p *live.BrowserInputProject) { p.ProjectID = strings.Repeat("p", 81) }},
		{"zero version", func(p *live.BrowserInputProject) { p.CredentialVersion = 0 }},
		{"negative version", func(p *live.BrowserInputProject) { p.CredentialVersion = -1 }},
		{"LIVE config", func(p *live.BrowserInputProject) { p.Config.Environment = "LIVE" }},
		{"lowercase MOCK", func(p *live.BrowserInputProject) { p.Config.Environment = "mock" }},
		{"invalid endpoint", func(p *live.BrowserInputProject) { p.Config.Endpoint = "http://test-project.livekit.cloud" }},
		{"invalid key", func(p *live.BrowserInputProject) { p.Config.APIKey = "invalid key" }},
		{"short secret", func(p *live.BrowserInputProject) { p.Config.APISecret = "short" }},
		{"no stream hosts", func(p *live.BrowserInputProject) { p.Config.StreamHosts = nil }},
		{"invalid stream host", func(p *live.BrowserInputProject) { p.Config.StreamHosts = []string{"localhost"} }},
		{"nil transport", func(p *live.BrowserInputProject) { p.Transport = nil }},
		{"typed nil transport", func(p *live.BrowserInputProject) { var typedNil *countingTransport; p.Transport = typedNil }},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			p.CredentialVersion = 2
			tc.change(&p)
			requireBrowserInputConfigError(t, []live.BrowserInputProject{base, p}, rt)
		})
	}
	if rt.calls != 0 {
		t.Fatalf("constructor called transport %d times", rt.calls)
	}
}

func TestBrowserInputRuntimeCanonicalBrowserURL(t *testing.T) {
	rt := &countingTransport{}
	base := browserInputProject(rt)
	valid := []string{
		"ws://127.0.0.1:1", "wss://127.0.0.1:65535/",
		"ws://[::1]:1/", "wss://[::1]:65535",
	}
	for _, raw := range valid {
		t.Run("valid "+raw, func(t *testing.T) {
			p := base
			p.BrowserURL = raw
			got, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{p})
			if err != nil || got == nil {
				t.Fatalf("valid URL rejected: runtime=%v err=%v", got, err)
			}
		})
	}
	invalid := []string{
		"", "http://127.0.0.1:7880", "WS://127.0.0.1:7880",
		"ws://localhost:7880", "ws://example.com:7880", "ws://192.168.0.1:7880",
		"ws://127.0.0.2:7880", "ws://127.0.0.01:7880", "ws://127.0.0.1",
		"ws://127.0.0.1:0", "ws://127.0.0.1:65536", "ws://127.0.0.1:07880",
		"ws://127.0.0.1:+1", "ws://127.0.0.1:abc", "ws://127.0.0.1:/",
		"ws://127.0.0.1:7880/path", "ws://127.0.0.1:7880/%2F",
		"ws://127.0.0.1:7880/?", "ws://127.0.0.1:7880/#",
		"ws://127.0.0.1:7880?x=1", "ws://127.0.0.1:7880#x",
		"ws://user@127.0.0.1:7880", "ws://[::ffff:127.0.0.1]:7880",
		"ws://[0:0:0:0:0:0:0:1]:7880", "ws://[::1]", "ws://[::1]:0",
		"ws://[::1]:65536", "ws://[::1]:07880", "ws://[::1]:7880/path",
		"ws://[::1]:7880/?", "ws://[::1]:7880/#",
		"ws://[::1%25lo0]:7880", "ws://127.0.0.1:7880 ", " ws://127.0.0.1:7880",
		"ws://127.0.0.1:7880/" + strings.Repeat("a", 257),
	}
	for _, raw := range invalid {
		t.Run("invalid "+raw, func(t *testing.T) {
			p := base
			p.CredentialVersion = 2
			p.BrowserURL = raw
			requireBrowserInputConfigError(t, []live.BrowserInputProject{base, p}, rt)
		})
	}
	if rt.calls != 0 {
		t.Fatalf("constructor called transport %d times", rt.calls)
	}
}

func TestBrowserInputRuntimeRedactsAllFormatting(t *testing.T) {
	rt := &countingTransport{}
	p := browserInputProject(rt)
	p.ProjectID = "private_project_marker"
	p.Config.APIKey = "private_key_marker"
	p.Config.APISecret = "private_secret_marker_1234567890123456"
	p.Config.Endpoint = "https://private-endpoint.livekit.cloud"
	p.BrowserURL = "wss://127.0.0.1:7880"
	runtime, err := live.NewBrowserInputRuntime([]live.BrowserInputProject{p})
	if err != nil || runtime == nil {
		t.Fatalf("valid secret-bearing config: runtime=%v err=%v", runtime, err)
	}
	for _, value := range []any{p, runtime} {
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, output := range []string{fmt.Sprint(value), fmt.Sprintf("%#v", value), string(jsonBytes)} {
			for _, secret := range []string{p.ProjectID, p.Config.APIKey, p.Config.APISecret, p.Config.Endpoint, p.BrowserURL} {
				if strings.Contains(output, secret) {
					t.Errorf("formatted %T exposed configuration value", value)
				}
			}
		}
	}
	if rt.calls != 0 {
		t.Fatalf("constructor called transport %d times", rt.calls)
	}
}

func requireBrowserInputConfigError(t *testing.T, projects []live.BrowserInputProject, rt *countingTransport) {
	t.Helper()
	got, err := live.NewBrowserInputRuntime(projects)
	if got != nil || !errors.Is(err, live.ErrMediaConfig) {
		t.Fatalf("invalid config: runtime=%v err=%v", got, err)
	}
	if rt.calls != 0 {
		t.Fatalf("invalid config called transport %d times", rt.calls)
	}
	for _, project := range projects {
		for _, secret := range []string{project.ProjectID, project.Config.APIKey, project.Config.APISecret, project.Config.Endpoint, project.BrowserURL} {
			if secret != "" && strings.Contains(err.Error(), secret) {
				t.Fatalf("config error exposed supplied value")
			}
		}
	}
}
