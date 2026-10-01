// Storefront design model of the admin Design page: the document shape of contracts/storefront-v2.md section B, the
// strict parsers of the Go answers (internal/design: draft, versions, media, preview token) and the pure helpers the
// editor uses (blank section factories, payload cleaning, local "what is still missing" checks). Pure: no fetch, no React.
// The server (Go internal/design.Normalize) is the validation authority; the local checks only save a round trip.

export const MAX = { header: 8, footer: 12, sections: 20, pages: 20, media: 60 } as const;
export const SECTION_TYPES = ["hero", "featured_collection", "product_grid", "rich_text", "image_text"] as const;
export type SectionType = (typeof SECTION_TYPES)[number];
export type Nullable = string | null;

export type NavItem = { label: string; kind: "home" | "all_products" | "collection" | "page" | "url"; target: Nullable };
export type Section =
  | { type: "hero"; image_id: Nullable; heading: Nullable; subheading: Nullable; cta_label: Nullable; cta_kind: "all_products" | "collection" | "page" | null; cta_target: Nullable }
  | { type: "featured_collection"; collection_slug: string; heading: Nullable; limit: number }
  | { type: "product_grid"; heading: Nullable; sort: "newest" | "price_asc" | "price_desc"; limit: number }
  | { type: "rich_text"; heading: Nullable; body: string }
  | { type: "image_text"; image_id: Nullable; heading: Nullable; body: string; image_side: "left" | "right" };
export type Page = { slug: string; title: string; body: string };
export type DesignDocument = {
  profile: {
    name: string; tagline: Nullable; logo_image_id: Nullable; favicon_image_id: Nullable; accent_color: string; announcement: Nullable;
    contact: { email: Nullable; phone: Nullable; address: Nullable; line_url: Nullable; facebook_url: Nullable; instagram_url: Nullable };
  };
  nav: { header: NavItem[]; footer: NavItem[] };
  home: { sections: Section[] };
  pages: Page[];
};
export type Draft = { version: number; document: DesignDocument; updated_at: string | null; published_version: number | null };
export type VersionInfo = { version: number; kind: "publish" | "rollback"; source_version: number; published_at: string; published_by: string };
export type VersionList = { items: VersionInfo[]; live_version: number | null };
export type MediaItem = { id: string; content_type: string; size_bytes: number; width: number | null; height: number | null; created_at: string };
export type PreviewToken = { token: string; draft_version: number; expires_at: string };

export const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export const slugShape = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

const record = (v: unknown): Record<string, unknown> | null => (v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : null);
const int = (v: unknown): v is number => typeof v === "number" && Number.isSafeInteger(v);
const text = (v: unknown): v is string => typeof v === "string";
const stamp = (v: unknown): v is string => typeof v === "string" && !Number.isNaN(Date.parse(v));
function need(ok: boolean): void {
  if (!ok) throw new Error("design_shape");
}

// The document is whatever the server normalised; the editor needs the closed top level and arrays, the rest is read
// defensively by the components (a missing optional string renders as empty).
export function parseDocument(value: unknown): DesignDocument {
  const d = record(value);
  need(!!d && Object.keys(d).sort().join(",") === "home,nav,pages,profile");
  const doc = d as Record<string, unknown>;
  const profile = record(doc.profile);
  const nav = record(doc.nav);
  const home = record(doc.home);
  need(!!profile && !!nav && !!home && Array.isArray(nav.header) && Array.isArray(nav.footer) && Array.isArray(home.sections) && Array.isArray(doc.pages));
  need(text(profile!.name) && text(profile!.accent_color) && !!record(profile!.contact));
  return doc as unknown as DesignDocument;
}

export function parseDraft(value: unknown): Draft {
  const d = record(value);
  need(!!d && Object.keys(d).sort().join(",") === "document,published_version,updated_at,version");
  const r = d as Record<string, unknown>;
  need(int(r.version) && r.version >= 0 && (r.published_version === null || int(r.published_version)) && (r.updated_at === null || stamp(r.updated_at)));
  return { version: r.version as number, document: parseDocument(r.document), updated_at: r.updated_at as string | null, published_version: r.published_version as number | null };
}

function parseVersion(value: unknown): VersionInfo {
  const v = record(value);
  need(!!v && Object.keys(v).sort().join(",") === "kind,published_at,published_by,source_version,version");
  const r = v as Record<string, unknown>;
  need(int(r.version) && int(r.source_version) && (r.kind === "publish" || r.kind === "rollback") && stamp(r.published_at) && text(r.published_by));
  return r as unknown as VersionInfo;
}
export const parseVersionInfo = parseVersion;

export function parseVersions(value: unknown): VersionList {
  const v = record(value);
  need(!!v && Object.keys(v).sort().join(",") === "items,live_version" && Array.isArray(v!.items) && (v!.live_version === null || int(v!.live_version)));
  return { items: (v!.items as unknown[]).map(parseVersion), live_version: v!.live_version as number | null };
}

export function parseMedia(value: unknown): MediaItem {
  const v = record(value);
  need(!!v && Object.keys(v).sort().join(",") === "content_type,created_at,height,id,size_bytes,width");
  const r = v as Record<string, unknown>;
  need(text(r.id) && uuid.test(r.id) && text(r.content_type) && int(r.size_bytes) && stamp(r.created_at) && (r.width === null || int(r.width)) && (r.height === null || int(r.height)));
  return r as unknown as MediaItem;
}

export function parseMediaList(value: unknown): MediaItem[] {
  const v = record(value);
  need(!!v && Object.keys(v).join(",") === "items" && Array.isArray(v!.items) && v!.items.length <= MAX.media);
  return (v!.items as unknown[]).map(parseMedia);
}

export function parsePreviewToken(value: unknown): PreviewToken {
  const v = record(value);
  need(!!v && Object.keys(v).sort().join(",") === "draft_version,expires_at,token");
  const r = v as Record<string, unknown>;
  need(text(r.token) && /^[A-Za-z0-9_-]{43}$/.test(r.token) && int(r.draft_version) && stamp(r.expires_at));
  return r as unknown as PreviewToken;
}

export function blankSection(type: SectionType, firstImage: string | null): Section {
  switch (type) {
    case "hero": return { type, image_id: firstImage, heading: null, subheading: null, cta_label: null, cta_kind: null, cta_target: null };
    case "featured_collection": return { type, collection_slug: "", heading: null, limit: 8 };
    case "product_grid": return { type, heading: null, sort: "newest", limit: 12 };
    case "rich_text": return { type, heading: null, body: "" };
    case "image_text": return { type, image_id: firstImage, heading: null, body: "", image_side: "left" };
  }
}

// JSON sent to PUT design/draft: every cleared field ("") becomes null (the server reads null as unset and "is required"
// when the field is mandatory). `body` keeps "" because an empty text is a legal value.
export function cleanForSave(doc: DesignDocument): unknown {
  const walk = (value: unknown, key: string): unknown => {
    if (typeof value === "string") return value === "" && key !== "body" ? null : value;
    if (Array.isArray(value)) return value.map((item) => walk(item, key));
    if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, walk(v, k)]));
    return value;
  };
  return walk(doc, "");
}

export type Issue = { path: string; reason: "required" | "slug" | "page" };
// What the server would refuse for a missing mandatory value; lets the editor disable Save with a precise hint.
export function localIssues(doc: DesignDocument): Issue[] {
  const out: Issue[] = [];
  const blank = (v: Nullable | undefined) => !v || v.trim() === "";
  if (blank(doc.profile.name)) out.push({ path: "profile.name", reason: "required" });
  const pageSlugs = new Set(doc.pages.map((p) => p.slug));
  const navs: [string, NavItem[]][] = [["nav.header", doc.nav.header], ["nav.footer", doc.nav.footer]];
  for (const [base, items] of navs)
    items.forEach((item, i) => {
      const path = `${base}[${i}]`;
      if (blank(item.label)) out.push({ path: `${path}.label`, reason: "required" });
      if (item.kind === "collection" && !slugShape.test(item.target ?? "")) out.push({ path: `${path}.target`, reason: "slug" });
      if (item.kind === "page" && !pageSlugs.has(item.target ?? "")) out.push({ path: `${path}.target`, reason: "page" });
      if (item.kind === "url" && !/^https:\/\/\S+$/.test(item.target ?? "")) out.push({ path: `${path}.target`, reason: "required" });
    });
  doc.home.sections.forEach((s, i) => {
    const path = `home.sections[${i}]`;
    if ((s.type === "hero" || s.type === "image_text") && blank(s.image_id)) out.push({ path: `${path}.image_id`, reason: "required" });
    if (s.type === "featured_collection" && !slugShape.test(s.collection_slug)) out.push({ path: `${path}.collection_slug`, reason: "slug" });
    if (s.type === "hero" && s.cta_kind === "collection" && !slugShape.test(s.cta_target ?? "")) out.push({ path: `${path}.cta_target`, reason: "slug" });
    if (s.type === "hero" && s.cta_kind === "page" && !pageSlugs.has(s.cta_target ?? "")) out.push({ path: `${path}.cta_target`, reason: "page" });
  });
  const seen = new Set<string>();
  doc.pages.forEach((p, i) => {
    if (!slugShape.test(p.slug)) out.push({ path: `pages[${i}].slug`, reason: "slug" });
    else if (seen.has(p.slug)) out.push({ path: `pages[${i}].slug`, reason: "slug" });
    seen.add(p.slug);
    if (blank(p.title)) out.push({ path: `pages[${i}].title`, reason: "required" });
  });
  return out;
}

// Move item i by delta inside a copy; out-of-range moves return the same array contents.
export function moved<T>(list: T[], i: number, delta: number): T[] {
  const j = i + delta;
  if (j < 0 || j >= list.length) return list;
  const next = [...list];
  [next[i], next[j]] = [next[j], next[i]];
  return next;
}
