import {
  company,
  companyConfig,
  operatorSentence,
  platformPath,
  type PlatformLocale,
  type PlatformPage,
} from "../../lib/company";
import { platformCopy } from "../../lib/platform-copy";
import { platformLegal } from "../../lib/platform-legal";

export function CompanyFacts({ locale }: { locale: PlatformLocale }) {
  const c = platformCopy[locale],
    { email } = companyConfig();
  return (
    <section
      className="ps-company"
      aria-label={c.company}
      data-testid="company-facts"
    >
      <h2>{c.company}</h2>
      <p className="ps-company-name">
        {company.legalEnglish}
        <br />
        {company.legalChinese}
      </p>
      <address>{company.address}</address>
      <dl>
        <div>
          <dt>{c.cr}</dt>
          <dd>{company.registrationNumber}</dd>
        </div>
        <div>
          <dt>{c.br}</dt>
          <dd>{company.businessRegistrationNumber}</dd>
        </div>
        <div>
          <dt>{c.incorporated}</dt>
          <dd>{company.incorporatedOn}</dd>
        </div>
        <div>
          <dt>{c.validity}</dt>
          <dd>{company.businessRegistrationValidity}</dd>
        </div>
        <div>
          <dt>{c.email}</dt>
          <dd>
            <a href={`mailto:${email}`}>{email}</a>
          </dd>
        </div>
      </dl>
    </section>
  );
}
export function PlatformFooter({ locale }: { locale: PlatformLocale }) {
  const c = platformCopy[locale];
  return (
    <footer className="ps-footer">
      <div className="ps-footer-inner">
        <p className="ps-operated">{operatorSentence(locale)}</p>
        <nav aria-label={c.company}>
          {(["about", "privacy", "terms", "data-deletion", "contact"] as const).map(
            (p) => (
              <a key={p} href={platformPath(locale, p)}>
                {c.pages[p]}
              </a>
            ),
          )}
        </nav>
        <CompanyFacts locale={locale} />
      </div>
    </footer>
  );
}
export function PlatformAbout({ locale }: { locale: PlatformLocale }) {
  const c = platformCopy[locale];
  return (
    <article data-testid="about-page">
      <a className="ps-back" href={platformPath(locale)}>
        ← {c.pages.home}
      </a>
      <h1>
        {company.legalEnglish}
        <br />
        {company.legalChinese}
      </h1>
      <p>{operatorSentence(locale)}</p>
      {c.about.map((text) => (
        <p key={text}>{text}</p>
      ))}
      <CompanyFacts locale={locale} />
      <nav aria-label={c.pages.about} data-testid="about-links">
        {(["terms", "privacy", "data-deletion", "contact"] as const).map(
          (p) => (
            <a key={p} href={platformPath(locale, p)}>
              {c.pages[p]}
            </a>
          ),
        )}
      </nav>
    </article>
  );
}
export function PlatformDocument({
  locale,
  page,
}: {
  locale: PlatformLocale;
  page: Exclude<PlatformPage, "home" | "about">;
}) {
  const c = platformCopy[locale];
  return (
    <article>
      <a className="ps-back" href={platformPath(locale)}>
        ← {c.pages.home}
      </a>
      <h1>{c.pages[page]}</h1>
      {platformLegal[locale][page].map(([title, text]) => (
        <section className="ps-legal-section" key={title}>
          <h2>{title}</h2>
          <p>{text}</p>
        </section>
      ))}
      <CompanyFacts locale={locale} />
    </article>
  );
}
