// Purpose: derive dedicated live-settings leaf routes from source admission and transport.
// Depends on: finite TypeScript reader, method/export resolution and sink accounting.
// Used by: leaf and CI-DRIFT; never reads Go routes or a pinned route list.
// Invariant: consume only the proven native-fetch slot; other/unknown sinks remain findings.
package main

import (
	"fmt"
	"strings"
)

func (s *bffScanner) liveSettings(leaf *bffModule, refs ...location) {
	fail := func(ref location, why string) { addUnresolved(&s.out, ref, "live-settings: "+why) }
	helper, forward, err := s.bindingFunc(leaf, "liveSettingsProxy")
	if err != nil {
		fail(refs[0], err.Error())
		return
	}
	refs = append(refs, leaf.importRefs["liveSettingsProxy"], forward.ref)
	// These closed source seams bind admission to the actual path and request method.
	body := bffText(forward.tokens)
	if !strings.Contains(body, "const route = settingsResource ( path )") ||
		!bffLiveMethodGuard(forward.tokens) {
		fail(forward.ref, "unsupported admission guard")
		return
	}
	model, admit, err := s.bindingFunc(helper, "settingsResource")
	if err != nil {
		fail(forward.ref, err.Error())
		return
	}
	if strings.TrimSpace(bffText(admit.tokens)) != "const r = resources . find ( ( [ , re ] ) => re . test ( path ) ) ; return r ? { kind : r [ 0 ] , methods : r [ 2 ] } : null ;" {
		fail(admit.ref, "unsupported resource resolver")
		return
	}
	resources, err := s.reader.symbol(model, "resources", nil, 0)
	if err != nil || len(resources.array) == 0 {
		fail(admit.ref, fmt.Sprintf("non-finite resources: %v", err))
		return
	}
	refs = append(refs, admit.ref)
	// Prove precisely one shared slot, retaining changed-method provenance. A changed
	// URL stays unresolved; no route can be inferred from admission alone.
	var transport []bffToken
	var sinkMethod string
	for _, call := range bffCalls(helper, forward.tokens, "transport") {
		args := bffSplit(call, ",")
		if len(args) > 0 && len(args[len(args)-1]) == 0 {
			args = args[:len(args)-1]
		}
		if len(args) != 2 || len(args[0]) != 1 || args[0][0].kind != "template" || args[0][0].text != "`${authConfig.apiOrigin}/v1/admin/stores/${store}/${path}${new URL(request.url).search}`" {
			continue
		}
		if transport != nil {
			fail(helper.ref(call), "multiple resource transport slots")
			return
		}
		var more []location
		sinkMethod, more, err = s.liveSettingsMethod(helper, args[1])
		if err != nil {
			fail(helper.ref(call), err.Error())
			return
		}
		refs = append(refs, helper.ref(call))
		refs = append(refs, more...)
		transport = call
	}
	if transport == nil {
		fail(forward.ref, "unproven resource transport URL")
		return
	}
	s.consumeSink(helper, transport)
	for _, call := range bffCalls(leaf, leaf.tokens, "liveSettingsProxy") {
		args := bffSplit(call, ",")
		if len(args) != 4 || strings.TrimSpace(bffText(args[0])) != "request" || strings.TrimSpace(bffText(args[1])) != "p . store" || len(args[3]) != 1 || args[3][0].text != "fetch" {
			fail(leaf.ref(call), "unsupported leaf transport arguments")
			continue
		}
		params, ok := leaf.defs["p"]
		if !ok || strings.TrimSpace(bffText(params.tokens)) != "await context . params" {
			fail(leaf.ref(call), "unproven leaf parameters")
			continue
		}
		env := map[string]bffValue{"p": {object: map[string]bffValue{}}}
		for _, part := range strings.Split(leaf.name, "/") {
			if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") && !strings.HasPrefix(part, "[...") {
				env["p"].object[strings.Trim(part, "[]")] = bffValue{texts: []string{"{}"}}
			}
		}
		path, err := s.reader.eval(leaf, args[2], env, 0)
		if err != nil || len(path.texts) != 1 {
			fail(leaf.ref(call), "non-finite leaf resource")
			continue
		}
		// A fetch value is consumed at this exact argument token only. Alias values,
		// member accesses and extra uses elsewhere are still finalized by the graph.
		s.consumeValue(leaf, args[3][0])
		callRefs := append(append([]location{}, refs...), leaf.ref(call), params.ref)
		matched := false
		for _, row := range resources.array {
			if len(row.array) != 3 || !row.array[1].re || len(row.array[1].texts) != 1 {
				fail(admit.ref, "unsupported resource tuple")
				continue
			}
			alternatives, err := bffExpand(row.array[1].texts[0])
			if err != nil {
				fail(admit.ref, err.Error())
				continue
			}
			for _, p := range alternatives {
				if p != path.texts[0] {
					continue
				}
				matched = true
				for _, method := range row.array[2].array {
					if len(method.texts) != 1 || !bffHTTP(method.texts[0]) {
						fail(admit.ref, "non-finite admission method")
						continue
					}
					verb := method.texts[0]
					if !bffExports(leaf)[verb] || !s.forwardingExport(leaf, verb, map[string]bool{}, 0) {
						continue
					}
					if sinkMethod != "" {
						verb = sinkMethod
					}
					s.emit(verb, bffStorePrefix, path, append(append(callRefs, row.refs...), method.refs...)...)
				}
			}
		}
		if !matched {
			fail(leaf.ref(call), "leaf resource not admitted")
		}
	}
}

// The real helper uses one shorthand body spread, which cannot override method.
// Keep every other field/spread in source order for the existing RequestInit proof.
func (s *bffScanner) liveSettingsMethod(m *bffModule, options []bffToken) (string, []location, error) {
	if len(options) < 2 || options[0].text != "{" || bffClose(options, 0) != len(options)-1 {
		return "", nil, fmt.Errorf("unsupported transport options")
	}
	normalized := []bffToken{options[0]}
	for _, field := range bffSplit(options[1:len(options)-1], ",") {
		if len(field) == 0 {
			continue
		}
		if strings.TrimSpace(bffText(field)) == "... ( body === undefined ? { } : { body } )" {
			continue
		}
		// A computed literal key has the same override order as an ordinary key.
		// Dynamic keys/accessors could replace method and must fail closed.
		if field[0].text == "[" {
			if len(field) < 5 || field[1].kind != "string" || field[2].text != "]" || field[3].text != ":" {
				return "", []location{m.ref(field)}, fmt.Errorf("unknown computed transport key")
			}
			field = append([]bffToken{field[1]}, field[3:]...)
		}
		if field[0].text != "..." && (len(field) < 3 || (field[0].kind != "ident" && field[0].kind != "string") || field[1].text != ":") {
			return "", []location{m.ref(field)}, fmt.Errorf("unsupported transport member")
		}
		normalized = append(normalized, field...)
		normalized = append(normalized, bffToken{text: ",", kind: "punct", start: field[len(field)-1].end, end: field[len(field)-1].end})
	}
	normalized = append(normalized, options[len(options)-1])
	method, refs, err := s.requestMethod(m, normalized, "request . method", 0)
	return method, append(refs, m.ref(options)), err
}

// Require the exact rejecting branch, not a substring inside a bypassed condition.
func bffLiveMethodGuard(t []bffToken) bool {
	for i := 0; i+1 < len(t); i++ {
		// Inspect the helper's outer statement list only. A function/block's
		// nested rejecting branch does not constrain the reached transport.
		if t[i].text == "(" || t[i].text == "[" || t[i].text == "{" {
			end := bffClose(t, i)
			if end < 0 {
				return false
			}
			i = end
			continue
		}
		if t[i].text != "if" || t[i+1].text != "(" {
			continue
		}
		end := bffClose(t, i+1)
		if end < 0 {
			return false
		}
		if strings.TrimSpace(bffText(t[i+2:end])) != "! route . methods . includes ( request . method )" {
			// Also skip unbraced conditional statements; their nested if is not
			// an unconditional outer guard. Unknown shapes stay unsupported.
			i = end
			if i+1 < len(t) && t[i+1].text == "{" {
				i = bffClose(t, i+1)
				if i < 0 {
					return false
				}
				continue
			}
			for i+1 < len(t) && t[i+1].text != ";" {
				i++
				if t[i].text == "(" || t[i].text == "[" || t[i].text == "{" {
					i = bffClose(t, i)
					if i < 0 {
						return false
					}
				}
			}
			continue
		}
		if end+1 >= len(t) || t[end+1].text != "{" {
			return false
		}
		close := bffClose(t, end+1)
		if close < 0 {
			return false
		}
		branch := strings.TrimSpace(bffText(t[end+2 : close]))
		if branch != "const r = denied ( 405 , \"method_not_allowed\" ) ; r . headers . set ( \"Allow\" , route . methods . join ( \", \" ) ) ; return r ;" {
			return false
		}
		for _, call := range bffCalls(nil, t, "transport") {
			if len(call) > 0 && call[0].start < t[close].end {
				return false
			}
		}
		return true
	}
	return false
}
