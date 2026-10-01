package design

// schema.go is the trust boundary of the design document: Normalize turns whatever JSON the merchant client sent into
// the canonical document of contracts/storefront-v2.md section B, or refuses it with the path of the first offender
// (ValidationError -> HTTP 422 details.path). The schema is closed: an unknown key anywhere is an error, strings are
// length-capped in characters (merchant text is Chinese), URLs are https (line_url may also be line://), image ids are
// canonical UUIDs (their existence in design.store_media is checked by the caller, which owns the transaction), and
// markdown bodies pass markdownProblem. Nothing here touches the database.
//
// Leniency, deliberate: the containers nav, nav.header, nav.footer, home, home.sections, pages and profile.contact may
// be absent (normalised to empty/null); every leaf the contract does not mark "|null" is required. A nullable string
// that arrives as "" or whitespace is stored as null, so a cleared form field means "unset".

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"livecommerce/internal/command"
)

// Contract limits (storefront-v2 section B).
const (
	MaxDocumentBytes = 256 << 10
	maxHeaderNav     = 8
	maxFooterNav     = 12
	maxSections      = 20
	maxPages         = 20
)

var (
	slugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	phonePattern = regexp.MustCompile(`^[0-9+()#*. -]{3,30}$`)
	// entityPattern is any HTML character reference (&lt; &#60; &#x3c; ...): a plain-text field never carries markup, escaped or not.
	entityPattern = regexp.MustCompile(`&[a-zA-Z#][^;\s]{0,10};`)

	headerKinds = []string{"home", "all_products", "collection", "page", "url"}
	footerKinds = []string{"page", "url", "collection"}
	ctaKinds    = []string{"all_products", "collection", "page"}
	sortModes   = []string{"newest", "price_asc", "price_desc"}
	imageSides  = []string{"left", "right"}

	sectionKeys = map[string][]string{
		"hero":                {"type", "image_id", "heading", "subheading", "cta_label", "cta_kind", "cta_target"},
		"featured_collection": {"type", "collection_slug", "heading", "limit"},
		"product_grid":        {"type", "heading", "sort", "limit"},
		"rich_text":           {"type", "heading", "body"},
		"image_text":          {"type", "image_id", "heading", "body", "image_side"},
	}
)

// ValidationError names the first offending path (e.g. "home.sections[2].heading") and a short reason that never
// echoes the submitted value. It satisfies errors.Is(err, command.ErrInvalid) so generic classifiers still say 422.
type ValidationError struct{ Path, Reason string }

func (e *ValidationError) Error() string {
	return "design document invalid at " + e.Path + ": " + e.Reason
}

// Is makes a ValidationError an invalid-request error for the shared HTTP classifier.
func (e *ValidationError) Is(target error) bool { return target == command.ErrInvalid }

// Refs maps every referenced store-media id to the path of its first use (for the "image not found" error path).
type Refs map[string]string

type pageRef struct{ path, slug string }

type walker struct {
	err      *ValidationError
	refs     Refs
	pageRefs []pageRef
}

func (w *walker) bad(path, reason string) {
	if w.err == nil {
		w.err = &ValidationError{Path: path, Reason: reason}
	}
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// Normalize validates raw (one JSON object) and returns its canonical encoding plus the image ids it references.
func Normalize(raw []byte) ([]byte, Refs, error) {
	if len(raw) == 0 || len(raw) > MaxDocumentBytes {
		return nil, nil, &ValidationError{Path: "", Reason: "document is empty or too large"}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, nil, &ValidationError{Path: "", Reason: "not valid JSON"}
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, nil, &ValidationError{Path: "", Reason: "trailing data after the document"}
	}
	w := &walker{refs: Refs{}}
	out := w.document(doc)
	if w.err != nil {
		return nil, nil, w.err
	}
	encoded, err := json.Marshal(out)
	if err != nil || len(encoded) > MaxDocumentBytes {
		return nil, nil, &ValidationError{Path: "", Reason: "document is too large"}
	}
	return encoded, w.refs, nil
}

func (w *walker) document(x any) map[string]any {
	root := w.object(x, "", "profile", "nav", "home", "pages")
	if root == nil {
		return nil
	}
	out := map[string]any{}
	if _, ok := root["profile"]; !ok {
		w.bad("profile", "is required")
	}
	out["profile"] = w.profile(root["profile"])
	out["nav"] = w.nav(root["nav"])
	out["home"] = w.home(root["home"])
	pages, slugs := w.pages(root["pages"])
	out["pages"] = pages
	for _, ref := range w.pageRefs { // page links must point at a page of this same document
		if !slugs[ref.slug] {
			w.bad(ref.path, "no page with this slug")
		}
	}
	return out
}

// object type-checks x and refuses any key outside keys. A missing or null x is treated as absent (nil, no error).
func (w *walker) object(x any, path string, keys ...string) map[string]any {
	if x == nil {
		return nil
	}
	m, ok := x.(map[string]any)
	if !ok {
		w.bad(path, "must be an object")
		return nil
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		known := false
		for _, allowed := range keys {
			known = known || k == allowed
		}
		if !known {
			w.bad(join(path, k), "unknown key")
		}
	}
	return m
}

func (w *walker) array(x any, path string, max int) []any {
	if x == nil {
		return nil
	}
	a, ok := x.([]any)
	if !ok {
		w.bad(path, "must be an array")
		return nil
	}
	if len(a) > max {
		w.bad(path, fmt.Sprintf("too many items (max %d)", max))
		return nil
	}
	return a
}

// text reads one single-line string. Absent, null, "" and whitespace are "unset": nil, or an error when required.
func (w *walker) text(m map[string]any, path, key string, max int, required bool) any {
	p := join(path, key)
	s, present := w.rawString(m, p, key)
	if !present || strings.TrimSpace(s) == "" {
		if required {
			w.bad(p, "is required")
		}
		return nil
	}
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		w.bad(p, fmt.Sprintf("too long (max %d characters)", max))
		return nil
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			w.bad(p, "must be a single line without control characters")
			return nil
		}
	}
	if reason := plainTextProblem(s); reason != "" {
		w.bad(p, reason)
		return nil
	}
	return s
}

// plainTextProblem is THE rule for every plain-text field (name, tagline, announcement, labels, headings, titles, contact
// text): no angle brackets and no HTML entities, so neither raw nor pre-escaped markup is ever stored (contracts/storefront-v2.md
// section B: "no HTML anywhere"). Markdown bodies have their own grammar (markdownProblem); URLs, slugs and enums are
// validated by their own stricter checks.
func plainTextProblem(s string) string {
	if strings.ContainsAny(s, "<>") || entityPattern.MatchString(s) {
		return "HTML is not allowed"
	}
	return ""
}

func (w *walker) rawString(m map[string]any, p, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	s, isString := v.(string)
	if !isString {
		w.bad(p, "must be a string")
		return "", false
	}
	return s, true
}

// markdown reads a restricted-markdown body: the key is required, the text may be empty.
func (w *walker) markdown(m map[string]any, path, key string, max int) string {
	p := join(path, key)
	s, present := w.rawString(m, p, key)
	if !present {
		w.bad(p, "is required")
		return ""
	}
	if utf8.RuneCountInString(s) > max {
		w.bad(p, fmt.Sprintf("too long (max %d characters)", max))
		return ""
	}
	if reason := markdownProblem(s); reason != "" {
		w.bad(p, reason)
	}
	return s
}

func (w *walker) enum(m map[string]any, path, key string, allowed []string, required bool) any {
	p := join(path, key)
	s, present := w.rawString(m, p, key)
	if !present || s == "" { // a cleared <select> sends "": unset
		if required {
			w.bad(p, "is required")
		}
		return nil
	}
	for _, a := range allowed {
		if s == a {
			return s
		}
	}
	w.bad(p, "not one of: "+strings.Join(allowed, ", "))
	return nil
}

func (w *walker) integer(m map[string]any, path, key string, lo, hi int64) any {
	p := join(path, key)
	v, present := m[key]
	if !present || v == nil {
		w.bad(p, "is required")
		return nil
	}
	n, ok := v.(json.Number)
	var i int64
	if ok {
		var err error
		i, err = n.Int64()
		ok = err == nil
	}
	if !ok || i < lo || i > hi {
		w.bad(p, fmt.Sprintf("must be an integer between %d and %d", lo, hi))
		return nil
	}
	return i
}

func (w *walker) slug(m map[string]any, path, key string, required bool) string {
	p := join(path, key)
	s, present := w.rawString(m, p, key)
	if !present || s == "" {
		if required {
			w.bad(p, "is required")
		}
		return ""
	}
	if len(s) > 80 || !slugPattern.MatchString(s) {
		w.bad(p, "must be a lowercase slug (a-z, 0-9, hyphens, max 80)")
		return ""
	}
	return s
}

func (w *walker) imageID(m map[string]any, path, key string, required bool) any {
	p := join(path, key)
	s, present := w.rawString(m, p, key)
	if !present || s == "" {
		if required {
			w.bad(p, "is required")
		}
		return nil
	}
	if !command.ValidID(s) {
		w.bad(p, "must be a store-media image id")
		return nil
	}
	if _, seen := w.refs[s]; !seen {
		w.refs[s] = p
	}
	return s
}

// httpsURL accepts an absolute https URL (or, when line is true, a line:// deep link) with a host and no credentials.
func httpsURL(s string, line bool) bool {
	if len(s) > 300 || strings.ContainsAny(s, " \t\r\n<>\"'") {
		return false
	}
	if line && strings.HasPrefix(s, "line://") && len(s) > len("line://") {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

func (w *walker) link(m map[string]any, path, key string, line bool) any {
	p := join(path, key)
	s, present := w.rawString(m, p, key)
	if !present || strings.TrimSpace(s) == "" {
		return nil
	}
	s = strings.TrimSpace(s)
	if !httpsURL(s, line) {
		w.bad(p, "must be an absolute https URL")
		return nil
	}
	return s
}

func (w *walker) profile(x any) map[string]any {
	const path = "profile"
	m := w.object(x, path, "name", "tagline", "logo_image_id", "favicon_image_id", "accent_color", "announcement", "contact")
	if m == nil {
		m = map[string]any{}
	}
	out := map[string]any{
		"name":             w.text(m, path, "name", 60, true),
		"tagline":          w.text(m, path, "tagline", 120, false),
		"logo_image_id":    w.imageID(m, path, "logo_image_id", false),
		"favicon_image_id": w.imageID(m, path, "favicon_image_id", false),
		"announcement":     w.text(m, path, "announcement", 140, false),
	}
	color, present := w.rawString(m, "profile.accent_color", "accent_color")
	switch {
	case !present:
		w.bad("profile.accent_color", "is required")
	case !colorPattern.MatchString(color):
		w.bad("profile.accent_color", "must be #RRGGBB")
	default:
		out["accent_color"] = strings.ToLower(color)
	}
	cm := w.object(m["contact"], "profile.contact", "email", "phone", "address", "line_url", "facebook_url", "instagram_url")
	if cm == nil {
		cm = map[string]any{}
	}
	contact := map[string]any{
		"email":         w.text(cm, "profile.contact", "email", 120, false),
		"phone":         w.text(cm, "profile.contact", "phone", 30, false),
		"address":       w.text(cm, "profile.contact", "address", 200, false),
		"line_url":      w.link(cm, "profile.contact", "line_url", true),
		"facebook_url":  w.link(cm, "profile.contact", "facebook_url", false),
		"instagram_url": w.link(cm, "profile.contact", "instagram_url", false),
	}
	if email, ok := contact["email"].(string); ok {
		if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
			w.bad("profile.contact.email", "must be an email address")
		}
	}
	if phone, ok := contact["phone"].(string); ok && !phonePattern.MatchString(phone) {
		w.bad("profile.contact.phone", "may contain digits, spaces and + ( ) - . # * only")
	}
	out["contact"] = contact
	return out
}

func (w *walker) nav(x any) map[string]any {
	m := w.object(x, "nav", "header", "footer")
	if m == nil {
		m = map[string]any{}
	}
	return map[string]any{
		"header": w.navItems(m["header"], "nav.header", maxHeaderNav, headerKinds),
		"footer": w.navItems(m["footer"], "nav.footer", maxFooterNav, footerKinds),
	}
}

func (w *walker) navItems(x any, path string, max int, kinds []string) []any {
	out := []any{}
	for i, item := range w.array(x, path, max) {
		p := fmt.Sprintf("%s[%d]", path, i)
		m := w.object(item, p, "label", "kind", "target")
		if m == nil {
			w.bad(p, "must be an object")
			continue
		}
		label := w.text(m, p, "label", 30, true)
		kind := w.enum(m, p, "kind", kinds, true)
		var target any
		switch kind {
		case "collection":
			target = w.targetSlug(m, p, false)
		case "page":
			target = w.targetSlug(m, p, true)
		case "url":
			target = w.link(m, p, "target", false)
			if target == nil {
				w.bad(join(p, "target"), "must be an absolute https URL")
			}
		case "home", "all_products":
			if v, present := m["target"]; present && v != nil {
				w.bad(join(p, "target"), "must be null for this kind")
			}
		}
		out = append(out, map[string]any{"label": label, "kind": kind, "target": target})
	}
	return out
}

// targetSlug reads a required slug target; page targets are cross-checked against the document's pages afterwards.
func (w *walker) targetSlug(m map[string]any, path string, isPage bool) any {
	s := w.slug(m, path, "target", true)
	if s == "" {
		return nil
	}
	if isPage {
		w.pageRefs = append(w.pageRefs, pageRef{join(path, "target"), s})
	}
	return s
}

func (w *walker) home(x any) map[string]any {
	m := w.object(x, "home", "sections")
	if m == nil {
		m = map[string]any{}
	}
	sections := []any{}
	for i, item := range w.array(m["sections"], "home.sections", maxSections) {
		p := fmt.Sprintf("home.sections[%d]", i)
		raw, ok := item.(map[string]any)
		if !ok {
			w.bad(p, "must be an object")
			continue
		}
		kind, _ := raw["type"].(string)
		keys, known := sectionKeys[kind]
		if !known {
			w.bad(join(p, "type"), "not one of: hero, featured_collection, product_grid, rich_text, image_text")
			continue
		}
		sm := w.object(raw, p, keys...)
		sections = append(sections, w.section(kind, sm, p))
	}
	return map[string]any{"sections": sections}
}

func (w *walker) section(kind string, m map[string]any, p string) map[string]any {
	out := map[string]any{"type": kind}
	out["heading"] = w.text(m, p, "heading", 80, false)
	switch kind {
	case "hero":
		out["image_id"] = w.imageID(m, p, "image_id", true)
		out["subheading"] = w.text(m, p, "subheading", 160, false)
		out["cta_label"] = w.text(m, p, "cta_label", 24, false)
		ctaKind := w.enum(m, p, "cta_kind", ctaKinds, false)
		out["cta_kind"] = ctaKind
		var target any
		switch ctaKind {
		case "collection":
			target = w.targetSlugKey(m, p, "cta_target", false)
		case "page":
			target = w.targetSlugKey(m, p, "cta_target", true)
		default:
			if v, present := m["cta_target"]; present && v != nil {
				w.bad(join(p, "cta_target"), "must be null for this cta_kind")
			}
		}
		out["cta_target"] = target
	case "featured_collection":
		out["collection_slug"] = w.slug(m, p, "collection_slug", true)
		out["limit"] = w.integer(m, p, "limit", 4, 24)
	case "product_grid":
		out["sort"] = w.enum(m, p, "sort", sortModes, true)
		out["limit"] = w.integer(m, p, "limit", 4, 48)
	case "rich_text":
		out["body"] = w.markdown(m, p, "body", 4000)
	case "image_text":
		out["image_id"] = w.imageID(m, p, "image_id", true)
		out["body"] = w.markdown(m, p, "body", 2000)
		out["image_side"] = w.enum(m, p, "image_side", imageSides, true)
	}
	return out
}

func (w *walker) targetSlugKey(m map[string]any, path, key string, isPage bool) any {
	s := w.slug(m, path, key, true)
	if s == "" {
		return nil
	}
	if isPage {
		w.pageRefs = append(w.pageRefs, pageRef{join(path, key), s})
	}
	return s
}

func (w *walker) pages(x any) ([]any, map[string]bool) {
	out, slugs := []any{}, map[string]bool{}
	for i, item := range w.array(x, "pages", maxPages) {
		p := fmt.Sprintf("pages[%d]", i)
		m := w.object(item, p, "slug", "title", "body")
		if m == nil {
			w.bad(p, "must be an object")
			continue
		}
		slug := w.slug(m, p, "slug", true)
		if slug != "" {
			if slugs[slug] {
				w.bad(join(p, "slug"), "duplicate page slug")
			}
			slugs[slug] = true
		}
		out = append(out, map[string]any{
			"slug": slug, "title": w.text(m, p, "title", 80, true), "body": w.markdown(m, p, "body", 20000),
		})
	}
	return out, slugs
}

// DefaultDocument is what a store without any draft or published version shows: the store name, the accent of the
// admin palette and one product grid (contracts/storefront-v2.md section B, last paragraph). It always validates.
func DefaultDocument(storeName string) []byte {
	// The store name comes from control.stores (free text of the signup flow): strip what plainTextProblem would refuse.
	name := strings.TrimSpace(entityPattern.ReplaceAllString(strings.NewReplacer("<", "", ">", "").Replace(storeName), ""))
	if r := []rune(name); len(r) > 60 {
		name = strings.TrimSpace(string(r[:60]))
	}
	if name == "" {
		name = "Store"
	}
	doc := map[string]any{
		"profile": map[string]any{
			"name": name, "tagline": nil, "logo_image_id": nil, "favicon_image_id": nil,
			"accent_color": "#247965", "announcement": nil,
			"contact": map[string]any{"email": nil, "phone": nil, "address": nil, "line_url": nil, "facebook_url": nil, "instagram_url": nil},
		},
		"nav":   map[string]any{"header": []any{}, "footer": []any{}},
		"home":  map[string]any{"sections": []any{map[string]any{"type": "product_grid", "heading": nil, "sort": "newest", "limit": 12}}},
		"pages": []any{},
	}
	encoded, _ := json.Marshal(doc) // a literal of maps and strings cannot fail to encode
	return encoded
}
