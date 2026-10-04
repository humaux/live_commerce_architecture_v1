// Shell chrome rendered from the store design document (contracts/storefront-v2.md section B): announcement bar, header
// (logo/name, nav.header, search, cart) and footer (nav.footer, contact block with LINE/Facebook/Instagram, legal links).
// Server components; the only client islands are the cart button (CartDrawer.tsx), the phone menu and the locale links.
// BFF/Go: none here (the document was read once by app/[locale]/layout.tsx via lib/shop-upstream.ts loadShop).
// Merchant text is rendered only as React text nodes; outbound links come pre-filtered by lib/design.ts.
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { resolveNav, safeExternal } from "../lib/design";
import type { Design, NavLink } from "../lib/design";
import { legalFooterLinks } from "../lib/legal-copy";
import { storeImage } from "../lib/routes";
import { shopCopy } from "../lib/shop-copy";
import { HeaderCart } from "./CartDrawer";
import { ChatIcon, FacebookIcon, InstagramIcon, MailIcon, PhoneIcon, PinIcon, SearchIcon } from "./icons";
import LocaleSwitch from "./LocaleSwitch";
import MobileMenu from "./MobileMenu";

function NavAnchor({ link, className }: { link: NavLink; className?: string }) {
  if (link.external)
    return (
      <a className={className} href={link.href} target="_blank" rel="noopener noreferrer">
        {link.label}
      </a>
    );
  return link.href.startsWith("/") ? (
    <Link className={className} href={link.href}>
      {link.label}
    </Link>
  ) : (
    <a className={className} href={link.href}>
      {link.label}
    </a>
  );
}

export function PreviewBanner({ locale }: { locale: Locale }) {
  const copy = shopCopy[locale];
  return (
    <div className="sf-preview" role="status" data-testid="preview-banner">
      <span>{copy.previewBanner}</span>
      <a href={`/${locale}`}>{copy.previewExit}</a>
    </div>
  );
}

export function Announcement({ text }: { text: string }) {
  return (
    <p className="sf-announce" data-testid="announcement">
      {text}
    </p>
  );
}

// `closed`: the host has no published store, so the cart and search would lead nowhere; only the brand, the language links and the legal
// pages stay.
export function ShopHeader({ locale, design, preview, closed = false }: { locale: Locale; design: Design; preview: string | null; closed?: boolean }) {
  const copy = shopCopy[locale];
  const { profile, nav } = design;
  const links = resolveNav(locale, nav.header, preview);
  const home = `/${locale}${preview ? `?preview=${encodeURIComponent(preview)}` : ""}`;
  return (
    <header className="sf-header">
      <div className="sf-header__row">
        <MobileMenu locale={locale} links={links} preview={preview} searchable={!closed} />
        <Link className="sf-brand" href={home} aria-label={profile.name}>
          {profile.logo_image_id ? <img src={storeImage(profile.logo_image_id)} alt={profile.name} height={34} /> : <span>{profile.name}</span>}
        </Link>
        <nav className="sf-nav" aria-label={copy.mainNav}>
          <ul>
            {links.map((link) => (
              <li key={`${link.href}:${link.label}`}>
                <NavAnchor link={link} />
              </li>
            ))}
          </ul>
        </nav>
        <div className="sf-header__tools">
          {!closed && (
            <>
              <form className="sf-search sf-search--inline" role="search" action={`/${locale}/search`} method="get">
                <label className="sr-only" htmlFor="sf-q">
                  {copy.search}
                </label>
                <input id="sf-q" name="q" type="search" placeholder={copy.searchPlaceholder} maxLength={60} autoComplete="off" enterKeyHint="search" />
                {preview && <input type="hidden" name="preview" value={preview} />}
                <button type="submit" aria-label={copy.searchSubmit}>
                  <SearchIcon />
                </button>
              </form>
              <Link className="sf-iconbtn sf-searchlink" href={withSearch(locale, preview)} aria-label={copy.search}>
                <SearchIcon />
              </Link>
              <HeaderCart locale={locale} />
            </>
          )}
        </div>
      </div>
    </header>
  );
}

const withSearch = (locale: Locale, preview: string | null) => `/${locale}/search${preview ? `?preview=${encodeURIComponent(preview)}` : ""}`;

export function ShopFooter({ locale, design, preview }: { locale: Locale; design: Design; preview: string | null }) {
  const copy = shopCopy[locale];
  const { profile, nav } = design;
  const links = resolveNav(locale, nav.footer, preview);
  const c = profile.contact;
  const line = safeExternal(c.line_url);
  const hasContact = !!(c.email || c.phone || c.address || line || c.facebook_url || c.instagram_url);
  const telHref = c.phone ? `tel:${c.phone.replace(/[^\d+]/g, "")}` : null;
  return (
    <footer className="sf-footer">
      <div className="sf-footer__grid">
        <section className="sf-footer__brand" aria-label={profile.name}>
          <p className="sf-footer__name">{profile.name}</p>
          {profile.tagline && <p className="sf-footer__tag">{profile.tagline}</p>}
          {hasContact && (
            <ul className="sf-contact" aria-label={copy.contact}>
              {c.phone && telHref && (
                <li>
                  <PhoneIcon />
                  <a href={telHref}>{c.phone}</a>
                </li>
              )}
              {c.email && (
                <li>
                  <MailIcon />
                  <a href={`mailto:${c.email}`}>{c.email}</a>
                </li>
              )}
              {c.address && (
                <li>
                  <PinIcon />
                  <span>{c.address}</span>
                </li>
              )}
              {line && (
                <li>
                  <ChatIcon />
                  <a href={line} target={line.startsWith("https://") ? "_blank" : undefined} rel="noopener noreferrer">
                    LINE
                  </a>
                </li>
              )}
              {c.facebook_url && (
                <li>
                  <FacebookIcon />
                  <a href={c.facebook_url} target="_blank" rel="noopener noreferrer">
                    Facebook
                  </a>
                </li>
              )}
              {c.instagram_url && (
                <li>
                  <InstagramIcon />
                  <a href={c.instagram_url} target="_blank" rel="noopener noreferrer">
                    Instagram
                  </a>
                </li>
              )}
            </ul>
          )}
        </section>
        {links.length > 0 && (
          <nav className="sf-footer__nav" aria-label={copy.footerNav}>
            <ul>
              {links.map((link) => (
                <li key={`${link.href}:${link.label}`}>
                  <NavAnchor link={link} />
                </li>
              ))}
            </ul>
          </nav>
        )}
        <nav className="sf-footer__legal" aria-label="Legal">
          <ul>
            {legalFooterLinks(locale).map((l) => (
              <li key={l.href}>
                <a href={l.href}>{l.label}</a>
              </li>
            ))}
          </ul>
        </nav>
      </div>
      <div className="sf-footer__base">
        <LocaleSwitch locale={locale} />
        <p>© {new Date().getFullYear()} {profile.name}</p>
      </div>
    </footer>
  );
}
