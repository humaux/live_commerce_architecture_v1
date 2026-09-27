package live

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"livecommerce/internal/integrations/livekit"
)

type recoveryRoundTrip func(*http.Request) (*http.Response, error)

func (f recoveryRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRecoveryEgressWithoutIDUsesRoomDiscovery(t *testing.T) {
	const room = "lc_0123456789abcdef0123456789abcdef"
	const egress = "EG_recovered_1"
	var path string
	client, err := livekit.New(livekit.Config{Environment: "MOCK", Endpoint: "https://tenant.livekit.cloud",
		APIKey: "fixture_key", APISecret: strings.Repeat("x", 32), StreamHosts: []string{"ingest.example.com"}},
		recoveryRoundTrip(func(r *http.Request) (*http.Response, error) {
			path = r.URL.Path
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
				`{"items":[{"egress_id":"` + egress + `","room_name":"` + room + `","status":"EGRESS_ACTIVE","started_at":"123","updated_at":456,"ended_at":"0"}]}`))}, nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	source, observation, err := observeRecoveryEgress(context.Background(), client, room, "")
	if err != nil || source != "ROOM" || observation.EgressID != egress || path != "/twirp/livekit.Egress/ListEgress" {
		t.Fatalf("missing Egress ID did not discover exact room: source=%s obs=%+v path=%s err=%v", source, observation, path, err)
	}
}

func TestRecoveryEgressOnlyDoesNotRequireInputProject(t *testing.T) {
	key := mediaProjectKey{"project", 1}
	client := &livekit.Client{}
	r := &MediaRecoveryObserver{projects: map[mediaProjectKey]mediaEndpoint{
		key: {identity: "https://tenant.livekit.cloud", client: client},
	}, input: &BrowserInputRuntime{projects: map[mediaProjectKey]browserInputEndpoint{}}, withInput: true}
	_, _, ok := r.recoveryEndpoints(key, "https://tenant.livekit.cloud", false)
	if !ok {
		t.Fatal("Egress-only member incorrectly required browser input project")
	}
	_, _, ok = r.recoveryEndpoints(key, "https://tenant.livekit.cloud", true)
	if ok {
		t.Fatal("input-required member accepted absent browser input project")
	}
	_, _, ok = r.recoveryEndpoints(key, "https://other.livekit.cloud", false)
	if ok {
		t.Fatal("mismatched endpoint accepted")
	}
}
