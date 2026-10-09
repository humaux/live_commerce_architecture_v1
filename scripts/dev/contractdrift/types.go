// Purpose: share normalized route/source records across the three independent extractors.
// Depends on: Go stdlib strings and regexp; no application imports.
// Used by: Go/contracts/BFF parsers, comparison, baseline ratchet and CLI.
package main

import (
	"regexp"
	"strings"
)

type location struct {
	File string `json:"file"`
	Line int    `json:"line"`
	End  int    `json:"end,omitempty"`
}
type route struct {
	Method    string     `json:"method"`
	Path      string     `json:"path"`
	Locations []location `json:"locations"`
}
type finding struct {
	Kind      string     `json:"kind"`
	Method    string     `json:"method,omitempty"`
	Path      string     `json:"path,omitempty"`
	Detail    string     `json:"detail,omitempty"`
	Locations []location `json:"locations"`
}
type inventory struct {
	Routes     []route
	Unresolved []finding
	Notes      []finding
}

var param = regexp.MustCompile(`\{[^/{}]+\}|\[\[?[^/\]]+\]\]?|:[A-Za-z_][A-Za-z_0-9]*`)

func normalizePath(path string) string {
	path = strings.SplitN(path, "?", 2)[0]
	return param.ReplaceAllString(path, "{}")
}
func addRoute(out *inventory, method, path string, refs ...location) {
	out.Routes = append(out.Routes, route{strings.ToUpper(method), normalizePath(path), refs})
}
func addUnresolved(out *inventory, ref location, detail string) {
	out.Unresolved = append(out.Unresolved, finding{Kind: "UNRESOLVED", Detail: detail, Locations: []location{ref}})
}
