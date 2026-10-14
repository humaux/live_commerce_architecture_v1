// Purpose: Owns AppShellV2, the redesigned admin frame: dark sidebar with the nine NEEDS section 1 groups (collapsible second level), store card, topbar (locale, help, notifications, enter-my-store, account), breadcrumb slot and a mobile drawer below 768 px. Not switched on for any page yet.
// Depends on: react, ./shell2.module.css, ./logic (initialOpenGroups); no API, routing, permissions or translation: nav data, copy and slots are props.
// Used by: packages/ui/src/ds/index.ts; apps/admin/components/UiGallery.tsx (the W3 page migration will mount it).
"use client";
import { useCallback, useEffect, useId, useRef, useState, type ReactNode, type RefObject } from "react";
import s from "./shell2.module.css";
import { initialOpenGroups } from "./logic";

export type ShellNavItem = { id: string; label: string; href: string };
/** A group is either a link (`href`) or a collapsible parent with second-level `items` (no icons at level two). */
export type ShellNavGroup = { id: string; label: string; icon?: ReactNode; href?: string; items?: readonly ShellNavItem[] };

const DRAWER_QUERY = "(max-width: 767.98px)";

/** Mobile drawer behaviour: while open on a narrow screen, trap Tab, close on Escape, lock page scroll, make the rest inert, restore focus. */
function useDrawer(open: boolean, close: () => void, drawer: RefObject<HTMLElement | null>, rest: RefObject<HTMLElement | null>) {
  useEffect(() => {
    if (!open) return;
    const before = document.activeElement as HTMLElement | null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    rest.current?.setAttribute("inert", "");
    const controls = () =>
      Array.from(drawer.current?.querySelectorAll<HTMLElement>('a[href],button:not([aria-disabled="true"])') ?? []).filter((e) => e.getClientRects().length);
    controls()[0]?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close();
      }
      if (event.key !== "Tab") return;
      const list = controls();
      const first = list[0];
      const last = list.at(-1);
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    };
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      rest.current?.removeAttribute("inert");
      document.body.style.overflow = overflow;
      before?.focus();
    };
  }, [open, close, drawer, rest]);
}

export type AppShellV2Props = {
  nav: readonly ShellNavGroup[];
  /** id of the current group or second-level item. */
  currentId?: string;
  navLabel: string;
  brand: ReactNode;
  store: { name: string; region?: string; logo?: ReactNode };
  topbar: {
    locale: ReactNode;
    help: ReactNode;
    notifications: ReactNode;
    account: ReactNode;
    enterStore: { label: string; href: string };
  };
  breadcrumb?: ReactNode;
  children: ReactNode;
  menuLabel: string;
  closeLabel: string;
  skipLabel: string;
};

/** The v2 admin frame. Group parents toggle with aria-expanded; the group holding the current page starts open. */
export function AppShellV2({ nav, currentId, navLabel, brand, store, topbar, breadcrumb, children, menuLabel, closeLabel, skipLabel }: AppShellV2Props) {
  const [open, setOpen] = useState(false);
  const [expanded, setExpanded] = useState(() => initialOpenGroups(nav, currentId));
  const drawer = useRef<HTMLElement>(null);
  const rest = useRef<HTMLDivElement>(null);
  // Stable identity: useDrawer's effect depends on it, and a re-run would restore focus to the menu button and re-focus the first link.
  const close = useCallback(() => setOpen(false), []);
  const idBase = useId();
  useDrawer(open, close, drawer, rest);
  // A drawer left open while the window grows past 768 px would trap focus on a desktop sidebar.
  useEffect(() => {
    const query = window.matchMedia(DRAWER_QUERY);
    const widen = () => {
      if (!query.matches) setOpen(false);
    };
    query.addEventListener("change", widen);
    return () => query.removeEventListener("change", widen);
  }, []);
  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  return (
    <div className={s.shell} data-ds-shell="2">
      <a className={s.skip} href="#ds-main">
        {skipLabel}
      </a>
      {open && <button type="button" tabIndex={-1} className={s.backdrop} aria-label={closeLabel} onClick={close} />}
      <nav ref={drawer} id="ds-nav" className={s.nav} data-open={open} aria-label={navLabel} data-shell-rail>
        <button type="button" className={s.close} aria-label={closeLabel} onClick={close}>
          <span aria-hidden="true">×</span>
        </button>
        <div className={s.brand}>{brand}</div>
        <div className={s.store}>
          <span className={s.storeLogo} aria-hidden="true">{store.logo}</span>
          <span className={s.storeText}>
            <span className={s.storeName}>{store.name}</span>
            {store.region && <span className={s.storeRegion}>{store.region}</span>}
          </span>
        </div>
        <ul className={s.groups}>
          {nav.map((group) => {
            const isOpen = expanded.has(group.id);
            const holdsCurrent = group.id === currentId || Boolean(group.items?.some((i) => i.id === currentId));
            const icon = <span className={s.groupIcon} aria-hidden="true">{group.icon}</span>;
            if (!group.items?.length)
              return (
                <li key={group.id}>
                  <a className={s.groupLink} href={group.href ?? "#"} aria-current={group.id === currentId ? "page" : undefined}>
                    {icon}
                    <span className={s.groupLabel}>{group.label}</span>
                  </a>
                </li>
              );
            const subId = `${idBase}-${group.id}`;
            return (
              <li key={group.id}>
                <button type="button" className={s.groupRow} data-current={holdsCurrent} aria-expanded={isOpen} aria-controls={subId} onClick={() => toggle(group.id)}>
                  {icon}
                  <span className={s.groupLabel}>{group.label}</span>
                  <span className={s.chev} aria-hidden="true">▸</span>
                </button>
                {isOpen && (
                  <ul className={s.sub} id={subId}>
                    {group.items.map((item) => (
                      <li key={item.id}>
                        <a className={s.item} href={item.href} aria-current={item.id === currentId ? "page" : undefined} onClick={close}>
                          {item.label}
                        </a>
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            );
          })}
        </ul>
      </nav>
      <div className={s.work} ref={rest}>
        <header className={s.topbar} data-shell-topbar>
          <button type="button" className={s.menuButton} aria-label={menuLabel} aria-expanded={open} aria-controls="ds-nav" onClick={() => setOpen(true)}>
            <span aria-hidden="true">☰</span>
          </button>
          <div className={s.topbarRight}>
            {topbar.locale}
            {topbar.help}
            {topbar.notifications}
            <a className={s.enter} href={topbar.enterStore.href} target="_blank" rel="noopener noreferrer">
              {topbar.enterStore.label}
            </a>
            {topbar.account}
          </div>
        </header>
        {breadcrumb && <div className={s.crumbRow}>{breadcrumb}</div>}
        <main id="ds-main" className={s.main} tabIndex={-1}>
          {children}
        </main>
      </div>
    </div>
  );
}
