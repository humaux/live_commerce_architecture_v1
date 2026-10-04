package httpapi

// images_test.go: transport guards of the product-photo routes that run before any SQL (nil pool is never reached):
// bearer syntax, multipart shape, authenticate-before-body ordering, the single `file` part, the 2 MiB cap, and the
// raw-bytes response headers. The body-shape guards (field name, part count, size caps) run against readSingleFile
// directly: the handler now authenticates (catalog:write) before it reads the body, so a nil-pool handler refuses
// them with 401 before the parser is reached.

import (
	"bytes"
	"io"
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

// TestUploadRefusesUnauthenticatedBeforeBodyRead is the security-ordering gate: for every refusal that happens at
// or before the authorization step, the handler must answer without consuming the request body. The body fails the
// test on the first Read, so any regression to read-then-authenticate is a hard failure, not a silent 401 after
// buffering up to 2 MiB.
func TestUploadRefusesUnauthenticatedBeforeBodyRead(t *testing.T) {
	h := NewHandler(nil)
	small, content := multipartBody(t, map[string][]byte{"file": []byte("x")}, "file")
	for _, tt := range []struct {
		name, auth string
		want       int
	}{
		{"no bearer", "", 401},
		{"malformed bearer", "Bearer a b", 401},
		// well-formed bearer, but the pool is nil: platform.WithScope refuses before any body read. With the old
		// read-first ordering this case consumed the multipart body before answering 401.
		{"unauthorized bearer", "Bearer " + strings.Repeat("b", 43), 401},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, imagesPath, small)
			r.Header.Set("Content-Type", content)
			if tt.auth != "" {
				r.Header.Set("Authorization", tt.auth)
			}
			r.Body = &failingReadCloser{t: t, ReadCloser: r.Body}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.want {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
		})
	}
}

type failingReadCloser struct {
	t *testing.T
	io.ReadCloser
}

func (f *failingReadCloser) Read([]byte) (int, error) {
	f.t.Fatal("request body was read before authentication")
	return 0, io.EOF
}

// TestReadSingleFileGuards pins the multipart parser refusals at the layer where they happen (the handler reaches
// them only after authorization): 413 when the body or the `file` part is over the cap, 422 for any other
// malformation (no part, another field name, a second part, an empty body).
func TestReadSingleFileGuards(t *testing.T) {
	small := map[string][]byte{"file": []byte("x"), "other": []byte("y")}
	for _, tt := range []struct {
		name string
		body func() (*bytes.Buffer, string)
		want int
	}{
		{"wrong field name", func() (*bytes.Buffer, string) { return multipartBody(t, small, "other") }, 422},
		{"two parts", func() (*bytes.Buffer, string) { return multipartBody(t, small, "file", "other") }, 422},
		{"empty body", func() (*bytes.Buffer, string) { return &bytes.Buffer{}, "multipart/form-data; boundary=x" }, 422},
		{"file over 2 MiB", func() (*bytes.Buffer, string) {
			return multipartBody(t, map[string][]byte{"file": make([]byte, catalog.MaxImageBytes+1)}, "file")
		}, 413},
		{"body over cap", func() (*bytes.Buffer, string) {
			return multipartBody(t, map[string][]byte{"file": make([]byte, maxUploadBody+1)}, "file")
		}, 413},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body, content := tt.body()
			r := httptest.NewRequest(http.MethodPost, imagesPath, body)
			r.Header.Set("Content-Type", content)
			r.Body = http.MaxBytesReader(httptest.NewRecorder(), r.Body, maxUploadBody)
			if _, status := readSingleFile(r); status != tt.want {
				t.Fatalf("status %d, want %d", status, tt.want)
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
