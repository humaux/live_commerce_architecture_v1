package design

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"livecommerce/internal/command"
)

const img1 = "11111111-1111-4111-8111-111111111111"
const img2 = "22222222-2222-4222-8222-222222222222"

// full is a document using every section type, both nav lists and a page, all valid.
func full() map[string]any {
	return map[string]any{
		"profile": map[string]any{
			"name": "小店", "tagline": "好物", "logo_image_id": img1, "favicon_image_id": nil, "accent_color": "#AABBCC", "announcement": "週末免運",
			"contact": map[string]any{"email": "a@b.co", "phone": "+886 2 1234-5678", "address": "台北市", "line_url": "https://line.me/R/ti/p/@x",
				"facebook_url": "https://facebook.com/x", "instagram_url": nil},
		},
		"nav": map[string]any{
			"header": []any{
				map[string]any{"label": "首頁", "kind": "home", "target": nil},
				map[string]any{"label": "全部", "kind": "all_products"},
				map[string]any{"label": "新品", "kind": "collection", "target": "new-in"},
				map[string]any{"label": "關於", "kind": "page", "target": "about"},
				map[string]any{"label": "部落格", "kind": "url", "target": "https://example.com/blog"},
			},
			"footer": []any{map[string]any{"label": "關於", "kind": "page", "target": "about"}},
		},
		"home": map[string]any{"sections": []any{
			map[string]any{"type": "hero", "image_id": img1, "heading": "Hi", "subheading": nil, "cta_label": "逛逛", "cta_kind": "page", "cta_target": "about"},
			map[string]any{"type": "featured_collection", "collection_slug": "new-in", "heading": nil, "limit": 8},
			map[string]any{"type": "product_grid", "heading": "全部", "sort": "price_asc", "limit": 12},
			map[string]any{"type": "rich_text", "heading": nil, "body": "**粗** and *斜*\n\n- a\n- [site](https://example.com/x)"},
			map[string]any{"type": "image_text", "image_id": img2, "heading": nil, "body": "text", "image_side": "left"},
		}},
		"pages": []any{map[string]any{"slug": "about", "title": "關於我們", "body": "hello"}},
	}
}

func normalize(t *testing.T, doc any) (map[string]any, Refs, error) {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	out, refs, err := Normalize(raw)
	if err != nil {
		return nil, nil, err
	}
	var back map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	return back, refs, nil
}

func wantPath(t *testing.T, doc any, path string) {
	t.Helper()
	_, _, err := normalize(t, doc)
	var v *ValidationError
	if !errors.As(err, &v) || v.Path != path {
		t.Fatalf("want ValidationError at %q, got %v", path, err)
	}
	if !errors.Is(err, command.ErrInvalid) {
		t.Fatalf("ValidationError must satisfy errors.Is(command.ErrInvalid)")
	}
}

func TestFullDocumentNormalizes(t *testing.T) {
	out, refs, err := normalize(t, full())
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[img1] != "profile.logo_image_id" || refs[img2] != "home.sections[4].image_id" {
		t.Fatalf("refs = %v", refs)
	}
	profile := out["profile"].(map[string]any)
	if profile["accent_color"] != "#aabbcc" || profile["favicon_image_id"] != nil {
		t.Fatalf("profile not normalised: %v", profile)
	}
	hero := out["home"].(map[string]any)["sections"].([]any)[0].(map[string]any)
	if _, ok := hero["subheading"]; !ok || hero["subheading"] != nil {
		t.Fatalf("absent nullable must be explicit null: %v", hero)
	}
}

func TestDefaultDocumentValidates(t *testing.T) {
	for _, name := range []string{"Shop", "", strings.Repeat("店", 90)} {
		if _, _, err := Normalize(DefaultDocument(name)); err != nil {
			t.Fatalf("default for %q: %v", name, err)
		}
	}
}

func TestUnknownKeysAreRefusedWithPath(t *testing.T) {
	cases := map[string]func(d map[string]any){
		"extra":               func(d map[string]any) { d["extra"] = 1 },
		"profile.css":         func(d map[string]any) { d["profile"].(map[string]any)["css"] = "x" },
		"profile.contact.fax": func(d map[string]any) { d["profile"].(map[string]any)["contact"].(map[string]any)["fax"] = "1" },
		"nav.header[0].icon":  func(d map[string]any) { d["nav"].(map[string]any)["header"].([]any)[0].(map[string]any)["icon"] = "x" },
		"home.sections[0].html": func(d map[string]any) {
			d["home"].(map[string]any)["sections"].([]any)[0].(map[string]any)["html"] = "x"
		},
		"home.sections[1].cta_label": func(d map[string]any) {
			d["home"].(map[string]any)["sections"].([]any)[1].(map[string]any)["cta_label"] = "x"
		},
		"pages[0].script": func(d map[string]any) { d["pages"].([]any)[0].(map[string]any)["script"] = "x" },
	}
	for path, mutate := range cases {
		d := full()
		mutate(d)
		wantPath(t, d, path)
	}
}

func TestFieldRules(t *testing.T) {
	sections := func(d map[string]any) []any { return d["home"].(map[string]any)["sections"].([]any) }
	cases := []struct {
		path   string
		mutate func(d map[string]any)
	}{
		{"profile.name", func(d map[string]any) { d["profile"].(map[string]any)["name"] = strings.Repeat("a", 61) }},
		{"profile.name", func(d map[string]any) { d["profile"].(map[string]any)["name"] = "  " }},
		{"profile.accent_color", func(d map[string]any) { d["profile"].(map[string]any)["accent_color"] = "red" }},
		{"profile.logo_image_id", func(d map[string]any) { d["profile"].(map[string]any)["logo_image_id"] = "not-a-uuid" }},
		{"profile.contact.line_url", func(d map[string]any) {
			d["profile"].(map[string]any)["contact"].(map[string]any)["line_url"] = "http://line.me/x"
		}},
		{"profile.contact.facebook_url", func(d map[string]any) {
			d["profile"].(map[string]any)["contact"].(map[string]any)["facebook_url"] = "javascript:alert(1)"
		}},
		{"profile.contact.email", func(d map[string]any) { d["profile"].(map[string]any)["contact"].(map[string]any)["email"] = "no-at" }},
		{"profile.contact.phone", func(d map[string]any) { d["profile"].(map[string]any)["contact"].(map[string]any)["phone"] = "call me" }},
		{"nav.header[2].target", func(d map[string]any) {
			d["nav"].(map[string]any)["header"].([]any)[2].(map[string]any)["target"] = "Bad Slug"
		}},
		{"nav.header[0].target", func(d map[string]any) {
			d["nav"].(map[string]any)["header"].([]any)[0].(map[string]any)["target"] = "x"
		}},
		{"nav.header[4].target", func(d map[string]any) {
			d["nav"].(map[string]any)["header"].([]any)[4].(map[string]any)["target"] = "http://example.com"
		}},
		{"nav.header[3].target", func(d map[string]any) {
			d["nav"].(map[string]any)["header"].([]any)[3].(map[string]any)["target"] = "missing"
		}},
		{"nav.footer[0].kind", func(d map[string]any) {
			d["nav"].(map[string]any)["footer"].([]any)[0].(map[string]any)["kind"] = "home"
		}},
		{"home.sections[0].image_id", func(d map[string]any) { delete(sections(d)[0].(map[string]any), "image_id") }},
		{"home.sections[0].cta_target", func(d map[string]any) { sections(d)[0].(map[string]any)["cta_kind"] = "all_products" }},
		{"home.sections[1].limit", func(d map[string]any) { sections(d)[1].(map[string]any)["limit"] = 3 }},
		{"home.sections[1].limit", func(d map[string]any) { sections(d)[1].(map[string]any)["limit"] = 4.5 }},
		{"home.sections[2].limit", func(d map[string]any) { sections(d)[2].(map[string]any)["limit"] = 49 }},
		{"home.sections[2].sort", func(d map[string]any) { sections(d)[2].(map[string]any)["sort"] = "random" }},
		{"home.sections[4].image_side", func(d map[string]any) { sections(d)[4].(map[string]any)["image_side"] = "top" }},
		{"home.sections[3].type", func(d map[string]any) { sections(d)[3].(map[string]any)["type"] = "custom_html" }},
		{"pages[0].slug", func(d map[string]any) { d["pages"].([]any)[0].(map[string]any)["slug"] = "About" }},
		{"pages[0].title", func(d map[string]any) { delete(d["pages"].([]any)[0].(map[string]any), "title") }},
		{"pages[1].slug", func(d map[string]any) {
			d["pages"] = append(d["pages"].([]any), map[string]any{"slug": "about", "title": "t", "body": ""})
		}},
		{"profile", func(d map[string]any) { delete(d, "profile") }},
	}
	for i, c := range cases {
		d := full()
		c.mutate(d)
		t.Run(fmt.Sprintf("%d_%s", i, c.path), func(t *testing.T) { wantPath(t, d, c.path) })
	}
}

func TestListLimits(t *testing.T) {
	over := func(n int, item any) []any {
		out := make([]any, n)
		for i := range out {
			out[i] = item
		}
		return out
	}
	home := map[string]any{"label": "x", "kind": "home"}
	d := full()
	d["nav"].(map[string]any)["header"] = over(8, home)
	if _, _, err := normalize(t, d); err != nil {
		t.Fatalf("8 header items must pass: %v", err)
	}
	d["nav"].(map[string]any)["header"] = over(9, home)
	wantPath(t, d, "nav.header")
	d = full()
	d["nav"].(map[string]any)["footer"] = over(13, map[string]any{"label": "x", "kind": "url", "target": "https://e.com"})
	wantPath(t, d, "nav.footer")
	d = full()
	d["home"].(map[string]any)["sections"] = over(21, map[string]any{"type": "product_grid", "sort": "newest", "limit": 4})
	wantPath(t, d, "home.sections")
	d = full()
	pages := make([]any, 21)
	for i := range pages {
		pages[i] = map[string]any{"slug": fmt.Sprintf("p%d", i), "title": "t", "body": ""}
	}
	d["pages"] = pages
	wantPath(t, d, "pages")
}

func TestMarkdownXSS(t *testing.T) {
	bad := []string{
		`<script>alert(1)</script>`, `a < b`, `<img src=x onerror=alert(1)>`,
		`[x](javascript:alert(1))`, `[x](JAVASCRIPT:alert(1))`, `[x](data:text/html;base64,AAAA)`, `[x](http://example.com)`, `[x](//example.com)`,
		`[x](/relative)`, `[x]( https://example.com)`, `[x](https://exa mple.com)`, `![x](https://example.com/a.png)`, `[x](https://a.com` + "\x00" + `)`,
		`[x](https://a.com"onmouseover="alert(1))`, `[x](https://a.com) and [y](ftp://b.com)`, `[a](https://a.com)](javascript:1)`, "tab\x07bell",
	}
	for _, s := range bad {
		if markdownProblem(s) == "" {
			t.Errorf("markdown %q must be refused", s)
		}
	}
	good := []string{"", "plain text & more", "**b** *i*\n\n- one\n- two", "[ok](https://example.com/a?b=1&c=2#d)", "中文 **粗體** [連結](https://例え.jp/)", "a > b"}
	for _, s := range good {
		if r := markdownProblem(s); r != "" {
			t.Errorf("markdown %q refused: %s", s, r)
		}
	}
	d := full()
	d["pages"].([]any)[0].(map[string]any)["body"] = "<b>x</b>"
	wantPath(t, d, "pages[0].body")
}

func TestStructuralRefusals(t *testing.T) {
	for _, raw := range []string{``, `[]`, `"x"`, `{"profile":{}} {}`, `{"profile":{"name":"a","accent_color":"#000000"}`, strings.Repeat(" ", MaxDocumentBytes+1)} {
		if _, _, err := Normalize([]byte(raw)); err == nil {
			t.Errorf("%.30q must be refused", raw)
		}
	}
	if _, _, err := Normalize([]byte(`{"profile":{"name":"a","accent_color":"#000000"}}`)); err != nil {
		t.Fatalf("minimal document must pass (containers optional): %v", err)
	}
}

func TestTokenHash(t *testing.T) {
	good := strings.Repeat("A", 43)
	if h, ok := TokenHash(good); !ok || len(h) != 32 {
		t.Fatal("43 url-safe chars must hash")
	}
	for _, bad := range []string{"", strings.Repeat("A", 42), strings.Repeat("A", 44), strings.Repeat("+", 43), good + "\n"} {
		if _, ok := TokenHash(bad); ok {
			t.Errorf("%q must be refused", bad)
		}
	}
}
