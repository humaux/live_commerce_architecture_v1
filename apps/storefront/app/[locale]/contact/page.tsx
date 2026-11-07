// Purpose: serve the buyer contact task on its dedicated locale route.
// Depends on: published-store gate and ContactForm; does not expose buyer data in the URL.
import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { gate } from "../../../lib/shop-page";
import ContactForm from "../../../components/contact/ContactForm";
export const metadata = { robots: { index: false, follow: false } };
export default async function ContactPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const { preview } = await gate();
  return <ContactForm locale={locale} preview={preview} />;
}
