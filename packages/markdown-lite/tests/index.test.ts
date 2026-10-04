import test from "node:test";
import assert from "node:assert/strict";
import { escapeHtml, isSafeLinkTarget, markdownProblem, renderMarkdown } from "../src/index.ts";

// Every tag in the output must be one of these; anything else is an XSS hole.
const TAG = /<\/?(?:p|ul|li|strong|em|br)>|<a href="https:\/\/[^"<>\s]*" rel="noopener noreferrer nofollow" target="_blank">|<\/a>/y;
function onlyWhitelistedTags(html: string): boolean {
  let rest = html;
  while (rest.length) {
    const lt = rest.indexOf("<");
    if (lt < 0) return true;
    rest = rest.slice(lt);
    TAG.lastIndex = 0;
    const m = TAG.exec(rest);
    if (!m) return false;
    rest = rest.slice(m[0].length);
  }
  return true;
}

test("the four constructs render", () => {
  assert.equal(renderMarkdown("a **b** *c*"), "<p>a <strong>b</strong> <em>c</em></p>");
  assert.equal(renderMarkdown("one\ntwo\n\nthree"), "<p>one<br>two</p><p>three</p>");
  assert.equal(renderMarkdown("- a\n- **b**"), "<ul><li>a</li><li><strong>b</strong></li></ul>");
  assert.equal(
    renderMarkdown("[x *y*](https://example.com/a?b=1&c=2)"),
    '<p><a href="https://example.com/a?b=1&amp;c=2" rel="noopener noreferrer nofollow" target="_blank">x <em>y</em></a></p>',
  );
  assert.equal(renderMarkdown("中文 **粗體**"), "<p>中文 <strong>粗體</strong></p>");
  assert.equal(renderMarkdown(""), "");
});

test("XSS payloads stay inert text", () => {
  const payloads = [
    "<script>alert(1)</script>",
    '<img src=x onerror="alert(1)">',
    "[x](javascript:alert(1))",
    "[x](JaVaScRiPt:alert(1))",
    "[x](data:text/html;base64,PHNjcmlwdD4=)",
    "[x](http://example.com)",
    "[x](//example.com)",
    '[x](https://a.com"onmouseover="alert(1))',
    "[x](https://a.com'onmouseover='alert(1))",
    "[<script>](https://a.com)",
    "[x](https://a.com/<script>)",
    "![x](https://a.com/a.png)",
    "**<b>x</b>**",
    "*[a](https://a.com/**b**)*",
    "&lt;script&gt;",
    "\u0000<script>",
    "[[x](https://a.com)](https://b.com)",
    "[x](https://a.com\u0000)",
  ];
  for (const payload of payloads) {
    const html = renderMarkdown(payload);
    assert.ok(onlyWhitelistedTags(html), `unexpected tag in ${JSON.stringify(html)} for ${JSON.stringify(payload)}`);
    assert.ok(!/href="(?!https:\/\/)/.test(html), `non-https href for ${JSON.stringify(payload)}`);
    assert.ok(!/<[^>]*\son[a-z]+=/i.test(html), `event handler attribute for ${JSON.stringify(payload)}`);
  }
  assert.equal(renderMarkdown("<script>alert(1)</script>"), "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>");
  assert.equal(renderMarkdown("[x](javascript:alert(1))"), "<p>[x](javascript:alert(1))</p>");
});

test("fuzz: any string over a hostile alphabet renders only whitelisted tags", () => {
  const parts = ["[", "]", "(", ")", "*", "**", "<", ">", '"', "'", "&", "- ", "\n", "\n\n", " ", "https://", "a.com", "javascript:", "x", "![", "](", "\u0000", "\r"];
  let seed = 123456789;
  const next = () => (seed = (seed * 1103515245 + 12345) & 0x7fffffff);
  for (let i = 0; i < 3000; i++) {
    let s = "";
    for (let n = next() % 24; n > 0; n--) s += parts[next() % parts.length];
    const html = renderMarkdown(s);
    assert.ok(onlyWhitelistedTags(html), `unexpected tag in ${JSON.stringify(html)} for ${JSON.stringify(s)}`);
    assert.ok(!/href="(?!https:\/\/)/.test(html));
  }
});

test("validator mirrors the Go rules", () => {
  for (const bad of ["<b>", "a < b", "[x](javascript:1)", "[x](http://a.com)", "![x](https://a.com/a.png)", "[x](https://a.com", "bell\u0007", "[a](https://a.com)](x)"])
    assert.notEqual(markdownProblem(bad), null, bad);
  for (const good of ["", "plain & text", "**b** *i*\n- a", "[ok](https://example.com/a?b=1)", "a > b", "中文"])
    assert.equal(markdownProblem(good), null, good);
  assert.ok(isSafeLinkTarget("https://a.com/x") && !isSafeLinkTarget("http://a.com") && !isSafeLinkTarget("https://a.com/(x)"));
  assert.equal(escapeHtml(`<a href="x">&'`), "&lt;a href=&quot;x&quot;&gt;&amp;&#39;");
});
