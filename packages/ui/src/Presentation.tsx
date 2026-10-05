"use client";

// Presentational building blocks. Domain data, commands and translations stay with the caller.
import {
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
  type InputHTMLAttributes,
  type HTMLAttributes,
  type Ref,
} from "react";
import s from "./Presentation.module.css";
export { s as presentationStyles };

// Keep the browser's keyboard/calendar control and native value contract. The
// resting label does not inherit the browser installation's English placeholder.
export function DateControl({ emptyLabel, value, ...props }: {
  emptyLabel: string;
} & InputHTMLAttributes<HTMLInputElement>) {
  const text = typeof value === "string" && value ? value.replace("T", " ") : emptyLabel;
  return <span className={s.dateControl}>
    <span aria-hidden="true" className={s.dateDisplay}>{text}</span>
    <input {...props} type={props.type ?? "date"} value={value} />
  </span>;
}

export function PageHeader({
  title,
  description,
  actions,
  children,
  className = "",
  ...props
}: {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
} & Omit<HTMLAttributes<HTMLElement>, "title">) {
  return (
    <header {...props} className={`${s.header} ${className}`}>
      <div className={s.heading}>
        <h1>{title}</h1>
        {description && <p>{description}</p>}
        {children}
      </div>
      {actions && <div className={s.actions}>{actions}</div>}
    </header>
  );
}

export function FormRow({
  children,
  className = "",
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div {...props} className={`${s.formRow} ${className}`}>
      {children}
    </div>
  );
}

export function Field({
  label,
  children,
  hint,
  id,
  width = "medium",
  className = "",
}: {
  label: ReactNode;
  children: ReactNode;
  hint?: ReactNode;
  id: string;
  width?: "short" | "medium" | "long" | "full";
  className?: string;
}) {
  return (
    <div className={`${s.field} ${className}`} data-width={width}>
      <label htmlFor={id}>{label}</label>
      <div className={s.control}>{children}</div>
      <div id={`${id}-hint`} className={s.hint}>
        {hint}
      </div>
    </div>
  );
}

export function Badge({
  tone = "neutral",
  className = "",
  children,
  ...props
}: HTMLAttributes<HTMLSpanElement> & {
  tone?: "neutral" | "success" | "warning" | "danger" | "info";
}) {
  return (
    <span {...props} className={`${s.badge} ${className}`} data-tone={tone}>
      {children}
    </span>
  );
}

function useScrollFrame() {
  const ref = useRef<HTMLDivElement>(null);
  const [edges, setEdges] = useState({ before: false, after: false });
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const measure = () =>
      setEdges({
        before: element.scrollLeft > 1,
        after:
          element.scrollWidth - element.clientWidth - element.scrollLeft > 1,
      });
    const resize = new ResizeObserver(measure);
    const mutation = new MutationObserver(measure);
    resize.observe(element);
    for (const child of element.children) resize.observe(child);
    mutation.observe(element, { childList: true, subtree: true });
    element.addEventListener("scroll", measure, { passive: true });
    measure();
    return () => {
      resize.disconnect();
      mutation.disconnect();
      element.removeEventListener("scroll", measure);
    };
  }, []);
  return { ref, edges };
}

export function TabStrip({
  children,
  label,
  previousLabel,
  nextLabel,
  ...props
}: {
  children: ReactNode;
  label: string;
  previousLabel: string;
  nextLabel: string;
} & HTMLAttributes<HTMLDivElement>) {
  const { ref, edges } = useScrollFrame();
  useEffect(() => {
    const element = ref.current;
    const selected = element?.querySelector<HTMLElement>(
      '[aria-selected="true"],[aria-pressed="true"],.active',
    );
    if (!element || !selected) return;
    const outer = element.getBoundingClientRect(),
      inner = selected.getBoundingClientRect();
    if (inner.left < outer.left) element.scrollLeft -= outer.left - inner.left;
    else if (inner.right > outer.right)
      element.scrollLeft += inner.right - outer.right;
  }, [children, ref]);
  return (
    <div className={s.tabs}>
      {(edges.before || edges.after) && (
        <button
          type="button"
          className={s.scrollButton}
          aria-label={previousLabel}
          aria-disabled={!edges.before}
          onClick={() => edges.before && ref.current?.scrollBy({ left: -240 })}
        >
          <Chevron reverse />
        </button>
      )}
      <div role="group" {...props} aria-label={props["aria-label"] ?? label} ref={ref} className={`${s.tabStrip} ${props.className ?? ""}`}>
        {children}
      </div>
      {(edges.before || edges.after) && (
        <button
          type="button"
          className={s.scrollButton}
          aria-label={nextLabel}
          aria-disabled={!edges.after}
          onClick={() => edges.after && ref.current?.scrollBy({ left: 240 })}
        >
          <Chevron />
        </button>
      )}
    </div>
  );
}

function Chevron({ reverse = false }: { reverse?: boolean }) {
  return (
    <svg
      aria-hidden="true"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
    >
      <path d={reverse ? "m15 5-7 7 7 7" : "m9 5 7 7-7 7"} />
    </svg>
  );
}

export function TableFrame({
  children,
  label,
  scrollHint,
  className = "",
}: {
  children: ReactNode;
  label: string;
  scrollHint: string;
  className?: string;
}) {
  const { ref, edges } = useScrollFrame();
  const hintId = useId();
  return (
    <div className={`${s.tableContainer} ${className}`}>
      <div
        ref={ref}
        className={s.tableFrame}
        role="region"
        aria-label={label}
        tabIndex={edges.before || edges.after ? 0 : undefined}
        aria-describedby={edges.before || edges.after ? hintId : undefined}
      >
        {children}
      </div>
      {(edges.before || edges.after) && (
        <p id={hintId} className={s.scrollHint}>
          {scrollHint}
        </p>
      )}
    </div>
  );
}

export function FilePicker({
  label,
  emptyLabel,
  onChange,
  className = "",
  inputRef,
  fileName,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  emptyLabel: string;
  inputRef?: Ref<HTMLInputElement>;
  fileName?: string;
}) {
  const [name, setName] = useState("");
  return (
    <div className={`${s.filePicker} ${className}`}>
      <label className={s.fileButton} data-disabled={!!props.disabled}>
        <span>{label}</span>
      <input
        ref={inputRef}
          {...props}
          type="file"
          aria-label={props["aria-label"] ?? label}
          onChange={(event) => {
            setName(
              Array.from(event.currentTarget.files ?? [])
                .map((file) => file.name)
                .join(", "),
            );
            onChange?.(event);
          }}
        />
      </label>
      <span className={s.fileName}>{(fileName ?? name) || emptyLabel}</span>
    </div>
  );
}
