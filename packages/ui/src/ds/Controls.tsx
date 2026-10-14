// Purpose: Owns the DS-1 interactive primitives: Button (disabled-with-reason, loading), ButtonLink, SegmentedSwitch, Tabs, Badge, StatusPill, Alert. All copy arrives through props.
// Depends on: react, ./controls.module.css, ./logic (button state, status families); no API, routing or translation.
// Used by: packages/ui/src/ds/index.ts and the other ds components; apps/admin/components/UiGallery.tsx.
"use client";
import {
  useId,
  useRef,
  useState,
  type AnchorHTMLAttributes,
  type ButtonHTMLAttributes,
  type KeyboardEvent,
  type ReactNode,
  type Ref,
} from "react";
import s from "./controls.module.css";
import { buttonState, statusTone, type StatusDomain, type StatusOf, type Tone } from "./logic";

export type ButtonVariant = "primary" | "secondary" | "danger" | "link";
export type ButtonSize = "sm" | "md" | "lg";

export type ButtonProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, "disabled" | "type"> & {
  /** React 19 passes `ref` as a prop; it lands on the <button>. */
  ref?: Ref<HTMLButtonElement>;
  variant?: ButtonVariant;
  size?: ButtonSize;
  type?: "button" | "submit" | "reset";
  /** Shows a spinner and swallows clicks. Keep the children as the label; `loadingLabel` is announced instead while busy. */
  loading?: boolean;
  loadingLabel?: string;
  /** There is no plain `disabled`: a button that cannot be used says why. Clicking it reveals the reason in place. */
  disabledReason?: string;
};

/** A button that is never silently dead: loading shows a spinner, a disabled one explains itself (title, aria-describedby and a visible note on click). */
export function Button({
  variant = "secondary",
  size = "md",
  type = "button",
  loading = false,
  loadingLabel,
  disabledReason,
  onClick,
  children,
  className = "",
  ...props
}: ButtonProps) {
  const state = buttonState(disabledReason, loading);
  const [revealed, setRevealed] = useState(false);
  const reasonId = useId();
  const reasonShown = state.reason !== undefined && !state.busy;
  const button = (
    <button
      {...props}
      type={type}
      className={`${s.button} ${className}`}
      data-variant={variant}
      data-size={size}
      aria-disabled={state.blocked || undefined}
      aria-busy={state.busy || undefined}
      aria-describedby={reasonShown ? reasonId : props["aria-describedby"]}
      title={reasonShown ? state.reason : props.title}
      onClick={(event) => {
        if (state.blocked) {
          event.preventDefault();
          if (reasonShown) setRevealed(true);
          return;
        }
        onClick?.(event);
      }}
    >
      {state.busy && <span className={s.spinner} aria-hidden="true" />}
      {children}
      {state.busy && loadingLabel && <span className={s.sr}>{loadingLabel}</span>}
    </button>
  );
  if (!reasonShown) return button;
  return (
    <span className={s.reasonWrap}>
      {button}
      <span id={reasonId} className={revealed ? s.reason : s.sr} role={revealed ? "status" : undefined}>
        {state.reason}
      </span>
    </span>
  );
}

/** An anchor that looks like a Button (navigation is a link, not a button). */
export function ButtonLink({
  variant = "secondary",
  size = "md",
  className = "",
  children,
  ...props
}: AnchorHTMLAttributes<HTMLAnchorElement> & { variant?: ButtonVariant; size?: ButtonSize }) {
  return (
    <a {...props} className={`${s.button} ${className}`} data-variant={variant} data-size={size}>
      {children}
    </a>
  );
}

/** The 是/否 switch. Both words are visible text; assistive tech sees one switch named by `label`. */
export function SegmentedSwitch({
  checked,
  onChange,
  label,
  onLabel,
  offLabel,
  disabledReason,
  ...props
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  label: string;
  onLabel: string;
  offLabel: string;
  disabledReason?: string;
} & Omit<ButtonHTMLAttributes<HTMLButtonElement>, "onChange" | "role" | "children" | "aria-label" | "aria-checked" | "disabled" | "type">) {
  const state = buttonState(disabledReason, false);
  return (
    <button
      {...props}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      aria-disabled={state.blocked || undefined}
      title={state.reason}
      className={s.switch}
      onClick={() => {
        if (!state.blocked) onChange(!checked);
      }}
    >
      <span className={s.seg} data-side="on" data-on={checked} aria-hidden="true">{onLabel}</span>
      <span className={s.seg} data-side="off" data-on={!checked} aria-hidden="true">{offLabel}</span>
    </button>
  );
}

export type TabItem = { id: string; label: ReactNode; count?: number };

/** Underline tabs (page level) or section tabs (form pages: current tab white, others grey). Arrow keys, Home and End move and select. */
export function Tabs({
  items,
  value,
  onChange,
  variant = "underline",
  ariaLabel,
  idPrefix,
}: {
  items: readonly TabItem[];
  value: string;
  onChange: (id: string) => void;
  variant?: "underline" | "section";
  ariaLabel: string;
  /** Prefix of the tab ids; TabPanel uses the same prefix to point back at its tab. */
  idPrefix: string;
}) {
  const refs = useRef<Array<HTMLButtonElement | null>>([]);
  const move = (event: KeyboardEvent, index: number) => {
    const last = items.length - 1;
    const next =
      event.key === "ArrowRight" ? (index === last ? 0 : index + 1)
      : event.key === "ArrowLeft" ? (index === 0 ? last : index - 1)
      : event.key === "Home" ? 0
      : event.key === "End" ? last
      : -1;
    const item = items[next];
    if (!item) return;
    event.preventDefault();
    onChange(item.id);
    refs.current[next]?.focus();
  };
  return (
    <div className={s.tabs} data-variant={variant} role="tablist" aria-label={ariaLabel}>
      {items.map((item, index) => (
        <button
          key={item.id}
          ref={(node) => {
            refs.current[index] = node;
          }}
          id={`${idPrefix}-tab-${item.id}`}
          type="button"
          role="tab"
          className={s.tab}
          aria-selected={item.id === value}
          aria-controls={`${idPrefix}-panel-${item.id}`}
          tabIndex={item.id === value ? 0 : -1}
          onClick={() => onChange(item.id)}
          onKeyDown={(event) => move(event, index)}
        >
          {item.label}
          {item.count !== undefined && <span className={s.tabCount}>{item.count}</span>}
        </button>
      ))}
    </div>
  );
}

/** The panel of one tab; render it for the active tab only. */
export function TabPanel({ idPrefix, id, children }: { idPrefix: string; id: string; children: ReactNode }) {
  return (
    <div role="tabpanel" id={`${idPrefix}-panel-${id}`} aria-labelledby={`${idPrefix}-tab-${id}`} tabIndex={0}>
      {children}
    </div>
  );
}

const glyphs: Record<Tone, string> = { neutral: "–", info: "●", success: "✓", warning: "!", danger: "✕" };

/** A tone-coloured label with a shape glyph, so meaning never rests on colour alone. */
export function Badge({ tone = "neutral", children }: { tone?: Tone; children: ReactNode }) {
  return (
    <span className={s.badge} data-tone={tone}>
      <span className={s.glyph} aria-hidden="true">{glyphs[tone]}</span>
      {children}
    </span>
  );
}

/**
 * A badge from one domain's closed vocabulary (payment, fulfilment, refund, connection). The status must belong to the
 * domain (compile-time) and the label comes from the caller's copy, never from this package.
 */
export function StatusPill<D extends StatusDomain>({ domain, status, label }: { domain: D; status: StatusOf<D>; label: string }) {
  return (
    <span data-status-domain={domain} data-status={status}>
      <Badge tone={statusTone(domain, status)}>{label}</Badge>
    </span>
  );
}

/** Inline message. Danger is role="alert" (announced at once); the other tones are polite status text. */
export function Alert({
  tone = "info",
  title,
  children,
  action,
  onDismiss,
  dismissLabel,
}: {
  tone?: Tone;
  title?: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  onDismiss?: () => void;
  /** Required with onDismiss so the close button has a name. */
  dismissLabel?: string;
}) {
  return (
    <div className={s.alert} data-tone={tone} role={tone === "danger" ? "alert" : "status"}>
      <span className={s.glyph} aria-hidden="true">{glyphs[tone]}</span>
      <div className={s.alertBody}>
        {title && <strong className={s.alertTitle}>{title}</strong>}
        {children && <div>{children}</div>}
      </div>
      {action && <div className={s.alertAction}>{action}</div>}
      {onDismiss && dismissLabel && (
        <button type="button" className={s.dismiss} aria-label={dismissLabel} onClick={onDismiss}>
          <span aria-hidden="true">×</span>
        </button>
      )}
    </div>
  );
}
