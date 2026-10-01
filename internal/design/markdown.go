package design

// markdown.go validates the restricted markdown of body fields (contracts/storefront-v2.md section B): paragraphs,
// **bold**, *italic*, "- " lists and [text](https://...) links, nothing else. Validation (not rendering) is what this
// package owns: the stored text is rendered by packages/markdown-lite, which escapes first and applies the same link
// rule, so the two stay consistent by construction. Raw HTML is refused outright ("<" anywhere), as are images,
// non-https link targets (javascript:, data:, http:, relative) and C0 control characters.

import (
	"regexp"
	"strings"
)

// linkPattern is the one link construct. The text may not contain brackets or newlines; the target runs to the first ")".
// packages/markdown-lite/src/index.ts uses the identical pattern.
var linkPattern = regexp.MustCompile(`\[[^\[\]\n]*\]\(([^)\n]*)\)`)

// linkTarget is an absolute https URL with no whitespace, quotes, angle brackets or parentheses.
var linkTarget = regexp.MustCompile(`^https://[^\s<>"'()]{1,500}$`)

// markdownProblem returns "" when s is acceptable restricted markdown, else a short reason (no user text echoed).
func markdownProblem(s string) string {
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\r' && r != '\t') || r == 0x7f {
			return "control characters are not allowed"
		}
	}
	if strings.ContainsRune(s, '<') {
		return "raw HTML is not allowed"
	}
	if strings.Contains(s, "![") {
		return "images are not allowed in text"
	}
	for _, m := range linkPattern.FindAllStringSubmatch(s, -1) {
		if !linkTarget.MatchString(m[1]) {
			return "links must be absolute https URLs"
		}
	}
	if strings.Contains(linkPattern.ReplaceAllString(s, ""), "](") {
		return "malformed link"
	}
	return ""
}
