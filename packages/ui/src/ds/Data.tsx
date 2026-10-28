// Purpose: Owns the DS-2 list pattern: DataTable (selection with page/all-results scope, thumbnail cell, row actions with one primary at most, empty/loading/error rows), MenuButton, FilterBar controls and URL-friendly Pagination.
// Depends on: React, shared Icon/Controls, ./data.module.css, ./Controls (Button), ./logic (selection, row actions, page window); no API, routing or translation.
// Used by: packages/ui/src/ds/index.ts; apps/admin/components/UiGallery.tsx.
"use client";
import { useEffect, useId, useRef, useState, type CSSProperties, type FormEvent, type MouseEvent, type ReactNode, type RefObject } from "react";
import { Icon, type IconName } from "./Icon";
import s from "./data.module.css";
import { Button, ButtonLink } from "./Controls";
import {
  menuItemKind,
  offerAllResults,
  pageSelection,
  pageWindow,
  resolveRowActions,
  selectAllResults,
  selectionCount,
  toggleRow,
  togglePage,
  clearSelection,
  type ActionVariant,
  type SelectionState,
} from "./logic";

/* ------------------------------- menu ------------------------------- */

/** An item either runs `onSelect` or, with `href`, navigates (rendered as a link, not a button). */
export type MenuItem = { id: string; label: string; variant?: "danger"; disabledReason?: string } & (
  | { onSelect: () => void; href?: undefined }
  | { href: string; target?: "_blank"; onSelect?: () => void }
);

/** A disclosure menu (aria-expanded, not role=menu): Escape and an outside click close it, focus returns to the trigger. */
export function MenuButton({
  label,
  items,
  align = "end",
  size = "md",
  disabledReason,
  icon,
  variant = "secondary",
  caret = true,
}: {
  label: ReactNode;
  items: readonly MenuItem[];
  align?: "start" | "end";
  size?: "sm" | "md" | "lg";
  disabledReason?: string;
  /** Icon-only disclosure: no extra caret; label remains available to assistive technology. */
  icon?: IconName;
  variant?: "secondary" | "ghost";
  /** false drops the chevron, as on the Figma Language switch (1397:105152); aria-expanded still announces the disclosure. */
  caret?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const wrap = useRef<HTMLSpanElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const pendingFocus = useRef<"first" | "last" | null>(null);
  useEffect(() => {
    if (!open || !pendingFocus.current) return;
    const items = list.current?.querySelectorAll<HTMLElement>("a[href],button");
    (pendingFocus.current === "last" ? items?.[items.length - 1] : items?.[0])?.focus();
    pendingFocus.current = null;
  }, [open]);
  const [note, setNote] = useState<string>();
  const [place, setPlace] = useState<CSSProperties>();
  const id = useId();
  // The list is position: fixed under the trigger so a table's scroll frame (overflow) never clips it; it follows the trigger on scroll and resize.
  const reposition = () => {
    const box = trigger.current?.getBoundingClientRect();
    if (!box) return;
    const height = list.current?.offsetHeight ?? Math.min(window.innerHeight * 0.6, items.length * 44 + 8);
    const width = list.current?.offsetWidth ?? 220;
    const up = window.innerHeight - box.bottom < height + 8 && box.top > height;
    const left = Math.max(16, Math.min(align === "end" ? box.right - width : box.left, window.innerWidth - width - 16));
    setPlace({ position: "fixed", left, right: "auto", ...(up ? { top: "auto", bottom: window.innerHeight - box.top + 4 } : { top: box.bottom + 4, bottom: "auto" }) });
  };
  useEffect(() => {
    if (!open) return;
    reposition();
    const away = (event: Event) => {
      if (!wrap.current?.contains(event.target as Node)) setOpen(false);
    };
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        trigger.current?.focus();
      }
    };
    document.addEventListener("pointerdown", away);
    document.addEventListener("keydown", key);
    window.addEventListener("scroll", reposition, true);
    window.addEventListener("resize", reposition);
    return () => {
      document.removeEventListener("pointerdown", away);
      document.removeEventListener("keydown", key);
      window.removeEventListener("scroll", reposition, true);
      window.removeEventListener("resize", reposition);
    };
    // reposition only reads refs and align
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, align, items.length]);
  return (
    <span className={s.menuWrap} ref={wrap} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(false); }}>
      <Button
        ref={trigger}
        size={size}
        variant={variant}
        iconOnly={Boolean(icon)}
        aria-label={icon && typeof label === "string" ? label : undefined}
        aria-haspopup="true"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        disabledReason={disabledReason}
        onClick={() => {
          if (!open) reposition();
          setOpen(!open);
        }}
        onKeyDown={(event) => {
          if (!["ArrowDown", "ArrowUp"].includes(event.key) || disabledReason) return;
          event.preventDefault();
          pendingFocus.current = event.key === "ArrowUp" ? "last" : "first";
          reposition();
          setOpen(true);
          if (open) {
            const nodes = list.current?.querySelectorAll<HTMLElement>("a[href],button");
            (event.key === "ArrowUp" ? nodes?.[nodes.length - 1] : nodes?.[0])?.focus();
          }
        }}
        data-menu-trigger=""
      >
        {icon ? <><Icon name={icon} size={16} /><span className={s.sr}>{label}</span></> : <>{label}{caret && <Icon name="chevronDown" size={16} />}</>}
      </Button>
      {open && (
        <div ref={list} className={s.menu} id={id} data-align={align} role="group" style={place} onKeyDown={(event) => {
          const keys = ["ArrowDown", "ArrowUp", "Home", "End"];
          if (!keys.includes(event.key)) return;
          event.preventDefault();
          const nodes = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("a[href],button"));
          const index = nodes.indexOf(document.activeElement as HTMLElement);
          const next = event.key === "Home" ? 0 : event.key === "End" ? nodes.length - 1 : (index + (event.key === "ArrowDown" ? 1 : -1) + nodes.length) % nodes.length;
          nodes[next]?.focus();
        }}>
          {items.map((item) => (
            <div key={item.id}>
              {menuItemKind(item) === "link" && item.href !== undefined ? (
                <a
                  className={s.menuItem}
                  data-variant={item.variant}
                  href={item.href}
                  target={item.target}
                  rel={item.target === "_blank" ? "noopener noreferrer" : undefined}
                  onClick={() => {
                    setOpen(false);
                    item.onSelect?.();
                  }}
                >
                  {item.label}
                </a>
              ) : (
                <button
                  type="button"
                  className={s.menuItem}
                  data-variant={item.variant}
                  aria-disabled={item.disabledReason ? true : undefined}
                  onClick={() => {
                    if (item.disabledReason) return setNote(item.disabledReason);
                    setOpen(false);
                    trigger.current?.focus();
                    item.onSelect?.();
                  }}
                >
                  {item.label}
                </button>
              )}
              {note !== undefined && item.disabledReason === note && <div className={s.menuReason} role="status">{note}</div>}
            </div>
          ))}
        </div>
      )}
    </span>
  );
}

/* ------------------------------ table ------------------------------ */

export type Column<R> = { id: string; header: ReactNode; cell: (row: R) => ReactNode; align?: "end"; width?: string; /** free-text column: may wrap (long names) while every other column stays on one line */ wrap?: boolean };
export type RowAction = { id: string; label: string; variant?: ActionVariant; onSelect?: () => void; href?: string; more?: boolean };

/** Product/media thumbnail cell. `alt` is required (use the product name); no image shows a dashed placeholder with `emptyLabel`. */
export function ThumbCell({ src, alt, emptyLabel }: { src?: string; alt: string; emptyLabel: string }) {
  // eslint-disable-next-line @next/next/no-img-element -- admin thumbnails are same-origin or data: URLs, no optimiser
  return src ? <img className={s.thumb} src={src} alt={alt} width={64} height={64} loading="lazy" /> : <span className={s.thumbEmpty}>{emptyLabel}</span>;
}

export type SelectionLabels = {
  selectPage: string;
  selectRow: (row: number) => string;
  selected: (n: number) => string;
  pageOnly: string;
  allResults: (n: number) => string;
  selectAllResults: (n: number) => string;
  clear: string;
};

function CheckBox({ checked, mixed, label, onChange, focusRef }: { checked: boolean; mixed?: boolean; label: string; onChange: () => void; focusRef?: RefObject<HTMLInputElement | null> }) {
  return (
    <input
      type="checkbox"
      className={s.check}
      aria-label={label}
      checked={checked}
      ref={(node) => {
        if (node) node.indeterminate = Boolean(mixed);
        if (focusRef) focusRef.current = node;
      }}
      onChange={onChange}
    />
  );
}

/**
 * The list table. Selection is controlled by the caller (`state`/`onChange` from ./logic): the header checkbox picks the page,
 * and when more results exist off-page the banner offers "all N results" (scope "all", which callers must confirm before a bulk command).
 * Each row may carry at most one primary action (throws in development, demotes in production). States are rows, so the header never moves.
 */
export function DataTable<R>({
  caption,
  columns,
  rows,
  rowId,
  status = "ready",
  empty,
  error,
  loadingLabel,
  selection,
  actions,
  actionsHeader,
  moreLabel,
  scrollHint,
}: {
  caption: string;
  columns: readonly Column<R>[];
  rows: readonly R[];
  rowId: (row: R) => string;
  status?: "ready" | "loading" | "error";
  empty: ReactNode;
  error: ReactNode;
  loadingLabel: string;
  /** `filterKey` identifies the current filter/search; "all results" stays bound to it (see bulkTarget). */
  selection?: { state: SelectionState; onChange: (next: SelectionState) => void; total: number; filterKey: string; labels: SelectionLabels; actions?: ReactNode };
  actions?: (row: R) => readonly RowAction[];
  actionsHeader?: string;
  moreLabel?: string;
  /** Localized mobile affordance; horizontal scrolling is the single mobile table rule. */
  scrollHint?: string;
}) {
  const hintId = useId();
  const ids = rows.map(rowId);
  const span = columns.length + (selection ? 1 : 0) + (actions ? 1 : 0);
  const strict = process.env.NODE_ENV !== "production";
  const count = selection ? selectionCount(selection.state, selection.total, ids) : 0;
  // When the selection bar closes (bulk command applied, Clear) the control that had focus unmounts with it and focus falls to
  // <body>; land it on the stable page checkbox instead (#87). After a tick, so a closing dialog restores first; never steals focus.
  const pageCheck = useRef<HTMLInputElement>(null);
  const hadBar = useRef(false);
  useEffect(() => {
    const closed = hadBar.current && count === 0;
    hadBar.current = count > 0;
    if (!closed) return;
    const timer = setTimeout(() => { if (!document.activeElement || document.activeElement === document.body) pageCheck.current?.focus(); });
    return () => clearTimeout(timer);
  }, [count]);
  const renderActions = (row: R) => {
    const list = resolveRowActions(actions?.(row) ?? [], strict);
    const visible = list.filter((a) => !a.more);
    const more = list.filter((a) => a.more);
    const one = (a: RowAction) =>
      a.href ? (
        <ButtonLink key={a.id} size="sm" variant={a.variant ?? "secondary"} href={a.href}>
          {a.label}
        </ButtonLink>
      ) : (
        <Button key={a.id} size="sm" variant={a.variant ?? "secondary"} onClick={a.onSelect}>
          {a.label}
        </Button>
      );
    return (
      <div className={s.actions}>
        {visible.map(one)}
        {more.length > 0 && moreLabel && (
          <MenuButton size="sm" icon="more" label={moreLabel} items={more.map((a): MenuItem => {
            const base = { id: a.id, label: a.label, variant: a.variant === "danger" ? ("danger" as const) : undefined };
            return a.href ? { ...base, href: a.href, onSelect: a.onSelect } : { ...base, onSelect: a.onSelect ?? (() => {}) };
          })} />
        )}
      </div>
    );
  };
  return (
    <div>
      {selection && count > 0 && (
        <div className={s.banner} role="status" data-selection-bar>
          <strong>{selection.labels.selected(count)}</strong>
          <span>{selection.state.scope === "all" ? selection.labels.allResults(selection.total) : selection.labels.pageOnly}</span>
          {offerAllResults(selection.state, ids, selection.total) && (
            <Button variant="link" onClick={() => selection.onChange(selectAllResults(selection.state, ids, selection.total, selection.filterKey))}>
              {selection.labels.selectAllResults(selection.total)}
            </Button>
          )}
          <Button variant="link" onClick={() => selection.onChange(clearSelection())}>
            {selection.labels.clear}
          </Button>
          {selection.actions && <div className={s.selectionActions}>{selection.actions}</div>}
        </div>
      )}
      <div className={s.scrollHint} id={hintId} data-table-scroll-hint><Icon name="chevronLeft" size={16} /><span>{scrollHint ?? caption}</span><Icon name="chevronRight" size={16} /></div>
      <div className={s.scroll} tabIndex={0} role="group" aria-label={caption} aria-describedby={hintId} data-table-scroll>
        <table className={s.table} aria-busy={status === "loading" || undefined}>
          <caption className={s.sr}>{caption}</caption>
          <thead>
            <tr>
              {selection && (
                <th scope="col" className={s.selectCell}>
                  <CheckBox
                    focusRef={pageCheck}
                    label={selection.labels.selectPage}
                    checked={selection.state.scope === "all" || pageSelection(selection.state, ids) === "all"}
                    mixed={selection.state.scope === "page" && pageSelection(selection.state, ids) === "some"}
                    onChange={() => selection.onChange(togglePage(selection.state, ids))}
                  />
                </th>
              )}
              {columns.map((c) => (
                <th key={c.id} scope="col" data-align={c.align} data-wrap={c.wrap || undefined} style={c.width ? { width: c.width } : undefined}>
                  {c.header}
                </th>
              ))}
              {actions && (
                <th scope="col" data-align="end" data-actions="">
                  {actionsHeader ?? <span className={s.sr}>{caption}</span>}
                </th>
              )}
            </tr>
          </thead>
          <tbody>
            {status === "loading" &&
              [0, 1, 2].map((i) => (
                <tr key={i} aria-hidden="true">
                  <td colSpan={span}>
                    <span className={s.skeleton} />
                  </td>
                </tr>
              ))}
            {status === "loading" && (
              <tr>
                <td colSpan={span}>
                  <span className={s.sr} role="status">{loadingLabel}</span>
                </td>
              </tr>
            )}
            {status === "error" && (
              <tr>
                <td colSpan={span} className={s.fullRow}>{error}</td>
              </tr>
            )}
            {status === "ready" && rows.length === 0 && (
              <tr>
                <td colSpan={span} className={s.fullRow}>{empty}</td>
              </tr>
            )}
            {status === "ready" &&
              rows.map((row, index) => {
                const id = ids[index] as string;
                const picked = selection ? selection.state.scope === "all" || selection.state.ids.has(id) : false;
                return (
                  <tr key={id} data-selected={picked}>
                    {selection && (
                      <td className={s.selectCell}>
                        <CheckBox label={selection.labels.selectRow(index + 1)} checked={picked} onChange={() => selection.onChange(toggleRow(selection.state, id, ids))} />
                      </td>
                    )}
                    {columns.map((c) => (
                      <td key={c.id} data-align={c.align} data-wrap={c.wrap || undefined}>
                        {c.cell(row)}
                      </td>
                    ))}
                    {actions && <td data-align="end" data-actions="">{renderActions(row)}</td>}
                  </tr>
                );
              })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/* ----------------------------- filter bar ----------------------------- */

/** Row of filters above a table. Children: FilterSelect, SearchBox, FilterDivider, a bulk MenuButton. */
export function FilterBar({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className={s.filters} role="group" aria-label={label}>
      {children}
    </div>
  );
}

export function FilterSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: readonly { value: string; label: string }[];
}) {
  const id = useId();
  return (
    <span className={s.filterField}>
      <label htmlFor={id}>{label}</label>
      <select id={id} className={s.select} value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </span>
  );
}

/** Search box with its own button (Enter or click submits); the button's accessible name is `buttonLabel`. */
export function SearchBox({
  label,
  value,
  onChange,
  onSubmit,
  buttonLabel,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  onSubmit: (value: string) => void;
  buttonLabel: string;
  placeholder?: string;
}) {
  return (
    <form
      className={s.search}
      role="search"
      onSubmit={(event: FormEvent) => {
        event.preventDefault();
        onSubmit(value);
      }}
    >
      <input className={s.searchInput} type="search" aria-label={label} placeholder={placeholder} value={value} onChange={(event) => onChange(event.target.value)} />
      <button type="submit" className={`${s.searchButton} ${s.focus}`} aria-label={buttonLabel}>
        <Icon name="search" size={16} />
      </button>
    </form>
  );
}

export function FilterDivider() {
  return <span className={s.divider} role="separator" aria-orientation="vertical" />;
}

/** Bulk menu: disabled with a reason until something is selected. */
export function BulkMenu({ label, items, disabledReason }: { label: string; items: readonly MenuItem[]; disabledReason?: string }) {
  return <MenuButton label={label} items={items} align="start" disabledReason={disabledReason} />;
}

/* ----------------------------- pagination ----------------------------- */

export type PaginationLabels = { nav: string; previous: string; next: string; page: (n: number) => string; summary?: string };

/**
 * Page links are real anchors (`hrefFor(n)` builds the URL, see withPage in ./logic) so the page survives refresh, back and share.
 * `onNavigate` lets a client router take over the click; without it the browser follows the link.
 */
export function Pagination({
  page,
  pageCount,
  hrefFor,
  labels,
  onNavigate,
}: {
  page: number;
  pageCount: number;
  hrefFor: (page: number) => string;
  labels: PaginationLabels;
  onNavigate?: (page: number, event: MouseEvent<HTMLAnchorElement>) => void;
}) {
  // Issue 107: an in-place page change can unmount the link that had focus (Next turns into a disabled, non-focusable arrow on the
  // last page), dropping focus to <body>. Keep it in the pager on the current page link, next to where the user acted (WAI-ARIA
  // APG, Developing a Keyboard Interface: never lose focus when the focused element goes away). Never steals focus that moved on.
  const nav = useRef<HTMLElement>(null);
  const navigated = useRef(false);
  useEffect(() => {
    if (!navigated.current) return;
    navigated.current = false;
    if (!document.activeElement || document.activeElement === document.body)
      nav.current?.querySelector<HTMLElement>('[aria-current="page"]')?.focus();
  }, [page]);
  const link = (n: number, content: ReactNode, name: string, disabled = false, current = false) =>
    disabled ? (
      // a disabled arrow is not an <a> without href (axe aria-prohibited-attr): a link-role span keeps the name, aria-disabled and no tab stop
      <span className={s.pageLink} role="link" aria-label={name} aria-disabled="true">
        {content}
      </span>
    ) : (
    <a
      className={s.pageLink}
      href={hrefFor(n)}
      aria-label={name}
      aria-current={current ? "page" : undefined}
      onClick={(event) => {
        onNavigate?.(n, event);
        navigated.current = event.defaultPrevented; // only an in-place page change; a followed link reloads anyway
      }}
    >
      {content}
    </a>
    );
  return (
    <nav ref={nav} className={s.pager} aria-label={labels.nav}>
      <span className={s.summary}>{labels.summary}</span>
      <ul className={s.pages}>
        <li>{link(page - 1, <Icon name="chevronLeft" size={16} />, labels.previous, page <= 1)}</li>
        {pageWindow(page, pageCount).map((entry, i) => (
          <li key={entry === "gap" ? `gap-${i}` : entry}>
            {entry === "gap" ? <span className={s.gap} aria-hidden="true">…</span> : link(entry, entry, labels.page(entry), false, entry === page)}
          </li>
        ))}
        <li>{link(page + 1, <Icon name="chevronRight" size={16} />, labels.next, page >= pageCount)}</li>
      </ul>
    </nav>
  );
}
