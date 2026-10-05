// Purpose: text rules of the manual sends (live-console-v1 §3.3-3.5): NFC + control-character + per-platform length validation, the
// §3.5 public-reply content rule (shared validator in msgtemplates), template resolution to plain text, and the display-copy link
// scrub (§3.4: no bearer link persists in a display copy).
// Depends on: internal/msgtemplates (Resolve, ValidatePublicSafe), golang.org/x/text/unicode/norm.
// Used by: internal/inbox/send.go (planners), internal/httpapi/inbox_send.go (maps SendError to the transport codes).
// Invariants: LCN08 (each §3.5 pattern rejected server-side), LCN13 (display copy has no link, typed, pasted or full-width).

package inbox

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	"livecommerce/internal/msgtemplates"
)

// Send kinds (inbox.outbound_messages.kind; also the msgtemplates kinds).
const (
	KindDM          = "dm"
	KindPrivate     = "private_reply"
	KindPublic      = "public_reply"
	KindRecommend   = "recommend"
	maxMessenger    = 2000 // runes
	maxInstagram    = 1000 // bytes (UTF-8)
	maxPublicRunes  = 300
	linkPlaceholder = "{{連結}}"
)

// SendError is a refusal with a fixed transport status and code (never a driver message). Max is set for invalid_text (the limit that
// was exceeded), Reason for public_reply_forbidden_content.
type SendError struct {
	Status int
	Code   string
	Max    int
	Reason string
}

func (e *SendError) Error() string { return e.Code }

func invalidText(max int, reason string) error {
	return &SendError{Status: 422, Code: "invalid_text", Max: max, Reason: reason}
}

// TextInput is the body of A4/A5/A12: either a literal text or a published template reference.
type TextInput struct {
	Text            string `json:"text"`
	TemplateID      string `json:"template_id"`
	TemplateVersion int64  `json:"template_version"`
}

// resolvedText is the validated plain text of one send and the template that produced it (nil for literal text).
type resolvedText struct {
	text       string
	templateID *string
	version    *int64
}

// resolveText turns a TextInput into validated text for kind on platform (instagram vs everything else).
// Calls msgtemplates.Resolve (migration 0121) for a template reference; no network call.
func (s *Service) resolveText(ctx context.Context, tx pgx.Tx, kind, platform string, in TextInput) (resolvedText, error) {
	var out resolvedText
	hasText, hasTemplate := in.Text != "", in.TemplateID != ""
	if hasText == hasTemplate || (hasTemplate && in.TemplateVersion < 1) || (hasText && in.TemplateVersion != 0) {
		return out, invalidText(0, "text_or_template")
	}
	text := in.Text
	if hasTemplate {
		if s.templates == nil {
			return out, invalidText(0, "template_unavailable")
		}
		r, err := s.templates.Resolve(ctx, tx, in.TemplateID, in.TemplateVersion)
		if err != nil {
			if msgtemplates.IsNotFound(err) {
				return out, invalidText(0, "template_not_found")
			}
			return out, err
		}
		if !slices.Contains(r.Kinds, kind) || ((kind == KindPublic || kind == KindRecommend) && !r.PublicSafe) {
			return out, invalidText(0, "template_kind")
		}
		// Variable substitution beyond the fixed recommend template belongs to the order-for-buyer unit (LC-B6).
		if strings.Contains(r.Body, "{{") {
			return out, invalidText(0, "template_variables")
		}
		text = r.Body
		out.templateID, out.version = &r.TemplateID, &r.Version
	}
	checked, err := checkText(kind, platform, text)
	if err != nil {
		return resolvedText{}, err
	}
	out.text = checked
	return out, nil
}

// checkText applies NFC, the control-character rule and the per-kind limit; public kinds also run the §3.5 content rule.
func checkText(kind, platform, raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", invalidText(0, "encoding")
	}
	text := norm.NFC.String(raw)
	if strings.TrimSpace(text) == "" {
		return "", invalidText(0, "empty")
	}
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' {
			return "", invalidText(0, "control_character")
		}
	}
	switch {
	case kind == KindPublic || kind == KindRecommend:
		if utf8.RuneCountInString(text) > maxPublicRunes {
			return "", invalidText(maxPublicRunes, "too_long")
		}
		if reason := msgtemplates.ValidatePublicSafe(text, ""); reason != "" {
			return "", &SendError{Status: 422, Code: "public_reply_forbidden_content", Reason: reason}
		}
	case platform == "instagram":
		if len(text) > maxInstagram {
			return "", invalidText(maxInstagram, "too_long")
		}
	default:
		if utf8.RuneCountInString(text) > maxMessenger {
			return "", invalidText(maxMessenger, "too_long")
		}
	}
	return text, nil
}

var linkDomain = regexp.MustCompile(`[a-z0-9-]+\.(com|tw|hk|cn|net|org|shop|store|me|io|app|link|ly)`)

// linkRune reports whether r folds (NFKC) to one printable ASCII byte, so full-width forms of URL characters join a candidate run.
func linkRune(r rune) bool {
	n := norm.NFKC.String(string(r))
	return len(n) == 1 && n[0] > 0x20 && n[0] < 0x7f
}

// scrubLinks replaces every URL or bare-domain run with {{連結}} before the display copy is sealed (§3.4). A run is a maximal sequence
// of URL-capable characters (ASCII or their full-width forms); a CJK character or a space ends it, so surrounding prose survives.
// ponytail: the TLD list is the §3.5 one; an unlisted TLD without a scheme or "www." is kept in the display copy (the dispatch copy and
// the Graph request are unaffected). Add TLDs here when §3.5 grows.
func scrubLinks(text string) string {
	var out strings.Builder
	var run []rune
	flush := func() {
		if len(run) == 0 {
			return
		}
		raw := string(run)
		folded := strings.ToLower(norm.NFKC.String(raw))
		if strings.Contains(folded, "://") || strings.HasPrefix(folded, "www.") || strings.Contains(folded, "www.") || linkDomain.MatchString(folded) {
			out.WriteString(linkPlaceholder)
		} else {
			out.WriteString(raw)
		}
		run = run[:0]
	}
	for _, r := range text {
		if linkRune(r) {
			run = append(run, r)
			continue
		}
		flush()
		out.WriteRune(r)
	}
	flush()
	return out.String()
}

// ErrorDetails is the bounded details object of the transport envelope: the exceeded limit and/or a fixed reason code.
func (e *SendError) ErrorDetails() map[string]any {
	d := map[string]any{}
	if e.Max > 0 {
		d["max"] = e.Max
	}
	if e.Reason != "" {
		d["reason"] = e.Reason
	}
	return d
}
