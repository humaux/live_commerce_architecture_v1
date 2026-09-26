package meta

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWebhookDispatchOnlyLiteralConfiguredPaths(t *testing.T) {
	const page = "/v1/meta/webhooks/123/page"
	const instagram = "/v1/meta/webhooks/123/instagram"
	called := ""
	router := webhookDispatch(map[string]http.Handler{
		page: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = "page"
			w.WriteHeader(http.StatusAccepted)
		}),
		instagram: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = "instagram"
			w.WriteHeader(http.StatusAccepted)
		}),
	})
	for _, tc := range []struct{ target, want string }{{page, "page"}, {instagram, "instagram"}} {
		called = ""
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.target, nil))
		if w.Code != http.StatusAccepted || called != tc.want {
			t.Fatal("literal configured path not dispatched")
		}
	}
	for _, target := range []string{
		page + "/", "/v1/meta/webhooks/123/other", "/v1/meta/webhooks/124/page",
		"/v1/meta//webhooks/123/page", "/v1/meta/./webhooks/123/page",
		"/v1/meta/x/../webhooks/123/page", "/v1/meta/webhooks/123%2Fpage",
		"/v1/%6deta/webhooks/123/page", "/v1/meta/webhooks/123/%70age",
	} {
		t.Run(target, func(t *testing.T) {
			called = ""
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, target, nil))
			if w.Code != http.StatusNotFound || called != "" || w.Header().Get("Location") != "" {
				t.Fatal("nonliteral path dispatched or redirected")
			}
		})
	}
	request := httptest.NewRequest(http.MethodGet, page, nil)
	request.URL.RawPath = "/v1/meta/webhooks/123/%ZZ"
	called = ""
	w := httptest.NewRecorder()
	router.ServeHTTP(w, request)
	if w.Code != http.StatusNotFound || called != "" {
		t.Fatal("noncanonical raw path dispatched")
	}
}

func TestRuntimeConstructorsRejectInvalidBeforeDatabase(t *testing.T) {
	if _, err := NewWebhookRouter(nil, nil, nil, nil); !errors.Is(err, ErrRuntimeConfig) {
		t.Fatal("router accepted missing dependencies")
	}
	if _, err := NewWebhookRouter(context.Background(), nil, &PayloadKeyring{}, []WebhookEndpoint{{Path: "forged"}}); !errors.Is(err, ErrRuntimeConfig) {
		t.Fatal("router accepted missing pool")
	}
	if _, err := NewConsumerClient(nil, nil, nil, nil, 4); !errors.Is(err, ErrRuntimeConfig) {
		t.Fatal("consumer client accepted missing dependencies")
	}
	if _, err := NewConsumerClient(context.Background(), nil, nil, &PayloadKeyring{}, 17); !errors.Is(err, ErrRuntimeConfig) {
		t.Fatal("consumer client accepted invalid concurrency")
	}
}

func TestWebhookRouterRejectsDuplicateAndForgedEndpointsBeforeDatabase(t *testing.T) {
	first, err := NewVerifier(Config{AppID: "123", Object: "page", AppSecret: "1234567890abcdef", VerifyToken: "abcdef1234567890"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewVerifier(Config{AppID: "123", Object: "page", AppSecret: "abcdef1234567890", VerifyToken: "1234567890abcdef"})
	if err != nil {
		t.Fatal(err)
	}
	pool := &pgxpool.Pool{} // No call may reach this unopened pool.
	for _, endpoints := range [][]WebhookEndpoint{
		{{Path: "/v1/meta/webhooks/123/page", Verifier: first}, {Path: "/v1/meta/webhooks/123/page", Verifier: second}},
		{{Path: "/v1/meta/webhooks/124/page", Verifier: first}},
		{{Path: "/v1/meta/webhooks/123/page/", Verifier: first}},
	} {
		if _, err := NewWebhookRouter(context.Background(), pool, &PayloadKeyring{}, endpoints); !errors.Is(err, ErrRuntimeConfig) {
			t.Fatal("duplicate or forged endpoint reached database")
		}
	}
}
