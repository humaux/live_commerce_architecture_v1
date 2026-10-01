"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  locales,
  localeNames,
  localizedPath,
  type Locale,
} from "@live-commerce/i18n";
import { copy } from "@/lib/copy";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import { signalLogout } from "@/lib/session-events";
import { customersCopy } from "@/lib/customers-copy";
import { designCopy } from "@/lib/design-copy";
import { teamCopy } from "@/lib/team-copy";
import { catalogCopy } from "@/lib/catalog-v2-copy";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { promotionsCopy } from "@/lib/promotions-copy";
import { navAccessFrom, navVisible, type NavAccess } from "@/lib/team-model";
import { BillingBanner } from "./BillingBanner";
import { Icon } from "./Icon";

export function WorkspaceFrame({
  locale,
  storeName,
  active,
  locked = false,
  onSection,
  onBeforeNavigate,
  children,
}: {
  locale: Locale;
  storeName: string;
  active: string;
  locked?: boolean;
  onSection?: (section: string) => void;
  onBeforeNavigate?: () => boolean;
  children: ReactNode;
}) {
  const c = copy[locale];
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const [navOpen, setNavOpen] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const [signOutFailed, setSignOutFailed] = useState(false);
  const signOutBusy = useRef(false);
  // Role-aware nav: the caller's role/permissions from GET /api/stores (-> Go /v1/admin/stores). Unknown = show everything.
  const [access, setAccess] = useState<NavAccess>(null);
  const storeParam = search.get("store");
  useEffect(() => {
    const abort = new AbortController();
    fetch("/api/stores", { credentials: "same-origin", cache: "no-store", signal: abort.signal })
      .then((response) => (response.ok ? response.json() : null))
      .then((body: unknown) => setAccess(navAccessFrom(body, storeParam)))
      .catch(() => undefined);
    return () => abort.abort();
  }, [storeParam]);
  // catalog-media: website-service / Meta-messages / platform-support entries removed: they led to a "not connected"
  // placeholder panel. Re-add an entry only together with a real page.
  // catalog-core: products and collections are real pages; the ledger (home) stays reachable as Inventory.
  // merchant-tools: the dashboard is the landing (/{locale}); the stock ledger is its own page (/{locale}/inventory).
  // Nav ids that are their own page under /[locale]/<id> (one entry per page; units append here).
  const pageRoutes = ["products", "collections", "customers", "finance", "billing", "design", "team", "promotions"];
  const nav = [
    ["dashboard", "dashboard", toolsCopy[locale].navDashboard],
    ["products", "product", c.products],
    ["collections", "product", catalogCopy[locale].nav.collections],
    ["inventory", "inventory", c.inventory],
    ["orders", "orders", c.orders],
    ["live", "live", c.live],
    ["customers", "support", customersCopy[locale].nav.customers],
    ["finance", "orders", customersCopy[locale].nav.finance],
    ["billing", "settings", customersCopy[locale].nav.billing],
    ["design", "product", designCopy[locale].title],
    ["ads", "meta", c.ads],
    ["promotions", "orders", promotionsCopy[locale].nav],
    ["team", "support", teamCopy[locale].nav],
    ["settings", "settings", c.settings],
  ];
  function select(id: string) {
    if (onBeforeNavigate && !onBeforeNavigate()) return;
    setNavOpen(false);
    const storeQuery = storeParam ? `?store=${encodeURIComponent(storeParam)}` : "";
    if (id === "dashboard") router.push(`/${locale}/${storeQuery}`);
    else if (id === "inventory" && active !== "inventory") router.push(`/${locale}/inventory${storeQuery}`);
    else if (id === "orders")
      router.push(
        `/${locale}/orders${search.get("store") ? `?store=${encodeURIComponent(search.get("store")!)}` : ""}`,
      );
    else if (id === "settings")
      router.push(
        `/${locale}/settings${search.get("store") ? `?store=${encodeURIComponent(search.get("store")!)}` : ""}`,
      );
    else if (id === "ads")
      router.push(
        `/${locale}/ads${search.get("store") ? `?store=${encodeURIComponent(search.get("store")!)}` : ""}`,
      );
    else if (id === "live")
      router.push(
        `/${locale}/studio${search.get("store") ? `?store=${encodeURIComponent(search.get("store")!)}` : ""}`,
      );
    else if (pageRoutes.includes(id))
      router.push(
        `/${locale}/${id}${search.get("store") ? `?store=${encodeURIComponent(search.get("store")!)}` : ""}`,
      );
    else if (["settings", "orders", "live", "ads", ...pageRoutes].includes(active))
      router.push(`/${locale}/`);
    else onSection?.(id);
  }
  async function signOut() {
    if (locked || signOutBusy.current) return;
    signOutBusy.current = true;
    setSigningOut(true);
    setSignOutFailed(false);
    signalLogout();
    try {
      const csrf = csrfCookie();
      if (!csrf || !(await sessionBoundary(csrf)) || csrfCookie() !== csrf)
        throw new Error("session_changed");
      const response = await fetch("/api/auth/logout", {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: "{}",
      });
      if (response.status !== 204 && response.status !== 401)
        throw new Error("logout_failed");
      signalLogout();
      window.location.replace(`/${locale}/`);
    } catch {
      setSignOutFailed(true);
      setSigningOut(false);
      signOutBusy.current = false;
    }
  }
  return (
    <div className="workspace">
      <a className="skip-link" href="#main">
        {c.skipToContent}
      </a>
      <aside className={`rail ${navOpen ? "open" : ""}`}>
        <div className="brand">{c.title}</div>
        <nav aria-label={c.title}>
          {nav.filter(([id]) => navVisible(id, access)).map(([id, icon, label]) => (
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
        {signOutFailed && (
          <p role="alert" style={{ padding: "0 20px", color: "#fff" }}>
            {c.signOutFailed}
          </p>
        )}
        <button
          type="button"
          className="nav-item"
          data-testid="workspace-sign-out"
          disabled={locked || signingOut}
          onClick={() => void signOut()}
        >
          {signingOut ? c.signingOut : c.signOut}
        </button>
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
              onChange={(event) => {
                if (onBeforeNavigate && !onBeforeNavigate()) return;
                router.push(
                  localizedPath(
                    event.target.value as Locale,
                    `${pathname}?${search}`,
                  ),
                );
              }}
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
          <BillingBanner locale={locale} storeId={search.get("store")} />
          {children}
        </main>
      </div>
    </div>
  );
}
