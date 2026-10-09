// Purpose: resolve buyer adapters and track actual forwarding methods.
// Depends on: finite regex expansion and TypeScript source bindings; Go stdlib only.
// Used by: scanBFF and the CI-DRIFT gate; never executes application code.
// Invariant: local mint/guards do not produce upstream routes; unknown sinks are unresolved.
package main

import (
	"fmt"
	"strings"
)

func (s *bffScanner) buyer(leaf *bffModule, refs ...location) {
	m, _, e := s.bindingFunc(leaf, "handleBuyerRequest")
	if e != nil {
		addUnresolved(&s.out, refs[0], e.Error())
		return
	}
	f, ok := m.funcs["route"]
	if !ok {
		addUnresolved(&s.out, m.ref(m.tokens), "missing buyer admission")
		return
	}
	refs = append(refs, f.ref)
	if transport, ok := m.funcs["upstream"]; ok {
		for _, call := range bffCalls(m, transport.tokens, "fetch") {
			refs = append(refs, m.ref(call))
			args := bffSplit(call, ",")
			if len(args) == 0 || len(args[0]) != 1 || !strings.Contains(args[0][0].text, "/v1/buyer/${path}") {
				addUnresolved(&s.out, m.ref(call), "buyer upstream prefix shape changed")
			}
		}
	}
	for _, call := range bffCalls(m, m.tokens, "upstream") {
		args := bffSplit(call, ",")
		if len(args) > 5 && bffText(args[4]) == "target . privatePath + query " && bffText(args[5]) == "request . method " {
			refs = append(refs, m.ref(call))
		}
	}
	d, ok := m.defs["exact"]
	if !ok {
		addUnresolved(&s.out, f.ref, "missing buyer exact table")
		return
	}
	v, e := s.reader.eval(m, d.tokens, nil, 0)
	if e != nil {
		addUnresolved(&s.out, d.ref, e.Error())
	}
	for _, methods := range v.object {
		for method, x := range methods.object {
			if !bffHTTP(method) {
				continue
			}
			p, ok := x.object["privatePath"]
			if !ok {
				addUnresolved(&s.out, d.ref, "buyer table missing privatePath")
				continue
			}
			s.emit(method, "/v1/buyer/", p, append(append(append(append(refs, d.ref), v.refs...), methods.refs...), x.refs...)...)
		}
	}
	// Session and direct lookup/link mappings are read from upstream argument positions.
	for _, call := range bffCalls(m, m.tokens, "upstream") {
		args := bffSplit(call, ",")
		if len(args) > 5 && len(args[4]) == 1 && len(args[5]) == 1 && args[4][0].kind == "string" && bffHTTP(bffLiteral(args[5][0])) {
			s.evalEmit(m, args[4], bffLiteral(args[5][0]), "/v1/buyer/", append(refs, m.ref(call))...)
		} else if len(args) > 5 && bffText(args[4]) == "target . privatePath + query " && bffText(args[5]) == "request . method " {
			refs = append(refs, m.ref(call))
		} else {
			addUnresolved(&s.out, m.ref(call), "unsupported buyer upstream path/method")
		}
	}
	// Closed regex branches retain same suffix, choosing the base/read vs action method.
	t := f.tokens
	for i, x := range t {
		if x.kind != "regex" {
			continue
		}
		if !strings.Contains(x.text, "[^/]") {
			continue
		}
		paths, e := bffExpand(bffUnquote(x))
		if e != nil {
			addUnresolved(&s.out, m.ref(t[i:i+1]), e.Error())
			continue
		}
		end := i + 1
		for end < len(t) && t[end].text != "return" {
			end++
		} // Branch method mapping is derived from allowed/ternary tokens in its balanced block.
		branchEnd := len(t)
		for j := end; j < len(t); j++ {
			if t[j].text == "const" && j > end && t[j+1].text != "allowed" && t[j+1].text != "kind" && t[j+1].text != "proof" && t[j+1].text != "verify" {
				branchEnd = j
				break
			}
		}
		branch := t[i:branchEnd]
		var methods []string
		for _, tok := range branch {
			v := bffLiteral(tok)
			if tok.kind == "string" && bffHTTP(v) {
				methods = append(methods, v)
			}
		}
		if len(methods) == 0 || len(methods) > 2 || !strings.Contains(bffText(branch), "privatePath : suffix") {
			addUnresolved(&s.out, m.ref(t[i:i+1]), "buyer regex without closed methods")
			continue
		}
		baseTrue := false
		for j := 0; j+4 < len(branch); j++ { // default captured action and the method condition agree => base takes true branch.
			if branch[j].text != "??" || branch[j+1].kind != "string" {
				continue
			}
			def := bffLiteral(branch[j+1])
			for k := j + 2; k+3 < len(branch); k++ {
				if branch[k].text == "===" && branch[k+1].kind == "string" && bffLiteral(branch[k+1]) == def && branch[k+2].text == "?" && branch[k+3].kind == "string" && bffHTTP(bffLiteral(branch[k+3])) {
					baseTrue = true
				}
			}
		}
		shortest := int(^uint(0) >> 1)
		for _, p := range paths {
			shortest = min(shortest, len(p))
		}
		for _, p := range paths {
			method := methods[len(methods)-1]
			if len(methods) > 1 {
				if (len(p) == shortest) == baseTrue {
					method = methods[0]
				}
			}
			s.emit(method, "/v1/buyer/", bffValue{texts: []string{p}}, append(refs, m.ref(branch))...)
		}
	}
}

// Leaf aliases and sink calls are route provenance, rather than unrelated file lines.
func bffLeafRefs(m *bffModule) []location {
	var refs []location
	for i, t := range m.tokens {
		if t.text == "export" {
			end := i + 1
			for end < len(m.tokens) && end < i+7 && m.tokens[end].text != "=" && m.tokens[end].text != "(" {
				end++
			}
			refs = append(refs, m.ref(m.tokens[i:min(end+1, len(m.tokens))]))
		}
	}
	for _, sink := range []string{"callBackend", "fetch", "privateIdentity", "importProxy", "customerTagsBFF", "picklistProxy", "handleBuyerRequest", "listProducts", "getProduct", "authenticatedStores"} {
		for _, call := range bffCalls(m, m.tokens, sink) {
			refs = append(refs, m.ref(call))
		}
	}
	if len(refs) == 0 {
		refs = []location{m.ref(m.tokens)}
	}
	return refs
}

// Extra forwarding cannot hide behind the existing admission table.
func (s *bffScanner) extraSinks(m *bffModule, refs ...location) {
	for _, sink := range []string{"callBackend", "fetch", "merchantBackend", "privateIdentity"} {
		for _, call := range bffCalls(m, m.tokens, sink) {
			args := bffSplit(call, ",")
			if len(args) == 0 {
				continue
			}
			arg := bffText(args[0])
			if sink == "callBackend" && arg == "path + url . search " {
				continue
			}
			if sink == "fetch" && len(args[0]) == 1 && args[0][0].kind == "template" && strings.HasSuffix(args[0][0].text, "/${path}`") {
				continue
			}
			method := ""
			if len(args) > 1 {
				for i := 0; i+2 < len(args[1]); i++ {
					if args[1][i].text == "method" && args[1][i+1].text == ":" && args[1][i+2].kind == "string" {
						method = bffLiteral(args[1][i+2])
					}
				}
			}
			if method == "" {
				addUnresolved(&s.out, m.ref(call), "extra forwarding sink without finite method")
				continue
			}
			v, e := s.dynamicPath(m, args[0])
			if e != nil {
				addUnresolved(&s.out, m.ref(call), e.Error())
				continue
			}
			prefix := bffStorePrefix
			if sink == "privateIdentity" {
				prefix = "/v1/identity/"
				method = "POST"
			}
			if sink == "fetch" {
				prefix = ""
				valid := true
				for i, p := range v.texts {
					at := strings.Index(p, "/v1/")
					if at < 0 {
						valid = false
						break
					}
					v.texts[i] = p[at:]
				}
				if !valid {
					addUnresolved(&s.out, m.ref(call), "extra fetch URL has no fixed API path")
					continue
				}
			}
			s.emit(method, prefix, v, append(refs, m.ref(call))...)
		}
	}
}

// Resolve the method and provenance of the identity transport itself. The browser
// callback's GET must not replace the actual POST, and a transport edit is touched.
func (s *bffScanner) identitySink(m *bffModule) (string, []location, error) {
	owner, f, e := s.bindingFunc(m, "privateIdentity")
	if e != nil {
		return "", nil, e
	}
	for _, call := range bffCalls(owner, f.tokens, "fetch") {
		args := bffSplit(call, ",")
		if len(args) < 2 {
			continue
		}
		if len(args[0]) != 1 || !strings.Contains(args[0][0].text, "/v1/identity/${path}") {
			return "", nil, fmt.Errorf("identity upstream prefix shape changed")
		}
		for i := 0; i+2 < len(args[1]); i++ {
			if args[1][i].text == "method" && args[1][i+1].text == ":" && args[1][i+2].kind == "string" && bffHTTP(bffLiteral(args[1][i+2])) {
				return bffLiteral(args[1][i+2]), []location{m.importRefs["privateIdentity"], owner.ref(call)}, nil
			}
		}
	}
	return "", nil, fmt.Errorf("identity upstream method unresolved")
}
func (s *bffScanner) backendRefs(m *bffModule) []location {
	owner, f, e := s.bindingFunc(m, "callBackend")
	if e != nil {
		return nil
	}
	refs := []location{m.importRefs["callBackend"]}
	for _, call := range bffCalls(owner, f.tokens, "merchantBackend") {
		refs = append(refs, owner.ref(call))
		if len(call) > 0 {
			args := bffSplit(call, ",")
			if len(args[0]) != 1 || !strings.Contains(args[0][0].text, "/v1/admin/stores/${storeID}/${resource}") {
				addUnresolved(&s.out, owner.ref(call), "backend store prefix shape changed")
			}
		}
	}
	return refs
}

// A handler's exported alias counts only when its body reaches a forwarding sink.
// Recognition tables alone and local 405 aliases cannot create upstream methods.
func (s *bffScanner) forwardingExport(m *bffModule, name string, seen map[string]bool, depth int) bool {
	if depth > 16 {
		return false
	}
	key := m.name + ":" + name
	if seen[key] {
		return false
	}
	seen[key] = true
	if i, ok := m.imports[name]; ok {
		owner, e := s.reader.load(i[0])
		if e != nil {
			return false
		}
		return s.forwardingExport(owner, i[1], seen, depth+1)
	}
	var t []bffToken
	if f, ok := m.funcs[name]; ok {
		t = f.tokens
	} else if d, ok := m.defs[name]; ok {
		t = d.tokens
	} else {
		return false
	}
	for i, x := range t {
		if x.text == "fetch" || x.text == "callBackend" || x.text == "privateIdentity" || x.text == "merchantBackend" || x.text == "transport" {
			if i+1 < len(t) && t[i+1].text == "(" {
				return true
			}
		}
		if i+2 < len(t) && x.text == "adapter" && t[i+1].text == "." && t[i+2].text == "forward" {
			return true
		}
		if x.kind == "ident" && ((i+1 < len(t) && t[i+1].text == "(") || len(t) == 1) {
			if s.forwardingExport(m, x.text, seen, depth+1) {
				return true
			}
		}
	}
	return false
}
func (s *bffScanner) filterMethods(m *bffModule, start int) {
	allowed := map[string]bool{}
	for method := range bffExports(m) {
		allowed[method] = s.forwardingExport(m, method, map[string]bool{}, 0)
	}
	kept := s.out.Routes[:start]
	for _, r := range s.out.Routes[start:] {
		if allowed[r.Method] {
			kept = append(kept, r)
		}
	}
	s.out.Routes = kept
}

// Method tables describe browser admission. A fixed transport method overrides the
// admitted method; the usual request.method seam passes it through unchanged.
func (s *bffScanner) forwardingMethod(m *bffModule, start int) {
	for _, sink := range []string{"fetch", "callBackend", "transport"} {
		for _, call := range bffCalls(m, m.tokens, sink) {
			args := bffSplit(call, ",")
			if len(args) < 2 {
				continue
			}
			path := bffText(args[0])
			generic := sink == "callBackend" && path == "path + url . search " || sink == "fetch" && (path == "origin " || strings.Contains(path, "${path}") || strings.Contains(path, "${report}")) || sink == "transport" && strings.Contains(path, "/imports/${param}/${action}")
			if !generic {
				continue
			}
			options := args[1]
			refs := []location{m.ref(call)}
			if len(options) == 1 && options[0].kind == "ident" {
				if d, ok := m.defs[options[0].text]; ok {
					options = d.tokens
					refs = append(refs, d.ref)
				}
			}
			method := ""
			resolved := false
			for i := 0; i+2 < len(options); i++ {
				if options[i].text == "..." && options[i+1].text == "init" {
					if d, ok := m.defs["init"]; ok {
						options = append(append([]bffToken{}, options...), d.tokens...)
						refs = append(refs, d.ref)
					}
				}
				if options[i].text != "method" || options[i+1].text != ":" {
					continue
				}
				if options[i+2].kind == "string" && bffHTTP(bffLiteral(options[i+2])) {
					method = bffLiteral(options[i+2])
					resolved = true
				} else if i+4 < len(options) && options[i+2].text == "request" && options[i+3].text == "." && options[i+4].text == "method" {
					resolved = true
				} else {
					addUnresolved(&s.out, m.ref(options[i:i+3]), "unsupported forwarding method expression")
				}
				break
			}
			if !resolved {
				addUnresolved(&s.out, m.ref(call), "forwarding method not resolved")
				continue
			}
			for i := start; i < len(s.out.Routes); i++ {
				if method != "" {
					s.out.Routes[i].Method = method
				}
				s.out.Routes[i].Locations = append(s.out.Routes[i].Locations, refs...)
			}
		}
	}
}
func bffStaticMethod(options []bffToken) (string, error) {
	for i := 0; i+2 < len(options); i++ {
		if options[i].text == "method" && options[i+1].text == ":" && options[i+2].kind == "string" && bffHTTP(bffLiteral(options[i+2])) {
			return bffLiteral(options[i+2]), nil
		}
	}
	return "", fmt.Errorf("static upstream method unresolved")
}
