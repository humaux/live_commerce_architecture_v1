// live_flow_test.go covers the A5 route registration DB-free: the studio + live-flow families are
// mounted on one mux exactly as NewHandler will (Studio on, with and without the page-live-videos
// jobs client), so a ServeMux pattern ambiguity panics here instead of at first use inside the
// real-PG gate (the round-1 panic was a methodless results fallback colliding with studio's
// "GET /live-sessions/{session_id}").
package httpapi

import (
	"net/http"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func TestLiveFlowRouteRegistration(t *testing.T) {
	mux := http.NewServeMux()
	registerStudioRoutes(mux, nil, true, nil, nil)
	registerLiveFlowRoutes(mux, nil, true, nil)

	jobs, err := river.NewClient(riverpgxv5.New(nil), &river.Config{Schema: "river"})
	if err != nil {
		t.Fatal(err)
	}
	mux = http.NewServeMux()
	registerStudioRoutes(mux, nil, true, nil, nil)
	registerLiveFlowRoutes(mux, nil, true, jobs)
}
