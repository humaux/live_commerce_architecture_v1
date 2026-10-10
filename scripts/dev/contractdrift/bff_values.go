// Purpose: resolve finite TypeScript constants, imported fragments and templates.
// Depends on: bff_ts.go lexical modules; fmt and strings only.
// Used by: scanBFF and the CI-DRIFT gate; never executes application code.
// Invariant: unsupported or cyclic values produce explicit errors, not guessed routes.
package main

import (
	"fmt"
	"strings"
)

func (r *bffReader) symbol(m *bffModule, name string, env map[string]bffValue, depth int) (bffValue, error) {
	if depth > 32 {
		return bffValue{}, fmt.Errorf("cyclic/deep binding %s", name)
	}
	if v, ok := env[name]; ok {
		return v, nil
	}
	if d, ok := m.defs[name]; ok {
		v, e := r.eval(m, d.tokens, env, depth+1)
		v.refs = append(v.refs, d.ref)
		return v, e
	}
	if imp, ok := m.imports[name]; ok {
		other, e := r.load(imp[0])
		if e != nil {
			return bffValue{}, e
		}
		v, e := r.symbol(other, imp[1], env, depth+1)
		v.refs = append(v.refs, m.importRefs[name])
		return v, e
	}
	return bffValue{}, fmt.Errorf("unknown binding %s", name)
}
func bffProduct(a, b []string) ([]string, error) {
	if len(a)*len(b) > 4096 {
		return nil, fmt.Errorf("finite expansion exceeds 4096")
	}
	var out []string
	for _, x := range a {
		for _, y := range b {
			out = append(out, x+y)
		}
	}
	return out, nil
}
func (r *bffReader) eval(m *bffModule, t []bffToken, env map[string]bffValue, depth int) (bffValue, error) {
	if len(t) == 0 {
		return bffValue{}, fmt.Errorf("empty expression")
	}
	if depth > 32 {
		return bffValue{}, fmt.Errorf("expression depth")
	}
	// Constant conditionals are needed for per-leaf adapter paths. Unknown
	// conditions remain unresolved rather than selecting one convenient branch.
	for i := 0; i < len(t); i++ {
		if t[i].text == "(" || t[i].text == "[" || t[i].text == "{" {
			end := bffClose(t, i)
			if end < 0 {
				break
			}
			i = end
			continue
		}
		if t[i].text != "?" {
			continue
		}
		colon := -1
		nest := 0
		for j := i + 1; j < len(t); j++ {
			if t[j].text == "?" {
				nest++
			}
			if t[j].text == ":" {
				if nest == 0 {
					colon = j
					break
				}
				nest--
			}
		}
		if colon < 0 {
			return bffValue{}, fmt.Errorf("unbalanced conditional")
		}
		cond := t[:i]
		if len(cond) < 3 || cond[len(cond)-2].text != "===" || cond[len(cond)-1].kind != "string" {
			return bffValue{}, fmt.Errorf("unsupported conditional")
		}
		left, e := r.eval(m, cond[:len(cond)-2], env, depth+1)
		if e != nil || len(left.texts) != 1 {
			return bffValue{}, fmt.Errorf("non-finite conditional")
		}
		branch := t[colon+1:]
		if left.texts[0] == bffLiteral(cond[len(cond)-1]) {
			branch = t[i+1 : colon]
		}
		v, e := r.eval(m, branch, env, depth+1)
		v.refs = append(v.refs, left.refs...)
		return v, e
	}
	if t[0].text == "(" && bffClose(t, 0) == len(t)-1 {
		return r.eval(m, t[1:len(t)-1], env, depth+1)
	}
	parts := bffSplit(t, "+")
	if len(parts) > 1 {
		v := bffValue{texts: []string{""}}
		for _, p := range parts {
			x, e := r.eval(m, p, env, depth+1)
			if e != nil {
				return v, e
			}
			v.texts, e = bffProduct(v.texts, x.texts)
			v.refs = append(v.refs, x.refs...)
			if e != nil {
				return v, e
			}
		}
		return v, nil
	}
	if t[0].text == "new" && len(t) > 3 && t[1].text == "RegExp" && t[2].text == "(" {
		end := bffClose(t, 2)
		if end < 0 {
			return bffValue{}, fmt.Errorf("unbalanced RegExp")
		}
		args := bffSplit(t[3:end], ",")
		if len(args) > 1 && len(args[1]) > 0 {
			flags, e := r.eval(m, args[1], env, depth+1)
			if e != nil || len(flags.texts) != 1 || flags.texts[0] != "" {
				return bffValue{}, fmt.Errorf("unsupported RegExp flags")
			}
		}
		v, e := r.eval(m, args[0], env, depth+1)
		v.re = true
		v.refs = append(v.refs, m.ref(t[:end+1]))
		return v, e
	}
	if t[0].text == "{" {
		end := bffClose(t, 0)
		if end < 0 {
			return bffValue{}, fmt.Errorf("unbalanced object")
		}
		v := bffValue{object: map[string]bffValue{}}
		for _, entry := range bffSplit(t[1:end], ",") {
			if len(entry) == 0 {
				continue
			}
			if entry[0].text == "..." {
				x, e := r.eval(m, entry[1:], env, depth+1)
				if e != nil {
					return v, e
				}
				for k, y := range x.object {
					y.refs = append(y.refs, x.refs...)
					v.object[k] = y
				}
				continue
			}
			if len(entry) < 3 || entry[1].text != ":" {
				return v, fmt.Errorf("unsupported object entry")
			}
			key := entry[0].text
			if entry[0].kind == "string" {
				key = bffUnquote(entry[0])
			}
			x, e := r.eval(m, entry[2:], env, depth+1)
			if e != nil {
				x.refs = append(x.refs, m.ref(entry))
				x.texts = []string{"!UNRESOLVED:" + e.Error()}
			}
			x.refs = append(x.refs, m.ref(entry))
			v.object[key] = x
		}
		return v, nil
	}
	if t[0].text == "[" {
		end := bffClose(t, 0)
		if end < 0 {
			return bffValue{}, fmt.Errorf("unbalanced array")
		}
		v := bffValue{}
		for _, p := range bffSplit(t[1:end], ",") {
			if len(p) == 0 {
				continue
			}
			x, e := r.eval(m, p, env, depth+1)
			if e != nil {
				return v, e
			}
			v.array = append(v.array, x)
			v.refs = append(v.refs, x.refs...)
		}
		return v, nil
	}
	if len(t) == 1 && (t[0].kind == "string" || t[0].kind == "regex") {
		if t[0].kind == "regex" && strings.LastIndex(t[0].text, "/") != len(t[0].text)-1 {
			return bffValue{}, fmt.Errorf("unsupported regex flags")
		}
		return bffValue{texts: []string{bffUnquote(t[0])}, refs: []location{m.ref(t)}, re: t[0].kind == "regex"}, nil
	}
	if len(t) == 1 && t[0].kind == "template" {
		raw := t[0].text[1 : len(t[0].text)-1]
		v := bffValue{texts: []string{""}, refs: []location{m.ref(t)}}
		for len(raw) > 0 {
			start := strings.Index(raw, "${")
			if start < 0 {
				v.texts, _ = bffProduct(v.texts, []string{bffUnquote(bffToken{text: "`" + raw + "`"})})
				break
			}
			v.texts, _ = bffProduct(v.texts, []string{bffUnquote(bffToken{text: "`" + raw[:start] + "`"})})
			end := start + 2
			brace := 1
			for end < len(raw) && brace > 0 {
				switch raw[end] {
				case '"', '\'', '`':
					end = bffQuotedEnd(raw, end)
					continue
				case '{':
					brace++
				case '}':
					brace--
				}
				if brace > 0 {
					end++
				}
			}
			if brace > 0 {
				return v, fmt.Errorf("unbalanced template")
			}
			expr := raw[start+2 : end]
			x, e := r.eval(m, bffLex(expr), env, depth+1)
			if e != nil {
				return v, e
			}
			v.texts, e = bffProduct(v.texts, x.texts)
			if e != nil {
				return v, e
			}
			v.refs = append(v.refs, x.refs...)
			raw = raw[end+1:]
		}
		return v, nil
	}
	if t[0].kind == "ident" {
		v, e := r.symbol(m, t[0].text, env, depth+1)
		if e != nil {
			return v, e
		}
		for i := 1; i < len(t); {
			if t[i].text == "as" {
				return v, nil
			}
			if t[i].text == "." && i+1 < len(t) {
				x, ok := v.object[t[i+1].text]
				if !ok {
					return v, fmt.Errorf("unknown property %s", t[i+1].text)
				}
				x.refs = append(x.refs, v.refs...)
				v = x
				i += 2
			} else {
				return v, fmt.Errorf("unsupported expression after %s", t[0].text)
			}
		}
		return v, nil
	}
	return bffValue{}, fmt.Errorf("unsupported expression %s", t[0].text)
}
