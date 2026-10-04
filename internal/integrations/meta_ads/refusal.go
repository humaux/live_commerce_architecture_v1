package metaads

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// GraphRefusal is route-private, comparable data. Only Meta's public-facing
// error_user_msg is admitted, never error.message, debug fields or the raw body.
type GraphRefusal struct{ UserMessage string }

// PlainUserMessage strips markup and non-printing controls and caps Unicode
// characters, not UTF-8 bytes. It does not translate or replace Meta's wording.
// The UI must still render this as a text node, never HTML.
func PlainUserMessage(raw string) string {
	z := html.NewTokenizer(strings.NewReader(raw))
	z.SetMaxBuf(64 << 10)
	text := make([]rune, 0, 300)
	hidden := ""
	for {
		switch z.Next() {
		case html.ErrorToken:
			return strings.TrimSpace(string(text))
		case html.StartTagToken:
			name, _ := z.TagName()
			if string(name) == "script" || string(name) == "style" {
				hidden = string(name)
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if string(name) == hidden {
				hidden = ""
			}
		case html.TextToken:
			if hidden != "" {
				continue
			}
			for _, r := range string(z.Text()) {
				if unicode.IsControl(r) && r != '\n' && r != '\t' {
					continue
				}
				text = append(text, r)
				if len(text) == 300 {
					return strings.TrimSpace(string(text))
				}
			}
		}
	}
}
