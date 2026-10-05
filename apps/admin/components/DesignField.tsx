// Labelled form field of the Design editor: label text, control, and the inline error line (role=alert) used for both
// local "required" hints and the server's 422 path message. Presentational only.
import {
  cloneElement,
  isValidElement,
  useId,
  type ReactNode,
  type ReactElement,
} from "react";
import { Field as SharedField } from "@live-commerce/ui";

export function Field({
  label,
  error,
  children,
}: {
  label: string;
  error?: string;
  children: ReactNode;
}) {
  const generated = useId();
  // Keep compound native controls (e.g. colour + swatch) wrapped by their existing
  // label; single native controls share Field without changing values or events.
  if (
    isValidElement(children) &&
    ["input", "select", "textarea"].includes(String(children.type))
  ) {
    const control = children as ReactElement<{
      id?: string;
      type?: string;
      maxLength?: number;
      "aria-describedby"?: string;
    }>;
    const id = control.props.id ?? generated;
    const width =
      children.type === "textarea"
        ? "full"
        : control.props.type === "url" || (control.props.maxLength ?? 0) > 120
          ? "long"
          : control.props.type === "tel" ||
              (control.props.maxLength ?? 120) <= 40
            ? "short"
            : "medium";
    return (
      <SharedField
        id={id}
        label={label}
        width={width}
        className={error ? "design-field has-error" : "design-field"}
        hint={
          error ? (
            <span className="design-error" role="alert">
              {error}
            </span>
          ) : undefined
        }
      >
        {cloneElement(control, {
          id,
          "aria-describedby":
            [
              control.props["aria-describedby"],
              error ? `${id}-hint` : undefined,
            ]
              .filter(Boolean)
              .join(" ") || undefined,
        })}
      </SharedField>
    );
  }
  return (
    <label className={error ? "design-field has-error" : "design-field"}>
      <span className="design-label">{label}</span>
      {children}
      {error ? (
        <small className="design-error" role="alert">
          {error}
        </small>
      ) : null}
    </label>
  );
}
