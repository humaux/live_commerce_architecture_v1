package metaconnect

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	metaoauth "livecommerce/internal/integrations/meta/oauth"
)

func TestR11PageConnectAllowsDeclinedReadInsights(t *testing.T) {
	old := granted("pages_show_list", "pages_manage_metadata", "pages_read_engagement", "pages_messaging")
	a := accountDoc{ID: "11", Tasks: []string{"MESSAGING", "MODERATE"}}
	e, ok := entryOf(a, old)
	if !ok || len(e.Missing) != 0 {
		t.Fatalf("declined optional insights must retain Page eligibility: %+v", e)
	}
	old["read_insights"] = true
	e, ok = entryOf(a, old)
	if !ok || len(e.Missing) != 0 {
		t.Fatalf("new grant: %+v", e)
	}
}

func TestR11OptionalReadInsightsSurvivesGrantedPickResult(t *testing.T) {
	for _, state := range []string{"granted", "declined"} {
		t.Run(state, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v26.0/me/permissions":
					permissions := []map[string]string{}
					for _, name := range []string{"pages_show_list", "pages_manage_metadata", "pages_read_engagement", "pages_messaging"} {
						permissions = append(permissions, map[string]string{"permission": name, "status": "granted"})
					}
					permissions = append(permissions, map[string]string{"permission": "read_insights", "status": state})
					json.NewEncoder(w).Encode(map[string]any{"data": permissions})
				case "/v26.0/me/accounts":
					io.WriteString(w, `{"data":[{"id":"11","name":"Synthetic Shop","tasks":["MESSAGING","MODERATE"]}]}`)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			t.Cleanup(srv.Close)
			g, err := metaoauth.NewGraph(srv.URL, "v26.0", nil)
			if err != nil {
				t.Fatal(err)
			}
			s := &Service{graph: g}
			pages, scopes, err := s.listPages(context.Background(), []byte("synthetic-grant"))
			if err != nil || len(pages) != 1 || len(pages[0].Missing) != 0 {
				t.Fatalf("decline must not block pick: %+v %v", pages, err)
			}
			if slices.Contains(scopes, "read_insights") != (state == "granted") {
				t.Fatalf("stored grant must preserve optional capability: %v", scopes)
			}
		})
	}
}
