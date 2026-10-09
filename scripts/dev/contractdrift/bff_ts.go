// Purpose: finite TypeScript source reader for BFF admission grammars.
// Depends on: Go stdlib only; used by bff.go, never executes TypeScript.
// Invariant: unsupported expressions fail explicitly; source spans follow every dependency.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type bffToken struct {
	text, kind string
	start, end int
}
type bffExpr struct {
	tokens []bffToken
	ref    location
}
type bffModule struct {
	name, source string
	tokens       []bffToken
	defs         map[string]bffExpr
	imports      map[string][2]string
	importRefs   map[string]location
	funcs        map[string]bffExpr
}
type bffReader struct {
	root    string
	modules map[string]*bffModule
}
type bffValue struct {
	texts  []string
	object map[string]bffValue
	array  []bffValue
	refs   []location
	re     bool
}

func bffLex(s string) []bffToken {
	var out []bffToken
	for i := 0; i < len(s); {
		if unicode.IsSpace(rune(s[i])) {
			i++
			continue
		}
		start := i
		c := s[i]
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && s[i:i+2] != "*/" {
				i++
			}
			i = min(i+2, len(s))
			continue
		}
		kind := "punct"
		if c == '"' || c == '\'' || c == '`' {
			kind = "string"
			if c == '`' {
				kind = "template"
			}
			i = bffQuotedEnd(s, i)
			if i > len(s) {
				kind = "invalid"
			}
		}
		if i == start && c == '/' {
			// A regex may start after an expression opener, assignment, comma, or return.
			prev := ""
			if len(out) > 0 {
				prev = out[len(out)-1].text
			}
			if prev == "=" || prev == ":" || prev == "(" || prev == "[" || prev == "," || prev == "return" || prev == "&&" || prev == "||" || prev == "!" {
				kind = "regex"
				i++
				class := false
				for i < len(s) {
					if s[i] == '\\' {
						i += 2
						continue
					}
					if s[i] == '[' {
						class = true
					}
					if s[i] == ']' {
						class = false
					}
					if s[i] == '/' && !class {
						i++
						break
					}
					i++
				}
				for i < len(s) && unicode.IsLetter(rune(s[i])) {
					i++
				}
			}
		}
		if i == start && (unicode.IsLetter(rune(c)) || c == '_' || c == '$' || unicode.IsDigit(rune(c))) {
			kind = "ident"
			i++
			for i < len(s) && (unicode.IsLetter(rune(s[i])) || unicode.IsDigit(rune(s[i])) || s[i] == '_' || s[i] == '$') {
				i++
			}
		}
		if i == start {
			i++
			for _, p := range []string{"...", "===", "!==", "=>", "??", "?.", "&&", "||", "==", "!="} {
				if strings.HasPrefix(s[start:], p) {
					i = start + len(p)
					break
				}
			}
		}
		i = min(i, len(s))
		out = append(out, bffToken{s[start:i], kind, start, i})
	}
	return out
}
func bffClose(t []bffToken, i int) int {
	if i >= len(t) {
		return -1
	}
	close := map[string]string{"(": ")", "[": "]", "{": "}"}[t[i].text]
	if close == "" {
		return -1
	}
	depth := 1
	for j := i + 1; j < len(t); j++ {
		if t[j].text == t[i].text {
			depth++
		}
		if t[j].text == close {
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// Nested template expressions must stay inside one token, so their commas and
// semicolons cannot terminate the outer declaration.
func bffQuotedEnd(s string, start int) int {
	quote := s[start]
	for i := start + 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == quote {
			return i + 1
		}
		if quote == '`' && s[i] == '$' && i+1 < len(s) && s[i+1] == '{' {
			depth := 1
			i += 2
			for i < len(s) && depth > 0 {
				switch s[i] {
				case '"', '\'', '`':
					i = bffQuotedEnd(s, i)
					continue
				case '{':
					depth++
				case '}':
					depth--
				}
				i++
			}
			i--
		}
	}
	return len(s) + 1
}
func bffSplit(t []bffToken, sep string) [][]bffToken {
	var out [][]bffToken
	start := 0
	for i := 0; i < len(t); i++ {
		if t[i].text == "(" || t[i].text == "[" || t[i].text == "{" {
			j := bffClose(t, i)
			if j < 0 {
				break
			}
			i = j
			continue
		}
		if t[i].text == sep {
			out = append(out, t[start:i])
			start = i + 1
		}
	}
	out = append(out, t[start:])
	return out
}
func (m *bffModule) ref(t []bffToken) location {
	if len(t) == 0 {
		return location{m.name, 1, 1}
	}
	return location{m.name, 1 + strings.Count(m.source[:t[0].start], "\n"), 1 + strings.Count(m.source[:t[len(t)-1].end], "\n")}
}
func bffUnquote(t bffToken) string {
	if len(t.text) < 2 {
		return ""
	}
	if t.kind == "regex" {
		last := strings.LastIndex(t.text, "/")
		if last < 1 {
			return ""
		}
		return strings.ReplaceAll(t.text[1:last], `\/`, `/`)
	}
	s := t.text[1 : len(t.text)-1]
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
func (r *bffReader) load(name string) (*bffModule, error) {
	name = filepath.ToSlash(filepath.Clean(name))
	if m := r.modules[name]; m != nil {
		return m, nil
	}
	data, err := os.ReadFile(filepath.Join(r.root, name))
	if err != nil {
		return nil, err
	}
	m := &bffModule{name: name, source: string(data), defs: map[string]bffExpr{}, imports: map[string][2]string{}, importRefs: map[string]location{}, funcs: map[string]bffExpr{}}
	m.tokens = bffLex(m.source)
	r.modules[name] = m
	t := m.tokens
	for i := 0; i < len(t); i++ {
		if t[i].text == "import" || t[i].text == "export" && i+1 < len(t) && t[i+1].text == "{" {
			end := i + 1
			for end < len(t) && t[end].text != ";" {
				end++
			}
			frag := t[i:end]
			from := -1
			for j := range frag {
				if frag[j].text == "from" && j+1 < len(frag) && frag[j+1].kind == "string" {
					from = j
				}
			}
			if from > 0 {
				source := bffUnquote(frag[from+1])
				file := r.importPath(name, source)
				for j := 1; j < from; j++ {
					if frag[j].kind != "ident" || frag[j].text == "type" || frag[j].text == "as" {
						continue
					}
					remote, local := frag[j].text, frag[j].text
					if j+2 < from && frag[j+1].text == "as" {
						local = frag[j+2].text
						j += 2
					}
					m.imports[local] = [2]string{file, remote}
					m.importRefs[local] = m.ref(frag)
				}
			}
			i = end
			continue
		}
		if t[i].text == "const" || t[i].text == "let" {
			if i+1 >= len(t) || t[i+1].kind != "ident" {
				continue
			}
			name := t[i+1].text
			j := i + 2
			for j < len(t) && t[j].text != "=" && t[j].text != ";" {
				j++
			}
			if j == len(t) || t[j].text != "=" {
				continue
			}
			start := j + 1
			end := start
			for end < len(t) && t[end].text != ";" {
				if t[end].text == "(" || t[end].text == "[" || t[end].text == "{" {
					k := bffClose(t, end)
					if k < 0 {
						break
					}
					end = k
				}
				end++
			}
			m.defs[name] = bffExpr{t[start:end], m.ref(t[i:end])}
		}
		if t[i].text == "function" && i+1 < len(t) {
			name := t[i+1].text
			j := i + 2
			for j < len(t) && t[j].text != "(" {
				j++
			}
			j = bffClose(t, j)
			if j < 0 {
				continue
			}
			j++ // Skip a typed return shape, then take the actual body.
			if j < len(t) && t[j].text == ":" {
				j++
				for j < len(t) && t[j].text != "{" {
					j++
				}
				if j < len(t) {
					k := bffClose(t, j)
					if k >= 0 && k+1 < len(t) && t[k+1].text == "{" {
						j = k + 1
					}
				}
			}
			for j < len(t) && t[j].text != "{" {
				j++
			}
			if k := bffClose(t, j); k >= 0 {
				m.funcs[name] = bffExpr{t[j+1 : k], m.ref(t[i : k+1])}
			}
		}
	}
	return m, nil
}
func (r *bffReader) importPath(from, source string) string {
	var p string
	if strings.HasPrefix(source, "@/") {
		parts := strings.Split(from, "/")
		p = filepath.Join(parts[0], parts[1], source[2:])
	} else if strings.HasPrefix(source, ".") {
		p = filepath.Join(filepath.Dir(from), source)
	} else {
		return source
	}
	if !strings.HasSuffix(p, ".ts") {
		if _, err := os.Stat(filepath.Join(r.root, p+".ts")); err == nil {
			p += ".ts"
		} else {
			p = filepath.Join(p, "index.ts")
		}
	}
	return filepath.ToSlash(p)
}
