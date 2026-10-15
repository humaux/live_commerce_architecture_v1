// Purpose: reproduce frozen-value escapes through the built production CLI.
// Depends on: Git fixtures and the independently pinned pre-mutation dispatcher.
// Used by: CI; local Go tests are forbidden for this unit.
// Invariant: every escape gives a positioned freeze ERROR, even before pin comparison.

package main

import (
	"strings"
	"testing"
)

func TestGoDispatcherFrozenValueEscapeCLI(t *testing.T) {
	cases := []struct {
		name, body, extra string
		inside            bool
	}{
		{"pointer alias New", `p := gate; *p = *regexp.MustCompile(".*");`, "", false},
		{"pointer helper", "widen(gate);", `func widen(p *regexp.Regexp){*p = *regexp.MustCompile(".*")}`, false},
		{"pointer method", "gate.Longest();", "", false},
		{"slice alias", `c := columns; c[0] = "sku";`, "", false},
		{"slice helper", "clobber(columns);", `func clobber(p []string){p[0]="sku"}`, false},
		{"return escape", "_ = leak();", "func leak() *regexp.Regexp{return gate}", true},
		{"goroutine capture", "go func(){gate.Longest()}();", "", true},
		{"inside mutating method", "gate.Longest();", "", true},
		{"inside alias", "p := gate; p.Longest();", "", true},
		{"inside field store", "sink.P = gate;", "var sink struct{P *regexp.Regexp}", true},
		{"inside composite", "_ = []*regexp.Regexp{gate};", "", true},
		{"inside channel", "ch := make(chan *regexp.Regexp,1); ch <- gate;", "", true},
		{"inside unknown call", "clobber(columns);", `func clobber(p []string){p[0]="sku"}`, true},
		{"nonfrozen initializer", "", "var alias = gate", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, p := routeTableCLIFixture(t)
			source := mustReplace(t, routeTableCLISource, `"net/http";`, `"net/http"; "regexp";`)
			source = mustReplace(t, source, "selected := match(r.URL.Path)", `_ = gate.MatchString("x"); _ = len(columns); selected := match(r.URL.Path)`)
			source += `
var gate = regexp.MustCompile("x")
var columns = []string{"id"}
`
			mustWrite(t, p, source)
			pinFixtureDispatcher(t, root, "router.Routes", "")
			assertFreezeCLI(t, root, true)
			anchor := "func New() http.Handler { "
			if tc.inside {
				anchor = "selected := match(r.URL.Path)"
			}
			if tc.body != "" {
				replacement := tc.body + anchor
				if !tc.inside {
					replacement = anchor + tc.body
				}
				source = mustReplace(t, source, anchor, replacement)
			}
			mustWrite(t, p, source+"\n"+tc.extra+"\n")
			code, out := goReviewProcessCLI(t, root)
			if code != 1 || !strings.Contains(out, "ERROR (dispatcher freeze)") || !strings.Contains(out, "frozen var") || !strings.Contains(out, "routes.go:") {
				t.Fatalf("escape accepted or unpositioned: exit%d %s", code, out)
			}
		})
	}
}
