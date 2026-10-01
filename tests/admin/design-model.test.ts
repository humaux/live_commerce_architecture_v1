// store-design: pure logic of the Design editor (apps/admin/lib/design-model.ts, design-request.ts): payload cleaning,
// the local "still missing" hints, reorder, the strict Go-shape parsers and the BFF allowlist grammar. Synthetic data only.
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  blankSection, cleanForSave, localIssues, moved, parseDraft, parseMediaList, parsePreviewToken, parseVersions, type DesignDocument,
} from "../../apps/admin/lib/design-model.ts";
import { designGetPaths, designPostPaths, designPutPaths, isDesignPath, isDesignUpload, validDesignRequest } from "../../apps/admin/lib/design-request.ts";

const IMG = "11111111-1111-4111-8111-111111111111";
const doc = (): DesignDocument => ({
  profile: { name: "Shop", tagline: null, logo_image_id: null, favicon_image_id: null, accent_color: "#247965", announcement: null,
    contact: { email: null, phone: null, address: null, line_url: null, facebook_url: null, instagram_url: null } },
  nav: { header: [], footer: [] },
  home: { sections: [] },
  pages: [],
});

test("cleanForSave turns cleared fields into null but keeps an empty body", () => {
  const d = doc();
  d.profile.tagline = "";
  d.pages = [{ slug: "about", title: "About", body: "" }];
  d.home.sections = [{ type: "rich_text", heading: "", body: "" }];
  const out = cleanForSave(d) as DesignDocument;
  assert.equal(out.profile.tagline, null);
  assert.equal(out.pages[0].body, "");
  assert.equal((out.home.sections[0] as { heading: unknown }).heading, null);
  assert.equal((out.home.sections[0] as { body: unknown }).body, "");
  assert.equal(d.profile.tagline, "", "input is not mutated");
});

test("localIssues names each missing mandatory value by server path", () => {
  assert.deepEqual(localIssues(doc()), []);
  const d = doc();
  d.profile.name = " ";
  d.pages = [{ slug: "Bad Slug", title: "", body: "" }, { slug: "ok", title: "t", body: "" }, { slug: "ok", title: "t", body: "" }];
  d.nav.header = [
    { label: "", kind: "home", target: null },
    { label: "c", kind: "collection", target: "" },
    { label: "p", kind: "page", target: "missing" },
    { label: "u", kind: "url", target: "http://x.com" },
  ];
  d.home.sections = [blankSection("hero", null), blankSection("featured_collection", null), { ...blankSection("image_text", IMG), image_id: IMG }];
  const paths = localIssues(d).map((i) => i.path).sort();
  assert.deepEqual(paths, [
    "home.sections[0].image_id", "home.sections[1].collection_slug",
    "nav.header[0].label", "nav.header[1].target", "nav.header[2].target", "nav.header[3].target",
    "pages[0].slug", "pages[0].title", "pages[2].slug", "profile.name",
  ].sort());
});

test("moved swaps neighbours and ignores out-of-range moves", () => {
  assert.deepEqual(moved([1, 2, 3], 0, 1), [2, 1, 3]);
  assert.deepEqual(moved([1, 2, 3], 2, -1), [1, 3, 2]);
  assert.deepEqual(moved([1, 2, 3], 0, -1), [1, 2, 3]);
  assert.deepEqual(moved([1, 2, 3], 2, 1), [1, 2, 3]);
});

test("parsers accept the exact Go shapes and refuse extra or missing keys", () => {
  const draft = { version: 2, document: doc(), updated_at: "2026-10-01T10:00:00Z", published_version: null };
  assert.equal(parseDraft(draft).version, 2);
  assert.throws(() => parseDraft({ ...draft, extra: 1 }));
  assert.throws(() => parseDraft({ ...draft, document: { ...doc(), css: "x" } }));
  assert.throws(() => parseDraft({ ...draft, version: 1.5 }));
  const media = { id: IMG, content_type: "image/png", size_bytes: 10, width: 1, height: null, created_at: "2026-10-01T10:00:00Z" };
  assert.equal(parseMediaList({ items: [media] }).length, 1);
  assert.throws(() => parseMediaList({ items: [{ ...media, id: "not-a-uuid" }] }));
  assert.throws(() => parseMediaList({ items: Array(61).fill(media) }));
  assert.equal(parseVersions({ items: [], live_version: null }).live_version, null);
  assert.throws(() => parseVersions({ items: [{ version: 1, kind: "other", source_version: 1, published_at: "2026-10-01T10:00:00Z", published_by: "x" }], live_version: 1 }));
  assert.equal(parsePreviewToken({ token: "A".repeat(43), draft_version: 1, expires_at: "2026-10-01T10:15:00Z" }).draft_version, 1);
  assert.throws(() => parsePreviewToken({ token: "short", draft_version: 1, expires_at: "2026-10-01T10:15:00Z" }));
});

test("BFF allowlist covers exactly the design resources", () => {
  for (const ok of ["design/draft", "design/versions", "design/media", `design/media/${IMG}`, "design/publish", "design/rollback", "design/preview-token", `design/media/${IMG}/delete`])
    assert.ok(isDesignPath(ok), ok);
  for (const bad of ["design", "design/", "design/draft/x", "design/media/x", `design/media/${IMG}/other`, "design/../draft", "designs/draft", "design/Draft"])
    assert.ok(!isDesignPath(bad), bad);
  assert.ok(designGetPaths.includes("design/draft") && designPostPaths.includes("design/publish") && designPutPaths === "design/draft");
  assert.ok(isDesignUpload("POST", "design/media") && !isDesignUpload("GET", "design/media") && !isDesignUpload("POST", `design/media/${IMG}`));
});

test("validDesignRequest refuses queries and bodies on reads, ignores other paths", () => {
  const get = (url: string, init: RequestInit = {}) => new Request(url, { method: "GET", ...init });
  assert.ok(validDesignRequest(get("http://x/api/stores/s/design/draft"), "design/draft"));
  assert.ok(!validDesignRequest(get("http://x/api/stores/s/design/draft?a=1"), "design/draft"));
  assert.ok(!validDesignRequest(get("http://x/api/stores/s/design/draft?"), "design/draft"));
  assert.ok(!validDesignRequest(get("http://x/api/stores/s/design/draft", { headers: { "idempotency-key": "abcdefgh" } }), "design/draft"));
  assert.ok(!validDesignRequest(get("http://x/api/stores/s/design/draft", { headers: { "content-length": "5" } }), "design/draft"));
  assert.ok(!validDesignRequest(new Request("http://x/a/design/publish?x=1", { method: "POST", body: "{}" }), "design/publish"));
  assert.ok(validDesignRequest(new Request("http://x/a/design/publish", { method: "POST", body: "{}" }), "design/publish"));
  assert.ok(validDesignRequest(get("http://x/a/products?limit=1"), "products"));
});
