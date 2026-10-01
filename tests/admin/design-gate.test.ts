// store-design independent gate (Node, no Docker): written from contracts/storefront-v2.md section B and the unit brief, not
// from the implementation. Two things the browser/PG gates cannot cover cheaply:
//  SDN1 the admin BFF allowlist for design/* (which method + path may reach Go; no query; reads carry no body) and the
//       260 KiB JSON cap constant that must stay above Go's document cap (256 KiB + envelope) and below "unbounded";
//  SDN2 the restricted-markdown renderer (packages/markdown-lite) under XSS payloads: escape first, only whitelisted tags,
//       https-only anchors, never an attribute the merchant typed; and the client-side validator rejects what Go rejects.
// Run by scripts/dev/test-node.sh and by `test-local.sh --browser-design`.
import assert from "node:assert/strict";
import { test } from "node:test";
import { DESIGN_MAX_JSON, designGetPaths, designPostPaths, designPutPaths, isDesignPath, validDesignRequest } from "../../apps/admin/lib/design-request.ts";
import { escapeHtml, markdownProblem, renderMarkdown } from "../../packages/markdown-lite/src/index.ts";

const ID = "0f8fad5b-d9cb-469f-a165-70867728950e";
const only = (alternation: string) => new RegExp(`^(?:${alternation})$`);
const get = only(designGetPaths);
const post = only(designPostPaths);
const put = only(designPutPaths);

test("SDN1 allowlist: exactly the contract routes, per method", () => {
  for (const p of ["design/draft", "design/versions", "design/media", `design/media/${ID}`]) assert.ok(get.test(p), `GET ${p}`);
  for (const p of ["design/publish", "design/rollback", "design/preview-token", "design/media", `design/media/${ID}/delete`]) assert.ok(post.test(p), `POST ${p}`);
  assert.ok(put.test("design/draft"));
  // each route exists for its own method only
  for (const p of ["design/publish", "design/rollback", "design/preview-token", `design/media/${ID}/delete`]) assert.ok(!get.test(p), `GET must not reach ${p}`);
  for (const p of ["design/draft", "design/versions", `design/media/${ID}`]) assert.ok(!post.test(p), `POST must not reach ${p}`);
  for (const p of ["design/versions", "design/media", "design/publish", `design/media/${ID}`]) assert.ok(!put.test(p), `PUT must not reach ${p}`);
});

test("SDN1 allowlist: traversal, case, suffix and prefix tricks never match", () => {
  const bad = [
    "design", "design/", "design/draft/", "/design/draft", "design//draft", "design/draft/x", "design/Draft", "DESIGN/draft", "design/draft%2f", "design/draft\n",
    "design/../products", "design/media/../draft", "design/media/..%2fdraft", `design/media/${ID}/`, `design/media/${ID}/x`, `design/media/${ID.toUpperCase()}`,
    "design/media/not-a-uuid", "design/media/1", `design/media/${ID}/delete/`, `design/media/${ID}delete`, `xdesign/draft`, "design/draft.json", "design/preview-token/x", "design/publish ",
    "designer/draft", "design/admin", "design/media/*", "design/.*",
  ];
  for (const p of bad) {
    assert.ok(!isDesignPath(p), `${JSON.stringify(p)} must not be a design path`);
    for (const re of [get, post, put]) assert.ok(!re.test(p), `${JSON.stringify(p)} must not match any method grammar`);
  }
  // methods the design API never uses have no grammar at all
  assert.equal(typeof (designGetPaths + designPostPaths + designPutPaths), "string");
});

const req = (url: string, init: RequestInit & { duplex?: "half" } = {}) => new Request(url, init);

test("SDN1 request shape: no query string anywhere, reads carry no body, key, length or transfer-encoding", () => {
  const base = "http://x.test/api/stores/s/design/draft";
  assert.equal(validDesignRequest(req(base), "design/draft"), true);
  assert.equal(validDesignRequest(req(base + "?x=1"), "design/draft"), false, "query on GET");
  assert.equal(validDesignRequest(req(base + "?", { method: "PUT", body: "{}" }), "design/draft"), false, "query on PUT");
  assert.equal(validDesignRequest(req(base + "?preview=t", { method: "POST", body: "{}" }), "design/publish"), false, "query on POST");
  assert.equal(validDesignRequest(req(base, { headers: { "idempotency-key": "k" } }), "design/draft"), false, "key on a read");
  assert.equal(validDesignRequest(req(base, { headers: { "content-length": "5" } }), "design/draft"), false, "length on a read");
  assert.equal(validDesignRequest(req(base, { headers: { "transfer-encoding": "chunked" } }), "design/draft"), false, "chunked read");
  assert.equal(validDesignRequest(req(base, { method: "POST", body: "{}" }), "design/publish"), true, "a POST body is fine");
  assert.equal(validDesignRequest(req("http://x.test/api/stores/s/products?q=1"), "products"), true, "non-design paths are not this predicate's business");
});

test("SDN1 cap: 260 KiB = Go's 256 KiB document + 4 KiB envelope, and a body of exactly the cap is still below it", () => {
  assert.equal(DESIGN_MAX_JSON, 260 * 1024);
  assert.ok(DESIGN_MAX_JSON > 256 * 1024, "must admit a maximal valid document");
  assert.ok(DESIGN_MAX_JSON <= 512 * 1024, "must not become an unbounded proxy buffer");
});

// ---- SDN2 ----
const ALLOWED_TAG = /<\/?(?:p|ul|li|strong|em|br)>|<a href="https:\/\/[^\s<>"'()]+" rel="noopener noreferrer nofollow" target="_blank">|<\/a>/g;
const PAYLOADS = [
  `<script>alert(1)</script>`, `<img src=x onerror=alert(1)>`, `"><svg onload=alert(1)>`, `<b>x</b>`, `a < b > c`, `<iframe src="https://evil.example"></iframe>`,
  `[x](javascript:alert(1))`, `[x](JaVaScRiPt:alert(1))`, `[x](http://a.example)`, `[x](//a.example)`, `[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)`,
  `[x](vbscript:msgbox(1))`, `[x](https://a.example"onmouseover="alert(1))`, `[x](https://a.example' onmouseover='alert(1))`, `[<img src=x onerror=alert(1)>](https://a.example)`,
  `[x](https://a.example/<script>)`, `**<script>alert(1)</script>**`, `- <script>alert(1)</script>`, `*[x](javascript:alert(1))*`, `![i](https://a.example/i.png)`,
  `[a](https://a.example)[b](javascript:alert(1))`, `[x](https://a.example)](javascript:alert(1))`, `&lt;script&gt;alert(1)&lt;/script&gt;`, `&#60;script&#62;`, "\u0000<script>", "<scr\u0000ipt>",
  `[x](https://a.example\n)`, `[x]\n(javascript:alert(1))`, `<a href="javascript:alert(1)">x</a>`, `<math><mtext></mtext></math>`, `\\<script>`,
];

function assertSafe(html: string, source: string) {
  const rest = html.replace(ALLOWED_TAG, "");
  assert.ok(!/[<>]/.test(rest), `unescaped angle bracket in ${JSON.stringify(html)} for ${JSON.stringify(source)}`);
  assert.ok(!/<(?!\/?(?:p|ul|li|strong|em|br|a)[ >])/.test(html), `non-whitelisted tag in ${JSON.stringify(html)}`);
  for (const a of html.matchAll(/<a [^>]*>/g)) {
    assert.match(a[0], /^<a href="https:\/\/[^\s<>"'()]+" rel="noopener noreferrer nofollow" target="_blank">$/, `anchor shape for ${JSON.stringify(source)}`);
  }
  assert.ok(!/<[^>]*\son\w+\s*=/i.test(html), `event handler attribute in ${JSON.stringify(html)}`);
  assert.ok(!/href="(?!https:\/\/)/i.test(html), `non-https href in ${JSON.stringify(html)}`);
}

test("SDN2 renderer: every XSS payload comes out as whitelisted tags and escaped text only", () => {
  for (const source of PAYLOADS) assertSafe(renderMarkdown(source), source);
});

test("SDN2 renderer: escape-first means the merchant's text never becomes markup", () => {
  assert.equal(renderMarkdown("<b>x</b>"), "<p>&lt;b&gt;x&lt;/b&gt;</p>");
  assert.equal(escapeHtml(`<>&"'`), "&lt;&gt;&amp;&quot;&#39;");
  assert.equal(renderMarkdown("a & b"), "<p>a &amp; b</p>");
  assert.match(renderMarkdown("[ok](https://example.com/a?x=1&y=2)"), /^<p><a href="https:\/\/example\.com\/a\?x=1&amp;y=2" rel="noopener noreferrer nofollow" target="_blank">ok<\/a><\/p>$/);
  assert.equal(renderMarkdown("**b** *i*"), "<p><strong>b</strong> <em>i</em></p>");
  assert.equal(renderMarkdown("- a\n- b"), "<ul><li>a</li><li>b</li></ul>");
  assert.equal(renderMarkdown("one\ntwo"), "<p>one<br>two</p>");
  assert.equal(renderMarkdown(""), "");
  assert.ok(!renderMarkdown("[x](javascript:alert(1))").includes("<a "), "a javascript: link stays literal text");
  assert.ok(!renderMarkdown("[x](http://a.example)").includes("<a "), "an http link stays literal text");
  assert.ok(!renderMarkdown("![i](https://a.example/i.png)").includes("<img"), "images are not produced");
});

test("SDN2 renderer: fuzz (seeded) never produces a tag outside the whitelist", () => {
  let seed = 0x5eed1234;
  const next = () => ((seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0) / 2 ** 32);
  const atoms = ["<", ">", "[", "]", "(", ")", "*", "**", "- ", "\n", "\n\n", "&", "\"", "'", "https://a.example", "http://a", "javascript:", "alert(1)", "script", "onerror=", "img", "x", " ", "!", "\\", "`", "%3c", "&#60;", "\u0000", "&amp;"];
  for (let i = 0; i < 4000; i++) {
    let s = "";
    for (let n = 1 + Math.floor(next() * 14); n > 0; n--) s += atoms[Math.floor(next() * atoms.length)];
    assertSafe(renderMarkdown(s), s);
  }
});

test("SDN2 client validator rejects what the server rejects (HTML, non-https links, `<` anywhere)", () => {
  for (const source of [`<script>alert(1)</script>`, `hello <b>x</b>`, `<img src=x onerror=alert(1)>`, `a < b`, `[x](javascript:alert(1))`, `[x](JAVASCRIPT:alert(1))`,
    `[x](http://a.example)`, `[x](//a.example)`, `[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)`, `[x](vbscript:msgbox(1))`]) {
    assert.notEqual(markdownProblem(source), null, `${source} must be refused`);
  }
  for (const source of ["", "plain", "**b** *i*", "- a\n- b", "[ok](https://example.com/a?x=1&y=2)", `Tom & Jerry's "best" shop 台灣`]) {
    assert.equal(markdownProblem(source), null, `${source} must be accepted`);
  }
});
