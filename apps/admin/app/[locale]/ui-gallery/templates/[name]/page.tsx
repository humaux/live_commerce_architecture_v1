// Purpose: Four developer-only template routes; fail closed before parsing any gallery data.
// Depends on: Next notFound, Locale contract and UiGalleryTemplates; no session, API or real store data.
// Used by: integrator CI screenshots and click matrix. Invariant: LC_UI_GALLERY must equal "1".
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { UiGalleryTemplate } from "@/components/UiGalleryTemplates";
import { templateNames, type TemplateName } from "@/src/ui-gallery-template-copy";
export const metadata: Metadata = { title: "Design system templates", robots: { index: false, follow: false } };
type Raw = Record<string, string | string[] | undefined>;
const pick = (raw: Raw, key: string, allowed: readonly string[], fallback: string) => {
  const value = raw[key];
  return typeof value === "string" && allowed.includes(value) ? value : fallback;
};
export default async function TemplatePage({ params, searchParams }: { params: Promise<{ locale: string; name: string }>; searchParams: Promise<Raw> }) {
  if (process.env.LC_UI_GALLERY !== "1") notFound();
  const { locale, name } = await params;
  if (!isLocale(locale) || !templateNames.includes(name as TemplateName)) notFound();
  const raw = await searchParams;
  const page = typeof raw.page === "string" && /^[1-9]\d{0,2}$/.test(raw.page) ? Number(raw.page) : 1;
  return <UiGalleryTemplate locale={locale} name={name as TemplateName} initial={{
    payment: pick(raw, "payment", ["all", "paid", "unpaid"], "all"), state: pick(raw, "state", ["ready", "loading", "error", "empty"], "ready"), page,
    section: pick(raw, "section", ["general", "media", "pricing"], "general"), setting: pick(raw, "setting", ["profile", "notices"], "profile"),
    nav: typeof raw.nav === "string" && /^[a-z-]{1,40}$/.test(raw.nav) ? raw.nav : "",
  }} />;
}
