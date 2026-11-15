package foundation_test

// TCV09 TestCvsNoIframeAndPrivacy (contracts/taiwan-cvs-logistics-v1.md §10 TCV09, §5.2 "no iframe, no target", §7.1 "no logging of request/response
// bodies, keys, recipient data, trade numbers or codes", PROCESS section 5, I11). Tier REVIEW + regression (source guards; no database).
// The reviewer/security_reviewer verdicts of TCV09 are not this test's. Runtime leak scans are in TCV03 (Location/body), TCV05 (operations, job args,
// events, captured process logs) and TCV06 (recipient sentinel); this file guards the source so a leak cannot be reintroduced silently.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const tcgBase = "00c1d94" // the branch base of the R2 CVS lane: "frame-src unchanged" is measured against it

func tcgRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func tcgRead(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return string(b)
}

func tcgWalk(t *testing.T, root string, dirs []string, keep func(rel string) bool) []string {
	t.Helper()
	var out []string
	for _, d := range dirs {
		_ = filepath.Walk(filepath.Join(root, d), func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			if info.IsDir() {
				if base := info.Name(); base == "node_modules" || base == ".next" || base == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if keep(rel) {
				out = append(out, rel)
			}
			return nil
		})
	}
	return out
}

// codeLines returns the non-comment lines of a Go/TS source with their 1-based numbers (a line is dropped when it is a comment line; a trailing
// comment is cut).
func codeLines(src string) map[int]string {
	out := map[int]string{}
	for i, l := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "*") || strings.HasPrefix(trim, "/*") {
			continue
		}
		if k := strings.Index(l, "//"); k >= 0 && !strings.Contains(l[:k], `"`) {
			l = l[:k]
		}
		out[i+1] = l
	}
	return out
}

// TCV09's storefront exception (taiwan-cvs-logistics-v1 §10 TCV09, storefront-v2 §G6): exactly one canonical replay detail rule may append the
// official player frame to the complete shopping policy. These literals are the only admitted exception; everything else fails the guard.
const (
	tcgReplaySource = `/:locale(zh-CN|zh-TW|en)/replays/:sessionID([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`
	tcgReplayValue  = "${shoppingCSP}; frame-src https://www.facebook.com/plugins/video.php"
)

// tcgLexToken is one token of the closed TypeScript subset these source guards read: identifiers, quoted strings, template literals whose only
// interpolation is a bare identifier, punctuation, and comments (dropped). It is deliberately not a parser: anything outside the subset is an error,
// so an unrecognised construct fails closed instead of being skipped silently.
type tcgLexToken struct {
	kind string // ident | string | template | punct
	text string // literal content for string/template, source spelling otherwise
}

func tcgIsIdent(r byte) bool {
	return r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func tcgLex(src string) ([]tcgLexToken, error) {
	var toks []tcgLexToken
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			if j := strings.IndexByte(src[i:], '\n'); j >= 0 {
				i += j + 1
			} else {
				i = len(src)
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				return nil, fmt.Errorf("unterminated block comment at byte %d", i)
			}
			i += 2 + j + 2
		case c == '"' || c == '\'':
			quote := c
			var b strings.Builder
			j := i + 1
			for ; j < len(src); j++ {
				if src[j] == '\\' && j+1 < len(src) {
					j++
					b.WriteByte(src[j])
					continue
				}
				if src[j] == quote {
					break
				}
				b.WriteByte(src[j])
			}
			if j >= len(src) {
				return nil, fmt.Errorf("unterminated string at byte %d", i)
			}
			toks = append(toks, tcgLexToken{"string", b.String()})
			i = j + 1
		case c == '`':
			var b strings.Builder
			j := i + 1
			for ; j < len(src); j++ {
				if src[j] == '\\' && j+1 < len(src) {
					j++
					b.WriteByte(src[j])
					continue
				}
				if src[j] == '`' {
					break
				}
				if src[j] == '$' && j+1 < len(src) && src[j+1] == '{' {
					k := j + 2
					for k < len(src) && tcgIsIdent(src[k]) {
						k++
					}
					if k == j+2 || k >= len(src) || src[k] != '}' {
						return nil, fmt.Errorf("unsupported template interpolation at byte %d", j)
					}
					b.WriteString(src[j : k+1])
					j = k
					continue
				}
				b.WriteByte(src[j])
			}
			if j >= len(src) {
				return nil, fmt.Errorf("unterminated template literal at byte %d", i)
			}
			toks = append(toks, tcgLexToken{"template", b.String()})
			i = j + 1
		case tcgIsIdent(c):
			j := i + 1
			for j < len(src) && tcgIsIdent(src[j]) {
				j++
			}
			toks = append(toks, tcgLexToken{"ident", src[i:j]})
			i = j
		default:
			toks = append(toks, tcgLexToken{"punct", string(c)})
			i++
		}
	}
	return toks, nil
}

// tcgCSPFrameSrc returns the frame-src directives of one CSP literal. A directive starts the value or follows ';'; prose that merely mentions
// frame-src is refused as unrecognised instead of becoming an allowance.
func tcgCSPFrameSrc(policy string) ([]string, error) {
	var out []string
	recognised := false
	for _, part := range strings.Split(policy, ";") {
		fields := strings.Fields(part)
		if len(fields) == 0 || fields[0] != "frame-src" {
			continue
		}
		recognised = true
		out = append(out, strings.Join(fields, " "))
	}
	if strings.Contains(policy, "frame-src") && !recognised {
		return nil, fmt.Errorf("frame-src text is not a CSP directive: %q", policy)
	}
	return out, nil
}

// tcgFrameSrcTokens collects frame-src directives from the string/template literals of a token stream, skipping the literal tokens in skip.
func tcgFrameSrcTokens(toks []tcgLexToken, skip map[int]bool) ([]string, error) {
	var out []string
	for i, tok := range toks {
		if skip[i] || (tok.kind != "string" && tok.kind != "template") {
			continue
		}
		dirs, err := tcgCSPFrameSrc(tok.text)
		if err != nil {
			return nil, err
		}
		out = append(out, dirs...)
	}
	return out, nil
}

func tcgFrameSrcSource(src string) ([]string, error) {
	toks, err := tcgLex(src)
	if err != nil {
		return nil, err
	}
	return tcgFrameSrcTokens(toks, nil)
}

func tcgTokensText(toks []tcgLexToken) string {
	parts := make([]string, len(toks))
	for i, tok := range toks {
		parts[i] = tok.kind + ":" + tok.text
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// tcgStorefrontFrameSrc verifies that apps/storefront/next.config.ts carries exactly one complete §G6 replay rule object (the canonical route and the
// canonical value: `${shoppingCSP}` plus the one official player frame), drops that object, and returns the frame-src directives of the remainder for
// the original base comparison. A second, widened, partial, duplicate or hand-rolled exception fails closed.
func tcgStorefrontFrameSrc(src string) ([]string, error) {
	toks, err := tcgLex(src)
	if err != nil {
		return nil, err
	}
	var sources []int
	for i, tok := range toks {
		if tok.kind == "string" && tok.text == tcgReplaySource {
			sources = append(sources, i)
		}
	}
	if len(sources) != 1 {
		return nil, fmt.Errorf("canonical replay rule source %q appears %d times, want exactly 1", tcgReplaySource, len(sources))
	}
	objOpen, depth := -1, 0
	for i := sources[0] - 1; i >= 0; i-- {
		if toks[i].kind != "punct" {
			continue
		}
		switch toks[i].text {
		case "}":
			depth++
		case "{":
			if depth == 0 {
				objOpen = i
			} else {
				depth--
			}
		}
		if objOpen >= 0 {
			break
		}
	}
	if objOpen < 0 {
		return nil, fmt.Errorf("canonical replay rule source is not inside an object")
	}
	objClose := -1
	depth = 0
	for i := objOpen; i < len(toks); i++ {
		if toks[i].kind != "punct" {
			continue
		}
		switch toks[i].text {
		case "{":
			depth++
		case "}":
			depth--
			if depth == 0 {
				objClose = i
			}
		}
		if objClose >= 0 {
			break
		}
	}
	if objClose < 0 {
		return nil, fmt.Errorf("canonical replay rule object is not closed")
	}
	want := []tcgLexToken{
		{"punct", "{"},
		{"ident", "source"}, {"punct", ":"}, {"string", tcgReplaySource}, {"punct", ","},
		{"ident", "headers"}, {"punct", ":"}, {"punct", "["},
		{"punct", "{"}, {"ident", "key"}, {"punct", ":"}, {"string", "Content-Security-Policy"}, {"punct", ","},
		{"ident", "value"}, {"punct", ":"}, {"template", tcgReplayValue}, {"punct", ","},
		{"punct", "}"}, {"punct", ","}, {"punct", "]"}, {"punct", ","},
		{"punct", "}"},
	}
	got := toks[objOpen : objClose+1]
	if len(got) != len(want) {
		return nil, fmt.Errorf("canonical replay rule object has %d tokens, want %d: %s", len(got), len(want), tcgTokensText(got))
	}
	for i := range want {
		if got[i] != want[i] {
			return nil, fmt.Errorf("canonical replay rule object token %d is %q, want %q", i, got[i].kind+":"+got[i].text, want[i].kind+":"+want[i].text)
		}
	}
	for i, tok := range toks {
		if i >= objOpen && i <= objClose {
			continue
		}
		if tok.kind == "template" && tok.text == tcgReplayValue {
			return nil, fmt.Errorf("canonical replay CSP value appears outside the verified replay rule")
		}
	}
	skip := make(map[int]bool, objClose-objOpen+1)
	for i := objOpen; i <= objClose; i++ {
		skip[i] = true
	}
	return tcgFrameSrcTokens(toks, skip)
}

func TestCvsNoIframeAndPrivacy(t *testing.T) {
	root := tcgRoot(t)

	t.Run("CSP frame-src is unchanged and no ECPay frame exists", func(t *testing.T) {
		for _, app := range []string{"admin", "storefront"} {
			rel := "apps/" + app + "/next.config.ts"
			now := tcgRead(t, root, rel)
			base, err := exec.Command("git", "-C", root, "show", tcgBase+":"+rel).Output()
			if err != nil {
				t.Errorf("NOT_RUN: cannot read %s at the base %s: %v", rel, tcgBase, err)
				continue
			}
			want, err := tcgFrameSrcSource(string(base))
			if err != nil {
				t.Errorf("NOT_RUN: %s at the base %s: %v", rel, tcgBase, err)
				continue
			}
			var got []string
			if app == "storefront" {
				// Only apps/storefront may carry the single §G6 replay exception: the verified object is dropped before the original base comparison,
				// so any other frame-src in the remainder still fails here.
				got, err = tcgStorefrontFrameSrc(now)
			} else {
				got, err = tcgFrameSrcSource(now)
			}
			if err != nil {
				t.Errorf("%s: %v", rel, err)
				continue
			}
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("%s: frame-src directives changed: %v (base %v)", rel, got, want)
			}
			for _, d := range got {
				if strings.Contains(strings.ToLower(d), "ecpay") {
					t.Errorf("%s: a frame-src allows ECPay: %s", rel, d)
				}
			}
			if !strings.Contains(now, `key: "X-Frame-Options", value: "DENY"`) {
				t.Errorf("%s: X-Frame-Options DENY was removed", rel)
			}
			// the top-level map POST needs form-action entries; they must be the two exact map endpoints, never a wildcard host
			// backtick is a delimiter too: next.config.ts builds the policy as a template literal (dev-only 'unsafe-eval')
			for _, m := range regexp.MustCompile("https?://[^\\s\"'`;]*ecpay[^\\s\"'`;]*|\\*\\.ecpay[^\\s\"'`;]*").FindAllString(now, -1) {
				if m != "https://logistics-stage.ecpay.com.tw/Express/map" && m != "https://logistics.ecpay.com.tw/Express/map" {
					t.Errorf("%s: an ECPay CSP source other than the two map endpoints: %s", rel, m)
				}
			}
		}
		if s := tcgRead(t, root, "apps/storefront/next.config.ts"); !strings.Contains(s, "https://logistics-stage.ecpay.com.tw/Express/map") || !strings.Contains(s, "https://logistics.ecpay.com.tw/Express/map") {
			t.Error("the storefront form-action must allow exactly the stage and production map endpoints (top-level form POST)")
		}
	})

	t.Run("only the ecpay package names ECPay hosts in code; no iframe, target or window.open in the map picker", func(t *testing.T) {
		goFiles := tcgWalk(t, root, []string{"internal", "cmd"}, func(rel string) bool {
			return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && !strings.Contains(rel, "internal/integrations/shipping/ecpay/")
		})
		for _, rel := range goFiles {
			for n, l := range codeLines(tcgRead(t, root, rel)) {
				if strings.Contains(l, "ecpay.com.tw") {
					t.Errorf("%s:%d dials or names an ECPay host outside the ecpay package: %q", rel, n, strings.TrimSpace(l))
				}
			}
		}
		// The apps never dial ECPay: a host string may appear only as an allowlist that validates the form action the Go API returns
		// (storefront lib/cvs-contract.ts, admin lib/logistics-model.ts); any fetch()/XMLHttpRequest naming ECPay is refused. The iframe and
		// window.open guard covers the CVS files (the wider buyer app has its own, unrelated window.open in payment code).
		tsFiles := tcgWalk(t, root, []string{"apps"}, func(rel string) bool {
			return (strings.HasSuffix(rel, ".ts") || strings.HasSuffix(rel, ".tsx")) && !strings.Contains(rel, "/tests/") && !strings.HasSuffix(rel, "next.config.ts")
		})
		for _, rel := range tsFiles {
			low := strings.ToLower(filepath.Base(rel))
			cvsFile := strings.Contains(low, "cvs") || strings.Contains(low, "ecpay")
			for n, l := range codeLines(tcgRead(t, root, rel)) {
				if strings.Contains(l, "ecpay.com.tw") && regexp.MustCompile(`fetch\(|XMLHttpRequest|axios`).MatchString(l) {
					t.Errorf("%s:%d dials an ECPay host from app code: %q", rel, n, strings.TrimSpace(l))
				}
				if cvsFile && (strings.Contains(l, "<iframe") || strings.Contains(l, "window.open")) {
					t.Errorf("%s:%d %q: no iframe / window.open in the CVS files (the admin print tab is opened by an anchor/form, not window.open)", rel, n, strings.TrimSpace(l))
				}
			}
		}
		picker := tcgRead(t, root, "apps/storefront/components/CvsPickup.tsx")
		for n, l := range codeLines(picker) {
			// the only legitimate `target` is on an official store-search anchor (checked below); the map form never carries one
			if regexp.MustCompile(`form\.target|\.target\s*=[^=]|setAttribute\(\s*["']target|target:\s*["']`).MatchString(l) {
				t.Errorf("CvsPickup.tsx:%d sets a target on the map form: %q (same tab only, F4)", n, strings.TrimSpace(l))
			}
		}
		// each anchor with target="_blank" carries rel noopener noreferrer
		for _, m := range regexp.MustCompile(`<a [^>]*target="_blank"[^>]*>`).FindAllString(picker, -1) {
			if !strings.Contains(m, `rel="noopener noreferrer"`) {
				t.Errorf("an anchor opens a new tab without rel=noopener noreferrer: %s", m)
			}
		}
		if !regexp.MustCompile(`form\.submit\(\)`).MatchString(picker) {
			t.Error("CvsPickup.tsx must submit the map form top-level (form.submit())")
		}
		if regexp.MustCompile(`form\.setAttribute\(\s*["']target`).MatchString(picker) {
			t.Error("the map form must not set a target attribute")
		}
	})

	t.Run("no key, recipient or trade-number identifier in a logging statement of the CVS code", func(t *testing.T) {
		logging := regexp.MustCompile(`slog\.|\blog\.(Print|Fatal|Panic)|fmt\.(Print|Fprint)|\.Logger\b|logger\.`)
		forbidden := regexp.MustCompile(`(?i)hashkey|hashiv|hash_key|hash_iv|MerchantTradeNo|CVSPaymentNo|CVSValidationNo|ReceiverName|ReceiverCellPhone|SenderCellPhone|recipient|\bphone\b|snapshot|AllPayLogisticsID|LogisticsID`)
		files := tcgWalk(t, root, []string{"internal/integrations/shipping/ecpay", "internal/fulfillment", "internal/checkout", "internal/buyerhttp", "internal/httpapi"}, func(rel string) bool {
			base := filepath.Base(rel)
			return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && !strings.Contains(rel, "/ecpaytest/") &&
				(strings.HasPrefix(base, "cvs") || strings.Contains(rel, "internal/integrations/shipping/ecpay/"))
		})
		if len(files) < 6 {
			t.Fatalf("CVS source set suspiciously small: %v", files)
		}
		for _, rel := range files {
			for n, l := range codeLines(tcgRead(t, root, rel)) {
				if logging.MatchString(l) && forbidden.MatchString(l) {
					t.Errorf("%s:%d logs a forbidden identifier: %q", rel, n, strings.TrimSpace(l))
				}
			}
		}
	})

	t.Run("PROCESS section 5: package comments and app file headers", func(t *testing.T) {
		for _, dir := range []string{"internal/integrations/shipping/ecpay", "internal/integrations/shipping/ecpay/ecpayroute", "internal/integrations/shipping/ecpay/ecpaytest"} {
			found := false
			entries, _ := os.ReadDir(filepath.Join(root, dir))
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
					if regexp.MustCompile(`(?m)^// Package \w+ (owns|is)`).MatchString(tcgRead(t, root, dir+"/"+e.Name())) {
						found = true
					}
				}
			}
			if !found {
				t.Errorf("%s has no `// Package x owns ...` comment", dir)
			}
		}
		// every CVS-named app file starts with a comment naming its BFF route or Go endpoint
		apps := tcgWalk(t, root, []string{"apps"}, func(rel string) bool {
			low := strings.ToLower(filepath.Base(rel))
			// route and component files (PROCESS section 5 names "route/component"); pure data/copy modules are exempt
			return (strings.HasSuffix(rel, ".tsx") || strings.HasSuffix(rel, "route.ts")) && !strings.Contains(rel, "/tests/") && (strings.Contains(low, "cvs") || strings.Contains(low, "ecpay") || strings.Contains(rel, "/cvs"))
		})
		if len(apps) == 0 {
			t.Error("no CVS app files found")
		}
		for _, rel := range apps {
			src := tcgRead(t, root, rel)
			head := src
			if len(head) > 1500 {
				head = head[:1500]
			}
			body := strings.TrimSpace(src)
			for strings.HasPrefix(body, `"use client"`) || strings.HasPrefix(body, `'use client'`) { // a directive must stay first
				body = strings.TrimSpace(body[strings.Index(body, "\n")+1:])
			}
			head = body
			if len(head) > 1500 {
				head = head[:1500]
			}
			first := strings.TrimSpace(strings.SplitN(body, "\n", 2)[0])
			if !(strings.HasPrefix(first, "//") || strings.HasPrefix(first, "/*")) {
				t.Errorf("%s does not start with a comment (PROCESS section 5, Frontend)", rel)
				continue
			}
			if !strings.Contains(head, "/api/") && !strings.Contains(head, "/v1/") {
				t.Errorf("%s: the header comment names neither a BFF route (/api/...) nor a Go endpoint (/v1/...)", rel)
			}
		}
		// the Go CVS files start with a comment as well
		for _, rel := range tcgWalk(t, root, []string{"internal/fulfillment", "internal/checkout", "internal/buyerhttp", "internal/httpapi"}, func(rel string) bool {
			base := filepath.Base(rel)
			return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && strings.HasPrefix(base, "cvs")
		}) {
			if first := strings.TrimSpace(strings.SplitN(strings.TrimSpace(tcgRead(t, root, rel)), "\n", 2)[0]); !strings.HasPrefix(first, "//") {
				t.Errorf("%s does not start with a comment", rel)
			}
		}
	})

	t.Run("TCV09 scoped replay exception calibration", func(t *testing.T) {
		const rel = "apps/storefront/next.config.ts"
		now := tcgRead(t, root, rel)
		base, err := exec.Command("git", "-C", root, "show", tcgBase+":"+rel).Output()
		if err != nil {
			t.Fatalf("NOT_RUN: cannot read %s at the base %s: %v", rel, tcgBase, err)
		}
		baseDirs, err := tcgFrameSrcSource(string(base))
		if err != nil {
			t.Fatalf("NOT_RUN: %s at the base %s: %v", rel, tcgBase, err)
		}
		guard := func(src string) error {
			got, err := tcgStorefrontFrameSrc(src)
			if err != nil {
				return err
			}
			if strings.Join(got, "|") != strings.Join(baseDirs, "|") {
				return fmt.Errorf("remainder frame-src %v differs from the base %v", got, baseDirs)
			}
			for _, d := range got {
				if strings.Contains(strings.ToLower(d), "ecpay") {
					return fmt.Errorf("remainder frame-src allows ECPay: %s", d)
				}
			}
			return nil
		}
		if err := guard(now); err != nil {
			t.Fatalf("the current %s is not accepted: %v", rel, err)
		}
		// extraRule builds a complete extra header rule object; it is inserted before the (unique) claim rule comment to prove that a second frame
		// allowance anywhere in the config is refused.
		extraRule := func(source, value string) string {
			return fmt.Sprintf("      {\n        source: %q,\n        headers: [\n          {\n            key: \"Content-Security-Policy\",\n            value: %q,\n          },\n        ],\n      },\n", source, value)
		}
		claimAt := strings.Index(now, "// The claim page and the manual-order link page")
		if claimAt < 0 {
			t.Fatalf("cannot locate the claim rule in %s", rel)
		}
		openAt := strings.LastIndex(now[:claimAt], "{")
		if openAt < 0 {
			t.Fatalf("cannot locate the claim rule object in %s", rel)
		}
		insert := func(rule string) string { return now[:openAt] + rule + now[openAt:] }
		cases := []struct {
			name  string
			src   string
			admit bool
		}{
			{"comment mentioning frame-src is not a directive", now[:openAt] + "        // frame-src https://www.facebook.com/plugins/video.php only in a comment\n" + now[openAt:], true},
			{"ECPay frame", insert(extraRule("/:path*", "default-src 'self'; frame-src https://logistics.ecpay.com.tw/Express/map")), false},
			{"global official frame allowance", insert(extraRule("/:path*", "default-src 'self'; frame-src https://www.facebook.com/plugins/video.php")), false},
			{"expanded replay route", strings.Replace(now, tcgReplaySource, "/:locale(zh-CN|zh-TW|en)/replays/:path*", 1), false},
			{"other frame host", strings.Replace(now, tcgReplayValue, "${shoppingCSP}; frame-src https://www.facebook.com.evil.example/plugins/video.php", 1), false},
			{"other player path", strings.Replace(now, tcgReplayValue, "${shoppingCSP}; frame-src https://www.facebook.com/plugins/video2.php", 1), false},
			{"duplicate replay exception", insert(extraRule(tcgReplaySource, tcgReplayValue)), false},
			{"missing base interpolation", strings.Replace(now, tcgReplayValue, "frame-src https://www.facebook.com/plugins/video.php", 1), false},
		}
		for _, c := range cases {
			if c.src == now {
				t.Errorf("%s: calibration mutation was not applied", c.name)
				continue
			}
			err := guard(c.src)
			if c.admit && err != nil {
				t.Errorf("%s: rejected an admitted source: %v", c.name, err)
			}
			if !c.admit && err == nil {
				t.Errorf("%s: accepted a source that must be rejected", c.name)
			}
		}
	})
}
