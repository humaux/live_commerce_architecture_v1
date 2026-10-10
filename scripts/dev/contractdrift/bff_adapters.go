// Purpose: resolve closed admin and shop leaves to their upstream routes.
// Depends on: finite TypeScript values, admission helpers and shared inventory types.
// Used by: scanBFF and the CI-DRIFT gate; never executes application code.
// Invariant: source methods and paths retain leaf, helper and sink provenance.
package main

import (
	"fmt"
	"strings"
)

func (s *bffScanner) leaf(m *bffModule) {
	before := len(s.out.Routes)
	refs := bffLeafRefs(m)
	if len(bffCalls(m, m.tokens, "callBackend")) > 0 {
		refs = append(refs, s.backendRefs(m)...)
	}
	exports := bffExports(m)
	name := m.name
	if strings.Contains(name, "/stores/[store]/[...resource]/") {
		defer func() { s.filterMethods(m, before); s.forwardingMethod(m, before) }()
		if len(bffCalls(m, m.tokens, "callBackend"))+len(bffCalls(m, m.tokens, "fetch")) == 0 {
			addUnresolved(&s.out, refs[0], "catchall lacks a reachable forwarding sink")
			return
		}
		if d, ok := m.defs["routes"]; ok {
			v, e := s.reader.eval(m, d.tokens, nil, 0)
			if e != nil {
				addUnresolved(&s.out, d.ref, e.Error())
			} else {
				s.table(m, v, bffStorePrefix, append(refs, d.ref)...)
			}
		} else if _, ok := m.imports["routes"]; ok {
			v, e := s.reader.symbol(m, "routes", nil, 0)
			if e != nil {
				addUnresolved(&s.out, refs[0], e.Error())
			} else {
				s.table(m, v, bffStorePrefix, refs...)
			}
		} else {
			addUnresolved(&s.out, refs[0], "missing method admission table")
		}
		if f, ok := m.funcs["route"]; ok {
			for i := 0; i+1 < len(f.tokens); i++ {
				if strings.HasSuffix(f.tokens[i].text, "Route") && f.tokens[i+1].text == "(" && f.tokens[i].text != "claimLinkRoute" {
					s.admit(m, f.tokens[i].text, bffStorePrefix, append(refs, m.importRefs[f.tokens[i].text])...)
				}
			}
		}
		return
	}
	if strings.Contains(name, "/tools/[...resource]/") {
		defer func() { s.filterMethods(m, before); s.forwardingMethod(m, before) }()
		s.admit(m, "toolsRoute", bffStorePrefix, refs...)
		s.admit(m, "trackingRoute", bffStorePrefix, refs...)
		return
	}
	if strings.Contains(name, "/reports/[report]/") {
		defer func() { s.filterMethods(m, before); s.forwardingMethod(m, before) }()
		s.admit(m, "reportsRoute", bffStorePrefix, refs...)
		return
	}
	if strings.Contains(name, "/imports/[param]/[action]/") {
		defer func() { s.filterMethods(m, before); s.forwardingMethod(m, before) }()
		helper, _, e := s.bindingFunc(m, "importProxy")
		if e != nil {
			addUnresolved(&s.out, refs[0], e.Error())
		} else {
			s.admit(helper, "importRoute", bffStorePrefix, append(refs, helper.ref(helper.tokens))...)
			s.sinkContext(helper, helper.funcs["importReply"].tokens)
			s.forwardingMethod(helper, before)
			// The adapter supplies native fetch in this one verified parameter slot.
			for _, call := range bffCalls(m, m.tokens, "importProxy") {
				args := bffSplit(call, ",")
				if len(args) == 5 && len(args[4]) == 1 && args[4][0].text == "fetch" {
					s.consumeValue(m, args[4][0])
				}
			}
			for _, call := range bffCalls(helper, helper.funcs["importProxy"].tokens, "importReply") {
				args := bffSplit(call, ",")
				if len(args) == 5 && len(args[4]) == 1 && args[4][0].text == "transport" {
					s.consumeValue(helper, args[4][0])
				}
			}
		}
		return
	}
	if strings.Contains(name, "/buyer/[...path]/") {
		s.buyer(m, refs...)
		return
	}
	if calls := bffCalls(m, m.tokens, "customerTagsBFF"); len(calls) > 0 {
		owner, adapter, e := s.bindingFunc(m, "customerTagsBFF")
		if e != nil {
			addUnresolved(&s.out, refs[0], e.Error())
			return
		}
		// Consume exactly the frozen adapter's forward slot, not every occurrence
		// of callBackend in the helper or any other object named forward.
		for _, call := range bffCalls(owner, adapter.tokens, "proxyCustomerTags") {
			args := bffSplit(call, ",")
			if len(args) != 4 || len(args[3]) < 2 || args[3][0].text != "{" {
				continue
			}
			for _, field := range bffSplit(args[3][1:len(args[3])-1], ",") {
				if len(field) == 3 && field[0].text == "forward" && field[1].text == ":" && field[2].text == "callBackend" {
					s.consumeValue(owner, field[2])
				}
			}
		}
		gram, e := s.reader.load("apps/admin/lib/customer-tags-request.ts")
		if e != nil {
			addUnresolved(&s.out, refs[0], e.Error())
			return
		}
		temp := s.out
		s.out = inventory{}
		s.admit(gram, "customerTagsRoute", bffStorePrefix, append(refs, owner.ref(owner.tokens))...)
		candidate := s.out
		s.out = temp
		for _, call := range calls {
			args := bffSplit(call, ",")
			if len(args) < 3 {
				continue
			}
			v, e := s.dynamicPath(m, args[2])
			if e != nil {
				addUnresolved(&s.out, m.ref(call), e.Error())
				continue
			}
			for _, path := range v.texts {
				for _, r := range candidate.Routes {
					if r.Path == bffStorePrefix+path && exports[r.Method] {
						s.out.Routes = append(s.out.Routes, r)
					}
				}
			}
		}
		s.out.Unresolved = append(s.out.Unresolved, candidate.Unresolved...)
		if strings.Contains(name, "/customers/route.ts") {
			s.directSinks(m, exports, refs...)
		}
		return
	}
	if calls := bffCalls(m, m.tokens, "picklistProxy"); len(calls) > 0 {
		for _, call := range calls {
			args := bffSplit(call, ",")
			if len(args) < 3 || len(args[2]) != 1 {
				addUnresolved(&s.out, m.ref(call), "unsupported picklist kind")
				continue
			}
			kind := bffLiteral(args[2][0])
			helper, e := s.reader.load("apps/admin/lib/picklist-proxy.ts")
			if e != nil {
				addUnresolved(&s.out, m.ref(call), e.Error())
				continue
			}
			d, ok := helper.defs["path"]
			if !ok {
				addUnresolved(&s.out, m.ref(call), "missing picklist upstream expression")
				continue
			}
			v, e := s.reader.eval(helper, d.tokens, map[string]bffValue{"kind": {texts: []string{kind}}, "template": {texts: []string{"{}"}}}, 0)
			if e != nil {
				addUnresolved(&s.out, d.ref, e.Error())
			} else {
				s.sinkContext(helper, helper.funcs["picklistProxy"].tokens)
				consumedPath := false
				for _, sink := range bffCalls(helper, helper.funcs["picklistProxy"].tokens, "callBackend") {
					args := bffSplit(sink, ",")
					if len(args) < 2 {
						continue
					}
					if consumedPath || len(args[0]) != 1 || args[0][0].text != "path" {
						continue
					}
					consumedPath = true
					s.consumeSink(helper, sink)
					method, e := bffStaticMethod(args[1])
					if e != nil {
						addUnresolved(&s.out, helper.ref(sink), e.Error())
					} else {
						s.emit(method, bffStorePrefix, v, append(append(refs, d.ref), helper.ref(sink))...)
					}
				}
			}
		}
		return
	}
	if strings.Contains(name, "/auth/password/") {
		s.password(m, refs...)
		return
	}
	if strings.Contains(name, "/shop/") {
		s.shop(m, refs...)
		return
	}
	s.directSinks(m, exports, refs...)
	if strings.HasSuffix(name, "/app/api/stores/route.ts") {
		owner, _, e := s.bindingFunc(m, "authenticatedStores")
		if e != nil {
			addUnresolved(&s.out, refs[0], e.Error())
		} else {
			s.authenticatedStores(owner, refs...)
		}
	}
	if len(s.out.Routes) == before && len(exports) > 0 {
		addUnresolved(&s.out, refs[0], "unaccounted BFF leaf forwarding/handler shape")
	}
}
func (s *bffScanner) dynamicPath(m *bffModule, t []bffToken) (bffValue, error) {
	if len(t) == 1 && t[0].kind == "template" {
		raw := t[0].text // Query values do not affect METHOD/path comparison.
		if q := strings.Index(raw, "?"); q >= 0 {
			raw = raw[:q] + "`"
		}
		env := map[string]bffValue{}
		for _, part := range strings.Split(m.name, "/") {
			if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") && !strings.HasPrefix(part, "[...") {
				env[strings.Trim(part, "[]")] = bffValue{texts: []string{"{}"}}
			}
		}
		raw = strings.ReplaceAll(raw, "${new URL(request.url).search}", "")
		copy := t[0]
		copy.text = raw
		return s.reader.eval(m, []bffToken{copy}, env, 0)
	}
	return s.reader.eval(m, t, nil, 0)
}
func (s *bffScanner) directSinks(m *bffModule, exports map[string]bool, refs ...location) {
	for _, sink := range []string{"privateIdentity", "callBackend", "merchantBackend", "fetch"} {
		for _, call := range bffCalls(m, m.tokens, sink) {
			s.consumeSink(m, call)
			args := bffSplit(call, ",")
			if len(args) == 0 {
				continue
			}
			method := ""
			prefix := bffStorePrefix
			if sink == "privateIdentity" {
				var e error
				var transportRefs []location
				method, transportRefs, e = s.identitySink(m)
				if e != nil {
					addUnresolved(&s.out, m.ref(call), e.Error())
					continue
				}
				refs = append(refs, transportRefs...)
				prefix = "/v1/identity/"
			}
			if method == "" && len(args) > 1 {
				for i := 0; i+2 < len(args[1]); i++ {
					if args[1][i].text == "method" && args[1][i+1].text == ":" && args[1][i+2].kind == "string" {
						method = bffLiteral(args[1][i+2])
					}
				}
			}
			if method == "" {
				for m := range exports {
					if method != "" {
						method = ""
						break
					}
					method = m
				}
			}
			if method == "" {
				addUnresolved(&s.out, m.ref(call), "dynamic upstream method without closed admission")
				continue
			}
			if sink == "privateIdentity" && len(args[0]) == 1 && strings.Contains(args[0][0].text, "${name}") {
				actions, e := s.reader.symbol(m, "teamActions", nil, 0)
				if e != nil {
					addUnresolved(&s.out, m.ref(call), e.Error())
					continue
				}
				for _, a := range actions.array {
					v, e := s.reader.eval(m, args[0], map[string]bffValue{"name": a}, 0)
					if e != nil {
						addUnresolved(&s.out, m.ref(call), e.Error())
					} else {
						s.emit(method, prefix, v, append(refs, actions.refs...)...)
					}
				}
				continue
			}
			v, e := s.dynamicPath(m, args[0])
			if e != nil {
				addUnresolved(&s.out, m.ref(call), e.Error())
				continue
			}
			if sink == "fetch" {
				prefix = ""
				for i, p := range v.texts {
					at := strings.Index(p, "/v1/")
					if at < 0 {
						e = fmt.Errorf("fetch URL has no fixed API path")
						break
					}
					v.texts[i] = p[at:]
				}
			}
			if e != nil {
				addUnresolved(&s.out, m.ref(call), e.Error())
			} else {
				s.emit(method, prefix, v, append(refs, m.ref(call))...)
			}
		}
	}
}
func (s *bffScanner) authenticatedStores(m *bffModule, refs ...location) {
	f, ok := m.funcs["authenticatedStores"]
	if !ok {
		addUnresolved(&s.out, m.ref(m.tokens), "unknown stores list helper")
		return
	}
	for _, call := range bffCalls(m, f.tokens, "merchantBackend") {
		args := bffSplit(call, ",")
		if len(args) < 3 || len(args[0]) != 1 || bffLiteral(args[0][0]) != "/v1/admin/stores" {
			continue
		}
		s.consumeSink(m, call)
		method, more, err := s.requestMethod(m, args[2], "", 0)
		if err != nil {
			addUnresolved(&s.out, m.ref(call), err.Error())
			return
		}
		s.emit(method, "", bffValue{texts: []string{"/v1/admin/stores"}}, append(append(refs, f.ref), more...)...)
		return
	}
	addUnresolved(&s.out, f.ref, "stores list upstream path changed")
}
func (s *bffScanner) password(m *bffModule, refs ...location) {
	if calls := bffCalls(m, m.tokens, "handleStep1"); len(calls) > 0 {
		helper, f, e := s.bindingFunc(m, "handleStep1")
		if e != nil {
			addUnresolved(&s.out, refs[0], e.Error())
			return
		}
		for _, call := range calls {
			args := bffSplit(call, ",")
			if len(args) < 2 {
				addUnresolved(&s.out, m.ref(call), "unknown password purpose")
				continue
			}
			purpose, e := s.reader.eval(m, args[1], nil, 0)
			if e != nil {
				addUnresolved(&s.out, m.ref(call), e.Error())
				continue
			}
			for _, sink := range bffCalls(helper, f.tokens, "privateIdentity") {
				s.consumeSink(helper, sink)
				args := bffSplit(sink, ",")
				v, e := s.reader.eval(helper, args[0], map[string]bffValue{"purpose": purpose}, 0)
				if e != nil {
					addUnresolved(&s.out, helper.ref(sink), e.Error())
				} else {
					method, transportRefs, e := s.identitySink(helper)
					if e != nil {
						addUnresolved(&s.out, helper.ref(sink), e.Error())
					} else {
						s.emit(method, "/v1/identity/", v, append(append(refs, f.ref), transportRefs...)...)
					}
				}
			}
		}
		return
	}
	helper, f, e := s.bindingFunc(m, "handleVerify")
	if e != nil {
		addUnresolved(&s.out, refs[0], e.Error())
		return
	}
	for _, sink := range bffCalls(helper, f.tokens, "privateIdentity") {
		s.consumeSink(helper, sink)
		args := bffSplit(sink, ",")
		method, transportRefs, e := s.identitySink(helper)
		if e != nil {
			addUnresolved(&s.out, helper.ref(sink), e.Error())
		} else {
			s.evalEmit(helper, args[0], method, "/v1/identity/", append(append(refs, f.ref), transportRefs...)...)
		}
	}
}
func (s *bffScanner) shop(m *bffModule, refs ...location) {
	for name, imp := range m.imports {
		if !strings.Contains(imp[0], "shop-upstream") {
			continue
		}
		helper, e := s.reader.load(imp[0])
		if e != nil {
			addUnresolved(&s.out, refs[0], e.Error())
			continue
		}
		d, ok := helper.defs[imp[1]]
		if !ok {
			addUnresolved(&s.out, refs[0], "unsupported shop helper "+name)
			continue
		}
		method := ""
		transportRefs := []location{m.importRefs[name]}
		s.sinkContext(helper, helper.funcs["getJSON"].tokens)
		consumedPath := false
		for _, sink := range bffCalls(helper, helper.funcs["getJSON"].tokens, "fetch") {
			if consumedPath || len(sink) == 0 || !strings.HasSuffix(sink[0].text, "/v1/buyer/${path}`") {
				continue
			}
			consumedPath = true
			s.consumeSink(helper, sink)
			args := bffSplit(sink, ",")
			if len(args) < 2 {
				continue
			}
			method, e = bffStaticMethod(args[1])
			transportRefs = append(transportRefs, helper.ref(sink))
			if len(args[0]) != 1 || !strings.Contains(args[0][0].text, "/v1/buyer/${path}") {
				e = fmt.Errorf("shop upstream prefix shape changed")
			}
			if e != nil {
				addUnresolved(&s.out, helper.ref(sink), e.Error())
			}
		}
		if method == "" {
			addUnresolved(&s.out, d.ref, "shop upstream method unresolved")
			continue
		}
		refs = append(refs, transportRefs...)
		for _, call := range bffCalls(helper, d.tokens, "getJSON") {
			args := bffSplit(call, ",")
			if len(args) == 0 {
				continue
			}
			t := args[0]
			if len(t) == 1 && t[0].kind == "template" {
				raw := t[0].text
				at := strings.Index(raw, "${encodeURIComponent(")
				if at >= 0 {
					raw = raw[:at] + "{}`"
				}
				copy := t[0]
				copy.text = raw
				s.evalEmit(helper, []bffToken{copy}, method, "/v1/buyer/", append(refs, d.ref)...)
			} else if len(t) > 1 && t[0].text == "listPath" {
				f := helper.funcs["listPath"]
				for _, x := range f.tokens {
					if x.kind == "template" && strings.Contains(x.text, "${qs") {
						p := strings.SplitN(x.text[1:], "${", 2)[0]
						s.emit(method, "/v1/buyer/", bffValue{texts: []string{p}}, append(append(refs, d.ref), f.ref)...)
						break
					}
				}
			} else {
				s.evalEmit(helper, t, method, "/v1/buyer/", append(refs, d.ref)...)
			}
		}
	}
}
func (s *bffScanner) primaryOrigin(m *bffModule) {
	if len(bffCalls(m, m.tokens, "canonicalRedirect")) == 0 {
		return
	}
	helper, _, e := s.bindingFunc(m, "canonicalRedirect")
	if e != nil {
		addUnresolved(&s.out, m.ref(m.tokens), e.Error())
		return
	}
	for _, call := range bffCalls(helper, helper.tokens, "fetcher") {
		s.consumeSink(helper, call)
		args := bffSplit(call, ",")
		if len(args) == 0 || len(args[0]) != 1 {
			addUnresolved(&s.out, helper.ref(call), "unknown primary-origin upstream")
			continue
		}
		raw := args[0][0].text
		at := strings.Index(raw, "/v1/")
		if at < 0 {
			addUnresolved(&s.out, helper.ref(call), "missing primary-origin path")
			continue
		}
		p := strings.TrimSuffix(raw[at:], "`")
		if len(args) < 2 {
			addUnresolved(&s.out, helper.ref(call), "primary-origin method unresolved")
			continue
		}
		method, e := bffStaticMethod(args[1])
		if e != nil {
			addUnresolved(&s.out, helper.ref(call), e.Error())
		} else {
			s.emit(method, "", bffValue{texts: []string{p}}, m.ref(m.tokens), helper.ref(call))
		}
	}
}
