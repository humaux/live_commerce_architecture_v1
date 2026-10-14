// Purpose: Route /{locale}/ui-gallery: the design-system gallery. Rendered only when LC_UI_GALLERY=1; every other deployment answers 404 so it never ships to merchants.
// Depends on: next (notFound), @live-commerce/i18n (isLocale), ../../../components/UiGallery; reads one env flag at request time, no API, session or store data.
// Used by: Next.js App Router; tests/admin/ui-gallery-runner.mjs (--browser-ui-gallery) screenshots and audits it.
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { UiGallery } from "@/components/UiGallery";

// Developer-only page, not localized product copy; the title exists so the document passes the axe document-title rule.
export const metadata: Metadata = { title: "Design system gallery", robots: { index: false, follow: false } };

type Raw = Record<string, string | string[] | undefined>;
const one = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v) ?? "";
const pick = (v: string, allowed: readonly string[], fallback: string) => (allowed.includes(v) ? v : fallback);

export default async function UiGalleryPage({ params, searchParams }: { params: Promise<{ locale: string }>; searchParams: Promise<Raw> }) {
  if (process.env.LC_UI_GALLERY !== "1") notFound();
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const raw = await searchParams;
  const page = one(raw.page);
  const initial = {
    tab: pick(one(raw.tab), ["a", "b", "c"], "a"),
    stab: pick(one(raw.stab), ["a", "b", "c"], "a"),
    sw: pick(one(raw.sw), ["0", "1"], "1"),
    setting: pick(one(raw.setting), ["profile", "shipping", "payments", "notices"], "profile"),
    list: pick(one(raw.list), ["unread", "unshipped", "unpaid"], ""),
    nav: one(raw.nav).slice(0, 40),
    page: /^[1-9][0-9]{0,3}$/.test(page) ? Number(page) : 1,
    tstate: pick(one(raw.tstate), ["ready", "loading", "error", "empty"], "ready"),
  };
  const query = new URLSearchParams();
  for (const [k, v] of Object.entries(raw)) if (typeof v === "string") query.set(k, v);
  const search = query.toString();
  return <UiGallery locale={locale} initial={initial} search={search ? `?${search}` : ""} />;
}
