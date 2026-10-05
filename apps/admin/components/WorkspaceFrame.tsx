// Purpose: Owns admin workspace navigation, workspace loading and logout controls.
// Depends on: react, next/navigation, @live-commerce/i18n, @live-commerce/ui, @/src/shell-copy, @/src/page-title, @/src/routes, @/src/shell/api, @/lib/team-model, @/lib/model, ./BillingBanner, ./Icon, @/lib/company, ./OperatorFooter
// Used by: apps/admin/components/Customers.tsx, apps/admin/components/StudioClaims.tsx, apps/admin/components/MerchantOrders.tsx, apps/admin/components/Finance.tsx, apps/admin/components/Ledger.tsx, apps/admin/components/Billing.tsx, apps/admin/components/Promotions.tsx, apps/admin/components/CustomerDetail.tsx, apps/admin/components/SettingsWizard.tsx, apps/admin/components/Attribution.tsx, apps/admin/components/Studio.tsx, apps/admin/components/ProductImport.tsx, apps/admin/components/ManualOrder.tsx, apps/admin/components/Dashboard.tsx, apps/admin/components/Ads.tsx, apps/admin/components/Team.tsx, apps/admin/components/Design.tsx, apps/admin/components/ProductList.tsx, apps/admin/components/CollectionManager.tsx, apps/admin/components/ProductEditor.tsx, apps/admin/app/[locale]/orders/cvs-print/page.tsx
"use client";
// W0 shell only. API boundary: src/shell/api.ts; existing child page bodies are unchanged.
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import {
  locales,
  localeNames,
  localizedPath,
  type Locale,
} from "@live-commerce/i18n";
import { AppShell, shellStyles as s } from "@live-commerce/ui";
import { shellCopy } from "@/src/shell-copy";
import { pageTitle } from "@/src/page-title";
import { canOpen, matchRoute, visibleGroups } from "@/src/routes";
import { readWorkspace, logoutWorkspace } from "@/src/shell/api";
import { navAccessFrom } from "@/lib/team-model";
import type { Store } from "@/lib/model";
import { BillingBanner } from "./BillingBanner";
import { Icon } from "./Icon";
import { company } from "@/lib/company";
import { OperatorFooter } from "./OperatorFooter";

/** Owns admin workspace navigation, workspace loading and logout controls. Loads workspace context and submits logout through src/shell/api. */
export function WorkspaceFrame({
  locale,
  active,
  locked = false,
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
  const c = shellCopy[locale],
    router = useRouter(),
    pathname = usePathname(),
    search = useSearchParams();
  const storeParam = search.get("store");
  const [data, setData] = useState<{
    key: string | null;
    stores: Store[];
  } | null>(null);
  const [error, setError] = useState<"expired" | "unavailable" | null>(null),
    [retry, setRetry] = useState(0);
  const [open, setOpen] = useState(false),
    [expanded, setExpanded] = useState<string | null>(null);
  const [signingOut, setSigningOut] = useState(false),
    [signOutFailed, setSignOutFailed] = useState(false);
  const busy = useRef(false);
  const expired = useRef(false);
  const close = useCallback(() => setOpen(false), []);
  // Scope the result to its request key, even during the render before the next effect runs.
  const stores =
    !error && !signingOut && data?.key === storeParam ? data.stores : [];
  const access = navAccessFrom({ items: stores }, storeParam);
  const route = matchRoute(pathname.replace(/^\/(zh-CN|zh-TW|en)/, "") || "/");
  const allowed = route && canOpen(route, access);
  const nav = visibleGroups(access);
  // The first page this role may open, in registry order. A role without Overview (orders:read, e.g. marketing- or
  // catalog-only staff) lands there instead of a 403 whose way back pointed at the same 403.
  const home = nav[0]?.routes[0]?.path ?? "/";
  useEffect(() => {
    // URL changes must not revive this shell after a logout/401. Recovery uses
    // the existing full-page sign-in link and creates a new shell instance.
    if (expired.current) return;
    const abort = new AbortController();
    let current = true;
    const clearSession = () => {
      expired.current = true;
      current = false;
      abort.abort();
      setData(null);
      setError("expired");
      setOpen(false);
      setExpanded(null);
      const url = new URL(window.location.href);
      if (url.searchParams.has("store")) {
        url.searchParams.delete("store");
        window.history.replaceState(
          null,
          "",
          `${url.pathname}${url.search}${url.hash}`,
        );
      }
    };
    const onStorage = (event: StorageEvent) => {
      if (event.key === "commerce-session-logout") clearSession();
    };
    let channel: BroadcastChannel | null = null;
    try {
      channel = new BroadcastChannel("commerce-session");
      channel.onmessage = (event: MessageEvent) => {
        if (event.data?.type === "logout") clearSession();
      };
    } catch {
      // Same-tab and storage notifications remain available.
    }
    window.addEventListener("commerce-session-logout", clearSession);
    window.addEventListener("storage", onStorage);
    setError(null);
    setData(null);
    const refresh = () =>
      readWorkspace(abort.signal)
        .then((stores) => {
          if (current) {
            setData({ key: storeParam, stores });
            setError(null);
          }
        })
        .catch((cause: unknown) => {
          if (current && !abort.signal.aborted) {
            if (
              cause instanceof Error &&
              cause.message === "workspace_session_expired"
            )
              clearSession();
            else setError("unavailable");
          }
        });
    const onFocus = () => {
      if (current) void refresh();
    };
    void refresh();
    window.addEventListener("focus", onFocus);
    return () => {
      current = false;
      abort.abort();
      window.removeEventListener("commerce-session-logout", clearSession);
      window.removeEventListener("storage", onStorage);
      window.removeEventListener("focus", onFocus);
      channel?.close();
    };
  }, [storeParam, retry]);
  useEffect(() => {
    close();
    const media = matchMedia("(min-width: 1024px)");
    media.addEventListener("change", close);
    return () => media.removeEventListener("change", close);
  }, [pathname, close]);
  const before = () => !locked && (!onBeforeNavigate || onBeforeNavigate());
  function navigate(path: string) {
    if (!before()) return;
    close();
    router.push(
      `/${locale}${path === "/" ? "" : path}${storeParam ? `?store=${encodeURIComponent(storeParam)}` : ""}`,
    );
  }
  useEffect(() => {
    if (
      data &&
      data.key === storeParam &&
      route?.path === "/" &&
      !allowed &&
      home !== "/"
    )
      router.replace(
        `/${locale}${home}${storeParam ? `?store=${encodeURIComponent(storeParam)}` : ""}`,
      );
  }, [data, storeParam, route, allowed, home, locale, router]);
  async function signOut() {
    if (locked || busy.current) return;
    busy.current = true;
    setSigningOut(true);
    setSignOutFailed(false);
    try {
      await logoutWorkspace();
      window.location.replace(`/${locale}/`);
    } catch {
      setSignOutFailed(true);
      setSigningOut(false);
      busy.current = false;
    }
  }
  function groupView(group: (typeof nav)[number]) {
    const selected = route?.group === group.id;
    const reveal = expanded === group.id || (expanded === null && selected);
    const singleton = group.routes.length === 1;
    return (
      <div key={group.id}>
        <button
          type="button"
          className={s.navButton}
          disabled={locked}
          data-active={selected}
          data-testid={
            singleton && group.routes[0].id === "orders"
              ? "nav-orders"
              : `nav-group-${group.id}`
          }
          aria-current={singleton && selected ? "page" : undefined}
          aria-expanded={singleton ? undefined : reveal}
          onClick={() =>
            singleton
              ? navigate(group.routes[0].path)
              : setExpanded(reveal ? "" : group.id)
          }
        >
          <Icon name={group.icon} />
          <span>{c[group.id]}</span>
          {!singleton && (
            <span className={s.chevron} aria-hidden="true">
              {reveal ? "−" : "+"}
            </span>
          )}
        </button>
        {!singleton &&
          reveal &&
          group.routes.map((item) => (
            <button
              type="button"
              key={item.id}
              className={s.subLink}
              disabled={locked}
              data-testid={`nav-${item.id}`}
              aria-current={route?.id === item.id ? "page" : undefined}
              onClick={() => navigate(item.path)}
            >
              {c[item.labelKey]}
            </button>
          ))}
      </div>
    );
  }
  const selectedStore = storeParam
    ? stores.find((store) => store.id === storeParam)
    : [...stores].sort((a, b) => a.id.localeCompare(b.id))[0];
  return (
    <>
      <title>
        {error === "expired" || signingOut
          ? c.signIn
          : route
            ? pageTitle(locale, pathname)
            : "Commerce workspace"}
      </title>
      <AppShell
        open={open}
        close={close}
        closeLabel={c.closeMenu}
        skipLabel={c.skip}
        rail={
          <>
            <div
              className="platform-shell-brand"
              data-testid="shell-platform-brand"
            >
              {company.productName}
            </div>
            {nav.length > 0 && (
              <nav className={s.navigation} aria-label={c.navigation}>
                {nav.filter((g) => g.id !== "settings").map(groupView)}
              </nav>
            )}
            {nav.some((g) => g.id === "settings") && (
              <nav className={s.bottom} aria-label={c.settings}>
                {nav.filter((g) => g.id === "settings").map(groupView)}
              </nav>
            )}
          </>
        }
        topbar={
          <>
            <button
              type="button"
              className={s.menu}
              aria-label={c.openMenu}
              aria-expanded={open}
              aria-controls="workspace-navigation"
              onClick={() => setOpen(true)}
            >
              <Icon name="menu" />
            </button>
            {selectedStore && (
              <label className={s.store}>
                <span className="sr-only">{c.store}</span>
                <span className={s.brand} data-testid="shell-store-brand" aria-hidden="true">
                  <span className={s.brandMark}>{Array.from(selectedStore.name.trim())[0]?.toLocaleUpperCase(locale)}</span>
                  <span className={s.brandName} data-testid="shell-store-name" title={selectedStore.name}>{selectedStore.name}</span>
                  <span className={s.storeChevron}>⌄</span>
                </span>
                <select
                  aria-label={c.store}
                  data-testid="shell-store-selector"
                  value={selectedStore?.id ?? ""}
                  disabled={locked || !stores.length}
                  onChange={(e) => {
                    if (before())
                      window.location.assign(
                        `/${locale}/?store=${encodeURIComponent(e.target.value)}`,
                      );
                  }}
                >
                  {stores.map((store) => (
                    <option value={store.id} key={store.id}>
                      {store.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <label>
              <span className="sr-only">{c.language}</span>
              <select
                aria-label={c.language}
                data-testid="locale-switch"
                value={locale}
                disabled={locked}
                onChange={(e) => {
                  if (before())
                    router.push(
                      localizedPath(
                        e.target.value as Locale,
                        error === "expired" || signingOut
                          ? "/"
                          : `${pathname}?${search}`,
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
            <details className={s.popover}>
              <summary>{c.help}</summary>
              <div className={s.panel}>
                <p>{c.helpText}</p>
              </div>
            </details>
            <details className={s.popover}>
              <summary>{c.account}</summary>
              <div className={s.panel}>
                {signOutFailed && <p role="alert">{c.signOutFailed}</p>}
                <button
                  type="button"
                  data-testid="workspace-sign-out"
                  disabled={locked || signingOut}
                  onClick={() => void signOut()}
                >
                  {signingOut ? c.signingOut : c.signOut}
                </button>
              </div>
            </details>
          </>
        }
      >
        {route && selectedStore && (
          <nav
            className={s.crumbs}
            data-shell-route={route.path}
            aria-label={c.navigation}
          >
            {route.id !== "dashboard" && (
              <>
                <a
                  href={`/${locale}/`}
                  onClick={(e) => {
                    e.preventDefault();
                    navigate("/");
                  }}
                >
                  {c.overview}
                </a>
                {c[route.group] !== pageTitle(locale, pathname) && (
                  <>
                    <span aria-hidden="true">/</span>
                    <span>{c[route.group]}</span>
                  </>
                )}
                <span aria-hidden="true">/</span>
                <span aria-current="page">{pageTitle(locale, pathname)}</span>
              </>
            )}
          </nav>
        )}
        {error ? (
          <div
            className={s.status}
            role="alert"
            data-testid={
              error === "expired" ? "shell-session-expired" : undefined
            }
          >
            <p>{error === "expired" ? c.sessionExpired : c.unavailable}</p>
            {error === "expired" ? (
              <a data-testid="shell-sign-in" href={localizedPath(locale, "/")}>
                {c.signIn}
              </a>
            ) : (
              <button onClick={() => setRetry((n) => n + 1)}>{c.retry}</button>
            )}
          </div>
        ) : !data || data.key !== storeParam ? (
          <p role="status">{c.loading}</p>
        ) : !allowed ? (
          <div className={s.status} data-testid="route-forbidden">
            <h1>{c.forbidden}</h1>
            <button
              data-testid="route-forbidden-home"
              onClick={() => navigate(home)}
            >
              {c.back}
            </button>
          </div>
        ) : (
          <>
            <BillingBanner
              locale={locale}
              storeId={selectedStore?.id ?? null}
            />
            {children}
          </>
        )}
        <OperatorFooter locale={locale} />
      </AppShell>
    </>
  );
}
