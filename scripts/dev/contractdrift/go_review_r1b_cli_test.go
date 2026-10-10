// Purpose: pin PR33 r1b methodless handlers and unknown range tails to actual CLI exit codes.
// Depends on: Go stdlib, compiled reviewCLI process and Git-backed source fixtures.
// Used by: CI-DRIFT regression gates and the parent integrator's independent review.
// Invariant: known prefix routes never hide unknown registrations or guessed child coverage.
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGoReviewR1BInlineMethodlessCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, tc := range []struct{ name, registration, marker string }{
		{"Handle", `mux.Handle("/v1/review_inline", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))`, "MOUNT"},
		{"HandleFunc", `mux.HandleFunc("/v1/review_inline", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })`, "EXACT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t, "aligned")
			p := filepath.Join(root, "internal/httpapi/routes.go")
			mustWrite(t, p, readText(t, p)+"\nfunc reviewInline(mux *http.ServeMux) { "+tc.registration+" }\n")
			code, out := call(root)
			if code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED "+tc.marker+" /v1/review_inline") {
				t.Fatalf("inline business handler escaped: exit=%d %s", code, out)
			}
		})
	}
}

func TestGoReviewR1BHandle405BodyCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"static405", `w.WriteHeader(http.StatusMethodNotAllowed)`, 0},
		{"first200", `w.WriteHeader(http.StatusOK); w.WriteHeader(http.StatusMethodNotAllowed)`, 1},
		{"dynamicStatus", `w.WriteHeader(statusCode())`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t, "aligned")
			p := filepath.Join(root, "internal/httpapi/routes.go")
			mustWrite(t, p, readText(t, p)+"\nfunc review405(mux *http.ServeMux) { mux.Handle(\"/v1/review_405\", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { "+tc.body+" })) }\nfunc statusCode() int { return 405 }\n")
			code, out := call(root)
			if code != tc.code || (code == 1 && !strings.Contains(out, "UNRESOLVED MOUNT /v1/review_405")) || (code == 0 && strings.Contains(out, "UNRESOLVED MOUNT /v1/review_405")) {
				t.Fatalf("405 proof crossed actual body/import boundary: exit=%d %s", code, out)
			}
		})
	}
}

func TestGoReviewR1BUnknownRangeTailCLI(t *testing.T) {
	call := reviewCLI(t)
	for _, tc := range []struct{ name, setup string }{
		{"emptyAppend", `paths := append([]string{}, dynamicPaths()...)`},
		{"knownAppend", `paths := append([]string{rootPrefix+"/widgets/{widget_id}"}, dynamicPaths()...)`},
		{"knownUnknownBranch", `paths := []string{}; if enabled { paths = []string{rootPrefix+"/widgets/{widget_id}"} } else { paths = dynamicPaths() }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t, "aligned")
			p := filepath.Join(root, "internal/httpapi/routes.go")
			mustWrite(t, p, readText(t, p)+"\nfunc reviewRange(mux *http.ServeMux, enabled bool) {\n "+tc.setup+"\n for _, p := range paths { mux.HandleFunc(\"GET \"+p, handler) }\n}\nfunc dynamicPaths() []string { return []string{\"/v1/review_dynamic\"} }\n")
			code, out := call(root)
			if code != 1 || !strings.Contains(out, "ERROR (TOUCHED) UNRESOLVED") || !strings.Contains(out, "Go mux registration pattern cannot be statically resolved") {
				t.Fatalf("known items hid unknown range tail: exit=%d %s", code, out)
			}
		})
	}
}
