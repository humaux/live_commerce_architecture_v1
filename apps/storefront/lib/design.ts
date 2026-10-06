// Store design document model for the storefront shell (contracts/storefront-v2.md section B). Pure functions, no I/O:
// the server layer (lib/shop-upstream.ts) reads GET /v1/buyer/design/published|preview through the BFF key and this
// module coerces whatever came back into a closed, safe shape. Go already validates and normalises the document at save
// time (internal/design); this is the second wall: a drifted or hand-edited row can never crash a page or inject markup,
// because every string is only ever rendered as React text (markdown goes through packages/markdown-lite, which escapes).
// It never fetches, never decides a price or stock, and never trusts a URL it has not matched against an allowlist.

export type NavKind = "home" | "all_products" | "collection" | "page" | "url";
export type NavItem = { label: string; kind: NavKind; target: string | null };
export type Contact = {
  email: string | null;
  phone: string | null;
  address: string | null;
  line_url: string | null;
  facebook_url: string | null;
  instagram_url: string | null;
};
export type Profile = {
  name: string;
  tagline: string | null;
  logo_image_id: string | null;
  favicon_image_id: string | null;
  accent_color: string;
  announcement: string | null;
  contact: Contact;
};
export type Section =
  | {
      type: "hero";
      image_id: string | null;
      heading: string | null;
      subheading: string | null;
      cta_label: string | null;
      cta_kind: "all_products" | "collection" | "page" | null;
      cta_target: string | null;
    }
  | { type: "featured_collection"; collection_slug: string; heading: string | null; limit: number }
  | { type: "product_grid"; heading: string | null; sort: "newest" | "price_asc" | "price_desc"; limit: number }
  | { type: "rich_text"; heading: string | null; body: string }
  | { type: "image_text"; image_id: string | null; heading: string | null; body: string; image_side: "left" | "right" };
export type Page = { slug: string; title: string; body: string };
export type Design = {
  profile: Profile;
  nav: { header: NavItem[]; footer: NavItem[] };
  home: { sections: Section[] };
  pages: Page[];
};

export const DEFAULT_ACCENT = "#247965";
const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
const HEX = /^#[0-9a-f]{6}$/;

const isRecord = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
// Go counts these limits in runes (internal/design/schema.go): clip by code point, never by UTF-16 unit (that halves emoji text and can split a pair).
const clip = (v: string, max: number): string => (v.length <= max ? v : [...v].slice(0, max).join(""));
const text = (v: unknown, max: number): string | null =>
  typeof v === "string" && v.trim() !== "" ? clip(v.trim(), max) : null;
const image = (v: unknown): string | null => (typeof v === "string" && UUID.test(v) ? v : null);
const slug = (v: unknown): string | null => (typeof v === "string" && v.length <= 80 && SLUG.test(v) ? v : null);
const limit = (v: unknown, min: number, max: number, fallback: number) =>
  typeof v === "number" && Number.isInteger(v) ? Math.min(max, Math.max(min, v)) : fallback;

// Outbound link allowlist, same families the Go validator accepts for contact/nav targets.
export function safeExternal(v: unknown): string | null {
  if (typeof v !== "string" || v.length > 500 || /[\s<>"']/.test(v)) return null;
  return /^(?:https:\/\/|line:\/\/|tel:|mailto:)/.test(v) ? v : null;
}
const https = (v: unknown): string | null => {
  const s = safeExternal(v);
  return s && s.startsWith("https://") ? s : null;
};

// The contract default for a store with no published version (the same document Go returns with version 0).
export function defaultDesign(name: string): Design {
  return {
    profile: {
      name: name || "Store",
      tagline: null,
      logo_image_id: null,
      favicon_image_id: null,
      accent_color: DEFAULT_ACCENT,
      announcement: null,
      contact: { email: null, phone: null, address: null, line_url: null, facebook_url: null, instagram_url: null },
    },
    nav: { header: [], footer: [] },
    home: { sections: [{ type: "product_grid", heading: null, sort: "newest", limit: 12 }] },
    pages: [],
  };
}

function navItems(raw: unknown, header: boolean): NavItem[] {
  if (!Array.isArray(raw)) return [];
  const kinds: NavKind[] = header ? ["home", "all_products", "collection", "page", "url"] : ["page", "url", "collection"];
  const out: NavItem[] = [];
  for (const item of raw.slice(0, header ? 8 : 12)) {
    if (!isRecord(item)) continue;
    const label = text(item.label, 30);
    const kind = kinds.find((k) => k === item.kind);
    if (!label || !kind) continue;
    if (kind === "home" || kind === "all_products") out.push({ label, kind, target: null });
    else if (kind === "url") {
      const url = safeExternal(item.target);
      if (url) out.push({ label, kind, target: url });
    } else {
      const target = slug(item.target);
      if (target) out.push({ label, kind, target });
    }
  }
  return out;
}

function sections(raw: unknown): Section[] {
  if (!Array.isArray(raw)) return [];
  const out: Section[] = [];
  for (const s of raw.slice(0, 20)) {
    if (!isRecord(s)) continue;
    const heading = text(s.heading, 80);
    switch (s.type) {
      case "hero": {
        const kind = s.cta_kind === "all_products" || s.cta_kind === "collection" || s.cta_kind === "page" ? s.cta_kind : null;
        const target = kind === "collection" || kind === "page" ? slug(s.cta_target) : null;
        const label = text(s.cta_label, 24);
        out.push({
          type: "hero",
          image_id: image(s.image_id),
          heading,
          subheading: text(s.subheading, 160),
          // A button needs a label and a destination; either alone is dropped rather than rendered half-broken.
          cta_label: kind && (kind === "all_products" || target) ? label : null,
          cta_kind: kind && (kind === "all_products" || target) ? kind : null,
          cta_target: target,
        });
        break;
      }
      case "featured_collection": {
        const collection = slug(s.collection_slug);
        if (collection) out.push({ type: "featured_collection", collection_slug: collection, heading, limit: limit(s.limit, 4, 24, 8) });
        break;
      }
      case "product_grid": {
        const sort = s.sort === "price_asc" || s.sort === "price_desc" ? s.sort : "newest";
        out.push({ type: "product_grid", heading, sort, limit: limit(s.limit, 4, 48, 12) });
        break;
      }
      case "rich_text":
        out.push({ type: "rich_text", heading, body: typeof s.body === "string" ? clip(s.body, 4000) : "" });
        break;
      case "image_text":
        out.push({
          type: "image_text",
          image_id: image(s.image_id),
          heading,
          body: typeof s.body === "string" ? clip(s.body, 2000) : "",
          image_side: s.image_side === "left" ? "left" : "right",
        });
        break;
    }
  }
  return out;
}

// Coerce an untrusted `document` into a Design; `storeName` is the fallback for a missing profile name.
export function normalizeDesign(raw: unknown, storeName = ""): Design {
  const base = defaultDesign(storeName);
  if (!isRecord(raw)) return base;
  const p = isRecord(raw.profile) ? raw.profile : {};
  const c = isRecord(p.contact) ? p.contact : {};
  const accent = typeof p.accent_color === "string" && HEX.test(p.accent_color.toLowerCase()) ? p.accent_color.toLowerCase() : DEFAULT_ACCENT;
  const nav = isRecord(raw.nav) ? raw.nav : {};
  const home = isRecord(raw.home) ? raw.home : {};
  const pages: Page[] = [];
  if (Array.isArray(raw.pages))
    for (const page of raw.pages.slice(0, 20)) {
      if (!isRecord(page)) continue;
      const s = slug(page.slug);
      const title = text(page.title, 80);
      if (s && title) pages.push({ slug: s, title, body: typeof page.body === "string" ? clip(page.body, 20000) : "" });
    }
  return {
    profile: {
      name: text(p.name, 60) ?? base.profile.name,
      tagline: text(p.tagline, 120),
      logo_image_id: image(p.logo_image_id),
      favicon_image_id: image(p.favicon_image_id),
      accent_color: accent,
      announcement: text(p.announcement, 140),
      contact: {
        email: text(c.email, 120),
        phone: text(c.phone, 30),
        address: text(c.address, 200),
        line_url: safeExternal(c.line_url),
        facebook_url: https(c.facebook_url),
        instagram_url: https(c.instagram_url),
      },
    },
    nav: { header: navItems(nav.header, true), footer: navItems(nav.footer, false) },
    home: { sections: Array.isArray(home.sections) ? sections(home.sections) : base.home.sections },
    pages,
  };
}

// Internal route for a nav/CTA target; null for kinds that are external (rendered as <a href=url> by the caller).
export function internalHref(locale: string, kind: NavKind | "none", target: string | null): string | null {
  switch (kind) {
    case "home":
      return `/${locale}`;
    case "all_products":
      return `/${locale}/products`;
    case "collection":
      return target ? `/${locale}/collections/${target}` : null;
    case "page":
      return target ? `/${locale}/pages/${target}` : null;
    default:
      return null;
  }
}

// Preview mode keeps the token on in-site links so the whole draft can be browsed (the token only ever travels to the
// BFF in a header; Referrer-Policy is same-origin, so it never leaves this origin).
export function withPreview(href: string, preview: string | null): string {
  return preview && href.startsWith("/") ? `${href}${href.includes("?") ? "&" : "?"}preview=${encodeURIComponent(preview)}` : href;
}

// WCAG relative luminance / contrast, so a merchant's pale accent still yields readable buttons and links.
function channel(v: number): number {
  const s = v / 255;
  return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}
export function luminance(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
}
export function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}
// Text colour for a filled accent button: white or pure black, whichever contrasts more. Pure black (not the page ink) on
// purpose: for any colour, max(contrast with white, contrast with black) >= 4.58, whereas an off-black leaves a gap of
// mid-tone accents (e.g. #7a5cff) where neither choice reaches 4.5:1.
export function onAccent(accent: string): string {
  return contrast(accent, "#ffffff") >= contrast(accent, "#000000") ? "#ffffff" : "#000000";
}
// Accent usable as text/link colour on a white page (>= 4.5:1): darken in 6% steps toward black until it passes.
export function accentText(accent: string): string {
  let n = parseInt(accent.slice(1), 16);
  let color = accent;
  for (let i = 0; i < 20 && contrast(color, "#ffffff") < 4.5; i++) {
    const r = Math.round(((n >> 16) & 255) * 0.94);
    const g = Math.round(((n >> 8) & 255) * 0.94);
    const b = Math.round((n & 255) * 0.94);
    n = (r << 16) | (g << 8) | b;
    color = `#${n.toString(16).padStart(6, "0")}`;
  }
  return color;
}

export type NavLink = { label: string; href: string; external: boolean };

// Header/footer nav items -> concrete links. Internal kinds keep the preview token; "url" kinds are external and
// were already restricted to https/line/tel/mailto by normalizeDesign.
export function resolveNav(locale: string, items: NavItem[], preview: string | null): NavLink[] {
  const out: NavLink[] = [];
  for (const item of items) {
    if (item.kind === "url") {
      if (item.target) out.push({ label: item.label, href: item.target, external: item.target.startsWith("https://") });
      continue;
    }
    const href = internalHref(locale, item.kind, item.target);
    if (href) out.push({ label: item.label, href: withPreview(href, preview), external: false });
  }
  return out;
}
