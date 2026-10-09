// Purpose: expand finite anchored BFF grammars after normalizing known parameter languages.
// Depends on: Go regexp/syntax and finite product helper; used by the source extractor.
// Invariant: unbounded character classes, unsupported regex flags and unanchored grammars stay unresolved.
package main

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
)

var bffUUID = regexp.MustCompile(`\[0-9a-f\]\{8\}(?:-\[0-9a-f\]\{4\}-\[0-9a-f\]\{4\}-\[0-9a-f\]\{4\}|\(\?:-\[0-9a-f\]\{4\}\)\{3\})-\[0-9a-f\]\{12\}`)

func bffExpand(pattern string) ([]string, error) {
	if !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
		return nil, fmt.Errorf("admission regex is not anchored")
	}
	pattern = strings.ReplaceAll(pattern, `\/`, `/`)
	pattern = bffUUID.ReplaceAllString(pattern, "{}")
	for _, p := range []string{`[A-Z]{2}`, `[a-z][a-z0-9_-]{0,39}`, `[0-9_]{1,80}`, `[^/]+`} {
		pattern = strings.ReplaceAll(pattern, p, "{}")
	}
	re, e := syntax.Parse(pattern, syntax.Perl)
	if e != nil {
		return nil, e
	}
	return bffExpandRE(re)
}
func bffExpandRE(re *syntax.Regexp) ([]string, error) {
	switch re.Op {
	case syntax.OpEmptyMatch, syntax.OpBeginText, syntax.OpEndText, syntax.OpBeginLine, syntax.OpEndLine:
		return []string{""}, nil
	case syntax.OpLiteral:
		return []string{string(re.Rune)}, nil
	case syntax.OpCapture:
		return bffExpandRE(re.Sub[0])
	case syntax.OpConcat:
		out := []string{""}
		for _, s := range re.Sub {
			x, e := bffExpandRE(s)
			if e != nil {
				return nil, e
			}
			out, e = bffProduct(out, x)
			if e != nil {
				return nil, e
			}
		}
		return out, nil
	case syntax.OpAlternate:
		var out []string
		for _, s := range re.Sub {
			x, e := bffExpandRE(s)
			if e != nil {
				return nil, e
			}
			out = append(out, x...)
		}
		if len(out) > 4096 {
			return nil, fmt.Errorf("alternation expansion cap")
		}
		return out, nil
	case syntax.OpQuest:
		x, e := bffExpandRE(re.Sub[0])
		return append(x, ""), e
	default:
		return nil, fmt.Errorf("unbounded/unknown parameter grammar %s", re.Op)
	}
}
func bffHTTP(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}
func bffLiteral(t bffToken) string {
	if t.kind == "string" {
		return bffUnquote(t)
	}
	return t.text
}
func bffText(t []bffToken) string {
	var b strings.Builder
	for _, x := range t {
		b.WriteString(x.text)
		b.WriteByte(' ')
	}
	return b.String()
}
func bffInt(t bffToken) int { n, _ := strconv.Atoi(t.text); return n }
