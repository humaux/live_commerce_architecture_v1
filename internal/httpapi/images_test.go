package httpapi

// images_test.go: transport guards of the product-photo routes that run before any SQL (nil pool is never reached):
// bearer syntax, multipart shape, the single `file` part, the 2 MiB cap, and the raw-bytes response headers.

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"livecommerce/internal/catalog"
)

const imagesPath = "/v1/admin/stores/11111111-1111-4111-8111-111111111111/products/22222222-2222-4222-8222-222222222222/images"

func multipartBody(t *testing.T, parts map[string][]byte, order ...string) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for _, name := range order {
		fw, err := w.CreateFormFile(name, "../../etc/passwd.svg") // hostile filename must be ignored
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(parts[name])
	}
	_ = w.Close()
	return &b, w.FormDataContentType()
}

func TestUploadImageTransportGuards(t *testing.T) {
	h := NewHandler(nil)
	bearer := "Bearer " + strings.Repeat("a", 43)
	small := map[string][]byte{"file": []byte("x"), "other": []byte("y")}
	form := func(parts map[string][]byte, order ...string) func() (*bytes.Buffer, string) {
		return func() (*bytes.Buffer, string) { return multipartBody(t, parts, order...) }
	}
	for _, tt := range []struct {
		name, auth string
		body       func() (*bytes.Buffer, string)
		want       int
	}{
		{"no bearer", "", form(small, "file"), 401},
		{"spaces in bearer", "Bearer a b", form(small, "file"), 401},
		{"json is not multipart", bearer, func() (*bytes.Buffer, string) { return bytes.NewBufferString("{}"), "application/json" }, 415},
		{"wrong field name", bearer, form(small, "other"), 422},
		{"two parts", bearer, form(small, "file", "other"), 422},
		{"empty body", bearer, func() (*bytes.Buffer, string) { return &bytes.Buffer{}, "multipart/form-data; boundary=x" }, 422},
		{"file over 2 MiB", bearer, form(map[string][]byte{"file": make([]byte, catalog.MaxImageBytes+1)}, "file"), 413},
		{"body over cap", bearer, form(map[string][]byte{"file": make([]byte, maxUploadBody+1)}, "file"), 413},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body, content := tt.body()
			r := httptest.NewRequest(http.MethodPost, imagesPath, body)
			r.Header.Set("Content-Type", content)
			if tt.auth != "" {
				r.Header.Set("Authorization", tt.auth)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestImageReadAndWriteRoutesRequireLogin(t *testing.T) {
	h := NewHandler(nil)
	for _, tt := range []struct{ method, path string }{
		{"GET", imagesPath}, {"GET", imagesPath + "/33333333-3333-4333-8333-333333333333"},
		{"POST", imagesPath + "/33333333-3333-4333-8333-333333333333/delete"}, {"POST", imagesPath + "/order"},
	} {
		r := httptest.NewRequest(tt.method, tt.path, strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Errorf("%s %s: status %d", tt.method, tt.path, w.Code)
		}
	}
}

func TestRawResponseHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	respond(w, 200, rawResponse{contentType: "image/png", body: []byte("png")})
	if w.Code != 200 || w.Body.String() != "png" || w.Header().Get("Content-Type") != "image/png" ||
		w.Header().Get("Cache-Control") != "private, max-age=300" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("raw response %d %v", w.Code, w.Header())
	}
}
