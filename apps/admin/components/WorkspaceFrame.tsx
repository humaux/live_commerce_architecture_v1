"use client";

import { useState, type ReactNode } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  locales,
  localeNames,
  localizedPath,
  type Locale,
} from "@live-commerce/i18n";
import { copy } from "@/lib/copy";
import { Icon } from "./Icon";

export function WorkspaceFrame({
  locale,
  storeName,
  active,
  locked = false,
  onSection,
  children,
}: {
  locale: Locale;
  storeName: string;
  active: string;
  locked?: boolean;
  onSection?: (section: string) => void;
  children: ReactNode;
}) {
  const c = copy[locale];
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const [navOpen, setNavOpen] = useState(false);
  const nav = [
    ["products", "product", c.products],
    ["inventory", "inventory", c.inventory],
    ["orders", "orders", c.orders],
    ["live", "live", c.live],
    ["siteChat", "chat", c.siteChat],
    ["meta", "meta", c.meta],
    ["support", "support", c.support],
    ["settings", "settings", c.settings],
  ];
  function select(id: string) {
    setNavOpen(false);
    if (id === "orders")
      router.push(
        `/${locale}/orders${search.get("store") ? `?store=${encodeURIComponent(search.get("store")!)}` : ""}`,
      );
    else if (id === "settings")
      router.push(
        `/${locale}/settings${search.get("store") ? `?store=${encodeURIComponent(search.get("store")!)}` : ""}`,
      );
    else if (active === "settings" || active === "orders") router.push(`/${locale}/`);
    else onSection?.(id);
  }
  return (
    <div className="workspace">
      <a className="skip-link" href="#main">
        {active === "settings" ? c.settings : active === "orders" ? c.orders : c.heading}
      </a>
      <aside className={`rail ${navOpen ? "open" : ""}`}>
        <div className="brand">{c.title}</div>
        <nav aria-label={c.title}>
          {nav.map(([id, icon, label]) => (
            <button
              key={id}
              type="button"
              data-testid={id === "orders" ? "nav-orders" : undefined}
              disabled={locked}
              className={active === id ? "nav-item active" : "nav-item"}
              aria-current={active === id ? "page" : undefined}
              onClick={() => select(id)}
            >
              <Icon name={icon} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
        <div className="channel-status">
          <h2>{c.channels}</h2>
          {[c.website, "Facebook", "Instagram", "WhatsApp", "LINE"].map(
            (name) => (
              <div key={name}>
                <span>{name}</span>
                <span className="disconnected">{c.notConnected}</span>
              </div>
            ),
          )}
        </div>
      </aside>
      <div className="work-area">
        <header className="topbar">
          <button
            type="button"
            className="mobile-menu icon-button"
            aria-label={c.menu}
            aria-expanded={navOpen}
            onClick={() => setNavOpen(!navOpen)}
          >
            <Icon name="menu" />
          </button>
          <div className="store-label">
            <Icon name="inventory" />
            <span>{storeName}</span>
          </div>
          <div className="top-spacer" />
          <label className="language">
            <span className="sr-only">{c.language}</span>
            <select
              data-testid="locale-switch"
              aria-label={c.language}
              value={locale}
              disabled={locked}
              onChange={(event) =>
                router.push(
                  localizedPath(
                    event.target.value as Locale,
                    `${pathname}?${search}`,
                  ),
                )
              }
            >
              {locales.map((lang) => (
                <option key={lang} value={lang}>
                  {localeNames[lang]}
                </option>
              ))}
            </select>
          </label>
          <div className="user-label">
            <span className="avatar">M</span>
            {c.user}
          </div>
        </header>
        <main id="main" className="main">
          {children}
        </main>
      </div>
    </div>
  );
}
