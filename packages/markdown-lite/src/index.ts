// markdown-lite: the restricted markdown of storefront design text (contracts/storefront-v2.md section B): paragraphs,
// **bold**, *italic*, "- " lists and [text](https://...) links, nothing else.
// Used by: apps/admin/components/DesignPages.tsx (live preview now), apps/storefront (storefront-shell, later). The Go side
// (internal/design/markdown.go, markdownProblem) validates the same grammar at save time; this module renders it.
// It never emits anything but the whitelisted tags below, never passes raw HTML, images or non-https links through
// (they stay literal, escaped text), and has no dependency. Escape first, then apply the constructs: every character the
// merchant typed is HTML-escaped before a single tag is introduced, so no input can add markup of its own.
// Not a markdown implementation: no headings, code, tables, nesting beyond bold/italic inside list items and links.
// A package.json is intentionally absent (no lockfile change in unit store-design): consumers import it by relative path
// or tsconfig alias; add package.json + a pnpm-lock importer line when storefront-shell makes it a real workspace dependency.

const ENTITIES: Record<string, string> = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

export function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => ENTITIES[c]);
}

// Same pattern and target rule as internal/design/markdown.go (linkPattern, linkTarget); keep the three in sync.
const LINK = /\[([^[\]\n]*)\]\(([^)\n]*)\)/g;
const TARGET = /^https:\/\/[^\s<>"'()]{1,500}$/;

/** True for an absolute https URL with no whitespace, quotes, angle brackets or parentheses. */
export function isSafeLinkTarget(url: string): boolean {
  return TARGET.test(url);
}

/** Mirror of the Go validator (internal/design markdownProblem): null when acceptable, else a short English reason. */
export function markdownProblem(text: string): string | null {
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(text)) return "control characters are not allowed";
  if (text.includes("<")) return "raw HTML is not allowed";
  if (text.includes("![")) return "images are not allowed in text";
  for (const match of text.matchAll(LINK)) if (!isSafeLinkTarget(match[2])) return "links must be absolute https URLs";
  if (text.replace(LINK, "").includes("](")) return "malformed link";
  return null;
}

function emphasis(escaped: string): string {
  return escaped.replace(/\*\*([^*\n]+)\*\*/g, "<strong>$1</strong>").replace(/\*([^*\n]+)\*/g, "<em>$1</em>");
}

// One line of text -> HTML. Links are lifted into placeholders first so emphasis can never rewrite a URL.
function inline(line: string): string {
  const escaped = escapeHtml(line);
  const anchors: string[] = [];
  const lifted = escaped.replace(LINK, (whole, text: string, target: string) => {
    // The target was escaped with the rest: an entity other than &amp; means the merchant typed a quote or bracket.
    if (!isSafeLinkTarget(target) || /&(?:quot|#39|lt|gt);/.test(target)) return whole;
    anchors.push(`<a href="${target}" rel="noopener noreferrer nofollow" target="_blank">${emphasis(text)}</a>`);
    return `\u0000${anchors.length - 1}\u0000`;
  });
  return emphasis(lifted).replace(/\u0000(\d+)\u0000/g, (_, index: string) => anchors[Number(index)] ?? "");
}

/** Render restricted markdown to an HTML string made only of p, ul, li, strong, em, br and a (https, nofollow). */
export function renderMarkdown(source: string): string {
  const lines = source.replace(/\u0000/g, "").replace(/\r\n?/g, "\n").split("\n");
  const html: string[] = [];
  let paragraph: string[] = [];
  let list: string[] = [];
  const flush = () => {
    if (paragraph.length) html.push(`<p>${paragraph.map(inline).join("<br>")}</p>`);
    if (list.length) html.push(`<ul>${list.map((item) => `<li>${inline(item)}</li>`).join("")}</ul>`);
    paragraph = [];
    list = [];
  };
  for (const line of lines) {
    if (line.trim() === "") flush();
    else if (line.startsWith("- ")) {
      if (paragraph.length) flush();
      list.push(line.slice(2));
    } else {
      if (list.length) flush();
      paragraph.push(line);
    }
  }
  flush();
  return html.join("");
}
