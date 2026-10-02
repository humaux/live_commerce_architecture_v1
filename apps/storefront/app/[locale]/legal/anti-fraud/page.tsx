import { notFound } from "next/navigation";
import { isLocale } from "@live-commerce/i18n";
import { antiFraudCopy } from "../../../../lib/anti-fraud-copy";
import { browseCopy } from "../../../../lib/browse-copy";
import PageHead from "../../../../components/PageHead";
import { shopCopy } from "../../../../lib/shop-copy";

type Params = Promise<{ locale: string }>;
export async function generateMetadata({ params }: { params: Params }) {
  const { locale } = await params;
  return isLocale(locale) ? { title: browseCopy[locale].fraud } : {};
}
export default async function Page({ params }: { params: Params }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const copy = antiFraudCopy[locale];
  return <main className="sf-wrap sf-safety">
    <PageHead title={browseCopy[locale].fraud} label={shopCopy[locale].breadcrumb} crumbs={[{ href: `/${locale}`, label: shopCopy[locale].home }]} />
    <p className="sf-safety__intro">{copy.intro}</p>
    {copy.sections.map(section => <section key={section.heading}><h2>{section.heading}</h2><p>{section.text}</p></section>)}
    <a className="sf-safety__source" href="https://165.npa.gov.tw/" rel="noreferrer">{copy.source}</a>
  </main>;
}
