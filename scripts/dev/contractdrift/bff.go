// Purpose: enumerate BFF forwarding routes from TypeScript source, independently of Go routes.
// Depends on: bff_ts.go finite lexer/resolver and shared inventory types; used by CI-DRIFT CLI.
// Invariant: guard-only proxies and local handlers are not route producers. Unsupported
// forwarding/grammar shapes produce UNRESOLVED, never a guessed finite inventory.
//
// Mapping table (upstream METHOD, query stripped):
// admin stores catchall: /api/stores/[store]/X -> /v1/admin/stores/{}/X.
// admin tools catchall: /api/stores/[store]/tools/X -> same prefix + X (tools stripped).
// admin reports/picklist/customer/import leaves: closed helper admission + same store prefix.
// admin identity: privateIdentity suffix -> /v1/identity/suffix, always POST; GET callback -> POST.
// admin meta/ads leaves: callBackend suffix -> store prefix (server-selected store).
// admin stores list: authenticatedStores -> GET /v1/admin/stores.
// storefront buyer: admitted privatePath -> /v1/buyer/privatePath; session prepare is local,
// activate -> bootstrap; reset/logout -> retire, read -> session, methods from sinks.
// storefront shop leaves: reached catalog helper path -> /v1/buyer/catalog/v2/products[/{}].
// storefront proxy: reached primary-origin helper -> GET /v1/buyer/storefront/primary-origin.
// admin proxy: recognition-only URL guards; no upstream inventory of its own.
// Package main runs the CI contract drift gate without application dependencies.
package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type bffScanner struct {
	reader *bffReader
	out    inventory
	seen   map[string]bool
}

const bffStorePrefix = "/v1/admin/stores/{}/"

func scanBFF(root string) (inventory, error) {
	s := &bffScanner{reader: &bffReader{root: root, modules: map[string]*bffModule{}}, seen: map[string]bool{}}
	err := filepath.WalkDir(filepath.Join(root, "apps"), func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			if p == filepath.Join(root, "apps") {
				return nil
			}
			return e
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".next" {
				return filepath.SkipDir
			}
			return nil
		}
		name, _ := filepath.Rel(root, p)
		name = filepath.ToSlash(name)
		if strings.Contains(name, "/app/api/") && strings.HasSuffix(name, "/route.ts") {
			m, e := s.reader.load(name)
			if e != nil {
				return e
			}
			s.leaf(m)
		}
		return nil
	})
	if err != nil {
		return s.out, err
	}
	if m, e := s.reader.load("apps/storefront/proxy.ts"); e == nil {
		s.primaryOrigin(m)
	}
	if m, e := s.reader.load("apps/admin/proxy.ts"); e == nil {
		for _, sink := range []string{"fetch", "callBackend", "privateIdentity", "merchantBackend"} {
			if len(bffCalls(m, m.tokens, sink)) > 0 {
				s.directSinks(m, bffExports(m), bffLeafRefs(m)...)
				break
			}
		}
	}
	s.dedupe()
	return s.out, nil
}
func (s *bffScanner) dedupe() {
	routes := map[string]route{}
	for _, r := range s.out.Routes {
		k := r.Method + " " + r.Path
		if prev, ok := routes[k]; ok {
			prev.Locations = append(prev.Locations, r.Locations...)
			routes[k] = prev
		} else {
			routes[k] = r
		}
	}
	s.out.Routes = nil
	keys := []string{}
	for k := range routes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r := routes[k]
		refs := map[location]bool{}
		var clean []location
		for _, l := range r.Locations {
			if l.Line > 0 && !refs[l] {
				refs[l] = true
				clean = append(clean, l)
			}
		}
		sort.Slice(clean, func(i, j int) bool {
			if clean[i].File != clean[j].File {
				return clean[i].File < clean[j].File
			}
			return clean[i].Line < clean[j].Line
		})
		r.Locations = clean
		s.out.Routes = append(s.out.Routes, r)
	}
}
func (s *bffScanner) emit(method, prefix string, v bffValue, refs ...location) {
	if len(v.texts) == 0 {
		for _, l := range append(v.refs, refs...) {
			addUnresolved(&s.out, l, "non-finite BFF route expression")
		}
		return
	}
	for _, pattern := range v.texts {
		if strings.HasPrefix(pattern, "!UNRESOLVED:") {
			for _, l := range append(v.refs, refs...) {
				addUnresolved(&s.out, l, strings.TrimPrefix(pattern, "!UNRESOLVED:"))
			}
			continue
		}
		paths := []string{pattern}
		var e error
		if v.re {
			paths, e = bffExpand(pattern)
		}
		if e != nil {
			for _, l := range append(v.refs, refs...) {
				addUnresolved(&s.out, l, "BFF grammar: "+e.Error())
			}
			continue
		}
		for _, p := range paths {
			if prefix != "" {
				p = prefix + strings.TrimPrefix(p, "/")
			}
			addRoute(&s.out, method, p, append(append([]location{}, refs...), v.refs...)...)
		}
	}
}
func (s *bffScanner) evalEmit(m *bffModule, t []bffToken, method, prefix string, refs ...location) {
	v, e := s.reader.eval(m, t, nil, 0)
	if e != nil {
		addUnresolved(&s.out, m.ref(t), e.Error())
		return
	}
	s.emit(method, prefix, v, refs...)
}
func bffExports(m *bffModule) map[string]bool {
	out := map[string]bool{}
	t := m.tokens
	for i := 0; i < len(t); i++ {
		if t[i].text != "export" {
			continue
		}
		j := i + 1
		for j < len(t) && j < i+5 {
			if bffHTTP(t[j].text) {
				out[t[j].text] = true
				break
			}
			j++
		}
	}
	return out
}
func bffCalls(m *bffModule, t []bffToken, name string) [][]bffToken {
	var out [][]bffToken
	for i := 0; i+1 < len(t); i++ {
		if t[i].text == name && t[i+1].text == "(" && (i == 0 || t[i-1].text != "function") {
			j := bffClose(t, i+1)
			if j >= 0 {
				out = append(out, t[i+2:j])
			}
		}
	}
	return out
}
func (s *bffScanner) bindingFunc(m *bffModule, name string) (*bffModule, bffExpr, error) {
	if f, ok := m.funcs[name]; ok {
		return m, f, nil
	}
	if d, ok := m.defs[name]; ok {
		for i, t := range d.tokens {
			if t.text == "=>" {
				return m, bffExpr{d.tokens[i+1:], d.ref}, nil
			}
		}
	}
	if i, ok := m.imports[name]; ok {
		other, e := s.reader.load(i[0])
		if e != nil {
			return nil, bffExpr{}, e
		}
		return s.bindingFunc(other, i[1])
	}
	return nil, bffExpr{}, fmt.Errorf("unknown admission helper %s", name)
}
func (s *bffScanner) admit(m *bffModule, name, prefix string, refs ...location) {
	key := m.name + ":" + name + ":" + prefix
	if s.seen[key] {
		return
	}
	s.seen[key] = true
	owner, f, e := s.bindingFunc(m, name)
	if e != nil {
		addUnresolved(&s.out, m.ref(m.tokens), e.Error())
		return
	}
	m = owner
	refs = append(refs, f.ref)
	count := len(s.out.Routes)
	t := f.tokens
	// Closed method object or [method, regexp, kind] tuple tables referenced by the admission function.
	for name, d := range m.defs {
		referenced := false
		for _, x := range t {
			if x.text == name {
				referenced = true
				break
			}
		}
		if !referenced {
			continue
		}
		if len(d.tokens) == 0 {
			continue
		}
		if d.tokens[0].text != "{" && d.tokens[0].text != "[" {
			continue
		}
		v, e := s.reader.eval(m, d.tokens, nil, 0)
		if e != nil {
			addUnresolved(&s.out, d.ref, e.Error())
			continue
		}
		s.table(m, v, prefix, append(refs, d.ref)...)
	}
	// Direct method predicates and regex tests, including method ternary read/write grammars.
	for i := 0; i+2 < len(t); i++ {
		if t[i].text != "method" || t[i+1].text != "===" || t[i+2].kind != "string" || !bffHTTP(bffLiteral(t[i+2])) {
			continue
		}
		method := bffLiteral(t[i+2])
		end := i + 3
		for end < len(t) && t[end].text != ";" && t[end].text != "return" && t[end].text != ":" {
			end++
		}
		frag := t[i+3 : end]
		knownPredicate := false
		for j := 0; j < len(frag); j++ {
			if frag[j].text == "path" && j+2 < len(frag) && frag[j+1].text == "===" && frag[j+2].kind == "string" {
				s.evalEmit(m, frag[j+2:j+3], method, prefix, refs...)
				knownPredicate = true
			}
			if frag[j].kind == "regex" {
				s.evalEmit(m, frag[j:j+1], method, prefix, refs...)
				knownPredicate = true
			}
			if frag[j].text == "new" && j+2 < len(frag) && frag[j+1].text == "RegExp" && frag[j+2].text == "(" {
				k := bffClose(frag, j+2)
				if k >= 0 {
					s.evalEmit(m, frag[j:k+1], method, prefix, refs...)
					knownPredicate = true
				}
			}
			if j+2 < len(frag) && frag[j+1].text == "." && frag[j+2].text == "test" && frag[j].kind == "ident" {
				knownPredicate = true
				_, local := m.defs[frag[j].text]
				_, imported := m.imports[frag[j].text]
				if !local && !imported {
					addUnresolved(&s.out, m.ref(frag[j:j+3]), "unknown admission regex binding "+frag[j].text)
				}
				if d, ok := m.defs[frag[j].text]; ok {
					s.evalEmit(m, d.tokens, method, prefix, append(refs, d.ref)...)
				}
				if imp, ok := m.imports[frag[j].text]; ok {
					other, e := s.reader.load(imp[0])
					if e == nil {
						if d, ok := other.defs[imp[1]]; ok {
							s.evalEmit(other, d.tokens, method, prefix, append(refs, d.ref)...)
						}
					}
				}
			}
		}
		if !knownPredicate && name != "importRoute" && !strings.Contains(bffText(t), "parts [") {
			addUnresolved(&s.out, m.ref(t[i:end]), "unsupported method/path admission predicate")
		}
	}
	// Calls to another admission helper, not validation or recognition-only siblings.
	for i := 0; i+1 < len(t); i++ {
		if t[i+1].text == "(" && strings.HasSuffix(t[i].text, "Route") && t[i].text != name {
			s.admit(m, t[i].text, prefix, refs...)
		}
	}
	if strings.Contains(bffText(t), "parts [") && strings.Contains(bffText(t), "split (") {
		s.operations(m, f, prefix, refs...)
	}
	if name == "reportsRoute" {
		s.reports(m, prefix, refs...)
	}
	if name == "importRoute" {
		s.imports(m, f, prefix, refs...)
	}
	if len(s.out.Routes) == count {
		addUnresolved(&s.out, f.ref, "unsupported admission helper "+name)
	}
}
func (s *bffScanner) table(m *bffModule, v bffValue, prefix string, refs ...location) {
	refs = append(refs, v.refs...)
	for method, x := range v.object {
		if !bffHTTP(method) {
			continue
		}
		if x.object != nil {
			for path := range x.object {
				s.emit(method, prefix, bffValue{texts: []string{path}, refs: x.object[path].refs}, refs...)
			}
		} else {
			s.emit(method, prefix, x, refs...)
		}
	}
	for _, row := range v.array {
		if len(row.array) >= 2 && len(row.array[0].texts) == 1 && bffHTTP(row.array[0].texts[0]) {
			s.emit(row.array[0].texts[0], prefix, row.array[1], refs...)
		}
	}
}
func (s *bffScanner) operations(m *bffModule, f bffExpr, prefix string, refs ...location) {
	t := f.tokens
	root := ""
	methods := map[int]string{}
	var actions []string
	for i := 0; i+5 < len(t); i++ {
		if t[i].text == "parts" && t[i+1].text == "[" && t[i+2].text == "0" && t[i+4].text == "!==" && t[i+5].kind == "string" {
			root = bffLiteral(t[i+5])
		}
		if t[i].text == "method" && t[i+1].text == "===" && bffHTTP(bffLiteral(t[i+2])) {
			method := bffLiteral(t[i+2])
			for j := i + 3; j+4 < len(t) && t[j].text != "return"; j++ {
				if t[j].text == "parts" && t[j+1].text == "." && t[j+2].text == "length" && t[j+3].text == "===" {
					methods[bffInt(t[j+4])] = method
				}
			}
		}
		if t[i].text == "[" {
			end := bffClose(t, i)
			if end > i && end+2 < len(t) && t[end+1].text == "." && t[end+2].text == "includes" {
				v, e := s.reader.eval(m, t[i:end+1], nil, 0)
				if e == nil {
					for _, x := range v.array {
						actions = append(actions, x.texts...)
					}
				}
			}
		}
	}
	if root == "" || len(methods) != 3 || len(actions) == 0 {
		addUnresolved(&s.out, f.ref, "operations split/includes shape changed")
		return
	}
	s.emit(methods[1], prefix, bffValue{texts: []string{root}}, refs...)
	s.emit(methods[2], prefix, bffValue{texts: []string{root + "/{}"}}, refs...)
	for _, a := range actions {
		s.emit(methods[3], prefix, bffValue{texts: []string{root + "/{}/" + a}}, refs...)
	}
}
func (s *bffScanner) reports(m *bffModule, prefix string, refs ...location) {
	d, ok := m.defs["routes"]
	if !ok {
		return
	}
	t := d.tokens
	if len(t) < 6 || t[1].text != "." || t[2].text != "flatMap" {
		addUnresolved(&s.out, d.ref, "unsupported reports flatMap")
		return
	}
	names, e := s.reader.symbol(m, t[0].text, nil, 0)
	if e != nil {
		addUnresolved(&s.out, d.ref, e.Error())
		return
	}
	for _, name := range names.array {
		env := map[string]bffValue{"name": name}
		for i := 0; i+3 < len(t); i++ {
			if t[i].text == "new" && t[i+1].text == "RegExp" {
				end := bffClose(t, i+2)
				if end < 0 {
					continue
				}
				v, e := s.reader.eval(m, t[i:end+1], env, 0)
				if e != nil {
					addUnresolved(&s.out, d.ref, e.Error())
				} else {
					s.emit("GET", prefix, v, append(append(refs, d.ref), names.refs...)...)
				}
				i = end
			}
		}
	}
}
func (s *bffScanner) imports(m *bffModule, f bffExpr, prefix string, refs ...location) {
	t := f.tokens
	for i := 0; i+2 < len(t); i++ {
		if t[i].text != "method" || t[i+1].text != "===" {
			continue
		}
		method := bffLiteral(t[i+2])
		if !bffHTTP(method) {
			continue
		}
		j := i + 3
		for j < len(t) && t[j].text != "return" {
			j++
		}
		var params, actions []string
		uuid := false
		for k := i + 3; k+2 < j; k++ {
			if t[k].text == "param" && t[k+1].text == "===" && t[k+2].kind == "string" {
				params = append(params, bffLiteral(t[k+2]))
			}
			if t[k].text == "action" && t[k+1].text == "===" && t[k+2].kind == "string" {
				actions = append(actions, bffLiteral(t[k+2]))
			}
			if t[k].text == "test" {
				uuid = true
			}
		}
		if len(params) == 0 && uuid {
			params = []string{"{}"}
		}
		if len(params) == 0 || len(actions) == 0 {
			addUnresolved(&s.out, f.ref, "unsupported import admission")
			continue
		}
		for _, p := range params {
			for _, a := range actions {
				s.emit(method, prefix, bffValue{texts: []string{"imports/" + p + "/" + a}}, refs...)
			}
		}
	}
}
