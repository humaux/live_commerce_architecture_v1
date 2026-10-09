// Purpose: account for each forwarding call in a producer and its reached helper contexts.
// Depends on: finite TypeScript tokens/values and BFF adapters; Go stdlib only.
// Used by: producer finalization; call identities are scoped to the actual root producer.
// Invariant: adapters consume individual proven calls; remaining calls are extracted or unresolved.
package main

import (
	"fmt"
	"sort"
	"strings"
)

type bffSinkContext struct {
	m      *bffModule
	tokens []bffToken
	refs   []location
}

func bffCallKey(m *bffModule, call []bffToken) string {
	if len(call) == 0 {
		return m.name + ":empty"
	}
	return fmt.Sprintf("%s:%d:%d", m.name, call[0].start, call[len(call)-1].end)
}
func (s *bffScanner) sinkContext(m *bffModule, tokens []bffToken, refs ...location) {
	if s.contexts == nil {
		s.contexts = map[string]bffSinkContext{}
	}
	key := bffCallKey(m, tokens)
	prior := s.contexts[key]
	s.contexts[key] = bffSinkContext{m: m, tokens: tokens, refs: bffUniqueRefs(append(prior.refs, refs...))}
}
func (s *bffScanner) consumeSink(m *bffModule, call []bffToken) {
	if s.consumed == nil {
		s.consumed = map[string]bool{}
	}
	s.consumed[bffCallKey(m, call)] = true
}

func (s *bffScanner) consumeValue(m *bffModule, t bffToken) {
	if s.consumedValues == nil {
		s.consumedValues = map[string]bool{}
	}
	s.consumedValues[fmt.Sprintf("%s:%d", m.name, t.start)] = true
}

func bffSinkName(name string) bool {
	switch name {
	case "fetch", "fetcher", "transport", "callBackend", "merchantBackend", "privateIdentity":
		return true
	}
	return false
}

// Resolve only top-level RequestInit members, respecting spread/override order.
// An absent method is fetch's GET default; it is never an admission passthrough.
func (s *bffScanner) requestMethod(m *bffModule, options []bffToken, pass string, depth int) (string, []location, error) {
	if depth > 16 {
		return "", nil, fmt.Errorf("cyclic forwarding options")
	}
	refs := []location{m.ref(options)}
	if len(options) == 0 {
		return "GET", refs, nil
	}
	if len(options) == 1 && options[0].kind == "ident" {
		d, ok := m.defs[options[0].text]
		if !ok {
			return "", refs, fmt.Errorf("unknown forwarding options %s", options[0].text)
		}
		method, more, err := s.requestMethod(m, d.tokens, pass, depth+1)
		return method, append(append(refs, d.ref), more...), err
	}
	if options[0].text != "{" || bffClose(options, 0) != len(options)-1 {
		return "", refs, fmt.Errorf("unsupported forwarding options")
	}
	method := "GET"
	var methodErr error
	for _, field := range bffSplit(options[1:len(options)-1], ",") {
		if len(field) == 0 {
			continue
		}
		if field[0].text == "..." {
			if s.withoutMethod(m, field[1:], depth+1) {
				continue
			}
			var more []location
			method, more, methodErr = s.requestMethod(m, field[1:], pass, depth+1)
			refs = append(refs, more...)
			continue
		}
		if field[0].text != "method" && bffLiteral(field[0]) != "method" {
			continue
		}
		value := field
		if len(field) > 1 && field[1].text == ":" {
			value = field[2:]
		}
		switch {
		case pass != "" && strings.TrimSpace(bffText(value)) == pass:
			method, methodErr = "", nil
		case len(value) == 1 && value[0].kind == "string" && bffHTTP(bffLiteral(value[0])):
			method, methodErr = bffLiteral(value[0]), nil
		default:
			method, methodErr = "", fmt.Errorf("unsupported forwarding method expression")
		}
	}
	return method, refs, methodErr
}

// A spread containing only body/header fields cannot override method, even when
// its condition is dynamic. Unknown objects still fail closed.
func (s *bffScanner) withoutMethod(m *bffModule, t []bffToken, depth int) bool {
	if len(t) == 0 || depth > 16 {
		return false
	}
	if t[0].text == "(" && bffClose(t, 0) == len(t)-1 {
		return s.withoutMethod(m, t[1:len(t)-1], depth+1)
	}
	if len(t) == 1 && t[0].kind == "ident" {
		d, ok := m.defs[t[0].text]
		return ok && s.withoutMethod(m, d.tokens, depth+1)
	}
	if t[0].text == "{" && bffClose(t, 0) == len(t)-1 {
		for _, f := range bffSplit(t[1:len(t)-1], ",") {
			if len(f) == 0 {
				continue
			}
			if f[0].text == "..." {
				if !s.withoutMethod(m, f[1:], depth+1) {
					return false
				}
				continue
			}
			if len(f) < 2 || f[1].text != ":" || f[0].text == "method" || bffLiteral(f[0]) == "method" {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(t); i++ {
		if t[i].text == "(" || t[i].text == "[" || t[i].text == "{" {
			end := bffClose(t, i)
			if end < 0 {
				return false
			}
			i = end
			continue
		}
		if t[i].text == "?" {
			for j := i + 1; j < len(t); j++ {
				if t[j].text == "(" || t[j].text == "[" || t[j].text == "{" {
					end := bffClose(t, j)
					if end < 0 {
						return false
					}
					j = end
					continue
				}
				if t[j].text == ":" {
					return s.withoutMethod(m, t[i+1:j], depth+1) && s.withoutMethod(m, t[j+1:], depth+1)
				}
			}
		}
	}
	return false
}

func (s *bffScanner) remainingSinks() {
	visited := map[string]bool{}
	for {
		keys := []string{}
		for key := range s.contexts {
			if !visited[key] {
				keys = append(keys, key)
			}
		}
		if len(keys) == 0 {
			return
		}
		sort.Strings(keys)
		for _, key := range keys {
			visited[key] = true
			ctx := s.contexts[key]
			m := ctx.m
			for _, sink := range []string{"fetch", "fetcher", "transport", "callBackend", "merchantBackend", "privateIdentity"} {
				for _, call := range bffCalls(m, ctx.tokens, sink) {
					if s.consumed[bffCallKey(m, call)] || s.closedSinks[bffCallKey(m, call)] {
						continue
					}
					s.consumeSink(m, call)
					args := bffSplit(call, ",")
					if len(args) == 0 || len(args[0]) == 0 {
						s.sinkUnknown(ctx, call, "empty forwarding sink")
						continue
					}
					refs := []location{m.ref(call)}
					method, prefix := "", bffStorePrefix
					var err error
					if sink == "privateIdentity" {
						var more []location
						method, more, err = s.identitySink(m)
						refs = append(refs, more...)
						prefix = "/v1/identity/"
					} else {
						at := 1
						if sink == "merchantBackend" {
							at = 2
							prefix = ""
						}
						var options []bffToken
						if len(args) > at {
							options = args[at]
						}
						var more []location
						method, more, err = s.requestMethod(m, options, "", 0)
						refs = append(refs, more...)
					}
					if err != nil {
						s.sinkUnknown(ctx, call, err.Error())
						continue
					}
					v, err := s.dynamicPath(m, args[0])
					if err == nil && (sink == "fetch" || sink == "fetcher" || sink == "transport") {
						prefix = ""
						for i, p := range v.texts {
							at := strings.Index(p, "/v1/")
							if at < 0 {
								err = fmt.Errorf("extra fetch URL has no fixed API path")
								break
							}
							v.texts[i] = p[at:]
						}
					}
					if err != nil {
						s.sinkUnknown(ctx, call, err.Error())
						continue
					}
					s.emit(method, prefix, v, append(refs, ctx.refs...)...)
				}
			}
		}
	}
}

func (s *bffScanner) sinkUnknown(ctx bffSinkContext, call []bffToken, detail string) {
	addUnresolved(&s.out, ctx.m.ref(call), detail)
	for _, ref := range ctx.refs {
		addUnresolved(&s.out, ref, detail)
	}
}
