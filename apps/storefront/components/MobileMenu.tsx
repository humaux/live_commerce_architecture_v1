"use client";

// Phone navigation: a menu button that opens a native <dialog> sheet with the design document's header nav, the search box
// and the language links. No BFF/Go call. Links arrive resolved from the server (lib/design.ts resolveNav), so this file
// never builds a URL from merchant text. Closes itself on every navigation (usePathname) and on Esc/backdrop.
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { NavLink } from "../lib/design";
import { shopCopy } from "../lib/shop-copy";
import { CloseIcon, MenuIcon, SearchIcon } from "./icons";
import LocaleSwitch from "./LocaleSwitch";

export default function MobileMenu({ locale, links, preview }: { locale: Locale; links: NavLink[]; preview: string | null }) {
  const copy = shopCopy[locale];
  const ref = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState(false);
  const pathname = usePathname();
  useEffect(() => setOpen(false), [pathname]);
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);
  return (
    <>
      <button type="button" className="sf-iconbtn sf-menubtn" aria-label={copy.menu} aria-haspopup="dialog" onClick={() => setOpen(true)} data-testid="menu-open">
        <MenuIcon />
      </button>
      <dialog
        ref={ref}
        className="sf-sheet"
        aria-label={copy.mainNav}
        onClose={() => setOpen(false)}
        onClick={(event) => {
          if (event.target === event.currentTarget) setOpen(false);
        }}
      >
        <div className="sf-sheet__panel">
          <div className="sf-sheet__head">
            <button type="button" className="sf-iconbtn" aria-label={copy.closeMenu} onClick={() => setOpen(false)}>
              <CloseIcon />
            </button>
          </div>
          <form className="sf-search sf-search--sheet" role="search" action={`/${locale}/search`} method="get">
            <label className="sr-only" htmlFor="sf-sheet-q">
              {copy.search}
            </label>
            <input id="sf-sheet-q" name="q" type="search" placeholder={copy.searchPlaceholder} maxLength={60} autoComplete="off" enterKeyHint="search" />
            {preview && <input type="hidden" name="preview" value={preview} />}
            <button type="submit" aria-label={copy.searchSubmit}>
              <SearchIcon />
            </button>
          </form>
          <nav aria-label={copy.mainNav}>
            <ul className="sf-sheet__list">
              {links.map((link) => (
                <li key={`${link.href}:${link.label}`}>
                  {link.external ? (
                    <a href={link.href} target="_blank" rel="noopener noreferrer">
                      {link.label}
                    </a>
                  ) : link.href.startsWith("/") ? (
                    <Link href={link.href}>{link.label}</Link>
                  ) : (
                    <a href={link.href}>{link.label}</a>
                  )}
                </li>
              ))}
            </ul>
          </nav>
          <div className="sf-sheet__foot">
            <LocaleSwitch locale={locale} />
          </div>
        </div>
      </dialog>
    </>
  );
}
