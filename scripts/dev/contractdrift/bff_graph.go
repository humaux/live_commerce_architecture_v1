// Purpose: build and inspect the finite graph of BFF runtime bindings and their provenance.
// Depends on: finite TypeScript reader, sink accounting and Go stdlib only.
// Used by: producer finalization; unknown reached bindings fail closed without recursive finding trees.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// Platform responses/header access and pure crypto/IP APIs are non-forwarding
// leaves, not wildcard exemptions for external packages. Workspace exports are
// resolved from their actual source by the reader; other reached imports fail closed.
func bffNonForwardingImport(path, name string) bool {
	switch path {
	case "next/server":
		return name == "NextResponse" || name == "NextRequest"
	case "next/headers":
		return name == "headers" || name == "cookies"
	case "node:crypto":
		return name == "createHmac" || name == "randomBytes" || name == "timingSafeEqual"
	case "node:net":
		return name == "isIP"
	case "react":
		return name == "cache" // Its callback body is still traversed.
	}
	return false
}
func bffCanonicalSinkImport(path, name string) bool {
	return name == "callBackend" && path == "apps/admin/lib/backend.ts" || (name == "merchantBackend" || name == "privateIdentity") && path == "apps/admin/lib/auth.ts"
}

// Build each reachable binding/body once, then propagate hazards and provenance
// over reverse edges. Cycles and shared bodies cannot replicate finding trees.
func (s *bffScanner) unaccountedSinkValues(root string) {
	type hazard struct {
		detail string
		ref    location
	}
	type node struct {
		m       *bffModule
		tokens  []bffToken
		edges   map[string][]location
		hazards map[hazard]bool
		pending bool
	}
	nodes := map[string]*node{}
	queue := []string{}
	var body func(*bffModule, []bffToken) string
	body = func(m *bffModule, t []bffToken) string {
		key := "body:" + bffCallKey(m, t)
		if nodes[key] == nil {
			nodes[key] = &node{m: m, tokens: t, edges: map[string][]location{}, hazards: map[hazard]bool{}}
			queue = append(queue, key)
		}
		return key
	}
	var resolve func(*bffModule, string, int) string
	resolve = func(m *bffModule, name string, depth int) string {
		// Value namespaces win over a same-named, erased type declaration.
		if f, ok := m.funcs[name]; ok {
			if m.name == "apps/admin/lib/auth.ts" && name == "merchantBackend" {
				s.proveMerchantTransport(m, f)
			}
			return body(m, f.tokens)
		}
		if d, ok := m.defs[name]; ok {
			return body(m, d.tokens)
		}
		imp, imported := m.imports[name]
		if m.typeImports[name] || m.types[name] && !imported {
			return ""
		}
		key := "binding:" + m.name + ":" + name
		if n := nodes[key]; n != nil {
			if n.pending {
				n.hazards[hazard{"cyclic reached import binding", m.importRefs[name]}] = true
			}
			return key
		}
		n := &node{m: m, edges: map[string][]location{}, hazards: map[hazard]bool{}, pending: true}
		nodes[key] = n
		defer func() { n.pending = false }()
		refs := m.bindingRefs[name]
		if len(refs) == 0 {
			refs = []location{m.importRefs[name]}
		}
		origin := m.ref(nil)
		if imported {
			origin = m.importRefs[name]
		}
		if depth > 128 {
			n.hazards[hazard{"forwarding import depth unresolved", origin}] = true
			return key
		}
		if !imported {
			n.hazards[hazard{"unknown reached source binding " + name, origin}] = true
			return key
		}
		if bffNonForwardingImport(imp[0], imp[1]) {
			return ""
		}
		owner, err := s.reader.load(imp[0])
		if err != nil {
			n.hazards[hazard{"unknown reached import binding " + name, origin}] = true
			return key
		}
		if child := resolve(owner, imp[1], depth+1); child != "" {
			n.edges[child] = bffUniqueRefs(refs)
		}
		return key
	}
	if m := s.reader.modules[root]; m != nil {
		for name := range bffExports(m) {
			resolve(m, name, 0)
		}
		if strings.HasSuffix(root, "/proxy.ts") {
			resolve(m, "proxy", 0)
		}
	}
	for _, ctx := range s.contexts {
		if ctx.m.name == root && bffCallKey(ctx.m, ctx.tokens) == bffCallKey(ctx.m, ctx.m.tokens) {
			continue
		}
		body(ctx.m, ctx.tokens)
	}
	for head := 0; head < len(queue); head++ {
		key := queue[head]
		n := nodes[key]
		m := n.m
		t := n.tokens
		for i, tok := range t {
			if tok.kind != "ident" || i+1 < len(t) && t[i+1].text == ":" {
				continue
			}
			name := tok.text
			imp, imported := m.imports[name]
			remote := name
			if imported {
				remote = imp[1]
			}
			_, functionValue := m.funcs[name]
			_, definedValue := m.defs[name]
			if !functionValue && !definedValue && !imported && (i == 0 || t[i-1].text != "." && t[i-1].text != "?.") && (name == "globalThis" || name == "global" || name == "window" || name == "self") {
				n.hazards[hazard{"unsupported global namespace capability", m.ref(t[i : i+1])}] = true
				continue
			}
			if bffSinkName(remote) && !functionValue && !definedValue && (!imported || bffCanonicalSinkImport(imp[0], remote)) {
				if s.consumedValues[fmt.Sprintf("%s:%d", m.name, tok.start)] {
					continue
				}
				literal := name == remote && i+1 < len(t) && t[i+1].text == "(" && (i == 0 || t[i-1].text != "." && t[i-1].text != "?.")
				if !literal {
					n.hazards[hazard{"unaccounted forwarding sink value (alias, member or higher-order use)", m.ref(t[i : i+1])}] = true
				}
				continue
			}
			if i > 0 && (t[i-1].text == "." || t[i-1].text == "?." || t[i-1].text == "const" || t[i-1].text == "let" || t[i-1].text == "function") {
				continue
			}
			if !functionValue && !definedValue && (!imported || m.typeImports[name]) {
				continue
			}
			if child := resolve(m, name, 0); child != "" {
				n.edges[child] = bffUniqueRefs(append(n.edges[child], m.ref(t[i:i+1])))
			}
		}
	}
	// Cache unique ancestor edge locations, not recursively nested diagnostics.
	parents := map[string]map[string][]location{}
	for key, n := range nodes {
		for child, refs := range n.edges {
			if parents[child] == nil {
				parents[child] = map[string][]location{}
			}
			parents[child][key] = refs
		}
	}
	ancestors := func(start string) []location {
		seen := map[string]bool{start: true}
		q := []string{start}
		refs := []location{}
		for head := 0; head < len(q); head++ {
			for parent, edgeRefs := range parents[q[head]] {
				refs = append(refs, edgeRefs...)
				if !seen[parent] {
					seen[parent] = true
					q = append(q, parent)
				}
			}
		}
		return bffUniqueRefs(refs)
	}
	for key, n := range nodes {
		refs := ancestors(key)
		if len(n.tokens) > 0 {
			s.sinkContext(n.m, n.tokens, refs...)
		}
		for h := range n.hazards {
			addUnresolved(&s.out, h.ref, h.detail)
			for _, ref := range refs {
				addUnresolved(&s.out, ref, h.detail)
			}
		}
	}
}

func bffUniqueRefs(refs []location) []location {
	seen := map[location]bool{}
	out := []location{}
	for _, ref := range refs {
		if ref.File != "" && ref.Line > 0 && !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].End < out[j].End
	})
	return out
}

// The canonical merchant adapter passes its third RequestInit parameter through
// one literal fetch. Prove that body instead of granting a module/name exemption.
func (s *bffScanner) proveMerchantTransport(m *bffModule, f bffExpr) {
	if refs := bffConfigUses(m); len(refs) > 0 {
		for _, ref := range refs {
			addUnresolved(&s.out, ref, "unsupported authConfig runtime reference or escape")
		}
		return
	}
	if !bffMerchantOrigin(m) {
		for _, ref := range []location{m.defs["authConfig"].ref, m.funcs["readAuthConfig"].ref, m.funcs["exactOrigin"].ref} {
			if ref.Line > 0 {
				addUnresolved(&s.out, ref, "merchant API origin binding is not proven URL.origin")
			}
		}
		return
	}
	path, init := "", ""
	for i, t := range m.tokens {
		if t.text == "function" && i+2 < len(m.tokens) && m.tokens[i+1].text == "merchantBackend" && m.tokens[i+2].text == "(" {
			end := bffClose(m.tokens, i+2)
			if end > i {
				p := bffSplit(m.tokens[i+3:end], ",")
				if len(p) > 2 && len(p[0]) > 0 && len(p[2]) > 0 {
					path, init = p[0][0].text, p[2][0].text
				}
			}
			break
		}
	}
	if path == "" || init == "" {
		return
	}
	for i, t := range f.tokens {
		if t.text == path {
			addUnresolved(&s.out, m.ref(f.tokens[i:i+1]), "merchant path parameter is not a proven passthrough")
			return
		}
		if t.text != init {
			continue
		}
		if i > 0 && f.tokens[i-1].text == "..." {
			continue
		}
		if i+2 < len(f.tokens) && f.tokens[i+1].text == "." && f.tokens[i+2].text == "headers" {
			continue
		}
		addUnresolved(&s.out, m.ref(f.tokens[i:i+1]), "merchant init parameter is not a proven passthrough")
		return
	}
	var proven []bffToken
	for _, call := range bffCalls(m, f.tokens, "fetch") {
		a := bffSplit(call, ",")
		if len(a) < 2 || len(a[0]) != 1 || a[0][0].kind != "template" || a[0][0].text != "`${authConfig.apiOrigin}${"+path+"}`" {
			continue
		}
		options := a[1]
		if len(options) < 2 || options[0].text != "{" || bffClose(options, 0) != len(options)-1 {
			return
		}
		spreads := 0
		for _, field := range bffSplit(options[1:len(options)-1], ",") {
			if len(field) == 0 {
				continue
			}
			if field[0].text == "..." {
				if len(field) != 2 || field[1].text != init {
					return
				}
				spreads++
				continue
			}
			if field[0].kind != "ident" && field[0].kind != "string" || bffLiteral(field[0]) == "method" {
				return
			}
		}
		if spreads != 1 || proven != nil {
			return
		}
		proven = call
	}
	if proven != nil {
		s.consumeSink(m, proven)
	}
}

// Only the existing config construction is admitted: its API value is produced
// by the unshadowed helper that creates a native URL and returns URL.origin.
func bffMerchantOrigin(m *bffModule) bool {
	if strings.TrimSpace(bffText(m.defs["authConfig"].tokens)) != "readAuthConfig ( process . env )" {
		return false
	}
	if _, ok := m.defs["exactOrigin"]; ok {
		return false
	}
	if _, ok := m.defs["URL"]; ok {
		return false
	}
	if _, ok := m.imports["URL"]; ok {
		return false
	}
	origin, ok := m.funcs["exactOrigin"]
	if !ok {
		return false
	}
	t := origin.tokens
	constructor, returns := 0, 0
	for i, tok := range t {
		if i+7 < len(t) && tok.text == "const" && bffText(t[i:i+8]) == "const url = new URL ( value ) " {
			constructor++
		}
		if tok.text == "return" {
			returns++
			if i+4 >= len(t) || bffText(t[i+1:i+5]) != "url . origin ; " {
				return false
			}
		}
		if tok.text == "url" && i+1 < len(t) && t[i+1].text == "=" && (i == 0 || t[i-1].text != "const") {
			return false
		}
	}
	if constructor != 1 || returns != 1 {
		return false
	}
	// The actual native URL value may only be initialized and read through
	// primitive fields; passing/aliasing/computed access cannot prove origin.
	primitive := map[string]bool{"protocol": true, "hostname": true, "pathname": true, "username": true, "password": true, "search": true, "hash": true, "origin": true, "href": true, "host": true, "port": true}
	for i, tok := range t {
		if tok.text != "url" || i > 0 && (t[i-1].text == "." || t[i-1].text == "?.") || i+1 < len(t) && t[i+1].text == ":" {
			continue
		}
		if i > 0 && i+6 < len(t) && bffText(t[i-1:i+7]) == "const url = new URL ( value ) " {
			continue
		}
		if i+3 < len(t) && t[i+1].text == "." && primitive[t[i+2].text] {
			read := false
			switch t[i+3].text {
			case ";", ",", ")", "]", "}", "?", ":", "&&", "||", "??", "===", "!==", "==", "!=", ".":
				read = true
			}
			if i > 0 && t[i-1].text == "delete" {
				read = false
			}
			if read {
				continue
			}
		}
		return false
	}
	config, ok := m.funcs["readAuthConfig"]
	if !ok {
		return false
	}
	count := 0
	for i := 0; i < len(config.tokens); i++ {
		tok := config.tokens[i]
		if tok.text == "=>" && i+1 < len(config.tokens) && config.tokens[i+1].text == "{" {
			end := bffClose(config.tokens, i+1)
			if end < 0 {
				return false
			}
			i = end
			continue
		}
		if tok.text == "..." {
			return false
		}
		if tok.text == "return" && (i+1 >= len(config.tokens) || config.tokens[i+1].text != "null" && config.tokens[i+1].text != "{") {
			return false
		}
		if tok.text != "apiOrigin" {
			continue
		}
		if i+2 >= len(config.tokens) || config.tokens[i+1].text != ":" || config.tokens[i+2].text != "exactOrigin" {
			return false
		}
		count++
	}
	return count == 1
}

// Config is a privileged runtime object, not a value that arbitrary helpers may
// retain or mutate. Admit its proven construction, boolean guards and primitive
// field reads; computed access, member writes and object escapes stay opaque.
func bffConfigUses(m *bffModule) []location {
	fields := map[string]bool{}
	for i, t := range m.tokens {
		if t.text != "AuthConfig" || i < 1 || m.tokens[i-1].text != "type" {
			continue
		}
		j := i + 1
		for j < len(m.tokens) && m.tokens[j].text != "{" {
			j++
		}
		end := bffClose(m.tokens, j)
		if end < 0 {
			break
		}
		for _, f := range bffSplit(m.tokens[j+1:end], ";") {
			if len(f) < 3 || f[0].kind != "ident" || f[1].text != ":" {
				continue
			}
			primitive := true
			for _, v := range f[2:] {
				if v.text != "string" && v.text != "boolean" && v.text != "null" && v.text != "|" {
					primitive = false
				}
			}
			if primitive {
				fields[f[0].text] = true
			}
		}
		break
	}
	refs := []location{}
	for i, tok := range m.tokens {
		if tok.kind == "template" {
			text := tok.text
			for {
				at := strings.Index(text, "${")
				if at < 0 {
					break
				}
				text = text[at+2:]
				end := strings.Index(text, "}")
				if end < 0 {
					break
				}
				expr := bffLex(text[:end])
				has := false
				for _, x := range expr {
					if x.kind == "ident" && x.text == "authConfig" {
						has = true
					}
				}
				if has && !(len(expr) == 3 && expr[0].text == "authConfig" && expr[1].text == "." && fields[expr[2].text]) {
					refs = append(refs, m.ref(m.tokens[i:i+1]))
				}
				text = text[end+1:]
			}
			continue
		}
		if tok.kind != "ident" || tok.text != "authConfig" {
			continue
		}
		if i+1 < len(m.tokens) && m.tokens[i+1].text == ":" {
			continue
		}
		if i > 0 && m.tokens[i-1].text == "const" && i+1 < len(m.tokens) && m.tokens[i+1].text == "=" {
			continue
		} // construction checked separately
		if i+2 < len(m.tokens) && (m.tokens[i+1].text == "." || m.tokens[i+1].text == "?.") && fields[m.tokens[i+2].text] {
			read := i+3 == len(m.tokens)
			if i+3 < len(m.tokens) {
				switch m.tokens[i+3].text {
				case ";", ",", ")", "]", "}", "?", ":", "&&", "||", "??", "===", "!==", "==", "!=", ".", "?.":
					read = true
				}
			}
			if i > 0 && m.tokens[i-1].text == "delete" {
				read = false
			}
			if read {
				continue
			}
		} else if i+1 < len(m.tokens) && m.tokens[i+1].text != "[" && m.tokens[i+1].text != "." && m.tokens[i+1].text != "?." {
			if i > 0 && m.tokens[i-1].text == "!" {
				continue
			}
			if m.tokens[i+1].text == "&&" || m.tokens[i+1].text == "?" {
				continue
			}
			if i > 1 && m.tokens[i-2].text == "if" && m.tokens[i-1].text == "(" && m.tokens[i+1].text == ")" {
				continue
			}
		}
		refs = append(refs, m.ref(m.tokens[i:i+1]))
	}
	return bffUniqueRefs(refs)
}
