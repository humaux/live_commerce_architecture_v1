// Purpose: Owns the DS-1 page-structure components: Card, HelpHint, PageHeader, FormSection, ActionBars, SettingsLayout, KpiCard. Copy arrives through props only.
// Depends on: react, ./layout.module.css; no API, routing or translation.
// Used by: packages/ui/src/ds/index.ts; apps/admin/components/UiGallery.tsx.
"use client";
import { useId, useState, type ReactNode } from "react";
import { Icon } from "./Icon";
import s from "./layout.module.css";

type Level = 2 | 3 | 4;
const tag = (level: Level) => (level === 2 ? "h2" : level === 3 ? "h3" : "h4") as "h2" | "h3" | "h4";

/** A "?" toggle: the label names the button, the help text appears inline under the title row when opened. */
function useHelp(help: { label: string; content: ReactNode } | undefined) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const button = help && (
    <button type="button" className={s.help} aria-label={help.label} aria-expanded={open} aria-controls={id} onClick={() => setOpen(!open)}>
      <Icon name="help" size={16} />
    </button>
  );
  const panel = help && open && (
    <p id={id} className={s.helpText}>
      {help.content}
    </p>
  );
  return { button, panel };
}

/** White card with a blue title row: title, optional help "?" and counter on the left, actions on the right. */
export function Card({
  title,
  level = 2,
  meta,
  actions,
  help,
  flush = false,
  children,
  footer,
  id,
}: {
  title?: ReactNode;
  level?: Level;
  /** Small grey text next to the title, e.g. the "0 / 12" counter. */
  meta?: ReactNode;
  actions?: ReactNode;
  help?: { label: string; content: ReactNode };
  /** Removes body padding (tables sit flush against the card edge). */
  flush?: boolean;
  children?: ReactNode;
  id?: string;
  footer?: ReactNode;
}) {
  const { button, panel } = useHelp(help);
  const headingId = useId();
  const Heading = tag(level);
  const head = title !== undefined || actions;
  return (
    <section className={s.card} id={id} aria-labelledby={title !== undefined ? headingId : undefined}>
      {head && (
        <div className={s.cardHead}>
          {title !== undefined && (
            <Heading id={headingId} className={s.cardTitle}>
              {title}
              {button}
            </Heading>
          )}
          {meta && <span className={s.cardMeta}>{meta}</span>}
          {actions && <div className={s.cardActions}>{actions}</div>}
        </div>
      )}
      {panel}
      <div className={s.cardBody} data-flush={flush}>
        {children}
      </div>
      {footer && <div className={s.cardFooter}>{footer}</div>}
    </section>
  );
}

/** A form section: a Card whose title carries the help "?" and the field counter. */
export function FormSection(props: Omit<Parameters<typeof Card>[0], "flush">) {
  return <Card {...props} />;
}

export type Crumb = { label: string; href?: string };

/** Breadcrumb trail; the last crumb is the current page and is not a link. Also usable in the AppShellV2 breadcrumb slot. */
export function Breadcrumb({ label, crumbs }: { label: string; crumbs: readonly Crumb[] }) {
  return (
    <nav aria-label={label}>
      <ol className={s.crumbs}>
        {crumbs.map((crumb, i) => {
          const current = i === crumbs.length - 1;
          return (
            <li key={`${i}-${crumb.label}`}>
              {i > 0 && <Icon name="chevronRight" size={16} />}
              {crumb.href && !current ? <a href={crumb.href}>{crumb.label}</a> : <span aria-current={current ? "page" : undefined}>{crumb.label}</span>}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

/** Breadcrumb, h1, description and page-level actions. */
export function PageHeader({
  breadcrumbLabel,
  crumbs,
  title,
  description,
  actions,
}: {
  breadcrumbLabel: string;
  crumbs?: readonly Crumb[];
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className={s.pageHeader}>
      <div className={s.pageHeading}>
        {crumbs && crumbs.length > 0 && <Breadcrumb label={breadcrumbLabel} crumbs={crumbs} />}
        <h1 className={s.pageTitle}>{title}</h1>
        {description && <p className={s.pageDescription}>{description}</p>}
      </div>
      {actions && <div className={s.pageActions}>{actions}</div>}
    </header>
  );
}

/**
 * The save bar of a form page. Desktop shows it above and below the form; at < 768 px only the bottom one is shown and it
 * sticks to the viewport bottom. `actions` is rendered twice: give its controls no `id`. `dirtyNote` replaces `note` while there are unsaved changes.
 */
export function FormActionBar({
  label,
  note,
  dirty = false,
  dirtyNote,
  state,
  savingNote,
  savedNote,
  actions,
  children,
}: {
  label: string;
  note?: ReactNode;
  dirty?: boolean;
  dirtyNote?: ReactNode;
  state?: "clean" | "dirty" | "saving" | "saved";
  savingNote?: ReactNode;
  savedNote?: ReactNode;
  actions: ReactNode;
  children: ReactNode;
}) {
  const activeState = state ?? (dirty ? "dirty" : "clean");
  const message = activeState === "saving" ? savingNote : activeState === "saved" ? savedNote : activeState === "dirty" ? dirtyNote : note;
  const bar = (position: "top" | "bottom") => (
    <div data-state={activeState} className={`${s.bar} ${position === "top" ? s.barTop : s.barBottom}`} role="group" aria-label={label}>
      <div className={`${s.barNote} ${dirty ? s.barDirty : ""}`} role={position === "bottom" ? "status" : undefined}>{message}</div>
      <div className={s.barControls}>{actions}</div>
    </div>
  );
  return (
    <div className={s.formFrame}>
      {bar("top")}
      <div className={s.formContent}>{children}</div>
      {bar("bottom")}
    </div>
  );
}

/** Compatibility alias; all save-bar layout lives in FormActionBar. */
export const ActionBars = FormActionBar;

export type SettingsGroup = { id: string; title: string; items: readonly { id: string; label: string; href: string }[] };

/** One card with a left sub-nav (group titles, current item marked by a blue bar) and the content on the right. */
export function SettingsLayout({
  navLabel,
  groups,
  current,
  children,
}: {
  navLabel: string;
  groups: readonly SettingsGroup[];
  current: string;
  children: ReactNode;
}) {
  return (
    <div className={`${s.card} ${s.settings}`}>
      <nav className={s.settingsNav} aria-label={navLabel}>
        {groups.map((group) => (
          <div key={group.id}>
            <p className={s.settingsGroupTitle}>{group.title}</p>
            <ul className={s.settingsGroup}>
              {group.items.map((item) => (
                <li key={item.id}>
                  <a className={s.settingsLink} href={item.href} aria-current={item.id === current ? "page" : undefined}>
                    {item.label}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>
      <div className={s.settingsBody}>{children}</div>
    </div>
  );
}

/** A number that opens its list: with `href` the whole card is one link whose name is "value label". */
export function KpiCard({
  value,
  label,
  href,
  icon,
  trend,
}: {
  value: string;
  label: string;
  href?: string;
  icon?: ReactNode;
  trend?: { text: string; tone: "success" | "danger" | "neutral" };
}) {
  const body = (
    <>
      {icon && <span className={s.kpiIcon} aria-hidden="true">{icon}</span>}
      <span>
        <span className={s.kpiValue}>{value}</span>
        <span className={s.kpiLabel}>{label}</span>
        <span className={s.kpiTrend} data-tone={trend?.tone}>{trend?.text ?? "\u00a0"}</span>
      </span>
    </>
  );
  return href ? (
    <a className={s.kpi} href={href}>
      {body}
    </a>
  ) : (
    <div className={s.kpi}>{body}</div>
  );
}

export type TimelineItem = { id: string; title: string; time: string; dateTime: string; detail?: string; tone?: "success" | "neutral" };
/** Chronological facts; every visible time has a machine-readable ISO instant. */
export function Timeline({ label, items }: { label: string; items: readonly TimelineItem[] }) {
  return <ol className={s.timeline} aria-label={label}>{items.map(item => <li key={item.id}>
    <span className={s.timelineIcon}><Icon name={item.tone === "success" ? "check" : "info"} size={16} /></span>
    <div><strong>{item.title}</strong><time dateTime={item.dateTime}>{item.time}</time>{item.detail && <p>{item.detail}</p>}</div>
  </li>)}</ol>;
}
/** Task-specific two-column detail: reading order remains main then summary on mobile. */
export function DetailLayout({ main, summary }: { main: ReactNode; summary: ReactNode }) {
  return <div className={s.detail}><div className={s.detailMain}>{main}</div><aside>{summary}</aside></div>;
}
export function Stack({ children }: { children: ReactNode }) { return <div className={s.stack}>{children}</div>; }
export function Inline({ children }: { children: ReactNode }) { return <div className={s.inline}>{children}</div>; }
