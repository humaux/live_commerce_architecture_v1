// Purpose: classify three independent inventories and enforce a shrinking, touched-line-aware legacy baseline.
// Depends on: Go stdlib JSON/sorting and git source ranges; baseline never suppresses current source changes.
// Used by: CI-DRIFT CLI and git-backed fixture tests; no API mutation.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type baseline struct {
	Version int       `json:"version"`
	Seed    string    `json:"seed"`
	Entries []finding `json:"entries"`
}

var mismatchKinds = map[string]bool{"GO_ONLY": true, "CONTRACT_ONLY": true, "BFF_NO_GO": true, "METHOD_MISMATCH": true}

// Internal-only namespaces are annotations, not suppression: their Go/contract drift still fails.
var internalOnlyPrefixes = []string{"/health", "/ready", "/v1/meta/", "/v1/webhooks/", "/v1/cvs/ecpay/", "/v1/worker/", "/internal/"}

func routeKey(r route) string     { return r.Method + " " + r.Path }
func findingKey(f finding) string { return f.Kind + " " + f.Method + " " + f.Path }
func locations(in []location) []location {
	seen := map[location]bool{}
	out := []location{}
	for _, r := range in {
		if r.Line < 1 || r.File == "" {
			continue
		}
		if r.End < r.Line {
			r.End = r.Line
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.End < b.End
	})
	return out
}
func routeMap(in inventory) map[string]route {
	out := map[string]route{}
	for _, r := range in.Routes {
		k := routeKey(r)
		p := out[k]
		p.Method, p.Path = r.Method, r.Path
		p.Locations = locations(append(p.Locations, r.Locations...))
		out[k] = p
	}
	return out
}
func methodSet(in map[string]route, path string) string {
	methods := []string{}
	for _, r := range in {
		if r.Path == path {
			methods = append(methods, r.Method)
		}
	}
	sort.Strings(methods)
	return strings.Join(methods, ",")
}
func compare(goInv, contracts, bff inventory) []finding {
	goRoutes, docRoutes, bffRoutes := routeMap(goInv), routeMap(contracts), routeMap(bff)
	out := []finding{}
	paths := map[string]bool{}
	opaqueRefs := func(path string) []location {
		refs := []location{}
		for _, f := range goInv.Unresolved {
			if f.Path == "" {
				continue
			}
			if f.Method == "PREFIX" && strings.HasPrefix(path, f.Path) || f.Method == "EXACT" && path == f.Path {
				refs = append(refs, f.Locations...)
			}
		}
		return refs
	}
	addMissing := func(a, b map[string]route, kind string) {
		for key, r := range a {
			paths[r.Path] = true
			if _, ok := b[key]; !ok {
				refs := opaqueRefs(r.Path)
				if (kind == "CONTRACT_ONLY" || kind == "BFF_NO_GO") && len(refs) > 0 {
					out = append(out, finding{Kind: "UNRESOLVED", Method: r.Method, Path: r.Path, Detail: "Go custom dispatch is opaque; absence is not proof of an unserved route", Locations: locations(append(refs, r.Locations...))})
				} else {
					out = append(out, finding{Kind: kind, Method: r.Method, Path: r.Path, Locations: r.Locations})
				}
			}
		}
	}
	addMissing(goRoutes, docRoutes, "GO_ONLY")
	addMissing(docRoutes, goRoutes, "CONTRACT_ONLY")
	addMissing(bffRoutes, goRoutes, "BFF_NO_GO")
	addMissing(goRoutes, bffRoutes, "GO_NOT_IN_BFF")
	for path := range paths {
		g, d, b := methodSet(goRoutes, path), methodSet(docRoutes, path), methodSet(bffRoutes, path)
		bad := g != "" && d != "" && g != d
		for _, method := range strings.Split(b, ",") {
			if method != "" && g != "" {
				if _, ok := goRoutes[method+" "+path]; !ok {
					bad = true
				}
			}
		}
		if bad {
			refs := []location{}
			for _, m := range []map[string]route{goRoutes, docRoutes, bffRoutes} {
				for _, r := range m {
					if r.Path == path {
						refs = append(refs, r.Locations...)
					}
				}
			}
			if opaque := opaqueRefs(path); len(opaque) > 0 {
				out = append(out, finding{Kind: "UNRESOLVED", Method: "go=" + g + ";contract=" + d + ";bff=" + b, Path: path, Detail: "partial Go methods under opaque dispatch cannot establish METHOD_MISMATCH", Locations: locations(append(refs, opaque...))})
			} else {
				out = append(out, finding{Kind: "METHOD_MISMATCH", Method: "go=" + g + ";contract=" + d + ";bff=" + b, Path: path, Locations: locations(refs)})
			}
		}
	}
	for _, inv := range []inventory{goInv, contracts, bff} {
		out = append(out, inv.Unresolved...)
		out = append(out, inv.Notes...)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := findingKey(out[i]), findingKey(out[j])
		if a != b {
			return a < b
		}
		return fmt.Sprint(out[i].Locations) < fmt.Sprint(out[j].Locations)
	})
	return out
}

// Previously exposed BFF mappings cannot disappear behind aggregate backend-only INFO or another producer.
// Complete retirement requires deleting the formerly present Go route and contract in the same PR.
func bffRegressions(base, now sourceInventories) []finding {
	oldGo, oldDocs := routeMap(base.Go), routeMap(base.Contracts)
	goRoutes, docRoutes := routeMap(now.Go), routeMap(now.Contracts)
	out := []finding{}
	for producer, routes := range base.BFF.Producers {
		current := routeMap(inventory{Routes: now.BFF.Producers[producer]})
		for key, before := range routeMap(inventory{Routes: routes}) {
			if _, ok := current[key]; ok {
				continue
			}
			_, wasGo := oldGo[key]
			_, wasDoc := oldDocs[key]
			_, stillGo := goRoutes[key]
			_, stillDoc := docRoutes[key]
			if wasGo && wasDoc && !stillGo && !stillDoc {
				continue
			}
			refs := append([]location(nil), before.Locations...)
			for _, after := range current {
				refs = append(refs, after.Locations...)
			}
			out = append(out, finding{Kind: "BFF_REMOVED", Method: before.Method, Path: before.Path,
				Detail: "merge-base forwarding mapping removed or retargeted: " + producer, Locations: locations(refs)})
		}
	}
	return out
}
func decodeBaseline(data []byte) (baseline, error) {
	var b baseline
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&b); err != nil {
		return b, err
	}
	if _, err := d.Token(); err != io.EOF {
		return b, fmt.Errorf("trailing baseline JSON")
	}
	if b.Version != 1 {
		return b, fmt.Errorf("unsupported baseline version")
	}
	seen := map[string]bool{}
	for _, f := range b.Entries {
		k := findingKey(f)
		if !mismatchKinds[f.Kind] || f.Method == "" || f.Path == "" || len(f.Locations) == 0 || seen[k] {
			return b, fmt.Errorf("invalid or duplicate baseline entry %q", k)
		}
		seen[k] = true
	}
	return b, nil
}
func readBaseline(path string) (baseline, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return baseline{Version: 1}, false, nil
	}
	if err != nil {
		return baseline{}, false, err
	}
	b, err := decodeBaseline(data)
	return b, true, err
}
func writeBaseline(path string, b baseline) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".baseline-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(append(data, '\n')); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return err
	}
	if err = os.Chmod(name, 0644); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func baselineMap(b baseline) map[string]finding {
	out := map[string]finding{}
	for _, f := range b.Entries {
		out[findingKey(f)] = f
	}
	return out
}
