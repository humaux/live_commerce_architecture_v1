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
import { canOpen, matchRoute, visibleGroups } from "@/src/routes";
import { readWorkspace, logoutWorkspace } from "@/src/shell/api";
import { navAccessFrom } from "@/lib/team-model";
import type { Store } from "@/lib/model";
import { BillingBanner } from "./BillingBanner";
import { Icon } from "./Icon";

export function WorkspaceFrame({
  locale,
  storeName,
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
  const [error, setError] = useState(false),
    [retry, setRetry] = useState(0);
  const [open, setOpen] = useState(false),
    [expanded, setExpanded] = useState<string | null>(null);
  const [signingOut, setSigningOut] = useState(false),
    [signOutFailed, setSignOutFailed] = useState(false);
  const busy = useRef(false);
  const close = useCallback(() => setOpen(false), []);
  // Scope the result to its request key, even during the render before the next effect runs.
  const stores = data?.key === storeParam ? data.stores : [];
  const access = navAccessFrom({ items: stores }, storeParam);
  const route = matchRoute(pathname.replace(/^\/(zh-CN|zh-TW|en)/, "") || "/");
  const allowed = route && canOpen(route, access);
  const nav = visibleGroups(access);
  useEffect(() => {
    const abort = new AbortController();
    let current = true;
    setError(false);
    setData(null);
    readWorkspace(abort.signal)
      .then((stores) => {
        if (current) setData({ key: storeParam, stores });
      })
      .catch(() => {
        if (current && !abort.signal.aborted) setError(true);
      });
    return () => {
      current = false;
      abort.abort();
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
  const selectedStore =
    stores.find((store) => store.id === storeParam) ??
    [...stores].sort((a, b) => a.id.localeCompare(b.id))[0];
  return (
    <>
      <title>{route ? c[route.labelKey] : "Commerce workspace"}</title>
      <AppShell
        open={open}
        close={close}
        closeLabel={c.closeMenu}
        skipLabel={c.skip}
        rail={
          <>
            <div className={s.brand}>
              <Icon name="inventory" />
              <span>{c.navigation}</span>
            </div>
            <nav className={s.navigation} aria-label={c.navigation}>
              {nav.filter((g) => g.id !== "settings").map(groupView)}
            </nav>
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
            <label className={s.store}>
              <span className="sr-only">{c.store}</span>
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
                {stores.length ? (
                  stores.map((store) => (
                    <option value={store.id} key={store.id}>
                      {store.name}
                    </option>
                  ))
                ) : (
                  <option value="">{storeName}</option>
                )}
              </select>
            </label>
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
        {route && (
          <nav
            className={s.crumbs}
            data-shell-route={route.path}
            aria-label={c.navigation}
          >
            <a
              href={`/${locale}/`}
              onClick={(e) => {
                e.preventDefault();
                navigate("/");
              }}
            >
              {c.overview}
            </a>
            {route.id !== "dashboard" && (
              <>
                <span aria-hidden="true">/</span>
                <span>{c[route.group]}</span>
                <span aria-hidden="true">/</span>
                <span aria-current="page">{c[route.labelKey]}</span>
              </>
            )}
          </nav>
        )}
        {error ? (
          <div className={s.status} role="alert">
            <p>{c.unavailable}</p>
            <button onClick={() => setRetry((n) => n + 1)}>{c.retry}</button>
          </div>
        ) : !data || data.key !== storeParam ? (
          <p role="status">{c.loading}</p>
        ) : !allowed ? (
          <div className={s.status} data-testid="route-forbidden">
            <h1>{c.forbidden}</h1>
            <button onClick={() => navigate("/")}>{c.back}</button>
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
      </AppShell>
    </>
  );
}
